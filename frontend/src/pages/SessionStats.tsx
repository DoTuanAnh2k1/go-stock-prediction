import { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import { Panel, Icon } from '../components/ui';
import { useLanguage } from '../context/LangContext';

// ── Types ────────────────────────────────────────────────────────────────────
interface SessionWindow {
  start: string;
  end: string;
  is_open: boolean;
}

interface DirAccRow {
  algorithm: string;
  total: number;
  correct: number;
  accuracy: number;
}

interface BotDetail {
  bot_id: string;
  display_name: string;
  algorithm: string;
  currency: string;
  initial_capital: number;
  current_value: number;
  session_pnl: number;
  total_return_pct: number;
  trades: number;
  wins: number;
  losses: number;
  breakeven: number;
  win_rate: number;
}

interface SessionStatsData {
  market: string;
  session: SessionWindow;
  direction_accuracy: DirAccRow[];
  bot_trades: BotDetail[];
}

// ── Helpers ──────────────────────────────────────────────────────────────────
const ALGO_DISPLAY: Record<string, string> = {
  lstm_nn: 'LSTM', lstm: 'LSTM', arima_garch: 'ARIMA', arima: 'ARIMA',
  moving_average: 'MA', ema: 'EMA', ema_macd: 'EMA/MACD',
  lightgbm: 'LightGBM', random_forest: 'RF', xgboost: 'XGBoost',
  gru: 'GRU', ensemble: 'Ensemble',
};
function algoName(key: string) { return ALGO_DISPLAY[key.toLowerCase()] || key; }

function fmtTime(iso: string) {
  try {
    return new Date(iso).toLocaleString('vi-VN', {
      hour: '2-digit', minute: '2-digit',
      day: '2-digit', month: '2-digit', year: 'numeric',
    });
  } catch { return iso; }
}

function fmtMoney(v: number, currency = 'USD') {
  return v.toLocaleString('en-US', { maximumFractionDigits: 2 }) + ' ' + currency;
}

function pct(v: number) { return (v * 100).toFixed(1) + '%'; }
function pnl(v: number) { return (v >= 0 ? '+' : '') + v.toFixed(2); }

async function apiFetch(path: string) {
  const token = localStorage.getItem('vns_token') || '';
  const res = await fetch(path, {
    headers: { Accept: 'application/json', Authorization: `Bearer ${token}` },
  });
  if (!res.ok) throw new Error('HTTP ' + res.status);
  return res.json();
}

// ── Sort key type ─────────────────────────────────────────────────────────────
type SortKey = 'display_name' | 'algorithm' | 'session_pnl' | 'current_value' | 'total_return_pct' | 'trades' | 'win_rate';

// ── Component ────────────────────────────────────────────────────────────────
export default function SessionStats() {
  const { marketKey } = useParams<{ marketKey: string }>();
  const { t } = useLanguage();
  const [data, setData] = useState<SessionStatsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const [sortKey, setSortKey] = useState<SortKey>('session_pnl');
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc');
  const [search, setSearch] = useState('');
  const [page, setPage] = useState(1);
  const PAGE_SIZE = 20;

  function toggleSort(key: SortKey) {
    if (sortKey === key) setSortDir(d => d === 'desc' ? 'asc' : 'desc');
    else { setSortKey(key); setSortDir('desc'); }
    setPage(1);
  }

  useEffect(() => {
    if (!marketKey) return;
    setPage(1);
    setSearch('');
    setLoading(true);
    setError('');
    apiFetch(`/api/markets/${marketKey}/session-stats`)
      .then(setData)
      .catch(() => setError('Lỗi tải dữ liệu'))
      .finally(() => setLoading(false));
  }, [marketKey]);

  if (loading) return <div className="page-loading">{t.common.loading}</div>;
  if (error || !data) return <div className="page-error">{error || t.common.noData}</div>;

  const { session, direction_accuracy, bot_trades } = data;
  const totalDir = direction_accuracy.reduce((s, r) => s + r.total, 0);
  const totalCorrect = direction_accuracy.reduce((s, r) => s + r.correct, 0);
  const overallAcc = totalDir > 0 ? totalCorrect / totalDir : null;
  const totalTrades = bot_trades.reduce((s, r) => s + r.trades, 0);
  const totalPnL = bot_trades.reduce((s, r) => s + r.session_pnl, 0);
  const totalWins = bot_trades.reduce((s, r) => s + r.wins, 0);
  const overallWR = totalTrades > 0 ? totalWins / totalTrades : null;

  const SORTABLE_COLS: SortKey[] = ['display_name', 'algorithm', 'current_value', 'session_pnl', 'total_return_pct', 'trades', 'win_rate'];

  const filteredBots = bot_trades.filter(r => {
    if (!search) return true;
    const q = search.toLowerCase();
    return r.display_name.toLowerCase().includes(q) || r.algorithm.toLowerCase().includes(q);
  });

  const sortedBots = [...filteredBots].sort((a, b) => {
    const v = sortKey === 'display_name' || sortKey === 'algorithm'
      ? a[sortKey].localeCompare(b[sortKey])
      : (a[sortKey] as number) - (b[sortKey] as number);
    return sortDir === 'desc' ? -v : v;
  });

  const totalPages = Math.ceil(sortedBots.length / PAGE_SIZE);
  const pagedBots = sortedBots.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE);

  return (
    <div className="session-stats">
      {/* Session banner */}
      <div className={'session-banner' + (session.is_open ? ' open' : ' closed')}>
        <Icon name={session.is_open ? 'activity' : 'clock'} size={16} />
        <span className="session-status">
          {session.is_open ? 'Đang mở' : 'Đã đóng'}
        </span>
        <span className="session-window">
          {fmtTime(session.start)} → {fmtTime(session.end)}
        </span>
      </div>

      {/* Summary KPIs */}
      <div className="session-kpis">
        <div className="session-kpi">
          <div className="session-kpi-label">Độ chính xác hướng</div>
          <div className={'session-kpi-val' + (overallAcc !== null && overallAcc >= 0.5 ? ' up' : ' dn')}>
            {overallAcc !== null ? pct(overallAcc) : '—'}
          </div>
          <div className="session-kpi-sub">{totalCorrect}/{totalDir} dự đoán</div>
        </div>
        <div className="session-kpi">
          <div className="session-kpi-label">Tổng lệnh bot</div>
          <div className="session-kpi-val">{totalTrades}</div>
          <div className="session-kpi-sub">Win rate: {overallWR !== null ? pct(overallWR) : '—'}</div>
        </div>
        <div className="session-kpi">
          <div className="session-kpi-label">Tổng PnL</div>
          <div className={'session-kpi-val' + (totalPnL >= 0 ? ' up' : ' dn')}>{pnl(totalPnL)}</div>
          <div className="session-kpi-sub">{bot_trades.length} bot có lệnh</div>
        </div>
      </div>

      {/* Direction Accuracy table */}
      <Panel title="Độ chính xác hướng theo thuật toán">
        {direction_accuracy.length === 0 ? (
          <div className="no-data-msg">{t.common.noData}</div>
        ) : (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Thuật toán</th>
                  <th className="num">Tổng</th>
                  <th className="num">Đúng</th>
                  <th className="num">Độ chính xác</th>
                  <th style={{ width: '100%' }}></th>
                </tr>
              </thead>
              <tbody>
                {direction_accuracy
                  .slice()
                  .sort((a, b) => b.accuracy - a.accuracy)
                  .map(row => (
                    <tr key={row.algorithm}>
                      <td>{algoName(row.algorithm)}</td>
                      <td className="num">{row.total}</td>
                      <td className="num">{row.correct}</td>
                      <td className={'num' + (row.accuracy >= 0.5 ? ' up' : ' dn')}>
                        {pct(row.accuracy)}
                      </td>
                      <td>
                        <div className="acc-bar-wrap">
                          <div
                            className="acc-bar-fill"
                            style={{
                              width: pct(row.accuracy),
                              background: row.accuracy >= 0.5 ? 'var(--up)' : 'var(--dn)',
                            }}
                          />
                        </div>
                      </td>
                    </tr>
                  ))}
              </tbody>
            </table>
          </div>
        )}
      </Panel>

      {/* Bot detail table (per-bot, sortable, searchable, paginated) */}
      <Panel title={`Chi tiết bot giao dịch (${filteredBots.length}/${bot_trades.length})`}>
        {/* Search bar */}
        <div style={{ marginBottom: 12 }}>
          <input
            type="text"
            placeholder="Tìm bot hoặc thuật toán..."
            value={search}
            onChange={e => { setSearch(e.target.value); setPage(1); }}
            className="session-search"
          />
        </div>

        {filteredBots.length === 0 ? (
          <div className="no-data-msg">{t.common.noData}</div>
        ) : (
          <>
            <div className="table-wrap" style={{ overflowX: 'auto' }}>
              <table className="data-table session-bot-table">
                <thead>
                  <tr>
                    <th className="num" style={{ minWidth: 48, width: 48 }}>#</th>
                    {(
                      [
                        ['display_name', 'Bot', 180],
                        ['algorithm', 'Thuật toán', 100],
                        ['current_value', 'Tài khoản', 140],
                        ['session_pnl', 'PnL phiên', 110],
                        ['total_return_pct', 'Tổng lợi nhuận', 160],
                        ['trades', 'Lệnh', 70],
                        ['wins_col', 'Thắng', 70],
                        ['losses_col', 'Thua', 70],
                        ['breakeven_col', 'Hòa', 70],
                        ['win_rate', 'Win rate', 90],
                      ] as [string, string, number][]
                    ).map(([k, label, mw]) => {
                      const isSortable = SORTABLE_COLS.includes(k as SortKey);
                      return (
                        <th
                          key={k}
                          style={{ minWidth: mw }}
                          className={isSortable ? 'sortable-th' : ''}
                          onClick={isSortable ? () => toggleSort(k as SortKey) : undefined}
                        >
                          <span className="th-inner">
                            {label}
                            {isSortable && (
                              <span className={'sort-icon' + (sortKey === k ? ' active' : '')}>
                                {sortKey === k ? (sortDir === 'desc' ? ' ▼' : ' ▲') : ' ⇅'}
                              </span>
                            )}
                          </span>
                        </th>
                      );
                    })}
                  </tr>
                </thead>
                <tbody key={`page-${page}`}>
                  {pagedBots.map((row, idx) => {
                    const profit = row.current_value - row.initial_capital;
                    const rowNum = (page - 1) * PAGE_SIZE + idx + 1;
                    return (
                      <tr key={`${page}-${idx}-${row.bot_id}`}>
                        <td className="num row-num">{rowNum}</td>
                        <td className="bot-name">{row.display_name}</td>
                        <td className="algo-badge">{algoName(row.algorithm)}</td>
                        <td className="num">{fmtMoney(row.current_value, row.currency)}</td>
                        <td className={'num' + (row.session_pnl > 0 ? ' up' : row.session_pnl < 0 ? ' dn' : '')}>
                          {row.session_pnl !== 0 ? pnl(row.session_pnl) : '—'}
                        </td>
                        <td className={'num' + (profit >= 0 ? ' up' : ' dn')}>
                          {pnl(profit)} <span style={{ opacity: 0.7 }}>({row.total_return_pct.toFixed(2)}%)</span>
                        </td>
                        <td className="num">{row.trades}</td>
                        <td className="num up">{row.wins}</td>
                        <td className="num dn">{row.losses}</td>
                        <td className="num">{row.breakeven}</td>
                        <td className={'num' + (row.win_rate >= 0.5 ? ' up' : ' dn')}>
                          {row.trades > 0 ? pct(row.win_rate) : '—'}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>

            {/* Pagination */}
            {totalPages > 1 && (
              <div className="session-pagination">
                <button className="pg-btn" onClick={() => setPage(1)} disabled={page === 1}>«</button>
                <button className="pg-btn" onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1}>‹</button>
                <span className="pg-info">Trang {page} / {totalPages} ({filteredBots.length} bot)</span>
                <button className="pg-btn" onClick={() => setPage(p => Math.min(totalPages, p + 1))} disabled={page === totalPages}>›</button>
                <button className="pg-btn" onClick={() => setPage(totalPages)} disabled={page === totalPages}>»</button>
              </div>
            )}
          </>
        )}
      </Panel>
    </div>
  );
}
