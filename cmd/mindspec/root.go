package main

import (
	"fmt"
	"os"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/trace"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

// newExecutor creates a GitExecutor rooted at the given path.
// Used as a factory function across CLI commands.
func newExecutor(root string) executor.Executor {
	return executor.NewGitExecutor(root)
}

// Set by goreleaser ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var cmdStartTime time.Time

var rootCmd = &cobra.Command{
	Use:     "mindspec",
	Short:   "MindSpec: Spec-Driven Development and Self-Documentation System",
	Long:    `MindSpec is a spec-driven development + context management framework.`,
	Version: version + " (" + commit + ") " + date,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		cmdStartTime = time.Now()

		// Resolve trace path: flag takes precedence, then env var.
		tracePath, _ := cmd.Flags().GetString("trace")
		if tracePath == "" {
			tracePath = os.Getenv("MINDSPEC_TRACE")
		}
		if tracePath != "" {
			if err := trace.Init(tracePath); err != nil {
				return err
			}
		}

		trace.Emit(trace.NewEvent("command.start").WithData(map[string]any{
			"command": cmd.Name(),
			"args":    args,
		}))
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		trace.Emit(trace.NewEvent("command.end").
			WithDuration(time.Since(cmdStartTime)).
			WithData(map[string]any{
				"command": cmd.Name(),
			}))
		return trace.Close()
	},
}

func findRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot determine working directory: %w", err)
	}
	root, err := workspace.FindRoot(cwd)
	if err != nil {
		return "", fmt.Errorf("workspace not found: %w", err)
	}
	return root, nil
}

func findLocalRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot determine working directory: %w", err)
	}
	root, err := workspace.FindLocalRoot(cwd)
	if err != nil {
		return "", fmt.Errorf("workspace not found: %w", err)
	}
	return root, nil
}

func init() {
	rootCmd.PersistentFlags().String("trace", "", "Write trace events to file (use - for stderr)")

	rootCmd.AddCommand(benchCmd)
	rootCmd.AddCommand(adrCmd)
	rootCmd.AddCommand(cleanupCmd)
	rootCmd.AddCommand(approveCmd) // hidden backward-compat alias
	rootCmd.AddCommand(beadCmd)
	rootCmd.AddCommand(completeCmd)
	rootCmd.AddCommand(contextCmd)
	rootCmd.AddCommand(domainCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(implCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(instructCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(nextCmd)
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(specCmd)
	rootCmd.AddCommand(specInitCmd) // hidden backward-compat alias
	rootCmd.AddCommand(hookCmd)
	rootCmd.AddCommand(stateCmd)
	rootCmd.AddCommand(traceCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(agentmindCmd)
	rootCmd.AddCommand(recordCmd)
}
