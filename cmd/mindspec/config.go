package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

// inertAnnotation marks a config block that is parsed, defaulted, validated,
// and surfaced here, but INERT: nothing in this binary reads it to change
// behavior yet (spec 109 R9). Only panel: and the pre-existing top-level
// keys drive in-binary behavior today.
const inertAnnotation = "declared, not yet enforced"

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Inspect the effective mindspec orchestration config",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the effective config (defaults merged with .mindspec/config.yaml)",
	Long: `Print the effective config — including the panel:, models:, loop:,
and runner: orchestration blocks (spec 109) alongside the pre-existing
keys — to stdout. Read-only: it writes no file and exits 0 on a valid
config. The models:, loop:, and runner: blocks are annotated "` + inertAnnotation + `"
because only panel: and the pre-existing keys drive in-binary behavior
in this release.

With --gate <name> (spec 112 R8/R9), prints that single gate's resolved
creation-time defaults instead — the expanded reviewer slots, expected
reviewer count, raw approve_threshold expression, and effective
substitution policy — as text, or as JSON with --json. --gate accepts one
of the five panel.gates keys (spec_approve, plan_approve, bead,
final_review, adhoc); --json requires --gate. Both forms stay read-only.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findRoot()
		if err != nil {
			return err
		}
		cfg, err := config.Load(root)
		if err != nil {
			return err
		}

		gate, _ := cmd.Flags().GetString("gate")
		asJSON, _ := cmd.Flags().GetBool("json")
		if gate == "" {
			if asJSON {
				return fmt.Errorf("--json requires --gate <name>: the resolved view is per-gate\nrecovery: pass --gate (one of %s) alongside --json", strings.Join(config.PanelGateKeys, ", "))
			}
			out, err := renderConfig(cfg)
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			fmt.Fprint(w, out)
			fmt.Fprint(w, reviewerCountNotesFor(cfg, root))
			return nil
		}

		if asJSON {
			data, err := gateResolvedJSON(cfg, gate)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		}
		out, err := renderGateResolved(cfg, gate)
		if err != nil {
			return err
		}
		fmt.Fprint(cmd.OutOrStdout(), out)
		return nil
	},
}

func init() {
	configShowCmd.Flags().String("gate", "", "print the resolved creation-time defaults for one panel gate (spec_approve|plan_approve|bead|final_review|adhoc)")
	configShowCmd.Flags().Bool("json", false, "with --gate, print the resolved view as JSON instead of text")
	configCmd.AddCommand(configShowCmd)
}

// escapeConfigValue renders a config-controlled string, s, safely for
// inclusion in `mindspec config show`'s YAML-like stdout (final-review G2).
// Every string in this render path — protected-branch names, source globs,
// reviewer family names, the raw approve_threshold expression, models: keys
// and values, loop.gate_authority keys and values, on_reject,
// controller_handoff, handoff_log, and runner — is read from
// .mindspec/config.yaml, a file a repo-write attacker can poison; without
// escaping, a value carrying ANSI/control bytes or an embedded newline
// reaches the terminal raw and can forge extra, attacker-chosen display
// lines.
//
// Safe-set rule: s renders UNCHANGED iff every rune in it is printable
// ASCII in [0x20, 0x7e] (space through `~`) — this covers letters, digits,
// and all ASCII punctuation, so every existing plain value (including the
// R9 AC's `approve_threshold: n-1`) is byte-for-byte identical to before.
// Anything else — C0/C1 control bytes (including ESC/BEL and \n/\r), DEL,
// or non-ASCII/invalid-UTF-8 runes — is rendered as a single-line,
// double-quoted Go string literal via strconv.Quote, which cannot itself
// contain a raw control byte or a literal newline, so a hostile value can
// never span or forge additional output lines.
func escapeConfigValue(s string) string {
	return termsafe.Escape(s)
}

// renderConfig renders the effective config, cfg, as human-readable
// YAML-like text (spec 109 R9). It is a PURE function over *config.Config —
// no fs, no panel scan — so `mindspec config show` is exercised without
// spawning a process. The caller-side panel.ReviewerCountNote scan (R8)
// lives in reviewerCountNotesFor, not here.
func renderConfig(cfg *config.Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("renderConfig: nil config")
	}

	var b strings.Builder

	fmt.Fprintln(&b, "# Effective mindspec config (defaults merged with .mindspec/config.yaml)")
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "protected_branches:")
	for _, br := range cfg.ProtectedBranches {
		fmt.Fprintf(&b, "  - %s\n", escapeConfigValue(br))
	}
	fmt.Fprintf(&b, "merge_strategy: %s\n", escapeConfigValue(cfg.MergeStrategy))
	fmt.Fprintf(&b, "worktree_root: %s\n", escapeConfigValue(cfg.WorktreeRoot))
	fmt.Fprintf(&b, "auto_finalize: %t\n", cfg.AutoFinalize)
	fmt.Fprintf(&b, "auto_open_finalize_pr: %t\n", cfg.AutoOpenFinalizePR)
	fmt.Fprintf(&b, "auto_merge_finalize_pr: %t\n", cfg.AutoMergeFinalizePR)
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "enforcement:")
	fmt.Fprintf(&b, "  pre_commit_hook: %t\n", cfg.Enforcement.PreCommitHook)
	fmt.Fprintf(&b, "  cli_guards: %t\n", cfg.Enforcement.CLIGuards)
	fmt.Fprintf(&b, "  agent_hooks: %t\n", cfg.Enforcement.AgentHooks)
	fmt.Fprintf(&b, "  panel_gate: %t\n", cfg.Enforcement.PanelGate)
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "recording:")
	fmt.Fprintf(&b, "  enabled: %t\n", cfg.Recording.Enabled)
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "decomposition:")
	fmt.Fprintf(&b, "  max_beads: %d\n", cfg.Decomposition.MaxBeads)
	fmt.Fprintf(&b, "  max_scope_overlap: %g\n", cfg.Decomposition.MaxScopeOverlap)
	fmt.Fprintf(&b, "  min_scope_overlap: %g\n", cfg.Decomposition.MinScopeOverlap)
	fmt.Fprintf(&b, "  max_chain_depth: %d\n", cfg.Decomposition.MaxChainDepth)
	fmt.Fprintf(&b, "  min_parallelism: %g\n", cfg.Decomposition.MinParallelism)
	fmt.Fprintln(&b)

	if len(cfg.SourceGlobs) == 0 {
		fmt.Fprintln(&b, "source_globs: []")
	} else {
		fmt.Fprintln(&b, "source_globs:")
		for _, g := range cfg.SourceGlobs {
			fmt.Fprintf(&b, "  - %s\n", escapeConfigValue(g))
		}
	}
	fmt.Fprintln(&b)

	// panel: drives in-binary behavior today (creation-time defaults for a
	// fresh panel.json, spec 109 R2) — NOT annotated inert.
	fmt.Fprintln(&b, "panel:")
	fmt.Fprintln(&b, "  reviewers:")
	for _, r := range cfg.Panel.Reviewers {
		fmt.Fprintf(&b, "    - family: %s\n", escapeConfigValue(r.Family))
		// CountValue(), not the raw *int Count field: spec 112 pointerized
		// Count so an absent count (nil, default 1) is distinguishable from
		// an explicit `count: 0`; %d on the pointer itself would compile
		// (go vet included) but print the address, not the count.
		fmt.Fprintf(&b, "      count: %d\n", r.CountValue())
	}
	// PanelApproveThresholdExpr is the RAW approve_threshold expression,
	// rendered verbatim (no trim/normalize) — its resolver contract is
	// "exactly as configured" (Bead 2/3 panel note); resolution to an int
	// stays single-homed in internal/panel.Panel.ApproveThreshold.
	// escapeConfigValue is a no-op for the plain "n-1"/integer expressions
	// validateOrchestration allows, so the R9 AC substring is unaffected.
	fmt.Fprintf(&b, "  approve_threshold: %s\n", escapeConfigValue(cfg.PanelApproveThresholdExpr()))
	fmt.Fprintln(&b, "  substitution:")
	fmt.Fprintf(&b, "    claude_sub_on_quota: %t\n", cfg.Panel.Substitution.ClaudeSubOnQuota)
	renderSubstitutes(&b, "    ", cfg.Panel.Substitution.Substitutes)
	if cfg.Panel.Note != "" {
		fmt.Fprintf(&b, "  note: %s\n", escapeConfigValue(cfg.Panel.Note))
	}
	if err := renderGates(&b, cfg); err != nil {
		return "", err
	}
	renderKnownModelWarnings(&b, cfg)
	fmt.Fprintln(&b)

	// models: free-form phase -> model-id map, INERT (spec 109 R3). Map
	// keys are sorted for deterministic output.
	if len(cfg.Models) == 0 {
		fmt.Fprintf(&b, "models: {}  # %s\n", inertAnnotation)
	} else {
		fmt.Fprintf(&b, "models:  # %s\n", inertAnnotation)
		phases := make([]string, 0, len(cfg.Models))
		for k := range cfg.Models {
			phases = append(phases, k)
		}
		sort.Strings(phases)
		for _, k := range phases {
			fmt.Fprintf(&b, "  %s: %s\n", escapeConfigValue(k), escapeConfigValue(cfg.Models[k]))
		}
	}
	fmt.Fprintln(&b)

	// loop: governance skeleton, INERT (spec 109 R4). GateAuthority is a
	// map — its keys are sorted for deterministic output (Bead 2/3 panel
	// note: an unsorted map range would make this command's output
	// nondeterministic).
	fmt.Fprintf(&b, "loop:  # %s\n", inertAnnotation)
	fmt.Fprintf(&b, "  enabled: %t\n", cfg.Loop.Enabled)
	fmt.Fprintln(&b, "  gate_authority:")
	gateKeys := make([]string, 0, len(cfg.Loop.GateAuthority))
	for k := range cfg.Loop.GateAuthority {
		gateKeys = append(gateKeys, k)
	}
	sort.Strings(gateKeys)
	for _, k := range gateKeys {
		fmt.Fprintf(&b, "    %s: %s\n", escapeConfigValue(k), escapeConfigValue(cfg.Loop.GateAuthority[k]))
	}
	fmt.Fprintln(&b, "  halt:")
	fmt.Fprintf(&b, "    max_rounds_per_bead: %d\n", cfg.Loop.Halt.MaxRoundsPerBead)
	fmt.Fprintf(&b, "    panel_deadlock_rounds: %d\n", cfg.Loop.Halt.PanelDeadlockRounds)
	fmt.Fprintf(&b, "    max_consecutive_impl_failures: %d\n", cfg.Loop.Halt.MaxConsecutiveImplFailures)
	fmt.Fprintf(&b, "    on_reject: %s\n", escapeConfigValue(cfg.Loop.Halt.OnReject))
	fmt.Fprintln(&b, "  budget:")
	fmt.Fprintf(&b, "    max_beads_per_wake: %d\n", cfg.Loop.Budget.MaxBeadsPerWake)
	fmt.Fprintf(&b, "    token_budget: %d\n", cfg.Loop.Budget.TokenBudget)
	fmt.Fprintln(&b, "  context:")
	fmt.Fprintf(&b, "    controller_handoff: %s\n", escapeConfigValue(cfg.Loop.Context.ControllerHandoff))
	fmt.Fprintf(&b, "  handoff_log: %s\n", escapeConfigValue(cfg.Loop.HandoffLog))
	fmt.Fprintln(&b)

	// runner: orchestration adapter selector, INERT (spec 109 R10) — no
	// adapter dispatch is wired in this release.
	fmt.Fprintf(&b, "runner: %s  # %s\n", escapeConfigValue(cfg.Runner), inertAnnotation)

	return b.String(), nil
}

// renderSubstitutes renders panel.substitution.substitutes (spec 112 R5) at
// indent, in sorted-key order — Go map iteration order must never leak into
// this command's output — followed by the slot-id-preservation convention
// line: a substituted reviewer writes its verdict under reviewer_id "<slot>
// <substitute-model>-sub", keeping the slot id, so verdicts stay comparable
// across rounds. The substitutes key is never omitted (R8): an empty map
// still renders "substitutes: {}".
func renderSubstitutes(b *strings.Builder, indent string, substitutes map[string]string) {
	if len(substitutes) == 0 {
		fmt.Fprintf(b, "%ssubstitutes: {}\n", indent)
	} else {
		fmt.Fprintf(b, "%ssubstitutes:\n", indent)
		keys := make([]string, 0, len(substitutes))
		for k := range substitutes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(b, "%s  %s: %s\n", indent, escapeConfigValue(k), escapeConfigValue(substitutes[k]))
		}
	}
	fmt.Fprintf(b, "%s# a substituted reviewer writes reviewer_id \"<slot> <substitute-model>-sub\", keeping the slot id\n", indent)
}

// renderReviewerEntry renders one AS-CONFIGURED reviewer entry (spec 112
// R8) — model/family/lens/count exactly as the operator set them, not the
// resolved chain — at the given indent. Load's validateReviewerEntries
// guarantees at least one of Model/Family is non-empty, so the leading
// "- " always gets a field on the same line.
func renderReviewerEntry(b *strings.Builder, indent string, r config.Reviewer) {
	fmt.Fprintf(b, "%s- ", indent)
	wroteFirst := false
	if r.Model != "" {
		fmt.Fprintf(b, "model: %s\n", escapeConfigValue(r.Model))
		wroteFirst = true
	}
	if r.Family != "" {
		if wroteFirst {
			fmt.Fprintf(b, "%s  family: %s\n", indent, escapeConfigValue(r.Family))
		} else {
			fmt.Fprintf(b, "family: %s\n", escapeConfigValue(r.Family))
			wroteFirst = true
		}
	}
	if r.Lens != "" {
		fmt.Fprintf(b, "%s  lens: %s\n", indent, escapeConfigValue(r.Lens))
	}
	fmt.Fprintf(b, "%s  count: %d\n", indent, r.CountValue())
}

// renderGates renders panel.gates (spec 112 R8) — only CONFIGURED gates, in
// config.PanelGateKeys enum declaration order (never map-iteration order) —
// each with its as-configured reviewer entries, its resolved reviewer sum
// (PanelGateExpectedReviewers), and its RAW threshold expression
// (PanelGateApproveThresholdExpr, never resolved to an int here). The gates
// key is never omitted (R8): no configured gates still renders "gates: {}".
func renderGates(b *strings.Builder, cfg *config.Config) error {
	if len(cfg.Panel.Gates) == 0 {
		fmt.Fprintln(b, "  gates: {}")
		return nil
	}
	fmt.Fprintln(b, "  gates:")
	for _, gate := range config.PanelGateKeys {
		gp, ok := cfg.Panel.Gates[gate]
		if !ok {
			continue
		}
		fmt.Fprintf(b, "    %s:\n", escapeConfigValue(gate))
		if len(gp.Reviewers) > 0 {
			fmt.Fprintln(b, "      reviewers:")
			for _, r := range gp.Reviewers {
				renderReviewerEntry(b, "        ", r)
			}
		}
		sum, err := cfg.PanelGateExpectedReviewers(gate)
		if err != nil {
			return err
		}
		expr, err := cfg.PanelGateApproveThresholdExpr(gate)
		if err != nil {
			return err
		}
		fmt.Fprintf(b, "      expected_reviewers: %d\n", sum)
		fmt.Fprintf(b, "      approve_threshold: %s\n", escapeConfigValue(expr))
	}
	return nil
}

// renderKnownModelWarnings appends one advisory comment per DISTINCT model
// id — drawn from the global reviewers, every configured gate's reviewers,
// and either side of substitutes — that is absent from config.KnownModels()
// (spec 112 R8). Advisory-only by construction: never consulted by
// validation, never affects the exit code (an unrecognized id still exits
// 0); ids are deduplicated and sorted for deterministic output.
func renderKnownModelWarnings(b *strings.Builder, cfg *config.Config) {
	known := make(map[string]bool, len(config.KnownModels()))
	for _, m := range config.KnownModels() {
		known[m] = true
	}
	seen := make(map[string]bool)
	var unknown []string
	add := func(id string) {
		if id == "" || known[id] || seen[id] {
			return
		}
		seen[id] = true
		unknown = append(unknown, id)
	}
	collect := func(reviewers []config.Reviewer) {
		for _, r := range reviewers {
			id := r.Model
			if id == "" {
				id = r.Family
			}
			add(id)
		}
	}
	collect(cfg.Panel.Reviewers)
	for _, gate := range config.PanelGateKeys {
		if gp, ok := cfg.Panel.Gates[gate]; ok {
			collect(gp.Reviewers)
		}
	}
	for k, v := range cfg.Panel.Substitution.Substitutes {
		add(k)
		add(v)
	}
	sort.Strings(unknown)
	for _, id := range unknown {
		fmt.Fprintf(b, "  # model %s not in the known-model list — fine if intentional\n", escapeConfigValue(id))
	}
}

// gateResolvedSlot is one member of gateResolvedDoc.Slots — the JSON/text
// shape of a config.ReviewerSlot (spec 112 R9).
type gateResolvedSlot struct {
	Slot  string `json:"slot"`
	Model string `json:"model"`
	Lens  string `json:"lens"`
}

// gateResolvedSubstitution is the "substitution" member of gateResolvedDoc
// (spec 112 R9): the effective substitution policy. Substitutes marshals
// through encoding/json, which emits map keys in sorted order — the same
// ordering renderSubstitutes uses for the text path — so both paths agree.
// InForce is "substitutes" when Substitutes is non-empty (R5: the map IS
// the policy), else "claude_sub_on_quota" (the legacy field keeps its 109
// meaning).
type gateResolvedSubstitution struct {
	Substitutes      map[string]string `json:"substitutes"`
	ClaudeSubOnQuota bool              `json:"claude_sub_on_quota"`
	InForce          string            `json:"in_force"`
}

// gateResolvedDoc is the R9 stable contract: config show --gate <name>
// --json's exact five members. Evolution is additive-only (documented in
// .mindspec/domains/workflow/interfaces.md) — renaming, retyping, or
// removing a member here is a breaking change no follow-up may make
// silently.
type gateResolvedDoc struct {
	Gate              string                   `json:"gate"`
	Slots             []gateResolvedSlot       `json:"slots"`
	ExpectedReviewers int                      `json:"expected_reviewers"`
	ApproveThreshold  string                   `json:"approve_threshold"`
	Substitution      gateResolvedSubstitution `json:"substitution"`
}

// buildGateResolvedDoc resolves gate's creation-time defaults entirely
// through the R3 config resolvers (PanelGateReviewerSlots/
// PanelGateExpectedReviewers/PanelGateApproveThresholdExpr), so
// renderGateResolved and gateResolvedJSON cannot disagree with them or with
// each other — both delegate here. Returns the resolver's own ADR-0035
// error (already carrying a "recovery:" line enumerating the five valid
// gate keys) for a gate name outside config.PanelGateKeys.
func buildGateResolvedDoc(cfg *config.Config, gate string) (gateResolvedDoc, error) {
	slots, err := cfg.PanelGateReviewerSlots(gate)
	if err != nil {
		return gateResolvedDoc{}, err
	}
	expected, err := cfg.PanelGateExpectedReviewers(gate)
	if err != nil {
		return gateResolvedDoc{}, err
	}
	threshold, err := cfg.PanelGateApproveThresholdExpr(gate)
	if err != nil {
		return gateResolvedDoc{}, err
	}

	docSlots := make([]gateResolvedSlot, len(slots))
	for i, s := range slots {
		docSlots[i] = gateResolvedSlot{Slot: s.Slot, Model: s.Model, Lens: s.Lens}
	}

	inForce := "claude_sub_on_quota"
	if len(cfg.Panel.Substitution.Substitutes) > 0 {
		inForce = "substitutes"
	}

	// A config with no configured substitutes leaves this map nil; marshal
	// it as "{}" (never "null") so JSON consumers (e.g. jq) can always
	// index into .substitution.substitutes without a null-check, matching
	// the never-null treatment renderSubstitutes already gives the text
	// path and slots gives docSlots above.
	substitutes := cfg.Panel.Substitution.Substitutes
	if substitutes == nil {
		substitutes = map[string]string{}
	}

	return gateResolvedDoc{
		Gate:              gate,
		Slots:             docSlots,
		ExpectedReviewers: expected,
		ApproveThreshold:  threshold,
		Substitution: gateResolvedSubstitution{
			Substitutes:      substitutes,
			ClaudeSubOnQuota: cfg.Panel.Substitution.ClaudeSubOnQuota,
			InForce:          inForce,
		},
	}, nil
}

// gateResolvedJSON marshals gate's resolved view (spec 112 R9) with the
// real encoding/json encoder — never string concatenation — so a hostile
// config-controlled string (a model id, a lens, a substitutes key/value)
// round-trips as a properly escaped JSON string rather than forging
// document structure.
func gateResolvedJSON(cfg *config.Config, gate string) ([]byte, error) {
	doc, err := buildGateResolvedDoc(cfg, gate)
	if err != nil {
		return nil, err
	}
	return json.Marshal(doc)
}

// renderGateResolved renders gate's resolved view (spec 112 R8/R9) as
// human-readable text: the expanded slots, expected reviewer count, raw
// approve_threshold expression, and effective substitution policy — every
// config-controlled string escaped. Delegates to buildGateResolvedDoc so
// this cannot disagree with gateResolvedJSON or the R3 resolvers.
func renderGateResolved(cfg *config.Config, gate string) (string, error) {
	doc, err := buildGateResolvedDoc(cfg, gate)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "gate: %s\n", escapeConfigValue(doc.Gate))
	fmt.Fprintln(&b, "slots:")
	for _, s := range doc.Slots {
		fmt.Fprintf(&b, "  - slot: %s\n", escapeConfigValue(s.Slot))
		fmt.Fprintf(&b, "    model: %s\n", escapeConfigValue(s.Model))
		fmt.Fprintf(&b, "    lens: %s\n", escapeConfigValue(s.Lens))
	}
	fmt.Fprintf(&b, "expected_reviewers: %d\n", doc.ExpectedReviewers)
	fmt.Fprintf(&b, "approve_threshold: %s\n", escapeConfigValue(doc.ApproveThreshold))
	fmt.Fprintln(&b, "substitution:")
	renderSubstitutes(&b, "  ", doc.Substitution.Substitutes)
	fmt.Fprintf(&b, "  claude_sub_on_quota: %t\n", doc.Substitution.ClaudeSubOnQuota)
	fmt.Fprintf(&b, "  in_force: %s\n", doc.Substitution.InForce)

	return b.String(), nil
}

// reviewerCountNotesFor scans registered panels under root's review roots
// and returns one panel.ReviewerCountNote line per panel whose recorded
// expected_reviewers differs from the gate-appropriate config default (spec
// 109 R8, gate-aware per spec 112 R7) — empty when no panel is registered or
// every recorded count matches, the common case. The scan/append lives
// HERE, not in renderConfig, which stays pure over *config.Config alone
// (R9); this function performs the ONLY fs I/O `config show` does, and it
// is read-only — panel.Scan opens no files for writing. A malformed
// registration (Err != nil) is skipped: it has no ExpectedReviewers to
// compare. cfg.PanelGateAdvisoryDefault is the SAME single-home selection
// rule internal/complete's reviewerCountAdvisory call site resolves
// through, so the two callers cannot drift from each other; a registration
// where ok is false (the R7 skip carve-outs) is skipped.
func reviewerCountNotesFor(cfg *config.Config, root string) string {
	var b strings.Builder
	for _, reg := range panel.Scan(configShowReviewRoots(root)...) {
		if reg.Err != nil {
			continue
		}
		configDefault, ok := cfg.PanelGateAdvisoryDefault(reg.Panel.Gate, reg.Panel.IsBead())
		if !ok {
			continue
		}
		note := panel.ReviewerCountNote(reg.Panel.ExpectedReviewers, configDefault)
		if note == "" {
			continue
		}
		fmt.Fprintf(&b, "panel %s: %s\n", termsafe.Escape(reg.Slug()), note)
	}
	return b.String()
}

// configShowReviewRoots returns the roots `config show` scans for
// registered panels: the repo root itself (the legacy/canonical root
// `review/` convention) plus every spec's own directory (the co-located
// `<spec-dir>/reviews/` convention, spec 106) plus the workspace dir
// (`.mindspec`, spec 123 R8c — the ad-hoc `.mindspec/reviews/<slug>`
// convention `panel create --gate adhoc` now produces). panel.Scan
// already globs both the `review/` and `reviews/` segments under each
// given root, so this list is the set of DIRECTORIES to check, not the
// segment names. Unlike internal/complete's panelGateRoots (which is
// NEVER extended to `.mindspec` — ad-hoc panels stay outside every
// lifecycle gate, ADR-0037), this is not layout-aware or bead-scoped —
// `config show` (and `panel tally`/`panel verify` via
// findPanelRegistration, panel.go) has no bead/spec context, so it
// checks every convention that might hold a registered panel.
// Best-effort: an unreadable specs directory yields just the repo root
// plus the workspace dir.
func configShowReviewRoots(root string) []string {
	roots := []string{root, workspace.MindspecDir(root)}
	specsDir := workspace.SpecsDir(root)
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		return roots
	}
	for _, e := range entries {
		if e.IsDir() {
			roots = append(roots, filepath.Join(specsDir, e.Name()))
		}
	}
	return roots
}
