# Phase 3: Frontend Pages

Sau khi API endpoints hoạt động, build frontend UI.

---

## NASDAQ 100 Page

Tạo `frontend/src/pages/Nasdaq.tsx` — theo pattern `Gold.tsx`:

### Layout:
1. **KPI cards row** — 4 cards cho top stocks (AAPL, MSFT, NVDA, GOOGL): giá hiện tại, thay đổi %, sparkline
2. **Instrument tabs** — chọn stock để xem chi tiết
3. **Price chart** — LineChart giá close 30/90/180 ngày, toggle timeframe
4. **Predictions section** — bảng dự đoán mới nhất, chart predicted vs actual
5. **Action buttons** — Crawl Now, Predict Now (gọi trigger endpoints, cần JWT)

### API calls:
- `GET /api/nasdaq/latest` → KPI cards
- `GET /api/nasdaq/chart?symbol={sym}&days={n}` → Price chart
- `GET /api/nasdaq/predictions/latest` → Predictions table
- `GET /api/nasdaq/predictions/chart?symbol={sym}` → Prediction chart
- `POST /api/trigger/nasdaq-crawler` → Crawl button
- `POST /api/trigger/nasdaq-predict` → Predict button

### Route:
Sửa `frontend/src/App.tsx`:
```tsx
<Route path="/markets/nasdaq100" element={<ErrorBoundary><Nasdaq /></ErrorBoundary>} />
```

Thêm redirect:
```tsx
<Route path="/nasdaq" element={<Navigate to="/markets/nasdaq100" replace />} />
```

---

## Crypto Page (thay placeholder)

Sửa `frontend/src/pages/Crypto.tsx` — thay toàn bộ placeholder bằng full page theo pattern `Gold.tsx`:

### Layout:
1. **KPI cards** — 2 cards: Bitcoin (BTC), Ethereum (ETH) — giá, market cap, volume 24h, sparkline
2. **Coin tabs** — chọn BTC hoặc ETH
3. **Price chart** — LineChart 30/90/180 ngày
4. **Predictions** — bảng + chart predicted vs actual
5. **Action buttons** — Crawl Now, Predict Now

### API calls:
- `GET /api/crypto/latest` → KPI cards
- `GET /api/crypto/chart?coin={id}&days={n}` → Price chart
- `GET /api/crypto/predictions/latest` → Predictions table
- `GET /api/crypto/predictions/chart?coin={id}` → Prediction chart
- `POST /api/trigger/crypto-crawler` → Crawl button
- `POST /api/trigger/crypto-predict` → Predict button

---

## Fuel Page (Giá Xăng)

Tạo `frontend/src/pages/Fuel.tsx` — theo pattern `Gold.tsx`:

### Layout:
1. **KPI cards row** — 4 cards cho 4 sản phẩm: RON 95-III, E5 RON 92, Diesel, Dầu hỏa
   - Giá hiện tại (VND/lít), thay đổi so với kỳ trước, sparkline
   - Hiển thị: nhân giá từ API (nghìn VND) × 1000 → VND đầy đủ
2. **Product tabs** — chọn sản phẩm xăng dầu
3. **Price chart** — LineChart 90/180/365 ngày (timeframe dài hơn do dữ liệu thưa ~52 kỳ/năm)
4. **Predictions section** — bảng + chart predicted vs actual
5. **Action buttons** — Crawl Now, Predict Now
6. **Info banner** — ghi chú: "Giá xăng điều chỉnh theo chu kỳ ~7 ngày"

### API calls:
- `GET /api/fuel/latest` → KPI cards
- `GET /api/fuel/chart?product={id}&days={n}` → Price chart
- `GET /api/fuel/predictions/latest` → Predictions table
- `GET /api/fuel/predictions/chart?product={id}` → Prediction chart
- `POST /api/trigger/fuel-crawler` → Crawl button
- `POST /api/trigger/fuel-predict` → Predict button

### Route:
Sửa `frontend/src/App.tsx`:
```tsx
<Route path="/markets/fuel" element={<ErrorBoundary><Fuel /></ErrorBoundary>} />
```

Thêm redirect:
```tsx
<Route path="/fuel" element={<Navigate to="/markets/fuel" replace />} />
```

---

## Navigation

Sửa sidebar/nav component:

```
Markets
  ├── VN30          → /markets/vn30
  ├── Gold          → /markets/gold
  ├── NASDAQ 100    → /markets/nasdaq100    ← NEW
  ├── Crypto        → /markets/crypto       (cập nhật từ placeholder)
  └── Giá Xăng      → /markets/fuel         ← NEW
```

---

## DataContext integration

Sửa `frontend/src/context/DataContext.tsx` (hoặc tương đương):
- Thêm fetch cho `/api/nasdaq/latest`, `/api/crypto/latest`, `/api/fuel/latest` vào initial data load
- Expose `nasdaqSources`, `cryptoSources`, `fuelSources` trong context data

---

## API helper functions

Sửa `frontend/src/api.ts` (hoặc tương đương):

```ts
// NASDAQ
export const crawlNasdaq = () => authPost('/api/trigger/nasdaq-crawler');
export const predictNasdaq = () => authPost('/api/trigger/nasdaq-predict');

// Crypto
export const crawlCrypto = () => authPost('/api/trigger/crypto-crawler');
export const predictCrypto = () => authPost('/api/trigger/crypto-predict');

// Fuel
export const crawlFuel = () => authPost('/api/trigger/fuel-crawler');
export const predictFuel = () => authPost('/api/trigger/fuel-predict');
```

---

## Checklist

- [ ] `frontend/src/pages/Nasdaq.tsx` — full page
- [ ] `frontend/src/pages/Crypto.tsx` — thay placeholder
- [ ] `frontend/src/pages/Fuel.tsx` — full page
- [ ] `frontend/src/App.tsx` — thêm routes NASDAQ + Fuel
- [ ] Sidebar/nav — thêm NASDAQ + Fuel links
- [ ] `frontend/src/context/DataContext.tsx` — fetch data mới
- [ ] `frontend/src/api.ts` — helper functions
- [ ] Docker rebuild frontend: `docker-compose build frontend`
- [ ] Visual test trên browser
