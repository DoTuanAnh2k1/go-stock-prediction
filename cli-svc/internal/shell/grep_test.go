package shell

import (
	"strings"
	"testing"
)

func TestSplitPipe(t *testing.T) {
	cmd, grep := splitPipe("get market latest market gold | grep -A 2 BTC")
	if cmd != "get market latest market gold" {
		t.Errorf("cmd=%q", cmd)
	}
	if grep != "grep -A 2 BTC" {
		t.Errorf("grep=%q", grep)
	}
	if c, g := splitPipe("get schedules list"); c != "get schedules list" || g != "" {
		t.Errorf("no-pipe split: c=%q g=%q", c, g)
	}
}

func TestParseGrep(t *testing.T) {
	g, ok := parseGrep("grep -i -A 2 -B 1 hello world")
	if !ok {
		t.Fatal("parse failed")
	}
	if g.pattern != "hello world" || !g.ignoreCase || g.after != 2 || g.before != 1 {
		t.Errorf("got %+v", g)
	}
	if _, ok := parseGrep("notgrep x"); ok {
		t.Error("should reject non-grep spec")
	}
}

func TestApplyGrep(t *testing.T) {
	out := "alpha\nBETA\ngamma\nBETA-2\ndelta"

	// simple match
	if got := applyGrep(out, "grep BETA"); got != "BETA\nBETA-2" {
		t.Errorf("simple: %q", got)
	}
	// case-insensitive
	if got := applyGrep(out, "grep -i beta"); got != "BETA\nBETA-2" {
		t.Errorf("ci: %q", got)
	}
	// context -A 1 around the first BETA (line index 1) → BETA, gamma; group sep then BETA-2, delta
	got := applyGrep(out, "grep -A 1 BETA")
	if !strings.Contains(got, "BETA\ngamma") || !strings.Contains(got, "BETA-2\ndelta") {
		t.Errorf("context: %q", got)
	}
	// no match
	if got := applyGrep(out, "grep zzz"); !strings.Contains(got, "no match") {
		t.Errorf("nomatch: %q", got)
	}
}
