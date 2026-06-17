package instruct

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Spec 093 Req 14 (open-panel-rounds block) + Req 15 (SessionStart
// auto-include). This is the human/agent-readable COMPANION to the
// Bead 4 pre-complete hook gate: B4 BLOCKS `mindspec complete`, B5
// INFORMS the agent where it stands before attempting it (FINDINGS
// item 8 — post-compaction panel-state recovery).
//
// The decision logic here is a faithful, READ-ONLY reproduction of the
// Bead 4 decision matrix (Spec 093 Req 11/12): verdict count vs
// expected, the N−1 APPROVE threshold (computed by the SAME
// panel.Panel.ApproveThreshold the gate uses), reviewed_head_sha
// staleness, round/filename mismatch, REJECT/hard_block, abandonment.
// It renders the gate's would-be verdict ("gate would PASS/BLOCK") so a
// compacted agent knows whether `mindspec complete` will succeed —
// without itself making any decision (no exit code, no enforcement).
//
// Boundary: panel.Tally/Scan are fs-only (zero git). Staleness is a
// git comparison, so the CALLER resolves each panel's live branch SHA
// (one `git rev-parse bead/<id>` per bead panel) and hands it to the
// pure formatter below. The formatter itself makes zero subprocess
// calls and is fully unit-testable.

// PanelGateVerdict is the would-be pre-complete-hook outcome for one
// panel, derived without enforcing anything.
type PanelGateVerdict int

const (
	// GatePass — the gate would let `mindspec complete` through.
	GatePass PanelGateVerdict = iota
	// GateBlock — the gate would block `mindspec complete`.
	GateBlock
	// GateWarn — the gate would pass with a Warn (abandoned panels,
	// missing-ref pass-through); the complete proceeds but is audited.
	GateWarn
)

// PanelStateEntry is one panel's resolved state: the fs-derived tally
// plus the caller-resolved live branch SHA (the single git input). It
// is the formatter's pure input — build it from a panel.Result and a
// branch-SHA lookup, then format with no further I/O.
type PanelStateEntry struct {
	// Slug is the panel directory basename (review/<slug>).
	Slug string
	// Tally is the fs-derived round/verdict state (panel.Tally).
	Tally *panel.Result

	// LiveBranchSHA is the current `git rev-parse bead/<bead-id>` of the
	// panel's bead branch, trimmed. Empty means the caller could not
	// resolve it: BranchMissing distinguishes "branch gone" (the
	// rerun-after-merge Pass-through case) from "not a bead panel /
	// not looked up".
	LiveBranchSHA string
	// BranchMissing is true when `git rev-parse bead/<id>` FAILED
	// because the branch no longer exists — the documented
	// rerun-after-merge case (Spec 093 Req 11 missing-ref semantics):
	// the gate passes through to complete's own idempotent handling.
	BranchMissing bool
}

// shaPrefix returns the 7-char short form used in Block texts, or the
// whole string when shorter. Empty in → empty out.
func shaPrefix(sha string) string {
	if len(sha) <= 7 {
		return sha
	}
	return sha[:7]
}

// verdict computes the would-be gate decision and the one-line reason,
// mirroring the Bead 4 decision matrix (Spec 093 Reqs 11/12) read-only.
// It assumes e.Tally != nil.
func (e PanelStateEntry) verdict() (PanelGateVerdict, string) {
	r := e.Tally

	// Unregistered (no panel.json) — fail-open, no gate (HC-4). Should
	// not appear in the block (Scan only returns registered dirs) but
	// stay defensive.
	if r.Panel == nil {
		if r.PanelErr != nil {
			return GateBlock, "panel.json present but unreadable — fix or remove it"
		}
		return GatePass, "no panel.json — unregistered, gate does not apply"
	}
	p := r.Panel
	n := p.ExpectedReviewers
	thr := p.ApproveThreshold()

	// Abandoned → Pass with Warn (legitimate exit, audited; Req 12).
	if p.Abandoned {
		reason := p.AbandonReason
		if strings.TrimSpace(reason) == "" {
			reason = "(no reason recorded — abandon_reason is required)"
		}
		return GateWarn, fmt.Sprintf("panel abandoned: %s", reason)
	}

	// Round / filename-max disagreement → Block (Req 11).
	if r.RoundMismatch {
		return GateBlock, fmt.Sprintf(
			"panel.json round (%d) out of date vs verdict files (round %d) — re-run /ms-panel-run step 0",
			p.Round, r.LatestRound)
	}

	// reviewed_head_sha staleness (Req 11). Only meaningful for bead
	// panels whose live branch SHA the caller resolved.
	if p.IsBead() {
		switch {
		case e.BranchMissing:
			// rerun-after-merge: Pass with Warn, defer to complete.
			return GateWarn, fmt.Sprintf(
				"branch bead/%s no longer exists — assuming the merge landed; deferring to `mindspec complete`",
				*p.BeadID)
		case e.LiveBranchSHA != "" && e.LiveBranchSHA != p.ReviewedHeadSHA:
			return GateBlock, fmt.Sprintf(
				"round %d reviewed %s, branch now at %s — commits landed after review; bump round and re-panel (/ms-panel-run step 0)",
				r.LatestRound, shaPrefix(p.ReviewedHeadSHA), shaPrefix(e.LiveBranchSHA))
		}
	}

	// Incomplete — verdicts below expected (Req 12).
	if !r.Complete() {
		return GateBlock, fmt.Sprintf(
			"round %d incomplete: %d/%d verdicts present (missing %d) — finish /ms-panel-run or tally first",
			r.LatestRound, len(r.Verdicts), n, r.MissingCount())
	}

	// REJECT or hard_block → Block (Req 12), takes precedence over the
	// APPROVE count.
	if r.Rejects > 0 || len(r.HardBlocks) > 0 {
		return GateBlock, fmt.Sprintf(
			"%d/%d APPROVE but a REJECT or hard_block is recorded — halt path, see /ms-panel-tally",
			r.Approves, n)
	}

	// Threshold (N−1) met → Pass.
	if r.Approves >= thr {
		return GatePass, fmt.Sprintf(
			"%d/%d APPROVE — meets threshold %d/%d; `mindspec complete` would proceed",
			r.Approves, n, thr, n)
	}

	// Short of threshold → Block (Req 12).
	return GateBlock, fmt.Sprintf(
		"%d/%d APPROVE — threshold is %d/%d. Run /ms-bead-fix with %s, then re-panel",
		r.Approves, n, thr, n, panel.ConsolidatedName(r.LatestRound))
}

// renderPanelState formats the open-panel-rounds block (Spec 093
// Req 14) from already-resolved entries. PURE: no git, no fs, no
// subprocess — its inputs (tally + live SHA) are pre-resolved by the
// caller, which is what makes it unit-testable. Returns "" when there
// are no registered panels (the zero-cost / clean-state contract:
// callers append nothing).
func renderPanelState(entries []PanelStateEntry) string {
	if len(entries) == 0 {
		return ""
	}

	// Deterministic order by slug.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Slug < entries[j].Slug })

	var b strings.Builder
	b.WriteString("## Open Panel Rounds\n\n")
	b.WriteString("Where each open review panel stands vs the `mindspec complete` gate (Bead 4). ")
	b.WriteString("This INFORMS; the `mindspec complete` gate ENFORCES.\n")

	for _, e := range entries {
		v, reason := e.verdict()
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("- **%s** — %s\n", e.Slug, gateLabel(v)))
		b.WriteString(fmt.Sprintf("  - %s\n", reason))
		if e.Tally != nil && e.Tally.Panel != nil {
			b.WriteString(fmt.Sprintf("  - latest round %d · %d/%d verdicts · %d APPROVE (threshold %d)\n",
				e.Tally.LatestRound,
				len(e.Tally.Verdicts),
				e.Tally.Panel.ExpectedReviewers,
				e.Tally.Approves,
				e.Tally.Panel.ApproveThreshold()))
			if e.Tally.HasConsolidated {
				b.WriteString(fmt.Sprintf("  - %s is present (feed it to /ms-bead-fix)\n",
					panel.ConsolidatedName(e.Tally.LatestRound)))
			}
		}
	}

	return b.String()
}

// gateLabel renders the would-be verdict as the "gate would …" line
// computed by the same panel.Tally the hook uses.
func gateLabel(v PanelGateVerdict) string {
	switch v {
	case GatePass:
		return "gate would PASS"
	case GateWarn:
		return "gate would PASS (with Warn)"
	default:
		return "gate would BLOCK"
	}
}

// BranchSHAResolver resolves the live SHA of a bead branch
// (`git rev-parse bead/<bead-id>`). It returns (sha, exists): exists ==
// false means the branch is gone (the rerun-after-merge Pass-through).
// Injected so gatherPanelState stays testable and the single git input
// is explicit (Spec 093 Req 11 / ADR-0030 subprocess budget).
type BranchSHAResolver func(beadID string) (sha string, exists bool)

// HasIncompletePanel reports whether any registered panel under the
// given roots has an incomplete latest round (Spec 093 Req 15
// auto-include condition). It is fs-only (panel.Scan + panel.Tally,
// zero git, zero bd) — the ONLY work the SessionStart hook performs
// outside the auto-include branch, so a session with no open panel pays
// just one glob + stat per panel dir and adds zero git/bd subprocess
// cost (12s budget). A registered-but-malformed panel counts as
// incomplete (it needs the agent's attention, and surfacing it is
// cheaper than silently dropping it).
func HasIncompletePanel(roots ...string) bool {
	for _, reg := range panel.Scan(roots...) {
		res, err := panel.Tally(reg.Dir)
		if err != nil {
			continue
		}
		if res.PanelErr != nil {
			return true
		}
		if !res.Complete() {
			return true
		}
	}
	return false
}

// gatherPanelState scans the given roots for registered panels and
// builds one PanelStateEntry per panel, resolving each bead panel's
// live branch SHA via resolve. The fs Scan/Tally are zero-git; the only
// git work is one resolve call per bead panel (Req 14 budget). Returns
// nil when no panels are registered (zero added cost — the caller
// appends nothing).
func gatherPanelState(resolve BranchSHAResolver, roots ...string) []PanelStateEntry {
	regs := panel.Scan(roots...)
	if len(regs) == 0 {
		return nil
	}
	var entries []PanelStateEntry
	for _, reg := range regs {
		res, err := panel.Tally(reg.Dir)
		if err != nil {
			continue
		}
		entry := PanelStateEntry{Slug: reg.Slug(), Tally: res}
		if res.Panel != nil && res.Panel.IsBead() && resolve != nil {
			sha, exists := resolve(*res.Panel.BeadID)
			if exists {
				entry.LiveBranchSHA = sha
			} else {
				entry.BranchMissing = true
			}
		}
		entries = append(entries, entry)
	}
	return entries
}

// --- In-progress-beads block (Spec 093 Req 14 bullet 1) ----------------
//
// Lists the IN_PROGRESS beads of the active epics with their worktree
// path and last-commit summary, CAPPED at the active bead + at most 3
// other in-progress beads (deterministic bd-id order); the remainder is
// summarized as "… and N more (no git detail)". The cap bounds the git
// subprocess fan-out per ADR-0030 (Req 14 budget): only the capped beads
// get a `git log -1 --oneline` call.

// inProgressDetailCap is the number of in-progress beads that get full
// git detail: the active bead + at most 3 others (Spec 093 Req 14
// bullet 1 / AC L1098-1099 — "6 in-progress beads → git detail for
// active+3 only"). The 4th..Nth are summarized.
const inProgressDetailCap = 4

// BeadStateEntry is one in-progress bead's resolved display state: the
// caller (gatherInProgressBeads) fills Worktree + LastCommit by reading
// git/bd, so the renderInProgressBeads formatter stays pure and
// directly unit-testable (the cap-test target).
type BeadStateEntry struct {
	// ID is the bead ID (bd id), e.g. "mindspec-cter.5".
	ID string
	// Title is the bead's title (best-effort; may be empty).
	Title string
	// Worktree is the resolved worktree path, or "" if none is checked
	// out for this bead.
	Worktree string
	// LastCommit is the `git log -1 --oneline bead/<id>` summary, or ""
	// when the branch is unresolved / beyond the detail cap.
	LastCommit string
	// Active marks the currently-claimed bead; it always sorts first and
	// always receives git detail (never summarized away by the cap).
	Active bool
}

// renderInProgressBeads formats the in-progress-beads block (Spec 093
// Req 14 bullet 1) from already-resolved entries. PURE: no git/bd/fs —
// its inputs are pre-resolved by gatherInProgressBeads, which is what
// makes the cap directly testable. Returns "" when there are no
// in-progress beads (zero-cost contract). The caller passes entries in
// the order they should render; this formatter applies the active+3 cap
// and summarizes the remainder.
func renderInProgressBeads(entries []BeadStateEntry) string {
	if len(entries) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("## In-Progress Beads\n\n")
	b.WriteString("Claimed beads (IN_PROGRESS) with their worktree and last commit. ")
	b.WriteString("Git detail is capped at the active bead + 3 others (Req 14).\n")

	for i, e := range entries {
		if i >= inProgressDetailCap {
			break
		}
		b.WriteString("\n")
		label := e.ID
		if e.Active {
			label = e.ID + " (active)"
		}
		if e.Title != "" {
			b.WriteString(fmt.Sprintf("- **%s** — %s\n", label, e.Title))
		} else {
			b.WriteString(fmt.Sprintf("- **%s**\n", label))
		}
		if e.Worktree != "" {
			b.WriteString(fmt.Sprintf("  - worktree: `%s`\n", e.Worktree))
		} else {
			b.WriteString("  - worktree: (none checked out)\n")
		}
		if e.LastCommit != "" {
			b.WriteString(fmt.Sprintf("  - last commit: %s\n", e.LastCommit))
		} else {
			b.WriteString("  - last commit: (branch unresolved)\n")
		}
	}

	// Remainder beyond the cap → one summary line, no git detail (Spec
	// 093 Req 14 bullet 1 / AC L1098-1099 verbatim wording).
	if remainder := len(entries) - inProgressDetailCap; remainder > 0 {
		b.WriteString(fmt.Sprintf("\n- … and %d more (no git detail)\n", remainder))
	}

	return b.String()
}

// inProgressBead is the minimal bd shape gatherInProgressBeads decodes
// from `bd list --status=in_progress`.
type inProgressBead struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// inProgressLister returns the in-progress beads of the active epics, in
// deterministic bd-id order. Injected so gatherInProgressBeads is
// testable without bd.
type inProgressLister func() ([]inProgressBead, error)

// beadLastCommitFn resolves the `git log -1 --oneline bead/<id>` summary
// for a bead. Injected (defaults to git) so the gatherer is testable and
// the single git fan-out point is explicit (Req 14 cap budget).
type beadLastCommitFn func(beadID string) string

// gitBeadLastCommit is the production beadLastCommitFn: the one-line tip
// summary of bead/<id>, or "" when the branch is unresolved.
func gitBeadLastCommit(beadID string) string {
	line, err := gitutil.LogOneline("", workspace.BeadBranch(beadID))
	if err != nil {
		return ""
	}
	return line
}

// gatherInProgressBeads builds the (capped) BeadStateEntry list for the
// in-progress-beads block. It is the IO half (bd list + worktree scan +
// git log); the pure renderInProgressBeads does the formatting and cap.
//
// Ordering (Spec 093 Req 14 bullet 1): the active bead first, then the
// other in-progress beads in deterministic bd-id order. The git
// last-commit lookup runs ONLY for the active bead + the first 3 others
// (inProgressDetailCap) — the remainder carries no git detail, bounding
// the subprocess fan-out (ADR-0030). Worktree resolution reuses
// resolveBeadWorktree (the existing helper) and is likewise capped.
func gatherInProgressBeads(list inProgressLister, lastCommit beadLastCommitFn, activeBead string) []BeadStateEntry {
	beads, err := list()
	if err != nil || len(beads) == 0 {
		return nil
	}

	// Deterministic bd-id order, then float the active bead to the front
	// so it always lands inside the detail cap (Req 14).
	sort.Slice(beads, func(i, j int) bool { return beads[i].ID < beads[j].ID })
	sort.SliceStable(beads, func(i, j int) bool {
		return beads[i].ID == activeBead && beads[j].ID != activeBead
	})

	entries := make([]BeadStateEntry, 0, len(beads))
	for i, bd := range beads {
		e := BeadStateEntry{ID: bd.ID, Title: bd.Title, Active: bd.ID == activeBead && activeBead != ""}
		// Resolve worktree + last commit ONLY within the detail cap; the
		// summarized remainder pays no git/worktree cost (Req 14 budget).
		if i < inProgressDetailCap {
			e.Worktree = resolveBeadWorktree(bd.ID)
			if lastCommit != nil {
				e.LastCommit = lastCommit(bd.ID)
			}
		}
		entries = append(entries, e)
	}
	return entries
}

// activeEpicInProgressLister is the production inProgressLister: it lists
// the in-progress children of the active (open/in_progress) epics via
// the shared phase.Cache (PERF-1 — reuses the cache's epic/children bd
// calls rather than issuing a fresh `bd list`), deduping by id.
func activeEpicInProgressLister(cache *phase.Cache) inProgressLister {
	return func() ([]inProgressBead, error) {
		epics, err := cache.ActiveEpics()
		if err != nil {
			return nil, err
		}
		seen := make(map[string]bool)
		var out []inProgressBead
		for _, ep := range epics {
			kids, err := cache.GetChildren(ep.ID)
			if err != nil {
				continue
			}
			for _, k := range kids {
				if strings.EqualFold(strings.TrimSpace(k.Status), "in_progress") && !seen[k.ID] {
					seen[k.ID] = true
					out = append(out, inProgressBead{ID: k.ID, Title: k.Title})
				}
			}
		}
		return out, nil
	}
}

// --- Stale-agent-worktrees block (Spec 093 Req 14 bullet 3) ------------
//
// Spec 093 Req 14 criterion (verbatim): "Stale agent worktrees:
// `bead.WorktreeList()` filtered to `.worktrees/worktree-*` without a
// matching in-progress bead, plus dir-scan of `.claude/worktrees/agent-*`."
// A worktree is STALE when it is a bead worktree (path basename starts
// with worktree-) whose bead is NOT among the currently in-progress
// beads — i.e. left behind after a merge/abandon — OR an
// agent-scratch dir under .claude/worktrees/agent-*.

// StaleWorktreeEntry is one stale worktree: its display path plus the
// source that flagged it (so the render can hint at cleanup).
type StaleWorktreeEntry struct {
	// Path is the worktree directory path.
	Path string
	// Source is "worktree-list" (a bead.WorktreeList entry with no
	// matching in-progress bead) or "agent-scan" (a .claude/worktrees/
	// agent-* dir).
	Source string
}

// renderStaleWorktrees formats the stale-agent-worktrees block (Spec 093
// Req 14 bullet 3) from already-resolved entries. PURE: no fs/bd — the
// scan is done by gatherStaleWorktrees. Returns "" when none are stale.
func renderStaleWorktrees(entries []StaleWorktreeEntry) string {
	if len(entries) == 0 {
		return ""
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })

	var b strings.Builder
	b.WriteString("## Stale Agent Worktrees\n\n")
	b.WriteString("Bead worktrees with no matching in-progress bead, plus `.claude/worktrees/agent-*` scratch dirs — ")
	b.WriteString("candidates for cleanup (left behind after a merge/abandon).\n\n")

	for _, e := range entries {
		b.WriteString(fmt.Sprintf("- `%s` (%s)\n", e.Path, e.Source))
	}

	return b.String()
}

// worktreeLister returns the bd-known worktrees. Injected (defaults to
// bead.WorktreeList) so gatherStaleWorktrees is testable without bd.
type worktreeLister func() ([]bead.WorktreeListEntry, error)

// gatherStaleWorktrees implements the Req 14 bullet-3 criterion: scan
// bead.WorktreeList() for `worktree-*` entries (BeadWorktreePrefix) whose
// bead is not currently in-progress, then dir-scan `.claude/worktrees/
// agent-*` under each root. inProgressIDs is the set of currently
// in-progress bead IDs (from the same gather pass). roots are the scan
// roots (worktree + main). Returns nil when nothing is stale.
func gatherStaleWorktrees(list worktreeLister, inProgressIDs map[string]bool, roots ...string) []StaleWorktreeEntry {
	var entries []StaleWorktreeEntry
	seen := make(map[string]bool)

	// (a) bead.WorktreeList() filtered to worktree-* without a matching
	// in-progress bead.
	if list != nil {
		if wts, err := list(); err == nil {
			for _, wt := range wts {
				if wt.IsMain {
					continue
				}
				base := filepath.Base(wt.Path)
				// Bead worktrees are "worktree-<beadID>"; spec worktrees
				// ("worktree-spec-...") are NOT bead worktrees and are not
				// stale-agent candidates (Req 14 targets bead worktrees).
				if !strings.HasPrefix(base, workspace.BeadWorktreePrefix) ||
					strings.HasPrefix(base, workspace.SpecWorktreePrefix) {
					continue
				}
				beadID := strings.TrimPrefix(base, workspace.BeadWorktreePrefix)
				if inProgressIDs[beadID] {
					continue // has a live in-progress bead → not stale
				}
				if !seen[wt.Path] {
					seen[wt.Path] = true
					entries = append(entries, StaleWorktreeEntry{Path: wt.Path, Source: "worktree-list"})
				}
			}
		}
	}

	// (b) dir-scan of .claude/worktrees/agent-* under each root.
	for _, root := range roots {
		if root == "" {
			continue
		}
		matches, err := filepath.Glob(filepath.Join(root, ".claude", "worktrees", "agent-*"))
		if err != nil {
			continue
		}
		for _, m := range matches {
			info, err := os.Stat(m)
			if err != nil || !info.IsDir() {
				continue
			}
			if !seen[m] {
				seen[m] = true
				entries = append(entries, StaleWorktreeEntry{Path: m, Source: "agent-scan"})
			}
		}
	}

	return entries
}

// --- Composite Panel/Subagent State block (Spec 093 Reqs 14-15) --------

// buildPanelStateBlock is the IO entrypoint for the full Panel/Subagent
// State block (Spec 093 Reqs 14-15): it gathers the three sub-blocks
// (each via its own injected-resolver gatherer) and composes them. It is
// a package-level var so the Req-15 stub-guard test can install a
// call-counting fake and prove it is invoked exactly once when
// --panel-state is requested and NEVER when it is not (zero git/bd
// subprocess attributable to panel-state on a panel-less session).
var buildPanelStateBlock = func(cache *phase.Cache, mainRoot, activeWorktree, activeBead string) string {
	roots := panelScanRoots(mainRoot, activeWorktree)

	inProgress := gatherInProgressBeads(
		activeEpicInProgressLister(cache), gitBeadLastCommit, activeBead)
	inProgressIDs := make(map[string]bool, len(inProgress))
	for _, e := range inProgress {
		inProgressIDs[e.ID] = true
	}

	panels := gatherPanelState(liveBranchSHA, roots...)
	stale := gatherStaleWorktrees(bead.WorktreeList, inProgressIDs, roots...)

	return renderFullPanelState(inProgress, panels, stale)
}

// renderFullPanelState composes the three Req-14 sub-blocks under a
// single "Panel/Subagent State" heading (Spec 093 Req 15 AC L1100-1103):
// in-progress beads, open panel rounds, and stale agent worktrees. PURE:
// every sub-block is rendered from pre-resolved inputs. Returns "" when
// all three sub-blocks are empty (zero-cost / clean-state contract — the
// caller appends nothing).
func renderFullPanelState(inProgress []BeadStateEntry, panels []PanelStateEntry, stale []StaleWorktreeEntry) string {
	ip := renderInProgressBeads(inProgress)
	op := renderPanelState(panels)
	sw := renderStaleWorktrees(stale)
	if ip == "" && op == "" && sw == "" {
		return ""
	}

	var b strings.Builder
	b.WriteString("# Panel/Subagent State\n\n")
	b.WriteString("Recovery snapshot of in-flight panels and worktrees (post-compaction; Reqs 14-15). ")
	b.WriteString("This INFORMS; the `mindspec complete` gate ENFORCES.\n")

	for _, sub := range []string{ip, op, sw} {
		if sub != "" {
			b.WriteString("\n")
			b.WriteString(sub)
		}
	}

	return b.String()
}
