package main

// panel.go: `mindspec panel create | verify | tally` — the CLI half of the
// ADR-0040 portability contract (spec 110 Bead 4). Every subcommand is a
// THIN adapter over internal/panel: `create` calls panel.Create (Bead 1's
// leaf-safe writer); `verify`/`tally` call panel.ResolveGateFacts +
// panel.PanelGateDecision — the SAME decision `mindspec complete`'s gate
// enforces (internal/complete.panelGate). No subcommand re-implements the
// allow/block matrix (R7a, pinned below by
// TestPanelVerbs_DecisionIsPanelGateDecision).

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var panelCmd = &cobra.Command{
	Use:   "panel",
	Short: "Manage review panels (create, verify, tally)",
}

// revParseForPanelFn resolves ref's commit SHA in root — the seam `panel
// create` uses to capture reviewed_head_sha AT WRITE TIME (swappable in
// tests, mirroring internal/complete's gateRevParseFn).
var revParseForPanelFn = func(root, ref string) (string, error) {
	return newExecutor(root).RevParseRef(root, ref)
}

// panelWorktreeListFn resolves the bead-worktree list for the panel gate's
// dirty-tree check (mirroring internal/complete's worktreeListFn seam).
// Swappable in tests so `panel verify`/`panel tally` never spawn a real
// `bd` subprocess.
var panelWorktreeListFn = bead.WorktreeList

// tallyWarnOut is where `panel tally` prints its Warn-path advisory
// message (mirroring internal/complete's panelAdvisoryOut seam so tests
// can capture it without redirecting the real os.Stderr).
var tallyWarnOut io.Writer = os.Stderr

var panelCreateCmd = &cobra.Command{
	Use:   "create <slug> --spec <id> --target <ref> [--bead <id>] [--round N]",
	Short: "Register (or re-panel) a review panel",
	Long: `panel create writes <panel-dir>/panel.json and rewrites BRIEF.md's
machine-managed header in one atomic operation (internal/panel.Create):
expected_reviewers and approve_threshold are stamped from the configured
panel defaults (config.PanelExpectedReviewers / PanelApproveThresholdExpr,
spec 109), and reviewed_head_sha is captured from --target's live commit
AT WRITE TIME. A --round N re-panel co-bumps round + reviewed_head_sha in
the SAME write, leaving prior verdict files and the skill-authored BRIEF
body untouched.

A --bead <id> panel expects --target bead/<id> — the exact ref
` + "`mindspec complete`'s gate rev-parses for staleness. A --target that" + `
diverges from bead/<id> can only FAIL SAFE: at gate time the recorded
reviewed_head_sha will not match the live bead/<id> tip, producing a
stale-SHA Block, never a false-PASS.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slug := args[0]
		if err := validatePanelSlug(slug); err != nil {
			return err
		}

		specID, _ := cmd.Flags().GetString("spec")
		target, _ := cmd.Flags().GetString("target")
		beadID, _ := cmd.Flags().GetString("bead")
		round, _ := cmd.Flags().GetInt("round")

		// Control-byte discipline (spec-109-final-review G2): a --bead/
		// --target value carrying a control byte must never reach
		// panel.json or a rendered/recovery message. Same check
		// validatePanelSlug applies to the slug itself.
		if err := rejectControlBytes("--bead", beadID); err != nil {
			return err
		}
		if err := rejectControlBytes("--target", target); err != nil {
			return err
		}
		if strings.TrimSpace(specID) == "" {
			return fmt.Errorf("--spec is required")
		}
		if strings.TrimSpace(target) == "" {
			return fmt.Errorf("--target is required")
		}
		if round <= 0 {
			round = 1
		}

		root, err := findRoot()
		if err != nil {
			return err
		}
		cfg, err := config.Load(root)
		if err != nil {
			return err
		}

		dir, err := panelDirFor(root, specID, slug)
		if err != nil {
			return err
		}

		sha, err := revParseForPanelFn(root, target)
		if err != nil {
			return fmt.Errorf("resolving --target %q: %w", target, err)
		}

		var beadPtr *string
		if beadID != "" {
			beadPtr = &beadID
		}

		in := panel.CreateInput{
			BeadID:               beadPtr,
			Spec:                 specID,
			Target:               target,
			Round:                round,
			ExpectedReviewers:    cfg.PanelExpectedReviewers(),
			ApproveThresholdExpr: cfg.PanelApproveThresholdExpr(),
			ReviewedHeadSHA:      sha,
		}
		if err := panel.Create(dir, in); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "panel %s registered: round %d, %d expected reviewer(s), reviewed_head_sha %s\n",
			slug, round, in.ExpectedReviewers, sha)
		return nil
	},
}

var panelVerifyCmd = &cobra.Command{
	Use:   "verify <slug>",
	Short: "Print a read-only completeness/staleness report; writes nothing",
	Long: `panel verify prints, for the named panel: verdicts present vs
expected_reviewers, per-slot parse status (malformed files named),
reviewed_head_sha vs the live target tip, and a PASS/BLOCK preview
computed by panel.PanelGateDecision over panel.ResolveGateFacts — the
SAME decision ` + "`mindspec complete`" + `'s gate enforces, never a second
implementation. It writes no file and always exits 0 (a read-only report
is not itself a gate).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slug := args[0]
		if err := validatePanelSlug(slug); err != nil {
			return err
		}
		root, err := findRoot()
		if err != nil {
			return err
		}
		reg, err := findPanelRegistration(root, slug)
		if err != nil {
			return err
		}
		facts := resolvePanelGateFacts(root, reg)
		line, _ := renderPanelVerify(facts.Res, facts)
		fmt.Fprintln(cmd.OutOrStdout(), line)
		return nil
	},
}

var panelTallyCmd = &cobra.Command{
	Use:   "tally <slug>",
	Short: "Print the per-slot verdicts, decision, and aggregated concrete changes required",
	Long: `panel tally prints the per-slot verdict table, the aggregate
APPROVE/REQUEST_CHANGES/REJECT counts + resolved threshold, the
panel.PanelGateDecision decision, and the aggregated
concrete_changes_required (read presentation-only from each
REQUEST_CHANGES/REJECT verdict file — never feeding the decision). The
exit code is derived from the decision's Action ALONE: Allow -> 0; Warn
-> 0 with the advisory printed (non-blocking, parity with
internal/complete's Warn handling); Block -> non-zero with a final
recovery line (ADR-0035).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slug := args[0]
		if err := validatePanelSlug(slug); err != nil {
			return err
		}
		root, err := findRoot()
		if err != nil {
			return err
		}
		reg, err := findPanelRegistration(root, slug)
		if err != nil {
			return err
		}
		facts := resolvePanelGateFacts(root, reg)
		changes := collectSlotChanges(reg, facts.Res)
		body, d := renderPanelTally(facts.Res, facts, changes)
		fmt.Fprintln(cmd.OutOrStdout(), body)
		return tallyExitAction(d, slug)
	},
}

func init() {
	panelCreateCmd.Flags().String("spec", "", "Owning spec ID")
	panelCreateCmd.Flags().String("target", "", "Reviewed ref (e.g. bead/<id>)")
	panelCreateCmd.Flags().String("bead", "", "Bead ID this panel targets (omit for a non-bead panel)")
	panelCreateCmd.Flags().Int("round", 1, "Panel round (default 1; bump on re-panel)")

	panelCmd.AddCommand(panelCreateCmd)
	panelCmd.AddCommand(panelVerifyCmd)
	panelCmd.AddCommand(panelTallyCmd)
}

// validatePanelSlug rejects an unsafe <slug> positional argument BEFORE any
// filepath.Join reaches it (spec 110 R1): empty, `.`, `..`, any occurrence
// of `/` or `\`, an absolute path, or any control character. This closes
// both the path-traversal class (a slug like "../../etc" escaping the
// panel-directory root) and the terminal-injection class (the
// spec-109-final-review G2 finding). All three subcommands (create/
// verify/tally) call this first.
func validatePanelSlug(slug string) error {
	const recovery = "pass a plain slug: a single path segment with no `.`/`..`, no `/` or `\\`, and no control characters"
	if slug == "" {
		return guard.NewFailure("panel slug must not be empty", recovery)
	}
	if slug == "." || slug == ".." {
		return guard.NewFailure(fmt.Sprintf("panel slug %q is not a valid directory name", slug), recovery)
	}
	if strings.ContainsAny(slug, "/\\") {
		return guard.NewFailure(fmt.Sprintf("panel slug %q must not contain a path separator", slug), recovery)
	}
	if filepath.IsAbs(slug) {
		return guard.NewFailure(fmt.Sprintf("panel slug %q must not be an absolute path", slug), recovery)
	}
	return rejectControlBytes("slug", slug)
}

// rejectControlBytes rejects a value containing any control character
// (including \n/\r/NUL) — the control-byte discipline validatePanelSlug
// applies to <slug> and `panel create` additionally applies to --bead/
// --target: a value bearing a control byte must never reach a rendered
// message or a guard.NewFailure recovery line, where an embedded newline
// could forge a fake `recovery:` line (spec-109-final-review G2).
func rejectControlBytes(label, value string) error {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return guard.NewFailure(
				fmt.Sprintf("%s %q contains a control character and is rejected", label, value),
				"remove any control characters (including newlines) from the value")
		}
	}
	return nil
}

// panelDirFor resolves the panel directory for slug, layout-aware —
// reusing the same workspace.DetectLayout + workspace.SpecDir logic
// internal/complete.panelGateRoots uses: flat -> co-located
// <spec-dir>/reviews/<slug>, otherwise the repo-root review/<slug>
// convention.
func panelDirFor(root, specID, slug string) (string, error) {
	if layout, _ := workspace.DetectLayout(root); layout == workspace.LayoutFlat {
		specDir, err := workspace.SpecDir(root, specID)
		if err != nil {
			return "", fmt.Errorf("resolving spec dir for %q: %w", specID, err)
		}
		return filepath.Join(specDir, "reviews", slug), nil
	}
	return filepath.Join(root, "review", slug), nil
}

// findPanelRegistration scans every review root `config show` also scans
// (configShowReviewRoots, config.go) and returns the registration whose
// Slug() == slug. A slug not found among any registered panel is a clear
// error with a recovery hint — never a panic, never a silent pass.
func findPanelRegistration(root, slug string) (panel.Registration, error) {
	for _, reg := range panel.Scan(configShowReviewRoots(root)...) {
		if reg.Slug() == slug {
			return reg, nil
		}
	}
	return panel.Registration{}, guard.NewFailure(
		fmt.Sprintf("no registered panel found for slug %q", slug),
		fmt.Sprintf("mindspec panel create %s --spec <id> --target <ref>", slug))
}

// resolvePanelGateFacts gathers panel.GateFacts for reg exactly as
// internal/complete.panelGate does: beadID from the matched panel.json
// (empty for a non-bead panel), scanRoot the panel dir's grandparent, and
// the git I/O wired through the executor. facts.Res is always non-nil
// (panel.ResolveGateFacts tallies reg.Dir internally).
func resolvePanelGateFacts(root string, reg panel.Registration) panel.GateFacts {
	beadID := ""
	if reg.Err == nil && reg.Panel.IsBead() {
		beadID = *reg.Panel.BeadID
	}
	scanRoot := panel.PanelDirScanRoot(reg.Dir)
	exec := newExecutor(root)
	return panel.ResolveGateFacts(reg, beadID, scanRoot, panel.GateIO{
		RevParse:      exec.RevParseRef,
		Status:        exec.Status,
		IsRefNotFound: exec.IsRefNotFound,
		Worktree:      panelBeadWorktreePath(beadID),
	})
}

// panelBeadWorktreePath resolves the bead worktree path for the dirty-tree
// check ("" = absent -> dirty check skipped), lazily so its cost (and the
// panelWorktreeListFn subprocess) is only paid on the dirty-check path —
// mirroring internal/complete's own lazy Worktree closure.
func panelBeadWorktreePath(beadID string) func() string {
	return func() string {
		if beadID == "" {
			return ""
		}
		entries, err := panelWorktreeListFn()
		if err != nil {
			return ""
		}
		expectedName := workspace.BeadWorktreeName(beadID)
		expectedBranch := workspace.BeadBranch(beadID)
		for _, e := range entries {
			if e.Name == expectedName || e.Branch == expectedBranch {
				return e.Path
			}
		}
		return ""
	}
}

// renderPanelVerify renders `panel verify`'s read-only completeness/
// staleness report from res/facts (Bead 4 step 2). It is PURE: no I/O. The
// returned action is panel.PanelGateDecision(facts).Action — the identical
// decision `mindspec complete`'s gate enforces (R7a); this function
// computes no allow/block logic of its own.
func renderPanelVerify(res *panel.Result, facts panel.GateFacts) (string, panel.GateAction) {
	d := panel.PanelGateDecision(facts)

	slug := ""
	if facts.Reg != nil {
		slug = facts.Reg.Slug()
	}

	var b strings.Builder
	fmt.Fprintf(&b, "panel %s: %d/%d verdict(s) present", slug, len(res.Verdicts), res.ExpectedReviewers())
	if len(res.Malformed) > 0 {
		fmt.Fprintf(&b, " (malformed: %s)", strings.Join(res.Malformed, ", "))
	}
	b.WriteString("\n")

	if res.Panel != nil && res.Panel.ReviewedHeadSHA != "" {
		switch {
		case facts.MissingRef:
			fmt.Fprintf(&b, "reviewed_head_sha: %s (target branch no longer exists — assumed merged)\n", res.Panel.ReviewedHeadSHA)
		case facts.GitErr != nil:
			fmt.Fprintf(&b, "reviewed_head_sha: %s (could not verify live tip: %v)\n", res.Panel.ReviewedHeadSHA, facts.GitErr)
		case facts.HeadSHA != "":
			fmt.Fprintf(&b, "reviewed_head_sha: %s — live tip: %s\n", res.Panel.ReviewedHeadSHA, facts.HeadSHA)
		}
	}

	switch d.Action {
	case panel.Allow:
		b.WriteString("PASS\n")
	case panel.Warn:
		fmt.Fprintf(&b, "PASS (advisory: %s)\n", d.Message)
	case panel.Block:
		fmt.Fprintf(&b, "BLOCK: %s\n", d.Message)
	}

	return strings.TrimRight(b.String(), "\n"), d.Action
}

// slotChanges is one reviewer slot's presentation-only
// concrete_changes_required aggregation for `panel tally` (Bead 4 step 3).
// DecodeErr, when non-empty, is an advisory note — the file could not be
// re-read/re-parsed, or its concrete_changes_required key was absent or
// not an array of strings. This NEVER affects the gate decision or the
// exit code; it is presentation only.
type slotChanges struct {
	Slot      string
	Changes   []string
	DecodeErr string
}

// panelVerdictChangesJSON is the on-disk shape `panel tally` re-reads for
// its concrete_changes_required aggregation. panel.Tally's own verdictJSON
// parser strips this field (it is presentation-only, never gate input),
// so tally re-decodes it itself.
type panelVerdictChangesJSON struct {
	ConcreteChangesRequired json.RawMessage `json:"concrete_changes_required"`
}

// collectSlotChanges iterates res.Verdicts of the latest round and, for
// each REQUEST_CHANGES/REJECT verdict, re-decodes its verdict file's
// concrete_changes_required array (Bead 4 step 3). A re-parse failure, an
// absent key, or a non-array-of-strings type attributes ZERO items to
// that slot with an advisory DecodeErr — never fatal, never silently
// dropped. This read never feeds panel.PanelGateDecision or the exit code.
func collectSlotChanges(reg panel.Registration, res *panel.Result) []slotChanges {
	if res == nil {
		return nil
	}
	var out []slotChanges
	for _, v := range res.Verdicts {
		if v.Verdict != panel.VerdictRequestChanges && v.Verdict != panel.VerdictReject {
			continue
		}
		sc := slotChanges{Slot: v.Slot}
		data, err := os.ReadFile(filepath.Join(reg.Dir, v.File))
		if err != nil {
			sc.DecodeErr = fmt.Sprintf("could not re-read %s: %v", v.File, err)
			out = append(out, sc)
			continue
		}
		var parsed panelVerdictChangesJSON
		if err := json.Unmarshal(data, &parsed); err != nil {
			sc.DecodeErr = fmt.Sprintf("could not re-parse %s: %v", v.File, err)
			out = append(out, sc)
			continue
		}
		if len(parsed.ConcreteChangesRequired) == 0 {
			sc.DecodeErr = fmt.Sprintf("%s has no concrete_changes_required key", v.File)
			out = append(out, sc)
			continue
		}
		var items []string
		if err := json.Unmarshal(parsed.ConcreteChangesRequired, &items); err != nil {
			sc.DecodeErr = fmt.Sprintf("%s's concrete_changes_required is not an array of strings", v.File)
			out = append(out, sc)
			continue
		}
		sc.Changes = items
		out = append(out, sc)
	}
	return out
}

// renderPanelTally renders `panel tally`'s body from res/facts/changes
// (Bead 4 step 3). It is PURE: no I/O. The returned Decision is
// panel.PanelGateDecision(facts) — the identical decision `mindspec
// complete`'s gate enforces (R7a); this function computes no allow/block
// logic of its own. changes entries are escaped (escapeConfigValue,
// config.go) before rendering, so a REQUEST_CHANGES author cannot inject
// extra lines into the aggregated output via a newline/control byte.
func renderPanelTally(res *panel.Result, facts panel.GateFacts, changes []slotChanges) (string, panel.Decision) {
	d := panel.PanelGateDecision(facts)

	slug := ""
	if facts.Reg != nil {
		slug = facts.Reg.Slug()
	}

	var b strings.Builder
	fmt.Fprintf(&b, "panel %s — per-slot verdicts:\n", slug)
	if len(res.Verdicts) == 0 {
		b.WriteString("  (none)\n")
	}
	for _, v := range res.Verdicts {
		hb := ""
		if v.HardBlock {
			hb = " hard_block"
		}
		fmt.Fprintf(&b, "  %s: %s%s\n", v.Slot, v.Verdict, hb)
	}
	if len(res.Malformed) > 0 {
		fmt.Fprintf(&b, "  malformed (counted as missing): %s\n", strings.Join(res.Malformed, ", "))
	}

	expected := res.ExpectedReviewers()
	threshold := 0
	if res.Panel != nil {
		threshold = res.Panel.ApproveThreshold()
	}
	neutral := len(res.Verdicts) - res.Approves - res.Rejects
	fmt.Fprintf(&b, "\nAPPROVE %d / REQUEST_CHANGES %d / REJECT %d — threshold %d/%d\n",
		res.Approves, neutral, res.Rejects, threshold, expected)

	b.WriteString("\ndecision: ")
	switch d.Action {
	case panel.Allow:
		b.WriteString("PASS\n")
	case panel.Warn:
		fmt.Fprintf(&b, "PASS (advisory)\n%s\n", d.Message)
	case panel.Block:
		fmt.Fprintf(&b, "BLOCK\n%s\n", d.Message)
	}

	if len(changes) > 0 {
		b.WriteString("\nconcrete_changes_required (aggregated):\n")
		for _, sc := range changes {
			if sc.DecodeErr != "" {
				fmt.Fprintf(&b, "  %s: (advisory) %s\n", sc.Slot, escapeConfigValue(sc.DecodeErr))
				continue
			}
			for _, c := range sc.Changes {
				fmt.Fprintf(&b, "  %s: %s\n", sc.Slot, escapeConfigValue(c))
			}
		}
	}

	return strings.TrimRight(b.String(), "\n"), d
}

// tallyExitAction derives `panel tally`'s exit purely from d.Action — the
// SAME panel.Decision renderPanelTally already returns — never from raw
// verdict counts (res.Approves etc.): panel.Allow -> nil (exit 0);
// panel.Warn -> print the advisory to tallyWarnOut and return nil (exit 0,
// non-blocking, parity with internal/complete.panelGate's Warn handling);
// panel.Block -> guard.NewFailure carrying PanelGateDecision's raw-`git
// merge` fence in the body plus a genuine recovery line (ADR-0035). A
// regression that re-derives Allow/Block from the raw counts instead of
// d.Action (passing every planned gate yet exiting 0 on a stale-SHA or
// hard_block Block, the lola-f4a8 class) is caught by
// TestPanelTally_ExitCodeTracksDecision.
func tallyExitAction(d panel.Decision, slug string) error {
	switch d.Action {
	case panel.Warn:
		fmt.Fprintf(tallyWarnOut, "panel advisory: %s\n", d.Message)
		return nil
	case panel.Block:
		return guard.NewFailure(d.Message, fmt.Sprintf(
			"re-run the panel (mindspec panel create %s --round <N+1> ...), then mindspec complete <bead>", slug))
	default:
		return nil
	}
}
