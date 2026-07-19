package main

// Spec 092 (agent-contract-hardening) Bead 8: approval-gate
// discoverability (Req 10).
//
//   - Req 10a: `mindspec --help` carries an "Approval Gates" section
//     listing the three canonical noun-verb gate commands; the section
//     does not leak into subcommand help.
//   - Req 10b: near-miss spellings (`mindspec aprove impl`) and the
//     hidden deprecated alias's bare/unknown-target paths
//     (`mindspec approve`, `mindspec approve bogus`) surface the
//     canonical noun-verb form; the alias's real subcommands keep
//     routing (DQ-3: alias stays hidden).
//
// Subprocess style follows help_golden_test.go (buildMindspecBinary in
// testhelpers_test.go).

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

// canonicalGateCommands are the three approval-gate commands in the
// canonical noun-verb order (spec 092 Req 10).
var canonicalGateCommands = []string{
	"mindspec spec approve <id>",
	"mindspec plan approve <id>",
	"mindspec impl approve <id>",
}

func runBinary(t *testing.T, bin string, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

func TestRootHelpListsApprovalGates(t *testing.T) {
	bin := buildMindspecBinary(t)

	out, err := runBinary(t, bin, "", "--help")
	if err != nil {
		t.Fatalf("mindspec --help: %v\n%s", err, out)
	}

	if !strings.Contains(out, "Approval Gates:") {
		t.Errorf("mindspec --help missing 'Approval Gates:' section (spec 092 Req 10a)\n--- output ---\n%s\n--- end ---", out)
	}
	for _, cmd := range canonicalGateCommands {
		if !strings.Contains(out, cmd) {
			t.Errorf("mindspec --help Approval Gates section missing %q\n--- output ---\n%s\n--- end ---", cmd, out)
		}
	}
}

func TestSubcommandHelpOmitsApprovalGatesSection(t *testing.T) {
	bin := buildMindspecBinary(t)

	// Children inherit the root usage template; the
	// {{if not .HasParent}} guard must keep the section root-only.
	out, err := runBinary(t, bin, "", "spec", "--help")
	if err != nil {
		t.Fatalf("mindspec spec --help: %v\n%s", err, out)
	}
	if strings.Contains(out, "Approval Gates:") {
		t.Errorf("mindspec spec --help leaked the root Approval Gates section\n--- output ---\n%s\n--- end ---", out)
	}
}

// TestBareInvocationPrintsHelpWithApprovalGates pins the root RunE
// no-args path (panel M10): bare `mindspec` must keep the pre-spec-092
// behavior — help on stdout, exit 0 — now including the Approval Gates
// section.
func TestBareInvocationPrintsHelpWithApprovalGates(t *testing.T) {
	bin := buildMindspecBinary(t)

	cmd := exec.Command(bin)
	cmd.Dir = t.TempDir()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("bare mindspec: expected exit 0, got %v\nstderr=%q", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Usage:") || !strings.Contains(out, "Available Commands:") {
		t.Errorf("bare mindspec should print help to stdout\n--- stdout ---\n%s\n--- end ---", out)
	}
	if !strings.Contains(out, "Approval Gates:") {
		t.Errorf("bare mindspec help missing 'Approval Gates:' section\n--- stdout ---\n%s\n--- end ---", out)
	}
	for _, c := range canonicalGateCommands {
		if !strings.Contains(out, c) {
			t.Errorf("bare mindspec help missing %q\n--- stdout ---\n%s\n--- end ---", c, out)
		}
	}
}

func TestNearMissApproveSurfacesCanonicalForm(t *testing.T) {
	bin := buildMindspecBinary(t)

	// Spec 092 Req 10b AC: `mindspec aprove impl` produces output
	// containing the canonical `impl approve` suggestion.
	out, err := runBinary(t, bin, t.TempDir(), "aprove", "impl")
	if err == nil {
		t.Fatalf("mindspec aprove impl: expected non-zero exit\n%s", out)
	}
	if !strings.Contains(out, `unknown command "aprove"`) {
		t.Errorf("expected unknown-command error\n--- output ---\n%s\n--- end ---", out)
	}
	if !strings.Contains(out, "mindspec impl approve <id>") {
		t.Errorf("near-miss output missing canonical `impl approve` suggestion (spec 092 Req 10b)\n--- output ---\n%s\n--- end ---", out)
	}
}

// TestNearMissSuggestForListsGateFamilies kills deletion of the
// SuggestFor entries on spec/plan/impl (panel M8): the "Did you mean
// this?" list for an `approve` typo is fed ONLY by SuggestFor —
// "aprove" is levenshtein-far from all three family names and is no
// prefix of any, so cobra's distance/prefix paths cannot produce these
// suggestions. The tab-prefixed assertion ("\timpl\n") distinguishes
// the suggestion list from the approval-gates block (whose lines start
// with two spaces), so deleting any one family's SuggestFor entry
// fails this test.
func TestNearMissSuggestForListsGateFamilies(t *testing.T) {
	bin := buildMindspecBinary(t)

	out, err := runBinary(t, bin, t.TempDir(), "aprove", "impl")
	if err == nil {
		t.Fatalf("mindspec aprove impl: expected non-zero exit\n%s", out)
	}
	if !strings.Contains(out, "Did you mean this?") {
		t.Fatalf("expected SuggestFor suggestion list\n--- output ---\n%s\n--- end ---", out)
	}
	for _, family := range []string{"\tspec\n", "\tplan\n", "\timpl\n"} {
		if !strings.Contains(out, family) {
			t.Errorf("suggestion list missing gate family %q (SuggestFor entry deleted?)\n--- output ---\n%s\n--- end ---", strings.TrimSpace(family), out)
		}
	}
}

// TestNearMissDistanceTwoTriggersGatesHint pins the near-miss distance
// boundary (panel M11): "aprov" is at levenshtein distance exactly 2
// from "approve" (insert 'p', append 'e'; the length difference of 2
// also makes 2 the lower bound), so a mutation tightening
// isApproveNearMiss from <=2 to <=1 fails this test.
func TestNearMissDistanceTwoTriggersGatesHint(t *testing.T) {
	bin := buildMindspecBinary(t)

	out, err := runBinary(t, bin, t.TempDir(), "aprov")
	if err == nil {
		t.Fatalf("mindspec aprov: expected non-zero exit\n%s", out)
	}
	if !strings.Contains(out, "Approval gates use the noun-verb order") {
		t.Errorf("distance-2 near-miss 'aprov' missing the approval-gates hint\n--- output ---\n%s\n--- end ---", out)
	}
	if !strings.Contains(out, "mindspec impl approve <id>") {
		t.Errorf("distance-2 near-miss 'aprov' missing canonical `impl approve` suggestion\n--- output ---\n%s\n--- end ---", out)
	}
}

func TestUnknownCommandStillSuggestsByDistance(t *testing.T) {
	bin := buildMindspecBinary(t)

	// Guard against the custom root Args/RunE regressing cobra's
	// ordinary levenshtein suggestions for non-approve typos.
	out, err := runBinary(t, bin, t.TempDir(), "complet")
	if err == nil {
		t.Fatalf("mindspec complet: expected non-zero exit\n%s", out)
	}
	if !strings.Contains(out, "Did you mean this?") || !strings.Contains(out, "complete") {
		t.Errorf("expected 'complete' suggestion for typo 'complet'\n--- output ---\n%s\n--- end ---", out)
	}
	if strings.Contains(out, "Approval gates use the noun-verb order") {
		t.Errorf("non-approve typo should not trigger the approval-gates hint\n--- output ---\n%s\n--- end ---", out)
	}
}

func TestApproveAliasUnknownTargetSurfacesCanonicalForm(t *testing.T) {
	bin := buildMindspecBinary(t)

	for _, args := range [][]string{
		{"approve", "bogus"}, // unknown target
		{"approve"},          // bare alias
	} {
		out, err := runBinary(t, bin, t.TempDir(), args...)
		if err == nil {
			t.Fatalf("mindspec %s: expected non-zero exit\n%s", strings.Join(args, " "), out)
		}
		if !strings.Contains(out, "mindspec impl approve <id>") {
			t.Errorf("mindspec %s output missing canonical `impl approve` suggestion (spec 092 Req 10b)\n--- output ---\n%s\n--- end ---", strings.Join(args, " "), out)
		}
	}
}

func TestApproveAliasSubcommandsStillRoute(t *testing.T) {
	bin := buildMindspecBinary(t)

	// DQ-3: the hidden deprecated alias keeps working. Run outside any
	// workspace: reaching the workspace-not-found error proves the
	// alias resolved to the real RunE instead of the unknown-target
	// path. The spec-id-shaped "999-some-spec" (spec 120 R3: a
	// non-spec-ID-shaped value would now refuse at the CLI surface
	// BEFORE findRoot ever runs) passes the early ingress gate and
	// reaches the workspace-lookup failure this test actually probes.
	out, err := runBinary(t, bin, t.TempDir(), "approve", "impl", "999-some-spec")
	if err == nil {
		t.Fatalf("mindspec approve impl 999-some-spec outside a workspace: expected non-zero exit\n%s", out)
	}
	if !strings.Contains(out, "workspace not found") {
		t.Errorf("expected `approve impl` to route to the implementation gate (workspace-not-found error), got\n--- output ---\n%s\n--- end ---", out)
	}
	if strings.Contains(out, "unknown approval target") {
		t.Errorf("`approve impl` fell through to the unknown-target error — alias routing broken\n--- output ---\n%s\n--- end ---", out)
	}
}
