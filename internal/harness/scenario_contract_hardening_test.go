package harness

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// --- Spec 092 LLM regression scenarios (HC-6) ---
//
// All five are recovery-style pins for the 2026-06-10 field notes. Analyzer
// wrong-actions are logged (not failed) — the scenario-specific assertions
// carry the regression pin.

func runContractHardeningScenario(t *testing.T, scenario Scenario) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping LLM test in short mode")
	}
	report, _ := runScenario(t, scenario)
	if len(report.WrongActions) > 0 {
		t.Logf("wrong actions (logged, recovery-style scenario): %d", len(report.WrongActions))
		for _, wa := range report.WrongActions {
			t.Logf("  [%s] %s", wa.Rule, wa.Reason)
		}
	}
}

func TestLLM_StalePhaseImplApprove(t *testing.T) {
	runContractHardeningScenario(t, ScenarioStalePhaseImplApprove())
}

func TestLLM_CompleteFromDoomedWorktree(t *testing.T) {
	runContractHardeningScenario(t, ScenarioCompleteFromDoomedWorktree())
}

func TestLLM_PrecommitReexportComplete(t *testing.T) {
	runContractHardeningScenario(t, ScenarioPrecommitReexportComplete())
}

func TestLLM_WrongDirectoryGuardRecovery(t *testing.T) {
	runContractHardeningScenario(t, ScenarioWrongDirectoryGuardRecovery())
}

func TestLLM_ApprovalGateDiscovery(t *testing.T) {
	runContractHardeningScenario(t, ScenarioApprovalGateDiscovery())
}

// --- Deterministic unit tests (no LLM) ---

func TestAllScenariosRegistersContractHardening(t *testing.T) {
	want := map[string]bool{
		"stale_phase_impl_approve":       false,
		"complete_from_doomed_worktree":  false,
		"precommit_reexport_complete":    false,
		"wrong_directory_guard_recovery": false,
		"approval_gate_discovery":        false,
	}
	for _, s := range AllScenarios() {
		if _, ok := want[s.Name]; ok {
			want[s.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("scenario %q is not registered in AllScenarios()", name)
		}
	}
}

func TestArgsInOrder(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		first, second string
		want          bool
	}{
		{"canonical impl approve", []string{"impl", "approve", "001-gate"}, "impl", "approve", true},
		{"deprecated approve impl", []string{"approve", "impl", "001-gate"}, "approve", "impl", true},
		{"canonical is not deprecated", []string{"impl", "approve", "001-gate"}, "approve", "impl", false},
		{"deprecated is not canonical", []string{"approve", "impl", "001-gate"}, "impl", "approve", false},
		{"missing token", []string{"approve", "spec", "001-x"}, "approve", "impl", false},
		{"empty args", nil, "impl", "approve", false},
		{"substring does not match", []string{"implx", "approve"}, "impl", "approve", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := argsInOrder(tt.args, tt.first, tt.second); got != tt.want {
				t.Errorf("argsInOrder(%v, %q, %q) = %v, want %v", tt.args, tt.first, tt.second, got, tt.want)
			}
		})
	}
}

func TestResolveStartDir(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, ".worktrees", "worktree-spec-001-x", ".worktrees", "worktree-abc-1")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Run("plain relative path", func(t *testing.T) {
		got, err := resolveStartDir(root, ".worktrees/worktree-spec-001-x")
		if err != nil {
			t.Fatalf("resolveStartDir: %v", err)
		}
		if got != filepath.Join(root, ".worktrees", "worktree-spec-001-x") {
			t.Errorf("got %q", got)
		}
	})

	t.Run("glob resolves single match", func(t *testing.T) {
		got, err := resolveStartDir(root, ".worktrees/worktree-spec-001-x/.worktrees/worktree-*")
		if err != nil {
			t.Fatalf("resolveStartDir: %v", err)
		}
		if got != nested {
			t.Errorf("got %q, want %q", got, nested)
		}
	})

	t.Run("missing dir errors", func(t *testing.T) {
		if _, err := resolveStartDir(root, "no/such/dir"); err == nil {
			t.Error("expected error for missing directory")
		}
	})

	t.Run("ambiguous glob errors", func(t *testing.T) {
		if err := os.MkdirAll(filepath.Join(root, ".worktrees", "worktree-spec-001-x", ".worktrees", "worktree-abc-2"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := resolveStartDir(root, ".worktrees/worktree-spec-001-x/.worktrees/worktree-*"); err == nil {
			t.Error("expected error for ambiguous glob")
		}
	})

	t.Run("file is not a directory", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(root, "afile"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := resolveStartDir(root, "afile"); err == nil {
			t.Error("expected error for non-directory")
		}
	})
}

// dirCapturingAgent records the RunOpts it was invoked with.
type dirCapturingAgent struct {
	gotOpts RunOpts
}

func (a *dirCapturingAgent) Name() string { return "dir-capture" }
func (a *dirCapturingAgent) Run(_ context.Context, _ *Sandbox, _ string, opts RunOpts) (*SessionResult, error) {
	a.gotOpts = opts
	return &SessionResult{}, nil
}

// TestRunSessionStartDirPlumbing verifies Scenario.StartDir reaches
// RunOpts.Dir (resolved after Setup), and that the default stays empty
// (= sandbox.Root in Agent.Run).
func TestRunSessionStartDirPlumbing(t *testing.T) {
	root := t.TempDir()
	sandbox := &Sandbox{Root: root, t: t}

	t.Run("default is empty", func(t *testing.T) {
		agent := &dirCapturingAgent{}
		_, err := RunSession(context.Background(), agent, Scenario{Name: "x"}, sandbox)
		if err != nil {
			t.Fatalf("RunSession: %v", err)
		}
		if agent.gotOpts.Dir != "" {
			t.Errorf("RunOpts.Dir = %q, want empty (sandbox.Root default)", agent.gotOpts.Dir)
		}
	})

	t.Run("StartDir created by Setup is resolved", func(t *testing.T) {
		agent := &dirCapturingAgent{}
		scenario := Scenario{
			Name:     "x",
			StartDir: "sub/wt-*",
			Setup: func(s *Sandbox) error {
				// Worktree dirs appear during Setup in real scenarios.
				return os.MkdirAll(filepath.Join(s.Root, "sub", "wt-dynamic-1"), 0o755)
			},
		}
		_, err := RunSession(context.Background(), agent, scenario, sandbox)
		if err != nil {
			t.Fatalf("RunSession: %v", err)
		}
		want := filepath.Join(root, "sub", "wt-dynamic-1")
		if agent.gotOpts.Dir != want {
			t.Errorf("RunOpts.Dir = %q, want %q", agent.gotOpts.Dir, want)
		}
	})

	t.Run("unresolvable StartDir errors", func(t *testing.T) {
		agent := &dirCapturingAgent{}
		_, err := RunSession(context.Background(), agent, Scenario{Name: "x", StartDir: "missing-dir"}, sandbox)
		if err == nil {
			t.Error("expected error for unresolvable StartDir")
		}
	})
}
