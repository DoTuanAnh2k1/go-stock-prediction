import { useState } from 'react';
import { useData } from '../context/DataContext';
import { Panel, KPI, Icon, Chg, Seg } from '../components/ui';
import { Sparkline, LineChart } from '../components/charts';
import type { StockItem, MoverItem } from '../types';

export default function Stocks() {
  const { data: D } = useData();
  const { fmt } = D;
  const [q, setQ] = useState('');
  const [sector, setSector] = useState('');
  const [sortBy, setSortBy] = useState<keyof StockItem>('chgPct');
  const [dir, setDir] = useState(-1);
  const [sel, setSel] = useState<StockItem | null>(null);

  const sectors = [...new Set(D.stocks.map((s) => s.sector))];

  let rows = D.stocks.filter((s) =>
    (!q || s.sym.toLowerCase().includes(q.toLowerCase()) || s.name.toLowerCase().includes(q.toLowerCase())) &&
    (!sector || s.sector === sector)
  );
  rows = [...rows].sort((a, b) => {
    const av = a[sortBy] as any, bv = b[sortBy] as any;
    return (typeof av === 'string' ? av.localeCompare(bv) : av - bv) * dir;
  });

  function sortHead(key: keyof StockItem, label: string, cls = 'r') {
    return (
      <th className={`${cls} th-sort`} onClick={() => { if (sortBy === key) setDir(-dir); else { setSortBy(key); setDir(-1); } }}>
        {label} {sortBy === key && <span className="caret">{dir < 0 ? '▼' : '▲'}</span>}
      </th>
    );
  }

  const vn30Val = D.indices.vn30.val;

  return (
    <div className="content__inner fade">
      <div className="grid grid--kpis section-gap">
        <KPI label="VN30-Index" value={vn30Val ? fmt.price(vn30Val) : '—'} chgPct={D.indices.vn30.chgPct} chgAbs={D.indices.vn30.chg} spark={D.indices.vn30.series.slice(-22)} />
        <KPI label="Số mã" value={D.stocks.length ? D.stocks.length + ' mã' : '—'} sub="đã tải" />
        <KPI label="Tăng / Giảm" value={D.stocks.length ? D.stocks.filter((s) => s.chgPct > 0).length + ' / ' + D.stocks.filter((s) => s.chgPct < 0).length : '— / —'} sub="trên tổng số mã" />
        <KPI label="Thanh khoản" value={D.indices.vnindex.vol ? fmt.compact(D.indices.vnindex.vol * 1e6) : '—'} sub="cổ phiếu khớp lệnh" />
      </div>

      <Panel className="section-gap">
        <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'center' }}>
          <div className="search" style={{ width: 280 }}>
            <Icon name="search" size={15} />
            <input placeholder="Tìm mã CK (VD: VCB, FPT...)" value={q} onChange={(e) => setQ(e.target.value)} />
          </div>
          <select className="sel" value={sector} onChange={(e) => setSector(e.target.value)}>
            <option value="">Tất cả ngành</option>
            {sectors.map((s) => <option key={s} value={s}>{s}</option>)}
          </select>
          <select className="sel" defaultValue="">
            <option value="">Tất cả sàn</option>
            <option>HOSE</option><option>HNX</option><option>UPCOM</option>
          </select>
          <span style={{ marginLeft: 'auto', fontSize: 12, color: 'var(--text-3)', fontFamily: 'var(--font-mono)' }}>{rows.length} mã</span>
        </div>
      </Panel>

      <Panel title="Bảng giá cổ phiếu" sub="cập nhật cuối 15:00" flush className="section-gap"
        tools={<button className="btn btn--sm btn--ghost"><Icon name="refresh" size={13} />Làm mới</button>}>
        {rows.length === 0
          ? <div className="empty">
              <div className="empty__icon"><Icon name="layers" size={18} /></div>
              <p>Chưa có dữ liệu cổ phiếu. Bấm crawl để lấy data.</p>
            </div>
          : <div style={{ overflowX: 'auto' }}>
              <table className="tbl">
                <thead><tr>
                  {sortHead('sym', 'Mã', 'l')}
                  <th>Ngành</th>
                  {sortHead('price', 'Giá')}
                  {sortHead('change', 'Δ')}
                  {sortHead('chgPct', '±%')}
                  {sortHead('volume', 'KL (M)')}
                  {sortHead('value', 'GT (ngàn tỷ)')}
                  <th className="c">7 phiên</th>
                  <th className="c"></th>
                </tr></thead>
                <tbody>
                  {rows.map((s) => (
                    <tr key={s.sym} className="clickable" onClick={() => setSel(s)}>
                      <td><div className="sym">{s.sym}</div><div className="co">{s.name}</div></td>
                      <td style={{ color: 'var(--text-3)', fontSize: 12 }}>{s.sector}</td>
                      <td className="r num">{fmt.price(s.price)}</td>
                      <td className="r num" style={{ color: s.change > 0 ? 'var(--up)' : s.change < 0 ? 'var(--down)' : 'var(--text-3)' }}>{fmt.sign(s.change)}</td>
                      <td className="r"><Chg pct={s.chgPct} badge /></td>
                      <td className="r num" style={{ color: 'var(--text-2)' }}>{s.volume.toFixed(1)}</td>
                      <td className="r num" style={{ color: 'var(--text-2)' }}>{s.value.toFixed(2)}</td>
                      <td className="c"><div style={{ display: 'flex', justifyContent: 'center' }}><Sparkline data={s.spark} w={86} h={26} fill={false} /></div></td>
                      <td className="c"><Icon name="caretDown" size={14} style={{ transform: 'rotate(-90deg)', color: 'var(--text-3)' }} /></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
        }
      </Panel>

      <div className="grid grid--halves section-gap">
        <MoverPanel title="Tăng mạnh nhất" icon="arrowUp" items={D.gainers} direction="up" fmt={fmt} />
        <MoverPanel title="Giảm mạnh nhất" icon="arrowDown" items={D.losers} direction="down" fmt={fmt} />
      </div>

      {D.active.length > 0 && (
        <Panel title="Giao dịch nhiều nhất" sub="theo khối lượng" flush style={{ paddingBottom: 8 }}>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)' }}>
            {D.active.map((s, i) => (
              <div key={s.sym} className="lrow clickable" style={{ borderRight: i % 3 !== 2 ? '1px solid var(--border)' : 'none', cursor: 'pointer' }} onClick={() => setSel(s as any)}>
                <span className="badge badge--muted" style={{ minWidth: 22, justifyContent: 'center' }}>{i + 1}</span>
                <div className="lrow__main">
                  <div className="lrow__sym">{s.sym}</div>
                  <div className="lrow__sub">{s.volume.toFixed(1)}M cp</div>
                </div>
                <div className="lrow__rt">
                  <div className="num" style={{ fontWeight: 600 }}>{fmt.price(s.price)}</div>
                  <div style={{ fontSize: 11 }}><Chg pct={s.chgPct} /></div>
                </div>
              </div>
            ))}
          </div>
        </Panel>
      )}

      {sel && <StockDrawer s={sel} onClose={() => setSel(null)} predictions={D.predictions} fmt={fmt} />}
    </div>
  );
}

function MoverPanel({ title, icon, items, direction, fmt }: { title: string; icon: string; items: MoverItem[]; direction: 'up' | 'down'; fmt: any }) {
  return (
    <Panel title={title} flush
      tools={<Icon name={icon} size={15} style={{ color: direction === 'up' ? 'var(--up)' : 'var(--down)' }} />}>
      {items.length === 0
        ? <div className="empty">
            <div className="empty__icon"><Icon name="layers" size={18} /></div>
            <p>Chưa có dữ liệu</p>
          </div>
        : items.map((s) => (
          <div key={s.sym} className="lrow">
            <div className="lrow__main">
              <div className="lrow__sym">{s.sym}</div>
              <div className="lrow__sub">{s.name}</div>
            </div>
            <Sparkline data={s.spark} w={70} h={24} fill={false} color={direction === 'up' ? 'var(--up)' : 'var(--down)'} />
            <div className="lrow__rt" style={{ minWidth: 88 }}>
              <div className="num" style={{ fontWeight: 600 }}>{fmt.price(s.price)}</div>
              <div style={{ fontSize: 11.5 }}><Chg pct={s.chgPct} /></div>
            </div>
          </div>
        ))
      }
    </Panel>
  );
}

function StockDrawer({ s, onClose, predictions, fmt }: { s: StockItem; onClose: () => void; predictions: any[]; fmt: any }) {
  const [range, setRange] = useState('30');
  const n = range === '7' ? 7 : 30;
  const hist = (s.hist || []).slice(-n);
  const pred = predictions.find((p) => p.sym === s.sym);
  return (
    <div style={{ position: 'fixed', inset: 0, zIndex: 40, display: 'flex', justifyContent: 'flex-end', background: 'oklch(0 0 0 / 0.5)' }} onClick={onClose}>
      <div className="fade" style={{ width: 560, maxWidth: '92vw', background: 'var(--bg-2)', borderLeft: '1px solid var(--border-strong)', height: '100%', overflowY: 'auto' }} onClick={(e) => e.stopPropagation()}>
        <div className="panel__head" style={{ position: 'sticky', top: 0, background: 'var(--bg-2)', zIndex: 2 }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              <span style={{ fontSize: 19, fontWeight: 700 }}>{s.sym}</span>
              <span className="badge badge--neutral">{s.exchange}</span>
              {s.vn30 && <span className="badge badge--muted">VN30</span>}
            </div>
            <div style={{ fontSize: 12, color: 'var(--text-3)', marginTop: 2 }}>{s.name} · {s.sector}</div>
          </div>
          <button className="btn btn--icon btn--ghost" style={{ marginLeft: 'auto' }} onClick={onClose}>✕</button>
        </div>
        <div style={{ padding: 18 }}>
          <div style={{ display: 'flex', alignItems: 'baseline', gap: 12, marginBottom: 14 }}>
            <span className="num" style={{ fontSize: 34, fontWeight: 600, letterSpacing: '-1.5px' }}>{fmt.price(s.price)}</span>
            <div><Chg pct={s.chgPct} abs={s.change} /></div>
          </div>
          <Seg options={[{ value: '7', label: '7 phiên' }, { value: '30', label: '30 phiên' }]} value={range} onChange={setRange} />
          <div style={{ marginTop: 12 }}>
            {hist.length > 0
              ? <LineChart series={[{ name: s.sym, data: hist, color: s.chgPct >= 0 ? 'var(--up)' : 'var(--down)' }]} labels={hist.map((_, i) => i % 5 === 0 ? `${i + 1}` : '')} height={220} area valueFmt={(v) => fmt.price(v)} />
              : <div className="empty" style={{ height: 220, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
                  <div className="empty__icon"><Icon name="layers" size={18} /></div>
                  <p>Chưa có lịch sử giá</p>
                </div>
            }
          </div>
          <div className="grid" style={{ gridTemplateColumns: '1fr 1fr', gap: 1, background: 'var(--border)', border: '1px solid var(--border)', marginTop: 16 }}>
            {[['Khối lượng', s.volume.toFixed(1) + 'M'], ['Giá trị', s.value.toFixed(2) + ' ngàn tỷ']].map(([l, v]) => (
              <div key={l} style={{ background: 'var(--surface)', padding: '12px 14px' }}>
                <div style={{ fontSize: 11, color: 'var(--text-3)', textTransform: 'uppercase', letterSpacing: 0.4 }}>{l}</div>
                <div className="num" style={{ fontSize: 16, fontWeight: 600, marginTop: 4 }}>{v}</div>
              </div>
            ))}
          </div>
          {pred && (
            <div style={{ marginTop: 16 }}>
              <div className="sec-head" style={{ margin: '0 0 10px' }}><h2>Dự đoán phiên kế</h2><div className="line"></div></div>
              <div className="panel" style={{ padding: 16 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
                  <span className={`algo algo--${pred.algoCls}`}>{pred.algoShort}</span>
                  <div>
                    <div style={{ fontSize: 11, color: 'var(--text-3)' }}>Giá dự đoán {pred.target}</div>
                    <div className="num" style={{ fontSize: 20, fontWeight: 600 }}>{fmt.price(pred.pred)}</div>
                  </div>
                  <div style={{ marginLeft: 'auto', textAlign: 'right' }}>
                    <Chg pct={pred.deltaPct} />
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
