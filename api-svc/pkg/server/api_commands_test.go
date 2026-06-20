package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	authpb "go-stock-prediction/proto/auth"
)

// withClaims returns a request carrying the given JWT MapClaims in context,
// mirroring what JWTMiddleware injects for a valid token.
func withClaims(req *http.Request, claims jwt.MapClaims) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), claimsKey, claims))
}

func adminClaims() jwt.MapClaims {
	return jwt.MapClaims{"user_id": float64(1), "role": "admin", "sub": "admin"}
}

func userClaims() jwt.MapClaims {
	return jwt.MapClaims{"user_id": float64(42), "role": "user", "sub": "bob"}
}

// callHandler runs a handler with panic recovery (no DB / gRPC singleton in tests).
func callHandler(h http.HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	func() {
		defer func() { recover() }()
		h(w, req)
	}()
	return w
}

// ─── DTO JSON shape tests ────────────────────────────────────────────────────

func TestCommandDTO_JSONFields(t *testing.T) {
	dto := commandToDTO(&authpb.Command{
		Id: 1, Name: "gold latest", Description: "show gold",
		HandlerKey: "market.latest", Args: `{"market":"gold"}`, Enabled: true,
		CreatedAt: "2026-06-20T00:00:00", UpdatedAt: "2026-06-20T00:00:00",
	})
	b, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, f := range []string{"id", "name", "description", "handler_key", "args", "enabled", "created_at", "updated_at"} {
		if _, ok := raw[f]; !ok {
			t.Errorf("commandDTO JSON missing field %q", f)
		}
	}
	// args must be a JSON object, not a quoted string.
	var argsObj map[string]interface{}
	if err := json.Unmarshal(raw["args"], &argsObj); err != nil {
		t.Errorf("args is not a JSON object: %s", raw["args"])
	}
	if argsObj["market"] != "gold" {
		t.Errorf("args.market = %v, want gold", argsObj["market"])
	}
}

func TestCommandDTO_EmptyArgsDefaultsToObject(t *testing.T) {
	dto := commandToDTO(&authpb.Command{Id: 2, Name: "x", Args: ""})
	b, _ := json.Marshal(dto)
	var raw map[string]json.RawMessage
	json.Unmarshal(b, &raw) //nolint:errcheck
	if string(raw["args"]) != "{}" {
		t.Errorf("empty args should default to {}, got %s", raw["args"])
	}
}

func TestHandlerDTO_JSONFields(t *testing.T) {
	dto := handlerToDTO(&authpb.CliHandler{
		HandlerKey: "market.latest", DisplayName: "Market latest",
		Verb: "get", Resource: "market", ArgSchema: `[{"name":"market"}]`, Enabled: true,
	})
	b, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, f := range []string{"handler_key", "display_name", "verb", "resource", "arg_schema", "enabled"} {
		if _, ok := raw[f]; !ok {
			t.Errorf("handlerDTO JSON missing field %q", f)
		}
	}
	// arg_schema must be a JSON array.
	var arr []interface{}
	if err := json.Unmarshal(raw["arg_schema"], &arr); err != nil {
		t.Errorf("arg_schema is not a JSON array: %s", raw["arg_schema"])
	}
}

func TestHandlerDTO_EmptyArgSchemaDefaultsToArray(t *testing.T) {
	dto := handlerToDTO(&authpb.CliHandler{HandlerKey: "x", ArgSchema: ""})
	b, _ := json.Marshal(dto)
	var raw map[string]json.RawMessage
	json.Unmarshal(b, &raw) //nolint:errcheck
	if string(raw["arg_schema"]) != "[]" {
		t.Errorf("empty arg_schema should default to [], got %s", raw["arg_schema"])
	}
}

func TestCommandGroupDTO_JSONFields(t *testing.T) {
	dto := commandGroupToDTO(&authpb.CommandGroupResponse{
		Id: 1, Name: "ops", Description: "ops cmds", CommandIds: []int64{1, 2},
		CreatedAt: "t", UpdatedAt: "t",
	})
	b, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, f := range []string{"id", "name", "description", "command_ids", "created_at", "updated_at"} {
		if _, ok := raw[f]; !ok {
			t.Errorf("commandGroupDTO JSON missing field %q", f)
		}
	}
}

func TestCommandGroupDTO_NilCommandIDsIsEmptyArray(t *testing.T) {
	dto := commandGroupToDTO(&authpb.CommandGroupResponse{Id: 1, Name: "g", CommandIds: nil})
	b, _ := json.Marshal(dto)
	var raw map[string]json.RawMessage
	json.Unmarshal(b, &raw) //nolint:errcheck
	if string(raw["command_ids"]) != "[]" {
		t.Errorf("command_ids must be [] not null when empty, got %s", raw["command_ids"])
	}
}

// ─── args <-> string boundary helpers ────────────────────────────────────────

func TestArgsToString_Default(t *testing.T) {
	if got := argsToString(nil, "{}"); got != "{}" {
		t.Errorf("argsToString(nil) = %q, want {}", got)
	}
	if got := argsToString(json.RawMessage(`{"a":1}`), "{}"); got != `{"a":1}` {
		t.Errorf("argsToString = %q", got)
	}
}

// ─── Auth gating tests ───────────────────────────────────────────────────────

func TestListCommands_RequiresAuth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/commands", nil)
	w := callHandler(ListCommandsHandler, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without JWT, got %d", w.Code)
	}
}

func TestListCommands_RejectsNonAdmin(t *testing.T) {
	req := withClaims(httptest.NewRequest(http.MethodGet, "/api/commands", nil), userClaims())
	w := callHandler(ListCommandsHandler, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for role=user, got %d", w.Code)
	}
}

func TestCreateCommand_RejectsNonAdmin(t *testing.T) {
	body, _ := json.Marshal(createCommandRequest{Name: "x", HandlerKey: "h"})
	req := withClaims(httptest.NewRequest(http.MethodPost, "/api/commands", bytes.NewReader(body)), userClaims())
	w := callHandler(CreateCommandHandler, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for role=user, got %d", w.Code)
	}
}

func TestCreateCommand_BadBody(t *testing.T) {
	req := withClaims(httptest.NewRequest(http.MethodPost, "/api/commands", bytes.NewReader([]byte("{not json"))), adminClaims())
	w := callHandler(CreateCommandHandler, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for malformed body, got %d", w.Code)
	}
}

func TestUpdateCommand_InvalidID(t *testing.T) {
	body, _ := json.Marshal(createCommandRequest{Name: "x"})
	req := withClaims(httptest.NewRequest(http.MethodPut, "/api/commands/abc", bytes.NewReader(body)), adminClaims())
	req.SetPathValue("id", "abc")
	w := callHandler(UpdateCommandHandler, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-numeric id, got %d", w.Code)
	}
}

func TestDeleteCommand_InvalidID(t *testing.T) {
	req := withClaims(httptest.NewRequest(http.MethodDelete, "/api/commands/xyz", nil), adminClaims())
	req.SetPathValue("id", "xyz")
	w := callHandler(DeleteCommandHandler, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-numeric id, got %d", w.Code)
	}
}

func TestListCommandHandlers_RejectsNonAdmin(t *testing.T) {
	req := withClaims(httptest.NewRequest(http.MethodGet, "/api/command-handlers", nil), userClaims())
	w := callHandler(ListCommandHandlersHandler, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for role=user, got %d", w.Code)
	}
}

// Admin requests with no gRPC singleton should reach requireAuthClient and get 503.
func TestListCommands_AdminNoAuthClient(t *testing.T) {
	req := withClaims(httptest.NewRequest(http.MethodGet, "/api/commands", nil), adminClaims())
	w := callHandler(ListCommandsHandler, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 (no auth client), got %d", w.Code)
	}
}

// ─── /api/me/commands ────────────────────────────────────────────────────────

func TestGetMyCommands_RequiresAuth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/me/commands", nil)
	w := callHandler(GetMyCommandsHandler, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without JWT, got %d", w.Code)
	}
}

func TestGetMyCommands_AuthedNoAuthClient(t *testing.T) {
	req := withClaims(httptest.NewRequest(http.MethodGet, "/api/me/commands", nil), userClaims())
	w := callHandler(GetMyCommandsHandler, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 (no auth client), got %d", w.Code)
	}
}

// ─── internal upsert ─────────────────────────────────────────────────────────

func TestUpsertHandlers_RejectsWhenSecretMismatch(t *testing.T) {
	body := []byte(`{"secret":"wrong","handlers":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/command-handlers/upsert", bytes.NewReader(body))
	w := callHandler(UpsertCommandHandlersHandler, req)
	// With no INTERNAL_SECRET configured (empty), any request is rejected 401.
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for secret mismatch, got %d", w.Code)
	}
}

func TestUpsertHandlers_RejectsEmptySecretConfig(t *testing.T) {
	// Even a matching-looking empty secret must be rejected when config secret is "".
	body := []byte(`{"secret":"","handlers":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/command-handlers/upsert", bytes.NewReader(body))
	w := callHandler(UpsertCommandHandlersHandler, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when no internal secret configured, got %d", w.Code)
	}
}

func TestUpsertHandlers_BadBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/command-handlers/upsert", bytes.NewReader([]byte("{nope")))
	w := callHandler(UpsertCommandHandlersHandler, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for malformed body, got %d", w.Code)
	}
}

// ─── command groups gating ───────────────────────────────────────────────────

func TestListCommandGroups_RejectsNonAdmin(t *testing.T) {
	req := withClaims(httptest.NewRequest(http.MethodGet, "/api/command-groups", nil), userClaims())
	w := callHandler(ListCommandGroupsHandler, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestSetGroupCommands_BadBody(t *testing.T) {
	req := withClaims(httptest.NewRequest(http.MethodPut, "/api/command-groups/1/commands", bytes.NewReader([]byte("x"))), adminClaims())
	req.SetPathValue("id", "1")
	w := callHandler(SetGroupCommandsHandler, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for malformed body, got %d", w.Code)
	}
}

func TestSetGroupCommands_InvalidGroupID(t *testing.T) {
	body, _ := json.Marshal(setGroupCommandsRequest{CommandIDs: []int64{1, 2}})
	req := withClaims(httptest.NewRequest(http.MethodPut, "/api/command-groups/zzz/commands", bytes.NewReader(body)), adminClaims())
	req.SetPathValue("id", "zzz")
	w := callHandler(SetGroupCommandsHandler, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-numeric group id, got %d", w.Code)
	}
}

func TestRemoveUserFromCmdGroup_InvalidUID(t *testing.T) {
	req := withClaims(httptest.NewRequest(http.MethodDelete, "/api/command-groups/1/users/abc", nil), adminClaims())
	req.SetPathValue("id", "1")
	req.SetPathValue("uid", "abc")
	w := callHandler(RemoveUserFromCmdGroupHandler, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-numeric uid, got %d", w.Code)
	}
}

func TestCreateCommandGroup_AdminNoAuthClient(t *testing.T) {
	body, _ := json.Marshal(createGroupRequest{Name: "ops"})
	req := withClaims(httptest.NewRequest(http.MethodPost, "/api/command-groups", bytes.NewReader(body)), adminClaims())
	w := callHandler(CreateCommandGroupHandler, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 (no auth client), got %d", w.Code)
	}
}
