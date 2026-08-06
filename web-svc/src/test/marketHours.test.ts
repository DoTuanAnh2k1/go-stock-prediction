import { describe, it, expect } from 'vitest';
import { marketStatus } from '../utils/marketHours';

// All timestamps are created via Date constructor so they reflect the actual
// UTC instant; the module converts to Eastern internally.

/** Saturday 2026-08-01 UTC noon — definitely a weekend in Eastern too */
const SAT = new Date('2026-08-01T12:00:00Z');
/** Monday 2026-08-03 UTC noon — weekday, not a holiday */
const MON = new Date('2026-08-03T12:00:00Z');
/** 2026-01-01 is New Year's Day (Thursday) — NYSE holiday */
const NEW_YEARS_2026 = new Date('2026-01-01T16:00:00Z');
/** 2026-07-04 is Independence Day (Saturday) → observed Friday 2026-07-03 */
const JULY4_2026_SAT = new Date('2026-07-04T16:00:00Z');
const JULY3_2026_FRI = new Date('2026-07-03T16:00:00Z');

describe('marketStatus', () => {
  describe('non-NYSE markets (GOLD, CRYPTO) always open', () => {
    it('GOLD is open on a Saturday', () => {
      expect(marketStatus('GOLD', SAT).open).toBe(true);
    });
    it('CRYPTO is open on a Saturday', () => {
      expect(marketStatus('CRYPTO', SAT).open).toBe(true);
    });
    it('unknown market key is open', () => {
      expect(marketStatus('', SAT).open).toBe(true);
    });
    it('reason is null for always-open markets', () => {
      expect(marketStatus('GOLD', SAT).reason).toBeNull();
    });
  });

  describe('NASDAQ weekend closure', () => {
    it('NASDAQ is closed on Saturday', () => {
      const s = marketStatus('NASDAQ', SAT);
      expect(s.open).toBe(false);
      expect(s.reason).toBe('weekend');
    });
    it('NASDAQ100 is closed on Saturday', () => {
      expect(marketStatus('NASDAQ100', SAT).open).toBe(false);
    });
    it('SP500 is closed on Saturday', () => {
      expect(marketStatus('SP500', SAT).open).toBe(false);
    });
    it('NASDAQ is open on a regular Monday', () => {
      expect(marketStatus('NASDAQ', MON).open).toBe(true);
    });
  });

  describe('NYSE holiday closure', () => {
    it('NASDAQ is closed on New Year\'s Day 2026 (Thursday)', () => {
      const s = marketStatus('NASDAQ', NEW_YEARS_2026);
      expect(s.open).toBe(false);
      expect(s.reason).toBe('holiday');
    });

    it('SP500 is closed on Independence Day observed (Friday 2026-07-03)', () => {
      // July 4 2026 is a Saturday → observed on Friday July 3
      const s = marketStatus('SP500', JULY3_2026_FRI);
      expect(s.open).toBe(false);
      expect(s.reason).toBe('holiday');
    });

    it('SP500 is closed on the actual Saturday Jul 4 (weekend, not holiday)', () => {
      const s = marketStatus('SP500', JULY4_2026_SAT);
      expect(s.open).toBe(false);
      // Weekend check fires first
      expect(s.reason).toBe('weekend');
    });
  });

  describe('case insensitivity', () => {
    it('accepts lowercase nasdaq', () => {
      const s = marketStatus('nasdaq', SAT);
      expect(s.open).toBe(false);
    });
    it('accepts mixed-case Nasdaq100', () => {
      const s = marketStatus('Nasdaq100', SAT);
      expect(s.open).toBe(false);
    });
  });
});
