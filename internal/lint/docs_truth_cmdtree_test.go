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
// answered with TWO INDEPENDENT signals that MUST AGREE — disagreement
// is a hard test failure, never a silent tie-break, because every
// prior (AST-only) version failed by trusting a single signal:
//
//   - BEHAVIOURAL: the exit code of `<binary> <path...> --help`, run
//     with explicit argv (probeBehavioralStub). A genuine stub sets
//     DisableFlagParsing, so cobra never intercepts `--help` as a
//     flag — it falls straight through to the stub's Run, which
//     unconditionally prints one line and os.Exit(2)s. A live command
//     has flag parsing enabled, so cobra intercepts `--help` centrally
//     and returns before Run/RunE ever runs, exiting 0.
//   - STRUCTURAL: the dump's HasRun && !HasRunE && DisableFlagParsing
//     (structuralStub). The DisableFlagParsing conjunct is this file's
//     one addition beyond the ruling's literal text — see below.
//
// ## Why DisableFlagParsing had to join the structural signal
//
// The ruling's stub_signal section specifies the structural half as
// bare `Run != nil && RunE == nil`, reasoned to become vacuous (never
// wrong) once deprecated_commands.go is eventually deleted, "because
// neither signal is a list". That reasoning implicitly assumed every
// Run-set/RunE-unset node comes from cmd/mindspec's own stub
// constructor — true within cmd/mindspec's OWN source, but the
// judge's 93-path measurement was taken against the AST tree, which
// never contained cobra's auto-registered `help` command (AST
// couldn't see it — see above). Once B makes `help` a real node
// (exactly the divergence B exists to fix), it is a PERMANENT,
// unavoidable counterexample to the bare rule: cobra's own
// InitDefaultHelpCmd (spf13/cobra@v1.8.1 command.go:1262) sets `Run`,
// never `RunE`, for every cobra program that has ever existed — and
// `help` is obviously, permanently live (verified:
// `mindspec help --help` exits 0). Bare Run-set/RunE-unset therefore
// structurally disagrees with the behavioural signal for `help`
// forever, which would make this lint permanently unbuildable if the
// two signals must agree unconditionally.
//
// DisableFlagParsing is what actually explains the correlation the
// ruling is trying to capture: a stub's exit-2-on-`--help` behaviour
// depends on DisableFlagParsing, not on Run-vs-RunE per se (RunE could
// just as well be used by a hypothetical future stub). Every one of
// cmd/mindspec's six real stubs sets it; cobra's `help` does not
// (verified below, TestCmdTree_HelpIsLiveNotStub). This is a narrow,
// evidence-driven repair of an unmeasured gap in the ruling's literal
// text — escalated to, and APPROVED by, the bead author (mindspec-ng3g)
// per the ruling's own sign-off instruction, with one required
// refinement recorded in the three points below.
//
// IMPORTANT, per that sign-off: DisableFlagParsing is a CORRELATE, not
// the property. It explains WHY a stub happens to exit 2 on `--help`
// today; it does not define what a stub IS. Its use here is safe for
// exactly one reason, and the reasoning must not be separated from it:
//
//  1. It is the STRUCTURAL half of a two-signal check, cross-checked
//     against an INDEPENDENT behavioural signal (the real exit code).
//  2. Safety rests entirely on that cross-check. If a future stub is
//     ever built WITHOUT setting DisableFlagParsing, the structural
//     signal reports "live" while the behavioural signal (still
//     exit 2, because the stub's Run still os.Exit(2)s once cobra's
//     flag parsing lets it run) reports "stub" — disagreement, and the
//     build HARD-FAILS LOUDLY. That is the correct, safe direction: a
//     loud break, not a silent misclassification.
//  3. Consequently, THE STRUCTURAL SIGNAL MUST NEVER BE USED ALONE.
//     Collapsing to bare "HasRun && !HasRunE && DisableFlagParsing"
//     with no live behavioural cross-check would be, verbatim, wrong
//     version #3 from the ruling's own history ("Hidden &&
//     DisableFlagParsing... a coincidence of ONE helper's
//     implementation, not a contract any other stub is bound to") —
//     this is discriminator attempt SIX, and the only reason it is not
//     also wrong is the cross-check in point 1. A future maintainer
//     tempted to "simplify" by deleting the behavioural probe and
//     trusting the structural signal alone would silently resurrect
//     that exact, already-rejected failure mode.
//
// A cleaner alternative EXISTS and was NOT taken: exclude cobra's
// auto-registered builtins (`help`, `completion` and its children) by
// PROVENANCE — they come from InitDefaultHelpCmd/InitDefaultCompletionCmd,
// never from cmd/mindspec's own source — which would let the structural
// signal stay the ruling's literal bare `HasRun && !HasRunE` with no
// correlate field needed at all. That is more principled in the
// abstract (no reliance on a field that merely correlates); it was not
// implemented here because distinguishing "cobra-authored" from
// "cmd/mindspec-authored" from OUTSIDE the cobra package, at dump time,
// has no clean signal of its own (cobra does not expose command
// provenance), and the two-signal design this ruling already mandates
// makes the correlate-plus-cross-check approach above sufficient and
// no less safe. Recorded here for whichever future author next touches
// this discriminator.
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
// signal (now DisableFlagParsing-qualified) still degrades correctly:
// no node in the remaining source sets DisableFlagParsing at all, so
// no node is ever marked stub, `--help` behaviourally returns 0
// everywhere, the two signals still agree ("no stubs"), and R5 simply
// never fires — the exact vacuous-not-wrong degradation the ruling
// requires, just anchored on the field that actually causes it.
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

// hasFlag reports whether name (a long flag name, or a single-char
// shorthand) is visible at n. No ancestor walk is needed: the dump
// already resolved inheritance transitively via cobra's own
// InheritedFlags(), which itself walks the FULL ancestor chain (cobra
// updateParentsPflags visits every parent, not just the immediate
// one), so n.Flags already contains everything n can see.
func (n *cmdNode) hasFlag(name string) bool {
	_, ok := n.Flags[name]
	return ok
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
func (root *cmdNode) resolve(words []string) resolveResult {
	if len(words) == 0 {
		return resolveResult{Resolved: false, Reason: "empty invocation"}
	}
	if words[0] != root.Name {
		return resolveResult{Resolved: false, Reason: fmt.Sprintf("root name mismatch: %q", words[0])}
	}
	cur := root
	i := 1
	for i < len(words) {
		w := words[i]
		if strings.HasPrefix(w, "-") {
			break
		}
		if len(cur.Children) == 0 {
			break // leaf command: remaining words are positional args
		}
		child := cur.findChild(w)
		if child == nil {
			return resolveResult{Resolved: false, Reason: fmt.Sprintf("no subcommand %q under %q", w, cur.path())}
		}
		cur = child
		i++
	}
	if cur.IsStub {
		return resolveResult{Resolved: false, Reason: fmt.Sprintf("%q resolves to a one-shot deprecation stub (Run, not RunE)", cur.path())}
	}
	// Single pass over the trailing words: validate every --flag/short
	// flag against cur, and collect whatever is left as candidate
	// positionals for the ArgMax check below. A long flag without an
	// inline `--name=value` is assumed to consume the NEXT word as its
	// value UNLESS its registered Type is "bool" (a bool flag takes no
	// value at all — this is the fidelity win real flag-type data gives
	// over the AST model's blind "always skip" guess, O2-r2-4).
	var positionals []string
	skipNextAsFlagValue := false
	for _, w := range words[i:] {
		if skipNextAsFlagValue {
			skipNextAsFlagValue = false
			continue
		}
		switch {
		case strings.HasPrefix(w, "--"):
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
				skipNextAsFlagValue = true
			}
		case len(w) == 2 && w[0] == '-' && w != "--":
			name := w[1:]
			if !cur.hasFlag(name) {
				return resolveResult{Resolved: false, Reason: fmt.Sprintf("flag -%s not registered on %q", name, cur.path())}
			}
		default:
			positionals = append(positionals, w)
		}
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
	if len(probeErrors) > 0 {
		return nil, fmt.Errorf("Args validator probe failure(s), refusing to guess:\n  %s", strings.Join(probeErrors, "\n  "))
	}
	if len(disagreements) > 0 {
		return nil, fmt.Errorf("stub-signal disagreement(s) between behavioural and structural checks — this is ALWAYS a hard failure, never a silent tie-break:\n  %s", strings.Join(disagreements, "\n  "))
	}
	if root.Name != "mindspec" {
		return nil, fmt.Errorf(`__cmdtree root Use %q does not start with "mindspec"`, dump.Use)
	}
	return root, nil
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

	structuralStub := d.HasRun && !d.HasRunE && d.DisableFlagParsing
	behavioralStub, err := probeBehavioralStub(binPath, tmp, pathWordsFromNode(n))
	if err != nil {
		*probeErrors = append(*probeErrors, fmt.Sprintf("%s: behavioural stub probe: %v", n.path(), err))
	} else if behavioralStub != structuralStub {
		*disagreements = append(*disagreements, fmt.Sprintf("%s: behavioural(exit-code-derived)=%v structural(hasRun&&!hasRunE&&disableFlagParsing)=%v", n.path(), behavioralStub, structuralStub))
	} else {
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
func runMindspec(binPath, tmp string, args []string) ([]byte, error) {
	cmd := exec.Command(binPath, args...)
	cmd.Env = stripCmdTreeEnv(tmp)
	return cmd.Output()
}

// stripCmdTreeEnv builds a hermetic environment for building/execing
// the mindspec binary this file introspects — the SAME scrubbing
// shape cmd/mindspec/testhelpers_test.go's strippedEnv already
// establishes and exercises in CI (spec 084 Bead 2/3): no inherited
// AGENTMIND_BIN, an empty PATH (via emptyDir), a fresh HOME (so config
// probes can't pick up developer-host state), and no OTEL_*/
// CLAUDE_CODE_ENABLE_TELEMETRY leakage. Duplicated rather than
// imported: cmd/mindspec/testhelpers_test.go is a _test.go file in a
// different package (main), and Go does not allow importing another
// package's test-only sources — the binding ruling's mandate to REUSE
// this shape (not invent a second, subtly different hermetic-env
// idiom) is honored by copying the exact same env-var list, not by a
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
			strings.HasPrefix(kv, "CLAUDE_CODE_ENABLE_TELEMETRY="):
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
// is obviously, permanently live. Without the DisableFlagParsing
// conjunct this file adds to the structural signal, this would be a
// standing two-signal disagreement (a hard failure) on every single
// run, for a command cmd/mindspec never even wrote.
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
//     exits 0. Injecting HasRun=true/HasRunE=false/
//     DisableFlagParsing=true (a fabricated stub claim) must disagree.
//   - "bench" is a real deprecation stub (deprecated_commands.go) —
//     behaviourally `mindspec bench --help` exits 2. Injecting
//     HasRun=false/HasRunE=true (a fabricated live claim) must
//     disagree.
func TestStubSignalDisagreement_IsHardFailure(t *testing.T) {
	binPath, err := buildMindspecBinaryHermetic(t.TempDir())
	if err != nil {
		t.Fatalf("buildMindspecBinaryHermetic: %v", err)
	}

	cases := []struct {
		name    string
		nodeUse string
		fake    cmdTreeDump // HasRun/HasRunE/DisableFlagParsing only
	}{
		{"live path falsely claimed stub", "doctor", cmdTreeDump{HasRun: true, HasRunE: false, DisableFlagParsing: true}},
		{"stub path falsely claimed live", "bench", cmdTreeDump{HasRun: false, HasRunE: true, DisableFlagParsing: false}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			child := c.fake
			child.Use = c.nodeUse
			root := &cmdTreeDump{Use: "mindspec", Children: []*cmdTreeDump{&child}}

			var disagreements, probeErrors []string
			linkDumpedNode(root, nil, binPath, t.TempDir(), &disagreements, &probeErrors)

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
		})
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
