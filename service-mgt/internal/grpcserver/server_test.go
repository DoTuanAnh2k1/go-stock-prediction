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
func (m *memStore) UpdateStatus(id, s string) error {
	if r, ok := m.rows[id]; ok {
		r.Status = s
	}
	return nil
}
func (m *memStore) Delete(id string) error                       { delete(m.rows, id); return nil }
func (m *memStore) FlushLastSeen(map[string]time.Time) error     { return nil }
func (m *memStore) LoadAll() ([]*store.Instance, error)          { return nil, nil }

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
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return registrypb.NewRegistryClient(conn)
}

func TestRegisterThenDiscover(t *testing.T) {
	c := dial(t)
	ctx := context.Background()
	reg, err := c.Register(ctx, &registrypb.RegisterRequest{ServiceName: "auth-svc", Address: "auth-svc", Port: 8120})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if reg.InstanceId == "" {
		t.Fatalf("want instance id")
	}
	hb, err := c.Heartbeat(ctx, &registrypb.HeartbeatRequest{InstanceId: reg.InstanceId})
	if err != nil || !hb.Ok {
		t.Fatalf("heartbeat: %v ok=%v", err, hb.GetOk())
	}
	disc, err := c.Discover(ctx, &registrypb.DiscoverRequest{ServiceName: "auth-svc"})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(disc.Instances) != 1 || disc.Instances[0].Port != 8120 {
		t.Fatalf("discover mismatch: %+v", disc.Instances)
	}
}

func TestRegisterValidation(t *testing.T) {
	c := dial(t)
	_, err := c.Register(context.Background(), &registrypb.RegisterRequest{ServiceName: "x"}) // missing address/port
	if err == nil {
		t.Fatalf("expected InvalidArgument for missing address/port")
	}
}
