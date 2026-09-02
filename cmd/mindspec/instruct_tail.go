package main

import (
	"fmt"
	"os"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/instruct"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/resolve"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/workspace/containment"
)

// emitInstruct derives state from beads and prints mode-appropriate guidance.
// This is the "instruct-tail" convention: every state-changing command
// (approve, next, complete) calls this after transitioning to emit
// guidance for the new mode.
//
// root is the main repo root (for spec dirs and guard).
func emitInstruct(root string) error {
	// ADR-0023: derive state from beads, not focus files.
	ctx, _ := phase.ResolveContext(root)

	// Spec 128 R2.3: a genuine bd/Dolt derivation failure, or an empty
	// Phase (multi-active-spec ambiguity — the only non-error way Phase
	// stays "" per ResolveContextFromDirWithCache), renders the honest
	// indeterminate form — NEVER idle.md. A failure to derive state is
	// never presented as a derived idle state.
	if ctx == nil || ctx.DerivationFailed || ctx.Phase == "" {
		reason := "no single active spec/bead could be resolved (ambiguous, or discovery failed)"
		if ctx != nil && ctx.DerivationFailed {
			reason = ctx.DerivationError
		}
		return renderIndeterminateTail(root, reason)
	}

	mc := &state.Focus{
		Mode:       ctx.Phase,
		ActiveSpec: ctx.SpecID,
		ActiveBead: ctx.BeadID,
	}
	if ctx.WorktreePath != "" {
		mc.ActiveWorktree = ctx.WorktreePath
	}

	// Bead 5 extension window (a): a fresh pre-epic draft (e.g. right
	// after `spec create`, invoked from the main checkout) derives
	// bd-empty idle above — bd has no trace of a draft until `spec
	// approve` creates its epic. Consult the same on-disk/branch draft
	// scan bead 3's `state show` already applies before finalizing idle,
	// so this tail agrees with `state show` for a draft instead of
	// reporting idle while state show reports spec.
	if mc.Mode == state.ModeIdle && mc.ActiveSpec == "" {
		drafts, draftErr := resolve.DiscoverDraftSpecsWithCache(phase.NewCache(), root)
		switch {
		case draftErr != nil:
			return renderIndeterminateTail(root, draftErr.Error())
		case len(drafts) == 1:
			mc = &state.Focus{Mode: state.ModeSpec, ActiveSpec: drafts[0].SpecID}
		case len(drafts) > 1:
			return renderIndeterminateTail(root, fmt.Sprintf("%d pre-epic spec drafts found with no active spec resolved; ambiguous target", len(drafts)))
		}
	}

	// CWD redirect: if on main with active worktree, emit redirect only.
	if wtPath := guard.ActiveWorktreePath(root); wtPath != "" && guard.IsMainCWD(root) {
		fmt.Fprintf(os.Stdout, "\n## Worktree Redirect\n\nYou are in the main worktree. Switch to:\n\n  %s\n\nThen run `mindspec instruct` for mode-appropriate guidance.\n", containment.EmitCd(wtPath))
		return nil
	}

	iCtx := instruct.BuildContext(root, mc)

	// Add worktree check when an active worktree is set.
	if mc.ActiveWorktree != "" {
		if warning := instruct.CheckWorktree(mc.ActiveWorktree); warning != "" {
			iCtx.Warnings = append(iCtx.Warnings, "[worktree] "+warning)
		}
	}

	output, err := instruct.Render(iCtx)
	if err != nil {
		return fmt.Errorf("rendering guidance: %w", err)
	}

	fmt.Fprint(os.Stdout, output)
	return nil
}

// renderIndeterminateTail prints the shared indeterminate-mode guidance
// (spec 128 R2.3) to stdout — reason names the discovery failure or
// unresolved ambiguity. The tail never fails the verb it trails: a
// rendering error is the only error this returns, matching emitInstruct's
// own contract.
func renderIndeterminateTail(root, reason string) error {
	output, err := instruct.RenderIndeterminate(root, reason)
	if err != nil {
		return fmt.Errorf("rendering guidance: %w", err)
	}
	fmt.Fprint(os.Stdout, output)
	return nil
}
