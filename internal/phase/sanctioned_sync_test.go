package phase

// sanctioned_sync_test.go — the R3.3 (round-2 G3.1) call-site enumeration
// fence for SanctionedDerivedSync, following the `impl adopt` house
// pattern (cmd/mindspec/impl_adopt_test.go's
// TestAdoptSpec_OnlyDirectProductionCallSiteIsItsCommandHandler): an
// AST walk over every non-test .go file under cmd/ and internal/,
// matching CallExprs whose function resolves — by IMPORT PATH IDENTITY,
// never by the literal source-level spelling of a local qualifier — to
// phase.SanctionedDerivedSync (or, from within this package itself, the
// bare identifier).
//
// Bead 1's form of this fence asserts every found call site is inside
// the allowlist {cmd/mindspec/release.go} — satisfiable NOW, before bead
// 2 wires the real caller, because zero call sites trivially satisfy
// "found ⊆ allowlist". Bead 2 tightens this to "exactly one call site,
// and it is cmd/mindspec/release.go's syncPhase" when it adds the real
// wiring — see this file's doc comment for the tightening instruction so
// bead 2 does not need to rediscover the pattern.
//
// Like the adopt fence, this is syntax-shaped, not object/call-graph
// based: it cannot prove SanctionedDerivedSync is never reached through
// a function value, method value, or interface — only that no DIRECT
// `pkg.SanctionedDerivedSync(...)` (or dot-imported/bare) call exists
// outside the allowlist. That residual is review-caught, not mechanical,
// exactly as the adopt fence's own doc comment states.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// sanctionedSyncAllowlist names the only files a production call site of
// SanctionedDerivedSync may live in. Bead 1: empty of REAL callers (the
// allowlist exists so a future addition is deliberate, not to claim one
// exists yet) — bead 2 wires cmd/mindspec/release.go as the sole entry
// and tightens the assertion below from "found ⊆ allowlist" to
// "exactly one, and it is this file".
var sanctionedSyncAllowlist = map[string]bool{
	"cmd/mindspec/release.go": true,
}

// phaseImportPath is this package's own import path, used to resolve
// cross-package call sites by identity (O2-1/G1-B3-05 discipline: never
// match on the literal spelling of a local qualifier).
const phaseImportPath = "github.com/mrmaxsteel/mindspec/internal/phase"

func TestSanctionedDerivedSync_NoCallersOutsideAllowlist(t *testing.T) {
	root := phasePkgTestRepoRoot(t)
	var sites []sanctionedSyncCallSite
	for _, top := range []string{"cmd", "internal"} {
		dir := filepath.Join(root, top)
		if err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			sites = append(sites, findSanctionedDerivedSyncCallSites(t, root, path)...)
			return nil
		}); err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
	}
	for _, s := range sites {
		if !sanctionedSyncAllowlist[s.file] {
			t.Errorf("production call site of SanctionedDerivedSync found outside the allowlist: %s (func %s) — every caller must be a sanctioned, guard-routed internal sync, never an ad-hoc bypass of the mindspec repair phase operator guard", s.file, s.funcName)
		}
	}
}

type sanctionedSyncCallSite struct {
	file, funcName string
}

// findSanctionedDerivedSyncCallSites parses absPath and returns one
// sanctionedSyncCallSite per call expression resolving — by import
// identity — to phase.SanctionedDerivedSync, tagged with the name of its
// nearest enclosing top-level function/method (or "<package-level>").
// A file whose package IS "phase" itself matches a bare
// `SanctionedDerivedSync(...)` call (no qualifier needed, same package).
func findSanctionedDerivedSyncCallSites(t *testing.T, root, absPath string) []sanctionedSyncCallSite {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, absPath, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", absPath, err)
	}
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		t.Fatalf("relativizing %s: %v", absPath, err)
	}

	samePackage := file.Name != nil && file.Name.Name == "phase"
	qualifiedNames := map[string]bool{}
	dotImported := false
	for _, imp := range file.Imports {
		path, unquoteErr := strconv.Unquote(imp.Path.Value)
		if unquoteErr != nil || path != phaseImportPath {
			continue
		}
		switch {
		case imp.Name == nil:
			qualifiedNames["phase"] = true
		case imp.Name.Name == ".":
			dotImported = true
		case imp.Name.Name == "_":
			// Blank import: no identifier can ever call through it.
		default:
			qualifiedNames[imp.Name.Name] = true
		}
	}

	var sites []sanctionedSyncCallSite
	var funcStack []string
	var visit func(n ast.Node) bool
	visit = func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.FuncDecl:
			name := v.Name.Name
			if v.Recv != nil && len(v.Recv.List) == 1 {
				if starExpr, ok := v.Recv.List[0].Type.(*ast.StarExpr); ok {
					if id, ok := starExpr.X.(*ast.Ident); ok {
						name = id.Name + "." + name
					}
				} else if id, ok := v.Recv.List[0].Type.(*ast.Ident); ok {
					name = id.Name + "." + name
				}
			}
			funcStack = append(funcStack, name)
			ast.Inspect(v.Body, visit)
			funcStack = funcStack[:len(funcStack)-1]
			return false
		case *ast.CallExpr:
			matched := false
			switch fn := v.Fun.(type) {
			case *ast.SelectorExpr:
				if pkgIdent, ok := fn.X.(*ast.Ident); ok && qualifiedNames[pkgIdent.Name] && fn.Sel.Name == "SanctionedDerivedSync" {
					matched = true
				}
			case *ast.Ident:
				if fn.Name != "SanctionedDerivedSync" {
					break
				}
				if dotImported || samePackage {
					matched = true
				}
			}
			if matched {
				name := "<package-level>"
				if len(funcStack) > 0 {
					name = funcStack[len(funcStack)-1]
				}
				// The function's own definition is a FuncDecl, never a
				// CallExpr, so this cannot self-match sanctioned_sync.go's
				// declaration of SanctionedDerivedSync.
				sites = append(sites, sanctionedSyncCallSite{file: rel, funcName: name})
			}
		}
		return true
	}
	ast.Inspect(file, visit)
	return sites
}

// phasePkgTestRepoRoot returns the mindspec repository root by walking up
// from the test's working directory until it finds go.mod with the
// mindspec module path. Mirrors cmd/mindspec/testhelpers_test.go's
// repoRootFromTestDir (kept as a separate copy: importing the cmd/
// package's test helper from internal/phase would cycle).
func phasePkgTestRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for {
		gm := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(gm); err == nil {
			if strings.Contains(string(data), "module github.com/mrmaxsteel/mindspec") {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate repo root (go.mod with mindspec module) walking up from " + wd)
		}
		dir = parent
	}
}
