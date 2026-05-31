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
interface NasdaqLatestItem {
  symbol: string;
  close_price: number;
  trading_date: string;
  currency: string;
  change?: number;
  change_percent?: number;
}

interface ChartData {
  dates: string[];
  prices: number[];
}

interface PredictionItem {
  symbol: string;
  algorithm_name: string;
  current_price: number;
  predicted_price: number;
  confidence: number;
  prediction_date: string;
  target_date?: string;
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

function fmtUSD(v: number): string {
  return '$' + v.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

function fmtUSDShort(v: number): string {
  if (v >= 1000) return '$' + (v / 1000).toFixed(1) + 'K';
  return '$' + v.toFixed(0);
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

// ── Top symbols to show as KPI cards ─────────────────────────────────────────
const KPI_SYMBOLS = ['QQQ', 'AAPL', 'MSFT', 'NVDA'];

export default function Nasdaq() {
  const { isLoggedIn } = useAuth();

  // State
  const [latest, setLatest] = useState<NasdaqLatestItem[]>([]);
  const [activeSym, setActiveSym] = useState<string | null>(null);
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
      apiFetch('/api/nasdaq/latest'),
      apiFetch('/api/nasdaq/predictions/latest'),
    ]).then(([latestRes, predsRes]) => {
      if (latestRes.status === 'fulfilled' && latestRes.value) {
        const raw = Array.isArray(latestRes.value) ? latestRes.value
          : Array.isArray(latestRes.value?.data) ? latestRes.value.data : [];
        setLatest(raw);
        if (raw.length > 0 && !activeSym) {
          setActiveSym(raw[0].symbol);
        }
      }
      if (predsRes.status === 'fulfilled' && predsRes.value) {
        const raw = Array.isArray(predsRes.value) ? predsRes.value
          : Array.isArray(predsRes.value?.data) ? predsRes.value.data : [];
        setPreds(raw);
      }
      setLoading(false);
    });
  }, []);

  // Load chart when symbol or days changes
  useEffect(() => {
    if (!activeSym) return;
    setChartLoading(true);
    Promise.allSettled([
      apiFetch(`/api/nasdaq/chart?symbol=${encodeURIComponent(activeSym)}&days=${days}`),
      apiFetch(`/api/nasdaq/predictions/chart?symbol=${encodeURIComponent(activeSym)}&days=${days}`),
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
  }, [activeSym, days]);

  // KPI cards: prioritise known symbols, fill with whatever we got
  const kpiItems = (() => {
    const bySymbol = new Map(latest.map((s) => [s.symbol, s]));
    const result: NasdaqLatestItem[] = [];
    for (const sym of KPI_SYMBOLS) {
      const item = bySymbol.get(sym);
      if (item) result.push(item);
    }
    for (const item of latest) {
      if (!KPI_SYMBOLS.includes(item.symbol)) result.push(item);
    }
    return result.slice(0, 4);
  })();

  const n = parseInt(days);
  const chartLabels = chart.dates.map((d, i) =>
    i % Math.ceil(n / 7) === 0 ? ddmm(d) : ''
  );

  if (loading) {
    return (
      <div className="content__inner fade">
        <MarketTabs marketKey="nasdaq100" />
        <div className="grid grid--kpis section-gap">
          {[1, 2, 3, 4].map((i) => <KPI key={i} label="—" value="—" sub="Loading..." />)}
        </div>
        <div className="empty section-gap">
          <div className="empty__icon"><Icon name="layers" size={18} /></div>
          <p>Đang tải dữ liệu NASDAQ 100...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="content__inner fade">
      <MarketTabs marketKey="nasdaq100" />
      {/* KPI cards */}
      <div className="grid grid--kpis section-gap">
        {kpiItems.length === 0
          ? [1, 2, 3, 4].map((i) => (
              <KPI key={i} label="—" value="N/A" sub="Chưa có dữ liệu" />
            ))
          : kpiItems.map((s) => (
              <KPI
                key={s.symbol}
                label={s.symbol}
                value={fmtUSD(num(s.close_price))}
                sub={s.trading_date ? s.trading_date.slice(0, 10) : '—'}
                chgPct={s.change_percent != null ? num(s.change_percent) : undefined}
                sparkColor="var(--accent)"
              />
            ))
        }
      </div>

      {/* Price chart */}
      <Panel
        title="Biểu đồ giá NASDAQ 100"
        dot={activeSym || '—'}
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
                    authPost('/api/trigger/nasdaq-crawler').then((ok) =>
                      vnsToast(ok ? 'Đã gửi yêu cầu thu thập NASDAQ' : 'Không thể gửi yêu cầu thu thập')
                    )
                  }
                >
                  <Icon name="download" size={13} />Thu thập
                </button>
                <button
                  className="btn btn--sm"
                  style={{ background: 'var(--accent)', borderColor: 'var(--accent)', color: '#fff' }}
                  onClick={() =>
                    authPost('/api/trigger/nasdaq-predict').then((ok) =>
                      vnsToast(ok ? 'Đã gửi yêu cầu chạy dự đoán NASDAQ' : 'Không thể gửi yêu cầu dự đoán')
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
        {/* Symbol selector chips */}
        {latest.length > 0 && (
          <div className="chips" style={{ marginBottom: 16 }}>
            {latest.map((s) => (
              <button
                key={s.symbol}
                className={`chip ${activeSym === s.symbol ? 'active' : ''}`}
                onClick={() => setActiveSym(s.symbol)}
              >
                {s.symbol}
              </button>
            ))}
          </div>
        )}

        {activeSym && (() => {
          const cur = latest.find((s) => s.symbol === activeSym);
          return cur ? (
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 14, marginBottom: 10 }}>
              <span className="num" style={{ fontSize: 30, fontWeight: 600, letterSpacing: '-1px' }}>
                {fmtUSD(num(cur.close_price))}
              </span>
              <span style={{ fontSize: 12, color: 'var(--text-3)' }}>USD / share</span>
              {cur.change_percent != null && <Chg pct={num(cur.change_percent)} />}
              <span style={{ marginLeft: 'auto', fontSize: 11, color: 'var(--text-3)', fontFamily: 'var(--font-mono)' }}>
                NASDAQ 100 · {cur.trading_date ? cur.trading_date.slice(0, 10) : '—'}
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
            series={[{ name: activeSym || 'Price', data: chart.prices, color: 'var(--accent)' }]}
            labels={chartLabels}
            height={300}
            area
            yFmt={fmtUSDShort}
            valueFmt={fmtUSD}
            padL={58}
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
        <Panel title="Dự đoán NASDAQ phiên mai" flush>
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
                    <th>Mã</th>
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
                    return (
                      <tr key={i}>
                        <td className="sym" style={{ fontSize: 12.5 }}>{p.symbol}</td>
                        <td className="c"><span className={`algo algo--${cls}`}>{short}</span></td>
                        <td className="r num" style={{ color: 'var(--text-2)', fontSize: 12 }}>{fmtUSD(cur)}</td>
                        <td className="r num" style={{ fontWeight: 600, fontSize: 12 }}>{fmtUSD(pred)}</td>
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

        <Panel title="Dự đoán vs Thực tế" sub={activeSym ? activeSym + ' · Ensemble' : 'NASDAQ 100'}>
          {predChart.labels.length > 0 ? (
            <>
              <LineChart
                series={[
                  { name: 'Thực tế', data: predChart.actual, color: 'var(--text-2)', w: 1.8 },
                  { name: 'Dự đoán', data: predChart.pred, color: 'var(--accent)', dash: '5 4', w: 2 },
                ]}
                labels={predChart.labels.map((l, i) => i % 5 === 0 ? l : '')}
                height={236}
                yFmt={fmtUSDShort}
                valueFmt={fmtUSD}
                padL={58}
              />
              <Legend items={[['Thực tế', 'var(--text-2)'], ['Dự đoán', 'var(--accent)']]} />
            </>
          ) : (
            <div className="empty" style={{ height: 236, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
              <div className="empty__icon"><Icon name="layers" size={18} /></div>
              <p>Chưa có dữ liệu so sánh dự đoán</p>
            </div>
          )}
        </Panel>
      </div>

      {/* Latest prices table */}
      <div className="sec-head section-gap"><h2>Bảng giá NASDAQ 100</h2><div className="line"></div></div>
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
                  <th>Mã</th>
                  <th className="r">Giá đóng (USD)</th>
                  <th className="r">±%</th>
                  <th className="c">Ngày</th>
                </tr>
              </thead>
              <tbody>
                {latest.map((s, i) => (
                  <tr key={i} className="clickable" onClick={() => setActiveSym(s.symbol)}>
                    <td className="sym">{s.symbol}</td>
                    <td className="r num" style={{ fontWeight: 600 }}>{fmtUSD(num(s.close_price))}</td>
                    <td className="r">
                      {s.change_percent != null
                        ? <Chg pct={num(s.change_percent)} />
                        : <span style={{ color: 'var(--text-3)' }}>—</span>}
                    </td>
                    <td className="c num" style={{ color: 'var(--text-3)', fontSize: 12 }}>
                      {s.trading_date ? s.trading_date.slice(0, 10) : '—'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Panel>
    </div>
  );
}
