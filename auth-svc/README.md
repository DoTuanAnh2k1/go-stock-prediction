# auth-svc — Java Auth/RBAC Service

Java Spring Boot 3 gRPC service that is the **source of truth for authentication, users, and market groups** in the `go-stock-prediction` platform.

> **Naming note:** the directory was renamed to `auth-svc`, but the Maven `artifactId` is still `auth-service`. The built jar is `auth-service-*.jar`, and the Spring application name (`spring.application.name`) is `auth-service`. Only the directory was renamed — don't be confused by the `auth-service` strings inside the build/runtime.

---

## 1. Overview

- **Stack:** Java 21, Spring Boot 3.3.6, Spring Data JPA, `grpc-server-spring-boot-starter` (net.devh), `com.auth0:java-jwt`, Spring Security BCrypt, Flyway.
- **Protocol/port:** gRPC on `:8120` (configurable via `GRPC_PORT`). **Internal only** — not exposed outside the Docker network. No TLS (`grpc.server.security.enabled=false`) since traffic stays inside the cluster.
- **Role in the system:** owns all auth logic, the `users` table, and the market-group RBAC tables. The Go `api-svc` (`:8118`) is a thin proxy: it forwards auth / user / market-group requests here over gRPC, and **validates JWTs locally** using the shared `JWT_SECRET` (it does not call back to this service to verify each token).
- **JWT:** issued by this service, signed with **HMAC256** over the shared secret. 24-hour expiry (`jwt.expiration-ms = 86400000`).

JWT claims emitted by `JwtService.generateToken`:

| Claim | Value |
|-------|-------|
| `sub` | user id (as string) |
| `username` | username |
| `role` | `super_admin` / `admin` / `user` |
| `user_id` | numeric user id |
| `accessible_markets` | `string[]` of market keys (e.g. `["GOLD","NASDAQ"]`); `super_admin` always gets all four |
| `iat` / `exp` | issued-at / expires-at (24h) |

---

## 2. Responsibilities

- **Login** — verify username + BCrypt password, compute accessible markets, mint a signed JWT.
- **JWT generation** — HMAC256, 24h, with the claims above.
- **User CRUD** — list, create, update (profile + role), delete (soft delete), reset password, change own password.
- **Market groups** — CRUD groups, assign market keys to a group, add/remove users to/from groups, list a group's users, list a user's groups.
- **Password hashing** — BCrypt (`BCryptPasswordEncoder`, bean in `config/JwtConfig.java`). Hashes only; never plaintext.
- **Flyway migrations** — owns RBAC schema (V1) and user profile columns (V2).
- **Super-admin seeding** — seeds `chon/super_admin` on startup if missing.

---

## 3. RBAC model

Three roles, validated against `["super_admin","admin","user"]`. Permission rules are enforced in `UserService` and `MarketGroupService` based on the **caller's role** (passed in each request's `CallerMeta`).

**Accessible markets** (`UserService.getAccessibleMarkets`):
- `super_admin` → always all four: `GOLD`, `NASDAQ`, `CRYPTO`, `SP500`.
- `admin` / `user` → the union of market keys from the groups they belong to (empty list if none).

**Permission rules enforced in code:**

| Action | Rule |
|--------|------|
| Create / update / delete user, reset password, group ops | caller must be `admin` or `super_admin` (`requireAdminOrSuperAdmin`) |
| Create user with role `super_admin` | only a `super_admin` caller may do so |
| Assign / change role to `super_admin` (`updateUserRole`, `updateUser`) | only a `super_admin` caller |
| Modify a `super_admin` target (role/profile) | nobody except a `super_admin` caller |
| Delete a `super_admin` target | nobody except a `super_admin` caller (soft delete) |
| Reset a `super_admin` target's password | **nobody** — always denied |
| Reset password when caller is `admin` | only allowed against `user` targets (not `admin`/`super_admin`) |
| Add user to a market group | the user being added must not be `super_admin` |
| Change own password (`changePassword`) | must supply correct current password; new password ≥ **12** chars |
| Reset password (admin/super_admin resetting another) | new password ≥ **6** chars |

Net effect: `super_admin` is immutable and undeletable except by another `super_admin` (in practice, itself), and is the only role that can mint/grant `super_admin`. User deletion is **soft delete** — sets `deleted_at`; every lookup filters `WHERE deleted_at IS NULL` (GORM soft-delete compatibility shared with the Go side).

---

## 4. Database schema owned

This service shares the TimescaleDB/PostgreSQL 16 database (`db`, container `timescaledb`) with the rest of the platform. Flyway owns the auth/RBAC schema; JPA runs in `ddl-auto: validate` (does not create tables).

**Tables:**

| Table | Owner / notes |
|-------|---------------|
| `users` | the JPA `User` entity binds to it; columns `id, username, password_hash, role, full_name, email, phone, created_at, updated_at, deleted_at`. (The base table is created by the Go side / `database.sql`; this service adds the profile columns via V2.) |
| `market_groups` | created by V1 — `id BIGSERIAL PK, name UNIQUE, description, created_at, updated_at` |
| `market_group_markets` | created by V1 — `(group_id, market_key)` PK; `group_id` FK → `market_groups` `ON DELETE CASCADE`. Modeled as a JPA `@ElementCollection` of `marketKeys` on `MarketGroup` |
| `user_market_groups` | created by V1 — `(user_id, group_id)` PK; `group_id` FK → `market_groups` `ON DELETE CASCADE`; index on `user_id`. No FK on `user_id` (the `users` table may be migrated by GORM after Flyway runs) |

**Flyway migrations** (`src/main/resources/db/migration/`):

- `V1__create_auth_tables.sql` — creates `market_groups`, `market_group_markets`, `user_market_groups` (+ `idx_umg_user_id`).
- `V2__add_user_profile_fields.sql` — `ALTER TABLE users ADD COLUMN IF NOT EXISTS full_name VARCHAR(100) / email VARCHAR(255) / phone VARCHAR(30)`.

Flyway config: `enabled: true`, `baseline-on-migrate: true`. The build includes **`flyway-database-postgresql`** (Flyway 10.x split PG support into its own module) so Flyway recognizes PostgreSQL 16.

---

## 5. SuperAdminSeeder

`seeder/SuperAdminSeeder` implements `ApplicationRunner` and runs on startup. If no non-deleted user with username `chon` and role `super_admin` exists, it creates one:

- username: `chon`
- password: `Ch1nch2n@` (stored as a BCrypt hash)
- role: `super_admin`

Idempotent — skips creation if such a user already exists.

---

## 6. gRPC contract

Defined in `src/main/proto/auth.proto` (`service AuthService`), implemented by `grpc/AuthGrpcServiceImpl`. Most mutating RPCs carry a `CallerMeta { caller_id, caller_role }` so the service can enforce RBAC on the caller.

**Auth**

| RPC | Request → Response | Notes |
|-----|--------------------|-------|
| `Login` | `LoginRequest` → `LoginResponse` | authenticate, return JWT + username + role |
| `GetMe` | `CallerMeta` → `UserResponse` | resolves the caller by id |
| `ChangePassword` | `ChangePassRequest` → `Empty` | requires correct old password; new ≥ 12 chars |

**User management**

| RPC | Request → Response | Notes |
|-----|--------------------|-------|
| `ListUsers` | `CallerMeta` → `ListUsersResponse` | all non-deleted users |
| `CreateUser` | `CreateUserRequest` → `UserResponse` | role defaults to `user` if invalid; `409 ALREADY_EXISTS` on dup username |
| `DeleteUser` | `DeleteUserRequest` → `Empty` | soft delete |
| `UpdateUserRole` | `UpdateRoleRequest` → `UserResponse` | role-only update |
| `UpdateUser` | `UpdateUserRequest` → `UserResponse` | profile (full_name/email/phone) + optional role |
| `ResetPassword` | `ResetPasswordRequest` → `Empty` | new ≥ 6 chars |

**Market groups**

| RPC | Request → Response |
|-----|--------------------|
| `ListMarketGroups` | `CallerMeta` → `ListGroupsResponse` |
| `CreateMarketGroup` | `CreateGroupRequest` → `MarketGroupResponse` |
| `UpdateMarketGroup` | `UpdateGroupRequest` → `MarketGroupResponse` |
| `DeleteMarketGroup` | `DeleteGroupRequest` → `Empty` |
| `SetGroupMarkets` | `SetGroupMarketsRequest` → `Empty` (validates keys against `GOLD/NASDAQ/CRYPTO/SP500`) |
| `ListGroupUsers` | `GroupRequest` → `ListUsersResponse` |
| `AddUserToGroup` | `UserGroupRequest` → `Empty` |
| `RemoveUserFromGroup` | `UserGroupRequest` → `Empty` |
| `GetUserMarketGroups` | `UserRequest` → `ListGroupsResponse` |

Errors are returned as gRPC `StatusRuntimeException` with codes such as `UNAUTHENTICATED`, `PERMISSION_DENIED`, `NOT_FOUND`, `ALREADY_EXISTS`, `INVALID_ARGUMENT`.

---

## 7. Directory structure

```
auth-svc/
├── pom.xml                                  # Maven build (artifactId still "auth-service")
├── src/
│   ├── main/
│   │   ├── proto/auth.proto                 # gRPC service + message definitions (18 RPCs)
│   │   ├── resources/
│   │   │   ├── application.yml              # datasource, JPA, Flyway, gRPC port, JWT settings
│   │   │   └── db/migration/
│   │   │       ├── V1__create_auth_tables.sql       # market_groups + RBAC join tables
│   │   │       └── V2__add_user_profile_fields.sql  # full_name/email/phone on users
│   │   └── java/vn/gostock/auth/
│   │       ├── AuthServiceApplication.java  # Spring Boot entry point
│   │       ├── config/JwtConfig.java        # BCryptPasswordEncoder bean
│   │       ├── entity/                      # User, MarketGroup, UserMarketGroup(+Id)
│   │       ├── repository/                  # Spring Data JPA repositories
│   │       ├── service/
│   │       │   ├── UserService.java         # auth + user CRUD + RBAC rules
│   │       │   ├── MarketGroupService.java  # group CRUD + membership + RBAC rules
│   │       │   └── JwtService.java          # HMAC256 token generation
│   │       ├── seeder/SuperAdminSeeder.java # seeds chon/super_admin on startup
│   │       └── grpc/AuthGrpcServiceImpl.java# @GrpcService — implements all RPCs
│   └── test/java/vn/gostock/auth/grpc/AuthGrpcServiceImplTest.java
└── (Dockerfile lives at ../deploy/auth-svc.Dockerfile)
```

---

## 8. Build & run

### Environment variables

| Variable | Default | Used for |
|----------|---------|----------|
| `DB_HOST` | `localhost` | Postgres host (`db` in compose) |
| `DB_PORT` | `5432` | Postgres port |
| `DB_NAME` | `go_stock_prediction` | database name |
| `DB_USER` | `postgres` | datasource user |
| `DB_PASSWORD` | `123` | datasource password |
| `JWT_SECRET` | `change-me-in-production` | **must match `api-svc`** — shared HMAC256 signing key |
| `GRPC_PORT` | `8120` | gRPC listen port |

### Local

```bash
cd auth-svc
mvn package            # compiles, runs protoc, builds target/auth-service-1.0.0.jar
java -jar target/auth-service-*.jar
```

The protobuf Maven plugin generates the gRPC Java stubs from `auth.proto` during the build (no manual codegen). On first run, Flyway applies V1/V2 and the seeder creates `chon/super_admin`.

### Docker (via compose)

The Dockerfile is in `deploy/` (flat layout). Build context is `../auth-svc` with `dockerfile ../deploy/auth-svc.Dockerfile` (multi-stage: `maven:3.9-eclipse-temurin-21` builder → `eclipse-temurin:21-jre` runtime, `mvn package -DskipTests`). The compose service key, `container_name`, and directory name are all `auth-svc`; internal DNS is `auth-svc:8120`.

From the repo root:

```bash
docker compose --env-file .env -f deploy/docker-compose.yaml up -d auth-svc
```

`auth-svc` depends on `db` (healthy). `api-svc` reaches it at `auth-svc:8120` (`AUTH_GRPC_TARGET`).

---

## 9. Tests

```bash
cd auth-svc
mvn verify            # runs unit/integration tests (surefire), incl. AuthGrpcServiceImplTest
```

`mvn package` skips tests via the Dockerfile (`-DskipTests`); run `mvn verify` (or `mvn test`) locally to execute them. Test sources live under `src/test/java/vn/gostock/auth/`.
