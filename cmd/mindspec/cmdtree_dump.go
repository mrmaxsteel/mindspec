package main

// cmd/mindspec/cmdtree_dump.go registers the hidden `__cmdtree`
// introspection command consumed by internal/lint's docs-truth R1
// check (mindspec-ng3g, W0 Bead 9). It replaces an AST-reconstructed
// model of the cobra.Command tree (which had to be independently
// specified wrong five times — see the deleted
// docs_truth_cmdtree_test.go's own history) with ground truth read
// directly off the real, fully-initialised tree this binary actually
// builds: no proxy for "is this verb live" left to get wrong, because
// the answer comes from cobra itself.
//
// Why a hidden command in THIS, the normal/untagged binary, rather
// than a build-tagged dump build: a build tag would make the lint
// certify a binary that is not byte-identical to the one MindSpec
// ships (binding ruling, /tmp/w0-panel/JUDGMENT.json,
// build_tag_divergence). cobra itself ships exactly this species of
// hidden, machine-readable introspection command (`__complete`), and
// this project already ships hidden commands (`spec-init`, the
// deprecation stubs in deprecated_commands.go) — `__cmdtree` is
// unremarkable precedent, not a new category of thing this binary
// does.
//
// Contract `__cmdtree` holds to (all load-bearing for the lint that
// consumes it):
//
//   - Reads NO repo, git, bd, network, or config state. It walks the
//     in-memory cobra.Command tree ONLY.
//   - Walks the FULLY-INITIALISED tree: by the time any leaf's RunE
//     runs (including this one), cobra's ExecuteC has already called
//     InitDefaultHelpCmd/InitDefaultCompletionCmd on the root, so
//     `help` and `completion` (and its zsh/bash/etc. children)
//     already exist as real nodes — the exact divergence the AST
//     model got wrong (it had no way to know cobra auto-registers
//     these).
//   - Excludes its OWN node from the dump (a lint walking "every
//     verb" should not have to know `__cmdtree` is special-cased
//     out-of-band).
//   - Flag and Args fidelity is read off cobra's own resolution
//     (LocalFlags/InheritedFlags, and invoking the real Args
//     validator) rather than re-derived from source, so a future
//     flag or Args-validator change can never drift from what this
//     dump reports — there is nothing left to keep in sync by hand.
import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// cmdTreeFlag is one flag visible on a node (own or inherited), with
// enough fidelity to end the "does a long flag consume the next
// word" guess a source-only model is stuck making: a bool flag
// consumes nothing, so Type tells a resolver `--foo bar` (bar is
// foo's value) apart from `--foo bar` (bar is a NEW positional,
// because foo is a bool and takes no value).
type cmdTreeFlag struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Type      string `json:"type"` // pflag.Value.Type(): "bool", "string", "int", ...
}

// cmdTreeNode is one node of the dumped tree.
type cmdTreeNode struct {
	Name    string `json:"name"` // first field of Use
	Use     string `json:"use"`
	Hidden  bool   `json:"hidden"`
	HasRun  bool   `json:"hasRun"`
	HasRunE bool   `json:"hasRunE"`
	// DisableFlagParsing is the field cmd/mindspec/deprecated_commands.go's
	// stubDeprecated sets so a stub's deprecation message fires even on
	// `--help` (cobra never gets to intercept it as a flag). It is what
	// makes the STRUCTURAL half of internal/lint's two-signal stub check
	// (mindspec-ng3g) agree with the BEHAVIOURAL half (does `<path>
	// --help` exit 2): Run-set-without-RunE ALONE is not sufficient —
	// cobra's own built-in `help` command is permanently Run-set,
	// RunE-unset, and genuinely live (verified: `mindspec help --help`
	// exits 0), because it does not set this field.
	DisableFlagParsing bool          `json:"disableFlagParsing"`
	OwnFlags           []cmdTreeFlag `json:"ownFlags"`
	InheritedFlags     []cmdTreeFlag `json:"inheritedFlags"`
	// ArgArities is the set of positional-argument counts, within
	// 0..argArityProbeCap, that this node's Args validator actually
	// accepts — see probeArgArities. ArgUnbounded is true when the
	// cap itself was accepted (no upper bound found in the probed
	// range, or c.Args == nil) or the field allows any count of args
	// e.g cobra.ArbitraryArgs.
	ArgArities    []int  `json:"argArities,omitempty"`
	ArgUnbounded  bool   `json:"argUnbounded"`
	ArgProbeError string `json:"argProbeError,omitempty"`

	Children []*cmdTreeNode `json:"children,omitempty"`
}

// argArityProbeCap is the highest arity probed by probeArgArities.
// Every Args validator in cmd/mindspec today is stock cobra with a
// small bound (per the binding ruling's audit: 30 ExactArgs, 8
// NoArgs, 2 MinimumNArgs, 2 MaximumNArgs, 1 ArbitraryArgs, zero custom
// func literals) — a cap comfortably above every real bound lets an
// unbounded validator be told apart from a bounded one: if the cap
// itself is accepted, the node is reported ArgUnbounded rather than
// "capped at argArityProbeCap".
const argArityProbeCap = 8

var cmdTreeDumpCmd = &cobra.Command{
	Use:    "__cmdtree",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		tree := dumpCmdTree(cmd.Root(), cmd)
		enc := json.NewEncoder(cmd.OutOrStdout())
		return enc.Encode(tree)
	},
}

func init() {
	rootCmd.AddCommand(cmdTreeDumpCmd)
}

// dumpCmdTree recursively walks c (and its children), excluding the
// exclude node (the running __cmdtree command itself) from the
// output.
func dumpCmdTree(c, exclude *cobra.Command) *cmdTreeNode {
	// Mirrors what cobra's own Command.execute() does for whichever
	// leaf a real invocation dispatches to (InitDefaultHelpFlag /
	// InitDefaultVersionFlag are only called on the FOUND command,
	// not eagerly on the whole tree) — calling them here on every
	// node reconstructs, for each command, the flag set it would
	// actually have if IT were the one invoked.
	c.InitDefaultHelpFlag()
	c.InitDefaultVersionFlag()

	n := &cmdTreeNode{
		Name:               c.Name(),
		Use:                c.Use,
		Hidden:             c.Hidden,
		HasRun:             c.Run != nil,
		HasRunE:            c.RunE != nil,
		DisableFlagParsing: c.DisableFlagParsing,
		OwnFlags:           dumpFlags(c.LocalFlags()),
	}
	n.InheritedFlags = dumpFlags(c.InheritedFlags())
	n.ArgArities, n.ArgUnbounded, n.ArgProbeError = probeArgArities(c)

	for _, child := range c.Commands() {
		if child == exclude {
			continue
		}
		n.Children = append(n.Children, dumpCmdTree(child, exclude))
	}
	return n
}

func dumpFlags(fs *pflag.FlagSet) []cmdTreeFlag {
	var out []cmdTreeFlag
	fs.VisitAll(func(f *pflag.Flag) {
		out = append(out, cmdTreeFlag{Name: f.Name, Shorthand: f.Shorthand, Type: f.Value.Type()})
	})
	return out
}

// probeArgArities reconstructs the set of positional-argument counts
// c.Args accepts by INVOKING it — ground truth, never a re-derivation
// of cobra's PositionalArgs constructors from source. Each arity
// 0..argArityProbeCap is tried TWICE with fresh synthetic args and any
// panic recovered: a validator that panics, or that disagrees with
// itself between the two calls at the same arity, is unreliable and
// reported via ArgProbeError rather than guessed past — the stated
// limit the binding ruling's flags_and_args section requires (a
// future impure or non-deterministic Args validator must fail loudly,
// not report a guess). c.Args == nil (the cobra default when a
// command sets no Args field) is reported unbounded, matching the
// prior AST model's "no Args field => unconstrained" safe default.
func probeArgArities(c *cobra.Command) (arities []int, unbounded bool, probeErr string) {
	if c.Args == nil {
		return nil, true, ""
	}
	for n := 0; n <= argArityProbeCap; n++ {
		synthetic := make([]string, n)
		for i := range synthetic {
			synthetic[i] = fmt.Sprintf("arg%d", i)
		}
		ok1, panic1 := probeArgsOnce(c, synthetic)
		if panic1 != "" {
			return arities, false, fmt.Sprintf("Args(%d args) panicked: %s", n, panic1)
		}
		ok2, panic2 := probeArgsOnce(c, synthetic)
		if panic2 != "" {
			return arities, false, fmt.Sprintf("Args(%d args) panicked on second call: %s", n, panic2)
		}
		if ok1 != ok2 {
			return arities, false, fmt.Sprintf("Args(%d args) is non-deterministic: %v then %v", n, ok1, ok2)
		}
		if ok1 {
			arities = append(arities, n)
			if n == argArityProbeCap {
				unbounded = true
			}
		}
	}
	return arities, unbounded, ""
}

// probeArgsOnce invokes c.Args(c, args) with a panic recovered into
// panicMsg rather than propagated — a single misbehaving validator
// must not take down the whole dump.
func probeArgsOnce(c *cobra.Command, args []string) (accepted bool, panicMsg string) {
	defer func() {
		if r := recover(); r != nil {
			panicMsg = fmt.Sprintf("%v", r)
		}
	}()
	return c.Args(c, args) == nil, ""
}
