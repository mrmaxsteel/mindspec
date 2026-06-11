package main

import (
	"fmt"
	"io"

	"github.com/mrmaxsteel/mindspec/internal/ownership"
	"github.com/spf13/cobra"
)

// `mindspec source populate` is one of the exactly two new top-level
// subcommands spec 091 introduces (Req 19; the other is `mindspec
// ownership populate`). Repo-wide, no domain argument: it prints the
// Req 12 agent prompt for declaring source_globs in
// .mindspec/config.yaml and nothing more (ZFC — no pre-filled glob
// values, no file writes).

var sourceCmd = &cobra.Command{
	Use:   "source",
	Short: "Manage doc-sync source classification",
}

var sourcePopulateCmd = &cobra.Command{
	Use:   "populate",
	Short: "Print an agent prompt for populating source_globs",
	Long: `Prints a templated agent prompt instructing the resident coding agent
to inspect the repo and propose source_globs: entries for
.mindspec/config.yaml. A non-empty source_globs list FULLY REPLACES
mindspec's built-in source classifier, so the proposed list must cover
everything the doc-sync gate should treat as source. The framework
itself proposes no globs (ZFC).`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSourcePopulate(cmd.OutOrStdout())
	},
}

// runSourcePopulate emits the Req 12 repo-wide source-globs prompt to
// w. Extracted from the RunE so the command's print behavior is
// unit-covered (panel R3-1).
func runSourcePopulate(w io.Writer) error {
	fmt.Fprintln(w, ownership.BuildSourcePopulatePrompt())
	return nil
}

func init() {
	sourceCmd.AddCommand(sourcePopulateCmd)
	// Registered here rather than in root.go so spec 091 Bead 3's CLI
	// surface lands entirely in this file (the bead's files-in-scope
	// boundary); cobra supports registration from any init().
	rootCmd.AddCommand(sourceCmd)
}
