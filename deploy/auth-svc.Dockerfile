# Stage 1: Build
FROM maven:3.9-eclipse-temurin-21 AS builder
WORKDIR /app
COPY pom.xml .
RUN mvn dependency:go-offline -B
COPY src ./src
RUN mvn package -DskipTests -B

# Stage 2: Runtime
FROM eclipse-temurin:21-jre
WORKDIR /app
ARG GIT_SHA=unknown
ARG BUILD_TIME=unknown
ARG GIT_DIRTY=unknown

# ---- OpenTelemetry Java agent (factor 14 — distributed tracing) ----
# Auto-instrumentation: agent tự extract W3C traceparent từ gRPC metadata (nối chuỗi
# từ api-svc) và export OTLP/gRPC tới OTel Collector. Attach qua JAVA_TOOL_OPTIONS
# (khai trong ConfigMap k8s). Cấu hình còn lại (endpoint, service name, sampler) qua
# env OTEL_* trong ConfigMap. Pin version cho reproducible build.
ARG OTEL_AGENT_VERSION=2.11.0
ADD https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/download/v${OTEL_AGENT_VERSION}/opentelemetry-javaagent.jar /otel/opentelemetry-javaagent.jar
RUN chmod 0644 /otel/opentelemetry-javaagent.jar
# Shared version-stamp dir 0777 (volume inits world-writable for non-root UIDs).
# The printf consumes the version args so a changed SHA/BUILD_TIME busts the ENV
# below — a bare ENV layer is keyed on the literal string and freezes values.
RUN mkdir -p /versions && chmod 0777 /versions && \
    printf 'git_sha=%s build_time=%s dirty=%s\n' "$GIT_SHA" "$BUILD_TIME" "$GIT_DIRTY" > /etc/image-version
ENV GIT_SHA=$GIT_SHA BUILD_TIME=$BUILD_TIME GIT_DIRTY=$GIT_DIRTY
COPY --from=builder /app/target/auth-service-*.jar app.jar
ENTRYPOINT ["java", "-jar", "app.jar"]
