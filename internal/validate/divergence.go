// Package validate.
//
// divergence.go implements the bead-level ADR-divergence check (Spec 087
// Bead 2). For every changed file in the bead's diff range it asks two
// questions:
//
//  1. Is the file claimed by an OWNERSHIP.yaml manifest under one of
//     the spec's impacted domains? When no manifest claims it the file
//     is "unowned" and surfaced as `adr-divergence-unowned`.
//  2. If owned, does the plan's cited-ADR set cover the owning domain
//     (via the canonical IsDomainCovered predicate from plan.go)? When
//     not covered the file is "uncovered" and surfaced as
//     `adr-divergence-uncovered`.
//
// HC-4 layer 2: paths whose first segment is `viz/`, `agentmind/`, or
// `bench/` are dropped before attribution — those trees are out of
// doc-sync scope per the OWNERSHIP.yaml load-time check in
// ownership.go.
//
// HC-6 import discipline: this file MUST NOT import `os/exec` or
// `internal/gitutil`, and MUST NOT call `exec.Command("git"|"bd", ...)`.
// All git reads go through executor.Executor.
package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/contextpack"
	"github.com/mrmaxsteel/mindspec/internal/executor"
)

// DivergenceFinding is the structured machine-readable record of a single
// divergence detection. The string-formatted Issue messages on *Result
// remain for humans; this slice is the seam the supersede flow (Bead 3
// step 4) consumes when seeding a placeholder ADR's `Domains` field.
//
// Kind is one of:
//   - "uncovered" — file IS attributed to a domain, but no cited ADR
//     covers that domain. Domain + ManifestPath are populated.
//   - "unowned"   — no OWNERSHIP.yaml under any spec-impacted domain
//     claims the file. Domain + ManifestPath are empty.
type DivergenceFinding struct {
	Domain       string // empty when Kind == "unowned"
	Path         string
	ManifestPath string // empty when Kind == "unowned" or fallback ownership
	Kind         string // "uncovered" | "unowned"
}

// ValidateDivergence runs the ADR-divergence lane for a single bead's
// diff range. It loads spec metadata + plan frontmatter from `specDir`
// internally (revision 7 — callers do not pass citations or store).
//
// Returns:
//   - *Result with sub-command "adr-divergence" populated with one
//     Issue per finding plus any load/IO errors.
//   - []DivergenceFinding structured records (one per detection). When
//     a load step fails before attribution can run the slice is nil
//     and the *Result carries the failure as a single error.
func ValidateDivergence(
	exec executor.Executor,
	root, specDir, beadID string,
	base, head string,
) (*Result, []DivergenceFinding) {
	targetID := beadID
	if targetID == "" {
		targetID = filepath.Base(specDir)
	}
	r := &Result{SubCommand: "adr-divergence", TargetID: targetID}

	// Load spec metadata (impacted-domains list). When spec.md is
	// missing the gate degrades to a silent no-op — pre-Spec-087
	// fixtures (legacy complete/approve tests) ship without spec.md
	// and the gate must not block on absent inputs. A spec.md that
	// IS present but malformed surfaces normally.
	meta, err := contextpack.ParseSpec(specDir)
	if err != nil {
		if os.IsNotExist(err) || strings.Contains(err.Error(), "no such file") {
			return r, nil
		}
		r.AddError("adr-divergence-spec", err.Error())
		return r, nil
	}

	// Load plan frontmatter for citation list. Missing plan.md is
	// treated identically to missing spec.md — degrade to no-op so
	// legacy fixtures pass.
	planPath := filepath.Join(specDir, "plan.md")
	planBytes, err := os.ReadFile(planPath)
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil
		}
		r.AddError("adr-divergence-load", fmt.Sprintf("cannot read plan: %v", err))
		return r, nil
	}
	fm, err := parsePlanFrontmatter(string(planBytes))
	if err != nil {
		r.AddError("adr-divergence-load", fmt.Sprintf("cannot parse plan frontmatter: %v", err))
		return r, nil
	}

	store := adr.NewFileStore(root)

	// Resolve domain list to consult for attribution. Prefer the
	// spec's declared impacted-domains; when that's empty (spec has
	// no `## Impacted Domains` section) fall back to enumerating
	// every domain directory under .mindspec/docs/domains/ so a file
	// that lives outside the spec's stated scope is still surfaced as
	// owned-but-uncovered rather than misclassified as unowned.
	candidateDomains := append([]string(nil), meta.Domains...)
	if len(candidateDomains) == 0 {
		disc, derr := listDomainDirs(root)
		if derr != nil {
			r.AddError("adr-divergence-load", fmt.Sprintf("cannot list domains: %v", derr))
			return r, nil
		}
		candidateDomains = disc
	}
	sort.Strings(candidateDomains)

	// Diff range.
	changed, err := exec.ChangedFiles(base, head)
	if err != nil {
		r.AddError("adr-divergence-diff", err.Error())
		return r, nil
	}

	var findings []DivergenceFinding
	for _, path := range changed {
		if path == "" {
			continue
		}
		// HC-4 layer 2: drop excluded first-segment trees.
		seg := path
		if idx := strings.Index(path, "/"); idx >= 0 {
			seg = path[:idx]
		}
		if _, bad := excludedFirstSegments[seg]; bad {
			continue
		}

		domain, own, attrErr := attributeDomain(root, path, candidateDomains)
		if attrErr != nil {
			r.AddError("adr-divergence-attribute",
				fmt.Sprintf("attributing %s: %v", path, attrErr))
			continue
		}

		if domain == "" {
			r.AddError("adr-divergence-unowned",
				fmt.Sprintf("file %s is not claimed by any OWNERSHIP.yaml for the spec's impacted domains %v; add it to an existing manifest or create a new domain dir at .mindspec/docs/domains/<name>/OWNERSHIP.yaml",
					path, meta.Domains))
			findings = append(findings, DivergenceFinding{
				Path: path,
				Kind: "unowned",
			})
			continue
		}

		if IsDomainCovered(store, fm.ADRCitations, domain) {
			continue
		}

		manifestRef := ""
		if own != nil {
			manifestRef = own.ManifestPath
		}
		if manifestRef == "" {
			manifestRef = fmt.Sprintf("<fallback: internal/%s/**>", domain)
		}
		r.AddError("adr-divergence-uncovered",
			fmt.Sprintf("file %s attributed to domain %q (manifest: %s) but no cited ADR covers %q",
				path, domain, manifestRef, domain))
		ownManifest := ""
		if own != nil {
			ownManifest = own.ManifestPath
		}
		findings = append(findings, DivergenceFinding{
			Domain:       domain,
			Path:         path,
			ManifestPath: ownManifest,
			Kind:         "uncovered",
		})
	}

	return r, findings
}
