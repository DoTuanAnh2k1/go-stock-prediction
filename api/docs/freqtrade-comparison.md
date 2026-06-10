# So Sanh Chi Tiet: Freqtrade vs go-stock-prediction

---

## 1. Tong Quan Kien Truc

### Freqtrade

Freqtrade la mot trading bot framework ma nguon mo, viet hoan toan bang Python, tap trung vao thi truong crypto. Kien truc monolithic voi cac module lien ket chat che:

```
freqtrade/
├── Exchange Layer (CCXT)     — ket noi 100+ san crypto
├── Strategy Engine           — IStrategy interface, user-defined logic
├── FreqAI                    — ML subsystem (train, predict, retrain)
├── Backtesting Engine        — OHLCV simulation voi slippage, fees
├── Hyperopt                  — Bayesian optimization (Optuna/NSGAIIISampler)
├── Trade Manager             — quan ly position, stoploss, ROI
├── Data Pipeline             — download-data, convert-data (CLI)
└── Interfaces                — WebUI, Telegram Bot, REST API
```

Dac diem noi bat: toan bo chay trong mot process Python duy nhat, su dung SQLite cho trade persistence, giao tiep real-time qua CCXT voi san giao dich.

### go-stock-prediction

Kien truc microservice tach biet ro rang theo trach nhiem, da ngon ngu:

```
┌─────────────────────────────────────────────────────────────┐
│  Nginx (:80)  — reverse proxy                               │
└─────────────────────┬───────────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────────┐
│  Go API Backend (:8118)                                     │
│  - HTTP endpoints (/api/*)                                  │
│  - JWT auth, rate limiting                                  │
│  - Repository pattern (MySQL/GORM)                          │
│  - gRPC client → Python service                             │
└──────────────────────┬──────────────────────────────────────┘
                        │ gRPC (:8119)
┌──────────────────────▼──────────────────────────────────────┐
│  Python Prediction Service                                  │
│  - Crawlers: VNDirect, Yahoo Finance, CoinGecko, SJC API   │
│  - 11 algorithms: VWMA, EMA/MACD, LSTM, GRU, ARIMA-GARCH, │
│    EGARCH, SARIMA, LightGBM, XGBoost, Random Forest,       │
│    Ensemble                                                 │
│  - APScheduler cron jobs                                    │
│  - Walk-forward backtesting                                 │
│  - Reconciliation & direction accuracy tracking             │
└─────────────────────────────────────────────────────────────┘
                        │
┌──────────────────────▼──────────────────────────────────────┐
│  MySQL (:3306) — 6 prediction tables, training logs,       │
│  cron_schedules, users, direction_correct tracking          │
└─────────────────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────┐
│  React Frontend (:36018) — dashboard, charts, settings      │
└─────────────────────────────────────────────────────────────┘
```

So sanh kien truc tong quan:

| Tieu chi | Freqtrade | go-stock-prediction |
|---|---|---|
| Ngon ngu chinh | Python | Go (API) + Python (ML) |
| Kien truc | Monolithic | Microservice (gRPC) |
| Database | SQLite | MySQL voi GORM |
| Thi truong | Crypto (CCXT) | VN30, Gold, NASDAQ, SP500, Crypto |
| Output chinh | BUY/SELL signal | Predicted price + confidence |
| Settlement | Realtime | T+2.5 (VN), market-aware |
| Giao tiep | Telegram, REST | JWT REST API, React dashboard |

---

## 2. Thuat Toan va Technical Indicators

### Freqtrade: IStrategy indicators

Freqtrade khong tu cung cap thuat toan ML ngoai box — nguoi dung tu viet trong `populate_indicators()`. FreqAI mo rong voi cac model ML nhung user phai tu define features. Cac indicator pho bien trong community:

- RSI, MACD, Bollinger Bands, EMA, SMA (qua ta-lib hoac pandas-ta)
- Entry/exit signal la boolean: `enter_long = 1` khi dieu kien thoa
- Stoploss cung (`stoploss = -0.10`), minimal ROI (`{"0": 0.01}`)
- Trailing stop, protection mechanisms

FreqAI ho tro model: LightGBM, XGBoost, neural networks, CNN, CatBoost — nhung tat ca phai duoc nguoi dung cau hinh thong qua config JSON va chien luoc Python.

### go-stock-prediction: 11 thuat toan tich hop san

He thong tich hop 11 thuat toan doc lap, moi thuat toan la mot class Python implement abstract `PredictionAlgorithm`:

**Nhom Technical Analysis:**
- `moving_average` — VWMA voi trend slope projection, RSI momentum scaling, StochRSI overlay
- `ema_macd` — EMA slope projection + MACD momentum boost + Bollinger %B overlay

**Nhom Neural Network:**
- `lstm_nn` — PyTorch 2-layer LSTM (hidden=64, seq=60, dropout=0.2, 50 epochs)
- `gru` — PyTorch GRU architecture tuong tu LSTM

**Nhom Thong ke/Kinh te luong:**
- `arima_garch` — statsmodels ARIMA(2,1,2) + arch GARCH(1,1)
- `egarch` — Exponential GARCH (xu ly leverage effect)
- `sarima` — Seasonal ARIMA cho chuoi co mua vu

**Nhom Tree-based ML:**
- `lightgbm` — LightGBM regression voi 30 features + Optuna hyperopt
- `xgboost` — XGBoost regression voi 30 features + Optuna hyperopt
- `random_forest` — Random Forest regression voi 30 features

**Ensemble:**
- `ensemble` — Equal-weight average cua 10 base algorithms, confidence trung binh

So sanh ve chieu rong thuat toan:

| Nhom | Freqtrade (FreqAI) | go-stock-prediction |
|---|---|---|
| Technical Analysis | User-defined | VWMA, EMA/MACD tich hop san |
| Statistical/Econometric | Khong co | ARIMA-GARCH, EGARCH, SARIMA |
| Neural Network | LSTM, CNN (user config) | LSTM, GRU (built-in PyTorch) |
| Tree-based | LightGBM, XGBoost (user config) | LightGBM, XGBoost, Random Forest |
| Ensemble | Khong built-in | Equal-weight ensemble 10 models |

---

## 3. Feature Engineering

### Freqtrade / FreqAI

FreqAI su dung ba tang feature engineering co tinh tu dong hoa cao:

- `feature_engineering_expand_all()` — tu dong nhan features theo nhieu chieu: `include_timeframes` (multi-timeframe), `include_corr_pairlist` (correlated pairs), `indicator_periods_candles` (nhieu period). Ket qua co the la 10,000+ features.
- `feature_engineering_expand_basic()` — tuong tu nhung khong expand theo period.
- `feature_engineering_standard()` — buoc cuoi, xu ly custom transformations.

FreqAI tu dong ap dung: MinMaxScaler normalization, variance thresholding, va cac outlier detection methods (Dissimilarity Index, SVM, DBSCAN, PCA). Day la diem manh lon cua FreqAI — auto-scaling va outlier handling hoan toan tu dong.

### go-stock-prediction: 30-feature enhanced set

Du an nay ap dung mot feature set co dinh 30 features, duoc dinh nghia trong `prediction/src/algorithms/features.py`:

**Group A — Lag Returns (10 features):** `lag_1` den `lag_10` (log returns)

**Group B — MA Ratios (4 features):** `price/MA5`, `price/MA10`, `price/MA20`, `price/MA50`

**Group C — Multi-timeframe Returns (3 features):** 5-day, 10-day, 20-day log returns

**Group D — RSI (1 feature):** RSI(14) qua pandas-ta hoac numpy fallback

**Group E — Stochastic RSI (2 features):** StochRSI %K, %D (length=14, k=3, d=3)

**Group F — Bollinger %B (1 feature):** vi tri gia trong Bollinger Bands [0,1]

**Group G — MACD normalized (2 features):** MACD line/price, MACD histogram/price

**Group H — Rolling Volatility (3 features):** std cua log returns 5d, 10d, 20d

**Group I — Rate of Change (1 feature):** ROC(10) tinh theo phan tram

**Group J — Momentum (2 features):** momentum 5-period va 10-period

**Group K — Volume Ratio (1 feature):** current_vol / avg_vol(10)

Dac diem trien khai: dung pandas-ta khi available, tu dong fallback ve pure numpy neu khong co — dam bao chay duoc trong moi moi truong. Features duoc normalize ngam qua viec dung log returns va ratio (self-normalizing), khong can MinMaxScaler explicit.

So sanh feature engineering:

| Tieu chi | FreqAI | go-stock-prediction |
|---|---|---|
| So luong features | 10,000+ (auto-expand) | 30 (co dinh, curated) |
| Auto-scaling | MinMaxScaler tu dong | Implicit qua log returns/ratios |
| Multi-timeframe | Ho tro qua config | 3 timeframe co dinh (5d/10d/20d) |
| Correlated pairs | Ho tro | Chua co |
| Outlier detection | DI, SVM, DBSCAN, PCA | Chua co |
| Pandas-ta | User-defined | Tich hop, graceful fallback |
| Interpretability | Kho (10K features) | Cao (30 named features) |

---

## 4. Hyperparameter Optimization

### Freqtrade Hyperopt

Freqtrade Hyperopt dung Optuna voi `NSGAIIISampler` (multi-objective). Nguoi dung dinh nghia parameter space trong class strategy:

```python
# Trong strategy file
buy_rsi = IntParameter(20, 40, default=30, space='buy')
sell_rsi = DecimalParameter(55.0, 80.0, default=70.0, space='sell')
```

Optimization spaces co the la: buy/sell signal, ROI levels, stoploss, trailing stop, protection parameters. Loss functions built-in: SharpeHyperOptLoss, SortinoHyperOptLoss, MaxDrawDownHyperOptLoss — moi cai optimize theo metric khac nhau. Ket qua export ra JSON de cap nhat vao strategy.

Diem manh: Hyperopt toi uu hoa toan bo strategy logic (entry + exit + risk management), khong chi model params.

### Optuna trong go-stock-prediction

Optuna duoc tich hop truc tiep trong `_tune_lightgbm_params()` va tuong tu cho XGBoost, voi Bayesian optimization (Tree-structured Parzen Estimator):

```
LightGBM search space:
- learning_rate: Float log-uniform [0.01, 0.2]
- num_leaves: Int [8, 64]
- min_data_in_leaf: Int [3, 30]
- n_estimators: Int [50, 300]
- subsample: Float [0.6, 1.0]
- colsample_bytree: Float [0.6, 1.0]

XGBoost search space:
- learning_rate: Float log-uniform [0.01, 0.2]
- max_depth: Int [3, 10]
- n_estimators: Int [50, 300]
- subsample: Float [0.6, 1.0]
- colsample_bytree: Float [0.6, 1.0]
- min_child_weight: Int [1, 10]
```

Dieu kien kich hoat: chi chay khi `n_data_points >= 200` va `len(X_val) >= 5`, timeout 120 giay, 30 trials. Loss function: Mean Absolute Error tren validation set. Neu Optuna khong duoc cai dat hoac khong du du lieu, tu dong fallback ve default params — thiet ke graceful degradation.

Scope hep hon Freqtrade: chi optimize model hyperparameters, khong optimize entry/exit logic.

So sanh hyperopt:

| Tieu chi | Freqtrade Hyperopt | go-stock-prediction Optuna |
|---|---|---|
| Optimizer | NSGAIIISampler (NSGA-III) | TPE (Tree-structured Parzen) |
| Scope | Toan bo strategy | Model hyperparameters |
| Loss functions | Sharpe, Sortino, MaxDrawdown | MAE tren validation set |
| Ket qua | JSON strategy update | Runtime params, khong persist |
| CPU | Parallel (all cores) | Single-threaded |
| Trigger | CLI command | Tu dong trong train() |

---

## 5. Backtesting

### Freqtrade Backtesting Engine

Freqtrade co backtesting engine chuyen nghiep, mo phong trading thuc te:
- OHLCV candle simulation voi nhieu timeframe
- Slippage, trading fees (maker/taker)
- `--timeframe-detail` de simulate intra-candle movements
- Multi-strategy comparison trong mot lan chay
- Metrics: Sharpe ratio, Sortino ratio, Calmar ratio, Profit Factor, Max Drawdown, Win Rate, consecutive streaks
- Export JSON cho visualization qua FreqUI

Quan trong: Freqtrade backtest simulate quyet dinh BUY/SELL va P&L thuc te, khong chi do accuracy cua prediction.

### go-stock-prediction: Walk-Forward Backtesting

Du an implement walk-forward backtesting trong `prediction/src/orchestrator/training.py`, ham `run_historical_backtest()`:

Co che:
- `train_window`: so ngay du lieu lich su cho moi fold training (mac dinh 30)
- `step_size`: so ngay giua cac fold (mac dinh 6)
- `MAX_HISTORY = 270` ngay history cho moi prediction point
- `BATCH_SIZE = 200` records bulk insert vao MySQL

Walk-forward loop:
```
for window_end in range(train_window, n, step_size):
    price_list = prices[window_end - hist_len : window_end]   # training window
    for t in range(window_end, window_end + step_size):       # test fold
        result = algo.predict(price_list)
        accuracy = 1 - |actual - predicted| / actual
        status = "confirmed" if accuracy >= 0.70 else "wrong"
```

Ho tro tat ca 5 markets: VN30, Gold, NASDAQ, Crypto, SP500. Ket qua luu vao 5 prediction tables voi `direction_correct` tracking va concurrency guard (409 neu dang chay). Metrics: accuracy percentage, direction accuracy per algorithm per market.

Diem con thieu so voi Freqtrade: khong co slippage simulation, khong co fees, khong co Sharpe/Sortino/Drawdown metrics, khong simulate P&L trading thuc te.

---

## 6. Data Pipeline

### Freqtrade

Data pipeline hoan toan phu thuoc vao san giao dich qua CCXT:
- `freqtrade download-data` — tai OHLCV tu san (Binance, Kraken, OKX, Bybit...)
- `freqtrade convert-data` / `convert-trade-data` — chuyen doi dinh dang
- Du lieu luu dang JSON files, khong co database persistence cho historical data
- Real-time data tu websocket/REST cua san trong live trading
- Han che: FreqAI khong ho tro dynamic pairlists (can biet truoc tat ca symbols)

### go-stock-prediction

Custom crawlers cho tung data source, duoc deploy nhu APScheduler cron jobs:

| Crawler | Source | Schedule |
|---|---|---|
| VN30 | VNDirect API | 12:00 hang ngay |
| Gold | Yahoo Finance XAU + BTMC API + Phu Quy | Moi gio |
| NASDAQ | Yahoo Finance (15 symbols) | Moi gio phut 15 |
| Crypto BTC/ETH/SOL | CoinGecko | Moi gio phut 45 |
| S&P 500 | Yahoo Finance (16 symbols: SPY, QQQ, JPM...) | Moi gio phut 30 |

Pipeline logic: crawl -> training (moi 10 lan crawl) -> predict. Du lieu persist vao MySQL voi separate tables per market. Lich cron dynamic — thay doi qua UI Settings khong can restart.

Diem manh: da dang data source khong gioi han vao CCXT, ho tro tai san phi crypto (co phieu VN, xang dau, vang vat ly SJC).

---

## 7. Strategy Framework: IStrategy vs PredictionAlgorithm

### Freqtrade IStrategy Interface

```python
class MyStrategy(IStrategy):
    # Risk parameters
    stoploss = -0.10
    minimal_roi = {"0": 0.01}
    timeframe = '15m'
    max_open_trades = 3

    def populate_indicators(self, dataframe, metadata):
        dataframe['rsi'] = ta.RSI(dataframe['close'], timeperiod=14)
        dataframe['ema26'] = ta.EMA(dataframe['close'], timeperiod=26)
        return dataframe

    def populate_entry_trend(self, dataframe, metadata):
        dataframe.loc[
            (dataframe['rsi'] < 30) & (dataframe['ema26'] > dataframe['close']),
            'enter_long'
        ] = 1
        return dataframe

    def populate_exit_trend(self, dataframe, metadata):
        dataframe.loc[dataframe['rsi'] > 70, 'exit_long'] = 1
        return dataframe
```

Output: binary signal (enter_long/exit_long), ket hop voi stoploss va ROI de quyet dinh trade thuc te.

### go-stock-prediction PredictionAlgorithm Interface

```python
class PredictionAlgorithm(ABC):
    _market_key: str = ""  # set by registry per market

    @abstractmethod
    def predict(self, prices: list[float], volumes: list[float] | None) -> PredictionResult:
        """Returns predicted_price + confidence [0,1]"""

    def train(self, prices: list[float], volumes: list[float] | None) -> None:
        """Pre-train and cache model. No-op for stateless algorithms."""

    def train_batch(self, series: list[tuple[...]]) -> None:
        """Train on multiple price series — for stateful algorithms like LSTM."""

    def is_trained(self) -> bool:
        """True if instance has cached trained model."""
```

Output: `PredictionResult(predicted_price, confidence, current_price, algorithm_name)`. Khong co concept BUY/SELL — day la price forecasting, khong phai signal generation. Market-aware clamp: `get_max_change_pct(self._market_key)` gioi han prediction change theo tung thi truong.

So sanh philosophy:

| Khia canh | Freqtrade IStrategy | PredictionAlgorithm |
|---|---|---|
| Output | BUY/SELL signal | Predicted price + confidence |
| Risk management | Built-in (stoploss, ROI) | Tang rieng (trading sim) |
| Timeframe | Candle-based (OHLCV) | Daily close price |
| Market types | Crypto spot/futures | Equity, commodity, crypto |
| Reuse | Per-strategy | Per-market voi registry caching |

---

## 8. Nhung Gi Da Ap Dung Tu Freqtrade

Danh sach cu the cac y tuong tu Freqtrade/FreqAI da duoc implement trong go-stock-prediction:

### Enhanced Feature Set (30 features) — tu FreqAI feature engineering practice:
- **StochRSI %K/%D** — overbought/oversold detection chinh xac hon RSI don thuan
- **Bollinger %B** — vi tri gia trong band thay vi chi dung SMA
- **MACD normalized** (MACD/price, histogram/price) — scale-independent
- **Multi-timeframe returns** (5d/10d/20d) — tuong duong `include_timeframes` cua FreqAI
- **Rolling volatility** (std cua log returns) — 3 windows
- **Rate of Change** (ROC 10) — momentum indicator bo sung
- **Momentum 5/10** — price momentum truc tiep
- **MA ratios mo rong**: them MA10 va MA50 (truoc chi co MA5/MA20)

### Optuna Hyperparameter Optimization — tu Freqtrade Hyperopt approach:
- LightGBM: 6 hyperparameters, 30 trials, 120s timeout
- XGBoost: tuong tu voi max_depth, min_child_weight
- Graceful fallback ve default params khi khong du data

### StochRSI trong VWMA predictor — tu indicator practice:
- Overlay StochRSI %K len VWMA prediction de ap dung mean-reversion bias khi overbought/oversold

### Bollinger %B trong EMA/MACD predictor:
- Mean-reversion nudge khi gia gan upper/lower band

### Walk-forward backtesting — tu FreqAI adaptive retraining concept:
- Sliding window training + test fold, khong dung toan bo data de train mot lan
- Tranh look-ahead bias

---

## 9. Nhung Gi Co The Ap Dung Tiep (Roadmap)

### Tu FreqAI — tiem nang cao:

1. **Auto-scaling / RobustScaler:** FreqAI tu dong normalize features truoc khi dua vao model. Hien tai go-stock-prediction dung implicit normalization (log returns, ratios) nhung chua co explicit scaler. Them RobustScaler (tot hon MinMax voi outliers) co the cai thien LSTM va GRU.

2. **Outlier detection (Dissimilarity Index):** FreqAI co DI score de phat hien prediction points nam ngoai training distribution. Co the implement don gian bang Mahalanobis distance hoac Isolation Forest — tra ve confidence thap khi DI cao.

3. **Adaptive retraining trigger:** FreqAI retrain khi model drift duoc phat hien. go-stock-prediction hien retrain theo lich co dinh (moi 10 lan crawl hoac weekly). Co the them drift detection dua tren sliding accuracy window.

4. **Correlated assets features:** FreqAI ho tro `include_corr_pairlist` — dua price/return cua asset tuong quan vao feature. Vi du: BTC return lam feature cho ETH prediction, hoac S&P 500 lam feature cho NASDAQ.

### Tu Freqtrade Hyperopt — tiem nang trung binh:

5. **Multi-objective hyperopt:** Thay vi chi minimize MAE, optimize dong thoi MAE va direction accuracy (hai objective) bang NSGAIIISampler cua Optuna.

6. **Persist best hyperparams:** Hien tai Optuna chay lai moi lan train. Luu best params vao DB (bang `training_metrics` hoac table rieng) de reuse, chi re-optimize khi data thay doi dang ke.

### Tu Freqtrade Backtesting — tiem nang cao:

7. **Sharpe/Sortino ratio cho backtest:** Bo sung financial metrics vao backtesting endpoint. Hien tai chi co accuracy va direction_correct — them Sharpe ratio, Max Drawdown, Win Rate se cung cap goc nhin risk-adjusted performance.

8. **Slippage va fees simulation:** Trong trading simulation engine (bot trading), them mo phong slippage 0.1-0.3% va phi giao dich 0.15-0.25% (phi thuc cua HOSE) de ket qua simulation sat thuc hon.

9. **Intraday detail:** Freqtrade co `--timeframe-detail` de simulate intra-candle. Co the tan dung intraday data da co trong du an de cai thien signal timing.

### Tu FreqAI model types — tiem nang trung binh:

10. **CatBoost:** FreqAI ho tro CatBoost, thuong tot voi financial time series do xu ly categorical features natively. Co the them nhu thuat toan thu 12 — cung feature set 30 features, khong can feature engineering moi.

11. **Attention mechanism / Transformer:** FreqAI ho tro CNN-LSTM. Buoc tiep theo tu nhien cho du an la them Transformer predictor (self-attention) hoac CNN-LSTM hybrid, dac biet phu hop cho Crypto voi volatility cao.

---

## 10. Diem Manh Rieng cua go-stock-prediction

Nhung thu Freqtrade khong co hoac khong thiet ke cho:

### 1. Da thi truong da loai tai san thuc su
Freqtrade sinh ra cho crypto, CCXT chi ket noi san crypto. go-stock-prediction ho tro VN30 equity (VNDirect), vang vat ly SJC (gia mua/ban khac nhau), xang dau (khong co tren san nao), NASDAQ/S&P 500 stocks — qua custom crawlers cho tung nguon.

### 2. VN market-specific rules
- T+2.5 settlement khong co trong bat ky framework quoc te nao
- Bien do ±7% HOSE, ±10% HNX — market-aware clamp duoc implement chinh xac
- Crawling VNDirect API voi format dac thu VN
- Phan tich thi truong ~90% ca nhan nha dau tu (noise cao hon)

### 3. Econometric models tich hop san
ARIMA-GARCH, EGARCH, SARIMA la ba model kinh te luong khong co trong FreqAI. GARCH dac biet quan trong cho thi truong VN voi volatility clustering (co phieu VN co phuong sai thay doi ro ret theo phase thi truong). EGARCH xu ly leverage effect — khi gia giam tac dong len volatility manh hon khi tang, day la tinh chat quan sat duoc ro rang tren HOSE.

### 4. Direction accuracy tracking
go-stock-prediction theo doi khong chi price accuracy ma con direction accuracy (dung huong tang/giam) — duoc luu vao cot `direction_correct` nullable boolean trong 6 prediction tables, tong hop qua `DirectionAccuracyStore`. Freqtrade khong co concept nay vi no operate o level signal, khong level prediction.

### 5. Dynamic cron schedules qua UI
APScheduler voi DB-backed `cron_schedules` table, poll moi 60 giay, cho phep thay doi lich live khong restart service. Freqtrade yeu cau restart de doi config.

### 6. Multi-service Go architecture
Go API Backend xu ly HTTP voi hieu nang cao, type safety, va concurrency tot hon Python. gRPC separation dam bao ML workloads nang khong block API responses. Freqtrade la monolithic Python — blocking operations trong trading loop co the gay missed signals.

### 7. Reconciliation system
Sau khi gia thuc te co, he thong tu dong reconcile predictions — tinh accuracy, cap nhat `direction_correct`, danh dau "confirmed"/"wrong". Day la feedback loop khep kin de danh gia thuat toan tren du lieu thuc, khong chi backtest gia dinh.

---

## 11. Ket Luan

Freqtrade va go-stock-prediction giai quyet hai bai toan khac nhau o cung domain tai chinh:

**Freqtrade** la mot trading execution framework hoan chinh — muc tieu la thuc hien giao dich tu dong voi risk management, stoploss, ROI targeting. Diem manh la backtesting engine phuc tap voi slippage/fees, Hyperopt toi uu toan chien luoc, va he sinh thai CCXT ket noi 100+ san crypto. FreqAI bo sung ML voi auto-feature expansion va adaptive retraining.

**go-stock-prediction** la mot price forecasting va market analytics platform — muc tieu la du doan gia chinh xac tren nhieu loai tai san dac thu VN, cung cap insight qua dashboard. Diem manh la breadth cua thi truong (VN equity, vang vat ly, xang dau), depth cua thuat toan (11 models bao gom econometric), va kien truc microservice production-ready voi Go backend.

Hai he thong bo sung cho nhau hon la canh tranh. Cac cai tien da ap dung tu Freqtrade (enhanced features, Optuna hyperopt, walk-forward backtesting) da nang chat luong prediction len dang ke. Roadmap tiep theo nen tap trung vao: Sharpe/drawdown metrics cho backtesting engine, explicit feature normalization (RobustScaler), correlated assets cross-market features, va Transformer predictor de xu ly long-range temporal dependencies — dac biet co gia tri cho thi truong crypto va NASDAQ voi bien dong phi tuyen cao.

---

**Nguon tham khao:**
- Freqtrade documentation: https://www.freqtrade.io/en/stable/
- FreqAI documentation: https://www.freqtrade.io/en/stable/freqai/
- FreqAI feature engineering: https://www.freqtrade.io/en/stable/freqai-feature-engineering/
- Freqtrade Hyperopt: https://www.freqtrade.io/en/stable/hyperopt/
- Freqtrade Backtesting: https://www.freqtrade.io/en/stable/backtesting/
