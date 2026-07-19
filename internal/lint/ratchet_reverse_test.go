// ratchet_reverse_test.go — spec 120 R6 scans (e) and (f): the
// reverse-derivation pair.
//
// DEFENSE-IN-DEPTH ONLY (the round-5 falsification): derivation
// shapes are UNBOUNDED (TrimPrefix, ReadDir, ReadFile+Unmarshal,
// Index-slice — four shapes in two review rounds), so NO completeness
// claim is made here or anywhere for these two scans. The hard
// guarantee lives at the five authority-bearing CONSUMER classes
// (scans (a)/(b)/(c)/(g)/(h) + the waist); these two scans merely fail
// EARLIER, with better diagnostics, on the two commonest reverse
// shapes:
//
//	(e) TestTrimPrefixReverseDerivationGated_DefenseInDepth —
//	    strings.TrimPrefix of a workspace.*Prefix (an ID parsed back
//	    OUT of a branch/dir name) must have an idvalidate gate in the
//	    same declaration.
//	(f) TestRootEnumerationReverseDerivationGated_DefenseInDepth —
//	    os.ReadDir of the specs/worktrees roots (agent-creatable dir
//	    names) must gate via idvalidate in the same declaration, or
//	    carry an audited non-ID-use justification.
//
// Deleting a gated site's idvalidate call turns the scan RED (proven
// by the delete-the-gate fixtures below).
package lint

import (
	"go/ast"
	"strings"
	"testing"
)

// scanTrimPrefixReverse returns every strings.TrimPrefix call whose
// prefix argument is a workspace.*Prefix constant (selector form, or
// the bare ident inside internal/workspace itself) or one of the
// branch/worktree literal values, where the enclosing declaration has
// NO idvalidate gate.
func scanTrimPrefixReverse(u *rUniverse) (gated, ungated []rSite) {
	for _, f := range u.files {
		strLocal := f.importLocal(stringsImport)
		if strLocal == "" {
			continue
		}
		wsLocal := f.importLocal(workspaceImport)
		inWorkspace := f.pkgDir == "internal/workspace"
		ast.Inspect(f.file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "TrimPrefix" {
				return true
			}
			x, ok := sel.X.(*ast.Ident)
			if !ok || x.Name != strLocal {
				return true
			}
			if len(call.Args) != 2 {
				return true
			}
			prefixName := ""
			switch p := call.Args[1].(type) {
			case *ast.SelectorExpr:
				if px, ok := p.X.(*ast.Ident); ok && wsLocal != "" && px.Name == wsLocal && inlinePrefixSelectors[p.Sel.Name] {
					prefixName = "workspace." + p.Sel.Name
				}
			case *ast.Ident:
				if inWorkspace && inlinePrefixSelectors[p.Name] {
					prefixName = p.Name
				}
			case *ast.BasicLit:
				if v, ok := unquote(p.Value); ok {
					if item, ok := literalMatches(v); ok {
						prefixName = "lit:" + item
					}
				}
			}
			if prefixName == "" {
				return true
			}
			site := rSite{
				Rel:    f.rel,
				Func:   enclosingFunc(f, call.Pos()),
				Detail: "strings.TrimPrefix(_, " + prefixName + ")",
				Line:   u.fset.Position(call.Pos()).Line,
			}
			if subtreeCallsGate(enclosingFuncNode(f, call.Pos()), idvalidateGates) {
				gated = append(gated, site)
			} else {
				ungated = append(ungated, site)
			}
			return true
		})
	}
	return gated, ungated
}

// rootEnumMarkers detect a specs-root / worktrees-root expression:
// the accessor calls or the root literals.
var rootEnumAccessors = map[string]bool{
	"SpecsDir":            true,
	"WorktreesDir":        true,
	"DefaultWorktreesDir": true,
}

func exprHasRootMarker(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if found {
			return false
		}
		switch v := n.(type) {
		case *ast.SelectorExpr:
			if rootEnumAccessors[v.Sel.Name] {
				found = true
				return false
			}
		case *ast.BasicLit:
			if s, ok := unquote(v.Value); ok {
				if strings.Contains(s, ".mindspec/specs") || strings.Contains(s, "docs/specs") || strings.Contains(s, ".worktrees") {
					found = true
					return false
				}
			}
		}
		return true
	})
	return found
}

// scanRootEnumeration returns every os.ReadDir call over a
// specs/worktrees root, split by whether the enclosing declaration
// carries an idvalidate gate.
func scanRootEnumeration(u *rUniverse) (gated, ungated []rSite) {
	for _, f := range u.files {
		osLocal := f.importLocal(osImport)
		if osLocal == "" {
			continue
		}
		for _, decl := range f.file.Decls {
			// Root-tainted idents: assigned (or ranged) from an
			// expression carrying a root marker — one-level trace.
			tainted := map[string]bool{}
			ast.Inspect(decl, func(n ast.Node) bool {
				switch st := n.(type) {
				case *ast.AssignStmt:
					for i, lhs := range st.Lhs {
						id, ok := lhs.(*ast.Ident)
						if !ok || i >= len(st.Rhs) && len(st.Rhs) != 1 {
							continue
						}
						rhs := st.Rhs[0]
						if len(st.Rhs) > i {
							rhs = st.Rhs[i]
						}
						if exprHasRootMarker(rhs) {
							tainted[id.Name] = true
						}
					}
				case *ast.RangeStmt:
					if exprHasRootMarker(st.X) {
						if id, ok := st.Value.(*ast.Ident); ok {
							tainted[id.Name] = true
						}
						if id, ok := st.Key.(*ast.Ident); ok {
							tainted[id.Name] = true
						}
					}
				}
				return true
			})
			ast.Inspect(decl, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "ReadDir" {
					return true
				}
				x, ok := sel.X.(*ast.Ident)
				if !ok || x.Name != osLocal {
					return true
				}
				if len(call.Args) != 1 {
					return true
				}
				arg := call.Args[0]
				argTainted := exprHasRootMarker(arg)
				if !argTainted {
					ast.Inspect(arg, func(m ast.Node) bool {
						if id, ok := m.(*ast.Ident); ok && tainted[id.Name] {
							argTainted = true
							return false
						}
						return true
					})
				}
				if !argTainted {
					return true
				}
				site := rSite{
					Rel:    f.rel,
					Func:   enclosingFunc(f, call.Pos()),
					Detail: "os.ReadDir(<specs/worktrees root>)",
					Line:   u.fset.Position(call.Pos()).Line,
				}
				if subtreeCallsGate(decl, idvalidateGates) {
					gated = append(gated, site)
				} else {
					ungated = append(ungated, site)
				}
				return true
			})
		}
	}
	return gated, ungated
}

type rootEnumEntry struct {
	Count int
	// Justification: why the enumerated names never exercise ID
	// authority at this site (non-ID use). A site whose names ARE
	// treated as IDs must be idvalidate-gated instead, never
	// allowlisted-ungated.
	Justification string
}

// TestTrimPrefixReverseDerivationGated_DefenseInDepth is spec 120 R6
// scan (e) — DEFENSE-IN-DEPTH, not a completeness guarantee (see the
// file doc comment): every workspace-prefix TrimPrefix reverse
// derivation must carry an idvalidate gate in its declaration.
func TestTrimPrefixReverseDerivationGated_DefenseInDepth(t *testing.T) {
	u := loadRatchetUniverse(t)
	gated, ungated := scanTrimPrefixReverse(u)
	if len(gated) == 0 {
		t.Fatal("scan (e) found zero gated TrimPrefix reverse-derivation sites — the scanner has regressed (the D2/inventory sites must appear gated)")
	}
	var problems []string
	for _, s := range ungated {
		problems = append(problems, "UNGATED reverse derivation (add an idvalidate gate before using the trimmed value as an ID): "+s.Key()+" [~line "+itoa(s.Line)+"]")
	}
	failOnProblems(t, "scan (e) TrimPrefix reverse derivation", problems)

	t.Run("fixture_deleted_idvalidate_is_red", func(t *testing.T) {
		// Two-way non-vacuity: the fixture mirrors a real gated site
		// with its idvalidate call DELETED — must be flagged.
		fu := fixtureUniverse(t, "ratchet_trimprefix_ungated.go.txt")
		fgated, fungated := scanTrimPrefixReverse(fu)
		if len(fungated) == 0 {
			t.Fatalf("expected the gate-deleted TrimPrefix fixture to be flagged; gated=%d ungated=%d", len(fgated), len(fungated))
		}
	})

	t.Run("fixture_gated_shape_is_green", func(t *testing.T) {
		// The same shape WITH its idvalidate gate must not be
		// flagged (no false positive on the gated inventory sites).
		fu := fixtureUniverse(t, "ratchet_trimprefix_gated.go.txt")
		fgated, fungated := scanTrimPrefixReverse(fu)
		if len(fungated) != 0 || len(fgated) == 0 {
			t.Fatalf("expected gated fixture to pass; gated=%d ungated=%d", len(fgated), len(fungated))
		}
	})
}

// TestRootEnumerationReverseDerivationGated_DefenseInDepth is spec 120
// R6 scan (f) — DEFENSE-IN-DEPTH, not a completeness guarantee (see
// the file doc comment): os.ReadDir over the specs/worktrees roots
// must gate enumerated names via idvalidate in the same declaration,
// or carry an audited non-ID-use justification.
func TestRootEnumerationReverseDerivationGated_DefenseInDepth(t *testing.T) {
	u := loadRatchetUniverse(t)
	gated, ungated := scanRootEnumeration(u)
	if len(gated) == 0 {
		t.Fatal("scan (f) found zero gated root-enumeration sites — the scanner has regressed (mover.listSpecIDs / spec.List must appear gated)")
	}
	for k, e := range rootEnumerationAllowlist {
		if e.Justification == "" {
			t.Errorf("allowlist entry %q missing its non-ID-use justification", k)
		}
	}
	failOnProblems(t, "scan (f) root enumeration",
		diffSites(ungated, func() map[string]int {
			out := map[string]int{}
			for k, e := range rootEnumerationAllowlist {
				out[k] = e.Count
			}
			return out
		}()))

	t.Run("fixture_ungated_enumeration_flagged", func(t *testing.T) {
		// A specs-root ReadDir whose names reach a Join with no
		// idvalidate in the declaration must be flagged.
		fu := fixtureUniverse(t, "ratchet_rootenum_ungated.go.txt")
		_, fungated := scanRootEnumeration(fu)
		if len(fungated) == 0 {
			t.Fatal("expected the ungated specs-root enumeration fixture to be flagged")
		}
	})

	t.Run("fixture_deleted_allowlisted_site_flagged", func(t *testing.T) {
		// Two-way completeness for (f) (S3): a stale allowlist entry
		// whose enumeration site no longer exists must be flagged,
		// mirroring the (a)/(b)/(c)/(g)/(h) synth-map pattern.
		synth := map[string]int{"internal/nowhere/gone.go deletedFunc os.ReadDir(<specs/worktrees root>)": 1}
		problems := diffSites(nil, synth)
		assertProblemPresent(t, problems, "STALE", "internal/nowhere/gone.go")
	})
}
