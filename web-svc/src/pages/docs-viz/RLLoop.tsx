import { useEffect, useRef, useCallback } from 'react';

const W = 600;
const H = 250;
const CX = W / 2;
const CY = H / 2 - 10;
const RX = 180;
const RY = 90;

// Nodes on the ellipse: State, Agent, Action, Env, Reward
const NODE_ANGLES = [
  270, // State (top)
  0,   // Agent (right)
  90,  // Action (bottom-right)
  180, // Env (left)
  225, // Reward (bottom-left)
];
const NODE_LABELS = ['State', 'Agent (Q)', 'Action', 'Môi trường', 'Reward'];
const NODE_W = [52, 60, 52, 72, 56];
const NODE_H = 22;

function ellipsePoint(angleDeg: number): [number, number] {
  const a = (angleDeg * Math.PI) / 180;
  return [CX + RX * Math.cos(a), CY + RY * Math.sin(a)];
}

const ACTIONS = ['Mua', 'Giữ', 'Bán'];
const ACTION_COLORS = ['var(--up)', 'var(--text-3)', 'var(--down)'];
const STEP_MS = 600;
const N_STEPS = 5; // number of loop iterations before restart

interface Props {
  playing: boolean;
  arg?: string;
}

export default function RLLoop({ playing }: Props) {
  const svgRef = useRef<SVGSVGElement>(null);
  const rafRef = useRef<number>(0);
  const lastTickRef = useRef<number>(0);
  const stepRef = useRef<number>(0);

  const drawStep = useCallback((step: number) => {
    const svg = svgRef.current;
    if (!svg) return;

    // Dot position: moves through 5 nodes, looping
    const nodeIdx = step % NODE_ANGLES.length;
    const [dotX, dotY] = ellipsePoint(NODE_ANGLES[nodeIdx]);

    const dot = svg.querySelector('#rl-dot') as SVGCircleElement | null;
    if (dot) {
      dot.setAttribute('cx', String(dotX));
      dot.setAttribute('cy', String(dotY));
    }

    // Highlight active node
    for (let n = 0; n < NODE_ANGLES.length; n++) {
      const nodeEl = svg.querySelector(`#rl-node-${n}`) as SVGRectElement | null;
      if (nodeEl) {
        nodeEl.setAttribute(
          'fill',
          n === nodeIdx ? 'var(--accent)' : 'var(--surface)'
        );
        nodeEl.setAttribute(
          'stroke',
          n === nodeIdx ? 'var(--accent)' : 'var(--border)'
        );
      }
      const nodeTxt = svg.querySelector(`#rl-txt-${n}`) as SVGTextElement | null;
      if (nodeTxt) {
        nodeTxt.setAttribute(
          'fill',
          n === nodeIdx ? 'var(--bg)' : 'var(--text-2)'
        );
        nodeTxt.setAttribute('font-weight', n === nodeIdx ? '600' : '400');
      }
    }

    // Action display (at action node, idx 2)
    const actionI = (step >> 1) % 3;
    const actionEl = svg.querySelector('#action-label') as SVGTextElement | null;
    if (actionEl) {
      actionEl.textContent = ACTIONS[actionI];
      actionEl.setAttribute('fill', ACTION_COLORS[actionI]);
      actionEl.setAttribute('opacity', nodeIdx === 2 ? '1' : '0.6');
    }

    // Reward display
    const rewardVal = step % 2 === 0 ? '+0.15' : '-0.07';
    const rewardColor = step % 2 === 0 ? 'var(--up)' : 'var(--down)';
    const rewardEl = svg.querySelector('#reward-label') as SVGTextElement | null;
    if (rewardEl) {
      rewardEl.textContent = rewardVal;
      rewardEl.setAttribute('fill', rewardColor);
      rewardEl.setAttribute('opacity', nodeIdx === 4 ? '1' : '0.5');
    }

    // Q-value bars (3 bars: buy, hold, sell)
    const qVals = [
      0.3 + 0.4 * Math.sin(step * 0.9),
      0.4 + 0.2 * Math.cos(step * 0.7),
      0.25 + 0.35 * Math.sin(step * 1.1 + 1),
    ];
    const qMax = Math.max(...qVals);
    for (let b = 0; b < 3; b++) {
      const bar = svg.querySelector(`#q-bar-${b}`) as SVGRectElement | null;
      if (!bar) continue;
      const norm = qVals[b] / qMax;
      const bh = norm * 28;
      const isAgent = nodeIdx === 1;
      bar.setAttribute('height', String(bh));
      bar.setAttribute('y', String(80 - bh));
      bar.setAttribute('fill', isAgent && b === actionI ? 'var(--accent)' : 'var(--surface-3)');
      bar.setAttribute('opacity', isAgent ? '1' : '0.5');
    }
  }, []);

  useEffect(() => {
    if (!playing) {
      drawStep(3);
      return;
    }
    stepRef.current = 0;
    lastTickRef.current = 0;

    function frame(now: number) {
      if (lastTickRef.current === 0) lastTickRef.current = now;
      if (now - lastTickRef.current >= STEP_MS) {
        lastTickRef.current = now;
        stepRef.current += 1;
      }
      drawStep(stepRef.current);
      rafRef.current = requestAnimationFrame(frame);
    }
    rafRef.current = requestAnimationFrame(frame);
    return () => cancelAnimationFrame(rafRef.current);
  }, [playing, drawStep]);

  // Build node rects
  const nodes = NODE_ANGLES.map((angle, i) => {
    const [x, y] = ellipsePoint(angle);
    return { x, y, label: NODE_LABELS[i], w: NODE_W[i] };
  });

  // Build edge path segments (ellipse arcs between consecutive nodes)
  // We draw a full ellipse as the "track"

  // Q-bar positions (near Agent node)
  const [agentX, agentY] = ellipsePoint(0);
  const qBarX = agentX - 36;
  const qBarY = agentY - 10;

  return (
    <svg
      ref={svgRef}
      viewBox={`0 0 ${W} ${H}`}
      width="100%"
      style={{ display: 'block', height: 250 }}
      aria-label="Minh hoạ vòng lặp Reinforcement Learning"
    >
      <defs>
        <marker id="arr-rl" markerWidth="6" markerHeight="6" refX="3" refY="3" orient="auto">
          <path d="M0,0 L0,6 L6,3 Z" fill="var(--border-strong)" />
        </marker>
      </defs>

      {/* Ellipse track */}
      <ellipse
        cx={CX} cy={CY} rx={RX} ry={RY}
        fill="none"
        stroke="var(--border)"
        strokeWidth="1.5"
        strokeDasharray="6 4"
      />

      {/* Arrow overlay on ellipse (direction indicator) */}
      <ellipse
        cx={CX} cy={CY} rx={RX} ry={RY}
        fill="none"
        stroke="var(--border-strong)"
        strokeWidth="1"
        strokeDasharray="2 14"
        strokeDashoffset="-6"
        markerMid="url(#arr-rl)"
      />

      {/* Q-value mini bars */}
      {[0, 1, 2].map((b) => (
        <rect
          key={b}
          id={`q-bar-${b}`}
          x={qBarX + b * 14}
          y={qBarY + 28 - 12}
          width={10}
          height={12}
          rx={1}
          fill="var(--surface-3)"
        />
      ))}
      <text x={qBarX + 15} y={qBarY - 8}
        textAnchor="middle" fontSize="9"
        fill="var(--text-3)" fontFamily="var(--font-ui)">
        Q-vals
      </text>
      {['B', 'G', 'S'].map((lbl, b) => (
        <text key={b}
          x={qBarX + b * 14 + 5} y={qBarY + 40}
          textAnchor="middle" fontSize="8"
          fill="var(--text-3)" fontFamily="var(--font-ui)"
        >{lbl}</text>
      ))}

      {/* Node boxes */}
      {nodes.map((n, i) => (
        <g key={i}>
          <rect
            id={`rl-node-${i}`}
            x={n.x - n.w / 2} y={n.y - NODE_H / 2}
            width={n.w} height={NODE_H}
            rx={3}
            fill="var(--surface)"
            stroke="var(--border)"
            strokeWidth="1.2"
          />
          <text
            id={`rl-txt-${i}`}
            x={n.x} y={n.y + 4}
            textAnchor="middle" fontSize="10"
            fill="var(--text-2)" fontFamily="var(--font-ui)"
          >
            {n.label}
          </text>
        </g>
      ))}

      {/* Action sub-label */}
      {(() => {
        const [ax, ay] = ellipsePoint(90);
        return (
          <text
            id="action-label"
            x={ax} y={ay + 22}
            textAnchor="middle" fontSize="11"
            fill="var(--up)" fontFamily="var(--font-ui)"
            fontWeight="600"
          >
            Mua
          </text>
        );
      })()}

      {/* Reward sub-label */}
      {(() => {
        const [rx2, ry2] = ellipsePoint(225);
        return (
          <text
            id="reward-label"
            x={rx2} y={ry2 + 22}
            textAnchor="middle" fontSize="11"
            fill="var(--up)" fontFamily="var(--font-ui)"
            fontWeight="700"
          >
            +0.15
          </text>
        );
      })()}

      {/* Moving dot */}
      <circle
        id="rl-dot"
        cx={CX} cy={CY - RY} r={6}
        fill="var(--accent)"
        stroke="var(--bg)"
        strokeWidth="2"
        style={{ filter: 'drop-shadow(0 0 4px var(--accent))' }}
      />

      {/* Bottom caption */}
      <text x={W / 2} y={H - 6}
        textAnchor="middle" fontSize="11"
        fill="var(--text-3)" fontFamily="var(--font-ui)">
        Quan sát → Hành động (Q-values) → Nhận thưởng → Cập nhật trọng số
      </text>
    </svg>
  );
}
