import { useEffect, useRef, useState, useCallback } from 'react';

// ── Market options ─────────────────────────────────────────────────────────────

interface MarketOption {
  label: string;
  apiPath: string;
}

const MARKETS: MarketOption[] = [
  { label: 'CRYPTO', apiPath: 'crypto' },
  { label: 'GOLD',   apiPath: 'gold'   },
  { label: 'NASDAQ', apiPath: 'nasdaq' },
  { label: 'SP500',  apiPath: 'sp500'  },
];

// Window size N
const WIN = 5;
// How many slide steps to show
const SLIDE = 2;

// ── Synthetic fallback ─────────────────────────────────────────────────────────

const SYNTH_RAW: number[] = [
  91500, 91800, 92100, 91750, 92400, 92650, 92300, 92900,
  93100, 92800, 93400,
];

function prefersReducedMotion(): boolean {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

function sampleTo(arr: number[], maxN: number): number[] {
  if (arr.length <= maxN) return arr;
  const step = arr.length / maxN;
  return Array.from({ length: maxN }, (_, i) => arr[Math.floor(i * step)]);
}

function fmtPrice(v: number): string {
  if (v >= 10000) return v.toFixed(0);
  if (v >= 100)   return v.toFixed(2);
  return v.toFixed(3);
}

function normalize(prices: number[]): number[] {
  const mn = Math.min(...prices);
  const mx = Math.max(...prices);
  const range = mx - mn || 1;
  return prices.map((v) => (v - mn) / range);
}

// ── Component ──────────────────────────────────────────────────────────────────

// eslint-disable-next-line @typescript-eslint/no-unused-vars
export default function SlidingWindow(_props: { playing: boolean; arg?: string }) {
  const [marketIdx, setMarketIdx]   = useState(0);
  const [rawPrices, setRawPrices]   = useState<number[]>(SYNTH_RAW);
  const [status, setStatus]         = useState<'loading' | 'live' | 'synthetic'>('synthetic');

  // Trace state
  const [winOffset, setWinOffset]   = useState(0);
  const [step, setStep]             = useState(0);   // 0=intro, 1..WIN=sum, WIN+1=result
  const [slidesDone, setSlidesDone] = useState(0);

  const [autoPlay, setAutoPlay]     = useState(() => !prefersReducedMotion());

  // Refs for auto-play interval to avoid stale closure over state
  const winOffsetRef  = useRef(winOffset);
  const stepRef       = useRef(step);
  const slidesDoneRef = useRef(slidesDone);
  const rawPricesRef  = useRef(rawPrices);
  winOffsetRef.current  = winOffset;
  stepRef.current       = step;
  slidesDoneRef.current = slidesDone;
  rawPricesRef.current  = rawPrices;

  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // ── Fetch ─────────────────────────────────────────────────────────────────────

  useEffect(() => {
    const market = MARKETS[marketIdx];
    setStatus('loading');

    const token = localStorage.getItem('vns_token') || '';
    const headers: HeadersInit = token ? { Authorization: `Bearer ${token}` } : {};
    let cancelled = false;

    async function load() {
      try {
        const res = await fetch(`/api/${market.apiPath}/chart?days=2`, { headers });
        if (!res.ok) throw new Error('fetch failed');
        const data = await res.json();
        if (cancelled) return;

        // closes/prices may contain nulls or string decimals — coerce and drop non-finite.
        const rawSrc: unknown[] = (data?.closes ?? data?.prices ?? []) as unknown[];
        const raw: number[] = rawSrc.map(Number).filter((v) => Number.isFinite(v) && v > 0);
        if (raw.length < WIN + SLIDE + 1) throw new Error('too few points');

        const trimmed = sampleTo(raw, 20);
        setRawPrices(trimmed);
        setStatus('live');
      } catch {
        if (cancelled) return;
        setRawPrices(SYNTH_RAW);
        setStatus('synthetic');
      }
    }

    load();
    return () => { cancelled = true; };
  }, [marketIdx]);

  // Reset trace on data change
  useEffect(() => {
    setStep(0);
    setWinOffset(0);
    setSlidesDone(0);
  }, [rawPrices]);

  // ── Derived values ────────────────────────────────────────────────────────────

  const maxOffset      = Math.max(0, rawPrices.length - WIN - 1);
  const effectiveSlide = Math.min(SLIDE, maxOffset);

  const windowPrices  = rawPrices.slice(winOffset, winOffset + WIN);
  const nextPrice     = rawPrices[winOffset + WIN] ?? null;

  let sumAcc = 0;
  const addedCount = Math.min(step, WIN);
  for (let i = 0; i < addedCount; i++) sumAcc += windowPrices[i];

  const maVal         = windowPrices.reduce((s, v) => s + v, 0) / WIN;
  const inSumPhase    = step >= 1 && step <= WIN;
  const inResultPhase = step === WIN + 1;
  const canSlide      = inResultPhase && slidesDone < effectiveSlide && nextPrice !== null;

  // ── Tick function (used by auto-play) ─────────────────────────────────────────

  const tick = useCallback(() => {
    const prices = rawPricesRef.current;
    const maxOff = Math.max(0, prices.length - WIN - 1);
    const effSl  = Math.min(SLIDE, maxOff);
    const s      = stepRef.current;
    const wo     = winOffsetRef.current;
    const sd     = slidesDoneRef.current;

    if (s < WIN + 1) {
      // Still accumulating or hitting result
      setStep(s + 1);
    } else {
      // At result — try slide
      if (sd < effSl && wo + 1 <= maxOff) {
        setWinOffset(wo + 1);
        setSlidesDone(sd + 1);
        setStep(1); // restart sum phase in new window
      } else {
        // Loop: reset everything
        setWinOffset(0);
        setSlidesDone(0);
        setStep(0);
      }
    }
  }, []); // no deps — reads from refs

  // ── Auto-play ────────────────────────────────────────────────────────────────

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
    timerRef.current = setInterval(tick, 900);
    return clearTimer;
  }, [autoPlay, tick, clearTimer]);

  // ── Handlers ─────────────────────────────────────────────────────────────────

  function handleNext() {
    setAutoPlay(false);
    if (step < WIN + 1) {
      setStep(step + 1);
    } else if (canSlide) {
      setWinOffset(winOffset + 1);
      setSlidesDone(slidesDone + 1);
      setStep(1);
    }
  }

  function handlePrev() {
    setAutoPlay(false);
    if (step > 0) {
      setStep(step - 1);
    } else if (winOffset > 0) {
      setWinOffset(winOffset - 1);
      setSlidesDone(Math.max(0, slidesDone - 1));
      setStep(WIN + 1);
    }
  }

  function handleReset() {
    setAutoPlay(false);
    setStep(0);
    setWinOffset(0);
    setSlidesDone(0);
  }

  function handleToggleAuto() {
    if (prefersReducedMotion()) return;
    setAutoPlay((a) => !a);
  }

  // ── Chart rendering ───────────────────────────────────────────────────────────

  const normed = normalize(rawPrices);
  const N      = rawPrices.length;
  const SVG_W  = 500;
  const SVG_H  = 120;
  const PAD_B  = 20;
  const BAR_H  = SVG_H - PAD_B;
  const barW   = SVG_W / N;

  const mn = Math.min(...rawPrices);
  const mx = Math.max(...rawPrices);
  const maDotIdx = winOffset + (WIN - 1) / 2;
  const maDotX   = (maDotIdx + 0.5) * barW;
  const maDotY   = BAR_H - ((maVal - mn) / (mx - mn || 1)) * BAR_H * 0.85 - 4;

  const marketOpt = MARKETS[marketIdx];

  return (
    <div className="trace-viz">
      {/* ── Top bar ── */}
      <div className="trace-viz__topbar">
        <div className="docs-viz__market-select" style={{ position: 'static' }}>
          <label htmlFor="sw-market-sel">Thị trường:</label>
          <select
            id="sw-market-sel"
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
            Dữ liệu thật · {marketOpt.label}
          </span>
        )}
        {status === 'synthetic' && (
          <span className="docs-viz__data-badge" style={{ position: 'static' }}>
            Minh hoạ · dữ liệu giả định
          </span>
        )}
        {status === 'loading' && (
          <span className="docs-viz__data-badge" style={{ position: 'static' }}>Đang tải…</span>
        )}
      </div>

      {/* ── Formula bar ── */}
      <div className="trace-viz__formula">
        <span className="trace-viz__formula-label">Công thức:</span>
        <code className="trace-viz__formula-code">
          MA = (p₁ + p₂ + p₃ + p₄ + p₅) / {WIN}
        </code>
      </div>

      {/* ── Mini price chart ── */}
      <div className="trace-viz__chart-wrap">
        <svg
          viewBox={`0 0 ${SVG_W} ${SVG_H}`}
          width="100%"
          style={{ display: 'block', height: SVG_H }}
          aria-label="Biểu đồ giá với cửa sổ MA"
        >
          {/* Price bars */}
          {normed.map((v, i) => {
            const inWin = i >= winOffset && i < winOffset + WIN;
            const barH  = v * BAR_H * 0.85 + BAR_H * 0.08;
            return (
              <rect
                key={i}
                x={i * barW + 1}
                y={BAR_H - barH}
                width={barW - 2}
                height={barH}
                fill={inWin ? 'var(--accent)' : 'var(--surface-3)'}
                opacity={inWin ? 0.7 : 0.4}
                rx={1}
              />
            );
          })}

          {/* Window outline */}
          <rect
            x={winOffset * barW} y={0}
            width={WIN * barW} height={BAR_H}
            fill="var(--accent)" opacity={0.1} rx={2}
          />
          <rect
            x={winOffset * barW} y={0}
            width={WIN * barW} height={BAR_H}
            fill="none"
            stroke="var(--accent)" strokeWidth="1.5" strokeDasharray="4 3" rx={2}
          />

          {/* MA dot */}
          {inResultPhase && (
            <circle
              cx={maDotX} cy={maDotY} r={5}
              fill="var(--accent)" stroke="var(--bg)" strokeWidth="2"
            />
          )}

          {/* Bar index labels for window */}
          {windowPrices.map((_, i) => (
            <text
              key={i}
              x={(winOffset + i + 0.5) * barW} y={SVG_H - 4}
              textAnchor="middle" fontSize="9"
              fill={i < addedCount || inResultPhase ? 'var(--accent)' : 'var(--text-3)'}
              fontFamily="var(--font-ui)"
            >
              p{winOffset + i + 1}
            </text>
          ))}

          {/* MA value label */}
          {inResultPhase && (
            <text
              x={maDotX + 8} y={maDotY - 6}
              fontSize="9" fill="var(--accent)"
              fontFamily="var(--font-ui)" fontWeight="600"
            >
              MA={fmtPrice(maVal)}
            </text>
          )}
        </svg>
        <p className="trace-viz__chart-note">
          Bản chạy thật minh hoạ trung bình lõi; production dùng VWMA (trọng số theo volume) + RSI/StochRSI.
        </p>
      </div>

      {/* ── Step 0: intro ── */}
      {step === 0 && (
        <div className="trace-viz__intro">
          Nhấn <strong>Sau ▶</strong> hoặc <strong>▶ Tự chạy</strong> để xem từng giá được cộng vào tổng.
        </div>
      )}

      {/* ── Sum trace table ── */}
      {(inSumPhase || inResultPhase) && (
        <div className="trace-viz__sum-trace">
          <table className="trace-viz__table">
            <thead>
              <tr>
                <th>Bước</th>
                <th>Giá</th>
                <th>Phép tính</th>
                <th>Σ tích luỹ</th>
              </tr>
            </thead>
            <tbody>
              {windowPrices.map((price, i) => {
                const revealed  = i < addedCount;
                const highlight = i === addedCount && inSumPhase;
                const pending   = !revealed && !highlight;
                const runSum    = windowPrices.slice(0, i + 1).reduce((s, v) => s + v, 0);
                return (
                  <tr
                    key={i}
                    className={
                      highlight ? 'trace-viz__row--active'
                      : revealed ? 'trace-viz__row--done'
                      : pending  ? 'trace-viz__row--pending'
                      : ''
                    }
                  >
                    <td>Bước {i + 1}</td>
                    <td>
                      {revealed || highlight
                        ? <strong>p{winOffset + i + 1} = {fmtPrice(price)}</strong>
                        : `p${winOffset + i + 1}`}
                    </td>
                    <td>
                      {revealed || highlight
                        ? (highlight
                            ? <span className="trace-viz__cell--flash">Σ + {fmtPrice(price)}</span>
                            : `Σ + ${fmtPrice(price)}`)
                        : '—'}
                    </td>
                    <td>
                      {revealed || highlight ? fmtPrice(runSum) : '—'}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>

          {/* Running accumulator */}
          <div className="trace-viz__accumulator" style={{ marginTop: '8px' }}>
            <div className="trace-viz__acc-block">
              <span className="trace-viz__acc-label">Σ hiện tại</span>
              <span className="trace-viz__acc-value">{fmtPrice(sumAcc)}</span>
            </div>
            <div className="trace-viz__acc-sep">/</div>
            <div className="trace-viz__acc-block">
              <span className="trace-viz__acc-label">N</span>
              <span className="trace-viz__acc-value">{WIN}</span>
            </div>
            {inResultPhase && (
              <>
                <div className="trace-viz__acc-sep">=</div>
                <div className="trace-viz__acc-block">
                  <span className="trace-viz__acc-label">MA</span>
                  <span className="trace-viz__acc-value" style={{ color: 'var(--accent)', fontWeight: 700 }}>
                    {fmtPrice(maVal)}
                  </span>
                </div>
              </>
            )}
          </div>
        </div>
      )}

      {/* ── Result ── */}
      {inResultPhase && (
        <div className="trace-viz__result">
          <div className="trace-viz__result-equation">
            MA = ({windowPrices.map((p, i) => (
              <span key={i}>{i > 0 ? ' + ' : ''}{fmtPrice(p)}</span>
            ))}) / {WIN}
            {' = '}
            <strong className="trace-viz__cell--up">{fmtPrice(maVal)}</strong>
          </div>
          {canSlide ? (
            <div className="trace-viz__result-interp">
              Trượt cửa sổ 1 nhịp: bỏ p{winOffset + 1} ({fmtPrice(windowPrices[0])}),
              thêm p{winOffset + WIN + 1} ({nextPrice !== null ? fmtPrice(nextPrice) : '?'}).
              Nhấn <strong>Sau ▶</strong> để tiếp tục.
            </div>
          ) : (
            <div className="trace-viz__result-interp">
              {slidesDone > 0
                ? `Đã trượt cửa sổ ${slidesDone} lần — đường MA hình thành từ chuỗi các giá trị này.`
                : 'Trung bình này là 1 điểm trên đường MA. Cửa sổ trượt qua toàn bộ chuỗi tạo ra đường MA.'}
            </div>
          )}
        </div>
      )}

      {/* ── Controls ── */}
      <div className="trace-viz__controls">
        <button
          className="trace-viz__ctrl-btn"
          onClick={handlePrev}
          disabled={step === 0 && winOffset === 0}
          aria-label="Bước trước"
        >
          ◀ Trước
        </button>

        <button
          className={`trace-viz__ctrl-btn${autoPlay ? ' trace-viz__ctrl-btn--active' : ''}`}
          onClick={handleToggleAuto}
          aria-label={autoPlay ? 'Tạm dừng' : 'Tự chạy'}
          disabled={prefersReducedMotion()}
          title={prefersReducedMotion() ? 'Tắt do prefers-reduced-motion' : ''}
        >
          {autoPlay ? '⏸ Tạm dừng' : '▶ Tự chạy'}
        </button>

        <button
          className="trace-viz__ctrl-btn"
          onClick={handleNext}
          disabled={inResultPhase && !canSlide}
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
            ? `Cửa sổ ${winOffset + 1} · Bước 0 / ${WIN + 1}`
            : inResultPhase
            ? `Cửa sổ ${winOffset + 1} · MA = ${fmtPrice(maVal)}`
            : `Cửa sổ ${winOffset + 1} · Bước ${step} / ${WIN + 1} · cộng p${winOffset + step}`}
          {effectiveSlide > 0 && ` · Trượt ${slidesDone}/${effectiveSlide}`}
        </span>
      </div>

      {/* Screen-reader live region */}
      <div aria-live="polite" className="trace-viz__sr-only">
        {inSumPhase
          ? `Bước ${step}: cộng p${winOffset + step} = ${fmtPrice(windowPrices[step - 1])}, tổng = ${fmtPrice(sumAcc)}`
          : inResultPhase
          ? `Kết quả: MA = ${fmtPrice(maVal)}`
          : ''}
      </div>
    </div>
  );
}
