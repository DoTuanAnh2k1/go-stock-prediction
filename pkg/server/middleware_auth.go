package server

import (
	"go-stock-prediction/pkg/config"
	"net/http"
)

func APIKeyMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		expectedKey := config.GetServerConfig().APIKey
		if expectedKey == "" || apiKey != expectedKey {
			ResponseError(w, http.StatusUnauthorized, "Invalid or missing API key")
			return
		}
		next(w, r)
	}
}
