import { useEffect, useRef, useId, useState, useCallback } from 'react';
import { createPortal } from 'react-dom';

interface Props {
  code: string;
}

function getTheme(): string {
  const attr = document.documentElement.getAttribute('data-theme');
  return attr === 'dark' || attr === null ? 'dark' : 'default';
}

function getAccent(): string {
  const v = getComputedStyle(document.documentElement)
    .getPropertyValue('--accent-override')
    .trim();
  return v || '#5B8DEF';
}

function prefersReducedMotion(): boolean {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

// ── Zoom/pan state ─────────────────────────────────────────────────────────────

interface Transform {
  scale: number;
  tx: number;
  ty: number;
}

const SCALE_MIN = 0.5;
const SCALE_MAX = 4;
const SCALE_STEP = 0.25;

function clampScale(s: number): number {
  return Math.max(SCALE_MIN, Math.min(SCALE_MAX, s));
}

// ── Inner diagram renderer + pan/zoom ─────────────────────────────────────────

interface DiagramPaneProps {
  code: string;
  renderId: string;
  fullscreen?: boolean;
}

function DiagramPane({ code, renderId, fullscreen = false }: DiagramPaneProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const wrapRef = useRef<HTMLDivElement>(null);
  const [tf, setTf] = useState<Transform>({ scale: 1, tx: 0, ty: 0 });
  const dragRef = useRef<{ startX: number; startY: number; tx: number; ty: number } | null>(null);

  // ── Render mermaid diagram ──────────────────────────────────────────────────

  useEffect(() => {
    let cancelled = false;
    const container = containerRef.current;
    if (!container) return;

    async function render(theme: string) {
      if (cancelled || !container) return;
      try {
        const mermaid = (await import('mermaid')).default;
        const accent = getAccent();
        mermaid.initialize({
          startOnLoad: false,
          theme: theme as 'dark' | 'default',
          themeVariables:
            theme === 'dark'
              ? {
                  primaryColor: accent,
                  primaryTextColor: '#e8eaf0',
                  lineColor: '#5c6070',
                  edgeLabelBackground: '#1e2132',
                  nodeBorder: accent,
                  mainBkg: '#252838',
                  clusterBkg: '#1e2132',
                }
              : {
                  primaryColor: accent,
                  primaryTextColor: '#1a1d2e',
                  lineColor: '#8c93a8',
                  edgeLabelBackground: '#f4f5f8',
                  nodeBorder: accent,
                  mainBkg: '#ffffff',
                  clusterBkg: '#f4f5f8',
                },
        });

        const id = `${renderId}-${theme}${fullscreen ? '-fs' : ''}`;
        const { svg } = await mermaid.render(id, code);
        if (cancelled || !container) return;
        container.innerHTML = svg;
      } catch {
        if (cancelled || !container) return;
        container.innerHTML = '';
        const pre = document.createElement('pre');
        pre.style.cssText =
          'padding:12px;font-size:12px;color:var(--text-3);overflow-x:auto;';
        pre.textContent = code;
        const err = document.createElement('div');
        err.style.cssText =
          'font-size:12px;color:var(--down);padding:4px 12px 8px;font-family:var(--font-ui)';
        err.textContent = 'Sơ đồ lỗi cú pháp';
        container.appendChild(pre);
        container.appendChild(err);
      }
    }

    const currentTheme = getTheme();
    render(currentTheme);

    const observer = new MutationObserver(() => {
      const newTheme = getTheme();
      render(newTheme);
    });
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['data-theme'],
    });

    return () => {
      cancelled = true;
      observer.disconnect();
    };
  }, [code, renderId, fullscreen]);

  // ── Wheel zoom ────────────────────────────────────────────────────────────

  const onWheel = useCallback((e: React.WheelEvent<HTMLDivElement>) => {
    e.preventDefault();
    const delta = e.deltaY > 0 ? -SCALE_STEP : SCALE_STEP;
    setTf((prev) => ({ ...prev, scale: clampScale(prev.scale + delta) }));
  }, []);

  // ── Drag pan ──────────────────────────────────────────────────────────────

  const onMouseDown = useCallback((e: React.MouseEvent<HTMLDivElement>) => {
    if (e.button !== 0) return;
    setTf((prev) => {
      dragRef.current = { startX: e.clientX, startY: e.clientY, tx: prev.tx, ty: prev.ty };
      return prev;
    });
    // Pointer capture for smooth drag outside bounds
    const el = e.currentTarget as HTMLElement;
    if (el.setPointerCapture && e.nativeEvent instanceof PointerEvent) {
      el.setPointerCapture(e.nativeEvent.pointerId);
    }
  }, []);

  const onMouseMove = useCallback((e: React.MouseEvent<HTMLDivElement>) => {
    if (!dragRef.current) return;
    const { startX, startY, tx, ty } = dragRef.current;
    const dx = e.clientX - startX;
    const dy = e.clientY - startY;
    setTf((prev) => ({ ...prev, tx: tx + dx, ty: ty + dy }));
  }, []);

  const stopDrag = useCallback(() => {
    dragRef.current = null;
  }, []);

  // ── Reset transform ───────────────────────────────────────────────────────

  const reset = useCallback(() => {
    setTf({ scale: 1, tx: 0, ty: 0 });
  }, []);

  const zoomIn = useCallback(() => {
    setTf((prev) => ({ ...prev, scale: clampScale(prev.scale + SCALE_STEP) }));
  }, []);

  const zoomOut = useCallback(() => {
    setTf((prev) => ({ ...prev, scale: clampScale(prev.scale - SCALE_STEP) }));
  }, []);

  const reduced = prefersReducedMotion();
  const isDragging = tf.scale > 1;

  return (
    <div className={`docs-mermaid__pane${fullscreen ? ' docs-mermaid__pane--fullscreen' : ''}`}>
      {/* Toolbar */}
      <div className="docs-mermaid__toolbar">
        <button
          className="docs-mermaid__tool-btn"
          onClick={zoomIn}
          title="Phóng to"
          aria-label="Phóng to sơ đồ"
        >+</button>
        <button
          className="docs-mermaid__tool-btn"
          onClick={zoomOut}
          title="Thu nhỏ"
          aria-label="Thu nhỏ sơ đồ"
        >−</button>
        <button
          className="docs-mermaid__tool-btn"
          onClick={reset}
          title="Đặt lại"
          aria-label="Đặt lại kích thước"
        >⟲</button>
      </div>

      {/* Zoomable/pannable area */}
      <div
        ref={wrapRef}
        className={`docs-mermaid__zoom-area${isDragging ? ' docs-mermaid__zoom-area--grab' : ''}`}
        onWheel={onWheel}
        onMouseDown={onMouseDown}
        onMouseMove={onMouseMove}
        onMouseUp={stopDrag}
        onMouseLeave={stopDrag}
      >
        <div
          className="docs-mermaid__svg"
          style={{
            transform: `translate(${tf.tx}px, ${tf.ty}px) scale(${tf.scale})`,
            transformOrigin: 'center center',
            transition: reduced ? 'none' : undefined,
          }}
        >
          <div ref={containerRef} />
        </div>
      </div>
    </div>
  );
}

// ── Fullscreen overlay ────────────────────────────────────────────────────────

interface FullscreenOverlayProps {
  code: string;
  renderId: string;
  onClose: () => void;
}

function FullscreenOverlay({ code, renderId, onClose }: FullscreenOverlayProps) {
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose();
    }
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [onClose]);

  return createPortal(
    <div
      className="docs-mermaid__overlay"
      role="dialog"
      aria-modal="true"
      aria-label="Sơ đồ phóng to"
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div className="docs-mermaid__overlay-inner">
        <button
          className="docs-mermaid__overlay-close"
          onClick={onClose}
          aria-label="Đóng"
        >✕</button>
        <DiagramPane code={code} renderId={renderId} fullscreen />
      </div>
    </div>,
    document.body
  );
}

// ── Main component ────────────────────────────────────────────────────────────

export default function MermaidBlock({ code }: Props) {
  const uid = useId().replace(/:/g, '');
  const renderId = `mermaid-${uid}`;
  const [fsOpen, setFsOpen] = useState(false);

  return (
    <div className="docs-mermaid">
      {/* Fullscreen button in top-right of outer wrapper */}
      <button
        className="docs-mermaid__tool-btn docs-mermaid__fullscreen-btn"
        onClick={() => setFsOpen(true)}
        title="Phóng to toàn màn hình"
        aria-label="Mở sơ đồ toàn màn hình"
      >⛶</button>

      <DiagramPane code={code} renderId={renderId} />

      {fsOpen && (
        <FullscreenOverlay
          code={code}
          renderId={renderId}
          onClose={() => setFsOpen(false)}
        />
      )}
    </div>
  );
}
