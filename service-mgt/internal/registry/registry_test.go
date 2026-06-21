package registry

import (
	"testing"
	"time"

	"go-stock-prediction/service-mgt/internal/store"
)

type fakeStore struct{ rows map[string]*store.Instance }

func newFake() *fakeStore { return &fakeStore{rows: map[string]*store.Instance{}} }
func (f *fakeStore) Upsert(i *store.Instance) error {
	cp := *i
	f.rows[i.InstanceID] = &cp
	return nil
}
func (f *fakeStore) UpdateStatus(id, s string) error {
	if r, ok := f.rows[id]; ok {
		r.Status = s
	}
	return nil
}
func (f *fakeStore) Delete(id string) error { delete(f.rows, id); return nil }
func (f *fakeStore) FlushLastSeen(m map[string]time.Time) error { return nil }
func (f *fakeStore) LoadAll() ([]*store.Instance, error) {
	out := []*store.Instance{}
	for _, v := range f.rows {
		out = append(out, v)
	}
	return out, nil
}

func testCfg() Config {
	return Config{DefaultTTL: 30 * time.Second, EvictGrace: 60 * time.Second, FlushInterval: time.Hour, ReaperTick: time.Second}
}

func TestRegisterDiscover(t *testing.T) {
	c := New(newFake(), testCfg())
	id, ttl, err := c.Register(&store.Instance{ServiceName: "auth-svc", Address: "auth-svc", Port: 8120})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if id == "" || ttl != 30 {
		t.Fatalf("bad register result id=%q ttl=%d", id, ttl)
	}
	got := c.Discover("auth-svc")
	if len(got) != 1 || got[0].Port != 8120 {
		t.Fatalf("discover mismatch: %+v", got)
	}
}

func TestLeaseExpiryMarksDown(t *testing.T) {
	c := New(newFake(), testCfg())
	id, _, _ := c.Register(&store.Instance{ServiceName: "x", Address: "x", Port: 1})
	// no heartbeat; advance past TTL
	c.Tick(time.Now().Add(31 * time.Second))
	if len(c.Discover("x")) != 0 {
		t.Fatalf("expired instance must not be discoverable")
	}
	// heartbeat on expired-but-not-evicted lease revives it
	if ok := c.Heartbeat(id); !ok {
		t.Fatalf("heartbeat should still find the instance before grace evict")
	}
	if len(c.Discover("x")) != 1 {
		t.Fatalf("heartbeat should revive to UP")
	}
}

func TestHeartbeatUnknown(t *testing.T) {
	c := New(newFake(), testCfg())
	if c.Heartbeat("nope") {
		t.Fatalf("unknown heartbeat must return false")
	}
}

func TestEvictAfterGrace(t *testing.T) {
	c := New(newFake(), testCfg())
	c.Register(&store.Instance{ServiceName: "x", InstanceID: "x-1", Address: "x", Port: 1})
	base := time.Now()
	c.Tick(base.Add(31 * time.Second)) // -> DOWN
	c.Tick(base.Add(95 * time.Second)) // past TTL+grace -> evict
	if len(c.List()) != 0 {
		t.Fatalf("instance should be evicted after grace, got %d", len(c.List()))
	}
}
