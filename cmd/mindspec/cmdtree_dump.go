package main

// cmd/mindspec/cmdtree_dump.go registers the hidden `__cmdtree`
// introspection command consumed by internal/lint's docs-truth R1
// check (mindspec-ng3g, W0 Bead 9). It replaces an AST-reconstructed
// model of the cobra.Command tree (which had to be independently
// specified wrong five times — see the deleted
// docs_truth_cmdtree_test.go's own history) with ground truth read
// directly off the real, fully-initialized tree this binary actually
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
//   - Walks the FULLY-INITIALIZED tree: by the time any leaf's RunE
//     runs (including this one), cobra's ExecuteC has already called
//     InitDefaultHelpCmd/InitDefaultCompletionCmd on the root, so
//     `help` and `completion` (and its zsh/bash/etc. children)
//     already exist as real nodes — the exact divergence the AST
//     model got wrong (it had no way to know cobra auto-registers
//     these).
//   - Records the PROVENANCE of every node (AutoRegistered) — see
//     sourceRegisteredCommands below — so a consumer can tell "a
//     command cmd/mindspec's own source added" apart from "a command
//     cobra bolted on at Execute() time" WITHOUT relying on any field
//     whose value merely correlates with that distinction (spec
//     w0-docs-truth final gate, L1-1: the previous design used
//     DisableFlagParsing as that correlate, which is a fact about how
//     a stub happens to behave today, not a fact about who added the
//     node — a future stub that forgot to set it would be invisible
//     to that scheme).
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

// sourceRegisteredCommands is the provenance table: every *cobra.Command
// pointer that was already attached to rootCmd's tree at the moment
// snapshotSourceRegisteredCommands ran. Go guarantees every package
// init() completes before main() runs, and every command cmd/mindspec's
// OWN source adds is attached via an AddCommand call reached from an
// init() function (grep any cmd/mindspec/*.go for "rootCmd.AddCommand"
// or a parent's own AddCommand chain) — so calling
// snapshotSourceRegisteredCommands(rootCmd) as the FIRST statement in
// main(), before rootCmd.Execute() runs, captures exactly "every
// command this repo's source wrote", with nothing yet added by cobra's
// own ExecuteC (InitDefaultHelpCmd/InitDefaultCompletionCmd, which run
// later, inside Execute()). A node present in the FULLY-INITIALIZED
// tree __cmdtree later walks but ABSENT from this snapshot is, by
// construction, cobra's own auto-registered addition, never
// cmd/mindspec's — a provenance fact, not a behavioral correlate.
//
// Constraint this depends on, found auditing this change (not a
// finding in the final gate, but the same class): `__cmdtree`'s
// AutoRegistered field is correct ONLY when this binary is invoked the
// normal way (main() -> snapshot -> Execute()) — e.g. always true of
// internal/lint's docs-truth lint, which always execs a freshly built
// binary as a subprocess. cmd/mindspec's OWN test suite calls
// rootCmd.Execute() IN-PROCESS at 20+ call sites (config_test.go,
// otel_test.go, panel_test.go, ...), never through main(), so
// sourceRegisteredCommands stays empty there — but none of those
// tests invoke `__cmdtree` itself, so this is dormant today. If one
// ever did, every node would report AutoRegistered=true (since the
// map is empty), forcing structuralStub false everywhere; the six
// real stubs would then disagree with their own correctly-stub
// behavioral signal and the build would hard-fail LOUDLY — the safe
// direction, not a silent misclassification, but worth knowing before
// adding an in-process `__cmdtree` test.
var sourceRegisteredCommands = map[*cobra.Command]bool{}

// snapshotSourceRegisteredCommands walks c's tree and records every
// node's pointer identity into sourceRegisteredCommands. Call exactly
// once, from main(), before rootCmd.Execute() — see the var doc above
// for why that ordering is what makes the snapshot correct.
func snapshotSourceRegisteredCommands(c *cobra.Command) {
	sourceRegisteredCommands[c] = true
	for _, child := range c.Commands() {
		snapshotSourceRegisteredCommands(child)
	}
}

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
	// AutoRegistered reports whether this node is one cobra bolted on
	// itself (InitDefaultHelpCmd/InitDefaultCompletionCmd, called
	// inside rootCmd.Execute()) rather than one cmd/mindspec's own
	// source added — see sourceRegisteredCommands' doc comment above.
	// This is the PROVENANCE half of internal/lint's two-signal stub
	// check (mindspec-ng3g): it is what lets the structural signal
	// stay the ruling's literal `HasRun && !HasRunE`, scoped to nodes
	// this repo actually wrote, with no reliance on any field (like
	// DisableFlagParsing below) that merely correlates with a stub's
	// behavior today — cobra's own built-in `help` command is
	// permanently Run-set/RunE-unset and genuinely live (verified:
	// `mindspec help --help` exits 0), and AutoRegistered=true is why
	// it is correctly excluded regardless of any other field.
	AutoRegistered bool `json:"autoRegistered"`
	// DisableFlagParsing is the field cmd/mindspec/deprecated_commands.go's
	// stubDeprecated sets so a stub's deprecation message fires even on
	// `--help` (cobra never gets to intercept it as a flag). It explains
	// WHY each of today's six real stubs happens to exit 2 on `--help` —
	// i.e. why the BEHAVIORAL half of the two-signal check (does
	// `<path> --help` exit 2) currently agrees with the structural half
	// for every existing stub — but, per the final-gate finding that
	// retired its former role here (L1-1), it is NOT what makes the two
	// signals independent, and a future stub is not required to set it:
	// see internal/lint/docs_truth_cmdtree_test.go's package doc comment
	// for the honest independence property.
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
		AutoRegistered:     !sourceRegisteredCommands[c],
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
