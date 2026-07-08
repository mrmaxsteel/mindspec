package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/panel"
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
in this release.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findRoot()
		if err != nil {
			return err
		}
		cfg, err := config.Load(root)
		if err != nil {
			return err
		}
		out, err := renderConfig(cfg)
		if err != nil {
			return err
		}
		w := cmd.OutOrStdout()
		fmt.Fprint(w, out)
		fmt.Fprint(w, reviewerCountNotesFor(cfg, root))
		return nil
	},
}

func init() {
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
	for _, r := range s {
		if r < 0x20 || r > 0x7e {
			return strconv.Quote(s)
		}
	}
	return s
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

// reviewerCountNotesFor scans registered panels under root's review roots
// and returns one panel.ReviewerCountNote line per panel whose recorded
// expected_reviewers differs from cfg's current PanelExpectedReviewers
// default (spec 109 R8) — empty when no panel is registered or every
// recorded count matches, the common case. The scan/append lives HERE, not
// in renderConfig, which stays pure over *config.Config alone (R9); this
// function performs the ONLY fs I/O `config show` does, and it is
// read-only — panel.Scan opens no files for writing. A malformed
// registration (Err != nil) is skipped: it has no ExpectedReviewers to
// compare.
func reviewerCountNotesFor(cfg *config.Config, root string) string {
	var b strings.Builder
	configDefault := cfg.PanelExpectedReviewers()
	for _, reg := range panel.Scan(configShowReviewRoots(root)...) {
		if reg.Err != nil {
			continue
		}
		note := panel.ReviewerCountNote(reg.Panel.ExpectedReviewers, configDefault)
		if note == "" {
			continue
		}
		fmt.Fprintf(&b, "panel %s: %s\n", reg.Slug(), note)
	}
	return b.String()
}

// configShowReviewRoots returns the roots `config show` scans for
// registered panels: the repo root itself (the legacy/canonical root
// `review/` convention) plus every spec's own directory (the co-located
// `<spec-dir>/reviews/` convention, spec 106). panel.Scan already globs
// both the `review/` and `reviews/` segments under each given root, so
// this list is the set of DIRECTORIES to check, not the segment names.
// Unlike internal/complete's panelGateRoots, this is not layout-aware or
// bead-scoped — `config show` has no bead/spec context, so it checks every
// convention that might hold a registered panel. Best-effort: an
// unreadable specs directory yields just the repo root.
func configShowReviewRoots(root string) []string {
	roots := []string{root}
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
