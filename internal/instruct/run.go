package instruct

import (
	"context"
	"fmt"
	"io"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/resolve"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/trace"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Options tunes a Run invocation. The zero value reproduces the bare
// `mindspec instruct` behavior; PanelState opts into the Spec 093
// Req 14 open-panel-rounds block.
type Options struct {
	// PanelState, when true, gathers and renders the open-panel-rounds
	// block (Spec 093 Req 14) for the active bead's scan roots and
	// appends it to the output (markdown + JSON `panel_state`). This is
	// the explicit `--panel-state` flag path; the SessionStart hook sets
	// it conditionally for implement-mode auto-include (Req 15).
	PanelState bool
}

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
//
// PERF-1: one phase.Cache per Run invocation is shared across the guard /
// resolve / phase / instruct / state stack so a warm `mindspec instruct`
// makes ≤3 bd calls (≤4 with an active bead via state.checkBeadStatus).
// The cache is allocated before the guard.ActiveWorktreePathWithCache call
// so that lookup shares its `bd list --type=epic` with the rest of the
// invocation.
func Run(ctx context.Context, cwd, format, specFlag string, out io.Writer) error {
	return RunWithOptions(ctx, cwd, format, specFlag, out, Options{})
}

// RunWithOptions is Run with explicit Options (Spec 093 Req 14). Run is
// the Options{}-zero-value wrapper; all existing callers keep working.
func RunWithOptions(ctx context.Context, cwd, format, specFlag string, out io.Writer, opts Options) error {
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

	// PERF-1: per-invocation cache, shared by every cache-aware helper below.
	cache := phase.NewCache()

	// Protected branch check FIRST: main/master → always idle.
	// This must run before guard/worktree checks which query beads (slow dolt cold start).
	if specFlag == "" {
		if _, ok := RenderIdleIfProtected(mainRoot); ok {
			return handleNoState(cache, mainRoot, format, out)
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	// CWD redirect: if running from main with an active worktree,
	// emit ONLY the redirect message — no normal guidance.
	if wtPath := guard.ActiveWorktreePathWithCache(cache, mainRoot); wtPath != "" && guard.IsMainCWDWithCache(cache, mainRoot) {
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
	specID, resolveErr := resolve.ResolveTargetWithCache(cache, mainRoot, specFlag)

	var mc *state.Focus
	if resolveErr != nil {
		if ambErr, ok := resolveErr.(*resolve.ErrAmbiguousTarget); ok {
			return handleAmbiguous(cache, mainRoot, format, out, ambErr)
		}
		// Try phase context for beads-derived state.
		pctx, ctxErr := phase.ResolveContextWithCache(cache, mainRoot)
		if ctxErr != nil || pctx == nil || pctx.Phase == "" {
			return handleNoState(cache, mainRoot, format, out)
		}
		mc = &state.Focus{
			Mode:       pctx.Phase,
			ActiveSpec: pctx.SpecID,
			ActiveBead: pctx.BeadID,
		}
	} else {
		// Derive mode from beads. Single cache-shared path:
		//   - resolve.ResolveModeWithCache → cache.FindEpicBySpecID + cache.FindEpic
		//   - phase.ResolveContextFromDirWithCache → reuses cached epic/children
		mode, _ := resolve.ResolveModeWithCache(cache, mainRoot, specID)
		// Try to find active bead via phase context
		pctx, _ := phase.ResolveContextFromDirWithCache(cache, mainRoot, localRoot)
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

	bctx := BuildContextWithCache(cache, mainRoot, mc)

	// Add worktree check when an active worktree is set.
	if mc.ActiveWorktree != "" {
		if warning := CheckWorktree(mc.ActiveWorktree); warning != "" {
			bctx.Warnings = append(bctx.Warnings, "[worktree] "+warning)
		}
	}

	// Spec 093 Req 14: when --panel-state is requested, gather the full
	// Panel/Subagent State block — in-progress beads (capped git detail),
	// open panel rounds (per-panel tally vs the complete gate, with each
	// bead panel's live branch SHA resolved for staleness), and stale
	// agent worktrees — from the active bead's scan roots (worktree +
	// main root, union/deduped). All three render empty when nothing is
	// open → Render appends nothing (zero-cost contract, Req 15). All
	// git/bd subprocesses fire ONLY inside this branch — the Req 15
	// stub-guard (the SessionStart hook gates entry on the fs-only
	// HasIncompletePanel, so a panel-less session pays zero added cost).
	if opts.PanelState {
		bctx.PanelState = buildPanelStateBlock(cache, mainRoot, mc.ActiveWorktree, mc.ActiveBead)
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
func handleNoState(cache *phase.Cache, root, format string, out io.Writer) error {
	mc := &state.Focus{Mode: state.ModeIdle}
	ctx := BuildContextWithCache(cache, root, mc)
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
func handleAmbiguous(cache *phase.Cache, root, format string, out io.Writer, ambErr *resolve.ErrAmbiguousTarget) error {
	mc := &state.Focus{Mode: "ambiguous"}
	ctx := BuildContextWithCache(cache, root, mc)
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

// panelScanRoots returns the deduped scan roots for --panel-state: the
// active bead worktree (where /ms-panel-run writes review/) plus the
// main root. panel.Scan dedupes by resolved path, so passing both even
// when they coincide is safe. Empty entries are dropped by Scan.
func panelScanRoots(mainRoot, activeWorktree string) []string {
	roots := make([]string, 0, 2)
	if activeWorktree != "" {
		roots = append(roots, activeWorktree)
	}
	if mainRoot != "" {
		roots = append(roots, mainRoot)
	}
	return roots
}

// liveBranchSHA resolves `git rev-parse bead/<beadID>` for the panel
// staleness check. exists == false signals a deleted branch (the
// rerun-after-merge Pass-through, Spec 093 Req 11) — the only git work
// the panel-state path performs beyond the fs Scan (ADR-0030 budget).
func liveBranchSHA(beadID string) (sha string, exists bool) {
	branch := workspace.BeadBranch(beadID)
	s, err := gitutil.RevParseRef("", branch)
	if err != nil {
		return "", false
	}
	return s, true
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
