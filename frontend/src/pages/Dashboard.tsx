import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useData } from '../context/DataContext';
import { Panel, KPI, Seg, Icon, Chg, ConfBar } from '../components/ui';
import { Sparkline, LineChart, BarChart, HBars, Donut } from '../components/charts';

export default function Dashboard() {
  const { data: D } = useData();
  const { fmt } = D;
  const [idx, setIdx] = useState('vnindex');
  const index = D.indices[idx as 'vnindex' | 'vn30'];
  const watch = D.stocks.slice(0, 8);
  const ens = (D.algos || []).find((a) => a.cls === 'ens');
  const ensAcc = ens ? ens.acc : (D.stats && D.stats.acc ? D.stats.acc : 0);
  const ensAccDisplay = ensAcc ? (+ensAcc).toFixed(1) + '%' : '—%';

  const indexVal = index.val;
  const indexDisplay = indexVal ? fmt.price(indexVal) : '—';
  const vnindexVal = D.indices.vnindex.val;
  const vnindexDisplay = vnindexVal ? fmt.price(vnindexVal) : '—';
  const vn30Val = D.indices.vn30.val;
  const vn30Display = vn30Val ? fmt.price(vn30Val) : '—';
  const volSource = D.indices.vn30.vol || D.indices.vnindex.vol;
  const volDisplay = volSource ? fmt.compact(volSource * 1e6) : '—';

  return (
    <div className="content__inner fade">
      <div className="grid grid--kpis section-gap">
        <KPI label="VN-Index" value={vnindexDisplay} chgPct={D.indices.vnindex.chgPct} chgAbs={D.indices.vnindex.chg} spark={D.indices.vnindex.series.slice(-22)} />
        <KPI label="VN30" value={vn30Display} chgPct={D.indices.vn30.chgPct} chgAbs={D.indices.vn30.chg} spark={D.indices.vn30.series.slice(-22)} />
        <KPI label="Thanh khoản" value={volDisplay} sub="cổ phiếu khớp lệnh" />
        <KPI label="Độ chính xác ENS" value={ensAccDisplay} sub="mô hình tổng hợp" chgPct={ens ? ens.accDelta : 0} accent />
      </div>

      <div className="grid grid--main section-gap">
        <Panel
          title="Diễn biến chỉ số"
          dot={idx === 'vnindex' ? 'VN-Index' : 'VN30'}
          tools={<Seg options={[{ value: 'vnindex', label: 'VN-Index' }, { value: 'vn30', label: 'VN30' }]} value={idx} onChange={setIdx} />}
        >
          <div style={{ display: 'flex', alignItems: 'baseline', gap: 12, marginBottom: 8 }}>
            <span className="num" style={{ fontSize: 28, fontWeight: 600, letterSpacing: '-1px' }}>{indexDisplay}</span>
            {indexVal ? <Chg pct={index.chgPct} abs={index.chg} /> : null}
            <span style={{ marginLeft: 'auto', fontSize: 11, color: 'var(--text-3)', fontFamily: 'var(--font-mono)' }}>Phiên hôm nay · đóng cửa</span>
          </div>
          {index.series.length > 0
            ? <LineChart
                series={[{ name: idx === 'vnindex' ? 'VN-Index' : 'VN30', data: index.series, color: 'var(--accent)' }]}
                labels={index.series.map((_, i) => `${9 + Math.floor(i / 7)}h`)}
                height={580} area yFmt={(v) => v.toFixed(0)} valueFmt={(v) => fmt.price(v)} padL={52}
              />
            : <div className="empty" style={{ height: 580, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
                <div className="empty__icon"><Icon name="layers" size={18} /></div>
                <p>Chưa có dữ liệu chỉ số</p>
              </div>
          }
        </Panel>
        <Panel title="Độ rộng thị trường" sub="VN30">
          <Breadth stocks={D.stocks} fmt={fmt} />
        </Panel>
      </div>

      <div className="grid grid--side section-gap">
        <Panel
          title="Danh sách theo dõi"
          tools={<button className="btn btn--sm btn--ghost"><Icon name="refresh" size={13} />Làm mới</button>}
          flush
        >
          {watch.length === 0
            ? <div className="empty">
                <div className="empty__icon"><Icon name="layers" size={18} /></div>
                <p>Chưa có dữ liệu. Đang tải...</p>
              </div>
            : <div style={{ overflowX: 'auto' }}>
                <table className="tbl">
                  <thead><tr><th>Mã</th><th className="r">Giá</th><th className="r">±%</th><th className="r">KL (M)</th><th className="c" style={{ width: 110 }}>7 phiên</th></tr></thead>
                  <tbody>
                    {watch.map((s) => (
                      <tr key={s.sym} className="clickable">
                        <td><div className="sym">{s.sym}</div><div className="co">{s.name}</div></td>
                        <td className="r num">{fmt.price(s.price)}</td>
                        <td className="r"><Chg pct={s.chgPct} /></td>
                        <td className="r num" style={{ color: 'var(--text-2)' }}>{s.volume.toFixed(1)}</td>
                        <td className="c"><div style={{ display: 'flex', justifyContent: 'center' }}><Sparkline data={s.spark} w={92} h={28} fill={false} /></div></td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
          }
        </Panel>

        <Panel title="Trạng thái mô hình ML" sub={D.algos.length ? D.algos.length + ' thuật toán' : '—'} flush>
          {D.algos.length === 0
            ? <div className="empty">
                <div className="empty__icon"><Icon name="layers" size={18} /></div>
                <p>Chưa có dữ liệu huấn luyện</p>
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
      </div>

      <div className="sec-head"><h2>Dự đoán mới nhất</h2><div className="line"></div></div>
      <Panel flush className="section-gap">
        {D.predictions.length === 0
          ? <div className="empty">
              <div className="empty__icon"><Icon name="layers" size={18} /></div>
              <p>Chưa có dự đoán. Đang tải...</p>
            </div>
          : <div style={{ overflowX: 'auto' }}>
              <table className="tbl">
                <thead><tr>
                  <th>Mã</th><th className="r">Giá hiện tại</th><th className="r">Giá dự đoán</th><th className="r">Δ dự kiến</th>
                  <th className="c">Thuật toán</th><th className="r">Độ tin cậy</th><th>Phiên mục tiêu</th>
                </tr></thead>
                <tbody>
                  {D.predictions.slice(0, 8).map((p, i) => (
                    <tr key={i} className="clickable">
                      <td><div className="sym">{p.sym}</div><div className="co">{p.name}</div></td>
                      <td className="r num" style={{ color: 'var(--text-2)' }}>{fmt.price(p.cur)}</td>
                      <td className="r num" style={{ fontWeight: 600 }}>{fmt.price(p.pred)}</td>
                      <td className="r"><Chg pct={p.deltaPct} /></td>
                      <td className="c"><span className={`algo algo--${p.algoCls}`}>{p.algoShort}</span></td>
                      <td className="r"><ConfBar v={p.conf} /></td>
                      <td className="num" style={{ color: 'var(--text-2)', fontSize: 12 }}>{p.target}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
        }
      </Panel>

      <div className="grid grid--halves" style={{ paddingBottom: 8 }}>
        <Panel title="So sánh độ chính xác thuật toán" sub="30 phiên">
          {D.algos.length === 0
            ? <div className="empty">
                <div className="empty__icon"><Icon name="layers" size={18} /></div>
                <p>Chưa có dữ liệu huấn luyện</p>
              </div>
            : <HBars items={D.algos.map((a) => ({
                label: a.name, value: a.acc,
                color: a.cls === 'ens' ? 'oklch(0.72 0.14 300)' : a.cls === 'lstm' ? 'oklch(0.74 0.13 200)' : a.cls === 'arima' ? 'var(--gold)' : 'var(--up)',
              }))} />
          }
        </Panel>
        <Panel title="Số dự đoán theo ngày" sub="7 ngày">
          {D.dailyCounts.values.length === 0
            ? <div className="empty">
                <div className="empty__icon"><Icon name="layers" size={18} /></div>
                <p>Chưa có dữ liệu</p>
              </div>
            : <BarChart data={D.dailyCounts.values} labels={D.dailyCounts.labels} height={500} color="var(--accent)" valueFmt={(v) => v + ' dự đoán'} />
          }
        </Panel>
      </div>

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
      title="Top Simulation Bots"
      sub="3 bots hiệu suất cao nhất"
      className="section-gap"
      tools={
        <Link to="/simulation" style={{ fontSize: 12, color: 'var(--accent)', textDecoration: 'none', display: 'flex', alignItems: 'center', gap: 4 }}>
          Xem tất cả →
        </Link>
      }
      style={{ paddingBottom: 4 }}
    >
      {bots.length === 0 ? (
        <div className="empty" style={{ padding: '24px 0' }}>
          <div className="empty__icon"><Icon name="layers" size={16} /></div>
          <p style={{ fontSize: 12 }}>Chưa có dữ liệu simulation. <Link to="/simulation" style={{ color: 'var(--accent)' }}>Chạy backtest</Link></p>
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

function Breadth({ stocks, fmt }: { stocks: any[]; fmt: any }) {
  if (!stocks.length) {
    return (
      <div className="empty">
        <div className="empty__icon"><Icon name="layers" size={18} /></div>
        <p>Chưa có dữ liệu thị trường</p>
      </div>
    );
  }
  const up   = stocks.filter((s) => s.chgPct > 0).length;
  const down = stocks.filter((s) => s.chgPct < 0).length;
  const flat = stocks.length - up - down;
  const total = stocks.length || 1;
  const sectors = [...new Set(stocks.map((s) => s.sector))].map((name) => {
    const items = stocks.filter((s) => s.sector === name);
    const avg = items.reduce((a: number, s: any) => a + s.chgPct, 0) / items.length;
    return { name, avg };
  }).sort((a, b) => b.avg - a.avg);
  return (
    <div>
      <div style={{ display: 'flex', gap: 16, alignItems: 'center', marginBottom: 14 }}>
        <Donut value={up / total * 100} color="var(--up)" label="tăng giá" />
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 9 }}>
          <StatRow dot="var(--up)"   label="Tăng"        value={up} />
          <StatRow dot="var(--down)" label="Giảm"        value={down} />
          <StatRow dot="var(--flat)" label="Tham chiếu"  value={flat} />
        </div>
      </div>
      <div className="bar" style={{ height: 8, display: 'flex' }}>
        <div style={{ width: up   / total * 100 + '%', background: 'var(--up)' }}></div>
        <div style={{ width: flat / total * 100 + '%', background: 'var(--flat)' }}></div>
        <div style={{ width: down / total * 100 + '%', background: 'var(--down)' }}></div>
      </div>
      <div style={{ fontSize: 11, color: 'var(--text-3)', textTransform: 'uppercase', letterSpacing: 0.5, fontWeight: 600, margin: '16px 0 8px' }}>Ngành dẫn dắt</div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 7 }}>
        {sectors.slice(0, 5).map((s) => (
          <div key={s.name} style={{ display: 'flex', alignItems: 'center', fontSize: 12.5 }}>
            <span style={{ color: 'var(--text-2)' }}>{s.name}</span>
            <span className="num" style={{ marginLeft: 'auto', color: s.avg > 0 ? 'var(--up)' : 'var(--down)' }}>{fmt.pct(s.avg)}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function StatRow({ dot, label, value }: { dot: string; label: string; value: number }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13 }}>
      <span style={{ width: 8, height: 8, background: dot }}></span>
      <span style={{ color: 'var(--text-2)' }}>{label}</span>
      <span className="num" style={{ marginLeft: 'auto', fontWeight: 600 }}>{value}</span>
    </div>
  );
}
