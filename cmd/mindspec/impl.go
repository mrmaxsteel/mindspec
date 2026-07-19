package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/approve"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/mrmaxsteel/mindspec/internal/workspace/containment"
	"github.com/spf13/cobra"
)

var implCmd = &cobra.Command{
	Use:   "impl",
	Short: "Implementation lifecycle commands",
	// Spec 092 Req 10b: typos of the deprecated `approve` verb suggest
	// the noun-verb command families.
	SuggestFor: []string{"approve", "aprove"},
}

var implApproveCmd = &cobra.Command{
	Use:   "approve <id>",
	Short: "Approve implementation and transition to idle",
	Long: `Verifies review mode is active for the given spec,
pushes the spec branch to remote (if available), cleans up
worktrees and branches locally, and transitions state to idle.
This is the final human gate in the spec lifecycle.`,
	Args: cobra.ExactArgs(1),
	RunE: approveImplRunE,
}

func init() {
	implApproveCmd.Flags().String("allow-doc-skew", "", "Override the doc-sync gate with a recorded reason (records reason+by+at on spec epic metadata)")
	implApproveCmd.Flags().String("override-adr", "", "Override the ADR-divergence gate with a recorded reason (records mindspec_adr_override_* on spec epic metadata)")
	implApproveCmd.Flags().String("supersede-adr", "", "Pre-create a placeholder ADR (Status: Proposed) at the supplied ID and bypass the divergence gate (records mindspec_adr_supersede_* on spec epic metadata)")
	implCmd.AddCommand(implApproveCmd)
}

// approveImplRunE is shared between `impl approve` and `approve impl`.
func approveImplRunE(cmd *cobra.Command, args []string) error {
	specID := args[0]

	// R3 explicit-ingress early gate (ADR-0042, AC-7): a hostile args[0]
	// refuses HERE, before any SpecWorktreePath composition or os.Chdir —
	// not deep in composition where the value might already have been
	// used to probe the filesystem.
	if err := idvalidate.SpecID(specID); err != nil {
		return guard.NewFailure(
			fmt.Sprintf("%s is not a valid spec ID: %v", termsafe.Escape(specID), err),
			"mindspec spec list   (pick a listed spec ID and re-run)",
		)
	}

	// Spec 092 Req 4 (mindspec-qxsy): capture the shell's invocation
	// directory BEFORE any auto-chdir. FinalizeEpic removes the spec
	// worktree, so if the shell sat inside it the cd-back NOTE below is
	// the only way to tell the agent how to recover its cwd.
	invocationCwd := captureInvocationCwd()

	root, err := findRoot()
	if err != nil {
		return err
	}

	// Auto-cd into the spec worktree so phase resolution finds the correct
	// context. Without this, running from main fails because DiscoverActiveSpecs
	// doesn't find closed epics (review mode).
	cfg, cfgErr := config.Load(root)
	if cfgErr != nil {
		cfg = config.DefaultConfig()
	}
	// specID already validated above; this waist call cannot fail.
	specWtPath, _ := workspace.SpecWorktreePath(root, cfg, specID)
	if info, err := os.Stat(specWtPath); err == nil && info.IsDir() {
		// R5 check-at-use (ADR-0042 §4, AC-11): re-validate containment of
		// the composed spec-worktree path immediately before the auto-cd.
		// This site already tolerates a chdir failure silently (best-
		// effort convenience, not the primary gate), so a containment
		// failure is likewise a skip-and-warn, not a hard command failure.
		if ctErr := containment.CheckContainment(root, specWtPath); ctErr != nil {
			fmt.Fprintf(os.Stderr, "warning: refusing auto-cd into spec worktree: %v\n", ctErr)
		} else {
			_ = os.Chdir(specWtPath)
		}
	}

	// Spec 086 Bead 3: --allow-doc-skew override flag (shared between
	// `mindspec impl approve` and `mindspec approve impl`). Explicit
	// empty reason rejected per spec Req 12.
	allowDocSkew, _ := cmd.Flags().GetString("allow-doc-skew")
	if cmd.Flags().Changed("allow-doc-skew") && strings.TrimSpace(allowDocSkew) == "" {
		return fmt.Errorf("--allow-doc-skew requires a non-empty reason")
	}

	// Spec 087 Bead 3: --override-adr / --supersede-adr override
	// flags (shared with `mindspec approve impl`). Same discipline
	// as --allow-doc-skew. Mutually exclusive — distinct audit
	// namespaces per spec.md Requirement 13.
	overrideADR, _ := cmd.Flags().GetString("override-adr")
	if cmd.Flags().Changed("override-adr") && strings.TrimSpace(overrideADR) == "" {
		return fmt.Errorf("--override-adr requires a non-empty reason")
	}
	supersedeADR, _ := cmd.Flags().GetString("supersede-adr")
	if cmd.Flags().Changed("supersede-adr") {
		if err := idvalidate.ADRID(supersedeADR); err != nil {
			return fmt.Errorf("--supersede-adr: %w", err)
		}
	}
	if cmd.Flags().Changed("override-adr") && cmd.Flags().Changed("supersede-adr") {
		return fmt.Errorf("--override-adr and --supersede-adr are mutually exclusive")
	}

	exec := newExecutor(root)
	result, approveErr := approve.ApproveImpl(root, specID, exec, approve.ImplOpts{
		AllowDocSkew: allowDocSkew,
		OverrideADR:  overrideADR,
		SupersedeADR: supersedeADR,
	})

	if tailErr := implApproveTail(os.Stdout, os.Stderr, root, invocationCwd, specID, cfg, result, approveErr, emitInstruct); tailErr != nil {
		os.Exit(1)
	}
	return nil
}

// implApproveTail is the production tail of `impl approve` — everything
// after ApproveImpl returns. Extracted (spec 092 panel R3-1) so unit
// tests exercise the REAL wiring and ordering instead of a simulation:
//
//  1. Spec 092 Req 3b (mindspec-qxsy): chdir to the repo root
//     immediately — FinalizeEpic (inside ApproveImpl) removes the spec
//     worktree this command auto-chdir'd into — so all tail output and
//     the bd subprocesses instructFn spawns run from a valid cwd.
//  2. On ApproveImpl failure (panel R2-3): FinalizeEpic can fail AFTER
//     removing the spec worktree, so the error path still emits the
//     Req 4 cd-back NOTE — as the LAST line of stderr. Channel choice,
//     stated once: stdout carries success output only, so the
//     error-path NOTE goes to stderr; on success the NOTE is the LAST
//     line of stdout.
//  3. On success: summary, instruct tail, then the Req 4 cd-back NOTE
//     as the LAST line of stdout.
//
// Returns approveErr unchanged so the caller owns the exit code (HC-4).
func implApproveTail(stdout, stderr io.Writer, root, invocationCwd, specID string, cfg *config.Config, result *approve.ImplResult, approveErr error, instructFn func(string) error) error {
	if chdirErr := os.Chdir(root); chdirErr != nil {
		fmt.Fprintf(stderr, "warning: could not chdir to repo root %s: %v\n", root, chdirErr)
	}

	if approveErr != nil {
		fmt.Fprintf(stderr, "error: %v\n", approveErr)
		emitCdBackNote(stderr, invocationCwd, root)
		return approveErr
	}

	fmt.Fprintf(stdout, "Implementation for %s approved. Mode: idle.\n", result.SpecID)
	for _, w := range result.Warnings {
		fmt.Fprintf(stderr, "warning: %s\n", w)
	}

	if result.SpecBranch != "" {
		fmt.Fprintln(stdout)
		fmt.Fprintf(stdout, "Summary:\n")
		fmt.Fprintf(stdout, "  Branch:   %s\n", result.SpecBranch)
		if result.CommitCount > 0 {
			fmt.Fprintf(stdout, "  Commits:  %d\n", result.CommitCount)
		}
		if result.DiffStat != "" {
			fmt.Fprintf(stdout, "\n%s\n", result.DiffStat)
		}
		// Bug wu7t panel round 1 (Group 2): the two PR instructions are
		// mutually exclusive. FinalizeBranch set means the spec branch
		// was ALREADY merged — telling the operator to open a spec-branch
		// PR would point them at a dead carrier, contradicting the NOTE
		// below. So the spec-branch instruction prints only on the
		// normal (FinalizeBranch == "") path.
		if result.Pushed && result.FinalizeBranch == "" {
			fmt.Fprintf(stdout, "\nBranch pushed to remote. Create a PR to merge into main:\n")
			fmt.Fprintf(stdout, "  gh pr create --head %s --base main --title \"[SPEC %s] <title>\" --body \"<description>\"\n", result.SpecBranch, specID)
		}
		if result.FinalizeBranch != "" {
			// Bug wu7t: %s (result.SpecBranch) was already merged into main
			// before this ran, so the epic-close JSONL export commit could
			// not ride it — it landed instead on a fresh from-main branch,
			// already pushed. Until a PR from that branch merges, main's
			// committed .beads/issues.jsonl is stale and the bd post-merge
			// hook will keep reverting the epic-close/bead-done state in
			// Dolt on every subsequent merge/FF.
			fmt.Fprintf(stdout, "\nNOTE: %s was already merged into main, so the epic-close JSONL export landed on a separate branch instead: %s (already pushed).\n", result.SpecBranch, result.FinalizeBranch)
			fmt.Fprintf(stdout, "Open and merge a PR from %s into main:\n", result.FinalizeBranch)
			fmt.Fprintf(stdout, "  gh pr create --head %s --base main --title \"chore(beads): finalize epic for spec %s\" --body \"<description>\"\n", result.FinalizeBranch, specID)
			fmt.Fprintf(stdout, "Until that PR merges, main's committed .beads/issues.jsonl is STALE: the bd post-merge hook will keep reverting the epic-close/bead-done state in Dolt on every subsequent merge/FF.\n")

			// Spec 121 R1-R3 (mindspec-uxl4, ADR-0041 §4): automate the
			// manual dance the NOTE above just described — auto-open (and,
			// opt-in, auto-merge) the finalize PR. Strictly AFTER the
			// finalize mutation chain above has durably completed; every
			// failure degrades to the NOTE already printed, with exit 0.
			runFinalizePRAutomation(stdout, stderr, cfg, specID, result.EpicID, result.FinalizeBranch)
		}
	}
	fmt.Fprintln(stdout)

	if err := instructFn(root); err != nil {
		fmt.Fprintf(stderr, "warning: could not emit guidance: %v\n", err)
	}

	// Spec 092 Req 4: when the shell's invocation directory was removed
	// by the terminal mutation, the cd-back NOTE is the LAST line of
	// stdout.
	emitCdBackNote(stdout, invocationCwd, root)
	return nil
}
