package shell

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

const maxSuggestRows = 8

// View renders the live, bottom-pinned block: the prompt input, the completion
// dropdown directly beneath the cursor token, and a hint/error line. Command
// output is NOT rendered here — it is pushed to the scrollback above via
// tea.Println (see update.go).
func (m Model) View() string {
	var b strings.Builder
	b.WriteString(m.input.View())
	if d := m.renderSuggest(); d != "" {
		b.WriteString("\n")
		b.WriteString(d)
	}
	b.WriteString("\n")
	switch {
	case m.err != "":
		b.WriteString(m.th.Err.Render("✗ " + m.err))
	case !m.loaded:
		b.WriteString(m.th.Dim.Render("connecting to API…"))
	default:
		b.WriteString(m.th.Dim.Render(m.hintLine()))
	}
	return b.String()
}

// banner is printed once (above the prompt) when the session is ready.
func (m Model) banner() string {
	return m.headerBar() + "\n" + m.welcome()
}

// headerBar is a full-width status bar.
func (m Model) headerBar() string {
	w := m.width
	if w <= 0 {
		w = 80
	}
	left := m.th.Header.Render("stock-prediction CLI")
	badge := m.th.Badge.Render(m.sess.Username + " · " + roleLabel(m.sess.Role))
	gap := w - lipgloss.Width(left) - lipgloss.Width(badge)
	if gap < 1 {
		gap = 1
	}
	return left + m.th.HeaderBG.Render(strings.Repeat(" ", gap)) + badge
}

func (m Model) hintLine() string {
	return "Tab suggest · ↑↓ history · Enter run · cmd | grep -A 2 X · help · clear · exit"
}

// renderSuggest draws the floating completion dropdown. Returns "" when there is
// nothing to suggest.
func (m Model) renderSuggest() string {
	if !m.showSuggest || len(m.suggest) == 0 {
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
	w := m.width
	if w <= 0 {
		w = 80
	}
	avail := w - m.sugCol - 1
	if avail < 16 {
		avail = 16
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
	b.WriteString(m.th.Dim.Render("Type a command (get/set/update/delete). Press Tab for suggestions, ↑/↓ for history, Enter to run."))
	return b.String()
}

func (m Model) helpText() string {
	var b strings.Builder
	b.WriteString(m.th.Title.Render("Command reference") + "\n")
	b.WriteString(m.th.Dim.Render("Syntax: <verb> <category> <name> [arg value ...]   e.g.  get market latest market gold") + "\n")
	b.WriteString(m.th.Dim.Render("Verbs: get (GET) · set (POST) · update (PUT) · delete (DELETE). Quote values with spaces.") + "\n")
	for _, h := range m.reg.All() {
		if !m.allowed.Allows(h.Key) {
			continue
		}
		cat, name := split(h.Key)
		args := make([]string, 0, len(h.ArgSchema))
		for _, a := range h.ArgSchema {
			tok := a.Name + " "
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
			m.th.Title.Render(fmt.Sprintf("%-9s %-12s", cat, name)),
			m.th.Dim.Render(strings.Join(args, " "))))
	}
	return strings.TrimRight(b.String(), "\n")
}
