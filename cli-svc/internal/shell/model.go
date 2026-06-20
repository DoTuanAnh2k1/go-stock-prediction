// Package shell implements a per-SSH-session bubbletea shell. Each SSH session
// gets its own Model instance, which keeps the shell multi-session safe.
package shell

import (
	"context"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	th      *Theme

	input  textinput.Model
	loaded bool
	width  int
	height int
	err    string

	// live completion dropdown state (kube-prompt style)
	suggest []Suggestion
	sugIdx  int // highlighted row
	sugCol  int // screen column where the current token starts

	// completion-cycle state: while cycling with Tab/arrows the candidate list
	// is frozen so it doesn't collapse to the just-inserted value.
	completing bool
	compBase   string       // input text before the token being completed
	compList   []Suggestion // frozen candidate list for the cycle

	// showSuggest: the dropdown is only visible after the user presses Tab; it
	// stays hidden while typing so the screen isn't cluttered.
	showSuggest bool

	// command history (most-recent last); histIdx is the recall cursor.
	cmdHistory []string
	histIdx    int
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

// New builds a Model for an SSH session. The renderer (from the SSH session) is
// required for color to survive the wish/PTY boundary; pass nil in tests.
func New(sess Session, c client.HTTPClient, reg *handlers.Registry, r *lipgloss.Renderer) Model {
	th := NewTheme(r)

	ti := textinput.New()
	ti.Placeholder = "get market latest market gold"
	ti.Prompt = "> "
	ti.PromptStyle = th.Prompt
	ti.TextStyle = th.Text
	ti.PlaceholderStyle = th.Dim
	// Bind the cursor styles to the session renderer, otherwise its reverse-video
	// block is stripped by the default (non-TTY) renderer and no cursor shows.
	ti.Cursor.Style = th.Cursor
	ti.Cursor.TextStyle = th.Text
	ti.Focus()
	ti.CharLimit = 512

	return Model{
		sess:   sess,
		client: c,
		reg:    reg,
		th:     th,
		input:  ti,
	}
}

// Init starts the cursor blink and loads the user's allowed commands.
func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.loadCommands())
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
	cmdPart, grepPart := splitPipe(line)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60e9)
		defer cancel()
		out, _ := runner.Run(ctx, cmdPart)
		if grepPart != "" {
			out = applyGrep(out, grepPart)
		}
		return resultMsg{output: out}
	}
}
