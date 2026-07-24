// mindspec reattest — the spec 125 R4 git-corroborated re-attest engine.
//
// ReattestLandedMerge is the derivation engine behind the EXPLICIT,
// operator-invoked `mindspec reattest <bead-id>` verb (cmd/mindspec).
// It (re)writes the merge-time landed-binding of an ALREADY-MERGED bead
// under ADR-0041 §2(ii) as amended by spec 125 ("Amendment (Spec 125):
// Re-attested landed-bindings under §2(ii)"): a landed-binding written
// AFTER merge time is admissible iff it is DERIVED from an independent
// git EXACT-second-parent match — never from an operator-asserted SHA
// pair corroborating itself (circular: there is deliberately NO
// parameter, flag, or argument through which a caller can supply a
// merge/second-parent pair — AC-8(iii)), never from subject text alone,
// and never from an agent-writable binding uncorroborated to a real
// exact merge.
//
// Ownership vs landed-ness (the §2(ii) amendment's two-source rule,
// same model as FindLandedMerge):
//
//   - OWNERSHIP is SUBJECT-NOMINATED: a candidate merge is this bead's
//     ONLY if its subject names bead/<id> (either subject form, parsed
//     by parseMergeSubjectBeadBranch, FULL branch-name equality — never
//     prefix/substring). An ANONYMOUS merge (subject names no bead)
//     cannot be nominated, so it is REFUSED even under explicit
//     operator invocation — operator assertion alone never substitutes
//     for the subject nominator (plan-gate G3-B1: this is the
//     documented, operator-vouched trust in the subject-to-name mapping
//     spec 121 already relied on, inside the threat boundary; datum (a)
//     is not claimed as independent-of-subject ownership proof).
//   - LANDED-NESS is git TOPOLOGY: a REAL two-parent first-parent merge
//     on the scanned spec branch. The amendment's admissible standalone
//     data consulted here: (a) the git-derived exact-second-parent
//     merge from this scan itself (the branch-deleted happy path — no
//     surviving tip required; topology, not the subject, is the
//     corroboration, so it is non-circular); (b) a surviving
//     bead-branch tip, which the scanned match must EQUAL; (c) a
//     registered panel's reviewed_head_sha, which must EQUAL the match's
//     second parent; (d) an existing binding git-corroborated to an
//     exact merge. When multiple data are present they must AGREE — a
//     contradiction REFUSES, never writes.
//
// Fail-closed: no owned exact merge → refuse (no guess, no write) to
// the audited ADR-0035 `mindspec-q9ea` human attested-restore exit BY
// NAME — datum (e), the explicitly-blessed, NEVER-sole exit for the
// genuinely-no-mechanical-corroboration case. Owned candidates with
// DIFFERENT second parents → ambiguity → refuse. Same-second-parent
// re-merges are ONE bead's repeated landings: the NEWEST names the
// written merge SHA, while Requirement 3's landed-vs-reverted content
// check is anchored on the OLDEST merge M₁ (the AC-2e masked-revert
// rule, reused VERBATIM from FindLandedMerge's discrimination); a
// REVERTED classification refuses and writes nothing.
//
// The audit record (AC-7/AC-9) is written in the SAME metadata call as
// the binding, under the mindspec_landed_reattest_* keys: acting
// identity/authority, before/after binding values (empty-string before
// when absent), RFC3339 UTC timestamp, invoking operation, the
// corroborating git datum, and the branch actually scanned (so a
// mis-scoped --spec-branch invocation is reconstructable). It lives in
// the same mutable bd metadata map: detectable-by-inspection, NOT
// cryptographically tamper-proof — exactly the amendment's claim, no
// more. An already-correct binding — one already naming the NEWEST
// same-second-parent merge (the same merge FindLandedMerge's
// newest-names-SHA rule derives) — is a byte-identical NO-OP: nothing
// is written at all (no duplicate audit churn, and the time-bearing
// reattest_at key cannot dirty a converged state). A binding naming an
// OLDER member of the same-second-parent re-merge group is STALE, not
// converged (spec 125 final-review FIX-3): it is re-written to the
// newest, audited, so the binding and FindLandedMerge's derived
// identity converge on the same merge. A CONTRADICTORY
// existing binding follows the G3-1 discipline: overwritten ONLY with
// the git-corroborated exact identity, the prior value recorded in the
// audit — never silently kept, never replaced uncorroborated.
package lifecycle

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
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
	// ReattestStateNoOwnedMerge: no two-parent first-parent merge on the
	// scanned branch names this bead (truly-bare, or only an anonymous/
	// decoy/descendant merge exists). Forward exit: the audited ADR-0035
	// mindspec-q9ea human attested-restore.
	ReattestStateNoOwnedMerge = "no-owned-exact-merge"
	// ReattestStateAmbiguous: owned candidates exist but carry DIFFERENT
	// second parents — genuine ambiguity about which landing is this
	// bead's tip; refused, never guessed (spec 125 R4 fail-closed rule).
	ReattestStateAmbiguous = "ambiguous-owned-second-parents"
	// ReattestStateTipContradiction: a surviving bead-branch tip (datum
	// (b)) does not EQUAL the owned match's second parent — the decoy /
	// stale-branch shape (AC-8(ii)).
	ReattestStateTipContradiction = "surviving-tip-contradiction"
	// ReattestStatePanelContradiction: a registered panel's
	// reviewed_head_sha (datum (c)) does not EQUAL the owned match's
	// second parent.
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
}

func (e *ReattestRefusal) Error() string {
	return fmt.Sprintf("%s %s on %s [%s]: %s", ErrReattestRefused.Error(), e.BeadID, e.SpecBranch, e.State, e.Detail)
}

// Unwrap makes errors.Is(err, ErrReattestRefused) succeed.
func (e *ReattestRefusal) Unwrap() error { return ErrReattestRefused }

// ReattestResult reports what the derivation established and whether a
// write happened. Wrote == false is the convergent no-op: the existing
// binding already git-corroborates to the derived identity, so NOTHING
// was written (byte-identical metadata, no audit churn).
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
func ReattestLandedMerge(root, specBranch, beadID, actor string) (*ReattestResult, error) {
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
	var candidates []gitutil.MergeCommit
	for _, m := range merges {
		if len(m.Parents) != 2 {
			continue
		}
		if name, present := parseMergeSubjectBeadBranch(m.Subject); present && name == beadBranch {
			candidates = append(candidates, m)
		}
	}
	if len(candidates) == 0 {
		return nil, &ReattestRefusal{
			BeadID: beadID, SpecBranch: specBranch, State: ReattestStateNoOwnedMerge,
			Detail: fmt.Sprintf("no two-parent first-parent merge on %s names %s in its subject — nothing is git-corroborable here, and neither an operator-asserted SHA pair (circular) nor an anonymous-subject merge (no ownership nominator) is admissible; the only remaining exit is the audited ADR-0035 mindspec-q9ea human attested-restore", specBranch, beadBranch),
		}
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
				BeadID: beadID, SpecBranch: specBranch, State: ReattestStateTipContradiction,
				Detail: fmt.Sprintf("surviving branch %s tip %s contradicts owned merge %s's second parent %s — the named merge did not land this branch's tip", beadBranch, tip, newest.SHA, secondParent),
			}
		}
		corroboration += fmt.Sprintf("; (b) surviving branch tip %s", tip)
	}

	// Datum (c): a registered panel's reviewed_head_sha must EQUAL the
	// derived second parent.
	if reviewed, haveReviewed := reviewedHeadSHAForBead(root, specBranch, beadID); haveReviewed && reviewed != "" {
		if reviewed != secondParent {
			// FIX-5 (final-review): reviewed_head_sha comes from the
			// AGENT-WRITABLE panel.json and this Detail reaches terminal
			// output via the cmd refusal rendering — render it escaped so
			// it can never carry a raw hostile value.
			return nil, &ReattestRefusal{
				BeadID: beadID, SpecBranch: specBranch, State: ReattestStatePanelContradiction,
				Detail: fmt.Sprintf("registered panel reviewed_head_sha %s contradicts owned merge %s's second parent %s", termsafe.Escape(reviewed), newest.SHA, secondParent),
			}
		}
		corroboration += fmt.Sprintf("; (c) panel reviewed_head_sha %s", reviewed)
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
				BeadID: beadID, SpecBranch: specBranch, State: ReattestStateReverted,
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
	if priorMerge == newest.SHA && priorSecond == secondParent {
		return &ReattestResult{
			BeadID: beadID, SpecBranch: specBranch,
			MergeSHA: newest.SHA, FirstParent: newest.Parents[0], SecondParent: secondParent,
			Corroboration:     corroboration + fmt.Sprintf("; (d) existing binding %s git-corroborated", priorMerge),
			Wrote:             false,
			PriorMergeSHA:     priorMerge,
			PriorSecondParent: priorSecond,
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
	}
	if writeErr := reattestBindingFn(beadID, updates); writeErr != nil {
		return nil, fmt.Errorf("recording the re-attested landed-binding for %s (merge %s): %w", beadID, newest.SHA, writeErr)
	}
	return &ReattestResult{
		BeadID: beadID, SpecBranch: specBranch,
		MergeSHA: newest.SHA, FirstParent: newest.Parents[0], SecondParent: secondParent,
		Corroboration:     corroboration,
		Wrote:             true,
		PriorMergeSHA:     priorMerge,
		PriorSecondParent: priorSecond,
	}, nil
}
