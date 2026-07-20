// disposition_query.go: spec 117 Bead 3 — the R3 read-side Q1-Q5 query
// surface over the per-panel disposition store. Every function here is a
// pure, in-memory computation over already-loaded rows/manifests except
// LoadStore, whose only I/O is reading the matched JSONL files; nothing
// in this file mutates the store (that is Bead 2's AppendRecord/
// WriteTerminalManifest territory).
//
// "genuine" and "false-positive" are NOT redefined here: Q1/Q2 consume
// Bead 1's exported GenuineDispositions/FalsePositiveDispositions maps
// (disposition.go) as the single source of truth — a finding whose
// disposition is plain "deferred" belongs to neither set and is counted
// only in the denominator, exactly as Bead 1 documents.
package panel

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// DefaultGlobPattern is the store-wide glob R3 names: every panel's
// dispositions.jsonl across every spec's reviews dir.
const DefaultGlobPattern = ".mindspec/specs/*/reviews/*/dispositions.jsonl"

// LoadStore reads every file matched by pattern (a filepath.Glob
// pattern), validates each JSONL line with Validate (a query surface
// must never trust an unvalidated line — the SAME schema gate Bead 2's
// append op and Bead 1's `validate` leaf apply), and decodes it into
// either a DispositionRow or a CoverageManifest per its `record`
// discriminator. Matched files are visited in sorted-path order and
// lines within a file in on-disk order, so the returned slices are
// deterministic across runs and platforms. A pattern matching zero files
// is not itself an error (an empty/new store is a legitimate state for
// every Q1-Q5 computation, which must render `0/N`, never panic, on an
// empty rows/manifests pair).
func LoadStore(pattern string) ([]DispositionRow, []CoverageManifest, error) {
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid glob %s: %s", termsafe.Escape(pattern), termsafe.Escape(err.Error()))
	}
	sort.Strings(files)

	var rows []DispositionRow
	var manifests []CoverageManifest
	for _, f := range files {
		fr, fm, err := loadDispositionFile(f)
		if err != nil {
			return nil, nil, err
		}
		rows = append(rows, fr...)
		manifests = append(manifests, fm...)
	}
	return rows, manifests, nil
}

// recordDiscriminator decodes just the `record` field of an
// already-Validate-passed line — Validate has already confirmed it is
// present, a string, and one of RecordDisposition/RecordPanelManifest,
// so no error path is reachable here that Validate did not already
// reject.
type recordDiscriminator struct {
	Record string `json:"record"`
}

// loadDispositionFile reads path line-by-line, running Validate on each
// non-blank line before decoding it into the row/manifest struct it
// discriminates as. A validation failure aborts the whole load with a
// termsafe-rendered file:line message — a query surface silently
// tolerating a malformed line would compute wrong Q1-Q5 numbers instead
// of failing loudly.
func loadDispositionFile(path string) ([]DispositionRow, []CoverageManifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("opening %s: %s", termsafe.Escape(path), termsafe.Escape(err.Error()))
	}
	defer f.Close()

	var rows []DispositionRow
	var manifests []CoverageManifest

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue // blank line — not a record
		}
		// Copy out of the scanner's reused buffer before it is retained
		// past the next Scan() call (json.Unmarshal below keeps
		// references into it via string fields).
		data := append([]byte(nil), raw...)

		if err := Validate(data); err != nil {
			return nil, nil, fmt.Errorf("%s:%d: %s", termsafe.Escape(path), line, err.Error())
		}

		var disc recordDiscriminator
		if err := json.Unmarshal(data, &disc); err != nil {
			return nil, nil, fmt.Errorf("%s:%d: %s", termsafe.Escape(path), line, termsafe.Escape(err.Error()))
		}
		switch disc.Record {
		case RecordDisposition:
			var row DispositionRow
			if err := json.Unmarshal(data, &row); err != nil {
				return nil, nil, fmt.Errorf("%s:%d: %s", termsafe.Escape(path), line, termsafe.Escape(err.Error()))
			}
			rows = append(rows, row)
		case RecordPanelManifest:
			var m CoverageManifest
			if err := json.Unmarshal(data, &m); err != nil {
				return nil, nil, fmt.Errorf("%s:%d: %s", termsafe.Escape(path), line, termsafe.Escape(err.Error()))
			}
			manifests = append(manifests, m)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("reading %s: %s", termsafe.Escape(path), termsafe.Escape(err.Error()))
	}
	return rows, manifests, nil
}

// ModelRate is one per-model numerator/denominator pair — Q1's
// genuine/total or Q2's false-positive/total for a single model.
type ModelRate struct {
	Model       string
	Numerator   int
	Denominator int
}

// Render renders the pinned "N/D" form (e.g. "0/5"). This is a plain
// integer format, never a division, so it cannot itself produce NaN/Inf
// or panic regardless of Denominator — including the Denominator == 0
// case (renders "0/0"), which is the divide-by-EMPTY guard R3 mandates:
// a model must never be silently dropped from the listing, and its rate
// must never be computed via float division without a zero check (see
// Percent below, which IS a division and DOES guard it).
func (r ModelRate) Render() string {
	return fmt.Sprintf("%d/%d", r.Numerator, r.Denominator)
}

// Percent returns the numerator/denominator rate as a percentage,
// guarded against the divide-by-empty case: Denominator == 0 returns 0
// rather than computing 0.0/0.0 (NaN in IEEE-754 float division) or
// N/0 (+Inf). Every caller that wants a human-readable percentage
// alongside Render's "N/D" form must go through this guard, not a raw
// float division.
func (r ModelRate) Percent() float64 {
	if r.Denominator == 0 {
		return 0
	}
	return float64(r.Numerator) / float64(r.Denominator) * 100
}

// ComputeModelRates computes, for every distinct model appearing in
// rows, the (numerator, denominator) pair under match — denominator is
// the model's total row count, numerator the count of those rows whose
// Disposition satisfies match. extraModels are additional model names
// to include in the result even if they have ZERO rows in the store
// (e.g. a reviewer slot that produced no disposition rows at all): each
// renders explicit 0/0 rather than being silently omitted, proving the
// divide-by-empty guard independently of any model that merely has a
// real zero NUMERATOR (denominator > 0) like fable's false-positive 0/5.
// Results are returned sorted by model name for deterministic output.
func ComputeModelRates(rows []DispositionRow, match func(disposition string) bool, extraModels ...string) []ModelRate {
	type acc struct{ numerator, denominator int }
	counts := make(map[string]*acc)

	for _, extra := range extraModels {
		if _, ok := counts[extra]; !ok {
			counts[extra] = &acc{}
		}
	}
	for _, row := range rows {
		a, ok := counts[row.Model]
		if !ok {
			a = &acc{}
			counts[row.Model] = a
		}
		a.denominator++
		if match(row.Disposition) {
			a.numerator++
		}
	}

	models := make([]string, 0, len(counts))
	for m := range counts {
		models = append(models, m)
	}
	sort.Strings(models)

	out := make([]ModelRate, 0, len(models))
	for _, m := range models {
		a := counts[m]
		out = append(out, ModelRate{Model: m, Numerator: a.numerator, Denominator: a.denominator})
	}
	return out
}

// ManifestModels returns the sorted, de-duplicated set of every model
// named in any coverage manifest's slots[] roster. Q1/Q2 union this
// with the models appearing in disposition ROWS so a model that was
// ROSTERED on a panel but produced ZERO disposition rows (a reviewer
// that found nothing) is still listed — as an explicit 0/0 — rather
// than silently dropped. That keeps the codex-effectiveness queries
// honest: "reviewed-but-found-nothing" is a visible outcome, not an
// absence (adjudicated from panel finding G1-001).
func ManifestModels(manifests []CoverageManifest) []string {
	seen := map[string]bool{}
	for _, m := range manifests {
		for _, s := range m.Slots {
			seen[s.Model] = true
		}
	}
	out := make([]string, 0, len(seen))
	for model := range seen {
		out = append(out, model)
	}
	sort.Strings(out)
	return out
}

// ComputeQ1 is the per-model genuine-find rate (genuine/total): the
// count of rows whose Disposition is in GenuineDispositions (Bead 1's
// single source of truth) over the model's total row count. extraModels
// (typically ManifestModels(manifests) at the CLI) are forced into the
// listing at 0/0 even with zero rows.
func ComputeQ1(rows []DispositionRow, extraModels ...string) []ModelRate {
	return ComputeModelRates(rows, func(d string) bool { return GenuineDispositions[d] }, extraModels...)
}

// ComputeQ2 is the per-model false-positive rate (false-positive/total):
// the count of rows whose Disposition is in FalsePositiveDispositions
// (Bead 1's single source of truth) over the model's total row count.
// extraModels (typically ManifestModels(manifests) at the CLI) are
// forced into the listing at 0/0 even with zero rows.
func ComputeQ2(rows []DispositionRow, extraModels ...string) []ModelRate {
	return ComputeModelRates(rows, func(d string) bool { return FalsePositiveDispositions[d] }, extraModels...)
}

// Q3Result is the convergence-rate query result: how many of the
// store's rows carry a non-empty ConvergentWith, out of the total row
// count, plus the ordered list of those rows themselves.
type Q3Result struct {
	ConvergentCount int
	Total           int
	ConvergentRows  []DispositionRow
}

// ComputeQ3 computes the convergence rate: rows with non-empty
// ConvergentWith over the total row count, plus the row list itself
// (sorted by Reviewer for deterministic output — row order in the
// source store depends on file-glob order, which is an incidental
// on-disk detail, not a meaningful sort key for this report).
func ComputeQ3(rows []DispositionRow) Q3Result {
	res := Q3Result{Total: len(rows)}
	for _, row := range rows {
		if len(row.ConvergentWith) > 0 {
			res.ConvergentCount++
			res.ConvergentRows = append(res.ConvergentRows, row)
		}
	}
	// Sort by Reviewer, then break ties deterministically on the
	// content-derived ID (unique per row) so the output order is stable
	// across runs even when two rows share a Reviewer token (F1-2).
	sort.Slice(res.ConvergentRows, func(i, j int) bool {
		a, b := res.ConvergentRows[i], res.ConvergentRows[j]
		if a.Reviewer != b.Reviewer {
			return a.Reviewer < b.Reviewer
		}
		return a.ID < b.ID
	})
	return res
}

// Q4Result is one canonical-gate row of the per-gate yield query: the
// count of GENUINE disposition rows whose Gate maps to this canonical
// key, and the denominator — the SUM of every coverage manifest's Slots
// roster length whose Gate matches (a gate spans multiple panels, e.g.
// "bead" spans panel-116-bead1/2/3a/3b, so its denominator sums all
// four panels' slot counts).
type Q4Result struct {
	Gate      string
	Genuine   int
	SlotTotal int
}

// ComputeQ4 computes the per-gate genuine-per-slot yield over
// CanonicalGateKeys, in that fixed canonical order (spec_approve,
// plan_approve, bead, final_review, adhoc) — every canonical gate
// appears in the result even if the store has no rows/manifests for it
// yet (0/0), consistent with the same never-drop discipline as Q1/Q2.
func ComputeQ4(rows []DispositionRow, manifests []CoverageManifest) []Q4Result {
	genuineByGate := make(map[string]int, len(CanonicalGateKeys))
	for _, row := range rows {
		if GenuineDispositions[row.Disposition] {
			genuineByGate[row.Gate]++
		}
	}
	slotsByGate := make(map[string]int, len(CanonicalGateKeys))
	for _, m := range manifests {
		slotsByGate[m.Gate] += len(m.Slots)
	}

	out := make([]Q4Result, 0, len(CanonicalGateKeys))
	for _, gate := range CanonicalGateKeys {
		out = append(out, Q4Result{
			Gate:      gate,
			Genuine:   genuineByGate[gate],
			SlotTotal: slotsByGate[gate],
		})
	}
	return out
}

// Q5Filter narrows ComputeQ5's finding listing. A zero-value (empty
// string) field means "no filter on this dimension" — safe because
// Gate/Severity/Disposition are all REQUIRED non-empty fields on every
// valid row (Validate's requireString rejects the empty string), so ""
// never collides with a real stored value.
type Q5Filter struct {
	Gate        string
	Severity    string
	Disposition string
}

// ComputeQ5 returns the subset of rows matching every non-empty
// dimension of filter (gate/severity/disposition) — the finding-by-model
// listing R3 mandates, filterable on those three axes. Rows are returned
// in the order they were loaded (deterministic per LoadStore).
//
// Each filter dimension is matched CASE-INSENSITIVELY (strings.EqualFold):
// stored gate/severity/disposition values are lowercase canonical tokens
// (e.g. "major", "bead", "false-contamination"), so an operator's
// `--severity MAJOR` matches the stored `major` rather than silently
// returning zero rows (M1).
func ComputeQ5(rows []DispositionRow, filter Q5Filter) []DispositionRow {
	var out []DispositionRow
	for _, row := range rows {
		if filter.Gate != "" && !strings.EqualFold(row.Gate, filter.Gate) {
			continue
		}
		if filter.Severity != "" && !strings.EqualFold(row.Severity, filter.Severity) {
			continue
		}
		if filter.Disposition != "" && !strings.EqualFold(row.Disposition, filter.Disposition) {
			continue
		}
		out = append(out, row)
	}
	return out
}

// EscapeList termsafe-escapes every element of ss and joins them with
// ", " — the shared rendering helper for a ConvergentWith list (agent-
// writable free text, R2), so the CLI's Q3 listing never renders a raw
// reviewer token that could carry a control byte or non-printable rune.
func EscapeList(ss []string) string {
	escaped := make([]string, len(ss))
	for i, s := range ss {
		escaped[i] = termsafe.Escape(s)
	}
	return strings.Join(escaped, ", ")
}
