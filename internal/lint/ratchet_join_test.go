// ratchet_join_test.go — spec 120 R6 scan (c): the Join scan
// (TestJoinWithIDForbidden).
//
// Any filepath.Join call outside internal/workspace whose arguments
// include a specID/beadID/epicID-named identifier fails unless
// allowlisted WITH the covering gate. Per the plan, entries are never
// allowlisted-with-a-false-gate: contextpack/budgeter.go's bd-metadata
// spec_id Join is GATED (idvalidate before the Join) because its
// provenance is agent-writable.
package lint

import (
	"go/ast"
	"testing"
)

type joinEntry struct {
	Count int
	// Gate names the idvalidate gate covering the ID argument
	// (in-function, at ingress, or validate-and-drop enumeration).
	Gate string
}

// scanJoinWithID returns every filepath.Join call outside
// internal/workspace with an ID-named argument.
func scanJoinWithID(u *rUniverse) []rSite {
	var sites []rSite
	for _, f := range u.files {
		if f.pkgDir == "internal/workspace" {
			continue
		}
		fpLocal := f.importLocal(filepathImport)
		if fpLocal == "" {
			continue
		}
		ast.Inspect(f.file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Join" {
				return true
			}
			x, ok := sel.X.(*ast.Ident)
			if !ok || x.Name != fpLocal {
				return true
			}
			for _, arg := range call.Args {
				if name, ok := exprContainsIDIdent(arg, nil); ok {
					sites = append(sites, rSite{
						Rel:    f.rel,
						Func:   enclosingFunc(f, call.Pos()),
						Detail: "filepath.Join(..." + name + "...)",
						Line:   u.fset.Position(call.Pos()).Line,
					})
					break // one site per Join call
				}
			}
			return true
		})
	}
	return sites
}

func joinAllowCounts(entries map[string]joinEntry) map[string]int {
	out := map[string]int{}
	for k, e := range entries {
		out[k] = e.Count
	}
	return out
}

// TestJoinWithIDForbidden is spec 120 R6 scan (c) (AC-14).
func TestJoinWithIDForbidden(t *testing.T) {
	u := loadRatchetUniverse(t)
	sites := scanJoinWithID(u)

	for k, e := range joinWithIDAllowlist {
		if e.Gate == "" {
			t.Errorf("allowlist entry %q missing its covering gate", k)
		}
	}
	failOnProblems(t, "scan (c) filepath.Join with ID",
		diffSites(sites, joinAllowCounts(joinWithIDAllowlist)))

	t.Run("fixture_new_join_flagged", func(t *testing.T) {
		fu := fixtureUniverse(t, "ratchet_join_id.go.txt")
		problems := diffSites(scanJoinWithID(fu), joinAllowCounts(joinWithIDAllowlist))
		assertProblemPresent(t, problems, "UNAUDITED", "filepath.Join(...specID...)")
	})

	t.Run("fixture_deleted_allowlisted_site_flagged", func(t *testing.T) {
		synth := map[string]int{"internal/nowhere/gone.go deletedFunc filepath.Join(...specID...)": 1}
		problems := diffSites(nil, synth)
		assertProblemPresent(t, problems, "STALE", "internal/nowhere/gone.go")
	})
}
