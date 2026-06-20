package shell

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

const maxSuggestRows = 7

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	promptStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	errStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203"))

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("231")).
			Background(lipgloss.Color("24")).
			Padding(0, 1)
	roleBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("232")).
			Background(lipgloss.Color("114")).
			Padding(0, 1)

	// Dropdown (kube-prompt style): solid blocks, highlighted selection.
	sugTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Background(lipgloss.Color("238"))
	sugDescStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Background(lipgloss.Color("238"))
	sugSelText   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("232")).Background(lipgloss.Color("212"))
	sugSelDesc   = lipgloss.NewStyle().Foreground(lipgloss.Color("236")).Background(lipgloss.Color("212"))
	sugScrollSty = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Background(lipgloss.Color("238"))
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
	// header(1) + err(1) + input(1) + dropdown(maxSuggestRows)
	vpHeight := h - 3 - maxSuggestRows
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

	var b strings.Builder
	b.WriteString(m.headerBar())
	b.WriteString("\n")
	b.WriteString(m.viewport.View())
	b.WriteString("\n")
	if m.err != "" {
		b.WriteString(errStyle.Render("✗ " + m.err))
	} else {
		b.WriteString(dimStyle.Render(m.hintLine()))
	}
	b.WriteString("\n")
	b.WriteString(m.input.View())
	b.WriteString("\n")
	b.WriteString(m.renderSuggest())
	return b.String()
}

// headerBar is the top status bar spanning the terminal width.
func (m Model) headerBar() string {
	left := headerStyle.Render("stock-prediction CLI")
	badge := roleBadge.Render(m.sess.Username + " · " + roleLabel(m.sess.Role))
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(badge)
	if gap < 1 {
		gap = 1
	}
	bg := lipgloss.NewStyle().Background(lipgloss.Color("24"))
	return left + bg.Render(strings.Repeat(" ", gap)) + badge
}

func (m Model) hintLine() string {
	return "↑/↓ navigate · Tab complete · Enter run · help · clear · exit"
}

// renderSuggest draws the floating completion dropdown below the prompt. It
// always emits exactly maxSuggestRows lines so the layout never jumps.
func (m Model) renderSuggest() string {
	if len(m.suggest) == 0 {
		return strings.Repeat("\n", maxSuggestRows-1)
	}

	// Column widths across ALL suggestions (stable while typing).
	maxText, maxDesc := 0, 0
	for _, s := range m.suggest {
		if n := utf8.RuneCountInString(s.Text); n > maxText {
			maxText = n
		}
		if n := utf8.RuneCountInString(s.Desc); n > maxDesc {
			maxDesc = n
		}
	}
	// Clamp the box to the terminal width.
	avail := m.width - m.sugCol - 1
	if avail < 10 {
		avail = 10
	}
	if maxText+maxDesc+4 > avail {
		maxDesc = avail - maxText - 4
		if maxDesc < 0 {
			maxDesc = 0
		}
	}

	// Sliding window so the highlighted row stays visible.
	start := 0
	if len(m.suggest) > maxSuggestRows {
		if m.sugIdx >= maxSuggestRows {
			start = m.sugIdx - maxSuggestRows + 1
		}
	}
	end := start + maxSuggestRows
	if end > len(m.suggest) {
		end = len(m.suggest)
	}

	indent := strings.Repeat(" ", m.sugCol)
	var lines []string
	for i := start; i < end; i++ {
		s := m.suggest[i]
		more := ""
		if i == start && start > 0 {
			more = "▲"
		} else if i == end-1 && end < len(m.suggest) {
			more = "▼"
		}
		textStyle, descStyle := sugTextStyle, sugDescStyle
		if i == m.sugIdx {
			textStyle, descStyle = sugSelText, sugSelDesc
		}
		textCell := textStyle.Render(" " + ljust(s.Text, maxText) + "  ")
		descCell := descStyle.Render(ljust(s.Desc, maxDesc) + " ")
		scroll := sugScrollSty.Render(rjust(more, 1) + " ")
		lines = append(lines, indent+textCell+descCell+scroll)
	}
	// Pad to a fixed height for a stable layout.
	for len(lines) < maxSuggestRows-1 {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func ljust(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		if n <= 1 {
			return string(r[:n])
		}
		return string(r[:n-1]) + "…"
	}
	return s + strings.Repeat(" ", n-len(r))
}

func rjust(s string, n int) string {
	r := []rune(s)
	if len(r) >= n {
		return s
	}
	return strings.Repeat(" ", n-len(r)) + s
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
		b.WriteString(dimStyle.Render("super_admin — you may run any command.") + "\n")
	} else {
		n := len(m.allowed.Commands())
		b.WriteString(dimStyle.Render(fmt.Sprintf("%s — %d command(s) granted.", roleLabel(m.sess.Role), n)) + "\n")
	}
	b.WriteString(dimStyle.Render("Start typing a verb (get/set/update/delete); suggestions appear below."))
	return b.String()
}

func (m Model) helpText() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Command reference") + "\n")
	b.WriteString(dimStyle.Render("Syntax: <verb> <resource> [key=value ...]") + "\n")
	b.WriteString(dimStyle.Render("Verbs: get (GET) · set (POST) · update (PUT) · delete (DELETE)") + "\n\n")
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
		b.WriteString(fmt.Sprintf("  %s %s %s\n",
			promptStyle.Render(fmt.Sprintf("%-7s", h.Verb)),
			titleStyle.Render(fmt.Sprintf("%-22s", h.Resource)),
			dimStyle.Render(strings.Join(args, " "))))
	}
	return b.String()
}
