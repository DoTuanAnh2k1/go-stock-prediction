# Build stage
FROM golang:1.23-alpine AS builder
RUN apk add --no-cache git make
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o go-stock-prediction ./cmd/app

# Run stage
FROM alpine:latest
RUN apk add --no-cache ca-certificates wget
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /app/go-stock-prediction .
COPY --from=builder /app/web ./web
USER appuser
EXPOSE 31300
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s CMD wget -qO- http://localhost:31300/health/simple || exit 1
ENTRYPOINT ["./go-stock-prediction"]
