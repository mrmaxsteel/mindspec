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
	foundBlock := false
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
			foundBlock = true
			break
		}
	}
	if !foundBlock {
		t.Fatal("could not find the DestructionOutcome const block (DestructionAncestor) in outcome.go")
	}
	if len(names) == 0 {
		t.Fatal("the DestructionOutcome const block has no named specs")
	}

	last := names[len(names)-1]
	if last != "DestructionOutcomeCount" {
		t.Fatalf("DestructionOutcomeCount must be the FINAL spec in its const block (a variant appended after it escapes the len(table)==DestructionOutcomeCount discipline every consumer copies); got last spec %q, full block %v", last, names)
	}
}
