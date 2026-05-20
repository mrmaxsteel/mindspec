package bead

import (
	"strings"
)

// GitUserEmail returns a best-effort `git config --get user.email` value
// or "unknown" if git is unavailable or unconfigured. It lives in
// `internal/bead` because enforcement packages
// (`internal/{validate,approve,complete,state,phase}`) are forbidden by
// the ADR-0030 boundary doctrine (and the
// `TestEnforcementHasNoGitLeaks` invariant at
// `internal/lint/boundary_test.go`) from importing `os/exec` — this
// helper provides those callers with a safe identity read for skew-
// override audit metadata (spec 086 Bead 3).
//
// Routes through the package-private `execCommand` var so tests can
// stub it without spawning a real git subprocess.
func GitUserEmail() string {
	cmd := execCommand("git", "config", "--get", "user.email")
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	email := strings.TrimSpace(string(out))
	if email == "" {
		return "unknown"
	}
	return email
}
