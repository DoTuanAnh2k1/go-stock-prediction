package shell

import (
	"strings"
	"testing"
)

func TestSplitPipe(t *testing.T) {
	cmd, grep := splitPipe("get market latest market gold | grep -A 2 BTC")
	if cmd != "get market latest market gold" || grep != "grep -A 2 BTC" {
		t.Errorf("cmd=%q grep=%q", cmd, grep)
	}
	// A '|' inside the quoted pattern must NOT be treated as the pipe.
	cmd, grep = splitPipe(`get session list | grep "A|B"`)
	if cmd != "get session list" || grep != `grep "A|B"` {
		t.Errorf("quoted-pipe split: cmd=%q grep=%q", cmd, grep)
	}
	if c, g := splitPipe("get schedules list"); c != "get schedules list" || g != "" {
		t.Errorf("no-pipe split: c=%q g=%q", c, g)
	}
}

func TestParseGrep(t *testing.T) {
	g, ok := parseGrep(`grep -i -A 2 -B 1 "hello world"`)
	if !ok {
		t.Fatal("parse failed")
	}
	if g.pattern != "hello world" || !g.ignoreCase || g.after != 2 || g.before != 1 {
		t.Errorf("got %+v", g)
	}
	if g2, _ := parseGrep(`grep "A|B"`); g2.pattern != "A|B" {
		t.Errorf("quoted alternation pattern=%q", g2.pattern)
	}
	if _, ok := parseGrep("notgrep x"); ok {
		t.Error("should reject non-grep spec")
	}
}

func TestApplyGrep(t *testing.T) {
	out := "alpha one\nBETA two\ngamma three\nBETA-2 four\ndelta GOLD"

	// quoted phrase with a space
	if got := applyGrep(out, `grep "BETA two"`); got != "BETA two" {
		t.Errorf("phrase: %q", got)
	}
	// regex alternation
	got := applyGrep(out, `grep "alpha|GOLD"`)
	if got != "alpha one\ndelta GOLD" {
		t.Errorf("alternation: %q", got)
	}
	// case-insensitive regex
	if got := applyGrep(out, `grep -i "beta"`); got != "BETA two\nBETA-2 four" {
		t.Errorf("ci: %q", got)
	}
	// context -A 1
	got = applyGrep(out, "grep -A 1 BETA")
	if !strings.Contains(got, "BETA two\ngamma three") || !strings.Contains(got, "BETA-2 four\ndelta GOLD") {
		t.Errorf("context: %q", got)
	}
	// no match
	if got := applyGrep(out, "grep zzz"); !strings.Contains(got, "no match") {
		t.Errorf("nomatch: %q", got)
	}
}
