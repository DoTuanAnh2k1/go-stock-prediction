package shell

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

const maxSuggestRows = 8

func (m *Model) resizeViewport() {
	w := m.width
	h := m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	vpHeight := h - 4
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

// View renders the full screen, kube-prompt style: header, the input line, the
// live completion dropdown directly under it, then the scrolling output, then a
// hint/error line at the very bottom.
func (m Model) View() string {
	if !m.ready {
		return "initializing...\n"
	}

	drop := m.renderSuggest()
	dropLines := 0
	if drop != "" {
		dropLines = strings.Count(drop, "\n") + 1
	}

	vpH := m.height - 3 - dropLines
	if vpH < 1 {
		vpH = 1
	}
	vp := m.viewport
	vp.Height = vpH
	vp.GotoBottom()

	var b strings.Builder
	b.WriteString(m.headerBar())
	b.WriteString("\n")
	b.WriteString(m.input.View())
	b.WriteString("\n")
	if drop != "" {
		b.WriteString(drop)
		b.WriteString("\n")
	}
	b.WriteString(vp.View())
	b.WriteString("\n")
	if m.err != "" {
		b.WriteString(m.th.Err.Render("✗ " + m.err))
	} else {
		b.WriteString(m.th.Dim.Render(m.hintLine()))
	}
	return b.String()
}

// headerBar is the top status bar spanning the terminal width.
func (m Model) headerBar() string {
	left := m.th.Header.Render("stock-prediction CLI")
	badge := m.th.Badge.Render(m.sess.Username + " · " + roleLabel(m.sess.Role))
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(badge)
	if gap < 1 {
		gap = 1
	}
	return left + m.th.HeaderBG.Render(strings.Repeat(" ", gap)) + badge
}

func (m Model) hintLine() string {
	return "↑/↓ navigate · Tab complete · Enter run · help · clear · exit"
}

// renderSuggest draws the floating completion dropdown. Returns "" when there is
// nothing to suggest (so the layout collapses cleanly).
func (m Model) renderSuggest() string {
	if len(m.suggest) == 0 {
		return ""
	}

	maxText, maxDesc := 0, 0
	for _, s := range m.suggest {
		if n := utf8.RuneCountInString(s.Text); n > maxText {
			maxText = n
		}
		if n := utf8.RuneCountInString(s.Desc); n > maxDesc {
			maxDesc = n
		}
	}
	avail := m.width - m.sugCol - 1
	if avail < 12 {
		avail = 12
	}
	if maxText+maxDesc+4 > avail {
		maxDesc = avail - maxText - 4
		if maxDesc < 0 {
			maxDesc = 0
		}
	}

	start := 0
	if len(m.suggest) > maxSuggestRows && m.sugIdx >= maxSuggestRows {
		start = m.sugIdx - maxSuggestRows + 1
	}
	end := start + maxSuggestRows
	if end > len(m.suggest) {
		end = len(m.suggest)
	}

	indent := strings.Repeat(" ", m.sugCol)
	var lines []string
	for i := start; i < end; i++ {
		s := m.suggest[i]
		marker := " "
		if i == start && start > 0 {
			marker = "▲"
		} else if i == end-1 && end < len(m.suggest) {
			marker = "▼"
		}
		textStyle, descStyle := m.th.SugText, m.th.SugDesc
		if i == m.sugIdx {
			textStyle, descStyle = m.th.SugSelText, m.th.SugSelDesc
		}
		text := textStyle.Render(" " + ljust(s.Text, maxText) + "  ")
		desc := descStyle.Render(ljust(s.Desc, maxDesc) + " ")
		scroll := m.th.SugScroll.Render(marker + " ")
		lines = append(lines, indent+text+desc+scroll)
	}
	return strings.Join(lines, "\n")
}

func ljust(s string, n int) string {
	if n < 0 {
		n = 0
	}
	r := []rune(s)
	if len(r) > n {
		if n <= 1 {
			return string(r[:n])
		}
		return string(r[:n-1]) + "…"
	}
	return s + strings.Repeat(" ", n-len(r))
}

func roleLabel(role string) string {
	if role == "" {
		return "user"
	}
	return role
}

func (m Model) welcome() string {
	var b strings.Builder
	b.WriteString(m.th.Title.Render("Welcome, "+m.sess.Username) + "\n")
	if m.allowed.IsSuperAdmin() {
		b.WriteString(m.th.Dim.Render("super_admin — you may run any command.") + "\n")
	} else {
		n := len(m.allowed.Commands())
		b.WriteString(m.th.Dim.Render(fmt.Sprintf("%s — %d command(s) granted.", roleLabel(m.sess.Role), n)) + "\n")
	}
	b.WriteString(m.th.Dim.Render("Start typing a verb (get/set/update/delete); suggestions appear below the prompt."))
	return b.String()
}

func (m Model) helpText() string {
	var b strings.Builder
	b.WriteString(m.th.Title.Render("Command reference") + "\n")
	b.WriteString(m.th.Dim.Render("Syntax: <verb> <resource> [key=value ...]") + "\n")
	b.WriteString(m.th.Dim.Render("Verbs: get (GET) · set (POST) · update (PUT) · delete (DELETE)") + "\n\n")
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
			m.th.Prompt.Render(fmt.Sprintf("%-7s", h.Verb)),
			m.th.Title.Render(fmt.Sprintf("%-22s", h.Resource)),
			m.th.Dim.Render(strings.Join(args, " "))))
	}
	return b.String()
}
