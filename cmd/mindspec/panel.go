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
	"unicode"

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
// tests, mirroring internal/complete's gateRevParseFn). Spec 113 R1 routes
// a SECOND caller through this same seam: `nonBeadTargetRevParse` (below)
// uses it to rev-parse a non-bead panel's RECORDED panel.json.target for
// `panel verify`/`panel tally` staleness — so both the write-time capture
// and the read-time non-bead staleness check are stubbable at one place in
// tests, and never spawn a real git subprocess.
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
stale-SHA Block, never a false-PASS.

An optional --gate <name> (spec 112 R9 / spec 113 R3) stamps the
decision-inert panel.json "gate" field and resolves expected_reviewers/
approve_threshold from that gate's creation-time defaults
(config.PanelGateExpectedReviewers/PanelGateApproveThresholdExpr) instead
of the global panel.reviewers/approve_threshold. --gate must be one of
the five config.PanelGateKeys (spec_approve, plan_approve, bead,
final_review, adhoc); omitting it is byte-identical to today: the global
defaults are used and no "gate" key is written.`,
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
		gate, _ := cmd.Flags().GetString("gate")

		// Control-byte discipline (spec-109-final-review G2): a --bead/
		// --target value carrying a control byte must never reach
		// panel.json or a rendered/recovery message. Same check
		// validatePanelSlug applies to the slug itself. --gate (spec 113
		// R3) gets the same discipline, applied BEFORE the enum-membership
		// check below so a control-byte value never reaches a rendered
		// message either way.
		if err := rejectControlBytes("--bead", beadID); err != nil {
			return err
		}
		if err := rejectControlBytes("--target", target); err != nil {
			return err
		}
		if err := rejectControlBytes("--gate", gate); err != nil {
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
		// --gate membership, validated BEFORE any filesystem write or
		// root/config resolution side effect (spec 113 R3): a value
		// outside config.PanelGateKeys — the single enum declaration,
		// internal/config/config.go:101 — is rejected with a recovery
		// line naming all five keys (ADR-0035), never a second copy of
		// the enum.
		if gate != "" && !isValidPanelGateKey(gate) {
			keys := strings.Join(config.PanelGateKeys, ", ")
			return guard.NewFailure(
				fmt.Sprintf("--gate %q is not one of the five valid panel gate keys (%s)", gate, keys),
				fmt.Sprintf("pass one of %s to --gate", keys))
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

		// Spec 113 R3: when --gate is set, resolve the creation-time
		// defaults through 112 R3's gate-scoped resolvers instead of the
		// global ones; when omitted, EXACTLY today's global-resolver
		// calls with Gate: "" — preserving the 112-R9
		// byte-identical-when-absent contract. The resolver errors are
		// unreachable post-validation (gate is already confirmed a
		// member of PanelGateKeys above) but returned defensively rather
		// than ignored.
		expectedReviewers := cfg.PanelExpectedReviewers()
		approveThresholdExpr := cfg.PanelApproveThresholdExpr()
		if gate != "" {
			expectedReviewers, err = cfg.PanelGateExpectedReviewers(gate)
			if err != nil {
				return err
			}
			approveThresholdExpr, err = cfg.PanelGateApproveThresholdExpr(gate)
			if err != nil {
				return err
			}
		}

		in := panel.CreateInput{
			BeadID:               beadPtr,
			Spec:                 specID,
			Target:               target,
			Round:                round,
			ExpectedReviewers:    expectedReviewers,
			ApproveThresholdExpr: approveThresholdExpr,
			ReviewedHeadSHA:      sha,
			Gate:                 gate,
		}
		if err := panel.Create(dir, in); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "panel %s registered: round %d, %d expected reviewer(s), reviewed_head_sha %s\n",
			slug, round, in.ExpectedReviewers, sha)
		fmt.Fprintf(cmd.OutOrStdout(), "panel directory: %s\n", dir)
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

		// Spec 113 R1: a non-bead panel is not complete-gated, so its exit
		// path must never route through tallyExitAction's bead-templated
		// recovery (whose `<bead>` literal would re-introduce a forbidden
		// `mindspec complete <bead>` instruction on a panel with no bead to
		// complete). tallyExitAction itself stays 2-arg and byte-identical
		// (its sole test caller, TestPanelTally_ExitCodeTracksDecision, is
		// unmodified) — the non-bead recovery is rendered HERE instead.
		if reg.Err == nil && reg.Panel.IsBead() {
			return tallyExitAction(d, slug)
		}
		target := ""
		if facts.Res != nil && facts.Res.Panel != nil {
			target = facts.Res.Panel.Target
		}
		return tallyExitActionNonBead(d, slug, target, reg.Panel.Gate)
	},
}

func init() {
	panelCreateCmd.Flags().String("spec", "", "Owning spec ID")
	panelCreateCmd.Flags().String("target", "", "Reviewed ref (e.g. bead/<id>)")
	panelCreateCmd.Flags().String("bead", "", "Bead ID this panel targets (omit for a non-bead panel)")
	panelCreateCmd.Flags().Int("round", 1, "Panel round (default 1; bump on re-panel)")
	panelCreateCmd.Flags().String("gate", "", "Panel gate mix (spec_approve|plan_approve|bead|final_review|adhoc); stamps the decision-inert gate field and its creation-time defaults (omit for global defaults)")

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

// rejectControlBytes rejects a value containing any control character —
// C0 (including \n/\r/NUL), DEL, or C1 (U+0080-U+009F, e.g. the CSI
// U+009B terminal-injection vector report.go's stripControl already
// documents as the 'codex-render-leak #2' incident) — via
// unicode.IsControl, mirroring stripControl's own predicate. The
// control-byte discipline validatePanelSlug applies to <slug> and `panel
// create` additionally applies to --bead/--target: a value bearing a
// control byte must never reach a rendered message or a guard.NewFailure
// recovery line, where an embedded newline could forge a fake `recovery:`
// line (spec-109-final-review G2).
func rejectControlBytes(label, value string) error {
	for _, r := range value {
		if unicode.IsControl(r) {
			return guard.NewFailure(
				fmt.Sprintf("%s %q contains a control character and is rejected", label, value),
				"remove any control characters (including newlines) from the value")
		}
	}
	return nil
}

// isValidPanelGateKey reports whether gate is a member of
// config.PanelGateKeys — the SINGLE enum declaration
// (internal/config/config.go:101). This iterates that slice rather than
// duplicating its literal values, per spec 113 R3: `panel create --gate`
// must never carry a second copy of the five-key enum.
func isValidPanelGateKey(gate string) bool {
	for _, k := range config.PanelGateKeys {
		if k == gate {
			return true
		}
	}
	return false
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
//
// Spec 113 R1: for a NON-BEAD panel (beadID == ""), internal/panel.
// ResolveGateFacts unconditionally rev-parses "bead/"+beadID — the
// always-absent literal ref "bead/" when beadID is empty. Rather than let
// that doomed rev-parse run (it always fails ErrRefNotFound, short-
// circuiting PanelGateDecision at the missing-ref leg and shadowing
// staleness/incomplete/REJECT/threshold), the bead-path's
// exec.RevParseRef is swapped for nonBeadTargetRevParse, which ignores the
// "bead/"-derived ref argument ResolveGateFacts passes and instead
// rev-parses the panel's RECORDED reg.Panel.Target. internal/panel itself
// is untouched (zero-byte diff) — this is purely caller-side fact
// gathering through the GateIO seam ADR-0037 designed for exactly this.
func resolvePanelGateFacts(root string, reg panel.Registration) panel.GateFacts {
	beadID := ""
	if reg.Err == nil && reg.Panel.IsBead() {
		beadID = *reg.Panel.BeadID
	}
	scanRoot := panel.PanelDirScanRoot(reg.Dir)
	exec := newExecutor(root)

	revParse := exec.RevParseRef
	if beadID == "" {
		revParse = nonBeadTargetRevParse(reg)
	}

	return panel.ResolveGateFacts(reg, beadID, scanRoot, panel.GateIO{
		RevParse:      revParse,
		Status:        exec.Status,
		IsRefNotFound: exec.IsRefNotFound,
		Worktree:      panelBeadWorktreePath(beadID),
	})
}

// nonBeadTargetRevParse builds the GateIO.RevParse closure a non-bead
// panel's `panel verify`/`panel tally` uses (Bead 1 Half 1, R1): it IGNORES
// the "bead/"+beadID ref argument panel.ResolveGateFacts always passes
// (beadID == "" for a non-bead panel, so that ref is the literal,
// always-absent "bead/") and instead rev-parses reg.Panel's RECORDED
// Target in the caller-supplied scanRoot, through the SAME
// revParseForPanelFn seam `panel create` uses to capture reviewed_head_sha
// at write time. This un-shadows PanelGateDecision's staleness (leg 6) and
// incomplete/REJECT/threshold legs (8)-(10) for a non-bead panel: a stale
// target now Blocks, a genuinely-deleted target (exec.IsRefNotFound, still
// wired to errors.Is(err, gitutil.ErrRefNotFound)) still gets the honest
// missing-ref Warn, and a transient git error still gets the honest
// transient Warn (leg 5b) — never a false MissingRef.
//
// When the registration itself is unparsed (reg.Err != nil) or its Target
// is empty (a panel.json that, however it was produced, records no target
// ref), this returns a plain non-ErrRefNotFound error so the facts surface
// as the honest transient GitErr Warn rather than a false "target deleted"
// claim — though an unreadable registration Blocks first regardless, at
// leg (2), before this closure is ever invoked.
func nonBeadTargetRevParse(reg panel.Registration) func(scanRoot, ref string) (string, error) {
	return func(scanRoot, _ string) (string, error) {
		if reg.Err != nil {
			return "", fmt.Errorf("panel.json records no target ref: %w", reg.Err)
		}
		target := strings.TrimSpace(reg.Panel.Target)
		if target == "" {
			return "", fmt.Errorf("panel.json records no target ref")
		}
		return revParseForPanelFn(scanRoot, target)
	}
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

	nonBead := res == nil || res.Panel == nil || !res.Panel.IsBead()
	target := ""
	if res != nil && res.Panel != nil {
		target = res.Panel.Target
	}
	if nonBead {
		d = sanitizeNonBeadDecision(d, slug, target, facts.GitErr)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "panel %s: %d/%d verdict(s) present", slug, len(res.Verdicts), res.ExpectedReviewers())
	if len(res.Malformed) > 0 {
		fmt.Fprintf(&b, " (malformed: %s)", strings.Join(res.Malformed, ", "))
	}
	b.WriteString("\n")

	if res.Panel != nil && res.Panel.ReviewedHeadSHA != "" {
		// displayTarget renders target (read RAW from a repo-write-
		// attacker-poisonable panel.json) via escapeConfigValue
		// (config.go, spec 109 G2's escaper) so a control byte/ANSI
		// sequence/embedded newline in a hand-edited panel.json's
		// "target" can never forge extra terminal lines here (spec-113-
		// final G2, finding 2). A no-op for any clean target (every
		// printable-ASCII value, including every real branch/spec ref).
		displayTarget := escapeConfigValue(target)
		switch {
		case facts.MissingRef && nonBead:
			fmt.Fprintf(&b, "reviewed_head_sha: %s (target %s no longer exists)\n", res.Panel.ReviewedHeadSHA, displayTarget)
		case facts.MissingRef:
			fmt.Fprintf(&b, "reviewed_head_sha: %s (target branch no longer exists — assumed merged)\n", res.Panel.ReviewedHeadSHA)
		case facts.GitErr != nil && nonBead:
			// facts.GitErr wraps "rev-parse %s: %w" (gitutil.RevParseRef)
			// with the RAW target — a NUL byte in target defeats the
			// ErrRefNotFound sentinel match, routing here instead of the
			// escaped MissingRef leg above (spec-113-final S2, empirically
			// confirmed with the real binary: a NUL/ESC/newline-bearing
			// target re-leaked through this exact %v). Render the error's
			// STRING through escapeConfigValue, never raw, so those
			// control bytes can never reach the terminal here either.
			fmt.Fprintf(&b, "reviewed_head_sha: %s (could not verify target %s: %s)\n", res.Panel.ReviewedHeadSHA, displayTarget, escapeConfigValue(facts.GitErr.Error()))
		case facts.GitErr != nil:
			fmt.Fprintf(&b, "reviewed_head_sha: %s (could not verify live tip: %v)\n", res.Panel.ReviewedHeadSHA, facts.GitErr)
		case facts.HeadSHA != "" && nonBead:
			fmt.Fprintf(&b, "reviewed_head_sha: %s — target %s live tip: %s\n", res.Panel.ReviewedHeadSHA, displayTarget, facts.HeadSHA)
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

	if res == nil || res.Panel == nil || !res.Panel.IsBead() {
		target := ""
		if res != nil && res.Panel != nil {
			target = res.Panel.Target
		}
		d = sanitizeNonBeadDecision(d, slug, target, facts.GitErr)
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

// tallyExitActionNonBead mirrors tallyExitAction's Action -> exit contract
// for a NON-BEAD panel (Bead 1 step 4, R1), rendered here in the RunE
// handler rather than inside the pinned 2-arg tallyExitAction so that
// helper — and its sole test caller, TestPanelTally_ExitCodeTracksDecision
// — stays byte-identical. d.Message has ALREADY been sanitized by
// renderPanelTally/sanitizeNonBeadDecision, so it carries no bead/<empty>
// fragment and no RawMergeFence. This function's own Block recovery line
// names a genuine re-panel command against the recorded target and NEVER
// emits `mindspec complete <bead>` — a non-bead panel is not
// complete-gated, so that instruction would be actively wrong here.
//
// The recovery line embeds a copyable `mindspec panel create ...` command,
// so target and gate — both read RAW from a repo-write-attacker-poisonable
// panel.json — go through escapeConfigValue (kills a control byte/embedded
// newline reaching stdout, spec-113-final G2 finding 2) THEN
// shellQuoteTarget (single-quotes the result so a shell metacharacter like
// `;`/`$`/a backtick can never execute if the printed line is copied into a
// shell, finding 1). When target is empty (the unreadable-registration
// tally edge, F3-2), the `--target` flag is OMITTED entirely rather than
// rendering a dangling `--target ` with nothing after it. gate (F3-1) is
// included only when the panel recorded one, so the re-panel recovery
// preserves that gate's creation-time defaults.
func tallyExitActionNonBead(d panel.Decision, slug, target, gate string) error {
	switch d.Action {
	case panel.Warn:
		fmt.Fprintf(tallyWarnOut, "panel advisory: %s\n", d.Message)
		return nil
	case panel.Block:
		var cmd strings.Builder
		fmt.Fprintf(&cmd, "mindspec panel create %s --round <N+1> --spec <id>", slug)
		if target != "" {
			fmt.Fprintf(&cmd, " --target %s", shellQuoteTarget(escapeConfigValue(target)))
		}
		if gate != "" {
			fmt.Fprintf(&cmd, " --gate %s", shellQuoteTarget(escapeConfigValue(gate)))
		}
		return guard.NewFailure(d.Message, "re-run the panel: "+cmd.String())
	default:
		return nil
	}
}

// shellQuoteTarget renders s as a POSIX-shell single-quoted literal for
// embedding in a copyable `mindspec panel create ... --target/--gate <s>`
// recovery command (spec-113-final G2, finding 1): s is wrapped in single
// quotes with any embedded single quote escaped as `'\''`, so a shell
// metacharacter in s (`;`, `$`, a backtick, ...) can never execute when the
// printed recovery line is copied into a shell. s is ALWAYS quoted, even a
// "safe" value with no metacharacters, for one predictable rendering —
// mirrors internal/otel/config.go's own shellQuote (a different package;
// this trivial helper is duplicated locally rather than imported across an
// unrelated dependency boundary).
func shellQuoteTarget(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// sanitizeNonBeadDecision rewrites d's Message for a NON-BEAD panel (Bead 1
// Half 2, R1). panel.PanelGateDecision's shared templates are keyed by
// f.BeadID; a non-bead panel's BeadID is always "" (Bead 1 Half 1 feeds it
// the recorded target instead of a doomed "bead/"+beadID rev-parse), which
// produces malformed empty-interpolation fragments in three of its
// templates: the missing-ref leg's `references branch bead/,`, the
// transient-git-error leg's `panel for : ` / `could not verify branch
// bead/`, and the trailing RawMergeFence("") (`git merge bead/ ` with
// nothing to name) appended by legs (2)/(4)/(6)/(8)/(9)/(10). This
// function renames all three to the panel's recorded slug/target and NEVER
// introduces a `mindspec complete` instruction — a non-bead panel is not
// complete-gated.
//
// It NEVER touches d.Action, and it never mutates the shared
// panel.Decision internal/instruct/internal/complete read for BEAD panels
// — those callers never reach this function (it is applied only in this
// package's render layer, gated strictly on non-bead), so the shared
// Decision.Message stays byte-identical for every bead panel (the
// TestPanelVerbs_DecisionIsPanelGateDecision /
// TestPanelTally_ExitCodeTracksDecision pins, and the AC-global
// internal/instruct + internal/complete unmodified-test fence).
//
// target is rendered here TWICE with two different disciplines (spec-113-
// final G2, both findings): as plain display text ("target %s no longer
// exists") it is escapeConfigValue'd so a control byte/embedded newline in
// a hand-edited panel.json can never forge extra output lines (finding 2);
// inside the parenthetical copyable `mindspec panel create ... --target
// <target>` advisory it is ADDITIONALLY shell single-quoted
// (shellQuoteTarget) so a shell metacharacter (`;`, `$`, backtick, ...) in
// target can never execute if that line is copied into a shell (finding
// 1). Leg (5) is only reachable with a non-empty target (an empty target
// never rev-parses to a real gitutil.ErrRefNotFound — see
// nonBeadTargetRevParse), so no dangling/empty `--target` case arises
// here; that edge belongs to tallyExitActionNonBead (F3-2).
//
// gitErr is facts.GitErr (leg (5b) only): internal/panel's shared
// PanelGateDecision template interpolates f.GitErr RAW (%v) into
// d.Message BEFORE this function ever runs. A hostile target containing a
// NUL byte defeats gitutil.RevParseRef's ErrRefNotFound sentinel match (a
// non-*exec.ExitError wrap), routing the panel to this leg (5b) INSTEAD
// of the escaped leg (5) missing-ref path above — and the wrapped error
// string ("rev-parse <target>: <err>") re-embeds the target's raw control
// bytes (spec-113-final S2, empirically confirmed: a captured stderr held
// a raw NUL, a raw ESC/ANSI sequence, and a bare newline splitting the
// advisory into a forged extra line). Rather than patch substrings of the
// already-interpolated message (fragile if gitErr.Error() ever collided
// with template text), leg (5b) rebuilds its whole sentence from scratch
// with gitErr.Error() run through escapeConfigValue — never raw.
func sanitizeNonBeadDecision(d panel.Decision, slug, target string, gitErr error) panel.Decision {
	msg := d.Message
	if msg == "" {
		return d
	}

	// Legs (2)/(4)/(6)/(8)/(9)/(10): strip the trailing empty-bead fence.
	// A non-bead panel is not complete-gated and must not carry
	// `git merge bead/ ` advice with an empty interpolation.
	if fence := panel.RawMergeFence(""); strings.HasSuffix(msg, fence) {
		msg = strings.TrimSuffix(msg, fence)
	}

	displayTarget := escapeConfigValue(target)

	// Leg (5) missing-ref: the whole sentence is malformed when BeadID is
	// "" ("panel for  references branch bead/, which no longer exists —
	// ..."); replace it wholesale with a target-naming missing-ref
	// advisory (the Warn Action is preserved — only the Message changes).
	if strings.Contains(msg, "references branch bead/,") {
		msg = fmt.Sprintf(
			"panel %s target %s no longer exists — the reviewed ref was deleted; "+
				"re-create the panel against a live ref (mindspec panel create %s --spec <id> --target %s)",
			slug, displayTarget, slug, shellQuoteTarget(displayTarget))
	}

	// Leg (5b) transient git error: the shared template's message is
	// "panel for : could not verify branch bead/ (transient git error:
	// <RAW gitErr>) — staleness check skipped; ..." (BeadID == ""). Detect
	// it by its stable "could not verify branch bead/" fragment and
	// rebuild the ENTIRE sentence — renaming the empty bead/ fragments to
	// the recorded slug/target AND re-interpolating gitErr through
	// escapeConfigValue instead of carrying forward its already-raw %v
	// text (spec-113-final S2).
	if gitErr != nil && strings.Contains(msg, "could not verify branch bead/") {
		msg = fmt.Sprintf(
			"panel %s: could not verify target %s (transient git error: %s) — staleness check skipped; "+
				"proceeding per the gate's fail-open posture, but this is NOT a confirmed merge",
			slug, displayTarget, escapeConfigValue(gitErr.Error()))
	}

	d.Message = msg
	return d
}
