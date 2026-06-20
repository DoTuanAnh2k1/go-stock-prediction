package shell

import (
	"fmt"
	"strings"
)

// ParsedInput is the result of parsing a typed command line.
//
// Grammar:
//
//	<verb> <resource> [key=value ...]
//
// e.g. "get market.latest market=gold"
type ParsedInput struct {
	Verb     string
	Resource string
	Args     map[string]string
}

// Parse splits a raw line into verb, resource and key=value args.
func Parse(line string) (*ParsedInput, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	p := &ParsedInput{Verb: fields[0], Args: map[string]string{}}
	if len(fields) >= 2 && !strings.Contains(fields[1], "=") {
		p.Resource = fields[1]
		fields = fields[2:]
	} else {
		fields = fields[1:]
	}
	for _, f := range fields {
		idx := strings.Index(f, "=")
		if idx < 0 {
			return nil, fmt.Errorf("invalid argument %q (expected key=value)", f)
		}
		key := f[:idx]
		val := f[idx+1:]
		if key == "" {
			return nil, fmt.Errorf("invalid argument %q (empty key)", f)
		}
		p.Args[key] = val
	}
	return p, nil
}
