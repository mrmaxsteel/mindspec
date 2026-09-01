package lifecycle

// Spec 127 R2 (bead 4): DeriveOrphanHint's own hermetic, pure fixture
// table — no git, no bd, one row per guard.DestructionOutcome variant,
// count-sentinel-pinned (B-r4-3) so a sixth outcome without a fixture
// here reds this table rather than silently falling through unnoticed.
// The AC-3 outcome oracle (outcome_oracle_test.go) separately proves
// each hint's claimed outcome against real-git ground truth; this file
// proves the derivation's OWN rendering logic in isolation.

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
)

func TestDeriveOrphanHint_OutcomeTable(t *testing.T) {
	type check struct {
		name  string
		check func(t *testing.T, h OrphanHint)
	}
	table := []struct {
		outcome  guard.DestructionOutcome
		evidence gitutil.WorkDestructionEvidence
		checks   []check
	}{
		{
			// R2(b): "ancestor of main but not of the spec branch" ->
			// deletion hint, never a merge — and no preserve-first
			// clause (ancestry of main means every commit is already
			// reachable there; nothing is at risk).
			outcome:  guard.DestructionAncestor,
			evidence: gitutil.WorkDestructionEvidence{AncestorOf: "main"},
			checks: []check{
				{"single deletion line", func(t *testing.T, h OrphanHint) {
					if len(h.Lines) != 1 {
						t.Fatalf("Lines = %v, want exactly 1 (delete only, no preserve-first)", h.Lines)
					}
				}},
				{"deletion command", func(t *testing.T, h OrphanHint) {
					if h.Lines[0] != "git branch -D bead/foo" {
						t.Errorf("Lines[0] = %q, want %q", h.Lines[0], "git branch -D bead/foo")
					}
				}},
				{"evidence note names ancestry", func(t *testing.T, h OrphanHint) {
					if !strings.Contains(h.EvidenceNote, "main") {
						t.Errorf("EvidenceNote = %q, want it to name the ancestor ref", h.EvidenceNote)
					}
				}},
				{"no adopt invocation", func(t *testing.T, h OrphanHint) {
					if strings.Contains(strings.Join(h.Lines, " "), "impl adopt") {
						t.Errorf("an ancestor-of-main deletion never names adopt (no follow-on terminal transition): %v", h.Lines)
					}
				}},
			},
		},
		{
			// R2(b): superseded -> stale-branch deletion + the full
			// adopt invocation, ALWAYS preceded by a preserve-first tag
			// line (unconditional for this outcome — see
			// DeriveOrphanHint's own doc comment on why no extra
			// ancestry probe is needed).
			outcome:  guard.DestructionSuperseded,
			evidence: gitutil.WorkDestructionEvidence{SupersededVia: "main", LandedMergeSHA: "abc123"},
			checks: []check{
				{"three lines: preserve, delete, adopt", func(t *testing.T, h OrphanHint) {
					if len(h.Lines) != 3 {
						t.Fatalf("Lines = %v, want exactly 3 (preserve-first, delete, adopt rerun)", h.Lines)
					}
				}},
				{"preserve-first tag line first", func(t *testing.T, h OrphanHint) {
					if !strings.HasPrefix(h.Lines[0], "git tag preserve/") {
						t.Errorf("Lines[0] = %q, want a preserve-first `git tag` line", h.Lines[0])
					}
				}},
				{"deletion line second", func(t *testing.T, h OrphanHint) {
					if h.Lines[1] != "git branch -D bead/foo" {
						t.Errorf("Lines[1] = %q, want %q", h.Lines[1], "git branch -D bead/foo")
					}
				}},
				{"adopt invocation last, full flags", func(t *testing.T, h OrphanHint) {
					if !strings.Contains(h.Lines[2], `mindspec impl adopt 042-test --reason "<why`) {
						t.Errorf("Lines[2] = %q, want the full impl adopt invocation", h.Lines[2])
					}
				}},
				{"evidence note names the landed merge", func(t *testing.T, h OrphanHint) {
					if !strings.Contains(h.EvidenceNote, "abc123") {
						t.Errorf("EvidenceNote = %q, want it to name the enriched landed-merge SHA", h.EvidenceNote)
					}
				}},
			},
		},
		{
			// R2(b): stale-deletion -> inspection first, a concrete
			// `git diff`, states plainly that merging would delete
			// landed work — NEVER a destructive command.
			outcome:  guard.DestructionStaleDeletion,
			evidence: gitutil.WorkDestructionEvidence{DeletedPaths: []string{"landed.txt"}},
			checks: []check{
				{"single inspection line", func(t *testing.T, h OrphanHint) {
					if len(h.Lines) != 1 {
						t.Fatalf("Lines = %v, want exactly 1 (inspection only)", h.Lines)
					}
				}},
				{"git diff, never a destructive floor match", func(t *testing.T, h OrphanHint) {
					if !strings.HasPrefix(h.Lines[0], "git diff spec/foo bead/foo") {
						t.Errorf("Lines[0] = %q, want a git diff inspection line", h.Lines[0])
					}
					if guard.IsFloorMatch(h.Lines[0]) {
						t.Errorf("a stale-deletion hint's line must never match a destructive floor family: %q", h.Lines[0])
					}
				}},
				{"evidence note states merging would delete", func(t *testing.T, h OrphanHint) {
					if !strings.Contains(h.EvidenceNote, "delete") {
						t.Errorf("EvidenceNote = %q, want it to state plainly that merging would delete landed work", h.EvidenceNote)
					}
				}},
			},
		},
		{
			// R2(b): evidence-error -> inspection-first, never the
			// merge (S3-9's leg).
			outcome:  guard.DestructionEvidenceError,
			evidence: gitutil.WorkDestructionEvidence{FailedProbe: "IsAncestor(bead/foo, spec/foo)"},
			checks: []check{
				{"single inspection line, no destructive match", func(t *testing.T, h OrphanHint) {
					if len(h.Lines) != 1 {
						t.Fatalf("Lines = %v, want exactly 1", h.Lines)
					}
					if guard.IsFloorMatch(h.Lines[0]) {
						t.Errorf("an evidence-error hint's line must never match a destructive floor family: %q", h.Lines[0])
					}
				}},
				{"evidence note names the failed probe", func(t *testing.T, h OrphanHint) {
					if !strings.Contains(h.EvidenceNote, "IsAncestor") {
						t.Errorf("EvidenceNote = %q, want it to name the failed probe", h.EvidenceNote)
					}
				}},
			},
		},
		{
			// R2(b): the converging common case — byte-unchanged from
			// Orphan.RecoveryCommand()'s pre-existing text.
			outcome:  guard.DestructionClean,
			evidence: gitutil.WorkDestructionEvidence{},
			checks: []check{
				{"byte-identical mindspec complete line", func(t *testing.T, h OrphanHint) {
					want := "mindspec complete bead-1"
					if len(h.Lines) != 1 || h.Lines[0] != want {
						t.Errorf("Lines = %v, want exactly [%q]", h.Lines, want)
					}
				}},
				{"no evidence note", func(t *testing.T, h OrphanHint) {
					if h.EvidenceNote != "" {
						t.Errorf("EvidenceNote = %q, want empty (no extra framing for the safe common case)", h.EvidenceNote)
					}
				}},
			},
		},
	}

	// B-r4-3's exhaustiveness discipline: a sixth outcome added to
	// guard.DestructionOutcome without a fixture here reds this
	// assertion, not silently.
	if len(table) != int(guard.DestructionOutcomeCount) {
		t.Fatalf("fixture table has %d rows, want %d (guard.DestructionOutcomeCount) — a new outcome variant needs its own row", len(table), guard.DestructionOutcomeCount)
	}

	for _, row := range table {
		t.Run(row.outcome.String(), func(t *testing.T) {
			h := DeriveOrphanHint(row.outcome, row.evidence, "bead-1", "bead/foo", "042-test", "spec/foo")
			if h.Outcome != row.outcome {
				t.Errorf("h.Outcome = %v, want %v", h.Outcome, row.outcome)
			}
			if len(h.Lines) == 0 {
				t.Fatal("Lines must never be empty — every outcome names at least one recovery action")
			}
			for _, c := range row.checks {
				t.Run(c.name, func(t *testing.T) { c.check(t, h) })
			}
		})
	}
}

// TestDeriveOrphanHint_HostileBeadBranchNeverLeaksRawControlBytes pins
// the escaping discipline every rendered LINE (not merely the
// EvidenceNote) must carry: lifecycle.Orphan.BeadBranch is bd-list free
// text, never idvalidate'd (see orphans.go), so a hostile value must
// never reach a pasteable recovery line unescaped — across every
// outcome that renders beadBranch into a line at all.
func TestDeriveOrphanHint_HostileBeadBranchNeverLeaksRawControlBytes(t *testing.T) {
	hostileBranch := "bead/foo\x1b[31mFAKE\x1b[0m;evil"
	outcomes := []struct {
		outcome  guard.DestructionOutcome
		evidence gitutil.WorkDestructionEvidence
	}{
		{guard.DestructionAncestor, gitutil.WorkDestructionEvidence{AncestorOf: "main"}},
		{guard.DestructionSuperseded, gitutil.WorkDestructionEvidence{SupersededVia: "main"}},
		{guard.DestructionStaleDeletion, gitutil.WorkDestructionEvidence{}},
		{guard.DestructionEvidenceError, gitutil.WorkDestructionEvidence{FailedProbe: "probe"}},
	}
	for _, o := range outcomes {
		t.Run(o.outcome.String(), func(t *testing.T) {
			h := DeriveOrphanHint(o.outcome, o.evidence, "foo", hostileBranch, "042-test", "spec/foo")
			for _, l := range h.Lines {
				if strings.ContainsRune(l, 0x1b) {
					t.Errorf("recovery line leaks a raw ESC control byte from BeadBranch: %q", l)
				}
			}
		})
	}
}

// TestDeriveOrphanHint_DestructiveLinesMatchOnlyReviewedShape is the
// registry obligation for internal/approve/impl.go's implOrphanRefusal
// and internal/approve/adopt.go's adoptOrphanPresentRefusal — both
// spread this file's OrphanHint.Lines as a guard.NewFailure variadic
// argument, an operand internal/lint's convention scan cannot trace
// across the function boundary back to guard.NewDestructiveCommand
// (isConstructorDerived only walks the SAME enclosing function as the
// NewFailure call — R5(b)'s own stated cross-function limitation), so
// the scan registers those two call sites as opaque rather than proving
// them provenance-exempt. This test is the real, disprovable claim that
// stands in its place: over every guard.DestructionOutcome and a
// representative battery of beadBranch/specID values, ANY line this
// file's DeriveOrphanHint can produce that matches a destructive floor
// family is EXACTLY the one reviewed, constructor-derived shape —
// `git branch -D <branch>` — never a different or unconstructed
// destructive command. A future change that lets a raw destructive
// string escape through Lines (bypassing guard.NewDestructiveCommand)
// reds this test.
func TestDeriveOrphanHint_DestructiveLinesMatchOnlyReviewedShape(t *testing.T) {
	branches := []string{"bead/foo", "bead/mindspec-abcd.1", "bead/proj-slug-123"}
	specIDs := []string{"042-test", "127-lifecycle-verb-trustworthiness"}
	fixtures := []struct {
		outcome  guard.DestructionOutcome
		evidence gitutil.WorkDestructionEvidence
	}{
		{guard.DestructionAncestor, gitutil.WorkDestructionEvidence{AncestorOf: "main"}},
		{guard.DestructionAncestor, gitutil.WorkDestructionEvidence{AncestorOf: "spec/foo"}},
		{guard.DestructionSuperseded, gitutil.WorkDestructionEvidence{SupersededVia: "main"}},
		{guard.DestructionSuperseded, gitutil.WorkDestructionEvidence{SupersededVia: "main", LandedMergeSHA: "deadbeef"}},
		{guard.DestructionStaleDeletion, gitutil.WorkDestructionEvidence{DeletedPaths: []string{"a.txt"}}},
		{guard.DestructionEvidenceError, gitutil.WorkDestructionEvidence{FailedProbe: "IsAncestor(x, y)"}},
		{guard.DestructionClean, gitutil.WorkDestructionEvidence{}},
	}
	for _, branch := range branches {
		for _, specID := range specIDs {
			for _, f := range fixtures {
				h := DeriveOrphanHint(f.outcome, f.evidence, "foo", branch, specID, "main")
				for _, l := range h.Lines {
					matches := guard.FindFloorMatches(l)
					if len(matches) == 0 {
						continue
					}
					if len(matches) != 1 || matches[0].Family != guard.FamilyGitBranchDeleteForce {
						t.Errorf("outcome=%v branch=%q specID=%q line %q matches an UNREVIEWED floor shape: %+v", f.outcome, branch, specID, l, matches)
					}
					if want := "git branch -D " + branch; l != want {
						t.Errorf("outcome=%v branch=%q specID=%q destructive line = %q, want the exact constructor-derived shape %q", f.outcome, branch, specID, l, want)
					}
				}
			}
		}
	}
}

// TestEvaluateOrphanHint_FindLandedMergeSeamPinned mirrors the
// AC-17 netEffectLandedFn precedent: findLandedMergeFn's default must be
// exactly FindLandedMerge, both to prove no accidental drift and so a
// test that reassigns it always restores the real implementation.
func TestEvaluateOrphanHint_FindLandedMergeSeamPinned(t *testing.T) {
	got := reflect.ValueOf(findLandedMergeFn).Pointer()
	want := reflect.ValueOf(FindLandedMerge).Pointer()
	if got != want {
		t.Error("findLandedMergeFn's default must be FindLandedMerge itself")
	}
}
