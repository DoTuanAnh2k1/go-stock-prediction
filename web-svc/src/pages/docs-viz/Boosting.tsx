import { useEffect, useRef, useCallback } from 'react';

const W = 600;
const H = 240;
const PAD_L = 40;
const PAD_R = 20;
const PAD_T = 28;
const PAD_B = 44;
const CHART_W = W - PAD_L - PAD_R;
const CHART_H = H - PAD_T - PAD_B;
const N_TREES = 5;
const TREE_STEP_MS = 900;

// X axis points
const NX = 30;
const xs = Array.from({ length: NX }, (_, i) => PAD_L + (i / (NX - 1)) * CHART_W);

// Target function (true values — normalized 0..1)
const target: number[] = Array.from({ length: NX }, (_, i) => {
  const t = i / (NX - 1);
  return (
    0.3 +
    0.3 * Math.sin(t * Math.PI * 2.5) +
    0.15 * Math.cos(t * Math.PI * 5) +
    0.08 * Math.sin(t * Math.PI * 9)
  );
});

// Each tree adds a smaller correction towards the target
// Tree k: contributes learningRate^k of remaining residual
function buildPredictions(): number[][] {
  const lr = 0.55;
  const preds: number[][] = [];
  let current = Array(NX).fill(0.5); // flat start

  for (let k = 0; k < N_TREES; k++) {
    const correction = current.map((v, i) => (target[i] - v) * lr);
    const next = current.map((v, i) => v + correction[i]);
    preds.push(next.slice());
    current = next;
  }
  return preds;
}

const predictions = buildPredictions();

function toY(v: number): number {
  return PAD_T + CHART_H - v * CHART_H;
}

function makePath(arr: number[]): string {
  return arr
    .map((v, i) => (i === 0 ? 'M' : 'L') + `${xs[i].toFixed(1)},${toY(v).toFixed(1)}`)
    .join(' ');
}

function interpolatePaths(a: number[], b: number[], t: number): number[] {
  return a.map((v, i) => v + (b[i] - v) * t);
}

interface Props {
  playing: boolean;
  arg?: string;
}

export default function Boosting({ playing }: Props) {
  const rafRef = useRef<number>(0);
  const startRef = useRef<number>(0);
  const svgRef = useRef<SVGSVGElement>(null);

  const TOTAL_MS = N_TREES * TREE_STEP_MS + 600;

  const drawAt = useCallback((elapsed: number) => {
    const svg = svgRef.current;
    if (!svg) return;

    const frac = Math.min(elapsed / TOTAL_MS, 1);
    // Which tree step are we in? 0 = flat, 1..N_TREES = after tree k
    const treePhase = frac * N_TREES;
    const treeFloor = Math.floor(treePhase);
    const treeFrac = treePhase - treeFloor;

    // Current prediction line
    const fromArr =
      treeFloor === 0 ? Array(NX).fill(0.5) : predictions[treeFloor - 1];
    const toArr =
      treeFloor >= N_TREES ? predictions[N_TREES - 1] : predictions[treeFloor];
    const current = interpolatePaths(fromArr, toArr, treeFrac);

    const predPath = svg.querySelector('#pred-path') as SVGPathElement | null;
    if (predPath) {
      predPath.setAttribute('d', makePath(current));
    }

    // Tree correction arrows (residuals) — show for each completed tree
    for (let k = 0; k < N_TREES; k++) {
      const treeEl = svg.querySelector(`#tree-${k}`) as SVGGElement | null;
      if (!treeEl) continue;
      // Tree k becomes visible when treeFloor > k or (treeFloor == k and treeFrac > 0)
      const visible = treePhase > k;
      treeEl.setAttribute('opacity', visible ? String(Math.min((treePhase - k), 1)) : '0');
    }

    // Show "converged" label
    const convLabel = svg.querySelector('#conv-label') as SVGTextElement | null;
    if (convLabel) {
      convLabel.setAttribute('opacity', frac >= 0.95 ? '1' : '0');
    }
  }, []);

  useEffect(() => {
    if (!playing) {
      drawAt(TOTAL_MS);
      return;
    }
    startRef.current = performance.now();
    function frame(now: number) {
      const elapsed = now - startRef.current;
      drawAt(elapsed % (TOTAL_MS + 800));
      rafRef.current = requestAnimationFrame(frame);
    }
    rafRef.current = requestAnimationFrame(frame);
    return () => cancelAnimationFrame(rafRef.current);
  }, [playing, drawAt]);

  // Tree chip positions (right side legend)
  const chipColors = ['var(--accent)', 'var(--accent)', 'var(--accent)', 'var(--accent)', 'var(--accent)'];
  const chipOpacities = [0.85, 0.65, 0.5, 0.38, 0.28];

  return (
    <svg
      ref={svgRef}
      viewBox={`0 0 ${W} ${H}`}
      width="100%"
      style={{ display: 'block', height: 240 }}
      aria-label="Minh hoạ Gradient Boosting"
    >
      <defs>
        <marker id="arr-up" markerWidth="5" markerHeight="5" refX="2.5" refY="2.5" orient="auto">
          <path d="M0,5 L2.5,0 L5,5 Z" fill="var(--up)" opacity="0.7" />
        </marker>
      </defs>

      {/* Axes */}
      <line x1={PAD_L} y1={PAD_T} x2={PAD_L} y2={PAD_T + CHART_H}
        stroke="var(--border)" strokeWidth="1" />
      <line x1={PAD_L} y1={PAD_T + CHART_H} x2={PAD_L + CHART_W} y2={PAD_T + CHART_H}
        stroke="var(--border)" strokeWidth="1" />

      {/* Target (dashed) */}
      <path
        d={makePath(target)}
        fill="none"
        stroke="var(--text-3)"
        strokeWidth="1.5"
        strokeDasharray="5 4"
      />
      <text x={PAD_L + CHART_W - 2} y={toY(target[NX - 1]) - 6}
        textAnchor="end" fontSize="10" fill="var(--text-3)"
        fontFamily="var(--font-ui)">Mục tiêu</text>

      {/* Tree correction indicators (small vertical ticks scattered) */}
      {Array.from({ length: N_TREES }, (_, k) => {
        // Place a few residual arrows at sample positions for this tree
        const sampleIdxs = [Math.round(NX / 5 * (k + 0.5)), Math.round(NX / 5 * (k + 1.5) % NX)];
        return (
          <g key={k} id={`tree-${k}`} opacity={0}>
            {sampleIdxs.map((si, j) => {
              const xi = xs[si];
              const fromV = k === 0 ? 0.5 : predictions[k - 1][si];
              const toV = predictions[k][si];
              const dy = toY(fromV) - toY(toV);
              if (Math.abs(dy) < 2) return null;
              return (
                <line key={j}
                  x1={xi} y1={toY(fromV)}
                  x2={xi} y2={toY(fromV) - dy * 0.8}
                  stroke="var(--up)"
                  strokeWidth="1.5"
                  opacity={0.6}
                  markerEnd="url(#arr-up)"
                />
              );
            })}
            {/* Tree chip label */}
            <rect
              x={PAD_L + CHART_W + 8}
              y={PAD_T + k * 26}
              width={50} height={18}
              rx={2}
              fill={chipColors[k]}
              opacity={chipOpacities[k]}
            />
            <text
              x={PAD_L + CHART_W + 33}
              y={PAD_T + k * 26 + 12}
              textAnchor="middle" fontSize="9"
              fill="var(--bg)" fontFamily="var(--font-ui)"
              fontWeight="600"
            >
              Cây {k + 1}
            </text>
          </g>
        );
      })}

      {/* Current prediction path */}
      <path
        id="pred-path"
        d={makePath(Array(NX).fill(0.5))}
        fill="none"
        stroke="var(--accent)"
        strokeWidth="2.2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />

      {/* Convergence label */}
      <text id="conv-label"
        x={PAD_L + CHART_W / 2} y={PAD_T - 8}
        textAnchor="middle" fontSize="10"
        fill="var(--accent)" fontFamily="var(--font-ui)"
        fontWeight="600"
        opacity={0}
      >
        Hội tụ sau {N_TREES} cây
      </text>

      {/* Axis labels */}
      <text x={PAD_L - 6} y={PAD_T} textAnchor="end" fontSize="9"
        fill="var(--text-3)" fontFamily="var(--font-ui)">Cao</text>
      <text x={PAD_L - 6} y={PAD_T + CHART_H + 4} textAnchor="end" fontSize="9"
        fill="var(--text-3)" fontFamily="var(--font-ui)">Thấp</text>

      {/* Bottom caption */}
      <text x={W / 2} y={H - 6}
        textAnchor="middle" fontSize="11"
        fill="var(--text-3)" fontFamily="var(--font-ui)">
        Mỗi cây sửa phần sai còn lại · đường accent hội tụ về mục tiêu (nét đứt)
      </text>
    </svg>
  );
}
