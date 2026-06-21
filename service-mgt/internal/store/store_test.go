package store

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestStore(t *testing.T) Store {
	// Unique in-memory DB name per test (cache=shared lets the connection pool
	// see the same DB) so tests stay isolated from one another.
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&Instance{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewGorm(db)
}

func TestUpsertThenLoadAll(t *testing.T) {
	s := newTestStore(t)
	in := &Instance{ServiceName: "api-svc", InstanceID: "api-1", Address: "api-svc", Port: 8118, Status: "UP", TTLSeconds: 30}
	if err := s.Upsert(in); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// second upsert with same instance_id must update, not duplicate
	in.Port = 9999
	if err := s.Upsert(in); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	all, err := s.LoadAll()
	if err != nil {
		t.Fatalf("loadall: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("want 1 row, got %d", len(all))
	}
	if all[0].Port != 9999 {
		t.Fatalf("want updated port 9999, got %d", all[0].Port)
	}
}

func TestUpdateStatusAndDelete(t *testing.T) {
	s := newTestStore(t)
	_ = s.Upsert(&Instance{ServiceName: "x", InstanceID: "x-1", Address: "x", Port: 1, Status: "UP", TTLSeconds: 30})
	if err := s.UpdateStatus("x-1", "DOWN"); err != nil {
		t.Fatalf("status: %v", err)
	}
	all, _ := s.LoadAll()
	if all[0].Status != "DOWN" {
		t.Fatalf("want DOWN, got %s", all[0].Status)
	}
	if err := s.Delete("x-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	all, _ = s.LoadAll()
	if len(all) != 0 {
		t.Fatalf("want 0 after delete, got %d", len(all))
	}
}

func TestFlushLastSeen(t *testing.T) {
	s := newTestStore(t)
	_ = s.Upsert(&Instance{ServiceName: "x", InstanceID: "x-1", Address: "x", Port: 1, Status: "UP", TTLSeconds: 30})
	ts := time.Now().Add(time.Hour).Truncate(time.Second)
	if err := s.FlushLastSeen(map[string]time.Time{"x-1": ts}); err != nil {
		t.Fatalf("flush: %v", err)
	}
	all, _ := s.LoadAll()
	if !all[0].LastSeen.Equal(ts) {
		t.Fatalf("want %v, got %v", ts, all[0].LastSeen)
	}
}
