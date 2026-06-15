# Bot Variants Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 9 param-variant bots per market×algo (495 new bots total), expose a `/variants` API endpoint, and add a "Variants" comparison tab on the bot detail page.

**Architecture:** The seeder (Python) inserts new bot rows with `_v2`…`_v10` suffixes — existing bots are untouched. A new Go API endpoint `GET /api/simulation/bots/{id}/variants` fetches all sibling bots (same market+algo) and computes KPIs for each by reusing the existing `computeKPIs` helper. The frontend adds a tab switcher to `SimulationBot.tsx` that renders a sortable comparison table.

**Tech Stack:** Python (seeder), Go (API handler + GORM), TypeScript/React (frontend tab)

---

## File Map

| File | Change |
|------|--------|
| `prediction/src/simulation/seeder.py` | Add `VARIANTS` list; extend `seed_bots()` to create 9 variants per combo |
| `api/pkg/store/repository/simulation.go` | Add `GetSimBotsByMarketAlgo` to `SimulationStore` interface |
| `api/pkg/store/mysql/simulation.go` | Implement `GetSimBotsByMarketAlgo` |
| `api/pkg/server/api_simulation.go` | Add `simBotVariant` DTO + `GetSimBotVariants` handler |
| `api/pkg/server/router.go` | Register `GET /api/simulation/bots/{id}/variants` before catch-all |
| `frontend/src/pages/SimulationBot.tsx` | Add tab state + Variants tab with sortable table |

---

## Task 1: Extend seeder with 9 param variants

**Files:**
- Modify: `prediction/src/simulation/seeder.py`

- [ ] **Step 1: Add VARIANTS list and extend seed_bots()**

Replace the content of `prediction/src/simulation/seeder.py` with:

```python
"""Seed sim_bots table with all trading bots (5 markets × 11 algorithms × 10 variants)."""
from __future__ import annotations

from decimal import Decimal

from src.database.connection import session_scope
from src.database.models import SimBot
from src.utils.logger import get_logger

log = get_logger("simulation.seeder")

ALGORITHMS = [
    ("moving_average", "Moving Average"),
    ("ema", "EMA/MACD"),
    ("lstm_nn", "LSTM"),
    ("arima_garch", "ARIMA-GARCH"),
    ("lightgbm", "LightGBM"),
    ("sarima", "SARIMA"),
    ("egarch", "EGARCH"),
    ("gru_nn", "GRU"),
    ("random_forest", "Random Forest"),
    ("xgboost", "XGBoost"),
    ("ensemble", "Ensemble"),
]

MARKETS = [
    ("VN30", "VN30", Decimal("1000000000"), "VND"),
    ("GOLD", "Gold", Decimal("1000"), "USD"),
    ("NASDAQ", "NASDAQ", Decimal("1000"), "USD"),
    ("SP500", "S&P 500", Decimal("1000"), "USD"),
    ("CRYPTO", "Crypto", Decimal("1000"), "USD"),
]

# Variant configs: (suffix, label, buy, sell, conf, sl, tp)
# suffix="" is the original/default bot (no suffix on ID).
VARIANTS = [
    ("",    "Default",         Decimal("0.50"), Decimal("0.30"), Decimal("0.40"), Decimal("5.00"),  Decimal("8.00")),
    ("_v2", "Conservative",    Decimal("1.00"), Decimal("0.80"), Decimal("0.60"), Decimal("5.00"),  Decimal("10.00")),
    ("_v3", "Aggressive",      Decimal("0.30"), Decimal("0.20"), Decimal("0.30"), Decimal("3.00"),  Decimal("5.00")),
    ("_v4", "High Confidence", Decimal("0.50"), Decimal("0.30"), Decimal("0.70"), Decimal("5.00"),  Decimal("8.00")),
    ("_v5", "Trend Follow",    Decimal("1.50"), Decimal("0.50"), Decimal("0.50"), Decimal("7.00"),  Decimal("15.00")),
    ("_v6", "Tight Exit",      Decimal("0.50"), Decimal("0.30"), Decimal("0.40"), Decimal("3.00"),  Decimal("5.00")),
    ("_v7", "Wide Exit",       Decimal("0.50"), Decimal("0.30"), Decimal("0.40"), Decimal("8.00"),  Decimal("15.00")),
    ("_v8", "Momentum",        Decimal("0.80"), Decimal("0.50"), Decimal("0.55"), Decimal("6.00"),  Decimal("12.00")),
    ("_v9", "Scalping",        Decimal("0.20"), Decimal("0.20"), Decimal("0.30"), Decimal("2.00"),  Decimal("3.00")),
    ("_v10","Swing",           Decimal("2.00"), Decimal("1.00"), Decimal("0.65"), Decimal("10.00"), Decimal("20.00")),
]


def seed_bots() -> int:
    """Insert sim_bots rows if they don't already exist. Returns count inserted."""
    inserted = 0

    with session_scope() as session:
        for market_key, market_display, initial_capital, currency in MARKETS:
            for algo_key, algo_display in ALGORITHMS:
                for suffix, variant_label, buy, sell, conf, sl, tp in VARIANTS:
                    bot_id = f"{market_key.lower()}_{algo_key}{suffix}"
                    if suffix:
                        display_name = f"{market_display} — {algo_display} ({variant_label})"
                    else:
                        display_name = f"{market_display} — {algo_display}"

                    existing = session.query(SimBot).filter(SimBot.id == bot_id).first()
                    if existing is None:
                        bot = SimBot(
                            id=bot_id,
                            market=market_key,
                            algorithm=algo_key,
                            display_name=display_name,
                            initial_capital=initial_capital,
                            currency=currency,
                            buy_threshold=buy,
                            sell_threshold=sell,
                            min_confidence=conf,
                            stop_loss=sl,
                            take_profit=tp,
                            max_position_pct=Decimal("15.00"),
                            max_positions=5,
                            is_active=True,
                        )
                        session.add(bot)
                        inserted += 1

        session.commit()

    log.info("sim.seeder.done", inserted=inserted)
    return inserted
```

- [ ] **Step 2: Verify seeder runs without error (dry-run count)**

```bash
docker exec prediction_service python -c "
from src.simulation.seeder import seed_bots
n = seed_bots()
print(f'Inserted: {n}')
"
```

Expected: `Inserted: 495` (first run) or `Inserted: 0` (already seeded). No exceptions.

- [ ] **Step 3: Verify row count in DB**

```bash
docker exec db mysql -uroot -p123 go_stock_prediction -e "SELECT COUNT(*) FROM sim_bots;"
```

Expected: `550`

- [ ] **Step 4: Commit**

```bash
git add prediction/src/simulation/seeder.py
git commit -m "feat(sim): add 9 param-variant bots per market×algo (550 bots total)"
```

---

## Task 2: Add GetSimBotsByMarketAlgo to repository interface + MySQL implementation

**Files:**
- Modify: `api/pkg/store/repository/simulation.go`
- Modify: `api/pkg/store/mysql/simulation.go`

- [ ] **Step 1: Add method to SimulationStore interface**

In `api/pkg/store/repository/simulation.go`, add one line to the `SimulationStore` interface after `GetActiveSimBots`:

```go
// GetSimBotsByMarketAlgo returns all bots for a given market and algorithm key.
GetSimBotsByMarketAlgo(market, algorithm string) ([]modelsdb.SimBot, error)
```

Full interface after edit:

```go
package repository

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
)

// SimulationStore — interface for trading simulation DB operations.
type SimulationStore interface {
	// Bots
	GetAllSimBots() ([]modelsdb.SimBot, error)
	GetSimBotByID(id string) (*modelsdb.SimBot, error)
	GetActiveSimBots() ([]modelsdb.SimBot, error)
	UpdateSimBotConfig(bot *modelsdb.SimBot) error
	// GetSimBotsByMarketAlgo returns all bots for a given market and algorithm key.
	GetSimBotsByMarketAlgo(market, algorithm string) ([]modelsdb.SimBot, error)

	// Sessions
	CreateSimSession(s *modelsdb.SimSession) error
	UpdateSimSession(s *modelsdb.SimSession) error
	GetLatestSimSession(botID string) (*modelsdb.SimSession, error)
	GetBestSimSessionForChart(botID string) (*modelsdb.SimSession, error)
	GetLatestLiveSimSession(botID string) (*modelsdb.SimSession, error)
	GetSimSessionsByBot(botID string, limit int) ([]modelsdb.SimSession, error)

	// Trades
	CreateSimTrade(t *modelsdb.SimTrade) error
	GetSimTrades(sessionID int64, offset, limit int) ([]modelsdb.SimTrade, int64, error)

	// Snapshots
	CreateSimPortfolioSnapshot(s *modelsdb.SimPortfolioSnapshot) error
	GetSimPortfolioSnapshots(sessionID int64) ([]modelsdb.SimPortfolioSnapshot, error)
}
```

- [ ] **Step 2: Implement in mysql package**

Add at the end of `api/pkg/store/mysql/simulation.go`:

```go
// GetSimBotsByMarketAlgo returns all bots with the given market and algorithm,
// ordered by ID for consistent display.
func (c *Client) GetSimBotsByMarketAlgo(market, algorithm string) ([]modelsdb.SimBot, error) {
	var bots []modelsdb.SimBot
	err := c.Db.Where("market = ? AND algorithm = ?", market, algorithm).
		Order("id ASC").
		Find(&bots).Error
	return bots, err
}
```

- [ ] **Step 3: Verify Go compiles**

```bash
cd api && go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add api/pkg/store/repository/simulation.go api/pkg/store/mysql/simulation.go
git commit -m "feat(api): add GetSimBotsByMarketAlgo to SimulationStore"
```

---

## Task 3: Add GetSimBotVariants handler

**Files:**
- Modify: `api/pkg/server/api_simulation.go`
- Modify: `api/pkg/server/router.go`

- [ ] **Step 1: Add simBotVariant DTO**

In `api/pkg/server/api_simulation.go`, add this struct after `simBotDetail` (around line 304):

```go
type simBotVariant struct {
	simBotJSON
	KPIs simBotKPIs `json:"kpis"`
}
```

- [ ] **Step 2: Add GetSimBotVariants handler**

Add this function after the `GetSimBot` handler (after line ~479):

```go
// GetSimBotVariants godoc
//
//	@Summary      Get variant bots for comparison
//	@Description  Returns all bots sharing the same market and algorithm as {id}, each with KPIs from their best backtest session.
//	@Tags         Simulation
//	@Produce      json
//	@Param        id   path      string  true  "Bot ID"
//	@Success      200  {array}   simBotVariant
//	@Failure      404  {object}  ResponseFailure
//	@Failure      500  {object}  ResponseFailure
//	@Router       /api/simulation/bots/{id}/variants [get]
func GetSimBotVariants(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		ResponseError(w, http.StatusBadRequest, "bot id is required")
		return
	}

	store := repository.GetSingleton()

	// Resolve the source bot to get market+algorithm.
	src, err := store.GetSimBotByID(id)
	if err != nil {
		ResponseError(w, http.StatusNotFound, "bot not found")
		return
	}

	siblings, err := store.GetSimBotsByMarketAlgo(src.Market, src.Algorithm)
	if err != nil {
		logger.Logger.Errorf("GetSimBotVariants: %v", err)
		ResponseError(w, http.StatusInternalServerError, "failed to fetch variants")
		return
	}

	variants := make([]simBotVariant, 0, len(siblings))
	for _, bot := range siblings {
		v := simBotVariant{simBotJSON: botToJSON(bot)}

		sess, sessErr := store.GetBestSimSessionForChart(bot.ID)
		if sessErr != nil || sess == nil {
			sess, sessErr = store.GetLatestSimSession(bot.ID)
		}
		if sessErr == nil && sess != nil {
			snaps, _ := store.GetSimPortfolioSnapshots(sess.ID)
			trades, _, _ := store.GetSimTrades(sess.ID, 0, 100000)
			v.KPIs = computeKPIs(snaps, trades)
		}

		variants = append(variants, v)
	}

	ResponseSuccess(w, http.StatusOK, variants)
}
```

- [ ] **Step 3: Register route in router.go**

In `api/pkg/server/router.go`, add this line **before** the existing `GET /api/simulation/bots/` catch-all (around line 145):

```go
mux.HandleFunc("GET /api/simulation/bots/{id}/variants", GetSimBotVariants)
```

The block should look like:

```go
mux.HandleFunc("GET /api/simulation/bots/{id}/trades", GetSimBotTrades)
mux.HandleFunc("GET /api/simulation/bots/{id}/chart", GetSimBotChart)
mux.HandleFunc("GET /api/simulation/bots/{id}/variants", GetSimBotVariants)   // ← add here
mux.HandleFunc("PUT /api/simulation/bots/{id}/config", AuthRequired(UpdateSimBotConfig))
mux.HandleFunc("POST /api/simulation/bots/{id}/toggle", AuthRequired(ToggleSimBot))
mux.HandleFunc("POST /api/simulation/bots/{id}/run", AuthRequired(TriggerSimBotRun))
mux.HandleFunc("POST /api/simulation/run-all", AuthRequired(TriggerSimRunAll))
// GET /api/simulation/bots/{id} must come last (catch-all for bot detail)
mux.HandleFunc("GET /api/simulation/bots/", GetSimBot)
```

Also add log line in the startup log block:
```go
logger.Logger.Info("  GET  /api/simulation/bots/{id}/variants")
```

- [ ] **Step 4: Build and verify**

```bash
cd api && go build ./...
```

Expected: no errors.

- [ ] **Step 5: Smoke test endpoint**

```bash
TOKEN=$(curl -s -X POST http://localhost:8118/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

curl -s "http://localhost:8118/api/simulation/bots/nasdaq_lstm_nn/variants" \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool | head -40
```

Expected: JSON array of 10 objects, each with `id`, `market`, `algorithm`, `buy_threshold`, `kpis.total_return_pct`, etc.

- [ ] **Step 6: Commit**

```bash
git add api/pkg/server/api_simulation.go api/pkg/server/router.go
git commit -m "feat(api): add GET /api/simulation/bots/{id}/variants endpoint"
```

---

## Task 4: Frontend — Variants tab in SimulationBot.tsx

**Files:**
- Modify: `frontend/src/pages/SimulationBot.tsx`

- [ ] **Step 1: Add state and fetch**

In the state declarations block (around line 187), add:

```tsx
const [activeTab, setActiveTab] = useState<'detail' | 'variants'>('detail');
const [variants, setVariants] = useState<VariantBot[]>([]);
const [variantSort, setVariantSort] = useState<{ col: string; dir: 'asc' | 'desc' }>({ col: 'return', dir: 'desc' });
```

Add the `VariantBot` interface near the top of the file with the other interfaces:

```tsx
interface VariantKPIs {
  total_return_pct: number;
  sharpe_ratio: number;
  win_rate_pct: number;
  max_drawdown_pct: number;
  profit_factor: number;
  total_trades: number;
}

interface VariantBot {
  id: string;
  display_name: string;
  buy_threshold: number | null;
  sell_threshold: number | null;
  min_confidence: number | null;
  stop_loss: number | null;
  take_profit: number | null;
  kpis: VariantKPIs;
}
```

Add a `useEffect` to fetch variants when `botId` is available (add after the liveChart useEffect, around line 237):

```tsx
useEffect(() => {
  if (!botId) return;
  apiFetch(`/api/simulation/bots/${encodeURIComponent(botId)}/variants`)
    .then((data: VariantBot[]) => setVariants(Array.isArray(data) ? data : []))
    .catch(() => setVariants([]));
}, [botId]);
```

- [ ] **Step 2: Replace the header tab area**

Find the existing header section that contains the back button + bot name (around line 304). Replace the `<h2>` section-gap header for trades (around line 604) — but first, add a tab bar **inside the page's main header** just below the bot name row. Locate this block:

```tsx
{/* Header */}
<div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 20, flexWrap: 'wrap' }}>
```

Change `marginBottom: 20` to `marginBottom: 12` and add the tab bar immediately after the closing `</div>` of that flex row:

```tsx
{/* Tab bar */}
<div style={{ display: 'flex', gap: 0, borderBottom: '1px solid var(--border)', marginBottom: 20 }}>
  <button
    onClick={() => setActiveTab('detail')}
    style={{
      padding: '8px 18px', fontSize: 13, fontWeight: activeTab === 'detail' ? 700 : 400,
      color: activeTab === 'detail' ? 'var(--text-1)' : 'var(--text-3)',
      borderBottom: activeTab === 'detail' ? '2px solid var(--accent, #58a6ff)' : '2px solid transparent',
      background: 'none', border: 'none', borderBottom: activeTab === 'detail' ? '2px solid var(--accent, #58a6ff)' : '2px solid transparent',
      cursor: 'pointer',
    }}
  >
    Chi tiết
  </button>
  <button
    onClick={() => setActiveTab('variants')}
    style={{
      padding: '8px 18px', fontSize: 13, fontWeight: activeTab === 'variants' ? 700 : 400,
      color: activeTab === 'variants' ? 'var(--text-1)' : 'var(--text-3)',
      background: 'none', border: 'none', borderBottom: activeTab === 'variants' ? '2px solid var(--accent, #58a6ff)' : '2px solid transparent',
      cursor: 'pointer',
    }}
  >
    Variants ({variants.length || 10})
  </button>
</div>
```

- [ ] **Step 3: Wrap detail content in conditional render**

Find the first section after the tab bar (the live session banner / KPI grid). Wrap all existing content sections (KPI grid, chart, bot config, trades) in:

```tsx
{activeTab === 'detail' && (
  <>
    {/* … all existing detail sections … */}
  </>
)}
```

- [ ] **Step 4: Add Variants tab content**

After the `{activeTab === 'detail' && ...}` block, add:

```tsx
{activeTab === 'variants' && (
  <VariantsTab
    variants={variants}
    currentBotId={botId || ''}
    sort={variantSort}
    onSort={(col) =>
      setVariantSort(s => ({ col, dir: s.col === col && s.dir === 'desc' ? 'asc' : 'desc' }))
    }
    currency={currency}
    onNavigate={(id) => navigate(`/simulation/${id}`)}
  />
)}
```

- [ ] **Step 5: Add VariantsTab component**

Add this component **before** the `export default function SimulationBot()` declaration:

```tsx
function SortIcon({ active, dir }: { active: boolean; dir: 'asc' | 'desc' }) {
  if (!active) return <span style={{ color: 'var(--text-3)', fontSize: 10 }}> ⇅</span>;
  return <span style={{ color: 'var(--accent, #58a6ff)', fontSize: 10 }}>{dir === 'desc' ? ' ↓' : ' ↑'}</span>;
}

function VariantsTab({
  variants, currentBotId, sort, onSort, currency, onNavigate,
}: {
  variants: VariantBot[];
  currentBotId: string;
  sort: { col: string; dir: 'asc' | 'desc' };
  onSort: (col: string) => void;
  currency: string;
  onNavigate: (id: string) => void;
}) {
  const cols: { key: string; label: string; right?: boolean }[] = [
    { key: 'name', label: 'Variant' },
    { key: 'buy', label: 'Buy', right: true },
    { key: 'sell', label: 'Sell', right: true },
    { key: 'conf', label: 'Conf', right: true },
    { key: 'sl', label: 'SL', right: true },
    { key: 'tp', label: 'TP', right: true },
    { key: 'return', label: 'Return%', right: true },
    { key: 'sharpe', label: 'Sharpe', right: true },
    { key: 'win', label: 'Win%', right: true },
    { key: 'trades', label: 'Trades', right: true },
  ];

  const sorted = [...variants].sort((a, b) => {
    const dir = sort.dir === 'desc' ? -1 : 1;
    switch (sort.col) {
      case 'buy':    return ((a.buy_threshold ?? 0) - (b.buy_threshold ?? 0)) * dir;
      case 'sell':   return ((a.sell_threshold ?? 0) - (b.sell_threshold ?? 0)) * dir;
      case 'conf':   return ((a.min_confidence ?? 0) - (b.min_confidence ?? 0)) * dir;
      case 'sl':     return ((a.stop_loss ?? 0) - (b.stop_loss ?? 0)) * dir;
      case 'tp':     return ((a.take_profit ?? 0) - (b.take_profit ?? 0)) * dir;
      case 'return': return (a.kpis.total_return_pct - b.kpis.total_return_pct) * dir;
      case 'sharpe': return (a.kpis.sharpe_ratio - b.kpis.sharpe_ratio) * dir;
      case 'win':    return (a.kpis.win_rate_pct - b.kpis.win_rate_pct) * dir;
      case 'trades': return (a.kpis.total_trades - b.kpis.total_trades) * dir;
      default:       return a.display_name.localeCompare(b.display_name) * dir;
    }
  });

  if (variants.length === 0) {
    return (
      <div className="empty section-gap" style={{ padding: 48 }}>
        <p style={{ color: 'var(--text-3)' }}>Chưa có variant nào.</p>
      </div>
    );
  }

  return (
    <div className="section-gap">
      <div style={{ overflowX: 'auto' }}>
        <table className="tbl">
          <thead>
            <tr>
              {cols.map(c => (
                <th
                  key={c.key}
                  className={c.right ? 'r' : ''}
                  style={{ cursor: 'pointer', userSelect: 'none', whiteSpace: 'nowrap' }}
                  onClick={() => onSort(c.key)}
                >
                  {c.label}
                  <SortIcon active={sort.col === c.key} dir={sort.dir} />
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {sorted.map(v => {
              const isCurrent = v.id === currentBotId;
              return (
                <tr
                  key={v.id}
                  style={{ background: isCurrent ? 'var(--surface-2)' : undefined, cursor: 'pointer' }}
                  onClick={() => !isCurrent && onNavigate(v.id)}
                >
                  <td>
                    <span
                      style={{
                        color: isCurrent ? 'var(--accent, #58a6ff)' : 'var(--text-1)',
                        fontWeight: isCurrent ? 700 : 400,
                        fontSize: 13,
                      }}
                    >
                      {v.display_name.split('—').pop()?.trim() || v.display_name}
                      {isCurrent && <span style={{ color: 'var(--text-3)', fontWeight: 400, marginLeft: 6 }}>← đây</span>}
                    </span>
                  </td>
                  <td className="r num" style={{ fontSize: 12 }}>{v.buy_threshold != null ? v.buy_threshold.toFixed(1) + '%' : '—'}</td>
                  <td className="r num" style={{ fontSize: 12 }}>{v.sell_threshold != null ? v.sell_threshold.toFixed(1) + '%' : '—'}</td>
                  <td className="r num" style={{ fontSize: 12 }}>{v.min_confidence != null ? (v.min_confidence * 100).toFixed(0) + '%' : '—'}</td>
                  <td className="r num" style={{ fontSize: 12 }}>{v.stop_loss != null ? v.stop_loss.toFixed(1) + '%' : '—'}</td>
                  <td className="r num" style={{ fontSize: 12 }}>{v.take_profit != null ? v.take_profit.toFixed(1) + '%' : '—'}</td>
                  <td className="r">
                    <span className="num" style={{ fontSize: 12, color: v.kpis.total_return_pct >= 0 ? 'var(--up)' : 'var(--down)' }}>
                      {v.kpis.total_return_pct >= 0 ? '+' : ''}{v.kpis.total_return_pct.toFixed(2)}%
                    </span>
                  </td>
                  <td className="r num" style={{ fontSize: 12 }}>{v.kpis.sharpe_ratio.toFixed(2)}</td>
                  <td className="r num" style={{ fontSize: 12 }}>{v.kpis.win_rate_pct.toFixed(1)}%</td>
                  <td className="r num" style={{ fontSize: 12 }}>{v.kpis.total_trades}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
```

- [ ] **Step 6: Fix min_confidence display**

Note: `min_confidence` from the API comes as a decimal (e.g. `0.40` for 40%). The `VariantsTab` renders it as `(v.min_confidence * 100).toFixed(0) + '%'` which is correct. The bot config section already shows `(bot.min_confidence ?? 0) * 100` — verify this is consistent.

- [ ] **Step 7: Build frontend**

```bash
cd frontend && npm run build 2>&1 | tail -20
```

Expected: Build succeeds with no TypeScript errors.

- [ ] **Step 8: Verify in browser**

Navigate to any bot detail page (e.g. `http://localhost/simulation/nasdaq_lstm_nn`). Confirm:
- "Chi tiết" and "Variants (10)" tabs appear
- "Chi tiết" tab shows existing KPIs + chart + trades (unchanged)
- "Variants (10)" tab shows table with 10 rows
- Current bot row is highlighted + shows "← đây"
- Click another row navigates to that bot's page
- Click column headers sorts the table

- [ ] **Step 9: Commit**

```bash
git add frontend/src/pages/SimulationBot.tsx
git commit -m "feat(frontend): add Variants comparison tab to bot detail page"
```

---

## Task 5: Rebuild Docker and run backtest for new bots

- [ ] **Step 1: Rebuild prediction container (seeder runs on startup)**

```bash
docker-compose build prediction && docker-compose up -d prediction
```

Wait ~30 seconds for service to start, then verify:

```bash
docker exec db mysql -uroot -p123 go_stock_prediction -e "SELECT COUNT(*) as total FROM sim_bots;"
```

Expected: `550`

- [ ] **Step 2: Rebuild API container**

```bash
docker-compose build api && docker-compose up -d api
```

- [ ] **Step 3: Rebuild frontend container**

```bash
docker-compose build frontend && docker-compose up -d frontend
```

- [ ] **Step 4: Run backtest for all bots to generate session data**

```bash
TOKEN=$(curl -s -X POST http://localhost/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

curl -s -X POST http://localhost/api/trigger/simulation-backtest \
  -H "Authorization: Bearer $TOKEN"
```

This runs in background. Monitor completion:

```bash
docker-compose logs -f prediction 2>&1 | grep -E "backtest|sim\."
```

- [ ] **Step 5: Final verification**

```bash
# Variants endpoint returns 10 items
curl -s "http://localhost/api/simulation/bots/nasdaq_lstm_nn/variants" | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'Count: {len(d)}, top bot: {d[0][\"id\"]}')"

# Leaderboard shows 550 bots
curl -s "http://localhost/api/simulation/leaderboard" | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'Total bots: {d[\"summary\"][\"total_bots\"]}')"
```

Expected: `Count: 10` and `Total bots: 550`
