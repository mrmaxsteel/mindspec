// ratchet_argv_test.go — spec 120 R6 scan (g): the whole-tree,
// wrapper-agnostic bd/git exec-operand audit (TestArgvIDOperandGated).
//
// THE exhaustive, by-construction enforcer of the round-9
// gate-all-ids rule. Every bd/git invocation across cmd/ + internal/
// is resolved SEMANTICALLY at the exec seam / wrapper-call graph —
// never by a fixed wrapper-name list:
//
//   - direct exec.Command / exec.CommandContext spawns whose command
//     operand resolves to "bd"/"git" (string literal, one-level const
//     fold, package execCommand-style seam var — Command AND
//     CommandContext — a single-assignment LOCAL exec-constructor
//     alias like `spawn := exec.Command`, a single-assignment
//     string-literal local, or a variable traced to
//     exec.LookPath("bd"|"git") — the harness Sandbox.runBD shape
//     that is invisible to name matchers). Command resolution FAILS
//     CLOSED (round 11): an operand that cannot be PROVEN
//     non-bd/non-git surfaces as an "exec-unresolved" site that must
//     be classified like any other — a resolved non-bd/non-git
//     literal is the ONLY basis for skipping a spawn;
//   - generic runners whose command operand is a function PARAMETER:
//     their call sites resolve the literal at the caller (and a
//     caller forwarding its own parameter joins the runner set,
//     transitively);
//   - operand-forwarding wrappers: a site classified "caller-audited"
//     promotes its enclosing function into the wrapper set, and every
//     call site of that function (by name, by method selector — incl.
//     nested-selector receivers and method values, resolved
//     fail-closed by bare-name over-approximation, round 11 — by
//     package selector, or through a package/local seam var like
//     `var runBDFn = bead.RunBD` / `run := d.driveBD`) becomes an
//     auditable site itself — iterated to a fixed point, so ANY
//     future wrapper is covered.
//
// RESIDUAL — stated honestly (round 11, F-F3/O1). "Zero unclassified"
// means: every DISCOVERED site carries an audited disposition, and
// spawn discovery itself has no silent skip (unresolved command
// operands are conservatively included). What remains OUTSIDE the
// structural guarantee:
//   - audit prose is trusted: gate-upstream notes and non-id CLASS
//     assignments are audited claims, verified structurally only for
//     gate-in-func/gate-in-file markers, caller-audited taint, and
//     the closed class enum — not full dataflow;
//   - gate checks verify structural PRESENCE of an idvalidate call in
//     the declaration/file, not that it dominates the spawn path;
//   - caller ATTRIBUTION for exotic indirection (a wrapper func value
//     smuggled through an interface, channel, map, or a field named
//     unlike its target) resolves by bare-name over-approximation and
//     can miss the caller — the wrapper's exec SEAM itself is still
//     discovered and classified, so the spawn never escapes; only the
//     caller-side audit obligation could.
//
// Call-site-keyed: every discovered site MUST carry exactly one of
// two real dispositions in the audit table (plus the mechanical
// caller-audited plumbing marker):
//
//	gated         — every ID-position operand passes idvalidate at or
//	                before the call. Markers "gate-in-func:" and
//	                "gate-in-file:" are STRUCTURALLY verified (an
//	                idvalidate/requireValidBeadID call must exist in
//	                the enclosing declaration/file); "gate-upstream:"
//	                is audited prose naming the upstream gate.
//	non-id        — AUDITED-ALLOWLISTED, genuine NON-id operands ONLY.
//	                Round 11: every non-id entry must name its operand
//	                class from the CLOSED TYPED ENUM
//	                (nonIDOperandClass); the enum has deliberately no
//	                class for "id operand with safe provenance", so a
//	                provenance justification — however novel its
//	                wording — cannot create an exemption. The historic
//	                banned-phrase check ("bd-minted", "not
//	                agent-steerable", ...) remains as belt (round 9:
//	                no id operand is trusted by provenance).
//	caller-audited — the site forwards the enclosing function's own
//	                parameters into the argv; the operands are audited
//	                at that function's CALL SITES instead (verified:
//	                the argv must actually be parameter-tainted).
package lint

import (
	"go/ast"
	"go/token"
	"sort"
	"strings"
	"testing"
)

// ---------- audit table schema ----------

type argvDisposition string

const (
	dispGated         argvDisposition = "gated"
	dispNonID         argvDisposition = "non-id"
	dispCallerAudited argvDisposition = "caller-audited"
)

// nonIDOperandClass is the CLOSED TYPED ENUM of permitted non-id
// operand classes (round 11). A non-id allowlist entry is only valid
// when its Class names one of these; the Note is descriptive prose,
// never the authority. There is deliberately NO class for "id operand
// with safe provenance" — round 9's no-provenance-trust rule is thereby
// structural: an id operand cannot be allowlisted at all, however the
// prose is worded, because no permitted class describes it.
type nonIDOperandClass string

const (
	// opClassFrameworkSubcommandOrFlag: framework-authored subcommands,
	// flags, and config keys (bd/git verbs, --flags, git-config keys).
	opClassFrameworkSubcommandOrFlag nonIDOperandClass = "framework-subcommand-or-flag"
	// opClassStringLiteral: in-file string-literal argv (including
	// scenario-authored harness fixtures).
	opClassStringLiteral nonIDOperandClass = "string-literal"
	// opClassWaistComposedBranch: branch/worktree operands composed via
	// the scan-(a)-audited (string,error) waist helpers.
	opClassWaistComposedBranch nonIDOperandClass = "waist-composed-branch"
	// opClassRevParseSHA: git SHAs / rev-parse'd refs (hex-validated by
	// git itself) and merge-base endpoints.
	opClassRevParseSHA nonIDOperandClass = "rev-parse-sha"
	// opClassPathspec: repo-relative paths / "--"-separated pathspecs.
	opClassPathspec nonIDOperandClass = "pathspec"
	// opClassFreeText: genuine free-text operands (titles, metadata,
	// commit messages) in non-id argv positions.
	opClassFreeText nonIDOperandClass = "free-text"
	// opClassNoOperand: no-operand verbs (--version, dolt commit).
	opClassNoOperand nonIDOperandClass = "no-operand"
	// opClassRejectOptionLikeGated: typed non-id parameters behind a
	// RejectOptionLike guard on dynamic refs (the gitutil seam).
	opClassRejectOptionLikeGated nonIDOperandClass = "reject-option-like-gated"
	// opClassCallerForwarded is NOT a permitted non-id class: it is the
	// mechanical marker class for caller-audited plumbing entries only.
	opClassCallerForwarded nonIDOperandClass = "caller-forwarded"
)

// permittedNonIDClasses: the closed set a dispNonID entry may claim.
var permittedNonIDClasses = map[nonIDOperandClass]bool{
	opClassFrameworkSubcommandOrFlag: true,
	opClassStringLiteral:             true,
	opClassWaistComposedBranch:       true,
	opClassRevParseSHA:               true,
	opClassPathspec:                  true,
	opClassFreeText:                  true,
	opClassNoOperand:                 true,
	opClassRejectOptionLikeGated:     true,
}

type argvEntry struct {
	Count int
	Disp  argvDisposition
	// Class: REQUIRED for non-id entries (must be a permitted class
	// from the closed enum) and for caller-audited entries (must be
	// opClassCallerForwarded); FORBIDDEN for gated entries (the gate
	// marker in Note is their authority).
	Class nonIDOperandClass
	// Note: for gated — "gate-in-func: ..."/"gate-in-file: ..."/
	// "gate-upstream: ..."; for non-id — descriptive prose naming the
	// operands (the Class carries the authority); for caller-audited —
	// where the callers are.
	Note string
}

// bannedProvenancePhrases: an allowlist justification resting on id
// provenance is itself a failure (round 9 — bd ids are agent-writable;
// no id operand is trusted by provenance).
var bannedProvenancePhrases = []string{
	"bd-minted",
	"minted by bd",
	"not agent-steerable",
	"not agent steerable",
	"bd-sourced, safe",
	"trusted provenance",
	"ids are trusted",
	"id is trusted",
}

// checkArgvEntrySchema validates one audit-table entry; returns
// problem strings.
func checkArgvEntrySchema(key string, e argvEntry) []string {
	var problems []string
	if e.Note == "" {
		problems = append(problems, "SCHEMA: entry missing note: "+key)
	}
	low := strings.ToLower(e.Note)
	for _, phrase := range bannedProvenancePhrases {
		if strings.Contains(low, phrase) {
			problems = append(problems, "SCHEMA: id-provenance justification is FORBIDDEN (round 9 — no id operand is trusted by provenance): "+key+" note contains "+quote(phrase))
		}
	}
	switch e.Disp {
	case dispGated:
		if !strings.HasPrefix(e.Note, "gate-in-func:") && !strings.HasPrefix(e.Note, "gate-in-file:") && !strings.HasPrefix(e.Note, "gate-upstream:") {
			problems = append(problems, "SCHEMA: gated entry must carry a gate-in-func:/gate-in-file:/gate-upstream: marker: "+key)
		}
		if e.Class != "" {
			problems = append(problems, "SCHEMA: gated entry must not carry an operand class (the gate marker is its authority): "+key)
		}
	case dispNonID:
		// Round 11: the CLOSED TYPED ENUM is the authority — prose
		// alone (however novel its wording) never creates an exemption.
		if e.Class == "" {
			problems = append(problems, "SCHEMA: non-id entry must declare its operand class from the closed enum: "+key)
		} else if !permittedNonIDClasses[e.Class] {
			problems = append(problems, "SCHEMA: "+quote(string(e.Class))+" is not a permitted non-id operand class (round 9/11 — there is no class for id operands, whatever their claimed provenance): "+key)
		}
	case dispCallerAudited:
		if e.Class != opClassCallerForwarded {
			problems = append(problems, "SCHEMA: caller-audited entry must carry the caller-forwarded marker class: "+key)
		}
	default:
		problems = append(problems, "SCHEMA: unknown disposition "+string(e.Disp)+" for "+key)
	}
	return problems
}

// ---------- semantic discovery engine ----------

// argvSite is one discovered bd/git invocation site.
type argvSite struct {
	rSite
	// spawn is true for exec-seam sites; false for wrapper-call sites.
	spawn bool
	// tainted reports whether the operand argv at this site references
	// the enclosing function's parameters (required for
	// caller-audited).
	tainted bool
	// fn is the enclosing top-level function (nil at file scope).
	fn *rFunc
	// call is the AST node, for gate checks.
	call *ast.CallExpr
	file *rFile
}

// argvScanner runs the fixed-point discovery over a universe.
type argvScanner struct {
	u *rUniverse
	// funcIndex: pkgDir -> name -> funcs (methods and plain funcs share
	// the name index; method callers resolve by Sel name in-package).
	funcIndex map[string]map[string][]*rFunc
	// aliasIndex: pkgDir -> var name -> referenced func (seam vars).
	aliasIndex map[string]map[string]*rFunc
	// execCmdVars: pkgDir -> var name bound to exec.Command /
	// exec.CommandContext -> the command-operand arg index (0 for
	// Command, 1 for CommandContext).
	execCmdVars map[string]map[string]int
}

func newArgvScanner(u *rUniverse) *argvScanner {
	s := &argvScanner{
		u:           u,
		funcIndex:   map[string]map[string][]*rFunc{},
		aliasIndex:  map[string]map[string]*rFunc{},
		execCmdVars: map[string]map[string]int{},
	}
	for pkg, fns := range u.funcs {
		s.funcIndex[pkg] = map[string][]*rFunc{}
		for _, fn := range fns {
			s.funcIndex[pkg][fn.name] = append(s.funcIndex[pkg][fn.name], fn)
		}
	}
	// Resolve package seam vars: `var execCommand = exec.Command`,
	// `var execCommandContext = exec.CommandContext`, and
	// `var runBDFn = bead.RunBD` / `var x = localFunc`.
	for pkg, vars := range u.pkgVars {
		s.execCmdVars[pkg] = map[string]int{}
		s.aliasIndex[pkg] = map[string]*rFunc{}
		for name, init := range vars {
			switch v := init.(type) {
			case *ast.SelectorExpr:
				x, ok := v.X.(*ast.Ident)
				if !ok {
					continue
				}
				if idx, ok := execCmdArgIdx(v.Sel.Name); ok && s.pkgImportsAs(pkg, x.Name, execImport) {
					s.execCmdVars[pkg][name] = idx
					continue
				}
				// Cross-package function reference pkglocal.Func.
				if target := s.resolveCrossPkgFunc(pkg, x.Name, v.Sel.Name); target != nil {
					s.aliasIndex[pkg][name] = target
				}
			case *ast.Ident:
				// Same-package function reference.
				if fns := s.funcIndex[pkg][v.Name]; len(fns) == 1 && fns[0].recv == "" {
					s.aliasIndex[pkg][name] = fns[0]
				}
			}
		}
	}
	return s
}

// pkgImportsAs reports whether some file in pkg imports importPath
// under local name.
func (s *argvScanner) pkgImportsAs(pkg, local, importPath string) bool {
	for _, f := range s.u.files {
		if f.pkgDir != pkg {
			continue
		}
		if f.imports[local] == importPath {
			return true
		}
	}
	return false
}

const modulePrefix = "github.com/mrmaxsteel/mindspec/"

// resolveCrossPkgFunc resolves pkglocal.Func from within pkg to the
// target package's function.
func (s *argvScanner) resolveCrossPkgFunc(pkg, local, funcName string) *rFunc {
	for _, f := range s.u.files {
		if f.pkgDir != pkg {
			continue
		}
		imp, ok := f.imports[local]
		if !ok {
			continue
		}
		target := strings.TrimPrefix(imp, modulePrefix)
		if target == imp {
			continue // stdlib / external
		}
		for _, fn := range s.funcIndex[target][funcName] {
			if fn.recv == "" {
				return fn
			}
		}
	}
	return nil
}

// execCmdArgIdx reports the command-operand arg index for an os/exec
// spawn constructor by selector name: Command -> 0, CommandContext ->
// 1 (the ctx precedes the command). ok=false for anything else.
func execCmdArgIdx(sel string) (int, bool) {
	switch sel {
	case "Command":
		return 0, true
	case "CommandContext":
		return 1, true
	}
	return 0, false
}

// funcLocals gathers per-function analysis: LookPath-traced vars,
// single-assignment string-literal locals (round 11 — the
// `program := "bd"` shape), single-assignment LOCAL exec-constructor
// aliases (`spawn := exec.Command` / `exec.CommandContext`, round 11
// delta — var -> command-operand arg index), and parameter taint.
type funcLocals struct {
	lookpath  map[string]string // var -> "bd"/"git"/...
	strlit    map[string]string // SINGLE-assignment var -> string-literal value
	execalias map[string]int    // SINGLE-assignment var -> exec cmd-arg index
	tainted   map[string]bool   // param-tainted idents
}

func analyzeFuncLocals(fn *rFunc) *funcLocals {
	fl := &funcLocals{lookpath: map[string]string{}, strlit: map[string]string{}, execalias: map[string]int{}, tainted: map[string]bool{}}
	for _, p := range fn.params {
		fl.tainted[p] = true
	}
	execLocal := fn.file.importLocal(execImport)
	body := ast.Node(fn.decl)
	// Binding counts (one dedicated pass): strlit / exec-alias
	// resolution is only SOUND for a var bound exactly once in the whole
	// function — a reassigned var stays unresolved and therefore FAILS
	// CLOSED at the spawn (never "resolved to its first value"). Both
	// binding forms count: `:=`/`=` assignment (AssignStmt) AND
	// `var x = …` declaration (DeclStmt→GenDecl(var)→ValueSpec), so a
	// `var spawn = exec.Command` later reassigned still fails closed.
	assignCount := map[string]int{}
	ast.Inspect(body, func(n ast.Node) bool {
		switch st := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range st.Lhs {
				if id, ok := lhs.(*ast.Ident); ok {
					assignCount[id.Name]++
				}
			}
		case *ast.DeclStmt:
			gd, ok := st.Decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				return true
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, nm := range vs.Names {
					assignCount[nm.Name]++
				}
			}
		}
		return true
	})
	// Single-initialization LOCAL exec-constructor aliases declared via
	// `var spawn = exec.Command` / `exec.CommandContext` (round 11 delta
	// — the DeclStmt binding form, the last local function-value alias
	// shape). Same execImport check + reassignment guard as the
	// AssignStmt path below. A `var spawn exec.CmdFunc` with no
	// initializer never binds a value here → stays unresolved → fails
	// closed.
	if execLocal != "" {
		ast.Inspect(body, func(n ast.Node) bool {
			st, ok := n.(*ast.DeclStmt)
			if !ok {
				return true
			}
			gd, ok := st.Decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				return true
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				// Handle single AND tuple binds pairwise (`var a, b =
				// exec.Command, x`), mirroring the AssignStmt path. A spec
				// with no initializer (len(Values)==0) binds no value → skip
				// → stays unresolved → fails closed.
				if !ok || len(vs.Values) != len(vs.Names) {
					continue
				}
				for i, nm := range vs.Names {
					if assignCount[nm.Name] != 1 {
						continue
					}
					if sel, ok := vs.Values[i].(*ast.SelectorExpr); ok {
						if x, ok := sel.X.(*ast.Ident); ok && x.Name == execLocal {
							if idx, ok := execCmdArgIdx(sel.Sel.Name); ok {
								fl.execalias[nm.Name] = idx
							}
						}
					}
				}
			}
			return true
		})
	}
	// Two passes are enough for the shapes in this tree (straight-line
	// assignment chains).
	for pass := 0; pass < 2; pass++ {
		ast.Inspect(body, func(n ast.Node) bool {
			st, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for i, lhs := range st.Lhs {
				id, ok := lhs.(*ast.Ident)
				if !ok {
					continue
				}
				var rhs ast.Expr
				if len(st.Rhs) == len(st.Lhs) {
					rhs = st.Rhs[i]
				} else if len(st.Rhs) == 1 {
					rhs = st.Rhs[0]
				} else {
					continue
				}
				// Single-assignment string-literal trace (round 11):
				// program := "bd".
				if bl, ok := rhs.(*ast.BasicLit); ok && bl.Kind == token.STRING && assignCount[id.Name] == 1 {
					if v, ok := unquote(bl.Value); ok {
						fl.strlit[id.Name] = v
						continue
					}
				}
				// Single-assignment LOCAL exec-constructor alias (round
				// 11 delta): spawn := exec.Command / exec.CommandContext.
				// A reassigned var stays unresolved and FAILS CLOSED at
				// the call site (never "resolved to its first value").
				if execLocal != "" && assignCount[id.Name] == 1 {
					if sel, ok := rhs.(*ast.SelectorExpr); ok {
						if x, ok := sel.X.(*ast.Ident); ok && x.Name == execLocal {
							if idx, ok := execCmdArgIdx(sel.Sel.Name); ok {
								fl.execalias[id.Name] = idx
								continue
							}
						}
					}
				}
				// LookPath trace: x, err := exec.LookPath("bd")
				if call, ok := rhs.(*ast.CallExpr); ok && i == 0 {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "LookPath" {
						if len(call.Args) == 1 {
							if bl, ok := call.Args[0].(*ast.BasicLit); ok && bl.Kind == token.STRING {
								if v, ok := unquote(bl.Value); ok {
									fl.lookpath[id.Name] = v
									continue
								}
							}
						}
					}
				}
				// Param-taint propagation.
				taints := false
				ast.Inspect(rhs, func(m ast.Node) bool {
					if mid, ok := m.(*ast.Ident); ok && fl.tainted[mid.Name] {
						taints = true
						return false
					}
					return true
				})
				if taints {
					fl.tainted[id.Name] = true
				}
			}
			return true
		})
	}
	return fl
}

// resolveCmdOperand resolves a spawn's command-name operand.
// Returns ("bd"/"git"/other literal, "lit") for resolvable strings
// (string literal, package const, LookPath-traced local, or a
// single-assignment string-literal local — round 11), ("", "param")
// when the operand is a function parameter, and ("", "dynamic")
// otherwise. A "dynamic" result is NOT a skip: the caller FAILS CLOSED
// and surfaces the spawn as a site requiring classification.
func (s *argvScanner) resolveCmdOperand(e ast.Expr, f *rFile, fl *funcLocals, paramNames []string) (string, string) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			if val, ok := unquote(v.Value); ok {
				return val, "lit"
			}
		}
	case *ast.Ident:
		if val, ok := s.u.consts[f.pkgDir][v.Name]; ok {
			return val, "lit"
		}
		if fl != nil {
			if val, ok := fl.lookpath[v.Name]; ok {
				return val, "lit"
			}
			if val, ok := fl.strlit[v.Name]; ok {
				return val, "lit"
			}
		}
		for _, p := range paramNames {
			if v.Name == p {
				return "", "param"
			}
		}
	}
	return "", "dynamic"
}

// exprsTainted reports whether any expression references a
// param-tainted ident.
func exprsTainted(args []ast.Expr, fl *funcLocals) bool {
	if fl == nil {
		return false
	}
	for _, a := range args {
		hit := false
		ast.Inspect(a, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && fl.tainted[id.Name] {
				hit = true
				return false
			}
			return true
		})
		if hit {
			return true
		}
	}
	return false
}

// enclosingRFunc finds the rFunc whose decl contains pos.
func (s *argvScanner) enclosingRFunc(f *rFile, pos token.Pos) *rFunc {
	for _, fn := range s.u.funcs[f.pkgDir] {
		if fn.file == f && fn.decl.Pos() <= pos && pos <= fn.decl.End() {
			return fn
		}
	}
	return nil
}

// discover runs the full fixed-point discovery. `table` supplies the
// caller-audited dispositions that promote wrappers.
func (s *argvScanner) discover(table map[string]argvEntry) []argvSite {
	var sites []argvSite
	seen := map[string]bool{} // site identity: file:pos

	addSite := func(f *rFile, call *ast.CallExpr, spawn bool, cmd string, operands []ast.Expr) {
		posKey := f.rel + ":" + itoa(int(call.Pos()))
		if seen[posKey] {
			return
		}
		seen[posKey] = true
		fn := s.enclosingRFunc(f, call.Pos())
		var fl *funcLocals
		if fn != nil {
			fl = analyzeFuncLocals(fn)
		}
		detail := "exec-" + cmd + " " + calleeText(call.Fun)
		if !spawn {
			detail = "call " + calleeText(call.Fun)
		}
		sites = append(sites, argvSite{
			rSite: rSite{
				Rel:    f.rel,
				Func:   enclosingFunc(f, call.Pos()),
				Detail: detail,
				Line:   s.u.fset.Position(call.Pos()).Line,
			},
			spawn:   spawn,
			tainted: exprsTainted(operands, fl),
			fn:      fn,
			call:    call,
			file:    f,
		})
	}

	// Pass 1: direct spawns + param-cmd runner resolution (own fixed
	// point, independent of the table).
	type paramRunner struct {
		fn     *rFunc
		cmdIdx int // index in fn.params of the command-name param
	}
	paramRunners := map[string]paramRunner{} // funcKey -> runner

	directSpawn := func(f *rFile, call *ast.CallExpr, cmdArgIdx int) {
		if len(call.Args) <= cmdArgIdx {
			return
		}
		fn := s.enclosingRFunc(f, call.Pos())
		var fl *funcLocals
		var params []string
		if fn != nil {
			fl = analyzeFuncLocals(fn)
			params = fn.params
		}
		val, kind := s.resolveCmdOperand(call.Args[cmdArgIdx], f, fl, params)
		switch {
		case kind == "lit":
			// A resolved literal is the ONLY basis for skipping: a
			// non-bd/non-git literal is PROVEN out of scope.
			if val == "bd" || val == "git" {
				addSite(f, call, true, val, call.Args[cmdArgIdx+1:])
			}
		case kind == "param" && fn != nil:
			idx := -1
			cmdIdent, _ := call.Args[cmdArgIdx].(*ast.Ident)
			for i, p := range fn.params {
				if cmdIdent != nil && p == cmdIdent.Name {
					idx = i
				}
			}
			if idx >= 0 {
				paramRunners[fn.key()] = paramRunner{fn: fn, cmdIdx: idx}
			}
		default:
			// FAIL CLOSED (round 11): a command operand that cannot be
			// PROVEN non-bd/non-git is conservatively a site REQUIRING
			// classification — never a silent skip. The command operand
			// itself is included in the audited operand slice (it could
			// itself carry the id).
			addSite(f, call, true, "unresolved", call.Args[cmdArgIdx:])
		}
	}

	for _, f := range s.u.files {
		execLocal := f.importLocal(execImport)
		ast.Inspect(f.file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch fun := call.Fun.(type) {
			case *ast.SelectorExpr:
				x, ok := fun.X.(*ast.Ident)
				if !ok || execLocal == "" || x.Name != execLocal {
					return true
				}
				switch fun.Sel.Name {
				case "Command":
					directSpawn(f, call, 0)
				case "CommandContext":
					directSpawn(f, call, 1)
				}
			case *ast.Ident:
				// Package-level exec seam var (var execCommand =
				// exec.Command / var execCommandContext =
				// exec.CommandContext).
				if idx, ok := s.execCmdVars[f.pkgDir][fun.Name]; ok {
					directSpawn(f, call, idx)
					return true
				}
				// LOCAL exec-constructor alias (round 11 delta):
				// `spawn := exec.Command; spawn(program, id)` — the
				// package-var check above never saw it, so consult the
				// enclosing function's single-assignment execalias.
				if fn := s.enclosingRFunc(f, call.Pos()); fn != nil {
					if idx, ok := analyzeFuncLocals(fn).execalias[fun.Name]; ok {
						directSpawn(f, call, idx)
					}
				}
			}
			return true
		})
	}

	// Param-cmd runner fixed point: calls of runners with a literal
	// bd/git command are sites; forwarding callers join the runner set.
	for {
		before := len(paramRunners)
		for _, f := range s.u.files {
			ast.Inspect(f.file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				for _, runner := range s.resolveCalleeFuncs(f, call) {
					pr, ok := paramRunners[runner.key()]
					if !ok || pr.cmdIdx >= len(call.Args) {
						continue
					}
					fn := s.enclosingRFunc(f, call.Pos())
					var fl *funcLocals
					var params []string
					if fn != nil {
						fl = analyzeFuncLocals(fn)
						params = fn.params
					}
					val, kind := s.resolveCmdOperand(call.Args[pr.cmdIdx], f, fl, params)
					switch {
					case kind == "lit":
						if val == "bd" || val == "git" {
							rest := append([]ast.Expr{}, call.Args[:pr.cmdIdx]...)
							rest = append(rest, call.Args[pr.cmdIdx+1:]...)
							addSite(f, call, true, val, rest)
						}
					case kind == "param" && fn != nil:
						idx := -1
						cmdIdent, _ := call.Args[pr.cmdIdx].(*ast.Ident)
						for i, p := range fn.params {
							if cmdIdent != nil && p == cmdIdent.Name {
								idx = i
							}
						}
						if idx >= 0 {
							if _, exists := paramRunners[fn.key()]; !exists {
								paramRunners[fn.key()] = paramRunner{fn: fn, cmdIdx: idx}
							}
						}
					default:
						// FAIL CLOSED (round 11): an unresolvable
						// command operand at a runner call site is a
						// site requiring classification, same as at
						// the exec seam.
						addSite(f, call, true, "unresolved", call.Args)
					}
				}
				return true
			})
		}
		if len(paramRunners) == before {
			break
		}
	}

	// Pass 2: caller-audited wrapper fixed point, driven by the table.
	for {
		// Wrapper funcs = enclosing functions of sites whose table
		// entry says caller-audited.
		wrappers := map[string]*rFunc{}
		for _, site := range sites {
			e, ok := table[site.Key()]
			if !ok || e.Disp != dispCallerAudited || site.fn == nil {
				continue
			}
			wrappers[site.fn.key()] = site.fn
		}
		grew := false
		for _, f := range s.u.files {
			ast.Inspect(f.file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				for _, target := range s.resolveCalleeFuncs(f, call) {
					if _, ok := wrappers[target.key()]; !ok {
						continue
					}
					posKey := f.rel + ":" + itoa(int(call.Pos()))
					if seen[posKey] {
						continue
					}
					addSite(f, call, false, "", call.Args)
					grew = true
				}
				return true
			})
		}
		if !grew {
			break
		}
	}

	sort.Slice(sites, func(i, j int) bool {
		if sites[i].Rel != sites[j].Rel {
			return sites[i].Rel < sites[j].Rel
		}
		return sites[i].Line < sites[j].Line
	})
	return sites
}

// resolveCalleeFuncs resolves a call's callee to candidate declared
// functions: same-package ident calls, method selector calls,
// cross-package pkglocal.Func calls, package seam-var aliases, local
// method-value aliases (`run := d.driveBD; run(...)`), and — FAIL
// CLOSED, round 11 — nested-selector receivers (o.in.driveBD) by
// bare-name over-approximation across the packages the file can see.
// Over-approximation only ever ADDS auditable sites; it never hides
// one.
func (s *argvScanner) resolveCalleeFuncs(f *rFile, call *ast.CallExpr) []*rFunc {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		// Seam-var alias first.
		if target, ok := s.aliasIndex[f.pkgDir][fun.Name]; ok {
			return []*rFunc{target}
		}
		var out []*rFunc
		for _, fn := range s.funcIndex[f.pkgDir][fun.Name] {
			if fn.recv == "" {
				out = append(out, fn)
			}
		}
		if len(out) > 0 {
			return out
		}
		// Local method-value / func-value alias (round 11):
		// `run := d.driveBD` (or `run := someFunc`) then `run(...)`.
		return s.resolveLocalAlias(f, call.Pos(), fun.Name)
	case *ast.SelectorExpr:
		x, ok := fun.X.(*ast.Ident)
		if !ok {
			// Nested selector / call-result receiver (o.in.driveBD,
			// mk().driveBD): the receiver cannot be resolved
			// syntactically. FAIL CLOSED: resolve by bare callee name
			// across the visible packages so a wrapper reached this
			// way still joins the caller-audited fixed point.
			return s.resolveByName(f, fun.Sel.Name)
		}
		// Cross-package pkglocal.Func.
		if imp, ok := f.imports[x.Name]; ok {
			target := strings.TrimPrefix(imp, modulePrefix)
			if target != imp {
				var out []*rFunc
				for _, fn := range s.funcIndex[target][fun.Sel.Name] {
					if fn.recv == "" {
						out = append(out, fn)
					}
				}
				return out
			}
			return nil
		}
		// Method call through a receiver ident (or a func-typed field
		// selected off one): match by bare name across the visible
		// packages — same-package methods resolve as before, and a
		// cross-package method-typed receiver no longer escapes.
		return s.resolveByName(f, fun.Sel.Name)
	}
	return nil
}

// resolveByName resolves a callee by bare name across every package
// visible to f (its own plus its module-internal imports), matching
// both plain functions and methods. Used FAIL-CLOSED when the receiver
// expression cannot be resolved syntactically.
func (s *argvScanner) resolveByName(f *rFile, name string) []*rFunc {
	var out []*rFunc
	seen := map[*rFunc]bool{}
	add := func(fns []*rFunc) {
		for _, fn := range fns {
			if !seen[fn] {
				seen[fn] = true
				out = append(out, fn)
			}
		}
	}
	add(s.funcIndex[f.pkgDir][name])
	for _, imp := range f.imports {
		target := strings.TrimPrefix(imp, modulePrefix)
		if target == imp || target == f.pkgDir {
			continue
		}
		add(s.funcIndex[target][name])
	}
	return out
}

// resolveLocalAlias resolves an ident callee that matched no declared
// function to a local function-value alias in the enclosing function —
// BOTH binding forms: `name := <func expr>` (AssignStmt) and
// `var name = <func expr>` (DeclStmt→GenDecl(var)→ValueSpec), round 11
// fail-closed.
func (s *argvScanner) resolveLocalAlias(f *rFile, pos token.Pos, name string) []*rFunc {
	fn := s.enclosingRFunc(f, pos)
	if fn == nil {
		return nil
	}
	var out []*rFunc
	// handleRHS resolves a single `name = rhs` binding to candidate
	// declared wrapper funcs. A LOCAL exec-constructor alias
	// (`spawn := exec.Command` / `var spawn = exec.Command`) used as a
	// call callee is a direct SPAWN, not a wrapper forward: it is
	// surfaced as a spawn site by the direct-spawn discovery walk (via
	// funcLocals.execalias) and yields NO wrapper here — but is NOT
	// silently dropped as a generic unresolved external selector.
	handleRHS := func(rhs ast.Expr) {
		switch r := rhs.(type) {
		case *ast.SelectorExpr:
			if x, ok := r.X.(*ast.Ident); ok {
				if el := f.importLocal(execImport); el != "" && x.Name == el {
					if _, ok := execCmdArgIdx(r.Sel.Name); ok {
						return
					}
				}
				if imp, ok := f.imports[x.Name]; ok {
					if target := strings.TrimPrefix(imp, modulePrefix); target != imp {
						out = append(out, s.funcIndex[target][r.Sel.Name]...)
					}
					return
				}
			}
			out = append(out, s.resolveByName(f, r.Sel.Name)...)
		case *ast.Ident:
			out = append(out, s.funcIndex[f.pkgDir][r.Name]...)
		}
	}
	ast.Inspect(fn.decl, func(n ast.Node) bool {
		switch st := n.(type) {
		case *ast.AssignStmt:
			for i, lhs := range st.Lhs {
				id, ok := lhs.(*ast.Ident)
				if !ok || id.Name != name || i >= len(st.Rhs) {
					continue
				}
				handleRHS(st.Rhs[i])
			}
		case *ast.DeclStmt:
			gd, ok := st.Decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				return true
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				// Tuple-aware (`var a, b = exec.Command, x`): match the target
				// name pairwise, mirroring the AssignStmt path.
				if !ok || len(vs.Values) != len(vs.Names) {
					continue
				}
				for i, nm := range vs.Names {
					if nm.Name == name {
						handleRHS(vs.Values[i])
					}
				}
			}
		}
		return true
	})
	return out
}

// auditArgvSites classifies discovered sites against the table and
// returns problems (two-way + schema + structural checks).
func auditArgvSites(u *rUniverse, sites []argvSite, table map[string]argvEntry) []string {
	var problems []string
	for k, e := range table {
		problems = append(problems, checkArgvEntrySchema(k, e)...)
	}
	counts := map[string]int{}
	firstLine := map[string]int{}
	byKey := map[string][]argvSite{}
	for _, s := range sites {
		counts[s.Key()]++
		if _, ok := firstLine[s.Key()]; !ok {
			firstLine[s.Key()] = s.Line
		}
		byKey[s.Key()] = append(byKey[s.Key()], s)
	}
	var keys []string
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		e, ok := table[k]
		if !ok {
			problems = append(problems, "UNCLASSIFIED bd/git exec site (classify gated / non-id / caller-audited): "+k+" [~line "+itoa(firstLine[k])+", count "+itoa(counts[k])+"]")
			continue
		}
		if e.Count != counts[k] {
			problems = append(problems, "COUNT drift for "+k+": table expects "+itoa(e.Count)+", scan found "+itoa(counts[k])+" [~line "+itoa(firstLine[k])+"] — re-audit")
		}
		for _, s := range byKey[k] {
			switch e.Disp {
			case dispGated:
				if strings.HasPrefix(e.Note, "gate-in-func:") {
					if !subtreeCallsGate(enclosingFuncNode(s.file, s.call.Pos()), idvalidateGates) {
						problems = append(problems, "GATE MISSING: entry claims gate-in-func but no idvalidate/requireValidBeadID call found in the enclosing declaration: "+k)
					}
				} else if strings.HasPrefix(e.Note, "gate-in-file:") {
					if !subtreeCallsGate(s.file.file, idvalidateGates) {
						problems = append(problems, "GATE MISSING: entry claims gate-in-file but no idvalidate/requireValidBeadID call found in the file: "+k)
					}
				}
			case dispCallerAudited:
				if !s.tainted {
					problems = append(problems, "NOT A WRAPPER: caller-audited entry but the argv at this site does not forward the enclosing function's parameters: "+k)
				}
			}
		}
	}
	var akeys []string
	for k := range table {
		akeys = append(akeys, k)
	}
	sort.Strings(akeys)
	for _, k := range akeys {
		if _, ok := counts[k]; !ok {
			problems = append(problems, "STALE audit entry (site no longer present — delete): "+k)
		}
	}
	return problems
}

// TestArgvIDOperandGated is spec 120 R6 scan (g) (AC-14/AC-27): the
// exhaustive whole-tree bd/git exec-operand audit.
func TestArgvIDOperandGated(t *testing.T) {
	u := loadRatchetUniverse(t)
	s := newArgvScanner(u)
	sites := s.discover(argvAuditTable)
	if len(sites) == 0 {
		t.Fatal("scan (g) discovered zero bd/git exec sites — the scanner has regressed")
	}
	failOnProblems(t, "scan (g) bd/git exec-operand audit",
		auditArgvSites(u, sites, argvAuditTable))

	t.Run("fixture_ungated_id_argv_flagged", func(t *testing.T) {
		// An exec argv carrying a specID-named identifier, absent
		// from the table, must be flagged UNCLASSIFIED.
		fu := fixtureUniverse(t, "ratchet_argv_ungated.go.txt")
		fs := newArgvScanner(fu)
		problems := auditArgvSites(fu, fs.discover(argvAuditTable), argvAuditTable)
		assertProblemPresent(t, problems, "UNCLASSIFIED", "ungatedShow")
	})

	t.Run("fixture_nonid_named_var_still_flagged", func(t *testing.T) {
		// Round-8 call-site-keyed discriminator: the id operand hides
		// in a NON-id-named variable — the site is still flagged,
		// because classification is keyed by call site, never by
		// operand names.
		fu := fixtureUniverse(t, "ratchet_argv_nonid_var.go.txt")
		fs := newArgvScanner(fu)
		problems := auditArgvSites(fu, fs.discover(argvAuditTable), argvAuditTable)
		assertProblemPresent(t, problems, "UNCLASSIFIED", "hiddenOperand")
	})

	t.Run("fixture_new_wrapper_flagged", func(t *testing.T) {
		// Round-10 wrapper-agnostic discriminator: a NEW wrapper that
		// spawns bd via exec.LookPath("bd")+exec.Command(bdPath, ...),
		// matching neither bead.RunBD nor Sandbox.runBD by name, is
		// discovered at its exec seam and flagged; classifying it
		// caller-audited then flags its id-passing CALLER too.
		fu := fixtureUniverse(t, "ratchet_argv_new_wrapper.go.txt")
		fs := newArgvScanner(fu)
		problems := auditArgvSites(fu, fs.discover(argvAuditTable), argvAuditTable)
		assertProblemPresent(t, problems, "UNCLASSIFIED", "myFutureBDDriver")

		// Now classify the wrapper spawn caller-audited: the caller
		// site (callsFutureDriver) must surface for classification.
		synth := map[string]argvEntry{}
		for k, v := range argvAuditTable {
			synth[k] = v
		}
		synth["internal/fixture/ratchet_argv_new_wrapper.go myFutureBDDriver exec-bd exec.Command"] = argvEntry{
			Count: 1, Disp: dispCallerAudited, Class: opClassCallerForwarded,
			Note: "fixture wrapper — operands audited at callers",
		}
		problems = auditArgvSites(fu, fs.discover(synth), synth)
		assertProblemPresent(t, problems, "UNCLASSIFIED", "callsFutureDriver")
	})

	t.Run("fixture_id_provenance_justification_rejected", func(t *testing.T) {
		// Round-9 no-exemption discriminator: an allowlist entry
		// justified by id provenance is ITSELF a test failure.
		problems := checkArgvEntrySchema("internal/fixture/x.go f call runBDFn", argvEntry{
			Count: 1, Disp: dispNonID,
			Note: "operand is bd-minted and therefore not agent-steerable",
		})
		assertProblemPresent(t, problems, "SCHEMA", "id-provenance", "FORBIDDEN")
	})

	t.Run("fixture_deleted_audited_site_flagged", func(t *testing.T) {
		synth := map[string]argvEntry{
			"internal/nowhere/gone.go deletedFunc call runBDFn": {Count: 1, Disp: dispNonID, Class: opClassStringLiteral, Note: "framework-authored literal subcommand only"},
		}
		problems := auditArgvSites(u, nil, synth)
		assertProblemPresent(t, problems, "STALE", "internal/nowhere/gone.go")
	})

	t.Run("fixture_local_cmd_var_flagged", func(t *testing.T) {
		// Round-11 fail-closed discriminator (i): the bd command name
		// hides in a LOCAL variable (program := "bd") — the spawn must
		// still surface for classification via single-assignment
		// propagation.
		fu := fixtureUniverse(t, "ratchet_argv_localvar_cmd.go.txt")
		fs := newArgvScanner(fu)
		problems := auditArgvSites(fu, fs.discover(argvAuditTable), argvAuditTable)
		assertProblemPresent(t, problems, "UNCLASSIFIED", "localVarCmd")
	})

	t.Run("fixture_dynamic_cmd_fail_closed", func(t *testing.T) {
		// Round-11 fail-closed default: a command operand that cannot
		// be PROVEN non-bd/non-git (here an env read) is conservatively
		// a site REQUIRING classification — never silently skipped.
		fu := fixtureUniverse(t, "ratchet_argv_localvar_cmd.go.txt")
		fs := newArgvScanner(fu)
		problems := auditArgvSites(fu, fs.discover(argvAuditTable), argvAuditTable)
		assertProblemPresent(t, problems, "UNCLASSIFIED", "dynamicCmd")
	})

	t.Run("fixture_nested_selector_and_method_value_callers_flagged", func(t *testing.T) {
		// Round-11 caller-resolution discriminator (ii): the wrapper is
		// reached through a nested selector (o.in.driveBD) and a method
		// value (run := d.driveBD). With the spawn classified
		// caller-audited, BOTH callers must surface for classification.
		fu := fixtureUniverse(t, "ratchet_argv_nested_wrapper.go.txt")
		fs := newArgvScanner(fu)
		synth := map[string]argvEntry{}
		for k, v := range argvAuditTable {
			synth[k] = v
		}
		synth["internal/fixture/ratchet_argv_nested_wrapper.go innerDriver.driveBD exec-bd exec.Command"] = argvEntry{
			Count: 1, Disp: dispCallerAudited, Class: opClassCallerForwarded,
			Note: "fixture wrapper — operands audited at callers",
		}
		problems := auditArgvSites(fu, fs.discover(synth), synth)
		assertProblemPresent(t, problems, "UNCLASSIFIED", "nestedSelectorCaller")
		assertProblemPresent(t, problems, "UNCLASSIFIED", "methodValueCaller")
	})

	t.Run("fixture_exec_constructor_alias_flagged", func(t *testing.T) {
		// Round-11 DELTA fail-closed discriminator: a LOCAL exec-seam
		// function-value alias (spawn := exec.Command) and a
		// CommandContext PACKAGE seam var both previously escaped spawn
		// discovery silently. Both bd spawns must now surface as
		// UNCLASSIFIED (fail-closed: no silent skip).
		fu := fixtureUniverse(t, "ratchet_argv_exec_alias.go.txt")
		fs := newArgvScanner(fu)
		problems := auditArgvSites(fu, fs.discover(argvAuditTable), argvAuditTable)
		assertProblemPresent(t, problems, "UNCLASSIFIED", "aliasEscape")
		assertProblemPresent(t, problems, "UNCLASSIFIED", "ctxSeamEscape")
		// The last binding form: `var spawn = exec.Command` (DeclStmt).
		assertProblemPresent(t, problems, "UNCLASSIFIED", "localVarDeclAliasEscape")
		assertProblemPresent(t, problems, "UNCLASSIFIED", "tupleVarDeclAliasEscape")
	})

	t.Run("fixture_novel_provenance_class_rejected", func(t *testing.T) {
		// Round-11 closed-enum discriminator: a NOVEL-worded provenance
		// justification (none of the historic banned spellings) cannot
		// slip through prose review — the entry's CLASS is the
		// authority, and "safe id provenance" is not a permitted non-id
		// operand class.
		problems := checkArgvEntrySchema("internal/fixture/x.go f call runBDFn", argvEntry{
			Count: 1, Disp: dispNonID,
			Class: nonIDOperandClass("canonical-tracker-issue-key"),
			Note:  "canonical issue key from the authoritative tracker, so safe",
		})
		assertProblemPresent(t, problems, "SCHEMA", "not a permitted non-id operand class")

		// An entry that names NO class at all is equally rejected —
		// free-form prose alone never creates an exemption.
		problems = checkArgvEntrySchema("internal/fixture/x.go f call runBDFn", argvEntry{
			Count: 1, Disp: dispNonID, Note: "operands are fine, trust me",
		})
		assertProblemPresent(t, problems, "SCHEMA", "must declare its operand class")
	})

	t.Run("caller_audited_requires_forwarding", func(t *testing.T) {
		// The plumbing marker cannot be abused to exempt a literal
		// site: caller-audited on a non-forwarding site is rejected.
		fu := fixtureUniverse(t, "ratchet_argv_ungated.go.txt")
		fs := newArgvScanner(fu)
		synth := map[string]argvEntry{
			"internal/fixture/ratchet_argv_ungated.go ungatedShow exec-bd exec.Command": {
				Count: 1, Disp: dispCallerAudited, Class: opClassCallerForwarded,
				Note: "bogus — this site passes a local, not a parameter"},
		}
		problems := auditArgvSites(fu, fs.discover(synth), synth)
		assertProblemPresent(t, problems, "NOT A WRAPPER")
	})
}
