package main

// panel_disposition.go: `mindspec panel disposition …` — the CLI parent
// command for the spec 117 panel-disposition telemetry store (ADR-0043).
// Bead 1 lands the parent command plus its `validate` leaf, a thin
// adapter over internal/panel.Validate + internal/panel.HygienePredicate
// (R2/R5/R6(a)). Later beads register their own leaves (`append`/
// `check`/`query`) from their OWN cmd files via their own init()
// AddCommand call against panelDispositionCmd — this file never grows a
// second leaf for those.

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/spf13/cobra"
)

var panelDispositionCmd = &cobra.Command{
	Use:   "disposition",
	Short: "Panel-disposition telemetry store (schema, validation, capture, query)",
}

var panelDispositionValidateCmd = &cobra.Command{
	Use:   "validate <file|glob>...",
	Short: "Validate JSONL disposition/coverage-manifest files against the R2/R6(a) schema + R5 hygiene",
	Long: `validate reads each JSON line of every file matched by the given
path(s)/glob(s) (a bare path with no glob metacharacter is read as a
single file) and runs internal/panel.Validate followed by
internal/panel.HygienePredicate on it — the SAME two pure functions
Bead 2's transactional append op gates every write on. It writes
nothing and makes no filesystem change beyond reading the named files.

Exit code: 0 if every line of every matched file passes both gates;
non-zero on the FIRST failure, with a termsafe-rendered message naming
the file, line number, and the failure. A glob that matches no file is
itself an error (a silently-empty glob must not report success).`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		files, err := expandDispositionFileArgs(args)
		if err != nil {
			return err
		}
		for _, f := range files {
			if err := validateDispositionFile(f); err != nil {
				return err
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "OK: %d file(s), all lines valid\n", len(files))
		return nil
	},
}

// expandDispositionFileArgs resolves each positional argument to a
// sorted, de-duplicated list of file paths: a glob-metacharacter-bearing
// argument is expanded via filepath.Glob (and must match at least one
// file); a plain argument is passed through as a literal path (existence
// is checked when it is opened, not here, so the error message is
// uniform with the glob-miss case's file-not-found leg).
func expandDispositionFileArgs(args []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, a := range args {
		if hasGlobMeta(a) {
			matches, err := filepath.Glob(a)
			if err != nil {
				return nil, fmt.Errorf("invalid glob %s: %s", termsafe.Escape(a), termsafe.Escape(err.Error()))
			}
			if len(matches) == 0 {
				return nil, fmt.Errorf("glob %s matched no files", termsafe.Escape(a))
			}
			for _, m := range matches {
				if !seen[m] {
					seen[m] = true
					out = append(out, m)
				}
			}
			continue
		}
		if !seen[a] {
			seen[a] = true
			out = append(out, a)
		}
	}
	sort.Strings(out)
	return out, nil
}

// hasGlobMeta reports whether s contains a filepath.Match metacharacter
// (`*`, `?`, `[`) — the same trigger filepath.Glob itself interprets.
func hasGlobMeta(s string) bool {
	for _, r := range s {
		if r == '*' || r == '?' || r == '[' {
			return true
		}
	}
	return false
}

// validateDispositionFile runs Validate + HygienePredicate over every
// non-empty line of path, returning the first failure with the file
// name and 1-based line number folded into a termsafe-rendered message.
func validateDispositionFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening %s: %s", termsafe.Escape(path), termsafe.Escape(err.Error()))
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Disposition rows/manifests are small JSON objects, but raise the
	// default 64KiB token cap generously (1MiB) so a long free-prose
	// `note`/`summary` line never trips a spurious bufio.ErrTooLong.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		raw := scanner.Bytes()
		trimmed := len(raw)
		for trimmed > 0 && (raw[trimmed-1] == ' ' || raw[trimmed-1] == '\t' || raw[trimmed-1] == '\r') {
			trimmed--
		}
		if trimmed == 0 {
			continue // blank line — not a record, not a failure
		}
		data := raw[:trimmed]
		if err := panel.Validate(data); err != nil {
			return fmt.Errorf("%s:%d: %s", termsafe.Escape(path), line, err.Error())
		}
		if err := panel.HygienePredicate(data); err != nil {
			return fmt.Errorf("%s:%d: %s", termsafe.Escape(path), line, err.Error())
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading %s: %s", termsafe.Escape(path), termsafe.Escape(err.Error()))
	}
	return nil
}

func init() {
	panelDispositionCmd.AddCommand(panelDispositionValidateCmd)
	panelCmd.AddCommand(panelDispositionCmd)
}
