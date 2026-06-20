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
	c := NewCompleter(handlers.NewRegistry(), NewAllowedSet("super_admin", nil))
	got := c.Suggest("")
	for _, v := range []string{"get", "set", "update", "delete"} {
		if !contains(got, v) {
			t.Errorf("expected verb %q in %v", v, got)
		}
	}
	got = c.Suggest("ge")
	if !contains(got, "get") || contains(got, "set") {
		t.Errorf("prefix filter failed: %v", got)
	}
}

func TestCompleterCategoriesAndNames(t *testing.T) {
	c := NewCompleter(handlers.NewRegistry(), NewAllowedSet("super_admin", nil))

	cats := c.Suggest("get ")
	if !contains(cats, "market") || !contains(cats, "monitoring") {
		t.Errorf("missing categories: %v", cats)
	}
	if contains(cats, "trigger") { // trigger is a set category, not get
		t.Errorf("get categories leaked trigger: %v", cats)
	}

	names := c.Suggest("get market ")
	for _, n := range []string{"latest", "prices", "predictions"} {
		if !contains(names, n) {
			t.Errorf("missing name %q under 'get market': %v", n, names)
		}
	}
}

func TestCompleterFiltersByPermission(t *testing.T) {
	allowed := NewAllowedSet("user", []client.AllowedCommand{{HandlerKey: "market.latest"}})
	c := NewCompleter(handlers.NewRegistry(), allowed)

	if cats := c.Suggest("get "); !contains(cats, "market") {
		t.Errorf("allowed category missing: %v", cats)
	}
	if names := c.Suggest("get market "); !contains(names, "latest") || contains(names, "prices") {
		t.Errorf("permission leak in names: %v", names)
	}
	if got := c.Suggest("get users "); len(got) != 0 {
		t.Errorf("disallowed category should yield no names: %v", got)
	}
}

func TestCompleterArgNamesAndValues(t *testing.T) {
	c := NewCompleter(handlers.NewRegistry(), NewAllowedSet("super_admin", nil))

	if got := c.Suggest("get market latest "); !contains(got, "market") {
		t.Errorf("expected 'market' arg name: %v", got)
	}
	got := c.Suggest("get market latest market ")
	if !contains(got, "gold") || !contains(got, "sp500") {
		t.Errorf("expected market value choices: %v", got)
	}
	got = c.Suggest("get market latest market cr")
	if len(got) != 1 || !strings.HasSuffix(got[0], "crypto") {
		t.Errorf("expected only crypto: %v", got)
	}
}
