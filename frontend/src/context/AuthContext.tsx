import { createContext, useContext, useState, useEffect, ReactNode } from 'react';

interface AuthUser { username: string; role: string; }
interface AuthContextValue {
  user: AuthUser | null;
  isLoggedIn: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue>({
  user: null,
  isLoggedIn: false,
  login: async () => {},
  logout: () => {},
});

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

  // Validate stored token on mount
  useEffect(() => {
    const token = localStorage.getItem('vns_token');
    if (!token) { setUser(null); return; }
    fetch('/api/auth/me', { headers: { Authorization: `Bearer ${token}` } })
      .then(r => r.ok ? r.json() : null)
      .then(data => {
        if (data?.username) {
          setUser({ username: data.username, role: data.role || 'user' });
        } else {
          localStorage.removeItem('vns_token');
          localStorage.removeItem('vns_user');
          setUser(null);
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
  };

  const logout = () => {
    localStorage.removeItem('vns_token');
    localStorage.removeItem('vns_user');
    setUser(null);
  };

  return (
    <AuthContext.Provider value={{ user, isLoggedIn: !!user, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() { return useContext(AuthContext); }
