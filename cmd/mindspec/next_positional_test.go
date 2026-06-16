package main

// Spec 101 R1 (mindspec-3cj0.1 / mfe0 / #146.1): `mindspec next <bead-id>`
// must honor the positional on the claim path. These tests pin the
// help-text accuracy and the positional-vs-`--pick` conflict guard.

import (
	"bytes"
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
