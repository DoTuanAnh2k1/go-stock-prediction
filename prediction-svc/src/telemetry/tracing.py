"""OpenTelemetry tracing initialisation for prediction-svc.

Reads standard OTel env vars:
  OTEL_EXPORTER_OTLP_ENDPOINT   (default: http://otel-collector.observability:4317)
  OTEL_EXPORTER_OTLP_PROTOCOL   (default: grpc)
  OTEL_SERVICE_NAME              (default: prediction-svc)
  OTEL_TRACES_SAMPLER            (default: parentbased_always_on)
  OTEL_RESOURCE_ATTRIBUTES       e.g. service.namespace=stock,deployment.environment=kind

Fail-safe: if the OTel libraries are not installed or the exporter cannot
reach the collector, initialisation succeeds with a no-op tracer so the
service continues operating normally.

Usage:
    from src.telemetry.tracing import init_tracing, shutdown_tracing
    init_tracing()    # call once at startup, after logger is up
    shutdown_tracing()  # call at SIGTERM
"""
from __future__ import annotations

import os
from src.utils.logger import get_logger

log = get_logger("telemetry.tracing")

_tracer_provider = None


def init_tracing() -> None:
    """Initialise OTel tracing.  Safe to call even when deps are absent."""
    global _tracer_provider

    # ----------------------------------------------------------------
    # Check deps are available — degrade gracefully if not
    # ----------------------------------------------------------------
    try:
        from opentelemetry import trace
        from opentelemetry.sdk.trace import TracerProvider
        from opentelemetry.sdk.trace.export import BatchSpanProcessor
        from opentelemetry.sdk.resources import Resource
    except ImportError:
        log.warning("telemetry.tracing.disabled", reason="opentelemetry-sdk not installed")
        return

    try:
        from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
    except ImportError:
        log.warning("telemetry.tracing.disabled",
                    reason="opentelemetry-exporter-otlp-proto-grpc not installed")
        return

    # ----------------------------------------------------------------
    # Build resource from env
    # ----------------------------------------------------------------
    service_name = os.environ.get("OTEL_SERVICE_NAME", "prediction-svc")
    raw_attrs = os.environ.get("OTEL_RESOURCE_ATTRIBUTES", "")
    resource_attrs: dict[str, str] = {"service.name": service_name}
    for pair in raw_attrs.split(","):
        pair = pair.strip()
        if "=" in pair:
            k, _, v = pair.partition("=")
            resource_attrs[k.strip()] = v.strip()

    resource = Resource.create(resource_attrs)

    # ----------------------------------------------------------------
    # Sampler
    # ----------------------------------------------------------------
    sampler_name = os.environ.get("OTEL_TRACES_SAMPLER", "parentbased_always_on")
    sampler = _build_sampler(sampler_name)

    # ----------------------------------------------------------------
    # TracerProvider + OTLP exporter
    # ----------------------------------------------------------------
    try:
        endpoint = os.environ.get(
            "OTEL_EXPORTER_OTLP_ENDPOINT",
            "http://otel-collector.observability:4317",
        )
        exporter = OTLPSpanExporter(endpoint=endpoint, insecure=True)
        processor = BatchSpanProcessor(exporter)
        provider = TracerProvider(resource=resource, sampler=sampler)
        provider.add_span_processor(processor)

        trace.set_tracer_provider(provider)
        _tracer_provider = provider
        log.info("telemetry.tracing.init", service=service_name,
                 endpoint=endpoint, sampler=sampler_name)
    except Exception as exc:
        log.warning("telemetry.tracing.init.failed", error=str(exc))


def shutdown_tracing() -> None:
    """Flush and shut down the tracer provider."""
    global _tracer_provider
    if _tracer_provider is None:
        return
    try:
        _tracer_provider.shutdown()
        log.info("telemetry.tracing.shutdown")
    except Exception as exc:
        log.warning("telemetry.tracing.shutdown.error", error=str(exc))
    finally:
        _tracer_provider = None


def _build_sampler(name: str):
    """Build an OTel sampler from the OTEL_TRACES_SAMPLER name.
    Falls back to ALWAYS_ON on unknown names.
    """
    try:
        from opentelemetry.sdk.trace.sampling import (
            ALWAYS_ON,
            ALWAYS_OFF,
            ParentBased,
            TraceIdRatioBased,
        )
        name = name.lower()
        if name == "always_on":
            return ALWAYS_ON
        if name == "always_off":
            return ALWAYS_OFF
        if name == "parentbased_always_on":
            return ParentBased(root=ALWAYS_ON)
        if name == "parentbased_always_off":
            return ParentBased(root=ALWAYS_OFF)
        if name.startswith("traceidratio"):
            # e.g. traceidratio(0.1) or just traceidratio
            ratio = 1.0
            if "(" in name:
                ratio = float(name.split("(")[1].rstrip(")"))
            return TraceIdRatioBased(ratio)
        if name.startswith("parentbased_traceidratio"):
            ratio = 1.0
            if "(" in name:
                ratio = float(name.split("(")[1].rstrip(")"))
            return ParentBased(root=TraceIdRatioBased(ratio))
        log.warning("telemetry.tracing.unknown_sampler", name=name, fallback="parentbased_always_on")
        return ParentBased(root=ALWAYS_ON)
    except Exception:
        return None


def get_grpc_server_interceptor():
    """Return a gRPC server-side OTel interceptor, or None if unavailable.

    Uses opentelemetry-instrumentation-grpc when available.
    The interceptor extracts W3C traceparent from incoming metadata, creating
    a child span so traces chain from api-svc → prediction-svc.
    """
    try:
        from opentelemetry.instrumentation.grpc import server_interceptor
        interceptor = server_interceptor()
        log.info("telemetry.grpc_interceptor.ready")
        return interceptor
    except Exception as exc:
        log.warning("telemetry.grpc_interceptor.unavailable", error=str(exc))
        return None
