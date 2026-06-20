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
  isLoading: boolean;
  role: string;
  accessibleMarkets: string[];
  canAccessMarket: (marketKey: string) => boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue>({
  user: null,
  isLoggedIn: false,
  isLoading: true,
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

  const [isLoading, setIsLoading] = useState(true);

  // Validate stored token on mount
  useEffect(() => {
    const token = localStorage.getItem('vns_token');
    if (!token) { setUser(null); setAccessibleMarkets([]); setIsLoading(false); return; }
    fetch('/api/auth/me', { headers: { Authorization: `Bearer ${token}` } })
      .then(r => r.ok ? r.json() : null)
      .then(data => {
        // Handle both bare response {username, role} and wrapped {data: {username, role}}
        const u = data?.data ?? data;
        if (u?.username) {
          setUser({ username: u.username, role: u.role || 'user' });
          const payload = parseJwt(token);
          setAccessibleMarkets(payload?.accessible_markets ?? []);
        } else {
          localStorage.removeItem('vns_token');
          localStorage.removeItem('vns_user');
          setUser(null);
          setAccessibleMarkets([]);
        }
      })
      .catch(() => {})
      .finally(() => setIsLoading(false));
  }, []);

  const login = async (username: string, password: string) => {
    const res = await fetch('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      throw new Error((err as any).message || (err as any).data?.message || 'Đăng nhập thất bại');
    }
    const data = await res.json();
    // Handle both bare response and wrapped {data: {token, user}}
    const respData = data.data ?? data;
    const token = respData.token;
    const userObj: AuthUser = {
      username: respData.user?.username || username,
      role: respData.user?.role || 'user',
    };
    localStorage.setItem('vns_token', token);
    localStorage.setItem('vns_user', JSON.stringify(userObj));
    setUser(userObj);
    const payload = parseJwt(token);
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
    if (role === 'admin') return true;
    return accessibleMarkets.includes(marketKey);
  };

  return (
    <AuthContext.Provider value={{
      user, isLoggedIn: !!user, isLoading, role, accessibleMarkets, canAccessMarket, login, logout,
    }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() { return useContext(AuthContext); }
