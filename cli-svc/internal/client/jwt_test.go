package client

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func makeJWT(claims map[string]any) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`))
	body, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(body)
	return header + "." + payload + ".sig"
}

func TestDecodeJWTClaims(t *testing.T) {
	tok := makeJWT(map[string]any{"role": "admin", "user_id": float64(42), "sub": "bob"})
	role, uid := decodeJWTClaims(tok)
	if role != "admin" {
		t.Errorf("role=%q want admin", role)
	}
	if uid != 42 {
		t.Errorf("uid=%d want 42", uid)
	}
}

func TestDecodeJWTBad(t *testing.T) {
	role, uid := decodeJWTClaims("not-a-jwt")
	if role != "" || uid != 0 {
		t.Errorf("expected empty claims, got role=%q uid=%d", role, uid)
	}
}

func TestUnwrapHelpers(t *testing.T) {
	obj := unwrapObject(map[string]any{"data": map[string]any{"token": "x"}})
	if obj["token"] != "x" {
		t.Errorf("unwrapObject failed: %v", obj)
	}
	arr := unwrapArray(map[string]any{"data": []any{1.0, 2.0}})
	if len(arr) != 2 {
		t.Errorf("unwrapArray failed: %v", arr)
	}
	arr2 := unwrapArray([]any{1.0})
	if len(arr2) != 1 {
		t.Errorf("unwrapArray bare failed: %v", arr2)
	}
}
