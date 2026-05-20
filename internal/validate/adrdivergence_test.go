package validate

import "testing"

// TestCheckADRDivergence_Stub confirms the spec-086 Bead 2 stub returns an
// empty *Result with the expected sub-command and target tagging. The real
// body lands in spec-087 F1.
func TestCheckADRDivergence_Stub(t *testing.T) {
	r := CheckADRDivergence("/tmp/root", "HEAD~1", nil)
	if r == nil {
		t.Fatal("CheckADRDivergence returned nil; expected empty *Result")
	}
	if r.SubCommand != "adr-divergence" {
		t.Errorf("SubCommand = %q, want %q", r.SubCommand, "adr-divergence")
	}
	if r.TargetID != "HEAD~1" {
		t.Errorf("TargetID = %q, want %q", r.TargetID, "HEAD~1")
	}
	if len(r.Issues) != 0 {
		t.Errorf("expected no issues, got %d: %+v", len(r.Issues), r.Issues)
	}
	if r.HasFailures() {
		t.Error("stub should not report failures")
	}
}
