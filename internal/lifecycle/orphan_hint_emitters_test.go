package lifecycle

// Spec 127 R2(a)'s anti-drift leg (AC-3's own falsifier: "an orphan-hint
// emitter exists outside the enumeration"): a call-site enumeration test
// that walks cmd/+internal/ production Go source (AST-only — no
// compilation, no import of the scanned packages) for every SYNTACTIC
// REFERENCE, resolved by IMPORT-PATH IDENTITY (never the literal
// "lifecycle" spelling — an aliased or dot import must still be
// caught), to this package's orphan-hint entrypoints:
// EvaluateOrphanHint, EvaluateOrphanHintAgainstMain, and
// DeriveOrphanHint.
//
// A reference is either a CallExpr's callee OR a bare value expression
// (a package-level seam-var assignment, e.g. `var evaluateOrphanHintFn
// = lifecycle.EvaluateOrphanHint`) — every production consumer in this
// spec threads the real evaluation through exactly such a seam so its
// own tests can stub it (see complete.go/impl.go/orphaned_beads.go/
// adopt.go's own seam doc comments), never a direct inline call. Scanning
// only CallExprs would therefore find NOTHING — every real site is a
// bare selector on the right-hand side of a var declaration.
//
// This is the SAME shape and rationale as internal/lint's `RecoveryCommand`-
// method scan and cmd/mindspec/impl_adopt_test.go's AdoptSpec call-site
// enumeration: syntax-shaped, not object/call-graph based —
// reachability through a function value, method value, or
// wrapper-satisfied interface is NOT mechanically detected here either,
// and is review-caught under the same residual class R5(b)/R1(e)
// already record.
//
// NARROWED SCOPE, stated precisely (bead-4 fix round 1, BLOCKING-3 —
// G1-3): this cross-package leg matches ONLY a reference reached through
// an IMPORT of internal/lifecycle — structurally blind to a call from
// WITHIN internal/lifecycle itself, because no file ever imports its own
// package. TestOrphanHintEmitters_SamePackageCallersArePinned below
// closes exactly that gap for DIRECT calls to DeriveOrphanHint (the one
// production entrypoint a same-package emitter could call with a
// fabricated, non-evaluated outcome). What NEITHER leg catches — named
// here rather than silently overclaimed, the same disclosure bead 2's
// samePackageEscapeShapes settled on — is SEAM REUSE: a new production
// consumer that renders a second recovery surface by calling an
// ALREADY-ENUMERATED seam var directly (e.g. a second call site added to
// implEvaluateOrphanHintFn from a NEW function in impl.go) adds no new
// lifecycle-package-qualified reference for this scan to find at all.
// TestOrphanHintEmitters_SeamReuseIsReviewCaught documents that residual
// explicitly; it is REVIEW-CAUGHT, not mechanically closed.
import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const lifecycleImportPath = "github.com/mrmaxsteel/mindspec/internal/lifecycle"

// orphanHintWantedSymbols is the closed set this scan matches — the
// package's three orphan-hint entrypoints. Exported so a hypothetical
// sibling scan (there is none today) could reuse it without drifting.
var orphanHintWantedSymbols = map[string]bool{
	"EvaluateOrphanHint":            true,
	"EvaluateOrphanHintAgainstMain": true,
	"DeriveOrphanHint":              true,
}

type orphanHintRef struct {
	file, context, symbol string
}

func (r orphanHintRef) key() string { return r.file + "\t" + r.context + "\t" + r.symbol }

// findOrphanHintRefs parses absPath (relative name rel, for reporting)
// and returns one orphanHintRef per reference to any symbol in
// orphanHintWantedSymbols that resolves — by IMPORT PATH IDENTITY — to
// this package (github.com/mrmaxsteel/mindspec/internal/lifecycle).
// Collects EVERY match found in the file; never returns early on the
// first (see this file's own package doc comment, and
// TestOrphanHintEmitters_DefeatUniqueness below, which proves it).
func findOrphanHintRefs(t *testing.T, absPath, rel string) []orphanHintRef {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, absPath, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", absPath, err)
	}

	qualified := map[string]bool{}
	dotImported := false
	for _, imp := range file.Imports {
		path, uerr := strconv.Unquote(imp.Path.Value)
		if uerr != nil || path != lifecycleImportPath {
			continue
		}
		switch {
		case imp.Name == nil:
			qualified["lifecycle"] = true
		case imp.Name.Name == ".":
			dotImported = true
		case imp.Name.Name != "_":
			qualified[imp.Name.Name] = true
		}
	}
	if len(qualified) == 0 && !dotImported {
		return nil
	}

	var refs []orphanHintRef

	if len(qualified) > 0 {
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || !orphanHintWantedSymbols[sel.Sel.Name] {
				return true
			}
			xid, ok := sel.X.(*ast.Ident)
			if !ok || !qualified[xid.Name] {
				return true
			}
			refs = append(refs, orphanHintRef{file: rel, context: enclosingContext(file, n.Pos()), symbol: sel.Sel.Name})
			return true
		})
	}
	if dotImported {
		ast.Inspect(file, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok || !orphanHintWantedSymbols[id.Name] {
				return true
			}
			// A dot import makes every exported name of the imported
			// package a bare identifier in THIS file's scope — but that
			// also means a same-named LOCAL declaration would shadow it.
			// This scan does not attempt to disambiguate a shadowed bare
			// identifier from the real dot-imported one (the same
			// syntax-shaped limitation this file's own doc comment
			// states); no production file in this spec dot-imports
			// lifecycle today (grep-verified), so this leg is defensive,
			// not currently exercised.
			refs = append(refs, orphanHintRef{file: rel, context: enclosingContext(file, id.Pos()), symbol: id.Name})
			return true
		})
	}
	return refs
}

// enclosingContext names the nearest enclosing top-level FuncDecl name
// (qualified as Recv.Type + "." + Name for a method) or, for a
// package-level var/const ValueSpec (the shape every real orphan-hint
// seam assignment in this spec uses), that ValueSpec's first name.
// "<package-level>" when neither applies.
func enclosingContext(file *ast.File, pos token.Pos) string {
	best := "<package-level>"
	var bestStart token.Pos
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Pos() <= pos && pos <= d.End() && d.Pos() > bestStart {
				name := d.Name.Name
				if d.Recv != nil && len(d.Recv.List) > 0 {
					name = recvTypeName(d.Recv.List[0].Type) + "." + name
				}
				best, bestStart = name, d.Pos()
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok || len(vs.Names) == 0 {
					continue
				}
				if vs.Pos() <= pos && pos <= vs.End() && vs.Pos() > bestStart {
					best, bestStart = vs.Names[0].Name, vs.Pos()
				}
			}
		}
	}
	return best
}

func recvTypeName(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.StarExpr:
		return recvTypeName(v.X)
	}
	return "?"
}

// scanRepoForOrphanHintRefs walks cmd/ and internal/ (production files
// only — _test.go and testdata excluded) and returns EVERY reference
// found across the whole repo.
func scanRepoForOrphanHintRefs(t *testing.T) []orphanHintRef {
	t.Helper()
	root := repoRootFromLifecycleTestDir(t)
	var all []orphanHintRef
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
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				t.Fatalf("relativizing %s: %v", path, rerr)
			}
			all = append(all, findOrphanHintRefs(t, path, rel)...)
			return nil
		}); err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
	}
	return all
}

// repoRootFromLifecycleTestDir resolves the repo root from this test
// file's own package directory (internal/lifecycle -> two levels up).
func repoRootFromLifecycleTestDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(filepath.Join(wd, "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("resolved root %s has no go.mod — repoRootFromLifecycleTestDir's relative path assumption broke: %v", root, err)
	}
	return root
}

// wantOrphanHintRefs is the pinned, exact enumeration: one seam-var
// reference per production consumer (spec 127 R2(a)). complete.go's two
// render sites (:520/:552 in the pre-bead-4 numbering) collapse to ONE
// reference here because bead 4's implementation computes each orphan's
// hint ONCE (before branching on self-orphan status) rather than
// re-deriving it per branch — the derivation is still consulted for
// every rendered line, just without a duplicate git evaluation.
func wantOrphanHintRefs() []string {
	return []string{
		orphanHintRef{file: "internal/approve/adopt.go", context: "adoptEvaluateOrphanHintFn", symbol: "EvaluateOrphanHintAgainstMain"}.key(),
		orphanHintRef{file: "internal/approve/impl.go", context: "implEvaluateOrphanHintFn", symbol: "EvaluateOrphanHint"}.key(),
		orphanHintRef{file: "internal/complete/complete.go", context: "evaluateOrphanHintFn", symbol: "EvaluateOrphanHint"}.key(),
		orphanHintRef{file: "internal/doctor/orphaned_beads.go", context: "evaluateOrphanHintFn", symbol: "EvaluateOrphanHint"}.key(),
	}
}

// TestOrphanHintEmitters_ExactEnumeration is R2(a)'s anti-drift leg: the
// EXACT, sorted set of production references to this package's
// orphan-hint entrypoints. A new render surface that reaches for
// lifecycle.EvaluateOrphanHint (or either sibling) without this scan
// finding it EXACTLY once, at EXACTLY this enumerated site, REDs this
// test — the enforcement this leg exists for. Falsified equally by a
// MISSING reference (a consumer bypassed the derivation) or an EXTRA
// one (a new, un-reviewed emitter) — see the defeat test below for
// proof this scan does not stop at the first match either way.
func TestOrphanHintEmitters_ExactEnumeration(t *testing.T) {
	refs := scanRepoForOrphanHintRefs(t)
	got := make([]string, len(refs))
	for i, r := range refs {
		got[i] = r.key()
	}
	sort.Strings(got)

	want := wantOrphanHintRefs()
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("found %d orphan-hint reference(s), want exactly %d:\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("reference[%d] = %q, want %q\nfull got: %v\nfull want: %v", i, got[i], want[i], got, want)
		}
	}
}

// TestOrphanHintEmitters_DefeatUniqueness is the mechanism's own
// self-test (spec 127's own house lesson, bead 5's decoy-shaped anti-
// drift bug: a walker that took the FIRST match instead of requiring
// uniqueness passed against a planted decoy). It parses a SYNTHETIC
// two-reference fixture (via go/parser over an in-memory source string,
// no real file) and asserts findOrphanHintRefs returns BOTH — not one,
// not a de-duplicated one — proving this scan does not silently collapse
// multiple real references in a single file into a false single-site
// report.
func TestOrphanHintEmitters_DefeatUniqueness(t *testing.T) {
	src := `package decoy

import "github.com/mrmaxsteel/mindspec/internal/lifecycle"

var firstSeam = lifecycle.EvaluateOrphanHint

func secondSite() {
	_ = lifecycle.EvaluateOrphanHint
}
`
	dir := t.TempDir()
	path := filepath.Join(dir, "decoy.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	refs := findOrphanHintRefs(t, path, "decoy.go")
	if len(refs) != 2 {
		t.Fatalf("found %d reference(s) in a fixture with TWO, got %+v — this scan must never stop at the first match", len(refs), refs)
	}
	seen := map[string]bool{}
	for _, r := range refs {
		seen[r.context] = true
	}
	if !seen["firstSeam"] || !seen["secondSite"] {
		t.Errorf("expected references in BOTH firstSeam and secondSite, got %+v", refs)
	}
}

// TestOrphanHintEmitters_AliasedImportStillMatches is O2-1's reproduced
// leniency check (the same class R1(e)'s AdoptSpec scan fixes): an
// aliased import of this package must not hide a reference.
func TestOrphanHintEmitters_AliasedImportStillMatches(t *testing.T) {
	src := `package decoy

import lc "github.com/mrmaxsteel/mindspec/internal/lifecycle"

var seam = lc.EvaluateOrphanHintAgainstMain
`
	dir := t.TempDir()
	path := filepath.Join(dir, "decoy.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	refs := findOrphanHintRefs(t, path, "decoy.go")
	if len(refs) != 1 || refs[0].symbol != "EvaluateOrphanHintAgainstMain" || refs[0].context != "seam" {
		t.Fatalf("aliased import must still be matched by import-path identity, got %+v", refs)
	}
}

// --- same-package leg (bead-4 fix round 1, BLOCKING-3) -----------------

// approvedSamePackageDeriveOrphanHintCallers is the CLOSED, pinned set
// of the only two production functions in this package allowed to call
// DeriveOrphanHint directly: the two wrapper entrypoints every EXTERNAL
// consumer actually reaches through (EvaluateOrphanHint,
// EvaluateOrphanHintAgainstMain). Both always run a REAL
// EvaluateWorkDestruction evaluation before calling DeriveOrphanHint —
// that is the entire evidentiary guarantee DeriveOrphanHint's own doc
// comment describes ("enforced by review + the emitter-enumeration
// anti-drift test"). A THIRD same-package caller would call
// DeriveOrphanHint with an outcome value that need not have come from a
// real evaluation at all (the type's own zero-value legitimacy,
// documented there) — review-caught before this fix, mechanically
// caught after it.
var approvedSamePackageDeriveOrphanHintCallers = map[string]bool{
	"EvaluateOrphanHint":            true,
	"EvaluateOrphanHintAgainstMain": true,
}

// samePackageDeriveOrphanHintRef is one bare (unqualified) reference to
// DeriveOrphanHint found inside a package's own production source (never
// _test.go — this file's/bead-4's own AC-3 table calling the pure
// derivation directly, over real-git fixtures, is that table's whole
// design, not an emitter).
type samePackageDeriveOrphanHintRef struct {
	file, context string
}

// scanPackageForBareDeriveCalls walks EVERY production (non-_test.go)
// .go file directly inside dir and returns one ref per bare, unqualified
// CallExpr callee named "DeriveOrphanHint" — the same-package half of
// R2(a)'s anti-drift leg that findOrphanHintRefs/scanRepoForOrphanHintRefs
// above structurally cannot cover (they only ever match an IMPORTED,
// qualified or dot-imported, reference — a file inside the package being
// scanned never imports itself).
func scanPackageForBareDeriveCalls(t *testing.T, dir string) []samePackageDeriveOrphanHintRef {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading package dir %s: %v", dir, err)
	}
	var refs []samePackageDeriveOrphanHintRef
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		fset := token.NewFileSet()
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Fatalf("parsing %s: %v", path, perr)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := call.Fun.(*ast.Ident)
			if !ok || id.Name != "DeriveOrphanHint" {
				return true
			}
			refs = append(refs, samePackageDeriveOrphanHintRef{file: e.Name(), context: enclosingContext(file, call.Pos())})
			return true
		})
	}
	return refs
}

// TestOrphanHintEmitters_SamePackageCallersArePinned is R2(a)'s
// same-package leg (bead-4 fix round 1, BLOCKING-3/G1-3): every bare
// call to DeriveOrphanHint anywhere in this package's production source
// must resolve to one of the two approved wrapper functions — a new
// same-package emitter (calling DeriveOrphanHint directly, with a
// fabricated or otherwise non-evaluated outcome) reds this test the
// moment it is added, never silently passing the way
// TestOrphanHintEmitters_ExactEnumeration alone would (it cannot see
// this file at all — no import to match).
func TestOrphanHintEmitters_SamePackageCallersArePinned(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	refs := scanPackageForBareDeriveCalls(t, wd)
	if len(refs) == 0 {
		t.Fatal("found zero same-package DeriveOrphanHint callers — expected exactly the two approved wrappers; this scan's own confinement broke, or the wrappers were renamed without updating this test")
	}
	for _, r := range refs {
		if !approvedSamePackageDeriveOrphanHintCallers[r.context] {
			t.Errorf("unapproved same-package DeriveOrphanHint call in %s, context %q — only %v may call this derivation directly; every other caller must go through EvaluateOrphanHint/EvaluateOrphanHintAgainstMain so the outcome it renders is guaranteed to come from a real evaluation", r.file, r.context, approvedSamePackageDeriveOrphanHintCallers)
		}
	}
	seen := map[string]bool{}
	for _, r := range refs {
		seen[r.context] = true
	}
	for approved := range approvedSamePackageDeriveOrphanHintCallers {
		if !seen[approved] {
			t.Errorf("expected an approved caller %q to call DeriveOrphanHint directly, found none — a wrapper was refactored to no longer call the derivation, or renamed without updating this pin", approved)
		}
	}
}

// TestOrphanHintEmitters_DefeatSamePackageDirectDerivation is this new
// leg's own defeat test (the same non-vacuity discipline every anti-
// drift scan in this spec carries): plants a THIRD same-package caller —
// an "adversarialSixthEmitter"-shaped function calling DeriveOrphanHint
// with a fabricated outcome, exactly the shape G1's panel finding
// reproduced against the real production file — in a fixture written to
// a fresh temp dir (never the real orphan_hints.go itself) and confirms
// the scan flags it.
func TestOrphanHintEmitters_DefeatSamePackageDirectDerivation(t *testing.T) {
	dir := t.TempDir()
	src := `package lifecycle

import (
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
)

func adversarialSixthEmitter() string {
	hint := DeriveOrphanHint(guard.DestructionOutcome(0), gitutil.WorkDestructionEvidence{}, "x", "y", "z", "w")
	return hint.Lines[0]
}
`
	path := filepath.Join(dir, "adversarial.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	refs := scanPackageForBareDeriveCalls(t, dir)
	found := false
	for _, r := range refs {
		if r.context == "adversarialSixthEmitter" {
			found = true
		}
	}
	if !found {
		t.Fatal("planted same-package direct-derivation caller was not detected — the same-package scan is hollow")
	}
}

// TestOrphanHintEmitters_SeamReuseIsReviewCaught is this leg's own
// disclosed residual (bead-4 fix round 1, G1-3's second half — the same
// "state exactly what it catches and what it does not" discipline bead
// 2's samePackageEscapeShapes settled on for its own unfixturable
// escape). A NEW production function that renders a SECOND recovery
// surface by calling an ALREADY-ENUMERATED seam var directly (e.g. a
// second call to implEvaluateOrphanHintFn from a new function in
// impl.go, rather than adding a fresh `lifecycle.EvaluateOrphanHint`
// reference of its own) adds NO new lifecycle-package-qualified
// reference for scanRepoForOrphanHintRefs to find — that scan is keyed
// entirely on syntactic references to THIS package's symbols, and a
// seam-var call site one package away is, by construction, invisible to
// it. This test plants exactly that shape (a synthetic second-consumer
// file calling an already-declared seam var from a NEW function) and
// confirms — deliberately asserting the CURRENT, undetected behavior,
// not fixing it — that findOrphanHintRefs produces no reference for it.
// Review is the only enforcement for this residual class; if it is ever
// mechanically closed, this test should be rewritten to assert detection
// instead of its absence.
func TestOrphanHintEmitters_SeamReuseIsReviewCaught(t *testing.T) {
	src := `package approve

func implSecondaryOrphanRefusal(root, beadID, beadBranch, specID, specBranch string) string {
	hint := implEvaluateOrphanHintFn(root, beadID, beadBranch, specID, specBranch)
	return hint.Lines[0]
}
`
	dir := t.TempDir()
	path := filepath.Join(dir, "second_consumer.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	refs := findOrphanHintRefs(t, path, "second_consumer.go")
	if len(refs) != 0 {
		t.Fatalf("a seam-var-reuse second consumer unexpectedly produced %d reference(s) — if the scan now detects this shape, rewrite this test to assert detection and update this file's doc comment (the residual this test names would then be closed)", len(refs))
	}
}
