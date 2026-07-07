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

// ── Algorithm family classification ──────────────────────────────────────────

type AlgoFamily = 'neural' | 'tree' | 'statistical' | 'ensemble' | 'rl' | 'unknown';

function getAlgoFamily(key: string): AlgoFamily {
  if (['lstm_nn', 'gru_nn', 'transformer_nn'].includes(key)) return 'neural';
  if (['lightgbm', 'xgboost', 'random_forest'].includes(key)) return 'tree';
  if (['arima_garch', 'egarch', 'sarima', 'ema', 'moving_average'].includes(key)) return 'statistical';
  if (key === 'ensemble') return 'ensemble';
  if (key === 'rl_dqn') return 'rl';
  return 'unknown';
}

function getFamilyLabel(family: AlgoFamily): string {
  switch (family) {
    case 'neural': return 'Mạng nơ-ron (Neural Network)';
    case 'tree': return 'Cây quyết định / Boosting';
    case 'statistical': return 'Mô hình thống kê';
    case 'ensemble': return 'Ensemble (bỏ phiếu có trọng số)';
    case 'rl': return 'Reinforcement Learning (DQN)';
    default: return 'Mô hình dự đoán';
  }
}

function getFamilyDesc(family: AlgoFamily): string {
  switch (family) {
    case 'neural':
      return 'Chuỗi giá nạp tuần tự qua các lớp nơ-ron — mô hình học mẫu hình phức tạp từ lịch sử.';
    case 'tree':
      return 'Cây quyết định học từ ~30 đặc trưng kỹ thuật — mỗi nút tách dữ liệu theo ngưỡng.';
    case 'statistical':
      return 'Mô hình hoá chuỗi thời gian bằng phương trình toán học — phân tích xu hướng & biến động.';
    case 'ensemble':
      return 'Tổng hợp dự đoán từ 10 thuật toán cơ sở theo trọng số độ chính xác hướng.';
    case 'rl':
      return 'Agent học chiến lược qua thưởng/phạt — quan sát → hành động → cập nhật Q-values.';
    default:
      return 'Mô hình học máy dự đoán giá dựa trên chuỗi lịch sử.';
  }
}

// ── Price formatting ───────────────────────────────────────────────────────────

function fmtPrice(v: number): string {
  if (v >= 10000) return v.toLocaleString('vi-VN', { maximumFractionDigits: 0 });
  if (v >= 100)   return v.toFixed(2);
  return v.toFixed(4);
}

function fmtDelta(d: number): string {
  const sign = d >= 0 ? '+' : '';
  if (Math.abs(d) >= 10000) return sign + d.toFixed(0);
  if (Math.abs(d) >= 1)     return sign + d.toFixed(2);
  return sign + d.toFixed(4);
}

function fmtPct(d: number, cur: number): string {
  if (cur === 0) return '';
  const pct = (d / cur) * 100;
  const sign = pct >= 0 ? '+' : '';
  return `${sign}${pct.toFixed(2)}%`;
}

// ── Sparkline SVG ─────────────────────────────────────────────────────────────

function Sparkline({ prices }: { prices: number[] }) {
  if (prices.length < 2) return null;

  const W = 240;
  const H = 60;
  const PAD = 4;
  const mn = Math.min(...prices);
  const mx = Math.max(...prices);
  const range = mx - mn || 1;

  const pts = prices.map((v, i) => {
    const x = PAD + (i / (prices.length - 1)) * (W - PAD * 2);
    const y = H - PAD - ((v - mn) / range) * (H - PAD * 2);
    return `${x},${y}`;
  });

  const lastX = PAD + ((prices.length - 1) / (prices.length - 1)) * (W - PAD * 2);
  const lastY = H - PAD - ((prices[prices.length - 1] - mn) / range) * (H - PAD * 2);

  const polyPts = pts.join(' ');

  // fill path
  const fillPts = [
    `${PAD},${H - PAD}`,
    ...pts,
    `${lastX},${H - PAD}`,
  ].join(' ');

  return (
    <svg
      viewBox={`0 0 ${W} ${H}`}
      width="100%"
      style={{ display: 'block', height: H }}
      aria-label="Biểu đồ giá đầu vào"
    >
      <defs>
        <linearGradient id="spark-fill" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="var(--accent)" stopOpacity="0.18" />
          <stop offset="100%" stopColor="var(--accent)" stopOpacity="0.02" />
        </linearGradient>
      </defs>
      {/* Fill area */}
      <polygon points={fillPts} fill="url(#spark-fill)" />
      {/* Line */}
      <polyline
        points={polyPts}
        fill="none"
        stroke="var(--accent)"
        strokeWidth="1.5"
        strokeLinejoin="round"
        strokeLinecap="round"
      />
      {/* Last point "current" dot */}
      <circle cx={lastX} cy={lastY} r={4} fill="var(--accent)" stroke="var(--bg)" strokeWidth="1.5" />
      <text
        x={lastX - 6} y={lastY - 8}
        textAnchor="end"
        fontSize="9"
        fill="var(--accent)"
        fontFamily="var(--font-ui)"
        fontWeight="600"
      >
        hiện tại
      </text>
    </svg>
  );
}

// ── Token stream for neural viz ────────────────────────────────────────────────

interface NeuralTokensProps {
  prices: number[];
  step: number;    // 0..N-1 = reveal token, N = done
  totalTokens: number;
}

function NeuralTokens({ prices, step, totalTokens }: NeuralTokensProps) {
  const displayPrices = prices.slice(-totalTokens);
  return (
    <div className="algo-predict__token-stream" aria-label="Chuỗi giá nạp vào mạng">
      {displayPrices.map((p, i) => {
        const revealed = i < step;
        const active   = i === step - 1;
        return (
          <span
            key={i}
            className={
              'algo-predict__token' +
              (active ? ' algo-predict__token--active' : '') +
              (!revealed ? ' algo-predict__token--pending' : '')
            }
          >
            {revealed ? fmtPrice(p) : '···'}
          </span>
        );
      })}
      <span className="algo-predict__token-arrow">→</span>
      <span
        className={'algo-predict__token-box' + (step >= totalTokens ? ' algo-predict__token-box--ready' : '')}
      >
        Mạng nơ-ron
      </span>
    </div>
  );
}

// ── Data & state types ─────────────────────────────────────────────────────────

interface PredResult {
  predictedPrice: number;
  currentPrice: number;
  confidence: number | null;
  symbol: string;
  market: string;
}

function prefersReducedMotion(): boolean {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

// ── Animation phases ───────────────────────────────────────────────────────────
// phase 0: showing input sparkline
// phase 1: animating mechanism (tokens streaming / box highlight)
// phase 2: output revealed

type Phase = 0 | 1 | 2;

const NEURAL_TOKENS = 10;   // how many price tokens to stream for neural
const PHASE1_TICKS  = 12;   // total ticks in mechanism phase

// ── Main component ─────────────────────────────────────────────────────────────

// VizBlock passes `playing` and `arg`. ownControls=true so playing is managed here.
// eslint-disable-next-line @typescript-eslint/no-unused-vars
export default function AlgoPredict(_props: { playing?: boolean; arg?: string }) {
  const { arg } = _props;
  const algoKey = (arg ?? '').trim();
  const family   = getAlgoFamily(algoKey);

  // ── Market selector ─────────────────────────────────────────────────────────
  const [marketIdx, setMarketIdx] = useState(0);

  // ── Data ────────────────────────────────────────────────────────────────────
  const [prices, setPrices]   = useState<number[]>([]);
  const [result, setResult]   = useState<PredResult | null>(null);
  const [status, setStatus]   = useState<'loading' | 'live' | 'synthetic'>('loading');

  // ── Animation ───────────────────────────────────────────────────────────────
  const [phase, setPhase]     = useState<Phase>(0);
  const [mechTick, setMechTick] = useState(0);  // sub-ticks within phase 1
  const [autoPlay, setAutoPlay] = useState(() => !prefersReducedMotion());

  const timerRef    = useRef<ReturnType<typeof setInterval> | null>(null);
  const phaseRef    = useRef<Phase>(phase);
  const mechRef     = useRef(mechTick);
  phaseRef.current  = phase;
  mechRef.current   = mechTick;

  // ── Fetch ───────────────────────────────────────────────────────────────────
  useEffect(() => {
    if (!algoKey) {
      setStatus('synthetic');
      setPrices([]);
      setResult(null);
      return;
    }

    const market = MARKETS[marketIdx];
    setStatus('loading');
    setPrices([]);
    setResult(null);
    setPhase(0);
    setMechTick(0);

    const token = localStorage.getItem('vns_token') || '';
    const headers: HeadersInit = token ? { Authorization: `Bearer ${token}` } : {};
    let cancelled = false;

    async function load() {
      try {
        const [predRes, chartRes] = await Promise.all([
          fetch(`/api/${market.apiPath}/predictions/latest`, { headers }),
          fetch(`/api/${market.apiPath}/chart?days=2`, { headers }),
        ]);
        if (cancelled) return;
        if (!predRes.ok || !chartRes.ok) throw new Error('fetch failed');

        const predData  = await predRes.json();
        const chartData = await chartRes.json();
        if (cancelled) return;

        // ── Parse predictions ──
        const allPreds: Array<{
          algorithm_name: string;
          predicted_price: unknown;
          current_price: unknown;
          confidence: unknown;
          symbol: string;
        }> = (predData?.data ?? []);

        // Filter: exact algo key, not per-symbol
        const matched = allPreds.filter(
          (p) => p.algorithm_name === algoKey && !p.algorithm_name.endsWith('__ps')
        );

        if (matched.length === 0) throw new Error('no prediction for this algo');

        // Pick symbol with a valid prediction
        let chosenPred: typeof matched[0] | null = null;
        for (const p of matched) {
          const pred = Number(p.predicted_price);
          const cur  = Number(p.current_price);
          if (Number.isFinite(pred) && Number.isFinite(cur) && cur > 0) {
            chosenPred = p;
            break;
          }
        }
        if (!chosenPred) throw new Error('no finite prediction found');

        const predictedPrice = Number(chosenPred.predicted_price);
        const currentPrice   = Number(chosenPred.current_price);
        const confidence     = chosenPred.confidence != null
          ? (Number.isFinite(Number(chosenPred.confidence)) ? Number(chosenPred.confidence) : null)
          : null;

        // ── Parse chart prices ──
        const rawSrc: unknown[] = (chartData?.closes ?? chartData?.prices ?? []) as unknown[];
        const rawPrices: number[] = rawSrc
          .map(Number)
          .filter((v) => Number.isFinite(v) && v > 0);

        // Take last 24 points
        const trimmed = rawPrices.slice(-24);
        if (trimmed.length < 4) throw new Error('too few chart points');

        setPrices(trimmed);
        setResult({
          predictedPrice,
          currentPrice,
          confidence,
          symbol: chosenPred.symbol,
          market: market.label,
        });
        setStatus('live');
      } catch {
        if (cancelled) return;
        // Synthetic fallback
        const base = 92000;
        const synth = Array.from({ length: 20 }, (_, i) =>
          base + Math.sin(i * 0.7) * 800 + i * 30
        );
        const synResult: PredResult = {
          predictedPrice: base + 540,
          currentPrice: base,
          confidence: 0.71,
          symbol: 'SYN',
          market: market.label,
        };
        setPrices(synth);
        setResult(synResult);
        setStatus('synthetic');
      }
    }

    load();
    return () => { cancelled = true; };
  }, [marketIdx, algoKey]);

  // ── Reset animation on new data ────────────────────────────────────────────
  useEffect(() => {
    setPhase(0);
    setMechTick(0);
  }, [prices, result]);

  // ── Tick function ──────────────────────────────────────────────────────────
  const tick = useCallback(() => {
    const p  = phaseRef.current;
    const mt = mechRef.current;

    if (p === 0) {
      setPhase(1);
      setMechTick(0);
    } else if (p === 1) {
      const nextTick = mt + 1;
      if (nextTick >= PHASE1_TICKS) {
        setPhase(2);
        setMechTick(0);
      } else {
        setMechTick(nextTick);
      }
    } else {
      // phase 2 done — loop back
      setPhase(0);
      setMechTick(0);
    }
  }, []);

  // ── Auto-play ──────────────────────────────────────────────────────────────
  const clearTimer = useCallback(() => {
    if (timerRef.current !== null) {
      clearInterval(timerRef.current);
      timerRef.current = null;
    }
  }, []);

  useEffect(() => {
    if (!autoPlay || prefersReducedMotion() || status === 'loading') {
      clearTimer();
      return;
    }
    clearTimer();
    // phase 0 lingers 600ms, phase 1 ticks 400ms each, phase 2 lingers 1.5s
    const delay = phase === 0 ? 700 : phase === 2 ? 1600 : 400;
    timerRef.current = setTimeout(() => {
      tick();
    }, delay);
    return clearTimer;
  }, [autoPlay, phase, mechTick, status, tick, clearTimer]);

  // ── Derived ────────────────────────────────────────────────────────────────
  const marketOpt   = MARKETS[marketIdx];

  // Token reveal progress for neural (0..NEURAL_TOKENS)
  const neuralStep  = phase === 1
    ? Math.min(Math.round((mechTick / PHASE1_TICKS) * NEURAL_TOKENS), NEURAL_TOKENS)
    : phase === 2 ? NEURAL_TOKENS : 0;

  // Mechanism box highlight progress (0..1)
  const mechProgress = phase === 1 ? mechTick / PHASE1_TICKS : phase === 2 ? 1 : 0;

  const delta       = result ? result.predictedPrice - result.currentPrice : 0;
  const isUp        = delta >= 0;

  // ── Early return: no arg ────────────────────────────────────────────────────
  if (!algoKey) {
    return (
      <div className="algo-predict algo-predict--missing">
        <span>Thiếu tham số thuật toán.</span>
        <span className="algo-predict__missing-hint">
          Dùng cú pháp: <code>algo-predict &lt;algorithm_key&gt;</code>
          <br />
          Ví dụ: <code>algo-predict lstm_nn</code>
        </span>
      </div>
    );
  }

  // ── Mechanism box content ──────────────────────────────────────────────────
  function renderMechanism() {
    const isActive = phase >= 1;
    const isDone   = phase === 2;

    if (family === 'neural') {
      return (
        <div className="algo-predict__mech">
          <NeuralTokens
            prices={prices.slice(-NEURAL_TOKENS)}
            step={neuralStep}
            totalTokens={Math.min(NEURAL_TOKENS, prices.length)}
          />
          <div
            className={'algo-predict__mech-box' + (isDone ? ' algo-predict__mech-box--done' : isActive ? ' algo-predict__mech-box--active' : '')}
            style={{ '--mech-progress': mechProgress } as React.CSSProperties}
          >
            <span className="algo-predict__mech-label">{getFamilyLabel(family)}</span>
            <span className="algo-predict__mech-sublabel">{getFamilyDesc(family)}</span>
          </div>
        </div>
      );
    }

    return (
      <div className="algo-predict__mech">
        <div className="algo-predict__mech-bar-wrap">
          <div
            className="algo-predict__mech-bar"
            style={{ width: `${Math.round(mechProgress * 100)}%` }}
          />
          <span className="algo-predict__mech-bar-label">
            {Math.round(mechProgress * 100)}%
          </span>
        </div>
        <div
          className={'algo-predict__mech-box' + (isDone ? ' algo-predict__mech-box--done' : isActive ? ' algo-predict__mech-box--active' : '')}
        >
          <span className="algo-predict__mech-label">{getFamilyLabel(family)}</span>
          <span className="algo-predict__mech-sublabel">{getFamilyDesc(family)}</span>
        </div>
      </div>
    );
  }

  // ── Render ────────────────────────────────────────────────────────────────
  return (
    <div className="algo-predict">
      {/* Top bar */}
      <div className="trace-viz__topbar">
        <div className="docs-viz__market-select" style={{ position: 'static' }}>
          <label htmlFor="ap-market-sel">Thị trường:</label>
          <select
            id="ap-market-sel"
            value={marketIdx}
            onChange={(e) => {
              setMarketIdx(Number(e.target.value));
              setAutoPlay(false);
            }}
          >
            {MARKETS.map((m, i) => (
              <option key={m.apiPath} value={i}>{m.label}</option>
            ))}
          </select>
        </div>

        {status === 'live' && result && (
          <span className="docs-viz__data-badge docs-viz__data-badge--live" style={{ position: 'static' }}>
            Dữ liệu thật · {marketOpt.label} · {result.symbol}
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

        <span className="algo-predict__algo-tag">
          {algoKey}
        </span>
      </div>

      {/* ── Phase indicator strip ── */}
      <div className="algo-predict__phases">
        <div className={'algo-predict__phase-step' + (phase >= 0 ? ' algo-predict__phase-step--done' : '')}>
          <span className="algo-predict__phase-num">1</span>
          <span>INPUT · chuỗi giá thật</span>
        </div>
        <div className="algo-predict__phase-arrow">→</div>
        <div className={'algo-predict__phase-step' + (phase >= 1 ? ' algo-predict__phase-step--active' : '') + (phase >= 2 ? ' algo-predict__phase-step--done' : '')}>
          <span className="algo-predict__phase-num">2</span>
          <span>CƠ CHẾ · {getFamilyLabel(family)}</span>
        </div>
        <div className="algo-predict__phase-arrow">→</div>
        <div className={'algo-predict__phase-step' + (phase === 2 ? ' algo-predict__phase-step--active' : '')}>
          <span className="algo-predict__phase-num">3</span>
          <span>OUTPUT · dự đoán thật</span>
        </div>
      </div>

      {/* ── Main body: 3-panel layout ── */}
      <div className="algo-predict__body">

        {/* Panel 1: Input sparkline */}
        <div className={'algo-predict__panel algo-predict__panel--input' + (phase === 0 ? ' algo-predict__panel--highlight' : '')}>
          <div className="algo-predict__panel-header">
            <span className="algo-predict__panel-title">INPUT</span>
            <span className="algo-predict__panel-sub">~{prices.length} điểm giá gần nhất</span>
          </div>
          {prices.length >= 2 ? (
            <div className="algo-predict__spark-wrap">
              <Sparkline prices={prices} />
              <div className="algo-predict__price-range">
                <span>min: {fmtPrice(Math.min(...prices))}</span>
                <span>max: {fmtPrice(Math.max(...prices))}</span>
              </div>
            </div>
          ) : (
            <div className="algo-predict__panel-loading">
              {status === 'loading' ? 'Đang tải…' : 'Không có dữ liệu'}
            </div>
          )}
          <div className="algo-predict__panel-note">
            Số thật từ API · nội bộ được chuẩn hoá trước khi nạp vào mô hình
          </div>
        </div>

        {/* Connector */}
        <div className="algo-predict__connector">
          <div className={'algo-predict__connector-line' + (phase >= 1 ? ' algo-predict__connector-line--active' : '')} />
          <span className={'algo-predict__connector-arrow' + (phase >= 1 ? ' algo-predict__connector-arrow--active' : '')}>▶</span>
        </div>

        {/* Panel 2: Mechanism */}
        <div className={'algo-predict__panel algo-predict__panel--mech' + (phase === 1 ? ' algo-predict__panel--highlight' : '')}>
          <div className="algo-predict__panel-header">
            <span className="algo-predict__panel-title">CƠ CHẾ</span>
            <span className="algo-predict__panel-sub">{getFamilyLabel(family)}</span>
          </div>
          {renderMechanism()}
          <div className="algo-predict__panel-note">
            Nội bộ là hộp đen · input &amp; output là số thật đo được
          </div>
        </div>

        {/* Connector */}
        <div className="algo-predict__connector">
          <div className={'algo-predict__connector-line' + (phase >= 2 ? ' algo-predict__connector-line--active' : '')} />
          <span className={'algo-predict__connector-arrow' + (phase >= 2 ? ' algo-predict__connector-arrow--active' : '')}>▶</span>
        </div>

        {/* Panel 3: Output */}
        <div className={'algo-predict__panel algo-predict__panel--output' + (phase === 2 ? ' algo-predict__panel--highlight' : '')}>
          <div className="algo-predict__panel-header">
            <span className="algo-predict__panel-title">OUTPUT</span>
            <span className="algo-predict__panel-sub">dự đoán giờ kế tiếp</span>
          </div>

          {phase < 2 || !result ? (
            <div className="algo-predict__output-pending">
              <span>—</span>
              <span className="algo-predict__output-pending-note">Đang tính toán…</span>
            </div>
          ) : (
            <div className="algo-predict__output-card">
              <div className="algo-predict__output-row algo-predict__output-row--current">
                <span className="algo-predict__output-lbl">Giá hiện tại</span>
                <span className="algo-predict__output-val">{fmtPrice(result.currentPrice)}</span>
              </div>
              <div className={'algo-predict__output-row algo-predict__output-row--predicted'}>
                <span className="algo-predict__output-lbl">Dự đoán</span>
                <strong className={'algo-predict__output-pred' + (isUp ? ' algo-predict__output-pred--up' : ' algo-predict__output-pred--down')}>
                  {fmtPrice(result.predictedPrice)}
                </strong>
              </div>
              <div className={'algo-predict__output-row algo-predict__output-row--delta'}>
                <span className="algo-predict__output-lbl">Δ</span>
                <span className={'algo-predict__output-delta' + (isUp ? ' algo-predict__output-delta--up' : ' algo-predict__output-delta--down')}>
                  {fmtDelta(delta)}
                  {result.currentPrice > 0 && (
                    <span className="algo-predict__output-pct">
                      &nbsp;({fmtPct(delta, result.currentPrice)})
                    </span>
                  )}
                </span>
              </div>
              <div className="algo-predict__output-row algo-predict__output-row--dir">
                <span className="algo-predict__output-lbl">Hướng</span>
                <span className={'algo-predict__output-dir' + (isUp ? ' algo-predict__output-dir--up' : ' algo-predict__output-dir--down')}>
                  {isUp ? '↑ Tăng' : '↓ Giảm'}
                </span>
              </div>
              {result.confidence != null && (
                <div className="algo-predict__output-row algo-predict__output-row--conf">
                  <span className="algo-predict__output-lbl">Confidence</span>
                  <span className="algo-predict__output-conf">
                    {(result.confidence * 100).toFixed(1)}%
                    <span className="algo-predict__conf-bar-wrap">
                      <span
                        className="algo-predict__conf-bar"
                        style={{ width: `${Math.min(result.confidence * 100, 100)}%` }}
                      />
                    </span>
                  </span>
                </div>
              )}
            </div>
          )}

          <div className="algo-predict__panel-note">
            Dự đoán target = now + 1h · từ API thật
          </div>
        </div>
      </div>

      {/* ── Controls ── */}
      <div className="trace-viz__controls">
        <button
          className="trace-viz__ctrl-btn"
          onClick={() => {
            setAutoPlay(false);
            setPhase(0);
            setMechTick(0);
          }}
          aria-label="Đặt lại"
        >
          ⟲ Đặt lại
        </button>

        <button
          className={`trace-viz__ctrl-btn${autoPlay ? ' trace-viz__ctrl-btn--active' : ''}`}
          onClick={() => {
            if (prefersReducedMotion()) return;
            setAutoPlay((a) => !a);
          }}
          disabled={prefersReducedMotion() || status === 'loading'}
          title={prefersReducedMotion() ? 'Tắt do prefers-reduced-motion' : ''}
          aria-label={autoPlay ? 'Tạm dừng' : 'Tự chạy'}
        >
          {autoPlay ? '⏸ Tạm dừng' : '▶ Tự chạy'}
        </button>

        <button
          className="trace-viz__ctrl-btn"
          onClick={() => {
            setAutoPlay(false);
            tick();
          }}
          disabled={status === 'loading'}
          aria-label="Bước tiếp theo"
        >
          Sau ▶
        </button>

        <span className="trace-viz__progress">
          {status === 'loading'
            ? 'Đang tải dữ liệu…'
            : phase === 0
            ? 'Phase 1/3 · INPUT'
            : phase === 1
            ? `Phase 2/3 · CƠ CHẾ (${Math.round(mechProgress * 100)}%)`
            : 'Phase 3/3 · OUTPUT'}
        </span>
      </div>
    </div>
  );
}
