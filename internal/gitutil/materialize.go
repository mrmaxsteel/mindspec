// Spec 128 Bead 6 (OQ-1(a) / ADR-0041 §1 amendment): the prospective-
// snapshot protocol's git primitives.
//
// This file gives `mindspec complete`'s per-bead content gates
// (doc-sync, ADR-divergence) a way to measure the bead worktree's
// PROSPECTIVE content — tracked, untracked, and deleted, exactly what
// `git add -A` would stage — as an immutable candidate tree BEFORE any
// commit exists, and gives the caller (internal/executor,
// internal/complete) a write-ahead protocol for turning a gate-passed
// candidate into the real bead-branch commits with an at-least-once,
// idempotently-replayable resume contract.
//
// The three moving parts, each documented at its own primitive below:
//
//  1. SnapshotWorktreeTree — builds the candidate tree T via a TEMPORARY
//     index (the repo's real index, refs, and worktree are untouched).
//  2. MaterializeStageCommit + the Mindspec-Stage trailer — builds a
//     commit object (plumbing, no hooks) from a candidate tree, tagged
//     with which stage of the M(aterialize)/R(eset)/S(ync) sequence it
//     represents, so a later re-entry can classify it from the commit
//     itself rather than guessing from a tree diff.
//  3. The write-ahead marker (MaterializationMarkerRef +
//     ReadMaterializationMarker/PublishMaterializationMarker/
//     ClearMaterializationMarker) + AdvanceBranchExpectedOld — a
//     durably-published "this OID is about to become (or has become)
//     the bead branch's new tip" record, published via per-ref
//     compare-and-swap BEFORE the branch ref itself moves, so a crash
//     between the two leaves durable, race-safe evidence for the next
//     invocation's re-entry discriminator.
package gitutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// MindspecStageTrailerKey is the pinned commit-message trailer key the
// write-ahead protocol reads its stage classification from (bead 6,
// round-6 G1): total under every re-entry, never inferred from a tree
// diff (which is ambiguous when a stage's own content happens to be a
// no-op — see StageMaterialization/StageArtifactSync's own docs).
const MindspecStageTrailerKey = "Mindspec-Stage"

// The two named stages the write-ahead marker's trailer distinguishes.
// StageMaterialization is commit C (built from the gate-measured
// snapshot tree T, parented on the bead's pre-run tip A).
// StageArtifactSync is commit D (C's tree with the tracker artifact's
// blob refreshed, parented on C — or, when C itself elides, directly on
// A; see ReadMaterializationMarker's doc comment for the elision rules).
const (
	StageMaterialization = "materialization"
	StageArtifactSync    = "artifact-sync"
)

// nullOID is git's null object ID (40 zeros) — update-ref's documented
// "the reference is expected to not exist" old-value convention
// (git-update-ref(1)), used by PublishMaterializationMarker's
// absent→C leg to get CREATE semantics rather than an unconditional
// overwrite.
const nullOID = "0000000000000000000000000000000000000000"

// SnapshotWorktreeTree builds a candidate tree T from wtPath's
// PROSPECTIVE content — tracked, untracked, and deleted, exactly what
// `git add -A` would stage — via a TEMPORARY index (GIT_INDEX_FILE):
// wtPath's real index, refs, and worktree are never touched; the only
// objects written are unreferenced loose objects (the same
// non-mutating discipline as stripNovelPaths above, in
// workdestruction.go).
//
// The temp index is SEEDED FROM wtPath's OWN HEAD tip FIRST
// (`read-tree HEAD`) before `add -A` runs (round-2 G1's
// synthetic-deletion fix, reproduced and fixed this round): an
// UNSEEDED temp index plus excludePaths makes any excluded,
// currently-TRACKED artifact ABSENT from T — nothing ever staged it
// back in — and a commit built from T would silently DELETE that
// artifact from the branch. Seeded, the exclusion means "unchanged at
// tip": the artifact's tip blob/mode is carried over into T untouched,
// because the seed already has it and `add -A -- . <exclusion>` never
// re-stages it.
//
// excludePaths are named-artifact exclusions in plain repo-relative
// path form (e.g. ".beads/issues.jsonl"); this function itself builds
// git's short exclude-pathspec form (":!<path>") for each, so callers
// need not know that syntax.
func SnapshotWorktreeTree(wtPath string, excludePaths []string) (tree string, err error) {
	tip, err := RevParseHEAD(wtPath)
	if err != nil {
		return "", fmt.Errorf("resolving %s HEAD to seed the snapshot: %w", wtPath, err)
	}

	idxDir, idxPath, err := tempIndexPath()
	if err != nil {
		return "", fmt.Errorf("allocating temporary index: %w", err)
	}
	defer os.RemoveAll(idxDir)
	env := append(os.Environ(), "GIT_INDEX_FILE="+idxPath)

	readCmd := execCommand("git", gitArgs(wtPath, "read-tree", tip)...)
	readCmd.Env = env
	if out, rerr := readCmd.CombinedOutput(); rerr != nil {
		return "", fmt.Errorf("read-tree %s (seeding snapshot of %s): %s: %w", tip, wtPath, strings.TrimSpace(string(out)), rerr)
	}

	addArgs := []string{"add", "-A", "--", "."}
	for _, p := range excludePaths {
		addArgs = append(addArgs, ":!"+p)
	}
	addCmd := execCommand("git", gitArgs(wtPath, addArgs...)...)
	addCmd.Env = env
	if out, aerr := addCmd.CombinedOutput(); aerr != nil {
		return "", fmt.Errorf("add -A (snapshotting %s): %s: %w", wtPath, strings.TrimSpace(string(out)), aerr)
	}

	writeCmd := execCommand("git", gitArgs(wtPath, "write-tree")...)
	writeCmd.Env = env
	out, werr := writeCmd.Output()
	if werr != nil {
		return "", fmt.Errorf("write-tree (snapshotting %s): %w", wtPath, werr)
	}
	return strings.TrimSpace(string(out)), nil
}

// ChangedFilesTree runs `git diff --name-only <base> <treeOID> --` in
// workdir — the tree-vs-tree leg of the prospective-snapshot protocol
// (spec 128 bead 6): unlike DiffNameOnly's `base..head` RANGE-string
// form, this passes base and treeOID as two SEPARATE revision operands
// (git diff's plain two-sided form, which `A..B` itself desugars to),
// since treeOID is typically a bare tree object (from
// SnapshotWorktreeTree) rather than a commit — git diff accepts either
// transparently, commit-ish or tree-ish, on either side. workdir is
// threaded explicitly (never a hardcoded g.Root, never the caller's
// cwd) so this is always measured against the SAME bead worktree the
// snapshot itself was taken in — closing the g.Root-rooted
// (ChangedFiles) and cwd-rooted (DiffNameOnlyRef("", ref)) holes the
// OQ-1 mechanism names.
func ChangedFilesTree(workdir, base, treeOID string) ([]string, error) {
	if err := rejectOptionLike(base); err != nil {
		return nil, err
	}
	if err := rejectOptionLike(treeOID); err != nil {
		return nil, err
	}
	cmd := execCommand("git", gitArgs(workdir, "diff", "--name-only", base, treeOID, "--")...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("diff --name-only %s %s: %w", base, treeOID, err)
	}
	return splitLines(string(out)), nil
}

// buildStageCommitMessage appends the pinned Mindspec-Stage trailer
// (blank-line separated, per git trailer convention) to subject.
func buildStageCommitMessage(subject, stage string) string {
	return subject + "\n\n" + MindspecStageTrailerKey + ": " + stage + "\n"
}

// ReadStageTrailer parses ref's commit message (via CommitMessageBody)
// for the pinned Mindspec-Stage trailer. present is false — with a nil
// error — when the commit carries no such trailer (an ordinary,
// non-write-ahead commit), OR when it carries MORE THAN ONE
// Mindspec-Stage line (fix-round G2): a well-formed write-ahead commit
// this protocol builds carries exactly one, so any duplicate — whether
// consistent or conflicting — is an AMBIGUOUS encoding (a crafted or
// corrupt commit), never resolved by picking one arbitrarily. Bead 6's
// re-entry discriminator reads the stage from this ENCODING alone,
// never infers it from a tree diff, which is ambiguous when a stage's
// own content happens to be a no-op — and never guesses past an
// ambiguous encoding either.
func ReadStageTrailer(workdir, ref string) (stage string, present bool, err error) {
	body, err := CommitMessageBody(workdir, ref)
	if err != nil {
		return "", false, err
	}
	prefix := MindspecStageTrailerKey + ": "
	var found []string
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, prefix) {
			found = append(found, strings.TrimSpace(strings.TrimPrefix(line, prefix)))
		}
	}
	if len(found) != 1 {
		return "", false, nil
	}
	return found[0], true, nil
}

// MaterializeStageCommit builds (via `git commit-tree`, plumbing — no
// hooks run) a write-ahead stage commit from tree, parented on parent,
// carrying the pinned Mindspec-Stage: <stage> trailer appended to
// subject. Mirrors CommitTreeMerge's signing-aware construction: when
// the target repository's effective config asks for signed commits
// (CommitSigningEnabled), the produced object is signed too, so a
// stage commit carries the same provenance guarantee a porcelain
// `git commit` would have. This does not publish any ref or marker —
// callers publish the write-ahead marker (PublishMaterializationMarker)
// BEFORE advancing any branch ref to the returned OID.
func MaterializeStageCommit(workdir, tree, parent, stage, subject string) (string, error) {
	for _, s := range []string{tree, parent} {
		if err := rejectOptionLike(s); err != nil {
			return "", err
		}
	}
	signing, err := CommitSigningEnabled(workdir)
	if err != nil {
		return "", err
	}
	msg := buildStageCommitMessage(subject, stage)
	argv := []string{"commit-tree"}
	if signing {
		argv = append(argv, "-S")
	}
	argv = append(argv, tree, "-p", parent, "-m", msg)
	cmd := execCommand("git", gitArgs(workdir, argv...)...)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return "", fmt.Errorf("commit-tree (stage=%s) in %s: %w: %s", stage, workdir, err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("commit-tree (stage=%s) in %s: %w", stage, workdir, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ResetIndexOnlyMixed runs `git reset --quiet --mixed HEAD` in workdir
// — refreshes the REAL index to match HEAD's tree, leaving the
// worktree's files untouched. Bead 6: called immediately after a
// branch ref has been advanced (AdvanceBranchExpectedOld) to a
// commit-tree-built stage commit; without it, paths newly tracked via
// that commit read as phantom staged deletions in `git status` (the
// index still reflects the PRE-advance tip while HEAD now points past
// it) — this refresh is what makes the checked-out branch's real index
// agree with its own new HEAD again.
func ResetIndexOnlyMixed(workdir string) error {
	cmd := execCommand("git", gitArgs(workdir, "reset", "--quiet", "--mixed", "HEAD")...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("reset --quiet --mixed HEAD in %s: %s", workdir, strings.TrimSpace(string(out)))
	}
	return nil
}

// UpdateRefCAS runs `git update-ref <ref> <newSHA> <oldSHA>` in workdir
// — the three-operand compare-and-swap form: the write FAILS
// (non-nil error) if ref's CURRENT value does not equal oldSHA exactly.
// Pass oldSHA == "" to mean "ref must not currently exist" (translated
// to the null OID, git's own documented CREATE-semantics convention).
// Bead 6: the write-ahead materialization marker's race-safe
// publication (absent→C uses the empty-oldSHA CREATE form, C→D uses
// the real prior OID) and the bead-branch tip's own expected-old
// advance both route through this one primitive.
func UpdateRefCAS(workdir, ref, newSHA, oldSHA string) error {
	if err := rejectOptionLike(ref); err != nil {
		return err
	}
	if err := rejectOptionLike(newSHA); err != nil {
		return err
	}
	old := oldSHA
	if old == "" {
		old = nullOID
	} else if err := rejectOptionLike(old); err != nil {
		return err
	}
	cmd := execCommand("git", gitArgs(workdir, "update-ref", ref, newSHA, old)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("update-ref (CAS) %s %s (expected old %s): %s", ref, newSHA, oldSHA, strings.TrimSpace(string(out)))
	}
	return nil
}

// AdvanceBranchExpectedOld moves the LOCAL branch ref
// `refs/heads/<branch>` in workdir to newSHA, requiring its CURRENT
// value to equal expectedOld exactly — bead 6's compare-and-swap tip
// advance: a concurrent mover FAILS this call rather than silently
// clobbering the other mover's work (ADR-0035 refusal territory for
// the caller, not this primitive).
func AdvanceBranchExpectedOld(workdir, branch, newSHA, expectedOld string) error {
	if err := rejectOptionLike(branch); err != nil {
		return err
	}
	return UpdateRefCAS(workdir, "refs/heads/"+branch, newSHA, expectedOld)
}

// MaterializationMarkerRef returns the write-ahead materialization
// marker ref for beadID (spec 128 bead 6):
// refs/mindspec/materialized/<bead-id>. Mirrors
// MergeSourceMarkerRef's own naming convention (spec 127).
func MaterializationMarkerRef(beadID string) string {
	return "refs/mindspec/materialized/" + beadID
}

// PublishMaterializationMarker publishes oid as beadID's write-ahead
// marker via per-ref compare-and-swap: expectedOld == "" gives CREATE
// semantics (fails if the marker already exists — the first
// publication of a run); a non-empty expectedOld requires the marker's
// CURRENT value to equal it exactly (the C→D transition). A failed
// publication (an unexpected existing value: another invocation in
// flight, or a leftover) is the caller's ADR-0035 refusal territory —
// this primitive only reports the CAS failure; it never overwrites
// another invocation's recovery evidence.
func PublishMaterializationMarker(workdir, beadID, oid, expectedOld string) error {
	return UpdateRefCAS(workdir, MaterializationMarkerRef(beadID), oid, expectedOld)
}

// ReadMaterializationMarker resolves beadID's write-ahead marker ref,
// if any, and — when present — the Mindspec-Stage trailer of the
// commit it points at. Absence is (`"", "", false, nil`) — NOT an
// error — exactly like RevParseRef's own "ref genuinely absent"
// contract (ErrRefNotFound), which this wraps into a plain bool so
// callers above gitutil need not import the sentinel.
//
// present == true with stage == "" (nil error) is a foreign marker
// object — one this protocol did not itself publish (no
// Mindspec-Stage trailer). Bead 6's re-entry discriminator treats this
// identically to "no marker for this run": the caller deletes it and
// proceeds as a normal run (round-6 G1's "read from the encoding, never
// the diff" contract — a marker whose OWN encoding is absent carries no
// resumable information).
func ReadMaterializationMarker(workdir, beadID string) (oid, stage string, present bool, err error) {
	oid, err = RevParseRef(workdir, MaterializationMarkerRef(beadID))
	if err != nil {
		if errors.Is(err, ErrRefNotFound) {
			return "", "", false, nil
		}
		return "", "", false, err
	}
	stage, _, terr := ReadStageTrailer(workdir, oid)
	if terr != nil {
		return "", "", false, fmt.Errorf("reading stage trailer of marker %s (bead %s): %w", oid, beadID, terr)
	}
	return oid, stage, true, nil
}

// ClearMaterializationMarker deletes beadID's write-ahead marker ref via
// COMPARE-AND-SWAP against expectedOID — the exact OID the caller
// observed/owns (fix-round G4): `git update-ref -d <ref> <expectedOID>`
// is git's own documented delete-with-old-value form (git-update-ref(1):
// "Delete the reference ... if it is exactly at <oldvalue>"), so the
// delete FAILS ATOMICALLY if another invocation has since published a
// DIFFERENT marker — a stale clear (one that observed an OLDER value
// before a race) can never delete evidence it does not own, closing the
// clear-side counterpart to the marker-publish CAS.
//
// expectedOID == "" means the caller believes no marker exists (a
// no-op: there is nothing to CAS against). Idempotent for the caller's
// OWN OID too: if the marker has ALREADY been cleared (by this call
// racing a concurrent clear, or a prior invocation) the ref is simply
// absent, which is treated as success, not a CAS mismatch — cleared is
// cleared, regardless of who did it. A marker present with a DIFFERENT
// current value than expectedOID is a genuine mismatch and returns an
// error naming it, run on both the ordinary and the landed-merge-
// reconcile completion paths.
func ClearMaterializationMarker(workdir, beadID, expectedOID string) error {
	if expectedOID == "" {
		return nil
	}
	// Final-review round 1 (R12/G8): gate BOTH dynamic operands like every
	// sibling marker primitive in this file (UpdateRefCAS gates all three
	// of its operands). Every current caller passes a rev-parse-produced
	// hex SHA for expectedOID, but ADR-0042 §1 forbids trusting an
	// id-shaped operand by PROVENANCE — this runtime gate is what the
	// argv-audit table's opClassRejectOptionLikeGated entry claims, and it
	// catches a future caller forwarding non-SHA text.
	ref := MaterializationMarkerRef(beadID)
	if err := rejectOptionLike(ref); err != nil {
		return err
	}
	if err := rejectOptionLike(expectedOID); err != nil {
		return err
	}
	cmd := execCommand("git", gitArgs(workdir, "update-ref", "-d", ref, expectedOID)...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	curOID, _, present, rerr := ReadMaterializationMarker(workdir, beadID)
	if rerr == nil && !present {
		// Already gone (this call's own effect, a concurrent clear, or a
		// prior invocation) — idempotent success.
		return nil
	}
	if rerr == nil && present && curOID != expectedOID {
		return fmt.Errorf("clear-marker CAS mismatch for %s: expected %s, found %s (a concurrent invocation has since published a different marker — not deleted): %s",
			beadID, expectedOID, curOID, strings.TrimSpace(string(out)))
	}
	return fmt.Errorf("update-ref -d (CAS) %s (expected %s): %s", MaterializationMarkerRef(beadID), expectedOID, strings.TrimSpace(string(out)))
}
