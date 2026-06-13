// Market trading hours — mirror của prediction/src/utils/market_calendar.py.
// NASDAQ & SP500 đóng cửa Thứ 7, Chủ nhật và ngày lễ NYSE (theo giờ US/Eastern).
// GOLD & CRYPTO giao dịch cả cuối tuần → luôn mở.

const NYSE_MARKETS = new Set(['NASDAQ', 'NASDAQ100', 'SP500']);

export type MarketStatus = { open: boolean; reason: 'weekend' | 'holiday' | null };

// Ngày hiện tại theo US/Eastern (không phụ thuộc timezone trình duyệt).
function easternDateParts(now: Date): { y: number; m: number; d: number } {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: 'America/New_York',
    year: 'numeric',
    month: 'numeric',
    day: 'numeric',
  }).formatToParts(now);
  const get = (t: string) => Number(parts.find((p) => p.type === t)?.value);
  return { y: get('year'), m: get('month'), d: get('day') };
}

function ymd(y: number, m: number, d: number): string {
  return `${y}-${String(m).padStart(2, '0')}-${String(d).padStart(2, '0')}`;
}

// Thứ trong tuần của một ngày lịch (0=CN … 6=Thứ 7), dùng UTC để tránh lệch TZ.
function dow(y: number, m: number, d: number): number {
  return new Date(Date.UTC(y, m - 1, d)).getUTCDay();
}

// Ngày `weekday` (0=CN…6=T7) lần thứ n trong tháng → ngày trong tháng.
function nthWeekday(y: number, m: number, weekday: number, n: number): number {
  const firstDow = dow(y, m, 1);
  const offset = (weekday - firstDow + 7) % 7;
  return 1 + offset + (n - 1) * 7;
}

function lastWeekday(y: number, m: number, weekday: number): number {
  const lastDay = new Date(Date.UTC(y, m, 0)).getUTCDate(); // ngày cuối tháng
  const lastDow = dow(y, m, lastDay);
  const offset = (lastDow - weekday + 7) % 7;
  return lastDay - offset;
}

// Lễ Phục sinh (computus Anonymous Gregorian) → {m, d}.
function easter(y: number): { m: number; d: number } {
  const a = y % 19;
  const b = Math.floor(y / 100);
  const c = y % 100;
  const d = Math.floor(b / 4);
  const e = b % 4;
  const f = Math.floor((b + 8) / 25);
  const g = Math.floor((b - f + 1) / 3);
  const h = (19 * a + b - d - g + 15) % 30;
  const i = Math.floor(c / 4);
  const k = c % 4;
  const l = (32 + 2 * e + 2 * i - h - k) % 7;
  const mm = Math.floor((a + 11 * h + 22 * l) / 451);
  const month = Math.floor((h + l - 7 * mm + 114) / 31);
  const day = ((h + l - 7 * mm + 114) % 31) + 1;
  return { m: month, d: day };
}

// Lễ cố định + quy tắc observed (CN→Thứ 2, Thứ 7→Thứ 6) → "YYYY-MM-DD".
function observed(y: number, m: number, d: number): string {
  const w = dow(y, m, d);
  let dt = new Date(Date.UTC(y, m - 1, d));
  if (w === 0) dt = new Date(Date.UTC(y, m - 1, d + 1));
  else if (w === 6) dt = new Date(Date.UTC(y, m - 1, d - 1));
  return ymd(dt.getUTCFullYear(), dt.getUTCMonth() + 1, dt.getUTCDate());
}

function nyseHolidays(y: number): Set<string> {
  const h = new Set<string>();
  // Lễ ngày-cố-định (observed)
  ([[1, 1], [6, 19], [7, 4], [12, 25]] as const).forEach(([m, d]) => h.add(observed(y, m, d)));
  // Lễ theo thứ tự trong tuần (1=Thứ 2 … 4=Thứ 5)
  h.add(ymd(y, 1, nthWeekday(y, 1, 1, 3)));   // MLK Day
  h.add(ymd(y, 2, nthWeekday(y, 2, 1, 3)));   // Presidents' Day
  h.add(ymd(y, 5, lastWeekday(y, 5, 1)));     // Memorial Day
  h.add(ymd(y, 9, nthWeekday(y, 9, 1, 1)));   // Labor Day
  h.add(ymd(y, 11, nthWeekday(y, 11, 4, 4))); // Thanksgiving (Thứ 5)
  // Good Friday = Easter − 2 ngày
  const e = easter(y);
  const gf = new Date(Date.UTC(y, e.m - 1, e.d - 2));
  h.add(ymd(gf.getUTCFullYear(), gf.getUTCMonth() + 1, gf.getUTCDate()));
  return h;
}

/** Trạng thái giao dịch của một market ở thời điểm hiện tại. */
export function marketStatus(marketKey: string, now: Date = new Date()): MarketStatus {
  const key = (marketKey || '').trim().toUpperCase();
  if (!NYSE_MARKETS.has(key)) return { open: true, reason: null };

  const { y, m, d } = easternDateParts(now);
  const w = dow(y, m, d);
  if (w === 0 || w === 6) return { open: false, reason: 'weekend' };
  if (nyseHolidays(y).has(ymd(y, m, d))) return { open: false, reason: 'holiday' };
  return { open: true, reason: null };
}
