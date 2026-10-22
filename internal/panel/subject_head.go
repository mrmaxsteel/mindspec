package panel

import (
	"fmt"
	"strings"
)

// ResolveSubjectHead resolves the live subject of a non-bead panel. A spec or
// plan review pinned to an object name follows its spec branch when it exists.
// Moving refs keep their recorded target. Only a missing branch permits fallback.
// The caller supplies spec branch composition to keep this package a leaf.
func ResolveSubjectHead(p Panel, workdir string, branchForSpec func(string) (string, error), revParse func(string, string) (string, error), isRefNotFound func(error) bool) (string, error) {
	target := strings.TrimSpace(p.Target)
	if target == "" {
		return "", fmt.Errorf("panel.json records no target ref")
	}
	if (p.Gate == "spec_approve" || p.Gate == "plan_approve") && looksLikeCommitSHA(target) {
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
