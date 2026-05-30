import { Routes, Route, Navigate } from 'react-router-dom';
import { useState, useEffect } from 'react';
import { useData } from './context/DataContext';
import { Sidebar, Topbar, Ticker, MobNav, ErrorBoundary } from './components/ui';
import { TweaksPanel, TweakSection, TweakColor, TweakRadio, TweakSlider, TweakToggle, useTweaks } from './components/tweaks-panel';
import Dashboard from './pages/Dashboard';
import Stocks from './pages/Stocks';
import Predictions from './pages/Predictions';
import Training from './pages/Training';
import Gold from './pages/Gold';
import Guide from './pages/Guide';

const TWEAK_DEFAULTS = {
  accent: '#5B8DEF',
  density: 'regular' as const,
  fontScale: 100,
  ticker: true,
};

export default function App() {
  const { status } = useData();
  const [t, setTweak] = useTweaks(TWEAK_DEFAULTS);
  const [theme, setTheme] = useState<'dark' | 'light'>(
    () => (localStorage.getItem('vns_theme') as 'dark' | 'light') || 'dark'
  );

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('vns_theme', theme);
  }, [theme]);

  useEffect(() => {
    const r = document.documentElement;
    // Convert hex accent to CSS custom property
    // The CSS uses oklch-based accent, so we set it via data-accent for overriding
    r.style.setProperty('--accent-override', t.accent);
    r.setAttribute('data-density', t.density);
    r.style.fontSize = t.fontScale + '%';
  }, [t.accent, t.density, t.fontScale]);

  return (
    <div className="app">
      <Sidebar status={status} />
      <div className="main">
        {t.ticker && <Ticker />}
        <Topbar theme={theme} setTheme={setTheme} status={status} />
        <div className="content">
          <Routes>
            <Route path="/" element={<ErrorBoundary><Dashboard /></ErrorBoundary>} />
            <Route path="/dashboard" element={<Navigate to="/" replace />} />
            <Route path="/stocks" element={<ErrorBoundary><Stocks /></ErrorBoundary>} />
            <Route path="/predictions" element={<ErrorBoundary><Predictions /></ErrorBoundary>} />
            <Route path="/training" element={<ErrorBoundary><Training /></ErrorBoundary>} />
            <Route path="/gold" element={<ErrorBoundary><Gold /></ErrorBoundary>} />
            <Route path="/guide" element={<ErrorBoundary><Guide /></ErrorBoundary>} />
          </Routes>
        </div>
      </div>
      <TweaksPanel title="Tweaks">
        <TweakSection label="Giao diện" />
        <TweakColor label="Màu nhấn" value={t.accent}
          options={['#5B8DEF', '#2FB57C', '#C9A23F', '#8B7CF0', '#E0856B', '#4AA8C0']}
          onChange={(v) => setTweak('accent', v as string)} />
        <TweakRadio label="Mật độ" value={t.density}
          options={['compact', 'regular', 'comfy']}
          onChange={(v) => setTweak('density', v)} />
        <TweakSlider label="Cỡ chữ" value={t.fontScale} min={90} max={115} step={5} unit="%"
          onChange={(v) => setTweak('fontScale', v)} />
        <TweakToggle label="Thanh ticker" value={t.ticker}
          onChange={(v) => setTweak('ticker', v)} />
      </TweaksPanel>
      <MobNav />
    </div>
  );
}
