import { useState, useEffect, useCallback } from 'react';
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

export default function MarketGroups() {
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
    if (!confirm('Xóa group này?')) return;
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

  const addUser = async (groupId: number) => {
    const uid = parseInt(addUserId, 10);
    if (isNaN(uid)) return;
    const res = await fetch(`/api/market-groups/${groupId}/users`, {
      method: 'POST', headers: authHeaders(),
      body: JSON.stringify({ user_id: uid }),
    });
    if (res.ok) { setAddUserId(''); fetchGroups(); } else setError('Failed to add user');
  };

  const removeUser = async (groupId: number, userId: number) => {
    const res = await fetch(`/api/market-groups/${groupId}/users/${userId}`, {
      method: 'DELETE', headers: authHeaders(),
    });
    if (res.ok) fetchGroups();
  };

  if (loading) return <div className="mg-loading">Đang tải...</div>;

  return (
    <div className="mg-page">
      <h1 className="mg-title">Market Groups</h1>
      {error && (
        <div className="mg-error">
          {error}<button onClick={() => setError('')}>✕</button>
        </div>
      )}

      <div className="mg-create-form">
        <h2>Tạo group mới</h2>
        <input className="mg-input" placeholder="Tên group" value={newName} onChange={e => setNewName(e.target.value)} />
        <input className="mg-input" placeholder="Mô tả (tùy chọn)" value={newDesc} onChange={e => setNewDesc(e.target.value)} />
        <button className="mg-btn mg-btn--primary" onClick={createGroup}>Tạo</button>
      </div>

      <div className="mg-list">
        {groups.length === 0 && <p className="mg-empty">Chưa có group nào.</p>}
        {groups.map(g => (
          <div key={g.id} className="mg-card">
            <div className="mg-card-header">
              {editId === g.id ? (
                <div className="mg-edit-row">
                  <input className="mg-input" value={editName} onChange={e => setEditName(e.target.value)} />
                  <input className="mg-input" value={editDesc} onChange={e => setEditDesc(e.target.value)} />
                  <button className="mg-btn mg-btn--primary" onClick={saveEdit}>Lưu</button>
                  <button className="mg-btn" onClick={() => setEditId(null)}>Hủy</button>
                </div>
              ) : (
                <>
                  <div className="mg-card-info">
                    <span className="mg-card-name">{g.name}</span>
                    {g.description && <span className="mg-card-desc">{g.description}</span>}
                    <div className="mg-market-tags">
                      {(g.market_keys ?? []).map(k => <span key={k} className="mg-tag">{k}</span>)}
                    </div>
                  </div>
                  <div className="mg-card-actions">
                    <button className="mg-btn" onClick={() => toggleExpand(g)}>
                      {expandedId === g.id ? 'Thu gọn' : 'Quản lý'}
                    </button>
                    <button className="mg-btn" onClick={() => { setEditId(g.id); setEditName(g.name); setEditDesc(g.description); }}>Sửa</button>
                    <button className="mg-btn mg-btn--danger" onClick={() => deleteGroup(g.id)}>Xóa</button>
                  </div>
                </>
              )}
            </div>

            {expandedId === g.id && (
              <div className="mg-panel">
                <div className="mg-panel-section">
                  <h3>Markets</h3>
                  <div className="mg-checkboxes">
                    {MARKET_KEYS.map(k => (
                      <label key={k} className="mg-checkbox-label">
                        <input type="checkbox" checked={selectedMarkets.includes(k)}
                          onChange={e => {
                            if (e.target.checked) setSelectedMarkets(m => [...m, k]);
                            else setSelectedMarkets(m => m.filter(x => x !== k));
                          }} />
                        {k}
                      </label>
                    ))}
                  </div>
                  <button className="mg-btn mg-btn--primary" onClick={() => saveMarkets(g.id)}>Lưu markets</button>
                </div>

                <div className="mg-panel-section">
                  <h3>Thêm user</h3>
                  <div style={{ display: 'flex', gap: 8 }}>
                    <select className="mg-input" value={addUserId} onChange={e => setAddUserId(e.target.value)}>
                      <option value="">-- Chọn user --</option>
                      {users.map(u => <option key={u.id} value={u.id}>{u.username} ({u.role})</option>)}
                    </select>
                    <button className="mg-btn mg-btn--primary" onClick={() => addUser(g.id)}>Thêm</button>
                  </div>
                </div>

                <GroupUsersList groupId={g.id} onRemove={removeUser} />
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

function GroupUsersList({ groupId, onRemove }: { groupId: number; onRemove: (g: number, u: number) => void }) {
  const [members, setMembers] = useState<User[]>([]);

  useEffect(() => {
    fetch(`/api/market-groups/${groupId}/users`, { headers: authHeaders() })
      .then(r => r.ok ? r.json() : null)
      .then(d => setMembers((unwrap(d) as User[]) ?? []));
  }, [groupId]);

  if (members.length === 0) return <p className="mg-empty">Chưa có user.</p>;
  return (
    <div className="mg-panel-section">
      <h3>Users trong group</h3>
      <ul className="mg-user-list">
        {members.map(u => (
          <li key={u.id} className="mg-user-item">
            <span>{u.username}</span>
            <span className={`mg-role-badge mg-role-badge--${u.role.replace('_', '-')}`}>{u.role}</span>
            <button className="mg-btn mg-btn--danger mg-btn--sm" onClick={() => onRemove(groupId, u.id)}>Xóa</button>
          </li>
        ))}
      </ul>
    </div>
  );
}
