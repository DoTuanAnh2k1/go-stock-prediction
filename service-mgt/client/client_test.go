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
