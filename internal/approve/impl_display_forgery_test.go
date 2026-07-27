package approve

// Spec 120 R4 (converging pass): implOrphanRefusal is the impl.go call
// site that renders lifecycle.Orphan fields directly (o.BeadID,
// o.BeadBranch, o.SpecBranch — all unvalidated bd-list data, see
// internal/lifecycle/orphans.go). These tests pin that a hostile Orphan
// can never render unescaped through this specific function, independent
// of the full ApproveImpl orphan-gate wiring already covered by
// orphan_gate_test.go.

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
)

// stubImplNormalUnmergedHint pins the spec 127 R2 hint-derivation seam to
// the deterministic "normal unmerged" outcome, byte-identical to the
// pre-bead-4 Orphan.RecoveryCommand() text: neither test in this file
// has a real underlying git repo (tmp is a bare t.TempDir(), no `git
// init`), and this file's whole point is pinning implOrphanRefusal's OWN
// message-wrapper escaping — independent of the shared work-destruction
// predicate's git evaluation, which orphan_gate_test.go already covers.
func stubImplNormalUnmergedHint(t *testing.T) {
	t.Helper()
	orig := implEvaluateOrphanHintFn
	t.Cleanup(func() { implEvaluateOrphanHintFn = orig })
	implEvaluateOrphanHintFn = func(workdir, beadID, beadBranch, specID, specBranch string) lifecycle.OrphanHint {
		return lifecycle.OrphanHint{Outcome: guard.DestructionClean, Lines: []string{"mindspec complete " + idrender.Bead(beadID)}}
	}
}

func TestImplOrphanRefusal_HostileOrphanFieldsForcedSafe(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	stubImplNormalUnmergedHint(t)

	hostileBeadID := "bead-x\n--force"
	hostileBranch := "bead/bead-x\x1b[31mFAKE\x1b[0m;evil"
	o := lifecycle.Orphan{
		BeadID:     hostileBeadID,
		BeadBranch: hostileBranch,
		SpecBranch: "spec/010-test",
	}

	err := implOrphanRefusal(tmp, "010-test", o)
	if err == nil {
		t.Fatal("implOrphanRefusal returned nil")
	}
	msg := err.Error()

	// The hostile bead ID is an ID-typed position: it must be forced
	// through strconv.Quote (idrender.Bead), not rendered raw.
	wantQuoted := strconv.Quote(hostileBeadID)
	if !strings.Contains(msg, wantQuoted) {
		t.Errorf("refusal message missing forced-quoted bead ID %q:\n%s", wantQuoted, msg)
	}
	// The raw hostile branch (a control-byte-bearing free-text position)
	// must be escaped, not rendered raw — termsafe.Escape strips/encodes
	// the ESC bytes so they can never move the cursor or fake a line.
	if strings.ContainsRune(msg, 0x1b) {
		t.Errorf("refusal message contains a raw ESC control byte from BeadBranch:\n%q", msg)
	}
	// guard.NewFailure joins the body and o.RecoveryCommand() with exactly
	// one structural newline (ADR-0035's recovery-line convention); the
	// hostile newline embedded in BeadID must be folded into the escaped
	// `\n` two-byte sequence by idrender.Bead's strconv.Quote — never
	// surface as a SECOND real newline that forges an extra terminal
	// line. (guard.FormatFailure would itself panic on a recovery command
	// containing a raw newline — proving idrender.Bead is load-bearing
	// here, not merely cosmetic.)
	if got := strings.Count(msg, "\n"); got != 1 {
		t.Errorf("refusal message has %d real newlines, want exactly 1 (body/recovery-line separator); hostile newline leaked raw: %q", got, msg)
	}
	if !strings.HasSuffix(msg, "recovery: mindspec complete "+wantQuoted) {
		t.Errorf("refusal message's final recovery line was not the forced-quoted RecoveryCommand:\n%s", msg)
	}
}

// TestImplOrphanRefusal_CleanOrphanByteIdentical is the clean-fixture
// counterpart (F3 discipline): a genuine bead ID and ordinary branch names
// must still render byte-identically through the escaping helpers.
func TestImplOrphanRefusal_CleanOrphanByteIdentical(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")
	stubImplNormalUnmergedHint(t)

	const cleanBeadID = "mindspec-9cyu.1"
	o := lifecycle.Orphan{
		BeadID:     cleanBeadID,
		BeadBranch: "bead/" + cleanBeadID,
		SpecBranch: "spec/010-test",
	}

	err := implOrphanRefusal(tmp, "010-test", o)
	if err == nil {
		t.Fatal("implOrphanRefusal returned nil")
	}
	msg := err.Error()
	wantPrefix := "bead " + cleanBeadID + " (branch bead/" + cleanBeadID + ") was closed without running mindspec complete and is not merged into spec/010-test"
	if !strings.HasPrefix(msg, wantPrefix) {
		t.Errorf("clean-fixture message changed:\ngot:  %s\nwant prefix: %s", msg, wantPrefix)
	}
}

// TestImplOrphanRefusal_MultiLineHintThreadsAllLinesInOrder is bead-4
// fix round 1's MAJOR fix (G1-5/O3-1): the accepted consumer-parity
// narrowing claimed each consumer's own per-package tests compensate for
// the missing cross-package literal-parity test — but no existing
// fixture in this package stubbed a MULTI-LINE (non-Clean) hint at all;
// every implOrphanRefusal test above uses the single-line Clean shape.
// This stubs the Superseded shape (preserve-tag, delete, adopt-
// invocation — three lines) and asserts every line appears, in order
// and unmodified, as its OWN "recovery: " line in the final rendered
// message — the fidelity check the narrowing's "each package
// independently tests its own plumbing" claim needed and did not have.
func TestImplOrphanRefusal_MultiLineHintThreadsAllLinesInOrder(t *testing.T) {
	tmp := t.TempDir()
	writeSpecDir(t, tmp, "010-test")

	orig := implEvaluateOrphanHintFn
	t.Cleanup(func() { implEvaluateOrphanHintFn = orig })
	wantLines := []string{
		`git tag preserve/bead-x bead/bead-x   (preserve bead-x's commits before deleting — its content is not guaranteed reachable from spec/010-test)`,
		"git branch -D bead/bead-x",
		`mindspec impl adopt 010-test --reason "<why bead-x's content already reached main outside the lifecycle>"`,
	}
	implEvaluateOrphanHintFn = func(workdir, beadID, beadBranch, specID, specBranch string) lifecycle.OrphanHint {
		return lifecycle.OrphanHint{
			Outcome:      guard.DestructionSuperseded,
			EvidenceNote: "bead bead-x's branch bead/bead-x is superseded — its content already landed in spec/010-test via another route per the shared work-destruction predicate",
			Lines:        append([]string{}, wantLines...),
		}
	}

	o := lifecycle.Orphan{BeadID: "bead-x", BeadBranch: "bead/bead-x", SpecBranch: "spec/010-test"}
	err := implOrphanRefusal(tmp, "010-test", o)
	if err == nil {
		t.Fatal("implOrphanRefusal returned nil")
	}
	msg := err.Error()

	lastIdx := -1
	for i, want := range wantLines {
		idx := strings.Index(msg, want)
		if idx < 0 {
			t.Fatalf("line %d (%q) missing verbatim from the rendered message:\n%s", i, want, msg)
		}
		if idx <= lastIdx {
			t.Fatalf("line %d (%q) did not appear AFTER the previous line — order not preserved:\n%s", i, want, msg)
		}
		lastIdx = idx
		if !strings.Contains(msg, "recovery: "+want) {
			t.Errorf("expected line %q to render as its own recovery line, got:\n%s", want, msg)
		}
	}
}

// TestImplOrphanRefusal_FidelityAcrossAllOutcomes is bead-4 fix round 5's
// answer to G1-5A: spec.md AC-3 and plan.md claimed the multi-line
// fixture above proves implOrphanRefusal's line-threading fidelity "for
// every outcome" — it does not, because it stubs only
// guard.DestructionSuperseded. G1 reproduced the gap: an outcome-
// conditional truncation written to drop the final hint line ONLY when
// hint.Outcome == guard.DestructionAncestor (or, separately,
// guard.DestructionEvidenceError) left that Superseded-only fixture
// green. This table drives implOrphanRefusal over the FULL closed
// outcome set — asserted against guard.DestructionOutcomeCount, never a
// literal count (bead 2 shipped five stale written counts; a sixth
// outcome variant must red this table's length assertion before it
// reds anything else) — with a FABRICATED three-line hint per outcome.
// The fabrication is deliberate, not an oversight: real DeriveOrphanHint
// output is a single line for several outcomes (StaleDeletion,
// EvidenceError, Clean), and this table's job is implOrphanRefusal's OWN
// threading of hint.Lines into guard.NewFailure, never the derivation's
// own content (orphan_hints_test.go already fixtures that separately,
// per outcome, hermetically).
func TestImplOrphanRefusal_FidelityAcrossAllOutcomes(t *testing.T) {
	outcomes := make([]guard.DestructionOutcome, 0, int(guard.DestructionOutcomeCount))
	for o := guard.DestructionOutcome(0); o < guard.DestructionOutcomeCount; o++ {
		outcomes = append(outcomes, o)
	}
	if len(outcomes) != int(guard.DestructionOutcomeCount) {
		t.Fatalf("this table covers %d outcomes, want %d (guard.DestructionOutcomeCount) — a new outcome variant needs its own row", len(outcomes), int(guard.DestructionOutcomeCount))
	}

	for _, outcome := range outcomes {
		t.Run(outcome.String(), func(t *testing.T) {
			tmp := t.TempDir()
			writeSpecDir(t, tmp, "010-test")

			orig := implEvaluateOrphanHintFn
			t.Cleanup(func() { implEvaluateOrphanHintFn = orig })
			wantLines := []string{
				fmt.Sprintf("synthetic recovery line 1 for %s", outcome),
				fmt.Sprintf("synthetic recovery line 2 for %s", outcome),
				fmt.Sprintf("synthetic recovery line 3 for %s", outcome),
			}
			implEvaluateOrphanHintFn = func(workdir, beadID, beadBranch, specID, specBranch string) lifecycle.OrphanHint {
				return lifecycle.OrphanHint{
					Outcome:      outcome,
					EvidenceNote: fmt.Sprintf("synthetic evidence note for %s", outcome),
					Lines:        append([]string{}, wantLines...),
				}
			}

			o := lifecycle.Orphan{BeadID: "bead-x", BeadBranch: "bead/bead-x", SpecBranch: "spec/010-test"}
			err := implOrphanRefusal(tmp, "010-test", o)
			if err == nil {
				t.Fatal("implOrphanRefusal returned nil")
			}
			msg := err.Error()

			lastIdx := -1
			for i, want := range wantLines {
				idx := strings.Index(msg, want)
				if idx < 0 {
					t.Fatalf("outcome %s: line %d (%q) missing verbatim from the rendered message:\n%s", outcome, i, want, msg)
				}
				if idx <= lastIdx {
					t.Fatalf("outcome %s: line %d (%q) did not appear AFTER the previous line — order not preserved:\n%s", outcome, i, want, msg)
				}
				lastIdx = idx
				if !strings.Contains(msg, "recovery: "+want) {
					t.Errorf("outcome %s: expected line %q to render as its own recovery line, got:\n%s", outcome, want, msg)
				}
			}
		})
	}
}
