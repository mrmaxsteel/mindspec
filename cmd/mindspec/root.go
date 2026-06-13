package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/trace"
	versionpkg "github.com/mrmaxsteel/mindspec/internal/version"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

// approvalGatesBlock lists the canonical noun-verb approval-gate
// commands with their one-line phase descriptions (spec 092 Req 10).
// It is surfaced in three places: the root help "Approval Gates"
// section (usage-template suffix, init below), the root
// unknown-command error for approve-like near-misses
// (rootUnknownCommandError), and the hidden deprecated `approve`
// alias's error text (cmd/mindspec/approve.go).
const approvalGatesBlock = `  mindspec spec approve <id>   Approve a spec and transition to Plan Mode
  mindspec plan approve <id>   Approve a plan and transition toward Implementation Mode
  mindspec impl approve <id>   Approve implementation and transition to idle`

// newExecutor creates a MindspecExecutor rooted at the given path.
// Used as a factory function across CLI commands.
func newExecutor(root string) executor.Executor {
	return executor.NewMindspecExecutor(root)
}

// Set by goreleaser ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// currentVersion returns the bare semver stamped on every journal entry
// (spec 094 Req 3). It reads the internal/version single source of truth
// (version.Current()), which Bead 1 documents as the value Set() from
// main's ldflags-injected `version` var at startup (init below) — NOT the
// decorated cobra --version string (whose commit hash the entropy scrub
// would eat). A non-release/local/test build returns "dev".
func currentVersion() string {
	return versionpkg.Current()
}

var cmdStartTime time.Time

var rootCmd = &cobra.Command{
	Use:     "mindspec",
	Short:   "MindSpec: Spec-Driven Development and Self-Documentation System",
	Long:    `MindSpec is a spec-driven development + context management framework.`,
	Version: version + " (" + commit + ") " + date,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		cmdStartTime = time.Now()

		// Resolve trace path: flag takes precedence, then env var.
		tracePath, _ := cmd.Flags().GetString("trace")
		if tracePath == "" {
			tracePath = os.Getenv("MINDSPEC_TRACE")
		}
		if tracePath != "" {
			if err := trace.Init(tracePath); err != nil {
				return err
			}
		}

		trace.Emit(trace.NewEvent("command.start").WithData(map[string]any{
			"command": cmd.Name(),
			"args":    args,
		}))
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		trace.Emit(trace.NewEvent("command.end").
			WithDuration(time.Since(cmdStartTime)).
			WithData(map[string]any{
				"command": cmd.Name(),
			}))

		// Spec 094 Bead 2 (Req 2): success-path self-emit. cobra runs
		// PersistentPostRunE ONLY on RunE success, so reaching here means the
		// leaf command SUCCEEDED. emitFriction appends exactly one redacted,
		// isolated journal entry IFF the leaf used a bound escape-hatch flag
		// or is a completed `repair phase`; a clean success appends nothing
		// (the always-on privacy boundary, A1). It is BEST-EFFORT / NON-FATAL
		// — any journal error or redaction drop is swallowed inside
		// emitFriction and never returned, so a successful side-effecting
		// command (complete, impl approve) never becomes a post-mutation
		// failure. The opt-in --trace/MINDSPEC_TRACE path is untouched
		// (ADR-0027); journaling is always-on and independent of it.
		emitFriction(cmd)

		return trace.Close()
	},
	// Spec 092 Req 10b: Args+RunE replace cobra's legacyArgs
	// unknown-command error so approve-like near-misses (e.g.
	// `mindspec aprove impl`) surface the canonical noun-verb gate
	// commands. ArbitraryArgs suppresses the legacy error in Find;
	// RunE reproduces it (plus the gates block) and keeps bare
	// `mindspec` printing help.
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		return rootUnknownCommandError(cmd, args[0])
	},
}

// rootUnknownCommandError reproduces cobra's root-level
// unknown-command error (skipped because rootCmd.Args is set) and, for
// approve-like near-misses, appends the canonical approval-gate
// commands (spec 092 Req 10b).
func rootUnknownCommandError(cmd *cobra.Command, name string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "unknown command %q for %q", name, cmd.CommandPath())
	if suggestions := cmd.SuggestionsFor(name); len(suggestions) > 0 {
		b.WriteString("\n\nDid you mean this?\n")
		for _, s := range suggestions {
			fmt.Fprintf(&b, "\t%v\n", s)
		}
	}
	if isApproveNearMiss(name) {
		b.WriteString("\n\nApproval gates use the noun-verb order:\n")
		b.WriteString(approvalGatesBlock)
	}
	return errors.New(b.String())
}

// isApproveNearMiss reports whether name is a near-miss spelling of
// the deprecated `approve` verb (spec 092 Req 10b). The exact spelling
// "approve" never reaches this path — it resolves to the hidden
// backward-compat alias in cmd/mindspec/approve.go.
func isApproveNearMiss(name string) bool {
	return levenshtein(strings.ToLower(name), "approve") <= 2
}

// levenshtein returns the edit distance between a and b. cobra's own
// implementation is unexported, so the near-miss check carries its
// own (two-row dynamic programming, case-sensitive).
func levenshtein(a, b string) int {
	ar, br := []rune(a), []rune(b)
	prev := make([]int, len(br)+1)
	cur := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		cur[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			cur[j] = min(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(br)]
}

func findRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot determine working directory: %w", err)
	}
	root, err := workspace.FindRoot(cwd)
	if err != nil {
		return "", fmt.Errorf("workspace not found: %w", err)
	}
	return root, nil
}

func findLocalRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot determine working directory: %w", err)
	}
	root, err := workspace.FindLocalRoot(cwd)
	if err != nil {
		return "", fmt.Errorf("workspace not found: %w", err)
	}
	return root, nil
}

func init() {
	// Spec 094 Bead 2 (Req 3): wire the build's bare semver (the
	// ldflags-injected `version` var, root.go:35 default "dev") into the
	// internal/version single source of truth so version.Current() — stamped
	// on every friction journal entry — is the build's real semver in
	// release builds, not "dev". Set ignores a blank arg and is idempotent;
	// the decorated cobra --version string (version+commit+date) is NOT used
	// (its commit hash is exactly what the redaction entropy scrub eats).
	versionpkg.Set(version)

	rootCmd.PersistentFlags().String("trace", "", "Write trace events to file (use - for stderr)")

	// Spec 092 Req 10a: root help gains an "Approval Gates" section
	// listing the canonical noun-verb gate commands. The
	// {{if not .HasParent}} guard keeps the section off subcommand
	// help (children inherit the root usage template).
	rootCmd.SetUsageTemplate(rootCmd.UsageTemplate() +
		"{{if not .HasParent}}\nApproval Gates:\n" + approvalGatesBlock + "\n{{end}}")

	// Spec 092 Req 10b: near-miss spellings surface the canonical
	// noun-verb form (used by SuggestionsFor in
	// rootUnknownCommandError). 2 is cobra's default, pinned
	// explicitly per the spec.
	rootCmd.SuggestionsMinimumDistance = 2

	rootCmd.AddCommand(adrCmd)
	rootCmd.AddCommand(cleanupCmd)
	rootCmd.AddCommand(approveCmd) // hidden backward-compat alias
	rootCmd.AddCommand(beadCmd)
	rootCmd.AddCommand(completeCmd)
	rootCmd.AddCommand(contextCmd)
	rootCmd.AddCommand(domainCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(implCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(instructCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(nextCmd)
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(repairCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(specCmd)
	rootCmd.AddCommand(specInitCmd) // hidden backward-compat alias
	rootCmd.AddCommand(hookCmd)
	rootCmd.AddCommand(stateCmd)
	rootCmd.AddCommand(traceCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(otelCmd)
	rootCmd.AddCommand(recordCmd)

	// Spec 094 Bead 3 (Req 4 / Req 5): the owner-invoked report loop —
	// `mindspec report` consolidates the friction journal into the local,
	// non-synced report store; `report list` is the triage view.
	rootCmd.AddCommand(reportCmd)

	// Spec 084 Bead 3: register one-shot deprecation stubs for removed
	// top-level commands (HC #7). Each stub is hidden and exits 2 with
	// exactly one stderr line. See cmd/mindspec/deprecated_commands.go.
	rootCmd.AddCommand(agentmindDeprecatedCmd)
	rootCmd.AddCommand(vizDeprecatedCmd)
	rootCmd.AddCommand(benchDeprecatedCmd)
}
