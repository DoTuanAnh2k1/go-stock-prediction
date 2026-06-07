import { useState, useEffect } from 'react';
import { NavLink } from 'react-router-dom';
import { Panel, KPI, Icon, Chg, Seg, ConfBar, vnsToast } from '../components/ui';
import { LineChart } from '../components/charts';
import { useAuth } from '../context/AuthContext';
import { useLanguage } from '../context/LangContext';

// ── Market tabs ───────────────────────────────────────────────────────────────
function MarketTabs({ marketKey }: { marketKey: string }) {
  const { t } = useLanguage();
  const base = '/markets/' + marketKey;
  return (
    <div className="market-tabs">
      <NavLink to={base} end className={({ isActive }) => 'market-tab' + (isActive ? ' active' : '')}>
        <Icon name="candles" size={14} />
        {t.marketTabs.overview}
      </NavLink>
      <NavLink to={base + '/predictions'} className={({ isActive }) => 'market-tab' + (isActive ? ' active' : '')}>
        <Icon name="pulse" size={14} />
        {t.marketTabs.predictions}
      </NavLink>
      <NavLink to={base + '/detail'} className={({ isActive }) => 'market-tab' + (isActive ? ' active' : '')}>
        <Icon name="layers" size={14} />
        {t.marketTabs.detail}
      </NavLink>
      <NavLink to={base + '/training'} className={({ isActive }) => 'market-tab' + (isActive ? ' active' : '')}>
        <Icon name="cpu" size={14} />
        {t.marketTabs.training}
      </NavLink>
    </div>
  );
}

// ── Types ────────────────────────────────────────────────────────────────────
interface CryptoLatestItem {
  coin_id: string;
  symbol: string;
  name?: string;
  close_price: number;
  market_cap?: number;
  volume_24h?: number;
  trading_date: string;
  change_percent?: number;
}

interface ChartData {
  dates: string[];
  prices: number[];
  granularity?: string;
}

interface PredictionItem {
  coin_id?: string;
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
function buildMultiAlgoData(
  list: any[],
  dateField: string,
  actualField: string,
  predField: string,
  algoField: string,
  fmtDate: (s: string) => string,
  useIntraday = false,
): { labels: string[]; actual: (number | null)[]; predSeries: { key: string; data: (number | null)[] }[] } {
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
    ? (s: string) => {
        if (!s) return '';
        try {
          const d = new Date(s);
          if (isNaN(d.getTime())) return s.slice(11, 16) || s;
          return ('0' + d.getHours()).slice(-2) + ':' + ('0' + d.getMinutes()).slice(-2);
        } catch { return s.slice(11, 16) || s; }
      }
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

// ── Helpers ──────────────────────────────────────────────────────────────────
function num(x: any): number {
  const n = typeof x === 'number' ? x : parseFloat(x);
  return isFinite(n) ? n : 0;
}

function getToken(): string {
  return localStorage.getItem('vns_token') || '';
}

async function apiFetch(path: string): Promise<any> {
  const res = await fetch(path, { headers: { Accept: 'application/json' } });
  if (!res.ok) throw new Error('HTTP ' + res.status);
  return res.json();
}

async function authPost(path: string): Promise<boolean> {
  const res = await fetch(path, {
    method: 'POST',
    headers: { Authorization: `Bearer ${getToken()}` },
  });
  return res.ok;
}

function fmtCrypto(v: number): string {
  if (v >= 1000) {
    return '$' + v.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  }
  return '$' + v.toFixed(2);
}

function fmtCryptoShort(v: number): string {
  if (v >= 1000) return '$' + (v / 1000).toFixed(1) + 'K';
  return '$' + v.toFixed(2);
}

function fmtMarketCap(v: number): string {
  if (v >= 1e12) return '$' + (v / 1e12).toFixed(2) + 'T';
  if (v >= 1e9) return '$' + (v / 1e9).toFixed(2) + 'B';
  if (v >= 1e6) return '$' + (v / 1e6).toFixed(2) + 'M';
  return '$' + v.toLocaleString('en-US');
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
    ensemble: { short: 'ENS', cls: 'ens' },
  };
  return map[(name || '').toLowerCase()] || { short: (name || '?').slice(0, 5).toUpperCase(), cls: 'unknown' };
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

// ── Coin definitions ──────────────────────────────────────────────────────────
const COINS = [
  { id: 'bitcoin',  label: 'Bitcoin',  symbol: 'BTC', color: '#F7931A' },
  { id: 'ethereum', label: 'Ethereum', symbol: 'ETH', color: '#627EEA' },
  { id: 'solana',   label: 'Solana',   symbol: 'SOL', color: '#9945FF' },
];

export default function Crypto() {
  const { isLoggedIn } = useAuth();
  const { t } = useLanguage();

  const [latest, setLatest] = useState<CryptoLatestItem[]>([]);
  const [activeCoin, setActiveCoin] = useState<string>('bitcoin');
  const [days, setDays] = useState('90');
  const [chart, setChart] = useState<ChartData>({ dates: [], prices: [] });
  const [preds, setPreds] = useState<PredictionItem[]>([]);
  const [confirmedResults, setConfirmedResults] = useState<PredictionItem[]>([]);
  const [predChart, setPredChart] = useState<PredChartData>({ labels: [], actual: [], predSeries: [] });
  const [predChartCoin, setPredChartCoin] = useState<string>('bitcoin');
  const [predChartAlgo, setPredChartAlgo] = useState<string>('');
  const [loading, setLoading] = useState(true);
  const [chartLoading, setChartLoading] = useState(false);
  const [predTableCoin, setPredTableCoin] = useState<string>('');
  const [confirmedCoin, setConfirmedCoin] = useState<string>('');
  const [predPage, setPredPage] = useState(0);
  const [confirmedPage, setConfirmedPage] = useState(0);

  // Load initial data
  useEffect(() => {
    setLoading(true);
    Promise.allSettled([
      apiFetch('/api/crypto/latest'),
      apiFetch('/api/crypto/predictions/latest'),
      apiFetch('/api/crypto/predictions/latest-results'),
    ]).then(([latestRes, predsRes, confirmedRes]) => {
      if (latestRes.status === 'fulfilled' && latestRes.value) {
        const raw = Array.isArray(latestRes.value) ? latestRes.value
          : Array.isArray(latestRes.value?.data) ? latestRes.value.data : [];
        setLatest(raw);
      }
      if (predsRes.status === 'fulfilled' && predsRes.value) {
        const raw = Array.isArray(predsRes.value) ? predsRes.value
          : Array.isArray(predsRes.value?.data) ? predsRes.value.data : [];
        setPreds(raw);
      }
      if (confirmedRes.status === 'fulfilled' && confirmedRes.value) {
        const raw = Array.isArray(confirmedRes.value) ? confirmedRes.value
          : Array.isArray(confirmedRes.value?.data) ? confirmedRes.value.data : [];
        setConfirmedResults(raw);
      }
      setLoading(false);
    });
  }, []);

  // Load price chart when coin or days changes
  useEffect(() => {
    setChartLoading(true);
    apiFetch(`/api/crypto/chart?coin=${encodeURIComponent(activeCoin)}&days=${days}`)
      .then((v: any) => {
        setChart({
          dates: Array.isArray(v.dates) ? v.dates : [],
          prices: Array.isArray(v.prices) ? v.prices.map(num) : [],
          granularity: v.granularity ?? '1d',
        });
      })
      .catch(() => {})
      .finally(() => setChartLoading(false));
  }, [activeCoin, days]);

  // Load prediction vs actual chart when predChartCoin or predChartAlgo or days changes
  useEffect(() => {
    const algoParam = predChartAlgo ? `&algorithm=${encodeURIComponent(predChartAlgo)}` : '';
    apiFetch(`/api/crypto/predictions/chart?coin=${encodeURIComponent(predChartCoin)}&days=${days}${algoParam}`)
      .then((v: any) => {
        const list = Array.isArray(v) ? v : Array.isArray(v?.data) ? v.data : [];
        if (list.length > 0) {
          setPredChart(buildMultiAlgoData(list, 'date', 'actual_price', 'predicted_price', 'algorithm_name', ddmm, days === '1'));
        } else {
          setPredChart({ labels: [], actual: [], predSeries: [] });
        }
      })
      .catch(() => setPredChart({ labels: [], actual: [], predSeries: [] }));
  }, [predChartCoin, predChartAlgo, days]);

  const activeCoinDef = COINS.find((c) => c.id === activeCoin) || COINS[0];
  const predChartCoinDef = COINS.find((c) => c.id === predChartCoin) || COINS[0];
  const fmtLabel = chart.granularity === '1h'
    ? (s: string) => s.length >= 16 ? s.slice(11, 16) : s
    : ddmm;
  const chartLabels = chart.dates.map(fmtLabel);

  // Deduplicate predictions: keep only the first occurrence per (coin + algorithm)
  const dedupPreds = (() => {
    const seen = new Set<string>();
    return preds.filter((p) => {
      const key = (p.coin_id || p.symbol || '') + '|' + p.algorithm_name;
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    });
  })();

  const PAGE_SIZE = 10;

  const matchCoin = (coinId: string | undefined, sym: string | undefined, filterId: string) =>
    COINS.some(c => c.id === filterId && (c.id === (coinId || '').toLowerCase() || c.symbol === (sym || '').toUpperCase()));

  const filteredDedupPreds = predTableCoin
    ? dedupPreds.filter(p => matchCoin(p.coin_id, p.symbol, predTableCoin))
    : dedupPreds;
  const predPageCount = Math.ceil(filteredDedupPreds.length / PAGE_SIZE);
  const predPagedItems = filteredDedupPreds.slice(predPage * PAGE_SIZE, (predPage + 1) * PAGE_SIZE);

  const filteredConfirmed = confirmedCoin
    ? confirmedResults.filter(r => matchCoin(r.coin_id, r.symbol, confirmedCoin))
    : confirmedResults;
  const confirmedPageCount = Math.ceil(filteredConfirmed.length / PAGE_SIZE);
  const confirmedPagedItems = filteredConfirmed.slice(confirmedPage * PAGE_SIZE, (confirmedPage + 1) * PAGE_SIZE);

  // Collect unique algorithm names from confirmed results for the filter dropdown
  const confirmedAlgos = Array.from(new Set(confirmedResults.map((r) => r.algorithm_name))).sort();

  // Build KPI items: prefer BTC and ETH, then others
  const kpiCoins = (() => {
    const byId = new Map(latest.map((c) => [c.coin_id, c]));
    const bySym = new Map(latest.map((c) => [c.symbol?.toUpperCase(), c]));
    const result: CryptoLatestItem[] = [];
    for (const coin of COINS) {
      const item = byId.get(coin.id) || bySym.get(coin.symbol);
      if (item) result.push(item);
    }
    // Fill remaining slots from the API response
    for (const item of latest) {
      if (!result.find((r) => r.coin_id === item.coin_id)) result.push(item);
    }
    return result.slice(0, 4);
  })();

  if (loading) {
    return (
      <div className="content__inner fade">
        <MarketTabs marketKey="crypto" />
        <div className="grid grid--kpis section-gap">
          {[1, 2, 3, 4].map((i) => <KPI key={i} label="—" value="—" sub={t.common.loading} />)}
        </div>
        <div className="empty section-gap">
          <div className="empty__icon"><Icon name="layers" size={18} /></div>
          <p>{t.crypto.loadingData}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="content__inner fade">
      <MarketTabs marketKey="crypto" />
      {/* KPI cards */}
      <div className="grid grid--kpis section-gap">
        {kpiCoins.length === 0 ? (
          [1, 2, 3, 4].map((i) => <KPI key={i} label="—" value="N/A" sub={t.crypto.noDataKpi} />)
        ) : (
          kpiCoins.map((c) => {
            const coinDef = COINS.find((d) => d.id === c.coin_id || d.symbol === c.symbol?.toUpperCase());
            return (
              <KPI
                key={c.coin_id}
                label={coinDef ? `${coinDef.label} (${coinDef.symbol})` : (c.symbol || c.coin_id)}
                value={fmtCrypto(num(c.close_price))}
                sub={c.market_cap ? 'MCap: ' + fmtMarketCap(num(c.market_cap)) : fmtDT(c.trading_date)}
                chgPct={c.change_percent != null ? num(c.change_percent) : undefined}
                sparkColor={coinDef?.color || 'var(--accent)'}
              />
            );
          })
        )}
      </div>

      {/* Price chart */}
      <Panel
        title={t.crypto.priceChart}
        dot={activeCoinDef.label + ' (' + activeCoinDef.symbol + ')'}
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
              value={days}
              onChange={setDays}
            />
            {isLoggedIn && (
              <>
                <button
                  className="btn btn--sm"
                  onClick={() =>
                    authPost('/api/trigger/crypto-crawler').then((ok) =>
                      vnsToast(ok ? t.crypto.collectRequest : t.crypto.collectFail)
                    )
                  }
                >
                  <Icon name="download" size={13} />{t.common.collect}
                </button>
                <button
                  className="btn btn--sm"
                  style={{ background: 'var(--accent)', borderColor: 'var(--accent)', color: '#fff' }}
                  onClick={() =>
                    authPost('/api/trigger/crypto-predict').then((ok) =>
                      vnsToast(ok ? t.crypto.predictRequest : t.crypto.predictFail)
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
        {/* Coin tabs */}
        <div className="chips" style={{ marginBottom: 16 }}>
          {COINS.map((c) => (
            <button
              key={c.id}
              className={`chip ${activeCoin === c.id ? 'active' : ''}`}
              onClick={() => setActiveCoin(c.id)}
            >
              {c.label} ({c.symbol})
            </button>
          ))}
          {/* Extra coins from API not in COINS list */}
          {latest
            .filter((c) => !COINS.find((d) => d.id === c.coin_id || d.symbol === c.symbol?.toUpperCase()))
            .map((c) => (
              <button
                key={c.coin_id}
                className={`chip ${activeCoin === c.coin_id ? 'active' : ''}`}
                onClick={() => setActiveCoin(c.coin_id)}
              >
                {c.symbol || c.coin_id}
              </button>
            ))
          }
        </div>

        {/* Current price display */}
        {(() => {
          const cur = latest.find((c) => c.coin_id === activeCoin);
          return cur ? (
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 14, marginBottom: 10 }}>
              <span className="num" style={{ fontSize: 30, fontWeight: 600, letterSpacing: '-1px' }}>
                {fmtCrypto(num(cur.close_price))}
              </span>
              <span style={{ fontSize: 12, color: 'var(--text-3)' }}>USD</span>
              {cur.change_percent != null && <Chg pct={num(cur.change_percent)} />}
              {cur.volume_24h != null && (
                <span style={{ fontSize: 11, color: 'var(--text-3)' }}>
                  Vol 24h: {fmtMarketCap(num(cur.volume_24h))}
                </span>
              )}
              <span style={{ marginLeft: 'auto', fontSize: 11, color: 'var(--text-3)', fontFamily: 'var(--font-mono)' }}>
                {fmtDT(cur.trading_date)}
              </span>
            </div>
          ) : null;
        })()}

        {chartLoading ? (
          <div className="empty" style={{ height: 540, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
            <div className="empty__icon"><Icon name="refresh" size={18} /></div>
            <p>{t.common.loadingChart}</p>
          </div>
        ) : chart.prices.length > 0 ? (
          <LineChart
            series={[{ name: activeCoinDef.label, data: chart.prices, color: activeCoinDef.color }]}
            labels={chartLabels}
            height={540}
            area
            yFmt={fmtCryptoShort}
            valueFmt={fmtCrypto}
            padL={70}
          />
        ) : (
          <div className="empty" style={{ height: 540, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
            <div className="empty__icon"><Icon name="layers" size={18} /></div>
            <p>{t.crypto.noChartData}</p>
          </div>
        )}
      </Panel>

      {/* Predictions + Latest confirmed results */}
      <div className="grid grid--halves section-gap">
        <Panel title={t.crypto.tomorrowPred} flush tools={
          <div className="chips" style={{ margin: 0 }}>
            <button className={`chip${predTableCoin === '' ? ' active' : ''}`} style={{ fontSize: 12, padding: '3px 10px' }} onClick={() => { setPredTableCoin(''); setPredPage(0); }}>{t.common.all}</button>
            {COINS.map(c => (
              <button key={c.id} className={`chip${predTableCoin === c.id ? ' active' : ''}`} style={{ fontSize: 12, padding: '3px 10px' }} onClick={() => { setPredTableCoin(c.id); setPredPage(0); }}>{c.symbol}</button>
            ))}
          </div>
        }>
          {filteredDedupPreds.length === 0 ? (
            <div className="empty">
              <div className="empty__icon"><Icon name="layers" size={18} /></div>
              <p>{dedupPreds.length === 0 ? t.common.noPredictions : t.crypto.noPredForCoin}</p>
            </div>
          ) : (
            <div style={{ overflowX: 'auto' }}>
              <table className="tbl">
                <thead>
                  <tr>
                    <th>{t.common.coin}</th>
                    <th className="c">{t.common.status}</th>
                    <th className="r">{t.common.current}</th>
                    <th className="r">{t.common.predicted}</th>
                    <th className="r">±%</th>
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
                    const sym = p.symbol || p.coin_id || '—';
                    return (
                      <tr key={i}>
                        <td className="sym" style={{ fontSize: 12.5 }}>{sym.toUpperCase()}</td>
                        <td className="c"><span className={`algo algo--${cls}`}>{short}</span></td>
                        <td className="r num" style={{ color: 'var(--text-2)', fontSize: 12 }}>{fmtCrypto(cur)}</td>
                        <td className="r num" style={{ fontWeight: 600, fontSize: 12 }}>{fmtCrypto(pred)}</td>
                        <td className="r"><Chg pct={deltaPct} /></td>
                        <td className="r"><ConfBar v={conf} /></td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
              {predPageCount > 1 && (
                <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', gap: 8, padding: '10px 16px', borderTop: '1px solid var(--border)', fontSize: 12, color: 'var(--text-2)' }}>
                  <button className="btn btn--sm" disabled={predPage === 0} onClick={() => setPredPage(p => p - 1)} style={{ minWidth: 28, padding: '2px 8px' }}>‹</button>
                  <span>{predPage + 1} / {predPageCount}</span>
                  <button className="btn btn--sm" disabled={predPage >= predPageCount - 1} onClick={() => setPredPage(p => p + 1)} style={{ minWidth: 28, padding: '2px 8px' }}>›</button>
                </div>
              )}
            </div>
          )}
        </Panel>

        <Panel title={t.common.latestPredResults} flush tools={
          <div className="chips" style={{ margin: 0 }}>
            <button className={`chip${confirmedCoin === '' ? ' active' : ''}`} style={{ fontSize: 12, padding: '3px 10px' }} onClick={() => { setConfirmedCoin(''); setConfirmedPage(0); }}>{t.common.all}</button>
            {COINS.map(c => (
              <button key={c.id} className={`chip${confirmedCoin === c.id ? ' active' : ''}`} style={{ fontSize: 12, padding: '3px 10px' }} onClick={() => { setConfirmedCoin(c.id); setConfirmedPage(0); }}>{c.symbol}</button>
            ))}
          </div>
        }>
          {filteredConfirmed.length === 0 ? (
            <div className="empty">
              <div className="empty__icon"><Icon name="pulse" size={18} /></div>
              <p>{confirmedResults.length === 0 ? t.common.noConfirmedResults : t.crypto.noConfirmedForCoin}</p>
            </div>
          ) : (
            <div style={{ overflowX: 'auto' }}>
              <table className="tbl">
                <thead>
                  <tr>
                    <th>{t.common.coin}</th>
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
                      : acc >= 95 ? 'var(--up)'
                      : acc >= 80 ? 'oklch(0.78 0.18 80)'
                      : 'var(--dn)';
                    const { short, cls } = algoShort(r.algorithm_name);
                    const sym = r.symbol || r.coin_id || '—';
                    return (
                      <tr key={i}>
                        <td className="sym" style={{ fontSize: 12.5 }}>{sym.toUpperCase()}</td>
                        <td className="c"><span className={`algo algo--${cls}`}>{short}</span></td>
                        <td className="r num" style={{ fontWeight: 600, fontSize: 12 }}>{fmtCrypto(predicted)}</td>
                        <td className="r num" style={{ color: 'var(--text-2)', fontSize: 12 }}>
                          {r.actual_price != null ? fmtCrypto(actual) : <span style={{ color: 'var(--text-3)' }}>—</span>}
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
              {confirmedPageCount > 1 && (
                <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', gap: 8, padding: '10px 16px', borderTop: '1px solid var(--border)', fontSize: 12, color: 'var(--text-2)' }}>
                  <button className="btn btn--sm" disabled={confirmedPage === 0} onClick={() => setConfirmedPage(p => p - 1)} style={{ minWidth: 28, padding: '2px 8px' }}>‹</button>
                  <span>{confirmedPage + 1} / {confirmedPageCount}</span>
                  <button className="btn btn--sm" disabled={confirmedPage >= confirmedPageCount - 1} onClick={() => setConfirmedPage(p => p + 1)} style={{ minWidth: 28, padding: '2px 8px' }}>›</button>
                </div>
              )}
            </div>
          )}
        </Panel>
      </div>

      {/* Prediction vs Actual chart — full width */}
      <Panel
        title={t.common.predVsActual}
        sub={predChartCoinDef.label + ' · ' + (predChartAlgo ? algoShort(predChartAlgo).short : t.common.allAlgos)}
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
              value={days}
              onChange={setDays}
            />
            {/* Coin selector tabs */}
            <div className="chips" style={{ margin: 0 }}>
              {COINS.map((c) => (
                <button
                  key={c.id}
                  className={`chip${predChartCoin === c.id ? ' active' : ''}`}
                  style={{ fontSize: 12, padding: '3px 10px' }}
                  onClick={() => setPredChartCoin(c.id)}
                >
                  {c.symbol}
                </button>
              ))}
            </div>
            {/* Algorithm filter */}
            {confirmedAlgos.length > 0 && (
              <select
                className="select-sm"
                value={predChartAlgo}
                onChange={(e) => setPredChartAlgo(e.target.value)}
                style={{
                  fontSize: 12,
                  padding: '3px 8px',
                  borderRadius: 6,
                  border: '1px solid var(--border)',
                  background: 'var(--surface-2)',
                  color: 'var(--text-1)',
                  cursor: 'pointer',
                }}
              >
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
              height={540}
              yFmt={fmtCryptoShort}
              valueFmt={fmtCrypto}
              padL={70}
            />
            <Legend items={[
              [t.common.actual, 'var(--text-2)'],
              ...predChart.predSeries.map((ps, i) => [algoDisplayName(ps.key), algoColor(ps.key, i)] as [string, string]),
            ]} />
          </>
        ) : (
          <div className="empty" style={{ height: 540, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
            <div className="empty__icon"><Icon name="layers" size={18} /></div>
            <p>{t.crypto.noCompareData} {predChartCoinDef.label}</p>
          </div>
        )}
      </Panel>

      {/* Market info table */}
      <div className="sec-head section-gap"><h2>{t.crypto.marketInfo}</h2><div className="line"></div></div>
      <Panel flush className="section-gap">
        {latest.length === 0 ? (
          <div className="empty">
            <div className="empty__icon"><Icon name="layers" size={18} /></div>
            <p>{t.common.noDataCollect}</p>
          </div>
        ) : (
          <div style={{ overflowX: 'auto' }}>
            <table className="tbl">
              <thead>
                <tr>
                  <th>{t.common.coin}</th>
                  <th className="r">USD</th>
                  <th className="r">±%</th>
                  <th className="r">MCap</th>
                  <th className="r">Vol 24h</th>
                  <th className="c">{t.common.date}</th>
                </tr>
              </thead>
              <tbody>
                {latest.map((c, i) => {
                  const coinDef = COINS.find((d) => d.id === c.coin_id || d.symbol === c.symbol?.toUpperCase());
                  return (
                    <tr key={i} className="clickable" onClick={() => setActiveCoin(c.coin_id)}>
                      <td className="sym">
                        {coinDef ? coinDef.label : (c.name || c.coin_id)}
                        <span style={{ marginLeft: 6, fontSize: 10, color: 'var(--text-3)' }}>
                          {c.symbol?.toUpperCase()}
                        </span>
                      </td>
                      <td className="r num" style={{ fontWeight: 600 }}>{fmtCrypto(num(c.close_price))}</td>
                      <td className="r">
                        {c.change_percent != null
                          ? <Chg pct={num(c.change_percent)} />
                          : <span style={{ color: 'var(--text-3)' }}>—</span>}
                      </td>
                      <td className="r num" style={{ color: 'var(--text-2)' }}>
                        {c.market_cap != null ? fmtMarketCap(num(c.market_cap)) : '—'}
                      </td>
                      <td className="r num" style={{ color: 'var(--text-3)', fontSize: 12 }}>
                        {c.volume_24h != null ? fmtMarketCap(num(c.volume_24h)) : '—'}
                      </td>
                      <td className="c num" style={{ color: 'var(--text-3)', fontSize: 12 }}>
                        {fmtDT(c.trading_date)}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </Panel>
    </div>
  );
}
