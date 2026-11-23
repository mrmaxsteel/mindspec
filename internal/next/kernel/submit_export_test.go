package kernel

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
