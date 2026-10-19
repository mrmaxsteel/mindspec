package gitutil

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// stageExplicitPaths preserves updates to indexed files beneath ignored
// directories without force-adding any untracked ignored files (spec 129 R8).
// The caller retains the audited git process boundary and validates paths.
func stageExplicitPaths(paths []string, run func(...string) ([]byte, error)) error {
	add := func(args ...string) error {
		if out, err := run(args...); err != nil {
			return fmt.Errorf("git add: %s", strings.TrimSpace(string(out)))
		}
		return nil
	}
	for _, path := range paths {
		if out, err := run("ls-files", "--cached", "--error-unmatch", "--", path); err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 {
				return fmt.Errorf("git ls-files: %s: %w", strings.TrimSpace(string(out)), err)
			}
			// Untracked explicit paths retain ordinary git-add ignore checks.
			if err := add("add", "--", path); err != nil {
				return err
			}
			continue
		}
		if err := add("add", "-u", "--", path); err != nil {
			return err
		}
		// A directory path can also contain new, non-ignored files. Stage
		// those separately so the tracked-file leg does not drop additions.
		out, err := run("ls-files", "--others", "--exclude-standard", "-z", "--", path)
		if err != nil {
			return fmt.Errorf("git ls-files untracked: %w", err)
		}
		if len(out) > 0 {
			untracked := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
			args := append([]string{"--literal-pathspecs", "add", "--"}, untracked...)
			if err := add(args...); err != nil {
				return err
			}
		}
	}
	return nil
}
