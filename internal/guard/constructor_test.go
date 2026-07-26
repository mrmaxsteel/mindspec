package guard

import (
	"os/exec"
	"strings"
	"testing"
)

func TestNewDestructiveCommand_ValidFloorMatch(t *testing.T) {
	d := NewDestructiveCommand("git branch -D bead/x", DestructionSuperseded)
	if !d.Valid() {
		t.Fatal("expected Valid() to be true")
	}
	if got := d.String(); got != "git branch -D bead/x" {
		t.Fatalf("String() = %q, want the trimmed command", got)
	}
}

func TestNewDestructiveCommand_TrimsWhitespace(t *testing.T) {
	d := NewDestructiveCommand("  git reset --hard HEAD~1  ", DestructionStaleDeletion)
	if got := d.String(); got != "git reset --hard HEAD~1" {
		t.Fatalf("String() = %q, want trimmed", got)
	}
}

func TestNewDestructiveCommand_PanicsOnEmpty(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic on an empty command")
		}
	}()
	NewDestructiveCommand("   ", DestructionSuperseded)
}

func TestNewDestructiveCommand_PanicsOnNonDestructive(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected a panic on a non-destructive command")
		}
		if !strings.Contains(r.(string), "matches no reviewed destructive floor family") {
			t.Fatalf("unexpected panic message: %v", r)
		}
	}()
	NewDestructiveCommand("mindspec complete <bead>", DestructionAncestor)
}

// TestNewDestructiveCommand_ZeroValueIsInvalid pins R5(b)'s fail-closed
// zero value: a caller that skips the constructor gets an inert
// placeholder, never a usable command.
func TestNewDestructiveCommand_ZeroValueIsInvalid(t *testing.T) {
	var d DestructiveCommand
	if d.Valid() {
		t.Fatal("zero value must not be Valid()")
	}
	if !strings.Contains(d.String(), "invalid-destructive-command") {
		t.Fatalf("zero value String() = %q, want a fail-closed placeholder", d.String())
	}
}

// TestNewDestructiveCommand_MandatoryOutcomeAcceptsZeroValue pins
// R5(b)'s "non-defaultable" wording precisely: DestructionAncestor IS
// DestructionOutcome's own zero value (bead 1's F1-3), and passing it
// here is legitimate — the enforcement is the mandatory PARAMETER
// (Go has no default args; every call site must name a
// DestructionOutcome value), not a non-zero check on the outcome's
// value. This constructor makes no claim that a DestructionAncestor
// outcome licenses THIS command — that mapping is R2/R4's job.
func TestNewDestructiveCommand_MandatoryOutcomeAcceptsZeroValue(t *testing.T) {
	var zero DestructionOutcome // == DestructionAncestor
	d := NewDestructiveCommand("rm -rf ../stale-worktree", zero)
	if !d.Valid() {
		t.Fatal("expected a valid DestructiveCommand even when outcome is DestructionOutcome's zero value")
	}
}

// TestDestructiveCommand_CrossPackageBypassesFailToCompile is the
// compile-negative half of AC-9(vi): a raw-string composite literal
// and an explicit string-to-DestructiveCommand conversion, each in
// their own throwaway package under testdata/ (excluded from ordinary
// `go build ./...`/`go test ./...` pattern expansion — Go's own
// testdata convention — but still buildable by explicit path, which
// is exactly what this test does). Both must fail with a message
// naming the unexported-field/inconvertible-type reason, proving the
// opacity claim empirically rather than by inspection alone (the
// bead-1 lesson: pin behaviour, not presence).
func TestDestructiveCommand_CrossPackageBypassesFailToCompile(t *testing.T) {
	cases := []struct {
		name   string
		dir    string
		substr string
	}{
		{"raw-string-composite-literal", "./testdata/bypass_rawstring", "unexported field"},
		{"explicit-conversion", "./testdata/bypass_conversion", "cannot convert"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd := exec.Command("go", "build", c.dir)
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("expected `go build %s` to fail (opacity bypass), it succeeded:\n%s", c.dir, out)
			}
			if !strings.Contains(string(out), c.substr) {
				t.Fatalf("expected build failure to mention %q, got:\n%s", c.substr, out)
			}
		})
	}
}
