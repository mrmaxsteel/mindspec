// ratchet_universe_test.go — shared AST infrastructure for the spec 120
// R6 consumer-ratchet scans (ADR-0042). The eight scans
// ((a)/(b)/(c)/(d)/(e)/(f)/(g)/(h)) parse every non-test .go file under
// cmd/ + internal/ into a single "universe" and walk it with go/ast,
// following the internal/lint precedent set by boundary_test.go
// (spec 085). Each scan compares its findings TWO-WAY against an
// audited allowlist: an un-audited new site fails on the scan side; a
// stale allowlist entry fails on the allowlist side.
package lint

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// rFile is one parsed non-test Go source file in the ratchet universe.
type rFile struct {
	rel     string // repo-root-relative slash path, e.g. "internal/panel/gate.go"
	pkgDir  string // repo-root-relative slash dir, e.g. "internal/panel"
	file    *ast.File
	imports map[string]string // local name -> import path (unquoted)
}

// rFunc is a top-level function or method declaration.
type rFunc struct {
	file   *rFile
	decl   *ast.FuncDecl
	name   string // bare name, e.g. "runBD"
	recv   string // receiver type name ("" for plain funcs)
	params []string
}

func (f *rFunc) key() string {
	if f.recv != "" {
		return f.file.pkgDir + "." + f.recv + "." + f.name
	}
	return f.file.pkgDir + "." + f.name
}

// rUniverse is the parsed non-test source tree the scans walk.
type rUniverse struct {
	fset  *token.FileSet
	files []*rFile
	// consts: pkgDir -> const name -> string value (package-level
	// string constants, one-level fold like boundary_test.go).
	consts map[string]map[string]string
	// pkgVars: pkgDir -> var name -> initializer expression
	// (package-level var decls; used to resolve seam vars like
	// `var runBDFn = bead.RunBD` and `var execCommand = exec.Command`).
	pkgVars map[string]map[string]ast.Expr
	// funcs: pkgDir -> list of declared funcs/methods.
	funcs map[string][]*rFunc
}

// repoRootDir resolves the repository root from this test file's
// location (the boundary_test.go precedent).
func repoRootDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

// loadRatchetUniverse parses every non-test .go file under cmd/ and
// internal/ (skipping testdata dirs). Cached per-process because the
// eight scans each walk the same tree.
var cachedUniverse *rUniverse

func loadRatchetUniverse(t *testing.T) *rUniverse {
	t.Helper()
	if cachedUniverse != nil {
		return cachedUniverse
	}
	root := repoRootDir(t)
	u := &rUniverse{
		fset:    token.NewFileSet(),
		consts:  map[string]map[string]string{},
		pkgVars: map[string]map[string]ast.Expr{},
		funcs:   map[string][]*rFunc{},
	}
	for _, top := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			name := d.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				return nil
			}
			relOS, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return rerr
			}
			rel := filepath.ToSlash(relOS)
			f, perr := parser.ParseFile(u.fset, path, nil, parser.ParseComments)
			if perr != nil {
				return perr
			}
			u.addFile(rel, f)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", top, err)
		}
	}
	cachedUniverse = u
	return u
}

// addFile registers a parsed file (production tree or fixture) into
// the universe, indexing imports, package-level consts/vars, and
// function declarations.
func (u *rUniverse) addFile(rel string, f *ast.File) {
	rf := &rFile{
		rel:     rel,
		pkgDir:  pathDir(rel),
		file:    f,
		imports: map[string]string{},
	}
	for _, imp := range f.Imports {
		p, ok := unquote(imp.Path.Value)
		if !ok {
			continue
		}
		local := ""
		if imp.Name != nil {
			local = imp.Name.Name
		} else {
			local = p[strings.LastIndex(p, "/")+1:]
		}
		if local == "_" || local == "." {
			continue
		}
		rf.imports[local] = p
	}
	u.files = append(u.files, rf)
	if u.consts[rf.pkgDir] == nil {
		u.consts[rf.pkgDir] = map[string]string{}
	}
	if u.pkgVars[rf.pkgDir] == nil {
		u.pkgVars[rf.pkgDir] = map[string]ast.Expr{}
	}
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			switch d.Tok {
			case token.CONST:
				for _, spec := range d.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, nm := range vs.Names {
						if i >= len(vs.Values) {
							continue
						}
						if bl, ok := vs.Values[i].(*ast.BasicLit); ok && bl.Kind == token.STRING {
							if v, ok := unquote(bl.Value); ok {
								u.consts[rf.pkgDir][nm.Name] = v
							}
						}
					}
				}
			case token.VAR:
				for _, spec := range d.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, nm := range vs.Names {
						if i >= len(vs.Values) {
							continue
						}
						u.pkgVars[rf.pkgDir][nm.Name] = vs.Values[i]
					}
				}
			}
		case *ast.FuncDecl:
			fn := &rFunc{file: rf, decl: d, name: d.Name.Name}
			if d.Recv != nil && len(d.Recv.List) > 0 {
				fn.recv = recvTypeName(d.Recv.List[0].Type)
			}
			if d.Type.Params != nil {
				for _, fld := range d.Type.Params.List {
					for _, nm := range fld.Names {
						fn.params = append(fn.params, nm.Name)
					}
				}
			}
			u.funcs[rf.pkgDir] = append(u.funcs[rf.pkgDir], fn)
		}
	}
}

func pathDir(rel string) string {
	i := strings.LastIndex(rel, "/")
	if i < 0 {
		return "."
	}
	return rel[:i]
}

func recvTypeName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return recvTypeName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr: // generic receiver
		return recvTypeName(t.X)
	}
	return ""
}

// importLocal returns the file-local name that binds importPath, or ""
// when the file does not import it.
func (f *rFile) importLocal(importPath string) string {
	for local, p := range f.imports {
		if p == importPath {
			return local
		}
	}
	return ""
}

const (
	workspaceImport  = "github.com/mrmaxsteel/mindspec/internal/workspace"
	idvalidateImport = "github.com/mrmaxsteel/mindspec/internal/idvalidate"
	idrenderImport   = "github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	filepathImport   = "path/filepath"
	execImport       = "os/exec"
	fmtImport        = "fmt"
	stringsImport    = "strings"
	osImport         = "os"
	beadImport       = "github.com/mrmaxsteel/mindspec/internal/bead"
)

// isIDName reports whether an identifier is spec/bead/epic-ID-named
// per the spec 120 R6 matcher: the lowercased name ends in "specid",
// "beadid", or "epicid" (covers specID, BeadID, rawSpecID, the
// ResolvedWork.SpecID-shaped selector fields, etc.).
func isIDName(name string) bool {
	l := strings.ToLower(name)
	return strings.HasSuffix(l, "specid") || strings.HasSuffix(l, "beadid") || strings.HasSuffix(l, "epicid")
}

// exprContainsIDIdent reports whether the expression subtree contains
// an ID-named identifier or an ID-named selector field, and returns
// the first such name. Callee names are skipped (a call of
// titleFromSpecID(x) is not itself an ID value — its arguments still
// match). `exclude` receives each node first; a true return prunes
// that subtree (used to exempt idrender.*-wrapped args).
func exprContainsIDIdent(e ast.Expr, exclude func(ast.Node) bool) (string, bool) {
	// Collect callee-name idents so function names never match.
	calleeIdents := map[*ast.Ident]bool{}
	ast.Inspect(e, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			calleeIdents[fun] = true
		case *ast.SelectorExpr:
			calleeIdents[fun.Sel] = true
		}
		return true
	})
	found := ""
	ast.Inspect(e, func(n ast.Node) bool {
		if found != "" {
			return false
		}
		if n == nil {
			return true
		}
		if exclude != nil && exclude(n) {
			return false
		}
		switch v := n.(type) {
		case *ast.Ident:
			if !calleeIdents[v] && isIDName(v.Name) {
				found = v.Name
				return false
			}
		case *ast.SelectorExpr:
			if !calleeIdents[v.Sel] && isIDName(v.Sel.Name) {
				found = v.Sel.Name
				return false
			}
		}
		return true
	})
	return found, found != ""
}

// enclosingFunc returns the name of the nearest enclosing top-level
// declaration for a position: "Recv.Name"/"Name" for a FuncDecl, the
// var name for a package-level var initializer (seam closures like
// lifecycle's listOpenBeadsFn), or "<file-scope>".
func enclosingFunc(f *rFile, pos token.Pos) string {
	for _, decl := range f.file.Decls {
		if decl.Pos() > pos || pos > decl.End() {
			continue
		}
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv != nil && len(d.Recv.List) > 0 {
				return recvTypeName(d.Recv.List[0].Type) + "." + d.Name.Name
			}
			return d.Name.Name
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok || spec.Pos() > pos || pos > spec.End() {
					continue
				}
				if len(vs.Names) > 0 {
					return vs.Names[0].Name
				}
			}
		}
	}
	return "<file-scope>"
}

// enclosingFuncNode returns the innermost enclosing declaration node
// (FuncDecl or GenDecl spec) so gate checks can search the SAME
// function body that contains a flagged call.
func enclosingFuncNode(f *rFile, pos token.Pos) ast.Node {
	for _, decl := range f.file.Decls {
		if decl.Pos() <= pos && pos <= decl.End() {
			return decl
		}
	}
	return f.file
}

// subtreeCallsGate reports whether the subtree contains a call to any
// of the named gates. Gate names are "pkglocal.Func" selectors (e.g.
// "idvalidate.BeadID") or bare identifiers (e.g. "requireValidBeadID").
func subtreeCallsGate(root ast.Node, gates []string) bool {
	found := false
	ast.Inspect(root, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := calleeText(call.Fun)
		for _, g := range gates {
			if name == g {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// idvalidateGates is the recognized structural gate-call set: the
// idvalidate validators plus the harness's shared fail-fast helper
// (internal/harness/idgate.go, Bead 5 R7).
var idvalidateGates = []string{
	"idvalidate.SpecID", "idvalidate.BeadID",
	"requireValidBeadID", "requireValidBeadIDs",
	"s.requireValidBeadID", "sandbox.requireValidBeadID",
}

// calleeText renders a callee expression as a dotted name:
// ident "Foo" -> "Foo"; selector x.Sel -> "x.Sel" (one level).
func calleeText(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		if x, ok := v.X.(*ast.Ident); ok {
			return x.Name + "." + v.Sel.Name
		}
		return "?." + v.Sel.Name
	}
	return ""
}

// rSite is one discovered call/concat/render site, keyed for the
// two-way allowlist comparison. Keys are file+func+detail (never line
// numbers, so unrelated edits don't churn the audit tables); Count
// disambiguates multiple same-shaped sites within one function — a
// NEW same-shaped site in the same function changes the count and
// fails two-way.
type rSite struct {
	Rel    string // repo-relative file
	Func   string // enclosing top-level decl
	Detail string // scan-specific shape, e.g. helper name or callee
	Line   int    // diagnostics only — never part of the key
}

func (s rSite) Key() string {
	return s.Rel + " " + s.Func + " " + s.Detail
}

// collapseSites folds a site list into key -> count.
func collapseSites(sites []rSite) map[string]int {
	out := map[string]int{}
	for _, s := range sites {
		out[s.Key()]++
	}
	return out
}

// diffSites performs the two-way comparison between discovered sites
// and an audited allowlist (key -> expected count). Returns
// human-readable problems: unaudited new sites (scan side) and stale
// allowlist entries (allowlist side).
func diffSites(found []rSite, allow map[string]int) []string {
	var problems []string
	counts := collapseSites(found)
	lines := map[string]int{}
	for _, s := range found {
		if _, ok := lines[s.Key()]; !ok {
			lines[s.Key()] = s.Line
		}
	}
	var keys []string
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		want, ok := allow[k]
		if !ok {
			problems = append(problems, "UNAUDITED site (add to allowlist with its covering gate/justification): "+k+" [~line "+itoa(lines[k])+", count "+itoa(counts[k])+"]")
			continue
		}
		if want != counts[k] {
			problems = append(problems, "COUNT drift for "+k+": allowlist expects "+itoa(want)+", scan found "+itoa(counts[k])+" [~line "+itoa(lines[k])+"] — re-audit the new/removed site")
		}
	}
	var akeys []string
	for k := range allow {
		akeys = append(akeys, k)
	}
	sort.Strings(akeys)
	for _, k := range akeys {
		if _, ok := counts[k]; !ok {
			problems = append(problems, "STALE allowlist entry (site no longer present — delete the entry): "+k)
		}
	}
	return problems
}

// parseFixture parses a testdata fixture (.go.txt) into a throwaway
// universe so scans can be exercised against known-bad shapes without
// the fixtures participating in the build.
func fixtureUniverse(t *testing.T, names ...string) *rUniverse {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	td := filepath.Join(filepath.Dir(thisFile), "testdata")
	u := &rUniverse{
		fset:    token.NewFileSet(),
		consts:  map[string]map[string]string{},
		pkgVars: map[string]map[string]ast.Expr{},
		funcs:   map[string][]*rFunc{},
	}
	for _, name := range names {
		full := filepath.Join(td, name)
		src, err := os.ReadFile(full)
		if err != nil {
			t.Fatalf("read fixture %s: %v", name, err)
		}
		f, err := parser.ParseFile(u.fset, full, src, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse fixture %s: %v", name, err)
		}
		u.addFile("internal/fixture/"+strings.TrimSuffix(name, ".txt"), f)
	}
	return u
}

// assertProblemPresent fails unless some problem string contains all
// the given substrings.
func assertProblemPresent(t *testing.T, problems []string, substrs ...string) {
	t.Helper()
	for _, p := range problems {
		ok := true
		for _, s := range substrs {
			if !strings.Contains(p, s) {
				ok = false
				break
			}
		}
		if ok {
			return
		}
	}
	t.Fatalf("expected a problem containing %q, got %d problems:\n  %s",
		substrs, len(problems), strings.Join(problems, "\n  "))
}

// failOnProblems is the standard red path: print every problem and
// fail.
func failOnProblems(t *testing.T, scan string, problems []string) {
	t.Helper()
	if len(problems) == 0 {
		return
	}
	t.Fatalf("%s: %d problem(s):\n  %s", scan, len(problems), strings.Join(problems, "\n  "))
}
