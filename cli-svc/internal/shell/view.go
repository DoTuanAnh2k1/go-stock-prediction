package shell

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	promptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

func (m *Model) resizeViewport() {
	w := m.width
	h := m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	vpHeight := h - 4 // header + input + spacing
	if vpHeight < 3 {
		vpHeight = 3
	}
	if !m.ready {
		m.viewport = viewport.New(w, vpHeight)
		m.ready = true
	} else {
		m.viewport.Width = w
		m.viewport.Height = vpHeight
	}
	m.input.Width = w - 4
}

func (m *Model) refreshViewport() {
	if !m.ready {
		m.resizeViewport()
	}
	m.viewport.SetContent(strings.Join(m.history, "\n\n"))
	m.viewport.GotoBottom()
}

// View renders the full screen.
func (m Model) View() string {
	if !m.ready {
		return "initializing...\n"
	}
	header := titleStyle.Render("stock-prediction CLI") + dimStyle.Render(
		fmt.Sprintf("  %s@%s", m.sess.Username, roleLabel(m.sess.Role)))
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString(m.viewport.View())
	b.WriteString("\n")
	if m.err != "" {
		b.WriteString(errStyle.Render(m.err))
		b.WriteString("\n")
	}
	b.WriteString(promptStyle.Render(m.input.View()))
	return b.String()
}

func roleLabel(role string) string {
	if role == "" {
		return "user"
	}
	return role
}

func (m Model) welcome() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Welcome, "+m.sess.Username) + "\n")
	if m.allowed.IsSuperAdmin() {
		b.WriteString("Role: super_admin — you may run any command.\n")
	} else {
		n := len(m.allowed.Commands())
		b.WriteString(fmt.Sprintf("Role: %s — %d command(s) granted.\n", roleLabel(m.sess.Role), n))
	}
	b.WriteString(dimStyle.Render("Type 'help' for the command reference, 'clear' to reset, 'exit' to quit. Tab completes."))
	return b.String()
}

func (m Model) helpText() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Command reference") + "\n")
	b.WriteString("Syntax: <verb> <resource> [key=value ...]\n")
	b.WriteString("Verbs: get (GET), set (POST), update (PUT), delete (DELETE)\n\n")
	for _, h := range m.reg.All() {
		if !m.allowed.Allows(h.Key) {
			continue
		}
		args := make([]string, 0, len(h.ArgSchema))
		for _, a := range h.ArgSchema {
			tok := a.Name + "="
			if len(a.Choices) > 0 {
				tok += "{" + strings.Join(a.Choices, "|") + "}"
			} else {
				tok += "<" + a.Type + ">"
			}
			if !a.Required {
				tok = "[" + tok + "]"
			}
			args = append(args, tok)
		}
		b.WriteString(fmt.Sprintf("  %-7s %-22s %s\n", h.Verb, h.Resource, strings.Join(args, " ")))
	}
	return b.String()
}
