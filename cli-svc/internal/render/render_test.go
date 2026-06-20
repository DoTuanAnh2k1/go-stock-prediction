package render

import (
	"strings"
	"testing"
)

func TestAutoArray(t *testing.T) {
	raw := []any{
		map[string]any{"job_key": "crawler_gold", "enabled": true, "cron_expression": "0 0 * * * *"},
		map[string]any{"job_key": "daily_backup", "enabled": false, "cron_expression": "0 0 3 * * *"},
	}
	out := Auto("Schedules", raw)
	for _, want := range []string{"job_key", "crawler_gold", "daily_backup", "true", "false"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestAutoEnvelopeArray(t *testing.T) {
	raw := map[string]any{
		"success": true,
		"data": []any{
			map[string]any{"algorithm": "lstm_nn", "total": 10.0, "correct": 7.0},
		},
	}
	out := Auto("Direction accuracy", raw)
	for _, want := range []string{"algorithm", "lstm_nn", "total", "correct"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	// Integer floats render without decimals.
	if !strings.Contains(out, "10") || strings.Contains(out, "10.0000") {
		t.Errorf("number formatting wrong:\n%s", out)
	}
}

func TestAutoSingleObject(t *testing.T) {
	raw := map[string]any{"data": map[string]any{"price": 2500.5, "source": "SJC"}}
	out := Auto("Latest", raw)
	for _, want := range []string{"field", "value", "price", "source", "SJC"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestAutoEmbeddedArrayField(t *testing.T) {
	// {data:[...], pipelines:[...]} — should prefer data.
	raw := map[string]any{
		"data": []any{
			map[string]any{"id": 1.0, "status": "success"},
		},
		"pipelines": []any{"crawler_gold"},
	}
	out := Auto("Pipeline reports", raw)
	if !strings.Contains(out, "status") || !strings.Contains(out, "success") {
		t.Errorf("expected data array rendered:\n%s", out)
	}
}

func TestTableNoRows(t *testing.T) {
	out := Table("Empty", []string{"a"}, nil)
	if !strings.Contains(out, "no rows") {
		t.Errorf("expected no rows message: %q", out)
	}
}
