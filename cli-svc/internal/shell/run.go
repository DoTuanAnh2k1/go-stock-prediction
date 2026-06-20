package shell

import (
	"context"
	"fmt"
	"net/http"

	"go-stock-prediction/cli-svc/internal/client"
	"go-stock-prediction/cli-svc/internal/handlers"
)

// Runner resolves a typed line to a handler, enforces permissions, executes the
// API call and renders the result. It is the testable core of the shell.
type Runner struct {
	reg     *handlers.Registry
	client  client.HTTPClient
	allowed *AllowedSet
	jwt     string
}

// NewRunner builds a Runner.
func NewRunner(reg *handlers.Registry, c client.HTTPClient, allowed *AllowedSet, jwt string) *Runner {
	return &Runner{reg: reg, client: c, allowed: allowed, jwt: jwt}
}

// Run executes a raw command line and returns the rendered output (table or
// message). The bool reports whether the command was permitted+valid enough to
// attempt execution (false → output is an error/permission message).
func (r *Runner) Run(ctx context.Context, line string) (string, bool) {
	parsed, err := Parse(line)
	if err != nil {
		return "error: " + err.Error(), false
	}
	h, ok := r.reg.Resolve(parsed.Verb, parsed.Resource)
	if !ok {
		return fmt.Sprintf("unknown command: %s %s", parsed.Verb, parsed.Resource), false
	}
	if !r.allowed.Allows(h.Key) {
		return fmt.Sprintf("permission denied: you are not allowed to run %q (%s)", h.Resource, h.Key), false
	}
	args, err := h.Validate(parsed.Args)
	if err != nil {
		return "error: " + err.Error(), false
	}
	raw, status, err := h.Execute(ctx, r.client, r.jwt, args)
	if err != nil {
		return "request failed: " + err.Error(), false
	}
	if status >= 400 {
		return fmt.Sprintf("API error (HTTP %d): %s", status, httpMessage(raw, status)), false
	}
	return h.Render(raw), true
}

func httpMessage(raw any, status int) string {
	if m, ok := raw.(map[string]any); ok {
		for _, k := range []string{"error", "message"} {
			if s, ok := m[k].(string); ok && s != "" {
				return s
			}
		}
	}
	if s, ok := raw.(string); ok && s != "" {
		return s
	}
	return http.StatusText(status)
}
