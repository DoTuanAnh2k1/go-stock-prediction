import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Panel, KPI, Icon, Chg, vnsToast } from '../components/ui';
import { LineChart } from '../components/charts';
import { useAuth } from '../context/AuthContext';

// ── Types ────────────────────────────────────────────────────────────────────

interface BotSession {
  id: number;
  status: string;
  start_date: string;
  end_date: string;
}

interface BotKPIs {
  total_return_pct: number | null;
  annualized_return_pct: number | null;
  sharpe_ratio: number | null;
  max_drawdown_pct: number | null;
  win_rate_pct: number | null;
  profit_factor: number | null;
  total_trades: number | null;
  best_trade_pct: number | null;
  worst_trade_pct: number | null;
}

interface BotDetail {
  id: string;
  market: string;
  algorithm: string;
  display_name: string;
  initial_capital: number | null;
  currency: string;
  is_active: boolean;
  buy_threshold: number | null;
  sell_threshold: number | null;
  min_confidence: number | null;
  stop_loss: number | null;
  take_profit: number | null;
  last_session: BotSession | null;
  kpis: BotKPIs | null;
}

interface BotChart {
  bot_id: string;
  session_id: number;
  dates: string[];
  values: number[];
  returns_pct: number[];
}

interface Trade {
  id: number;
  symbol: string;
  action: 'BUY' | 'SELL';
  quantity: number;
  price: number;
  trade_value: number;
  signal_strength: number | null;
  confidence: number | null;
  trade_date: string;
  close_reason: string | null;
  pnl: number | null;
  pnl_pct: number | null;
}

interface TradesResponse {
  bot_id: string;
  session_id: number;
  total: number;
  page: number;
  limit: number;
  data: Trade[];
}

// ── Helpers ──────────────────────────────────────────────────────────────────

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

function fmtCapital(v: number | null, currency: string): string {
  if (v == null) return '—';
  if (currency === 'VND') {
    if (v >= 1e9) return (v / 1e9).toFixed(2) + ' tỷ VND';
    if (v >= 1e6) return (v / 1e6).toFixed(1) + 'M VND';
    return v.toLocaleString('vi-VN') + ' VND';
  }
  if (v >= 1e6) return '$' + (v / 1e6).toFixed(2) + 'M';
  if (v >= 1e3) return '$' + (v / 1e3).toFixed(1) + 'K';
  return '$' + v.toFixed(2);
}

function fmtValueShort(v: number | null, currency: string): string {
  if (v == null) return '—';
  if (currency === 'VND') {
    if (v >= 1e9) return (v / 1e9).toFixed(1) + 'B';
    if (v >= 1e6) return (v / 1e6).toFixed(1) + 'M';
    if (v >= 1e3) return (v / 1e3).toFixed(1) + 'K';
    return v.toFixed(0);
  }
  if (v >= 1e6) return '$' + (v / 1e6).toFixed(1) + 'M';
  if (v >= 1e3) return '$' + (v / 1e3).toFixed(1) + 'K';
  return '$' + v.toFixed(0);
}

function algoLabel(algo: string): string {
  const map: Record<string, string> = {
    lstm_nn: 'LSTM',
    arima_garch: 'ARIMA',
    moving_average: 'MA',
    ema: 'EMA',
    ema_macd: 'EMA/MACD',
    ensemble: 'Ensemble',
    lightgbm: 'LightGBM',
    random_forest: 'Random Forest',
    xgboost: 'XGBoost',
    sarima: 'SARIMA',
    gru: 'GRU',
  };
  return map[(algo || '').toLowerCase()] || algo;
}

function algoClass(algo: string): string {
  const map: Record<string, string> = {
    lstm_nn: 'lstm',
    arima_garch: 'arima',
    moving_average: 'ma',
    ema: 'ema',
    ema_macd: 'ema',
    ensemble: 'ens',
    lightgbm: 'lgb',
  };
  return map[(algo || '').toLowerCase()] || 'unknown';
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

// ── Component ─────────────────────────────────────────────────────────────────

export default function SimulationBot() {
  const { botId } = useParams<{ botId: string }>();
  const { isLoggedIn } = useAuth();
  const navigate = useNavigate();

  const [bot, setBot] = useState<BotDetail | null>(null);
  const [chart, setChart] = useState<BotChart | null>(null);
  const [trades, setTrades] = useState<Trade[]>([]);
  const [tradePage, setTradePage] = useState(1);
  const [tradeTotal, setTradeTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [chartLoading, setChartLoading] = useState(false);
  const [tradesLoading, setTradesLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [activeChart, setActiveChart] = useState<'value' | 'return'>('value');

  const TRADE_LIMIT = 50;

  useEffect(() => {
    if (!botId) return;
    setLoading(true);
    setError(null);
    apiFetch(`/api/simulation/bots/${encodeURIComponent(botId)}`)
      .then((d: BotDetail) => {
        setBot(d);
        setLoading(false);
      })
      .catch((e) => {
        setError(String(e));
        setLoading(false);
      });
  }, [botId]);

  useEffect(() => {
    if (!botId) return;
    setChartLoading(true);
    apiFetch(`/api/simulation/bots/${encodeURIComponent(botId)}/chart`)
      .then((d: BotChart) => {
        setChart(d);
        setChartLoading(false);
      })
      .catch(() => {
        setChart(null);
        setChartLoading(false);
      });
  }, [botId]);

  useEffect(() => {
    if (!botId) return;
    setTradesLoading(true);
    apiFetch(`/api/simulation/bots/${encodeURIComponent(botId)}/trades?page=${tradePage}&limit=${TRADE_LIMIT}`)
      .then((d: TradesResponse) => {
        setTrades(Array.isArray(d.data) ? d.data : []);
        setTradeTotal(d.total || 0);
        setTradesLoading(false);
      })
      .catch(() => {
        setTrades([]);
        setTradesLoading(false);
      });
  }, [botId, tradePage]);

  if (loading) {
    return (
      <div className="content__inner fade">
        <div className="empty section-gap" style={{ padding: '80px 20px' }}>
          <div className="empty__icon"><Icon name="layers" size={18} /></div>
          <p>Đang tải chi tiết bot...</p>
        </div>
      </div>
    );
  }

  if (error || !bot) {
    return (
      <div className="content__inner fade">
        <div className="empty section-gap" style={{ padding: '80px 20px' }}>
          <div className="empty__icon"><Icon name="layers" size={18} /></div>
          <p style={{ color: 'var(--down)' }}>Không tìm thấy bot hoặc chưa có dữ liệu simulation.</p>
          {error && <p style={{ fontSize: 11, color: 'var(--text-3)', marginTop: 4 }}>{error}</p>}
          <button className="btn btn--sm" style={{ marginTop: 16 }} onClick={() => navigate('/simulation')}>
            ← Quay lại Leaderboard
          </button>
        </div>
      </div>
    );
  }

  const kpis = bot.kpis;
  const currency = bot.currency;

  // Chart data
  const n = chart?.dates.length || 0;
  const chartStep = Math.max(1, Math.ceil(n / 8));
  const chartLabels = (chart?.dates || []).map(ddmm);

  const totalPages = Math.ceil(tradeTotal / TRADE_LIMIT);

  return (
    <div className="content__inner fade">
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 20, flexWrap: 'wrap' }}>
        <button
          className="btn btn--sm btn--ghost"
          onClick={() => navigate('/simulation')}
          style={{ display: 'flex', alignItems: 'center', gap: 6 }}
        >
          <Icon name="arrowUp" size={13} style={{ transform: 'rotate(-90deg)' }} />
          Leaderboard
        </button>
        <h1 style={{ fontSize: 22, fontWeight: 700, letterSpacing: '-0.5px', flex: 1 }}>
          {bot.display_name}
        </h1>
        <span className="badge badge--muted" style={{ fontSize: 11, padding: '3px 8px', letterSpacing: 0.5 }}>
          {bot.market}
        </span>
        <span className={`algo algo--${algoClass(bot.algorithm)}`}>
          {algoLabel(bot.algorithm)}
        </span>
        {isLoggedIn ? (
          <button
            className="btn btn--sm"
            style={{
              fontSize: 11,
              padding: '3px 10px',
              background: bot.is_active ? 'var(--up-bg)' : 'var(--surface-2)',
              color: bot.is_active ? 'var(--up)' : 'var(--text-3)',
              border: '1px solid ' + (bot.is_active ? 'var(--up)' : 'var(--border)'),
              fontFamily: 'var(--font-mono)',
              cursor: 'pointer',
            }}
            title={bot.is_active ? 'Click to disable bot' : 'Click to enable bot'}
            onClick={() => {
              authPost(`/api/simulation/bots/${botId}/toggle`).then((ok) => {
                if (ok) {
                  setBot({ ...bot, is_active: !bot.is_active });
                  vnsToast(!bot.is_active ? 'Bot activated' : 'Bot disabled');
                } else {
                  vnsToast('Failed to update bot (admin required)');
                }
              });
            }}
          >
            {bot.is_active ? 'ACTIVE' : 'INACTIVE'}
          </button>
        ) : (
          <span
            style={{
              fontSize: 11,
              padding: '3px 8px',
              background: bot.is_active ? 'var(--up-bg)' : 'var(--surface-2)',
              color: bot.is_active ? 'var(--up)' : 'var(--text-3)',
              border: '1px solid ' + (bot.is_active ? 'var(--up)' : 'var(--border)'),
              fontFamily: 'var(--font-mono)',
            }}
          >
            {bot.is_active ? 'ACTIVE' : 'INACTIVE'}
          </span>
        )}
        {isLoggedIn && (
          <button
            className="btn btn--sm"
            style={{ background: 'var(--accent)', borderColor: 'var(--accent)', color: '#fff' }}
            onClick={() =>
              authPost(`/api/simulation/bots/${botId}/run`).then((ok) =>
                vnsToast(ok ? 'Đã gửi yêu cầu chạy backtest cho bot này' : 'Không thể gửi yêu cầu')
              )
            }
          >
            <Icon name="play" size={13} />Run Backtest
          </button>
        )}
      </div>

      {/* Session info */}
      {bot.last_session && (
        <div style={{ marginBottom: 16, fontSize: 12, color: 'var(--text-3)', fontFamily: 'var(--font-mono)', display: 'flex', gap: 12, flexWrap: 'wrap' }}>
          <span>
            <Icon name="clock" size={12} style={{ display: 'inline', verticalAlign: 'middle', marginRight: 4 }} />
            Simulated from {fmtDT(bot.last_session.start_date)} to {fmtDT(bot.last_session.end_date)}
          </span>
          <span style={{ color: 'var(--border-strong)' }}>|</span>
          <span>
            Session #{bot.last_session.id} · {bot.last_session.status.toUpperCase()}
          </span>
          <span style={{ color: 'var(--border-strong)' }}>|</span>
          <span>
            Initial: {fmtCapital(bot.initial_capital, currency)}
          </span>
        </div>
      )}

      {/* KPI cards */}
      <div className="grid grid--kpis section-gap">
        <KPI
          label="Total Return"
          value={(kpis?.total_return_pct ?? 0).toFixed(2) + '%'}
          chgPct={kpis?.total_return_pct ?? 0}
          accent={(kpis?.total_return_pct ?? 0) > 0}
        />
        <KPI
          label="Sharpe Ratio"
          value={(kpis?.sharpe_ratio ?? 0).toFixed(2)}
          sub={(kpis?.sharpe_ratio ?? 0) >= 1.5 ? 'Xuất sắc' : (kpis?.sharpe_ratio ?? 0) >= 1 ? 'Tốt' : 'Thấp'}
        />
        <KPI
          label="Win Rate"
          value={(kpis?.win_rate_pct ?? 0).toFixed(1) + '%'}
          sub={String(kpis?.total_trades ?? 0) + ' giao dịch'}
        />
        <KPI
          label="Max Drawdown"
          value={(kpis?.max_drawdown_pct ?? 0).toFixed(1) + '%'}
          sub="rủi ro tối đa"
        />
      </div>

      {/* Extra stats row */}
      <div className="grid section-gap" style={{ gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))', gap: 'var(--gap)' }}>
        <StatCard label="Annualized Return" value={((kpis?.annualized_return_pct ?? 0) >= 0 ? '+' : '') + (kpis?.annualized_return_pct ?? 0).toFixed(1) + '%'} color={(kpis?.annualized_return_pct ?? 0) >= 0 ? 'var(--up)' : 'var(--down)'} />
        <StatCard label="Profit Factor" value={(kpis?.profit_factor ?? 0).toFixed(2)} color="var(--text)" />
        <StatCard label="Total Trades" value={String(kpis?.total_trades ?? 0)} color="var(--text)" />
        <StatCard label="Best Trade" value={'+' + (kpis?.best_trade_pct ?? 0).toFixed(1) + '%'} color="var(--up)" />
        <StatCard label="Worst Trade" value={(kpis?.worst_trade_pct ?? 0).toFixed(1) + '%'} color="var(--down)" />
        <StatCard label="Min Confidence" value={((bot.min_confidence ?? 0) * 100).toFixed(0) + '%'} color="var(--text-2)" />
      </div>

      {/* Portfolio chart */}
      <Panel
        title="Biểu đồ danh mục"
        className="section-gap"
        tools={
          <div className="seg">
            <button className={activeChart === 'value' ? 'active' : ''} onClick={() => setActiveChart('value')}>Giá trị</button>
            <button className={activeChart === 'return' ? 'active' : ''} onClick={() => setActiveChart('return')}>Return %</button>
          </div>
        }
      >
        {chartLoading ? (
          <div className="empty" style={{ height: 540, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
            <div className="empty__icon"><Icon name="refresh" size={18} /></div>
            <p>Đang tải biểu đồ...</p>
          </div>
        ) : chart && chart.dates.length > 1 ? (
          <>
            {activeChart === 'value' ? (
              <LineChart
                series={[{
                  name: 'Portfolio Value',
                  data: chart.values,
                  color: 'var(--accent)',
                }]}
                labels={chartLabels}
                height={540}
                area
                yFmt={(v) => fmtValueShort(v, currency)}
                valueFmt={(v) => fmtCapital(v, currency)}
                padL={68}
              />
            ) : (
              <LineChart
                series={[{
                  name: 'Return %',
                  data: chart.returns_pct,
                  color: (chart.returns_pct[chart.returns_pct.length - 1] ?? 0) >= 0 ? 'var(--up)' : 'var(--down)',
                }]}
                labels={chartLabels}
                height={540}
                area
                yFmt={(v) => (v ?? 0).toFixed(1) + '%'}
                valueFmt={(v) => (v ?? 0).toFixed(2) + '%'}
                padL={52}
              />
            )}
          </>
        ) : (
          <div className="empty" style={{ height: 540, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
            <div className="empty__icon"><Icon name="layers" size={18} /></div>
            <p>{chart && chart.dates.length === 1
              ? `Chỉ có 1 ngày dữ liệu (${chart.dates[0]}). Cần ít nhất 2 ngày để vẽ biểu đồ.`
              : 'Chưa có dữ liệu biểu đồ. Hãy chạy backtest trước.'
            }</p>
          </div>
        )}
      </Panel>

      {/* Bot config panel */}
      <Panel title="Cấu hình bot" sub="chiến lược giao dịch" className="section-gap">
        <div style={{ display: 'flex', gap: 32, flexWrap: 'wrap', fontSize: 13 }}>
          <ConfigRow label="Buy threshold" value={(bot.buy_threshold ?? 0).toFixed(1) + '%'} />
          <ConfigRow label="Sell threshold" value={(bot.sell_threshold ?? 0).toFixed(1) + '%'} />
          <ConfigRow label="Min confidence" value={((bot.min_confidence ?? 0) * 100).toFixed(0) + '%'} />
          <ConfigRow label="Stop loss" value={(bot.stop_loss ?? 0).toFixed(1) + '%'} />
          <ConfigRow label="Take profit" value={(bot.take_profit ?? 0).toFixed(1) + '%'} />
          <ConfigRow label="Currency" value={currency} />
        </div>
      </Panel>

      {/* Trades table */}
      <div className="sec-head section-gap"><h2>Lịch sử giao dịch</h2><div className="line"></div></div>
      <Panel flush className="section-gap">
        {tradesLoading ? (
          <div className="empty" style={{ padding: 32 }}>
            <div className="empty__icon"><Icon name="refresh" size={18} /></div>
            <p>Đang tải giao dịch...</p>
          </div>
        ) : trades.length === 0 ? (
          <div className="empty" style={{ padding: 48 }}>
            <div className="empty__icon"><Icon name="layers" size={18} /></div>
            <p>Chưa có giao dịch nào. Hãy chạy backtest trước.</p>
          </div>
        ) : (
          <>
            <div style={{ overflowX: 'auto' }}>
              <table className="tbl">
                <thead>
                  <tr>
                    <th>Mã</th>
                    <th className="c">Loại</th>
                    <th>Ngày</th>
                    <th className="r">Giá</th>
                    <th className="r">Số lượng</th>
                    <th className="r">Giá trị</th>
                    <th className="r">Tín hiệu</th>
                    <th className="r">Conf.</th>
                    <th className="r">P&L</th>
                    <th className="r">P&L %</th>
                    <th>Lý do đóng</th>
                  </tr>
                </thead>
                <tbody>
                  {trades.map((t) => (
                    <tr key={t.id}>
                      <td className="sym">{t.symbol}</td>
                      <td className="c">
                        <span
                          style={{
                            padding: '2px 8px',
                            fontSize: 11,
                            fontWeight: 700,
                            letterSpacing: 0.5,
                            background: t.action === 'BUY' ? 'var(--up-bg)' : 'var(--down-bg)',
                            color: t.action === 'BUY' ? 'var(--up)' : 'var(--down)',
                            border: '1px solid ' + (t.action === 'BUY' ? 'var(--up)' : 'var(--down)'),
                          }}
                        >
                          {t.action}
                        </span>
                      </td>
                      <td className="num" style={{ color: 'var(--text-3)', fontSize: 12 }}>{fmtDT(t.trade_date)}</td>
                      <td className="r num" style={{ fontSize: 12 }}>
                        {t.price != null
                          ? (currency === 'VND'
                              ? t.price.toLocaleString('vi-VN')
                              : '$' + t.price.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 }))
                          : '—'}
                      </td>
                      <td className="r num" style={{ color: 'var(--text-2)', fontSize: 12 }}>
                        {t.quantity != null ? t.quantity.toLocaleString('en-US', { maximumFractionDigits: 2 }) : '—'}
                      </td>
                      <td className="r num" style={{ fontSize: 12 }}>
                        {fmtValueShort(t.trade_value, currency)}
                      </td>
                      <td className="r num" style={{ fontSize: 12, color: 'var(--text-2)' }}>
                        {t.signal_strength != null ? t.signal_strength.toFixed(1) : '—'}
                      </td>
                      <td className="r num" style={{ fontSize: 12 }}>
                        {t.confidence != null ? (t.confidence * 100).toFixed(0) + '%' : '—'}
                      </td>
                      <td className="r">
                        {t.pnl != null ? (
                          <span className="num" style={{ fontSize: 12, color: t.pnl >= 0 ? 'var(--up)' : 'var(--down)' }}>
                            {t.pnl >= 0 ? '+' : ''}{fmtValueShort(t.pnl, currency)}
                          </span>
                        ) : (
                          <span style={{ color: 'var(--text-3)', fontSize: 12 }}>—</span>
                        )}
                      </td>
                      <td className="r">
                        {t.pnl_pct != null ? (
                          <Chg pct={t.pnl_pct} />
                        ) : (
                          <span style={{ color: 'var(--text-3)', fontSize: 12 }}>—</span>
                        )}
                      </td>
                      <td style={{ fontSize: 11, color: 'var(--text-3)' }}>
                        {t.close_reason || '—'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            {/* Pagination */}
            {totalPages > 1 && (
              <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '12px 16px', borderTop: '1px solid var(--border)', fontSize: 12 }}>
                <span style={{ color: 'var(--text-3)' }}>
                  Trang {tradePage}/{totalPages} · {tradeTotal} giao dịch
                </span>
                <div style={{ marginLeft: 'auto', display: 'flex', gap: 6 }}>
                  <button
                    className="btn btn--sm btn--ghost"
                    disabled={tradePage <= 1}
                    onClick={() => setTradePage(tradePage - 1)}
                  >
                    ← Trước
                  </button>
                  <button
                    className="btn btn--sm btn--ghost"
                    disabled={tradePage >= totalPages}
                    onClick={() => setTradePage(tradePage + 1)}
                  >
                    Tiếp →
                  </button>
                </div>
              </div>
            )}
          </>
        )}
      </Panel>
    </div>
  );
}

// ── Sub-components ────────────────────────────────────────────────────────────

function StatCard({ label, value, color }: { label: string; value: string; color: string }) {
  return (
    <div style={{
      background: 'var(--surface)',
      border: '1px solid var(--border)',
      padding: '12px 14px',
    }}>
      <div style={{ fontSize: 10, color: 'var(--text-3)', textTransform: 'uppercase', letterSpacing: 0.6, fontWeight: 600, marginBottom: 6 }}>
        {label}
      </div>
      <div style={{ fontFamily: 'var(--font-mono)', fontSize: 20, fontWeight: 600, color }}>
        {value}
      </div>
    </div>
  );
}

function ConfigRow({ label, value }: { label: string; value: string }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <span style={{ fontSize: 10, color: 'var(--text-3)', textTransform: 'uppercase', letterSpacing: 0.6, fontWeight: 600 }}>{label}</span>
      <span style={{ fontFamily: 'var(--font-mono)', fontSize: 14, fontWeight: 600 }}>{value}</span>
    </div>
  );
}
