# Stage 1: Build Rust binary
FROM rust:1.86-slim AS builder

WORKDIR /app

RUN apt-get update && \
    apt-get install -y pkg-config libssl-dev && \
    rm -rf /var/lib/apt/lists/*

# Cache dependencies first (layer caching)
COPY Cargo.toml ./
RUN mkdir src && \
    echo "fn main() {}" > src/main.rs && \
    cargo build --release 2>/dev/null || true && \
    rm -rf src

# Build the real binary
COPY src ./src
COPY config.yaml ./
RUN touch src/main.rs && cargo build --release

# Stage 2: Minimal runtime image
FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update && \
    apt-get install -y ca-certificates libssl3 curl openssl && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/target/release/gateway /app/gateway
COPY config.yaml /app/config.yaml
COPY docker-entrypoint.sh /docker-entrypoint.sh

RUN useradd -m -u 1001 gateway && \
    chown -R gateway:gateway /app && \
    chmod +x /docker-entrypoint.sh && \
    mkdir -p /etc/gateway/certs && \
    chown -R gateway:gateway /etc/gateway

EXPOSE 80 443

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD curl -f http://localhost:80/healthz || exit 1

USER gateway

ENTRYPOINT ["/docker-entrypoint.sh"]
