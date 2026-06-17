import React, { useState, useEffect } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useData } from '../context/DataContext';
import { Panel, KPI, Icon, Chg } from '../components/ui';
import { HBars } from '../components/charts';
import { useLanguage } from '../context/LangContext';
import { useAuth } from '../context/AuthContext';
import { fetchMonitoringOverview, type MonitoringOverview, type MonitoringMarket } from '../api';

// ── Helpers ───────────────────────────────────────────────────────────────────

function fmtDT(s: string | null): string {
  if (!s) return '—';
  try {
    const d = new Date(s);
    if (isNaN(d.getTime())) return s.slice(0, 16).replace('T', ' ');
    const dd = ('0' + d.getDate()).slice(-2);
    const mm = ('0' + (d.getMonth() + 1)).slice(-2);
    const hh = ('0' + d.getHours()).slice(-2);
    const mi = ('0' + d.getMinutes()).slice(-2);
    return `${dd}/${mm} ${hh}:${mi}`;
  } catch {
    return s.slice(0, 16).replace('T', ' ');
  }
}

function algoLabel(algo: string): string {
  const map: Record<string, string> = {
    lstm_nn: 'LSTM', arima_garch: 'ARIMA', moving_average: 'MA',
    ema: 'EMA', ema_macd: 'EMA', ensemble: 'ENS', lightgbm: 'LGB',
    random_forest: 'RF', xgboost: 'XGB', sarima: 'SARIMA', gru_nn: 'GRU',
  };
  return map[(algo || '').toLowerCase()] || algo.slice(0, 5).toUpperCase();
}

const MARKET_COLORS: Record<string, string> = {
  GOLD: 'var(--gold)',
  NASDAQ: 'oklch(0.74 0.13 200)',
  SP500: 'var(--up)',
  CRYPTO: 'oklch(0.72 0.16 280)',
};

const MARKET_ICONS: Record<string, string> = {
  GOLD: 'gold',
  NASDAQ: 'candles',
  SP500: 'activity',
  CRYPTO: 'layers',
};

const MARKET_LINKS: Record<string, string> = {
  GOLD: '/markets/gold',
  NASDAQ: '/markets/nasdaq100',
  SP500: '/markets/sp500',
  CRYPTO: '/markets/crypto',
};

// ── Market Status Card ─────────────────────────────────────────────────────────

function MarketStatusCard({ market }: { market: MonitoringMarket }) {
  const { t } = useLanguage();
  const d = t.dashboard;
  const color = MARKET_COLORS[market.market] || 'var(--accent)';
  const icon = MARKET_ICONS[market.market] || 'candles';
  const { crawl, predictions } = market;
  const link = MARKET_LINKS[market.market] || '/';

  const missingCount = predictions.missing_today?.length ?? 0;
  const allAlgosPresent = missingCount === 0 && predictions.today_total > 0;
  const coveragePct = predictions.expected_algos > 0
    ? Math.round(predictions.today_total / predictions.expected_algos * 100)
    : 0;

  return (
    <Link to={link} style={{ textDecoration: 'none', color: 'inherit', display: 'block' }}>
      <div
        style={{
          padding: '18px 20px',
          background: 'var(--surface)',
          border: '1px solid var(--border)',
          borderTop: `3px solid ${color}`,
          height: '100%',
          boxSizing: 'border-box',
          transition: 'background .15s',
        }}
        onMouseEnter={e => (e.currentTarget.style.background = 'var(--surface-2)')}
        onMouseLeave={e => (e.currentTarget.style.background = 'var(--surface)')}
      >
        {/* Header row */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 14 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <Icon name={icon} size={16} style={{ color }} />
            <span style={{ fontWeight: 700, fontSize: 15, color }}>{market.market}</span>
          </div>
          <div style={{ display: 'flex', gap: 5, flexWrap: 'wrap', justifyContent: 'flex-end' }}>
            {!crawl.market_open && (
              <span style={{
                fontSize: 10, padding: '2px 7px',
                background: 'var(--surface-2)', color: 'var(--text-3)',
                fontFamily: 'var(--font-mono)', letterSpacing: '0.5px',
              }}>
                {d.marketClosed}
              </span>
            )}
            <span style={{
              fontSize: 10, padding: '2px 7px',
              background: crawl.stale ? 'var(--down-bg)' : 'var(--up-bg)',
              color: crawl.stale ? 'var(--down)' : 'var(--up)',
              fontFamily: 'var(--font-mono)', letterSpacing: '0.5px',
            }}>
              {crawl.stale ? d.stale : d.fresh}
            </span>
          </div>
        </div>

        {/* Prediction count — big number */}
        <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginBottom: 10 }}>
          <span style={{
            fontFamily: 'var(--font-mono)',
            fontSize: 28,
            fontWeight: 700,
            letterSpacing: '-1px',
            color: predictions.today_total === 0
              ? 'var(--text-3)'
              : allAlgosPresent
                ? color
                : 'var(--gold)',
          }}>
            {predictions.today_total}
          </span>
          <span style={{ fontSize: 12, color: 'var(--text-3)' }}>
            / {predictions.expected_algos} {d.predsCount}
          </span>
          {allAlgosPresent && (
            <span style={{ color: 'var(--up)', fontSize: 14, marginLeft: 2 }}>✓</span>
          )}
          {missingCount > 0 && (
            <span style={{
              fontSize: 10, padding: '2px 7px',
              background: 'var(--down-bg)', color: 'var(--down)',
              fontFamily: 'var(--font-mono)', marginLeft: 4,
            }}>
              -{missingCount}
            </span>
          )}
        </div>

        {/* Coverage bar */}
        <div style={{ height: 3, background: 'var(--surface-2)', marginBottom: 10, borderRadius: 2 }}>
          <div style={{
            height: '100%',
            width: `${Math.min(100, coveragePct)}%`,
            background: allAlgosPresent ? 'var(--up)' : crawl.stale ? 'var(--down)' : color,
            borderRadius: 2,
            transition: 'width .3s',
          }} />
        </div>

        {/* Crawl info */}
        <div style={{ fontSize: 11, color: 'var(--text-3)', display: 'flex', alignItems: 'center', gap: 6 }}>
          <span>{d.lastCrawl}:</span>
          <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--text-2)' }}>
            {crawl.last_crawl_at ? fmtDT(crawl.last_crawl_at) : d.never}
          </span>
          {crawl.staleness && (
            <span style={{ color: crawl.stale ? 'var(--down)' : 'var(--text-3)' }}>
              ({crawl.staleness})
            </span>
          )}
        </div>
        {(crawl.daily_today > 0 || crawl.intraday_today > 0) && (
          <div style={{ fontSize: 11, color: 'var(--text-3)', marginTop: 4, fontFamily: 'var(--font-mono)' }}>
            {crawl.daily_today}d · {crawl.intraday_today}i
          </div>
        )}
      </div>
    </Link>
  );
}

// ── Pipeline Summary Table ─────────────────────────────────────────────────────

function SummaryRow({ market: m }: { market: MonitoringMarket }) {
  const navigate = useNavigate();
  const color = MARKET_COLORS[m.market] || 'var(--accent)';
  const icon = MARKET_ICONS[m.market] || 'candles';
  const link = MARKET_LINKS[m.market] || '/';
  const { crawl, predictions } = m;

  const top3 = [...(predictions.algorithms || [])]
    .filter(a => a.reconciled > 0)
    .sort((a, b) => b.direction_accuracy - a.direction_accuracy)
    .slice(0, 3);

  return (
    <tr
      onClick={() => navigate(link)}
      style={{ cursor: 'pointer' }}
      onMouseEnter={e => { (e.currentTarget as HTMLTableRowElement).style.background = 'var(--surface-2)'; }}
      onMouseLeave={e => { (e.currentTarget as HTMLTableRowElement).style.background = ''; }}
    >
      <td style={{ padding: '7px 8px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <Icon name={icon} size={13} style={{ color }} />
          <span style={{ fontWeight: 700, color, fontFamily: 'var(--font-mono)', fontSize: 11, letterSpacing: '0.5px' }}>
            {m.market}
          </span>
        </div>
      </td>
      <td style={{ padding: '7px 8px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 5 }}>
          <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--text-2)', fontSize: 11 }}>
            {crawl.staleness || '—'}
          </span>
          <span style={{
            fontSize: 9, padding: '1px 5px',
            background: crawl.stale ? 'var(--down-bg)' : 'var(--up-bg)',
            color: crawl.stale ? 'var(--down)' : 'var(--up)',
            fontFamily: 'var(--font-mono)',
          }}>
            {crawl.stale ? 'STALE' : 'OK'}
          </span>
        </div>
      </td>
      <td style={{ padding: '7px 8px' }}>
        <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--text-3)', fontSize: 11 }}>
          {crawl.daily_today}d / {crawl.intraday_today}i
        </span>
      </td>
      <td style={{ padding: '7px 8px', textAlign: 'right' }}>
        <span style={{ fontFamily: 'var(--font-mono)', fontWeight: 600, fontSize: 12 }}>
          {predictions.today_total.toLocaleString()}
        </span>
      </td>
      <td className="pipeline-hide-mobile" style={{ padding: '7px 8px' }}>
        <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
          {top3.length === 0 ? (
            <span style={{ color: 'var(--text-3)', fontSize: 11 }}>—</span>
          ) : top3.map(a => {
            const acc = a.direction_accuracy * 100;
            const bg = acc >= 55
              ? 'var(--up-bg)'
              : acc >= 45
                ? 'color-mix(in oklch, var(--gold) 15%, transparent)'
                : 'var(--down-bg)';
            const fg = acc >= 55 ? 'var(--up)' : acc >= 45 ? 'var(--gold)' : 'var(--down)';
            return (
              <span key={a.algorithm} style={{
                fontSize: 10, padding: '2px 6px',
                background: bg, color: fg,
                fontFamily: 'var(--font-mono)',
                whiteSpace: 'nowrap',
              }}>
                {algoLabel(a.algorithm)} {acc.toFixed(0)}%
              </span>
            );
          })}
        </div>
      </td>
    </tr>
  );
}

function PipelineSummaryTable({ markets }: { markets: MonitoringMarket[] }) {
  if (markets.length === 0) return null;
  const headerCell: React.CSSProperties = {
    textAlign: 'left', padding: '3px 8px', fontSize: 10,
    fontWeight: 500, color: 'var(--text-3)',
    textTransform: 'uppercase', letterSpacing: '0.5px',
    borderBottom: '1px solid var(--border)',
  };
  return (
    <div style={{ marginTop: 16, borderTop: '1px solid var(--border)', paddingTop: 12 }}>
      <table style={{ width: '100%', borderCollapse: 'collapse' }}>
        <thead>
          <tr>
            <th style={headerCell}>Market</th>
            <th style={headerCell}>Crawl</th>
            <th style={headerCell}>Data</th>
            <th style={{ ...headerCell, textAlign: 'right' }}>Preds</th>
            <th className="pipeline-hide-mobile" style={headerCell}>Top Algo</th>
          </tr>
        </thead>
        <tbody>
          {markets.map(m => <SummaryRow key={m.market} market={m} />)}
        </tbody>
      </table>
    </div>
  );
}

// ── Main Dashboard ─────────────────────────────────────────────────────────────

export default function Dashboard() {
  const { data: D } = useData();
  const { t } = useLanguage();
  const { user, canAccessMarket } = useAuth();
  const d = t.dashboard;

  const [monitoring, setMonitoring] = useState<MonitoringOverview | null>(null);
  const [monLoading, setMonLoading] = useState(false);
  const [selectedDirMarket, setSelectedDirMarket] = useState<string | null>(null);

  useEffect(() => {
    if (!user) { setMonitoring(null); return; }
    setMonLoading(true);
    fetchMonitoringOverview()
      .then(setMonitoring)
      .catch(() => {})
      .finally(() => setMonLoading(false));
  }, [user]);

  const accessibleMarkets = monitoring
    ? monitoring.markets.filter(m => canAccessMarket(m.market))
    : [];

  const effectiveDirMarket = selectedDirMarket ?? (accessibleMarkets[0]?.market ?? null);

  // Direction accuracy per algo for the selected market
  const algoDirectionAcc = (() => {
    if (!monitoring || !effectiveDirMarket) return {} as Record<string, number>;
    const mkt = monitoring.markets.find(m => m.market === effectiveDirMarket);
    if (!mkt) return {} as Record<string, number>;
    const result: Record<string, number> = {};
    for (const row of (mkt.predictions.algorithms || [])) {
      if (row.reconciled > 0) {
        result[row.algorithm] = row.direction_accuracy * 100;
      }
    }
    return result;
  })();

  const ensDir = algoDirectionAcc['ensemble'] ?? null;
  const ensAccDisplay = ensDir != null ? ensDir.toFixed(1) + '%' : '—%';
  const ens = (D.algos || []).find((a) => a.cls === 'ens');

  const predsToday = monitoring
    ? accessibleMarkets.reduce((s, m) => s + m.predictions.today_total, 0)
    : null;
  const activeBots = monitoring?.bots.summary.active_bots ?? null;
  const freshCount = monitoring
    ? accessibleMarkets.filter(m => !m.crawl.stale || !m.crawl.market_open).length
    : null;
  const totalMarkets = accessibleMarkets.length || 4;

  return (
    <div className="content__inner fade">
      {/* KPI row */}
      <div className="grid grid--kpis section-gap">
        <KPI
          label={d.ensDir}
          value={ensAccDisplay}
          sub={d.ensDirSub}
          accent
        />
        <KPI
          label={d.totalPredictions}
          value={D.stats?.total ? D.stats.total.toLocaleString() : '—'}
          sub={d.allTime}
        />
        <KPI
          label={d.predsToday}
          value={predsToday != null ? String(predsToday) : user ? '…' : '—'}
          sub={d.acrossMarkets}
        />
        <KPI
          label={d.activeBots}
          value={activeBots != null ? String(activeBots) : user ? '…' : '—'}
          sub={d.inSimulation}
        />
      </div>

      {/* Market Pipeline Status */}
      <Panel
        title={d.marketStatus}
        sub={d.marketStatusDesc}
        className="section-gap"
        tools={
          user ? (
            <Link to="/monitoring" style={{ fontSize: 12, color: 'var(--accent)', textDecoration: 'none' }}>
              {d.viewAll}
            </Link>
          ) : undefined
        }
      >
        {!user ? (
          <div className="empty" style={{ padding: '24px 0' }}>
            <div className="empty__icon"><Icon name="activity" size={16} /></div>
            <p style={{ fontSize: 12 }}>{d.loginForStatus}</p>
          </div>
        ) : monLoading && !monitoring ? (
          <div className="empty" style={{ padding: '24px 0' }}>
            <div className="empty__icon"><Icon name="refresh" size={16} /></div>
            <p style={{ fontSize: 12 }}>{d.loadingStatus}</p>
          </div>
        ) : monitoring ? (
          <>
            <div style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(4, 1fr)',
              gap: 12,
            }}>
              {accessibleMarkets.map((m) => (
                <MarketStatusCard key={m.market} market={m} />
              ))}
            </div>
            <PipelineSummaryTable markets={accessibleMarkets} />
            {/* Footer summary */}
            <div style={{ marginTop: 10, display: 'flex', gap: 20, fontSize: 11, color: 'var(--text-3)', flexWrap: 'wrap', alignItems: 'center' }}>
              <span>
                {freshCount}/{totalMarkets} markets {d.fresh.toLowerCase()}
              </span>
              {monitoring.bots.summary.total_bots > 0 && (
                <span>
                  {monitoring.bots.summary.active_bots}/{monitoring.bots.summary.total_bots} {d.activeBots.toLowerCase()}
                </span>
              )}
              {monitoring.generated_at && (
                <span style={{ marginLeft: 'auto', fontFamily: 'var(--font-mono)' }}>
                  {fmtDT(monitoring.generated_at)}
                </span>
              )}
            </div>
          </>
        ) : null}
      </Panel>

      <TopSimulationBots />

      {/* Algorithm Direction Accuracy */}
      <div className="section-gap">
        {user && accessibleMarkets.length > 0 && (
          <div style={{ display: 'flex', gap: 6, marginBottom: 12, flexWrap: 'wrap' }}>
            {accessibleMarkets.map(m => {
              const isActive = m.market === effectiveDirMarket;
              const color = MARKET_COLORS[m.market] || 'var(--accent)';
              return (
                <button
                  key={m.market}
                  onClick={() => setSelectedDirMarket(m.market)}
                  style={{
                    padding: '4px 14px',
                    fontSize: 11,
                    fontWeight: isActive ? 700 : 400,
                    background: isActive ? color : 'var(--surface-2)',
                    color: isActive ? '#fff' : 'var(--text-2)',
                    border: isActive ? `1px solid ${color}` : '1px solid var(--border)',
                    cursor: 'pointer',
                    transition: 'all .15s',
                    fontFamily: 'var(--font-mono)',
                    letterSpacing: '0.5px',
                  }}
                >
                  {m.market}
                </button>
              );
            })}
          </div>
        )}
        <div className="grid grid--halves">
        <Panel
          title={d.dirAccuracy}
          sub={effectiveDirMarket ? `${effectiveDirMarket} · ${d.dirAccuracySub.split('·').slice(-1)[0].trim()}` : d.dirAccuracySub}
          flush
        >
          {D.algos.length === 0 ? (
            <div className="empty">
              <div className="empty__icon"><Icon name="layers" size={18} /></div>
              <p>{d.noTrainingData}</p>
            </div>
          ) : !user ? (
            <div className="empty" style={{ padding: '24px 0' }}>
              <div className="empty__icon"><Icon name="cpu" size={16} /></div>
              <p style={{ fontSize: 12 }}>{d.loginToSeeAccuracy}</p>
            </div>
          ) : D.algos.map((a) => {
            const dirAcc = algoDirectionAcc[a.id];
            const hasData = dirAcc != null && dirAcc > 0;
            const accColor = !hasData ? 'var(--text-3)'
              : dirAcc >= 55 ? 'var(--up)'
              : dirAcc >= 45 ? 'var(--gold)'
              : 'var(--down)';
            return (
              <div className="lrow" key={a.id}>
                <span className={`algo algo--${a.cls}`} style={{ minWidth: 52, textAlign: 'center' }}>{a.short}</span>
                <div className="lrow__main">
                  <div className="lrow__sym" style={{ fontSize: 12.5 }}>{a.name}</div>
                  <div className="bar" style={{ marginTop: 6, width: 130 }}>
                    {hasData && (
                      <div className="bar__fill" style={{
                        width: dirAcc + '%',
                        background: accColor,
                      }} />
                    )}
                  </div>
                </div>
                <div className="lrow__rt">
                  <div className="num" style={{ fontSize: 14, fontWeight: 600, color: accColor }}>
                    {hasData ? dirAcc.toFixed(1) + '%' : '—'}
                  </div>
                </div>
              </div>
            );
          })}
        </Panel>

        <Panel title={d.dirAccuracyChart} sub={effectiveDirMarket ? `${effectiveDirMarket} · ${d.dirAccuracySub.split('·').slice(-1)[0].trim()}` : d.dirAccuracySub}>
          {!user ? (
            <div className="empty" style={{ padding: '24px 0' }}>
              <div className="empty__icon"><Icon name="cpu" size={16} /></div>
              <p style={{ fontSize: 12 }}>{d.loginToSeeAccuracy}</p>
            </div>
          ) : Object.keys(algoDirectionAcc).length === 0 ? (
            <div className="empty">
              <div className="empty__icon"><Icon name="layers" size={18} /></div>
              <p style={{ fontSize: 12 }}>{d.noReconcileData}</p>
            </div>
          ) : (
            <HBars items={D.algos
              .filter(a => algoDirectionAcc[a.id] != null)
              .map((a) => ({
                label: a.name,
                value: algoDirectionAcc[a.id] ?? 0,
                color: (algoDirectionAcc[a.id] ?? 0) >= 55
                  ? 'var(--up)'
                  : (algoDirectionAcc[a.id] ?? 0) >= 45
                    ? 'var(--gold)'
                    : 'var(--down)',
              }))}
            />
          )}
        </Panel>
        </div>
      </div>
    </div>
  );
}

// ── Top Simulation Bots widget ────────────────────────────────────────────────

interface SimBotFull {
  rank: number;
  bot_id: string;
  display_name: string;
  total_return_pct: number;
  market: string;
  algorithm: string;
  // config (fetched from bot detail)
  buy_threshold?: number;
  sell_threshold?: number;
  min_confidence?: number;
  stop_loss?: number;
  take_profit?: number;
}

function ConfigChip({ label, value }: { label: string; value: string }) {
  return (
    <span style={{
      fontSize: 10, padding: '1px 6px',
      background: 'var(--surface-2)', color: 'var(--text-3)',
      fontFamily: 'var(--font-mono)', whiteSpace: 'nowrap',
    }}>
      {label}: {value}
    </span>
  );
}

function TopSimulationBots() {
  const { t } = useLanguage();
  const [bots, setBots] = useState<SimBotFull[]>([]);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    fetch('/api/simulation/leaderboard?limit=5', { headers: { Accept: 'application/json' } })
      .then((r) => r.ok ? r.json() : null)
      .then((d) => {
        if (!d || !Array.isArray(d.leaderboard)) { setLoaded(true); return; }
        setBots(d.leaderboard.slice(0, 5) as SimBotFull[]);
        setLoaded(true);
      })
      .catch(() => setLoaded(true));
  }, []);

  if (!loaded) return null;

  const MARKET_COLORS: Record<string, string> = {
    GOLD: 'var(--gold)', NASDAQ: 'oklch(0.74 0.13 200)',
    SP500: 'var(--up)', CRYPTO: 'oklch(0.72 0.16 280)',
  };

  return (
    <Panel
      title={t.dashboard.topSimBots}
      sub={t.dashboard.topBotsDesc}
      className="section-gap"
      tools={
        <Link to="/simulation" style={{ fontSize: 12, color: 'var(--accent)', textDecoration: 'none' }}>
          {t.dashboard.viewAll}
        </Link>
      }
      style={{ paddingBottom: 4 }}
    >
      {bots.length === 0 ? (
        <div className="empty" style={{ padding: '24px 0' }}>
          <div className="empty__icon"><Icon name="layers" size={16} /></div>
          <p style={{ fontSize: 12 }}>
            {t.dashboard.noSimData}{' '}
            <Link to="/simulation" style={{ color: 'var(--accent)' }}>{t.dashboard.runBacktest}</Link>
          </p>
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column' }}>
          {bots.map((b, i) => {
            const marketColor = MARKET_COLORS[b.market] || 'var(--accent)';
            return (
              <Link
                key={b.bot_id}
                to={'/simulation/' + b.bot_id}
                style={{ textDecoration: 'none', color: 'inherit' }}
              >
                <div
                  className="lrow"
                  style={{
                    cursor: 'pointer',
                    padding: '10px 16px',
                    borderBottom: '1px solid var(--border)',
                    flexDirection: 'column',
                    alignItems: 'stretch',
                    gap: 6,
                  }}
                >
                  {/* Top row: rank + name + return */}
                  <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                    <span style={{
                      fontFamily: 'var(--font-mono)', fontWeight: 700, fontSize: 15,
                      minWidth: 28, color: i === 0 ? 'var(--gold)' : 'var(--text-3)',
                    }}>
                      #{b.rank}
                    </span>
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{ fontWeight: 600, fontSize: 13, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {b.display_name}
                      </div>
                    </div>
                    <Chg pct={b.total_return_pct} />
                  </div>

                  {/* Bottom row: market badge + algo + config chips */}
                  <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap', paddingLeft: 38 }}>
                    <span style={{
                      fontSize: 10, padding: '1px 7px', fontWeight: 700,
                      background: `${marketColor}22`, color: marketColor,
                      fontFamily: 'var(--font-mono)',
                    }}>
                      {b.market}
                    </span>
                    <span style={{
                      fontSize: 10, padding: '1px 7px',
                      background: 'var(--surface-2)', color: 'var(--text-2)',
                      fontFamily: 'var(--font-mono)',
                    }}>
                      {algoLabel(b.algorithm)}
                    </span>
                    {b.buy_threshold != null && (
                      <ConfigChip label="buy" value={b.buy_threshold.toFixed(1) + '%'} />
                    )}
                    {b.sell_threshold != null && (
                      <ConfigChip label="sell" value={b.sell_threshold.toFixed(1) + '%'} />
                    )}
                    {b.min_confidence != null && (
                      <ConfigChip label="conf" value={Math.round(b.min_confidence * 100) + '%'} />
                    )}
                    {b.stop_loss != null && (
                      <ConfigChip label="sl" value={b.stop_loss.toFixed(1) + '%'} />
                    )}
                    {b.take_profit != null && (
                      <ConfigChip label="tp" value={b.take_profit.toFixed(1) + '%'} />
                    )}
                  </div>
                </div>
              </Link>
            );
          })}
        </div>
      )}
    </Panel>
  );
}
