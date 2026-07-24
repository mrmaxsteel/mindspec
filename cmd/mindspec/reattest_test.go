package main

// reattest_test.go — spec 125 Bead 4, the cmd-layer half of AC-7/AC-8/
// AC-11: the flag-surface enumeration (no bypass, no operator-asserted
// SHA pair), the --spec-branch fallback-only precedence (plan-gate
// F2-2), the ADR-0035 refusal rendering with named forward exits, the
// not-writable-via-doctor pin, and the ADR-0041 §2(ii) amendment anchor
// test (marker gone, anchors present, citing code cites the clause).

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
)

// TestReattestFlagSurfaceNoBypass is AC-8's flag-surface assertion: the
// verb's ENTIRE local flag set is exactly {--spec-branch} — scoping
// input only. There is no bypass flag of any kind, no --all/fleet flag,
// and no flag through which an operator could assert a merge or
// second-parent SHA pair (the circular datum R4 forbids); the single
// positional argument is the bead ID and nothing else.
func TestReattestFlagSurfaceNoBypass(t *testing.T) {
	var names []string
	reattestCmd.Flags().VisitAll(func(f *pflag.Flag) {
		names = append(names, f.Name)
	})
	if len(names) != 1 || names[0] != "spec-branch" {
		t.Errorf("reattest local flag set = %v, want exactly [spec-branch] (AC-8: no bypass surface)", names)
	}
	// Belt-and-suspenders: no bypass-shaped or SHA-assertion-shaped flag
	// may ever appear (fails loudly if the set above is ever widened).
	banned := []string{"all", "force", "allow", "skip", "override", "bypass", "merge-sha", "second-parent", "sha", "assert"}
	for _, n := range names {
		for _, b := range banned {
			if strings.Contains(n, b) {
				t.Errorf("reattest flag %q matches banned shape %q — corroboration must not be bypassable or assertable", n, b)
			}
		}
	}
	// Exactly ONE positional (the bead ID): a second operand — e.g. an
	// asserted SHA — is rejected by cobra's Args contract.
	if err := reattestCmd.Args(reattestCmd, []string{"mindspec-x1"}); err != nil {
		t.Errorf("one bead-id arg must be accepted: %v", err)
	}
	if err := reattestCmd.Args(reattestCmd, []string{"mindspec-x1", "deadbeef"}); err == nil {
		t.Error("a second positional operand (an asserted SHA) must be rejected — no operator-asserted pair surface exists")
	}
	if err := reattestCmd.Args(reattestCmd, []string{}); err == nil {
		t.Error("zero args must be rejected (single-bead, explicit invocation)")
	}
}

// TestReattestPrecedence_LinkageWins is plan-gate F2-2's first half: the
// bead's epic linkage is tried FIRST and WINS whenever derivable —
// --spec-branch is IGNORED (loudly) when the linkage resolves.
func TestReattestPrecedence_LinkageWins(t *testing.T) {
	var gotBranch string
	var stdout, stderr bytes.Buffer
	deps := reattestDeps{
		deriveSpecBranch: func(string) (string, error) { return "spec/125-linkage", nil },
		reattest: func(root, specBranch, beadID, actor string) (*lifecycle.ReattestResult, error) {
			gotBranch = specBranch
			return &lifecycle.ReattestResult{
				BeadID: beadID, SpecBranch: specBranch,
				MergeSHA: "1234567890abcdef1234567890abcdef12345678", SecondParent: "abc1234", Wrote: true,
				Corroboration: "(a) test",
			}, nil
		},
		actor:  "tester@host via test",
		stdout: &stdout,
		stderr: &stderr,
	}
	if err := runReattest(deps, t.TempDir(), "mindspec-x1", "spec/999-flag"); err != nil {
		t.Fatalf("runReattest: %v", err)
	}
	if gotBranch != "spec/125-linkage" {
		t.Errorf("scanned %q, want the linkage-derived spec/125-linkage (--spec-branch is fallback-only)", gotBranch)
	}
	if !strings.Contains(stderr.String(), "ignored") {
		t.Errorf("ignoring --spec-branch must be loud, stderr = %q", stderr.String())
	}
}

// TestReattestPrecedence_FlagFallbackOnly: with the linkage underivable,
// --spec-branch scopes the scan — accepted in either the full
// spec/<id> form or the bare <id> form, always normalized through the
// composition waist; a malformed value refuses with a recovery line.
func TestReattestPrecedence_FlagFallbackOnly(t *testing.T) {
	for _, flagVal := range []string{"spec/125-fallback", "125-fallback"} {
		var gotBranch string
		var stdout, stderr bytes.Buffer
		deps := reattestDeps{
			deriveSpecBranch: func(string) (string, error) { return "", nil },
			reattest: func(root, specBranch, beadID, actor string) (*lifecycle.ReattestResult, error) {
				gotBranch = specBranch
				return &lifecycle.ReattestResult{BeadID: beadID, SpecBranch: specBranch, MergeSHA: "1234567", SecondParent: "abc1234", Wrote: false}, nil
			},
			actor:  "tester",
			stdout: &stdout,
			stderr: &stderr,
		}
		if err := runReattest(deps, t.TempDir(), "mindspec-x1", flagVal); err != nil {
			t.Fatalf("runReattest(%q): %v", flagVal, err)
		}
		if gotBranch != "spec/125-fallback" {
			t.Errorf("flag %q scanned %q, want the normalized spec/125-fallback", flagVal, gotBranch)
		}
	}

	// Malformed flag value: refused through the waist, never composed.
	deps := reattestDeps{
		deriveSpecBranch: func(string) (string, error) { return "", nil },
		reattest: func(root, specBranch, beadID, actor string) (*lifecycle.ReattestResult, error) {
			t.Fatal("engine must not run on a malformed --spec-branch")
			return nil, nil
		},
		actor:  "tester",
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	}
	err := runReattest(deps, t.TempDir(), "mindspec-x1", "spec/../evil")
	if err == nil || !guard.HasFinalRecoveryLine(err.Error()) {
		t.Fatalf("malformed --spec-branch must refuse with a recovery line, got %v", err)
	}
}

// TestReattestLineageLookupErrorRefusesEvenWithFlag is the spec 125
// final-review FIX-1 pin (RED against the pre-fix fallback): a lineage
// LOOKUP ERROR is INDETERMINATE ownership — not the determinate
// no-lineage state the --spec-branch fallback exists for — and must
// fail CLOSED even when the operator supplied the flag. The pre-fix
// switch degraded a failed lookup to operator scoping with only a
// stderr warning, letting --spec-branch scope a scan whose ownership
// the lookup might have derived differently.
func TestReattestLineageLookupErrorRefusesEvenWithFlag(t *testing.T) {
	for _, flagVal := range []string{"spec/125-fallback", ""} {
		t.Run("flag="+flagVal, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			deps := reattestDeps{
				deriveSpecBranch: func(string) (string, error) {
					return "", errors.New("bd lookup exploded")
				},
				reattest: func(root, specBranch, beadID, actor string) (*lifecycle.ReattestResult, error) {
					t.Fatalf("engine must not run on an INDETERMINATE lineage lookup (flag=%q scoped the scan)", flagVal)
					return nil, nil
				},
				actor:  "tester",
				stdout: &stdout,
				stderr: &stderr,
			}
			err := runReattest(deps, t.TempDir(), "mindspec-x1", flagVal)
			if err == nil || !guard.HasFinalRecoveryLine(err.Error()) {
				t.Fatalf("a lineage lookup error must refuse with a recovery line, got %v", err)
			}
			if !strings.Contains(err.Error(), "lookup FAILED") {
				t.Errorf("refusal must name the failed lookup (indeterminate ownership), got %v", err)
			}
		})
	}
}

// TestReattestNoLinkageNoFlagRefuses: neither an epic linkage nor a
// --spec-branch — a clean refusal naming the flag as the forward exit,
// before any engine work.
func TestReattestNoLinkageNoFlagRefuses(t *testing.T) {
	deps := reattestDeps{
		deriveSpecBranch: func(string) (string, error) { return "", nil },
		reattest: func(root, specBranch, beadID, actor string) (*lifecycle.ReattestResult, error) {
			t.Fatal("engine must not run without a resolvable scan branch")
			return nil, nil
		},
		actor:  "tester",
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	}
	err := runReattest(deps, t.TempDir(), "mindspec-x1", "")
	if err == nil || !guard.HasFinalRecoveryLine(err.Error()) {
		t.Fatalf("expected a guard refusal with a recovery line, got %v", err)
	}
	if !strings.Contains(err.Error(), "--spec-branch") {
		t.Errorf("refusal must name the --spec-branch forward exit, got %v", err)
	}
}

// TestReattestRefusalRendering: every engine refusal state renders as an
// ADR-0035 guard failure with a final recovery line; the truly-bare
// state names the audited ADR-0035 q9ea human attested-restore exit BY
// NAME, with its explicit verify-first marker.
func TestReattestRefusalRendering(t *testing.T) {
	states := []string{
		lifecycle.ReattestStateNoOwnedMerge,
		lifecycle.ReattestStateAmbiguous,
		lifecycle.ReattestStateTipContradiction,
		lifecycle.ReattestStatePanelContradiction,
		lifecycle.ReattestStateReverted,
	}
	for _, state := range states {
		t.Run(state, func(t *testing.T) {
			refusal := &lifecycle.ReattestRefusal{
				BeadID: "mindspec-x1", SpecBranch: "spec/125-test", State: state, Detail: "fixture detail",
			}
			deps := reattestDeps{
				deriveSpecBranch: func(string) (string, error) { return "spec/125-test", nil },
				reattest: func(root, specBranch, beadID, actor string) (*lifecycle.ReattestResult, error) {
					return nil, fmt.Errorf("wrapped: %w", refusal)
				},
				actor:  "tester",
				stdout: &bytes.Buffer{},
				stderr: &bytes.Buffer{},
			}
			err := runReattest(deps, t.TempDir(), "mindspec-x1", "")
			if err == nil {
				t.Fatal("expected a refusal error")
			}
			if !guard.HasFinalRecoveryLine(err.Error()) {
				t.Errorf("refusal for %s lacks a final recovery line (ADR-0035): %v", state, err)
			}
			if state == lifecycle.ReattestStateNoOwnedMerge {
				if !strings.Contains(err.Error(), "q9ea") || !strings.Contains(err.Error(), "attested-restore") {
					t.Errorf("truly-bare refusal must name the ADR-0035 q9ea attested-restore exit, got %v", err)
				}
				if !strings.Contains(strings.ToLower(err.Error()), "verify") {
					t.Errorf("the q9ea exit is deliberately non-mechanical — the message must demand verification first, got %v", err)
				}
			}
		})
	}
}

// TestReattestSuccessOutput: the write path reports the derived identity
// + audit pointer, the G3-1 overwrite reports the prior binding, and the
// convergent no-op says nothing was written.
func TestReattestSuccessOutput(t *testing.T) {
	mk := func(res *lifecycle.ReattestResult) (string, error) {
		var stdout, stderr bytes.Buffer
		deps := reattestDeps{
			deriveSpecBranch: func(string) (string, error) { return "spec/125-test", nil },
			reattest: func(root, specBranch, beadID, actor string) (*lifecycle.ReattestResult, error) {
				return res, nil
			},
			actor:  "tester",
			stdout: &stdout,
			stderr: &stderr,
		}
		err := runReattest(deps, t.TempDir(), "mindspec-x1", "")
		return stdout.String(), err
	}

	out, err := mk(&lifecycle.ReattestResult{
		BeadID: "mindspec-x1", SpecBranch: "spec/125-test",
		MergeSHA: "1234567890abcdef1234567890abcdef12345678", SecondParent: "abcd1234", Wrote: true,
		Corroboration: "(a) test", PriorMergeSHA: "aaaa1111", PriorSecondParent: "bbbb2222",
	})
	if err != nil {
		t.Fatalf("write path: %v", err)
	}
	for _, want := range []string{"1234567890abcdef1234567890abcdef12345678", "abcd1234", "prior binding", "aaaa1111", "mindspec_landed_reattest_"} {
		if !strings.Contains(out, want) {
			t.Errorf("write output missing %q:\n%s", want, out)
		}
	}

	out, err = mk(&lifecycle.ReattestResult{
		BeadID: "mindspec-x1", SpecBranch: "spec/125-test",
		MergeSHA: "1234567890abcdef1234567890abcdef12345678", SecondParent: "abcd1234", Wrote: false,
	})
	if err != nil {
		t.Fatalf("no-op path: %v", err)
	}
	if !strings.Contains(out, "nothing written") {
		t.Errorf("no-op output must say nothing was written:\n%s", out)
	}
}

// TestReattestNotWiredThroughDoctor is AC-7's explicit-invocation pin:
// the re-attest write is produced ONLY by the top-level `reattest` verb —
// doctor neither dispatches it (no such subcommand in the live cobra
// tree) nor references the engine/seam anywhere in its sources (nor does
// the doctor's stale-OPEN lifecycle consumer), so an implicit `doctor`
// run cannot write bindings.
func TestReattestNotWiredThroughDoctor(t *testing.T) {
	// Cobra-tree half: reattest is a TOP-LEVEL command; doctor has no
	// reattest child at any depth.
	var top []string
	for _, c := range rootCmd.Commands() {
		top = append(top, useFirstWord(c.Use))
	}
	found := false
	for _, name := range top {
		if name == "reattest" {
			found = true
		}
	}
	if !found {
		t.Errorf("reattest must be registered as a top-level command, got %v", top)
	}
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if useFirstWord(c.Use) == "reattest" {
			t.Errorf("doctor subtree dispatches %q — the re-attest write must stay behind the explicit verb", c.Use)
		}
		for _, ch := range c.Commands() {
			walk(ch)
		}
	}
	for _, ch := range doctorCmd.Commands() {
		walk(ch)
	}

	// Source half: the doctor's sources (and its stale-OPEN lifecycle
	// consumer) never reference the engine or its write seam.
	repoRoot := repoRootFromTestDir(t)
	var doctorSources []string
	entries, err := os.ReadDir(filepath.Join(repoRoot, "internal", "doctor"))
	if err != nil {
		t.Fatalf("reading internal/doctor: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
			doctorSources = append(doctorSources, filepath.Join("internal", "doctor", e.Name()))
		}
	}
	doctorSources = append(doctorSources,
		filepath.Join("cmd", "mindspec", "doctor.go"),
		filepath.Join("internal", "lifecycle", "stale_open.go"),
	)
	for _, rel := range doctorSources {
		data, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			t.Fatalf("reading %s: %v", rel, err)
		}
		src := strings.ToLower(string(data))
		if strings.Contains(src, "reattestlandedmerge") || strings.Contains(src, "reattestbindingfn") {
			t.Errorf("%s references the re-attest engine/seam — doctor must never write bindings implicitly", rel)
		}
	}
}

// TestADR0041ReattestAmendment_Anchored is AC-11(ii): the finalized
// ADR-0041 §2(ii) amendment is present WITHOUT the plan-time PRE-DRAFT
// marker, carries the load-bearing anchors (derives-from-discipline,
// the STANDALONE (a)–(e) datum enumeration, the q9ea-blessed exit, the
// audit-field list, the detectable-by-inspection honesty clause, and
// the honest recovery scope), and the re-attest code cites §2(ii) by
// name — so the ADR-divergence gate sees the declared touchpoint. RED
// until the amendment landed (marker present) and RED again if any
// anchor is reworded away.
func TestADR0041ReattestAmendment_Anchored(t *testing.T) {
	t.Parallel()

	repoRoot := repoRootFromTestDir(t)
	adrPath := filepath.Join(repoRoot, ".mindspec", "adr", "ADR-0041-gate-before-mutate.md")
	data, err := os.ReadFile(adrPath)
	if err != nil {
		t.Fatalf("reading ADR-0041: %v", err)
	}
	adr := string(data)

	// The plan-time marker is GONE (the amendment is finalized).
	if strings.Contains(adr, "PRE-DRAFT") {
		t.Error("ADR-0041 still carries the PRE-DRAFT marker — the spec 125 amendment was not finalized (AC-11)")
	}
	// The spec's Validation Proofs grep: rg 're-attest|reattest' hits.
	if !strings.Contains(adr, "re-attest") && !strings.Contains(adr, "reattest") {
		t.Error("ADR-0041 must mention re-attest/reattest (spec 125 Validation Proofs)")
	}
	anchors := []string{
		"## Amendment (Spec 125): Re-attested landed-bindings under §2(ii)",
		"corroborated-identity discipline, not from WHEN it is written",
		"EXACT-second-parent match",
		"never\nfrom an operator-asserted SHA pair corroborating itself",
		"**(a)**", "**(b)**", "**(c)**", "**(d)**", "**(e)**",
		"`mindspec-q9ea` human attested-restore marker",
		"NEVER the sole\n  corroboration",
		"detectable-by-inspection, NOT cryptographically tamper-proof",
		"does not\nclaim fleet-wide recovery",
	}
	for _, want := range anchors {
		if !strings.Contains(adr, want) {
			t.Errorf("ADR-0041 §2(ii) amendment missing anchor %q (AC-11)", want)
		}
	}

	// Audit-field list: every field the amendment enumerates.
	for _, field := range []string{"acting identity/authority", "before/after binding values", "timestamp", "invoking operation", "corroborating git datum"} {
		if !strings.Contains(adr, field) {
			t.Errorf("ADR-0041 amendment audit-field list missing %q", field)
		}
	}

	// Citation-site half: the re-attest code cites the clause by name.
	for _, rel := range []string{
		filepath.Join("internal", "lifecycle", "reattest.go"),
		filepath.Join("cmd", "mindspec", "reattest.go"),
	} {
		src, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			t.Fatalf("reading %s: %v", rel, err)
		}
		if !strings.Contains(string(src), "ADR-0041") || !strings.Contains(string(src), "§2(ii)") {
			t.Errorf("%s must cite ADR-0041 §2(ii) (the amendment it implements)", rel)
		}
	}
}
