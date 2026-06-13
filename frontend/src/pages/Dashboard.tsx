import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useData } from '../context/DataContext';
import { Panel, KPI, Icon, Chg } from '../components/ui';
import { HBars } from '../components/charts';
import { useLanguage } from '../context/LangContext';

export default function Dashboard() {
  const { data: D } = useData();
  const { t } = useLanguage();
  const ens = (D.algos || []).find((a) => a.cls === 'ens');
  const ensAcc = ens ? ens.acc : (D.stats && D.stats.acc ? D.stats.acc : 0);
  const ensAccDisplay = ensAcc ? (+ensAcc).toFixed(1) + '%' : '—%';

  return (
    <div className="content__inner fade">
      <div className="grid grid--kpis section-gap">
        <KPI label={t.dashboard.ensAccuracy} value={ensAccDisplay} sub={t.dashboard.ensModel} chgPct={ens ? ens.accDelta : 0} accent />
      </div>

      <Panel title={t.dashboard.mlModelStatus} sub={D.algos.length ? D.algos.length + ' ' + t.dashboard.algorithms : '—'} flush className="section-gap">
          {D.algos.length === 0
            ? <div className="empty">
                <div className="empty__icon"><Icon name="layers" size={18} /></div>
                <p>{t.dashboard.noTrainingData}</p>
              </div>
            : D.algos.map((a) => (
              <div className="lrow" key={a.id}>
                <span className={`algo algo--${a.cls}`} style={{ minWidth: 52, textAlign: 'center' }}>{a.short}</span>
                <div className="lrow__main">
                  <div className="lrow__sym" style={{ fontSize: 12.5 }}>{a.name}</div>
                  <div className="bar" style={{ marginTop: 6, width: 130 }}>
                    <div className="bar__fill up" style={{ width: a.acc + '%' }}></div>
                  </div>
                </div>
                <div className="lrow__rt">
                  <div className="num" style={{ fontSize: 14, fontWeight: 600 }}>{a.acc}%</div>
                  <div style={{ fontSize: 11 }}><Chg pct={a.accDelta} /></div>
                </div>
              </div>
            ))
          }
      </Panel>

      <Panel title={t.dashboard.algoAccuracy} sub={t.dashboard.last30Sessions} className="section-gap">
        {D.algos.length === 0
          ? <div className="empty">
              <div className="empty__icon"><Icon name="layers" size={18} /></div>
              <p>{t.dashboard.noTrainingData}</p>
            </div>
          : <HBars items={D.algos.map((a) => ({
              label: a.name, value: a.acc,
              color: a.cls === 'ens' ? 'oklch(0.72 0.14 300)' : a.cls === 'lstm' ? 'oklch(0.74 0.13 200)' : a.cls === 'arima' ? 'var(--gold)' : 'var(--up)',
            }))} />
          }
      </Panel>

      <TopSimulationBots />
    </div>
  );
}

// ── Top Simulation Bots widget ───────────────────────────────────────────────

interface SimBot {
  rank: number;
  bot_id: string;
  display_name: string;
  total_return_pct: number;
  market: string;
}

function TopSimulationBots() {
  const { t } = useLanguage();
  const [bots, setBots] = useState<SimBot[]>([]);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    fetch('/api/simulation/leaderboard?limit=3', { headers: { Accept: 'application/json' } })
      .then((r) => r.ok ? r.json() : null)
      .then((d) => {
        if (d && Array.isArray(d.leaderboard)) {
          setBots(d.leaderboard.slice(0, 3));
        }
        setLoaded(true);
      })
      .catch(() => setLoaded(true));
  }, []);

  if (!loaded) return null;

  return (
    <Panel
      title={t.dashboard.topSimBots}
      sub={t.dashboard.topBotsDesc}
      className="section-gap"
      tools={
        <Link to="/simulation" style={{ fontSize: 12, color: 'var(--accent)', textDecoration: 'none', display: 'flex', alignItems: 'center', gap: 4 }}>
          {t.dashboard.viewAll}
        </Link>
      }
      style={{ paddingBottom: 4 }}
    >
      {bots.length === 0 ? (
        <div className="empty" style={{ padding: '24px 0' }}>
          <div className="empty__icon"><Icon name="layers" size={16} /></div>
          <p style={{ fontSize: 12 }}>{t.dashboard.noSimData} <Link to="/simulation" style={{ color: 'var(--accent)' }}>{t.dashboard.runBacktest}</Link></p>
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          {bots.map((b, i) => (
            <Link
              key={b.bot_id}
              to={'/simulation/' + b.bot_id}
              style={{ textDecoration: 'none', color: 'inherit' }}
            >
              <div className="lrow" style={{ cursor: 'pointer' }}>
                <span style={{ fontFamily: 'var(--font-mono)', fontWeight: 700, fontSize: 16, minWidth: 28, color: i === 0 ? 'var(--gold)' : 'var(--text-3)' }}>
                  #{b.rank}
                </span>
                <div className="lrow__main">
                  <div className="lrow__sym" style={{ fontSize: 13 }}>{b.display_name}</div>
                  <div style={{ fontSize: 11, color: 'var(--text-3)' }}>{b.market}</div>
                </div>
                <div className="lrow__rt">
                  <Chg pct={b.total_return_pct} />
                </div>
              </div>
            </Link>
          ))}
        </div>
      )}
    </Panel>
  );
}
