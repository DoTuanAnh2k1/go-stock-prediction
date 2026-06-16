import { useState, useEffect } from 'react';
import { useParams, NavLink } from 'react-router-dom';
import { useData } from '../context/DataContext';
import { Panel, Icon, Seg, MarketTabs } from '../components/ui';
import { LineChart } from '../components/charts';
import { fetchMarketPredictions } from '../api';
import { useLanguage } from '../context/LangContext';

// ── Constants ─────────────────────────────────────────────────────────────────
const ALGO_COLORS_MAP: Record<string, string> = {
  ema:            'oklch(0.72 0.18 150)',
  lstm_nn:        'oklch(0.74 0.13 200)',
  arima_garch:    'var(--gold)',
  moving_average: 'var(--up)',
  ensemble:       'oklch(0.72 0.14 300)',
};

const ALGO_ORDER_DETAIL = ['ema', 'lstm_nn', 'arima_garch', 'moving_average', 'ensemble'];

const GOLD_INSTRUMENTS = [
  { key: 'XAU', label: 'XAU/USD (Spot)', search: 'XAU' },
  { key: 'BTMC_SJC', label: 'BTMC/SJC', search: 'SJC' },
  { key: 'BTMC_NHAN', label: 'BTMC/Nhẫn tròn', search: 'nhan' },
];

const NASDAQ_INSTRUMENTS = [
  { key: 'QQQ',  label: 'QQQ'  },
  { key: 'AAPL', label: 'AAPL' },
  { key: 'MSFT', label: 'MSFT' },
  { key: 'NVDA', label: 'NVDA' },
  { key: 'GOOGL', label: 'GOOGL' },
  { key: 'AMZN', label: 'AMZN' },
  { key: 'META', label: 'META' },
  { key: 'TSLA', label: 'TSLA' },
];

const CRYPTO_INSTRUMENTS = [
  { key: 'bitcoin',  label: 'Bitcoin (BTC)'  },
  { key: 'ethereum', label: 'Ethereum (ETH)' },
];

// PERIOD_OPTIONS is computed inside the component using t for i18n

// ── Main component ────────────────────────────────────────────────────────────
export default function MarketDetail() {
  const { marketKey = 'gold' } = useParams<{ marketKey: string }>();
  const { data: D } = useData();
  const { t } = useLanguage();

  const PERIOD_OPTIONS = [
    { value: '30d', label: t.marketDetail.periodOptions.d30 },
    { value: '60d', label: t.marketDetail.periodOptions.d60 },
    { value: '90d', label: t.marketDetail.periodOptions.d90 },
  ];

  const isGold   = marketKey === 'gold';
  const isNasdaq = marketKey === 'nasdaq100';
  const isCrypto = marketKey === 'crypto';

  // Default symbol per market
  const defaultSymbol = isGold ? 'XAU'
    : isNasdaq ? 'QQQ'
    : isCrypto ? 'bitcoin'
    : 'VCB';

  const [symbol, setSymbol] = useState(defaultSymbol);
  const [period, setPeriod] = useState('30d');
  const [rows, setRows] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Reset symbol when market changes
  useEffect(() => {
    setSymbol(
      isGold ? 'XAU'
      : isNasdaq ? 'QQQ'
      : isCrypto ? 'bitcoin'
      : 'VCB'
    );
  }, [marketKey]); // eslint-disable-line react-hooks/exhaustive-deps

  // Get search term - gold uses a search alias, other markets use the key directly
  const searchTerm = isGold
    ? (GOLD_INSTRUMENTS.find(g => g.key === symbol)?.search ?? symbol)
    : symbol;

  // Fetch all predictions for the symbol (large limit, sorted asc)
  useEffect(() => {
    setLoading(true);
    setError(null);
    fetchMarketPredictions(marketKey, {
      search: searchTerm || undefined,
      limit: 500,
      sort_by: 'prediction_date',
      sort_dir: 'asc',
    }).then((res) => {
      setRows(Array.isArray(res.data) ? res.data : []);
    }).catch((e) => {
      setError(t.marketDetail.cannotLoad + ': ' + (e?.message || t.marketDetail.unknownError));
      setRows([]);
    }).finally(() => setLoading(false));
  }, [marketKey, searchTerm]);

  // ── Data processing ─────────────────────────────────────────────────────────
  const periodDays = period === '30d' ? 30 : period === '60d' ? 60 : 90;
  const cutoff = new Date();
  cutoff.setDate(cutoff.getDate() - periodDays);
  const cutoffStr = cutoff.toISOString().slice(0, 10);

  const dateSet = new Set<string>();
  const algoRows: Record<string, Record<string, number>> = {};
  const actualByDate: Record<string, number> = {};

  rows.forEach(row => {
    const date = (row.prediction_date || '').slice(0, 10);
    if (!date) return;
    dateSet.add(date);
    const algo = (row.algorithm_name || row.algorithm || '').toLowerCase();
    if (algo) {
      if (!algoRows[algo]) algoRows[algo] = {};
      algoRows[algo][date] = parseFloat(row.predicted_price) || 0;
    }
    if (row.status === 'confirmed' && row.actual_price != null) {
      actualByDate[date] = parseFloat(row.actual_price) || 0;
    }
  });

  const dates = Array.from(dateSet).sort().filter(d => d >= cutoffStr);

  // ── Chart series ────────────────────────────────────────────────────────────
  const series: { name: string; data: (number | null)[]; color: string; w: number; dash?: string }[] = [];

  const actualData = dates.map(d => actualByDate[d] ?? null);
  if (actualData.some(v => v != null)) {
    series.push({ name: t.common.actual, data: actualData, color: 'var(--text)', w: 2.2 });
  }

  ALGO_ORDER_DETAIL.forEach(algoKey => {
    if (!algoRows[algoKey]) return;
    const data = dates.map(d => algoRows[algoKey][d] ?? null);
    if (data.some(v => v != null)) {
      const algoInfo = D.algos.find(a => a.id === algoKey);
      series.push({
        name: algoInfo?.short || algoKey.toUpperCase().slice(0, 5),
        data,
        color: ALGO_COLORS_MAP[algoKey] || 'var(--text-3)',
        w: 1.8,
        dash: '4,3',
      });
    }
  });

  const chartLabels = dates.map(d => {
    const [, m, day] = d.split('-');
    return day + '/' + m;
  });

  const fmtPrice = isGold
    ? (n: number) => n >= 1e6 ? (n / 1e6).toFixed(1) + 'tr' : n.toLocaleString('vi-VN')
    : (isNasdaq || isCrypto)
    ? (n: number) => '$' + n.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
    : (n: number) => n.toLocaleString('vi-VN', { minimumFractionDigits: 2, maximumFractionDigits: 2 });

  // Display label for the selected symbol
  const symbolLabel = isGold
    ? (GOLD_INSTRUMENTS.find(g => g.key === symbol)?.label ?? symbol)
    : isNasdaq
    ? (NASDAQ_INSTRUMENTS.find(i => i.key === symbol)?.label ?? symbol)
    : isCrypto
    ? (CRYPTO_INSTRUMENTS.find(i => i.key === symbol)?.label ?? symbol)
    : symbol;

  const stockOptions = D.stocks.length > 0
    ? D.stocks.map(s => ({ key: s.sym, label: s.sym }))
    : [{ key: 'VCB', label: 'VCB' }];

  return (
    <div className="content__inner fade">
      <MarketTabs marketKey={marketKey} />

      {/* Controls */}
      <Panel className="section-gap" style={{ marginTop: 16 }}>
        <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'center' }}>
          <Icon name="filter" size={15} style={{ color: 'var(--text-3)' }} />
          {isGold ? (
            <select className="sel" value={symbol} onChange={e => setSymbol(e.target.value)}>
              {GOLD_INSTRUMENTS.map(g => (
                <option key={g.key} value={g.key}>{g.label}</option>
              ))}
            </select>
          ) : isNasdaq ? (
            <select className="sel" value={symbol} onChange={e => setSymbol(e.target.value)}>
              {NASDAQ_INSTRUMENTS.map(i => (
                <option key={i.key} value={i.key}>{i.label}</option>
              ))}
            </select>
          ) : isCrypto ? (
            <select className="sel" value={symbol} onChange={e => setSymbol(e.target.value)}>
              {CRYPTO_INSTRUMENTS.map(i => (
                <option key={i.key} value={i.key}>{i.label}</option>
              ))}
            </select>
          ) : (
            <select className="sel" value={symbol} onChange={e => setSymbol(e.target.value)}>
              {stockOptions.map(s => (
                <option key={s.key} value={s.key}>{s.label}</option>
              ))}
            </select>
          )}
          <Seg
            options={PERIOD_OPTIONS}
            value={period}
            onChange={setPeriod}
          />
        </div>
      </Panel>

      {/* Chart */}
      <Panel flush className="section-gap">
        <div style={{ padding: '16px 16px 8px', borderBottom: '1px solid var(--border)', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <span style={{ fontWeight: 600, fontSize: 14 }}>
            {t.marketDetail.algoComparison} — {symbolLabel}
          </span>
          {loading && (
            <span style={{ fontSize: 12, color: 'var(--text-3)', fontFamily: 'var(--font-mono)' }}>
              {t.marketDetail.loading}
            </span>
          )}
        </div>

        {error && (
          <div style={{ padding: '16px', color: 'var(--down)', fontSize: 13, display: 'flex', gap: 8, alignItems: 'center' }}>
            <Icon name="layers" size={15} />{error}
          </div>
        )}

        {!error && series.length === 0 && !loading ? (
          <div className="empty" style={{ padding: '60px 20px' }}>
            <div className="empty__icon"><Icon name="layers" size={18} /></div>
            <p>{t.marketDetail.noCompareData} {symbolLabel}</p>
          </div>
        ) : (
          <div style={{ padding: '16px 0 8px', opacity: loading ? 0.5 : 1, transition: 'opacity .15s' }}>
            <LineChart
              series={series}
              labels={chartLabels}
              height={560}
              yFmt={fmtPrice}
              valueFmt={fmtPrice}
            />
          </div>
        )}

        {/* Legend */}
        {series.length > 0 && (
          <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap', padding: '8px 16px 16px' }}>
            {series.map(s => (
              <span key={s.name} style={{ color: s.color, fontSize: 12, fontFamily: 'var(--font-mono)', fontWeight: 600, display: 'flex', alignItems: 'center', gap: 4 }}>
                ● {s.name}
              </span>
            ))}
          </div>
        )}
      </Panel>
    </div>
  );
}
