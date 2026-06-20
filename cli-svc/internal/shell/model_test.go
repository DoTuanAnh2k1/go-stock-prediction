package shell

import (
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"go-stock-prediction/cli-svc/internal/handlers"
)

// newTestModel builds a Model with a stub client and (optionally) a loaded
// permission set, mirroring what the bubbletea program would hold mid-session.
func newTestModel(t *testing.T, role string, loaded bool) Model {
	t.Helper()
	m := New(Session{Username: "u", Role: role}, &stubClient{}, handlers.NewRegistry(), nil)
	m.width = 120
	if loaded {
		mm, _ := m.Update(commandsLoadedMsg{cmds: nil})
		m = mm.(Model)
	}
	return m
}

func step(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	mm, cmd := m.Update(msg)
	return mm.(Model), cmd
}

// setInput sets the input and refreshes suggestions, as live typing would.
func setInput(m Model, s string) Model {
	m.input.SetValue(s)
	m.recomputeSuggest()
	return m
}

func key(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }

func containsText(ss []Suggestion, text string) bool {
	for _, s := range ss {
		if s.Text == text {
			return true
		}
	}
	return false
}

func TestCommandsLoadedReady(t *testing.T) {
	m := newTestModel(t, "super_admin", false)
	if m.loaded || m.runner != nil || m.comp != nil {
		t.Fatal("model should not be ready before load")
	}
	m, cmd := step(t, m, commandsLoadedMsg{cmds: nil})
	if !m.loaded || m.runner == nil || m.comp == nil {
		t.Fatal("model not ready after commandsLoadedMsg")
	}
	if cmd == nil {
		t.Fatal("expected a banner Println cmd on load")
	}
}

func TestEnterBeforeLoadedIsSafe(t *testing.T) {
	m := newTestModel(t, "super_admin", false)
	m.input.SetValue("get schedules list")
	m, cmd := step(t, m, key(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("expected a 'still loading' Println cmd")
	}
	if m.input.Value() != "" {
		t.Errorf("input should be cleared, got %q", m.input.Value())
	}
}

// The core report: with the dropdown open, Enter PICKS the highlighted item and
// advances — it does NOT execute (which previously ran an incomplete command and
// errored).
func TestEnterPicksNotRuns(t *testing.T) {
	m := newTestModel(t, "super_admin", true)
	m = setInput(m, "get market prices")
	m, cmd := step(t, m, key(tea.KeyEnter))
	if cmd != nil {
		t.Fatal("Enter with open dropdown should pick, not run (no cmd)")
	}
	if m.input.Value() != "get market prices " {
		t.Fatalf("Enter should commit the highlighted name + space, got %q", m.input.Value())
	}
}

func TestEnterBehaviour(t *testing.T) {
	t.Run("empty does nothing", func(t *testing.T) {
		m := newTestModel(t, "super_admin", true)
		m = setInput(m, "   ")
		_, cmd := step(t, m, key(tea.KeyEnter))
		if cmd != nil {
			t.Fatal("empty Enter should yield no cmd")
		}
	})

	t.Run("exit quits", func(t *testing.T) {
		m := newTestModel(t, "super_admin", true)
		m = setInput(m, "exit") // no verb matches → dropdown empty → Enter runs
		_, cmd := step(t, m, key(tea.KeyEnter))
		if cmd == nil {
			t.Fatal("exit produced no cmd")
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatal("exit did not produce QuitMsg")
		}
	})

	t.Run("Esc then Enter runs and clears input", func(t *testing.T) {
		m := newTestModel(t, "super_admin", true)
		m = setInput(m, "get schedules list")
		m, _ = step(t, m, key(tea.KeyEsc)) // close dropdown
		if !m.dismissed {
			t.Fatal("Esc should dismiss the dropdown")
		}
		m, cmd := step(t, m, key(tea.KeyEnter))
		if cmd == nil {
			t.Fatal("Esc+Enter should run the command")
		}
		if m.input.Value() != "" {
			t.Errorf("input not cleared after run: %q", m.input.Value())
		}
	})
}

// Tab cycles DOWN through the candidate list, inserting each value.
func TestTabCyclesThroughChoices(t *testing.T) {
	m := newTestModel(t, "super_admin", true)
	m = setInput(m, "get market latest market ")
	if len(m.suggest) != 4 {
		t.Fatalf("expected 4 market choices, got %d: %v", len(m.suggest), m.suggest)
	}
	want := []string{
		"get market latest market gold",
		"get market latest market nasdaq",
		"get market latest market crypto",
		"get market latest market sp500",
	}
	for i, w := range want {
		m, _ = step(t, m, key(tea.KeyTab))
		if m.input.Value() != w {
			t.Fatalf("Tab #%d → %q, want %q", i+1, m.input.Value(), w)
		}
	}
	m, _ = step(t, m, key(tea.KeyTab)) // wrap
	if m.input.Value() != want[0] {
		t.Fatalf("Tab wraparound → %q, want %q", m.input.Value(), want[0])
	}
	m, _ = step(t, m, key(tea.KeyShiftTab)) // back
	if m.input.Value() != want[len(want)-1] {
		t.Fatalf("Shift+Tab → %q, want %q", m.input.Value(), want[len(want)-1])
	}
}

// Regression: a spurious message (cursor-blink tick) mid-cycle must not reset it.
type noopMsg struct{}

func TestCycleSurvivesSpuriousMessages(t *testing.T) {
	m := newTestModel(t, "super_admin", true)
	m = setInput(m, "get market latest market ")
	m, _ = step(t, m, key(tea.KeyTab)) // gold
	m, _ = step(t, m, noopMsg{})       // blink-like tick
	m, _ = step(t, m, key(tea.KeyTab)) // must advance → nasdaq
	if m.input.Value() != "get market latest market nasdaq" {
		t.Fatalf("cycle reset by spurious message: got %q", m.input.Value())
	}
}

func TestDropdownNavigation(t *testing.T) {
	m := newTestModel(t, "super_admin", true)
	m = setInput(m, "")
	n := len(m.suggest)
	if n < 2 {
		t.Fatalf("expected several verb suggestions, got %d", n)
	}
	m, _ = step(t, m, key(tea.KeyDown))
	m, _ = step(t, m, key(tea.KeyDown))
	if m.sugIdx != 1 {
		t.Fatalf("two Downs → sugIdx %d, want 1", m.sugIdx)
	}
	m, _ = step(t, m, key(tea.KeyUp))
	m, _ = step(t, m, key(tea.KeyUp))
	if m.sugIdx != n-1 {
		t.Fatalf("Up wraparound → sugIdx %d, want %d", m.sugIdx, n-1)
	}
}

func TestThemeEmitsColor(t *testing.T) {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.ANSI256)
	th := NewTheme(r)
	if out := th.SugSelText.Render("x"); !strings.Contains(out, "\x1b[") {
		t.Fatalf("selected-row style emitted no ANSI: %q", out)
	}
	if out := th.Prompt.Render(">"); !strings.Contains(out, "\x1b[") {
		t.Fatalf("prompt style emitted no ANSI: %q", out)
	}
}
