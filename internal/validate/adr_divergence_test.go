package validate

import "testing"

// TestCheckADRDivergenceReturnsEmpty confirms the spec-086 Bead 2 stub
// returns an empty *Result with the expected sub-command. The real body
// lands in spec-087 F1. Plan stub specifies the Result has only
// SubCommand set — TargetID is intentionally absent per panel CONSENSUS
// Minor 7.
func TestCheckADRDivergenceReturnsEmpty(t *testing.T) {
	r := CheckADRDivergence("/tmp/root", "HEAD~1", nil)
	if r == nil {
		t.Fatal("CheckADRDivergence returned nil; expected empty *Result")
	}
	if r.SubCommand != "adr-divergence" {
		t.Errorf("SubCommand = %q, want %q", r.SubCommand, "adr-divergence")
	}
	if r.TargetID != "" {
		t.Errorf("TargetID = %q, want empty (plan stub specifies no TargetID)", r.TargetID)
	}
	if len(r.Issues) != 0 {
		t.Errorf("expected no issues, got %d: %+v", len(r.Issues), r.Issues)
	}
	if r.HasFailures() {
		t.Error("stub should not report failures")
	}
}
