package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mindspec/mindspec/internal/setup"
)

// Sandbox is an isolated git repository with recording shims installed,
// ready for running agent sessions or deterministic lifecycle tests.
type Sandbox struct {
	// Root is the path to the sandbox git repo.
	Root string
	// ShimBinDir is the directory containing recording shim scripts.
	ShimBinDir string
	// EventLogPath is the JSONL file where shims log command invocations.
	EventLogPath string
	// DoltPort is the sandbox's dolt server port (0 if beads init failed).
	DoltPort int
	// mindspecBinDir is the project's bin/ directory (contains mindspec binary).
	mindspecBinDir string

	t *testing.T
}

// NewSandbox creates a fresh git repo in t.TempDir() with .mindspec/,
// config.yaml, an initial commit, and recording shims installed.
func NewSandbox(t *testing.T) *Sandbox {
	t.Helper()

	root := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("creating sandbox root: %v", err)
	}

	// Init git repo
	mustRun(t, root, "git", "init")
	mustRun(t, root, "git", "config", "user.email", "test@mindspec.dev")
	mustRun(t, root, "git", "config", "user.name", "MindSpec Test")

	// Create .mindspec structure
	mindspecDir := filepath.Join(root, ".mindspec")
	docsDir := filepath.Join(mindspecDir, "docs")
	for _, dir := range []string{
		mindspecDir,
		docsDir,
		filepath.Join(docsDir, "specs"),
		filepath.Join(docsDir, "adr"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("creating %s: %v", dir, err)
		}
	}

	// Write default config.
	configContent := `protected_branches: [main]
merge_strategy: direct
worktree_root: .worktrees
enforcement:
  pre_commit_hook: true
  cli_guards: true
  agent_hooks: false
`
	if err := os.WriteFile(filepath.Join(mindspecDir, "config.yaml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	// Resolve project bin/ directory for mindspec binary — needed BEFORE initial
	// commit so the pre-commit hook shim can find `mindspec hook pre-commit`.
	binDir := projectBinDir()
	if binDir == "" && testing.Short() {
		t.Skip("skipping sandbox test: no mindspec binary (run make build)")
	}

	// Set up Claude Code integration: CLAUDE.md, slash commands, and hooks
	// (SessionStart runs mindspec instruct)
	if err := setupClaudeForSandbox(root); err != nil {
		t.Logf("warning: setup claude: %v", err)
	}

	// Initial commit (includes .mindspec/ and Claude Code setup files).
	// Must have mindspec in PATH for the pre-commit hook shim.
	mustRunWithBin(t, root, binDir, "git", "add", "-A")
	mustRunWithBin(t, root, binDir, "git", "commit", "-m", "initial commit")

	// Initialize beads in sandbox mode (no auto-sync, no git hooks).
	// Add .beads/ to .gitignore first — dolt server writes runtime files
	// (dolt-server.activity, dolt-server.port) that would make the worktree
	// appear dirty to mindspec complete's clean-tree check.
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".beads/\n.harness/\n.worktrees/\n.mindspec/session.json\n.mindspec/focus\n.mindspec/current-spec.json\n"), 0o644); err != nil {
		t.Fatalf("writing .gitignore: %v", err)
	}
	mustRun(t, root, "git", "add", ".gitignore")
	mustRun(t, root, "git", "commit", "-m", "add gitignore")
	doltPort := initBeads(t, root)

	// Set up recording shims — temporarily extend PATH so mindspec shim is created
	shimDir := filepath.Join(root, ".harness", "bin")
	logPath := filepath.Join(root, ".harness", "events.jsonl")
	if err := os.MkdirAll(filepath.Join(root, ".harness"), 0o755); err != nil {
		t.Fatalf("creating harness dir: %v", err)
	}

	if binDir != "" {
		origPath := os.Getenv("PATH")
		os.Setenv("PATH", binDir+":"+origPath)
		defer os.Setenv("PATH", origPath)
	}

	if err := InstallShims(shimDir, logPath); err != nil {
		// Non-fatal — some binaries may not be in PATH during unit tests
		t.Logf("warning: shim install: %v", err)
	}

	// Overwrite the bd shim with a CWD-pinned version that forces .beads/
	// resolution to the sandbox root, preventing leakage to the host project.
	if err := WritePinnedShim(shimDir, logPath, "bd", root); err != nil {
		t.Logf("warning: pinned bd shim: %v", err)
	}

	return &Sandbox{
		Root:           root,
		ShimBinDir:     shimDir,
		EventLogPath:   logPath,
		DoltPort:       doltPort,
		mindspecBinDir: binDir,
		t:              t,
	}
}

// Run executes a command in the sandbox with recording shims in PATH.
func (s *Sandbox) Run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = s.Root
	cmd.Env = s.Env()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ReadEvents reads the recorded command events from the JSONL log.
func (s *Sandbox) ReadEvents() []ActionEvent {
	s.t.Helper()
	if _, err := os.Stat(s.EventLogPath); os.IsNotExist(err) {
		return nil
	}
	events, err := ParseEvents(s.EventLogPath)
	if err != nil {
		s.t.Fatalf("reading events: %v", err)
	}
	return events
}

// WriteFile creates a file at the given relative path within the sandbox.
func (s *Sandbox) WriteFile(relPath, content string) {
	s.t.Helper()
	absPath := filepath.Join(s.Root, relPath)
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		s.t.Fatalf("creating dir for %s: %v", relPath, err)
	}
	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		s.t.Fatalf("writing %s: %v", relPath, err)
	}
}

// ReadFile reads a file at the given relative path within the sandbox.
func (s *Sandbox) ReadFile(relPath string) string {
	s.t.Helper()
	data, err := os.ReadFile(filepath.Join(s.Root, relPath))
	if err != nil {
		s.t.Fatalf("reading %s: %v", relPath, err)
	}
	return string(data)
}

// FileExists checks whether a file exists at the given relative path.
func (s *Sandbox) FileExists(relPath string) bool {
	_, err := os.Stat(filepath.Join(s.Root, relPath))
	return err == nil
}

// Commit stages all changes and commits with the given message.
func (s *Sandbox) Commit(msg string) {
	s.t.Helper()
	mustRun(s.t, s.Root, "git", "add", "-A")
	// Scenario setup may intentionally place focus in non-idle modes while still
	// committing bootstrap files on main. Use the documented escape hatch so
	// setup remains deterministic without weakening runtime enforcement.
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", msg)
	cmd.Dir = s.Root
	cmd.Env = append(os.Environ(), "MINDSPEC_ALLOW_MAIN=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		s.t.Fatalf("git commit --allow-empty -m %s failed: %v\n%s", msg, err, out)
	}
}

func mustRun(t *testing.T, dir string, name string, args ...string) string { //nolint:unparam // name kept for call-site clarity
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return string(out)
}

// mustRunWithBin is like mustRun but prepends binDir to PATH so that
// git hooks (e.g. pre-commit shim) can resolve the mindspec binary.
func mustRunWithBin(t *testing.T, dir, binDir string, name string, args ...string) { //nolint:unparam // name kept for call-site clarity
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if binDir != "" {
		cmd.Env = append(os.Environ(), "PATH="+binDir+":"+os.Getenv("PATH"))
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}

// Env returns the environment variables for running commands in the sandbox,
// with recording shims and the mindspec bin dir prepended to PATH.
func (s *Sandbox) Env() []string {
	// Build a single PATH: shimDir (first, for recording) + mindspecBinDir + original PATH.
	// We must avoid duplicate PATH entries since Go exec uses the last one.
	origPath := os.Getenv("PATH")
	newPath := s.ShimBinDir
	if s.mindspecBinDir != "" {
		newPath += ":" + s.mindspecBinDir
	}
	newPath += ":" + origPath

	// Start with os.Environ() minus PATH, then add our unified PATH.
	var env []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "PATH=") {
			env = append(env, e)
		}
	}
	env = append(env, "PATH="+newPath)
	return env
}

// GitBranch returns the current git branch name.
func (s *Sandbox) GitBranch() string {
	s.t.Helper()
	out := mustRun(s.t, s.Root, "git", "rev-parse", "--abbrev-ref", "HEAD")
	return strings.TrimSpace(out)
}

// BranchExists checks whether a git branch exists.
func (s *Sandbox) BranchExists(branch string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", branch)
	cmd.Dir = s.Root
	return cmd.Run() == nil
}

// WorktreeExists checks whether a worktree directory exists.
func (s *Sandbox) WorktreeExists(name string) bool {
	return s.FileExists(filepath.Join(".worktrees", name))
}

// ListBranches returns branch names matching the given prefix (e.g. "spec/", "bead/").
func (s *Sandbox) ListBranches(prefix string) []string {
	cmd := exec.Command("git", "branch", "--list", prefix+"*")
	cmd.Dir = s.Root
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		b := strings.TrimSpace(strings.TrimPrefix(line, "* "))
		if b != "" {
			branches = append(branches, b)
		}
	}
	return branches
}

// GitStatusClean returns true if the working tree has no uncommitted changes.
func (s *Sandbox) GitStatusClean() bool {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = s.Root
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == ""
}

// ListWorktrees returns linked git worktrees (excluding main) by name.
func (s *Sandbox) ListWorktrees() []string {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = s.Root
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "worktree ") {
			paths = append(paths, strings.TrimPrefix(line, "worktree "))
		}
	}

	var names []string
	rootClean := canonicalPath(s.Root)
	for _, p := range paths {
		p = canonicalPath(p)
		if p == rootClean {
			continue // skip main worktree
		}
		names = append(names, filepath.Base(p))
	}
	return names
}

func canonicalPath(path string) string {
	clean := filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(clean)
	if err != nil {
		return clean
	}
	return filepath.Clean(resolved)
}

// WriteFocus is deprecated. Focus files are no longer used (ADR-0023).
// Kept as a no-op for backward compatibility with scenario setup code.
func (s *Sandbox) WriteFocus(content string) {
	s.t.Helper()
	// No-op: focus files are no longer written (ADR-0023).
	// State is derived from beads.
}

// initBeads runs bd init in sandbox mode within the given root directory.
// Uses --server-port 0 so dolt picks a random free port (avoids collisions
// between parallel test sandboxes and the main project's dolt server).
// Returns the dolt server port (0 if init failed or port unreadable).
// Registers t.Cleanup() to stop the sandbox's dolt server on test teardown.
func initBeads(t *testing.T, root string) int {
	t.Helper()
	bdPath, err := exec.LookPath("bd")
	if err != nil {
		t.Logf("warning: bd not found, skipping beads init")
		return 0
	}

	// NOTE: We intentionally do NOT call `bd dolt killall` here.
	// It kills ALL dolt servers system-wide, including the host project's.
	// Each sandbox gets its own server on a random port via --server-port 0.

	cmd := exec.Command(bdPath, "init", "--sandbox", "--skip-hooks", "-q", "--server-port", "0")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("warning: bd init: %v\n%s", err, out)
		return 0
	}

	// Read the dolt server port assigned to this sandbox.
	port := readDoltPort(root)

	// Register cleanup to stop this sandbox's dolt server on test teardown.
	t.Cleanup(func() {
		stopSandboxDolt(root)
	})

	return port
}

// readDoltPort reads the dolt server port from .beads/dolt-server.port.
func readDoltPort(root string) int {
	data, err := os.ReadFile(filepath.Join(root, ".beads", "dolt-server.port"))
	if err != nil {
		return 0
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return port
}

// stopSandboxDolt stops the dolt server for a sandbox and waits for it to exit.
func stopSandboxDolt(root string) {
	// Read PID before stopping — we need it to wait for exit.
	pidData, _ := os.ReadFile(filepath.Join(root, ".beads", "dolt-server.pid"))
	pid, _ := strconv.Atoi(strings.TrimSpace(string(pidData)))

	// Try graceful stop via bd dolt stop.
	bdPath, err := exec.LookPath("bd")
	if err != nil {
		return
	}
	cmd := exec.Command(bdPath, "dolt", "stop")
	cmd.Dir = root
	_ = cmd.Run()

	// Wait for the process to actually exit so t.TempDir() cleanup can
	// remove the .beads/dolt directory without "directory not empty" errors.
	if pid > 0 {
		waitForProcessExit(pid, 5)
	}
}

// waitForProcessExit polls until the given PID no longer exists, up to maxSeconds.
func waitForProcessExit(pid, maxSeconds int) {
	for i := 0; i < maxSeconds*10; i++ {
		proc, err := os.FindProcess(pid)
		if err != nil {
			return
		}
		// On Unix, FindProcess always succeeds. Use Signal(0) to check existence.
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			return // process gone
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// bdCreateIssue is the JSON response from bd create --json.
type bdCreateIssue struct {
	ID string `json:"id"`
}

// CreateSpecEpic creates a lifecycle epic for a spec in the [SPEC NNN-slug] format
// with proper metadata for ADR-0023 phase derivation. Returns the epic ID.
func (s *Sandbox) CreateSpecEpic(specID string) string {
	s.t.Helper()

	// Parse spec num and title from specID (e.g., "001-calc" → 1, "calc")
	dashIdx := strings.Index(specID, "-")
	if dashIdx < 0 {
		s.t.Fatalf("invalid specID %q: expected NNN-slug format", specID)
	}
	numStr := specID[:dashIdx]
	slug := specID[dashIdx+1:]
	var num int
	if _, err := fmt.Sscanf(numStr, "%d", &num); err != nil {
		s.t.Fatalf("invalid spec number in %q: %v", specID, err)
	}

	epicTitle := fmt.Sprintf("[SPEC %s] %s", specID, slug)
	metadata := fmt.Sprintf(`{"spec_num":%d,"spec_title":"%s"}`, num, slug)

	args := []string{"create", "--title", epicTitle, "--type=epic", "--metadata", metadata, "--priority", "2", "--json"}
	out, err := s.runBD(args...)
	if err != nil {
		s.t.Fatalf("bd create spec epic %q: %v\n%s", specID, err, out)
	}
	var issue bdCreateIssue
	if err := json.Unmarshal([]byte(out), &issue); err != nil {
		s.t.Fatalf("parsing bd create output: %v\n%s", err, out)
	}
	return issue.ID
}

// CreateBead creates a beads issue in the sandbox and returns its ID.
// issueType is "epic" or "task". parentID is optional.
func (s *Sandbox) CreateBead(title, issueType, parentID string) string {
	s.t.Helper()
	args := []string{"create", "--title", title, "--type", issueType, "--priority", "2", "--json"}
	if parentID != "" {
		args = append(args, "--parent", parentID)
	}
	out, err := s.runBD(args...)
	if err != nil {
		s.t.Fatalf("bd create %q: %v\n%s", title, err, out)
	}
	var issue bdCreateIssue
	if err := json.Unmarshal([]byte(out), &issue); err != nil {
		s.t.Fatalf("parsing bd create output: %v\n%s", err, out)
	}
	return issue.ID
}

// ClaimBead sets a beads issue to in_progress status.
func (s *Sandbox) ClaimBead(beadID string) {
	s.t.Helper()
	out, err := s.runBD("update", beadID, "--status=in_progress")
	if err != nil {
		s.t.Fatalf("bd update %s: %v\n%s", beadID, err, out)
	}
}

// runBDMust executes a bd command in the sandbox, fataling on error.
func (s *Sandbox) runBDMust(args ...string) {
	s.t.Helper()
	out, err := s.runBD(args...)
	if err != nil {
		s.t.Fatalf("bd %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// runBD executes a bd command in the sandbox directory, returning stdout only.
func (s *Sandbox) runBD(args ...string) (string, error) {
	bdPath, err := exec.LookPath("bd")
	if err != nil {
		return "", fmt.Errorf("bd not found: %w", err)
	}
	cmd := exec.Command(bdPath, args...)
	cmd.Dir = s.Root
	out, err := cmd.Output() // stdout only (bd emits warnings to stderr)
	return string(out), err
}

// setupClaudeForSandbox installs CLAUDE.md, slash commands, and hooks via
// setup.RunClaude(). This gives the agent the SessionStart hook (runs
// `mindspec instruct`) and installs the pre-commit git hook shim.
func setupClaudeForSandbox(root string) error {
	_, err := setup.RunClaude(root, false)
	return err
}

// projectBinDir finds the mindspec project's bin/ directory by walking up
// from CWD to find go.mod, then checking for bin/mindspec.
func projectBinDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			binDir := filepath.Join(dir, "bin")
			if _, err := os.Stat(filepath.Join(binDir, "mindspec")); err == nil {
				return binDir
			}
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
