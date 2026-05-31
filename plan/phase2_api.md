# Phase 2: gRPC & API Endpoints

Sau khi Phase 1 hoàn thành (cả 3 markets build pass), thêm gRPC triggers và HTTP API.

---

## Bước 1 — Proto definition

Sửa `proto/prediction/prediction.proto`, thêm 6 RPCs:

```protobuf
// NASDAQ 100
rpc TriggerNasdaqCrawler(Empty) returns (TriggerResponse);
rpc TriggerNasdaqPredict(Empty) returns (TriggerResponse);

// Crypto
rpc TriggerCryptoCrawler(Empty) returns (TriggerResponse);
rpc TriggerCryptoPredict(Empty) returns (TriggerResponse);

// Fuel (Giá Xăng VN)
rpc TriggerFuelCrawler(Empty) returns (TriggerResponse);
rpc TriggerFuelPredict(Empty) returns (TriggerResponse);
```

Regenerate:
```bash
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       proto/prediction/prediction.proto
```

---

## Bước 2 — gRPC server handlers

Sửa `pkg/grpc/server/server.go`, thêm 6 handlers:

```go
func (s *Server) TriggerNasdaqCrawler(ctx context.Context, req *pb.Empty) (*pb.TriggerResponse, error) {
    go crawler.CronjobNasdaqCrawler()
    return &pb.TriggerResponse{Success: true, Message: "NASDAQ crawler triggered"}, nil
}

func (s *Server) TriggerNasdaqPredict(ctx context.Context, req *pb.Empty) (*pb.TriggerResponse, error) {
    go orchestrator.RunForMarket(context.Background(), "NASDAQ100")
    return &pb.TriggerResponse{Success: true, Message: "NASDAQ prediction triggered"}, nil
}

func (s *Server) TriggerCryptoCrawler(ctx context.Context, req *pb.Empty) (*pb.TriggerResponse, error) {
    go crawler.CronjobCryptoCrawler()
    return &pb.TriggerResponse{Success: true, Message: "Crypto crawler triggered"}, nil
}

func (s *Server) TriggerCryptoPredict(ctx context.Context, req *pb.Empty) (*pb.TriggerResponse, error) {
    go orchestrator.RunForMarket(context.Background(), "CRYPTO")
    return &pb.TriggerResponse{Success: true, Message: "Crypto prediction triggered"}, nil
}

func (s *Server) TriggerFuelCrawler(ctx context.Context, req *pb.Empty) (*pb.TriggerResponse, error) {
    go crawler.CronjobFuelCrawler()
    return &pb.TriggerResponse{Success: true, Message: "Fuel crawler triggered"}, nil
}

func (s *Server) TriggerFuelPredict(ctx context.Context, req *pb.Empty) (*pb.TriggerResponse, error) {
    go orchestrator.RunForMarket(context.Background(), "FUEL")
    return &pb.TriggerResponse{Success: true, Message: "Fuel prediction triggered"}, nil
}
```

---

## Bước 3 — HTTP Trigger endpoints

Tạo 6 files trigger mới, theo pattern `api_trigger_gold_crawler.go`:

| File | Method | Path | gRPC call |
|------|--------|------|-----------|
| `api_trigger_nasdaq_crawler.go` | POST | `/api/trigger/nasdaq-crawler` | `TriggerNasdaqCrawler` |
| `api_trigger_nasdaq_predict.go` | POST | `/api/trigger/nasdaq-predict` | `TriggerNasdaqPredict` |
| `api_trigger_crypto_crawler.go` | POST | `/api/trigger/crypto-crawler` | `TriggerCryptoCrawler` |
| `api_trigger_crypto_predict.go` | POST | `/api/trigger/crypto-predict` | `TriggerCryptoPredict` |
| `api_trigger_fuel_crawler.go` | POST | `/api/trigger/fuel-crawler` | `TriggerFuelCrawler` |
| `api_trigger_fuel_predict.go` | POST | `/api/trigger/fuel-predict` | `TriggerFuelPredict` |

Tất cả đều dùng `requireGRPCClient(w)` và wrap bằng `AuthRequired()`.

---

## Bước 4 — Data query endpoints

### NASDAQ API (`pkg/server/api_nasdaq.go`)

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/nasdaq/latest` | Giá NASDAQ mới nhất cho tất cả symbols |
| `GET` | `/api/nasdaq/prices` | Danh sách giá NASDAQ (paginated) |
| `GET` | `/api/nasdaq/chart` | Dữ liệu chart — query: `?symbol=AAPL&days=30` |

### NASDAQ Predictions (`pkg/server/api_nasdaq_prediction.go`)

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/nasdaq/predictions/latest` | Dự đoán NASDAQ mới nhất |
| `GET` | `/api/nasdaq/predictions/chart` | Chart dự đoán vs thực tế — query: `?symbol=AAPL&days=30` |
| `GET` | `/api/nasdaq/predictions` | Danh sách dự đoán (paginated) |

### Crypto API (`pkg/server/api_crypto.go`)

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/crypto/latest` | Giá crypto mới nhất |
| `GET` | `/api/crypto/prices` | Danh sách giá crypto (paginated) |
| `GET` | `/api/crypto/chart` | Dữ liệu chart — query: `?coin=bitcoin&days=30` |

### Crypto Predictions (`pkg/server/api_crypto_prediction.go`)

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/crypto/predictions/latest` | Dự đoán crypto mới nhất |
| `GET` | `/api/crypto/predictions/chart` | Chart dự đoán vs thực tế |
| `GET` | `/api/crypto/predictions` | Danh sách dự đoán (paginated) |

### Fuel API (`pkg/server/api_fuel.go`)

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/fuel/latest` | Giá xăng mới nhất cho tất cả sản phẩm |
| `GET` | `/api/fuel/prices` | Danh sách giá xăng (paginated) |
| `GET` | `/api/fuel/chart` | Dữ liệu chart — query: `?product=ron95_iii&days=180` |

### Fuel Predictions (`pkg/server/api_fuel_prediction.go`)

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/fuel/predictions/latest` | Dự đoán giá xăng mới nhất |
| `GET` | `/api/fuel/predictions/chart` | Chart dự đoán vs thực tế |
| `GET` | `/api/fuel/predictions` | Danh sách dự đoán (paginated) |

---

## Bước 5 — Router registration

Sửa `pkg/server/router.go`:

```go
// NASDAQ triggers (JWT required)
router.Handle("/api/trigger/nasdaq-crawler", AuthRequired(http.HandlerFunc(s.TriggerNasdaqCrawler))).Methods("POST")
router.Handle("/api/trigger/nasdaq-predict", AuthRequired(http.HandlerFunc(s.TriggerNasdaqPredict))).Methods("POST")

// NASDAQ data (public)
router.HandleFunc("/api/nasdaq/latest", s.GetNasdaqLatest).Methods("GET")
router.HandleFunc("/api/nasdaq/prices", s.GetNasdaqPrices).Methods("GET")
router.HandleFunc("/api/nasdaq/chart", s.GetNasdaqChart).Methods("GET")
router.HandleFunc("/api/nasdaq/predictions/latest", s.GetNasdaqPredictionsLatest).Methods("GET")
router.HandleFunc("/api/nasdaq/predictions/chart", s.GetNasdaqPredictionsChart).Methods("GET")
router.HandleFunc("/api/nasdaq/predictions", s.GetNasdaqPredictions).Methods("GET")

// Crypto triggers (JWT required)
router.Handle("/api/trigger/crypto-crawler", AuthRequired(http.HandlerFunc(s.TriggerCryptoCrawler))).Methods("POST")
router.Handle("/api/trigger/crypto-predict", AuthRequired(http.HandlerFunc(s.TriggerCryptoPredict))).Methods("POST")

// Crypto data (public)
router.HandleFunc("/api/crypto/latest", s.GetCryptoLatest).Methods("GET")
router.HandleFunc("/api/crypto/prices", s.GetCryptoPrices).Methods("GET")
router.HandleFunc("/api/crypto/chart", s.GetCryptoChart).Methods("GET")
router.HandleFunc("/api/crypto/predictions/latest", s.GetCryptoPredictionsLatest).Methods("GET")
router.HandleFunc("/api/crypto/predictions/chart", s.GetCryptoPredictionsChart).Methods("GET")
router.HandleFunc("/api/crypto/predictions", s.GetCryptoPredictions).Methods("GET")

// Fuel triggers (JWT required)
router.Handle("/api/trigger/fuel-crawler", AuthRequired(http.HandlerFunc(s.TriggerFuelCrawler))).Methods("POST")
router.Handle("/api/trigger/fuel-predict", AuthRequired(http.HandlerFunc(s.TriggerFuelPredict))).Methods("POST")

// Fuel data (public)
router.HandleFunc("/api/fuel/latest", s.GetFuelLatest).Methods("GET")
router.HandleFunc("/api/fuel/prices", s.GetFuelPrices).Methods("GET")
router.HandleFunc("/api/fuel/chart", s.GetFuelChart).Methods("GET")
router.HandleFunc("/api/fuel/predictions/latest", s.GetFuelPredictionsLatest).Methods("GET")
router.HandleFunc("/api/fuel/predictions/chart", s.GetFuelPredictionsChart).Methods("GET")
router.HandleFunc("/api/fuel/predictions", s.GetFuelPredictions).Methods("GET")
```

---

## Checklist

- [ ] `proto/prediction/prediction.proto` — thêm 6 RPCs
- [ ] Regenerate protobuf `*.pb.go`
- [ ] `pkg/grpc/server/server.go` — implement 6 handlers
- [ ] `pkg/server/api_trigger_nasdaq_crawler.go`
- [ ] `pkg/server/api_trigger_nasdaq_predict.go`
- [ ] `pkg/server/api_trigger_crypto_crawler.go`
- [ ] `pkg/server/api_trigger_crypto_predict.go`
- [ ] `pkg/server/api_trigger_fuel_crawler.go`
- [ ] `pkg/server/api_trigger_fuel_predict.go`
- [ ] `pkg/server/api_nasdaq.go` — 3 GET endpoints
- [ ] `pkg/server/api_nasdaq_prediction.go` — 3 GET endpoints
- [ ] `pkg/server/api_crypto.go` — 3 GET endpoints
- [ ] `pkg/server/api_crypto_prediction.go` — 3 GET endpoints
- [ ] `pkg/server/api_fuel.go` — 3 GET endpoints
- [ ] `pkg/server/api_fuel_prediction.go` — 3 GET endpoints
- [ ] `pkg/server/router.go` — đăng ký tất cả routes
- [ ] Test: `go build ./...` pass
