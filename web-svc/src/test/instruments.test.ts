import { describe, it, expect } from 'vitest';
import {
  GOLD_INSTRUMENTS,
  NASDAQ_INSTRUMENTS,
  CRYPTO_INSTRUMENTS,
  instrumentsFor,
} from '../constants/instruments';

describe('instrument constants', () => {
  it('GOLD_INSTRUMENTS includes XAU', () => {
    expect(GOLD_INSTRUMENTS.some((i) => i.key === 'XAU')).toBe(true);
  });

  it('NASDAQ_INSTRUMENTS includes AAPL and NVDA', () => {
    const keys = NASDAQ_INSTRUMENTS.map((i) => i.key);
    expect(keys).toContain('AAPL');
    expect(keys).toContain('NVDA');
  });

  it('CRYPTO_INSTRUMENTS has exactly 3 entries', () => {
    expect(CRYPTO_INSTRUMENTS).toHaveLength(3);
  });

  it('every instrument has a non-empty key and label', () => {
    const all = [...GOLD_INSTRUMENTS, ...NASDAQ_INSTRUMENTS, ...CRYPTO_INSTRUMENTS];
    for (const inst of all) {
      expect(inst.key.length).toBeGreaterThan(0);
      expect(inst.label.length).toBeGreaterThan(0);
    }
  });
});

describe('instrumentsFor', () => {
  it('returns NASDAQ list for nasdaq100', () => {
    const result = instrumentsFor('nasdaq100');
    expect(result).toBe(NASDAQ_INSTRUMENTS);
  });

  it('returns CRYPTO list for crypto', () => {
    expect(instrumentsFor('crypto')).toBe(CRYPTO_INSTRUMENTS);
  });

  it('returns GOLD list for gold', () => {
    expect(instrumentsFor('gold')).toBe(GOLD_INSTRUMENTS);
  });

  it('falls back to GOLD list for unknown keys', () => {
    expect(instrumentsFor('sp500')).toBe(GOLD_INSTRUMENTS);
    expect(instrumentsFor('')).toBe(GOLD_INSTRUMENTS);
  });
});
