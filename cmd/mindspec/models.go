package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// `mindspec models populate` mirrors `mindspec source populate`
// (source.go) and is one of spec 123's two new ZFC populate emitters
// (R6b): it prints an agent prompt for declaring the models: per-phase
// protocol in .mindspec/config.yaml and writes nothing. Repo-wide, no
// domain argument.

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Manage the declared per-phase model protocol",
}

var modelsPopulateCmd = &cobra.Command{
	Use:   "populate",
	Short: "Print an agent prompt for populating models:",
	Long: `Prints a templated agent prompt instructing the resident coding agent
to declare the per-phase model protocol under models: in
.mindspec/config.yaml. The framework proposes no model ids (ZFC) — this
key is declared-and-inert today; nothing in the mindspec binary reads
it to change behavior. Writes nothing.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runModelsPopulate(cmd.OutOrStdout())
	},
}

// modelsPopulatePrompt is the spec 123 Requirement 6(b) agent prompt,
// verbatim. ZFC: no pre-filled model ids — the framework never guesses
// a consumer's model protocol.
const modelsPopulatePrompt = `Populate the ` + "`models:`" + ` field in .mindspec/config.yaml.

Declare the model identity you use for each orchestration
phase you care about — the documented (not enforced)
vocabulary keys are ` + "`authoring`" + `, ` + "`implementation`" + `, and
` + "`review`" + `, but any key name is accepted.

IMPORTANT: models: is declared-and-inert today. Nothing in
the mindspec binary reads this key to change behavior — the
authoritative consumers of the model protocol remain the
orchestration skills (e.g. ` + "`ms-bead-impl`" + `, ` + "`ms-panel-run`" + `),
which you (or your operator) configure independently.
Populating this key makes the protocol discoverable and
honestly documented; it does not itself wire enforcement.

When done, edit .mindspec/config.yaml's ` + "`models:`" + ` map. Run
` + "`mindspec doctor`" + ` to confirm ` + "`missing-models`" + ` no longer Warns.
`

// runModelsPopulate emits the Requirement 6(b) prompt to w. Extracted
// from the RunE so the command's print behavior is unit-covered.
func runModelsPopulate(w io.Writer) error {
	// modelsPopulatePrompt already ends in "\n" (its raw-string literal's
	// own trailing line break); Fprint (not Fprintln) avoids a redundant
	// second trailing newline that go vet flags on a literal operand.
	fmt.Fprint(w, modelsPopulatePrompt)
	return nil
}

func init() {
	modelsCmd.AddCommand(modelsPopulateCmd)
	rootCmd.AddCommand(modelsCmd)
}
