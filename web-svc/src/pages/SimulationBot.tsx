import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Panel, KPI, Icon, Chg, vnsToast } from '../components/ui';
import { LineChart } from '../components/charts';
import { useAuth } from '../context/AuthContext';
import { useLanguage } from '../context/LangContext';

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
  action: 'BUY' | 'SELL' | 'HOLD';
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

interface VariantKPIs {
  total_return_pct: number;
  sharpe_ratio: number;
  win_rate_pct: number;
  max_drawdown_pct: number;
  profit_factor: number;
  total_trades: number;
}

interface VariantBot {
  id: string;
  display_name: string;
  buy_threshold: number | null;
  sell_threshold: number | null;
  min_confidence: number | null;
  stop_loss: number | null;
  take_profit: number | null;
  kpis: VariantKPIs;
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
  if (v >= 1e4) return '$' + (v / 1e3).toFixed(1) + 'K';
  return '$' + v.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
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
  if (v >= 1e4) return '$' + (v / 1e3).toFixed(1) + 'K';
  return '$' + v.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
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
    gru_nn: 'GRU',
    egarch: 'EGARCH',
    rl_dqn: 'RL DQN',
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
    gru_nn: 'lstm',
    egarch: 'arima',
    rl_dqn: 'rl',
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

// ── Variants components ───────────────────────────────────────────────────────

function SortIcon({ active, dir }: { active: boolean; dir: 'asc' | 'desc' }) {
  if (!active) return <span style={{ color: 'var(--text-3)', fontSize: 10 }}> ⇅</span>;
  return <span style={{ color: 'var(--accent, #58a6ff)', fontSize: 10 }}>{dir === 'desc' ? ' ↓' : ' ↑'}</span>;
}

function VariantsTab({
  variants, currentBotId, sort, onSort, currency, onNavigate,
}: {
  variants: VariantBot[];
  currentBotId: string;
  sort: { col: string; dir: 'asc' | 'desc' };
  onSort: (col: string) => void;
  currency: string;
  onNavigate: (id: string) => void;
}) {
  const cols: { key: string; label: string; right?: boolean }[] = [
    { key: 'name', label: 'Variant' },
    { key: 'buy', label: 'Buy', right: true },
    { key: 'sell', label: 'Sell', right: true },
    { key: 'conf', label: 'Conf', right: true },
    { key: 'sl', label: 'SL', right: true },
    { key: 'tp', label: 'TP', right: true },
    { key: 'return', label: 'Return%', right: true },
    { key: 'sharpe', label: 'Sharpe', right: true },
    { key: 'win', label: 'Win%', right: true },
    { key: 'trades', label: 'Trades', right: true },
  ];

  const sorted = [...variants].sort((a, b) => {
    const dir = sort.dir === 'desc' ? -1 : 1;
    switch (sort.col) {
      case 'buy':    return ((a.buy_threshold ?? 0) - (b.buy_threshold ?? 0)) * dir;
      case 'sell':   return ((a.sell_threshold ?? 0) - (b.sell_threshold ?? 0)) * dir;
      case 'conf':   return ((a.min_confidence ?? 0) - (b.min_confidence ?? 0)) * dir;
      case 'sl':     return ((a.stop_loss ?? 0) - (b.stop_loss ?? 0)) * dir;
      case 'tp':     return ((a.take_profit ?? 0) - (b.take_profit ?? 0)) * dir;
      case 'return': return (a.kpis.total_return_pct - b.kpis.total_return_pct) * dir;
      case 'sharpe': return (a.kpis.sharpe_ratio - b.kpis.sharpe_ratio) * dir;
      case 'win':    return (a.kpis.win_rate_pct - b.kpis.win_rate_pct) * dir;
      case 'trades': return (a.kpis.total_trades - b.kpis.total_trades) * dir;
      default:       return a.display_name.localeCompare(b.display_name) * dir;
    }
  });

  if (variants.length === 0) {
    return (
      <div className="empty section-gap" style={{ padding: 48 }}>
        <p style={{ color: 'var(--text-3)' }}>Chưa có variant nào.</p>
      </div>
    );
  }

  // currency is available for future use (e.g. showing currency symbol in columns)
  void currency;

  return (
    <div className="section-gap">
      <div className="tbl-scroll" style={{ overflowX: 'auto' }}>
        <table className="tbl">
          <thead>
            <tr>
              {cols.map(c => (
                <th
                  key={c.key}
                  className={c.right ? 'r' : ''}
                  style={{ cursor: 'pointer', userSelect: 'none', whiteSpace: 'nowrap' }}
                  onClick={() => onSort(c.key)}
                >
                  {c.label}
                  <SortIcon active={sort.col === c.key} dir={sort.dir} />
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {sorted.map(v => {
              const isCurrent = v.id === currentBotId;
              return (
                <tr
                  key={v.id}
                  style={{ background: isCurrent ? 'var(--surface-2)' : undefined, cursor: 'pointer' }}
                  onClick={() => !isCurrent && onNavigate(v.id)}
                >
                  <td>
                    <span
                      style={{
                        color: isCurrent ? 'var(--accent, #58a6ff)' : 'var(--text-1)',
                        fontWeight: isCurrent ? 700 : 400,
                        fontSize: 13,
                      }}
                    >
                      {v.display_name.split('—').pop()?.trim() || v.display_name}
                      {isCurrent && <span style={{ color: 'var(--text-3)', fontWeight: 400, marginLeft: 6 }}>← đây</span>}
                    </span>
                  </td>
                  <td className="r num" style={{ fontSize: 12 }}>{v.buy_threshold != null ? v.buy_threshold.toFixed(1) + '%' : '—'}</td>
                  <td className="r num" style={{ fontSize: 12 }}>{v.sell_threshold != null ? v.sell_threshold.toFixed(1) + '%' : '—'}</td>
                  <td className="r num" style={{ fontSize: 12 }}>{v.min_confidence != null ? (v.min_confidence * 100).toFixed(0) + '%' : '—'}</td>
                  <td className="r num" style={{ fontSize: 12 }}>{v.stop_loss != null ? v.stop_loss.toFixed(1) + '%' : '—'}</td>
                  <td className="r num" style={{ fontSize: 12 }}>{v.take_profit != null ? v.take_profit.toFixed(1) + '%' : '—'}</td>
                  <td className="r">
                    <span className="num" style={{ fontSize: 12, color: v.kpis.total_return_pct >= 0 ? 'var(--up)' : 'var(--down)' }}>
                      {v.kpis.total_return_pct >= 0 ? '+' : ''}{v.kpis.total_return_pct.toFixed(2)}%
                    </span>
                  </td>
                  <td className="r num" style={{ fontSize: 12 }}>{v.kpis.sharpe_ratio.toFixed(2)}</td>
                  <td className="r num" style={{ fontSize: 12 }}>{v.kpis.win_rate_pct.toFixed(1)}%</td>
                  <td className="r num" style={{ fontSize: 12 }}>{v.kpis.total_trades}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {/* Mobile card view for variants */}
      <div className="m-cards">
        {sorted.map(v => {
          const isCurrent = v.id === currentBotId;
          return (
            <div
              key={v.id}
              className={isCurrent ? 'm-card' : 'm-card clickable'}
              style={isCurrent ? { background: 'var(--surface-2)' } : undefined}
              onClick={() => !isCurrent && onNavigate(v.id)}
            >
              <div className="m-card__head">
                <div className="m-card__title">
                  <div className="m-card__name" style={{ color: isCurrent ? 'var(--accent, #58a6ff)' : undefined, fontWeight: isCurrent ? 700 : 600 }}>
                    {v.display_name.split('—').pop()?.trim() || v.display_name}
                    {isCurrent && <span style={{ color: 'var(--text-3)', fontWeight: 400, marginLeft: 6 }}>← đây</span>}
                  </div>
                </div>
                <span className="num" style={{ fontFamily: 'var(--font-mono)', fontSize: 13, fontWeight: 600, color: v.kpis.total_return_pct >= 0 ? 'var(--up)' : 'var(--down)' }}>
                  {v.kpis.total_return_pct >= 0 ? '+' : ''}{v.kpis.total_return_pct.toFixed(2)}%
                </span>
              </div>
              <div className="m-card__metrics">
                <Metric label="Buy" value={v.buy_threshold != null ? v.buy_threshold.toFixed(1) + '%' : '—'} />
                <Metric label="Sell" value={v.sell_threshold != null ? v.sell_threshold.toFixed(1) + '%' : '—'} />
                <Metric label="Conf" value={v.min_confidence != null ? (v.min_confidence * 100).toFixed(0) + '%' : '—'} />
                <Metric label="SL" value={v.stop_loss != null ? v.stop_loss.toFixed(1) + '%' : '—'} />
                <Metric label="TP" value={v.take_profit != null ? v.take_profit.toFixed(1) + '%' : '—'} />
                <Metric label="Sharpe" value={v.kpis.sharpe_ratio.toFixed(2)} />
                <Metric label="Win%" value={v.kpis.win_rate_pct.toFixed(1) + '%'} />
                <Metric label="Trades" value={String(v.kpis.total_trades)} />
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

// ── Mobile metric cell ─────────────────────────────────────────────────────────

function Metric({ label, value, color }: { label: string; value: string; color?: string }) {
  return (
    <div className="m-metric">
      <span className="m-metric__label">{label}</span>
      <span className="m-metric__value" style={color ? { color } : undefined}>{value}</span>
    </div>
  );
}

// ── Component ─────────────────────────────────────────────────────────────────

export default function SimulationBot() {
  const { botId } = useParams<{ botId: string }>();
  const { isLoggedIn } = useAuth();
  const navigate = useNavigate();
  const { t } = useLanguage();

  const [bot, setBot] = useState<BotDetail | null>(null);
  const [chart, setChart] = useState<BotChart | null>(null);
  const [liveChart, setLiveChart] = useState<BotChart | null>(null);
  const [trades, setTrades] = useState<Trade[]>([]);
  const [tradePage, setTradePage] = useState(1);
  const [tradeTotal, setTradeTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [chartLoading, setChartLoading] = useState(false);
  const [tradesLoading, setTradesLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [activeChart, setActiveChart] = useState<'value' | 'return'>('value');
  const [chartSession, setChartSession] = useState<'backtest' | 'live'>('backtest');
  const [tradeSession, setTradeSession] = useState<'backtest' | 'live'>('backtest');
  const [hideHold, setHideHold] = useState(true);
  const [activeTab, setActiveTab] = useState<'detail' | 'variants'>('detail');
  const [variants, setVariants] = useState<VariantBot[]>([]);
  const [variantSort, setVariantSort] = useState<{ col: string; dir: 'asc' | 'desc' }>({ col: 'return', dir: 'desc' });

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
    apiFetch(`/api/simulation/bots/${encodeURIComponent(botId)}/chart?mode=live`)
      .then((d: BotChart) => setLiveChart(d))
      .catch(() => setLiveChart(null));
  }, [botId]);

  useEffect(() => {
    if (!botId) return;
    const sessionId = tradeSession === 'backtest' ? chart?.session_id : liveChart?.session_id;
    if (sessionId == null) {
      setTrades([]);
      setTradeTotal(0);
      return;
    }
    setTradesLoading(true);
    apiFetch(
      `/api/simulation/bots/${encodeURIComponent(botId)}/trades?page=${tradePage}&limit=${TRADE_LIMIT}&session_id=${sessionId}&exclude_hold=${hideHold}`
    )
      .then((d: TradesResponse) => {
        setTrades(Array.isArray(d.data) ? d.data : []);
        setTradeTotal(d.total || 0);
        setTradesLoading(false);
      })
      .catch(() => {
        setTrades([]);
        setTradesLoading(false);
      });
  }, [botId, tradePage, chart, liveChart, tradeSession, hideHold]);

  useEffect(() => {
    if (!botId) return;
    apiFetch(`/api/simulation/bots/${encodeURIComponent(botId)}/variants`)
      .then((data: VariantBot[]) => setVariants(Array.isArray(data) ? data : []))
      .catch(() => setVariants([]));
  }, [botId]);

  if (loading) {
    return (
      <div className="content__inner fade">
        <div className="empty section-gap" style={{ padding: '80px 20px' }}>
          <div className="empty__icon"><Icon name="layers" size={18} /></div>
          <p>{t.simulationBot.loadingBot}</p>
        </div>
      </div>
    );
  }

  if (error || !bot) {
    return (
      <div className="content__inner fade">
        <div className="empty section-gap" style={{ padding: '80px 20px' }}>
          <div className="empty__icon"><Icon name="layers" size={18} /></div>
          <p style={{ color: 'var(--down)' }}>{t.simulationBot.botNotFound}</p>
          {error && <p style={{ fontSize: 11, color: 'var(--text-3)', marginTop: 4 }}>{error}</p>}
          <button className="btn btn--sm" style={{ marginTop: 16 }} onClick={() => navigate('/simulation')}>
            {t.simulationBot.backToLeaderboard}
          </button>
        </div>
      </div>
    );
  }

  const kpis = bot.kpis;
  const currency = bot.currency;

  // Active chart data based on session selector
  const activeChartData = chartSession === 'live' ? liveChart : chart;

  // Chart data
  const n = activeChartData?.dates.length || 0;
  const chartStep = Math.max(1, Math.ceil(n / 8));
  const chartLabels = (activeChartData?.dates || []).map(ddmm);

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
          {t.simulationBot.backToLeaderboard}
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
                vnsToast(ok ? t.simulationBot.runBacktestRequest : t.simulationBot.cannotSendRequest)
              )
            }
          >
            <Icon name="play" size={13} />Run Backtest
          </button>
        )}
      </div>

      {/* Tab bar */}
      <div style={{ display: 'flex', gap: 0, borderBottom: '1px solid var(--border)', marginBottom: 20 }}>
        <button
          onClick={() => setActiveTab('detail')}
          style={{
            padding: '8px 18px', fontSize: 13, fontWeight: activeTab === 'detail' ? 700 : 400,
            color: activeTab === 'detail' ? 'var(--text-1)' : 'var(--text-3)',
            background: 'none', border: 'none',
            borderBottom: activeTab === 'detail' ? '2px solid var(--accent, #58a6ff)' : '2px solid transparent',
            cursor: 'pointer',
          }}
        >
          Chi tiết
        </button>
        <button
          onClick={() => setActiveTab('variants')}
          style={{
            padding: '8px 18px', fontSize: 13, fontWeight: activeTab === 'variants' ? 700 : 400,
            color: activeTab === 'variants' ? 'var(--text-1)' : 'var(--text-3)',
            background: 'none', border: 'none',
            borderBottom: activeTab === 'variants' ? '2px solid var(--accent, #58a6ff)' : '2px solid transparent',
            cursor: 'pointer',
          }}
        >
          Variants ({variants.length})
        </button>
      </div>

      {activeTab === 'detail' && (
      <>

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

      {/* Live session status banner */}
      {liveChart && liveChart.session_id != null && (
        <div style={{
          marginBottom: 16,
          padding: '8px 14px',
          background: 'var(--surface)',
          border: '1px solid var(--border)',
          fontSize: 12,
          color: 'var(--text-2)',
          display: 'flex',
          alignItems: 'center',
          gap: 12,
          flexWrap: 'wrap',
        }}>
          <span style={{
            width: 8, height: 8, borderRadius: '50%',
            background: 'var(--up)', display: 'inline-block',
            boxShadow: '0 0 6px var(--up)',
            flexShrink: 0,
          }} />
          <span>Live session #{liveChart.session_id}</span>
          {liveChart.dates.length > 0 && (
            <>
              <span style={{ color: 'var(--border-strong)' }}>|</span>
              <span>
                Portfolio hôm nay:{' '}
                <span style={{ fontFamily: 'var(--font-mono)', fontWeight: 600, color: 'var(--text)' }}>
                  {fmtCapital(liveChart.values[liveChart.values.length - 1] ?? null, currency)}
                </span>
              </span>
              {liveChart.returns_pct.length > 0 && (
                <>
                  <span style={{ color: 'var(--border-strong)' }}>|</span>
                  <Chg pct={liveChart.returns_pct[liveChart.returns_pct.length - 1] ?? 0} />
                </>
              )}
            </>
          )}
          {liveChart.dates.length === 0 && (
            <>
              <span style={{ color: 'var(--border-strong)' }}>|</span>
              <span style={{ color: 'var(--text-3)' }}>Chưa có dữ liệu hôm nay</span>
            </>
          )}
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
          sub={(kpis?.sharpe_ratio ?? 0) >= 1.5 ? t.simulationBot.sharpeExcellent : (kpis?.sharpe_ratio ?? 0) >= 1 ? t.simulationBot.sharpeGood : t.simulationBot.sharpeLow}
        />
        <KPI
          label="Win Rate"
          value={(kpis?.win_rate_pct ?? 0).toFixed(1) + '%'}
          sub={String(kpis?.total_trades ?? 0) + ' ' + t.simulationBot.transactions}
        />
        <KPI
          label="Max Drawdown"
          value={(kpis?.max_drawdown_pct ?? 0).toFixed(1) + '%'}
          sub={t.simulationBot.maxRisk}
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
        title={t.simulationBot.portfolioChart}
        className="section-gap"
        tools={
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
            <div className="seg">
              <button className={activeChart === 'value' ? 'active' : ''} onClick={() => setActiveChart('value')}>{t.simulationBot.chartValue}</button>
              <button className={activeChart === 'return' ? 'active' : ''} onClick={() => setActiveChart('return')}>{t.simulationBot.chartReturn}</button>
            </div>
            <div className="seg" style={{ marginLeft: 8 }}>
              <button className={chartSession === 'backtest' ? 'active' : ''} onClick={() => setChartSession('backtest')}>Backtest</button>
              <button className={chartSession === 'live' ? 'active' : ''} onClick={() => setChartSession('live')}>Live</button>
            </div>
          </div>
        }
      >
        {chartLoading ? (
          <div className="empty" style={{ height: 540, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
            <div className="empty__icon"><Icon name="refresh" size={18} /></div>
            <p>{t.simulationBot.loadingChart}</p>
          </div>
        ) : chartSession === 'live' ? (
          // Live chart view
          activeChartData && activeChartData.dates.length > 1 ? (
            <>
              {activeChart === 'value' ? (
                <LineChart
                  series={[{
                    name: 'Portfolio Value (Live)',
                    data: activeChartData.values,
                    color: 'var(--up)',
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
                    name: 'Return % (Live)',
                    data: activeChartData.returns_pct,
                    color: (activeChartData.returns_pct[activeChartData.returns_pct.length - 1] ?? 0) >= 0 ? 'var(--up)' : 'var(--down)',
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
              <p style={{ textAlign: 'center', lineHeight: 1.6 }}>
                {liveChart
                  ? liveChart.dates.length === 0
                    ? 'Live session chưa có dữ liệu giao dịch.'
                    : `Live session bắt đầu ${liveChart.dates[0]}. Biểu đồ sẽ hiển thị sau khi có 2+ ngày giao dịch.`
                  : 'Không có dữ liệu live session.'}
              </p>
            </div>
          )
        ) : (
          // Backtest chart view
          activeChartData && activeChartData.dates.length > 1 ? (
            <>
              {activeChart === 'value' ? (
                <LineChart
                  series={[{
                    name: 'Portfolio Value',
                    data: activeChartData.values,
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
                    data: activeChartData.returns_pct,
                    color: (activeChartData.returns_pct[activeChartData.returns_pct.length - 1] ?? 0) >= 0 ? 'var(--up)' : 'var(--down)',
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
                ? `${t.simulationBot.onlyOneDay} (${chart.dates[0]}). ${t.simulationBot.needMinTwoDays}`
                : t.simulationBot.noChartData
              }</p>
            </div>
          )
        )}
      </Panel>

      {/* Bot config panel */}
      <Panel title={t.simulationBot.botConfig} sub={t.simulationBot.tradingStrategy} className="section-gap">
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
      <div className="sec-head section-gap" style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
        <h2>{t.simulationBot.tradeHistory}</h2>
        <div className="line" style={{ flex: 1 }} />
        <button
          className="btn btn--sm btn--ghost"
          style={{
            fontSize: 11,
            padding: '3px 10px',
            background: hideHold ? 'var(--accent-bg, var(--surface-2))' : 'transparent',
            color: hideHold ? 'var(--accent, var(--text-1))' : 'var(--text-3)',
            border: '1px solid ' + (hideHold ? 'var(--accent, var(--border))' : 'var(--border)'),
          }}
          onClick={() => { setHideHold(h => !h); setTradePage(1); }}
          title="Ẩn tín hiệu HOLD (không phát sinh giao dịch)"
        >
          {hideHold ? 'Ẩn HOLD' : 'Hiện HOLD'}
        </button>
        <div className="seg">
          <button
            className={tradeSession === 'backtest' ? 'active' : ''}
            onClick={() => { setTradeSession('backtest'); setTradePage(1); }}
          >
            Backtest
          </button>
          <button
            className={tradeSession === 'live' ? 'active' : ''}
            onClick={() => { setTradeSession('live'); setTradePage(1); }}
          >
            Live
          </button>
        </div>
      </div>
      <Panel flush className="section-gap">
        {tradesLoading ? (
          <div className="empty" style={{ padding: 32 }}>
            <div className="empty__icon"><Icon name="refresh" size={18} /></div>
            <p>{t.simulationBot.loadingTrades}</p>
          </div>
        ) : trades.length === 0 ? (
          <div className="empty" style={{ padding: 48 }}>
            <div className="empty__icon"><Icon name="layers" size={18} /></div>
            <p>
              {tradeSession === 'live' && liveChart
                ? 'Live session chưa có giao dịch nào hôm nay.'
                : tradeSession === 'live' && !liveChart
                  ? 'Không có live session.'
                  : t.simulationBot.noTrades}
            </p>
          </div>
        ) : (
          <>
            <div className="tbl-scroll" style={{ overflowX: 'auto' }}>
              <table className="tbl">
                <thead>
                  <tr>
                    <th>{t.simulationBot.colSymbol}</th>
                    <th className="c">{t.simulationBot.colType}</th>
                    <th>{t.simulationBot.colDate}</th>
                    <th className="r">{t.simulationBot.colPrice}</th>
                    <th className="r">{t.simulationBot.colQuantity}</th>
                    <th className="r">{t.simulationBot.colValue}</th>
                    <th className="r">{t.simulationBot.colSignal}</th>
                    <th className="r">Conf.</th>
                    <th className="r">P&L</th>
                    <th className="r">P&L %</th>
                    <th>{t.simulationBot.colCloseReason}</th>
                  </tr>
                </thead>
                <tbody>
                  {trades.filter(tr => !hideHold || tr.action !== 'HOLD').map((t) => (
                    <tr key={t.id}>
                      <td className="sym">{t.symbol}</td>
                      <td className="c">
                        <span
                          style={{
                            padding: '2px 8px',
                            fontSize: 11,
                            fontWeight: 700,
                            letterSpacing: 0.5,
                            background: t.action === 'BUY' ? 'var(--up-bg)' : t.action === 'SELL' ? 'var(--down-bg)' : 'var(--surface-2)',
                            color: t.action === 'BUY' ? 'var(--up)' : t.action === 'SELL' ? 'var(--down)' : 'var(--text-3)',
                            border: '1px solid ' + (t.action === 'BUY' ? 'var(--up)' : t.action === 'SELL' ? 'var(--down)' : 'var(--border)'),
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

            {/* Mobile card view for trades */}
            <div className="m-cards">
              {trades.filter(tr => !hideHold || tr.action !== 'HOLD').map((tr) => (
                <div key={tr.id} className="m-card">
                  <div className="m-card__head">
                    <span
                      style={{
                        padding: '2px 8px',
                        fontSize: 11,
                        fontWeight: 700,
                        letterSpacing: 0.5,
                        flexShrink: 0,
                        background: tr.action === 'BUY' ? 'var(--up-bg)' : tr.action === 'SELL' ? 'var(--down-bg)' : 'var(--surface-2)',
                        color: tr.action === 'BUY' ? 'var(--up)' : tr.action === 'SELL' ? 'var(--down)' : 'var(--text-3)',
                        border: '1px solid ' + (tr.action === 'BUY' ? 'var(--up)' : tr.action === 'SELL' ? 'var(--down)' : 'var(--border)'),
                      }}
                    >
                      {tr.action}
                    </span>
                    <div className="m-card__title">
                      <div className="m-card__name">{tr.symbol}</div>
                      <div className="m-card__sub">
                        {fmtDT(tr.trade_date)}{tr.close_reason ? ' · ' + tr.close_reason : ''}
                      </div>
                    </div>
                    {tr.pnl_pct != null && <Chg pct={tr.pnl_pct} />}
                  </div>
                  <div className="m-card__metrics">
                    <Metric
                      label={t.simulationBot.colPrice}
                      value={tr.price != null
                        ? (currency === 'VND'
                            ? tr.price.toLocaleString('vi-VN')
                            : '$' + tr.price.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 }))
                        : '—'}
                    />
                    <Metric label={t.simulationBot.colQuantity} value={tr.quantity != null ? tr.quantity.toLocaleString('en-US', { maximumFractionDigits: 2 }) : '—'} />
                    <Metric label={t.simulationBot.colValue} value={fmtValueShort(tr.trade_value, currency)} />
                    <Metric label={t.simulationBot.colSignal} value={tr.signal_strength != null ? tr.signal_strength.toFixed(1) : '—'} />
                    <Metric label="Conf." value={tr.confidence != null ? (tr.confidence * 100).toFixed(0) + '%' : '—'} />
                    <Metric
                      label="P&L"
                      value={tr.pnl != null ? (tr.pnl >= 0 ? '+' : '') + fmtValueShort(tr.pnl, currency) : '—'}
                      color={tr.pnl != null ? (tr.pnl >= 0 ? 'var(--up)' : 'var(--down)') : undefined}
                    />
                  </div>
                </div>
              ))}
            </div>
            {/* Pagination */}
            {totalPages > 1 && (
              <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '12px 16px', borderTop: '1px solid var(--border)', fontSize: 12 }}>
                <span style={{ color: 'var(--text-3)' }}>
                  {t.simulationBot.page} {tradePage}/{totalPages} · {tradeTotal} {t.simulationBot.trades}
                </span>
                <div style={{ marginLeft: 'auto', display: 'flex', gap: 6 }}>
                  <button
                    className="btn btn--sm btn--ghost"
                    disabled={tradePage <= 1}
                    onClick={() => setTradePage(tradePage - 1)}
                  >
                    {t.simulationBot.pagePrev}
                  </button>
                  <button
                    className="btn btn--sm btn--ghost"
                    disabled={tradePage >= totalPages}
                    onClick={() => setTradePage(tradePage + 1)}
                  >
                    {t.simulationBot.pageNext}
                  </button>
                </div>
              </div>
            )}
          </>
        )}
      </Panel>

      </>
      )}

      {activeTab === 'variants' && (
        <VariantsTab
          variants={variants}
          currentBotId={botId || ''}
          sort={variantSort}
          onSort={(col) =>
            setVariantSort(s => ({ col, dir: s.col === col && s.dir === 'desc' ? 'asc' : 'desc' }))
          }
          currency={currency}
          onNavigate={(id) => navigate(`/simulation/${id}`)}
        />
      )}
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
