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
// structural: an AST walk finds every mindspec_phase KEY WRITE across
// cmd/ and internal/ (excluding this package's own lock/order/migrate/
// sanctioned-sync plumbing and test files), and asserts each one lies
// within the syntactic extent of a CallExpr that resolves — by
// import-path identity, never literal qualifier spelling — to
// phase.WithPhaseWriteLock. A future writer that calls
// `bead.MergeMetadata(id, map[string]interface{}{"mindspec_phase": ...})`
// (or any *MergeMetadataFn seam) OUTSIDE such a call is red.
//
// Round-1 panel G5 hardening: "mindspec_phase KEY WRITE" is recognized in
// THREE shapes, not just the original bare-string composite literal:
//  1. `map[string]interface{}{"mindspec_phase": ...}` — the original
//     composite-literal anchor.
//  2. `m["mindspec_phase"] = ...` — a map-INDEX ASSIGNMENT (an lvalue
//     IndexExpr on the left of an AssignStmt). An rvalue IndexExpr READ
//     (`x := m["mindspec_phase"]`) is deliberately NOT flagged — reading
//     the current value to decide something is not a write, and this
//     package's own guarded writers routinely read `stored` this way.
//  3. A composite-literal key, or an index-assignment's index, that is a
//     bare Ident resolving to a FILE-LOCAL `const X = "mindspec_phase"`
//     declaration — not only a literal string.
//
// Stated, honest residual (round-1 panel G5's OR-clause: partial
// hardening + a narrowed claim, rather than a false total guarantee):
// this walker does NOT trace arbitrary data flow — an update map built
// by a HELPER FUNCTION and passed opaquely into a MergeMetadata-family
// call (e.g. `bead.MergeMetadata(id, buildPhaseUpdate())`) is invisible
// to it, and a package-level (not file-local) const, or a key derived
// from string concatenation/formatting, are equally unresolved. Closing
// that class requires whole-program data-flow analysis this test does
// not attempt; the three shapes above are the ones a developer
// ACCIDENTALLY reintroducing an unlocked writer is actually likely to
// write (the accidental-drift threat model this test exists for), not
// an adversarial-evasion guarantee.
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

// resolveFileConstStrings maps file-local const identifier names to
// their literal string values, for single-value `const X = "..."`
// declarations (round-1 panel G5-b). Only direct BasicLit STRING values
// are resolved — a const built from another const or an expression is
// out of this walker's stated residual.
func resolveFileConstStrings(file *ast.File) map[string]string {
	out := map[string]string{}
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) != len(vs.Values) {
				continue
			}
			for i, name := range vs.Names {
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				if val, err := strconv.Unquote(lit.Value); err == nil {
					out[name.Name] = val
				}
			}
		}
	}
	return out
}

// resolvesToMindspecPhaseKey reports whether e is either the literal
// string "mindspec_phase" or a bare Ident bound to a file-local const of
// that value (round-1 panel G5-b).
func resolvesToMindspecPhaseKey(e ast.Expr, constStrs map[string]string) bool {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return false
		}
		val, err := strconv.Unquote(v.Value)
		return err == nil && val == mindspecPhaseKey
	case *ast.Ident:
		return constStrs[v.Name] == mindspecPhaseKey
	}
	return false
}

// findUnguardedPhaseWrites parses absPath and returns one description
// string per mindspec_phase KEY WRITE (composite-literal key, or
// map-index assignment — see the file doc comment for the three
// recognized shapes) that is NOT lexically nested inside a CallExpr
// resolving to phase.WithPhaseWriteLock (by import-path identity, or a
// bare unqualified call when the file's own package is "phase").
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

	constStrs := resolveFileConstStrings(file)

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

	// isPhaseKey matches shape 1 (composite-literal key) and shape 3
	// (const-bound key) together, since resolvesToMindspecPhaseKey
	// already handles both.
	isPhaseKey := func(n ast.Node) bool {
		kv, ok := n.(*ast.KeyValueExpr)
		if !ok {
			return false
		}
		return resolvesToMindspecPhaseKey(kv.Key, constStrs)
	}

	// isPhaseIndexWrite matches shape 2: an AssignStmt whose LHS is an
	// IndexExpr keyed by "mindspec_phase" (literal or const-bound) — an
	// lvalue WRITE, never an rvalue READ (`x := m["mindspec_phase"]` is
	// a *ast.AssignStmt too, but its "mindspec_phase" IndexExpr sits on
	// the RHS, which this only inspects the LHS of).
	isPhaseIndexWrite := func(n ast.Node) (token.Pos, bool) {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return 0, false
		}
		for _, lhs := range assign.Lhs {
			idx, ok := lhs.(*ast.IndexExpr)
			if !ok {
				continue
			}
			if resolvesToMindspecPhaseKey(idx.Index, constStrs) {
				return idx.Pos(), true
			}
		}
		return 0, false
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
			if pos, ok := isPhaseIndexWrite(inner); ok {
				guarded[pos] = true
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
			enclosingFunc := func() string {
				if len(funcStack) > 0 {
					return funcStack[len(funcStack)-1]
				}
				return "<package-level>"
			}
			if isPhaseKey(n) && !guarded[n.Pos()] {
				pos := fset.Position(n.Pos())
				violations = append(violations, rel+":"+strconv.Itoa(pos.Line)+" (func "+enclosingFunc()+")")
			}
			if idxPos, ok := isPhaseIndexWrite(n); ok && !guarded[idxPos] {
				pos := fset.Position(idxPos)
				violations = append(violations, rel+":"+strconv.Itoa(pos.Line)+" (func "+enclosingFunc()+") [map-index write]")
			}
		}
		return true
	}
	ast.Inspect(file, visit)
	return violations
}

// TestFindUnguardedPhaseWrites_CatchesIndexAssignmentEvasion is round-1
// panel G5's own red-on-new-unlocked-writer proof for shape 2: a
// map-index ASSIGNMENT outside any WithPhaseWriteLock call must be
// flagged.
func TestFindUnguardedPhaseWrites_CatchesIndexAssignmentEvasion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "probe.go")
	source := `package probe

func evade(m map[string]interface{}) {
	m["mindspec_phase"] = "done"
}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("writing probe source: %v", err)
	}
	violations := findUnguardedPhaseWrites(t, dir, path)
	if len(violations) != 1 {
		t.Fatalf("expected exactly 1 unguarded map-index write, got %d: %v", len(violations), violations)
	}
}

// TestFindUnguardedPhaseWrites_CatchesConstBoundKeyEvasion is round-1
// panel G5's red-on-new-unlocked-writer proof for shape 3: a
// composite-literal key that is a file-local const bound to
// "mindspec_phase" (not the literal string itself) must be flagged.
func TestFindUnguardedPhaseWrites_CatchesConstBoundKeyEvasion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "probe.go")
	source := `package probe

const phaseKey = "mindspec_phase"

func evade() map[string]interface{} {
	return map[string]interface{}{phaseKey: "done"}
}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("writing probe source: %v", err)
	}
	violations := findUnguardedPhaseWrites(t, dir, path)
	if len(violations) != 1 {
		t.Fatalf("expected exactly 1 unguarded const-bound-key write, got %d: %v", len(violations), violations)
	}
}

// TestFindUnguardedPhaseWrites_RvalueIndexReadNotFlagged proves the
// deliberate non-flagging half of shape 2's contract: an rvalue map READ
// (`m["mindspec_phase"]` on the right of an assignment, or standalone)
// must NEVER be flagged — this package's own guarded writers routinely
// read the stored value this way, and flagging reads would make the
// anti-drift test noisy to the point of being ignored.
func TestFindUnguardedPhaseWrites_RvalueIndexReadNotFlagged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "probe.go")
	source := `package probe

func read(m map[string]interface{}) interface{} {
	v := m["mindspec_phase"]
	return v
}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("writing probe source: %v", err)
	}
	violations := findUnguardedPhaseWrites(t, dir, path)
	if len(violations) != 0 {
		t.Fatalf("an rvalue map READ must never be flagged as a write: %v", violations)
	}
}

// TestFindUnguardedPhaseWrites_GuardedShapesNotFlagged proves the walker
// still correctly recognizes all three shapes as GUARDED when they are
// properly nested inside a WithPhaseWriteLock call.
func TestFindUnguardedPhaseWrites_GuardedShapesNotFlagged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "probe.go")
	source := `package probe

import "github.com/mrmaxsteel/mindspec/internal/phase"

const phaseKey = "mindspec_phase"

func guarded(root, epicID string, mergeFn phase.MergeMetadataFunc) error {
	return phase.WithPhaseWriteLock(root, epicID, mergeFn, func(string, bool) (map[string]interface{}, error) {
		m := map[string]interface{}{"mindspec_phase": "plan"}
		m2 := map[string]interface{}{phaseKey: "plan"}
		m3 := map[string]interface{}{}
		m3["mindspec_phase"] = "plan"
		_ = m2
		_ = m3
		return m, nil
	})
}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("writing probe source: %v", err)
	}
	violations := findUnguardedPhaseWrites(t, dir, path)
	if len(violations) != 0 {
		t.Fatalf("all three shapes nested inside WithPhaseWriteLock must be guarded, got violations: %v", violations)
	}
}

// TestFindUnguardedPhaseWrites_CatchesHelperBuiltUpdateMapEvasion is
// round-2 panel G4.2: AC-5(vi) requires a helper-built update map
// reaching a MergeMetadata-family call OUTSIDE the lock to be red. The
// evasion this probes: the "mindspec_phase" composite literal lives
// inside a SEPARATE helper function (buildUpdate), not at the
// mergeFn(...) call site itself — a call/dataflow-anchored companion to
// the composite-literal/index-write anchors above. This is a POSITIVE
// proof, not a walker change: findUnguardedPhaseWrites operates per-FILE
// over every top-level FuncDecl's body, so a composite literal inside
// ANY function — including one merely CALLED to build an update map
// elsewhere — is found and, since it is not lexically nested inside a
// WithPhaseWriteLock call anywhere in the file, correctly flagged as
// unguarded regardless of which function ultimately hands it to
// MergeMetadata. The genuine, disclosed residual (stated in this file's
// doc comment) is narrower: a key or map built via string
// concatenation/formatting, reflection, or data that never appears as a
// visible composite literal or index-assignment ANYWHERE in the scanned
// tree is still invisible — true whole-program data-flow analysis, which
// this test does not attempt to close.
func TestFindUnguardedPhaseWrites_CatchesHelperBuiltUpdateMapEvasion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "probe.go")
	source := `package probe

func buildUpdate() map[string]interface{} {
	return map[string]interface{}{"mindspec_phase": "done"}
}

func evade(id string, mergeFn func(string, map[string]interface{}) error) {
	updates := buildUpdate()
	_ = mergeFn(id, updates)
}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("writing probe source: %v", err)
	}
	violations := findUnguardedPhaseWrites(t, dir, path)
	if len(violations) != 1 {
		t.Fatalf("expected exactly 1 unguarded helper-built-update-map write, got %d: %v", len(violations), violations)
	}
	if !strings.Contains(violations[0], "buildUpdate") {
		t.Errorf("violation should attribute to the helper function that actually builds the literal, got: %s", violations[0])
	}
}

// TestFindUnguardedPhaseWrites_HelperBuiltUpdateMapGuardedWhenNested
// proves the companion positive case: the SAME helper-built-map shape
// is NOT flagged when the helper's call happens to be a closure literal
// nested inside a WithPhaseWriteLock call (i.e. the composite literal
// itself ends up lexically inside the lock call's extent).
func TestFindUnguardedPhaseWrites_HelperBuiltUpdateMapGuardedWhenNested(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "probe.go")
	source := `package probe

import "github.com/mrmaxsteel/mindspec/internal/phase"

func guarded(root, epicID string, mergeFn phase.MergeMetadataFunc) error {
	return phase.WithPhaseWriteLock(root, epicID, mergeFn, func(string, bool) (map[string]interface{}, error) {
		buildUpdate := func() map[string]interface{} {
			return map[string]interface{}{"mindspec_phase": "done"}
		}
		return buildUpdate(), nil
	})
}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("writing probe source: %v", err)
	}
	violations := findUnguardedPhaseWrites(t, dir, path)
	if len(violations) != 0 {
		t.Fatalf("a helper closure LEXICALLY NESTED inside WithPhaseWriteLock must be guarded, got violations: %v", violations)
	}
}
