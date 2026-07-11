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
// reusing this scan) and an error (non-nil only on a Block, INCLUDING the
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
// survive), and writes the merged array. Each entry is SELF-CONTAINED
// (round-3 redesign): it carries the applied refutation's full
// slot/round/reason/evidence content, so the pre-close reconciliation can
// settle it as an honest `panel_refuted` audit from the marker ALONE,
// without ever re-reading a mutable panel directory. If EITHER the read or
// the write fails, the refutation is NOT applied: this function returns a
// Block (a genuine guard.NewFailure, not an abort-with-applied), so the RC
// stays unresolved.
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
	var applied []panel.Refutation
	appliedSeen := make(map[string]bool) // dedup by (slot, round) ACROSS matched panels (item 3/step 4).
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
			return matched, guard.NewFailure(d.Message, fmt.Sprintf(
				"re-run the panel (/ms-panel-run step 0 for %s), then `mindspec complete %s`",
				beadID, beadID))
		case panel.Warn:
			if firstWarn == "" {
				firstWarn = d.Message
			}
		case panel.Allow:
			for _, ref := range d.AppliedRefutations {
				key := pendingEntryKey(ref.Slot, ref.Round)
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
			return matched, guard.NewFailure(fmt.Sprintf(
				"the refutation could not be durably recorded, so the REQUEST_CHANGES from %s remains unresolved (%v) — retry, or resolve the finding",
				strings.Join(slots, ", "), err),
				fmt.Sprintf("mindspec complete %s", beadID))
		}
	}

	if firstWarn != "" && warnOut != nil {
		fmt.Fprintf(warnOut, "panel gate: %s\n", firstWarn)
	}
	return matched, nil
}

// --- Durable-obligation protocol (Spec 114 R2 / Bead 2) ---------------------
//
// A refutation is "applied" (clears its RC) ONLY once its obligation is
// durably on bead metadata, and the FULL set of recorded obligations is
// reconciled — flushed to the panel_refuted audit from the marker itself,
// already covered of-record, or refused — before EVERY close (panel-present,
// no-panel, AND hatch). All reads/writes below go through the (fail-closed)
// completeGetMetadataFn / completeMergeMetadataFn seams.

// refutationPendingEntry is one element of the durable `refutation_pending_entries`
// bead-metadata array: the obligation a refutation created, recorded at
// apply time. It is SELF-CONTAINED (round-3 redesign): it carries the FULL
// refutation content — slot, round, reason, evidence — copied from the
// panel.json refutations entry whose application it records, so the
// pre-close reconciliation can settle the obligation as an honest
// `panel_refuted` audit from this marker ALONE, never re-reading a mutable
// panel directory. (The former origin-panel field, and the re-tally-based
// "verified discharge" it fed, are deliberately GONE: any discharge that
// trusts on-disk panel files has an irreducible spoofing surface — three
// review rounds each found a distinct false-discharge variant — so the
// obligation is now always settled as the truthful panel_refuted record
// instead.)
type refutationPendingEntry struct {
	Slot     string `json:"slot"`
	Round    int    `json:"round"`
	Reason   string `json:"reason,omitempty"`
	Evidence string `json:"evidence,omitempty"`
}

// pendingEntryKey is the (slot, round) dedup identity shared by every
// union helper below.
func pendingEntryKey(slot string, round int) string {
	return slot + "\x00" + strconv.Itoa(round)
}

// decodePendingEntries decodes the raw `refutation_pending_entries`
// metadata value (nil, a []interface{} of maps from a real bd JSON
// round-trip, or an already-typed slice from a test double) via a
// marshal/unmarshal round-trip. A genuinely ABSENT value (nil raw) decodes
// to (nil, nil) — no obligation, a no-op for every caller. A
// PRESENT-but-malformed value (a marshal, unmarshal, or shape error)
// returns an ERROR, never a silently-empty set: this array is the durable
// obligation store, and treating a corrupt store as empty would DROP a
// recorded obligation — the fail-OPEN direction this protocol exists to
// close. Callers Refuse/Block on the error (fail-closed, symmetric with
// the step-5(d) GetMetadata read-error rule).
func decodePendingEntries(raw interface{}) ([]refutationPendingEntry, error) {
	if raw == nil {
		return nil, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("re-encoding refutation_pending_entries: %w", err)
	}
	var out []refutationPendingEntry
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("decoding refutation_pending_entries: %w", err)
	}
	return out, nil
}

// decodeRefutations decodes a raw `panel_refuted_entries` value into
// []panel.Refutation — same absent-is-empty / present-but-corrupt-is-error
// discipline as decodePendingEntries (a corrupt satisfying-audit array must
// Refuse, never read as "nothing covered").
func decodeRefutations(raw interface{}) ([]panel.Refutation, error) {
	if raw == nil {
		return nil, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("re-encoding panel_refuted_entries: %w", err)
	}
	var out []panel.Refutation
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("decoding panel_refuted_entries: %w", err)
	}
	return out, nil
}

// unionPendingEntries returns the deterministic (slot, round)-deduplicated
// union of existing pending entries and the current run's newly recorded
// ones (an obligation is never clobbered). A key present in BOTH keeps the
// existing entry, back-filling an empty Reason/Evidence from the added one —
// so a content-less marker written by an older binary upgrades in place the
// next time the same refutation applies. The result is sorted for
// determinism.
func unionPendingEntries(existing, add []refutationPendingEntry) []refutationPendingEntry {
	idx := make(map[string]int)
	var out []refutationPendingEntry
	merge := func(e refutationPendingEntry) {
		key := pendingEntryKey(e.Slot, e.Round)
		if i, ok := idx[key]; ok {
			if out[i].Reason == "" {
				out[i].Reason = e.Reason
			}
			if out[i].Evidence == "" {
				out[i].Evidence = e.Evidence
			}
			return
		}
		idx[key] = len(out)
		out = append(out, e)
	}
	for _, e := range existing {
		merge(e)
	}
	for _, a := range add {
		merge(a)
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

// persistRefutationPending is the DURABLE-OBLIGATION marker write (Spec 114
// R2 step 5a): read-then-UNION-then-write the `refutation_pending_entries`
// array, fail-closed on either the read or the write. Called from panelGate
// ONLY when this run's decision(s) carried a non-empty AppliedRefutations
// set. Each persisted entry copies the applied refutation's FULL
// slot/round/reason/evidence content (round-3 redesign), making the marker
// self-contained for the pre-close reconciliation.
func persistRefutationPending(beadID string, applied []panel.Refutation) error {
	existingMeta, err := completeGetMetadataFn(beadID)
	if err != nil {
		return fmt.Errorf("reading existing refutation obligations: %w", err)
	}
	existing, decErr := decodePendingEntries(existingMeta["refutation_pending_entries"])
	if decErr != nil {
		// A present-but-corrupt obligation store cannot be safely unioned
		// into — fail-closed, the refutation is NOT applied (panelGate
		// Blocks), exactly like a read error above.
		return fmt.Errorf("reading existing refutation obligations: %w", decErr)
	}
	add := make([]refutationPendingEntry, 0, len(applied))
	for _, a := range applied {
		add = append(add, refutationPendingEntry{
			Slot:     a.Slot,
			Round:    a.Round,
			Reason:   a.Reason,
			Evidence: a.Evidence,
		})
	}
	merged := unionPendingEntries(existing, add)
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
	existing, decErr := decodeRefutations(existingMeta["panel_refuted_entries"])
	if decErr != nil {
		// Fail-closed: unioning into a corrupt audit array could lose an
		// earlier satisfied entry — the completion fails pre-close instead.
		return fmt.Errorf("reading existing panel_refuted_entries: %w", decErr)
	}
	merged := unionRefutations(existing, entries)
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

// --- Spec 115 Bead 1: the shared uncovered-obligation core -----------------
//
// uncoveredPendingEntry pairs a still-outstanding refutation_pending_entries
// record with its full content: a SETTLING caller (reconcilePendingRefutations)
// needs the reason/evidence to write a truthful panel_refuted audit, while a
// CHECK-ONLY caller (CheckPendingObligations) only needs the (slot, round)
// identity to name in its refusal. Carrying both costs nothing and keeps the
// two callers thin wrappers over one decode+coverage core.
type uncoveredPendingEntry struct {
	Slot     string
	Round    int
	Reason   string
	Evidence string
}

// uncoveredPendingObligations is the SHARED core both CheckPendingObligations
// (the exported check-only predicate) and reconcilePendingRefutations (the
// settle path) are thin wrappers over — extracted from what was previously
// reconcilePendingRefutations' own inline decode+coverage logic, with the
// settle step (writePanelRefutedMetadata) left OUT of this function. It reads
// beadID's metadata via the caller-supplied getMeta (dependency-injected so
// internal/approve can pass its own bead.GetMetadata seam without reaching
// into internal/complete's completeGetMetadataFn) and returns every pending
// entry NOT (slot, round)-exactly covered by a durable panel_refuted_entries
// record.
//
// Fail-closed throughout, matching Spec 114 R2's discipline exactly: a
// metadata read error, a present-but-corrupt refutation_pending_entries or
// panel_refuted_entries value (INCLUDING a present key whose JSON value is
// null — the R8 finding: plain map indexing cannot distinguish that from a
// genuinely absent key, so both fields are read via the comma-ok idiom and a
// present-but-null value errors explicitly), or a shape-invalid entry (empty
// slot / round < 1) all return a non-nil error — NEVER decode-as-empty,
// which would drop a recorded obligation (the fail-OPEN direction this
// protocol exists to close). A bead with no recorded pending entries
// returns (nil, nil) — a genuine no-op, §6 fail-open preserved for pristine
// beads.
func uncoveredPendingObligations(beadID string, getMeta func(string) (map[string]interface{}, error)) ([]uncoveredPendingEntry, error) {
	meta, err := getMeta(beadID)
	if err != nil {
		return nil, fmt.Errorf(
			"bead %s metadata could not be read to verify its refutation obligations are satisfied — an unreadable metadata store cannot prove the bead is obligation-free (%v)",
			beadID, err)
	}
	// Distinguish an ABSENT refutation_pending_entries key (the genuine
	// no-obligation no-op below) from a PRESENT key whose JSON value is
	// null: map indexing alone (meta["refutation_pending_entries"]) returns
	// nil for BOTH, and decodePendingEntries(nil) reads a present-null the
	// same as absent — a fail-OPEN hole (Spec 115 Bead 1 R8 finding). A
	// present-but-null value IS a present-but-corrupt value under the R3
	// fail-closed contract, so it must error, never decode-as-empty.
	rawPending, presentPending := meta["refutation_pending_entries"]
	if presentPending && rawPending == nil {
		return nil, fmt.Errorf(
			"bead %s carries a present-but-null refutation_pending_entries value — a present-but-corrupt obligation store cannot prove the bead is obligation-free",
			beadID)
	}
	pending, decErr := decodePendingEntries(rawPending)
	if decErr != nil {
		// PRESENT-but-malformed obligation store: fail-closed. Decoding it
		// as empty would silently drop every recorded obligation (fail-OPEN);
		// an ABSENT key (raw nil) is the genuine no-obligation no-op below.
		return nil, fmt.Errorf(
			"bead %s carries a refutation_pending_entries record that could not be decoded — a corrupt obligation store cannot prove the bead is obligation-free (%v)",
			beadID, decErr)
	}
	if len(pending) == 0 {
		return nil, nil
	}

	// Settle only the pending entries NOT already covered by a durable
	// panel_refuted audit — those obligations are met of-record. The
	// covering read is fail-closed too: a present-but-corrupt audit array
	// errors rather than reading as "nothing covered". Same present-null vs
	// absent distinction as above, for contract symmetry.
	rawRefuted, presentRefuted := meta["panel_refuted_entries"]
	if presentRefuted && rawRefuted == nil {
		return nil, fmt.Errorf(
			"bead %s carries a present-but-null panel_refuted_entries value — a present-but-corrupt audit store cannot prove which obligations are already satisfied",
			beadID)
	}
	coveredRefuted, decErr := decodeRefutations(rawRefuted)
	if decErr != nil {
		return nil, fmt.Errorf(
			"bead %s carries a panel_refuted_entries record that could not be decoded — a corrupt audit store cannot prove which obligations are already satisfied (%v)",
			beadID, decErr)
	}
	covered := make(map[string]bool, len(coveredRefuted))
	for _, c := range coveredRefuted {
		covered[pendingEntryKey(c.Slot, c.Round)] = true
	}

	var out []uncoveredPendingEntry
	for _, p := range pending {
		if p.Slot == "" || p.Round < 1 {
			// A shape-invalid marker cannot be settled (or checked) as a
			// truthful entry — error (fail-closed), symmetric with the
			// decode error above.
			return nil, fmt.Errorf(
				"bead %s carries a malformed refutation_pending entry (slot %q, round %d) — a shape-invalid obligation store cannot prove the bead is obligation-free",
				beadID, p.Slot, p.Round)
		}
		if covered[pendingEntryKey(p.Slot, p.Round)] {
			// Already covered by a durable audit — a no-op, not re-flagged.
			continue
		}
		out = append(out, uncoveredPendingEntry{Slot: p.Slot, Round: p.Round, Reason: p.Reason, Evidence: p.Evidence})
	}
	return out, nil
}

// CheckPendingObligations is the exported CHECK-ONLY obligation predicate
// (Spec 115 R3's single-home reuse): a dependency-injected reader over the
// SAME uncoveredPendingObligations core reconcilePendingRefutations uses, so
// internal/approve's impl-approve gate can check a bead's durable
// refutation_pending_entries obligations without importing
// internal/complete's completeGetMetadataFn seam or reaching into any
// complete-package internals. It NEVER settles anything — no metadata write,
// no panel read, no origin matching — unlike reconcilePendingRefutations
// (below), which consumes the identical core to SETTLE.
//
// Returns nil when the bead has no recorded pending entries, or every entry
// is (slot, round)-exactly covered by a durable panel_refuted_entries
// record. Returns a non-nil error naming the first uncovered slot@round
// otherwise, and a non-nil error (never decode-as-empty) on a metadata read
// error, a present-but-corrupt entries value, or a shape-invalid entry
// (empty slot / round < 1) — the identical fail-closed discipline
// reconcilePendingRefutations already enforces, single-sourced here.
func CheckPendingObligations(beadID string, getMeta func(string) (map[string]interface{}, error)) error {
	uncovered, err := uncoveredPendingObligations(beadID, getMeta)
	if err != nil {
		return err
	}
	if len(uncovered) == 0 {
		return nil
	}
	e := uncovered[0]
	return fmt.Errorf(
		"bead %s carries an unresolved refutation_pending obligation (%s@round %d) not yet covered by a durable panel_refuted record",
		beadID, e.Slot, e.Round)
}

// PanelGateRoots is the exported thin wrapper over panelGateRoots (Spec 115
// R2's single-home reuse): it returns the SAME layout-aware scan roots the
// authoritative panel gate itself uses, so a consumer decorating a refusal
// with the panel's unresolved slots (the impl-approve gate's advisory slot
// naming) reads the identical registered panel the gate would — never a
// second, potentially-diverging root order. No behavior change; every
// existing caller of panelGateRoots (this file) is untouched.
func PanelGateRoots(root, wtPath, specID string) []string {
	return panelGateRoots(root, wtPath, specID)
}

// reconcilePendingRefutations enforces Spec 114 R2's durable-obligation
// invariant: every `refutation_pending` entry recorded on bead metadata —
// the FULL unioned set across every prior run, not just this run's — is
// settled BEFORE close, on EVERY completion path (panel-present, no-panel,
// AND hatch — the hatches except the GATE, never the obligation). Called
// from complete.Run AFTER the last blocking gate (ADR-divergence) and
// BEFORE the step-4 close.
//
// Settlement is SATISFY-FROM-MARKER (round-3 redesign): every pending entry
// not already covered by a durable `panel_refuted_entries` record — the
// already-covered no-op is (slot, round)-exact, so a compound-failure retry
// whose audit landed but whose close failed never re-Refuses or re-writes
// (plan L533-535) — is flushed to the `panel_refuted` audit (slot, round,
// reason, evidence, timestamp) FROM THE MARKER ITSELF. No re-tally, no
// panel read, no origin matching: reconciliation never opens a panel
// directory, so no arrangement of removed/corrupted/aliased panel files can
// influence how an obligation settles (the round-1/2/3 false-discharge
// variants are unrepresentable by construction), and the same code path
// serves the panel-present, panel-removed, and hatch cases identically.
// The marker records that a refutation WAS durably applied, so the
// panel_refuted audit it produces is truthful even when panel.json is long
// gone. (The former re-tally-based "verified discharge" and its separate
// audit key are deleted; a finding that resolves naturally WITHOUT a
// refutation ever applying never creates a marker, so it never reaches this
// function — AC4b holds by construction.)
//
// Fail-closed: a completeGetMetadataFn read error, a present-but-corrupt
// entries value, and a shape-invalid entry (empty slot / round < 1) all
// Refuse — never decode-as-empty, which would DROP a recorded obligation
// (the fail-OPEN direction this protocol exists to close) — and a
// panel_refuted write failure fails completion pre-close (the bead stays
// in_progress, never merged with an unsettled obligation). A bead with NO
// recorded pending reconciles to a no-op — §6 fail-open preserved for
// genuinely pristine beads.
func reconcilePendingRefutations(beadID string) error {
	refuse := func(msg string) error {
		return guard.NewFailure(msg, fmt.Sprintf("mindspec complete %s", beadID))
	}

	// Spec 115 Bead 1: the decode + (slot, round)-coverage walk (Plan
	// L533-535) is now the SHARED uncoveredPendingObligations core, single-
	// homed with the exported check-only CheckPendingObligations predicate.
	// This function remains the only one that SETTLES the result.
	uncovered, err := uncoveredPendingObligations(beadID, completeGetMetadataFn)
	if err != nil {
		return refuse(err.Error())
	}
	if len(uncovered) == 0 {
		return nil
	}

	toSatisfy := make([]panel.Refutation, 0, len(uncovered))
	for _, u := range uncovered {
		toSatisfy = append(toSatisfy, panel.Refutation{
			Slot:     u.Slot,
			Round:    u.Round,
			Reason:   u.Reason,
			Evidence: u.Evidence,
		})
	}

	if err := writePanelRefutedMetadata(beadID, toSatisfy); err != nil {
		// Non-swallowing: an obligation may NEVER merge un-audited. The
		// completion fails pre-close and the bead stays in_progress.
		return fmt.Errorf("recording panel_refuted for %s: %w", beadID, err)
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
