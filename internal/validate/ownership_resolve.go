// Package validate.
//
// ownership_resolve.go provides the shared Impacted-Domains
// normalization helper (spec 100 R1, bead mindspec-4ft2). A spec's
// `## Impacted Domains` entries may be file paths (e.g.
// `internal/genevieve/review.py`) rather than domain NAMES. The
// bead-time divergence gate and the two plan-time gates
// (checkADRCoverage, checkADRCitations) all consume those raw strings
// BY NAME, so a file-path entry never resolves and yields spurious
// `adr-divergence-unowned` / `adr-coverage-missing` / `adr-cite-irrelevant`.
//
// normalizeImpactedDomains resolves each raw entry to its owning-domain
// NAME at a SINGLE source consumed by all three gates: an entry that
// names a domain dir (`.mindspec/docs/domains/<entry>/OWNERSHIP.yaml`)
// is kept verbatim; a path-like entry is glob-matched against every
// domain's OWNERSHIP `paths:` and replaced with the owning domain's
// name; zero / more-than-one owner is a hard ERROR naming the entry.
//
// This is the ZFC-clean reading of ADR-0036 (resolution from EXPLICIT
// per-domain manifests, no synthesized fallback) and the amendment to
// ADR-0032 sub-decision 1 (path-like identifiers are normalized to
// their dir-name owner, not rejected outright).
//
// It lives in internal/validate so it reuses the existing unexported
// primitives (resolveDomains, listDomainDirs, loadOwnershipForRef,
// matchesAny/GlobMatch) with ZERO new exports and adds NO new
// cross-package edge. ParseSpec/contextpack is deliberately NOT made to
// depend on validate — normalization lives in the gate layer; the
// parser stays dumb.
package validate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// normalizeImpactedDomains resolves each raw `## Impacted Domains`
// entry to its owning-domain NAME and returns the normalized,
// de-duplicated domain-NAME set plus any resolution errors.
//
// Resolution rule per entry:
//
//  1. The entry names a domain dir (its OWNERSHIP.yaml exists under
//     .mindspec/docs/domains/<entry>/) → KEEP verbatim. No glob match
//     attempted.
//  2. The entry is a bare name token (no path separator) that does NOT
//     name a domain dir → KEEP verbatim. A plain domain name without an
//     on-disk manifest is a legacy named-domain declaration; the
//     downstream coverage gate handles its uncovered/unowned state as
//     before — normalization does not invent an error for a plain name.
//  3. The entry is path-like (contains `/`) and does NOT name a domain
//     dir → glob-match against EVERY domain's OWNERSHIP `paths:`
//     (minus Exclude). Exactly one owner → REPLACE with that owner's
//     dir-NAME; zero owners → ERROR naming the entry; more-than-one
//     owner → ambiguity ERROR naming the entry and the conflicting
//     owners.
//
// ownerRef / exec mirror attributeDomain's plumbing so the helper reads
// the same tree as the calling gate: a non-empty ownerRef reads the
// per-domain manifests and the domain enumeration from that git ref
// (the divergence path passes the bead/spec ref); "" preserves the
// on-disk working-tree read (the plan path — ValidatePlan builds no
// executor, so exec may be nil there).
//
// The returned slice preserves first-seen order of the resolved names
// and is de-duplicated case-foldingly so two raw entries resolving to
// the same owner collapse to one. The errs slice is nil when every
// entry resolves cleanly.
func normalizeImpactedDomains(exec executor.Executor, root, ownerRef string, entries []string) (normalized []string, errs []string) {
	if len(entries) == 0 {
		return nil, nil
	}

	// Enumerate domain dirs once (ref-anchored when ownerRef is set, so
	// a branch-only domain dir is discovered from the diffed ref).
	domains, derr := resolveDomains(exec, root, ownerRef)
	if derr != nil {
		return nil, []string{fmt.Sprintf("cannot enumerate domain dirs for Impacted-Domains resolution: %v", derr)}
	}
	domainSet := make(map[string]struct{}, len(domains))
	for _, d := range domains {
		domainSet[d] = struct{}{}
	}

	// Spec 108 R7: resolve every path-like entry against a single per-run
	// OWNERSHIP cache, so each domain's manifest is loaded once for the
	// whole entry set rather than once per (entry × domain).
	ownCache := newOwnershipCache(exec, root, ownerRef)

	seen := make(map[string]struct{}, len(entries))
	appendName := func(name string) {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			return
		}
		if _, dup := seen[key]; dup {
			return
		}
		seen[key] = struct{}{}
		normalized = append(normalized, name)
	}

	for _, raw := range entries {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}

		// Rule 1: entry names a domain dir → keep verbatim.
		if _, ok := domainSet[entry]; ok {
			appendName(entry)
			continue
		}

		// Rule 2: bare name token (no path separator) that is not a
		// domain dir → keep verbatim (legacy named-domain without an
		// on-disk manifest; the coverage gate reports its state).
		if !strings.Contains(entry, "/") {
			appendName(entry)
			continue
		}

		// Rule 3: path-like entry → glob-match against every domain's
		// OWNERSHIP paths: and collect ALL owners (cardinality decides
		// keep / error — attributeDomain returns first-match, so it
		// cannot surface the >1-owner ambiguity ERROR; enumerate owners
		// here instead).
		var owners []string
		var loadErr error
		for _, d := range domains {
			o, oerr := ownCache.get(d)
			if oerr != nil {
				loadErr = oerr
				break
			}
			if !matchesAny(o.Paths, entry) {
				continue
			}
			if matchesAny(o.Exclude, entry) {
				continue
			}
			owners = append(owners, d)
		}
		if loadErr != nil {
			errs = append(errs, fmt.Sprintf("resolving Impacted-Domains entry %q: %v", entry, loadErr))
			continue
		}

		switch len(owners) {
		case 0:
			errs = append(errs, fmt.Sprintf("Impacted-Domains entry %q is not claimed by any domain's OWNERSHIP.yaml paths:; name an owning domain dir or add the path to a domain's OWNERSHIP.yaml", entry))
		case 1:
			appendName(owners[0])
		default:
			sort.Strings(owners)
			// R4 (spec 120): owners are on-disk domain-dir basenames
			// (listDomainDirs -> os.ReadDir e.Name(), agent-creatable, never
			// idvalidate'd). Escape PER ELEMENT before Join — never the
			// already-joined whole — so a hostile basename can neither forge
			// a terminal line nor swallow the ", " separator into its own
			// unescaped content. termsafe.Escape is byte-identical for a
			// genuine domain name.
			safeOwners := make([]string, len(owners))
			for i, o := range owners {
				safeOwners[i] = termsafe.Escape(o)
			}
			errs = append(errs, fmt.Sprintf("Impacted-Domains entry %q is ambiguous: claimed by more than one domain's OWNERSHIP.yaml (%s); make the manifests disjoint or name a single owning domain dir", entry, strings.Join(safeOwners, ", ")))
		}
	}

	return normalized, errs
}

// bareUnresolvedImpactedDomains is a SEPARATE, signature-preserving
// helper (spec 122 R1, PF-2) that identifies the Rule-2 entries — bare
// tokens naming no domain dir (:119-125 above) — but ONLY when the
// ownership model is IN USE in this workspace: at least one enumerated
// domain dir whose OWNERSHIP.yaml actually LOADS (Ownership.ManifestPath
// != "", i.e. Source() != "missing") through the SAME per-run ownCache
// normalizeImpactedDomains uses — not the mere on-disk PRESENCE of a
// domains directory or domain sub-directory, which is the resolved
// boundary the spec draws (a scaffolded-but-empty domains tree must not
// flip this gate). When no domain's manifest loads anywhere (a
// manifest-less workspace, or every domain dir under the enumeration
// root has no OWNERSHIP.yaml on disk), this returns nil — Rule 2's
// verbatim-keep, no-error carve-out is preserved exactly (ADR-0036's
// manifest-less doctrine, carve-out (ii)).
//
// It does NOT change normalizeImpactedDomains's 2-return contract or
// touch its 4 existing production call sites (plan.go:143, spec.go's
// checkImpactedDomainsResolutionParity, ownership.go's
// ResolveCandidateDomains, divergence.go:155): it independently
// re-enumerates domains and re-scans entries using the identical Rule-1 /
// Rule-2 predicate (a domain-dir name is never "bare"; a path-like entry
// with "/" is Rule-3 territory, not Rule 2). The returned entries are
// VERBATIM (trimmed, first-seen order, case-fold de-duplicated) —
// unescaped; callers apply termsafe.Escape when rendering, per the
// existing discipline at this call site.
//
// The caller decides severity: only the two AUTHORING consumers
// (checkImpactedDomainsResolutionParity in spec.go, ValidatePlan in
// plan.go) invoke this helper, and only promote a returned entry to an
// ERROR when the SPEC's own frontmatter status (validate.SpecStatusAt,
// NOT plan.go's isApproved which reads the PLAN's status) is an explicit
// case-folded "Draft" — every other status (Approved, any other explicit
// non-Draft value, or empty because no frontmatter / no status: key) is
// grandfathered and this helper's result is simply not surfaced.
func bareUnresolvedImpactedDomains(exec executor.Executor, root, ownerRef string, entries []string) []string {
	if len(entries) == 0 {
		return nil
	}

	domains, derr := resolveDomains(exec, root, ownerRef)
	if derr != nil || len(domains) == 0 {
		return nil
	}
	domainSet := make(map[string]struct{}, len(domains))
	for _, d := range domains {
		domainSet[d] = struct{}{}
	}

	// Ownership-model-in-use predicate: at least one domain dir whose
	// manifest actually LOADS (ManifestPath set), not merely a domain dir
	// that exists with no OWNERSHIP.yaml on disk (LoadOwnership returns a
	// non-error, ManifestPath=="" Ownership for a missing manifest file —
	// that is NOT "in use").
	ownCache := newOwnershipCache(exec, root, ownerRef)
	modelInUse := false
	for _, d := range domains {
		o, err := ownCache.get(d)
		if err == nil && o != nil && o.ManifestPath != "" {
			modelInUse = true
			break
		}
	}
	if !modelInUse {
		return nil
	}

	var bare []string
	seen := make(map[string]struct{}, len(entries))
	for _, raw := range entries {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		if _, ok := domainSet[entry]; ok {
			continue // Rule 1: names a domain dir.
		}
		if strings.Contains(entry, "/") {
			continue // Rule 3 territory: path-like, not a bare Rule-2 token.
		}
		key := strings.ToLower(entry)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		bare = append(bare, entry)
	}
	return bare
}

// impactedDomainsForwardOnlyErrors renders Requirement 1's forward-only
// error text for each Rule-2-bare entry bareUnresolvedImpactedDomains
// returned: the entry verbatim (termsafe-escaped per the existing
// discipline at this site — the fl91/SessionStart lesson), why it failed,
// the sorted + per-element-escaped list of available domain-dir names,
// and both working remedies (replace the label with one of those names,
// or declare a claimed path under the layout-aware domains root). One
// message per entry, matching the shape of the sibling Rule-3
// zero/multi-owner errors above.
func impactedDomainsForwardOnlyErrors(exec executor.Executor, root, ownerRef string, bare []string) []string {
	if len(bare) == 0 {
		return nil
	}

	domains, _ := resolveDomains(exec, root, ownerRef)
	sortedDomains := append([]string(nil), domains...)
	sort.Strings(sortedDomains)
	safeDomains := make([]string, len(sortedDomains))
	for i, d := range sortedDomains {
		safeDomains[i] = termsafe.Escape(d)
	}
	domainsList := strings.Join(safeDomains, ", ")
	rootLabel := domainsRootLabel(root)

	msgs := make([]string, 0, len(bare))
	for _, entry := range bare {
		msgs = append(msgs, fmt.Sprintf(
			"Impacted-Domains entry %s names neither a domain-dir name nor a path claimed by any domain's OWNERSHIP.yaml paths: (available domain dirs: %s); fix by replacing the entry with one of those names, or by declaring a claimed path instead (claim it in %s/<name>/OWNERSHIP.yaml if not yet claimed)",
			termsafe.Escape(entry), domainsList, rootLabel))
	}
	return msgs
}
