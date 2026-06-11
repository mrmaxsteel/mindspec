package adr

import "sort"

// Compile-time interface check.
var _ Store = (*OverlayStore)(nil)

// OverlayStore layers a branch-local ADR store (e.g. a spec worktree's
// .mindspec/docs/adr/) over a primary store (the main checkout). Reads
// consult the branch first: Get returns the branch hit when present,
// List/Search union-dedup by ID with branch entries winning.
//
// This closes the ADR-0022 asymmetry where workspace.SpecDir resolves
// spec files inside a spec worktree but workspace.ADRDir always points
// at the primary checkout — so a spec-introduced ADR (committed only on
// the spec branch) was invisible to plan-approve and bead-complete
// validation run from the primary checkout (mindspec-ew79).
type OverlayStore struct {
	branch  Store
	primary Store
}

// NewOverlayStore builds an OverlayStore. branch wins on ID conflicts.
func NewOverlayStore(branch, primary Store) *OverlayStore {
	return &OverlayStore{branch: branch, primary: primary}
}

// Get returns the ADR from the branch store when it resolves there,
// falling back to the primary store otherwise.
func (s *OverlayStore) Get(id string) (*ADR, error) {
	if a, err := s.branch.Get(id); err == nil {
		return a, nil
	}
	return s.primary.Get(id)
}

// List returns the union of both stores' results, deduplicated by ID
// with branch entries winning, sorted by ID for determinism.
func (s *OverlayStore) List(opts ListOpts) ([]ADR, error) {
	branch, err := s.branch.List(opts)
	if err != nil {
		return nil, err
	}
	primary, err := s.primary.List(opts)
	if err != nil {
		return nil, err
	}
	return unionByID(branch, primary), nil
}

// Search returns the union of both stores' results, deduplicated by ID
// with branch entries winning, sorted by ID for determinism.
func (s *OverlayStore) Search(query string) ([]ADR, error) {
	branch, err := s.branch.Search(query)
	if err != nil {
		return nil, err
	}
	primary, err := s.primary.Search(query)
	if err != nil {
		return nil, err
	}
	return unionByID(branch, primary), nil
}

// Create writes to the branch store: new ADRs authored while a spec
// branch is active belong on that branch, mirroring where SpecDir
// resolves the spec's own files.
func (s *OverlayStore) Create(title string, opts CreateOpts) (string, error) {
	return s.branch.Create(title, opts)
}

// Supersede applies to the branch store for the same reason as Create.
func (s *OverlayStore) Supersede(oldID, newID string) error {
	return s.branch.Supersede(oldID, newID)
}

// unionByID merges two ADR slices, dropping b-entries whose ID already
// appears in a (a wins), and returns the result sorted by ID.
func unionByID(a, b []ADR) []ADR {
	seen := make(map[string]struct{}, len(a))
	out := make([]ADR, 0, len(a)+len(b))
	for _, x := range a {
		seen[x.ID] = struct{}{}
		out = append(out, x)
	}
	for _, x := range b {
		if _, dup := seen[x.ID]; dup {
			continue
		}
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
