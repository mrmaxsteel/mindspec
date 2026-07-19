package harness

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
)

func assertCommandRan(t *testing.T, events []ActionEvent, command string, argSubstr ...string) { //nolint:unparam // command kept for call-site clarity
	t.Helper()
	for _, e := range events {
		if e.Command != command {
			continue
		}
		if e.ExitCode != 0 {
			continue
		}
		if len(argSubstr) == 0 {
			return // found successful command
		}
		args := eventArgs(e)
		if containsAll(args, argSubstr[0]) {
			return
		}
	}
	if len(argSubstr) > 0 {
		t.Errorf("command %q with arg %q was not found with exit code 0 in events", command, argSubstr[0])
	} else {
		t.Errorf("command %q was not found with exit code 0 in events", command)
	}
}

// commandRanSuccessfully returns true if the command ran with exit code 0
// and all argSubstr found in its args (non-asserting version of assertCommandRan).
func commandRanSuccessfully(events []ActionEvent, command string, argSubstr ...string) bool { //nolint:unparam // command may vary in future scenarios
	for _, e := range events {
		if e.Command != command || e.ExitCode != 0 {
			continue
		}
		if len(argSubstr) == 0 {
			return true
		}
		args := eventArgs(e)
		if containsAll(args, argSubstr[0]) {
			return true
		}
	}
	return false
}

// assertCommandRanEither checks that the command was invoked with one of the
// given arg patterns (each is a list of substrings that must all appear).
func assertCommandRanEither(t *testing.T, events []ActionEvent, command string, patterns ...[]string) {
	t.Helper()
	for _, e := range events {
		if e.Command != command {
			continue
		}
		if e.ExitCode != 0 {
			continue
		}
		args := eventArgs(e)
		for _, pattern := range patterns {
			matched := true
			for _, sub := range pattern {
				if !containsAll(args, sub) {
					matched = false
					break
				}
			}
			if matched {
				return
			}
		}
	}
	t.Errorf("command %q was not found with exit code 0 for any expected arg patterns %v", command, patterns)
}

func assertCommandContains(t *testing.T, events []ActionEvent, command, substr string) { //nolint:unparam // command may vary in future scenarios
	t.Helper()
	for _, e := range events {
		if e.Command != command {
			continue
		}
		if e.ExitCode != 0 {
			continue
		}
		args := eventArgs(e)
		for _, arg := range args {
			if arg == substr {
				return
			}
		}
	}
	t.Errorf("command %q with arg containing %q was not found with exit code 0 in events", command, substr)
}

// eventArgs returns args from both the Args map and ArgsList slice.
func eventArgs(e ActionEvent) []string {
	args := flatArgs(e.Args)
	args = append(args, e.ArgsList...)
	return args
}

// assertCommandSucceeded checks that the command was run AND exited with code 0.
func assertCommandSucceeded(t *testing.T, events []ActionEvent, command string, argSubstr ...string) {
	t.Helper()
	for _, e := range events {
		if e.Command != command {
			continue
		}
		if e.ExitCode != 0 {
			continue
		}
		args := eventArgs(e)
		matched := true
		for _, sub := range argSubstr {
			if !containsAll(args, sub) {
				matched = false
				break
			}
		}
		if matched {
			return
		}
	}
	if len(argSubstr) == 0 {
		t.Errorf("command %q was not found with exit code 0 in events", command)
		return
	}
	t.Errorf("command %q with args %v was not found with exit code 0 in events", command, argSubstr)
}

// assertNoPreApproveImplMainMergeOrPR enforces workflow ordering at the test
// layer: no direct merge-to-main or PR creation before approve impl is invoked.
//
// Note: internal git merge commands executed *inside* `mindspec approve impl`
// appear in event logs before the top-level `mindspec approve impl` event due to
// wrapper timing. We treat the known canonical internal merge command as allowed.
func assertNoPreApproveImplMainMergeOrPR(t *testing.T, events []ActionEvent) {
	t.Helper()
	if err := preApproveImplMainMergeOrPRViolation(events); err != nil {
		t.Fatal(err)
	}
}

func preApproveImplMainMergeOrPRViolation(events []ActionEvent) error {
	approveSeen := false
	for _, e := range events {
		args := eventArgs(e)

		if e.Command == "mindspec" && containsAll(args, "approve") && containsAll(args, "impl") {
			approveSeen = true
			continue
		}

		if approveSeen {
			continue
		}

		// Fail if PR creation/merge is attempted before approve impl.
		if e.Command == "gh" && (containsAll(args, "pr") && (containsAll(args, "create") || containsAll(args, "merge"))) {
			return fmt.Errorf("PR command ran before approve impl: %v", args)
		}

		// Fail if a non-canonical merge-to-main is attempted before approve impl.
		// Canonical internal merge pattern (from approve impl) is allowed:
		//   git ... merge --no-ff spec/<id> -m "Merge spec/<id> into main"
		if e.Command == "git" && e.ExitCode == 0 && containsAll(args, "merge") && containsAll(args, "main") {
			refs := mergeSourceRefs(args)
			// `git merge main` (only main as a source ref) merges main INTO
			// the current branch — a safe pull-main-in update, not a
			// merge-to-main. Allow it.
			allMain := len(refs) > 0
			for _, ref := range refs {
				if ref != "main" {
					allMain = false
					break
				}
			}
			if allMain {
				continue
			}

			isCanonicalInternal := containsAll(args, "--no-ff") &&
				containsAll(args, "spec/") &&
				containsAll(args, "-m") &&
				containsAll(args, "Merge spec/") &&
				containsAll(args, "into main")
			if !isCanonicalInternal {
				return fmt.Errorf("non-canonical merge of %v before approve impl (may land onto main): %v", refs, args)
			}
		}
	}

	return nil
}

// mergeSourceRefs returns the positional ref operands that follow the `merge`
// token in a `git merge ...` invocation, skipping flags and their values. It is
// used to distinguish a safe pull-main-in (`git merge main`) from a merge whose
// result could land onto main.
func mergeSourceRefs(args []string) []string {
	var refs []string
	seenMerge := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !seenMerge {
			if arg == "merge" {
				seenMerge = true
			}
			continue
		}
		switch {
		case arg == "--":
			// Separator; remaining args are operands.
			continue
		case arg == "-m" || arg == "-C" || arg == "-s" || arg == "-X":
			// Flag with a following value; skip both.
			i++
		case strings.HasPrefix(arg, "-"):
			// Any other flag; skip it.
		default:
			refs = append(refs, arg)
		}
	}
	return refs
}

func assertBranchIs(t *testing.T, sandbox *Sandbox, expected string) {
	t.Helper()
	actual := sandbox.GitBranch()
	if actual != expected {
		t.Errorf("expected current branch %q, got %q", expected, actual)
	}
}

func assertNoBranches(t *testing.T, sandbox *Sandbox, prefix string) {
	t.Helper()
	branches := sandbox.ListBranches(prefix)
	if len(branches) > 0 {
		t.Errorf("expected no branches with prefix %q, found: %v", prefix, branches)
	}
}

func assertNoWorktrees(t *testing.T, sandbox *Sandbox) {
	t.Helper()
	wts := sandbox.ListWorktrees()
	if len(wts) > 0 {
		t.Errorf("expected no worktrees, found: %v", wts)
	}
}

func assertEventCWDContains(t *testing.T, events []ActionEvent, substr string) {
	t.Helper()
	for _, e := range events {
		if strings.Contains(e.CWD, substr) {
			return
		}
	}
	t.Errorf("no event had CWD containing %q", substr)
}

func assertHasBranches(t *testing.T, sandbox *Sandbox, prefix string) {
	t.Helper()
	branches := sandbox.ListBranches(prefix)
	if len(branches) == 0 {
		t.Errorf("expected at least one branch with prefix %q, found none", prefix)
	}
}

func assertHasWorktrees(t *testing.T, sandbox *Sandbox) {
	t.Helper()
	wts := sandbox.ListWorktrees()
	if len(wts) == 0 {
		t.Error("expected at least one worktree, found none")
	}
}

// assertNoUserFilesModified checks that no files outside .mindspec/ are dirty.
// .mindspec/session.json is written by the SessionStart hook and is expected noise.
func assertNoUserFilesModified(t *testing.T, sandbox *Sandbox) {
	t.Helper()
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = sandbox.Root
	out, err := cmd.Output()
	if err != nil {
		t.Errorf("git status failed: %v", err)
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		// Skip .mindspec/ infrastructure files (session.json, focus, etc.)
		file := strings.TrimSpace(line[2:]) // strip status prefix
		if strings.HasPrefix(file, ".mindspec/") {
			continue
		}
		t.Errorf("unexpected modified file outside .mindspec/: %s", line)
	}
}

// assertHasNonMainBranch checks that at least one branch besides "main" exists.
func assertHasNonMainBranch(t *testing.T, sandbox *Sandbox) {
	t.Helper()
	cmd := exec.Command("git", "branch", "--list")
	cmd.Dir = sandbox.Root
	out, err := cmd.Output()
	if err != nil {
		t.Errorf("git branch --list failed: %v", err)
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		b := strings.TrimSpace(strings.TrimPrefix(line, "* "))
		if b != "" && b != "main" {
			return
		}
	}
	t.Error("expected at least one non-main branch, found none")
}

// assertMainCommitCountUnchanged verifies that main has the same number of
// commits as when setup recorded the count in .harness/main_commit_count.
// Infrastructure commits (e.g. bd prime's .beads/backup) are excluded —
// only user-file-touching commits count.
func assertMainCommitCountUnchanged(t *testing.T, sandbox *Sandbox) {
	t.Helper()
	expected := strings.TrimSpace(sandbox.ReadFile(".harness/main_commit_count"))

	// Count commits on main, excluding those that ONLY touch .beads/ files
	// (bd prime commits .beads/backup during SessionStart — not agent work)
	cmd := exec.Command("git", "rev-list", "main")
	cmd.Dir = sandbox.Root
	out, err := cmd.Output()
	if err != nil {
		t.Errorf("git rev-list main failed: %v", err)
		return
	}
	userCommits := 0
	for _, sha := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if sha == "" {
			continue
		}
		// Check what files this commit touched
		diffCmd := exec.Command("git", "diff-tree", "--no-commit-id", "--name-only", "-r", sha)
		diffCmd.Dir = sandbox.Root
		diffOut, err := diffCmd.Output()
		if err != nil {
			userCommits++ // assume user commit if we can't check
			continue
		}
		files := strings.TrimSpace(string(diffOut))
		if files == "" {
			userCommits++ // empty diff = initial commit or merge
			continue
		}
		// If ALL changed files are under .beads/, it's infrastructure
		allBeads := true
		for _, f := range strings.Split(files, "\n") {
			if !strings.HasPrefix(f, ".beads/") {
				allBeads = false
				break
			}
		}
		if !allBeads {
			userCommits++
		}
	}
	expectedInt := 0
	fmt.Sscanf(expected, "%d", &expectedInt)
	if userCommits != expectedInt {
		t.Errorf("main branch user commit count changed: expected %d, got %d (agent committed directly to main)", expectedInt, userCommits)
	}
}

// beadStatus is the minimal structure returned by `bd list --json`.
type beadStatus struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Title  string `json:"title"`
}

// beadStatusStr returns the status of a bead by ID, or "unknown" on error.
//
// R7 (spec 120, round 10): beadID is an id-position operand reaching a
// `bd show` spawn via runBD — gated with requireValidBeadID, fail-fast
// t.Fatalf BEFORE the spawn.
func beadStatusStr(sandbox *Sandbox, beadID string) string {
	requireValidBeadID(sandbox.t, beadID)
	out, err := sandbox.runBD("show", beadID, "--json")
	if err != nil {
		return "unknown"
	}
	var infos []beadStatus
	if err := json.Unmarshal([]byte(out), &infos); err != nil || len(infos) == 0 {
		return "unknown"
	}
	return strings.TrimSpace(strings.ToLower(infos[0].Status))
}

// assertBeadsMinCount verifies that at least minCount child beads exist under
// the given epicID. Useful when bead IDs are created dynamically (e.g. by plan approve).
func assertBeadsMinCount(t testing.TB, sandbox *Sandbox, epicID string, minCount int) {
	t.Helper()
	// R7 (spec 120, round 10): epicID is an id-position operand reaching
	// a `bd list --parent` spawn via runBD — gated with
	// requireValidBeadID, fail-fast t.Fatalf BEFORE the spawn.
	requireValidBeadID(t, epicID)
	// Query all statuses to count beads regardless of lifecycle state.
	var allBeads []beadStatus
	for _, status := range []string{"open", "in_progress", "closed"} {
		out, err := sandbox.runBD("list", "--json", "--parent", epicID, "--status="+status)
		if err != nil {
			continue
		}
		var beads []beadStatus
		if err := json.Unmarshal([]byte(out), &beads); err != nil {
			continue
		}
		allBeads = append(allBeads, beads...)
	}
	if len(allBeads) < minCount {
		t.Errorf("expected at least %d beads under epic %s, got %d", minCount, epicID, len(allBeads))
	}
}

// assertBeadsState queries individual bead statuses via `bd show <id> --json`
// and asserts each matches the expected status. Uses per-bead queries instead
// of `bd list --json --parent` which has a known bug where --json is ignored
// when combined with --parent/--status filters.
func assertBeadsState(t testing.TB, sandbox *Sandbox, _ string, expectedStatuses map[string]string) {
	t.Helper()
	for id, want := range expectedStatuses {
		// R7 (spec 120, round 10): id is an id-position operand reaching
		// a `bd show` spawn via runBD — gated with requireValidBeadID,
		// fail-fast t.Fatalf BEFORE the spawn.
		requireValidBeadID(t, id)
		out, err := sandbox.runBD("show", id, "--json")
		if err != nil {
			t.Errorf("bead %q: bd show failed: %v", id, err)
			continue
		}
		// bd show --json returns an array, not a single object.
		var infos []beadStatus
		if err := json.Unmarshal([]byte(out), &infos); err != nil {
			t.Errorf("bead %q: unmarshal bd show: %v (raw: %.200s)", id, err, out)
			continue
		}
		if len(infos) == 0 {
			t.Errorf("bead %q: bd show returned empty array", id)
			continue
		}
		if infos[0].Status != want {
			t.Errorf("bead %q status: got %q, want %q", id, infos[0].Status, want)
		}
	}
}

// assertMergeTopology checks that at least one merge commit from a bead/ branch
// exists on the given specBranch (or on any branch if specBranch was already
// deleted by impl approve) after a bead→spec merge.
func assertMergeTopology(t testing.TB, sandbox *Sandbox, specBranch string) {
	t.Helper()
	// R7 (spec 120): specBranch is a dynamic operand reaching a git
	// spawn — guard with gitutil.RejectOptionLike, fail-fast t.Fatalf
	// before the spawn (SEC-5).
	if err := gitutil.RejectOptionLike(specBranch); err != nil {
		t.Fatalf("assertMergeTopology: %v", err)
	}
	// Try the specified branch first; fall back to --all if it no longer exists
	// (impl approve deletes the spec branch after merging).
	cmd := exec.Command("git", "log", "--merges", "--oneline", specBranch)
	cmd.Dir = sandbox.Root
	out, err := cmd.Output()
	if err != nil {
		// Branch may have been deleted by impl approve — search all refs.
		cmd = exec.Command("git", "log", "--merges", "--oneline", "--all")
		cmd.Dir = sandbox.Root
		out, err = cmd.Output()
		if err != nil {
			t.Errorf("git log --merges --all: %v", err)
			return
		}
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.Contains(line, "bead/") {
			return
		}
	}
	t.Errorf("no merge commit from a bead/ branch found on %s (or --all); merges: %s", specBranch, strings.TrimSpace(string(out)))
}

// assertMindspecMode runs `mindspec state show` in the sandbox and checks that
// the current mode matches the expected value.
func assertMindspecMode(t *testing.T, sandbox *Sandbox, expectedMode string) {
	t.Helper()
	out := mustRun(t, sandbox.Root, filepath.Join(sandbox.mindspecBinDir, "mindspec"), "state", "show")
	if !strings.Contains(out, expectedMode) {
		t.Errorf("expected mindspec mode %q, got output: %s", expectedMode, strings.TrimSpace(out))
	}
}

// assertCommitMessage checks that at least one commit in git log --oneline matches
// the given regex pattern (e.g. `impl\(bead-id\):`).
func assertCommitMessage(t testing.TB, sandbox *Sandbox, pattern string) {
	t.Helper()
	re, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("invalid pattern %q: %v", pattern, err)
	}
	cmd := exec.Command("git", "log", "--oneline", "--all")
	cmd.Dir = sandbox.Root
	out, err := cmd.Output()
	if err != nil {
		t.Errorf("git log --oneline: %v", err)
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if re.MatchString(line) {
			return
		}
	}
	t.Errorf("no commit message matching %q found in git log", pattern)
}
