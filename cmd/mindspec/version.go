package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd is the subcommand form of `mindspec --version` (spec 096
// Req 5, bug 2b4n). Agents and humans reach for `mindspec version` too,
// but cobra only wires the `--version` FLAG (from rootCmd.Version), so
// the bare subcommand previously errored `unknown command "version"`.
//
// Its stdout is BYTE-EQUAL to `mindspec --version`. The `--version` flag
// renders cobra's default version template, which in cobra v1.8.1 is
// `{{with .Name}}{{printf "%s " .}}{{end}}{{printf "version %s" .Version}}\n`
// — i.e. "<rootCmd.Name()> version <rootCmd.Version>\n". We reproduce
// that exact decorated string by reading the SAME source (the root
// command's Name + Version, the latter being the ldflags-injected
// "version (commit) date" string built in root.go), NOT a hand-built
// literal name or a different format. No SetVersionTemplate override
// exists in the tree, so the default template is authoritative; the
// byte-equality test pins this against the live `--version` output.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the mindspec version (same output as --version)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		root := cmd.Root()
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s version %s\n", root.Name(), root.Version)
		return err
	},
}
