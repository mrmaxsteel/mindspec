package main

// report.go — spec 094 Bead 3 (Req 4 / Req 5 / Req 7 / Req 3 regression-stale
// loop; HC-3 / HC-4 / HC-6 / HC-7 / HC-8): the owner-invoked REPORT LOOP that
// closes the self-improvement cycle.
//
//   - `mindspec report`        — consolidate the append-only journal.jsonl
//     into reports.jsonl (the §Storage Contract 2-file design), print a
//     summary. v1 is OWNER-LOCAL: it writes the local report store only and
//     attempts NO remote push (the owner's remote push is deferred to the
//     follow-on; the feedback-remote contract from Bead 4 is consulted only
//     to confirm no push is permitted by default). In CI (GITHUB_ACTIONS),
//     it is a no-op beyond the journal (HC-6).
//   - `mindspec report list`   — a triage view over reports.jsonl (NOT bd):
//     fingerprint, command, escape-hatch, count, version range, and the
//     derived open/regression/stale status (Req 5 / Req 3). With
//     --resolve <fingerprint> [--version vX] it marks a report resolved.
//
// # Untrusted-corpus render (Req 7 / HC-4)
//
// The friction store holds only closed-set enum tokens + fingerprint +
// version (Bead 2 journals enums-only), so most fields are structurally safe.
// But Req 7 binds the RENDER surface: every field this command prints is run
// through a defense-in-depth render scrub (renderField) that scrubs via
// internal/redact, neutralises markdown auto-linking, strips control/newline
// characters so no injected `recovery:` line can be auto-executed, and
// length-caps. The output is code-fenced. This holds even though v1 emits no
// free-text field — it is the explicit backstop the spec asks for, so a future
// free-text field cannot regress the render contract.
//
// # Store isolation (HC-3) — NO bd/dolt/git egress
//
// Both files live under journal.Dir() (the machine-global, non-synced,
// git-tree-guarded 0600 store). This command NEVER writes via bd, NEVER
// touches .beads/issues.jsonl, and NEVER enters a bd/dolt/git path (Req 4 /
// ADR-0023). The store-isolation egress proof asserts the fingerprint is
// absent from every surface `bd dolt push` would send.

import (
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/journal"
	"github.com/mrmaxsteel/mindspec/internal/redact"
)

// inCI reports whether the process is running in a non-interactive CI
// environment (HC-6): with GITHUB_ACTIONS set, `mindspec report` is a no-op
// beyond the journal — no report write, no prompt, no push. Read via env
// only (the same agent-proof channel ADR-0037 uses for its gate).
func inCI() bool {
	return os.Getenv("GITHUB_ACTIONS") != ""
}

// reportCmd is the registered singleton (root.go init). Tests build fresh
// instances via newReportCmd() so per-Execute flag state never leaks across
// invocations of a shared cobra singleton.
var reportCmd = newReportCmd()

// newReportCmd builds a fresh `report` command (with its `list` subcommand).
// Used both for registration and by tests for isolation.
func newReportCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "report",
		Short: "Consolidate the friction journal into a local friction report",
		Long: `Consolidate the always-on friction journal into the local, non-synced
friction report store (reports.jsonl), collapsing events by fingerprint.

v1 is OWNER-LOCAL: report writes the local report store only and attempts NO
remote push (the owner's cross-install push is deferred). It NEVER writes to
the beads tracker, .beads/issues.jsonl, or any bd/dolt/git path — the friction
store is isolated from the shared remote (HC-3).

In CI (GITHUB_ACTIONS set) report is a no-op beyond the journal (HC-6).`,
		Args: cobra.NoArgs,
		RunE: runReport,
	}
	c.AddCommand(newReportListCmd())
	return c
}

// newReportListCmd builds a fresh `report list` command.
func newReportListCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "list",
		Short: "Triage the consolidated friction reports",
		Long: `List the consolidated friction reports (read from the local report store,
NOT the beads tracker), showing each report's fingerprint, command,
escape-hatch, occurrence count, version range, and triage status
(open / regression / stale).

The FINGERPRINT column prints the FULL fingerprint — copy it verbatim into
--resolve. Mark a report resolved with:
  mindspec report list --resolve <fingerprint> [--version <vX>]

A report resolved at version X that recurs at a running version >= X is a
REGRESSION; a recurrence at < X is stale (suppressed). A dev/unparseable
version is treated as unbounded-newest (fails toward surfacing a regression).`,
		Args: cobra.NoArgs,
		RunE: runReportList,
	}
	c.Flags().String("resolve", "", "Mark the report with this fingerprint resolved")
	c.Flags().String("version", "", "The resolved-in version for --resolve — a concrete semver or the current build version (defaults to the current build version; any other value is rejected)")
	return c
}

// runReport consolidates the journal into reports.jsonl and prints a summary.
// HC-6: a no-op beyond the journal in CI. HC-3: never a bd/dolt/git write.
func runReport(cmd *cobra.Command, _ []string) error {
	out := cmd.OutOrStdout()

	if inCI() {
		// HC-6: in CI, report is a no-op beyond the journal — no report write,
		// no prompt, no push.
		fmt.Fprintln(out, "report: CI detected (GITHUB_ACTIONS) — no-op beyond the journal (HC-6).")
		return nil
	}

	// v1 owner-local: confirm no remote push is permitted by default (Bead 4's
	// fail-closed contract). We NEVER push regardless; this resolves WHETHER a
	// push would be permitted purely so the summary can state it, and proves
	// `report` makes no network call (the resolver only reads local config).
	globalDir, gerr := journal.Dir()
	if gerr == nil {
		if target, terr := config.ResolveFeedbackRemote(globalDir); terr == nil && target.CanPush {
			// Even with a credential, v1 defers the push — say so, do nothing.
			fmt.Fprintln(out, "report: a feedback-remote is configured, but v1 is owner-local — the remote push is deferred (no push attempted).")
		}
	}

	reports, err := journal.Consolidate()
	if err != nil {
		return fmt.Errorf("report: consolidating the friction journal: %w", err)
	}

	if len(reports) == 0 {
		// Empty / no-journal is a clean message, never an error.
		fmt.Fprintln(out, "report: no friction events recorded yet — nothing to consolidate.")
		return nil
	}

	if err := journal.WriteReports(reports); err != nil {
		return fmt.Errorf("report: writing the consolidated report store: %w", err)
	}

	path, _ := journal.ReportsPath()
	total := 0
	for _, r := range reports {
		total += r.Count
	}
	fmt.Fprintf(out, "report: consolidated %d friction event(s) into %d report(s) at %s\n",
		total, len(reports), renderField(path))
	fmt.Fprintln(out, "Run `mindspec report list` to triage.")
	return nil
}

// runReportList prints the triage view over reports.jsonl, OR resolves a
// report when --resolve is given. Reads the friction store, never bd.
func runReportList(cmd *cobra.Command, _ []string) error {
	out := cmd.OutOrStdout()

	if fp, _ := cmd.Flags().GetString("resolve"); fp != "" {
		ver, _ := cmd.Flags().GetString("version")
		if ver == "" {
			ver = currentVersion() // the running build's bare semver (or "dev")
		}
		if err := journal.MarkResolved(fp, ver); err != nil {
			return fmt.Errorf("report list: %w", err)
		}
		fmt.Fprintf(out, "report list: marked %s resolved_in %s\n",
			renderField(fp), renderField(ver))
		return nil
	}

	reports, err := journal.ReadReports()
	if err != nil {
		return fmt.Errorf("report list: reading the friction report store: %w", err)
	}
	if len(reports) == 0 {
		// Empty / never-consolidated is a clean message, never an error.
		fmt.Fprintln(out, "report list: no friction reports — run `mindspec report` first.")
		return nil
	}

	// Render inside a code fence (Req 7 / HC-4): the body is untrusted by
	// contract even though it is enum-only, so fence it so no consumer
	// auto-links or auto-executes a rendered line.
	fmt.Fprintln(out, "```")
	// The FINGERPRINT column is full-width (64 hex) so the printed identifier IS
	// the one `--resolve` accepts (codex-completeness #3) — no truncated prefix.
	fmt.Fprintf(out, "%-64s  %-10s  %-14s  %-12s  %5s  %-16s  %s\n",
		"FINGERPRINT", "COMMAND", "ESCAPE-HATCH", "STATUS", "COUNT", "VERSION-RANGE", "RESOLVED-IN")
	for _, r := range reports {
		status := r.Classify()
		versionRange := renderField(r.FirstVersion)
		if r.LastVersion != r.FirstVersion {
			versionRange = renderField(r.FirstVersion) + ".." + renderField(r.LastVersion)
		}
		resolved := "-"
		if r.ResolvedInVersion != "" {
			resolved = renderField(r.ResolvedInVersion)
		}
		fmt.Fprintf(out, "%-64s  %-10s  %-14s  %-12s  %5d  %-16s  %s\n",
			renderFingerprint(r.Fingerprint),
			renderField(r.Command),
			renderField(r.EscapeHatch),
			string(status),
			r.Count,
			versionRange,
			resolved,
		)
	}
	fmt.Fprintln(out, "```")
	fmt.Fprintln(out, "Resolve a report: mindspec report list --resolve <fingerprint> [--version <vX>]")
	return nil
}

// maxRenderField is the per-field render length cap (Req 7 / HC-4). Enum
// tokens are short; the cap is the defense-in-depth backstop for any future
// free-text field that could surface here.
const maxRenderField = 120

// renderField applies the untrusted-corpus RENDER rules (Req 7 / HC-4) to a
// single field the report command prints. Even though the store is enum-only,
// the render surface treats every value as untrusted (the spec binds Req 7 to
// THIS surface):
//
//   - scrub via internal/redact (a residual-leak field DROPS to a placeholder,
//     never the raw value — HC-7);
//   - strip control/newline characters so no injected `recovery:` line can be
//     reconstituted and auto-executed by a downstream agent;
//   - neutralise markdown auto-linking by inserting a zero-width-safe break in
//     a `](` / `://` sequence so a pasted body cannot become a live link;
//   - length-cap.
func renderField(s string) string {
	if s == "" {
		return ""
	}
	clean, ok := redact.Scrub(s)
	if !ok {
		// Fail-closed: an unclassifiable value never renders raw.
		return "<redacted>"
	}
	clean = stripControl(clean)
	clean = neutralizeLinks(clean)
	if len(clean) > maxRenderField {
		clean = clean[:maxRenderField] + "…"
	}
	return clean
}

// fingerprintHexLen is the expected length of a real redact.Fingerprint value
// (a SHA-256 hex digest). The displayed identifier must be exactly this so a
// user can copy it straight into --resolve.
const fingerprintHexLen = 64

// renderFingerprint renders the FULL fingerprint identifier shown in the
// `report list` view (codex-completeness #3 — the shown identifier is the one
// `--resolve` accepts, no truncated prefix). A real fingerprint is the
// lowercase-hex SHA-256 of the normalized identity; it carries NO user value.
//
// The render surface is untrusted by contract (Req 7 / HC-4): a HAND-CRAFTED
// reports.jsonl can plant an oversized / non-hex / control-laden `fingerprint`
// value (codex-render-leak #1). The fix is SCRUB-FULL-THEN-DISPLAY with a
// fail-closed allowlist on the shape:
//
//   - if the FULL value is a well-formed fingerprint (exactly 64 lowercase-hex
//     chars), it is rendered verbatim — it is a self-generated safe hash, so
//     it bypasses the entropy catch-all (which would otherwise replace the
//     high-entropy hex with `<token>` and make it unresolvable) while still
//     being provably value-free;
//   - ANYTHING else (oversized, non-hex, a planted prefix like
//     `LEAKPREFIX123456`, control chars) is NOT a real fingerprint → it is run
//     through the full renderField scrub, which fails closed to `<redacted>`
//     for an unclassifiable/oversized value and NEVER emits a raw prefix.
func renderFingerprint(fp string) string {
	if isFingerprintHex(fp) {
		return fp // a real, value-free hash — show it in full for --resolve
	}
	// Not a well-formed fingerprint: treat as untrusted, scrub + fail-closed.
	return renderField(fp)
}

// isFingerprintHex reports whether s is exactly a redact.Fingerprint digest:
// fingerprintHexLen lowercase hex chars. This is the allowlist that lets the
// self-generated safe hash render verbatim while a planted value falls through
// to the fail-closed scrub.
func isFingerprintHex(s string) bool {
	if len(s) != fingerprintHexLen {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// stripControl removes ALL Unicode control characters so a multi-line or
// terminal-escape injection payload cannot reconstitute a `recovery:`-style
// line a downstream automation would auto-execute, nor smuggle a raw terminal
// escape to the user's terminal (Req 7 / HC-4 / P3). \n \r \t collapse to a
// single space; every OTHER control rune is DROPPED. This covers:
//
//   - C0 controls (U+0000–001F) and DEL (U+007F);
//   - C1 controls (U+0080–009F) — including CSI U+009B (``), the
//     single-byte control-sequence introducer that a naive C0-only strip let
//     reach the terminal as the raw bytes `c2 9b …` (codex-render-leak #2).
//
// unicode.IsControl(r) is true for exactly the C0+C1+DEL range, so it is the
// faithful "all control runes" predicate (printable Unicode survives).
func stripControl(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' {
			b.WriteByte(' ')
			continue
		}
		if unicode.IsControl(r) {
			continue // drop every other C0 / C1 / DEL control rune
		}
		b.WriteRune(r)
	}
	return b.String()
}

// neutralizeLinks defangs markdown / URL auto-linking so a rendered field can
// never become a live, clickable link or a markdown auto-link injection
// (Req 7 / HC-4). It breaks the two trigger sequences a renderer needs:
// `](` (markdown link) and `://` (bare URL scheme). The break is a
// zero-width-safe marker that keeps the text human-readable while killing the
// auto-link.
func neutralizeLinks(s string) string {
	s = strings.ReplaceAll(s, "](", "] (")
	s = strings.ReplaceAll(s, "://", ":/​/")
	return s
}
