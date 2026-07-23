package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// `mindspec commands populate` mirrors `mindspec source populate`
// (source.go) and `mindspec models populate` (models.go) — it is spec
// 123's second new ZFC populate emitter (R7c): it prints an agent
// prompt for declaring the consumer's build/test guidance under
// commands: in .mindspec/config.yaml and writes nothing. Repo-wide, no
// domain argument.

var commandsCmd = &cobra.Command{
	Use:   "commands",
	Short: "Manage the declared build/test command guidance",
}

var commandsPopulateCmd = &cobra.Command{
	Use:   "populate",
	Short: "Print an agent prompt for populating commands:",
	Long: `Prints a templated agent prompt instructing the resident coding agent
to declare THIS repo's build/test commands under commands: in
.mindspec/config.yaml. Once populated, "mindspec init" and every
"mindspec setup <agent>" verb render the declared commands as the
managed AGENTS.md "Build & Test" section — mindspec never guesses your
build system (ADR-0036 ZFC), and never plants its OWN build commands
into a consuming repo (ADR-0040's consumer-identity clause). Writes
nothing.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCommandsPopulate(cmd.OutOrStdout())
	},
}

// commandsPopulatePrompt is the spec 123 Requirement 7(c) agent prompt,
// verbatim. ZFC: no pre-filled commands — the framework never guesses a
// consumer's build system, and this repo's OWN build (`make build` /
// `make test`) must never leak in as a default (ADR-0040's
// consumer-identity clause — the #211 failure mode this prompt exists
// to prevent).
const commandsPopulatePrompt = `Populate the ` + "`commands:`" + ` field in .mindspec/config.yaml.

Inspect THIS repo — its build tooling, package manager, and
test runner — and declare the shell command for each task
you want documented. The documented (not enforced) vocabulary
keys are ` + "`build`" + ` and ` + "`test`" + `, but any key name is accepted.

IMPORTANT: the framework proposes no commands here — not even
its own (mindspec is a Go project; ` + "`make build`" + `/` + "`make test`" + `
are MINDSPEC's commands, never assume they apply to the repo
you are onboarding). Reach your own determination by reading
this repo's actual build tooling.

Once declared, ` + "`mindspec init`" + ` and every ` + "`mindspec setup <agent>`" + `
verb render your commands: entries as the managed AGENTS.md
"Build & Test" section; while unset, that section is omitted
entirely rather than showing a placeholder.

When done, edit .mindspec/config.yaml's ` + "`commands:`" + ` map. Run
` + "`mindspec doctor`" + ` to confirm ` + "`missing-commands`" + ` no longer Warns,
then re-run ` + "`mindspec setup <agent>`" + ` to refresh the managed block.
`

// runCommandsPopulate emits the Requirement 7(c) prompt to w. Extracted
// from the RunE so the command's print behavior is unit-covered.
func runCommandsPopulate(w io.Writer) error {
	// commandsPopulatePrompt already ends in "\n" (its raw-string
	// literal's own trailing line break); Fprint (not Fprintln) avoids a
	// redundant second trailing newline that go vet flags on a literal
	// operand.
	fmt.Fprint(w, commandsPopulatePrompt)
	return nil
}

func init() {
	commandsCmd.AddCommand(commandsPopulateCmd)
	rootCmd.AddCommand(commandsCmd)
}
