import { useState, useEffect } from 'react';
import { Panel, Icon } from '../components/ui';
import { useAuth } from '../context/AuthContext';
import { fetchSchedules, updateSchedule, triggerEndpoint, type ScheduleItem } from '../api';

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

function cronLabel(expr: string): string {
  const p = parseCron(expr);
  const pad = (n: number) => ('0' + n).slice(-2);
  switch (p.type) {
    case 'every_hour':    return 'Mỗi giờ';
    case 'every_n_hours': return `Mỗi ${p.interval} giờ`;
    case 'daily':         return `Hàng ngày ${pad(p.hour)}:${pad(p.minute)}`;
    case 'weekly': {
      const d = DOW_OPTIONS.find(x => x.value === p.dow)?.label || p.dow;
      return `${d} ${pad(p.hour)}:${pad(p.minute)}`;
    }
  }
}

// ── Schedule editor ───────────────────────────────────────────────────────────

function ScheduleEditor({ expr, onChange }: { expr: string; onChange: (newExpr: string) => void }) {
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
        <option value="every_hour">Mỗi giờ</option>
        <option value="every_n_hours">Mỗi N giờ</option>
        <option value="daily">Hàng ngày lúc</option>
        <option value="weekly">Hàng tuần vào</option>
      </select>

      {/* Every N hours */}
      {type === 'every_n_hours' && (
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ fontSize: 13, color: 'var(--text-2)' }}>Mỗi</span>
          <select
            className="sel"
            value={interval}
            onChange={(e) => setInterval(Number(e.target.value))}
            style={{ fontSize: 13 }}
          >
            {[2, 3, 4, 6, 8, 12].map((n) => (
              <option key={n} value={n}>{n} giờ</option>
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
          <span style={{ fontSize: 11, color: 'var(--text-3)' }}>giờ : phút</span>
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
        {loading ? 'Đang chạy…' : label}
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

function PipelineTriggerBtn({ label, icon, steps }: { label: string; icon: string; steps: PipelineStep[] }) {
  const [running, setRunning] = useState(false);
  const [stepStates, setStepStates] = useState<StepState[]>(() => steps.map(() => 'idle'));
  const [errorMsg, setErrorMsg] = useState('');

  const reset = () => {
    setStepStates(steps.map(() => 'idle'));
    setErrorMsg('');
  };

  const run = async () => {
    setRunning(true);
    setErrorMsg('');
    const states: StepState[] = steps.map(() => 'idle');
    setStepStates([...states]);

    for (let i = 0; i < steps.length; i++) {
      states[i] = 'running';
      setStepStates([...states]);
      try {
        await triggerEndpoint(steps[i].endpoint);
        states[i] = 'ok';
        setStepStates([...states]);
      } catch (e: any) {
        states[i] = 'error';
        setStepStates([...states]);
        setErrorMsg(`${steps[i].label}: ${e?.message || 'Lỗi'}`);
        break;
      }
    }

    setRunning(false);
    setTimeout(reset, 6000);
  };

  const allDone  = stepStates.every(s => s === 'ok');
  const hasError = stepStates.some(s => s === 'error');
  const anyRan   = stepStates.some(s => s !== 'idle');

  const stepIcon = (s: StepState) => {
    if (s === 'idle')    return <span style={{ color: 'var(--text-4)', fontSize: 10 }}>○</span>;
    if (s === 'running') return <span style={{ color: 'var(--accent)', fontSize: 10, animation: 'pulse 1s infinite' }}>●</span>;
    if (s === 'ok')      return <span style={{ color: 'var(--up)',   fontSize: 10 }}>✓</span>;
    return                      <span style={{ color: 'var(--down)', fontSize: 10 }}>✗</span>;
  };

  return (
    <div style={{
      border: '1px solid var(--border)',
      borderRadius: 8,
      padding: '10px 12px',
      background: hasError ? 'rgba(220,60,60,0.04)' : allDone ? 'rgba(47,181,124,0.04)' : 'var(--surface-2)',
      minWidth: 150,
      display: 'flex',
      flexDirection: 'column',
      gap: 8,
    }}>
      {/* Header button */}
      <button
        className="btn btn--ghost"
        style={{
          display: 'flex', alignItems: 'center', gap: 7,
          padding: '6px 0', fontSize: 13, fontWeight: 600,
          background: 'none', border: 'none', cursor: running ? 'not-allowed' : 'pointer',
          color: hasError ? 'var(--down)' : allDone ? 'var(--up)' : 'var(--text-1)',
          justifyContent: 'flex-start',
        }}
        disabled={running}
        onClick={run}
      >
        <Icon name={icon} size={14} />
        {label}
      </button>

      {/* Steps */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
        {steps.map((step, i) => (
          <div key={step.endpoint} style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 11, color: 'var(--text-3)' }}>
            {stepIcon(stepStates[i])}
            <span style={{ color: stepStates[i] === 'running' ? 'var(--accent)' : stepStates[i] === 'ok' ? 'var(--up)' : stepStates[i] === 'error' ? 'var(--down)' : 'var(--text-3)' }}>
              {step.label}
            </span>
          </div>
        ))}
      </div>

      {/* Status message */}
      {anyRan && (
        <div style={{ fontSize: 11, marginTop: 2 }}>
          {allDone  && <span style={{ color: 'var(--up)' }}>✓ Hoàn thành</span>}
          {hasError && <span style={{ color: 'var(--down)' }}>✗ {errorMsg}</span>}
          {running  && <span style={{ color: 'var(--accent)' }}>Đang chạy…</span>}
        </div>
      )}
    </div>
  );
}

// ── Cron schedule row ─────────────────────────────────────────────────────────
function ScheduleRow({ s, onSaved }: { s: ScheduleItem; onSaved: () => void }) {
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
      <td style={{ padding: '10px 12px', fontSize: 13, verticalAlign: 'top' }}>
        <div style={{ fontWeight: 500 }}>{s.job_name}</div>
        <div style={{ fontSize: 11, color: 'var(--text-3)', marginTop: 2, fontFamily: 'var(--font-mono)' }}>
          {s.job_key}
        </div>
      </td>

      {/* Schedule column */}
      <td style={{ padding: '10px 12px', verticalAlign: 'top' }}>
        {editing ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <ScheduleEditor expr={expr} onChange={setExpr} />
            {err && <div style={{ fontSize: 11, color: 'var(--down)' }}>{err}</div>}
          </div>
        ) : (
          <div>
            <div style={{ fontSize: 13, fontWeight: 500 }}>{cronLabel(s.cron_expression)}</div>
            <div style={{ fontSize: 11, color: 'var(--text-3)', fontFamily: 'var(--font-mono)', marginTop: 2 }}>
              {s.cron_expression}
            </div>
          </div>
        )}
      </td>

      {/* Enabled column */}
      <td style={{ padding: '10px 12px', verticalAlign: 'top' }}>
        {editing ? (
          <label style={{ display: 'flex', alignItems: 'center', gap: 6, cursor: 'pointer', fontSize: 13 }}>
            <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
            {enabled ? 'Bật' : 'Tắt'}
          </label>
        ) : (
          <span className={`badge ${s.enabled ? 'badge--up' : 'badge--muted'}`}>
            {s.enabled ? 'Bật' : 'Tắt'}
          </span>
        )}
      </td>

      {/* Actions column */}
      <td style={{ padding: '10px 12px', verticalAlign: 'top' }}>
        {editing ? (
          <div style={{ display: 'flex', gap: 6 }}>
            <button className="btn btn--primary" style={{ fontSize: 12, padding: '5px 12px' }} disabled={saving} onClick={save}>
              {saving ? '...' : 'Lưu'}
            </button>
            <button className="btn btn--ghost" style={{ fontSize: 12, padding: '5px 12px' }} onClick={cancel}>
              Huỷ
            </button>
          </div>
        ) : (
          <button className="btn btn--ghost" style={{ fontSize: 12, padding: '5px 10px' }} onClick={() => setEditing(true)}>
            <Icon name="settings" size={12} />
          </button>
        )}
      </td>
    </tr>
  );
}

// ── Main component ────────────────────────────────────────────────────────────
export default function Settings() {
  const { user } = useAuth();

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
    if (newPw !== confirmPw) { setPwError('Mật khẩu mới không khớp'); return; }
    if (newPw.length < 6)   { setPwError('Mật khẩu mới phải có ít nhất 6 ký tự'); return; }
    setPwLoading(true);
    try {
      const res = await fetch('/api/auth/password', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${getToken()}` },
        body: JSON.stringify({ current_password: currentPw, new_password: newPw }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.message || 'Đổi mật khẩu thất bại');
      setPwSuccess('Đổi mật khẩu thành công');
      setCurrentPw(''); setNewPw(''); setConfirmPw('');
    } catch (err: unknown) {
      setPwError(err instanceof Error ? err.message : 'Đổi mật khẩu thất bại');
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
      setSchedError('Không thể tải lịch tác vụ: ' + (e?.message || ''));
    } finally {
      setSchedLoading(false);
    }
  };

  useEffect(() => { loadSchedules(); }, []);

  // ── Render ────────────────────────────────────────────────────────────────────
  return (
    <div className="content__inner" style={{ display: 'flex', flexDirection: 'column', gap: 16, maxWidth: 680 }}>
      {/* Account info */}
      <Panel title="Thông tin tài khoản">
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12, padding: '4px 0' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <div style={{
              width: 40, height: 40, borderRadius: '50%',
              background: 'var(--accent)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              color: '#fff', fontWeight: 700, fontSize: 16,
            }}>
              {user?.username?.[0]?.toUpperCase()}
            </div>
            <div>
              <div style={{ fontWeight: 600, fontSize: 15 }}>{user?.username}</div>
              <div style={{ fontSize: 12, color: 'var(--text-2)', marginTop: 2 }}>
                <span className={`badge ${user?.role === 'admin' ? 'badge--up' : 'badge--muted'}`}>
                  {user?.role}
                </span>
              </div>
            </div>
          </div>
        </div>
      </Panel>

      {/* Change password */}
      <Panel title="Đổi mật khẩu">
        <form onSubmit={handleChangePassword} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          {pwError && <div className="modal-error">{pwError}</div>}
          {pwSuccess && (
            <div style={{
              background: 'rgba(47,181,124,0.1)', border: '1px solid rgba(47,181,124,0.3)',
              color: '#2FB57C', padding: '8px 10px', borderRadius: 6, fontSize: 12,
              display: 'flex', alignItems: 'center', gap: 6,
            }}>
              <Icon name="bell" size={13} />{pwSuccess}
            </div>
          )}
          <div className="field">
            <label className="field__label">Mật khẩu hiện tại</label>
            <input className="field__input" type="password" value={currentPw}
              onChange={e => setCurrentPw(e.target.value)} required placeholder="••••••••" autoComplete="current-password" />
          </div>
          <div className="field">
            <label className="field__label">Mật khẩu mới</label>
            <input className="field__input" type="password" value={newPw}
              onChange={e => setNewPw(e.target.value)} required placeholder="Tối thiểu 6 ký tự" autoComplete="new-password" />
          </div>
          <div className="field">
            <label className="field__label">Xác nhận mật khẩu mới</label>
            <input className="field__input" type="password" value={confirmPw}
              onChange={e => setConfirmPw(e.target.value)} required placeholder="••••••••" autoComplete="new-password" />
          </div>
          <div>
            <button className="btn btn--primary" type="submit" disabled={pwLoading}>
              {pwLoading ? 'Đang cập nhật...' : 'Cập nhật mật khẩu'}
            </button>
          </div>
        </form>
      </Panel>

      {/* Cron schedules */}
      <Panel
        title="Lịch tác vụ tự động"
        tools={
          <button className="btn btn--ghost btn--icon" onClick={loadSchedules} disabled={schedLoading} title="Tải lại">
            <Icon name="refresh" size={14} />
          </button>
        }
      >
        {schedError && (
          <div style={{ padding: '12px', color: 'var(--down)', fontSize: 13 }}>{schedError}</div>
        )}
        {schedLoading && !schedules.length ? (
          <div style={{ padding: '24px', color: 'var(--text-3)', fontSize: 13, textAlign: 'center' }}>Đang tải…</div>
        ) : schedules.length === 0 ? (
          <div style={{ padding: '24px', color: 'var(--text-3)', fontSize: 13, textAlign: 'center' }}>
            Chưa có dữ liệu lịch tác vụ
          </div>
        ) : (
          <div style={{ overflowX: 'auto' }}>
            <table className="tbl" style={{ width: '100%' }}>
              <thead>
                <tr>
                  <th style={{ padding: '8px 12px', fontSize: 12, color: 'var(--text-3)' }}>Tác vụ</th>
                  <th style={{ padding: '8px 12px', fontSize: 12, color: 'var(--text-3)' }}>Lịch</th>
                  <th style={{ padding: '8px 12px', fontSize: 12, color: 'var(--text-3)' }}>Trạng thái</th>
                  <th style={{ padding: '8px 12px', fontSize: 12, color: 'var(--text-3)' }}></th>
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

      {/* Manual triggers */}
      <Panel title="Thao tác thủ công">
        <div style={{ padding: '4px 0', fontSize: 12, color: 'var(--text-3)', marginBottom: 14 }}>
          Chạy pipeline ngay lập tức: crawl dữ liệu mới → dự đoán → cập nhật bot trading
        </div>

        {/* Pipeline cards */}
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 10, marginBottom: 16 }}>
          <PipelineTriggerBtn label="Vàng" icon="gold" steps={[
            { label: 'Crawl',      endpoint: 'gold-crawler'        },
            { label: 'Dự đoán',   endpoint: 'gold-predict'        },
            { label: 'Bot trading', endpoint: 'simulation-live-step' },
          ]} />
          <PipelineTriggerBtn label="NASDAQ" icon="nasdaq" steps={[
            { label: 'Crawl',      endpoint: 'nasdaq-crawler'      },
            { label: 'Dự đoán',   endpoint: 'nasdaq-predict'      },
            { label: 'Bot trading', endpoint: 'simulation-live-step' },
          ]} />
          <PipelineTriggerBtn label="S&P 500" icon="pulse" steps={[
            { label: 'Crawl',      endpoint: 'sp500-crawler'       },
            { label: 'Dự đoán',   endpoint: 'sp500-predict'       },
            { label: 'Bot trading', endpoint: 'simulation-live-step' },
          ]} />
          <PipelineTriggerBtn label="Crypto" icon="crypto" steps={[
            { label: 'Crawl',      endpoint: 'crypto-crawler'      },
            { label: 'Dự đoán',   endpoint: 'crypto-predict'      },
            { label: 'Bot trading', endpoint: 'simulation-live-step' },
          ]} />
          <PipelineTriggerBtn label="Xăng" icon="fuel" steps={[
            { label: 'Crawl',      endpoint: 'fuel-crawler'        },
            { label: 'Dự đoán',   endpoint: 'fuel-predict'        },
            { label: 'Bot trading', endpoint: 'simulation-live-step' },
          ]} />
          <PipelineTriggerBtn label="VN30" icon="candles" steps={[
            { label: 'Crawl',      endpoint: 'crawler'             },
            { label: 'Dự đoán',   endpoint: 'predict'             },
            { label: 'Bot trading', endpoint: 'simulation-live-step' },
          ]} />
        </div>

        {/* Utility actions */}
        <div style={{ borderTop: '1px solid var(--border)', paddingTop: 12, display: 'flex', flexWrap: 'wrap', gap: 8 }}>
          <TriggerBtn label="Huấn luyện tất cả"  endpoint="train"       icon="cpu"     />
          <TriggerBtn label="Reconcile"           endpoint="reconcile"   icon="refresh" />
        </div>
      </Panel>
    </div>
  );
}
