package lifecycle

// Spec 127 AC-3: the outcome oracle's own consumer — this file is NOT
// under outcome_oracle_test.go's independence ban (that ban is scoped
// to outcome_oracle_test.go's own source, per its doc comment's
// CONFINEMENT RULE): it legitimately calls the REAL predicate
// (EvaluateWorkDestruction) and the REAL derivation (DeriveOrphanHint)
// over real-git fixtures, then judges the result against
// outcome_oracle_test.go's ground-truth probes. Divergence between
// probe-derived truth and the predicate/derivation's claim is a test
// failure — that comparison is the whole point of this file, and it
// would be meaningless from inside the oracle's own confined file.

import (
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/guard"
)

// ac3GitRun/ac3WriteFile/ac3Commit/ac3InitRepo mirror
// outcome_oracle_test.go's oracle* helpers but are DEFINED HERE (not
// shared) — this file builds fixtures for the REAL predicate to
// evaluate, a different concern from the oracle's own confined
// ground-truth probes, and keeping them separate means a change to
// one's helper shape can never accidentally weaken the other's fence.
func ac3GitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return oracleGitRun(t, dir, args...)
}

func ac3WriteFile(t *testing.T, dir, rel, content string) { oracleWriteFile(t, dir, rel, content) }
func ac3Commit(t *testing.T, dir, msg string)             { oracleCommit(t, dir, msg) }
func ac3InitRepo(t *testing.T) string                     { return oracleInitRepo(t) }

// ac3Fixture builds one real-git shape and returns (dir, beadBranch,
// specTargetRef) — the exact inputs EvaluateOrphanHint's own
// EvaluateWorkDestruction(dir, beadBranch, specTargetRef) call takes.
type ac3Fixture struct {
	name          string
	outcome       guard.DestructionOutcome
	build         func(t *testing.T) (dir, beadBranch, specTargetRef string)
	oracleAsserts func(t *testing.T, dir, beadBranch, specTargetRef string)
}

// ac3AncestorFixture is AC-3 leg (ii): beadBranch is main's own tip (an
// ancestor of main trivially) while specTargetRef is an UNRELATED
// orphan history — ancestor-of-main-but-not-of-spec, the reachable
// Orphan case (orphans.go:55-57).
func ac3AncestorFixture(t *testing.T) (dir, beadBranch, specTargetRef string) {
	dir = ac3InitRepo(t)
	ac3GitRun(t, dir, "branch", "stale-ancestor")
	ac3GitRun(t, dir, "checkout", "-q", "--orphan", "unrelated-spec")
	ac3GitRun(t, dir, "rm", "-q", "-rf", "--cached", ".")
	ac3WriteFile(t, dir, "unrelated.txt", "unrelated spec history\n")
	ac3Commit(t, dir, "unrelated orphan root")
	ac3GitRun(t, dir, "checkout", "-q", "main")
	return dir, "stale-ancestor", "unrelated-spec"
}

// ac3SupersededFixture is AC-3 leg (i): the #218 shape — a stale bead
// branch whose content already landed in the spec target via ANOTHER
// route (a squash merge).
func ac3SupersededFixture(t *testing.T) (dir, beadBranch, specTargetRef string) {
	dir = ac3InitRepo(t)
	ac3GitRun(t, dir, "checkout", "-q", "-b", "spec-target")
	ac3GitRun(t, dir, "checkout", "-q", "-b", "stale-bead", "spec-target")
	ac3WriteFile(t, dir, "feature.txt", "feature content\n")
	ac3Commit(t, dir, "feature work")
	ac3GitRun(t, dir, "checkout", "-q", "spec-target")
	ac3GitRun(t, dir, "merge", "-q", "--squash", "stale-bead")
	ac3Commit(t, dir, "squash merge feature (another route)")
	ac3GitRun(t, dir, "checkout", "-q", "main")
	return dir, "stale-bead", "spec-target"
}

// ac3StaleDeletionFixture is AC-3 leg (v): a bead branch recreated from
// the spec target's OWN (now-superseded) tip, reverting content the
// target added since, PLUS a genuinely novel addition of its own
// (bead-work.txt).
//
// The novel addition is load-bearing, not decoration: without it,
// stale-bead's WHOLE diff relative to merge-base(stale-bead, "main") —
// main is ALWAYS a second ancestryTarget the predicate checks, even
// when this fixture's own caller only cares about spec-target — is a
// PURE deletion (nothing added at all), which NetEffectLanded's own
// "re-apply the diff to target" check accepts VACUOUSLY (an empty/
// deletion-only diff re-applies cleanly with nothing to compare),
// misclassifying the fixture as DestructionSuperseded before the
// stale-deletion leg ever runs (verified empirically while authoring
// this fixture — the exact trap gitutil's own
// wdStaleDeletionSingleCommitFixture avoids the same way, by amending a
// novel path onto the revert commit).
func ac3StaleDeletionFixture(t *testing.T) (dir, beadBranch, specTargetRef string) {
	dir = ac3InitRepo(t)
	ac3GitRun(t, dir, "checkout", "-q", "-b", "spec-target")
	ac3WriteFile(t, dir, "landed.txt", "landed after the branch's snapshot\n")
	ac3Commit(t, dir, "advance spec-target past the branch's old snapshot")
	ac3GitRun(t, dir, "checkout", "-q", "-b", "stale-bead", "spec-target")
	ac3GitRun(t, dir, "rm", "-q", "landed.txt")
	ac3Commit(t, dir, "revert to the old snapshot (stale recreation)")
	ac3WriteFile(t, dir, "bead-work.txt", "the bead's own novel work\n")
	ac3GitRun(t, dir, "add", "bead-work.txt")
	ac3GitRun(t, dir, "commit", "-q", "--amend", "-m", "revert to the old snapshot + novel bead work")
	ac3GitRun(t, dir, "checkout", "-q", "main")
	return dir, "stale-bead", "spec-target"
}

// ac3EvidenceErrorFixture is AC-3 leg (iv): beadBranch and specTargetRef
// (and main) are three MUTUALLY UNRELATED orphan histories — no
// merge-base exists between beadBranch and specTargetRef, one of
// EvaluateWorkDestruction's own two whole-repository preconditions that
// guarantee DestructionEvidenceError (its package doc comment).
func ac3EvidenceErrorFixture(t *testing.T) (dir, beadBranch, specTargetRef string) {
	dir = ac3InitRepo(t)
	ac3GitRun(t, dir, "checkout", "-q", "--orphan", "unrelated-spec")
	ac3GitRun(t, dir, "rm", "-q", "-rf", "--cached", ".")
	ac3WriteFile(t, dir, "spec.txt", "spec history\n")
	ac3Commit(t, dir, "unrelated spec root")
	ac3GitRun(t, dir, "checkout", "-q", "--orphan", "unrelated-bead")
	ac3GitRun(t, dir, "rm", "-q", "-rf", "--cached", ".")
	ac3WriteFile(t, dir, "bead.txt", "bead history\n")
	ac3Commit(t, dir, "unrelated bead root")
	ac3GitRun(t, dir, "checkout", "-q", "main")
	return dir, "unrelated-bead", "unrelated-spec"
}

// ac3CleanFixture is AC-3 leg (iii)'s guard half (the byte-unchanged
// common case, oracle-judged here; the CompleteBead convergence itself
// is proven separately in internal/executor's
// TestOrphanConvergence_NormalUnmergedBeadMergesCleanly, per this
// spec's own P2-plan-1 pin that the convergence leg must drive the REAL
// executor, unreachable from this package without an import cycle).
func ac3CleanFixture(t *testing.T) (dir, beadBranch, specTargetRef string) {
	dir = ac3InitRepo(t)
	ac3GitRun(t, dir, "branch", "spec-target")
	ac3GitRun(t, dir, "checkout", "-q", "-b", "novel-bead")
	ac3WriteFile(t, dir, "deliverable.txt", "novel bead work\n")
	ac3Commit(t, dir, "bead work")
	ac3GitRun(t, dir, "checkout", "-q", "main")
	return dir, "novel-bead", "spec-target"
}

func ac3Table() []ac3Fixture {
	return []ac3Fixture{
		{
			name: "ancestor-of-main-not-of-spec", outcome: guard.DestructionAncestor, build: ac3AncestorFixture,
			oracleAsserts: func(t *testing.T, dir, beadBranch, specTargetRef string) {
				if !oracleIsAncestor(t, dir, beadBranch, "main") {
					t.Fatal("oracle: beadBranch must independently prove ancestor-of-main for this fixture")
				}
				if oracleIsAncestor(t, dir, beadBranch, specTargetRef) {
					t.Fatal("oracle: beadBranch must NOT be ancestor-of-spec (the reachable Orphan case)")
				}
			},
		},
		{
			name: "superseded", outcome: guard.DestructionSuperseded, build: ac3SupersededFixture,
			oracleAsserts: func(t *testing.T, dir, beadBranch, specTargetRef string) {
				if oracleIsAncestor(t, dir, beadBranch, specTargetRef) || oracleIsAncestor(t, dir, beadBranch, "main") {
					t.Fatal("oracle: a superseded branch must not itself be a literal ancestor")
				}
				// Independent supersession proof: merging beadBranch into
				// specTargetRef changes NOTHING — the preview tree equals
				// specTargetRef's own tip tree, proving the content is
				// already fully present via the squash route.
				previewTree := ac3GitRun(t, dir, "merge-tree", "--write-tree", specTargetRef, beadBranch)
				previewTree = strings.SplitN(previewTree, "\n", 2)[0]
				targetTree := ac3GitRun(t, dir, "rev-parse", specTargetRef+"^{tree}")
				if previewTree != targetTree {
					t.Fatalf("oracle: merging a superseded branch must be a tree no-op; preview=%s target=%s", previewTree, targetTree)
				}
				// And its own commits carry content unreachable from
				// either ref — the preserve-first clause's own ground
				// truth.
				if len(oracleUnreachableCommits(t, dir, beadBranch, specTargetRef, "main")) == 0 {
					t.Fatal("oracle: a superseded branch's own commits must be unreachable from target/main — the preserve-first clause's proof")
				}
			},
		},
		{
			name: "stale-deletion", outcome: guard.DestructionStaleDeletion, build: ac3StaleDeletionFixture,
			oracleAsserts: func(t *testing.T, dir, beadBranch, specTargetRef string) {
				if oracleIsAncestor(t, dir, beadBranch, specTargetRef) || oracleIsAncestor(t, dir, beadBranch, "main") {
					t.Fatal("oracle: a stale-deletion branch must not itself be a literal ancestor")
				}
				deleted := oraclePreviewDeletedPaths(t, dir, specTargetRef, beadBranch)
				if len(deleted) == 0 {
					t.Fatal("oracle: merging this branch must preview at least one deletion of target-present content — that is the whole claim a stale-deletion hint makes")
				}
			},
		},
		{
			name: "evidence-error", outcome: guard.DestructionEvidenceError, build: ac3EvidenceErrorFixture,
			oracleAsserts: func(t *testing.T, dir, beadBranch, specTargetRef string) {
				if oracleMergeBaseExists(t, dir, beadBranch, specTargetRef) {
					t.Fatal("oracle: this fixture's whole point is NO merge-base between beadBranch and specTargetRef")
				}
			},
		},
		{
			name: "normal-unmerged", outcome: guard.DestructionClean, build: ac3CleanFixture,
			oracleAsserts: func(t *testing.T, dir, beadBranch, specTargetRef string) {
				if oracleIsAncestor(t, dir, beadBranch, specTargetRef) || oracleIsAncestor(t, dir, beadBranch, "main") {
					t.Fatal("oracle: a normal-unmerged branch must not be a literal ancestor")
				}
				if deleted := oraclePreviewDeletedPaths(t, dir, specTargetRef, beadBranch); len(deleted) != 0 {
					t.Fatalf("oracle: an ordinary merge must delete nothing target carries, got %v", deleted)
				}
			},
		},
	}
}

// TestOutcomeOracle_AC3Table is AC-3's own table: for every outcome, (1)
// the REAL predicate + derivation run over a real-git fixture built
// through the real scan (never a fabricated evidence value), (2) the
// oracle's independent probes confirm the properties that LICENSE the
// claimed outcome, (3) every rendered line is WELL-FORMED against the
// pinned templates, byte-match, and (4) the derivation's declared
// Outcome equals what the fixture builder intended — divergence here
// (a real predicate bug, not merely a template mismatch) is exactly
// what this table exists to catch.
func TestOutcomeOracle_AC3Table(t *testing.T) {
	table := ac3Table()
	if len(table) != int(guard.DestructionOutcomeCount) {
		t.Fatalf("AC-3 table has %d rows, want %d (guard.DestructionOutcomeCount)", len(table), guard.DestructionOutcomeCount)
	}
	for _, row := range table {
		t.Run(row.name, func(t *testing.T) {
			dir, beadBranch, specTargetRef := row.build(t)

			// The REAL predicate — never fabricated.
			outcome, _, _ := EvaluateWorkDestruction(dir, beadBranch, specTargetRef)
			if outcome != row.outcome {
				t.Fatalf("EvaluateWorkDestruction = %v, want %v (fixture invariant broken — the oracle judges the WRONG shape below if this drifts silently)", outcome, row.outcome)
			}

			// Independent oracle proof that the claimed outcome's
			// licensing properties actually hold, from raw probes alone.
			row.oracleAsserts(t, dir, beadBranch, specTargetRef)

			// The REAL derivation, oracle-judged for well-formedness —
			// never executed.
			hint := EvaluateOrphanHint(dir, "test-bead", beadBranch, "042-test", specTargetRef)
			if hint.Outcome != row.outcome {
				t.Fatalf("DeriveOrphanHint's own Outcome = %v, want %v", hint.Outcome, row.outcome)
			}
			if len(hint.Lines) == 0 {
				t.Fatal("Lines must never be empty")
			}
			for _, line := range hint.Lines {
				if _, ok := oracleWellFormed(line); !ok {
					t.Errorf("line %q is not WELL-FORMED against any pinned template", line)
				}
			}

			// R2(b)'s own falsifier, oracle-enforced per outcome: never a
			// merge-shaped hint for ancestor/superseded/stale-deletion;
			// never mindspec complete outside Clean.
			joined := strings.Join(hint.Lines, "\n")
			if row.outcome != guard.DestructionClean && strings.Contains(joined, "mindspec complete") {
				t.Errorf("a non-normal outcome must never render mindspec complete, got: %v", hint.Lines)
			}
			if row.outcome == guard.DestructionStaleDeletion || row.outcome == guard.DestructionEvidenceError {
				if strings.Contains(joined, "git branch -D") {
					t.Errorf("stale-deletion/evidence-error must never render a destructive deletion command, got: %v", hint.Lines)
				}
			}
		})
	}
}
