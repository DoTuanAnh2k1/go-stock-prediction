import { useState, useEffect, useCallback } from 'react';
import { Panel } from '../components/ui';
import { useLanguage } from '../context/LangContext';
import './MarketGroups.css';

// ── Types ─────────────────────────────────────────────────────────────────────

interface Command {
  id: number;
  name: string;
  description: string;
  handler_key: string;
  enabled: boolean;
}

interface CommandGroup {
  id: number;
  name: string;
  description: string;
  command_ids: number[];
}

interface User {
  id: number;
  username: string;
  role: string;
}

// ── Auth helpers ──────────────────────────────────────────────────────────────

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

// ── GroupUsersList ─────────────────────────────────────────────────────────────

interface CgStrings {
  usersInGroup: string;
  noUsers: string;
  removeUser: string;
}

function GroupUsersList({
  groupId,
  onRemove,
  refreshKey,
  cg,
}: {
  groupId: number;
  onRemove: (g: number, u: number) => void;
  refreshKey: number;
  cg: CgStrings;
}) {
  const [members, setMembers] = useState<User[]>([]);

  useEffect(() => {
    fetch(`/api/command-groups/${groupId}/users`, { headers: authHeaders() })
      .then(r => r.ok ? r.json() : null)
      .then(d => setMembers((unwrap(d) as User[]) ?? []));
  }, [groupId, refreshKey]);

  if (members.length === 0) {
    return (
      <div className="mg-expand-section">
        <div className="mg-expand-section__label">{cg.usersInGroup}</div>
        <div style={{ fontSize: 13, color: 'var(--text-3)', fontStyle: 'italic' }}>{cg.noUsers}</div>
      </div>
    );
  }

  return (
    <div className="mg-expand-section">
      <div className="mg-expand-section__label">{cg.usersInGroup}</div>
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
              {cg.removeUser}
            </button>
          </div>
        ))}
      </div>
    </div>
  );
}

// ── Main CommandGroups page ───────────────────────────────────────────────────

export default function CommandGroups() {
  const { t } = useLanguage();
  const cg = t.commandGroups;

  const [groups, setGroups] = useState<CommandGroup[]>([]);
  const [commands, setCommands] = useState<Command[]>([]);
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Create form
  const [newName, setNewName] = useState('');
  const [newDesc, setNewDesc] = useState('');

  // Edit state
  const [editId, setEditId] = useState<number | null>(null);
  const [editName, setEditName] = useState('');
  const [editDesc, setEditDesc] = useState('');

  // Expanded group management
  const [expandedId, setExpandedId] = useState<number | null>(null);
  const [selectedCommandIds, setSelectedCommandIds] = useState<number[]>([]);
  const [addUserId, setAddUserId] = useState('');
  const [memberRefresh, setMemberRefresh] = useState(0);

  const fetchGroups = useCallback(async () => {
    try {
      const res = await fetch('/api/command-groups', { headers: authHeaders() });
      if (!res.ok) throw new Error(cg.cannotLoad);
      const raw = await res.json();
      setGroups((unwrap(raw) as CommandGroup[]) ?? []);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : cg.cannotLoad);
    }
  }, [cg.cannotLoad]);

  const fetchCommands = useCallback(async () => {
    try {
      const res = await fetch('/api/commands', { headers: authHeaders() });
      if (!res.ok) return;
      const raw = await res.json();
      setCommands((unwrap(raw) as Command[]) ?? []);
    } catch { /* silent */ }
  }, []);

  const fetchUsers = useCallback(async () => {
    try {
      const res = await fetch('/api/users', { headers: authHeaders() });
      if (!res.ok) return;
      const raw = await res.json();
      const all: User[] = (unwrap(raw) as User[]) ?? [];
      setUsers(all.filter(u => u.role !== 'super_admin'));
    } catch { /* silent */ }
  }, []);

  useEffect(() => {
    setLoading(true);
    Promise.all([fetchGroups(), fetchCommands(), fetchUsers()]).finally(() => setLoading(false));
  }, [fetchGroups, fetchCommands, fetchUsers]);

  const createGroup = async () => {
    if (!newName.trim()) return;
    const res = await fetch('/api/command-groups', {
      method: 'POST', headers: authHeaders(),
      body: JSON.stringify({ name: newName.trim(), description: newDesc.trim() }),
    });
    if (res.ok) {
      setNewName('');
      setNewDesc('');
      fetchGroups();
    } else {
      const d = unwrap(await res.json().catch(() => ({}))) as { message?: string } | null;
      setError(d?.message || cg.createFail);
    }
  };

  const deleteGroup = async (id: number) => {
    if (!confirm(cg.deleteConfirm)) return;
    const res = await fetch(`/api/command-groups/${id}`, { method: 'DELETE', headers: authHeaders() });
    if (res.ok) {
      if (expandedId === id) setExpandedId(null);
      fetchGroups();
    } else {
      setError(cg.deleteFail);
    }
  };

  const saveEdit = async () => {
    if (editId == null) return;
    const res = await fetch(`/api/command-groups/${editId}`, {
      method: 'PUT', headers: authHeaders(),
      body: JSON.stringify({ name: editName.trim(), description: editDesc.trim() }),
    });
    if (res.ok) {
      setEditId(null);
      fetchGroups();
    } else {
      const d = unwrap(await res.json().catch(() => ({}))) as { message?: string } | null;
      setError(d?.message || cg.saveFail);
    }
  };

  const toggleExpand = (g: CommandGroup) => {
    if (expandedId === g.id) { setExpandedId(null); return; }
    setExpandedId(g.id);
    setSelectedCommandIds([...(g.command_ids ?? [])]);
    setAddUserId('');
  };

  const saveCommands = async (groupId: number) => {
    const res = await fetch(`/api/command-groups/${groupId}/commands`, {
      method: 'PUT', headers: authHeaders(),
      body: JSON.stringify({ command_ids: selectedCommandIds }),
    });
    if (res.ok) fetchGroups();
    else setError(cg.saveCommandsFail);
  };

  const addUser = async (groupId: number) => {
    const uid = parseInt(addUserId, 10);
    if (isNaN(uid)) return;
    const res = await fetch(`/api/command-groups/${groupId}/users`, {
      method: 'POST', headers: authHeaders(),
      body: JSON.stringify({ user_id: uid }),
    });
    if (res.ok) {
      setAddUserId('');
      fetchGroups();
      setMemberRefresh(n => n + 1);
    } else {
      setError(cg.addUserFail);
    }
  };

  const removeUser = async (groupId: number, userId: number) => {
    const res = await fetch(`/api/command-groups/${groupId}/users/${userId}`, {
      method: 'DELETE', headers: authHeaders(),
    });
    if (res.ok) {
      fetchGroups();
      setMemberRefresh(n => n + 1);
    }
  };

  const toggleCommandId = (id: number, checked: boolean) => {
    setSelectedCommandIds(prev =>
      checked ? [...prev, id] : prev.filter(x => x !== id)
    );
  };

  if (loading) {
    return <div style={{ padding: 40, textAlign: 'center', color: 'var(--text-2)', fontSize: 13 }}>{cg.loading}</div>;
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
      <Panel title={cg.createGroup}>
        <div style={{ maxWidth: 680 }}>
          <div style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))',
            gap: '10px 16px',
          }}>
            <div className="field" style={{ minWidth: 0 }}>
              <label className="field__label">
                {cg.groupName}<Req />
              </label>
              <input
                className="field__input"
                placeholder={cg.groupName}
                value={newName}
                onChange={e => setNewName(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && createGroup()}
              />
            </div>
            <div className="field" style={{ minWidth: 0 }}>
              <label className="field__label">
                {cg.groupDesc}
                <span style={{ color: 'var(--text-3)', fontWeight: 400 }}> ({cg.optional})</span>
              </label>
              <input
                className="field__input"
                placeholder={cg.groupDescPlaceholder}
                value={newDesc}
                onChange={e => setNewDesc(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && createGroup()}
              />
            </div>
          </div>
          <div style={{ marginTop: 12 }}>
            <button className="btn btn--primary" onClick={createGroup} disabled={!newName.trim()}>
              {cg.createBtn}
            </button>
          </div>
        </div>
      </Panel>

      {/* Group list */}
      <Panel
        title={t.nav.commandGroups}
        sub={groups.length > 0 ? `${groups.length} ${cg.groupCount}` : undefined}
      >
        {groups.length === 0 ? (
          <div style={{ padding: '16px 0', color: 'var(--text-3)', fontStyle: 'italic', fontSize: 13 }}>
            {cg.noGroups}
          </div>
        ) : (
          <div className="mg-group-list">
            {groups.map(g => (
              <div key={g.id} className="mg-group-card">
                {/* Card header */}
                <div className="mg-group-card__header">
                  {editId === g.id ? (
                    <div className="mg-edit-row">
                      <div className="field" style={{ flex: 1, minWidth: 0 }}>
                        <label className="field__label">{cg.groupName}</label>
                        <input
                          className="field__input"
                          value={editName}
                          onChange={e => setEditName(e.target.value)}
                          autoFocus
                        />
                      </div>
                      <div className="field" style={{ flex: 1, minWidth: 0 }}>
                        <label className="field__label">{cg.groupDesc}</label>
                        <input
                          className="field__input"
                          value={editDesc}
                          onChange={e => setEditDesc(e.target.value)}
                        />
                      </div>
                      <div className="mg-edit-actions">
                        <button className="btn btn--primary" onClick={saveEdit}>{cg.save}</button>
                        <button className="btn" onClick={() => setEditId(null)}>{cg.cancel}</button>
                      </div>
                    </div>
                  ) : (
                    <>
                      <div className="mg-group-info">
                        <span className="mg-group-name">{g.name}</span>
                        {g.description && (
                          <span style={{ fontSize: 12, color: 'var(--text-3)' }}>{g.description}</span>
                        )}
                        {(g.command_ids ?? []).length > 0 && (
                          <div className="mg-market-chips">
                            <span className="mg-market-chip">
                              {g.command_ids.length} {cg.commandsLabel}
                            </span>
                          </div>
                        )}
                      </div>
                      <div className="mg-group-actions">
                        <button
                          className="btn"
                          onClick={() => toggleExpand(g)}
                          style={{ fontSize: 12 }}
                        >
                          {expandedId === g.id ? cg.collapse : cg.manage}
                        </button>
                        <button
                          className="btn btn--ghost"
                          onClick={() => { setEditId(g.id); setEditName(g.name); setEditDesc(g.description); }}
                          style={{ fontSize: 12 }}
                        >
                          {cg.edit}
                        </button>
                        <button
                          className="btn btn--ghost"
                          onClick={() => deleteGroup(g.id)}
                          style={{ fontSize: 12, color: 'var(--down)' }}
                        >
                          {cg.delete}
                        </button>
                      </div>
                    </>
                  )}
                </div>

                {/* Expanded management panel */}
                {expandedId === g.id && (
                  <div className="mg-expand-panel">
                    {/* Commands section */}
                    <div className="mg-expand-section">
                      <div className="mg-expand-section__label">{cg.commandsSection}</div>
                      {commands.length === 0 ? (
                        <div style={{ fontSize: 13, color: 'var(--text-3)', fontStyle: 'italic' }}>{cg.noCommandsAvailable}</div>
                      ) : (
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                          {commands.map(cmd => (
                            <label key={cmd.id} className="mg-checkbox-label">
                              <input
                                type="checkbox"
                                checked={selectedCommandIds.includes(cmd.id)}
                                onChange={e => toggleCommandId(cmd.id, e.target.checked)}
                              />
                              <span style={{ fontSize: 13, fontWeight: 500 }}>{cmd.name}</span>
                              {cmd.description && (
                                <span style={{ fontSize: 12, color: 'var(--text-3)' }}>— {cmd.description}</span>
                              )}
                              {!cmd.enabled && (
                                <span className="badge badge--muted" style={{ fontSize: 10 }}>{cg.disabledLabel}</span>
                              )}
                            </label>
                          ))}
                        </div>
                      )}
                      <button
                        className="btn btn--primary"
                        style={{ marginTop: 8, fontSize: 12 }}
                        onClick={() => saveCommands(g.id)}
                      >
                        {cg.saveCommands}
                      </button>
                    </div>

                    {/* Add user section */}
                    <div className="mg-expand-section">
                      <div className="mg-expand-section__label">{cg.addUser}</div>
                      <div style={{ display: 'flex', gap: 8, alignItems: 'flex-end' }}>
                        <div className="field" style={{ flex: 1, minWidth: 0, margin: 0 }}>
                          <select
                            className="field__input"
                            value={addUserId}
                            onChange={e => setAddUserId(e.target.value)}
                            style={{ fontSize: 13 }}
                          >
                            <option value="">{cg.selectUser}</option>
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
                          {cg.addBtn}
                        </button>
                      </div>
                    </div>

                    {/* Users in group */}
                    <GroupUsersList
                      groupId={g.id}
                      onRemove={removeUser}
                      refreshKey={memberRefresh}
                      cg={cg}
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
