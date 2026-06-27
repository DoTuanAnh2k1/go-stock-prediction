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
ENV GIT_SHA=$GIT_SHA BUILD_TIME=$BUILD_TIME GIT_DIRTY=$GIT_DIRTY
USER appuser
EXPOSE 8118
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s CMD wget -qO- http://localhost:8118/health/simple || exit 1
ENTRYPOINT ["./api-server"]
