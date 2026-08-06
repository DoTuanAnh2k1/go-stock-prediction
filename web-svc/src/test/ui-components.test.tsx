import React from 'react';
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Icon, Chg } from '../components/ui';

// Icon and Chg are pure presentational components — no context required.

describe('Icon component', () => {
  it('renders an svg element', () => {
    const { container } = render(<Icon name="search" />);
    const svg = container.querySelector('svg');
    expect(svg).not.toBeNull();
  });

  it('applies default size 18', () => {
    const { container } = render(<Icon name="sun" />);
    const svg = container.querySelector('svg')!;
    expect(svg.getAttribute('width')).toBe('18');
    expect(svg.getAttribute('height')).toBe('18');
  });

  it('applies custom size', () => {
    const { container } = render(<Icon name="moon" size={24} />);
    const svg = container.querySelector('svg')!;
    expect(svg.getAttribute('width')).toBe('24');
    expect(svg.getAttribute('height')).toBe('24');
  });

  it('forwards extra props (data-testid)', () => {
    render(<Icon name="bell" data-testid="icon-bell" />);
    expect(screen.getByTestId('icon-bell')).toBeTruthy();
  });
});

describe('Chg component — badge mode', () => {
  it('shows UP arrow and green-ish class for positive pct', () => {
    const { container } = render(<Chg pct={2.5} badge />);
    const span = container.querySelector('span')!;
    expect(span.className).toContain('badge--up');
    expect(span.textContent).toContain('▲');
  });

  it('shows DOWN arrow for negative pct', () => {
    const { container } = render(<Chg pct={-1.2} badge />);
    const span = container.querySelector('span')!;
    expect(span.className).toContain('badge--down');
    expect(span.textContent).toContain('▼');
  });

  it('shows dash for flat pct (within ±0.01)', () => {
    const { container } = render(<Chg pct={0} badge />);
    const span = container.querySelector('span')!;
    expect(span.className).toContain('badge--muted');
    expect(span.textContent).toContain('–');
  });
});

describe('Chg component — inline mode', () => {
  it('renders positive pct with + sign', () => {
    const { container } = render(<Chg pct={3.14} />);
    expect(container.textContent).toContain('+3.14%');
  });

  it('renders negative pct with minus sign', () => {
    const { container } = render(<Chg pct={-0.75} />);
    expect(container.textContent).toContain('-0.75%');
  });

  it('renders abs value when provided', () => {
    const { container } = render(<Chg pct={1.5} abs={150} />);
    expect(container.textContent).toContain('+150.00');
  });

  it('applies custom fmt', () => {
    const fmt = {
      vnd: (n: number) => `${n}`,
      price: (n: number) => `${n}`,
      pct: (_: number) => 'CUSTOM_PCT',
      sign: (_: number) => 'CUSTOM_SIGN',
      compact: (n: number) => `${n}`,
      goldShort: (n: number) => `${n}`,
    };
    const { container } = render(<Chg pct={5} fmt={fmt} />);
    expect(container.textContent).toContain('CUSTOM_PCT');
  });
});
