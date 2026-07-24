package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/harness"
)

// bead_clarify_test.go pins spec 124 (impl-readiness-gate) Bead 3's
// `mindspec bead clarify` verb: AC-11 (durability across a fresh bd
// show), AC-15 (restart-proof categorical per-bead cap, no --finalize
// surface), and the verb-level rejections (unknown ordinal, missing
// span, malformed bead id).
//
// The durability/restart-proof tests drive the REAL mindspec binary
// against a real bd + git sandbox (internal/harness.NewSandbox) — each
// `sb.Run("mindspec", "bead", "clarify", ...)` call is a genuinely
// separate OS process with no transcript memory, and a subsequent
// `bd show` in another fresh process is the AC-11/AC-15 durability
// proof. Per the repo's no-skip-gating convention for real-bd/real-git
// flows, a missing bd/git on PATH is a hard failure, never t.Skip.

// clarifySetupSandbox builds the mindspec binary, prepends it to PATH
// (restored via t.Cleanup), and returns a fresh sandbox — bd/git
// required on PATH (hard failure, never skip, if absent).
func clarifySetupSandbox(t *testing.T) *harness.Sandbox {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping real-bd/real-git end-to-end flow under -short")
	}
	if _, err := exec.LookPath("bd"); err != nil {
		t.Fatalf("real bd required for this AC-11/AC-15 test (no-skip-gating, spec 124 plan-gate F3-1): %v", err)
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("real git required: %v", err)
	}

	binPath := buildMindspecBinary(t)
	binDir := filepath.Dir(binPath)
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", binDir+string(os.PathListSeparator)+origPath)
	t.Cleanup(func() { os.Setenv("PATH", origPath) })

	return harness.NewSandbox(t)
}

func clarifyBDMetadata(t *testing.T, sb *harness.Sandbox, beadID string) map[string]interface{} {
	t.Helper()
	out, err := sb.Run("bd", "show", beadID, "--json")
	if err != nil {
		t.Fatalf("bd show %s: %v\n%s", beadID, err, out)
	}
	var records []struct {
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := json.Unmarshal([]byte(out), &records); err != nil {
		t.Fatalf("parsing bd show %s --json: %v\nraw: %s", beadID, err, out)
	}
	if len(records) == 0 {
		t.Fatalf("bd show %s --json returned no records", beadID)
	}
	return records[0].Metadata
}

const clarifyRecordJSON = `{
  "report": [
    {"ordinal": 1, "signal": "SR-2", "reason": "AC-7 has no decidable check"}
  ],
  "clarifications": [
    {"ordinal": 1, "reason": "AC-7 has no decidable check", "answer": "AC-7 is verified by go test ./foo -run TestBar", "span": "plan.md §Bead 2: \"AC-7 — go test ./foo -run TestBar passes\""}
  ]
}`

// TestBeadClarify_WritesRecord_SurvivesFreshBDShow pins AC-11's
// durability half: a successful clarify write is visible, complete
// (report + clarifications), from a FRESH `bd show` process.
func TestBeadClarify_WritesRecord_SurvivesFreshBDShow(t *testing.T) {
	sb := clarifySetupSandbox(t)
	epicID := sb.CreateSpecEpic("124-clarify-durability")
	beadID := sb.CreateBead("Bead 1", "task", epicID)

	sb.WriteFile("record.json", clarifyRecordJSON)

	out, err := sb.Run("mindspec", "bead", "clarify", beadID, "--file", "record.json")
	if err != nil {
		t.Fatalf("mindspec bead clarify %s: %v\n%s", beadID, err, out)
	}

	meta := clarifyBDMetadata(t, sb, beadID)
	raw, ok := meta["mindspec_readiness_attempt"]
	if !ok {
		t.Fatalf("expected mindspec_readiness_attempt metadata key on %s; got %v", beadID, meta)
	}
	rawJSON, _ := json.Marshal(raw)
	if !strings.Contains(string(rawJSON), "AC-7 has no decidable check") {
		t.Errorf("expected the report reason durable in a fresh bd show; got: %s", rawJSON)
	}
	if !strings.Contains(string(rawJSON), "plan.md") {
		t.Errorf("expected the clarification span durable in a fresh bd show; got: %s", rawJSON)
	}
}

// TestBeadClarify_SecondAttempt_RefusedFreshProcess pins AC-15: a
// freshly-spawned SECOND `mindspec bead clarify` process (no transcript
// memory) is refused once an attempt record already exists — including
// a record that renames/renumbers the reason — and the cycle is forced
// to escalate (no re-dispatch path exists from this refusal).
func TestBeadClarify_SecondAttempt_RefusedFreshProcess(t *testing.T) {
	sb := clarifySetupSandbox(t)
	epicID := sb.CreateSpecEpic("124-clarify-cap")
	beadID := sb.CreateBead("Bead 1", "task", epicID)

	sb.WriteFile("record.json", clarifyRecordJSON)
	if out, err := sb.Run("mindspec", "bead", "clarify", beadID, "--file", "record.json"); err != nil {
		t.Fatalf("first clarify: %v\n%s", err, out)
	}

	// A renamed/renumbered second attempt — the cap is categorical, not
	// keyed to "the same reason".
	secondRecord := `{
  "report": [
    {"ordinal": 3, "signal": "SR-4", "reason": "an entirely different, never-before-seen reason"}
  ],
  "clarifications": [
    {"ordinal": 3, "reason": "an entirely different, never-before-seen reason", "answer": "resolved", "span": "spec.md §R1"}
  ]
}`
	sb.WriteFile("record2.json", secondRecord)

	out, err := sb.Run("mindspec", "bead", "clarify", beadID, "--file", "record2.json")
	if err == nil {
		t.Fatalf("expected the SECOND clarify (fresh process) to be refused, got exit 0:\n%s", out)
	}
	if !strings.Contains(out, "already carries a readiness-attempt record") {
		t.Errorf("expected the cap-refusal message, got:\n%s", out)
	}

	// The durable trail is exactly the FIRST record — the second write
	// never landed.
	meta := clarifyBDMetadata(t, sb, beadID)
	rawJSON, _ := json.Marshal(meta["mindspec_readiness_attempt"])
	if !strings.Contains(string(rawJSON), "AC-7 has no decidable check") {
		t.Errorf("expected the FIRST record's reason to remain the durable trail, got: %s", rawJSON)
	}
	if strings.Contains(string(rawJSON), "never-before-seen") {
		t.Errorf("the second, refused record must NOT have landed, got: %s", rawJSON)
	}
}

// TestBeadClarify_UnknownOrdinalRejected pins the source-span-grounding
// verb-level rejection: a clarification citing an ordinal absent from
// the record's own report is refused, with NO metadata write.
func TestBeadClarify_UnknownOrdinalRejected(t *testing.T) {
	sb := clarifySetupSandbox(t)
	epicID := sb.CreateSpecEpic("124-clarify-unknown-ordinal")
	beadID := sb.CreateBead("Bead 1", "task", epicID)

	record := `{
  "report": [{"ordinal": 1, "signal": "SR-2", "reason": "x"}],
  "clarifications": [{"ordinal": 99, "reason": "x", "answer": "y", "span": "spec.md §R1"}]
}`
	sb.WriteFile("record.json", record)

	out, err := sb.Run("mindspec", "bead", "clarify", beadID, "--file", "record.json")
	if err == nil {
		t.Fatalf("expected a refusal for an unknown ordinal, got exit 0:\n%s", out)
	}

	meta := clarifyBDMetadata(t, sb, beadID)
	if _, ok := meta["mindspec_readiness_attempt"]; ok {
		t.Errorf("expected NO metadata write on a refused unknown-ordinal record; got %v", meta)
	}
}

// TestBeadClarify_MissingSpanRejected pins the presence-only span check.
func TestBeadClarify_MissingSpanRejected(t *testing.T) {
	sb := clarifySetupSandbox(t)
	epicID := sb.CreateSpecEpic("124-clarify-missing-span")
	beadID := sb.CreateBead("Bead 1", "task", epicID)

	record := `{
  "report": [{"ordinal": 1, "signal": "SR-2", "reason": "x"}],
  "clarifications": [{"ordinal": 1, "reason": "x", "answer": "y", "span": ""}]
}`
	sb.WriteFile("record.json", record)

	out, err := sb.Run("mindspec", "bead", "clarify", beadID, "--file", "record.json")
	if err == nil {
		t.Fatalf("expected a refusal for a missing span, got exit 0:\n%s", out)
	}

	meta := clarifyBDMetadata(t, sb, beadID)
	if _, ok := meta["mindspec_readiness_attempt"]; ok {
		t.Errorf("expected NO metadata write on a refused missing-span record; got %v", meta)
	}
}

// TestBeadClarify_MalformedBeadID pins the ingress refusal — no fixture
// or sandbox needed since this refuses before any bd/file I/O.
func TestBeadClarify_MalformedBeadID(t *testing.T) {
	for _, id := range []string{"mindspec-1\n--help", "mindspec-1;evil", ""} {
		_, err := captureStdout(t, func() error {
			return beadClarifyCmd.RunE(beadClarifyCmd, []string{id})
		})
		if err == nil {
			t.Errorf("clarify(%q): expected a refusal, got nil", id)
		}
	}
}

// TestBeadClarify_MissingFileFlag pins that --file is required.
func TestBeadClarify_MissingFileFlag(t *testing.T) {
	if err := beadClarifyCmd.Flags().Set("file", ""); err != nil {
		t.Fatalf("resetting --file: %v", err)
	}
	_, err := captureStdout(t, func() error {
		return beadClarifyCmd.RunE(beadClarifyCmd, []string{"mindspec-abcd.1"})
	})
	if err == nil {
		t.Fatal("expected a refusal when --file is not supplied")
	}
	if !strings.Contains(err.Error(), "--file") {
		t.Errorf("expected the refusal to name --file, got: %v", err)
	}
}

// TestBeadClarify_OversizeFileRejected pins FX-2 (codex-G4): a --file
// larger than the read cap is refused cleanly (no OOM), before any bd
// call. No fixture/sandbox needed — the refusal is pre-bd.
func TestBeadClarify_OversizeFileRejected(t *testing.T) {
	tmp := t.TempDir()
	recordPath := filepath.Join(tmp, "big.json")
	// A syntactically-plausible-but-oversize JSON payload: a valid opening
	// plus > maxClarifyRecordBytes of filler.
	big := `{"report": [{"ordinal": 1, "signal": "SR-2", "reason": "` +
		strings.Repeat("x", maxClarifyRecordBytes+16) + `"}]}`
	if err := os.WriteFile(recordPath, []byte(big), 0o644); err != nil {
		t.Fatalf("writing oversize record: %v", err)
	}
	if err := beadClarifyCmd.Flags().Set("file", recordPath); err != nil {
		t.Fatalf("setting --file: %v", err)
	}
	t.Cleanup(func() { _ = beadClarifyCmd.Flags().Set("file", "") })

	_, err := captureStdout(t, func() error {
		return beadClarifyCmd.RunE(beadClarifyCmd, []string{"mindspec-abcd.3"})
	})
	if err == nil {
		t.Fatal("expected an oversize --file to be refused")
	}
	if !strings.Contains(err.Error(), "cap") {
		t.Errorf("expected the refusal to name the size cap, got: %v", err)
	}
}

// TestBeadClarify_UnknownFieldRejected pins FX-2 (codex-G4): a record
// carrying an unknown/typo'd field is rejected deterministically
// (DisallowUnknownFields), not silently mis-parsed. Pre-bd refusal.
func TestBeadClarify_UnknownFieldRejected(t *testing.T) {
	tmp := t.TempDir()
	recordPath := filepath.Join(tmp, "typo.json")
	// "clarifcations" (misspelled) is an unknown field — a silent
	// drop would let a record with zero real clarifications through.
	typo := `{"report": [{"ordinal": 1, "signal": "SR-2", "reason": "x"}], "clarifcations": []}`
	if err := os.WriteFile(recordPath, []byte(typo), 0o644); err != nil {
		t.Fatalf("writing typo record: %v", err)
	}
	if err := beadClarifyCmd.Flags().Set("file", recordPath); err != nil {
		t.Fatalf("setting --file: %v", err)
	}
	t.Cleanup(func() { _ = beadClarifyCmd.Flags().Set("file", "") })

	_, err := captureStdout(t, func() error {
		return beadClarifyCmd.RunE(beadClarifyCmd, []string{"mindspec-abcd.3"})
	})
	if err == nil {
		t.Fatal("expected an unknown-field record to be rejected")
	}
	if !strings.Contains(err.Error(), "not valid JSON") {
		t.Errorf("expected the strict-parse refusal, got: %v", err)
	}
}

// TestBeadClarify_NoFinalizeFlag pins R8e (derive-don't-write): the verb
// carries no --finalize / update surface.
func TestBeadClarify_NoFinalizeFlag(t *testing.T) {
	if beadClarifyCmd.Flags().Lookup("finalize") != nil {
		t.Error("bead clarify must not carry a --finalize flag (R8e derive-don't-write)")
	}
	if beadClarifyCmd.Flags().Lookup("update") != nil {
		t.Error("bead clarify must not carry an --update flag (R8e derive-don't-write)")
	}
}
