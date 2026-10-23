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
	"encoding/json"
	"fmt"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var panelDispositionAppendCmd = &cobra.Command{
	Use:   "append --spec <id> --panel <name> --data <literal|-|@file>",
	Short: "Append one disposition row or coverage manifest via the transactional op",
	Long: `append writes disposition rows and legacy coverage manifests. It refuses
round, refutation, deferral, rebind, abandon, verdict, launch and substitution
events, which only the panel
verbs write. A manifest on a non-legacy panel is reserved to panel tally. It
dispatches to the panel ledger store, which — under ONE lock held
on that panel's dedicated dispositions.lock file — validates the
record (schema + hygiene) BEFORE touching the data file, checks it
for uniqueness against the file's current content (a disposition row
keyed on its own "id"; a coverage manifest keyed on
{spec, panel, round, created_at}), and either no-ops (the key already exists) or
appends it as a single atomic write.

A validation or hygiene failure exits non-zero and leaves the data file
byte-unchanged (gate-before-mutate); it is never created if it did not
already exist.

--data supplies the one JSON line to append:
  - a literal JSON object, passed inline;
  - "-" to read the line from stdin;
  - "@<path>" to read it from a file.

A disposition row needs only the finding: reviewer, model, severity,
summary and disposition. append fills record, spec and gate (from the
panel registration; spec falls back to --spec), panel (from --panel),
created_at (now), convergent_with ([]) and backfilled (false) when they
are absent. A rejected row prints the full shape with an example.

For a disposition row, the canonical "id" is COMPUTED from the record
content (a content hash of {spec, panel, reviewer, summary}); any "id"
you supply is overridden, so a re-append of the same finding is an
idempotent retry rather than a duplicate. Prefer "@<file>" or "-" (a
quoted heredoc) over an inline literal so untrusted finding text is
never interpolated into a shell-quoted argument.`,
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
		record, err = fillDispositionRow(record, specID, panelName, time.Now)
		if err != nil {
			return err
		}

		// FR-3: enforce the canonical content-derived id for a disposition
		// row, overriding any operator-supplied id, so live capture gets
		// R2's stable-content-id + R6 retry-idempotency for free (a no-op
		// passthrough for a coverage manifest or an undecodable line, which
		// AppendRecord's own Validate gate then accepts or rejects).
		record = panel.CanonicalizeDispositionRowID(record)

		specDir, err := resolveDispositionSpecDir(specID)
		if err != nil {
			return err
		}

		var discriminator struct {
			Record string `json:"record"`
		}
		if err := json.Unmarshal(record, &discriminator); err != nil {
			return err
		}
		if panel.IsVerbRecord(discriminator.Record) {
			verb := map[string]string{
				panel.RecordRound: "create", panel.RecordRefutation: "refute",
				panel.RecordDeferral: "defer", panel.RecordRebind: "rebind",
				panel.RecordAbandon: "abandon", panel.RecordVerdict: "submit",
				panel.RecordLaunch: "launch", panel.RecordSubstitution: "launch",
			}[discriminator.Record]
			return termsafe.NewFailure(fmt.Sprintf("record %s is reserved to the panel verbs", discriminator.Record), fmt.Sprintf("use panel %s", verb))
		}
		if discriminator.Record == panel.RecordPanelManifest {
			dir := filepath.Join(specDir, "reviews", panelName)
			if err := panel.AppendRecordsWithCheck(dir, func() error {
				legacy, err := panel.LegacyPanel(dir)
				if err != nil {
					return err
				}
				if !legacy {
					return fmt.Errorf("a manifest on a non-legacy panel is reserved to panel tally")
				}
				return nil
			}, record); err != nil {
				return err
			}
		} else {
			if discriminator.Record == panel.RecordDisposition {
				if err := panel.Validate(record); err != nil {
					return fmt.Errorf("%w\n%s", err, dispositionRowShape)
				}
			}
			if err := panel.AppendRecord(specDir, panelName, record); err != nil {
				return err
			}
		}

		fmt.Fprintf(cmd.OutOrStdout(), "OK: appended to spec %s panel %s dispositions.jsonl\n", idrender.Spec(specID), panelName)
		return nil
	},
}

var panelDispositionCheckCmd = &cobra.Command{
	Use:   "check [panel] --spec <id> [--panel <name>]",
	Short: "Run the completeness floor against the durable per-panel store",
	Long: `check reads
ONLY a panel's dispositions.jsonl (its coverage manifest plus its
disposition rows) — never a raw verdict file — and, for every manifest
slot whose terminal verdict is REQUEST_CHANGES or REJECT, requires at
least one disposition row naming that slot (as "reviewer" or in
"convergent_with[]").

This check is ledger-only; source currency is checked by the panel gate,
complete, and impl approve, which already read verdict files.

With --panel, only that one panel is checked. Without it, every panel
directory under <spec-dir>/reviews/ that has a dispositions.jsonl is
checked. A positional panel name is the same as --panel; without --spec
the spec comes from that panel's registration. Exit code: 0 if every
checked panel passes the floor; non-zero, naming every failing panel +
slot, otherwise.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		specID, _ := cmd.Flags().GetString("spec")
		panelName, _ := cmd.Flags().GetString("panel")
		if len(args) == 1 {
			if panelName != "" && panelName != args[0] {
				return fmt.Errorf("the positional panel %s and --panel %s differ; give one", termsafe.Escape(args[0]), termsafe.Escape(panelName))
			}
			panelName = args[0]
		}
		if specID == "" && panelName != "" {
			specID = dispositionPanelSpec(panelName)
		}

		if specID == "" {
			return fmt.Errorf("--spec is required: mindspec panel disposition check --spec <id> [--panel <name>]")
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

// dispositionRowShape is the hint printed when a disposition row fails
// validation: the fields the caller must give, and an example.
const dispositionRowShape = `a disposition row needs: reviewer, model, severity, summary, disposition
(one of confirmed-fixed, confirmed-scope-trim, deferred, false-contamination).
Use panel defer for confirmed-deferred and panel refute for audited-refuted.
Optional: convergent_with, evidence_ref,
note, round. append fills record, id, spec, gate, panel, created_at,
convergent_with and backfilled when they are absent.
example: {"reviewer":"R1","model":"opus","severity":"minor","summary":"rename X","disposition":"confirmed-fixed","evidence_ref":"abc1234"}`

// fillDispositionRow completes a disposition row from the command flags
// and the panel registration, so the caller supplies only the finding.
// A record with no "record" field is a disposition row. It fills record,
// panel (from --panel), spec and gate (from the panel registration; spec
// falls back to --spec),
// created_at (now), convergent_with ([]) and backfilled (false) when they
// are absent; a field the caller gave is kept. Other record kinds and
// undecodable input pass through unchanged for the store's own validation.
func fillDispositionRow(record []byte, specID, panelName string, now func() time.Time) ([]byte, error) {
	var m map[string]interface{}
	if err := json.Unmarshal(bytes.TrimSpace(record), &m); err != nil || m == nil {
		return record, nil
	}
	if _, ok := m["record"]; !ok {
		m["record"] = panel.RecordDisposition
	}
	if r, _ := m["record"].(string); r != panel.RecordDisposition {
		return record, nil
	}
	if _, ok := m["panel"]; !ok {
		m["panel"] = panelName
	}
	_, hasSpec := m["spec"]
	_, hasGate := m["gate"]
	if root, err := findPanelRoot(config.IsolationRootFor); err == nil && len(panelSlugMatches(root, panelName)) > 0 {
		reg, err := findPanelRegistration(root, panelName, specID)
		if err != nil {
			return nil, err
		}
		if reg.Err != nil {
			return nil, reg.Err
		}
		if reg.Panel.Spec != specID {
			return nil, fmt.Errorf("panel spec must match --spec")
		}
		if !hasSpec {
			m["spec"] = panel.RecordSpec(reg.Panel)
		}
		if !hasGate {
			gate := reg.Panel.Gate
			if gate == "" {
				gate = "adhoc"
			}
			m["gate"] = gate
		}
	}
	if _, ok := m["spec"]; !ok {
		m["spec"] = specID
	}
	if _, ok := m["created_at"]; !ok {
		m["created_at"] = now().UTC().Format(time.RFC3339)
	}
	if _, ok := m["convergent_with"]; !ok {
		m["convergent_with"] = []string{}
	}
	if _, ok := m["backfilled"]; !ok {
		m["backfilled"] = false
	}
	if _, ok := m["id"]; !ok {
		m["id"] = ""
	}
	return json.Marshal(m)
}

// dispositionPanelSpec returns the spec recorded in the panel's
// registration, or "" when the slug is invalid or does not resolve to one
// registration.
func dispositionPanelSpec(slug string) string {
	if validatePanelSlug(slug) != nil {
		return ""
	}
	root, err := findPanelRoot(config.IsolationRootFor)
	if err != nil {
		return ""
	}
	reg, err := findPanelRegistration(root, slug)
	if err != nil || reg.Err != nil {
		return ""
	}
	return reg.Panel.Spec
}

// resolveDispositionSpecDir resolves --spec's value to an on-disk spec
// directory via the same layout-aware workspace.SpecDir logic every
// other panel subcommand uses (panelDirFor, panel.go).
func resolveDispositionSpecDir(specID string) (string, error) {
	root, err := findPanelRoot(config.IsolationRootFor)
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
