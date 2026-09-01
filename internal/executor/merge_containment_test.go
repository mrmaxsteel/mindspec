package executor

// merge_containment_test.go — spec 127 final review, O1-1/O2-2 (MAJOR).
//
// TestMergeChokepoint_ResolvableProducersConsultThePreflight (AC-7(iv))
// performs its operand-correspondence analysis over exactly ONE directory,
// internal/executor, and its total-call-site sentinel counts only within
// it. That is the right scope for the analysis — every merge producer is
// supposed to live there — but on its own it left the plainest resolvable
// shape of all UNSEEN: a bare `gitutil.MergeInto(wt, branch)` in
// internal/approve, internal/complete, or cmd/mindspec is never parsed, so
// it neither fails the correspondence check nor moves the count. AC-7(iv)
// stated its residual purely as a list of exotic SHAPES (dot-import,
// embedded field, container element, multi-hop named type, interface
// dispatch, reflection, function-local declaration); PACKAGE SCOPE appeared
// in none of them, so a reader concluded the ordinary direct call was
// machine-caught everywhere when it was machine-caught in one package.
//
// This closes the gap by WIDENING THE MECHANISM rather than narrowing the
// claim: a repo-wide containment ratchet asserting that no production file
// OUTSIDE internal/executor calls gitutil.MergeInto/MergeBranch at all. The
// two ratchets compose into the claim AC-7(iv) makes: every merge producer
// is IN internal/executor (this test), and every producer in
// internal/executor consults the preflight over its own operands
// (merge_chokepoint_test.go). A producer added anywhere else now REDs here
// with a message naming where it must move.
//
// SCOPE, stated exactly. This scan is deliberately BROADER and BLUNTER
// than the chokepoint's: it needs no operand dataflow, so it flags on
// name/identity alone and covers three call shapes —
//
//	  1. a selector call on the file's own gitutil import identifier,
//	     whatever that file aliases it to (never the literal "gitutil");
//	  2. a bare call to MergeInto/MergeBranch in a file that DOT-imports
//	     gitutil — the dot-import shape merge_chokepoint_test.go names as
//	     an explicit residual;
//	  3. a bare call to MergeInto/MergeBranch inside package gitutil's own
//	     non-test files.
//
// Its residual is the same one every AST-level scan in this spec carries
// and cannot escape without go/types: a producer reached through a
// function value, struct field, interface, or reflection is invisible
// here. That class is review-caught, exactly as merge_chokepoint_test.go
// documents for its own scan — this test narrows nothing it claims and
// claims nothing about that class.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// mergeProducerNames are the two gitutil entrypoints that actually perform
// a merge. Kept as a named set so a third one added to gitutil is a
// one-line extension here rather than a silent hole.
var mergeProducerNames = map[string]bool{"MergeInto": true, "MergeBranch": true}

// mergeContainmentDir is the ONE package a merge producer may live in.
var mergeContainmentDir = filepath.Join("internal", "executor")

// gitutilDotImported reports whether file imports gitutil with `.`, in
// which case a bare MergeInto(...)/MergeBranch(...) call resolves to it.
func gitutilDotImported(file *ast.File) bool {
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != gitutilImportPath {
			continue
		}
		return imp.Name != nil && imp.Name.Name == "."
	}
	return false
}

// gitutilImported reports whether file imports gitutil at all (under any
// name). A file that does not import it cannot reach a producer through a
// selector on it.
func gitutilImported(file *ast.File) bool {
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err == nil && path == gitutilImportPath {
			return true
		}
	}
	return false
}

// TestMergeProducersAreContainedInTheExecutorPackage is the repo-wide half
// of AC-7(iv): the chokepoint's operand-correspondence analysis is
// package-scoped by design, so this asserts the population it analyses is
// the whole population.
func TestMergeProducersAreContainedInTheExecutorPackage(t *testing.T) {
	root := mergeChokepointRepoRoot(t)

	type violation struct {
		file string
		pos  token.Position
		call string
	}
	var violations []violation
	scanned := 0
	insideExecutor := 0

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			// beads/ is a vendored sibling tool, not this module's
			// source; the rest never hold production Go.
			case "beads", "testdata", ".git", ".worktrees", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}

		fset := token.NewFileSet()
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			// A file this scan cannot parse is an EVIDENCE ERROR, never a
			// clean file: failing the test is the fail-closed disposition.
			t.Errorf("parsing %s: %v — this containment scan cannot certify a file it cannot read", rel, perr)
			return nil
		}
		scanned++

		inExecutor := strings.HasPrefix(rel, mergeContainmentDir+string(filepath.Separator))
		if inExecutor {
			insideExecutor++
		}

		gitutilName := ""
		if gitutilImported(file) {
			gitutilName = gitutilLocalName(file)
		}
		bareResolves := gitutilDotImported(file) || file.Name.Name == "gitutil"

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			var name string
			switch fn := unwrapParens(call.Fun).(type) {
			case *ast.SelectorExpr:
				xid, ok := fn.X.(*ast.Ident)
				if !ok || gitutilName == "" || xid.Name != gitutilName {
					return true
				}
				name = fn.Sel.Name
			case *ast.Ident:
				if !bareResolves {
					return true
				}
				name = fn.Name
			default:
				return true
			}
			if !mergeProducerNames[name] {
				return true
			}
			if inExecutor {
				return true // merge_chokepoint_test.go owns this call site
			}
			violations = append(violations, violation{file: rel, pos: fset.Position(call.Pos()), call: name})
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}

	// Probe self-checks: a scan that silently read nothing would pass
	// vacuously, which is the failure mode this whole spec exists to
	// eliminate.
	if scanned < 100 {
		t.Fatalf("containment scan visited only %d production .go files — its own walk is broken, not the tree", scanned)
	}
	if insideExecutor == 0 {
		t.Fatalf("containment scan visited no file under %s — the package moved and this scan's exemption now points at nothing", mergeContainmentDir)
	}

	for _, v := range violations {
		t.Errorf("%s calls gitutil.%s at %s — a merge producer outside %s is invisible to AC-7(iv)'s operand-correspondence scan (which reads only that directory), so nothing checks that it consults preflightMergeDestruction over its own operands. Move the producer into %s, or extend BOTH ratchets deliberately.",
			v.file, v.call, v.pos, mergeContainmentDir, mergeContainmentDir)
	}
}
