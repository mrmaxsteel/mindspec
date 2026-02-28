// Package specmeta is deprecated. Use state.Lifecycle instead.
// This stub exists only to keep the build green while callers are migrated in Bead 4.
package specmeta

// Meta holds spec metadata from frontmatter.
// Deprecated: use state.Lifecycle instead.
type Meta struct {
	MoleculeID  string            `yaml:"molecule_id"`
	StepMapping map[string]string `yaml:"step_mapping"`
	Status      string            `yaml:"status"`
}

// Read parses spec.md frontmatter for a spec.
// Deprecated: use state.ReadLifecycle instead.
func Read(specDir string) (*Meta, error) {
	return &Meta{}, nil
}

// EnsureFullyBound reads and validates spec metadata.
// Deprecated: use state.ReadLifecycle instead.
func EnsureFullyBound(root, specID string) (*Meta, error) {
	return &Meta{}, nil
}

// EnsureBound reads spec metadata.
// Deprecated: use state.ReadLifecycle instead.
func EnsureBound(root, specID string) (*Meta, error) {
	return &Meta{}, nil
}

// Write is a no-op stub.
// Deprecated: molecules are no longer used.
func Write(specDir string, meta *Meta) error {
	return nil
}

// ReadForSpec reads spec metadata for a spec directory.
// Deprecated: use state.ReadLifecycle instead.
func ReadForSpec(root, specID string) (*Meta, error) {
	return &Meta{}, nil
}

// Backfill is a no-op stub.
// Deprecated: molecules are no longer used.
func Backfill(root, specID, moleculeID string) error {
	return nil
}
