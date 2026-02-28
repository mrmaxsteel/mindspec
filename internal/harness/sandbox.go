package harness

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

	// Write default config
	configContent := `protected_branches: [main]
merge_strategy: direct
worktree_root: .worktrees
enforcement:
  pre_commit_hook: true
  cli_guards: true
  agent_hooks: true
`
	if err := os.WriteFile(filepath.Join(mindspecDir, "config.yaml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	// Initial commit
	mustRun(t, root, "git", "add", "-A")
	mustRun(t, root, "git", "commit", "-m", "initial commit")

	// Set up recording shims
	shimDir := filepath.Join(root, ".harness", "bin")
	logPath := filepath.Join(root, ".harness", "events.jsonl")
	if err := os.MkdirAll(filepath.Join(root, ".harness"), 0o755); err != nil {
		t.Fatalf("creating harness dir: %v", err)
	}

	if err := InstallShims(shimDir, logPath); err != nil {
		// Non-fatal — some binaries may not be in PATH during unit tests
		t.Logf("warning: shim install: %v", err)
	}

	return &Sandbox{
		Root:         root,
		ShimBinDir:   shimDir,
		EventLogPath: logPath,
		t:            t,
	}
}

// Run executes a command in the sandbox with recording shims in PATH.
func (s *Sandbox) Run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = s.Root
	cmd.Env = append(os.Environ(), ShimEnv(s.ShimBinDir)...)
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
	mustRun(s.t, s.Root, "git", "commit", "--allow-empty", "-m", msg)
}

func mustRun(t *testing.T, dir string, name string, args ...string) string {
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
// with recording shims prepended to PATH.
func (s *Sandbox) Env() []string {
	return append(os.Environ(), ShimEnv(s.ShimBinDir)...)
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
