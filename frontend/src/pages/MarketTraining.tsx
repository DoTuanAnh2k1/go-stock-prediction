import { useState, useEffect, useCallback } from 'react';
import { useParams, NavLink } from 'react-router-dom';
import { useData } from '../context/DataContext';
import { Panel, Icon } from '../components/ui';
import { fetchMarketTraining } from '../api';
import { useLanguage } from '../context/LangContext';

// ── Helpers ──────────────────────────────────────────────────────────────────
function fmtDatetime(s: any): string {
  if (!s) return '—';
  try {
    const d = new Date(s);
    if (isNaN(d.getTime())) return String(s).slice(0, 16);
    return ('0' + d.getDate()).slice(-2) + '/' + ('0' + (d.getMonth() + 1)).slice(-2) + '/' + d.getFullYear().toString().slice(-2)
      + ' ' + ('0' + d.getHours()).slice(-2) + ':' + ('0' + d.getMinutes()).slice(-2);
  } catch (_e) { return String(s).slice(0, 16); }
}
function num(x: any): number {
  const n = typeof x === 'number' ? x : parseFloat(x);
  return isFinite(n) ? n : 0;
}
function shortId(s: string): string {
  return s ? s.slice(0, 8).toUpperCase() : '—';
}
function fmtDuration(ms: number): string {
  if (!ms) return '—';
  if (ms < 1000) return ms + ' ms';
  if (ms < 60000) return (ms / 1000).toFixed(1) + ' s';
  return Math.floor(ms / 60000) + ' min ' + Math.floor((ms % 60000) / 1000) + ' s';
}

// ── Pagination ────────────────────────────────────────────────────────────────
function Pagination({ page, totalPages, onChange }: { page: number; totalPages: number; onChange: (p: number) => void }) {
  const { t } = useLanguage();
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
        {t.marketTraining.prevPage}
      </button>
      {pages.map((p, i) =>
        p === '...'
          ? <span key={'e' + i} className="pagination__ellipsis">…</span>
          : <button key={p} className={`pagination__btn ${p === page ? 'active' : ''}`} onClick={() => onChange(p as number)}>{p}</button>
      )}
      <button className="pagination__btn" disabled={page >= totalPages} onClick={() => onChange(page + 1)}>
        {t.marketTraining.nextPage}
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
  const { t } = useLanguage();
  const base = '/markets/' + marketKey;
  const overviewIcon = marketKey === 'gold' ? 'gold' : 'candles';
  return (
    <div className="market-tabs">
      <NavLink to={base} end className={({ isActive }) => 'market-tab' + (isActive ? ' active' : '')}>
        <Icon name={overviewIcon} size={14} />
        {t.marketTabs.overview}
      </NavLink>
      <NavLink to={base + '/predictions'} className={({ isActive }) => 'market-tab' + (isActive ? ' active' : '')}>
        <Icon name="pulse" size={14} />
        {t.marketTabs.predictions}
      </NavLink>
      <NavLink to={base + '/detail'} className={({ isActive }) => 'market-tab' + (isActive ? ' active' : '')}>
        <Icon name="layers" size={14} />
        {t.marketTabs.detail}
      </NavLink>
      <NavLink to={base + '/training'} className={({ isActive }) => 'market-tab' + (isActive ? ' active' : '')}>
        <Icon name="cpu" size={14} />
        {t.marketTabs.training}
      </NavLink>
    </div>
  );
}

// ── Main component ────────────────────────────────────────────────────────────
export default function MarketTraining() {
  const { marketKey = 'vn30' } = useParams<{ marketKey: string }>();
  const { data: D } = useData();
  const { t } = useLanguage();

  const [page, setPage]             = useState(1);
  const limit                       = 20;
  const [sortBy, setSortBy]         = useState('started_at');
  const [sortDir, setSortDir]       = useState<'asc' | 'desc'>('desc');
  const [algorithm, setAlgorithm]   = useState('');
  const [rows, setRows]             = useState<any[]>([]);
  const [total, setTotal]           = useState(0);
  const [totalPages, setTotalPages] = useState(0);
  const [loading, setLoading]       = useState(false);
  const [error, setError]           = useState<string | null>(null);

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
    fetchMarketTraining(marketKey, {
      page, limit,
      sort_by: sortBy,
      sort_dir: sortDir,
      algorithm: algorithm || undefined,
    }).then((res) => {
      setRows(Array.isArray(res.data) ? res.data : []);
      setTotal(res.total || 0);
      setTotalPages(res.total_pages || 0);
    }).catch((e) => {
      setError(t.marketTraining.cannotLoad + ': ' + (e?.message || t.marketTraining.unknownError));
      setRows([]);
    }).finally(() => setLoading(false));
  }, [marketKey, page, sortBy, sortDir, algorithm]);

  const marketLabel = marketKey === 'gold' ? 'Vàng'
    : marketKey === 'nasdaq100' ? 'NASDAQ 100'
    : marketKey === 'crypto'    ? 'Crypto'
    : marketKey === 'fuel'      ? 'Giá Xăng'
    : 'VN30';

  return (
    <div className="content__inner fade">
      <MarketTabs marketKey={marketKey} />

      {/* Controls */}
      <Panel className="section-gap" style={{ marginTop: 16 }}>
        <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'center' }}>
          <Icon name="filter" size={15} style={{ color: 'var(--text-3)' }} />
          <select className="sel" value={algorithm} onChange={(e) => { setAlgorithm(e.target.value); setPage(1); }}>
            <option value="">{t.marketTraining.allAlgos}</option>
            {D.algos.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
          </select>
          <span style={{ marginLeft: 'auto', fontSize: 12, color: 'var(--text-3)', fontFamily: 'var(--font-mono)' }}>
            {total > 0 ? total.toLocaleString() + ' ' + t.marketTraining.sessions : ''}
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
                <div className="empty__icon"><Icon name="cpu" size={18} /></div>
                <p>{t.marketTraining.noTrainingHistory} {marketLabel}</p>
              </div>
            : <table className="tbl">
                <thead>
                  <tr>
                    <th>{t.marketTraining.colSession}</th>
                    <th>{t.marketTraining.colAlgo}</th>
                    <SortTh label={t.marketTraining.colTotal} field="total_count" sortBy={sortBy} sortDir={sortDir} onSort={handleSort} className="r" />
                    <SortTh label={t.marketTraining.colSuccess} field="success_count" sortBy={sortBy} sortDir={sortDir} onSort={handleSort} className="r" />
                    <SortTh label={t.marketTraining.colError} field="error_count" sortBy={sortBy} sortDir={sortDir} onSort={handleSort} className="r" />
                    <SortTh label={t.marketTraining.colAccuracy} field="accuracy" sortBy={sortBy} sortDir={sortDir} onSort={handleSort} className="r" />
                    <SortTh label={t.marketTraining.colDuration} field="duration_ms" sortBy={sortBy} sortDir={sortDir} onSort={handleSort} className="r" />
                    <SortTh label={t.marketTraining.colStarted} field="started_at" sortBy={sortBy} sortDir={sortDir} onSort={handleSort} className="r" />
                    <SortTh label={t.marketTraining.colCompleted} field="completed_at" sortBy={sortBy} sortDir={sortDir} onSort={handleSort} className="r" />
                  </tr>
                </thead>
                <tbody>
                  {rows.map((row, i) => {
                    const algoKey = (row.algorithm_name || '').toLowerCase();
                    const algoCls = algoKey.includes('lstm') ? 'lstm' : algoKey.includes('arima') ? 'arima' : algoKey.includes('ema') ? 'ema' : algoKey.includes('ensemble') ? 'ens' : 'ma';
                    const algoShort = algoKey.includes('lstm') ? 'LSTM' : algoKey.includes('arima') ? 'ARIMA' : algoKey.includes('ema') ? 'EMA' : algoKey.includes('ensemble') ? 'ENS' : 'MA';
                    const acc = row.accuracy != null ? Math.round(num(row.accuracy) > 1 ? num(row.accuracy) : num(row.accuracy) * 100) : null;
                    const errors = num(row.error_count);
                    return (
                      <tr key={i}>
                        <td>
                          <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12, color: 'var(--accent)' }}>
                            {shortId(row.session_id || row.id || String(i))}
                          </span>
                        </td>
                        <td><span className={`algo algo--${algoCls}`}>{algoShort}</span></td>
                        <td className="r num" style={{ color: 'var(--text-2)' }}>{num(row.total_count) || '—'}</td>
                        <td className="r num" style={{ color: 'var(--up)' }}>{num(row.success_count) || '—'}</td>
                        <td className="r num" style={{ color: errors > 0 ? 'var(--down)' : 'var(--text-3)' }}>{errors || '—'}</td>
                        <td className="r num" style={{ fontWeight: 600, color: acc == null ? 'var(--text-3)' : acc > 85 ? 'var(--up)' : acc > 72 ? 'var(--text)' : 'var(--down)' }}>
                          {acc != null ? acc + '%' : '—'}
                        </td>
                        <td className="r" style={{ fontSize: 12, color: 'var(--text-2)', fontFamily: 'var(--font-mono)' }}>
                          {fmtDuration(num(row.duration_ms))}
                        </td>
                        <td className="r" style={{ fontSize: 12, color: 'var(--text-3)', fontFamily: 'var(--font-mono)' }}>
                          {fmtDatetime(row.started_at)}
                        </td>
                        <td className="r" style={{ fontSize: 12, color: 'var(--text-3)', fontFamily: 'var(--font-mono)' }}>
                          {fmtDatetime(row.completed_at)}
                        </td>
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
