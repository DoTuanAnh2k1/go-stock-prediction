package shell

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Theme holds all styles for one SSH session. Styles MUST be built from the
// session's own lipgloss.Renderer (via wish/bubbletea MakeRenderer) — the
// default renderer detects color from the server's stdout (not a TTY) and would
// strip all ANSI, leaving a monochrome UI.
type Theme struct {
	Title  lipgloss.Style
	Prompt lipgloss.Style
	Dim    lipgloss.Style
	Err    lipgloss.Style
	Text   lipgloss.Style // input text (renderer-bound)
	Cursor lipgloss.Style // block cursor (renderer-bound so reverse isn't stripped)

	Header   lipgloss.Style
	HeaderBG lipgloss.Style
	Badge    lipgloss.Style

	SugText    lipgloss.Style
	SugDesc    lipgloss.Style
	SugSelText lipgloss.Style
	SugSelDesc lipgloss.Style
	SugScroll  lipgloss.Style
}

// NewTheme builds the palette from a session renderer. A nil renderer (tests)
// falls back to the default renderer.
func NewTheme(r *lipgloss.Renderer) *Theme {
	if r == nil {
		r = lipgloss.DefaultRenderer()
	}
	// If the client gave us no usable color profile, assume a 256-color xterm so
	// the UI still renders in color rather than falling back to plain text.
	if r.ColorProfile() == termenv.Ascii {
		r.SetColorProfile(termenv.ANSI256)
	}

	c := func(s string) lipgloss.Color { return lipgloss.Color(s) }
	return &Theme{
		Title:  r.NewStyle().Bold(true).Foreground(c("231")),
		Prompt: r.NewStyle().Bold(true).Foreground(c("212")),
		Dim:    r.NewStyle().Foreground(c("245")),
		Err:    r.NewStyle().Bold(true).Foreground(c("203")),
		Text:   r.NewStyle().Foreground(c("252")),
		Cursor: r.NewStyle().Foreground(c("212")), // Reverse(true) → bright block

		Header:   r.NewStyle().Bold(true).Foreground(c("231")).Background(c("25")).Padding(0, 1),
		HeaderBG: r.NewStyle().Background(c("25")),
		Badge:    r.NewStyle().Bold(true).Foreground(c("232")).Background(c("114")).Padding(0, 1),

		SugText:    r.NewStyle().Foreground(c("252")).Background(c("237")),
		SugDesc:    r.NewStyle().Foreground(c("245")).Background(c("237")),
		SugSelText: r.NewStyle().Bold(true).Foreground(c("231")).Background(c("31")),
		SugSelDesc: r.NewStyle().Foreground(c("195")).Background(c("31")),
		SugScroll:  r.NewStyle().Foreground(c("244")).Background(c("237")),
	}
}
