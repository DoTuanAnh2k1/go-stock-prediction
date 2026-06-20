import { useState, useEffect, useCallback } from 'react';
import { Panel } from '../components/ui';
import { useLanguage } from '../context/LangContext';
import './MarketGroups.css';

const MARKET_KEYS = ['GOLD', 'NASDAQ', 'CRYPTO', 'SP500'];

interface MarketGroup {
  id: number;
  name: string;
  description: string;
  market_keys: string[];
}

interface User {
  id: number;
  username: string;
  role: string;
}

function getToken() { return localStorage.getItem('vns_token') ?? ''; }
function authHeaders(): Record<string, string> {
  return { Authorization: `Bearer ${getToken()}`, 'Content-Type': 'application/json' };
}

function unwrap(data: unknown): unknown {
  if (data && typeof data === 'object' && 'data' in data) {
    return (data as { data: unknown }).data;
  }
  return data;
}

function Req() {
  return <span style={{ color: 'var(--down)' }}> *</span>;
}

export default function MarketGroups() {
  const { t } = useLanguage();
  const mg = t.marketGroups;

  const [groups, setGroups] = useState<MarketGroup[]>([]);
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const [newName, setNewName] = useState('');
  const [newDesc, setNewDesc] = useState('');
  const [editId, setEditId] = useState<number | null>(null);
  const [editName, setEditName] = useState('');
  const [editDesc, setEditDesc] = useState('');
  const [expandedId, setExpandedId] = useState<number | null>(null);
  const [selectedMarkets, setSelectedMarkets] = useState<string[]>([]);
  const [addUserId, setAddUserId] = useState('');

  const fetchGroups = useCallback(async () => {
    try {
      const res = await fetch('/api/market-groups', { headers: authHeaders() });
      if (!res.ok) throw new Error('Failed to fetch groups');
      const raw = await res.json();
      setGroups((unwrap(raw) as MarketGroup[]) ?? []);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Unknown error');
    }
  }, []);

  const fetchUsers = useCallback(async () => {
    try {
      const res = await fetch('/api/users', { headers: authHeaders() });
      if (!res.ok) return;
      const raw = await res.json();
      const all: User[] = (unwrap(raw) as User[]) ?? [];
      setUsers(all.filter(u => u.role !== 'super_admin'));
    } catch {
      // silent
    }
  }, []);

  useEffect(() => {
    setLoading(true);
    Promise.all([fetchGroups(), fetchUsers()]).finally(() => setLoading(false));
  }, [fetchGroups, fetchUsers]);

  const createGroup = async () => {
    if (!newName.trim()) return;
    const res = await fetch('/api/market-groups', {
      method: 'POST', headers: authHeaders(),
      body: JSON.stringify({ name: newName.trim(), description: newDesc.trim() }),
    });
    if (res.ok) {
      setNewName('');
      setNewDesc('');
      fetchGroups();
    } else {
      const d = unwrap(await res.json().catch(() => ({}))) as { message?: string } | null;
      setError(d?.message || 'Failed to create group');
    }
  };

  const deleteGroup = async (id: number) => {
    if (!confirm(mg.deleteConfirm)) return;
    const res = await fetch(`/api/market-groups/${id}`, { method: 'DELETE', headers: authHeaders() });
    if (res.ok) fetchGroups();
  };

  const saveEdit = async () => {
    if (editId == null) return;
    const res = await fetch(`/api/market-groups/${editId}`, {
      method: 'PUT', headers: authHeaders(),
      body: JSON.stringify({ name: editName.trim(), description: editDesc.trim() }),
    });
    if (res.ok) {
      setEditId(null);
      fetchGroups();
    } else {
      const d = unwrap(await res.json().catch(() => ({}))) as { message?: string } | null;
      setError(d?.message || 'Failed');
    }
  };

  const toggleExpand = (g: MarketGroup) => {
    if (expandedId === g.id) { setExpandedId(null); return; }
    setExpandedId(g.id);
    setSelectedMarkets([...(g.market_keys ?? [])]);
    setAddUserId('');
  };

  const saveMarkets = async (groupId: number) => {
    const res = await fetch(`/api/market-groups/${groupId}/markets`, {
      method: 'PUT', headers: authHeaders(),
      body: JSON.stringify({ market_keys: selectedMarkets }),
    });
    if (res.ok) fetchGroups(); else setError('Failed to update markets');
  };

  const [memberRefresh, setMemberRefresh] = useState(0);

  const addUser = async (groupId: number) => {
    const uid = parseInt(addUserId, 10);
    if (isNaN(uid)) return;
    const res = await fetch(`/api/market-groups/${groupId}/users`, {
      method: 'POST', headers: authHeaders(),
      body: JSON.stringify({ user_id: uid }),
    });
    if (res.ok) {
      setAddUserId('');
      fetchGroups();
      setMemberRefresh(n => n + 1);
    } else setError('Failed to add user');
  };

  const removeUser = async (groupId: number, userId: number) => {
    const res = await fetch(`/api/market-groups/${groupId}/users/${userId}`, {
      method: 'DELETE', headers: authHeaders(),
    });
    if (res.ok) {
      fetchGroups();
      setMemberRefresh(n => n + 1);
    }
  };

  if (loading) {
    return <div style={{ padding: 40, textAlign: 'center', color: 'var(--text-2)', fontSize: 13 }}>{mg.loading}</div>;
  }

  return (
    <div className="mg-page">
      {error && (
        <div className="mg-error-banner">
          <span>{error}</span>
          <button className="btn btn--icon btn--ghost" onClick={() => setError('')} aria-label="Dismiss">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
          </button>
        </div>
      )}

      {/* Create group panel */}
      <Panel title={mg.createGroup}>
        <div style={{ maxWidth: 680 }}>
          <div style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))',
            gap: '10px 16px',
          }}>
            <div className="field" style={{ minWidth: 0 }}>
              <label className="field__label">
                {mg.groupName}<Req />
              </label>
              <input
                className="field__input"
                placeholder={mg.groupName}
                value={newName}
                onChange={e => setNewName(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && createGroup()}
              />
            </div>
            <div className="field" style={{ minWidth: 0 }}>
              <label className="field__label">
                {mg.groupDesc}
                <span style={{ color: 'var(--text-3)', fontWeight: 400 }}> ({mg.optional})</span>
              </label>
              <input
                className="field__input"
                placeholder={mg.groupDescPlaceholder}
                value={newDesc}
                onChange={e => setNewDesc(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && createGroup()}
              />
            </div>
          </div>
          <div style={{ marginTop: 12 }}>
            <button className="btn btn--primary" onClick={createGroup} disabled={!newName.trim()}>
              {mg.createBtn}
            </button>
          </div>
        </div>
      </Panel>

      {/* Group list */}
      <Panel
        title="Market Groups"
        sub={groups.length > 0 ? `${groups.length} ${mg.groupCount}` : undefined}
      >
        {groups.length === 0 ? (
          <div style={{ padding: '16px 0', color: 'var(--text-3)', fontStyle: 'italic', fontSize: 13 }}>
            {mg.noGroups}
          </div>
        ) : (
          <div className="mg-group-list">
            {groups.map(g => (
              <div key={g.id} className="mg-group-card">
                {/* Card header */}
                <div className="mg-group-card__header">
                  {editId === g.id ? (
                    /* Inline edit row */
                    <div className="mg-edit-row">
                      <div className="field" style={{ flex: 1, minWidth: 0 }}>
                        <label className="field__label">{mg.groupName}</label>
                        <input
                          className="field__input"
                          value={editName}
                          onChange={e => setEditName(e.target.value)}
                          autoFocus
                        />
                      </div>
                      <div className="field" style={{ flex: 1, minWidth: 0 }}>
                        <label className="field__label">{mg.groupDesc}</label>
                        <input
                          className="field__input"
                          value={editDesc}
                          onChange={e => setEditDesc(e.target.value)}
                        />
                      </div>
                      <div className="mg-edit-actions">
                        <button className="btn btn--primary" onClick={saveEdit}>{mg.save}</button>
                        <button className="btn" onClick={() => setEditId(null)}>{mg.cancel}</button>
                      </div>
                    </div>
                  ) : (
                    <>
                      <div className="mg-group-info">
                        <span className="mg-group-name">{g.name}</span>
                        {g.description && (
                          <span style={{ fontSize: 12, color: 'var(--text-3)' }}>{g.description}</span>
                        )}
                        {(g.market_keys ?? []).length > 0 && (
                          <div className="mg-market-chips">
                            {(g.market_keys ?? []).map(k => (
                              <span key={k} className="mg-market-chip">{k}</span>
                            ))}
                          </div>
                        )}
                      </div>
                      <div className="mg-group-actions">
                        <button
                          className="btn"
                          onClick={() => toggleExpand(g)}
                          style={{ fontSize: 12 }}
                        >
                          {expandedId === g.id ? mg.collapse : mg.manage}
                        </button>
                        <button
                          className="btn btn--ghost"
                          onClick={() => { setEditId(g.id); setEditName(g.name); setEditDesc(g.description); }}
                          style={{ fontSize: 12 }}
                        >
                          {mg.edit}
                        </button>
                        <button
                          className="btn btn--ghost"
                          onClick={() => deleteGroup(g.id)}
                          style={{ fontSize: 12, color: 'var(--down)' }}
                        >
                          {mg.delete}
                        </button>
                      </div>
                    </>
                  )}
                </div>

                {/* Expanded management panel */}
                {expandedId === g.id && (
                  <div className="mg-expand-panel">
                    {/* Markets section */}
                    <div className="mg-expand-section">
                      <div className="mg-expand-section__label">{mg.marketsSection}</div>
                      <div className="mg-checkboxes">
                        {MARKET_KEYS.map(k => (
                          <label key={k} className="mg-checkbox-label">
                            <input
                              type="checkbox"
                              checked={selectedMarkets.includes(k)}
                              onChange={e => {
                                if (e.target.checked) setSelectedMarkets(m => [...m, k]);
                                else setSelectedMarkets(m => m.filter(x => x !== k));
                              }}
                            />
                            <span style={{ fontSize: 13, fontWeight: 500 }}>{k}</span>
                          </label>
                        ))}
                      </div>
                      <button
                        className="btn btn--primary"
                        style={{ marginTop: 8, fontSize: 12 }}
                        onClick={() => saveMarkets(g.id)}
                      >
                        {mg.saveMarkets}
                      </button>
                    </div>

                    {/* Add user section */}
                    <div className="mg-expand-section">
                      <div className="mg-expand-section__label">{mg.addUser}</div>
                      <div style={{ display: 'flex', gap: 8, alignItems: 'flex-end' }}>
                        <div className="field" style={{ flex: 1, minWidth: 0, margin: 0 }}>
                          <select
                            className="field__input"
                            value={addUserId}
                            onChange={e => setAddUserId(e.target.value)}
                            style={{ fontSize: 13 }}
                          >
                            <option value="">{mg.selectUser}</option>
                            {users.map(u => (
                              <option key={u.id} value={u.id}>
                                {u.username} ({u.role})
                              </option>
                            ))}
                          </select>
                        </div>
                        <button
                          className="btn btn--primary"
                          style={{ fontSize: 12, whiteSpace: 'nowrap' }}
                          onClick={() => addUser(g.id)}
                          disabled={!addUserId}
                        >
                          {mg.addBtn}
                        </button>
                      </div>
                    </div>

                    {/* Users in group */}
                    <GroupUsersList
                      groupId={g.id}
                      onRemove={removeUser}
                      refreshKey={memberRefresh}
                      mg={mg}
                    />
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </Panel>
    </div>
  );
}

interface MgStrings {
  usersInGroup: string;
  noUsers: string;
  removeUser: string;
}

function GroupUsersList({
  groupId,
  onRemove,
  refreshKey,
  mg,
}: {
  groupId: number;
  onRemove: (g: number, u: number) => void;
  refreshKey: number;
  mg: MgStrings;
}) {
  const [members, setMembers] = useState<User[]>([]);

  useEffect(() => {
    fetch(`/api/market-groups/${groupId}/users`, { headers: authHeaders() })
      .then(r => r.ok ? r.json() : null)
      .then(d => setMembers((unwrap(d) as User[]) ?? []));
  }, [groupId, refreshKey]);

  if (members.length === 0) {
    return (
      <div className="mg-expand-section">
        <div className="mg-expand-section__label">{mg.usersInGroup}</div>
        <div style={{ fontSize: 13, color: 'var(--text-3)', fontStyle: 'italic' }}>{mg.noUsers}</div>
      </div>
    );
  }

  return (
    <div className="mg-expand-section">
      <div className="mg-expand-section__label">{mg.usersInGroup}</div>
      <div className="mg-user-list">
        {members.map(u => (
          <div key={u.id} className="mg-user-row">
            <span style={{ fontWeight: 500, fontSize: 13 }}>{u.username}</span>
            <span className={`role-badge role-badge--${u.role.replace('_', '-')}`}>{u.role}</span>
            <button
              className="btn btn--ghost"
              style={{ fontSize: 11, color: 'var(--down)', marginLeft: 'auto' }}
              onClick={() => onRemove(groupId, u.id)}
            >
              {mg.removeUser}
            </button>
          </div>
        ))}
      </div>
    </div>
  );
}
