package executor

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func remoteProbeViolations(file *ast.File) []string {
	aliases := map[string]bool{}
	for _, imp := range file.Imports {
		if imp.Path.Value == `"github.com/mrmaxsteel/mindspec/internal/gitutil"` {
			name := "gitutil"
			if imp.Name != nil {
				name = imp.Name.Name
			}
			aliases[name] = true
		}
	}
	var bad []string
	for _, decl := range file.Decls {
		name := "package initializer"
		if fn, ok := decl.(*ast.FuncDecl); ok {
			name = fn.Name.Name
		}
		ast.Inspect(decl, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || !aliases[pkg.Name] {
				return true
			}
			if (sel.Sel.Name == "HasRemote" || sel.Sel.Name == "RemoteConfigured") && name != "specBranchBase" {
				bad = append(bad, name+":"+sel.Sel.Name)
			}
			return true
		})
	}
	return bad
}

func TestExecutor_HasRemoteOnlyInSpecBranchBase(t *testing.T) {
	const planted = `package executor
 import g "github.com/mrmaxsteel/mindspec/internal/gitutil"
 var wrong = g.HasRemote()
 func FinalizeEpic() { g.RemoteConfigured() }
 func specBranchBase() { g.HasRemote() }
 `
	f, err := parser.ParseFile(token.NewFileSet(), "planted.go", planted, 0)
	if err != nil {
		t.Fatal(err)
	}
	if bad := remoteProbeViolations(f); len(bad) != 2 {
		t.Fatalf("positive control missed violations: %v", bad)
	}
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate source")
	}
	entries, err := os.ReadDir(filepath.Dir(here))
	if err != nil {
		t.Fatal(err)
	}
	files := 0
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		files++
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(filepath.Dir(here), entry.Name()), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		if bad := remoteProbeViolations(f); len(bad) != 0 {
			t.Errorf("%s probes outside specBranchBase: %v", entry.Name(), bad)
		}
	}
	if files == 0 {
		t.Fatal("no production files scanned")
	}
}
