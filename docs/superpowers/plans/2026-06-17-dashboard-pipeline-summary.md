# Dashboard Pipeline Summary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add daily/intraday data counts to market cards and a compact 4-row summary table (one per market) inside the existing "Market Pipeline Status" Panel on the Dashboard.

**Architecture:** Two UI additions to `frontend/src/pages/Dashboard.tsx` only — no new API calls, no backend changes. The `fetchMonitoringOverview()` call already runs on mount; both additions consume the existing `monitoring` state. A CSS class for mobile column hide is added to `styles.css`.

**Tech Stack:** React 18, TypeScript, react-router-dom `useNavigate`, existing CSS variables (`--up`, `--down`, `--gold`, `--font-mono`, etc.)

## Global Constraints

- Only touch `frontend/src/pages/Dashboard.tsx` and `frontend/src/styles.css`
- Do not add new API calls or state — reuse existing `monitoring` / `accessibleMarkets`
- Follow existing inline-style patterns (no new CSS classes except `pipeline-hide-mobile`)
- `MARKET_COLORS`, `MARKET_ICONS`, `MARKET_LINKS`, `algoLabel()` are already defined in Dashboard.tsx — reuse them
- All existing types are already imported: `MonitoringMarket` from `'../api'`

---

### Task 1: Add daily/intraday count line to MarketStatusCard

**Files:**
- Modify: `frontend/src/pages/Dashboard.tsx` — `MarketStatusCard` component (lines ~159–169)

**Interfaces:**
- Consumes: `crawl.daily_today: number`, `crawl.intraday_today: number` — already on `MonitoringCrawl` type
- Produces: nothing (UI only)

- [ ] **Step 1: Locate the crawl info div in MarketStatusCard**

In `Dashboard.tsx`, find this block (around line 159):

```tsx
        {/* Crawl info */}
        <div style={{ fontSize: 11, color: 'var(--text-3)', display: 'flex', alignItems: 'center', gap: 6 }}>
          <span>{d.lastCrawl}:</span>
          <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--text-2)' }}>
            {crawl.last_crawl_at ? fmtDT(crawl.last_crawl_at) : d.never}
          </span>
          {crawl.staleness && (
            <span style={{ color: crawl.stale ? 'var(--down)' : 'var(--text-3)' }}>
              ({crawl.staleness})
            </span>
          )}
        </div>
```

- [ ] **Step 2: Add daily/intraday line immediately after that div**

Replace the block above with:

```tsx
        {/* Crawl info */}
        <div style={{ fontSize: 11, color: 'var(--text-3)', display: 'flex', alignItems: 'center', gap: 6 }}>
          <span>{d.lastCrawl}:</span>
          <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--text-2)' }}>
            {crawl.last_crawl_at ? fmtDT(crawl.last_crawl_at) : d.never}
          </span>
          {crawl.staleness && (
            <span style={{ color: crawl.stale ? 'var(--down)' : 'var(--text-3)' }}>
              ({crawl.staleness})
            </span>
          )}
        </div>
        {(crawl.daily_today > 0 || crawl.intraday_today > 0) && (
          <div style={{ fontSize: 11, color: 'var(--text-3)', marginTop: 4, fontFamily: 'var(--font-mono)' }}>
            {crawl.daily_today}d · {crawl.intraday_today}i
          </div>
        )}
```

- [ ] **Step 3: Verify build compiles**

```bash
cd /home/chronical/Projects/private/go-stock-prediction/frontend
npm run build 2>&1 | tail -10
```

Expected: no TypeScript errors. Warnings about bundle size are OK.

- [ ] **Step 4: Commit**

```bash
cd /home/chronical/Projects/private/go-stock-prediction
git add frontend/src/pages/Dashboard.tsx
git commit -m "feat(dashboard): show daily/intraday data counts on market cards"
```

---

### Task 2: Add PipelineSummaryTable and wire it into the Panel

**Files:**
- Modify: `frontend/src/pages/Dashboard.tsx` — add two new components + one import + one JSX insertion
- Modify: `frontend/src/styles.css` — add mobile hide class

**Interfaces:**
- Consumes: `accessibleMarkets: MonitoringMarket[]` (already computed in `Dashboard`)
- Consumes: `MARKET_COLORS`, `MARKET_ICONS`, `MARKET_LINKS`, `algoLabel()` — all already defined above in the file
- Produces: `<PipelineSummaryTable markets={accessibleMarkets} />` — rendered inside the monitoring Panel

- [ ] **Step 1: Add `useNavigate` to the react-router-dom import**

Find line 2 in `Dashboard.tsx`:

```tsx
import { Link } from 'react-router-dom';
```

Replace with:

```tsx
import { Link, useNavigate } from 'react-router-dom';
```

- [ ] **Step 2: Add the two new components before the `Dashboard()` function**

In `Dashboard.tsx`, find the comment line `// ── Main Dashboard ──` (around line 175). Insert the following two components **immediately before** that comment:

```tsx
// ── Pipeline Summary Table ─────────────────────────────────────────────────────

function SummaryRow({ market: m }: { market: MonitoringMarket }) {
  const navigate = useNavigate();
  const color = MARKET_COLORS[m.market] || 'var(--accent)';
  const icon = MARKET_ICONS[m.market] || 'candles';
  const link = MARKET_LINKS[m.market] || '/';
  const { crawl, predictions } = m;

  const top3 = [...(predictions.algorithms || [])]
    .filter(a => a.reconciled > 0)
    .sort((a, b) => b.direction_accuracy - a.direction_accuracy)
    .slice(0, 3);

  return (
    <tr
      onClick={() => navigate(link)}
      style={{ cursor: 'pointer' }}
      onMouseEnter={e => { (e.currentTarget as HTMLTableRowElement).style.background = 'var(--surface-2)'; }}
      onMouseLeave={e => { (e.currentTarget as HTMLTableRowElement).style.background = ''; }}
    >
      <td style={{ padding: '7px 8px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <Icon name={icon} size={13} style={{ color }} />
          <span style={{ fontWeight: 700, color, fontFamily: 'var(--font-mono)', fontSize: 11, letterSpacing: '0.5px' }}>
            {m.market}
          </span>
        </div>
      </td>
      <td style={{ padding: '7px 8px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 5 }}>
          <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--text-2)', fontSize: 11 }}>
            {crawl.staleness || '—'}
          </span>
          <span style={{
            fontSize: 9, padding: '1px 5px',
            background: crawl.stale ? 'var(--down-bg)' : 'var(--up-bg)',
            color: crawl.stale ? 'var(--down)' : 'var(--up)',
            fontFamily: 'var(--font-mono)',
          }}>
            {crawl.stale ? 'STALE' : 'OK'}
          </span>
        </div>
      </td>
      <td style={{ padding: '7px 8px' }}>
        <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--text-3)', fontSize: 11 }}>
          {crawl.daily_today}d / {crawl.intraday_today}i
        </span>
      </td>
      <td style={{ padding: '7px 8px', textAlign: 'right' }}>
        <span style={{ fontFamily: 'var(--font-mono)', fontWeight: 600, fontSize: 12 }}>
          {predictions.today_total.toLocaleString()}
        </span>
      </td>
      <td className="pipeline-hide-mobile" style={{ padding: '7px 8px' }}>
        <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
          {top3.length === 0 ? (
            <span style={{ color: 'var(--text-3)', fontSize: 11 }}>—</span>
          ) : top3.map(a => {
            const acc = a.direction_accuracy * 100;
            const bg = acc >= 55
              ? 'var(--up-bg)'
              : acc >= 45
                ? 'color-mix(in oklch, var(--gold) 15%, transparent)'
                : 'var(--down-bg)';
            const fg = acc >= 55 ? 'var(--up)' : acc >= 45 ? 'var(--gold)' : 'var(--down)';
            return (
              <span key={a.algorithm} style={{
                fontSize: 10, padding: '2px 6px',
                background: bg, color: fg,
                fontFamily: 'var(--font-mono)',
                whiteSpace: 'nowrap',
              }}>
                {algoLabel(a.algorithm)} {acc.toFixed(0)}%
              </span>
            );
          })}
        </div>
      </td>
    </tr>
  );
}

function PipelineSummaryTable({ markets }: { markets: MonitoringMarket[] }) {
  if (markets.length === 0) return null;
  const headerCell: React.CSSProperties = {
    textAlign: 'left', padding: '3px 8px', fontSize: 10,
    fontWeight: 500, color: 'var(--text-3)',
    textTransform: 'uppercase', letterSpacing: '0.5px',
    borderBottom: '1px solid var(--border)',
  };
  return (
    <div style={{ marginTop: 16, borderTop: '1px solid var(--border)', paddingTop: 12 }}>
      <table style={{ width: '100%', borderCollapse: 'collapse' }}>
        <thead>
          <tr>
            <th style={headerCell}>Market</th>
            <th style={headerCell}>Crawl</th>
            <th style={headerCell}>Data</th>
            <th style={{ ...headerCell, textAlign: 'right' }}>Preds</th>
            <th className="pipeline-hide-mobile" style={headerCell}>Top Algo</th>
          </tr>
        </thead>
        <tbody>
          {markets.map(m => <SummaryRow key={m.market} market={m} />)}
        </tbody>
      </table>
    </div>
  );
}

```

- [ ] **Step 3: Wire PipelineSummaryTable into the Panel**

In `Dashboard.tsx`, inside the monitoring Panel's JSX, find this block (around line 280–305):

```tsx
          <>
            <div style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(4, 1fr)',
              gap: 12,
            }}>
              {accessibleMarkets.map((m) => (
                <MarketStatusCard key={m.market} market={m} />
              ))}
            </div>
            {/* Footer summary */}
            <div style={{ marginTop: 10, ...
```

Insert `<PipelineSummaryTable markets={accessibleMarkets} />` between the closing `</div>` of the grid and the `{/* Footer summary */}` comment:

```tsx
          <>
            <div style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(4, 1fr)',
              gap: 12,
            }}>
              {accessibleMarkets.map((m) => (
                <MarketStatusCard key={m.market} market={m} />
              ))}
            </div>
            <PipelineSummaryTable markets={accessibleMarkets} />
            {/* Footer summary */}
            <div style={{ marginTop: 10, display: 'flex', gap: 20, fontSize: 11, color: 'var(--text-3)', flexWrap: 'wrap', alignItems: 'center' }}>
```

- [ ] **Step 4: Add mobile hide CSS to styles.css**

Open `frontend/src/styles.css` and find the last `@media` block (around line 809). Add the following **after** all existing rules, at the end of the file:

```css
/* Dashboard pipeline summary table — hide top-algo column on small screens */
@media (max-width: 640px) {
  .pipeline-hide-mobile { display: none; }
}
```

- [ ] **Step 5: Verify build compiles without errors**

```bash
cd /home/chronical/Projects/private/go-stock-prediction/frontend
npm run build 2>&1 | tail -15
```

Expected: no TypeScript errors. If there's a TS error about `React.CSSProperties`, add `import React from 'react';` or replace `React.CSSProperties` with the inline type `{ [key: string]: string | number }`.

- [ ] **Step 6: Visual verification**

```bash
# Rebuild and restart frontend container
cd /home/chronical/Projects/private/go-stock-prediction
docker-compose build frontend && docker-compose up -d frontend
```

Open browser at `http://localhost` (or `https://localhost`), log in, go to Dashboard. Verify:
- Each market card now shows a line like `11d · 44i` below the "Last crawl" timestamp
- Below the 4 market cards, a compact table appears with 4 rows
- Each row shows: market icon+name | crawl staleness + OK/STALE badge | Xd / Yi | prediction count | top-3 algo chips colored by accuracy
- Clicking a table row navigates to that market page
- Row hover shows `var(--surface-2)` background

- [ ] **Step 7: Commit**

```bash
cd /home/chronical/Projects/private/go-stock-prediction
git add frontend/src/pages/Dashboard.tsx frontend/src/styles.css
git commit -m "feat(dashboard): add pipeline summary table with per-market crawl and algo accuracy"
```
