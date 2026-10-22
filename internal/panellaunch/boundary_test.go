package panellaunch

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func launchBoundary(source string) error {
	f, err := parser.ParseFile(token.NewFileSet(), "source.go", source, 0)
	if err != nil {
		return err
	}
	for _, spec := range f.Imports {
		value, _ := strconv.Unquote(spec.Path.Value)
		if strings.HasSuffix(value, "/internal/gitutil") {
			return &os.PathError{Op: "forbidden import", Path: value, Err: os.ErrPermission}
		}
	}
	var violation error
	ast.Inspect(f, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Command" && sel.Sel.Name != "CommandContext" {
			return true
		}
		index := 0
		if sel.Sel.Name == "CommandContext" {
			index = 1
		}
		if len(call.Args) > index {
			if lit, ok := call.Args[index].(*ast.BasicLit); ok {
				s, _ := strconv.Unquote(lit.Value)
				if s == "git" {
					violation = &os.PathError{Op: "forbidden command", Path: s, Err: os.ErrPermission}
				}
			}
		}
		return true
	})
	return violation
}
func TestPanelLaunch_NoGitCalls(t *testing.T) {
	entries, e := os.ReadDir(".")
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		count++
		raw, e := os.ReadFile(filepath.Clean(entry.Name()))
		if e != nil {
			t.Fatal(e)
		}
		if e = launchBoundary(string(raw)); e != nil {
			t.Fatal(e)
		}
	}
	if count == 0 {
		t.Fatal("no package sources inspected")
	}
	for _, source := range []string{`package p; import "github.com/mrmaxsteel/mindspec/internal/gitutil"`, `package p; func f(){exec.Command("git")}`, `package p; func f(){exec.CommandContext(ctx,"git")}`} {
		if launchBoundary(source) == nil {
			t.Fatalf("detector accepted %s", source)
		}
	}
}
