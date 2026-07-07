import { useState, useEffect } from 'react';
import registry from './registry';

interface Props {
  spec: string;
}

function prefersReducedMotion(): boolean {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

export default function VizBlock({ spec }: Props) {
  const lines = spec.split('\n').filter((l) => l.trim() !== '');

  // First line may be "<key>" or "<key> <arg>" — split on first space
  const firstLine = lines[0]?.trim() ?? '';
  const spaceIdx  = firstLine.indexOf(' ');
  const key = spaceIdx === -1 ? firstLine : firstLine.slice(0, spaceIdx);
  const arg = spaceIdx === -1 ? undefined : firstLine.slice(spaceIdx + 1).trim() || undefined;

  // Second line (if any) is the caption
  const caption = lines[1]?.trim() ?? '';

  const entry = registry[key];

  const [playing, setPlaying] = useState(() => !prefersReducedMotion());

  useEffect(() => {
    // Respect reduced-motion changes at runtime
    const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
    const handler = (e: MediaQueryListEvent) => {
      if (e.matches) setPlaying(false);
    };
    mq.addEventListener('change', handler);
    return () => mq.removeEventListener('change', handler);
  }, []);

  if (!entry) {
    return (
      <div className="docs-viz docs-viz--fallback">
        <span className="docs-viz__fallback-msg">
          Chưa có minh hoạ: <code>{key}</code>
        </span>
      </div>
    );
  }

  const Comp = entry.component;
  const captionText = caption || entry.defaultCaption;
  const isConceptOnly = entry.dataDriven === false;
  // Components with ownControls manage their own play/pause — hide the outer button
  const hasOwnControls = entry.ownControls === true;

  return (
    <figure className="docs-viz">
      <div className="docs-viz__canvas">
        {/* Concept-only badge (top-left) for non-data-driven viz */}
        {isConceptOnly && (
          <span className="docs-viz__concept-badge" aria-label="Minh hoạ khái niệm">
            Minh hoạ khái niệm
          </span>
        )}

        {/* Outer play/stop button — hidden for components that own their controls */}
        {!hasOwnControls && (
          <button
            className={`docs-viz__play-btn${playing ? ' docs-viz__play-btn--stop' : ''}`}
            onClick={() => setPlaying((p) => !p)}
            aria-label={playing ? 'Dừng animation' : 'Phát animation'}
            title={playing ? 'Dừng' : 'Phát'}
          >
            {playing ? (
              <svg width="12" height="12" viewBox="0 0 12 12" fill="currentColor">
                <rect x="2" y="1" width="3" height="10" rx="1" />
                <rect x="7" y="1" width="3" height="10" rx="1" />
              </svg>
            ) : (
              <svg width="12" height="12" viewBox="0 0 12 12" fill="currentColor">
                <path d="M2.5 1.5L10.5 6L2.5 10.5V1.5Z" />
              </svg>
            )}
            {playing ? 'Dừng' : 'Phát'}
          </button>
        )}

        <Comp playing={playing} arg={arg} />
      </div>
      {captionText && (
        <figcaption className="docs-viz__caption">{captionText}</figcaption>
      )}
    </figure>
  );
}
