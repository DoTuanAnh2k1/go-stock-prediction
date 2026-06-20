package shell

import (
	"testing"

	"go-stock-prediction/cli-svc/internal/client"
)

func cmds(keys ...string) []client.AllowedCommand {
	out := make([]client.AllowedCommand, 0, len(keys))
	for i, k := range keys {
		out = append(out, client.AllowedCommand{ID: int64(i + 1), Name: k, HandlerKey: k})
	}
	return out
}

func TestAllowedSet(t *testing.T) {
	tests := []struct {
		name    string
		role    string
		granted []client.AllowedCommand
		key     string
		want    bool
	}{
		{"super_admin bypass unlisted", "super_admin", nil, "user.delete", true},
		{"super_admin bypass anything", "super_admin", nil, "market.latest", true},
		{"user in union", "user", cmds("market.latest", "market.prices"), "market.prices", true},
		{"user not in union", "user", cmds("market.latest"), "trigger.train", false},
		{"empty allowed denies all (user)", "user", nil, "market.latest", false},
		{"empty allowed denies all (admin)", "admin", nil, "trigger.backup", false},
		{"admin granted via group", "admin", cmds("trigger.backup"), "trigger.backup", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewAllowedSet(tt.role, tt.granted)
			if got := a.Allows(tt.key); got != tt.want {
				t.Errorf("Allows(%s) for role=%s = %v want %v", tt.key, tt.role, got, tt.want)
			}
		})
	}
}

func TestUnionAcrossGroups(t *testing.T) {
	// Simulate /me/commands returning the union of two groups.
	granted := cmds("market.latest", "market.prices", "trigger.train")
	a := NewAllowedSet("user", granted)
	for _, k := range []string{"market.latest", "market.prices", "trigger.train"} {
		if !a.Allows(k) {
			t.Errorf("expected %s allowed", k)
		}
	}
	if a.Allows("user.delete") {
		t.Error("user.delete must not be allowed")
	}
}
