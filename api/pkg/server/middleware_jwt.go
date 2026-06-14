package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"go-stock-prediction/pkg/config"
	authpb "go-stock-prediction/proto/auth"
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

func getClaims(r *http.Request) jwt.MapClaims {
	claims, _ := r.Context().Value(claimsKey).(jwt.MapClaims)
	return claims
}

// getAccessibleMarkets extracts the accessible_markets list from JWT claims.
func getAccessibleMarkets(claims jwt.MapClaims) []string {
	if claims == nil {
		return nil
	}
	raw, ok := claims["accessible_markets"]
	if !ok {
		return nil
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	markets := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			markets = append(markets, s)
		}
	}
	return markets
}

// isSuperAdmin returns true if the JWT role claim is "super_admin".
func isSuperAdmin(claims jwt.MapClaims) bool {
	if claims == nil {
		return false
	}
	role, _ := claims["role"].(string)
	return role == "super_admin"
}

// callerFromClaims extracts caller_id and caller_role from JWT claims into a CallerMeta proto.
func callerFromClaims(claims jwt.MapClaims) *authpb.CallerMeta {
	if claims == nil {
		return &authpb.CallerMeta{}
	}
	idFloat, _ := claims["user_id"].(float64)
	role, _ := claims["role"].(string)
	return &authpb.CallerMeta{CallerId: int64(idFloat), CallerRole: role}
}

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

func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	claims := getClaims(r)
	if claims == nil {
		ResponseError(w, http.StatusUnauthorized, "authentication required")
		return false
	}
	role, _ := claims["role"].(string)
	if role != "admin" && role != "super_admin" {
		ResponseError(w, http.StatusForbidden, "admin access required")
		return false
	}
	return true
}

// AdminRequired wraps a handler requiring admin or super_admin role.
func AdminRequired(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireAdmin(w, r) {
			return
		}
		next(w, r)
	}
}

// MarketRequired wraps a handler requiring the caller to have access to a specific market.
// super_admin bypasses the check.
func MarketRequired(marketKey string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			claims := getClaims(r)
			if claims == nil {
				ResponseError(w, http.StatusUnauthorized, "authentication required")
				return
			}
			if isSuperAdmin(claims) {
				next(w, r)
				return
			}
			for _, m := range getAccessibleMarkets(claims) {
				if m == marketKey {
					next(w, r)
					return
				}
			}
			ResponseError(w, http.StatusForbidden, "no access to market: "+marketKey)
		}
	}
}
