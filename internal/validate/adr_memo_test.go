package validate

import (
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/adr"
)

// countingADRStore wraps an adr.Store and tallies Get calls per ID so a
// test can assert that memoization reduces each distinct cited ADR to a
// single disk read per validation run (spec 108 R8).
type countingADRStore struct {
	inner adr.Store
	gets  map[string]int
}

func (c *countingADRStore) Get(id string) (*adr.ADR, error) {
	c.gets[id]++
	return c.inner.Get(id)
}

func (c *countingADRStore) List(opts adr.ListOpts) ([]adr.ADR, error) { return c.inner.List(opts) }
func (c *countingADRStore) Search(q string) ([]adr.ADR, error)        { return c.inner.Search(q) }
func (c *countingADRStore) Create(title string, opts adr.CreateOpts) (string, error) {
	return c.inner.Create(title, opts)
}
func (c *countingADRStore) Supersede(oldID, newID string) error {
	return c.inner.Supersede(oldID, newID)
}

// TestADRParsedOncePerValidationRun proves spec 108 R8: with the memoizing
// decorator in place, each distinct cited ADR is read from the underlying
// store at most once per run, even though the divergence coverage loop
// probes it once per changed file. Injected through the adrStoreForSpecFn
// seam, which ValidateDivergence wraps in newMemoStore. RED-on-revert:
// dropping the newMemoStore wrap re-reads ADR-0201 three times and
// ADR-0202 twice for this fixture.
func TestADRParsedOncePerValidationRun(t *testing.T) {
	root, specDir, mock := diagCachingFixture(t)

	counting := &countingADRStore{gets: map[string]int{}}
	orig := adrStoreForSpecFn
	adrStoreForSpecFn = func(r, sd string) adr.Store {
		counting.inner = orig(r, sd)
		return counting
	}
	t.Cleanup(func() { adrStoreForSpecFn = orig })

	ValidateDivergence(mock, root, specDir, "mindspec-r8.1", "BASE", "HEAD", "", false)

	for _, id := range []string{"ADR-0201", "ADR-0202"} {
		switch {
		case counting.gets[id] == 0:
			t.Errorf("cited ADR %s never read; fixture did not exercise the coverage path", id)
		case counting.gets[id] > 1:
			t.Errorf("cited ADR %s read %d times per run; want at most 1 (memoized)", id, counting.gets[id])
		}
	}
}
