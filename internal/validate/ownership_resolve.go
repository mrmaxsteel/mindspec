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
			errs = append(errs, fmt.Sprintf("Impacted-Domains entry %q is ambiguous: claimed by more than one domain's OWNERSHIP.yaml (%s); make the manifests disjoint or name a single owning domain dir", entry, strings.Join(owners, ", ")))
		}
	}

	return normalized, errs
}
