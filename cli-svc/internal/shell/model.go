// Package shell implements a per-SSH-session bubbletea shell. Each SSH session
// gets its own Model instance, which keeps the shell multi-session safe.
package shell

import (
	"context"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"go-stock-prediction/cli-svc/internal/client"
	"go-stock-prediction/cli-svc/internal/handlers"
)

// Session carries the authenticated identity established at SSH login time.
type Session struct {
	Username string
	Role     string
	UserID   int64
	JWT      string
}

// Model is a single SSH session's bubbletea model.
type Model struct {
	sess    Session
	client  client.HTTPClient
	reg     *handlers.Registry
	allowed *AllowedSet
	runner  *Runner
	comp    *Completer

	input    textinput.Model
	viewport viewport.Model
	history  []string // rendered output blocks
	ready    bool
	width    int
	height   int
	err      string
}

// commandsLoadedMsg is emitted once GET /me/commands resolves.
type commandsLoadedMsg struct {
	cmds []client.AllowedCommand
	err  error
}

// resultMsg is emitted when a command finishes executing.
type resultMsg struct {
	output string
}

// New builds a Model for an SSH session.
func New(sess Session, c client.HTTPClient, reg *handlers.Registry) Model {
	ti := textinput.New()
	ti.Placeholder = "get market.latest market=gold"
	ti.Prompt = "> "
	ti.Focus()
	ti.CharLimit = 512

	return Model{
		sess:   sess,
		client: c,
		reg:    reg,
		input:  ti,
	}
}

// Init kicks off loading the user's allowed commands.
func (m Model) Init() tea.Cmd {
	return m.loadCommands()
}

func (m Model) loadCommands() tea.Cmd {
	c := m.client
	jwt := m.sess.JWT
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15e9)
		defer cancel()
		cmds, err := c.GetMyCommands(ctx, jwt)
		return commandsLoadedMsg{cmds: cmds, err: err}
	}
}

func (m Model) runCommand(line string) tea.Cmd {
	runner := m.runner
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60e9)
		defer cancel()
		out, _ := runner.Run(ctx, line)
		return resultMsg{output: out}
	}
}
