import { useState, useEffect } from 'react';
import { useData } from '../context/DataContext';
import { Panel, KPI, Icon, Chg, Seg, ConfBar } from '../components/ui';
import { LineChart, HBars, Scatter } from '../components/charts';
import { useLanguage } from '../context/LangContext';

// ── Color/name helpers ─────────────────────────────────────────────────────────
const ALGO_COLORS: Record<string, string> = {
  lstm_nn:        'oklch(0.74 0.13 200)',
  lstm:           'oklch(0.74 0.13 200)',
  arima_garch:    'var(--gold)',
  arima:          'var(--gold)',
  moving_average: 'var(--up)',
  ema:            'oklch(0.72 0.18 150)',
  ema_macd:       'oklch(0.72 0.18 150)',
  lightgbm:       'oklch(0.75 0.16 30)',
  random_forest:  'oklch(0.72 0.14 270)',
  xgboost:        'oklch(0.73 0.17 350)',
  gru:            'oklch(0.72 0.15 240)',
  ensemble:       'oklch(0.72 0.14 300)',
};
const FALLBACK_COLORS = ['var(--accent)', 'oklch(0.72 0.14 300)', 'var(--gold)', 'oklch(0.72 0.18 150)', 'var(--up)'];
function algoColor(key: string, idx: number): string {
  return ALGO_COLORS[key.toLowerCase()] || FALLBACK_COLORS[idx % FALLBACK_COLORS.length];
}
const ALGO_DISPLAY: Record<string, string> = {
  lstm_nn: 'LSTM', lstm: 'LSTM', arima_garch: 'ARIMA', arima: 'ARIMA',
  moving_average: 'MA', ema: 'EMA', ema_macd: 'EMA/MACD',
  lightgbm: 'LightGBM', random_forest: 'RF', xgboost: 'XGBoost',
  gru: 'GRU', ensemble: 'Ensemble',
};
function algoDisplayName(key: string): string {
  return ALGO_DISPLAY[key.toLowerCase()] || key;
}
function buildMultiAlgoData(
  list: any[],
  dateField: string,
  actualField: string,
  predField: string,
  algoField: string,
  fmtDate: (s: string) => string,
): { labels: string[]; actual: (number | null)[]; predSeries: { key: string; data: (number | null)[] }[] } {
  const sorted = [...list].sort((a, b) => (a[dateField] || '') < (b[dateField] || '') ? -1 : 1);
  const uniqueDates: string[] = [];
  const dateIndex = new Map<string, number>();
  for (const it of sorted) {
    const d = it[dateField] || '';
    if (!dateIndex.has(d)) { dateIndex.set(d, uniqueDates.length); uniqueDates.push(d); }
  }
  const n = uniqueDates.length;
  const actual: (number | null)[] = new Array(n).fill(null);
  for (const it of sorted) {
    const idx = dateIndex.get(it[dateField] || '');
    if (idx !== undefined && actual[idx] === null && it[actualField] != null) {
      const v = parseFloat(it[actualField]);
      actual[idx] = isFinite(v) ? v : null;
    }
  }
  const algoMap = new Map<string, (number | null)[]>();
  for (const it of sorted) {
    const key = (it[algoField] || 'unknown').toLowerCase();
    if (!algoMap.has(key)) algoMap.set(key, new Array(n).fill(null));
    const idx = dateIndex.get(it[dateField] || '');
    if (idx !== undefined && it[predField] != null) {
      const v = parseFloat(it[predField]);
      algoMap.get(key)![idx] = isFinite(v) ? v : null;
    }
  }
  return { labels: uniqueDates.map(fmtDate), actual, predSeries: Array.from(algoMap.entries()).map(([key, data]) => ({ key, data })) };
}

export default function Predictions() {
  const { data: D } = useData();
  const { fmt } = D;
  const { t } = useLanguage();
  const [algo, setAlgo] = useState('');
  const [range, setRange] = useState('30');
  const [compareSym, setCompareSym] = useState('');

  const conf = D.confirmed.filter((c) => !algo || c.algo === algo);
  const avgAcc = D.confirmed.length ? D.confirmed.reduce((a, c) => a + c.acc, 0) / D.confirmed.length : 0;
  const avgErr = D.confirmed.length ? D.confirmed.reduce((a, c) => a + c.err, 0) / D.confirmed.length : 0;
  const summary = {
    total:     D.stats ? D.stats.total : (D.predictions.length + D.confirmed.length),
    acc:       D.stats ? D.stats.acc : +avgAcc.toFixed(1),
    confirmed: D.confirmed.length,
    err:       +avgErr.toFixed(2),
  };

  const effectiveSym = compareSym || (D.stocks[0] ? D.stocks[0].sym : '');

  // ── Compare chart: real API ──────────────────────────────────────────────────
  const [compareData, setCompareData] = useState<{ labels: string[]; actual: (number | null)[]; predSeries: { key: string; data: (number | null)[] }[] } | null>(null);
  const [compareLoading, setCompareLoading] = useState(false);

  useEffect(() => {
    if (!effectiveSym) { setCompareData(null); return; }
    setCompareLoading(true);
    const url = '/api/predictions/compare/' + encodeURIComponent(effectiveSym) + '?days=90' + (algo ? '&algorithm=' + encodeURIComponent(algo) : '');
    fetch(url, { headers: { Accept: 'application/json' } })
      .then(function(r) { if (!r.ok) throw new Error('HTTP ' + r.status); return r.json(); })
      .then(function(res: any) {
        const raw: any[] = Array.isArray(res.data) ? res.data : [];
        const ddmm = function(s: string) {
          try {
            const d = new Date(s);
            if (isNaN(d.getTime())) return s.slice(0, 10);
            return ('0' + d.getDate()).slice(-2) + '/' + ('0' + (d.getMonth() + 1)).slice(-2);
          } catch (_e) { return s ? s.slice(0, 10) : ''; }
        };
        setCompareData(buildMultiAlgoData(raw, 'date', 'actual', 'predicted', 'algorithm', ddmm));
      })
      .catch(function() { setCompareData(null); })
      .finally(function() { setCompareLoading(false); });
  }, [effectiveSym, algo]);

  // ── Scatter chart: real API ──────────────────────────────────────────────────
  const [scatterData, setScatterData] = useState<{ x: number; y: number; label: string; color: string }[]>([]);

  useEffect(() => {
    const url = '/api/predictions/error-distribution' + (algo ? '?algorithm=' + encodeURIComponent(algo) : '');
    fetch(url, { headers: { Accept: 'application/json' } })
      .then(function(r) { if (!r.ok) throw new Error('HTTP ' + r.status); return r.json(); })
      .then(function(res: any) {
        const raw: any[] = Array.isArray(res.data) ? res.data : [];
        const pts = raw.slice(0, 500).map(function(it: any) {
          // Derive color from the runtime algoMap via D.accTrend.series (same palette logic).
          const algoKey = (it.algorithm || '').toLowerCase();
          const series = D.accTrend.series.find((s) => s.key === algoKey);
          const color = series ? series.color : 'var(--text-3)';
          const predChg = parseFloat(it.predicted_change_pct) || 0;
          const actChg  = parseFloat(it.actual_change_pct) || 0;
          return {
            x: Math.abs(predChg),
            y: Math.abs(predChg - actChg),
            label: (it.symbol || '?') + ' \u00b7 ' + (it.algorithm || ''),
            color,
          };
        });
        setScatterData(pts);
      })
      .catch(function() { setScatterData([]); });
  }, [algo, D.accTrend.series]);

  const scatterPts = scatterData;

  const sorted = [...D.confirmed].sort((a, b) => b.acc - a.acc);
  const best  = sorted.slice(0, 5);
  const worst = sorted.slice(-5).reverse();

  return (
    <div className="content__inner fade">
      <div className="grid grid--kpis section-gap">
        <KPI label={t.predictions.totalPredictions} value={summary.total || '—'}     sub={t.predictions.last30Days} />
        <KPI label={t.predictions.avgAccuracy}      value={summary.acc ? summary.acc + '%' : '—%'} sub={t.predictions.allModels} accent />
        <KPI label={t.predictions.confirmed}        value={summary.confirmed || '—'} sub={t.predictions.withActualPrice} />
        <KPI label={t.predictions.avgError}         value={summary.err ? summary.err + '%' : '—%'} sub={t.predictions.errorDiff} />
      </div>

      <Panel className="section-gap">
        <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'center' }}>
          <Icon name="filter" size={15} style={{ color: 'var(--text-3)' }} />
          <select className="sel" value={algo} onChange={(e) => setAlgo(e.target.value)}>
            <option value="">{t.common.allAlgos}</option>
            {D.algos.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
          </select>
          <Seg options={[{ value: '7', label: t.dateRange.d7 }, { value: '30', label: t.dateRange.d30 }, { value: '90', label: t.dateRange.d90 }]} value={range} onChange={setRange} />
          <div style={{ marginLeft: 'auto', display: 'flex', gap: 8 }}>
            <button className="btn btn--sm"><Icon name="download" size={13} />{t.common.exportCsv}</button>
            <button className="btn btn--sm btn--ghost"><Icon name="refresh" size={13} />{t.common.refresh}</button>
          </div>
        </div>
      </Panel>

      <div className="grid grid--wide section-gap">
        <Panel title={t.predictions.algoPerformance} sub={t.predictions.accuracyLabel}>
          {D.algos.length === 0
            ? <div className="empty">
                <div className="empty__icon"><Icon name="layers" size={18} /></div>
                <p>{t.predictions.noTrainingData}</p>
              </div>
            : <>
                <HBars items={D.algos.map((a) => ({
                  label: a.name, value: a.acc,
                  color: a.cls === 'ens' ? 'oklch(0.72 0.14 300)' : a.cls === 'lstm' ? 'oklch(0.74 0.13 200)' : a.cls === 'arima' ? 'var(--gold)' : a.cls === 'ema' ? 'oklch(0.72 0.18 150)' : 'var(--up)',
                }))} />
                <div style={{ marginTop: 18, display: 'grid', gridTemplateColumns: 'repeat(2,1fr)', gap: 1, background: 'var(--border)', border: '1px solid var(--border)' }}>
                  {D.algos.map((a) => (
                    <div key={a.id} style={{ background: 'var(--surface)', padding: '10px 12px' }}>
                      <span className={`algo algo--${a.cls}`}>{a.short}</span>
                      <div className="num" style={{ fontSize: 13, marginTop: 6, color: 'var(--text-2)' }}>MAE {a.mae}</div>
                    </div>
                  ))}
                </div>
              </>
          }
        </Panel>
        <Panel title={t.predictions.accuracyTrend} sub={t.predictions.recentSessions}>
          {D.accTrend.labels.length === 0
            ? <div className="empty">
                <div className="empty__icon"><Icon name="layers" size={18} /></div>
                <p>{t.predictions.noTrendData}</p>
              </div>
            : <>
                <LineChart
                  series={D.accTrend.series.map((s) => ({ name: s.name, data: s.data, color: s.color }))}
                  labels={D.accTrend.labels} height={540} yFmt={(v) => v.toFixed(0) + '%'} valueFmt={(v) => v.toFixed(1) + '%'}
                />
                <Legend items={D.accTrend.series.map((s) => [s.name, s.color] as [string, string])} />
              </>
          }
        </Panel>
      </div>

      <div className="sec-head"><h2>{t.predictions.allPredictions}</h2><div className="line"></div></div>
      <Panel flush className="section-gap">
        {conf.length === 0
          ? <div className="empty">
              <div className="empty__icon"><Icon name="layers" size={18} /></div>
              <p>{t.predictions.noConfirmedPred}</p>
            </div>
          : <div style={{ overflowX: 'auto' }}>
              <table className="tbl">
                <thead><tr>
                  <th>{t.predictions.colSymbol}</th><th className="c">{t.predictions.colAlgo}</th><th className="r">{t.predictions.colPredPrice}</th><th className="r">{t.predictions.colActualPrice}</th>
                  <th className="r">{t.predictions.colPredDelta}</th><th className="r">{t.predictions.colActualDelta}</th><th className="r">{t.predictions.colConfidence}</th>
                  <th className="r">{t.predictions.colAccuracy}</th><th className="c">{t.predictions.colStatus}</th>
                </tr></thead>
                <tbody>
                  {conf.map((c, i) => (
                    <tr key={i}>
                      <td><div className="sym">{c.sym}</div><div className="co">{c.name}</div></td>
                      <td className="c"><span className={`algo algo--${c.algoCls}`}>{c.algoShort}</span></td>
                      <td className="r num">{fmt.price(c.pred)}</td>
                      <td className="r num" style={{ color: 'var(--text-2)' }}>{fmt.price(c.actual)}</td>
                      <td className="r"><Chg pct={c.predDelta} /></td>
                      <td className="r"><Chg pct={c.actualDelta} /></td>
                      <td className="r"><ConfBar v={c.conf} /></td>
                      <td className="r num" style={{ fontWeight: 600, color: c.acc > 85 ? 'var(--up)' : c.acc > 72 ? 'var(--text)' : 'var(--down)' }}>{c.acc}%</td>
                      <td className="c">
                        <span className={`badge badge--${c.status === 'hit' ? 'up' : c.status === 'miss' ? 'down' : 'neutral'}`}>
                          {c.status === 'hit' ? t.predictions.statusHit : c.status === 'miss' ? t.predictions.statusMiss : t.predictions.statusNear}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
        }
      </Panel>

      <div className="grid grid--side section-gap">
        <Panel title={t.predictions.compareChart} dot={effectiveSym || '—'}
          tools={D.stocks.length > 0 &&
            <select className="sel" value={effectiveSym} onChange={(e) => setCompareSym(e.target.value)}>
              {D.stocks.map((s) => <option key={s.sym}>{s.sym}</option>)}
            </select>
          }>
          {compareLoading
            ? <div className="empty" style={{ height: 540, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
                <div className="empty__icon"><Icon name="layers" size={18} /></div>
                <p>{t.predictions.loadingCompare}</p>
              </div>
            : (!compareData || compareData.labels.length === 0)
              ? <div className="empty" style={{ height: 540, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
                  <div className="empty__icon"><Icon name="layers" size={18} /></div>
                  <p>{t.predictions.noCompareData}</p>
                </div>
              : <>
                  <LineChart
                    series={[
                      { name: t.common.actual, data: compareData.actual, color: 'var(--text-2)', w: 1.8 },
                      ...compareData.predSeries.map((ps, i) => ({
                        name: algoDisplayName(ps.key),
                        data: ps.data,
                        color: algoColor(ps.key, i),
                        dash: '5 4',
                        w: 1.6,
                      })),
                    ]}
                    labels={compareData.labels} height={540} valueFmt={(v) => fmt.price(v)}
                  />
                  <Legend items={[
                    [t.common.actual, 'var(--text-2)'],
                    ...compareData.predSeries.map((ps, i) => [algoDisplayName(ps.key), algoColor(ps.key, i)] as [string, string]),
                  ]} />
                </>
          }
        </Panel>

        <Panel title={t.predictions.errorDist} sub={t.predictions.errorDistSub}>
          {scatterPts.length === 0
            ? <div className="empty" style={{ height: 520, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
                <div className="empty__icon"><Icon name="layers" size={18} /></div>
                <p>{t.predictions.noAnalysisData}</p>
              </div>
            : <>
                <Scatter points={scatterPts} height={520} xLabel={t.predictions.confidenceAxis} />
                <Legend items={D.accTrend.series.map((s) => [s.name, s.color] as [string, string])} />
              </>
          }
        </Panel>
      </div>

      {(best.length > 0 || worst.length > 0) && (
        <div className="grid grid--halves" style={{ paddingBottom: 8 }}>
          <FeaturedPanel title={t.predictions.top5Accurate} items={best} good fmt={fmt} />
          <FeaturedPanel title={t.predictions.top5Inaccurate} items={worst} good={false} fmt={fmt} />
        </div>
      )}
    </div>
  );
}

function Legend({ items }: { items: [string, string][] }) {
  return (
    <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap', marginTop: 12, fontSize: 12, color: 'var(--text-3)' }}>
      {items.map(([l, c]) => (
        <span key={l} style={{ display: 'inline-flex', alignItems: 'center', gap: 7 }}>
          <span style={{ width: 12, height: 3, background: c, display: 'inline-block' }}></span>{l}
        </span>
      ))}
    </div>
  );
}

function FeaturedPanel({ title, items, good, fmt }: { title: string; items: any[]; good: boolean; fmt: any }) {
  const { t } = useLanguage();
  return (
    <Panel title={title} flush>
      {items.length === 0
        ? <div className="empty">
            <div className="empty__icon"><Icon name="layers" size={18} /></div>
            <p>{t.predictions.noData}</p>
          </div>
        : items.map((c, i) => (
          <div key={i} className="lrow">
            <span className="badge badge--muted" style={{ minWidth: 22, justifyContent: 'center' }}>{i + 1}</span>
            <div className="lrow__main">
              <div className="lrow__sym">{c.sym} <span className={`algo algo--${c.algoCls}`} style={{ marginLeft: 4 }}>{c.algoShort}</span></div>
              <div className="lrow__sub">{t.predictions.predLabel} {fmt.price(c.pred)} · {t.predictions.actualLabel} {fmt.price(c.actual)}</div>
            </div>
            <div className="lrow__rt">
              <div className="num" style={{ fontWeight: 600, color: good ? 'var(--up)' : 'var(--down)' }}>{c.acc}%</div>
              <div style={{ fontSize: 11, color: 'var(--text-3)' }}>{t.predictions.errorLabel} {c.err}%</div>
            </div>
          </div>
        ))
      }
    </Panel>
  );
}
