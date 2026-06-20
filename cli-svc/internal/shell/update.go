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

		case tea.KeyTab, tea.KeyDown, tea.KeyCtrlN:
			m.cycle(+1)
			return m, nil

		case tea.KeyShiftTab, tea.KeyUp, tea.KeyCtrlP:
			m.cycle(-1)
			return m, nil

		case tea.KeyEsc:
			// Hide the dropdown so the next Enter runs the command.
			m.suggest = nil
			m.completing = false
			m.dismissed = true
			return m, nil

		case tea.KeyEnter:
			line := strings.TrimSpace(m.input.Value())
			if line == "" {
				return m, nil
			}
			// If the dropdown is open, Enter PICKS the highlighted suggestion and
			// advances to the next token — it does not run the command. The user
			// presses Esc (or completes the line) and Enter again to run.
			if !m.dismissed && len(m.suggest) > 0 {
				m.commitSuggest()
				return m, nil
			}

			m.suggest = nil
			m.input.SetValue("")
			m.err = ""
			m.dismissed = false
			if line == "exit" || line == "quit" {
				return m, tea.Quit
			}
			if line == "clear" {
				return m, tea.ClearScreen
			}
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

	// Default: forward to the text input. Only refresh the dropdown when the
	// input VALUE actually changed — otherwise spurious messages (notably the
	// cursor-blink tick) would reset an in-progress Tab completion cycle.
	old := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != old {
		m.recomputeSuggest()
	}
	return m, cmd
}

// commitSuggest inserts the highlighted suggestion, adds a trailing space and
// advances to the next token's dropdown — this is what Enter does while the
// dropdown is open (pick, don't run).
func (m *Model) commitSuggest() {
	if len(m.suggest) == 0 {
		return
	}
	s := m.suggest[m.sugIdx]
	m.input.SetValue(tokenBase(m.input.Value()) + s.Text + " ")
	m.input.CursorEnd()
	m.completing = false
	m.recomputeSuggest()
}

// recomputeSuggest refreshes the live dropdown for the current input value. It
// ends any in-progress completion cycle (the user typed/edited the line).
func (m *Model) recomputeSuggest() {
	m.completing = false
	m.dismissed = false
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
// highlighted candidate into the input — kube-prompt style. The candidate list
// is frozen for the duration of the cycle so repeated Tab keeps walking the
// full list instead of collapsing to the just-inserted value. The user types a
// space (or any character) to end the cycle and advance to the next token.
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
