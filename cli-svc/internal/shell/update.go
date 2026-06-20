package shell

import (
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

// Update handles all messages for the session model.
//
// The shell runs INLINE (no alt-screen): the live View (prompt + dropdown) is
// pinned at the bottom, and output is pushed into the scrollback above via
// tea.Println — the familiar shell / kube-prompt feel.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = m.width - 4
		return m, nil

	case commandsLoadedMsg:
		if msg.err != nil {
			m.err = "failed to load your commands: " + msg.err.Error()
			m.allowed = NewAllowedSet(m.sess.Role, nil)
		} else {
			m.allowed = NewAllowedSet(m.sess.Role, msg.cmds)
		}
		m.loaded = true
		m.runner = NewRunner(m.reg, m.client, m.allowed, m.sess.JWT)
		m.comp = NewCompleter(m.reg, m.allowed)
		m.recomputeSuggest()
		return m, tea.Println(m.banner())

	case resultMsg:
		// Output already scrolled above via Println below; nothing to do but
		// keep the prompt responsive.
		return m, tea.Println(msg.output)

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyCtrlD:
			return m, tea.Quit

		case tea.KeyTab:
			m.acceptSuggest()
			return m, nil

		case tea.KeyDown, tea.KeyCtrlN:
			if n := len(m.suggest); n > 0 {
				m.sugIdx = (m.sugIdx + 1) % n
			}
			return m, nil

		case tea.KeyUp, tea.KeyCtrlP:
			if n := len(m.suggest); n > 0 {
				m.sugIdx = (m.sugIdx - 1 + n) % n
			}
			return m, nil

		case tea.KeyEsc:
			m.suggest = nil
			return m, nil

		case tea.KeyEnter:
			line := strings.TrimSpace(m.input.Value())
			m.suggest = nil
			m.input.SetValue("")
			m.err = ""
			if line == "" {
				return m, nil
			}
			if line == "exit" || line == "quit" {
				return m, tea.Quit
			}
			if line == "clear" {
				return m, tea.ClearScreen
			}
			// Everything below needs the loaded permission set.
			if !m.loaded || m.runner == nil {
				m.recomputeSuggest()
				return m, tea.Println(m.th.Dim.Render("still loading your permissions, please wait…"))
			}
			echo := tea.Println(m.th.Prompt.Render("> ") + line)
			m.recomputeSuggest()
			if line == "help" || line == "?" {
				return m, tea.Sequence(echo, tea.Println(m.helpText()))
			}
			return m, tea.Sequence(echo, m.runCommand(line))
		}
	}

	// Default: forward to the text input, then refresh the live dropdown.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.recomputeSuggest()
	return m, cmd
}

// recomputeSuggest refreshes the live dropdown for the current input value.
func (m *Model) recomputeSuggest() {
	if m.comp == nil {
		m.suggest = nil
		return
	}
	line := m.input.Value()
	m.suggest = m.comp.SuggestRich(line)
	if m.sugIdx >= len(m.suggest) {
		m.sugIdx = 0
	}
	m.sugCol = utf8.RuneCountInString(m.input.Prompt) + tokenStartCol(line)
}

// acceptSuggest inserts the highlighted suggestion, replacing the current token.
// Whole tokens (verb/resource/name=value) get a trailing space to advance to the
// next stage; a bare "name=" does not (the user types the value next).
func (m *Model) acceptSuggest() {
	if len(m.suggest) == 0 {
		return
	}
	s := m.suggest[m.sugIdx]
	line := m.input.Value()

	var base string
	switch {
	case line == "" || strings.HasSuffix(line, " "):
		base = line
	default:
		if idx := strings.LastIndex(line, " "); idx >= 0 {
			base = line[:idx+1]
		}
	}
	newLine := base + s.Text
	if !strings.HasSuffix(s.Text, "=") {
		newLine += " "
	}
	m.input.SetValue(newLine)
	m.input.CursorEnd()
	m.sugIdx = 0
	m.recomputeSuggest()
}

// tokenStartCol returns the rune column at which the current (last) token begins.
func tokenStartCol(line string) int {
	if line == "" || strings.HasSuffix(line, " ") {
		return utf8.RuneCountInString(line)
	}
	idx := strings.LastIndex(line, " ")
	if idx < 0 {
		return 0
	}
	return utf8.RuneCountInString(line[:idx+1])
}
