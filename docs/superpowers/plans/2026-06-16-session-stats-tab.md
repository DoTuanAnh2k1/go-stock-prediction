# Session Stats Tab Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Thêm tab "Session Stats" vào trang NASDAQ và SP500, hiển thị direction accuracy + bot trading performance trong phiên giao dịch gần nhất (20:00–03:30 ICT).

**Architecture:** Backend tính session window trong Go (ICT timezone), query DB theo window đó, trả về 1 endpoint `GET /api/markets/{key}/session-stats`. Frontend thêm route mới + component `SessionStats.tsx` được dùng chung cho cả NASDAQ và SP500.

**Tech Stack:** Go (net/http, GORM raw SQL, shopspring/decimal), React + TypeScript, existing `Panel/KPI/Icon` UI components.

---

## File Map

**Tạo mới:**
- `api/pkg/models/models_api/session_stats_dto.go` — response DTOs
- `api/pkg/store/repository/session_stats.go` — `SessionStatsStore` interface
- `api/pkg/store/mysql/session_stats.go` — MySQL implementation (2 raw SQL queries)
- `api/pkg/server/api_session_stats.go` — HTTP handler + session window logic
- `frontend/src/pages/SessionStats.tsx` — React page dùng chung cho NASDAQ và SP500

**Sửa:**
- `api/pkg/store/repository/repository.go` — embed `SessionStatsStore` vào `DatabaseStore`
- `api/pkg/server/router.go` — đăng ký route + log
- `frontend/src/App.tsx` — thêm route `/markets/:marketKey/session-stats`
- `frontend/src/pages/Nasdaq.tsx` — thêm tab "Session Stats" vào `MarketTabs`
- `frontend/src/pages/SP500.tsx` — thêm tab "Session Stats" vào `MarketTabs`

---

## Task 1: DTOs — Response types

**Files:**
- Create: `api/pkg/models/models_api/session_stats_dto.go`

- [ ] **Tạo file DTO**

```go
// api/pkg/models/models_api/session_stats_dto.go
package modelsapi

import "time"

// SessionWindow mô tả phiên giao dịch được truy vấn.
type SessionWindow struct {
	Start  time.Time `json:"start"`
	End    time.Time `json:"end"`
	IsOpen bool      `json:"is_open"`
}

// SessionDirAccRow — direction accuracy của 1 thuật toán trong phiên.
type SessionDirAccRow struct {
	Algorithm string  `json:"algorithm"`
	Total     int64   `json:"total"`
	Correct   int64   `json:"correct"`
	Accuracy  float64 `json:"accuracy"`
}

// SessionBotRow — bot trading stats của 1 thuật toán trong phiên.
type SessionBotRow struct {
	Algorithm      string  `json:"algorithm"`
	Trades         int64   `json:"trades"`
	Wins           int64   `json:"wins"`
	Losses         int64   `json:"losses"`
	Breakeven      int64   `json:"breakeven"`
	TotalPnL       float64 `json:"total_pnl"`
	AvgPnLPerTrade float64 `json:"avg_pnl_per_trade"`
	WinRate        float64 `json:"win_rate"`
}

// SessionStatsResponse — full response của GET /api/markets/{key}/session-stats.
type SessionStatsResponse struct {
	Market            string             `json:"market"`
	Session           SessionWindow      `json:"session"`
	DirectionAccuracy []SessionDirAccRow `json:"direction_accuracy"`
	BotTrades         []SessionBotRow    `json:"bot_trades"`
}
```

- [ ] **Commit**

```bash
git add api/pkg/models/models_api/session_stats_dto.go
git commit -m "feat(api): add SessionStats DTOs"
```

---

## Task 2: Store interface

**Files:**
- Create: `api/pkg/store/repository/session_stats.go`
- Modify: `api/pkg/store/repository/repository.go`

- [ ] **Tạo interface**

```go
// api/pkg/store/repository/session_stats.go
package repository

import (
	"time"

	modelsapi "go-stock-prediction/pkg/models/models_api"
)

// SessionStatsStore cung cấp queries thống kê theo session window.
type SessionStatsStore interface {
	// GetSessionDirAccuracy trả về direction accuracy per algorithm trong [from, to).
	// market: "NASDAQ" | "SP500" | "GOLD" | "CRYPTO"
	GetSessionDirAccuracy(market string, from, to time.Time) ([]modelsapi.SessionDirAccRow, error)

	// GetSessionBotTrades trả về bot trading stats per algorithm trong [from, to).
	// market: "NASDAQ" | "SP500" | "GOLD" | "CRYPTO"
	GetSessionBotTrades(market string, from, to time.Time) ([]modelsapi.SessionBotRow, error)
}
```

- [ ] **Embed vào DatabaseStore** — sửa `api/pkg/store/repository/repository.go`, thêm `SessionStatsStore` vào danh sách embedded interfaces (sau `MonitoringStore`):

```go
// Trong DatabaseStore interface, thêm sau MonitoringStore:
SessionStatsStore
```

- [ ] **Commit**

```bash
git add api/pkg/store/repository/session_stats.go api/pkg/store/repository/repository.go
git commit -m "feat(api): add SessionStatsStore interface"
```

---

## Task 3: MySQL implementation

**Files:**
- Create: `api/pkg/store/mysql/session_stats.go`

- [ ] **Tạo implementation**

```go
// api/pkg/store/mysql/session_stats.go
package mysql

import (
	"fmt"
	"strings"
	"time"

	modelsapi "go-stock-prediction/pkg/models/models_api"
)

// predTableMap maps market key → prediction table name.
var predTableMap = map[string]string{
	"GOLD":   "gold_predictions",
	"NASDAQ": "nasdaq_predictions",
	"CRYPTO": "crypto_predictions",
	"SP500":  "sp500_predictions",
}

// GetSessionDirAccuracy returns per-algorithm direction accuracy for predictions
// whose prediction_date falls in [from, to).
func (c *Client) GetSessionDirAccuracy(market string, from, to time.Time) ([]modelsapi.SessionDirAccRow, error) {
	tbl, ok := predTableMap[strings.ToUpper(market)]
	if !ok {
		return nil, fmt.Errorf("GetSessionDirAccuracy: unknown market %q", market)
	}

	query := fmt.Sprintf(`
		SELECT
			algorithm_name AS algorithm,
			COUNT(*)                                                    AS total,
			SUM(CASE WHEN direction_correct = 1 THEN 1 ELSE 0 END)    AS correct
		FROM %s
		WHERE prediction_date >= ? AND prediction_date < ?
		  AND direction_correct IS NOT NULL
		  AND deleted_at IS NULL
		GROUP BY algorithm_name
		ORDER BY algorithm_name
	`, tbl)

	type raw struct {
		Algorithm string
		Total     int64
		Correct   int64
	}
	var rows []raw
	if err := c.Db.Raw(query, from, to).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("GetSessionDirAccuracy(%s): %w", market, err)
	}

	result := make([]modelsapi.SessionDirAccRow, len(rows))
	for i, r := range rows {
		acc := 0.0
		if r.Total > 0 {
			acc = float64(r.Correct) / float64(r.Total)
		}
		result[i] = modelsapi.SessionDirAccRow{
			Algorithm: r.Algorithm,
			Total:     r.Total,
			Correct:   r.Correct,
			Accuracy:  acc,
		}
	}
	return result, nil
}

// GetSessionBotTrades returns per-algorithm bot trading stats for SELL trades
// whose trade_date falls in [from, to).
func (c *Client) GetSessionBotTrades(market string, from, to time.Time) ([]modelsapi.SessionBotRow, error) {
	query := `
		SELECT
			sb.algorithm,
			COUNT(*)                                                    AS trades,
			SUM(CASE WHEN st.pnl > 0 THEN 1 ELSE 0 END)              AS wins,
			SUM(CASE WHEN st.pnl < 0 THEN 1 ELSE 0 END)              AS losses,
			SUM(CASE WHEN st.pnl = 0 THEN 1 ELSE 0 END)              AS breakeven,
			COALESCE(SUM(st.pnl), 0)                                   AS total_pnl,
			COALESCE(AVG(st.pnl), 0)                                   AS avg_pnl_per_trade
		FROM sim_trades st
		JOIN sim_bots sb ON st.bot_id = sb.id
		WHERE sb.market  = ?
		  AND st.action  = 'SELL'
		  AND st.pnl     IS NOT NULL
		  AND st.trade_date >= ? AND st.trade_date < ?
		GROUP BY sb.algorithm
		ORDER BY SUM(st.pnl) DESC
	`

	type raw struct {
		Algorithm      string
		Trades         int64
		Wins           int64
		Losses         int64
		Breakeven      int64
		TotalPnL       float64
		AvgPnLPerTrade float64
	}
	var rows []raw
	if err := c.Db.Raw(query, strings.ToUpper(market), from, to).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("GetSessionBotTrades(%s): %w", market, err)
	}

	result := make([]modelsapi.SessionBotRow, len(rows))
	for i, r := range rows {
		wr := 0.0
		if r.Trades > 0 {
			wr = float64(r.Wins) / float64(r.Trades)
		}
		result[i] = modelsapi.SessionBotRow{
			Algorithm:      r.Algorithm,
			Trades:         r.Trades,
			Wins:           r.Wins,
			Losses:         r.Losses,
			Breakeven:      r.Breakeven,
			TotalPnL:       r.TotalPnL,
			AvgPnLPerTrade: r.AvgPnLPerTrade,
			WinRate:        wr,
		}
	}
	return result, nil
}
```

- [ ] **Build để kiểm tra compile**

```bash
cd /home/chronical/Projects/private/go-stock-prediction/api && go build ./...
```
Expected: no errors.

- [ ] **Commit**

```bash
git add api/pkg/store/mysql/session_stats.go
git commit -m "feat(api): implement SessionStats MySQL queries"
```

---

## Task 4: HTTP handler + session window logic

**Files:**
- Create: `api/pkg/server/api_session_stats.go`

- [ ] **Tạo handler**

```go
// api/pkg/server/api_session_stats.go
package server

import (
	"net/http"
	"time"

	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	"go-stock-prediction/pkg/store/repository"
)

// ict là múi giờ Asia/Ho_Chi_Minh dùng để tính session window.
var ict = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		return time.UTC
	}
	return loc
}()

// nasdaqSession trả về (start, end, isOpen) của phiên NASDAQ/SP500 gần nhất.
// Phiên = 20:00 ICT hôm nay → 03:30 ICT hôm sau.
// Nếu giờ hiện tại nằm trong [20:00, 23:59] → phiên đang mở (bắt đầu hôm nay).
// Nếu giờ hiện tại nằm trong [00:00, 03:30] → phiên đang mở (bắt đầu hôm qua).
// Nếu giờ hiện tại nằm trong (03:30, 20:00) → phiên vừa xong (hôm qua 20:00 → hôm nay 03:30).
func nasdaqSession(now time.Time) (start, end time.Time, isOpen bool) {
	n := now.In(ict)
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, ict)

	open := today.Add(20 * time.Hour)             // 20:00 hôm nay
	close := today.Add(3*time.Hour + 30*time.Minute) // 03:30 hôm nay

	switch {
	case !n.Before(open):
		// >= 20:00 → phiên đang mở, bắt đầu hôm nay
		start = open
		end = close.Add(24 * time.Hour) // 03:30 ngày mai
		isOpen = true
	case n.Before(close):
		// < 03:30 → phiên đang mở, bắt đầu hôm qua
		start = open.Add(-24 * time.Hour) // 20:00 hôm qua
		end = close
		isOpen = true
	default:
		// 03:30–20:00 → phiên vừa xong
		start = open.Add(-24 * time.Hour) // 20:00 hôm qua
		end = close
		isOpen = false
	}
	return
}

// GetMarketSessionStats godoc
//
//	@Summary      Session stats for a market
//	@Description  Returns direction accuracy and bot trading stats for the current or most recent trading session.
//	@Tags         Markets
//	@Produce      json
//	@Param        key  path  string  true  "Market key: nasdaq100 or sp500"
//	@Success      200  {object}  modelsapi.SessionStatsResponse
//	@Failure      400  {object}  ResponseFailure
//	@Failure      403  {object}  ResponseFailure
//	@Failure      500  {object}  ResponseFailure
//	@Router       /api/markets/{key}/session-stats [get]
func GetMarketSessionStats(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	marketKey := pathToMarketKey(key)

	if marketKey != "NASDAQ" && marketKey != "SP500" {
		ResponseError(w, http.StatusBadRequest, "session-stats only supported for nasdaq100 and sp500")
		return
	}
	if !checkMarketAccess(w, r, marketKey) {
		return
	}

	start, end, isOpen := nasdaqSession(time.Now())
	store := repository.GetSingleton()

	dirAcc, err := store.GetSessionDirAccuracy(marketKey, start, end)
	if err != nil {
		logger.Logger.Errorf("[session-stats/%s] dir accuracy: %v", key, err)
		ResponseError(w, http.StatusInternalServerError, "failed to get direction accuracy")
		return
	}
	if dirAcc == nil {
		dirAcc = []modelsapi.SessionDirAccRow{}
	}

	botTrades, err := store.GetSessionBotTrades(marketKey, start, end)
	if err != nil {
		logger.Logger.Errorf("[session-stats/%s] bot trades: %v", key, err)
		ResponseError(w, http.StatusInternalServerError, "failed to get bot trades")
		return
	}
	if botTrades == nil {
		botTrades = []modelsapi.SessionBotRow{}
	}

	ResponseSuccess(w, http.StatusOK, modelsapi.SessionStatsResponse{
		Market: marketKey,
		Session: modelsapi.SessionWindow{
			Start:  start,
			End:    end,
			IsOpen: isOpen,
		},
		DirectionAccuracy: dirAcc,
		BotTrades:         botTrades,
	})
}
```

- [ ] **Build**

```bash
cd /home/chronical/Projects/private/go-stock-prediction/api && go build ./...
```
Expected: no errors.

- [ ] **Commit**

```bash
git add api/pkg/server/api_session_stats.go
git commit -m "feat(api): add GetMarketSessionStats handler with session window logic"
```

---

## Task 5: Đăng ký route

**Files:**
- Modify: `api/pkg/server/router.go`

- [ ] **Thêm route** — trong `router.go`, thêm sau dòng `mux.HandleFunc("/api/markets/{key}/training", ...)`:

```go
mux.HandleFunc("/api/markets/{key}/session-stats", AuthRequired(GetMarketSessionStats))
```

- [ ] **Thêm log** — trong phần log routes, sau dòng log training:

```go
logger.Logger.Info("  GET  /api/markets/{key}/session-stats [auth+market]")
```

- [ ] **Build + chạy nhanh**

```bash
cd /home/chronical/Projects/private/go-stock-prediction/api && go build ./...
```
Expected: no errors.

- [ ] **Commit**

```bash
git add api/pkg/server/router.go
git commit -m "feat(api): register /api/markets/{key}/session-stats route"
```

---

## Task 6: Frontend — SessionStats page

**Files:**
- Create: `frontend/src/pages/SessionStats.tsx`

- [ ] **Tạo component**

```tsx
// frontend/src/pages/SessionStats.tsx
import { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import { Panel, Icon } from '../components/ui';
import { useAuth } from '../context/AuthContext';

// ── Types ────────────────────────────────────────────────────────────────────
interface SessionWindow {
  start: string;
  end: string;
  is_open: boolean;
}
interface DirAccRow {
  algorithm: string;
  total: number;
  correct: number;
  accuracy: number;
}
interface BotRow {
  algorithm: string;
  trades: number;
  wins: number;
  losses: number;
  breakeven: number;
  total_pnl: number;
  avg_pnl_per_trade: number;
  win_rate: number;
}
interface SessionStats {
  market: string;
  session: SessionWindow;
  direction_accuracy: DirAccRow[];
  bot_trades: BotRow[];
}

// ── Helpers ──────────────────────────────────────────────────────────────────
const ALGO_LABEL: Record<string, string> = {
  lstm_nn: 'LSTM', gru_nn: 'GRU', ensemble: 'Ensemble',
  arima_garch: 'ARIMA', sarima: 'SARIMA', egarch: 'EGARCH',
  moving_average: 'MA', ema: 'EMA', lightgbm: 'LightGBM',
  xgboost: 'XGBoost', random_forest: 'RF',
};
function algoLabel(k: string) { return ALGO_LABEL[k] || k; }

function fmtDT(s: string) {
  if (!s) return '—';
  const d = new Date(s);
  if (isNaN(d.getTime())) return s.slice(0, 16).replace('T', ' ');
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${pad(d.getDate())}/${pad(d.getMonth()+1)} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function pct(v: number) { return (v * 100).toFixed(1) + '%'; }

function AccBar({ value }: { value: number }) {
  const color = value >= 0.6 ? 'var(--up)' : value >= 0.45 ? 'var(--gold)' : 'var(--down)';
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
      <div style={{ flex: 1, height: 6, background: 'var(--border)', borderRadius: 3 }}>
        <div style={{ width: pct(value), height: '100%', background: color, borderRadius: 3 }} />
      </div>
      <span style={{ minWidth: 40, textAlign: 'right', color, fontWeight: 600, fontSize: 13 }}>
        {pct(value)}
      </span>
    </div>
  );
}

function PnlCell({ v }: { v: number }) {
  const color = v > 0 ? 'var(--up)' : v < 0 ? 'var(--down)' : 'var(--text-3)';
  return <span style={{ color, fontWeight: 600 }}>{v > 0 ? '+' : ''}{v.toFixed(2)}</span>;
}

async function apiFetch(path: string) {
  const token = localStorage.getItem('vns_token') || '';
  const res = await fetch(path, { headers: { Accept: 'application/json', Authorization: `Bearer ${token}` } });
  if (!res.ok) throw new Error('HTTP ' + res.status);
  return res.json();
}

// ── Component ─────────────────────────────────────────────────────────────────
export default function SessionStats() {
  const { marketKey } = useParams<{ marketKey: string }>();
  const { user, canAccessMarket } = useAuth();

  const canonicalKey = marketKey === 'nasdaq100' ? 'NASDAQ' : 'SP500';

  const [data, setData] = useState<SessionStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    if (user && !canAccessMarket(canonicalKey)) { setLoading(false); return; }
    setLoading(true);
    apiFetch(`/api/markets/${marketKey}/session-stats`)
      .then((d) => { setData(d?.data ?? d); setLoading(false); })
      .catch((e) => { setError(e.message); setLoading(false); });
  }, [marketKey]);

  if (loading) return (
    <div className="empty section-gap">
      <div className="empty__icon"><Icon name="layers" size={18} /></div>
      <p>Đang tải thống kê phiên...</p>
    </div>
  );

  if (error || !data) return (
    <div className="empty section-gap">
      <div className="empty__icon"><Icon name="layers" size={18} /></div>
      <p>{error || 'Không có dữ liệu'}</p>
    </div>
  );

  const { session, direction_accuracy, bot_trades } = data;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      {/* Session info banner */}
      <div style={{
        display: 'flex', alignItems: 'center', gap: 12,
        padding: '10px 16px', borderRadius: 8,
        background: session.is_open ? 'color-mix(in oklch, var(--up) 12%, transparent)' : 'var(--surface-2)',
        border: '1px solid ' + (session.is_open ? 'var(--up)' : 'var(--border)'),
        fontSize: 13,
      }}>
        <span style={{
          padding: '2px 8px', borderRadius: 4, fontSize: 11, fontWeight: 700,
          background: session.is_open ? 'var(--up)' : 'var(--text-3)',
          color: '#fff',
        }}>
          {session.is_open ? 'ĐANG MỞ' : 'ĐÃ ĐÓNG'}
        </span>
        <span style={{ color: 'var(--text-2)' }}>
          Phiên: <strong>{fmtDT(session.start)}</strong> → <strong>{fmtDT(session.end)}</strong>
        </span>
      </div>

      {/* Direction Accuracy */}
      <Panel title="Direction Accuracy" dot={canonicalKey}>
        {direction_accuracy.length === 0 ? (
          <p style={{ color: 'var(--text-3)', padding: '16px 0', fontSize: 13 }}>
            Chưa có dự đoán đã reconcile trong phiên này.
          </p>
        ) : (
          <table className="tbl" style={{ width: '100%' }}>
            <thead>
              <tr>
                <th>Thuật toán</th>
                <th style={{ textAlign: 'right' }}>Đã reconcile</th>
                <th style={{ textAlign: 'right' }}>Đúng</th>
                <th style={{ minWidth: 160 }}>Accuracy</th>
              </tr>
            </thead>
            <tbody>
              {[...direction_accuracy]
                .sort((a, b) => b.accuracy - a.accuracy)
                .map((row) => (
                  <tr key={row.algorithm}>
                    <td><strong>{algoLabel(row.algorithm)}</strong></td>
                    <td style={{ textAlign: 'right', color: 'var(--text-2)' }}>{row.total}</td>
                    <td style={{ textAlign: 'right', color: 'var(--text-2)' }}>{row.correct}</td>
                    <td><AccBar value={row.accuracy} /></td>
                  </tr>
                ))}
            </tbody>
          </table>
        )}
      </Panel>

      {/* Bot Trades */}
      <Panel title="Bot Trading" dot={canonicalKey}>
        {bot_trades.length === 0 ? (
          <p style={{ color: 'var(--text-3)', padding: '16px 0', fontSize: 13 }}>
            Chưa có giao dịch bot nào trong phiên này.
          </p>
        ) : (
          <table className="tbl" style={{ width: '100%' }}>
            <thead>
              <tr>
                <th>Thuật toán</th>
                <th style={{ textAlign: 'right' }}>Trades</th>
                <th style={{ textAlign: 'right' }}>W / L / B</th>
                <th style={{ textAlign: 'right' }}>Win Rate</th>
                <th style={{ textAlign: 'right' }}>Total PnL</th>
                <th style={{ textAlign: 'right' }}>Avg/trade</th>
              </tr>
            </thead>
            <tbody>
              {bot_trades.map((row) => (
                <tr key={row.algorithm}>
                  <td><strong>{algoLabel(row.algorithm)}</strong></td>
                  <td style={{ textAlign: 'right', color: 'var(--text-2)' }}>{row.trades}</td>
                  <td style={{ textAlign: 'right', fontSize: 12, color: 'var(--text-2)' }}>
                    <span style={{ color: 'var(--up)' }}>{row.wins}</span>
                    {' / '}
                    <span style={{ color: 'var(--down)' }}>{row.losses}</span>
                    {' / '}
                    <span>{row.breakeven}</span>
                  </td>
                  <td style={{ textAlign: 'right' }}>
                    <span style={{
                      color: row.win_rate >= 0.6 ? 'var(--up)' : row.win_rate >= 0.4 ? 'var(--gold)' : 'var(--down)',
                      fontWeight: 600,
                    }}>
                      {pct(row.win_rate)}
                    </span>
                  </td>
                  <td style={{ textAlign: 'right' }}><PnlCell v={row.total_pnl} /></td>
                  <td style={{ textAlign: 'right' }}><PnlCell v={row.avg_pnl_per_trade} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Panel>
    </div>
  );
}
```

- [ ] **Commit**

```bash
git add frontend/src/pages/SessionStats.tsx
git commit -m "feat(frontend): add SessionStats page component"
```

---

## Task 7: Đăng ký route React + thêm tab

**Files:**
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/pages/Nasdaq.tsx`
- Modify: `frontend/src/pages/SP500.tsx`

- [ ] **App.tsx** — thêm import và route. Tìm block routes markets và thêm:

```tsx
// Thêm import ở đầu file (cùng chỗ với các page imports):
import SessionStats from './pages/SessionStats';

// Thêm route sau dòng /markets/:marketKey/detail:
<Route path="/markets/:marketKey/session-stats" element={<ErrorBoundary><SessionStats /></ErrorBoundary>} />
```

- [ ] **Nasdaq.tsx — MarketTabs** — thêm tab Session Stats. Tìm component `MarketTabs` trong `Nasdaq.tsx` và thêm NavLink cuối:

```tsx
<NavLink to={base + '/session-stats'} className={({ isActive }) => 'market-tab' + (isActive ? ' active' : '')}>
  <Icon name="pulse" size={14} />
  Phiên
</NavLink>
```

- [ ] **SP500.tsx — MarketTabs** — làm y hệt cho SP500.tsx (tìm cùng component `MarketTabs` và thêm NavLink tương tự).

- [ ] **Build frontend**

```bash
cd /home/chronical/Projects/private/go-stock-prediction/frontend && npm run build 2>&1 | tail -20
```
Expected: build thành công, không có TypeScript errors.

- [ ] **Commit**

```bash
git add frontend/src/App.tsx frontend/src/pages/Nasdaq.tsx frontend/src/pages/SP500.tsx
git commit -m "feat(frontend): add Session Stats tab to NASDAQ and SP500"
```

---

## Task 8: Docker rebuild + verify

**Files:** none (Docker ops)

- [ ] **Rebuild API container**

```bash
cd /home/chronical/Projects/private/go-stock-prediction
docker-compose build api && docker-compose up -d api
```

- [ ] **Rebuild frontend container**

```bash
docker-compose build frontend && docker-compose up -d frontend
```

- [ ] **Test API endpoint thủ công**

```bash
TOKEN=$(curl -s -X POST http://localhost/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"chon","password":"Ch1nch2n@"}' | jq -r '.token')

curl -s http://localhost/api/markets/nasdaq100/session-stats \
  -H "Authorization: Bearer $TOKEN" | jq '{market, session, dir_count: (.direction_accuracy | length), bot_count: (.bot_trades | length)}'
```
Expected: `market: "NASDAQ"`, session window đúng, 2 arrays có dữ liệu (hoặc rỗng nếu ngoài giờ).

- [ ] **Verify trên trình duyệt** — mở `http://localhost`, đăng nhập, vào NASDAQ → tab "Phiên", kiểm tra hiển thị đúng.

- [ ] **Commit final nếu cần hotfix**

```bash
git add -p
git commit -m "fix: session-stats post-deploy fixes"
```
