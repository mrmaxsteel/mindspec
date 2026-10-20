package panel

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFinding_ItemsAndHashes(t *testing.T) {
	text := "fix /tmp/build/output and /Users/max/private before landing"
	v := FindingVerdict{Verdict: VerdictRequestChanges, ConcreteChangesRequired: []string{text, "second"}}
	findings, err := Findings(v)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 2 || findings[0].ID != "ccr[0]" || findings[1].ID != "ccr[1]" {
		t.Fatalf("wrong findings: %+v", findings)
	}
	if findings[0].Text != "fix <path> and <path> before landing" {
		t.Fatalf("unredacted storage: %q", findings[0].Text)
	}
	sum := sha256.Sum256([]byte(text))
	if findings[0].SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatal("hash does not pin raw finding text")
	}
	if err := HygienePredicate(mustMarshal(t, findings[0])); err != nil {
		t.Fatal(err)
	}
	changed := v
	changed.ConcreteChangesRequired = []string{strings.ReplaceAll(text, "max", "other"), "second"}
	f, err := Findings(changed)
	if err != nil {
		t.Fatal(err)
	}
	if f[0].Text != findings[0].Text || f[0].SHA256 == findings[0].SHA256 {
		t.Fatal("redaction hid a raw text change from hash")
	}
}

func TestFinding_EmptyListRationale(t *testing.T) {
	v := FindingVerdict{Verdict: VerdictRequestChanges, Rationale: "blocking rationale"}
	f, err := Findings(v)
	if err != nil || len(f) != 1 || f[0].ID != "ccr[0]" || f[0].Text != v.Rationale {
		t.Fatalf("rationale fallback: %+v %v", f, err)
	}
	for _, rationale := range []string{"", " \n\t"} {
		v.Rationale = rationale
		if _, err := Findings(v); err == nil || !strings.Contains(err.Error(), "INVALID") {
			t.Fatalf("empty rationale accepted: %v", err)
		}
	}
	for _, verdict := range []string{VerdictApprove, VerdictReject} {
		f, err := Findings(FindingVerdict{Verdict: verdict})
		if err != nil || len(f) != 0 {
			t.Fatalf("non-RC findings: %v %v", f, err)
		}
	}
	if _, err := Findings(FindingVerdict{Verdict: "BOGUS"}); err == nil {
		t.Fatal("unknown verdict accepted")
	}
}

func TestFinding_SlotHashPinsOrderedList(t *testing.T) {
	v := FindingVerdict{Verdict: VerdictRequestChanges, ConcreteChangesRequired: []string{"one", "two"}}
	f, err := SlotFinding(v)
	if err != nil {
		t.Fatal(err)
	}
	expected := sha256.Sum256([]byte(`["one","two"]`))
	if f.ID != "slot" || f.SHA256 != hex.EncodeToString(expected[:]) {
		t.Fatalf("slot hash: %+v", f)
	}
	v.ConcreteChangesRequired = []string{"two", "one"}
	swapped, err := SlotFinding(v)
	if err != nil {
		t.Fatal(err)
	}
	if swapped.SHA256 == f.SHA256 {
		t.Fatal("list reorder retained slot discharge")
	}
	v.ConcreteChangesRequired = nil
	v.Rationale = "fallback"
	f, err = SlotFinding(v)
	if err != nil {
		t.Fatal(err)
	}
	expected = sha256.Sum256([]byte("fallback"))
	if f.SHA256 != hex.EncodeToString(expected[:]) {
		t.Fatal("rationale slot hash differs from raw text")
	}
}

func TestFinding_LatestRoundAndEvidenceArray(t *testing.T) {
	dir := t.TempDir()
	for name, data := range map[string]string{
		"R1-round-1.json": `{"verdict":"REQUEST_CHANGES","rationale":"old"}`,
		"R1-round-2.json": `{"verdict":"REQUEST_CHANGES","concrete_changes_required":["current"],"findings":["evidence only"]}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ReadFindingVerdict(dir, "R1", 1); err == nil {
		t.Fatal("stale round accepted")
	}
	if _, err := ReadFindingVerdict(dir, "R2", 2); err == nil {
		t.Fatal("missing slot accepted")
	}
	if _, err := ReadFindingVerdict(dir, "../R1", 2); err == nil {
		t.Fatal("path slot accepted")
	}
	v, err := ReadFindingVerdict(dir, "R1", 2)
	if err != nil {
		t.Fatal(err)
	}
	f, err := Findings(v)
	if err != nil || len(f) != 1 || f[0].Text != "current" {
		t.Fatalf("wrong current finding: %+v %v", f, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "R1-round-2.json"), []byte(`{"verdict":"REQUEST_CHANGES","findings":["evidence only"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFindingVerdict(dir, "R1", 2); err == nil {
		t.Fatal("evidence array supplied a Finding")
	}
}

func TestFinding_RedactPaths(t *testing.T) {
	for text, want := range map[string]string{
		"/tmp/":                         "<path>",
		"`/Users/me/file` and '/tmp/a'": "`<path>` and '<path>'",
		"repo/file and https://example.test/file": "repo/file and https://example.test/file",
	} {
		if got := RedactFindingPaths(text); got != want {
			t.Errorf("%q: %q want %q", text, got, want)
		}
	}
}
