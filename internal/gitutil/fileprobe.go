package gitutil

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrRefAbsent means the probed local branch does not exist.
var ErrRefAbsent = errors.New("ref absent")

// ErrPathNotAtRef means the branch exists but does not carry the path.
var ErrPathNotAtRef = errors.New("path not at ref")

// FileAtRefTyped reads a local branch blob, distinguishing absence from git failures.
func FileAtRefTyped(workdir, ref, path string) ([]byte, error) {
	if err := RejectOptionLike(ref); err != nil {
		return nil, err
	}
	if err := RejectOptionLike(path); err != nil {
		return nil, err
	}
	out, err := execCommand("git", "-C", workdir, "rev-parse", "--verify", "--quiet", "refs/heads/"+ref).CombinedOutput()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 && len(out) == 0 {
			return nil, ErrRefAbsent
		}
		return nil, fmt.Errorf("probing branch %s: %w: %s", ref, err, strings.TrimSpace(string(out)))
	}
	out, err = execCommand("git", "-C", workdir, "ls-tree", "refs/heads/"+ref, "--", path).Output()
	if err != nil {
		return nil, fmt.Errorf("probing path %s at %s: %w: %s", path, ref, err, strings.TrimSpace(string(out)))
	}
	if len(out) == 0 {
		return nil, ErrPathNotAtRef
	}
	out, err = execCommand("git", "-C", workdir, "show", "refs/heads/"+ref+":"+path).Output()
	if err != nil {
		return nil, fmt.Errorf("reading %s:%s: %w", ref, path, err)
	}
	return out, nil
}
