package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

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

	// Write default config. agent_hooks: false makes PreToolUse enforcement hooks
	// no-op — non-enforcement scenarios run from the main repo root (not a worktree),
	// so worktree-file and worktree-bash guards would incorrectly block tool calls.
	// Enforcement scenarios (HookBlocks*) can override this in their Setup func.
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

	// Set up Claude Code integration: CLAUDE.md, slash commands, and full hooks
	// (SessionStart runs mindspec instruct; PreToolUse hooks installed but no-op)
	if err := setupClaudeForSandbox(root); err != nil {
		t.Logf("warning: setup claude: %v", err)
	}

	// Initial commit (includes .mindspec/ and Claude Code setup files)
	mustRun(t, root, "git", "add", "-A")
	mustRun(t, root, "git", "commit", "-m", "initial commit")

	// Initialize beads in sandbox mode (no auto-sync, no git hooks).
	// Add .beads/ to .gitignore first — dolt server writes runtime files
	// (dolt-server.activity, dolt-server.port) that would make the worktree
	// appear dirty to mindspec complete's clean-tree check.
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".beads/\n.harness/\n.mindspec/session.json\n.mindspec/focus\n.mindspec/current-spec.json\n"), 0o644); err != nil {
		t.Fatalf("writing .gitignore: %v", err)
	}
	mustRun(t, root, "git", "add", ".gitignore")
	mustRun(t, root, "git", "commit", "-m", "add gitignore")
	initBeads(t, root)

	// Resolve project bin/ directory for mindspec binary
	binDir := projectBinDir()

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

	return &Sandbox{
		Root:           root,
		ShimBinDir:     shimDir,
		EventLogPath:   logPath,
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

// WriteFocus writes a focus file to the sandbox .mindspec/focus.
func (s *Sandbox) WriteFocus(content string) {
	s.t.Helper()
	s.WriteFile(".mindspec/focus", content)
}

// WriteLifecycle writes a lifecycle.yaml to the given spec directory.
func (s *Sandbox) WriteLifecycle(specID, content string) {
	s.t.Helper()
	relPath := fmt.Sprintf(".mindspec/docs/specs/%s/lifecycle.yaml", specID)
	s.WriteFile(relPath, content)
}

// initBeads runs bd init in sandbox mode within the given root directory.
// Uses --server-port 0 so dolt picks a random free port (avoids collisions
// between parallel test sandboxes and the main project's dolt server).
// Kills orphan dolt servers first to avoid "too many dolt sql-server" errors.
func initBeads(t *testing.T, root string) {
	t.Helper()
	bdPath, err := exec.LookPath("bd")
	if err != nil {
		t.Logf("warning: bd not found, skipping beads init")
		return
	}

	// Kill orphan dolt servers that may have leaked from previous test runs.
	killCmd := exec.Command(bdPath, "dolt", "killall")
	killCmd.Dir = root
	_ = killCmd.Run() // best-effort

	cmd := exec.Command(bdPath, "init", "--sandbox", "--skip-hooks", "-q", "--server-port", "0")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("warning: bd init: %v\n%s", err, out)
	}
}

// bdCreateIssue is the JSON response from bd create --json.
type bdCreateIssue struct {
	ID string `json:"id"`
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
// setup.RunClaude(). This gives the agent the full SessionStart hook (runs
// `mindspec instruct`) and PreToolUse hooks. The PreToolUse enforcement hooks
// are no-ops because config.yaml has agent_hooks: false — non-enforcement
// scenarios work from the main repo root (not a worktree), so worktree/file
// guards would incorrectly block tool calls.
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
