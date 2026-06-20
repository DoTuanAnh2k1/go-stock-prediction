package shell

import (
	"sort"
	"strings"

	"go-stock-prediction/cli-svc/internal/handlers"
)

// Completer produces tab-completion suggestions for the current input, filtered
// to what the user is allowed to run.
type Completer struct {
	reg     *handlers.Registry
	allowed *AllowedSet
}

// NewCompleter builds a completer over the catalog and the user's allowed-set.
func NewCompleter(reg *handlers.Registry, allowed *AllowedSet) *Completer {
	return &Completer{reg: reg, allowed: allowed}
}

// allowedHandler reports whether a handler is runnable by this user.
func (c *Completer) allowedHandler(h *handlers.Handler) bool {
	return c.allowed.Allows(h.Key)
}

// allowedVerbs returns verbs that have at least one allowed handler.
func (c *Completer) allowedVerbs() []string {
	set := map[string]bool{}
	for _, h := range c.reg.All() {
		if c.allowedHandler(h) {
			set[h.Verb] = true
		}
	}
	return sortedSet(set)
}

// allowedResources returns resources under a verb that the user can run.
func (c *Completer) allowedResources(verb string) []string {
	set := map[string]bool{}
	for _, h := range c.reg.All() {
		if h.Verb == verb && c.allowedHandler(h) {
			set[h.Resource] = true
		}
	}
	return sortedSet(set)
}

// Suggest returns completion candidates for the raw line. It returns the list of
// full-token suggestions for the token currently being typed.
//
// Stages:
//   - typing the verb        → suggest allowed verbs
//   - typing the resource    → suggest allowed resources under the verb
//   - typing an arg          → suggest "name=" for each arg, or value choices
func (c *Completer) Suggest(line string) []string {
	// Determine whether the line ends with a space (token complete) so we know
	// to suggest the NEXT token vs filter the current one.
	endsWithSpace := strings.HasSuffix(line, " ")
	fields := strings.Fields(line)

	// Stage 1: verb.
	if len(fields) == 0 || (len(fields) == 1 && !endsWithSpace) {
		prefix := ""
		if len(fields) == 1 {
			prefix = fields[0]
		}
		return filterPrefix(c.allowedVerbs(), prefix)
	}

	verb := fields[0]

	// Stage 2: resource.
	if len(fields) == 1 || (len(fields) == 2 && !endsWithSpace && !strings.Contains(fields[1], "=")) {
		prefix := ""
		if len(fields) == 2 {
			prefix = fields[1]
		}
		return filterPrefix(c.allowedResources(verb), prefix)
	}

	// Stage 3: args. Need a resolved handler.
	resource := fields[1]
	h, ok := c.reg.Resolve(verb, resource)
	if !ok || !c.allowedHandler(h) {
		return nil
	}

	// Determine the token currently being typed.
	curTok := ""
	if !endsWithSpace {
		curTok = fields[len(fields)-1]
	}

	// Collect already-provided arg names.
	provided := map[string]bool{}
	start := 2
	for i := start; i < len(fields); i++ {
		if i == len(fields)-1 && !endsWithSpace {
			break // current token not yet committed
		}
		if eq := strings.Index(fields[i], "="); eq > 0 {
			provided[fields[i][:eq]] = true
		}
	}

	// If the current token contains '=', suggest value choices.
	if eq := strings.Index(curTok, "="); eq >= 0 {
		name := curTok[:eq]
		valPrefix := curTok[eq+1:]
		for _, spec := range h.ArgSchema {
			if spec.Name == name && len(spec.Choices) > 0 {
				out := []string{}
				for _, ch := range spec.Choices {
					if strings.HasPrefix(ch, valPrefix) {
						out = append(out, name+"="+ch)
					}
				}
				return out
			}
		}
		return nil
	}

	// Otherwise suggest remaining arg names as "name=".
	out := []string{}
	for _, spec := range h.ArgSchema {
		if provided[spec.Name] {
			continue
		}
		cand := spec.Name + "="
		if strings.HasPrefix(cand, curTok) {
			out = append(out, cand)
		}
	}
	sort.Strings(out)
	return out
}

func filterPrefix(items []string, prefix string) []string {
	out := []string{}
	for _, it := range items {
		if strings.HasPrefix(it, prefix) {
			out = append(out, it)
		}
	}
	return out
}

func sortedSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
