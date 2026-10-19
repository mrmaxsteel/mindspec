package lint

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// rowcapFindings resolves package-level function variables across files before
// inspecting calls. Methods (notably harness sandbox.runBD) are not runners.
func rowcapFindings(sources map[string][]byte) ([]string, error) {
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	aliases := map[string]map[string]int{}
	literal := func(e ast.Expr) string {
		if s, ok := e.(*ast.BasicLit); ok && s.Kind == token.STRING {
			v, _ := strconv.Unquote(s.Value)
			return v
		}
		return ""
	}
	for path, src := range sources {
		f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		files[path] = f
		pkg := filepath.Dir(path)
		if aliases[pkg] == nil {
			aliases[pkg] = map[string]int{}
		}
	}
	resolve := func(path string, f *ast.File, e ast.Expr) int {
		if id, ok := e.(*ast.Ident); ok {
			return aliases[filepath.Dir(path)][id.Name]
		}
		s, ok := e.(*ast.SelectorExpr)
		if !ok {
			return 0
		}
		id, ok := s.X.(*ast.Ident)
		if !ok {
			return 0
		}
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			name := filepath.Base(p)
			if imp.Name != nil {
				name = imp.Name.Name
			}
			if name != id.Name {
				continue
			}
			if p == "github.com/mrmaxsteel/mindspec/internal/bead" && (s.Sel.Name == "RunBD" || s.Sel.Name == "RunBDCombined") {
				return 1
			}
			if p == "os/exec" && s.Sel.Name == "Command" {
				return 2
			}
		}
		return 0
	}
	for changed := true; changed; {
		changed = false
		for path, f := range files {
			for _, decl := range f.Decls {
				g, ok := decl.(*ast.GenDecl)
				if !ok || g.Tok != token.VAR {
					continue
				}
				for _, spec := range g.Specs {
					v := spec.(*ast.ValueSpec)
					for i, value := range v.Values {
						if i >= len(v.Names) {
							continue
						}
						if kind := resolve(path, f, value); kind != 0 && aliases[filepath.Dir(path)][v.Names[i].Name] == 0 {
							aliases[filepath.Dir(path)][v.Names[i].Name] = kind
							changed = true
						}
					}
				}
			}
		}
	}
	var findings []string
	for path, f := range files {
		if filepath.ToSlash(filepath.Dir(path)) == "internal/bead" {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			kind := resolve(path, f, call.Fun)
			if kind == 0 {
				return true
			}
			args := call.Args
			if kind == 2 {
				if len(args) == 0 || literal(args[0]) != "bd" {
					return true
				}
				args = args[1:]
			}
			if len(args) == 0 || (literal(args[0]) != "ready" && literal(args[0]) != "list") {
				return true
			}
			line := fset.Position(call.Pos()).Line
			for _, group := range f.Comments {
				for _, c := range group.List {
					cl := fset.Position(c.Pos()).Line
					if (cl == line || cl == line-1) && strings.HasPrefix(c.Text, "// rowcap-allowlisted: ") && strings.TrimSpace(strings.TrimPrefix(c.Text, "// rowcap-allowlisted: ")) != "" {
						return true
					}
				}
			}
			findings = append(findings, fmt.Sprintf("%s:%d: %s read bypasses bead JSON waist", path, line, literal(args[0])))
			return true
		})
	}
	sort.Strings(findings)
	return findings, nil
}

func TestBDRowCapWaist(t *testing.T) {
	root := filepath.Join("..", "..")
	sources := map[string][]byte{}
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err == nil {
				sources[rel] = src
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	findings, err := rowcapFindings(sources)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		t.Error(f)
	}
	for _, fixture := range []string{"direct", "funcvar", "execbd"} {
		t.Run(fixture, func(t *testing.T) {
			src, err := os.ReadFile("testdata/rowcap_" + fixture + ".go.txt")
			if err != nil {
				t.Fatal(err)
			}
			bad, err := rowcapFindings(map[string][]byte{"internal/probe/probe.go": src})
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if fixture == "funcvar" {
				want = 2 // Both RunBD and RunBDCombined aliases must resolve.
			}
			if len(bad) != want {
				t.Fatalf("expected %d injected row-cap bypasses, got %v", want, bad)
			}
			t.Logf("injected bypass rejected: %v", bad)
		})
	}
	t.Run("allowed", func(t *testing.T) {
		src := []byte(`package probe
import "github.com/mrmaxsteel/mindspec/internal/bead"
var keys = map[string]bool{"list": true}
func probe(sandbox Sandbox) {
// rowcap-allowlisted: probe reads no rows
bead.RunBD("list", "--json")
bead.RunBDCombined("ready", "--json") // rowcap-allowlisted: probe reads no rows
sandbox.runBD("ready", "--json")
}`)
		got, err := rowcapFindings(map[string][]byte{"internal/probe/probe.go": src})
		if err != nil || len(got) != 0 {
			t.Fatalf("allowed sources: %v, %v", got, err)
		}
	})
	t.Run("cross-file-alias", func(t *testing.T) {
		got, err := rowcapFindings(map[string][]byte{
			"internal/probe/seam.go": []byte(`package probe
import b "github.com/mrmaxsteel/mindspec/internal/bead"
var run = b.RunBDCombined`),
			"internal/probe/call.go": []byte(`package probe
func probe() { run("list", "--json") }`),
		})
		if err != nil || len(got) != 1 {
			t.Fatalf("cross-file alias was not detected: %v, %v", got, err)
		}
	})
}
