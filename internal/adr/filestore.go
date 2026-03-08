package adr

import "strings"

// Compile-time interface check.
var _ Store = (*FileStore)(nil)

// FileStore implements Store by reading ADR markdown files from disk.
type FileStore struct {
	root string
}

// NewFileStore creates a FileStore rooted at the given project root.
func NewFileStore(root string) *FileStore {
	return &FileStore{root: root}
}

func (s *FileStore) List(opts ListOpts) ([]ADR, error) {
	return List(s.root, opts)
}

func (s *FileStore) Get(id string) (*ADR, error) {
	return Show(s.root, id)
}

func (s *FileStore) Search(query string) ([]ADR, error) {
	all, err := ScanADRs(s.root)
	if err != nil {
		return nil, err
	}

	q := strings.ToLower(query)
	var result []ADR
	for _, a := range all {
		if strings.Contains(strings.ToLower(a.Title), q) ||
			strings.Contains(strings.ToLower(a.Content), q) ||
			strings.Contains(strings.ToLower(a.ID), q) {
			result = append(result, a)
		}
	}
	return result, nil
}

func (s *FileStore) Create(title string, opts CreateOpts) (string, error) {
	return Create(s.root, title, opts)
}

func (s *FileStore) Supersede(oldID, newID string) error {
	return Supersede(s.root, oldID, newID)
}
