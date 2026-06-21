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
	inst      *store.Instance
	expireAt  time.Time
	dirtySeen time.Time // last heartbeat time not yet flushed to DB
}

// Core is the in-memory registry cache backed by a write-through Store.
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

// Register persists the instance (write-through) then caches it with a fresh lease.
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

// Heartbeat renews a lease in cache only. A status revival (DOWN->UP) is a
// write-through. Returns false if the instance is unknown (client re-registers).
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
	if e.inst.Status != "UP" {
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

// Discover returns healthy (UP, unexpired) instances of serviceName from cache.
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

// flush persists the latest heartbeat times accumulated since the last flush.
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

// WarmStart loads persisted rows into the cache, marked already-expired so a
// service that does not heartbeat is reaped after the eviction grace.
func (c *Core) WarmStart() error {
	rows, err := c.st.LoadAll()
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for _, r := range rows {
		e := &entry{inst: r, expireAt: now}
		if c.byName[r.ServiceName] == nil {
			c.byName[r.ServiceName] = map[string]*entry{}
		}
		c.byName[r.ServiceName][r.InstanceID] = e
		c.byID[r.InstanceID] = e
	}
	return nil
}

// StartBackground runs the reaper and flusher loops until stop is closed.
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
