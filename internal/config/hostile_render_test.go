package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestLoad_GateAuthorityKeyEscapesControlBytes extends the
// TestLoad_UnknownGateKeyEscapesControlBytes pattern (AC-20, spec 120
// R8): a hostile loop.gate_authority key must not reach the refusal
// text raw at EITHER interpolation point (the message clause's %q, and
// the recovery clause, which previously repeated the key via a bare
// %s).
func TestLoad_GateAuthorityKeyEscapesControlBytes(t *testing.T) {
	ResetCache()
	defer ResetCache()

	// YAML double-quoted scalar escapes: \a = BEL (0x07), \e = ESC
	// (0x1b) — decoded into the actual map key, not raw in this Go
	// source.
	content := "loop:\n  gate_authority:\n    \"bad\\a\\e key\": \"nonsense\"\n"

	root := t.TempDir()
	dir := filepath.Join(root, ".mindspec")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(root)
	if err == nil {
		t.Fatal("expected Load to refuse the malformed gate_authority value, got nil error")
	}
	msg := err.Error()

	if strings.ContainsRune(msg, '\x07') {
		t.Errorf("error text contains a raw BEL byte: %q", msg)
	}
	if strings.ContainsRune(msg, '\x1b') {
		t.Errorf("error text contains a raw ESC byte: %q", msg)
	}
	// Both interpolation points (message clause + recovery clause) must
	// carry the QUOTED form of the key, not a raw repeat.
	if got := strings.Count(msg, strconv.Quote("bad\a\x1b key")); got != 2 {
		t.Errorf("expected the quoted key at both interpolation points (message + recovery), got %d occurrences in: %q", got, msg)
	}

	// Clean-fixture consistency: a normal key is quoted the SAME way at
	// both interpolation points (the panel.gates precedent quotes
	// unconditionally via %q/strconv.Quote — this is fmt's own quoting
	// convention for naming an offending config key, not the
	// identity-preserving termsafe.Escape/idrender discipline used for
	// free-text/ID render positions elsewhere in R4). What must hold
	// here is consistency between the two interpolation points, and
	// that the clean key's quoted form carries no extra control bytes.
	ResetCache()
	content2 := "loop:\n  gate_authority:\n    bead_merge: \"nonsense\"\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content2), 0644); err != nil {
		t.Fatal(err)
	}
	_, err2 := Load(root)
	if err2 == nil {
		t.Fatal("expected Load to refuse the malformed gate_authority value, got nil error")
	}
	msg2 := err2.Error()
	if got := strings.Count(msg2, strconv.Quote("bead_merge")); got != 2 {
		t.Errorf("expected the quoted clean key at both interpolation points, got %d occurrences in: %q", got, msg2)
	}
}

// TestExpandSlots_DuplicateReviewersStayDistinct kills the dedup mutant
// (AC-20, spec 120 R8): two identically-shaped reviewer entries (same
// model, same count) must each still expand to their OWN, DISTINCT slot
// (R1, R2) — never collapsed to a single slot by an accidental
// map/set-keyed dedup of "identical" entries. Declaration order and
// count are what matter, not entry uniqueness.
func TestExpandSlots_DuplicateReviewersStayDistinct(t *testing.T) {
	one := 1
	reviewers := []Reviewer{
		{Model: "claude-opus-4-8", Count: &one},
		{Model: "claude-opus-4-8", Count: &one},
	}
	slots := expandSlots(reviewers)
	if len(slots) != 2 {
		t.Fatalf("expected 2 distinct slots for 2 identical reviewer entries, got %d: %+v", len(slots), slots)
	}
	if slots[0].Slot != "R1" || slots[1].Slot != "R2" {
		t.Errorf("expected slots R1, R2 in order, got %q, %q", slots[0].Slot, slots[1].Slot)
	}
	if slots[0].Model != "claude-opus-4-8" || slots[1].Model != "claude-opus-4-8" {
		t.Errorf("both slots must carry the shared model, got %+v", slots)
	}
}
