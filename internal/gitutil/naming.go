package gitutil

import (
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// FindNamingMerges finds two-parent merges that name this bead on the spec
// branch's first-parent chain. It does not check the bead tip or content.
func FindNamingMerges(root, specBranch, beadID string) ([]MergeCommit, error) {
	branch, err := workspace.BeadBranch(beadID)
	if err != nil {
		return nil, err
	}
	merges, err := FirstParentMerges(root, specBranch)
	if err != nil {
		return nil, err
	}
	var found []MergeCommit
	for _, m := range merges {
		name, namesBead := ParseMergeSubjectBeadBranch(m.Subject)
		if len(m.Parents) == 2 && namesBead && name == branch {
			found = append(found, m)
		}
	}
	return found, nil
}

// ParseMergeSubjectBeadBranch nominates a bead branch from a merge subject.
func ParseMergeSubjectBeadBranch(subject string) (branch string, present bool) {
	const beadPrefix = "bead/"
	if rest, ok := strings.CutPrefix(subject, "Merge "); ok {
		// `Merge bead/<id>` (and any trailing text after the token).
		if strings.HasPrefix(rest, beadPrefix) {
			return cutBranchToken(rest), true
		}
		// `Merge branch 'bead/<id>'` / `Merge branch 'bead/<id>' into …`.
		if quoted, ok := strings.CutPrefix(rest, "branch '"); ok {
			if end := strings.IndexByte(quoted, '\''); end > 0 {
				if tok := quoted[:end]; strings.HasPrefix(tok, beadPrefix) {
					return tok, true
				}
			}
		}
	}
	// Conservative fallback (G2-2): ANY bead/… token anywhere in the
	// subject — an unrecognized format still NOMINATES the bead it names;
	// it is never read as "names no bead".
	if idx := strings.Index(subject, beadPrefix); idx >= 0 {
		return cutBranchToken(subject[idx:]), true
	}
	return "", false
}

// cutBranchToken truncates s at the first byte that cannot plausibly
// belong to a branch-name token (whitespace, quotes, closing brackets,
// common separators, control bytes). It never trims characters INSIDE a
// valid bead-branch name, and over-capture is safe by construction: a
// token carrying trailing junk fails the caller's full-name equality
// check and REFUSES — never a false attribution.
func cutBranchToken(s string) string {
	end := strings.IndexFunc(s, func(r rune) bool {
		switch r {
		case ' ', '\t', '\'', '"', '`', ']', ')', '>', ',', ';':
			return true
		}
		return r < 0x20
	})
	if end >= 0 {
		return s[:end]
	}
	return s
}
