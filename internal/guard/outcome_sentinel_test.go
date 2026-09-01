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
// half below walks every OTHER top-level const declaration and rejects
// any spec explicitly typed DestructionOutcome. This catches the common
// authoring shape (an explicit type on the new spec); it does NOT catch
// a spec with no explicit type whose value expression merely happens to
// evaluate to a DestructionOutcome (e.g. one derived by arithmetic from
// DestructionOutcomeCount) — go/ast's static Type field is nil for
// those, and resolving the expression's type would need full type-
// checking (go/types), which this lightweight parse-only walk
// deliberately does not perform. Naming that boundary here rather than
// letting the doc comment overclaim exhaustiveness it does not have.
//
// Widened twice more (spec 127 bead-1 fix round 3->4, NEW-O2R-c): the
// walk above parsed ONLY the single file "outcome.go", and rejected ONLY
// token.CONST declarations — two further escapes of the identical class,
// verified by on-disk mutation: a `const DestructionSixthVariant
// DestructionOutcome = 99` declared in any OTHER file of this package
// (a walk scoped to one file's AST cannot see it), and a `var
// DestructionSixthVariant DestructionOutcome = 99` rather than a const
// (the rejection loop below skipped any GenDecl whose Tok was not
// token.CONST). Both now parse the WHOLE package directory
// (parser.ParseDir, filtering out _test.go — a test-local value of this
// type is a fixture, not a production variant this sentinel need
// police) and accept token.VAR alongside token.CONST in the rejection
// loop. The one escape this still cannot see — an untyped spec whose
// value merely evaluates to a DestructionOutcome by arithmetic — is
// unchanged and still named honestly above.
//
// Widened a fifth time (spec 127 bead-1 fix round 5, NEW-O2G-3): the
// rejection loop matched only `vs.Type` whose identifier is literally
// "DestructionOutcome", so a variant declared through a type ALIAS —
// `type DOAlias = DestructionOutcome` followed by `const
// DestructionSixthVariant DOAlias = 99` — escaped, even though that spec
// IS explicitly typed (unlike the untyped-arithmetic escape named above,
// which this doc comment already disclaimed honestly). One level of
// alias resolution closes it: a first pass over the same parsed package
// collects every top-level `type X = DestructionOutcome` alias name
// (token.TYPE declarations whose Spec is a *ast.TypeSpec with Assign set
// — i.e. `=`, not a defined type `type X DestructionOutcome`, which is a
// distinct type the compiler would not accept a DestructionOutcome value
// as without a conversion, and is out of scope here) into a set; the
// rejection loop then matches vs.Type's identifier against that set as
// well as the literal name. A second-level alias (`type B = A` where A
// is itself an alias of DestructionOutcome) is NOT resolved — one level
// closes the shape a real author would plausibly reach for and this
// walk's own on-disk mutation test targets; chasing arbitrarily deep
// alias chains would be the same unbounded-generality trade the
// untyped-arithmetic escape above already declines.
//
// Widened a sixth time (spec 127 bead-1 fix round 6, NEW-G1sub-12): both
// Ident assertions — the alias collector's `ts.Type.(*ast.Ident)` and the
// rejection loop's `vs.Type.(*ast.Ident)` — failed on a PARENTHESIZED
// type: `const DestructionSixthVariantParen (DestructionOutcome) = 99`
// compiles, is gofmt-clean (gofmt does not strip redundant parens around
// a const spec's type), and is a real, assignable DestructionOutcome
// variant, yet both assertions saw a *ast.ParenExpr rather than an
// *ast.Ident and silently skipped it — the identical escape as the
// untyped-arithmetic one this doc comment names above, but for a spec
// that IS explicitly typed, which the walk otherwise claims to cover.
// ast.Unparen (stdlib since Go 1.22; this module's go.mod floor is 1.23)
// unwraps any parenthesization before both assertions, closing the
// parenthesized form and its alias-of-parenthesized variant at the same
// time. A parenthesized alias declaration (`type DOAliasParen =
// (DestructionOutcome)`) is likewise covered, since the collector's
// assertion is unwrapped too.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestDestructionOutcomeCountIsFinalConstSpec(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parsing package guard: %v", err)
	}
	pkg, ok := pkgs["guard"]
	if !ok {
		var found []string
		for name := range pkgs {
			found = append(found, name)
		}
		t.Fatalf("package %q not found in parsed directory (found: %v)", "guard", found)
	}

	var names []string
	var primaryDecl *ast.GenDecl
	for _, file := range pkg.Files {
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
		if primaryDecl != nil {
			break
		}
	}
	if primaryDecl == nil {
		t.Fatal("could not find the DestructionOutcome const block (DestructionAncestor) anywhere in package guard")
	}
	if len(names) == 0 {
		t.Fatal("the DestructionOutcome const block has no named specs")
	}

	last := names[len(names)-1]
	if last != "DestructionOutcomeCount" {
		t.Fatalf("DestructionOutcomeCount must be the FINAL spec in its const block (a variant appended after it escapes the len(table)==DestructionOutcomeCount discipline every consumer copies); got last spec %q, full block %v", last, names)
	}

	// NEW-O2G-3 (spec 127 bead-1 fix round 5): collect one level of type
	// alias — `type X = DestructionOutcome`, Assign set — so the
	// rejection loop below also catches a variant declared through an
	// alias, not only the literal type name.
	aliasNames := map[string]bool{}
	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || ts.Assign == token.NoPos {
					continue
				}
				if ident, ok := ast.Unparen(ts.Type).(*ast.Ident); ok && ident.Name == "DestructionOutcome" {
					aliasNames[ts.Name.Name] = true
				}
			}
		}
	}

	// NEW-O2-d, widened by NEW-O2R-c: no OTHER const OR var declaration
	// anywhere in the package's non-test files may explicitly type a spec
	// DestructionOutcome (or an alias of it, NEW-O2G-3) — that would be a
	// variant declared outside the pinned block, invisible to both the
	// check above and to every consumer's len(table)==DestructionOutcomeCount
	// count.
	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd == primaryDecl || (gd.Tok != token.CONST && gd.Tok != token.VAR) {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok || vs.Type == nil {
					continue
				}
				ident, ok := ast.Unparen(vs.Type).(*ast.Ident)
				if !ok || (ident.Name != "DestructionOutcome" && !aliasNames[ident.Name]) {
					continue
				}
				var declared []string
				for _, n := range vs.Names {
					declared = append(declared, n.Name)
				}
				t.Fatalf("DestructionOutcome variant(s) %v declared OUTSIDE the pinned const block containing DestructionAncestor/DestructionOutcomeCount (type %q) — this escapes the len(table)==DestructionOutcomeCount discipline every consumer copies", declared, ident.Name)
			}
		}
	}
}
