package guard

// Spec 127 bead-2 rework, O2-r2-3: DestructiveFamilyCount's own stated
// purpose ("a family added here without a corresponding AC-9(i) probe
// is caught at development time") does not hold as shipped —
// AllFamilies is pure documentation; matchGit/matchBd/matchRm never
// consult it, so a new DestructiveFamily constant plus a live matcher
// `return` case can ship with zero probes while
// len(AllFamilies)==DestructiveFamilyCount stays satisfied (verified:
// adding FamilyGitFilterBranch plus a matchGit case for it, WITHOUT
// adding it to AllFamilies, left `go build ./... && go test
// -short ./internal/guard/... ./internal/lint/...` fully green).
//
// This walks classifier.go's own AST (the bead-1 sentinel pattern,
// B-r4-3/outcome_sentinel_test.go) to make the sentinel's claim true
// rather than aspirational: (1) every package-level constant
// explicitly typed DestructiveFamily must appear in AllFamilies, and
// (2) every Family*-named identifier appearing in a return statement
// anywhere in this package's non-test files must resolve to a
// declared DestructiveFamily constant that is itself in AllFamilies —
// closing exactly the shape a real floor extension takes (a new
// const plus a new matcher case), per ADR-0035's in-diff extension
// obligation.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestAllFamilies_EveryDeclaredFamilyIsRegistered(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parsing package guard: %v", err)
	}
	pkg, ok := pkgs["guard"]
	if !ok {
		t.Fatal("package guard not found in parsed directory")
	}

	registered := map[string]bool{}
	for _, fam := range AllFamilies {
		registered[string(fam)] = true
	}

	// declaredNameToValue: every package-level DestructiveFamily-typed
	// const's identifier name -> its string value.
	declaredNameToValue := map[string]string{}
	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok || vs.Type == nil {
					continue
				}
				ident, isDF := ast.Unparen(vs.Type).(*ast.Ident)
				if !isDF || ident.Name != "DestructiveFamily" {
					continue
				}
				for i, nm := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					lit, ok := vs.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					if v, ok := unquoteFamilyLit(lit.Value); ok {
						declaredNameToValue[nm.Name] = v
					}
				}
			}
		}
	}
	if len(declaredNameToValue) == 0 {
		t.Fatal("found zero DestructiveFamily-typed const declarations — this sentinel's own probe is broken")
	}

	// (1) every declared constant must be registered in AllFamilies.
	for name, val := range declaredNameToValue {
		if !registered[val] {
			t.Errorf("DestructiveFamily constant %s (%q) is declared but NOT in AllFamilies — a family added without extending AllFamilies ships with zero AC-9(i) probes", name, val)
		}
	}

	// (2) every Family*-named identifier RETURNED anywhere in this
	// package's non-test files must resolve to a declared constant
	// that is itself in AllFamilies — catching a live matcher case
	// naming a family that was declared but never registered (the
	// on-disk mutation this test exists to red).
	for _, file := range pkg.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			ret, ok := n.(*ast.ReturnStmt)
			if !ok {
				return true
			}
			for _, r := range ret.Results {
				id, ok := r.(*ast.Ident)
				if !ok || !strings.HasPrefix(id.Name, "Family") {
					continue
				}
				val, known := declaredNameToValue[id.Name]
				if !known {
					t.Errorf("return statement names %s, which is not a DestructiveFamily constant this sentinel could resolve", id.Name)
					continue
				}
				if !registered[val] {
					t.Errorf("return statement names %s (%q), which is not in AllFamilies", id.Name, val)
				}
			}
			return true
		})
	}
}

// unquoteFamilyLit strips the surrounding quotes from a string
// literal's raw source form (a local copy of internal/lint's unquote,
// duplicated rather than imported — package guard cannot import
// internal/lint, which itself imports internal/guard).
func unquoteFamilyLit(raw string) (string, bool) {
	if len(raw) < 2 {
		return "", false
	}
	first, last := raw[0], raw[len(raw)-1]
	if (first == '"' && last == '"') || (first == '`' && last == '`') {
		return raw[1 : len(raw)-1], true
	}
	return "", false
}
