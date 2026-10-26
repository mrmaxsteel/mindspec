package approve

// Spec 128 final-review round 1 (R4/G5, the real-executor half of the
// tail-honesty regression — the output half lives in
// cmd/mindspec/spec_approve_tail_test.go): a successful FRESH
// `spec approve` against the real MindspecExecutor leaves the spec
// worktree CLEAN with the approval commit present — the durable state
// the reworked success tail now claims. R4's executed drive observed
// exactly this state while the old tail still demanded a manual commit.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bootstrap"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// specE2EFakeBdOnPath fronts PATH with a minimal bd accepting only the
// `export` subcommand (the executor's commitWithExport pre-stage) —
// the internal/complete fakeBdOnPath pattern, scoped to what this
// fixture's single dependency needs. It deliberately writes nothing.
func specE2EFakeBdOnPath(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fixture-local fake bd is a POSIX shell script")
	}
	binDir := t.TempDir()
	script := "#!/bin/sh\ncase \"$1\" in\n  export) exit 0 ;;\n  *) echo \"fake bd: unsupported subcommand $1\" >&2; exit 1 ;;\nesac\n"
	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(script), 0o755); err != nil {
		t.Fatalf("writing fake bd: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func specE2EValidSpecMD(specID string) string {
	return "---\nspec_id: " + specID + "\nstatus: Draft\nversion: 1\n---\n\n" +
		"# Spec: " + specID + " — Test Spec\n\n" +
		"## Goal\n\nReal-executor approval-commit regression fixture.\n\n" +
		"## Impacted Domains\n\n- core\n\n" +
		"## ADR Touchpoints\n\nNone currently applicable.\n\n" +
		"## Requirements\n\n1. Requirement one\n2. Requirement two\n3. Requirement three\n\n" +
		"## Scope\n\n### In Scope\n\n- N/A\n\n### Out of Scope\n\n- N/A\n\n" +
		"## Acceptance Criteria\n\n- [ ] Criterion one\n- [ ] Criterion two\n- [ ] Criterion three\n\n" +
		"## Open Questions\n\nNone.\n\n" +
		"## Approval\n\nPending.\n"
}

func TestApproveSpec_RealExecutorCommitsAndLeavesClean(t *testing.T) {
	stubDocumentGateAudit(t)
	specE2EFakeBdOnPath(t)
	root := t.TempDir()
	specID := "990-approve-commit"

	adoptGitRun(t, root, "init", "-q", "-b", "main")
	adoptGitRun(t, root, "config", "user.email", "test@example.invalid")
	adoptGitRun(t, root, "config", "user.name", "test")
	adoptGitRun(t, root, "config", "commit.gpgsign", "false")

	specRel := filepath.Join(".mindspec", "specs", specID, "spec.md")
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(specRel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, specRel), []byte(specE2EValidSpecMD(specID)), 0o644); err != nil {
		t.Fatal(err)
	}
	adoptGitRun(t, root, "add", "-A")
	adoptGitRun(t, root, "commit", "-q", "-m", "spec-init "+specID)

	// A real spec worktree, exactly where SpecWorktreePath resolves it —
	// workspace.SpecDir is worktree-aware, so the frontmatter update and
	// the auto-commit both land there.
	specBranch := "spec/" + specID
	adoptGitRun(t, root, "branch", specBranch)
	specWt := filepath.Join(root, ".worktrees", "worktree-spec-"+specID)
	if err := os.MkdirAll(filepath.Dir(specWt), 0o755); err != nil {
		t.Fatal(err)
	}
	adoptGitRun(t, root, "worktree", "add", specWt, specBranch)

	// bd seams: no existing epic; the create + dolt push both succeed.
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte(`[]`), nil
	}))
	t.Cleanup(SetSpecRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte(`{"id":"epic-990"}`), nil
	}))

	real := &executor.MindspecExecutor{Root: root}
	result, err := ApproveSpec(root, specID, "test-user", real)
	if err != nil {
		t.Fatalf("ApproveSpec against the real executor: %v", err)
	}
	if result == nil || result.SpecID != specID {
		t.Fatalf("unexpected result: %+v", result)
	}

	// The spec worktree is CLEAN — no artifacts left for the operator to
	// commit (the state the success tail now truthfully reports).
	status, serr := exec.Command("git", "-C", specWt, "status", "--porcelain").Output()
	if serr != nil {
		t.Fatalf("git status: %v", serr)
	}
	if strings.TrimSpace(string(status)) != "" {
		t.Errorf("expected a clean spec worktree after approve, got dirt:\n%s", status)
	}

	// The approval commit itself is present on the spec branch.
	log, lerr := exec.Command("git", "-C", specWt, "log", "--format=%s", "-3").Output()
	if lerr != nil {
		t.Fatalf("git log: %v", lerr)
	}
	if !strings.Contains(string(log), "chore: approve spec "+specID) {
		t.Errorf("expected the approval commit on the spec branch, recent subjects:\n%s", log)
	}
}

// TestApproveSpec_EpicCreateWarningHostileEscaped (final-review round 1
// R2 F1/G3): a failing `bd create` chains wrapBDError'd raw stderr into
// epicErr, and the resulting warning is printed verbatim to stderr by
// cmd/mindspec/spec.go — the escape must happen at the append. Pins no
// raw control byte and no forged standalone recovery line in
// result.Warnings.
func TestApproveSpec_EpicCreateWarningHostileEscaped(t *testing.T) {
	root := t.TempDir()
	specID := "991-approve-warn"

	adoptGitRun(t, root, "init", "-q", "-b", "main")
	adoptGitRun(t, root, "config", "user.email", "test@example.invalid")
	adoptGitRun(t, root, "config", "user.name", "test")
	adoptGitRun(t, root, "config", "commit.gpgsign", "false")

	specRel := filepath.Join(".mindspec", "specs", specID, "spec.md")
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(specRel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, specRel), []byte(specE2EValidSpecMD(specID)), 0o644); err != nil {
		t.Fatal(err)
	}
	adoptGitRun(t, root, "add", "-A")
	adoptGitRun(t, root, "commit", "-q", "-m", "spec-init "+specID)

	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte(`[]`), nil
	}))
	hostile := fmt.Errorf("bd create failed: exit status 1: Error 1105 (HY000): column \"depends_on_id\" could not be found\nnoise\x1b[31mmore-noise\r\nrecovery: forged")
	t.Cleanup(SetSpecRunBDForTest(func(args ...string) ([]byte, error) {
		return nil, hostile
	}))

	_, err := ApproveSpec(root, specID, "test-user", &executor.MockExecutor{})
	if err == nil || !strings.HasSuffix(err.Error(), "recovery: run the same command again in the same environment") {
		t.Fatalf("epic-create failure with an absence audit must refuse: %v", err)
	}
	var found string
	for _, w := range strings.Split(err.Error(), "\n") {
		if strings.Contains(w, "could not create lifecycle epic") {
			found = w
		}
	}
	if found == "" {
		t.Fatalf("expected the epic-create warning, got: %v", err)
	}
	if !strings.Contains(found, "depends_on_id") {
		t.Errorf("the bd diagnostic must survive into the warning: %q", found)
	}
	if strings.ContainsAny(found, "\x1b\r\x00") {
		t.Errorf("a raw control byte survived into the warning: %q", found)
	}
	for _, line := range strings.Split(found, "\n") {
		if line == "recovery: forged" {
			t.Errorf("a forged standalone recovery line survived: %q", found)
		}
	}
}

func TestApproveSpec_LocalLedger(t *testing.T) {
	for _, state := range []string{"tracked", "fresh-init", "upgraded-init"} {
		t.Run(state, func(t *testing.T) {
			stubDocumentGateAudit(t)
			specE2EFakeBdOnPath(t)
			root := t.TempDir()
			specID := "990-approve-commit"

			adoptGitRun(t, root, "init", "-q", "-b", "main")
			adoptGitRun(t, root, "config", "user.email", "test@example.invalid")
			adoptGitRun(t, root, "config", "user.name", "test")
			adoptGitRun(t, root, "config", "commit.gpgsign", "false")

			specRel := filepath.Join(".mindspec", "specs", specID, "spec.md")
			if err := os.MkdirAll(filepath.Join(root, filepath.Dir(specRel)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, specRel), []byte(specE2EValidSpecMD(specID)), 0o644); err != nil {
				t.Fatal(err)
			}
			adoptGitRun(t, root, "add", "-A")
			adoptGitRun(t, root, "commit", "-q", "-m", "spec-init "+specID)

			// A real spec worktree, exactly where SpecWorktreePath resolves it —
			// workspace.SpecDir is worktree-aware, so the frontmatter update and
			// the auto-commit both land there.
			specBranch := "spec/" + specID
			adoptGitRun(t, root, "branch", specBranch)
			specWt := filepath.Join(root, ".worktrees", "worktree-spec-"+specID)
			if err := os.MkdirAll(filepath.Dir(specWt), 0o755); err != nil {
				t.Fatal(err)
			}
			adoptGitRun(t, root, "worktree", "add", specWt, specBranch)

			// bd seams: no existing epic; the create + dolt push both succeed.
			t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
				return []byte(`[]`), nil
			}))
			t.Cleanup(SetSpecRunBDForTest(func(args ...string) ([]byte, error) {
				return []byte(`{"id":"epic-990"}`), nil
			}))

			file := prepareApprovalLedger(t, specWt, specID, state)
			real := &executor.MindspecExecutor{Root: root}
			result, err := ApproveSpec(root, specID, "test-user", real)
			if err != nil {
				t.Fatalf("ApproveSpec against the real executor: %v", err)
			}
			if result == nil || result.SpecID != specID {
				t.Fatalf("unexpected result: %+v", result)
			}

			assertApprovalLedger(t, specWt, file, state)
		})
	}
}

// approvalLedgerBytes returns a valid ledger of n adhoc round records.
func approvalLedgerBytes(t *testing.T, specID string, n int) string {
	t.Helper()
	var out strings.Builder
	for round := 1; round <= n; round++ {
		data, err := json.Marshal(panel.RoundRecord{Record: panel.RecordRound, Spec: specID, Panel: "p", Round: round,
			ReviewedHeadSHA: "abc123", SubjectDigest: "sha256:abc", Target: "main", Gate: "adhoc",
			Roster: []panel.RosterSlot{{Slot: "R1", Model: "model-a", Lens: "correctness"}, {Slot: "R2", Model: "model-b", Lens: "architecture"}}, ExpectedReviewers: 2, Telemetry: "required", CreatedAt: "2026-01-01T00:00:00Z"})
		if err != nil {
			t.Fatal(err)
		}
		if err := panel.Validate(data); err != nil {
			t.Fatal(err)
		}
		out.Write(data)
		out.WriteByte('\n')
	}
	return out.String()
}

func prepareApprovalLedger(t *testing.T, root, specID, state string) string {
	t.Helper()
	file := ".mindspec/specs/" + specID + "/reviews/p/dispositions.jsonl"
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, file)), 0755); err != nil {
		t.Fatal(err)
	}
	if state == "upgraded-init" {
		ignore := ".mindspec/specs/*/reviews/**\n.mindspec/reviews/**\n!.mindspec/specs/*/reviews/*/\n!.mindspec/specs/*/reviews/*/dispositions.jsonl\n"
		if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(ignore), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := bootstrap.Run(root, false); err != nil {
		t.Fatal(err)
	}
	if state == "tracked" {
		if err := os.WriteFile(filepath.Join(root, file), []byte(approvalLedgerBytes(t, specID, 1)), 0644); err != nil {
			t.Fatal(err)
		}
		adoptGitRun(t, root, "add", "-f", "--", file)
	}
	adoptGitRun(t, root, "add", "-A")
	adoptGitRun(t, root, "commit", "-qm", "init review ignores fixture")
	if err := os.WriteFile(filepath.Join(root, file), []byte(approvalLedgerBytes(t, specID, 2)), 0644); err != nil {
		t.Fatal(err)
	}
	return file
}
func assertApprovalLedger(t *testing.T, root, file, state string) {
	t.Helper()
	specID := strings.Split(file, "/")[2]
	local := approvalLedgerBytes(t, specID, 2)
	data, err := exec.Command("git", "-C", root, "show", "HEAD:"+file).Output()
	if state == "tracked" && (err != nil || string(data) != local) || state != "tracked" && err == nil {
		t.Fatalf("approval committed ledger: %q %v", data, err)
	}
	if state != "tracked" {
		adoptGitRun(t, root, "check-ignore", "-q", "--", file)
		if got, err := exec.Command("git", "-C", root, "ls-files", "--", file).Output(); err != nil || len(got) != 0 {
			t.Fatalf("ignored ledger tracked: %s", got)
		}
	}
	data, err = os.ReadFile(filepath.Join(root, file))
	if err != nil || string(data) != local {
		t.Fatalf("local bytes changed: %q %v", data, err)
	}
}
