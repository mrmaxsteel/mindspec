package lifecycle

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

type reviewedSHAClass string

const (
	reviewedSHAEmpty       reviewedSHAClass = "empty"
	reviewedSHAMalformed   reviewedSHAClass = "malformed"
	reviewedSHAObjectID    reviewedSHAClass = "object-id"
	reviewedSHAPlaceholder reviewedSHAClass = "placeholder"
)

// Classify before any git call. Whitespace and control bytes are malformed,
// including leading/trailing whitespace; trimming would hide that provenance.
func classifyReviewedSHA(sha string) reviewedSHAClass {
	switch {
	case sha == "":
		return reviewedSHAEmpty
	case strings.HasPrefix(sha, "-"), strings.ContainsAny(sha, "^~:"), strings.Contains(sha, "@{"), strings.Contains(sha, ".."), strings.ContainsFunc(sha, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }):
		return reviewedSHAMalformed
	case wellFormedGitObjectID(sha):
		return reviewedSHAObjectID
	default:
		return reviewedSHAPlaceholder
	}
}

func ownedMergeCandidates(merges []gitutil.MergeCommit, beadBranch string) []gitutil.MergeCommit {
	var candidates []gitutil.MergeCommit
	for _, m := range merges {
		if name, present := parseMergeSubjectBeadBranch(m.Subject); len(m.Parents) == 2 && present && name == beadBranch {
			candidates = append(candidates, m)
		}
	}
	return candidates
}

// Source position and an explicit target are checked exactly. In particular,
// parseMergeSubjectBeadBranch's any-token fallback is not an ownership rule here.
func reachableCandidates(root, specBranch, beadBranch string) ([]gitutil.MergeCommit, error) {
	merges, err := reachableMergesFn(root, specBranch)
	if err != nil {
		return nil, fmt.Errorf("scanning reachable merges on %s: %w", termsafe.Escape(specBranch), err)
	}
	var candidates []gitutil.MergeCommit
	for _, m := range merges {
		if len(m.Parents) == 2 && (m.Subject == "Merge "+beadBranch ||
			m.Subject == "Merge branch '"+beadBranch+"' into "+specBranch ||
			m.Subject == "Merge "+beadBranch+" into "+specBranch) {
			candidates = append(candidates, m)
		}
	}
	return candidates, nil
}

// fallbackSafety re-derives F3 and F4 from git at the CURRENT tip. F1 is
// checked by each caller before selecting m. All read failures (F5) stay errors.
var (
	fallbackBranchTipFn      = resolveBranchTip
	fallbackForwardContentFn = func(root, base, head, target string) (gitutil.Subsumption, error) {
		return landedContentSubsumedFn(root, base, head, target)
	}
	fallbackReverseContentFn = func(root, head, target string) (bool, error) {
		return landedRevertShapeFn(root, head, target)
	}
)

func fallbackSafety(root, specBranch, beadBranch string, m gitutil.MergeCommit) error {
	tip, survives, err := fallbackBranchTipFn(root, beadBranch)
	if err != nil {
		return fmt.Errorf("resolving surviving branch %s: %s", termsafe.Escape(beadBranch), termsafe.Escape(err.Error()))
	}
	if survives && tip != m.Parents[1] {
		return fmt.Errorf("%w: surviving branch %s tip %s contradicts merge %s's second parent %s", ErrLandedMergeContradiction, termsafe.Escape(beadBranch), termsafe.Escape(tip), m.SHA, m.Parents[1])
	}
	outcome, err := fallbackForwardContentFn(root, m.Parents[0], m.SHA, specBranch)
	if err != nil {
		return fmt.Errorf("checking net effect of merge %s: %s", termsafe.Escape(m.SHA), termsafe.Escape(err.Error()))
	}
	if outcome == gitutil.SubsumptionCleanDivergence {
		reverted, err := fallbackReverseContentFn(root, m.SHA, specBranch)
		if err != nil {
			return fmt.Errorf("checking revert shape of merge %s: %s", termsafe.Escape(m.SHA), termsafe.Escape(err.Error()))
		}
		if reverted {
			return fmt.Errorf("%w: merge %s was reverted or cleanly removed after landing at %s's current tip", ErrLandedMergeReverted, m.SHA, termsafe.Escape(specBranch))
		}
	}
	return nil
}

// auditMatches is the single validity and equality contract shared by the
// automatic read-back and the validity-qualified convergent no-op.
func auditMatches(meta map[string]interface{}, specBranch, merge, second string) bool {
	value := func(key string) string { s, _ := meta[key].(string); return s }
	_, atErr := time.Parse(time.RFC3339, value(reattestKeyAt))
	return value(reattestKeyOp) == reattestOp && atErr == nil &&
		strings.TrimSpace(value(reattestKeyActor)) != "" && value(reattestKeyScannedBranch) == specBranch &&
		wellFormedGitObjectID(value(reattestKeyMerge)) && wellFormedGitObjectID(value(reattestKeySecond)) &&
		value(reattestKeyMerge) == merge && value(reattestKeySecond) == second &&
		value("mindspec_landed_merge_sha") == merge && value("mindspec_landed_second_parent") == second
}

// advisoryRegistrationReport never refuses. A missing object is deliberately
// different from a known non-ancestor; neither is a fallback safety check.
func advisoryRegistrationReport(root, specBranch, beadID string) (lines []string, count int, rootsText string) {
	roots := []string{root}
	specID := strings.TrimPrefix(specBranch, workspace.SpecBranchPrefix)
	if idvalidate.SpecID(specID) == nil {
		if specDir, err := workspace.SpecDir(root, specID); err == nil && specDir != "" {
			roots = append(roots, specDir)
		}
	}
	for _, r := range panel.ForBead(landedPanelScanFn(roots...), beadID) {
		sha := r.Panel.ReviewedHeadSHA
		switch classifyReviewedSHA(sha) {
		case reviewedSHAMalformed:
			lines = append(lines, fmt.Sprintf("advisory: registration %s carries a malformed reviewed_head_sha; ignored", termsafe.Escape(r.Dir)))
		case reviewedSHAObjectID:
			ancestor, err := isAncestorFn(root, sha, specBranch)
			if ancestor && err == nil {
				continue
			}
			count++
			verdict := fmt.Sprintf("which is outside %s's tip history", termsafe.Escape(specBranch))
			if err != nil {
				verdict = "(not determinable locally)"
			}
			lines = append(lines, fmt.Sprintf("advisory: registration %s (round %d) reviewed %s, %s", termsafe.Escape(r.Dir), r.Panel.Round, termsafe.Escape(sha), verdict))
		}
	}
	for i, r := range roots {
		roots[i] = termsafe.Escape(r)
	}
	rootsText = strings.Join(roots, ", ")
	lines = append(lines, "advisory: panel roots scanned: "+rootsText)
	return lines, count, rootsText
}
