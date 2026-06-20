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

// Tab completes the highlighted suggestion and advances to the next token.
func TestTabCompletion(t *testing.T) {
	m := newTestModel(t, "super_admin", true)

	// Partial verb → Tab completes to "get " (verb + trailing space).
	m.input.SetValue("ge")
	m.recomputeSuggest()
	m, _ = step(t, m, key(tea.KeyTab))
	if m.input.Value() != "get " {
		t.Fatalf("verb tab-complete: got %q, want %q", m.input.Value(), "get ")
	}

	// After the verb, Tab fills a resource (then a space).
	m, _ = step(t, m, key(tea.KeyTab))
	got := m.input.Value()
	if !strings.HasPrefix(got, "get ") || !strings.HasSuffix(got, " ") || strings.TrimSpace(got) == "get" {
		t.Fatalf("resource tab-complete produced %q", got)
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
	m, _ = step(t, m, key(tea.KeyDown))
	if m.sugIdx != 1 {
		t.Fatalf("Down → sugIdx %d, want 1", m.sugIdx)
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
