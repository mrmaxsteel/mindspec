package panel

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// ADR-0037 AMENDMENT NOTE (Spec 099): this is the SINGLE home of the
// panel-gate enforced contract (registration §1, round derivation §2, pass
// rule §3, staleness §4, dirty-tree §5, fail-open/fail-closed asymmetry
// §6, escape hatches §7, trust boundary §8). It was relocated here from
// internal/hook/precomplete.go so that the in-binary `mindspec complete` gate
// — the SOLE panel-gate enforcement point — reads one decision over one set
// of facts. The PreToolUse hook this note once described as a
// defense-in-depth backstop is RETIRED (spec 102): the in-binary gate is the
// only enforcer now (internal/complete/panel_advisory.go:45-47), so there is
// no second invoker left to keep in sync.
//
// internal/panel is a dependency-clean LEAF: this file imports exactly TWO
// internal packages, both stdlib-only pure-string leaves: internal/termsafe
// (spec 116's construction-boundary field escaping; ADR-0037 amendment,
// 2026-07-11) and internal/idvalidate (spec 120 R2/ADR-0042: beadID is
// validated at the ResolveGateFacts seam before the inline "bead/"+beadID
// concat below, closing ingress (iv) — internal/panel stays a leaf by
// validating in-package rather than importing internal/workspace). The
// git/status/ref-not-found I/O is still supplied by the CALLER through the
// GateIO seam (function closures), and the bead-branch prefix is still
// inlined as the literal "bead/" (== workspace.BeadBranchPrefix) rather than
// importing internal/workspace. The invariant's recorded purpose — no
// config coupling, no git/status I/O, decision purity — is preserved; only
// its letter ("imports NO internal package") changes, to admit these two
// stdlib-only leaves, and it is now machine-checked (not just documented) by
// TestPanelLeafImports_StdlibPlusTermsafeOnly (AC7, extended by spec 120).

// SkipPanelEnv is the env-only escape hatch for the panel gate (Spec 093
// Req 13a, ADR-0037 §7). It is read via os.Getenv ONLY by callers — the
// command string is NEVER consulted. The variable name is documented for
// HUMANS in /ms-panel-tally § Escape hatch; it is NEVER printed in a Block
// message (HC-7) — see the decision tests.
const SkipPanelEnv = "MINDSPEC_SKIP_PANEL"

// GateAction is the panel gate's decision outcome. It mirrors the hook's
// Pass/Block/Warn (hook.Action) so the wiring layer can map one to the other.
type GateAction int

const (
	// Allow proceeds silently (the hook maps this to Pass / exit 0).
	Allow GateAction = iota
	// Block is a hard deny (the hook maps this to Block / stderr+exit2).
	Block
	// Warn is advisory, non-blocking (the hook maps this to Warn).
	Warn
)

// Decision is the panel gate's outcome: an action plus the message to surface.
// The caller maps it to its own protocol (the hook -> hook.Result, the
// in-binary gate -> guard.NewFailure for Block).
//
// AppliedRefutations is populated only on the full pass-rule Allow return.
// Warn and short-circuit paths do not apply discharges. Callers compare
// Action and Message rather than comparing Decision directly (it has a slice).
type Decision struct {
	Action             GateAction
	Message            string
	AppliedRefutations []Refutation
	AppliedDeferrals   []Deferral
}

// RawMergeFence is appended to every Block message (Spec 093 Req 12 / G3-1,
// ADR-0037 §8). Once the gate blocks `mindspec complete`, a raw
// `git merge bead/<id>` on the spec branch is the obvious workaround; the
// matcher only fires on `mindspec complete` and mindspec installs no
// pre-merge-commit git hook, so the fence is prose by necessity.
//
// This function's own rendered template IS a real FamilyGitMerge match
// (spec 127 R5(a)) — verified: guard.FindFloorMatches on its literal
// template string returns [{Family:git merge}]. It reaches EIGHT real
// `Decision{Message:}` diagnostics in this file — seven reached as a
// %s ARGUMENT to an outer fmt.Sprintf, one via strings.Builder's
// WriteString. Deliberately NOT cited by absolute line number: spec
// 127 bead-2 rework round 1 (RULING 10) cited eight exact line numbers
// here as "counted directly from this file, not carried over from a
// prior estimate", and round 2's confirm found every one of those
// eight numbers ALREADY stale (a later edit to this same comment block
// shifted every site by +25 lines without the citation being
// recomputed — independently caught by four reviewer slots, O2c-3/
// S1-4/S1-CONFIRM-2/S2-r2-NEW1/S3-r2-NEW1). A line number drifts on
// the next unrelated edit; the count and shape split do not, and
// TestRawMergeFence_CallSiteShapeSentinel (internal/panel/
// rawmergefence_test.go) re-parses this file fresh on every run and
// REDs the moment either one changes — a `grep -n
// 'RawMergeFence(f.BeadID)' internal/panel/gate.go` is also always
// available to a reader who wants the CURRENT line numbers, which
// this comment no longer tries to duplicate.
//
// internal/lint's product-diagnostic scan does NOT reach any of the
// eight: the scan's Sprintf fold rule inspects only the template,
// args[0], by design (R5c's own stated shape) — never a substituted
// argument — and strings.Builder composition is not a fold shape the
// scan recognizes at all. This is a genuine, disclosed reach gap, not
// a mechanism failure: an exemption-list entry is the WRONG
// disposition (R5(a)'s own text: the exemption list's domain is
// exactly the swept GUIDANCE surfaces — "every emitted lifecycle
// diagnostic is outside it... never exemption-listed" — and this is
// product output), an allowlist entry collides with AC-9(ii)'s pinned
// final count, and widening the fold rules to trace Sprintf arguments
// or strings.Builder composition would revive the unbounded-fold-reach
// shape the round-6 spec gate closed. Recorded here, reviewed, rather
// than left for a reader to discover the scan does not see it.
func RawMergeFence(beadID string) string {
	if beadID == "" {
		return ""
	}
	return fmt.Sprintf(
		"\nDo NOT bypass with raw `git merge bead/%s` — it skips bd closure, "+
			"worktree cleanup, and this gate; only `mindspec complete` merges bead branches.",
		termsafe.Escape(beadID))
}

// GateFacts is the fully-resolved, I/O-free input to PanelGateDecision. Every
// field is gathered by a caller's I/O layer (env, fs scan, git) — via
// ResolveGateFacts and the GateIO seam — so the decision itself is a pure
// function of these facts: the one testable home of the allow/block logic
// (Spec 093 Req 12).
type GateFacts struct {
	// BeadID is the bead the matched `mindspec complete` targets.
	BeadID string

	// SkipEnv reports whether MINDSPEC_SKIP_PANEL == "1" (Req 13a).
	SkipEnv bool

	// Reg is the registration that named this bead, with its tally. nil
	// means no panel.json referenced the bead — fail-open (HC-4).
	Reg *Registration
	Res *Result

	// HeadSHA is the current `git rev-parse bead/<id>` in the scan root.
	// MissingRef true means the branch GENUINELY no longer exists (exit-1 /
	// ErrRefNotFound) — the rerun-after-merge pass-through (Req 11). When
	// MissingRef is true HeadSHA is "".
	HeadSHA    string
	MissingRef bool

	// SubjectDigest is the live subject on a SHA mismatch. SubjectErr is a
	// fail-closed read error (including a pinned spec branch resolution failure),
	// separate from the ordinary rev-parse fail-open GitErr.
	SubjectDigest string
	SubjectErr    error
	// ArtifactDetails names unresolved paths absent from the original round tree.
	ArtifactDetails string

	// GitErr true means the rev-parse failed with a TRANSIENT or structural
	// error (not a clean "ref absent"): exit 128, git missing, lock
	// contention. It is deliberately NOT folded into MissingRef so a transient
	// failure is not silently treated as a confirmed branch deletion (the
	// false-clear noted by the round-2 panel). Still fail-open per the spec's
	// deliberate posture (Req 11/12, advisory-backstopped), but surfaced
	// HONESTLY as a distinct Warn rather than a "merge already landed" note.
	GitErr error

	// WorktreeAbsent reports that the bead worktree could not be found, so
	// the porcelain dirty check was skipped (Req 11 missing-worktree
	// pass-through). UserDirt lists user-authored uncommitted paths (artifact
	// paths already filtered out) in the resolved worktree.
	WorktreeAbsent bool
	WorktreePath   string
	UserDirt       []string
}

// PanelGateDecision renders the pass/block decision for a single matched
// panel from fully-resolved facts (Spec 093 Req 12, ADR-0037 §§3-6). It is
// pure: no env, no fs, no git. The short-circuit ORDER is load-bearing and
// pinned by gate finding T3-1 —
//
//	(0) escape hatch  → Allow + Warn (audited at complete time)
//	(1) no panel      → Allow (fail-open, HC-4)
//	(2) malformed reg → Block (a registered-but-unparseable panel is NOT
//	                    "no panel"; it must not tip fail-open)
//	(2b) integrity/floor → Block before abandonment
//	(3) abandoned     → Allow + Warn  (BEFORE staleness — an abandoned
//	                    panel whose branch gained commits must never be
//	                    false-Blocked by the stale-SHA rule)
//	(4) round mismatch→ Block
//	(5) missing ref   → Allow + Warn  (rerun-after-merge)
//	(5b) transient gitErr → Allow + Warn  (honest, NOT "merge landed")
//	(6) stale subject → Block  (legacy: stale SHA; read errors also Block)
//	(7) dirty tree    → Block  (materialize-past-review bypass; skipped when worktree absent)
//	(8) incomplete    → Block  (verdicts < expected_reviewers)
//	(9) REJECT/hard   → Block  (halt path, no vote count overrides)
//	(9.5) unresolved  → Block (every finding must be discharged)
//	(10) pass rule    → Allow when all findings are discharged
//
// false POSITIVES (a wrongful Block) are the pinned bug class (Req 9); the
// missing-ref and missing-worktree pass-throughs exist to keep the
// documented partial-failure recovery rerun unblocked.
func panelGateDecision(f GateFacts) Decision {
	// (0) Escape hatch — env-only, audited. Never names the variable in a
	// Block (this is a Warn/Allow path, so HC-7 is moot here, but we still
	// keep the message hatch-name-free except for this legitimate audit).
	if f.SkipEnv {
		return Decision{Action: Warn, Message: fmt.Sprintf(
			"panel gate skipped via %s for %s", SkipPanelEnv, termsafe.Escape(f.BeadID))}
	}

	// (1) Fail-open: no registered panel for the bead.
	if f.Reg == nil || f.Res == nil {
		return Decision{Action: Allow}
	}

	// slug is escaped once here (Spec 116 R2): it is a disk directory name
	// (filepath.Base(dir)), validated for the CLI verbs' argv but NOT for
	// internal/complete's/internal/instruct's scan-matched dirs, and every
	// leg below (2/3/4/8/9/9.5/10) interpolates this same escaped value.
	slug := termsafe.Escape(f.Reg.Slug())

	// (2) A panel.json exists but could not be parsed — registered but
	// malformed. It must NOT read as "no panel".
	if f.Res.PanelErr != nil || f.Res.Panel == nil {
		return Decision{Action: Block, Message: fmt.Sprintf(
			"panel %s registration (panel.json) is unreadable — fix or remove it before completing%s",
			slug, RawMergeFence(f.BeadID))}
	}
	p := f.Res.Panel
	round := f.Res.LatestRound
	if round == 0 {
		round = p.Round
	}

	if err := f.Res.intactError(); err != nil {
		return Decision{Action: Block, Message: fmt.Sprintf("panel %s registration: %s%s", slug, err, RawMergeFence(f.BeadID))}
	}

	// (3) Abandoned — legitimate exit, audited on bead metadata at complete
	// time (Req 13e). Checked BEFORE staleness (T3-1) so an abandoned panel
	// whose branch advanced is never false-Blocked.
	if p.Abandoned {
		reason := strings.TrimSpace(p.AbandonReason)
		if reason == "" {
			reason = "(no reason recorded — abandon_reason is required; /ms-panel-tally abandon procedure)"
		} else {
			reason = termsafe.Escape(reason)
		}
		return Decision{Action: Warn, Message: fmt.Sprintf(
			"panel %s round %d abandoned: %s — completing per the recorded abandonment", slug, round, reason)}
	}

	// (4) Round mismatch — panel.json.round disagrees with the
	// filename-derived latest round in either direction (Req 11).
	if f.Res.RoundMismatch {
		return Decision{Action: Block, Message: fmt.Sprintf(
			"panel %s: panel.json round (%d) out of date vs verdict files (round %d) — re-run /ms-panel-run step 0%s",
			slug, p.Round, f.Res.LatestRound, RawMergeFence(f.BeadID))}
	}

	// (5) Missing ref — the bead branch GENUINELY no longer exists (exit-1
	// ErrRefNotFound). The merge already landed (completion deletes the
	// branch); pass through to complete.Run's idempotent handling rather than
	// false-block the recovery rerun (Req 11). The bead-branch literal
	// "bead/"+BeadID (== workspace.BeadBranchPrefix) is inlined to keep
	// internal/panel a leaf (no internal/workspace import).
	if f.MissingRef {
		escBeadID := termsafe.Escape(f.BeadID)
		return Decision{Action: Warn, Message: fmt.Sprintf(
			"panel for %s references branch bead/%s, which no longer exists — assuming the merge already landed; "+
				"deferring to mindspec complete's own handling", escBeadID, escBeadID)}
	}

	// (5b) Transient/structural git error — the staleness rev-parse could not
	// run (not a clean "ref absent"). The spec's posture is deliberately
	// fail-open (Req 11/12, advisory-backstopped), so we still Allow+Warn; but
	// — unlike (5) — we do NOT claim the merge landed, because a transient
	// error is NOT evidence the branch was deleted. Surfacing it honestly
	// closes the round-2 false-clear (a transient error conflated with a
	// genuine deletion) without false-blocking a legitimate completion.
	if f.GitErr != nil {
		escBeadID := termsafe.Escape(f.BeadID)
		return Decision{Action: Warn, Message: fmt.Sprintf(
			"panel for %s: could not verify branch bead/%s (transient git error: %s) — staleness check skipped; "+
				"proceeding per the gate's fail-open posture, but this is NOT a confirmed merge",
			escBeadID, escBeadID, termsafe.Escape(f.GitErr.Error()))}
	}

	// (6) Subject freshness. Legacy panels retain the exact stale-SHA message.
	if f.SubjectErr != nil || (p.ReviewedHeadSHA != "" && f.HeadSHA != "" && !shaEqual(p.ReviewedHeadSHA, f.HeadSHA) && (p.SubjectDigest == "" || p.SubjectDigest != f.SubjectDigest)) {
		// Escape AFTER short() truncates to avoid splitting an escape sequence.
		message := fmt.Sprintf("panel round %d reviewed %s, branch now at %s — commits landed after review; bump round and re-panel (/ms-panel-run step 0)", round, termsafe.Escape(short(p.ReviewedHeadSHA)), termsafe.Escape(short(f.HeadSHA)))
		if f.SubjectErr != nil {
			message = fmt.Sprintf("panel %s: could not verify subject tree: %s — bump round and re-panel (/ms-panel-run step 0)", slug, termsafe.Escape(f.SubjectErr.Error()))
		}
		return Decision{Action: Block, Message: fmt.Sprintf("%s%s", message, RawMergeFence(f.BeadID))}
	}

	// (7) Dirty tree — uncommitted USER edits would be MATERIALIZED into a
	// commit past review by the prospective-snapshot protocol (Req 11;
	// spec 128 Bead 6 replaced the porcelain CommitAll this refusal used
	// to name with commit-tree-built M/S-stage commits — the refusal
	// reasoning is unchanged, only the mechanism's name). Artifact dirt is
	// already filtered out by the caller. Skipped entirely when the
	// worktree is absent (the missing-worktree partial-failure rerun
	// window).
	if !f.WorktreeAbsent && len(f.UserDirt) > 0 {
		var b strings.Builder
		text := "uncommitted changes in %s — `mindspec complete` would materialize them into a commit past review; commit and re-panel, or revert:"
		if f.BeadID == "" && !p.IsBead() {
			text = "uncommitted changes in %s — approval would include content past review; commit and review the listed paths:"
		}
		fmt.Fprintf(&b, text,
			termsafe.Escape(f.WorktreePath))
		// The "\n  " separator is the template's OWN real newline — only
		// the interpolated path itself is escaped, never the layout.
		for _, d := range f.UserDirt {
			fmt.Fprintf(&b, "\n  %s", termsafe.Escape(d))
		}
		b.WriteString(RawMergeFence(f.BeadID))
		return Decision{Action: Block, Message: b.String()}
	}

	// (8) Incomplete — fewer valid verdicts than expected reviewers. Name
	// the PRESENT verdict files; the missing-slot NAMES are not derivable
	// from the Req 6 schema (it carries only an expected_reviewers int —
	// gate finding T3-2).
	n := p.ExpectedReviewers
	if !f.Res.Complete() {
		present := presentVerdictFiles(f.Res)
		return Decision{Action: Block, Message: fmt.Sprintf(
			"panel %s round %d incomplete: %d/%d verdicts present (%s) — finish /ms-panel-run or tally first%s",
			slug, round, len(f.Res.Verdicts), n, present, RawMergeFence(f.BeadID))}
	}

	// (9) REJECT or hard_block — halt path; no vote count overrides an
	// evidence-bearing gate (the lola-f4a8 artifact-gate rule, mechanized).
	if f.Res.Rejects > 0 || len(f.Res.HardBlocks) > 0 || f.Res.InvalidCount() > 0 || f.Res.unexpectedVeto() {
		detail := fmt.Sprintf("%d/%d APPROVE, %d INVALID%s", f.Res.Approves, n, f.Res.InvalidCount(), f.Res.unexpectedVetoNames()+f.Res.invalidNames())
		if len(f.Res.HardBlocks) > 0 {
			// Escape each slot PER ELEMENT before joining (Spec 116 R2).
			hardBlocks := make([]string, len(f.Res.HardBlocks))
			for i, h := range f.Res.HardBlocks {
				hardBlocks[i] = termsafe.Escape(h)
			}
			detail += fmt.Sprintf(", hard_block from %s", strings.Join(hardBlocks, ", "))
		}
		return Decision{Action: Block, Message: fmt.Sprintf(
			"panel %s round %d: %s — %s recorded — halt path, see /ms-panel-tally%s",
			slug, round, detail, f.Res.vetoSummary(), RawMergeFence(f.BeadID))}
	}

	// (9.5) Every finding must be discharged; approval counts are descriptive.
	if !f.Res.PassRule(true) {
		details := f.Res.UnresolvedNames() + f.ArtifactDetails
		if f.BeadID == "" && !p.IsBead() {
			return Decision{Action: Block, Message: fmt.Sprintf("panel %s round %d: unresolved REQUEST_CHANGES / non-APPROVE verdict(s) from %s", slug, round, details)}
		}
		return Decision{Action: Block, Message: fmt.Sprintf(
			"panel %s round %d: unresolved REQUEST_CHANGES / non-APPROVE verdict(s) from %s — %d/%d APPROVE. Run /ms-bead-fix with %s, then re-panel%s",
			slug, round, details, f.Res.Approves, n, ConsolidatedName(round), RawMergeFence(f.BeadID))}
	}
	return Decision{Action: Allow, AppliedRefutations: f.Res.AppliedRefutations(), AppliedDeferrals: f.Res.EffectiveDeferrals()}
}

// presentVerdictFiles renders the present-verdict-file list for the
// incomplete Block. Missing-slot names are not derivable from the schema,
// so the contract is to enumerate what IS present (T3-2). Malformed files
// (counted as missing) are named separately so the agent can fix them.
func presentVerdictFiles(res *Result) string {
	// Escape each filename PER ELEMENT before joining — escaping the
	// joined whole would also quote the ", " separators (Spec 116 R2).
	var files []string
	for _, v := range res.Verdicts {
		files = append(files, termsafe.Escape(v.File))
	}
	sort.Strings(files)
	out := "present: "
	if len(files) == 0 {
		out += "none"
	} else {
		out += strings.Join(files, ", ")
	}
	if len(res.Malformed) > 0 {
		malformed := make([]string, len(res.Malformed))
		for i, m := range res.Malformed {
			malformed[i] = termsafe.Escape(m)
		}
		out += "; malformed (count as missing): " + strings.Join(malformed, ", ")
	}
	return out
}

// shaEqual compares two git SHAs allowing one to be an abbreviation of the
// other (panel.json may record a short reviewed_head_sha; rev-parse yields
// the full 40-char form).
func shaEqual(a, b string) bool {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	if a == "" || b == "" {
		return false
	}
	if len(a) == len(b) {
		return a == b
	}
	if len(a) < len(b) {
		return strings.HasPrefix(b, a)
	}
	return strings.HasPrefix(a, b)
}

func short(sha string) string {
	sha = strings.TrimSpace(sha)
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// --- injectable fact-gathering (Spec 093 Req 11; Spec 099 leaf seam) --------

// GateIO carries the git/status/ref-not-found I/O the caller injects into
// ResolveGateFacts so internal/panel does the PURE fact filtering
// (userDirt/isArtifactPath) while the CALLER supplies the git I/O — keeping
// internal/panel a leaf (no gitutil/workspace/config import). The hook wires
// these to gitutil.RevParseRef / gitutil.Status / errors.Is(e,
// gitutil.ErrRefNotFound); the in-binary gate (Bead 2) wires the same.
type GateIO struct {
	// SpecDir is resolved by the caller through workspace.SpecsDir.
	SpecDir string
	// SubjectTree returns the subject digest at ref, or an error (fail closed).
	SubjectTree func(scanRoot, ref string, roots SubjectRoots) (string, error)
	// ArtifactTree returns the full tree listing at the round's recorded commit.
	ArtifactTree func(scanRoot, ref string) (string, error)
	// RevParse runs `git rev-parse <ref>` in scanRoot, returning the SHA. A
	// genuine "ref absent" error must satisfy IsRefNotFound; any other error
	// is treated as a transient/structural git error.
	RevParse func(scanRoot, ref string) (string, error)
	// Status runs `git status --porcelain` in the worktree.
	Status func(worktree string) (string, error)
	// IsRefNotFound reports whether a RevParse error is the genuine
	// branch-deleted case (e.g. errors.Is(e, gitutil.ErrRefNotFound)) vs a
	// transient/structural failure.
	IsRefNotFound func(error) bool
	// Worktree resolves the bead worktree path ("" = absent → dirty check
	// skipped). It is a closure (not a pre-resolved string) so the worktree
	// resolution — which may itself cost a `git worktree list` subprocess in
	// the caller — is only paid on the dirty-check path, NEVER on the
	// abandoned / round-mismatch / missing-ref / transient-gitErr short
	// circuits that exit before the dirty check. Subject reads add one
	// subprocess only on a SHA mismatch (Spec 130 R3.3).
	Worktree func() string
}

// ResolveGateFacts gathers the git facts (Req 11) for one matched
// registration and returns the I/O-free GateFacts for PanelGateDecision. The
// caller pre-resolves scanRoot (the panel dir's grandparent) and injects the
// git I/O (incl. the bead-worktree resolver) via deps, so this function does
// NO git wiring that would need an internal import. Matched-path git budget:
// at most three on a SHA mismatch (rev-parse + subject tree + porcelain);
// equal SHAs make no subject read. The worktree resolver is lazy.
func ResolveGateFacts(reg Registration, beadID, scanRoot string, deps GateIO) GateFacts {
	f := GateFacts{BeadID: beadID, Reg: &reg}

	res, err := Tally(reg.Dir)
	if err != nil {
		// Directory unreadable — treat as malformed registration (Block),
		// not "no panel": surface a non-nil Result with PanelErr so the
		// decision blocks rather than fails open.
		f.Res = &Result{Dir: reg.Dir, PanelErr: err}
		return f
	}
	f.Res = res

	// Abandoned and round-mismatch decisions need no git. To honor T3-1
	// (abandoned checked before staleness) AND keep the git budget low, we
	// cheaply skip git entirely on the abandoned/round-mismatch paths.
	if res.Panel != nil && (res.Panel.Abandoned || res.RoundMismatch) {
		return f
	}

	// Composition-waist gate (ADR-0042 §1, spec 120 R2 ingress (iv)):
	// beadID feeds an inline "bead/"+beadID branch-ref concat below — the
	// SAME class the workspace.BeadBranch waist gates elsewhere — so it is
	// validated in-package (keeping internal/panel a leaf) before the
	// concat is ever built or a git subprocess is spawned. A malformed
	// beadID is treated as a git error (fail-closed, matching the
	// unreadable-registration disposition above) rather than composing a
	// hostile ref. beadID == "" is the LEGITIMATE non-bead-panel sentinel
	// (deps.RevParse is nonBeadTargetRevParse in that case, which ignores
	// the ref argument entirely and resolves reg.Panel.Target instead) —
	// only a NON-EMPTY, malformed beadID is gated here.
	if beadID != "" {
		if err := idvalidate.BeadID(beadID); err != nil {
			f.GitErr = fmt.Errorf("invalid bead id %q: %w", beadID, err)
			return f
		}
	}

	// (6) staleness — one `git rev-parse bead/<id>` in the scan root. The
	// bead-branch ref "bead/"+beadID (== workspace.BeadBranchPrefix) is
	// inlined to keep internal/panel a leaf.
	sha, rerr := deps.RevParse(scanRoot, "bead/"+beadID)
	if rerr != nil {
		// A pinned spec branch must be readable. Check its error marker
		// before the missing-ref sentinel: a present ref with a missing
		// commit is not a confirmed deletion.
		// Distinguish a GENUINE missing ref (branch deleted — the merge
		// landed) from a transient/structural git error. Only the former is
		// the rerun-after-merge pass-through (Req 11); a transient error is
		// surfaced as a distinct Warn so it is not mistaken for a confirmed
		// deletion (round-2 false-clear).
		if errors.Is(rerr, ErrSubjectHead) {
			f.SubjectErr = rerr
		} else if deps.IsRefNotFound != nil && deps.IsRefNotFound(rerr) {
			f.MissingRef = true
		} else {
			f.GitErr = rerr
		}
		return f
	}
	f.HeadSHA = sha
	f.ArtifactDetails = ArtifactPathDetails(res, scanRoot, deps.ArtifactTree)
	if p := res.Panel; p != nil && p.SubjectDigest != "" && p.ReviewedHeadSHA != "" && !shaEqual(p.ReviewedHeadSHA, sha) {
		if deps.SubjectTree == nil {
			f.SubjectErr = fmt.Errorf("subject-tree reader unavailable")
		} else {
			f.SubjectDigest, f.SubjectErr = deps.SubjectTree(scanRoot, sha, Subject(p.Gate, PanelSpecDir(reg.Dir, deps.SpecDir)))
			if f.SubjectErr == nil && f.SubjectDigest == "" {
				f.SubjectErr = fmt.Errorf("empty subject digest")
			}
		}
		if f.SubjectErr != nil {
			return f
		}
	}

	// (7) dirty tree — one `git status --porcelain` in the resolved bead
	// worktree (Req 11). The worktree resolver is invoked HERE (not earlier)
	// so its cost is only paid on the dirty-check path; "" means absent.
	wt := ""
	if deps.Worktree != nil {
		wt = deps.Worktree()
	}
	if wt == "" {
		f.WorktreeAbsent = true
		return f
	}
	f.WorktreePath = wt
	out, serr := deps.Status(wt)
	if serr != nil {
		// Porcelain failed (missing worktree raced in) → skip the dirty
		// check, mirroring the missing-worktree pass-through (Req 11).
		f.WorktreeAbsent = true
		return f
	}
	f.UserDirt = userDirtPaths(out, PanelSpecDir(reg.Dir, deps.SpecDir))
	return f
}

// PanelDirScanRoot returns the scan root that owns a panel dir: the panel
// dir's grandparent. This resolves BOTH supported conventions (Spec 106 Bead
// 4) without special-casing: a repo-root `review/<slug>` grandparent is the
// repo root, and a co-located `<spec-dir>/reviews/<slug>` grandparent is the
// spec dir — each a valid git workdir for the gate's bead/<id> staleness
// rev-parse. Pure path math (filepath only) so callers need not re-derive it.
func PanelDirScanRoot(panelDir string) string {
	return filepath.Dir(filepath.Dir(panelDir))
}

// userDirtPaths parses `git status --porcelain` output and returns the
// user-authored dirty paths, filtering out ADR-0025 artifact paths
// from ToolArtifactPaths, which never block (Req 11; spec 130 R4.6).
// Pure path filtering adds no subprocess call. ADR-0037 (Spec 130) permits
// three calls on a SHA mismatch: rev-parse, subject-tree read, and porcelain.
func userDirtPaths(porcelain string, specDir ...string) []string {
	var out []string
	for _, line := range strings.Split(porcelain, "\n") {
		path, _ := PorcelainPath(line)
		if path == "" || isArtifactPath(path, specDir...) {
			continue
		}
		out = append(out, path)
	}
	return out
}

// PorcelainPath decodes a git porcelain v1 line, selecting the destination
// for renames and copies. An arrow inside a quoted name is part of the name.
// On invalid quoting it returns the raw path and an error, so callers can
// either refuse the status or classify the raw path as user dirt.
func PorcelainPath(line string) (string, error) {
	line = strings.TrimRight(line, "\r")
	if len(line) < 4 {
		return "", nil
	}
	path := strings.TrimSpace(line[3:])
	if strings.ContainsAny(line[:2], "RC") {
		renameTail := path
		if strings.HasPrefix(path, "\"") {
			quoted, err := strconv.QuotedPrefix(path)
			if err != nil {
				return path, err
			}
			renameTail = path[len(quoted):]
		}
		if idx := strings.Index(renameTail, " -> "); idx >= 0 {
			path = strings.TrimSpace(renameTail[idx+4:])
		}
	}
	if strings.HasPrefix(path, "\"") {
		decoded, err := strconv.Unquote(path)
		if err != nil {
			return path, err
		}
		path = decoded
	}
	return path, nil
}

func isArtifactPath(p string, specDir ...string) bool {
	dir := ""
	if len(specDir) > 0 {
		dir = specDir[0]
	}
	return IsToolArtifactPath(p, dir)
}

// RebindDecision applies the ordinary registration and vote checks with only
// staleness ignored. Abandonment remains a refusal, never an escape hatch.
func RebindDecision(reg Registration, res *Result) Decision {
	if res == nil || res.Panel == nil {
		return Decision{Action: Block, Message: "panel registration is unreadable"}
	}
	f := GateFacts{Reg: &reg, Res: res, HeadSHA: res.Panel.ReviewedHeadSHA, WorktreeAbsent: true}
	if res.Panel.IsBead() {
		f.BeadID = *res.Panel.BeadID
	}
	d := PanelGateDecision(f)
	if d.Action == Warn {
		d.Action = Block
		if res.Panel.Abandoned {
			d.Message = fmt.Sprintf("rebind refuses an abandoned panel: %s", termsafe.Escape(strings.TrimSpace(res.Panel.AbandonReason)))
		}
	}
	return d
}

// RebindPaths keeps every diff path except the derived subject exclusions.
// Subject roots do not limit the judgement recorded by a rebind.
func RebindPaths(paths []string, subject SubjectRoots) []string {
	out := make([]string, 0, len(paths))
	for _, name := range paths {
		excluded := false
		for _, pattern := range subject.Exclusions {
			if strings.HasSuffix(pattern, "/**") {
				prefix := strings.TrimSuffix(pattern, "/**")
				parts := strings.Split(name, "/")
				depth := len(strings.Split(prefix, "/"))
				if len(parts) > depth {
					excluded, _ = path.Match(prefix, strings.Join(parts[:depth], "/"))
				}
			} else {
				excluded = name == pattern
			}
			if excluded {
				break
			}
		}
		if !excluded {
			out = append(out, name)
		}
	}
	return out
}

// LedgerBackedRebinds returns only snapshot entries whose complete payload
// matches a rebind event or the first round's carried baseline, ignoring time.
// Legacy exemption does not confer landed-SHA evidence on an unbacked rebind.
func LedgerBackedRebinds(dir string, p Panel) []Rebind {
	if _, err := LegacyPanel(dir); err != nil {
		return nil
	}
	lines, err := ReadLedger(dir)
	if err != nil {
		return nil
	}
	var backed []Rebind
	first := true
	for _, line := range lines {
		var kind recordDiscriminator
		if json.Unmarshal(line, &kind) != nil {
			return nil
		}
		switch kind.Record {
		case RecordRebind:
			var r Rebind
			if json.Unmarshal(line, &r) != nil {
				return nil
			}
			backed = append(backed, r)
		case RecordRound:
			if first {
				var r RoundRecord
				if json.Unmarshal(line, &r) != nil {
					return nil
				}
				backed = append(backed, r.Carried.Rebinds...)
				first = false
			}
		}
	}
	var matched []Rebind
	for _, r := range p.Rebinds {
		if matchesRebind(r, backed) {
			matched = append(matched, r)
		}
	}
	return matched
}

// PanelGateDecision applies one decision engine and renders lifecycle refusals.
func PanelGateDecision(f GateFacts) Decision {
	d := panelGateDecision(f)
	if f.BeadID != "" || f.Reg == nil || f.Reg.Panel.IsBead() {
		return d
	}
	p := f.Reg.Panel
	if f.Res != nil && f.Res.Panel != nil {
		p = *f.Res.Panel
	}
	if !IsLifecycleGate(p.Gate) {
		return d
	}
	if d.Action == Warn && (f.GitErr != nil || f.MissingRef) {
		d.Message = fmt.Sprintf("target %s cannot be verified", termsafe.Escape(p.Target))
		if f.MissingRef {
			d.Message = fmt.Sprintf("target %s no longer exists", termsafe.Escape(p.Target))
		}
	}
	if d.Action == Block || (d.Action == Warn && (f.GitErr != nil || f.MissingRef)) {
		d.Message, _, _ = strings.Cut(d.Message, "\nrecovery:")
		d.Message = fmt.Sprintf("gate %s panel %s: %s\nrecovery: mindspec panel create %s --spec %s --gate %s --target spec/%s --round %d",
			termsafe.Escape(p.Gate), termsafe.Escape(f.Reg.Slug()), d.Message,
			promptShellQuote(f.Reg.Slug()), promptShellQuote(p.Spec), termsafe.Escape(p.Gate), promptShellQuote(p.Spec), p.Round+1)
	}
	return d
}
