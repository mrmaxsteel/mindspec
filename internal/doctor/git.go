package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// checkGit runs git-related health checks.
func checkGit(r *Report, root string) {
	checkStateJsonTracked(r, root)
}

// checkStateJsonTracked detects .mindspec/state.json tracked by git.
// state.json is a local runtime cursor (ADR-0015) and should be gitignored.
// Projects created before this change need a one-time `git rm --cached`.
func checkStateJsonTracked(r *Report, root string) {
	cmd := exec.Command("git", "ls-files", "--error-unmatch", ".mindspec/state.json")
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		// Not tracked — good.
		r.Checks = append(r.Checks, Check{
			Name:    "state.json git tracking",
			Status:  OK,
			Message: "not tracked by git",
		})
		return
	}

	// Still tracked — needs fix.
	r.Checks = append(r.Checks, Check{
		Name:    "state.json git tracking",
		Status:  Error,
		Message: "tracked by git — state.json is a local runtime cursor and should be gitignored (run with --fix to auto-repair)",
		FixFunc: func() error {
			return untrackStateJson(root)
		},
	})
}

// untrackStateJson removes .mindspec/state.json from git index and ensures
// it is listed in .gitignore.
func untrackStateJson(root string) error {
	// git rm --cached (keeps file on disk)
	cmd := exec.Command("git", "rm", "--cached", ".mindspec/state.json")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		return &fixError{op: "git rm --cached", detail: strings.TrimSpace(string(out))}
	}

	// Ensure .gitignore has the entry
	cmd = exec.Command("git", "check-ignore", "-q", ".mindspec/state.json")
	cmd.Dir = root
	if err := cmd.Run(); err == nil {
		return nil // already gitignored
	}

	// Append to .gitignore
	return appendGitignoreEntry(root, ".mindspec/state.json")
}

// appendGitignoreEntry appends an entry to .gitignore if not already present.
func appendGitignoreEntry(root, entry string) error {
	p := filepath.Join(root, ".gitignore")
	data, _ := os.ReadFile(p)
	content := string(data)

	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == entry {
			return nil
		}
	}

	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "\n# MindSpec local state (runtime cursor, not version-controlled)\n" + entry + "\n"
	return os.WriteFile(p, []byte(content), 0o644)
}

type fixError struct {
	op     string
	detail string
}

func (e *fixError) Error() string {
	if e.detail != "" {
		return e.op + ": " + e.detail
	}
	return e.op + " failed"
}
