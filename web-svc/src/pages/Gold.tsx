import { useState, useEffect } from 'react';
import { useData } from '../context/DataContext';
import { Panel, KPI, Icon, Chg, Seg, ConfBar, MarketTabs } from '../components/ui';
import { Sparkline, LineChart, Candlestick } from '../components/charts';
import { crawlGold, predictGold, goldBacktest, goldChart } from '../api';
import { vnsToast } from '../components/ui';
import { useLanguage } from '../context/LangContext';
import { useAuth } from '../context/AuthContext';

// ── Helpers ──────────────────────────────────────────────────────────────────
function num(x: any): number {
  const n = typeof x === 'number' ? x : parseFloat(x);
  return isFinite(n) ? n : 0;
}

function ddmm(s: string): string {
  if (!s) return '';
  try {
    const d = new Date(s);
    if (isNaN(d.getTime())) return s.slice(0, 10);
    return ('0' + d.getDate()).slice(-2) + '/' + ('0' + (d.getMonth() + 1)).slice(-2);
  } catch {
    return s.slice(0, 10);
  }
}

function fmtDT(s: string): string {
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

function algoShort(name: string): { short: string; cls: string } {
  const map: Record<string, { short: string; cls: string }> = {
    lstm_nn: { short: 'LSTM', cls: 'lstm' },
    lstm: { short: 'LSTM', cls: 'lstm' },
    arima_garch: { short: 'ARIMA', cls: 'arima' },
    moving_average: { short: 'MA', cls: 'ma' },
    ema: { short: 'EMA', cls: 'ema' },
    ensemble: { short: 'ENS', cls: 'ens' },
  };
  return map[(name || '').toLowerCase()] || { short: (name || '?').slice(0, 5).toUpperCase(), cls: 'unknown' };
}

interface GoldConfirmedItem {
  symbol?: string;
  source?: string;
  product_type?: string;
  algorithm_name: string;
  predicted_price: number;
  actual_price?: number | null;
  accuracy?: number | null;
  prediction_date: string;
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

export default function Gold() {
  const { data: D } = useData();
  const { fmt } = D;
  const { t } = useLanguage();
  const { user, canAccessMarket } = useAuth();
  const srcs = D.goldSources || [];
  const [active, setActive] = useState<string | null>(null);
  const [days, setDays] = useState('180');
  const [confirmedResults, setConfirmedResults] = useState<GoldConfirmedItem[]>([]);
  const [predGoldFilter, setPredGoldFilter] = useState('');    // '' | 'xau' | 'sjc'
  const [confirmedGoldFilter, setConfirmedGoldFilter] = useState('');
  const [predPage, setPredPage] = useState(0);
  const [confirmedPage, setConfirmedPage] = useState(0);
  const [chartData, setChartData] = useState<{ labels: string[]; sell: number[]; opens: number[]; highs: number[]; lows: number[]; closes: number[]; granularity?: string }>({ labels: [], sell: [], opens: [], highs: [], lows: [], closes: [], granularity: '1d' });
  const [chartLoading, setChartLoading] = useState(false);
  const [chartType, setChartType] = useState<'line' | 'candle'>('line');
  const [predChartAlgo, setPredChartAlgo] = useState('');

  const src = srcs.find((g) => g.id === active) || srcs[0] || null;
  const isOz = src ? src.unit === 'oz' : false;

  useEffect(() => {
    if (!src) return;
    const daysN = parseInt(days, 10) || 180;
    setChartLoading(true);
    goldChart(src.source, src.product, daysN)
      .then((c) => setChartData({ labels: c.labels, sell: c.sell, opens: c.opens, highs: c.highs, lows: c.lows, closes: c.closes, granularity: c.granularity }))
      .finally(() => setChartLoading(false));
  }, [src?.id, days]);

  useEffect(() => {
    fetch('/api/gold/predictions/latest-results', { headers: { Accept: 'application/json', Authorization: `Bearer ${localStorage.getItem('vns_token') || ''}` } })
      .then((res) => res.ok ? res.json() : null)
      .then((data) => {
        if (data) {
          const raw = Array.isArray(data) ? data : Array.isArray(data?.data) ? data.data : [];
          setConfirmedResults(raw);
        }
      })
      .catch(() => {});
  }, []);

  const hist = chartData.sell;
  const histLabels = chartData.labels;
  const hasOHLC = chartData.closes.length > 0;

  const find = (pred: (g: any) => boolean) => srcs.find(pred);
  let kpis = [
    find((g) => /sjc/i.test(g.id) && !/nhan/i.test(g.id)),
    find((g) => /doji/i.test(g.id)) || find((g) => /btmc/i.test(g.id) && !/nhan/i.test(g.id)),
    find((g) => /pnj/i.test(g.id) || /nhan/i.test(g.id)),
    find((g) => g.unit === 'oz' || /xau/i.test(g.id)),
  ].filter(Boolean) as typeof srcs;
  const seen = new Set<string>();
  kpis = kpis.filter((g) => !seen.has(g.id) && seen.add(g.id) as any);
  for (let i = 0; kpis.length < 4 && i < srcs.length; i++) {
    if (!seen.has(srcs[i].id)) { kpis.push(srcs[i]); seen.add(srcs[i].id); }
  }

  const fmtGold = (v: number) => isOz ? '$' + v.toFixed(0) : (v / 1e6).toFixed(2) + 'tr';
  const fmtFull = (v: number) => isOz ? '$' + fmt.price(v) : fmt.vnd(Math.round(v));

  const PAGE_SIZE = 10;
  const filteredGoldPreds = predGoldFilter === 'xau' ? D.goldPreds.filter((p) => p.isOz)
    : predGoldFilter === 'sjc' ? D.goldPreds.filter((p) => !p.isOz)
    : D.goldPreds;
  const predPageCount = Math.ceil(filteredGoldPreds.length / PAGE_SIZE);
  const predPagedItems = filteredGoldPreds.slice(predPage * PAGE_SIZE, (predPage + 1) * PAGE_SIZE);

  const isXauConfirmed = (r: GoldConfirmedItem) => (r.source || r.symbol || '').toLowerCase().includes('xau');
  const filteredConfirmed = confirmedGoldFilter === 'xau' ? confirmedResults.filter(isXauConfirmed)
    : confirmedGoldFilter === 'sjc' ? confirmedResults.filter((r) => !isXauConfirmed(r))
    : confirmedResults;
  const confirmedPageCount = Math.ceil(filteredConfirmed.length / PAGE_SIZE);
  const confirmedPagedItems = filteredConfirmed.slice(confirmedPage * PAGE_SIZE, (confirmedPage + 1) * PAGE_SIZE);

  if (user && !canAccessMarket('GOLD')) {
    return (
      <div className="content__inner fade">
        <div className="empty section-gap">
          <div className="empty__icon"><Icon name="gold" size={18} /></div>
          <p>{t.gold.noAccess}</p>
        </div>
      </div>
    );
  }

  if (srcs.length === 0) {
    return (
      <div className="content__inner fade">
        <MarketTabs marketKey="gold" />
        <div className="grid grid--kpis section-gap">
          {[1, 2, 3, 4].map((i) => <KPI key={i} label="—" value="—" sub={t.gold.loading} />)}
        </div>
        <div className="empty section-gap">
          <div className="empty__icon"><Icon name="layers" size={18} /></div>
          <p>{t.gold.loadingData}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="content__inner fade">
      <MarketTabs marketKey="gold" />
      <div className="grid grid--kpis section-gap">
        {kpis.map((g) => {
          const oz = g.unit === 'oz';
          return (
            <KPI key={g.id} label={g.name}
              value={oz ? '$' + fmt.price(g.sell) : (g.sell / 1e6).toFixed(2) + ' tr'}
              sub={'/ ' + g.unit} chgPct={g.chgPct} spark={g.spark} sparkColor="var(--gold)" />
          );
        })}
      </div>

      <Panel
        title={t.gold.priceChart}
        dot={src ? src.name : '—'}
        className="section-gap"
        tools={
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            <Seg options={[{ value: '1', label: t.dateRange.today }, { value: '7', label: t.dateRange.d7 }, { value: '30', label: t.dateRange.d30 }, { value: '90', label: t.dateRange.d90 }, { value: '180', label: t.dateRange.d180 }]} value={days} onChange={setDays} />
            {hasOHLC && (
              <Seg
                options={[
                  { value: 'line', label: t.common.chartType.line },
                  { value: 'candle', label: t.common.chartType.candle },
                ]}
                value={chartType}
                onChange={(v) => setChartType(v as 'line' | 'candle')}
              />
            )}
            <button className="btn btn--sm" style={{ background: 'var(--gold)', borderColor: 'var(--gold)', color: 'oklch(0.2 0.02 80)' }}
              onClick={() => crawlGold().then((ok) => vnsToast(ok ? t.gold.collectRequest : t.gold.collectFail))}>
              <Icon name="download" size={13} />{t.common.collect}
            </button>
          </div>
        }
      >
        <div className="chips" style={{ marginBottom: 16 }}>
          {srcs.map((g) => (
            <button key={g.id} className={`chip ${src && src.id === g.id ? 'active' : ''}`} onClick={() => setActive(g.id)}>{g.name}</button>
          ))}
        </div>
        {src && (
          <div style={{ display: 'flex', alignItems: 'baseline', gap: 14, marginBottom: 10 }}>
            <span className="num" style={{ fontSize: 30, fontWeight: 600, letterSpacing: '-1px' }}>{fmtFull(src.sell)}</span>
            <span style={{ fontSize: 12, color: 'var(--text-3)' }}>{t.gold.sellPrice} / {src.unit}</span>
            <Chg pct={src.chgPct} />
            <span style={{ marginLeft: 'auto', fontSize: 11, color: 'var(--text-3)', fontFamily: 'var(--font-mono)' }}>{src.vendor} · {src.region}</span>
          </div>
        )}
        {(() => {
          const fmtHistLabels = histLabels.length
            ? histLabels.map((l) => {
                if (chartData.granularity === '1h') {
                  return l.length >= 16 ? l.slice(11, 16) : l;
                }
                const p = l.slice(5).split('-'); return p[1] + '/' + p[0];
              })
            : hist.map((_, i) => `${i + 1}`);
          if (chartLoading) {
            return (
              <div className="empty" style={{ height: 540, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
                <div className="empty__icon"><Icon name="layers" size={18} /></div>
                <p>...</p>
              </div>
            );
          }
          if (hist.length === 0) {
            return (
              <div className="empty" style={{ height: 540, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
                <div className="empty__icon"><Icon name="layers" size={18} /></div>
                <p>{t.gold.noGoldHistory}</p>
              </div>
            );
          }
          if (chartType === 'candle' && hasOHLC) {
            return (
              <Candlestick
                data={chartData.closes.map((c, i) => ({ o: chartData.opens[i], h: chartData.highs[i], l: chartData.lows[i], c }))}
                labels={fmtHistLabels}
                height={540}
                yFmt={fmtGold}
                valueFmt={fmtFull}
                padL={58}
              />
            );
          }
          return (
            <LineChart
              series={[{ name: src!.name, data: hist, color: 'var(--gold)' }]}
              labels={fmtHistLabels}
              height={540} area yFmt={fmtGold} valueFmt={fmtFull} padL={58}
            />
          );
        })()}
      </Panel>

      <div className="sec-head"><h2>{t.gold.providerComparison}</h2><div className="line"></div></div>
      <Panel flush className="section-gap">
        <div style={{ overflowX: 'auto' }}>
          <table className="tbl">
            <thead><tr>
              <th>{t.gold.colProvider}</th><th>{t.gold.colType}</th><th className="c">{t.gold.colRegion}</th>
              <th className="r">{t.gold.colBuyPrice}</th><th className="r">{t.gold.colSellPrice}</th><th className="r">{t.gold.colSpread}</th><th className="r">±%</th><th className="c" style={{ width: 110 }}>24 mốc</th>
            </tr></thead>
            <tbody>
              {D.goldSources.map((g) => {
                const oz = g.unit === 'oz';
                return (
                  <tr key={g.id} className="clickable" onClick={() => setActive(g.id)}>
                    <td className="sym">{g.vendor}</td>
                    <td style={{ color: 'var(--text-2)' }}>{g.name}</td>
                    <td className="c" style={{ color: 'var(--text-3)', fontSize: 12 }}>{g.region}</td>
                    <td className="r num" style={{ color: 'var(--text-2)' }}>{oz ? '$' + fmt.price(g.buy) : fmt.vnd(g.buy)}</td>
                    <td className="r num" style={{ fontWeight: 600 }}>{oz ? '$' + fmt.price(g.sell) : fmt.vnd(g.sell)}</td>
                    <td className="r num" style={{ color: 'var(--text-3)' }}>{oz ? '$' + (g.sell - g.buy) : fmt.vnd(g.sell - g.buy)}</td>
                    <td className="r"><Chg pct={g.chgPct} /></td>
                    <td className="c"><div style={{ display: 'flex', justifyContent: 'center' }}><Sparkline data={g.spark} w={90} h={26} fill={false} color="var(--gold)" /></div></td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </Panel>

      <div className="grid grid--halves section-gap">
        <Panel title={t.gold.tomorrowPred} flush
          tools={
            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center' }}>
              <div className="chips" style={{ margin: 0 }}>
                <button className={`chip${predGoldFilter === '' ? ' active' : ''}`} style={{ fontSize: 12, padding: '3px 10px' }} onClick={() => { setPredGoldFilter(''); setPredPage(0); }}>{t.common.all}</button>
                <button className={`chip${predGoldFilter === 'xau' ? ' active' : ''}`} style={{ fontSize: 12, padding: '3px 10px' }} onClick={() => { setPredGoldFilter('xau'); setPredPage(0); }}>XAU/USD</button>
                <button className={`chip${predGoldFilter === 'sjc' ? ' active' : ''}`} style={{ fontSize: 12, padding: '3px 10px' }} onClick={() => { setPredGoldFilter('sjc'); setPredPage(0); }}>SJC</button>
              </div>
              <button className="btn btn--sm" style={{ background: 'var(--gold)', borderColor: 'var(--gold)', color: 'oklch(0.2 0.02 80)' }}
                onClick={() => predictGold().then((ok) => vnsToast(ok ? t.gold.predictRequest : t.gold.predictFail))}>
                <Icon name="play" size={13} />{t.gold.runPrediction}
              </button>
              <button className="btn btn--sm" style={{ background: 'var(--accent)', borderColor: 'var(--accent)' }}
                onClick={() => goldBacktest().then((ok) => vnsToast(ok ? t.gold.backtestRequest : t.gold.backtestFail))}>
                <Icon name="activity" size={13} />{t.gold.historicalBacktest}
              </button>
            </div>
          }>
          {filteredGoldPreds.length === 0
            ? <div className="empty">
                <div className="empty__icon"><Icon name="layers" size={18} /></div>
                <p>{D.goldPreds.length === 0 ? t.gold.noGoldPred : t.gold.noPredForType}</p>
              </div>
            : <div style={{ overflowX: 'auto' }}>
                <table className="tbl">
                  <thead><tr><th>{t.common.source}</th><th className="c">TT</th><th className="r">{t.common.current}</th><th className="r">{t.common.predicted}</th><th className="r">±%</th><th className="r">{t.common.accuracy}</th><th className="r">{t.common.confidence}</th></tr></thead>
                  <tbody>
                    {predPagedItems.map((p, i) => {
                      return (
                        <tr key={i}>
                          <td className="sym" style={{ fontSize: 12.5 }}>{p.src}</td>
                          <td className="c"><span className={`algo algo--${p.algoCls}`}>{p.algoShort}</span></td>
                          <td className="r num" style={{ color: 'var(--text-2)', fontSize: 12 }}>{p.isOz ? '$' + p.cur : fmt.goldShort(p.cur)}</td>
                          <td className="r num" style={{ fontWeight: 600, fontSize: 12 }}>{p.isOz ? '$' + p.pred : fmt.goldShort(p.pred)}</td>
                          <td className="r"><Chg pct={p.deltaPct} /></td>
                          <td className="r">
                            {(() => {
                              let acc = p.accuracy != null ? num(p.accuracy) : null;
                              if (acc != null && acc > 0 && acc <= 1) acc = Math.round(acc * 100);
                              if (acc != null) {
                                const color = acc >= 60 ? 'var(--up)' : acc >= 50 ? 'oklch(0.78 0.18 80)' : 'var(--dn)';
                                return <span style={{ color, fontWeight: 600, fontSize: 12 }}>{acc}%</span>;
                              }
                              return <span style={{ color: 'var(--text-3)' }}>—</span>;
                            })()}
                          </td>
                          <td className="r"><ConfBar v={p.conf} /></td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
                {predPageCount > 1 && (
                  <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', gap: 8, padding: '10px 16px', borderTop: '1px solid var(--border)', fontSize: 12, color: 'var(--text-2)' }}>
                    <button className="btn btn--sm" disabled={predPage === 0} onClick={() => setPredPage((p) => p - 1)} style={{ minWidth: 28, padding: '2px 8px' }}>‹</button>
                    <span>{predPage + 1} / {predPageCount}</span>
                    <button className="btn btn--sm" disabled={predPage >= predPageCount - 1} onClick={() => setPredPage((p) => p + 1)} style={{ minWidth: 28, padding: '2px 8px' }}>›</button>
                  </div>
                )}
              </div>
          }
        </Panel>

        <Panel title={t.common.latestPredResults} flush tools={
          <div className="chips" style={{ margin: 0 }}>
            <button className={`chip${confirmedGoldFilter === '' ? ' active' : ''}`} style={{ fontSize: 12, padding: '3px 10px' }} onClick={() => { setConfirmedGoldFilter(''); setConfirmedPage(0); }}>{t.common.all}</button>
            <button className={`chip${confirmedGoldFilter === 'xau' ? ' active' : ''}`} style={{ fontSize: 12, padding: '3px 10px' }} onClick={() => { setConfirmedGoldFilter('xau'); setConfirmedPage(0); }}>XAU/USD</button>
            <button className={`chip${confirmedGoldFilter === 'sjc' ? ' active' : ''}`} style={{ fontSize: 12, padding: '3px 10px' }} onClick={() => { setConfirmedGoldFilter('sjc'); setConfirmedPage(0); }}>SJC</button>
          </div>
        }>
          {filteredConfirmed.length === 0 ? (
            <div className="empty">
              <div className="empty__icon"><Icon name="pulse" size={18} /></div>
              <p>{confirmedResults.length === 0 ? t.common.noConfirmedResults : t.gold.noConfirmedForType}</p>
            </div>
          ) : (
            <div style={{ overflowX: 'auto' }}>
              <table className="tbl">
                <thead>
                  <tr>
                    <th>{t.common.source}</th>
                    <th className="c">TT</th>
                    <th className="r">{t.common.predicted}</th>
                    <th className="r">{t.common.actual}</th>
                    <th className="r">{t.common.deviation}</th>
                    <th className="r">{t.common.accuracy}</th>
                    <th className="c">{t.common.date}</th>
                  </tr>
                </thead>
                <tbody>
                  {confirmedPagedItems.map((r, i) => {
                    const isOzRow = (r.source || r.symbol || '').toLowerCase().includes('xau') ||
                      (r.product_type || '').toLowerCase().includes('xau');
                    const predicted = num(r.predicted_price);
                    const actual = num(r.actual_price ?? 0);
                    const fmtPrice = (v: number) => isOzRow ? '$' + v.toFixed(0) : fmt.vnd(Math.round(v));
                    const deviationPct = actual ? +((predicted - actual) / actual * 100).toFixed(2) : 0;
                    const absDeviation = Math.abs(deviationPct);
                    const deviationColor = absDeviation < 3
                      ? 'var(--up)'
                      : absDeviation < 5
                        ? 'oklch(0.78 0.18 80)'
                        : 'var(--dn)';
                    let acc = r.accuracy != null ? num(r.accuracy) : null;
                    if (acc != null && acc > 0 && acc <= 1) acc = Math.round(acc * 100);
                    const accColor = acc == null ? 'var(--text-3)'
                      : acc >= 60 ? 'var(--up)'
                      : acc >= 50 ? 'oklch(0.78 0.18 80)'
                      : 'var(--dn)';
                    const { short, cls } = algoShort(r.algorithm_name);
                    const label = r.source || r.symbol || r.product_type || '—';
                    return (
                      <tr key={i}>
                        <td className="sym" style={{ fontSize: 12.5 }}>{label}</td>
                        <td className="c"><span className={`algo algo--${cls}`}>{short}</span></td>
                        <td className="r num" style={{ fontWeight: 600, fontSize: 12 }}>{fmtPrice(predicted)}</td>
                        <td className="r num" style={{ color: 'var(--text-2)', fontSize: 12 }}>
                          {r.actual_price != null ? fmtPrice(actual) : <span style={{ color: 'var(--text-3)' }}>—</span>}
                        </td>
                        <td className="r num" style={{ color: deviationColor, fontSize: 12, fontWeight: 500 }}>
                          {r.actual_price != null
                            ? (deviationPct >= 0 ? '+' : '') + deviationPct.toFixed(2) + '%'
                            : <span style={{ color: 'var(--text-3)' }}>—</span>}
                        </td>
                        <td className="r">
                          {acc != null
                            ? <span style={{ color: accColor, fontWeight: 600, fontSize: 12 }}>{acc}%</span>
                            : <span style={{ color: 'var(--text-3)' }}>—</span>}
                        </td>
                        <td className="c num" style={{ color: 'var(--text-3)', fontSize: 12 }}>
                          {fmtDT(r.prediction_date)}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
              {confirmedPageCount > 1 && (
                <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', gap: 8, padding: '10px 16px', borderTop: '1px solid var(--border)', fontSize: 12, color: 'var(--text-2)' }}>
                  <button className="btn btn--sm" disabled={confirmedPage === 0} onClick={() => setConfirmedPage((p) => p - 1)} style={{ minWidth: 28, padding: '2px 8px' }}>‹</button>
                  <span>{confirmedPage + 1} / {confirmedPageCount}</span>
                  <button className="btn btn--sm" disabled={confirmedPage >= confirmedPageCount - 1} onClick={() => setConfirmedPage((p) => p + 1)} style={{ minWidth: 28, padding: '2px 8px' }}>›</button>
                </div>
              )}
            </div>
          )}
        </Panel>
      </div>

      {/* Prediction vs Actual chart — full width */}
      <Panel title={t.common.predVsActual} sub={t.gold.subEnsemble} className="section-gap"
        tools={
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center' }}>
            <Seg options={[{ value: '1', label: t.dateRange.today }, { value: '7', label: t.dateRange.d7 }, { value: '30', label: t.dateRange.d30 }, { value: '90', label: t.dateRange.d90 }, { value: '180', label: t.dateRange.d180 }]} value={days} onChange={setDays} />
            {/* Algorithm filter — from confirmed results */}
            {Array.from(new Set(confirmedResults.map((r) => r.algorithm_name))).sort().length > 0 && (
              <select
                value={predChartAlgo}
                onChange={(e) => setPredChartAlgo(e.target.value)}
                style={{ fontSize: 12, padding: '3px 8px', borderRadius: 6, border: '1px solid var(--border)', background: 'var(--surface-2)', color: 'var(--text-1)', cursor: 'pointer' }}
              >
                <option value="">{t.common.allAlgos}</option>
                {Array.from(new Set(confirmedResults.map((r) => r.algorithm_name))).sort().map((algo) => (
                  <option key={algo} value={algo}>{algoShort(algo).short} ({algo})</option>
                ))}
              </select>
            )}
          </div>
        }
      >
        {D.goldPredActual && D.goldPredActual.labels.length > 0
          ? <>
              <LineChart
                series={[
                  { name: t.common.actual, data: D.goldPredActual.actual, color: 'var(--text-2)', w: 1.8 },
                  { name: t.common.predicted, data: D.goldPredActual.pred, color: 'var(--gold)', dash: '5 4', w: 2 },
                ]}
                labels={D.goldPredActual.labels}
                height={680} yFmt={(v) => (v / 1e6).toFixed(1) + 'tr'} valueFmt={(v) => fmt.vnd(Math.round(v))} padL={50}
                highlightable
              />
              <Legend items={[[t.common.actual, 'var(--text-2)'], [t.common.predicted, 'var(--gold)']]} />
            </>
          : <div className="empty" style={{ height: 680, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
              <div className="empty__icon"><Icon name="layers" size={18} /></div>
              <p>{t.gold.noCompareData}</p>
            </div>
        }
      </Panel>

      <Panel title={t.gold.sjcDetail} sub={t.gold.last10Days} flush style={{ paddingBottom: 8 }}
        tools={<button className="btn btn--sm btn--ghost"><Icon name="refresh" size={13} />{t.common.refresh}</button>}>
        {D.goldDetail.length === 0
          ? <div className="empty">
              <div className="empty__icon"><Icon name="layers" size={18} /></div>
              <p>{t.gold.noHistData}</p>
            </div>
          : <div style={{ overflowX: 'auto' }}>
              <table className="tbl">
                <thead><tr><th>{t.gold.colDate}</th><th className="r">{t.gold.colBuyVnd}</th><th className="r">{t.gold.colSellVnd}</th><th className="r">{t.gold.colSpreadLabel}</th></tr></thead>
                <tbody>
                  {D.goldDetail.map((r, i) => (
                    <tr key={i}>
                      <td className="num" style={{ color: 'var(--text-2)' }}>{fmtDT(r.date)}</td>
                      <td className="r num">{fmt.vnd(r.buy)}</td>
                      <td className="r num" style={{ fontWeight: 600 }}>{fmt.vnd(r.sell)}</td>
                      <td className="r num" style={{ color: 'var(--text-3)' }}>{fmt.vnd(r.spread)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
        }
      </Panel>
    </div>
  );
}
