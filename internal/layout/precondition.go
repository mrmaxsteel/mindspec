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
// block-candidates and TOLERATED refs (Req 11, bead sc0w). The migration TARGET
// and the remote default branch — the branches the flatten actually lands onto
// — plus the remote HEAD are excluded, so refs that do not target the migration
// branch or the remote default are not arbitrarily counted as blockers. Locked
// agent worktrees, external forks, and operator-allowlisted refs are tolerated,
// never counted as blockers:
//   - a local branch in lockedWorktrees is a locked agent worktree → tolerated;
//   - a remote-tracking ref under a remote OTHER than origin is an external
//     fork → tolerated (it cannot be drained, only fingerprint-guarded at merge);
//   - a ref whose branch name is in allowlist is an operator-declared
//     known-irrelevant branch → tolerated (the bead-sc0w escape for old/
//     abandoned branches that would otherwise wall the flatten).
func classifyRefs(locals, remotes []string, target, remoteDefault string, lockedWorktrees, allowlist map[string]bool) (candidates, tolerated []string) {
	for _, r := range locals {
		if r == target || (remoteDefault != "" && r == remoteDefault) {
			continue
		}
		if lockedWorktrees[r] {
			tolerated = append(tolerated, r)
			continue
		}
		if allowlisted(r, allowlist) {
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
		if branch == target || branch == "HEAD" || (remoteDefault != "" && branch == remoteDefault) {
			continue
		}
		if allowlisted(r, allowlist) {
			tolerated = append(tolerated, r)
			continue
		}
		candidates = append(candidates, r)
	}
	return candidates, tolerated
}

// allowlisted reports whether ref is operator-allowlisted: a match on the FULL
// ref name (covering a slashed local branch such as "fix/old-thing") or on the
// branch part of an "origin/<branch>" remote-tracking ref.
func allowlisted(ref string, allowlist map[string]bool) bool {
	if allowlist[ref] {
		return true
	}
	if b := strings.TrimPrefix(ref, "origin/"); b != ref && allowlist[b] {
		return true
	}
	return false
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
	RemoteDefault    string          // remote default branch (e.g. "main"/"master"); excluded from blockers alongside Target (bead sc0w)
	LockedWorktrees  map[string]bool // local branches checked out in LOCKED worktrees (tolerated)
	Allowlist        map[string]bool // operator-declared known-irrelevant branch names (tolerated, never blockers — bead sc0w)
	Force            bool            // drop EVERY discovered blocker (the unrelated-stale-branch escape — bead sc0w); dropped blockers surface as a WARN
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

	candidates, tolerated := classifyRefs(locals, remotes, target, opts.RemoteDefault, opts.LockedWorktrees, opts.Allowlist)

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

	// --force escape (bead sc0w): the operator has judged the discovered
	// pre-flatten branches irrelevant. Drop them to TOLERATED and surface a
	// WARN so the bypass is auditable rather than silent.
	if opts.Force && len(res.Blocking) > 0 {
		names := make([]string, 0, len(res.Blocking))
		for _, c := range res.Blocking {
			names = append(names, c.Name)
			res.Tolerated = append(res.Tolerated, c.Name)
		}
		res.Warnings = append(res.Warnings,
			"--force: bypassing "+fmt.Sprintf("%d", len(names))+" unmerged pre-flatten blocker(s) at operator request: "+strings.Join(names, ", "))
		res.Blocking = nil
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
