// Reattest derives an already-merged bead's binding from git, recording the
// prior values and an inspectable audit in one metadata write (specs 125/129,
// ADR-0041). Ownership is subject-nominated; operator-supplied SHA pairs are
// never accepted. Primary candidates retain Spec-125 ambiguity and revert
// checks. With zero primary candidates the source-named reachable fallback
// requires F1/F3/F4/F5; registrations are advisory only. Every read failure
// refuses before writing. A fallback no-op requires a valid matching audit
// tuple; a primary no-op requires the binding to match the newest owned merge.
package lifecycle

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// reattestBindingFn is the R4 write seam (AC-11(i)'s re-attest half):
// every re-attest binding+audit write routes through this variable, whose
// production default is pinned to the real bead.MergeMetadata by a
// pointer anti-drift test — the same seam family as the executor's
// mergeBindingFn/mergeBindingReadFn and this package's
// landedBindingMetadataFn, so the hermetic tests provably exercise the
// real bd write path and the gate cannot go hollow.
var reattestBindingFn = bead.MergeMetadata

// The mindspec_landed_reattest_* audit keys (spec 125 R4/AC-7/AC-9),
// written in the SAME MergeMetadata call as the binding keys so binding
// and audit land atomically from the caller's perspective. Flat keys in
// the existing bd metadata map — inspectable via `bd show <id> --json`.
const (
	reattestKeyActor         = "mindspec_landed_reattest_actor"
	reattestKeyAt            = "mindspec_landed_reattest_at"
	reattestKeyOp            = "mindspec_landed_reattest_op"
	reattestKeyCorroboration = "mindspec_landed_reattest_corroboration"
	reattestKeyPriorMerge    = "mindspec_landed_reattest_prior_merge_sha"
	reattestKeyPriorSecond   = "mindspec_landed_reattest_prior_second_parent"
	reattestKeyScannedBranch = "mindspec_landed_reattest_scanned_branch"
	reattestKeyMerge         = "mindspec_landed_reattest_merge_sha"
	reattestKeySecond        = "mindspec_landed_reattest_second_parent"
)

// reattestOp is the invoking-operation audit value: the one explicit
// surface that may produce this write. There is deliberately no doctor
// (or any implicit) writer — AC-7 pins that.
const reattestOp = "mindspec reattest"

// ErrReattestRefused is the typed sentinel every fail-closed re-attest
// refusal wraps (test with errors.Is; reach the state detail with
// errors.As on *ReattestRefusal). A refusal writes NOTHING — the bead's
// metadata is byte-identical to its pre-call state.
var ErrReattestRefused = errors.New("reattest refused: no git-corroborated exact landed merge for bead")

// ReattestRefusal states — the closed set of fail-closed reasons.
const (
	// ReattestStateNoOwnedMerge: no first-parent or admitted reachable
	// two-parent merge on the scanned branch names this bead (truly-bare, or only an anonymous/
	// decoy/descendant merge exists). The ancestor check can give more
	// evidence. An attested restore requires a verified merge.
	ReattestStateNoOwnedMerge  = "no-owned-exact-merge"
	ReattestStateAncestor      = "ancestor-without-naming-merge"
	ReattestStateUnknown       = "unknown"
	ReattestStateIndeterminate = "indeterminate"
	// ReattestStateAmbiguous: owned candidates exist but carry DIFFERENT
	// second parents — genuine ambiguity about which landing is this
	// bead's tip; refused, never guessed (spec 125 R4 fail-closed rule).
	ReattestStateAmbiguous = "ambiguous-owned-second-parents"
	// ReattestStateMultipleFallbackCandidates refuses independent reachable
	// merges even when they share a second parent. No grouping is safe.
	ReattestStateMultipleFallbackCandidates = "multiple-fallback-candidates"
	// ReattestStateTipContradiction: a surviving bead-branch tip (datum
	// (b)) does not EQUAL the owned match's second parent — the decoy /
	// stale-branch shape (AC-8(ii)).
	ReattestStateTipContradiction = "surviving-tip-contradiction"
	// ReattestStatePanelContradiction is retained for API compatibility.
	// Non-matching panel sets no longer produce it (spec 129).
	ReattestStatePanelContradiction = "panel-sha-contradiction"
	// ReattestStateReverted: the R3 content check (anchored on the OLDEST
	// same-second-parent merge M₁) classifies the bead's landed content
	// as no longer present at the scanned branch's tip.
	ReattestStateReverted = "content-reverted-at-tip"
)

// ReattestRefusal is a fail-closed re-attest refusal: the state names
// WHY, Detail carries the human-readable evidence. It wraps
// ErrReattestRefused (errors.Is) so callers can classify without string
// matching, and the cmd surface renders each state's forward exit.
type ReattestRefusal struct {
	BeadID     string
	SpecBranch string
	State      string
	Detail     string
	Fallback   bool
	// NoRestore suppresses branch restoration when it cannot resolve the
	// diagnosed state. Recovery holds commands for the read-only diagnosis.
	NoRestore bool
	Recovery  []string
}

func (e *ReattestRefusal) Error() string {
	return fmt.Sprintf("%s %s on %s [%s]: %s", ErrReattestRefused.Error(), termsafe.Escape(e.BeadID), termsafe.Escape(e.SpecBranch), e.State, e.Detail)
}

// Unwrap makes errors.Is(err, ErrReattestRefused) succeed.
func (e *ReattestRefusal) Unwrap() error { return ErrReattestRefused }

// ReattestResult reports what the derivation established and whether a
// write happened. Wrote == false is the convergent no-op: the existing
// binding already matches the derived identity (and, on the fallback,
// a VALID audit matches too), so NOTHING was written (byte-identical
// metadata, no audit churn).
type ReattestResult struct {
	BeadID     string
	SpecBranch string
	// MergeSHA / FirstParent / SecondParent are the DERIVED identity: the
	// NEWEST owned exact-second-parent merge (same-second-parent re-merges
	// are one bead's repeated landings; the newest names the merge).
	MergeSHA     string
	FirstParent  string
	SecondParent string
	// Corroboration names the admissible amendment data ((a)–(d)) that
	// jointly corroborated the identity — the value recorded in the audit.
	Corroboration string
	Wrote         bool
	// PriorMergeSHA / PriorSecondParent are the before-values ("" when
	// previously absent) — recorded in the audit on a write (G3-1).
	PriorMergeSHA     string
	PriorSecondParent string
	// Lines describe fallback provenance and advisory registrations. The optional
	// report callback emits them before the write.
	Lines []string
}

// reattestNowFn is the timestamp source for the audit record (seamed so
// tests can pin a deterministic value; production default is time.Now).
var reattestNowFn = time.Now

// ReattestLandedMerge derives and (re)writes beadID's landed-binding
// from an INDEPENDENT git scan of specBranch, per the package doc
// comment above (ADR-0041 §2(ii), spec 125 R4/R5). specBranch is
// SCOPING input only — it names WHERE to scan and is recorded in the
// audit (mindspec_landed_reattest_scanned_branch); it never substitutes
// for corroboration. actor is the acting identity/authority recorded in
// the audit (the cmd surface supplies user@host + argv0).
//
// It returns a *ReattestResult on success (Wrote=false for the
// convergent no-op), a *ReattestRefusal (wrapping ErrReattestRefused)
// for every fail-closed refusal, and a plain error for infra/git/read
// failures — which are NEVER classified as either attestation or
// refusal.
// An optional report callback receives fallback provenance and advisories
// before the single metadata write. Non-CLI callers may omit it.
func ReattestLandedMerge(root, specBranch, beadID, actor string, reports ...func(string)) (*ReattestResult, error) {
	if strings.TrimSpace(actor) == "" {
		return nil, fmt.Errorf("reattest requires a non-empty acting identity for the audit record")
	}
	beadBranch, err := workspace.BeadBranch(beadID)
	if err != nil {
		return nil, fmt.Errorf("invalid bead id: %w", err)
	}

	merges, err := firstParentMergesFn(root, specBranch)
	if err != nil {
		return nil, fmt.Errorf("scanning %s for a landed merge of %s: %w", specBranch, beadID, err)
	}

	// OWNERSHIP nomination — identical rule to FindLandedMerge's single
	// candidate entry point: two-parent first-parent merges whose subject
	// names THIS bead by full branch-name equality (AC-2f). An octopus
	// (>2-parent) merge is excluded even when its subject matches; an
	// anonymous subject nominates nothing; a different bead's token
	// nominates that other bead, never this one.
	candidates := ownedMergeCandidates(merges, beadBranch)
	fallback := len(candidates) == 0
	if fallback {
		candidates, err = reachableCandidates(root, specBranch, beadBranch)
		if err != nil {
			return nil, err
		}
		if len(candidates) > 1 {
			var pairs []string
			for _, m := range candidates {
				pairs = append(pairs, fmt.Sprintf("merge %s, second parent %s", termsafe.Escape(m.SHA), termsafe.Escape(m.Parents[1])))
			}
			return nil, &ReattestRefusal{BeadID: beadID, SpecBranch: specBranch, Fallback: true,
				State:  ReattestStateMultipleFallbackCandidates,
				Detail: "multiple admitted reachable candidates (" + strings.Join(pairs, "; ") + "); the fallback requires exactly one candidate, even when second parents agree"}
		}
	}
	if len(candidates) == 0 {
		r := ancestorWithoutNamingMerge(root, specBranch, beadID, beadBranch, merges)
		if r.State == ReattestStateAncestor {
			dependents, err := ancestorDependentsFn(beadID)
			if err != nil {
				r.State = ReattestStateIndeterminate
				r.Detail = "cannot read dependent beads: " + termsafe.Escape(err.Error())
			} else {
				setAncestorRecovery(r, dependents)
			}
		}
		return nil, r
	}

	// LANDED-NESS: all owned candidates must agree on ONE second parent.
	// Same-second-parent duplicates are one bead's re-merges (newest names
	// the merge, oldest anchors the content check); DIFFERENT second
	// parents are genuine ambiguity about which landing is the bead's tip
	// — with no surviving write-time ground truth, that is refused, never
	// guessed (spec 125 R4 "fail-closed on ambiguity").
	newest := candidates[0] // firstParentMergesFn is newest-first
	secondParent := newest.Parents[1]
	for _, c := range candidates[1:] {
		if c.Parents[1] != secondParent {
			return nil, &ReattestRefusal{
				BeadID: beadID, SpecBranch: specBranch, State: ReattestStateAmbiguous,
				Detail: fmt.Sprintf("merges %s (second parent %s) and %s (second parent %s) both name %s but disagree on the landed tip — refusing to pick one", newest.SHA, secondParent, c.SHA, c.Parents[1], beadBranch),
			}
		}
	}
	oldest := candidates[len(candidates)-1]

	corroboration := fmt.Sprintf("(a) git-derived exact-second-parent merge %s (second parent %s)", newest.SHA, secondParent)

	// Datum (b): a surviving bead-branch tip must EQUAL the derived second
	// parent — a mismatch is the decoy/stale-branch contradiction, refused.
	tip, survives, tipErr := resolveBranchTip(root, beadBranch)
	if tipErr != nil {
		return nil, fmt.Errorf("resolving surviving branch %s: %w", beadBranch, tipErr)
	}
	if survives {
		if tip != secondParent {
			return nil, &ReattestRefusal{
				BeadID: beadID, SpecBranch: specBranch, State: ReattestStateTipContradiction, Fallback: fallback,
				Detail: fmt.Sprintf("surviving branch %s tip %s contradicts owned merge %s's second parent %s — the named merge did not land this branch's tip", beadBranch, tip, newest.SHA, secondParent),
			}
		}
		corroboration += fmt.Sprintf("; (b) surviving branch tip %s", tip)
	}

	// Datum (c) is a SET, never a veto. Fallback registrations are
	// advisory output only and play no part in corroboration.
	if !fallback {
		reviewed := reviewedHeadSHAForBead(root, specBranch, beadID)
		if _, exact := reviewed[secondParent]; exact {
			corroboration += fmt.Sprintf("; (c) panel reviewed_head_sha %s", secondParent)
		} else if len(reviewed) > 0 {
			corroboration += "; (c) panel set non-corroborating"
		}
	}

	// Landed-vs-reverted: Requirement 3's discrimination REUSED VERBATIM
	// from FindLandedMerge, anchored on the OLDEST same-second-parent
	// merge M₁ (AC-2e — a later re-merge's own first parent can itself be
	// the post-revert state). A REVERTED classification refuses and writes
	// nothing; an infra error is UNDETERMINED and propagates, never mapped
	// to either classification (plan-gate O2-1).
	outcome, subErr := landedContentSubsumedFn(root, oldest.Parents[0], oldest.SHA, specBranch)
	if subErr != nil {
		return nil, fmt.Errorf("checking net effect of merge %s since it landed: %w", oldest.SHA, subErr)
	}
	if outcome == gitutil.SubsumptionCleanDivergence {
		rev, revErr := landedRevertShapeFn(root, oldest.SHA, specBranch)
		if revErr != nil {
			return nil, fmt.Errorf("checking revert shape of merge %s against %s's current tip: %w", oldest.SHA, specBranch, revErr)
		}
		if rev {
			return nil, &ReattestRefusal{
				BeadID: beadID, SpecBranch: specBranch, State: ReattestStateReverted, Fallback: fallback,
				Detail: fmt.Sprintf("merge %s's content is no longer present at %s's current tip (reverted or cleanly removed after landing) — a re-attested binding would mis-attest reverted work", oldest.SHA, specBranch),
			}
		}
	}

	// Before-values, read through the SAME read seam FindLandedMerge uses
	// (landedBindingMetadataFn — pointer-pinned to bead.GetMetadata). A
	// read failure is an infra error: without the prior values the write
	// could neither prove no-op convergence nor record an honest audit.
	meta, readErr := landedBindingMetadataFn(beadID)
	if readErr != nil {
		return nil, fmt.Errorf("reading existing landed-binding metadata for %s: %w", beadID, readErr)
	}
	var priorMerge, priorSecond string
	if meta != nil {
		s, _ := meta["mindspec_landed_merge_sha"].(string)
		p, _ := meta["mindspec_landed_second_parent"].(string)
		priorMerge = strings.TrimSpace(s)
		priorSecond = strings.TrimSpace(p)
	}

	// Datum (d): an existing binding is trusted ONLY when git-corroborated
	// per R5 — and the ALREADY-CORRECT convergent state (spec 125
	// final-review FIX-3) requires it to name the NEWEST
	// same-second-parent merge, i.e. exactly the scan-derived identity
	// FindLandedMerge's newest-names-SHA rule produces. Only then is the
	// re-run a byte-identical no-op: nothing written, no audit churn
	// (this is also what keeps the time-bearing reattest_at key from
	// dirtying a converged state on every re-run). A binding naming an
	// OLDER member of the re-merge group is STALE — it falls through to
	// the write below and is re-written to the newest, audited, with the
	// prior values recorded.
	var lines []string
	if fallback {
		if err := fallbackSafety(root, specBranch, beadBranch, newest); err != nil {
			state := ""
			switch {
			case errors.Is(err, ErrLandedMergeContradiction):
				state = ReattestStateTipContradiction
			case errors.Is(err, ErrLandedMergeReverted):
				state = ReattestStateReverted
			default:
				return nil, err
			}
			return nil, &ReattestRefusal{BeadID: beadID, SpecBranch: specBranch, State: state, Detail: err.Error(), Fallback: true}
		}
		advisory, count, roots := advisoryRegistrationReport(root, specBranch, beadID)
		lines = append(lines, fmt.Sprintf("candidate %s reached %s's tip off the first-parent chain; its subject names no target", termsafe.Escape(newest.SHA), termsafe.Escape(specBranch)))
		lines = append(lines, advisory...)
		corroboration = fmt.Sprintf("off-first-parent fallback; single-candidate, branch-tip, not-reverted and read checks passed; advisory registrations outside tip: %d; panel roots scanned: %s", count, roots)
		for _, report := range reports {
			if report != nil {
				for _, line := range lines {
					report(line)
				}
			}
		}
	}
	// Spec-125 FIX-3 is unchanged on the primary path. Only a fallback
	// candidate needs the Spec-129 audited tuple before it can converge.
	if (!fallback && priorMerge == newest.SHA && priorSecond == secondParent) ||
		(fallback && auditMatches(meta, specBranch, newest.SHA, secondParent)) {
		return &ReattestResult{
			BeadID: beadID, SpecBranch: specBranch,
			MergeSHA: newest.SHA, FirstParent: newest.Parents[0], SecondParent: secondParent,
			Corroboration:     corroboration + fmt.Sprintf("; (d) existing binding %s git-corroborated", priorMerge),
			Wrote:             false,
			PriorMergeSHA:     priorMerge,
			PriorSecondParent: priorSecond,
			Lines:             lines,
		}, nil
	}

	// Write ONLY the scan-derived SHAs (never a caller-supplied value —
	// none exists), plus the audit record, in ONE MergeMetadata call. A
	// present-but-uncorroborated prior binding is the G3-1 contradiction:
	// overwritten with the derived exact identity, prior values recorded.
	updates := map[string]interface{}{
		"mindspec_landed_merge_sha":     newest.SHA,
		"mindspec_landed_second_parent": secondParent,
		reattestKeyActor:                actor,
		reattestKeyAt:                   reattestNowFn().UTC().Format(time.RFC3339),
		reattestKeyOp:                   reattestOp,
		reattestKeyCorroboration:        corroboration,
		reattestKeyPriorMerge:           priorMerge,
		reattestKeyPriorSecond:          priorSecond,
		reattestKeyScannedBranch:        specBranch,
		reattestKeyMerge:                newest.SHA,
		reattestKeySecond:               secondParent,
	}
	if writeErr := reattestBindingFn(beadID, updates); writeErr != nil {
		return nil, fmt.Errorf("recording the re-attested landed-binding for %s (merge %s): %w", beadID, newest.SHA, writeErr)
	}
	return &ReattestResult{
		BeadID: beadID, SpecBranch: specBranch,
		MergeSHA: newest.SHA, FirstParent: newest.Parents[0], SecondParent: secondParent,
		Corroboration:     corroboration,
		Wrote:             true,
		Lines:             lines,
		PriorMergeSHA:     priorMerge,
		PriorSecondParent: priorSecond,
	}, nil
}

var (
	ancestorRangeRevertedFn = gitutil.RangeRevertedAt
	ancestorContentFn       = gitutil.ContentSubsumedOutcome
	ancestorDependentsFn    = ancestorDependents
	ancestorChildrenFn      = phase.FetchChildren
	ancestorIssueFn         = bead.RunBD
)

// ancestorWithoutNamingMerge only reads git. Callers must first exclude
// candidates in both the first-parent and reachable merge scans.
func ancestorWithoutNamingMerge(root, specBranch, beadID, beadBranch string, merges []gitutil.MergeCommit) *ReattestRefusal {
	r := &ReattestRefusal{BeadID: beadID, SpecBranch: specBranch, State: ReattestStateNoOwnedMerge,
		Detail: fmt.Sprintf("no first-parent or admitted reachable merge on %s names %s in its subject. Use an attested restore only after a person verifies the merge.", termsafe.Escape(specBranch), termsafe.Escape(beadBranch))}
	indeterminate := func(err error) *ReattestRefusal {
		r.State, r.NoRestore = ReattestStateIndeterminate, true
		r.Detail = "git evidence is indeterminate: " + termsafe.Escape(err.Error())
		return r
	}
	tip, exists, err := resolveBranchTip(root, beadBranch)
	if err != nil {
		return indeterminate(err)
	}
	if !exists {
		return r
	}
	specTip, err := landedRevParseRefFn(root, specBranch)
	if err != nil {
		return indeterminate(err)
	}
	ancestor, err := isAncestorFn(root, tip, specTip)
	if err != nil {
		return indeterminate(err)
	}
	if !ancestor {
		return r
	}
	base := specTip
	for {
		isBase, err := isAncestorFn(root, base, tip)
		if err != nil {
			return indeterminate(err)
		}
		if isBase {
			break
		}
		base, err = landedRevParseRefFn(root, base+"^1")
		if errors.Is(err, gitutil.ErrRefNotFound) {
			r.State = ReattestStateUnknown
			r.NoRestore = true
			r.Detail = fmt.Sprintf("work range is unknown for tip %s at spec tip %s: no first-parent base exists.", termsafe.Escape(tip), termsafe.Escape(specTip))
			return r
		}
		if err != nil {
			return indeterminate(err)
		}
	}
	if base == tip {
		return r
	}
	// A naming merge belongs to the spec's first-parent chain.
	for _, m := range merges {
		name, named := parseMergeSubjectBeadBranch(m.Subject)
		if len(m.Parents) == 2 && named && name != beadBranch && m.Parents[1] == tip {
			r.NoRestore = true
			r.Detail = fmt.Sprintf("tip %s is the second parent of merge %s (%s). This is not %s's own work.", termsafe.Escape(tip), termsafe.Escape(m.SHA), termsafe.Escape(m.Subject), termsafe.Escape(beadID))
			return r
		}
	}
	r.NoRestore = true
	outcome, contentErr := ancestorContentFn(root, base, tip, specTip)
	reverted, revertErr := ancestorRangeRevertedFn(root, base, tip, specTip)
	if contentErr != nil || revertErr != nil || outcome != gitutil.SubsumptionLanded || reverted {
		r.Detail = fmt.Sprintf("commits in %s..%s are in the spec history at %s. The tool cannot show that their changes are present.", termsafe.Escape(base), termsafe.Escape(tip), termsafe.Escape(specTip))
		for _, err := range []error{contentErr, revertErr} {
			if err != nil {
				r.Detail += " Evidence is indeterminate: " + termsafe.Escape(err.Error())
			}
		}
		return r
	}
	r.State = ReattestStateAncestor
	r.Detail = fmt.Sprintf("%s's changes are present on the spec branch at %s (range %s..%s), but no merge names %s. A re-merge cannot help. A person must verify the work before using the override.", termsafe.Escape(beadID), termsafe.Escape(specTip), termsafe.Escape(base), termsafe.Escape(tip), termsafe.Escape(beadID))
	return r
}

func setAncestorRecovery(r *ReattestRefusal, dependents []string) {
	if len(dependents) == 0 {
		r.Detail += " No bead waits on " + termsafe.Escape(r.BeadID) + "."
		r.Recovery = []string{"mindspec doctor"}
		return
	}
	for _, dependent := range dependents {
		r.Recovery = append(r.Recovery, fmt.Sprintf("mindspec next %s --allow-not-ready=MF-3 --evidence <file>", idrender.Bead(dependent)))
	}
}

// ancestorDependents reads open children of the same epic. Each dependency
// comes from show, since list output need not include dependency edges.
func ancestorDependents(beadID string) ([]string, error) {
	epic, _, err := phase.FindEpicForBead(beadID)
	if errors.Is(err, phase.ErrNoEpicLineage) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	children, err := ancestorChildrenFn(epic)
	if err != nil {
		return nil, err
	}
	var dependents []string
	seen := map[string]bool{}
	for _, child := range children {
		if child.ID == beadID || child.Parent != "" && child.Parent != epic || strings.EqualFold(child.Status, "closed") || seen[child.ID] {
			continue
		}
		if err := idvalidate.BeadID(child.ID); err != nil {
			return nil, err
		}
		data, err := ancestorIssueFn("show", child.ID, "--json")
		if err != nil {
			return nil, err
		}
		var rows []struct {
			ID           string `json:"id"`
			Dependencies []struct {
				ID   string `json:"id"`
				Type string `json:"dependency_type"`
			} `json:"dependencies"`
		}
		if err := json.Unmarshal(data, &rows); err != nil {
			return nil, err
		}
		if len(rows) != 1 || rows[0].ID != child.ID {
			return nil, fmt.Errorf("dependent bead read did not match the requested bead")
		}
		for _, dep := range rows[0].Dependencies {
			if dep.ID == beadID && dep.Type == "blocks" {
				dependents = append(dependents, child.ID)
				seen[child.ID] = true
				break
			}
		}
	}
	return dependents, nil
}
