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

func key(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }

func containsText(ss []Suggestion, text string) bool {
	for _, s := range ss {
		if s.Text == text {
			return true
		}
	}
	return false
}

// commandsLoadedMsg must flip the model to a usable state and emit the banner.
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

// Regression for the "Enter does nothing" report: Enter on a real command before
// permissions load must NOT panic and must clear the input (deferring the run).
func TestEnterBeforeLoadedIsSafe(t *testing.T) {
	m := newTestModel(t, "super_admin", false)
	m.input.SetValue("get schedules.list")
	m, cmd := step(t, m, key(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("expected a 'still loading' Println cmd")
	}
	if m.input.Value() != "" {
		t.Errorf("input should be cleared, got %q", m.input.Value())
	}
}

// Enter handling for the control words and a real command once loaded.
func TestEnterBehaviour(t *testing.T) {
	t.Run("empty does nothing", func(t *testing.T) {
		m := newTestModel(t, "super_admin", true)
		m.input.SetValue("   ")
		_, cmd := step(t, m, key(tea.KeyEnter))
		if cmd != nil {
			t.Fatal("empty Enter should yield no cmd")
		}
	})

	t.Run("exit quits", func(t *testing.T) {
		m := newTestModel(t, "super_admin", true)
		m.input.SetValue("exit")
		_, cmd := step(t, m, key(tea.KeyEnter))
		if cmd == nil {
			t.Fatal("exit produced no cmd")
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatal("exit did not produce QuitMsg")
		}
	})

	t.Run("command runs and clears input", func(t *testing.T) {
		m := newTestModel(t, "super_admin", true)
		m.input.SetValue("get schedules.list")
		m, cmd := step(t, m, key(tea.KeyEnter))
		if cmd == nil {
			t.Fatal("valid command produced no cmd (Enter would appear dead)")
		}
		if m.input.Value() != "" {
			t.Errorf("input not cleared after run: %q", m.input.Value())
		}
		// Input is cleared, so the dropdown resets to the fresh verb menu.
		if !containsText(m.suggest, "get") {
			t.Errorf("dropdown should reset to the verb menu after Enter, got %v", m.suggest)
		}
	})
}

// Tab cycles DOWN through the candidate list, inserting each value (kube-prompt
// style). Regression: Tab once filled the first value then jumped past the menu,
// so you could never reach the other choices.
func TestTabCyclesThroughChoices(t *testing.T) {
	m := newTestModel(t, "super_admin", true)
	m.input.SetValue("get market.latest market=")
	m.recomputeSuggest()
	if len(m.suggest) != 4 {
		t.Fatalf("expected 4 market choices, got %d: %v", len(m.suggest), m.suggest)
	}

	want := []string{
		"get market.latest market=gold",
		"get market.latest market=nasdaq",
		"get market.latest market=crypto",
		"get market.latest market=sp500",
	}
	for i, w := range want {
		m, _ = step(t, m, key(tea.KeyTab))
		if m.input.Value() != w {
			t.Fatalf("Tab #%d → %q, want %q", i+1, m.input.Value(), w)
		}
	}
	// Wraps back to the first.
	m, _ = step(t, m, key(tea.KeyTab))
	if m.input.Value() != want[0] {
		t.Fatalf("Tab wraparound → %q, want %q", m.input.Value(), want[0])
	}
	// Shift+Tab cycles backwards.
	m, _ = step(t, m, key(tea.KeyShiftTab))
	if m.input.Value() != want[len(want)-1] {
		t.Fatalf("Shift+Tab → %q, want %q", m.input.Value(), want[len(want)-1])
	}
}

// Regression: an unrelated message (e.g. the cursor-blink tick) arriving
// mid-cycle must NOT reset the completion cycle. Previously the default branch
// recomputed on every message, collapsing the frozen candidate list so Tab
// stayed stuck on the first choice.
type noopMsg struct{}

func TestCycleSurvivesSpuriousMessages(t *testing.T) {
	m := newTestModel(t, "super_admin", true)
	m.input.SetValue("get market.latest market=")
	m.recomputeSuggest()

	m, _ = step(t, m, key(tea.KeyTab)) // → gold
	m, _ = step(t, m, noopMsg{})       // blink-like tick
	m, _ = step(t, m, key(tea.KeyTab)) // must advance → nasdaq, not stay on gold
	if m.input.Value() != "get market.latest market=nasdaq" {
		t.Fatalf("cycle reset by spurious message: got %q", m.input.Value())
	}
	if !m.completing {
		t.Fatal("completion cycle should survive a no-op message")
	}
}

// Typing after a Tab cycle ends the cycle and resumes live filtering.
func TestTabThenTypeResumesLiveSuggest(t *testing.T) {
	m := newTestModel(t, "super_admin", true)
	m.input.SetValue("")
	m.recomputeSuggest()
	m, _ = step(t, m, key(tea.KeyTab)) // fills first verb
	if !m.completing {
		t.Fatal("should be in completion cycle after Tab")
	}
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if m.completing {
		t.Fatal("typing should end the completion cycle")
	}
}

// Arrow keys move the dropdown selection (with wraparound).
func TestDropdownNavigation(t *testing.T) {
	m := newTestModel(t, "super_admin", true)
	m.input.SetValue("")
	m.recomputeSuggest()
	n := len(m.suggest)
	if n < 2 {
		t.Fatalf("expected several verb suggestions, got %d", n)
	}
	// First Down selects index 0 (begins the cycle), the next advances to 1.
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

// Regression for the monochrome dropdown: a theme built from a color-capable
// renderer must emit ANSI escapes (the wish per-session renderer is what wires
// this up live; here we assert the theme itself produces color).
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
