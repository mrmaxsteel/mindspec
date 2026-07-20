package main

// panel_disposition_store.go: spec 117 Bead 2 — the `append` and `check`
// leaves under `mindspec panel disposition` (registered from THIS file's
// own init(), never touching panel_disposition.go per the plan's
// shared-file note). `append` is the thin CLI adapter over
// internal/panel.AppendRecord (R6(b)'s canonical transactional write);
// `check` is the thin CLI adapter over internal/panel.CheckCompleteness
// (the R1(b) completeness floor).

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var panelDispositionAppendCmd = &cobra.Command{
	Use:   "append --spec <id> --panel <name> --data <literal|-|@file>",
	Short: "Append one disposition row or coverage manifest via the R6(b) transactional op",
	Long: `append is the SOLE write path onto a panel's dispositions.jsonl. It
dispatches to internal/panel.AppendRecord, which — under ONE lock held
on that panel's dedicated dispositions.lock file — validates the
record (schema + R5 hygiene) BEFORE touching the data file, checks it
for uniqueness against the file's current content (a disposition row
keyed on its own "id"; a coverage manifest keyed on
{spec, panel, round}), and either no-ops (the key already exists) or
appends it as a single atomic write.

A validation or hygiene failure exits non-zero and leaves the data file
byte-unchanged (gate-before-mutate); it is never created if it did not
already exist.

--data supplies the one JSON line to append:
  - a literal JSON object, passed inline;
  - "-" to read the line from stdin;
  - "@<path>" to read it from a file.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		specID, _ := cmd.Flags().GetString("spec")
		panelName, _ := cmd.Flags().GetString("panel")
		dataArg, _ := cmd.Flags().GetString("data")

		if specID == "" {
			return fmt.Errorf("--spec is required")
		}
		if panelName == "" {
			return fmt.Errorf("--panel is required")
		}
		if err := validatePanelSlug(panelName); err != nil {
			return err
		}
		if dataArg == "" {
			return fmt.Errorf("--data is required (a literal JSON line, \"-\" for stdin, or \"@<path>\")")
		}

		record, err := readDispositionData(cmd, dataArg)
		if err != nil {
			return err
		}

		specDir, err := resolveDispositionSpecDir(specID)
		if err != nil {
			return err
		}

		if err := panel.AppendRecord(specDir, panelName, record); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "OK: appended to spec %s panel %s dispositions.jsonl\n", idrender.Spec(specID), panelName)
		return nil
	},
}

var panelDispositionCheckCmd = &cobra.Command{
	Use:   "check --spec <id> [--panel <name>]",
	Short: "Run the R1(b) completeness floor against the durable per-panel store",
	Long: `check dispatches to internal/panel.CheckCompleteness, which reads
ONLY a panel's dispositions.jsonl (its coverage manifest plus its
disposition rows) — never a raw verdict file — and, for every manifest
slot whose terminal verdict is REQUEST_CHANGES or REJECT, requires at
least one disposition row naming that slot (as "reviewer" or in
"convergent_with[]").

With --panel, only that one panel is checked. Without it, every panel
directory under <spec-dir>/reviews/ that has a dispositions.jsonl is
checked. Exit code: 0 if every checked panel passes the floor; non-zero,
naming every failing panel + slot, otherwise.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		specID, _ := cmd.Flags().GetString("spec")
		panelName, _ := cmd.Flags().GetString("panel")

		if specID == "" {
			return fmt.Errorf("--spec is required")
		}
		if panelName != "" {
			if err := validatePanelSlug(panelName); err != nil {
				return err
			}
		}

		specDir, err := resolveDispositionSpecDir(specID)
		if err != nil {
			return err
		}

		var panels []string
		if panelName != "" {
			panels = []string{panelName}
		} else {
			panels, err = discoverDispositionPanels(specDir)
			if err != nil {
				return err
			}
		}
		if len(panels) == 0 {
			return fmt.Errorf("no panel dispositions.jsonl found under %s", termsafe.Escape(filepath.Join(specDir, "reviews")))
		}

		var failures []string
		for _, p := range panels {
			if err := panel.CheckCompleteness(specDir, p); err != nil {
				failures = append(failures, err.Error())
			}
		}
		if len(failures) > 0 {
			return fmt.Errorf("%s", strings.Join(failures, "\n"))
		}

		fmt.Fprintf(cmd.OutOrStdout(), "OK: completeness floor satisfied for %d panel(s)\n", len(panels))
		return nil
	},
}

// resolveDispositionSpecDir resolves --spec's value to an on-disk spec
// directory via the same layout-aware workspace.SpecDir logic every
// other panel subcommand uses (panelDirFor, panel.go).
func resolveDispositionSpecDir(specID string) (string, error) {
	root, err := findRoot()
	if err != nil {
		return "", err
	}
	specDir, err := workspace.SpecDir(root, specID)
	if err != nil {
		return "", fmt.Errorf("resolving --spec %s: %w", idrender.Spec(specID), err)
	}
	return specDir, nil
}

// readDispositionData resolves --data's value to the raw JSON line
// bytes to append: "-" reads all of stdin; a leading "@" reads the
// named file; anything else is used as a literal inline value.
func readDispositionData(cmd *cobra.Command, dataArg string) ([]byte, error) {
	switch {
	case dataArg == "-":
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return nil, fmt.Errorf("reading --data from stdin: %w", err)
		}
		return bytes.TrimSpace(data), nil
	case strings.HasPrefix(dataArg, "@"):
		path := dataArg[1:]
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading --data file %s: %s", termsafe.Escape(path), termsafe.Escape(err.Error()))
		}
		return bytes.TrimSpace(data), nil
	default:
		return []byte(dataArg), nil
	}
}

// discoverDispositionPanels lists, in sorted order, every subdirectory
// of <specDir>/reviews/ that contains a dispositions.jsonl file. A
// missing reviews/ directory yields an empty (nil) list, not an error —
// the caller reports "no panel dispositions.jsonl found" uniformly.
func discoverDispositionPanels(specDir string) ([]string, error) {
	reviewsDir := filepath.Join(specDir, "reviews")
	entries, err := os.ReadDir(reviewsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", termsafe.Escape(reviewsDir), err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, statErr := os.Stat(filepath.Join(reviewsDir, e.Name(), "dispositions.jsonl")); statErr == nil {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

func init() {
	panelDispositionAppendCmd.Flags().String("spec", "", "Spec ID")
	panelDispositionAppendCmd.Flags().String("panel", "", "Panel name (a bare slug — no path separators)")
	panelDispositionAppendCmd.Flags().String("data", "", `Record JSON line: a literal, "-" for stdin, or "@path" for a file`)

	panelDispositionCheckCmd.Flags().String("spec", "", "Spec ID")
	panelDispositionCheckCmd.Flags().String("panel", "", "Panel name (default: check every panel under the spec)")

	panelDispositionCmd.AddCommand(panelDispositionAppendCmd)
	panelDispositionCmd.AddCommand(panelDispositionCheckCmd)
}
