// Package server wires the wish SSH server: password auth against the API,
// per-session bubbletea shells, host-key management and the boot-time handler
// catalog upsert.
package server

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	bm "github.com/charmbracelet/wish/bubbletea"
	tea "github.com/charmbracelet/bubbletea"

	"go-stock-prediction/cli-svc/internal/client"
	"go-stock-prediction/cli-svc/internal/handlers"
	"go-stock-prediction/cli-svc/internal/shell"
)

// context keys for values stored at SSH auth time.
type ctxKey string

const (
	ctxKeyJWT    ctxKey = "cli.jwt"
	ctxKeyRole   ctxKey = "cli.role"
	ctxKeyUserID ctxKey = "cli.user_id"
)

// Server is the cli-svc SSH server.
type Server struct {
	cfg    Config
	client client.HTTPClient
	reg    *handlers.Registry
}

// New builds a Server with the default catalog and HTTP client.
func New(cfg Config) *Server {
	return &Server{
		cfg:    cfg,
		client: client.New(cfg.APIBaseURL),
		reg:    handlers.NewRegistry(),
	}
}

// NewWith builds a Server with an injected HTTP client (used in tests).
func NewWith(cfg Config, c client.HTTPClient, reg *handlers.Registry) *Server {
	return &Server{cfg: cfg, client: c, reg: reg}
}

// passwordHandler authenticates the SSH user against POST /auth/login and, on
// success, stashes the JWT/role/user_id into the SSH context.
func (s *Server) passwordHandler(ctx ssh.Context, password string) bool {
	c, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	res, err := s.client.Login(c, ctx.User(), password)
	if err != nil || res == nil || res.Token == "" {
		log.Printf("ssh auth failed for user=%q: %v", ctx.User(), err)
		return false
	}
	ctx.SetValue(ctxKeyJWT, res.Token)
	ctx.SetValue(ctxKeyRole, res.Role)
	ctx.SetValue(ctxKeyUserID, res.UserID)
	log.Printf("ssh auth ok for user=%q role=%q", ctx.User(), res.Role)
	return true
}

// teaHandler builds a fresh bubbletea model for each session (multi-session
// safe — one Model per connection).
func (s *Server) teaHandler(sess ssh.Session) (tea.Model, []tea.ProgramOption) {
	jwt, _ := sess.Context().Value(ctxKeyJWT).(string)
	role, _ := sess.Context().Value(ctxKeyRole).(string)
	uid, _ := sess.Context().Value(ctxKeyUserID).(int64)

	// Per-session renderer: detects the client's color profile from the PTY so
	// lipgloss emits ANSI. Using the default renderer would key off the server's
	// stdout (not a TTY) and strip all color.
	renderer := bm.MakeRenderer(sess)

	m := shell.New(shell.Session{
		Username: sess.User(),
		Role:     role,
		UserID:   uid,
		JWT:      jwt,
	}, s.client, s.reg, renderer)

	return m, []tea.ProgramOption{tea.WithAltScreen()}
}

// upsertCatalog pushes the handler catalog to the API on boot. The gateway/API
// may still be starting, so it retries a few times with backoff. Failures are
// non-fatal — the SSH server still serves; the catalog just won't be registered.
func (s *Server) upsertCatalog() {
	if s.cfg.InternalSecret == "" {
		log.Printf("INTERNAL_SECRET empty — skipping handler catalog upsert")
		return
	}
	payload := s.reg.UpsertPayload(s.cfg.InternalSecret)
	const attempts = 10
	for i := 1; i <= attempts; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		status, err := s.client.PostInternal(ctx, "/command-handlers/upsert", payload)
		cancel()
		// Success only on a 2xx. A non-2xx (e.g. 500 while auth-svc is still
		// starting and api-svc cannot reach it) is retryable, not done.
		if err == nil && status >= 200 && status < 300 {
			log.Printf("handler catalog upsert: HTTP %d (%d handlers)", status, len(payload.Handlers))
			return
		}
		if err != nil {
			log.Printf("handler catalog upsert attempt %d/%d failed: %v", i, attempts, err)
		} else {
			log.Printf("handler catalog upsert attempt %d/%d got HTTP %d, retrying", i, attempts, status)
		}
		time.Sleep(3 * time.Second)
	}
	log.Printf("handler catalog upsert giving up after %d attempts", attempts)
}

// ListenAndServe starts the SSH server (blocking).
func (s *Server) ListenAndServe() error {
	if err := os.MkdirAll(filepath.Dir(s.cfg.HostKeyPath), 0o700); err != nil {
		log.Printf("warning: could not create host key dir: %v", err)
	}

	srv, err := wish.NewServer(
		wish.WithAddress(s.cfg.SSHListenAddr),
		wish.WithHostKeyPath(s.cfg.HostKeyPath),
		wish.WithPasswordAuth(s.passwordHandler),
		wish.WithMiddleware(
			bm.Middleware(s.teaHandler),
		),
	)
	if err != nil {
		return err
	}

	// Best-effort catalog upsert in the background so a slow/unavailable API
	// does not block SSH availability.
	go s.upsertCatalog()

	log.Printf("cli-svc SSH server listening on %s (API=%s)", s.cfg.SSHListenAddr, s.cfg.APIBaseURL)
	return srv.ListenAndServe()
}
