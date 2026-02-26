package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mindspec/mindspec/internal/resolve"
	"github.com/mindspec/mindspec/internal/state"
	"github.com/mindspec/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "Manage MindSpec workflow state",
	Long:  `Read and write the .mindspec/state.json file that tracks current mode and active work.`,
}

var stateSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set the current mode and active work",
	Long:  `Update .mindspec/state.json with the current mode, active spec, and active bead.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		mode, _ := cmd.Flags().GetString("mode")
		spec, _ := cmd.Flags().GetString("spec")
		bead, _ := cmd.Flags().GetString("bead")

		if mode == "" {
			return fmt.Errorf("--mode is required")
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}

		root, err := workspace.FindRoot(cwd)
		if err != nil {
			return err
		}

		if err := state.SetMode(root, mode, spec, bead); err != nil {
			return err
		}

		// Note: parent status propagation handled natively by beads molecules

		fmt.Printf("State updated: mode=%s", mode)
		if spec != "" {
			fmt.Printf(", spec=%s", spec)
		}
		if bead != "" {
			fmt.Printf(", bead=%s", bead)
		}
		fmt.Println()

		return nil
	},
}

var stateShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the current MindSpec state",
	Long: `Show the current MindSpec state. By default shows state derived from
the molecule (ADR-0015). Use --spec to target a specific spec.
If multiple active specs exist and no --spec is given, shows the ambiguity.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		specFlag, _ := cmd.Flags().GetString("spec")

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}

		root, err := workspace.FindRoot(cwd)
		if err != nil {
			return err
		}

		// Try resolver-based state derivation
		specID, resolveErr := resolve.ResolveTarget(root, specFlag)
		if resolveErr != nil {
			if ambErr, ok := resolveErr.(*resolve.ErrAmbiguousTarget); ok {
				return ambErr
			}
			// Fall back to raw state.json
			s, err := state.Read(root)
			if err != nil {
				return err
			}
			data, _ := json.MarshalIndent(s, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		// Derive mode from molecule
		mode, modeErr := resolve.ResolveMode(root, specID)

		// Read base state for bead info
		s, _ := state.Read(root)
		if s == nil {
			s = &state.State{}
		}
		s.ActiveSpec = specID
		if modeErr == nil {
			s.Mode = mode
		}

		data, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			return fmt.Errorf("formatting state: %w", err)
		}
		fmt.Println(string(data))
		return nil
	},
}

var stateClearFlagCmd = &cobra.Command{
	Use:   "clear-flag",
	Short: "Clear the needs_clear flag after a context reset",
	Long:  `Clears the needs_clear flag in state.json. Called by the SessionStart hook after /clear.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findRoot()
		if err != nil {
			return err
		}

		if err := state.ClearNeedsClear(root); err != nil {
			return fmt.Errorf("clearing needs_clear flag: %w", err)
		}

		fmt.Println("needs_clear flag cleared.")
		return nil
	},
}

func init() {
	stateSetCmd.Flags().String("mode", "", "Mode to set (idle, spec, plan, implement)")
	stateSetCmd.Flags().String("spec", "", "Active spec ID (required for spec, plan, implement modes)")
	stateSetCmd.Flags().String("bead", "", "Active bead ID (required for implement mode)")

	stateShowCmd.Flags().String("spec", "", "Target spec ID (auto-detected if exactly one active spec)")

	stateCmd.AddCommand(stateSetCmd)
	stateCmd.AddCommand(stateShowCmd)
	stateCmd.AddCommand(stateClearFlagCmd)
}
