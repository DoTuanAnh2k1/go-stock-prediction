package client

import (
	"context"

	"github.com/google/uuid"
)

// HeaderRequestID is the correlation-ID header carried on every outbound HTTP
// request to the API. The value is a fresh UUID v4. The same name is used by
// all services in the stack so a single request can be traced end-to-end; the
// corresponding log field is "request_id".
const HeaderRequestID = "X-Request-ID"

// reqIDKey is the private context key under which a per-command request id is
// stored. Using a dedicated unexported type avoids collisions with other
// packages' context values.
type reqIDKey struct{}

// WithRequestID returns a child context carrying id as the correlation id for
// all outbound HTTP calls made under it. Runner.Run mints one id per resolved
// CLI command and threads it through here so every HTTP call that command makes
// shares the same id.
func WithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, reqIDKey{}, id)
}

// NewRequestID returns a fresh UUID v4 string.
func NewRequestID() string {
	return uuid.NewString()
}

// requestIDFor returns the correlation id to attach to an outbound request: the
// one threaded through ctx (shared across a command's calls) if present,
// otherwise a freshly minted per-request id.
func requestIDFor(ctx context.Context) string {
	if ctx != nil {
		if id, ok := ctx.Value(reqIDKey{}).(string); ok && id != "" {
			return id
		}
	}
	return NewRequestID()
}
