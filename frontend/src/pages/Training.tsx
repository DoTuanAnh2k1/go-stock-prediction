import { useData } from '../context/DataContext';
import { Panel, KPI, Icon, Chg } from '../components/ui';
import { Donut } from '../components/charts';
import { train } from '../api';
import { vnsToast } from '../components/ui';
import { useLanguage } from '../context/LangContext';

export default function Training() {
  const { data: D } = useData();
  const { t } = useLanguage();
  const running = D.trainJobs.find((j) => j.status === 'running');
  const bestAlgo = D.algos.length ? [...D.algos].sort((a, b) => b.acc - a.acc)[0] : null;

  const handleTrain = () => {
    train().then((ok) => vnsToast(ok ? t.training.trainRequest : t.training.trainFail));
  };

  return (
    <div className="content__inner fade">
      <div className="grid grid--kpis section-gap">
        <KPI label={t.training.modelsTrained} value={D.algos.length ? D.algos.length + ' / ' + D.algos.length : '—'} sub={D.algos.length ? D.algos.map((a) => a.short).join(' · ') : '—'} />
        <KPI label={t.training.bestAccuracy} value={bestAlgo ? bestAlgo.acc.toFixed(1) + '%' : '—%'} sub={bestAlgo ? bestAlgo.name : '—'} accent />
        <KPI label={t.training.totalData} value={D.stocks.length ? D.stocks.length + ' mã' : '—'} sub={t.training.stocksTracked} />
        <KPI label={t.training.lastTrained} value={bestAlgo && bestAlgo.trainedAt !== '—' ? bestAlgo.trainedAt : '—'} sub={t.training.automatic} />
      </div>

      <div className="grid grid--content section-gap">
        <Panel title={t.training.runningTask}
          tools={running
            ? <span className="badge badge--accent"><span className="spinner-dots" style={{ marginRight: 4 }}><i></i><i></i><i></i></span> RUNNING</span>
            : <span className="badge badge--muted">IDLE</span>
          }>
          {running
            ? <div>
                <div style={{ display: 'flex', alignItems: 'baseline', gap: 10 }}>
                  <span style={{ fontSize: 16, fontWeight: 600 }}>{running.name}</span>
                  <span style={{ fontSize: 12, color: 'var(--text-3)' }}>{running.sub}</span>
                </div>
                <div style={{ display: 'flex', justifyContent: 'space-between', margin: '16px 0 6px', fontSize: 12 }}>
                  <span style={{ color: 'var(--text-3)' }}>{t.training.progress}</span>
                  <span className="num" style={{ fontWeight: 600 }}>{running.progress}%</span>
                </div>
                <div className="bar" style={{ height: 10 }}><div className="bar__fill" style={{ width: running.progress + '%' }}></div></div>
                <div style={{ display: 'flex', gap: 8, marginTop: 16 }}>
                  <button className="btn btn--accent" onClick={handleTrain}><Icon name="play" size={13} />{t.training.retrainAll}</button>
                  <button className="btn">{t.training.stop}</button>
                </div>
              </div>
            : <div>
                <div style={{ color: 'var(--text-3)', fontSize: 13, marginBottom: 16 }}>{t.training.noRunningTask}</div>
                <button className="btn btn--accent" onClick={handleTrain}><Icon name="play" size={13} />{t.training.trainNow}</button>
              </div>
          }
        </Panel>

        <Panel title={t.training.trainingQueue} sub={D.trainJobs.length ? D.trainJobs.length + ' ' + t.training.tasks : '—'} flush>
          {D.trainJobs.length === 0
            ? <div className="empty">
                <div className="empty__icon"><Icon name="layers" size={18} /></div>
                <p>{t.training.noQueuedTasks}</p>
              </div>
            : D.trainJobs.map((j, i) => (
              <div key={i} className="lrow">
                <span className={`badge badge--${j.status === 'running' ? 'accent' : j.status === 'done' ? 'up' : 'muted'}`} style={{ minWidth: 74, justifyContent: 'center' }}>
                  {j.status === 'running' ? t.training.statusRunning : j.status === 'done' ? t.training.statusDone : t.training.statusWaiting}
                </span>
                <div className="lrow__main">
                  <div className="lrow__sym" style={{ fontSize: 13 }}>{j.name}</div>
                  <div className="lrow__sub">{j.sub}</div>
                </div>
                <div style={{ width: 110 }}>
                  <div className="bar" style={{ height: 6 }}><div className={`bar__fill ${j.status === 'done' ? 'up' : ''}`} style={{ width: j.progress + '%' }}></div></div>
                  <div style={{ fontSize: 10.5, color: 'var(--text-3)', textAlign: 'right', marginTop: 4, fontFamily: 'var(--font-mono)' }}>{j.eta}</div>
                </div>
              </div>
            ))
          }
        </Panel>
      </div>

      <div className="sec-head"><h2>{t.training.models}</h2><div className="line"></div></div>
      {D.algos.length === 0
        ? <div className="empty section-gap">
            <div className="empty__icon"><Icon name="layers" size={18} /></div>
            <p>{t.training.noModelData}</p>
          </div>
        : <div className="grid grid--kpis section-gap">
            {D.algos.map((a) => (
              <div className="panel" key={a.id} style={{ padding: 18, display: 'flex', flexDirection: 'column', gap: 14 }}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <span className={`algo algo--${a.cls}`} style={{ fontSize: 12 }}>{a.short}</span>
                  <span className="badge badge--up">{t.training.trained}</span>
                </div>
                <div>
                  <div style={{ fontWeight: 600, fontSize: 14 }}>{a.name}</div>
                  {a.desc && <div style={{ fontSize: 11.5, color: 'var(--text-3)', marginTop: 2 }}>{a.desc}</div>}
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
                  <Donut value={a.acc} size={72} stroke={8}
                    color={a.cls === 'ens' ? 'oklch(0.72 0.14 300)' : a.cls === 'lstm' ? 'oklch(0.74 0.13 200)' : a.cls === 'arima' ? 'var(--gold)' : 'var(--up)'}
                    label="acc" />
                  <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 8, fontSize: 12 }}>
                    <KV k="MAE" v={a.mae} />
                    <KV k={t.training.accuracyDelta} v={<Chg pct={a.accDelta} />} />
                    <KV k={t.training.updated} v={a.trainedAt} />
                  </div>
                </div>
              </div>
            ))}
          </div>
      }

      <Panel title={t.training.trainingLog} sub={t.training.realtime} flush style={{ paddingBottom: 8 }}
        tools={<span className="badge badge--up"><span className="mkt__dot live" style={{ width: 6, height: 6 }}></span> LIVE</span>}>
        <div style={{ background: 'oklch(0.13 0.012 245)', padding: '12px 0', maxHeight: 320, overflowY: 'auto', fontFamily: 'var(--font-mono)', fontSize: 12 }}>
          {D.trainLogs.length === 0
            ? <div style={{ padding: '20px 18px', color: 'var(--text-3)', fontSize: 12 }}>{t.training.noLogs}</div>
            : D.trainLogs.map(([t, lvl, msg], i) => (
              <div key={i} style={{ display: 'flex', gap: 14, padding: '4px 18px' }}>
                <span style={{ color: 'var(--text-3)', minWidth: 64 }}>{t}</span>
                <span style={{ minWidth: 64, fontWeight: 700, color: lvl === 'success' ? 'var(--up)' : lvl === 'warning' ? 'var(--gold)' : lvl === 'error' ? 'var(--down)' : 'oklch(0.74 0.13 200)' }}>{lvl.toUpperCase()}</span>
                <span style={{ color: 'var(--text-2)' }}>{msg}</span>
              </div>
            ))
          }
        </div>
      </Panel>
    </div>
  );
}

function KV({ k, v }: { k: string; v: React.ReactNode }) {
  return (
    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
      <span style={{ color: 'var(--text-3)' }}>{k}</span>
      <span className="num" style={{ fontWeight: 600 }}>{v}</span>
    </div>
  );
}
