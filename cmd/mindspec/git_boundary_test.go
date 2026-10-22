package main

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

func checkoutGitBoundary(path string) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return nil, err
	}
	var hits []string
	for _, imp := range f.Imports {
		value, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil, err
		}
		if value == "os/exec" || value == "internal/gitutil" || strings.HasSuffix(value, "/internal/gitutil") {
			hits = append(hits, value)
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if value, err := strconv.Unquote(lit.Value); err == nil && value == "git" {
				hits = append(hits, value)
			}
		}
		return true
	})
	return hits, nil
}

func TestPanelCheckout_GitOnlyViaExecutor(t *testing.T) {
	for _, path := range []string{"panel_checkout.go", "panel_submit.go", "panel_prompt.go", "panel_launch.go", "panel_recover.go", "../../internal/workspace/slotroot.go"} {
		if _, err := os.Stat(path); os.IsNotExist(err) && path != "panel_checkout.go" && path != "../../internal/workspace/slotroot.go" {
			continue
		}
		hits, err := checkoutGitBoundary(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 0 {
			t.Errorf("%s bypasses executor: %v", path, hits)
		}
	}
	t.Run("negative", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "wrong.go")
		slotWrite(t, path, "package wrong\nimport \"os/exec\"\nvar _ = exec.Command(\"git\")\n")
		hits, err := checkoutGitBoundary(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 2 || hits[0] != "os/exec" || hits[1] != "git" {
			t.Fatalf("scanner failed to detect wrong implementation: %v", hits)
		}
	})
}
