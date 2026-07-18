package doctor

// Test-only surface for the lifecycle-integrity checks (Spec 119 Bead 2,
// final-review F1). Mirrors the phase.SetRunBDForTest convention: exported
// helpers rather than importing "testing" into production code. This
// exists so a cross-package fixture (internal/instruct's AC-15 same-text
// proof, lifecycle_findings_crosscheck_test.go) can drive
// checkLifecycleIntegrity over a REAL fixture repo — with the underlying
// bd layer stubbed at the phase seams — and compare the rendered text
// against `mindspec instruct`'s output. The lifecycle predicates and the
// shared aggregate scan stay UNSTUBBED: the property under test is that
// both consumers render the SAME text from the same real scan.

// RunLifecycleIntegrityChecksForTest runs ONLY the aggregate
// lifecycle-integrity check (checkLifecycleIntegrity) against root,
// returning the resulting Report. Test-only surface for cross-package
// AC-15 verification.
func RunLifecycleIntegrityChecksForTest(root string) *Report {
	r := &Report{}
	checkLifecycleIntegrity(r, root)
	return r
}
