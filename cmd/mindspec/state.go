package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/resolve"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "Manage MindSpec workflow state",
	Long:  `Read and write the focus cursor that tracks current mode and active work.`,
}

var stateSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set the current mode and active work (deprecated)",
	Long:  `Deprecated: lifecycle state is now derived from beads (ADR-0023). This command is a no-op.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("state set is deprecated: lifecycle state is now derived from beads (ADR-0023)")
		return nil
	},
}

// stateOutput is the JSON output format for state show.
type stateOutput struct {
	Mode       string `json:"mode"`
	ActiveSpec string `json:"activeSpec,omitempty"`
	ActiveBead string `json:"activeBead,omitempty"`
	SpecBranch string `json:"specBranch,omitempty"`
}

var stateShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the current MindSpec state",
	Long: `Show the current MindSpec state derived from beads.
Use --spec to target a specific spec.
If multiple active specs exist and no --spec is given, shows the ambiguity.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		specFlag, _ := cmd.Flags().GetString("spec")

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}

		root, err := workspace.FindLocalRoot(cwd)
		if err != nil {
			return err
		}

		// ADR-0023: derive state from beads, not focus files.
		specID, resolveErr := resolve.ResolveTarget(root, specFlag)
		if resolveErr != nil {
			if ambErr, ok := resolveErr.(*resolve.ErrAmbiguousTarget); ok {
				return ambErr
			}
			// Try phase context
			ctx, ctxErr := phase.ResolveContext(root)
			if ctxErr != nil || ctx == nil {
				return fmt.Errorf("no active state found")
			}
			out := &stateOutput{
				Mode:       ctx.Phase,
				ActiveSpec: ctx.SpecID,
				ActiveBead: ctx.BeadID,
			}
			data, _ := json.MarshalIndent(out, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		// Derive mode from beads
		mode, _ := resolve.ResolveMode(root, specID)
		ctx, _ := phase.ResolveContextFromDir(root, root)
		activeBead := ""
		if ctx != nil {
			activeBead = ctx.BeadID
		}

		out := &stateOutput{
			Mode:       mode,
			ActiveSpec: specID,
			ActiveBead: activeBead,
			SpecBranch: state.SpecBranch(specID),
		}

		data, err := json.MarshalIndent(out, "", "  ")
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
