package main

import (
	"os"
	"time"

	"github.com/mindspec/mindspec/internal/trace"
	"github.com/spf13/cobra"
)

var cmdStartTime time.Time

var rootCmd = &cobra.Command{
	Use:   "mindspec",
	Short: "MindSpec: Spec-Driven Development and Self-Documentation System",
	Long:  `MindSpec is a spec-driven development + context management framework.`,
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

func init() {
	rootCmd.PersistentFlags().String("trace", "", "Write trace events to file (use - for stderr)")

	rootCmd.AddCommand(benchCmd)
	rootCmd.AddCommand(adrCmd)
	rootCmd.AddCommand(approveCmd)
	rootCmd.AddCommand(beadCmd)
	rootCmd.AddCommand(completeCmd)
	rootCmd.AddCommand(contextCmd)
	rootCmd.AddCommand(domainCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(glossaryCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(instructCmd)
	rootCmd.AddCommand(nextCmd)
	rootCmd.AddCommand(specInitCmd)
	rootCmd.AddCommand(stateCmd)
	rootCmd.AddCommand(traceCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(vizCmd)
}
