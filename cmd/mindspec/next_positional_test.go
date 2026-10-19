package main

// Spec 101 R1 (mindspec-3cj0.1 / mfe0 / #146.1): `mindspec next <bead-id>`
// must honor the positional on the claim path. These tests pin the
// help-text accuracy and the positional-vs-`--pick` conflict guard.

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"os/exec"
	"strings"
	"testing"
)

// The `next` long help must no longer imply the positional is accepted only
// under --emit-only ("accepted generally"), and must describe that it is
// honored on the claim path (claim that bead).
func TestNextHelpDescribesPositionalOnClaimPath(t *testing.T) {
	bin := buildMindspecBinary(t)

	cmd := exec.Command(bin, "next", "--help")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("mindspec next --help: %v\nstderr=%q", err, stderr.String())
	}
	out := stdout.String()

	// The pre-R1 help only mentioned the positional in the --emit-only
	// paragraph ("Accepts an optional positional bead ID."), implying it is
	// ignored on the claim path. The corrected help must name the claim
	// path explicitly.
	if !strings.Contains(out, "claim path") {
		t.Errorf("next --help must describe positional honoring on the claim path\n--- output ---\n%s\n--- end ---", out)
	}
	// Must mention claiming that specific bead.
	if !strings.Contains(out, "claim that bead") {
		t.Errorf("next --help must state the positional claims that bead\n--- output ---\n%s\n--- end ---", out)
	}
}

// Supplying both a positional bead ID and a non-zero --pick is a conflict —
// the command must error before claiming anything (exactly one selector).
func TestNextPositionalPickConflictErrors(t *testing.T) {
	bin := buildMindspecBinary(t)

	cmd := exec.Command(bin, "next", "mindspec-xxxx", "--pick=2")
	cmd.Dir = repoRootFromTestDir(t)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected non-zero exit for positional + --pick conflict\nstdout=%q\nstderr=%q", stdout.String(), stderr.String())
	}
	combined := stdout.String() + stderr.String()
	if !strings.Contains(combined, "exactly one selector") {
		t.Errorf("conflict error must mention 'exactly one selector'\nstdout=%q\nstderr=%q", stdout.String(), stderr.String())
	}
}

func TestNext_SelectorOnEmptySet_Refuses(t *testing.T) {
	for _, tc := range []struct {
		name     string
		args     []string
		sentence string
	}{
		{"positional", []string{"epic-129.1"}, "no ready plan bead for spec 129-fixture; positional epic-129.1 cannot be claimed from an empty ready set"},
		{"pick", []string{"--pick=1"}, "--pick 1 is out of range: 0 items"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newNextHonestyFixture(t)
			f.rows = nil
			f.write(t)
			before := f.audit(t)
			out, err := f.run(t, append([]string{"--spec", "129-fixture"}, tc.args...)...)
			if err == nil {
				t.Fatalf("empty selector succeeded: %s", out)
			}
			requireText(t, err.Error(), tc.sentence)
			if !strings.HasSuffix(err.Error(), "\nrecovery: mindspec next --spec 129-fixture") {
				t.Fatalf("selector recovery must end refusal: %v", err)
			}
			if f.audit(t) != before {
				t.Fatal("selector refusal mutated")
			}
		})
	}
}
func TestNext_NearMissPositional_Refuses(t *testing.T) {
	for _, nonempty := range []bool{false, true} {
		t.Run(fmt.Sprint(nonempty), func(t *testing.T) {
			f := newNextHonestyFixture(t)
			if !nonempty {
				f.rows = nil
			}
			f.rows = append(f.rows, nextHonestyRow("nonplan-r", "neutral", "epic-129", "task", "open"))
			f.write(t)
			before := f.audit(t)
			_, err := f.run(t, "--spec", "129-fixture", "nonplan-r")
			if err == nil {
				t.Fatal("near-miss positional claimed")
			}
			requireText(t, err.Error(), "bead nonplan-r is bd-ready and linked to spec 129-fixture by parent edge, but it is not a plan bead", "recovery: bd show nonplan-r --json")
			if f.audit(t) != before {
				t.Fatal("near-miss refusal mutated")
			}
		})
	}
}
func TestNext_UnlinkedReadyPositional_Refuses(t *testing.T) {
	f := newNextHonestyFixture(t)
	f.rows = append(f.rows, nextHonestyRow("epic-777.1", "other", "epic-777", "task", "open"))
	f.write(t)
	before := f.audit(t)
	_, err := f.run(t, "--spec", "129-fixture", "epic-777.1")
	if err == nil {
		t.Fatal("unlinked ready bead claimed")
	}
	requireText(t, err.Error(), "bead epic-777.1 is bd-ready but is not linked to spec 129-fixture (parent: epic-777)")
	if !strings.HasSuffix(err.Error(), "\nrecovery: mindspec next --spec 129-fixture") {
		t.Fatalf("unlinked recovery: %v", err)
	}
	if f.audit(t) != before {
		t.Fatal("unlinked refusal mutated")
	}
}
func TestNext_N4NoLineagePositionalRefuses(t *testing.T) {
	for _, allow := range []bool{false, true} {
		t.Run(fmt.Sprint(allow), func(t *testing.T) {
			f := newNextHonestyFixture(t)
			f.rows = append(f.rows, nextHonestyRow("epic-129.4", "neutral", "", "task", "open"))
			f.write(t)
			old := nextGateReadinessFn
			calls := 0
			nextGateReadinessFn = func(string, string, bool) (*next.GateResult, error) {
				calls++
				return nil, errors.New("gate should never be reached")
			}
			t.Cleanup(func() { nextGateReadinessFn = old })
			before := f.audit(t)
			args := []string{"--spec", "129-fixture", "epic-129.4"}
			if allow {
				args = append(args, "--allow-not-ready")
			}
			_, err := f.run(t, args...)
			if err == nil {
				t.Fatal("no-lineage N4 selected")
			}
			requireText(t, err.Error(), "bead epic-129.4 has no lineage, so readiness cannot evaluate it; no readiness override applies", "recovery: bd show epic-129.4 --json")
			if calls != 0 || f.audit(t) != before {
				t.Fatalf("no-lineage N4 reached gate (%d) or mutated", calls)
			}
		})
	}
}
func TestNext_N4WithLineageReachesMF1(t *testing.T) {
	f := newNextHonestyFixture(t)
	f.rows = append(f.rows, nextHonestyRow("epic-129.4", "follow-up", "epic-129", "bug", "open"))
	f.write(t)
	calls := 0
	old := nextGateReadinessFn
	nextGateReadinessFn = func(root, id string, allow bool) (*next.GateResult, error) {
		calls++
		return next.GateReadiness(root, id, allow)
	}
	t.Cleanup(func() { nextGateReadinessFn = old })
	before := f.audit(t)
	_, err := f.run(t, "--spec", "129-fixture", "epic-129.4")
	if err == nil || calls != 1 {
		t.Fatalf("N4 must reach gate once: %d %v", calls, err)
	}
	requireText(t, err.Error(), "MF-1: FAIL")
	if f.audit(t) != before {
		t.Fatal("NOT-READY refusal mutated")
	}
}
