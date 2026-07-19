// ratchet_composition_test.go — spec 120 R6 scan (a): the
// composition-helper call-site allowlist
// (TestWorkspaceCompositionCallSiteAllowlist).
//
// Every non-test call of the ten workspace composition helpers across
// cmd/ + internal/ is enumerated and compared TWO-WAY against the
// audited allowlist below. Each entry NAMES its covering gate: the
// waist itself (every helper validates its ID argument internally and
// returns (string, error) — spec 120 R2, Bead 2), plus any early
// D-gate/ingress gate that fires before composition. internal/workspace
// is excluded: it IS the waist (its in-package unit tests are the
// enforcement there, AC-2).
package lint

import (
	"go/ast"
	"testing"
)

// compositionHelpers is the ten-helper waist surface (spec 120 R2).
var compositionHelpers = map[string]bool{
	"SpecBranch":           true,
	"BeadBranch":           true,
	"SpecWorktreeName":     true,
	"BeadWorktreeName":     true,
	"SpecWorktreePath":     true,
	"BeadWorktreePath":     true,
	"FinalizeBranch":       true,
	"FinalizeWorktreeName": true,
	"FinalizeWorktreePath": true,
	"SpecDir":              true,
}

// compEntry is one audited call site of a composition helper.
type compEntry struct {
	Count int
	// Gate names the covering gate. Non-empty required. Every site is
	// covered by the waist itself ("waist" = the helper validates its
	// ID argument in-package and fails closed, AC-2); entries add the
	// upstream ingress/D-gate where one fires earlier.
	Gate string
}

// scanCompositionCalls returns every call site of the ten helpers in
// the universe, outside internal/workspace, keyed by
// file+func+helper.
func scanCompositionCalls(u *rUniverse) []rSite {
	var sites []rSite
	for _, f := range u.files {
		if f.pkgDir == "internal/workspace" {
			continue
		}
		wsLocal := f.importLocal(workspaceImport)
		if wsLocal == "" {
			continue
		}
		ast.Inspect(f.file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			x, ok := sel.X.(*ast.Ident)
			if !ok || x.Name != wsLocal {
				return true
			}
			if !compositionHelpers[sel.Sel.Name] {
				return true
			}
			sites = append(sites, rSite{
				Rel:    f.rel,
				Func:   enclosingFunc(f, call.Pos()),
				Detail: wsLocal + "." + sel.Sel.Name,
				Line:   u.fset.Position(call.Pos()).Line,
			})
			return true
		})
	}
	return sites
}

func allowCounts(entries map[string]compEntry) map[string]int {
	out := map[string]int{}
	for k, e := range entries {
		out[k] = e.Count
	}
	return out
}

// TestWorkspaceCompositionCallSiteAllowlist is spec 120 R6 scan (a)
// (AC-14): two-way over every non-test call of the ten waist helpers.
func TestWorkspaceCompositionCallSiteAllowlist(t *testing.T) {
	u := loadRatchetUniverse(t)
	sites := scanCompositionCalls(u)

	for k, e := range compositionCallAllowlist {
		if e.Gate == "" {
			t.Errorf("allowlist entry %q missing its covering-gate justification", k)
		}
	}
	failOnProblems(t, "scan (a) composition-helper call sites",
		diffSites(sites, allowCounts(compositionCallAllowlist)))

	t.Run("fixture_new_helper_call_flagged", func(t *testing.T) {
		// Negative fixture: a brand-new workspace helper call site
		// must surface as UNAUDITED (two-way, scan side).
		fu := fixtureUniverse(t, "ratchet_new_helper_call.go.txt")
		fsites := scanCompositionCalls(fu)
		problems := diffSites(fsites, allowCounts(compositionCallAllowlist))
		assertProblemPresent(t, problems, "UNAUDITED", "workspace.BeadBranch")
	})

	t.Run("fixture_deleted_allowlisted_site_flagged", func(t *testing.T) {
		// Two-way direction: an allowlist entry whose site no longer
		// exists must surface as STALE (allowlist side).
		synth := map[string]int{"internal/nowhere/gone.go deletedFunc workspace.SpecBranch": 1}
		problems := diffSites(nil, synth)
		assertProblemPresent(t, problems, "STALE", "internal/nowhere/gone.go")
	})
}
