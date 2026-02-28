package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// checkGit runs git-related health checks.
func checkGit(r *Report, root string) {
	checkRuntimeFilesTracked(r, root)
}

// runtimeFiles lists MindSpec local runtime files that should be gitignored.
var runtimeFiles = []string{
	".mindspec/session.json",
	".mindspec/focus",
}

// checkRuntimeFilesTracked detects MindSpec runtime files tracked by git.
// These are local runtime files (ADR-0015) and should be gitignored.
func checkRuntimeFilesTracked(r *Report, root string) {
	for _, file := range runtimeFiles {
		cmd := exec.Command("git", "ls-files", "--error-unmatch", file)
		cmd.Dir = root
		if err := cmd.Run(); err != nil {
			// Not tracked — good.
			r.Checks = append(r.Checks, Check{
				Name:    file + " git tracking",
				Status:  OK,
				Message: "not tracked by git",
			})
			continue
		}

		// Still tracked — needs fix.
		trackedFile := file
		r.Checks = append(r.Checks, Check{
			Name:    file + " git tracking",
			Status:  Error,
			Message: "tracked by git — runtime file should be gitignored (run with --fix to auto-repair)",
			FixFunc: func() error {
				return untrackRuntimeFile(root, trackedFile)
			},
		})
	}
}

// untrackRuntimeFile removes a runtime file from git index and ensures
// it is listed in .gitignore.
func untrackRuntimeFile(root, file string) error {
	// git rm --cached (keeps file on disk)
	cmd := exec.Command("git", "rm", "--cached", file)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		return &fixError{op: "git rm --cached " + file, detail: strings.TrimSpace(string(out))}
	}

	// Ensure .gitignore has the entry
	cmd = exec.Command("git", "check-ignore", "-q", file)
	cmd.Dir = root
	if err := cmd.Run(); err == nil {
		return nil // already gitignored
	}

	// Append to .gitignore
	return appendGitignoreEntry(root, file)
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
	content += "\n# MindSpec local runtime file (not version-controlled)\n" + entry + "\n"
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
