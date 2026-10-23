package panel

import (
	"fmt"
	"strings"
)

// ResolveSubjectHead resolves the live subject of a non-bead panel. A spec or
// plan review pinned to an object name follows its spec branch when it exists.
// Moving refs keep their recorded target. Only a missing branch permits fallback.
// The caller supplies spec branch composition and Git resolution to keep this
// package a leaf. symbolicFullName returns Git's selected ref, or empty for an
// object pin, including a full SHA that is also a branch name.
func ResolveSubjectHead(p Panel, workdir string, branchForSpec func(string) (string, error), revParse func(string, string) (string, error), symbolicFullName func(string, string) (string, error), isRefNotFound func(error) bool) (string, error) {
	target := strings.TrimSpace(p.Target)
	if target == "" {
		return "", fmt.Errorf("panel.json records no target ref")
	}
	if (p.Gate == "spec_approve" || p.Gate == "plan_approve") && looksLikeCommitSHA(target) {
		// Git owns ref precedence, including tag/branch collisions and full
		// object IDs. Do not guess namespaces for hexadecimal names here.
		name, err := symbolicFullName(workdir, target)
		if err != nil && !isRefNotFound(err) {
			return "", err
		}
		if err == nil && name != "" {
			return revParse(workdir, name)
		}
		// A missing (including ambiguous) object pin still names the spec's
		// subject. Only the spec branch's absence permits target fallback.
		branch, err := branchForSpec(p.Spec)
		if err != nil {
			return "", err
		}
		sha, err := revParse(workdir, branch)
		if err == nil {
			return sha, nil
		}
		if !isRefNotFound(err) {
			return "", err
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
