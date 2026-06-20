import { Routes, Route, Navigate } from 'react-router-dom';
import { useState, useEffect } from 'react';
import { useData } from './context/DataContext';
import { AuthProvider, useAuth } from './context/AuthContext';
import { LangProvider, useLanguage } from './context/LangContext';
import { Sidebar, Topbar, Ticker, MobNav, ErrorBoundary } from './components/ui';
import { TweaksPanel, TweakSection, TweakColor, TweakRadio, TweakSlider, TweakToggle, useTweaks } from './components/tweaks-panel';
import { LoginModal } from './components/LoginModal';
import Dashboard from './pages/Dashboard';
import Predictions from './pages/Predictions';
import Training from './pages/Training';
import Gold from './pages/Gold';
import Crypto from './pages/Crypto';
import Nasdaq from './pages/Nasdaq';
import SP500 from './pages/SP500';
import Guide from './pages/Guide';
import MarketPredictions from './pages/MarketPredictions';
import MarketTraining from './pages/MarketTraining';
import SessionStats from './pages/SessionStats';
import Users from './pages/Users';
import MarketGroups from './pages/MarketGroups';
import Commands from './pages/Commands';
import CommandGroups from './pages/CommandGroups';
import Settings from './pages/Settings';
import Simulation from './pages/Simulation';
import SimulationBot from './pages/SimulationBot';
import Monitoring from './pages/Monitoring';

const TWEAK_DEFAULTS = {
  accent: '#5B8DEF',
  density: 'regular' as const,
  fontScale: 100,
  ticker: true,
};

function AppInner() {
  const { status } = useData();
  const { user, logout, isLoading } = useAuth();
  const { t: tr } = useLanguage();
  const [t, setTweak] = useTweaks(TWEAK_DEFAULTS);
  const [theme, setTheme] = useState<'dark' | 'light'>(
    () => (localStorage.getItem('vns_theme') as 'dark' | 'light') || 'dark'
  );
  const [showLogin, setShowLogin] = useState(false);

  // Auto-show login modal when auth check completes and user is not logged in
  useEffect(() => {
    if (!isLoading && !user) setShowLogin(true);
  }, [isLoading, user]);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(
    () => localStorage.getItem('vns_sidebar') === '1'
  );

  const toggleSidebar = () => {
    setSidebarCollapsed(c => {
      const next = !c;
      localStorage.setItem('vns_sidebar', next ? '1' : '0');
      return next;
    });
  };

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('vns_theme', theme);
  }, [theme]);

  useEffect(() => {
    const r = document.documentElement;
    r.style.setProperty('--accent-override', t.accent);
    r.setAttribute('data-density', t.density);
    r.style.fontSize = t.fontScale + '%';
  }, [t.accent, t.density, t.fontScale]);

  return (
    <div className={`app${sidebarCollapsed ? ' app--sidebar-collapsed' : ''}`}>
      <Sidebar status={status} collapsed={sidebarCollapsed} onToggle={toggleSidebar} />
      <div className="main">
        {t.ticker && <Ticker />}
        <Topbar
          theme={theme}
          setTheme={setTheme}
          status={status}
          user={user}
          onLoginClick={() => setShowLogin(true)}
          onLogout={logout}
        />
        <div className="content">
          <Routes>
            {/* ── Main routes ─────────────────────────────────── */}
            <Route path="/" element={<ErrorBoundary><Dashboard /></ErrorBoundary>} />
            <Route path="/dashboard" element={<Navigate to="/" replace />} />

            {/* ── Market routes ────────────────────────────────── */}
            <Route path="/markets/gold" element={<ErrorBoundary><Gold /></ErrorBoundary>} />

            <Route path="/markets/:marketKey/predictions" element={<ErrorBoundary><MarketPredictions /></ErrorBoundary>} />
            <Route path="/markets/:marketKey/training" element={<ErrorBoundary><MarketTraining /></ErrorBoundary>} />
            <Route path="/markets/:marketKey/session" element={<ErrorBoundary><SessionStats /></ErrorBoundary>} />

            <Route path="/markets/crypto" element={<ErrorBoundary><Crypto /></ErrorBoundary>} />
            <Route path="/markets/nasdaq100" element={<ErrorBoundary><Nasdaq /></ErrorBoundary>} />
            <Route path="/markets/sp500" element={<ErrorBoundary><SP500 /></ErrorBoundary>} />

            {/* ── Simulation ───────────────────────────────────── */}
            <Route path="/simulation" element={<ErrorBoundary><Simulation /></ErrorBoundary>} />
            <Route path="/simulation/:botId" element={<ErrorBoundary><SimulationBot /></ErrorBoundary>} />

            {/* ── Support ──────────────────────────────────────── */}
            <Route path="/guide" element={<ErrorBoundary><Guide /></ErrorBoundary>} />
            <Route path="/settings" element={<ErrorBoundary><Settings /></ErrorBoundary>} />
            <Route path="/monitoring" element={<ErrorBoundary><Monitoring /></ErrorBoundary>} />

            {/* ── Admin ────────────────────────────────────────── */}
            <Route path="/admin/users" element={<ErrorBoundary><Users /></ErrorBoundary>} />
            <Route path="/admin/market-groups" element={<ErrorBoundary><MarketGroups /></ErrorBoundary>} />
            <Route path="/admin/commands" element={<ErrorBoundary><Commands /></ErrorBoundary>} />
            <Route path="/admin/command-groups" element={<ErrorBoundary><CommandGroups /></ErrorBoundary>} />

            {/* ── Legacy redirects (keep bookmarks working) ────── */}
            <Route path="/gold" element={<Navigate to="/markets/gold" replace />} />
            <Route path="/crypto" element={<Navigate to="/markets/crypto" replace />} />
            <Route path="/nasdaq" element={<Navigate to="/markets/nasdaq100" replace />} />
          </Routes>
        </div>
      </div>
      <TweaksPanel title="Tweaks">
        <TweakSection label={tr.tweaks.interface} />
        <TweakColor label={tr.tweaks.accentColor} value={t.accent}
          options={['#5B8DEF', '#2FB57C', '#C9A23F', '#8B7CF0', '#E0856B', '#4AA8C0']}
          onChange={(v) => setTweak('accent', v as string)} />
        <TweakRadio label={tr.tweaks.density} value={t.density}
          options={['compact', 'regular', 'comfy']}
          onChange={(v) => setTweak('density', v)} />
        <TweakSlider label={tr.tweaks.fontSize} value={t.fontScale} min={90} max={115} step={5} unit="%"
          onChange={(v) => setTweak('fontScale', v)} />
        <TweakToggle label={tr.tweaks.ticker} value={t.ticker}
          onChange={(v) => setTweak('ticker', v)} />
      </TweaksPanel>
      <MobNav />
      {showLogin && <LoginModal onClose={() => setShowLogin(false)} required={!user} />}
    </div>
  );
}

export default function App() {
  return (
    <AuthProvider>
      <LangProvider>
        <AppInner />
      </LangProvider>
    </AuthProvider>
  );
}
