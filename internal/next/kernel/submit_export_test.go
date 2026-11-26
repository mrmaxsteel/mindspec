package kernel

// VerifyCloseWritesForTest exposes the production projection to timestamp checks.
func VerifyCloseWritesForTest(s State, r Record, receipt Receipt) ([]Write, error) {
	return verifyCloseWrites(s, r, receipt)
}

// DisableArtifactCheckForTest exposes the missing-check refusal to external tests.
// Tests that use it must not run in parallel.
func DisableArtifactCheckForTest(kind string) func() {
	check, exists := artifactChecks[kind]
	if !exists {
		panic("test needs a registered artifact check")
	}
	delete(artifactChecks, kind)
	return func() { artifactChecks[kind] = check }
}
