import { useState, useEffect } from 'react';
import { Panel, Icon } from '../components/ui';
import { useAuth } from '../context/AuthContext';
import { fetchSchedules, updateSchedule, triggerEndpoint, type ScheduleItem } from '../api';

function getToken() {
  return localStorage.getItem('vns_token') || '';
}

// ── Cron expression human-readable hint ──────────────────────────────────────
function cronHint(expr: string): string {
  const parts = expr.trim().split(/\s+/);
  if (parts.length !== 6) return '';
  const [, min, hour, , , dow] = parts;
  const h = parseInt(hour), m = parseInt(min);
  if (isNaN(h) || isNaN(m)) return '';
  const t = ('0' + h).slice(-2) + ':' + ('0' + m).slice(-2);
  if (dow && dow !== '*' && dow !== '?') return `Hàng tuần ${dow} ${t}`;
  return `Hàng ngày lúc ${t}`;
}

// ── Trigger button ────────────────────────────────────────────────────────────
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
      setTimeout(() => setMsg(null), 4000);
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
      <button
        className="btn btn--ghost"
        style={{ display: 'flex', alignItems: 'center', gap: 7, padding: '8px 14px', fontSize: 13 }}
        disabled={loading}
        onClick={run}
      >
        <Icon name={icon} size={14} />
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

// ── Cron schedule row ─────────────────────────────────────────────────────────
function ScheduleRow({ s, onSaved }: { s: ScheduleItem; onSaved: () => void }) {
  const [editing, setEditing] = useState(false);
  const [expr, setExpr] = useState(s.cron_expression);
  const [enabled, setEnabled] = useState(s.enabled);
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState('');

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
      <td style={{ padding: '10px 12px', fontSize: 13 }}>
        <div style={{ fontWeight: 500 }}>{s.job_name}</div>
        <div style={{ fontSize: 11, color: 'var(--text-3)', marginTop: 2, fontFamily: 'var(--font-mono)' }}>
          {s.job_key}
        </div>
      </td>
      <td style={{ padding: '10px 12px' }}>
        {editing ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
            <input
              className="field__input"
              style={{ fontFamily: 'var(--font-mono)', fontSize: 13, width: 190 }}
              value={expr}
              onChange={(e) => setExpr(e.target.value)}
              placeholder="0 0 12 * * *"
            />
            <div style={{ fontSize: 11, color: 'var(--text-3)' }}>{cronHint(expr)}</div>
            {err && <div style={{ fontSize: 11, color: 'var(--down)' }}>{err}</div>}
          </div>
        ) : (
          <div>
            <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12, color: 'var(--accent)' }}>
              {s.cron_expression}
            </span>
            <div style={{ fontSize: 11, color: 'var(--text-3)', marginTop: 2 }}>{cronHint(s.cron_expression)}</div>
          </div>
        )}
      </td>
      <td style={{ padding: '10px 12px' }}>
        {editing ? (
          <label style={{ display: 'flex', alignItems: 'center', gap: 6, cursor: 'pointer' }}>
            <input
              type="checkbox"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
            />
            <span style={{ fontSize: 12 }}>{enabled ? 'Bật' : 'Tắt'}</span>
          </label>
        ) : (
          <span className={`badge ${s.enabled ? 'badge--up' : 'badge--muted'}`}>
            {s.enabled ? 'Bật' : 'Tắt'}
          </span>
        )}
      </td>
      <td style={{ padding: '10px 12px' }}>
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
          <button
            className="btn btn--ghost"
            style={{ fontSize: 12, padding: '5px 10px' }}
            onClick={() => setEditing(true)}
          >
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
                  <th style={{ padding: '8px 12px', fontSize: 12, color: 'var(--text-3)' }}>Lịch (6-field cron)</th>
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
        <div style={{ padding: '10px 12px', borderTop: '1px solid var(--border)', fontSize: 11, color: 'var(--text-3)' }}>
          Định dạng cron 6 trường: giây phút giờ ngày tháng ngày-tuần (ví dụ: 0 0 12 * * * = mỗi ngày 12:00)
        </div>
      </Panel>

      {/* Manual triggers */}
      <Panel title="Thao tác thủ công">
        <div style={{ padding: '4px 0', fontSize: 12, color: 'var(--text-3)', marginBottom: 12 }}>
          Kích hoạt tác vụ ngay lập tức (chạy nền, không chờ lịch cron)
        </div>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 10 }}>
          <TriggerBtn label="Crawl VN30"  endpoint="crawler"      icon="candles" />
          <TriggerBtn label="Crawl Vàng"  endpoint="gold-crawler" icon="gold"    />
          <TriggerBtn label="Huấn luyện"  endpoint="train"        icon="cpu"     />
          <TriggerBtn label="Dự đoán"     endpoint="predict"      icon="pulse"   />
          <TriggerBtn label="Reconcile"   endpoint="reconcile"    icon="refresh" />
        </div>
      </Panel>
    </div>
  );
}
