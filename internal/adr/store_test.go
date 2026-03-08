package adr

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeADR(t *testing.T, root, id, title, status string, domains []string) {
	t.Helper()
	adrDir := filepath.Join(root, ".mindspec", "docs", "adr")
	os.MkdirAll(adrDir, 0o755)

	content := "# " + id + ": " + title + "\n\n"
	content += "- **Status**: " + status + "\n"
	content += "- **Domain(s)**: " + joinDomains(domains) + "\n"
	content += "- **Supersedes**: n/a\n"
	content += "- **Superseded-by**: n/a\n\n"
	content += "## Decision\nSome decision text.\n"

	os.WriteFile(filepath.Join(adrDir, id+".md"), []byte(content), 0o644)
}

func joinDomains(domains []string) string {
	if len(domains) == 0 {
		return "core"
	}
	s := ""
	for i, d := range domains {
		if i > 0 {
			s += ", "
		}
		s += d
	}
	return s
}

// TestFileStore_InterfaceSatisfaction is a compile-time check (var _ Store above),
// but this test also exercises the basic operations.
func TestFileStore_List(t *testing.T) {
	root := t.TempDir()
	writeADR(t, root, "ADR-0001", "First", "Accepted", []string{"core"})
	writeADR(t, root, "ADR-0002", "Second", "Proposed", []string{"workflow"})

	store := NewFileStore(root)

	// List all
	all, err := store.List(ListOpts{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 ADRs, got %d", len(all))
	}

	// Filter by status
	accepted, err := store.List(ListOpts{Status: "Accepted"})
	if err != nil {
		t.Fatalf("List(Accepted): %v", err)
	}
	if len(accepted) != 1 {
		t.Errorf("expected 1 Accepted ADR, got %d", len(accepted))
	}

	// Filter by domain
	workflow, err := store.List(ListOpts{Domain: "workflow"})
	if err != nil {
		t.Fatalf("List(workflow): %v", err)
	}
	if len(workflow) != 1 {
		t.Errorf("expected 1 workflow ADR, got %d", len(workflow))
	}
}

func TestFileStore_Get(t *testing.T) {
	root := t.TempDir()
	writeADR(t, root, "ADR-0001", "Test ADR", "Accepted", []string{"core"})

	store := NewFileStore(root)

	a, err := store.Get("ADR-0001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if a.Title != "Test ADR" {
		t.Errorf("expected title 'Test ADR', got %q", a.Title)
	}
	if a.Status != "Accepted" {
		t.Errorf("expected status 'Accepted', got %q", a.Status)
	}

	// Missing ADR
	_, err = store.Get("ADR-9999")
	if err == nil {
		t.Error("expected error for missing ADR")
	}
}

func TestFileStore_Search(t *testing.T) {
	root := t.TempDir()
	writeADR(t, root, "ADR-0001", "Worktree Management", "Accepted", []string{"gitops"})
	writeADR(t, root, "ADR-0002", "Bead Lifecycle", "Accepted", []string{"workflow"})

	store := NewFileStore(root)

	results, err := store.Search("worktree")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "ADR-0001" {
		t.Errorf("expected ADR-0001, got %s", results[0].ID)
	}

	// Case insensitive
	results, err = store.Search("BEAD")
	if err != nil {
		t.Fatalf("Search(BEAD): %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for case-insensitive search, got %d", len(results))
	}

	// No match
	results, err = store.Search("nonexistent")
	if err != nil {
		t.Fatalf("Search(nonexistent): %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// --- MemoryStore: in-memory Store for testing swappability ---

// MemoryStore implements Store with in-memory data for testing.
type MemoryStore struct {
	adrs []ADR
}

var _ Store = (*MemoryStore)(nil)

func NewMemoryStore(adrs []ADR) *MemoryStore {
	return &MemoryStore{adrs: adrs}
}

func (m *MemoryStore) List(opts ListOpts) ([]ADR, error) {
	var result []ADR
	for _, a := range m.adrs {
		if opts.Status != "" && !strings.EqualFold(a.Status, opts.Status) {
			continue
		}
		if opts.Domain != "" {
			found := false
			target := strings.ToLower(opts.Domain)
			for _, d := range a.Domains {
				if strings.ToLower(d) == target {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		result = append(result, a)
	}
	return result, nil
}

func (m *MemoryStore) Get(id string) (*ADR, error) {
	for _, a := range m.adrs {
		if a.ID == id {
			return &a, nil
		}
	}
	return nil, fmt.Errorf("%s not found", id)
}

func (m *MemoryStore) Search(query string) ([]ADR, error) {
	q := strings.ToLower(query)
	var result []ADR
	for _, a := range m.adrs {
		if strings.Contains(strings.ToLower(a.Title), q) ||
			strings.Contains(strings.ToLower(a.Content), q) {
			result = append(result, a)
		}
	}
	return result, nil
}

func (m *MemoryStore) Create(title string, opts CreateOpts) (string, error) {
	return "", fmt.Errorf("MemoryStore.Create not implemented")
}

func (m *MemoryStore) Supersede(oldID, newID string) error {
	return fmt.Errorf("MemoryStore.Supersede not implemented")
}

func TestMemoryStore_List(t *testing.T) {
	store := NewMemoryStore([]ADR{
		{ID: "ADR-0001", Title: "First", Status: "Accepted", Domains: []string{"core"}},
		{ID: "ADR-0002", Title: "Second", Status: "Proposed", Domains: []string{"workflow"}},
	})

	all, err := store.List(ListOpts{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2, got %d", len(all))
	}

	accepted, err := store.List(ListOpts{Status: "Accepted"})
	if err != nil {
		t.Fatalf("List(Accepted): %v", err)
	}
	if len(accepted) != 1 {
		t.Errorf("expected 1, got %d", len(accepted))
	}
}

func TestMemoryStore_Get(t *testing.T) {
	store := NewMemoryStore([]ADR{
		{ID: "ADR-0001", Title: "Test", Status: "Accepted"},
	})

	a, err := store.Get("ADR-0001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if a.Title != "Test" {
		t.Errorf("expected title 'Test', got %q", a.Title)
	}

	_, err = store.Get("ADR-9999")
	if err == nil {
		t.Error("expected error for missing ADR")
	}
}

func TestMemoryStore_Search(t *testing.T) {
	store := NewMemoryStore([]ADR{
		{ID: "ADR-0001", Title: "Worktree Management", Content: "## Decision\nUse worktrees."},
		{ID: "ADR-0002", Title: "Bead Lifecycle", Content: "## Decision\nTrack beads."},
	})

	results, err := store.Search("worktree")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 || results[0].ID != "ADR-0001" {
		t.Errorf("expected ADR-0001, got %v", results)
	}
}

// TestMemoryStore_Swappable proves any function accepting Store works with MemoryStore.
func TestMemoryStore_Swappable(t *testing.T) {
	var store Store = NewMemoryStore([]ADR{
		{ID: "ADR-0001", Title: "Test ADR", Status: "Superseded", SupersededBy: "ADR-0002"},
	})

	// Use the store through the interface — same operations consumers use
	a, err := store.Get("ADR-0001")
	if err != nil {
		t.Fatalf("Get via interface: %v", err)
	}
	if a.Status != "Superseded" {
		t.Errorf("expected Superseded, got %q", a.Status)
	}
	if a.SupersededBy != "ADR-0002" {
		t.Errorf("expected SupersededBy ADR-0002, got %q", a.SupersededBy)
	}
}

func TestFileStore_EmptyDirectory(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".mindspec", "docs", "adr"), 0o755)

	store := NewFileStore(root)

	all, err := store.List(ListOpts{})
	if err != nil {
		t.Fatalf("List on empty dir: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected 0 ADRs, got %d", len(all))
	}
}
