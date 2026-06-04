import { useState, useEffect, useCallback } from 'react';
import { useData } from '../context/DataContext';
import { Panel, KPI, Icon, Chg, Seg, vnsToast } from '../components/ui';
import { Sparkline, LineChart } from '../components/charts';
import { fetchMarketPage } from '../api';
import { useAuth } from '../context/AuthContext';
import type { StockItem, MoverItem } from '../types';

function toStockItems(stocks: any[]): StockItem[] {
  return (stocks || []).map((s: any) => ({
    sym: s.symbol || '',
    name: s.company_name || s.symbol || '',
    sector: s.sector || '—',
    exchange: s.exchange || 'HOSE',
    price: parseFloat(s.current_price) || 0,
    change: parseFloat(s.change) || 0,
    chgPct: parseFloat(s.change_percent) || 0,
    volume: (parseFloat(s.volume) || 0) / 1e6,
    value: (parseFloat(s.value) || 0) / 1e12,
    vn30: s.is_vn30 !== false,
    spark: [],
    hist: [],
  }));
}

function getToken(): string {
  return localStorage.getItem('vns_token') || '';
}
async function authPost(path: string): Promise<boolean> {
  const res = await fetch(path, { method: 'POST', headers: { Authorization: `Bearer ${getToken()}` } });
  return res.ok;
}

export default function Stocks() {
  const { data: D } = useData();
  const { fmt } = D;
  const { isLoggedIn } = useAuth();
  const [q, setQ] = useState('');
  const [sector, setSector] = useState('');
  const [sortBy, setSortBy] = useState<'price' | 'change_percent'>('change_percent');
  const [sortDesc, setSortDesc] = useState(true);
  const [page, setPage] = useState(1);
  const [pagedStocks, setPagedStocks] = useState<StockItem[]>([]);
  const [meta, setMeta] = useState({ total: 0, page: 1, pageSize: 10, totalPages: 1 });
  const [loading, setLoading] = useState(false);
  const [sel, setSel] = useState<StockItem | null>(null);
  const [activeSym, setActiveSym] = useState<string | null>(null);
  const [chartDays, setChartDays] = useState('90');
  const [stockChart, setStockChart] = useState<{ dates: string[]; prices: number[] }>({ dates: [], prices: [] });
  const [chartLoading, setChartLoading] = useState(false);

  const sectors = [...new Set(D.stocks.map((s) => s.sector))].filter(Boolean);

  useEffect(() => {
    if (D.stocks.length > 0 && !activeSym) {
      setActiveSym(D.stocks[0].sym);
    }
  }, [D.stocks]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!activeSym) return;
    setChartLoading(true);
    fetch(`/api/stocks/${encodeURIComponent(activeSym)}/history?days=${chartDays}`)
      .then(r => r.json())
      .then(v => {
        const priceData: any[] = v?.price_data || [];
        const reversed = [...priceData].reverse();
        setStockChart({
          dates: reversed.map((p) => (p.trading_date || '').slice(0, 10)),
          prices: reversed.map((p) => parseFloat(p.close_price) || 0),
        });
      })
      .catch(() => setStockChart({ dates: [], prices: [] }))
      .finally(() => setChartLoading(false));
  }, [activeSym, chartDays]);

  const doFetch = useCallback((pg: number, sb: string, sd: boolean, searchQ: string, sec: string) => {
    setLoading(true);
    fetchMarketPage({
      page: pg,
      pageSize: 10,
      sortBy: sb as 'price' | 'change_percent',
      sortOrder: sd ? 'desc' : 'asc',
      q: searchQ,
      sector: sec,
    }).then((res: any) => {
      setPagedStocks(toStockItems(res.stocks || []));
      setMeta({
        total: res.stocks_total || 0,
        page: res.stocks_page || pg,
        pageSize: res.stocks_page_size || 10,
        totalPages: res.stocks_total_pages || 1,
      });
    }).catch(() => {}).finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    doFetch(1, 'change_percent', true, '', '');
  }, [doFetch]);

  useEffect(() => {
    const t = setTimeout(() => {
      setPage(1);
      doFetch(1, sortBy, sortDesc, q, sector);
    }, 300);
    return () => clearTimeout(t);
  }, [q]); // eslint-disable-line react-hooks/exhaustive-deps

  function handleSort(key: 'price' | 'change_percent') {
    const newDesc = sortBy === key ? !sortDesc : true;
    setSortBy(key);
    setSortDesc(newDesc);
    setPage(1);
    doFetch(1, key, newDesc, q, sector);
  }

  function handleSector(sec: string) {
    setSector(sec);
    setPage(1);
    doFetch(1, sortBy, sortDesc, q, sec);
  }

  function handlePage(p: number) {
    setPage(p);
    doFetch(p, sortBy, sortDesc, q, sector);
  }

  function sortHead(key: 'price' | 'change_percent', label: string) {
    return (
      <th className="r th-sort" onClick={() => handleSort(key)}>
        {label}{' '}
        {sortBy === key && <span className="caret">{sortDesc ? '▼' : '▲'}</span>}
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
          <select className="sel" value={sector} onChange={(e) => handleSector(e.target.value)}>
            <option value="">Tất cả ngành</option>
            {sectors.map((s) => <option key={s} value={s}>{s}</option>)}
          </select>
          <select className="sel" defaultValue="">
            <option value="">Tất cả sàn</option>
            <option>HOSE</option><option>HNX</option><option>UPCOM</option>
          </select>
          <span style={{ marginLeft: 'auto', fontSize: 12, color: 'var(--text-3)', fontFamily: 'var(--font-mono)' }}>
            {meta.total > 0 ? meta.total + ' mã' : (loading ? '...' : D.stocks.length + ' mã')}
          </span>
        </div>
      </Panel>

      {/* Price chart */}
      <Panel
        title="Biểu đồ giá VN30"
        dot={activeSym || '—'}
        className="section-gap"
        tools={
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            <Seg
              options={[
                { value: '30', label: '30N' },
                { value: '90', label: '90N' },
                { value: '180', label: '180N' },
              ]}
              value={chartDays}
              onChange={setChartDays}
            />
            {isLoggedIn && (
              <>
                <button
                  className="btn btn--sm"
                  onClick={() =>
                    authPost('/api/trigger/crawler').then((ok) =>
                      vnsToast(ok ? 'Đã gửi yêu cầu thu thập VN30' : 'Không thể gửi yêu cầu thu thập')
                    )
                  }
                >
                  <Icon name="download" size={13} />Thu thập
                </button>
                <button
                  className="btn btn--sm"
                  style={{ background: 'var(--accent)', borderColor: 'var(--accent)', color: '#fff' }}
                  onClick={() =>
                    authPost('/api/trigger/predict').then((ok) =>
                      vnsToast(ok ? 'Đã gửi yêu cầu chạy dự đoán VN30' : 'Không thể gửi yêu cầu dự đoán')
                    )
                  }
                >
                  <Icon name="play" size={13} />Dự đoán
                </button>
              </>
            )}
          </div>
        }
      >
        {/* Symbol selector chips */}
        {D.stocks.length > 0 && (
          <div className="chips" style={{ marginBottom: 16 }}>
            {D.stocks.map((s) => (
              <button
                key={s.sym}
                className={`chip ${activeSym === s.sym ? 'active' : ''}`}
                onClick={() => setActiveSym(s.sym)}
              >
                {s.sym}
              </button>
            ))}
          </div>
        )}

        {activeSym && (() => {
          const cur = D.stocks.find((s) => s.sym === activeSym);
          return cur ? (
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 14, marginBottom: 10 }}>
              <span className="num" style={{ fontSize: 30, fontWeight: 600, letterSpacing: '-1px' }}>
                {fmt.price(cur.price)}
              </span>
              <span style={{ fontSize: 12, color: 'var(--text-3)' }}>nghìn đ / cp</span>
              <Chg pct={cur.chgPct} abs={cur.change} />
              <span style={{ marginLeft: 'auto', fontSize: 11, color: 'var(--text-3)', fontFamily: 'var(--font-mono)' }}>
                VN30 · {cur.sym}
              </span>
            </div>
          ) : null;
        })()}

        {chartLoading ? (
          <div className="empty" style={{ height: 400, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
            <div className="empty__icon"><Icon name="refresh" size={18} /></div>
            <p>Đang tải biểu đồ...</p>
          </div>
        ) : stockChart.prices.length > 0 ? (
          <LineChart
            series={[{ name: activeSym || 'Giá', data: stockChart.prices, color: 'var(--accent)' }]}
            labels={stockChart.dates}
            height={400}
            area
            yFmt={(v) => v.toFixed(1)}
            valueFmt={(v) => fmt.price(v)}
            padL={52}
          />
        ) : (
          <div className="empty" style={{ height: 400, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
            <div className="empty__icon"><Icon name="layers" size={18} /></div>
            <p>Chưa có dữ liệu biểu đồ. Hãy thu thập dữ liệu trước.</p>
          </div>
        )}
      </Panel>

      <Panel title="Bảng giá cổ phiếu" sub="cập nhật cuối 15:00" flush className="section-gap"
        tools={<button className="btn btn--sm btn--ghost"><Icon name="refresh" size={13} />Làm mới</button>}>
        {loading && pagedStocks.length === 0
          ? <div className="empty"><p>Đang tải...</p></div>
          : pagedStocks.length === 0
            ? <div className="empty">
                <div className="empty__icon"><Icon name="layers" size={18} /></div>
                <p>Chưa có dữ liệu cổ phiếu. Hãy bấm crawl để lấy data.</p>
              </div>
            : <div style={{ overflowX: 'auto' }}>
                <table className="tbl">
                  <thead><tr>
                    <th className="l">Mã</th>
                    <th>Ngành</th>
                    {sortHead('price', 'Giá')}
                    <th className="r">Δ</th>
                    {sortHead('change_percent', '±%')}
                    <th className="r">KL (M)</th>
                    <th className="r">GT (Ngàn tỷ)</th>
                    <th className="c">7 phiên</th>
                    <th className="c"></th>
                  </tr></thead>
                  <tbody>
                    {pagedStocks.map((s) => (
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
        {meta.totalPages > 1 && (
          <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', gap: 6, padding: '12px 16px', borderTop: '1px solid var(--border)' }}>
            <button
              className="btn btn--sm btn--ghost"
              disabled={page <= 1 || loading}
              onClick={() => handlePage(page - 1)}
            >←</button>
            {Array.from({ length: meta.totalPages }, (_, i) => i + 1).map((p) => (
              <button
                key={p}
                className={`btn btn--sm ${p === page ? 'btn--primary' : 'btn--ghost'}`}
                onClick={() => handlePage(p)}
                disabled={loading}
              >{p}</button>
            ))}
            <button
              className="btn btn--sm btn--ghost"
              disabled={page >= meta.totalPages || loading}
              onClick={() => handlePage(page + 1)}
            >→</button>
            <span style={{ fontSize: 12, color: 'var(--text-3)', marginLeft: 8 }}>
              {meta.total} mã tổng
            </span>
          </div>
        )}
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
              ? <LineChart series={[{ name: s.sym, data: hist, color: s.chgPct >= 0 ? 'var(--up)' : 'var(--down)' }]} labels={hist.map((_, i) => `${i + 1}`)} height={520} area valueFmt={(v) => fmt.price(v)} />
              : <div className="empty" style={{ height: 520, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
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
