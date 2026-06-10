import { useState, useEffect, useCallback } from 'react';
import { useData } from '../context/DataContext';
import { Panel, KPI, Icon, Chg, Seg, ConfBar, vnsToast } from '../components/ui';
import { Sparkline, LineChart } from '../components/charts';
import { fetchMarketPage } from '../api';
import { useAuth } from '../context/AuthContext';
import { useLanguage } from '../context/LangContext';
import type { StockItem, MoverItem } from '../types';

function toStockItems(stocks: any[]): StockItem[] {
  return (stocks || []).map((s: any) => ({
    sym: s.symbol || '',
    name: s.company_name || s.symbol || '',
    sector: s.sector || '—',
    exchange: s.exchange || 'HOSE',
    price: parseFloat(s.current_price) || 0,
    change: parseFloat(s.change) || 0,
    chgPct: parseFloat(s.change_percent) || 0,
    volume: (parseFloat(s.volume) || 0) / 1e6,
    value: (parseFloat(s.value) || 0) / 1e12,
    vn30: s.is_vn30 !== false,
    spark: [],
    hist: [],
  }));
}

function getToken(): string {
  return localStorage.getItem('vns_token') || '';
}
async function authPost(path: string): Promise<boolean> {
  const res = await fetch(path, { method: 'POST', headers: { Authorization: `Bearer ${getToken()}` } });
  return res.ok;
}
async function apiFetch(path: string): Promise<any> {
  const res = await fetch(path, { headers: { Accept: 'application/json' } });
  if (!res.ok) throw new Error('HTTP ' + res.status);
  return res.json();
}

// ── Helpers ───────────────────────────────────────────────────────────────────
function num(x: any): number {
  const n = typeof x === 'number' ? x : parseFloat(x);
  return isFinite(n) ? n : 0;
}

function ddmm(s: string): string {
  if (!s) return '';
  try {
    const d = new Date(s);
    if (isNaN(d.getTime())) return s.slice(0, 10);
    return ('0' + d.getDate()).slice(-2) + '/' + ('0' + (d.getMonth() + 1)).slice(-2);
  } catch {
    return s.slice(0, 10);
  }
}

function fmtDT(s: string): string {
  if (!s) return '—';
  try {
    const d = new Date(s);
    if (isNaN(d.getTime())) return s.slice(0, 16).replace('T', ' ');
    const dd = ('0' + d.getDate()).slice(-2);
    const mm = ('0' + (d.getMonth() + 1)).slice(-2);
    const hh = ('0' + d.getHours()).slice(-2);
    const mi = ('0' + d.getMinutes()).slice(-2);
    return `${dd}/${mm} ${hh}:${mi}`;
  } catch {
    return s.slice(0, 16).replace('T', ' ');
  }
}

function algoShort(name: string): { short: string; cls: string } {
  const map: Record<string, { short: string; cls: string }> = {
    lstm_nn: { short: 'LSTM', cls: 'lstm' },
    lstm: { short: 'LSTM', cls: 'lstm' },
    arima_garch: { short: 'ARIMA', cls: 'arima' },
    moving_average: { short: 'MA', cls: 'ma' },
    ema: { short: 'EMA', cls: 'ema' },
    ema_macd: { short: 'EMA', cls: 'ema' },
    ensemble: { short: 'ENS', cls: 'ens' },
    lightgbm: { short: 'LGBM', cls: 'ens' },
    random_forest: { short: 'RF', cls: 'ma' },
    xgboost: { short: 'XGB', cls: 'lstm' },
  };
  return map[(name || '').toLowerCase()] || { short: (name || '?').slice(0, 5).toUpperCase(), cls: 'unknown' };
}

const ALGO_COLORS: Record<string, string> = {
  lstm_nn:        'oklch(0.74 0.13 200)',
  lstm:           'oklch(0.74 0.13 200)',
  arima_garch:    'var(--gold)',
  arima:          'var(--gold)',
  moving_average: 'var(--up)',
  ema:            'oklch(0.72 0.18 150)',
  ema_macd:       'oklch(0.72 0.18 150)',
  lightgbm:       'oklch(0.75 0.16 30)',
  random_forest:  'oklch(0.72 0.14 270)',
  xgboost:        'oklch(0.73 0.17 350)',
  gru:            'oklch(0.72 0.15 240)',
  ensemble:       'oklch(0.72 0.14 300)',
};
const FALLBACK_COLORS = ['var(--accent)', 'oklch(0.72 0.14 300)', 'var(--gold)', 'oklch(0.72 0.18 150)', 'var(--up)'];
function algoColor(key: string, idx: number): string {
  return ALGO_COLORS[key.toLowerCase()] || FALLBACK_COLORS[idx % FALLBACK_COLORS.length];
}
const ALGO_DISPLAY: Record<string, string> = {
  lstm_nn: 'LSTM', lstm: 'LSTM', arima_garch: 'ARIMA', arima: 'ARIMA',
  moving_average: 'MA', ema: 'EMA', ema_macd: 'EMA/MACD',
  lightgbm: 'LightGBM', random_forest: 'RF', xgboost: 'XGBoost',
  gru: 'GRU', ensemble: 'Ensemble',
};
function algoDisplayName(key: string): string {
  return ALGO_DISPLAY[key.toLowerCase()] || key;
}

interface PredictionItem {
  symbol?: string;
  algorithm_name: string;
  current_price: number;
  predicted_price: number;
  confidence: number;
  prediction_date: string;
  target_date?: string;
  actual_price?: number | null;
  accuracy?: number | null;
}

interface PredChartData {
  labels: string[];
  actual: (number | null)[];
  predSeries: { key: string; data: (number | null)[] }[];
}

function buildMultiAlgoData(
  list: any[],
  dateField: string,
  actualField: string,
  predField: string,
  algoField: string,
  fmtDate: (s: string) => string,
  useIntraday = false,
): PredChartData {
  // When intraday mode: use prediction_date (full datetime) as key so multiple
  // predictions within the same calendar day are not collapsed into one point.
  // Fallback to dateField when prediction_date is absent.
  const getKey = (it: any): string => {
    if (useIntraday) {
      const pd = it['prediction_date'] || it[dateField] || '';
      return pd;
    }
    return it[dateField] || '';
  };
  const fmtLabel = useIntraday
    ? (() => {
        let prevDay = '';
        return (s: string) => {
          if (!s) return '';
          try {
            const d = new Date(s);
            if (isNaN(d.getTime())) return s.slice(11, 16) || s;
            const hhmm = ('0' + d.getHours()).slice(-2) + ':' + ('0' + d.getMinutes()).slice(-2);
            const day = ('0' + (d.getMonth() + 1)).slice(-2) + '/' + ('0' + d.getDate()).slice(-2);
            if (day !== prevDay) { prevDay = day; return day + ' ' + hhmm; }
            return hhmm;
          } catch { return s.slice(11, 16) || s; }
        };
      })()
    : fmtDate;
  const sorted = [...list].sort((a, b) => (getKey(a)) < (getKey(b)) ? -1 : 1);
  const uniqueDates: string[] = [];
  const dateIndex = new Map<string, number>();
  for (const it of sorted) {
    const d = getKey(it);
    if (!dateIndex.has(d)) { dateIndex.set(d, uniqueDates.length); uniqueDates.push(d); }
  }
  const n = uniqueDates.length;
  const actual: (number | null)[] = new Array(n).fill(null);
  for (const it of sorted) {
    const idx = dateIndex.get(getKey(it));
    if (idx !== undefined && actual[idx] === null && it[actualField] != null) {
      const v = parseFloat(it[actualField]);
      actual[idx] = isFinite(v) ? v : null;
    }
  }
  const algoMap = new Map<string, (number | null)[]>();
  for (const it of sorted) {
    const key = (it[algoField] || 'unknown').toLowerCase();
    if (!algoMap.has(key)) algoMap.set(key, new Array(n).fill(null));
    const idx = dateIndex.get(getKey(it));
    if (idx !== undefined && it[predField] != null) {
      const v = parseFloat(it[predField]);
      algoMap.get(key)![idx] = isFinite(v) ? v : null;
    }
  }
  return { labels: uniqueDates.map(fmtLabel), actual, predSeries: Array.from(algoMap.entries()).map(([key, data]) => ({ key, data })) };
}

function Legend({ items }: { items: [string, string][] }) {
  return (
    <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap', marginTop: 12, fontSize: 12, color: 'var(--text-3)' }}>
      {items.map(([l, c]) => (
        <span key={l} style={{ display: 'inline-flex', alignItems: 'center', gap: 7 }}>
          <span style={{ width: 12, height: 3, background: c, display: 'inline-block' }}></span>{l}
        </span>
      ))}
    </div>
  );
}

const PAGE_SIZE = 10;

export default function Stocks() {
  const { data: D } = useData();
  const { fmt } = D;
  const { isLoggedIn } = useAuth();
  const { t } = useLanguage();
  const [q, setQ] = useState('');
  const [sector, setSector] = useState('');
  const [sortBy, setSortBy] = useState<'price' | 'change_percent'>('change_percent');
  const [sortDesc, setSortDesc] = useState(true);
  const [page, setPage] = useState(1);
  const [pagedStocks, setPagedStocks] = useState<StockItem[]>([]);
  const [meta, setMeta] = useState({ total: 0, page: 1, pageSize: 10, totalPages: 1 });
  const [loading, setLoading] = useState(false);
  const [sel, setSel] = useState<StockItem | null>(null);
  const [activeSym, setActiveSym] = useState<string | null>(null);
  const [chartDays, setChartDays] = useState('90');
  const [stockChart, setStockChart] = useState<{ dates: string[]; prices: number[]; granularity?: string }>({ dates: [], prices: [], granularity: '1d' });
  const [chartLoading, setChartLoading] = useState(false);

  // Prediction section state
  const [vn30Preds, setVn30Preds] = useState<PredictionItem[]>([]);
  const [confirmedResults, setConfirmedResults] = useState<PredictionItem[]>([]);
  const [predSym, setPredSym] = useState('');
  const [confirmedSym, setConfirmedSym] = useState('');
  const [predPage, setPredPage] = useState(0);
  const [confirmedPage, setConfirmedPage] = useState(0);

  // Prediction vs Actual chart state
  const [predChart, setPredChart] = useState<PredChartData>({ labels: [], actual: [], predSeries: [] });
  const [predChartSym, setPredChartSym] = useState('');
  const [predChartAlgo, setPredChartAlgo] = useState('');
  const [predChartDays, setPredChartDays] = useState('30');

  const sectors = [...new Set(D.stocks.map((s) => s.sector))].filter(Boolean);

  useEffect(() => {
    if (D.stocks.length > 0 && !activeSym) {
      setActiveSym(D.stocks[0].sym);
    }
  }, [D.stocks]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!activeSym) return;
    setChartLoading(true);
    if (chartDays === '1') {
      // Intraday: use /chart endpoint with period=1D&interval=1h
      fetch(`/api/stocks/${encodeURIComponent(activeSym)}/chart?period=1D&interval=1h`)
        .then(r => r.json())
        .then(v => {
          const raw: any[] = v?.data || [];
          setStockChart({
            dates: raw.map((p: any) => p.timestamp || ''),
            prices: raw.map((p: any) => parseFloat(p.close) || 0),
            granularity: '1h',
          });
        })
        .catch(() => setStockChart({ dates: [], prices: [], granularity: '1h' }))
        .finally(() => setChartLoading(false));
    } else {
      fetch(`/api/stocks/${encodeURIComponent(activeSym)}/history?days=${chartDays}`)
        .then(r => r.json())
        .then(v => {
          const priceData: any[] = v?.price_data || [];
          const reversed = [...priceData].reverse();
          setStockChart({
            dates: reversed.map((p) => (p.trading_date || '').slice(0, 10)),
            prices: reversed.map((p) => parseFloat(p.close_price) || 0),
            granularity: '1d',
          });
        })
        .catch(() => setStockChart({ dates: [], prices: [], granularity: '1d' }))
        .finally(() => setChartLoading(false));
    }
  }, [activeSym, chartDays]);

  // Load predictions data
  useEffect(() => {
    Promise.allSettled([
      apiFetch('/api/predictions?limit=200'),
      apiFetch('/api/predictions?status=confirmed&limit=200'),
    ]).then(([predsRes, confirmedRes]) => {
      if (predsRes.status === 'fulfilled' && predsRes.value) {
        const raw = Array.isArray(predsRes.value) ? predsRes.value
          : Array.isArray(predsRes.value?.data) ? predsRes.value.data
          : Array.isArray(predsRes.value?.predictions) ? predsRes.value.predictions : [];
        setVn30Preds(raw);
      }
      if (confirmedRes.status === 'fulfilled' && confirmedRes.value) {
        const raw = Array.isArray(confirmedRes.value) ? confirmedRes.value
          : Array.isArray(confirmedRes.value?.data) ? confirmedRes.value.data
          : Array.isArray(confirmedRes.value?.predictions) ? confirmedRes.value.predictions : [];
        setConfirmedResults(raw);
      }
    });
  }, []);

  // Load prediction vs actual chart
  useEffect(() => {
    const effectiveSym = predChartSym || D.stocks[0]?.sym || 'VCB';
    const algoParam = predChartAlgo ? `&algorithm=${encodeURIComponent(predChartAlgo)}` : '';
    apiFetch(`/api/predictions/compare/${encodeURIComponent(effectiveSym)}?days=${predChartDays}${algoParam}`)
      .then((v: any) => {
        const list = Array.isArray(v) ? v : Array.isArray(v?.data) ? v.data : [];
        if (list.length > 0) {
          setPredChart(buildMultiAlgoData(list, 'date', 'actual_price', 'predicted_price', 'algorithm_name', ddmm, predChartDays === '1'));
        } else {
          setPredChart({ labels: [], actual: [], predSeries: [] });
        }
      })
      .catch(() => setPredChart({ labels: [], actual: [], predSeries: [] }));
  }, [predChartSym, predChartAlgo, predChartDays, D.stocks]); // eslint-disable-line react-hooks/exhaustive-deps

  const doFetch = useCallback((pg: number, sb: string, sd: boolean, searchQ: string, sec: string) => {
    setLoading(true);
    fetchMarketPage({
      page: pg,
      pageSize: 10,
      sortBy: sb as 'price' | 'change_percent',
      sortOrder: sd ? 'desc' : 'asc',
      q: searchQ,
      sector: sec,
    }).then((res: any) => {
      setPagedStocks(toStockItems(res.stocks || []));
      setMeta({
        total: res.stocks_total || 0,
        page: res.stocks_page || pg,
        pageSize: res.stocks_page_size || 10,
        totalPages: res.stocks_total_pages || 1,
      });
    }).catch(() => {}).finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    doFetch(1, 'change_percent', true, '', '');
  }, [doFetch]);

  useEffect(() => {
    const timer = setTimeout(() => {
      setPage(1);
      doFetch(1, sortBy, sortDesc, q, sector);
    }, 300);
    return () => clearTimeout(timer);
  }, [q]); // eslint-disable-line react-hooks/exhaustive-deps

  function handleSort(key: 'price' | 'change_percent') {
    const newDesc = sortBy === key ? !sortDesc : true;
    setSortBy(key);
    setSortDesc(newDesc);
    setPage(1);
    doFetch(1, key, newDesc, q, sector);
  }

  function handleSector(sec: string) {
    setSector(sec);
    setPage(1);
    doFetch(1, sortBy, sortDesc, q, sec);
  }

  function handlePage(p: number) {
    setPage(p);
    doFetch(p, sortBy, sortDesc, q, sector);
  }

  function sortHead(key: 'price' | 'change_percent', label: string) {
    return (
      <th className="r th-sort" onClick={() => handleSort(key)}>
        {label}{' '}
        {sortBy === key && <span className="caret">{sortDesc ? '▼' : '▲'}</span>}
      </th>
    );
  }

  const vn30Val = D.indices.vn30.val;

  // Derived prediction section data
  const predSymbols = Array.from(new Set(vn30Preds.map((p) => p.symbol).filter(Boolean))).sort() as string[];
  const filteredPreds = predSym ? vn30Preds.filter((p) => p.symbol === predSym) : vn30Preds;
  const predPageCount = Math.ceil(filteredPreds.length / PAGE_SIZE);
  const predPagedItems = filteredPreds.slice(predPage * PAGE_SIZE, (predPage + 1) * PAGE_SIZE);

  const confirmedSymbols = Array.from(new Set(confirmedResults.map((r) => r.symbol).filter(Boolean))).sort() as string[];
  const filteredConfirmed = confirmedSym ? confirmedResults.filter((r) => r.symbol === confirmedSym) : confirmedResults;
  const confirmedPageCount = Math.ceil(filteredConfirmed.length / PAGE_SIZE);
  const confirmedPagedItems = filteredConfirmed.slice(confirmedPage * PAGE_SIZE, (confirmedPage + 1) * PAGE_SIZE);

  // Unique algorithms from confirmed for dropdown
  const confirmedAlgos = Array.from(new Set(confirmedResults.map((r) => r.algorithm_name))).sort();

  // VN30 symbols for pred chart selector
  const vn30Symbols = D.stocks.map((s) => s.sym);

  return (
    <div className="content__inner fade">
      {/* 1. KPI Cards */}
      <div className="grid grid--kpis section-gap">
        <KPI label="VN30-Index" value={vn30Val ? fmt.price(vn30Val) : '—'} chgPct={D.indices.vn30.chgPct} chgAbs={D.indices.vn30.chg} spark={D.indices.vn30.series.slice(-22)} />
        <KPI label={t.stocks.upDown} value={D.stocks.length ? D.stocks.length + ' ' + t.stocks.symbolsLoaded : '—'} sub={t.stocks.symbolsLoaded} />
        <KPI label={t.stocks.upDown} value={D.stocks.length ? D.stocks.filter((s) => s.chgPct > 0).length + ' / ' + D.stocks.filter((s) => s.chgPct < 0).length : '— / —'} sub={t.stocks.overTotal} />
        <KPI label={t.dashboard.liquidity} value={D.indices.vnindex.vol ? fmt.compact(D.indices.vnindex.vol * 1e6) : '—'} sub={t.dashboard.stocksMatched} />
      </div>

      {/* Search/filter bar */}
      <Panel className="section-gap">
        <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'center' }}>
          <div className="search" style={{ width: 280 }}>
            <Icon name="search" size={15} />
            <input placeholder={t.stocks.searchPlaceholder} value={q} onChange={(e) => setQ(e.target.value)} />
          </div>
          <select className="sel" value={sector} onChange={(e) => handleSector(e.target.value)}>
            <option value="">{t.stocks.allSectors}</option>
            {sectors.map((s) => <option key={s} value={s}>{s}</option>)}
          </select>
          <select className="sel" defaultValue="">
            <option value="">{t.stocks.allExchanges}</option>
            <option>HOSE</option><option>HNX</option><option>UPCOM</option>
          </select>
          <span style={{ marginLeft: 'auto', fontSize: 12, color: 'var(--text-3)', fontFamily: 'var(--font-mono)' }}>
            {meta.total > 0 ? meta.total + ' mã' : (loading ? '...' : D.stocks.length + ' mã')}
          </span>
        </div>
      </Panel>

      {/* 2. Price Chart */}
      <Panel
        title={t.stocks.priceChart}
        dot={activeSym || '—'}
        className="section-gap"
        tools={
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            <Seg
              options={[
                { value: '1', label: t.dateRange.today },
                { value: '7', label: t.dateRange.d7 },
                { value: '30', label: t.dateRange.d30 },
                { value: '90', label: t.dateRange.d90 },
                { value: '180', label: t.dateRange.d180 },
              ]}
              value={chartDays}
              onChange={setChartDays}
            />
            {isLoggedIn && (
              <>
                <button
                  className="btn btn--sm"
                  onClick={() =>
                    authPost('/api/trigger/crawler').then((ok) =>
                      vnsToast(ok ? t.stocks.collectRequest : t.stocks.collectFail)
                    )
                  }
                >
                  <Icon name="download" size={13} />{t.common.collect}
                </button>
                <button
                  className="btn btn--sm"
                  style={{ background: 'var(--accent)', borderColor: 'var(--accent)', color: '#fff' }}
                  onClick={() =>
                    authPost('/api/trigger/predict').then((ok) =>
                      vnsToast(ok ? t.stocks.predictRequest : t.stocks.predictFail)
                    )
                  }
                >
                  <Icon name="play" size={13} />{t.common.predict}
                </button>
              </>
            )}
          </div>
        }
      >
        {/* Symbol selector chips */}
        {D.stocks.length > 0 && (
          <div className="chips" style={{ marginBottom: 16 }}>
            {D.stocks.map((s) => (
              <button
                key={s.sym}
                className={`chip ${activeSym === s.sym ? 'active' : ''}`}
                onClick={() => setActiveSym(s.sym)}
              >
                {s.sym}
              </button>
            ))}
          </div>
        )}

        {activeSym && (() => {
          const cur = D.stocks.find((s) => s.sym === activeSym);
          return cur ? (
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 14, marginBottom: 10 }}>
              <span className="num" style={{ fontSize: 30, fontWeight: 600, letterSpacing: '-1px' }}>
                {fmt.price(cur.price)}
              </span>
              <span style={{ fontSize: 12, color: 'var(--text-3)' }}>{t.stocks.thousandVnd}</span>
              <Chg pct={cur.chgPct} abs={cur.change} />
              <span style={{ marginLeft: 'auto', fontSize: 11, color: 'var(--text-3)', fontFamily: 'var(--font-mono)' }}>
                VN30 · {cur.sym}
              </span>
            </div>
          ) : null;
        })()}

        {chartLoading ? (
          <div className="empty" style={{ height: 400, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
            <div className="empty__icon"><Icon name="refresh" size={18} /></div>
            <p>{t.stocks.loadingChart}</p>
          </div>
        ) : stockChart.prices.length > 0 ? (
          <LineChart
            series={[{ name: activeSym || 'Giá', data: stockChart.prices, color: 'var(--accent)' }]}
            labels={stockChart.dates.map((s) => {
              if (stockChart.granularity === '1h') {
                return s.length >= 16 ? s.slice(11, 16) : s;
              }
              return ddmm(s);
            })}
            height={400}
            area
            yFmt={(v) => v.toFixed(1)}
            valueFmt={(v) => fmt.price(v)}
            padL={52}
          />
        ) : (
          <div className="empty" style={{ height: 400, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
            <div className="empty__icon"><Icon name="layers" size={18} /></div>
            <p>{t.stocks.noChartData}</p>
          </div>
        )}
      </Panel>

      {/* Price table */}
      <Panel title={t.stocks.priceTable} sub={t.stocks.updatedAt} flush className="section-gap"
        tools={<button className="btn btn--sm btn--ghost"><Icon name="refresh" size={13} />{t.common.refresh}</button>}>
        {loading && pagedStocks.length === 0
          ? <div className="empty"><p>{t.stocks.loadingData}</p></div>
          : pagedStocks.length === 0
            ? <div className="empty">
                <div className="empty__icon"><Icon name="layers" size={18} /></div>
                <p>{t.stocks.noStockData}</p>
              </div>
            : <div style={{ overflowX: 'auto' }}>
                <table className="tbl">
                  <thead><tr>
                    <th className="l">{t.stocks.colSymbol}</th>
                    <th>{t.stocks.colSector}</th>
                    {sortHead('price', t.stocks.colPrice)}
                    <th className="r">{t.stocks.colDelta}</th>
                    {sortHead('change_percent', t.stocks.colChangePct)}
                    <th className="r">{t.stocks.colVolume}</th>
                    <th className="r">{t.stocks.colValue}</th>
                    <th className="c">{t.stocks.col7Sessions}</th>
                    <th className="c"></th>
                  </tr></thead>
                  <tbody>
                    {pagedStocks.map((s) => (
                      <tr key={s.sym} className="clickable" onClick={() => setSel(s)}>
                        <td><div className="sym">{s.sym}</div><div className="co">{s.name}</div></td>
                        <td style={{ color: 'var(--text-3)', fontSize: 12 }}>{s.sector}</td>
                        <td className="r num">{fmt.price(s.price)}</td>
                        <td className="r num" style={{ color: s.change > 0 ? 'var(--up)' : s.change < 0 ? 'var(--down)' : 'var(--text-3)' }}>{fmt.sign(s.change)}</td>
                        <td className="r"><Chg pct={s.chgPct} badge /></td>
                        <td className="r num" style={{ color: 'var(--text-2)' }}>{s.volume.toFixed(1)}</td>
                        <td className="r num" style={{ color: 'var(--text-2)' }}>{s.value.toFixed(2)}</td>
                        <td className="c"><div style={{ display: 'flex', justifyContent: 'center' }}><Sparkline data={s.spark} w={86} h={26} fill={false} /></div></td>
                        <td className="c"><Icon name="caretDown" size={14} style={{ transform: 'rotate(-90deg)', color: 'var(--text-3)' }} /></td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
        }
        {meta.totalPages > 1 && (
          <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', gap: 6, padding: '12px 16px', borderTop: '1px solid var(--border)' }}>
            <button
              className="btn btn--sm btn--ghost"
              disabled={page <= 1 || loading}
              onClick={() => handlePage(page - 1)}
              style={{ minWidth: 28, padding: '2px 8px' }}
            >‹</button>
            <span style={{ fontSize: 12, color: 'var(--text-2)' }}>{page} / {meta.totalPages}</span>
            <button
              className="btn btn--sm btn--ghost"
              disabled={page >= meta.totalPages || loading}
              onClick={() => handlePage(page + 1)}
              style={{ minWidth: 28, padding: '2px 8px' }}
            >›</button>
            <span style={{ fontSize: 12, color: 'var(--text-3)', marginLeft: 8 }}>
              {meta.total} {t.stocks.totalSymbols}
            </span>
          </div>
        )}
      </Panel>

      {/* 3. Predictions Section */}
      <div className="grid grid--halves section-gap">
        {/* Left: Tomorrow's predictions */}
        <Panel title="Dự đoán VN30 phiên mai" flush tools={
          predSymbols.length > 0 && (
            <select value={predSym} onChange={(e) => { setPredSym(e.target.value); setPredPage(0); }}
              style={{ fontSize: 12, padding: '3px 8px', borderRadius: 6, border: '1px solid var(--border)', background: 'var(--surface-2)', color: 'var(--text-1)', cursor: 'pointer' }}>
              <option value="">{t.common.allSymbols}</option>
              {predSymbols.map((s) => <option key={s} value={s}>{s}</option>)}
            </select>
          )
        }>
          {filteredPreds.length === 0 ? (
            <div className="empty">
              <div className="empty__icon"><Icon name="layers" size={18} /></div>
              <p>{vn30Preds.length === 0 ? t.common.noPredictions : 'Không có dữ liệu cho mã này.'}</p>
            </div>
          ) : (
            <>
              <div style={{ overflowX: 'auto' }}>
                <table className="tbl">
                  <thead>
                    <tr>
                      <th>{t.common.symbol}</th>
                      <th className="c">{t.common.status}</th>
                      <th className="r">{t.common.current}</th>
                      <th className="r">{t.common.predicted}</th>
                      <th className="r">±%</th>
                      <th className="r">{t.common.accuracy}</th>
                      <th className="r">{t.common.confidence}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {predPagedItems.map((p, i) => {
                      const cur = num(p.current_price);
                      const pred = num(p.predicted_price);
                      const deltaPct = cur ? +((pred - cur) / cur * 100).toFixed(2) : 0;
                      const { short, cls } = algoShort(p.algorithm_name);
                      let conf = num(p.confidence);
                      if (conf > 0 && conf <= 1) conf = Math.round(conf * 100);
                      return (
                        <tr key={i}>
                          <td className="sym" style={{ fontSize: 12.5 }}>{p.symbol}</td>
                          <td className="c"><span className={`algo algo--${cls}`}>{short}</span></td>
                          <td className="r num" style={{ color: 'var(--text-2)', fontSize: 12 }}>{fmt.price(cur)}</td>
                          <td className="r num" style={{ fontWeight: 600, fontSize: 12 }}>{fmt.price(pred)}</td>
                          <td className="r"><Chg pct={deltaPct} /></td>
                          <td className="r">
                            {(() => {
                              let acc = p.accuracy != null ? num(p.accuracy) : null;
                              if (acc != null && acc > 0 && acc <= 1) acc = Math.round(acc * 100);
                              if (acc != null) {
                                const color = acc >= 60 ? 'var(--up)' : acc >= 50 ? 'oklch(0.78 0.18 80)' : 'var(--dn)';
                                return <span style={{ color, fontWeight: 600, fontSize: 12 }}>{acc}%</span>;
                              }
                              return <span style={{ color: 'var(--text-3)' }}>—</span>;
                            })()}
                          </td>
                          <td className="r"><ConfBar v={conf} /></td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
              {predPageCount > 1 && (
                <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', gap: 8, padding: '10px 16px', borderTop: '1px solid var(--border)', fontSize: 12, color: 'var(--text-2)' }}>
                  <button className="btn btn--sm" disabled={predPage === 0} onClick={() => setPredPage((p) => p - 1)} style={{ minWidth: 28, padding: '2px 8px' }}>‹</button>
                  <span>{predPage + 1} / {predPageCount}</span>
                  <button className="btn btn--sm" disabled={predPage >= predPageCount - 1} onClick={() => setPredPage((p) => p + 1)} style={{ minWidth: 28, padding: '2px 8px' }}>›</button>
                </div>
              )}
            </>
          )}
        </Panel>

        {/* Right: Latest confirmed results */}
        <Panel title={t.common.latestPredResults} flush tools={
          confirmedSymbols.length > 0 && (
            <select value={confirmedSym} onChange={(e) => { setConfirmedSym(e.target.value); setConfirmedPage(0); }}
              style={{ fontSize: 12, padding: '3px 8px', borderRadius: 6, border: '1px solid var(--border)', background: 'var(--surface-2)', color: 'var(--text-1)', cursor: 'pointer' }}>
              <option value="">{t.common.allSymbols}</option>
              {confirmedSymbols.map((s) => <option key={s} value={s}>{s}</option>)}
            </select>
          )
        }>
          {filteredConfirmed.length === 0 ? (
            <div className="empty">
              <div className="empty__icon"><Icon name="pulse" size={18} /></div>
              <p>{confirmedResults.length === 0 ? t.common.noConfirmedResults : 'Không có dữ liệu cho mã này.'}</p>
            </div>
          ) : (
            <>
              <div style={{ overflowX: 'auto' }}>
                <table className="tbl">
                  <thead>
                    <tr>
                      <th>{t.common.symbol}</th>
                      <th className="c">{t.common.status}</th>
                      <th className="r">{t.common.predicted}</th>
                      <th className="r">{t.common.actual}</th>
                      <th className="r">{t.common.deviation}</th>
                      <th className="r">{t.common.accuracy}</th>
                      <th className="c">{t.common.date}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {confirmedPagedItems.map((r, i) => {
                      const predicted = num(r.predicted_price);
                      const actual = num(r.actual_price ?? 0);
                      const deviationPct = actual ? +((predicted - actual) / actual * 100).toFixed(2) : 0;
                      const absDeviation = Math.abs(deviationPct);
                      const deviationColor = absDeviation < 3
                        ? 'var(--up)'
                        : absDeviation < 5
                          ? 'oklch(0.78 0.18 80)'
                          : 'var(--dn)';
                      let acc = r.accuracy != null ? num(r.accuracy) : null;
                      if (acc != null && acc > 0 && acc <= 1) acc = Math.round(acc * 100);
                      const accColor = acc == null ? 'var(--text-3)'
                        : acc >= 60 ? 'var(--up)'
                        : acc >= 50 ? 'oklch(0.78 0.18 80)'
                        : 'var(--dn)';
                      const { short, cls } = algoShort(r.algorithm_name);
                      return (
                        <tr key={i}>
                          <td className="sym" style={{ fontSize: 12.5 }}>{r.symbol}</td>
                          <td className="c"><span className={`algo algo--${cls}`}>{short}</span></td>
                          <td className="r num" style={{ fontWeight: 600, fontSize: 12 }}>{fmt.price(predicted)}</td>
                          <td className="r num" style={{ color: 'var(--text-2)', fontSize: 12 }}>
                            {r.actual_price != null ? fmt.price(actual) : <span style={{ color: 'var(--text-3)' }}>—</span>}
                          </td>
                          <td className="r num" style={{ color: deviationColor, fontSize: 12, fontWeight: 500 }}>
                            {r.actual_price != null
                              ? (deviationPct >= 0 ? '+' : '') + deviationPct.toFixed(2) + '%'
                              : <span style={{ color: 'var(--text-3)' }}>—</span>}
                          </td>
                          <td className="r">
                            {acc != null
                              ? <span style={{ color: accColor, fontWeight: 600, fontSize: 12 }}>{acc}%</span>
                              : <span style={{ color: 'var(--text-3)' }}>—</span>}
                          </td>
                          <td className="c num" style={{ color: 'var(--text-3)', fontSize: 12 }}>
                            {fmtDT(r.prediction_date)}
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
              {confirmedPageCount > 1 && (
                <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', gap: 8, padding: '10px 16px', borderTop: '1px solid var(--border)', fontSize: 12, color: 'var(--text-2)' }}>
                  <button className="btn btn--sm" disabled={confirmedPage === 0} onClick={() => setConfirmedPage((p) => p - 1)} style={{ minWidth: 28, padding: '2px 8px' }}>‹</button>
                  <span>{confirmedPage + 1} / {confirmedPageCount}</span>
                  <button className="btn btn--sm" disabled={confirmedPage >= confirmedPageCount - 1} onClick={() => setConfirmedPage((p) => p + 1)} style={{ minWidth: 28, padding: '2px 8px' }}>›</button>
                </div>
              )}
            </>
          )}
        </Panel>
      </div>

      {/* 4. Prediction vs Actual Chart */}
      <Panel
        title={t.common.predVsActual}
        sub={(predChartSym || (D.stocks[0]?.sym || 'VN30')) + ' · ' + (predChartAlgo ? algoShort(predChartAlgo).short : t.common.allAlgos)}
        className="section-gap"
        tools={
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center' }}>
            <Seg
              options={[
                { value: '1', label: t.dateRange.today },
                { value: '7', label: t.dateRange.d7 },
                { value: '30', label: t.dateRange.d30 },
                { value: '90', label: t.dateRange.d90 },
                { value: '180', label: t.dateRange.d180 },
              ]}
              value={predChartDays}
              onChange={setPredChartDays}
            />
            {/* Symbol selector */}
            {vn30Symbols.length > 0 && (
              <select value={predChartSym} onChange={(e) => setPredChartSym(e.target.value)}
                style={{ fontSize: 12, padding: '3px 8px', borderRadius: 6, border: '1px solid var(--border)', background: 'var(--surface-2)', color: 'var(--text-1)', cursor: 'pointer' }}>
                {vn30Symbols.map((s) => <option key={s} value={s}>{s}</option>)}
              </select>
            )}
            {/* Algorithm filter */}
            {confirmedAlgos.length > 0 && (
              <select value={predChartAlgo} onChange={(e) => setPredChartAlgo(e.target.value)}
                style={{ fontSize: 12, padding: '3px 8px', borderRadius: 6, border: '1px solid var(--border)', background: 'var(--surface-2)', color: 'var(--text-1)', cursor: 'pointer' }}>
                <option value="">{t.common.allAlgos}</option>
                {confirmedAlgos.map((algo) => (
                  <option key={algo} value={algo}>{algoShort(algo).short} ({algo})</option>
                ))}
              </select>
            )}
          </div>
        }
      >
        {predChart.labels.length > 0 && (predChart.predSeries.length > 0 || predChart.actual.some((v) => v != null)) ? (
          <>
            <LineChart
              series={[
                { name: t.common.actual, data: predChart.actual, color: 'var(--text-2)', w: 1.8 },
                ...predChart.predSeries.map((ps, i) => ({
                  name: algoDisplayName(ps.key),
                  data: ps.data,
                  color: algoColor(ps.key, i),
                  dash: '5 4',
                  w: 1.6,
                })),
              ]}
              labels={predChart.labels}
              height={680}
              yFmt={(v) => v.toFixed(1)}
              valueFmt={(v) => fmt.price(v)}
              padL={52}
              highlightable
            />
            <Legend items={[
              [t.common.actual, 'var(--text-2)'],
              ...predChart.predSeries.map((ps, i) => [algoDisplayName(ps.key), algoColor(ps.key, i)] as [string, string]),
            ]} />
          </>
        ) : (
          <div className="empty" style={{ height: 680, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
            <div className="empty__icon"><Icon name="layers" size={18} /></div>
            <p>{t.common.noCompareData}</p>
          </div>
        )}
      </Panel>

      {/* Gainers / Losers — VN30 specific */}
      <div className="grid grid--halves section-gap">
        <MoverPanel title={t.stocks.biggestGainers} icon="arrowUp" items={D.gainers} direction="up" fmt={fmt} />
        <MoverPanel title={t.stocks.biggestLosers} icon="arrowDown" items={D.losers} direction="down" fmt={fmt} />
      </div>

      {D.active.length > 0 && (
        <Panel title={t.stocks.mostActive} sub={t.stocks.byVolume} flush style={{ paddingBottom: 8 }}>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)' }}>
            {D.active.map((s, i) => (
              <div key={s.sym} className="lrow clickable" style={{ borderRight: i % 3 !== 2 ? '1px solid var(--border)' : 'none', cursor: 'pointer' }} onClick={() => setSel(s as any)}>
                <span className="badge badge--muted" style={{ minWidth: 22, justifyContent: 'center' }}>{i + 1}</span>
                <div className="lrow__main">
                  <div className="lrow__sym">{s.sym}</div>
                  <div className="lrow__sub">{s.volume.toFixed(1)}M {t.stocks.shares}</div>
                </div>
                <div className="lrow__rt">
                  <div className="num" style={{ fontWeight: 600 }}>{fmt.price(s.price)}</div>
                  <div style={{ fontSize: 11 }}><Chg pct={s.chgPct} /></div>
                </div>
              </div>
            ))}
          </div>
        </Panel>
      )}

      {sel && <StockDrawer s={sel} onClose={() => setSel(null)} predictions={D.predictions} fmt={fmt} />}
    </div>
  );
}

function MoverPanel({ title, icon, items, direction, fmt }: { title: string; icon: string; items: MoverItem[]; direction: 'up' | 'down'; fmt: any }) {
  const { t } = useLanguage();
  return (
    <Panel title={title} flush
      tools={<Icon name={icon} size={15} style={{ color: direction === 'up' ? 'var(--up)' : 'var(--down)' }} />}>
      {items.length === 0
        ? <div className="empty">
            <div className="empty__icon"><Icon name="layers" size={18} /></div>
            <p>{t.stocks.noData}</p>
          </div>
        : items.map((s) => (
          <div key={s.sym} className="lrow">
            <div className="lrow__main">
              <div className="lrow__sym">{s.sym}</div>
              <div className="lrow__sub">{s.name}</div>
            </div>
            <Sparkline data={s.spark} w={70} h={24} fill={false} color={direction === 'up' ? 'var(--up)' : 'var(--down)'} />
            <div className="lrow__rt" style={{ minWidth: 88 }}>
              <div className="num" style={{ fontWeight: 600 }}>{fmt.price(s.price)}</div>
              <div style={{ fontSize: 11.5 }}><Chg pct={s.chgPct} /></div>
            </div>
          </div>
        ))
      }
    </Panel>
  );
}

function StockDrawer({ s, onClose, predictions, fmt }: { s: StockItem; onClose: () => void; predictions: any[]; fmt: any }) {
  const { t } = useLanguage();
  const [range, setRange] = useState('30');
  const n = range === '7' ? 7 : 30;
  const hist = (s.hist || []).slice(-n);
  const pred = predictions.find((p) => p.sym === s.sym);
  return (
    <div style={{ position: 'fixed', inset: 0, zIndex: 40, display: 'flex', justifyContent: 'flex-end', background: 'oklch(0 0 0 / 0.5)' }} onClick={onClose}>
      <div className="fade" style={{ width: 560, maxWidth: '92vw', background: 'var(--bg-2)', borderLeft: '1px solid var(--border-strong)', height: '100%', overflowY: 'auto' }} onClick={(e) => e.stopPropagation()}>
        <div className="panel__head" style={{ position: 'sticky', top: 0, background: 'var(--bg-2)', zIndex: 2 }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              <span style={{ fontSize: 19, fontWeight: 700 }}>{s.sym}</span>
              <span className="badge badge--neutral">{s.exchange}</span>
              {s.vn30 && <span className="badge badge--muted">VN30</span>}
            </div>
            <div style={{ fontSize: 12, color: 'var(--text-3)', marginTop: 2 }}>{s.name} · {s.sector}</div>
          </div>
          <button className="btn btn--icon btn--ghost" style={{ marginLeft: 'auto' }} onClick={onClose}>✕</button>
        </div>
        <div style={{ padding: 18 }}>
          <div style={{ display: 'flex', alignItems: 'baseline', gap: 12, marginBottom: 14 }}>
            <span className="num" style={{ fontSize: 34, fontWeight: 600, letterSpacing: '-1.5px' }}>{fmt.price(s.price)}</span>
            <div><Chg pct={s.chgPct} abs={s.change} /></div>
          </div>
          <Seg options={[{ value: '7', label: t.dateRange.d7 }, { value: '30', label: t.dateRange.d30 }]} value={range} onChange={setRange} />
          <div style={{ marginTop: 12 }}>
            {hist.length > 0
              ? <LineChart series={[{ name: s.sym, data: hist, color: s.chgPct >= 0 ? 'var(--up)' : 'var(--down)' }]} labels={hist.map((_, i) => `${i + 1}`)} height={520} area valueFmt={(v) => fmt.price(v)} />
              : <div className="empty" style={{ height: 520, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
                  <div className="empty__icon"><Icon name="layers" size={18} /></div>
                  <p>{t.stocks.noHistPrice}</p>
                </div>
            }
          </div>
          <div className="grid" style={{ gridTemplateColumns: '1fr 1fr', gap: 1, background: 'var(--border)', border: '1px solid var(--border)', marginTop: 16 }}>
            {[['Khối lượng', s.volume.toFixed(1) + 'M'], ['Giá trị', s.value.toFixed(2) + ' ngàn tỷ']].map(([l, v]) => (
              <div key={l} style={{ background: 'var(--surface)', padding: '12px 14px' }}>
                <div style={{ fontSize: 11, color: 'var(--text-3)', textTransform: 'uppercase', letterSpacing: 0.4 }}>{l}</div>
                <div className="num" style={{ fontSize: 16, fontWeight: 600, marginTop: 4 }}>{v}</div>
              </div>
            ))}
          </div>
          {pred && (
            <div style={{ marginTop: 16 }}>
              <div className="sec-head" style={{ margin: '0 0 10px' }}><h2>{t.stocks.nextSessionPred}</h2><div className="line"></div></div>
              <div className="panel" style={{ padding: 16 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
                  <span className={`algo algo--${pred.algoCls}`}>{pred.algoShort}</span>
                  <div>
                    <div style={{ fontSize: 11, color: 'var(--text-3)' }}>{t.stocks.predPrice} {pred.target}</div>
                    <div className="num" style={{ fontSize: 20, fontWeight: 600 }}>{fmt.price(pred.pred)}</div>
                  </div>
                  <div style={{ marginLeft: 'auto', textAlign: 'right' }}>
                    <Chg pct={pred.deltaPct} />
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
