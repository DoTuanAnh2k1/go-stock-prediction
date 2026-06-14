# Mobile UI Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix horizontal scroll on mobile and add Settings/Monitoring navigation items to the bottom nav after login.

**Architecture:** Three independent edits — one TSX file (Users.tsx table wrap), one CSS file (topbar mobile tightening), one component (MobNav in ui.tsx becomes auth-aware). No new files, no new dependencies.

**Tech Stack:** React + TypeScript, plain CSS (`styles.css`), React Router `NavLink`.

---

## Correction vs spec

After inspecting the actual code, `MarketPredictions.tsx:261` and `MarketTraining.tsx:183` already have `overflowX: 'auto'` wrappers — the table tags appear inside them. The only table without a wrapper is `Users.tsx:142`. The plan reflects this.

---

## Files

| File | Change |
|------|--------|
| `frontend/src/pages/Users.tsx` | Wrap bare `<table>` with `<div style={{overflowX:'auto'}}>` |
| `frontend/src/styles.css` | Add 2 rules to `@media (max-width: 599px)` block for topbar |
| `frontend/src/components/ui.tsx` | `MobNav` reads `useAuth()`, appends conditional nav items |

---

## Task 1: Wrap Users table

**Files:**
- Modify: `frontend/src/pages/Users.tsx` around line 142

Context: The user list renders a bare `<table>` with no scroll wrapper. On narrow screens the 5-column table overflows horizontally.

- [ ] **Step 1: Open the file and find the table**

  In `frontend/src/pages/Users.tsx`, locate this block (around line 140–145):

  ```tsx
        ) : (
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
  ```

- [ ] **Step 2: Wrap it**

  Change:

  ```tsx
        ) : (
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
  ```

  To:

  ```tsx
        ) : (
          <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
  ```

  Then find the closing `</table>` that matches this table (it ends the `users.map(...)` tbody, before the closing `</Panel>`), and add `</div>` after it:

  ```tsx
            </tbody>
          </table>
          </div>
  ```

- [ ] **Step 3: Verify TypeScript compiles**

  ```bash
  cd /home/chronical/Projects/private/go-stock-prediction/frontend
  npx tsc --noEmit 2>&1 | head -20
  ```

  Expected: no errors.

- [ ] **Step 4: Commit**

  ```bash
  git add frontend/src/pages/Users.tsx
  git commit -m "fix: wrap Users table with overflow-x auto for mobile scroll"
  ```

---

## Task 2: Tighten topbar on mobile

**Files:**
- Modify: `frontend/src/styles.css` — `@media (max-width: 599px)` block

Context: On mobile the topbar (height 44px) still shows bell button, VI/EN toggle, sun/moon toggle, and auth section with `gap: 16px`. On a 375px screen this is cramped. Reducing gap and hiding the unused bell button gives breathing room.

- [ ] **Step 1: Find the mobile media query**

  In `frontend/src/styles.css`, find this block (around line 565):

  ```css
  /* ---- MOBILE (no sidebar, bottom nav) ---- */
  @media (max-width: 599px) {
    .app { grid-template-columns: 1fr; }
    .sidebar { display: none; }
    .content { padding: 10px 10px 68px; }
    .topbar { padding: 0 10px; height: 44px; }
    .topbar__crumb { display: none; }
    .search { display: none; }
    .ticker { display: none; }
    .mob-nav { display: flex; }
    .kpi__value { font-size: 20px; }
  }
  ```

- [ ] **Step 2: Add two rules inside the block**

  Change:

  ```css
    .topbar { padding: 0 10px; height: 44px; }
  ```

  To:

  ```css
    .topbar { padding: 0 10px; height: 44px; gap: 8px; }
    .topbar .btn--icon { display: none; }
  ```

  The second rule hides the bell notification button (`.btn--icon` class, which is only used on the bell in the topbar on mobile).

- [ ] **Step 3: Verify in browser (or confirm no CSS parse error)**

  ```bash
  cd /home/chronical/Projects/private/go-stock-prediction/frontend
  npx vite build 2>&1 | tail -5
  ```

  Expected: build succeeds, no CSS errors.

- [ ] **Step 4: Commit**

  ```bash
  git add frontend/src/styles.css
  git commit -m "fix: tighten topbar gap and hide bell on mobile"
  ```

---

## Task 3: MobNav auth-aware items

**Files:**
- Modify: `frontend/src/components/ui.tsx` — `MobNav` function (around line 412)

Context: `MobNav` is the fixed bottom navigation shown only on mobile (≤599px). It currently renders 7 hardcoded items from `getNav(t)`. After login, the sidebar (hidden on mobile) shows Monitoring and Settings. These two pages are unreachable on mobile. Admin users also can't reach `/admin/users`.

The bottom nav already has `overflow-x: auto` + `scrollbar-width: none`, so adding more items just scrolls horizontally — no layout change needed.

- [ ] **Step 1: Find `MobNav` in ui.tsx**

  Around line 412 in `frontend/src/components/ui.tsx`:

  ```tsx
  export function MobNav() {
    const { t } = useLanguage();
    const NAV = getNav(t);
    return (
      <nav className="mob-nav">
        {NAV.map((n) => (
          <NavLink key={n.id} to={n.path} end={n.path === '/'}
            className={({ isActive }) => `mob-nav__item ${isActive ? 'active' : ''}`}>
            <Icon name={n.icon} size={18} />
            <span>{n.label}</span>
          </NavLink>
        ))}
      </nav>
    );
  }
  ```

- [ ] **Step 2: Replace with auth-aware version**

  ```tsx
  export function MobNav() {
    const { t } = useLanguage();
    const { user } = useAuth();
    const NAV = getNav(t);

    const authItems: NavItem[] = user ? [
      { id: 'monitoring', path: '/monitoring', label: t.nav.monitoring, icon: 'activity' },
      { id: 'settings',   path: '/settings',   label: t.nav.settings,   icon: 'settings' },
      ...(user.role === 'admin'
        ? [{ id: 'users', path: '/admin/users', label: t.nav.users, icon: 'user' }]
        : []),
    ] : [];

    return (
      <nav className="mob-nav">
        {[...NAV, ...authItems].map((n) => (
          <NavLink key={n.id} to={n.path} end={n.path === '/'}
            className={({ isActive }) => `mob-nav__item ${isActive ? 'active' : ''}`}>
            <Icon name={n.icon} size={18} />
            <span>{n.label}</span>
          </NavLink>
        ))}
      </nav>
    );
  }
  ```

  Note: `useAuth` is already imported at the top of `ui.tsx` (line 4: `import { useAuth } from '../context/AuthContext';`). No new imports needed.

- [ ] **Step 3: Confirm i18n keys exist**

  The labels use `t.nav.monitoring`, `t.nav.settings`, `t.nav.users`. Verify these exist:

  ```bash
  grep -n "monitoring\|settings\|users" /home/chronical/Projects/private/go-stock-prediction/frontend/src/i18n.ts | grep "nav\."
  ```

  Expected: lines showing `monitoring:`, `settings:`, `users:` under the `nav` section in both VI and EN translations.

- [ ] **Step 4: Verify TypeScript compiles**

  ```bash
  cd /home/chronical/Projects/private/go-stock-prediction/frontend
  npx tsc --noEmit 2>&1 | head -20
  ```

  Expected: no errors.

- [ ] **Step 5: Commit**

  ```bash
  git add frontend/src/components/ui.tsx
  git commit -m "feat: MobNav shows Monitoring and Settings after login on mobile"
  ```

---

## Manual verification checklist

After all tasks done, open the app in a browser with mobile viewport (375px width, DevTools):

- [ ] Navigate to `/admin/users` (as admin) — table scrolls horizontally without causing page-level horizontal scroll
- [ ] Log in → bottom nav shows Monitoring and Settings items (scrollable)
- [ ] Log in as admin → bottom nav also shows Users item
- [ ] Log out → Monitoring/Settings/Users items disappear from bottom nav
- [ ] Topbar is not cramped — bell icon is hidden, gap is tighter
- [ ] Tap Monitoring in bottom nav → `/monitoring` loads correctly
- [ ] Tap Settings in bottom nav → `/settings` loads correctly
