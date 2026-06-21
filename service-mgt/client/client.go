// Package client is the thin service-mgt SDK embedded by each Go service: it
// registers on boot, renews its lease in the background, and resolves peer
// endpoints with a short cache — always falling back to a static target when
// the registry is disabled or unreachable.
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
// dead registry: the dial is non-blocking and register failures are retried by
// the heartbeat loop. No-op when disabled.
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

// Stop ends the heartbeat loop, best-effort deregisters, and closes the conn.
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
