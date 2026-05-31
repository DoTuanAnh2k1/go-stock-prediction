import React, { useState } from 'react';
import { NavLink, useLocation } from 'react-router-dom';
import { useData } from '../context/DataContext';
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
  search:    <><circle cx="11" cy="11" r="7"/><line x1="21" y1="21" x2="16.5" y2="16.5"/></>,
  sun:       <><circle cx="12" cy="12" r="4"/><line x1="12" y1="2" x2="12" y2="4"/><line x1="12" y1="20" x2="12" y2="22"/><line x1="4.2" y1="4.2" x2="5.6" y2="5.6"/><line x1="18.4" y1="18.4" x2="19.8" y2="19.8"/><line x1="2" y1="12" x2="4" y2="12"/><line x1="20" y1="12" x2="22" y2="12"/><line x1="4.2" y1="19.8" x2="5.6" y2="18.4"/><line x1="18.4" y1="5.6" x2="19.8" y2="4.2"/></>,
  moon:      <path d="M21 12.8A8 8 0 1 1 11.2 3a6.5 6.5 0 0 0 9.8 9.8z"/>,
  refresh:   <><path d="M21 12a9 9 0 1 1-2.6-6.4"/><polyline points="21 3 21 9 15 9"/></>,
  download:  <><path d="M12 3v12"/><polyline points="7 10 12 15 17 10"/><line x1="4" y1="20" x2="20" y2="20"/></>,
  play:      <polygon points="6 4 19 12 6 20"/>,
  bell:      <><path d="M18 8a6 6 0 1 0-12 0c0 7-3 8-3 8h18s-3-1-3-8"/><path d="M10.5 20a1.8 1.8 0 0 0 3 0"/></>,
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
const NAV = [
  { id: 'dashboard',   path: '/',            label: 'Tổng quan',   icon: 'grid' },
  { id: 'stocks',      path: '/stocks',       label: 'Cổ phiếu',   icon: 'candles' },
  { id: 'predictions', path: '/predictions',  label: 'Dự đoán',    icon: 'pulse' },
  { id: 'training',    path: '/training',     label: 'Huấn luyện', icon: 'cpu' },
  { id: 'gold',        path: '/gold',         label: 'Giá vàng',   icon: 'gold' },
  { id: 'crypto',      path: '/crypto',       label: 'Crypto',      icon: 'crypto' },
  { id: 'guide',       path: '/guide',        label: 'Hướng dẫn',  icon: 'book' },
];

// ── Sidebar ───────────────────────────────────────────────────────────────────
export function Sidebar({ status }: { status: 'loading' | 'live' | 'demo' }) {
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
        <div className="nav__label">Thị trường</div>
        {NAV.slice(0, 3).map((n) => (
          <NavLink key={n.id} to={n.path} end={n.path === '/'}
            className={({ isActive }) => `nav__item ${isActive ? 'active' : ''}`}>
            <Icon name={n.icon} size={17} /><span>{n.label}</span>
          </NavLink>
        ))}
        <div className="nav__label">Mô hình &amp; Tài sản</div>
        {NAV.slice(3, 6).map((n) => (
          <NavLink key={n.id} to={n.path}
            className={({ isActive }) => `nav__item ${isActive ? 'active' : ''}`}>
            <Icon name={n.icon} size={17} /><span>{n.label}</span>
          </NavLink>
        ))}
        <div className="nav__label">Hỗ trợ</div>
        {NAV.slice(6).map((n) => (
          <NavLink key={n.id} to={n.path}
            className={({ isActive }) => `nav__item ${isActive ? 'active' : ''}`}>
            <Icon name={n.icon} size={17} /><span>{n.label}</span>
          </NavLink>
        ))}
      </nav>
      <div className="sidebar__foot">
        <div className="mkt">
          <div className="mkt__row">
            <span className={`mkt__dot ${status === 'live' ? 'live' : 'closed'}`}></span>
            <span style={{ color: 'var(--text-2)' }}>
              {status === 'live' ? 'Dữ liệu trực tiếp' : status === 'loading' ? 'Đang tải dữ liệu…' : 'Dữ liệu mẫu'}
            </span>
          </div>
          <div className="mkt__row" style={{ color: 'var(--text-3)', fontSize: 11, fontFamily: 'var(--font-mono)' }}>
            <Icon name="clock" size={13} /><span>VN30 · phiên gần nhất</span>
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
  '/':           ['Tổng quan thị trường', 'DASHBOARD'],
  '/stocks':     ['Cổ phiếu', 'EQUITIES · VN30'],
  '/predictions':['Dự đoán', 'FORECASTS'],
  '/training':   ['Huấn luyện mô hình', 'ML TRAINING'],
  '/gold':       ['Giá vàng', 'GOLD'],
  '/crypto':     ['Cryptocurrency', 'CRYPTO · Coming Soon'],
  '/guide':      ['Hướng dẫn sử dụng', 'USER GUIDE'],
};

export function Topbar({ theme, setTheme }: { theme: 'dark' | 'light'; setTheme: (t: 'dark' | 'light') => void; status?: string }) {
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
