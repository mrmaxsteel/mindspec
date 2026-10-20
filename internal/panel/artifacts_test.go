package panel

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestToolArtifactPaths_SingleHome(t *testing.T) {
	for _, dir := range []string{".mindspec/specs", ".mindspec/docs/specs", "docs/specs"} {
		patterns := ToolArtifactPaths("/checkout/" + dir + "/130-test")
		if len(patterns) != 2 || patterns[0] != ".beads/issues.jsonl" || patterns[1] != dir+"/*/reviews/*/dispositions.jsonl" {
			t.Fatalf("wrong home: %v", patterns)
		}
		for _, name := range []string{patterns[0], dir + "/130-test/reviews/p/dispositions.jsonl"} {
			if !IsToolArtifactPath(name, dir) {
				t.Errorf("artifact rejected: %s", name)
			}
		}
		for _, name := range []string{dir + "/130-test/reviews/p/panel.json", dir + "/130-test/reviews/p/nested/dispositions.jsonl", "src/reviews/p/dispositions.jsonl", ".mindspec/reviews/p/dispositions.jsonl"} {
			if IsToolArtifactPath(name, dir) {
				t.Errorf("user path accepted: %s", name)
			}
		}
	}
	for _, file := range []string{"gate.go", "../next/guard.go", "../complete/materialize.go"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		tree, err := parser.ParseFile(token.NewFileSet(), file, data, 0)
		if err != nil {
			t.Fatal(err)
		}
		usesHome := false
		ast.Inspect(tree, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.Ident:
				if n.Name == "IsToolArtifactPath" || n.Name == "ToolArtifactPaths" {
					usesHome = true
				}
			case *ast.BasicLit:
				if n.Kind == token.STRING {
					value, _ := strconv.Unquote(n.Value)
					if value == ".beads/issues.jsonl" || strings.Contains(value, "dispositions.jsonl") {
						t.Errorf("%s holds a local artifact entry %q", file, value)
					}
				}
			}
			return true
		})
		if !usesHome {
			t.Errorf("%s does not use the shared artifact home", file)
		}
	}
	if got := userDirtPaths(" M .mindspec/specs/130-test/reviews/p/dispositions.jsonl\n M source.go\n", ".mindspec/specs"); len(got) != 1 || got[0] != "source.go" {
		t.Fatalf("gate classification: %v", got)
	}
}
