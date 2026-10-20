package panel

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
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

func TestGate_QuotedLedgerIsArtifact(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_ASKPASS", "/usr/bin/false")
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return string(out)
	}
	git("init", "-q")
	git("config", "core.quotePath", "true")
	ledger := ".mindspec/specs/130-test/reviews/café/dispositions.jsonl"
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, ledger)), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ledger), []byte("{}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	status := git("status", "--porcelain", "--untracked-files=all")
	if !strings.Contains(status, `caf\303\251`) {
		t.Fatalf("fixture must exercise git's C-style quoting: %q", status)
	}
	if dirt := userDirtPaths(status, ".mindspec/specs"); len(dirt) != 0 {
		t.Fatalf("quoted ledger classified as user dirt: %q", dirt)
	}
	// Quoted rename destinations use the same decoding before matching.
	if dirt := userDirtPaths(`R  old-ledger -> `+strings.TrimPrefix(strings.TrimSpace(status), "?? ")+"\n", ".mindspec/specs"); len(dirt) != 0 {
		t.Fatalf("quoted renamed ledger classified as user dirt: %q", dirt)
	}
}

func TestGate_PorcelainQuotedNamesAndRenames(t *testing.T) {
	for _, status := range []string{
		` M ".mindspec/specs/130-test/reviews/caf\303\251 -> draft/dispositions.jsonl"`,
		`R  "old -> ledger" -> ".mindspec/specs/130-test/reviews/caf\303\251/dispositions.jsonl"`,
	} {
		if dirt := userDirtPaths(status+"\n", ".mindspec/specs"); len(dirt) != 0 {
			t.Errorf("quoted ledger classified as user dirt: %q", dirt)
		}
	}
	for _, name := range []string{"'a'", "`notes`", "plain.txt"} {
		if dirt := userDirtPaths(" M "+name+"\n", ".mindspec/specs"); len(dirt) != 1 || dirt[0] != name {
			t.Errorf("plain path %q changed: %q", name, dirt)
		}
	}
}
