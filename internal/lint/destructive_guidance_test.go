// destructive_guidance_test.go — spec 127 R5(c): the repo-wide
// destructive-guidance convention scan. Follows the ratchet_universe_test.go
// cached-universe / boundary_test.go repo-root-walk precedent (this is
// NOT a Req-21 extension — Req 21, internal/guard/recovery_convention_test.go,
// stays package-local by its own documented import-cycle constraint;
// this scan lives here specifically because it needs to import
// internal/guard's classifier, which internal/guard's own tests cannot
// do of themselves for every OTHER package).
//
// Two legs:
//   - the PRODUCT-DIAGNOSTIC leg: guard.NewFailure/FormatFailure call
//     arguments, RecoveryCommand-shaped method returns, and
//     `Message:`-keyed composite-literal fields, repo-wide. Literal
//     matches are flagged in EVERY scanned position, including
//     message bodies; the fail-closed-on-unprovable OBLIGATION,
//     though, is scoped to COMMAND positions only (C-r4-6,
//     diagFinding.commandPosition) — a guard.NewFailure/FormatFailure
//     call's first argument and a `Message:` field are message-shaped
//     free prose, and an unprovable one needs no registry entry
//     (spec 127 bead-2 rework, RULING 10/O3-r2-6: the prior header
//     claimed the broader, false "fail-closed on any operand"
//     reading). Reach is further bounded to the fold's own PROVABLE
//     shapes (a literal, an fmt.Sprintf literal TEMPLATE — never its
//     substituted args — a package-level const, a function-local
//     single-assignment bind): a helper function's OWN Sprintf
//     argument or a strings.Builder-composed value is invisible to
//     this leg entirely, not merely unprovable. internal/panel/gate.go's
//     RawMergeFence (8 real call sites) is the concrete, disclosed
//     instance of this reach limit — see its own doc comment.
//   - the GUIDANCE leg: the shipped-guidance globs (walked live by
//     guidanceGlobDirs, four roots today) plus the
//     evaluated canonical setup-guidance builders, checked against the
//     known-sites exemption list. This leg classifies LINE BY LINE
//     (scanGuidanceSurfaces): a destructive command spanning a line
//     break — a backslash continuation, a markdown reflow, or a
//     multi-row table — is invisible to it (spec 127 bead-2 rework,
//     O2-r2-11), the same way the product-diagnostic leg's reach is
//     bounded above. A reflow that splits a listed exemption entry's
//     line both REDs it as stale-absent (correctly — the exact quoted
//     line is gone) AND removes the sweep's ability to see the
//     command at all, so a later "fix" of the stale entry could green
//     the sweep over text that still instructs the same command. Not
//     fixed in this bead; recorded as a known, disclosed limitation.
//
// Both legs share one fact: "is this text dangerous?" is not decidable
// from the text (spec 127 Non-Goals) — this scan claims exactly what
// guard.FindFloorMatches's reviewed finite floor can prove, nothing
// more. A destructive family or program outside that floor is caught
// by review under the ADR-0035 amendment's in-diff extension
// obligation, not by this scan.
package lint

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/setup"
)

// ---------------------------------------------------------------------
// Folding: the provable-shapes rules (spec 127 R5c).
// ---------------------------------------------------------------------

// pkgConstFolder computes, per package directory, every package-level
// CONST's folded string value — extending ratchet_universe_test.go's
// existing one-level-BasicLit-only const capture to full recursive
// concatenation folding (needed for claudeMDManagedBlock/
// agentsMDBlockTemplate, both declared as chains of backtick-literal +
// quoted-escape segments joined by `+`). Built once per universe and
// cached on it via a side map, independent of rUniverse.consts (which
// this file does not mutate).
type pkgConstFolder struct {
	u     *rUniverse
	byPkg map[string]map[string]string
	// byPkgFull mirrors byPkg's keys, recording whether that entry's
	// OWN fold was fully literal (spec 127 bead-2 rework round 2,
	// O2c-1's third laundering path): before this fix, Get discarded
	// foldExpr's third return value entirely, so a package-level
	// const/var whose value was only PARTIALLY foldable (e.g. `"note:
	// " + buildRecovery(b)`) was still reported as fully literal to
	// every caller that referenced it by name — laundering exactly
	// like the bindLaunder/sprintfLaunder shapes below.
	byPkgFull map[string]map[string]bool
	memo      map[string]map[string]bool // in-progress guard against cyclic const refs
	sites     map[string]exprSite        // pkgDir.name -> AST site, PER-FOLDER (never shared across universes)
}

func newPkgConstFolder(u *rUniverse) *pkgConstFolder {
	f := &pkgConstFolder{u: u, byPkg: map[string]map[string]string{}, byPkgFull: map[string]map[string]bool{}, memo: map[string]map[string]bool{}, sites: map[string]exprSite{}}
	for _, file := range u.files {
		for _, decl := range file.file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || (gen.Tok != token.CONST && gen.Tok != token.VAR) {
				continue
			}
			for _, spec := range gen.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, nm := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					f.registerPkgLevel(file.pkgDir, nm.Name, vs.Values[i], file)
				}
			}
		}
	}
	return f
}

func (f *pkgConstFolder) registerPkgLevel(pkgDir, name string, val ast.Expr, file *rFile) {
	if f.byPkg[pkgDir] == nil {
		f.byPkg[pkgDir] = map[string]string{}
	}
	if f.memo[pkgDir] == nil {
		f.memo[pkgDir] = map[string]bool{}
	}
	// Lazily folded on first lookup (Get below does the recursive work
	// and memoizes).
	f.sites[pkgDir+"."+name] = exprSite{expr: val, file: file}
}

type exprSite struct {
	expr ast.Expr
	file *rFile
}

// Get returns the folded string value of package-level const/var name
// in pkgDir, resolving one level of cross-reference to another
// package-level const/var in the SAME package (the depth this bead
// needs — claudeMDManagedBlock/agentsMDBlockTemplate are pure literal
// concatenation chains with no such cross-reference), plus whether
// that fold was FULLY literal (bead-2 rework round 2, O2c-1 —
// PREVIOUSLY discarded via `v, ok, _ := foldExpr(...)`, laundering a
// partially-foldable package-level const/var to fullyLiteral=true for
// every caller that referenced it by name).
func (f *pkgConstFolder) Get(pkgDir, name string) (string, bool, bool) {
	key := pkgDir + "." + name
	if v, ok := f.byPkg[pkgDir][name]; ok {
		return v, true, f.byPkgFull[pkgDir][name]
	}
	site, ok := f.sites[key]
	if !ok {
		return "", false, false
	}
	if f.memo[pkgDir][name] {
		return "", false, false // cycle guard
	}
	f.memo[pkgDir][name] = true
	v, ok, full := foldExpr(site.expr, site.file, f, nil, 0)
	if ok {
		f.byPkg[pkgDir][name] = v
		if f.byPkgFull[pkgDir] == nil {
			f.byPkgFull[pkgDir] = map[string]bool{}
		}
		f.byPkgFull[pkgDir][name] = full
	}
	return v, ok, full
}

// bindValue is one localBinds entry: its folded skeleton plus whether
// that fold was fully literal (bead-2 rework round 2, O2c-1's second
// laundering path — PREVIOUSLY, localBinds discarded foldExpr's third
// return value via `v, ok, _ :=`, so a function-local bind whose
// value was only PARTIALLY foldable was recorded as fully literal to
// every later operand that referenced it by name).
type bindValue struct {
	skel string
	full bool
}

// localBinds computes function-scoped single-assignment string binds:
// every `:=`/`var` name assigned EXACTLY ONCE within fn's body (top
// level and nested blocks — ast.Inspect walks the whole subtree but
// does not cross into nested FuncLit bodies, which get their own
// scope) to a foldable expression. A name touched more than once, or
// once with a non-foldable value, is excluded — "single-assignment"
// per spec 127 R5(c).
func localBinds(fn ast.Node, file *rFile, f *pkgConstFolder) map[string]bindValue {
	binds := map[string]bindValue{}
	excluded := map[string]bool{}
	assign := func(name string, val ast.Expr) {
		if excluded[name] {
			return
		}
		if _, exists := binds[name]; exists {
			excluded[name] = true
			delete(binds, name)
			return
		}
		if v, ok, full := foldExpr(val, file, f, nil, 0); ok {
			binds[name] = bindValue{skel: v, full: full}
		} else {
			excluded[name] = true
		}
	}
	ast.Inspect(fn, func(n ast.Node) bool {
		switch d := n.(type) {
		case *ast.FuncLit:
			return n == fn // don't cross into nested closures' own scope
		case *ast.AssignStmt:
			if len(d.Lhs) == len(d.Rhs) {
				for i, lhs := range d.Lhs {
					if id, ok := lhs.(*ast.Ident); ok && id.Name != "_" {
						assign(id.Name, d.Rhs[i])
					}
				}
			}
		case *ast.GenDecl:
			if d.Tok == token.CONST || d.Tok == token.VAR {
				for _, spec := range d.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, nm := range vs.Names {
						if i < len(vs.Values) {
							assign(nm.Name, vs.Values[i])
						}
					}
				}
			}
		}
		return true
	})
	return binds
}

// unfoldableMarker replaces a fragment foldExpr cannot resolve inside
// a `+`-concatenation (spec 127 bead-2 rework, RULING 5/O3-r2-1): a
// bare space (the prior substitution) DISAPPEARS as a token in its
// own right once tokenRe splits on whitespace, so
// `"git -C " + dir + " reset --hard"` folded to `git -C   reset
// --hard` and skipGlobalOptions then consumed the literal word
// "reset" as -C's OPERAND — defeating global-option normalization for
// every family behind a dynamic-path `git -C`/`git -c`, this
// codebase's own dominant command-building shape. The marker is
// wrapped in explicit spaces so it still separates adjacent literal
// fragments exactly as a bare space did (no fusion across the
// dynamic gap), but its own non-whitespace content (�) survives
// tokenization as one real, non-empty, non-flag-shaped token — so a
// global option's operand slot stays OCCUPIED instead of vanishing.
const unfoldableMarker = " � "

// foldExpr computes the literal "skeleton" of expr: the concatenation
// of every literal-provable fragment, with non-literal fragments
// replaced by unfoldableMarker (above) so adjacent literal tokens on
// either side of a dynamic gap never fuse into a token neither side
// alone would form, AND the gap itself still occupies a token slot.
// Three return values: skel is the folded skeleton; hasLiteral is
// false ONLY when expr contributes NO literal content at all — a bare
// call, a bare non-local identifier, or an unresolvable selector;
// fullyLiteral is false whenever ANY leaf of expr's tree did not fold
// (spec 127 bead-2 rework, RULING 5/S1-r2-1) — a partially-foldable
// expression (a literal flag prefix concatenated with a dynamic ID/
// branch-name suffix, matching this codebase's own convention of
// "flags are literal, values are dynamic") sets hasLiteral=true but
// fullyLiteral=false: callers that gate the opaque-operand obligation
// on "no literal content" alone (the old, wrong test) would wrongly
// exempt a finding whose destructive content lives ENTIRELY in the
// unfoldable remainder — this file's callers now gate on fullyLiteral
// instead. hasLiteral and fullyLiteral agree everywhere EXCEPT inside
// a BinaryExpr, the sole source of a genuine partial fold.
//
// depth guards against runaway recursion through mutually-referential
// consts/calls; it is not expected to matter for any real site at this
// base. file provides the enclosing file's import table, needed to
// resolve a cross-package selector const (e.g.
// `containment.RejectionLever`) to the OTHER package's pkgDir.
//
// Bead-2 rework round 2 (O2c-1, BLOCKING) found THREE of this
// function's four fold paths laundered a partial fold to
// fullyLiteral=true despite this doc comment's own universal claim:
// the *ast.Ident case (a local bind — fixed by reading bindValue.full,
// below), the *ast.SelectorExpr/const-lookup case (pkgConstFolder.Get
// — fixed by propagating Get's own third return value), and the
// fmt.Sprintf case inside *ast.CallExpr (fixed below). Only the direct
// BinaryExpr leaf was ever correct. All four now agree with this
// comment; TestFoldExpr_FullyLiteralAcrossAllFourPaths pins it.
func foldExpr(expr ast.Expr, file *rFile, f *pkgConstFolder, binds map[string]bindValue, depth int) (skel string, hasLiteral bool, fullyLiteral bool) {
	pkgDir := file.pkgDir
	if depth > 6 {
		return "", false, false
	}
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			if v, ok := unquote(e.Value); ok {
				return v, true, true
			}
		}
		return "", false, false
	case *ast.ParenExpr:
		return foldExpr(e.X, file, f, binds, depth+1)
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", false, false
		}
		ls, lhas, lfull := foldExpr(e.X, file, f, binds, depth+1)
		rs, rhas, rfull := foldExpr(e.Y, file, f, binds, depth+1)
		if !lhas && !rhas {
			return "", false, false
		}
		if !lhas {
			ls = unfoldableMarker
		}
		if !rhas {
			rs = unfoldableMarker
		}
		return ls + rs, true, lfull && rfull
	case *ast.Ident:
		if binds != nil {
			if bv, ok := binds[e.Name]; ok {
				return bv.skel, true, bv.full
			}
		}
		if v, ok, full := f.Get(pkgDir, e.Name); ok {
			return v, true, full
		}
		return "", false, false
	case *ast.SelectorExpr:
		// A cross-package selector const, e.g. containment.RejectionLever
		// — resolve the package alias to its import path, then to that
		// package's own pkgDir (an internal/cmd package only; an
		// external dependency has no pkgDir in this universe and
		// correctly fails to fold).
		xid, ok := e.X.(*ast.Ident)
		if !ok {
			return "", false, false
		}
		importPath, ok := file.imports[xid.Name]
		if !ok {
			return "", false, false
		}
		const modPrefix = "github.com/mrmaxsteel/mindspec/"
		if !strings.HasPrefix(importPath, modPrefix) {
			return "", false, false
		}
		otherPkgDir := strings.TrimPrefix(importPath, modPrefix)
		if v, ok, full := f.Get(otherPkgDir, e.Sel.Name); ok {
			return v, true, full
		}
		return "", false, false
	case *ast.CallExpr:
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			// fmt.Sprintf: provable via its literal TEMPLATE only
			// (spec 127 R5c's own listed shape) — substituted args
			// are deliberately never inspected FOR SKELETON purposes
			// (the skeleton stays template-only, by design). But
			// fullyLiteral must be false whenever there IS a
			// substituted arg (len(e.Args) > 1): a literal template
			// wrapping an opaque, genuinely destructive substituted
			// argument (spec 127 bead-2 rework round 2, O2c-1/
			// S1-CONFIRM-1's fixture: `fmt.Sprintf("note: %s",
			// buildRecovery(b))`) is NOT a fully-proven operand — that
			// is the whole point of gating on fullyLiteral rather than
			// hasLiteral (RULING 5). At minimum (no per-arg folding
			// attempted here, matching the design principle above):
			// any additional argument forces fullyLiteral=false,
			// conservative in the safe direction.
			if xid, ok2 := sel.X.(*ast.Ident); ok2 && xid.Name == "fmt" && sel.Sel.Name == "Sprintf" && len(e.Args) > 0 {
				tskel, thas, tfull := foldExpr(e.Args[0], file, f, binds, depth+1)
				return tskel, thas, tfull && len(e.Args) == 1
			}
			// Spec 127 bead-2 rework, RULING 4/O2-r2-2: the former
			// RecoveryCommand/containment.EmitCd "trusted helper"
			// rules are DELETED. Both returned ("", true) — "proven
			// to contain no destructive literal" — which is strictly
			// STRONGER, and strictly WORSE when wrong, than the
			// ("", false, false) an ordinary unresolvable call
			// returns: a false operand in a command position carries
			// an opaque-operand-registry obligation; a trusted one
			// carried none. Three escapes needed neither: (a) a
			// RecoveryCommand method with a named result and a bare
			// `return` (leg (ii) below only folds
			// ret.Results[0], skipping len(ret.Results)==0); (b) a
			// func-TYPED FIELD named RecoveryCommand (leg (ii) only
			// scans *ast.FuncDecl, never a closure value); (c) the
			// EmitCd boundary matched the bare IDENTIFIER
			// "containment", not a resolved import path, unlike
			// isConstructorDerived's own strict resolution a few
			// lines below. Falling through to the unresolved-call
			// default now routes every real site through the SAME
			// two governance tests every other opaque operand answers
			// to — measured at exactly 4 real sites, now registered
			// in guard.OpaqueOperandRegistry (registries.go).
		}
		return "", false, false
	}
	return "", false, false
}

// ---------------------------------------------------------------------
// Constructor provenance (R5b's scan-side half): does expr trace to
// an approved guard.NewDestructiveCommand(...) call?
// ---------------------------------------------------------------------

// isConstructorDerived reports whether expr is a `.String()` call
// whose receiver traces to an approved guard.NewDestructiveCommand(...)
// call — dataflow to the approved constructor's call site, never
// merely the static type (spec 127 D-r4-2): a wrapper that returns a
// FORGED value under the same name, or a same-named function
// elsewhere, does not match this AST shape at all (it is a different,
// unresolved call), so it does not qualify.
//
// Two receiver shapes qualify:
//   - directly a `guard.NewDestructiveCommand(...)` call (only
//     reachable today via `.String()` chained straight off a call
//     that itself ignores the error return with `_` positionally,
//     e.g. `guard.NewDestructiveCommand(cmd, outcome)` used as a
//     single-value expression is NOT valid Go for a two-result func,
//     so this shape is now vestigial for a real two-result signature
//     — kept only so a future single-result variant, should one ever
//     exist, is not silently unrecognized);
//   - a local identifier BOUND via a single, error-checked, two-
//     result assignment to a `guard.NewDestructiveCommand(...)` call
//     — the shape every REAL caller must use today, since
//     NewDestructiveCommand's signature is (DestructiveCommand,
//     error) (spec 127 bead-2 rework RULING 1). Before this fix, the
//     scan's sole accepted shape was the first, direct-chain form,
//     which the two-result signature makes syntactically impossible
//     to write — there was no live production caller to expose the
//     gap, but a later bead's first correctly error-handled
//     conversion would have been rejected outright (bead-2 rework
//     round 2, G1-C1, BLOCKING).
//
// fnNode is the innermost enclosing function/declaration (from
// enclosingFuncNode) — nil when no dataflow tracing is possible (the
// direct-chain shape needs none).
func isConstructorDerived(expr ast.Expr, fnNode ast.Node, file *rFile) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "String" {
		return false
	}
	if inner, ok := sel.X.(*ast.CallExpr); ok && isNewDestructiveCommandCall(inner, file) {
		return true
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok || id.Name == "_" || fnNode == nil {
		return false
	}
	return identIsErrorHandledConstructorBind(id.Name, fnNode, file)
}

// isNewDestructiveCommandCall is the callee-name-spoof defense (spec
// 127 R5b): a bare `NewDestructiveCommand(...)` only qualifies from
// WITHIN internal/guard itself; a qualified
// `pkg.NewDestructiveCommand(...)` only qualifies when `pkg`'s IMPORT
// PATH actually resolves to internal/guard — a same-named function in
// some OTHER package does not match by name alone.
func isNewDestructiveCommandCall(call *ast.CallExpr, file *rFile) bool {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name == "NewDestructiveCommand" && file.pkgDir == "internal/guard"
	case *ast.SelectorExpr:
		if fn.Sel.Name != "NewDestructiveCommand" {
			return false
		}
		xid, ok := fn.X.(*ast.Ident)
		if !ok {
			return false
		}
		return file.imports[xid.Name] == "github.com/mrmaxsteel/mindspec/internal/guard"
	}
	return false
}

// identIsErrorHandledConstructorBind walks fnNode's body for EXACTLY
// ONE two-result assignment (`:=` or `=`) of the shape `name, errName
// := guard.NewDestructiveCommand(...)`, with errName a real
// (non-blank) identifier, followed somewhere later in the SAME body
// by an `if errName != nil { ... }` whose block exits control flow
// (return/panic/continue/break/goto) — the ordinary Go handle-and-bail
// idiom. Four ways this returns false, each pinned by its own negative
// fixture (bead-2 rework round 2, G1-C1's required change; the fourth
// added round 3, RULING 2/G1-confirm2-2):
//   - a SECOND two-result NewDestructiveCommand bind of name anywhere
//     in the body voids provenance entirely — ambiguous which binding
//     a later use traces to, the same single-assignment discipline
//     localBinds (above) already applies to a plain fold
//     ("overwritten binding");
//   - a blank (`_`) error identifier never gets a later check to find
//     ("ignored" — the caller explicitly discarded the only signal
//     that would prove the happy path was actually reached);
//   - a real error identifier with no later handle-and-bail block
//     ("error-unhandled" — same reasoning, checked structurally rather
//     than by name);
//   - a PLAIN reassignment of name (`name = <anything>`, not a second
//     two-result NewDestructiveCommand bind) anywhere in the body also
//     voids provenance (identReassignedAfter, below) — round 2's check
//     only invalidated on a second matching TWO-RESULT bind, so a
//     plain `d = guard.DestructiveCommand{}` after a correctly
//     error-checked bind kept tracing as constructor-derived even
//     though d no longer held the constructor's output.
//
// This is a TEXTUAL/structural check, not a control-flow reachability
// proof — see errCheckedAfter's own doc comment (spec 127 bead-2
// rework round 3, RULING 2) for what it does not establish.
func identIsErrorHandledConstructorBind(name string, fnNode ast.Node, file *rFile) bool {
	var bindErrName string
	var bindEnd token.Pos
	var bindStmt *ast.AssignStmt
	count := 0
	ast.Inspect(fnNode, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Lhs) != 2 || len(as.Rhs) != 1 {
			return true
		}
		lhs0, ok := as.Lhs[0].(*ast.Ident)
		if !ok || lhs0.Name != name {
			return true
		}
		call, ok := as.Rhs[0].(*ast.CallExpr)
		if !ok || !isNewDestructiveCommandCall(call, file) {
			return true
		}
		errIdent, ok := as.Lhs[1].(*ast.Ident)
		if !ok {
			return true
		}
		count++
		bindErrName = errIdent.Name
		bindEnd = as.End()
		bindStmt = as
		return true
	})
	if count != 1 || bindErrName == "" || bindErrName == "_" {
		return false
	}
	if identReassignedAfter(name, fnNode, bindStmt) {
		return false
	}
	return errCheckedAfter(fnNode, bindErrName, bindEnd)
}

// identReassignedAfter reports whether name is the target of any
// *ast.AssignStmt in fnNode's body OTHER than exclude (the qualifying
// constructor bind itself) — spec 127 bead-2 rework round 3, RULING 2/
// G1-confirm2-2 (BLOCKING): a plain `name = <anything>` reassignment
// after a correctly error-checked bind still passed round 2's SHAPE
// check, because round 2 only invalidated on a second matching TWO-
// RESULT NewDestructiveCommand bind, never on an ordinary single-value
// reassignment — the value `.String()` is eventually called on may no
// longer be the constructor's output at all. This is a cheap, TEXTUAL
// invalidation: ANY reassignment voids provenance, regardless of
// whether it would runtime-execute before or after the eventual
// `.String()` call (this scan does not attempt control-flow ordering)
// — not a claim that the bind is the value's ONLY possible source at
// the .String() call site, which would need real dataflow (go/types +
// SSA), out of this bead's scope.
func identReassignedAfter(name string, fnNode ast.Node, exclude *ast.AssignStmt) bool {
	found := false
	ast.Inspect(fnNode, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || as == exclude {
			return true
		}
		for _, lhs := range as.Lhs {
			if id, ok := lhs.(*ast.Ident); ok && id.Name == name {
				found = true
			}
		}
		return true
	})
	return found
}

// errCheckedAfter reports whether fnNode's body contains an
// `if errName != nil { ... }` (either operand order) positioned after
// pos, whose block exits control flow (blockExits, below).
//
// This is a TEXTUAL/structural check — a later-positioned exiting
// `if` — not a control-flow REACHABILITY proof (spec 127 bead-2 rework
// round 3, RULING 2/G1-confirm2-3, BLOCKING): it does not establish
// that the check actually runs before any use of the bound identifier
// at runtime. Never descending into an *ast.DeferStmt or *ast.GoStmt
// closes the one concrete bypass adversarial review demonstrated: a
// check placed inside `defer func() { if err != nil { panic(err) } }()`
// is textually after the bind and syntactically an exiting block, so
// it matched this shape, even though a deferred closure runs AFTER the
// surrounding return expression — including any `.String()` call on
// the same line — has already evaluated (a `go func(){...}()` launched
// goroutine has the identical problem: it may not even have run yet).
// Excluding both closes exactly the demonstrated escape; it is not a
// claim that every possible way of writing a check that never
// synchronously gates its guarded call is now caught — that would need
// actual control-flow analysis, out of this scan's scope.
func errCheckedAfter(fnNode ast.Node, errName string, pos token.Pos) bool {
	found := false
	ast.Inspect(fnNode, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.DeferStmt, *ast.GoStmt:
			return false
		}
		ifs, ok := n.(*ast.IfStmt)
		if !ok || ifs.Pos() < pos {
			return true
		}
		bin, ok := ifs.Cond.(*ast.BinaryExpr)
		if !ok || bin.Op != token.NEQ {
			return true
		}
		idExpr, otherExpr := bin.X, bin.Y
		if _, ok := idExpr.(*ast.Ident); !ok {
			idExpr, otherExpr = bin.Y, bin.X
		}
		id, ok := idExpr.(*ast.Ident)
		if !ok || id.Name != errName {
			return true
		}
		nilID, ok := otherExpr.(*ast.Ident)
		if !ok || nilID.Name != "nil" {
			return true
		}
		if blockExits(ifs.Body) {
			found = true
		}
		return true
	})
	return found
}

// blockExits reports whether block's statement list ends in a
// control-flow-exiting statement — the ordinary "handle and bail"
// shape. A block that merely references the error without exiting
// (logs it and falls through, say) does not count: a later
// `.String()` call would then be reachable even on the error path,
// resting provenance on an unchecked assumption exactly like the
// ignored-error case this function's caller must reject.
func blockExits(block *ast.BlockStmt) bool {
	if block == nil || len(block.List) == 0 {
		return false
	}
	switch last := block.List[len(block.List)-1].(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return last.Tok == token.BREAK || last.Tok == token.CONTINUE || last.Tok == token.GOTO
	case *ast.ExprStmt:
		if call, ok := last.X.(*ast.CallExpr); ok {
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "panic" {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------
// Product-diagnostic leg: collecting scan sites.
// ---------------------------------------------------------------------

// diagFinding is one scanned operand's classification result.
type diagFinding struct {
	site       rSite
	matches    []guard.Match // non-empty => a floor-family hit was found
	hasLiteral bool          // only meaningful when matches is empty
	// fullyLiteral is false whenever ANY leaf of the scanned
	// expression's tree did not fold — a partially-foldable
	// expression (hasLiteral=true) whose destructive content lives
	// entirely in the unfoldable remainder is NOT fullyLiteral (spec
	// 127 bead-2 rework, RULING 5/S1-r2-1). The unprovable-command-
	// operand governance test gates on THIS field, never on
	// hasLiteral alone.
	fullyLiteral bool
	provenance   bool // constructor-derived — exempt regardless
	// commandPosition is false for a guard.NewFailure/FormatFailure
	// call's FIRST argument (the free-form message) and true for every
	// other scanned position (recovery-command args, RecoveryCommand
	// returns, Message-composite-lit fields). An unprovable message
	// position needs no registry entry — free prose is never itself a
	// pasteable command, and requiring one would revive C-r4-6's
	// unsatisfiable unbounded clause (message text is dynamic at
	// nearly every one of the ~112 call sites). A LITERAL match on a
	// message position is still flagged (message bodies can render
	// commands too, O3-r2-7) — only the unprovable-needs-registry
	// obligation is scoped to command positions.
	commandPosition bool
}

// isFailureConstructorCall reports whether call invokes
// guard.NewFailure/guard.FormatFailure — bare inside package guard
// itself, qualified (`guard.NewFailure`) elsewhere.
func isFailureConstructorCall(call *ast.CallExpr, pkgDir string) (string, bool) {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		if pkgDir == "internal/guard" && (fn.Name == "NewFailure" || fn.Name == "FormatFailure") {
			return fn.Name, true
		}
	case *ast.SelectorExpr:
		if x, ok := fn.X.(*ast.Ident); ok && x.Name == "guard" {
			if fn.Sel.Name == "NewFailure" || fn.Sel.Name == "FormatFailure" {
				return fn.Sel.Name, true
			}
		}
	}
	return "", false
}

// scanProductDiagnostics walks the whole universe (production, non-
// test files under cmd/+internal) collecting a diagFinding for every
// operand in scope: guard.NewFailure/FormatFailure arguments,
// RecoveryCommand-shaped method returns, and Message-keyed composite-
// literal fields (spec 127 R5c legs i/ii/iii).
func scanProductDiagnostics(t *testing.T, u *rUniverse, f *pkgConstFolder) []diagFinding {
	t.Helper()
	var out []diagFinding
	for _, file := range u.files {
		binds := map[string]map[string]bindValue{} // enclosing func key -> binds, computed lazily
		bindsFor := func(pos token.Pos) map[string]bindValue {
			key := enclosingFunc(file, pos)
			if b, ok := binds[key]; ok {
				return b
			}
			node := enclosingFuncNode(file, pos)
			b := localBinds(node, file, f)
			binds[key] = b
			return b
		}
		ast.Inspect(file.file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CallExpr:
				// internal/guard/recovery.go's own NewFailure forwarding
				// its commands... to FormatFailure is the formatter's
				// OWN definition, not a diagnostic emission site — every
				// REAL command string originates at some OTHER call
				// site, independently scanned there. Req 21
				// (recovery_convention_test.go) already covers this
				// package's own constructors.
				if file.pkgDir == "internal/guard" && enclosingFunc(file, node.Pos()) == "NewFailure" {
					return true
				}
				if _, ok := isFailureConstructorCall(node, file.pkgDir); ok {
					b := bindsFor(node.Pos())
					fnNode := enclosingFuncNode(file, node.Pos())
					for i, arg := range node.Args {
						if node.Ellipsis != token.NoPos && i == len(node.Args)-1 {
							// A variadic spread of a runtime-built slice
							// (e.g. report.RecoveryCommands()...) has NO
							// per-element literal to fold — genuinely
							// opaque unless provenance/registry-covered.
							out = append(out, diagFinding{
								site:            rSite{Rel: file.rel, Func: enclosingFunc(file, node.Pos()), Detail: "variadic-spread:" + calleeText(arg), Line: fset(u, node.Pos())},
								hasLiteral:      false,
								fullyLiteral:    false,
								provenance:      isConstructorDerived(arg, fnNode, file),
								commandPosition: true,
							})
							continue
						}
						skel, hasLit, fullyLit := foldExpr(arg, file, f, b, 0)
						matches := guard.FindFloorMatches(skel)
						out = append(out, diagFinding{
							site:            rSite{Rel: file.rel, Func: enclosingFunc(file, node.Pos()), Detail: detailFor(matches, skel, i), Line: fset(u, node.Pos())},
							matches:         matches,
							hasLiteral:      hasLit,
							fullyLiteral:    fullyLit,
							provenance:      isConstructorDerived(arg, fnNode, file),
							commandPosition: i >= 1,
						})
					}
				}
			case *ast.FuncDecl:
				// KNOWN, DISCLOSED gap (spec 127 bead-2 rework,
				// S1-r2-2): this only scans a bare `return <expr>`
				// with at least one result — a RecoveryCommand method
				// using a NAMED result plus a bare `return` (no result
				// list on the return statement itself) is invisible
				// here. Not exploitable TODAY (all three real
				// implementers — Orphan, StaleOpenBead, and the third
				// this spec's Background inventoried — use a single
				// bare `return <expr>` with no named result), but this
				// is a scoping decision recorded honestly rather than
				// an unconditional guarantee: since the RecoveryCommand
				// trust boundary itself is DELETED (RULING 4), a
				// RecoveryCommand call this leg misses is still caught
				// as an ordinary unresolved call requiring registry
				// coverage — it does not silently pass either way.
				if node.Name.Name == "RecoveryCommand" && node.Body != nil {
					b := localBinds(node.Body, file, f)
					ast.Inspect(node.Body, func(rn ast.Node) bool {
						if _, isLit := rn.(*ast.FuncLit); isLit {
							return rn == node.Body
						}
						ret, ok := rn.(*ast.ReturnStmt)
						if !ok || len(ret.Results) == 0 {
							return true
						}
						skel, hasLit, fullyLit := foldExpr(ret.Results[0], file, f, b, 0)
						matches := guard.FindFloorMatches(skel)
						out = append(out, diagFinding{
							site:            rSite{Rel: file.rel, Func: enclosingFunc(file, node.Pos()), Detail: detailFor(matches, skel, 0), Line: fset(u, ret.Pos())},
							matches:         matches,
							hasLiteral:      hasLit,
							fullyLiteral:    fullyLit,
							provenance:      isConstructorDerived(ret.Results[0], node, file),
							commandPosition: true,
						})
						return true
					})
				}
			case *ast.CompositeLit:
				for _, elt := range node.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					key, ok := kv.Key.(*ast.Ident)
					if !ok || key.Name != "Message" {
						continue
					}
					b := bindsFor(node.Pos())
					skel, hasLit, fullyLit := foldExpr(kv.Value, file, f, b, 0)
					matches := guard.FindFloorMatches(skel)
					out = append(out, diagFinding{
						site:         rSite{Rel: file.rel, Func: enclosingFunc(file, node.Pos()), Detail: detailFor(matches, skel, 0), Line: fset(u, kv.Pos())},
						matches:      matches,
						hasLiteral:   hasLit,
						fullyLiteral: fullyLit,
						provenance:   isConstructorDerived(kv.Value, enclosingFuncNode(file, node.Pos()), file),
						// A Message field is message-shaped, like a
						// NewFailure/FormatFailure call's first
						// argument — free-form finding prose, not a
						// pasteable command. An unprovable Message
						// field therefore needs no registry entry (the
						// same reasoning as arg0); a LITERAL match is
						// still flagged either way (O3-r2-7).
						commandPosition: false,
					})
				}
			}
			return true
		})
	}
	return out
}

func detailFor(matches []guard.Match, skel string, argIdx int) string {
	if len(matches) > 0 {
		var texts []string
		for _, m := range matches {
			texts = append(texts, string(m.Family)+":"+m.Text)
		}
		return strings.Join(texts, " | ")
	}
	return "arg" + itoa(argIdx) + ":unprovable"
}

func fset(u *rUniverse, pos token.Pos) int {
	return u.fset.Position(pos).Line
}

// ---------------------------------------------------------------------
// Guidance leg: the shipped-guidance globs plus the canonical
// setup-guidance builders (spec 127 R5c/H-r6-6).
// ---------------------------------------------------------------------

// guidanceGlobDirs is the closed set of tracked-file guidance roots
// (spec 127 R5c: `.claude/agents/**`, `.claude/skills/**`,
// `plugins/*/skills/**`, and `project-docs/**`). Walked live at test time — a newly added
// file under any of these is automatically included (the anti-drift
// leg is the live filesystem walk itself, not a hardcoded list).
//
// `project-docs/**` was ADDED at spec 127's final confirm round (F3):
// this spec's own fix rounds shipped `project-docs/user/guides/
// merge-safety.md`, an OPERATOR-FACING guidance file telling a human
// which git commands to run around a preserved merge — the exact class
// this sweep exists for — into a tree no glob reached. A sweep that
// misses the guidance its own spec shipped is the enumeration defect
// this spec is about, one directory over. The whole `project-docs`
// tree is taken rather than the one `user/guides` subdirectory, for
// the same reason these roots are walked live rather than listed: the
// next guidance file will not be added where the last one was.
func guidanceGlobDirs(root string) []string {
	var dirs []string
	for _, d := range []string{
		filepath.Join(root, ".claude", "agents"),
		filepath.Join(root, ".claude", "skills"),
		filepath.Join(root, "project-docs"),
	} {
		if _, err := os.Stat(d); err == nil {
			dirs = append(dirs, d)
		}
	}
	pluginsRoot := filepath.Join(root, "plugins")
	entries, err := os.ReadDir(pluginsRoot)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			skillsDir := filepath.Join(pluginsRoot, e.Name(), "skills")
			if _, err := os.Stat(skillsDir); err == nil {
				dirs = append(dirs, skillsDir)
			}
		}
	}
	return dirs
}

// walkGuidanceFiles reads every regular file under the guidance globs,
// keyed by its repo-relative slash path.
func walkGuidanceFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, dir := range guidanceGlobDirs(root) {
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return rerr
			}
			out[filepath.ToSlash(rel)] = string(data)
			return nil
		})
		if err != nil {
			t.Fatalf("walking guidance dir %s: %v", dir, err)
		}
	}
	return out
}

// guidanceHitKey is the content-grain identity of one matched line on
// one surface (spec 127 H-r6-5: surface + quoted matched string; line
// numbers are advisory only, never part of this key).
type guidanceHitKey struct {
	Surface string
	Text    string
}

// scanGuidanceSurfaces line-scans every surface's content and returns
// the occurrence count of each distinct (surface, matched-line) pair —
// "matched line" meaning any line on which guard.FindFloorMatches
// found at least one hit. Multiple floor families matching within one
// line are folded into that single line-level hit (the exemption
// list's own grain, per R5a: "an entry pins surface + quoted string +
// count", not one entry per family).
func scanGuidanceSurfaces(surfaces map[string]string) map[guidanceHitKey]int {
	hits := map[guidanceHitKey]int{}
	for surface, content := range surfaces {
		for _, line := range strings.Split(content, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if len(guard.FindFloorMatches(trimmed)) > 0 {
				hits[guidanceHitKey{Surface: surface, Text: trimmed}]++
			}
		}
	}
	return hits
}

// exemptionMap converts the guard package's exemption list into the
// same (surface,text)->count shape scanGuidanceSurfaces produces.
func exemptionMap(list []guard.KnownSiteExemptionEntry) map[guidanceHitKey]int {
	m := map[guidanceHitKey]int{}
	for _, e := range list {
		m[guidanceHitKey{Surface: e.Surface, Text: e.Text}] = e.Count
	}
	return m
}

// diffGuidanceHits performs the exemption list's three machine
// assertions (spec 127 R5a): (i) presence at the pinned count — a
// found hit not equal-count to its exemption entry is a STALE entry
// (deleted/reworded/duplicated warning); (ii) an unlisted floor match
// is RED; a listed entry whose line is entirely absent from the live
// scan is also STALE. Assertion (iii) additions-red falls out of (ii)
// automatically since a brand-new exemption entry with no live hit is
// unfalsifiable by construction of this same comparison.
func diffGuidanceHits(found, exempt map[guidanceHitKey]int) (unlisted, staleCount, staleAbsent []string) {
	for k, n := range found {
		want, ok := exempt[k]
		if !ok {
			unlisted = append(unlisted, k.Surface+": "+k.Text)
			continue
		}
		if want != n {
			staleCount = append(staleCount, k.Surface+": "+k.Text+" (want count "+itoa(want)+", found "+itoa(n)+")")
		}
	}
	for k := range exempt {
		if _, ok := found[k]; !ok {
			staleAbsent = append(staleAbsent, k.Surface+": "+k.Text)
		}
	}
	sort.Strings(unlisted)
	sort.Strings(staleCount)
	sort.Strings(staleAbsent)
	return
}

// canonicalGuidanceSurfaces names the setup.CanonicalGuidanceSurfaces
// test seam locally so the mutation-fixture tests below read as
// "take the shipped surfaces, mutate a copy."
func canonicalGuidanceSurfaces() map[string]string {
	return setup.CanonicalGuidanceSurfaces()
}

// ---------------------------------------------------------------------
// Top-level tests.
// ---------------------------------------------------------------------

func allGuidanceSurfaces(t *testing.T) map[string]string {
	t.Helper()
	root := repoRootDir(t)
	out := walkGuidanceFiles(t, root)
	for k, v := range canonicalGuidanceSurfaces() {
		out[k] = v
	}
	return out
}

// TestDestructiveGuidanceSweep_KnownSitesOnly is AC-10(iii): the
// guidance sweep finds zero floor matches outside the known-sites
// exemption list.
func TestDestructiveGuidanceSweep_KnownSitesOnly(t *testing.T) {
	found := scanGuidanceSurfaces(allGuidanceSurfaces(t))
	unlisted, staleCount, staleAbsent := diffGuidanceHits(found, exemptionMap(guard.KnownSitesExemptionList))
	if len(unlisted) > 0 {
		t.Errorf("UNLISTED floor match(es) on swept guidance surfaces (add a reviewed exemption entry or remove the destructive text):\n  %s", strings.Join(unlisted, "\n  "))
	}
	if len(staleCount) > 0 {
		t.Errorf("exemption entry count drift (duplication or partial removal):\n  %s", strings.Join(staleCount, "\n  "))
	}
	if len(staleAbsent) > 0 {
		t.Errorf("STALE exemption entry — its quoted line is no longer present verbatim (greening by deleting/rewording the warning is itself a red diff):\n  %s", strings.Join(staleAbsent, "\n  "))
	}
}

// bypassBlockBannedStrings is AC-10(i)'s own literal enumeration for
// .claude/agents/spec-orchestrator.md's deleted bypass block: none of
// these six strings may appear anywhere in the file, whether or not
// any of them would ALSO be caught by the general floor-match sweep
// above (`bd update ... --metadata` in particular matches no R5(a)
// family at all — Req-19's ban is a distinct, older mechanism — so
// TestDestructiveGuidanceSweep_KnownSitesOnly's classifier-driven check
// cannot stand in for this one).
var bypassBlockBannedStrings = []string{
	"git stash push",
	"git stash drop",
	"git merge --no-ff bead/",
	"git worktree remove",
	"git branch -D",
	"bd update",
}

// bypassBlockPinnedInvocations is AC-10(i)'s replacement-guidance leg:
// the three named, invocable recoveries R5(e) requires in place of the
// deleted bypass block. Each is asserted present verbatim (not merely
// "some mindspec command appears") — AC-11(a)'s own leaf-identity
// resolution is a SEPARATE obligation (cmd/mindspec/named_invocation_test.go),
// this test only pins that the TEXT survives in the guidance artifact.
var bypassBlockPinnedInvocations = []string{
	`mindspec complete <bead-id> "<one-line description>"`,
	"mindspec repair phase <spec-id>",
	"mindspec panel create <slug> --spec",
}

// TestSpecOrchestratorTemplate_BypassBlockReplaced is AC-10(i): the raw
// bypass block (stash / raw merge / forced worktree removal / branch
// -D / raw `bd update --metadata`) is gone from the shipped agent
// template, and its replacement names the three pinned, non-destructive
// invocations R5(e) mandates. Red-on-revert: restoring any one of the
// six banned strings, or deleting any one of the three pinned
// invocations, REDs this test (verified manually at authoring time by
// reintroducing each banned string in turn and re-running this test).
func TestSpecOrchestratorTemplate_BypassBlockReplaced(t *testing.T) {
	root := repoRootDir(t)
	path := filepath.Join(root, ".claude", "agents", "spec-orchestrator.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	content := string(data)

	for _, banned := range bypassBlockBannedStrings {
		if strings.Contains(content, banned) {
			t.Errorf("%s still contains banned bypass-block string %q — spec 127 R5(e) removes the raw bypass block outright", path, banned)
		}
	}
	for _, want := range bypassBlockPinnedInvocations {
		if !strings.Contains(content, want) {
			t.Errorf("%s is missing pinned replacement invocation %q — R5(e)'s replacement guidance names this invocation verbatim", path, want)
		}
	}
}

// TestDestructiveGuidanceSweep_ExemptionEntriesNotHollow is the
// exemption list's assertion (ii): every entry's quoted string still
// matches its recorded floor family, independent of the live scan —
// catching a hollow entry (one that never matched anything) or
// classifier drift.
func TestDestructiveGuidanceSweep_ExemptionEntriesNotHollow(t *testing.T) {
	if problems := hollowExemptionEntries(guard.KnownSitesExemptionList); len(problems) > 0 {
		for _, p := range problems {
			t.Error(p)
		}
	}
}

// hollowExemptionEntries is the shipped hollow-entry detector, factored
// out so TestDestructiveGuidanceSweep_ExemptionEntriesNotHollow (over
// the real list) and the hollow subtest below (over a mutated COPY)
// call the identical mechanism — spec 127 bead-2 rework, O2-r2-14: the
// prior hollow subtest inlined its own copy of this loop and asserted
// only fixture setup, so deleting this detector would have left that
// subtest green.
func hollowExemptionEntries(list []guard.KnownSiteExemptionEntry) []string {
	var problems []string
	for _, e := range list {
		matches := guard.FindFloorMatches(e.Text)
		found := false
		for _, m := range matches {
			if m.Family == e.Family {
				found = true
			}
		}
		if !found {
			problems = append(problems, fmt.Sprintf("exemption entry %s/%q recorded family %s, but the classifier no longer matches it (hollow entry or classifier drift): got %+v", e.Surface, e.Text, e.Family, matches))
		}
	}
	return problems
}

// TestDestructiveGuidanceSweep_UnlistedFixtures_AllRenderingForms pins
// AC-10(iii)'s "unlisted matches fail in every rendering form" clause
// — form is irrelevant by design (H-r6-2): a fenced block, an inline
// code span, and bare prose all trigger the same finding.
func TestDestructiveGuidanceSweep_UnlistedFixtures_AllRenderingForms(t *testing.T) {
	cases := map[string]string{
		"fenced":     "```\ngit branch -D bead/x\n```",
		"inline":     "run `git branch -D bead/x` to clean up",
		"bare-prose": "you can just run git branch -D bead/x here",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			surfaces := map[string]string{"synthetic-fixture.md": content}
			found := scanGuidanceSurfaces(surfaces)
			unlisted, _, _ := diffGuidanceHits(found, exemptionMap(guard.KnownSitesExemptionList))
			if len(unlisted) == 0 {
				t.Fatalf("%s form: expected an unlisted floor match, found none", name)
			}
		})
	}
}

// TestDestructiveGuidanceSweep_WarningDeletionRewordDuplicationHollow
// exercises the design principle's mechanical closure (spec 127
// R5a): greening the sweep by removing, rewording, or duplicating a
// KNOWN site's warning is itself a red diff — never a passing one.
//
// Rewritten in spec 127 bead-2's rework round (RULING 6/O2-r2-14/
// O3-r2-8): the deletion/reword/duplication cases now mutate a COPY of
// the REAL allGuidanceSurfaces() map (deleting/rewording/duplicating
// the listed line IN the real file's content) and diff against the
// REAL guard.KnownSitesExemptionList — pinning diffGuidanceHits's
// behavior when a real, listed warning changes, not merely its
// comparison logic over hand-built one-key maps (AC-10(iv)'s
// canonical-surface plant fixtures already used this stronger shape;
// this test now matches it). The "hollow" case calls the shipped
// hollowExemptionEntries detector directly, and "duplication"
// literally duplicates the line in content rather than incrementing a
// count map.
func TestDestructiveGuidanceSweep_WarningDeletionRewordDuplicationHollow(t *testing.T) {
	if len(guard.KnownSitesExemptionList) == 0 {
		t.Fatal("exemption list is empty — nothing to exercise")
	}
	entry := guard.KnownSitesExemptionList[0]
	real := allGuidanceSurfaces(t)
	content, ok := real[entry.Surface]
	if !ok || !strings.Contains(content, entry.Text) {
		t.Fatalf("fixture setup error: real surface %q does not contain entry.Text verbatim", entry.Surface)
	}
	exempt := exemptionMap(guard.KnownSitesExemptionList)

	copyOf := func() map[string]string {
		m := make(map[string]string, len(real))
		for k, v := range real {
			m[k] = v
		}
		return m
	}

	t.Run("deletion", func(t *testing.T) {
		mutated := copyOf()
		mutated[entry.Surface] = strings.Replace(content, entry.Text, "the warning is gone now", 1)
		found := scanGuidanceSurfaces(mutated)
		_, _, staleAbsent := diffGuidanceHits(found, exempt)
		if len(staleAbsent) == 0 {
			t.Fatal("expected deleting the real listed line to register as a stale-absent entry")
		}
	})

	t.Run("reword", func(t *testing.T) {
		mutated := copyOf()
		mutated[entry.Surface] = strings.Replace(content, entry.Text, entry.Text+" (reworded)", 1)
		found := scanGuidanceSurfaces(mutated)
		unlisted, _, staleAbsent := diffGuidanceHits(found, exempt)
		if len(staleAbsent) == 0 {
			t.Fatal("expected the reworded real line to register as stale-absent under the original quoted string")
		}
		if len(unlisted) == 0 {
			t.Fatal("expected the reworded line itself to be an unlisted floor match")
		}
	})

	t.Run("duplication", func(t *testing.T) {
		mutated := copyOf()
		// Duplicate the line IN CONTENT (not by incrementing a count
		// map) — scanGuidanceSurfaces' own occurrence counting must be
		// what registers the drift.
		mutated[entry.Surface] = content + "\n" + entry.Text
		found := scanGuidanceSurfaces(mutated)
		_, staleCount, _ := diffGuidanceHits(found, exempt)
		if len(staleCount) == 0 {
			t.Fatal("expected a real duplicated line to register as a count-drift stale entry")
		}
	})

	t.Run("hollow", func(t *testing.T) {
		mutated := make([]guard.KnownSiteExemptionEntry, len(guard.KnownSitesExemptionList))
		copy(mutated, guard.KnownSitesExemptionList)
		mutated = append(mutated, guard.KnownSiteExemptionEntry{Surface: "synthetic", Text: "mindspec complete <bead>", Count: 1, Family: guard.FamilyGitMerge})
		problems := hollowExemptionEntries(mutated)
		if len(problems) == 0 {
			t.Fatal("expected the shipped hollow-entry detector to flag the synthetic hollow entry")
		}
	})
}

// TestDestructiveGuidanceSweep_CanonicalSurfaceMutationFixtures is
// AC-10(iv): planting a floor match in a lifecycle-skill Go literal
// AND in a managed-block literal (via the CanonicalGuidanceSurfaces
// test seam) REDs the sweep — the tracked-copy sweep alone cannot
// catch canonical-literal drift.
func TestDestructiveGuidanceSweep_CanonicalSurfaceMutationFixtures(t *testing.T) {
	base := canonicalGuidanceSurfaces()
	exempt := exemptionMap(guard.KnownSitesExemptionList)

	t.Run("lifecycle-skill-literal", func(t *testing.T) {
		mutated := map[string]string{}
		for k, v := range base {
			mutated[k] = v
		}
		mutated["ms-impl-approve"] = mutated["ms-impl-approve"] + "\nplant: git branch -D bead/x\n"
		found := scanGuidanceSurfaces(mutated)
		unlisted, _, _ := diffGuidanceHits(found, exempt)
		if len(unlisted) == 0 {
			t.Fatal("expected the planted floor match in the ms-impl-approve skill-map literal to be unlisted-RED")
		}
	})

	t.Run("managed-block-literal", func(t *testing.T) {
		mutated := map[string]string{}
		for k, v := range base {
			mutated[k] = v
		}
		mutated["claudeMDManagedBlock"] = mutated["claudeMDManagedBlock"] + "\nplant: git branch -D bead/x\n"
		found := scanGuidanceSurfaces(mutated)
		unlisted, _, _ := diffGuidanceHits(found, exempt)
		if len(unlisted) == 0 {
			t.Fatal("expected the planted floor match in claudeMDManagedBlock to be unlisted-RED")
		}
	})
}

// allowlistKey/registryKey mirror rSite.Key()'s content grain (file +
// func + detail) for the two guard-side registries, so the product-
// diagnostic scan can two-way-compare against them the same way
// diffSites already does for the ratchet scans.
func allowlistMap() map[string]bool {
	m := map[string]bool{}
	for _, e := range guard.DestructiveGuidanceAllowlist {
		// Reconstruct detailFor's "<Family>:<matched text>" compound —
		// registries.go stores the plain matched text (content grain,
		// directly re-checkable by FindFloorMatches); the live scan's
		// key additionally carries the family label.
		m[rSite{Rel: e.File, Func: e.Func, Detail: string(e.Family) + ":" + e.Detail}.Key()] = true
	}
	return m
}

func registryMap() map[string]bool {
	m := map[string]bool{}
	for _, e := range guard.OpaqueOperandRegistry {
		m[rSite{Rel: e.File, Func: e.Func, Detail: e.Detail}.Key()] = true
	}
	return m
}

// TestProductDiagnosticScan_DestructiveMatchesAreGoverned is AC-9's
// core positive claim over the product-diagnostic leg: every floor-
// family match found anywhere in scope (guard.NewFailure/FormatFailure
// arguments, RecoveryCommand-shaped method returns, Message-keyed
// composite-literal fields) is EITHER constructor-derived OR present
// on the destructive-guidance allowlist — never neither. The
// AC-9(i) injection fixture (a NEW, unlisted match) is exercised
// separately below over a synthetic internal/approve-shaped fixture,
// not over the real universe (which must stay green).
func TestProductDiagnosticScan_DestructiveMatchesAreGoverned(t *testing.T) {
	u := loadRatchetUniverse(t)
	f := newPkgConstFolder(u)
	findings := scanProductDiagnostics(t, u, f)
	allow := allowlistMap()
	var ungoverned []string
	for _, fd := range findings {
		if len(fd.matches) == 0 {
			continue
		}
		if fd.provenance || allow[fd.site.Key()] {
			continue
		}
		ungoverned = append(ungoverned, fd.site.Key()+" -> "+fd.site.Detail)
	}
	if len(ungoverned) > 0 {
		t.Errorf("destructive match(es) neither constructor-derived nor allowlisted:\n  %s", strings.Join(ungoverned, "\n  "))
	}
}

// TestProductDiagnosticScan_UnprovableCommandOperandsAreGoverned is
// the opaque-operand half: every command-position operand (never the
// free-form message position — see diagFinding.commandPosition) the
// fold rules cannot prove FULLY LITERAL is EITHER constructor-derived
// OR present on the opaque-operand registry. Gates on fullyLiteral,
// not hasLiteral (spec 127 bead-2 rework, RULING 5/S1-r2-1): the old
// hasLiteral-only gate let a PARTIALLY-folded operand — matches=[],
// hasLiteral=true — through unregistered whenever the destructive
// content actually lived entirely in the unfoldable remainder (a
// helper-function call, a method call, a closure call, or a
// Sprintf-substituted argument all fold to the SAME shape).
func TestProductDiagnosticScan_UnprovableCommandOperandsAreGoverned(t *testing.T) {
	u := loadRatchetUniverse(t)
	f := newPkgConstFolder(u)
	findings := scanProductDiagnostics(t, u, f)
	registry := registryMap()
	var ungoverned []string
	for _, fd := range findings {
		if len(fd.matches) > 0 || fd.fullyLiteral || !fd.commandPosition {
			continue
		}
		if fd.provenance || registry[fd.site.Key()] {
			continue
		}
		ungoverned = append(ungoverned, fd.site.Key()+" -> "+fd.site.Detail)
	}
	if len(ungoverned) > 0 {
		t.Errorf("unprovable command operand(s) neither constructor-derived nor registered:\n  %s", strings.Join(ungoverned, "\n  "))
	}
}

// TestProductDiagnosticScan_AllowlistMembershipExact is the allowlist's
// additions-red / stale-entry two-way check over the LIVE scan: every
// allowlist entry must correspond to a real, still-matching finding
// (a converted site whose entry lingers is stale), and every real
// destructive-match finding not constructor-derived must be on the
// allowlist (an addition with no entry is red — proven by the
// previous test; this test proves the OTHER direction).
func TestProductDiagnosticScan_AllowlistMembershipExact(t *testing.T) {
	u := loadRatchetUniverse(t)
	f := newPkgConstFolder(u)
	findings := scanProductDiagnostics(t, u, f)
	live := map[string]bool{}
	for _, fd := range findings {
		if len(fd.matches) > 0 && !fd.provenance {
			live[fd.site.Key()] = true
		}
	}
	for _, e := range guard.DestructiveGuidanceAllowlist {
		k := rSite{Rel: e.File, Func: e.Func, Detail: string(e.Family) + ":" + e.Detail}.Key()
		if !live[k] {
			t.Errorf("STALE allowlist entry (no matching live finding — the site was converted; delete the entry): %s", k)
		}
	}
}

// TestProductDiagnosticScan_OpaqueRegistryMembershipExact is the
// opaque-operand registry's stale-entry check, mirroring the allowlist
// check above.
func TestProductDiagnosticScan_OpaqueRegistryMembershipExact(t *testing.T) {
	u := loadRatchetUniverse(t)
	f := newPkgConstFolder(u)
	findings := scanProductDiagnostics(t, u, f)
	live := map[string]bool{}
	for _, fd := range findings {
		if len(fd.matches) == 0 && !fd.fullyLiteral && fd.commandPosition && !fd.provenance {
			live[fd.site.Key()] = true
		}
	}
	for _, e := range guard.OpaqueOperandRegistry {
		k := rSite{Rel: e.File, Func: e.Func, Detail: e.Detail}.Key()
		if !live[k] {
			t.Errorf("STALE opaque-operand entry (no matching live finding — the site was converted; delete the entry): %s", k)
		}
	}
}

// TestProductDiagnosticScan_InjectionFixture is AC-9(i)'s core
// mechanism proof: a static `git merge` hint landed in a SYNTHETIC
// internal/approve-shaped fixture (a throwaway universe, never the
// real package — the real tree must stay green) is found by the scan
// and named by file:line, unlisted (no allowlist entry exists for a
// site that was never seeded).
func TestProductDiagnosticScan_InjectionFixture(t *testing.T) {
	src := `package approve

import "github.com/mrmaxsteel/mindspec/internal/guard"

func injectedRefusal(beadBranch string) error {
	return guard.NewFailure("a synthetic refusal", "git merge --no-ff "+beadBranch)
}
`
	u := singleFileUniverse(t, "internal/approve/injected_fixture.go", src)
	f := newPkgConstFolder(u)
	findings := scanProductDiagnostics(t, u, f)
	found := false
	for _, fd := range findings {
		if len(fd.matches) == 0 {
			continue
		}
		if fd.site.Rel == "internal/approve/injected_fixture.go" && fd.site.Func == "injectedRefusal" {
			found = true
			if fd.site.Line == 0 {
				t.Error("expected the finding to carry a real line number")
			}
		}
	}
	if !found {
		t.Fatal("expected the scan to find and name the injected git-merge hint in internal/approve/injected_fixture.go")
	}
	// And it must be unlisted — nothing seeded this synthetic site.
	if allowlistMap()[rSite{Rel: "internal/approve/injected_fixture.go", Func: "injectedRefusal", Detail: "git merge:git merge"}.Key()] {
		t.Fatal("fixture setup error: the synthetic site must not already be on the allowlist")
	}
}

// injectionTableCase is one row of TestProductDiagnosticScan_InjectionTable
// below. exprSrc is a Go expression (referencing `dynamic string`,
// the enclosing function's parameter) rendered as the SECOND argument
// to guard.NewFailure — the operand shape a real internal/approve-style
// refusal builds. family is the expected match ("" for the four
// recorded non-match exclusions).
type injectionTableCase struct {
	name    string
	family  guard.DestructiveFamily
	exprSrc string
}

// TestProductDiagnosticScan_InjectionTable is AC-9(i)'s full mechanism
// proof, delivered at the boundary the AC actually specifies (spec
// 127 bead-2 rework, RULING 9/F1-r2-3/O2-r2-7/O3-r2-2): ONE probe per
// floor family — all 17 — driven through the REAL SCAN over a
// synthetic internal/approve-shaped fixture, plus option-cluster
// re-spellings, BOTH global-option normalization forms (`+`-
// concatenation AND fmt.Sprintf) for BOTH `-C` and `-c`, and all four
// recorded non-match exclusions — asserting the scan REDs each
// positive (naming file:line) and stays green on every exclusion.
// TestFindFloorMatches_OnePerFamily (classifier_test.go, package
// guard) proves the CLASSIFIER's own contract; that proof does NOT by
// itself show the SCAN's fold layer reaches every family in every
// operand shape — the scan feeds the classifier a FOLDED skeleton,
// never the author's original string, and TestProductDiagnosticScan_
// DynamicGapGitCNormalization above is the proof that gap is not
// theoretical (a normalization clause that passed at the unit level
// failed through the scan before RULING 5's fix).
func TestProductDiagnosticScan_InjectionTable(t *testing.T) {
	cases := []injectionTableCase{
		{"merge", guard.FamilyGitMerge, `"git merge --no-ff "+dynamic`},
		{"reset", guard.FamilyGitReset, `"git reset --hard "+dynamic`},
		{"restore", guard.FamilyGitRestore, `"git restore "+dynamic`},
		{"clean", guard.FamilyGitCleanForce, `"git clean -fd "+dynamic`},
		{"branch-delete", guard.FamilyGitBranchDeleteForce, `"git branch -D "+dynamic`},
		{"push-force", guard.FamilyGitPushForce, `"git push --force "+dynamic`},
		{"stash-drop", guard.FamilyGitStashDropClear, `"git stash drop "+dynamic`},
		{"worktree-remove-force", guard.FamilyGitWorktreeRemoveForce, `"git worktree remove --force "+dynamic`},
		{"checkout-pathspec", guard.FamilyGitCheckoutPathspecDiscard, `"git checkout -- "+dynamic`},
		{"switch-discard", guard.FamilyGitSwitchDiscardChanges, `"git switch --discard-changes "+dynamic`},
		{"checkout-force", guard.FamilyGitCheckoutForce, `"git checkout -f "+dynamic`},
		{"update-ref-delete", guard.FamilyGitUpdateRefDelete, `"git update-ref -d "+dynamic`},
		{"tag-delete", guard.FamilyGitTagDelete, `"git tag -d "+dynamic`},
		{"push-delete", guard.FamilyGitPushDelete, `"git push origin --delete "+dynamic`},
		{"reflog-expire", guard.FamilyGitReflogExpireOrGCPrune, `"git reflog expire --expire=now "+dynamic`},
		{"rm-rf", guard.FamilyRmRf, `"rm -rf "+dynamic`},
		{"bd-delete-force", guard.FamilyBdDeleteForce, `"bd delete "+dynamic+" --force"`},

		// Option-cluster re-spellings (R5(a)'s equivalence).
		{"clean-cluster", guard.FamilyGitCleanForce, `"git clean -fdx "+dynamic`},
		{"rm-cluster", guard.FamilyRmRf, `"rm -fr "+dynamic`},
		{"push-force-short", guard.FamilyGitPushForce, `"git push -f "+dynamic`},

		// Global-option normalization: BOTH forms, BOTH -C and -c
		// (RULING 5/O3-r2-1's own regression class).
		{"dash-C-concat", guard.FamilyGitReset, `"git -C "+dynamic+" reset --hard"`},
		{"dash-c-concat", guard.FamilyGitReset, `"git -c "+dynamic+" reset --hard"`},
		{"dash-C-sprintf", guard.FamilyGitReset, `fmt.Sprintf("git -C %s reset --hard", dynamic)`},
		{"dash-c-sprintf", guard.FamilyGitReset, `fmt.Sprintf("git -c %s reset --hard", dynamic)`},

		// The four recorded non-match exclusions.
		{"exclusion-merge-abort", "", `"git merge --abort "+dynamic`},
		{"exclusion-worktree-prune", "", `"git worktree prune "+dynamic`},
		{"exclusion-rm-cached", "", `"git rm --cached "+dynamic`},
		{"exclusion-push-force-with-lease", "", `"git push --force-with-lease "+dynamic`},
	}

	// AC-9(i)'s own "one probe per floor family" claim, driven off
	// guard.AllFamilies rather than trusted by construction (spec 127
	// bead-2 rework round 2, O2c-2, MAJOR): round 1 shipped this as a
	// hardcoded 28-row literal with no coverage sentinel — round 1's own
	// F1-3 required exactly this fix and it did not land. Mirrors
	// classifier_test.go's TestFindFloorMatches_OnePerFamily sentinel at
	// the SCAN's own boundary (not just the classifier's): a family
	// added to AllFamilies/matchGit/matchBd/matchRm without a
	// corresponding row here is named explicitly, not merely absorbed
	// into an unnoticed green run.
	covered := map[guard.DestructiveFamily]bool{}
	for _, c := range cases {
		if c.family != "" {
			covered[c.family] = true
		}
	}
	for _, fam := range guard.AllFamilies {
		if !covered[fam] {
			t.Errorf("family %s has no probe in this table (AC-9(i): one probe per floor family)", fam)
		}
	}
	if got, want := len(covered), guard.DestructiveFamilyCount; got != want {
		t.Errorf("this table covers %d distinct families, guard.DestructiveFamilyCount says %d — keep them in lockstep", got, want)
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rel := "internal/approve/injection_table_" + strings.ReplaceAll(c.name, "-", "_") + "_fixture.go"
			src := "package approve\n\n" +
				`import (` + "\n" +
				`	"fmt"` + "\n\n" +
				`	"github.com/mrmaxsteel/mindspec/internal/guard"` + "\n" +
				")\n\n" +
				"func injectedRefusal(dynamic string) error {\n" +
				"	return guard.NewFailure(\"a synthetic refusal\", " + c.exprSrc + ")\n" +
				"}\n"
			u := singleFileUniverse(t, rel, src)
			f := newPkgConstFolder(u)
			findings := scanProductDiagnostics(t, u, f)

			var matchedHere []guard.Match
			var line int
			for _, fd := range findings {
				if fd.site.Rel != rel || fd.site.Func != "injectedRefusal" {
					continue
				}
				matchedHere = append(matchedHere, fd.matches...)
				if fd.site.Line != 0 {
					line = fd.site.Line
				}
			}

			if c.family == "" {
				if len(matchedHere) > 0 {
					t.Fatalf("expected NO floor match (recorded exclusion), got %+v", matchedHere)
				}
				return
			}
			found := false
			for _, m := range matchedHere {
				if m.Family == c.family {
					found = true
				}
			}
			if !found {
				t.Fatalf("expected the scan to find %s in %s, got %+v", c.family, rel, matchedHere)
			}
			if line == 0 {
				t.Error("expected the finding to carry a real line number")
			}
		})
	}
}

// TestProductDiagnosticScan_DynamicGapGitCNormalization pins RULING 5/
// O3-r2-1 through the SCAN, not just the classifier: a dynamic `git
// -C <path>` operand — `"git -C " + wtPath + " reset --hard"`, this
// codebase's own dominant command-building shape — must still be
// found as a real FamilyGitReset match. Before this fix, foldExpr's
// bare-space substitution for the unfoldable `wtPath` fragment folded
// to `git -C   reset --hard`, and skipGlobalOptions then consumed the
// literal word "reset" as -C's own operand, making the match
// disappear entirely.
func TestProductDiagnosticScan_DynamicGapGitCNormalization(t *testing.T) {
	src := `package approve

import "github.com/mrmaxsteel/mindspec/internal/guard"

func dynamicGapRefusal(wtPath string) error {
	return guard.NewFailure("a synthetic refusal", "git -C "+wtPath+" reset --hard")
}
`
	u := singleFileUniverse(t, "internal/approve/dynamic_gap_fixture.go", src)
	f := newPkgConstFolder(u)
	findings := scanProductDiagnostics(t, u, f)
	found := false
	for _, fd := range findings {
		for _, m := range fd.matches {
			if m.Family == guard.FamilyGitReset && fd.site.Func == "dynamicGapRefusal" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("expected the scan to find a FamilyGitReset match through the dynamic -C gap")
	}
}

// TestProductDiagnosticScan_PartialFoldDestructiveInUnfoldableHalf
// pins RULING 5/S1-r2-1: a `+`-concatenation whose destructive content
// lives ENTIRELY in an unfoldable fragment (a helper-function call) —
// `"note: " + buildRecovery(beadBranch)` where buildRecovery itself
// returns `"git branch -D " + beadBranch` — must not be silently
// exempted from the opaque-operand governance gate just because its
// OTHER half ("note: ") is literal. Before this fix, any partial fold
// set hasLiteral=true unconditionally, and the unprovable-operand gate
// skipped every hasLiteral=true finding regardless of whether the
// fold was actually complete.
func TestProductDiagnosticScan_PartialFoldDestructiveInUnfoldableHalf(t *testing.T) {
	src := `package approve

import "github.com/mrmaxsteel/mindspec/internal/guard"

func buildRecovery(beadBranch string) string {
	return "git branch -D " + beadBranch
}

func partialFoldRefusal(beadBranch string) error {
	return guard.NewFailure("a refusal", "note: "+buildRecovery(beadBranch))
}
`
	u := singleFileUniverse(t, "internal/approve/partial_fold_fixture.go", src)
	f := newPkgConstFolder(u)
	findings := scanProductDiagnostics(t, u, f)
	var target *diagFinding
	for i := range findings {
		if findings[i].site.Func == "partialFoldRefusal" && findings[i].commandPosition {
			target = &findings[i]
		}
	}
	if target == nil {
		t.Fatal("expected a command-position finding for partialFoldRefusal's second argument")
	}
	if len(target.matches) != 0 {
		t.Fatalf("fixture setup error: expected the STATIC fold to see no match (the destructive content lives in the unresolved call), got %+v", target.matches)
	}
	if target.fullyLiteral {
		t.Fatal("expected fullyLiteral=false — the buildRecovery(...) fragment is unresolved, so this finding must require registry/provenance governance")
	}
}

// TestFoldExpr_FullyLiteralAcrossAllFourPaths is bead-2 rework round
// 2's O2c-1 (BLOCKING) fix, pinned directly: foldExpr's own doc
// comment claims fullyLiteral is false whenever ANY leaf did not
// fold, "whatever shape reaches it" — but round 2's confirm measured
// that claim true for exactly ONE of foldExpr's four fold paths (the
// direct BinaryExpr leaf, pinned above) and false for the other
// three. Each subtest below launders the SAME partially-foldable
// value — a literal prefix concatenated with buildRecovery(b), an
// opaque helper call returning a genuinely destructive string — through
// one of the three previously-broken paths, plus the direct-BinaryExpr
// control for comparison. A fifth fold path added later without this
// same treatment is exactly the regression this test exists to catch:
// it is not parametrized over an enum of paths (none exists — a fold
// path is a case arm in foldExpr's own type switch), so the honest
// claim this test backs is "these four, individually, are right
// today", not "every future path will be caught automatically".
func TestFoldExpr_FullyLiteralAcrossAllFourPaths(t *testing.T) {
	const helper = `func buildRecovery(b string) string {
	return "git branch -D " + b
}

`
	cases := []struct {
		name string
		body string
	}{
		{
			name: "directPartial-BinaryExpr-control",
			body: `func directPartialRefusal(b string) error {
	return guard.NewFailure("a refusal", "note: "+buildRecovery(b))
}
`,
		},
		{
			name: "bindLaunder-localBind",
			body: `func bindLaunderRefusal(b string) error {
	cmd := "note: " + buildRecovery(b)
	return guard.NewFailure("a refusal", cmd)
}
`,
		},
		{
			name: "sprintfLaunder-fmtSprintf",
			body: `func sprintfLaunderRefusal(b string) error {
	return guard.NewFailure("a refusal", fmt.Sprintf("note: %s", buildRecovery(b)))
}
`,
		},
		{
			name: "varLaunder-packageLevelVar",
			body: `var zzWrapped = "note: " + buildRecovery("x")

func varLaunderRefusal() error {
	return guard.NewFailure("a refusal", zzWrapped)
}
`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := "package approve\n\nimport (\n\t\"fmt\"\n\n\t\"github.com/mrmaxsteel/mindspec/internal/guard\"\n)\n\n" + helper + c.body
			rel := "internal/approve/fold_launder_" + strings.ReplaceAll(c.name, "-", "_") + "_fixture.go"
			u := singleFileUniverse(t, rel, src)
			f := newPkgConstFolder(u)
			findings := scanProductDiagnostics(t, u, f)
			var target *diagFinding
			for i := range findings {
				if findings[i].commandPosition && strings.Contains(findings[i].site.Func, "Refusal") {
					target = &findings[i]
				}
			}
			if target == nil {
				t.Fatalf("expected a command-position finding in %s", rel)
			}
			if len(target.matches) != 0 {
				t.Fatalf("fixture setup error: expected no STATIC match (content lives in the unresolved buildRecovery call), got %+v", target.matches)
			}
			if target.fullyLiteral {
				t.Fatalf("expected fullyLiteral=false for %s — the buildRecovery(...) fragment is unresolved regardless of which fold path carries it; a true result here means this shape LAUNDERS a genuinely destructive operand past the opaque-operand governance gate", c.name)
			}
		})
	}
}

// singleFileUniverse parses src as a single file at the given repo-
// relative path and registers it into a fresh, throwaway rUniverse —
// used for injection/provenance fixtures that must never touch the
// real production tree.
func singleFileUniverse(t *testing.T, rel, src string) *rUniverse {
	t.Helper()
	u := &rUniverse{
		fset:    token.NewFileSet(),
		consts:  map[string]map[string]string{},
		pkgVars: map[string]map[string]ast.Expr{},
		funcs:   map[string][]*rFunc{},
	}
	f, err := parser.ParseFile(u.fset, rel, src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parsing fixture %s: %v", rel, err)
	}
	u.addFile(rel, f)
	return u
}

// ---------------------------------------------------------------------
// Same-package construction invariant (spec 127 R5b/G-r5-3): opacity
// seals only CROSS-package construction of guard.DestructiveCommand;
// any file inside package guard can populate its unexported fields
// directly (Go does not scope unexported-field access below the
// package). This invariant FLAGS that gap — it does NOT close it, and
// as of spec 127 bead-2 rework round 3's RULING 1 no comment in this
// codebase should claim otherwise: a populated composite literal, an
// unexported-field write, or a type CONVERSION producing the opaque
// type is red anywhere in a "package guard" file OUTSIDE
// NewDestructiveCommand's own implementation IN THE SHAPES LISTED
// BELOW — a syntactic (AST-level, no go/types) check over an
// unboundedly spellable language, layered UNDER human review of any
// same-package change, never a substitute for it (see constructor.go's
// own doc comment for the full accounting of why "comprehensive" was
// the wrong word for this mechanism). Package identity is by PACKAGE
// CLAUSE name ("package guard"), not by repo path, so a fixture
// universe need not live at a real "internal/guard" path to be
// recognized.
//
// Widened (spec 127 bead-2 rework, RULING 2/G1-r2-1/O2-r2-1): the
// original check matched a composite literal's Type ONLY against the
// literal identifier "DestructiveCommand", missing three same-package
// forges that need neither unsafe nor reflect, all verified to
// compile and produce a Valid()==true value with the real invariant
// test staying green while present: (a) a TYPE ALIAS composite
// literal (`type dcAlias = DestructiveCommand; dcAlias{...}` — this
// IS a composite literal of that exact type; node.Type is the Ident
// "dcAlias", which the old identifier-equality test missed); (b) a
// shadow struct with an identical underlying field set, CONVERTED
// across (`DestructiveCommand(dcShadow{...})` — nothing flagged the
// conversion expression at all); (c) a DEFINED type
// (`type dcDefined DestructiveCommand`) composite-literalled then
// converted. Two passes now: first, collect every package-level type
// name that IS DestructiveCommand, an alias of it, or a defined type
// over it (resolving `type X = DestructiveCommand` and
// `type X DestructiveCommand` across every "package guard" file in
// the universe, not just the current one — an alias may be declared
// in one file and forged in another); second, flag a composite
// literal OR a conversion CallExpr naming any of those resolved type
// names, outside the constructor. This closes (a) and (c) via the
// composite-literal leg (dcAlias/dcDefined are now in the resolved
// name set) and (b) via the new conversion-CallExpr leg (a bare
// `DestructiveCommand(...)` call outside the constructor is always
// red, regardless of what its argument's own type is).
func scanSamePackageInvariant(u *rUniverse) []string {
	var problems []string
	var guardFiles []*rFile
	for _, file := range u.files {
		if file.file.Name != nil && file.file.Name.Name == "guard" {
			guardFiles = append(guardFiles, file)
		}
	}

	// Pass 1: resolve type identity — DestructiveCommand itself, plus
	// every alias (`type X = DestructiveCommand`) and defined type
	// (`type X DestructiveCommand`) declared anywhere in the universe's
	// "package guard" files, to a FIXED POINT (spec 127 bead-2 rework
	// round 2, G1-1/O2-1: round 1's single pass only admitted a
	// TypeSpec whose RHS was literally the Ident "DestructiveCommand" —
	// an alias OF an alias, e.g. `type L1 = DestructiveCommand; type L2
	// = L1`, was never added to dcTypeNames because L2's own RHS names
	// "L1", not "DestructiveCommand", and no second pass ever
	// revisited it). Also builds structFieldTypes — every struct type
	// declared in package guard, field name -> declared field type —
	// and containerElemTypes — every named array/slice/map type over
	// some other type, name -> that array/slice's element (or map's
	// value) type — both used below to resolve an ELIDED composite
	// literal's effective type one level at a time
	// (checkElidedComposites).
	//
	// This walk visits EVERY *ast.GenDecl of kind TYPE anywhere in the
	// file's AST, via ast.Inspect, not just file.file.Decls' top-level
	// entries (spec 127 bead-2 rework round 3, RULING 1/G1-confirm2-1,
	// BLOCKING): a `type` declaration inside a FUNCTION BODY is exactly
	// as capable of forging a same-package value as a package-level
	// one, and round 2's direct iteration over file.file.Decls never
	// descended into a *ast.FuncDecl's Body to find one. Same-name
	// collision between a function-local type and an unrelated
	// identically-named type elsewhere in the package is an accepted,
	// pre-existing limitation of this scan's flat, package-wide type
	// namespace (this invariant's own header comment above: package
	// identity is by package CLAUSE, not by real Go scoping) — this
	// walk does not add a NEW one.
	dcTypeNames := map[string]bool{"DestructiveCommand": true}
	typeRHS := map[string]ast.Expr{}
	structFieldTypes := map[string]map[string]ast.Expr{}
	containerElemTypes := map[string]ast.Expr{}
	for _, file := range guardFiles {
		ast.Inspect(file.file, func(n ast.Node) bool {
			gd, ok := n.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				return true
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				typeRHS[ts.Name.Name] = ts.Type
				switch rt := ts.Type.(type) {
				case *ast.StructType:
					if rt.Fields != nil {
						fields := map[string]ast.Expr{}
						for _, f := range rt.Fields.List {
							for _, nm := range f.Names {
								fields[nm.Name] = f.Type
							}
						}
						structFieldTypes[ts.Name.Name] = fields
					}
				case *ast.ArrayType:
					// Spec 127 bead-2 rework round 3, RULING 1/O2c2-4
					// (MAJOR): a composite literal whose OUTER type is a
					// NAMED slice/array type over DestructiveCommand
					// (`type zzWrapSlice []DestructiveCommand`) with an
					// elided-type inner element resolves through this
					// lookaside — the same kind of syntactic TypeSpec
					// lookup already performed for `type X = Y`/
					// `type X Y` identity chains above, one AST-node-
					// kind short of what the seal already claimed to do.
					containerElemTypes[ts.Name.Name] = rt.Elt
				case *ast.MapType:
					containerElemTypes[ts.Name.Name] = rt.Value
				}
			}
			return true
		})
	}
	for changed := true; changed; {
		changed = false
		for name, rhs := range typeRHS {
			if dcTypeNames[name] {
				continue
			}
			if id, ok := ast.Unparen(rhs).(*ast.Ident); ok && dcTypeNames[id.Name] {
				dcTypeNames[name] = true
				changed = true
			}
		}
	}

	// Pass 2: flag every composite literal (direct OR elided-type, see
	// checkElidedComposites below) or conversion naming a resolved
	// type-identity member, and every unexported-field write or
	// unsafe/reflect.NewAt usage, outside NewDestructiveCommand's own
	// implementation.
	for _, file := range guardFiles {
		for _, imp := range file.file.Imports {
			if p, ok := unquote(imp.Path.Value); ok && p == "unsafe" {
				problems = append(problems, file.rel+": imports \"unsafe\" — recorded outside this convention's threat model (spec 127 R5b), still red wherever the AST scan can see it")
			}
		}
		for _, decl := range file.file.Decls {
			fn, isFn := decl.(*ast.FuncDecl)
			isConstructorImpl := isFn && fn.Recv == nil && fn.Name.Name == "NewDestructiveCommand"
			ast.Inspect(decl, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.CompositeLit:
					if isConstructorImpl {
						return true
					}
					if id, ok := ast.Unparen(node.Type).(*ast.Ident); ok && dcTypeNames[id.Name] {
						problems = append(problems, file.rel+":"+itoa(u.fset.Position(node.Pos()).Line)+": "+id.Name+" composite literal outside NewDestructiveCommand's implementation (same-package forge)")
					}
					checkElidedComposites(node, dcTypeNames, structFieldTypes, containerElemTypes, func(pos token.Pos, typeName string) {
						problems = append(problems, file.rel+":"+itoa(u.fset.Position(pos).Line)+": "+typeName+" composite literal (elided type, resolved from its enclosing array/slice/map/struct-field context) outside NewDestructiveCommand's implementation (same-package forge)")
					})
				case *ast.CallExpr:
					if isConstructorImpl {
						return true
					}
					if id, ok := ast.Unparen(node.Fun).(*ast.Ident); ok && dcTypeNames[id.Name] {
						problems = append(problems, file.rel+":"+itoa(u.fset.Position(node.Pos()).Line)+": conversion to "+id.Name+" outside NewDestructiveCommand's implementation (same-package forge)")
					}
				case *ast.AssignStmt:
					if isConstructorImpl {
						return true
					}
					for _, lhs := range node.Lhs {
						sel, ok := lhs.(*ast.SelectorExpr)
						if !ok {
							continue
						}
						if sel.Sel.Name == "command" || sel.Sel.Name == "valid" {
							problems = append(problems, file.rel+":"+itoa(u.fset.Position(node.Pos()).Line)+": unexported-field write ."+sel.Sel.Name+" outside NewDestructiveCommand's implementation")
						}
					}
				case *ast.SelectorExpr:
					if xid, ok := node.X.(*ast.Ident); ok && xid.Name == "reflect" && node.Sel.Name == "NewAt" {
						problems = append(problems, file.rel+":"+itoa(u.fset.Position(node.Pos()).Line)+": reflect.NewAt usage — recorded outside this convention's threat model (spec 127 R5b), still red wherever the AST scan can see it")
					}
				}
				return true
			})
		}
	}
	return problems
}

// elidedChildType resolves the type expression a child element of a
// composite literal typed parentType would have if the child's OWN
// Type field is nil (elided) — an array/slice/map literal's element
// inherits parentType's Elt (array/slice) or Value (map) regardless of
// key; a struct literal's KEYED field inherits that field's declared
// type from structFieldTypes; a NAMED array/slice/map type's own
// literal (`zzWrapSlice{{...}}`, parentType an *ast.Ident naming that
// type) inherits its declared element/value type from
// containerElemTypes (spec 127 bead-2 rework round 3, RULING 1/
// O2c2-4) — checked BEFORE the key-based struct-field lookaside, since
// a named container's own elements are ordinarily positional (key ==
// nil), unlike struct fields. Returns nil when parentType resolves to
// none of these shapes (e.g. an unresolved cross-package type, or a
// struct literal using positional — unkeyed — elements, which this
// scan does not attempt to resolve field-by-field).
func elidedChildType(parentType ast.Expr, key ast.Expr, structFieldTypes map[string]map[string]ast.Expr, containerElemTypes map[string]ast.Expr) ast.Expr {
	switch pt := ast.Unparen(parentType).(type) {
	case *ast.ArrayType:
		return pt.Elt
	case *ast.MapType:
		return pt.Value
	case *ast.Ident:
		if elem, ok := containerElemTypes[pt.Name]; ok {
			return elem
		}
		if key == nil {
			return nil
		}
		kid, ok := key.(*ast.Ident)
		if !ok {
			return nil
		}
		return structFieldTypes[pt.Name][kid.Name]
	}
	return nil
}

// checkElidedComposites walks lit's own elements for a nested
// *ast.CompositeLit whose Type field is nil — elided, resolved from
// its enclosing context via elidedChildType — reporting through
// report(pos, typeName) whenever the resolved type is a dcTypeNames
// member (spec 127 bead-2 rework round 2, O2-1/O3-confirm-1: an
// elided-type slice/map/struct-field literal — `[]DestructiveCommand{
// {command: cmd, valid: true}}`, `map[int]DestructiveCommand{0: {...}}`
// — has node.Type == nil syntactically, so round 1's direct
// `ast.Unparen(node.Type).(*ast.Ident)` assertion on the INNER literal
// always failed, silently ignoring it; this walk resolves the inner
// literal's type from the OUTER literal it is nested in instead).
// Recurses into every resolved child (using a synthetic copy that
// carries the resolved type forward) so a doubly-nested elision — a
// slice of a struct with an elided-field elided-slice, say — keeps
// resolving one level at a time.
//
// The resolved child type is unwrapped through one *ast.StarExpr
// before the Ident check (spec 127 bead-2 rework round 3, RULING 1/
// O3-confirm2-NEW-1, BLOCKING): an elided-type literal of a POINTER
// element (`[]*DestructiveCommand{{command: cmd, valid: true}}` — Go's
// own composite-literal elision rule applies through a pointer element
// type exactly as it does through a plain one) resolves its Elt to
// `*DestructiveCommand`, not `DestructiveCommand` directly, so the
// bare Ident assertion previously missed it.
func checkElidedComposites(lit *ast.CompositeLit, dcTypeNames map[string]bool, structFieldTypes map[string]map[string]ast.Expr, containerElemTypes map[string]ast.Expr, report func(pos token.Pos, typeName string)) {
	for _, elt := range lit.Elts {
		var key, valueExpr ast.Expr
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			key, valueExpr = kv.Key, kv.Value
		} else {
			valueExpr = elt
		}
		childLit, ok := valueExpr.(*ast.CompositeLit)
		if !ok {
			continue
		}
		if childLit.Type != nil {
			checkElidedComposites(childLit, dcTypeNames, structFieldTypes, containerElemTypes, report)
			continue
		}
		childType := elidedChildType(lit.Type, key, structFieldTypes, containerElemTypes)
		resolved := ast.Unparen(childType)
		if star, ok := resolved.(*ast.StarExpr); ok {
			resolved = ast.Unparen(star.X)
		}
		if id, ok := resolved.(*ast.Ident); ok && dcTypeNames[id.Name] {
			report(childLit.Pos(), id.Name)
		}
		synthetic := *childLit
		synthetic.Type = childType
		checkElidedComposites(&synthetic, dcTypeNames, structFieldTypes, containerElemTypes, report)
	}
}

// TestSamePackageInvariant_RealGuardPackageIsClean proves the
// invariant holds over the REAL internal/guard source today —
// NewDestructiveCommand's own composite literal is correctly exempted
// (a false positive here would mean the invariant itself is broken).
func TestSamePackageInvariant_RealGuardPackageIsClean(t *testing.T) {
	u := loadRatchetUniverse(t)
	if problems := scanSamePackageInvariant(u); len(problems) > 0 {
		t.Fatalf("unexpected same-package-invariant problem(s) in the real internal/guard package:\n  %s", strings.Join(problems, "\n  "))
	}
}

// TestSamePackageInvariant_SamePackageHelperForge is AC-9(vi): a
// helper function elsewhere in package guard that directly populates
// the unexported fields is red.
func TestSamePackageInvariant_SamePackageHelperForge(t *testing.T) {
	src := `package guard

func forgedHelper(cmd string) DestructiveCommand {
	return DestructiveCommand{command: cmd, valid: true}
}
`
	u := singleFileUniverse(t, "internal/guard/forge_fixture.go", src)
	problems := scanSamePackageInvariant(u)
	if len(problems) == 0 {
		t.Fatal("expected the same-package helper forge to be flagged")
	}
}

// TestSamePackageInvariant_WrapperReturnedForgedValue is AC-9(vi): a
// wrapper that assigns the unexported fields via a zero-value var
// (not a composite literal) is also red.
func TestSamePackageInvariant_WrapperReturnedForgedValue(t *testing.T) {
	src := `package guard

func wrapperForge(cmd string) DestructiveCommand {
	var d DestructiveCommand
	d.command = cmd
	d.valid = true
	return d
}
`
	u := singleFileUniverse(t, "internal/guard/wrapper_fixture.go", src)
	problems := scanSamePackageInvariant(u)
	if len(problems) == 0 {
		t.Fatal("expected the wrapper's field-write forge to be flagged")
	}
}

// TestSamePackageInvariant_TypeAliasCompositeLiteral is RULING 2/
// G1-r2-1/O2-r2-1(a): a composite literal of a TYPE ALIAS of
// DestructiveCommand is a composite literal of that exact type
// (`type dcAlias = DestructiveCommand`), even though node.Type is the
// Ident "dcAlias" rather than "DestructiveCommand" — the old
// identifier-equality check missed it entirely.
func TestSamePackageInvariant_TypeAliasCompositeLiteral(t *testing.T) {
	src := `package guard

type dcAlias = DestructiveCommand

func aliasForge(cmd string) DestructiveCommand {
	return dcAlias{command: cmd, valid: true}
}
`
	u := singleFileUniverse(t, "internal/guard/alias_fixture.go", src)
	problems := scanSamePackageInvariant(u)
	if len(problems) == 0 {
		t.Fatal("expected the type-alias composite literal forge to be flagged")
	}
}

// TestSamePackageInvariant_TwoLevelAliasChain is bead-2 rework round
// 2's G1-1 (STILL_OPEN, confirmed): round 1's Pass 1 only admitted a
// TypeSpec whose RHS was literally the Ident "DestructiveCommand" — an
// alias OF an alias (`type L1 = DestructiveCommand; type L2 = L1`) was
// never added to dcTypeNames, since L2's RHS names "L1", not
// "DestructiveCommand", and no fixed-point iteration ever revisited
// it. Round 2 replaces the single pass with a fixed-point loop
// (scanSamePackageInvariant, above) specifically to close this.
func TestSamePackageInvariant_TwoLevelAliasChain(t *testing.T) {
	src := `package guard

type g1AliasLevel1 = DestructiveCommand
type g1AliasLevel2 = g1AliasLevel1

func twoLevelAliasForge(cmd string) DestructiveCommand {
	return g1AliasLevel2{command: cmd, valid: true}
}
`
	u := singleFileUniverse(t, "internal/guard/two_level_alias_fixture.go", src)
	problems := scanSamePackageInvariant(u)
	if len(problems) == 0 {
		t.Fatal("expected the two-level alias chain composite literal forge to be flagged")
	}
}

// TestSamePackageInvariant_ElidedSliceLiteral is bead-2 rework round
// 2's O2-1/O3-confirm-1 (confirmed independently by two slots): a
// composite literal whose element TYPE is elided — inferred from the
// enclosing slice's own element type — has node.Type == nil
// syntactically, so round 1's direct Ident-equality assertion on the
// INNER literal always failed silently. checkElidedComposites (above)
// resolves the inner literal's type from the OUTER `[]DestructiveCommand`
// literal it is nested in.
func TestSamePackageInvariant_ElidedSliceLiteral(t *testing.T) {
	src := `package guard

func forgeViaElidedSliceLiteral(cmd string) DestructiveCommand {
	return []DestructiveCommand{{command: cmd, valid: true}}[0]
}
`
	u := singleFileUniverse(t, "internal/guard/elided_slice_fixture.go", src)
	problems := scanSamePackageInvariant(u)
	if len(problems) == 0 {
		t.Fatal("expected the elided-type slice-literal forge to be flagged")
	}
}

// TestSamePackageInvariant_ElidedMapLiteral mirrors the slice case
// above for a map literal's elided VALUE type (O2-1's second forge).
func TestSamePackageInvariant_ElidedMapLiteral(t *testing.T) {
	src := `package guard

func forgeViaElidedMapLiteral(cmd string) DestructiveCommand {
	return map[int]DestructiveCommand{0: {command: cmd, valid: true}}[0]
}
`
	u := singleFileUniverse(t, "internal/guard/elided_map_fixture.go", src)
	problems := scanSamePackageInvariant(u)
	if len(problems) == 0 {
		t.Fatal("expected the elided-type map-literal forge to be flagged")
	}
}

// TestSamePackageInvariant_ElidedStructFieldLiteral rounds out the
// same-shape family: an elided literal nested as a STRUCT FIELD's
// value (rather than a slice/map element) resolves from that field's
// own declared type, via structFieldTypes.
func TestSamePackageInvariant_ElidedStructFieldLiteral(t *testing.T) {
	src := `package guard

type g1Wrap struct {
	D DestructiveCommand
}

func forgeViaElidedStructFieldLiteral(cmd string) DestructiveCommand {
	w := g1Wrap{D: DestructiveCommand{command: cmd, valid: true}}
	_ = w
	return g1Wrap{D: {command: cmd, valid: true}}.D
}
`
	u := singleFileUniverse(t, "internal/guard/elided_struct_field_fixture.go", src)
	problems := scanSamePackageInvariant(u)
	// Exactly ONE forge is elided (the second literal); the first
	// (explicit DestructiveCommand{...}) is also correctly flagged by
	// the pre-existing direct check, so at least two problems total —
	// asserting >=1 keeps this test robust to either mechanism firing.
	if len(problems) == 0 {
		t.Fatal("expected the elided-type struct-field-literal forge to be flagged")
	}
}

// TestSamePackageInvariant_ElidedPointerElementLiteral is spec 127
// bead-2 rework round 3's RULING 1/O3-confirm2-NEW-1 (BLOCKING): an
// elided-type literal of a POINTER element (`[]*DestructiveCommand{
// {...}}`, valid Go elision-through-pointer) resolved its Elt to
// `*DestructiveCommand`, which the pre-fix bare-Ident assertion never
// matched.
func TestSamePackageInvariant_ElidedPointerElementLiteral(t *testing.T) {
	src := `package guard

func forgeViaElidedPointerElementLiteral(cmd string) DestructiveCommand {
	return *[]*DestructiveCommand{{command: cmd, valid: true}}[0]
}
`
	u := singleFileUniverse(t, "internal/guard/elided_pointer_fixture.go", src)
	problems := scanSamePackageInvariant(u)
	if len(problems) == 0 {
		t.Fatal("expected the elided-type pointer-element literal forge to be flagged")
	}
}

// TestSamePackageInvariant_NamedSliceTypeElidedLiteral is spec 127
// bead-2 rework round 3's RULING 1/O2c2-4 (MAJOR): a composite literal
// whose OUTER type is a NAMED slice type over DestructiveCommand
// (not the type-identity chain dcTypeNames tracks, but a CONTAINER
// type wrapping it) with an elided-type inner element. This needs no
// go/types — resolving `type X []Y` to its Elt is the same syntactic
// TypeSpec lookup already performed for `type X = Y`/`type X Y`
// identity chains, via the new containerElemTypes lookaside.
func TestSamePackageInvariant_NamedSliceTypeElidedLiteral(t *testing.T) {
	src := `package guard

type g1WrapSliceNamed []DestructiveCommand

func forgeViaNamedSliceTypeElidedLiteral(cmd string) DestructiveCommand {
	return g1WrapSliceNamed{{command: cmd, valid: true}}[0]
}
`
	u := singleFileUniverse(t, "internal/guard/named_slice_type_fixture.go", src)
	problems := scanSamePackageInvariant(u)
	if len(problems) == 0 {
		t.Fatal("expected the named-slice-type elided literal forge to be flagged")
	}
}

// TestSamePackageInvariant_NamedMapTypeElidedLiteral mirrors the named
// slice case above for a NAMED map type's elided VALUE type
// (O2c2-4's second required fixture).
func TestSamePackageInvariant_NamedMapTypeElidedLiteral(t *testing.T) {
	src := `package guard

type g1WrapMapNamed map[int]DestructiveCommand

func forgeViaNamedMapTypeElidedLiteral(cmd string) DestructiveCommand {
	return g1WrapMapNamed{0: {command: cmd, valid: true}}[0]
}
`
	u := singleFileUniverse(t, "internal/guard/named_map_type_fixture.go", src)
	problems := scanSamePackageInvariant(u)
	if len(problems) == 0 {
		t.Fatal("expected the named-map-type elided literal forge to be flagged")
	}
}

// TestSamePackageInvariant_FunctionLocalTypeAliasForge is spec 127
// bead-2 rework round 3's RULING 1/G1-confirm2-1 (BLOCKING): Pass 1's
// fixed-point alias/defined-type resolution now walks the WHOLE file
// AST (ast.Inspect), not just file.file.Decls' top-level entries — a
// `type` declaration inside a FUNCTION BODY is exactly as capable of
// forging a same-package value as a package-level one.
func TestSamePackageInvariant_FunctionLocalTypeAliasForge(t *testing.T) {
	src := `package guard

func localAliasForge(cmd string) DestructiveCommand {
	type localAlias = DestructiveCommand
	return localAlias{command: cmd, valid: true}
}
`
	u := singleFileUniverse(t, "internal/guard/local_alias_fixture.go", src)
	problems := scanSamePackageInvariant(u)
	if len(problems) == 0 {
		t.Fatal("expected the function-local type-alias composite literal forge to be flagged")
	}
}

// TestSamePackageInvariant_FunctionLocalDefinedTypeForge is
// G1-confirm2-1's second required shape: a function-local DEFINED
// type (not an alias), composite-literalled — the same identity-chain
// resolution TestSamePackageInvariant_DefinedTypeCompositeThenConvert
// already pins at package scope, now pinned at function scope.
//
// Spec 127 bead-2 rework round 3's own doc comment above claimed this
// fixture pins the round-3 Pass-1 ast.Inspect fix (the walk-whole-file
// change that lets dcTypeNames see a function-local type declaration).
// Round 4's O2c3-1 (MINOR) found that claim FALSE: the original
// fixture ended in `return DestructiveCommand(d)`, an explicit
// conversion to the always-seeded name "DestructiveCommand" that
// Pass 2's pre-existing, unconditional CallExpr conversion check
// catches regardless of whether `localDT` ever entered dcTypeNames —
// verified by running this exact source against the PRE-round-3 code
// and finding it already flagged (the conversion, not the composite
// literal). The trailing conversion — and the DestructiveCommand
// return type that made a same-package composite literal of the
// sealed type itself an available, independently-flaggable substitute
// — are both dropped so this fixture actually requires the round-3
// fix: the function now returns nothing, so the ONLY expression
// capable of tripping the scan is `localDT{...}` itself, resolvable
// only via the fixed-point walk that lets dcTypeNames see the
// function-local `type localDT DestructiveCommand` declaration.
func TestSamePackageInvariant_FunctionLocalDefinedTypeForge(t *testing.T) {
	src := `package guard

func localDefinedTypeForge(cmd string) {
	type localDT DestructiveCommand
	d := localDT{command: cmd, valid: true}
	_ = d
}
`
	u := singleFileUniverse(t, "internal/guard/local_defined_type_fixture.go", src)
	problems := scanSamePackageInvariant(u)
	if len(problems) == 0 {
		t.Fatal("expected the function-local defined-type composite literal forge to be flagged")
	}
}

// TestSamePackageInvariant_ElidedCompositeControl_EmbeddedNonElided is
// O2-1's own control: a NON-elided inner literal, embedded inside an
// unrelated wrapper composite literal, was already correctly caught by
// round 1's direct check (the inner literal names DestructiveCommand
// explicitly) — this pins that the elision-resolution ADDITION does
// not somehow stop that pre-existing case from firing.
func TestSamePackageInvariant_ElidedCompositeControl_EmbeddedNonElided(t *testing.T) {
	src := `package guard

type g1WrapSlice struct {
	Items []DestructiveCommand
}

func embeddedNonElidedForge(cmd string) DestructiveCommand {
	w := g1WrapSlice{Items: []DestructiveCommand{DestructiveCommand{command: cmd, valid: true}}}
	return w.Items[0]
}
`
	u := singleFileUniverse(t, "internal/guard/embedded_non_elided_fixture.go", src)
	problems := scanSamePackageInvariant(u)
	if len(problems) == 0 {
		t.Fatal("expected the embedded, explicitly-typed inner literal to still be flagged")
	}
}

// TestSamePackageInvariant_ShadowStructConvertedAcross is RULING 2/
// G1-r2-1/O2-r2-1(b): a distinct struct with an identical underlying
// field set, CONVERTED to DestructiveCommand, needs no unsafe and no
// reflect — Go allows a conversion between two struct types whose
// underlying field sequences match. Nothing in the old invariant
// inspected conversion expressions at all.
func TestSamePackageInvariant_ShadowStructConvertedAcross(t *testing.T) {
	src := `package guard

type dcShadow struct {
	command string
	valid   bool
}

func shadowForge(cmd string) DestructiveCommand {
	return DestructiveCommand(dcShadow{command: cmd, valid: true})
}
`
	u := singleFileUniverse(t, "internal/guard/shadow_fixture.go", src)
	problems := scanSamePackageInvariant(u)
	if len(problems) == 0 {
		t.Fatal("expected the shadow-struct conversion forge to be flagged")
	}
}

// TestSamePackageInvariant_DefinedTypeCompositeThenConvert is
// RULING 2/G1-r2-1/O2-r2-1(c): a DEFINED type over DestructiveCommand
// (`type dcDefined DestructiveCommand`), composite-literalled then
// converted back.
func TestSamePackageInvariant_DefinedTypeCompositeThenConvert(t *testing.T) {
	src := `package guard

type dcDefined DestructiveCommand

func definedForge(cmd string) DestructiveCommand {
	d := dcDefined{command: cmd, valid: true}
	return DestructiveCommand(d)
}
`
	u := singleFileUniverse(t, "internal/guard/defined_fixture.go", src)
	problems := scanSamePackageInvariant(u)
	if len(problems) == 0 {
		t.Fatal("expected the defined-type composite-then-convert forge to be flagged")
	}
}

// TestSamePackageInvariant_UnsafeUsage is AC-9(vi): a visible unsafe
// bypass is red wherever the AST scan can see it, even though it is
// recorded as outside this convention's threat model.
func TestSamePackageInvariant_UnsafeUsage(t *testing.T) {
	src := `package guard

import "unsafe"

func unsafeForge(cmd string) DestructiveCommand {
	var d DestructiveCommand
	p := (*string)(unsafe.Pointer(&d))
	*p = cmd
	return d
}
`
	u := singleFileUniverse(t, "internal/guard/unsafe_fixture.go", src)
	problems := scanSamePackageInvariant(u)
	if len(problems) == 0 {
		t.Fatal("expected the unsafe import to be flagged")
	}
}

// samePackageEscapeShapes is the fixture-backed catalog of concrete
// same-package construction shapes scanSamePackageInvariant is proven
// to catch — each name is one of the TestSamePackageInvariant_<name>
// fixtures above. This is the set the scan CATCHES, not the set that
// EXISTS: it is not comprehensive (constructor.go's own doc comment
// says so at length), and a NAMED POINTER TYPE
// (`type dcNamedPtr *DestructiveCommand` used as a slice element with
// an elided literal) is a known, unfixtured escape outside this set —
// verified to compile and forge a live value, filed as bd
// mindspec-erpg, and deliberately not closed here.
//
// Spec 127 bead-2's rework rounds each produced a written escape COUNT
// (in this file, in spec.md, or both) that went stale at least once —
// rounds 4 and 5 both had to fix a number that no longer matched the
// fixtures. TestSamePackageInvariant_FixtureManifestMatches, below,
// replaces that count with a name-for-name reconciliation against the
// TestSamePackageInvariant_* functions actually declared in this
// file, so this list and the fixtures cannot silently diverge again —
// deriving beats writing down a cardinality by hand.
var samePackageEscapeShapes = []string{
	"SamePackageHelperForge",
	"WrapperReturnedForgedValue",
	"TypeAliasCompositeLiteral",
	"TwoLevelAliasChain",
	"ElidedSliceLiteral",
	"ElidedMapLiteral",
	"ElidedStructFieldLiteral",
	"ElidedPointerElementLiteral",
	"NamedSliceTypeElidedLiteral",
	"NamedMapTypeElidedLiteral",
	"FunctionLocalTypeAliasForge",
	"FunctionLocalDefinedTypeForge",
	"ShadowStructConvertedAcross",
	"DefinedTypeCompositeThenConvert",
}

// samePackageEscapeControlFixtures lists the TestSamePackageInvariant_*
// functions in this file that are NOT themselves a distinct escape
// shape, so TestSamePackageInvariant_FixtureManifestMatches can
// exclude them by name (not by count, for the same reason
// samePackageEscapeShapes above is a name list): RealGuardPackageIsClean
// is the negative control run against the real internal/guard package;
// ElidedCompositeControl_EmbeddedNonElided pins that a pre-existing,
// already-caught case still fires after the elision-resolution
// addition, rather than introducing a new shape; UnsafeUsage exercises
// the separate unsafe/reflect.NewAt defense R5(b)'s prose calls out on
// its own, distinct from the enumerated same-package escape-shape
// list.
var samePackageEscapeControlFixtures = []string{
	"RealGuardPackageIsClean",
	"ElidedCompositeControl_EmbeddedNonElided",
	"UnsafeUsage",
}

// TestSamePackageInvariant_FixtureManifestMatches asserts that
// samePackageEscapeShapes plus samePackageEscapeControlFixtures is
// EXACTLY the set of TestSamePackageInvariant_* functions declared in
// this file — not more (a fixture landed without adding its name to
// either list) and not fewer (a listed name with no fixture behind
// it). This is the sentinel spec 127 bead-2 rework round 5's ruling
// asked for in place of a hand-written cardinality: the reconciliation
// runs against this file's OWN current source, so it cannot itself go
// stale the way a written number did five times over this bead's
// rework rounds.
func TestSamePackageInvariant_FixtureManifestMatches(t *testing.T) {
	path := filepath.Join(repoRootDir(t), "internal", "lint", "destructive_guidance_test.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading this test file to reconcile its own fixture manifest: %v", err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, data, 0)
	if err != nil {
		t.Fatalf("parsing this test file to reconcile its own fixture manifest: %v", err)
	}

	const prefix = "TestSamePackageInvariant_"
	found := map[string]bool{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil {
			continue
		}
		name := fn.Name.Name
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		short := strings.TrimPrefix(name, prefix)
		if short == "FixtureManifestMatches" {
			continue
		}
		found[short] = true
	}

	expected := map[string]bool{}
	for _, name := range samePackageEscapeShapes {
		expected[name] = true
	}
	for _, name := range samePackageEscapeControlFixtures {
		expected[name] = true
	}

	for name := range found {
		if !expected[name] {
			t.Errorf("TestSamePackageInvariant_%s exists in this file but is not listed in samePackageEscapeShapes or samePackageEscapeControlFixtures — add it to whichever set actually describes it", name)
		}
	}
	for name := range expected {
		if !found[name] {
			t.Errorf("samePackageEscapeShapes or samePackageEscapeControlFixtures names %q but no TestSamePackageInvariant_%s function exists in this file", name, name)
		}
	}
}

// TestConstructorProvenance_CalleeNameSpoofRejected is AC-9(vi)'s
// callee-name-spoof defense: a DIFFERENT package's function that
// happens to share the name "NewDestructiveCommand" does not qualify
// as constructor-derived merely by name — the scan must resolve the
// callee's actual import path.
func TestConstructorProvenance_CalleeNameSpoofRejected(t *testing.T) {
	src := `package approve

import spoof "github.com/mrmaxsteel/mindspec/internal/lifecycle"

func spoofedRefusal(branch string, outcome guard.DestructionOutcome) string {
	d, err := spoof.NewDestructiveCommand(branch, outcome)
	if err != nil {
		return "spoofed error"
	}
	return d.String()
}
`
	expr, fnNode, file := findSingleReturnAndFunc(t, singleFileUniverse(t, "internal/approve/spoof_fixture.go", src), "internal/approve/spoof_fixture.go")
	// The import alias "spoof" resolves to internal/lifecycle, not
	// internal/guard — the callee-name-spoof defense (spec 127 R5b)
	// must reject it even though the two-value-bind SHAPE (round-2's
	// own fix, G1-C1) is otherwise identical to a real caller's.
	if isConstructorDerived(expr, fnNode, file) {
		t.Fatal("expected the callee-name spoof (internal/lifecycle.NewDestructiveCommand) to be REJECTED as provenance")
	}
}

// TestConstructorProvenance_RealConstructorAccepted is the positive
// half: a genuine, error-handled guard.NewDestructiveCommand(...)
// two-result bind, `.String()`'d after the error check, IS accepted
// as provenance. This fixture is deliberately COMPILE-VALID Go
// against the real two-result signature — the fixture this replaces
// chained `.String()` straight off the constructor call as a single
// expression, which is not valid Go once NewDestructiveCommand
// returns (DestructiveCommand, error): that shape can never occur in
// real, compiling code, so accepting only it left the actual
// achievable shape unrecognized (bead-2 rework round 2, G1-C1,
// BLOCKING — "replace the uncompilable positive fixture with
// compile-valid source").
func TestConstructorProvenance_RealConstructorAccepted(t *testing.T) {
	src := `package approve

import "github.com/mrmaxsteel/mindspec/internal/guard"

func realRefusal(branch string, outcome guard.DestructionOutcome) string {
	d, err := guard.NewDestructiveCommand("git branch -D "+branch, outcome)
	if err != nil {
		return "fallback"
	}
	return d.String()
}
`
	expr, fnNode, file := findSingleReturnAndFunc(t, singleFileUniverse(t, "internal/approve/real_fixture.go", src), "internal/approve/real_fixture.go")
	if !isConstructorDerived(expr, fnNode, file) {
		t.Fatal("expected the real, error-handled guard.NewDestructiveCommand(...) bind's .String() call to be ACCEPTED as provenance")
	}
}

// TestConstructorProvenance_IgnoredErrorRejected is G1-C1's first
// required negative fixture: `d, _ := NewDestructiveCommand(...)`
// discards the only signal that the happy path was actually reached
// — no later check can exist for a blank identifier, so this can
// never qualify.
func TestConstructorProvenance_IgnoredErrorRejected(t *testing.T) {
	src := `package approve

import "github.com/mrmaxsteel/mindspec/internal/guard"

func ignoredErrorRefusal(branch string, outcome guard.DestructionOutcome) string {
	d, _ := guard.NewDestructiveCommand("git branch -D "+branch, outcome)
	return d.String()
}
`
	expr, fnNode, file := findSingleReturnAndFunc(t, singleFileUniverse(t, "internal/approve/ignored_err_fixture.go", src), "internal/approve/ignored_err_fixture.go")
	if isConstructorDerived(expr, fnNode, file) {
		t.Fatal("expected an ignored-error (`_`) constructor bind to be REJECTED as provenance")
	}
}

// TestConstructorProvenance_ErrorUnhandledRejected is G1-C1's second
// required negative fixture: the error is bound to a real identifier
// but never checked before `.String()` is reached — the caller never
// actually confirmed the happy path, same risk as the ignored case,
// caught structurally rather than by name.
func TestConstructorProvenance_ErrorUnhandledRejected(t *testing.T) {
	src := `package approve

import "github.com/mrmaxsteel/mindspec/internal/guard"

func unhandledErrorRefusal(branch string, outcome guard.DestructionOutcome) string {
	d, err := guard.NewDestructiveCommand("git branch -D "+branch, outcome)
	_ = err
	return d.String()
}
`
	expr, fnNode, file := findSingleReturnAndFunc(t, singleFileUniverse(t, "internal/approve/unhandled_err_fixture.go", src), "internal/approve/unhandled_err_fixture.go")
	if isConstructorDerived(expr, fnNode, file) {
		t.Fatal("expected an error-unhandled (bound, never checked) constructor bind to be REJECTED as provenance")
	}
}

// TestConstructorProvenance_OverwrittenBindRejected is G1-C1's third
// required negative fixture: `d` is bound TWICE from two different
// NewDestructiveCommand calls — which one a later `.String()` traces
// to is ambiguous, so neither binding qualifies (the same single-
// assignment discipline localBinds, above, already applies to a
// plain fold).
func TestConstructorProvenance_OverwrittenBindRejected(t *testing.T) {
	src := `package approve

import "github.com/mrmaxsteel/mindspec/internal/guard"

func overwrittenBindRefusal(branchA, branchB string, outcome guard.DestructionOutcome) string {
	d, err := guard.NewDestructiveCommand("git branch -D "+branchA, outcome)
	if err != nil {
		return "fallback"
	}
	d, err = guard.NewDestructiveCommand("git branch -D "+branchB, outcome)
	if err != nil {
		return "fallback"
	}
	return d.String()
}
`
	expr, fnNode, file := findSingleReturnAndFunc(t, singleFileUniverse(t, "internal/approve/overwritten_bind_fixture.go", src), "internal/approve/overwritten_bind_fixture.go")
	if isConstructorDerived(expr, fnNode, file) {
		t.Fatal("expected a twice-bound (overwritten) constructor identifier to be REJECTED as provenance")
	}
}

// TestConstructorProvenance_PlainReassignmentRejected is spec 127
// bead-2 rework round 3's required fixture (RULING 2/G1-confirm2-2,
// BLOCKING): after a correctly error-checked bind, a PLAIN
// reassignment of the bound identifier (not a second two-result
// NewDestructiveCommand call) must still void provenance — the value
// `.String()` is eventually called on may no longer be the
// constructor's output at all.
func TestConstructorProvenance_PlainReassignmentRejected(t *testing.T) {
	src := `package approve

import "github.com/mrmaxsteel/mindspec/internal/guard"

func reassignedBindRefusal(branch string, outcome guard.DestructionOutcome) string {
	d, err := guard.NewDestructiveCommand("git branch -D "+branch, outcome)
	if err != nil {
		return "fallback"
	}
	d = guard.DestructiveCommand{}
	return d.String()
}
`
	expr, fnNode, file := findSingleReturnAndFunc(t, singleFileUniverse(t, "internal/approve/reassigned_bind_fixture.go", src), "internal/approve/reassigned_bind_fixture.go")
	if isConstructorDerived(expr, fnNode, file) {
		t.Fatal("expected a plainly-reassigned constructor identifier to be REJECTED as provenance")
	}
}

// TestConstructorProvenance_DeferredCheckRejected is spec 127 bead-2
// rework round 3's required fixture (RULING 2/G1-confirm2-3,
// BLOCKING): an error check that only ever runs inside a deferred
// closure never gates the synchronous `.String()` call in the same
// return statement — errCheckedAfter now never descends into an
// *ast.DeferStmt, so this check is never found and the bind cannot
// qualify as provenance.
func TestConstructorProvenance_DeferredCheckRejected(t *testing.T) {
	src := `package approve

import "github.com/mrmaxsteel/mindspec/internal/guard"

func deferredCheckRefusal(branch string, outcome guard.DestructionOutcome) string {
	d, err := guard.NewDestructiveCommand("git branch -D "+branch, outcome)
	defer func() {
		if err != nil {
			panic(err)
		}
	}()
	return d.String()
}
`
	expr, fnNode, file := findSingleReturnAndFunc(t, singleFileUniverse(t, "internal/approve/deferred_check_fixture.go", src), "internal/approve/deferred_check_fixture.go")
	if isConstructorDerived(expr, fnNode, file) {
		t.Fatal("expected a deferred-only error check to be REJECTED as provenance (it does not gate the synchronous .String() call)")
	}
}

// findSingleReturnAndFunc locates u's registered file at rel, the
// `return <expr>.String()`-shaped statement's expression (skipping
// any other single-result return, e.g. a fixture's own `return
// "fallback"` error leg), and that expression's enclosing function
// node — shared setup for the constructor-provenance fixtures above.
func findSingleReturnAndFunc(t *testing.T, u *rUniverse, rel string) (ast.Expr, ast.Node, *rFile) {
	t.Helper()
	var file *rFile
	for _, f := range u.files {
		if f.rel == rel {
			file = f
		}
	}
	if file == nil {
		t.Fatal("fixture file not registered")
	}
	var expr ast.Expr
	ast.Inspect(file.file, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			return true
		}
		call, ok := ret.Results[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "String" {
			return true
		}
		expr = ret.Results[0]
		return true
	})
	if expr == nil {
		t.Fatal("fixture setup error: no `.String()`-shaped return expression found")
	}
	return expr, enclosingFuncNode(file, expr.Pos()), file
}

// ---------------------------------------------------------------------
// Bootstrap discipline (spec 127 G-r5-4/H-r6-5): the committed seed
// manifest and its THREE landed hermetic fixtures — registry identity
// (β), content regeneration (γ), and manifest-vs-Background
// reconciliation (α) — applied uniformly to all three seeded artifacts
// ((α) applies specifically to the known_sites_exemption_list section;
// see TestBootstrapManifest_BackgroundReconciliation, below, for its
// current scope). (α) was cited as existing before either
// implementing it, in bead-2 rework round 1 (spec 127 bead-2 rework,
// O2-r2-4/O3-r2-3) — that round deleted the false claim rather than
// implementing it. Round 2 (O3-3) implemented (α)'s FORWARD direction
// (every site spec.md Background reviewed is still present). Round 3
// (RULING 4/O3-3, this rework round) added the REVERSE direction
// (every manifest entry is accounted for by something Background
// reviewed, or a named widening) after proving the forward direction
// alone let a fabricated, unreviewed entry pass every landed fixture
// undetected. This comment block itself went stale for a full round
// between round 2 landing (α) and this correction — a standing lesson
// for whoever next widens this mechanism: update THIS header, not only
// the function doc comments below it.
// ---------------------------------------------------------------------

// manifestEntry is one parsed line of destructive_seed_manifest.txt.
type manifestEntry struct {
	fields []string
}

// parseSeedManifest reads the committed manifest into its three
// named sections.
func parseSeedManifest(t *testing.T) map[string][]manifestEntry {
	t.Helper()
	path := filepath.Join(repoRootDir(t), "internal", "lint", "testdata", "destructive_seed_manifest.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading seed manifest: %v", err)
	}
	sections := map[string][]manifestEntry{}
	var current string
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			current = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			continue
		}
		if current == "" {
			t.Fatalf("manifest line outside any section: %q", line)
		}
		sections[current] = append(sections[current], manifestEntry{fields: strings.Split(line, "\t")})
	}
	return sections
}

// TestBootstrapManifest_AllowlistRegistryIdentity is fixture (β) for
// the destructive-guidance allowlist: registry membership equals the
// manifest exactly. At bead 2's own base NOTHING had been converted yet
// (bead 2 is the seeding bead), so the comparison held with zero
// bead-exit records. Bead 5 (spec 127 R3c) is the first to convert
// seeded sites — internal/approve/plan.go's beadCreateFailure and
// checkExistingBeadsSafety, both now constructor-routed — and removed
// BOTH the registries.go entries and this manifest's two corresponding
// [destructive_guidance_allowlist] lines in that SAME commit (the exit
// note at the top of destructive_seed_manifest.txt), so the two-way
// comparison below never has to reason about a partially-updated pair:
// a later bead converting a further seeded site follows the identical
// discipline.
func TestBootstrapManifest_AllowlistRegistryIdentity(t *testing.T) {
	sections := parseSeedManifest(t)
	manifestKeys := map[string]bool{}
	for _, e := range sections["destructive_guidance_allowlist"] {
		if len(e.fields) != 4 {
			t.Fatalf("malformed allowlist manifest line: %v", e.fields)
		}
		manifestKeys[e.fields[0]+"\t"+e.fields[1]+"\t"+e.fields[2]] = true
	}
	registryKeys := map[string]bool{}
	for _, e := range guard.DestructiveGuidanceAllowlist {
		registryKeys[e.File+"\t"+e.Func+"\t"+e.Detail] = true
	}
	for k := range manifestKeys {
		if !registryKeys[k] {
			t.Errorf("manifest entry %q has no corresponding registries.go entry (β: registry ≡ manifest − named bead-exit records — no exits are recorded yet)", k)
		}
	}
	for k := range registryKeys {
		if !manifestKeys[k] {
			t.Errorf("registries.go entry %q is not in the committed seed manifest — additions are red", k)
		}
	}
}

// TestBootstrapManifest_OpaqueRegistryIdentity mirrors the above for
// the opaque-operand registry.
func TestBootstrapManifest_OpaqueRegistryIdentity(t *testing.T) {
	sections := parseSeedManifest(t)
	manifestKeys := map[string]bool{}
	for _, e := range sections["opaque_operand_registry"] {
		if len(e.fields) != 3 {
			t.Fatalf("malformed opaque-operand manifest line: %v", e.fields)
		}
		manifestKeys[e.fields[0]+"\t"+e.fields[1]+"\t"+e.fields[2]] = true
	}
	registryKeys := map[string]bool{}
	for _, e := range guard.OpaqueOperandRegistry {
		registryKeys[e.File+"\t"+e.Func+"\t"+e.Detail] = true
	}
	for k := range manifestKeys {
		if !registryKeys[k] {
			t.Errorf("manifest entry %q has no corresponding registries.go entry", k)
		}
	}
	for k := range registryKeys {
		if !manifestKeys[k] {
			t.Errorf("registries.go entry %q is not in the committed seed manifest — additions are red", k)
		}
	}
}

// TestBootstrapManifest_ExemptionListIdentity mirrors the above for
// the known-sites exemption list — at content grain (surface + exact
// quoted text + count), per H-r6-5.
func TestBootstrapManifest_ExemptionListIdentity(t *testing.T) {
	sections := parseSeedManifest(t)
	manifestKeys := map[string]bool{}
	for _, e := range sections["known_sites_exemption_list"] {
		if len(e.fields) != 4 {
			t.Fatalf("malformed exemption manifest line: %v", e.fields)
		}
		manifestKeys[e.fields[0]+"\t"+e.fields[1]+"\t"+e.fields[2]] = true
	}
	registryKeys := map[string]bool{}
	for _, e := range guard.KnownSitesExemptionList {
		registryKeys[e.Surface+"\t"+e.Text+"\t"+itoa(e.Count)] = true
	}
	for k := range manifestKeys {
		if !registryKeys[k] {
			t.Errorf("manifest entry %q has no corresponding registries.go entry", k)
		}
	}
	for k := range registryKeys {
		if !manifestKeys[k] {
			t.Errorf("registries.go entry %q is not in the committed seed manifest — additions are red", k)
		}
	}
}

// backgroundExpectedExemption is one quoted fragment fixture (α)
// (below) checks against the manifest — transcribed BY HAND directly
// from spec.md's Background section ("The live class, inventoried"),
// never from registries.go or the manifest itself: the whole point of
// this fixture is to catch a site a LATER bead could sneak into the
// seed that Background, reviewed BEFORE any bead existed, never
// named. quoted is a SUBSTRING match against the manifest's own
// (longer) recorded line — Background's own prose quotes a short,
// representative fragment of each site, not the full line.
type backgroundExpectedExemption struct {
	surface string
	quoted  string
}

// backgroundExpectedExemptions is spec.md Background's own "eight
// sites" enumeration (quoted verbatim there: "the ms-bead-cycle
// prohibition ... (plugin SKILL.md:116, tracked :94), the ms-bead-cycle
// partial-failure bare-prose residual ... (plugin :120, tracked :98),
// the ms-impl-approve descriptive residual ... (tracked SKILL.md:32,
// canonical skill-map literal claude.go:892), the CLAUDE.md
// managed-block guardrails line ... (const claudeMDManagedBlock,
// claude.go:947), and the AGENTS.md managed-block prohibition ...
// (const agentsMDBlockTemplate, internal/setup/codex.go:351)").
var backgroundExpectedExemptions = []backgroundExpectedExemption{
	{"plugins/mindspec/skills/ms-bead-cycle/SKILL.md", "Never merge a bead branch with raw `git merge bead/<id>`"},
	{".claude/skills/ms-bead-cycle/SKILL.md", "Never merge a bead branch with raw `git merge bead/<id>`"},
	{"plugins/mindspec/skills/ms-bead-cycle/SKILL.md", "stopped between bd-close and the actual git merge"},
	{".claude/skills/ms-bead-cycle/SKILL.md", "stopped between bd-close and the actual git merge"},
	{".claude/skills/ms-impl-approve/SKILL.md", "a bead branch already raw-`git merge`d"},
	{"ms-impl-approve", "bead branch already raw-`git merge`d"},
	{"claudeMDManagedBlock", "never raw `git merge bead/<id>`"},
	{"agentsMDBlockTemplate", "Never merge a bead branch with raw `git merge bead/<id>`"},
}

// backgroundJustifiedWideningExemptions is the ONE named, recorded
// widening backgroundExpectedExemptions itself does not cover — the
// "ms-bead-cycle" canonical skill-map literal's own two matches
// (registries.go's KnownSitesExemptionList doc comment, RULING 7):
// spec.md Background's "eight sites" tally names the canonical literal
// for ms-impl-approve's residual but not, by the same reasoning, for
// ms-bead-cycle's two entries — a widening the mechanism's own design
// justifies (every lifecycle-gate skill's canonical literal is swept,
// not only the ones Background happened to name), not unreviewed
// drift. Listed here, by name, so the reverse-direction reconciliation
// below (fixture (α), spec 127 bead-2 rework round 3, RULING 4) can
// account for it WITHOUT silently accepting every unnamed manifest
// entry — anything NOT in this list and NOT in
// backgroundExpectedExemptions is red.
//
// This list is ITSELF an escape hatch a future bead's diff could
// widen in the same coordinated change as a fabricated manifest entry
// and registries.go site — spec 127 bead-2 rework round 4's RULING 3
// (O3-confirm3-NEW-1/G1-confirm3-3, BLOCKING) proved exactly that by
// simulating a full attacker bead (a fabricated site inserted into a
// REAL on-disk skill file, plus matching registries.go/manifest/this-
// list rows): every TestBootstrapManifest_* fixture passed. No in-repo
// mechanism can stop an author with commit access from appending to an
// in-repo list — pinning this list's count (below) raises the cost of
// a silent addition (a reviewer sees the diff to the sentinel) but does
// NOT close the class; a coordinated, reviewed-looking diff that bumps
// both the list and its sentinel together still lands. See
// TestBootstrapManifest_BackgroundReconciliation's own doc comment for
// what fixture (α) can and cannot claim as a result.
var backgroundJustifiedWideningExemptions = []backgroundExpectedExemption{
	{"ms-bead-cycle", "Never merge a bead branch with raw `git merge bead/<id>`"},
	{"ms-bead-cycle", "stopped between bd-close and the actual git merge"},
}

// TestBackgroundJustifiedWideningExemptions_CountSentinel pins this
// list's own length (spec 127 bead-2 rework round 4, RULING 3): not a
// closure over who may edit it — nothing in-repo can be — but a silent
// widening now costs a reviewed sentinel-diff (the same discipline
// registries_test.go's TestKnownSitesExemptionList_CountSentinel
// already applies one layer up), rather than being invisible.
func TestBackgroundJustifiedWideningExemptions_CountSentinel(t *testing.T) {
	if got, want := len(backgroundJustifiedWideningExemptions), 2; got != want {
		t.Errorf("len(backgroundJustifiedWideningExemptions) = %d, want %d", got, want)
	}
}

// backgroundExpectedBypassBlockFamilies is spec.md Background's
// bypass-block bullet for .claude/agents/spec-orchestrator.md: "raw
// git merge --no-ff bead/<id>, git worktree remove ... --force, git
// branch -D bead/<id>" plus "git stash push" (the ADJACENT safe
// command in the same code block — `git stash drop`, the floor-
// matching one two lines later, is what actually lands in the
// exemption list; stash push itself matches no family). Named at
// COMMAND-FAMILY grain per spec.md's own scoping (J-r7-1): this bullet
// reconciled at that coarser grain, not exact quoted-string grain like
// the eight above, ONLY while the four seed-only orchestrator-block
// entries were still live in the manifest. BEAD 7 EXITS all four
// (R5(e) deletes the whole bypass block outright), so this var is now
// HISTORICAL — retained so a reader can see what the pre-bead-7 seed
// stood for — and TestBootstrapManifest_BackgroundReconciliation below
// asserts the orchestrator surface's manifest entries are exactly
// EMPTY post-exit, not a family-set match against this var. The four
// seed-only entries are covered at EXACT grain by fixture (β)'s
// bead-7 exit records (TestBootstrapManifest_AllowlistRegistryIdentity's
// sibling for the exemption list, TestBootstrapManifest_ExemptionListIdentity).
var backgroundExpectedBypassBlockFamilies = []guard.DestructiveFamily{
	guard.FamilyGitBranchDeleteForce,
	guard.FamilyGitMerge,
	guard.FamilyGitStashDropClear,
	guard.FamilyGitWorktreeRemoveForce,
}

// TestBootstrapManifest_BackgroundReconciliation is fixture (α)
// (spec 127 R5(c)'s G-r5-4 bootstrap discipline; landed in bead-2's
// SECOND rework round, O3-3 — round 1 deleted the false claim that (α)
// existed but did not implement it, which O3 ruled was NOT the same as
// delivering AC-9(ii)'s reconciliation requirement).
//
// Round 2 checked only the FORWARD direction — everything Background
// named, reviewed at spec time before any bead existed, is still
// present — NOT exact equality against the manifest's full
// known_sites_exemption_list section. Background's own "eight sites"
// tally does not separately enumerate the "ms-bead-cycle" canonical
// skill-map literal's own two matches (it names the canonical literal
// for ms-impl-approve's residual but not, by the same reasoning, for
// ms-bead-cycle's two entries) — registries.go's own KnownSitesExemptionList
// doc comment (RULING 7) already records this as a widening the
// mechanism's own design justifies (every lifecycle-gate skill's
// canonical literal is swept, not only the ones Background happened to
// name), not as unreviewed drift.
//
// Spec 127 bead-2 rework round 3's O3-3 (RULING 4, BLOCKING) proved the
// forward direction ALONE insufficient: a fabricated, unreviewed
// exemption entry — added consistently to BOTH registries.go and this
// manifest, so fixture (β)'s registry-vs-manifest identity check stayed
// green too — passed every landed bootstrap fixture undetected,
// directly falsifying this manifest's own header claim ("a bead
// introducing an unreviewed site cannot enter this list unnoticed") and
// AC-9(ii)'s stated purpose for (α). This now ALSO checks the REVERSE
// direction below: every manifest entry (outside the orchestrator
// surface, reconciled both directions already at family grain) must be
// accounted for by something Background actually named
// (backgroundExpectedExemptions) or by the recorded widening
// (backgroundJustifiedWideningExemptions) — anything else is red.
//
// Neither direction is exact equality against the manifest's full
// known_sites_exemption_list section at PER-ENTRY grain — that would
// either fail on the already-explained, justified widening or require
// silently narrowing this fixture's own claim to match it. This states
// the two SUBSET claims it can actually back: nothing Background
// reviewed has disappeared, and nothing outside what Background
// reviewed (or the named widening) has appeared.
//
// What this fixture's purpose IS, narrowed (spec 127 bead-2 rework
// round 4, RULING 3 — O3-confirm3-NEW-1/G1-confirm3-3, BLOCKING):
// spec.md R5(c) originally stated (α)'s purpose as "so a bead-
// introduced site cannot enter the seed" — an unachievable claim, not
// merely an unmet one. Round 4's attacker-bead simulation (a fabricated
// site inserted into a REAL on-disk skill file, with matching rows
// added consistently to registries.go, this manifest, AND
// backgroundJustifiedWideningExemptions in the same diff) passed every
// TestBootstrapManifest_* fixture. No in-repo mechanism can stop an
// author with commit access from appending to an in-repo list — that
// is not a gap in this fixture, it is a property of what a repo-local
// reconciliation can ever prove. What (α) actually does: reconcile the
// manifest against a NAMED expectation set (backgroundExpectedExemptions
// plus the one recorded widening), which catches ACCIDENTAL drift — a
// site silently dropped, or an unnamed site appearing without its
// widening ever being recorded. A DELIBERATE, coordinated insertion
// that edits the expectation set and the manifest together in one diff
// is review-caught, not machine-prevented — exactly the same class of
// limit R5(b)'s same-package AST invariant now states about itself.
func TestBootstrapManifest_BackgroundReconciliation(t *testing.T) {
	sections := parseSeedManifest(t)
	entries := sections["known_sites_exemption_list"]
	const orchestratorSurface = ".claude/agents/spec-orchestrator.md"

	bySurfaceText := map[string][]string{}
	familiesBySurface := map[string]map[guard.DestructiveFamily]bool{}
	for _, e := range entries {
		if len(e.fields) != 4 {
			t.Fatalf("malformed exemption manifest line: %v", e.fields)
		}
		surface, text, family := e.fields[0], e.fields[1], guard.DestructiveFamily(e.fields[3])
		bySurfaceText[surface] = append(bySurfaceText[surface], text)
		if familiesBySurface[surface] == nil {
			familiesBySurface[surface] = map[guard.DestructiveFamily]bool{}
		}
		familiesBySurface[surface][family] = true
	}

	for _, exp := range backgroundExpectedExemptions {
		found := false
		for _, text := range bySurfaceText[exp.surface] {
			if strings.Contains(text, exp.quoted) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("fixture (α), forward direction: spec.md Background names %q at surface %q, reviewed at spec time before any bead existed — no manifest line at that surface contains it (a site Background never reviewed may have entered the seed, or one it DID review may have been dropped)", exp.quoted, exp.surface)
		}
	}

	// Reverse direction (RULING 4, BLOCKING): every non-orchestrator
	// manifest entry must be accounted for by something Background
	// actually reviewed, or by the one named widening — not merely
	// present in both registries.go and this manifest, which O3's
	// fabricated-entry probe proved insufficient.
	accountedFor := func(surface, text string) bool {
		for _, exp := range backgroundExpectedExemptions {
			if exp.surface == surface && strings.Contains(text, exp.quoted) {
				return true
			}
		}
		for _, exp := range backgroundJustifiedWideningExemptions {
			if exp.surface == surface && strings.Contains(text, exp.quoted) {
				return true
			}
		}
		return false
	}
	for _, e := range entries {
		surface, text := e.fields[0], e.fields[1]
		if surface == orchestratorSurface {
			continue // reconciled both directions, at family grain, below
		}
		if !accountedFor(surface, text) {
			t.Errorf("fixture (α), reverse direction: manifest entry at surface %q (text %q) is accounted for by NEITHER spec.md Background's own enumeration NOR the recorded widening exceptions — an unreviewed site may have entered the seed unnoticed", surface, text)
		}
	}

	// Bead 7 exits all four seed-only orchestrator-block entries in the
	// SAME commit that deletes spec-orchestrator.md's raw bypass block
	// (R5(e)) — so the manifest's orchestrator-surface family set is
	// now EMPTY, not a match against backgroundExpectedBypassBlockFamilies
	// (that var is retained as a historical record of what the pre-bead-7
	// seed stood for; see its own doc comment). A leftover entry here —
	// the exact leftover/second-entry shape AC-9's burn-down fixtures
	// probe one layer up in registries_test.go — REDs this assertion.
	if got := familiesBySurface[orchestratorSurface]; len(got) != 0 {
		t.Errorf("fixture (α), family grain: manifest surface %q still carries families %v after bead 7's exit — the seed-only orchestrator-block entries must be fully removed, not merely reduced", orchestratorSurface, got)
	}
}

// TestBootstrapManifest_ContentRegeneration is fixture (γ), applied
// uniformly to all three artifacts: the classifier, run over each
// manifest entry's quoted matched text, still matches that entry's
// recorded family — a hollow entry or classifier drift is red. The
// allowlist/exemption sections carry a family column; the opaque-
// operand section does not (it records the ABSENCE of a provable
// literal, not a match) and is skipped here by design.
func TestBootstrapManifest_ContentRegeneration(t *testing.T) {
	sections := parseSeedManifest(t)
	check := func(section string, textIdx, familyIdx int) {
		for _, e := range sections[section] {
			if len(e.fields) <= familyIdx {
				continue
			}
			text := e.fields[textIdx]
			wantFamily := guard.DestructiveFamily(e.fields[familyIdx])
			matches := guard.FindFloorMatches(text)
			found := false
			for _, m := range matches {
				if m.Family == wantFamily {
					found = true
				}
			}
			if !found {
				t.Errorf("%s entry %q: classifier no longer matches recorded family %s (hollow entry or classifier drift): got %+v", section, text, wantFamily, matches)
			}
		}
	}
	check("destructive_guidance_allowlist", 2, 3)
	check("known_sites_exemption_list", 1, 3)
}
