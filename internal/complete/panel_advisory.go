package complete

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
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
// nothing when no panel.json exists). Hard enforcement lives in the
// authoritative in-binary gate (panelGate below) — the sole panel-gate
// enforcer now that the PreToolUse hook is retired (HC-4).
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
		label = "would PASS (vote only — the in-binary gate also checks staleness + dirty tree)"
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
//
// ADR-0030 boundary: internal/complete is an ENFORCEMENT package and must NOT
// import internal/gitutil directly. The seams therefore route the git I/O
// through the EXECUTOR (the git-I/O boundary, internal/executor) rather than
// gitutil. The default seams use a stateless MindspecExecutor — RevParseRef /
// Status / IsRefNotFound take their workdir as an argument and ignore the
// executor's Root, so a zero-value executor is a sufficient, thin, byte-
// identical pass-through to gitutil.RevParseRef / gitutil.Status /
// errors.Is(e, gitutil.ErrRefNotFound).
var (
	gateExecutor        executor.Executor = &executor.MindspecExecutor{}
	gateRevParseFn                        = gateExecutor.RevParseRef
	gateStatusFn                          = gateExecutor.Status
	gateIsRefNotFoundFn                   = gateExecutor.IsRefNotFound
)

// panelGate is the AUTHORITATIVE in-binary panel gate (Spec 099 Bead 2,
// R1+R5; ADR-0037). It runs in complete.Run at the step-2.25 site — BEFORE
// exec.CommitAll, bd close, and the bead→spec merge — over the DECLARED
// beadID (no shell parsing; ADR-0036 Zero Framework Cognition). It invokes
// the extracted panel.PanelGateDecision over panel.GateFacts produced by
// panel.ResolveGateFacts. With the PreToolUse hook retired, this in-binary
// gate is the sole authoritative panel-gate enforcement — it gathers the
// staleness + dirty-tree facts itself rather than leaning on any hook.
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
// reusing this scan), the SET of refutations this run's decision(s) applied
// — Spec 114 R2, deduplicated by (slot, round) across every matched panel,
// nil when none — and an error (non-nil only on a Block, INCLUDING the
// durable-obligation marker-write Block below).
//
// DURABLE-OBLIGATION protocol, part (a) (Spec 114 R2 step 5a): when the
// decision Allows carrying a non-empty AppliedRefutations set, the
// refutation is "applied" ONLY once its `refutation_pending` obligation is
// DURABLY persisted on bead metadata — folded into this function rather than
// left to a later best-effort write. It reads the existing
// `refutation_pending_entries` (fail-closed, via completeGetMetadataFn),
// UNIONS this run's applied (slot, round) entries into that set (never a
// bare replace — an older still-unsatisfied pending from a prior run must
// survive), and writes the merged array. If EITHER the read or the write
// fails, the refutation is NOT applied: this function returns a Block (a
// genuine guard.NewFailure, not an abort-with-applied) and reports NO
// applied refutations, so the RC stays unresolved.
func panelGate(beadID string, roots []string, wtPath string, panelGateEnabled bool, warnOut io.Writer) (*panel.Registration, []panel.Refutation, error) {
	if beadID == "" {
		return nil, nil, nil
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
		return nil, nil, nil
	}

	var matched *panel.Registration
	var firstWarn string
	var applied []panel.Refutation
	appliedSeen := make(map[string]bool) // dedup by "slot\x00round" ACROSS matched panels (item 3/step 4).
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
			IsRefNotFound: gateIsRefNotFoundFn,
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
			return matched, nil, guard.NewFailure(d.Message, fmt.Sprintf(
				"re-run the panel (/ms-panel-run step 0 for %s), then `mindspec complete %s`",
				beadID, beadID))
		case panel.Warn:
			if firstWarn == "" {
				firstWarn = d.Message
			}
		case panel.Allow:
			for _, ref := range d.AppliedRefutations {
				key := ref.Slot + "\x00" + strconv.Itoa(ref.Round)
				if appliedSeen[key] {
					continue
				}
				appliedSeen[key] = true
				applied = append(applied, ref)
			}
		}
	}

	if len(applied) > 0 {
		if err := persistRefutationPending(beadID, applied); err != nil {
			slots := make([]string, 0, len(applied))
			for _, a := range applied {
				slots = append(slots, a.Slot)
			}
			sort.Strings(slots)
			return matched, nil, guard.NewFailure(fmt.Sprintf(
				"the refutation could not be durably recorded, so the REQUEST_CHANGES from %s remains unresolved (%v) — retry, or resolve the finding",
				strings.Join(slots, ", "), err),
				fmt.Sprintf("mindspec complete %s", beadID))
		}
	}

	if firstWarn != "" && warnOut != nil {
		fmt.Fprintf(warnOut, "panel gate: %s\n", firstWarn)
	}
	return matched, applied, nil
}

// --- Durable-obligation protocol (Spec 114 R2 / Bead 2) ---------------------
//
// A refutation is "applied" (clears its RC) ONLY once its obligation is
// durably on bead metadata, and the FULL set of recorded obligations is
// reconciled — satisfied, verified-discharged, or refused — before EVERY
// close (panel-present, no-panel, AND hatch). All reads/writes below go
// through the (fail-closed) completeGetMetadataFn / completeMergeMetadataFn
// seams.

// refutationPendingEntry is one element of the durable `refutation_pending_entries`
// bead-metadata array: the (slot, round) obligation a refutation created.
// Deliberately carries no reason/evidence — those are re-derived from the
// live panel.json's Refutations at reconciliation time (or from the current
// run's own AppliedRefutations, which already carry them).
type refutationPendingEntry struct {
	Slot  string `json:"slot"`
	Round int    `json:"round"`
}

// dischargedEntry is one element of the `refutation_discharged_entries`
// bead-metadata array: a SYSTEM-VERIFIED fact (a re-tally proved the RC is
// no longer live), never an operator-authored refutation — so it carries a
// synthetic Reason and no Evidence field.
type dischargedEntry struct {
	Slot   string `json:"slot"`
	Round  int    `json:"round"`
	Reason string `json:"reason"`
}

// pendingEntryKey is the (slot, round) dedup identity shared by every
// union helper below.
func pendingEntryKey(slot string, round int) string {
	return slot + "\x00" + strconv.Itoa(round)
}

// decodePendingEntries tolerantly decodes the raw
// `refutation_pending_entries` metadata value (nil, a []interface{} of
// maps from a real bd JSON round-trip, or an already-typed slice from a
// test double) via a marshal/unmarshal round-trip. A malformed/unexpected
// shape decodes to nil (empty), never an error — GetMetadata already owns
// the fail-closed read-error signal; this only reshapes an already-clean
// read.
func decodePendingEntries(raw interface{}) []refutationPendingEntry {
	if raw == nil {
		return nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var out []refutationPendingEntry
	if json.Unmarshal(data, &out) != nil {
		return nil
	}
	return out
}

// decodeRefutations tolerantly decodes a raw `panel_refuted_entries` value
// into []panel.Refutation (same round-trip tolerance as
// decodePendingEntries).
func decodeRefutations(raw interface{}) []panel.Refutation {
	if raw == nil {
		return nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var out []panel.Refutation
	if json.Unmarshal(data, &out) != nil {
		return nil
	}
	return out
}

// unionPendingEntries returns the deterministic (slot, round)-deduplicated
// union of existing pending entries and the current run's newly applied
// refutations — existing entries win ties (never clobbered), then new ones
// are appended, then the whole set is sorted for determinism.
func unionPendingEntries(existing []refutationPendingEntry, add []panel.Refutation) []refutationPendingEntry {
	seen := make(map[string]bool)
	var out []refutationPendingEntry
	for _, e := range existing {
		key := pendingEntryKey(e.Slot, e.Round)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, e)
	}
	for _, a := range add {
		key := pendingEntryKey(a.Slot, a.Round)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, refutationPendingEntry{Slot: a.Slot, Round: a.Round})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Slot != out[j].Slot {
			return out[i].Slot < out[j].Slot
		}
		return out[i].Round < out[j].Round
	})
	return out
}

// unionRefutations returns the (slot, round)-deduplicated union of existing
// panel_refuted_entries and the newly satisfied entries — same dedup/sort
// discipline as unionPendingEntries.
func unionRefutations(existing, add []panel.Refutation) []panel.Refutation {
	seen := make(map[string]bool)
	var out []panel.Refutation
	for _, e := range existing {
		key := pendingEntryKey(e.Slot, e.Round)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, e)
	}
	for _, a := range add {
		key := pendingEntryKey(a.Slot, a.Round)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Slot != out[j].Slot {
			return out[i].Slot < out[j].Slot
		}
		return out[i].Round < out[j].Round
	})
	return out
}

// decodeDischargedEntries mirrors decodePendingEntries for
// `refutation_discharged_entries`.
func decodeDischargedEntries(raw interface{}) []dischargedEntry {
	if raw == nil {
		return nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var out []dischargedEntry
	if json.Unmarshal(data, &out) != nil {
		return nil
	}
	return out
}

// unionDischargedEntries is unionPendingEntries's twin for
// refutation_discharged_entries.
func unionDischargedEntries(existing, add []dischargedEntry) []dischargedEntry {
	seen := make(map[string]bool)
	var out []dischargedEntry
	for _, e := range existing {
		key := pendingEntryKey(e.Slot, e.Round)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, e)
	}
	for _, a := range add {
		key := pendingEntryKey(a.Slot, a.Round)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Slot != out[j].Slot {
			return out[i].Slot < out[j].Slot
		}
		return out[i].Round < out[j].Round
	})
	return out
}

// persistRefutationPending is the DURABLE-OBLIGATION marker write (Spec 114
// R2 step 5a): read-then-UNION-then-write the `refutation_pending_entries`
// array, fail-closed on either the read or the write. Called from panelGate
// ONLY when this run's decision(s) carried a non-empty AppliedRefutations
// set.
func persistRefutationPending(beadID string, applied []panel.Refutation) error {
	existingMeta, err := completeGetMetadataFn(beadID)
	if err != nil {
		return fmt.Errorf("reading existing refutation obligations: %w", err)
	}
	merged := unionPendingEntries(decodePendingEntries(existingMeta["refutation_pending_entries"]), applied)
	if err := completeMergeMetadataFn(beadID, map[string]interface{}{
		"refutation_pending_entries": merged,
	}); err != nil {
		return fmt.Errorf("writing refutation_pending_entries: %w", err)
	}
	return nil
}

// writePanelRefutedMetadata records the panel_refuted satisfying audit
// (Spec 114 R2 step 5b) for entries whose obligation is covered by THIS
// run's AppliedRefutations — UNIONING into any existing
// `panel_refuted_entries` (read-then-union, fail-closed) so a later satisfy
// never clobbers an earlier one. Unlike writePanelAuditMetadata
// (best-effort), this RETURNS the merge error non-swallowing: it runs
// inside the pre-close reconciliation, and a failure there fails completion
// pre-close (AC11).
func writePanelRefutedMetadata(beadID string, entries []panel.Refutation) error {
	if len(entries) == 0 {
		return nil
	}
	existingMeta, err := completeGetMetadataFn(beadID)
	if err != nil {
		return fmt.Errorf("reading existing panel_refuted_entries: %w", err)
	}
	merged := unionRefutations(decodeRefutations(existingMeta["panel_refuted_entries"]), entries)
	meta := map[string]interface{}{
		"panel_refuted":         true,
		"panel_refuted_at":      time.Now().UTC().Format(time.RFC3339),
		"panel_refuted_entries": merged,
	}
	if err := completeMergeMetadataFn(beadID, meta); err != nil {
		return fmt.Errorf("writing panel_refuted metadata: %w", err)
	}
	return nil
}

// writeRefutationDischargedMetadata records the refutation_discharged
// VERIFIED-resolution audit (Spec 114 R2 step 5c) — UNIONING into any
// existing `refutation_discharged_entries` (read-then-union, fail-closed),
// non-swallowing (a failure fails completion pre-close, mirroring
// writePanelRefutedMetadata).
func writeRefutationDischargedMetadata(beadID string, entries []dischargedEntry) error {
	if len(entries) == 0 {
		return nil
	}
	existingMeta, err := completeGetMetadataFn(beadID)
	if err != nil {
		return fmt.Errorf("reading existing refutation_discharged_entries: %w", err)
	}
	merged := unionDischargedEntries(decodeDischargedEntries(existingMeta["refutation_discharged_entries"]), entries)
	meta := map[string]interface{}{
		"refutation_discharged":         true,
		"refutation_discharged_at":      time.Now().UTC().Format(time.RFC3339),
		"refutation_discharged_entries": merged,
	}
	if err := completeMergeMetadataFn(beadID, meta); err != nil {
		return fmt.Errorf("writing refutation_discharged metadata: %w", err)
	}
	return nil
}

// dischargeEvidence reports whether res's re-tally affirmatively shows slot
// is no longer a latest-round REQUEST_CHANGES at/after round (Spec 114 R2
// step 5c's two-disjunct verified-discharge test): (i) res.LatestRound >
// round (a later round supersedes it), OR (ii) at res.LatestRound == round,
// slot's latest verdict is present and is NOT REQUEST_CHANGES (the
// reviewer flipped). A Warn panel (abandoned / missing-ref / transient-
// gitErr) whose re-tally still shows the slot as a latest-round RC at the
// pending round does NOT meet this test.
func dischargeEvidence(res *panel.Result, slot string, round int) bool {
	if res == nil {
		return false
	}
	if res.LatestRound > round {
		return true
	}
	if res.LatestRound != round {
		return false
	}
	for _, v := range res.Verdicts {
		if v.Slot == slot {
			return v.Verdict != panel.VerdictRequestChanges
		}
	}
	return false
}

// reconcilePendingRefutations enforces Spec 114 R2's durable-obligation
// invariant: every `refutation_pending` entry recorded on bead metadata —
// the FULL unioned set across every prior run, not just this run's — must
// be Satisfied, verified-Discharged, or Refused BEFORE close, on EVERY
// completion path (panel-present, no-panel, AND hatch). Called from
// complete.Run AFTER the last blocking gate (ADR-divergence) and BEFORE the
// step-4 close.
//
// panelReg is the matched registration panelGate returned (nil on the
// no-panel / no-match paths — panelGate does NOT tally on the hatch paths,
// so this function does its OWN re-tally of panelReg.Dir for discharge
// evidence, uniformly across every path). applied is THIS run's
// AppliedRefutations set (panelGate's third return); a pending entry it
// covers Satisfies. Discharge fires ONLY on affirmative re-tally evidence
// (dischargeEvidence) — never on a bare Allow/Warn gate action, since Warn
// paths (abandoned/missing-ref/transient-gitErr) pass WITHOUT the RC
// resolving. No panel dir, an erroring re-tally, a re-tally that still
// shows the RC live, or a completeGetMetadataFn read error all Refuse
// (fail-closed, over-conservative: never a lost obligation, never a false
// discharge). A bead with NO recorded pending reconciles to a no-op — §6
// fail-open preserved for genuinely pristine beads.
func reconcilePendingRefutations(beadID string, panelReg *panel.Registration, applied []panel.Refutation) error {
	meta, err := completeGetMetadataFn(beadID)
	if err != nil {
		return guard.NewFailure(fmt.Sprintf(
			"bead %s metadata could not be read to verify its refutation obligations are satisfied — an unreadable metadata store cannot prove the bead is obligation-free (%v)",
			beadID, err),
			fmt.Sprintf("mindspec complete %s", beadID))
	}
	pending := decodePendingEntries(meta["refutation_pending_entries"])
	if len(pending) == 0 {
		return nil
	}

	appliedIdx := make(map[string]panel.Refutation, len(applied))
	for _, a := range applied {
		appliedIdx[pendingEntryKey(a.Slot, a.Round)] = a
	}

	// Re-tally the matched panel dir ONCE for discharge evidence (fs-only,
	// no git). No panel dir, or a re-tally error, leaves res nil — discharge
	// evidence is then UNAVAILABLE for every entry not covered by THIS run's
	// applied set, so those Refuse (fail-closed).
	var res *panel.Result
	if panelReg != nil {
		if r, tallyErr := panelTallyFn(panelReg.Dir); tallyErr == nil {
			res = r
		}
	}

	var toSatisfy []panel.Refutation
	var toDischarge []dischargedEntry
	var unresolved []string

	for _, p := range pending {
		if ref, ok := appliedIdx[pendingEntryKey(p.Slot, p.Round)]; ok {
			toSatisfy = append(toSatisfy, ref)
			continue
		}
		if dischargeEvidence(res, p.Slot, p.Round) {
			toDischarge = append(toDischarge, dischargedEntry{
				Slot: p.Slot, Round: p.Round,
				Reason: fmt.Sprintf("RC resolved naturally — the slot's latest-round verdict is no longer REQUEST_CHANGES at/after round %d", p.Round),
			})
			continue
		}
		unresolved = append(unresolved, fmt.Sprintf("%s@%d", p.Slot, p.Round))
	}

	if len(unresolved) > 0 {
		sort.Strings(unresolved)
		return guard.NewFailure(fmt.Sprintf(
			"this bead carries an unsatisfied refutation obligation for %s that cannot be verified as satisfied or resolved — restore the panel so it can be satisfied or discharged, or restore the audit",
			strings.Join(unresolved, ", ")),
			fmt.Sprintf("mindspec complete %s", beadID))
	}

	if err := writePanelRefutedMetadata(beadID, toSatisfy); err != nil {
		return fmt.Errorf("recording panel_refuted for %s: %w", beadID, err)
	}
	if err := writeRefutationDischargedMetadata(beadID, toDischarge); err != nil {
		return fmt.Errorf("recording refutation_discharged for %s: %w", beadID, err)
	}
	return nil
}

// reviewerCountAdvisory prints the caller-side panel.ReviewerCountNote
// advisory (spec 109 R8) for the panel matched by panelGate, IFF its
// recorded expected_reviewers differs from configDefault (the current
// panel.PanelExpectedReviewers() config default). It is advisory only:
// panelGate has ALREADY computed the Allow/Block decision by the time
// complete.Run calls this (immediately after the panelGate call, step
// 2.25) — this cannot alter that decision, it only appends a line to the
// SAME writer panelGate's own Warn messages use. reg is read-only here; a
// nil registration (no matched panel — the common case) or a malformed one
// (Err != nil, no ExpectedReviewers to compare) prints nothing.
func reviewerCountAdvisory(reg *panel.Registration, configDefault int, w io.Writer) {
	if reg == nil || reg.Err != nil || w == nil {
		return
	}
	if note := panel.ReviewerCountNote(reg.Panel.ExpectedReviewers, configDefault); note != "" {
		fmt.Fprintf(w, "panel advisory: %s\n", note)
	}
}

// panelGateRoots returns the directories the authoritative panel gate scans,
// chosen by the project's docs layout (Spec 106 Bead 4, AC13). panel.Scan globs
// BOTH the repo-root `review/` and the co-located `reviews/` segment under each
// root, so the RETURNED SET decides which conventions actually drive the gate:
//
//   - flat (post-flatten): the co-located <spec-dir>/reviews/ ONLY. The repo
//     root is omitted, so a leftover root review/<slug>/panel.json no longer
//     drives the gate once the tree is flat and Bead 5 has migrated root review/
//     away.
//   - canonical/legacy/greenfield (pre-flatten, incl. a transient mixed tree):
//     the bead-worktree + repo-root review/ convention UNION the co-located
//     <spec-dir>/reviews/ convention. BOTH drive the gate through the
//     transition — root review/ stays live until Bead 5 migrates it, so this
//     spec's own remaining beads, reviewed at repo-root review/<slug>, keep
//     gating complete.
//
// The layout is read once from the main repo root via workspace.DetectLayout;
// its mixed-tree error is non-fatal here — the returned kind still selects the
// safe transitional union (anything not flat → union).
func panelGateRoots(root, wtPath, specID string) []string {
	specDirs := specScopedReviewRoots(root, specID)
	if layout, _ := workspace.DetectLayout(root); layout == workspace.LayoutFlat {
		return dedupeRoots(specDirs...)
	}
	roots := make([]string, 0, len(specDirs)+2)
	roots = append(roots, wtPath, root)
	roots = append(roots, specDirs...)
	return dedupeRoots(roots...)
}

// specScopedReviewRoots resolves the spec directory whose co-located reviews/
// subdir (a sibling of workspace.RecordingDir) holds this spec's panels. Passed
// to panel.Scan as a root, it contributes the <spec-dir>/reviews/<slug> panels.
// Returns nil when specID is empty or not resolvable (the gate then scans only
// the repo-root/worktree review/ convention).
func specScopedReviewRoots(root, specID string) []string {
	if specID == "" {
		return nil
	}
	specDir, err := workspace.SpecDir(root, specID)
	if err != nil || specDir == "" {
		return nil
	}
	return []string{specDir}
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
