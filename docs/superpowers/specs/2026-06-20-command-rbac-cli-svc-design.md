# Command RBAC + cli-svc — Design Spec

Date: 2026-06-20 · Branch: `feat/command-rbac-cli-svc`

## 1. Mục tiêu

Thêm service mới **`cli-svc`**: một SSH server tương tác (shell) cho server headless,
tiêu thụ API của hệ thống và hiển thị kết quả dạng **table**. Đi kèm là một hệ thống
**phân quyền theo command** (Command RBAC): admin khai báo command, gom vào group,
gán user vào group → user chỉ execute được command trong group của mình.

## 2. Quyết định cốt lõi (đã chốt với user)

| Vấn đề | Quyết định |
|---|---|
| Ngôn ngữ/framework cli-svc | **Go** + `charmbracelet/wish` (SSH server, đa phiên) + `bubbletea`/`bubbles`/`lipgloss` (shell UI) + `go-pretty` (table) |
| Transport | SSH expose trực tiếp **port 2345**; **không** qua gateway (gateway chỉ HTTP) |
| API calls | Đi **qua gateway**: `API_BASE_URL=http://gateway-svc/api` |
| SSH auth | = tài khoản dashboard: password-callback → `POST /api/auth/login`; OK thì giữ JWT trong phiên, fail thì từ chối SSH |
| Command granularity | **Full command line** (verb+resource+args cụ thể) |
| Khai báo command | **Động**: admin CRUD, lưu DB |
| Lưu ở đâu | **auth-svc** (Flyway + gRPC), mirror market_groups; api-svc proxy |
| Enforce quyền | **cli-svc** (catalog = lớp quyền của CLI); api-svc giữ role/market guard sẵn có làm biên thật |
| Quản lý | **Cả web (web-svc) lẫn cli** |
| Command schema | **Handler có sẵn trong cli + args cố định** (không free method/path) |
| 4 verb | `get`→GET, `set`→POST, `update`→PUT, `delete`→DELETE |

## 3. Kiến trúc

```
 ssh user@host -p 2345 (TCP, expose trực tiếp)
        │
        ▼
 ┌────────────┐  HTTP /api   ┌─────────────┐  /api  ┌──────────┐  gRPC  ┌──────────┐
 │  cli-svc   │─────────────▶│ gateway-svc │───────▶│ api-svc  │───────▶│ auth-svc │
 │ wish+TUI   │ login+CRUD   │  :80/:443   │        │  :8118   │        │  :8120   │
 └────────────┘              └─────────────┘        └──────────┘        └──────────┘
   handler catalog (code)                            proxy endpoints    Flyway V3 RBAC
   enforce command perms                             /api/commands...   commands/groups
```

## 4. Data model — auth-svc Flyway `V3__create_command_rbac.sql`

```sql
CREATE TABLE IF NOT EXISTS cli_handlers (
    handler_key  VARCHAR(80)  PRIMARY KEY,
    display_name VARCHAR(120) NOT NULL,
    verb         VARCHAR(10)  NOT NULL,
    resource     VARCHAR(40)  NOT NULL,
    arg_schema   JSONB        NOT NULL DEFAULT '[]',
    enabled      BOOLEAN      NOT NULL DEFAULT true
);
CREATE TABLE IF NOT EXISTS commands (
    id           BIGSERIAL    PRIMARY KEY,
    name         VARCHAR(120) NOT NULL UNIQUE,
    description  TEXT,
    handler_key  VARCHAR(80)  NOT NULL REFERENCES cli_handlers(handler_key),
    args         JSONB        NOT NULL DEFAULT '{}',
    enabled      BOOLEAN      NOT NULL DEFAULT true,
    created_at   TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMP    NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS command_groups (
    id          BIGSERIAL    PRIMARY KEY,
    name        VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    created_at  TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP    NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS command_group_commands (
    group_id   BIGINT NOT NULL REFERENCES command_groups(id) ON DELETE CASCADE,
    command_id BIGINT NOT NULL REFERENCES commands(id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, command_id)
);
CREATE TABLE IF NOT EXISTS user_command_groups (
    user_id  BIGINT NOT NULL,
    group_id BIGINT NOT NULL REFERENCES command_groups(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, group_id)
);
CREATE INDEX IF NOT EXISTS idx_ucg_user_id ON user_command_groups(user_id);
```

**Semantics:** allowed-commands(user) = UNION qua mọi command_group user thuộc, lọc `enabled=true`
(command + handler). `super_admin` → tất cả command. `admin` → quản lý + chạy tất cả.
`user` → chỉ command trong group. User không group → catalog rỗng.

## 5. Handler catalog (nguồn sự thật = cli-svc code)

cli-svc khởi động → upsert catalog vào auth-svc qua gRPC `UpsertHandlers` (giống `DEFAULT_SCHEDULES`).
Web/cli đọc danh sách handler khi admin tạo command. Mỗi handler biết: gọi API nào + render bảng ra sao.

Initial catalog (mở rộng sau):

| handler_key | verb | API call | args |
|---|---|---|---|
| `market.latest` | get | GET /api/{market}/latest | market |
| `market.prices` | get | GET /api/{market}/prices | market, limit |
| `market.predictions` | get | GET /api/{market}/predictions/latest | market |
| `direction.accuracy` | get | GET /api/predictions/direction-accuracy | market |
| `monitoring.overview` | get | GET /api/monitoring/overview | — |
| `schedules.list` | get | GET /api/schedules | — |
| `pipeline.reports` | get | GET /api/pipeline-reports | pipeline, limit |
| `training.status` | get | GET /api/training/status | — |
| `users.list` | get | GET /api/users | — |
| `backups.list` | get | GET /api/backups | — |
| `trigger.train` | set | POST /api/trigger/train | algorithm? |
| `trigger.crawler` | set | POST /api/trigger/{market}-crawler | market |
| `trigger.predict` | set | POST /api/trigger/{market}-predict | market |
| `trigger.reconcile` | set | POST /api/trigger/reconcile | — |
| `trigger.backup` | set | POST /api/trigger/backup | — |
| `schedule.update` | update | PUT /api/schedules/{key} | key, cron_expression, enabled |
| `user.update` | update | PUT /api/users/{id} | id, role?, full_name?, email?, phone? |
| `backup.delete` | delete | DELETE /api/backups/{filename} | filename |
| `user.delete` | delete | DELETE /api/users/{id} | id |

market ∈ {gold, nasdaq, crypto, sp500}.

## 6. gRPC contract — thêm vào auth.proto (CẢ 2 bản: api-svc/proto/auth + auth-svc/src/main/proto)

Messages: `Command`, `CommandGroup`, `CliHandler`, plus request/response cho CRUD.
RPCs thêm vào `service AuthService`:

```
// Handler catalog
rpc UpsertHandlers(UpsertHandlersRequest) returns (Empty);
rpc ListHandlers(CallerMeta)              returns (ListHandlersResponse);
// Commands
rpc ListCommands(CallerMeta)              returns (ListCommandsResponse);
rpc CreateCommand(CreateCommandRequest)   returns (CommandResponse);
rpc UpdateCommand(UpdateCommandRequest)   returns (CommandResponse);
rpc DeleteCommand(DeleteCommandRequest)   returns (Empty);
// Command groups (mirror market group RPCs)
rpc ListCommandGroups(CallerMeta)             returns (ListCommandGroupsResponse);
rpc CreateCommandGroup(CreateCmdGroupRequest) returns (CommandGroupResponse);
rpc UpdateCommandGroup(UpdateCmdGroupRequest) returns (CommandGroupResponse);
rpc DeleteCommandGroup(DeleteCmdGroupRequest) returns (Empty);
rpc SetGroupCommands(SetGroupCommandsRequest) returns (Empty);
rpc ListCmdGroupUsers(CmdGroupRequest)        returns (ListUsersResponse);
rpc AddUserToCmdGroup(UserCmdGroupRequest)    returns (Empty);
rpc RemoveUserFromCmdGroup(UserCmdGroupRequest) returns (Empty);
// Enforcement: command user được phép (cli gọi sau login)
rpc GetUserCommands(UserRequest)              returns (ListCommandsResponse);
```

## 7. api-svc — proxy endpoints (mirror api_market_groups.go)

- `GET/POST /api/commands`, `PUT/DELETE /api/commands/{id}` (AdminRequired cho ghi)
- `GET /api/command-handlers` (list handler catalog, AdminRequired)
- `GET/POST /api/command-groups`, `PUT/DELETE /api/command-groups/{id}`
- `PUT /api/command-groups/{id}/commands` (set commands), `GET/POST/DELETE .../users`
- `GET /api/me/commands` — command user hiện tại được phép (AuthRequired) → cli gọi sau login
- (internal) cli-svc gọi `UpsertHandlers` qua api-svc hoặc trực tiếp auth-svc? → **trực tiếp auth-svc gRPC** từ cli-svc KHÔNG; cli-svc chỉ nói HTTP. Vậy thêm `POST /api/command-handlers/upsert` (internal, no-JWT trên app-network) để cli-svc đẩy catalog. Bảo vệ: chỉ chấp nhận từ network nội bộ / shared secret header.

Proto regen Go: `cd api-svc && protoc ... proto/auth/auth.proto`. Swagger: `make swagger`.

## 8. cli-svc — cấu trúc

```
cli-svc/
├── README.md            # chức năng + command reference (get/set/update/delete) + cách SSH
├── go.mod
├── main.go              # wish.Server :2345 + middleware
├── internal/
│   ├── server/          # wish setup, password-auth → /api/auth/login, host key, upsert handlers on boot
│   ├── shell/           # model.go/update.go/view.go/completer.go (bubbletea, 1 Model/phiên)
│   ├── handlers/        # handler catalog: registry + mỗi handler {call api, render table}
│   ├── client/          # HTTP client → gateway (/api), gắn JWT từ ctx
│   └── render/          # go-pretty/lipgloss table helpers
└── config: env API_BASE_URL, SSH_LISTEN_ADDR, INTERNAL_SECRET
```

Flow phiên: SSH connect → password-auth gọi login → lưu {jwt, role, user_id} vào ssh.Context →
bubbletea shell khởi tạo → gọi `GET /api/me/commands` → build allowed-set + completer →
user gõ → resolve về (handler_key+args) → check ∈ allowed (super_admin bypass) →
chạy handler → render table → in vào viewport.

## 9. web-svc — UI

- Trang `/admin/commands` — CRUD command (chọn handler từ catalog, điền args theo arg_schema), giống `MarketGroups.tsx`.
- Trang `/admin/command-groups` — CRUD group, set commands, gán/bỏ user.
- Chỉ hiện với role admin/super_admin. Thêm nav + i18n VI/EN.

## 10. Deploy

Thêm service `cli-svc` vào `deploy/docker-compose.yaml` (context `../cli-svc`,
dockerfile `../deploy/cli-svc.Dockerfile`, depends_on gateway-svc, ports `2345:2345`,
env API_BASE_URL/SSH_LISTEN_ADDR/INTERNAL_SECRET, volume `../cli-svc/keys`).
Tạo `deploy/cli-svc.Dockerfile` (multi-stage Go build).

## 11. Testing

- **auth-svc (Java/JUnit):** repository + gRPC service cho command/group CRUD, union-permission query, super_admin bypass.
- **api-svc (Go):** handler tests cho api_commands.go / api_command_groups.go (mock pattern hiện có).
- **cli-svc (Go):** unit test handler registry (resolve, arg validation), completer, render table; enforce allowed-set logic (super_admin bypass, union, empty). Mock HTTP client.
- **web-svc:** (nếu có test harness) component smoke; nếu không, bỏ qua.

## 12. Docs cần cập nhật khi xong

CLAUDE.md (service thứ 6, Ports `2345/SSH`, Docker Compose Services, cấu trúc thư mục,
API endpoints commands/command-groups, gRPC contract, Flyway V3), README.md root (sơ đồ),
Makefile (`cli-build`), cli-svc/README.md riêng.

## 13. Out of scope (YAGNI)

- Không free method/path mapping. Không per-arg-value catalog tự sinh. Không SSH-qua-gateway.
- Không audit log riêng cho command execution (giai đoạn sau nếu cần).
