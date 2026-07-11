package panel

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// TestPanelLeafImports_StdlibPlusTermsafeOnly (Spec 116 AC7) machine-checks
// the amended internal/panel leaf invariant (ADR-0037 amendment, gate.go's
// package doc comment): the package's non-test *.go files import exactly
// ONE github.com/mrmaxsteel/mindspec-prefixed package, internal/termsafe —
// the stdlib-only, pure-string escaper — and no other internal package.
// Before Spec 116 the invariant was "imports NO internal package at all";
// this test pins the amended letter in code, the same way ADR-0030's
// executor boundary is enforced by internal/lint/boundary_test.go rather
// than by convention, so any future second internal import fails a test
// immediately rather than drifting past review.
func TestPanelLeafImports_StdlibPlusTermsafeOnly(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed to resolve this test file's path")
	}
	pkgDir := filepath.Dir(thisFile)

	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		t.Fatalf("reading %s: %v", pkgDir, err)
	}

	const modulePrefix = "github.com/mrmaxsteel/mindspec/"
	const wantOnly = modulePrefix + "internal/termsafe"

	seen := map[string]bool{}
	var nonTestFiles []string

	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		nonTestFiles = append(nonTestFiles, e.Name())

		full := filepath.Join(pkgDir, e.Name())
		f, err := parser.ParseFile(fset, full, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parsing imports of %s: %v", full, err)
		}
		for _, imp := range f.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("%s: unquoting import path %s: %v", e.Name(), imp.Path.Value, err)
			}
			if !strings.HasPrefix(path, modulePrefix) {
				continue // stdlib or third-party — the leaf invariant only constrains internal imports
			}
			seen[path] = true
			if path != wantOnly {
				t.Errorf("%s imports %s — internal/panel's leaf invariant (ADR-0037 amendment, Spec 116) permits exactly ONE internal import, %s, and no other", e.Name(), path, wantOnly)
			}
		}
	}

	if len(nonTestFiles) == 0 {
		t.Fatal("no non-test *.go files found in internal/panel — the scan found nothing to check")
	}
	if !seen[wantOnly] {
		t.Errorf("expected internal/panel to import %s (the construction-boundary escaper, Spec 116) somewhere in its non-test files, but no file did", wantOnly)
	}
	if len(seen) != 1 {
		t.Errorf("internal/panel imports %d distinct internal packages, want exactly 1 (%s): %v", len(seen), wantOnly, seen)
	}
}
