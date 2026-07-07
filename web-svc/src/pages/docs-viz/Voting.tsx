import { useEffect, useRef, useState, useCallback } from 'react';

// ── Types ──────────────────────────────────────────────────────────────────────

interface MarketOption {
  label: string;
  apiPath: string;
  dirParam: string;
}

const MARKETS: MarketOption[] = [
  { label: 'CRYPTO', apiPath: 'crypto', dirParam: 'CRYPTO' },
  { label: 'GOLD',   apiPath: 'gold',   dirParam: 'GOLD'   },
  { label: 'NASDAQ', apiPath: 'nasdaq', dirParam: 'NASDAQ' },
  { label: 'SP500',  apiPath: 'sp500',  dirParam: 'SP500'  },
];

const BASE_ALGOS = [
  'moving_average', 'ema', 'lstm_nn', 'arima_garch',
  'lightgbm', 'sarima', 'egarch', 'gru_nn', 'random_forest', 'xgboost',
];

const ALGO_SHORT: Record<string, string> = {
  moving_average: 'MA',
  ema: 'EMA',
  lstm_nn: 'LSTM',
  arima_garch: 'ARIMA',
  lightgbm: 'LGB',
  sarima: 'SARIMA',
  egarch: 'EGARCH',
  gru_nn: 'GRU',
  random_forest: 'RF',
  xgboost: 'XGB',
};

interface AlgoRow {
  name: string;
  shortName: string;
  predictedPrice: number;
  currentPrice: number;
  delta: number;
  vote: 1 | 0;   // 1 = tăng, 0 = giảm
  weight: number;
  contribution: number; // weight * vote
}

// ── Synthetic fallback ─────────────────────────────────────────────────────────

function makeSynthetic(): AlgoRow[] {
  const base = 92000;
  const rows: [string, number, number, number][] = [
    ['moving_average', 92450, base, 0.55],
    ['ema',            92310, base, 0.62],
    ['lstm_nn',        92800, base, 0.78],
    ['arima_garch',    91600, base, 0.41],
    ['lightgbm',       92550, base, 0.71],
    ['xgboost',        92200, base, 0.68],
    ['random_forest',  91900, base, 0.38],
    ['gru_nn',         92700, base, 0.74],
  ];
  return rows.map(([name, pred, cur, w]) => {
    const delta = pred - cur;
    const vote: 1 | 0 = delta >= 0 ? 1 : 0;
    return {
      name,
      shortName: ALGO_SHORT[name] ?? name.toUpperCase(),
      predictedPrice: pred,
      currentPrice: cur,
      delta,
      vote,
      weight: w,
      contribution: w * vote,
    };
  });
}

function fmtPrice(v: number): string {
  if (v >= 10000) return v.toFixed(0);
  if (v >= 100)   return v.toFixed(1);
  return v.toFixed(2);
}

function fmtW(v: number): string { return v.toFixed(3); }

function prefersReducedMotion(): boolean {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

// ── Props ──────────────────────────────────────────────────────────────────────

// VizBlock passes `playing` but for ownControls viz we manage it ourselves.
// We accept the prop to satisfy the interface but ignore it.
// eslint-disable-next-line @typescript-eslint/no-unused-vars
export default function Voting(_props: { playing: boolean; arg?: string }) {
  // ── Market ──────────────────────────────────────────────────────────────────
  const [marketIdx, setMarketIdx]   = useState(0);
  const [algos, setAlgos]           = useState<AlgoRow[]>(makeSynthetic);
  const [symbol, setSymbol]         = useState('');
  const [status, setStatus]         = useState<'loading' | 'live' | 'synthetic'>('synthetic');

  // ── Trace state ─────────────────────────────────────────────────────────────
  const [step, setStep]             = useState(0);       // 0 = formula; 1..n = algo row; n+1 = result
  const [autoPlay, setAutoPlay]     = useState(() => !prefersReducedMotion());
  const timerRef                    = useRef<ReturnType<typeof setInterval> | null>(null);
  const stepRef                     = useRef(step);
  stepRef.current = step;

  const totalSteps = algos.length + 1; // steps 1..n = algo rows, step n+1 = result

  // ── Fetch ───────────────────────────────────────────────────────────────────

  useEffect(() => {
    const market = MARKETS[marketIdx];
    setStatus('loading');
    setAlgos(makeSynthetic());
    setSymbol('');
    setStep(0);

    const token = localStorage.getItem('vns_token') || '';
    const headers: HeadersInit = token ? { Authorization: `Bearer ${token}` } : {};

    let cancelled = false;

    async function load() {
      try {
        const [dirRes, predRes] = await Promise.all([
          fetch(`/api/predictions/direction-accuracy?market=${market.dirParam}`, { headers }),
          fetch(`/api/${market.apiPath}/predictions/latest`, { headers }),
        ]);
        if (cancelled) return;
        if (!dirRes.ok || !predRes.ok) throw new Error('fetch failed');

        const dirData  = await dirRes.json();
        const predData = await predRes.json();
        if (cancelled) return;

        const weightMap: Record<string, number> = {};
        for (const a of (dirData?.algorithms ?? [])) {
          if (typeof a.algorithm === 'string' && !a.algorithm.endsWith('__ps')) {
            weightMap[a.algorithm] = typeof a.direction_accuracy === 'number' ? a.direction_accuracy : 0.5;
          }
        }

        const preds: Array<{
          symbol: string;
          algorithm_name: string;
          predicted_price: number;
          current_price: number;
        }> = (predData?.data ?? []).filter(
          (p: { algorithm_name?: string }) =>
            typeof p.algorithm_name === 'string' && !p.algorithm_name.endsWith('__ps')
        );

        if (preds.length === 0) throw new Error('no predictions');

        // Pick symbol with most algo coverage
        const symCounts: Record<string, number> = {};
        for (const p of preds) { symCounts[p.symbol] = (symCounts[p.symbol] ?? 0) + 1; }
        const bestSym = Object.entries(symCounts).sort((a, b) => b[1] - a[1])[0][0];

        const symPreds = preds.filter((p) => p.symbol === bestSym);
        const algoMap: Record<string, AlgoRow> = {};

        for (const p of symPreds) {
          const algoName = p.algorithm_name;
          if (!BASE_ALGOS.includes(algoName)) continue;
          // API returns decimal prices as strings — coerce and skip bad values.
          const pred = Number(p.predicted_price);
          const cur  = Number(p.current_price);
          if (!Number.isFinite(pred) || !Number.isFinite(cur)) continue;
          const delta = pred - cur;
          const vote: 1 | 0 = delta >= 0 ? 1 : 0;
          const weight = Number(weightMap[algoName] ?? 0.5);
          algoMap[algoName] = {
            name: algoName,
            shortName: ALGO_SHORT[algoName] ?? algoName.slice(0, 6).toUpperCase(),
            predictedPrice: pred,
            currentPrice: cur,
            delta,
            vote,
            weight,
            contribution: weight * vote,
          };
        }

        const result: AlgoRow[] = BASE_ALGOS.filter((n) => algoMap[n]).map((n) => algoMap[n]);
        if (result.length < 2) throw new Error('too few');

        setAlgos(result);
        setSymbol(bestSym);
        setStatus('live');
      } catch {
        if (cancelled) return;
        setAlgos(makeSynthetic());
        setSymbol('');
        setStatus('synthetic');
      }
    }

    load();
    return () => { cancelled = true; };
  }, [marketIdx]);

  // ── Auto-play ───────────────────────────────────────────────────────────────

  const clearTimer = useCallback(() => {
    if (timerRef.current !== null) {
      clearInterval(timerRef.current);
      timerRef.current = null;
    }
  }, []);

  useEffect(() => {
    if (!autoPlay || prefersReducedMotion()) {
      clearTimer();
      return;
    }
    clearTimer();
    timerRef.current = setInterval(() => {
      setStep((s) => {
        if (s >= totalSteps) { return 0; } // loop
        return s + 1;
      });
    }, 900);
    return clearTimer;
  }, [autoPlay, totalSteps, clearTimer]);

  // Reset step when algos change (new market loaded)
  useEffect(() => { setStep(0); }, [algos]);

  // ── Derived accumulator ──────────────────────────────────────────────────────

  let numAcc = 0;
  let denAcc = 0;
  const visibleRows = Math.max(0, step - 1); // step=1 reveals row 0, etc.
  for (let i = 0; i < Math.min(visibleRows, algos.length); i++) {
    numAcc += algos[i].contribution;
    denAcc += algos[i].weight;
  }
  const pFinal = denAcc > 0 ? numAcc / denAcc : 0.5;
  const showResult = step > algos.length;

  const marketOpt = MARKETS[marketIdx];

  // ── Handlers ─────────────────────────────────────────────────────────────────

  function handlePrev() {
    setAutoPlay(false);
    setStep((s) => Math.max(0, s - 1));
  }
  function handleNext() {
    setAutoPlay(false);
    setStep((s) => Math.min(totalSteps, s + 1));
  }
  function handleReset() {
    setAutoPlay(false);
    setStep(0);
  }
  function handleToggleAuto() {
    if (prefersReducedMotion()) return;
    setAutoPlay((a) => !a);
  }

  // ── Gauge angle helper ───────────────────────────────────────────────────────
  // Maps p in [0,1] to angle for a half-circle gauge (-180 to 0 deg from left)
  // We use a 180-deg arc: p=0 → angle=-180°, p=0.5 → -90°, p=1 → 0°
  function gaugeAngle(p: number): number {
    return -180 + p * 180; // degrees
  }
  function gaugePt(p: number, r: number, cx: number, cy: number) {
    const a = (gaugeAngle(p) * Math.PI) / 180;
    return { x: cx + Math.cos(a) * r, y: cy + Math.sin(a) * r };
  }

  const GCX = 80;  // gauge center x
  const GCY = 60;  // gauge center y
  const GR  = 52;  // gauge radius

  const needlePt = gaugePt(showResult ? pFinal : 0.5, GR - 8, GCX, GCY);

  return (
    <div className="trace-viz">
      {/* ── Top bar ── */}
      <div className="trace-viz__topbar">
        <div className="docs-viz__market-select" style={{ position: 'static' }}>
          <label htmlFor="voting-market-sel">Thị trường:</label>
          <select
            id="voting-market-sel"
            value={marketIdx}
            onChange={(e) => { setMarketIdx(Number(e.target.value)); setAutoPlay(false); }}
          >
            {MARKETS.map((m, i) => (
              <option key={m.apiPath} value={i}>{m.label}</option>
            ))}
          </select>
        </div>

        {status === 'live' && (
          <span className="docs-viz__data-badge docs-viz__data-badge--live" style={{ position: 'static' }}>
            Dữ liệu thật · {marketOpt.label}{symbol ? ` · ${symbol}` : ''}
          </span>
        )}
        {status === 'synthetic' && (
          <span className="docs-viz__data-badge" style={{ position: 'static' }}>
            Minh hoạ · dữ liệu giả định
          </span>
        )}
        {status === 'loading' && (
          <span className="docs-viz__data-badge" style={{ position: 'static' }}>
            Đang tải…
          </span>
        )}
      </div>

      {/* ── Formula bar ── */}
      <div className="trace-viz__formula">
        <span className="trace-viz__formula-label">Công thức:</span>
        <code className="trace-viz__formula-code">
          P(tăng) = Σ(wᵢ · vᵢ) / Σ wᵢ
        </code>
        <span className="trace-viz__formula-note">
          &nbsp;— vᵢ = 1 nếu Δᵢ &gt; 0
        </span>
      </div>

      {/* ── Step 0: instruction ── */}
      {step === 0 && (
        <div className="trace-viz__intro">
          Nhấn <strong>Sau ▶</strong> hoặc <strong>▶ Tự chạy</strong> để xem từng thuật toán tính đóng góp vào Ensemble.
        </div>
      )}

      {/* ── Main table (rows revealed one by one) ── */}
      <div className="trace-viz__table-wrap">
        <table className="trace-viz__table">
          <thead>
            <tr>
              <th>Thuật toán</th>
              <th>Dự đoán</th>
              <th>Hiện tại</th>
              <th>Δ</th>
              <th>Hướng</th>
              <th>w (acc)</th>
              <th>c = w·v</th>
            </tr>
          </thead>
          <tbody>
            {algos.map((row, i) => {
              const revealed  = i < visibleRows;
              const highlight = i === visibleRows && step > 0 && !showResult;
              const pending   = !revealed && !highlight;
              return (
                <tr
                  key={row.name}
                  className={
                    highlight ? 'trace-viz__row--active'
                    : revealed ? 'trace-viz__row--done'
                    : pending  ? 'trace-viz__row--pending'
                    : ''
                  }
                >
                  <td><strong>{row.shortName}</strong></td>
                  <td>{revealed || highlight ? fmtPrice(row.predictedPrice) : '—'}</td>
                  <td>{revealed || highlight ? fmtPrice(row.currentPrice)   : '—'}</td>
                  <td className={row.delta >= 0 ? 'trace-viz__cell--up' : 'trace-viz__cell--down'}>
                    {revealed || highlight
                      ? (row.delta >= 0 ? '+' : '') + fmtPrice(row.delta)
                      : '—'}
                  </td>
                  <td className={row.vote === 1 ? 'trace-viz__cell--up' : 'trace-viz__cell--down'}>
                    {revealed || highlight ? (row.vote === 1 ? '↑ Tăng' : '↓ Giảm') : '—'}
                  </td>
                  <td>{revealed || highlight ? fmtW(row.weight) : '—'}</td>
                  <td className="trace-viz__cell--contrib">
                    {revealed || highlight
                      ? (highlight
                          ? <span className="trace-viz__cell--flash">{fmtW(row.contribution)}</span>
                          : fmtW(row.contribution))
                      : '—'}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {/* ── Accumulator panel ── */}
      {step > 0 && (
        <div className="trace-viz__accumulator">
          <div className="trace-viz__acc-block">
            <span className="trace-viz__acc-label">Tử số N (Σ w·v)</span>
            <span className="trace-viz__acc-value">{numAcc.toFixed(3)}</span>
          </div>
          <div className="trace-viz__acc-sep">÷</div>
          <div className="trace-viz__acc-block">
            <span className="trace-viz__acc-label">Mẫu số D (Σ w)</span>
            <span className="trace-viz__acc-value">{denAcc.toFixed(3)}</span>
          </div>

          {/* Gauge */}
          {showResult && (
            <div className="trace-viz__gauge-wrap">
              <svg
                viewBox={`0 0 ${GCX * 2} ${GCY + 20}`}
                width="160"
                height="80"
                aria-label={`Gauge P(tăng) = ${(pFinal * 100).toFixed(1)}%`}
              >
                {/* Background arc */}
                <path
                  d={`M ${GCX - GR} ${GCY} A ${GR} ${GR} 0 0 1 ${GCX + GR} ${GCY}`}
                  fill="none"
                  stroke="var(--surface-3)"
                  strokeWidth="8"
                  strokeLinecap="round"
                />
                {/* Down zone (left half) */}
                <path
                  d={`M ${GCX - GR} ${GCY} A ${GR} ${GR} 0 0 1 ${GCX} ${GCY - GR}`}
                  fill="none"
                  stroke="var(--down)"
                  strokeWidth="5"
                  strokeLinecap="round"
                  opacity="0.35"
                />
                {/* Up zone (right half) */}
                <path
                  d={`M ${GCX} ${GCY - GR} A ${GR} ${GR} 0 0 1 ${GCX + GR} ${GCY}`}
                  fill="none"
                  stroke="var(--up)"
                  strokeWidth="5"
                  strokeLinecap="round"
                  opacity="0.35"
                />
                {/* Needle */}
                <line
                  x1={GCX} y1={GCY}
                  x2={needlePt.x} y2={needlePt.y}
                  stroke="var(--accent)"
                  strokeWidth="2.5"
                  strokeLinecap="round"
                />
                <circle cx={GCX} cy={GCY} r="4" fill="var(--accent)" />
                {/* P label */}
                <text
                  x={GCX} y={GCY + 16}
                  textAnchor="middle"
                  fontSize="11"
                  fontWeight="700"
                  fill={pFinal >= 0.5 ? 'var(--up)' : 'var(--down)'}
                  fontFamily="var(--font-ui)"
                >
                  {(pFinal * 100).toFixed(1)}%
                </text>
                {/* Axis labels */}
                <text x={GCX - GR - 4} y={GCY + 4} textAnchor="end" fontSize="8" fill="var(--down)" fontFamily="var(--font-ui)">0</text>
                <text x={GCX + GR + 4} y={GCY + 4} textAnchor="start" fontSize="8" fill="var(--up)"   fontFamily="var(--font-ui)">1</text>
              </svg>
            </div>
          )}
        </div>
      )}

      {/* ── Result box ── */}
      {showResult && (
        <div className="trace-viz__result">
          <div className="trace-viz__result-equation">
            P(tăng) = {numAcc.toFixed(3)} / {denAcc.toFixed(3)} ={' '}
            <strong className={pFinal >= 0.5 ? 'trace-viz__cell--up' : 'trace-viz__cell--down'}>
              {pFinal.toFixed(3)}
            </strong>
          </div>
          <div className="trace-viz__result-interp">
            {pFinal > 0.5
              ? `Phe tăng thắng thế (P = ${(pFinal * 100).toFixed(1)}% > 50%) — Ensemble dự đoán TĂNG.`
              : pFinal < 0.5
              ? `Phe giảm thắng thế (P = ${(pFinal * 100).toFixed(1)}% < 50%) — Ensemble dự đoán GIẢM.`
              : 'Hai phe cân bằng (P = 50%) — Ensemble giữ nguyên trạng thái.'}
          </div>
        </div>
      )}

      {/* ── Controls ── */}
      <div className="trace-viz__controls">
        <button
          className="trace-viz__ctrl-btn"
          onClick={handlePrev}
          disabled={step === 0}
          aria-label="Bước trước"
        >
          ◀ Trước
        </button>

        <button
          className={`trace-viz__ctrl-btn${autoPlay ? ' trace-viz__ctrl-btn--active' : ''}`}
          onClick={handleToggleAuto}
          aria-label={autoPlay ? 'Tạm dừng tự chạy' : 'Tự chạy'}
          title={prefersReducedMotion() ? 'Tắt do prefers-reduced-motion' : ''}
          disabled={prefersReducedMotion()}
        >
          {autoPlay ? '⏸ Tạm dừng' : '▶ Tự chạy'}
        </button>

        <button
          className="trace-viz__ctrl-btn"
          onClick={handleNext}
          disabled={step >= totalSteps}
          aria-label="Bước tiếp theo"
        >
          Sau ▶
        </button>

        <button
          className="trace-viz__ctrl-btn"
          onClick={handleReset}
          aria-label="Đặt lại"
        >
          ⟲ Đặt lại
        </button>

        <span className="trace-viz__progress">
          {step === 0
            ? 'Bước 0 / ' + totalSteps
            : showResult
            ? `Bước ${totalSteps} / ${totalSteps} — KẾT QUẢ`
            : `Bước ${step} / ${totalSteps}`}
        </span>
      </div>
    </div>
  );
}
