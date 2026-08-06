import { describe, it, expect } from 'vitest';
import { translations } from '../i18n';

describe('i18n translations', () => {
  it('has both VI and EN locales', () => {
    expect(translations).toHaveProperty('vi');
    expect(translations).toHaveProperty('en');
  });

  it('VI nav.overview is Vietnamese', () => {
    expect(translations.vi.nav.overview).toBe('Tổng quan');
  });

  it('EN nav.overview is English', () => {
    expect(translations.en.nav.overview).toBe('Overview');
  });

  it('VI and EN have the same top-level keys', () => {
    const viKeys = Object.keys(translations.vi).sort();
    const enKeys = Object.keys(translations.en).sort();
    expect(viKeys).toEqual(enKeys);
  });

  it('nav section has all expected keys in both locales', () => {
    const required = ['overview', 'markets', 'simulation', 'guide', 'settings', 'users', 'docs'];
    for (const key of required) {
      expect(translations.vi.nav).toHaveProperty(key);
      expect(translations.en.nav).toHaveProperty(key);
    }
  });

  it('page titles are tuples [display, breadcrumb] for the dashboard route', () => {
    const dashVI = translations.vi.titles['/'];
    const dashEN = translations.en.titles['/'];
    expect(Array.isArray(dashVI)).toBe(true);
    expect(dashVI).toHaveLength(2);
    expect(Array.isArray(dashEN)).toBe(true);
    expect(dashEN).toHaveLength(2);
    // Breadcrumb is always "DASHBOARD"
    expect(dashEN[1]).toBe('DASHBOARD');
  });

  it('common.loading is not empty in either locale', () => {
    expect(translations.vi.common.loading.length).toBeGreaterThan(0);
    expect(translations.en.common.loading.length).toBeGreaterThan(0);
  });
});
