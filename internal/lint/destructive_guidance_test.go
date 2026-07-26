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
//   - the GUIDANCE leg: the three shipped-guidance globs plus the
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
	memo  map[string]map[string]bool // in-progress guard against cyclic const refs
	sites map[string]exprSite        // pkgDir.name -> AST site, PER-FOLDER (never shared across universes)
}

func newPkgConstFolder(u *rUniverse) *pkgConstFolder {
	f := &pkgConstFolder{u: u, byPkg: map[string]map[string]string{}, memo: map[string]map[string]bool{}, sites: map[string]exprSite{}}
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
// concatenation chains with no such cross-reference).
func (f *pkgConstFolder) Get(pkgDir, name string) (string, bool) {
	key := pkgDir + "." + name
	if v, ok := f.byPkg[pkgDir][name]; ok {
		return v, true
	}
	site, ok := f.sites[key]
	if !ok {
		return "", false
	}
	if f.memo[pkgDir][name] {
		return "", false // cycle guard
	}
	f.memo[pkgDir][name] = true
	v, ok, _ := foldExpr(site.expr, site.file, f, nil, 0)
	if ok {
		f.byPkg[pkgDir][name] = v
	}
	return v, ok
}

// localBinds computes function-scoped single-assignment string binds:
// every `:=`/`var` name assigned EXACTLY ONCE within fn's body (top
// level and nested blocks — ast.Inspect walks the whole subtree but
// does not cross into nested FuncLit bodies, which get their own
// scope) to a foldable expression. A name touched more than once, or
// once with a non-foldable value, is excluded — "single-assignment"
// per spec 127 R5(c).
func localBinds(fn ast.Node, file *rFile, f *pkgConstFolder) map[string]string {
	binds := map[string]string{}
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
		if v, ok, _ := foldExpr(val, file, f, nil, 0); ok {
			binds[name] = v
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
func foldExpr(expr ast.Expr, file *rFile, f *pkgConstFolder, binds map[string]string, depth int) (skel string, hasLiteral bool, fullyLiteral bool) {
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
			if v, ok := binds[e.Name]; ok {
				return v, true, true
			}
		}
		if v, ok := f.Get(pkgDir, e.Name); ok {
			return v, true, true
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
		if v, ok := f.Get(otherPkgDir, e.Sel.Name); ok {
			return v, true, true
		}
		return "", false, false
	case *ast.CallExpr:
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			// fmt.Sprintf: provable via its literal TEMPLATE only
			// (spec 127 R5c's own listed shape) — substituted args
			// are deliberately never inspected.
			if xid, ok2 := sel.X.(*ast.Ident); ok2 && xid.Name == "fmt" && sel.Sel.Name == "Sprintf" && len(e.Args) > 0 {
				return foldExpr(e.Args[0], file, f, binds, depth+1)
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

// isConstructorDerived reports whether expr is (directly, or through
// one level of parens/selector) a `.String()` call on a
// guard.NewDestructiveCommand(...)/NewDestructiveCommand(...) result —
// dataflow to the approved constructor's call site, never merely the
// static type (spec 127 D-r4-2): a wrapper that returns a FORGED value
// under the same name, or a same-named function elsewhere, does not
// match this AST shape at all (it is a different, unresolved call),
// so it does not qualify.
func isConstructorDerived(expr ast.Expr, file *rFile) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "String" {
		return false
	}
	inner, ok := sel.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	// The callee-name-spoof defense (spec 127 R5b): a bare
	// `NewDestructiveCommand(...)` only qualifies from WITHIN
	// internal/guard itself; a qualified `pkg.NewDestructiveCommand(...)`
	// only qualifies when `pkg`'s IMPORT PATH actually resolves to
	// internal/guard — a same-named function in some OTHER package
	// does not match by name alone.
	switch fn := inner.Fun.(type) {
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
		binds := map[string]map[string]string{} // enclosing func key -> binds, computed lazily
		bindsFor := func(pos token.Pos) map[string]string {
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
								provenance:      isConstructorDerived(arg, file),
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
							provenance:      isConstructorDerived(arg, file),
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
							provenance:      isConstructorDerived(ret.Results[0], file),
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
						provenance:   isConstructorDerived(kv.Value, file),
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
// Guidance leg: the three shipped-guidance globs plus the canonical
// setup-guidance builders (spec 127 R5c/H-r6-6).
// ---------------------------------------------------------------------

// guidanceGlobDirs is the closed set of tracked-file guidance roots
// (spec 127 R5c: `.claude/agents/**`, `.claude/skills/**`,
// `plugins/*/skills/**`). Walked live at test time — a newly added
// file under any of these is automatically included (the anti-drift
// leg is the live filesystem walk itself, not a hardcoded list).
func guidanceGlobDirs(root string) []string {
	var dirs []string
	for _, d := range []string{filepath.Join(root, ".claude", "agents"), filepath.Join(root, ".claude", "skills")} {
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
// package). This invariant closes that gap: a populated composite
// literal, an unexported-field write, or a type CONVERSION producing
// the opaque type is red anywhere in a "package guard" file OUTSIDE
// NewDestructiveCommand's own implementation. Package identity is by
// PACKAGE CLAUSE name ("package guard"), not by repo path, so a
// fixture universe need not live at a real "internal/guard" path to
// be recognized.
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
	// "package guard" files.
	dcTypeNames := map[string]bool{"DestructiveCommand": true}
	for _, file := range guardFiles {
		for _, decl := range file.file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if id, ok := ast.Unparen(ts.Type).(*ast.Ident); ok && id.Name == "DestructiveCommand" {
					dcTypeNames[ts.Name.Name] = true
				}
			}
		}
	}

	// Pass 2: flag every composite literal or conversion naming a
	// resolved type-identity member, and every unexported-field write
	// or unsafe/reflect.NewAt usage, outside NewDestructiveCommand's
	// own implementation.
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

// TestConstructorProvenance_CalleeNameSpoofRejected is AC-9(vi)'s
// callee-name-spoof defense: a DIFFERENT package's function that
// happens to share the name "NewDestructiveCommand" does not qualify
// as constructor-derived merely by name — the scan must resolve the
// callee's actual import path.
func TestConstructorProvenance_CalleeNameSpoofRejected(t *testing.T) {
	src := `package approve

import spoof "github.com/mrmaxsteel/mindspec/internal/lifecycle"

func spoofedRefusal(branch string) string {
	return spoof.NewDestructiveCommand(branch).String()
}
`
	u := singleFileUniverse(t, "internal/approve/spoof_fixture.go", src)
	// Locate the CallExpr inside spoofedRefusal's return statement and
	// confirm isConstructorDerived rejects it: the import alias "spoof"
	// resolves to internal/lifecycle, not internal/guard.
	var file *rFile
	for _, f := range u.files {
		if f.rel == "internal/approve/spoof_fixture.go" {
			file = f
		}
	}
	if file == nil {
		t.Fatal("fixture file not registered")
	}
	var expr ast.Expr
	ast.Inspect(file.file, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if ok && len(ret.Results) == 1 {
			expr = ret.Results[0]
		}
		return true
	})
	if expr == nil {
		t.Fatal("fixture setup error: no return expression found")
	}
	if isConstructorDerived(expr, file) {
		t.Fatal("expected the callee-name spoof (internal/lifecycle.NewDestructiveCommand) to be REJECTED as provenance")
	}
}

// TestConstructorProvenance_RealConstructorAccepted is the positive
// half: a genuine guard.NewDestructiveCommand(...).String() call IS
// accepted as provenance.
func TestConstructorProvenance_RealConstructorAccepted(t *testing.T) {
	src := `package approve

import "github.com/mrmaxsteel/mindspec/internal/guard"

func realRefusal(branch string, outcome guard.DestructionOutcome) string {
	return guard.NewDestructiveCommand("git branch -D "+branch, outcome).String()
}
`
	u := singleFileUniverse(t, "internal/approve/real_fixture.go", src)
	var file *rFile
	for _, f := range u.files {
		if f.rel == "internal/approve/real_fixture.go" {
			file = f
		}
	}
	if file == nil {
		t.Fatal("fixture file not registered")
	}
	var expr ast.Expr
	ast.Inspect(file.file, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if ok && len(ret.Results) == 1 {
			expr = ret.Results[0]
		}
		return true
	})
	if expr == nil {
		t.Fatal("fixture setup error: no return expression found")
	}
	if !isConstructorDerived(expr, file) {
		t.Fatal("expected the real guard.NewDestructiveCommand(...).String() call to be ACCEPTED as provenance")
	}
}

// ---------------------------------------------------------------------
// Bootstrap discipline (spec 127 G-r5-4/H-r6-5): the committed seed
// manifest and its TWO landed hermetic fixtures — registry identity
// (β) and content regeneration (γ) — applied uniformly to all three
// seeded artifacts. Fixture (α) — manifest-vs-independent-Background-
// derived-enumeration reconciliation — is NOT landed (spec 127 bead-2
// rework, O2-r2-4/O3-r2-3): it was cited as existing in two places
// (this manifest's own header and registries.go's doc comments)
// before either implemented it; both citations are now corrected to
// state plainly that (α) does not exist, rather than leaving a named
// fixture cited and absent. (β)+(γ) together still catch a hollow or
// drifted entry; what only (α) would additionally catch — a
// bead-introduced site entering the seed at the WRONG base — is
// covered today by the manifest header's own reproducible
// `git diff --stat` verification instead of a standing fixture.
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
// the destructive-guidance allowlist: at this base, NOTHING has been
// converted yet (bead 2 is the seeding bead), so registry membership
// equals the manifest exactly — no bead-exit records exist to
// subtract. A later bead that converts a site removes BOTH the
// registries.go entry and this manifest's corresponding line in the
// SAME commit, so the two-way comparison never has to reason about a
// partially-updated pair.
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
