# cli-svc — Interactive SSH CLI

`cli-svc` is an **interactive SSH server** for the stock-prediction platform. You
SSH in with your **dashboard account**, get a shell, and run four verbs
(`get` / `set` / `update` / `delete`) against the platform API. Results render as
**tables**. Each command you may run is governed by **per-command RBAC**
(Command RBAC) fetched from the API after login.

- SSH server: [`charmbracelet/wish`](https://github.com/charmbracelet/wish) — multi-session, one shell per connection.
- Shell UI: [`bubbletea`](https://github.com/charmbracelet/bubbletea) / `bubbles` / `lipgloss` — each SSH session is its own bubbletea model (multi-session safe).
- Tables: [`go-pretty`](https://github.com/jedib0t/go-pretty).

## How it works

```
ssh user@host -p 2345
   │  password auth → POST {API_BASE_URL}/auth/login
   ▼
cli-svc shell (bubbletea)
   │  on init → GET {API_BASE_URL}/me/commands → allowed-set
   ▼
type "<verb> <category> <name> [arg value ...]"
   │  resolve → handler, enforce permission, call API, render table
```

- **Login** authenticates against the dashboard login endpoint. On success the JWT,
  role and user id are kept in the SSH session; on failure the SSH auth is rejected.
- **Permissions:** after login the shell fetches `GET /me/commands`. You may only run
  handlers in that allowed-set. `super_admin` bypasses the check and may run any
  handler in the catalog. An empty allowed-set (no command groups) denies everything
  for non-super-admin roles. Disallowed commands print a clear `permission denied`.

## SSH in

Use the same username/password as the web dashboard.

```bash
ssh chon@localhost -p 2345        # super_admin from the seeder, for example
# password: <your dashboard password>
```

Inside the shell:

| Input | Effect |
|-------|--------|
| `help` or `?` | Show the command reference (only commands you may run) |
| `help <verb> <category> <name>` | Detailed help for one command (syntax, args, example), e.g. `help get market latest` |
| `Tab` | Open the suggestion dropdown (hidden by default); press again to cycle |
| `↓` / `↑` (dropdown open) | Navigate the suggestions; `Shift+Tab` goes back |
| `Enter` (dropdown open) | **Pick** the highlighted suggestion and advance to the next token |
| `↑` / `↓` (dropdown closed) | Recall previous / next command from history |
| `Enter` (dropdown closed) | **Run** the command |
| `Esc` | Close the dropdown |
| `clear` | Clear the screen |
| `exit` / `quit` / `Ctrl-C` / `Ctrl-D` | Disconnect |

### Filtering output (grep)

Pipe any command's output through a built-in `grep` with `-A`/`-B`/`-C`/`-i`:

```
get session list | grep -i gold
get market prices market crypto | grep -A 2 BTC
get schedules list | grep -B 1 -A 1 crawler
get session list | grep "GOLD|CRYPTO"        # regex alternation
get session list | grep -i "gold gru"         # quoted phrase with a space
```

`-A n` keeps n lines after a match, `-B n` before, `-C n` both, `-i` ignores case.
The pattern is a **regular expression** (so `A|B` matches A or B); **quote** it to
include spaces or a leading `-`. A `|` inside the quotes is part of the pattern,
not the pipe.

## Command reference

Everything is **space-separated**: `<verb> <category> <name> [arg value ...]`.
Verbs map to HTTP methods: `get`→GET, `set`→POST, `update`→PUT, `delete`→DELETE.
`market` ∈ `{gold, nasdaq, crypto, sp500}`. Quote values that contain spaces
(e.g. cron expressions): `"0 0 2 * * *"`.

### get (GET)

| Category / name | Args | Example |
|-----------------|------|---------|
| `market latest` | `market` (required) | `get market latest market gold` |
| `market prices` | `market` (required), `limit` | `get market prices market nasdaq limit 20` |
| `market predictions` | `market` (required) | `get market predictions market crypto` |
| `direction accuracy` | `market` ∈ `{GOLD,NASDAQ,CRYPTO,SP500}` (required) | `get direction accuracy market GOLD` |
| `monitoring overview` | — | `get monitoring overview` |
| `schedules list` | — | `get schedules list` |
| `pipeline reports` | `pipeline`, `limit` | `get pipeline reports pipeline crawler_gold limit 10` |
| `training status` | — | `get training status` |
| `users list` | — | `get users list` |
| `backups list` | — | `get backups list` |
| `session list` | — | `get session list` (bot trading sessions / leaderboard) |

### set (POST)

| Category / name | Args | Example |
|-----------------|------|---------|
| `trigger train` | `algorithm` (optional) | `set trigger train algorithm lstm_nn` |
| `trigger crawler` | `market` (required) | `set trigger crawler market gold` |
| `trigger predict` | `market` (required) | `set trigger predict market sp500` |
| `trigger reconcile` | — | `set trigger reconcile` |
| `trigger backup` | — | `set trigger backup` |

### update (PUT)

| Category / name | Args | Example |
|-----------------|------|---------|
| `schedule update` | `key` (required), `cron_expression` (required), `enabled` ∈ `{true,false}` | `update schedule update key crawler_gold cron_expression "0 0 2 * * *" enabled true` |
| `user update` | `id` (required), `role` ∈ `{user,admin,super_admin}`, `full_name`, `email`, `phone` | `update user update id 5 role admin email a@b.com` |

### delete (DELETE)

| Category / name | Args | Example |
|-----------------|------|---------|
| `backup delete` | `filename` (required) | `delete backup delete filename backup-2026.sql.gz` |
| `user delete` | `id` (required) | `delete user delete id 7` |

## Tab completion

The dropdown is **hidden until you press `Tab`**, context-aware, and **filtered to
what you are allowed to run**:

1. Empty / typing the verb → allowed verbs (`get`, `set`, `update`, `delete`).
2. After a verb → allowed categories under that verb (`market`, `schedules`, …).
3. After a category → names under it (`latest`, `prices`, …).
4. After the name → remaining argument names, then each argument's value choices.

`Tab` opens the dropdown and cycles through candidates (filling the current token);
`↑`/`↓`/`Shift+Tab` navigate it; `Enter` **picks** the highlighted candidate and
moves to the next token. With the dropdown closed, `↑`/`↓` recall command history
and `Enter` runs the command.

## Configuration

| Env var | Default | Purpose |
|---------|---------|---------|
| `API_BASE_URL` | `http://gateway-svc/api` | Base URL of the platform API (via gateway) |
| `SSH_LISTEN_ADDR` | `:2345` | SSH listen address |
| `INTERNAL_SECRET` | _(empty)_ | Shared secret sent with the boot-time handler-catalog upsert; if empty the upsert is skipped |
| `SSH_HOST_KEY_PATH` | `/etc/cli-svc/keys/host_key` | SSH host key path (auto-generated Ed25519 if missing) |

On boot, cli-svc POSTs its handler catalog to
`{API_BASE_URL}/command-handlers/upsert` (`{secret, handlers:[...]}`) so the API/auth
service knows which handlers exist for admins to grant. This is best-effort and
non-fatal if the API is not yet ready.

## Build & run

### Local

```bash
cd cli-svc
go mod tidy
go build ./...
go vet ./...
go test ./...

# Run against a local API:
API_BASE_URL=http://localhost:8118/api \
SSH_LISTEN_ADDR=:2345 \
SSH_HOST_KEY_PATH=./keys/host_key \
INTERNAL_SECRET=dev-secret \
go run .

# Then connect:
ssh chon@localhost -p 2345
```

### Docker

The image is built via `deploy/cli-svc.Dockerfile` (multi-stage; build context
is `cli-svc/`). docker-compose wires it as a service exposing `2345:2345`.

```bash
docker build -f deploy/cli-svc.Dockerfile -t cli-svc ./cli-svc
docker run --rm -p 2345:2345 \
  -e API_BASE_URL=http://gateway-svc/api \
  -e INTERNAL_SECRET=dev-secret \
  -v "$PWD/cli-svc/keys:/etc/cli-svc/keys" \
  cli-svc
```

## Structure

```
cli-svc/
├── main.go                 # start the wish SSH server
├── internal/
│   ├── server/             # wish setup, password auth → /auth/login, host key, catalog upsert
│   ├── shell/              # per-session bubbletea model + parse/enforce/run/completer
│   ├── handlers/           # handler catalog: registry + each handler {execute, render}
│   ├── client/             # HTTP client to API_BASE_URL (Bearer JWT) + JWT claim decode
│   └── render/             # go-pretty table helpers
└── README.md
```

## Tests

Table-driven tests cover:

- **Handler registry** — verb+resource resolution, arg validation against `arg_schema`
  (required/optional, int type, choices), request building, upsert payload.
- **Enforcement** — `super_admin` bypass, union allowed-set membership, empty allowed-set
  denies all.
- **Render** — JSON response → table containing expected headers/cells.
- **Parser & completer** — line parsing and permission-filtered completion.

The HTTP client is an interface, so tests mock it — no real network required.
