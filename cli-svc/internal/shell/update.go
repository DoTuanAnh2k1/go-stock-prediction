package shell

import (
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

// Update handles all messages for the session model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeViewport()
		m.refreshViewport()
		return m, nil

	case commandsLoadedMsg:
		if msg.err != nil {
			m.err = "failed to load your commands: " + msg.err.Error()
			m.allowed = NewAllowedSet(m.sess.Role, nil)
		} else {
			m.allowed = NewAllowedSet(m.sess.Role, msg.cmds)
		}
		m.runner = NewRunner(m.reg, m.client, m.allowed, m.sess.JWT)
		m.comp = NewCompleter(m.reg, m.allowed)
		m.history = append(m.history, m.welcome())
		m.refreshViewport()
		m.recomputeSuggest()
		return m, nil

	case resultMsg:
		m.history = append(m.history, msg.output)
		m.input.SetValue("")
		m.err = ""
		m.refreshViewport()
		m.recomputeSuggest()
		return m, nil

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
			if line == "" {
				return m, nil
			}
			if line == "exit" || line == "quit" {
				return m, tea.Quit
			}
			if line == "help" || line == "?" {
				m.history = append(m.history, m.helpText())
				m.input.SetValue("")
				m.err = ""
				m.refreshViewport()
				m.recomputeSuggest()
				return m, nil
			}
			if line == "clear" {
				m.history = nil
				m.input.SetValue("")
				m.err = ""
				m.refreshViewport()
				m.recomputeSuggest()
				return m, nil
			}
			if m.runner == nil {
				m.history = append(m.history, dimStyle.Render("still loading your permissions, please wait..."))
				m.refreshViewport()
				return m, nil
			}
			m.history = append(m.history, promptStyle.Render("> ")+line)
			return m, m.runCommand(line)
		}
	}

	// Default: forward to the text input (and viewport for scroll keys), then
	// recompute the live dropdown against the new input value.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	m.recomputeSuggest()
	return m, tea.Batch(cmds...)
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
