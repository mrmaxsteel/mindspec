package harness

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// --- Evidence-gating helper unit tests (panel R2-M1 + R3-MAJ-1) ---
//
// Synthetic event streams derived from the committed run-3 falsepos
// transcript (review/prep/bead9_green_run3_reexport_FAIL_falsepos.log):
// a parent `mindspec complete` event logged at EXIT (so it appears
// AFTER its git children in the stream) whose duration window covers
// mindspec's own executor-spawned `git add -A` / auto-commit /
// `chore: sync beads artifact` subprocesses.

// falseposStyleEvents builds the run-3 falsepos shape: agent runs
// complete at t0; the executor's git children land (shim exit-time
// stamps) inside the parent's window; the parent complete event is
// logged LAST, at t0+8s, with DurationMS=8000.
func falseposStyleEvents(beadWt string) (events []ActionEvent, parentEnd time.Time) {
	parentEnd = time.Date(2026, 6, 11, 15, 30, 8, 0, time.UTC) // t0+8s
	events = []ActionEvent{
		{Command: "git", ArgsList: []string{"-C", beadWt, "add", "-A"},
			Timestamp: parentEnd.Add(-6 * time.Second), ExitCode: 0},
		{Command: "git", ArgsList: []string{"-C", beadWt, "commit", "-m", "impl(repo-11s.1): Created reexport.go with Reexport() function"},
			Timestamp: parentEnd.Add(-5 * time.Second), ExitCode: 0},
		{Command: "git", ArgsList: []string{"-C", beadWt, "add", "-A"},
			Timestamp: parentEnd.Add(-3 * time.Second), ExitCode: 0},
		{Command: "git", ArgsList: []string{"-C", beadWt, "commit", "-m", "chore: sync beads artifact"},
			Timestamp: parentEnd.Add(-2 * time.Second), ExitCode: 0},
		{Command: "mindspec", ArgsList: []string{"complete", "repo-11s.1", "Created reexport.go with Reexport() function"},
			Timestamp: parentEnd, DurationMS: 8000, ExitCode: 0},
	}
	return events, parentEnd
}

// TestManualArtifactCommitViolations_FalseposEventsExcluded replays the
// run-3 falsepos shape under the corrected logic: mindspec's own
// executor-spawned add/commit children must NOT be flagged.
func TestManualArtifactCommitViolations_FalseposEventsExcluded(t *testing.T) {
	events, _ := falseposStyleEvents("/sandbox/.worktrees/worktree-repo-11s.1")
	if got := manualArtifactCommitViolations(events); len(got) != 0 {
		t.Errorf("mindspec-spawned executor commits must not be flagged; got violations: %v", got)
	}
}

// TestManualArtifactCommitViolations_ParentWindowNeedsUntruncatedStream
// pins the R3-MAJ-1 mechanism itself: the parent complete event is
// logged at exit and lands AFTER its git children — at the scan
// boundary — so attribution against the TRUNCATED scan window (the
// pre-fix bug) cannot see the parent, while attribution against the
// full stream can. The child here ends 5s before the parent, far
// outside any ±1s slack, so only the parent's duration window (not the
// slack accident) can attribute it.
func TestManualArtifactCommitViolations_ParentWindowNeedsUntruncatedStream(t *testing.T) {
	events, _ := falseposStyleEvents("/wt")
	child := events[1] // executor auto-commit, parentEnd-5s
	parentIdx := len(events) - 1

	if !mindspecSpawnedGit(events, child) {
		t.Error("full-stream attribution must cover the executor child via the parent's duration window")
	}
	if mindspecSpawnedGit(events[:parentIdx], child) {
		t.Error("truncated-window attribution unexpectedly matched — the parent is outside the slice; this test's premise is broken")
	}
}

// TestManualArtifactCommitViolations_AgentBlanketCommitFlagged: an
// agent-issued `git commit -am` BEFORE the complete and OUTSIDE any
// mindspec window (90s earlier) must be flagged.
func TestManualArtifactCommitViolations_AgentBlanketCommitFlagged(t *testing.T) {
	events, parentEnd := falseposStyleEvents("/wt")
	agent := ActionEvent{
		Command:   "git",
		ArgsList:  []string{"commit", "-am", "checkpoint everything"},
		Timestamp: parentEnd.Add(-90 * time.Second),
		ExitCode:  0,
	}
	events = append([]ActionEvent{agent}, events...)

	got := manualArtifactCommitViolations(events)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 violation for the agent's blanket commit, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0], "checkpoint everything") {
		t.Errorf("violation should cite the agent commit; got %q", got[0])
	}
}

// TestManualArtifactCommitViolations_ExplicitBeadsInsideWindowStillFlagged:
// an explicit `.beads` pathspec is agent-only (mindspec never passes
// one) and must be flagged even INSIDE a mindspec window — attribution
// applies only to the fuzzy blanket heuristics.
func TestManualArtifactCommitViolations_ExplicitBeadsInsideWindowStillFlagged(t *testing.T) {
	events, parentEnd := falseposStyleEvents("/wt")
	inWindow := ActionEvent{
		Command:   "git",
		ArgsList:  []string{"add", ".beads/issues.jsonl"},
		Timestamp: parentEnd.Add(-4 * time.Second),
		ExitCode:  0,
	}
	// Insert before the parent (stream order: children first).
	events = append(events[:4:4], append([]ActionEvent{inWindow}, events[4:]...)...)

	got := manualArtifactCommitViolations(events)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 violation for the explicit .beads add, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0], ".beads/issues.jsonl") {
		t.Errorf("violation should cite the .beads pathspec; got %q", got[0])
	}
}

// TestManualArtifactCommitViolations_ZeroDurationParentNoAttribution:
// a parent with DurationMS=0 (shim clock fallback) has no window — the
// <=0 guard skips it, so its in-stream "children" are treated as
// agent-issued and the blanket commit is flagged.
func TestManualArtifactCommitViolations_ZeroDurationParentNoAttribution(t *testing.T) {
	events, _ := falseposStyleEvents("/wt")
	events[len(events)-1].DurationMS = 0

	got := manualArtifactCommitViolations(events)
	if len(got) == 0 {
		t.Error("with a zero-duration parent there is no attribution window; the blanket add+commit chain must be flagged")
	}
}
