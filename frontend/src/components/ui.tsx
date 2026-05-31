import React, { useState } from 'react';
import { NavLink, useLocation, Link } from 'react-router-dom';
import { useData } from '../context/DataContext';
import { useAuth } from '../context/AuthContext';
import { Sparkline } from './charts';
import type { FmtUtils } from '../types';

// ── Icon definitions ─────────────────────────────────────────────────────────
const I: Record<string, React.ReactNode> = {
  grid:      <><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/></>,
  candles:   <><line x1="6" y1="3" x2="6" y2="21"/><rect x="3.5" y="8" width="5" height="8"/><line x1="16" y1="3" x2="16" y2="21"/><rect x="13.5" y="6" width="5" height="7"/></>,
  pulse:     <polyline points="2,13 7,13 10,5 14,19 17,11 22,11"/>,
  cpu:       <><rect x="6" y="6" width="12" height="12"/><line x1="9" y1="3" x2="9" y2="6"/><line x1="15" y1="3" x2="15" y2="6"/><line x1="9" y1="18" x2="9" y2="21"/><line x1="15" y1="18" x2="15" y2="21"/><line x1="3" y1="9" x2="6" y2="9"/><line x1="3" y1="15" x2="6" y2="15"/><line x1="18" y1="9" x2="21" y2="9"/><line x1="18" y1="15" x2="21" y2="15"/></>,
  gold:      <><ellipse cx="12" cy="6" rx="7" ry="2.5"/><path d="M5 6v6c0 1.4 3.1 2.5 7 2.5s7-1.1 7-2.5V6"/><path d="M5 12v6c0 1.4 3.1 2.5 7 2.5s7-1.1 7-2.5v-6"/></>,
  book:      <><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"/></>,
  settings:  <><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></>,
  search:    <><circle cx="11" cy="11" r="7"/><line x1="21" y1="21" x2="16.5" y2="16.5"/></>,
  sun:       <><circle cx="12" cy="12" r="4"/><line x1="12" y1="2" x2="12" y2="4"/><line x1="12" y1="20" x2="12" y2="22"/><line x1="4.2" y1="4.2" x2="5.6" y2="5.6"/><line x1="18.4" y1="18.4" x2="19.8" y2="19.8"/><line x1="2" y1="12" x2="4" y2="12"/><line x1="20" y1="12" x2="22" y2="12"/><line x1="4.2" y1="19.8" x2="5.6" y2="18.4"/><line x1="18.4" y1="5.6" x2="19.8" y2="4.2"/></>,
  moon:      <path d="M21 12.8A8 8 0 1 1 11.2 3a6.5 6.5 0 0 0 9.8 9.8z"/>,
  refresh:   <><path d="M21 12a9 9 0 1 1-2.6-6.4"/><polyline points="21 3 21 9 15 9"/></>,
  download:  <><path d="M12 3v12"/><polyline points="7 10 12 15 17 10"/><line x1="4" y1="20" x2="20" y2="20"/></>,
  play:      <polygon points="6 4 19 12 6 20"/>,
  bell:      <><path d="M18 8a6 6 0 1 0-12 0c0 7-3 8-3 8h18s-3-1-3-8"/><path d="M10.5 20a1.8 1.8 0 0 0 3 0"/></>,
  user:      <><circle cx="12" cy="8" r="4"/><path d="M4 20c0-4 3.6-7 8-7s8 3 8 7"/></>,
  caretUp:   <polyline points="6 14 12 8 18 14"/>,
  caretDown: <polyline points="6 10 12 16 18 10"/>,
  arrowUp:   <><line x1="12" y1="19" x2="12" y2="5"/><polyline points="6 11 12 5 18 11"/></>,
  arrowDown: <><line x1="12" y1="5" x2="12" y2="19"/><polyline points="6 13 12 19 18 13"/></>,
  filter:    <polygon points="3 4 21 4 14 12 14 19 10 21 10 12"/>,
  clock:     <><circle cx="12" cy="12" r="9"/><polyline points="12 7 12 12 15 14"/></>,
  layers:    <><polygon points="12 2 22 8.5 12 15 2 8.5"/><polyline points="2 15.5 12 22 22 15.5"/></>,
  crypto:    <><circle cx="12" cy="12" r="9"/><path d="M9 8h4.5a2.5 2.5 0 0 1 0 5H9"/><path d="M9 13h5a2.5 2.5 0 0 1 0 5H9"/><line x1="9" y1="8" x2="9" y2="18"/><line x1="11" y1="6" x2="11" y2="8"/><line x1="13" y1="18" x2="13" y2="20"/></>,
};

export function Icon({ name, size = 18, sw = 1.7, style, ...p }: { name: string; size?: number; sw?: number; style?: React.CSSProperties; [key: string]: any }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={sw} strokeLinecap="round" strokeLinejoin="round" style={style} {...p}>
      {I[name]}
    </svg>
  );
}

// ── Chg (change cell) ────────────────────────────────────────────────────────
export function Chg({ pct, abs, badge, fmt }: { pct: number; abs?: number; badge?: boolean; fmt?: FmtUtils }) {
  const f = fmt || defaultFmt;
  const dir = pct > 0.01 ? 'up' : pct < -0.01 ? 'down' : 'flat';
  if (badge) {
    return <span className={`badge badge--${dir === 'flat' ? 'muted' : dir}`}>{dir === 'up' ? '▲' : dir === 'down' ? '▼' : '–'} {f.pct(Math.abs(pct))}</span>;
  }
  return (
    <span className={`num ${dir}`}>
      {abs != null && <span style={{ marginRight: 8 }}>{f.sign(abs)}</span>}
      {f.pct(pct)}
    </span>
  );
}

const defaultFmt: FmtUtils = {
  vnd: (n) => n.toLocaleString('vi-VN'),
  price: (n) => n.toLocaleString('vi-VN', { minimumFractionDigits: 2, maximumFractionDigits: 2 }),
  pct: (n) => (n > 0 ? '+' : '') + n.toFixed(2) + '%',
  sign: (n) => (n > 0 ? '+' : '') + n.toFixed(2),
  compact: (n) => n >= 1e6 ? (n / 1e6).toFixed(1) + 'M' : n >= 1e3 ? (n / 1e3).toFixed(1) + 'K' : '' + n,
  goldShort: (n) => n >= 1e6 ? (n / 1e6).toFixed(2) + 'tr' : n.toLocaleString('vi-VN'),
};

// ── Panel ─────────────────────────────────────────────────────────────────────
export function Panel({
  title, sub, dot, tools, children, flush, className = '', style,
}: {
  title?: React.ReactNode;
  sub?: React.ReactNode;
  dot?: React.ReactNode;
  tools?: React.ReactNode;
  children?: React.ReactNode;
  flush?: boolean;
  className?: string;
  style?: React.CSSProperties;
}) {
  return (
    <div className={`panel ${className}`} style={style}>
      {(title || tools) && (
        <div className="panel__head">
          {title && <div className="panel__title">{title}{dot && <span className="dot"> · {dot}</span>}</div>}
          {sub && <span className="panel__sub">{sub}</span>}
          {tools && <div className="panel__tools">{tools}</div>}
        </div>
      )}
      <div className={`panel__body ${flush ? 'flush' : ''}`}>{children}</div>
    </div>
  );
}

// ── KPI ───────────────────────────────────────────────────────────────────────
export function KPI({
  label, value, sub, chgPct, chgAbs, spark, sparkColor, accent,
}: {
  label: string;
  value: React.ReactNode;
  sub?: React.ReactNode;
  chgPct?: number;
  chgAbs?: number;
  spark?: number[];
  sparkColor?: string;
  accent?: boolean;
}) {
  return (
    <div className="kpi">
      <div className="kpi__label">{label}</div>
      <div className="kpi__value" style={accent ? { color: 'var(--accent)' } : undefined}>{value}</div>
      <div className="kpi__meta">
        {chgPct != null ? <Chg pct={chgPct} abs={chgAbs} /> : <span style={{ color: 'var(--text-3)' }}>{sub}</span>}
        {chgPct != null && sub && <span style={{ color: 'var(--text-3)', fontSize: 11 }}>{sub}</span>}
      </div>
      {spark && <div className="kpi__spark"><Sparkline data={spark} w={120} h={40} color={sparkColor} /></div>}
    </div>
  );
}

// ── Seg ───────────────────────────────────────────────────────────────────────
export function Seg({ options, value, onChange }: { options: { value: string; label: string }[]; value: string; onChange: (v: string) => void }) {
  return (
    <div className="seg">
      {options.map((o) => (
        <button key={o.value} className={value === o.value ? 'active' : ''} onClick={() => onChange(o.value)}>{o.label}</button>
      ))}
    </div>
  );
}

// ── ConfBar ───────────────────────────────────────────────────────────────────
export function ConfBar({ v }: { v: number }) {
  const c = v >= 85 ? 'var(--up)' : v >= 75 ? 'var(--accent)' : 'var(--gold)';
  return (
    <div style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }}>
      <div className="bar" style={{ width: 46, height: 5 }}>
        <div className="bar__fill" style={{ width: v + '%', background: c }}></div>
      </div>
      <span className="num" style={{ fontSize: 12, color: 'var(--text-2)' }}>{v}%</span>
    </div>
  );
}

// ── Nav config ────────────────────────────────────────────────────────────────
interface NavItem { id: string; path: string; label: string; icon: string; }

// Market definitions with sub-pages
interface MarketDef {
  key: string;
  label: string;
  icon: string;
  color: string;
  comingSoon?: boolean;
}

const MARKETS: MarketDef[] = [
  { key: 'vn30',  label: 'VN30',  icon: 'candles', color: 'var(--accent)' },
  { key: 'gold',  label: 'Vàng',  icon: 'gold',    color: 'var(--gold)'   },
  { key: 'crypto',label: 'Crypto',icon: 'crypto',   color: 'var(--text-3)', comingSoon: true },
];

const MARKET_SUBS = [
  { path: '',             label: 'Tổng quan' },
  { path: '/predictions', label: 'Dự đoán'   },
  { path: '/training',    label: 'Huấn luyện'},
];

// Flat list for MobNav (top-level items only)
const NAV: NavItem[] = [
  { id: 'dashboard', path: '/',             label: 'Tổng quan', icon: 'grid'    },
  { id: 'vn30',      path: '/markets/vn30', label: 'VN30',      icon: 'candles' },
  { id: 'gold',      path: '/markets/gold', label: 'Vàng',      icon: 'gold'    },
  { id: 'guide',     path: '/guide',        label: 'Hướng dẫn', icon: 'book'    },
];

// ── Sidebar ───────────────────────────────────────────────────────────────────
export function Sidebar({ status }: { status: 'loading' | 'live' | 'demo' }) {
  const { pathname } = useLocation();
  const { user } = useAuth();

  return (
    <aside className="sidebar">
      <div className="brand">
        <div className="brand__mark"><span>V</span></div>
        <div>
          <div className="brand__name">VNStock</div>
          <div className="brand__sub">Terminal</div>
        </div>
      </div>
      <nav className="nav">
        {/* Dashboard */}
        <NavLink to="/" end className={({ isActive }) => `nav__item ${isActive ? 'active' : ''}`}>
          <Icon name="grid" size={17} /><span>Tổng quan</span>
        </NavLink>

        {/* Markets section */}
        <div className="nav__label">Thị trường</div>
        {MARKETS.map((m) => {
          const base = '/markets/' + m.key;
          const expanded = pathname.startsWith(base);
          return (
            <React.Fragment key={m.key}>
              {/* Market parent row */}
              <Link
                to={base}
                className={`nav__item nav__item--market ${expanded ? 'active' : ''}`}
                style={expanded ? { borderLeftColor: m.color, color: 'var(--text)' } : {}}
              >
                <Icon name={m.icon} size={17} style={expanded ? { color: m.color } : {}} />
                <span>{m.label}</span>
                {m.comingSoon && (
                  <span className="badge badge--muted" style={{ marginLeft: 'auto', fontSize: 9, padding: '1px 5px' }}>Soon</span>
                )}
                {!m.comingSoon && (
                  <Icon
                    name={expanded ? 'caretUp' : 'caretDown'}
                    size={13}
                    style={{ marginLeft: 'auto', color: 'var(--text-3)' }}
                  />
                )}
              </Link>

              {/* Sub-items — only render when expanded */}
              {expanded && !m.comingSoon && MARKET_SUBS.map((sub) => {
                const subPath = base + sub.path;
                const subIcon = sub.path === '' ? (m.key === 'gold' ? 'gold' : 'candles') : sub.path === '/predictions' ? 'pulse' : 'cpu';
                return (
                  <NavLink
                    key={sub.path}
                    to={subPath}
                    end={sub.path === ''}
                    className={({ isActive }) => `nav__sub-item ${isActive ? 'active' : ''}`}
                  >
                    <Icon name={subIcon} size={13} />
                    <span>{sub.label}</span>
                  </NavLink>
                );
              })}
            </React.Fragment>
          );
        })}

        {/* Support section */}
        <div className="nav__label">Hỗ trợ</div>
        <NavLink to="/guide" className={({ isActive }) => `nav__item ${isActive ? 'active' : ''}`}>
          <Icon name="book" size={17} /><span>Hướng dẫn</span>
        </NavLink>
        {user && (
          <NavLink to="/settings" className={({ isActive }) => `nav__item ${isActive ? 'active' : ''}`}>
            <Icon name="settings" size={17} /><span>Cài đặt</span>
          </NavLink>
        )}
      </nav>

      {user?.role === 'admin' && (
        <div className="sidebar__admin">
          <NavLink to="/admin/users" className={({ isActive }) => `nav__item ${isActive ? 'active' : ''}`}>
            <Icon name="user" size={17} /><span>Người dùng</span>
          </NavLink>
        </div>
      )}

      <div className="sidebar__foot">
        <div className="mkt">
          <div className="mkt__row">
            <span className={`mkt__dot ${status === 'live' ? 'live' : 'closed'}`}></span>
            <span style={{ color: 'var(--text-2)' }}>
              {status === 'live' ? 'Dữ liệu trực tiếp' : status === 'loading' ? 'Đang tải dữ liệu…' : 'Dữ liệu mẫu'}
            </span>
          </div>
          <div className="mkt__row" style={{ color: 'var(--text-3)', fontSize: 11, fontFamily: 'var(--font-mono)' }}>
            <Icon name="clock" size={13} /><span>Dữ liệu · phiên gần nhất</span>
          </div>
        </div>
      </div>
    </aside>
  );
}

// ── Ticker ────────────────────────────────────────────────────────────────────
export function Ticker() {
  const { data } = useData();
  const items = data.stocks;
  const fmt = data.fmt;
  if (!items.length) return null;
  const row = items.map((s, i) => (
    <span className="ticker__item" key={i}>
      <span className="ticker__sym">{s.sym}</span>
      <span className="ticker__px">{fmt.price(s.price)}</span>
      <span className={`ticker__chg ${s.chgPct > 0 ? 'up' : s.chgPct < 0 ? 'down' : 'flat'}`}>
        {s.chgPct > 0 ? '▲' : s.chgPct < 0 ? '▼' : '–'}{fmt.pct(Math.abs(s.chgPct)).replace('+', '')}
      </span>
    </span>
  ));
  return (
    <div className="ticker">
      <div className="ticker__track">{row}{row}</div>
    </div>
  );
}

// ── Topbar ────────────────────────────────────────────────────────────────────
const TITLES: Record<string, [string, string]> = {
  '/':                         ['Tổng quan thị trường',  'DASHBOARD'],
  '/markets/vn30':             ['VN30',                  'EQUITIES · VN30'],
  '/markets/vn30/predictions': ['Dự đoán VN30',          'PREDICTIONS · VN30'],
  '/markets/vn30/training':    ['Huấn luyện VN30',       'TRAINING · VN30'],
  '/markets/gold':             ['Vàng',                  'GOLD'],
  '/markets/gold/predictions': ['Dự đoán Vàng',          'PREDICTIONS · GOLD'],
  '/markets/gold/training':    ['Huấn luyện Vàng',       'TRAINING · GOLD'],
  '/markets/crypto':           ['Cryptocurrency',        'CRYPTO · Coming Soon'],
  '/guide':                    ['Hướng dẫn sử dụng',     'USER GUIDE'],
  '/settings':                 ['Cài đặt',               'SETTINGS'],
  '/admin/users':              ['Quản lý người dùng',    'ADMIN · USERS'],
  // legacy paths (still reachable until redirect fires)
  '/stocks':                   ['VN30',                  'EQUITIES · VN30'],
  '/predictions':              ['Dự đoán',               'FORECASTS'],
  '/training':                 ['Huấn luyện mô hình',    'ML TRAINING'],
  '/gold':                     ['Giá vàng',              'GOLD'],
  '/crypto':                   ['Cryptocurrency',        'CRYPTO · Coming Soon'],
};

export function Topbar({ theme, setTheme, user, onLoginClick, onLogout }: {
  theme: 'dark' | 'light';
  setTheme: (t: 'dark' | 'light') => void;
  status?: string;
  user?: { username: string } | null;
  onLoginClick?: () => void;
  onLogout?: () => void;
}) {
  const [q, setQ] = useState('');
  const { pathname } = useLocation();
  const [t1, t2] = TITLES[pathname] || ['', ''];
  return (
    <div className="topbar">
      <div className="topbar__title">{t1}</div>
      <div className="topbar__crumb">/ {t2}</div>
      <div className="topbar__spacer"></div>
      <div className="search">
        <Icon name="search" size={15} />
        <input placeholder="Tìm mã CK, ngành..." value={q} onChange={(e) => setQ(e.target.value)} />
        <kbd>/</kbd>
      </div>
      <button className="btn btn--icon btn--ghost" title="Thông báo"><Icon name="bell" size={16} /></button>
      <div className="theme-tog">
        <button className={theme === 'light' ? 'active' : ''} onClick={() => setTheme('light')} title="Sáng"><Icon name="sun" size={15} /></button>
        <button className={theme === 'dark' ? 'active' : ''} onClick={() => setTheme('dark')} title="Tối"><Icon name="moon" size={15} /></button>
      </div>
      {user ? (
        <div className="auth-user">
          <Icon name="user" size={14} />
          <span className="auth-user__name">{user.username}</span>
          <button className="btn btn--icon btn--ghost" title="Đăng xuất" onClick={onLogout}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
              <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/>
              <polyline points="16 17 21 12 16 7"/>
              <line x1="21" y1="12" x2="9" y2="12"/>
            </svg>
          </button>
        </div>
      ) : (
        <button className="btn btn--sm btn--ghost auth-login-btn" onClick={onLoginClick}>
          <Icon name="user" size={14} />
          Đăng nhập
        </button>
      )}
    </div>
  );
}

// ── MobNav ────────────────────────────────────────────────────────────────────
export function MobNav() {
  return (
    <nav className="mob-nav">
      {NAV.map((n) => (
        <NavLink key={n.id} to={n.path} end={n.path === '/'}
          className={({ isActive }) => `mob-nav__item ${isActive ? 'active' : ''}`}>
          <Icon name={n.icon} size={18} />
          <span>{n.label}</span>
        </NavLink>
      ))}
    </nav>
  );
}

// ── vnsToast ──────────────────────────────────────────────────────────────────
export function vnsToast(msg: string): void {
  let el = document.getElementById('vns-toast') as HTMLDivElement | null;
  if (!el) {
    el = document.createElement('div');
    el.id = 'vns-toast';
    el.style.cssText = 'position:fixed;right:18px;bottom:18px;z-index:200;background:var(--bg-2);border:1px solid var(--border-strong);color:var(--text);padding:10px 14px;font-size:13px;font-family:var(--font-ui);box-shadow:0 8px 24px rgba(0,0,0,.35);max-width:320px';
    document.body.appendChild(el);
  }
  el.textContent = msg;
  el.style.opacity = '1';
  el.style.transition = '';
  clearTimeout((el as any)._t);
  (el as any)._t = setTimeout(() => {
    el!.style.opacity = '0';
    el!.style.transition = 'opacity .4s';
  }, 3000);
}

// ── ErrorBoundary ─────────────────────────────────────────────────────────────
export class ErrorBoundary extends React.Component<{ children: React.ReactNode }, { err: Error | null }> {
  constructor(p: { children: React.ReactNode }) { super(p); this.state = { err: null }; }
  static getDerivedStateFromError(err: Error) { return { err }; }
  componentDidCatch(err: Error) { console.error('Page error:', err); }
  render() {
    if (this.state.err) {
      return (
        <div className="content__inner">
          <div className="empty" style={{ padding: '80px 20px' }}>
            <div className="empty__icon"><Icon name="layers" size={18} /></div>
            <p>Không thể hiển thị trang này với dữ liệu hiện tại.</p>
            <p style={{ fontSize: 11, color: 'var(--text-3)', marginTop: 8, fontFamily: 'var(--font-mono)' }}>
              {String(this.state.err?.message || this.state.err)}
            </p>
          </div>
        </div>
      );
    }
    return this.props.children;
  }
}
