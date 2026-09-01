package phase

// writer_inventory_test.go — AC-5(vi), the R3.3 anti-drift test: every
// `MergeMetadata`-family call that writes the mindspec_phase KEY must
// execute inside WithPhaseWriteLock's critical section. Introducing a new
// unlocked writer must be RED.
//
// Mechanism (grep/seam-anchored, per the plan): every existing writer in
// this bead is refactored so its `map[string]interface{}{"mindspec_phase":
// ...}` composite literal lives inside a func literal passed as an
// argument to a phase.WithPhaseWriteLock (or, from within this package,
// SanctionedDerivedSync's own nested WithPhaseWriteLock) call — the ACTUAL
// bead.MergeMetadata-family invocation now happens ONLY inside
// WithPhaseWriteLock's own body (phaselock.go), never at the call sites
// themselves. So the anti-drift invariant this test enforces is
// structural: an AST walk finds every `"mindspec_phase"` key/value pair in
// a composite literal across cmd/ and internal/ (excluding this package's
// own lock/order/migrate/sanctioned-sync plumbing and test files), and
// asserts each one lies within the syntactic extent of a CallExpr that
// resolves — by import-path identity, never literal qualifier spelling —
// to phase.WithPhaseWriteLock. A future writer that calls
// `bead.MergeMetadata(id, map[string]interface{}{"mindspec_phase": ...})`
// (or any *MergeMetadataFn seam) OUTSIDE such a call is red.
//
// The literal string composed via fmt.Sprintf in spec.go's epic-CREATION
// seed (`spec.go:84`, building a JSON string for `bd create --metadata`)
// is NOT a Go composite literal and is correctly invisible to this AST
// walk — matching the spec's own stated exclusion for that seed (there is
// no stored value to race when the epic does not exist yet).

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

const mindspecPhaseKey = "mindspec_phase"

func TestMindspecPhaseWritesAllLockGuarded(t *testing.T) {
	root := phasePkgTestRepoRoot(t)
	var violations []string
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
			violations = append(violations, findUnguardedPhaseWrites(t, root, path)...)
			return nil
		}); err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
	}
	if len(violations) > 0 {
		t.Fatalf("mindspec_phase written outside WithPhaseWriteLock's critical section:\n  %s", strings.Join(violations, "\n  "))
	}
}

// findUnguardedPhaseWrites parses absPath and returns one description
// string per `"mindspec_phase"` composite-literal key/value pair that is
// NOT lexically nested inside a CallExpr resolving to
// phase.WithPhaseWriteLock (by import-path identity, or a bare
// unqualified call when the file's own package is "phase").
func findUnguardedPhaseWrites(t *testing.T, root, absPath string) []string {
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
		default:
			qualifiedNames[imp.Name.Name] = true
		}
	}

	isLockCall := func(call *ast.CallExpr) bool {
		switch fn := call.Fun.(type) {
		case *ast.SelectorExpr:
			if pkgIdent, ok := fn.X.(*ast.Ident); ok && qualifiedNames[pkgIdent.Name] && fn.Sel.Name == "WithPhaseWriteLock" {
				return true
			}
		case *ast.Ident:
			if fn.Name == "WithPhaseWriteLock" && (dotImported || samePackage) {
				return true
			}
		}
		return false
	}

	isPhaseKey := func(n ast.Node) bool {
		kv, ok := n.(*ast.KeyValueExpr)
		if !ok {
			return false
		}
		lit, ok := kv.Key.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return false
		}
		val, err := strconv.Unquote(lit.Value)
		return err == nil && val == mindspecPhaseKey
	}

	guarded := map[token.Pos]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isLockCall(call) {
			return true
		}
		ast.Inspect(call, func(inner ast.Node) bool {
			if isPhaseKey(inner) {
				guarded[inner.Pos()] = true
			}
			return true
		})
		return true
	})

	var funcStack []string
	var violations []string
	var visit func(n ast.Node) bool
	visit = func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.FuncDecl:
			funcStack = append(funcStack, v.Name.Name)
			ast.Inspect(v.Body, visit)
			funcStack = funcStack[:len(funcStack)-1]
			return false
		default:
			if isPhaseKey(n) && !guarded[n.Pos()] {
				name := "<package-level>"
				if len(funcStack) > 0 {
					name = funcStack[len(funcStack)-1]
				}
				pos := fset.Position(n.Pos())
				violations = append(violations, rel+":"+strconv.Itoa(pos.Line)+" (func "+name+")")
			}
		}
		return true
	}
	ast.Inspect(file, visit)
	return violations
}
