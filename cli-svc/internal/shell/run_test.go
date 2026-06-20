package shell

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"go-stock-prediction/cli-svc/internal/client"
	"go-stock-prediction/cli-svc/internal/handlers"
)

type stubClient struct {
	resp   any
	status int
}

func (s *stubClient) Login(context.Context, string, string) (*client.LoginResult, error) {
	return &client.LoginResult{Token: "t"}, nil
}
func (s *stubClient) GetMyCommands(context.Context, string) ([]client.AllowedCommand, error) {
	return nil, nil
}
func (s *stubClient) Do(context.Context, string, string, string, url.Values, any) (any, int, error) {
	st := s.status
	if st == 0 {
		st = 200
	}
	return s.resp, st, nil
}
func (s *stubClient) PostInternal(context.Context, string, any) (int, error) { return 200, nil }

func TestRunner(t *testing.T) {
	reg := handlers.NewRegistry()

	t.Run("permission denied for non-allowed", func(t *testing.T) {
		allowed := NewAllowedSet("user", nil)
		r := NewRunner(reg, &stubClient{}, allowed, "jwt")
		out, ok := r.Run(context.Background(), "get market.latest market=gold")
		if ok {
			t.Fatal("expected denial")
		}
		if !strings.Contains(out, "permission denied") {
			t.Errorf("got %q", out)
		}
	})

	t.Run("super_admin runs anything and renders table", func(t *testing.T) {
		resp := map[string]any{"data": []any{
			map[string]any{"algorithm": "lstm_nn", "total": 10.0, "correct": 7.0},
		}}
		r := NewRunner(reg, &stubClient{resp: resp}, NewAllowedSet("super_admin", nil), "jwt")
		out, ok := r.Run(context.Background(), "get direction.accuracy market=GOLD")
		if !ok {
			t.Fatalf("expected success, got %q", out)
		}
		if !strings.Contains(out, "algorithm") || !strings.Contains(out, "lstm_nn") {
			t.Errorf("table missing expected cells: %q", out)
		}
	})

	t.Run("allowed user runs granted command", func(t *testing.T) {
		allowed := NewAllowedSet("user", []client.AllowedCommand{{HandlerKey: "market.latest"}})
		resp := map[string]any{"data": map[string]any{"price": 2500.0, "source": "SJC"}}
		r := NewRunner(reg, &stubClient{resp: resp}, allowed, "jwt")
		out, ok := r.Run(context.Background(), "get market.latest market=gold")
		if !ok {
			t.Fatalf("expected success: %q", out)
		}
		if !strings.Contains(out, "price") {
			t.Errorf("expected price in output: %q", out)
		}
	})

	t.Run("unknown command", func(t *testing.T) {
		r := NewRunner(reg, &stubClient{}, NewAllowedSet("super_admin", nil), "jwt")
		out, ok := r.Run(context.Background(), "get bogus.thing")
		if ok || !strings.Contains(out, "unknown command") {
			t.Errorf("got ok=%v out=%q", ok, out)
		}
	})

	t.Run("validation error surfaces", func(t *testing.T) {
		r := NewRunner(reg, &stubClient{}, NewAllowedSet("super_admin", nil), "jwt")
		out, ok := r.Run(context.Background(), "get market.latest")
		if ok || !strings.Contains(out, "required") {
			t.Errorf("got ok=%v out=%q", ok, out)
		}
	})

	t.Run("API error status", func(t *testing.T) {
		r := NewRunner(reg, &stubClient{status: 403, resp: map[string]any{"error": "forbidden"}}, NewAllowedSet("super_admin", nil), "jwt")
		out, ok := r.Run(context.Background(), "get users.list")
		if ok || !strings.Contains(out, "403") || !strings.Contains(out, "forbidden") {
			t.Errorf("got ok=%v out=%q", ok, out)
		}
	})
}
