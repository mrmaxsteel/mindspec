package lint

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Each exemption names one legacy-key test. No file is exempt.
var legacyMergeStrategyTests = map[string]bool{
	"internal/config/config_test.go:TestConfigLoad_LegacyMergeStrategyIgnored":                      true,
	"internal/doctor/config_test.go:TestDoctor_LegacyMergeStrategyInfo":                             true,
	"internal/doctor/config_test.go:TestDoctor_LegacyMergeStrategyCommentedOrNested":                true,
	"internal/lint/no_merge_strategy_writers_test.go:TestNoMergeStrategyWriters_PlantedWriterFails": true,
}

var mergeStrategyWriterRE = regexp.MustCompile(`(?m)(?:^|[\n{,])[ \t]*["']?merge_strategy["']?[ \t]*:`)
var mergeStrategyTagRE = regexp.MustCompile(`yaml:"merge_strategy(?:,|")`)

func scanMergeStrategyWriters(root string) ([]string, error) {
	var findings []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".worktrees", ".beads", "vendor", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		isGo := strings.HasSuffix(rel, ".go")
		isFixture := false
		for _, part := range strings.Split(rel, "/") {
			if part == "testdata" || part == "fixtures" {
				isFixture = true
			}
		}
		if !isGo && !isFixture && rel != ".mindspec/config.yaml" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !isGo {
			for _, loc := range mergeStrategyWriterRE.FindAllIndex(data, -1) {
				line := 1 + strings.Count(string(data[:loc[0]]), "\n")
				findings = append(findings, fmt.Sprintf("%s:%d", rel, line))
			}
			return nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, data, 0)
		if err != nil {
			return err
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && legacyMergeStrategyTests[rel+":"+fn.Name.Name] {
				continue
			}
			ast.Inspect(decl, func(node ast.Node) bool {
				lit, ok := node.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				value, err := strconv.Unquote(lit.Value)
				if err == nil && (mergeStrategyWriterRE.MatchString(value) || mergeStrategyTagRE.MatchString(value)) {
					findings = append(findings, fmt.Sprintf("%s:%d", rel, fset.Position(lit.Pos()).Line))
				}
				return true
			})
		}
		return nil
	})
	sort.Strings(findings)
	return findings, err
}

func TestNoMergeStrategyWriters(t *testing.T) {
	findings, err := scanMergeStrategyWriters("../..")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("removed config key has writers:\n%s", strings.Join(findings, "\n"))
	}
}

func TestNoMergeStrategyWriters_PlantedWriterFails(t *testing.T) {
	root := t.TempDir()
	// A permitted legacy test does not exempt another writer in the same file.
	files := map[string]string{
		"internal/config/config_test.go":        "package config\nfunc TestConfigLoad_LegacyMergeStrategyIgnored() { _ = \"merge_strategy: pr\\n\" }\nconst ordinaryConfig = \"merge_strategy: direct\\n\"\n",
		"internal/example/testdata/config.yaml": "merge_strategy: pr\n",
		".mindspec/config.yaml":                 "merge_strategy: auto\n",
	}
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	findings, err := scanMergeStrategyWriters(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".mindspec/config.yaml:1", "internal/config/config_test.go:3", "internal/example/testdata/config.yaml:1"}
	if !reflect.DeepEqual(findings, want) {
		t.Fatalf("want three planted writers %v, got %v", want, findings)
	}
}
