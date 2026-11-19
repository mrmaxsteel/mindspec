package kernel_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const module = "github.com/mrmaxsteel/mindspec/"

func boundary(root string) ([]string, error) {
	var failures []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "kerneltest" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		leaf := strings.HasPrefix(filepath.ToSlash(rel), "kernelid/")
		for _, imp := range file.Imports {
			value, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			bad := leaf && strings.Contains(strings.Split(value, "/")[0], ".")
			for _, p := range []string{"internal/next", "internal/approve", "internal/complete", "cmd"} {
				if value == module+p || strings.HasPrefix(value, module+p+"/") {
					if p != "internal/next" || !strings.HasPrefix(value, module+"internal/next/kernel/") {
						bad = true
					}
				}
			}
			if bad {
				failures = append(failures, rel+": "+value)
			}
		}
		return nil
	})
	return failures, err
}
func TestS134Boundary_Imports(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	failures, err := boundary(filepath.Dir(file))
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) > 0 {
		t.Fatalf("forbidden imports: %v", failures)
	}
}
func TestS134Boundary_PlantedImports(t *testing.T) {
	for _, c := range []struct{ path, imp string }{{"planted.go", module + "internal/next"}, {"kernelid/planted.go", module + "internal/phase"}} {
		t.Run(c.path, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, c.path)
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("package planted\nimport "+strconv.Quote(c.imp)+"\n"), 0644); err != nil {
				t.Fatal(err)
			}
			found, err := boundary(root)
			if err != nil || len(found) != 1 {
				t.Fatalf("detector did not fire: %v %v", found, err)
			}
		})
	}
}
func TestS134Boundary_CIInstall(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..")
	ci, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(ci)
	test := strings.Index(text, "\n  test:")
	if test < 0 {
		t.Fatal("test job missing")
	}
	end := strings.Index(text[test:], "\n  build:")
	if end < 0 {
		t.Fatal("test job end missing")
	}
	job := text[test : test+end]
	install := strings.Index(job, "scripts/ci-install-bd.sh")
	tests := strings.Index(job, "go test")
	if install < 0 || tests < 0 || install > tests {
		t.Fatal("bd install must precede go test in the test job")
	}
	script, err := os.ReadFile(filepath.Join(root, "scripts", "ci-install-bd.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"v1.3.4/beads_1.3.4_linux_amd64.tar.gz", "175c26904bde3ff9168d2282ff6b56f17b17f60c22fa5acb3dacc9fb3465303a", "sha256sum -c", "bd-installed:"} {
		if !strings.Contains(string(script), want) {
			t.Errorf("installer missing %s", want)
		}
	}
}
