package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mindspec/mindspec/internal/resolve"
	"github.com/mindspec/mindspec/internal/state"
	"github.com/mindspec/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "Manage MindSpec workflow state",
	Long:  `Read and write the mode-cache that tracks current mode and active work.`,
}

var stateSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set the current mode and active work",
	Long:  `Update the mode-cache with the current mode, active spec, and active bead.`,
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

		mc := &state.ModeCache{
			Mode:       mode,
			ActiveSpec: spec,
			ActiveBead: bead,
		}
		if spec != "" {
			mc.SpecBranch = state.SpecBranch(spec)
			mc.ActiveWorktree = state.SpecWorktreePath(root, spec)
		}
		if err := state.WriteModeCache(root, mc); err != nil {
			return err
		}

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
			// Fall back to mode-cache
			mc, err := state.ReadModeCache(root)
			if err != nil {
				return fmt.Errorf("no active state: %w", err)
			}
			data, _ := json.MarshalIndent(mc, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		// Derive mode from molecule
		mode, modeErr := resolve.ResolveMode(root, specID)

		mc, _ := state.ReadModeCache(root)
		if mc == nil {
			mc = &state.ModeCache{}
		}
		mc.ActiveSpec = specID
		mc.SpecBranch = state.SpecBranch(specID)
		if modeErr == nil {
			mc.Mode = mode
		}

		data, err := json.MarshalIndent(mc, "", "  ")
		if err != nil {
			return fmt.Errorf("formatting state: %w", err)
		}
		fmt.Println(string(data))
		return nil
	},
}

var stateWriteSessionCmd = &cobra.Command{
	Use:   "write-session",
	Short: "Record session freshness metadata from SessionStart hook",
	Long:  `Writes sessionSource and sessionStartedAt to session.json. Called by the SessionStart hook with the source field from stdin JSON.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		source, _ := cmd.Flags().GetString("source")
		if source == "" {
			source = "unknown"
		}

		root, err := findRoot()
		if err != nil {
			return err
		}

		sess := &state.Session{
			SessionSource:    source,
			SessionStartedAt: time.Now().UTC().Format(time.RFC3339),
		}

		// On compact, preserve BeadClaimedAt so the freshness gate
		// still blocks mindspec next — compact is NOT a fresh context.
		if source == "compact" {
			if prev, readErr := state.ReadSession(root); readErr == nil {
				sess.BeadClaimedAt = prev.BeadClaimedAt
			}
		}

		if err := state.WriteSessionFile(root, sess); err != nil {
			return fmt.Errorf("writing session metadata: %w", err)
		}

		fmt.Printf("Session recorded: source=%s\n", source)
		return nil
	},
}

func init() {
	stateSetCmd.Flags().String("mode", "", "Mode to set (idle, spec, plan, implement)")
	stateSetCmd.Flags().String("spec", "", "Active spec ID (required for spec, plan, implement modes)")
	stateSetCmd.Flags().String("bead", "", "Active bead ID (required for implement mode)")

	stateShowCmd.Flags().String("spec", "", "Target spec ID (auto-detected if exactly one active spec)")

	stateWriteSessionCmd.Flags().String("source", "", "Session source from SessionStart hook (startup, clear, resume, compact)")

	stateCmd.AddCommand(stateSetCmd)
	stateCmd.AddCommand(stateShowCmd)
	stateCmd.AddCommand(stateWriteSessionCmd)
}
