# Build stage
# Build context is the repo root so the sibling service-mgt module (referenced
# via `replace go-stock-prediction/service-mgt => ../service-mgt`) is available.
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git make
WORKDIR /src
COPY service-mgt/ ./service-mgt/
COPY api-svc/ ./api-svc/
WORKDIR /src/api-svc
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /api-server ./cmd

# Run stage
FROM alpine:latest
RUN apk add --no-cache ca-certificates wget tzdata postgresql-client
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /api-server .
# Backup directory — owned by appuser so a fresh backup_data volume inherits
# write permission for the scheduled/manual database backup.
RUN mkdir -p /backups && chown appuser:appgroup /backups
ARG GIT_SHA=unknown
ARG BUILD_TIME=unknown
ARG GIT_DIRTY=unknown
# Consume the version args in a RUN so a changed GIT_SHA/BUILD_TIME busts the
# cache for the ENV below — a bare ENV layer is keyed on the literal instruction
# string and would otherwise freeze the first build's values. Also seeds the
# shared version-stamp dir 0777 (volume inits world-writable for non-root UIDs).
RUN mkdir -p /versions && chmod 0777 /versions && \
    printf 'git_sha=%s build_time=%s dirty=%s\n' "$GIT_SHA" "$BUILD_TIME" "$GIT_DIRTY" > /etc/image-version
ENV GIT_SHA=$GIT_SHA BUILD_TIME=$BUILD_TIME GIT_DIRTY=$GIT_DIRTY
USER appuser
EXPOSE 8118
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s CMD wget -qO- http://localhost:8118/health/simple || exit 1
ENTRYPOINT ["./api-server"]
