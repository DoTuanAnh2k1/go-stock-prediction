// Package render turns decoded JSON API responses into pretty tables.
package render

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

// applyStyle sets a consistent style and preserves header casing (go-pretty
// upper-cases headers by default; we keep the JSON key casing).
func applyStyle(t table.Writer) {
	t.SetStyle(table.StyleLight)
	style := t.Style()
	style.Format.Header = text.FormatDefault
}

// Table renders rows (each a map column->value) with the given ordered headers.
// If headers is empty it derives a stable union of keys from the rows.
func Table(title string, headers []string, rows []map[string]any) string {
	if len(rows) == 0 {
		if title != "" {
			return title + "\n(no rows)"
		}
		return "(no rows)"
	}
	if len(headers) == 0 {
		headers = deriveHeaders(rows)
	}
	t := table.NewWriter()
	if title != "" {
		t.SetTitle(title)
	}
	hdr := make(table.Row, len(headers))
	for i, h := range headers {
		hdr[i] = h
	}
	t.AppendHeader(hdr)
	for _, r := range rows {
		row := make(table.Row, len(headers))
		for i, h := range headers {
			row[i] = cell(r[h])
		}
		t.AppendRow(row)
	}
	applyStyle(t)
	return t.Render()
}

// KeyValue renders a single object as a two-column key/value table.
func KeyValue(title string, obj map[string]any) string {
	if len(obj) == 0 {
		if title != "" {
			return title + "\n(empty)"
		}
		return "(empty)"
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	t := table.NewWriter()
	if title != "" {
		t.SetTitle(title)
	}
	t.AppendHeader(table.Row{"field", "value"})
	for _, k := range keys {
		t.AppendRow(table.Row{k, cell(obj[k])})
	}
	applyStyle(t)
	return t.Render()
}

// Auto inspects a raw decoded JSON value and renders the most sensible table.
// It unwraps a {success,data} envelope, renders arrays as multi-row tables,
// objects-of-arrays by picking the first array field, and single objects as
// key/value tables.
func Auto(title string, raw any) string {
	val := unwrap(raw)
	switch v := val.(type) {
	case []any:
		return Table(title, nil, toRows(v))
	case map[string]any:
		// Prefer an embedded array field (e.g. {data:[...], pipelines:[...]}).
		if arrKey, arr := firstArray(v); arrKey != "" {
			return Table(title+" ("+arrKey+")", nil, toRows(arr))
		}
		return KeyValue(title, v)
	case nil:
		return title + "\n(no data)"
	default:
		return fmt.Sprintf("%s\n%v", title, v)
	}
}

func unwrap(raw any) any {
	m, ok := raw.(map[string]any)
	if !ok {
		return raw
	}
	if d, ok := m["data"]; ok {
		return d
	}
	return m
}

func firstArray(m map[string]any) (string, []any) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	// Prefer a key literally named "data" if it's an array.
	if d, ok := m["data"].([]any); ok {
		return "data", d
	}
	for _, k := range keys {
		if arr, ok := m[k].([]any); ok && len(arr) > 0 {
			if _, isObj := arr[0].(map[string]any); isObj {
				return k, arr
			}
		}
	}
	return "", nil
}

func toRows(arr []any) []map[string]any {
	rows := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		if m, ok := item.(map[string]any); ok {
			rows = append(rows, m)
		} else {
			rows = append(rows, map[string]any{"value": item})
		}
	}
	return rows
}

func deriveHeaders(rows []map[string]any) []string {
	seen := map[string]bool{}
	var headers []string
	for _, r := range rows {
		keys := make([]string, 0, len(r))
		for k := range r {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if !seen[k] {
				seen[k] = true
				headers = append(headers, k)
			}
		}
	}
	return headers
}

func cell(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		// Render integers without trailing ".0".
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%.4g", x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case []any:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			parts = append(parts, cell(e))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		return fmt.Sprintf("%v", x)
	default:
		return fmt.Sprintf("%v", x)
	}
}
