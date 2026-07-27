package executor

// Spec 127 AC-7(iv): the chokepoint anti-drift test. Every
// gitutil.MergeInto/gitutil.MergeBranch call site in this package must be
// preceded — in the same enclosing function, by source position — by a
// call to preflightMergeDestruction (dataflow: a real evaluation feeding
// the merge call, not mere call-site adjacency — F3-r2-3). A new raw
// producer that skips the preflight call entirely is caught here, at
// build-and-test time, regardless of whether any behavioral test happens
// to exercise it.
//
// This is an AST-level presence-and-ordering check, not a full dataflow
// analysis: it does not prove the preflight call's branch/target operands
// are the SAME branch/target the merge call uses (that is asserted
// behaviorally by the AC-7/AC-8/target-drift-backstop tests elsewhere in
// this package, which force a destructive outcome and observe the merge
// never runs). What it proves mechanically is the narrower, purely
// structural claim AC-7(iv) needs: a MergeInto/MergeBranch call with NO
// preceding preflightMergeDestruction call anywhere in its enclosing
// function is red, so a bead that adds a fourth raw producer and forgets
// the call entirely cannot land silently.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
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

// TestMergeChokepoint_EveryProducerConsultsThePreflight is AC-7(iv):
// every gitutil.MergeInto/gitutil.MergeBranch call site in this package
// is preceded, in the same enclosing function, by a call to
// preflightMergeDestruction.
func TestMergeChokepoint_EveryProducerConsultsThePreflight(t *testing.T) {
	root := mergeChokepointRepoRoot(t)
	pkgDir := filepath.Join(root, "internal", "executor")
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		t.Fatalf("reading %s: %v", pkgDir, err)
	}

	fset := token.NewFileSet()
	type funcSpan struct {
		file          string
		name          string
		start, end    token.Pos
		mergeCalls    []token.Pos
		preflightCall token.Pos // 0 (token.NoPos) if none found
	}
	var funcs []*funcSpan

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
		if filepath.Ext(e.Name()) == ".go" && len(e.Name()) > 8 && e.Name()[len(e.Name())-8:] == "_test.go" {
			continue
		}
		path := filepath.Join(pkgDir, e.Name())
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			fs := &funcSpan{file: e.Name(), name: fd.Name.Name, start: fd.Body.Pos(), end: fd.Body.End()}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch fn := call.Fun.(type) {
				case *ast.SelectorExpr:
					// gitutil.MergeInto(...) / gitutil.MergeBranch(...)
					if xid, ok := fn.X.(*ast.Ident); ok && xid.Name == "gitutil" &&
						(fn.Sel.Name == "MergeInto" || fn.Sel.Name == "MergeBranch") {
						fs.mergeCalls = append(fs.mergeCalls, call.Pos())
					}
					// g.preflightMergeDestruction(...) — any receiver name,
					// selector method call.
					if fn.Sel.Name == "preflightMergeDestruction" {
						if fs.preflightCall == token.NoPos || call.Pos() < fs.preflightCall {
							fs.preflightCall = call.Pos()
						}
					}
				}
				return true
			})
			if len(fs.mergeCalls) > 0 {
				funcs = append(funcs, fs)
			}
		}
	}

	if len(funcs) == 0 {
		t.Fatal("found zero MergeInto/MergeBranch call sites in internal/executor — this test's own probe is broken (the producers must have moved or been renamed)")
	}

	totalCalls := 0
	for _, fs := range funcs {
		totalCalls += len(fs.mergeCalls)
		if fs.preflightCall == token.NoPos {
			t.Errorf("function %s (%s) calls gitutil.MergeInto/MergeBranch %d time(s) with NO call to preflightMergeDestruction anywhere in its body — a merge producer must consult the R4 preflight (AC-7(iv))", fs.name, fs.file, len(fs.mergeCalls))
			continue
		}
		for _, mc := range fs.mergeCalls {
			if mc < fs.preflightCall {
				t.Errorf("function %s (%s): a gitutil.MergeInto/MergeBranch call at %s precedes its preflightMergeDestruction call at %s — the preflight must run before the merge, not after", fs.name, fs.file, fset.Position(mc), fset.Position(fs.preflightCall))
			}
		}
	}

	// AC-7(iv) pins the enumeration as "whole and unstaged": exactly the
	// three producers this spec names (CompleteBead's bead→spec MergeInto,
	// FinalizeEpic's bead→spec auto-merge MergeInto, and the direct
	// spec→main MergeBranch). A count outside {3} means either a producer
	// was removed (shrinking the surface this AC guards) or a new one
	// appeared — both worth a human's attention, not a silent pass.
	const wantMergeCallSites = 3
	if totalCalls != wantMergeCallSites {
		t.Errorf("found %d total MergeInto/MergeBranch call site(s) in internal/executor production code, want exactly %d (spec 127's three enumerated producers) — re-audit this count if a producer was legitimately added or removed", totalCalls, wantMergeCallSites)
	}
}
