package shell

import (
	"regexp"
	"strconv"
	"strings"
)

// splitPipe splits "cmd | grep ..." into the command and the grep spec, ignoring
// any '|' that appears inside "double quotes" (so a regex like "A|B" survives).
func splitPipe(line string) (cmd, grep string) {
	inQuote := false
	for i, r := range line {
		switch r {
		case '"':
			inQuote = !inQuote
		case '|':
			if !inQuote {
				return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:])
			}
		}
	}
	return strings.TrimSpace(line), ""
}

type grepSpec struct {
	pattern    string
	ignoreCase bool
	after      int // -A
	before     int // -B
}

// parseGrep parses `grep [-i] [-A n] [-B n] [-C n] pattern`. The pattern may be
// quoted (to include spaces or a leading '-') and is treated as a regular
// expression. Uses the shared tokenizer so "abc xyz" stays one token.
func parseGrep(spec string) (*grepSpec, bool) {
	toks := tokenize(spec)
	if len(toks) == 0 || toks[0] != "grep" {
		return nil, false
	}
	g := &grepSpec{}
	var pat []string
	atoi := func(s string) int { n, _ := strconv.Atoi(s); return n }
	for i := 1; i < len(toks); {
		switch toks[i] {
		case "-i":
			g.ignoreCase = true
			i++
		case "-A":
			if i+1 < len(toks) {
				g.after = atoi(toks[i+1])
				i += 2
			} else {
				i++
			}
		case "-B":
			if i+1 < len(toks) {
				g.before = atoi(toks[i+1])
				i += 2
			} else {
				i++
			}
		case "-C":
			if i+1 < len(toks) {
				n := atoi(toks[i+1])
				g.after, g.before = n, n
				i += 2
			} else {
				i++
			}
		default:
			pat = append(pat, toks[i])
			i++
		}
	}
	g.pattern = strings.Join(pat, " ")
	return g, true
}

// matcher returns the line-matching function for the spec: a regular expression
// when the pattern compiles, otherwise a literal substring match.
func (g *grepSpec) matcher() func(string) bool {
	expr := g.pattern
	if g.ignoreCase {
		expr = "(?i)" + expr
	}
	if re, err := regexp.Compile(expr); err == nil {
		return re.MatchString
	}
	needle := g.pattern
	if g.ignoreCase {
		needle = strings.ToLower(needle)
	}
	return func(line string) bool {
		hay := line
		if g.ignoreCase {
			hay = strings.ToLower(line)
		}
		return strings.Contains(hay, needle)
	}
}

// applyGrep filters rendered output by a grep spec (regex match, with -A/-B
// context lines and "--" separators between non-adjacent groups, like Unix grep).
func applyGrep(output, spec string) string {
	g, ok := parseGrep(spec)
	if !ok || g.pattern == "" {
		return output
	}
	lines := strings.Split(output, "\n")
	match := g.matcher()

	keep := make([]bool, len(lines))
	any := false
	for i, l := range lines {
		if match(l) {
			any = true
			for j := i - g.before; j <= i+g.after; j++ {
				if j >= 0 && j < len(lines) {
					keep[j] = true
				}
			}
		}
	}
	if !any {
		return "(no match for \"" + g.pattern + "\")"
	}

	var out []string
	prev := -2
	withCtx := g.before > 0 || g.after > 0
	for i := 0; i < len(lines); i++ {
		if !keep[i] {
			continue
		}
		if withCtx && prev >= 0 && i > prev+1 {
			out = append(out, "--")
		}
		out = append(out, lines[i])
		prev = i
	}
	return strings.Join(out, "\n")
}
