package doctor

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// runtimePatterns are Beads runtime filenames that should not be git-tracked.
var runtimePatterns = map[string]bool{
	"bd.sock":         true,
	"daemon.lock":     true,
	"daemon.log":      true,
	"daemon.pid":      true,
	"sync-state.json": true,
	"last-touched":    true,
	".local_version":  true,
	"db.sqlite":       true,
	"bd.db":           true,
	"redirect":        true,
	".sync.lock":      true,
}

// runtimeExtensions are file extensions for Beads runtime artifacts.
var runtimeExtensions = []string{".db", ".db-wal", ".db-shm", ".db-journal"}

// durableFiles are expected Beads durable state files.
var durableFiles = []string{"issues.jsonl", "config.yaml", "metadata.json"}

func checkBeads(r *Report, root string) {
	beadsDir := filepath.Join(root, ".beads")

	if !dirExists(beadsDir) {
		r.Checks = append(r.Checks, Check{
			Name:    "Beads",
			Status:  Missing,
			Message: ".beads/ directory not found — run `beads init`",
		})
		return
	}

	r.Checks = append(r.Checks, Check{Name: "Beads", Status: OK, Message: ".beads/ directory exists"})

	// Check durable state files
	var found []string
	for _, f := range durableFiles {
		if fileExists(filepath.Join(beadsDir, f)) {
			found = append(found, f)
		}
	}
	if len(found) > 0 {
		r.Checks = append(r.Checks, Check{
			Name:    "Beads durable state",
			Status:  OK,
			Message: fmt.Sprintf("(%s)", strings.Join(found, ", ")),
		})
	} else {
		r.Checks = append(r.Checks, Check{
			Name:    "Beads durable state",
			Status:  Missing,
			Message: "no durable state files found (issues.jsonl, config.yaml, metadata.json)",
		})
	}

	// Check for git-tracked runtime artifacts
	checkTrackedRuntime(r, root)
}

func checkTrackedRuntime(r *Report, root string) {
	cmd := exec.Command("git", "ls-files", ".beads/")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		// git not available or not a git repo — skip with warning
		r.Checks = append(r.Checks, Check{
			Name:    "Beads runtime artifacts",
			Status:  Warn,
			Message: "could not run git ls-files (git not available or not a repo)",
		})
		return
	}

	tracked := strings.TrimSpace(string(out))
	if tracked == "" {
		r.Checks = append(r.Checks, Check{
			Name:    "Beads runtime artifacts",
			Status:  OK,
			Message: "none tracked by git",
		})
		return
	}

	var violations []string
	for _, line := range strings.Split(tracked, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		filename := filepath.Base(line)
		if isRuntimeArtifact(filename) {
			violations = append(violations, line)
		}
	}

	if len(violations) > 0 {
		msg := fmt.Sprintf("tracked by git: %s — add to .beads/.gitignore and run `git rm --cached <file>`",
			strings.Join(violations, ", "))
		r.Checks = append(r.Checks, Check{
			Name:    "Beads runtime artifacts",
			Status:  Error,
			Message: msg,
		})
	} else {
		r.Checks = append(r.Checks, Check{
			Name:    "Beads runtime artifacts",
			Status:  OK,
			Message: "none tracked by git",
		})
	}
}

func isRuntimeArtifact(filename string) bool {
	if runtimePatterns[filename] {
		return true
	}
	for _, ext := range runtimeExtensions {
		if strings.HasSuffix(filename, ext) {
			return true
		}
	}
	return false
}
