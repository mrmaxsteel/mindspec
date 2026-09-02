// docs_truth_cmdtree_test.go builds internal/lint's model of the real
// cobra.Command tree from GROUND TRUTH — the compiled cmd/mindspec
// binary's own `__cmdtree` introspection dump (cmd/mindspec/
// cmdtree_dump.go) — rather than reconstructing it from source AST.
// It is the R1 ground truth consumed by docs_truth_test.go
// (mindspec-ng3g, W0 Bead 9).
//
// # Why a built binary, not the AST
//
// This file previously (through 4ef82ee4) reconstructed the tree by
// parsing cmd/mindspec's own .go source and re-implementing enough of
// cobra's construction/registration semantics to answer "what verbs,
// flags, and stub-vs-live handlers does this binary actually have".
// That approach had to be independently re-specified WRONG FIVE TIMES
// trying to answer one question — "is this verb live, or a one-shot
// deprecation stub" — because every version checked a PROXY for a
// property the binary states exactly (tree membership, then Hidden,
// then Hidden&&DisableFlagParsing, then a literal-only Run/RunE read,
// then a Run/RunE read that didn't scope the assignment's receiver —
// see the binding ruling for the full history). It also could not see
// cobra's own auto-registered `help`/`completion` commands at all
// (they don't exist anywhere in cmd/mindspec's source — cobra adds
// them at Execute() time), so a doc mentioning either was a guaranteed
// false positive.
//
// cmd/mindspec already builds and executes itself at 30+ call sites
// in its own test suite (cmd/mindspec/testhelpers_test.go's
// buildMindspecBinary + strippedEnv), most without a `-short` guard,
// so reading ground truth off the real binary is an idiom this repo
// has already proven in CI, not a new one — reused here (see
// stripCmdTreeEnv) rather than re-invented. Measured cost (the
// binding ruling, /tmp/w0-panel/JUDGMENT.json): ~1.6s for a
// from-scratch build, ~8ms per exec. buildCmdTree's sync.Once pays
// that cost exactly once per `go test` run, regardless of how many
// tests in this package ask for the tree.
//
// # The stub signal — two independent signals, disagreement is a hard failure
//
// The property this lint wants is "does invoking this command run
// cobra's normal handler, or cmd/mindspec's one-shot deprecation
// message followed by os.Exit(2)". Per the binding ruling, that is
// answered with TWO signals that MUST AGREE — disagreement is a hard
// test failure, never a silent tie-break, because every prior
// (AST-only) version failed by trusting a single signal:
//
//   - STRUCTURAL: the dump's !AutoRegistered && HasRun && !HasRunE
//     (structuralStub) — the ruling's literal `Run != nil && RunE ==
//     nil` text, scoped by PROVENANCE (cmd/mindspec/cmdtree_dump.go's
//     AutoRegistered field) rather than by any field describing how
//     the node happens to BEHAVE. A node is structurally a stub only
//     if cmd/mindspec's own source added it (not cobra's
//     InitDefaultHelpCmd/InitDefaultCompletionCmd) and it is
//     Run-set/RunE-unset.
//   - BEHAVIOURAL: the exit code of `<binary> <path...> --help`, run
//     with explicit argv (probeBehavioralStub). A genuine stub sets
//     DisableFlagParsing, so cobra never intercepts `--help` as a
//     flag — it falls straight through to the stub's Run, which
//     unconditionally prints one line and os.Exit(2)s. A live command
//     (or a stub that OMITS DisableFlagParsing) has flag parsing
//     enabled, so cobra intercepts `--help` centrally and returns
//     before Run/RunE ever runs, exiting 0.
//
// ## The honest independence property (final-gate finding L1-1)
//
// An EARLIER version of this file used `HasRun && !HasRunE &&
// DisableFlagParsing` as the structural signal — DisableFlagParsing as
// a stand-in for "is this cobra's own auto-registered `help`, or one
// of our real stubs". That was wrong, and the doc comment that shipped
// alongside it recorded a FALSE safety argument: it claimed a future
// stub that omitted DisableFlagParsing would make the two signals
// disagree and hard-fail. It does not, because BOTH signals as written
// were functions of DisableFlagParsing: behaviourally, cobra only
// intercepts `--help` centrally (exit 0) when flag parsing is
// enabled, i.e. when DisableFlagParsing is false; structurally, the
// old formula reports "live" whenever DisableFlagParsing is false, for
// exactly the same reason. In the dfp=false half of the space the two
// signals therefore constant-agreed on "live" and could never
// disagree — a dfp-less stub was silently classified live, with no
// disagreement and no hard failure. This was discriminator attempt
// SIX; see the final-gate verdict (L1-1) for the empirical probe that
// found it (a Hidden, Run-set/RunE-unset, os.Exit(2) command with no
// DisableFlagParsing: `--help` on it exits 0, a bare invocation exits
// 2 — exactly the disagreement the old formula could never see).
//
// THIS version replaces the DisableFlagParsing conjunct with
// AutoRegistered — a fact about WHO added the node (provenance,
// snapshotted before cobra's Execute() ever runs; see
// cmd/mindspec/cmdtree_dump.go), not a fact about how the node
// happens to behave at runtime. That makes the two signals genuinely
// independent along the axis that matters:
//
//   - The STRUCTURAL signal is a pure function of (provenance, Run,
//     RunE) — it never reads DisableFlagParsing and therefore cannot
//     move with it. A dfp-less, source-added, Run-set/RunE-unset node
//     is unconditionally reported "stub" structurally, regardless of
//     dfp.
//   - The BEHAVIOURAL signal is a pure function of runtime dispatch
//     (does `--help` reach Run) and therefore DOES still depend on
//     dfp — that dependency is real and is not being removed; it is
//     no longer ALSO baked into the structural signal, which is what
//     made the two move together before.
//
// Concretely: a hypothetical future stub with Run set, RunE unset, and
// DisableFlagParsing OMITTED is structurally "stub" (not auto-
// registered, Run-set/RunE-unset — dfp plays no part) while
// behaviourally "live" (`--help` exits 0, since dfp=false lets cobra
// intercept it) — a genuine disagreement, and the build HARD-FAILS
// LOUDLY. TestCmdTree_DfpLessStubIsCaughtByProvenanceSignal below
// proves this against a REAL built binary containing exactly that
// shape, reproducing the final-gate probe as a permanent regression
// fixture.
//
// ## Two regions worth naming precisely (confirm-round findings L1-C2, L4-CONFIRM-1)
//
// An earlier version of this comment claimed ONE residual region where
// the two signals "cannot disagree", for auto-registered nodes. That
// claim was itself imprecise in both directions — corrected below —
// and named only one of the two regions that actually matter.
//
// ### Auto-registered nodes: the structural signal is PINNED, not the
// ### two signals made unable to disagree
//
// For every AUTO-REGISTERED node (`help`, `completion` and its
// children), AutoRegistered short-circuits structuralStub straight to
// "live" — the Run/RunE check is never even reached. That is a
// statement about ONE signal being pinned, not about disagreement
// being unreachable: disagreement is reachable here, and it is exactly
// what makes the empty-snapshot provenance hazard fail loudly rather
// than silently.
//
// Concretely (final-gate confirm round, L4-CONFIRM-1): if
// sourceRegisteredCommands (cmd/mindspec/cmdtree_dump.go) is ever empty
// when `__cmdtree` runs — which happens if something invokes
// rootCmd.Execute() in-process without going through main(), the only
// place that snapshot is populated — every node, including cmd/mindspec's
// own six real stubs, reports AutoRegistered=true. The PRODUCER is
// silently wrong under that condition: an isolated in-process
// `__cmdtree` invocation with an empty snapshot returns a valid,
// successfully-encoded JSON document with every source node's
// provenance mis-attributed, no error, nothing loud about it at all.
// It is only THIS PACKAGE's consumer-side cross-check that turns that
// into a hard failure: AutoRegistered=true forces structuralStub=false
// for the six real stubs too, which disagrees with their correctly-stub
// BEHAVIOURAL signal (`--help` still exits 2 for each of them,
// independent of the dump), and escalateTreeProblems hard-fails on that
// disagreement. Verified directly: neutralising
// snapshotSourceRegisteredCommands's call in an isolated checkout
// reproduces exactly this — six disagreements, one per real stub, hard
// build failure.
//
// So: safety here is a property of the CONSUMER'S CROSS-CHECK, not a
// guarantee the dump itself makes. Any future consumer of `__cmdtree`
// that reads AutoRegistered without ALSO cross-checking it against an
// independent behavioural signal (as this file does) inherits the
// hazard silently — see cmd/mindspec/cmdtree_dump.go's own provenance
// note, which this paragraph is the honest counterpart to.
//
// ### The one region that genuinely constant-agrees: a RunE-shaped one-shot stub
//
// structuralStub requires HasRun (Run set) && !HasRunE (RunE unset). A
// node that is source-added, Hidden, one-shot (its handler prints a
// message and os.Exit(2)s) but written with RunE instead of Run never
// satisfies HasRun, so structuralStub is unconditionally false for it —
// AutoRegistered and DisableFlagParsing play no part. Behaviourally, if
// such a node also omits DisableFlagParsing, cobra intercepts `--help`
// centrally and exits 0 before RunE ever runs — also "not stub". Both
// signals silently agree "live" for a dfp-less, RunE-shaped one-shot
// stub; this scheme cannot see it. There is no fixture proving this is
// UNREACHABLE, because it is not: today's six real stubs all happen to
// use Run (deprecated_commands.go's stubDeprecated), and that helper's
// own doc comment explains why RunE cannot be used for a stub there
// (RunE-returned errors exit 1, not the documented 2) — but that is a
// fact about today's one call site, not a property this two-signal
// design enforces.
//
// THE STRUCTURAL SIGNAL MUST NEVER BE USED ALONE regardless: collapsing
// to bare "!AutoRegistered && HasRun && !HasRunE" with no live
// behavioural cross-check would remove the only thing that makes a
// wrong provenance computation, or a future cobra change to how
// auto-registered commands are shaped, visible. A future maintainer
// tempted to "simplify" by deleting the behavioural probe and trusting
// the structural signal alone must not.
//
// ### A rule for future authors, not just for the lint (final-gate finding L1-C3)
//
// The flip side of the structural signal being HasRun-based: an
// ordinary, non-deprecated cobra command written with `Run:` (cobra's
// own idiom, used throughout its docs) and no DisableFlagParsing is
// structurally "stub" here while behaviourally live — a genuine
// disagreement, and this build HARD-FAILS on it (the safe direction,
// never a silent misclassification). In cmd/mindspec, `Run:` is
// reserved for deprecated_commands.go's one-shot deprecation stubs;
// any new, genuinely live command must use `RunE:`. The disagreement
// error text (linkDumpedNode, below) says this explicitly so the
// failure reads as "use RunE" rather than as a lint bug.
//
// `__complete`, the OTHER Run-set/RunE-unset cobra builtin, never
// appears in the dump at all: cobra registers it transiently, deep
// inside Find()'s dynamic completion-request detection, and removes
// it again before returning (spf13/cobra@v1.8.1 completions.go:195,
// :263-291) — it is not part of the tree ExecuteC's
// InitDefaultHelpCmd/InitDefaultCompletionCmd calls install up front,
// so there is nothing here for it to disagree about.
//
// When deprecated_commands.go is eventually deleted, the structural
// signal still degrades correctly: no remaining source-added node sets
// Run without RunE, so no node is ever marked stub, `--help`
// behaviourally returns 0 everywhere, the two signals still agree
// ("no stubs"), and R5 simply never fires — the exact vacuous-not-wrong
// degradation the ruling requires.
package lint

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
)

// cmdFlag is one flag visible on a node — its own, or inherited from
// an ancestor's persistent flags (already resolved transitively by
// cmd/mindspec's dump via cobra's own InheritedFlags(), so this file
// never needs to walk ancestors itself). Type is pflag's
// Value.Type() ("bool", "string", "int", ...) — see resolve()'s
// flag-value-skip logic for why this ends a heuristic the AST model
// was stuck guessing at.
type cmdFlag struct {
	Name      string
	Shorthand string
	Type      string
}

// cmdNode is one node of the real command tree, built once from
// cmd/mindspec's __cmdtree dump.
type cmdNode struct {
	Use      string
	Name     string // first whitespace-delimited field of Use
	IsStub   bool
	ArgMax   int                // max positional args this node's Args validator allows; -1 = unconstrained
	Flags    map[string]cmdFlag // keyed by both long name and shorthand (when set) — see linkDumpedNode
	Parent   *cmdNode
	Children []*cmdNode
}

func (n *cmdNode) findChild(name string) *cmdNode {
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// path returns the dotted verb path for diagnostics, e.g. "panel disposition validate".
func (n *cmdNode) path() string {
	var parts []string
	for cur := n; cur != nil; cur = cur.Parent {
		parts = append([]string{cur.Name}, parts...)
	}
	return strings.Join(parts, " ")
}

// resolveResult is the outcome of resolving a documented invocation's
// word list against a cmdTree.
type resolveResult struct {
	Resolved bool
	Reason   string // populated when !Resolved
	Node     *cmdNode
}

// resolve walks words (already whitespace-tokenized, words[0] expected
// to be the root's own name, e.g. "mindspec") down the tree as far as
// real children exist, then treats any remaining non-flag words as
// positional arguments (a leaf command's own business, not this
// resolver's — except when the deepest node reached sets an Args
// validator this lint models, the ArgMax guard below) and checks any
// `--flag`/short-flag words against the deepest node reached (plus
// its ancestors' persistent flags, already folded into Flags).
// resolve's single unified pass replaces an earlier two-phase design
// (descend-until-first-flag, THEN validate flags/positionals against
// wherever descent stopped) that had a seam final-gate finding
// L4-FINAL-2 found: a real root flag preceding a bogus verb (e.g.
// `mindspec --trace x totallybogus`) made the OLD descent loop stop at
// "--trace" — the FIRST flag-shaped word, no matter where in the
// invocation it appeared — and hand everything after it, including a
// real subcommand word like "totallybogus", to the trailing
// flag/positional loop as an unconstrained positional against
// whichever (possibly still-root) node descent had reached, never
// consulting cur.findChild again. A routine "global flag before the
// verb" invocation shape therefore smuggled an absent command past R1
// even though the real command tree data (Bead 9) was correct — the
// consumer simply stopped asking it. This version interleaves flag
// recognition and subcommand descent in ONE pass, so a flag anywhere
// in the invocation never stops descent into a LATER real subcommand
// word, and an absent word is still checked against cur's real
// children no matter how many flags precede it.
func (root *cmdNode) resolve(words []string) resolveResult {
	if len(words) == 0 {
		return resolveResult{Resolved: false, Reason: "empty invocation"}
	}
	if words[0] != root.Name {
		return resolveResult{Resolved: false, Reason: fmt.Sprintf("root name mismatch: %q", words[0])}
	}
	cur := root
	var positionals []string
	// pendingFlag holds the flag TOKEN (e.g. "--trace" or "-f") that
	// still needs a value, between seeing that flag and consuming the
	// word after it — empty means nothing is pending. Final-gate
	// finding L4-FINAL-2's second still-open case: the old
	// `skipNextAsFlagValue bool` set this unconditionally and never
	// checked afterward whether a value word actually arrived, so
	// `mindspec --trace` (the flag as the LAST word, no value) silently
	// resolved — the real binary's pflag.Parse rejects it with "flag
	// needs an argument: --trace" (verified against the built binary).
	// Naming the token (not just a bool) lets the end-of-loop check
	// below report which flag was left dangling.
	pendingFlag := ""
	sawDoubleDash := false // "--": cobra's own end-of-flags marker; everything after is literal, never a flag or a subcommand name.
	for _, w := range words[1:] {
		if pendingFlag != "" {
			pendingFlag = ""
			continue
		}
		if !sawDoubleDash {
			switch {
			case w == "--":
				sawDoubleDash = true
				continue
			case strings.HasPrefix(w, "--"):
				// A long flag without an inline `--name=value` is
				// assumed to consume the NEXT word as its value UNLESS
				// its registered Type is "bool" (a bool flag takes no
				// value at all — the fidelity win real flag-type data
				// gives over the AST model's blind "always skip" guess,
				// O2-r2-4). Checked against cur — wherever descent has
				// reached SO FAR, which is exactly right: cobra resolves
				// each command's own (and inherited) flags at whatever
				// point in the argv it is currently parsing.
				name := strings.TrimPrefix(w, "--")
				hasInlineValue := false
				if eq := strings.IndexByte(name, '='); eq >= 0 {
					name = name[:eq]
					hasInlineValue = true
				}
				flag, ok := cur.Flags[name]
				if !ok {
					return resolveResult{Resolved: false, Reason: fmt.Sprintf("flag --%s not registered on %q", name, cur.path())}
				}
				if !hasInlineValue && flag.Type != "bool" {
					pendingFlag = w
				}
				continue
			case len(w) > 1 && w[0] == '-':
				// A short flag WORD, one or more shorthand characters
				// clustered together (pflag's parseShortArg /
				// parseSingleShortArg, spf13/pflag@v1.0.5 flag.go:
				// 1007-1074, the version this repo's go.sum pins):
				// walk the characters left to right. A "bool"-typed
				// shorthand (pflag sets NoOptDefVal for it) consumes no
				// value, so the NEXT character starts a NEW shorthand
				// in the SAME word — this is what makes `-hv` (help +
				// version, final-gate finding L4-FINAL-2's reported
				// false rejection: the real binary accepts it and
				// prints help, verified against the built binary) two
				// flags, not one unregistered flag named "hv". The
				// first NON-bool shorthand ends the cluster: pflag
				// takes an inline `-f=value`, or the REST of the word
				// (`-fvalue`), as that flag's value; a bare trailing
				// `-f` with nothing left in the word takes the NEXT
				// word as its value, mirrored below with pendingFlag
				// exactly as the long-flag case above.
				shorthands := w[1:]
				for len(shorthands) > 0 {
					name := shorthands[:1]
					flag, ok := cur.Flags[name]
					if !ok {
						return resolveResult{Resolved: false, Reason: fmt.Sprintf("flag -%s not registered on %q", name, cur.path())}
					}
					switch {
					case len(shorthands) > 2 && shorthands[1] == '=':
						// "-f=value": inline value, cluster ends here.
						shorthands = ""
					case flag.Type == "bool":
						// "-f" consumes nothing; keep walking the cluster.
						shorthands = shorthands[1:]
						continue
					case len(shorthands) > 1:
						// "-fvalue": rest of the word is f's value.
						shorthands = ""
					default:
						// "-f" alone at the end of the word: f's value
						// is the NEXT word (or nothing — the
						// end-of-loop pendingFlag check below catches
						// that case exactly like the long-flag form).
						pendingFlag = "-" + name
						shorthands = ""
					}
					break
				}
				continue
			}
		}
		// A non-flag word: descend into a matching child while cur still
		// HAS children (a leaf's own business is its positional args,
		// never a subcommand lookup — unchanged from before); report
		// unresolved if it does not match any real child. Once cur has
		// no children left (or "--" was seen), every remaining non-flag
		// word is a candidate positional for the ArgMax check below.
		if !sawDoubleDash && len(cur.Children) > 0 {
			child := cur.findChild(w)
			if child == nil {
				return resolveResult{Resolved: false, Reason: fmt.Sprintf("no subcommand %q under %q", w, cur.path())}
			}
			cur = child
			continue
		}
		positionals = append(positionals, w)
	}
	if pendingFlag != "" {
		return resolveResult{Resolved: false, Reason: fmt.Sprintf("flag needs an argument: %s (nothing follows it in this invocation)", pendingFlag)}
	}
	// A root-specific application behavior, NOT stock cobra's legacyArgs
	// (root.Args is cobra.ArbitraryArgs, which accepts any positional
	// count — legacyArgs never runs here): cmd/mindspec/root.go pairs
	// that with a RunE that manually reproduces the "unknown command"
	// error whenever it receives ANY leftover positional word
	// (rootUnknownCommandError, spec 092 Req 10b — added so near-misses
	// like `mindspec aprove impl` surface the canonical noun-verb gate
	// commands instead of cobra's default message). The only way this
	// resolver's own descent loop can hand root a non-empty positionals
	// list is "--": every other word is always checked against
	// findChild first (immediately above), so this reproduces
	// rootUnknownCommandError's behavior exactly — including that it
	// fires even when the leftover word happens to spell a real
	// subcommand name, verified against the built binary
	// (`mindspec -- doctor` -> "unknown command \"doctor\" for
	// \"mindspec\"", exit 1; final-gate finding L4-FINAL-2's first
	// reported false resolution, `mindspec -- panel disposition
	// validate`).
	if cur == root && len(positionals) > 0 {
		return resolveResult{Resolved: false, Reason: fmt.Sprintf("unknown command %q for %q (root's RunE rejects any leftover positional word once descent stops at root — see rootUnknownCommandError in cmd/mindspec/root.go)", positionals[0], cur.path())}
	}
	if cur.IsStub {
		return resolveResult{Resolved: false, Reason: fmt.Sprintf("%q resolves to a one-shot deprecation stub (source-added, Run-set/RunE-unset — see docs_truth_cmdtree_test.go's two-signal cross-check)", cur.path())}
	}
	// ArgMax >= 0 means cur's Args validator has a bound this lint
	// modeled by actually invoking it (probeArgArities, cmd/mindspec/
	// cmdtree_dump.go): placeholders are skipped (same reasoning as
	// isPlaceholderWord's doc comment — a doc author's `<spec-id>`
	// can't be told apart from zero or one real args, so only a
	// concrete EXTRA word is ever reported), and the first concrete
	// positional beyond the bound fails. A validator's own LOWER bound
	// (e.g. ExactArgs's minimum) is a documented residual gap, not
	// enforced here, for the identical reason.
	if cur.ArgMax >= 0 {
		seen := 0
		for _, w := range mergeQuotedPositionals(positionals) {
			if isPlaceholderWord(w) {
				continue
			}
			seen++
			if seen > cur.ArgMax {
				if cur.ArgMax == 0 {
					return resolveResult{Resolved: false, Reason: fmt.Sprintf("%q does not accept positional arguments (Args: cobra.NoArgs), got %q", cur.path(), w)}
				}
				return resolveResult{Resolved: false, Reason: fmt.Sprintf("%q accepts at most %d positional argument(s) (Args: cobra.ExactArgs/MaximumNArgs), got extra %q", cur.path(), cur.ArgMax, w)}
			}
		}
	}
	return resolveResult{Resolved: true, Node: cur}
}

// isPlaceholderWord reports whether w is doc metasyntax for "some
// value goes here" (`<bead-id>`, `[--flag]`, a quoted string) rather
// than a literal positional argument a doc author actually typed, so
// the ArgMax arity check above doesn't false-positive on the ordinary
// placeholder forms docs use to describe a command's syntax.
func isPlaceholderWord(w string) bool {
	if strings.HasPrefix(w, "<") || strings.HasPrefix(w, "[") {
		return true
	}
	return len(w) >= 2 && (w[0] == '"' || w[0] == '\'')
}

// mergeQuotedPositionals collapses a doc author's quoted, multi-word
// placeholder phrase back into one token before the ArgMax check
// counts positionals. tokenizeInvocation is a plain whitespace split,
// so `mindspec adr create "Use WebSockets for real-time updates"`
// (project-docs/user/README.md:84, a TRUE claim: adr.go sets Args:
// cobra.ExactArgs(1)) arrives here as five separate words — `"Use`,
// `WebSockets`, `for`, `real-time`, `updates"` — of which only the
// first satisfies isPlaceholderWord's "starts with a quote" test; the
// other four would each miscount as an extra concrete positional
// argument and false-positive a true claim. This merges every word
// from an opening quote through the word that closes it (matching
// quote character) into a single placeholder-shaped token, so
// isPlaceholderWord sees one phrase, not five words. An unterminated
// quote (malformed doc text) still collapses to one trailing token
// rather than leaking unmerged words into the count.
func mergeQuotedPositionals(words []string) []string {
	var out []string
	var quote byte
	var buf strings.Builder
	for _, w := range words {
		if quote == 0 {
			if len(w) >= 1 && (w[0] == '"' || w[0] == '\'') && !(len(w) > 1 && w[len(w)-1] == w[0]) {
				quote = w[0]
				buf.Reset()
				buf.WriteString(w)
				continue
			}
			out = append(out, w)
			continue
		}
		buf.WriteByte(' ')
		buf.WriteString(w)
		if len(w) > 0 && w[len(w)-1] == quote {
			out = append(out, buf.String())
			quote = 0
		}
	}
	if quote != 0 {
		out = append(out, buf.String())
	}
	return out
}

// --- ground truth: build + introspect the real binary ----------------

// cmdTreeFlagDump/cmdTreeDump mirror cmd/mindspec/cmdtree_dump.go's
// JSON shape exactly (cmdTreeFlag/cmdTreeNode there).
type cmdTreeFlagDump struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand"`
	Type      string `json:"type"`
}

type cmdTreeDump struct {
	Name               string            `json:"name"`
	Use                string            `json:"use"`
	Hidden             bool              `json:"hidden"`
	HasRun             bool              `json:"hasRun"`
	HasRunE            bool              `json:"hasRunE"`
	AutoRegistered     bool              `json:"autoRegistered"`
	DisableFlagParsing bool              `json:"disableFlagParsing"`
	OwnFlags           []cmdTreeFlagDump `json:"ownFlags"`
	InheritedFlags     []cmdTreeFlagDump `json:"inheritedFlags"`
	ArgArities         []int             `json:"argArities"`
	ArgUnbounded       bool              `json:"argUnbounded"`
	ArgProbeError      string            `json:"argProbeError"`
	Children           []*cmdTreeDump    `json:"children"`
}

var (
	cmdTreeOnce   sync.Once
	cmdTreeCached *cmdNode
	cmdTreeErr    error
)

// buildCmdTree returns the real cmd/mindspec command tree, built and
// introspected exactly once per `go test` run (sync.Once) regardless
// of how many tests call it — the binding ruling's measured cost
// (~1.6s build + ~8ms/exec) is paid once, not per test. t is used only
// to obtain a scratch TempDir for the build; the Once body never calls
// a Fatal-style method on it (t.Fatalf inside a sync.Once.Do would
// Goexit out of the closure, leaving cmdTreeCached/cmdTreeErr at their
// zero values for every LATER caller — a silent-success-looking nil
// tree). Every failure path here returns a plain error instead, for
// the caller (same call shape docs_truth_test.go already uses) to
// t.Fatalf on.
func buildCmdTree(t *testing.T) (*cmdNode, error) {
	t.Helper()
	tmp := t.TempDir()
	cmdTreeOnce.Do(func() {
		cmdTreeCached, cmdTreeErr = buildCmdTreeOnce(tmp)
	})
	return cmdTreeCached, cmdTreeErr
}

func buildCmdTreeOnce(tmp string) (*cmdNode, error) {
	binPath, err := buildMindspecBinaryHermetic(tmp)
	if err != nil {
		return nil, err
	}

	dumpOut, err := runMindspec(binPath, tmp, []string{"__cmdtree"})
	if err != nil {
		return nil, fmt.Errorf("%s __cmdtree: %w", binPath, err)
	}
	var dump cmdTreeDump
	if err := json.Unmarshal(dumpOut, &dump); err != nil {
		return nil, fmt.Errorf("unmarshal __cmdtree output: %w\noutput: %s", err, dumpOut)
	}

	var disagreements []string
	var probeErrors []string
	root := linkDumpedNode(&dump, nil, binPath, tmp, &disagreements, &probeErrors)
	if err := escalateTreeProblems(disagreements, probeErrors); err != nil {
		return nil, err
	}
	if root.Name != "mindspec" {
		return nil, fmt.Errorf(`__cmdtree root Use %q does not start with "mindspec"`, dump.Use)
	}
	return root, nil
}

// escalateTreeProblems is the sole gate between a collected
// disagreement/probe-error and a hard build failure — extracted to a
// pure, independently-callable function (final-gate finding L1-2)
// specifically so a fixture can assert the escalation itself, rather
// than only asserting that linkDumpedNode appended to a slice. Before
// this extraction, nothing in this package's tests exercised the `if
// len(disagreements) > 0` check that turns a detected disagreement
// into an aborted build — deleting or short-circuiting it (e.g. `if
// false && len(disagreements) > 0`) left the whole suite green, which
// is exactly the "safety mechanism whose own failure is invisible"
// class this lane keeps re-finding. See
// TestEscalateTreeProblems_DisagreementIsHardFailure.
func escalateTreeProblems(disagreements, probeErrors []string) error {
	if len(probeErrors) > 0 {
		return fmt.Errorf("Args validator probe failure(s), refusing to guess:\n  %s", strings.Join(probeErrors, "\n  "))
	}
	if len(disagreements) > 0 {
		return fmt.Errorf("stub-signal disagreement(s) between behavioural and structural checks — this is ALWAYS a hard failure, never a silent tie-break:\n  %s", strings.Join(disagreements, "\n  "))
	}
	return nil
}

// buildMindspecBinaryHermetic builds ./cmd/mindspec into <tmp>/mindspec,
// returning the binary path. Shared by buildCmdTreeOnce (the real,
// cached tree) and any test that needs its OWN independent throwaway
// binary (e.g. TestStubSignalDisagreement_IsHardFailure, which must
// inject a false structural claim onto a real path without disturbing
// the shared cache).
//
// Deliberately does NOT set cmd.Env (inherits the real environment,
// same as cmd/mindspec/testhelpers_test.go's buildMindspecBinary):
// stripCmdTreeEnv's scrubbed HOME/PATH is for EXECUTING the already-
// built binary only, never for the build step itself — `go build`
// needs the real HOME/GOPATH/GOCACHE/GOMODCACHE to reuse the existing
// module cache rather than re-populating one from scratch under a
// throwaway HOME (which is slow, needs network, and — module cache
// files are marked read-only by the go tool — leaves files t.TempDir's
// cleanup cannot remove).
func buildMindspecBinaryHermetic(tmp string) (string, error) {
	repoRoot, err := repoRootPlain()
	if err != nil {
		return "", err
	}
	binPath := filepath.Join(tmp, "mindspec")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/mindspec")
	buildCmd.Dir = repoRoot
	var stderr strings.Builder
	buildCmd.Stderr = &stderr
	if err := buildCmd.Run(); err != nil {
		return "", fmt.Errorf("go build ./cmd/mindspec: %w\nstderr: %s", err, stderr.String())
	}
	return binPath, nil
}

// linkDumpedNode recursively converts a cmdTreeDump into a *cmdNode,
// computing IsStub via the two-signal cross-check for every node
// (appending to disagreements on mismatch) and ArgMax from the dump's
// probed arities (appending to probeErrors when the dump itself
// reported an unreliable Args validator). words is the path of real
// argv words from the root to this node's PARENT (empty at the root),
// used to invoke `<binPath> <words...> <name> --help` for the
// behavioural signal.
func linkDumpedNode(d *cmdTreeDump, parent *cmdNode, binPath, tmp string, disagreements, probeErrors *[]string) *cmdNode {
	n := &cmdNode{
		Use:    d.Use,
		Name:   firstField(d.Use),
		Parent: parent,
		Flags:  map[string]cmdFlag{},
	}
	for _, f := range d.OwnFlags {
		addCmdFlag(n.Flags, f)
	}
	for _, f := range d.InheritedFlags {
		addCmdFlag(n.Flags, f)
	}

	if d.ArgProbeError != "" {
		*probeErrors = append(*probeErrors, fmt.Sprintf("%s: %s", n.path(), d.ArgProbeError))
	}
	switch {
	case d.ArgUnbounded, len(d.ArgArities) == 0:
		n.ArgMax = -1
	default:
		n.ArgMax = d.ArgArities[len(d.ArgArities)-1] // probeArgArities emits arities in ascending order
	}

	structuralStub := !d.AutoRegistered && d.HasRun && !d.HasRunE
	behavioralStub, err := probeBehavioralStub(binPath, tmp, pathWordsFromNode(n))
	switch {
	case err != nil:
		*probeErrors = append(*probeErrors, fmt.Sprintf("%s: behavioural stub probe: %v", n.path(), err))
		// Fail CLOSED (final-gate finding L1-2): an unclassifiable node
		// must not silently count as a resolvable live verb. This only
		// matters if escalateTreeProblems is ever bypassed — the
		// non-empty probeErrors already aborts the whole build via
		// escalateTreeProblems — but a regression in THAT escalation
		// must degrade safely, not open R5.
		n.IsStub = true
	case behavioralStub != structuralStub:
		// L1-C3: the most likely CAUSE of this disagreement in practice
		// is not a lint bug but an ordinary cobra command written with
		// `Run:` and no DisableFlagParsing — cmd/mindspec reserves bare
		// Run for one-shot deprecation stubs (deprecated_commands.go's
		// stubDeprecated); any other command must use RunE, or it is
		// structurally "stub" here (!AutoRegistered && HasRun &&
		// !HasRunE) while behaviourally live (`--help` still exits 0),
		// exactly this disagreement.
		*disagreements = append(*disagreements, fmt.Sprintf("%s: behavioural(exit-code-derived)=%v structural(!autoRegistered&&hasRun&&!hasRunE)=%v -- if this node is a genuinely live command, use RunE instead of Run; cmd/mindspec reserves Run for one-shot deprecation stubs (see deprecated_commands.go's stubDeprecated)", n.path(), behavioralStub, structuralStub))
		n.IsStub = true // fail CLOSED — see the case above.
	default:
		n.IsStub = structuralStub
	}

	for _, cd := range d.Children {
		n.Children = append(n.Children, linkDumpedNode(cd, n, binPath, tmp, disagreements, probeErrors))
	}
	return n
}

// pathWordsFromNode reconstructs the argv words from root to n
// (inclusive) by walking n.Parent — used only during linkDumpedNode,
// before n.path()'s own ancestor chain would otherwise be usable, so
// it takes the same walk path() does.
func pathWordsFromNode(n *cmdNode) []string {
	var parts []string
	for cur := n; cur != nil; cur = cur.Parent {
		parts = append([]string{cur.Name}, parts...)
	}
	return parts
}

func addCmdFlag(m map[string]cmdFlag, f cmdTreeFlagDump) {
	cf := cmdFlag{Name: f.Name, Shorthand: f.Shorthand, Type: f.Type}
	m[f.Name] = cf
	if f.Shorthand != "" {
		m[f.Shorthand] = cf
	}
}

func firstField(use string) string {
	fields := strings.Fields(use)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// probeBehavioralStub runs `<binPath> <pathWords[1:]...> --help` and
// reports whether the exit code matches a one-shot deprecation stub
// (exit 2) or a live command (exit 0) — the BEHAVIOURAL half of the
// two-signal stub check. --help is used, never a bare invocation:
// bare `mindspec doctor` does real work (the judge measured this;
// see the binding ruling), but `--help` on a live command is always
// inert (intercepted centrally by cobra before Run/RunE), and on a
// genuine stub (DisableFlagParsing=true) it is swallowed as a literal
// arg and still fires the stub's Run. Any exit code other than 0 or 2
// is an anomaly this lint refuses to interpret — returned as an error
// rather than silently classified either way.
func probeBehavioralStub(binPath, tmp string, pathWords []string) (bool, error) {
	args := append(append([]string{}, pathWords[1:]...), "--help")
	_, runErr := runMindspec(binPath, tmp, args)
	if runErr == nil {
		return false, nil // exit 0: live
	}
	var exitErr *exec.ExitError
	if !errors.As(runErr, &exitErr) {
		return false, fmt.Errorf("exec %s %s: %w", binPath, strings.Join(args, " "), runErr)
	}
	switch exitErr.ExitCode() {
	case 2:
		return true, nil
	default:
		return false, fmt.Errorf("%s %s exited %d, expected 0 (live) or 2 (stub)", binPath, strings.Join(args, " "), exitErr.ExitCode())
	}
}

// runMindspec execs binPath with args in a hermetic env, returning
// combined stdout (stderr is discarded on success; included in the
// wrapped error on failure via *exec.ExitError, which callers inspect
// for its exit code — Stderr is intentionally not captured here since
// no caller needs the stub's one-line message text, only its exit
// code).
// runMindspec sets cmd.Dir = tmp (final-gate finding L3-2): every exec
// site here runs the built binary from INSIDE this repo's own working
// directory otherwise (there is no other cwd to inherit — this file's
// tests run as part of `go test ./internal/lint/`), so a relative
// MINDSPEC_TRACE — or any future flag/env var this binary resolves
// relative to cwd — would drop an untracked artifact straight into the
// tracked source tree instead of the throwaway tmp this function
// already receives for exactly this purpose. Every other stateful
// binary-exec site in cmd/mindspec's OWN test suite
// (approval_gates_test.go, doctor_migration_test.go,
// greenfield_e2e_test.go, otel_test.go, next_dirty_test.go) sets
// cmd.Dir to a temp dir; this one now matches that idiom.
func runMindspec(binPath, tmp string, args []string) ([]byte, error) {
	cmd := exec.Command(binPath, args...)
	cmd.Dir = tmp
	cmd.Env = stripCmdTreeEnv(tmp)
	return cmd.Output()
}

// stripCmdTreeEnv builds a hermetic environment for building/execing
// the mindspec binary this file introspects — the SAME scrubbing
// shape cmd/mindspec/testhelpers_test.go's strippedEnv already
// establishes and exercises in CI (spec 084 Bead 2/3), PLUS one
// addition this file needs and strippedEnv's own callers do not
// (final-gate finding L3-2): no inherited AGENTMIND_BIN, an empty PATH
// (via emptyDir), a fresh HOME (so config probes can't pick up
// developer-host state), no OTEL_*/CLAUDE_CODE_ENABLE_TELEMETRY
// leakage, and — the addition — no ambient MINDSPEC_* leakage either.
// strippedEnv's own callers never needed that: they invoke the binary
// from a scratch workspace dir they control end to end, so an
// ambient MINDSPEC_TRACE pointed somewhere reasonable is (at worst)
// harmless there. This file is different: runMindspec execs FROM
// INSIDE the repo (see its own doc comment), so an ambient
// MINDSPEC_TRACE with a RELATIVE path would write into the tracked
// source tree, and an unwritable one would make every `__cmdtree`
// call exit 1 with an opaque, env-var-blind error — the whole
// docs-truth suite becomes unrunnable and the failure is
// undiagnosable from the error text alone. Scrubbing it here removes
// that failure mode entirely; it is not merely "the same shape",
// deliberately stricter for a reason specific to how this file execs.
// Duplicated rather than imported: cmd/mindspec/testhelpers_test.go is
// a _test.go file in a different package (main), and Go does not
// allow importing another package's test-only sources — the binding
// ruling's mandate to REUSE this shape (not invent a second, subtly
// different hermetic-env idiom) is honored by copying the same
// env-var list (plus the one documented addition), not by a
// cross-package import that does not exist as a possibility.
// emptyDir/homeDir are subdirectories of the caller-supplied tmp
// (itself a t.TempDir(), so both are cleaned up with it) rather than
// fresh t.TempDir() calls, because this runs from inside buildCmdTree's
// sync.Once body, which — per that function's own doc comment — must
// never touch a *testing.T once linkDumpedNode's exec loop begins.
func stripCmdTreeEnv(tmp string) []string {
	emptyDir := filepath.Join(tmp, "empty-path")
	homeDir := filepath.Join(tmp, "home")
	_ = os.MkdirAll(emptyDir, 0o755)
	_ = os.MkdirAll(homeDir, 0o755)

	out := []string{}
	for _, kv := range os.Environ() {
		switch {
		case strings.HasPrefix(kv, "AGENTMIND_BIN="),
			strings.HasPrefix(kv, "PATH="),
			strings.HasPrefix(kv, "HOME="),
			strings.HasPrefix(kv, "OTEL_"),
			strings.HasPrefix(kv, "CLAUDE_CODE_ENABLE_TELEMETRY="),
			strings.HasPrefix(kv, "MINDSPEC_"):
			continue
		}
		out = append(out, kv)
	}
	out = append(out, "PATH="+emptyDir)
	out = append(out, "HOME="+homeDir)
	out = append(out, "AGENTMIND_BIN=")
	return out
}

// repoRootPlain returns the mindspec repository root by walking up
// from this source file's own directory until go.mod names the
// mindspec module — the runtime.Caller-based equivalent of
// docs_truth_skills_test.go's realRepoRoot(t), but error-returning
// (never t.Fatal) because it runs inside buildCmdTree's sync.Once
// body — see that function's doc comment for why.
func repoRootPlain() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller failed")
	}
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..")
	gm := filepath.Join(dir, "go.mod")
	data, err := os.ReadFile(gm)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", gm, err)
	}
	if !strings.Contains(string(data), "module github.com/mrmaxsteel/mindspec") {
		return "", fmt.Errorf("%s does not declare the mindspec module", gm)
	}
	return dir, nil
}

// --- helpers shared with docs_truth_test.go --------------------------
//
// realRepoRoot(t) — the t-taking equivalent of repoRootPlain, used by
// docs_truth_test.go/docs_truth_skills_test.go — already lives in
// docs_truth_skills_test.go; not redefined here.

// dumpTree is a debugging aid (unused by any assertion; kept for
// developer sanity-checking with `go test -run TestDumpCmdTree -v`).
func dumpTree(n *cmdNode, depth int) []string {
	var out []string
	stub := ""
	if n.IsStub {
		stub = " [STUB]"
	}
	out = append(out, strings.Repeat("  ", depth)+n.path()+stub)
	children := append([]*cmdNode{}, n.Children...)
	sort.Slice(children, func(i, j int) bool { return children[i].Name < children[j].Name })
	for _, c := range children {
		out = append(out, dumpTree(c, depth+1)...)
	}
	return out
}

func TestDumpCmdTree(t *testing.T) {
	root, err := buildCmdTree(t)
	if err != nil {
		t.Fatalf("buildCmdTree: %v", err)
	}
	if testing.Verbose() {
		for _, line := range dumpTree(root, 0) {
			t.Log(line)
		}
	}
}

// TestCmdTree_HelpIsLiveNotStub is the regression fixture for the one
// documented, evidence-driven deviation from the ruling's literal
// structural-signal text (see the package doc comment): cobra's own
// `help` command is Run-set/RunE-unset — the AST-era stub shape — but
// is obviously, permanently live. Without the AutoRegistered exclusion
// this file adds to the structural signal, this would be a standing
// two-signal disagreement (a hard failure) on every single run, for a
// command cmd/mindspec never even wrote.
func TestCmdTree_HelpIsLiveNotStub(t *testing.T) {
	root, err := buildCmdTree(t)
	if err != nil {
		t.Fatalf("buildCmdTree: %v", err)
	}
	res := root.resolve([]string{"mindspec", "help", "doctor"})
	if !res.Resolved {
		t.Fatalf("expected mindspec help doctor to resolve live, got unresolved: %s", res.Reason)
	}
	res = root.resolve([]string{"mindspec", "help"})
	if !res.Resolved {
		t.Fatalf("expected mindspec help to resolve live, got unresolved: %s", res.Reason)
	}
}

// TestCmdTree_CompletionAutoCommandsResolve pins the fix that
// motivated B: the AST model reported completion (and its zsh/bash
// children) as nonexistent, because cobra installs them at Execute()
// time, invisible to any static reading of cmd/mindspec's source.
// Reading the real, fully-initialised tree makes them ordinary,
// correctly-live nodes.
func TestCmdTree_CompletionAutoCommandsResolve(t *testing.T) {
	root, err := buildCmdTree(t)
	if err != nil {
		t.Fatalf("buildCmdTree: %v", err)
	}
	for _, words := range [][]string{
		{"mindspec", "completion", "zsh"},
		{"mindspec", "completion", "bash"},
	} {
		if res := root.resolve(words); !res.Resolved {
			t.Errorf("expected %q to resolve, got: %s", joinWords(words), res.Reason)
		}
	}
}

// TestStubSignalDisagreement_IsHardFailure is the RED-on-inject
// regression test for the ruling's core anti-proxy requirement: the
// two signals disagreeing must be a hard failure, never a silent
// tie-break. It builds its OWN throwaway binary (independent of the
// shared buildCmdTree cache — this must not pollute or depend on
// timing with the cached tree's own tempdir lifecycle) and injects a
// FALSE structural claim onto a REAL, known path in each direction,
// observing linkDumpedNode actually append to disagreements against
// the genuine behavioural signal from that real binary:
//
//   - "doctor" is a real, live command (HasRunE set, no
//     DisableFlagParsing) — behaviourally `mindspec doctor --help`
//     exits 0. Injecting HasRun=true/HasRunE=false (a fabricated stub
//     claim) must disagree.
//   - "bench" is a real deprecation stub (deprecated_commands.go) —
//     behaviourally `mindspec bench --help` exits 2. Injecting
//     HasRun=false/HasRunE=true (a fabricated live claim) must
//     disagree.
//   - "doctor" again, this time with DisableFlagParsing left false (a
//     dfp-less stub claim, AutoRegistered also left false since
//     doctor is source-registered) — proving the fix is genuinely
//     independent of dfp: under the OLD (pre-L1-1) formula
//     `HasRun&&!HasRunE&&DisableFlagParsing`, this exact injection
//     would compute structuralStub=false (since dfp=false), agree
//     with doctor's real behavioural signal (also live), and MISS the
//     fabricated claim entirely — the precise bug the final gate
//     found. The current formula (!AutoRegistered&&HasRun&&!HasRunE)
//     ignores dfp and correctly disagrees.
func TestStubSignalDisagreement_IsHardFailure(t *testing.T) {
	binPath, err := buildMindspecBinaryHermetic(t.TempDir())
	if err != nil {
		t.Fatalf("buildMindspecBinaryHermetic: %v", err)
	}

	cases := []struct {
		name    string
		nodeUse string
		fake    cmdTreeDump // HasRun/HasRunE/DisableFlagParsing/AutoRegistered only
	}{
		{"live path falsely claimed stub", "doctor", cmdTreeDump{HasRun: true, HasRunE: false, DisableFlagParsing: true}},
		{"stub path falsely claimed live", "bench", cmdTreeDump{HasRun: false, HasRunE: true, DisableFlagParsing: false}},
		{"dfp-less stub claim on a live path (would have been MISSED by the pre-L1-1 formula)", "doctor", cmdTreeDump{HasRun: true, HasRunE: false, DisableFlagParsing: false}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			child := c.fake
			child.Use = c.nodeUse
			root := &cmdTreeDump{Use: "mindspec", Children: []*cmdTreeDump{&child}}

			var disagreements, probeErrors []string
			linked := linkDumpedNode(root, nil, binPath, t.TempDir(), &disagreements, &probeErrors)

			if len(probeErrors) != 0 {
				t.Fatalf("expected no probe errors (real binary, real path), got: %v", probeErrors)
			}
			found := false
			for _, d := range disagreements {
				if strings.Contains(d, "mindspec "+c.nodeUse+":") {
					found = true
				}
			}
			if !found {
				t.Fatalf("expected a disagreement naming %q, got: %v", c.nodeUse, disagreements)
			}
			// Fail-closed pin (L1-2): a disagreeing node's IsStub must be
			// true, not the false zero-value, so a future regression in
			// escalateTreeProblems degrades to "everything unresolved",
			// never to "everything silently live".
			if len(linked.Children) != 1 || !linked.Children[0].IsStub {
				t.Fatalf("expected the disagreeing node's IsStub to be fail-closed true, got: %+v", linked.Children)
			}
		})
	}
}

// dfpLessStubProbeSource is appended, via `go build -overlay`, to a
// COPY of cmd/mindspec/cmdtree_dump.go's real content — never written
// to the actual repo tree (see buildDfpLessStubProbeBinary) — to
// produce a real binary containing a genuine, Hidden, Run-set/
// RunE-unset, os.Exit(2) command that OMITS DisableFlagParsing: the
// exact shape the final gate's L1-1 probe used (a `zzprobe`-style
// command) to prove the pre-fix discriminator's dfp=false blind spot.
// Appending to cmdtree_dump.go specifically (rather than any other
// production file) is incidental — it already imports cobra and
// "fmt", so only "os" needs adding — and has no effect on the probe:
// the injected command is registered via its own init(), independent
// of anything else in that file.
const dfpLessStubProbeSource = `

// zzDfpLessStubProbeCmd is a synthetic, TEST-ONLY command injected via
// go build -overlay — see TestCmdTree_DfpLessStubIsCaughtByProvenanceSignal
// in internal/lint. It is never part of any binary MindSpec ships.
var zzDfpLessStubProbeCmd = &cobra.Command{
	Use:    "zzprobe-dfpless",
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(os.Stderr, "zzprobe stub message")
		os.Exit(2)
	},
}

func init() {
	rootCmd.AddCommand(zzDfpLessStubProbeCmd)
}
`

// buildDfpLessStubProbeBinary builds a probe binary containing exactly
// the dfp-less stub shape above, WITHOUT writing anything to the real
// repository tree: it reads cmd/mindspec/cmdtree_dump.go's real
// content from disk, appends dfpLessStubProbeSource plus an "os"
// import to a COPY written under tmp, and points `go build -overlay`
// at that copy in place of the real file for this build only. `go
// build -overlay` cannot ADD a new file to a package's file list in
// this Go toolchain (verified empirically: `go list -f
// '{{len .GoFiles}}'` is unchanged when the overlay names a path that
// doesn't already exist on disk) — it CAN replace an EXISTING file's
// content, which is what this does, so `git status` in the real repo
// stays clean throughout.
func buildDfpLessStubProbeBinary(tmp string) (string, error) {
	repoRoot, err := repoRootPlain()
	if err != nil {
		return "", err
	}
	srcPath := filepath.Join(repoRoot, "cmd", "mindspec", "cmdtree_dump.go")
	orig, err := os.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", srcPath, err)
	}
	const importNeedle = "\"encoding/json\"\n\t\"fmt\"\n"
	patched := strings.Replace(string(orig), importNeedle, "\"encoding/json\"\n\t\"fmt\"\n\t\"os\"\n", 1)
	if patched == string(orig) {
		return "", fmt.Errorf("could not locate import block to patch in %s (expected %q)", srcPath, importNeedle)
	}
	patched += dfpLessStubProbeSource

	patchedPath := filepath.Join(tmp, "cmdtree_dump_dfpless_probe.go")
	if err := os.WriteFile(patchedPath, []byte(patched), 0o644); err != nil {
		return "", err
	}
	overlay := struct{ Replace map[string]string }{Replace: map[string]string{srcPath: patchedPath}}
	overlayBytes, err := json.Marshal(overlay)
	if err != nil {
		return "", err
	}
	overlayPath := filepath.Join(tmp, "overlay.json")
	if err := os.WriteFile(overlayPath, overlayBytes, 0o644); err != nil {
		return "", err
	}

	binPath := filepath.Join(tmp, "mindspec-dfpless-probe")
	buildCmd := exec.Command("go", "build", "-overlay", overlayPath, "-o", binPath, "./cmd/mindspec")
	buildCmd.Dir = repoRoot
	var stderr strings.Builder
	buildCmd.Stderr = &stderr
	if err := buildCmd.Run(); err != nil {
		return "", fmt.Errorf("go build -overlay ./cmd/mindspec: %w\nstderr: %s", err, stderr.String())
	}
	return binPath, nil
}

// TestCmdTree_DfpLessStubIsCaughtByProvenanceSignal is the permanent
// regression fixture for final-gate finding L1-1's required change (c):
// build a REAL throwaway binary containing a Run+os.Exit(2) command
// with no DisableFlagParsing, and assert the two-signal cross-check
// records a disagreement for it. This reproduces the exact probe the
// final gate used (a Hidden `zzprobe`-shaped command), end to end,
// through the actual __cmdtree dump + linkDumpedNode pipeline — not a
// fabricated cmdTreeDump struct (that is what
// TestStubSignalDisagreement_IsHardFailure's third case already
// covers, more cheaply; this test additionally proves the REAL
// AutoRegistered computation in cmd/mindspec/cmdtree_dump.go correctly
// marks a genuinely-new, source-added command as NOT auto-registered,
// which a fabricated-struct fixture cannot exercise).
func TestCmdTree_DfpLessStubIsCaughtByProvenanceSignal(t *testing.T) {
	tmp := t.TempDir()
	binPath, err := buildDfpLessStubProbeBinary(tmp)
	if err != nil {
		t.Fatalf("buildDfpLessStubProbeBinary: %v", err)
	}

	// Sanity-check the probe's own real, un-linked behaviour first —
	// this is the exact pair of exit codes the final gate's evidence
	// recorded (--help exits 0 despite the command being a real stub;
	// a bare invocation exits 2), confirming the built probe actually
	// reproduces the reported shape before asking linkDumpedNode
	// anything about it.
	if _, err := runMindspec(binPath, tmp, []string{"zzprobe-dfpless", "--help"}); err != nil {
		t.Fatalf("zzprobe-dfpless --help: expected exit 0 (dfp=false lets cobra intercept --help), got: %v", err)
	}
	if _, err := runMindspec(binPath, tmp, []string{"zzprobe-dfpless"}); err == nil {
		t.Fatal("zzprobe-dfpless (bare): expected a non-zero exit (the stub's Run os.Exit(2)s), got success")
	}

	dumpOut, err := runMindspec(binPath, tmp, []string{"__cmdtree"})
	if err != nil {
		t.Fatalf("%s __cmdtree: %v", binPath, err)
	}
	var dump cmdTreeDump
	if err := json.Unmarshal(dumpOut, &dump); err != nil {
		t.Fatalf("unmarshal __cmdtree output: %v\noutput: %s", err, dumpOut)
	}

	var probeNode *cmdTreeDump
	for _, c := range dump.Children {
		if c.Use == "zzprobe-dfpless" {
			probeNode = c
		}
	}
	if probeNode == nil {
		t.Fatalf("zzprobe-dfpless node not found in __cmdtree dump")
	}
	if probeNode.AutoRegistered {
		t.Fatal("expected zzprobe-dfpless (a real command added by an init(), same as cmd/mindspec's own stubs) to be AutoRegistered=false")
	}
	if probeNode.DisableFlagParsing {
		t.Fatal("test setup error: expected the injected probe to have DisableFlagParsing=false (that is the whole point of this fixture)")
	}

	var disagreements, probeErrors []string
	linkDumpedNode(&dump, nil, binPath, tmp, &disagreements, &probeErrors)
	if len(probeErrors) != 0 {
		t.Fatalf("expected no probe errors, got: %v", probeErrors)
	}
	found := false
	for _, d := range disagreements {
		if strings.Contains(d, "mindspec zzprobe-dfpless:") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a disagreement naming zzprobe-dfpless (structural=stub, behavioural=live via --help), got: %v", disagreements)
	}
}

// TestEscalateTreeProblems_DisagreementIsHardFailure is the regression
// fixture for final-gate finding L1-2: before escalateTreeProblems was
// extracted, the escalation from a collected disagreement to a hard
// build failure was inline in buildCmdTreeOnce and exercised by NO
// fixture — TestStubSignalDisagreement_IsHardFailure only asserted
// that linkDumpedNode appended to the disagreements slice, never that
// a non-empty slice actually aborts anything. This calls
// escalateTreeProblems directly and asserts a non-nil error naming the
// disagreeing path, for both the disagreement and the probe-error
// case.
func TestEscalateTreeProblems_DisagreementIsHardFailure(t *testing.T) {
	err := escalateTreeProblems([]string{"mindspec zzprobe: behavioural(exit-code-derived)=false structural(...)=true"}, nil)
	if err == nil {
		t.Fatal("expected a non-nil error for a non-empty disagreements slice")
	}
	if !strings.Contains(err.Error(), "mindspec zzprobe") {
		t.Fatalf("expected the error to name the disagreeing path, got: %v", err)
	}

	err = escalateTreeProblems(nil, []string{"mindspec bogus: Args(2 args) panicked: boom"})
	if err == nil {
		t.Fatal("expected a non-nil error for a non-empty probeErrors slice")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected the error to surface the probe-error text, got: %v", err)
	}

	if err := escalateTreeProblems(nil, nil); err != nil {
		t.Fatalf("expected nil error for two empty slices, got: %v", err)
	}
}

// TestArgProbeError_IsHardFailure is the analogous regression test for
// the OTHER refuse-to-guess requirement: an Args validator that
// panics or answers non-deterministically must fail loudly
// (probeArgArities, cmd/mindspec/cmdtree_dump.go), never report a
// guessed arity. Exercised directly against the dump's own JSON shape
// (ArgProbeError non-empty), the same field linkDumpedNode checks.
func TestArgProbeError_IsHardFailure(t *testing.T) {
	d := &cmdTreeDump{Use: "mindspec", ArgProbeError: "Args(2 args) panicked: boom"}
	var disagreements, probeErrors []string
	// binPath deliberately does not exist: linkDumpedNode also runs the
	// behavioural stub probe against it, which fails independently (a
	// second, unrelated probeErrors entry) — asserting the SPECIFIC
	// "boom" text below proves the ArgProbeError path itself surfaced,
	// not merely that probeErrors is non-empty for some other reason.
	linkDumpedNode(d, nil, "/nonexistent/mindspec-binary-not-invoked", t.TempDir(), &disagreements, &probeErrors)
	found := false
	for _, e := range probeErrors {
		if strings.Contains(e, "boom") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ArgProbeError text to be surfaced in probeErrors, got: %v", probeErrors)
	}
}
