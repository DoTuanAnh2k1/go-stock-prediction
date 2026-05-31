package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"go-stock-prediction/pkg/config"
)

type contextKey string

const claimsKey contextKey = "jwt_claims"

// JWTMiddleware parses the Bearer token and injects claims into context.
// Non-blocking — requests without a valid token continue as unauthenticated.
func JWTMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			cfg := config.GetServerConfig()
			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(cfg.JWTSecret), nil
			})
			if err == nil && token.Valid {
				if claims, ok := token.Claims.(jwt.MapClaims); ok {
					r = r.WithContext(context.WithValue(r.Context(), claimsKey, claims))
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// getClaims extracts JWT claims from context (nil if unauthenticated).
func getClaims(r *http.Request) jwt.MapClaims {
	claims, _ := r.Context().Value(claimsKey).(jwt.MapClaims)
	return claims
}

// requireAuth returns false and writes 401 if no valid JWT claims in context.
func requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if getClaims(r) == nil {
		ResponseError(w, http.StatusUnauthorized, "authentication required")
		return false
	}
	return true
}

// AuthRequired wraps a handler requiring JWT authentication.
func AuthRequired(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireAuth(w, r) {
			return
		}
		next(w, r)
	}
}

// requireAdmin returns false and writes 403 if user is not admin.
func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	claims := getClaims(r)
	if claims == nil {
		ResponseError(w, http.StatusUnauthorized, "authentication required")
		return false
	}
	role, _ := claims["role"].(string)
	if role != "admin" {
		ResponseError(w, http.StatusForbidden, "admin access required")
		return false
	}
	return true
}
