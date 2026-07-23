package main

// Spec 122 (domain-adr-gate-truthfulness) Bead 4, AC-14(a): the ceremony
// non-inflation surface guard. Requirement 7's claim is that this spec adds
// NO new gate lane, NO new CLI flag, and NO new config key anywhere — every
// change either rejects an authoring-time input EARLIER (R1), passes a
// previously-false-failing correct input (R2/R3), or corrects hint TEXT
// (R4); none of those need a new flag, override, or config key. This file
// pins the four surfaces the spec names (`mindspec complete --help`,
// `mindspec impl approve --help`, `mindspec validate --help`, and the
// `mindspec config show` effective-config key set) to the exact
// pre-spec-122 baseline, recorded here from the ACTUAL rendered output at
// bead-implementation time (spec 122's Beads 1-3 land no flag/key changes,
// by design, so "current" and "pre-spec-122" coincide) — so any FUTURE
// change (this spec or a later one) that adds a flag or config key to one
// of these surfaces trips this guard red.

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// commandFlagSet returns the EXACT set of long-flag names ("--name") a
// cobra command exposes, walking pflag METADATA rather than scraping the
// rendered --help text (spec 122 Bead 4, FX-2 / codex G-1). The prior
// regex-over-help-text approach (`--[a-zA-Z][a-zA-Z0-9-]*`) truncated at
// the first non-`[-a-z0-9]` byte, so an underscore-named flag like
// `--trace_json` extracted only as `--trace` — and because `--trace` was
// already pinned, the NEW flag slipped past the guard undetected,
// defeating its whole purpose. VisitAll over the flag objects yields each
// flag's real, whole `.Name`, so `--trace_json` is a distinct member that
// assertSetEqual reports as unexpected.
//
// It unions LocalFlags (the command's own local + persistent flags) with
// InheritedFlags (persistent flags merged down from ancestors, e.g. the
// root's `--trace`) — the same two sources cobra composes to render a
// leaf command's Flags:/Global Flags: help sections — and calls
// InitDefaultHelpFlag first so the auto-injected `--help` (added lazily at
// help/execute time, absent from a never-executed command's flag set) is
// present exactly as the operator sees it.
func commandFlagSet(cmd *cobra.Command) map[string]bool {
	cmd.InitDefaultHelpFlag()
	set := map[string]bool{}
	add := func(f *pflag.Flag) { set["--"+f.Name] = true }
	cmd.LocalFlags().VisitAll(add)
	cmd.InheritedFlags().VisitAll(add)
	return set
}

// resolveCommand locates a leaf command by its argv path off the real
// rootCmd (the in-process command tree this binary ships), so the guard
// reads the SAME command objects `mindspec <path> --help` renders — no
// subprocess build, no help-text parsing.
func resolveCommand(t *testing.T, path ...string) *cobra.Command {
	t.Helper()
	cmd, _, err := rootCmd.Find(path)
	if err != nil {
		t.Fatalf("resolveCommand %v: %v", path, err)
	}
	// rootCmd.Find returns the nearest ancestor when the full path is a
	// bare parent command (e.g. `validate` with subcommands), but for the
	// leaf paths this guard uses Find lands exactly on the named command;
	// assert that so a future rename does not silently retarget the guard.
	if cmd.Name() != path[len(path)-1] {
		t.Fatalf("resolveCommand %v resolved to %q, not the requested leaf", path, cmd.Name())
	}
	return cmd
}

func setOf(items ...string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, it := range items {
		set[it] = true
	}
	return set
}

// diffSets is the pure comparison core: the sorted members of want missing
// from got, and the sorted members of got not in want. Factored out of
// assertSetEqual so the guard-of-the-guard can assert the DIFF directly
// (proving a planted underscore flag lands in `unexpected`) without faking
// a *testing.T.
func diffSets(got, want map[string]bool) (missing, unexpected []string) {
	for k := range want {
		if !got[k] {
			missing = append(missing, k)
		}
	}
	for k := range got {
		if !want[k] {
			unexpected = append(unexpected, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(unexpected)
	return missing, unexpected
}

// assertSetEqual fails with a readable diff (missing / unexpected members)
// when got != want. An "unexpected" member is exactly the ceremony-
// inflation signal this guard exists to catch; a "missing" member would
// mean a flag/key silently disappeared (also a surface change worth
// failing loudly on, even though R7 only promises no ADDITIONS).
func assertSetEqual(t *testing.T, label string, got, want map[string]bool) {
	t.Helper()
	missing, unexpected := diffSets(got, want)
	if len(missing) > 0 {
		t.Errorf("%s: baseline member(s) disappeared: %v", label, missing)
	}
	if len(unexpected) > 0 {
		t.Errorf("%s: NEW member(s) not in the pre-spec-122 baseline (ceremony inflation): %v", label, unexpected)
	}
}

// TestCeremonyNonInflation_HelpFlags pins AC-14(a)'s three flag surfaces to
// the pre-spec-122 baseline, reading each command's flag set from pflag
// metadata (FX-2).
func TestCeremonyNonInflation_HelpFlags(t *testing.T) {
	cases := []struct {
		name string
		path []string
		want map[string]bool
	}{
		{
			name: "mindspec complete",
			path: []string{"complete"},
			want: setOf("--allow-doc-skew", "--help", "--override-adr", "--spec", "--supersede-adr", "--trace"),
		},
		{
			name: "mindspec impl approve",
			path: []string{"impl", "approve"},
			want: setOf("--allow-doc-skew", "--help", "--override-adr", "--supersede-adr", "--trace"),
		},
		{
			name: "mindspec validate",
			path: []string{"validate"},
			want: setOf("--format", "--help", "--trace"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := commandFlagSet(resolveCommand(t, tc.path...))
			assertSetEqual(t, tc.name, got, tc.want)
		})
	}
}

// TestCeremonyNonInflation_FlagGuardCatchesUnderscore is the guard-of-the-
// guard for FX-2: it proves the metadata extractor sees an underscore-named
// flag as its WHOLE name (so assertSetEqual would flag it as inflation),
// where the old regex approach would have truncated it to a
// already-pinned prefix and let it pass. It builds a throwaway command
// carrying the `complete` baseline PLUS a planted `--trace_json`, extracts
// via commandFlagSet, and asserts the diff against the baseline names the
// underscore flag as unexpected — AND that the truncated prefix `--trace`
// (a real baseline member) is NOT falsely consumed by it.
func TestCeremonyNonInflation_FlagGuardCatchesUnderscore(t *testing.T) {
	baseline := setOf("--allow-doc-skew", "--help", "--override-adr", "--spec", "--supersede-adr", "--trace")

	planted := &cobra.Command{Use: "sample"}
	fs := planted.Flags()
	// Reconstruct the baseline flag surface on a throwaway command...
	fs.String("allow-doc-skew", "", "")
	fs.String("override-adr", "", "")
	fs.String("spec", "", "")
	fs.String("supersede-adr", "", "")
	fs.String("trace", "", "")
	// ...then plant the underscore flag the old regex-scrape would have
	// silently truncated to the already-pinned "--trace".
	fs.String("trace_json", "", "")

	got := commandFlagSet(planted)

	_, unexpected := diffSets(got, baseline)
	foundUnderscore := false
	for _, u := range unexpected {
		if u == "--trace_json" {
			foundUnderscore = true
		}
	}
	if !foundUnderscore {
		t.Errorf("guard-of-the-guard FAILED: a planted --trace_json flag was NOT reported as unexpected (unexpected=%v) — the flag extractor is truncating underscore names again", unexpected)
	}
	// The whole-name extractor must ALSO still see the genuine "--trace"
	// baseline member (i.e. the underscore flag did not shadow/consume it).
	if !got["--trace"] {
		t.Errorf("guard-of-the-guard FAILED: the baseline --trace flag went missing from the extracted set %v", got)
	}
}

// configKeyRe matches a YAML-shaped "key:" token at the START of a
// (post-comment-strip) line, capturing its leading-space indent and name.
var configKeyRe = regexp.MustCompile(`^([ ]*)([A-Za-z_][A-Za-z0-9_]*):`)

// configKeySet flattens renderConfig's rendered text into the set of
// dotted key paths it declares (e.g. "enforcement.pre_commit_hook"),
// tracking nesting by indentation. List-item lines (leading "-", e.g. the
// individual protected_branches/panel.reviewers entries) are skipped
// entirely — this guard's claim is about named orchestration KNOBS (map
// keys), not the field shape of existing list elements — with one known,
// harmless over-capture: a scalar field nested two levels under a skipped
// list-item line (e.g. panel.reviewers[].count) still matches this regex
// and gets attributed to the enclosing map key's path (surfacing here as
// "panel.reviewers.count"). That is conservative (adds a pinned key, never
// hides one), so it does not weaken the guard.
func configKeySet(rendered string) map[string]bool {
	type frame struct {
		indent int
		path   string
	}
	var stack []frame
	keys := map[string]bool{}
	for _, rawLine := range strings.Split(rendered, "\n") {
		line := rawLine
		if idx := strings.Index(line, "  #"); idx >= 0 {
			line = line[:idx]
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "-") {
			continue
		}
		m := configKeyRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		indent := len(m[1])
		name := m[2]
		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		path := name
		if len(stack) > 0 {
			path = stack[len(stack)-1].path + "." + name
		}
		keys[path] = true
		stack = append(stack, frame{indent: indent, path: path})
	}
	return keys
}

// TestCeremonyNonInflation_ConfigKeys pins AC-14(a)'s fourth surface:
// `mindspec config show`'s effective-config key set (called in-process via
// renderConfig, the exact function `config show` invokes, over a
// no-.mindspec-config.yaml temp dir so the result is DefaultConfig() —
// deterministic and independent of this repo's real .mindspec/config.yaml,
// which any developer could otherwise edit to add a NEW key and hide it
// from a --help-only guard).
func TestCeremonyNonInflation_ConfigKeys(t *testing.T) {
	tmp := t.TempDir()
	cfg, err := config.Load(tmp)
	if err != nil {
		t.Fatalf("config.Load(empty dir): %v", err)
	}
	rendered, err := renderConfig(cfg)
	if err != nil {
		t.Fatalf("renderConfig: %v", err)
	}
	got := configKeySet(rendered)

	want := setOf(
		"auto_finalize",
		"auto_merge_finalize_pr",
		"auto_open_finalize_pr",
		"commands",
		"decomposition",
		"decomposition.max_beads",
		"decomposition.max_chain_depth",
		"decomposition.max_scope_overlap",
		"decomposition.min_parallelism",
		"decomposition.min_scope_overlap",
		"enforcement",
		"enforcement.agent_hooks",
		"enforcement.cli_guards",
		"enforcement.panel_gate",
		"enforcement.pre_commit_hook",
		"loop",
		"loop.budget",
		"loop.budget.max_beads_per_wake",
		"loop.budget.token_budget",
		"loop.context",
		"loop.context.controller_handoff",
		"loop.enabled",
		"loop.gate_authority",
		"loop.gate_authority.bead_merge",
		"loop.gate_authority.impl_approve",
		"loop.gate_authority.plan_approve",
		"loop.gate_authority.spec_approve",
		"loop.halt",
		"loop.halt.max_consecutive_impl_failures",
		"loop.halt.max_rounds_per_bead",
		"loop.halt.on_reject",
		"loop.halt.panel_deadlock_rounds",
		"loop.handoff_log",
		"merge_strategy",
		"models",
		"panel",
		"panel.approve_threshold",
		"panel.gates",
		"panel.reviewers",
		"panel.reviewers.count",
		"panel.substitution",
		"panel.substitution.claude_sub_on_quota",
		"panel.substitution.substitutes",
		"protected_branches",
		"recording",
		"recording.enabled",
		"runner",
		"source_globs",
		"worktree_root",
	)
	assertSetEqual(t, "mindspec config show", got, want)
}
