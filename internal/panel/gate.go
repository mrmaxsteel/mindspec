package panel

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// ADR-0037 AMENDMENT NOTE (Spec 099): this is the SINGLE home of the
// panel-gate enforced contract (registration §1, round derivation §2, N−1
// threshold §3, staleness §4, dirty-tree §5, fail-open/fail-closed asymmetry
// §6, escape hatches §7, trust boundary §8). It was relocated here from
// internal/hook/precomplete.go so that BOTH the in-binary `mindspec complete`
// gate (the AUTHORITATIVE enforcement point, Spec 099 Bead 2) and the
// PreToolUse hook (now a defense-in-depth BACKSTOP) invoke the IDENTICAL
// decision over IDENTICAL facts — they cannot disagree by construction.
//
// internal/panel is a dependency-clean LEAF: this file imports NO internal
// package. The git/status/ref-not-found I/O is supplied by the CALLER through
// the GateIO seam (function closures), and the bead-branch prefix is inlined
// as the literal "bead/" (== workspace.BeadBranchPrefix) rather than importing
// internal/workspace, so the leaf invariant holds.

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
type Decision struct {
	Action  GateAction
	Message string
}

// RawMergeFence is appended to every Block message (Spec 093 Req 12 / G3-1,
// ADR-0037 §8). Once the gate blocks `mindspec complete`, a raw
// `git merge bead/<id>` on the spec branch is the obvious workaround; the
// matcher only fires on `mindspec complete` and mindspec installs no
// pre-merge-commit git hook, so the fence is prose by necessity.
func RawMergeFence(beadID string) string {
	return fmt.Sprintf(
		"\nDo NOT bypass with raw `git merge bead/%s` — it skips bd closure, "+
			"worktree cleanup, and this gate; only `mindspec complete` merges bead branches.",
		beadID)
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
//	(3) abandoned     → Allow + Warn  (BEFORE staleness — an abandoned
//	                    panel whose branch gained commits must never be
//	                    false-Blocked by the stale-SHA rule)
//	(4) round mismatch→ Block
//	(5) missing ref   → Allow + Warn  (rerun-after-merge)
//	(5b) transient gitErr → Allow + Warn  (honest, NOT "merge landed")
//	(6) stale SHA     → Block  (the lola-f4a8 pin)
//	(7) dirty tree    → Block  (CommitAll bypass; skipped when worktree absent)
//	(8) incomplete    → Block  (verdicts < expected_reviewers)
//	(9) REJECT/hard   → Block  (halt path, no vote count overrides)
//	(10) threshold    → Allow iff APPROVE ≥ N−1, else Block
//
// false POSITIVES (a wrongful Block) are the pinned bug class (Req 9); the
// missing-ref and missing-worktree pass-throughs exist to keep the
// documented partial-failure recovery rerun unblocked.
func PanelGateDecision(f GateFacts) Decision {
	// (0) Escape hatch — env-only, audited. Never names the variable in a
	// Block (this is a Warn/Allow path, so HC-7 is moot here, but we still
	// keep the message hatch-name-free except for this legitimate audit).
	if f.SkipEnv {
		return Decision{Action: Warn, Message: fmt.Sprintf(
			"panel gate skipped via %s for %s", SkipPanelEnv, f.BeadID)}
	}

	// (1) Fail-open: no registered panel for the bead.
	if f.Reg == nil || f.Res == nil {
		return Decision{Action: Allow}
	}

	slug := f.Reg.Slug()

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

	// (3) Abandoned — legitimate exit, audited on bead metadata at complete
	// time (Req 13e). Checked BEFORE staleness (T3-1) so an abandoned panel
	// whose branch advanced is never false-Blocked.
	if p.Abandoned {
		reason := strings.TrimSpace(p.AbandonReason)
		if reason == "" {
			reason = "(no reason recorded — abandon_reason is required; /ms-panel-tally abandon procedure)"
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
		return Decision{Action: Warn, Message: fmt.Sprintf(
			"panel for %s references branch bead/%s, which no longer exists — assuming the merge already landed; "+
				"deferring to mindspec complete's own handling", f.BeadID, f.BeadID)}
	}

	// (5b) Transient/structural git error — the staleness rev-parse could not
	// run (not a clean "ref absent"). The spec's posture is deliberately
	// fail-open (Req 11/12, advisory-backstopped), so we still Allow+Warn; but
	// — unlike (5) — we do NOT claim the merge landed, because a transient
	// error is NOT evidence the branch was deleted. Surfacing it honestly
	// closes the round-2 false-clear (a transient error conflated with a
	// genuine deletion) without false-blocking a legitimate completion.
	if f.GitErr != nil {
		return Decision{Action: Warn, Message: fmt.Sprintf(
			"panel for %s: could not verify branch bead/%s (transient git error: %v) — staleness check skipped; "+
				"proceeding per the gate's fail-open posture, but this is NOT a confirmed merge",
			f.BeadID, f.BeadID, f.GitErr)}
	}

	// (6) Stale SHA — verdicts reviewed a different commit. BLOCK, never
	// Warn (a Warn here is the same prose-under-pressure failure the gate
	// exists to close — the lola-f4a8 bypass class, Req 11).
	if p.ReviewedHeadSHA != "" && f.HeadSHA != "" && !shaEqual(p.ReviewedHeadSHA, f.HeadSHA) {
		return Decision{Action: Block, Message: fmt.Sprintf(
			"panel round %d reviewed %s, branch now at %s — commits landed after review; "+
				"bump round and re-panel (/ms-panel-run step 0)%s",
			round, short(p.ReviewedHeadSHA), short(f.HeadSHA), RawMergeFence(f.BeadID))}
	}

	// (7) Dirty tree — uncommitted USER edits would be auto-committed past
	// review by CommitAll (Req 11). Artifact dirt is already filtered out by
	// the caller. Skipped entirely when the worktree is absent (the
	// missing-worktree partial-failure rerun window).
	if !f.WorktreeAbsent && len(f.UserDirt) > 0 {
		var b strings.Builder
		fmt.Fprintf(&b, "uncommitted changes in %s — `mindspec complete` would auto-commit them past review (CommitAll); commit and re-panel, or revert:",
			f.WorktreePath)
		for _, d := range f.UserDirt {
			fmt.Fprintf(&b, "\n  %s", d)
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
	if f.Res.Rejects > 0 || len(f.Res.HardBlocks) > 0 {
		detail := fmt.Sprintf("%d/%d APPROVE", f.Res.Approves, n)
		if len(f.Res.HardBlocks) > 0 {
			detail += fmt.Sprintf(", hard_block from %s", strings.Join(f.Res.HardBlocks, ", "))
		}
		return Decision{Action: Block, Message: fmt.Sprintf(
			"panel %s round %d: %s — HARD block / REJECT recorded — halt path, see /ms-panel-tally%s",
			slug, round, detail, RawMergeFence(f.BeadID))}
	}

	// (10) Threshold — N−1 (Req 12, single home in panel.ApproveThreshold).
	threshold := p.ApproveThreshold()
	if f.Res.Approves >= threshold && threshold > 0 {
		return Decision{Action: Allow}
	}
	return Decision{Action: Block, Message: fmt.Sprintf(
		"panel %s round %d: %d/%d APPROVE — threshold is %d/%d. Run /ms-bead-fix with %s, then re-panel%s",
		slug, round, f.Res.Approves, n, threshold, n, ConsolidatedName(round), RawMergeFence(f.BeadID))}
}

// presentVerdictFiles renders the present-verdict-file list for the
// incomplete Block. Missing-slot names are not derivable from the schema,
// so the contract is to enumerate what IS present (T3-2). Malformed files
// (counted as missing) are named separately so the agent can fix them.
func presentVerdictFiles(res *Result) string {
	var files []string
	for _, v := range res.Verdicts {
		files = append(files, v.File)
	}
	sort.Strings(files)
	out := "present: "
	if len(files) == 0 {
		out += "none"
	} else {
		out += strings.Join(files, ", ")
	}
	if len(res.Malformed) > 0 {
		out += "; malformed (count as missing): " + strings.Join(res.Malformed, ", ")
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
	// circuits that exit before the dirty check (preserving the two-subprocess
	// matched-path budget).
	Worktree func() string
}

// ResolveGateFacts gathers the git facts (Req 11) for one matched
// registration and returns the I/O-free GateFacts for PanelGateDecision. The
// caller pre-resolves scanRoot (the panel dir's grandparent) and injects the
// git I/O (incl. the bead-worktree resolver) via deps, so this function does
// NO git wiring that would need an internal import. Matched-path git budget:
// at most TWO subprocesses (rev-parse + porcelain) — the worktree resolver is
// only invoked on the dirty-check path.
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

	// (7) staleness — one `git rev-parse bead/<id>` in the scan root. The
	// bead-branch ref "bead/"+beadID (== workspace.BeadBranchPrefix) is
	// inlined to keep internal/panel a leaf.
	sha, rerr := deps.RevParse(scanRoot, "bead/"+beadID)
	if rerr != nil {
		// Distinguish a GENUINE missing ref (branch deleted — the merge
		// landed) from a transient/structural git error. Only the former is
		// the rerun-after-merge pass-through (Req 11); a transient error is
		// surfaced as a distinct Warn so it is not mistaken for a confirmed
		// deletion (round-2 false-clear).
		if deps.IsRefNotFound != nil && deps.IsRefNotFound(rerr) {
			f.MissingRef = true
		} else {
			f.GitErr = rerr
		}
		return f
	}
	f.HeadSHA = sha

	// (8) dirty tree — one `git status --porcelain` in the resolved bead
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
	f.UserDirt = userDirtPaths(out)
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
// (.beads/issues.jsonl) which are designed-for and never block (Req 11).
// Pure path filtering — no bd-export normalization call (matched-path git
// budget stays at two subprocesses).
func userDirtPaths(porcelain string) []string {
	var out []string
	for _, line := range strings.Split(porcelain, "\n") {
		line = strings.TrimRight(line, "\r")
		if len(line) < 4 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		if idx := strings.Index(path, " -> "); idx >= 0 {
			path = strings.TrimSpace(path[idx+4:])
		}
		if path == "" || isArtifactPath(path) {
			continue
		}
		out = append(out, path)
	}
	return out
}

// artifactPaths mirrors internal/next's ADR-0025 classification
// (.beads/issues.jsonl). Kept as a local copy rather than importing
// internal/next to avoid a panel→next dependency edge; the list is one entry
// today and any addition is a one-line append in both places.
var artifactPaths = []string{
	".beads/issues.jsonl",
}

func isArtifactPath(p string) bool {
	for _, a := range artifactPaths {
		if p == a {
			return true
		}
	}
	return false
}
