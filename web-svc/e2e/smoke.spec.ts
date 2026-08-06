/**
 * E2E smoke test — requires a running app at BASE_URL (default http://localhost).
 *
 * Run manually:
 *   npm run e2e
 *   BASE_URL=http://localhost:3000 npm run e2e
 *
 * NOT wired into `npm run build` or `npm run test` (vitest).
 * Install browsers once: npx playwright install chromium
 */
import { test, expect } from '@playwright/test';

test('app shell loads and renders navigation', async ({ page }) => {
  await page.goto('/');

  // The page title should be set by the app (Vite sets the default in index.html)
  await expect(page).toHaveTitle(/.+/);

  // The root element should exist and contain something
  const root = page.locator('#root');
  await expect(root).toBeVisible();

  // The sidebar nav should render at least one link
  const navLinks = page.locator('nav a, aside a, [class*="sidebar"] a');
  await expect(navLinks.first()).toBeVisible({ timeout: 10_000 });
});

test('navigating to /guide does not crash the app', async ({ page }) => {
  await page.goto('/guide');

  // Should not show an error page or blank screen
  const root = page.locator('#root');
  await expect(root).toBeVisible();
  await expect(root).not.toBeEmpty();
});
