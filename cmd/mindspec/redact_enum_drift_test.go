package main

// cmd/mindspec/redact_enum_drift_test.go — spec 094 Bead 1 hardening r2
// (repanel-correctness + repanel-codex): a DRIFT GUARD that walks the
// REAL cobra command tree (rootCmd, built in root.go's init via every
// AddCommand) and asserts that every top-level command's Use first-word
// is in redact.CommandTokens AND every leaf subcommand's Use first-word
// is in redact.SubcommandTokens.
//
// This mirrors the runtime.GOOS init drift-guard in internal/redact:
// because internal/redact CANNOT import package main, the lockstep
// invariant ("the first word of every child cobra command's Use string"
// is an allowlist member) can only be enforced from THIS package, where
// rootCmd is in scope. A future-added subcommand whose first-word is not
// in SubcommandTokens FAILS THE BUILD here, instead of silently dropping
// every event that carries it (the repanel-correctness event-loss bug:
// `impl`/`spec`/`bead` were missing, so `approve impl --override-adr`
// — Bead 2's primary override-on-impl capture — was silently dropped).
//
// The walk is recursive (every depth), so it would also catch a missing
// grandchild token; only the root command itself ("mindspec") is exempt.

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/mrmaxsteel/mindspec/internal/redact"
)

// useFirstWord returns the first whitespace-delimited word of a cobra
// command's Use string (e.g. "impl [spec-id]" → "impl"), which is the
// token RedactEvent validates against its allowlist.
func useFirstWord(use string) string {
	use = strings.TrimSpace(use)
	if i := strings.IndexAny(use, " \t"); i >= 0 {
		return use[:i]
	}
	return use
}

// TestRedactEnum_NoCobraDrift walks the live rootCmd tree and asserts the
// redact allowlists are a SUPERSET of every command/subcommand first-word
// the binary can actually dispatch. A missing token would make
// RedactEvent silently DROP that command's friction events.
func TestRedactEnum_NoCobraDrift(t *testing.T) {
	// Force cobra's lazy built-ins (`help`, `completion`) into the tree so
	// the walk is DETERMINISTIC regardless of test ordering: these are
	// always registered in the real binary (not disabled), are dispatchable
	// top-level commands, and so are legitimately emittable — the allowlist
	// must cover them. Without this, the guard would only see them if some
	// other test had already triggered Execute(), making the check flaky.
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultCompletionCmd()

	// Sanity: the tree must be populated (init ran AddCommand). If this is
	// empty the walk would vacuously pass and hide real drift.
	if len(rootCmd.Commands()) == 0 {
		t.Fatal("rootCmd has no subcommands — init/AddCommand did not run; the drift guard would be vacuous")
	}

	var topLevelSeen, subSeen int
	for _, top := range rootCmd.Commands() {
		topWord := useFirstWord(top.Use)
		topLevelSeen++
		if _, ok := redact.CommandTokens[topWord]; !ok {
			t.Errorf("top-level command %q (Use %q) is NOT in redact.CommandTokens — its success/friction events would be silently DROPPED; add it (keep the allowlist in lockstep with cmd/mindspec)", topWord, top.Use)
		}
		// Recurse into the whole subtree; every descendant's first-word must
		// be a SubcommandTokens member (RedactEvent validates Subcommand
		// against that set regardless of nesting depth).
		walkSubcommands(t, top, &subSeen)
	}

	t.Logf("drift guard walked the real cobra tree: %d top-level commands, %d (sub)commands checked", topLevelSeen, subSeen)
	if subSeen == 0 {
		t.Error("walked zero subcommands — the tree walk is not exercising SubcommandTokens (guard would be vacuous for the repanel-correctness gap)")
	}
}

func walkSubcommands(t *testing.T, parent *cobra.Command, subSeen *int) {
	t.Helper()
	for _, child := range parent.Commands() {
		word := useFirstWord(child.Use)
		*subSeen++
		if _, ok := redact.SubcommandTokens[word]; !ok {
			t.Errorf("subcommand %q (parent %q, Use %q) is NOT in redact.SubcommandTokens — RedactEvent would silently DROP its events; add it (lockstep with cmd/mindspec)", word, useFirstWord(parent.Use), child.Use)
		}
		walkSubcommands(t, child, subSeen)
	}
}

// TestRedactEnum_PrimaryCapturePaths is the targeted repanel-correctness
// regression: the three previously-missing leaf subcommands — `impl`
// (approve impl, Bead 2's override-on-impl capture), `spec` (approve/bead/
// validate spec), `bead` (context bead) — must now be KEPT by RedactEvent,
// not dropped.
func TestRedactEnum_PrimaryCapturePaths(t *testing.T) {
	keeps := []redact.Event{
		{Command: "approve", Subcommand: "impl", OS: "darwin", Version: "dev", EscapeHatch: "override-adr"},
		{Command: "bead", Subcommand: "spec", OS: "linux", Version: "dev"},
		{Command: "context", Subcommand: "bead", OS: "windows", Version: "dev"},
		{Command: "validate", Subcommand: "spec", OS: "darwin", Version: "dev"},
	}
	for _, ev := range keeps {
		if _, ok := redact.RedactEvent(ev); !ok {
			t.Errorf("legitimate event %+v was DROPPED — SubcommandTokens still missing %q", ev, ev.Subcommand)
		}
	}
}
