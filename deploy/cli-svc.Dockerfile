# cli-svc — interactive SSH server (wish + bubbletea).
# Build context is ../cli-svc (set by docker-compose), so paths are relative to
# the cli-svc/ directory.

# ---- builder ----
FROM golang:1.26 AS builder

WORKDIR /src

# Cache dependencies first.
COPY go.mod go.sum ./
RUN go mod download

# Build the static binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/cli-svc .

# ---- runtime ----
FROM alpine:3.20

RUN adduser -D -u 10001 cli && \
    mkdir -p /etc/cli-svc/keys && \
    chown -R cli:cli /etc/cli-svc

COPY --from=builder /out/cli-svc /usr/local/bin/cli-svc

USER cli

ENV SSH_LISTEN_ADDR=:2345 \
    SSH_HOST_KEY_PATH=/etc/cli-svc/keys/host_key

EXPOSE 2345

ENTRYPOINT ["/usr/local/bin/cli-svc"]
