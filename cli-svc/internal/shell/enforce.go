package shell

import (
	"go-stock-prediction/cli-svc/internal/client"
)

// AllowedSet captures the per-session permission state derived from the user's
// role and their allowed command list (GET /me/commands).
type AllowedSet struct {
	Role     string
	commands []client.AllowedCommand
	// keys is the set of handler_keys the user may run (any args).
	keys map[string]bool
}

// NewAllowedSet builds an allowed-set from the role + command list.
func NewAllowedSet(role string, cmds []client.AllowedCommand) *AllowedSet {
	keys := map[string]bool{}
	for _, c := range cmds {
		keys[c.HandlerKey] = true
	}
	return &AllowedSet{Role: role, commands: cmds, keys: keys}
}

// IsSuperAdmin reports whether the role bypasses command checks.
func (a *AllowedSet) IsSuperAdmin() bool {
	return a.Role == "super_admin"
}

// Allows reports whether the user may run the given handler_key.
// super_admin bypasses (may run any catalog handler). Any other role may run a
// handler only if it appears in their union allowed-set. An empty allowed-set
// (no command groups) denies everything for non-super_admin.
func (a *AllowedSet) Allows(handlerKey string) bool {
	if a.IsSuperAdmin() {
		return true
	}
	return a.keys[handlerKey]
}

// HandlerKeys returns the sorted-insertion handler keys explicitly granted
// (ignores super_admin bypass — used for completion hints of named commands).
func (a *AllowedSet) Commands() []client.AllowedCommand {
	return a.commands
}
