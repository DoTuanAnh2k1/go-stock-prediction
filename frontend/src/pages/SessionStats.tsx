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

interface BotTradeRow {
  algorithm: string;
  trades: number;
  wins: number;
  losses: number;
  breakeven: number;
  total_pnl: number;
  avg_pnl_per_trade: number;
  win_rate: number;
}

interface SessionStatsData {
  market: string;
  session: SessionWindow;
  direction_accuracy: DirAccRow[];
  bot_trades: BotTradeRow[];
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

// ── Component ────────────────────────────────────────────────────────────────
export default function SessionStats() {
  const { marketKey } = useParams<{ marketKey: string }>();
  const { t } = useLanguage();
  const [data, setData] = useState<SessionStatsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!marketKey) return;
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
  const totalPnL = bot_trades.reduce((s, r) => s + r.total_pnl, 0);
  const totalWins = bot_trades.reduce((s, r) => s + r.wins, 0);
  const overallWR = totalTrades > 0 ? totalWins / totalTrades : null;

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
          <div className="session-kpi-sub">{bot_trades.length} thuật toán có lệnh</div>
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
                  <th>Bar</th>
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

      {/* Bot Trades table */}
      <Panel title="Giao dịch bot theo thuật toán">
        {bot_trades.length === 0 ? (
          <div className="no-data-msg">{t.common.noData}</div>
        ) : (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Thuật toán</th>
                  <th className="num">Lệnh</th>
                  <th className="num">Thắng</th>
                  <th className="num">Thua</th>
                  <th className="num">Hòa</th>
                  <th className="num">Win rate</th>
                  <th className="num">PnL</th>
                  <th className="num">PnL/lệnh</th>
                </tr>
              </thead>
              <tbody>
                {bot_trades
                  .slice()
                  .sort((a, b) => b.total_pnl - a.total_pnl)
                  .map(row => (
                    <tr key={row.algorithm}>
                      <td>{algoName(row.algorithm)}</td>
                      <td className="num">{row.trades}</td>
                      <td className="num up">{row.wins}</td>
                      <td className="num dn">{row.losses}</td>
                      <td className="num">{row.breakeven}</td>
                      <td className={'num' + (row.win_rate >= 0.5 ? ' up' : ' dn')}>
                        {pct(row.win_rate)}
                      </td>
                      <td className={'num' + (row.total_pnl >= 0 ? ' up' : ' dn')}>
                        {pnl(row.total_pnl)}
                      </td>
                      <td className={'num' + (row.avg_pnl_per_trade >= 0 ? ' up' : ' dn')}>
                        {pnl(row.avg_pnl_per_trade)}
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
