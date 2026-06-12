package harness

import "testing"

// TestLLM_PanelGateBlocksPrematureComplete runs the Spec 093 panel-gate
// scenario (skips under -short per HC-1, like the other LLM scenarios).
func TestLLM_PanelGateBlocksPrematureComplete(t *testing.T) {
	runContractHardeningScenario(t, ScenarioPanelGateBlocksPrematureComplete())
}

// TestAllScenariosRegistersPanelGate pins the scenario into AllScenarios().
func TestAllScenariosRegistersPanelGate(t *testing.T) {
	found := false
	for _, s := range AllScenarios() {
		if s.Name == "panel_gate_blocks_premature_complete" {
			found = true
		}
	}
	if !found {
		t.Error("panel_gate_blocks_premature_complete is not registered in AllScenarios()")
	}
}
