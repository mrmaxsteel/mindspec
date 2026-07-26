package guard

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestNewDestructiveCommand_ValidFloorMatch(t *testing.T) {
	d, err := NewDestructiveCommand("git branch -D bead/x", DestructionSuperseded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !d.Valid() {
		t.Fatal("expected Valid() to be true")
	}
	if got := d.String(); got != "git branch -D bead/x" {
		t.Fatalf("String() = %q, want the trimmed command", got)
	}
}

func TestNewDestructiveCommand_TrimsWhitespace(t *testing.T) {
	d, err := NewDestructiveCommand("  git reset --hard HEAD~1  ", DestructionStaleDeletion)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := d.String(); got != "git reset --hard HEAD~1" {
		t.Fatalf("String() = %q, want trimmed", got)
	}
}

// TestNewDestructiveCommand_ErrorsOnEmpty pins RULING 1 (spec 127
// bead-2 rework): a guard that panics on legitimate destructive input
// is a denial of service — this constructor returns an error (never
// panics) and hands back the type's own invalid zero value, not a
// process-terminating failure.
func TestNewDestructiveCommand_ErrorsOnEmpty(t *testing.T) {
	d, err := NewDestructiveCommand("   ", DestructionSuperseded)
	if err == nil {
		t.Fatal("expected an error on an empty command")
	}
	if d.Valid() {
		t.Fatal("expected the returned value to be the invalid zero value")
	}
}

// TestNewDestructiveCommand_ErrorsOnNonFloorMatch is renamed from
// _PanicsOnNonDestructive (O1-r2-2): a non-match means "off this
// reviewed floor", never "certified non-destructive" — the old name
// and message both asserted the latter.
func TestNewDestructiveCommand_ErrorsOnNonFloorMatch(t *testing.T) {
	d, err := NewDestructiveCommand("mindspec complete <bead>", DestructionAncestor)
	if err == nil {
		t.Fatal("expected an error on a command matching no reviewed floor family")
	}
	if !strings.Contains(err.Error(), "matches no family on the reviewed destructive floor") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if strings.Contains(err.Error(), "is for destructive commands only") {
		t.Fatalf("error message must not claim floor membership certifies destructiveness (the old, corrected wording): %v", err)
	}
	if d.Valid() {
		t.Fatal("expected the returned value to be the invalid zero value")
	}
}

// TestNewDestructiveCommand_JoinedMultiIDBdDeleteForceNoLongerPanics
// is F1-r2-1's own regression, pinned at the constructor: the exact
// string internal/approve/plan.go's beadCreateFailure renders via
// strings.Join(created, " ") for 8+ created beads no longer errors —
// it is a genuine floor match — and, independent of that fix, no
// legitimate destructive input reaches a panic here at all (RULING 1).
func TestNewDestructiveCommand_JoinedMultiIDBdDeleteForceNoLongerPanics(t *testing.T) {
	ids := make([]string, 10)
	for i := range ids {
		ids[i] = "mindspec-ab0" + string(rune('0'+i%10)) + ".1"
	}
	cmd := "bd delete " + strings.Join(ids, " ") + " --force"
	d, err := NewDestructiveCommand(cmd, DestructionStaleDeletion)
	if err != nil {
		t.Fatalf("unexpected error for a genuine floor match: %v", err)
	}
	if !d.Valid() {
		t.Fatal("expected a valid DestructiveCommand")
	}
}

// TestNewDestructiveCommand_QuotedGlobalOptionPathNoLongerPanics is
// G1-r2-2's own regression, pinned at the constructor: a legitimately
// quoted, whitespace-bearing `git -C <path>` operand is a genuine
// floor match (not a non-match, and never a panic either way).
func TestNewDestructiveCommand_QuotedGlobalOptionPathNoLongerPanics(t *testing.T) {
	cmd := `git -C "/tmp/work tree" reset --hard HEAD`
	d, err := NewDestructiveCommand(cmd, DestructionStaleDeletion)
	if err != nil {
		t.Fatalf("unexpected error for a genuine floor match: %v", err)
	}
	if !d.Valid() {
		t.Fatal("expected a valid DestructiveCommand")
	}
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
	d, err := NewDestructiveCommand("rm -rf ../stale-worktree", zero)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
// bead-1 lesson: pin behavior, not presence).
func TestDestructiveCommand_CrossPackageBypassesFailToCompile(t *testing.T) {
	// S2-r2-2: under a genuinely read-only GOCACHE, a nested `go
	// build` for a never-before-built package cannot write new
	// compile-cache entries and fails with a permission error BEFORE
	// ever reaching the compiler's type-check diagnostics — tripping
	// this test's own "expected build failure to mention ..." branch
	// with a confusing, unrelated message. Give each subprocess its
	// own writable cache so the outcome depends only on the
	// compiler's type-check result, never on the ambient GOCACHE's
	// write permissions.
	cacheDir := t.TempDir()
	env := append(os.Environ(), "GOCACHE="+cacheDir)

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
			cmd.Env = env
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
