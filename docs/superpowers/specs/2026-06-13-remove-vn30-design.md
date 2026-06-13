# Remove VN30 Market — Design Spec

**Date:** 2026-06-13  
**Scope:** Xóa hoàn toàn VN30 market khỏi hệ thống (crawler, prediction, bot, API, frontend, DB)  
**Approach:** Big Bang — tất cả layers trong một lần

---

## Motivation

VN30 market không còn được sử dụng: kết quả predict thấp, market không hoạt động liên tục. Xóa toàn bộ để giảm complexity.

---

## Section 1 — Python Prediction Service

| File | Action |
|------|--------|
| `prediction/src/crawlers/vn30.py` | DELETE |
| `prediction/src/scheduler/jobs.py` | Xóa `job_crawl_vn30`, `job_predict_vn30`, `job_train_vn30`, VN30 khỏi job registry |
| `prediction/src/scheduler/manager.py` | Xóa `crawler_stock`, `predict_vn30`, `train_vn30` khỏi `DEFAULT_SCHEDULES` |
| `prediction/src/orchestrator/runner.py` | Xóa `_predict_vn30()`, bỏ VN30 khỏi `run_all_markets()` |
| `prediction/src/orchestrator/training.py` | Xóa VN30 khỏi training collection và train loop |
| `prediction/src/database/models.py` | Xóa `Stock`, `StockPrice`, `StockIntradayPrice`, `Exchange`, `Prediction` ORM models |
| `prediction/src/database/repository.py` | Xóa toàn bộ stock/VN30 methods |
| `prediction/src/grpc_server/server.py` | Xóa 4 VN30 RPC handlers: TriggerCrawler, TriggerStockHistory, TriggerStockCrawl, TriggerStockPredict |
| `prediction/src/algorithms/base.py` | Xóa VN30 khỏi `MARKET_MAX_CHANGE` dict |
| `prediction/src/algorithms/registry.py` | Xóa VN30 market key |
| `prediction/src/simulation/engine.py` | Xóa VN30 khỏi simulation markets |
| `prediction/tests/` | Xóa VN30-specific test cases |

---

## Section 2 — Go API Backend

| File | Action |
|------|--------|
| `api/pkg/models/models_svc/viet30.go` | DELETE |
| `api/pkg/models/models_db/stock.go` | DELETE (Stock, StockPrice GORM structs) |
| `api/pkg/models/models_db/migrations.go` | Xóa auto-migrate cho VN30 tables |
| `api/pkg/server/api_stock_detail.go` | DELETE |
| `api/pkg/server/api_stock_actions.go` | DELETE |
| `api/pkg/server/api_trigger_crawler.go` | DELETE |
| `api/pkg/server/api_trigger_predict.go` | DELETE |
| `api/pkg/server/api_trigger_stock_history.go` | DELETE |
| `api/pkg/server/router.go` | Xóa tất cả `/api/stocks/*`, `/api/market/overview`, VN30 trigger routes |
| `api/pkg/store/repository/repository.go` | Xóa `GetVN30Stocks`, `GetAllStocks`, `GetLatestStockPricesForVN30`, ... |
| `api/pkg/store/mysql/stock.go` | DELETE |
| `api/proto/prediction/prediction.proto` | Xóa 4 VN30 RPCs → regenerate Go + Python stubs |
| `api/pkg/service/predict/registry/algorithms.go` | Không thay đổi (algorithms không VN30-specific) |

---

## Section 3 — Frontend

| File | Action |
|------|--------|
| `frontend/src/pages/Stocks.tsx` | DELETE |
| `frontend/src/pages/Dashboard.tsx` | Xóa VN30 KPI card, market breadth, sector breakdown |
| `frontend/src/pages/Simulation.tsx` | Xóa `'VN30'` khỏi market selector |
| `frontend/src/App.tsx` | Xóa route `/markets/vn30`, xóa menu item Stocks |
| `frontend/src/types/index.ts` | Xóa `StockItem` interface, `vn30` boolean field |
| `frontend/src/i18n.ts` | Xóa VN30 labels |

---

## Section 4 — Database (irreversible)

```sql
DROP TABLE IF EXISTS predictions;
DROP TABLE IF EXISTS stock_intraday_prices;
DROP TABLE IF EXISTS stock_prices;
DROP TABLE IF EXISTS stocks;
DROP TABLE IF EXISTS exchanges;
```

- Thực thi thủ công sau khi tất cả code đã deploy
- Go auto-migrate sẽ không recreate vì đã xóa GORM structs
- Proto cần regenerate sau khi chỉnh `.proto` file

---

## Out of Scope

- Các market khác (GOLD, NASDAQ, CRYPTO, SP500) — không thay đổi
- Simulation cho các market khác — giữ nguyên
- Algorithm registry Go-side — không thay đổi
- `direction_accuracy` endpoint — cần test với VN30 bị bỏ (chỉ query `predictions` table đã drop)
