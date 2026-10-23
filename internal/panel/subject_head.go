package panel

import (
	"errors"
	"fmt"
	"strings"
)

// ErrSubjectHead marks a failure to read a pinned spec or plan's live branch.
// Such failures block the gate rather than taking its missing-ref advisory path.
var ErrSubjectHead = errors.New("cannot resolve live spec branch")

// ResolveSubjectHead resolves the live panel subject. A spec or plan panel
// pinned to a commit tracks its spec branch, using exactly refs/heads/<branch>.
// A hex-looking target is read as a commit, not as a branch or tag name.
// Only confirmed branch absence permits fallback to the recorded target;
// an unreadable branch or an uncertain existence check returns ErrSubjectHead.
// Other targets and bead panels resolve their recorded target. The caller
// supplies branch composition and Git access to keep this package a leaf.
func ResolveSubjectHead(p Panel, workdir string, branchForSpec func(string) (string, error), revParse func(string, string) (string, error), refExists func(string, string) (bool, error)) (string, error) {
	target := strings.TrimSpace(p.Target)
	if target == "" {
		return "", fmt.Errorf("panel.json records no target ref")
	}
	if (p.Gate == "spec_approve" || p.Gate == "plan_approve") && looksLikeCommitSHA(target) {
		branch, err := branchForSpec(p.Spec)
		if err != nil {
			return "", fmt.Errorf("%w: %w", ErrSubjectHead, err)
		}
		ref := "refs/heads/" + branch
		exists, err := refExists(workdir, ref)
		if err != nil {
			return "", fmt.Errorf("%w %s: %w", ErrSubjectHead, ref, err)
		}
		if exists {
			sha, err := revParse(workdir, ref)
			if err != nil {
				return "", fmt.Errorf("%w %s: %w", ErrSubjectHead, ref, err)
			}
			if sha == "" {
				return "", fmt.Errorf("%w %s: empty commit", ErrSubjectHead, ref)
			}
			return sha, nil
		}
	}
	return revParse(workdir, target)
}

// Git accepts unambiguous hexadecimal object names with at least four digits.
func looksLikeCommitSHA(ref string) bool {
	if len(ref) < 4 || len(ref) > 64 {
		return false
	}
	for _, c := range ref {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}
