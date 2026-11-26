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

// PublishSlotVerdictForTest runs the publish step of a slot submit for file,
// as a retry inside Submit does.
func PublishSlotVerdictForTest(s State, step, file string) (string, error) {
	r := s.Records[step]
	t, _, err := r.Tuple()
	if err != nil {
		return "", err
	}
	pending, err := pendingSlotVerdict(s, r, t, file)
	if err != nil {
		return "", err
	}
	return pending.publish(s)
}
