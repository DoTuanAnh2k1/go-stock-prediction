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

var verbDesc = map[string]string{
	"get":    "read",
	"set":    "create / trigger",
	"update": "modify",
	"delete": "remove",
}

// Completer produces tab-completion suggestions for the current input, filtered
// to what the user is allowed to run. Grammar: verb category name [arg val ...].
type Completer struct {
	reg     *handlers.Registry
	allowed *AllowedSet
}

func NewCompleter(reg *handlers.Registry, allowed *AllowedSet) *Completer {
	return &Completer{reg: reg, allowed: allowed}
}

func (c *Completer) allows(h *handlers.Handler) bool { return c.allowed.Allows(h.Key) }

// split returns (category, name) for a handler key "category.name".
func split(key string) (string, string) {
	if i := strings.IndexByte(key, '.'); i >= 0 {
		return key[:i], key[i+1:]
	}
	return key, ""
}

func (c *Completer) verbs() []string {
	set := map[string]bool{}
	for _, h := range c.reg.All() {
		if c.allows(h) {
			set[h.Verb] = true
		}
	}
	return sortedSet(set)
}

func (c *Completer) categories(verb string) []string {
	set := map[string]bool{}
	for _, h := range c.reg.All() {
		if h.Verb == verb && c.allows(h) {
			cat, _ := split(h.Key)
			set[cat] = true
		}
	}
	return sortedSet(set)
}

func (c *Completer) names(verb, category string) []string {
	set := map[string]bool{}
	for _, h := range c.reg.All() {
		if h.Verb != verb || !c.allows(h) {
			continue
		}
		cat, name := split(h.Key)
		if cat == category {
			set[name] = true
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

// SuggestRich returns completion candidates for the current token of the line.
func (c *Completer) SuggestRich(line string) []Suggestion {
	endsWithSpace := strings.HasSuffix(line, " ")
	toks := strings.Fields(line)
	n := len(toks)

	// Which token are we completing, and its current prefix?
	idx := 0
	prefix := ""
	if endsWithSpace {
		idx = n
	} else if n > 0 {
		idx = n - 1
		prefix = toks[idx]
	}

	switch {
	case idx == 0: // verb
		out := []Suggestion{}
		for _, v := range c.verbs() {
			if strings.HasPrefix(v, prefix) {
				out = append(out, Suggestion{Text: v, Desc: verbDesc[v]})
			}
		}
		return out

	case idx == 1: // category
		verb := toks[0]
		out := []Suggestion{}
		for _, cat := range c.categories(verb) {
			if strings.HasPrefix(cat, prefix) {
				out = append(out, Suggestion{Text: cat, Desc: ""})
			}
		}
		return out

	case idx == 2: // name
		verb, cat := toks[0], toks[1]
		out := []Suggestion{}
		for _, name := range c.names(verb, cat) {
			if !strings.HasPrefix(name, prefix) {
				continue
			}
			desc := ""
			if h, ok := c.reg.Resolve(verb, cat+"."+name); ok {
				desc = h.DisplayName
			}
			out = append(out, Suggestion{Text: name, Desc: desc})
		}
		return out
	}

	// idx >= 3 → arguments. Resolve the handler from category.name.
	h, ok := c.reg.Resolve(toks[0], toks[1]+"."+toks[2])
	if !ok || !c.allows(h) {
		return nil
	}

	// Argument tokens start at index 3 and alternate name, value, name, value …
	argPos := idx - 3
	if argPos%2 == 0 {
		// Editing an argument NAME — suggest not-yet-provided names.
		provided := map[string]bool{}
		for i := 3; i < idx; i += 2 {
			if i < len(toks) {
				provided[toks[i]] = true
			}
		}
		out := []Suggestion{}
		for _, spec := range h.ArgSchema {
			if provided[spec.Name] || !strings.HasPrefix(spec.Name, prefix) {
				continue
			}
			out = append(out, Suggestion{Text: spec.Name, Desc: argDesc(spec)})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Text < out[j].Text })
		return out
	}

	// Editing an argument VALUE for the preceding name.
	name := toks[idx-1]
	for _, spec := range h.ArgSchema {
		if spec.Name == name && len(spec.Choices) > 0 {
			out := []Suggestion{}
			for _, ch := range spec.Choices {
				if strings.HasPrefix(ch, prefix) {
					out = append(out, Suggestion{Text: ch, Desc: "value"})
				}
			}
			return out
		}
	}
	return nil
}

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
