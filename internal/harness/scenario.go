// Package harness contains the LLM behavioral test harness.
//
// scenario.go declares the Scenario type and the AllScenarios() dispatcher.
// Individual scenarios live in:
//
//	scenario_spec_lifecycle.go  — spec/plan/impl approve flows
//	scenario_bead_lifecycle.go  — single/multi bead, crash resume, stop, blocked
//	scenario_safety.go          — interrupt-for-bug, bugfix-branch
//	scenario_worktree.go        — worktree topology scenarios + shared worktree helpers
//	asserts.go                  — assertion helpers and small support types
package harness

import "testing"

// Scenario defines a behavioral test scenario for an agent session.
type Scenario struct {
	Name        string                                                     // scenario identifier (e.g. "single_bead")
	Description string                                                     // human-readable description
	Setup       func(sandbox *Sandbox) error                               // prepares sandbox state before agent runs
	Prompt      string                                                     // the prompt given to the agent
	Assertions  func(t *testing.T, sandbox *Sandbox, events []ActionEvent) // post-run assertions
	MaxTurns    int                                                        // turn budget (0 = unlimited)
	TimeoutMin  int                                                        // scenario runtime timeout in minutes (0 = default 10m)
	Model       string                                                     // model override (e.g. "haiku")

	// StartDir is the agent's starting working directory, relative to the
	// sandbox root (Spec 092 Req 16). Empty means sandbox root. It is
	// resolved AFTER Setup runs and may contain a glob pattern (e.g.
	// ".worktrees/worktree-spec-001-x/.worktrees/worktree-*") because bead
	// worktree paths embed IDs that bd assigns dynamically during Setup.
	// The pattern must resolve to exactly one existing directory.
	StartDir string
}

// AllScenarios returns all defined behavior scenarios.
func AllScenarios() []Scenario {
	return []Scenario{
		ScenarioSpecToIdle(),
		ScenarioSingleBead(),
		ScenarioMultiBeadDeps(),
		ScenarioInterruptForBug(),
		ScenarioResumeAfterCrash(),
		ScenarioSpecInit(),
		ScenarioSpecApprove(),
		ScenarioPlanApprove(),
		ScenarioImplApprove(),
		ScenarioSpecStatus(),
		ScenarioMultipleActiveSpecs(),
		ScenarioStaleWorktree(),
		ScenarioCompleteFromSpecWorktree(),
		ScenarioApproveSpecFromWorktree(),
		ScenarioApprovePlanFromWorktree(),
		ScenarioBugfixBranch(),
		ScenarioBlockedBeadTransition(),
		ScenarioUnmergedBeadGuard(),
		ScenarioStopAfterComplete(),
		ScenarioStopDoesNotBlockApproveImpl(),
		ScenarioBeadsArtifactPassthrough(),
		ScenarioStalePhaseImplApprove(),
		ScenarioCompleteFromDoomedWorktree(),
		ScenarioPrecommitReexportComplete(),
		ScenarioWrongDirectoryGuardRecovery(),
		ScenarioApprovalGateDiscovery(),
	}
}
