//! Observability wiring for the gateway (factor 14).
//!
//! Two independent pillars:
//!  * **Tracing** — OpenTelemetry OTLP/gRPC exporter → OTel Collector → Tempo.
//!    The gateway is the EDGE service, so it MINTS the root span for every
//!    inbound request and INJECTS the W3C `traceparent` into the outbound
//!    reqwest request so the trace continues into api-svc.
//!  * **Metrics** — a Prometheus recorder rendered as text on the admin port
//!    :9100 (`/metrics`). Never proxied to the public route table.
//!
//! Both are best-effort: if the collector is unreachable at boot we still run,
//! spans just fail to export. Nothing here should ever panic the process.

use std::collections::HashMap;

use opentelemetry::propagation::{Injector, TextMapPropagator};
use opentelemetry::trace::TracerProvider as _;
use opentelemetry::KeyValue;
use opentelemetry_otlp::WithExportConfig;
use opentelemetry_sdk::propagation::TraceContextPropagator;
use opentelemetry_sdk::trace::{Sampler, TracerProvider};
use opentelemetry_sdk::Resource;
use tracing::info;
use tracing_subscriber::layer::SubscriberExt;
use tracing_subscriber::util::SubscriberInitExt;
use tracing_subscriber::{EnvFilter, Layer};

pub mod metrics;

/// Default collector endpoint when `OTEL_EXPORTER_OTLP_ENDPOINT` is unset.
const DEFAULT_OTLP_ENDPOINT: &str = "http://otel-collector.observability:4317";
const DEFAULT_SERVICE_NAME: &str = "gateway-svc";

/// Guard returned from [`init`]; drop it on shutdown to flush pending spans.
pub struct TelemetryGuard {
    provider: Option<TracerProvider>,
}

impl TelemetryGuard {
    /// Flush and tear down the tracer provider so buffered spans are exported.
    pub fn shutdown(&mut self) {
        if let Some(provider) = self.provider.take() {
            // force_flush drains the batch span processor; shutdown stops it.
            for r in provider.force_flush() {
                if let Err(e) = r {
                    tracing::warn!(error = %e, "otel flush error on shutdown");
                }
            }
            if let Err(e) = provider.shutdown() {
                tracing::warn!(error = %e, "otel provider shutdown error");
            }
        }
    }
}

impl Drop for TelemetryGuard {
    fn drop(&mut self) {
        self.shutdown();
    }
}

/// Build the tracing subscriber (console fmt layer preserved) plus, when an
/// OTLP endpoint is reachable, an OpenTelemetry layer that ships spans to the
/// collector. Returns a guard whose `Drop` flushes the exporter.
///
/// This REPLACES the former `init_tracing()` in main.rs: it keeps the exact
/// same compact+ansi console output and adds the otel layer beside it.
pub fn init() -> TelemetryGuard {
    let log_level = std::env::var("RUST_LOG").unwrap_or_else(|_| "info,gateway=debug".to_string());
    let log_format = std::env::var("LOG_FORMAT").unwrap_or_else(|_| "console".to_string());

    // Console layer — identical formatting to the previous init_tracing().
    let fmt_layer = if log_format == "json" {
        tracing_subscriber::fmt::layer().json().boxed()
    } else {
        tracing_subscriber::fmt::layer()
            .with_ansi(true) // force colors even without a TTY (Docker/k8s)
            .with_target(false)
            .compact()
            .boxed()
    };

    // W3C propagator installed globally so extract/inject use traceparent.
    opentelemetry::global::set_text_map_propagator(TraceContextPropagator::new());

    let provider = build_tracer_provider();

    let registry = tracing_subscriber::registry()
        .with(EnvFilter::new(log_level))
        .with(fmt_layer);

    match &provider {
        Some(p) => {
            let tracer = p.tracer(DEFAULT_SERVICE_NAME);
            // Register as the global provider so span context propagates.
            opentelemetry::global::set_tracer_provider(p.clone());
            registry
                .with(tracing_opentelemetry::layer().with_tracer(tracer))
                .init();
            info!("telemetry: OTLP tracing enabled");
        }
        None => {
            registry.init();
            info!("telemetry: OTLP tracing disabled (no exporter)");
        }
    }

    TelemetryGuard { provider }
}

fn build_tracer_provider() -> Option<TracerProvider> {
    let endpoint = std::env::var("OTEL_EXPORTER_OTLP_ENDPOINT")
        .unwrap_or_else(|_| DEFAULT_OTLP_ENDPOINT.to_string());
    let service_name =
        std::env::var("OTEL_SERVICE_NAME").unwrap_or_else(|_| DEFAULT_SERVICE_NAME.to_string());

    let exporter = match opentelemetry_otlp::SpanExporter::builder()
        .with_tonic()
        .with_endpoint(&endpoint)
        .build()
    {
        Ok(e) => e,
        Err(e) => {
            tracing::warn!(error = %e, endpoint = %endpoint, "otel: failed to build OTLP exporter");
            return None;
        }
    };

    let provider = TracerProvider::builder()
        .with_batch_exporter(exporter, opentelemetry_sdk::runtime::Tokio)
        .with_sampler(Sampler::ParentBased(Box::new(Sampler::AlwaysOn)))
        .with_resource(build_resource(&service_name))
        .build();

    Some(provider)
}

/// Resource attributes: service.name plus anything in OTEL_RESOURCE_ATTRIBUTES
/// (comma-separated `k=v` pairs, per the OTel spec).
fn build_resource(service_name: &str) -> Resource {
    let mut kvs = vec![KeyValue::new(
        opentelemetry_semantic_conventions::resource::SERVICE_NAME,
        service_name.to_string(),
    )];
    if let Ok(attrs) = std::env::var("OTEL_RESOURCE_ATTRIBUTES") {
        for pair in attrs.split(',') {
            if let Some((k, v)) = pair.split_once('=') {
                let (k, v) = (k.trim(), v.trim());
                if !k.is_empty() && !v.is_empty() {
                    kvs.push(KeyValue::new(k.to_string(), v.to_string()));
                }
            }
        }
    }
    Resource::new(kvs)
}

/// Adapter letting the OTel propagator write into a plain `HashMap`, which we
/// then copy onto the outbound reqwest header map (see `proxy/client.rs`).
struct HashMapInjector<'a>(&'a mut HashMap<String, String>);

impl Injector for HashMapInjector<'_> {
    fn set(&mut self, key: &str, value: String) {
        self.0.insert(key.to_string(), value);
    }
}

/// Serialize the *current* span's context into W3C headers (`traceparent`,
/// `tracestate`). Call from inside the request span so the child service joins
/// the same trace. Returns an empty map when no active context exists.
pub fn traceparent_headers() -> HashMap<String, String> {
    use tracing_opentelemetry::OpenTelemetrySpanExt;

    let cx = tracing::Span::current().context();
    let mut carrier = HashMap::new();
    let propagator = TraceContextPropagator::new();
    propagator.inject_context(&cx, &mut HashMapInjector(&mut carrier));
    carrier
}
