import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { Panel, KPI, Icon, Chg, vnsToast } from '../components/ui';
import { BarChart } from '../components/charts';
import { useAuth } from '../context/AuthContext';
import { useLanguage } from '../context/LangContext';

// ── Types ────────────────────────────────────────────────────────────────────

interface SimPeriod {
  start: string;
  end: string;
}

interface LeaderboardEntry {
  rank: number;
  bot_id: string;
  display_name: string;
  market: string;
  algorithm: string;
  currency: string;
  is_active: boolean;
  initial_capital: number | null;
  final_value: number | null;
  total_return_pct: number | null;
  annualized_return_pct: number | null;
  sharpe_ratio: number | null;
  max_drawdown_pct: number | null;
  win_rate_pct: number | null;
  profit_factor: number | null;
  total_trades: number | null;
  simulation_period: SimPeriod | null | undefined;
}

interface LeaderboardSummary {
  total_bots: number;
  best_market: string;
  best_algorithm: string;
  avg_return_pct: number;
}

interface LeaderboardMarketDist {
  market: string;
  count: number;
  avg_return_pct: number;
}

interface LeaderboardTopReturn {
  display_name: string;
  total_return_pct: number;
}

interface LeaderboardResponse {
  leaderboard: LeaderboardEntry[];
  summary: LeaderboardSummary;
  distribution: LeaderboardMarketDist[];
  top_returns: LeaderboardTopReturn[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}

const PAGE_SIZE_OPTIONS = [25, 50, 100, 200];

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

function fmtCapital(v: number | null, currency: string): string {
  if (v == null) return '—';
  if (currency === 'VND') {
    if (v >= 1e9) return (v / 1e9).toFixed(2) + ' tỷ';
    if (v >= 1e6) return (v / 1e6).toFixed(1) + 'M';
    return v.toLocaleString('vi-VN');
  }
  if (v >= 1e6) return '$' + (v / 1e6).toFixed(2) + 'M';
  if (v >= 1e4) return '$' + (v / 1e3).toFixed(1) + 'K';
  return '$' + v.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

function algoLabel(algo: string): string {
  const map: Record<string, string> = {
    lstm_nn: 'LSTM',
    arima_garch: 'ARIMA',
    moving_average: 'MA',
    ema: 'EMA',
    ema_macd: 'EMA',
    ensemble: 'ENS',
    lightgbm: 'LGB',
    random_forest: 'RF',
    xgboost: 'XGB',
    sarima: 'SARIMA',
    gru: 'GRU',
    gru_nn: 'GRU',
    egarch: 'EGARCH',
    rl_dqn: 'RL DQN',
  };
  return map[(algo || '').toLowerCase()] || algo.slice(0, 5).toUpperCase();
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

const MARKETS = ['ALL', 'GOLD', 'NASDAQ', 'SP500', 'CRYPTO'];
// Keys must match the algorithm values stored in sim_bots (see prediction-svc seeder).
const ALGORITHMS = [
  'ALL',
  'ensemble',
  'lstm_nn',
  'gru_nn',
  'arima_garch',
  'egarch',
  'sarima',
  'moving_average',
  'ema',
  'lightgbm',
  'xgboost',
  'random_forest',
  'rl_dqn',
];
const CURRENCIES = ['ALL', 'USD', 'VND'];

type SortKey =
  | 'rank'
  | 'display_name'
  | 'market'
  | 'algorithm'
  | 'initial_capital'
  | 'final_value'
  | 'total_return_pct'
  | 'annualized_return_pct'
  | 'sharpe_ratio'
  | 'max_drawdown_pct'
  | 'win_rate_pct'
  | 'total_trades';

// String columns sort alphabetically (ascending by default); the rest are numeric.
const STRING_SORT_KEYS: SortKey[] = ['display_name', 'market', 'algorithm'];

// ── Component ─────────────────────────────────────────────────────────────────

export default function Simulation() {
  const { isLoggedIn } = useAuth();
  const navigate = useNavigate();
  const { t } = useLanguage();

  const [entries, setEntries] = useState<LeaderboardEntry[]>([]);
  const [summary, setSummary] = useState<LeaderboardSummary | null>(null);
  const [distribution, setDistribution] = useState<LeaderboardMarketDist[]>([]);
  const [topReturns, setTopReturns] = useState<LeaderboardTopReturn[]>([]);
  const [total, setTotal] = useState(0);
  const [totalPages, setTotalPages] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [marketFilter, setMarketFilter] = useState('ALL');
  const [algoFilter, setAlgoFilter] = useState('ALL');
  const [currencyFilter, setCurrencyFilter] = useState('ALL');
  const [searchInput, setSearchInput] = useState('');
  const [search, setSearch] = useState('');

  const [sortKey, setSortKey] = useState<SortKey>('total_return_pct');
  const [sortAsc, setSortAsc] = useState(false);

  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(50);

  // silent=true skips the full-page loading flash (used by the auto-refresh poll).
  const load = useCallback((silent = false) => {
    if (!silent) setLoading(true);
    setError(null);
    const params = new URLSearchParams();
    if (marketFilter !== 'ALL') params.set('market', marketFilter);
    if (algoFilter !== 'ALL') params.set('algorithm', algoFilter);
    if (currencyFilter !== 'ALL') params.set('currency', currencyFilter);
    if (search) params.set('search', search);
    params.set('sort_by', sortKey);
    params.set('sort_dir', sortAsc ? 'asc' : 'desc');
    params.set('page', String(page));
    params.set('page_size', String(pageSize));
    apiFetch('/api/simulation/leaderboard?' + params.toString())
      .then((d: LeaderboardResponse) => {
        setEntries(Array.isArray(d.leaderboard) ? d.leaderboard : []);
        setSummary(d.summary || null);
        setDistribution(Array.isArray(d.distribution) ? d.distribution : []);
        setTopReturns(Array.isArray(d.top_returns) ? d.top_returns : []);
        setTotal(d.total ?? 0);
        setTotalPages(d.total_pages ?? 0);
        setLoading(false);
      })
      .catch((e) => {
        setError(String(e));
        setLoading(false);
      });
  }, [marketFilter, algoFilter, currencyFilter, search, sortKey, sortAsc, page, pageSize]);

  useEffect(() => { load(); }, [load]);

  // Debounce the search box into the active term; reset to page 1.
  useEffect(() => {
    const id = setTimeout(() => { setSearch(searchInput.trim()); setPage(1); }, 350);
    return () => clearTimeout(id);
  }, [searchInput]);

  // Auto-refresh leaderboard every 60s (silent — no loading flash). Resets when
  // filters/sort/page change because `load` is a new callback.
  useEffect(() => {
    const id = setInterval(() => load(true), 60000);
    return () => clearInterval(id);
  }, [load]);

  function handleSort(key: SortKey) {
    if (sortKey === key) {
      setSortAsc(!sortAsc);
    } else {
      setSortKey(key);
      // rank and string columns read best ascending by default; metrics descending.
      setSortAsc(key === 'rank' || STRING_SORT_KEYS.includes(key));
    }
    setPage(1);
  }

  // Entries arrive already filtered + sorted + paginated from the server.
  const sorted = entries;

  // Bar chart: top 8 by return (server-computed over the full filtered set).
  const barData = topReturns.map((e) => e.total_return_pct);
  const barLabels = topReturns.map((e) => e.display_name.split('—')[0].trim().slice(0, 6));

  function SortIndicator({ k }: { k: SortKey }) {
    if (sortKey !== k) return <span style={{ color: 'var(--text-3)', marginLeft: 3, fontSize: 10 }}>⇅</span>;
    return <span style={{ marginLeft: 3, fontSize: 10 }}>{sortAsc ? '↑' : '↓'}</span>;
  }

  function th(label: string, k: SortKey, cls = '') {
    return (
      <th className={cls + ' clickable'} onClick={() => handleSort(k)} style={{ cursor: 'pointer', userSelect: 'none', whiteSpace: 'nowrap' }}>
        {label}<SortIndicator k={k} />
      </th>
    );
  }

  if (loading) {
    return (
      <div className="content__inner fade">
        <div className="grid grid--kpis section-gap">
          {[1, 2, 3, 4].map((i) => <KPI key={i} label="—" value="—" sub="Loading..." />)}
        </div>
        <div className="empty section-gap">
          <div className="empty__icon"><Icon name="layers" size={18} /></div>
          <p>{t.simulation.loadingData}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="content__inner fade">
      {/* Summary KPI row */}
      <div className="grid grid--kpis section-gap">
        <KPI
          label={t.simulation.totalBots}
          value={summary?.total_bots != null ? String(summary.total_bots) : String(total)}
          sub={t.simulation.monitored}
        />
        <KPI
          label={t.simulation.bestMarket}
          value={summary?.best_market || '—'}
          sub={t.simulation.byAvgReturn}
          accent
        />
        <KPI
          label={t.simulation.bestAlgo}
          value={summary?.best_algorithm ? algoLabel(summary.best_algorithm) : '—'}
          sub={t.simulation.avgPerformance}
        />
        <KPI
          label={t.simulation.avgReturn}
          value={summary?.avg_return_pct != null ? summary.avg_return_pct.toFixed(2) + '%' : '—'}
          chgPct={summary?.avg_return_pct}
        />
      </div>

      {/* Error state */}
      {error && (
        <div className="empty section-gap" style={{ padding: '20px', background: 'var(--surface)', border: '1px solid var(--border)' }}>
          <div className="empty__icon"><Icon name="layers" size={18} /></div>
          <p style={{ color: 'var(--down)' }}>{t.simulation.cannotLoad}</p>
          <p style={{ fontSize: 11, color: 'var(--text-3)', marginTop: 4 }}>{error}</p>
        </div>
      )}

      {/* Charts row */}
      {!error && topReturns.length > 0 && (
        <div className="grid grid--halves section-gap">
          <Panel title={t.simulation.topReturnBots} sub={t.simulation.highest}>
            <BarChart
              data={barData}
              labels={barLabels}
              height={390}
              color="var(--accent)"
              colorByValue
              valueFmt={(v) => v.toFixed(2) + '%'}
            />
          </Panel>
          <Panel title={t.simulation.marketDist} sub={t.simulation.byBotCount}>
            <MarketDistribution distribution={distribution} />
          </Panel>
        </div>
      )}

      {/* Filter bar */}
      <Panel
        title={t.simulation.leaderboard}
        sub={total + ' ' + t.simulation.bots}
        className="section-gap"
        tools={
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center' }}>
            {/* Market filter */}
            <div className="seg">
              {MARKETS.map((m) => (
                <button key={m} className={marketFilter === m ? 'active' : ''} onClick={() => { setMarketFilter(m); setPage(1); }}>
                  {m}
                </button>
              ))}
            </div>
            {/* Currency filter */}
            <div className="seg">
              {CURRENCIES.map((c) => (
                <button key={c} className={currencyFilter === c ? 'active' : ''} onClick={() => { setCurrencyFilter(c); setPage(1); }}>
                  {c}
                </button>
              ))}
            </div>
            {/* Algorithm filter */}
            <select
              value={algoFilter}
              onChange={(e) => { setAlgoFilter(e.target.value); setPage(1); }}
              style={{
                background: 'var(--surface)',
                border: '1px solid var(--border)',
                color: 'var(--text)',
                padding: '5px 8px',
                fontSize: 12,
                fontFamily: 'var(--font-ui)',
              }}
            >
              {ALGORITHMS.map((a) => (
                <option key={a} value={a}>{a === 'ALL' ? t.simulation.allAlgos : algoLabel(a)}</option>
              ))}
            </select>
            {/* Bot search */}
            <input
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              placeholder={t.simulation.searchBot}
              style={{
                background: 'var(--surface)',
                border: '1px solid var(--border)',
                color: 'var(--text)',
                padding: '5px 8px',
                fontSize: 12,
                fontFamily: 'var(--font-ui)',
                minWidth: 140,
              }}
            />
            {isLoggedIn && (
              <button
                className="btn btn--sm"
                style={{ background: 'var(--accent)', borderColor: 'var(--accent)', color: '#fff' }}
                onClick={() =>
                  authPost('/api/simulation/run-all').then((ok) =>
                    vnsToast(ok ? t.simulation.runAllBacktest : t.simulation.cannotSendRequest)
                  )
                }
              >
                <Icon name="play" size={13} />Run All Backtest
              </button>
            )}
          </div>
        }
        flush
      >
        {total === 0 ? (
          <div className="empty" style={{ padding: '48px 20px' }}>
            <div className="empty__icon"><Icon name="layers" size={18} /></div>
            <p>{t.simulation.noSimData}</p>
            {isLoggedIn && (
              <button
                className="btn btn--sm"
                style={{ marginTop: 16, background: 'var(--accent)', borderColor: 'var(--accent)', color: '#fff' }}
                onClick={() =>
                  authPost('/api/simulation/run-all').then((ok) =>
                    vnsToast(ok ? t.simulation.runAllBacktest : t.simulation.cannotSendRequest)
                  )
                }
              >
                <Icon name="play" size={13} />{t.simulation.runNow}
              </button>
            )}
          </div>
        ) : (
          <>
          <div className="tbl-scroll" style={{ overflowX: 'auto' }}>
            <table className="tbl">
              <thead>
                <tr>
                  {th(t.simulation.colRank, 'rank', 'c')}
                  {th(t.simulation.colBot, 'display_name')}
                  {th(t.simulation.colMarket, 'market')}
                  {th(t.simulation.colAlgo, 'algorithm', 'c')}
                  {isLoggedIn && <th className="c">{t.simulation.colStatus}</th>}
                  {th(t.simulation.colInitCapital, 'initial_capital', 'r')}
                  {th(t.simulation.colFinalValue, 'final_value', 'r')}
                  {th('Return%', 'total_return_pct', 'r')}
                  {th('Annlzd%', 'annualized_return_pct', 'r')}
                  {th('Sharpe', 'sharpe_ratio', 'r')}
                  {th('Max DD', 'max_drawdown_pct', 'r')}
                  {th('Win%', 'win_rate_pct', 'r')}
                  {th('Trades', 'total_trades', 'r')}
                </tr>
              </thead>
              <tbody>
                {sorted.map((e) => (
                  <tr
                    key={e.bot_id}
                    className="clickable"
                    onClick={() => navigate('/simulation/' + e.bot_id)}
                  >
                    <td className="c num" style={{ color: e.rank <= 3 ? 'var(--gold)' : 'var(--text-3)', fontWeight: e.rank <= 3 ? 700 : 400 }}>
                      {e.rank <= 3 ? ['🥇', '🥈', '🥉'][e.rank - 1] : e.rank}
                    </td>
                    <td>
                      <div style={{ fontWeight: 600, fontSize: 13 }}>{e.display_name}</div>
                      <div style={{ fontSize: 11, color: 'var(--text-3)', fontFamily: 'var(--font-mono)' }}>
                        {e.simulation_period?.start ? fmtDT(e.simulation_period.start) : '—'} → {e.simulation_period?.end ? fmtDT(e.simulation_period.end) : '—'}
                      </div>
                    </td>
                    <td>
                      <span className="badge badge--muted" style={{ fontSize: 10.5, letterSpacing: 0.3 }}>
                        {e.market}
                      </span>
                    </td>
                    <td className="c">
                      <span className={`algo algo--${algoClass(e.algorithm)}`}>{algoLabel(e.algorithm)}</span>
                    </td>
                    {isLoggedIn && (
                      <td className="c">
                        <button
                          style={{
                            fontSize: 10,
                            padding: '2px 8px',
                            background: e.is_active ? 'var(--up-bg)' : 'var(--surface-2)',
                            color: e.is_active ? 'var(--up)' : 'var(--text-3)',
                            border: '1px solid ' + (e.is_active ? 'var(--up)' : 'var(--border)'),
                            fontFamily: 'var(--font-mono)',
                            cursor: 'pointer',
                            fontWeight: 600,
                          }}
                          title={e.is_active ? 'Click to disable' : 'Click to enable'}
                          onClick={(ev) => {
                            ev.stopPropagation();
                            authPost(`/api/simulation/bots/${e.bot_id}/toggle`).then((ok) => {
                              if (ok) {
                                setEntries((prev) =>
                                  prev.map((b) => b.bot_id === e.bot_id ? { ...b, is_active: !b.is_active } : b)
                                );
                                vnsToast(!e.is_active ? `${e.display_name} activated` : `${e.display_name} disabled`);
                              } else {
                                vnsToast('Failed (admin required)');
                              }
                            });
                          }}
                        >
                          {e.is_active ? 'ON' : 'OFF'}
                        </button>
                      </td>
                    )}
                    <td className="r num" style={{ color: 'var(--text-2)', fontSize: 12 }}>
                      {fmtCapital(e.initial_capital, e.currency)}
                    </td>
                    <td className="r num" style={{ fontWeight: 600, fontSize: 12 }}>
                      {fmtCapital(e.final_value, e.currency)}
                    </td>
                    <td className="r">
                      <Chg pct={e.total_return_pct} />
                    </td>
                    <td className="r num" style={{ fontSize: 12, color: (e.annualized_return_pct ?? 0) >= 0 ? 'var(--up)' : 'var(--down)' }}>
                      {(e.annualized_return_pct ?? 0) >= 0 ? '+' : ''}{(e.annualized_return_pct ?? 0).toFixed(1)}%
                    </td>
                    <td className="r num" style={{ fontSize: 12, color: (e.sharpe_ratio ?? 0) >= 1.5 ? 'var(--up)' : (e.sharpe_ratio ?? 0) >= 0 ? 'var(--text-2)' : 'var(--down)' }}>
                      {(e.sharpe_ratio ?? 0).toFixed(2)}
                    </td>
                    <td className="r">
                      <span className="num" style={{ fontSize: 12, color: 'var(--down)' }}>
                        {(e.max_drawdown_pct ?? 0).toFixed(1)}%
                      </span>
                    </td>
                    <td className="r num" style={{ fontSize: 12, color: (e.win_rate_pct ?? 0) >= 55 ? 'var(--up)' : 'var(--text-2)' }}>
                      {(e.win_rate_pct ?? 0).toFixed(1)}%
                    </td>
                    <td className="r num" style={{ color: 'var(--text-2)', fontSize: 12 }}>{e.total_trades}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Mobile card view — same data, stacked instead of scrolling sideways */}
          <div className="m-cards">
            {sorted.map((e) => (
              <div
                key={e.bot_id}
                className="m-card clickable"
                onClick={() => navigate('/simulation/' + e.bot_id)}
              >
                <div className="m-card__head">
                  <span className="m-card__rank" style={{ color: e.rank <= 3 ? 'var(--gold)' : 'var(--text-3)' }}>
                    {e.rank <= 3 ? ['🥇', '🥈', '🥉'][e.rank - 1] : '#' + e.rank}
                  </span>
                  <div className="m-card__title">
                    <div className="m-card__name">{e.display_name}</div>
                    <div className="m-card__sub">
                      {e.simulation_period?.start ? fmtDT(e.simulation_period.start) : '—'} → {e.simulation_period?.end ? fmtDT(e.simulation_period.end) : '—'}
                    </div>
                  </div>
                  <Chg pct={e.total_return_pct ?? 0} />
                </div>
                <div className="m-card__badges">
                  <span className="badge badge--muted" style={{ fontSize: 10.5, letterSpacing: 0.3 }}>{e.market}</span>
                  <span className={`algo algo--${algoClass(e.algorithm)}`}>{algoLabel(e.algorithm)}</span>
                  {isLoggedIn && (
                    <button
                      style={{
                        marginLeft: 'auto',
                        fontSize: 10,
                        padding: '2px 8px',
                        background: e.is_active ? 'var(--up-bg)' : 'var(--surface-2)',
                        color: e.is_active ? 'var(--up)' : 'var(--text-3)',
                        border: '1px solid ' + (e.is_active ? 'var(--up)' : 'var(--border)'),
                        fontFamily: 'var(--font-mono)',
                        cursor: 'pointer',
                        fontWeight: 600,
                      }}
                      title={e.is_active ? 'Click to disable' : 'Click to enable'}
                      onClick={(ev) => {
                        ev.stopPropagation();
                        authPost(`/api/simulation/bots/${e.bot_id}/toggle`).then((ok) => {
                          if (ok) {
                            setEntries((prev) =>
                              prev.map((b) => b.bot_id === e.bot_id ? { ...b, is_active: !b.is_active } : b)
                            );
                            vnsToast(!e.is_active ? `${e.display_name} activated` : `${e.display_name} disabled`);
                          } else {
                            vnsToast('Failed (admin required)');
                          }
                        });
                      }}
                    >
                      {e.is_active ? 'ON' : 'OFF'}
                    </button>
                  )}
                </div>
                <div className="m-card__metrics">
                  <Metric label={t.simulation.colInitCapital} value={fmtCapital(e.initial_capital, e.currency)} />
                  <Metric label={t.simulation.colFinalValue} value={fmtCapital(e.final_value, e.currency)} />
                  <Metric
                    label="Annlzd%"
                    value={((e.annualized_return_pct ?? 0) >= 0 ? '+' : '') + (e.annualized_return_pct ?? 0).toFixed(1) + '%'}
                    color={(e.annualized_return_pct ?? 0) >= 0 ? 'var(--up)' : 'var(--down)'}
                  />
                  <Metric label="Sharpe" value={(e.sharpe_ratio ?? 0).toFixed(2)} />
                  <Metric label="Max DD" value={(e.max_drawdown_pct ?? 0).toFixed(1) + '%'} color="var(--down)" />
                  <Metric
                    label="Win%"
                    value={(e.win_rate_pct ?? 0).toFixed(1) + '%'}
                    color={(e.win_rate_pct ?? 0) >= 55 ? 'var(--up)' : undefined}
                  />
                  <Metric label="Trades" value={String(e.total_trades ?? 0)} />
                </div>
              </div>
            ))}
          </div>

          {/* Pagination */}
          <div style={{
            display: 'flex', flexWrap: 'wrap', gap: 12, alignItems: 'center',
            justifyContent: 'space-between', padding: '12px 14px',
            borderTop: '1px solid var(--border)',
          }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12, color: 'var(--text-3)' }}>
              <span>{t.simulation.rowsPerPage}:</span>
              <select
                value={pageSize}
                onChange={(e) => { setPageSize(Number(e.target.value)); setPage(1); }}
                style={{
                  background: 'var(--surface)', border: '1px solid var(--border)',
                  color: 'var(--text)', padding: '4px 8px', fontSize: 12,
                }}
              >
                {PAGE_SIZE_OPTIONS.map((n) => <option key={n} value={n}>{n}</option>)}
              </select>
              <span style={{ fontFamily: 'var(--font-mono)' }}>
                {total === 0 ? 0 : (page - 1) * pageSize + 1}–{Math.min(page * pageSize, total)} / {total}
              </span>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <button
                className="btn btn--sm"
                disabled={page <= 1 || loading}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
              >
                ‹ {t.simulation.prevPage}
              </button>
              <span style={{ fontSize: 12, color: 'var(--text-3)', fontFamily: 'var(--font-mono)', minWidth: 80, textAlign: 'center' }}>
                {t.simulation.page} {page} / {Math.max(1, totalPages)}
              </span>
              <button
                className="btn btn--sm"
                disabled={page >= totalPages || loading}
                onClick={() => setPage((p) => p + 1)}
              >
                {t.simulation.nextPage} ›
              </button>
            </div>
          </div>
          </>
        )}
      </Panel>
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

// ── Market distribution mini chart ───────────────────────────────────────────

function MarketDistribution({ distribution }: { distribution: LeaderboardMarketDist[] }) {
  const { t } = useLanguage();
  if (distribution.length === 0) {
    return (
      <div className="empty" style={{ height: 390, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
        <div className="empty__icon"><Icon name="layers" size={18} /></div>
        <p>{t.simulation.noData}</p>
      </div>
    );
  }

  // Server already sorts by avg return desc; map to the chart's local shape.
  const markets = distribution.map((d) => ({
    market: d.market,
    count: d.count,
    avgReturn: d.avg_return_pct,
  }));

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
      {markets.map(({ market, count, avgReturn }) => (
        <div key={market} style={{ display: 'flex', alignItems: 'center', gap: 10, fontSize: 13 }}>
          <span style={{ minWidth: 70, fontWeight: 600 }}>{market}</span>
          <div style={{ flex: 1, height: 6, background: 'var(--surface-2)', position: 'relative' }}>
            <div
              style={{
                position: 'absolute', left: 0, top: 0, bottom: 0,
                width: Math.abs(avgReturn) / 60 * 100 + '%',
                background: avgReturn >= 0 ? 'var(--up)' : 'var(--down)',
                maxWidth: '100%',
              }}
            />
          </div>
          <span className="num" style={{ minWidth: 52, textAlign: 'right', fontSize: 12, color: avgReturn >= 0 ? 'var(--up)' : 'var(--down)' }}>
            {avgReturn >= 0 ? '+' : ''}{avgReturn.toFixed(1)}%
          </span>
          <span style={{ minWidth: 28, textAlign: 'right', fontSize: 11, color: 'var(--text-3)' }}>
            {count}b
          </span>
        </div>
      ))}
    </div>
  );
}
