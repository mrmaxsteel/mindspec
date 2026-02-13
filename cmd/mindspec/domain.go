package main

import (
	"fmt"

	"github.com/mindspec/mindspec/internal/domain"
	"github.com/spf13/cobra"
)

var domainCmd = &cobra.Command{
	Use:   "domain",
	Short: "Manage DDD bounded contexts",
}

var domainAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Scaffold a new domain with template docs and context map entry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findRoot()
		if err != nil {
			return err
		}

		name := args[0]
		if err := domain.Add(root, name); err != nil {
			return err
		}

		fmt.Printf("Domain scaffolded: docs/domains/%s/\n", name)
		fmt.Printf("Consider creating an ADR for the new '%s' domain.\n", name)
		return nil
	},
}

var domainListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all domains with ownership and relationships",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findRoot()
		if err != nil {
			return err
		}

		entries, err := domain.List(root)
		if err != nil {
			return err
		}

		fmt.Print(domain.FormatTable(entries))
		return nil
	},
}

var domainShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show detailed information about a domain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findRoot()
		if err != nil {
			return err
		}

		jsonFlag, _ := cmd.Flags().GetBool("json")

		info, err := domain.Show(root, args[0])
		if err != nil {
			return err
		}

		if jsonFlag {
			out, err := domain.FormatJSON(info)
			if err != nil {
				return err
			}
			fmt.Println(out)
		} else {
			fmt.Print(domain.FormatSummary(info))
		}
		return nil
	},
}

func init() {
	domainShowCmd.Flags().Bool("json", false, "Output as JSON")

	domainCmd.AddCommand(domainAddCmd)
	domainCmd.AddCommand(domainListCmd)
	domainCmd.AddCommand(domainShowCmd)
}
