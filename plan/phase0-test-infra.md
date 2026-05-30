# Phase 0: Test Infrastructure Setup (Prerequisite)

**Mục tiêu**: Setup toàn bộ hạ tầng test để các phase sau chỉ cần viết test, không cần config lại.

## 0.1: Test Database

- [ ] Tạo `docker-compose.test.yml` — MySQL riêng cho test (port 3307)
- [ ] Tạo script `scripts/setup-test-db.sh` — init schema + seed data
- [ ] Tạo `testdata/` directory cho fixtures:
  - `testdata/stocks.json` — 5 stocks mẫu (VCB, FPT, VIC, VNM, HPG)
  - `testdata/stock_prices.json` — 30 ngày giá mẫu cho mỗi stock
  - `testdata/predictions.json` — predictions mẫu
  - `testdata/gold_prices.json` — giá vàng mẫu

## 0.2: Test Helpers

- [ ] Tạo `pkg/testutil/db.go`:
  - `SetupTestDB()` — connect test MySQL, auto-migrate, return cleanup func
  - `SeedTestData()` — load fixtures vào DB
  - `CleanupTestDB()` — truncate tables
- [ ] Tạo `pkg/testutil/http.go`:
  - `NewTestServer()` — khởi tạo HTTP server với test DB
  - `MakeRequest()` — helper gọi API và parse response
- [ ] Tạo `pkg/testutil/fixtures.go`:
  - `LoadFixtures()` — đọc JSON fixtures từ `testdata/`

## 0.3: Makefile

- [ ] Thêm targets vào `Makefile`:
  ```makefile
  test:              go test ./... -v -count=1
  test-coverage:     go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out
  test-unit:         go test ./... -short -v
  test-integration:  go test ./... -run Integration -v
  test-db-up:        docker-compose -f docker-compose.test.yml up -d
  test-db-down:      docker-compose -f docker-compose.test.yml down -v
  ```

## 0.4: CI/CD

- [ ] Thêm vào `.github/workflows/test.yml`:
  - Setup Go + MySQL service container
  - Run `make test`
  - Upload coverage report
  - Fail PR nếu coverage giảm

## 0.5: Convention

- File test đặt cùng package: `algo.go` → `algo_test.go`
- Integration test có tag: `func TestIntegration_XXX(t *testing.T) { if testing.Short() { t.Skip() } }`
- Table-driven tests cho mọi trường hợp
- Dùng `testify/assert` hoặc stdlib — chọn 1, dùng xuyên suốt

## Definition of Done

- [ ] `make test` chạy thành công (dù chưa có test case nào)
- [ ] `make test-db-up` khởi tạo test DB
- [ ] Test helpers compile thành công
- [ ] CI pipeline chạy trên GitHub Actions
