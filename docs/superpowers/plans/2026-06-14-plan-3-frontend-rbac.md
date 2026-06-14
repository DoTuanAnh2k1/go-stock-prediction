# Frontend RBAC + Docker Compose Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cập nhật frontend để đọc `accessible_markets` từ JWT, ẩn/hiện market tabs theo quyền, thêm trang Market Groups management cho admin, cập nhật trang Users, và wire auth service vào docker-compose.

**Architecture:** `AuthContext` decode JWT client-side để extract `accessible_markets: string[]`. Sidebar ẩn tab GOLD/NASDAQ/CRYPTO/SP500 nếu không trong list. Trang `/admin/market-groups` (admin+) gọi API `/api/market-groups/*`. Docker Compose thêm service `auth` port 8120 internal, `api` depends on `auth`.

**Tech Stack:** React 18, TypeScript, jwtDecode từ `jwt-decode` package (đã có hoặc cần install), Vite, CSS existing.

**Prerequisite:** Plan 1 + Plan 2 đã complete. Java Auth Service chạy và Go API proxy hoạt động.

---

## File Map

### Tạo mới
```
frontend/src/pages/MarketGroups.tsx
frontend/src/pages/MarketGroups.css
```

### Sửa đổi
```
frontend/src/context/AuthContext.tsx     ← thêm accessibleMarkets, role expose
frontend/src/components/ui/Sidebar.tsx  ← ẩn market tabs theo accessibleMarkets
frontend/src/pages/Users.tsx            ← disable delete/edit cho super_admin
frontend/src/i18n.ts                    ← thêm translations cho MarketGroups
frontend/src/App.tsx                    ← thêm route /admin/market-groups
docker-compose.yml                      ← thêm auth service
.env                                    ← thêm AUTH_GRPC_TARGET
```

---

## Task 1: Cài jwt-decode + cập nhật AuthContext

**Files:**
- Modify: `frontend/src/context/AuthContext.tsx`

- [ ] **Kiểm tra jwt-decode đã có chưa:**
```bash
cd frontend && cat package.json | grep jwt
```

Nếu chưa có:
```bash
cd frontend && npm install jwt-decode
```

- [ ] **Thay thế toàn bộ `frontend/src/context/AuthContext.tsx`:**

```tsx
import { createContext, useContext, useState, useEffect, ReactNode } from 'react';
import { jwtDecode } from 'jwt-decode';

interface AuthUser {
  username: string;
  role: string;
}

interface JwtPayload {
  sub: string;
  username: string;
  role: string;
  user_id: number;
  accessible_markets: string[];
  exp: number;
}

interface AuthContextValue {
  user: AuthUser | null;
  isLoggedIn: boolean;
  role: string;
  accessibleMarkets: string[];
  canAccessMarket: (marketKey: string) => boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue>({
  user: null,
  isLoggedIn: false,
  role: '',
  accessibleMarkets: [],
  canAccessMarket: () => false,
  login: async () => {},
  logout: () => {},
});

function parseJwt(token: string): JwtPayload | null {
  try {
    return jwtDecode<JwtPayload>(token);
  } catch {
    return null;
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(() => {
    const stored = localStorage.getItem('vns_user');
    if (!stored) return null;
    try {
      const parsed = JSON.parse(stored);
      return { username: parsed.username || '', role: parsed.role || 'user' };
    } catch {
      return null;
    }
  });

  const [accessibleMarkets, setAccessibleMarkets] = useState<string[]>(() => {
    const token = localStorage.getItem('vns_token');
    if (!token) return [];
    const payload = parseJwt(token);
    return payload?.accessible_markets ?? [];
  });

  // Validate stored token on mount
  useEffect(() => {
    const token = localStorage.getItem('vns_token');
    if (!token) { setUser(null); setAccessibleMarkets([]); return; }
    fetch('/api/auth/me', { headers: { Authorization: `Bearer ${token}` } })
      .then(r => r.ok ? r.json() : null)
      .then(data => {
        if (data?.username) {
          setUser({ username: data.username, role: data.role || 'user' });
          // Re-parse JWT for markets (MeHandler returns only username+role)
          const payload = parseJwt(token);
          setAccessibleMarkets(payload?.accessible_markets ?? []);
        } else {
          localStorage.removeItem('vns_token');
          localStorage.removeItem('vns_user');
          setUser(null);
          setAccessibleMarkets([]);
        }
      })
      .catch(() => {});
  }, []);

  const login = async (username: string, password: string) => {
    const res = await fetch('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      throw new Error((err as any).message || 'Đăng nhập thất bại');
    }
    const data = await res.json();
    const userObj: AuthUser = {
      username: data.user?.username || username,
      role: data.user?.role || 'user',
    };
    localStorage.setItem('vns_token', data.token);
    localStorage.setItem('vns_user', JSON.stringify(userObj));
    setUser(userObj);

    // Decode JWT to extract accessible_markets
    const payload = parseJwt(data.token);
    setAccessibleMarkets(payload?.accessible_markets ?? []);
  };

  const logout = () => {
    localStorage.removeItem('vns_token');
    localStorage.removeItem('vns_user');
    setUser(null);
    setAccessibleMarkets([]);
  };

  const role = user?.role ?? '';

  const canAccessMarket = (marketKey: string): boolean => {
    if (role === 'super_admin') return true;
    return accessibleMarkets.includes(marketKey);
  };

  return (
    <AuthContext.Provider value={{
      user, isLoggedIn: !!user, role, accessibleMarkets, canAccessMarket, login, logout
    }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() { return useContext(AuthContext); }
```

- [ ] **Kiểm tra TypeScript compile:**
```bash
cd frontend && npx tsc --noEmit 2>&1 | head -30
```

- [ ] **Commit:**
```bash
git add frontend/src/context/AuthContext.tsx frontend/package.json frontend/package-lock.json
git commit -m "feat(auth): AuthContext — add accessibleMarkets, canAccessMarket from JWT decode"
```

---

## Task 2: Cập nhật Sidebar — ẩn market tabs

**Files:**
- Modify: `frontend/src/components/ui/Sidebar.tsx` (hoặc file tương đương chứa sidebar links)

- [ ] **Tìm file Sidebar:**
```bash
find /home/chronical/Projects/private/go-stock-prediction/frontend/src -name "Sidebar*" | head -5
cat <path-found>/Sidebar.tsx | head -80
```

- [ ] **Trong Sidebar component, import và dùng `useAuth`:**

```tsx
import { useAuth } from '../../context/AuthContext';

// Trong component function:
const { canAccessMarket, isLoggedIn } = useAuth();
```

- [ ] **Wrap mỗi market nav item với điều kiện:**

Tìm các link/item cho Gold, NASDAQ, Crypto, SP500 và thêm điều kiện:

```tsx
{/* Gold — chỉ hiện khi có quyền */}
{canAccessMarket('GOLD') && (
  <NavItem to="/markets/gold" icon={<GoldIcon />} label={tr.nav.gold} />
)}

{/* NASDAQ */}
{canAccessMarket('NASDAQ') && (
  <NavItem to="/markets/nasdaq100" icon={<NasdaqIcon />} label={tr.nav.nasdaq} />
)}

{/* Crypto */}
{canAccessMarket('CRYPTO') && (
  <NavItem to="/markets/crypto" icon={<CryptoIcon />} label={tr.nav.crypto} />
)}

{/* SP500 */}
{canAccessMarket('SP500') && (
  <NavItem to="/markets/sp500" icon={<SP500Icon />} label={tr.nav.sp500} />
)}

{/* Market Groups — chỉ hiện với admin và super_admin */}
{isLoggedIn && (role === 'admin' || role === 'super_admin') && (
  <NavItem to="/admin/market-groups" icon={<GroupIcon />} label={tr.nav.marketGroups} />
)}
```

Trong đoạn trên, `role` lấy từ `useAuth()`:
```tsx
const { canAccessMarket, isLoggedIn, role } = useAuth();
```

- [ ] **Kiểm tra TypeScript compile + Vite build:**
```bash
cd frontend && npx tsc --noEmit 2>&1 | head -20
```

- [ ] **Commit:**
```bash
git add frontend/src/components/ui/Sidebar.tsx
git commit -m "feat(auth): sidebar — hide market tabs based on accessibleMarkets JWT claims"
```

---

## Task 3: Trang MarketGroups.tsx

**Files:**
- Create: `frontend/src/pages/MarketGroups.tsx`
- Create: `frontend/src/pages/MarketGroups.css`

- [ ] **Tạo `frontend/src/pages/MarketGroups.tsx`:**

```tsx
import { useState, useEffect, useCallback } from 'react';
import './MarketGroups.css';

const API = (token: string) => ({
  headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
});

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

const MARKET_KEYS = ['GOLD', 'NASDAQ', 'CRYPTO', 'SP500'];

export default function MarketGroups() {
  const token = localStorage.getItem('vns_token') ?? '';
  const [groups, setGroups] = useState<MarketGroup[]>([]);
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

  // Expanded group for managing markets + users
  const [expandedId, setExpandedId] = useState<number | null>(null);
  const [selectedMarkets, setSelectedMarkets] = useState<string[]>([]);
  const [addUserId, setAddUserId] = useState('');

  const fetchGroups = useCallback(async () => {
    try {
      const res = await fetch('/api/market-groups', { headers: API(token).headers });
      if (!res.ok) throw new Error('Failed to fetch groups');
      const data = await res.json();
      setGroups(data.data ?? data ?? []);
    } catch (e: any) {
      setError(e.message);
    }
  }, [token]);

  const fetchUsers = useCallback(async () => {
    try {
      const res = await fetch('/api/users', { headers: API(token).headers });
      if (!res.ok) return;
      const data = await res.json();
      // Filter out super_admin from the list shown
      const all: User[] = data.data ?? data ?? [];
      setUsers(all.filter(u => u.role !== 'super_admin'));
    } catch {}
  }, [token]);

  useEffect(() => {
    setLoading(true);
    Promise.all([fetchGroups(), fetchUsers()]).finally(() => setLoading(false));
  }, [fetchGroups, fetchUsers]);

  const createGroup = async () => {
    if (!newName.trim()) return;
    const res = await fetch('/api/market-groups', {
      method: 'POST',
      headers: API(token).headers,
      body: JSON.stringify({ name: newName.trim(), description: newDesc.trim() }),
    });
    if (res.ok) {
      setNewName(''); setNewDesc('');
      fetchGroups();
    } else {
      const d = await res.json().catch(() => ({}));
      setError(d.message || 'Failed to create group');
    }
  };

  const deleteGroup = async (id: number) => {
    if (!confirm('Xóa group này?')) return;
    const res = await fetch(`/api/market-groups/${id}`, {
      method: 'DELETE', headers: API(token).headers,
    });
    if (res.ok) fetchGroups();
  };

  const startEdit = (g: MarketGroup) => {
    setEditId(g.id); setEditName(g.name); setEditDesc(g.description);
  };

  const saveEdit = async () => {
    if (editId == null) return;
    const res = await fetch(`/api/market-groups/${editId}`, {
      method: 'PUT',
      headers: API(token).headers,
      body: JSON.stringify({ name: editName.trim(), description: editDesc.trim() }),
    });
    if (res.ok) { setEditId(null); fetchGroups(); }
    else {
      const d = await res.json().catch(() => ({}));
      setError(d.message || 'Failed to update group');
    }
  };

  const toggleExpand = (g: MarketGroup) => {
    if (expandedId === g.id) { setExpandedId(null); return; }
    setExpandedId(g.id);
    setSelectedMarkets([...g.market_keys]);
    setAddUserId('');
  };

  const saveMarkets = async (groupId: number) => {
    const res = await fetch(`/api/market-groups/${groupId}/markets`, {
      method: 'PUT',
      headers: API(token).headers,
      body: JSON.stringify({ market_keys: selectedMarkets }),
    });
    if (res.ok) fetchGroups();
    else setError('Failed to update markets');
  };

  const addUserToGroup = async (groupId: number) => {
    const uid = parseInt(addUserId, 10);
    if (isNaN(uid)) return;
    const res = await fetch(`/api/market-groups/${groupId}/users`, {
      method: 'POST',
      headers: API(token).headers,
      body: JSON.stringify({ user_id: uid }),
    });
    if (res.ok) { setAddUserId(''); fetchGroups(); }
    else setError('Failed to add user');
  };

  const removeUserFromGroup = async (groupId: number, userId: number) => {
    const res = await fetch(`/api/market-groups/${groupId}/users/${userId}`, {
      method: 'DELETE', headers: API(token).headers,
    });
    if (res.ok) fetchGroups();
  };

  if (loading) return <div className="mg-loading">Đang tải...</div>;

  return (
    <div className="mg-page">
      <h1 className="mg-title">Market Groups</h1>
      {error && <div className="mg-error">{error}<button onClick={() => setError('')}>✕</button></div>}

      {/* Create form */}
      <div className="mg-create-form">
        <h2>Tạo group mới</h2>
        <input
          className="mg-input" placeholder="Tên group"
          value={newName} onChange={e => setNewName(e.target.value)}
        />
        <input
          className="mg-input" placeholder="Mô tả (tùy chọn)"
          value={newDesc} onChange={e => setNewDesc(e.target.value)}
        />
        <button className="mg-btn mg-btn--primary" onClick={createGroup}>Tạo</button>
      </div>

      {/* Group list */}
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
                    <span className="mg-card-desc">{g.description}</span>
                    <div className="mg-market-tags">
                      {g.market_keys.map(k => (
                        <span key={k} className="mg-tag">{k}</span>
                      ))}
                    </div>
                  </div>
                  <div className="mg-card-actions">
                    <button className="mg-btn" onClick={() => toggleExpand(g)}>
                      {expandedId === g.id ? 'Thu gọn' : 'Quản lý'}
                    </button>
                    <button className="mg-btn" onClick={() => startEdit(g)}>Sửa</button>
                    <button className="mg-btn mg-btn--danger" onClick={() => deleteGroup(g.id)}>Xóa</button>
                  </div>
                </>
              )}
            </div>

            {/* Expanded panel */}
            {expandedId === g.id && (
              <div className="mg-panel">
                <div className="mg-panel-section">
                  <h3>Markets</h3>
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
                        {k}
                      </label>
                    ))}
                  </div>
                  <button className="mg-btn mg-btn--primary" onClick={() => saveMarkets(g.id)}>
                    Lưu markets
                  </button>
                </div>

                <div className="mg-panel-section">
                  <h3>Thêm user vào group</h3>
                  <select
                    className="mg-input"
                    value={addUserId}
                    onChange={e => setAddUserId(e.target.value)}
                  >
                    <option value="">-- Chọn user --</option>
                    {users.map(u => (
                      <option key={u.id} value={u.id}>
                        {u.username} ({u.role})
                      </option>
                    ))}
                  </select>
                  <button className="mg-btn mg-btn--primary" onClick={() => addUserToGroup(g.id)}>
                    Thêm
                  </button>
                </div>

                <GroupUsers groupId={g.id} token={token} onRemove={removeUserFromGroup} />
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

function GroupUsers({ groupId, token, onRemove }: {
  groupId: number;
  token: string;
  onRemove: (groupId: number, userId: number) => void;
}) {
  const [users, setUsers] = useState<User[]>([]);

  useEffect(() => {
    fetch(`/api/market-groups/${groupId}/users`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then(r => r.ok ? r.json() : null)
      .then(d => setUsers(d?.data ?? d ?? []));
  }, [groupId, token]);

  if (users.length === 0) return <p className="mg-empty">Group chưa có user nào.</p>;
  return (
    <div className="mg-panel-section">
      <h3>Users trong group</h3>
      <ul className="mg-user-list">
        {users.map(u => (
          <li key={u.id} className="mg-user-item">
            <span>{u.username}</span>
            <span className="mg-role-badge mg-role-badge--{u.role}">{u.role}</span>
            <button className="mg-btn mg-btn--danger mg-btn--sm"
              onClick={() => onRemove(groupId, u.id)}>
              Xóa
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}
```

- [ ] **Tạo `frontend/src/pages/MarketGroups.css`:**

```css
.mg-page { padding: 24px; max-width: 900px; }
.mg-title { font-size: 1.5rem; font-weight: 700; margin-bottom: 24px; }
.mg-loading { padding: 40px; text-align: center; opacity: 0.6; }
.mg-empty { opacity: 0.5; font-style: italic; }
.mg-error {
  background: var(--color-danger, #e53e3e);
  color: white;
  padding: 10px 16px;
  border-radius: 6px;
  margin-bottom: 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.mg-error button { background: none; border: none; color: white; cursor: pointer; font-size: 1rem; }

.mg-create-form {
  background: var(--color-surface, #1e2130);
  border: 1px solid var(--color-border, #2d3048);
  border-radius: 10px;
  padding: 20px;
  margin-bottom: 24px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.mg-create-form h2 { font-size: 1rem; font-weight: 600; margin: 0 0 4px; }

.mg-input {
  background: var(--color-bg, #131622);
  border: 1px solid var(--color-border, #2d3048);
  border-radius: 6px;
  padding: 8px 12px;
  color: inherit;
  font-size: 0.875rem;
  width: 100%;
  box-sizing: border-box;
}

.mg-btn {
  padding: 7px 14px;
  border-radius: 6px;
  border: 1px solid var(--color-border, #2d3048);
  background: var(--color-surface, #1e2130);
  color: inherit;
  cursor: pointer;
  font-size: 0.8rem;
  transition: opacity 0.15s;
}
.mg-btn:hover { opacity: 0.8; }
.mg-btn--primary {
  background: var(--accent-override, #5B8DEF);
  border-color: transparent;
  color: white;
}
.mg-btn--danger { background: #c53030; border-color: transparent; color: white; }
.mg-btn--sm { padding: 4px 10px; font-size: 0.75rem; }

.mg-list { display: flex; flex-direction: column; gap: 12px; }
.mg-card {
  background: var(--color-surface, #1e2130);
  border: 1px solid var(--color-border, #2d3048);
  border-radius: 10px;
  overflow: hidden;
}
.mg-card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 16px;
  gap: 12px;
}
.mg-card-info { flex: 1; display: flex; flex-direction: column; gap: 4px; }
.mg-card-name { font-weight: 600; font-size: 0.95rem; }
.mg-card-desc { font-size: 0.8rem; opacity: 0.6; }
.mg-card-actions { display: flex; gap: 6px; flex-shrink: 0; }
.mg-edit-row { display: flex; gap: 8px; align-items: center; flex: 1; }

.mg-market-tags { display: flex; gap: 6px; flex-wrap: wrap; margin-top: 4px; }
.mg-tag {
  background: var(--accent-override, #5B8DEF)22;
  color: var(--accent-override, #5B8DEF);
  border: 1px solid var(--accent-override, #5B8DEF)44;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 600;
}

.mg-panel {
  border-top: 1px solid var(--color-border, #2d3048);
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}
.mg-panel-section { display: flex; flex-direction: column; gap: 8px; }
.mg-panel-section h3 { font-size: 0.85rem; font-weight: 600; opacity: 0.7; margin: 0; }
.mg-checkboxes { display: flex; gap: 16px; flex-wrap: wrap; }
.mg-checkbox-label { display: flex; align-items: center; gap: 6px; font-size: 0.875rem; cursor: pointer; }

.mg-user-list { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: 6px; }
.mg-user-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 10px;
  background: var(--color-bg, #131622);
  border-radius: 6px;
}
.mg-user-item span:first-child { flex: 1; font-size: 0.875rem; }
.mg-role-badge {
  font-size: 0.7rem;
  padding: 2px 8px;
  border-radius: 4px;
  background: var(--color-surface, #1e2130);
  border: 1px solid var(--color-border, #2d3048);
}
```

- [ ] **Commit:**
```bash
git add frontend/src/pages/MarketGroups.tsx frontend/src/pages/MarketGroups.css
git commit -m "feat(auth): add MarketGroups page — CRUD groups, assign markets and users"
```

---

## Task 4: Cập nhật Users.tsx — disable super_admin actions

**Files:**
- Modify: `frontend/src/pages/Users.tsx`

- [ ] **Trong `Users.tsx`, import `useAuth` và lấy `role`:**

```tsx
import { useAuth } from '../context/AuthContext';
// trong component:
const { role: callerRole } = useAuth();
```

- [ ] **Thêm role badge column và disable delete cho super_admin:**

```tsx
// Trong table row rendering:
<tr key={user.id}>
  <td>{user.id}</td>
  <td>{user.username}</td>
  <td>
    <span className={`role-badge role-badge--${user.role}`}>
      {user.role}
    </span>
  </td>
  <td>{user.created_at}</td>
  <td>
    {/* Disable delete nếu target là super_admin và caller không phải super_admin */}
    <button
      onClick={() => handleDelete(user.id)}
      disabled={user.role === 'super_admin' && callerRole !== 'super_admin'}
      title={user.role === 'super_admin' && callerRole !== 'super_admin'
        ? 'Không thể xóa super_admin'
        : 'Xóa user'}
    >
      Xóa
    </button>
  </td>
</tr>
```

- [ ] **Thêm CSS cho role badges (trong file CSS của Users hoặc global):**

```css
.role-badge {
  font-size: 0.7rem;
  padding: 2px 8px;
  border-radius: 4px;
  font-weight: 600;
}
.role-badge--super_admin { background: #805ad5; color: white; }
.role-badge--admin       { background: #2b6cb0; color: white; }
.role-badge--user        { background: #2d3748; color: #a0aec0; }
```

- [ ] **Kiểm tra TypeScript:**
```bash
cd frontend && npx tsc --noEmit 2>&1 | head -20
```

- [ ] **Commit:**
```bash
git add frontend/src/pages/Users.tsx
git commit -m "feat(auth): Users page — role badge, disable delete for super_admin"
```

---

## Task 5: i18n + App.tsx route

**Files:**
- Modify: `frontend/src/i18n.ts`
- Modify: `frontend/src/App.tsx`

- [ ] **Trong `frontend/src/i18n.ts`, thêm key `marketGroups` vào nav section:**

```ts
// Trong object VI:
nav: {
  // ... existing keys ...
  marketGroups: 'Nhóm thị trường',
}

// Trong object EN:
nav: {
  // ... existing keys ...
  marketGroups: 'Market Groups',
}
```

- [ ] **Trong `frontend/src/App.tsx`, thêm import và route:**

```tsx
import MarketGroups from './pages/MarketGroups';

// Trong <Routes>:
{/* ── Admin ────────────────────────────────────────────── */}
<Route path="/admin/users" element={<ErrorBoundary><Users /></ErrorBoundary>} />
<Route path="/admin/market-groups" element={<ErrorBoundary><MarketGroups /></ErrorBoundary>} />
```

- [ ] **Kiểm tra build:**
```bash
cd frontend && npm run build 2>&1 | tail -20
```
Expected: `✓ built in ...`

- [ ] **Commit:**
```bash
git add frontend/src/i18n.ts frontend/src/App.tsx
git commit -m "feat(auth): add /admin/market-groups route and i18n key"
```

---

## Task 6: Docker Compose — wire auth service

**Files:**
- Modify: `docker-compose.yml`
- Modify: `.env`

- [ ] **Đọc `docker-compose.yml` hiện tại:**
```bash
cat docker-compose.yml
```

- [ ] **Thêm service `auth` và update service `api` trong `docker-compose.yml`:**

```yaml
services:
  # ... db, prediction giữ nguyên ...

  auth:
    build: ./auth-service
    depends_on:
      db:
        condition: service_healthy
    environment:
      DB_HOST: db
      DB_PORT: "3306"
      DB_NAME: ${MYSQL_DB_NAME:-go_stock_prediction}
      DB_USER: ${MYSQL_USER:-root}
      DB_PASSWORD: ${MYSQL_PASSWORD:-123}
      JWT_SECRET: ${JWT_SECRET:-change-me-in-production}
      GRPC_PORT: "8120"
    expose:
      - "8120"
    restart: unless-stopped
    networks:
      - default

  api:
    build: ./api
    depends_on:
      db:
        condition: service_healthy
      prediction:
        condition: service_started
      auth:                          # ← thêm dependency
        condition: service_started
    environment:
      # ... env vars hiện tại giữ nguyên ...
      AUTH_GRPC_TARGET: auth:8120    # ← thêm
    # ... phần còn lại giữ nguyên
```

- [ ] **Thêm vào `.env`:**
```
# Auth Service gRPC
AUTH_GRPC_TARGET=auth:8120
```

- [ ] **Commit:**
```bash
git add docker-compose.yml .env
git commit -m "feat(auth): docker-compose — add auth service, wire AUTH_GRPC_TARGET to api"
```

---

## Task 7: Integration test

- [ ] **Build tất cả images:**
```bash
docker compose build
```
Expected: tất cả build thành công, không có error.

- [ ] **Start stack:**
```bash
docker compose up -d
sleep 15  # chờ services start
```

- [ ] **Kiểm tra auth service khởi động:**
```bash
docker compose logs auth | grep -E "(Started|ERROR|Super admin|Flyway)"
```
Expected:
- `Flyway ... Successfully applied 1 migration`
- `Super admin 'chon' seeded successfully`
- `Started AuthServiceApplication`

- [ ] **Test login với super_admin:**
```bash
TOKEN=$(curl -s -X POST http://localhost:8118/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"chon","password":"Ch1nch2n@"}' | jq -r '.token')
echo "Token: $TOKEN" | head -c 50
echo "..."
```
Expected: token không empty.

- [ ] **Decode và kiểm tra accessible_markets:**
```bash
echo $TOKEN | cut -d. -f2 | base64 -d 2>/dev/null | python3 -m json.tool | grep -A5 accessible_markets
```
Expected: `"accessible_markets": ["GOLD", "NASDAQ", "CRYPTO", "SP500"]`

- [ ] **Test login với regular user không có group:**
```bash
# Tạo user test
curl -s -X POST http://localhost:8118/api/users \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"testpass12345","role":"user"}' | jq .

# Login với user đó
USER_TOKEN=$(curl -s -X POST http://localhost:8118/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"testpass12345"}' | jq -r '.token')

# accessible_markets phải là []
echo $USER_TOKEN | cut -d. -f2 | base64 -d 2>/dev/null | python3 -m json.tool | grep accessible_markets
```
Expected: `"accessible_markets": []`

- [ ] **Test market endpoint bị block:**
```bash
curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer $USER_TOKEN" \
  http://localhost:8118/api/gold/latest
```
Expected: `403`

- [ ] **Tạo market group, gán GOLD, gán user, re-login, test access:**
```bash
# Tạo group
GROUP_ID=$(curl -s -X POST http://localhost:8118/api/market-groups \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Gold Only","description":"GOLD access"}' | jq -r '.data.id')

# Gán GOLD vào group
curl -s -X PUT http://localhost:8118/api/market-groups/${GROUP_ID}/markets \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"market_keys":["GOLD"]}' | jq .

# Lấy user ID của testuser
USER_ID=$(curl -s http://localhost:8118/api/users \
  -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.username=="testuser") | .id')

# Gán user vào group
curl -s -X POST http://localhost:8118/api/market-groups/${GROUP_ID}/users \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":${USER_ID}}" | jq .

# Re-login để lấy JWT mới
NEW_TOKEN=$(curl -s -X POST http://localhost:8118/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"testpass12345"}' | jq -r '.token')

# accessible_markets phải có GOLD
echo $NEW_TOKEN | cut -d. -f2 | base64 -d 2>/dev/null | python3 -m json.tool | grep accessible_markets

# GOLD endpoint phải return 200
curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer $NEW_TOKEN" \
  http://localhost:8118/api/gold/latest
# Expected: 200

# NASDAQ endpoint phải return 403
curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer $NEW_TOKEN" \
  http://localhost:8118/api/nasdaq/latest
# Expected: 403
```

- [ ] **Test trigger với user thường (phải bị block):**
```bash
curl -s -o /dev/null -w "%{http_code}" \
  -X POST http://localhost:8118/api/trigger/gold-crawler \
  -H "Authorization: Bearer $NEW_TOKEN"
# Expected: 403
```

- [ ] **Cleanup test data:**
```bash
curl -s -X DELETE http://localhost:8118/api/users/${USER_ID} \
  -H "Authorization: Bearer $TOKEN" | jq .
```

- [ ] **Commit final:**
```bash
git add -A
git commit -m "feat(auth): integration tests pass — RBAC + market groups + JWT access control complete"
```
