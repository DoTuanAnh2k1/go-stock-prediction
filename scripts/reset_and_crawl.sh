#!/usr/bin/env bash
# =============================================================================
# reset_and_crawl.sh — Xóa sạch dữ liệu và crawl lại toàn bộ
#
# Cách dùng:
#   ./scripts/reset_and_crawl.sh              # crawl 365 ngày, không xóa DB
#   ./scripts/reset_and_crawl.sh --reset      # xóa sạch DB rồi crawl lại
#   ./scripts/reset_and_crawl.sh --days 90    # crawl 90 ngày
#   ./scripts/reset_and_crawl.sh --reset --days 180
#
# Yêu cầu: docker-compose đang chạy (mysql_db + go_stock_prediction_app)
# =============================================================================

set -euo pipefail

# ── Config ───────────────────────────────────────────────────────────────────
APP_URL="${APP_URL:-http://localhost:31300}"
MYSQL_CONTAINER="${MYSQL_CONTAINER:-mysql_db}"
MYSQL_DB="${MYSQL_DB:-go_stock_prediction}"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_PASS="${MYSQL_PASS:-123}"
DAYS=365
DO_RESET=0

# ── Parse args ───────────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --reset)   DO_RESET=1; shift ;;
    --days)    DAYS="$2"; shift 2 ;;
    *) echo "Unknown arg: $1" >&2; exit 1 ;;
  esac
done

# ── Helpers ──────────────────────────────────────────────────────────────────
green()  { echo -e "\033[32m$*\033[0m"; }
yellow() { echo -e "\033[33m$*\033[0m"; }
red()    { echo -e "\033[31m$*\033[0m"; }
bold()   { echo -e "\033[1m$*\033[0m"; }

mysql_exec() {
  docker exec "$MYSQL_CONTAINER" mysql -u"$MYSQL_USER" -p"$MYSQL_PASS" \
    --silent --skip-column-names "$MYSQL_DB" -e "$1" 2>/dev/null
}

trigger() {
  local name="$1" url="$2"
  yellow "→ $name ..."
  local code
  code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$url")
  if [[ "$code" == "200" || "$code" == "202" ]]; then
    green "  ✓ $name started (HTTP $code)"
  else
    red "  ✗ $name failed (HTTP $code)"
    return 1
  fi
}

wait_for_app() {
  echo -n "Chờ app sẵn sàng"
  local i=0
  until curl -sf "$APP_URL/health/simple" &>/dev/null; do
    sleep 2; echo -n "."; ((i++))
    if [[ $i -gt 30 ]]; then red " TIMEOUT"; exit 1; fi
  done
  green " OK"
}

wait_seconds() {
  local sec="$1" msg="$2"
  yellow "⏱  $msg ($sec giây)..."
  sleep "$sec"
}

# ── Main ─────────────────────────────────────────────────────────────────────
bold "======================================================"
bold " VNStock — Reset & Crawl"
bold " App:  $APP_URL"
bold " Days: $DAYS ngày lịch sử"
bold " Reset DB: $([ $DO_RESET -eq 1 ] && echo 'CÓ' || echo 'KHÔNG')"
bold "======================================================"
echo

# Kiểm tra app đang chạy
wait_for_app

# ── Bước 1: Reset DB (nếu --reset) ──────────────────────────────────────────
if [[ $DO_RESET -eq 1 ]]; then
  yellow "🗑  Xóa sạch dữ liệu..."
  mysql_exec "SET FOREIGN_KEY_CHECKS=0;
    TRUNCATE TABLE stock_prices;
    TRUNCATE TABLE predictions;
    TRUNCATE TABLE gold_prices;
    TRUNCATE TABLE gold_predictions;
    TRUNCATE TABLE training_logs;
    TRUNCATE TABLE sync_logs;
    SET FOREIGN_KEY_CHECKS=1;"
  green "  ✓ Đã xóa: stock_prices, predictions, gold_prices, gold_predictions, training_logs, sync_logs"
  echo
fi

# ── Bước 2: Crawl lịch sử giá cổ phiếu VN30 ────────────────────────────────
bold "📈 Bước 1/4 — Crawl lịch sử cổ phiếu ($DAYS ngày)"
trigger "Stock History" "$APP_URL/api/trigger/stock-history?days=$DAYS"
wait_seconds 120 "Đang crawl 30 mã VN30 × $DAYS ngày (mất khoảng 2-5 phút)"

# Kiểm tra kết quả
COUNT=$(mysql_exec "SELECT COUNT(*) FROM stock_prices;" 2>/dev/null || echo 0)
green "  → Đã có $COUNT bản ghi trong stock_prices"
echo

# ── Bước 3: Crawl lịch sử vàng ──────────────────────────────────────────────
bold "🥇 Bước 2/4 — Crawl lịch sử giá vàng"
trigger "Gold History" "$APP_URL/api/trigger/gold-history"
wait_seconds 30 "Đang crawl giá vàng SJC + XAU/USD"

COUNT=$(mysql_exec "SELECT COUNT(*) FROM gold_prices;" 2>/dev/null || echo 0)
green "  → Đã có $COUNT bản ghi trong gold_prices"
echo

# ── Bước 4: Crawl giá hôm nay ───────────────────────────────────────────────
bold "📊 Bước 3/4 — Crawl giá cổ phiếu hôm nay"
trigger "Crawler (today)" "$APP_URL/api/trigger/crawler"
wait_seconds 20 "Đang crawl giá hiện tại"
echo

# ── Bước 5: Chạy dự đoán ────────────────────────────────────────────────────
bold "🔮 Bước 4/4 — Chạy dự đoán ML"
trigger "Predict" "$APP_URL/api/trigger/predict"
wait_seconds 60 "Đang chạy LSTM + ARIMA-GARCH + Moving Average"

COUNT=$(mysql_exec "SELECT COUNT(*) FROM predictions;" 2>/dev/null || echo 0)
green "  → Đã có $COUNT dự đoán trong predictions"
echo

# ── Tổng kết ─────────────────────────────────────────────────────────────────
bold "======================================================"
bold " ✅ Hoàn thành!"
bold "======================================================"
mysql_exec "SELECT
  (SELECT COUNT(*) FROM stocks) as stocks,
  (SELECT COUNT(*) FROM stock_prices) as stock_prices,
  (SELECT COUNT(*) FROM gold_prices) as gold_prices,
  (SELECT COUNT(*) FROM predictions) as predictions;" \
  | awk '{printf "  Stocks: %s | StockPrices: %s | GoldPrices: %s | Predictions: %s\n", $1,$2,$3,$4}'
echo
green "  Dashboard: $APP_URL"
green "  Gold:      $APP_URL/gold"
