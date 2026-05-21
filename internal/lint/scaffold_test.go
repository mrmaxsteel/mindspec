// TestNoMoleculeScaffoldingSymbols is a regression pin for spec-089
// (ceremony collapse). The plan's converged design retired the
// molecule-step scaffolding vocabulary — `mol.pour`,
// `closeoutTargets`, and `EnsureFullyBound` — in favor of the
// flatter lifecycle introduced by F1/F3 (specs 087, 088) and
// finalized by F5 (089). A synthesis-time grep already confirmed
// none of these literals appear in the `internal/` tree; this test
// fails any future commit that reintroduces them.
//
// See spec 089 Requirement 7 and ADR-0034.
package lint

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bannedScaffoldingSymbols are the retired molecule-step literals.
// They are matched as raw byte substrings against every non-test
// `.go` file under `internal/`. (The exception is this test file
// itself, which by necessity contains the literals as test data.)
var bannedScaffoldingSymbols = []string{
	"mol.pour",
	"closeoutTargets",
	"EnsureFullyBound",
}

// TestNoMoleculeScaffoldingSymbols walks the `internal/` tree and
// fails if any banned literal reappears. The test deliberately
// scans byte substrings rather than AST nodes so it catches the
// symbols regardless of context (comments, strings, identifiers).
func TestNoMoleculeScaffoldingSymbols(t *testing.T) {
	// The test runs with cwd == internal/lint; walk one level up to
	// scan the whole internal/ tree.
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve internal/ root: %v", err)
	}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Skip this regression test itself — it necessarily
		// contains the banned literals as test data.
		if strings.HasSuffix(path, "scaffold_test.go") {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, sym := range bannedScaffoldingSymbols {
			if bytes.Contains(data, []byte(sym)) {
				t.Errorf("scaffolding symbol %q reintroduced at %s", sym, path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk internal/: %v", err)
	}
}
