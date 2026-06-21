# Service Management (Registry/Discovery) — Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a standalone Go gRPC service registry (`service-mgt`) with register / heartbeat (lease-TTL) / discover, plus a Go client SDK, and wire `api-svc` to use it for discovering `auth-svc` and `prediction-svc` — all behind a global enable flag with static fallback so current behavior is unchanged when disabled.

**Architecture:** `service-mgt` is a new Go gRPC server on `:8121` (internal). Postgres (`service_instances`) is the source of truth via **write-through** cache; an in-memory cache is the read layer. Heartbeats touch the cache only (lease renewal); a flusher persists `last_seen` periodically; a reaper marks expired leases `DOWN`. Clients register on boot, heartbeat in the background, and resolve peers via `Discover` with a short client-side cache, falling back to static env targets when the registry is disabled or unreachable.

**Tech Stack:** Go 1.25, `google.golang.org/grpc`, GORM v2 + Postgres/TimescaleDB, zerolog (repo logger style), protoc (Go stubs), Docker Compose.

## Global Constraints

- Registry service is its own Go module: `module go-stock-prediction/service-mgt` (mirrors `cli-svc`). Go directive `go 1.25`.
- Logging: zerolog, one line, English messages, no emoji, ANSI forced — consistent with repo.
- Timezone ICT-at-rest: all timestamps use `time.Now()` with `time.Local = Asia/Ho_Chi_Minh` set in `main`. DB columns are `TIMESTAMP WITHOUT TIME ZONE`.
- Generated protobuf files (`*.pb.go`) are never hand-edited.
- Global feature flag `SERVICE_MGT_ENABLED` (bool, **default `false`**). When false, every integrated service behaves exactly as today (static env targets). `REGISTRY_GRPC_TARGET` default `localhost:8121` (Docker: `service-mgt:8121`).
- Decimal/price rules N/A here. No new external infra beyond the existing Postgres.
- Schema lives in `database.sql` (mounted as `/docker-entrypoint-initdb.d/01-schema.sql`); AutoMigrate stays disabled.
- Default lease values: `ttl_seconds=30`, client heartbeat interval `10s`, reaper tick `1s`, DOWN→evict grace `60s`, flusher interval `30s`, client discover cache TTL `5s`.

---

### Task 1: Scaffold `service-mgt` module + proto + Go stubs

**Files:**
- Create: `service-mgt/go.mod`, `service-mgt/proto/registry/registry.proto`
- Create (generated): `service-mgt/proto/registry/registry.pb.go`, `service-mgt/proto/registry/registry_grpc.pb.go`

**Interfaces:**
- Produces: proto package `registry`, Go import `go-stock-prediction/service-mgt/proto/registry` exposing `RegistryClient`, `RegistryServer`, messages `RegisterRequest{ServiceName, InstanceId, Address string; Port int32; Metadata map[string]string; TtlSeconds int32}`, `RegisterResponse{InstanceId string; LeaseTtlSeconds int32}`, `HeartbeatRequest{InstanceId string}`, `HeartbeatResponse{Ok bool}`, `DeregisterRequest{InstanceId string}`, `DiscoverRequest{ServiceName string}`, `Instance{ServiceName, InstanceId, Address string; Port int32; Metadata map[string]string; Status string}`, `DiscoverResponse{Instances []*Instance}`, `Empty{}`, `ListServicesResponse{Instances []*Instance}`.

- [ ] **Step 1: Create the module file**

`service-mgt/go.mod`:
```
module go-stock-prediction/service-mgt

go 1.25
```

- [ ] **Step 2: Write the proto**

`service-mgt/proto/registry/registry.proto`:
```proto
syntax = "proto3";
package registry;

option go_package = "go-stock-prediction/service-mgt/proto/registry";
option java_package = "vn.gostock.registry.proto";
option java_multiple_files = true;

service Registry {
  rpc Register(RegisterRequest)     returns (RegisterResponse);
  rpc Heartbeat(HeartbeatRequest)   returns (HeartbeatResponse);
  rpc Deregister(DeregisterRequest) returns (Empty);
  rpc Discover(DiscoverRequest)     returns (DiscoverResponse);
  rpc ListServices(Empty)           returns (ListServicesResponse);
}

message Empty {}

message RegisterRequest {
  string service_name = 1;
  string instance_id  = 2;   // optional; server generates if empty
  string address      = 3;
  int32  port         = 4;
  map<string, string> metadata = 5;
  int32  ttl_seconds  = 6;    // optional; server default if 0
}
message RegisterResponse {
  string instance_id       = 1;
  int32  lease_ttl_seconds = 2;
}
message HeartbeatRequest  { string instance_id = 1; }
message HeartbeatResponse { bool ok = 1; }
message DeregisterRequest { string instance_id = 1; }
message DiscoverRequest   { string service_name = 1; }

message Instance {
  string service_name = 1;
  string instance_id  = 2;
  string address      = 3;
  int32  port         = 4;
  map<string, string> metadata = 5;
  string status       = 6;   // "UP" | "DOWN"
}
message DiscoverResponse     { repeated Instance instances = 1; }
message ListServicesResponse { repeated Instance instances = 1; }
```

- [ ] **Step 3: Generate Go stubs**

Run (from repo root; requires protoc + protoc-gen-go + protoc-gen-go-grpc on PATH):
```bash
cd service-mgt && protoc -Iproto \
  --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  proto/registry/registry.proto
```
Expected: creates `proto/registry/registry.pb.go` and `registry_grpc.pb.go`, exit 0.

- [ ] **Step 4: Verify the module compiles**

Run:
```bash
cd service-mgt && go mod tidy && go build ./...
```
Expected: PASS (downloads grpc/protobuf deps, builds the generated stubs).

- [ ] **Step 5: Commit**

```bash
git add service-mgt/go.mod service-mgt/go.sum service-mgt/proto
git commit -m "feat(service-mgt): scaffold module and registry proto"
```

---

### Task 2: DB schema + GORM model + write-through store

**Files:**
- Modify: `database.sql` (append `service_instances` table, before "END OF SCHEMA")
- Create: `service-mgt/internal/store/model.go`, `service-mgt/internal/store/store.go`
- Test: `service-mgt/internal/store/store_test.go`

**Interfaces:**
- Produces: `store.Instance` GORM struct; `store.Store` interface:
  ```go
  type Store interface {
      Upsert(inst *Instance) error                 // write-through: insert or update by InstanceID
      UpdateStatus(instanceID, status string) error
      Delete(instanceID string) error
      FlushLastSeen(seen map[string]time.Time) error // batch update last_seen
      LoadAll() ([]*Instance, error)                 // warm-start
  }
  ```
  and `store.NewGorm(db *gorm.DB) Store`.

- [ ] **Step 1: Add the table to `database.sql`**

Insert before the `END OF SCHEMA` banner:
```sql
CREATE TABLE IF NOT EXISTS service_instances (
    id            BIGSERIAL     PRIMARY KEY,
    service_name  VARCHAR(64)   NOT NULL,
    instance_id   VARCHAR(128)  NOT NULL UNIQUE,
    address       VARCHAR(255)  NOT NULL,
    port          INTEGER       NOT NULL,
    metadata      JSONB,
    status        VARCHAR(16)   NOT NULL DEFAULT 'UP',
    ttl_seconds   INTEGER       NOT NULL DEFAULT 30,
    last_seen     TIMESTAMP     NOT NULL DEFAULT NOW(),
    registered_at TIMESTAMP     NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP     NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_service_instances_name   ON service_instances(service_name, status);
CREATE INDEX IF NOT EXISTS idx_service_instances_status ON service_instances(status);
```

- [ ] **Step 2: Write the failing store test**

`service-mgt/internal/store/store_test.go` (uses sqlite in-memory via gorm for unit isolation):
```go
package store

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestStore(t *testing.T) Store {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil { t.Fatalf("open: %v", err) }
	if err := db.AutoMigrate(&Instance{}); err != nil { t.Fatalf("migrate: %v", err) }
	return NewGorm(db)
}

func TestUpsertThenLoadAll(t *testing.T) {
	s := newTestStore(t)
	in := &Instance{ServiceName: "api-svc", InstanceID: "api-1", Address: "api-svc", Port: 8118, Status: "UP", TTLSeconds: 30}
	if err := s.Upsert(in); err != nil { t.Fatalf("upsert: %v", err) }
	// second upsert with same instance_id must update, not duplicate
	in.Port = 9999
	if err := s.Upsert(in); err != nil { t.Fatalf("re-upsert: %v", err) }
	all, err := s.LoadAll()
	if err != nil { t.Fatalf("loadall: %v", err) }
	if len(all) != 1 { t.Fatalf("want 1 row, got %d", len(all)) }
	if all[0].Port != 9999 { t.Fatalf("want updated port 9999, got %d", all[0].Port) }
}

func TestUpdateStatusAndDelete(t *testing.T) {
	s := newTestStore(t)
	_ = s.Upsert(&Instance{ServiceName: "x", InstanceID: "x-1", Address: "x", Port: 1, Status: "UP", TTLSeconds: 30})
	if err := s.UpdateStatus("x-1", "DOWN"); err != nil { t.Fatalf("status: %v", err) }
	all, _ := s.LoadAll()
	if all[0].Status != "DOWN" { t.Fatalf("want DOWN, got %s", all[0].Status) }
	if err := s.Delete("x-1"); err != nil { t.Fatalf("delete: %v", err) }
	all, _ = s.LoadAll()
	if len(all) != 0 { t.Fatalf("want 0 after delete, got %d", len(all)) }
}

func TestFlushLastSeen(t *testing.T) {
	s := newTestStore(t)
	_ = s.Upsert(&Instance{ServiceName: "x", InstanceID: "x-1", Address: "x", Port: 1, Status: "UP", TTLSeconds: 30})
	ts := time.Now().Add(time.Hour).Truncate(time.Second)
	if err := s.FlushLastSeen(map[string]time.Time{"x-1": ts}); err != nil { t.Fatalf("flush: %v", err) }
	all, _ := s.LoadAll()
	if !all[0].LastSeen.Equal(ts) { t.Fatalf("want %v, got %v", ts, all[0].LastSeen) }
}
```

- [ ] **Step 3: Run the test to confirm it fails**

Run: `cd service-mgt && go test ./internal/store/ -run Test -v`
Expected: FAIL (compile error — `Instance` / `NewGorm` undefined).

- [ ] **Step 4: Implement the model**

`service-mgt/internal/store/model.go`:
```go
package store

import "time"

// Instance is the persisted registration row. Source of truth in Postgres.
type Instance struct {
	ID           int64             `gorm:"primaryKey;column:id"`
	ServiceName  string            `gorm:"column:service_name"`
	InstanceID   string            `gorm:"column:instance_id;uniqueIndex"`
	Address      string            `gorm:"column:address"`
	Port         int32             `gorm:"column:port"`
	Metadata     map[string]string `gorm:"column:metadata;serializer:json"`
	Status       string            `gorm:"column:status"`
	TTLSeconds   int32             `gorm:"column:ttl_seconds"`
	LastSeen     time.Time         `gorm:"column:last_seen"`
	RegisteredAt time.Time         `gorm:"column:registered_at"`
	UpdatedAt    time.Time         `gorm:"column:updated_at"`
}

func (Instance) TableName() string { return "service_instances" }
```

- [ ] **Step 5: Implement the store**

`service-mgt/internal/store/store.go`:
```go
package store

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store interface {
	Upsert(inst *Instance) error
	UpdateStatus(instanceID, status string) error
	Delete(instanceID string) error
	FlushLastSeen(seen map[string]time.Time) error
	LoadAll() ([]*Instance, error)
}

type gormStore struct{ db *gorm.DB }

func NewGorm(db *gorm.DB) Store { return &gormStore{db: db} }

func (g *gormStore) Upsert(inst *Instance) error {
	now := time.Now()
	if inst.RegisteredAt.IsZero() { inst.RegisteredAt = now }
	inst.UpdatedAt = now
	if inst.LastSeen.IsZero() { inst.LastSeen = now }
	if inst.Status == "" { inst.Status = "UP" }
	return g.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "instance_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"service_name", "address", "port", "metadata", "status", "ttl_seconds", "last_seen", "updated_at"}),
	}).Create(inst).Error
}

func (g *gormStore) UpdateStatus(instanceID, status string) error {
	return g.db.Model(&Instance{}).Where("instance_id = ?", instanceID).
		Updates(map[string]any{"status": status, "updated_at": time.Now()}).Error
}

func (g *gormStore) Delete(instanceID string) error {
	return g.db.Where("instance_id = ?", instanceID).Delete(&Instance{}).Error
}

func (g *gormStore) FlushLastSeen(seen map[string]time.Time) error {
	if len(seen) == 0 { return nil }
	return g.db.Transaction(func(tx *gorm.DB) error {
		for id, ts := range seen {
			if err := tx.Model(&Instance{}).Where("instance_id = ?", id).
				Update("last_seen", ts).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (g *gormStore) LoadAll() ([]*Instance, error) {
	var out []*Instance
	err := g.db.Find(&out).Error
	return out, err
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd service-mgt && go mod tidy && go test ./internal/store/ -v`
Expected: PASS (3 tests). `go mod tidy` adds `gorm.io/gorm`, `gorm.io/driver/postgres`, `github.com/glebarez/sqlite`.

- [ ] **Step 7: Commit**

```bash
git add database.sql service-mgt/internal/store service-mgt/go.mod service-mgt/go.sum
git commit -m "feat(service-mgt): service_instances schema + write-through store"
```

---

### Task 3: Registry core (cache, lease, reaper, flusher)

**Files:**
- Create: `service-mgt/internal/registry/registry.go`
- Test: `service-mgt/internal/registry/registry_test.go`

**Interfaces:**
- Consumes: `store.Store`, `store.Instance`.
- Produces: `registry.Core` with:
  ```go
  func New(s store.Store, cfg Config) *Core
  func (c *Core) Register(in *store.Instance) (string, int32, error) // returns instanceID, ttl; write-through
  func (c *Core) Heartbeat(instanceID string) bool                   // cache-only; false if unknown
  func (c *Core) Deregister(instanceID string) error                 // write-through delete
  func (c *Core) Discover(serviceName string) []*store.Instance      // healthy (UP, unexpired) from cache
  func (c *Core) List() []*store.Instance
  func (c *Core) Tick(now time.Time)                                  // reaper step (exported for tests)
  func (c *Core) WarmStart() error
  func (c *Core) StartBackground(stop <-chan struct{})                // reaper + flusher goroutines
  ```
  `type Config struct { DefaultTTL time.Duration; EvictGrace time.Duration; FlushInterval time.Duration; ReaperTick time.Duration }`.

- [ ] **Step 1: Write the failing core test**

`service-mgt/internal/registry/registry_test.go`:
```go
package registry

import (
	"testing"
	"time"

	"go-stock-prediction/service-mgt/internal/store"
)

type fakeStore struct{ rows map[string]*store.Instance }

func newFake() *fakeStore { return &fakeStore{rows: map[string]*store.Instance{}} }
func (f *fakeStore) Upsert(i *store.Instance) error { cp := *i; f.rows[i.InstanceID] = &cp; return nil }
func (f *fakeStore) UpdateStatus(id, s string) error { if r, ok := f.rows[id]; ok { r.Status = s }; return nil }
func (f *fakeStore) Delete(id string) error { delete(f.rows, id); return nil }
func (f *fakeStore) FlushLastSeen(m map[string]time.Time) error { return nil }
func (f *fakeStore) LoadAll() ([]*store.Instance, error) {
	out := []*store.Instance{}
	for _, v := range f.rows { out = append(out, v) }
	return out, nil
}

func testCfg() Config {
	return Config{DefaultTTL: 30 * time.Second, EvictGrace: 60 * time.Second, FlushInterval: time.Hour, ReaperTick: time.Second}
}

func TestRegisterDiscover(t *testing.T) {
	c := New(newFake(), testCfg())
	id, ttl, err := c.Register(&store.Instance{ServiceName: "auth-svc", Address: "auth-svc", Port: 8120})
	if err != nil { t.Fatalf("register: %v", err) }
	if id == "" || ttl != 30 { t.Fatalf("bad register result id=%q ttl=%d", id, ttl) }
	got := c.Discover("auth-svc")
	if len(got) != 1 || got[0].Port != 8120 { t.Fatalf("discover mismatch: %+v", got) }
}

func TestLeaseExpiryMarksDown(t *testing.T) {
	c := New(newFake(), testCfg())
	id, _, _ := c.Register(&store.Instance{ServiceName: "x", Address: "x", Port: 1})
	// no heartbeat; advance past TTL
	c.Tick(time.Now().Add(31 * time.Second))
	if len(c.Discover("x")) != 0 { t.Fatalf("expired instance must not be discoverable") }
	// heartbeat on expired-but-not-evicted lease revives it
	if ok := c.Heartbeat(id); !ok { t.Fatalf("heartbeat should still find the instance before grace evict") }
	if len(c.Discover("x")) != 1 { t.Fatalf("heartbeat should revive to UP") }
}

func TestHeartbeatUnknown(t *testing.T) {
	c := New(newFake(), testCfg())
	if c.Heartbeat("nope") { t.Fatalf("unknown heartbeat must return false") }
}

func TestEvictAfterGrace(t *testing.T) {
	c := New(newFake(), testCfg())
	c.Register(&store.Instance{ServiceName: "x", InstanceID: "x-1", Address: "x", Port: 1})
	base := time.Now()
	c.Tick(base.Add(31 * time.Second)) // -> DOWN
	c.Tick(base.Add(95 * time.Second)) // past TTL+grace -> evict
	if len(c.List()) != 0 { t.Fatalf("instance should be evicted after grace, got %d", len(c.List())) }
}
```

- [ ] **Step 2: Run the test to confirm it fails**

Run: `cd service-mgt && go test ./internal/registry/ -v`
Expected: FAIL (compile error — `New`, `Core`, `Config` undefined).

- [ ] **Step 3: Implement the core**

`service-mgt/internal/registry/registry.go`:
```go
package registry

import (
	"sync"
	"time"

	"github.com/google/uuid"

	"go-stock-prediction/service-mgt/internal/store"
)

type Config struct {
	DefaultTTL    time.Duration
	EvictGrace    time.Duration
	FlushInterval time.Duration
	ReaperTick    time.Duration
}

type entry struct {
	inst     *store.Instance
	expireAt time.Time
	dirtySeen time.Time // last heartbeat time not yet flushed
}

type Core struct {
	mu  sync.RWMutex
	cfg Config
	st  store.Store
	// serviceName -> instanceID -> entry
	byName map[string]map[string]*entry
	byID   map[string]*entry
}

func New(s store.Store, cfg Config) *Core {
	return &Core{cfg: cfg, st: s, byName: map[string]map[string]*entry{}, byID: map[string]*entry{}}
}

func (c *Core) Register(in *store.Instance) (string, int32, error) {
	if in.InstanceID == "" {
		in.InstanceID = in.ServiceName + "-" + uuid.NewString()
	}
	ttl := c.cfg.DefaultTTL
	if in.TTLSeconds > 0 {
		ttl = time.Duration(in.TTLSeconds) * time.Second
	} else {
		in.TTLSeconds = int32(ttl / time.Second)
	}
	in.Status = "UP"
	now := time.Now()
	in.LastSeen = now
	// write-through: DB first
	if err := c.st.Upsert(in); err != nil {
		return "", 0, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e := &entry{inst: in, expireAt: now.Add(ttl)}
	if c.byName[in.ServiceName] == nil {
		c.byName[in.ServiceName] = map[string]*entry{}
	}
	c.byName[in.ServiceName][in.InstanceID] = e
	c.byID[in.InstanceID] = e
	return in.InstanceID, int32(ttl / time.Second), nil
}

func (c *Core) Heartbeat(instanceID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.byID[instanceID]
	if !ok {
		return false
	}
	now := time.Now()
	e.expireAt = now.Add(time.Duration(e.inst.TTLSeconds) * time.Second)
	e.dirtySeen = now
	if e.inst.Status != "UP" { // revive: status change is a write-through
		e.inst.Status = "UP"
		_ = c.st.UpdateStatus(instanceID, "UP")
	}
	return true
}

func (c *Core) Deregister(instanceID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.st.Delete(instanceID); err != nil {
		return err
	}
	c.removeLocked(instanceID)
	return nil
}

func (c *Core) removeLocked(instanceID string) {
	e, ok := c.byID[instanceID]
	if !ok {
		return
	}
	delete(c.byID, instanceID)
	if m := c.byName[e.inst.ServiceName]; m != nil {
		delete(m, instanceID)
		if len(m) == 0 {
			delete(c.byName, e.inst.ServiceName)
		}
	}
}

func (c *Core) Discover(serviceName string) []*store.Instance {
	c.mu.RLock()
	defer c.mu.RUnlock()
	now := time.Now()
	out := []*store.Instance{}
	for _, e := range c.byName[serviceName] {
		if e.inst.Status == "UP" && now.Before(e.expireAt) {
			cp := *e.inst
			out = append(out, &cp)
		}
	}
	return out
}

func (c *Core) List() []*store.Instance {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := []*store.Instance{}
	for _, e := range c.byID {
		cp := *e.inst
		out = append(out, &cp)
	}
	return out
}

// Tick runs one reaper pass: expired leases -> DOWN; expired+grace -> evict.
func (c *Core) Tick(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, e := range c.byID {
		if now.After(e.expireAt.Add(c.cfg.EvictGrace)) {
			_ = c.st.Delete(id)
			c.removeLocked(id)
			continue
		}
		if now.After(e.expireAt) && e.inst.Status == "UP" {
			e.inst.Status = "DOWN"
			_ = c.st.UpdateStatus(id, "DOWN")
		}
	}
}

func (c *Core) flush() {
	c.mu.Lock()
	seen := map[string]time.Time{}
	for id, e := range c.byID {
		if !e.dirtySeen.IsZero() {
			seen[id] = e.dirtySeen
			e.dirtySeen = time.Time{}
			e.inst.LastSeen = seen[id]
		}
	}
	c.mu.Unlock()
	_ = c.st.FlushLastSeen(seen)
}

func (c *Core) WarmStart() error {
	rows, err := c.st.LoadAll()
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for _, r := range rows {
		// loaded rows start expired so a missing service is reaped unless it heartbeats
		e := &entry{inst: r, expireAt: now}
		if c.byName[r.ServiceName] == nil {
			c.byName[r.ServiceName] = map[string]*entry{}
		}
		c.byName[r.ServiceName][r.InstanceID] = e
		c.byID[r.InstanceID] = e
	}
	return nil
}

func (c *Core) StartBackground(stop <-chan struct{}) {
	go func() {
		rt := time.NewTicker(c.cfg.ReaperTick)
		ft := time.NewTicker(c.cfg.FlushInterval)
		defer rt.Stop()
		defer ft.Stop()
		for {
			select {
			case <-stop:
				c.flush()
				return
			case <-rt.C:
				c.Tick(time.Now())
			case <-ft.C:
				c.flush()
			}
		}
	}()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd service-mgt && go mod tidy && go test ./internal/registry/ -v`
Expected: PASS (4 tests). `go mod tidy` adds `github.com/google/uuid`.

- [ ] **Step 5: Commit**

```bash
git add service-mgt/internal/registry service-mgt/go.mod service-mgt/go.sum
git commit -m "feat(service-mgt): registry core with lease TTL, reaper, flusher"
```

---

### Task 4: gRPC server handlers + entrypoint + config

**Files:**
- Create: `service-mgt/internal/grpcserver/server.go`
- Create: `service-mgt/internal/config/config.go`
- Create: `service-mgt/main.go`
- Test: `service-mgt/internal/grpcserver/server_test.go`

**Interfaces:**
- Consumes: `registry.Core`, proto `registry` package.
- Produces: `grpcserver.New(core *registry.Core) registrypb.RegistryServer`; `config.Load() Config` with `Config{GRPCPort string; DB store dsn fields; DefaultTTLSeconds, EvictGraceSeconds, FlushIntervalSeconds int}`.

- [ ] **Step 1: Write the failing server test (in-process gRPC via bufconn)**

`service-mgt/internal/grpcserver/server_test.go`:
```go
package grpcserver

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"go-stock-prediction/service-mgt/internal/registry"
	"go-stock-prediction/service-mgt/internal/store"
	registrypb "go-stock-prediction/service-mgt/proto/registry"
)

type memStore struct{ rows map[string]*store.Instance }
func (m *memStore) Upsert(i *store.Instance) error { cp := *i; m.rows[i.InstanceID] = &cp; return nil }
func (m *memStore) UpdateStatus(id, s string) error { if r, ok := m.rows[id]; ok { r.Status = s }; return nil }
func (m *memStore) Delete(id string) error { delete(m.rows, id); return nil }
func (m *memStore) FlushLastSeen(map[string]time.Time) error { return nil }
func (m *memStore) LoadAll() ([]*store.Instance, error) { return nil, nil }

func dial(t *testing.T) registrypb.RegistryClient {
	lis := bufconn.Listen(1 << 20)
	core := registry.New(&memStore{rows: map[string]*store.Instance{}},
		registry.Config{DefaultTTL: 30 * time.Second, EvictGrace: 60 * time.Second, FlushInterval: time.Hour, ReaperTick: time.Second})
	srv := grpc.NewServer()
	registrypb.RegisterRegistryServer(srv, New(core))
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil { t.Fatalf("dial: %v", err) }
	t.Cleanup(func() { conn.Close() })
	return registrypb.NewRegistryClient(conn)
}

func TestRegisterThenDiscover(t *testing.T) {
	c := dial(t)
	ctx := context.Background()
	reg, err := c.Register(ctx, &registrypb.RegisterRequest{ServiceName: "auth-svc", Address: "auth-svc", Port: 8120})
	if err != nil { t.Fatalf("register: %v", err) }
	if reg.InstanceId == "" { t.Fatalf("want instance id") }
	hb, err := c.Heartbeat(ctx, &registrypb.HeartbeatRequest{InstanceId: reg.InstanceId})
	if err != nil || !hb.Ok { t.Fatalf("heartbeat: %v ok=%v", err, hb.GetOk()) }
	disc, err := c.Discover(ctx, &registrypb.DiscoverRequest{ServiceName: "auth-svc"})
	if err != nil { t.Fatalf("discover: %v", err) }
	if len(disc.Instances) != 1 || disc.Instances[0].Port != 8120 { t.Fatalf("discover mismatch: %+v", disc.Instances) }
}
```

- [ ] **Step 2: Run the test to confirm it fails**

Run: `cd service-mgt && go test ./internal/grpcserver/ -v`
Expected: FAIL (compile error — `New` undefined).

- [ ] **Step 3: Implement the gRPC server**

`service-mgt/internal/grpcserver/server.go`:
```go
package grpcserver

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go-stock-prediction/service-mgt/internal/registry"
	"go-stock-prediction/service-mgt/internal/store"
	registrypb "go-stock-prediction/service-mgt/proto/registry"
)

type Server struct {
	registrypb.UnimplementedRegistryServer
	core *registry.Core
}

func New(core *registry.Core) *Server { return &Server{core: core} }

func toPB(i *store.Instance) *registrypb.Instance {
	return &registrypb.Instance{
		ServiceName: i.ServiceName, InstanceId: i.InstanceID, Address: i.Address,
		Port: i.Port, Metadata: i.Metadata, Status: i.Status,
	}
}

func (s *Server) Register(_ context.Context, in *registrypb.RegisterRequest) (*registrypb.RegisterResponse, error) {
	if in.ServiceName == "" || in.Address == "" || in.Port == 0 {
		return nil, status.Error(codes.InvalidArgument, "service_name, address, port required")
	}
	id, ttl, err := s.core.Register(&store.Instance{
		ServiceName: in.ServiceName, InstanceID: in.InstanceId, Address: in.Address,
		Port: in.Port, Metadata: in.Metadata, TTLSeconds: in.TtlSeconds,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "register: %v", err)
	}
	return &registrypb.RegisterResponse{InstanceId: id, LeaseTtlSeconds: ttl}, nil
}

func (s *Server) Heartbeat(_ context.Context, in *registrypb.HeartbeatRequest) (*registrypb.HeartbeatResponse, error) {
	if !s.core.Heartbeat(in.InstanceId) {
		return nil, status.Error(codes.NotFound, "unknown instance; re-register")
	}
	return &registrypb.HeartbeatResponse{Ok: true}, nil
}

func (s *Server) Deregister(_ context.Context, in *registrypb.DeregisterRequest) (*registrypb.Empty, error) {
	if err := s.core.Deregister(in.InstanceId); err != nil {
		return nil, status.Errorf(codes.Internal, "deregister: %v", err)
	}
	return &registrypb.Empty{}, nil
}

func (s *Server) Discover(_ context.Context, in *registrypb.DiscoverRequest) (*registrypb.DiscoverResponse, error) {
	insts := s.core.Discover(in.ServiceName)
	out := make([]*registrypb.Instance, 0, len(insts))
	for _, i := range insts {
		out = append(out, toPB(i))
	}
	return &registrypb.DiscoverResponse{Instances: out}, nil
}

func (s *Server) ListServices(_ context.Context, _ *registrypb.Empty) (*registrypb.ListServicesResponse, error) {
	insts := s.core.List()
	out := make([]*registrypb.Instance, 0, len(insts))
	for _, i := range insts {
		out = append(out, toPB(i))
	}
	return &registrypb.ListServicesResponse{Instances: out}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd service-mgt && go mod tidy && go test ./internal/grpcserver/ -v`
Expected: PASS.

- [ ] **Step 5: Implement config**

`service-mgt/internal/config/config.go`:
```go
package config

import (
	"fmt"
	"os"
)

type Config struct {
	GRPCPort             string
	DBHost, DBPort       string
	DBUser, DBPassword   string
	DBName               string
	DefaultTTLSeconds    int
	EvictGraceSeconds    int
	FlushIntervalSeconds int
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func Load() Config {
	return Config{
		GRPCPort:             env("GRPC_PORT", "8121"),
		DBHost:               env("POSTGRES_HOST", "localhost"),
		DBPort:               env("POSTGRES_PORT", "5432"),
		DBUser:               env("POSTGRES_USER", "postgres"),
		DBPassword:           env("POSTGRES_PASSWORD", "123"),
		DBName:               env("POSTGRES_DB", "go_stock_prediction"),
		DefaultTTLSeconds:    30,
		EvictGraceSeconds:    60,
		FlushIntervalSeconds: 30,
	}
}

func (c Config) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Ho_Chi_Minh",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName)
}
```

- [ ] **Step 6: Implement the entrypoint**

`service-mgt/main.go`:
```go
// Command service-mgt is the central service registry: services register on
// boot, renew a lease via Heartbeat, and resolve peers via Discover.
package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"google.golang.org/grpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"go-stock-prediction/service-mgt/internal/config"
	"go-stock-prediction/service-mgt/internal/grpcserver"
	"go-stock-prediction/service-mgt/internal/registry"
	"go-stock-prediction/service-mgt/internal/store"
	registrypb "go-stock-prediction/service-mgt/proto/registry"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		panic(fmt.Sprintf("load tz: %v", err))
	}
	time.Local = loc

	zerolog.TimeFieldFormat = time.RFC3339
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, NoColor: false}).With().Timestamp().Logger()
	log.Logger = logger

	cfg := config.Load()

	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{})
	if err != nil {
		logger.Fatal().Err(err).Msg("service-mgt: db connect failed")
	}
	st := store.NewGorm(db)

	core := registry.New(st, registry.Config{
		DefaultTTL:    time.Duration(cfg.DefaultTTLSeconds) * time.Second,
		EvictGrace:    time.Duration(cfg.EvictGraceSeconds) * time.Second,
		FlushInterval: time.Duration(cfg.FlushIntervalSeconds) * time.Second,
		ReaperTick:    time.Second,
	})
	if err := core.WarmStart(); err != nil {
		logger.Warn().Err(err).Msg("service-mgt: warm-start failed")
	}
	stop := make(chan struct{})
	core.StartBackground(stop)

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		logger.Fatal().Err(err).Msgf("service-mgt: listen :%s failed", cfg.GRPCPort)
	}
	grpcSrv := grpc.NewServer()
	registrypb.RegisterRegistryServer(grpcSrv, grpcserver.New(core))
	go func() {
		logger.Info().Msgf("service-mgt: gRPC registry listening on :%s", cfg.GRPCPort)
		if err := grpcSrv.Serve(lis); err != nil {
			logger.Fatal().Err(err).Msg("service-mgt: serve failed")
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)
	s := <-sig
	logger.Info().Msgf("service-mgt: received %v, shutting down", s)
	close(stop)
	grpcSrv.GracefulStop()
	logger.Info().Msg("service-mgt: stopped")
}
```

- [ ] **Step 7: Build the whole module**

Run: `cd service-mgt && go mod tidy && go build ./... && go test ./...`
Expected: build PASS; all tests PASS.

- [ ] **Step 8: Commit**

```bash
git add service-mgt
git commit -m "feat(service-mgt): gRPC server, config, and entrypoint"
```

---

### Task 5: Dockerfile + docker-compose service + env wiring

**Files:**
- Create: `deploy/service-mgt.Dockerfile`
- Modify: `deploy/docker-compose.yaml` (add `service-mgt` service; add env to `api-svc`)
- Modify: `.env` (add `SERVICE_MGT_ENABLED`, `REGISTRY_GRPC_TARGET`)

**Interfaces:**
- Produces: container `service-mgt` on the `app-network`, internal `expose: 8121`. Other services reach it at `service-mgt:8121`.

- [ ] **Step 1: Write the Dockerfile**

`deploy/service-mgt.Dockerfile`:
```dockerfile
# Build stage
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o service-mgt .

# Run stage
FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /app/service-mgt .
USER appuser
EXPOSE 8121
ENTRYPOINT ["./service-mgt"]
```

- [ ] **Step 2: Add the compose service**

In `deploy/docker-compose.yaml`, add after the `auth-svc:` block (before `api-svc:`):
```yaml
  service-mgt:
    build:
      context: ../service-mgt
      dockerfile: ../deploy/service-mgt.Dockerfile
    container_name: service-mgt
    depends_on:
      db:
        condition: service_healthy
    restart: unless-stopped
    environment:
      GRPC_PORT: "8121"
      POSTGRES_HOST: db
      POSTGRES_PORT: "5432"
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-123}
      POSTGRES_DB: go_stock_prediction
      LOG_FORMAT: "${LOG_FORMAT:-console}"
    expose:
      - "8121"
    networks:
      - app-network
```

- [ ] **Step 3: Wire env into api-svc (compose)**

In the `api-svc:` `environment:` block add:
```yaml
      SERVICE_MGT_ENABLED: "${SERVICE_MGT_ENABLED:-false}"
      REGISTRY_GRPC_TARGET: "service-mgt:8121"
```
And under `api-svc:` `depends_on:` add:
```yaml
      service-mgt:
        condition: service_started
```

- [ ] **Step 4: Add to `.env`**

Append to `.env`:
```
# Service Management (registry/discovery)
SERVICE_MGT_ENABLED=false
REGISTRY_GRPC_TARGET=service-mgt:8121
```

- [ ] **Step 5: Verify the image builds and starts**

Run:
```bash
docker compose --env-file .env -f deploy/docker-compose.yaml build service-mgt
docker compose --env-file .env -f deploy/docker-compose.yaml up -d service-mgt
docker logs service-mgt --tail 20
```
Expected: log line `service-mgt: gRPC registry listening on :8121`. No crash loop.

- [ ] **Step 6: Commit**

```bash
git add deploy/service-mgt.Dockerfile deploy/docker-compose.yaml .env
git commit -m "build(service-mgt): dockerfile, compose service, env flag wiring"
```

---

### Task 6: Go client SDK (register / heartbeat / discover + flag + fallback)

**Files:**
- Create: `service-mgt/client/client.go`
- Test: `service-mgt/client/client_test.go`

**Interfaces:**
- Produces (imported by api-svc, cli-svc as `regclient "go-stock-prediction/service-mgt/client"`):
  ```go
  type Options struct {
      Enabled       bool          // SERVICE_MGT_ENABLED
      RegistryTarget string       // REGISTRY_GRPC_TARGET
      ServiceName   string
      Address       string        // advertise addr (docker service name)
      Port          int32
      TTLSeconds    int32          // 0 => server default
      HeartbeatEvery time.Duration // default 10s
      DiscoverCacheTTL time.Duration // default 5s
  }
  type Client struct { /* ... */ }
  func New(opts Options) *Client
  func (c *Client) Start(ctx context.Context) error      // connect + Register + heartbeat loop; no-op if !Enabled
  func (c *Client) Stop()                                 // Deregister best-effort + close
  // Resolve returns "host:port" for serviceName. When disabled/unreachable/empty it returns staticFallback.
  func (c *Client) Resolve(serviceName, staticFallback string) string
  ```

- [ ] **Step 1: Write the failing client test (disabled mode falls back)**

`service-mgt/client/client_test.go`:
```go
package client

import (
	"context"
	"testing"
	"time"
)

func TestDisabledResolveReturnsFallback(t *testing.T) {
	c := New(Options{Enabled: false, ServiceName: "api-svc", Address: "api-svc", Port: 8118})
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("start (disabled) must not error: %v", err)
	}
	got := c.Resolve("auth-svc", "auth-svc:8120")
	if got != "auth-svc:8120" {
		t.Fatalf("disabled Resolve must return fallback, got %q", got)
	}
	c.Stop()
}

func TestEnabledUnreachableResolveFallsBack(t *testing.T) {
	c := New(Options{
		Enabled: true, RegistryTarget: "127.0.0.1:1", // nothing listening
		ServiceName: "api-svc", Address: "api-svc", Port: 8118,
		HeartbeatEvery: 50 * time.Millisecond, DiscoverCacheTTL: 10 * time.Millisecond,
	})
	_ = c.Start(context.Background()) // start must not block/panic even if registry is down
	got := c.Resolve("auth-svc", "auth-svc:8120")
	if got != "auth-svc:8120" {
		t.Fatalf("unreachable Resolve must fall back, got %q", got)
	}
	c.Stop()
}
```

- [ ] **Step 2: Run the test to confirm it fails**

Run: `cd service-mgt && go test ./client/ -v`
Expected: FAIL (compile error — `New`, `Options` undefined).

- [ ] **Step 3: Implement the client**

`service-mgt/client/client.go`:
```go
package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	registrypb "go-stock-prediction/service-mgt/proto/registry"
)

type Options struct {
	Enabled          bool
	RegistryTarget   string
	ServiceName      string
	Address          string
	Port             int32
	TTLSeconds       int32
	HeartbeatEvery   time.Duration
	DiscoverCacheTTL time.Duration
}

type cacheEntry struct {
	endpoints []string
	expireAt  time.Time
}

type Client struct {
	opts       Options
	conn       *grpc.ClientConn
	rc         registrypb.RegistryClient
	instanceID string
	stop       chan struct{}
	wg         sync.WaitGroup

	mu    sync.Mutex
	cache map[string]cacheEntry
}

func New(opts Options) *Client {
	if opts.HeartbeatEvery == 0 {
		opts.HeartbeatEvery = 10 * time.Second
	}
	if opts.DiscoverCacheTTL == 0 {
		opts.DiscoverCacheTTL = 5 * time.Second
	}
	return &Client{opts: opts, stop: make(chan struct{}), cache: map[string]cacheEntry{}}
}

// Start connects, registers, and runs the heartbeat loop. It never blocks on a
// dead registry: dial uses a non-blocking client and register failures are
// retried by the heartbeat loop. No-op when disabled.
func (c *Client) Start(_ context.Context) error {
	if !c.opts.Enabled {
		return nil
	}
	conn, err := grpc.NewClient(c.opts.RegistryTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	c.conn = conn
	c.rc = registrypb.NewRegistryClient(conn)
	c.register() // best-effort; loop retries
	c.wg.Add(1)
	go c.loop()
	return nil
}

func (c *Client) register() {
	if c.rc == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := c.rc.Register(ctx, &registrypb.RegisterRequest{
		ServiceName: c.opts.ServiceName, Address: c.opts.Address,
		Port: c.opts.Port, TtlSeconds: c.opts.TTLSeconds,
	})
	if err == nil {
		c.instanceID = resp.InstanceId
	}
}

func (c *Client) loop() {
	defer c.wg.Done()
	t := time.NewTicker(c.opts.HeartbeatEvery)
	defer t.Stop()
	for {
		select {
		case <-c.stop:
			return
		case <-t.C:
			if c.instanceID == "" {
				c.register()
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_, err := c.rc.Heartbeat(ctx, &registrypb.HeartbeatRequest{InstanceId: c.instanceID})
			cancel()
			if status.Code(err) == codes.NotFound {
				c.instanceID = "" // re-register next tick
			}
		}
	}
}

// Resolve returns host:port for serviceName, or staticFallback when disabled,
// unreachable, cache-empty, or the service has no UP instances.
func (c *Client) Resolve(serviceName, staticFallback string) string {
	if !c.opts.Enabled || c.rc == nil {
		return staticFallback
	}
	c.mu.Lock()
	if e, ok := c.cache[serviceName]; ok && time.Now().Before(e.expireAt) && len(e.endpoints) > 0 {
		ep := e.endpoints[0]
		c.mu.Unlock()
		return ep
	}
	c.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := c.rc.Discover(ctx, &registrypb.DiscoverRequest{ServiceName: serviceName})
	if err != nil || len(resp.Instances) == 0 {
		return staticFallback
	}
	eps := make([]string, 0, len(resp.Instances))
	for _, in := range resp.Instances {
		eps = append(eps, fmt.Sprintf("%s:%d", in.Address, in.Port))
	}
	c.mu.Lock()
	c.cache[serviceName] = cacheEntry{endpoints: eps, expireAt: time.Now().Add(c.opts.DiscoverCacheTTL)}
	c.mu.Unlock()
	return eps[0]
}

func (c *Client) Stop() {
	if !c.opts.Enabled {
		return
	}
	close(c.stop)
	c.wg.Wait()
	if c.rc != nil && c.instanceID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, _ = c.rc.Deregister(ctx, &registrypb.DeregisterRequest{InstanceId: c.instanceID})
		cancel()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd service-mgt && go test ./client/ -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add service-mgt/client
git commit -m "feat(service-mgt): Go client SDK with flag gating and static fallback"
```

---

### Task 7: Integrate api-svc — register on boot + discover peers via registry

**Files:**
- Modify: `api-svc/go.mod` (add `require go-stock-prediction/service-mgt vX` + `replace` to local path)
- Modify: `api-svc/pkg/config/init.go` (read `SERVICE_MGT_ENABLED`, `REGISTRY_GRPC_TARGET`)
- Modify: `api-svc/pkg/config/config.go` (+ `models_config`) — getters `GetServiceMgtEnabled() bool`, `GetRegistryTarget() string`
- Modify: `api-svc/cmd/main.go` (start client, resolve targets for authclient + grpcclient, stop on shutdown)
- Test: `api-svc/pkg/config/config_test.go` (assert defaults: disabled, target default)

**Interfaces:**
- Consumes: `regclient "go-stock-prediction/service-mgt/client"`, `config.GetServiceMgtEnabled()`, `config.GetRegistryTarget()`, existing `config.GetAuthGRPCConfig()` (static fallback) and `config.GetGRPCConfig().ClientTarget`.

- [ ] **Step 1: Add the module dependency**

In `api-svc/go.mod` add under `require (`:
```
	go-stock-prediction/service-mgt v0.0.0
```
and at end of file:
```
replace go-stock-prediction/service-mgt => ../service-mgt
```

- [ ] **Step 2: Write the failing config test**

Append to `api-svc/pkg/config/config_test.go` (create if absent with `package config`):
```go
func TestServiceMgtDefaults(t *testing.T) {
	os.Unsetenv("SERVICE_MGT_ENABLED")
	os.Unsetenv("REGISTRY_GRPC_TARGET")
	InitConfig()
	if GetServiceMgtEnabled() {
		t.Fatalf("SERVICE_MGT_ENABLED must default to false")
	}
	if GetRegistryTarget() != "localhost:8121" {
		t.Fatalf("registry target default = %q, want localhost:8121", GetRegistryTarget())
	}
}
```
(Add `import ("os"; "testing")` if the file is new.)

- [ ] **Step 3: Run the test to confirm it fails**

Run: `cd api-svc && go test ./pkg/config/ -run TestServiceMgtDefaults -v`
Expected: FAIL (undefined `GetServiceMgtEnabled` / `GetRegistryTarget`).

- [ ] **Step 4: Add config fields, loader, getters**

In `api-svc/pkg/models/models_config/config.go` add to the `Config` struct a section (mirror existing style):
```go
	ServiceMgt ServiceMgtConfig
```
and the type:
```go
type ServiceMgtConfig struct {
	Enabled        bool
	RegistryTarget string
}
```
In `api-svc/pkg/config/init.go`, within the config build, add:
```go
		ServiceMgt: models_config.ServiceMgtConfig{
			Enabled:        env.GetEnv("SERVICE_MGT_ENABLED", "false") == "true",
			RegistryTarget: env.GetEnv("REGISTRY_GRPC_TARGET", "localhost:8121"),
		},
```
In `api-svc/pkg/config/config.go` add:
```go
func GetServiceMgtEnabled() bool { return Get().ServiceMgt.Enabled }
func GetRegistryTarget() string  { return Get().ServiceMgt.RegistryTarget }
```

- [ ] **Step 5: Run the config test to verify it passes**

Run: `cd api-svc && go test ./pkg/config/ -run TestServiceMgtDefaults -v`
Expected: PASS.

- [ ] **Step 6: Wire the client into main**

In `api-svc/cmd/main.go`, add import `regclient "go-stock-prediction/service-mgt/client"` and `"context"`. Replace the two client-init lines so targets resolve through the registry with static fallback:
```go
	// Service registry client (no-op when SERVICE_MGT_ENABLED=false)
	reg := regclient.New(regclient.Options{
		Enabled:        config.GetServiceMgtEnabled(),
		RegistryTarget: config.GetRegistryTarget(),
		ServiceName:    "api-svc",
		Address:        "api-svc",
		Port:           8118,
	})
	if err := reg.Start(context.Background()); err != nil {
		logger.Logger.Warnf("service registry start failed, using static targets: %v", err)
	}

	authTarget := reg.Resolve("auth-svc", config.GetAuthGRPCConfig())
	predTarget := reg.Resolve("prediction-svc", config.GetGRPCConfig().ClientTarget)

	authclient.Init(authTarget)
	grpcclient.Init(predTarget)
```
And in the shutdown section add `reg.Stop()` before `authclient.Close()`.

- [ ] **Step 7: Build api-svc**

Run: `cd api-svc && go mod tidy && go build ./cmd && go test ./pkg/config/ -v`
Expected: build PASS; config tests PASS.

- [ ] **Step 8: Commit**

```bash
git add api-svc/go.mod api-svc/go.sum api-svc/pkg/config api-svc/pkg/models/models_config api-svc/cmd/main.go
git commit -m "feat(api-svc): resolve auth/prediction targets via service registry (flag-gated, static fallback)"
```

---

### Task 8: End-to-end verification + docs

**Files:**
- Modify: `CLAUDE.md` (architecture: add service-mgt; Ports table `:8121`; compose service; env vars; registry/discovery convention)
- Modify: `deploy/api-svc.Dockerfile` — confirm build context includes sibling module. **Note:** api-svc Dockerfile build context is `../api-svc`, which does NOT include `../service-mgt`. The `replace` directive needs the sibling at build time. Fix: change `api-svc` compose `build.context` to repo root `..` and `dockerfile` accordingly, OR vendor. Use context `..` + `dockerfile: deploy/api-svc.Dockerfile` and update COPY paths.

**Interfaces:** none (integration + docs).

- [ ] **Step 1: Fix api-svc build context for the sibling module**

In `deploy/docker-compose.yaml` `api-svc:` change:
```yaml
    build:
      context: ..
      dockerfile: deploy/api-svc.Dockerfile
```
In `deploy/api-svc.Dockerfile` adjust to copy both modules:
```dockerfile
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git make
WORKDIR /src
COPY service-mgt/ ./service-mgt/
COPY api-svc/ ./api-svc/
WORKDIR /src/api-svc
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /api-server ./cmd
```
(Keep the run stage; copy `/api-server`.)

- [ ] **Step 2: Bring the stack up with the flag OFF (regression check)**

Run:
```bash
docker compose --env-file .env -f deploy/docker-compose.yaml up -d --build api-svc service-mgt
docker logs api-svc --tail 30
```
Expected: api-svc boots normally; with `SERVICE_MGT_ENABLED=false` it logs nothing about registry resolution failures and connects to `auth-svc:8120` / `prediction-svc:8119` as before. Hit `curl -s localhost:8118/health` (via gateway or direct) — healthy.

- [ ] **Step 3: Flip the flag ON and verify discovery**

Run:
```bash
SERVICE_MGT_ENABLED=true docker compose --env-file .env -f deploy/docker-compose.yaml up -d service-mgt api-svc
sleep 6
docker logs service-mgt --tail 30
```
Expected: service-mgt logs a Register from `api-svc`. (auth-svc/prediction-svc are NOT yet registering — that is Phase 2 — so api-svc's `Resolve("auth-svc", ...)` returns the **static fallback** `auth-svc:8120`. Confirm api-svc still works: login via `POST /api/auth/login` succeeds.)

- [ ] **Step 4: Inspect the registry directly (optional sanity)**

Run (from a host with grpcurl, or add a tiny debug later):
```bash
docker exec service-mgt sh -c 'wget -qO- http://localhost:8121 2>/dev/null || echo "gRPC only"'
```
Expected: confirms process is up. (No HTTP endpoint by design; `ListServices` is gRPC.)

- [ ] **Step 5: Update CLAUDE.md**

Add to the architecture bullets a `Service Management` entry; add `service-mgt` row to the Ports table (`8121 / gRPC / internal`); add it to Docker Compose Services table (depends_on db); add env vars `SERVICE_MGT_ENABLED`, `REGISTRY_GRPC_TARGET`; add a "Service discovery" convention paragraph describing register/heartbeat/discover, write-through cache, lease-TTL, and the flag + static fallback rule. (Use the doc-updater agent or edit directly.)

- [ ] **Step 6: Commit**

```bash
git add deploy/api-svc.Dockerfile deploy/docker-compose.yaml CLAUDE.md
git commit -m "build,docs(service-mgt): repo-root build context for api-svc; phase 1 docs"
```

---

## Self-Review

**Spec coverage:**
- Registry (Go gRPC :8121) → Tasks 1,4. Register/Heartbeat/Deregister/Discover/ListServices → Task 1 proto, Task 4 handlers.
- Client-side discovery (no proxy) → Task 6 `Resolve` returns endpoint; caller dials directly.
- Push lease-TTL heartbeat → Task 3 core (`expireAt`, reaper), Task 6 heartbeat loop.
- Write-through cache; heartbeat cache-only; periodic flush → Task 2 store, Task 3 (`Register`/`UpdateStatus`/`Deregister` write-through, `Heartbeat` cache-only, `flush()`).
- In-memory read layer → Task 3 `byName`/`byID`, `Discover` reads cache.
- Discover client cache TTL + fail/refresh → Task 6 `cache`, `DiscoverCacheTTL`. (Refresh-on-fail: cache only stores successful lookups; a failed peer call by the caller triggers a fresh `Resolve` which re-queries when TTL elapsed. Per-call invalidation on dial failure is a Phase 2 refinement — noted, not silently dropped.)
- Env flag default false + static fallback → Task 6 (`Enabled` gating), Task 7 wiring, Task 5 `.env`.
- Bootstrap seed `REGISTRY_GRPC_TARGET` → Task 5/7.
- DB table `service_instances` in database.sql → Task 2.
- Docker-compose service + Dockerfile → Task 5; build-context fix → Task 8.
- Phase 1 = foundation + Go client + api-svc integration → all tasks. Phase 2 (Java/Python register) and Phase 3 (Rust gateway/cli-svc) are separate plans, explicitly out of scope here.

**Placeholder scan:** No TBD/TODO; all code steps contain full code. `ListServices` debug HTTP intentionally omitted (gRPC only) and called out.

**Type consistency:** `store.Instance` fields (`InstanceID`, `TTLSeconds`, `LastSeen`) used consistently across store/registry/grpcserver. Proto field names (`InstanceId`, `TtlSeconds`, `LeaseTtlSeconds`) match generated Go casing used in tests and handlers. `regclient.Options`/`Resolve` signatures match Task 7 usage. Core method set in Task 3 interface matches calls in Task 4 server and `main`.

**Known follow-ups (Phase 2/3, not gaps):** auth-svc/prediction-svc register; gateway-svc & cli-svc integration; per-call dial-failure cache invalidation; optional grpc health/reflection for `grpcurl`.
