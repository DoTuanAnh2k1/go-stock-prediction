import { useState } from 'react';
import { useData } from '../context/DataContext';
import { Panel, KPI, Icon, Chg, Seg, ConfBar } from '../components/ui';
import { Sparkline, LineChart } from '../components/charts';
import { crawlGold, predictGold } from '../api';
import { vnsToast } from '../components/ui';

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
  const srcs = D.goldSources || [];
  const [active, setActive] = useState<string | null>(null);
  const [days, setDays] = useState('180');

  const src = srcs.find((g) => g.id === active) || srcs[0] || null;
  const isOz = src ? src.unit === 'oz' : false;
  const n = days === '30' ? 30 : days === '90' ? 90 : 180;
  const hist = src ? (src.hist || []).slice(-n) : [];

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

  if (srcs.length === 0) {
    return (
      <div className="content__inner fade">
        <div className="grid grid--kpis section-gap">
          {[1, 2, 3, 4].map((i) => <KPI key={i} label="—" value="—" sub="đang tải..." />)}
        </div>
        <div className="empty section-gap">
          <div className="empty__icon"><Icon name="layers" size={18} /></div>
          <p>Đang tải dữ liệu vàng...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="content__inner fade">
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
        title="Biểu đồ giá vàng"
        dot={src ? src.name : '—'}
        className="section-gap"
        tools={
          <div style={{ display: 'flex', gap: 8 }}>
            <Seg options={[{ value: '30', label: '30N' }, { value: '90', label: '90N' }, { value: '180', label: '180N' }]} value={days} onChange={setDays} />
            <button className="btn btn--sm" style={{ background: 'var(--gold)', borderColor: 'var(--gold)', color: 'oklch(0.2 0.02 80)' }}
              onClick={() => crawlGold().then((ok) => vnsToast(ok ? 'Đã gửi yêu cầu thu thập giá vàng' : 'Chưa gọi được API thu thập'))}>
              <Icon name="download" size={13} />Thu thập
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
            <span style={{ fontSize: 12, color: 'var(--text-3)' }}>giá bán / {src.unit}</span>
            <Chg pct={src.chgPct} />
            <span style={{ marginLeft: 'auto', fontSize: 11, color: 'var(--text-3)', fontFamily: 'var(--font-mono)' }}>{src.vendor} · {src.region}</span>
          </div>
        )}
        {hist.length > 0
          ? <LineChart
              series={[{ name: src!.name, data: hist, color: 'var(--gold)' }]}
              labels={hist.map((_, i) => i % Math.ceil(n / 7) === 0 ? `${i + 1}` : '')}
              height={300} area yFmt={fmtGold} valueFmt={fmtFull} padL={58}
            />
          : <div className="empty" style={{ height: 300, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
              <div className="empty__icon"><Icon name="layers" size={18} /></div>
              <p>Chưa có lịch sử giá vàng. Bấm thu thập để lấy data.</p>
            </div>
        }
      </Panel>

      <div className="sec-head"><h2>So sánh giá các nhà cung cấp</h2><div className="line"></div></div>
      <Panel flush className="section-gap">
        <div style={{ overflowX: 'auto' }}>
          <table className="tbl">
            <thead><tr>
              <th>Nhà cung cấp</th><th>Loại</th><th className="c">Khu vực</th>
              <th className="r">Giá mua</th><th className="r">Giá bán</th><th className="r">Chênh lệch</th><th className="r">±%</th><th className="c" style={{ width: 110 }}>24 mốc</th>
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
        <Panel title="Dự đoán giá vàng phiên mai" flush
          tools={
            <button className="btn btn--sm" style={{ background: 'var(--gold)', borderColor: 'var(--gold)', color: 'oklch(0.2 0.02 80)' }}
              onClick={() => predictGold().then((ok) => vnsToast(ok ? 'Đã gửi yêu cầu chạy dự đoán vàng' : 'Chưa gọi được API dự đoán'))}>
              <Icon name="play" size={13} />Chạy dự đoán
            </button>
          }>
          {D.goldPreds.length === 0
            ? <div className="empty">
                <div className="empty__icon"><Icon name="layers" size={18} /></div>
                <p>Chưa có dự đoán giá vàng</p>
              </div>
            : <div style={{ overflowX: 'auto' }}>
                <table className="tbl">
                  <thead><tr><th>Nguồn</th><th className="c">TT</th><th className="r">Hiện tại</th><th className="r">Dự đoán</th><th className="r">±%</th><th className="r">Tin cậy</th></tr></thead>
                  <tbody>
                    {D.goldPreds.map((p, i) => {
                      const oz = p.src === 'XAU/USD';
                      return (
                        <tr key={i}>
                          <td className="sym" style={{ fontSize: 12.5 }}>{p.src}</td>
                          <td className="c"><span className={`algo algo--${p.algoCls}`}>{p.algoShort}</span></td>
                          <td className="r num" style={{ color: 'var(--text-2)', fontSize: 12 }}>{oz ? '$' + p.cur : fmt.goldShort(p.cur)}</td>
                          <td className="r num" style={{ fontWeight: 600, fontSize: 12 }}>{oz ? '$' + p.pred : fmt.goldShort(p.pred)}</td>
                          <td className="r"><Chg pct={p.deltaPct} /></td>
                          <td className="r"><ConfBar v={p.conf} /></td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
          }
        </Panel>

        <Panel title="Dự đoán vs Thực tế" sub="BTMC SJC · Ensemble">
          {D.goldPredActual && D.goldPredActual.labels.length > 0
            ? <>
                <LineChart
                  series={[
                    { name: 'Thực tế', data: D.goldPredActual.actual, color: 'var(--text-2)', w: 1.8 },
                    { name: 'Dự đoán', data: D.goldPredActual.pred, color: 'var(--gold)', dash: '5 4', w: 2 },
                  ]}
                  labels={D.goldPredActual.labels.map((l, i) => i % 5 === 0 ? l : '')}
                  height={236} yFmt={(v) => (v / 1e6).toFixed(1) + 'tr'} valueFmt={(v) => fmt.vnd(Math.round(v))} padL={50}
                />
                <Legend items={[['Thực tế', 'var(--text-2)'], ['Dự đoán', 'var(--gold)']]} />
              </>
            : <div className="empty" style={{ height: 236, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
                <div className="empty__icon"><Icon name="layers" size={18} /></div>
                <p>Chưa có dữ liệu so sánh dự đoán</p>
              </div>
          }
        </Panel>
      </div>

      <Panel title="Chi tiết SJC 1 Lượng" sub="10 ngày gần nhất" flush style={{ paddingBottom: 8 }}
        tools={<button className="btn btn--sm btn--ghost"><Icon name="refresh" size={13} />Làm mới</button>}>
        {D.goldDetail.length === 0
          ? <div className="empty">
              <div className="empty__icon"><Icon name="layers" size={18} /></div>
              <p>Chưa có dữ liệu lịch sử</p>
            </div>
          : <div style={{ overflowX: 'auto' }}>
              <table className="tbl">
                <thead><tr><th>Ngày</th><th className="r">Giá mua (VND)</th><th className="r">Giá bán (VND)</th><th className="r">Chênh lệch</th></tr></thead>
                <tbody>
                  {D.goldDetail.map((r, i) => (
                    <tr key={i}>
                      <td className="num" style={{ color: 'var(--text-2)' }}>{r.date}</td>
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
