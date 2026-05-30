import { useState, useRef, useMemo } from 'react';

// ── Types ────────────────────────────────────────────────────────────────────
interface SparklineProps {
  data: number[];
  w?: number;
  h?: number;
  color?: string;
  fill?: boolean;
  strokeW?: number;
}

interface SeriesItem {
  name: string;
  data: (number | null)[];
  color: string;
  w?: number;
  dash?: string;
}

interface LineChartProps {
  series: SeriesItem[];
  labels: string[];
  height?: number;
  yFmt?: (v: number) => string;
  valueFmt?: (v: number) => string;
  area?: boolean;
  showGrid?: boolean;
  padL?: number;
}

interface BarChartProps {
  data: number[];
  labels: string[];
  color?: string;
  height?: number;
  yFmt?: (v: number) => string;
  valueFmt?: (v: number) => string;
  colorByValue?: boolean;
}

interface HBarItem {
  label: string;
  value: number;
  color: string;
}

interface HBarsProps {
  items: HBarItem[];
  max?: number;
  suffix?: string;
}

interface ScatterPoint {
  x: number;
  y: number;
  label: string;
  color?: string;
}

interface ScatterProps {
  points: ScatterPoint[];
  height?: number;
  xLabel?: string;
  yLabel?: string;
}

interface DonutProps {
  value: number;
  size?: number;
  stroke?: number;
  color?: string;
  label?: string;
}

// ── Helpers ──────────────────────────────────────────────────────────────────
function niceExtent(min: number, max: number, pad = 0.08): [number, number] {
  if (min === max) { min -= 1; max += 1; }
  const d = (max - min) * pad;
  return [min - d, max + d];
}

// ── Sparkline ────────────────────────────────────────────────────────────────
export function Sparkline({ data, w = 120, h = 34, color, fill = true, strokeW = 1.5 }: SparklineProps) {
  if (!data || data.length < 2) return null;
  const min = Math.min(...data), max = Math.max(...data);
  const [lo, hi] = niceExtent(min, max, 0.12);
  const dx = w / (data.length - 1);
  const yv = (v: number) => h - ((v - lo) / (hi - lo)) * h;
  const pts = data.map((v, i) => [i * dx, yv(v)]);
  const line = pts.map((p, i) => (i ? 'L' : 'M') + p[0].toFixed(1) + ',' + p[1].toFixed(1)).join(' ');
  const up = data[data.length - 1] >= data[0];
  const c = color || (up ? 'var(--up)' : 'var(--down)');
  const id = useMemo(() => 'sp' + Math.random().toString(36).slice(2, 8), []);
  return (
    <svg width={w} height={h} style={{ display: 'block' }} preserveAspectRatio="none">
      {fill && (
        <>
          <defs>
            <linearGradient id={id} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={c} stopOpacity="0.22" />
              <stop offset="100%" stopColor={c} stopOpacity="0" />
            </linearGradient>
          </defs>
          <path d={`${line} L${w},${h} L0,${h} Z`} fill={`url(#${id})`} />
        </>
      )}
      <path d={line} fill="none" stroke={c} strokeWidth={strokeW} strokeLinejoin="round" strokeLinecap="round" vectorEffect="non-scaling-stroke" />
    </svg>
  );
}

// ── LineChart ─────────────────────────────────────────────────────────────────
export function LineChart({ series, labels, height = 280, yFmt, valueFmt, area = false, showGrid = true, padL = 46 }: LineChartProps) {
  const ref = useRef<SVGSVGElement>(null);
  const [hover, setHover] = useState<{ idx: number; cx: number; cy: number } | null>(null);
  const W = 800, H = height;
  const padR = 14, padT = 14, padB = 26;
  const all = series.flatMap((s) => (s.data || []).filter((v): v is number => v != null));
  if (!all.length || labels.length < 2) {
    return <div style={{ height, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--text-3)', fontSize: 12 }}>Không có dữ liệu</div>;
  }
  const [lo, hi] = niceExtent(Math.min(...all), Math.max(...all), 0.08);
  const n = labels.length;
  const x = (i: number) => padL + (i / (n - 1)) * (W - padL - padR);
  const y = (v: number) => padT + (1 - (v - lo) / (hi - lo)) * (H - padT - padB);
  const fmtY = yFmt || ((v: number) => v.toFixed(0));
  const fmtV = valueFmt || ((v: number) => v.toFixed(2));
  const ticks = 4;
  const yTicks = Array.from({ length: ticks + 1 }, (_, i) => lo + (i / ticks) * (hi - lo));
  const xStep = Math.max(1, Math.ceil(n / 7));

  function move(e: React.MouseEvent) {
    if (!ref.current) return;
    const r = ref.current.getBoundingClientRect();
    const px = ((e.clientX - r.left) / r.width) * W;
    let idx = Math.round(((px - padL) / (W - padL - padR)) * (n - 1));
    idx = Math.max(0, Math.min(n - 1, idx));
    setHover({ idx, cx: e.clientX, cy: r.top });
  }

  return (
    <div style={{ position: 'relative' }}>
      <svg ref={ref} viewBox={`0 0 ${W} ${H}`} width="100%" height={H} onMouseMove={move} onMouseLeave={() => setHover(null)} style={{ display: 'block', overflow: 'visible' }}>
        <defs>
          {series.map((s, si) => (
            <linearGradient key={si} id={`lg${si}`} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={s.color} stopOpacity="0.18" />
              <stop offset="100%" stopColor={s.color} stopOpacity="0" />
            </linearGradient>
          ))}
        </defs>
        {showGrid && yTicks.map((t, i) => (
          <g key={i}>
            <line x1={padL} x2={W - padR} y1={y(t)} y2={y(t)} stroke="var(--grid-line)" strokeWidth="1" />
            <text x={padL - 8} y={y(t) + 3} textAnchor="end" fontSize="10" fill="var(--text-3)" fontFamily="var(--font-mono)">{fmtY(t)}</text>
          </g>
        ))}
        {labels.map((l, i) => i % xStep === 0 && (
          <text key={i} x={x(i)} y={H - 8} textAnchor="middle" fontSize="10" fill="var(--text-3)" fontFamily="var(--font-mono)">{l}</text>
        ))}
        {series.map((s, si) => {
          const pts = s.data.map((v, i) => v == null ? null : [x(i), y(v as number)]);
          const segs: [number, number][][] = [];
          let cur: [number, number][] = [];
          pts.forEach((p) => { if (p) cur.push(p as [number, number]); else { if (cur.length) segs.push(cur); cur = []; } });
          if (cur.length) segs.push(cur);
          const path = segs.map((seg) => seg.map((p, i) => (i ? 'L' : 'M') + p[0].toFixed(1) + ',' + p[1].toFixed(1)).join(' ')).join(' ');
          const last = pts.filter(Boolean).slice(-1)[0] as [number, number] | undefined;
          return (
            <g key={si}>
              {area && s.data[0] != null && (
                <path d={`${path} L${x(s.data.length - 1)},${H - padB} L${padL},${H - padB} Z`} fill={`url(#lg${si})`} />
              )}
              <path d={path} fill="none" stroke={s.color} strokeWidth={s.w || 1.8} strokeDasharray={s.dash || 'none'} strokeLinejoin="round" vectorEffect="non-scaling-stroke" />
              {last && <circle cx={last[0]} cy={last[1]} r="2.5" fill={s.color} />}
            </g>
          );
        })}
        {hover && (
          <g>
            <line x1={x(hover.idx)} x2={x(hover.idx)} y1={padT} y2={H - padB} stroke="var(--border-strong)" strokeWidth="1" />
            {series.map((s, si) => s.data[hover.idx] != null && (
              <circle key={si} cx={x(hover.idx)} cy={y(s.data[hover.idx] as number)} r="3.5" fill="var(--surface)" stroke={s.color} strokeWidth="2" />
            ))}
          </g>
        )}
      </svg>
      {hover && (
        <div className="cht-tip" style={{ left: hover.cx, top: hover.cy + 14 }}>
          <div className="cht-tip__t">{labels[hover.idx]}</div>
          {series.map((s, si) => s.data[hover.idx] != null && (
            <div key={si} style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
              <span style={{ width: 8, height: 2, background: s.color, display: 'inline-block' }}></span>
              <span style={{ color: 'var(--text-3)' }}>{s.name}</span>
              <span style={{ marginLeft: 'auto', color: 'var(--text)' }}>{fmtV(s.data[hover.idx] as number)}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// ── BarChart ──────────────────────────────────────────────────────────────────
export function BarChart({ data, labels, color = 'var(--accent)', height = 260, yFmt, valueFmt, colorByValue = false }: BarChartProps) {
  const ref = useRef<SVGSVGElement>(null);
  const [hover, setHover] = useState<{ i: number; cx: number; cy: number } | null>(null);
  const W = 800, H = height, padL = 40, padR = 12, padT = 12, padB = 26;
  if (!data || !data.length) {
    return <div style={{ height, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--text-3)', fontSize: 12 }}>Không có dữ liệu</div>;
  }
  const max = Math.max(...data, 0), min = Math.min(...data, 0);
  const lo = Math.min(0, min) * 1.1, hi = max * 1.12;
  const n = data.length;
  const bw = (W - padL - padR) / n;
  const y = (v: number) => padT + (1 - (v - lo) / (hi - lo)) * (H - padT - padB);
  const fmtY = yFmt || ((v: number) => v.toFixed(0));
  const fmtV = valueFmt || ((v: number) => v.toFixed(1));
  const yTicks = Array.from({ length: 5 }, (_, i) => lo + (i / 4) * (hi - lo));
  return (
    <div style={{ position: 'relative' }}>
      <svg ref={ref} viewBox={`0 0 ${W} ${H}`} width="100%" height={H} style={{ display: 'block', overflow: 'visible' }}>
        {yTicks.map((t, i) => (
          <g key={i}>
            <line x1={padL} x2={W - padR} y1={y(t)} y2={y(t)} stroke="var(--grid-line)" />
            <text x={padL - 8} y={y(t) + 3} textAnchor="end" fontSize="10" fill="var(--text-3)" fontFamily="var(--font-mono)">{fmtY(t)}</text>
          </g>
        ))}
        {data.map((v, i) => {
          const c = colorByValue ? (v >= 0 ? 'var(--up)' : 'var(--down)') : color;
          const yy = v >= 0 ? y(v) : y(0);
          const hh = Math.abs(y(v) - y(0));
          return (
            <g key={i}
              onMouseEnter={() => {
                if (!ref.current) return;
                const r = ref.current.getBoundingClientRect();
                setHover({ i, cx: r.left + (padL + bw * (i + 0.5)) / W * r.width, cy: r.top + yy });
              }}
              onMouseLeave={() => setHover(null)}>
              <rect x={padL + bw * i + bw * 0.18} y={yy} width={bw * 0.64} height={Math.max(1, hh)} fill={c} opacity={hover && hover.i === i ? 1 : 0.82} />
              <text x={padL + bw * (i + 0.5)} y={H - 8} textAnchor="middle" fontSize="10" fill="var(--text-3)" fontFamily="var(--font-mono)">{labels[i]}</text>
            </g>
          );
        })}
      </svg>
      {hover && (
        <div className="cht-tip" style={{ left: hover.cx, top: hover.cy }}>
          <div className="cht-tip__t">{labels[hover.i]}</div>
          <div style={{ color: 'var(--text)' }}>{fmtV(data[hover.i])}</div>
        </div>
      )}
    </div>
  );
}

// ── HBars ─────────────────────────────────────────────────────────────────────
export function HBars({ items, max = 100, suffix = '%' }: HBarsProps) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
      {items.map((it, i) => (
        <div key={i}>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 6, fontSize: 12.5 }}>
            <span style={{ fontWeight: 500 }}>{it.label}</span>
            <span className="num" style={{ color: it.color }}>{it.value.toFixed(1)}{suffix}</span>
          </div>
          <div className="bar" style={{ height: 8 }}>
            <div className="bar__fill" style={{ width: (it.value / max * 100) + '%', background: it.color }}></div>
          </div>
        </div>
      ))}
    </div>
  );
}

// ── Scatter ───────────────────────────────────────────────────────────────────
export function Scatter({ points, height = 320, xLabel }: ScatterProps) {
  const ref = useRef<SVGSVGElement>(null);
  const [hover, setHover] = useState<number | null>(null);
  const W = 800, H = height, padL = 44, padR = 14, padT = 14, padB = 34;
  if (!points || !points.length) {
    return <div style={{ height, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--text-3)', fontSize: 12 }}>Không có dữ liệu</div>;
  }
  const xs = points.map((p) => p.x), ys = points.map((p) => p.y);
  const [xlo, xhi] = niceExtent(Math.min(...xs), Math.max(...xs));
  const [ylo, yhi] = niceExtent(0, Math.max(...ys, 0.001));
  const x = (v: number) => padL + ((v - xlo) / (xhi - xlo)) * (W - padL - padR);
  const y = (v: number) => padT + (1 - (v - ylo) / (yhi - ylo)) * (H - padT - padB);
  const yTicks = Array.from({ length: 5 }, (_, i) => ylo + (i / 4) * (yhi - ylo));
  const xTicks = Array.from({ length: 5 }, (_, i) => xlo + (i / 4) * (xhi - xlo));
  return (
    <div style={{ position: 'relative' }}>
      <svg ref={ref} viewBox={`0 0 ${W} ${H}`} width="100%" height={H} style={{ display: 'block', overflow: 'visible' }}>
        {yTicks.map((t, i) => (
          <g key={i}>
            <line x1={padL} x2={W - padR} y1={y(t)} y2={y(t)} stroke="var(--grid-line)" />
            <text x={padL - 8} y={y(t) + 3} textAnchor="end" fontSize="10" fill="var(--text-3)" fontFamily="var(--font-mono)">{t.toFixed(1)}</text>
          </g>
        ))}
        {xTicks.map((t, i) => (
          <text key={i} x={x(t)} y={H - 12} textAnchor="middle" fontSize="10" fill="var(--text-3)" fontFamily="var(--font-mono)">{t.toFixed(1)}</text>
        ))}
        <line x1={x(0)} x2={x(0)} y1={padT} y2={H - padB} stroke="var(--border-strong)" strokeDasharray="3 3" />
        {points.map((p, i) => (
          <circle key={i} cx={x(p.x)} cy={y(p.y)} r={hover === i ? 5 : 3.4}
            fill={p.color || 'var(--accent)'} opacity={0.78}
            onMouseEnter={() => setHover(i)} onMouseLeave={() => setHover(null)} style={{ cursor: 'pointer' }} />
        ))}
        {xLabel && <text x={(W + padL) / 2} y={H} textAnchor="middle" fontSize="10.5" fill="var(--text-3)">{xLabel}</text>}
      </svg>
      {hover != null && (
        <div className="cht-tip" style={{ left: '50%', top: 8, position: 'absolute', transform: 'translateX(-50%)' }}>
          <span style={{ color: 'var(--text)' }}>{points[hover].label}</span>
        </div>
      )}
    </div>
  );
}

// ── Donut ─────────────────────────────────────────────────────────────────────
export function Donut({ value, size = 92, stroke = 9, color = 'var(--accent)', label }: DonutProps) {
  const safeVal = isFinite(value) ? value : 0;
  const r = (size - stroke) / 2, c = 2 * Math.PI * r;
  const off = c * (1 - safeVal / 100);
  return (
    <div style={{ position: 'relative', width: size, height: size }}>
      <svg width={size} height={size}>
        <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke="var(--surface-3)" strokeWidth={stroke} />
        <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke={color} strokeWidth={stroke}
          strokeDasharray={c} strokeDashoffset={off} strokeLinecap="butt" transform={`rotate(-90 ${size / 2} ${size / 2})`} />
      </svg>
      <div style={{ position: 'absolute', inset: 0, display: 'grid', placeItems: 'center', textAlign: 'center' }}>
        <div>
          <div className="num" style={{ fontSize: 18, fontWeight: 600 }}>{safeVal.toFixed(1)}</div>
          {label && <div style={{ fontSize: 9.5, color: 'var(--text-3)' }}>{label}</div>}
        </div>
      </div>
    </div>
  );
}
