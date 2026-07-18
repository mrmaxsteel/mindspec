package main

import (
	"fmt"
	"io"
	"os"

	"github.com/mrmaxsteel/mindspec/internal/doctor"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check the health of the current workspace",
	Long: `Validates project structure, documentation health, and Beads hygiene.

Use --fix to auto-repair fixable issues (e.g. tracked runtime files).
Use --fix --force to also replace user-authored values for mindspec-required
beads config keys (not just add missing ones).
Use --dry-run-migration to report which specs would migrate on their next
lifecycle command (per ADR-0034) without writing any state. Exits 0 even
when legacy specs are surfaced.
Use --ci to skip developer-local-environment checks (Beads merge driver,
bd version floor, bd schema drift, multiple-bd-on-PATH, stale hooks) that
a fresh CI checkout structurally cannot satisfy, while still gating on
every repo-integrity / lifecycle-divergence check.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fix, _ := cmd.Flags().GetBool("fix")
		force, _ := cmd.Flags().GetBool("force")
		dryRun, _ := cmd.Flags().GetBool("dry-run-migration")
		ci, _ := cmd.Flags().GetBool("ci")

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("cannot determine working directory: %w", err)
		}

		root, err := workspace.FindRoot(cwd)
		if err != nil {
			return fmt.Errorf("workspace not found: %w", err)
		}

		fmt.Printf("Workspace Root: %s\n", root)

		report := doctor.RunWithOptions(root, doctor.Options{Force: force, DryRunMigration: dryRun, SkipLocalEnv: ci})

		if fix {
			report.Fix()
		}

		printDoctorChecks(os.Stdout, report.Checks)

		if report.HasFailures() {
			os.Exit(1)
		}
		return nil
	},
}

// printDoctorChecks renders the report's check lines. Check names and
// messages can carry agent-writable content (spec-dir names, branch names,
// bead IDs, recovery text), so BOTH are routed through internal/termsafe at
// this render sink (spec 116 / spec 119 final-review O2) — the predicate
// strings themselves stay canonical so `mindspec instruct`'s renderer (the
// AC-15 wording-parity counterpart, which escapes at its own template sink)
// can never disagree with doctor on the finding text. Status tags are fixed
// literals and need no escaping.
func printDoctorChecks(w io.Writer, checks []doctor.Check) {
	for _, c := range checks {
		fmt.Fprintf(w, "%s: %s", termsafe.Escape(c.Name), statusTag(c.Status))
		if c.Message != "" {
			fmt.Fprintf(w, " %s", termsafe.Escape(c.Message))
		}
		fmt.Fprintln(w)
	}
}

func statusTag(s doctor.Status) string {
	switch s {
	case doctor.OK:
		return "[OK]"
	case doctor.Missing:
		return "[MISSING]"
	case doctor.Error:
		return "[ERROR]"
	case doctor.Warn:
		return "[WARN]"
	case doctor.Fixed:
		return "[FIXED]"
	default:
		return "[UNKNOWN]"
	}
}

func init() {
	doctorCmd.Flags().Bool("fix", false, "Auto-repair fixable issues")
	doctorCmd.Flags().Bool("force", false, "With --fix, also replace user-authored values for mindspec-required beads config keys")
	doctorCmd.Flags().Bool("dry-run-migration", false, "Report which specs would migrate on their next lifecycle command without writing any state")
	doctorCmd.Flags().Bool("ci", false, "Skip developer-local-environment checks (merge driver, bd version/schema, bd-on-PATH, hooks) that a fresh CI checkout cannot satisfy")
}
