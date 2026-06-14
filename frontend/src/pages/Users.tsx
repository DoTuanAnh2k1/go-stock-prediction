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
}

function getToken() {
  return localStorage.getItem('vns_token') || '';
}

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
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState('');

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
    } catch (e: any) {
      setError(e.message);
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
        body: JSON.stringify({ username: newUsername, password: newPassword, role: newRole }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.message || t.users.createFail);
      setNewUsername('');
      setNewPassword('');
      setNewRole('user');
      fetchUsers();
    } catch (e: any) {
      setCreateError(e.message);
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

  return (
    <div>
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
          <Panel title={t.users.createUser}>
            <form onSubmit={handleCreate} style={{ display: 'flex', flexDirection: 'column', gap: 10, maxWidth: 360 }}>
              {createError && <div className="modal-error">{createError}</div>}
              <div className="field">
                <label className="field__label">{t.users.username}</label>
                <input
                  className="field__input"
                  value={newUsername}
                  onChange={e => setNewUsername(e.target.value)}
                  autoComplete="off"
                  required
                />
              </div>
              <div className="field">
                <label className="field__label">{t.users.password}</label>
                <input
                  className="field__input"
                  type="password"
                  value={newPassword}
                  onChange={e => setNewPassword(e.target.value)}
                  autoComplete="new-password"
                  required
                />
              </div>
              <div className="field">
                <label className="field__label">{t.users.role}</label>
                <select
                  className="field__input"
                  value={newRole}
                  onChange={e => setNewRole(e.target.value as 'user' | 'admin')}
                >
                  <option value="user">User</option>
                  <option value="admin">Admin</option>
                </select>
              </div>
              <button className="btn btn--primary" type="submit" disabled={creating}>
                {creating ? t.users.creating : t.users.createBtn}
              </button>
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
                      <th style={{ textAlign: 'left', padding: '8px 12px', color: 'var(--text-2)', fontWeight: 500 }}>{t.users.colRole}</th>
                      <th style={{ textAlign: 'left', padding: '8px 12px', color: 'var(--text-2)', fontWeight: 500 }}>{t.users.colCreatedAt}</th>
                      <th style={{ width: 48 }}></th>
                    </tr>
                  </thead>
                  <tbody>
                    {users.map(u => (
                      <tr key={u.id} style={{ borderBottom: '1px solid var(--border)' }}>
                        <td style={{ padding: '8px 12px', color: 'var(--text-2)', fontFamily: 'var(--font-mono)', fontSize: 12 }}>{u.id}</td>
                        <td style={{ padding: '8px 12px', fontWeight: 500 }}>{u.username}</td>
                        <td style={{ padding: '8px 12px' }}>
                          <span className={`role-badge role-badge--${u.role.replace('_', '-')}`}>{u.role}</span>
                        </td>
                        <td style={{ padding: '8px 12px', color: 'var(--text-2)', fontFamily: 'var(--font-mono)', fontSize: 12 }}>
                          {u.created_at ? new Date(u.created_at).toLocaleString('vi-VN') : '—'}
                        </td>
                        <td style={{ padding: '8px 12px' }}>
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
