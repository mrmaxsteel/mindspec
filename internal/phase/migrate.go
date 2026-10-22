package phase

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/shellquote"
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
//
// Constructs a fresh cache; hot-path callers (e.g. complete.Run, which resolves
// the same epic again downstream) should use EnsureMigratedWithCache to share
// the one `bd list --type=epic` lookup. root locates the R3.3 phase write
// lock (`<root>/.mindspec/phase.lock`) this backfill now runs inside.
func EnsureMigrated(root, specID string) (bool, error) {
	return EnsureMigratedWithCache(root, NewCache(), specID)
}

// EnsureMigratedWithCache is the cache-aware variant of EnsureMigrated. The
// spec→epic resolution and every epic read route through the supplied cache,
// so a caller that also resolves the epic elsewhere in the same invocation
// pays for at most one `bd list --type=epic` (spec 107 wave 1).
//
// Spec 128 R3.3 (round-3 G1): the absent-check → derive → write sequence
// now runs entirely INSIDE WithPhaseWriteLock's critical section — this
// backfill is one of the mindspec_phase writers the R3.3 lock serializes,
// so a concurrent writer's fresher value can never be clobbered by a
// stale legacy backfill (and vice versa: the backfill's own absent-check
// re-reads the FRESH stored value under the lock, so it never re-fires
// after another writer has already set a real phase).
//
// Round-1 panel G1: the absent-check is PRESENCE-aware, not merely
// value-aware. A genuine epic-lookup failure under the lock is
// propagated by WithPhaseWriteLock itself (fn is never invoked, no write
// is attempted). A field that is PRESENT but fails validation (garbage,
// or a value state.IsValidPhase rejects) is NOT "genuinely missing" —
// only `present == false` backfills; a present-but-invalid value is left
// untouched here (repair phase's operator-facing guard is the correct
// lever for a corrupted value, not a silent backfill).
//
// Round-2 panel G1 (hard-block driver, renewed one writer over): the
// children read inside the callback now uses a FRESH per-call Cache
// constructed INSIDE the callback — never the caller-supplied `c`, whose
// children could have been fetched (and cached) BEFORE this call ever
// reached the lock, making it a stale snapshot at write time exactly
// like the bug class round-1 fixed for SanctionedDerivedSync — and a
// GetChildren failure now PROPAGATES (aborting with no write) instead of
// silently deriving "plan" from the resulting nil slice.
func EnsureMigratedWithCache(root string, c *Cache, specID string) (bool, error) {
	epicID, err := FindEpicBySpecIDWithCache(c, specID)
	if err != nil || epicID == "" {
		return false, nil // nothing to migrate
	}

	var (
		migrated bool
		derived  string
	)
	lockErr := WithPhaseWriteLock(root, epicID, mergeMetadataFn, func(_ string, present bool) (map[string]interface{}, error) {
		if present {
			return nil, nil // already migrated, post-080 native, or present-but-invalid (not this backfill's concern)
		}
		freshCache := NewCache()
		children, cerr := freshCache.GetChildren(epicID)
		if cerr != nil {
			return nil, fmt.Errorf("ensure-migrated: reading children of epic %s: %w", idrender.Bead(epicID), cerr)
		}
		derived = DerivePhaseFromChildren(children)
		migrated = true
		return map[string]interface{}{
			"mindspec_phase":       derived,
			"mindspec_migrated_at": time.Now().UTC().Format(time.RFC3339Nano),
		}, nil
	})
	if lockErr != nil {
		// Round-2 panel G3: a lock TIMEOUT gets a copy-pastable recovery
		// COMMAND, not prose — this function is caller-agnostic (wired
		// from plan/impl/complete's own preflight), so `mindspec repair
		// phase <spec-id>` (this file's own house convention: re-derive
		// and merge-write the phase) is the one re-run every caller's
		// verb converges on, unlike re-invoking the caller's own verb
		// (which would re-run its OTHER preflight steps too).
		var recoverySuffix string
		if errors.Is(lockErr, ErrPhaseLockTimeout) {
			recoverySuffix = fmt.Sprintf("\nrecovery: mindspec repair phase %s", shellquote.Quote(idrender.Spec(specID)))
		}
		return false, fmt.Errorf("ensure-migrated %s: %w%s", specID, lockErr, recoverySuffix)
	}
	if !migrated {
		return false, nil
	}
	fmt.Fprintf(os.Stderr, "event=lifecycle.migrated spec=%s epic=%s phase=%s\n",
		specID, epicID, derived)
	return true, nil
}
