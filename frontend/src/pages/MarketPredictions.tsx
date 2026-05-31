import { useState, useEffect, useCallback, useRef } from 'react';
import { useParams, NavLink } from 'react-router-dom';
import { useData } from '../context/DataContext';
import { Panel, Icon, Chg, ConfBar } from '../components/ui';
import { fetchMarketPredictions } from '../api';

// ── Helpers ──────────────────────────────────────────────────────────────────
function ddmm(s: any): string {
  if (!s) return '—';
  try {
    const d = new Date(s);
    if (isNaN(d.getTime())) return String(s).slice(0, 10);
    return ('0' + d.getDate()).slice(-2) + '/' + ('0' + (d.getMonth() + 1)).slice(-2) + '/' + d.getFullYear().toString().slice(-2);
  } catch (_e) { return String(s).slice(0, 10); }
}
function num(x: any): number {
  const n = typeof x === 'number' ? x : parseFloat(x);
  return isFinite(n) ? n : 0;
}
function conf(c: any): number {
  let n = num(c);
  if (n > 0 && n <= 1) n *= 100;
  return Math.round(n);
}
function fmtPrice(n: number): string {
  return n.toLocaleString('vi-VN', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}
function fmtGold(n: number): string {
  return n >= 1e6 ? (n / 1e6).toFixed(2) + ' tr' : n.toLocaleString('vi-VN');
}

// ── Pagination ────────────────────────────────────────────────────────────────
function Pagination({ page, totalPages, onChange }: { page: number; totalPages: number; onChange: (p: number) => void }) {
  if (totalPages <= 1) return null;
  const pages: (number | '...')[] = [];
  if (totalPages <= 7) {
    for (let i = 1; i <= totalPages; i++) pages.push(i);
  } else {
    pages.push(1);
    if (page > 3) pages.push('...');
    for (let i = Math.max(2, page - 1); i <= Math.min(totalPages - 1, page + 1); i++) pages.push(i);
    if (page < totalPages - 2) pages.push('...');
    pages.push(totalPages);
  }
  return (
    <div className="pagination">
      <button className="pagination__btn" disabled={page <= 1} onClick={() => onChange(page - 1)}>
        <Icon name="caretDown" size={13} style={{ transform: 'rotate(90deg)' }} />
        Trước
      </button>
      {pages.map((p, i) =>
        p === '...'
          ? <span key={'e' + i} className="pagination__ellipsis">…</span>
          : <button key={p} className={`pagination__btn ${p === page ? 'active' : ''}`} onClick={() => onChange(p as number)}>{p}</button>
      )}
      <button className="pagination__btn" disabled={page >= totalPages} onClick={() => onChange(page + 1)}>
        Tiếp
        <Icon name="caretDown" size={13} style={{ transform: 'rotate(-90deg)' }} />
      </button>
    </div>
  );
}

// ── Sort header cell ──────────────────────────────────────────────────────────
function SortTh({ label, field, sortBy, sortDir, onSort, className }: {
  label: string; field: string; sortBy: string; sortDir: 'asc' | 'desc';
  onSort: (f: string) => void; className?: string;
}) {
  const active = sortBy === field;
  return (
    <th className={`th-sort ${className || ''}`} onClick={() => onSort(field)}>
      {label}
      <span className="caret" style={{ marginLeft: 4, color: active ? 'var(--accent)' : 'var(--text-3)' }}>
        {active ? (sortDir === 'asc' ? '↑' : '↓') : '↕'}
      </span>
    </th>
  );
}

// ── Sub-nav tabs ──────────────────────────────────────────────────────────────
function MarketTabs({ marketKey }: { marketKey: string }) {
  const base = '/markets/' + marketKey;
  return (
    <div className="market-tabs">
      <NavLink to={base} end className={({ isActive }) => 'market-tab' + (isActive ? ' active' : '')}>
        <Icon name={marketKey === 'gold' ? 'gold' : 'candles'} size={14} />
        Tổng quan
      </NavLink>
      <NavLink to={base + '/predictions'} className={({ isActive }) => 'market-tab' + (isActive ? ' active' : '')}>
        <Icon name="pulse" size={14} />
        Dự đoán
      </NavLink>
      <NavLink to={base + '/detail'} className={({ isActive }) => 'market-tab' + (isActive ? ' active' : '')}>
        <Icon name="layers" size={14} />
        Chi tiết
      </NavLink>
      <NavLink to={base + '/training'} className={({ isActive }) => 'market-tab' + (isActive ? ' active' : '')}>
        <Icon name="cpu" size={14} />
        Huấn luyện
      </NavLink>
    </div>
  );
}

// ── Status badge ──────────────────────────────────────────────────────────────
function StatusBadge({ status, acc }: { status?: string; acc?: number }) {
  if (status === 'pending' || (!status && acc == null)) {
    return <span className="badge badge--muted">Đang chờ</span>;
  }
  if (status === 'confirmed' || acc != null) {
    const a = acc || 0;
    const cls = a > 85 ? 'badge--up' : a > 72 ? 'badge--accent' : 'badge--down';
    const label = a > 85 ? 'Chính xác' : a > 72 ? 'Gần đúng' : 'Sai lệch';
    return <span className={`badge ${cls}`}>{label}</span>;
  }
  return <span className="badge badge--muted">{status || '—'}</span>;
}

// ── Main component ────────────────────────────────────────────────────────────
export default function MarketPredictions() {
  const { marketKey = 'vn30' } = useParams<{ marketKey: string }>();
  const { data: D } = useData();

  const [page, setPage]             = useState(1);
  const limit                       = 20;
  const [search, setSearch]         = useState('');
  const [debouncedSearch, setDebouncedSearch] = useState('');
  const [sortBy, setSortBy]         = useState('prediction_date');
  const [sortDir, setSortDir]       = useState<'asc' | 'desc'>('desc');
  const [algorithm, setAlgorithm]   = useState('ema');
  const [status, setStatus]         = useState('');
  const [rows, setRows]             = useState<any[]>([]);
  const [total, setTotal]           = useState(0);
  const [totalPages, setTotalPages] = useState(0);
  const [loading, setLoading]       = useState(false);
  const [error, setError]           = useState<string | null>(null);

  // Reset page on filter/sort change
  const searchTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    if (searchTimer.current) clearTimeout(searchTimer.current);
    searchTimer.current = setTimeout(() => {
      setDebouncedSearch(search);
      setPage(1);
    }, 400);
    return () => { if (searchTimer.current) clearTimeout(searchTimer.current); };
  }, [search]);

  const handleSort = useCallback((field: string) => {
    setSortBy((prev) => {
      if (prev === field) { setSortDir((d) => d === 'asc' ? 'desc' : 'asc'); return prev; }
      setSortDir('desc');
      return field;
    });
    setPage(1);
  }, []);

  // Fetch
  useEffect(() => {
    setLoading(true);
    setError(null);
    fetchMarketPredictions(marketKey, {
      page, limit,
      search: debouncedSearch || undefined,
      sort_by: sortBy,
      sort_dir: sortDir,
      algorithm: algorithm || undefined,
      status: status || undefined,
    }).then((res) => {
      setRows(Array.isArray(res.data) ? res.data : []);
      setTotal(res.total || 0);
      setTotalPages(res.total_pages || 0);
    }).catch((e) => {
      setError('Không thể tải dữ liệu: ' + (e?.message || 'Lỗi không xác định'));
      setRows([]);
    }).finally(() => setLoading(false));
  }, [marketKey, page, debouncedSearch, sortBy, sortDir, algorithm, status]);

  const isGold = marketKey === 'gold';
  const marketLabel = isGold ? 'Vàng' : 'VN30';

  const FALLBACK_ALGOS = [
    { id: 'ema', short: 'EMA', name: 'EMA' },
    { id: 'lstm_nn', short: 'LSTM', name: 'LSTM' },
    { id: 'arima_garch', short: 'ARIMA', name: 'ARIMA' },
    { id: 'moving_average', short: 'MA', name: 'MA' },
    { id: 'ensemble', short: 'ENS', name: 'Ensemble' },
  ];
  const algos = D.algos.length > 0 ? D.algos : FALLBACK_ALGOS;

  return (
    <div className="content__inner fade">
      <MarketTabs marketKey={marketKey} />

      {/* Algorithm pills */}
      <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', marginTop: 12, marginBottom: 0 }}>
        {algos.map(a => (
          <button
            key={a.id}
            onClick={() => { setAlgorithm(a.id); setPage(1); }}
            style={{
              padding: '4px 12px',
              borderRadius: 4,
              border: '1px solid',
              borderColor: algorithm === a.id ? 'var(--accent)' : 'var(--border)',
              background: algorithm === a.id ? 'var(--accent)' : 'transparent',
              color: algorithm === a.id ? '#fff' : 'var(--text-2)',
              cursor: 'pointer',
              fontSize: 12,
              fontWeight: 600,
              fontFamily: 'var(--font-mono)',
              letterSpacing: '0.03em',
            }}
          >
            {a.short || a.name}
          </button>
        ))}
      </div>

      {/* Controls */}
      <Panel className="section-gap" style={{ marginTop: 8 }}>
        <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'center' }}>
          <Icon name="filter" size={15} style={{ color: 'var(--text-3)' }} />
          <div className="search" style={{ width: 220 }}>
            <Icon name="search" size={14} />
            <input
              placeholder="Tìm kiếm..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>
          <select className="sel" value={status} onChange={(e) => { setStatus(e.target.value); setPage(1); }}>
            <option value="">Tất cả trạng thái</option>
            <option value="pending">Đang chờ</option>
            <option value="confirmed">Đã xác nhận</option>
          </select>
          <span style={{ marginLeft: 'auto', fontSize: 12, color: 'var(--text-3)', fontFamily: 'var(--font-mono)' }}>
            {total > 0 ? total.toLocaleString() + ' bản ghi' : ''}
          </span>
        </div>
      </Panel>

      {/* Table */}
      <Panel flush className="section-gap">
        {error && (
          <div style={{ padding: '16px', color: 'var(--down)', fontSize: 13, display: 'flex', gap: 8, alignItems: 'center' }}>
            <Icon name="layers" size={15} />{error}
          </div>
        )}
        <div style={{ overflowX: 'auto', opacity: loading ? 0.5 : 1, transition: 'opacity .15s', pointerEvents: loading ? 'none' : 'auto' }}>
          {rows.length === 0 && !loading && !error
            ? <div className="empty" style={{ padding: '60px 20px' }}>
                <div className="empty__icon"><Icon name="pulse" size={18} /></div>
                <p>Chưa có dữ liệu dự đoán {marketLabel}</p>
              </div>
            : <table className="tbl">
                <thead>
                  <tr>
                    {isGold
                      ? <>
                          <SortTh label="Nguồn" field="source" sortBy={sortBy} sortDir={sortDir} onSort={handleSort} />
                          <th>Sản phẩm</th>
                        </>
                      : <SortTh label="Mã" field="symbol" sortBy={sortBy} sortDir={sortDir} onSort={handleSort} />
                    }
                    <SortTh label="Giá dự đoán" field="predicted_price" sortBy={sortBy} sortDir={sortDir} onSort={handleSort} className="r" />
                    <SortTh label="Giá thực tế" field="actual_price" sortBy={sortBy} sortDir={sortDir} onSort={handleSort} className="r" />
                    <th className="r">Δ dự đoán</th>
                    <SortTh label="Độ tin cậy" field="confidence" sortBy={sortBy} sortDir={sortDir} onSort={handleSort} className="r" />
                    <SortTh label="Độ chính xác" field="accuracy" sortBy={sortBy} sortDir={sortDir} onSort={handleSort} className="r" />
                    <th className="c">Trạng thái</th>
                    <SortTh label="Ngày dự đoán" field="prediction_date" sortBy={sortBy} sortDir={sortDir} onSort={handleSort} className="r" />
                  </tr>
                </thead>
                <tbody>
                  {rows.map((row, i) => {
                    const cur    = num(row.current_price);
                    const pred   = num(row.predicted_price);
                    const act    = row.actual_price != null ? num(row.actual_price) : null;
                    const delta  = cur ? +(((pred - cur) / cur) * 100).toFixed(2) : 0;
                    const acc    = row.accuracy != null ? Math.round(num(row.accuracy) > 1 ? num(row.accuracy) : num(row.accuracy) * 100) : null;
                    const fmtFn = isGold ? fmtGold : fmtPrice;
                    return (
                      <tr key={i}>
                        {isGold
                          ? <>
                              <td><div className="sym">{row.source || '—'}</div></td>
                              <td><span style={{ color: 'var(--text-2)', fontSize: 12 }}>{row.product_type || '—'}</span></td>
                            </>
                          : <td>
                              <div className="sym">{(row.stock && row.stock.symbol) || row.symbol || '—'}</div>
                              <div className="co">{(row.stock && row.stock.company_name) || ''}</div>
                            </td>
                        }
                        <td className="r num">{fmtFn(pred)}</td>
                        <td className="r num" style={{ color: 'var(--text-2)' }}>{act != null ? fmtFn(act) : <span style={{ color: 'var(--text-3)' }}>—</span>}</td>
                        <td className="r"><Chg pct={delta} /></td>
                        <td className="r"><ConfBar v={conf(row.confidence)} /></td>
                        <td className="r num" style={{ fontWeight: 600, color: acc == null ? 'var(--text-3)' : acc > 85 ? 'var(--up)' : acc > 72 ? 'var(--text)' : 'var(--down)' }}>
                          {acc != null ? acc + '%' : <span style={{ color: 'var(--text-3)' }}>—</span>}
                        </td>
                        <td className="c"><StatusBadge status={row.status} acc={acc ?? undefined} /></td>
                        <td className="r" style={{ color: 'var(--text-3)', fontSize: 12, fontFamily: 'var(--font-mono)' }}>{ddmm(row.prediction_date)}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
          }
        </div>
        {totalPages > 1 && (
          <div style={{ borderTop: '1px solid var(--border)', padding: '12px 16px', display: 'flex', justifyContent: 'flex-end' }}>
            <Pagination page={page} totalPages={totalPages} onChange={(p) => { setPage(p); window.scrollTo(0, 0); }} />
          </div>
        )}
      </Panel>
    </div>
  );
}
