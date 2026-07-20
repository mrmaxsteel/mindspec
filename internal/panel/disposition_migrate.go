// disposition_migrate.go: spec 117 Bead 4 — migrates the spec-116 seed
// dataset (R4) into the live per-panel disposition store.
//
// Two source artifacts, both outside this repo, feed the migration:
//
//   - The hand-authored raw seed, DefaultSeedPath
//     (/Users/Max/replit/mindspec-panel-verdicts/spec-116/DISPOSITIONS.jsonl):
//     21 rows in the PRE-migration shape (no record/id/created_at/
//     backfilled; short gate values; two skewed reviewer names).
//   - The archive's per-panel verdict files, DefaultArchiveDir's
//     <panel>/verdict-<slot>.json — the source of each panel's
//     synthesized R6(a) coverage manifest (slot token, model, terminal
//     verdict).
//
// MigrateSpec116Seed writes every migrated row and manifest through
// AppendRecord (Bead 2) — the sole write path — so R2/R6(a) validation,
// R5 hygiene, and id/{spec,panel,round}-keyed idempotency all apply
// exactly as they would to a live `/ms-panel-tally` capture. Re-running
// the migration is therefore safe: every row is a no-op replay on its
// content-derived id, and each panel's manifest is a no-op replay on
// {spec,panel,round} (round is pinned to 1, so there is only ever one
// candidate manifest per panel — "first-write-wins" and "the final
// manifest" coincide here).
package panel

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

const (
	// DefaultSeedPath is the raw spec-116 disposition seed's absolute
	// path — the migration SOURCE (R4) and the jq cross-check input
	// (plan's Testing Strategy).
	DefaultSeedPath = "/Users/Max/replit/mindspec-panel-verdicts/spec-116/DISPOSITIONS.jsonl"

	// DefaultArchiveDir is the archive's spec-116 root: DefaultArchiveDir/
	// <panel>/verdict-<slot>.json supplies every migrated coverage
	// manifest's slot roster.
	DefaultArchiveDir = "/Users/Max/replit/mindspec-panel-verdicts/spec-116"

	// Spec116ID is the "spec" field value every migrated spec-116 record
	// carries VERBATIM from the raw seed — a bare "116", never the spec
	// DIRECTORY slug (Spec116DirName).
	Spec116ID = "116"

	// Spec116DirName is the spec directory segment the migrated store
	// lands under: <repo>/.mindspec/specs/<Spec116DirName>/reviews/<panel>/
	// dispositions.jsonl.
	Spec116DirName = "116-panel-message-escaping"

	// spec116BackfillCreatedAt is the R4-pinned backfill created_at
	// value (full RFC 3339, the spec-116 archive-snapshot date) every
	// migrated row and manifest carries.
	spec116BackfillCreatedAt = "2026-07-11T00:00:00Z"

	// spec116MigrationRound is the pinned round every migrated coverage
	// manifest carries — R4: "round: 1 ... pinned so the migration is
	// deterministic".
	spec116MigrationRound = 1
)

// spec116Panels is the pinned, ordered list of the 8 spec-116 panels R4
// migrates, verbatim from the spec/plan enumeration.
var spec116Panels = []string{
	"panel-116-spec",
	"panel-116-plan",
	"panel-116-bead1",
	"panel-116-bead2",
	"panel-116-bead3a",
	"panel-116-bead3b",
	"final-116",
	"gapfix-panel",
}

// spec116PanelGateShort maps each of the 8 spec-116 panels to its raw
// short-form gate value (the seed's own `gate` field for every row in
// that panel) — pinned directly rather than derived from a row so a
// hypothetical finding-less panel would still migrate correctly.
var spec116PanelGateShort = map[string]string{
	"panel-116-spec":   "spec",
	"panel-116-plan":   "plan",
	"panel-116-bead1":  "bead",
	"panel-116-bead2":  "bead",
	"panel-116-bead3a": "bead",
	"panel-116-bead3b": "bead",
	"final-116":        "final",
	"gapfix-panel":     "gapfix",
}

// gateShortToCanonical is the R4 gate-key mapping table: the raw seed's
// short gate values map to config.PanelGateKeys' canonical keys
// (mirrored as CanonicalGateKeys, disposition.go). This is the ONLY
// mapping the migration applies (R4: "mapped only per that table").
var gateShortToCanonical = map[string]string{
	"spec":   "spec_approve",
	"plan":   "plan_approve",
	"bead":   "bead",
	"final":  "final_review",
	"gapfix": "adhoc",
}

// slotCanonicalKey identifies one (panel, raw-name) pair the R1/R4 "C1"
// canonicalization table rewrites.
type slotCanonicalKey struct {
	panel string
	name  string
}

// slotCanonicalization is the R1/R4 C1 canonicalization table: a raw
// seed reviewer/convergent_with name that skews from its panel's
// verdict-file slot token, rewritten to the actual token so the R1(b)
// floor check keys correctly. ONLY these two entries canonicalize;
// every other name (including gapfix-panel's own "G-codex", whose file
// already IS verdict-G-codex.json) is left verbatim by this map's
// absence.
var slotCanonicalization = map[slotCanonicalKey]string{
	{panel: "panel-116-bead3a", name: "G-codex"}:  "G1-codex",
	{panel: "gapfix-panel", name: "Sonnet-tests"}: "S-tests",
}

// canonicalizeSlot returns the canonical slot token for (panelName,
// name): the C1-mapped rewrite if one exists, else name verbatim.
func canonicalizeSlot(panelName, name string) string {
	if canon, ok := slotCanonicalization[slotCanonicalKey{panel: panelName, name: name}]; ok {
		return canon
	}
	return name
}

// rawDispositionRow mirrors the raw seed file's PRE-migration JSON
// shape (DISPOSITIONS.jsonl): no record/id/created_at/backfilled, a
// short-form gate, and possibly-skewed reviewer/convergent_with names.
type rawDispositionRow struct {
	Spec           string   `json:"spec"`
	Gate           string   `json:"gate"`
	Panel          string   `json:"panel"`
	Reviewer       string   `json:"reviewer"`
	Model          string   `json:"model"`
	Severity       string   `json:"severity"`
	Summary        string   `json:"summary"`
	ConvergentWith []string `json:"convergent_with"`
	Disposition    string   `json:"disposition"`
	Note           string   `json:"note"`
}

// rawVerdictFile decodes the single field this migration needs from an
// archive verdict-<slot>.json file: the terminal verdict. Every other
// field (lens, rationale, checks_run, findings, ...) is raw panel
// presentation output, not part of the disposition/coverage schema, and
// is never carried into the migrated store (R5 — the raw files
// themselves embed machine-local paths and are never committed).
type rawVerdictFile struct {
	Verdict string `json:"verdict"`
}

// slotModel derives the ACTUAL model family (R2: "model is the ACTUAL
// model, not slot-family") from a verdict-file slot token's leading
// letter. Verified against the raw seed's own `model` field for every
// one of the 66 archive slots: F->fable, O->opus, S->sonnet,
// G->gpt-5.6-sol (the codex family).
func slotModel(slot string) (string, error) {
	if slot == "" {
		return "", fmt.Errorf("panel: migrate: empty slot token")
	}
	switch slot[0] {
	case 'F', 'f':
		return "fable", nil
	case 'O', 'o':
		return "opus", nil
	case 'S', 's':
		return "sonnet", nil
	case 'G', 'g':
		return "gpt-5.6-sol", nil
	default:
		return "", fmt.Errorf("panel: migrate: slot token %s has no known model-prefix mapping", termsafe.Escape(slot))
	}
}

// mintRowID derives DispositionRow.ID: a stable content hash of
// {spec, panel, reviewer, summary} (R2's id contract — migrated rows
// carry no round, so round is deliberately excluded from the key). The
// same four inputs always mint the same id, so replaying the migration
// is retry-idempotent through AppendRecord's own id-keyed dedup.
func mintRowID(spec, panelName, reviewer, summary string) string {
	h := sha256.Sum256([]byte(spec + "\x00" + panelName + "\x00" + reviewer + "\x00" + summary))
	return "d-" + hex.EncodeToString(h[:])[:16]
}

// splitJSONLLinesBufCap bounds one seed JSONL line: the largest raw
// spec-116 row is a few KB, but a disposition `note`/`summary` is free
// reviewer prose, so the cap is set generously (64 MiB) to admit any
// realistic row while still guarding against an unbounded read. A line
// EXCEEDING this cap is a fail-CLOSED error (see splitJSONLLines), never
// a silent truncation.
const splitJSONLLinesBufCap = 64 * 1024 * 1024

// splitJSONLLines returns data's non-blank lines, trimmed of
// surrounding whitespace. It FAILS CLOSED: if bufio.Scanner stops early
// — most importantly on a line exceeding splitJSONLLinesBufCap, which
// otherwise makes Scan() halt SILENTLY and drop every subsequent line —
// the scanner's error is surfaced rather than swallowed, so a partial
// read can never masquerade as a complete (or empty) migration (AC4
// fail-closed design; S3).
func splitJSONLLines(data []byte) ([][]byte, error) {
	var lines [][]byte
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), splitJSONLLinesBufCap)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		out := make([]byte, len(line))
		copy(out, line)
		lines = append(lines, out)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("panel: migrate: scanning seed JSONL (a line may exceed the %d-byte cap): %s", splitJSONLLinesBufCap, termsafe.Escape(err.Error()))
	}
	return lines, nil
}

// loadRawSeedRows reads and parses seedPath's raw, PRE-migration JSONL
// rows.
func loadRawSeedRows(seedPath string) ([]rawDispositionRow, error) {
	data, err := os.ReadFile(seedPath)
	if err != nil {
		return nil, fmt.Errorf("panel: migrate: reading seed file %s: %w", termsafe.Escape(seedPath), err)
	}
	lines, err := splitJSONLLines(data)
	if err != nil {
		return nil, fmt.Errorf("panel: migrate: seed file %s: %w", termsafe.Escape(seedPath), err)
	}
	rows := make([]rawDispositionRow, 0, len(lines))
	for i, line := range lines {
		var r rawDispositionRow
		if err := json.Unmarshal(line, &r); err != nil {
			return nil, fmt.Errorf("panel: migrate: seed file %s line %d: %s", termsafe.Escape(seedPath), i+1, termsafe.Escape(err.Error()))
		}
		rows = append(rows, r)
	}
	return rows, nil
}

// buildDispositionRowJSON canonicalizes and marshals one raw seed row
// (already known to belong to panelName) into the migrated
// `record:"disposition"` JSON line R2 requires: canonical gate key,
// canonicalized reviewer/convergent_with, minted id, backfilled
// created_at, backfilled:true, no round.
func buildDispositionRowJSON(panelName string, r rawDispositionRow) ([]byte, error) {
	canonicalGate, ok := gateShortToCanonical[r.Gate]
	if !ok {
		return nil, fmt.Errorf("panel: migrate: seed row (panel %s, reviewer %s) has unmapped gate %s", termsafe.Escape(panelName), termsafe.Escape(r.Reviewer), termsafe.Escape(r.Gate))
	}

	reviewer := canonicalizeSlot(panelName, r.Reviewer)
	convergent := make([]string, len(r.ConvergentWith))
	for i, c := range r.ConvergentWith {
		convergent[i] = canonicalizeSlot(panelName, c)
	}

	row := DispositionRow{
		Record:         RecordDisposition,
		ID:             mintRowID(r.Spec, panelName, reviewer, r.Summary),
		Spec:           r.Spec,
		Gate:           canonicalGate,
		Panel:          panelName,
		Reviewer:       reviewer,
		Model:          r.Model,
		Severity:       r.Severity,
		Summary:        r.Summary,
		ConvergentWith: convergent,
		Disposition:    r.Disposition,
		Note:           r.Note,
		CreatedAt:      spec116BackfillCreatedAt,
		Backfilled:     true,
	}
	data, err := json.Marshal(row)
	if err != nil {
		return nil, fmt.Errorf("panel: migrate: marshaling disposition row: %w", err)
	}
	return data, nil
}

// buildCoverageManifest synthesizes panelName's R6(a) coverage manifest
// from the archive's <archiveDir>/<panelName>/verdict-<slot>.json files
// — one `slots[]` entry per file (slot = filename stem, model derived
// via slotModel, verdict read from the file's own `verdict` field).
func buildCoverageManifest(archiveDir, panelName, gateShort string) (CoverageManifest, error) {
	canonicalGate, ok := gateShortToCanonical[gateShort]
	if !ok {
		return CoverageManifest{}, fmt.Errorf("panel: migrate: panel %s has unmapped gate %s", termsafe.Escape(panelName), termsafe.Escape(gateShort))
	}

	panelDir := filepath.Join(archiveDir, panelName)
	entries, err := os.ReadDir(panelDir)
	if err != nil {
		return CoverageManifest{}, fmt.Errorf("panel: migrate: reading archive panel dir %s: %w", termsafe.Escape(panelDir), err)
	}

	var slots []ManifestSlot
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, "verdict-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		slot := strings.TrimSuffix(strings.TrimPrefix(name, "verdict-"), ".json")
		model, err := slotModel(slot)
		if err != nil {
			return CoverageManifest{}, err
		}

		filePath := filepath.Join(panelDir, name)
		data, err := os.ReadFile(filePath)
		if err != nil {
			return CoverageManifest{}, fmt.Errorf("panel: migrate: reading verdict file %s: %w", termsafe.Escape(filePath), err)
		}
		var vf rawVerdictFile
		if err := json.Unmarshal(data, &vf); err != nil {
			return CoverageManifest{}, fmt.Errorf("panel: migrate: parsing verdict file %s: %s", termsafe.Escape(filePath), termsafe.Escape(err.Error()))
		}
		if !isValidVerdict(vf.Verdict) {
			return CoverageManifest{}, fmt.Errorf("panel: migrate: verdict file %s has out-of-enum verdict %s", termsafe.Escape(filePath), termsafe.Escape(vf.Verdict))
		}
		slots = append(slots, ManifestSlot{Slot: slot, Model: model, Verdict: vf.Verdict})
	}
	if len(slots) == 0 {
		return CoverageManifest{}, fmt.Errorf("panel: migrate: archive panel dir %s has no verdict-*.json files", termsafe.Escape(panelDir))
	}
	// Deterministic slot order (alphabetical by token) so re-running the
	// migration always synthesizes byte-identical manifest JSON.
	sort.Slice(slots, func(i, j int) bool { return slots[i].Slot < slots[j].Slot })

	return CoverageManifest{
		Record:     RecordPanelManifest,
		Spec:       Spec116ID,
		Gate:       canonicalGate,
		Panel:      panelName,
		Round:      spec116MigrationRound,
		Slots:      slots,
		Backfilled: true,
	}, nil
}

// MigrateSpec116Seed migrates the spec-116 seed (R4): it reads
// seedPath's raw disposition rows and archiveDir's per-panel verdict
// files, and writes the migrated per-panel store — every disposition
// row plus each panel's ONE coverage manifest — into targetSpecDir
// (a spec directory: <targetSpecDir>/reviews/<panel>/dispositions.jsonl)
// entirely through AppendRecord/WriteTerminalManifest (Bead 2), so R2
// validation, R5 hygiene, and idempotency all apply. Rows for a panel
// are appended before that panel's manifest (matching the natural
// resolve-then-close-out order), though record position is never
// semantically significant (records are found by the `record` field).
//
// Calling this more than once is safe: it recomputes the same 21 rows
// and 8 manifests from the same two source artifacts every time, and
// AppendRecord's id/{spec,panel,round}-keyed idempotency makes every
// line a no-op replay.
func MigrateSpec116Seed(seedPath, archiveDir, targetSpecDir string) error {
	rawRows, err := loadRawSeedRows(seedPath)
	if err != nil {
		return err
	}

	byPanel := make(map[string][]rawDispositionRow, len(spec116Panels))
	for _, r := range rawRows {
		byPanel[r.Panel] = append(byPanel[r.Panel], r)
	}

	for _, panelName := range spec116Panels {
		gateShort, ok := spec116PanelGateShort[panelName]
		if !ok {
			return fmt.Errorf("panel: migrate: no gate mapping registered for panel %s", termsafe.Escape(panelName))
		}

		for _, r := range byPanel[panelName] {
			rowBytes, err := buildDispositionRowJSON(panelName, r)
			if err != nil {
				return err
			}
			if err := AppendRecord(targetSpecDir, panelName, rowBytes); err != nil {
				return fmt.Errorf("panel: migrate: appending disposition row (panel %s): %w", termsafe.Escape(panelName), err)
			}
		}

		manifest, err := buildCoverageManifest(archiveDir, panelName, gateShort)
		if err != nil {
			return err
		}
		if err := WriteTerminalManifest(targetSpecDir, panelName, manifest); err != nil {
			return fmt.Errorf("panel: migrate: writing coverage manifest (panel %s): %w", termsafe.Escape(panelName), err)
		}
	}

	return nil
}
