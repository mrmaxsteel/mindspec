package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

// releaseDeps collects the I/O seams `runRelease` drives so the strict 6-step
// reversal can be unit-tested at the cmd layer without `bd`/git on PATH and
// without a real worktree. The production wiring is assembled in releaseCmd's
// RunE (see newReleaseDeps).
//
// The two destructive steps are deliberately split across distinct seams in
// the order the spec mandates:
//   - removeWorktree removes the bead worktree AND chdirs to the repo root in
//     one call (the cwd-safety unit, executor.RemoveBeadWorktreeAndRestore);
//   - setOpen sets the bead back to open + clears its assignee LAST.
//
// runRelease enforces remove-FIRST / set-open-LAST by call ordering; tests
// record the order through these seams.
type releaseDeps struct {
	// root is the resolved MAIN repo root (the chdir target after removal).
	root string
	// beadWorktreePath is the absolute bead worktree path used for the
	// dirty-check (resolved by the caller via workspace.BeadWorktreePath).
	beadWorktreePath string

	// checkDirty classifies user-authored dirt in the bead worktree
	// (next.CheckDirtyTree). A non-empty slice with no --force blocks removal.
	checkDirty func(repoRoot, cwd string) ([]string, error)
	// removeWorktree removes the bead worktree via the executor (ADR-0030) and
	// chdirs to root immediately after (spec-092 cwd-safety). Step 3 + 4.
	removeWorktree func(beadID string) error
	// setOpen sets the bead back to open and clears the assignee (Step 5).
	setOpen func(beadID string) error
	// activeBead returns the currently-active bead ID (bd's single in_progress
	// child, ADR-0023) and the epic owning the released bead, for the Step 6
	// cursor rewind / mindspec_phase cache sync. A "" active bead or any error
	// leaves the cursor untouched.
	activeBead func(beadID string) (active string, epicID string, ok bool)
	// syncPhase re-derives and writes the epic's child-derived mindspec_phase
	// (Step 6.5) via the sanctioned redesign leg (phase.SanctionedDerivedSync,
	// spec 128 Bead 2) — backward permitted, re-derived fresh inside its own
	// lock critical section, never a value precomputed by the caller.
	// Best-effort; an error is a warning, never fatal.
	syncPhase func(epicID string)

	stdout io.Writer
	stderr io.Writer
}

var releaseCmd = &cobra.Command{
	Use:   "release <bead-id>",
	Short: "Reverse a claim: remove the bead worktree and return the bead to open",
	Long: `Cleanly reverses a claim made by ` + "`mindspec next`" + `, in this order:
  1. Resolve the repo root and the bead worktree path
  2. Dirty-check the bead worktree — refuse (no removal) if it has uncommitted
     user changes, unless --force is given
  3. Remove the bead worktree via the executor (bd worktree remove)
  4. chdir to the repo root immediately after removal (cwd-safety)
  5. Set the bead back to open and clear its assignee
  6. Rewind the state cursor only if it pointed at the released bead

The order is remove-worktree-first, set-open-last so a partial failure leaves a
recoverable "still-claimed, worktree-gone" state (re-run release or mindspec
next to recover) rather than an "open + stale-worktree" collision.

--force is a mindspec-level pre-gate: it allows removal of a DIRTY worktree
(uncommitted work is discarded). It is not a passthrough to bd.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		beadID := strings.TrimSpace(args[0])
		force, _ := cmd.Flags().GetBool("force")

		// R3 explicit-ingress early gate (ADR-0042, AC-6): a malformed
		// beadID refuses HERE, before any composition.
		if err := idvalidate.BeadID(beadID); err != nil {
			return guard.NewFailure(
				fmt.Sprintf("%s is not a valid bead ID: %v", termsafe.Escape(beadID), err),
				"bd ready   (pick a listed bead ID and re-run)",
			)
		}

		root, err := findRoot()
		if err != nil {
			return err
		}
		if err := bead.Preflight(root); err != nil {
			fmt.Fprintf(os.Stderr, "preflight failed: %v\n", err)
			os.Exit(1)
		}

		deps := newReleaseDeps(root, beadID)
		if err := runRelease(deps, beadID, force); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return nil
	},
}

// releaseSyncPhase performs release's step-6.5 cursor-rewind phase-cache
// sync. Spec 128 Bead 2 (OQ-3 option 1, the sanction boundary): this is
// the ONE production caller of phase.SanctionedDerivedSync — the child-
// derived phase is re-derived FRESH inside that function's own lock
// critical section (never precomputed here and merely closed over, the
// exact stale-target race the R3.3 lock exists to close), backward move
// permitted, because release's own guard-routed internal caller is
// re-deriving from the epic's own fresh evidence rather than asking a
// human to override a guard against possibly-stale evidence (the
// operator-facing `mindspec repair phase` guard is unaffected).
//
// Ahead of the sync itself, a v2 phase-semantics self-heal
// (phase.EnsurePhaseSemanticsV2) runs first — the same one-shot
// reconciliation `plan approve`/`impl approve`/`complete`/`next`'s claim
// path run — so a legacy epic converges before the sanctioned sync
// re-derives, rather than the sync racing a stale pre-v2 stored value.
//
// Both calls are best-effort writes: a failure never fails `release`
// itself (the sync is a cache refresh, not the terminal mutation), but
// round-1 panel G3 requires it be SURFACED, not silently discarded. The
// two legs get DIFFERENT recovery treatment (round-1 panel G5/R3-G2 —
// the pre-fix-round `mindspec release <bead-id>` recovery was a DEAD END
// on BOTH legs: the bead is already open by the time a re-run happens,
// so Step 6.5's own activeBead-cursor precondition is never met again):
//   - EnsurePhaseSemanticsV2 failure: the self-heal is idempotent and its
//     own effect (when it does anything at all) is a forward translation
//     — `mindspec repair phase <spec-id>` genuinely converges (that verb
//     runs the identical self-heal at its own entry), so it gets the
//     SAME always-convergent treatment as next.go's advance-on-claim
//     failure (describePhaseSyncFailure).
//   - SanctionedDerivedSync failure: this IS the sanctioned BACKWARD
//     write (AC-5(viii)'s own twin pins that the unforced OPERATOR guard
//     REFUSES this exact backward state) — there is genuinely NO
//     unforced copy-pastable fix. Recorded disposition: the recovery
//     line names ONLY the safe inspection action (`bd show <epic-id>`,
//     matching AC-5(iv)'s backward-branch convention and the house
//     "never a --force on the greppable recovery line" rule this
//     package's own dirty-tree refusal already follows), with the
//     forcing escape named as a separately-labeled operator choice in
//     the message body. A fuller fix (e.g. surfacing this state through
//     the agreement-table/result-message work) is explicitly routed to
//     beads 4/5, named here so it is not a silent dead end.
//
// Extracted to its own named, root-parameterized function (rather than
// an inline closure capturing newReleaseDeps' root) so it is directly
// unit-testable without exercising newReleaseDeps' full bd/git wiring.
func releaseSyncPhase(root, epicID, specID string) {
	recovery := nextPhaseAdvanceRecoveryCommand(specID)
	if _, err := phase.EnsurePhaseSemanticsV2(root, epicID); err != nil {
		fmt.Fprintln(os.Stderr, describePhaseSyncFailure(err, recovery))
	}
	if syncErr := phase.SanctionedDerivedSync(root, epicID); syncErr != nil {
		fmt.Fprintln(os.Stderr, describeReleaseSyncFailure(syncErr, epicID, specID))
	}
}

// describeReleaseSyncFailure renders a SanctionedDerivedSync failure —
// see releaseSyncPhase's doc comment for the disposition this
// implements (round-1 panel G5/R3-G2). specID may be empty (the same
// "<spec-id>" placeholder convention as elsewhere).
func describeReleaseSyncFailure(err error, epicID, specID string) string {
	specRef := "<spec-id>"
	if specID != "" {
		specRef = idrender.Spec(specID)
	}
	return fmt.Sprintf(
		"could not sync phase metadata for epic %s: %v\n"+
			"Operator choice — this is the sanctioned backward move (release), so no unforced automatic fix exists; force it only if intentional: mindspec repair phase %s --force-backward \"<reason>\".\n"+
			"recovery: bd show %s",
		idrender.Bead(epicID), err, specRef, idrender.Bead(epicID))
}

// newReleaseDeps assembles the production seams for `mindspec release`.
//
// Removal routes through the CONCRETE *MindspecExecutor (ADR-0030): WorktreeOps
// lives on the concrete type, not the Executor interface, and the
// remove-then-chdir cwd-safety unit (RemoveBeadWorktreeAndRestore) is a method
// on it. The bd-state mutation is placed HERE in cmd/mindspec (not in
// internal/next/beads.go, which Bead 3 owns).
func newReleaseDeps(root, beadID string) releaseDeps {
	cfg, cfgErr := config.Load(root)
	if cfgErr != nil {
		cfg = config.DefaultConfig()
	}
	// The bead worktree is nested under its spec worktree. Resolving the spec
	// worktree from the bead's epic mirrors how complete.go finds the bead
	// worktree; the path is used only for the dirty-check (the executor's
	// Remove resolves the worktree by name via bd). The same resolved specID
	// is reused below for releaseSyncPhase's convergent recovery commands
	// (spec 128 Bead 2 round-1 panel G5) — one lookup, two uses.
	specWorktree := root
	var resolvedSpecID string
	if _, specID, ferr := phase.FindEpicForBead(beadID); ferr == nil && specID != "" {
		// specID is D1-checked (phase.FindEpicForBead only returns a
		// non-empty specID that already passed idvalidate.SpecID) — this
		// waist call cannot fail.
		resolvedSpecID = specID
		if swt, swErr := workspace.SpecWorktreePath(root, cfg, specID); swErr == nil {
			specWorktree = swt
		}
	}
	// beadID already passed idvalidate.BeadID at the CLI early gate above;
	// this waist call cannot fail.
	beadWtPath, _ := workspace.BeadWorktreePath(specWorktree, cfg, beadID)

	exec := executor.NewMindspecExecutor(root)

	return releaseDeps{
		root:             root,
		beadWorktreePath: beadWtPath,
		checkDirty:       next.CheckDirtyTree,
		removeWorktree:   exec.RemoveBeadWorktreeAndRestore,
		setOpen:          defaultSetOpen,
		activeBead:       defaultActiveBead,
		// Spec 128 Bead 2: routed through phase.SanctionedDerivedSync (the
		// sanctioned redesign leg) — see releaseSyncPhase. Round-1 panel
		// G3: a lock/write failure is best-effort (the cursor-rewind
		// cache sync never fails `release` itself) but is no longer
		// silently discarded.
		syncPhase: func(epicID string) { releaseSyncPhase(root, epicID, resolvedSpecID) },
		stdout:    os.Stdout,
		stderr:    os.Stderr,
	}
}

// bdRunCombinedForRelease is the package-level bd seam for the Step 5 bead-state
// mutation, swappable in tests to assert the exact `bd update` args without
// `bd` on PATH.
var bdRunCombinedForRelease = bead.RunBDCombined

// defaultSetOpen sets the bead back to open and clears its assignee (Step 5).
func defaultSetOpen(beadID string) error {
	return defaultSetOpenVia(bdRunCombinedForRelease, beadID)
}

// defaultSetOpenVia is the seam-driven body of defaultSetOpen. bd runs in
// embedded auto-commit mode (complete.go honesty-clause): the
// `bd update --status open` write auto-commits to durable Dolt state, so no
// separate commit-reconcile shell-out is needed here. An empty --assignee
// clears the assignee.
func defaultSetOpenVia(run func(args ...string) ([]byte, error), beadID string) error {
	out, err := run("update", beadID, "--status=open", "--assignee=")
	if err != nil {
		// R4: beadID is an ID-typed position (idrender.Bead); the bd
		// subprocess output is agent-influenced porcelain text — escape it
		// per-line.
		return fmt.Errorf("setting bead %s back to open: %s", idrender.Bead(beadID), escapeLines(strings.TrimSpace(string(out))))
	}
	return nil
}

// escapeLines applies termsafe.Escape to each line of a (possibly
// multi-line) block of agent-influenced text — bd subprocess output —
// while preserving the real newlines that separate genuine lines (R4:
// per-line escaping for line-oriented bodies, never per-message, so a
// hostile line cannot forge additional lines while legitimate multi-line
// structure survives).
func escapeLines(s string) string {
	if s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = termsafe.Escape(l)
	}
	return strings.Join(lines, "\n")
}

// defaultActiveBead resolves the active bead (bd's single in_progress child,
// ADR-0023) and the epic owning the released bead, for the Step 6 cursor
// rewind. The active bead is read BEFORE Step 5 mutates state; the caller uses
// it to decide whether the released bead was the active one.
func defaultActiveBead(beadID string) (active, epicID string, ok bool) {
	epicID, _, err := phase.FindEpicForBead(beadID)
	if err != nil || epicID == "" {
		return "", "", false
	}
	// The active cursor is derived from the single in_progress child. Find it
	// by scanning the epic's children for the in_progress one.
	info, ferr := next.FetchBeadByID(beadID)
	if ferr != nil {
		return "", epicID, true
	}
	if strings.EqualFold(strings.TrimSpace(info.Status), "in_progress") {
		return beadID, epicID, true
	}
	return "", epicID, true
}

// runRelease performs the strict 6-step claim reversal (Spec 101 R2). All I/O
// is behind deps so the ordering, dirty-refuse/--force, and bead-state ACs are
// unit-testable; the cwd-safety AC is proven separately against the real
// executor (internal/executor, RemoveBeadWorktreeAndRestore).
func runRelease(deps releaseDeps, beadID string, force bool) error {
	beadID = strings.TrimSpace(beadID)
	if beadID == "" {
		return fmt.Errorf("release requires a bead ID")
	}

	// Step 1 (root + worktree path) is resolved by the caller into deps.

	// Step 2: dirty-check. Refuse (non-zero, NO removal) on user dirt unless
	// --force. --force is a mindspec-level PRE-GATE, decided HERE before
	// Remove — not a bd-flag passthrough (bead.WorktreeRemove is hardcoded
	// --force at the bd-CLI layer).
	if !force {
		userDirt, err := deps.checkDirty(deps.beadWorktreePath, deps.beadWorktreePath)
		if err != nil {
			return fmt.Errorf("checking bead worktree for uncommitted changes: %w", err)
		}
		if len(userDirt) > 0 {
			// R4: each porcelain entry is an agent-writable filename —
			// escape per-line; beadID is an ID-typed position (idrender).
			escapedUserDirt := make([]string, len(userDirt))
			for i, line := range userDirt {
				escapedUserDirt[i] = termsafe.Escape(line)
			}
			// Spec 127 R5(d)/(e), AC-9(ii)/AC-10(ii) (O2-r2-5, O3-r2-7): the
			// recovery: line carries ONLY the safe action (commit + re-run) —
			// never the --force discard. The discard is a distinct, separately-
			// labeled OPERATOR CHOICE folded into the message body instead: it
			// destroys only the operator's OWN uncommitted worktree changes, a
			// policy call no evidence can make for them (this is why it is not
			// constructor-derived and not merge-evidence-gated the way R1-R4's
			// hints are), so it belongs in the labeled-choice block, not the
			// machine-greppable recovery line an agent pastes reflexively. The
			// scan still covers this whole diagnostic body (O3-r2-7's "scope is
			// the whole emitted diagnostic" — relocating a destructive command
			// out of the recovery: position does not hide it from the sweep),
			// even though `mindspec release ... --force` itself matches no
			// reviewed floor family (classifier.go's own recorded {git,bd,rm}
			// program-set exclusion, O2-r2-13) — "the classifier still sees it"
			// names the scan's SCOPE, not a claim that this line is classified.
			msg := fmt.Sprintf(
				"cannot release bead %s: its worktree has uncommitted user changes:\n  %s\n"+
					"these may be your work in progress — release did NOT remove the worktree.\n"+
					"(.beads/issues.jsonl is auto-handled per ADR-0025 and never blocks)\n\n"+
					"Operator choice — discard instead: this is YOUR call, not one evidence can "+
					"make for you. To permanently DISCARD the uncommitted changes above rather "+
					"than keep them, re-run with `mindspec release %s --force`.",
				idrender.Bead(beadID), strings.Join(escapedUserDirt, "\n  "), idrender.Bead(beadID))
			return guard.NewFailure(msg,
				fmt.Sprintf("commit them (git add -A && git commit) then re-run `mindspec release %s`", idrender.Bead(beadID)))
		}
	}

	// Step 3 + 4: remove the bead worktree FIRST (via the executor, ADR-0030),
	// then chdir to root IMMEDIATELY after (spec-092 cwd-safety). Both happen
	// inside removeWorktree so the invariant cannot be reordered.
	if err := deps.removeWorktree(beadID); err != nil {
		// Remove-first / set-open-last: a removal failure leaves the bead
		// STILL CLAIMED (not yet open) — a recoverable state. Re-running
		// release converges once the worktree is gone.
		// R4: beadID is an ID-typed position — idrender.Bead, matching the
		// dirty-check branch above.
		return guard.NewFailure(
			fmt.Sprintf("removing the worktree for bead %s failed; the bead is left CLAIMED (recoverable).\ncause: %v", idrender.Bead(beadID), err),
			fmt.Sprintf("mindspec release %s", idrender.Bead(beadID)))
	}

	// Step 6 read: capture whether the released bead is the active one BEFORE
	// Step 5 mutates its status (set-open self-rewinds the derived cursor).
	active, epicID, haveCursor := deps.activeBead(beadID)

	// Step 5: set the bead back to open + clear assignee LAST. bd auto-commits
	// the write to durable Dolt state (embedded auto-commit mode).
	if err := deps.setOpen(beadID); err != nil {
		// The worktree is already gone; the bead is still claimed. This is the
		// recoverable "still-claimed, worktree-gone" state by design: a re-run
		// of release (worktree already gone → Remove is idempotent) or
		// `mindspec next` recovers.
		return guard.NewFailure(
			fmt.Sprintf("the bead %s worktree was removed but returning the bead to open failed (still-claimed, worktree-gone — recoverable).\ncause: %v", idrender.Bead(beadID), err),
			fmt.Sprintf("mindspec release %s", idrender.Bead(beadID)))
	}

	// Step 6 + 6.5: the cursor is DERIVED from bd's single in_progress child
	// (ADR-0023), so Step 5's set-open already self-rewinds the derived cursor.
	// Only sync the mindspec_phase metadata cache (the sanctioned redesign
	// leg, spec 128 Bead 2) when the released bead WAS the active one; a
	// non-active release leaves the cursor untouched. No value is
	// precomputed here — deps.syncPhase re-derives fresh inside its own
	// lock critical section (phase.SanctionedDerivedSync).
	if haveCursor && epicID != "" && active == beadID {
		deps.syncPhase(epicID)
	}

	fmt.Fprintf(deps.stdout, "Released bead %s.\nWorktree removed; bead returned to open (assignee cleared).\n", idrender.Bead(beadID))
	return nil
}

func init() {
	releaseCmd.Flags().Bool("force", false, "Remove the bead worktree even if it has uncommitted user changes (discards that work)")
}
