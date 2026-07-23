// Package validate.
//
// adr_domain_resolve.go implements Requirement 2 of spec 122
// (domain-adr-gate-truthfulness; GH #147/#145 + bead mindspec-6ou2):
// resolving the ADR side of every coverage comparison through the SAME
// deterministic explicit-manifest owner resolution the spec side
// already gets via normalizeImpactedDomains (spec 100 R1). Before this,
// a cited ADR's `Domain(s)` line was compared to the spec's resolved
// Impacted-Domains set by literal string equality — so an ADR writing
// a directory path (`src/orders/` or `src/orders`) never intersected
// the spec-resolved name `orders`, producing the spurious
// `adr-cite-irrelevant` / `adr-coverage-missing` pair that is bead
// mindspec-6ou2's filed items 3 and 4.
//
// This is documented in full at ADR-0032's third `## Amendment`
// section §(b) ("Symmetric comparison by name resolution"), finalized
// in spec 122 Bead 1 alongside the evidenced supersession of bead
// mindspec-6ou2's 6/6 panel decision (2026-06-26): the panel rejected
// "resolve-ADR-side-through-OWNERSHIP by name" on a ZFC/guessed-
// ownership objection that does not apply here, because resolution is
// restricted to DETERMINISTIC path-shaped entries glob-matched against
// EXPLICIT per-domain OWNERSHIP `paths:` — the identical mechanism the
// spec side already uses. A tuple/prose token (e.g. this repo's own
// ADR-0032 line `validation, adr, lifecycle, workflow`) is never parsed
// or guessed; it stays verbatim and compares literally, exactly as
// before. This is the ADR-side no-new-error doctrine: resolution can
// only ever help an intersection succeed — it never introduces a new
// failure, because ADR `Domain(s)` lines are historical documents this
// gate must not force churn on.
package validate

import (
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/executor"
)

// domainResolvingStore decorates an adr.Store so every ADR it returns
// has its Domains resolved through the deterministic explicit-manifest
// mechanism described above, before any comparison site
// (checkADRCitations' intersectFold, checkADRCoverage's coverageOf /
// the exported IsDomainCovered, and the bead-time divergence probe)
// ever sees it. Layering the resolution at the store — rather than
// threading a resolve-at-comparison helper into each of those call
// shapes — means every comparison site is fed with ZERO signature
// churn and none can silently miss the resolution step.
//
// Domain enumeration and the per-domain OWNERSHIP cache are built ONCE
// at construction, from the SAME exec/root/ownerRef the calling lane
// already resolved (nil/""/"" for the plan-time working-tree read;
// the divergence lane's own exec + ref-anchored ownerRef otherwise —
// so both comparison sides always see the same tree via one
// newOwnershipCache, mirroring normalizeImpactedDomains's plumbing).
// Resolved Domains are additionally memoized per ADR ID for the
// lifetime of this decorator instance — a non-observable performance
// choice layered beside the existing per-run newMemoStore cache.
type domainResolvingStore struct {
	inner     adr.Store
	domains   []string
	domainSet map[string]string // lower(name) -> canonical on-disk dir name
	ownCache  *ownershipCache
	resolved  map[string][]string // ADR ID -> resolved Domains, memoized
}

// newDomainResolvingStore builds the decorator described above. It is
// layered at the two gate-lane store constructions (ValidatePlan,
// ValidateDivergence) — deliberately NOT at the cmd-side adrReadStore
// (cmd/mindspec/adr.go), so `adr show`/`adr list` keep rendering the
// author's literal `Domain(s)` line unchanged (AC-12's user-facing
// verbs stay truthful to the document as written).
func newDomainResolvingStore(inner adr.Store, exec executor.Executor, root, ownerRef string) adr.Store {
	domains, _ := resolveDomains(exec, root, ownerRef)
	domainSet := make(map[string]string, len(domains))
	for _, d := range domains {
		domainSet[strings.ToLower(d)] = d
	}
	return &domainResolvingStore{
		inner:     inner,
		domains:   domains,
		domainSet: domainSet,
		ownCache:  newOwnershipCache(exec, root, ownerRef),
		resolved:  map[string][]string{},
	}
}

// resolveDomainsOf returns a's Domains with every entry passed through
// resolveEntry, memoized per ADR ID so repeat Get/List calls for the
// same ADR within one run resolve each entry at most once.
func (s *domainResolvingStore) resolveDomainsOf(a *adr.ADR) []string {
	if a == nil {
		return nil
	}
	if cached, ok := s.resolved[a.ID]; ok {
		return cached
	}
	out := make([]string, len(a.Domains))
	for i, entry := range a.Domains {
		out[i] = s.resolveEntry(entry)
	}
	s.resolved[a.ID] = out
	return out
}

// resolveEntry resolves a single ADR-side `Domain(s)` entry, mirroring
// normalizeImpactedDomains's Rule 1 (domain-dir name stays verbatim)
// and Rule 3 (path-shaped entry claimed by exactly one domain's
// OWNERSHIP paths: resolves to that owner's dir-name), with directory-
// shape completeness and NO error path:
//
//   - Rule 1: the entry, trimmed of a trailing slash, names a domain
//     dir → return the canonical on-disk name (case-insensitive
//     lookup, since the ADR parser already case-folds Domain(s)
//     tokens to lower-case).
//
//   - Non-path entries (no "/") are tuple/prose tokens (e.g. "adr",
//     "lifecycle", "api (lola, tools)") → returned EXACTLY as
//     authored, untouched.
//
//   - Path-shaped entries are resolved in TWO ordered phases, so a
//     clean literal resolution always wins over the directory-shape
//     fallback and neither phase can be corrupted by the other:
//
//     Phase 1 (literal): glob-match the trimmed label DIRECTLY against
//     every domain's OWNERSHIP paths: (honoring exclude:) — this
//     matches a bare file path ("src/orders/api.py"), and a directory
//     glob such as "src/orders/**" against the WITH-trailing-slash form
//     once trimmed. If ANY enumerated domain's manifest fails to load,
//     cardinality is UNKNOWABLE, so the entry stays literal (G-2: an
//     indeterminate resolution is never promoted — the no-new-error
//     doctrine makes staying literal always safe). Exactly one literal
//     owner resolves and returns IMMEDIATELY, never consulting the
//     child probe (G-3: a clean unique literal owner is authoritative
//     and must not be polluted by a coincidental child match); more
//     than one literal owner is ambiguous and stays literal.
//
//     Phase 2 (directory-shape fallback): ONLY when the literal token
//     has ZERO owners, run the synthetic child probe "<trimmed>/x" so a
//     directory label written WITHOUT a trailing slash ("src/orders")
//     still matches a "src/orders/**" glob, which does not match the
//     bare directory string itself. A domain that EXCLUDES the literal
//     trimmed label must NOT be resurrected by the probe (G-1: the
//     exclusion is honored against the DECLARED label, not the
//     synthetic child), and a mid-fallback load error is likewise
//     indeterminate and stays literal. Exactly one fallback owner
//     resolves; zero or more than one stays literal.
//
//     Across both phases, zero / ambiguous / indeterminate leaves the
//     entry EXACTLY as authored — no error, ever (the ADR-side
//     no-new-error doctrine).
func (s *domainResolvingStore) resolveEntry(entry string) string {
	trimmed := strings.TrimSuffix(strings.TrimSpace(entry), "/")
	if trimmed == "" {
		return entry
	}

	if canon, ok := s.domainSet[strings.ToLower(trimmed)]; ok {
		return canon
	}

	if !strings.Contains(trimmed, "/") {
		return entry
	}

	// Phase 1: resolve the LITERAL token, tracking load errors.
	var owners []string
	loadErr := false
	for _, d := range s.domains {
		o, err := s.ownCache.get(d)
		if err != nil {
			loadErr = true
			continue
		}
		if o == nil {
			continue
		}
		if ownerClaims(o, trimmed) {
			owners = append(owners, d)
		}
	}
	// G-2: any enumerated domain failed to load → cardinality unknowable,
	// stay literal. Checked BEFORE trusting len(owners)==1.
	if loadErr {
		return entry
	}
	// G-3: a clean unique literal owner is authoritative — return
	// immediately, never consult the child probe.
	if len(owners) == 1 {
		return owners[0]
	}
	if len(owners) > 1 {
		return entry // ambiguous
	}

	// Phase 2: zero literal owners → directory-completeness fallback.
	// The probe is a synthetic child so a slashless dir label matches a
	// /** glob; G-1: a domain that excludes the DECLARED label must not
	// be resurrected by the child match.
	probe := trimmed + "/x"
	for _, d := range s.domains {
		o, err := s.ownCache.get(d)
		if err != nil {
			return entry // load error mid-fallback → indeterminate
		}
		if o == nil {
			continue
		}
		if ownerClaims(o, probe) && !matchesAny(o.Exclude, trimmed) {
			owners = append(owners, d)
		}
	}
	if len(owners) == 1 {
		return owners[0]
	}
	return entry
}

// ownerClaims reports whether o's OWNERSHIP paths: (minus exclude:)
// claim path — the same matchesAny/Exclude mechanics
// normalizeImpactedDomains's Rule 3 uses for the spec side.
func ownerClaims(o *Ownership, path string) bool {
	return matchesAny(o.Paths, path) && !matchesAny(o.Exclude, path)
}

// Get returns id's ADR with Domains resolved. The inner ADR is copied
// (not mutated) before its Domains field is replaced, so a caller
// holding a reference to an inner store's own cached *adr.ADR (e.g.
// memoStore's cache, which this decorator wraps) never observes the
// resolved rewrite.
func (s *domainResolvingStore) Get(id string) (*adr.ADR, error) {
	a, err := s.inner.Get(id)
	if err != nil || a == nil {
		return a, err
	}
	out := *a
	out.Domains = s.resolveDomainsOf(a)
	return &out, nil
}

// List returns opts-matching ADRs with Domains resolved, same
// non-mutating-copy discipline as Get.
func (s *domainResolvingStore) List(opts adr.ListOpts) ([]adr.ADR, error) {
	list, err := s.inner.List(opts)
	if err != nil {
		return nil, err
	}
	out := make([]adr.ADR, len(list))
	for i := range list {
		a := list[i]
		a.Domains = s.resolveDomainsOf(&a)
		out[i] = a
	}
	return out, nil
}

// Search returns query-matching ADRs with Domains resolved, same
// non-mutating-copy discipline as Get.
func (s *domainResolvingStore) Search(query string) ([]adr.ADR, error) {
	list, err := s.inner.Search(query)
	if err != nil {
		return nil, err
	}
	out := make([]adr.ADR, len(list))
	for i := range list {
		a := list[i]
		a.Domains = s.resolveDomainsOf(&a)
		out[i] = a
	}
	return out, nil
}

// Create delegates unchanged: this decorator only reshapes read paths.
func (s *domainResolvingStore) Create(title string, opts adr.CreateOpts) (string, error) {
	return s.inner.Create(title, opts)
}

// Supersede delegates unchanged: this decorator only reshapes read paths.
func (s *domainResolvingStore) Supersede(oldID, newID string) error {
	return s.inner.Supersede(oldID, newID)
}
