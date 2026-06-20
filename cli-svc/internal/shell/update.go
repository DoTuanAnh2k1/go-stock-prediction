package shell

import (
	"strings"

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
		return m, nil

	case resultMsg:
		m.history = append(m.history, msg.output)
		m.input.SetValue("")
		m.refreshViewport()
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyCtrlD:
			return m, tea.Quit
		case tea.KeyTab:
			m.completeInput()
			return m, nil
		case tea.KeyEnter:
			line := strings.TrimSpace(m.input.Value())
			if line == "" {
				return m, nil
			}
			if line == "exit" || line == "quit" {
				return m, tea.Quit
			}
			if line == "help" || line == "?" {
				m.history = append(m.history, m.helpText())
				m.input.SetValue("")
				m.refreshViewport()
				return m, nil
			}
			if line == "clear" {
				m.history = nil
				m.input.SetValue("")
				m.refreshViewport()
				return m, nil
			}
			if m.runner == nil {
				m.history = append(m.history, "still loading your permissions, please wait...")
				m.refreshViewport()
				return m, nil
			}
			m.history = append(m.history, "> "+line)
			return m, m.runCommand(line)
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

// completeInput applies tab completion to the current input.
func (m *Model) completeInput() {
	if m.comp == nil {
		return
	}
	line := m.input.Value()
	suggestions := m.comp.Suggest(line)
	if len(suggestions) == 0 {
		return
	}
	if len(suggestions) == 1 {
		m.applyCompletion(line, suggestions[0])
		return
	}
	// Multiple: complete the longest common prefix and list options.
	lcp := longestCommonPrefix(suggestions)
	if lcp != "" {
		m.applyCompletion(line, lcp)
	}
	m.history = append(m.history, "candidates: "+strings.Join(suggestions, "  "))
	m.refreshViewport()
}

// applyCompletion replaces the final token of the line with the completed token.
func (m *Model) applyCompletion(line, completed string) {
	endsWithSpace := strings.HasSuffix(line, " ")
	if endsWithSpace {
		m.input.SetValue(line + completed)
	} else {
		idx := strings.LastIndex(line, " ")
		if idx < 0 {
			m.input.SetValue(completed)
		} else {
			m.input.SetValue(line[:idx+1] + completed)
		}
	}
	m.input.CursorEnd()
}

func longestCommonPrefix(items []string) string {
	if len(items) == 0 {
		return ""
	}
	prefix := items[0]
	for _, s := range items[1:] {
		for !strings.HasPrefix(s, prefix) {
			prefix = prefix[:len(prefix)-1]
			if prefix == "" {
				return ""
			}
		}
	}
	return prefix
}
