package executor

// Spec 127 AC-7(iv): the chokepoint anti-drift test. Every
// gitutil.MergeInto/gitutil.MergeBranch call site in this package must be
// preceded — in the same enclosing function-like construct (a *ast.FuncDecl
// or a *ast.FuncLit, whichever DIRECTLY contains it — never inherited from
// an enclosing scope), by its OWN, dedicated, operand-corresponding call to
// preflightMergeDestruction. A new raw producer that skips the preflight
// call entirely is caught here, at build-and-test time, regardless of
// whether any behavioral test happens to exercise it.
//
// Bead-6 fix round 1 rewrite (O2-1, S3-1, S3-2/G1-2 — three independently
// demonstrated ways the original scan was defeated):
//
//   - O2-1: the original scan only visited *ast.FuncDecl nodes reached via
//     file.Decls, so a package-level `var sneakyMerge = func(...) error {
//     return gitutil.MergeInto(...) }` was never visited AT ALL — not
//     merely unattributed, invisible. This rewrite treats every *ast.FuncLit
//     ANYWHERE in the file (at any nesting depth: bound to a package-level
//     var, a struct field, a closure inside a FuncDecl, a goroutine
//     argument) as its OWN span with its OWN preflight obligation, found by
//     a whole-file ast.Inspect that dispatches a per-span sub-scan at every
//     *ast.FuncDecl and *ast.FuncLit it encounters.
//   - S3-1: the original scan matched only a literal
//     `gitutil.MergeInto(...)`/`gitutil.MergeBranch(...)` *ast.SelectorExpr,
//     so `var adversarialMergeFn = gitutil.MergeInto` followed by
//     `adversarialMergeFn(...)` was invisible. This rewrite collects a
//     package-level alias set FIRST (any `var X = gitutil.MergeInto` /
//     `= gitutil.MergeBranch` assignment, across every file) and treats a
//     bare `X(...)` call as the aliased kind wherever X resolves.
//   - S3-2/G1-2: the original scan required only that SOME preflight call
//     precede ALL merge calls in the function — one preflight call could
//     "certify" an unbounded number of merge calls over DIFFERENT operand
//     pairs, and a nested closure's merge call was certified by an OUTER
//     function's preflight it can execute independently of. This rewrite
//     requires each merge call to consume its OWN, DISTINCT, nearest-
//     preceding preflight call within the SAME span whose operands
//     CORRESPOND (the preflight's branch argument must equal the merge
//     call's source-branch argument, by identifier/selector/literal text;
//     for gitutil.MergeBranch, which also carries an explicit target
//     branch argument, the preflight's target argument must correspond
//     too) — a preflight call already consumed by an earlier merge call in
//     the same span can never certify a second one.
//
// STATED RESIDUAL LIMITATIONS (this is still an AST-level presence-and-
// correspondence check, not a full dataflow analysis — G1's own ruling on
// this bead's panel: "appropriate as an intended-inventory review ratchet,
// not a substitute for operand dataflow" applies to what follows too):
//
//   - gitutil.MergeInto's signature is (targetWorkdir, sourceBranch) — the
//     target is a WORKDIR PATH, not a branch name, so it has no textual
//     form comparable to preflightMergeDestruction's target argument (a
//     branch name). Only the SOURCE operand is correspondence-checked for
//     MergeInto call sites; the target cannot be, structurally. This is a
//     stated, review-caught residual, not a silent gap.
//   - Operand correspondence is TEXTUAL (identifier name, `X.Sel` selector
//     form, or literal value) — not type-checked, not resolved through
//     assignment chains. Two DIFFERENT variables that happen to hold the
//     same runtime value at different textual names are not recognized as
//     corresponding; conversely this cannot be fooled by mere accidental
//     name reuse of UNRELATED values, since a real producer's own operand
//     naming is what this test's fixtures below assert against.
//   - Only a SINGLE-HOP alias (`var X = gitutil.MergeInto`) is resolved.
//     A second-level alias (`var Y = X`) or passing gitutil.MergeInto as a
//     function PARAMETER and calling it via the parameter name is not
//     traced — the identical "one level closes the shape a real author
//     would plausibly reach for" trade-off internal/guard's own
//     alias-resolution walk (outcome_sentinel_test.go) already makes and
//     names honestly for the identical class of escape.
//   - This is lexically-scoped, not a full call-graph trace: a merge call
//     reached through a call to a SEPARATE, independently-declared
//     function (not a closure, not an alias) must carry its OWN preflight
//     call within ITS OWN body — this was already true of the original
//     scan (every *ast.FuncDecl is independently required to hold its own
//     preflight call) and remains true here; this rewrite does not
//     introduce or remove that property.
//
// INLINE CALL-ARGUMENT CLOSURES ARE TRANSPARENT, ON PURPOSE: this
// package's own real producers pass their merge attempt as an INLINE
// func literal directly into resumeAwareMerge's mergeFn parameter —
// `resumeAwareMerge(..., func() error { return gitutil.MergeInto(...) },
// ...)` — evaluated SYNCHRONOUSLY as part of that very call, not stored,
// not deferred, not handed to a goroutine. G1-2's own finding is about a
// closure that "can execute independently" of the outer preflight's
// timing; an inline call argument cannot — it runs inside the same
// invocation, immediately after the preflight call the enclosing
// function made moments before. So a *ast.FuncLit that is a DIRECT
// element of some *ast.CallExpr's Args list is treated as TRANSPARENT:
// its calls are attributed to its ENCLOSING span, not a new one — this
// is what lets the real producers pass, while a *ast.FuncLit bound to a
// `var` (O2-1's shape — never a direct call argument) still gets its own,
// independent span and its own obligation.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// mergeChokepointRepoRoot resolves the repo root from this test file's own
// location (mirrors internal/guard/registries_test.go's
// repoRootFromGuardTestDir pattern) so the scan reads THIS worktree's real
// source, never an assumption about the caller's cwd.
func mergeChokepointRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Join(wd, "..", "..")
}

// mergeCallInfo is one gitutil.MergeInto/MergeBranch call site (or an
// alias-resolved equivalent) found inside a single span.
type mergeCallInfo struct {
	pos       token.Pos
	kind      string // "MergeInto" or "MergeBranch"
	sourceKey string // the source-branch operand, textually
	targetKey string // the target-branch operand, textually — "" for
	// MergeInto (structurally uncheckable; see the stated residual above)
}

// preflightCallInfo is one preflightMergeDestruction(branch, target, ...)
// call site found inside a single span.
type preflightCallInfo struct {
	pos       token.Pos
	branchKey string
	targetKey string
}

// funcSpan is one *ast.FuncDecl or *ast.FuncLit's OWN merge/preflight
// calls — never a nested FuncLit's (those get their own span; see
// collectSpanCalls).
type funcSpan struct {
	file       string
	label      string
	mergeCalls []mergeCallInfo
	preflight  []preflightCallInfo
}

// operandKey renders an operand expression to a comparable text form:
// an identifier's name, a selector's dotted form (e.g. "e.Branch"), or a
// literal's raw value (e.g. `"main"`, quotes included so a literal never
// collides with an identically-named identifier). Any other expression
// shape renders to a form that can never equal another operand's key
// (fail-closed: an unresolvable operand can never be treated as a match).
func operandKey(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return operandKey(v.X) + "." + v.Sel.Name
	case *ast.BasicLit:
		return v.Value
	case *ast.ParenExpr:
		return operandKey(v.X)
	default:
		return fmt.Sprintf("<unresolved:%T@%v>", e, e.Pos())
	}
}

// collectInlineArgFuncLits finds every *ast.FuncLit that is a DIRECT
// element of some *ast.CallExpr's Args list anywhere in file — a closure
// passed inline to a call, evaluated synchronously as part of that call,
// never stored or deferred. These are TRANSPARENT to the span-collection
// walk below (see this file's package doc comment, "INLINE CALL-ARGUMENT
// CLOSURES ARE TRANSPARENT"): their calls belong to their ENCLOSING span,
// not a new one of their own.
func collectInlineArgFuncLits(file *ast.File) map[*ast.FuncLit]bool {
	inline := map[*ast.FuncLit]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		for _, arg := range call.Args {
			if fl, ok := arg.(*ast.FuncLit); ok {
				inline[fl] = true
			}
		}
		return true
	})
	return inline
}

// collectSpanCalls walks root (a *ast.FuncDecl's or *ast.FuncLit's Body)
// collecting merge/preflight calls that belong DIRECTLY to it — descent
// stops at any NESTED, NON-INLINE *ast.FuncLit boundary, so a closure
// bound to a var/field/return (never a direct call argument) never has
// its calls attributed to its enclosing span (it gets its own span from
// the whole-file walk that dispatches this function). A nested FuncLit
// that IS a direct call argument (inlineArgs[fl]) is transparent: descent
// continues into it as if it were part of the current span.
func collectSpanCalls(root ast.Node, aliases map[string]string, inlineArgs map[*ast.FuncLit]bool) (merges []mergeCallInfo, preflights []preflightCallInfo) {
	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		if n != root {
			if fl, ok := n.(*ast.FuncLit); ok && !inlineArgs[fl] {
				return false
			}
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := call.Fun.(type) {
		case *ast.SelectorExpr:
			if xid, ok := fn.X.(*ast.Ident); ok && xid.Name == "gitutil" &&
				(fn.Sel.Name == "MergeInto" || fn.Sel.Name == "MergeBranch") {
				merges = append(merges, buildMergeCall(call, fn.Sel.Name))
			}
			if fn.Sel.Name == "preflightMergeDestruction" {
				preflights = append(preflights, buildPreflightCall(call))
			}
		case *ast.Ident:
			if kind, ok := aliases[fn.Name]; ok {
				merges = append(merges, buildMergeCall(call, kind))
			}
		}
		return true
	})
	return merges, preflights
}

func buildMergeCall(call *ast.CallExpr, kind string) mergeCallInfo {
	mc := mergeCallInfo{pos: call.Pos(), kind: kind}
	switch kind {
	case "MergeInto":
		// gitutil.MergeInto(targetWorkdir, sourceBranch string) — Args[1]
		// is the source branch; Args[0] is a workdir PATH, structurally
		// uncomparable to a preflight's branch-name target argument (see
		// the stated residual in this file's package doc comment).
		if len(call.Args) >= 2 {
			mc.sourceKey = operandKey(call.Args[1])
		}
	case "MergeBranch":
		// gitutil.MergeBranch(workdir, source, target string) — both
		// branch-name operands are checkable.
		if len(call.Args) >= 3 {
			mc.sourceKey = operandKey(call.Args[1])
			mc.targetKey = operandKey(call.Args[2])
		}
	}
	return mc
}

func buildPreflightCall(call *ast.CallExpr) preflightCallInfo {
	pc := preflightCallInfo{pos: call.Pos()}
	if len(call.Args) >= 2 {
		pc.branchKey = operandKey(call.Args[0])
		pc.targetKey = operandKey(call.Args[1])
	}
	return pc
}

// collectMergeFnAliases scans every top-level `var` declaration across
// every parsed file for a direct, single-hop alias of
// gitutil.MergeInto/gitutil.MergeBranch (spec 127 bead-6 fix round 1,
// S3-1): `var adversarialMergeFn = gitutil.MergeInto`. Only this one-hop
// form is resolved — see the stated residual in this file's package doc
// comment for why a second-level alias or a function-parameter
// indirection is not traced.
func collectMergeFnAliases(files []*ast.File) map[string]string {
	aliases := map[string]string{}
	for _, file := range files {
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, val := range vs.Values {
					sel, ok := val.(*ast.SelectorExpr)
					if !ok {
						continue
					}
					xid, ok := sel.X.(*ast.Ident)
					if !ok || xid.Name != "gitutil" {
						continue
					}
					if sel.Sel.Name != "MergeInto" && sel.Sel.Name != "MergeBranch" {
						continue
					}
					if i < len(vs.Names) {
						aliases[vs.Names[i].Name] = sel.Sel.Name
					}
				}
			}
		}
	}
	return aliases
}

// TestMergeChokepoint_EveryProducerConsultsThePreflight is AC-7(iv):
// every gitutil.MergeInto/gitutil.MergeBranch call site (direct or
// single-hop-aliased) in this package, in every function-like construct
// (FuncDecl or FuncLit, at any nesting depth), is preceded by its OWN,
// distinct, operand-corresponding call to preflightMergeDestruction.
func TestMergeChokepoint_EveryProducerConsultsThePreflight(t *testing.T) {
	root := mergeChokepointRepoRoot(t)
	pkgDir := filepath.Join(root, "internal", "executor")
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		t.Fatalf("reading %s: %v", pkgDir, err)
	}

	fset := token.NewFileSet()
	var files []*ast.File
	var fileNames []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".go" {
			continue
		}
		// Scan every non-test .go file: a producer belongs in production
		// code, and scanning test files too would only ever ADD false
		// findings (a test helper calling MergeInto directly against a
		// real repo, with no preflight, is a fixture-building convenience,
		// not a producer) — excluding _test.go avoids that noise while
		// never narrowing the production-code coverage this AC requires.
		if len(e.Name()) > 8 && e.Name()[len(e.Name())-8:] == "_test.go" {
			continue
		}
		path := filepath.Join(pkgDir, e.Name())
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}
		files = append(files, file)
		fileNames = append(fileNames, e.Name())
	}

	aliases := collectMergeFnAliases(files)

	var spans []*funcSpan
	for i, file := range files {
		fileName := fileNames[i]
		inlineArgs := collectInlineArgFuncLits(file)
		ast.Inspect(file, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.FuncDecl:
				if v.Body == nil {
					return true
				}
				merges, preflights := collectSpanCalls(v.Body, aliases, inlineArgs)
				spans = append(spans, &funcSpan{file: fileName, label: v.Name.Name, mergeCalls: merges, preflight: preflights})
			case *ast.FuncLit:
				if inlineArgs[v] {
					// Transparent — its calls are already attributed to
					// its enclosing span (see collectSpanCalls); it gets
					// no span of its own.
					return true
				}
				merges, preflights := collectSpanCalls(v.Body, aliases, inlineArgs)
				spans = append(spans, &funcSpan{
					file:       fileName,
					label:      fmt.Sprintf("func literal at %s", fset.Position(v.Pos())),
					mergeCalls: merges,
					preflight:  preflights,
				})
			}
			return true
		})
	}

	nonEmpty := 0
	totalCalls := 0
	for _, fs := range spans {
		if len(fs.mergeCalls) == 0 {
			continue
		}
		nonEmpty++
		totalCalls += len(fs.mergeCalls)

		sort.Slice(fs.mergeCalls, func(a, b int) bool { return fs.mergeCalls[a].pos < fs.mergeCalls[b].pos })
		sort.Slice(fs.preflight, func(a, b int) bool { return fs.preflight[a].pos < fs.preflight[b].pos })

		consumed := make([]bool, len(fs.preflight))
		for _, mc := range fs.mergeCalls {
			best := -1
			for i, pc := range fs.preflight {
				if pc.pos >= mc.pos || consumed[i] {
					continue
				}
				if pc.branchKey != mc.sourceKey {
					continue
				}
				if mc.targetKey != "" && pc.targetKey != mc.targetKey {
					continue
				}
				if best == -1 || pc.pos > fs.preflight[best].pos {
					best = i
				}
			}
			if best == -1 {
				t.Errorf("%s (%s): a gitutil.%s call at %s has no DISTINCT, operand-corresponding preflightMergeDestruction call preceding it in the same function-like construct — a merge producer must consult the R4 preflight over its OWN operands (AC-7(iv))", fs.label, fs.file, mc.kind, fset.Position(mc.pos))
				continue
			}
			consumed[best] = true
		}
	}

	if nonEmpty == 0 {
		t.Fatal("found zero MergeInto/MergeBranch call sites in internal/executor — this test's own probe is broken (the producers must have moved or been renamed)")
	}

	// AC-7(iv) pins the enumeration as "whole and unstaged": exactly the
	// three producers this spec names (CompleteBead's bead→spec MergeInto,
	// FinalizeEpic's bead→spec auto-merge, and the direct spec→main
	// MergeBranch). A count outside {3} means either a producer was
	// removed (shrinking the surface this AC guards) or a new one
	// appeared — both worth a human's attention, not a silent pass. G1's
	// ruling on this literal (bead-6 fix round 1): "appropriate as an
	// intended-inventory review ratchet, but not a substitute for operand
	// dataflow" — kept alongside, never instead of, the correspondence
	// check above.
	const wantMergeCallSites = 3
	if totalCalls != wantMergeCallSites {
		t.Errorf("found %d total MergeInto/MergeBranch call site(s) in internal/executor production code, want exactly %d (spec 127's three enumerated producers) — re-audit this count if a producer was legitimately added or removed", totalCalls, wantMergeCallSites)
	}
}
