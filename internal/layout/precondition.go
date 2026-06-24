package layout

import (
	"fmt"
	"strings"
)

// RefCandidate is a discovered ref the precondition evaluated. A ref BLOCKS the
// flatten iff it is unmerged into the target AND its merge-base layout
// fingerprint is still pre-flatten (Req 11).
type RefCandidate struct {
	Name       string
	Merged     bool // merged into the target branch
	PreFlatten bool // merge-base tree still carries the canonical .mindspec/docs layout
}

// blocks reports the Req 11 block predicate: unmerged AND pre-flatten.
func (c RefCandidate) blocks() bool { return !c.Merged && c.PreFlatten }

// blockingRefs filters the candidates to those that block the flatten. This is
// the pure decision core of the precondition (directly unit-tested for AC16).
func blockingRefs(cands []RefCandidate) []RefCandidate {
	var blocking []RefCandidate
	for _, c := range cands {
		if c.blocks() {
			blocking = append(blocking, c)
		}
	}
	return blocking
}

// PreconditionResult is the outcome of the migrate-layout discovery scan.
type PreconditionResult struct {
	Blocking  []RefCandidate // unmerged pre-flatten refs that BLOCK the flatten
	Tolerated []string       // locked-worktree branches + external-fork refs (NOT blockers)
	Warnings  []string       // e.g. offline → hosted PRs not consulted
}

// classifyRefs splits the discovered local and remote-tracking refs into
// block-candidates and TOLERATED refs (Req 11). The target itself and the
// remote HEAD are excluded. Locked agent worktrees and external forks are
// tolerated, never counted as blockers:
//   - a local branch in lockedWorktrees is a locked agent worktree → tolerated;
//   - a remote-tracking ref under a remote OTHER than origin is an external
//     fork → tolerated (it cannot be drained, only fingerprint-guarded at merge).
func classifyRefs(locals, remotes []string, target string, lockedWorktrees map[string]bool) (candidates, tolerated []string) {
	for _, r := range locals {
		if r == target {
			continue
		}
		if lockedWorktrees[r] {
			tolerated = append(tolerated, r)
			continue
		}
		candidates = append(candidates, r)
	}
	for _, r := range remotes {
		remote := r
		if i := strings.IndexByte(r, '/'); i >= 0 {
			remote = r[:i]
		}
		if remote != "origin" {
			tolerated = append(tolerated, r) // external fork
			continue
		}
		branch := strings.TrimPrefix(r, "origin/")
		if branch == target || branch == "HEAD" {
			continue
		}
		candidates = append(candidates, r)
	}
	return candidates, tolerated
}

// evaluateCandidate computes a candidate ref's merged/pre-flatten facts via the
// git seam: merged ⟺ merge-base(target, ref) == rev-parse(ref); pre-flatten ⟺
// the merge-base tree still carries the canonical `.mindspec/docs` layout.
func evaluateCandidate(git GitOps, root, target, ref string) (RefCandidate, error) {
	mb, err := git.MergeBase(target, ref)
	if err != nil {
		return RefCandidate{}, fmt.Errorf("merge-base %s %s: %w", target, ref, err)
	}
	refSha, err := git.RevParseRef(root, ref)
	if err != nil {
		return RefCandidate{}, fmt.Errorf("rev-parse %s: %w", ref, err)
	}
	sig, err := signatureAtRef(git, mb)
	if err != nil {
		return RefCandidate{}, fmt.Errorf("fingerprint %s: %w", ref, err)
	}
	return RefCandidate{
		Name:       ref,
		Merged:     mb == refSha,
		PreFlatten: sig == "canonical",
	}, nil
}

// PreconditionOptions tunes the discovery scan.
type PreconditionOptions struct {
	Target           string          // merge target (default "main")
	LockedWorktrees  map[string]bool // local branches checked out in LOCKED worktrees (tolerated)
	Offline          bool            // no hosted-PR consultation possible → WARN, degrade to refs
	RequireCleanTree bool            // refuse to proceed on a dirty idle working tree
}

// CheckPrecondition runs the deterministic branch/PR discovery scan (Req 11):
// it enumerates local + remote-tracking refs, classifies them, evaluates each
// block-candidate's merged/pre-flatten fingerprint, and returns the blocking
// set. Offline it degrades to local + remote-tracking refs and WARNS (it does
// not silently pass). When RequireCleanTree is set it first refuses a dirty
// working tree.
func CheckPrecondition(git GitOps, root string, opts PreconditionOptions) (*PreconditionResult, error) {
	target := opts.Target
	if target == "" {
		target = "main"
	}

	if opts.RequireCleanTree {
		dirty, err := dirtyNonOperational(git, root)
		if err != nil {
			return nil, err
		}
		if dirty {
			return nil, fmt.Errorf("migrate layout: refusing to run on a dirty working tree\nrecovery: commit or stash your changes, then re-run `mindspec migrate layout`")
		}
	}

	locals, err := git.LocalBranchRefs(root)
	if err != nil {
		return nil, fmt.Errorf("scanning local branch refs: %w", err)
	}
	remotes, err := git.RemoteTrackingRefs(root)
	if err != nil {
		// Remote-tracking enumeration failing is treated as offline: degrade.
		remotes = nil
		opts.Offline = true
	}

	candidates, tolerated := classifyRefs(locals, remotes, target, opts.LockedWorktrees)

	res := &PreconditionResult{Tolerated: tolerated}
	if opts.Offline || len(remotes) == 0 {
		res.Warnings = append(res.Warnings,
			"offline / no remote-tracking refs: hosted PRs could not be consulted; scan degraded to local + remote-tracking refs")
	}

	for _, ref := range candidates {
		c, err := evaluateCandidate(git, root, target, ref)
		if err != nil {
			return nil, err
		}
		if c.blocks() {
			res.Blocking = append(res.Blocking, c)
		}
	}
	return res, nil
}

// dirtyNonOperational reports whether the working tree has any change other
// than the mover's own run-state / lineage residue.
func dirtyNonOperational(git GitOps, root string) (bool, error) {
	out, err := git.Status(root)
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		p := strings.TrimSpace(line[3:])
		if idx := strings.Index(p, " -> "); idx >= 0 {
			p = strings.TrimSpace(p[idx+4:])
		}
		p = strings.Trim(p, "\"")
		if p == "" || isOperationalPath(p) {
			continue
		}
		return true, nil
	}
	return false, nil
}

// BlockingError renders a blocking precondition result as an actionable error.
func BlockingError(res *PreconditionResult) error {
	var b strings.Builder
	b.WriteString("migrate layout: blocked — unmerged pre-flatten branch/PR(s) must be drained first:")
	for _, c := range res.Blocking {
		fmt.Fprintf(&b, "\n  %s (unmerged, pre-flatten)", c.Name)
	}
	b.WriteString("\nrecovery: merge or close each branch/PR above, then re-run `mindspec migrate layout`")
	return fmt.Errorf("%s", b.String())
}
