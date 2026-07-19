// ratchet_inline_test.go — spec 120 R6 scan (b): the
// inline-composition scan (TestInlineBranchCompositionForbidden).
//
// Any string concatenation with a workspace.*Prefix constant, or with
// one of the branch/worktree/spec-dir literals (extended per rounds 3
// and 4 to the finalize prefixes, ".mindspec/specs/", and the
// "reviews/" path-segment concat with an ID-bearing identifier),
// OUTSIDE internal/workspace, fails unless allowlisted WITH a
// justification (in-package idvalidate gate, waist-validated operand,
// or mindspec-authored-literal provenance).
package lint

import (
	"go/ast"
	"go/token"
	"strings"
	"testing"
)

// inlinePrefixSelectors are the workspace prefix constants whose
// concatenation outside the waist is inline composition.
var inlinePrefixSelectors = map[string]bool{
	"SpecBranchPrefix":       true,
	"BeadBranchPrefix":       true,
	"SpecWorktreePrefix":     true,
	"BeadWorktreePrefix":     true,
	"FinalizeBranchPrefix":   true,
	"FinalizeWorktreePrefix": true,
}

// inlineLiterals is the round-3/round-4-extended literal set, ordered
// longest-first so a chain reports its most specific match.
var inlineLiterals = []string{
	".mindspec/specs/",
	"worktree-finalize-",
	"chore/finalize-",
	"worktree-spec-",
	"worktree-",
	"spec/",
	"bead/",
}

type inlineEntry struct {
	Count int
	// Justification is required: the in-package gate, the
	// waist-validated operand, or mindspec-authored provenance.
	Justification string
}

// literalMatches reports the matched forbidden literal for a string
// literal operand: exact match, or a suffix match where the byte
// before the suffix is not alphanumeric (so ".mindspec/" does not
// false-match "spec/", but "refs/heads/bead/" does match "bead/").
func literalMatches(val string) (string, bool) {
	for _, item := range inlineLiterals {
		if val == item {
			return item, true
		}
		if strings.HasSuffix(val, item) {
			pre := val[:len(val)-len(item)]
			last := pre[len(pre)-1]
			if !(last >= 'a' && last <= 'z' || last >= 'A' && last <= 'Z' || last >= '0' && last <= '9') {
				return item, true
			}
		}
	}
	return "", false
}

// flattenConcat flattens a maximal `+` chain into its operand list.
func flattenConcat(e ast.Expr, out *[]ast.Expr) {
	if b, ok := e.(*ast.BinaryExpr); ok && b.Op == token.ADD {
		flattenConcat(b.X, out)
		flattenConcat(b.Y, out)
		return
	}
	*out = append(*out, e)
}

// scanInlineComposition returns every forbidden inline-composition
// concat chain outside internal/workspace.
func scanInlineComposition(u *rUniverse) []rSite {
	var sites []rSite
	for _, f := range u.files {
		if f.pkgDir == "internal/workspace" {
			continue
		}
		wsLocal := f.importLocal(workspaceImport)
		// Track ADD-children so only maximal chains are considered.
		child := map[ast.Node]bool{}
		ast.Inspect(f.file, func(n ast.Node) bool {
			b, ok := n.(*ast.BinaryExpr)
			if !ok || b.Op != token.ADD {
				return true
			}
			if x, ok := b.X.(*ast.BinaryExpr); ok && x.Op == token.ADD {
				child[x] = true
			}
			if y, ok := b.Y.(*ast.BinaryExpr); ok && y.Op == token.ADD {
				child[y] = true
			}
			return true
		})
		ast.Inspect(f.file, func(n ast.Node) bool {
			b, ok := n.(*ast.BinaryExpr)
			if !ok || b.Op != token.ADD || child[b] {
				return true
			}
			var ops []ast.Expr
			flattenConcat(b, &ops)
			matched := map[string]bool{}
			hasID := false
			hasReviews := false
			for _, op := range ops {
				switch v := op.(type) {
				case *ast.BasicLit:
					if v.Kind != token.STRING {
						continue
					}
					val, ok := unquote(v.Value)
					if !ok {
						continue
					}
					if item, ok := literalMatches(val); ok {
						matched["lit:"+item] = true
					}
					if strings.Contains(val, "reviews/") {
						hasReviews = true
					}
				case *ast.SelectorExpr:
					if x, ok := v.X.(*ast.Ident); ok && wsLocal != "" && x.Name == wsLocal && inlinePrefixSelectors[v.Sel.Name] {
						matched["const:workspace."+v.Sel.Name] = true
					}
					if isIDName(v.Sel.Name) {
						hasID = true
					}
				case *ast.Ident:
					if isIDName(v.Name) {
						hasID = true
					}
				}
			}
			if hasReviews && hasID {
				matched["lit:reviews/-with-id"] = true
			}
			for item := range matched {
				sites = append(sites, rSite{
					Rel:    f.rel,
					Func:   enclosingFunc(f, b.Pos()),
					Detail: "concat " + item,
					Line:   u.fset.Position(b.Pos()).Line,
				})
			}
			return true
		})
	}
	return sites
}

func inlineAllowCounts(entries map[string]inlineEntry) map[string]int {
	out := map[string]int{}
	for k, e := range entries {
		out[k] = e.Count
	}
	return out
}

// TestInlineBranchCompositionForbidden is spec 120 R6 scan (b)
// (AC-14): inline branch/worktree/spec-dir composition outside the
// waist fails unless allowlisted with justification.
func TestInlineBranchCompositionForbidden(t *testing.T) {
	u := loadRatchetUniverse(t)
	sites := scanInlineComposition(u)

	for k, e := range inlineCompositionAllowlist {
		if e.Justification == "" {
			t.Errorf("allowlist entry %q missing its justification", k)
		}
	}
	failOnProblems(t, "scan (b) inline composition",
		diffSites(sites, inlineAllowCounts(inlineCompositionAllowlist)))

	t.Run("fixture_new_bead_concat_flagged", func(t *testing.T) {
		fu := fixtureUniverse(t, "ratchet_inline_concat.go.txt")
		problems := diffSites(scanInlineComposition(fu), inlineAllowCounts(inlineCompositionAllowlist))
		assertProblemPresent(t, problems, "UNAUDITED", `concat lit:bead/`)
	})

	t.Run("fixture_specs_reviews_concat_flagged", func(t *testing.T) {
		// G1's shape: ".mindspec/specs/"+x+"/reviews/" with an
		// ID-bearing identifier.
		fu := fixtureUniverse(t, "ratchet_inline_concat.go.txt")
		problems := diffSites(scanInlineComposition(fu), inlineAllowCounts(inlineCompositionAllowlist))
		assertProblemPresent(t, problems, "UNAUDITED", "concat lit:.mindspec/specs/")
		assertProblemPresent(t, problems, "UNAUDITED", "concat lit:reviews/-with-id")
	})

	t.Run("fixture_deleted_allowlisted_site_flagged", func(t *testing.T) {
		synth := map[string]int{"internal/nowhere/gone.go deletedFunc concat lit:bead/": 1}
		problems := diffSites(nil, synth)
		assertProblemPresent(t, problems, "STALE", "internal/nowhere/gone.go")
	})
}
