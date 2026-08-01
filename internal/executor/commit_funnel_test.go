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
//	    `gitutil.CommitAll`, both directly in its own span (never in a
//	    nested closure that may not run).
//	(3) EVERY production function or method named `CommitAll` either
//	    delegates to `commitWithExport`, or is provably non-committing at
//	    this scan's vocabulary (its body calls nothing in the `gitutil`
//	    package and never shells out through `os/exec`). Anything else
//	    REDS — a `CommitAll` implementation that can reach git without
//	    passing the funnel is exactly the bypass this scan exists to
//	    catch, and it is failed closed rather than judged.
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
// implementation either funnels or provably cannot commit. The
// MockExecutor's exemption is DERIVED from its body (it records and
// returns), never from a written allowlist.
//
// THIS IS NOT A UNIVERSAL CLAIM, on the same footing as the merge
// chokepoint scan's stated residuals. A dot-imported `gitutil`, a
// function-valued alias of `gitutil.CommitAll` stored in a var/field and
// called indirectly, reflection, and `os/exec` invocations that assemble
// `git commit` argv by hand somewhere other than a `CommitAll`
// implementation are none of them traced here. The first three are the
// same shapes merge_chokepoint_test.go documents as out of scope for a
// go/types-free ratchet, and the same precedent (R5(b)) governs; the
// fourth is bounded by property (3)'s fail-closed treatment of any
// `CommitAll` body that can reach `os/exec` at all.

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

// commitFunnelDecl is one production `func`/method declaration the scan
// cares about, with the reachability facts derived from its own body.
type commitFunnelDecl struct {
	file  string
	label string
	pos   token.Position
	// callsFunnel is true when the body calls commitWithExport.
	callsFunnel bool
	// gitutilCalls names every `<gitutil>.X` function the body calls, in
	// source order — the evidence that a body can reach git directly.
	gitutilCalls []string
	// callsOSExec is true when the body invokes os/exec, the other way a
	// body can reach git without naming the gitutil package.
	callsOSExec bool
	// preconditionPos / primitivePos are the positions of the funnel's
	// own precondition and commit calls, for the ordering check. Zero
	// when absent.
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
	// funnelDecls are declarations of commitWithExport itself.
	funnelDecls []commitFunnelDecl
	// primitiveDecls are declarations named CommitAll (the gitutil
	// definition, the executor method, and every other implementation).
	primitiveDecls []commitFunnelDecl
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
// one parsed production file.
func scanCommitFunnelFile(fset *token.FileSet, file *ast.File, name string) commitFunnelFileScan {
	gitutilName := gitutilLocalName(file)
	osExecName := commitFunnelImportLocalName(file, osExecImportPath, "exec")

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

		if fd.Name.Name != commitFunnelFuncName && fd.Name.Name != commitPrimitiveName {
			continue
		}
		d := commitFunnelDecl{file: name, label: commitFunnelDeclLabel(fd), pos: fset.Position(fd.Pos())}
		// The reachability facts are derived from the WHOLE body,
		// closures included: a body that can reach git from inside a
		// closure can still reach git.
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			pkg, sel := commitFunnelCalleeName(call)
			switch {
			case pkg == "" && sel == commitFunnelFuncName:
				d.callsFunnel = true
			case pkg == "" && sel == commitPreconditionName && d.preconditionPos == token.NoPos:
				d.preconditionPos = call.Pos()
			case pkg == gitutilName:
				d.gitutilCalls = append(d.gitutilCalls, sel)
				if sel == commitPrimitiveName && d.primitivePos == token.NoPos {
					d.primitivePos = call.Pos()
				}
			case pkg == osExecName && (sel == "Command" || sel == "CommandContext"):
				d.callsOSExec = true
			}
			return true
		})
		// A method call on a receiver (`g.commitWithExport(...)`) reads
		// as a qualified pair whose package half is the receiver name,
		// so recognise it by selector alone as well.
		if !d.callsFunnel {
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if _, sel := commitFunnelCalleeName(call); sel == commitFunnelFuncName {
					d.callsFunnel = true
				}
				return true
			})
		}
		if fd.Name.Name == commitFunnelFuncName {
			out.funnelDecls = append(out.funnelDecls, d)
		} else {
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

	var qualified, bare []commitFunnelCall
	var funnelDecls, primitiveDecls []commitFunnelDecl
	for i, file := range files {
		scan := scanCommitFunnelFile(fset, file, names[i])
		qualified = append(qualified, scan.qualifiedPrimitiveCalls...)
		funnelDecls = append(funnelDecls, scan.funnelDecls...)
		primitiveDecls = append(primitiveDecls, scan.primitiveDecls...)
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

	// Property (3): every CommitAll implementation funnels, or is
	// provably non-committing. This is what carries the external
	// Executor.CommitAll callers in internal/approve, internal/complete
	// and internal/spec without go/types.
	delegating := 0
	for _, d := range primitiveDecls {
		if d.callsFunnel {
			delegating++
			continue
		}
		// The gitutil definition itself is the primitive, not an
		// implementation of the interface — it is derived as such by
		// living in the package the funnel calls INTO.
		if strings.HasPrefix(filepath.ToSlash(d.file), "internal/gitutil/") {
			continue
		}
		if len(d.gitutilCalls) == 0 && !d.callsOSExec {
			// Provably non-committing at this scan's vocabulary: the body
			// cannot reach git at all. (This is how MockExecutor.CommitAll
			// passes — derived from its body, not from a written list.)
			continue
		}
		t.Errorf("%s (%s:%d) is a %s implementation that does NOT delegate to %s and CAN reach git "+
			"(gitutil calls: %v; os/exec: %v).\nEvery committing path must pass the funnel so the "+
			"R5(d)(v) precondition is discharged; failing closed rather than judging whether this "+
			"particular body happens to commit.",
			d.label, d.file, d.pos.Line, commitPrimitiveName, commitFunnelFuncName,
			d.gitutilCalls, d.callsOSExec)
	}
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
	scan := scanCommitFunnelFile(fset, file, "bypass.go")

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
	scan := scanCommitFunnelFile(fset, file, "closure.go")

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
	scan := scanCommitFunnelFile(fset, file, "alias.go")

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
	scan := scanCommitFunnelFile(fset, file, "reordered.go")

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
	scan := scanCommitFunnelFile(fset, file, "impls.go")

	if len(scan.primitiveDecls) != 2 {
		t.Fatalf("expected both CommitAll implementations, got %d", len(scan.primitiveDecls))
	}
	byLabel := map[string]commitFunnelDecl{}
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
	scan := scanCommitFunnelFile(fset, file, "shell.go")

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
	scan := scanCommitFunnelFile(fset, file, "internal/gitutil/scratch.go")

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
