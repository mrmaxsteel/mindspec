package panel

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"
)

// TestRawMergeFence_CallSiteShapeSentinel is the derive-don't-carry
// fix for RawMergeFence's own doc comment (spec 127 bead-2 rework
// round 2, O2c-3/S1-4/S1-CONFIRM-2/S2-r2-NEW1/S3-r2-NEW1 — the SAME
// finding, independently confirmed by four reviewer slots): the doc
// comment used to cite eight absolute line numbers as "counted
// directly from this file, not carried over from a prior estimate",
// and every one of those eight numbers was already stale by the time
// that sentence shipped (a later edit to the SAME comment block shifted
// every call site by +25 lines without the citation being recomputed —
// the fifth stale count this spec has produced, and the first one
// living inside the very artifact meant to stop count drift).
//
// The fix is to stop citing absolute line numbers at all — a reader
// can `grep -n 'RawMergeFence(f.BeadID)' internal/panel/gate.go` as
// easily as trust a comment, and grep cannot go stale silently. What
// the doc comment keeps claiming is the COUNT (8) and the SHAPE SPLIT
// (7 sites passing RawMergeFence's result as a %s argument to an
// outer fmt.Sprintf, 1 site appending it via strings.Builder) — and
// THIS test is what makes that claim self-checking: it re-parses
// gate.go fresh on every run and REDs the moment either number drifts,
// rather than trusting the prose.
func TestRawMergeFence_CallSiteShapeSentinel(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(thisFile), "gate.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}

	var sprintfArgSites, builderSites, other int
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fn, ok := call.Fun.(*ast.Ident)
		if !ok || fn.Name != "RawMergeFence" {
			return true
		}
		switch parent := findEnclosingCall(file, call); {
		case parent == "Sprintf":
			sprintfArgSites++
		case parent == "WriteString":
			builderSites++
		default:
			other++
		}
		return true
	})

	total := sprintfArgSites + builderSites + other
	if total != 8 {
		t.Errorf("RawMergeFence(...) call-site count = %d, want 8 (update this test AND RawMergeFence's own doc comment together if the count genuinely changed)", total)
	}
	if sprintfArgSites != 7 {
		t.Errorf("RawMergeFence(...) sites reached via an outer fmt.Sprintf %%s argument = %d, want 7", sprintfArgSites)
	}
	if builderSites != 1 {
		t.Errorf("RawMergeFence(...) sites reached via strings.Builder.WriteString = %d, want 1", builderSites)
	}
	if other != 0 {
		t.Errorf("RawMergeFence(...) sites reached via neither fmt.Sprintf nor strings.Builder = %d, want 0 (a new call shape needs its own accounting in RawMergeFence's doc comment)", other)
	}
}

// findEnclosingCall returns the callee name of the nearest enclosing
// *ast.CallExpr whose argument list directly contains target — "" if
// target is not found as a direct argument of any call in file.
func findEnclosingCall(file *ast.File, target *ast.CallExpr) string {
	found := ""
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		for _, arg := range call.Args {
			if arg == target {
				switch fn := call.Fun.(type) {
				case *ast.SelectorExpr:
					found = fn.Sel.Name
				case *ast.Ident:
					found = fn.Name
				}
			}
		}
		return true
	})
	return found
}
