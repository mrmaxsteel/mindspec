// Package validate.
//
// adr_memo.go provides the per-validation-run ADR-parse memoization
// (spec 108 R8 / report §3 Perf #4). The citation and coverage lanes call
// store.Get(id) O(domains × citations) times; without memoization every
// call re-reads and re-parses the ADR markdown file. memoStore caches the
// (*ADR, error) outcome by ID so each distinct cited ADR is read from disk
// at most once per run. It lives here, in internal/validate, so
// internal/adr need not change.
package validate

import "github.com/mrmaxsteel/mindspec/internal/adr"

// adrStoreForSpecFn is the package-level seam through which the plan and
// divergence lanes obtain the base ADR store for a run (spec 108 R8). It
// defaults to adrStoreForSpec; each call site wraps the returned store in
// newMemoStore, so a test can swap this for a counting store and assert
// that a full validation run reads each distinct cited ADR from disk at
// most once.
var adrStoreForSpecFn = adrStoreForSpec

// memoStore wraps an adr.Store so each distinct ADR ID is read and parsed
// AT MOST ONCE per validation run (spec 108 R8). Get memoizes the
// (*ADR, error) outcome by the exact id string requested, so the result is
// byte-identical to the unwrapped store's for every id — every coverage,
// relevance (adr-cite-irrelevant), and supersede-chain decision is
// unchanged; only the number of disk reads drops.
//
// List, Search, Create, and Supersede pass straight through. A validation
// run performs no Create/Supersede, so the Get cache never needs
// invalidation. The returned *ADR is shared across callers within a run;
// every consumer in this package reads it (Status, Domains, ID,
// SupersededBy) without mutation, so sharing is safe. The cache is
// per-instance (one memoStore per run) and holds no cross-run state.
type memoStore struct {
	inner adr.Store
	cache map[string]memoStoreEntry
}

type memoStoreEntry struct {
	a   *adr.ADR
	err error
}

// newMemoStore wraps inner so repeat Get(id) calls within one run hit the
// cache. adrStoreForSpec always returns a non-nil store, so newMemoStore
// does not guard against a nil inner.
func newMemoStore(inner adr.Store) *memoStore {
	return &memoStore{inner: inner, cache: map[string]memoStoreEntry{}}
}

// Get returns the ADR for id, reading it from the underlying store on the
// first request for that id and returning the memoized outcome (including
// any error) thereafter.
func (m *memoStore) Get(id string) (*adr.ADR, error) {
	if e, ok := m.cache[id]; ok {
		return e.a, e.err
	}
	a, err := m.inner.Get(id)
	m.cache[id] = memoStoreEntry{a: a, err: err}
	return a, err
}

func (m *memoStore) List(opts adr.ListOpts) ([]adr.ADR, error) { return m.inner.List(opts) }

func (m *memoStore) Search(query string) ([]adr.ADR, error) { return m.inner.Search(query) }

func (m *memoStore) Create(title string, opts adr.CreateOpts) (string, error) {
	return m.inner.Create(title, opts)
}

func (m *memoStore) Supersede(oldID, newID string) error { return m.inner.Supersede(oldID, newID) }
