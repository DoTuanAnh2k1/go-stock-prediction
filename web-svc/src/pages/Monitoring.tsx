import React, { useState, useEffect, useCallback } from 'react';
import { Panel, Icon } from '../components/ui';
import { useAuth } from '../context/AuthContext';
import { useLanguage } from '../context/LangContext';
import {
  fetchMonitoringOverview,
  fetchMonitoringBots,
  type MonitoringOverview,
  type MonitoringMarket,
  type MonitoringBotsPage,
  type MonitoringBotSortKey,
} from '../api';

// ── Helpers ──────────────────────────────────────────────────────────────────

function fmtDateTime(s: string | null): string {
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

function fmtPct(v: number): string {
  return (v * 100).toFixed(1) + '%';
}

function fmtNumber(v: number): string {
  return v.toLocaleString('en-US');
}

function fmtPnl(v: number): string {
  const formatted = Math.abs(v).toLocaleString('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
  return (v >= 0 ? '+' : '-') + formatted;
}

function fmtReturnPct(v: number): string {
  return (v >= 0 ? '+' : '') + v.toFixed(2) + '%';
}

const MARKET_COLORS: Record<string, string> = {
  GOLD:   'var(--gold)',
  NASDAQ: 'oklch(0.74 0.13 200)',
  SP500:  'oklch(0.72 0.15 145)',
  CRYPTO: '#F7931A',
};

const MARKET_ICONS: Record<string, string> = {
  GOLD:   'gold',
  NASDAQ: 'nasdaq',
  SP500:  'nasdaq',
  CRYPTO: 'crypto',
};

// ── Market Card ───────────────────────────────────────────────────────────────

function MarketCard({ market }: { market: MonitoringMarket }) {
  const { t } = useLanguage();
  const m = t.monitoring;
  const color = MARKET_COLORS[market.market] || 'var(--accent)';
  const icon = MARKET_ICONS[market.market] || 'candles';
  const { crawl, predictions } = market;

  return (
    <div className="panel" style={{ borderTop: `3px solid ${color}` }}>
      <div className="panel__head">
        <div className="panel__title" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <Icon name={icon} size={16} style={{ color }} />
          <span style={{ color }}>{market.market}</span>
        </div>
      </div>
      <div className="panel__body">
        {/* ── Crawl section ── */}
        <div style={{ marginBottom: 14 }}>
          <div style={{
            fontSize: 11, fontWeight: 600, letterSpacing: '0.9px',
            textTransform: 'uppercase', color: 'var(--text-3)', marginBottom: 8,
          }}>
            {m.crawlSection}
          </div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px 20px', fontSize: 13 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <span style={{ color: 'var(--text-3)' }}>{m.lastCrawl}:</span>
              <span className="mono" style={{ color: 'var(--text-2)' }}>
                {crawl.last_crawl_at ? fmtDateTime(crawl.last_crawl_at) : m.never}
              </span>
              <span
                style={{
                  fontSize: 11, padding: '1px 7px',
                  background: crawl.stale ? 'var(--down-bg)' : 'var(--up-bg)',
                  color: crawl.stale ? 'var(--down)' : 'var(--up)',
                  fontFamily: 'var(--font-mono)',
                }}
              >
                {crawl.staleness || m.never}
              </span>
              {!crawl.market_open && (
                <span style={{
                  fontSize: 11, padding: '1px 7px',
                  background: 'var(--text-3)', color: 'var(--bg)',
                  opacity: 0.75,
                }}>
                  {m.marketClosed}
                </span>
              )}
              {crawl.stale && crawl.market_open && (
                <span style={{
                  fontSize: 11, padding: '1px 7px',
                  background: 'var(--down-bg)', color: 'var(--down)',
                }}>
                  {m.staleWarning}
                </span>
              )}
            </div>
          </div>
          <div style={{ display: 'flex', gap: 24, marginTop: 6, fontSize: 12 }}>
            <div>
              <span style={{ color: 'var(--text-3)' }}>{m.dailyToday}: </span>
              <span className="mono" style={{ fontWeight: 600 }}>{fmtNumber(crawl.daily_today)}</span>
            </div>
            <div>
              <span style={{ color: 'var(--text-3)' }}>{m.intradayToday}: </span>
              <span className="mono" style={{ fontWeight: 600 }}>{fmtNumber(crawl.intraday_today)}</span>
            </div>
          </div>
        </div>

        {/* ── Predictions section ── */}
        <div>
          <div style={{
            fontSize: 11, fontWeight: 600, letterSpacing: '0.9px',
            textTransform: 'uppercase', color: 'var(--text-3)', marginBottom: 8,
          }}>
            {m.predSection}
          </div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px 20px', fontSize: 13, marginBottom: 8 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <span style={{ color: 'var(--text-3)' }}>{m.lastPredict}:</span>
              <span className="mono" style={{ color: 'var(--text-2)' }}>
                {predictions.last_predict_at ? fmtDateTime(predictions.last_predict_at) : '—'}
              </span>
              {predictions.last_predict_at && (
                <span style={{
                  fontSize: 11, padding: '1px 7px',
                  background: 'var(--surface-2)', color: 'var(--text-3)',
                  fontFamily: 'var(--font-mono)',
                }}>
                  {predictions.staleness}
                </span>
              )}
            </div>
          </div>
          <div style={{ display: 'flex', gap: 24, marginBottom: 8, fontSize: 12 }}>
            <div>
              <span style={{ color: 'var(--text-3)' }}>{m.todayTotal}: </span>
              <span className="mono" style={{ fontWeight: 600 }}>{predictions.today_total}</span>
            </div>
            <div>
              <span style={{ color: 'var(--text-3)' }}>{m.expectedAlgos}: </span>
              <span className="mono" style={{ fontWeight: 600 }}>{predictions.expected_algos}</span>
            </div>
          </div>

          {/* Missing algos badges */}
          {predictions.missing_today && predictions.missing_today.length > 0 ? (
            <div style={{ marginBottom: 8 }}>
              <span style={{ fontSize: 12, color: 'var(--text-3)', marginRight: 6 }}>
                {m.missingAlgos}:
              </span>
              <span style={{ display: 'inline-flex', flexWrap: 'wrap', gap: 4 }}>
                {predictions.missing_today.map((algo) => (
                  <span key={algo} style={{
                    fontSize: 11, padding: '2px 8px',
                    background: 'var(--down-bg)', color: 'var(--down)',
                    fontFamily: 'var(--font-mono)',
                  }}>
                    {algo}
                  </span>
                ))}
              </span>
            </div>
          ) : (
            <div style={{ marginBottom: 8 }}>
              <span style={{
                fontSize: 11, padding: '2px 8px',
                background: 'var(--up-bg)', color: 'var(--up)',
              }}>
                {m.noMissing}
              </span>
            </div>
          )}

          {/* Per-algo table */}
          {predictions.algorithms && predictions.algorithms.length > 0 ? (
            <div style={{ overflowX: 'auto' }}>
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12 }}>
                <thead>
                  <tr style={{ borderBottom: '1px solid var(--border)' }}>
                    <th style={{ textAlign: 'left', padding: '4px 8px', color: 'var(--text-3)', fontWeight: 600 }}>{m.colAlgo}</th>
                    <th style={{ textAlign: 'right', padding: '4px 8px', color: 'var(--text-3)', fontWeight: 600 }}>{m.colTodayCount}</th>
                    <th style={{ textAlign: 'right', padding: '4px 8px', color: 'var(--text-3)', fontWeight: 600 }}>{m.colDirAccuracy}</th>
                    <th style={{ textAlign: 'right', padding: '4px 8px', color: 'var(--text-3)', fontWeight: 600 }}>{m.colReconciled}</th>
                    <th style={{ textAlign: 'right', padding: '4px 8px', color: 'var(--text-3)', fontWeight: 600 }}>{m.colCorrect}</th>
                  </tr>
                </thead>
                <tbody>
                  {predictions.algorithms.map((row) => (
                    <tr key={row.algorithm} style={{ borderBottom: '1px solid var(--border)' }}>
                      <td style={{ padding: '4px 8px', fontFamily: 'var(--font-mono)', fontSize: 11 }}>
                        {row.algorithm}
                      </td>
                      <td style={{ textAlign: 'right', padding: '4px 8px' }} className="mono">
                        {row.today_count}
                      </td>
                      <td style={{ textAlign: 'right', padding: '4px 8px' }} className="mono">
                        <span style={{
                          color: row.direction_accuracy >= 0.55 ? 'var(--up)' : row.direction_accuracy >= 0.45 ? 'var(--gold)' : 'var(--down)',
                        }}>
                          {fmtPct(row.direction_accuracy)}
                        </span>
                      </td>
                      <td style={{ textAlign: 'right', padding: '4px 8px' }} className="mono">
                        {row.reconciled}
                      </td>
                      <td style={{ textAlign: 'right', padding: '4px 8px' }} className="mono">
                        {row.correct}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <div style={{ color: 'var(--text-3)', fontSize: 12 }}>{m.noAlgoData}</div>
          )}
        </div>
      </div>
    </div>
  );
}

// ── Sort key type ─────────────────────────────────────────────────────────────

type SortDir = 'asc' | 'desc';

const BOT_MARKETS = ['GOLD', 'NASDAQ', 'CRYPTO', 'SP500'];
const PAGE_SIZE_OPTIONS = [25, 50, 100, 200];

// ── Bots Table (server-side paginated / filtered / sorted) ──────────────────────

function BotsTable() {
  const { t } = useLanguage();
  const m = t.monitoring;

  // Query state.
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(50);
  const [market, setMarket] = useState('');
  const [sortKey, setSortKey] = useState<MonitoringBotSortKey>('return_pct');
  const [sortDir, setSortDir] = useState<SortDir>('desc');

  // Text inputs (debounced into the actual query terms).
  const [algoInput, setAlgoInput] = useState('');
  const [searchInput, setSearchInput] = useState('');
  const [algo, setAlgo] = useState('');
  const [search, setSearch] = useState('');

  // Data state.
  const [resp, setResp] = useState<MonitoringBotsPage | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  // Debounce text filters → reset to page 1.
  useEffect(() => {
    const id = setTimeout(() => { setAlgo(algoInput.trim()); setPage(1); }, 350);
    return () => clearTimeout(id);
  }, [algoInput]);
  useEffect(() => {
    const id = setTimeout(() => { setSearch(searchInput.trim()); setPage(1); }, 350);
    return () => clearTimeout(id);
  }, [searchInput]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError('');
    fetchMonitoringBots({
      page, page_size: pageSize,
      market: market || undefined,
      algorithm: algo || undefined,
      search: search || undefined,
      sort_by: sortKey, sort_dir: sortDir,
    })
      .then((res) => { if (!cancelled) setResp(res); })
      .catch((e: any) => { if (!cancelled) setError(e.message || m.error); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [page, pageSize, market, algo, search, sortKey, sortDir, m.error]);

  function handleSort(key: MonitoringBotSortKey) {
    if (sortKey === key) {
      setSortDir((d) => (d === 'desc' ? 'asc' : 'desc'));
    } else {
      setSortKey(key);
      setSortDir('desc');
    }
    setPage(1);
  }

  function SortHeader({ colKey, label, align = 'right' }: { colKey: MonitoringBotSortKey; label: React.ReactNode; align?: 'left' | 'right' }) {
    const active = sortKey === colKey;
    return (
      <th
        onClick={() => handleSort(colKey)}
        style={{
          textAlign: align, padding: '7px 10px',
          color: active ? 'var(--accent)' : 'var(--text-3)',
          fontWeight: 600, cursor: 'pointer', userSelect: 'none',
          whiteSpace: 'nowrap',
        }}
      >
        {label}
        {active && (
          <span style={{ marginLeft: 4, fontSize: 10 }}>
            {sortDir === 'desc' ? '▼' : '▲'}
          </span>
        )}
      </th>
    );
  }

  const rows = resp?.data || [];
  const total = resp?.total || 0;
  const totalPages = resp?.total_pages || 0;
  const rangeStart = total === 0 ? 0 : (page - 1) * pageSize + 1;
  const rangeEnd = Math.min(page * pageSize, total);

  const selectStyle: React.CSSProperties = {
    background: 'var(--surface-2)', color: 'var(--text-1)',
    border: '1px solid var(--border)', padding: '4px 8px', fontSize: 12,
    borderRadius: 4,
  };
  const inputStyle: React.CSSProperties = { ...selectStyle, minWidth: 120 };

  return (
    <div>
      {/* ── Filters ── */}
      <div style={{
        display: 'flex', flexWrap: 'wrap', gap: 10, alignItems: 'center',
        marginBottom: 12, padding: '0 4px',
      }}>
        <select
          value={market}
          onChange={(e) => { setMarket(e.target.value); setPage(1); }}
          style={selectStyle}
        >
          <option value="">{m.allMarkets}</option>
          {BOT_MARKETS.map((mk) => <option key={mk} value={mk}>{mk}</option>)}
        </select>
        <input
          value={algoInput}
          onChange={(e) => setAlgoInput(e.target.value)}
          placeholder={m.filterAlgo}
          style={inputStyle}
        />
        <input
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          placeholder={m.filterBotId}
          style={inputStyle}
        />
        {(market || algoInput || searchInput) && (
          <button
            className="btn btn--sm"
            onClick={() => { setMarket(''); setAlgoInput(''); setSearchInput(''); setPage(1); }}
          >
            {m.clearFilters}
          </button>
        )}
        <div style={{ marginLeft: 'auto', fontSize: 12, color: 'var(--text-3)', fontFamily: 'var(--font-mono)' }}>
          {loading ? '…' : `${fmtNumber(rangeStart)}–${fmtNumber(rangeEnd)} / ${fmtNumber(total)}`}
        </div>
      </div>

      {/* ── Error ── */}
      {error && (
        <div style={{ padding: '12px', color: 'var(--down)', fontSize: 13 }}>
          {m.error}: {error}
        </div>
      )}

      {/* ── Table ── */}
      {!error && rows.length === 0 && !loading ? (
        <div style={{ padding: '32px', textAlign: 'center', color: 'var(--text-3)' }}>
          {m.noBotsTable}
        </div>
      ) : (
        <div style={{ overflowX: 'auto', opacity: loading ? 0.6 : 1, transition: 'opacity 0.15s' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--border)' }}>
                <SortHeader colKey="bot_id" label={m.colBotId} align="left" />
                <SortHeader colKey="market" label={m.colMarket} align="left" />
                <SortHeader colKey="algorithm" label={m.colAlgoBot} align="left" />
                <SortHeader colKey="trades" label={m.colTrades} />
                <th style={{ textAlign: 'right', padding: '7px 10px', color: 'var(--text-3)', fontWeight: 600 }}>{m.colWLBE}</th>
                <SortHeader colKey="win_rate" label={m.colWinRate} />
                <SortHeader colKey="total_pnl" label={m.colPnl} />
                <SortHeader colKey="return_pct" label={
                  <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
                    {m.colReturnPct}
                    <span
                      title={m.returnPctNote}
                      style={{
                        fontSize: 10, width: 14, height: 14, borderRadius: '50%',
                        border: '1px solid var(--text-3)', display: 'inline-flex',
                        alignItems: 'center', justifyContent: 'center',
                        color: 'var(--text-3)', cursor: 'help', flexShrink: 0,
                      }}
                    >
                      i
                    </span>
                  </span>
                } />
                <SortHeader colKey="profit_factor" label={m.colProfitFactor} />
                <SortHeader colKey="open_positions" label={m.colOpenPos} />
                <SortHeader colKey="unrealized_pnl" label={m.colUnrealized} />
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => {
                const pnlColor = row.total_pnl >= 0 ? 'var(--up)' : 'var(--down)';
                const retColor = row.return_pct >= 0 ? 'var(--up)' : 'var(--down)';
                const wrColor = row.win_rate >= 0.55 ? 'var(--up)' : row.win_rate >= 0.45 ? 'var(--gold)' : 'var(--down)';
                return (
                  <tr key={row.bot_id} style={{ borderBottom: '1px solid var(--border)' }}>
                    <td style={{ padding: '7px 10px', fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--text-2)' }}>
                      {row.bot_id}
                    </td>
                    <td style={{ padding: '7px 10px' }}>
                      <span style={{
                        fontSize: 11, padding: '2px 7px',
                        background: `${MARKET_COLORS[row.market] || 'var(--accent)'}22`,
                        color: MARKET_COLORS[row.market] || 'var(--accent)',
                        fontFamily: 'var(--font-mono)',
                      }}>
                        {row.market}
                      </span>
                    </td>
                    <td style={{ padding: '7px 10px', fontFamily: 'var(--font-mono)', fontSize: 11 }}>
                      {row.algorithm}
                    </td>
                    <td style={{ textAlign: 'right', padding: '7px 10px' }} className="mono">
                      {fmtNumber(row.trades)}
                    </td>
                    <td style={{ textAlign: 'right', padding: '7px 10px', fontFamily: 'var(--font-mono)', fontSize: 12 }}>
                      <span style={{ color: 'var(--up)' }}>{row.wins}</span>
                      {' / '}
                      <span style={{ color: 'var(--down)' }}>{row.losses}</span>
                      {' / '}
                      <span style={{ color: 'var(--text-3)' }}>{row.breakeven}</span>
                    </td>
                    <td style={{ textAlign: 'right', padding: '7px 10px' }} className="mono">
                      <span style={{ color: wrColor }}>{fmtPct(row.win_rate)}</span>
                    </td>
                    <td style={{ textAlign: 'right', padding: '7px 10px' }} className="mono">
                      <span style={{ color: pnlColor }}>{fmtPnl(row.total_pnl)}</span>
                    </td>
                    <td style={{ textAlign: 'right', padding: '7px 10px' }} className="mono">
                      <span style={{ color: retColor }}>{fmtReturnPct(row.return_pct)}</span>
                    </td>
                    <td style={{ textAlign: 'right', padding: '7px 10px' }} className="mono">
                      <span style={{ color: row.profit_factor == null ? 'var(--up)' : undefined }}>
                        {row.profit_factor == null ? '∞' : row.profit_factor.toFixed(2)}
                      </span>
                    </td>
                    <td style={{ textAlign: 'right', padding: '7px 10px' }} className="mono">
                      <span style={{ color: row.open_positions > 0 ? 'var(--gold)' : undefined }}>
                        {fmtNumber(row.open_positions)}
                      </span>
                    </td>
                    <td style={{ textAlign: 'right', padding: '7px 10px' }} className="mono">
                      <span style={{ color: row.unrealized_pnl >= 0 ? 'var(--up)' : 'var(--down)' }}>
                        {fmtPnl(row.unrealized_pnl)}
                      </span>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* ── Pagination ── */}
      <div style={{
        display: 'flex', flexWrap: 'wrap', gap: 12, alignItems: 'center',
        justifyContent: 'space-between', marginTop: 14, padding: '0 4px',
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12, color: 'var(--text-3)' }}>
          <span>{m.rowsPerPage}:</span>
          <select
            value={pageSize}
            onChange={(e) => { setPageSize(Number(e.target.value)); setPage(1); }}
            style={selectStyle}
          >
            {PAGE_SIZE_OPTIONS.map((n) => <option key={n} value={n}>{n}</option>)}
          </select>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <button
            className="btn btn--sm"
            disabled={page <= 1 || loading}
            onClick={() => setPage((p) => Math.max(1, p - 1))}
          >
            ‹ {m.prevPage}
          </button>
          <span style={{ fontSize: 12, color: 'var(--text-3)', fontFamily: 'var(--font-mono)', minWidth: 80, textAlign: 'center' }}>
            {m.page} {page} / {Math.max(1, totalPages)}
          </span>
          <button
            className="btn btn--sm"
            disabled={page >= totalPages || loading}
            onClick={() => setPage((p) => p + 1)}
          >
            {m.nextPage} ›
          </button>
        </div>
      </div>
    </div>
  );
}

// ── Main page ─────────────────────────────────────────────────────────────────

export default function Monitoring() {
  const { user, canAccessMarket } = useAuth();
  const { t } = useLanguage();
  const m = t.monitoring;

  const [data, setData] = useState<MonitoringOverview | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [lastRefresh, setLastRefresh] = useState<Date | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const res = await fetchMonitoringOverview();
      setData(res);
      setLastRefresh(new Date());
    } catch (e: any) {
      setError(e.message || m.error);
    } finally {
      setLoading(false);
    }
  }, [m.error]);

  useEffect(() => {
    if (user) load();
    else setLoading(false);
  }, [user, load]);

  if (!user) {
    return (
      <div className="content__inner">
        <div className="empty" style={{ padding: '80px 20px' }}>
          <div className="empty__icon"><Icon name="activity" size={20} /></div>
          <p style={{ marginTop: 12 }}>{m.loginRequired}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="content__inner">
      {/* ── Header ── */}
      <div style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        marginBottom: 20, flexWrap: 'wrap', gap: 10,
      }}>
        <div>
          <h2 style={{ fontSize: 20, fontWeight: 700, letterSpacing: '-0.3px', marginBottom: 2 }}>
            {m.pageTitle}
          </h2>
          {data && (
            <div style={{ fontSize: 12, color: 'var(--text-3)', fontFamily: 'var(--font-mono)' }}>
              {m.generatedAt}: {fmtDateTime(data.generated_at)}
              {lastRefresh && (
                <span style={{ marginLeft: 12 }}>
                  · {m.refresh.toLowerCase()}: {fmtDateTime(lastRefresh.toISOString())}
                </span>
              )}
            </div>
          )}
        </div>
        <button
          className="btn btn--sm"
          onClick={load}
          disabled={loading}
          style={{ display: 'flex', alignItems: 'center', gap: 6 }}
        >
          <Icon name="refresh" size={14} />
          {loading ? '…' : m.refresh}
        </button>
      </div>

      {/* ── Loading ── */}
      {loading && (
        <div style={{ padding: '60px 20px', textAlign: 'center', color: 'var(--text-3)' }}>
          <Icon name="activity" size={20} style={{ marginBottom: 10 }} />
          <div>{m.loading}</div>
        </div>
      )}

      {/* ── Error ── */}
      {!loading && error && (
        <div style={{
          padding: '20px', background: 'var(--down-bg)', color: 'var(--down)',
          border: '1px solid var(--down)', marginBottom: 20, fontSize: 13,
        }}>
          {m.error}: {error}
        </div>
      )}

      {/* ── Content ── */}
      {!loading && data && (
        <>
          {/* Market cards grid */}
          <div style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(480px, 1fr))',
            gap: 16,
            marginBottom: 24,
          }}>
            {data.markets.filter(market => canAccessMarket(market.market)).map((market) => (
              <MarketCard key={market.market} market={market} />
            ))}
          </div>

          {/* ── Bots Summary ── */}
          <Panel title={m.botsSummary} style={{ marginBottom: 16 }}>
            <div style={{ display: 'flex', gap: 32, flexWrap: 'wrap', marginBottom: 16, padding: '0 4px' }}>
              <div>
                <div style={{ fontSize: 11, color: 'var(--text-3)', marginBottom: 2 }}>{m.totalBots}</div>
                <div className="mono" style={{ fontSize: 22, fontWeight: 700 }}>
                  {data.bots.summary.total_bots}
                </div>
              </div>
              <div>
                <div style={{ fontSize: 11, color: 'var(--text-3)', marginBottom: 2 }}>{m.activeBots}</div>
                <div className="mono" style={{ fontSize: 22, fontWeight: 700, color: 'var(--up)' }}>
                  {data.bots.summary.active_bots}
                </div>
              </div>
            </div>

            {data.bots.summary.by_market && data.bots.summary.by_market.length > 0 ? (
              <div style={{ overflowX: 'auto' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
                  <thead>
                    <tr style={{ borderBottom: '1px solid var(--border)' }}>
                      <th style={{ textAlign: 'left', padding: '6px 10px', color: 'var(--text-3)', fontWeight: 600 }}>{m.colMarket}</th>
                      <th style={{ textAlign: 'right', padding: '6px 10px', color: 'var(--text-3)', fontWeight: 600 }}>{m.colTrades}</th>
                      <th style={{ textAlign: 'right', padding: '6px 10px', color: 'var(--text-3)', fontWeight: 600 }}>{m.colWins}</th>
                      <th style={{ textAlign: 'right', padding: '6px 10px', color: 'var(--text-3)', fontWeight: 600 }}>{m.colLosses}</th>
                      <th style={{ textAlign: 'right', padding: '6px 10px', color: 'var(--text-3)', fontWeight: 600 }}>{m.colWinRate}</th>
                      <th style={{ textAlign: 'right', padding: '6px 10px', color: 'var(--text-3)', fontWeight: 600 }}>{m.colPnl}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.bots.summary.by_market.map((row) => {
                      const color = MARKET_COLORS[row.market] || 'var(--accent)';
                      const pnlColor = row.total_pnl >= 0 ? 'var(--up)' : 'var(--down)';
                      const wrColor = row.win_rate >= 0.55 ? 'var(--up)' : row.win_rate >= 0.45 ? 'var(--gold)' : 'var(--down)';
                      return (
                        <tr key={row.market} style={{ borderBottom: '1px solid var(--border)' }}>
                          <td style={{ padding: '7px 10px' }}>
                            <span style={{ color, fontWeight: 600 }}>{row.market}</span>
                          </td>
                          <td style={{ textAlign: 'right', padding: '7px 10px' }} className="mono">
                            {fmtNumber(row.trades)}
                          </td>
                          <td style={{ textAlign: 'right', padding: '7px 10px' }} className="mono">
                            <span style={{ color: 'var(--up)' }}>{fmtNumber(row.wins)}</span>
                          </td>
                          <td style={{ textAlign: 'right', padding: '7px 10px' }} className="mono">
                            <span style={{ color: 'var(--down)' }}>{fmtNumber(row.losses)}</span>
                          </td>
                          <td style={{ textAlign: 'right', padding: '7px 10px' }} className="mono">
                            <span style={{ color: wrColor }}>{fmtPct(row.win_rate)}</span>
                          </td>
                          <td style={{ textAlign: 'right', padding: '7px 10px' }} className="mono">
                            <span style={{ color: pnlColor }}>{fmtPnl(row.total_pnl)}</span>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            ) : (
              <div style={{ color: 'var(--text-3)', fontSize: 13, padding: '12px 0' }}>{m.noBotsData}</div>
            )}
          </Panel>

          {/* ── Bots full table ── */}
          <Panel
            title={m.botsTable}
            sub={
              <span style={{ fontSize: 11, color: 'var(--text-3)', fontFamily: 'var(--font-mono)' }}>
                {t.monitoring.colWinRate} ↓ default sort
              </span>
            }
          >
            <BotsTable />
          </Panel>
        </>
      )}
    </div>
  );
}
