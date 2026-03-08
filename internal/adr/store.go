package adr

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
