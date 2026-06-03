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
interface FuelLatestItem {
  product_type: string;
  price: number;
  trading_date: string;
  change?: number;
  change_percent?: number;
}

interface ChartData {
  dates: string[];
  prices: number[];
}

interface PredictionItem {
  product_type?: string;
  algorithm_name: string;
  current_price: number;
  predicted_price: number;
  confidence: number;
  prediction_date: string;
  actual_price?: number | null;
  accuracy?: number | null;
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

// API returns giá nghìn VND (e.g., 24.15 = 24.150 VND/lít)
// If value > 1000, it's already in VND; otherwise multiply by 1000
function normalizePrice(raw: number): number {
  return raw > 1000 ? raw : raw * 1000;
}

function fmtFuelPrice(v: number): string {
  const p = normalizePrice(v);
  return p.toLocaleString('vi-VN') + ' đ';
}

function fmtFuelShort(v: number): string {
  const p = normalizePrice(v);
  if (p >= 1000) return (p / 1000).toFixed(1) + 'K';
  return '' + Math.round(p);
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

// ── Product definitions ───────────────────────────────────────────────────────
const PRODUCTS: { id: string; label: string; short: string; color: string }[] = [
  { id: 'ron95_iii',  label: 'Xăng RON 95-III',    short: 'RON 95', color: 'var(--up)'     },
  { id: 'e5_ron92',   label: 'Xăng E5 RON 92-II',  short: 'E5 92',  color: 'var(--accent)' },
  { id: 'do_005s',    label: 'Dầu DO 0,05S-II',     short: 'Diesel', color: 'oklch(0.74 0.13 200)' },
  { id: 'kerosene',   label: 'Dầu hỏa 2-K',         short: 'Dầu hỏa', color: 'var(--gold)'  },
];

const KPI_PRODUCTS = PRODUCTS.map((p) => p.id);

export default function Fuel() {
  const { isLoggedIn } = useAuth();

  const [latest, setLatest] = useState<FuelLatestItem[]>([]);
  const [activeProduct, setActiveProduct] = useState<string>('ron95_iii');
  const [days, setDays] = useState('180');
  const [chart, setChart] = useState<ChartData>({ dates: [], prices: [] });
  const [preds, setPreds] = useState<PredictionItem[]>([]);
  const [confirmedResults, setConfirmedResults] = useState<PredictionItem[]>([]);
  const [predChart, setPredChart] = useState<PredChartData>({ labels: [], actual: [], pred: [] });
  const [loading, setLoading] = useState(true);
  const [chartLoading, setChartLoading] = useState(false);

  // Load initial data
  useEffect(() => {
    setLoading(true);
    Promise.allSettled([
      apiFetch('/api/fuel/latest'),
      apiFetch('/api/fuel/predictions/latest'),
      apiFetch('/api/fuel/predictions/latest-results'),
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

  // Load chart when product or days changes
  useEffect(() => {
    setChartLoading(true);
    Promise.allSettled([
      apiFetch(`/api/fuel/chart?product=${encodeURIComponent(activeProduct)}&days=${days}`),
      apiFetch(`/api/fuel/predictions/chart?product=${encodeURIComponent(activeProduct)}&days=${days}`),
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
  }, [activeProduct, days]);

  const activeProductDef = PRODUCTS.find((p) => p.id === activeProduct) || PRODUCTS[0];
  const n = parseInt(days);
  // Fuel data is sparse (~52 updates/year), use wider label spacing
  const labelEvery = Math.max(1, Math.ceil(n / 8));
  const chartLabels = chart.dates.map(ddmm);

  // KPI cards: one per defined product
  const kpiItems = (() => {
    const byProduct = new Map(latest.map((p) => [p.product_type, p]));
    return KPI_PRODUCTS
      .map((id) => ({ def: PRODUCTS.find((p) => p.id === id)!, item: byProduct.get(id) || null }))
      .filter((x) => x.def);
  })();

  if (loading) {
    return (
      <div className="content__inner fade">
        <MarketTabs marketKey="fuel" />
        <div className="grid grid--kpis section-gap">
          {[1, 2, 3, 4].map((i) => <KPI key={i} label="—" value="—" sub="Loading..." />)}
        </div>
        <div className="empty section-gap">
          <div className="empty__icon"><Icon name="layers" size={18} /></div>
          <p>Đang tải dữ liệu giá xăng...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="content__inner fade">
      <MarketTabs marketKey="fuel" />
      {/* Info banner */}
      <div
        className="panel section-gap"
        style={{
          borderColor: 'var(--gold)',
          background: 'color-mix(in oklch, var(--gold) 6%, var(--bg-1))',
          marginBottom: 0,
        }}
      >
        <div className="panel__body" style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '10px 16px' }}>
          <Icon name="clock" size={16} style={{ color: 'var(--gold)', flexShrink: 0 }} />
          <span style={{ fontSize: 13, color: 'var(--text-2)' }}>
            Giá xăng điều chỉnh theo chu kỳ ~7 ngày (quyết định của Bộ Công Thương). Dữ liệu thưa hơn thị trường chứng khoán — khoảng 52 chu kỳ/năm.
          </span>
        </div>
      </div>

      {/* KPI cards */}
      <div className="grid grid--kpis section-gap">
        {kpiItems.map(({ def, item }) => (
          <KPI
            key={def.id}
            label={def.short}
            value={item ? fmtFuelPrice(num(item.price)) : 'N/A'}
            sub={item ? (item.trading_date?.slice(0, 10) || '—') : 'Chưa có dữ liệu'}
            chgPct={item?.change_percent != null ? num(item.change_percent) : undefined}
            sparkColor={def.color}
          />
        ))}
      </div>

      {/* Price chart */}
      <Panel
        title="Biểu đồ giá xăng dầu"
        dot={activeProductDef.label}
        className="section-gap"
        tools={
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            <Seg
              options={[
                { value: '90', label: '90N' },
                { value: '180', label: '180N' },
                { value: '365', label: '1N' },
              ]}
              value={days}
              onChange={setDays}
            />
            {isLoggedIn && (
              <>
                <button
                  className="btn btn--sm"
                  onClick={() =>
                    authPost('/api/trigger/fuel-crawler').then((ok) =>
                      vnsToast(ok ? 'Đã gửi yêu cầu thu thập giá xăng' : 'Không thể gửi yêu cầu thu thập')
                    )
                  }
                >
                  <Icon name="download" size={13} />Thu thập
                </button>
                <button
                  className="btn btn--sm"
                  style={{ background: 'var(--up)', borderColor: 'var(--up)', color: '#fff' }}
                  onClick={() =>
                    authPost('/api/trigger/fuel-predict').then((ok) =>
                      vnsToast(ok ? 'Đã gửi yêu cầu chạy dự đoán giá xăng' : 'Không thể gửi yêu cầu dự đoán')
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
        {/* Product selector chips */}
        <div className="chips" style={{ marginBottom: 16 }}>
          {PRODUCTS.map((p) => (
            <button
              key={p.id}
              className={`chip ${activeProduct === p.id ? 'active' : ''}`}
              onClick={() => setActiveProduct(p.id)}
            >
              {p.label}
            </button>
          ))}
        </div>

        {/* Current price display */}
        {(() => {
          const cur = latest.find((p) => p.product_type === activeProduct);
          return cur ? (
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 14, marginBottom: 10 }}>
              <span className="num" style={{ fontSize: 30, fontWeight: 600, letterSpacing: '-1px' }}>
                {fmtFuelPrice(num(cur.price))}
              </span>
              <span style={{ fontSize: 12, color: 'var(--text-3)' }}>/ lít</span>
              {cur.change_percent != null && <Chg pct={num(cur.change_percent)} />}
              <span style={{ marginLeft: 'auto', fontSize: 11, color: 'var(--text-3)', fontFamily: 'var(--font-mono)' }}>
                {cur.trading_date?.slice(0, 10) || '—'}
              </span>
            </div>
          ) : null;
        })()}

        {chartLoading ? (
          <div className="empty" style={{ height: 540, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
            <div className="empty__icon"><Icon name="refresh" size={18} /></div>
            <p>Đang tải biểu đồ...</p>
          </div>
        ) : chart.prices.length > 0 ? (
          <LineChart
            series={[{ name: activeProductDef.label, data: chart.prices, color: activeProductDef.color }]}
            labels={chartLabels}
            height={540}
            area
            yFmt={fmtFuelShort}
            valueFmt={fmtFuelPrice}
            padL={54}
          />
        ) : (
          <div className="empty" style={{ height: 540, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
            <div className="empty__icon"><Icon name="layers" size={18} /></div>
            <p>Chưa có dữ liệu biểu đồ. Hãy thu thập dữ liệu trước.</p>
          </div>
        )}
      </Panel>

      {/* Predictions + Confirmed results */}
      <div className="grid grid--halves section-gap">
        <Panel title="Dự đoán giá xăng kỳ tới" flush>
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
                    <th>Sản phẩm</th>
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
                    const prodDef = PRODUCTS.find((d) => d.id === p.product_type);
                    const prodLabel = prodDef ? prodDef.short : (p.product_type || '—');
                    return (
                      <tr key={i}>
                        <td className="sym" style={{ fontSize: 12.5 }}>{prodLabel}</td>
                        <td className="c"><span className={`algo algo--${cls}`}>{short}</span></td>
                        <td className="r num" style={{ color: 'var(--text-2)', fontSize: 12 }}>{fmtFuelPrice(cur)}</td>
                        <td className="r num" style={{ fontWeight: 600, fontSize: 12 }}>{fmtFuelPrice(pred)}</td>
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

        <Panel title="Kết quả dự đoán gần nhất" flush>
          {confirmedResults.length === 0 ? (
            <div className="empty">
              <div className="empty__icon"><Icon name="pulse" size={18} /></div>
              <p>Chưa có kết quả đã xác nhận. Kết quả sẽ xuất hiện sau khi dự đoán được đối chiếu với giá thực tế.</p>
            </div>
          ) : (
            <div style={{ overflowX: 'auto' }}>
              <table className="tbl">
                <thead>
                  <tr>
                    <th>Sản phẩm</th>
                    <th className="c">TT</th>
                    <th className="r">Dự đoán</th>
                    <th className="r">Thực tế</th>
                    <th className="r">Lệch</th>
                    <th className="r">Độ CX</th>
                    <th className="c">Ngày</th>
                  </tr>
                </thead>
                <tbody>
                  {confirmedResults.map((r, i) => {
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
                    const prodDef = PRODUCTS.find((d) => d.id === r.product_type);
                    const prodLabel = prodDef ? prodDef.short : (r.product_type || '—');
                    return (
                      <tr key={i}>
                        <td className="sym" style={{ fontSize: 12.5 }}>{prodLabel}</td>
                        <td className="c"><span className={`algo algo--${cls}`}>{short}</span></td>
                        <td className="r num" style={{ fontWeight: 600, fontSize: 12 }}>{fmtFuelPrice(predicted)}</td>
                        <td className="r num" style={{ color: 'var(--text-2)', fontSize: 12 }}>
                          {r.actual_price != null ? fmtFuelPrice(actual) : <span style={{ color: 'var(--text-3)' }}>—</span>}
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
                          {ddmm(r.prediction_date)}
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

      {/* Prediction vs Actual chart — full width */}
      <Panel title="Dự đoán vs Thực tế" sub={activeProductDef.label + ' · Ensemble'} className="section-gap">
        {predChart.labels.length > 0 ? (
          <>
            <LineChart
              series={[
                { name: 'Thực tế', data: predChart.actual, color: 'var(--text-2)', w: 1.8 },
                { name: 'Dự đoán', data: predChart.pred, color: activeProductDef.color, dash: '5 4', w: 2 },
              ]}
              labels={predChart.labels}
              height={540}
              yFmt={fmtFuelShort}
              valueFmt={fmtFuelPrice}
              padL={54}
            />
            <Legend items={[['Thực tế', 'var(--text-2)'], ['Dự đoán', activeProductDef.color]]} />
          </>
        ) : (
          <div className="empty" style={{ height: 540, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
            <div className="empty__icon"><Icon name="layers" size={18} /></div>
            <p>Chưa có dữ liệu so sánh dự đoán</p>
          </div>
        )}
      </Panel>

      {/* Current prices table */}
      <div className="sec-head section-gap"><h2>Bảng giá xăng dầu hiện tại</h2><div className="line"></div></div>
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
                  <th>Sản phẩm</th>
                  <th className="r">Giá bán lẻ (VND/lít)</th>
                  <th className="r">±%</th>
                  <th className="c">Ngày điều chỉnh</th>
                </tr>
              </thead>
              <tbody>
                {latest.map((p, i) => {
                  const prodDef = PRODUCTS.find((d) => d.id === p.product_type);
                  return (
                    <tr key={i} className="clickable" onClick={() => setActiveProduct(p.product_type)}>
                      <td>
                        <span className="sym">{prodDef ? prodDef.label : p.product_type}</span>
                      </td>
                      <td className="r num" style={{ fontWeight: 600 }}>{fmtFuelPrice(num(p.price))}</td>
                      <td className="r">
                        {p.change_percent != null
                          ? <Chg pct={num(p.change_percent)} />
                          : <span style={{ color: 'var(--text-3)' }}>—</span>}
                      </td>
                      <td className="c num" style={{ color: 'var(--text-3)', fontSize: 12 }}>
                        {p.trading_date ? p.trading_date.slice(0, 10) : '—'}
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
