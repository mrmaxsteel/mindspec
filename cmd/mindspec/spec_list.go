package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/mindspec/mindspec/internal/speclist"
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

		specs, err := speclist.List(root)
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
			fmt.Fprintf(w, "%s\t%s\t%s\n", s.SpecID, s.Status, s.Phase)
		}
		return w.Flush()
	},
}

func init() {
	specListCmd.Flags().Bool("json", false, "Output as JSON")
	specCmd.AddCommand(specListCmd)
}
