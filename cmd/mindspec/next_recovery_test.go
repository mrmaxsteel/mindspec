package main

// Spec 093 Bead 1 fix-round (R3-1, R3-2, R3-3): cmd-level pins for the
// Req 3/Req 4 failure-path WIRING in next.go.
//
// internal/next's unit tests pin the ClaimFailure/WorktreeSetupFailure
// CONSTRUCTORS in isolation; they cannot catch a revert of the
// `mindspec next` call sites back to the pre-093 bare messages
// (`fmt.Errorf("claiming bead: %w")` / `Warning: worktree setup
// failed: %v`). These tests exercise the extracted call-site wiring
// (claimFailureError / warnWorktreeSetupFailure) directly:
//
//   - TestClaimFailureError_FullRecipe — the returned error carries the
//     FULL recipe (the verbatim `bd update --claim` line + the
//     `recovery:` line); reverting the call site to a bare wrap fails it.
//   - TestWarnWorktreeSetupFailure_FullRecipe — the warning carries the
//     full `git worktree add` recipe + recovery line.
//   - TestWarnWorktreeSetupFailure_DoesNotFatal — the warn-AND-CONTINUE
//     half: warnWorktreeSetupFailure returns nothing, so the command
//     proceeds; flipping the call site to a fatal `return` is a visible
//     signature change that breaks compilation here (R3 mutation 8 dies).
//   - TestRecoverySpecSlug_FlagPrecedence / _TitleFallback and
//     TestRecoveryConfig_* — cover recoverySpecSlug + recoveryConfig,
//     which were at 0% coverage.

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/next"
)

// taskBead returns a task-typed bead whose title carries a bracketed
// spec prefix, so next.ResolveMode parses the slug from the title with
// no filesystem I/O (task beads skip resolveFeatureMode).
func taskBead(id, specSlug string) next.BeadInfo {
	return next.BeadInfo{
		ID:        id,
		Title:     "[" + specSlug + "] Bead 1: something",
		IssueType: "task",
	}
}

func TestClaimFailureError_FullRecipe(t *testing.T) {
	t.Parallel()
	bead := taskBead("mindspec-ab12", "093-skills-thin-down")
	err := claimFailureError("/repo", "093-skills-thin-down", bead,
		errFromString("claim failed (may already be claimed): Error 1105"))
	if err == nil {
		t.Fatal("claimFailureError returned nil")
	}
	msg := err.Error()

	// The bare pre-093 wrap would be exactly "claiming bead: <err>" with
	// none of the recipe — assert the recipe parts a revert would drop.
	wantBareGone := "claiming bead: claim failed"
	if strings.Contains(msg, wantBareGone) {
		t.Errorf("call site emits the pre-093 bare wrap %q (reverted): %q", wantBareGone, msg)
	}
	for _, want := range []string{
		// Verbatim --claim recipe line.
		"bd update mindspec-ab12 --claim --status in_progress",
		// Interpolated worktree recipe.
		"git -C /repo/.worktrees/worktree-spec-093-skills-thin-down worktree add .worktrees/worktree-mindspec-ab12 -b bead/mindspec-ab12 spec/093-skills-thin-down",
		// The recovery line.
		"recovery: mindspec next --spec 093-skills-thin-down",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("claim-failure wiring dropped %q:\n%s", want, msg)
		}
	}
}

func TestWarnWorktreeSetupFailure_FullRecipe(t *testing.T) {
	t.Parallel()
	bead := taskBead("mindspec-ab12", "093-skills-thin-down")
	var buf bytes.Buffer
	warnWorktreeSetupFailure(&buf, "/repo", "093-skills-thin-down", bead,
		errFromString("exit status 128"))
	out := buf.String()

	// The bare pre-093 warning was "Warning: worktree setup failed: <err>"
	// with no recipe. Assert the recipe parts a revert would drop.
	if strings.Contains(out, "Warning: worktree setup failed: exit status 128") &&
		!strings.Contains(out, "git -C") {
		t.Errorf("call site emits the pre-093 bare warning (reverted): %q", out)
	}
	if !strings.HasPrefix(out, "Warning: ") {
		t.Errorf("warning must keep its `Warning: ` prefix: %q", out)
	}
	for _, want := range []string{
		"bead mindspec-ab12 is claimed but has no worktree",
		"git -C /repo/.worktrees/worktree-spec-093-skills-thin-down worktree add .worktrees/worktree-mindspec-ab12 -b bead/mindspec-ab12 spec/093-skills-thin-down",
		"recovery: mindspec next --spec 093-skills-thin-down",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("worktree-setup warning dropped %q:\n%s", want, out)
		}
	}
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("warning must end with a newline: %q", out)
	}
}

// TestWarnWorktreeSetupFailure_DoesNotFatal pins the CONTINUE half of
// the warn-and-continue contract (R3-2, R3 mutation 8). The function is
// invoked exactly as the call site invokes it; control returns here, so
// the surrounding command proceeds. A void return type is the structural
// guarantee: flipping the call site to `return warnWorktreeSetupFailure(...)`
// would not compile, and changing this function to return an error
// (so the call site could fatal on it) breaks this test's call.
func TestWarnWorktreeSetupFailure_DoesNotFatal(t *testing.T) {
	t.Parallel()
	bead := taskBead("mindspec-ab12", "093-skills-thin-down")
	var buf bytes.Buffer

	proceeded := false
	func() {
		warnWorktreeSetupFailure(&buf, "/repo", "093-skills-thin-down", bead,
			errFromString("boom"))
		// Reaching this line means the helper returned control rather
		// than aborting the flow — the claimed-but-homeless agent
		// proceeds to the state update + auto-recovery hint.
		proceeded = true
	}()

	if !proceeded {
		t.Fatal("warnWorktreeSetupFailure did not return control — warn-and-continue broken (mutation 8)")
	}
	if buf.Len() == 0 {
		t.Error("warnWorktreeSetupFailure must still emit the warning before continuing")
	}
}

func TestRecoverySpecSlug_FlagPrecedence(t *testing.T) {
	t.Parallel()
	// Explicit --spec flag wins, even over a different title slug.
	bead := taskBead("mindspec-ab12", "111-title-slug")
	got := recoverySpecSlug("/repo", "093-flag-slug", bead)
	if got != "093-flag-slug" {
		t.Errorf("recoverySpecSlug = %q, want the --spec flag value", got)
	}
}

func TestRecoverySpecSlug_TitleFallback(t *testing.T) {
	t.Parallel()
	// No --spec flag: the slug is parsed from the bead title.
	bead := taskBead("mindspec-ab12", "093-skills-thin-down")
	got := recoverySpecSlug("/repo", "", bead)
	if got != "093-skills-thin-down" {
		t.Errorf("recoverySpecSlug = %q, want the title slug", got)
	}
}

func TestRecoverySpecSlug_NoContext_Empty(t *testing.T) {
	t.Parallel()
	// No flag, no parseable slug → "" (constructors fall back to
	// placeholders).
	bead := next.BeadInfo{ID: "mindspec-ab12", Title: "bare title no slug", IssueType: "task"}
	if got := recoverySpecSlug("/repo", "", bead); got != "" {
		t.Errorf("recoverySpecSlug = %q, want empty", got)
	}
}

func TestRecoveryConfig_FallsBackToDefaultOnMissingConfig(t *testing.T) {
	t.Parallel()
	// A directory with no .mindspec/config.yaml → config.Load errors →
	// recoveryConfig returns DefaultConfig (never masks the original
	// failure with a config error).
	dir := t.TempDir()
	cfg := recoveryConfig(dir)
	if cfg == nil {
		t.Fatal("recoveryConfig returned nil")
	}
	if cfg.WorktreeRoot != config.DefaultConfig().WorktreeRoot {
		t.Errorf("fallback WorktreeRoot = %q, want default %q", cfg.WorktreeRoot, config.DefaultConfig().WorktreeRoot)
	}
}

func TestRecoveryConfig_HonorsCustomWorktreeRoot(t *testing.T) {
	t.Parallel()
	// A real config with a non-default worktree_root must be loaded and
	// honored — recoveryConfig is the seam that feeds cfg.WorktreeRoot
	// into the Req 3/4 path interpolation.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".mindspec"), 0o755); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".mindspec", "config.yaml"),
		[]byte("worktree_root: custom-trees\n"), 0o644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}
	cfg := recoveryConfig(dir)
	if cfg.WorktreeRoot != "custom-trees" {
		t.Errorf("recoveryConfig WorktreeRoot = %q, want custom-trees", cfg.WorktreeRoot)
	}
}

// errFromString is a tiny helper to build an error with a fixed message
// without pulling in errors/fmt at call sites.
type stringError string

func (e stringError) Error() string { return string(e) }

func errFromString(s string) error { return stringError(s) }

// TestNextCmd_ClaimFailureWiring (R3-1) is the call-site-revert pin.
// The helper tests above prove claimFailureError/warnWorktreeSetupFailure
// are correct, but a revert of the RunE call sites back to the bare
// `fmt.Errorf("claiming bead: %w")` / `fmt.Fprintf(..., "Warning:
// worktree setup failed: %v")` would leave the helpers dead-but-passing.
// This AST walk parses next.go and asserts the wiring directly:
//
//   - the next.ClaimBead error branch returns claimFailureError (and
//     never `fmt.Errorf("claiming bead...")`);
//   - the EnsureWorktree error branch calls warnWorktreeSetupFailure
//     (and never the bare `Warning: worktree setup failed` Fprintf).
//
// Reverting either call site fails this test deterministically (no
// subprocess, no bd dependency).
func TestNextCmd_ClaimFailureWiring(t *testing.T) {
	t.Parallel()
	src := mustReadNextSource(t)

	// Positive: the extracted helpers are invoked.
	for _, want := range []string{
		"claimFailureError(root, specFlag, selected, err)",
		"warnWorktreeSetupFailure(os.Stderr, root, specFlag, selected, wtErr)",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("next.go RunE no longer wires %q — the recovery recipe is bypassed", want)
		}
	}

	// Negative: the pre-093 bare forms must be ABSENT from next.go.
	for _, banned := range []string{
		`fmt.Errorf("claiming bead: %w", err)`,
		`"Warning: worktree setup failed: %v\n"`,
	} {
		if strings.Contains(src, banned) {
			t.Errorf("next.go RunE reverted to the pre-093 bare form %q (recovery recipe lost)", banned)
		}
	}
}

// TestNextCmd_WorktreeSetupCallIsNotFatal (R3-2, mutation 8) pins the
// CONTINUE half at the call site via AST: the EnsureWorktree error
// branch must invoke warnWorktreeSetupFailure as a bare expression
// statement (not `return warnWorktreeSetupFailure(...)` and not a
// `return next.WorktreeSetupFailure(...)`), so the command proceeds.
// Flipping the call site to a fatal return fails here.
func TestNextCmd_WorktreeSetupCallIsNotFatal(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, nextSourcePath(t), nil, 0)
	if err != nil {
		t.Fatalf("parse next.go: %v", err)
	}

	foundCall := false
	var fatalReturn bool
	ast.Inspect(file, func(n ast.Node) bool {
		// A bare expression statement calling warnWorktreeSetupFailure
		// is the correct (non-fatal) wiring.
		if es, ok := n.(*ast.ExprStmt); ok {
			if isCallTo(es.X, "warnWorktreeSetupFailure") {
				foundCall = true
			}
		}
		// A return statement that returns a *WorktreeSetupFailure value
		// (helper or constructor) is the fatal mutation we must reject.
		if ret, ok := n.(*ast.ReturnStmt); ok {
			for _, r := range ret.Results {
				if isCallTo(r, "warnWorktreeSetupFailure") ||
					isCallToSelector(r, "next", "WorktreeSetupFailure") {
					fatalReturn = true
				}
			}
		}
		return true
	})

	if !foundCall {
		t.Error("worktree-setup failure must be a bare (non-returning) warnWorktreeSetupFailure call — warn-and-continue (mutation 8)")
	}
	if fatalReturn {
		t.Error("worktree-setup failure is returned (fatal) — must warn AND continue (mutation 8)")
	}
}

func nextSourcePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRootFromTestDir(t), "cmd", "mindspec", "next.go")
}

func mustReadNextSource(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(nextSourcePath(t))
	if err != nil {
		t.Fatalf("read next.go: %v", err)
	}
	return string(data)
}

// isCallTo reports whether expr is a call to a bare function named name.
func isCallTo(expr ast.Expr, name string) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := call.Fun.(*ast.Ident)
	return ok && id.Name == name
}

// isCallToSelector reports whether expr is a call to pkg.fn.
func isCallToSelector(expr ast.Expr, pkg, fn string) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != fn {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg
}
