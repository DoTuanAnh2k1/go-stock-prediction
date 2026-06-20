package shell

import (
	"strings"
	"testing"

	"go-stock-prediction/cli-svc/internal/client"
	"go-stock-prediction/cli-svc/internal/handlers"
)

func contains(items []string, want string) bool {
	for _, it := range items {
		if it == want {
			return true
		}
	}
	return false
}

func TestCompleterVerbs(t *testing.T) {
	reg := handlers.NewRegistry()
	c := NewCompleter(reg, NewAllowedSet("super_admin", nil))

	got := c.Suggest("")
	for _, v := range []string{"get", "set", "update", "delete"} {
		if !contains(got, v) {
			t.Errorf("expected verb %q in %v", v, got)
		}
	}
	// Prefix filter.
	got = c.Suggest("ge")
	if !contains(got, "get") || contains(got, "set") {
		t.Errorf("prefix filter failed: %v", got)
	}
}

func TestCompleterResources(t *testing.T) {
	reg := handlers.NewRegistry()
	c := NewCompleter(reg, NewAllowedSet("super_admin", nil))

	got := c.Suggest("get ")
	if !contains(got, "market.latest") || !contains(got, "monitoring.overview") {
		t.Errorf("missing resources: %v", got)
	}
	// Should not include set-verb resources.
	if contains(got, "trigger.train") {
		t.Errorf("get suggestions leaked set resource: %v", got)
	}
}

func TestCompleterFiltersByPermission(t *testing.T) {
	reg := handlers.NewRegistry()
	allowed := NewAllowedSet("user", []client.AllowedCommand{{HandlerKey: "market.latest"}})
	c := NewCompleter(reg, allowed)

	got := c.Suggest("get ")
	if !contains(got, "market.latest") {
		t.Errorf("allowed resource missing: %v", got)
	}
	if contains(got, "users.list") {
		t.Errorf("disallowed resource leaked: %v", got)
	}
}

func TestCompleterArgChoices(t *testing.T) {
	reg := handlers.NewRegistry()
	c := NewCompleter(reg, NewAllowedSet("super_admin", nil))

	// Arg name suggestions.
	got := c.Suggest("get market.latest ")
	if !contains(got, "market=") {
		t.Errorf("expected market= suggestion: %v", got)
	}
	// Value choices.
	got = c.Suggest("get market.latest market=")
	if !contains(got, "market=gold") || !contains(got, "market=sp500") {
		t.Errorf("expected market value choices: %v", got)
	}
	// Value prefix filter.
	got = c.Suggest("get market.latest market=cr")
	if len(got) != 1 || !strings.HasSuffix(got[0], "crypto") {
		t.Errorf("expected only crypto: %v", got)
	}
}
