import type {
  AppData, FmtUtils, StockItem, MoverItem, PredictionItem,
  ConfirmedItem, AlgoItem, GoldSource, GoldPred, GoldPredActual,
  GoldDetailItem, IndexData, AccTrendSeries
} from '../types';

const BASE = '/api';

// ── Formatting utils ────────────────────────────────────────────────────────
export const fmt: FmtUtils = {
  vnd: (n) => n.toLocaleString('vi-VN'),
  price: (n) => n.toLocaleString('vi-VN', { minimumFractionDigits: 2, maximumFractionDigits: 2 }),
  pct: (n) => (n > 0 ? '+' : '') + n.toFixed(2) + '%',
  sign: (n) => (n > 0 ? '+' : '') + n.toFixed(2),
  compact: (n) => n >= 1e6 ? (n / 1e6).toFixed(1) + 'M' : n >= 1e3 ? (n / 1e3).toFixed(1) + 'K' : '' + n,
  goldShort: (n) => n >= 1e6 ? (n / 1e6).toFixed(2) + 'tr' : n.toLocaleString('vi-VN'),
};

// ── Helpers ─────────────────────────────────────────────────────────────────
function num(x: any): number {
  const n = typeof x === 'number' ? x : parseFloat(x);
  return isFinite(n) ? n : 0;
}
function arr<T>(x: any): T[] {
  return Array.isArray(x) ? x : [];
}
function pickList(d: any): any[] {
  return Array.isArray(d) ? d : (d && (d.data || d.items || d.stocks || d.predictions)) || [];
}
function ddmm(s: any): string {
  if (!s) return '';
  try {
    const d = new Date(s);
    if (isNaN(d.getTime())) return String(s).slice(0, 10);
    return ('0' + d.getDate()).slice(-2) + '/' + ('0' + (d.getMonth() + 1)).slice(-2);
  } catch (_e) {
    return String(s).slice(0, 10);
  }
}
function conf(c: any): number {
  let n = num(c);
  if (n > 0 && n <= 1) n *= 100;
  return Math.round(n);
}

// Static fallback: CSS class + short badge label only.
// The display name (`.name`) is overridden by /api/training/algorithms at runtime.
// Aliases (e.g. "lstm" → "lstm_nn") are kept so partial keys from other endpoints still resolve.
const ALGO_FALLBACK: Record<string, { short: string; cls: string; name: string }> = {
  lstm_nn:       { short: 'LSTM',  cls: 'lstm',  name: 'LSTM Neural Network' },
  lstm:          { short: 'LSTM',  cls: 'lstm',  name: 'LSTM Neural Network' },
  arima_garch:   { short: 'ARIMA', cls: 'arima', name: 'ARIMA-GARCH' },
  arima:         { short: 'ARIMA', cls: 'arima', name: 'ARIMA-GARCH' },
  moving_average:{ short: 'MA',    cls: 'ma',    name: 'Moving Average (VWMA)' },
  ma:            { short: 'MA',    cls: 'ma',    name: 'Moving Average (VWMA)' },
  ensemble:      { short: 'ENS',   cls: 'ens',   name: 'Ensemble' },
  ens:           { short: 'ENS',   cls: 'ens',   name: 'Ensemble' },
  ema:           { short: 'EMA',   cls: 'ema',   name: 'Exponential Moving Average' },
  sarima:        { short: 'SARM', cls: 'sarima', name: 'SARIMA' },
  egarch:        { short: 'EGA',  cls: 'egarch', name: 'EGARCH' },
  gru_nn:        { short: 'GRU',  cls: 'gru',    name: 'GRU Neural Network' },
  gru:           { short: 'GRU',  cls: 'gru',    name: 'GRU Neural Network' },
  random_forest: { short: 'RF',   cls: 'rf',     name: 'Random Forest' },
  xgboost:       { short: 'XGB',  cls: 'xgb',    name: 'XGBoost' },
};

// Runtime algo map — starts as a copy of the fallback, gets enriched from
// /api/training/algorithms in loadAll() so names always reflect the backend registry.
let _algoMap: Record<string, { short: string; cls: string; name: string }> = { ...ALGO_FALLBACK };

/** Look up display metadata for an algorithm key, with a safe fallback. */
function algoMeta(n: string): { short: string; cls: string; name: string } {
  return _algoMap[(n || '').toLowerCase()] || ALGO_FALLBACK[(n || '').toLowerCase()] || {
    short: (n || '?').toUpperCase().slice(0, 5),
    cls: 'unknown',
    name: n || '—',
  };
}

// Deterministic color palette for dynamic algorithm series.
const ALGO_COLORS: string[] = [
  'oklch(0.74 0.13 200)', // blue-ish (LSTM)
  'var(--gold)',           // gold (ARIMA)
  'var(--up)',             // green (MA)
  'oklch(0.72 0.18 150)', // teal (EMA)
  'oklch(0.72 0.14 300)', // purple (Ensemble)
  'oklch(0.75 0.15 30)',  // orange
  'oklch(0.70 0.18 260)', // violet
  'oklch(0.72 0.18 50)',   // amber  (SARIMA)
  'oklch(0.70 0.15 340)',  // rose   (EGARCH)
  'oklch(0.74 0.16 220)',  // sky    (GRU)
  'oklch(0.71 0.14 160)',  // emerald (Random Forest)
  'oklch(0.73 0.17 280)',  // indigo (XGBoost)
];
// Known ordering for stable color assignment across refreshes.
const ALGO_ORDER = ['lstm_nn', 'arima_garch', 'moving_average', 'ema', 'ensemble', 'sarima', 'egarch', 'gru_nn', 'random_forest', 'xgboost'];
function algoColor(key: string, idx: number): string {
  const canonical = ALGO_ORDER.indexOf(key);
  return ALGO_COLORS[canonical >= 0 ? canonical : idx % ALGO_COLORS.length];
}

function fetchJSON(url: string, opts?: RequestInit): Promise<any> {
  const token = localStorage.getItem('vns_token');
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (token) headers['Authorization'] = 'Bearer ' + token;
  return fetch(BASE + url, Object.assign({ headers }, opts || {}))
    .then((r) => { if (!r.ok) throw new Error('HTTP ' + r.status); return r.json(); });
}

// ── Build empty ─────────────────────────────────────────────────────────────
export function buildEmpty(): AppData {
  return {
    fmt,
    stocks: [], predictions: [], confirmed: [], algos: [],
    gainers: [], losers: [], active: [],
    goldSources: [], goldPreds: [], goldPredActual: null, goldDetail: [],
    stats: { total: 0, acc: 0 },
    indices: {
      vnindex: { val: 0, chg: 0, chgPct: 0, vol: 0, series: [] },
    },
    accTrend: { labels: [], series: [] },
    dailyCounts: { labels: [], values: [] },
    trainLogs: [], trainJobs: [],
    algoMap: { ...ALGO_FALLBACK },
    __live: false, __sources: {},
  };
}

// ── Stock history (sparklines) ───────────────────────────────────────────────
export function stockHistory(sym: string, days?: number): Promise<number[]> {
  return fetchJSON('/stocks/' + encodeURIComponent(sym) + '/history?days=' + (days || 30))
    .then((res: any) => {
      let list = pickList(res);
      if (!list.length && res && res.price_data) list = res.price_data;
      return list
        .slice()
        .sort((a: any, b: any) =>
          new Date(a.date || a.trading_date || a.created_at).getTime() -
          new Date(b.date || b.trading_date || b.created_at).getTime()
        )
        .map((p: any) => num(p.close_price || p.close || p.price))
        .filter((v: number) => v > 0);
    })
    .catch(() => []);
}

// ── Gold chart ───────────────────────────────────────────────────────────────
export function goldChart(source: string, product: string, days?: number): Promise<{ labels: string[]; buy: number[]; sell: number[]; granularity?: string }> {
  return fetchJSON('/gold/chart?source=' + source + '&product_type=' + product + '&days=' + (days || 180))
    .then((j: any) => ({
      labels: arr<string>(j.labels),
      buy:    arr<number>(j.buy_prices).map(num),
      sell:   arr<number>(j.sell_prices).map(num),
      granularity: j.granularity ?? '1d',
    }))
    .catch(() => ({ labels: [], buy: [], sell: [], granularity: '1d' }));
}

// ── Market page (backend pagination + sort) ──────────────────────────────────
export function fetchMarketPage(params: {
  page?: number;
  pageSize?: number;
  sortBy?: 'price' | 'change_percent';
  sortOrder?: 'asc' | 'desc';
  q?: string;
  sector?: string;
}): Promise<any> {
  const p = new URLSearchParams();
  if (params.page) p.set('page', String(params.page));
  if (params.pageSize) p.set('page_size', String(params.pageSize));
  if (params.sortBy) p.set('sort_by', params.sortBy);
  if (params.sortOrder) p.set('sort_order', params.sortOrder);
  if (params.q) p.set('q', params.q);
  if (params.sector) p.set('sector', params.sector);
  return fetchJSON('/market/overview?' + p.toString());
}

// ── Builders: live API → component shape ────────────────────────────────────
function buildStocks(ov: any): StockItem[] | null {
  const list = arr<any>(ov.stocks);
  if (!list.length) return null;
  return list.map((s: any) => ({
    sym: s.symbol || '',
    name: s.company_name || s.symbol || '',
    sector: s.sector || '—',
    exchange: s.exchange || 'HOSE',
    price: num(s.current_price),
    change: num(s.change),
    chgPct: num(s.change_percent),
    volume: num(s.volume) / 1e6,
    value: num(s.value) / 1e12,
    spark: [],
    hist: [],
  }));
}

function mover(s: any): MoverItem {
  const sym = (s.stock && s.stock.symbol) || s.symbol || '';
  return {
    sym, name: s.company_name || sym,
    price: num(s.current_price),
    chgPct: num(s.change_percent),
    volume: num(s.volume) / 1e6,
    spark: [],
  };
}

function buildPredsLatest(res: any): PredictionItem[] | null {
  const list = arr<any>((res && (res.data && res.data.predictions)) || (res && res.predictions) || res);
  if (!list.length) return null;
  return list.map((p: any) => {
    const m = algoMeta(p.algorithm_name);
    const cur = num(p.current_price), pred = num(p.predicted_price);
    const sym = (p.stock && p.stock.symbol) || p.symbol || '—';
    return {
      sym, name: (p.stock && p.stock.company_name) || sym,
      algo: p.algorithm_name, algoShort: m.short, algoCls: m.cls,
      cur, pred,
      deltaPct: cur ? +(((pred - cur) / cur) * 100).toFixed(2) : 0,
      conf: conf(p.confidence),
      date: ddmm(p.prediction_date),
      target: ddmm(p.target_date),
    };
  });
}

function buildConfirmed(res: any): ConfirmedItem[] | null {
  const list = arr<any>((res && (res.data && res.data.predictions)) || (res && res.predictions) || res)
    .filter((p: any) => p.actual_price != null);
  if (!list.length) return null;
  return list.map((p: any) => {
    const m = algoMeta(p.algorithm_name);
    const cur = num(p.current_price), pred = num(p.predicted_price), act = num(p.actual_price);
    const acc = p.accuracy != null
      ? conf(p.accuracy)
      : Math.max(50, +(100 - Math.abs(pred - act) / (act || 1) * 100 * 8).toFixed(1));
    const sym = (p.stock && p.stock.symbol) || p.symbol || '—';
    return {
      sym, name: (p.stock && p.stock.company_name) || sym,
      algo: p.algorithm_name, algoShort: m.short, algoCls: m.cls,
      pred, actual: act, cur,
      predDelta: cur ? +(((pred - cur) / cur) * 100).toFixed(2) : 0,
      actualDelta: cur ? +(((act - cur) / cur) * 100).toFixed(2) : 0,
      conf: conf(p.confidence), acc, err: +Math.abs((pred - act) / (act || 1) * 100).toFixed(2),
      target: ddmm(p.target_date),
      status: (acc > 85 ? 'hit' : acc > 72 ? 'close' : 'miss') as 'hit' | 'close' | 'miss',
    };
  });
}

/**
 * Build algo list from /api/predictions/accuracy (accList) enriched with
 * /api/training/algorithms (algoDefsRaw) which provides authoritative display names.
 * Either source alone still produces a valid result.
 */
function buildAlgos(accList: any, algoDefsRaw?: any): AlgoItem[] | null {
  // Index /api/training/algorithms by key for O(1) name lookup.
  const defsByKey: Record<string, { name: string; status: string; last_trained?: string | null; accuracy: number; training_time_seconds: number }> = {};
  const rawDefs = arr<any>((algoDefsRaw && (algoDefsRaw.data || algoDefsRaw)) || []);
  for (const d of rawDefs) {
    if (d.key) defsByKey[d.key] = d;
  }

  const list = arr<any>(accList.data || accList);
  if (!list.length && !rawDefs.length) return null;

  // If accuracy list is empty but we have training defs, show defs directly.
  const source = list.length ? list : rawDefs.map((d: any) => ({
    algorithm_name: d.key,
    accuracy_rate: d.accuracy * 100,
    avg_error: 0,
  }));
  if (!source.length) return null;

  return source.map((a: any) => {
    const key = a.algorithm_name || a.key || '';
    const def = defsByKey[key];
    const m = algoMeta(key);
    // Prefer name from /api/training/algorithms; fall back to the static ALGO_FALLBACK.
    const displayName = (def && def.name) ? def.name : m.name;
    const trainedAt = def && def.last_trained ? def.last_trained.slice(0, 10) : '—';
    return {
      id: key, short: m.short, name: displayName, cls: m.cls,
      desc: '', acc: +num(a.accuracy_rate).toFixed(1), accDelta: 0,
      mae: +num(a.avg_error).toFixed(2), trainedAt, epochs: null,
      status: (def && def.status) || 'trained',
    };
  });
}

function buildTrend(t: any): AppData['accTrend'] | null {
  if (Array.isArray(t)) {
    // Collect all weeks and all algorithm keys seen in the data.
    const by: Record<string, Record<string, number>> = {};
    const keysSet = new Set<string>();
    t.forEach((r: any) => {
      const w = r.week || r.date || '';
      (by[w] = by[w] || {})[r.algorithm_name] = num(r.accuracy_rate);
      if (r.algorithm_name) keysSet.add(r.algorithm_name);
    });
    const ws = Object.keys(by).sort();
    if (!ws.length) return null;
    // Sort keys by known order for stable color assignment.
    const keys = Array.from(keysSet).sort((a, b) => {
      const ai = ALGO_ORDER.indexOf(a), bi = ALGO_ORDER.indexOf(b);
      return (ai < 0 ? 999 : ai) - (bi < 0 ? 999 : bi);
    });
    const series: AccTrendSeries[] = keys.map((key, idx) => ({
      key,
      name: algoMeta(key).name,
      color: algoColor(key, idx),
      data: ws.map((w) => by[w][key] != null ? by[w][key] : null),
    }));
    return { labels: ws.map(ddmm), series };
  }
  if (t && t.weeks) {
    // Object format: { weeks: string[], <key>: number[] }
    const weeks: string[] = arr<string>(t.weeks);
    if (!weeks.length) return null;
    const keys = Object.keys(t).filter((k) => k !== 'weeks');
    const series: AccTrendSeries[] = keys.map((key, idx) => ({
      key,
      name: algoMeta(key).name,
      color: algoColor(key, idx),
      data: arr<number>(t[key]).map(num),
    }));
    return { labels: weeks.map(ddmm), series };
  }
  return null;
}

const GOLD_SRC: Record<string, string> = { BTMC: 'BTMC', BTMH: 'BTMH', SJC: 'SJC', DOJI: 'DOJI', PNJ: 'PNJ', PHUQUY: 'Phú Quý', XAU: 'XAU/USD' };
const GOLD_PROD: Record<string, string> = { sjc: 'Miếng', nhan_tron: 'Nhẫn Tròn', spot: '' };

function buildGold(latest: any): GoldSource[] | null {
  if (!latest) return null;
  const list = arr<any>(latest.data);
  if (!list.length) return null;
  return list.map((g: any) => {
    const nm = (GOLD_SRC[g.source] || g.source) + (GOLD_PROD[g.product_type] ? ' ' + GOLD_PROD[g.product_type] : '');
    return {
      id: (g.source + '_' + g.product_type).toLowerCase(),
      name: nm, vendor: GOLD_SRC[g.source] || g.source,
      region: '—', unit: g.product_type === 'spot' ? 'oz' : 'lượng',
      source: g.source, product: g.product_type,
      buy: num(g.buy_price), sell: num(g.sell_price),
      chg: 0, chgPct: 0, spark: [], hist: [],
    };
  });
}

function buildGoldPreds(res: any): GoldPred[] | null {
  if (!res) return null;
  const list = arr<any>(res.data || res);
  if (!list.length) return null;
  return list.map((p: any) => {
    const m = algoMeta(p.algorithm_name);
    const cur = num(p.current_price), pred = num(p.predicted_price);
    return {
      src: (GOLD_SRC[p.source] || p.source) + (GOLD_PROD[p.product_type] ? ' ' + GOLD_PROD[p.product_type] : ''),
      type: p.product_type === 'spot' ? 'Spot' : 'Bán',
      algo: p.algorithm_name, algoShort: m.short, algoCls: m.cls,
      cur, pred,
      deltaPct: cur ? +(((pred - cur) / cur) * 100).toFixed(2) : 0,
      conf: conf(p.confidence),
      date: ddmm(p.prediction_date),
      isOz: p.source === 'XAU',
      accuracy: p.accuracy != null ? p.accuracy : null,
    };
  });
}

function buildGoldPredActual(res: any): GoldPredActual | null {
  if (!res) return null;
  const list = arr<any>(res.data || res);
  if (!list.length) return null;
  return {
    labels: list.map((d: any) => ddmm(d.date)),
    actual: list.map((d: any) => d.actual_price != null ? num(d.actual_price) : null),
    pred:   list.map((d: any) => d.predicted_price != null ? num(d.predicted_price) : null),
  };
}

// ── Master loader ────────────────────────────────────────────────────────────
export function loadAll(): Promise<{ data: AppData; raw: Record<string, any> }> {
  const P = (u: string) => fetchJSON(u).catch(() => null);
  const jobs: Record<string, Promise<any>> = {
    stats:        P('/dashboard/stats'),
    algoDefs:     P('/training/algorithms'),
    goldLatest:   P('/gold/latest'),
    goldPreds:    P('/gold/predictions/latest'),
    goldPredChart:P('/gold/predictions/chart?source=BTMC&product_type=sjc&algorithm=ensemble&days=60'),
    goldDetail:   goldChart('SJC', 'sjc', 12).catch(() => null),
  };
  const keys = Object.keys(jobs);
  return Promise.all(keys.map((k) => jobs[k])).then((vals) => {
    const R: Record<string, any> = {};
    keys.forEach((k, i) => { R[k] = vals[i]; });
    const out = buildEmpty();
    const sources: Record<string, boolean> = {};

    // ── Build runtime algoMap from /api/training/algorithms ─────────────────
    // This ensures display names always reflect the backend registry.
    const rawDefs = arr<any>((R.algoDefs && (R.algoDefs.data || R.algoDefs)) || []);
    if (rawDefs.length) {
      const merged: Record<string, { short: string; cls: string; name: string }> = { ...ALGO_FALLBACK };
      for (const d of rawDefs) {
        if (!d.key) continue;
        const fallback = ALGO_FALLBACK[d.key] || { short: d.key.toUpperCase().slice(0, 5), cls: 'unknown', name: d.key };
        merged[d.key] = { short: fallback.short, cls: fallback.cls, name: d.name || fallback.name || d.key };
      }
      _algoMap = merged;
      out.algoMap = merged;
    }

    // Pass training algorithm defs so buildAlgos can use server-provided names.
    const al = buildAlgos({}, R.algoDefs); if (al && al.length) { out.algos = al; sources.acc = true; }

    if (R.stats) {
      out.stats = { total: R.stats.total_predictions || 0, acc: +num(R.stats.avg_accuracy).toFixed(1) };
      sources.stats = true;
    }

    const gs = buildGold(R.goldLatest); if (gs) { out.goldSources = gs; sources.gold = true; }
    const gp = buildGoldPreds(R.goldPreds); if (gp) { out.goldPreds = gp; }
    const gpa = buildGoldPredActual(R.goldPredChart); if (gpa) { out.goldPredActual = gpa; }
    if (R.goldDetail && R.goldDetail.labels && R.goldDetail.labels.length) {
      const L = R.goldDetail.labels, B = R.goldDetail.buy, S = R.goldDetail.sell;
      out.goldDetail = (L as string[]).map((d: string, i: number) => ({
        date: d, buy: B[i] || 0, sell: S[i] || 0, spread: (S[i] || 0) - (B[i] || 0),
      })).slice(-10).reverse();
    }

    out.__live = Object.keys(sources).length > 0;
    out.__sources = sources;
    return { data: out, raw: R };
  });
}

// ── Phase 2: sparklines + gold history (non-blocking) ───────────────────────
export function enrich(data: AppData, onUpdate: () => void): void {
  if (data.__sources && data.__sources.gold) {
    Promise.allSettled(
      data.goldSources.map((g) =>
        goldChart(g.source, g.product, 180).then((c) => {
          if (c.sell.length) {
            g.hist = c.sell; g.spark = c.sell.slice(-24); g.histLabels = c.labels;
            const prev = c.sell[c.sell.length - 2] || g.sell;
            g.chg = g.sell - prev;
            g.chgPct = prev ? +((g.chg / prev) * 100).toFixed(2) : 0;
          }
        })
      )
    ).then(() => { onUpdate && onUpdate(); });
  }
}

// ── Triggers ─────────────────────────────────────────────────────────────────
function trigger(path: string): Promise<boolean> {
  return fetch(BASE + path, { method: 'POST' }).then((r) => r.ok).catch(() => false);
}

export const crawlGold    = () => trigger('/trigger/gold-crawler');
export const predictGold  = () => trigger('/trigger/gold-predict');
export const train        = () => trigger('/trigger/train');
export const goldBacktest = () => fetch(BASE + '/trigger/gold-historical-backtest', {
  method: 'POST',
  headers: { Authorization: 'Bearer ' + getToken() },
}).then((r) => r.ok).catch(() => false);

// ── NASDAQ ───────────────────────────────────────────────────────────────────
export const crawlNasdaq   = () => trigger('/trigger/nasdaq-crawler');
export const predictNasdaq = () => trigger('/trigger/nasdaq-predict');

// ── Crypto ───────────────────────────────────────────────────────────────────
export const crawlCrypto   = () => trigger('/trigger/crypto-crawler');
export const predictCrypto = () => trigger('/trigger/crypto-predict');

// ── Schedule management ──────────────────────────────────────────────────────
export interface ScheduleItem {
  job_key: string;
  job_name: string;
  cron_expression: string;
  enabled: boolean;
  updated_at: string;
}

function getToken(): string {
  return localStorage.getItem('vns_token') || '';
}

export async function fetchSchedules(): Promise<ScheduleItem[]> {
  const res = await fetch('/api/schedules', {
    headers: { Authorization: `Bearer ${getToken()}` },
  });
  if (!res.ok) throw new Error('HTTP ' + res.status);
  return res.json();
}

export async function updateSchedule(key: string, expr: string, enabled: boolean): Promise<void> {
  const res = await fetch(`/api/schedules/${key}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${getToken()}`,
    },
    body: JSON.stringify({ cron_expression: expr, enabled }),
  });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error((data as any).message || 'HTTP ' + res.status);
  }
}

export async function triggerEndpoint(endpoint: string): Promise<string> {
  const res = await fetch(`/api/trigger/${endpoint}`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${getToken()}` },
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error((data as any).message || 'HTTP ' + res.status);
  return (data as any).message || 'OK';
}

// ── Market Predictions (server-side paginated) ────────────────────────────────
export interface MarketPredictionsParams {
  page?: number;
  limit?: number;
  search?: string;
  sort_by?: string;
  sort_dir?: 'asc' | 'desc';
  algorithm?: string;
  status?: string;
}

export interface MarketPageResponse {
  market: string;
  data: any[];
  total: number;
  page: number;
  limit: number;
  total_pages: number;
}

export function fetchMarketPredictions(
  marketKey: string,
  params: MarketPredictionsParams = {}
): Promise<MarketPageResponse> {
  const p = new URLSearchParams();
  if (params.page)      p.set('page', String(params.page));
  if (params.limit)     p.set('limit', String(params.limit));
  if (params.search)    p.set('search', params.search);
  if (params.sort_by)   p.set('sort_by', params.sort_by);
  if (params.sort_dir)  p.set('sort_dir', params.sort_dir);
  if (params.algorithm) p.set('algorithm', params.algorithm);
  if (params.status)    p.set('status', params.status);

  // Route to the correct market-specific predictions endpoint
  let endpoint: string;
  switch (marketKey) {
    case 'nasdaq100': endpoint = '/nasdaq/predictions';  break;
    case 'crypto':    endpoint = '/crypto/predictions';  break;
    case 'gold':      endpoint = '/gold/predictions';    break;
    default:          endpoint = '/predictions';         break; // vn30
  }

  return fetchJSON(endpoint + '?' + p.toString())
    .then((res: any) => {
      const data = Array.isArray(res) ? res : (res.data || res.predictions || res.items || []);
      const total = res.total != null ? res.total : (Array.isArray(res) ? res.length : data.length);
      const limit = params.limit || 20;
      return {
        market: marketKey,
        data,
        total,
        page: res.page || params.page || 1,
        limit,
        total_pages: res.total_pages != null ? res.total_pages : Math.ceil(total / limit),
      } as MarketPageResponse;
    })
    .catch(() => ({ market: marketKey, data: [], total: 0, page: 1, limit: params.limit || 20, total_pages: 0 }));
}

// ── Monitoring overview ──────────────────────────────────────────────────────

export interface MonitoringAlgorithmRow {
  algorithm: string;
  today_count: number;
  direction_accuracy: number;
  reconciled: number;
  correct: number;
}

export interface MonitoringPredictions {
  last_predict_at: string | null;
  staleness: string;
  today_total: number;
  expected_algos: number;
  missing_today: string[];
  algorithms: MonitoringAlgorithmRow[];
}

export interface MonitoringCrawl {
  last_crawl_at: string | null;
  staleness: string;
  stale: boolean;
  market_open: boolean;
  daily_today: number;
  intraday_today: number;
}

export interface MonitoringMarket {
  market: 'GOLD' | 'NASDAQ' | 'CRYPTO' | 'SP500';
  crawl: MonitoringCrawl;
  predictions: MonitoringPredictions;
}

export interface MonitoringBotByMarket {
  market: string;
  trades: number;
  wins: number;
  losses: number;
  win_rate: number;
  total_pnl: number;
}

export interface MonitoringBotSummary {
  total_bots: number;
  active_bots: number;
  by_market: MonitoringBotByMarket[];
}

export interface MonitoringBotRow {
  bot_id: string;
  market: string;
  algorithm: string;
  trades: number;
  wins: number;
  losses: number;
  breakeven: number;
  win_rate: number;
  total_pnl: number;
  return_pct: number;
  profit_factor: number;
}

export interface MonitoringOverview {
  generated_at: string;
  markets: MonitoringMarket[];
  bots: {
    summary: MonitoringBotSummary;
    table: MonitoringBotRow[];
  };
}

export async function fetchMonitoringOverview(): Promise<MonitoringOverview> {
  const res = await fetch('/api/monitoring/overview', {
    headers: {
      Accept: 'application/json',
      Authorization: `Bearer ${getToken()}`,
    },
  });
  if (!res.ok) throw new Error('HTTP ' + res.status);
  return res.json();
}

// ── Market Training (server-side paginated) ───────────────────────────────────
export interface MarketTrainingParams {
  page?: number;
  limit?: number;
  sort_by?: string;
  sort_dir?: 'asc' | 'desc';
  algorithm?: string;
}

export function fetchMarketTraining(
  marketKey: string,
  params: MarketTrainingParams = {}
): Promise<MarketPageResponse> {
  const p = new URLSearchParams();
  if (params.page)      p.set('page', String(params.page));
  if (params.limit)     p.set('limit', String(params.limit));
  if (params.sort_by)   p.set('sort_by', params.sort_by);
  if (params.sort_dir)  p.set('sort_dir', params.sort_dir);
  if (params.algorithm) p.set('algorithm', params.algorithm);
  // Training history is a shared endpoint for all markets
  return fetchJSON('/training/history?' + p.toString())
    .then((res: any) => {
      const data = Array.isArray(res) ? res : (res.data || res.items || []);
      const total = res.total != null ? res.total : data.length;
      const limit = params.limit || 20;
      return {
        market: marketKey,
        data,
        total,
        page: res.page || params.page || 1,
        limit,
        total_pages: res.total_pages != null ? res.total_pages : Math.ceil(total / limit),
      } as MarketPageResponse;
    })
    .catch(() => ({ market: marketKey, data: [], total: 0, page: 1, limit: params.limit || 20, total_pages: 0 }));
}
