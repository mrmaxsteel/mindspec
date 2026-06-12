package instruct

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/panel"
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
	b.WriteString("This INFORMS; the pre-complete hook ENFORCES.\n")

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
