package main

import (
	"encoding/json"
	"fmt"
	"os"

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
	Long:  `Print the contents of .mindspec/state.json.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}

		root, err := workspace.FindRoot(cwd)
		if err != nil {
			return err
		}

		s, err := state.Read(root)
		if err != nil {
			return err
		}

		data, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			return fmt.Errorf("formatting state: %w", err)
		}

		fmt.Println(string(data))
		return nil
	},
}

func init() {
	stateSetCmd.Flags().String("mode", "", "Mode to set (idle, spec, plan, implement)")
	stateSetCmd.Flags().String("spec", "", "Active spec ID (required for spec, plan, implement modes)")
	stateSetCmd.Flags().String("bead", "", "Active bead ID (required for implement mode)")

	stateCmd.AddCommand(stateSetCmd)
	stateCmd.AddCommand(stateShowCmd)
}
