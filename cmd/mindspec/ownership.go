package main

import (
	"fmt"

	"github.com/mrmaxsteel/mindspec/internal/ownership"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/spf13/cobra"
)

// `mindspec ownership populate` is one of the exactly two new
// top-level subcommands spec 091 introduces (Req 19; the other is
// `mindspec source populate`). It is a ZFC-compliant prompt emitter:
// the framework prints the prompt, the resident coding agent does the
// cognitive work of choosing paths. It never writes files.

var ownershipCmd = &cobra.Command{
	Use:   "ownership",
	Short: "Manage per-domain OWNERSHIP.yaml manifests",
}

var ownershipPopulateCmd = &cobra.Command{
	Use:   "populate [<domain>]",
	Short: "Print an agent prompt for populating OWNERSHIP.yaml paths",
	Long: `Prints a templated agent prompt instructing the resident coding agent
to inspect the repo and propose paths: entries for a domain's
OWNERSHIP.yaml. The framework itself proposes no paths (ZFC).

With no argument, one prompt is emitted per domain whose manifest is
missing or an empty stub (paths: []). With an explicit <domain>
argument the prompt is emitted regardless of the manifest's populated
state — useful for widening an existing paths: list.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			// Explicit-arg form: emit regardless of populated state
			// (Req 10; the Req 16 widen-hint relies on this).
			if err := validate.DomainName(args[0]); err != nil {
				return err
			}
			fmt.Println(ownership.BuildPopulatePrompt(args[0]))
			return nil
		}

		// No-arg form: one prompt per missing-or-empty manifest.
		root, err := findRoot()
		if err != nil {
			return err
		}
		domains, err := ownership.DomainsNeedingPopulate(root)
		if err != nil {
			return err
		}
		if len(domains) == 0 {
			fmt.Println("All domain OWNERSHIP.yaml manifests are populated (or no domains exist).")
			fmt.Println("To re-emit the prompt for a specific domain, run: mindspec ownership populate <domain>")
			return nil
		}
		for i, d := range domains {
			if i > 0 {
				fmt.Println("---")
			}
			fmt.Println(ownership.BuildPopulatePrompt(d))
		}
		return nil
	},
}

func init() {
	ownershipCmd.AddCommand(ownershipPopulateCmd)
	// Registered here rather than in root.go so spec 091 Bead 3's CLI
	// surface lands entirely in this file (the bead's files-in-scope
	// boundary); cobra supports registration from any init().
	rootCmd.AddCommand(ownershipCmd)
}
