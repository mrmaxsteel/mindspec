package guard

// Spec 127 bead-1 fix round, O2-5(a): DestructionOutcomeCount is meant to
// be the FINAL spec in its own iota const block — every consumer's
// fixture table asserts len(table) == DestructionOutcomeCount as its
// Go-has-no-exhaustiveness-check substitute (see outcome.go's package
// doc). That guarantee only holds while a NEW variant can only ever be
// inserted BEFORE the sentinel: appending one AFTER it leaves
// DestructionOutcomeCount unchanged and requires no fixture anywhere,
// which is exactly the fail-open shape a preflight predicate must never
// have (a variant no consumer switch handles falls through every arm —
// in a refusal gate, that means no refusal).
//
// This walks outcome.go's own AST rather than hand-counting, so
// appending a variant after DestructionOutcomeCount fails this test
// without any further wiring — the AST IS the source of truth the
// sentinel is supposed to describe.
//
// Extended (spec 127 bead-1 fix round 2, NEW-O2-d): the FINAL-spec check
// above only closes the fail-open shape for a variant appended INSIDE the
// pinned block. A variant declared in some OTHER const declaration —
// `const DestructionSixthVariant DestructionOutcome = 99` entirely on its
// own — escapes both that check and every consumer's own
// len(table)==DestructionOutcomeCount count: "the same silent-fail-open
// shape this test was added to remove, one step further out." The second
// half below walks every OTHER top-level const declaration in the file
// and rejects any spec explicitly typed DestructionOutcome. This catches
// the common authoring shape (an explicit type on the new spec); it does
// NOT catch a spec with no explicit type whose value expression merely
// happens to evaluate to a DestructionOutcome (e.g. one derived by
// arithmetic from DestructionOutcomeCount) — go/ast's static Type field
// is nil for those, and resolving the expression's type would need full
// type-checking (go/types), which this lightweight parse-only walk
// deliberately does not perform. Naming that boundary here rather than
// letting the doc comment overclaim exhaustiveness it does not have.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestDestructionOutcomeCountIsFinalConstSpec(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "outcome.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing outcome.go: %v", err)
	}

	var names []string
	var primaryDecl *ast.GenDecl
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		var blockNames []string
		containsAncestor := false
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, n := range vs.Names {
				blockNames = append(blockNames, n.Name)
				if n.Name == "DestructionAncestor" {
					containsAncestor = true
				}
			}
		}
		if containsAncestor {
			names = blockNames
			primaryDecl = gd
			break
		}
	}
	if primaryDecl == nil {
		t.Fatal("could not find the DestructionOutcome const block (DestructionAncestor) in outcome.go")
	}
	if len(names) == 0 {
		t.Fatal("the DestructionOutcome const block has no named specs")
	}

	last := names[len(names)-1]
	if last != "DestructionOutcomeCount" {
		t.Fatalf("DestructionOutcomeCount must be the FINAL spec in its const block (a variant appended after it escapes the len(table)==DestructionOutcomeCount discipline every consumer copies); got last spec %q, full block %v", last, names)
	}

	// NEW-O2-d: no OTHER const declaration in the file may explicitly
	// type a spec DestructionOutcome — that would be a variant declared
	// outside the pinned block, invisible to both the check above and to
	// every consumer's len(table)==DestructionOutcomeCount count.
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST || gd == primaryDecl {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || vs.Type == nil {
				continue
			}
			ident, ok := vs.Type.(*ast.Ident)
			if !ok || ident.Name != "DestructionOutcome" {
				continue
			}
			var declared []string
			for _, n := range vs.Names {
				declared = append(declared, n.Name)
			}
			t.Fatalf("DestructionOutcome variant(s) %v declared OUTSIDE the pinned const block containing DestructionAncestor/DestructionOutcomeCount — this escapes the len(table)==DestructionOutcomeCount discipline every consumer copies", declared)
		}
	}
}
