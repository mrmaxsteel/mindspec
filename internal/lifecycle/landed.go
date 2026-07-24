// Landed-merge-commit-identity predicate (Spec 119 R4, Bead 1).
//
// A landed bead is one whose bead/<id> branch has already been merged into
// its spec branch via the in-binary CompleteBead/FinalizeEpic merge path
// (gitutil.MergeInto — every bead->spec merge site, mindspec_executor.go).
// mindspec's forward-only lifecycle needs to recognize this state even when
// the bead itself is not yet closed in Dolt (a prior `mindspec complete` run
// that closed-but-did-not-finish, or an operator's out-of-band recovery
// merge) or when it IS closed but its obligations were never settled — both
// converge through the SAME reconcile path (internal/complete), which
// consumes FindLandedMerge/MergedUnclosed to decide whether it may safely
// skip the (now pointless, and for an absent branch, ERRORING) merge-base /
// merge / branch-cleanup git plumbing while still applying every gate.
package lifecycle

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// ErrLandedMergeNotFound is returned by FindLandedMerge when no first-parent
// merge commit on specBranch positively identifies as beadID's landed
// bead->spec merge. It is a typed sentinel (test with errors.Is) so callers
// can distinguish "positively evaluated, no match" from a genuine git/read
// failure that must NOT be silently treated the same way.
var ErrLandedMergeNotFound = errors.New("no landed merge commit positively identified for bead")

// LandedMergeNoEvidence is returned by FindLandedMerge, wrapping
// ErrLandedMergeNotFound (test via errors.Is), when a subject-scan candidate
// merge commit exists on specBranch but NO admissible datum — a registered
// panel, a surviving bead/<id> branch, or the merge-time landed-binding —
// confirms it belongs to THIS bead (spec 121 R5(a)/(c), ADR-0041 §2(ii)).
// The subject text alone is never sufficient (AC-11: a hand-crafted "Merge
// bead/<id>" commit over an unrelated second parent must refuse). Callers
// extract the candidate's SHAs via errors.As to render the R5(c)
// attested-restore forward exit — the one deliberately non-mechanical
// recovery line in the tree: the operator must VERIFY the named merge
// before recreating the branch against it.
type LandedMergeNoEvidence struct {
	BeadID       string
	SpecBranch   string
	MergeSHA     string
	SecondParent string
}

func (e *LandedMergeNoEvidence) Error() string {
	return fmt.Sprintf("%s: %s on %s (candidate merge %s, second parent %s — no admissible datum confirms it)",
		ErrLandedMergeNotFound, e.BeadID, e.SpecBranch, e.MergeSHA, e.SecondParent)
}

// Unwrap makes errors.Is(err, ErrLandedMergeNotFound) succeed for a
// LandedMergeNoEvidence, preserving every existing caller's classification
// while still letting callers that need the candidate detail reach it via
// errors.As.
func (e *LandedMergeNoEvidence) Unwrap() error { return ErrLandedMergeNotFound }

// LandedMerge is the positively-identified bead->spec merge commit for a
// bead.
type LandedMerge struct {
	// SHA is the merge commit itself (M).
	SHA string
	// FirstParent is M^1 — the spec branch's content immediately before
	// the merge. Together with SHA it is the M^1..M evidence range the
	// reconcile path's doc-sync / ADR-divergence gates evaluate.
	FirstParent string
	// SecondParent is the merged bead branch's tip at merge time.
	SecondParent string
}

// Seams (test stubs). Defaults call the real gitutil/panel packages.
// isAncestorFn is the SAME package-level seam orphans.go declares — one
// home per predicate, no duplicate binding. landedRevParseRefFn is used
// (rather than orphans.go's branchExistsFn/gitutil.BranchExists, which
// takes no workdir and always checks the CALLING PROCESS's cwd) because
// FindLandedMerge/MergedUnclosed must resolve the bead branch's existence
// and tip IN root — a specific repo or worktree, not wherever the process
// happens to be running.
var (
	firstParentMergesFn = gitutil.FirstParentMerges
	landedRevParseRefFn = gitutil.RevParseRef
	landedPanelScanFn   = panel.Scan
	// landedBindingMetadataFn reads a bead's bd metadata for the merge-time
	// landed-binding datum (spec 121 R5(b), ADR-0041 §2(ii)) — the third
	// admissible corroboration FindLandedMerge consults, alongside the
	// registered panel and the surviving branch tip, AND the source of the
	// spec 125 BINDING-SHA candidate entry point (G1-F2). Seamed so tests
	// can stub a binding without a real bd process; the production default
	// is pinned to bead.GetMetadata by a pointer anti-drift test (spec 125
	// plan F3-2 — the read gate cannot go hollow).
	landedBindingMetadataFn = bead.GetMetadata
	// landedContentSubsumedFn is Bead 1's shared net-effect primitive
	// (gitutil.ContentSubsumedOutcome — final-review r2 F2-2r: the
	// trichotomy form, NOT the boolean projection NetEffectLanded's leg
	// (a) consumes), consumed here for R5(d) revert/reapply-awareness: a
	// positively-corroborated candidate is identified while its own
	// content is subsumed at specBranch's CURRENT tip OR superseded by
	// later work built on it (a three-way CONFLICT); only the CLEAN
	// not-subsumed shape — a genuine backout — refuses.
	landedContentSubsumedFn = gitutil.ContentSubsumedOutcome
	// landedRevertShapeFn is spec 125 R3's reverse un-apply
	// sub-classification (gitutil.RevertShape), consulted ONLY on a
	// SubsumptionCleanDivergence outcome from landedContentSubsumedFn —
	// never on the Landed or Conflict arms: revert-shape (the tip carries
	// NONE of M's introduced content — a true `git revert M`, or the
	// clean-full-removal residual, content-indistinguishable from one)
	// refuses; NOT revert-shape (SOME of M's content survives at the tip —
	// partial supersession by later honest work, the 8nhe.2 evolved case)
	// identifies. A non-nil error is an UNDETERMINED result and is
	// propagated (plan-gate O2-1), never mapped to either classification.
	// Default pinned to the real primitive by a pointer anti-drift test.
	landedRevertShapeFn = gitutil.RevertShape
)

// landedBinding is the merge-time landed-binding datum read back from bd
// metadata: the three keys internal/executor's mergeBindingFn writes
// (via bead.MergeMetadata) immediately after a MergeInto succeeds and
// before any cleanup.
type landedBinding struct {
	mergeSHA     string
	secondParent string
}

// wellFormedGitObjectID reports whether s is shaped like a git object id:
// 7-64 hex digits (an abbreviated SHA-1 through a full SHA-256). Spec 121
// final-review G2-1 (ADR-0042 provenance): the landed-binding values are
// read from AGENT-WRITABLE bd metadata. Since spec 125 they are COMPARED
// DATA only (exact string equality against scan-derived SHAs — the
// ancestor-tolerant isAncestorFn legs are retired), but the structural
// gate stays: no such value may ever be usable as a git option (hex-only
// can never start with '-') or a revision EXPRESSION (no '.', '~', '^',
// ':', '@') should any future consumer pass one to git. A non-conforming
// value makes the binding datum ABSENT, never a git operand and never a
// confirmation.
func wellFormedGitObjectID(s string) bool {
	if len(s) < 7 || len(s) > 64 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

// landedBindingForBead reads beadID's recorded landed-binding metadata, if
// any. A missing/empty binding, or a metadata-read failure, yields
// (nil, false) — an UNAVAILABLE datum, never treated as a contradiction
// (the same best-effort discipline as reviewedHeadSHAForBead): the binding
// was only introduced in spec 121, so a pre-121 bead — or one whose binding
// write failed and was instead corroborated through a different datum —
// simply has none.
//
// G2-1 provenance gate: a present-but-malformed value (anything that is
// not a well-formed git object id — an option-like `--foo`, terminal
// control bytes, a rev expression) also yields (nil, false): the executor's
// own binding write only ever records rev-parse output, so a non-conforming
// value is a corrupted or crafted binding — it must never confirm a
// candidate, and above all never reach git as an operand.
func landedBindingForBead(beadID string) (*landedBinding, bool) {
	meta, err := landedBindingMetadataFn(beadID)
	if err != nil || meta == nil {
		return nil, false
	}
	sha, _ := meta["mindspec_landed_merge_sha"].(string)
	sp, _ := meta["mindspec_landed_second_parent"].(string)
	sha = strings.TrimSpace(sha)
	sp = strings.TrimSpace(sp)
	if sha == "" && sp == "" {
		return nil, false
	}
	if (sha != "" && !wellFormedGitObjectID(sha)) || (sp != "" && !wellFormedGitObjectID(sp)) {
		return nil, false
	}
	return &landedBinding{mergeSHA: sha, secondParent: sp}, true
}

// resolveBranchTip resolves branch's tip commit in root, distinguishing "the
// branch genuinely does not exist" (survives=false, err=nil) from an
// operational git failure (err != nil) via gitutil.ErrRefNotFound.
func resolveBranchTip(root, branch string) (tip string, survives bool, err error) {
	tip, err = landedRevParseRefFn(root, branch)
	if err != nil {
		if errors.Is(err, gitutil.ErrRefNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	return tip, true, nil
}

// parseMergeSubjectBeadBranch parses the OWNERSHIP nominator — a
// bead-branch name — out of a merge commit's subject line (spec 125 R5:
// ownership, "which bead does this merge belong to?", is the merge
// SUBJECT's bead-branch name; landed-ness is git topology and is never
// established here). Recognized shapes:
//
//   - gitutil.MergeInto's deterministic form: `Merge bead/<id>`;
//   - git's default conflict-recovery form:
//     `Merge branch 'bead/<id>' into <target>` (and the bare
//     `Merge branch 'bead/<id>'` variant) — the branch name survives in
//     both, so ownership is robust to the conflict-recovery subject
//     variation (AC-1b/AC-2b).
//
// Three-state contract, deliberately CONSERVATIVE (plan-gate G2-2):
//
//   - (branch, true) when a `bead/…` token is present in ANY recognizable
//     position — including a token embedded in a subject shape this parser
//     does not otherwise understand. A present-but-unrecognized token is
//     reported as PRESENT-and-named (nominating THAT token), NEVER
//     collapsed into the no-bead state — so a DIFFERENT bead's token in an
//     exotic subject REJECTS on the caller's full-name equality check
//     rather than slipping through the names-no-bead binding exception.
//   - ("", false) ONLY when genuinely NO `bead/…` token appears anywhere
//     in the subject — the true anonymous-subject case. Such a merge is
//     NOT automatically identifiable (G-1): the automatic path has no
//     binding-alone entry point, so an anonymous subject FAILS CLOSED and
//     is recovered only via the explicit audited `mindspec reattest`.
//
// The returned token is COMPARED DATA only — callers match it by FULL
// branch-name EQUALITY against workspace.BeadBranch(beadID) (never
// HasPrefix/Contains, AC-2f) and MUST NOT use it as a git operand or
// embed it unescaped in terminal output (the G2-1 discipline).
func parseMergeSubjectBeadBranch(subject string) (branch string, present bool) {
	const beadPrefix = "bead/"
	if rest, ok := strings.CutPrefix(subject, "Merge "); ok {
		// `Merge bead/<id>` (and any trailing text after the token).
		if strings.HasPrefix(rest, beadPrefix) {
			return cutBranchToken(rest), true
		}
		// `Merge branch 'bead/<id>'` / `Merge branch 'bead/<id>' into …`.
		if quoted, ok := strings.CutPrefix(rest, "branch '"); ok {
			if end := strings.IndexByte(quoted, '\''); end > 0 {
				if tok := quoted[:end]; strings.HasPrefix(tok, beadPrefix) {
					return tok, true
				}
			}
		}
	}
	// Conservative fallback (G2-2): ANY bead/… token anywhere in the
	// subject — an unrecognized format still NOMINATES the bead it names;
	// it is never read as "names no bead".
	if idx := strings.Index(subject, beadPrefix); idx >= 0 {
		return cutBranchToken(subject[idx:]), true
	}
	return "", false
}

// cutBranchToken truncates s at the first byte that cannot plausibly
// belong to a branch-name token (whitespace, quotes, closing brackets,
// common separators, control bytes). It never trims characters INSIDE a
// valid bead-branch name, and over-capture is safe by construction: a
// token carrying trailing junk fails the caller's full-name equality
// check and REFUSES — never a false attribution.
func cutBranchToken(s string) string {
	end := strings.IndexFunc(s, func(r rune) bool {
		switch r {
		case ' ', '\t', '\'', '"', '`', ']', ')', '>', ',', ';':
			return true
		}
		return r < 0x20
	})
	if end >= 0 {
		return s[:end]
	}
	return s
}

// FindLandedMerge positively identifies the bead->spec merge commit for
// beadID on specBranch (spec 119 R4; hardened by spec 121 R5(a)/(d) and
// rebuilt on spec 125 R5's root-of-trust model, ADR-0041 §2(ii)).
//
// Two orthogonal facts come from two different sources, and neither alone
// suffices:
//
//   - OWNERSHIP ("which bead") is the merge SUBJECT's bead-branch name,
//     parsed by parseMergeSubjectBeadBranch from EITHER subject shape
//     (`Merge bead/<id>`, or git's default conflict-recovery
//     `Merge branch 'bead/<id>' into …`) and matched by FULL branch-name
//     equality — never prefix/substring (AC-2f).
//   - LANDED-NESS ("did it land") is git TOPOLOGY: a first-parent merge
//     on specBranch with exactly two parents whose SECOND parent EQUALS
//     the bead's landed tip — an EXACT match against a REAL merge. The
//     pre-125 ancestor-TOLERANT confirmation legs are REMOVED: an
//     ancestor-only-consistent datum is NEVER a positive identification
//     (AC-2b/AC-2c) — with no exact-and-owned match this function
//     REFUSES, never picks the newest ancestor-consistent merge.
//
// Candidate generation has exactly ONE admissible entry point (spec 125
// R5; the anonymous-subject binding-SHA path was REMOVED as a BLOCKING
// forgery hole — see below):
//
//   - the SUBJECT-SCAN path: two-parent first-parent merges on specBranch
//     whose subject NAMES THIS bead by full branch-name equality
//     (ownership nomination, EITHER subject form — gitutil.MergeInto's
//     `Merge bead/<id>` or git's default conflict-recovery
//     `Merge branch 'bead/<id>' into …`).
//
// A merge whose subject names NO bead is NOT automatically identifiable
// (G-1, codex final-review). Git-corroborating a binding proves the merge
// is REAL with a given exact second parent — it does NOT prove the merge
// is THIS bead's — so a binding can never be an independent OWNERSHIP
// authority on the automatic path: doing so would let a METADATA-forge
// (below the git-history threat boundary) on a never-landed bead point at
// any real anonymous merge and be identified. Because mindspec's OWN
// merges ALWAYS name the bead, an anonymous subject is a hand-crafted
// operator merge; its recovery is the EXPLICIT, audited `mindspec
// reattest` (Bead 4), never a binding-alone automatic identification. R1's
// residual is therefore reconciled as FAIL-CLOSED here (safe refusal),
// with the audited explicit surface as the forward exit.
//
// A subject-named candidate is positively identified ONLY when at least
// one admissible datum EXACT-matches it AND no available datum
// contradicts it. The admissible data, all equality-only:
//
//   - a registered panel's reviewed_head_sha EQUAL to the candidate's
//     second parent (panel.Scan/ForBead over the repo root and, when
//     specID is derivable from specBranch, the spec's own co-located
//     reviews/ directory);
//   - a surviving bead/<id> branch's tip EQUAL to the second parent;
//   - the merge-time landed-binding resolving to this candidate (recorded
//     merge SHA naming it, or recorded second parent equal to its second
//     parent) — a git-corroborated CACHE over an already-subject-OWNED
//     candidate, never an ownership authority of its own.
//
// The binding and the panel SHA are a git-corroborated CACHE, never an
// authority (§2(ii) "never subject text alone" is preserved: the subject
// NOMINATES which bead; topology proves landed-ness). A cache pointing at
// no real exact merge (forgery, AC-2c) or at a DIFFERENT bead's real merge
// (ownership, AC-2d) is discarded/contradicted and identification refuses
// rather than following it. A candidate with NO confirming datum at all —
// every leg unavailable — returns a *LandedMergeNoEvidence carrying the
// candidate's SHAs (spec 121 R5(c), the attested-restore forward exit;
// AC-10's no-datum contract). No admitted candidate at all returns the
// plain ErrLandedMergeNotFound.
//
// Same-second-parent re-merges (ONE bead's repeated merges of the same
// tip — never an ownership ambiguity; spec 125 R5/AC-2e): the NEWEST
// exact match names the returned *LandedMerge.SHA, but the R5(d)
// content-check below is anchored on the OLDEST such merge M₁ — a later
// re-merge's own first parent can itself be the POST-REVERT state, so a
// newest-anchored three-way reads "no change" and would MIS-attest a bead
// whose content is actually reverted at the tip. The single-merge case
// reduces exactly to R3 (M₁ == the one merge).
//
// R5(d) revert/reapply-awareness: once a candidate is positively
// corroborated, the three-way outcome of its OWN content-introducing
// change (M^1..M) against specBranch's CURRENT tip
// (gitutil.ContentSubsumedOutcome, Bead 1's shared net-effect primitive in
// its final-review-r2 trichotomy form) decides identification: content
// still subsumed → identified; a three-way CONFLICT (the tip itself
// advanced past M^1 on M's region — later honest work built on/superseding
// M) → identified (landed-then-evolved, the F2-2r fix); a CLEAN divergence
// is SUB-CLASSIFIED by the reverse un-apply test (gitutil.RevertShape,
// spec 125 R3): only the revert SHAPE — the tip carries NONE of M's
// introduced content (a genuine `git revert M`, or its
// content-indistinguishable clean-full-removal residual) — is NOT
// identified; a clean divergence that RETAINS part of M's content
// (partial supersession by later honest work) identifies.
// A revert-then-reapply, in either shape (a later commit
// re-introducing the same net changes, or a cherry-pick of them), leaves
// the content subsumed, so M is identified by construction. "Ever
// reverted ⇒ reject" is structurally inexpressible under this mechanism.
//
// A fresh bead branch with zero own commits can never produce a matching
// merge commit by construction: `git merge --no-ff` of an already-ancestor
// branch performs no merge ("Already up to date") and creates no commit —
// so FindLandedMerge correctly reports the plain ErrLandedMergeNotFound
// (no candidate to name) for it.
//
// Any git/read failure (a first-parent-merges scan failure, a
// surviving-branch rev-parse failure, or a net-effect-check failure) is
// returned as-is (not wrapped in ErrLandedMergeNotFound) so callers can
// fail closed on infra trouble rather than silently treating it as "not
// found".
func FindLandedMerge(root, specBranch, beadID string) (*LandedMerge, error) {
	if strings.TrimSpace(beadID) == "" {
		return nil, fmt.Errorf("%w: empty bead id", ErrLandedMergeNotFound)
	}
	beadBranch, err := workspace.BeadBranch(beadID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid bead id %s: %v", ErrLandedMergeNotFound, idrender.Bead(beadID), err)
	}

	merges, err := firstParentMergesFn(root, specBranch)
	if err != nil {
		return nil, fmt.Errorf("scanning %s for a landed merge of %s: %w", specBranch, beadID, err)
	}

	reviewedHeadSHA, haveReviewed := reviewedHeadSHAForBead(root, specBranch, beadID)

	branchTip, branchSurvives, tipErr := resolveBranchTip(root, beadBranch)
	if tipErr != nil {
		return nil, fmt.Errorf("resolving surviving branch %s: %w", beadBranch, tipErr)
	}

	binding, haveBinding := landedBindingForBead(beadID)

	// Candidate generation — the two admissible entry points (doc comment
	// above). merges is newest-first (gitutil.FirstParentMerges), so
	// candidates is too.
	var candidates []gitutil.MergeCommit
	for _, m := range merges {
		// Spec 125 R3's octopus/parent guard (AC-6): bead->spec merges are
		// exactly two-parent (gitutil.MergeInto), so an octopus (>2-parent)
		// candidate is EXCLUDED here — never run through corroboration or
		// the revert/evolved discrimination, whose M^1/M^2 anchoring is
		// only meaningful for a two-parent merge.
		if len(m.Parents) != 2 {
			continue
		}
		name, present := parseMergeSubjectBeadBranch(m.Subject)
		// The ONLY candidate-generation entry point on the AUTOMATIC path
		// is the SUBJECT-SCAN: the merge's subject must NAME this bead by
		// full branch-name equality (AC-2f's no-prefix-collision rule).
		// Ownership comes from the SUBJECT, never from the agent-writable
		// binding alone.
		//
		// G-1 (codex adversarial, spec 125 final-review — BLOCKING fix):
		// there is deliberately NO anonymous-subject binding-SHA entry
		// point. A merge whose subject names NO bead cannot be identified
		// automatically, because the git-corroboration of a binding proves
		// only that the merge is REAL with this exact second parent — NOT
		// that it is THIS bead's. Admitting an anonymous merge on the
		// binding alone would make the agent-writable binding an
		// independent ownership authority: a forged binding (a
		// METADATA-forge, EASIER than a commit-forge and therefore BELOW
		// the documented git-history threat boundary) on a never-landed
		// bead X, pointing at any real anonymous merge, would make
		// FindLandedMerge(X) positively identify a merge that isn't X's —
		// an unsafe false-positive. mindspec's OWN merges ALWAYS name the
		// bead (both gitutil.MergeInto's `Merge <branch>` and git's
		// conflict-recovery default `Merge branch 'bead/X' into …` carry
		// the branch name), so an anonymous subject arises only from a
		// hand-crafted operator merge with a wholly-custom message; its
		// correct recovery is the EXPLICIT, operator-vouched, AUDITED
		// `mindspec reattest` (Bead 4), never a binding-alone automatic
		// identification here. A different-bead subject (G2-2: including an
		// unrecognized-format bead/… token) is likewise excluded.
		if present && name == beadBranch {
			candidates = append(candidates, m)
		}
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("%w: %s on %s", ErrLandedMergeNotFound, beadID, specBranch)
	}

	// The NEWEST admitted candidate is evaluated (and, on identification,
	// names the returned *LandedMerge.SHA); its same-second-parent
	// siblings form the re-merge group whose OLDEST member anchors the
	// content-check (AC-2e). A contradicted newest candidate REFUSES —
	// identification never keeps scanning past a contradiction to an
	// older, differently-parented match.
	newest := candidates[0]
	secondParent := newest.Parents[1]
	var group []gitutil.MergeCommit
	for _, c := range candidates {
		if c.Parents[1] == secondParent {
			group = append(group, c)
		}
	}
	oldest := group[len(group)-1]

	confirmed := false

	if haveReviewed && reviewedHeadSHA != "" {
		if reviewedHeadSHA == secondParent {
			confirmed = true
		} else {
			// Exact-equality only (spec 125 R5, AC-2c): a reviewed_head_sha
			// that does not EQUAL the second parent is a contradiction —
			// the pre-125 ancestor-tolerant confirmation (a panel that
			// reviewed an EARLIER head "confirming" a later merge) is the
			// misattribution vector this spec removes.
			return nil, fmt.Errorf("%w: %s on %s (reviewed_head_sha %s contradicts merge %s's second parent %s)",
				ErrLandedMergeNotFound, beadID, specBranch, reviewedHeadSHA, newest.SHA, secondParent)
		}
	}
	if branchSurvives {
		if branchTip == secondParent {
			confirmed = true
		} else {
			// Exact-equality only: a surviving branch whose tip is not the
			// merge's second parent (e.g. new unlanded commits) contradicts
			// — never the pre-125 ancestor-tolerant confirm.
			return nil, fmt.Errorf("%w: %s on %s (surviving branch %s tip %s contradicts merge %s's second parent %s)",
				ErrLandedMergeNotFound, beadID, specBranch, beadBranch, branchTip, newest.SHA, secondParent)
		}
	}
	if haveBinding {
		bindingNamesGroupMember := false
		if binding.mergeSHA != "" {
			for _, c := range group {
				if c.SHA == binding.mergeSHA {
					bindingNamesGroupMember = true
					break
				}
			}
		}
		switch {
		case bindingNamesGroupMember:
			// The recorded merge SHA names an owned exact match in this
			// group (possibly the OLDEST of a re-merge chain — the
			// complete-time write of the first landing).
			confirmed = true
		case binding.secondParent != "" && binding.secondParent == secondParent:
			confirmed = true
		case binding.secondParent != "":
			// Exact-equality only (spec 125 R5, AC-2c/AC-2d): a recorded
			// second parent that matches no real exact merge here is a
			// forged/stale cache — DISCARDED as confirmation and treated
			// as a contradiction, never followed and never softened by
			// the pre-125 ancestor tolerance.
			//
			// G2-1: binding.mergeSHA/binding.secondParent come from
			// agent-writable bd metadata — the provenance gate
			// (wellFormedGitObjectID in landedBindingForBead) already
			// constrains them to hex, but render them escaped anyway so
			// this message can never carry a raw hostile value.
			return nil, fmt.Errorf("%w: %s on %s (landed-binding merge %s/second-parent %s contradicts merge %s's second parent %s)",
				ErrLandedMergeNotFound, beadID, specBranch, termsafe.Escape(binding.mergeSHA), termsafe.Escape(binding.secondParent), newest.SHA, secondParent)
		}
	}

	if !confirmed {
		// R5(c): an owned candidate with every corroboration leg
		// unavailable — not identified, but NAME the candidate so the
		// caller can render the attested-restore forward exit (AC-10).
		return nil, &LandedMergeNoEvidence{
			BeadID: beadID, SpecBranch: specBranch,
			MergeSHA: newest.SHA, SecondParent: secondParent,
		}
	}

	// R5(d): revert/reapply-awareness. The three-way
	// merge-tree(base=M₁^1, ours=tip, theirs=M₁) outcome is
	// DISCRIMINATED (final-review r2 F2-2r), not collapsed to a boolean.
	// Spec 125 R5/AC-2e: M here is the OLDEST same-second-parent merge M₁
	// (the pre-first-landing anchor) — Requirement 3's discrimination
	// REUSED VERBATIM, parameterized by M = M₁; the newest merge governs
	// ONLY the reported *LandedMerge.SHA and supplies NEITHER the base NOR
	// the theirs merge of this check. Outcomes:
	//
	//   - SubsumptionLanded — M's content is present, net-effect, at
	//     specBranch's CURRENT tip (includes both AC-10(ii)
	//     revert-then-reapply shapes) → identified.
	//   - SubsumptionConflict — the tip has ITSELF advanced past M^1
	//     on M's own region, incompatibly with re-applying M. M is in
	//     the tip's first-parent history, so this means later honest
	//     work EVOLVED/superseded M's content (a conflict-resolution
	//     region edited again, a bead's file rewritten by a later
	//     bead) — landed-then-evolved → identified. Refusing here was
	//     the F2-2r permanent-refusal deadlock: no recovery converges
	//     (restoring the branch re-corroborates and re-refuses), and
	//     the only exits were the bd-close bypass or reverting
	//     legitimate later work — the §2(i) class ADR-0041 forbids.
	//   - SubsumptionCleanDivergence — the three-way applies M's
	//     change CLEANLY but the result differs from the tip: at
	//     least PART of M's content is no longer at the tip. Spec 125
	//     R3 (AC-5): this arm ALONE cannot distinguish "genuinely
	//     backed out" from "partially superseded by later honest
	//     work" (the 8nhe.2 false-negative — evolved content wrongly
	//     refused), so it is SUB-CLASSIFIED by landedRevertShapeFn
	//     (gitutil.RevertShape), the reverse un-apply no-op test:
	//     revert-shape — the tip carries NONE of M's introduced
	//     content (exactly what `git revert M` leaves behind, or the
	//     conservative clean-full-removal residual, which is
	//     content-indistinguishable from a revert and refuses by
	//     design: any datum here that accepted clean full removal
	//     would accept every real revert too) → NOT identified
	//     (AC-10(i) RED-on-revert preserved); NOT revert-shape — SOME
	//     of M's content survives at the tip (partial supersession)
	//     → identified (landed-then-evolved).
	outcome, subErr := landedContentSubsumedFn(root, oldest.Parents[0], oldest.SHA, specBranch)
	if subErr != nil {
		return nil, fmt.Errorf("checking net effect of merge %s since it landed: %w", oldest.SHA, subErr)
	}
	if outcome == gitutil.SubsumptionCleanDivergence {
		// Infra-error discipline (plan-gate O2-1): a non-nil error
		// from the reverse check is an UNDETERMINED result and is
		// propagated — the same fail-closed posture as subErr above —
		// never mapped to "identify" (a false-positive attestation on
		// an undetermined result) and never to "refuse".
		rev, revErr := landedRevertShapeFn(root, oldest.SHA, specBranch)
		if revErr != nil {
			return nil, fmt.Errorf("checking revert shape of merge %s against %s's current tip: %w", oldest.SHA, specBranch, revErr)
		}
		if rev {
			return nil, fmt.Errorf("%w: %s on %s (merge %s's content is no longer present at %s's current tip — it was reverted or cleanly removed after landing)",
				ErrLandedMergeNotFound, beadID, specBranch, oldest.SHA, specBranch)
		}
		// NOT revert-shape: part of M's content is still present at
		// the tip — later honest work partially superseded M's
		// surface (the 8nhe.2 evolved case) → fall through to
		// identify.
	}

	// AC-2e: the NEWEST same-second-parent exact match names the merge;
	// the content-check above was anchored on the OLDEST (M₁).
	return &LandedMerge{SHA: newest.SHA, FirstParent: newest.Parents[0], SecondParent: secondParent}, nil
}

// reviewedHeadSHAForBead looks up the reviewed_head_sha recorded by a
// registered panel targeting beadID, scanning the repo root and (when
// resolvable) the owning spec's co-located reviews/ directory. Best-effort:
// no registered panel, or one with an empty reviewed_head_sha, yields
// ("", false) — an UNAVAILABLE corroboration, never treated as a
// contradiction.
func reviewedHeadSHAForBead(root, specBranch, beadID string) (string, bool) {
	// Reverse-derivation gate (ADR-0042 §1 reverse): specID is parsed back
	// OUT of an agent-writable branch name via TrimPrefix. A malformed
	// result is never treated as an ID — corroboration simply proceeds
	// root-only (the same as when specID == "" today).
	specID := strings.TrimPrefix(specBranch, workspace.SpecBranchPrefix)
	if idvalidate.SpecID(specID) != nil {
		specID = ""
	}
	roots := []string{root}
	if specID != "" {
		if specDir, err := workspace.SpecDir(root, specID); err == nil && specDir != "" {
			roots = append(roots, specDir)
		}
	}
	regs := panel.ForBead(landedPanelScanFn(roots...), beadID)
	for _, r := range regs {
		if r.Err != nil {
			continue
		}
		if sha := strings.TrimSpace(r.Panel.ReviewedHeadSHA); sha != "" {
			return sha, true
		}
	}
	return "", false
}

// MergedUnclosed derives the "merged-unclosed" reconcile-eligibility state
// (Spec 119 R4, Bead 1 Step 4): FindLandedMerge positively identifies a
// landed merge AND — when bead/<id> still exists — it is an ancestor of
// specBranch's CURRENT tip. This second check is deliberately against the
// whole spec branch (not just the identified merge's second parent, which
// only pins the merge-TIME tip): a branch carrying NEW commits landed after
// the identified merge — genuinely unmerged, still-in-flight work — is
// correctly NOT flagged as merged-unclosed.
//
// Returns (landed, true, nil) when merged-unclosed; (nil, false, nil) when
// not (no candidate merge exists at all, OR the branch survives with new
// unlanded work — both are simply "not this state", not an error); and
// (nil, false, err) on a genuine git/read failure OR a spec 121
// *LandedMergeNoEvidence (a subject-scan candidate exists but no admissible
// datum confirms it) — the latter is deliberately propagated as an error,
// not swallowed into "not this state", so the caller can render the R5(c)
// attested-restore refusal naming the candidate's SHAs (errors.As) instead
// of a generic "nothing found" message.
func MergedUnclosed(root, specBranch, beadID string) (*LandedMerge, bool, error) {
	landed, err := FindLandedMerge(root, specBranch, beadID)
	if err != nil {
		var noEvidence *LandedMergeNoEvidence
		if errors.As(err, &noEvidence) {
			return nil, false, err
		}
		if errors.Is(err, ErrLandedMergeNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}

	beadBranch, err := workspace.BeadBranch(beadID)
	if err != nil {
		return nil, false, fmt.Errorf("invalid bead id %s: %w", idrender.Bead(beadID), err)
	}
	_, survives, tipErr := resolveBranchTip(root, beadBranch)
	if tipErr != nil {
		return nil, false, fmt.Errorf("resolving branch %s: %w", beadBranch, tipErr)
	}
	if !survives {
		return landed, true, nil
	}
	anc, ancErr := isAncestorFn(root, beadBranch, specBranch)
	if ancErr != nil {
		return nil, false, fmt.Errorf("checking ancestry of %s against %s: %w", beadBranch, specBranch, ancErr)
	}
	if !anc {
		return nil, false, nil
	}
	return landed, true, nil
}
