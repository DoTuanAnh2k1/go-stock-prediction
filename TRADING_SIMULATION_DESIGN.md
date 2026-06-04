# Trading Simulation — Thiết kế hệ thống Bot giả lập giao dịch

## Tổng quan

Mỗi bot = 1 cặp **(market × algorithm)**. Bot đọc prediction từ DB, phát sinh tín hiệu BUY/SELL/HOLD, giả lập giao dịch trên portfolio ảo, và theo dõi hiệu suất. Toàn bộ chạy trên dữ liệu lịch sử đã có sẵn trong hệ thống (không cần dữ liệu real-time mới).

**Tổng: 6 markets × 11 algorithms = 66 bots**

| Market | Symbols | Vốn ban đầu | Đơn vị |
|--------|---------|------------|--------|
| VN30 | 30 cổ phiếu HOSE | 1,000,000,000 | VND |
| Gold | XAU/USD, SJC, BTMC | 100,000 | USD |
| NASDAQ | 15 symbols | 100,000 | USD |
| S&P 500 | 16 symbols | 100,000 | USD |
| Crypto | BTC, ETH, SOL | 100,000 | USD |
| Fuel | 4 loại xăng dầu VN | 1,000,000,000 | VND |

---

## 1. Danh sách Bot

**11 Algorithms (áp dụng đồng đều cho tất cả markets):**

| Key | Tên | Ghi chú |
|-----|-----|---------|
| `moving_average` | Moving Average | VWMA + RSI + Bollinger |
| `ema` | EMA/MACD | EMA (12,26) + MACD signal |
| `lstm_nn` | LSTM | PyTorch 2 layers, seq=60 |
| `arima_garch` | ARIMA-GARCH | ARIMA(2,1,2) + GARCH(1,1) |
| `lightgbm` | LightGBM | n_estimators=200 |
| `sarima` | SARIMA | Seasonal ARIMA |
| `egarch` | EGARCH | Exponential GARCH |
| `gru_nn` | GRU | Gated Recurrent Unit |
| `random_forest` | Random Forest | n_estimators=200 |
| `xgboost` | XGBoost | Gradient boosting |
| `ensemble` | Ensemble | Equal-weight của 10 models trên |

### Bot ID convention: `{market}_{algorithm}`

**VN30 (11 bots — VND):**

| Bot ID | Algorithm |
|--------|-----------|
| `vn30_moving_average` | Moving Average |
| `vn30_ema` | EMA/MACD |
| `vn30_lstm_nn` | LSTM |
| `vn30_arima_garch` | ARIMA-GARCH |
| `vn30_lightgbm` | LightGBM |
| `vn30_sarima` | SARIMA |
| `vn30_egarch` | EGARCH |
| `vn30_gru_nn` | GRU |
| `vn30_random_forest` | Random Forest |
| `vn30_xgboost` | XGBoost |
| `vn30_ensemble` | Ensemble |

**Gold (11 bots — USD):** `gold_moving_average`, `gold_ema`, `gold_lstm_nn`, `gold_arima_garch`, `gold_lightgbm`, `gold_sarima`, `gold_egarch`, `gold_gru_nn`, `gold_random_forest`, `gold_xgboost`, `gold_ensemble`

**NASDAQ (11 bots — USD):** `nasdaq_moving_average`, `nasdaq_ema`, `nasdaq_lstm_nn`, `nasdaq_arima_garch`, `nasdaq_lightgbm`, `nasdaq_sarima`, `nasdaq_egarch`, `nasdaq_gru_nn`, `nasdaq_random_forest`, `nasdaq_xgboost`, `nasdaq_ensemble`

**S&P 500 (11 bots — USD):** `sp500_moving_average`, `sp500_ema`, `sp500_lstm_nn`, `sp500_arima_garch`, `sp500_lightgbm`, `sp500_sarima`, `sp500_egarch`, `sp500_gru_nn`, `sp500_random_forest`, `sp500_xgboost`, `sp500_ensemble`

**Crypto (11 bots — USD):** `crypto_moving_average`, `crypto_ema`, `crypto_lstm_nn`, `crypto_arima_garch`, `crypto_lightgbm`, `crypto_sarima`, `crypto_egarch`, `crypto_gru_nn`, `crypto_random_forest`, `crypto_xgboost`, `crypto_ensemble`

**Fuel (11 bots — VND):** `fuel_moving_average`, `fuel_ema`, `fuel_lstm_nn`, `fuel_arima_garch`, `fuel_lightgbm`, `fuel_sarima`, `fuel_egarch`, `fuel_gru_nn`, `fuel_random_forest`, `fuel_xgboost`, `fuel_ensemble`

---

## 2. Cơ chế phát sinh tín hiệu (Signal Generation)

### 2.1 Signal từ Prediction

Mỗi lần có prediction mới trong DB, bot đọc và tính:

```
signal_strength = (predicted_price - current_price) / current_price × 100   (%)
```

Phân loại tín hiệu theo `signal_strength` và `confidence`:

| Điều kiện | Tín hiệu | Ghi chú |
|-----------|---------|---------|
| `signal_strength > BUY_THRESHOLD` AND `confidence > MIN_CONFIDENCE` | **BUY** | Mua vào |
| `signal_strength < -SELL_THRESHOLD` AND `confidence > MIN_CONFIDENCE` | **SELL** | Bán ra |
| Còn lại | **HOLD** | Giữ nguyên |

**Tham số mặc định:**

| Tham số | Giá trị | Ý nghĩa |
|---------|---------|---------|
| `BUY_THRESHOLD` | 1.5% | Dự đoán tăng ≥ 1.5% thì mua |
| `SELL_THRESHOLD` | 1.0% | Dự đoán giảm ≥ 1.0% thì bán |
| `MIN_CONFIDENCE` | 0.60 | Confidence tối thiểu để hành động |
| `STOP_LOSS` | -5.0% | Auto bán nếu lỗ ≥ 5% |
| `TAKE_PROFIT` | 8.0% | Auto bán nếu lời ≥ 8% |
| `MAX_POSITION_SIZE` | 15% | Tối đa 15% portfolio/một mã |
| `MAX_OPEN_POSITIONS` | 5 | Tối đa 5 mã cùng lúc (với VN30) |

### 2.2 Quy tắc thực thi lệnh (Trade Execution)

**BUY:**
1. Kiểm tra: số lệnh mở < `MAX_OPEN_POSITIONS`
2. Tính `position_size = min(available_cash × 0.15, available_cash / remaining_slots)`
3. Mua tại giá `current_price` (trong simulation dùng giá thực tế ngày T)
4. Ghi nhận: entry_price, entry_date, quantity, algorithm, signal_strength

**SELL (chủ động):**
1. Bán toàn bộ vị thế của mã đó
2. Tính P&L = (exit_price - entry_price) × quantity

**SELL (stop-loss / take-profit):**
1. Mỗi ngày kiểm tra tất cả vị thế đang mở
2. Nếu `(current_price - entry_price) / entry_price` vượt ngưỡng → auto SELL

**Hạn chế giao dịch theo thị trường:**
- VN30: chỉ giao dịch các ngày trading thực tế (loại ngày nghỉ)
- NASDAQ/SP500: theo lịch NYSE (T2-T6, bỏ holiday)
- Crypto: 24/7
- Fuel: chỉ khi có cập nhật giá mới

---

## 3. Cấu trúc dữ liệu

### 3.1 DB Schema — 4 bảng mới

#### `sim_bots`
```sql
CREATE TABLE sim_bots (
    id           VARCHAR(50) PRIMARY KEY,      -- 'vn30_lstm'
    market       VARCHAR(20) NOT NULL,         -- 'VN30', 'GOLD', 'NASDAQ', 'SP500', 'CRYPTO', 'FUEL'
    algorithm    VARCHAR(50) NOT NULL,         -- 'lstm_nn', 'moving_average', ...
    display_name VARCHAR(100) NOT NULL,
    initial_capital   DECIMAL(20,2) NOT NULL,
    currency     VARCHAR(5) NOT NULL,          -- 'VND', 'USD'
    buy_threshold     DECIMAL(5,2) DEFAULT 1.50,
    sell_threshold    DECIMAL(5,2) DEFAULT 1.00,
    min_confidence    DECIMAL(4,2) DEFAULT 0.60,
    stop_loss         DECIMAL(5,2) DEFAULT 5.00,
    take_profit       DECIMAL(5,2) DEFAULT 8.00,
    max_position_pct  DECIMAL(5,2) DEFAULT 15.00,
    max_positions     INT DEFAULT 5,
    is_active    BOOLEAN DEFAULT TRUE,
    created_at   DATETIME DEFAULT NOW(),
    updated_at   DATETIME DEFAULT NOW()
);
```

#### `sim_sessions`
```sql
CREATE TABLE sim_sessions (
    id           BIGINT AUTO_INCREMENT PRIMARY KEY,
    bot_id       VARCHAR(50) NOT NULL,
    start_date   DATE NOT NULL,
    end_date     DATE,
    status       VARCHAR(20) DEFAULT 'running',   -- 'running', 'completed', 'paused'
    mode         VARCHAR(20) DEFAULT 'backtest',  -- 'backtest', 'live'
    created_at   DATETIME DEFAULT NOW(),
    FOREIGN KEY (bot_id) REFERENCES sim_bots(id)
);
```

#### `sim_trades`
```sql
CREATE TABLE sim_trades (
    id               BIGINT AUTO_INCREMENT PRIMARY KEY,
    session_id       BIGINT NOT NULL,
    bot_id           VARCHAR(50) NOT NULL,
    symbol           VARCHAR(20) NOT NULL,
    action           VARCHAR(5) NOT NULL,       -- 'BUY', 'SELL'
    quantity         DECIMAL(20,6) NOT NULL,
    price            DECIMAL(20,4) NOT NULL,
    trade_value      DECIMAL(20,2) NOT NULL,    -- price × quantity
    signal_strength  DECIMAL(8,4),             -- % change predicted
    confidence       DECIMAL(4,3),
    trade_date       DATE NOT NULL,
    close_reason     VARCHAR(20),               -- 'signal', 'stop_loss', 'take_profit'
    -- Filled in khi SELL:
    entry_trade_id   BIGINT,
    pnl              DECIMAL(20,2),
    pnl_pct          DECIMAL(8,4),
    created_at       DATETIME DEFAULT NOW(),
    FOREIGN KEY (session_id) REFERENCES sim_sessions(id)
);
```

#### `sim_portfolio_snapshots`
```sql
CREATE TABLE sim_portfolio_snapshots (
    id               BIGINT AUTO_INCREMENT PRIMARY KEY,
    session_id       BIGINT NOT NULL,
    bot_id           VARCHAR(50) NOT NULL,
    snapshot_date    DATE NOT NULL,
    cash_balance     DECIMAL(20,2) NOT NULL,
    positions_value  DECIMAL(20,2) NOT NULL,
    total_value      DECIMAL(20,2) NOT NULL,
    total_return_pct DECIMAL(8,4),             -- so với initial_capital
    open_positions   INT DEFAULT 0,
    FOREIGN KEY (session_id) REFERENCES sim_sessions(id),
    UNIQUE KEY uk_snapshot (session_id, snapshot_date)
);
```

---

## 4. Kiến trúc phần mềm

### 4.1 Python Prediction Service — module mới `prediction/src/simulation/`

```
prediction/src/simulation/
├── __init__.py
├── engine.py          # SimulationEngine — điều phối toàn bộ
├── bot.py             # TradingBot — state machine cho 1 bot
├── signal.py          # SignalGenerator — đọc predictions → tín hiệu
├── portfolio.py       # Portfolio — quản lý cash + positions
├── metrics.py         # PerformanceMetrics — tính toán KPIs
└── seeder.py          # seed bảng sim_bots khi startup
```

#### `signal.py` — SignalGenerator

```python
class SignalGenerator:
    """Đọc predictions từ DB, phát tín hiệu cho từng (market, algorithm, date)."""

    def get_signals(self, market: str, algorithm: str, date: date) -> list[TradeSignal]:
        """
        Trả về list TradeSignal cho ngày đó.
        Đọc từ bảng prediction tương ứng (predictions, gold_predictions, etc.)
        """
        ...

@dataclass
class TradeSignal:
    symbol: str
    action: Literal["BUY", "SELL", "HOLD"]
    signal_strength: float   # % change predicted
    confidence: float
    predicted_price: float
    current_price: float
    prediction_date: date
```

#### `portfolio.py` — Portfolio

```python
@dataclass
class Position:
    symbol: str
    quantity: float
    entry_price: float
    entry_date: date
    entry_trade_id: int

class Portfolio:
    initial_capital: float
    cash: float
    positions: dict[str, Position]   # symbol → Position

    def buy(self, symbol, price, cash_fraction, date) -> Trade | None
    def sell(self, symbol, price, date, reason) -> Trade | None
    def check_stop_loss_take_profit(self, current_prices, date) -> list[Trade]
    def snapshot(self, current_prices, date) -> PortfolioSnapshot
    def total_value(self, current_prices) -> float
```

#### `bot.py` — TradingBot

```python
class TradingBot:
    config: BotConfig
    portfolio: Portfolio
    signal_gen: SignalGenerator

    def step(self, date: date) -> list[Trade]:
        """Chạy 1 ngày: lấy tín hiệu → kiểm tra SL/TP → thực thi lệnh mới."""
        signals = self.signal_gen.get_signals(self.config.market, self.config.algorithm, date)
        trades = self.portfolio.check_stop_loss_take_profit(current_prices, date)
        for signal in signals:
            if signal.action == "BUY":
                trade = self.portfolio.buy(...)
            elif signal.action == "SELL":
                trade = self.portfolio.sell(...)
            if trade:
                trades.append(trade)
        return trades
```

#### `engine.py` — SimulationEngine

```python
class SimulationEngine:
    """Chạy backtest hoặc live simulation cho tất cả bots."""

    def run_backtest(self, bot_id: str, start_date: date, end_date: date) -> SimResult:
        """Walk-forward: lặp qua từng ngày, gọi bot.step(date)."""
        ...

    def run_all_bots_backtest(self, start_date: date, end_date: date) -> list[SimResult]:
        """Backtest song song tất cả bots."""
        ...

    def run_live_step(self):
        """Gọi hàng ngày bởi APScheduler — dùng prediction mới nhất."""
        ...
```

### 4.2 Go API Backend — `pkg/server/api_simulation.go`

```go
// GET  /api/simulation/bots              — danh sách tất cả bots + trạng thái
// GET  /api/simulation/bots/{id}         — chi tiết 1 bot + metrics
// GET  /api/simulation/bots/{id}/trades  — lịch sử giao dịch
// GET  /api/simulation/bots/{id}/chart   — portfolio value theo thời gian
// GET  /api/simulation/leaderboard       — xếp hạng bots theo total return
// POST /api/simulation/bots/{id}/run     — trigger backtest (gRPC → Python)
// POST /api/simulation/run-all           — trigger backtest tất cả bots
// PUT  /api/simulation/bots/{id}/config  — cập nhật tham số bot (admin)
```

### 4.3 gRPC — RPC mới trong `prediction.proto`

```protobuf
rpc TriggerSimulationBacktest(SimulationRequest) returns (TriggerResponse);
rpc TriggerSimulationLiveStep(Empty) returns (TriggerResponse);

message SimulationRequest {
  string bot_id = 1;        // "" = all bots
  string start_date = 2;    // "2024-01-01"
  string end_date = 3;      // "" = today
}
```

---

## 5. Metrics & Leaderboard

### 5.1 KPIs cho mỗi bot

| Metric | Công thức | Ý nghĩa |
|--------|----------|---------|
| **Total Return %** | `(final_value - initial) / initial × 100` | Lãi/lỗ tổng |
| **Annualized Return %** | `(1 + total_return)^(365/days) - 1` | Return quy năm |
| **Sharpe Ratio** | `mean(daily_returns) / std(daily_returns) × √252` | Return/risk |
| **Max Drawdown** | `max((peak - trough) / peak)` | Rủi ro sụt giảm lớn nhất |
| **Win Rate** | `winning_trades / total_closed_trades × 100` | % giao dịch có lãi |
| **Profit Factor** | `total_profit / total_loss` | Tỷ lệ lời/lỗ |
| **Total Trades** | Đếm closed trades | Tần suất giao dịch |
| **Avg Trade Duration** | `mean(exit_date - entry_date)` | Thời gian giữ trung bình |
| **Best Trade** | max(pnl_pct) | Giao dịch tốt nhất |
| **Worst Trade** | min(pnl_pct) | Giao dịch tệ nhất |

### 5.2 Leaderboard API Response

```json
{
  "leaderboard": [
    {
      "rank": 1,
      "bot_id": "crypto_ensemble",
      "display_name": "Crypto — Ensemble",
      "market": "CRYPTO",
      "algorithm": "ensemble",
      "currency": "USD",
      "initial_capital": 100000,
      "final_value": 147320.50,
      "total_return_pct": 47.32,
      "annualized_return_pct": 31.5,
      "sharpe_ratio": 2.14,
      "max_drawdown_pct": -8.7,
      "win_rate_pct": 64.2,
      "profit_factor": 2.8,
      "total_trades": 89,
      "simulation_period": { "start": "2024-01-01", "end": "2026-05-31" }
    }
  ],
  "summary": {
    "total_bots": 27,
    "best_market": "CRYPTO",
    "best_algorithm": "ensemble",
    "avg_return_pct": 18.4
  }
}
```

---

## 6. Frontend — Trang Simulation

### 6.1 Các views cần thêm

**`/simulation`** — Trang chính:
- Leaderboard table: sort theo Total Return, Sharpe, Win Rate
- Filter theo market, algorithm, currency
- Color-coded: xanh (dương), đỏ (âm)

**`/simulation/:botId`** — Chi tiết bot:
- Portfolio value chart (line chart theo thời gian)
- Drawdown chart
- Trade history table (entry/exit/P&L)
- Open positions (nếu đang chạy live)
- KPI cards: Return, Sharpe, Win Rate, Max DD

**Dashboard `/`** — Thêm widget nhỏ:
- Top 3 bots hiện tại
- Tổng bot đang dương/âm

---

## 7. Scheduler — Jobs mới

Thêm vào `prediction/src/scheduler/jobs.py`:

```python
JOB_FUNCTIONS["simulation_daily"] = run_simulation_live_step
# Chạy sau predict_daily (ví dụ: 19:00 hằng ngày)
# Đọc predictions mới nhất → cập nhật portfolio tất cả bots
```

Thêm hàng trong `cron_schedules`:

| Job Key | Schedule mặc định | Công việc |
|---------|-------------------|-----------|
| `simulation_daily` | `0 0 19 * * *` | Cập nhật portfolio tất cả bots sau predict |

---

## 8. Kế hoạch triển khai

### Phase 1 — Data layer (1-2 ngày)
- [ ] Tạo 4 bảng DB mới (migration)
- [ ] Go GORM models cho simulation tables
- [ ] Python SQLAlchemy models tương ứng
- [ ] `seeder.py` — seed 27 bots vào `sim_bots`

### Phase 2 — Simulation Engine (2-3 ngày)
- [ ] `signal.py` — đọc predictions từ tất cả 6 bảng prediction
- [ ] `portfolio.py` — quản lý cash/positions/snapshots
- [ ] `metrics.py` — tính KPIs
- [ ] `bot.py` — step function
- [ ] `engine.py` — backtest loop

### Phase 3 — Backend API (1-2 ngày)
- [ ] Proto RPC mới
- [ ] gRPC server handler trong Python
- [ ] Go API endpoints (leaderboard, bot detail, chart, trades)
- [ ] Router registration + Swagger annotations

### Phase 4 — Frontend (2-3 ngày)
- [ ] `Simulation.tsx` — leaderboard page
- [ ] `SimulationBot.tsx` — bot detail page
- [ ] Portfolio chart component (recharts)
- [ ] Dashboard widget

### Phase 5 — Testing & Tuning (1-2 ngày)
- [ ] Unit test: signal generation, portfolio logic, metrics
- [ ] Integration test: backtest một bot đầu đến cuối
- [ ] Verify kết quả với dữ liệu thực tế trong DB
- [ ] Tuning tham số BUY_THRESHOLD, STOP_LOSS

---

## 9. Lưu ý quan trọng

### Về dữ liệu
- **Backtest chỉ dùng được khi đã có predictions trong DB.** Cần chạy predict trước cho khoảng thời gian muốn backtest.
- Predictions dùng `current_price` tại thời điểm predict, không phải giá mở cửa ngày hôm sau — có **look-ahead bias nhỏ** (chấp nhận được cho simulation nội bộ).
- Fuel là commodity đặc biệt: giá không thay đổi hằng ngày (thay đổi theo đợt điều chỉnh) → `signal_strength` thường = 0, ít giao dịch.

### Về quy trình
- Luồng chuẩn: **Crawl → Predict → Simulation step** (theo thứ tự đó hàng ngày).
- Live simulation không tạo lệnh thật — chỉ giả lập dựa trên giá thực tế được crawl.
- Backtest và Live cùng dùng chung logic, chỉ khác nguồn dữ liệu đầu vào.

### Về so sánh
- VN30 và Fuel dùng VND, các sàn khác dùng USD — **không nên so sánh absolute P&L**, chỉ so sánh **%Return và Sharpe Ratio**.
- Leaderboard nên có 2 tab: VND markets và USD markets.
