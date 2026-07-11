# Stage 1: Build Rust binary
FROM rust:1.90-slim-bookworm AS builder

WORKDIR /app

RUN apt-get update && \
    apt-get install -y pkg-config libssl-dev protobuf-compiler && \
    rm -rf /var/lib/apt/lists/*

# Cache dependencies first (layer caching). build.rs is absent here, so cargo
# compiles deps (incl. tonic/prost) without running the proto codegen.
COPY Cargo.toml ./
RUN mkdir src && \
    echo "fn main() {}" > src/main.rs && \
    cargo build --release 2>/dev/null || true && \
    rm -rf src

# Build the real binary (build.rs compiles proto/registry.proto via protoc)
COPY build.rs ./
COPY proto ./proto
COPY src ./src
COPY config.yaml ./
RUN touch src/main.rs && cargo build --release

# Stage 2: Minimal runtime image
FROM debian:bookworm-slim

ARG GIT_SHA=unknown
ARG BUILD_TIME=unknown
ARG GIT_DIRTY=unknown
# ENV for the version args is set near the END of the stage (after a RUN that
# consumes them) so a changed SHA/BUILD_TIME busts the ENV cache.

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
# Shared version-stamp dir 0777 (volume inits world-writable for non-root UIDs).
# The printf consumes the version args so a changed SHA/BUILD_TIME busts the ENV below.
RUN mkdir -p /versions && chmod 0777 /versions && \
    printf 'git_sha=%s build_time=%s dirty=%s\n' "$GIT_SHA" "$BUILD_TIME" "$GIT_DIRTY" > /etc/image-version
ENV GIT_SHA=$GIT_SHA BUILD_TIME=$BUILD_TIME GIT_DIRTY=$GIT_DIRTY

EXPOSE 80 443

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD curl -f http://localhost:80/healthz || exit 1

# Entrypoint runs as root to provision/own TLS certs on the (possibly bind-mounted)
# certs volume, then drops privileges to the unprivileged `gateway` user via setpriv.
ENTRYPOINT ["/docker-entrypoint.sh"]
