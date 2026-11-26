package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// shellExpansions returns each command substitution that a POSIX shell runs
// in the line: a "$(" or a backtick outside single quotes. Double quotes do
// not stop a substitution.
func shellExpansions(line string) []string {
	var found []string
	single := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case single:
			if c == '\'' {
				single = false
			}
		case c == '\\':
			i++
		case c == '\'':
			single = true
		case c == '`':
			found = append(found, line[i:])
		case c == '$' && i+1 < len(line) && line[i+1] == '(':
			found = append(found, line[i:])
		}
	}
	return found
}

func TestPanelTally_NextBlockShellExpansionDetector(t *testing.T) {
	for line, want := range map[string]bool{
		`mindspec complete "x$(touch y)"`:   true,
		"mindspec complete x`touch y`":      true,
		`mindspec complete '"x$(touch y)"'`: false,
		`mindspec complete 'x'"$(id)"`:      true,
		`mindspec complete mindspec-abc.1`:  false,
	} {
		if got := len(shellExpansions(line)) > 0; got != want {
			t.Errorf("shellExpansions(%q) found = %v, want %v", line, got, want)
		}
	}
}

// TestPanelTally_NextBlockQuotesBeadOperand: a bead id that the panel
// recorded raw prints quoted for the shell on the pass and halted routes. A
// valid id prints bare.
func TestPanelTally_NextBlockQuotesBeadOperand(t *testing.T) {
	routes := map[string]panel.Decision{
		"pass":   {Action: panel.Allow, Leg: panel.LegPass},
		"halted": {Action: panel.Block, Message: "blocked"},
	}
	for _, bead := range []string{"mindspec-x$(touch /tmp/pwn)", "mindspec-x`touch /tmp/pwn`"} {
		for name, d := range routes {
			f := tallyRecoveryFacts{Slug: "demo", SpecID: "900-demo", Target: "main", Gate: "bead", BeadID: bead, NextRound: 2}
			lines := renderPanelNextBlock(d, f)
			named := false
			for _, line := range lines {
				named = named || strings.Contains(line, "touch /tmp/pwn")
				if found := shellExpansions(line); len(found) > 0 {
					t.Errorf("%s route prints a shell expansion of bead %q: %q", name, bead, line)
				}
			}
			if !named {
				t.Errorf("%s route does not name the bead %q: %q", name, bead, lines)
			}
		}
	}
	lines := renderPanelNextBlock(routes["pass"], tallyRecoveryFacts{Slug: "demo", SpecID: "900-demo", Target: "main", Gate: "bead", BeadID: "mindspec-abc.1", NextRound: 2})
	if len(lines) != 1 || lines[0] != "mindspec complete mindspec-abc.1" {
		t.Fatalf("a valid bead id is not bare: %q", lines)
	}
}

// TestPanelTally_NextBlockFixedLines pins the operator-action and incomplete
// halt lines as the contract quotes them.
func TestPanelTally_NextBlockFixedLines(t *testing.T) {
	if panelOperatorActionLine != "A REJECT or missing required measurement artifact needs operator action. A text edit does not supply an artifact." {
		t.Errorf("operator-action line = %q", panelOperatorActionLine)
	}
	if panelIncompleteHaltLine != "If that tally still blocks: halt and report the panel state." {
		t.Errorf("incomplete halt line = %q", panelIncompleteHaltLine)
	}
	f := tallyRecoveryFacts{Slug: "demo", SpecID: "900-demo", Target: "main", Gate: "bead", BeadID: "mindspec-abc.1", NextRound: 2}
	operator := renderPanelNextBlock(panel.Decision{Action: panel.Block, Leg: panel.LegOperator}, f)
	if len(operator) == 0 || operator[0] != panelOperatorActionLine {
		t.Errorf("operator route does not start with the operator-action line: %q", operator)
	}
	f.MissingSlots = []string{"R2"}
	incomplete := renderPanelNextBlock(panel.Decision{Action: panel.Block, Leg: panel.LegIncomplete}, f)
	if !containsLine(incomplete, panelIncompleteHaltLine) {
		t.Errorf("incomplete route has no halt line: %q", incomplete)
	}
}

// TestPanelTally_NextBlockEscapesSlotsError: the incomplete route prints a
// slot-status error escaped for the terminal.
func TestPanelTally_NextBlockEscapesSlotsError(t *testing.T) {
	raw := "cannot read slot\x1b]0;owned\x07\nrecovery: rm -rf /"
	f := tallyRecoveryFacts{Slug: "demo", SpecID: "900-demo", Target: "main", Gate: "bead", BeadID: "mindspec-abc.1", NextRound: 2, SlotsErr: errors.New(raw)}
	lines := renderPanelNextBlock(panel.Decision{Action: panel.Block, Leg: panel.LegIncomplete}, f)
	want := "the slot status could not be read: " + termsafe.Escape(raw)
	if len(lines) == 0 || lines[0] != want {
		t.Fatalf("first line = %q, want %q", lines, want)
	}
	for _, line := range lines {
		if strings.ContainsAny(line, "\x1b\x07\n") {
			t.Fatalf("incomplete route prints a raw control byte: %q", line)
		}
	}
}

func containsLine(lines []string, want string) bool {
	for _, line := range lines {
		if line == want {
			return true
		}
	}
	return false
}
