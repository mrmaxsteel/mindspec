package main

// cmd/mindspec/deprecated_commands.go — spec 084 (mindspec-otel-only)
// Bead 3 one-shot deprecation stubs.
//
// Per spec 084 Hard Constraint #7 ("one-shot deprecation messages on
// removed commands") and the per-command migration table at spec
// lines 411-417, invoking any of the removed top-level commands must
// emit exactly one stderr line of the form documented per command and
// exit with code 2. The five stub commands below are registered as
// hidden cobra commands so they do not appear in `mindspec --help`
// (closing Hard Constraint #6) but still run if a user types the old
// name out of muscle memory.
//
// This file is the canonical place under cmd/ or internal/ permitted
// to contain the literal substring `agentmind` outside the permanent
// specgate test introduced in Bead 4, per Hard Constraint #2.
//
// Documented carve-outs (residual `agentmind` substring hits Bead 4's
// AST scan must accept):
//
//  1. The `AGENTMIND_BIN` env-var name in `cmd/mindspec/testhelpers_test.go`
//     (env-var scrubbing for test hermeticity — not an `agentmind`
//     binary invocation, just stripping any inherited value).
//  2. The `agentmind` literal in banned-token lists in
//     `cmd/mindspec/help_golden_test.go` and `cmd/mindspec/record_test.go`
//     (the banlist *is* the assertion; the literal is load-bearing).
//  3. Explanatory comments in `internal/recording/markers.go`,
//     `cmd/mindspec/record.go`, `cmd/mindspec/trace.go`,
//     `cmd/mindspec/otel.go` that reference the historical
//     bench/agentmind subsystems or the sibling agentmind repo
//     (`github.com/mrmaxsteel/agentmind`).
//  4. The `agentmindDeprecatedCmd` Go identifier (here and in
//     `cmd/mindspec/root.go`) — this is an internal-package
//     identifier, not an import path / exec.Command first arg /
//     string literal. Bead 4's AST scan should match only
//     imports, exec.Command first args, and non-test string
//     literals per spec lines 207-209.
//  5. The `001-agentmind-first` test-fixture spec ID in
//     `internal/recording/recording_test.go` (historical fixture
//     name; renaming would churn the test corpus for no semantic
//     gain).
//
// Lifecycle: per spec line 314-318, the deprecation stubs live for
// exactly one mindspec release after spec 084 ships. A single-bead
// follow-up will delete this entire file in the next release.

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// stubDeprecated builds a hidden cobra command that prints the given
// stderr line and exits with code 2. We bypass cobra's RunE/error
// surface for two reasons:
//
//  1. We MUST emit exactly one stderr line. cobra's default error
//     surface would print "Error: <returned-err>\nUsage: ..." on top,
//     producing 4+ lines.
//  2. We MUST exit with code 2 deterministically. RunE-returned
//     errors exit with code 1 by default; os.Exit(2) is the only way
//     to guarantee the documented code.
func stubDeprecated(use, line string) *cobra.Command {
	return &cobra.Command{
		Use:    use,
		Hidden: true,
		// DisableFlagParsing so e.g. `mindspec bench run --spec-id X`
		// doesn't error out on unknown flags before our Run prints
		// the deprecation message.
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(os.Stderr, line)
			os.Exit(2)
		},
	}
}

// Per-command stderr lines per spec 084 plan Bead 3 step 6 (which
// pins them verbatim from spec lines 411-417).
const (
	depBenchMsg          = "bench moved: install agentmind from https://github.com/mrmaxsteel/agentmind (see ADR-0028 for rationale)"
	depAgentmindServeMsg = "agentmind serve moved: install agentmind from https://github.com/mrmaxsteel/agentmind and run 'agentmind serve' (see ADR-0027)"
	depAgentmindReplayMsg = "agentmind replay moved: install agentmind from https://github.com/mrmaxsteel/agentmind and run 'agentmind replay' (see ADR-0027)"
	depVizMsg            = "viz moved: install agentmind from https://github.com/mrmaxsteel/agentmind and run 'agentmind viz' (see ADR-0027)"
	depAgentmindSetupMsg = "agentmind setup renamed: use 'mindspec otel setup' (see ADR-0027 for rationale)"
	depAgentmindGenMsg   = "agentmind moved: install agentmind from https://github.com/mrmaxsteel/agentmind (see ADR-0027)"
)

// agentmindDeprecatedCmd is the parent stub. Direct invocation of
// `mindspec agentmind` (with no subcommand) emits depAgentmindGenMsg
// and exits 2. The serve/replay/setup children below emit their own
// per-command messages.
//
// We use a Run on the parent so cobra does not error with "unknown
// command" before our message can land. Subcommand invocations route
// through the children's Run because cobra dispatches the leaf first.
var agentmindDeprecatedCmd = func() *cobra.Command {
	c := &cobra.Command{
		Use:                "agentmind",
		Hidden:             true,
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(os.Stderr, depAgentmindGenMsg)
			os.Exit(2)
		},
	}
	c.AddCommand(stubDeprecated("serve", depAgentmindServeMsg))
	c.AddCommand(stubDeprecated("replay", depAgentmindReplayMsg))
	c.AddCommand(stubDeprecated("setup", depAgentmindSetupMsg))
	return c
}()

// vizDeprecatedCmd handles the top-level `mindspec viz` alias.
var vizDeprecatedCmd = stubDeprecated("viz", depVizMsg)

// benchDeprecatedCmd handles `mindspec bench …`. DisableFlagParsing on
// the parent catches the subcommand tail as positional args; we don't
// need separate stubs for `bench run`, `bench setup`, etc.
var benchDeprecatedCmd = stubDeprecated("bench", depBenchMsg)
