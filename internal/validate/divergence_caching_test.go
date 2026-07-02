package validate

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
)

// diagCachingFixture builds the shared fixture-repo for the R7+R8
// caching proofs: a spec impacting three domains (billing, payments,
// search), a plan citing one Accepted and one Proposed ADR, and a diff
// spanning the four divergence outcomes the golden locks —
// owned+Accepted-covered (silent), owned+uncovered, owned+Proposed-only,
// and unowned.
func diagCachingFixture(t *testing.T) (root, specDir string, mock *executor.MockExecutor) {
	t.Helper()
	root = t.TempDir()
	specDir = filepath.Join(root, ".mindspec", "docs", "specs", "900-caching")
	writeSpecAndPlan(t, root, specDir, "900-caching",
		[]string{"payments", "search", "billing"},
		[]string{"ADR-0201", "ADR-0202"},
	)
	writeADR(t, root, "ADR-0201", "Accepted", []string{"payments"})
	writeADR(t, root, "ADR-0202", "Proposed", []string{"billing"})
	writeManifest(t, root, "payments", "paths:\n  - internal/payments/**\n")
	writeManifest(t, root, "search", "paths:\n  - internal/search/**\n")
	writeManifest(t, root, "billing", "paths:\n  - internal/billing/**\n")
	mock = &executor.MockExecutor{
		ChangedFilesResult: []string{
			"internal/payments/charge.go", // owned + Accepted-covered → silent
			"internal/search/query.go",    // owned + uncovered
			"internal/billing/invoice.go", // owned + Proposed-only
			"internal/unclaimed/x.go",     // unowned
		},
	}
	return root, specDir, mock
}

// diagLines renders a Result's Issues as ordered "severity name: message"
// lines, normalizing any absolute manifest path under root to <ROOT> so
// the golden is stable across temp dirs while still asserting the full
// message text.
func diagLines(issues []Issue, root string) []string {
	out := make([]string, 0, len(issues))
	for _, i := range issues {
		msg := strings.ReplaceAll(i.Message, root, "<ROOT>")
		out = append(out, string(rune('0'+int(i.Severity)))+" "+i.Name+": "+msg)
	}
	return out
}

// TestValidateDiagnosticsByteIdenticalAfterCaching is the shared
// identical-outcomes proof for R7 (per-domain OWNERSHIP hoisting) and R8
// (per-run ADR-parse memoization). It locks the FULL ordered
// (severity, code, message) diagnostics set emitted by ValidateDivergence
// and checkADRCoverage over a fixture repo spanning owned/unowned/
// uncovered/proposed cases. The golden was captured from the pre-caching
// code and MUST remain byte-identical after the hoist + memoization —
// caching changes the number of reads, never the diagnostics.
func TestValidateDiagnosticsByteIdenticalAfterCaching(t *testing.T) {
	root, specDir, mock := diagCachingFixture(t)

	r, _ := ValidateDivergence(mock, root, specDir, "mindspec-golden.1", "BASE", "HEAD", "", false)
	gotDivergence := diagLines(r.Issues, root)

	wantDivergence := []string{
		`0 adr-divergence-uncovered: file internal/search/query.go attributed to domain "search" (manifest: <ROOT>/.mindspec/docs/domains/search/OWNERSHIP.yaml) but no cited ADR covers "search"`,
		`1 adr-divergence-proposed: file internal/billing/invoice.go attributed to domain "billing" is covered only by Proposed ADR ADR-0202 — flip it to Accepted before impl approve`,
		`0 adr-divergence-unowned: file internal/unclaimed/x.go is not claimed by any OWNERSHIP.yaml for the spec's impacted domains [payments search billing]; add it to an existing manifest or create a new domain dir at .mindspec/docs/domains/<name>/OWNERSHIP.yaml`,
	}
	if !equalLines(gotDivergence, wantDivergence) {
		t.Errorf("ValidateDivergence diagnostics drifted after caching:\n got: %s\nwant: %s",
			strings.Join(gotDivergence, "\n"), strings.Join(wantDivergence, "\n"))
	}

	// checkADRCoverage over the same citations/domains.
	r2 := &Result{SubCommand: "plan"}
	store := adrStoreForSpec(root, specDir)
	checkADRCoverage(r2, store, []ADRCitation{{ID: "ADR-0201"}, {ID: "ADR-0202"}},
		[]string{"billing", "payments", "search"})
	gotCoverage := diagLines(r2.Issues, root)

	wantCoverage := []string{
		`1 adr-coverage-proposed: impacted domain "billing" is covered only by Proposed ADR ADR-0202 — flip it to Accepted after the implementation ships`,
		"0 adr-coverage-missing: impacted domain \"search\" has no cited Accepted ADR; either add \"search\" to the `Domain(s)` line of an existing cited Accepted ADR, or run: mindspec adr create --domain search",
	}
	if !equalLines(gotCoverage, wantCoverage) {
		t.Errorf("checkADRCoverage diagnostics drifted after caching:\n got: %s\nwant: %s",
			strings.Join(gotCoverage, "\n"), strings.Join(wantCoverage, "\n"))
	}
}

func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
