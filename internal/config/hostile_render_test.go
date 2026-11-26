package config

import (
	"strings"
	"testing"
)

func TestLoad_GateAuthorityKeyEscapesControlBytes(t *testing.T) {
	cfg, warnings := s134Load(t, "loop:\n  gate_authority:\n    \"bad\\a\\e key\": nonsense\n")
	if cfg == nil || strings.ContainsAny(warnings, "\a\x1b") {
		t.Fatalf("unsafe warning: %q", warnings)
	}
	if len(cfg.RemovedLoopKeys()) != 1 || !strings.Contains(warnings, "ignored") {
		t.Fatalf("removed map must be ignored: %q", warnings)
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
