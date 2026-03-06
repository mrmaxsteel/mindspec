package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var adrCmd = &cobra.Command{
	Use:   "adr",
	Short: "Manage Architecture Decision Records",
}

var adrCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a new ADR from template",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		root, err := workspace.FindRoot(cwd)
		if err != nil {
			return err
		}

		domain, _ := cmd.Flags().GetString("domain")
		supersedes, _ := cmd.Flags().GetString("supersedes")

		var domains []string
		if domain != "" {
			for _, d := range strings.Split(domain, ",") {
				d = strings.TrimSpace(d)
				if d != "" {
					domains = append(domains, d)
				}
			}
		}

		path, err := adr.Create(root, args[0], adr.CreateOpts{
			Domains:    domains,
			Supersedes: supersedes,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Created: %s\n", path)
		fmt.Println("Fill in the Context and Decision sections, then update Status to Accepted when ready.")
		if supersedes != "" {
			fmt.Printf("Updated %s with Superseded-by reference.\n", supersedes)
		}

		return nil
	},
}

var adrListCmd = &cobra.Command{
	Use:   "list",
	Short: "List ADRs with optional filters",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		root, err := workspace.FindRoot(cwd)
		if err != nil {
			return err
		}

		status, _ := cmd.Flags().GetString("status")
		domain, _ := cmd.Flags().GetString("domain")

		adrs, err := adr.List(root, adr.ListOpts{
			Status: status,
			Domain: domain,
		})
		if err != nil {
			return err
		}

		if len(adrs) == 0 {
			fmt.Println("No ADRs found.")
			return nil
		}

		fmt.Print(adr.FormatTable(adrs))
		fmt.Printf("\n%d ADR(s)\n", len(adrs))
		return nil
	},
}

var adrShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show a single ADR summary",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		root, err := workspace.FindRoot(cwd)
		if err != nil {
			return err
		}

		jsonFlag, _ := cmd.Flags().GetBool("json")

		a, err := adr.Show(root, args[0])
		if err != nil {
			return err
		}

		if jsonFlag {
			out, err := adr.FormatJSON(a)
			if err != nil {
				return err
			}
			fmt.Println(out)
		} else {
			fmt.Print(adr.FormatSummary(a))
		}
		return nil
	},
}

func init() {
	adrCreateCmd.Flags().String("domain", "", "Domain(s) for the ADR (comma-separated)")
	adrCreateCmd.Flags().String("supersedes", "", "ADR ID to supersede (e.g., ADR-0001)")

	adrListCmd.Flags().String("status", "", "Filter by status (e.g., accepted, proposed, superseded)")
	adrListCmd.Flags().String("domain", "", "Filter by domain")

	adrShowCmd.Flags().Bool("json", false, "Output as JSON")

	adrCmd.AddCommand(adrCreateCmd)
	adrCmd.AddCommand(adrListCmd)
	adrCmd.AddCommand(adrShowCmd)
}
