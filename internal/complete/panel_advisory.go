package complete

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// defaultPanelSkipEnv reports whether the env-only panel-skip hatch is set
// for this process. Single-sourced on panel.SkipPanelEnv (Spec 099) so the
// audit write and the gate read the same variable name.
func defaultPanelSkipEnv() bool {
	return os.Getenv(panel.SkipPanelEnv) == "1"
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

// Panel-gate I/O seams (Spec 099 Bead 2). The AUTHORITATIVE in-binary gate
// (panelGate below) injects these into panel.ResolveGateFacts — the SAME
// rev-parse / porcelain / ref-not-found wiring the hook injects via its own
// seams — so the two call sites reach the IDENTICAL panel.PanelGateDecision
// over IDENTICAL panel.GateFacts (the anti-drift guarantee). Tests swap them
// to drive staleness / dirty-tree facts without a real repo.
var (
	gateRevParseFn = gitutil.RevParseRef
	gateStatusFn   = gitutil.Status
)

// panelGate is the AUTHORITATIVE in-binary panel gate (Spec 099 Bead 2,
// R1+R5; ADR-0037). It runs in complete.Run at the step-2.25 site — BEFORE
// exec.CommitAll, bd close, and the bead→spec merge — over the DECLARED
// beadID (no shell parsing; ADR-0036 Zero Framework Cognition). It invokes
// the SAME extracted panel.PanelGateDecision over panel.GateFacts produced by
// the SAME panel.ResolveGateFacts the PreToolUse hook uses (the hook is now a
// defense-in-depth backstop), so the two cannot disagree by construction.
//
// Ordering is load-bearing: the gate measures the bead/<id> tip as it stands
// BEFORE CommitAll. CommitAll would advance the tip past reviewed_head_sha
// (false-firing the §4 staleness clause) and clear user dirt (false-clearing
// the §5 dirty-tree clause), so the gate must run first (RED-on-revert if
// moved after CommitAll).
//
// Hatches (§7): the env-only skip (panel.SkipPanelEnv) and the
// enforcement.panel_gate config toggle short-circuit to a silent pass; the
// skip variable is NEVER named in any Block message (HC-7). Fail-open (§6):
// no panel.json registering the bead → no registration → pass silently, so
// the bead ACTUALLY completes (R2 dogfooding safety).
//
// On a Block from any matched panel it returns a guard.NewFailure whose body
// is the decision message (which already carries the raw-`git merge` fence,
// R5) and whose FINAL line is a genuine recovery command (re-panel +
// re-complete) — so the error passes guard.HasFinalRecoveryLine (ADR-0035)
// while keeping the fence in the body. The caller returns it BEFORE any
// mutation, exiting non-zero having mutated nothing (HC-4). A Warn (audited
// abandonment / missing-ref / transient git error) is printed to warnOut and
// the gate proceeds, parity with the hook's Warn path.
//
// The staleness HEAD source is the bead/<id> ref that panel.ResolveGateFacts
// rev-parses internally (in the panel's scanRoot) — the IDENTICAL source the
// hook uses. complete.Run resolves beadHead at step 2 for the per-bead
// doc-sync / adr gates; the panel gate does NOT re-derive it, it leans on the
// shared fact-gatherer's bead/<id> rev-parse so the two call sites cannot
// diverge.
//
// It returns the matched registration (for the post-completion audit writes,
// reusing this scan) and an error (non-nil only on a Block).
func panelGate(beadID string, roots []string, wtPath string, panelGateEnabled bool, warnOut io.Writer) (*panel.Registration, error) {
	if beadID == "" {
		return nil, nil
	}

	// (0) escape hatch — env-only, audited. The decision's Warn path keeps
	// the hatch-name out of any Block; passing SkipEnv true here also means a
	// skipped gate never blocks.
	skipEnv := panelSkipEnvFn()

	// (1) config toggle (§7): enforcement.panel_gate: false → skip the gate.
	// We still scan so the matched registration flows to the audit writes.
	regs := panel.ForBead(panelScanFn(roots...), beadID)
	if len(regs) == 0 {
		// Fail-open (§6): no registered panel → no gate, the bead completes.
		return nil, nil
	}

	var matched *panel.Registration
	var firstWarn string
	for i := range regs {
		if matched == nil {
			r := regs[i]
			matched = &r
		}
		// Honor the hatches with parity to the hook: a set skip env or a
		// disabled gate yields a Warn/Allow decision that never blocks. We
		// pass these through panel.GateFacts so the decision (not this
		// caller) owns the messaging, and the skip variable is never named.
		if skipEnv {
			d := panel.PanelGateDecision(panel.GateFacts{BeadID: beadID, SkipEnv: true})
			if d.Action == panel.Warn && firstWarn == "" {
				firstWarn = d.Message
			}
			continue
		}
		if !panelGateEnabled {
			// Config-disabled gate: do not evaluate facts, do not block.
			continue
		}

		scanRoot := panel.PanelDirScanRoot(regs[i].Dir)
		facts := panel.ResolveGateFacts(regs[i], beadID, scanRoot, panel.GateIO{
			RevParse:      gateRevParseFn,
			Status:        gateStatusFn,
			IsRefNotFound: func(e error) bool { return errors.Is(e, gitutil.ErrRefNotFound) },
			// Lazy worktree resolver: only invoked on the dirty-check path so
			// the abandoned / mismatch / missing-ref / transient-gitErr short
			// circuits pay no extra cost. complete.Run already resolved the
			// bead worktree (wtPath); "" means absent → dirty check skipped.
			Worktree: func() string { return wtPath },
		})
		d := panel.PanelGateDecision(facts)
		switch d.Action {
		case panel.Block:
			// A Block from any matched panel wins (R5). The decision.Message
			// already ends with the raw-`git merge` fence in its BODY; append
			// a GENUINE recovery line (re-panel + re-complete) via the guard
			// arg so the message passes guard.HasFinalRecoveryLine (ADR-0035)
			// with the fence still in the body BEFORE the recovery line. A
			// zero-command guard.NewFailure would PANIC — always pass one.
			return matched, guard.NewFailure(d.Message, fmt.Sprintf(
				"re-run the panel (/ms-panel-run step 0 for %s), then `mindspec complete %s`",
				beadID, beadID))
		case panel.Warn:
			if firstWarn == "" {
				firstWarn = d.Message
			}
		}
	}

	if firstWarn != "" && warnOut != nil {
		fmt.Fprintf(warnOut, "panel gate: %s\n", firstWarn)
	}
	return matched, nil
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
