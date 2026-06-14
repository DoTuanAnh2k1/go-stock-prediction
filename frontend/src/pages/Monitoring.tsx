import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { Panel, Icon } from '../components/ui';
import { useAuth } from '../context/AuthContext';
import { useLanguage } from '../context/LangContext';
import {
  fetchMonitoringOverview,
  type MonitoringOverview,
  type MonitoringMarket,
  type MonitoringBotRow,
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
              {crawl.stale && (
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

type BotSortKey = 'win_rate' | 'total_pnl' | 'return_pct' | 'profit_factor' | 'trades';
type SortDir = 'asc' | 'desc';

// ── Bots Table ────────────────────────────────────────────────────────────────

function BotsTable({ rows }: { rows: MonitoringBotRow[] }) {
  const { t } = useLanguage();
  const m = t.monitoring;
  const [sortKey, setSortKey] = useState<BotSortKey>('win_rate');
  const [sortDir, setSortDir] = useState<SortDir>('desc');

  const sorted = useMemo(() => {
    return [...rows].sort((a, b) => {
      const av = a[sortKey] as number;
      const bv = b[sortKey] as number;
      return sortDir === 'desc' ? bv - av : av - bv;
    });
  }, [rows, sortKey, sortDir]);

  function handleSort(key: BotSortKey) {
    if (sortKey === key) {
      setSortDir((d) => (d === 'desc' ? 'asc' : 'desc'));
    } else {
      setSortKey(key);
      setSortDir('desc');
    }
  }

  function SortHeader({ colKey, label }: { colKey: BotSortKey; label: React.ReactNode }) {
    const active = sortKey === colKey;
    return (
      <th
        onClick={() => handleSort(colKey)}
        style={{
          textAlign: 'right', padding: '7px 10px',
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

  if (!rows.length) {
    return (
      <div style={{ padding: '32px', textAlign: 'center', color: 'var(--text-3)' }}>
        {m.noBotsTable}
      </div>
    );
  }

  return (
    <div style={{ overflowX: 'auto' }}>
      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
        <thead>
          <tr style={{ borderBottom: '1px solid var(--border)' }}>
            <th style={{ textAlign: 'left', padding: '7px 10px', color: 'var(--text-3)', fontWeight: 600 }}>{m.colBotId}</th>
            <th style={{ textAlign: 'left', padding: '7px 10px', color: 'var(--text-3)', fontWeight: 600 }}>{m.colMarket}</th>
            <th style={{ textAlign: 'left', padding: '7px 10px', color: 'var(--text-3)', fontWeight: 600 }}>{m.colAlgoBot}</th>
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
          </tr>
        </thead>
        <tbody>
          {sorted.map((row) => {
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
                  {row.profit_factor.toFixed(2)}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

// ── Main page ─────────────────────────────────────────────────────────────────

export default function Monitoring() {
  const { user } = useAuth();
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
            {data.markets.map((market) => (
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
            <BotsTable rows={data.bots.table} />
          </Panel>
        </>
      )}
    </div>
  );
}
