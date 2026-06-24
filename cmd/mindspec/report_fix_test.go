package main

// report_fix_test.go — spec 094 Bead 3 (6-panel fix): CLI/render regression
// tests for the panel's demonstrated repros + completeness ACs:
//
//   - oversized fingerprint scrub-full-before-truncate (codex-render-leak #1);
//   - C1-control injection through `report list` (codex-render-leak #2);
//   - shell-metachar --version slot rejected + never rendered (R1);
//   - full fingerprint shown == the one --resolve accepts (codex-completeness #3);
//   - render path-scrub to <path>, output code-fenced, fail-closed on Scrub
//     ok=false (codex-completeness #5 / Req 7);
//   - egress proof over the beads/dolt surfaces + bd query output;
//   - no-push / no-network structural assertion;
//   - ADR placeholder-only + bootstrap-doc inspection.

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/journal"
)

// writeRawReports writes a hand-crafted reports.jsonl into the canonical store
// dir (the untrusted-corpus attacker surface) and returns nothing — the next
// `report list` reads it.
func writeRawReports(t *testing.T, dir, jsonl string) {
	t.Helper()
	rp := filepath.Join(dirCanon(t, dir), "reports.jsonl")
	if err := os.MkdirAll(filepath.Dir(rp), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rp, []byte(jsonl), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestReportList_OversizedFingerprintFailsClosed is the codex-render-leak #1
// repro: a hand-crafted reports.jsonl with an oversized fingerprint starting
// `LEAKPREFIX123456` must render `<redacted>`, NEVER a raw 16-byte prefix.
//
// RED-before: renderFingerprint truncated to 16 bytes BEFORE Scrub, so the row
// began with the raw `LEAKPREFIX123456`.
func TestReportList_OversizedFingerprintFailsClosed(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(journal.StateDirEnv, dir)
	t.Setenv("GITHUB_ACTIONS", "")

	oversized := "LEAKPREFIX123456" + strings.Repeat("A", 17*1024)
	writeRawReports(t, dir, `{"v":1,"fingerprint":"`+oversized+`","command":"complete","escape_hatch":"override-adr","count":1,"first_version":"1.0.0","last_version":"1.0.0"}`+"\n")

	out, err := execReport(t, "list")
	if err != nil {
		t.Fatalf("report list: %v", err)
	}
	if strings.Contains(out, "LEAKPREFIX") {
		t.Errorf("LEAK: raw oversized fingerprint prefix reached output:\n%s", out[:min(len(out), 200)])
	}
	if !strings.Contains(out, "<redacted>") {
		t.Errorf("oversized fingerprint should render <redacted>, got:\n%s", out[:min(len(out), 200)])
	}
}

// TestReportList_C1ControlInjection is the codex-render-leak #2 repro: a
// `first_version` carrying CSI U+009B (`2J`) must NOT reach stdout as the
// raw bytes `c2 9b 32 4a` — all C1 controls are stripped.
//
// RED-before: stripControl removed only C0/DEL, so U+009B survived.
func TestReportList_C1ControlInjection(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(journal.StateDirEnv, dir)
	t.Setenv("GITHUB_ACTIONS", "")

	//  is the C1 CSI; 2J is a clear-screen terminal sequence.
	writeRawReports(t, dir, `{"v":1,"fingerprint":"abc123","command":"complete","escape_hatch":"override-adr","count":1,"first_version":"2J","last_version":"2J"}`+"\n")

	out, err := execReport(t, "list")
	if err != nil {
		t.Fatalf("report list: %v", err)
	}
	if strings.ContainsRune(out, '') {
		t.Errorf("LEAK: C1 control U+009B reached output bytes: % x", []byte(out))
	}
	// The raw UTF-8 encoding of U+009B is c2 9b — assert those bytes are absent.
	if strings.Contains(out, "\xc2\x9b") {
		t.Errorf("LEAK: raw C1 bytes c2 9b reached output")
	}
}

// TestReportList_FullFingerprintIsResolvable is codex-completeness #3: the
// identifier SHOWN in `report list` is the FULL fingerprint, and it is exactly
// the value `--resolve` accepts (no truncated prefix).
func TestReportList_FullFingerprintIsResolvable(t *testing.T) {
	seedJournal(t, frictionEvent())
	t.Setenv("GITHUB_ACTIONS", "")

	reports, _ := journal.Consolidate()
	if err := journal.WriteReports(reports); err != nil {
		t.Fatal(err)
	}
	fullFP := reports[0].Fingerprint
	if len(fullFP) != 64 {
		t.Fatalf("expected a 64-hex fingerprint, got %d chars", len(fullFP))
	}

	out, err := execReport(t, "list")
	if err != nil {
		t.Fatalf("report list: %v", err)
	}
	if !strings.Contains(out, fullFP) {
		t.Errorf("report list must show the FULL fingerprint %s; got:\n%s", fullFP, out)
	}
	// The shown identifier resolves (no truncated prefix needed).
	if _, err := execReport(t, "list", "--resolve", fullFP, "--version", "1.0.0"); err != nil {
		t.Fatalf("the shown full fingerprint must be accepted by --resolve: %v", err)
	}
	out2, _ := execReport(t, "list")
	if !strings.Contains(out2, "regression") {
		t.Errorf("resolve via the shown fingerprint did not take effect:\n%s", out2)
	}
}

// TestReportList_ShellMetacharVersionRejectedAndNeverRendered is the R1
// slot-escaping AC: a shell-metachar --version value is REJECTED and the live
// string NEVER appears in any rendered field (resolve-echo or RESOLVED-IN).
func TestReportList_ShellMetacharVersionRejectedAndNeverRendered(t *testing.T) {
	seedJournal(t, frictionEvent())
	t.Setenv("GITHUB_ACTIONS", "")

	reports, _ := journal.Consolidate()
	if err := journal.WriteReports(reports); err != nil {
		t.Fatal(err)
	}
	fp := reports[0].Fingerprint

	const payload = "1.0.0; rm -rf /"
	out, err := execReport(t, "list", "--resolve", fp, "--version", payload)
	if err == nil {
		t.Errorf("a shell-metachar --version must be rejected; got no error.\n%s", out)
	}
	if strings.Contains(out, "rm -rf") || strings.Contains(out, "; rm") {
		t.Errorf("resolve-echo rendered the live shell string:\n%s", out)
	}

	// And it never persisted, so the RESOLVED-IN column can't carry it.
	listOut, _ := execReport(t, "list")
	if strings.Contains(listOut, "rm -rf") {
		t.Errorf("RESOLVED-IN column rendered the live shell string:\n%s", listOut)
	}
}

// TestReport_RendersStorePathScrubbed is Req 7: `mindspec report` renders the
// store path through the scrub so an absolute path is shown as <path>.
func TestReport_RendersStorePathScrubbed(t *testing.T) {
	seedJournal(t, frictionEvent())
	t.Setenv("GITHUB_ACTIONS", "")

	out, err := execReport(t)
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if !strings.Contains(out, "<path>") {
		t.Errorf("report should scrub the store path to <path>:\n%s", out)
	}
	// And the raw absolute store path must not appear.
	rp, _ := journal.ReportsPath()
	if strings.Contains(out, rp) {
		t.Errorf("report leaked the raw store path %q:\n%s", rp, out)
	}
}

// TestReportList_OutputIsCodeFenced is Req 7 / HC-4: the `report list` triage
// body is code-fenced so no consumer auto-links / auto-executes a rendered line.
func TestReportList_OutputIsCodeFenced(t *testing.T) {
	seedJournal(t, frictionEvent())
	t.Setenv("GITHUB_ACTIONS", "")
	reports, _ := journal.Consolidate()
	if err := journal.WriteReports(reports); err != nil {
		t.Fatal(err)
	}
	out, err := execReport(t, "list")
	if err != nil {
		t.Fatalf("report list: %v", err)
	}
	if strings.Count(out, "```") < 2 {
		t.Errorf("report list body must be code-fenced (two ``` lines):\n%s", out)
	}
}

// TestRenderField_FailsClosedOnUnscrubbable is Req 7 / HC-7: a value Scrub
// cannot classify (oversized) renders <redacted>, never raw.
func TestRenderField_FailsClosedOnUnscrubbable(t *testing.T) {
	// An oversized value trips Scrub's maxScrubInput → ok=false → fail closed.
	huge := strings.Repeat("Z", 1<<20)
	got := renderField(huge)
	if got != "<redacted>" {
		t.Errorf("unscrubbable value must render <redacted>, got %q (len %d)", got[:min(len(got), 40)], len(got))
	}
}

// TestReportList_EgressProof_BeadsAndDoltSurfaces is the codex-completeness #5
// egress proof: a report fingerprint is ABSENT from .beads/issues.jsonl, the
// dolt working-set/tracked tables, AND `bd` query output — not merely "the path
// is outside .beads". We assert over the real surfaces a `bd dolt push` sends.
func TestReportList_EgressProof_BeadsAndDoltSurfaces(t *testing.T) {
	dir := seedJournal(t, frictionEvent())
	t.Setenv("GITHUB_ACTIONS", "")
	reports, _ := journal.Consolidate()
	if err := journal.WriteReports(reports); err != nil {
		t.Fatal(err)
	}
	fp := reports[0].Fingerprint

	// 1) the store path itself is outside any .beads/dolt tree.
	rp := filepath.Join(dirCanon(t, dir), "reports.jsonl")
	if strings.Contains(rp, ".beads") || strings.Contains(rp, string(filepath.Separator)+"dolt") {
		t.Fatalf("store path under a bd/dolt tree (HC-3): %q", rp)
	}

	// 2) the fingerprint is absent from .beads/issues.jsonl (the committed
	//    beads JSONL the push ships). We scan the repo's .beads if present.
	repoBeads := repoRootFromTestDir(t)
	if repoBeads != "" {
		issues := filepath.Join(repoBeads, ".beads", "issues.jsonl")
		if data, err := os.ReadFile(issues); err == nil {
			if strings.Contains(string(data), fp) {
				t.Errorf("EGRESS: fingerprint %s present in %s", fp, issues)
			}
		}
		// 3) scan every tracked dolt/.beads artifact for the fingerprint.
		beadsDir := filepath.Join(repoBeads, ".beads")
		_ = filepath.Walk(beadsDir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if data, rerr := os.ReadFile(p); rerr == nil && strings.Contains(string(data), fp) {
				t.Errorf("EGRESS: fingerprint %s present in tracked beads artifact %s", fp, p)
			}
			return nil
		})
	}

	// 4) `bd` query output (if bd is installed) must not surface the
	//    fingerprint — the friction store is never a bd record.
	if out, ok := bdQuery(t); ok && strings.Contains(out, fp) {
		t.Errorf("EGRESS: fingerprint %s surfaced in `bd` query output", fp)
	}
}

// TestReport_NoPushNoNetwork is the no-push/no-network CI assertion
// (codex-completeness #5): report.go must NOT import any network/push package,
// so `report` can make no push or network call. A source-level import scan
// FAILS if a network/bd/dolt/git/http import is introduced.
func TestReport_NoPushNoNetwork(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "report.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse report.go: %v", err)
	}
	banned := []string{"net/http", "net", "os/exec", "dolt", "/bd", "go-git", "git2go"}
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		for _, b := range banned {
			if strings.Contains(path, b) {
				t.Errorf("report.go imports a push/network path %q — no-push contract regressed", path)
			}
		}
	}
}

// TestADR0038_PlaceholderOnly inspects ADR-0038 for placeholder-only examples:
// no live absolute path, secret token, email, or URL example may appear (the
// ADR is committed outside the redaction sink). It also confirms the status
// model language is reconciled to {open, regression, stale}.
func TestADR0038_PlaceholderOnly(t *testing.T) {
	root := repoRootFromTestDir(t)
	if root == "" {
		t.Skip("repo root not found")
	}
	path := filepath.Join(root, ".mindspec", "docs", "adr", "ADR-0038-friction-reporter.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("ADR-0038 not found: %v", err)
	}
	body := string(data)
	for _, leak := range []string{"ghp_", "sk-", "AKIA", "/Users/", "token=", "BEGIN RSA"} {
		if strings.Contains(body, leak) {
			t.Errorf("ADR-0038 contains a non-placeholder leak marker %q", leak)
		}
	}
	// The dead "resolved" status token must be gone from the status enumeration.
	if strings.Contains(body, "open/resolved/regression/stale") {
		t.Errorf("ADR-0038 still advertises the dead `resolved` status token")
	}
	if !strings.Contains(body, "{open, regression, stale}") {
		t.Errorf("ADR-0038 should state the reconciled status model {open, regression, stale}")
	}
}

// TestBootstrapParadoxDoc_Exists is codex-completeness #4: the named standalone
// bootstrap-paradox doc artifact exists, is placeholder-only, and cross-links
// the ADR.
func TestBootstrapParadoxDoc_Exists(t *testing.T) {
	root := repoRootFromTestDir(t)
	if root == "" {
		t.Skip("repo root not found")
	}
	// Spec 106: dogfood/user docs were evicted out of .mindspec/docs/user/
	// to top-level project-docs/ by the flatten.
	path := filepath.Join(root, "project-docs", "user", "guides", "friction-bootstrap-paradox.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("bootstrap-paradox doc artifact missing: %v", err)
	}
	body := string(data)
	if !strings.Contains(strings.ToLower(body), "bootstrap") || !strings.Contains(strings.ToLower(body), "install") {
		t.Errorf("bootstrap doc must describe the install-failure bootstrap paradox")
	}
	if !strings.Contains(body, "ADR-0038") {
		t.Errorf("bootstrap doc must cross-link ADR-0038")
	}
	for _, leak := range []string{"ghp_", "sk-", "AKIA", "/Users/", "token="} {
		if strings.Contains(body, leak) {
			t.Errorf("bootstrap doc contains a non-placeholder leak marker %q", leak)
		}
	}
}

// bdQuery runs `bd list` (a read-only query) and returns its combined output.
// If bd is not installed or errors, ok=false (the egress assertion is skipped
// rather than failing on a missing optional tool).
func bdQuery(t *testing.T) (string, bool) {
	t.Helper()
	path, err := exec.LookPath("bd")
	if err != nil {
		return "", false
	}
	out, err := exec.Command(path, "list").CombinedOutput()
	if err != nil {
		return "", false
	}
	return string(out), true
}
