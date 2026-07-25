package guard

// DestructionOutcome is the closed, named outcome set of the shared
// work-destruction predicate (internal/gitutil.EvaluateWorkDestruction,
// spec 127 R4, wrapped for the ADR-0030 enforcement packages by
// internal/lifecycle.EvaluateWorkDestruction). Every consumer of the
// predicate switches over this type EXHAUSTIVELY — a new variant with no
// fixture in a consumer's table is meant to go red at development time
// (spec 127 O2-r2-4). Go has no enum-exhaustiveness check and
// .golangci.yml carries no `exhaustive` linter, so the mechanism is this
// file's DestructionOutcomeCount sentinel: every consumer's fixture table
// asserts `len(table) == DestructionOutcomeCount`, and consumer switches
// carry no `default` arm over this type (spec 127 B-r4-3).
//
// The five outcomes correspond to the spec's evidence classes, in the
// order the predicate evaluates them:
//
//   - DestructionAncestor: the candidate branch is already an ancestor of
//     the target (or of main) — nothing to merge; the safe disposition is
//     branch deletion / no-op.
//   - DestructionSuperseded: the branch's content already landed via
//     another route (a squash merge, a tracker-only carrier, or an
//     equivalent content-level landing) — merging the stale snapshot
//     would regress landed work.
//   - DestructionStaleDeletion: the merge preview deletes content present
//     in the target that the branch never carried as its own authored
//     work — net of the branch's own novel contribution, its tree
//     reconstructs a prior state of the target, so the deletions are
//     staleness artifacts, not authored changes.
//   - DestructionClean: none of the above — an ordinary merge, including
//     one whose own content genuinely conflicts (a real conflict is
//     handled by the merge attempt itself, not by this predicate).
//   - DestructionEvidenceError: the predicate could not be evaluated (a
//     git/infra failure at any of its probes) — absence of evidence is
//     never treated as safety; every consumer fails closed on this
//     outcome.
type DestructionOutcome int

const (
	// DestructionAncestor: the branch is already an ancestor of the target
	// (or of main) — there is nothing to merge.
	DestructionAncestor DestructionOutcome = iota
	// DestructionSuperseded: the branch's content already landed via
	// another route.
	DestructionSuperseded
	// DestructionStaleDeletion: the merge preview deletes target content
	// the branch never authored — a staleness artifact.
	DestructionStaleDeletion
	// DestructionClean: an ordinary merge; no destructive class applies.
	DestructionClean
	// DestructionEvidenceError: the predicate could not be evaluated.
	DestructionEvidenceError
	// DestructionOutcomeCount is the exported count sentinel every
	// consumer's fixture table is pinned against (spec 127 B-r4-3) — it is
	// itself not a valid outcome value.
	DestructionOutcomeCount
)

// String renders o for diagnostics and test failure messages. Not itself a
// "consumer switch" in the exhaustiveness sense above (see the doc comment
// on DestructionOutcome) — it carries a fallback arm so an out-of-range
// value never panics a caller that merely wants to print it.
func (o DestructionOutcome) String() string {
	switch o {
	case DestructionAncestor:
		return "ancestor"
	case DestructionSuperseded:
		return "superseded"
	case DestructionStaleDeletion:
		return "stale-deletion"
	case DestructionClean:
		return "clean"
	case DestructionEvidenceError:
		return "evidence-error"
	default:
		return "unknown-destruction-outcome"
	}
}
