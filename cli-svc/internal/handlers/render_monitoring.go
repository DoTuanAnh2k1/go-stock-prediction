package handlers

import (
	"strings"

	"go-stock-prediction/cli-svc/internal/render"
)

// renderMonitoring turns the deeply-nested /monitoring/overview payload into a
// compact, readable layout: one row per market (crawl freshness + prediction
// summary) plus a one-line bots summary — instead of dumping nested objects.
func renderMonitoring(raw any) string {
	obj := asMap(raw)
	if d := asMap(obj["data"]); d != nil {
		obj = d
	}
	if obj == nil {
		return render.Auto("Monitoring", raw)
	}

	headers := []string{"market", "last_crawl", "stale", "daily", "intraday", "pred_today", "missing"}
	var rows []map[string]any
	for _, mi := range asArr(obj["markets"]) {
		m := asMap(mi)
		crawl := asMap(m["crawl"])
		pred := asMap(m["predictions"])
		rows = append(rows, map[string]any{
			"market":     m["market"],
			"last_crawl": crawl["last_crawl_at"],
			"stale":      crawl["stale"],
			"daily":      crawl["daily_today"],
			"intraday":   crawl["intraday_today"],
			"pred_today": pred["today_total"],
			"missing":    pred["missing_today"],
		})
	}

	parts := []string{render.Table("Monitoring — markets", headers, rows)}

	if bots := asMap(obj["bots"]); bots != nil {
		if sum := asMap(bots["summary"]); sum != nil {
			parts = append(parts, render.KeyValue("Bots", map[string]any{
				"total_bots":  sum["total_bots"],
				"active_bots": sum["active_bots"],
			}))
		}
	}
	return strings.Join(parts, "\n")
}

func asMap(v any) map[string]any { m, _ := v.(map[string]any); return m }
func asArr(v any) []any          { a, _ := v.([]any); return a }
