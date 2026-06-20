package client

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

// decodeJWTClaims pulls role and user_id out of a JWT payload without verifying
// the signature (cli-svc trusts the token it just received from the API).
func decodeJWTClaims(token string) (role string, userID int64) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", 0
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Some encoders use standard base64 padding.
		payload, err = base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return "", 0
		}
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", 0
	}
	if r, ok := claims["role"].(string); ok {
		role = r
	}
	if uid, ok := claims["user_id"].(float64); ok {
		userID = int64(uid)
	}
	return role, userID
}
