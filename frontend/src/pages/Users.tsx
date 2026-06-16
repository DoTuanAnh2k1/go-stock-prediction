import { useState, useEffect } from 'react';
import { Panel } from '../components/ui';
import { useAuth } from '../context/AuthContext';
import { useLanguage } from '../context/LangContext';
import MarketGroups from './MarketGroups';

interface UserItem {
  id: number;
  username: string;
  role: string;
  created_at: string;
  full_name?: string;
  email?: string;
  phone?: string;
}

function getToken() {
  return localStorage.getItem('vns_token') || '';
}

// ── Required-field asterisk ──────────────────────────────────────────────────

function Req() {
  return <span style={{ color: 'var(--down)' }}> *</span>;
}

// ── Reset-password modal ─────────────────────────────────────────────────────

interface ResetPasswordModalProps {
  user: UserItem;
  onClose: () => void;
  onSuccess: (msg: string) => void;
}

function ResetPasswordModal({ user, onClose, onSuccess }: ResetPasswordModalProps) {
  const { t } = useLanguage();
  const [newPassword, setNewPassword] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    if (newPassword.length < 6) {
      setError(t.users.passwordTooShort);
      return;
    }
    setSubmitting(true);
    try {
      const res = await fetch(`/api/users/${user.id}/reset-password`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${getToken()}`,
        },
        body: JSON.stringify({ new_password: newPassword }),
      });
      const data = await res.json().catch(() => ({})) as { message?: string };
      if (!res.ok) throw new Error(data.message || t.users.resetFail);
      onSuccess(t.users.resetSuccess);
      onClose();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : t.users.resetFail);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="modal-overlay" onClick={e => { if (e.target === e.currentTarget) onClose(); }}>
      <div className="modal-box">
        <div className="modal-header">
          <span className="modal-title">
            {t.users.resetPasswordFor} <strong>{user.username}</strong>
          </span>
          <button
            className="btn btn--icon btn--ghost"
            onClick={onClose}
            style={{ marginLeft: 'auto' }}
            aria-label="Close"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <line x1="18" y1="6" x2="6" y2="18"/>
              <line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
          </button>
        </div>
        <form className="modal-body" onSubmit={handleSubmit}>
          {error && <div className="modal-error">{error}</div>}
          <div className="field">
            <label className="field__label">{t.users.newPassword}</label>
            <input
              className="field__input"
              type="password"
              value={newPassword}
              onChange={e => setNewPassword(e.target.value)}
              placeholder={t.users.newPasswordPlaceholder}
              autoFocus
              autoComplete="new-password"
              minLength={6}
            />
          </div>
          <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 4 }}>
            <button type="button" className="btn" onClick={onClose} disabled={submitting}>
              {t.users.cancel}
            </button>
            <button type="submit" className="btn btn--primary" disabled={submitting || newPassword.length < 1}>
              {submitting ? t.users.resetting : t.users.resetBtn}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// ── Edit user modal ──────────────────────────────────────────────────────────

interface EditUserModalProps {
  user: UserItem;
  callerRole: string;
  onClose: () => void;
  onSuccess: (msg: string) => void;
}

function EditUserModal({ user, callerRole, onClose, onSuccess }: EditUserModalProps) {
  const { t } = useLanguage();
  const [fullName, setFullName] = useState(user.full_name || '');
  const [email, setEmail] = useState(user.email || '');
  const [phone, setPhone] = useState(user.phone || '');
  const [role, setRole] = useState<'user' | 'admin'>(
    user.role === 'admin' ? 'admin' : 'user'
  );
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  // super_admin can assign admin; admin can only set user
  const canPromoteToAdmin = callerRole === 'super_admin';

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setSubmitting(true);
    try {
      const res = await fetch(`/api/users/${user.id}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${getToken()}`,
        },
        body: JSON.stringify({
          full_name: fullName || undefined,
          email: email || undefined,
          phone: phone || undefined,
          role,
        }),
      });
      const data = await res.json().catch(() => ({})) as { message?: string };
      if (!res.ok) throw new Error(data.message || t.users.updateFail);
      onSuccess(t.users.updateSuccess);
      onClose();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : t.users.updateFail);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="modal-overlay" onClick={e => { if (e.target === e.currentTarget) onClose(); }}>
      <div className="modal-box" style={{ maxWidth: 480 }}>
        <div className="modal-header">
          <span className="modal-title">
            {t.users.editUserFor} <strong>{user.username}</strong>
          </span>
          <button
            className="btn btn--icon btn--ghost"
            onClick={onClose}
            style={{ marginLeft: 'auto' }}
            aria-label="Close"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <line x1="18" y1="6" x2="6" y2="18"/>
              <line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
          </button>
        </div>
        <form className="modal-body" onSubmit={handleSubmit}>
          {error && <div className="modal-error">{error}</div>}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: '10px 14px' }}>
            <div className="field" style={{ minWidth: 0 }}>
              <label className="field__label">
                {t.users.role}<Req />
              </label>
              <select
                className="field__input"
                value={role}
                onChange={e => setRole(e.target.value as 'user' | 'admin')}
              >
                <option value="user">User</option>
                {canPromoteToAdmin && <option value="admin">Admin</option>}
              </select>
            </div>
            <div className="field" style={{ minWidth: 0 }}>
              <label className="field__label">
                {t.users.fullName}
                <span style={{ color: 'var(--text-3)', fontWeight: 400 }}> ({t.users.optional})</span>
              </label>
              <input
                className="field__input"
                type="text"
                value={fullName}
                onChange={e => setFullName(e.target.value)}
                autoComplete="off"
              />
            </div>
            <div className="field" style={{ minWidth: 0 }}>
              <label className="field__label">
                {t.users.email}
                <span style={{ color: 'var(--text-3)', fontWeight: 400 }}> ({t.users.optional})</span>
              </label>
              <input
                className="field__input"
                type="email"
                value={email}
                onChange={e => setEmail(e.target.value)}
                autoComplete="off"
              />
            </div>
            <div className="field" style={{ minWidth: 0 }}>
              <label className="field__label">
                {t.users.phone}
                <span style={{ color: 'var(--text-3)', fontWeight: 400 }}> ({t.users.optional})</span>
              </label>
              <input
                className="field__input"
                type="tel"
                value={phone}
                onChange={e => setPhone(e.target.value)}
                autoComplete="off"
              />
            </div>
          </div>
          <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 12 }}>
            <button type="button" className="btn" onClick={onClose} disabled={submitting}>
              {t.users.cancel}
            </button>
            <button type="submit" className="btn btn--primary" disabled={submitting}>
              {submitting ? t.users.saving : t.users.save}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// ── Main Users page ──────────────────────────────────────────────────────────

export default function Users() {
  const { user, role: callerRole } = useAuth();
  const { t } = useLanguage();
  const [activeTab, setActiveTab] = useState<'users' | 'groups'>('users');
  const [users, setUsers] = useState<UserItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const [newUsername, setNewUsername] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [newRole, setNewRole] = useState<'user' | 'admin'>('user');
  const [newFullName, setNewFullName] = useState('');
  const [newEmail, setNewEmail] = useState('');
  const [newPhone, setNewPhone] = useState('');
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState('');

  // Reset password modal state
  const [resetTarget, setResetTarget] = useState<UserItem | null>(null);
  const [resetSuccessMsg, setResetSuccessMsg] = useState('');

  // Edit user modal state
  const [editTarget, setEditTarget] = useState<UserItem | null>(null);

  const fetchUsers = async () => {
    setLoading(true);
    setError('');
    try {
      const res = await fetch('/api/users', {
        headers: { Authorization: `Bearer ${getToken()}` },
      });
      if (!res.ok) throw new Error(t.users.cannotLoad);
      const data = await res.json();
      setUsers(data || []);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : t.users.cannotLoad);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { fetchUsers(); }, []);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreateError('');
    setCreating(true);
    try {
      const res = await fetch('/api/users', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${getToken()}`,
        },
        body: JSON.stringify({
          username: newUsername,
          password: newPassword,
          role: newRole,
          full_name: newFullName || undefined,
          email: newEmail || undefined,
          phone: newPhone || undefined,
        }),
      });
      const data = await res.json() as { message?: string };
      if (!res.ok) throw new Error(data.message || t.users.createFail);
      setNewUsername('');
      setNewPassword('');
      setNewRole('user');
      setNewFullName('');
      setNewEmail('');
      setNewPhone('');
      fetchUsers();
    } catch (e: unknown) {
      setCreateError(e instanceof Error ? e.message : t.users.createFail);
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (u: UserItem) => {
    if (!confirm(`${t.users.deleteConfirm} "${u.username}"?`)) return;
    try {
      await fetch(`/api/users/${u.id}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${getToken()}` },
      });
      fetchUsers();
    } catch {
      // silent — list will refresh anyway
    }
  };

  const canResetPassword = (u: UserItem): boolean => {
    if (callerRole === 'super_admin' && u.role !== 'super_admin') return true;
    if (callerRole === 'admin' && u.role === 'user') return true;
    return false;
  };

  // show edit button only when the caller has authority to modify the target
  const canEditUser = (u: UserItem): boolean => {
    if (callerRole === 'super_admin') return u.role !== 'super_admin';
    if (callerRole === 'admin') return u.role === 'user';
    return false;
  };

  const showSuccessBanner = (msg: string) => {
    setResetSuccessMsg(msg);
    setTimeout(() => setResetSuccessMsg(''), 4000);
  };

  return (
    <div>
      {/* Reset-password modal */}
      {resetTarget && (
        <ResetPasswordModal
          user={resetTarget}
          onClose={() => setResetTarget(null)}
          onSuccess={showSuccessBanner}
        />
      )}

      {/* Edit user modal */}
      {editTarget && (
        <EditUserModal
          user={editTarget}
          callerRole={callerRole || ''}
          onClose={() => setEditTarget(null)}
          onSuccess={msg => { showSuccessBanner(msg); fetchUsers(); }}
        />
      )}

      {/* Tab bar */}
      <div style={{ padding: '20px 20px 0' }}>
        <div style={{ display: 'flex', gap: 0, borderBottom: '1px solid var(--border)' }}>
          <button
            onClick={() => setActiveTab('users')}
            style={{
              background: 'none',
              border: 'none',
              borderBottom: activeTab === 'users' ? '2px solid var(--accent, #5B8DEF)' : '2px solid transparent',
              color: activeTab === 'users' ? 'var(--text)' : 'var(--text-3)',
              cursor: 'pointer',
              fontFamily: 'var(--font-ui)',
              fontSize: 13,
              fontWeight: activeTab === 'users' ? 600 : 400,
              padding: '8px 16px',
              marginBottom: -1,
              transition: 'color .15s, border-color .15s',
            }}
          >
            {t.nav.users}
          </button>
          <button
            onClick={() => setActiveTab('groups')}
            style={{
              background: 'none',
              border: 'none',
              borderBottom: activeTab === 'groups' ? '2px solid var(--accent, #5B8DEF)' : '2px solid transparent',
              color: activeTab === 'groups' ? 'var(--text)' : 'var(--text-3)',
              cursor: 'pointer',
              fontFamily: 'var(--font-ui)',
              fontSize: 13,
              fontWeight: activeTab === 'groups' ? 600 : 400,
              padding: '8px 16px',
              marginBottom: -1,
              transition: 'color .15s, border-color .15s',
            }}
          >
            {t.nav.marketGroups}
          </button>
        </div>
      </div>

      {/* Tab content */}
      {activeTab === 'users' ? (
        <div style={{ padding: '16px 20px 20px', display: 'flex', flexDirection: 'column', gap: '16px' }}>
          {resetSuccessMsg && (
            <div style={{
              background: 'oklch(0.70 0.16 152 / 0.12)',
              border: '1px solid oklch(0.70 0.16 152 / 0.35)',
              color: 'oklch(0.70 0.16 152)',
              padding: '8px 12px',
              fontSize: 13,
            }}>
              {resetSuccessMsg}
            </div>
          )}

          <Panel title={t.users.createUser}>
            <form onSubmit={handleCreate} style={{ maxWidth: 680 }}>
              {createError && (
                <div className="modal-error" style={{ gridColumn: '1 / -1', marginBottom: 8 }}>
                  {createError}
                </div>
              )}
              <div style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))',
                gap: '10px 16px',
              }}>
                {/* row 1: Username | Password */}
                <div className="field" style={{ minWidth: 0 }}>
                  <label className="field__label">
                    {t.users.username}<Req />
                  </label>
                  <input
                    className="field__input"
                    value={newUsername}
                    onChange={e => setNewUsername(e.target.value)}
                    autoComplete="off"
                    required
                  />
                </div>
                <div className="field" style={{ minWidth: 0 }}>
                  <label className="field__label">
                    {t.users.password}<Req />
                  </label>
                  <input
                    className="field__input"
                    type="password"
                    value={newPassword}
                    onChange={e => setNewPassword(e.target.value)}
                    autoComplete="new-password"
                    required
                  />
                </div>
                {/* row 2: Role | Full name */}
                <div className="field" style={{ minWidth: 0 }}>
                  <label className="field__label">
                    {t.users.role}<Req />
                  </label>
                  <select
                    className="field__input"
                    value={newRole}
                    onChange={e => setNewRole(e.target.value as 'user' | 'admin')}
                  >
                    <option value="user">User</option>
                    <option value="admin">Admin</option>
                  </select>
                </div>
                <div className="field" style={{ minWidth: 0 }}>
                  <label className="field__label">
                    {t.users.fullName}
                    <span style={{ color: 'var(--text-3)', fontWeight: 400 }}> ({t.users.optional})</span>
                  </label>
                  <input
                    className="field__input"
                    type="text"
                    value={newFullName}
                    onChange={e => setNewFullName(e.target.value)}
                    autoComplete="off"
                  />
                </div>
                {/* row 3: Email | Phone */}
                <div className="field" style={{ minWidth: 0 }}>
                  <label className="field__label">
                    {t.users.email}
                    <span style={{ color: 'var(--text-3)', fontWeight: 400 }}> ({t.users.optional})</span>
                  </label>
                  <input
                    className="field__input"
                    type="email"
                    value={newEmail}
                    onChange={e => setNewEmail(e.target.value)}
                    autoComplete="off"
                  />
                </div>
                <div className="field" style={{ minWidth: 0 }}>
                  <label className="field__label">
                    {t.users.phone}
                    <span style={{ color: 'var(--text-3)', fontWeight: 400 }}> ({t.users.optional})</span>
                  </label>
                  <input
                    className="field__input"
                    type="tel"
                    value={newPhone}
                    onChange={e => setNewPhone(e.target.value)}
                    autoComplete="off"
                  />
                </div>
                {/* submit — full width */}
                <div style={{ gridColumn: '1 / -1' }}>
                  <button className="btn btn--primary" type="submit" disabled={creating}>
                    {creating ? t.users.creating : t.users.createBtn}
                  </button>
                </div>
              </div>
            </form>
          </Panel>

          <Panel
            title={t.users.userList}
            sub={loading ? undefined : `${users.length} ${t.users.people}`}
          >
            {loading ? (
              <div style={{ padding: 16, color: 'var(--text-2)' }}>{t.users.loading}</div>
            ) : error ? (
              <div className="modal-error" style={{ margin: 0 }}>{error}</div>
            ) : users.length === 0 ? (
              <div style={{ padding: 16, color: 'var(--text-3)' }}>{t.users.noUsers}</div>
            ) : (
              <div style={{ overflowX: 'auto' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
                  <thead>
                    <tr style={{ borderBottom: '1px solid var(--border)' }}>
                      <th style={{ textAlign: 'left', padding: '8px 12px', color: 'var(--text-2)', fontWeight: 500 }}>{t.users.colId}</th>
                      <th style={{ textAlign: 'left', padding: '8px 12px', color: 'var(--text-2)', fontWeight: 500 }}>{t.users.colUsername}</th>
                      <th style={{ textAlign: 'left', padding: '8px 12px', color: 'var(--text-2)', fontWeight: 500 }}>{t.users.colFullName}</th>
                      <th style={{ textAlign: 'left', padding: '8px 12px', color: 'var(--text-2)', fontWeight: 500 }}>{t.users.colEmail}</th>
                      <th style={{ textAlign: 'left', padding: '8px 12px', color: 'var(--text-2)', fontWeight: 500 }}>{t.users.colPhone}</th>
                      <th style={{ textAlign: 'left', padding: '8px 12px', color: 'var(--text-2)', fontWeight: 500 }}>{t.users.colRole}</th>
                      <th style={{ textAlign: 'left', padding: '8px 12px', color: 'var(--text-2)', fontWeight: 500 }}>{t.users.colCreatedAt}</th>
                      <th style={{ width: 96 }}></th>
                    </tr>
                  </thead>
                  <tbody>
                    {users.map(u => (
                      <tr key={u.id} style={{ borderBottom: '1px solid var(--border)' }}>
                        <td style={{ padding: '8px 12px', color: 'var(--text-2)', fontFamily: 'var(--font-mono)', fontSize: 12 }}>{u.id}</td>
                        <td style={{ padding: '8px 12px', fontWeight: 500 }}>{u.username}</td>
                        <td style={{ padding: '8px 12px', color: 'var(--text-2)' }}>{u.full_name || '—'}</td>
                        <td style={{ padding: '8px 12px', color: 'var(--text-2)', fontFamily: 'var(--font-mono)', fontSize: 12 }}>{u.email || '—'}</td>
                        <td style={{ padding: '8px 12px', color: 'var(--text-2)', fontFamily: 'var(--font-mono)', fontSize: 12 }}>{u.phone || '—'}</td>
                        <td style={{ padding: '8px 12px' }}>
                          <span className={`role-badge role-badge--${u.role.replace('_', '-')}`}>{u.role}</span>
                        </td>
                        <td style={{ padding: '8px 12px', color: 'var(--text-2)', fontFamily: 'var(--font-mono)', fontSize: 12 }}>
                          {u.created_at ? new Date(u.created_at).toLocaleString('vi-VN') : '—'}
                        </td>
                        <td style={{ padding: '8px 12px', display: 'flex', gap: 4, alignItems: 'center' }}>
                          {/* Edit button */}
                          {canEditUser(u) && (
                            <button
                              className="btn btn--icon btn--ghost"
                              style={{ color: 'var(--text-2)' }}
                              onClick={() => setEditTarget(u)}
                              title={`${t.users.editUser}: ${u.username}`}
                            >
                              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
                                <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
                                <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                              </svg>
                            </button>
                          )}
                          {/* Reset password button */}
                          {canResetPassword(u) && (
                            <button
                              className="btn btn--icon btn--ghost"
                              style={{ color: 'var(--accent)' }}
                              onClick={() => setResetTarget(u)}
                              title={`${t.users.resetPassword}: ${u.username}`}
                            >
                              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
                                <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
                                <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
                              </svg>
                            </button>
                          )}
                          {/* Delete button */}
                          {u.username !== user?.username && (
                            <button
                              className="btn btn--icon btn--ghost"
                              style={{ color: 'var(--down)', opacity: (u.role === 'super_admin' && callerRole !== 'super_admin') ? 0.3 : 1 }}
                              onClick={() => handleDelete(u)}
                              disabled={u.role === 'super_admin' && callerRole !== 'super_admin'}
                              title={u.role === 'super_admin' && callerRole !== 'super_admin' ? 'Không thể xóa super_admin' : `Xóa ${u.username}`}
                            >
                              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
                                <polyline points="3 6 5 6 21 6"/>
                                <path d="M19 6l-1 14H6L5 6"/>
                                <path d="M10 11v6"/>
                                <path d="M14 11v6"/>
                                <path d="M9 6V4h6v2"/>
                              </svg>
                            </button>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Panel>
        </div>
      ) : (
        <MarketGroups />
      )}
    </div>
  );
}
