import { useState, useEffect } from 'react';
import { useData } from '../context/DataContext';
import { Panel, KPI, Icon, Chg, Seg, ConfBar } from '../components/ui';
import { LineChart, HBars, Scatter } from '../components/charts';

export default function Predictions() {
  const { data: D } = useData();
  const { fmt } = D;
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
  const [compareData, setCompareData] = useState<{ labels: string[]; actual: (number | null)[]; pred: (number | null)[] } | null>(null);
  const [compareLoading, setCompareLoading] = useState(false);

  useEffect(() => {
    if (!effectiveSym) { setCompareData(null); return; }
    setCompareLoading(true);
    const url = '/api/predictions/compare/' + encodeURIComponent(effectiveSym) + '?days=90' + (algo ? '&algorithm=' + encodeURIComponent(algo) : '');
    fetch(url, { headers: { Accept: 'application/json' } })
      .then(function(r) { if (!r.ok) throw new Error('HTTP ' + r.status); return r.json(); })
      .then(function(res: any) {
        const raw: any[] = Array.isArray(res.data) ? res.data : [];
        const ordered = raw.slice().reverse();
        const ddmm = function(s: string) {
          try {
            const d = new Date(s);
            if (isNaN(d.getTime())) return s.slice(0, 10);
            return ('0' + d.getDate()).slice(-2) + '/' + ('0' + (d.getMonth() + 1)).slice(-2);
          } catch (_e) { return s ? s.slice(0, 10) : ''; }
        };
        setCompareData({
          labels: ordered.map(function(it) { return ddmm(it.date || ''); }),
          actual: ordered.map(function(it) { const n = parseFloat(it.actual); return isFinite(n) ? n : null; }),
          pred:   ordered.map(function(it) { const n = parseFloat(it.predicted); return isFinite(n) ? n : null; }),
        });
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
        <KPI label="Tổng dự đoán"    value={summary.total || '—'}     sub="30 ngày qua" />
        <KPI label="Độ chính xác TB" value={summary.acc ? summary.acc + '%' : '—%'} sub="tất cả mô hình" accent />
        <KPI label="Đã xác nhận"     value={summary.confirmed || '—'} sub="có giá thực tế" />
        <KPI label="Sai số TB"       value={summary.err ? summary.err + '%' : '—%'} sub="|dự đoán − thực tế|" />
      </div>

      <Panel className="section-gap">
        <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'center' }}>
          <Icon name="filter" size={15} style={{ color: 'var(--text-3)' }} />
          <select className="sel" value={algo} onChange={(e) => setAlgo(e.target.value)}>
            <option value="">Tất cả thuật toán</option>
            {D.algos.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
          </select>
          <Seg options={[{ value: '7', label: '7N' }, { value: '30', label: '30N' }, { value: '90', label: '90N' }]} value={range} onChange={setRange} />
          <div style={{ marginLeft: 'auto', display: 'flex', gap: 8 }}>
            <button className="btn btn--sm"><Icon name="download" size={13} />Xuất CSV</button>
            <button className="btn btn--sm btn--ghost"><Icon name="refresh" size={13} />Làm mới</button>
          </div>
        </div>
      </Panel>

      <div className="grid grid--wide section-gap">
        <Panel title="Hiệu suất thuật toán" sub="độ chính xác">
          {D.algos.length === 0
            ? <div className="empty">
                <div className="empty__icon"><Icon name="layers" size={18} /></div>
                <p>Chưa có dữ liệu huấn luyện</p>
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
        <Panel title="Xu hướng độ chính xác" sub="phiên gần nhất">
          {D.accTrend.labels.length === 0
            ? <div className="empty">
                <div className="empty__icon"><Icon name="layers" size={18} /></div>
                <p>Chưa có dữ liệu xu hướng</p>
              </div>
            : <>
                <LineChart
                  series={D.accTrend.series.map((s) => ({ name: s.name, data: s.data, color: s.color }))}
                  labels={D.accTrend.labels} height={236} yFmt={(v) => v.toFixed(0) + '%'} valueFmt={(v) => v.toFixed(1) + '%'}
                />
                <Legend items={D.accTrend.series.map((s) => [s.name, s.color] as [string, string])} />
              </>
          }
        </Panel>
      </div>

      <div className="sec-head"><h2>Tất cả dự đoán</h2><div className="line"></div></div>
      <Panel flush className="section-gap">
        {conf.length === 0
          ? <div className="empty">
              <div className="empty__icon"><Icon name="layers" size={18} /></div>
              <p>Chưa có dự đoán đã xác nhận</p>
            </div>
          : <div style={{ overflowX: 'auto' }}>
              <table className="tbl">
                <thead><tr>
                  <th>Mã</th><th className="c">Thuật toán</th><th className="r">Giá dự đoán</th><th className="r">Giá thực tế</th>
                  <th className="r">Δ dự đoán</th><th className="r">Δ thực tế</th><th className="r">Độ tin cậy</th>
                  <th className="r">Độ chính xác</th><th className="c">Trạng thái</th>
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
                          {c.status === 'hit' ? 'Chính xác' : c.status === 'miss' ? 'Sai lệch' : 'Gần đúng'}
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
        <Panel title="Dự đoán vs Thực tế" dot={effectiveSym || '—'}
          tools={D.stocks.length > 0 &&
            <select className="sel" value={effectiveSym} onChange={(e) => setCompareSym(e.target.value)}>
              {D.stocks.map((s) => <option key={s.sym}>{s.sym}</option>)}
            </select>
          }>
          {compareLoading
            ? <div className="empty" style={{ height: 240, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
                <div className="empty__icon"><Icon name="layers" size={18} /></div>
                <p>Đang tải...</p>
              </div>
            : (!compareData || compareData.labels.length === 0)
              ? <div className="empty" style={{ height: 240, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
                  <div className="empty__icon"><Icon name="layers" size={18} /></div>
                  <p>Chưa có dữ liệu so sánh</p>
                </div>
              : <>
                  <LineChart
                    series={[
                      { name: 'Thực tế', data: compareData.actual, color: 'var(--text-2)', w: 1.8 },
                      { name: 'Dự đoán', data: compareData.pred, color: 'var(--accent)', dash: '5 4', w: 2 },
                    ]}
                    labels={compareData.labels} height={240} valueFmt={(v) => fmt.price(v)}
                  />
                  <Legend items={[['Thực tế', 'var(--text-2)'], ['Dự đoán', 'var(--accent)']]} />
                </>
          }
        </Panel>

        <Panel title="Phân bố sai số" sub="độ tin cậy × |sai số|">
          {scatterPts.length === 0
            ? <div className="empty" style={{ height: 220, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
                <div className="empty__icon"><Icon name="layers" size={18} /></div>
                <p>Chưa có dữ liệu phân tích</p>
              </div>
            : <>
                <Scatter points={scatterPts} height={220} xLabel="Độ tin cậy (%)" />
                <Legend items={D.accTrend.series.map((s) => [s.name, s.color] as [string, string])} />
              </>
          }
        </Panel>
      </div>

      {(best.length > 0 || worst.length > 0) && (
        <div className="grid grid--halves" style={{ paddingBottom: 8 }}>
          <FeaturedPanel title="Top 5 chính xác nhất" items={best} good fmt={fmt} />
          <FeaturedPanel title="Top 5 sai lệch nhất" items={worst} good={false} fmt={fmt} />
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
  return (
    <Panel title={title} flush>
      {items.length === 0
        ? <div className="empty">
            <div className="empty__icon"><Icon name="layers" size={18} /></div>
            <p>Chưa có dữ liệu</p>
          </div>
        : items.map((c, i) => (
          <div key={i} className="lrow">
            <span className="badge badge--muted" style={{ minWidth: 22, justifyContent: 'center' }}>{i + 1}</span>
            <div className="lrow__main">
              <div className="lrow__sym">{c.sym} <span className={`algo algo--${c.algoCls}`} style={{ marginLeft: 4 }}>{c.algoShort}</span></div>
              <div className="lrow__sub">DĐ {fmt.price(c.pred)} · TT {fmt.price(c.actual)}</div>
            </div>
            <div className="lrow__rt">
              <div className="num" style={{ fontWeight: 600, color: good ? 'var(--up)' : 'var(--down)' }}>{c.acc}%</div>
              <div style={{ fontSize: 11, color: 'var(--text-3)' }}>sai số {c.err}%</div>
            </div>
          </div>
        ))
      }
    </Panel>
  );
}
