package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mrmaxsteel/mindspec/internal/specinit"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var specInitCmd = &cobra.Command{
	Use:    "spec-init [spec-id]",
	Short:  "Initialize a new specification and enter Spec Mode",
	Long:   `Alias for 'mindspec spec create'. Use 'mindspec spec create' instead.`,
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		specID := args[0]
		title, _ := cmd.Flags().GetString("title")

		root, err := findRoot()
		if err != nil {
			return err
		}

		result, err := specinit.Run(root, specID, title)
		if err != nil {
			return err
		}

		specPath := filepath.Join(workspace.SpecDir(root, specID), "spec.md")
		relPath, err := filepath.Rel(root, specPath)
		if err != nil {
			relPath = specPath
		}
		fmt.Printf("Spec initialized: %s\n", filepath.ToSlash(relPath))

		if result.WorktreePath != "" {
			fmt.Printf("Worktree: %s (branch: %s)\n", result.WorktreePath, result.SpecBranch)
			fmt.Printf("\n  cd %s\n\n", result.WorktreePath)
		} else {
			fmt.Println()
		}

		if err := emitInstruct(root); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not emit guidance: %v\n", err)
		}

		return nil
	},
}

func init() {
	specInitCmd.Flags().String("title", "", "Spec title (derived from slug if omitted)")
}
