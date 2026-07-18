package doctor

// Test-only seam setters for the finalize-orphan checks (Spec 119 Bead 2).
// Mirrors the phase.SetRunBDForTest / phase.SetListJSONForTest convention:
// an exported setter returning a restore closure, rather than importing
// "testing" into production code. These exist so a cross-package fixture
// (internal/instruct's AC-15 same-text proof, lifecycle_findings_crosscheck_test.go)
// can drive checkFinalizeOrphans without a live `bd`, while leaving the
// REAL lifecycle predicates (findOutstandingFinalizeBranchesFn,
// staleTrackerOnMainFn) unstubbed — the property under test is that both
// `mindspec doctor` and the generated `mindspec instruct` guidance render
// the SAME text from those real predicates.

// SetFindEpicForFinalizeCheckForTest overrides the epic-resolution seam
// checkFinalizeOrphans uses to map a spec ID to its epic ID. Returns a
// restore func.
func SetFindEpicForFinalizeCheckForTest(fn func(specID string) (string, error)) func() {
	orig := findEpicForFinalizeCheckFn
	findEpicForFinalizeCheckFn = fn
	return func() { findEpicForFinalizeCheckFn = orig }
}

// SetFindEpicStatusForTest overrides the epic-status-resolution seam
// checkFinalizeOrphans uses to derive StaleTrackerOnMain's liveClosed
// argument. Returns a restore func.
func SetFindEpicStatusForTest(fn func(epicID string) (string, error)) func() {
	orig := findEpicStatusFn
	findEpicStatusFn = fn
	return func() { findEpicStatusFn = orig }
}

// RunFinalizeOrphanChecksForTest runs ONLY the finalize-orphan checks
// (checkFinalizeOrphans) against root, returning the resulting Report.
// Test-only surface for cross-package AC-15 verification.
func RunFinalizeOrphanChecksForTest(root string) *Report {
	r := &Report{}
	checkFinalizeOrphans(r, root)
	return r
}
