package executor

// Spec 127 R5(d)(v) — the COMMIT-FUNNEL anti-drift scan, the sibling of
// merge_chokepoint_test.go's merge-producer scan.
//
// R5(d)(v)'s preserved-merge obligation binds "every product path that
// commits inside a worktree". The shipped design discharges that
// obligation NOT by repeating the check at each call site but at a single
// funnel: `commitWithExport` calls `checkNoPreservedMerge` once, then
// delegates to `gitutil.CommitAll`. The committing paths — this
// package's own internal `commitWithExport` calls and the external
// `Executor.CommitAll` callers in `internal/approve`, `internal/complete`
// and `internal/spec` — reach it through that one function. (No count
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
//	(1) Each `gitutil.CommitAll` call THIS SCAN RESOLVES in production code
//	    is lexically inside `commitWithExport` — among the calls it
//	    resolves, the funnel is the only production caller of the commit
//	    primitive.
//	(2) `commitWithExport` is declared exactly once, and inside it the
//	    precondition `checkNoPreservedMerge` RUNS BEFORE
//	    `gitutil.CommitAll`, both DIRECTLY IN ITS OWN SPAN — a call to
//	    either that sits inside a nested closure is not counted at all,
//	    because a closure's body may never run at the point the funnel
//	    commits. "Before" here is EXECUTION order, established by
//	    construction and not by source position: the precondition must be
//	    evaluated unconditionally, and WITHOUT ITS RESULT BEING DISCARDED,
//	    by a top-level statement of the funnel body that precedes the
//	    statement the commit is reached from. A `defer` runs at return, a
//	    `go` runs concurrently, and an `if`/`switch` branch or a loop body
//	    may not run at all — each puts the check textually above a commit
//	    it does not precede, and the position comparison this replaced
//	    called every one of them correctly ordered. A bare call statement
//	    or a blank-assigned result runs the check and throws its refusal
//	    away, which is ordering without guarding (see
//	    commitFunnelDeriveOrder). A `goto` written in the funnel body
//	    breaks the statement-list reading the rule depends on — it can
//	    enter that list past the check — and is REFUSED outright rather
//	    than analysed (commitFunnelJump). (2)'s OWN RESIDUAL, on the same
//	    footing as (3)'s below: it does not prove the precondition's error is
//	    PROPAGATED — an error bound to a real variable that nothing tests
//	    is dataflow, which this scan does not do. Pinned by
//	    `StatedLimit_APreconditionWhoseErrorIsNeverTestedIsNotSeen`.
//	(3) No production function or method named `CommitAll` REACHES git —
//	    the `gitutil` package, or a process start named in this scan's
//	    finite vocabulary (`commitFunnelSpawnRoutes`) — except through
//	    `commitWithExport`, ALONG A CHAIN OF EDGES THIS SCAN RESOLVES.
//	    That qualifier is the whole content of the residual list below:
//	    the property is decided over the graph the scan can build, not
//	    over the program. This is decided against a call graph derived
//	    from the WHOLE production tree, not from the one method body:
//	    every call chain this scan can resolve BY NAME is followed, to
//	    unbounded depth, with edges INTO the funnel cut (reaching git
//	    through the funnel is the sanctioned path, and property (2) is
//	    what guards it). An implementation that reaches git off the funnel
//	    REDS whether or not it also calls the funnel; one that reaches git
//	    nowhere passes. Both answers are DERIVED — the MockExecutor is
//	    exempt because the graph shows no path from it to git, not because
//	    a written allowlist says so.
//	(4) No production caller inside package `gitutil` itself reaches
//	    `CommitAll` by its bare, same-package name — among the calls this
//	    scan resolves — which would bypass the funnel without ever writing
//	    the `gitutil.` qualifier this scan keys on.
//
// Property (3) is what carries the EXTERNAL callers. This scan has no
// go/types, so it cannot prove that `exec.CommitAll(...)` in
// `internal/complete` dispatches to `*MindspecExecutor`. It does not need
// to: (3) ranges over the production `CommitAll` DECLARATIONS this walk
// parses, so whichever one an interface call lands on, that
// implementation either funnels or has no path to git along an edge this
// scan resolves.
//
// WHAT THIS SCAN IS, AND WHAT IT IS NOT — the claim, narrowed at the
// fifth confirm round to what the mechanism delivers. This is a RATCHET
// AGAINST ACCIDENTAL REGRESSION: it catches the shapes an ordinary
// refactor produces — a new caller, a renamed helper, a delegation grown
// one hop longer, a precondition moved or wrapped or deferred — and it
// holds those shapes in place for the next change. It is NOT a proof
// against deliberate evasion. It resolves callees by a FINITE, SYNTACTIC
// enumeration of the ways a call can be WRITTEN: a bare identifier; a
// selector whose receiver is an identifier the file's import block does
// not bind; a selector whose receiver is an expression; each of those with
// or without explicit generic type arguments; and a qualified call through
// a name that import block does bind to a package inside this module.
// Go's grammar and scope rules admit ways to name a call that this
// enumeration does not cover. Five consecutive adversarial rounds on this
// file each found another one — helper indirection, generic instantiation,
// a spawn route outside the vocabulary, pointer method expressions,
// non-identifier receivers, `defer`, `goto`, a receiver shadowing an
// import name — and the list below is where the ones found so far are
// written down, in the expectation that it is not finished.
//
// The stakes of a residual are bounded in the direction that matters: it
// means A FUTURE BYPASS MIGHT NOT BE CAUGHT, not that the shipped product
// is unguarded. The runtime path this file watches over is
// `commitWithExport`, whose refusal is pinned by
// `TestCommitAll_RefusesOverAPreservedMerge` and exercised throughout the
// executor suite. R5(d)(v)'s obligation over what this scan does not
// resolve rests on review, exactly as R5(b)'s precedent governs the
// sibling merge-chokepoint scan — this file narrows the enforced claim to
// what a syntactic scan can carry, and leaves the rest where it has always
// been rather than pretending the scan is holding it.
//
// WHAT "BY NAME" MEANS, AND WHAT EVADES IT. The graph's edges are exactly
// the callees this scan can name without types: a bare `foo(...)` resolves
// to the production declarations named `foo` in the CALLING package, and
// so does a selector call whose receiver is not an identifier the file's
// import block binds — `x.foo(...)`, the pointer method expression
// `(*T).foo(...)`, the nested receiver `x.y.foo(...)`, `xs[0].foo(...)`,
// `v.(*T).foo(...)` (unioned over receivers, which over-approximates in
// the fail-closed direction). A package qualifier is a bare identifier by
// Go's grammar, so a selector on an EXPRESSION is a method call rather
// than a cross-package one; treating one as unresolvable, which this scan
// did until its fourth adversarial round, dropped direct
// statically-named calls silently. The converse does NOT hold — a bare
// identifier receiver is not necessarily a package, because a local
// declaration may shadow an import name, which is the residual named
// below. `pkg.Foo(...)` resolves through the file's own import block
// whenever `pkg` names a package inside this module — file-scoped, which
// is exactly what that residual is about. Depth is unbounded:
// reachability is a monotone fixed point, so
// recursion and mutual recursion converge rather than truncating at a hop
// limit. What that leaves outside — the honest residual, on the same
// footing as the merge chokepoint scan's:
//
//   - FUNCTION VALUES: a call through a var, a struct field, a parameter,
//     or through reflection, where no callee name is written — pinned by
//     `StatedLimit_AFunctionValueCallIsNotSeen`;
//   - CROSS-PACKAGE RECEIVERS: a method call whose receiver's type is
//     declared in ANOTHER package, since a method name is only matched
//     inside the calling package — pinned by
//     `StatedLimit_CrossPackageReceiverHopIsNotTraced`;
//   - RECEIVERS THAT SHADOW AN IMPORT NAME: the import block is FILE
//     scoped and the question is not — a function-local declaration may
//     legally shadow an import, and then `fmt.doCommit(path, msg)` is an
//     ordinary same-package method call on a local variable that this scan
//     classifies as a call into package `fmt` and drops. Resolving it
//     needs lexical scope, which is the go/types-shaped work this ratchet
//     does not do — pinned by
//     `StatedLimit_AReceiverShadowingAnImportNameIsNotResolved`;
//   - DOT-IMPORTS of `gitutil`, which bind names this scan never sees —
//     pinned by `StatedLimit_ADotImportedGitutilIsNotSeen`;
//   - FOREIGN CALLBACKS: a func handed to a stdlib or third-party package
//     and invoked from there, since only this module's source is parsed —
//     pinned by `StatedLimit_AForeignCallbackIsNotSeen`;
//   - PROCESS STARTS OUTSIDE THE VOCABULARY: "reaches git" is the finite,
//     named set in `commitFunnelSpawnRoutes`, not a general effect
//     analysis, so a third-party exec wrapper — or a stdlib route nobody
//     has added to that table — is not a reach this scan can see — pinned
//     by `StatedLimit_AThirdPartyProcessRunnerIsNotSeen`;
//   - THE SUBJECT BOUNDARY, described at the end of this header — pinned
//     by `StatedLimit_ARawArgvCommitterOutsideCommitAllIsNotSeen`.
//
// (Named, not numbered, deliberately: an ordinal list cross-referenced by
// position rots the first time an entry is inserted, which is the same
// failure mode as the call-site enumeration this whole file replaced.)
// FUNCTION VALUES, DOT-IMPORTS and FOREIGN CALLBACKS are the shapes
// merge_chokepoint_test.go documents as out of scope for a go/types-free
// ratchet, and the same precedent (R5(b)) governs. CROSS-PACKAGE
// RECEIVERS, IMPORT-SHADOWED RECEIVERS, PROCESS STARTS OUTSIDE THE
// VOCABULARY and the subject boundary are this scan's own, stated here
// rather than left to be discovered. Each bullet above carries the
// `StatedLimit` fixture named beside it — no count is written, because
// this header once wrote one a round before the fixtures existed, when
// three of the then-six were pinned and the disclosure said all were — so
// that widening the scan without widening this list REDS. Property (2)'s
// dataflow residual, stated with (2) above, is likewise pinned
// (`StatedLimit_APreconditionWhoseErrorIsNeverTestedIsNotSeen`).
//
// AND ONE RESIDUAL THAT IS UNPINNED, NECESSARILY: the enumeration itself.
// A fixture can pin a shape somebody has thought of; nothing pins the
// shape nobody has written down yet, and the section above says plainly
// that more of them exist. Every named limit here has a fixture holding it
// in place; the FINITENESS of the naming has none, and saying so is the
// only honest way to close a list that five rounds have each extended.
//
// What IS traced — and was not before this property was rebuilt — is the
// ordinary refactor shape `CommitAll` → same-package helper → process
// start, at any depth, whether or not the helper call carries explicit
// generic type arguments, and whether the helper is reached by a bare
// name or through a receiver EXPRESSION. That shape is the one refactors
// actually produce, and holding it is what this file is for.
//
// AND THE VOCABULARY ITSELF. `commitFunnelSpawnRoutes` is the finite table
// property (3) means by "reaches git", and its coverage fixture is DERIVED
// from it, so a route added without a fixture cannot go unexercised. That
// derivation is one-directional and the asymmetry is worth naming: a
// derived assertion cannot detect the DELETION of the thing it derives
// from — delete `os.StartProcess` and its subtest disappears with it,
// silently. `TestCommitFunnel_SpawnRouteVocabularyDoesNotShrink` is the
// independent floor that closes the other direction.
//
// AND THE RESIDUAL THAT IS NOT ABOUT EDGES AT ALL — the biggest one, and
// the one an earlier version of this header covered up. This scan has
// exactly two subjects: `gitutil.CommitAll` CALL SITES, and DECLARATIONS
// NAMED `CommitAll`. A function that commits by assembling `git commit`
// argv by hand, is not named `CommitAll`, and is not reachable from any
// declaration that is, is invisible to all four properties — it is not a
// bypass this scan fails to prove safe, it is a path this scan never looks
// at. The previous wording claimed that shape
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

// commitFunnelSpawnRoute is one stdlib way a body can start a process —
// and therefore reach `git` — without ever naming the gitutil package.
type commitFunnelSpawnRoute struct {
	importPath string
	// fallback is the identifier an unaliased import binds, used when the
	// file carries no matching import line (this file's own fixture
	// snippets are syntax-only).
	fallback string
	funcs    []string
}

// commitFunnelSpawnRoutes is this scan's ENTIRE vocabulary for "reaches
// git without going through gitutil". It is finite and stdlib-only on
// purpose, and that is a limit, not an oversight: a body that starts a
// process through a THIRD-PARTY runner, or through a helper in a package
// this walk does not parse, names none of these and is not seen. Stated
// in the header's residuals rather than left implicit — the previous
// version of this scan hedged the same fact as "at this scan's
// vocabulary" and never said what the vocabulary was.
var commitFunnelSpawnRoutes = []commitFunnelSpawnRoute{
	{importPath: osExecImportPath, fallback: "exec", funcs: []string{"Command", "CommandContext"}},
	{importPath: "os", fallback: "os", funcs: []string{"StartProcess"}},
	{importPath: "syscall", fallback: "syscall", funcs: []string{"Exec", "ForkExec", "StartProcess"}},
}

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
	// spawnEvidence names the process-start route the body invokes
	// (`os/exec.Command`, `os.StartProcess`, …) — the other way a body can
	// reach git without naming the gitutil package. Empty when the body
	// starts no process this scan's vocabulary recognises.
	spawnEvidence string
	// callees are the production declarations this body calls that the
	// scan can resolve BY NAME (see the header's residual list for what
	// that excludes). Deduplicated, in first-written order.
	callees []commitFunnelFuncKey
	// preconditionPos / primitivePos are the positions of the funnel's
	// own precondition and commit calls — derived from the funnel's DIRECT
	// span only. Zero when absent. They answer PRESENCE ("the funnel still
	// checks", "the funnel still commits"), never ordering: see
	// preconditionGuards.
	preconditionPos token.Pos
	primitivePos    token.Pos
	// preconditionGuards is true when the precondition is EVALUATED
	// UNCONDITIONALLY by a top-level statement of the funnel body that runs
	// strictly before the statement the commit is reached from — execution
	// order, established by construction, not source order. guardGap names
	// the construct standing in the way when it is false. See
	// commitFunnelDeriveOrder for why the source positions above cannot
	// answer this.
	preconditionGuards bool
	guardGap           string
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
// mistaken for a same-package method named Foo. It is FILE scoped, and
// lexical scope is not: a function-local declaration may legally shadow
// one of these names, and a same-package method call on it is then read as
// package-qualified and dropped. That is a DISCLOSED residual of this scan
// rather than an oversight of this function — see the header, and
// TestCommitFunnel_StatedLimit_AReceiverShadowingAnImportNameIsNotResolved.
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

// commitFunnelDirectCalls visits every call in node that belongs to node's
// OWN span: descent stops at a nested *ast.FuncLit, whose body may run
// later, elsewhere, or never at all.
func commitFunnelDirectCalls(node ast.Node, visit func(*ast.CallExpr)) {
	ast.Inspect(node, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CallExpr:
			visit(v)
		}
		return true
	})
}

// commitFunnelUnconditionalExprCalls visits the calls inside e that are
// evaluated whenever e itself is evaluated. It stops at a func literal (a
// body that runs later, if ever) and at the RIGHT operand of `&&` / `||`,
// which Go short-circuits — `cond && check()` writes the check above the
// commit and, whenever cond is false, never runs it.
func commitFunnelUnconditionalExprCalls(e ast.Expr, visit func(*ast.CallExpr)) {
	if e == nil {
		return
	}
	ast.Inspect(e, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BinaryExpr:
			if v.Op == token.LAND || v.Op == token.LOR {
				commitFunnelUnconditionalExprCalls(v.X, visit)
				return false
			}
		case *ast.CallExpr:
			visit(v)
		}
		return true
	})
}

// commitFunnelUnconditionalStmtCalls visits the calls stmt evaluates
// UNCONDITIONALLY — every time control reaches stmt, on every path out of
// it. What it leaves out is the whole point:
//
//   - `defer` and `go`: the call runs at return, or concurrently — after
//     the commit, or with no ordering against it at all;
//   - an `if` / `switch` / `select` BODY: the branch that skips it still
//     commits (only an `if`'s init statement and condition, and a switch's
//     init statement and tag, run unconditionally);
//   - a `for` / `range` BODY: it may run zero times (only the loop's init
//     statement and the range operand run unconditionally);
//   - a func literal's body: it may never be invoked.
//
// A bare block and a labelled statement are transparent — reaching them
// runs their contents — so this recurses through both.
func commitFunnelUnconditionalStmtCalls(stmt ast.Stmt, visit func(*ast.CallExpr)) {
	switch s := stmt.(type) {
	case *ast.ExprStmt:
		commitFunnelUnconditionalExprCalls(s.X, visit)
	case *ast.AssignStmt:
		for _, e := range s.Lhs {
			commitFunnelUnconditionalExprCalls(e, visit)
		}
		for _, e := range s.Rhs {
			commitFunnelUnconditionalExprCalls(e, visit)
		}
	case *ast.ReturnStmt:
		for _, e := range s.Results {
			commitFunnelUnconditionalExprCalls(e, visit)
		}
	case *ast.IncDecStmt:
		commitFunnelUnconditionalExprCalls(s.X, visit)
	case *ast.SendStmt:
		commitFunnelUnconditionalExprCalls(s.Chan, visit)
		commitFunnelUnconditionalExprCalls(s.Value, visit)
	case *ast.DeclStmt:
		gd, ok := s.Decl.(*ast.GenDecl)
		if !ok {
			return
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, e := range vs.Values {
				commitFunnelUnconditionalExprCalls(e, visit)
			}
		}
	case *ast.IfStmt:
		commitFunnelUnconditionalStmtCalls(s.Init, visit)
		commitFunnelUnconditionalExprCalls(s.Cond, visit)
	case *ast.SwitchStmt:
		commitFunnelUnconditionalStmtCalls(s.Init, visit)
		commitFunnelUnconditionalExprCalls(s.Tag, visit)
	case *ast.TypeSwitchStmt:
		commitFunnelUnconditionalStmtCalls(s.Init, visit)
		commitFunnelUnconditionalStmtCalls(s.Assign, visit)
	case *ast.ForStmt:
		commitFunnelUnconditionalStmtCalls(s.Init, visit)
	case *ast.RangeStmt:
		commitFunnelUnconditionalExprCalls(s.X, visit)
	case *ast.LabeledStmt:
		commitFunnelUnconditionalStmtCalls(s.Stmt, visit)
	case *ast.BlockStmt:
		for _, inner := range s.List {
			commitFunnelUnconditionalStmtCalls(inner, visit)
		}
	}
}

// commitFunnelDiscardedResults collects the calls in stmt whose RESULT is
// thrown away: a bare call statement, or an assignment landing entirely in
// blank identifiers. Such a call RUNS — so it satisfies any ordering rule,
// including the dominance rule below — and REFUSES NOTHING, because the
// error it returns is never seen and the commit proceeds regardless.
//
// This is the fifth shape of the same defect and it was found by auditing
// this file's own new ordering rule rather than by a reviewer: an ordering
// guarantee is not a guarding guarantee. `_ = checkNoPreservedMerge(…)`
// evades `errcheck` too (its check-blank option is off by default), so
// without this the whole static gate set would go green over it.
func commitFunnelDiscardedResults(stmt ast.Stmt, into map[*ast.CallExpr]bool) {
	ast.Inspect(stmt, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ExprStmt:
			if call, ok := s.X.(*ast.CallExpr); ok {
				into[call] = true
			}
		case *ast.AssignStmt:
			if len(s.Rhs) != 1 {
				return true
			}
			call, ok := s.Rhs[0].(*ast.CallExpr)
			if !ok {
				return true
			}
			for _, lhs := range s.Lhs {
				id, ok := lhs.(*ast.Ident)
				if !ok || id.Name != "_" {
					return true
				}
			}
			into[call] = true
		}
		return true
	})
}

// commitFunnelObstruction names the construct holding a precondition call
// that IS written in the funnel but is not evaluated on the way to the
// commit, so the failure tells the author what to change rather than only
// that something is wrong. A `defer` or `go` anywhere inside the statement
// is named ahead of the statement's own shape, because it is the specific
// thing that inverts text order and execution order.
func commitFunnelObstruction(stmt ast.Stmt, isPrecondition func(*ast.CallExpr) bool) string {
	holds := func(n ast.Node) bool {
		found := false
		commitFunnelDirectCalls(n, func(c *ast.CallExpr) {
			if isPrecondition(c) {
				found = true
			}
		})
		return found
	}
	async := ""
	ast.Inspect(stmt, func(n ast.Node) bool {
		if async != "" {
			return false
		}
		switch n.(type) {
		case *ast.DeferStmt:
			if holds(n) {
				async = "a `defer` statement, which runs when the funnel RETURNS — after the commit"
			}
		case *ast.GoStmt:
			if holds(n) {
				async = "a `go` statement, which runs concurrently, with no ordering against the commit"
			}
		}
		return true
	})
	if async != "" {
		return async
	}
	switch stmt.(type) {
	case *ast.IfStmt:
		return "a conditional branch, so the path that skips it still commits"
	case *ast.ForStmt, *ast.RangeStmt:
		return "a loop body, which may run zero times"
	case *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
		return "a case body, so the cases that skip it still commit"
	}
	return "a construct that is not evaluated on every path to the commit"
}

// commitFunnelJump names a NON-STRUCTURED JUMP written directly in the
// funnel's own span. The dominance rule below reads the funnel body as a
// list of top-level statements entered in order; a `goto` breaks exactly
// that reading, because it can enter the list PAST the statement holding
// the precondition and land on a label the commit is reached from. The
// check is then written above the commit, evaluated unconditionally by the
// rule's reckoning, and skipped at run time — the same position-versus-
// execution gap `defer` and `if` opened, arriving through a different
// construct.
//
// The response is refusal, not analysis: a funnel body containing a `goto`
// anywhere in its direct span cannot have dominance established BY
// CONSTRUCTION, whatever the jump's target, so this reds rather than
// attempting to decide which labels are reachable from where. That is
// fail-closed and it costs nothing real — the production funnel has no
// `goto`, and a funnel that grows one should be read by a person.
//
// `break` and `continue` need no such treatment, and the reason is worth
// writing down rather than leaving to be re-derived: their targets are the
// enclosing `for`, `switch` or `select` statement (a label on `break` must
// name one of those three, never a plain block), and this scan already
// declines to count anything inside those bodies as unconditionally
// evaluated. There is no statement they can skip that the dominance rule
// was counting on. A jump inside a nested closure is likewise not this
// funnel's control flow — Go has no cross-function `goto` — so the walk
// stops at a func literal.
func commitFunnelJump(body *ast.BlockStmt) string {
	found := ""
	ast.Inspect(body, func(n ast.Node) bool {
		if found != "" {
			return false
		}
		switch v := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BranchStmt:
			if v.Tok == token.GOTO {
				found = fmt.Sprintf("a `goto` written in the funnel body, which can jump PAST the statement "+
					"holding %s to a label the commit is reached from — so no ordering of top-level "+
					"statements establishes dominance and this scan refuses to guess which labels are "+
					"reachable", commitPreconditionName)
			}
		}
		return true
	})
	return found
}

// commitFunnelDeriveOrder decides property (2) over the funnel's own body:
// not merely WHERE the precondition and the commit are written, but whether
// the precondition RUNS on every path that reaches the commit, before the
// commit.
//
// Source position cannot answer that, and the gap is not theoretical. The
// fourth adversarial round on this file defeated the position-only check
// this replaces with `defer checkNoPreservedMerge(...)` written ABOVE the
// commit — textually first, temporally last — and again with the check
// wrapped in `if msg != ""`, textually first and, for an empty message,
// never. Both compiled, both left the scan green, and both broke the real
// behaviour: the property claimed temporal ordering and measured textual
// position, which is this artifact's own signature defect, a claim
// outrunning its mechanism.
//
// The rule enforced instead is DOMINANCE BY CONSTRUCTION: the precondition
// must be evaluated unconditionally, WITHOUT ITS RESULT BEING DISCARDED, by
// a top-level statement of the funnel body that comes strictly before the
// top-level statement the commit is reached from. That is STRICTER than
// execution order — a check and a commit in the same `if` branch is
// genuinely ordered and reds here anyway — and strict in the fail-closed
// direction: it accepts only the shapes where "before" is guaranteed by the
// language rather than by the reader's reasoning about which branch runs.
//
// WHAT THIS STILL DOES NOT PROVE, stated rather than left to be found: that
// the precondition's error is PROPAGATED. `err := checkNoPreservedMerge(…)`
// followed by an `err` that is never tested runs the check, keeps its
// result, and commits anyway — a dataflow question this scan does not
// answer. It is a residual of property (2), on the same footing as property
// (3)'s, and it is pinned by
// TestCommitFunnel_StatedLimit_APreconditionWhoseErrorIsNeverTestedIsNotSeen.
func commitFunnelDeriveOrder(d *commitFunnelDecl, body *ast.BlockStmt, gitutilName string) {
	isPrecondition := func(c *ast.CallExpr) bool {
		pkg, sel := commitFunnelCalleeName(c)
		return pkg == "" && sel == commitPreconditionName
	}
	isPrimitive := func(c *ast.CallExpr) bool {
		pkg, sel := commitFunnelCalleeName(c)
		return pkg == gitutilName && sel == commitPrimitiveName
	}

	// PRESENCE first, over the funnel's whole DIRECT span — a nested
	// closure is excluded, since its body may never run. This is what
	// separates "the funnel no longer checks" and "the funnel no longer
	// commits", each reported on its own terms, from "the check no longer
	// guards the commit".
	commitFunnelDirectCalls(body, func(c *ast.CallExpr) {
		if d.preconditionPos == token.NoPos && isPrecondition(c) {
			d.preconditionPos = c.Pos()
		}
		if d.primitivePos == token.NoPos && isPrimitive(c) {
			d.primitivePos = c.Pos()
		}
	})
	if d.preconditionPos == token.NoPos || d.primitivePos == token.NoPos {
		return
	}

	// A non-structured jump defeats the statement-list reading the rule
	// below depends on, so it is answered before that reading is attempted
	// rather than inside it.
	if jump := commitFunnelJump(body); jump != "" {
		d.guardGap = jump
		return
	}

	guard, commitAt, obstruction := -1, -1, ""
	for i, stmt := range body.List {
		if guard < 0 {
			discarded := map[*ast.CallExpr]bool{}
			commitFunnelDiscardedResults(stmt, discarded)
			sawDiscarded := false
			commitFunnelUnconditionalStmtCalls(stmt, func(c *ast.CallExpr) {
				switch {
				case !isPrecondition(c):
				case discarded[c]:
					sawDiscarded = true
				default:
					guard = i
				}
			})
			if guard < 0 && obstruction == "" {
				if sawDiscarded {
					obstruction = fmt.Sprintf("a statement that DISCARDS its result, so the refusal %s "+
						"returns never reaches the caller and the commit runs anyway",
						commitPreconditionName)
				} else {
					commitFunnelDirectCalls(stmt, func(c *ast.CallExpr) {
						if isPrecondition(c) {
							obstruction = commitFunnelObstruction(stmt, isPrecondition)
						}
					})
				}
			}
		}
		if commitAt < 0 {
			commitFunnelDirectCalls(stmt, func(c *ast.CallExpr) {
				if isPrimitive(c) {
					commitAt = i
				}
			})
		}
	}

	switch {
	case commitAt < 0:
		d.guardGap = "the commit resolves in the funnel's span but not in any top-level statement of its body"
	case guard < 0:
		if obstruction == "" {
			obstruction = "a construct that is not evaluated on every path to the commit"
		}
		d.guardGap = fmt.Sprintf("its only call to %s sits in %s, so the commit reached from statement %d runs unguarded",
			commitPreconditionName, obstruction, commitAt+1)
	case guard >= commitAt:
		d.guardGap = fmt.Sprintf("the commit is reached from statement %d of the funnel body, while %s is not evaluated until statement %d",
			commitAt+1, commitPreconditionName, guard+1)
	default:
		d.preconditionGuards = true
	}
}

// commitFunnelReceiverExpr is the pseudo-qualifier commitFunnelCalleeName
// returns for a selector whose receiver is an EXPRESSION rather than a bare
// identifier. A Go package qualifier is always a single identifier, so such
// a call is certainly a method call or method expression, and its NAME is
// resolvable inside the calling package exactly like `x.m(...)`. The
// sentinel routes it there while keeping it out of the two buckets that key
// on a real package name — the qualified-`gitutil` bucket and the bare
// same-package bucket — so `a.b.CommitAll(...)` is never mistaken for
// either. It cannot collide with a real import identifier: it is not a
// valid Go identifier at all.
const commitFunnelReceiverExpr = "<receiver-expression>"

// commitFunnelCalleeName renders a call's callee as either a bare
// identifier name (ident, "") or a qualified pair (pkg, sel). Parenthesized
// callees are unwrapped so `(gitutil.CommitAll)(...)` resolves identically
// to `gitutil.CommitAll(...)`.
func commitFunnelCalleeName(call *ast.CallExpr) (pkg, sel string) {
	fun := call.Fun
	for {
		switch v := fun.(type) {
		case *ast.ParenExpr:
			fun = v.X
			continue
		// An EXPLICITLY INSTANTIATED generic callee — `commitG[string](…)`
		// or `commitG[K, V](…)` — is an ordinary named call wearing type
		// arguments. Unwrapping to the underlying name resolves it exactly
		// as the inferred form `commitG(…)` already resolved, so whether a
		// bypass writes its type arguments cannot decide whether it is
		// seen. (Found by auditing this scan against shapes a hostile
		// reader would reach for, after the rebuild.)
		case *ast.IndexExpr:
			fun = v.X
			continue
		case *ast.IndexListExpr:
			fun = v.X
			continue
		}
		break
	}
	switch v := fun.(type) {
	case *ast.Ident:
		return "", v.Name
	case *ast.SelectorExpr:
		if x, ok := v.X.(*ast.Ident); ok {
			return x.Name, v.Sel.Name
		}
		// The receiver is an expression: a pointer method expression
		// `(*altExecutor).doCommit(a, path, msg)`, a nested receiver
		// `g.helper.doCommit(path, msg)`, an element `xs[0].doCommit(...)`,
		// a type assertion `v.(*T).doCommit(...)`. Every one of these is a
		// DIRECT, statically-named call whose method name resolves in the
		// calling package — none of them is the function-value or
		// reflection form the header's residual list discloses. Returning
		// ("", "") here, as this function did until the fourth adversarial
		// round found it three ways over, dropped the edge silently, so a
		// `CommitAll` reaching git through such a call read as unable to
		// reach git at all.
		return commitFunnelReceiverExpr, v.Sel.Name
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
	// Each spawn route resolved through the file's OWN name for it, so an
	// aliased `import osexec "os/exec"` is not a way past this.
	spawnFuncs := map[string]map[string]string{}
	for _, r := range commitFunnelSpawnRoutes {
		local := commitFunnelImportLocalName(file, r.importPath, r.fallback)
		if spawnFuncs[local] == nil {
			spawnFuncs[local] = map[string]string{}
		}
		for _, fn := range r.funcs {
			spawnFuncs[local][fn] = fmt.Sprintf("%s.%s", r.importPath, fn)
		}
	}
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
		// Not a package qualifier at all — a method call or method
		// expression, whether the receiver was written as an identifier
		// (`g.doCommit(...)`) or as an expression
		// (`commitFunnelReceiverExpr`: `(*T).doCommit(...)`,
		// `g.helper.doCommit(...)`). Without go/types the receiver's type
		// is unknown, so this resolves by method NAME within the calling
		// package, unioned over every receiver that declares it (see the
		// header's residuals for the cross-package receiver this misses).
		//
		// Reaching here means the qualifier matched NO import binding. The
		// two branches above decide the other way on the file's import
		// table alone, which is FILE scoped: a function-local declaration
		// shadowing an import name is routed there instead of here and its
		// edge is dropped. Disclosed in the header and pinned by
		// TestCommitFunnel_StatedLimit_AReceiverShadowingAnImportNameIsNotResolved.
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
			default:
				if ev := spawnFuncs[pkg][sel]; ev != "" && d.spawnEvidence == "" {
					d.spawnEvidence = ev
				}
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
			// only: a precondition that runs only inside a closure —
			// which may never run — must not read as a precondition the
			// commit is guarded by. Failing to find either call is
			// failing closed: property (2) then reports the funnel as no
			// longer checking, or no longer committing. And where BOTH
			// resolve, the ordering answer is derived from statement
			// dominance rather than from source position — see
			// commitFunnelDeriveOrder.
			commitFunnelDeriveOrder(d, fd.Body, gitutilName)
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
	if d.spawnEvidence != "" {
		return d.spawnEvidence
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
		!funnel.preconditionGuards {
		t.Errorf("%s (%s:%d) writes %s at offset %d and gitutil.%s at offset %d, but the precondition does "+
			"not GUARD the commit: %s.\n"+
			"Source order is not execution order. A `defer` runs at return, a `go` runs concurrently, and an "+
			"`if`/`switch` branch or a loop body may not run at all — every one of them puts the check "+
			"textually above a commit it does not precede. A precondition that does not run before the commit "+
			"refuses nothing, producing the two-parent chore: commit spec 125 shipped to fix.",
			commitFunnelFuncName, funnel.file, funnel.pos.Line, commitPreconditionName,
			funnel.preconditionPos, commitPrimitiveName, funnel.primitivePos, funnel.guardGap)
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
	if !strings.Contains(joined, "doCommit") || !strings.HasSuffix(joined, osExecImportPath+".Command") {
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

// TestCommitFunnel_ScanResolvesAnExplicitlyInstantiatedGenericCallee and
// TestCommitFunnel_ScanCatchesANonExecProcessStart are the two shapes a
// post-rebuild audit of this file against a hostile reader turned up —
// both were live evasions of the rebuilt property (3), both are now
// closed, and both are pinned here so they stay closed.
//
// The first: whether a bypass writes its type arguments cannot be allowed
// to decide whether the scan sees it. `commitG(path, msg)` resolved; the
// identical `commitG[string](path, msg)` did not, because the callee is
// an IndexExpr rather than an Ident.
func TestCommitFunnel_ScanResolvesAnExplicitlyInstantiatedGenericCallee(t *testing.T) {
	const generic = `package complete

import "os/exec"

type genExecutor struct{}

func (g *genExecutor) CommitAll(path, msg string) error {
	return commitG[string](path, msg)
}

func commitG[T any](path, msg string) error {
	return exec.Command("git", "-C", path, "commit", "-am", msg).Run()
}
`
	class := commitFunnelClassify(commitFunnelScanFixtures(t, map[string]string{
		"internal/executor/funnel.go": commitFunnelFixtureFunnel,
		"internal/complete/gen.go":    generic,
	}))
	witness, flagged := class.bypasses["(*genExecutor).CommitAll"]
	if !flagged {
		t.Fatalf("an explicitly instantiated generic callee must resolve like any other named call; "+
			"flagged: %v", class.bypasses)
	}
	if joined := strings.Join(witness, " → "); !strings.Contains(joined, "commitG") {
		t.Errorf("the witness must name the generic helper it followed, got %q", joined)
	}
}

// TestCommitFunnel_ScanResolvesACalleeWhoseReceiverIsAnExpression pins the
// third evasion of the rebuilt property (3), found three independent ways
// in the fourth adversarial round. `commitFunnelCalleeName` accepted a
// selector only when its receiver was a bare `*ast.Ident`, so two ORDINARY,
// statically-named calls were dropped from the graph without a trace:
//
//   - a pointer method expression, `(*altExecutor).doCommit(a, path, msg)`,
//     whose receiver is a parenthesized pointer TYPE;
//   - a nested receiver, `g.helper.doCommit(path, msg)`, whose receiver is
//     itself a selector.
//
// Neither is the function-value or reflection form the header's residual
// list discloses, and in both the callee name resolves in the calling
// package — so a `CommitAll` reaching git through one of them read as
// unable to reach git at all, while still nominally delegating. The
// control in each case is the same fixture with the helper's git call
// removed: the red must come from the helper's REACH, not from the shape
// of the call.
func TestCommitFunnel_ScanResolvesACalleeWhoseReceiverIsAnExpression(t *testing.T) {
	for _, tc := range []struct {
		name string
		call string
	}{
		{name: "pointer-method-expression", call: `(*altExecutor).doCommit(a, path, msg)`},
		{name: "nested-receiver", call: `a.helper.doCommit(path, msg)`},
		{name: "element-receiver", call: `a.helpers[0].doCommit(path, msg)`},
		{name: "type-asserted-receiver", call: `a.any.(*altExecutor).doCommit(path, msg)`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := func(helper string) string {
				return fmt.Sprintf(`package complete

import "os/exec"

type altExecutor struct {
	helper  *altExecutor
	helpers []*altExecutor
	any     interface{}
}

func (a *altExecutor) CommitAll(path, msg string) error {
	return %s
}

func (a *altExecutor) doCommit(path, msg string) error {
%s
}
`, tc.call, helper)
			}
			committing := body("\treturn exec.Command(\"git\", \"-C\", path, \"commit\", \"-am\", msg).Run()")
			inert := body("\t_ = path\n\t_ = msg\n\treturn nil")

			class := commitFunnelClassify(commitFunnelScanFixtures(t, map[string]string{
				"internal/executor/funnel.go": commitFunnelFixtureFunnel,
				"internal/complete/alt.go":    committing,
			}))
			witness, flagged := class.bypasses["(*altExecutor).CommitAll"]
			if !flagged {
				t.Fatalf("a direct, statically-named call whose receiver is an expression must resolve like "+
					"any other named call; flagged: %v", class.bypasses)
			}
			if joined := strings.Join(witness, " → "); !strings.Contains(joined, "doCommit") ||
				!strings.HasSuffix(joined, osExecImportPath+".Command") {
				t.Errorf("the witness must carry the chain that reaches git, got %q", joined)
			}

			control := commitFunnelClassify(commitFunnelScanFixtures(t, map[string]string{
				"internal/executor/funnel.go": commitFunnelFixtureFunnel,
				"internal/complete/alt.go":    inert,
			}))
			if len(control.bypasses) != 0 {
				t.Errorf("the control — the same call shape to a helper that cannot reach git — must NOT be "+
					"flagged, or the red above proves only that a selector was written: %v", control.bypasses)
			}
		})
	}
}

// The second: property (3)'s reach vocabulary was exactly `os/exec`, so a
// body that starts `git` through `syscall.Exec` or `os.StartProcess` —
// stdlib, no third-party anything — read as unable to reach git at all.
// The vocabulary is now the finite set in commitFunnelSpawnRoutes, and
// this test pins every route in it by DERIVING the fixture from that
// table, so a route added there without a fixture cannot go unexercised.
func TestCommitFunnel_ScanCatchesANonExecProcessStart(t *testing.T) {
	// Non-vacuity for the DERIVATION itself. This test's fixtures come
	// from the route table, so an emptied table produces zero subtests and
	// a silent green — a derived assertion cannot notice the deletion of
	// the thing it derives from. That is the boundary of the "derive
	// counts, never write them" rule this file otherwise follows, and
	// TestCommitFunnel_SpawnRouteVocabularyDoesNotShrink is the
	// independent sentinel that covers it. This guard is the local half:
	// zero subtests is a failure, not a pass.
	exercised := 0
	for _, route := range commitFunnelSpawnRoutes {
		for _, fn := range route.funcs {
			exercised++
			t.Run(route.importPath+"."+fn, func(t *testing.T) {
				src := fmt.Sprintf(`package complete

import %q

type spawnExecutor struct{}

func (s *spawnExecutor) CommitAll(path, msg string) error {
	return spawnGit(path, msg)
}

func spawnGit(path, msg string) error {
	_, _ = %s.%s()
	return nil
}
`, route.importPath, route.fallback, fn)
				class := commitFunnelClassify(commitFunnelScanFixtures(t, map[string]string{
					"internal/executor/funnel.go": commitFunnelFixtureFunnel,
					"internal/complete/spawn.go":  src,
				}))
				witness, flagged := class.bypasses["(*spawnExecutor).CommitAll"]
				if !flagged {
					t.Fatalf("a CommitAll reaching a process start through %s.%s must be caught; flagged: %v",
						route.importPath, fn, class.bypasses)
				}
				want := route.importPath + "." + fn
				if joined := strings.Join(witness, " → "); !strings.HasSuffix(joined, want) {
					t.Errorf("the witness must name the route it found; got %q, want it to end in %q", joined, want)
				}
			})
		}
	}
	if exercised == 0 {
		t.Fatal("commitFunnelSpawnRoutes is empty, so this test ran zero subtests and proved nothing — " +
			"the scan's entire process-start vocabulary would be gone with every assertion here still green")
	}
}

// TestCommitFunnel_SpawnRouteVocabularyDoesNotShrink is the sentinel the
// derived fixture above cannot be. DERIVING the coverage table protects
// against a route ADDED without a fixture; it cannot protect against a
// route DELETED — remove `os.StartProcess` and its subtest disappears with
// it, silently, leaving the disclosed vocabulary unpinned while every
// assertion above stays green.
//
// So this list is WRITTEN, deliberately, and it is the one written list in
// this file. That is not a relapse into the enumeration this whole artifact
// replaced, and the difference is what makes it safe: it does not claim to
// be complete, and nothing derives from it. It is a FLOOR — these routes
// are disclosed in this file's header and in R5(d)(v), so they must stay in
// the table. A route added above the floor is still covered, by the derived
// fixture; a route removed from under it REDS here, by name. The failure
// mode of a written enumeration is silent drift as the truth grows past it,
// and a floor has no such failure mode, because growth is exactly what it
// does not assert.
func TestCommitFunnel_SpawnRouteVocabularyDoesNotShrink(t *testing.T) {
	sentinel := []string{
		osExecImportPath + ".Command",
		osExecImportPath + ".CommandContext",
		"os.StartProcess",
		"syscall.Exec",
		"syscall.ForkExec",
		"syscall.StartProcess",
	}
	have := map[string]bool{}
	for _, route := range commitFunnelSpawnRoutes {
		for _, fn := range route.funcs {
			have[route.importPath+"."+fn] = true
		}
	}
	live := make([]string, 0, len(have))
	for r := range have {
		live = append(live, r)
	}
	sort.Strings(live)
	for _, want := range sentinel {
		if have[want] {
			continue
		}
		t.Errorf("%s is disclosed as part of this scan's process-start vocabulary — in this file's header "+
			"and in spec 127 R5(d)(v) — but is no longer in commitFunnelSpawnRoutes. Deleting a route "+
			"silently deletes its coverage fixture too, because that fixture is DERIVED from this table. "+
			"If the route is genuinely being dropped, drop it from the disclosure in the same commit; "+
			"otherwise restore it. Live vocabulary: %v", want, live)
	}
}

// TestCommitFunnel_StatedLimit_AThirdPartyProcessRunnerIsNotSeen pins the
// residual the route table above leaves standing: the vocabulary is
// stdlib-only, so a body that starts git through a third-party runner
// names none of it. Disclosed in the header; pinned here so widening the
// table without widening the disclosure REDS.
func TestCommitFunnel_StatedLimit_AThirdPartyProcessRunnerIsNotSeen(t *testing.T) {
	const thirdParty = `package complete

import "github.com/example/runner"

type vendoredExecutor struct{}

func (v *vendoredExecutor) CommitAll(path, msg string) error {
	return runner.Run("git", "-C", path, "commit", "-am", msg)
}
`
	class := commitFunnelClassify(commitFunnelScanFixtures(t, map[string]string{
		"internal/executor/funnel.go":   commitFunnelFixtureFunnel,
		"internal/complete/vendored.go": thirdParty,
	}))
	if _, flagged := class.bypasses["(*vendoredExecutor).CommitAll"]; flagged {
		t.Fatalf("the scan now sees a third-party process runner — widen the header's residual in the "+
			"same commit: %v", class.bypasses)
	}
	if class.delegating != 1 {
		t.Fatalf("the fixture set was not classified — expected the honest funnel to count as "+
			"delegating, got %d", class.delegating)
	}
	if _, seen := class.byLabel["(*vendoredExecutor).CommitAll"]; !seen {
		t.Fatal("the vendored implementation was never examined, so this test pins nothing")
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
// by name within the CALLING package, so the hop names a key no local
// declaration answers to and the edge is dropped. Contrast the same
// delegation with the helper in the same package, which IS caught — see
// TestCommitFunnel_ScanCatchesCommitAllDelegatingToACommittingHelper.
//
// The reason matters, and narrowed at the fourth adversarial round. This
// fixture writes `c.h.doCommit(…)`, a nested-receiver selector, which the
// scan used to drop for its SHAPE — before it ever reached the question of
// which package declares the callee. That shape now resolves
// (commitFunnelReceiverExpr; TestCommitFunnel_ScanResolvesACalleeWhoseRe-
// ceiverIsAnExpression), so this test pins what it always claimed to: the
// CROSS-PACKAGE boundary, and nothing else.
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

// TestCommitFunnel_StatedLimit_AReceiverShadowingAnImportNameIsNotResolved
// pins the OTHER half of the name-resolution boundary its sibling above
// pins — and it is a NARROWING, found at the fifth confirm round by two
// independent adversary slots (G1-N5/G3-N5) after the fourth had closed
// the receiver-EXPRESSION shape.
//
// The shape: a selector's receiver IS a bare identifier, so this scan asks
// the file's import block whether that identifier names a package. The
// import block is file-scoped and this question is not: Go lets a
// function-local declaration shadow an import name, and then
// `fmt.doCommit(path, msg)` is an ordinary same-package method call on a
// local variable while the import table still answers "that is package
// fmt". The edge is classified as a call into a package this walk either
// does not parse (a third-party or stdlib name) or parses under the wrong
// directory (an in-module one), and either way it is dropped.
//
// Resolving it needs lexical scope, which is the go/types-shaped work this
// ratchet does not do — so the boundary is DISCLOSED and pinned here
// instead, like the six beside it. The control in the same test is what
// makes that specific: the identical delegation through a local named `h`
// IS traced and IS flagged, so the miss is the shadowing, not the shape.
func TestCommitFunnel_StatedLimit_AReceiverShadowingAnImportNameIsNotResolved(t *testing.T) {
	// The local `fmt` shadows the file's own `fmt` import, which is used by
	// note() below, so the import is real rather than a parse-only prop.
	// Compiles and vets clean.
	const shadowed = `package complete

import (
	"fmt"
	"os/exec"
)

type shadowExecutor struct{}

type shadowHelper struct{}

func (h shadowHelper) doCommit(path, msg string) error {
	return exec.Command("git", "-C", path, "commit", "-am", msg).Run()
}

func (s *shadowExecutor) CommitAll(path, msg string) error {
	fmt := shadowHelper{}
	return fmt.doCommit(path, msg)
}

func note(p string) string { return fmt.Sprintf("note %s", p) }
`
	const unshadowed = `package complete

import (
	"fmt"
	"os/exec"
)

type shadowExecutor struct{}

type shadowHelper struct{}

func (h shadowHelper) doCommit(path, msg string) error {
	return exec.Command("git", "-C", path, "commit", "-am", msg).Run()
}

func (s *shadowExecutor) CommitAll(path, msg string) error {
	h := shadowHelper{}
	return h.doCommit(path, msg)
}

func note(p string) string { return fmt.Sprintf("note %s", p) }
`
	class := commitFunnelClassify(commitFunnelScanFixtures(t, map[string]string{
		"internal/executor/funnel.go": commitFunnelFixtureFunnel,
		"internal/complete/shadow.go": shadowed,
	}))
	if _, flagged := class.bypasses["(*shadowExecutor).CommitAll"]; flagged {
		t.Fatalf("the scan now resolves a receiver that shadows an import name — that is an IMPROVEMENT, "+
			"but the header's residual list and spec 127 R5(d)(v) still say it does not. Rewrite the "+
			"residual (and this test) in the same commit that widened the scan. Flagged: %v", class.bypasses)
	}
	// Non-vacuity: the set WAS classified, and the implementation this test
	// claims is missed is one the classifier actually SAW.
	if class.delegating != 1 {
		t.Fatalf("the fixture set was not classified — expected the honest funnel to count as "+
			"delegating, got %d", class.delegating)
	}
	if _, seen := class.byLabel["(*shadowExecutor).CommitAll"]; !seen {
		t.Fatal("the shadowing implementation was never examined, so this test pins nothing")
	}

	// The control: same delegation, same helper, same reach — only the
	// local's NAME differs, and this one is caught. Without it the red
	// above would be consistent with the scan missing the whole fixture.
	control := commitFunnelClassify(commitFunnelScanFixtures(t, map[string]string{
		"internal/executor/funnel.go": commitFunnelFixtureFunnel,
		"internal/complete/shadow.go": unshadowed,
	}))
	if _, flagged := control.bypasses["(*shadowExecutor).CommitAll"]; !flagged {
		t.Fatalf("the control must be flagged — an unshadowed local receiver delegating to a committing "+
			"same-package helper is exactly what property (3) catches. Bypasses: %v", control.bypasses)
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

// TestCommitFunnel_StatedLimit_APreconditionWhoseErrorIsNeverTestedIsNotSeen
// pins property (2)'s OWN residual — the one left standing after the
// ordering rule was rebuilt as dominance, and disclosed at the same time
// rather than a round later.
//
// The scan proves the precondition is evaluated unconditionally, before the
// commit, with its result not syntactically discarded. It does NOT prove
// the error is propagated: binding it to a real variable that nothing ever
// tests keeps the result, satisfies every syntactic rule above, and commits
// anyway. Answering that is dataflow, which a go/types-free AST scan does
// not do — the same boundary R5(b)'s precedent draws for the sibling merge
// chokepoint scan.
//
// This test REDS the day the scan starts answering it, which is exactly
// when the disclosure must change.
func TestCommitFunnel_StatedLimit_APreconditionWhoseErrorIsNeverTestedIsNotSeen(t *testing.T) {
	const src = `package scratch

func commitWithExport(path, msg string) error {
	err := checkNoPreservedMerge(path, "")
	if err != nil && false {
		return err
	}
	return gitutil.CommitAll(path, msg)
}
`
	fset, file := commitFunnelParse(t, "ignored_error.go", src)
	scan := scanCommitFunnelFile(fset, file, "ignored_error.go", nil)

	if len(scan.funnelDecls) != 1 {
		t.Fatalf("expected one funnel declaration, got %d", len(scan.funnelDecls))
	}
	d := scan.funnelDecls[0]
	// Non-vacuity: this shape is genuinely CLASSIFIED — both calls
	// resolve, and the ordering rule reaches a verdict on it — rather than
	// falling out of the scan for some unrelated reason.
	if d.preconditionPos == token.NoPos || d.primitivePos == token.NoPos {
		t.Fatalf("both calls must resolve for this to pin a DATAFLOW limit; precondition=%v primitive=%v",
			d.preconditionPos, d.primitivePos)
	}
	if !d.preconditionGuards {
		t.Fatalf("the scan now rejects a precondition whose error is never acted on — that is an "+
			"IMPROVEMENT, but commitFunnelDeriveOrder's disclosure and this file's header still say it "+
			"does not answer dataflow. Rewrite the disclosure (and this test) in the same commit that "+
			"widened the scan. Gap: %q", d.guardGap)
	}
}

// TestCommitFunnel_StatedLimit_AFunctionValueCallIsNotSeen pins the
// FUNCTION VALUES residual — the one the header has disclosed since the
// rebuild and nothing held in place until the fourth adversarial round
// counted the fixtures against the disclosure.
//
// Two shapes, one residual: a call through a struct field holding the
// committing function (`v.commit(path, msg)`, where `commit` is a field,
// not a method), and a call through `reflect`. In both, NO CALLEE NAME is
// written at the call site, so there is nothing for a go/types-free scan to
// resolve — `commitFunnelReceiverExpr`'s widening of receiver resolution
// does not and cannot reach either, which is exactly why this stays a
// residual after that widening.
func TestCommitFunnel_StatedLimit_AFunctionValueCallIsNotSeen(t *testing.T) {
	const indirect = `package complete

import (
	"os/exec"
	"reflect"
)

type valueExecutor struct{ commit func(string, string) error }

func newValueExecutor() *valueExecutor {
	return &valueExecutor{commit: doCommitIndirectly}
}

func (v *valueExecutor) CommitAll(path, msg string) error {
	return v.commit(path, msg)
}

type reflectExecutor struct{}

func (r *reflectExecutor) CommitAll(path, msg string) error {
	reflect.ValueOf(doCommitIndirectly).Call(nil)
	return nil
}

func doCommitIndirectly(path, msg string) error {
	return exec.Command("git", "-C", path, "commit", "-am", msg).Run()
}
`
	class := commitFunnelClassify(commitFunnelScanFixtures(t, map[string]string{
		"internal/executor/funnel.go":   commitFunnelFixtureFunnel,
		"internal/complete/indirect.go": indirect,
	}))
	for _, label := range []string{"(*valueExecutor).CommitAll", "(*reflectExecutor).CommitAll"} {
		if _, flagged := class.bypasses[label]; flagged {
			t.Fatalf("the scan now follows a call through a function value — that is an IMPROVEMENT, but the "+
				"header's residual list still says it does not. Rewrite the residual (and this test) in the "+
				"same commit that widened the scan. Flagged: %v", class.bypasses)
		}
		if _, seen := class.byLabel[label]; !seen {
			t.Fatalf("%s was never examined, so this test pins nothing", label)
		}
	}
	// Non-vacuity in the other direction: the committing helper the values
	// point at IS one the scan can see when it is NAMED at a call site —
	// so the residual is about the indirection, not about the helper.
	if class.delegating != 1 {
		t.Fatalf("the fixture set was not classified — expected the honest funnel to count as "+
			"delegating, got %d", class.delegating)
	}
	named := commitFunnelClassify(commitFunnelScanFixtures(t, map[string]string{
		"internal/executor/funnel.go": commitFunnelFixtureFunnel,
		"internal/complete/named.go": `package complete

import "os/exec"

type namedExecutor struct{}

func (n *namedExecutor) CommitAll(path, msg string) error {
	return doCommitIndirectly(path, msg)
}

func doCommitIndirectly(path, msg string) error {
	return exec.Command("git", "-C", path, "commit", "-am", msg).Run()
}
`,
	}))
	if _, flagged := named.bypasses["(*namedExecutor).CommitAll"]; !flagged {
		t.Fatalf("the SAME helper reached by its written name must be caught, or this test pins the helper "+
			"rather than the indirection: %v", named.bypasses)
	}
}

// TestCommitFunnel_StatedLimit_ADotImportedGitutilIsNotSeen pins the
// DOT-IMPORTS residual. A dot import binds `gitutil`'s exported names into
// the file's own scope, so the primitive is called as a bare `CommitAll(…)`
// with no qualifier anywhere — and this scan resolves the primitive through
// the file's import ALIAS for gitutil, which a dot import does not provide.
// commitFunnelImportDirs drops dot imports explicitly for the same reason.
func TestCommitFunnel_StatedLimit_ADotImportedGitutilIsNotSeen(t *testing.T) {
	dotImported := fmt.Sprintf(`package complete

import . "%s"

type dotExecutor struct{}

func (d *dotExecutor) CommitAll(path, msg string) error {
	return commitDotted(path, msg)
}

func commitDotted(path, msg string) error {
	return CommitAll(path, msg)
}
`, gitutilImportPath)
	scans := commitFunnelScanFixtures(t, map[string]string{
		"internal/executor/funnel.go": commitFunnelFixtureFunnel,
		"internal/complete/dot.go":    dotImported,
	})
	class := commitFunnelClassify(scans)
	if _, flagged := class.bypasses["(*dotExecutor).CommitAll"]; flagged {
		t.Fatalf("the scan now resolves a dot-imported gitutil — that is an IMPROVEMENT, but the header's "+
			"residual list still says it does not. Rewrite the residual (and this test) in the same commit "+
			"that widened the scan. Flagged: %v", class.bypasses)
	}
	if class.delegating != 1 {
		t.Fatalf("the fixture set was not classified — expected the honest funnel to count as "+
			"delegating, got %d", class.delegating)
	}
	if _, seen := class.byLabel["(*dotExecutor).CommitAll"]; !seen {
		t.Fatal("the dot-importing implementation was never examined, so this test pins nothing")
	}
	// And precisely WHY it is unseen: property (1) resolves no qualified
	// primitive call in that file at all, because none is written.
	for _, sc := range scans {
		for _, c := range sc.qualifiedPrimitiveCalls {
			if strings.HasPrefix(c.file, "internal/complete/") {
				t.Errorf("a dot-imported primitive call must not resolve as a QUALIFIED call: %+v", c)
			}
		}
	}
}

// TestCommitFunnel_StatedLimit_AForeignCallbackIsNotSeen pins the FOREIGN
// CALLBACKS residual: a committing method handed to a package this walk
// does not parse, and invoked from inside it. `filepath.WalkDir(path,
// w.commitEach)` writes `w.commitEach` as an ARGUMENT, never as a callee,
// so there is no call site for the graph to follow — and the invocation
// that does reach it lives in stdlib source this scan never reads.
func TestCommitFunnel_StatedLimit_AForeignCallbackIsNotSeen(t *testing.T) {
	const callback = `package complete

import (
	"io/fs"
	"os/exec"
	"path/filepath"
)

type walkExecutor struct{ msg string }

func (w *walkExecutor) CommitAll(path, msg string) error {
	w.msg = msg
	return filepath.WalkDir(path, w.commitEach)
}

func (w *walkExecutor) commitEach(p string, d fs.DirEntry, err error) error {
	return exec.Command("git", "-C", p, "commit", "-am", w.msg).Run()
}
`
	class := commitFunnelClassify(commitFunnelScanFixtures(t, map[string]string{
		"internal/executor/funnel.go":   commitFunnelFixtureFunnel,
		"internal/complete/callback.go": callback,
	}))
	if _, flagged := class.bypasses["(*walkExecutor).CommitAll"]; flagged {
		t.Fatalf("the scan now follows a func handed to a foreign package — that is an IMPROVEMENT, but the "+
			"header's residual list still says it does not. Rewrite the residual (and this test) in the same "+
			"commit that widened the scan. Flagged: %v", class.bypasses)
	}
	if class.delegating != 1 {
		t.Fatalf("the fixture set was not classified — expected the honest funnel to count as "+
			"delegating, got %d", class.delegating)
	}
	if _, seen := class.byLabel["(*walkExecutor).CommitAll"]; !seen {
		t.Fatal("the callback-passing implementation was never examined, so this test pins nothing")
	}
	// Non-vacuity: the SAME method, CALLED rather than passed, is caught —
	// so the residual is the handing-off, not the method.
	direct := commitFunnelClassify(commitFunnelScanFixtures(t, map[string]string{
		"internal/executor/funnel.go": commitFunnelFixtureFunnel,
		"internal/complete/direct.go": `package complete

import (
	"io/fs"
	"os/exec"
)

type walkExecutor struct{ msg string }

func (w *walkExecutor) CommitAll(path, msg string) error {
	return w.commitEach(path, nil, nil)
}

func (w *walkExecutor) commitEach(p string, d fs.DirEntry, err error) error {
	return exec.Command("git", "-C", p, "commit", "-am", w.msg).Run()
}
`,
	}))
	if _, flagged := direct.bypasses["(*walkExecutor).CommitAll"]; !flagged {
		t.Fatalf("the same method reached by a written call must be caught, or this test pins the method "+
			"rather than the hand-off: %v", direct.bypasses)
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
	if d.preconditionGuards {
		t.Fatal("the reordered funnel must be detected: the precondition follows the commit")
	}
	if !strings.Contains(d.guardGap, "statement 1") {
		t.Errorf("the failure must name the statement the commit is reached from, got %q", d.guardGap)
	}
}

// TestCommitFunnel_ScanRejectsAPreconditionThatDoesNotDominateTheCommit is
// the mutation proof for property (2)'s ORDERING guarantee, and the shape
// the fourth adversarial round planted against its predecessor. That
// predecessor compared `token.Pos` values — source-text offsets — and so
// answered a question about text while claiming an answer about execution.
//
// Every case below writes the precondition ABOVE the commit and is
// nevertheless unguarded at run time, and the `textuallyFirst` assertion in
// each is the regression pin: it asserts the offsets are still in the order
// the old check called correct, so this test reds again the moment anyone
// re-derives ordering from position. The `defer` case is the one that shipped
// past a full gate set — funnel scan and `golangci-lint` both green — and
// broke `TestCommitAll_RefusesOverAPreservedMerge` at run time.
func TestCommitFunnel_ScanRejectsAPreconditionThatDoesNotDominateTheCommit(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		// want is a substring the derived explanation must carry, so the
		// failure names the construct rather than only the verdict.
		want string
		// guards is the control: the honest shape must still pass, or
		// every red above would prove only that the check is strict.
		guards bool
	}{
		{
			name: "deferred-precondition",
			body: `	defer checkNoPreservedMerge(path, "")
	return gitutil.CommitAll(path, msg)`,
			want: "`defer` statement",
		},
		{
			name: "asynchronous-precondition",
			body: `	go checkNoPreservedMerge(path, "")
	return gitutil.CommitAll(path, msg)`,
			want: "`go` statement",
		},
		{
			name: "conditional-precondition",
			body: `	if msg != "" {
		if err := checkNoPreservedMerge(path, ""); err != nil {
			return err
		}
	}
	return gitutil.CommitAll(path, msg)`,
			want: "conditional branch",
		},
		{
			name: "short-circuited-precondition",
			body: `	if msg != "" && checkNoPreservedMerge(path, "") != nil {
		return errRefused
	}
	return gitutil.CommitAll(path, msg)`,
			want: "conditional branch",
		},
		{
			name: "loop-body-precondition",
			body: `	for _, p := range paths {
		if err := checkNoPreservedMerge(p, ""); err != nil {
			return err
		}
	}
	return gitutil.CommitAll(path, msg)`,
			want: "loop body",
		},
		{
			name: "case-body-precondition",
			body: `	switch msg {
	case "":
		if err := checkNoPreservedMerge(path, ""); err != nil {
			return err
		}
	}
	return gitutil.CommitAll(path, msg)`,
			want: "case body",
		},
		{
			// Found by this file's own hostile-reader pass over the
			// dominance rule above, not by a reviewer: an ORDERING
			// guarantee is not a GUARDING guarantee. Both shapes run the
			// precondition, unconditionally, strictly before the commit —
			// and both throw its refusal on the floor.
			name: "bare-call-precondition",
			body: `	checkNoPreservedMerge(path, "")
	return gitutil.CommitAll(path, msg)`,
			want: "DISCARDS its result",
		},
		{
			name: "blank-assigned-precondition",
			body: `	_ = checkNoPreservedMerge(path, "")
	return gitutil.CommitAll(path, msg)`,
			want: "DISCARDS its result",
		},
		{
			// The fifth confirm round's shape (G2). Every construct above
			// keeps the funnel body a list of statements entered in order
			// and hides the check inside one of them; this one leaves the
			// check at the top level, unconditional and undiscarded — where
			// the dominance rule counts it — and jumps over it. Legal Go:
			// the skipped `if` scopes its `err` to itself, so the jump
			// brings no variable into scope, and the label is in the same
			// block. Compiles and vets clean.
			name: "goto-past-the-precondition",
			body: `	if msg == "skip" {
		goto commit
	}
	if err := checkNoPreservedMerge(path, ""); err != nil {
		return err
	}
commit:
	return gitutil.CommitAll(path, msg)`,
			want: "`goto` written in the funnel body",
		},
		{
			name: "honest-funnel",
			body: `	if err := checkNoPreservedMerge(path, ""); err != nil {
		return err
	}
	if err := exportBeads(path); err != nil {
		return err
	}
	return gitutil.CommitAll(path, msg)`,
			guards: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := "package scratch\n\nfunc commitWithExport(path, msg string) error {\n" + tc.body + "\n}\n"
			fset, file := commitFunnelParse(t, tc.name+".go", src)
			scan := scanCommitFunnelFile(fset, file, tc.name+".go", nil)

			if len(scan.funnelDecls) != 1 {
				t.Fatalf("expected one funnel declaration, got %d", len(scan.funnelDecls))
			}
			d := scan.funnelDecls[0]
			// Non-vacuity: both calls must still RESOLVE. A shape that
			// reds because the scan stopped seeing the precondition at all
			// would be reported as "the funnel no longer checks" and would
			// prove nothing about ordering.
			if d.preconditionPos == token.NoPos || d.primitivePos == token.NoPos {
				t.Fatalf("both calls must resolve for this to be an ORDERING case; precondition=%v primitive=%v",
					d.preconditionPos, d.primitivePos)
			}
			if d.preconditionPos >= d.primitivePos {
				t.Fatalf("this fixture must write the precondition ABOVE the commit — otherwise it does not "+
					"pin the position-vs-execution gap; precondition=%v primitive=%v",
					d.preconditionPos, d.primitivePos)
			}
			if tc.guards {
				if !d.preconditionGuards {
					t.Fatalf("the honest funnel shape must satisfy the ordering guarantee, got gap %q", d.guardGap)
				}
				return
			}
			if d.preconditionGuards {
				t.Fatalf("a precondition that does not run before the commit must NOT satisfy the ordering " +
					"guarantee — source position said it did")
			}
			if !strings.Contains(d.guardGap, tc.want) {
				t.Errorf("the failure must name the construct that broke the ordering; got %q, want it to "+
					"mention %q", d.guardGap, tc.want)
			}
		})
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
	if len(rec.gitutilCalls) != 0 || rec.spawnEvidence != "" {
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
	if d.spawnEvidence != osExecImportPath+".Command" {
		t.Errorf("a CommitAll that shells out through os/exec must be derived as able to reach git; got %q", d.spawnEvidence)
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
