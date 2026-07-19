// ratchet_render_test.go — spec 120 R6 scan (h): the raw-ID-render
// scan (TestRawIDRenderForbidden).
//
// Any specID/beadID/epicID-named identifier (or an ID-named selector
// field, the ResolvedWork.SpecID shape) reaching a fmt Printf-family
// render position fails unless it:
//   - routes idrender.Spec/idrender.Bead (the forced-safe ID renderer,
//     Bead 5), or
//   - renders under a %q verb (strconv.Quote semantics — the same
//     forced-safe fallback idrender uses), or
//   - is allowlisted below WITH its covering gate.
//
// REJECTION-PATH matcher (round 11, O3): the name matcher alone is
// blind to a bare `id` variable or a bare `.ID` selector. So in
// addition, any render INSIDE an idvalidate.BeadID/SpecID failure
// branch of the exact expression the failed gate was applied to —
// `if err := idvalidate.BeadID(id); err != nil {
// fmt.Errorf("... %s", id) }`, and the split `err := ...; if err !=
// nil` form — is an ID render of a JUST-REJECTED (malformed) value and
// is matched regardless of the variable's name.
package lint

import (
	"go/ast"
	"go/token"
	"testing"
)

// fmtRenderFuncs maps fmt function name -> index of the format
// argument (-1 = no format string; every argument renders raw).
var fmtRenderFuncs = map[string]int{
	"Print":    -1,
	"Println":  -1,
	"Sprint":   -1,
	"Sprintln": -1,
	"Printf":   0,
	"Sprintf":  0,
	"Errorf":   0,
	"Fprint":   -1,
	"Fprintln": -1,
	"Fprintf":  1,
}

type renderEntry struct {
	Count int
	// Gate names the validation covering the rendered ID (waist,
	// ingress gate, in-func idvalidate) — "validated IDs stay raw" is
	// correct class behavior (spec 120 Provenance precision).
	Gate string
}

// formatVerbs extracts the verb letter for each argument consumed by
// a Printf-style format string. '*' width/precision consume an
// argument slot each (recorded as verb '*').
func formatVerbs(format string) []byte {
	var verbs []byte
	i := 0
	for i < len(format) {
		if format[i] != '%' {
			i++
			continue
		}
		i++
		if i < len(format) && format[i] == '%' {
			i++
			continue
		}
		for i < len(format) {
			c := format[i]
			if c == '*' {
				verbs = append(verbs, '*')
				i++
				continue
			}
			if c == '#' || c == '0' || c == '-' || c == ' ' || c == '+' || c == '.' || (c >= '0' && c <= '9') || c == '[' || c == ']' {
				i++
				continue
			}
			break
		}
		if i < len(format) {
			verbs = append(verbs, format[i])
			i++
		}
	}
	return verbs
}

// exprText renders an ident/selector expression as dotted text
// ("id", "b.ID", "res.SpecID"); "" for anything else.
func exprText(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		if x := exprText(v.X); x != "" {
			return x + "." + v.Sel.Name
		}
	}
	return ""
}

// rejectionScope is one idvalidate failure branch paired with the
// expression texts the failed gate call was applied to (round 11, O3).
type rejectionScope struct {
	pos, end token.Pos
	exprs    map[string]bool
}

// rejectionGateCallees: the gate calls whose FAILURE branch scopes the
// rejection-path matcher.
var rejectionGateCallees = map[string]bool{
	"idvalidate.BeadID": true,
	"idvalidate.SpecID": true,
}

// collectRejectionScopes finds the idvalidate rejection branches in a
// file: `if err := idvalidate.X(v); err != nil { ... }` and the split
// form `err := idvalidate.X(v)` followed by `if err != nil { ... }`
// (the binding clears on any reassignment of the error ident, so a
// LATER unrelated error branch is not falsely scoped).
func collectRejectionScopes(f *rFile) []rejectionScope {
	var scopes []rejectionScope
	gateArgTexts := func(call *ast.CallExpr) map[string]bool {
		if !rejectionGateCallees[calleeText(call.Fun)] {
			return nil
		}
		exprs := map[string]bool{}
		for _, a := range call.Args {
			if t := exprText(a); t != "" {
				exprs[t] = true
			}
		}
		if len(exprs) == 0 {
			return nil
		}
		return exprs
	}
	gateAssign := func(st *ast.AssignStmt) (string, map[string]bool) {
		if len(st.Rhs) != 1 || len(st.Lhs) == 0 {
			return "", nil
		}
		call, ok := st.Rhs[0].(*ast.CallExpr)
		if !ok {
			return "", nil
		}
		exprs := gateArgTexts(call)
		if exprs == nil {
			return "", nil
		}
		errID, ok := st.Lhs[0].(*ast.Ident)
		if !ok {
			return "", nil
		}
		return errID.Name, exprs
	}
	for _, decl := range f.file.Decls {
		pending := map[string]map[string]bool{} // err ident -> gated exprs
		initAssigns := map[*ast.AssignStmt]bool{}
		ast.Inspect(decl, func(n ast.Node) bool {
			switch st := n.(type) {
			case *ast.IfStmt:
				if init, ok := st.Init.(*ast.AssignStmt); ok {
					if _, exprs := gateAssign(init); exprs != nil {
						initAssigns[init] = true
						scopes = append(scopes, rejectionScope{pos: st.Body.Pos(), end: st.Body.End(), exprs: exprs})
						return true
					}
				}
				// Split form: bind the branch testing a pending err.
				for name, exprs := range pending {
					used := false
					ast.Inspect(st.Cond, func(m ast.Node) bool {
						if id, ok := m.(*ast.Ident); ok && id.Name == name {
							used = true
							return false
						}
						return true
					})
					if used {
						scopes = append(scopes, rejectionScope{pos: st.Body.Pos(), end: st.Body.End(), exprs: exprs})
						delete(pending, name)
					}
				}
			case *ast.AssignStmt:
				if initAssigns[st] {
					return true
				}
				if name, exprs := gateAssign(st); exprs != nil {
					pending[name] = exprs
					return true
				}
				for _, lhs := range st.Lhs {
					if id, ok := lhs.(*ast.Ident); ok {
						delete(pending, id.Name)
					}
				}
			}
			return true
		})
	}
	return scopes
}

// scanRawIDRender returns every fmt Printf-family argument position
// carrying an ID-named identifier — or, inside an idvalidate rejection
// branch, the just-rejected expression whatever its name (round 11) —
// that neither routes idrender nor renders under %q.
func scanRawIDRender(u *rUniverse) []rSite {
	var sites []rSite
	for _, f := range u.files {
		fmtLocal := f.importLocal(fmtImport)
		if fmtLocal == "" {
			continue
		}
		idrenderLocal := f.importLocal(idrenderImport)
		// Structural idrender-routing trace: an ident assigned from an
		// idrender.* call inside the same declaration (the
		// `safeBeadID := idrender.Bead(beadID)` shape, Bead 5) is
		// already forced-safe and exempt.
		idrenderVars := map[string]map[string]bool{} // decl func -> vars
		if idrenderLocal != "" {
			ast.Inspect(f.file, func(n ast.Node) bool {
				st, ok := n.(*ast.AssignStmt)
				if !ok {
					return true
				}
				for i, lhs := range st.Lhs {
					id, ok := lhs.(*ast.Ident)
					if !ok || i >= len(st.Rhs) {
						continue
					}
					c, ok := st.Rhs[i].(*ast.CallExpr)
					if !ok {
						continue
					}
					cs, ok := c.Fun.(*ast.SelectorExpr)
					if !ok {
						continue
					}
					cx, ok := cs.X.(*ast.Ident)
					if !ok || cx.Name != idrenderLocal {
						continue
					}
					fn := enclosingFunc(f, st.Pos())
					if idrenderVars[fn] == nil {
						idrenderVars[fn] = map[string]bool{}
					}
					idrenderVars[fn][id.Name] = true
				}
				return true
			})
		}
		rejScopes := collectRejectionScopes(f)
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
			if !ok || x.Name != fmtLocal {
				return true
			}
			// Rejection-path scope (round 11): the union of
			// just-rejected expression texts whose failure branch
			// lexically contains this render.
			scoped := map[string]bool{}
			for _, rs := range rejScopes {
				if rs.pos <= call.Pos() && call.Pos() <= rs.end {
					for t := range rs.exprs {
						scoped[t] = true
					}
				}
			}
			safeVars := idrenderVars[enclosingFunc(f, call.Pos())]
			fmtIdx, ok := fmtRenderFuncs[sel.Sel.Name]
			if !ok {
				return true
			}
			var verbs []byte
			firstArg := 0
			if fmtIdx >= 0 {
				firstArg = fmtIdx + 1
				if fmtIdx < len(call.Args) {
					if bl, ok := call.Args[fmtIdx].(*ast.BasicLit); ok && bl.Kind == token.STRING {
						if v, ok := unquote(bl.Value); ok {
							verbs = formatVerbs(v)
						}
					}
				}
			}
			exclude := func(m ast.Node) bool {
				// Prune idrender.*-routed subtrees and
				// idrender-assigned locals: forced-safe.
				switch v := m.(type) {
				case *ast.CallExpr:
					cs, ok := v.Fun.(*ast.SelectorExpr)
					if !ok {
						return false
					}
					cx, ok := cs.X.(*ast.Ident)
					if !ok {
						return false
					}
					return idrenderLocal != "" && cx.Name == idrenderLocal
				case *ast.Ident:
					return safeVars[v.Name]
				}
				return false
			}
			// matchScoped matches the just-rejected expression itself
			// (bare `id` idents / bare `.ID` selectors the suffix
			// matcher misses), honoring the same idrender/%q pruning.
			matchScoped := func(e ast.Expr) (string, bool) {
				if len(scoped) == 0 {
					return "", false
				}
				found := ""
				ast.Inspect(e, func(m ast.Node) bool {
					if found != "" || m == nil {
						return false
					}
					if exclude(m) {
						return false
					}
					switch v := m.(type) {
					case *ast.SelectorExpr:
						if scoped[exprText(v)] {
							found = v.Sel.Name
							return false
						}
					case *ast.Ident:
						if scoped[v.Name] {
							found = v.Name
							return false
						}
					}
					return true
				})
				return found, found != ""
			}
			for ai := firstArg; ai < len(call.Args); ai++ {
				if fmtIdx >= 0 {
					vi := ai - firstArg
					if vi < len(verbs) && verbs[vi] == 'q' {
						continue // %q = strconv.Quote semantics, forced-safe
					}
				}
				name, found := exprContainsIDIdent(call.Args[ai], exclude)
				if !found {
					name, found = matchScoped(call.Args[ai])
				}
				if !found {
					continue
				}
				sites = append(sites, rSite{
					Rel:    f.rel,
					Func:   enclosingFunc(f, call.Pos()),
					Detail: "fmt." + sel.Sel.Name + "(..." + name + "...)",
					Line:   u.fset.Position(call.Pos()).Line,
				})
			}
			return true
		})
	}
	return sites
}

func renderAllowCounts(entries map[string]renderEntry) map[string]int {
	out := map[string]int{}
	for k, e := range entries {
		out[k] = e.Count
	}
	return out
}

// TestRawIDRenderForbidden is spec 120 R6 scan (h) (AC-14).
func TestRawIDRenderForbidden(t *testing.T) {
	u := loadRatchetUniverse(t)
	sites := scanRawIDRender(u)

	for k, e := range rawIDRenderAllowlist {
		if e.Gate == "" {
			t.Errorf("allowlist entry %q missing its covering gate", k)
		}
	}
	failOnProblems(t, "scan (h) raw ID render",
		diffSites(sites, renderAllowCounts(rawIDRenderAllowlist)))

	t.Run("fixture_ungated_printf_flagged", func(t *testing.T) {
		// An ungated fmt.Printf of a beadID-named value must be
		// flagged; the idrender-routed and %q-rendered shapes in the
		// same fixture must NOT be.
		fu := fixtureUniverse(t, "ratchet_raw_id_render.go.txt")
		fsites := scanRawIDRender(fu)
		problems := diffSites(fsites, renderAllowCounts(rawIDRenderAllowlist))
		assertProblemPresent(t, problems, "UNAUDITED", "fmt.Printf(...beadID...)")
		for _, s := range fsites {
			if s.Func == "renderSafely" {
				t.Errorf("idrender-routed/%%q-rendered fixture shape falsely flagged: %s", s.Key())
			}
		}
	})

	t.Run("fixture_rejection_path_bare_id_flagged", func(t *testing.T) {
		// Round-11 (O3): a bare `id` (and a bare `.ID` selector)
		// rendered raw INSIDE the idvalidate rejection branch is an
		// ID render the suffix matcher misses — must be flagged; the
		// idrender-wrapped sibling stays green.
		fu := fixtureUniverse(t, "ratchet_raw_id_render_rejection.go.txt")
		fsites := scanRawIDRender(fu)
		problems := diffSites(fsites, renderAllowCounts(rawIDRenderAllowlist))
		assertProblemPresent(t, problems, "UNAUDITED", "rejectRawly", "fmt.Errorf(...id...)")
		assertProblemPresent(t, problems, "UNAUDITED", "rejectSelectorRawly")
		for _, s := range fsites {
			if s.Func == "rejectSafely" {
				t.Errorf("idrender-wrapped rejection render falsely flagged: %s", s.Key())
			}
		}
	})

	t.Run("fixture_deleted_allowlisted_site_flagged", func(t *testing.T) {
		synth := map[string]int{"internal/nowhere/gone.go deletedFunc fmt.Printf(...beadID...)": 1}
		problems := diffSites(nil, synth)
		assertProblemPresent(t, problems, "STALE", "internal/nowhere/gone.go")
	})
}
