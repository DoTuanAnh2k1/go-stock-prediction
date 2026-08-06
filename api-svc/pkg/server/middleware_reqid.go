package server

import (
	"net/http"

	"go-stock-prediction/pkg/reqid"
)

// RequestIDMiddleware is the outermost middleware in the chain.
// It reads X-Request-ID from the inbound request; if absent, it mints a new
// UUID v4. The ID is stored in the request context via reqid.WithRequestID and
// echoed back to the caller as X-Request-ID on the response.
//
// Placing this before JWTMiddleware and AccessLogMiddleware ensures every
// downstream handler and log line can access the correlation ID via
// logger.Ctx(r.Context()).
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(reqid.Header)
		if id == "" {
			id = reqid.New()
		}

		ctx := reqid.WithRequestID(r.Context(), id)
		w.Header().Set(reqid.Header, id)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
