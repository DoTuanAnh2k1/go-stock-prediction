import type {
  AppData, FmtUtils, StockItem, MoverItem, PredictionItem,
  ConfirmedItem, AlgoItem, GoldSource, GoldPred, GoldPredActual,
  GoldDetailItem, IndexData
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

const ALGO: Record<string, { short: string; cls: string; name: string }> = {
  lstm_nn:       { short: 'LSTM',  cls: 'lstm',  name: 'LSTM Neural Network' },
  lstm:          { short: 'LSTM',  cls: 'lstm',  name: 'LSTM Neural Network' },
  arima_garch:   { short: 'ARIMA', cls: 'arima', name: 'ARIMA-GARCH' },
  arima:         { short: 'ARIMA', cls: 'arima', name: 'ARIMA-GARCH' },
  moving_average:{ short: 'MA',    cls: 'ma',    name: 'Moving Average' },
  ma:            { short: 'MA',    cls: 'ma',    name: 'Moving Average' },
  ensemble:      { short: 'ENS',   cls: 'ens',   name: 'Ensemble' },
  ens:           { short: 'ENS',   cls: 'ens',   name: 'Ensemble' },
};
function algoMeta(n: string) {
  return ALGO[(n || '').toLowerCase()] || {
    short: (n || '?').toUpperCase().slice(0, 5),
    cls: 'lstm',
    name: n || '—',
  };
}

function fetchJSON(url: string, opts?: RequestInit): Promise<any> {
  return fetch(BASE + url, Object.assign({ headers: { Accept: 'application/json' } }, opts || {}))
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
      vn30:    { val: 0, chg: 0, chgPct: 0, vol: 0, series: [] },
    },
    accTrend: { labels: [], lstm: [], arima: [], ma: [] },
    dailyCounts: { labels: [], values: [] },
    trainLogs: [], trainJobs: [],
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
export function goldChart(source: string, product: string, days?: number): Promise<{ labels: string[]; buy: number[]; sell: number[] }> {
  return fetchJSON('/gold/chart?source=' + source + '&product_type=' + product + '&days=' + (days || 180))
    .then((j: any) => ({
      labels: arr<string>(j.labels),
      buy:    arr<number>(j.buy_prices).map(num),
      sell:   arr<number>(j.sell_prices).map(num),
    }))
    .catch(() => ({ labels: [], buy: [], sell: [] }));
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
    vn30: s.is_vn30 !== false,
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

function buildAlgos(accList: any): AlgoItem[] | null {
  const list = arr<any>(accList.data || accList);
  if (!list.length) return null;
  return list.map((a: any) => {
    const m = algoMeta(a.algorithm_name);
    return {
      id: a.algorithm_name, short: m.short, name: m.name, cls: m.cls,
      desc: '', acc: +num(a.accuracy_rate).toFixed(1), accDelta: 0,
      mae: +num(a.avg_error).toFixed(2), trainedAt: '—', epochs: null, status: 'trained',
    };
  });
}

function buildTrend(t: any): AppData['accTrend'] | null {
  if (Array.isArray(t)) {
    const by: Record<string, Record<string, number>> = {};
    t.forEach((r: any) => {
      const w = r.week || r.date || '';
      (by[w] = by[w] || {})[r.algorithm_name] = num(r.accuracy_rate);
    });
    const ws = Object.keys(by).sort();
    if (!ws.length) return null;
    return {
      labels: ws.map(ddmm),
      lstm:  ws.map((w) => by[w].lstm_nn || null),
      arima: ws.map((w) => by[w].arima_garch || null),
      ma:    ws.map((w) => by[w].moving_average || null),
    };
  }
  if (t && t.weeks) {
    return {
      labels: t.weeks.map(ddmm),
      lstm:   arr<number>(t.lstm_nn).map(num),
      arima:  arr<number>(t.arima_garch).map(num),
      ma:     arr<number>(t.moving_average).map(num),
    };
  }
  return null;
}

const GOLD_SRC: Record<string, string> = { BTMC: 'BTMC', BTMH: 'BTMH', SJC: 'SJC', DOJI: 'DOJI', PNJ: 'PNJ', PHUQUY: 'Phú Quý', XAU: 'XAU/USD' };
const GOLD_PROD: Record<string, string> = { sjc: 'Miếng', nhan_tron: 'Nhẫn Tròn', spot: '' };

function buildGold(latest: any): GoldSource[] | null {
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
    };
  });
}

function buildGoldPredActual(res: any): GoldPredActual | null {
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
    market:       P('/market/overview'),
    stats:        P('/dashboard/stats'),
    preds:        P('/predictions?limit=12'),
    confirmed:    P('/predictions?limit=80&status=confirmed'),
    acc:          P('/predictions/accuracy?days=30'),
    trend:        P('/predictions/accuracy-trend?days=90'),
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

    if (R.market) {
      const st = buildStocks(R.market);
      if (st) {
        out.stocks = st; sources.market = true;
        out.gainers = arr<any>(R.market.top_gainers).map(mover);
        out.losers  = arr<any>(R.market.top_losers).map(mover);
        out.active  = arr<any>(R.market.most_active).map(mover);
        if (!out.gainers.length) out.gainers = st.slice().sort((a, b) => b.chgPct - a.chgPct).slice(0, 6);
        if (!out.losers.length)  out.losers  = st.slice().sort((a, b) => a.chgPct - b.chgPct).slice(0, 6);
        if (!out.active.length)  out.active  = st.slice().sort((a, b) => b.volume - a.volume).slice(0, 6);
      }
      const v = num(R.market.vn30_index);
      if (v) {
        out.indices = {
          vnindex: { val: num(R.market.vnindex), chg: num(R.market.index_change), chgPct: num(R.market.index_percent), vol: num(R.market.total_volume) / 1e6, series: [] },
          vn30:    { val: v, chg: num(R.market.index_change), chgPct: num(R.market.index_percent), vol: 0, series: [] },
        };
      }
    }

    const pl = buildPredsLatest(R.preds); if (pl) { out.predictions = pl; sources.preds = true; }
    const cf = buildConfirmed(R.confirmed) || buildConfirmed(R.preds); if (cf) { out.confirmed = cf; }
    const al = buildAlgos(R.acc); if (al && al.length) { out.algos = al; sources.acc = true; }
    const tr = buildTrend(R.trend); if (tr) { out.accTrend = tr; }

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
  if (data.__sources && data.__sources.market) {
    const syms = data.stocks.slice(0, 30);
    Promise.allSettled(
      syms.map((s) => stockHistory(s.sym, 30).then((h) => { s.hist = h; s.spark = h.slice(-20); }))
    ).then(() => { onUpdate && onUpdate(); });
  }
  if (data.__sources && data.__sources.gold) {
    Promise.allSettled(
      data.goldSources.map((g) =>
        goldChart(g.source, g.product, 180).then((c) => {
          if (c.sell.length) {
            g.hist = c.sell; g.spark = c.sell.slice(-24);
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

export const crawlGold  = () => trigger('/trigger/gold-crawler');
export const predictGold = () => trigger('/trigger/gold-predict');
export const train      = () => trigger('/trigger/train');
