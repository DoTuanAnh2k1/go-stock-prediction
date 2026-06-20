package shell

import (
	"sort"
	"strings"

	"go-stock-prediction/cli-svc/internal/handlers"
)

// Suggestion is a single completion candidate: the token to insert plus a short
// description shown dimmed beside it (kube-prompt style).
type Suggestion struct {
	Text string
	Desc string
}

// verbDesc gives a human label for each verb shown in the dropdown.
var verbDesc = map[string]string{
	"get":    "read",
	"set":    "create / trigger",
	"update": "modify",
	"delete": "remove",
}

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

func (c *Completer) allowedHandler(h *handlers.Handler) bool {
	return c.allowed.Allows(h.Key)
}

func (c *Completer) allowedVerbs() []string {
	set := map[string]bool{}
	for _, h := range c.reg.All() {
		if c.allowedHandler(h) {
			set[h.Verb] = true
		}
	}
	return sortedSet(set)
}

func (c *Completer) allowedResources(verb string) []string {
	set := map[string]bool{}
	for _, h := range c.reg.All() {
		if h.Verb == verb && c.allowedHandler(h) {
			set[h.Resource] = true
		}
	}
	return sortedSet(set)
}

// Suggest returns just the insertable texts (used by tests).
func (c *Completer) Suggest(line string) []string {
	rich := c.SuggestRich(line)
	out := make([]string, len(rich))
	for i, s := range rich {
		out[i] = s.Text
	}
	return out
}

// SuggestRich returns completion candidates (with descriptions) for the raw line.
//
// Stages:
//   - typing the verb     → allowed verbs
//   - typing the resource → allowed resources under the verb
//   - typing an arg       → "name=" per arg, or value choices
func (c *Completer) SuggestRich(line string) []Suggestion {
	endsWithSpace := strings.HasSuffix(line, " ")
	fields := strings.Fields(line)

	// Stage 1: verb.
	if len(fields) == 0 || (len(fields) == 1 && !endsWithSpace) {
		prefix := ""
		if len(fields) == 1 {
			prefix = fields[0]
		}
		out := []Suggestion{}
		for _, v := range c.allowedVerbs() {
			if strings.HasPrefix(v, prefix) {
				out = append(out, Suggestion{Text: v, Desc: verbDesc[v]})
			}
		}
		return out
	}

	verb := fields[0]

	// Stage 2: resource.
	if len(fields) == 1 || (len(fields) == 2 && !endsWithSpace && !strings.Contains(fields[1], "=")) {
		prefix := ""
		if len(fields) == 2 {
			prefix = fields[1]
		}
		out := []Suggestion{}
		for _, res := range c.allowedResources(verb) {
			if !strings.HasPrefix(res, prefix) {
				continue
			}
			desc := ""
			if h, ok := c.reg.Resolve(verb, res); ok {
				desc = h.DisplayName
			}
			out = append(out, Suggestion{Text: res, Desc: desc})
		}
		return out
	}

	// Stage 3: args. Need a resolved handler.
	resource := fields[1]
	h, ok := c.reg.Resolve(verb, resource)
	if !ok || !c.allowedHandler(h) {
		return nil
	}

	curTok := ""
	if !endsWithSpace {
		curTok = fields[len(fields)-1]
	}

	// Already-provided arg names.
	provided := map[string]bool{}
	for i := 2; i < len(fields); i++ {
		if i == len(fields)-1 && !endsWithSpace {
			break
		}
		if eq := strings.Index(fields[i], "="); eq > 0 {
			provided[fields[i][:eq]] = true
		}
	}

	// Current token has '=' → suggest value choices.
	if eq := strings.Index(curTok, "="); eq >= 0 {
		name := curTok[:eq]
		valPrefix := curTok[eq+1:]
		for _, spec := range h.ArgSchema {
			if spec.Name == name && len(spec.Choices) > 0 {
				out := []Suggestion{}
				for _, ch := range spec.Choices {
					if strings.HasPrefix(ch, valPrefix) {
						out = append(out, Suggestion{Text: name + "=" + ch, Desc: "value"})
					}
				}
				return out
			}
		}
		return nil
	}

	// Otherwise suggest remaining arg names as "name=".
	out := []Suggestion{}
	for _, spec := range h.ArgSchema {
		if provided[spec.Name] {
			continue
		}
		cand := spec.Name + "="
		if !strings.HasPrefix(cand, curTok) {
			continue
		}
		out = append(out, Suggestion{Text: cand, Desc: argDesc(spec)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Text < out[j].Text })
	return out
}

// argDesc describes an arg spec for the dropdown (type/choices + optionality).
func argDesc(spec handlers.ArgSpec) string {
	var d string
	if len(spec.Choices) > 0 {
		d = "{" + strings.Join(spec.Choices, "|") + "}"
	} else {
		d = "<" + spec.Type + ">"
	}
	if spec.Required {
		return d + " required"
	}
	return d + " optional"
}

func sortedSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
