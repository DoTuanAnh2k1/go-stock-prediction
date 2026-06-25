# API Endpoints & gRPC Contract

Tất cả endpoints yêu cầu `Authorization: Bearer <token>` trừ khi ghi khác.

## Auth

| Method | Path | Ghi chú |
|--------|------|---------|
| `POST` | `/api/auth/login` | body: `{"username":"","password":""}` → JWT 24h với claims `sub`, `role`, `user_id`, `accessible_markets` |
| `GET` | `/api/auth/me` | Xác minh token |
| `PUT` | `/api/auth/password` | body: `{"old_password":"","new_password":""}` |

## User Management (admin JWT)

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/users` | Danh sách users |
| `POST` | `/api/users` | body: `{"username","password","role":"user\|admin","full_name?","email?","phone?"}` — 409 nếu trùng |
| `PUT` | `/api/users/{id}` | body: `{"full_name?","email?","phone?","role?"}` — không sửa được super_admin trừ chính super_admin |
| `DELETE` | `/api/users/{id}` | 400 nếu tự xóa hoặc xóa super_admin |
| `POST` | `/api/users/{id}/reset-password` | body: `{"new_password":"..."}` (≥6 ký tự) — super_admin reset admin/user; admin chỉ reset user |
| `GET` | `/api/users/{id}/market-groups` | — |

## Market Groups (admin JWT)

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/market-groups` | — |
| `POST` | `/api/market-groups` | body: `{"name","description"}` |
| `PUT` | `/api/market-groups/{id}` | — |
| `DELETE` | `/api/market-groups/{id}` | — |
| `PUT` | `/api/market-groups/{id}/markets` | body: `{"markets":["GOLD","NASDAQ","CRYPTO","SP500"]}` |
| `GET/POST` | `/api/market-groups/{id}/users` | GET: danh sách; POST body: `{"user_id":123}` |
| `DELETE` | `/api/market-groups/{id}/users/{uid}` | — |

## Command RBAC

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/command-handlers` | Catalog (admin JWT) |
| `POST` | `/api/command-handlers/upsert` | **internal** — header `X-Internal-Secret`; cli-svc gọi khi boot |
| `GET/POST` | `/api/commands` | admin JWT; POST body: `{"name","description","handler_key","args":{},"enabled":true}` |
| `PUT/DELETE` | `/api/commands/{id}` | admin JWT |
| `GET/POST` | `/api/command-groups` | admin JWT |
| `PUT/DELETE` | `/api/command-groups/{id}` | admin JWT |
| `PUT` | `/api/command-groups/{id}/commands` | body: `{"command_ids":[1,2,3]}` |
| `GET/POST/DELETE` | `/api/command-groups/{id}/users` | admin JWT |
| `GET` | `/api/me/commands` | JWT — cli-svc gọi sau login để build allowed-set |

## Predictions / Training

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/predictions/direction-accuracy` | `?market=GOLD\|NASDAQ\|SP500\|CRYPTO` → `{algorithms:[{algorithm, direction_accuracy, total, correct}]}` |
| `GET` | `/api/training/status` | `is_training`, `progress`, `phase` |
| `GET` | `/api/training/algorithms` | config, accuracy, last_trained per algorithm |
| `GET` | `/api/training/metrics` | Aggregate: avg time, success rate |
| `GET` | `/api/markets/{key}/predictions` | Phân trang — key: gold/nasdaq/crypto/sp500 |
| `GET` | `/api/markets/{key}/training` | — |

## Market Data (GOLD / NASDAQ / CRYPTO / SP500)

Pattern: `/api/{market}/{endpoint}` — market ∈ {gold, nasdaq, crypto, sp500}

| Endpoint | Ghi chú |
|----------|---------|
| `latest` | Giá mới nhất |
| `prices` | Danh sách giá |
| `chart` | Dữ liệu biểu đồ — `?days=N` (days=1 → intraday `1h`, else daily `1d`). Trả `dates`/`labels`, `prices` (close; gold: `buy_prices`/`sell_prices`) + mảng OHLC song song `opens`/`highs`/`lows`/`closes` cho biểu đồ nến. Mảng OHLC **rỗng** khi không có data nến (gold nguồn VN, hoặc coin chưa backfill) → frontend fallback line |
| `predictions/latest` | Dự đoán mới nhất |
| `predictions/chart` | Biểu đồ dự đoán vs thực tế |
| `predictions/latest-results` | Kết quả dự đoán mới nhất |
| `predictions` | Danh sách dự đoán |

## Monitoring

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/monitoring/overview` | JWT; cache 30s — `{markets:[{market, crawl, predictions}], bots:{summary}}`; `bots.table` luôn rỗng |
| `GET` | `/api/monitoring/bots` | JWT; server-side paging — `?page&page_size(max200)&market&algorithm&search&sort_by&sort_dir`; default sort_by=return_pct desc; cache 30s chung với /overview |

**Semantics monitoring:** `win_rate`/`profit_factor` chỉ tính SELL đã đóng — dễ gây hiểu nhầm. `return_pct` + `unrealized_pnl` + `open_positions` mới phản ánh thật. `profit_factor=null` (∞) khi 0 lệnh thua — sort null là cao nhất.

## Schedules / Pipeline Reports / Backup / Dashboard

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/schedules` | JWT |
| `PUT` | `/api/schedules/{key}` | body: `{"cron_expression","enabled"}` — validate trước khi lưu |
| `GET` | `/api/pipeline-reports` | JWT; `?pipeline=<key>&limit=<n>` (default 50, max 200) |
| `GET` | `/api/backups` | JWT |
| `GET` | `/api/backups/{filename}` | JWT; stream .sql.gz |
| `DELETE` | `/api/backups/{filename}` | admin JWT |
| `GET` | `/api/dashboard/stats` | — |

## Trigger Endpoints (admin JWT)

Tất cả đều wrap bằng `AdminRequired()` — yêu cầu role `admin` hoặc `super_admin`.

| Method | Path | Ghi chú |
|--------|------|---------|
| `POST` | `/api/trigger/train` | body: `{"algorithm":"lstm_nn"}` (optional) |
| `POST` | `/api/trigger/gold-crawler` | Đồng bộ |
| `POST` | `/api/trigger/gold-history` | Background |
| `POST` | `/api/trigger/gold-predict` | Background |
| `POST` | `/api/trigger/reconcile` | Đồng bộ |
| `POST` | `/api/trigger/historical-backtest` | `?train_window=30&step_size=6&market_key=GOLD\|NASDAQ100\|CRYPTO\|SP500\|ALL`; 202; 409 nếu đang chạy |
| `POST` | `/api/trigger/gold-historical-backtest` | Background |
| `POST` | `/api/trigger/nasdaq-crawler` | Background |
| `POST` | `/api/trigger/nasdaq-predict` | Background |
| `POST` | `/api/trigger/crypto-crawler` | Background |
| `POST` | `/api/trigger/crypto-predict` | Background |
| `POST` | `/api/trigger/crypto-history` | Backfill 180 ngày BTC/ETH/SOL; background |
| `POST` | `/api/trigger/sp500-crawler` | Background |
| `POST` | `/api/trigger/sp500-predict` | Background |
| `POST` | `/api/trigger/backup` | pg_dump → gzip; giữ 10 file gần nhất |
| `POST` | `/api/trigger/simulation-backtest` | Background |
| `POST` | `/api/trigger/simulation-live-step` | — |
| `POST` | `/api/trigger/sim-reset` | Đóng live session cũ, tạo session mới |

## Trigger thủ công (curl)

```bash
TOKEN=$(curl -s -X POST http://localhost:8118/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.token')

curl -X POST http://localhost:8118/api/trigger/gold-crawler -H "Authorization: Bearer $TOKEN"
curl -X POST http://localhost:8118/api/trigger/nasdaq-crawler -H "Authorization: Bearer $TOKEN"
curl -X POST http://localhost:8118/api/trigger/crypto-crawler -H "Authorization: Bearer $TOKEN"
curl -X POST http://localhost:8118/api/trigger/sp500-crawler -H "Authorization: Bearer $TOKEN"
curl -X POST http://localhost:8118/api/trigger/reconcile -H "Authorization: Bearer $TOKEN"
curl -X POST http://localhost:8118/api/trigger/train -H "Authorization: Bearer $TOKEN"
curl -X POST "http://localhost:8118/api/trigger/historical-backtest?train_window=30&step_size=6&market_key=ALL" -H "Authorization: Bearer $TOKEN"

# Cập nhật lịch cron
curl -X PUT http://localhost:8118/api/schedules/crawler_gold \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"cron_expression":"0 0 2 * * *","enabled":true}'
```

## gRPC Service Contract

### Prediction Service (prediction.proto)

`api-svc/proto/prediction/prediction.proto` — Python implements, Go calls.

| RPC | Ghi chú |
|-----|---------|
| `TriggerGoldCrawler` | Đồng bộ |
| `TriggerTrain(TriggerTrainRequest)` | field `algorithm` optional |
| `TriggerReconcile` | Đồng bộ |
| `TriggerGoldHistory` | Background |
| `TriggerGoldPredict` | Background |
| `TriggerHistoricalBacktest(BacktestRequest)` | Background; `market_key`: GOLD/NASDAQ100/CRYPTO/SP500/ALL |
| `TriggerNasdaqCrawler` / `TriggerNasdaqPredict` | Background |
| `TriggerCryptoCrawler` / `TriggerCryptoHistory` / `TriggerCryptoPredict` | Background |
| `TriggerSP500Crawler` / `TriggerSP500Predict` | Background |
| `TriggerSimulationBacktest` / `TriggerSimulationLiveStep` | — |
| `ResetSimBots` | Đóng live sessions cũ, tạo fresh session |
| `GetTrainingStatus` | `is_training`, `progress`, `phase`, `total/done algorithms` |

### Auth Service — Command RBAC RPCs (auth.proto)

**QUAN TRỌNG:** `auth.proto` tồn tại ở HAI nơi — phải đồng bộ:
- `api-svc/proto/auth/auth.proto` → Go stubs (protoc từ api-svc/)
- `auth-svc/src/main/proto/auth.proto` → Java stubs (Maven)

| RPC | Ghi chú |
|-----|---------|
| `UpsertHandlers` | Upsert catalog từ cli-svc boot |
| `ListHandlers` / `ListCommands` | — |
| `CreateCommand` / `UpdateCommand` / `DeleteCommand` | — |
| `ListCommandGroups` / `CreateCommandGroup` / `UpdateCommandGroup` / `DeleteCommandGroup` | — |
| `SetGroupCommands(SetGroupCommandsRequest)` | — |
| `ListCmdGroupUsers` / `AddUserToCmdGroup` / `RemoveUserFromCmdGroup` | — |
| `GetUserCommands(UserRequest)` | Union qua groups, lọc enabled — cli-svc gọi sau login |
