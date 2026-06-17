# Dashboard Pipeline Summary — Design Spec
Date: 2026-06-17

## Goal

Add pipeline monitoring data directly to the Dashboard overview page so users see crawl freshness, data counts, and top algo accuracy without navigating to /monitoring.

## Scope

Two changes inside `frontend/src/pages/Dashboard.tsx`, within the existing "Market Pipeline Status" Panel. No new pages, no new API calls (reuses `fetchMonitoringOverview` already called).

---

## Change 1 — MarketStatusCard: add daily/intraday counts

Add one small line below the "Last crawl" row inside `MarketStatusCard`:

```
11 daily · 44 intraday
```

- Font: `var(--font-mono)`, size 11, color `var(--text-3)`
- Only render if `crawl.daily_today > 0 || crawl.intraday_today > 0`
- No layout change to the card otherwise

---

## Change 2 — Compact Summary Table (new)

Placed below the 4 market cards grid, above the existing footer summary, separated by a thin divider.

### Columns

| Column | Content |
|--------|---------|
| Market | Icon + colored market name |
| Crawl | `{staleness}` + fresh/stale badge (reuse existing badge style) |
| Data | `{daily_today}d / {intraday_today}i` |
| Preds | `{today_total}` |
| Top-3 Algo | 3 chips: algo short name + accuracy %, sorted desc by direction_accuracy, only algos with reconciled > 0 |

### Algo chip colors
- accuracy ≥ 55%: `var(--up-bg)` / `var(--up)`
- accuracy ≥ 45%: `color-mix(in oklch, var(--gold) 15%, transparent)` / `var(--gold)`
- accuracy < 45%: `var(--down-bg)` / `var(--down)`

### Row behavior
- Entire row is clickable → navigate to market page (same links as `MARKET_LINKS`)
- Hover: `var(--surface-2)` background

### Responsive
- Mobile (≤ 640px): hide "Top-3 Algo" column, show 3 remaining columns

### Empty states
- No monitoring data: table not rendered
- Market not accessible (`canAccessMarket` = false): row not rendered
- No reconciled algos for a market: Top-3 column shows `—`

---

## Files touched

- `frontend/src/pages/Dashboard.tsx` — only file changed
  - `MarketStatusCard` component: add daily/intraday line
  - New `PipelineSummaryTable` component
  - Insert `<PipelineSummaryTable>` inside the monitoring Panel

## Non-goals

- No changes to Monitoring page
- No new API types (MonitoringMarket already has all needed fields)
- No changes to i18n (labels are short, inline English/Vietnamese acceptable)
