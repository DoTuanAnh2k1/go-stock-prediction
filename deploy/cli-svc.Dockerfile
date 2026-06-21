# cli-svc — interactive SSH server (wish + bubbletea).
# Build context is the repo root (set by docker-compose) so the sibling
# service-mgt module (referenced via `replace ... => ../service-mgt`) is present.

# ---- builder ----
FROM golang:1.26 AS builder

WORKDIR /src

# Sibling module first (needed by the replace directive), then cli-svc.
COPY service-mgt/ ./service-mgt/
COPY cli-svc/ ./cli-svc/

WORKDIR /src/cli-svc
RUN go mod download

# Build the static binary.
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
