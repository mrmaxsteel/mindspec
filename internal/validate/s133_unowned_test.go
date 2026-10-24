package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
)

func s133AssertUnowned(t *testing.T, r *Result, path string) {
	t.Helper()
	if r.HasFailures() {
		t.Fatalf("unowned must not fail: %+v", r.Issues)
	}
	count := 0
	for _, i := range r.Issues {
		if i.Name == "adr-divergence-unowned" && strings.Contains(i.Message, path) {
			count++
			if i.Severity != SevWarning || !strings.Contains(i.Message, "domains") {
				t.Fatalf("need unowned Warn with domains root: %+v", i)
			}
		}
	}
	if count != 1 {
		t.Fatalf("need one unowned Warn for %s: %+v", path, r.Issues)
	}
}

func s133UndeclaredControl(t *testing.T, root, specDir string) {
	t.Helper()
	specDir = filepath.Join(specDir, "scope-control")
	writeSpecAndPlan(t, root, specDir, "999-control", []string{"control"}, nil)
	const path = "internal/scope-control/file.go"
	manifest := "paths:\n  - internal/scope-control/**\n"
	if _, err := os.Stat(filepath.Join(root, ".mindspec/domains")); err == nil {
		writeFlatManifest(t, root, "outside", manifest)
	} else {
		writeManifest(t, root, "outside", manifest)
	}
	r, _ := ValidateDivergence(&executor.MockExecutor{ChangedFilesResult: []string{path}}, root, specDir, "", "BASE", "HEAD", "", false)
	for _, i := range r.Issues {
		if i.Name == "adr-divergence-unowned" && i.Severity == SevError && strings.Contains(i.Message, path) && strings.Contains(i.Message, "not in the spec") {
			return
		}
	}
	t.Fatalf("undeclared owner must remain Error: %+v", r.Issues)
}

func TestS133UnownedWarn_OutcomesAndFindings(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, impl := range []bool{false, true} {
			for _, kind := range []string{"unowned", "undeclared", "attribute", "uncovered"} {
				t.Run(policy+"/"+map[bool]string{true: "impl", false: "bead"}[impl]+"/"+kind, func(t *testing.T) {
					root := t.TempDir()
					specDir := filepath.Join(root, ".mindspec/docs/specs/999-unowned")
					writeSpecAndPlan(t, root, specDir, "999-unowned", []string{"core"}, nil)
					writeManifest(t, root, "core", "paths:\n  - internal/core/**\n")
					if err := os.WriteFile(filepath.Join(root, ".mindspec/config.yaml"), []byte("enforcement:\n  panel_presence: "+policy+"\n"), 0644); err != nil {
						t.Fatal(err)
					}
					config.ResetCache()
					t.Cleanup(config.ResetCache)
					path := "CHANGELOG.md"
					name := "adr-divergence-unowned"
					switch kind {
					case "undeclared":
						path = "internal/other/file.go"
						writeManifest(t, root, "other", "paths:\n  - internal/other/**\n")
					case "attribute":
						path = "internal/core/file.go"
						name = "adr-divergence-attribute"
						writeManifest(t, root, "core", "paths: [ broken\n")
					case "uncovered":
						path = "internal/core/file.go"
						name = "adr-divergence-uncovered"
					}
					r, findings := ValidateDivergence(&executor.MockExecutor{ChangedFilesResult: []string{path}}, root, specDir, "bead-1", "BASE", "HEAD", "", impl)
					if kind == "unowned" {
						s133AssertUnowned(t, r, path)
						if len(findings) != 1 || findings[0].Kind != "unowned" || findings[0].Path != path {
							t.Fatalf("warning lost structured finding: %+v", findings)
						}
					} else {
						if !r.HasFailures() {
							t.Fatalf("%s must remain Error: %+v", kind, r.Issues)
						}
						found := false
						for _, i := range r.Issues {
							if i.Name == name && i.Severity == SevError && strings.Contains(i.Message, path) {
								found = true
							}
						}
						if !found {
							t.Fatalf("missing named Error %s: %+v", name, r.Issues)
						}
					}
				})
			}
		}
	}
}
