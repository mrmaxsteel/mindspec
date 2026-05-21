package phase

import (
	"fmt"
	"os"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bead"
)

// mergeMetadataFn is the test seam for bead.MergeMetadata used by
// EnsureMigrated. Stubbed via SetMergeMetadataForTest.
var mergeMetadataFn = bead.MergeMetadata

// MergeMetadataFunc is the function signature for metadata-merge calls.
type MergeMetadataFunc func(issueID string, updates map[string]interface{}) error

// SetMergeMetadataForTest allows tests in other packages to stub the
// metadata-merge call site. Returns a restore function to be called in
// t.Cleanup (or via defer).
func SetMergeMetadataForTest(fn MergeMetadataFunc) func() {
	orig := mergeMetadataFn
	mergeMetadataFn = fn
	return func() { mergeMetadataFn = orig }
}

// EnsureMigrated runs one-shot legacy-to-metadata migration for the
// spec's lifecycle epic. Returns (migrated bool, err error).
//
// Per ADR-0034: if the spec's epic lacks mindspec_phase metadata
// (legacy pre-080 7-bead layout), derive the phase from existing
// ceremony children once, write mindspec_phase + mindspec_migrated_at,
// and return (true, nil). If the metadata is already present, or no
// epic exists yet (pre-approve-spec), return (false, nil).
//
// Idempotent: a second call on a migrated epic returns (false, nil).
func EnsureMigrated(specID string) (bool, error) {
	epicID, err := FindEpicBySpecID(specID)
	if err != nil || epicID == "" {
		return false, nil // nothing to migrate
	}
	c := NewCache()
	if storedPhase := readStoredPhaseWithCache(c, epicID); storedPhase != "" {
		return false, nil // already migrated or post-080 native
	}
	children, _ := c.GetChildren(epicID)
	derived := DerivePhaseFromChildren(children)
	if err := mergeMetadataFn(epicID, map[string]interface{}{
		"mindspec_phase":       derived,
		"mindspec_migrated_at": time.Now().UTC().Format(time.RFC3339Nano),
	}); err != nil {
		return false, fmt.Errorf("ensure-migrated %s: %w", specID, err)
	}
	fmt.Fprintf(os.Stderr, "event=lifecycle.migrated spec=%s epic=%s phase=%s\n",
		specID, epicID, derived)
	return true, nil
}
