package setup

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/rules"
)

// These frozen bodies are digest oracles. Setup never writes them as new content.
var productSurfaceEmbedExemptions = []string{
	"internal/setup/historical_skills/",
	"internal/setup/historical_workflows/",
}

var productSurfaceInternalRef = regexp.MustCompile(`ADR-[0-9]|[Ss]pec [0-9]{3}|\bR[0-9]+\.[0-9]|#[0-9]{3}|\bmindspec-[a-z0-9]{4}(?:\.[0-9]+)?\b`)
var productSurfaceEmbedWord = regexp.MustCompile("\"(?:[^\"\\\\]|\\\\.)*\"|`[^`]*`|[^\\s]+")

func productSurfaceScanFile(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return productSurfaceInternalRef.FindAllString(string(raw), -1), nil
}

// productSurfaceEmbeddedFiles resolves the data named by actual embed directives.
// It also scans plugin sources, which embed the skills installed by setup.
func productSurfaceEmbeddedFiles(root string) ([]string, error) {
	files := map[string]bool{}
	for _, dir := range []string{"internal", "plugins"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
			if err != nil {
				return err
			}
			for _, group := range parsed.Comments {
				for _, comment := range group.List {
					body, ok := strings.CutPrefix(comment.Text, "//go:embed")
					if !ok || len(body) == 0 || (body[0] != ' ' && body[0] != '\t') {
						continue
					}
					body = strings.TrimLeft(body, " \t")
					for _, word := range productSurfaceEmbedWord.FindAllString(body, -1) {
						if strings.HasPrefix(word, "\"") || strings.HasPrefix(word, "`") {
							word, err = strconv.Unquote(word)
							if err != nil {
								return err
							}
						}
						pattern, includeHidden := strings.CutPrefix(word, "all:")
						matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), filepath.FromSlash(pattern)))
						if err != nil {
							return err
						}
						if len(matches) == 0 {
							return fmt.Errorf("%s: embed pattern %q matched no files", path, word)
						}
						for _, match := range matches {
							err := filepath.WalkDir(match, func(dataPath string, dataEntry fs.DirEntry, dataErr error) error {
								if dataErr != nil {
									return dataErr
								}
								if dataPath != match && !includeHidden && strings.ContainsAny(dataEntry.Name()[:1], "._") {
									if dataEntry.IsDir() {
										return filepath.SkipDir
									}
									return nil
								}
								if !dataEntry.IsDir() {
									files[dataPath] = true
								}
								return nil
							})
							if err != nil {
								return err
							}
						}
					}
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	var selected []string
	for path := range files {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil, err
		}
		exempt := false
		for _, prefix := range productSurfaceEmbedExemptions {
			if strings.HasPrefix(filepath.ToSlash(rel), prefix) {
				exempt = true
			}
		}
		if !exempt {
			selected = append(selected, path)
		}
	}
	sort.Strings(selected)
	return selected, nil
}

func TestProductSurface_NoInternalRefs(t *testing.T) {
	t.Run("planted-R7.3", func(t *testing.T) {
		for _, tc := range []struct {
			name, whitespace string
		}{
			{"space", " "},
			{"tab", "\t"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				root := t.TempDir()
				for _, dir := range []string{"internal/data", "plugins"} {
					if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
						t.Fatal(err)
					}
				}
				path := filepath.Join(root, "internal/data/fixture.txt")
				if err := os.WriteFile(path, []byte("The planted reference is R7.3."), 0644); err != nil {
					t.Fatal(err)
				}
				source := "package data\nimport _ \"embed\"\n//go:embed" + tc.whitespace + "\"fixture.txt\"\nvar fixture string\n"
				if err := os.WriteFile(filepath.Join(root, "internal/data/data.go"), []byte(source), 0644); err != nil {
					t.Fatal(err)
				}
				files, err := productSurfaceEmbeddedFiles(root)
				if err != nil || len(files) != 1 || files[0] != path {
					t.Fatalf("planted embed not discovered: %v, %v", files, err)
				}
				hits, err := productSurfaceScanFile(files[0])
				if err != nil || len(hits) != 1 || hits[0] != "R7.3" {
					t.Fatalf("planted reference not detected: %v, %v", hits, err)
				}
			})
		}
	})
	t.Run("all-embedded-data", func(t *testing.T) {
		root, err := filepath.Abs("../..")
		if err != nil {
			t.Fatal(err)
		}
		files, err := productSurfaceEmbeddedFiles(root)
		if err != nil {
			t.Fatal(err)
		}
		foundRules, foundTemplates, foundSkills := false, false, false
		for _, path := range files {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				t.Fatal(err)
			}
			rel = filepath.ToSlash(rel)
			foundRules = foundRules || rel == "internal/rules/rules.yaml"
			foundTemplates = foundTemplates || strings.HasPrefix(rel, "internal/instruct/templates/")
			foundSkills = foundSkills || strings.HasPrefix(rel, "plugins/mindspec/skills/")
			hits, err := productSurfaceScanFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(hits) != 0 {
				t.Errorf("%s contains internal references: %v", rel, hits)
			}
		}
		if !foundRules || !foundTemplates || !foundSkills {
			t.Fatalf("missing embedded populations: rules=%v templates=%v skills=%v", foundRules, foundTemplates, foundSkills)
		}
	})
	t.Run("all-rule-fields", func(t *testing.T) {
		table, err := rules.Load()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(table)
		if err != nil {
			t.Fatal(err)
		}
		if hits := productSurfaceInternalRef.FindAll(raw, -1); len(hits) != 0 {
			t.Fatalf("rule fields contain internal references: %q", hits)
		}
		for _, planted := range []string{"ADR-0045", "Spec 131", "spec 131", "R7.3", "#253", "mindspec-zadp.2"} {
			if !productSurfaceInternalRef.MatchString(planted) {
				t.Errorf("pattern failed to detect %q", planted)
			}
		}
	})
}
