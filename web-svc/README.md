# web-svc — Web Dashboard (React SPA)

The `web-svc` service is the user-facing dashboard for the asset-price prediction
platform. It is a **React 18 + Vite + TypeScript single-page application (SPA)**
that is compiled to static files (`dist/`) and served by **nginx on internal port
`3000`**. It is never exposed directly — the Rust **gateway-svc** terminates TLS
and proxies traffic to it (`/` → `http://web-svc:3000`, `/api` → `http://api-svc:8118`).

The dashboard surfaces the whole pipeline:

- **Markets** — Gold (SJC/XAU), NASDAQ 100, Crypto (BTC/ETH/SOL), S&P 500: latest
  prices, charts, and per-market prediction / training / session views.
- **Predictions & Training** — aggregate prediction lists, per-algorithm accuracy,
  training history and status.
- **Simulation** — bot-trading leaderboard and per-bot detail.
- **Monitoring / Data Pipeline** — crawl freshness, per-algorithm prediction
  activity, and bot win/loss tables.
- **Admin** — user management and market-group (RBAC) management.
- **Settings** — dynamic cron schedule editor and pipeline reports.

---

## 1. Tech stack

| Concern        | Choice                                                        |
|----------------|--------------------------------------------------------------|
| Framework      | React 18 (`react`, `react-dom`)                              |
| Build tool     | Vite 6 (`@vitejs/plugin-react`), TypeScript 5               |
| Routing        | `react-router-dom` v6 (`BrowserRouter`)                     |
| Auth decode    | `jwt-decode` (client-side JWT claim reading)               |
| Styling        | Hand-written CSS (`src/styles.css`, per-page `.css`), CSS variables / theming |
| Production serve| nginx serving the static `dist/` build (SPA fallback)      |

There is no UI component library and no state-management library — state lives in
three React Contexts (`Auth`, `Lang`, `Data`).

---

## 2. Auth & RBAC on the client

Auth is implemented entirely in `src/context/AuthContext.tsx` and is JWT-based.

- **Login** — `login(username, password)` POSTs to `/api/x/grant` with credentials in the `X-Token: base64("user:pass")` header (body is a generic `{"request":""}`). On success
  the JWT and a small `{username, role}` object are stored in `localStorage`
  (`vns_token`, `vns_user`). The handler tolerates both bare and `{data: ...}`
  wrapped responses.
- **Token validation** — on mount, `AuthProvider` calls `GET /api/auth/me` with the
  stored bearer token; if it fails, the token/user are cleared.
- **Claims** — the JWT is decoded client-side to read `accessible_markets: string[]`
  (and `role`, `user_id`). These drive market-level gating.
- **`canAccessMarket(marketKey)`** — returns `true` for `super_admin` and `admin`,
  otherwise checks membership in `accessible_markets`.
- **`logout()`** — clears `localStorage` and resets context state.

UI gating:

- The **login modal** auto-opens when the auth check finishes and no user is logged
  in (`App.tsx`); it is dismissible only when not `required`.
- The **sidebar** renders the admin section (`/admin/users`, `/admin/market-groups`)
  only when `role` is `admin` or `super_admin` (`src/components/ui.tsx`). The market
  list in the sidebar is filtered through `canAccessMarket()`.
- **Admin pages** read `useAuth()` to enforce finer rules client-side, e.g.
  `super_admin` can manage `admin`/`user`; `admin` can manage only `user`;
  `super_admin` rows cannot be deleted by non-super-admins (`Users.tsx`).

> Client-side gating is for UX only — the **api-svc** backend enforces real
> authorization (`AuthRequired` / `MarketRequired` / `AdminRequired`) on every call.

---

## 3. Internationalisation (i18n)

`src/context/LangContext.tsx` + `src/i18n.ts` provide a VI/EN toggle.

- `LangProvider` holds the current language (`'vi' | 'en'`), defaulting to `vi`,
  persisted in `localStorage` under key **`vns_lang`**.
- `toggleLang()` flips VI ↔ EN and persists the choice.
- `useLanguage()` exposes `{ lang, toggleLang, t }` where `t` is the active
  translation table from `i18n.ts` (nav labels, market sub-tabs, page titles /
  breadcrumbs, topbar, tweaks panel, etc.). Both `vi` and `en` tables are defined.

---

## 4. Routing & pages

Routing is declared in `src/App.tsx` (wrapped by `BrowserRouter` in `src/main.tsx`).
Every page is wrapped in an `ErrorBoundary`.

| Route                                   | Page (`src/pages/`)        | Purpose |
|-----------------------------------------|----------------------------|---------|
| `/`                                     | `Dashboard.tsx`            | Market overview / home dashboard |
| `/dashboard`                            | → redirect to `/`          | Legacy alias |
| `/markets/gold`                         | `Gold.tsx`                 | Gold prices & charts |
| `/markets/crypto`                       | `Crypto.tsx`               | Crypto prices & charts |
| `/markets/nasdaq100`                    | `Nasdaq.tsx`               | NASDAQ 100 prices & charts |
| `/markets/sp500`                        | `SP500.tsx`                | S&P 500 prices & charts |
| `/markets/:marketKey/predictions`       | `MarketPredictions.tsx`    | Paginated predictions + algorithm comparison chart per market |
| `/markets/:marketKey/training`          | `MarketTraining.tsx`       | Training-session history per market |
| `/markets/:marketKey/session`           | `SessionStats.tsx`         | Session / KPI stats per market |
| `/simulation`                           | `Simulation.tsx`           | Bot-trading leaderboard |
| `/simulation/:botId`                    | `SimulationBot.tsx`        | Single bot detail |
| `/guide`                                | `Guide.tsx`                | User guide |
| `/settings`                             | `Settings.tsx`             | Cron schedules editor + pipeline reports |
| `/monitoring`                           | `Monitoring.tsx`           | Data-pipeline monitoring (crawl freshness, per-algo predictions, bots) |
| `/admin/users`                          | `Users.tsx`                | User management (admin only) |
| `/admin/market-groups`                  | `MarketGroups.tsx`         | Market-group / RBAC management (admin only) |
| `/gold`, `/crypto`, `/nasdaq`           | → redirects                | Legacy bookmark redirects to `/markets/*` |

Other top-level pages (`Predictions.tsx`, `Training.tsx`) exist as shared building
blocks used by the market and dashboard views.

Notable page behaviours:

- **`Monitoring.tsx`** — calls `GET /api/monitoring/overview`, then filters market
  cards through `canAccessMarket()`; shows crawl freshness, per-algorithm prediction
  stats, and bot summary/full tables.
- **`Settings.tsx`** — a "cron schedule" editor that parses/formats 6-field cron
  expressions into a friendly form (every hour / every N hours / daily / weekly),
  and a pipeline-reports view. Uses `fetchSchedules` / `updateSchedule` /
  `triggerEndpoint` from the API client.
- **`Users.tsx` / `MarketGroups.tsx`** — admin CRUD over users and market groups,
  with role-aware permission checks and role badges.

---

## 5. API integration

All backend access goes through the API client in **`src/api/index.ts`**, with a
single base path constant `BASE = '/api'`. Because the SPA is served behind the
gateway, relative `/api/*` calls are routed by gateway-svc to **api-svc:8118** — the
browser never talks to api-svc directly.

- **`fetchJSON(url, opts)`** — central helper. Reads `vns_token` from `localStorage`
  and, when present, sets `Authorization: Bearer <token>`. Throws on non-2xx.
- **`loadAll()`** — fan-out loader the `DataProvider` calls on startup: dashboard
  stats, training algorithms, gold latest/predictions/chart. Algorithm display names
  are enriched at runtime from `/api/training/algorithms` (overriding a static
  fallback map); failures degrade gracefully (page enters `demo` status).
- **`fetchMarketPredictions` / `fetchMarketPredictionChart`** — per-market endpoints.
  The symbol search box maps to a *different* query param per market (`symbol` for
  NASDAQ, `coin` for crypto, `source` for gold), matching the backend contract.
- **`fetchMonitoringOverview`**, **`fetchSchedules` / `updateSchedule`**,
  **`triggerEndpoint`**, and the trigger helpers (`crawlGold`, `train`, etc.) all
  attach the bearer token.

Auth header is sourced consistently via `localStorage['vns_token']` (helper
`getToken()` / inline reads).

---

## 6. Directory structure

```
web-svc/
├── index.html               # Vite HTML entry (mounts #root)
├── package.json             # deps + scripts (dev / build / preview)
├── vite.config.ts           # Vite config; dev proxy /api → localhost:31300; terser minify
├── tsconfig.json            # TypeScript project config
├── nginx.conf               # nginx server on :3000 with SPA fallback (try_files → index.html)
└── src/
    ├── main.tsx             # App bootstrap: BrowserRouter > DataProvider > App
    ├── App.tsx              # AuthProvider > LangProvider; route table; shell (Sidebar/Topbar/Ticker/Tweaks)
    ├── styles.css           # Global styles / CSS variables / theming
    ├── i18n.ts              # VI/EN translation tables
    ├── api/
    │   └── index.ts         # API client, formatters, data builders, triggers, monitoring/schedules
    ├── context/
    │   ├── AuthContext.tsx  # JWT auth, accessible_markets, canAccessMarket()
    │   ├── LangContext.tsx  # VI/EN toggle (localStorage vns_lang)
    │   └── DataContext.tsx  # App data loader (loadAll/enrich), loading|live|demo status
    ├── components/
    │   ├── ui.tsx           # Sidebar, Topbar, Ticker, MobNav, ErrorBoundary, Panel, Icon, market tabs
    │   ├── charts.tsx       # Chart components
    │   ├── tweaks-panel.tsx # Accent/density/font/ticker UI tweaks (persisted)
    │   └── LoginModal.tsx   # Login form → AuthContext.login()
    ├── pages/               # Route pages (Dashboard, Gold, Crypto, Nasdaq, SP500,
    │                        #   MarketPredictions, MarketTraining, SessionStats,
    │                        #   Simulation, SimulationBot, Monitoring, Settings,
    │                        #   Users, MarketGroups, Guide, Predictions, Training)
    ├── constants/
    │   └── instruments.ts   # Instrument / symbol constants
    ├── types/
    │   └── index.ts         # Shared TypeScript types (AppData, DTO shapes)
    └── utils/
        └── marketHours.ts   # Market-hours helpers
```

---

## 7. Build & run

### Local development

The Vite dev server proxies `/api` to a local backend (configured in
`vite.config.ts`, default `http://localhost:31300` — adjust to your api-svc port).

```bash
cd web-svc
npm install
npm run dev          # Vite dev server with HMR
```

### Production build

```bash
cd web-svc
npm run build        # tsc -b && vite build  → outputs static dist/
npm run preview      # (optional) preview the production build locally
```

The build is minified with terser (console/debugger stripped, top-level mangle).

### Docker / Compose

The Dockerfile for this service now lives in the flat `deploy/` directory as
**`deploy/web-svc.Dockerfile`** (build context `../web-svc`). It is a two-stage
build: stage 1 (`node:22-alpine`) runs `npm run build`; stage 2 (`nginx:alpine`)
copies `dist/` into nginx and uses `nginx.conf` (listens on `:3000`, SPA fallback to
`index.html`). TLS and `/api` routing are handled by gateway-svc, not by this nginx.

From the repo root:

```bash
docker compose --env-file .env -f deploy/docker-compose.yaml up -d
```

Compose service key == container name == directory name (`web-svc`).

---

## 8. Notes

- **No direct port.** `web-svc` is internal-only on `:3000`. Browsers reach it
  through **gateway-svc** (`:80`/`:443`), which proxies `/` → `web-svc:3000`.
- **API routing.** The SPA issues relative `/api/*` requests; gateway-svc
  longest-prefix routes `/api` → `api-svc:8118`. This is why the client uses
  `BASE = '/api'` with no host — same origin from the browser's perspective.
- **Auth storage keys.** `vns_token` (JWT), `vns_user` (`{username, role}`),
  `vns_lang` (VI/EN), plus UI prefs `vns_theme` and `vns_sidebar`.
- **Graceful degradation.** If startup data calls fail, `DataContext` flips status to
  `demo` instead of crashing; algorithm metadata falls back to a static map.
- **Client RBAC is cosmetic.** Real authorization is enforced by api-svc; the
  frontend only hides/shows controls based on decoded JWT claims.
