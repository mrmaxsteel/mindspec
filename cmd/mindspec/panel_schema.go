package main

import (
	"fmt"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/spf13/cobra"
)

var panelSchemaCmd = &cobra.Command{
	Use: "schema", Short: "Print the verdict JSON Schema", Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := panel.VerdictV2Schema()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return err
	},
}

func init() { panelCmd.AddCommand(panelSchemaCmd) }
