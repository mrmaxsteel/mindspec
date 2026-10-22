package config

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// scanFamilyLiterals checks syntax values, never comments or test fixtures.
func scanFamilyLiterals(root string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(root, "internal/config/review_tools.yaml"))
	if err != nil {
		return nil, err
	}
	var tools []ReviewTool
	if err := yaml.Unmarshal(data, &tools); err != nil {
		return nil, err
	}
	var hits []string
	check := func(path, value string) {
		if value == "claude-code-skills" || value == "claude-code-workflow" {
			return
		}
		if path == "internal/panel/disposition_migrate.go" {
			switch value {
			case "G-codex", "G1-codex", "G2-codex", "G3-codex":
				return
			}
		}
		for _, tool := range tools {
			if strings.Contains(strings.ToLower(value), strings.ToLower(tool.Name)) {
				hits = append(hits, fmt.Sprintf("%s: %q", path, value))
				break
			}
		}
	}
	var scanData func(string, *yaml.Node)
	scanData = func(path string, n *yaml.Node) {
		if n.Kind == yaml.ScalarNode {
			check(path, n.Value)
		}
		for _, child := range n.Content {
			scanData(path, child)
		}
	}
	for _, dir := range []string{"internal/config", "internal/panel", "cmd/mindspec"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if strings.HasSuffix(rel, "_test.go") || rel == "internal/config/review_tools.yaml" {
				return nil
			}
			if dir == "cmd/mindspec" && (!strings.HasPrefix(d.Name(), "panel") || !strings.HasSuffix(d.Name(), ".go")) {
				return nil
			}
			if strings.HasSuffix(rel, ".go") {
				f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
				if err != nil {
					return err
				}
				ast.Inspect(f, func(n ast.Node) bool {
					if id, ok := n.(*ast.Ident); ok && (id.Name == "KnownModels" || id.Name == "knownModels") {
						hits = append(hits, rel+": retired model registry "+id.Name)
					}
					if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						value, err := strconv.Unquote(lit.Value)
						if err == nil {
							check(rel, value)
						}
					}
					return true
				})
			} else {
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				var node yaml.Node
				if err := yaml.Unmarshal(data, &node); err != nil {
					return err
				}
				scanData(rel, &node)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(hits)
	return hits, nil
}

func TestNoFamilyLiterals(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	hits, err := scanFamilyLiterals(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Fatalf("tool literals: %v", hits)
	}
	t.Run("planted-literal", func(t *testing.T) {
		root := t.TempDir()
		for _, dir := range []string{"internal/config", "internal/panel", "cmd/mindspec"} {
			if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
				t.Fatal(err)
			}
		}
		for path, body := range map[string]string{"internal/config/review_tools.yaml": string(reviewToolsData), "internal/panel/planted.go": "package panel\nconst planted = \"codex\"\n", "internal/panel/disposition_migrate.go": "package panel\nvar historical = map[string]string{\"G-codex\":\"G1-codex\"}\n"} {
			if err := os.WriteFile(filepath.Join(root, path), []byte(body), 0644); err != nil {
				t.Fatal(err)
			}
		}
		hits, err := scanFamilyLiterals(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 1 || hits[0] != "internal/panel/planted.go: \"codex\"" {
			t.Fatalf("scanner did not report exactly the planted literal: %v", hits)
		}
		t.Logf("scanner reports %s; historical G1-codex passes", hits[0])
	})
}
