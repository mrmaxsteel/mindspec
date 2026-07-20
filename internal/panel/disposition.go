// disposition.go: spec 117 Bead 1 — the on-disk JSON schema for the
// panel-disposition telemetry store (ADR-0043), the pure-function
// validator (R2/R6(a)), and the R5 path-hygiene predicate.
//
// A per-panel store file (`<spec-dir>/reviews/<panel>/dispositions.jsonl`)
// carries two record kinds, distinguished by the `record` discriminator:
//
//   - `record: "disposition"` — one row per distinct finding the
//     `/ms-panel-tally` authority adjudicated (DispositionRow).
//   - `record: "panel"` — one durable coverage manifest per terminal
//     panel, capturing every verdict-file slot's token/model/terminal
//     verdict (CoverageManifest).
//
// Both are agent-writable, forgeable-by-content repo data (ADR-0037 §8):
// this package validates SHAPE, never authenticity. Validate and
// HygienePredicate are pure functions over a single JSON line's raw
// bytes — no filesystem or git access — so Bead 2's transactional append
// op, Bead 3's query surface, and Bead 4's migration all share exactly
// one schema definition and one acceptance rule.
package panel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// Record discriminator values (R2). Any other value is REJECTED by
// Validate.
const (
	RecordDisposition   = "disposition"
	RecordPanelManifest = "panel"
)

// DispositionEnum is the closed, ordered `disposition` enum (R2) — the
// seed README's definitions verbatim. Validate rejects any value outside
// this set.
var DispositionEnum = []string{
	"confirmed-fixed",
	"confirmed-deferred",
	"confirmed-scope-trim",
	"deferred",
	"false-contamination",
	"audited-refuted",
}

// GenuineDispositions is the derived "genuine" set (R2): a finding the
// tally authority confirmed real, in any of its resolved shapes. This is
// the SINGLE SOURCE OF TRUTH every consumer (Bead 3's Q1/Q4) must use —
// no consumer may compute "genuine" with a different set.
var GenuineDispositions = map[string]bool{
	"confirmed-fixed":      true,
	"confirmed-deferred":   true,
	"confirmed-scope-trim": true,
}

// FalsePositiveDispositions is the derived "false-positive" set (R2):
// a finding that was NOT genuine. Plain `deferred` is accepted-but-
// unadjudicated and belongs to NEITHER set (denominator only) — it is
// intentionally absent from both maps below.
var FalsePositiveDispositions = map[string]bool{
	"false-contamination": true,
	"audited-refuted":     true,
}

// VerdictEnum is the closed terminal-verdict enum (R6(a)) for a coverage
// manifest's nested `slots[].verdict`.
var VerdictEnum = []string{"APPROVE", "REQUEST_CHANGES", "REJECT"}

// CanonicalGateKeys mirrors config.PanelGateKeys
// (internal/config/config.go) byte-for-byte: the closed, ordered enum of
// panel-gate keys (spec_approve/plan_approve/bead/final_review/adhoc) R2
// pins `gate` to.
//
// This is a DELIBERATE, commented duplication, not a drift risk taken
// lightly: internal/panel is a stdlib+termsafe+idvalidate leaf
// (TestPanelLeafImports_StdlibPlusTermsafeOnly, spec 116 AC7, amended by
// spec 120 R2) that may import NEITHER internal/config NOR any third
// internal package, so R2's `gate ∈ config.PanelGateKeys` requirement
// cannot be satisfied by importing config directly here. Instead,
// cmd/mindspec (which already imports both internal/config and
// internal/panel, e.g. panel.go's --gate handling) carries
// TestDispositionGateKeysMirrorConfig, asserting byte-for-byte parity
// between this slice and config.PanelGateKeys on every test run — so any
// future change to the canonical enum in EITHER copy fails a test
// immediately, exactly the "never silently duplicate" guarantee
// config.go's own comment asks for, honored across the leaf boundary
// instead of by import.
var CanonicalGateKeys = []string{"spec_approve", "plan_approve", "bead", "final_review", "adhoc"}

// requiredDispositionFields is the exhaustive required-field list for a
// `record: "disposition"` row (R2). Order is the order Validate reports
// a missing field in (first-missing-wins; deterministic for tests).
var requiredDispositionFields = []string{
	"record", "id", "spec", "gate", "panel", "reviewer", "model",
	"severity", "summary", "convergent_with", "disposition", "created_at",
	"backfilled",
}

// requiredManifestFields is the exhaustive required-field list for a
// `record: "panel"` coverage manifest (R6(a)).
var requiredManifestFields = []string{
	"record", "spec", "gate", "panel", "round", "slots", "backfilled",
}

// requiredSlotFields is the exhaustive required-field list for one
// nested `slots[]` entry of a coverage manifest (R6(a)).
var requiredSlotFields = []string{"slot", "model", "verdict"}

// DispositionRow mirrors the `record: "disposition"` JSON schema (R2):
// one durable row per distinct finding the `/ms-panel-tally` authority
// adjudicated. Backfilled is the migration-provenance marker: true on
// migrated seed rows (R4), false on live-captured rows. Round is absent
// on every migrated row (R4) — only manifests carry Round.
type DispositionRow struct {
	Record         string   `json:"record"`
	ID             string   `json:"id"`
	Spec           string   `json:"spec"`
	Gate           string   `json:"gate"`
	Panel          string   `json:"panel"`
	Reviewer       string   `json:"reviewer"`
	Model          string   `json:"model"`
	Severity       string   `json:"severity"`
	Summary        string   `json:"summary"`
	ConvergentWith []string `json:"convergent_with"`
	Disposition    string   `json:"disposition"`
	EvidenceRef    string   `json:"evidence_ref,omitempty"`
	Note           string   `json:"note,omitempty"`
	CreatedAt      string   `json:"created_at"`
	Round          *int     `json:"round,omitempty"`
	Backfilled     bool     `json:"backfilled"`
}

// ManifestSlot is one nested entry of a CoverageManifest's `slots[]`
// array (R6(a)): the verdict-file slot token, the actual model that
// produced it, and its terminal verdict.
type ManifestSlot struct {
	Slot    string `json:"slot"`
	Model   string `json:"model"`
	Verdict string `json:"verdict"`
}

// CoverageManifest mirrors the `record: "panel"` JSON schema (R6(a)):
// the durable per-terminal-panel coverage manifest that makes the R1(b)
// completeness floor and Q4's per-slot denominator derivable from the
// store alone, with no raw-verdict-file dependency. Every terminal
// panel writes exactly one of these, including a finding-less
// all-APPROVE panel (Slots non-empty, zero disposition rows).
type CoverageManifest struct {
	Record     string         `json:"record"`
	Spec       string         `json:"spec"`
	Gate       string         `json:"gate"`
	Panel      string         `json:"panel"`
	Round      int            `json:"round"`
	Slots      []ManifestSlot `json:"slots"`
	Backfilled bool           `json:"backfilled"`
}

// isValidGate reports whether gate is one of CanonicalGateKeys (mirrored
// from config.PanelGateKeys — see its doc comment for why this package
// cannot import internal/config directly).
func isValidGate(gate string) bool {
	for _, k := range CanonicalGateKeys {
		if gate == k {
			return true
		}
	}
	return false
}

// isValidDisposition reports whether d is a member of DispositionEnum.
func isValidDisposition(d string) bool {
	for _, k := range DispositionEnum {
		if d == k {
			return true
		}
	}
	return false
}

// isValidVerdict reports whether v is a member of VerdictEnum.
func isValidVerdict(v string) bool {
	for _, k := range VerdictEnum {
		if v == k {
			return true
		}
	}
	return false
}

// decodeRecord decodes one JSON line into a generic map, preserving
// number literals as json.Number (UseNumber) so an integer-vs-float
// distinction (e.g. `round`) can be enforced without a lossy round
// trip through float64. Every render of the offending raw bytes routes
// through termsafe.Escape (ADR-0042) — the input is agent-writable,
// untrusted-provenance text.
func decodeRecord(data []byte) (map[string]interface{}, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var m map[string]interface{}
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("disposition record is not valid JSON: %s", termsafe.Escape(err.Error()))
	}
	// A second Decode call must hit io.EOF: a JSONL "line" file holding
	// more than one JSON value is not this function's contract (the
	// CLI leaf iterates lines itself); reject silently-truncated input
	// (e.g. a line with trailing garbage) rather than accept only its
	// first value.
	var extra json.RawMessage
	if err := dec.Decode(&extra); err == nil {
		return nil, fmt.Errorf("disposition record contains more than one JSON value")
	}
	if m == nil {
		return nil, fmt.Errorf("disposition record decoded to a null/empty JSON value")
	}
	return m, nil
}

// requireString returns m[field] as a string, erroring if the field is
// absent, not a JSON string, or the EMPTY string. kind labels the record
// kind ("disposition" row or "panel" manifest) in the error for a
// clearer recovery line. Empty is rejected uniformly because every
// caller is a REQUIRED string field (row id/spec/gate/panel/reviewer/
// model/severity/summary/disposition/created_at; manifest spec/gate/
// panel + each slot's slot/model/verdict) whose empty value is
// meaningless downstream — e.g. an empty `reviewer` cannot byte-match a
// slot token (R1), an empty `spec`/`panel` breaks the store's per-panel
// glob key. Optional fields never reach here (they use optionalString),
// and `convergent_with[]` entries are validated by requireStringArray,
// so this NEVER rejects a legitimate empty array `convergent_with: []`
// or an absent optional.
func requireString(m map[string]interface{}, field, kind string) (string, error) {
	v, ok := m[field]
	if !ok {
		return "", fmt.Errorf("%s record is missing required field %s", kind, termsafe.Escape(field))
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s record field %s must be a string, got %s", kind, termsafe.Escape(field), jsonTypeName(v))
	}
	if s == "" {
		return "", fmt.Errorf("%s record field %s must be a non-empty string", kind, termsafe.Escape(field))
	}
	return s, nil
}

// optionalString errors if m[field] IS present but not a JSON string
// (R2's "reject wrong-type present optionals"). Absence is not an
// error; the caller (Validate) validates shape only, so no value is
// returned.
func optionalString(m map[string]interface{}, field, kind string) error {
	v, ok := m[field]
	if !ok {
		return nil
	}
	if _, ok := v.(string); !ok {
		return fmt.Errorf("%s record field %s (optional) must be a string when present, got %s", kind, termsafe.Escape(field), jsonTypeName(v))
	}
	return nil
}

// requireBool errors if m[field] is absent or not a JSON boolean. The
// caller (Validate) validates shape only, so no value is returned.
func requireBool(m map[string]interface{}, field, kind string) error {
	v, ok := m[field]
	if !ok {
		return fmt.Errorf("%s record is missing required field %s", kind, termsafe.Escape(field))
	}
	if _, ok := v.(bool); !ok {
		return fmt.Errorf("%s record field %s must be a boolean, got %s", kind, termsafe.Escape(field), jsonTypeName(v))
	}
	return nil
}

// requireInt errors if m[field] is absent, not a JSON number, or a
// number with a fractional/exponent component (R6(a)'s `round` must be
// a whole number). The caller (Validate) validates shape only, so no
// value is returned.
func requireInt(m map[string]interface{}, field, kind string) error {
	v, ok := m[field]
	if !ok {
		return fmt.Errorf("%s record is missing required field %s", kind, termsafe.Escape(field))
	}
	return validateIntegerNumber(v, field, kind)
}

// optionalInt mirrors optionalString for an integer-valued optional
// field (DispositionRow.Round).
func optionalInt(m map[string]interface{}, field, kind string) error {
	v, ok := m[field]
	if !ok {
		return nil
	}
	return validateIntegerNumber(v, field, kind)
}

// validateIntegerNumber errors unless v is a json.Number with no
// fractional/exponent component.
func validateIntegerNumber(v interface{}, field, kind string) error {
	num, ok := v.(json.Number)
	if !ok {
		return fmt.Errorf("%s record field %s must be an integer, got %s", kind, termsafe.Escape(field), jsonTypeName(v))
	}
	if _, err := num.Int64(); err != nil {
		return fmt.Errorf("%s record field %s must be a whole-number integer, got %s", kind, termsafe.Escape(field), termsafe.Escape(num.String()))
	}
	return nil
}

// requireStringArray returns m[field] as a []string, erroring if the
// field is absent, not a JSON array, or contains a non-string element
// (R2's `convergent_with[]`).
func requireStringArray(m map[string]interface{}, field, kind string) ([]string, error) {
	v, ok := m[field]
	if !ok {
		return nil, fmt.Errorf("%s record is missing required field %s", kind, termsafe.Escape(field))
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%s record field %s must be an array, got %s", kind, termsafe.Escape(field), jsonTypeName(v))
	}
	out := make([]string, 0, len(arr))
	for i, el := range arr {
		s, ok := el.(string)
		if !ok {
			return nil, fmt.Errorf("%s record field %s[%d] must be a string, got %s", kind, termsafe.Escape(field), i, jsonTypeName(el))
		}
		out = append(out, s)
	}
	return out, nil
}

// jsonTypeName names v's decoded JSON type for an error message —
// never the value itself (values route through termsafe.Escape at
// their own call sites; a type name is not agent-controlled free text).
func jsonTypeName(v interface{}) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case json.Number:
		return "number"
	case string:
		return "string"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	default:
		return "unknown"
	}
}

// Validate parses one JSONL line (raw bytes) and enforces the R2 row
// schema or the R6(a) manifest schema, dispatching on the `record`
// discriminator. It enforces the R2 completeness property: no malformed
// disposition row OR coverage manifest passes — a bad `record`
// discriminator, a missing or wrong-type value in any required field, a
// wrong-type value in any PRESENT optional field, an out-of-enum
// `gate`/`disposition`/nested `verdict`, or a non-RFC-3339 `created_at`
// are all rejected. Validate makes no filesystem or git call and applies
// no path-hygiene check (see HygienePredicate) — the two are separate,
// composable gates, exactly as R6(b)'s append op runs both before any
// mutation.
func Validate(data []byte) error {
	m, err := decodeRecord(data)
	if err != nil {
		return err
	}

	recordVal, ok := m["record"]
	if !ok {
		return fmt.Errorf("disposition record is missing required field %s", termsafe.Escape("record"))
	}
	record, ok := recordVal.(string)
	if !ok {
		return fmt.Errorf("disposition record field %s must be a string, got %s", termsafe.Escape("record"), jsonTypeName(recordVal))
	}

	switch record {
	case RecordDisposition:
		return validateDispositionMap(m)
	case RecordPanelManifest:
		return validateManifestMap(m)
	default:
		return fmt.Errorf("disposition record field %s must be one of {%q,%q}, got %s", termsafe.Escape("record"), RecordDisposition, RecordPanelManifest, termsafe.Escape(record))
	}
}

// validateDispositionMap validates a `record: "disposition"` row's
// already-decoded fields against R2.
func validateDispositionMap(m map[string]interface{}) error {
	const kind = "disposition"

	for _, f := range requiredDispositionFields {
		if _, ok := m[f]; !ok {
			return fmt.Errorf("%s record is missing required field %s", kind, termsafe.Escape(f))
		}
	}

	if _, err := requireString(m, "id", kind); err != nil {
		return err
	}
	if _, err := requireString(m, "spec", kind); err != nil {
		return err
	}
	gate, err := requireString(m, "gate", kind)
	if err != nil {
		return err
	}
	if !isValidGate(gate) {
		return fmt.Errorf("%s record field %s must be one of %v, got %s", kind, termsafe.Escape("gate"), CanonicalGateKeys, termsafe.Escape(gate))
	}
	if _, err := requireString(m, "panel", kind); err != nil {
		return err
	}
	if _, err := requireString(m, "reviewer", kind); err != nil {
		return err
	}
	if _, err := requireString(m, "model", kind); err != nil {
		return err
	}
	if _, err := requireString(m, "severity", kind); err != nil {
		return err
	}
	if _, err := requireString(m, "summary", kind); err != nil {
		return err
	}
	if _, err := requireStringArray(m, "convergent_with", kind); err != nil {
		return err
	}
	disposition, err := requireString(m, "disposition", kind)
	if err != nil {
		return err
	}
	if !isValidDisposition(disposition) {
		return fmt.Errorf("%s record field %s must be one of %v, got %s", kind, termsafe.Escape("disposition"), DispositionEnum, termsafe.Escape(disposition))
	}
	createdAt, err := requireString(m, "created_at", kind)
	if err != nil {
		return err
	}
	if _, err := time.Parse(time.RFC3339, createdAt); err != nil {
		return fmt.Errorf("%s record field %s must be RFC-3339, got %s", kind, termsafe.Escape("created_at"), termsafe.Escape(createdAt))
	}
	if err := requireBool(m, "backfilled", kind); err != nil {
		return err
	}

	// Present-optional fields: absence is fine, wrong type is not.
	if err := optionalString(m, "evidence_ref", kind); err != nil {
		return err
	}
	if err := optionalString(m, "note", kind); err != nil {
		return err
	}
	if err := optionalInt(m, "round", kind); err != nil {
		return err
	}

	return nil
}

// validateManifestMap validates a `record: "panel"` coverage manifest's
// already-decoded fields against R6(a).
func validateManifestMap(m map[string]interface{}) error {
	const kind = "panel"

	for _, f := range requiredManifestFields {
		if _, ok := m[f]; !ok {
			return fmt.Errorf("%s record is missing required field %s", kind, termsafe.Escape(f))
		}
	}

	if _, err := requireString(m, "spec", kind); err != nil {
		return err
	}
	gate, err := requireString(m, "gate", kind)
	if err != nil {
		return err
	}
	if !isValidGate(gate) {
		return fmt.Errorf("%s record field %s must be one of %v, got %s", kind, termsafe.Escape("gate"), CanonicalGateKeys, termsafe.Escape(gate))
	}
	if _, err := requireString(m, "panel", kind); err != nil {
		return err
	}
	if err := requireInt(m, "round", kind); err != nil {
		return err
	}
	if err := requireBool(m, "backfilled", kind); err != nil {
		return err
	}

	slotsVal, ok := m["slots"]
	if !ok {
		return fmt.Errorf("%s record is missing required field %s", kind, termsafe.Escape("slots"))
	}
	slotsArr, ok := slotsVal.([]interface{})
	if !ok {
		return fmt.Errorf("%s record field %s must be an array, got %s", kind, termsafe.Escape("slots"), jsonTypeName(slotsVal))
	}
	// A terminal panel always has ≥1 reviewer slot (spec/plan ≈ 9–12,
	// bead = 8, final = 12); an empty roster would give Q4 a zero
	// denominator and prove nothing about the floor. Reject it (FB1).
	if len(slotsArr) == 0 {
		return fmt.Errorf("%s record field %s must be a non-empty array (a terminal panel has at least one reviewer slot)", kind, termsafe.Escape("slots"))
	}
	seenSlots := make(map[string]bool, len(slotsArr))
	for i, raw := range slotsArr {
		slot, ok := raw.(map[string]interface{})
		if !ok {
			return fmt.Errorf("%s record field %s[%d] must be an object, got %s", kind, termsafe.Escape("slots"), i, jsonTypeName(raw))
		}
		for _, f := range requiredSlotFields {
			if _, ok := slot[f]; !ok {
				return fmt.Errorf("%s record field slots[%d] is missing required field %s", kind, i, termsafe.Escape(f))
			}
		}
		slotToken, err := requireString(slot, "slot", fmt.Sprintf("%s slots[%d]", kind, i))
		if err != nil {
			return err
		}
		// Slot tokens are the per-panel slot identity (R1); a duplicate
		// would double-count that slot in Q4's per-slot denominator and
		// let one covering row satisfy the floor for two distinct
		// verdicts. Reject any repeat within a manifest (FB2).
		if seenSlots[slotToken] {
			return fmt.Errorf("%s record field %s has a duplicate slot token %s (each slot must appear at most once)", kind, termsafe.Escape("slots"), termsafe.Escape(slotToken))
		}
		seenSlots[slotToken] = true
		if _, err := requireString(slot, "model", fmt.Sprintf("%s slots[%d]", kind, i)); err != nil {
			return err
		}
		verdict, err := requireString(slot, "verdict", fmt.Sprintf("%s slots[%d]", kind, i))
		if err != nil {
			return err
		}
		if !isValidVerdict(verdict) {
			return fmt.Errorf("%s record field slots[%d].verdict must be one of %v, got %s", kind, i, VerdictEnum, termsafe.Escape(verdict))
		}
	}

	return nil
}

// hygieneTokens are the R5 forbidden path-prefix substrings: a stored
// row or manifest must never embed a machine-local absolute path (the
// fl91-adjacent leak class). Checked as a plain substring, not just a
// field prefix, because free prose (`note`) can embed the path anywhere
// in the string (e.g. "raw verdict lived at /tmp/o1-final116/...").
var hygieneTokens = []string{"/Users/", "/tmp/"}

// HygienePredicate parses one JSONL line (raw bytes) and rejects it if
// ANY string value anywhere in the record — top-level field, nested
// `slots[]` entry, or array element, over EITHER record kind — contains
// a `/Users/`- or `/tmp/`-prefixed path token (R5). It is independent of
// Validate: a record can fail hygiene regardless of whether it is
// otherwise schema-valid, and R6(b)'s append op runs both gates before
// any file mutation.
func HygienePredicate(data []byte) error {
	m, err := decodeRecord(data)
	if err != nil {
		return err
	}
	return walkHygiene(m, "$")
}

// walkHygiene recursively scans v (a decoded JSON value) for a
// forbidden path token in any string leaf, reporting path as a
// termsafe-escaped JSON-pointer-ish breadcrumb for the recovery line.
func walkHygiene(v interface{}, path string) error {
	switch val := v.(type) {
	case string:
		for _, tok := range hygieneTokens {
			if strings.Contains(val, tok) {
				return fmt.Errorf("disposition record field %s contains a forbidden path token %s: %s", termsafe.Escape(path), termsafe.Escape(tok), termsafe.Escape(val))
			}
		}
	case map[string]interface{}:
		for k, sub := range val {
			if err := walkHygiene(sub, path+"."+k); err != nil {
				return err
			}
		}
	case []interface{}:
		for i, sub := range val {
			if err := walkHygiene(sub, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}
