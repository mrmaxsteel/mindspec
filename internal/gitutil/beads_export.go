package gitutil

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// BeadsExportPublished reports whether the checkout at workdir publishes
// the beads export (.beads/issues.jsonl) through git. workdir must be the
// root of the checkout that a caller is about to commit in.
//
//   - Tracked in the index: the repo publishes the export.
//   - Untracked and git-ignored: the repo does not publish it. The beads
//     database is the store of record, and a commit must not try to stage
//     the file (git refuses an explicit add of an ignored path).
//   - Untracked and not ignored: the repo publishes it, so the first
//     export of a fresh repo still gets committed.
//
// Any other git failure is returned as an error.
func BeadsExportPublished(workdir string) (bool, error) {
	tracked, err := gitExitProbe(workdir, "ls-files", "--error-unmatch", "--", trackerJSONLPath)
	if err != nil {
		return false, err
	}
	if tracked {
		return true, nil
	}
	ignored, err := gitExitProbe(workdir, "check-ignore", "--quiet", "--", trackerJSONLPath)
	if err != nil {
		return false, err
	}
	return !ignored, nil
}

// gitExitProbe runs a git command whose exit status 0 means true and 1
// means false. Any other result is an error that carries git's stderr.
func gitExitProbe(workdir string, args ...string) (bool, error) {
	cmd := execCommand("git", gitArgs(workdir, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
}
