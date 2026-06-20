import { useState, useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { Panel, Icon } from '../components/ui';
import { useAuth } from '../context/AuthContext';
import { fetchSchedules, updateSchedule, triggerEndpoint, type ScheduleItem } from '../api';
import { useLanguage } from '../context/LangContext';

function getToken() {
  return localStorage.getItem('vns_token') || '';
}

// ── Cron schedule helpers ─────────────────────────────────────────────────────

type FreqType = 'every_hour' | 'every_n_hours' | 'daily' | 'weekly';

const DOW_OPTIONS = [
  { value: 'MON', label: 'Thứ 2' },
  { value: 'TUE', label: 'Thứ 3' },
  { value: 'WED', label: 'Thứ 4' },
  { value: 'THU', label: 'Thứ 5' },
  { value: 'FRI', label: 'Thứ 6' },
  { value: 'SAT', label: 'Thứ 7' },
  { value: 'SUN', label: 'Chủ nhật' },
];

interface ParsedSchedule {
  type: FreqType;
  interval: number;
  hour: number;
  minute: number;
  dow: string;
}

function parseCron(expr: string): ParsedSchedule {
  const defaults: ParsedSchedule = { type: 'daily', interval: 2, hour: 12, minute: 0, dow: 'MON' };
  const parts = expr.trim().split(/\s+/);
  if (parts.length !== 6) return defaults;
  const [, minStr, hourStr, , , dowStr] = parts;
  // every hour: 0 0 * * * *
  if (hourStr === '*' && dowStr === '*') return { ...defaults, type: 'every_hour' };
  // every N hours: 0 0 */N * * *
  if (hourStr.startsWith('*/') && dowStr === '*') {
    const n = parseInt(hourStr.slice(2));
    return { ...defaults, type: 'every_n_hours', interval: isNaN(n) ? 2 : n };
  }
  const h = parseInt(hourStr);
  const m = parseInt(minStr);
  // weekly: 0 0 H * * DOW
  if (dowStr !== '*' && !isNaN(h)) {
    return { ...defaults, type: 'weekly', hour: h, minute: isNaN(m) ? 0 : m, dow: dowStr };
  }
  // daily: 0 0 H * * *
  if (!isNaN(h)) {
    return { ...defaults, type: 'daily', hour: h, minute: isNaN(m) ? 0 : m };
  }
  return defaults;
}

function buildCron(p: ParsedSchedule): string {
  const h = p.hour, m = p.minute;
  switch (p.type) {
    case 'every_hour':    return '0 0 * * * *';
    case 'every_n_hours': return `0 0 */${p.interval} * * *`;
    case 'daily':         return `0 ${m} ${h} * * *`;
    case 'weekly':        return `0 ${m} ${h} * * ${p.dow}`;
  }
}

type CronLabelT = {
  everyHour: string;
  everyLabel: string;
  hours: string;
  daily: string;
  weekly: string;
};

function cronLabel(expr: string, tSched: CronLabelT, dowOptions: { value: string; label: string }[]): string {
  const p = parseCron(expr);
  const pad = (n: number) => ('0' + n).slice(-2);
  switch (p.type) {
    case 'every_hour':    return tSched.everyHour;
    case 'every_n_hours': return `${tSched.everyLabel} ${p.interval} ${tSched.hours}`;
    case 'daily':         return `${tSched.daily} ${pad(p.hour)}:${pad(p.minute)}`;
    case 'weekly': {
      const d = dowOptions.find(x => x.value === p.dow)?.label || p.dow;
      return `${tSched.weekly} ${d} ${pad(p.hour)}:${pad(p.minute)}`;
    }
  }
}

// ── Schedule editor ───────────────────────────────────────────────────────────

function ScheduleEditor({ expr, onChange }: { expr: string; onChange: (newExpr: string) => void }) {
  const { t } = useLanguage();
  const init = parseCron(expr);
  const [type, setType]         = useState<FreqType>(init.type);
  const [interval, setInterval] = useState(init.interval);
  const [hour, setHour]         = useState(init.hour);
  const [minute, setMinute]     = useState(init.minute);
  const [dow, setDow]           = useState(init.dow);

  useEffect(() => {
    onChange(buildCron({ type, interval, hour, minute, dow }));
  }, [type, interval, hour, minute, dow]);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      {/* Frequency type */}
      <select
        className="sel"
        value={type}
        onChange={(e) => setType(e.target.value as FreqType)}
        style={{ fontSize: 13 }}
      >
        <option value="every_hour">{t.settings.everyHour}</option>
        <option value="every_n_hours">{t.settings.everyNHours}</option>
        <option value="daily">{t.settings.daily}</option>
        <option value="weekly">{t.settings.weekly}</option>
      </select>

      {/* Every N hours */}
      {type === 'every_n_hours' && (
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ fontSize: 13, color: 'var(--text-2)' }}>{t.settings.everyLabel}</span>
          <select
            className="sel"
            value={interval}
            onChange={(e) => setInterval(Number(e.target.value))}
            style={{ fontSize: 13 }}
          >
            {[2, 3, 4, 6, 8, 12].map((n) => (
              <option key={n} value={n}>{n} {t.settings.hours}</option>
            ))}
          </select>
        </div>
      )}

      {/* Weekly: day selector */}
      {type === 'weekly' && (
        <select
          className="sel"
          value={dow}
          onChange={(e) => setDow(e.target.value)}
          style={{ fontSize: 13 }}
        >
          {DOW_OPTIONS.map((d) => <option key={d.value} value={d.value}>{d.label}</option>)}
        </select>
      )}

      {/* Daily / Weekly: time picker */}
      {(type === 'daily' || type === 'weekly') && (
        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <input
            type="number" min={0} max={23}
            value={hour}
            onChange={(e) => setHour(Math.max(0, Math.min(23, Number(e.target.value))))}
            className="field__input"
            style={{ width: 52, fontFamily: 'var(--font-mono)', fontSize: 13, textAlign: 'center' }}
          />
          <span style={{ color: 'var(--text-3)', fontWeight: 700 }}>:</span>
          <input
            type="number" min={0} max={59}
            value={minute}
            onChange={(e) => setMinute(Math.max(0, Math.min(59, Number(e.target.value))))}
            className="field__input"
            style={{ width: 52, fontFamily: 'var(--font-mono)', fontSize: 13, textAlign: 'center' }}
          />
          <span style={{ fontSize: 11, color: 'var(--text-3)' }}>{t.settings.hourMinute}</span>
        </div>
      )}

      {/* Preview */}
      <div style={{ fontSize: 11, color: 'var(--text-3)', fontFamily: 'var(--font-mono)', padding: '2px 0' }}>
        {buildCron({ type, interval, hour, minute, dow })}
      </div>
    </div>
  );
}

// ── Single trigger button ─────────────────────────────────────────────────────
function TriggerBtn({ label, endpoint, icon }: { label: string; endpoint: string; icon: string }) {
  const { t } = useLanguage();
  const [loading, setLoading] = useState(false);
  const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null);

  const run = async () => {
    setLoading(true);
    setMsg(null);
    try {
      const m = await triggerEndpoint(endpoint);
      setMsg({ ok: true, text: m });
    } catch (e: any) {
      setMsg({ ok: false, text: e?.message || 'Lỗi' });
    } finally {
      setLoading(false);
      setTimeout(() => setMsg(null), 5000);
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
      <button
        className="btn btn--ghost"
        style={{ display: 'flex', alignItems: 'center', gap: 7, padding: '7px 12px', fontSize: 12 }}
        disabled={loading}
        onClick={run}
      >
        <Icon name={icon} size={13} />
        {loading ? t.settings.running : label}
      </button>
      {msg && (
        <div style={{ fontSize: 11, color: msg.ok ? 'var(--up)' : 'var(--down)', paddingLeft: 4 }}>
          {msg.ok ? '✓' : '✗'} {msg.text}
        </div>
      )}
    </div>
  );
}

// ── Pipeline button (crawl → predict → simulation) ────────────────────────────
type PipelineStep = { label: string; endpoint: string };
type StepState = 'idle' | 'running' | 'ok' | 'error';

// ── Pipeline report types ─────────────────────────────────────────────────────
interface LogEntry {
  ts: string;
  level: 'info' | 'ok' | 'error' | 'warn';
  msg: string;
}

interface AlgoResult {
  key: string;
  name: string;
  status: 'ok' | 'none';
  count: number;
}

interface BotResult {
  bot_id: string;
  display_name: string;
  market: string;
  algorithm: string;
  is_active: boolean;
  currency: string;
  kpis: {
    total_return_pct: number | null;
    win_rate_pct: number | null;
    sharpe_ratio: number | null;
    max_drawdown_pct: number | null;
    total_trades: number | null;
  } | null;
}

interface PipelineReport {
  id: string;
  market: string;
  marketKey: string;
  timestamp: string;
  durationMs: number;
  steps: { label: string; status: StepState }[];
  logs: LogEntry[];
  algorithms: AlgoResult[];
  botResults: BotResult[];
  predictionsCount: number;
  success: boolean;
}

const KNOWN_ALGOS: { key: string; name: string }[] = [
  { key: 'lstm_nn',        name: 'LSTM' },
  { key: 'arima_garch',    name: 'ARIMA' },
  { key: 'moving_average', name: 'MA' },
  { key: 'ema_macd',       name: 'EMA/MACD' },
  { key: 'lightgbm',       name: 'LightGBM' },
  { key: 'ensemble',       name: 'Ensemble' },
];

const MARKET_PRED_ENDPOINT: Record<string, string> = {
  gold:      '/api/gold/predictions/latest',
  nasdaq100: '/api/nasdaq/predictions?limit=40',
  crypto:    '/api/crypto/predictions?limit=40',
  sp500:     '/api/sp500/predictions?limit=40',
};

const MARKET_SIM_KEY: Record<string, string> = {
  gold:      'GOLD',
  nasdaq100: 'NASDAQ100',
  crypto:    'CRYPTO',
  sp500:     'SP500',
};

const MARKET_SSE_KEY: Record<string, string> = {
  gold:      'GOLD',
  nasdaq100: 'NASDAQ100',
  crypto:    'CRYPTO',
  sp500:     'SP500',
};

function tsNow(): string {
  return new Date().toTimeString().slice(0, 8);
}

function persistReport(report: PipelineReport): void {
  try {
    const existing: PipelineReport[] = JSON.parse(localStorage.getItem('vns_pipeline_reports') || '[]');
    localStorage.setItem('vns_pipeline_reports', JSON.stringify([report, ...existing].slice(0, 30)));
  } catch { /* ignore */ }
}

// ── Pipeline modal ────────────────────────────────────────────────────────────
function PipelineModal({
  open, market, marketRoute, icon, steps, stepStates,
  logs, algoResults, predictions, botResults, running, onClose, onNavigate,
}: {
  open: boolean;
  market: string;
  marketRoute: string;
  icon: string;
  steps: PipelineStep[];
  stepStates: StepState[];
  logs: LogEntry[];
  algoResults: AlgoResult[];
  predictions: any[];
  botResults: BotResult[];
  running: boolean;
  onClose: () => void;
  onNavigate: () => void;
}) {
  const logRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, [logs.length]);

  if (!open) return null;

  const allDone  = stepStates.every(s => s === 'ok');
  const hasError = stepStates.some(s => s === 'error');
  const done     = !running && (allDone || hasError);

  return (
    <div
      style={{
        position: 'fixed', inset: 0, zIndex: 9999,
        background: 'rgba(0,0,0,0.65)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        padding: 20,
      }}
      onClick={(e) => { if (e.target === e.currentTarget && done) onClose(); }}
    >
      <div style={{
        background: 'var(--surface)',
        border: '1px solid var(--border)',
        borderRadius: 12,
        width: '100%', maxWidth: 680, maxHeight: '88vh',
        display: 'flex', flexDirection: 'column',
        overflow: 'hidden',
        boxShadow: '0 20px 60px rgba(0,0,0,0.5)',
      }}>
        {/* Header */}
        <div style={{
          padding: '14px 18px', borderBottom: '1px solid var(--border)',
          display: 'flex', alignItems: 'center', gap: 10, flexShrink: 0,
        }}>
          <Icon name={icon} size={16} />
          <span style={{ fontWeight: 700, fontSize: 15, flex: 1 }}>Pipeline: {market}</span>
          <span style={{
            fontSize: 11, fontFamily: 'var(--font-mono)', padding: '2px 8px',
            borderRadius: 4, border: '1px solid',
            ...(running
              ? { color: 'var(--accent)',  borderColor: 'var(--accent)',  background: 'rgba(91,141,239,0.1)' }
              : allDone
              ? { color: 'var(--up)',      borderColor: 'var(--up)',      background: 'rgba(47,181,124,0.1)' }
              : hasError
              ? { color: 'var(--down)',    borderColor: 'var(--down)',    background: 'rgba(220,60,60,0.1)' }
              : { color: 'var(--text-3)', borderColor: 'var(--border)',  background: 'none' }),
          }}>
            {running ? 'RUNNING' : allDone ? 'DONE' : hasError ? 'ERROR' : 'IDLE'}
          </span>
          {done && (
            <button
              onClick={onClose}
              style={{
                background: 'none', border: 'none', cursor: 'pointer',
                color: 'var(--text-3)', fontSize: 20, lineHeight: 1, padding: '0 4px',
              }}
            >x</button>
          )}
        </div>

        {/* Steps progress */}
        <div style={{
          padding: '10px 18px', borderBottom: '1px solid var(--border)',
          display: 'flex', gap: 6, flexWrap: 'wrap', flexShrink: 0,
        }}>
          {steps.map((step, i) => (
            <div key={step.endpoint} style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              {i > 0 && <span style={{ color: 'var(--border-strong)', fontSize: 10, margin: '0 2px' }}>›</span>}
              <div style={{
                display: 'flex', alignItems: 'center', gap: 5,
                padding: '4px 10px', borderRadius: 20, fontSize: 12,
                transition: 'all 0.2s',
                background: stepStates[i] === 'ok'      ? 'rgba(47,181,124,0.1)'
                           : stepStates[i] === 'error'   ? 'rgba(220,60,60,0.1)'
                           : stepStates[i] === 'running'  ? 'rgba(91,141,239,0.1)'
                           : 'var(--surface-2)',
                border: '1px solid ' + (
                  stepStates[i] === 'ok'      ? 'var(--up)'
                  : stepStates[i] === 'error'  ? 'var(--down)'
                  : stepStates[i] === 'running' ? 'var(--accent)'
                  : 'var(--border)'
                ),
                color: stepStates[i] === 'ok'      ? 'var(--up)'
                      : stepStates[i] === 'error'   ? 'var(--down)'
                      : stepStates[i] === 'running'  ? 'var(--accent)'
                      : 'var(--text-3)',
              }}>
                <span style={{ fontSize: 10 }}>
                  {stepStates[i] === 'idle'    ? '○'
                  : stepStates[i] === 'running' ? '●'
                  : stepStates[i] === 'ok'      ? '✓'
                  :                               '✗'}
                </span>
                {step.label}
              </div>
            </div>
          ))}
        </div>

        {/* Scrollable body */}
        <div style={{ flex: 1, overflow: 'auto', padding: '14px 18px', display: 'flex', flexDirection: 'column', gap: 16 }}>

          {/* Logs */}
          <div>
            <div style={{ fontSize: 10, color: 'var(--text-3)', textTransform: 'uppercase', letterSpacing: 0.6, fontWeight: 600, marginBottom: 6 }}>
              Logs ung dung
            </div>
            <div
              ref={logRef}
              style={{
                background: 'var(--surface-2)', border: '1px solid var(--border)', borderRadius: 6,
                padding: '8px 10px', height: 180, overflowY: 'auto',
                fontFamily: 'var(--font-mono)', fontSize: 11, lineHeight: 1.65,
              }}
            >
              {logs.length === 0
                ? <span style={{ color: 'var(--text-3)' }}>Cho bat dau...</span>
                : logs.map((log, i) => (
                  <div key={i} style={{ display: 'flex', gap: 8 }}>
                    <span style={{ color: 'var(--text-3)', minWidth: 64, flexShrink: 0 }}>{log.ts}</span>
                    <span style={{
                      minWidth: 48, flexShrink: 0, fontWeight: 600,
                      color: log.level === 'ok'    ? 'var(--up)'
                           : log.level === 'error'  ? 'var(--down)'
                           : log.level === 'warn'   ? '#C9A23F'
                           : 'var(--accent)',
                    }}>
                      {log.level === 'ok' ? '[ OK ]' : log.level === 'error' ? '[ERR]' : log.level === 'warn' ? '[WARN]' : '[INFO]'}
                    </span>
                    <span style={{ color: 'var(--text-2)', flex: 1, wordBreak: 'break-word' }}>{log.msg}</span>
                  </div>
                ))
              }
            </div>
          </div>

          {/* Algorithm results */}
          {algoResults.length > 0 && (
            <div>
              <div style={{ fontSize: 10, color: 'var(--text-3)', textTransform: 'uppercase', letterSpacing: 0.6, fontWeight: 600, marginBottom: 8 }}>
                Ket qua thuat toan
              </div>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                {algoResults.map((a) => (
                  <div key={a.key} style={{
                    display: 'flex', alignItems: 'center', gap: 5,
                    padding: '5px 12px', borderRadius: 6, fontSize: 12,
                    background: a.status === 'ok' ? 'rgba(47,181,124,0.08)' : 'rgba(220,60,60,0.05)',
                    border: '1px solid ' + (a.status === 'ok' ? 'rgba(47,181,124,0.3)' : 'rgba(220,60,60,0.2)'),
                  }}>
                    <span style={{ color: a.status === 'ok' ? 'var(--up)' : 'var(--down)', fontWeight: 700, fontSize: 11 }}>
                      {a.status === 'ok' ? '✓' : '✗'}
                    </span>
                    <span style={{ color: a.status === 'ok' ? 'var(--text-1)' : 'var(--text-3)', fontWeight: 500 }}>
                      {a.name}
                    </span>
                    {a.count > 0 && (
                      <span style={{ color: 'var(--text-3)', fontSize: 10 }}>({a.count})</span>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Predictions preview */}
          {predictions.length > 0 && (
            <div>
              <div style={{ fontSize: 10, color: 'var(--text-3)', textTransform: 'uppercase', letterSpacing: 0.6, fontWeight: 600, marginBottom: 8 }}>
                Du doan moi nhat ({predictions.length} ket qua)
              </div>
              <div style={{ overflowX: 'auto', border: '1px solid var(--border)', borderRadius: 6 }}>
                <table className="tbl" style={{ fontSize: 11, width: '100%' }}>
                  <thead>
                    <tr>
                      <th>Symbol</th>
                      <th>Thuat toan</th>
                      <th className="r">Gia hien tai</th>
                      <th className="r">Du doan</th>
                      <th className="r">Tin cay</th>
                    </tr>
                  </thead>
                  <tbody>
                    {predictions.slice(0, 8).map((p, i) => {
                      const sym  = (p.stock && p.stock.symbol) || p.symbol || p.source || '—';
                      const cur  = Number(p.current_price || 0);
                      const pred = Number(p.predicted_price || 0);
                      const chg  = cur > 0 ? ((pred - cur) / cur * 100) : 0;
                      const confRaw = Number(p.confidence ?? -1);
                      const confPct = confRaw < 0 ? null : confRaw <= 1 ? Math.round(confRaw * 100) : Math.round(confRaw);
                      return (
                        <tr key={i}>
                          <td className="sym">{sym}</td>
                          <td style={{ fontFamily: 'var(--font-mono)', fontSize: 10, color: 'var(--text-2)' }}>{p.algorithm_name || '—'}</td>
                          <td className="r num">{cur > 0 ? cur.toLocaleString() : '—'}</td>
                          <td className="r num" style={{ color: chg > 0 ? 'var(--up)' : chg < 0 ? 'var(--down)' : undefined }}>
                            {pred > 0 ? pred.toLocaleString() : '—'}
                          </td>
                          <td className="r num" style={{ color: 'var(--text-3)' }}>
                            {confPct != null ? confPct + '%' : '—'}
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {/* Bot trading results */}
          {botResults.length > 0 && (
            <div>
              <div style={{ fontSize: 10, color: 'var(--text-3)', textTransform: 'uppercase', letterSpacing: 0.6, fontWeight: 600, marginBottom: 8 }}>
                Bot giao dich ({botResults.length} bot)
              </div>
              <div style={{ overflowX: 'auto', border: '1px solid var(--border)', borderRadius: 6 }}>
                <table className="tbl" style={{ fontSize: 11, width: '100%' }}>
                  <thead>
                    <tr>
                      <th>Bot</th>
                      <th style={{ fontFamily: 'var(--font-mono)' }}>Algo</th>
                      <th className="r">Return</th>
                      <th className="r">Win Rate</th>
                      <th className="r">Sharpe</th>
                      <th className="r">Trades</th>
                      <th className="c">Status</th>
                    </tr>
                  </thead>
                  <tbody>
                    {botResults.slice(0, 8).map((b, i) => {
                      const ret    = b.kpis?.total_return_pct;
                      const win    = b.kpis?.win_rate_pct;
                      const sharpe = b.kpis?.sharpe_ratio;
                      const trades = b.kpis?.total_trades;
                      return (
                        <tr key={i}>
                          <td style={{ fontSize: 11, maxWidth: 160, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                            {b.display_name}
                          </td>
                          <td style={{ fontFamily: 'var(--font-mono)', fontSize: 10, color: 'var(--text-2)' }}>{b.algorithm}</td>
                          <td className="r num" style={{
                            color: ret != null ? (ret > 0 ? 'var(--up)' : ret < 0 ? 'var(--down)' : undefined) : undefined,
                            fontWeight: 600,
                          }}>
                            {ret != null ? (ret > 0 ? '+' : '') + ret.toFixed(1) + '%' : '—'}
                          </td>
                          <td className="r num">{win != null ? win.toFixed(0) + '%' : '—'}</td>
                          <td className="r num">{sharpe != null ? sharpe.toFixed(2) : '—'}</td>
                          <td className="r num" style={{ color: 'var(--text-3)' }}>{trades ?? '—'}</td>
                          <td className="c">
                            <span style={{
                              fontSize: 10, padding: '1px 7px', borderRadius: 3,
                              background: b.is_active ? 'rgba(47,181,124,0.1)' : 'var(--surface-2)',
                              color: b.is_active ? 'var(--up)' : 'var(--text-3)',
                              border: '1px solid ' + (b.is_active ? 'rgba(47,181,124,0.4)' : 'var(--border)'),
                            }}>
                              {b.is_active ? 'ACTIVE' : 'OFF'}
                            </span>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div style={{
          padding: '12px 18px', borderTop: '1px solid var(--border)',
          display: 'flex', gap: 8, alignItems: 'center', flexShrink: 0,
        }}>
          <span style={{ fontSize: 11, color: 'var(--text-3)', flex: 1 }}>
            {done && (allDone
              ? '✓ Report da luu vao bo nho cuc bo'
              : '✗ Pipeline gap loi — xem logs ben tren')}
            {running && <span style={{ color: 'var(--accent)', fontFamily: 'var(--font-mono)' }}>Dang chay pipeline...</span>}
          </span>
          {done && (
            <button
              className="btn btn--ghost btn--sm"
              style={{ fontSize: 12, display: 'flex', alignItems: 'center', gap: 5 }}
              onClick={onNavigate}
            >
              Xem {market}
              <Icon name="arrowUp" size={11} style={{ transform: 'rotate(90deg)' }} />
            </button>
          )}
          {done && (
            <button className="btn btn--primary btn--sm" style={{ fontSize: 12 }} onClick={onClose}>
              OK
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

function PipelineTriggerBtn({
  label, icon, steps, marketKey, marketRoute,
}: {
  label: string;
  icon: string;
  steps: PipelineStep[];
  marketKey: string;
  marketRoute: string;
}) {
  const { t } = useLanguage();
  const navigate = useNavigate();

  const [modalOpen,   setModalOpen]   = useState(false);
  const [running,     setRunning]     = useState(false);
  const [stepStates,  setStepStates]  = useState<StepState[]>(() => steps.map(() => 'idle'));
  const [logs,        setLogs]        = useState<LogEntry[]>([]);
  const [algoResults, setAlgoResults] = useState<AlgoResult[]>([]);
  const [predictions, setPredictions] = useState<any[]>([]);
  const [botResults,  setBotResults]  = useState<BotResult[]>([]);

  const run = async () => {
    setModalOpen(true);
    setRunning(true);
    setLogs([]);
    setAlgoResults([]);
    setPredictions([]);
    setBotResults([]);
    setStepStates(steps.map(() => 'idle'));

    const startAt = Date.now();
    const logsBuffer: LogEntry[] = [];
    const addLog = (level: LogEntry['level'], msg: string) => {
      const entry = { ts: tsNow(), level, msg };
      logsBuffer.push(entry);
      setLogs([...logsBuffer]);
    };

    const finalStates: StepState[] = steps.map(() => 'idle');
    let finalPreds:   any[]        = [];
    let finalAlgos:   AlgoResult[] = [];
    let finalBots:    BotResult[]  = [];

    for (let i = 0; i < steps.length; i++) {
      finalStates[i] = 'running';
      setStepStates([...finalStates]);
      addLog('info', `Bat dau: ${steps[i].label}...`);

      try {
        // Predict step (i === 1) → SSE stream for live logs
        if (i === 1) {
          const sseMarket = MARKET_SSE_KEY[marketKey] || marketKey.toUpperCase();
          const token = getToken();
          const sseUrl = `/api/pipeline/stream?market=${encodeURIComponent(sseMarket)}&token=${encodeURIComponent(token)}`;
          await new Promise<void>((resolve, reject) => {
            const sse = new EventSource(sseUrl);
            sse.onmessage = (e) => {
              try {
                const data = JSON.parse(e.data) as { level: string; msg: string; progress: number; done: boolean; error?: string };
                if (data.msg && data.msg !== '...') {
                  addLog(data.level as LogEntry['level'], data.msg);
                }
                if (data.done) {
                  sse.close();
                  if (data.level === 'error') {
                    reject(new Error(data.error || data.msg || 'Prediction failed'));
                  } else {
                    resolve();
                  }
                }
              } catch { /* ignore parse error */ }
            };
            sse.onerror = () => {
              sse.close();
              reject(new Error('SSE connection lost'));
            };
          });
        } else {
          const msg = await triggerEndpoint(steps[i].endpoint);
          addLog('ok', `${steps[i].label}: ${msg}`);
        }
        finalStates[i] = 'ok';
        setStepStates([...finalStates]);

        // After predict step: fetch & analyze results
        if (i === 1) {
          addLog('info', 'Dang lay ket qua du doan tu server...');
          const endpoint = MARKET_PRED_ENDPOINT[marketKey] || '/api/predictions?limit=40';
          try {
            const res  = await fetch(endpoint, { headers: { Accept: 'application/json' } });
            const json = await res.json();
            const list: any[] = Array.isArray(json) ? json : (json.data || json.predictions || json.items || []);
            finalPreds = list;
            setPredictions(list);

            const byAlgo: Record<string, number> = {};
            for (const p of list) {
              const k = (p.algorithm_name || '').toLowerCase();
              byAlgo[k] = (byAlgo[k] || 0) + 1;
            }
            finalAlgos = KNOWN_ALGOS.map(a => ({
              key:    a.key,
              name:   a.name,
              status: (byAlgo[a.key] || 0) > 0 ? 'ok' : 'none',
              count:  byAlgo[a.key] || 0,
            } as AlgoResult));
            setAlgoResults(finalAlgos);

            const okCnt   = finalAlgos.filter(x => x.status === 'ok').length;
            const noneCnt = finalAlgos.filter(x => x.status === 'none').length;
            addLog('ok', `${list.length} du doan · ${okCnt} thuat toan co ket qua${noneCnt > 0 ? ` · ${noneCnt} chua co du lieu` : ''}`);
            for (const a of finalAlgos) {
              if (a.status === 'ok')   addLog('ok',   `  ${a.name}: ${a.count} du doan`);
              else                      addLog('warn', `  ${a.name}: chua co ket qua (co the dang chay nen)`);
            }
          } catch {
            addLog('warn', 'Chua lay duoc du lieu du doan (pipeline chay nen, thu lai sau)');
          }
        }

        // After bot trading step: fetch leaderboard
        if (i === 2) {
          addLog('info', 'Dang lay ket qua bot giao dich...');
          const simMarket = MARKET_SIM_KEY[marketKey] || marketKey.toUpperCase();
          try {
            const res = await fetch(`/api/simulation/leaderboard?market=${simMarket}`, {
              headers: { Accept: 'application/json' },
            });
            if (res.ok) {
              const json = await res.json();
              const bots: BotResult[] = Array.isArray(json) ? json : (json.data || json.bots || []);
              finalBots = bots;
              setBotResults(bots);
              if (bots.length > 0) {
                const activeCnt = bots.filter(b => b.is_active).length;
                addLog('ok', `${bots.length} bot · ${activeCnt} dang hoat dong`);
                for (const b of bots.slice(0, 5)) {
                  const ret = b.kpis?.total_return_pct;
                  const win = b.kpis?.win_rate_pct;
                  addLog(ret != null && ret > 0 ? 'ok' : ret != null && ret < 0 ? 'warn' : 'info',
                    `  ${b.display_name}: Return ${ret != null ? (ret > 0 ? '+' : '') + ret.toFixed(1) + '%' : '--'} · Win ${win != null ? win.toFixed(0) + '%' : '--'}`);
                }
              } else {
                addLog('warn', 'Chua co bot nao cho market nay');
              }
            } else {
              addLog('warn', 'Khong lay duoc du lieu bot');
            }
          } catch {
            addLog('warn', 'Loi khi lay du lieu bot');
          }
        }
      } catch (e: any) {
        finalStates[i] = 'error';
        setStepStates([...finalStates]);
        addLog('error', `${steps[i].label}: ${e?.message || 'Loi khong xac dinh'}`);
        break;
      }
    }

    setRunning(false);

    // Save report
    persistReport({
      id:               Date.now().toString(),
      market:           label,
      marketKey,
      timestamp:        new Date().toISOString(),
      durationMs:       Date.now() - startAt,
      steps:            steps.map((s, i) => ({ label: s.label, status: finalStates[i] })),
      logs:             logsBuffer,
      algorithms:       finalAlgos,
      botResults:       finalBots,
      predictionsCount: finalPreds.length,
      success:          finalStates.every(s => s === 'ok'),
    });
    addLog('info', 'Report da luu · bam "Xem bao cao" de xem lai');
  };

  const handleClose    = () => { if (!running) setModalOpen(false); };
  const handleNavigate = () => { setModalOpen(false); navigate(marketRoute); };

  const allDone  = stepStates.every(s => s === 'ok');
  const hasError = stepStates.some(s => s === 'error');
  const anyRan   = stepStates.some(s => s !== 'idle');

  return (
    <>
      <div style={{
        border: '1px solid var(--border)', borderRadius: 8,
        padding: '10px 12px',
        background: hasError ? 'rgba(220,60,60,0.04)' : allDone ? 'rgba(47,181,124,0.04)' : 'var(--surface-2)',
        minWidth: 150, display: 'flex', flexDirection: 'column', gap: 8,
      }}>
        <button
          className="btn btn--ghost"
          style={{
            display: 'flex', alignItems: 'center', gap: 7,
            padding: '6px 0', fontSize: 13, fontWeight: 600,
            background: 'none', border: 'none',
            cursor: running ? 'not-allowed' : 'pointer',
            color: hasError ? 'var(--down)' : allDone ? 'var(--up)' : 'var(--text-1)',
            justifyContent: 'flex-start',
          }}
          disabled={running}
          onClick={run}
        >
          <Icon name={icon} size={14} />
          {label}
        </button>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
          {steps.map((step, i) => (
            <div
              key={step.endpoint}
              style={{
                display: 'flex', alignItems: 'center', gap: 6, fontSize: 11,
                color: stepStates[i] === 'running' ? 'var(--accent)'
                     : stepStates[i] === 'ok'      ? 'var(--up)'
                     : stepStates[i] === 'error'   ? 'var(--down)'
                     : 'var(--text-3)',
              }}
            >
              <span style={{ fontSize: 10 }}>
                {stepStates[i] === 'idle'    ? '○'
                : stepStates[i] === 'running' ? '●'
                : stepStates[i] === 'ok'      ? '✓'
                :                               '✗'}
              </span>
              {step.label}
            </div>
          ))}
        </div>

        {anyRan && (
          <div style={{ fontSize: 11, marginTop: 2, display: 'flex', flexDirection: 'column', gap: 2 }}>
            {allDone  && <span style={{ color: 'var(--up)' }}>✓ {t.settings.done}</span>}
            {hasError && <span style={{ color: 'var(--down)' }}>✗ Gap loi</span>}
            {running  && <span style={{ color: 'var(--accent)' }}>{t.settings.running}</span>}
            {!running && (
              <button
                style={{
                  background: 'none', border: 'none', cursor: 'pointer',
                  fontSize: 10, color: 'var(--accent)', textAlign: 'left', padding: 0,
                }}
                onClick={() => setModalOpen(true)}
              >
                Xem bao cao →
              </button>
            )}
          </div>
        )}
      </div>

      {modalOpen && (
        <PipelineModal
          open={modalOpen}
          market={label}
          marketRoute={marketRoute}
          icon={icon}
          steps={steps}
          stepStates={stepStates}
          logs={logs}
          algoResults={algoResults}
          predictions={predictions}
          botResults={botResults}
          running={running}
          onClose={handleClose}
          onNavigate={handleNavigate}
        />
      )}
    </>
  );
}

// ── Backup panel ──────────────────────────────────────────────────────────────

interface BackupFile {
  filename:   string;
  size:       number;
  created_at: string;
  size_human: string;
}

function BackupPanel() {
  const { user } = useAuth();
  const isAdmin = user?.role === 'admin';

  const [backups,       setBackups]       = useState<BackupFile[]>([]);
  const [loadingList,   setLoadingList]   = useState(false);
  const [listError,     setListError]     = useState('');
  const [backing,       setBacking]       = useState(false);
  const [backupMsg,     setBackupMsg]     = useState<{ ok: boolean; text: string } | null>(null);
  const [deletingFile,  setDeletingFile]  = useState<string | null>(null);
  const [downloadingFile, setDownloadingFile] = useState<string | null>(null);

  const loadBackups = async () => {
    setLoadingList(true);
    setListError('');
    try {
      const res = await fetch('/api/backups', {
        headers: { Authorization: `Bearer ${getToken()}` },
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.message || `HTTP ${res.status}`);
      }
      const data = await res.json();
      setBackups(Array.isArray(data) ? data : []);
    } catch (e: any) {
      setListError(e?.message || 'Không tải được danh sách backup');
    } finally {
      setLoadingList(false);
    }
  };

  useEffect(() => { loadBackups(); }, []);

  const triggerBackup = async () => {
    setBacking(true);
    setBackupMsg(null);
    try {
      const res = await fetch('/api/trigger/backup', {
        method: 'POST',
        headers: { Authorization: `Bearer ${getToken()}` },
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.message || `HTTP ${res.status}`);
      setBackupMsg({ ok: true, text: data.message || `Đã tạo: ${data.filename || ''}` });
      await loadBackups();
    } catch (e: any) {
      setBackupMsg({ ok: false, text: e?.message || 'Backup thất bại' });
    } finally {
      setBacking(false);
      setTimeout(() => setBackupMsg(null), 6000);
    }
  };

  const deleteBackup = async (filename: string) => {
    if (!window.confirm(`Xóa file "${filename}"?`)) return;
    setDeletingFile(filename);
    try {
      const res = await fetch(`/api/backups/${encodeURIComponent(filename)}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${getToken()}` },
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.message || `HTTP ${res.status}`);
      }
      setBackups(prev => prev.filter(b => b.filename !== filename));
    } catch (e: any) {
      alert(`Lỗi xóa: ${e?.message}`);
    } finally {
      setDeletingFile(null);
    }
  };

  const downloadBackup = async (filename: string) => {
    setDownloadingFile(filename);
    try {
      const res = await fetch(`/api/backups/${encodeURIComponent(filename)}`, {
        headers: { Authorization: `Bearer ${getToken()}` },
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.message || `HTTP ${res.status}`);
      }
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = filename;
      a.click();
      URL.revokeObjectURL(url);
    } catch (e: any) {
      alert(`Lỗi tải: ${e?.message}`);
    } finally {
      setDownloadingFile(null);
    }
  };

  const fmtDate = (iso: string) => {
    try {
      const d = new Date(iso);
      const dd = ('0' + d.getDate()).slice(-2);
      const mm = ('0' + (d.getMonth() + 1)).slice(-2);
      const yy = d.getFullYear();
      const hh = ('0' + d.getHours()).slice(-2);
      const mi = ('0' + d.getMinutes()).slice(-2);
      return `${dd}/${mm}/${yy} ${hh}:${mi}`;
    } catch { return iso.slice(0, 16); }
  };

  return (
    <Panel
      title="Sao lưu cơ sở dữ liệu"
      tools={
        <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
          <button
            className="btn btn--ghost btn--icon"
            onClick={loadBackups}
            disabled={loadingList}
            title="Tải lại danh sách"
          >
            <Icon name="refresh" size={14} />
          </button>
          {isAdmin && (
            <button
              className="btn btn--primary btn--sm"
              style={{ fontSize: 12, display: 'flex', alignItems: 'center', gap: 5, padding: '5px 12px' }}
              onClick={triggerBackup}
              disabled={backing}
            >
              <Icon name="cpu" size={12} />
              {backing ? 'Đang backup...' : 'Backup ngay'}
            </button>
          )}
        </div>
      }
    >
      {/* Backup status message */}
      {backupMsg && (
        <div style={{
          margin: '0 0 10px',
          padding: '8px 12px',
          borderRadius: 6,
          fontSize: 12,
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          background: backupMsg.ok ? 'rgba(47,181,124,0.1)' : 'rgba(220,60,60,0.1)',
          border: `1px solid ${backupMsg.ok ? 'rgba(47,181,124,0.3)' : 'rgba(220,60,60,0.3)'}`,
          color: backupMsg.ok ? 'var(--up)' : 'var(--down)',
        }}>
          {backupMsg.ok ? '✓' : '✗'} {backupMsg.text}
        </div>
      )}

      {/* Error state */}
      {listError && (
        <div style={{ padding: '12px', color: 'var(--down)', fontSize: 13 }}>{listError}</div>
      )}

      {/* Loading state */}
      {loadingList && backups.length === 0 ? (
        <div style={{ padding: '24px', color: 'var(--text-3)', fontSize: 13, textAlign: 'center' }}>
          Đang tải danh sách backup...
        </div>
      ) : !listError && backups.length === 0 ? (
        <div style={{ padding: '24px', color: 'var(--text-3)', fontSize: 13, textAlign: 'center' }}>
          Chưa có file backup nào. Nhấn "Backup ngay" để tạo bản sao lưu đầu tiên.
        </div>
      ) : (
        <div style={{ overflowX: 'auto' }}>
          <table className="tbl" style={{ width: '100%' }}>
            <thead>
              <tr>
                <th style={{ padding: '8px 12px', fontSize: 12, color: 'var(--text-3)' }}>Tên file</th>
                <th style={{ padding: '8px 12px', fontSize: 12, color: 'var(--text-3)', textAlign: 'right' }}>Kích thước</th>
                <th style={{ padding: '8px 12px', fontSize: 12, color: 'var(--text-3)' }}>Ngày tạo</th>
                <th style={{ padding: '8px 12px', fontSize: 12, color: 'var(--text-3)' }}></th>
              </tr>
            </thead>
            <tbody>
              {backups.map((b) => (
                <tr key={b.filename}>
                  <td style={{ padding: '10px 12px', fontSize: 12, fontFamily: 'var(--font-mono)', color: 'var(--text-2)', wordBreak: 'break-all' }}>
                    {b.filename}
                  </td>
                  <td style={{ padding: '10px 12px', fontSize: 12, fontFamily: 'var(--font-mono)', textAlign: 'right', color: 'var(--text-3)', whiteSpace: 'nowrap' }}>
                    {b.size_human || `${b.size} B`}
                  </td>
                  <td style={{ padding: '10px 12px', fontSize: 12, color: 'var(--text-3)', whiteSpace: 'nowrap' }}>
                    {fmtDate(b.created_at)}
                  </td>
                  <td style={{ padding: '10px 12px' }}>
                    <div style={{ display: 'flex', gap: 6, justifyContent: 'flex-end' }}>
                      <button
                        className="btn btn--ghost btn--sm"
                        style={{ fontSize: 11, display: 'flex', alignItems: 'center', gap: 4, padding: '4px 10px' }}
                        onClick={() => downloadBackup(b.filename)}
                        disabled={downloadingFile === b.filename}
                        title="Tải xuống"
                      >
                        <Icon name="arrowUp" size={11} style={{ transform: 'rotate(180deg)' }} />
                        {downloadingFile === b.filename ? '...' : 'Tải xuống'}
                      </button>
                      {isAdmin && (
                        <button
                          className="btn btn--ghost btn--sm"
                          style={{ fontSize: 11, color: 'var(--down)', display: 'flex', alignItems: 'center', gap: 4, padding: '4px 10px' }}
                          onClick={() => deleteBackup(b.filename)}
                          disabled={deletingFile === b.filename}
                          title="Xóa"
                        >
                          {deletingFile === b.filename ? '...' : 'Xóa'}
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Panel>
  );
}

// ── Server pipeline report types ──────────────────────────────────────────────
interface ServerPipelineReport {
  id: number;
  pipeline_key: string;
  market: string;
  status: 'success' | 'partial' | 'failed' | 'skipped';
  started_at: string;
  finished_at: string;
  duration_ms: number;
  crawled_count: number;
  predictions_count: number;
  trained: boolean;
  steps: { label: string; status: string; detail?: string }[];
  error: string;
}

// ── Helpers shared by server report components ─────────────────────────────────
function _fmtTs(iso: string): string {
  try {
    const d = new Date(iso);
    const dd = ('0' + d.getDate()).slice(-2);
    const mm = ('0' + (d.getMonth() + 1)).slice(-2);
    const hh = ('0' + d.getHours()).slice(-2);
    const mi = ('0' + d.getMinutes()).slice(-2);
    return `${dd}/${mm} ${hh}:${mi}`;
  } catch { return iso.slice(0, 16); }
}

function _fmtDuration(ms: number): string {
  if (!ms) return '';
  if (ms < 1000) return ms + 'ms';
  if (ms < 60000) return (ms / 1000).toFixed(0) + 's';
  return Math.floor(ms / 60000) + 'm' + Math.floor((ms % 60000) / 1000) + 's';
}

const PIPELINE_LABEL: Record<string, string> = {
  crawler_gold:   'Gold',
  crawler_nasdaq: 'NASDAQ',
  crawler_sp500:  'S&P 500',
  crawler_crypto: 'Crypto',
};

function _pipelineLabel(key: string): string {
  return PIPELINE_LABEL[key] || key;
}

const MARKET_TO_ICON: Record<string, string> = {
  GOLD:      'gold',
  NASDAQ:    'nasdaq',
  NASDAQ100: 'nasdaq',
  SP500:     'pulse',
  CRYPTO:    'crypto',
};

function _marketIcon(market: string): string {
  return MARKET_TO_ICON[market.toUpperCase()] || 'pulse';
}

// ── Server report detail modal ─────────────────────────────────────────────────
function ServerReportDetailModal({ report, onClose }: { report: ServerPipelineReport; onClose: () => void }) {
  const statusBadge = (s: string) => {
    const cfg: Record<string, { bg: string; border: string; color: string; label: string }> = {
      success: { bg: 'rgba(47,181,124,0.1)',  border: 'rgba(47,181,124,0.35)', color: 'var(--up)',   label: 'OK'      },
      partial: { bg: 'rgba(201,162,63,0.12)', border: 'rgba(201,162,63,0.4)',  color: '#C9A23F',     label: 'PARTIAL' },
      failed:  { bg: 'rgba(220,60,60,0.1)',   border: 'rgba(220,60,60,0.35)', color: 'var(--down)', label: 'ERR'     },
      skipped: { bg: 'var(--surface-2)',       border: 'var(--border)',         color: 'var(--text-3)', label: 'SKIP'  },
    };
    const c = cfg[s] || cfg.skipped;
    return (
      <span style={{
        fontSize: 10, fontFamily: 'var(--font-mono)', padding: '2px 8px',
        borderRadius: 4, border: `1px solid ${c.border}`,
        background: c.bg, color: c.color,
      }}>
        {c.label}
      </span>
    );
  };

  const stepStatusBadge = (s: string) => {
    const colors: Record<string, string> = { success: 'var(--up)', ok: 'var(--up)', failed: 'var(--down)', error: 'var(--down)', skipped: 'var(--text-3)' };
    return (
      <span style={{ fontSize: 10, color: colors[s] || 'var(--text-3)', fontWeight: 600 }}>
        {s === 'success' || s === 'ok' ? '✓' : s === 'failed' || s === 'error' ? '✗' : '○'}
      </span>
    );
  };

  return (
    <div
      style={{
        position: 'fixed', inset: 0, zIndex: 10000,
        background: 'rgba(0,0,0,0.7)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        padding: 20,
      }}
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div style={{
        background: 'var(--surface)',
        border: '1px solid var(--border)',
        borderRadius: 12,
        width: '100%', maxWidth: 640, maxHeight: '88vh',
        display: 'flex', flexDirection: 'column',
        overflow: 'hidden',
        boxShadow: '0 20px 60px rgba(0,0,0,0.5)',
      }}>
        {/* Header */}
        <div style={{
          padding: '14px 18px', borderBottom: '1px solid var(--border)',
          display: 'flex', alignItems: 'center', gap: 10, flexShrink: 0,
        }}>
          <Icon name={_marketIcon(report.market)} size={16} />
          <span style={{ fontWeight: 700, fontSize: 15, flex: 1 }}>
            Báo cáo: {_pipelineLabel(report.pipeline_key)} — {report.market}
          </span>
          {statusBadge(report.status)}
          <button
            onClick={onClose}
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--text-3)', fontSize: 20, lineHeight: 1, padding: '0 4px' }}
          >×</button>
        </div>

        {/* Scrollable body */}
        <div style={{ flex: 1, overflow: 'auto', padding: '16px 18px', display: 'flex', flexDirection: 'column', gap: 14 }}>

          {/* Meta fields */}
          <div style={{
            display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '8px 16px',
            background: 'var(--surface-2)', borderRadius: 8, padding: '12px 14px',
            border: '1px solid var(--border)',
          }}>
            <div>
              <div style={{ fontSize: 10, color: 'var(--text-3)', textTransform: 'uppercase', letterSpacing: 0.5, marginBottom: 3 }}>Bắt đầu</div>
              <div style={{ fontSize: 12, fontFamily: 'var(--font-mono)', color: 'var(--text-1)' }}>{_fmtTs(report.started_at)}</div>
            </div>
            <div>
              <div style={{ fontSize: 10, color: 'var(--text-3)', textTransform: 'uppercase', letterSpacing: 0.5, marginBottom: 3 }}>Kết thúc</div>
              <div style={{ fontSize: 12, fontFamily: 'var(--font-mono)', color: 'var(--text-1)' }}>{_fmtTs(report.finished_at)}</div>
            </div>
            <div>
              <div style={{ fontSize: 10, color: 'var(--text-3)', textTransform: 'uppercase', letterSpacing: 0.5, marginBottom: 3 }}>Thời lượng</div>
              <div style={{ fontSize: 12, fontFamily: 'var(--font-mono)', color: 'var(--text-1)' }}>{_fmtDuration(report.duration_ms) || '—'}</div>
            </div>
            <div>
              <div style={{ fontSize: 10, color: 'var(--text-3)', textTransform: 'uppercase', letterSpacing: 0.5, marginBottom: 3 }}>Đã train</div>
              <div style={{ fontSize: 12, fontFamily: 'var(--font-mono)', color: report.trained ? 'var(--up)' : 'var(--text-3)' }}>
                {report.trained ? 'Có' : 'Không'}
              </div>
            </div>
            <div>
              <div style={{ fontSize: 10, color: 'var(--text-3)', textTransform: 'uppercase', letterSpacing: 0.5, marginBottom: 3 }}>Dữ liệu crawl</div>
              <div style={{ fontSize: 12, fontFamily: 'var(--font-mono)', color: 'var(--text-1)' }}>{report.crawled_count}</div>
            </div>
            <div>
              <div style={{ fontSize: 10, color: 'var(--text-3)', textTransform: 'uppercase', letterSpacing: 0.5, marginBottom: 3 }}>Dự đoán</div>
              <div style={{ fontSize: 12, fontFamily: 'var(--font-mono)', color: 'var(--text-1)' }}>{report.predictions_count}</div>
            </div>
          </div>

          {/* Steps */}
          {report.steps && report.steps.length > 0 && (
            <div>
              <div style={{ fontSize: 10, color: 'var(--text-3)', textTransform: 'uppercase', letterSpacing: 0.6, fontWeight: 600, marginBottom: 8 }}>
                Các bước
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                {report.steps.map((step, i) => (
                  <div key={i} style={{
                    display: 'flex', alignItems: 'flex-start', gap: 8,
                    padding: '7px 10px', borderRadius: 6,
                    background: 'var(--surface-2)', border: '1px solid var(--border)',
                  }}>
                    <div style={{ paddingTop: 1, flexShrink: 0 }}>{stepStatusBadge(step.status)}</div>
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <span style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-1)' }}>{step.label}</span>
                      {step.detail && (
                        <span style={{ fontSize: 11, color: 'var(--text-3)', marginLeft: 8, fontFamily: 'var(--font-mono)' }}>
                          {step.detail}
                        </span>
                      )}
                    </div>
                    <span style={{
                      fontSize: 10, fontFamily: 'var(--font-mono)', padding: '1px 6px', borderRadius: 3, flexShrink: 0,
                      background: step.status === 'success' || step.status === 'ok' ? 'rgba(47,181,124,0.1)'
                               : step.status === 'failed' || step.status === 'error' ? 'rgba(220,60,60,0.1)'
                               : 'var(--surface-2)',
                      color: step.status === 'success' || step.status === 'ok' ? 'var(--up)'
                           : step.status === 'failed' || step.status === 'error' ? 'var(--down)'
                           : 'var(--text-3)',
                      border: '1px solid ' + (
                        step.status === 'success' || step.status === 'ok' ? 'rgba(47,181,124,0.3)'
                        : step.status === 'failed' || step.status === 'error' ? 'rgba(220,60,60,0.3)'
                        : 'var(--border)'
                      ),
                    }}>
                      {step.status}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Error */}
          {report.error && (
            <div style={{
              padding: '10px 12px', borderRadius: 6, fontSize: 12,
              background: 'rgba(220,60,60,0.08)', border: '1px solid rgba(220,60,60,0.3)',
              color: 'var(--down)', fontFamily: 'var(--font-mono)', wordBreak: 'break-word',
            }}>
              {report.error}
            </div>
          )}
        </div>

        {/* Footer */}
        <div style={{
          padding: '12px 18px', borderTop: '1px solid var(--border)',
          display: 'flex', gap: 8, alignItems: 'center', flexShrink: 0,
        }}>
          <span style={{ fontSize: 11, color: 'var(--text-3)', flex: 1 }}>
            {report.status === 'skipped' ? 'Market đóng cửa — pipeline bị bỏ qua.' : ''}
          </span>
          <button className="btn btn--primary btn--sm" style={{ fontSize: 12 }} onClick={onClose}>
            Đóng
          </button>
        </div>
      </div>
    </div>
  );
}

// ── Past reports panel ────────────────────────────────────────────────────────
function PastReportsPanel() {
  const [reports,   setReports]   = useState<ServerPipelineReport[]>([]);
  const [pipelines, setPipelines] = useState<string[]>([]);
  const [filter,    setFilter]    = useState('');
  const [selected,  setSelected]  = useState<ServerPipelineReport | null>(null);
  const [expanded,  setExpanded]  = useState(false);
  const [loading,   setLoading]   = useState(false);

  const loadReports = async () => {
    setLoading(true);
    try {
      let url = '/api/pipeline-reports?limit=100';
      if (filter) url += `&pipeline=${encodeURIComponent(filter)}`;
      const res = await fetch(url, {
        headers: { Authorization: `Bearer ${localStorage.getItem('vns_token') || ''}` },
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const json = await res.json();
      setReports(Array.isArray(json.data) ? json.data : []);
      if (Array.isArray(json.pipelines)) setPipelines(json.pipelines);
    } catch {
      setReports([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { loadReports(); }, [filter]);

  const displayList = expanded ? reports : reports.slice(0, 3);

  return (
    <>
      <Panel
        title={`Báo cáo pipeline${reports.length > 0 && !expanded ? ` (${reports.length})` : ''}`}
        tools={
          <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
            {/* Pipeline filter dropdown */}
            <select
              className="sel"
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              style={{ fontSize: 11, padding: '3px 7px', height: 28 }}
            >
              <option value="">Tất cả pipeline</option>
              {pipelines.map((p) => (
                <option key={p} value={p}>{_pipelineLabel(p)}</option>
              ))}
            </select>
            <button
              className="btn btn--ghost btn--icon"
              onClick={loadReports}
              disabled={loading}
              title="Tải lại"
            >
              <Icon name="refresh" size={14} />
            </button>
            <button
              className="btn btn--ghost btn--sm"
              style={{ fontSize: 11 }}
              onClick={() => setExpanded(e => !e)}
            >
              {expanded ? 'Thu gọn' : 'Xem tất cả'}
            </button>
          </div>
        }
      >
        {loading && reports.length === 0 ? (
          <div style={{ padding: '20px', color: 'var(--text-3)', fontSize: 13, textAlign: 'center' }}>
            Đang tải...
          </div>
        ) : reports.length === 0 ? (
          <div style={{ padding: '20px', color: 'var(--text-3)', fontSize: 13, textAlign: 'center' }}>
            Chưa có báo cáo nào. Pipeline sẽ tự tạo báo cáo khi chạy.
          </div>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 0 }}>
            {displayList.map((r, i) => {
              const statusCfg: Record<string, { bg: string; border: string; color: string; label: string }> = {
                success: { bg: 'rgba(47,181,124,0.1)',  border: 'rgba(47,181,124,0.3)',  color: 'var(--up)',   label: 'OK'      },
                partial: { bg: 'rgba(201,162,63,0.12)', border: 'rgba(201,162,63,0.35)', color: '#C9A23F',     label: 'PARTIAL' },
                failed:  { bg: 'rgba(220,60,60,0.1)',   border: 'rgba(220,60,60,0.3)',   color: 'var(--down)', label: 'ERR'     },
                skipped: { bg: 'var(--surface-2)',       border: 'var(--border)',          color: 'var(--text-3)', label: 'SKIP'  },
              };
              const sc = statusCfg[r.status] || statusCfg.skipped;
              const isLast = i === displayList.length - 1;
              return (
                <div
                  key={r.id}
                  style={{
                    display: 'flex', alignItems: 'center', gap: 10,
                    padding: '10px 14px',
                    borderBottom: !isLast ? '1px solid var(--border)' : undefined,
                    cursor: 'pointer',
                    background: 'transparent',
                    transition: 'background 0.15s',
                  }}
                  onClick={() => setSelected(r)}
                  onMouseEnter={e => (e.currentTarget.style.background = 'var(--surface-2)')}
                  onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
                >
                  <Icon name={_marketIcon(r.market)} size={14} style={{ color: 'var(--text-2)', flexShrink: 0 }} />
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontSize: 13, fontWeight: 500 }}>
                      {_pipelineLabel(r.pipeline_key)} · {r.market}
                    </div>
                    <div style={{ fontSize: 11, color: 'var(--text-3)', fontFamily: 'var(--font-mono)', marginTop: 1 }}>
                      {_fmtTs(r.started_at)}
                      {r.predictions_count > 0 ? ` · ${r.predictions_count} dự đoán` : ''}
                      {r.crawled_count > 0 ? ` · ${r.crawled_count} crawl` : ''}
                      {r.duration_ms > 0 ? ` · ${_fmtDuration(r.duration_ms)}` : ''}
                    </div>
                  </div>
                  <div style={{ display: 'flex', gap: 6, alignItems: 'center', flexShrink: 0 }}>
                    <span style={{
                      fontSize: 10, padding: '1px 7px', borderRadius: 3,
                      background: sc.bg, color: sc.color, border: `1px solid ${sc.border}`,
                    }}>
                      {sc.label}
                    </span>
                    <Icon name="arrowUp" size={11} style={{ color: 'var(--text-3)', transform: 'rotate(90deg)' }} />
                  </div>
                </div>
              );
            })}
            {!expanded && reports.length > 3 && (
              <div
                style={{ padding: '10px 14px', fontSize: 12, color: 'var(--accent)', cursor: 'pointer', textAlign: 'center' }}
                onClick={() => setExpanded(true)}
              >
                Xem thêm {reports.length - 3} báo cáo →
              </div>
            )}
          </div>
        )}
      </Panel>

      {selected && (
        <ServerReportDetailModal
          report={selected}
          onClose={() => setSelected(null)}
        />
      )}
    </>
  );
}

// ── Cron schedule row ─────────────────────────────────────────────────────────
function ScheduleRow({ s, onSaved }: { s: ScheduleItem; onSaved: () => void }) {
  const { t } = useLanguage();
  const [editing, setEditing] = useState(false);
  const [expr, setExpr]       = useState(s.cron_expression);
  const [enabled, setEnabled] = useState(s.enabled);
  const [saving, setSaving]   = useState(false);
  const [err, setErr]         = useState('');

  const save = async () => {
    setSaving(true);
    setErr('');
    try {
      await updateSchedule(s.job_key, expr, enabled);
      setEditing(false);
      onSaved();
    } catch (e: any) {
      setErr(e?.message || 'Lỗi');
    } finally {
      setSaving(false);
    }
  };

  const cancel = () => {
    setExpr(s.cron_expression);
    setEnabled(s.enabled);
    setEditing(false);
    setErr('');
  };

  return (
    <tr>
      {/* Job name column */}
      <td style={{ padding: '7px 10px', fontSize: 12, verticalAlign: 'middle' }}>
        <div style={{ fontWeight: 500, color: 'var(--text-1)' }}>{s.job_name}</div>
        <div style={{ fontSize: 10, color: 'var(--text-3)', marginTop: 1, fontFamily: 'var(--font-mono)' }}>
          {s.job_key}
        </div>
      </td>

      {/* Schedule column */}
      <td style={{ padding: '7px 10px', verticalAlign: 'middle' }}>
        {editing ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <ScheduleEditor expr={expr} onChange={setExpr} />
            {err && <div style={{ fontSize: 11, color: 'var(--down)' }}>{err}</div>}
          </div>
        ) : (
          <div title={s.cron_expression} style={{ cursor: 'default' }}>
            <div style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-1)' }}>
              {cronLabel(s.cron_expression, t.settings, DOW_OPTIONS)}
            </div>
            <div style={{ fontSize: 10, color: 'var(--text-3)', fontFamily: 'var(--font-mono)', marginTop: 1 }}>
              {s.cron_expression}
            </div>
          </div>
        )}
      </td>

      {/* Enabled column */}
      <td style={{ padding: '7px 10px', verticalAlign: 'middle' }}>
        {editing ? (
          <label style={{ display: 'flex', alignItems: 'center', gap: 6, cursor: 'pointer', fontSize: 12 }}>
            <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
            {enabled ? t.settings.enabled : t.settings.disabled}
          </label>
        ) : (
          <span className={`badge ${s.enabled ? 'badge--up' : 'badge--muted'}`}>
            {s.enabled ? t.settings.enabled : t.settings.disabled}
          </span>
        )}
      </td>

      {/* Actions column */}
      <td style={{ padding: '7px 10px', verticalAlign: 'middle' }}>
        {editing ? (
          <div style={{ display: 'flex', gap: 5 }}>
            <button className="btn btn--primary" style={{ fontSize: 11, padding: '4px 10px' }} disabled={saving} onClick={save}>
              {saving ? t.settings.saving : t.settings.save}
            </button>
            <button className="btn btn--ghost" style={{ fontSize: 11, padding: '4px 10px' }} onClick={cancel}>
              {t.settings.cancel}
            </button>
          </div>
        ) : (
          <button className="btn btn--ghost" style={{ fontSize: 11, padding: '4px 8px' }} onClick={() => setEditing(true)}>
            <Icon name="settings" size={11} />
          </button>
        )}
      </td>
    </tr>
  );
}

// ── Tab types ─────────────────────────────────────────────────────────────────
type SettingsTab = 'schedules' | 'triggers' | 'account' | 'backup' | 'reports';

interface TabDef {
  key: SettingsTab;
  icon: string;
  label: string;
}

// ── Main component ────────────────────────────────────────────────────────────
export default function Settings() {
  const { user } = useAuth();
  const { t } = useLanguage();

  // ── Active tab ───────────────────────────────────────────────────────────────
  const [activeTab, setActiveTab] = useState<SettingsTab>('schedules');

  // ── Password change ──────────────────────────────────────────────────────────
  const [currentPw, setCurrentPw] = useState('');
  const [newPw, setNewPw]         = useState('');
  const [confirmPw, setConfirmPw] = useState('');
  const [pwLoading, setPwLoading] = useState(false);
  const [pwError, setPwError]     = useState('');
  const [pwSuccess, setPwSuccess] = useState('');

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    setPwError('');
    setPwSuccess('');
    if (newPw !== confirmPw) { setPwError(t.settings.passwordMismatch); return; }
    if (newPw.length < 6)   { setPwError(t.settings.passwordTooShort); return; }
    setPwLoading(true);
    try {
      const res = await fetch('/api/auth/password', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${getToken()}` },
        body: JSON.stringify({ current_password: currentPw, new_password: newPw }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.message || t.settings.passwordFail);
      setPwSuccess(t.settings.passwordSuccess);
      setCurrentPw(''); setNewPw(''); setConfirmPw('');
    } catch (err: unknown) {
      setPwError(err instanceof Error ? err.message : t.settings.passwordFail);
    } finally {
      setPwLoading(false);
    }
  };

  // ── Schedules ────────────────────────────────────────────────────────────────
  const [schedules, setSchedules]       = useState<ScheduleItem[]>([]);
  const [schedLoading, setSchedLoading] = useState(false);
  const [schedError, setSchedError]     = useState('');

  const loadSchedules = async () => {
    setSchedLoading(true);
    setSchedError('');
    try {
      const data = await fetchSchedules();
      setSchedules(Array.isArray(data) ? data : []);
    } catch (e: any) {
      setSchedError(t.settings.cannotLoadSchedules + ': ' + (e?.message || ''));
    } finally {
      setSchedLoading(false);
    }
  };

  useEffect(() => { loadSchedules(); }, []);

  // ── Tab definitions ───────────────────────────────────────────────────────────
  const TABS: TabDef[] = [
    { key: 'schedules', icon: 'clock',    label: 'Lịch tác vụ'  },
    { key: 'triggers',  icon: 'pulse',    label: 'Thao tác'      },
    { key: 'account',   icon: 'user',     label: 'Tài khoản'     },
    { key: 'backup',    icon: 'download', label: 'Sao lưu'       },
    { key: 'reports',   icon: 'bar',      label: 'Báo cáo'       },
  ];

  // ── Render ────────────────────────────────────────────────────────────────────
  return (
    <div className="content__inner" style={{ display: 'flex', flexDirection: 'column', gap: 0, maxWidth: 780 }}>

      {/* ── Tab bar ── */}
      <div style={{
        display: 'flex', gap: 0,
        borderBottom: '1px solid var(--border)',
        marginBottom: 20,
        overflowX: 'auto',
      }}>
        {TABS.map(tab => {
          const active = activeTab === tab.key;
          return (
            <button
              key={tab.key}
              onClick={() => setActiveTab(tab.key)}
              style={{
                display: 'flex', alignItems: 'center', gap: 6,
                padding: '9px 16px',
                background: 'none',
                border: 'none',
                borderBottom: active ? '2px solid var(--accent)' : '2px solid transparent',
                color: active ? 'var(--accent)' : 'var(--text-2)',
                fontWeight: active ? 600 : 400,
                fontSize: 13,
                cursor: 'pointer',
                whiteSpace: 'nowrap',
                transition: 'color 0.15s',
                marginBottom: -1,
              }}
            >
              <Icon name={tab.icon} size={14} />
              {tab.label}
            </button>
          );
        })}
      </div>

      {/* ── Tab: Lịch tác vụ ── */}
      {activeTab === 'schedules' && (
        <Panel
          title={t.settings.cronSchedules}
          tools={
            <button className="btn btn--ghost btn--icon" onClick={loadSchedules} disabled={schedLoading} title="Tải lại">
              <Icon name="refresh" size={14} />
            </button>
          }
        >
          {schedError && (
            <div style={{ padding: '10px 12px', color: 'var(--down)', fontSize: 13 }}>{schedError}</div>
          )}
          {schedLoading && !schedules.length ? (
            <div style={{ padding: '20px', color: 'var(--text-3)', fontSize: 13, textAlign: 'center' }}>{t.settings.loadingSchedules}</div>
          ) : schedules.length === 0 ? (
            <div style={{ padding: '20px', color: 'var(--text-3)', fontSize: 13, textAlign: 'center' }}>
              {t.settings.noScheduleData}
            </div>
          ) : (
            <div style={{ overflowX: 'auto' }}>
              <table className="tbl" style={{ width: '100%' }}>
                <thead>
                  <tr>
                    <th style={{ padding: '6px 10px', fontSize: 11, color: 'var(--text-3)', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.04em' }}>{t.settings.colTask}</th>
                    <th style={{ padding: '6px 10px', fontSize: 11, color: 'var(--text-3)', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.04em' }}>{t.settings.colSchedule}</th>
                    <th style={{ padding: '6px 10px', fontSize: 11, color: 'var(--text-3)', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.04em' }}>{t.settings.colStatus}</th>
                    <th style={{ padding: '6px 10px', fontSize: 11, color: 'var(--text-3)', width: 60 }}></th>
                  </tr>
                </thead>
                <tbody>
                  {schedules.map((s) => (
                    <ScheduleRow key={s.job_key} s={s} onSaved={loadSchedules} />
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Panel>
      )}

      {/* ── Tab: Thao tác ── */}
      {activeTab === 'triggers' && (
        <Panel title={t.settings.manualTriggers}>
          <div style={{ fontSize: 12, color: 'var(--text-3)', marginBottom: 14 }}>
            {t.settings.manualDesc}
          </div>

          {/* Pipeline cards */}
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 10, marginBottom: 16 }}>
            <PipelineTriggerBtn label="Vàng" icon="gold" steps={[
              { label: t.settings.pipelineCrawl,      endpoint: 'gold-crawler'         },
              { label: t.settings.pipelinePredict,    endpoint: 'gold-predict'         },
              { label: t.settings.pipelineBotTrading, endpoint: 'simulation-live-step' },
            ]} marketKey="gold" marketRoute="/markets/gold" />
            <PipelineTriggerBtn label="NASDAQ" icon="nasdaq" steps={[
              { label: t.settings.pipelineCrawl,      endpoint: 'nasdaq-crawler'       },
              { label: t.settings.pipelinePredict,    endpoint: 'nasdaq-predict'       },
              { label: t.settings.pipelineBotTrading, endpoint: 'simulation-live-step' },
            ]} marketKey="nasdaq100" marketRoute="/markets/nasdaq100" />
            <PipelineTriggerBtn label="S&P 500" icon="pulse" steps={[
              { label: t.settings.pipelineCrawl,      endpoint: 'sp500-crawler'        },
              { label: t.settings.pipelinePredict,    endpoint: 'sp500-predict'        },
              { label: t.settings.pipelineBotTrading, endpoint: 'simulation-live-step' },
            ]} marketKey="sp500" marketRoute="/markets/sp500" />
            <PipelineTriggerBtn label="Crypto" icon="crypto" steps={[
              { label: t.settings.pipelineCrawl,      endpoint: 'crypto-crawler'       },
              { label: t.settings.pipelinePredict,    endpoint: 'crypto-predict'       },
              { label: t.settings.pipelineBotTrading, endpoint: 'simulation-live-step' },
            ]} marketKey="crypto" marketRoute="/markets/crypto" />
          </div>

          {/* Utility actions */}
          <div style={{ borderTop: '1px solid var(--border)', paddingTop: 12, display: 'flex', flexWrap: 'wrap', gap: 8 }}>
            <TriggerBtn label={t.settings.trainAll}  endpoint="train"       icon="cpu"     />
            <TriggerBtn label={t.settings.reconcile} endpoint="reconcile"   icon="refresh" />
          </div>
        </Panel>
      )}

      {/* ── Tab: Tài khoản (Account + Change Password merged) ── */}
      {activeTab === 'account' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <Panel title={t.settings.accountInfo}>
            {/* Avatar + username */}
            <div style={{ display: 'flex', alignItems: 'center', gap: 12, paddingBottom: 16, marginBottom: 16, borderBottom: '1px solid var(--border)' }}>
              <div style={{
                width: 44, height: 44, borderRadius: '50%',
                background: 'var(--accent)',
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                color: '#fff', fontWeight: 700, fontSize: 18,
                flexShrink: 0,
              }}>
                {user?.username?.[0]?.toUpperCase()}
              </div>
              <div>
                <div style={{ fontWeight: 600, fontSize: 15, lineHeight: 1.3 }}>{user?.username}</div>
                <div style={{ marginTop: 4 }}>
                  <span className={`badge ${user?.role === 'admin' ? 'badge--up' : 'badge--muted'}`}>
                    {user?.role}
                  </span>
                </div>
              </div>
            </div>

            {/* Change password inline */}
            <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-1)', marginBottom: 12 }}>
              {t.settings.changePassword}
            </div>
            <form onSubmit={handleChangePassword} style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
              {pwError && <div className="modal-error">{pwError}</div>}
              {pwSuccess && (
                <div style={{
                  background: 'rgba(47,181,124,0.1)', border: '1px solid rgba(47,181,124,0.3)',
                  color: '#2FB57C', padding: '7px 10px', borderRadius: 6, fontSize: 12,
                  display: 'flex', alignItems: 'center', gap: 6,
                }}>
                  <Icon name="bell" size={13} />{pwSuccess}
                </div>
              )}
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
                <div className="field" style={{ gridColumn: '1 / -1' }}>
                  <label className="field__label">{t.settings.currentPassword}</label>
                  <input className="field__input" type="password" value={currentPw}
                    onChange={e => setCurrentPw(e.target.value)} required placeholder="••••••••" autoComplete="current-password" />
                </div>
                <div className="field">
                  <label className="field__label">{t.settings.newPassword}</label>
                  <input className="field__input" type="password" value={newPw}
                    onChange={e => setNewPw(e.target.value)} required placeholder={t.settings.minChars} autoComplete="new-password" />
                </div>
                <div className="field">
                  <label className="field__label">{t.settings.confirmNewPassword}</label>
                  <input className="field__input" type="password" value={confirmPw}
                    onChange={e => setConfirmPw(e.target.value)} required placeholder="••••••••" autoComplete="new-password" />
                </div>
              </div>
              <div style={{ marginTop: 2 }}>
                <button className="btn btn--primary" type="submit" disabled={pwLoading} style={{ fontSize: 13 }}>
                  {pwLoading ? t.settings.updating : t.settings.updatePassword}
                </button>
              </div>
            </form>
          </Panel>
        </div>
      )}

      {/* ── Tab: Sao lưu ── */}
      {activeTab === 'backup' && <BackupPanel />}

      {/* ── Tab: Báo cáo ── */}
      {activeTab === 'reports' && <PastReportsPanel />}

    </div>
  );
}
