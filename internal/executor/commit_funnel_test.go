package executor

// Spec 127 R5(d)(v) — the COMMIT-FUNNEL anti-drift scan, the sibling of
// merge_chokepoint_test.go's merge-producer scan.
//
// R5(d)(v)'s preserved-merge obligation binds "every product path that
// commits inside a worktree". The shipped design discharges that
// obligation NOT by repeating the check at each call site but at a single
// funnel: `commitWithExport` calls `checkNoPreservedMerge` once, then
// delegates to `gitutil.CommitAll`. Every committing path — this
// package's own internal `commitWithExport` calls and the external
// `Executor.CommitAll` callers in `internal/approve`, `internal/complete`
// and `internal/spec` — reaches it through that one function. (No count
// is written here on purpose: a count is the thing this scan replaces.
// The membership is whatever the scan finds, and the assertions hold
// over all of it.)
//
// That funnel WAS the design and the enumeration WAS prose. An
// enumeration of N call sites rots the moment a caller is added; a funnel
// invariant does not. This scan converts the written count into a derived
// guarantee, asserting four properties over the repo's real production
// source:
//
//	(1) EVERY resolved `gitutil.CommitAll` call in production code is
//	    lexically inside `commitWithExport` — the funnel is the only
//	    production caller of the commit primitive.
//	(2) `commitWithExport` is declared exactly once, and inside it the
//	    precondition `checkNoPreservedMerge` is called BEFORE
//	    `gitutil.CommitAll`, both DIRECTLY IN ITS OWN SPAN — a call to
//	    either that sits inside a nested closure is not counted at all,
//	    because a closure's body may never run at the point the funnel
//	    commits.
//	(3) NO production function or method named `CommitAll` can reach git —
//	    the `gitutil` package or `os/exec` — except through
//	    `commitWithExport`. This is decided against a call graph derived
//	    from the WHOLE production tree, not from the one method body:
//	    every call chain this scan can resolve BY NAME is followed, to
//	    unbounded depth, with edges INTO the funnel cut (reaching git
//	    through the funnel is the sanctioned path, and property (2) is
//	    what guards it). An implementation that reaches git off the funnel
//	    REDS whether or not it also calls the funnel; one that reaches git
//	    nowhere passes. Both answers are DERIVED — the MockExecutor is
//	    exempt because the graph says it cannot reach git, never because a
//	    written allowlist says so.
//	(4) No production caller inside package `gitutil` itself reaches
//	    `CommitAll` by its bare, same-package name, which would bypass the
//	    funnel without ever writing the `gitutil.` qualifier this scan
//	    keys on.
//
// Property (3) is what carries the EXTERNAL callers. This scan has no
// go/types, so it cannot prove that `exec.CommitAll(...)` in
// `internal/complete` dispatches to `*MindspecExecutor`. It does not need
// to: (3) quantifies over EVERY production `CommitAll` implementation in
// the tree, so whichever one an interface call lands on, that
// implementation either funnels or cannot reach git.
//
// WHAT "BY NAME" MEANS, AND WHAT EVADES IT. The graph's edges are exactly
// the callees this scan can name without types: a bare `foo(...)` and a
// receiver call `x.foo(...)` both resolve to every production declaration
// named `foo` in the CALLING package (unioned over receivers, which
// over-approximates in the fail-closed direction), and `pkg.Foo(...)`
// resolves through the file's own import block whenever `pkg` names a
// package inside this module. Depth is unbounded: reachability is a
// monotone fixed point, so recursion and mutual recursion converge rather
// than truncating at a hop limit. What that leaves outside — the honest
// residual, on the same footing as the merge chokepoint scan's:
//
//   - a call through a function VALUE (a var, a struct field, a
//     parameter) or through reflection, where no callee name is written;
//   - a method call whose receiver's type is declared in ANOTHER package,
//     since the method name is only matched inside the calling package;
//   - a dot-imported `gitutil`, which binds names this scan never sees;
//   - a callback handed to a stdlib or third-party package and invoked
//     from there, since only this module's source is parsed.
//
// The first, third and fourth are the shapes merge_chokepoint_test.go
// documents as out of scope for a go/types-free ratchet, and the same
// precedent (R5(b)) governs. The second is this graph's own boundary and
// is stated here rather than left to be discovered: a `CommitAll` that
// delegates to a helper method on a type declared in a different package,
// where that helper shells out to git, is not traced. What IS traced —
// and was not before this property was rebuilt — is the ordinary refactor
// shape `CommitAll` → same-package helper → `os/exec`, at any depth.
//
// AND THE RESIDUAL THAT IS NOT ABOUT EDGES AT ALL — the biggest one, and
// the one an earlier version of this header covered up. This scan has
// exactly two subjects: `gitutil.CommitAll` CALL SITES, and DECLARATIONS
// NAMED `CommitAll`. A function that commits by assembling `git commit`
// argv through `os/exec` by hand, is not named `CommitAll`, and is not
// reachable from any declaration that is, is invisible to all four
// properties — it is not a bypass this scan fails to prove safe, it is a
// path this scan never looks at. The previous wording claimed that shape
// was "bounded by property (3)'s fail-closed treatment of any `CommitAll`
// body that can reach `os/exec` at all"; that was false then and would be
// false now, because property (3)'s reach analysis STARTS at `CommitAll`
// declarations and can only ever describe what they can get to. R5(d)(v)'s
// obligation over that shape rests on review and on the merge-producer
// scan's own coverage, not on this file. Naming it is the point: the
// defect this artifact exists to end is a claim outrunning its mechanism,
// and a residual list that quietly drops the widest gap is that same
// defect wearing a disclosure's clothes.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const (
	// commitFunnelFuncName is the one function every committing path
	// must pass through.
	commitFunnelFuncName = "commitWithExport"
	// commitPrimitiveName is the gitutil primitive the funnel delegates
	// to — `git add -A` + `git commit -m`, which run mid-merge SUCCEEDS
	// and silently produces the two-parent chore: commit spec 125
	// shipped to fix.
	commitPrimitiveName = "CommitAll"
	// commitPreconditionName is the R5(d)(v) precondition the funnel
	// consults before delegating.
	commitPreconditionName = "checkNoPreservedMerge"

	osExecImportPath = "os/exec"
)

// commitFunnelCall is one call site the scan resolved, labelled with the
// function-like construct that DIRECTLY contains it (never inherited from
// an enclosing scope — a nested closure gets its own label, because a
// closure's body may never run at the point the enclosing function
// commits).
type commitFunnelCall struct {
	file string
	// enclosing is the human-facing label, receiver included
	// (`(*MindspecExecutor).commitWithExport`), so a failure names the
	// exact declaration.
	enclosing string
	// enclosingName is the bare declared name the funnel identity is
	// matched on — empty for a func literal, which is never the funnel.
	enclosingName string
	pos           token.Position
}

// commitFunnelFuncKey identifies a production declaration by the package
// directory that declares it and its declared name. Methods are keyed by
// their method NAME, so several receivers can share one key: the graph's
// answer for a key is the union over every declaration under it, which
// over-approximates in the fail-closed direction.
type commitFunnelFuncKey struct {
	dir  string
	name string
}

// commitFunnelDecl is one production `func`/method declaration, with the
// reachability facts derived from its own body and the edges its body
// contributes to the whole-tree call graph.
type commitFunnelDecl struct {
	file  string
	label string
	pos   token.Position
	key   commitFunnelFuncKey
	// callsFunnel is true when the body calls commitWithExport.
	callsFunnel bool
	// gitutilCalls names every `<gitutil>.X` function the body calls, in
	// source order — the evidence that a body can reach git directly.
	gitutilCalls []string
	// callsOSExec is true when the body invokes os/exec, the other way a
	// body can reach git without naming the gitutil package.
	callsOSExec bool
	// callees are the production declarations this body calls that the
	// scan can resolve BY NAME (see the header's residual list for what
	// that excludes). Deduplicated, in first-written order.
	callees []commitFunnelFuncKey
	// preconditionPos / primitivePos are the positions of the funnel's
	// own precondition and commit calls, for the ordering check — derived
	// from the funnel's DIRECT span only. Zero when absent.
	preconditionPos token.Pos
	primitivePos    token.Pos
}

// commitFunnelFileScan is everything the scan derives from one file.
type commitFunnelFileScan struct {
	// qualifiedPrimitiveCalls are `<gitutil>.CommitAll(...)` calls,
	// resolved through the file's OWN import alias for gitutil.
	qualifiedPrimitiveCalls []commitFunnelCall
	// barePrimitiveCalls are same-package `CommitAll(...)` calls — only
	// meaningful inside package gitutil, where no qualifier is written.
	barePrimitiveCalls []commitFunnelCall
	// decls is EVERY function and method declaration in the file — the
	// nodes of the call graph property (3) is decided against, not just
	// the two names this scan asserts about.
	decls []*commitFunnelDecl
	// funnelDecls are declarations of commitWithExport itself.
	funnelDecls []*commitFunnelDecl
	// primitiveDecls are declarations named CommitAll (the gitutil
	// definition, the executor method, and every other implementation).
	primitiveDecls []*commitFunnelDecl
}

// commitFunnelSkipDirs are the directories the production-source walk
// never descends into. `testdata` holds deliberately-malformed lint
// fixtures; `beads` and `vendor` are third-party trees this repo's
// invariants do not govern; dot-directories hold no Go source.
var commitFunnelSkipDirs = map[string]bool{
	"testdata":     true,
	"beads":        true,
	"vendor":       true,
	"node_modules": true,
}

// commitFunnelImportLocalName resolves the identifier file's OWN import
// line binds importPath to, falling back to fallback when the file
// carries no matching import (this scan's own single-file fixture
// sources are syntax snippets with no import block; every real
// production file that uses a package imports it).
func commitFunnelImportLocalName(file *ast.File, importPath, fallback string) string {
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != importPath {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name
		}
		return fallback
	}
	return fallback
}

// commitFunnelModulePath is this module's import-path prefix, DERIVED
// from the merge chokepoint scan's own gitutil import path rather than
// written down a second time.
var commitFunnelModulePath = strings.TrimSuffix(gitutilImportPath, "/internal/gitutil")

// commitFunnelImportDirs splits file's import block in two: the
// identifiers naming packages INSIDE this module, mapped to the
// repo-relative directory that declares them (the call graph's key
// space), and the set of every identifier the block binds, in-module or
// not. The second result is what keeps a third-party `x.Foo()` from being
// mistaken for a same-package method named Foo.
//
// pkgNames maps a repo-relative directory to the package name its files
// declare, so an unaliased in-module import resolves to the identifier Go
// itself binds rather than to the last path element (they differ whenever
// a package's name is not its directory name). It is nil for this file's
// own fixture snippets, which import nothing; the last-path-element
// fallback then applies.
func commitFunnelImportDirs(file *ast.File, pkgNames map[string]string) (map[string]string, map[string]bool) {
	dirs := map[string]string{}
	bound := map[string]bool{}
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		inModule := ""
		if path == commitFunnelModulePath {
			inModule = "."
		} else if rest := strings.TrimPrefix(path, commitFunnelModulePath+"/"); rest != path {
			inModule = rest
		}
		local := path[strings.LastIndex(path, "/")+1:]
		if inModule != "" {
			if name, ok := pkgNames[inModule]; ok {
				local = name
			}
		}
		if imp.Name != nil {
			local = imp.Name.Name
		}
		// A blank import binds nothing; a dot import binds names this
		// scan cannot see at all (a stated residual).
		if local == "_" || local == "." {
			continue
		}
		bound[local] = true
		if inModule != "" {
			dirs[local] = inModule
		}
	}
	return dirs, bound
}

// commitFunnelDeclLabel renders a FuncDecl's label, receiver included, so
// a failure names the exact declaration (`(*MindspecExecutor).CommitAll`,
// not a bare `CommitAll` that could be any of several implementations).
func commitFunnelDeclLabel(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	return fmt.Sprintf("(%s).%s", commitFunnelTypeText(fd.Recv.List[0].Type), fd.Name.Name)
}

// commitFunnelTypeText renders a receiver type expression textually.
// Any shape it cannot render becomes a distinct placeholder rather than
// an empty string, so two unrelated receivers never collide in a label.
func commitFunnelTypeText(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.StarExpr:
		return "*" + commitFunnelTypeText(v.X)
	case *ast.IndexExpr:
		return commitFunnelTypeText(v.X)
	case *ast.IndexListExpr:
		return commitFunnelTypeText(v.X)
	case *ast.SelectorExpr:
		return commitFunnelTypeText(v.X) + "." + v.Sel.Name
	default:
		return fmt.Sprintf("unrendered-receiver-%T", e)
	}
}

// commitFunnelWalkSpans walks root attributing every CallExpr to the
// function-like construct that DIRECTLY contains it. Descent stops at
// every nested *ast.FuncLit boundary, which is then walked as its own
// span — a closure's calls are never credited to its enclosing function.
// A func literal's span carries an empty declared NAME, so a commit
// inside a closure can never satisfy the funnel-identity match no matter
// what encloses the closure.
func commitFunnelWalkSpans(root ast.Node, fset *token.FileSet, label, name string, visit func(*ast.CallExpr, string, string, token.Position)) {
	ast.Inspect(root, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.FuncLit:
			commitFunnelWalkSpans(v.Body, fset, fmt.Sprintf("func literal at %s", fset.Position(v.Pos())), "", visit)
			return false
		case *ast.CallExpr:
			visit(v, label, name, fset.Position(v.Pos()))
			return true
		}
		return true
	})
}

// commitFunnelCalleeName renders a call's callee as either a bare
// identifier name (ident, "") or a qualified pair (pkg, sel). Parenthesized
// callees are unwrapped so `(gitutil.CommitAll)(...)` resolves identically
// to `gitutil.CommitAll(...)`.
func commitFunnelCalleeName(call *ast.CallExpr) (pkg, sel string) {
	fun := call.Fun
	for {
		p, ok := fun.(*ast.ParenExpr)
		if !ok {
			break
		}
		fun = p.X
	}
	switch v := fun.(type) {
	case *ast.Ident:
		return "", v.Name
	case *ast.SelectorExpr:
		if x, ok := v.X.(*ast.Ident); ok {
			return x.Name, v.Sel.Name
		}
		return "", ""
	}
	return "", ""
}

// scanCommitFunnelFile derives every fact the funnel assertions need from
// one parsed production file. name is the file's repo-relative path — its
// directory is the package key every declaration and edge is recorded
// under. pkgNames is the whole tree's directory-to-package-name map (nil
// for the single-file fixtures below).
func scanCommitFunnelFile(fset *token.FileSet, file *ast.File, name string, pkgNames map[string]string) commitFunnelFileScan {
	gitutilName := gitutilLocalName(file)
	osExecName := commitFunnelImportLocalName(file, osExecImportPath, "exec")
	dir := filepath.ToSlash(filepath.Dir(name))
	importDirs, importBound := commitFunnelImportDirs(file, pkgNames)

	// resolveCallee renders one written callee as the key of the
	// production declaration(s) it names, when this scan can name any.
	// The graph build discards keys no declaration answers to, which is
	// how builtins, local func-valued variables and stdlib calls drop
	// out without being enumerated.
	resolveCallee := func(pkg, sel string) (commitFunnelFuncKey, bool) {
		if sel == "" {
			return commitFunnelFuncKey{}, false
		}
		if pkg == "" {
			// A bare name: a same-package function, a local func-valued
			// variable, or a builtin.
			return commitFunnelFuncKey{dir: dir, name: sel}, true
		}
		if d, ok := importDirs[pkg]; ok {
			// A package inside this module, under this file's own name
			// for it.
			return commitFunnelFuncKey{dir: d, name: sel}, true
		}
		if importBound[pkg] {
			// A package outside this module: not source this walk
			// parses, so there is nothing to follow.
			return commitFunnelFuncKey{}, false
		}
		// Not a package qualifier at all — a method call on a receiver.
		// Without go/types the receiver's type is unknown, so this
		// resolves by method NAME within the calling package, unioned
		// over every receiver that declares it (see the header's
		// residuals for the cross-package receiver this misses).
		return commitFunnelFuncKey{dir: dir, name: sel}, true
	}

	var out commitFunnelFileScan

	record := func(call *ast.CallExpr, label, declName string, pos token.Position) {
		pkg, sel := commitFunnelCalleeName(call)
		switch {
		case pkg == gitutilName && sel == commitPrimitiveName:
			out.qualifiedPrimitiveCalls = append(out.qualifiedPrimitiveCalls,
				commitFunnelCall{file: name, enclosing: label, enclosingName: declName, pos: pos})
		case pkg == "" && sel == commitPrimitiveName:
			out.barePrimitiveCalls = append(out.barePrimitiveCalls,
				commitFunnelCall{file: name, enclosing: label, enclosingName: declName, pos: pos})
		}
	}

	// Package-level declarations (var initializers, init-time
	// composite literals) are walked too: a commit call there belongs to
	// no function and must never pass unseen.
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			commitFunnelWalkSpans(decl, fset, fmt.Sprintf("package-level declaration in %s", name), "", record)
			continue
		}
		if fd.Body == nil {
			continue
		}
		commitFunnelWalkSpans(fd.Body, fset, commitFunnelDeclLabel(fd), fd.Name.Name, record)

		d := &commitFunnelDecl{
			file:  name,
			label: commitFunnelDeclLabel(fd),
			pos:   fset.Position(fd.Pos()),
			key:   commitFunnelFuncKey{dir: dir, name: fd.Name.Name},
		}
		// The reachability facts and the call-graph edges are derived
		// from the WHOLE body, closures included: a body that can reach
		// git from inside a closure can still reach git.
		seen := map[commitFunnelFuncKey]bool{}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			pkg, sel := commitFunnelCalleeName(call)
			switch {
			case pkg == gitutilName:
				d.gitutilCalls = append(d.gitutilCalls, sel)
			case pkg == osExecName && (sel == "Command" || sel == "CommandContext"):
				d.callsOSExec = true
			}
			// A method call on a receiver (`g.commitWithExport(...)`)
			// reads as a qualified pair whose package half is the
			// receiver name, so the funnel is recognised by selector
			// alone — bare call and method call both.
			if sel == commitFunnelFuncName {
				d.callsFunnel = true
			}
			if k, ok := resolveCallee(pkg, sel); ok && !seen[k] {
				seen[k] = true
				d.callees = append(d.callees, k)
			}
			return true
		})
		out.decls = append(out.decls, d)

		switch fd.Name.Name {
		case commitFunnelFuncName:
			// The funnel's own ordering facts come from its DIRECT span
			// only: commitFunnelWalkSpans hands a nested closure an
			// EMPTY declared name, and a precondition that runs only
			// inside a closure — which may never run — must not read as
			// a precondition the commit is guarded by. Failing to find
			// either call is failing closed: property (2) then reports
			// the funnel as no longer checking, or no longer committing.
			commitFunnelWalkSpans(fd.Body, fset, d.label, fd.Name.Name,
				func(call *ast.CallExpr, _, declName string, _ token.Position) {
					if declName == "" {
						return
					}
					pkg, sel := commitFunnelCalleeName(call)
					switch {
					case pkg == "" && sel == commitPreconditionName && d.preconditionPos == token.NoPos:
						d.preconditionPos = call.Pos()
					case pkg == gitutilName && sel == commitPrimitiveName && d.primitivePos == token.NoPos:
						d.primitivePos = call.Pos()
					}
				})
			out.funnelDecls = append(out.funnelDecls, d)
		case commitPrimitiveName:
			out.primitiveDecls = append(out.primitiveDecls, d)
		}
	}
	return out
}

// commitFunnelProductionFiles parses every non-test .go file under root
// that this repo's invariants govern. Returns the parsed files paired
// with their repo-relative names.
func commitFunnelProductionFiles(t *testing.T, root string) (*token.FileSet, []*ast.File, []string) {
	t.Helper()
	fset := token.NewFileSet()
	var files []*ast.File
	var names []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if path != root && (strings.HasPrefix(base, ".") || commitFunnelSkipDirs[base]) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parsed, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return fmt.Errorf("parsing %s: %w", path, perr)
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			rel = path
		}
		files = append(files, parsed)
		names = append(names, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walking production source under %s: %v", root, err)
	}
	return fset, files, names
}

// commitFunnelPackageNames maps each parsed directory to the package name
// its files declare, so an unaliased in-module import resolves to the
// identifier Go binds rather than to the directory's base name.
func commitFunnelPackageNames(files []*ast.File, names []string) map[string]string {
	out := map[string]string{}
	for i, f := range files {
		if f.Name == nil {
			continue
		}
		out[filepath.ToSlash(filepath.Dir(names[i]))] = f.Name.Name
	}
	return out
}

// commitFunnelClassification is property (3)'s answer over a whole parsed
// tree: one witness chain per CommitAll implementation that can reach git
// off the funnel, and how many implementations delegate to it.
type commitFunnelClassification struct {
	// bypasses maps a declaration's label to the call chain that reaches
	// git without passing the funnel — the evidence a failure prints,
	// rather than asking the reader to take the answer on trust.
	bypasses map[string][]string
	byLabel  map[string]*commitFunnelDecl
	// delegating counts implementations that call the funnel and have no
	// other reach.
	delegating int
}

// commitFunnelReachesGit renders the direct evidence that d's own body
// can reach git, or "" when it names neither route.
func commitFunnelReachesGit(d *commitFunnelDecl) string {
	if len(d.gitutilCalls) > 0 {
		return fmt.Sprintf("gitutil.%s", d.gitutilCalls[0])
	}
	if d.callsOSExec {
		return osExecImportPath
	}
	return ""
}

// commitFunnelReach computes, for every key in index, whether ANY
// declaration under it can reach git by a chain of name-resolvable calls,
// with edges into funnel cut. It is a monotone fixed point rather than a
// bounded walk, so recursion and mutual recursion converge instead of
// truncating at a depth limit.
func commitFunnelReach(index map[commitFunnelFuncKey][]*commitFunnelDecl, funnel commitFunnelFuncKey) map[commitFunnelFuncKey]bool {
	reach := map[commitFunnelFuncKey]bool{}
	for k, decls := range index {
		for _, d := range decls {
			if commitFunnelReachesGit(d) != "" {
				reach[k] = true
				break
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for k, decls := range index {
			if reach[k] {
				continue
			}
			for _, d := range decls {
				for _, c := range d.callees {
					if c == funnel || !reach[c] {
						continue
					}
					reach[k] = true
					changed = true
					break
				}
				if reach[k] {
					break
				}
			}
		}
	}
	return reach
}

// commitFunnelWitness renders the shortest name-resolvable call chain
// from start to a body that names gitutil or os/exec directly, with the
// funnel cut out. Returns nil when start cannot reach git that way.
func commitFunnelWitness(index map[commitFunnelFuncKey][]*commitFunnelDecl, reach map[commitFunnelFuncKey]bool, funnel commitFunnelFuncKey, start *commitFunnelDecl) []string {
	if ev := commitFunnelReachesGit(start); ev != "" {
		return []string{start.label, ev}
	}
	type step struct {
		key  commitFunnelFuncKey
		path []string
	}
	seen := map[commitFunnelFuncKey]bool{}
	var queue []step
	push := func(from []string, callees []commitFunnelFuncKey) {
		for _, c := range callees {
			if c == funnel || seen[c] || !reach[c] {
				continue
			}
			seen[c] = true
			queue = append(queue, step{key: c, path: append(append([]string{}, from...), c.name)})
		}
	}
	push([]string{start.label}, start.callees)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, d := range index[cur.key] {
			if ev := commitFunnelReachesGit(d); ev != "" {
				return append(cur.path, ev)
			}
		}
		for _, d := range index[cur.key] {
			push(cur.path, d.callees)
		}
	}
	return nil
}

// commitFunnelClassify decides property (3) over a set of scanned files.
// The production assertion below and this file's fixture tests run THIS
// function, so a fixture proving a bypass shape is caught proves it for
// the production tree too.
func commitFunnelClassify(scans []commitFunnelFileScan) commitFunnelClassification {
	index := map[commitFunnelFuncKey][]*commitFunnelDecl{}
	var funnelDecls, primitiveDecls []*commitFunnelDecl
	for _, s := range scans {
		for _, d := range s.decls {
			index[d.key] = append(index[d.key], d)
		}
		funnelDecls = append(funnelDecls, s.funnelDecls...)
		primitiveDecls = append(primitiveDecls, s.primitiveDecls...)
	}
	// Drop every edge no declaration answers to — builtins, local
	// func-valued variables, stdlib calls under a name this file's import
	// block did not bind.
	for _, decls := range index {
		for _, d := range decls {
			kept := d.callees[:0]
			for _, c := range d.callees {
				if len(index[c]) > 0 {
					kept = append(kept, c)
				}
			}
			d.callees = kept
		}
	}
	// The funnel is cut from the graph: reaching git THROUGH it is the
	// sanctioned path. A tree with no single funnel cuts nothing, so
	// every route reds — property (2) is what reports that case.
	var funnel commitFunnelFuncKey
	if len(funnelDecls) == 1 {
		funnel = funnelDecls[0].key
	}
	reach := commitFunnelReach(index, funnel)

	out := commitFunnelClassification{
		bypasses: map[string][]string{},
		byLabel:  map[string]*commitFunnelDecl{},
	}
	for _, d := range primitiveDecls {
		// The gitutil definition itself is the primitive, not an
		// implementation of the interface — derived as such by living in
		// the package the funnel calls INTO.
		if strings.HasPrefix(filepath.ToSlash(d.file), "internal/gitutil/") {
			continue
		}
		out.byLabel[d.label] = d
		if w := commitFunnelWitness(index, reach, funnel, d); w != nil {
			out.bypasses[d.label] = w
			continue
		}
		if d.callsFunnel {
			out.delegating++
		}
	}
	return out
}

// TestCommitFunnel_CommitAllIsReachedOnlyThroughCommitWithExport is spec
// 127 R5(d)(v)'s derived guarantee, replacing the prose enumeration of
// call sites that the final-review panel measured drifting twice. It
// fails if a production caller reaches gitutil.CommitAll outside the
// funnel, if a CommitAll implementation can reach git without funnelling,
// or if the funnel's precondition stops preceding its commit.
func TestCommitFunnel_CommitAllIsReachedOnlyThroughCommitWithExport(t *testing.T) {
	root := mergeChokepointRepoRoot(t)
	fset, files, names := commitFunnelProductionFiles(t, root)
	if len(files) == 0 {
		t.Fatalf("no production Go files found under %s — the scan would be vacuously green", root)
	}

	pkgNames := commitFunnelPackageNames(files, names)

	var qualified, bare []commitFunnelCall
	var funnelDecls []*commitFunnelDecl
	scans := make([]commitFunnelFileScan, 0, len(files))
	for i, file := range files {
		scan := scanCommitFunnelFile(fset, file, names[i], pkgNames)
		scans = append(scans, scan)
		qualified = append(qualified, scan.qualifiedPrimitiveCalls...)
		funnelDecls = append(funnelDecls, scan.funnelDecls...)
		// Bare same-package reach only exists inside package gitutil,
		// where CommitAll needs no qualifier; a bare CommitAll elsewhere
		// is a different, unrelated function.
		if strings.HasPrefix(filepath.ToSlash(names[i]), "internal/gitutil/") {
			bare = append(bare, scan.barePrimitiveCalls...)
		}
	}

	// Property (1): every resolved gitutil.CommitAll call is in the funnel.
	if len(qualified) == 0 {
		t.Fatalf("no gitutil.%s call sites found anywhere in production source — "+
			"either the primitive was renamed or this scan stopped resolving it; "+
			"a green run here would prove nothing", commitPrimitiveName)
	}
	sort.Slice(qualified, func(a, b int) bool { return qualified[a].pos.String() < qualified[b].pos.String() })
	funnelled := 0
	for _, c := range qualified {
		if c.enclosingName == commitFunnelFuncName {
			funnelled++
			continue
		}
		t.Errorf("R5(d)(v) commit funnel breached: %s calls gitutil.%s from %q at %s.\n"+
			"The preserved-merge precondition is discharged ONCE, inside %s; a caller that "+
			"reaches the commit primitive outside it commits over a preserved merge, producing "+
			"the two-parent chore: commit spec 125 shipped to fix. Route this call through %s.",
			c.file, commitPrimitiveName, c.enclosing, c.pos, commitFunnelFuncName, commitFunnelFuncName)
	}
	if funnelled == 0 {
		t.Errorf("no gitutil.%s call reaches the funnel %s — the funnel no longer commits, "+
			"so nothing this test asserts about it is load-bearing", commitPrimitiveName, commitFunnelFuncName)
	}

	// Property (2): the funnel is singular, and its precondition precedes
	// its commit.
	if len(funnelDecls) != 1 {
		labels := make([]string, 0, len(funnelDecls))
		for _, d := range funnelDecls {
			labels = append(labels, fmt.Sprintf("%s:%d", d.file, d.pos.Line))
		}
		t.Fatalf("expected exactly one production declaration of %s, found %d (%v) — "+
			"a second funnel is a second place the precondition can be omitted",
			commitFunnelFuncName, len(funnelDecls), labels)
	}
	funnel := funnelDecls[0]
	if funnel.preconditionPos == token.NoPos {
		t.Errorf("%s (%s:%d) does not call %s — the R5(d)(v) preserved-merge precondition "+
			"is no longer discharged anywhere on the commit path",
			commitFunnelFuncName, funnel.file, funnel.pos.Line, commitPreconditionName)
	}
	if funnel.primitivePos == token.NoPos {
		t.Errorf("%s (%s:%d) no longer calls gitutil.%s — the funnel and the commit primitive "+
			"have separated, so the precondition no longer guards the commit",
			commitFunnelFuncName, funnel.file, funnel.pos.Line, commitPrimitiveName)
	}
	if funnel.preconditionPos != token.NoPos && funnel.primitivePos != token.NoPos &&
		funnel.preconditionPos > funnel.primitivePos {
		t.Errorf("%s (%s:%d) calls %s at offset %d, AFTER gitutil.%s at offset %d — "+
			"a precondition checked after the commit has already happened refuses nothing",
			commitFunnelFuncName, funnel.file, funnel.pos.Line, commitPreconditionName,
			funnel.preconditionPos, commitPrimitiveName, funnel.primitivePos)
	}

	// Property (3): no CommitAll implementation reaches git except through
	// the funnel, decided against the whole-tree call graph. This is what
	// carries the external Executor.CommitAll callers in internal/approve,
	// internal/complete and internal/spec without go/types.
	class := commitFunnelClassify(scans)
	bypassed := make([]string, 0, len(class.bypasses))
	for label := range class.bypasses {
		bypassed = append(bypassed, label)
	}
	sort.Strings(bypassed)
	for _, label := range bypassed {
		d := class.byLabel[label]
		t.Errorf("%s (%s:%d) is a %s implementation that can reach git WITHOUT passing %s:\n    %s\n"+
			"Every committing path must pass the funnel so the R5(d)(v) precondition is discharged; "+
			"this fails closed on any reach the call graph resolves, rather than judging whether this "+
			"particular chain happens to commit. Route it through %s.",
			label, d.file, d.pos.Line, commitPrimitiveName, commitFunnelFuncName,
			strings.Join(class.bypasses[label], " → "), commitFunnelFuncName)
	}
	delegating := class.delegating
	if delegating == 0 {
		t.Errorf("no production %s implementation delegates to %s — the exported wrapper the "+
			"external internal/approve, internal/complete and internal/spec call sites reach "+
			"the precondition through no longer exists", commitPrimitiveName, commitFunnelFuncName)
	}

	// Property (4): no same-package bare reach inside gitutil.
	for _, c := range bare {
		t.Errorf("%s reaches %s by its bare same-package name from %q at %s — a same-package "+
			"caller bypasses the funnel without ever writing the qualifier the scan keys on",
			c.file, commitPrimitiveName, c.enclosing, c.pos)
	}
}

// commitFunnelParse parses a fixture source snippet for the collector
// unit tests below.
func commitFunnelParse(t *testing.T, name, src string) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		t.Fatalf("parsing fixture %s: %v", name, err)
	}
	return fset, file
}

// commitFunnelScanFixtures parses a set of fixture sources keyed by their
// repo-relative names and returns the scans in name order, so a fixture
// can span several files and several packages — the shapes the whole-tree
// call graph exists to answer, which a single snippet cannot express.
func commitFunnelScanFixtures(t *testing.T, srcs map[string]string) []commitFunnelFileScan {
	t.Helper()
	names := make([]string, 0, len(srcs))
	for n := range srcs {
		names = append(names, n)
	}
	sort.Strings(names)
	fset := token.NewFileSet()
	files := make([]*ast.File, 0, len(names))
	for _, n := range names {
		f, err := parser.ParseFile(fset, n, srcs[n], 0)
		if err != nil {
			t.Fatalf("parsing fixture %s: %v", n, err)
		}
		files = append(files, f)
	}
	pkgNames := commitFunnelPackageNames(files, names)
	scans := make([]commitFunnelFileScan, 0, len(files))
	for i, f := range files {
		scans = append(scans, scanCommitFunnelFile(fset, f, names[i], pkgNames))
	}
	return scans
}

// commitFunnelFixtureFunnel is the honest funnel every classification
// fixture below carries, so `delegating` is non-zero for the same reason
// the production tree's is and the fixtures differ from production in
// exactly the shape under test.
var commitFunnelFixtureFunnel = fmt.Sprintf(`package executor

import "%s"

type MindspecExecutor struct{}

func (g *MindspecExecutor) CommitAll(path, msg string) error {
	return g.commitWithExport(path, msg)
}

func (g *MindspecExecutor) commitWithExport(path, msg string) error {
	if err := checkNoPreservedMerge(path, ""); err != nil {
		return err
	}
	return gitutil.CommitAll(path, msg)
}
`, gitutilImportPath)

// TestCommitFunnel_ScanCatchesCommitAllDelegatingToACommittingHelper is
// the mutation proof for property (3)'s rebuilt form, and the shape three
// independent adversary slots planted against its predecessor: a
// CommitAll whose OWN body names neither gitutil nor os/exec, and commits
// through an ordinary same-package helper. The predecessor read that body,
// found no git vocabulary in it, and passed it as provably non-committing.
//
// The control in the same test is what makes the red meaningful: the
// IDENTICAL fixture with the helper's git call removed — same delegation,
// same helper, same names — is not flagged. The scan therefore reds on the
// helper's reach, not on the mere presence of a helper call.
func TestCommitFunnel_ScanCatchesCommitAllDelegatingToACommittingHelper(t *testing.T) {
	const committing = `package complete

import "os/exec"

type altExecutor struct{}

func (a *altExecutor) CommitAll(path, msg string) error {
	return a.doCommit(path, msg)
}

func (a *altExecutor) doCommit(path, msg string) error {
	return exec.Command("git", "-C", path, "commit", "-am", msg).Run()
}
`
	const inert = `package complete

type altExecutor struct{}

func (a *altExecutor) CommitAll(path, msg string) error {
	return a.doCommit(path, msg)
}

func (a *altExecutor) doCommit(path, msg string) error {
	_ = path
	_ = msg
	return nil
}
`
	class := commitFunnelClassify(commitFunnelScanFixtures(t, map[string]string{
		"internal/executor/funnel.go": commitFunnelFixtureFunnel,
		"internal/complete/alt.go":    committing,
	}))
	witness, flagged := class.bypasses["(*altExecutor).CommitAll"]
	if !flagged {
		t.Fatalf("a CommitAll that commits through a same-package helper must be caught; flagged: %v", class.bypasses)
	}
	joined := strings.Join(witness, " → ")
	if !strings.Contains(joined, "doCommit") || !strings.HasSuffix(joined, osExecImportPath) {
		t.Errorf("the failure must carry the chain that reaches git, got %q", joined)
	}
	if _, ok := class.bypasses["(*MindspecExecutor).CommitAll"]; ok {
		t.Errorf("the honest delegating implementation must not be flagged: %v", class.bypasses)
	}
	if class.delegating != 1 {
		t.Errorf("expected the one honest implementation to count as delegating, got %d", class.delegating)
	}

	control := commitFunnelClassify(commitFunnelScanFixtures(t, map[string]string{
		"internal/executor/funnel.go": commitFunnelFixtureFunnel,
		"internal/complete/alt.go":    inert,
	}))
	if len(control.bypasses) != 0 {
		t.Errorf("the control — the same delegation to a helper that cannot reach git — must NOT be "+
			"flagged, or the red above proves only that a helper was called: %v", control.bypasses)
	}
}

// TestCommitFunnel_ScanFollowsAHelperChainAcrossPackages proves the graph
// resolves an in-module qualified call through the calling file's OWN
// import block, so a CommitAll that commits by way of a helper package —
// two hops, neither of them naming git in the method body — is still
// caught. Depth is not the boundary; name-resolvability is.
func TestCommitFunnel_ScanFollowsAHelperChainAcrossPackages(t *testing.T) {
	deep := fmt.Sprintf(`package spec

import helpers "%s/internal/helpers"

type specExecutor struct{}

func (s *specExecutor) CommitAll(path, msg string) error {
	return helpers.Persist(path, msg)
}
`, commitFunnelModulePath)
	helpers := fmt.Sprintf(`package helpers

import "%s"

func Persist(path, msg string) error {
	return stage(path, msg)
}

func stage(path, msg string) error {
	return gitutil.CommitAll(path, msg)
}
`, gitutilImportPath)

	class := commitFunnelClassify(commitFunnelScanFixtures(t, map[string]string{
		"internal/executor/funnel.go": commitFunnelFixtureFunnel,
		"internal/spec/create.go":     deep,
		"internal/helpers/persist.go": helpers,
	}))
	witness, flagged := class.bypasses["(*specExecutor).CommitAll"]
	if !flagged {
		t.Fatalf("a cross-package helper chain must be followed; flagged: %v", class.bypasses)
	}
	joined := strings.Join(witness, " → ")
	for _, hop := range []string{"Persist", "stage", "gitutil.CommitAll"} {
		if !strings.Contains(joined, hop) {
			t.Errorf("the witness must name every hop it followed; %q is missing %q", joined, hop)
		}
	}
}

// TestCommitFunnel_StatedLimit_CrossPackageReceiverHopIsNotTraced PINS A
// DISCLOSED BOUNDARY rather than proving coverage: it asserts what this
// scan does NOT catch, so the header's residual list cannot quietly go
// stale in either direction. If someone later widens the claim without
// widening the scan, nothing here changes and the header lies; if someone
// widens the SCAN, this test REDS and forces the residual to be rewritten
// in the same commit.
//
// The shape: `CommitAll` delegates to a method on a receiver whose type is
// declared in ANOTHER package, and that method shells out. Resolution is
// by name within the CALLING package, so the hop resolves to nothing and
// the edge is dropped. Contrast the same delegation with the helper in the
// same package, which IS caught — see
// TestCommitFunnel_ScanCatchesCommitAllDelegatingToACommittingHelper.
func TestCommitFunnel_StatedLimit_CrossPackageReceiverHopIsNotTraced(t *testing.T) {
	caller := `package complete

type crossExecutor struct{ h helpers.Committer }

func (c *crossExecutor) CommitAll(path, msg string) error {
	return c.h.doCommit(path, msg)
}
`
	helper := `package helpers

import "os/exec"

type Committer struct{}

func (c Committer) doCommit(path, msg string) error {
	return exec.Command("git", "-C", path, "commit", "-am", msg).Run()
}
`
	class := commitFunnelClassify(commitFunnelScanFixtures(t, map[string]string{
		"internal/executor/funnel.go":   commitFunnelFixtureFunnel,
		"internal/complete/cross.go":    caller,
		"internal/helpers/committer.go": helper,
	}))
	if _, flagged := class.bypasses["(*crossExecutor).CommitAll"]; flagged {
		t.Fatalf("the scan now traces a cross-package receiver hop — that is an IMPROVEMENT, but the "+
			"header's residual list still says it does not. Rewrite the residual (and this test) in the "+
			"same commit that widened the scan. Flagged: %v", class.bypasses)
	}
	// Non-vacuity: a fixture that silently stopped being classified at all
	// would also produce no flag. The honest funnel in the same set must
	// still read as delegating, and the bypassing implementation must be
	// one the classifier actually SAW.
	if class.delegating != 1 {
		t.Fatalf("the fixture set was not classified — expected the honest funnel to count as "+
			"delegating, got %d", class.delegating)
	}
	if _, seen := class.byLabel["(*crossExecutor).CommitAll"]; !seen {
		t.Fatal("the cross-package implementation was never examined, so this test pins nothing")
	}
}

// TestCommitFunnel_StatedLimit_ARawArgvCommitterOutsideCommitAllIsNotSeen
// pins the WIDEST disclosed boundary — the one the header now names
// explicitly and an earlier version papered over. This scan's subjects are
// gitutil.CommitAll call sites and declarations named CommitAll; a
// function that assembles `git commit` argv by hand, is named something
// else, and is reachable from no CommitAll implementation is not a bypass
// the scan fails to prove safe, it is a path the scan never looks at.
//
// This test exists so that claim stays true by mechanism. It REDS the day
// the scan's subject set widens, which is exactly when the header must
// change.
func TestCommitFunnel_StatedLimit_ARawArgvCommitterOutsideCommitAllIsNotSeen(t *testing.T) {
	const raw = `package complete

import "os/exec"

func syncNow(path, msg string) error {
	return exec.Command("git", "-C", path, "commit", "-am", msg).Run()
}
`
	scans := commitFunnelScanFixtures(t, map[string]string{
		"internal/executor/funnel.go": commitFunnelFixtureFunnel,
		"internal/complete/sync.go":   raw,
	})
	class := commitFunnelClassify(scans)
	if len(class.bypasses) != 0 {
		t.Fatalf("the scan now reaches a committing path that is not a %s implementation — widen the "+
			"header's residual in the same commit: %v", commitPrimitiveName, class.bypasses)
	}
	// Non-vacuity: the set WAS classified (the honest funnel reads as
	// delegating), and the raw-argv committer is genuinely unseen rather
	// than merely unflagged — no property has a subject in that file at
	// all: it declares no CommitAll and calls no gitutil primitive.
	if class.delegating != 1 {
		t.Fatalf("the fixture set was not classified — expected the honest funnel to count as "+
			"delegating, got %d", class.delegating)
	}
	for _, sc := range scans {
		for _, c := range sc.qualifiedPrimitiveCalls {
			if strings.HasPrefix(c.file, "internal/complete/") {
				t.Errorf("unexpected primitive call site resolved in the raw-argv fixture: %+v", c)
			}
		}
		for _, d := range sc.primitiveDecls {
			if strings.HasPrefix(d.file, "internal/complete/") {
				t.Errorf("the raw-argv fixture must declare no %s: %+v", commitPrimitiveName, d.label)
			}
		}
	}
}

// TestCommitFunnel_ScanCatchesADelegatingCommitAllThatAlsoReachesGit
// proves property (3) no longer accepts an implementation the moment it
// mentions the funnel: a body that calls commitWithExport AND keeps a
// second, unguarded leg to git commits without the precondition whenever
// that second leg is the one taken.
func TestCommitFunnel_ScanCatchesADelegatingCommitAllThatAlsoReachesGit(t *testing.T) {
	const twoLegged = `package approve

import "os/exec"

type dualExecutor struct{ fast bool }

func (d *dualExecutor) CommitAll(path, msg string) error {
	if d.fast {
		return exec.Command("git", "-C", path, "commit", "-am", msg).Run()
	}
	return d.commitWithExport(path, msg)
}
`
	class := commitFunnelClassify(commitFunnelScanFixtures(t, map[string]string{
		"internal/executor/funnel.go": commitFunnelFixtureFunnel,
		"internal/approve/dual.go":    twoLegged,
	}))
	if _, flagged := class.bypasses["(*dualExecutor).CommitAll"]; !flagged {
		t.Fatalf("an implementation with a second, unguarded leg to git must be caught even though it "+
			"also calls the funnel; flagged: %v", class.bypasses)
	}
	if class.delegating != 1 {
		t.Errorf("a flagged implementation must not also be counted as honestly delegating, got %d", class.delegating)
	}
}

// TestCommitFunnel_ScanRejectsAPreconditionThatOnlyRunsInAClosure is the
// mutation proof for property (2)'s direct-span rule: a funnel whose
// checkNoPreservedMerge call sits in a closure it never invokes reads as a
// funnel with NO precondition, which property (2) reports — rather than as
// a guarded commit, which it is not.
func TestCommitFunnel_ScanRejectsAPreconditionThatOnlyRunsInAClosure(t *testing.T) {
	const src = `package scratch

func commitWithExport(path, msg string) error {
	guard := func() error {
		return checkNoPreservedMerge(path, "")
	}
	_ = guard
	return gitutil.CommitAll(path, msg)
}
`
	fset, file := commitFunnelParse(t, "closure_precondition.go", src)
	scan := scanCommitFunnelFile(fset, file, "closure_precondition.go", nil)

	if len(scan.funnelDecls) != 1 {
		t.Fatalf("expected one funnel declaration, got %d", len(scan.funnelDecls))
	}
	d := scan.funnelDecls[0]
	if d.preconditionPos != token.NoPos {
		t.Error("a precondition reachable only inside an uncalled closure must NOT count as the " +
			"funnel's precondition — the commit it is supposed to guard runs without it")
	}
	if d.primitivePos == token.NoPos {
		t.Fatal("the funnel's own direct commit must still resolve, or the failure would be reported " +
			"as a funnel that no longer commits rather than one that no longer checks")
	}
}

// TestCommitFunnel_ScanCatchesACallerOutsideTheFunnel is the mutation
// proof for property (1): a new committing path added anywhere in the
// tree, in the ordinary shape a future author would write it, is
// resolved and attributed to its own enclosing function — not silently
// credited to the funnel.
func TestCommitFunnel_ScanCatchesACallerOutsideTheFunnel(t *testing.T) {
	const src = `package scratch

func commitWithExport(path, msg string) error {
	if err := checkNoPreservedMerge(path, ""); err != nil {
		return err
	}
	return gitutil.CommitAll(path, msg)
}

func aNewCommittingPath(path, msg string) error {
	return gitutil.CommitAll(path, msg)
}
`
	fset, file := commitFunnelParse(t, "bypass.go", src)
	scan := scanCommitFunnelFile(fset, file, "bypass.go", nil)

	if len(scan.qualifiedPrimitiveCalls) != 2 {
		t.Fatalf("expected both gitutil.CommitAll calls to resolve, got %d", len(scan.qualifiedPrimitiveCalls))
	}
	var outside []string
	for _, c := range scan.qualifiedPrimitiveCalls {
		if c.enclosingName != commitFunnelFuncName {
			outside = append(outside, c.enclosing)
		}
	}
	if len(outside) != 1 || outside[0] != "aNewCommittingPath" {
		t.Fatalf("the bypassing caller must be attributed to its OWN function; got %v", outside)
	}
}

// TestCommitFunnel_ScanCatchesAClosureBypass proves the span walk does
// not credit a closure's commit to the enclosing function: a commit
// inside a callback may run at a time the enclosing function's own
// precondition check never covered.
func TestCommitFunnel_ScanCatchesAClosureBypass(t *testing.T) {
	const src = `package scratch

func withRetry(fn func() error) error { return fn() }

func looksFunnelled(path, msg string) error {
	if err := checkNoPreservedMerge(path, ""); err != nil {
		return err
	}
	return withRetry(func() error {
		return gitutil.CommitAll(path, msg)
	})
}
`
	fset, file := commitFunnelParse(t, "closure.go", src)
	scan := scanCommitFunnelFile(fset, file, "closure.go", nil)

	if len(scan.qualifiedPrimitiveCalls) != 1 {
		t.Fatalf("expected one gitutil.CommitAll call, got %d", len(scan.qualifiedPrimitiveCalls))
	}
	c := scan.qualifiedPrimitiveCalls[0]
	if c.enclosing == "looksFunnelled" || c.enclosingName != "" {
		t.Fatalf("a closure's commit must NOT be credited to its enclosing function; got label %q name %q",
			c.enclosing, c.enclosingName)
	}
	if !strings.HasPrefix(c.enclosing, "func literal at ") {
		t.Fatalf("expected the closure's own span label, got %q", c.enclosing)
	}
}

// TestCommitFunnel_ScanResolvesAnAliasedGitutilImport proves the scan
// keys on the file's OWN import alias, so a bypass written as
// `gu.CommitAll(...)` — ordinary Go, not a contrivance — is still caught.
func TestCommitFunnel_ScanResolvesAnAliasedGitutilImport(t *testing.T) {
	const src = `package scratch

import gu "github.com/mrmaxsteel/mindspec/internal/gitutil"

func anAliasedBypass(path, msg string) error {
	return gu.CommitAll(path, msg)
}
`
	fset, file := commitFunnelParse(t, "alias.go", src)
	scan := scanCommitFunnelFile(fset, file, "alias.go", nil)

	if len(scan.qualifiedPrimitiveCalls) != 1 {
		t.Fatalf("an aliased gitutil import must still resolve; got %d call sites", len(scan.qualifiedPrimitiveCalls))
	}
	if got := scan.qualifiedPrimitiveCalls[0].enclosing; got != "anAliasedBypass" {
		t.Fatalf("expected the aliased bypass attributed to anAliasedBypass, got %q", got)
	}
}

// TestCommitFunnel_ScanCatchesThePreconditionMovedAfterTheCommit is the
// mutation proof for property (2): reordering the funnel's own two calls
// leaves a precondition that refuses nothing, and the scan sees it.
func TestCommitFunnel_ScanCatchesThePreconditionMovedAfterTheCommit(t *testing.T) {
	const src = `package scratch

func commitWithExport(path, msg string) error {
	err := gitutil.CommitAll(path, msg)
	if cerr := checkNoPreservedMerge(path, ""); cerr != nil {
		return cerr
	}
	return err
}
`
	fset, file := commitFunnelParse(t, "reordered.go", src)
	scan := scanCommitFunnelFile(fset, file, "reordered.go", nil)

	if len(scan.funnelDecls) != 1 {
		t.Fatalf("expected one funnel declaration, got %d", len(scan.funnelDecls))
	}
	d := scan.funnelDecls[0]
	if d.preconditionPos == token.NoPos || d.primitivePos == token.NoPos {
		t.Fatalf("both calls must resolve; precondition=%v primitive=%v", d.preconditionPos, d.primitivePos)
	}
	if d.preconditionPos <= d.primitivePos {
		t.Fatal("the reordered funnel must be detected: the precondition follows the commit")
	}
}

// TestCommitFunnel_ScanCatchesANonDelegatingCommitAllImplementation is
// the mutation proof for property (3): a second Executor implementation
// whose CommitAll reaches git directly — the shape that would make the
// external internal/approve, internal/complete and internal/spec call
// sites bypass the precondition — is derived as non-delegating and
// git-capable.
func TestCommitFunnel_ScanCatchesANonDelegatingCommitAllImplementation(t *testing.T) {
	const src = `package scratch

type fastExecutor struct{}

func (f *fastExecutor) CommitAll(path, msg string) error {
	return gitutil.CommitAll(path, msg)
}

type recordingExecutor struct{ calls []string }

func (r *recordingExecutor) CommitAll(path, msg string) error {
	r.calls = append(r.calls, path)
	return nil
}
`
	fset, file := commitFunnelParse(t, "impls.go", src)
	scan := scanCommitFunnelFile(fset, file, "impls.go", nil)

	if len(scan.primitiveDecls) != 2 {
		t.Fatalf("expected both CommitAll implementations, got %d", len(scan.primitiveDecls))
	}
	byLabel := map[string]*commitFunnelDecl{}
	for _, d := range scan.primitiveDecls {
		byLabel[d.label] = d
	}
	fast, ok := byLabel["(*fastExecutor).CommitAll"]
	if !ok {
		t.Fatalf("missing the bypassing implementation; got %v", byLabel)
	}
	if fast.callsFunnel {
		t.Error("the bypassing implementation must NOT read as delegating")
	}
	if len(fast.gitutilCalls) == 0 {
		t.Error("the bypassing implementation must be derived as able to reach git")
	}
	rec, ok := byLabel["(*recordingExecutor).CommitAll"]
	if !ok {
		t.Fatalf("missing the recording implementation; got %v", byLabel)
	}
	if len(rec.gitutilCalls) != 0 || rec.callsOSExec {
		t.Error("a recording double must be derived as provably non-committing, not exempted by name")
	}
}

// TestCommitFunnel_ScanCatchesAnOSExecCommitAll proves property (3)'s
// second reach: a CommitAll that shells out to git directly, never
// naming the gitutil package, still fails closed.
func TestCommitFunnel_ScanCatchesAnOSExecCommitAll(t *testing.T) {
	const src = `package scratch

import "os/exec"

type shellExecutor struct{}

func (s *shellExecutor) CommitAll(path, msg string) error {
	return exec.Command("git", "-C", path, "commit", "-m", msg).Run()
}
`
	fset, file := commitFunnelParse(t, "shell.go", src)
	scan := scanCommitFunnelFile(fset, file, "shell.go", nil)

	if len(scan.primitiveDecls) != 1 {
		t.Fatalf("expected one CommitAll implementation, got %d", len(scan.primitiveDecls))
	}
	d := scan.primitiveDecls[0]
	if d.callsFunnel {
		t.Error("the shelling implementation must NOT read as delegating")
	}
	if !d.callsOSExec {
		t.Error("a CommitAll that shells out through os/exec must be derived as able to reach git")
	}
}

// TestCommitFunnel_ScanCatchesABareSamePackageCall is the mutation proof
// for property (4): inside package gitutil, CommitAll needs no qualifier,
// so a same-package caller would be invisible to the qualified-call
// resolution alone.
func TestCommitFunnel_ScanCatchesABareSamePackageCall(t *testing.T) {
	const src = `package gitutil

func SomeNewHelper(workdir, message string) error {
	return CommitAll(workdir, message)
}
`
	fset, file := commitFunnelParse(t, "internal/gitutil/scratch.go", src)
	scan := scanCommitFunnelFile(fset, file, "internal/gitutil/scratch.go", nil)

	if len(scan.barePrimitiveCalls) != 1 {
		t.Fatalf("expected the bare same-package call to resolve, got %d", len(scan.barePrimitiveCalls))
	}
	if got := scan.barePrimitiveCalls[0].enclosing; got != "SomeNewHelper" {
		t.Fatalf("expected the bare call attributed to SomeNewHelper, got %q", got)
	}
}

// TestCommitFunnel_ProductionWalkReachesTheKnownCallerPackages is the
// non-vacuity guard on the walk itself: a directory filter that silently
// stopped descending into the packages holding the external
// Executor.CommitAll call sites would leave every assertion above green
// over a tree it never read. Derived from the walk's own output — it
// asserts the packages were REACHED, never how many call sites they hold.
func TestCommitFunnel_ProductionWalkReachesTheKnownCallerPackages(t *testing.T) {
	root := mergeChokepointRepoRoot(t)
	_, _, names := commitFunnelProductionFiles(t, root)

	seen := map[string]bool{}
	for _, n := range names {
		seen[filepath.ToSlash(filepath.Dir(n))] = true
	}
	for _, dir := range []string{
		"internal/executor",
		"internal/gitutil",
		"internal/approve",
		"internal/complete",
		"internal/spec",
		"cmd/mindspec",
	} {
		if !seen[dir] {
			t.Errorf("the production walk never reached %s — every funnel assertion would be "+
				"vacuously green over the packages that call CommitAll", dir)
		}
	}
	if len(names) < len(seen) {
		t.Fatalf("walk accounting is inconsistent: %d files across %d directories", len(names), len(seen))
	}
}
