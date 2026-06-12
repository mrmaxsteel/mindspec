package complete

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/hook"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// defaultPanelSkipEnv reports whether the env-only panel-skip hatch is set
// for this process. Single-sourced on hook.SkipPanelEnv so the audit write
// and the gate read the same variable name.
func defaultPanelSkipEnv() bool {
	return os.Getenv(hook.SkipPanelEnv) == "1"
}

// Panel-gate seams for the complete-side advisory (Spec 093 Req 13d) and
// the audit writes (Reqs 13b/13e). panelScanFn is swapped in tests to
// inject a fabricated panel without a real review/ tree; panelTallyFn lets
// tests drive the tally shape directly. panelAdvisoryOut is the writer the
// advisory prints to (stderr in production — advisory, not gating).
var (
	panelScanFn      = panel.Scan
	panelTallyFn     = panel.Tally
	panelAdvisoryOut io.Writer
	// panelSkipEnvFn reports whether the env-only skip hatch was set for
	// this process (Req 13b audit). Defaults to the real os.Getenv check.
	panelSkipEnvFn = defaultPanelSkipEnv
)

// panelAdvisory prints the warning-only tally for any registered panel that
// references beadID (Spec 093 Req 13d). It is the ONLY panel signal for
// flows that never route through Claude Code hooks (codex sessions,
// raw-shell agents, externally-orchestrated panels). No registered panel →
// no output and no added subprocess cost (panel.Scan is fs-only and returns
// nothing when no panel.json exists). Hard enforcement stays at the hook
// layer alone (HC-4).
//
// It returns the matched registration so the caller can drive the post-
// completion audit writes (panel_gate_skipped / panel_abandoned) off the
// same scan, avoiding a second fs walk.
func panelAdvisory(beadID string, roots []string, w io.Writer) *panel.Registration {
	if beadID == "" {
		return nil
	}
	regs := panel.ForBead(panelScanFn(roots...), beadID)
	if len(regs) == 0 {
		return nil
	}
	reg := regs[0]
	if w == nil {
		return &reg
	}
	res, err := panelTallyFn(reg.Dir)
	if err != nil {
		fmt.Fprintf(w, "panel advisory: %s registered but its directory is unreadable: %v\n", reg.Slug(), err)
		return &reg
	}
	verdict, summary := res.VoteDecision()
	var label string
	switch verdict {
	case panel.VotePass:
		label = "would PASS (vote only — the hook also checks staleness + dirty tree)"
	case panel.VoteAbandoned:
		label = "abandoned (gate passes with a warning)"
	default:
		label = "would BLOCK"
	}
	fmt.Fprintf(w, "panel advisory: %s %s — gate %s\n", reg.Slug(), summary, label)
	return &reg
}

// dedupeRoots returns the non-empty, distinct roots in order — the scan-root
// set for the complete-side advisory (the bead worktree and the repo root).
func dedupeRoots(roots ...string) []string {
	var out []string
	for _, r := range roots {
		if r == "" {
			continue
		}
		dup := false
		for _, o := range out {
			if o == r {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, r)
		}
	}
	return out
}

// writePanelAuditMetadata records the post-completion panel audit entries on
// bead metadata via MergeMetadata (Spec 093 Reqs 13b/13e), mirroring the
// doc-skew override discipline: written ONLY after the terminal mutation
// succeeds, best-effort (a write failure warns but does not fail the
// lifecycle). reg is the panel matched by panelAdvisory (nil → nothing to
// audit). It writes:
//
//   - panel_gate_skipped + _at  when MINDSPEC_SKIP_PANEL was set for a
//     bead that had a registered panel (the env skip is only meaningful
//     against a real gate);
//   - panel_abandoned + _at + _reason  when the matched panel.json carries
//     "abandoned": true.
func writePanelAuditMetadata(beadID string, reg *panel.Registration, w io.Writer) {
	if reg == nil {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)

	if panelSkipEnvFn() {
		meta := map[string]interface{}{
			"panel_gate_skipped":    true,
			"panel_gate_skipped_at": now,
		}
		if err := completeMergeMetadataFn(beadID, meta); err != nil && w != nil {
			fmt.Fprintf(w, "Warning: could not record panel_gate_skipped metadata on %s: %v\n", beadID, err)
		}
	}

	if reg.Err == nil && reg.Panel.Abandoned {
		meta := map[string]interface{}{
			"panel_abandoned":        true,
			"panel_abandoned_at":     now,
			"panel_abandoned_reason": reg.Panel.AbandonReason,
		}
		if err := completeMergeMetadataFn(beadID, meta); err != nil && w != nil {
			fmt.Fprintf(w, "Warning: could not record panel_abandoned metadata on %s: %v\n", beadID, err)
		}
	}
}
