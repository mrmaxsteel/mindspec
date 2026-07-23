package adr

import "errors"

// ErrNotFound is the Store-level sentinel a Get implementation wraps
// (via fmt.Errorf("%w: ...", ErrNotFound) or errors.Is-compatible wrapping)
// when no ADR matches the requested id. It lets a caller that layers
// multiple Store instances — OverlayStore's branch-then-primary fallback —
// distinguish a genuine miss (safe to fall through to the next store) from
// every OTHER error a Get can return, such as a branch-local canonical-
// number collision, which must PROPAGATE rather than be silently masked by
// a fallback (final review G2). FileStore.Get (via Show/ResolveADRFile)
// wraps workspace.ErrADRNotFound into this sentinel; any future Store
// implementation should do the same for its own not-found case.
var ErrNotFound = errors.New("adr not found")

// Store abstracts access to architectural decision records.
// The default implementation (FileStore) reads from markdown files.
// Future implementations may read from beads/Dolt.
type Store interface {
	// List returns ADRs matching the given filters.
	List(opts ListOpts) ([]ADR, error)

	// Get returns a single ADR by ID (e.g., "ADR-0003").
	Get(id string) (*ADR, error)

	// Search returns ADRs whose content matches the query string.
	Search(query string) ([]ADR, error)

	// Create creates a new ADR and returns its ID.
	Create(title string, opts CreateOpts) (string, error)

	// Supersede marks oldID as superseded by newID.
	Supersede(oldID, newID string) error
}
