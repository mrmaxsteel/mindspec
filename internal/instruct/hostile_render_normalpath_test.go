package instruct

// Spec 120 R4 cluster 1 (round-5 fix-up): the single-active/normal
// instruct path (BuildContextWithCache, instruct.go) renders
// Context.ActiveSpec/ActiveBead/ActiveWorktree into every mode template
// (spec.md, plan.md, implement.md, review.md) — a DIFFERENT render
// surface from the already-fixed ambiguous-path seam (handleAmbiguous,
// pinned in hostile_render_test.go). ActiveSpec/ActiveBead are ID-typed
// positions (idrender.Spec/idrender.Bead); ActiveWorktree is free text,
// split into a RAW field (feeds implement.md's `shellsafe`-piped `cd`
// operand) and an escaped ActiveWorktreeDisplay field (feeds the
// display-only "Active Worktree" line) so escaping the display copy can
// never mangle the executable cd line.

import (
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/workspace/containment"
)

// hostileIDLikeSuffix is a printable-ASCII, metacharacter-bearing suffix
// that is malformed per idvalidate's grammar but passes termsafe.Escape
// unchanged (the idrender_test.go "120-x;evil" discriminator) — proving
// the ID-typed positions below need idrender, not just termsafe.
const hostileIDLikeSuffix = ";evil"

func TestBuildContext_HostileActiveSpecForcedQuoted_AcrossModes(t *testing.T) {
	root := setupTestProject(t)
	hostileSpec := "004-instruct" + hostileIDLikeSuffix
	wantQuoted := strconv.Quote(hostileSpec)

	for _, mode := range []string{state.ModeSpec, state.ModePlan, state.ModeReview} {
		t.Run(mode, func(t *testing.T) {
			s := &state.Focus{Mode: mode, ActiveSpec: hostileSpec}
			ctx := BuildContext(root, s)
			if ctx.ActiveSpec != wantQuoted {
				t.Fatalf("ctx.ActiveSpec = %q, want forced-quoted %q", ctx.ActiveSpec, wantQuoted)
			}
			out, err := Render(ctx)
			if err != nil {
				t.Fatalf("Render failed: %v", err)
			}
			assertCleanRender(t, out)
			if !strings.Contains(out, wantQuoted) {
				t.Errorf("%s template must render the forced-quoted hostile spec ID, got:\n%s", mode, out)
			}
			if strings.Contains(out, "`"+hostileSpec+"`") {
				t.Errorf("%s template rendered the hostile spec ID RAW (unquoted):\n%s", mode, out)
			}
		})
	}
}

func TestBuildContext_HostileActiveBeadForcedQuoted_ImplementMode(t *testing.T) {
	root := setupTestProject(t)
	hostileBead := "mindspec-9cyu.1" + hostileIDLikeSuffix
	wantQuoted := strconv.Quote(hostileBead)

	s := &state.Focus{Mode: state.ModeImplement, ActiveSpec: "004-instruct", ActiveBead: hostileBead}
	ctx := BuildContext(root, s)
	if ctx.ActiveBead != wantQuoted {
		t.Fatalf("ctx.ActiveBead = %q, want forced-quoted %q", ctx.ActiveBead, wantQuoted)
	}
	out, err := Render(ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	assertCleanRender(t, out)
	if !strings.Contains(out, wantQuoted) {
		t.Errorf("implement.md must render the forced-quoted hostile bead ID, got:\n%s", out)
	}
}

// TestBuildContext_CleanActiveSpecBead_ByteIdentical is the clean-fixture
// counterpart (F3 discipline): genuine IDs render byte-identically
// through the normal-path seam, exactly as before this fix.
func TestBuildContext_CleanActiveSpecBead_ByteIdentical(t *testing.T) {
	root := setupTestProject(t)
	const cleanSpec = "004-instruct"
	const cleanBead = "mindspec-9cyu.1"

	s := &state.Focus{Mode: state.ModeImplement, ActiveSpec: cleanSpec, ActiveBead: cleanBead}
	ctx := BuildContext(root, s)
	if ctx.ActiveSpec != cleanSpec {
		t.Errorf("ctx.ActiveSpec = %q, want byte-identical %q", ctx.ActiveSpec, cleanSpec)
	}
	if ctx.ActiveBead != cleanBead {
		t.Errorf("ctx.ActiveBead = %q, want byte-identical %q", ctx.ActiveBead, cleanBead)
	}

	out, err := Render(ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if !strings.Contains(out, "`"+cleanSpec+"`") {
		t.Errorf("clean spec ID must render byte-identically, got:\n%s", out)
	}
	if !strings.Contains(out, "`"+cleanBead+"`") {
		t.Errorf("clean bead ID must render byte-identically, got:\n%s", out)
	}
}

// TestImplementTemplate_ActiveWorktree_DisplayEscapedCdOperandFunctional
// pins the ActiveWorktree SPLIT (R4 cluster 1): the display-only "Active
// Worktree" line escapes a control-byte-bearing worktree path, while the
// executable `cd` line's shellsafe-piped operand keeps consuming the RAW
// path — never the escaped copy. Feeding the escaped copy into shellsafe
// would additionally single-quote strconv.Quote's own double-quoted
// literal, producing a visibly wrong cd line; this test's "exactly one
// shellsafe transform" assertion catches that regression directly.
func TestImplementTemplate_ActiveWorktree_DisplayEscapedCdOperandFunctional(t *testing.T) {
	root := setupTestProject(t)
	hostileWorktree := "/repo/.worktrees/worktree-bead\x1b[31mFAKE\x1b[0m x"

	s := &state.Focus{Mode: state.ModeImplement, ActiveSpec: "004-instruct", ActiveBead: "mindspec-9cyu.1", ActiveWorktree: hostileWorktree}
	ctx := BuildContext(root, s)

	// The RAW field must survive unchanged (it feeds the cd operand).
	if ctx.ActiveWorktree != hostileWorktree {
		t.Fatalf("ctx.ActiveWorktree (raw) = %q, want unchanged %q", ctx.ActiveWorktree, hostileWorktree)
	}
	wantDisplay := strconv.Quote(hostileWorktree)
	if ctx.ActiveWorktreeDisplay != wantDisplay {
		t.Fatalf("ctx.ActiveWorktreeDisplay = %q, want forced-quoted %q", ctx.ActiveWorktreeDisplay, wantDisplay)
	}

	out, err := Render(ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	wantDisplayLine := "**Active Worktree**: `" + wantDisplay + "`"
	if !strings.Contains(out, wantDisplayLine) {
		t.Errorf("expected the escaped display line %q, got:\n%s", wantDisplayLine, out)
	}
	// The DISPLAY line specifically must never carry the raw ESC byte —
	// that is this test's falsifier for the display-side escape.
	if strings.ContainsRune(wantDisplayLine, 0x1b) {
		t.Fatalf("test invariant broken: wantDisplayLine itself contains a raw ESC byte: %q", wantDisplayLine)
	}

	// The cd operand is FUNCTIONAL: it must stay the RAW path run through
	// containment.ShellSafe's conditional single-quoting (never
	// termsafe.Escape's strconv.Quote), so the raw ESC byte legitimately
	// survives INSIDE the shell-safe single quotes here — that is the
	// documented split, not a leak.
	wantCdOperand := containment.ShellSafe(hostileWorktree)
	wantCdLine := "Run `cd " + wantCdOperand + "` to enter the bead worktree."
	if !strings.Contains(out, wantCdLine) {
		t.Errorf("expected the RAW-then-shellsafe cd line %q, got:\n%s", wantCdLine, out)
	}
	// The escaped display copy must NOT have been fed through shellsafe
	// (that would double-transform it): the cd line's operand must not
	// contain the escaped copy's leading double-quote.
	if strings.Contains(out, "cd '"+wantDisplay) {
		t.Errorf("cd operand appears to have consumed the ESCAPED copy instead of the raw path:\n%s", out)
	}
}

// TestImplementTemplate_ActiveWorktree_CleanByteIdentical is the
// clean-fixture counterpart: an ordinary worktree path (no control bytes)
// renders byte-identically in both the display line and the cd operand,
// unchanged from before this fix.
func TestImplementTemplate_ActiveWorktree_CleanByteIdentical(t *testing.T) {
	root := setupTestProject(t)
	const cleanWorktree = "/repo/.worktrees/worktree-bead-abc"

	s := &state.Focus{Mode: state.ModeImplement, ActiveSpec: "004-instruct", ActiveBead: "mindspec-9cyu.1", ActiveWorktree: cleanWorktree}
	ctx := BuildContext(root, s)
	if ctx.ActiveWorktree != cleanWorktree {
		t.Errorf("ctx.ActiveWorktree = %q, want byte-identical %q", ctx.ActiveWorktree, cleanWorktree)
	}
	if ctx.ActiveWorktreeDisplay != cleanWorktree {
		t.Errorf("ctx.ActiveWorktreeDisplay = %q, want byte-identical %q", ctx.ActiveWorktreeDisplay, cleanWorktree)
	}

	out, err := Render(ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if !strings.Contains(out, "**Active Worktree**: `"+cleanWorktree+"`") {
		t.Errorf("clean worktree path must render byte-identically in the display line, got:\n%s", out)
	}
	if !strings.Contains(out, "Run `cd "+cleanWorktree+"` to enter the bead worktree.") {
		t.Errorf("clean worktree path must render byte-identically in the cd operand, got:\n%s", out)
	}
}
