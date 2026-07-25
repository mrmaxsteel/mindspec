package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mrmaxsteel/mindspec/internal/approve"
	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/spec"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/mrmaxsteel/mindspec/internal/workspace/containment"
	"github.com/spf13/cobra"
)

var specCmd = &cobra.Command{
	Use:   "spec",
	Short: "Spec lifecycle commands",
	// Spec 092 Req 10b: typos of the deprecated `approve` verb suggest
	// the noun-verb command families.
	SuggestFor: []string{"approve", "aprove"},
}

var specCreateCmd = &cobra.Command{
	Use:   "create <slug> | <NNN>-<slug>",
	Short: "Create a new specification and enter Spec Mode",
	Long: `Creates a new spec directory with spec.md from the template,
creates a branch and worktree, sets state to spec mode, and emits guidance.

Accepts either a bare kebab-case slug (e.g. "seam-x"), which is
auto-numbered to <NNN>-<slug> as max(existing spec numbers)+1
(zero-padded to 3 digits), or an explicit <NNN>-<slug> spec ID
(e.g. "127-seam-x"), which is used verbatim.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title, _ := cmd.Flags().GetString("title")

		root, err := findRoot()
		if err != nil {
			return err
		}

		specID, err := resolveSpecCreateID(root, args[0])
		if err != nil {
			return err
		}
		if specID != args[0] {
			fmt.Printf("Auto-numbered spec: %s\n", specID)
		}

		exec := newExecutor(root)
		result, err := spec.Run(root, specID, title, exec)
		if err != nil {
			return err
		}

		specDir, err := workspace.SpecDir(root, specID)
		if err != nil {
			return err
		}
		specPath := filepath.Join(specDir, "spec.md")
		relPath, err := filepath.Rel(root, specPath)
		if err != nil {
			relPath = specPath
		}
		fmt.Printf("Spec initialized: %s\n", filepath.ToSlash(relPath))

		if result.WorktreePath != "" {
			fmt.Printf("Worktree: %s (branch: %s)\n", result.WorktreePath, result.SpecBranch)
			fmt.Printf("\n  %s\n\n", containment.EmitCd(result.WorktreePath))
		} else {
			fmt.Println()
		}

		if err := emitInstruct(root); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not emit guidance: %v\n", err)
		}

		return nil
	},
}

// resolveSpecCreateID resolves the `spec create` positional argument to a
// full <NNN>-<slug> spec ID (GH #219). An argument that already validates
// as a spec ID (validate.SpecID) is returned verbatim. Otherwise it is
// treated as a bare slug and auto-numbered: max(existing spec numbers)+1,
// zero-padded to 3 digits (the on-disk convention). If the auto-numbered
// candidate STILL fails validation, the argument was not a well-formed
// kebab-case slug either, and the original validation error is returned
// with a hint naming both accepted forms — the auto-numbered candidate is
// never used unvalidated, so nothing reaches workspace.SpecDir /
// filepath.Join that idvalidate.SpecID has not passed (SEC-1).
func resolveSpecCreateID(root, arg string) (string, error) {
	argErr := validate.SpecID(arg)
	if argErr == nil {
		return arg, nil
	}
	candidate := fmt.Sprintf("%03d-%s", nextSpecNumber(root), arg)
	if err := validate.SpecID(candidate); err != nil {
		return "", fmt.Errorf("%w (pass a bare kebab-case slug to auto-number, e.g. `mindspec spec create seam-x`, or an explicit <NNN>-<slug>)", argErr)
	}
	return candidate, nil
}

// nextSpecNumber returns max(existing spec numbers)+1 for auto-numbering
// (GH #219): each entry under the SpecsDir enumeration root — AND under
// spec dirs inside existing worktrees beneath the default worktrees root
// (a pre-epic spec lives ONLY on its branch's worktree until it lands on
// main, the same on-disk shape GH #222 fixed for panel lookup) — has its
// leading digit run parsed; unreadable dirs are skipped best-effort.
// Each enumerated name is validate-and-skipped through idvalidate.SpecID
// before its number is parsed (the internal/spec.List root-enumeration
// gate pattern, ADR-0042/spec 120 scan (f)) — a non-spec dir name never
// influences the allocation. Gaps are NOT reused (max+1, never
// first-free): a retired spec's number stays retired. An empty workspace
// allocates 1.
func nextSpecNumber(root string) int {
	maxN := 0
	scan := func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if idvalidate.SpecID(name) != nil {
				continue
			}
			i := 0
			for i < len(name) && name[i] >= '0' && name[i] <= '9' {
				i++
			}
			if i == 0 {
				continue
			}
			n, err := strconv.Atoi(name[:i])
			if err != nil {
				continue
			}
			if n > maxN {
				maxN = n
			}
		}
	}
	scan(workspace.SpecsDir(root))
	wtRoot := workspace.DefaultWorktreesDir(root)
	wtEntries, err := os.ReadDir(wtRoot)
	if err != nil {
		return maxN + 1
	}
	for _, wt := range wtEntries {
		if !wt.IsDir() {
			continue
		}
		scan(filepath.Join(wtRoot, wt.Name(), ".mindspec", "specs"))
		scan(filepath.Join(wtRoot, wt.Name(), ".mindspec", "docs", "specs"))
	}
	return maxN + 1
}

var specApproveCmd = &cobra.Command{
	Use:   "approve <id>",
	Short: "Approve a spec and transition to Plan Mode",
	Long: `Validates the spec, updates the Approval section to APPROVED,
creates the spec bead and gate (if not already present),
resolves the spec gate in Beads, generates the context pack,
sets state to plan mode, and emits plan mode guidance.`,
	Args: cobra.ExactArgs(1),
	RunE: approveSpecRunE,
}

func init() {
	specCreateCmd.Flags().String("title", "", "Spec title (derived from slug if omitted)")
	specApproveCmd.Flags().String("approved-by", "user", "Identity of the approver")
	specCmd.AddCommand(specCreateCmd)
	specCmd.AddCommand(specApproveCmd)
}

// approveSpecRunE is shared between `spec approve` and `approve spec`.
func approveSpecRunE(cmd *cobra.Command, args []string) error {
	specID := args[0]
	if err := validate.SpecID(specID); err != nil {
		return err
	}
	approvedBy, _ := cmd.Flags().GetString("approved-by")

	root, err := findRoot()
	if err != nil {
		return err
	}

	if err := bead.Preflight(root); err != nil {
		fmt.Fprintf(os.Stderr, "warning: Beads preflight failed: %v (bead creation and gate resolution may fail)\n", err)
	}

	exec := newExecutor(root)
	result, err := approve.ApproveSpec(root, specID, approvedBy, exec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Spec %s approved.\n", specID)
	for _, w := range result.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Commit approval artifacts before continuing (required for clean-tree gates).")
	fmt.Printf("  2. Continue planning for %s.\n", specID)
	fmt.Println()

	if err := emitInstruct(root); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not emit guidance: %v\n", err)
	}

	return nil
}
