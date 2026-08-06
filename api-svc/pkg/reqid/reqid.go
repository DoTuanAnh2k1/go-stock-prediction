// Package reqid manages correlation IDs that flow end-to-end through the
// api-svc request path.
//
// Contract (identical across all 6 services):
//   - Inbound HTTP header:       X-Request-ID
//   - Outbound gRPC metadata key: x-request-id  (lowercase — gRPC requirement)
//   - Log field name:            request_id
//   - Value format:              UUID v4
package reqid

import (
	"context"
	"crypto/rand"
	"fmt"
)

// Header is the canonical HTTP header name for the correlation ID.
const Header = "X-Request-ID"

// MetaKey is the gRPC metadata key for the correlation ID.
// Must be lowercase — gRPC enforces lowercase metadata keys.
const MetaKey = "x-request-id"

// contextKey is an unexported type used as a context key to avoid collisions.
type contextKey struct{}

// WithRequestID returns a new context carrying the given request ID.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

// FromContext extracts the request ID from ctx.
// Returns ("", false) when no ID is present.
func FromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(contextKey{}).(string)
	if !ok || id == "" {
		return "", false
	}
	return id, true
}

// New generates a new UUID v4 using crypto/rand.
// github.com/google/uuid is already an indirect dependency (pulled in by OTel),
// but we implement directly here to keep this package import-free of pkg/logger
// and avoid cycles.
func New() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Extremely unlikely; fall back to a zeroed UUID rather than panicking.
		return "00000000-0000-4000-8000-000000000000"
	}
	// Set version 4 and variant bits (RFC 4122).
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
