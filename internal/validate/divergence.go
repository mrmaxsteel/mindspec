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
// implApprove selects the severity for Proposed-only coverage
// (mindspec-53qx panel condition C1 — the tolerance must reconcile):
//   - false (bead-complete lane): a domain covered only by a cited
//     Proposed ADR emits an advisory `adr-divergence-proposed` WARNING.
//     Mid-implementation Proposed is the legitimate state the
//     tri-state coverage exists to protect, so it never blocks here.
//   - true (impl-approve backstop): the same condition is an ERROR —
//     the implementation is shipping, so the Proposed ADR it validates
//     must be flipped to Accepted now; the existing --override-adr /
//     --supersede-adr flags remain the recorded escape hatch. Without
//     this the lifecycle loop would never close: Proposed-covered
//     would pass silently at every gate forever.
//
// Returns:
//   - *Result with sub-command "adr-divergence" populated with one
//     Issue per finding plus any load/IO errors.
//   - []DivergenceFinding structured records (one per detection). When
//     a load step fails before attribution can run the slice is nil
//     and the *Result carries the failure as a single error.
//     Proposed-only coverage does NOT produce a finding: the findings
//     slice seeds supersede placeholders for genuinely uncovered
//     domains, and a Proposed-covered domain already has its ADR.
//
// ownerRef (spec 095 / mindspec-vvs9) selects the tree the OWNERSHIP
// attribution input — both the per-domain manifests and the
// empty-impacted-domains directory enumeration — is read from. It is
// INDEPENDENT of base/head: the diff range and the attribution tree are
// separate inputs. A non-empty ownerRef reads attribution from that git
// ref (the per-bead lane passes beadHead; impl approve passes the
// spec-branch tip); "" preserves the on-disk working-tree read.
func ValidateDivergence(
	exec executor.Executor,
	root, specDir, beadID string,
	base, head, ownerRef string,
	implApprove bool,
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

	// mindspec-ew79: overlay the spec branch's ADR dir (the tree
	// specDir lives in, e.g. a spec worktree) over the primary
	// checkout, so spec-introduced ADRs count at bead-complete time.
	store := adrStoreForSpec(root, specDir)

	// Resolve domain list to consult for attribution. Prefer the
	// spec's declared impacted-domains; when that's empty (spec has
	// no `## Impacted Domains` section) fall back to enumerating
	// every domain directory under .mindspec/docs/domains/ so a file
	// that lives outside the spec's stated scope is still surfaced as
	// owned-but-uncovered rather than misclassified as unowned.
	//
	// Spec 100 R1 (mindspec-4ft2): normalize each declared
	// Impacted-Domains entry to its owning-domain NAME at the SHARED
	// helper before it becomes the candidate set. A file-path entry
	// (e.g. internal/genevieve/review.py) is resolved to its owner via
	// the per-domain OWNERSHIP paths: globs; a zero/multi-owner entry is
	// a hard error. The candidate set stays the resolved DECLARED
	// domains, so the per-file attributeDomain loop + blast-radius guard
	// below are UNCHANGED: a changed file owned by a domain NOT in the
	// resolved declared set still fails.
	normalized, normErrs := normalizeImpactedDomains(exec, root, ownerRef, meta.Domains)
	if len(normErrs) > 0 {
		for _, e := range normErrs {
			r.AddError("impacted-domains-resolve", e)
		}
		return r, nil
	}
	candidateDomains := normalized
	if len(candidateDomains) == 0 {
		// Ref-anchored enumeration (spec 095): a branch-only domain dir
		// must be discovered from the diffed ref, not the ambient root.
		disc, derr := resolveDomains(exec, root, ownerRef)
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

		// Spec 092 fix-up: skip non-source process artifacts before
		// attribution. The lane previously iterated EVERY changed
		// file, so beads JSONL snapshots, ADR/spec/domain docs under
		// .mindspec/docs/**, and review-panel notes were flagged as
		// "unowned" — claiming them in an OWNERSHIP.yaml would be
		// semantically wrong (they are the process record, not owned
		// source).
		if isProcessArtifact(path) {
			continue
		}

		domain, own, attrErr := attributeDomain(exec, root, ownerRef, path, candidateDomains)
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

		// mindspec-53qx + panel condition C1: tri-state probe instead of
		// the bool predicate, so Proposed-only coverage is surfaced
		// rather than passing silently at every gate.
		cov, proposedID := coverageOf(nil, store, fm.ADRCitations, domain)
		if cov == coveredAccepted {
			continue
		}
		if cov == coveredProposedOnly {
			if implApprove {
				r.AddError("adr-divergence-proposed",
					fmt.Sprintf("file %s attributed to domain %q is covered only by Proposed ADR %s — flip it to Accepted now that the implementation ships, or re-run with --override-adr \"<reason>\"",
						path, domain, proposedID))
			} else {
				r.AddWarning("adr-divergence-proposed",
					fmt.Sprintf("file %s attributed to domain %q is covered only by Proposed ADR %s — flip it to Accepted before impl approve",
						path, domain, proposedID))
			}
			// No DivergenceFinding: the supersede-placeholder seed is
			// for genuinely uncovered domains; this one has its ADR.
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

// isProcessArtifact reports whether path is a non-source process
// artifact the ADR-divergence lane must skip before domain
// attribution: documentation (doc-sync's isDocFile set, which covers
// every layout's lifecycle docs — ADRs, specs, domain docs, OWNERSHIP
// manifests — plus docs/, project-docs/, CLAUDE.md, AGENTS.md), the
// beads JSONL build artifact tree (.beads/, ADR-0025), and review-panel
// working notes (both the historical root review/** tree and the
// post-flatten co-located <spec-dir>/reviews/** tree). Mirrors doc-sync's
// classification so the two gates agree on what counts as governable source.
//
// Spec 106 Req 6/14: the project-docs/** dogfood-eviction tree is non-source
// (via isDocFile) so eviction trips neither the doc-sync source lane nor
// adr-divergence-unowned; both review matchers below classify non-source.
func isProcessArtifact(path string) bool {
	return isDocFile(path) ||
		strings.HasPrefix(path, ".beads/") ||
		strings.HasPrefix(path, "review/") ||
		isCoLocatedReview(path)
}

// isCoLocatedReview reports whether path is a co-located reviews artifact —
// any path carrying a `/reviews/` segment, e.g.
// .mindspec/specs/<id>/reviews/<slug>/panel.json (spec 106 Req 6). This is an
// INDEPENDENT matcher additive to the PERMANENT root review/ exclusion in
// isProcessArtifact: the literal "review/" does NOT substring-match
// "reviews/", so the two never collapse. Both must classify non-source so
// that, during the transition (the root review/** tree still live until the
// move bead migrates it, co-located <spec-dir>/reviews/** appearing after),
// neither reads as source/unowned and trips a gate. The root matcher is a
// PERMANENT historical-ref compatibility matcher — historical refs and
// external forks emit the root review/ path forever.
func isCoLocatedReview(path string) bool {
	return strings.Contains(path, "/reviews/")
}
