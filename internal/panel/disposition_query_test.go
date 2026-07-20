package panel

// disposition_query_test.go: spec 117 Bead 3 — pins AC3's query half.
// Every Q1-Q4 number here is the EXACT value AC3 pins (re-derivable with
// jq over the raw archive seed per the plan's documented cross-check),
// computed over the self-contained testdata/seed116 fixture — the
// MIGRATED-form 21 rows + 8 coverage manifests Bead 3 checks in
// independently of Bead 4's live migration.

import (
	"path/filepath"
	"sort"
	"testing"
)

// loadSeed116 loads the checked-in migrated-form seed fixture via
// LoadStore + the store-wide glob shape (one dispositions.jsonl per
// panel subdirectory, mirroring the real .mindspec/specs/<spec>/reviews
// layout but self-contained under testdata/).
func loadSeed116(t *testing.T) ([]DispositionRow, []CoverageManifest) {
	t.Helper()
	pattern := filepath.Join("testdata", "seed116", "*", "dispositions.jsonl")
	rows, manifests, err := LoadStore(pattern)
	if err != nil {
		t.Fatalf("LoadStore(%s) = %v, want nil", pattern, err)
	}
	if len(rows) != 21 {
		t.Fatalf("loaded %d disposition rows from seed116, want exactly 21", len(rows))
	}
	if len(manifests) != 8 {
		t.Fatalf("loaded %d coverage manifests from seed116, want exactly 8", len(manifests))
	}
	return rows, manifests
}

// rateMap indexes a []ModelRate by Model for easy pinned-value lookups.
func rateMap(rates []ModelRate) map[string]ModelRate {
	m := make(map[string]ModelRate, len(rates))
	for _, r := range rates {
		m[r.Model] = r
	}
	return m
}

// TestQueryMetrics_Q1GenuinePerModel pins AC3's Q1 numbers exactly:
// fable 3/5, opus 4/6, sonnet 6/7, gpt-5.6-sol 2/3.
func TestQueryMetrics_Q1GenuinePerModel(t *testing.T) {
	rows, _ := loadSeed116(t)
	got := rateMap(ComputeQ1(rows))

	want := map[string]ModelRate{
		"fable":       {Model: "fable", Numerator: 3, Denominator: 5},
		"opus":        {Model: "opus", Numerator: 4, Denominator: 6},
		"sonnet":      {Model: "sonnet", Numerator: 6, Denominator: 7},
		"gpt-5.6-sol": {Model: "gpt-5.6-sol", Numerator: 2, Denominator: 3},
	}
	if len(got) != len(want) {
		t.Fatalf("Q1 returned %d models, want %d: got=%+v", len(got), len(want), got)
	}
	for model, w := range want {
		g, ok := got[model]
		if !ok {
			t.Fatalf("Q1 missing model %q entirely — a model must never be dropped", model)
		}
		if g != w {
			t.Errorf("Q1[%s] = %s, want %s", model, g.Render(), w.Render())
		}
	}
}

// TestQueryMetrics_Q2FalsePositivePerModel pins AC3's Q2 numbers
// exactly: fable 0/5, opus 1/6, sonnet 1/7, gpt-5.6-sol 1/3. fable's 0/5
// is the REAL zero-numerator case AC3 names explicitly — distinct from
// the synthetic zero-DENOMINATOR guard in
// TestQueryMetrics_ZeroRowModelDivideByEmptyGuard below.
func TestQueryMetrics_Q2FalsePositivePerModel(t *testing.T) {
	rows, _ := loadSeed116(t)
	got := rateMap(ComputeQ2(rows))

	want := map[string]ModelRate{
		"fable":       {Model: "fable", Numerator: 0, Denominator: 5},
		"opus":        {Model: "opus", Numerator: 1, Denominator: 6},
		"sonnet":      {Model: "sonnet", Numerator: 1, Denominator: 7},
		"gpt-5.6-sol": {Model: "gpt-5.6-sol", Numerator: 1, Denominator: 3},
	}
	if len(got) != len(want) {
		t.Fatalf("Q2 returned %d models, want %d: got=%+v", len(got), len(want), got)
	}
	for model, w := range want {
		g, ok := got[model]
		if !ok {
			t.Fatalf("Q2 missing model %q entirely — a model must never be dropped", model)
		}
		if g != w {
			t.Errorf("Q2[%s] = %s, want %s", model, g.Render(), w.Render())
		}
		if model == "fable" && g.Numerator != 0 {
			t.Errorf("Q2[fable] numerator = %d, want the pinned real zero-numerator 0", g.Numerator)
		}
	}
	if got["fable"].Render() != "0/5" {
		t.Errorf(`Q2[fable].Render() = %q, want "0/5"`, got["fable"].Render())
	}
}

// TestQueryMetrics_Q3Convergence pins AC3's Q3 number: 4 of 21 rows
// carry a non-empty convergent_with, and names the exact 4 reviewers —
// G1-codex, O1, S1, S-tests (the last canonicalized from the raw
// seed's "Sonnet-tests" per R4/C1).
func TestQueryMetrics_Q3Convergence(t *testing.T) {
	rows, _ := loadSeed116(t)
	res := ComputeQ3(rows)

	if res.Total != 21 {
		t.Fatalf("Q3 total = %d, want 21", res.Total)
	}
	if res.ConvergentCount != 4 {
		t.Fatalf("Q3 convergent count = %d, want 4", res.ConvergentCount)
	}
	if len(res.ConvergentRows) != 4 {
		t.Fatalf("Q3 convergent row list has %d entries, want 4", len(res.ConvergentRows))
	}

	gotReviewers := make([]string, len(res.ConvergentRows))
	for i, r := range res.ConvergentRows {
		gotReviewers[i] = r.Reviewer
	}
	sort.Strings(gotReviewers)
	want := []string{"G1-codex", "O1", "S1", "S-tests"}
	sort.Strings(want)
	for i := range want {
		if gotReviewers[i] != want[i] {
			t.Errorf("Q3 convergent reviewers = %v, want (as a set) %v", gotReviewers, want)
			break
		}
	}
}

// TestQueryMetrics_Q4PerGateYield pins AC3's Q4 numbers exactly:
// spec_approve 5/9, plan_approve 3/9, bead 3/32, final_review 2/12,
// adhoc 2/4, in that canonical-gate order.
func TestQueryMetrics_Q4PerGateYield(t *testing.T) {
	rows, manifests := loadSeed116(t)
	got := ComputeQ4(rows, manifests)

	want := []Q4Result{
		{Gate: "spec_approve", Genuine: 5, SlotTotal: 9},
		{Gate: "plan_approve", Genuine: 3, SlotTotal: 9},
		{Gate: "bead", Genuine: 3, SlotTotal: 32},
		{Gate: "final_review", Genuine: 2, SlotTotal: 12},
		{Gate: "adhoc", Genuine: 2, SlotTotal: 4},
	}
	if len(got) != len(want) {
		t.Fatalf("Q4 returned %d gates, want %d: got=%+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("Q4[%d] = %+v, want %+v", i, got[i], w)
		}
	}
}

// TestQueryMetrics_ZeroRowModelDivideByEmptyGuard is the divide-by-EMPTY
// guard distinct from fable's real 0/5 (a real zero NUMERATOR over a
// non-empty denominator): a model with literally ZERO rows in the store
// (N=0) must still render explicit "0/0" — never dropped from the
// listing, and never computed via a raw float division that would
// yield NaN/+Inf.
func TestQueryMetrics_ZeroRowModelDivideByEmptyGuard(t *testing.T) {
	rows, _ := loadSeed116(t)
	const zeroRowModel = "zero-row-synthetic-model"

	rates := rateMap(ComputeQ1(rows, zeroRowModel))
	got, ok := rates[zeroRowModel]
	if !ok {
		t.Fatalf("ComputeQ1 with an explicit zero-row extraModel dropped it entirely — a model must never be silently omitted")
	}
	if got.Numerator != 0 || got.Denominator != 0 {
		t.Fatalf("zero-row model rate = %+v, want Numerator=0 Denominator=0", got)
	}
	if render := got.Render(); render != "0/0" {
		t.Errorf(`zero-row model Render() = %q, want "0/0"`, render)
	}
	if pct := got.Percent(); pct != 0 {
		t.Errorf("zero-row model Percent() = %v, want 0 (guarded, not NaN/+Inf from a 0/0 float division)", pct)
	}

	// Same guard for Q2.
	rates2 := rateMap(ComputeQ2(rows, zeroRowModel))
	got2, ok := rates2[zeroRowModel]
	if !ok {
		t.Fatalf("ComputeQ2 with an explicit zero-row extraModel dropped it entirely")
	}
	if render := got2.Render(); render != "0/0" {
		t.Errorf(`Q2 zero-row model Render() = %q, want "0/0"`, render)
	}

	// Sanity: fable's REAL zero-numerator case is a DIFFERENT shape
	// (0/5, non-empty denominator) — confirm the two guards are not
	// accidentally testing the same thing.
	fable := rateMap(ComputeQ2(rows))["fable"]
	if fable.Denominator == 0 {
		t.Fatalf("fable's Q2 denominator is 0 in the seed — the fixture no longer distinguishes the two guard cases")
	}
	if fable.Render() == got2.Render() {
		t.Fatalf("fable's real 0/N (%s) coincides with the synthetic 0/0 guard render (%s) — fixture drift", fable.Render(), got2.Render())
	}
}

// TestQueryMetrics_ManifestRosteredZeroRowModelShownAs00 pins FIX-B
// (finding G1-001): Q1/Q2 union their model set with the models rostered
// in the coverage MANIFESTS' slots[], so a model that was reviewed on a
// panel but produced ZERO disposition rows is listed as an explicit 0/0
// rather than silently dropped ("reviewed-but-found-nothing" must stay
// visible for the codex-effectiveness queries). This is distinct from
// fable's real 0/5 (a zero NUMERATOR over a non-empty denominator): here
// the model has NO rows at all, discovered only via the manifest roster.
func TestQueryMetrics_ManifestRosteredZeroRowModelShownAs00(t *testing.T) {
	rows, _ := loadSeed116(t)
	const quietModel = "haiku-quiet-reviewer" // rostered, but filed no findings

	// A synthetic terminal manifest whose sole slot's model has zero
	// disposition rows in the seed — the exact "reviewed, found nothing"
	// shape. (Constructed in-memory; not written to the fixture, so the
	// pinned AC3 numbers over the on-disk seed are untouched.)
	syntheticManifests := []CoverageManifest{{
		Record: RecordPanelManifest,
		Spec:   "116",
		Gate:   "bead",
		Panel:  "panel-116-synthetic",
		Round:  1,
		Slots: []ManifestSlot{
			{Slot: "H1", Model: quietModel, Verdict: "APPROVE"},
		},
		Backfilled: true,
	}}

	extras := ManifestModels(syntheticManifests)
	if len(extras) != 1 || extras[0] != quietModel {
		t.Fatalf("ManifestModels = %v, want exactly [%s]", extras, quietModel)
	}

	for _, tc := range []struct {
		name    string
		compute func([]DispositionRow, ...string) []ModelRate
	}{
		{"Q1", ComputeQ1},
		{"Q2", ComputeQ2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := rateMap(tc.compute(rows, extras...))

			got, ok := m[quietModel]
			if !ok {
				t.Fatalf("%s dropped the manifest-rostered zero-row model %q — it must appear as 0/0", tc.name, quietModel)
			}
			if got.Numerator != 0 || got.Denominator != 0 {
				t.Fatalf("%s[%s] = %+v, want Numerator=0 Denominator=0", tc.name, quietModel, got)
			}
			if render := got.Render(); render != "0/0" {
				t.Errorf(`%s[%s].Render() = %q, want "0/0"`, tc.name, quietModel, render)
			}

			// The four real seed models MUST keep their exact pinned rates
			// — the union adds the quiet model without perturbing them.
			realQ1 := map[string]string{"fable": "3/5", "opus": "4/6", "sonnet": "6/7", "gpt-5.6-sol": "2/3"}
			realQ2 := map[string]string{"fable": "0/5", "opus": "1/6", "sonnet": "1/7", "gpt-5.6-sol": "1/3"}
			want := realQ1
			if tc.name == "Q2" {
				want = realQ2
			}
			for model, exp := range want {
				if r, ok := m[model]; !ok || r.Render() != exp {
					t.Errorf("%s[%s] = %q (present=%v), want the pinned %q unchanged by the manifest union", tc.name, model, r.Render(), ok, exp)
				}
			}
		})
	}
}

// TestQueryMetrics_Q5Filters exercises the finding listing's three
// filter dimensions (gate/severity/disposition), independently and in
// combination, plus the unfiltered case.
func TestQueryMetrics_Q5Filters(t *testing.T) {
	rows, _ := loadSeed116(t)

	t.Run("unfiltered returns all rows", func(t *testing.T) {
		got := ComputeQ5(rows, Q5Filter{})
		if len(got) != 21 {
			t.Errorf("unfiltered Q5 = %d rows, want 21", len(got))
		}
	})

	t.Run("gate filter", func(t *testing.T) {
		got := ComputeQ5(rows, Q5Filter{Gate: "adhoc"})
		if len(got) != 2 {
			t.Fatalf("gate=adhoc Q5 = %d rows, want 2", len(got))
		}
		for _, r := range got {
			if r.Gate != "adhoc" {
				t.Errorf("gate filter leaked a non-adhoc row: %+v", r)
			}
		}
	})

	t.Run("severity filter", func(t *testing.T) {
		got := ComputeQ5(rows, Q5Filter{Severity: "blocking"})
		for _, r := range got {
			if r.Severity != "blocking" {
				t.Errorf("severity filter leaked a non-blocking row: %+v", r)
			}
		}
		if len(got) == 0 {
			t.Error("severity=blocking matched zero rows — the seed has at least one blocking finding")
		}
	})

	t.Run("disposition filter", func(t *testing.T) {
		got := ComputeQ5(rows, Q5Filter{Disposition: "false-contamination"})
		if len(got) != 3 {
			t.Fatalf("disposition=false-contamination Q5 = %d rows, want 3 (bead3a's O1/S1/G1-codex)", len(got))
		}
		for _, r := range got {
			if r.Disposition != "false-contamination" {
				t.Errorf("disposition filter leaked a non-matching row: %+v", r)
			}
		}
	})

	t.Run("combined gate+disposition filter narrows further", func(t *testing.T) {
		got := ComputeQ5(rows, Q5Filter{Gate: "bead", Disposition: "false-contamination"})
		if len(got) != 3 {
			t.Fatalf("gate=bead & disposition=false-contamination Q5 = %d rows, want 3", len(got))
		}
		got2 := ComputeQ5(rows, Q5Filter{Gate: "final_review", Disposition: "false-contamination"})
		if len(got2) != 0 {
			t.Fatalf("gate=final_review & disposition=false-contamination Q5 = %d rows, want 0 (no such combination exists in the seed)", len(got2))
		}
	})
}

// TestQueryMetrics_LoadStoreEmptyGlobIsNotAnError proves LoadStore over
// a glob matching zero files returns empty (not an error): a fresh spec
// with no captured dispositions yet is a legitimate state every Q1-Q5
// computation must handle by rendering explicit zeros, not by failing.
func TestQueryMetrics_LoadStoreEmptyGlobIsNotAnError(t *testing.T) {
	pattern := filepath.Join(t.TempDir(), "*", "dispositions.jsonl")
	rows, manifests, err := LoadStore(pattern)
	if err != nil {
		t.Fatalf("LoadStore over an empty glob returned an error: %v", err)
	}
	if len(rows) != 0 || len(manifests) != 0 {
		t.Fatalf("LoadStore over an empty glob returned %d rows / %d manifests, want 0/0", len(rows), len(manifests))
	}

	q1 := ComputeQ1(rows)
	if len(q1) != 0 {
		t.Errorf("Q1 over zero rows = %+v, want empty (no models to report)", q1)
	}
	q4 := ComputeQ4(rows, manifests)
	if len(q4) != len(CanonicalGateKeys) {
		t.Fatalf("Q4 over zero rows/manifests returned %d gates, want %d (every canonical gate still listed)", len(q4), len(CanonicalGateKeys))
	}
	for _, r := range q4 {
		if r.Genuine != 0 || r.SlotTotal != 0 {
			t.Errorf("Q4 gate %s = %+v over an empty store, want 0/0", r.Gate, r)
		}
		if render := (ModelRate{Numerator: r.Genuine, Denominator: r.SlotTotal}).Render(); render != "0/0" {
			t.Errorf("Q4 gate %s render = %q, want 0/0", r.Gate, render)
		}
	}
}
