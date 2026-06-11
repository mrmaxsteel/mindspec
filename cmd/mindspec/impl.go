package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/approve"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
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
	specWtPath := workspace.SpecWorktreePath(root, cfg, specID)
	if info, err := os.Stat(specWtPath); err == nil && info.IsDir() {
		_ = os.Chdir(specWtPath)
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
	result, err := approve.ApproveImpl(root, specID, exec, approve.ImplOpts{
		AllowDocSkew: allowDocSkew,
		OverrideADR:  overrideADR,
		SupersedeADR: supersedeADR,
	})

	// Spec 092 Req 3b (mindspec-qxsy): FinalizeEpic (inside ApproveImpl)
	// removes the spec worktree this command auto-chdir'd into above.
	// Move to the repo root immediately after it returns — before any
	// tail output and before emitInstruct — so the rest of the command
	// (and the bd subprocesses emitInstruct spawns) runs from a valid
	// cwd and the process exits 0.
	if chdirErr := os.Chdir(root); chdirErr != nil {
		fmt.Fprintf(os.Stderr, "warning: could not chdir to repo root %s: %v\n", root, chdirErr)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Implementation for %s approved. Mode: idle.\n", result.SpecID)
	for _, w := range result.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}

	if result.SpecBranch != "" {
		fmt.Println()
		fmt.Printf("Summary:\n")
		fmt.Printf("  Branch:   %s\n", result.SpecBranch)
		if result.CommitCount > 0 {
			fmt.Printf("  Commits:  %d\n", result.CommitCount)
		}
		if result.DiffStat != "" {
			fmt.Printf("\n%s\n", result.DiffStat)
		}
		if result.Pushed {
			fmt.Printf("\nBranch pushed to remote. Create a PR to merge into main:\n")
			fmt.Printf("  gh pr create --head %s --base main --title \"[SPEC %s] <title>\" --body \"<description>\"\n", result.SpecBranch, specID)
		}
	}
	fmt.Println()

	if err := emitInstruct(root); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not emit guidance: %v\n", err)
	}

	// Spec 092 Req 4: when the shell's invocation directory was removed
	// by the terminal mutation, the cd-back NOTE is the LAST line of
	// stdout.
	emitCdBackNote(os.Stdout, invocationCwd, root)

	return nil
}
