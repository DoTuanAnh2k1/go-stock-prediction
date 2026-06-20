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
//
// Interaction model:
//   - Dropdown is hidden until Tab is pressed.
//   - Dropdown OPEN:  Tab/↓ next · Shift+Tab/↑ prev · Enter pick+advance · Esc close
//   - Dropdown CLOSED: ↑/↓ recall command history · Enter run
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
		return m, tea.Println(m.banner())

	case resultMsg:
		return m, tea.Println(msg.output)

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyCtrlD:
			return m, tea.Quit

		case tea.KeyTab:
			// Open the dropdown on first Tab, then cycle.
			if !m.showSuggest {
				m.recomputeSuggest()
				if len(m.suggest) == 0 {
					return m, nil
				}
				m.showSuggest = true
			}
			m.cycle(+1)
			return m, nil

		case tea.KeyShiftTab:
			if m.showSuggest {
				m.cycle(-1)
			}
			return m, nil

		case tea.KeyDown, tea.KeyCtrlN:
			if m.showSuggest {
				m.cycle(+1)
			} else {
				m.historyNext()
			}
			return m, nil

		case tea.KeyUp, tea.KeyCtrlP:
			if m.showSuggest {
				m.cycle(-1)
			} else {
				m.historyPrev()
			}
			return m, nil

		case tea.KeyEsc:
			m.showSuggest = false
			m.completing = false
			m.suggest = nil
			return m, nil

		case tea.KeyEnter:
			line := strings.TrimSpace(m.input.Value())
			if line == "" {
				return m, nil
			}
			// Dropdown open → Enter picks the highlighted item and advances; it
			// does NOT run. Dropdown closed → Enter runs.
			if m.showSuggest && len(m.suggest) > 0 {
				m.commitSuggest()
				return m, nil
			}

			m.suggest = nil
			m.showSuggest = false
			m.completing = false
			m.input.SetValue("")
			m.err = ""
			m.pushHistory(line)
			if line == "exit" || line == "quit" {
				return m, tea.Quit
			}
			if line == "clear" {
				return m, tea.ClearScreen
			}
			if !m.loaded || m.runner == nil {
				return m, tea.Println(m.th.Dim.Render("still loading your permissions, please wait…"))
			}
			echo := tea.Println(m.th.Prompt.Render("> ") + line)
			if line == "help" || line == "?" {
				return m, tea.Sequence(echo, tea.Println(m.helpText()))
			}
			return m, tea.Sequence(echo, m.runCommand(line))
		}
	}

	// Default: forward to the text input. When the input VALUE changes (the user
	// typed), hide the dropdown (only Tab reopens it) and reset the history
	// cursor. Spurious messages (blink ticks) leave the value unchanged.
	old := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != old {
		m.showSuggest = false
		m.completing = false
		m.histIdx = len(m.cmdHistory)
	}
	return m, cmd
}

// commitSuggest inserts the highlighted suggestion + a trailing space and hides
// the dropdown (the user presses Tab again for the next token). This is Enter's
// behaviour while the dropdown is open.
func (m *Model) commitSuggest() {
	if len(m.suggest) == 0 {
		return
	}
	s := m.suggest[m.sugIdx]
	m.input.SetValue(tokenBase(m.input.Value()) + s.Text + " ")
	m.input.CursorEnd()
	m.completing = false
	m.showSuggest = false
	m.suggest = nil
}

// recomputeSuggest populates the candidate list for the current token. Called
// when the dropdown is opened with Tab.
func (m *Model) recomputeSuggest() {
	m.completing = false
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

// cycle moves through the suggestion list (dir +1 down, -1 up), inserting the
// highlighted candidate. The candidate list is frozen for the cycle so repeated
// Tab walks the full list instead of collapsing to the just-inserted value.
func (m *Model) cycle(dir int) {
	if !m.completing {
		if len(m.suggest) == 0 {
			return
		}
		m.completing = true
		m.compBase = tokenBase(m.input.Value())
		m.compList = m.suggest
		if dir >= 0 {
			m.sugIdx = 0
		} else {
			m.sugIdx = len(m.compList) - 1
		}
	} else {
		if len(m.compList) == 0 {
			return
		}
		m.sugIdx = (m.sugIdx + dir + len(m.compList)) % len(m.compList)
	}
	m.suggest = m.compList
	m.input.SetValue(m.compBase + m.compList[m.sugIdx].Text)
	m.input.CursorEnd()
}

// pushHistory records a run command and resets the recall cursor to the end.
func (m *Model) pushHistory(line string) {
	if n := len(m.cmdHistory); n == 0 || m.cmdHistory[n-1] != line {
		m.cmdHistory = append(m.cmdHistory, line)
	}
	m.histIdx = len(m.cmdHistory)
}

// historyPrev recalls an older command (Up).
func (m *Model) historyPrev() {
	if len(m.cmdHistory) == 0 {
		return
	}
	if m.histIdx > 0 {
		m.histIdx--
	}
	m.input.SetValue(m.cmdHistory[m.histIdx])
	m.input.CursorEnd()
}

// historyNext recalls a newer command (Down); past the newest clears the input.
func (m *Model) historyNext() {
	if len(m.cmdHistory) == 0 {
		return
	}
	if m.histIdx < len(m.cmdHistory)-1 {
		m.histIdx++
		m.input.SetValue(m.cmdHistory[m.histIdx])
	} else {
		m.histIdx = len(m.cmdHistory)
		m.input.SetValue("")
	}
	m.input.CursorEnd()
}

// tokenBase returns the input text up to (not including) the current token.
func tokenBase(line string) string {
	if line == "" || strings.HasSuffix(line, " ") {
		return line
	}
	if idx := strings.LastIndex(line, " "); idx >= 0 {
		return line[:idx+1]
	}
	return ""
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
