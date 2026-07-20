package panel

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// fixtureFiles returns the sorted list of *.jsonl basenames directly
// inside dir (internal/panel/testdata/disposition/{valid,invalid}), one
// per negative/positive fixture (Bead 1 plan Step 4/6).
func fixtureFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		names = append(names, e.Name())
	}
	if len(names) == 0 {
		t.Fatalf("%s contains no fixture files — the scan found nothing to check", dir)
	}
	sort.Strings(names)
	return names
}

// TestDispositionValidate_ValidFixturesAccept is the accept half of AC3:
// every one of the 21 migrated disposition rows + 8 migrated coverage
// manifests under testdata/disposition/valid/ passes Validate.
func TestDispositionValidate_ValidFixturesAccept(t *testing.T) {
	dir := filepath.Join("testdata", "disposition", "valid")
	names := fixtureFiles(t, dir)

	rows, manifests := 0, 0
	for _, name := range names {
		name := name
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("reading fixture: %v", err)
			}
			if err := Validate(data); err != nil {
				t.Fatalf("Validate(%s) = %v, want nil (valid fixture must ACCEPT)", name, err)
			}
		})
		switch {
		case len(name) >= 3 && name[:3] == "row":
			rows++
		case len(name) >= 8 && name[:8] == "manifest":
			manifests++
		}
	}

	if rows != 21 {
		t.Errorf("valid/ fixture set has %d row-*.jsonl files, want exactly 21 (the migrated spec-116 seed)", rows)
	}
	if manifests != 8 {
		t.Errorf("valid/ fixture set has %d manifest-*.jsonl files, want exactly 8 (one per spec-116 panel)", manifests)
	}
}

// validateAndHygiene runs BOTH gates the CLI's `validate` leaf runs per
// line (Bead 1 plan Step 5) and returns the first failure, exactly the
// combined accept/reject surface an invalid/ fixture is judged against:
// a fixture can be invalid under EITHER Validate's schema check or
// HygienePredicate's path-token check (or both), and either one alone
// is enough for the CLI to refuse the line.
func validateAndHygiene(data []byte) error {
	if err := Validate(data); err != nil {
		return err
	}
	return HygienePredicate(data)
}

// TestDispositionValidate_InvalidFixturesReject is the reject half of
// AC3: every fixture under testdata/disposition/invalid/ — the
// exhaustive per-field/per-nested-field negative matrix (Bead 1 plan
// Step 4), plus the R5 hygiene fixtures — fails the combined
// Validate+HygienePredicate gate the CLI leaf runs.
func TestDispositionValidate_InvalidFixturesReject(t *testing.T) {
	dir := filepath.Join("testdata", "disposition", "invalid")
	names := fixtureFiles(t, dir)

	for _, name := range names {
		name := name
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("reading fixture: %v", err)
			}
			if err := validateAndHygiene(data); err == nil {
				t.Fatalf("validateAndHygiene(%s) = nil, want a non-nil error (invalid fixture must REJECT)", name)
			}
		})
	}
}

// TestHygienePredicate_ValidFixturesClean is AC5's first half: zero
// /Users//tmp path tokens across every valid fixture (both record
// kinds).
func TestHygienePredicate_ValidFixturesClean(t *testing.T) {
	dir := filepath.Join("testdata", "disposition", "valid")
	names := fixtureFiles(t, dir)

	for _, name := range names {
		name := name
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("reading fixture: %v", err)
			}
			if err := HygienePredicate(data); err != nil {
				t.Fatalf("HygienePredicate(%s) = %v, want nil (valid fixtures carry no /Users//tmp tokens)", name, err)
			}
		})
	}
}

// TestHygienePredicate_RejectsPathTokens is AC5's second half: a fixture
// carrying a /Users/... or /tmp/... token is REJECTED, over both record
// kinds. It also proves HygienePredicate runs independently of
// Validate: the two hygiene fixtures below are otherwise schema-valid
// (built from the same base as testdata/disposition/valid), so a naive
// "hygiene only checked as part of Validate" implementation would wrongly
// let a hygiene-only violation through some other path.
func TestHygienePredicate_RejectsPathTokens(t *testing.T) {
	dir := filepath.Join("testdata", "disposition", "invalid")
	cases := []string{
		"row-hygiene-users-path.jsonl",
		"row-hygiene-tmp-path.jsonl",
		"manifest-hygiene-users-path.jsonl",
		"manifest-hygiene-tmp-path.jsonl",
	}
	for _, name := range cases {
		name := name
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("reading fixture: %v", err)
			}
			if err := HygienePredicate(data); err == nil {
				t.Fatalf("HygienePredicate(%s) = nil, want a non-nil error (a /Users/ or /tmp/ path token must be rejected)", name)
			}
		})
	}
}

// TestDispositionValidate_LeniencyTightenings names the Bead-1 panel's
// convergent findings (FB1/FB2/FB3 + O1's completeness fixtures): each
// fixture below must be REJECTED by Validate, and the leniency it targets
// would have bitten Bead 2's append op or Bead 3's Q4 denominator. The
// glob-driven TestDispositionValidate_InvalidFixturesReject also covers
// these files, but this test documents WHICH leniency each closes and
// asserts Validate itself (not the combined CLI gate) rejects them.
func TestDispositionValidate_LeniencyTightenings(t *testing.T) {
	dir := filepath.Join("testdata", "disposition", "invalid")
	cases := map[string]string{
		"manifest-empty-slots.jsonl":    "FB1: a coverage manifest with slots:[] must be rejected (a terminal panel has >=1 slot)",
		"manifest-dup-slot.jsonl":       "FB2: duplicate slots[].slot tokens must be rejected (would double-count Q4)",
		"row-empty-id.jsonl":            "FB3: an empty-string required row field must be rejected",
		"manifest-empty-panel.jsonl":    "FB3: an empty-string required manifest field must be rejected",
		"manifest-slot-nonobject.jsonl": "O1: a slots[] element that is a bare scalar (not an object) must be rejected",
		"row-round-fractional.jsonl":    "O1: a fractional round (1.5) must be rejected",
	}
	for name, why := range cases {
		name, why := name, why
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("reading fixture: %v", err)
			}
			if err := Validate(data); err == nil {
				t.Fatalf("Validate(%s) = nil, want a non-nil error — %s", name, why)
			}
		})
	}
}

// TestDispositionValidate_EmptyStringExemptions guards FB3's precise
// boundary: the empty-required-string rejection must NOT spill onto the
// legitimately-empty positions — an empty ARRAY convergent_with:[] and
// absent optionals (evidence_ref/note/round) — which every migrated seed
// row and manifest relies on. These records ACCEPT.
func TestDispositionValidate_EmptyStringExemptions(t *testing.T) {
	// A minimal row with convergent_with:[] and NO optional fields.
	row := `{"record":"disposition","id":"d-1","spec":"116","gate":"spec_approve","panel":"p","reviewer":"F1","model":"fable","severity":"major","summary":"s","convergent_with":[],"disposition":"confirmed-fixed","created_at":"2026-07-11T00:00:00Z","backfilled":true}`
	if err := Validate([]byte(row)); err != nil {
		t.Fatalf("Validate(minimal row with convergent_with:[] and no optionals) = %v, want nil", err)
	}
	// A minimal manifest with a single slot.
	manifest := `{"record":"panel","spec":"116","gate":"spec_approve","panel":"p","round":1,"slots":[{"slot":"F1","model":"fable","verdict":"APPROVE"}],"backfilled":true}`
	if err := Validate([]byte(manifest)); err != nil {
		t.Fatalf("Validate(minimal single-slot manifest) = %v, want nil", err)
	}
}

// TestDispositionValidate_RecordDiscriminator pins the two record-kind
// constants and confirms a third value is rejected (belt-and-suspenders
// alongside the fixture-driven tests above, naming the exact behavior).
func TestDispositionValidate_RecordDiscriminator(t *testing.T) {
	if RecordDisposition != "disposition" {
		t.Fatalf("RecordDisposition = %q, want %q", RecordDisposition, "disposition")
	}
	if RecordPanelManifest != "panel" {
		t.Fatalf("RecordPanelManifest = %q, want %q", RecordPanelManifest, "panel")
	}
	if err := Validate([]byte(`{"record":"nope"}`)); err == nil {
		t.Fatal("Validate of an out-of-enum record discriminator = nil, want error")
	}
}

// TestDispositionValidate_GenuineFalsePositiveSetsPartitioned pins R2's
// derived-metric invariant: genuine and false-positive are disjoint,
// plain "deferred" is in neither, and every DispositionEnum member is
// classified as exactly one of {genuine, false-positive, neither}.
func TestDispositionValidate_GenuineFalsePositiveSetsPartitioned(t *testing.T) {
	for _, d := range DispositionEnum {
		g, fp := GenuineDispositions[d], FalsePositiveDispositions[d]
		if g && fp {
			t.Errorf("disposition %q is in BOTH GenuineDispositions and FalsePositiveDispositions", d)
		}
	}
	if GenuineDispositions["deferred"] || FalsePositiveDispositions["deferred"] {
		t.Error(`"deferred" must be in NEITHER GenuineDispositions nor FalsePositiveDispositions (denominator-only, R2)`)
	}
	wantGenuine := []string{"confirmed-fixed", "confirmed-deferred", "confirmed-scope-trim"}
	for _, d := range wantGenuine {
		if !GenuineDispositions[d] {
			t.Errorf("GenuineDispositions[%q] = false, want true", d)
		}
	}
	wantFalsePositive := []string{"false-contamination", "audited-refuted"}
	for _, d := range wantFalsePositive {
		if !FalsePositiveDispositions[d] {
			t.Errorf("FalsePositiveDispositions[%q] = false, want true", d)
		}
	}
}

// TestDispositionValidate_MalformedJSONRejected covers the non-JSON /
// trailing-garbage / null-value legs of decodeRecord that the fixture
// matrix does not exercise via files (they are decode-time failures,
// not schema failures).
func TestDispositionValidate_MalformedJSONRejected(t *testing.T) {
	cases := map[string][]byte{
		"not json at all":      []byte("not json at all"),
		"empty string":         []byte(""),
		"trailing garbage":     []byte(`{"record":"disposition"} {"extra":true}`),
		"bare null":            []byte(`null`),
		"array instead of obj": []byte(`["record","disposition"]`),
	}
	for name, data := range cases {
		name, data := name, data
		t.Run(name, func(t *testing.T) {
			if err := Validate(data); err == nil {
				t.Fatalf("Validate(%s) = nil, want error", name)
			}
		})
	}
}
