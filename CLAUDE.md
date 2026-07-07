# CLAUDE.md

## Tổng quan dự án

Hệ thống dự đoán giá tài sản tài chính. Thu thập dữ liệu từ 4 nguồn (Gold SJC/XAU, NASDAQ, Crypto BTC/ETH/SOL, S&P 500), chạy 13 thuật toán ML (Moving Average, EMA/MACD, LSTM PyTorch, GRU PyTorch, ARIMA-GARCH, EGARCH, SARIMA, LightGBM, XGBoost, Random Forest, Ensemble, RL DQN, Transformer PatchTST), và hiển thị kết quả qua web dashboard. **Tất cả 4 market dự đoán "GIỜ KẾ TIẾP" (`target = now + 1h`)** — input vẫn dùng chuỗi giá daily-live (1 dòng/ngày, ghi đè mỗi lần crawl). NASDAQ/SP500 chỉ predict trong giờ phiên Mỹ. Reconcile tất cả markets theo GOLD-style: chấm khi `target_date <= now`, dùng giá live mới nhất.

**Kiến trúc: microservice (6 service chính + supporting)**
- **API Backend** (`api-svc/cmd`) — Go HTTP `:8118`, thin proxy cho auth (validate JWT locally, forward sang Java Auth qua gRPC), trigger gRPC sang Prediction Service, đọc DB trực tiếp.
- **Auth Service** (`auth-svc/`) — **Java** Spring Boot 3, gRPC `:8120`. Toàn bộ auth/RBAC: login, JWT (HMAC256), user CRUD, market groups, command RBAC, bcrypt, Flyway V1–V3. Seeds `chon/super_admin` on startup. Ba roles: `super_admin`, `admin`, `user`.
- **Prediction Service** (`prediction-svc/`) — **Python** gRPC `:8119`. Crawling, 13 thuật toán ML, training, APScheduler cron jobs.
- **Gateway Service** (`gateway-svc/`) — **Rust** Axum `:80`/`:443`. Longest-prefix routing: `/swagger`→404, `/api`→api-svc:8118, `/`→web-svc:3000. TLS self-signed cert tự tạo.
- **CLI Service** (`cli-svc/`) — **Go** SSH server `:2345` (expose trực tiếp, không qua gateway). charmbracelet/wish + bubbletea TUI. SSH auth → POST /api/x/grant (X-Token header). Command RBAC client-side. **KHÔNG tích hợp service-mgt.**
- **Service Management** (`service-mgt/`) — **Go** gRPC `:8121` (internal only). Registry/discovery — **4 service tích hợp** (api-svc, auth-svc, prediction-svc, gateway-svc). `SERVICE_MGT_ENABLED=false` by default — có static fallback.

## Lệnh thường dùng

```bash
# Build API Backend (Go)
cd api-svc && go build -o api-server ./cmd

# Start toàn bộ stack
docker compose --env-file .env -f deploy/docker-compose.yaml up -d
# make up / make reset

# Import schema thủ công (thường tự init từ database.sql khi container db lần đầu lên)
psql -U postgres -d go_stock_prediction -f database.sql

# Regenerate proto Go stubs
cd api-svc && protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/prediction/prediction.proto
cd api-svc && protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/auth/auth.proto

# Regenerate proto Python stubs
python -m grpc_tools.protoc -Iapi-svc/proto --python_out=prediction-svc/src/proto --grpc_python_out=prediction-svc/src/proto api-svc/proto/prediction/prediction.proto

# Regenerate Swagger — BẮT BUỘC --parseInternal --parseDependency (swag resolve type proto như authpb.UserResponse)
cd api-svc && swag init -g cmd/main.go -o docs/ --parseInternal --parseDependency

# Test Python prediction service
cd prediction-svc && make test
docker exec prediction-svc python -m pytest tests/ -v

# Theo dõi CPU/RAM từng container
make stats          # snapshot một lần
make stats-watch    # refresh 3s
```

## Công cụ khám phá code (cho AI assistant)

- **Codegraph trước tiên:** `codegraph_explore` làm tool ĐẦU TIÊN cho mọi câu hỏi (how does X work, where is X, call flow). Index sẵn toàn bộ codebase.
- **Phân tích symbol:** `codegraph_search` → `codegraph_callers`/`codegraph_callees` → `codegraph_impact`.
- **grep/Read:** Chỉ khi codegraph không đủ chi tiết.

## Luồng khởi động

### API Backend (api-svc/cmd/main.go)
1. `config.InitConfig()` → set timezone ICT → `logger.Init()` → `repository.Init()` (PostgreSQL)
2. `authclient.Init()` (→ Java Auth :8120) → `grpcclient.Init()` (→ Python Prediction :8119)
3. `server.StartBackupScheduler(store)` → `server.StartHTTPServer()` (:8118)
4. SIGTERM → `StopBackupScheduler()` + `grpcclient.Close()` + `authclient.Close()`

### Prediction Service (prediction-svc/src/main.py)
1. config → timezone (`TZ=Asia/Ho_Chi_Minh` + `time.tzset()`) → `init_logger()` → `init_db()`
2. `start_grpc_server(:8119)` → `init_scheduler()` (APScheduler, poll DB 60s)
3. `registry_client.start()` nếu `SERVICE_MGT_ENABLED=true`
4. SIGTERM → `registry_client.stop()` + `stop_grpc_server()` + `shutdown_scheduler()`

## Biến môi trường (.env)

```
# HTTP / gRPC
SERVER_PORT=8118
GRPC_SERVER_PORT=8119
GRPC_TARGET=prediction-svc:8119          # Docker; local: localhost:8119
AUTH_GRPC_TARGET=auth-svc:8120           # Docker; local: localhost:8120

# Auth
JWT_SECRET=change-me-in-production       # Bắt buộc đổi production
ADMIN_USERNAME=admin
ADMIN_PASSWORD=admin123

# Database
DB_DRIVER=postgresql
POSTGRES_HOST=db                         # Docker: db; local: localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=123
POSTGRES_DB=go_stock_prediction
POSTGRES_DEBUG=false                     # true = SQLAlchemy echo SQL (rất ồn)
DB_PASSWORD=123                          # Dùng bởi Java Auth (Flyway)

# Logging
LOG_LEVEL=INFO
LOG_FORMAT=console                       # console = màu ANSI; json = log aggregator

# Backup & RL
BACKUP_DIR=/backups                      # mount vào api-svc
RL_MODEL_DIR=/models                     # mount vào prediction-svc; file: rl_dqn_{market}.pt
DOCS_ROOT=/repo                          # mount vào api-svc; repo root read-only; phục vụ tab Docs

# CLI Service
INTERNAL_SECRET=change-me-in-production # Header X-Internal-Secret cho /api/command-handlers/upsert
API_BASE_URL=http://gateway-svc/api
SSH_LISTEN_ADDR=0.0.0.0:2345

# Service Registry (default off)
SERVICE_MGT_ENABLED=false
REGISTRY_GRPC_TARGET=service-mgt:8121

# Per-symbol models (default off)
PER_SYMBOL_ENABLED=false
PER_SYMBOL_MIN_POINTS=80
PER_SYMBOL_WORKERS=0                     # 0 = os.cpu_count()

# Crawl sanity guard (optional — có default trong code)
CRAWL_MAX_TICK_DEVIATION_GOLD=0.30       # ngưỡng lệch 1-lần cho daily-live gate
CRAWL_MAX_TICK_DEVIATION_NASDAQ=0.40
CRAWL_MAX_TICK_DEVIATION_SP500=0.40
CRAWL_MAX_TICK_DEVIATION_CRYPTO=0.80
```

## Ports

| Service | Port | Protocol | Ghi chú |
|---------|------|----------|---------|
| Gateway | 80/443 | HTTP/HTTPS | Proxy `/api`→api-svc:8118, `/`→web-svc:3000 |
| API Backend | 8118 | HTTP | Internal; Swagger tại `http://localhost:8118/swagger/` |
| Prediction Service | 8119 | gRPC | Internal |
| Auth Service | 8120 | gRPC | Internal |
| Service Management | 8121 | gRPC | Internal only — KHÔNG qua gateway |
| TimescaleDB | 5432 | TCP | Container `timescaledb` |
| pgAdmin | 8081 | HTTP | bind 127.0.0.1 |
| Frontend | 3000 | HTTP | Internal (qua gateway) |
| CLI Service | 2345 | SSH | Expose trực tiếp |

## Docker Compose Services

`deploy/docker-compose.yaml` — `make up` hoặc `docker compose --env-file .env -f deploy/docker-compose.yaml up -d`

| Service | Ngôn ngữ | Depends On | Ghi chú |
|---------|----------|------------|---------|
| `db` | TimescaleDB | — | Schema init từ `database.sql`; volume `postgres_data` |
| `service-mgt` | Go | `db` | gRPC :8121; build context = `../service-mgt` |
| `prediction-svc` | Python | `db`, `service-mgt` | gRPC :8119; build context = repo root; volume `rl_models:/models` |
| `auth-svc` | Java | `db`, `service-mgt` | gRPC :8120; Flyway migrations; seeds super_admin |
| `api-svc` | Go | `db`, `prediction-svc`, `auth-svc`, `service-mgt` | HTTP :8118; build context = repo root (copy service-mgt/); volume `backup_data:/backups`; bind-mount `../:/repo:ro` (DOCS_ROOT) |
| `gateway-svc` | Rust | `api-svc`, `web-svc`, `service-mgt` | expose :80/:443; bind-mount `./gateway-svc/certs` |
| `web-svc` | React/nginx | `api-svc` | port 3000 (qua gateway); không tích hợp service-mgt |
| `cli-svc` | Go | `gateway-svc` | SSH :2345; build context = `../cli-svc`; KHÔNG tích hợp service-mgt |
| `pgadmin` | — | `db` | expose 127.0.0.1:8081 |

## Conventions trong codebase

- **Repository pattern (Go):** Mọi DB access từ API Backend phải qua `DatabaseStore` interface (`api-svc/pkg/store/repository/`). Không gọi GORM trực tiếp từ service layer. `repository.GetSingleton()` trả instance toàn cục.
- **Algorithms (Python):** Mỗi thuật toán implement `PredictionAlgorithm` (`base.py`) với `predict(prices, volumes) -> PredictionResult`. Đăng ký metadata tại `api-svc/pkg/service/predict/registry/algorithms.go` (Go). 13 thuật toán: moving_average, ema, lstm_nn, gru_nn, arima_garch, egarch, sarima, lightgbm, xgboost, random_forest, ensemble, rl_dqn, transformer_nn. `transformer_nn` (PatchTST-lite) dùng thêm bảng `stock_fundamentals` (báo cáo tài chính yfinance, crawl thứ Bảy) làm static context — symbol truyền qua `_context_symbol` (runner set per-symbol cho NASDAQ/SP500) và `train_batch_labeled(series3)` (base có default strip label). **Direction head label = dấu return THÔ (`ret > 0`)** qua `_direction_targets(y_std, r_mean, r_std)` = `y_std > -r_mean/r_std` (return đã chuẩn hóa nên ngưỡng 0 = `ret > r_mean`, sai so với metric chấm `ret > 0`; từng gây bias "giảm" 95% → dưới chance, xem `docs/incidents/2026-07-transformer-direction-bias.md`). KHÔNG trong Ensemble. Bot simulation của transformer_nn dùng nhánh `is_conviction` (P(up) từ direction head, floor 0.52/0.48) — không dùng threshold %-giá.
- **Market-aware clamp:** Tất cả algorithms dùng `get_max_change_pct(self._market_key)` từ `base.py`. Giới hạn: GOLD/SP500 ±15%, NASDAQ100 ±20%, CRYPTO ±50%.
- **Direction accuracy:** `direction_correct` (nullable bool) trong 4 prediction tables. `True` = hướng đúng; `NULL` = chưa reconcile. Reconcile tất cả theo GOLD-style: `target_date <= now`, `actual` = giá live mới nhất. Orchestrator dùng dir_acc để set weight cho Ensemble mỗi run. **Frozen-actual guard:** `repository.direction_verdict(pred_diff, actual_diff)` trả `None` khi `actual == current` (giá live chưa nhích khỏi entry — đóng phiên sau target hoặc crawl trễ trên bảng daily-live). Cả `reconcile_predictions` (main) lẫn `backfill_direction_correct_*` **skip** row frozen (để `NULL`, pending) thay vì chấm sai — nếu không, cả batch bị chấm 0% vì `sign(actual_diff)=0` không bao giờ khớp `sign(pred_diff)`.
- **Cron schedules:** Nguồn sự thật Python-side = `DEFAULT_SCHEDULES` trong `prediction-svc/src/scheduler/manager.py` (upsert mỗi startup). Go `seedCronSchedules()` chỉ insert-if-not-exists. `daily_backup` là ngoại lệ — Go-side (`backup_scheduler.go`, insert-if-not-exists).
- **`auth.proto` hai bản:** `api-svc/proto/auth/auth.proto` (Go) và `auth-svc/src/main/proto/auth.proto` (Java) — phải đồng bộ thủ công khi thêm RPC.
- **Proto regeneration:** Sửa `prediction.proto` → tái sinh Go stubs (`protoc` từ `api-svc/`) và Python stubs (trong Dockerfile stage proto-builder). Không sửa tay file generated.
- **Data ordering:** DB trả `*_prices` với `ORDER BY trading_date DESC`. Python algorithms tự đảo ngược về ASC trước khi build feature sequences.
- **JWT middleware — non-blocking:** `JWTMiddleware` chỉ inject claims vào context, request không có token vẫn tiếp tục. Handler bảo vệ dùng `requireAuth()` / `requireAdmin()`.
- **Timezone — ICT-at-rest:** Toàn bộ `TIMESTAMP` trong DB lưu ICT wallclock (`TIMESTAMP WITHOUT TIME ZONE`). Python: **luôn `datetime.now()`** (container `TZ=Asia/Ho_Chi_Minh`), **tuyệt đối không `utcnow()`**. Go: `time.Local = Asia/Ho_Chi_Minh` set trong `main.go`.
- **Logging:** Một dòng màu, không emoji, tiếng Anh, ANSI forced kể cả Docker. api-svc: zerolog; auth-svc: logback + `GrpcLoggingInterceptor` (1 dòng/RPC); prediction-svc: structlog `ConsoleRenderer(colors=True)`; gateway-svc: tracing compact+ansi. Định dạng qua `LOG_FORMAT`.
- **Swagger annotations:** Mỗi handler có swaggo annotations. Sau khi sửa, chạy `cd api-svc && swag init -g cmd/main.go -o docs/ --parseInternal --parseDependency` (thiếu hai flag → lỗi `cannot find type definition`). Không sửa tay `api-svc/docs/`.
- **Decimal:** Go dùng `shopspring/decimal` cho giá. Python dùng `Decimal` stdlib hoặc pandas float64 (làm tròn trước khi lưu DB).
- **gRPC triggers:** Mọi trigger handler gọi `requireGRPCClient(w)` trước (503 nếu chưa init). Wrap bằng `AdminRequired()`.
- **Dynamic cron:** Python poll DB 60s, tự reschedule APScheduler. API `PUT /api/schedules/{key}` để chỉnh live.
- **Market calendar:** `market_calendar.py` — `is_market_open()` (guard ngày) và `is_intraday_open()` (guard giờ phiên). CRYPTO = 24/7. GOLD = 24/5 (đóng T7+CN). NASDAQ/SP500 = đóng cuối tuần + lễ NYSE + ngoài giờ phiên Mỹ (≈20:30–03:00 ICT).
- **Service discovery (service-mgt):** Client-side discovery; write-through cache (Postgres + in-memory). Reaper 1s (TTL 30s→DOWN, +60s→evict). Heartbeat 10s. Toàn bộ tắt khi `SERVICE_MGT_ENABLED=false`. Static fallback.
- **Per-symbol models:** Khi `PER_SYMBOL_ENABLED=true`, mỗi (mã × algo) có instance riêng; prediction lưu với `algorithm_name = f"{key}__ps"`. Seeder base algos: 3996 bot per-symbol + 444 bot pooled = 4440 tổng; `meta_stack` thêm 4 bot pooled + per-symbol bots khi `PER_SYMBOL_ENABLED`. Xem `algo-per-symbol.md`.
- **sim_portfolio_snapshots — upsert theo giờ:** 1 row/session/giờ via `ON CONFLICT (session_id, snapshot_at) DO UPDATE`. `CREATE UNIQUE INDEX` phải TÁCH RIÊNG khỏi DML transaction.
- **Java Auth — nguồn sự thật RBAC:** Flyway V1 (auth + market RBAC), V2 (full_name/email/phone), V3 (command RBAC). Go chỉ là thin proxy. `super_admin` = chon, không thể xóa/reset bởi ai khác.
- **Command RBAC:** Mirror market-group RBAC. `allowed-commands(user)` = UNION qua command_groups, lọc `enabled=true`. super_admin/admin bypass. Catalog upsert từ cli-svc qua `POST /api/command-handlers/upsert` (`X-Internal-Secret` header).
- **Shared feature builder:** `features.py` — `build_basic_features()` (14 features), `build_enhanced_features()` (~30 features). Pandas-ta nếu có, numpy fallback. `MIN_DATA_POINTS = 80`.
- **Optuna:** LightGBM và XGBoost dùng Optuna khi data ≥ 200 và optuna cài (`[ml]` extras). Max 30 trials, timeout 120s. RandomForest không dùng Optuna.
- **Tactic meta-stacking (simulation):** `meta_stack` là CHIẾN THUẬT giao dịch (tầng `simulation/`) — KHÔNG phải thuật toán dự đoán thứ 13, KHÔNG đăng ký vào `algorithms/registry.py` hay `algorithms.go`. `MetaStackModel` (`simulation/meta_stack.py`): đọc tất cả dự đoán các algo + direction accuracy rolling từng algo → LightGBM binary classifier + calibration isotonic → P(up) đã hiệu chỉnh → quyết định + size theo conviction. Fallback reliability-weighted vote khi thiếu checkpoint. `bot.py` nhánh `is_meta` (base_key == "meta_stack") → `_step_meta`. Bot pooled: `meta_stack` (1/market, 4 tổng); per-symbol: `meta_stack__ps` (gate `PER_SYMBOL_ENABLED`). Checkpoint: `${RL_MODEL_DIR}/meta_{market}.pkl` (pooled) / `meta_{market}_{symbol}.pkl` (per-symbol). Training: `train_meta_for_market()` + `train_meta_all()` trong `orchestrator/training.py`; cron `train_meta` (Chủ nhật 8AM); trigger tay: `POST /api/trigger/train` body `{"algorithm":"meta_stack"}`.
- **Crawl sanity guard** (`crawlers/sanity.py`): lọc tick giá rác ở tầng ingest, **không chặn** biến động thật/split. Hai hàm: (1) `check_update(market, key, new_price, last_price)` — "persistence confirmation" 2 nhịp cho daily-live upsert (`upsert_{gold,nasdaq,crypto,sp500}_price` trong `repository.py`): giá lệch ngoài ngưỡng market-aware bị GIỮ pending, chỉ chấp nhận khi crawl kế tiếp xác nhận lại cùng mức; reject → log `crawl.sanity.reject`, giữ giá cũ. (2) `batch_outlier_mask(prices, market)` — so sánh hai phía với bar liền kề trong chuỗi intraday, chỉ flag spike cô lập (lệch cả bar trước lẫn bar sau); điểm biên split không bị flag; log `crawl.sanity.spike_dropped`. Ngưỡng lệch 1-lần: GOLD 0.30, NASDAQ/SP500 0.40, CRYPTO 0.80 — override bằng env `CRAWL_MAX_TICK_DEVIATION_<MARKET>`. Giới hạn đã biết: `crawl_history`/backfill đi qua `check_update` — split thật trong batch lịch sử bị bỏ 1 lần rồi tự lành ở crawl live kế tiếp; `batch_outlier_mask` KHÔNG áp cho backfill historical.
- **Docs viewer (api-svc):** `GET /api/docs` và `GET /api/docs/raw` phục vụ file markdown từ `DOCS_ROOT` (mount read-only `../:/repo:ro`) cho tab Tài liệu trên web dashboard. Chỉ admin. Chống path traversal (reject `..`, chỉ `.md`, kiểm tra prefix `DOCS_ROOT`). Giới hạn 2MB/file.
- **Stock split handling** (chỉ NASDAQ/SP500 — gold/crypto không có split): Crawler (`nasdaq.py`/`sp500.py`) fetch `&events=split` từ Yahoo chart API, parse `events.splits` (numerator/denominator/ex-date) → `repo.record_split(...)` (INSERT ON CONFLICT DO NOTHING, idempotent). Sau khi crawl xong toàn bộ symbol, gọi `apply_pending_splits()` nếu có split mới. History adjustment (`orchestrator/splits.py`): CHỈ chỉnh bảng INTRADAY (`nasdaq_intraday_prices`/`sp500_intraday_prices`) — KHÔNG chỉnh daily (Yahoo daily đã split-adjust sẵn) và KHÔNG chỉnh predictions. Boundary phát hiện từ data (điểm consecutive close rớt ~ratio) chứ không dùng ex-date, vì intraday lưu lẫn scale (bar cũ backfill unadjusted, bar mới crawl adjusted). Chia mọi bar trước boundary cho ratio → chuỗi liên tục ở scale post-split. Idempotent (`applied_at` + sau adjust hết cliff). Non-cliff → mark applied, không đụng giá. Log `crawl.split.detected`, `split.applied`. Simulation split-aware: `engine.py` `_restore_portfolio_state` — bot NASDAQ/SP500 khi tái dựng vị thế mở, mỗi split có `split_date > entry_date`: quantity × ratio, entry_price / ratio (dồn nhiều split); fix bug "lỗ ảo -74%" do giữ vị thế qua split. KHÔNG mutate trade lịch sử. Bảng `stock_splits`: `(market_key, symbol, split_date)` unique; `applied_at NULL` = chưa apply. Repo methods: `record_split`, `list_unapplied_splits`, `list_splits_for_symbol`, `mark_split_applied`.

## Hướng dẫn mở rộng

### 1. Thêm thuật toán mới

1. Tạo `prediction-svc/src/algorithms/<tên>.py` — implement `PredictionAlgorithm`, bắt buộc `get_max_change_pct(self._market_key)` để clamp.
2. Đăng ký trong `prediction-svc/src/algorithms/registry.py` → `build_algorithms()`.
3. Thêm metadata vào `api-svc/pkg/service/predict/registry/algorithms.go` → `init()`.
4. (Tuỳ chọn) Thêm vào Ensemble trong `ensemble.py` — hiện có 10 base: `[ma, ema, lstm, arima, lgbm, sarima, egarch, gru, rf, xgb]`. `rl_dqn` không trong Ensemble.

**Lưu ý:** `build_algorithms()` dùng two-pass — base trước, Ensemble cuối (nhận base instances).

### 2. Thêm thị trường mới

1. Tạo DB model (Go GORM + Python ORM) và migration trong `database.sql`.
2. Thêm repository methods (Python + Go).
3. Tạo crawler `prediction-svc/src/crawlers/<tên>.py` implement `BaseCrawler`.
4. Đăng ký cron job trong `scheduler/jobs.py`.
5. Thêm trigger endpoints Go (`api_trigger_<tên>.go` + proto RPC mới nếu cần).

## Test

```bash
# Go API Backend (từ api-svc/)
cd api-svc && go test ./...
cd api-svc && make test-unit
cd api-svc && make test-coverage     # tạo coverage.html
cd api-svc && make test-db-up        # PostgreSQL port 5433
cd api-svc && make test-db-down

# Python (71 passed, 0 failed — make test-phase5)
cd prediction-svc && make test
docker exec prediction-svc python -m pytest tests/ -v
```

**Conventions:**
- Go: test file cùng package (`algo.go` → `algo_test.go`); integration tests có `if testing.Short() { t.Skip() }`
- Python: `tests/unit/test_<module>.py` (không cần DB); `tests/integration/test_<phase>.py` (cần Docker stack); `pytest.importorskip("torch")` cho optional deps

## Chi tiết tham khảo

@docs/claude/directory-structure.md
@docs/claude/api-endpoints.md
@docs/claude/database.md
