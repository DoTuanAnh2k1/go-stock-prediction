package handlers

import (
	"context"
	"net/url"
	"testing"

	"go-stock-prediction/cli-svc/internal/client"
)

// mockClient records the last request and returns canned responses.
type mockClient struct {
	lastMethod string
	lastPath   string
	lastQuery  url.Values
	lastBody   any
	resp       any
	status     int
}

func (m *mockClient) Login(context.Context, string, string) (*client.LoginResult, error) {
	return &client.LoginResult{Token: "t", Role: "user"}, nil
}
func (m *mockClient) GetMyCommands(context.Context, string) ([]client.AllowedCommand, error) {
	return nil, nil
}
func (m *mockClient) Do(_ context.Context, _, method, path string, q url.Values, body any) (any, int, error) {
	m.lastMethod, m.lastPath, m.lastQuery, m.lastBody = method, path, q, body
	st := m.status
	if st == 0 {
		st = 200
	}
	return m.resp, st, nil
}
func (m *mockClient) PostInternal(context.Context, string, any) (int, error) { return 200, nil }

func TestResolve(t *testing.T) {
	reg := NewRegistry()
	tests := []struct {
		verb, resource string
		wantKey        string
		wantOK         bool
	}{
		{"get", "market.latest", "market.latest", true},
		{"get", "monitoring.overview", "monitoring.overview", true},
		{"set", "trigger.train", "trigger.train", true},
		{"update", "schedule.update", "schedule.update", true},
		{"delete", "user.delete", "user.delete", true},
		{"get", "trigger.train", "", false}, // wrong verb
		{"get", "nope", "", false},
	}
	for _, tt := range tests {
		h, ok := reg.Resolve(tt.verb, tt.resource)
		if ok != tt.wantOK {
			t.Errorf("Resolve(%s,%s) ok=%v want %v", tt.verb, tt.resource, ok, tt.wantOK)
			continue
		}
		if ok && h.Key != tt.wantKey {
			t.Errorf("Resolve(%s,%s) key=%s want %s", tt.verb, tt.resource, h.Key, tt.wantKey)
		}
	}
}

func TestValidate(t *testing.T) {
	reg := NewRegistry()
	tests := []struct {
		name    string
		key     string
		args    map[string]string
		wantErr bool
	}{
		{"market ok", "market.latest", map[string]string{"market": "gold"}, false},
		{"market missing required", "market.latest", map[string]string{}, true},
		{"market bad choice", "market.latest", map[string]string{"market": "tesla"}, true},
		{"prices limit int ok", "market.prices", map[string]string{"market": "nasdaq", "limit": "10"}, false},
		{"prices limit not int", "market.prices", map[string]string{"market": "nasdaq", "limit": "ten"}, true},
		{"prices optional omitted", "market.prices", map[string]string{"market": "crypto"}, false},
		{"unknown arg", "market.latest", map[string]string{"market": "gold", "foo": "bar"}, true},
		{"train optional algo", "trigger.train", map[string]string{}, false},
		{"schedule enabled choice ok", "schedule.update", map[string]string{"key": "k", "cron_expression": "* * * * *", "enabled": "true"}, false},
		{"schedule enabled bad choice", "schedule.update", map[string]string{"key": "k", "cron_expression": "x", "enabled": "maybe"}, true},
		{"user id int", "user.delete", map[string]string{"id": "5"}, false},
		{"user id not int", "user.delete", map[string]string{"id": "abc"}, true},
		{"user.update role choice", "user.update", map[string]string{"id": "1", "role": "ceo"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, ok := reg.ByKey(tt.key)
			if !ok {
				t.Fatalf("handler %s missing", tt.key)
			}
			_, err := h.Validate(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate err=%v wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestExecuteBuildsRequest(t *testing.T) {
	reg := NewRegistry()
	mc := &mockClient{resp: map[string]any{"ok": true}}

	tests := []struct {
		key        string
		args       map[string]string
		wantMethod string
		wantPath   string
	}{
		{"market.latest", map[string]string{"market": "gold"}, "GET", "/gold/latest"},
		{"market.predictions", map[string]string{"market": "sp500"}, "GET", "/sp500/predictions/latest"},
		{"trigger.crawler", map[string]string{"market": "crypto"}, "POST", "/trigger/crypto-crawler"},
		{"trigger.predict", map[string]string{"market": "nasdaq"}, "POST", "/trigger/nasdaq-predict"},
		{"trigger.reconcile", map[string]string{}, "POST", "/trigger/reconcile"},
		{"schedule.update", map[string]string{"key": "crawler_gold", "cron_expression": "* * * * *"}, "PUT", "/schedules/crawler_gold"},
		{"user.delete", map[string]string{"id": "9"}, "DELETE", "/users/9"},
		{"backup.delete", map[string]string{"filename": "x.sql.gz"}, "DELETE", "/backups/x.sql.gz"},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			h, _ := reg.ByKey(tt.key)
			clean, err := h.Validate(tt.args)
			if err != nil {
				t.Fatalf("validate: %v", err)
			}
			if _, _, err := h.Execute(context.Background(), mc, "jwt", clean); err != nil {
				t.Fatalf("execute: %v", err)
			}
			if mc.lastMethod != tt.wantMethod {
				t.Errorf("method=%s want %s", mc.lastMethod, tt.wantMethod)
			}
			if mc.lastPath != tt.wantPath {
				t.Errorf("path=%s want %s", mc.lastPath, tt.wantPath)
			}
		})
	}
}

func TestPricesLimitQuery(t *testing.T) {
	reg := NewRegistry()
	mc := &mockClient{}
	h, _ := reg.ByKey("market.prices")
	clean, _ := h.Validate(map[string]string{"market": "gold", "limit": "25"})
	if _, _, err := h.Execute(context.Background(), mc, "jwt", clean); err != nil {
		t.Fatal(err)
	}
	if got := mc.lastQuery.Get("limit"); got != "25" {
		t.Errorf("limit query=%q want 25", got)
	}
}

func TestUpsertPayload(t *testing.T) {
	reg := NewRegistry()
	p := reg.UpsertPayload("sekret")
	if p.Secret != "sekret" {
		t.Errorf("secret=%q", p.Secret)
	}
	if len(p.Handlers) != len(reg.All()) {
		t.Errorf("handlers=%d want %d", len(p.Handlers), len(reg.All()))
	}
	// Spot check a known handler is present with its schema.
	found := false
	for _, h := range p.Handlers {
		if h.HandlerKey == "market.prices" {
			found = true
			if len(h.ArgSchema) != 2 {
				t.Errorf("market.prices arg schema len=%d want 2", len(h.ArgSchema))
			}
		}
	}
	if !found {
		t.Error("market.prices not in upsert payload")
	}
}
