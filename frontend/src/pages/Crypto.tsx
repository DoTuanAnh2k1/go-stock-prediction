import { useState, useEffect } from 'react';
import { NavLink } from 'react-router-dom';
import { Panel, KPI, Icon, Chg, Seg, ConfBar, vnsToast } from '../components/ui';
import { LineChart } from '../components/charts';
import { useAuth } from '../context/AuthContext';

// ── Market tabs ───────────────────────────────────────────────────────────────
function MarketTabs({ marketKey }: { marketKey: string }) {
  const base = '/markets/' + marketKey;
  return (
    <div className="market-tabs">
      <NavLink to={base} end className={({ isActive }) => 'market-tab' + (isActive ? ' active' : '')}>
        <Icon name="candles" size={14} />
        Tổng quan
      </NavLink>
      <NavLink to={base + '/predictions'} className={({ isActive }) => 'market-tab' + (isActive ? ' active' : '')}>
        <Icon name="pulse" size={14} />
        Dự đoán
      </NavLink>
      <NavLink to={base + '/detail'} className={({ isActive }) => 'market-tab' + (isActive ? ' active' : '')}>
        <Icon name="layers" size={14} />
        Chi tiết
      </NavLink>
      <NavLink to={base + '/training'} className={({ isActive }) => 'market-tab' + (isActive ? ' active' : '')}>
        <Icon name="cpu" size={14} />
        Huấn luyện
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
}

interface PredictionItem {
  coin_id?: string;
  symbol?: string;
  algorithm_name: string;
  current_price: number;
  predicted_price: number;
  confidence: number;
  prediction_date: string;
}

interface PredChartData {
  labels: string[];
  actual: (number | null)[];
  pred: (number | null)[];
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
];

export default function Crypto() {
  const { isLoggedIn } = useAuth();

  const [latest, setLatest] = useState<CryptoLatestItem[]>([]);
  const [activeCoin, setActiveCoin] = useState<string>('bitcoin');
  const [days, setDays] = useState('90');
  const [chart, setChart] = useState<ChartData>({ dates: [], prices: [] });
  const [preds, setPreds] = useState<PredictionItem[]>([]);
  const [predChart, setPredChart] = useState<PredChartData>({ labels: [], actual: [], pred: [] });
  const [loading, setLoading] = useState(true);
  const [chartLoading, setChartLoading] = useState(false);

  // Load initial data
  useEffect(() => {
    setLoading(true);
    Promise.allSettled([
      apiFetch('/api/crypto/latest'),
      apiFetch('/api/crypto/predictions/latest'),
    ]).then(([latestRes, predsRes]) => {
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
      setLoading(false);
    });
  }, []);

  // Load chart when coin or days changes
  useEffect(() => {
    setChartLoading(true);
    Promise.allSettled([
      apiFetch(`/api/crypto/chart?coin=${encodeURIComponent(activeCoin)}&days=${days}`),
      apiFetch(`/api/crypto/predictions/chart?coin=${encodeURIComponent(activeCoin)}&days=${days}`),
    ]).then(([chartRes, predChartRes]) => {
      if (chartRes.status === 'fulfilled' && chartRes.value) {
        const v = chartRes.value;
        setChart({
          dates: Array.isArray(v.dates) ? v.dates : [],
          prices: Array.isArray(v.prices) ? v.prices.map(num) : [],
        });
      }
      if (predChartRes.status === 'fulfilled' && predChartRes.value) {
        const v = predChartRes.value;
        const list = Array.isArray(v) ? v : Array.isArray(v?.data) ? v.data : [];
        if (list.length > 0) {
          setPredChart({
            labels: list.map((d: any) => ddmm(d.date || d.prediction_date || '')),
            actual: list.map((d: any) => d.actual_price != null ? num(d.actual_price) : null),
            pred: list.map((d: any) => d.predicted_price != null ? num(d.predicted_price) : null),
          });
        } else if (v.labels) {
          setPredChart({
            labels: Array.isArray(v.labels) ? v.labels.map(ddmm) : [],
            actual: Array.isArray(v.actual) ? v.actual.map((x: any) => x != null ? num(x) : null) : [],
            pred: Array.isArray(v.pred) ? v.pred.map((x: any) => x != null ? num(x) : null) : [],
          });
        }
      }
      setChartLoading(false);
    });
  }, [activeCoin, days]);

  const activeCoinDef = COINS.find((c) => c.id === activeCoin) || COINS[0];
  const n = parseInt(days);
  const chartLabels = chart.dates.map((d, i) =>
    i % Math.ceil(n / 7) === 0 ? ddmm(d) : ''
  );

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
          {[1, 2, 3, 4].map((i) => <KPI key={i} label="—" value="—" sub="Loading..." />)}
        </div>
        <div className="empty section-gap">
          <div className="empty__icon"><Icon name="layers" size={18} /></div>
          <p>Đang tải dữ liệu crypto...</p>
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
          [1, 2, 3, 4].map((i) => <KPI key={i} label="—" value="N/A" sub="Chưa có dữ liệu" />)
        ) : (
          kpiCoins.map((c) => {
            const coinDef = COINS.find((d) => d.id === c.coin_id || d.symbol === c.symbol?.toUpperCase());
            return (
              <KPI
                key={c.coin_id}
                label={coinDef ? `${coinDef.label} (${coinDef.symbol})` : (c.symbol || c.coin_id)}
                value={fmtCrypto(num(c.close_price))}
                sub={c.market_cap ? 'MCap: ' + fmtMarketCap(num(c.market_cap)) : c.trading_date?.slice(0, 10) || '—'}
                chgPct={c.change_percent != null ? num(c.change_percent) : undefined}
                sparkColor={coinDef?.color || 'var(--accent)'}
              />
            );
          })
        )}
      </div>

      {/* Price chart */}
      <Panel
        title="Biểu đồ giá Crypto"
        dot={activeCoinDef.label + ' (' + activeCoinDef.symbol + ')'}
        className="section-gap"
        tools={
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            <Seg
              options={[
                { value: '30', label: '30N' },
                { value: '90', label: '90N' },
                { value: '180', label: '180N' },
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
                      vnsToast(ok ? 'Đã gửi yêu cầu thu thập Crypto' : 'Không thể gửi yêu cầu thu thập')
                    )
                  }
                >
                  <Icon name="download" size={13} />Thu thập
                </button>
                <button
                  className="btn btn--sm"
                  style={{ background: 'var(--accent)', borderColor: 'var(--accent)', color: '#fff' }}
                  onClick={() =>
                    authPost('/api/trigger/crypto-predict').then((ok) =>
                      vnsToast(ok ? 'Đã gửi yêu cầu chạy dự đoán Crypto' : 'Không thể gửi yêu cầu dự đoán')
                    )
                  }
                >
                  <Icon name="play" size={13} />Dự đoán
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
                {cur.trading_date?.slice(0, 10) || '—'}
              </span>
            </div>
          ) : null;
        })()}

        {chartLoading ? (
          <div className="empty" style={{ height: 300, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
            <div className="empty__icon"><Icon name="refresh" size={18} /></div>
            <p>Đang tải biểu đồ...</p>
          </div>
        ) : chart.prices.length > 0 ? (
          <LineChart
            series={[{ name: activeCoinDef.label, data: chart.prices, color: activeCoinDef.color }]}
            labels={chartLabels}
            height={300}
            area
            yFmt={fmtCryptoShort}
            valueFmt={fmtCrypto}
            padL={70}
          />
        ) : (
          <div className="empty" style={{ height: 300, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
            <div className="empty__icon"><Icon name="layers" size={18} /></div>
            <p>Chưa có dữ liệu biểu đồ. Hãy thu thập dữ liệu trước.</p>
          </div>
        )}
      </Panel>

      {/* Predictions + Prediction chart */}
      <div className="grid grid--halves section-gap">
        <Panel title="Dự đoán Crypto phiên mai" flush>
          {preds.length === 0 ? (
            <div className="empty">
              <div className="empty__icon"><Icon name="layers" size={18} /></div>
              <p>Chưa có dự đoán. Hãy chạy dự đoán trước.</p>
            </div>
          ) : (
            <div style={{ overflowX: 'auto' }}>
              <table className="tbl">
                <thead>
                  <tr>
                    <th>Coin</th>
                    <th className="c">TT</th>
                    <th className="r">Hiện tại</th>
                    <th className="r">Dự đoán</th>
                    <th className="r">±%</th>
                    <th className="r">Tin cậy</th>
                  </tr>
                </thead>
                <tbody>
                  {preds.map((p, i) => {
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
            </div>
          )}
        </Panel>

        <Panel
          title="Dự đoán vs Thực tế"
          sub={activeCoinDef.label + ' · Ensemble'}
        >
          {predChart.labels.length > 0 ? (
            <>
              <LineChart
                series={[
                  { name: 'Thực tế', data: predChart.actual, color: 'var(--text-2)', w: 1.8 },
                  { name: 'Dự đoán', data: predChart.pred, color: activeCoinDef.color, dash: '5 4', w: 2 },
                ]}
                labels={predChart.labels.map((l, i) => i % 5 === 0 ? l : '')}
                height={236}
                yFmt={fmtCryptoShort}
                valueFmt={fmtCrypto}
                padL={70}
              />
              <Legend items={[['Thực tế', 'var(--text-2)'], ['Dự đoán', activeCoinDef.color]]} />
            </>
          ) : (
            <div className="empty" style={{ height: 236, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
              <div className="empty__icon"><Icon name="layers" size={18} /></div>
              <p>Chưa có dữ liệu so sánh dự đoán</p>
            </div>
          )}
        </Panel>
      </div>

      {/* Market info table */}
      <div className="sec-head section-gap"><h2>Thông tin thị trường Crypto</h2><div className="line"></div></div>
      <Panel flush className="section-gap">
        {latest.length === 0 ? (
          <div className="empty">
            <div className="empty__icon"><Icon name="layers" size={18} /></div>
            <p>Chưa có dữ liệu. Hãy thu thập dữ liệu trước.</p>
          </div>
        ) : (
          <div style={{ overflowX: 'auto' }}>
            <table className="tbl">
              <thead>
                <tr>
                  <th>Coin</th>
                  <th className="r">Giá (USD)</th>
                  <th className="r">±%</th>
                  <th className="r">Vốn hóa</th>
                  <th className="r">Vol 24h</th>
                  <th className="c">Ngày</th>
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
                        {c.trading_date?.slice(0, 10) || '—'}
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
