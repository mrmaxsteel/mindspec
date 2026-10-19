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
			if p == "os/exec" && s.Sel.Name == "CommandContext" {
				return 3
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
			if kind == 3 {
				if len(args) == 0 {
					return true
				}
				args = args[1:] // CommandContext's first argument is the context.
			}
			if kind == 2 || kind == 3 {
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

// rowcapSources walks the whole module, including root-level and plugin code.
func rowcapSources(root string) (map[string][]byte, error) {
	sources := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root {
				switch d.Name() {
				case "vendor":
					// Vendored dependencies belong to their upstream modules.
					return filepath.SkipDir
				case "testdata":
					// Test fixtures are not production sources.
					return filepath.SkipDir
				case ".worktrees":
					// Nested checkouts are independent copies of the module.
					return filepath.SkipDir
				case "generated", "gen":
					// Generated output trees are governed at their generator source.
					return filepath.SkipDir
				}
				if strings.HasPrefix(d.Name(), ".") {
					// Hidden directories contain metadata and local tool state.
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
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
	return sources, err
}

func TestBDRowCapWaist(t *testing.T) {
	sources, err := rowcapSources(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
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
		for _, fixture := range []struct {
			name, path, source string
		}{
			{"marked", "internal/probe/probe.go", `package probe
import "github.com/mrmaxsteel/mindspec/internal/bead"
func probe() {
// rowcap-allowlisted: probe reads no rows
bead.RunBD("list", "--json")
bead.RunBDCombined("ready", "--json") // rowcap-allowlisted: probe reads no rows
}`},
			{"redact-map-key", "internal/redact/redact.go", `package redact
var keys = map[string]bool{"list": true}`},
			{"sandbox-method", "internal/harness/probe.go", `package harness
func probe(sandbox Sandbox) { sandbox.runBD("ready", "--json") }`},
		} {
			t.Run(fixture.name, func(t *testing.T) {
				got, err := rowcapFindings(map[string][]byte{fixture.path: []byte(fixture.source)})
				if err != nil || len(got) != 0 {
					t.Fatalf("allowed %s: %v, %v", fixture.name, got, err)
				}
			})
		}
	})
	t.Run("allowlist-empty-reason", func(t *testing.T) {
		for _, reason := range []string{"", " ", " \t  "} {
			for _, placement := range []string{"above", "inline"} {
				t.Run(fmt.Sprintf("%q/%s", reason, placement), func(t *testing.T) {
					marker := "// rowcap-allowlisted:" + reason
					call := `bead.RunBD("list", "--json")`
					body := marker + "\n" + call
					if placement == "inline" {
						body = call + " " + marker
					}
					src := "package probe\nimport \"github.com/mrmaxsteel/mindspec/internal/bead\"\nfunc probe() {\n" + body + "\n}"
					got, err := rowcapFindings(map[string][]byte{"internal/probe/probe.go": []byte(src)})
					if err != nil || len(got) != 1 {
						t.Fatalf("empty allowlist reason must report the row read: got %v, %v", got, err)
					}
				})
			}
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

func TestBDRowCapWaist_Walker(t *testing.T) {
	newRoot := func(t *testing.T) string {
		t.Helper()
		root := t.TempDir()
		// Keep both old walk roots present so a restricted walker fails on
		// the missed call, rather than an unrelated missing-directory error.
		for _, dir := range []string{"internal", "cmd"} {
			if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
				t.Fatal(err)
			}
		}
		return root
	}
	for _, name := range []string{"root.go", "plugins/probe/probe.go", "tools/probe.go", "internal/probe/probe.go", "cmd/probe/probe.go", "generated-marker.go"} {
		t.Run(name, func(t *testing.T) {
			root := newRoot(t)
			path := filepath.Join(root, name)
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			src := []byte(`package probe
import "github.com/mrmaxsteel/mindspec/internal/bead"
func probe() { bead.RunBD("list", "--json") }`)
			if name == "generated-marker.go" {
				// Only named generated trees are excluded; a marker alone must
				// not hide an otherwise in-scope production source.
				src = append([]byte("// Code generated by fixture; DO NOT EDIT.\n"), src...)
			}
			if err := os.WriteFile(path, src, 0644); err != nil {
				t.Fatal(err)
			}
			sources, err := rowcapSources(root)
			if err != nil {
				t.Fatal(err)
			}
			bad, err := rowcapFindings(sources)
			if err != nil || len(bad) != 1 {
				t.Fatalf("walker must flag unmarked row read at %s: got %v, %v", name, bad, err)
			}
		})
	}
	t.Run("CommandContext", func(t *testing.T) {
		root := newRoot(t)
		src := []byte(`package probe
import "os/exec"
var command = exec.CommandContext
func probe() {
exec.CommandContext(ctx, "bd", "ready", "--json")
exec.CommandContext(ctx, "bd", "list", "--json")
command(ctx, "bd", "ready", "--json")
exec.CommandContext(ctx, "echo", "list")
// rowcap-allowlisted: probe reads no rows
exec.CommandContext(ctx, "bd", "list")
}`)
		if err := os.WriteFile(filepath.Join(root, "internal", "probe.go"), src, 0644); err != nil {
			t.Fatal(err)
		}
		sources, err := rowcapSources(root)
		if err != nil {
			t.Fatal(err)
		}
		bad, err := rowcapFindings(sources)
		if err != nil || len(bad) != 3 {
			t.Fatalf("walker must flag direct and aliased CommandContext bypasses: got %v, %v", bad, err)
		}
	})
	t.Run("excluded-trees", func(t *testing.T) {
		root := t.TempDir()
		for _, name := range []string{"vendor/probe.go", "testdata/probe.go", ".worktrees/probe.go", ".hidden/probe.go", "generated/probe.go", "gen/probe.go", "probe_test.go"} {
			path := filepath.Join(root, name)
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			src := []byte("invalid Go fixture must not be parsed")
			if err := os.WriteFile(path, src, 0644); err != nil {
				t.Fatal(err)
			}
		}
		sources, err := rowcapSources(root)
		if err != nil || len(sources) != 0 {
			t.Fatalf("excluded sources: %v, %v", sources, err)
		}
	})
}
