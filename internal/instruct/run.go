package instruct

import (
	"context"
	"fmt"
	"io"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/resolve"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/trace"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Run derives mode from beads state and writes mode-appropriate guidance.
//
// ctx is honored at step boundaries — if it is canceled or expires between
// stages, Run returns ctx.Err() without continuing the pipeline. This keeps
// goroutine leaks bounded to a single in-flight workspace/beads call when
// callers (e.g. the SessionStart hook) impose a deadline.
//
// cwd is the working directory used for workspace resolution (caller passes
// os.Getwd() for the CLI; the hook passes the already-resolved repo root).
// format is "" (markdown, default) or "json".
// specFlag is the optional --spec target ("" → auto-detect single active spec).
// out receives the rendered output (stdout for CLI; same for hook).
//
// Returns an error only on unrecoverable failures (workspace lookup, render
// errors, ambiguous target with multiple actives). Idle / no-state is NOT
// an error — it renders the idle template.
func Run(ctx context.Context, cwd, format, specFlag string, out io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	localRoot, err := workspace.FindLocalRoot(cwd)
	if err != nil {
		return err
	}
	// mainRoot resolves worktrees back to the main repo (for guard, spec lookup).
	mainRoot, _ := workspace.FindRoot(cwd)
	if mainRoot == "" {
		mainRoot = localRoot
	}

	// Protected branch check FIRST: main/master → always idle.
	// This must run before guard/worktree checks which query beads (slow dolt cold start).
	if specFlag == "" {
		if _, ok := RenderIdleIfProtected(mainRoot); ok {
			return handleNoState(mainRoot, format, out)
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	// CWD redirect: if running from main with an active worktree,
	// emit ONLY the redirect message — no normal guidance.
	if wtPath := guard.ActiveWorktreePath(mainRoot); wtPath != "" && guard.IsMainCWD(mainRoot) {
		msg := fmt.Sprintf("# MindSpec — CWD Redirect\n\nYou are in the main worktree. Run:\n\n  cd %s\n\nThen run `mindspec instruct` for mode-appropriate guidance.\n", wtPath)
		if format == "json" {
			fmt.Fprintf(out, `{"redirect":true,"worktree_path":%q,"message":"Switch to worktree"}`, wtPath)
			fmt.Fprintln(out)
		} else {
			fmt.Fprint(out, msg)
		}
		return nil
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	// ADR-0023: derive state from beads, not focus files.
	// First try resolver for spec targeting, then use phase context.
	specID, resolveErr := resolve.ResolveTarget(mainRoot, specFlag)

	var mc *state.Focus
	if resolveErr != nil {
		if ambErr, ok := resolveErr.(*resolve.ErrAmbiguousTarget); ok {
			return handleAmbiguous(mainRoot, format, out, ambErr)
		}
		// Try phase context for beads-derived state.
		pctx, ctxErr := phase.ResolveContext(mainRoot)
		if ctxErr != nil || pctx == nil || pctx.Phase == "" {
			return handleNoState(mainRoot, format, out)
		}
		mc = &state.Focus{
			Mode:       pctx.Phase,
			ActiveSpec: pctx.SpecID,
			ActiveBead: pctx.BeadID,
		}
	} else {
		// Derive mode from beads
		mode, _ := resolve.ResolveMode(mainRoot, specID)
		// Try to find active bead via phase context
		pctx, _ := phase.ResolveContextFromDir(mainRoot, localRoot)
		activeBead := ""
		if pctx != nil {
			activeBead = pctx.BeadID
		}
		mc = &state.Focus{
			Mode:       mode,
			ActiveSpec: specID,
			ActiveBead: activeBead,
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	// ADR-0023: ActiveWorktree is no longer stored in focus files.
	// Resolve it from git worktree list if we have an active bead.
	if mc.ActiveBead != "" && mc.ActiveWorktree == "" {
		mc.ActiveWorktree = resolveBeadWorktree(mc.ActiveBead)
	}

	bctx := BuildContext(mainRoot, mc)

	// Add worktree check when an active worktree is set.
	if mc.ActiveWorktree != "" {
		if warning := CheckWorktree(mc.ActiveWorktree); warning != "" {
			bctx.Warnings = append(bctx.Warnings, "[worktree] "+warning)
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if format == "json" {
		output, err := RenderJSON(bctx)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, output)
		return nil
	}

	output, err := Render(bctx)
	if err != nil {
		return err
	}
	trace.Emit(trace.NewEvent("instruct.render").
		WithSpec(mc.ActiveSpec).
		WithTokens(trace.EstimateTokens(output)).
		WithData(map[string]any{
			"tokens_total": trace.EstimateTokens(output),
			"mode":         mc.Mode,
			"template":     mc.Mode + ".md",
		}))
	fmt.Fprint(out, output)
	return nil
}

// handleNoState provides a graceful fallback when no state exists.
func handleNoState(root, format string, out io.Writer) error {
	mc := &state.Focus{Mode: state.ModeIdle}
	ctx := BuildContext(root, mc)
	// No warning needed — the idle template already tells the agent what to do.

	if format == "json" {
		output, err := RenderJSON(ctx)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, output)
		return nil
	}

	output, err := Render(ctx)
	if err != nil {
		return err
	}
	fmt.Fprint(out, output)
	return nil
}

// handleAmbiguous renders the ambiguous template listing all active specs.
func handleAmbiguous(root, format string, out io.Writer, ambErr *resolve.ErrAmbiguousTarget) error {
	mc := &state.Focus{Mode: "ambiguous"}
	ctx := BuildContext(root, mc)
	for _, s := range ambErr.Active {
		ctx.ActiveSpecList = append(ctx.ActiveSpecList, SpecInfo{
			SpecID: s.SpecID,
			Mode:   s.Mode,
		})
	}

	if format == "json" {
		output, err := RenderJSON(ctx)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, output)
		return nil
	}

	output, err := Render(ctx)
	if err != nil {
		return err
	}
	fmt.Fprint(out, output)
	return nil
}

// resolveBeadWorktree finds the worktree path for a bead by checking
// git worktree list for a matching bead branch or worktree name.
func resolveBeadWorktree(beadID string) string {
	entries, err := bead.WorktreeList()
	if err != nil {
		return ""
	}
	wtName := workspace.BeadWorktreeName(beadID)
	branchName := workspace.BeadBranch(beadID)
	for _, e := range entries {
		if e.Name == wtName || e.Branch == branchName {
			return e.Path
		}
	}
	return ""
}
