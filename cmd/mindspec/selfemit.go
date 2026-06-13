package main

// selfemit.go — spec 094 Bead 2 (Req 2 / Req 3 / Req 8): the success-path
// self-emit that turns an escape-hatch admission on a SUCCEEDING leaf
// command into one redacted, isolated journal entry.
//
// Mechanism reality (spec Req 2 / plan DQ2): cobra runs PersistentPostRunE
// ONLY when the leaf command's RunE returns nil (success). Gate-blocked /
// failed commands os.Exit(1) inside RunE BEFORE this hook (e.g.
// complete.go) or return an error (which also skips PostRunE), so they are
// structurally uncapturable — v1 captures SUCCESS-path friction only.
//
// The bound friction signals (plan DQ2, CORRECTED — leaf-local flags, NOT
// root persistent):
//   - --override-adr / --allow-doc-skew / --supersede-adr set (Changed) on
//     the SUCCEEDING leaf (`complete`, `impl approve`, the hidden
//     `approve impl`);
//   - a completed `repair phase`.
//
// MINDSPEC_ALLOW_MAIN is DELIBERATELY NOT bound (plan DQ6 / §Non-Goals): it
// is a raw-git bypass consumed in internal/hook/dispatch.go that never runs
// a capturable leaf, and an ambient os.Getenv check would fire a FALSE
// friction event on every command in any shell that exported it.
//
// Everything here is BEST-EFFORT / NON-FATAL (plan §API Contract): a
// journal error or a redaction drop is swallowed and NEVER returned from
// PersistentPostRunE, so an already-successful, side-effecting command
// (`complete`, `impl approve`) never becomes a post-mutation failure.

import (
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/mrmaxsteel/mindspec/internal/journal"
)

// escapeHatchFlags are the leaf-local override flags whose presence
// (Changed) on a SUCCEEDING command is a friction admission (plan DQ2).
// The map value is the closed-set EscapeHatch enum token persisted in the
// journal (redact.EscapeHatchTokens membership is what keeps the event
// from being dropped). Flag NAME → enum token (the token differs only in
// that the journal/storage contract uses the flag name verbatim here).
var escapeHatchFlags = map[string]string{
	"override-adr":   "override-adr",
	"allow-doc-skew": "allow-doc-skew",
	"supersede-adr":  "supersede-adr",
}

// detectFriction inspects a SUCCEEDING leaf command and returns the bound
// escape-hatch enum token (and true) if it is a v1 friction signal, else
// ("", false). It reads leaf-local override flags via cmd.Flags().Changed
// and recognises a completed `repair phase` by command path.
//
// Only the FIRST bound escape-hatch flag (in a deterministic order) is
// reported — the fingerprint is keyed on a single which-escape-hatch
// token, and an entry records one admission. Order: override-adr,
// allow-doc-skew, supersede-adr (stable, deterministic).
func detectFriction(cmd *cobra.Command) (escapeHatch string, ok bool) {
	// A completed `repair phase` (`mindspec repair phase <spec-id>`) is a
	// friction admission in its own right (the lifecycle needed manual
	// repair). Detect by the leaf+parent identity.
	if cmd.Name() == "phase" && cmd.Parent() != nil && cmd.Parent().Name() == "repair" {
		return "repair-phase", true
	}

	// Leaf-local override flags. Deterministic order so a multi-flag
	// invocation (unusual) records a stable single token.
	for _, name := range []string{"override-adr", "allow-doc-skew", "supersede-adr"} {
		if f := cmd.Flags().Lookup(name); f != nil && cmd.Flags().Changed(name) {
			return escapeHatchFlags[name], true
		}
	}
	return "", false
}

// commandTokens maps a leaf cobra command to the (Command, Subcommand)
// closed-set tokens RedactEvent validates against CommandTokens /
// SubcommandTokens. Command is the TOP-LEVEL command (the child of root);
// Subcommand is the leaf when it is nested below the top-level, else "".
//
// Examples:
//   - `complete`        → Command="complete", Subcommand=""
//   - `impl approve`    → Command="impl",     Subcommand="approve"
//   - `approve impl`    → Command="approve",  Subcommand="impl"
//   - `repair phase`    → Command="repair",   Subcommand="phase"
//
// This mirrors the drift-guard test's useFirstWord mapping (top-level Use
// first-word ∈ CommandTokens; leaf Use first-word ∈ SubcommandTokens), so
// the tokens are guaranteed enum-valid and the event is never dropped for
// an unknown command.
func commandTokens(cmd *cobra.Command) (command, subcommand string) {
	// Walk up to the top-level command (the direct child of root). root has
	// no parent; its children are the top-level commands.
	top := cmd
	for top.Parent() != nil && top.Parent().Parent() != nil {
		top = top.Parent()
	}
	command = top.Name()
	if cmd != top {
		subcommand = cmd.Name()
	}
	return command, subcommand
}

// emitFriction is invoked from PersistentPostRunE on every success. It is
// the load-bearing privacy boundary: a SUCCESS with NO bound friction
// signal (the common case — PersistentPostRunE runs on EVERY success)
// appends NOTHING. It is BEST-EFFORT — any error is swallowed by the
// caller; this function itself returns nothing.
func emitFriction(cmd *cobra.Command) {
	escapeHatch, ok := detectFriction(cmd)
	if !ok {
		// A1 (the privacy boundary): clean success → no journal entry.
		return
	}

	command, subcommand := commandTokens(cmd)

	// Best-effort: a redaction drop or I/O error never fails the command.
	// AppendSuccessEvent is itself fail-closed (a non-classifiable field is
	// dropped, never written raw).
	_ = journal.AppendSuccessEvent(journal.Event{
		Argv0:       argv0(),
		Command:     command,
		EscapeHatch: escapeHatch,
		Subcommand:  subcommand,
		Version:     currentVersion(),
		OS:          runtime.GOOS,
	})
}

// argv0 returns os.Args[0] (reduced to basename + scrubbed inside
// redact.RedactEvent — M3). Split out as a seam so a test can assert the
// stored value is the basename, never the verbatim invocation path.
func argv0() string {
	if len(os.Args) == 0 {
		return ""
	}
	return os.Args[0]
}
