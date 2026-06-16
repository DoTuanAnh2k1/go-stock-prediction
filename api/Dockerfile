# Build stage
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git make
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o api-server ./cmd

# Run stage
FROM alpine:latest
RUN apk add --no-cache ca-certificates wget tzdata mysql-client
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /app/api-server .
# Backup directory — owned by appuser so a fresh backup_data volume inherits
# write permission for the scheduled/manual database backup.
RUN mkdir -p /backups && chown appuser:appgroup /backups
USER appuser
EXPOSE 8118
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s CMD wget -qO- http://localhost:8118/health/simple || exit 1
ENTRYPOINT ["./api-server"]
