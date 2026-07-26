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
//     `Message:`-keyed composite-literal fields, repo-wide, fail-closed
//     on any operand the fold rules below cannot prove.
//   - the GUIDANCE leg: the three shipped-guidance globs plus the
//     evaluated canonical setup-guidance builders, checked against the
//     known-sites exemption list.
//
// Both legs share one fact: "is this text dangerous?" is not decidable
// from the text (spec 127 Non-Goals) — this scan claims exactly what
// guard.FindFloorMatches's reviewed finite floor can prove, nothing
// more. A destructive family or program outside that floor is caught
// by review under the ADR-0035 amendment's in-diff extension
// obligation, not by this scan.
package lint

import (
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
	v, ok := foldExpr(site.expr, site.file, f, nil, 0)
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
		if v, ok := foldExpr(val, file, f, nil, 0); ok {
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

// foldExpr computes the literal "skeleton" of expr: the concatenation
// of every literal-provable fragment, with non-literal fragments
// dropped (replaced by a single space, so adjacent literal tokens on
// either side of a dynamic gap never fuse into a token neither side
// alone would form). hasLiteral is false ONLY when expr contributes NO
// literal content at all — a bare call, a bare non-local identifier,
// or an unresolvable selector — which is exactly the genuinely opaque
// shape the opaque-operand registry exists for (spec 127 R5c, C-r4-6):
// a partially-foldable expression (a literal flag prefix concatenated
// with a dynamic ID/branch-name suffix, matching this codebase's own
// convention of "flags are literal, values are dynamic") is
// classified purely on its literal skeleton — the same principle the
// spec itself applies to fmt.Sprintf's literal TEMPLATE, extended
// uniformly to bare `+` concatenation.
//
// depth guards against runaway recursion through mutually-referential
// consts/calls; it is not expected to matter for any real site at this
// base. file provides the enclosing file's import table, needed to
// resolve a cross-package selector const (e.g.
// `containment.RejectionLever`) to the OTHER package's pkgDir.
func foldExpr(expr ast.Expr, file *rFile, f *pkgConstFolder, binds map[string]string, depth int) (string, bool) {
	pkgDir := file.pkgDir
	if depth > 6 {
		return "", false
	}
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			if v, ok := unquote(e.Value); ok {
				return v, true
			}
		}
		return "", false
	case *ast.ParenExpr:
		return foldExpr(e.X, file, f, binds, depth+1)
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", false
		}
		ls, lok := foldExpr(e.X, file, f, binds, depth+1)
		rs, rok := foldExpr(e.Y, file, f, binds, depth+1)
		if !lok && !rok {
			return "", false
		}
		if !lok {
			ls = " "
		}
		if !rok {
			rs = " "
		}
		return ls + rs, true
	case *ast.Ident:
		if binds != nil {
			if v, ok := binds[e.Name]; ok {
				return v, true
			}
		}
		if v, ok := f.Get(pkgDir, e.Name); ok {
			return v, true
		}
		return "", false
	case *ast.SelectorExpr:
		// A cross-package selector const, e.g. containment.RejectionLever
		// — resolve the package alias to its import path, then to that
		// package's own pkgDir (an internal/cmd package only; an
		// external dependency has no pkgDir in this universe and
		// correctly fails to fold).
		xid, ok := e.X.(*ast.Ident)
		if !ok {
			return "", false
		}
		importPath, ok := file.imports[xid.Name]
		if !ok {
			return "", false
		}
		const modPrefix = "github.com/mrmaxsteel/mindspec/"
		if !strings.HasPrefix(importPath, modPrefix) {
			return "", false
		}
		otherPkgDir := strings.TrimPrefix(importPath, modPrefix)
		if v, ok := f.Get(otherPkgDir, e.Sel.Name); ok {
			return v, true
		}
		return "", false
	case *ast.CallExpr:
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			// fmt.Sprintf: provable via its literal TEMPLATE only
			// (spec 127 R5c's own listed shape) — substituted args
			// are deliberately never inspected.
			if xid, ok2 := sel.X.(*ast.Ident); ok2 && xid.Name == "fmt" && sel.Sel.Name == "Sprintf" && len(e.Args) > 0 {
				return foldExpr(e.Args[0], file, f, binds, depth+1)
			}
			// Trust boundary, stated: any *.RecoveryCommand() call is
			// treated as contributing no literal content of its own —
			// TRUSTED, not ignored — because every RecoveryCommand-
			// shaped method's own return expressions are independently
			// scanned by this file's leg (ii) against the SAME
			// classifier. A caller passing one through is transitively
			// covered by that independent check, not exempted from it.
			if sel.Sel.Name == "RecoveryCommand" {
				return "", true
			}
			// containment.EmitCd is a second trusted helper: its own
			// body (internal/workspace/containment/containment.go)
			// unconditionally renders `cd <shell-safe target>` — never
			// a floor match, for any target — so treating its call
			// sites as contributing no literal content (rather than
			// as ~20 individually-registered opaque operands) is
			// exact, not a widened exemption: verified once, here.
			if xid, ok2 := sel.X.(*ast.Ident); ok2 && xid.Name == "containment" && sel.Sel.Name == "EmitCd" {
				return "", true
			}
		}
		return "", false
	}
	return "", false
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
	provenance bool          // constructor-derived — exempt regardless
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
								provenance:      isConstructorDerived(arg, file),
								commandPosition: true,
							})
							continue
						}
						skel, hasLit := foldExpr(arg, file, f, b, 0)
						matches := guard.FindFloorMatches(skel)
						out = append(out, diagFinding{
							site:            rSite{Rel: file.rel, Func: enclosingFunc(file, node.Pos()), Detail: detailFor(matches, skel, i), Line: fset(u, node.Pos())},
							matches:         matches,
							hasLiteral:      hasLit,
							provenance:      isConstructorDerived(arg, file),
							commandPosition: i >= 1,
						})
					}
				}
			case *ast.FuncDecl:
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
						skel, hasLit := foldExpr(ret.Results[0], file, f, b, 0)
						matches := guard.FindFloorMatches(skel)
						out = append(out, diagFinding{
							site:            rSite{Rel: file.rel, Func: enclosingFunc(file, node.Pos()), Detail: detailFor(matches, skel, 0), Line: fset(u, ret.Pos())},
							matches:         matches,
							hasLiteral:      hasLit,
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
					skel, hasLit := foldExpr(kv.Value, file, f, b, 0)
					matches := guard.FindFloorMatches(skel)
					out = append(out, diagFinding{
						site:       rSite{Rel: file.rel, Func: enclosingFunc(file, node.Pos()), Detail: detailFor(matches, skel, 0), Line: fset(u, kv.Pos())},
						matches:    matches,
						hasLiteral: hasLit,
						provenance: isConstructorDerived(kv.Value, file),
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
	for _, e := range guard.KnownSitesExemptionList {
		matches := guard.FindFloorMatches(e.Text)
		found := false
		for _, m := range matches {
			if m.Family == e.Family {
				found = true
			}
		}
		if !found {
			t.Errorf("exemption entry %s/%q recorded family %s, but the classifier no longer matches it (hollow entry or classifier drift): got %+v", e.Surface, e.Text, e.Family, matches)
		}
	}
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
func TestDestructiveGuidanceSweep_WarningDeletionRewordDuplicationHollow(t *testing.T) {
	if len(guard.KnownSitesExemptionList) == 0 {
		t.Fatal("exemption list is empty — nothing to exercise")
	}
	entry := guard.KnownSitesExemptionList[0]
	base := map[string]string{entry.Surface + ".synthetic": entry.Text + "\n"}

	t.Run("deletion", func(t *testing.T) {
		found := scanGuidanceSurfaces(map[string]string{entry.Surface + ".synthetic": "the warning is gone now\n"})
		exempt := map[guidanceHitKey]int{{Surface: entry.Surface + ".synthetic", Text: entry.Text}: 1}
		_, _, staleAbsent := diffGuidanceHits(found, exempt)
		if len(staleAbsent) == 0 {
			t.Fatal("expected the deletion to register as a stale-absent entry")
		}
	})

	t.Run("reword", func(t *testing.T) {
		reworded := entry.Text + " (reworded)"
		found := scanGuidanceSurfaces(map[string]string{entry.Surface + ".synthetic": reworded + "\n"})
		exempt := map[guidanceHitKey]int{{Surface: entry.Surface + ".synthetic", Text: entry.Text}: 1}
		unlisted, _, staleAbsent := diffGuidanceHits(found, exempt)
		if len(staleAbsent) == 0 {
			t.Fatal("expected the reworded line to register as stale-absent under the original quoted string")
		}
		if len(unlisted) == 0 {
			t.Fatal("expected the reworded line itself to be an unlisted floor match")
		}
	})

	t.Run("duplication", func(t *testing.T) {
		found := scanGuidanceSurfaces(base)
		found[guidanceHitKey{Surface: entry.Surface + ".synthetic", Text: entry.Text}]++ // simulate a second occurrence
		exempt := map[guidanceHitKey]int{{Surface: entry.Surface + ".synthetic", Text: entry.Text}: 1}
		_, staleCount, _ := diffGuidanceHits(found, exempt)
		if len(staleCount) == 0 {
			t.Fatal("expected a duplicated occurrence to register as a count-drift stale entry")
		}
	})

	t.Run("hollow", func(t *testing.T) {
		hollow := guard.KnownSiteExemptionEntry{Surface: "synthetic", Text: "mindspec complete <bead>", Count: 1, Family: guard.FamilyGitMerge}
		matches := guard.FindFloorMatches(hollow.Text)
		found := false
		for _, m := range matches {
			if m.Family == hollow.Family {
				found = true
			}
		}
		if found {
			t.Fatal("fixture setup error: expected the hollow entry's text to NOT match its claimed family")
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
// fold rules cannot prove is EITHER constructor-derived OR present on
// the opaque-operand registry.
func TestProductDiagnosticScan_UnprovableCommandOperandsAreGoverned(t *testing.T) {
	u := loadRatchetUniverse(t)
	f := newPkgConstFolder(u)
	findings := scanProductDiagnostics(t, u, f)
	registry := registryMap()
	var ungoverned []string
	for _, fd := range findings {
		if len(fd.matches) > 0 || fd.hasLiteral || !fd.commandPosition {
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
		if len(fd.matches) == 0 && !fd.hasLiteral && fd.commandPosition && !fd.provenance {
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
// literal or an unexported-field write of the opaque type is red
// anywhere in a "package guard" file OUTSIDE NewDestructiveCommand's
// own implementation. Package identity is by PACKAGE CLAUSE name
// ("package guard"), not by repo path, so a fixture universe need not
// live at a real "internal/guard" path to be recognized.
func scanSamePackageInvariant(u *rUniverse) []string {
	var problems []string
	for _, file := range u.files {
		if file.file.Name == nil || file.file.Name.Name != "guard" {
			continue
		}
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
					if id, ok := node.Type.(*ast.Ident); ok && id.Name == "DestructiveCommand" && !isConstructorImpl {
						problems = append(problems, file.rel+":"+itoa(u.fset.Position(node.Pos()).Line)+": DestructiveCommand composite literal outside NewDestructiveCommand's implementation (same-package forge)")
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
// manifest and its three hermetic fixtures — reconciliation (α),
// registry identity (β), content regeneration (γ) — applied uniformly
// to all three seeded artifacts.
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
