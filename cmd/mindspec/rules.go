package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/rules"
	"github.com/spf13/cobra"
)

var rulesCmd = newRulesCmd()

func newRulesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "List the conduct rules",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := rulesWorkspace(); err != nil {
				return err
			}
			scope, _ := cmd.Flags().GetString("scope")
			selected, err := rules.Filter(scope, "")
			if err != nil {
				return err
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(selected)
			}
			for _, r := range selected {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s: %s — %s\n", r.ID, r.Prohibition, r.Replacement); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().String("scope", "", "select orchestrator, implementer, reviewer or fixer rules")
	cmd.Flags().Bool("json", false, "print rules as JSON")
	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Print one conduct rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := rulesWorkspace(); err != nil {
				return err
			}
			r, err := rules.Lookup(args[0])
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s: %s — %s\nscope: %s\nenforced_by: %s\n", r.ID, r.Prohibition, r.Replacement, strings.Join(r.Scope, ", "), r.EnforcedBy)
			if err != nil {
				return err
			}
			if len(r.Gates) > 0 {
				if _, err = fmt.Fprintf(cmd.OutOrStdout(), "gates: %s\n", strings.Join(r.Gates, ", ")); err != nil {
					return err
				}
			}
			if r.Target != "" {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "target: %s\n", r.Target)
			}
			return err
		},
	})
	return cmd
}

func rulesWorkspace() error {
	root, err := findPanelRoot(config.IsolationRootFor)
	if err != nil {
		return err
	}
	_, err = config.Load(root)
	return err
}

func init() {
	rootCmd.AddCommand(rulesCmd)
}
