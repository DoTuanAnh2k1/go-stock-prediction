import { useEffect, useRef, useCallback } from 'react';

const W = 600;
const H = 240;
const N_TOKENS = 12;
const TOKEN_W = 32;
const TOKEN_H = 24;
const TOKEN_GAP = 6;
const SEQ_Y = 44;
const CELL_X = W - 120;
const CELL_Y = 80;
const CELL_W = 90;
const CELL_H = 52;
const HIDDEN_CELLS = 4;
const HIDDEN_Y = 158;
const HIDDEN_CELL_W = 16;
const HIDDEN_CELL_H = 16;
const HIDDEN_GAP = 6;
const STEP_MS = 420;
const TOTAL_STEPS = N_TOKENS + 2; // tokens + pause + predict

// Deterministic price changes for token labels
const changes = ['+0.3', '-0.1', '+0.5', '+0.2', '-0.4', '+0.1',
                 '-0.2', '+0.6', '+0.3', '-0.1', '+0.4', '+0.2'];

interface HiddenState {
  level: number; // 0..1
}

interface Props {
  playing: boolean;
  arg?: string;
}

export default function SequenceModel({ playing }: Props) {
  const svgRef = useRef<SVGSVGElement>(null);
  const stepRef = useRef<number>(0);
  const rafRef = useRef<number>(0);
  const lastTickRef = useRef<number>(0);

  // Build initial hidden states (will be updated)
  const hiddenInit: HiddenState[] = Array.from({ length: HIDDEN_CELLS }, (_, i) =>
    ({ level: 0.2 + i * 0.05 })
  );

  const drawStep = useCallback((step: number) => {
    const svg = svgRef.current;
    if (!svg) return;

    // Update token highlights
    for (let i = 0; i < N_TOKENS; i++) {
      const tok = svg.querySelector(`#tok-${i}`) as SVGRectElement | null;
      const isActive = i === step - 1;
      const isPast = i < step - 1;
      if (tok) {
        tok.setAttribute(
          'fill',
          isActive
            ? 'var(--accent)'
            : isPast
            ? 'var(--surface-3)'
            : 'var(--surface-2)'
        );
        tok.setAttribute('opacity', isPast ? '0.5' : '1');
      }
      const txt = svg.querySelector(`#tok-txt-${i}`) as SVGTextElement | null;
      if (txt) {
        txt.setAttribute(
          'fill',
          isActive ? 'var(--bg)' : isPast ? 'var(--text-3)' : 'var(--text-2)'
        );
      }
    }

    // Animate token moving into cell
    const flying = svg.querySelector('#flying-tok') as SVGRectElement | null;
    const flyingTxt = svg.querySelector('#flying-tok-txt') as SVGTextElement | null;
    const activeI = step - 1;
    if (activeI >= 0 && activeI < N_TOKENS) {
      const srcX = activeI * (TOKEN_W + TOKEN_GAP);
      if (flying) {
        flying.setAttribute('x', String(srcX));
        flying.setAttribute('y', String(SEQ_Y));
        flying.setAttribute('opacity', '1');
      }
      if (flyingTxt) {
        flyingTxt.setAttribute('x', String(srcX + TOKEN_W / 2));
        flyingTxt.setAttribute('y', String(SEQ_Y + TOKEN_H / 2 + 4));
        flyingTxt.setAttribute('opacity', '1');
        flyingTxt.textContent = changes[activeI];
      }
    } else {
      if (flying) flying.setAttribute('opacity', '0');
      if (flyingTxt) flyingTxt.setAttribute('opacity', '0');
    }

    // Update cell border brightness
    const cell = svg.querySelector('#mem-cell') as SVGRectElement | null;
    if (cell) {
      const pct = Math.min(activeI / N_TOKENS, 1);
      cell.setAttribute('stroke-width', String(1 + pct * 2));
    }

    // Update hidden state bars
    for (let j = 0; j < HIDDEN_CELLS; j++) {
      const bar = svg.querySelector(`#hid-bar-${j}`) as SVGRectElement | null;
      if (!bar) continue;
      const phase = (step + j * 3) % (HIDDEN_CELLS * 3);
      const level = hiddenInit[j].level + (step / TOTAL_STEPS) * 0.65 + 0.08 * Math.sin(phase);
      const clamped = Math.min(Math.max(level, 0.1), 0.95);
      const bh = clamped * HIDDEN_CELL_H;
      bar.setAttribute('height', String(bh));
      bar.setAttribute('y', String(HIDDEN_Y + HIDDEN_CELL_H - bh));
      bar.setAttribute('opacity', String(0.5 + clamped * 0.5));
    }

    // Show/hide predict arrow + label
    const arrow = svg.querySelector('#pred-arrow') as SVGGElement | null;
    if (arrow) {
      arrow.setAttribute('opacity', step >= TOTAL_STEPS - 1 ? '1' : '0');
    }

    // Connecting arrow from tokens to cell
    const conn = svg.querySelector('#conn-arrow') as SVGLineElement | null;
    if (conn) {
      conn.setAttribute('opacity', activeI >= 0 && activeI < N_TOKENS ? '1' : '0.15');
    }
  }, [hiddenInit]);

  useEffect(() => {
    if (!playing) {
      // static: show end state
      drawStep(TOTAL_STEPS);
      return;
    }
    stepRef.current = 0;
    lastTickRef.current = 0;

    function frame(now: number) {
      if (lastTickRef.current === 0) lastTickRef.current = now;
      const elapsed = now - lastTickRef.current;
      if (elapsed >= STEP_MS) {
        lastTickRef.current = now;
        stepRef.current = (stepRef.current + 1) % (TOTAL_STEPS + 3);
      }
      drawStep(stepRef.current);
      rafRef.current = requestAnimationFrame(frame);
    }
    rafRef.current = requestAnimationFrame(frame);
    return () => cancelAnimationFrame(rafRef.current);
  }, [playing, drawStep]);

  // Pre-compute token positions
  const tokens = Array.from({ length: N_TOKENS }, (_, i) => ({
    x: i * (TOKEN_W + TOKEN_GAP),
    label: changes[i],
  }));

  const hiddenXs = Array.from({ length: HIDDEN_CELLS }, (_, j) =>
    CELL_X + 8 + j * (HIDDEN_CELL_W + HIDDEN_GAP)
  );
  const hiddenTotalW = HIDDEN_CELLS * HIDDEN_CELL_W + (HIDDEN_CELLS - 1) * HIDDEN_GAP;
  const hiddenStartX = CELL_X + (CELL_W - hiddenTotalW) / 2;

  const totalTokensW = N_TOKENS * (TOKEN_W + TOKEN_GAP) - TOKEN_GAP;
  const offsetX = (W - totalTokensW - CELL_W - 40) / 2;

  return (
    <svg
      ref={svgRef}
      viewBox={`0 0 ${W} ${H}`}
      width="100%"
      style={{ display: 'block', height: 240 }}
      aria-label="Minh hoạ LSTM/GRU sequence model"
    >
      {/* Label: sequence */}
      <text x={offsetX + totalTokensW / 2} y={26} textAnchor="middle"
        fontSize="10" fill="var(--text-3)" fontFamily="var(--font-ui)"
        letterSpacing="0.08em" style={{ textTransform: 'uppercase' }}>
        Chuỗi giá (bước thời gian)
      </text>

      {/* Token rectangles */}
      <g transform={`translate(${offsetX}, 0)`}>
        {tokens.map((tok, i) => (
          <g key={i}>
            <rect
              id={`tok-${i}`}
              x={tok.x} y={SEQ_Y}
              width={TOKEN_W} height={TOKEN_H}
              rx={2}
              fill="var(--surface-2)"
            />
            <text
              id={`tok-txt-${i}`}
              x={tok.x + TOKEN_W / 2} y={SEQ_Y + TOKEN_H / 2 + 4}
              textAnchor="middle" fontSize="8"
              fill="var(--text-2)"
              fontFamily="var(--font-mono)"
            >
              {tok.label}
            </text>
          </g>
        ))}

        {/* Flying token (active) */}
        <rect
          id="flying-tok"
          x={0} y={SEQ_Y}
          width={TOKEN_W} height={TOKEN_H}
          rx={2}
          fill="var(--accent)"
          opacity={0}
          style={{ filter: 'drop-shadow(0 0 4px var(--accent))' }}
        />
        <text
          id="flying-tok-txt"
          x={TOKEN_W / 2} y={SEQ_Y + TOKEN_H / 2 + 4}
          textAnchor="middle" fontSize="8"
          fill="var(--bg)"
          fontFamily="var(--font-mono)"
          opacity={0}
        />

        {/* Connection arrow from last token to cell */}
        <line
          id="conn-arrow"
          x1={totalTokensW} y1={SEQ_Y + TOKEN_H / 2}
          x2={totalTokensW + 28} y2={SEQ_Y + TOKEN_H / 2}
          stroke="var(--accent)" strokeWidth="1.5"
          strokeDasharray="4 3"
          opacity={0.15}
          markerEnd="url(#arrow-acc)"
        />
      </g>

      {/* Arrow marker */}
      <defs>
        <marker id="arrow-acc" markerWidth="6" markerHeight="6"
          refX="3" refY="3" orient="auto">
          <path d="M0,0 L0,6 L6,3 Z" fill="var(--accent)" />
        </marker>
        <marker id="arrow-acc2" markerWidth="6" markerHeight="6"
          refX="3" refY="3" orient="auto">
          <path d="M0,0 L0,6 L6,3 Z" fill="var(--up)" />
        </marker>
      </defs>

      {/* Memory cell box */}
      <rect
        id="mem-cell"
        x={CELL_X} y={CELL_Y}
        width={CELL_W} height={CELL_H}
        rx={3}
        fill="var(--surface)"
        stroke="var(--accent)"
        strokeWidth={1.5}
      />
      <text
        x={CELL_X + CELL_W / 2} y={CELL_Y + 16}
        textAnchor="middle" fontSize="10"
        fill="var(--text-3)" fontFamily="var(--font-ui)"
        letterSpacing="0.06em"
      >
        ÔNG NHỚ (Cell)
      </text>
      <text
        x={CELL_X + CELL_W / 2} y={CELL_Y + 30}
        textAnchor="middle" fontSize="9"
        fill="var(--text-3)" fontFamily="var(--font-ui)"
      >
        Hidden State
      </text>

      {/* Hidden state bars (mini bar chart inside cell area below) */}
      <text
        x={CELL_X + CELL_W / 2} y={HIDDEN_Y - 6}
        textAnchor="middle" fontSize="9"
        fill="var(--text-3)" fontFamily="var(--font-ui)"
      >
        Trạng thái ẩn
      </text>
      {hiddenXs.map((hx, j) => (
        <rect
          key={j}
          id={`hid-bar-${j}`}
          x={hiddenStartX + j * (HIDDEN_CELL_W + HIDDEN_GAP)}
          y={HIDDEN_Y}
          width={HIDDEN_CELL_W}
          height={HIDDEN_CELL_H * 0.3}
          rx={1}
          fill="var(--accent)"
          opacity={0.4}
        />
      ))}

      {/* Predict output arrow */}
      <g id="pred-arrow" opacity={0}>
        <line
          x1={CELL_X + CELL_W / 2} y1={CELL_Y + CELL_H + 2}
          x2={CELL_X + CELL_W / 2} y2={CELL_Y + CELL_H + 26}
          stroke="var(--up)" strokeWidth="2"
          markerEnd="url(#arrow-acc2)"
        />
        <rect
          x={CELL_X + 8} y={CELL_Y + CELL_H + 28}
          width={CELL_W - 16} height={18}
          rx={2} fill="var(--up-bg)"
          stroke="var(--up)" strokeWidth="1"
        />
        <text
          x={CELL_X + CELL_W / 2} y={CELL_Y + CELL_H + 41}
          textAnchor="middle" fontSize="9"
          fill="var(--up)" fontFamily="var(--font-ui)"
          fontWeight="600"
        >
          Dự đoán bước kế
        </text>
      </g>

      {/* Bottom label */}
      <text
        x={W / 2} y={H - 6}
        textAnchor="middle" fontSize="11"
        fill="var(--text-3)" fontFamily="var(--font-ui)"
      >
        Mỗi token nạp tuần tự vào ô nhớ · trạng thái ẩn cập nhật liên tục
      </text>
    </svg>
  );
}
