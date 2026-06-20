package shell

import (
	"fmt"
	"strings"
	"unicode"
)

// ParsedInput is the result of parsing a typed command line.
//
// Grammar (space-separated, kube-prompt style):
//
//	<verb> <category> <name> [argname argvalue ...]
//
// e.g.  get market latest market gold
//
//	update schedule update key crawler_gold cron_expression "0 0 2 * * *"
//
// The resource is the category and name joined with a dot (market.latest), to
// match the handler keys. Argument values containing spaces must be quoted.
type ParsedInput struct {
	Verb     string
	Resource string
	Args     map[string]string
}

// Parse splits a raw line into verb, resource (category.name) and name/value args.
func Parse(line string) (*ParsedInput, error) {
	toks := tokenize(line)
	if len(toks) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	p := &ParsedInput{Verb: toks[0], Args: map[string]string{}}
	rest := toks[1:]

	switch {
	case len(rest) >= 2:
		p.Resource = rest[0] + "." + rest[1]
		rest = rest[2:]
	case len(rest) == 1:
		// Only a category typed — leave resource incomplete (Resolve fails with
		// a clear "unknown command").
		p.Resource = rest[0]
		rest = nil
	}

	for i := 0; i < len(rest); i += 2 {
		name := rest[i]
		if name == "" {
			return nil, fmt.Errorf("empty argument name")
		}
		if i+1 >= len(rest) {
			return nil, fmt.Errorf("missing value for argument %q", name)
		}
		p.Args[name] = rest[i+1]
	}
	return p, nil
}

// tokenize splits on whitespace, keeping "double quoted" runs together so that
// values with spaces (e.g. cron expressions) survive as a single token.
func tokenize(s string) []string {
	var toks []string
	var cur strings.Builder
	inQuote := false
	started := false
	flush := func() {
		if started {
			toks = append(toks, cur.String())
			cur.Reset()
			started = false
		}
	}
	for _, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
			started = true // an empty "" is still a token
		case unicode.IsSpace(r) && !inQuote:
			flush()
		default:
			cur.WriteRune(r)
			started = true
		}
	}
	flush()
	return toks
}
