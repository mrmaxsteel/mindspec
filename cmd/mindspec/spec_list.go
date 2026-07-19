package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/spec"
	"github.com/spf13/cobra"
)

var specListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all specs with status and lifecycle phase",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findRoot()
		if err != nil {
			return err
		}

		specs, err := spec.List(root)
		if err != nil {
			return fmt.Errorf("listing specs: %w", err)
		}

		jsonFlag, _ := cmd.Flags().GetBool("json")
		if jsonFlag {
			data, err := json.MarshalIndent(specs, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling JSON: %w", err)
			}
			fmt.Println(string(data))
			return nil
		}

		if len(specs) == 0 {
			fmt.Println("No specs found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SPEC ID\tSTATUS\tPHASE")
		for _, s := range specs {
			// R4: s.SpecID is the unvalidated on-disk spec-dir basename
			// (internal/spec/list.go's e.Name()) — agent-writable, never
			// spine-validated before this render — so route it through
			// idrender.Spec (spec 120 R4).
			fmt.Fprintf(w, "%s\t%s\t%s\n", idrender.Spec(s.SpecID), s.Status, s.Phase)
		}
		return w.Flush()
	},
}

func init() {
	specListCmd.Flags().Bool("json", false, "Output as JSON")
	specCmd.AddCommand(specListCmd)
}
