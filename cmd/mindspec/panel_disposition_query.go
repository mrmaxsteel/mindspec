package main

// panel_disposition_query.go: `mindspec panel disposition query …` — the
// spec 117 Bead 3 CLI leaf over internal/panel's Q1-Q5 read-side query
// surface (R3). It registers itself onto the EXISTING `disposition`
// parent command Bead 1 created in panel_disposition.go via its own
// init() AddCommand call — this file never edits panel_disposition.go.

import (
	"fmt"
	"path/filepath"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/spf13/cobra"
)

var (
	queryMetric      string
	queryDir         string
	querySpec        string
	queryGate        string
	querySeverity    string
	queryDisposition string
)

var panelDispositionQueryCmd = &cobra.Command{
	Use:   "query",
	Short: "Run one of the R3 Q1-Q5 disposition-telemetry queries over the per-panel JSONL store",
	Long: `query loads every dispositions.jsonl file matched by the resolved
glob (by default ` + "`" + panel.DefaultGlobPattern + "`" + `, every
panel of every spec) and reports one of five metrics:

  Q1  per-model genuine-find rate (genuine/total)
  Q2  per-model false-positive rate (false-positive/total)
  Q3  convergence rate (rows with non-empty convergent_with / total) + the row list
  Q4  per-gate genuine-per-slot yield (genuine rows / summed manifest slot rosters)
  Q5  finding listing, filterable on --gate/--severity/--disposition

--dir overrides the glob root entirely, pointing directly at a
self-contained "reviews"-shaped directory (one dispositions.jsonl per
subdirectory, no .mindspec/specs/<spec>/reviews nesting) — the shape of
a checked-in testdata fixture such as internal/panel/testdata/seed116.
--spec narrows the default root to one spec's live store
(.mindspec/specs/<spec>/reviews/*/dispositions.jsonl). --dir and --spec
are mutually exclusive.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if queryDir != "" && querySpec != "" {
			return fmt.Errorf("--dir and --spec are mutually exclusive")
		}
		pattern, err := resolveDispositionQueryGlob(queryDir, querySpec)
		if err != nil {
			return err
		}
		rows, manifests, err := panel.LoadStore(pattern)
		if err != nil {
			return err
		}
		return runDispositionQuery(cmd, queryMetric, rows, manifests)
	},
}

// resolveDispositionQueryGlob resolves --dir/--spec into the glob
// pattern panel.LoadStore reads. dir, if set, replaces the pattern
// entirely with "<dir>/*/dispositions.jsonl" (a flat, spec-agnostic
// reviews-shaped root). spec, if set, is validated with
// idvalidate.SpecID BEFORE being joined into a filesystem glob — spec is
// user-supplied and flows into filepath.Glob, the exact SEC-1 hazard
// idvalidate exists to close (see idvalidate's package doc). Neither
// set: the store-wide default.
func resolveDispositionQueryGlob(dir, spec string) (string, error) {
	if dir != "" {
		return filepath.Join(dir, "*", "dispositions.jsonl"), nil
	}
	if spec != "" {
		if err := idvalidate.SpecID(spec); err != nil {
			return "", fmt.Errorf("invalid --spec %s: %s", idrender.Spec(spec), termsafe.Escape(err.Error()))
		}
		return filepath.Join(".mindspec", "specs", spec, "reviews", "*", "dispositions.jsonl"), nil
	}
	return panel.DefaultGlobPattern, nil
}

// runDispositionQuery dispatches on metric and renders the result to
// cmd's stdout. Every rendered agent-value field (model, reviewer,
// gate, severity, disposition, summary, note, convergent_with entries)
// routes through termsafe.Escape (R2/ADR-0042) before being written.
func runDispositionQuery(cmd *cobra.Command, metric string, rows []panel.DispositionRow, manifests []panel.CoverageManifest) error {
	out := cmd.OutOrStdout()
	switch metric {
	case "Q1":
		// Union the row-derived model set with every model rostered in a
		// coverage manifest, so a reviewed-but-found-nothing model shows
		// 0/0 rather than being dropped (finding G1-001).
		fmt.Fprintln(out, "Q1 (genuine/total per model):")
		for _, r := range panel.ComputeQ1(rows, panel.ManifestModels(manifests)...) {
			fmt.Fprintf(out, "  %s: %s\n", termsafe.Escape(r.Model), r.Render())
		}
	case "Q2":
		fmt.Fprintln(out, "Q2 (false-positive/total per model):")
		for _, r := range panel.ComputeQ2(rows, panel.ManifestModels(manifests)...) {
			fmt.Fprintf(out, "  %s: %s\n", termsafe.Escape(r.Model), r.Render())
		}
	case "Q3":
		res := panel.ComputeQ3(rows)
		fmt.Fprintf(out, "Q3 (convergent rows / total): %d/%d\n", res.ConvergentCount, res.Total)
		for _, r := range res.ConvergentRows {
			fmt.Fprintf(out, "  %s (panel=%s, gate=%s): convergent_with=[%s]\n",
				termsafe.Escape(r.Reviewer), termsafe.Escape(r.Panel), termsafe.Escape(r.Gate), panel.EscapeList(r.ConvergentWith))
		}
	case "Q4":
		fmt.Fprintln(out, "Q4 (genuine / slot-total per gate):")
		for _, r := range panel.ComputeQ4(rows, manifests) {
			fmt.Fprintf(out, "  %s: %d/%d\n", termsafe.Escape(r.Gate), r.Genuine, r.SlotTotal)
		}
	case "Q5":
		filter := panel.Q5Filter{Gate: queryGate, Severity: querySeverity, Disposition: queryDisposition}
		findings := panel.ComputeQ5(rows, filter)
		fmt.Fprintf(out, "Q5 (%d finding(s)):\n", len(findings))
		for _, r := range findings {
			fmt.Fprintf(out, "  [%s/%s] gate=%s severity=%s disposition=%s model=%s reviewer=%s: %s\n",
				termsafe.Escape(r.Spec), termsafe.Escape(r.Panel), termsafe.Escape(r.Gate),
				termsafe.Escape(r.Severity), termsafe.Escape(r.Disposition), termsafe.Escape(r.Model),
				termsafe.Escape(r.Reviewer), termsafe.Escape(r.Summary))
		}
	case "":
		return fmt.Errorf("--metric is required, must be one of Q1, Q2, Q3, Q4, Q5")
	default:
		return fmt.Errorf("--metric must be one of Q1, Q2, Q3, Q4, Q5, got %s", termsafe.Escape(metric))
	}
	return nil
}

func init() {
	panelDispositionQueryCmd.Flags().StringVar(&queryMetric, "metric", "", "which query to run: Q1, Q2, Q3, Q4, or Q5 (required)")
	panelDispositionQueryCmd.Flags().StringVar(&queryDir, "dir", "", "glob root override: a reviews-shaped directory (one dispositions.jsonl per subdirectory), e.g. a checked-in testdata fixture; mutually exclusive with --spec")
	panelDispositionQueryCmd.Flags().StringVar(&querySpec, "spec", "", "narrow the default store-wide glob to one spec's live reviews dir; mutually exclusive with --dir")
	panelDispositionQueryCmd.Flags().StringVar(&queryGate, "gate", "", "Q5 filter: only findings whose gate matches")
	panelDispositionQueryCmd.Flags().StringVar(&querySeverity, "severity", "", "Q5 filter: only findings whose severity matches")
	panelDispositionQueryCmd.Flags().StringVar(&queryDisposition, "disposition", "", "Q5 filter: only findings whose disposition matches")
	panelDispositionCmd.AddCommand(panelDispositionQueryCmd)
}
