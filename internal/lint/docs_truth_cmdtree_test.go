// docs_truth_cmdtree_test.go builds a structural model of the real
// cobra.Command tree registered by cmd/mindspec, entirely from the AST —
// no build step, no binary, no hand-maintained verb list. It is the R1
// ground truth consumed by docs_truth_test.go (mindspec-ks4u, spec
// w0-docs-truth Bead 2).
//
// # Why AST, not a built binary
//
// The alternative (behavioral: build the binary, exec it, read exit
// codes) is truer to ground truth but costs a build step on every test
// run and needs no special git/bd ratchet accounting. The AST route
// keeps the lint fast, build-free, and hermetic, matching every other
// lint in this package (boundary_test.go, scaffold_test.go). Its one
// real cost is coupling to cmd/mindspec's construction idioms; see the
// stub-detection doc comment below for how that coupling is kept
// structural rather than a hardcoded verb list.
//
// # Tree construction
//
// cmd/mindspec's non-test .go files build cobra.Command values three
// ways, all handled here:
//
//  1. Direct: `var xCmd = &cobra.Command{Use: "...", ...}`.
//  2. One-level function indirection: `var xCmd = helper(args...)` where
//     helper is a same-package func whose body constructs and returns
//     a `*cobra.Command`. Fields on the returned literal that read a
//     helper parameter (e.g. `Use: use`) are resolved by mapping the
//     parameter's position back to the literal argument at the call
//     site — see resolveHelperCall.
//  3. Immediately-invoked func literal: `var xCmd = func() *cobra.Command
//     { c := &cobra.Command{...}; c.AddCommand(...); return c }()`. The
//     literal's body is unwrapped in place (unwrapIIFE): the local
//     variable built from the composite literal becomes this node, and
//     any AddCommand calls on that local become child edges, with
//     inline stubDeprecated(...)-shaped call args becoming anonymous
//     stub children.
//
// Parent/child edges elsewhere in the package (the ordinary case: an
// init() doing `parentCmd.AddCommand(childCmd)`) are collected by a
// single pass over every CallExpr in every file.
//
// # Stub detection (R5) — the discriminator, and why it is not
// Hidden/DisableFlagParsing
//
// Every genuine command in cmd/mindspec sets `RunE` (so cobra's error
// surface and exit-code plumbing apply uniformly). Grepping the whole
// non-test package for `Run: func` (the non-error-returning field)
// currently matches exactly two sites, both in deprecated_commands.go:
// stubDeprecated's returned literal (used by 5 of the 6 one-shot
// redirect stubs) and the hand-rolled bare-`agentmind` parent stub. Both
// exist for the one documented, load-bearing reason stated in that
// file's own doc comment: cobra's default RunE error surface can't
// guarantee "exactly one stderr line, exit code 2". A command that sets
// `Run` but not `RunE` is therefore treated as a deprecation stub here.
// `Hidden` and `DisableFlagParsing` are NOT part of this signal — both
// are also true of the live hidden alias `spec-init`
// (cmd/mindspec/spec_init.go), which sets neither Run nor RunE inline
// (its RunE is wired via assignment in init()) and so is correctly
// classified live. See TestSpecInitAliasResolvesLive /
// TestBenchStubDoesNotResolve below and docs_truth_test.go's R5 cases.
//
// This signal needs no name of deprecated_commands.go or of any verb:
// if that file is deleted (its own header says a follow-up will, after
// one release), grepping for `Run: func` in the remaining files finds
// nothing, no node is ever marked stub, and the six retired verbs
// simply vanish from the tree — R1 catches them as plain unresolved
// verbs. TestCmdTreeDegradesWithoutDeprecatedFile below proves this on
// a copy of the tree with that file physically removed.
package lint

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// cmdNode is one node of the reconstructed cobra command tree.
type cmdNode struct {
	Use      string
	Name     string // first whitespace-delimited field of Use
	IsStub   bool
	NoArgs   bool            // literal sets Args: cobra.NoArgs (O2-8)
	Flags    map[string]bool // flags registered directly on this node (Flags()/PersistentFlags()), PLUS cobra's auto-registered --help/-h (every node) and --version (root, when its literal sets Version) — see autoFlags (O2-6)
	Persist  map[string]bool // flags registered via PersistentFlags() — inherited by descendants
	Parent   *cmdNode
	Children []*cmdNode
}

func (n *cmdNode) findChild(name string) *cmdNode {
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// hasFlag reports whether name is registered on n or inherited from an
// ancestor's persistent flag set.
func (n *cmdNode) hasFlag(name string) bool {
	for cur := n; cur != nil; cur = cur.Parent {
		if cur.Flags[name] {
			return true
		}
		if cur.Persist[name] {
			return true
		}
	}
	return false
}

// path returns the dotted verb path for diagnostics, e.g. "panel disposition validate".
func (n *cmdNode) path() string {
	var parts []string
	for cur := n; cur != nil; cur = cur.Parent {
		parts = append([]string{cur.Name}, parts...)
	}
	return strings.Join(parts, " ")
}

// resolveResult is the outcome of resolving a documented invocation's
// word list against a cmdTree.
type resolveResult struct {
	Resolved bool
	Reason   string // populated when !Resolved
	Node     *cmdNode
}

// resolve walks words (already whitespace-tokenized, words[0] expected
// to be the root's own name, e.g. "mindspec") down the tree as far as
// real children exist, then treats any remaining non-flag words as
// positional arguments (a leaf command's own business, not this
// resolver's — except when the deepest node reached sets Args:
// cobra.NoArgs, O2-8's "at minimum" arity check: see the NoArgs guard
// below) and checks any `--flag`/`-h` words against the deepest node
// reached (plus its ancestors' persistent flags).
func (root *cmdNode) resolve(words []string) resolveResult {
	if len(words) == 0 {
		return resolveResult{Resolved: false, Reason: "empty invocation"}
	}
	if words[0] != root.Name {
		return resolveResult{Resolved: false, Reason: fmt.Sprintf("root name mismatch: %q", words[0])}
	}
	cur := root
	i := 1
	for i < len(words) {
		w := words[i]
		if strings.HasPrefix(w, "-") {
			break
		}
		if len(cur.Children) == 0 {
			break // leaf command: remaining words are positional args
		}
		child := cur.findChild(w)
		if child == nil {
			return resolveResult{Resolved: false, Reason: fmt.Sprintf("no subcommand %q under %q", w, cur.path())}
		}
		cur = child
		i++
	}
	if cur.IsStub {
		return resolveResult{Resolved: false, Reason: fmt.Sprintf("%q resolves to a one-shot deprecation stub (Run, not RunE)", cur.path())}
	}
	// Single pass over the trailing words: validate every --flag/-h
	// against cur, and collect whatever is left as candidate
	// positionals for the NoArgs check below. A long flag not already
	// in `--name=value` form is heuristically assumed to consume the
	// NEXT word as its value (e.g. `report list --resolve abc123`,
	// report.go:125) — this package has no flag-type info (string vs
	// bool) to do better, and over-permissively skipping a value-taking
	// flag's argument is the safe direction for a truth LINT (a missed
	// arity violation, not a false positive on a true claim).
	var positionals []string
	skipNextAsFlagValue := false
	for _, w := range words[i:] {
		if skipNextAsFlagValue {
			skipNextAsFlagValue = false
			continue
		}
		switch {
		case strings.HasPrefix(w, "--"):
			name := strings.TrimPrefix(w, "--")
			hasInlineValue := false
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				name = name[:eq]
				hasInlineValue = true
			}
			if !cur.hasFlag(name) {
				return resolveResult{Resolved: false, Reason: fmt.Sprintf("flag --%s not registered on %q", name, cur.path())}
			}
			if !hasInlineValue {
				skipNextAsFlagValue = true
			}
		case w == "-h":
			if !cur.hasFlag("h") {
				return resolveResult{Resolved: false, Reason: fmt.Sprintf("flag -h not registered on %q", cur.path())}
			}
		default:
			positionals = append(positionals, w)
		}
	}
	if cur.NoArgs {
		for _, w := range positionals {
			if isPlaceholderWord(w) {
				continue
			}
			return resolveResult{Resolved: false, Reason: fmt.Sprintf("%q does not accept positional arguments (Args: cobra.NoArgs), got %q", cur.path(), w)}
		}
	}
	return resolveResult{Resolved: true, Node: cur}
}

// isPlaceholderWord reports whether w is doc metasyntax for "some
// value goes here" (`<bead-id>`, `[--flag]`, a quoted string) rather
// than a literal positional argument a doc author actually typed, so
// the NoArgs arity check above doesn't false-positive on the ordinary
// placeholder forms docs use to describe a command's syntax. (Only
// reachable today from the NoArgs guard; kept general so a future
// ExactArgs/MaximumNArgs extension — see the package doc comment's
// residue list — can reuse it without a second definition.)
func isPlaceholderWord(w string) bool {
	if strings.HasPrefix(w, "<") || strings.HasPrefix(w, "[") {
		return true
	}
	return len(w) >= 2 && (w[0] == '"' || w[0] == '\'')
}

// --- AST extraction -------------------------------------------------

// commandDef is the intermediate record for one discovered
// cobra.Command construction, keyed by its package-level var name (or
// "" for anonymous stub children built inline, e.g.
// stubDeprecated("serve", ...) passed directly to AddCommand).
type commandDef struct {
	VarName      string
	Use          string
	HasRun       bool
	HasRunE      bool
	HasVersion   bool // literal sets a Version field (root.go:57) — seeds the auto --version flag (O2-6)
	NoArgs       bool // literal sets Args: cobra.NoArgs (O2-8)
	flagsByVar   map[string]bool
	persistByVar map[string]bool
}

func (d commandDef) isStub() bool { return d.HasRun && !d.HasRunE }

// edge is a parent->child AddCommand relationship. Child is either a
// package var name (resolved against the defs map) or, for an inline
// anonymous stub, empty with AnonChild populated directly.
type edge struct {
	Parent    string
	Child     string
	AnonChild *commandDef
}

// cmdTreeBuild is the parsed-but-not-yet-linked result of scanning a
// cmd/mindspec-shaped directory.
type cmdTreeBuild struct {
	Defs      map[string]*commandDef
	Edges     []edge
	FieldDefs map[string]*ast.FuncDecl // helper functions returning *cobra.Command, by name
}

// buildCmdTree parses every non-test .go file in dir (a package `main`
// directory shaped like cmd/mindspec) and returns the reconstructed
// root cobra.Command tree, rooted at the var named "rootCmd" whose Use
// is "mindspec". Returns an error only on unparseable Go source or a
// missing/malformed rootCmd — never on the absence of any particular
// command file (see the package doc comment's degradation guarantee).
func buildCmdTree(dir string) (*cmdNode, error) {
	build, err := scanCmdDir(dir)
	if err != nil {
		return nil, err
	}
	return linkCmdTree(build)
}

func scanCmdDir(dir string) (*cmdTreeBuild, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}
	fset := token.NewFileSet()
	build := &cmdTreeBuild{
		Defs:      map[string]*commandDef{},
		FieldDefs: map[string]*ast.FuncDecl{},
	}
	var files []*ast.File
	for _, ent := range entries {
		name := ent.Name()
		if ent.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		full := filepath.Join(dir, name)
		f, err := parser.ParseFile(fset, full, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", full, err)
		}
		files = append(files, f)
	}

	// Pass 0: index every top-level func decl (candidates for the
	// one-level helper-indirection rule).
	for _, f := range files {
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil {
				build.FieldDefs[fn.Name.Name] = fn
			}
		}
	}

	// Pass 1: package-level var decls that construct a *cobra.Command,
	// directly, via one-level helper indirection, or via an IIFE.
	for _, f := range files {
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					build.processVarValue(name.Name, vs.Values[i])
				}
			}
		}
	}

	// Pass 2: every top-level `<ident>.AddCommand(<arg>)` call anywhere
	// in the package (init() functions, registration helpers, etc.).
	// IIFE-internal AddCommand calls are already captured by
	// processVarValue's IIFE unwrap and are not re-walked here (their
	// receiver is a func-local var, not a package-level one, so this
	// generic walk — keyed by known package var names — never matches
	// them twice).
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "AddCommand" {
				return true
			}
			recv, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			if _, known := build.Defs[recv.Name]; !known {
				return true // not a tracked package-level command var (e.g. a local IIFE var)
			}
			if len(call.Args) != 1 {
				return true
			}
			build.addEdgeFromArg(recv.Name, call.Args[0])
			return true
		})
	}

	// Pass 3: flag registrations anywhere in the package:
	// <ident>.Flags().Method(name, ...) / <ident>.PersistentFlags().Method(name, ...).
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			methodName := sel.Sel.Name
			inner, ok := sel.X.(*ast.CallExpr)
			if !ok {
				return true
			}
			innerSel, ok := inner.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			var persistent bool
			switch innerSel.Sel.Name {
			case "Flags":
				persistent = false
			case "PersistentFlags":
				persistent = true
			default:
				return true
			}
			recv, ok := innerSel.X.(*ast.Ident)
			if !ok {
				return true
			}
			def, known := build.Defs[recv.Name]
			if !known {
				return true
			}
			argIdx := 0
			if strings.HasSuffix(methodName, "VarP") || strings.HasSuffix(methodName, "Var") {
				argIdx = 1
			}
			if len(call.Args) <= argIdx {
				return true
			}
			flagName, ok := stringLitValue(call.Args[argIdx])
			if !ok {
				return true
			}
			if def.flagsByVar == nil {
				def.flagsByVar = map[string]bool{}
			}
			if def.persistByVar == nil {
				def.persistByVar = map[string]bool{}
			}
			if persistent {
				def.persistByVar[flagName] = true
			} else {
				def.flagsByVar[flagName] = true
			}
			return true
		})
	}

	return build, nil
}

// processVarValue attempts to classify a package-level var's RHS
// expression as a cobra.Command construction (direct, helper-call, or
// IIFE) and, if so, registers it (and any IIFE-internal AddCommand
// edges) into the build.
func (b *cmdTreeBuild) processVarValue(varName string, rhs ast.Expr) {
	// Direct: `&cobra.Command{...}`.
	if u, ok := rhs.(*ast.UnaryExpr); ok && u.Op == token.AND {
		if cl, ok := u.X.(*ast.CompositeLit); ok && isCobraCommandType(cl.Type) {
			use, hasRun, hasRunE, hasVersion, noArgs := inspectCommandLiteral(cl)
			b.Defs[varName] = &commandDef{VarName: varName, Use: use, HasRun: hasRun, HasRunE: hasRunE, HasVersion: hasVersion, NoArgs: noArgs}
			return
		}
	}
	// IIFE: `func() *cobra.Command { ... }()`.
	if body, ok := isIIFECommandCall(rhs); ok {
		def, edges := b.unwrapIIFE(varName, body)
		if def != nil {
			def.VarName = varName
			b.Defs[varName] = def
			b.Edges = append(b.Edges, edges...)
		}
		return
	}
	// One-level helper indirection: `helper(args...)`.
	if call, ok := rhs.(*ast.CallExpr); ok {
		if def, edges := b.resolveHelperCall(call, varName); def != nil {
			def.VarName = varName
			b.Defs[varName] = def
			b.Edges = append(b.Edges, edges...)
		}
	}
}

func isCobraCommandType(t ast.Expr) bool {
	sel, ok := t.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "cobra" && sel.Sel.Name == "Command"
}

// inspectCommandLiteral extracts Use/Run/RunE/Version/Args presence
// from a &cobra.Command{...} composite literal's key-value fields.
func inspectCommandLiteral(cl *ast.CompositeLit) (use string, hasRun, hasRunE, hasVersion, noArgs bool) {
	for _, elt := range cl.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "Use":
			if s, ok := stringLitValue(kv.Value); ok {
				use = s
			}
		case "Run":
			hasRun = true
		case "RunE":
			hasRunE = true
		case "Version":
			hasVersion = true
		case "Args":
			if isCobraNoArgs(kv.Value) {
				noArgs = true
			}
		}
	}
	return use, hasRun, hasRunE, hasVersion, noArgs
}

// isCobraNoArgs reports whether e is the literal `cobra.NoArgs` — the
// one Args validator O2-8's "at minimum" fix enforces. ExactArgs(n),
// MaximumNArgs(n), and the other cobra.PositionalArgs forms are a
// documented remaining gap (package doc comment).
func isCobraNoArgs(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "cobra" && sel.Sel.Name == "NoArgs"
}

// resolveHelperCall implements the one-level function-call indirection
// rule: v is `helper(args...)` where helper is a same-package function
// whose body's sole cobra.Command construction is `return
// &cobra.Command{...}` (optionally via a named result / local var).
// Fields on that literal that read one of helper's own parameters are
// resolved back through args at this call site.
//
// O2-3: it ALSO walks the helper's body for `<local>.AddCommand(...)`
// and `<local>.Flags()/PersistentFlags().<Method>(name, ...)` calls on
// the same local the returned literal was built from — the extraction
// unwrapIIFE already performs for an inline IIFE body, reused here via
// addCommandArgsForLocal/flagCallsForLocal — so a command built by
// one-level helper indirection (`reportCmd = newReportCmd()`,
// report.go:81-92) is modeled with its real children and flags instead
// of as a childless, flagless leaf: without this, ANY invented
// subcommand under such a parent resolves true (the leaf
// positional-args arm swallows it), and a real flag on it
// false-positives as unregistered, which is worse — it turns a TRUE
// documented claim red and creates pressure to add an exception row.
//
// selfName is the var name this call's result will be known by in the
// tree (the caller's own package-level var name, or a freshly minted
// anonymous name for an inline helper call used directly as an
// AddCommand argument) — used to parent any children discovered here,
// and to seed further-nested anonymous children's names.
func (b *cmdTreeBuild) resolveHelperCall(call *ast.CallExpr, selfName string) (*commandDef, []edge) {
	fnIdent, ok := call.Fun.(*ast.Ident)
	if !ok {
		return nil, nil
	}
	fn, ok := b.FieldDefs[fnIdent.Name]
	if !ok || fn.Body == nil {
		return nil, nil
	}
	localName, cl := findReturnedCommandLiteralVar(fn.Body)
	if cl == nil {
		return nil, nil
	}
	paramIndex := map[string]int{}
	if fn.Type.Params != nil {
		idx := 0
		for _, field := range fn.Type.Params.List {
			for _, n := range field.Names {
				paramIndex[n.Name] = idx
				idx++
			}
		}
	}
	var use string
	var hasRun, hasRunE, hasVersion, noArgs bool
	for _, elt := range cl.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "Use":
			if s, ok := stringLitValue(kv.Value); ok {
				use = s
			} else if paramRef, ok := kv.Value.(*ast.Ident); ok {
				if idx, isParam := paramIndex[paramRef.Name]; isParam && idx < len(call.Args) {
					if s, ok := stringLitValue(call.Args[idx]); ok {
						use = s
					}
				}
			}
		case "Run":
			hasRun = true
		case "RunE":
			hasRunE = true
		case "Version":
			hasVersion = true
		case "Args":
			if isCobraNoArgs(kv.Value) {
				noArgs = true
			}
		}
	}

	flagsByVar, persistByVar := flagCallsForLocal(fn.Body, localName)

	var edges []edge
	anonCount := 0
	for _, arg := range addCommandArgsForLocal(fn.Body, localName) {
		switch a := arg.(type) {
		case *ast.Ident:
			edges = append(edges, edge{Parent: selfName, Child: a.Name})
		case *ast.CallExpr:
			anonCount++
			childName := fmt.Sprintf("%s#anon%d", selfName, anonCount)
			if childDef, childEdges := b.resolveHelperCall(a, childName); childDef != nil {
				childDef.VarName = childName
				edges = append(edges, edge{Parent: selfName, AnonChild: childDef})
				edges = append(edges, childEdges...)
			}
		}
	}

	def := &commandDef{Use: use, HasRun: hasRun, HasRunE: hasRunE, HasVersion: hasVersion, NoArgs: noArgs, flagsByVar: flagsByVar, persistByVar: persistByVar}
	return def, edges
}

// findReturnedCommandLiteralVar looks for a `return &cobra.Command{...}`
// (possibly `return localVar` where localVar was assigned from such a
// literal earlier in the same block) inside body. One level of local
// alias resolution only. Returns the local variable name the literal
// was built from — fed to flagCallsForLocal/addCommandArgsForLocal by
// resolveHelperCall (O2-3) — or "" when the literal is returned
// directly with no intermediate local (e.g. stubDeprecated's `return
// &cobra.Command{...}`, deprecated_commands.go:66), in which case
// those two scans correctly find nothing (no real identifier is ever
// named "").
func findReturnedCommandLiteralVar(body *ast.BlockStmt) (localName string, cl *ast.CompositeLit) {
	locals := map[string]*ast.CompositeLit{}
	for _, stmt := range body.List {
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			if len(s.Lhs) == 1 && len(s.Rhs) == 1 {
				if id, ok := s.Lhs[0].(*ast.Ident); ok {
					if u, ok := s.Rhs[0].(*ast.UnaryExpr); ok && u.Op == token.AND {
						if lit, ok := u.X.(*ast.CompositeLit); ok && isCobraCommandType(lit.Type) {
							locals[id.Name] = lit
						}
					}
				}
			}
		case *ast.ReturnStmt:
			if len(s.Results) == 1 {
				if u, ok := s.Results[0].(*ast.UnaryExpr); ok && u.Op == token.AND {
					if lit, ok := u.X.(*ast.CompositeLit); ok && isCobraCommandType(lit.Type) {
						cl = lit
						localName = ""
					}
				} else if id, ok := s.Results[0].(*ast.Ident); ok {
					if lit, ok := locals[id.Name]; ok {
						cl = lit
						localName = id.Name
					}
				}
			}
		}
	}
	return localName, cl
}

// flagCallsForLocal scans node for `<local>.Flags().Method(name, ...)`
// / `<local>.PersistentFlags().Method(name, ...)` calls and returns the
// flag names found, split by persistence. The same extraction Pass 3
// (scanCmdDir) performs for a known PACKAGE-LEVEL var, generalized to
// any AST scope (a helper function body, an IIFE body) and any local
// identifier — Pass 3 can't see flags registered on a helper's local
// var (e.g. `c.Flags().String(...)` inside newReportListCmd,
// report.go:125-126), because its receiver-identity check requires the
// receiver to already be a tracked package-level def.
func flagCallsForLocal(node ast.Node, localName string) (flags, persist map[string]bool) {
	flags = map[string]bool{}
	persist = map[string]bool{}
	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		methodName := sel.Sel.Name
		inner, ok := sel.X.(*ast.CallExpr)
		if !ok {
			return true
		}
		innerSel, ok := inner.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		var isPersistent bool
		switch innerSel.Sel.Name {
		case "Flags":
			isPersistent = false
		case "PersistentFlags":
			isPersistent = true
		default:
			return true
		}
		recv, ok := innerSel.X.(*ast.Ident)
		if !ok || recv.Name != localName {
			return true
		}
		argIdx := 0
		if strings.HasSuffix(methodName, "VarP") || strings.HasSuffix(methodName, "Var") {
			argIdx = 1
		}
		if len(call.Args) <= argIdx {
			return true
		}
		flagName, ok := stringLitValue(call.Args[argIdx])
		if !ok {
			return true
		}
		if isPersistent {
			persist[flagName] = true
		} else {
			flags[flagName] = true
		}
		return true
	})
	return flags, persist
}

// addCommandArgsForLocal scans node for single-argument
// `<local>.AddCommand(<arg>)` calls and returns each arg. Mirrors
// unwrapIIFE's existing single-arg-only extraction (a variadic
// `AddCommand(a, b, c)` registration is a separate, out-of-scope
// defect — O1-5 — left as-is here rather than incidentally fixed as a
// side effect of this scan).
func addCommandArgsForLocal(node ast.Node, localName string) []ast.Expr {
	var out []ast.Expr
	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "AddCommand" {
			return true
		}
		recv, ok := sel.X.(*ast.Ident)
		if !ok || recv.Name != localName || len(call.Args) != 1 {
			return true
		}
		out = append(out, call.Args[0])
		return true
	})
	return out
}

// isIIFECommandCall reports whether rhs is `func() *cobra.Command {
// ... }()` and, if so, returns the func literal's body.
func isIIFECommandCall(rhs ast.Expr) (*ast.BlockStmt, bool) {
	call, ok := rhs.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return nil, false
	}
	flit, ok := call.Fun.(*ast.FuncLit)
	if !ok || flit.Type.Results == nil || len(flit.Type.Results.List) != 1 {
		return nil, false
	}
	star, ok := flit.Type.Results.List[0].Type.(*ast.StarExpr)
	if !ok || !isCobraCommandType(star.X) {
		return nil, false
	}
	return flit.Body, true
}

// unwrapIIFE processes an immediately-invoked func literal's body:
// finds the local `c := &cobra.Command{...}` that becomes varName's
// def, and every `c.AddCommand(<arg>)` call on that same local becomes
// a child edge (with inline stubDeprecated(...)-shaped call args
// resolved to anonymous stub commandDefs via resolveHelperCall).
func (b *cmdTreeBuild) unwrapIIFE(varName string, body *ast.BlockStmt) (*commandDef, []edge) {
	var localName string
	var def *commandDef
	for _, stmt := range body.List {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			continue
		}
		id, ok := assign.Lhs[0].(*ast.Ident)
		if !ok {
			continue
		}
		u, ok := assign.Rhs[0].(*ast.UnaryExpr)
		if !ok || u.Op != token.AND {
			continue
		}
		cl, ok := u.X.(*ast.CompositeLit)
		if !ok || !isCobraCommandType(cl.Type) {
			continue
		}
		use, hasRun, hasRunE, hasVersion, noArgs := inspectCommandLiteral(cl)
		def = &commandDef{Use: use, HasRun: hasRun, HasRunE: hasRunE, HasVersion: hasVersion, NoArgs: noArgs}
		localName = id.Name
	}
	if def == nil {
		return nil, nil
	}
	var edges []edge
	anonCount := 0
	for _, arg := range addCommandArgsForLocal(body, localName) {
		switch a := arg.(type) {
		case *ast.Ident:
			edges = append(edges, edge{Parent: varName, Child: a.Name})
		case *ast.CallExpr:
			anonCount++
			childName := fmt.Sprintf("%s#anon%d", varName, anonCount)
			if childDef, childEdges := b.resolveHelperCall(a, childName); childDef != nil {
				childDef.VarName = childName
				edges = append(edges, edge{Parent: varName, AnonChild: childDef})
				edges = append(edges, childEdges...)
			}
		}
	}
	return def, edges
}

// addEdgeFromArg records parent->arg as an edge, resolving arg as
// either a known var reference or an inline helper call (anonymous
// stub child).
func (b *cmdTreeBuild) addEdgeFromArg(parent string, arg ast.Expr) {
	switch a := arg.(type) {
	case *ast.Ident:
		b.Edges = append(b.Edges, edge{Parent: parent, Child: a.Name})
	case *ast.CallExpr:
		childName := fmt.Sprintf("%s#anon%d", parent, len(b.Edges))
		if childDef, childEdges := b.resolveHelperCall(a, childName); childDef != nil {
			childDef.VarName = childName
			b.Edges = append(b.Edges, edge{Parent: parent, AnonChild: childDef})
			b.Edges = append(b.Edges, childEdges...)
		}
	}
}

func stringLitValue(e ast.Expr) (string, bool) {
	bl, ok := e.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(bl.Value)
	if err != nil {
		return "", false
	}
	return v, true
}

// autoFlags returns the flag set for a command node: whatever
// flagsByVar registered explicitly on it, plus cobra's own
// auto-registered flags — O2-6: --help/-h on EVERY command (cobra's
// InitDefaultHelpFlag), and --version additionally when the command's
// own literal sets a Version field (root.go:57 sets Version on
// rootCmd). Without this, resolve() reported "flag --help not
// registered" on the most ordinary true invocation a doc can contain
// (`mindspec --help`, `mindspec doctor --help`) — a guaranteed false
// positive that pressures the exception table to grow for no reason.
func autoFlags(flagsByVar map[string]bool, hasVersion bool) map[string]bool {
	out := map[string]bool{"help": true, "h": true}
	for k := range flagsByVar {
		out[k] = true
	}
	if hasVersion {
		out["version"] = true
	}
	return out
}

// linkCmdTree assembles the collected defs/edges into a *cmdNode tree
// rooted at "rootCmd".
func linkCmdTree(b *cmdTreeBuild) (*cmdNode, error) {
	nodes := map[string]*cmdNode{}
	for name, def := range b.Defs {
		nodes[name] = &cmdNode{
			Use:     def.Use,
			Name:    firstField(def.Use),
			IsStub:  def.isStub(),
			NoArgs:  def.NoArgs,
			Flags:   autoFlags(def.flagsByVar, def.HasVersion),
			Persist: def.persistByVar,
		}
	}
	// Materialize anonymous children discovered as edge.AnonChild.
	anonKey := func(e edge) string {
		if e.Child != "" {
			return e.Child
		}
		return e.AnonChild.VarName
	}
	for _, e := range b.Edges {
		if e.AnonChild != nil {
			nodes[e.AnonChild.VarName] = &cmdNode{
				Use:     e.AnonChild.Use,
				Name:    firstField(e.AnonChild.Use),
				IsStub:  e.AnonChild.isStub(),
				NoArgs:  e.AnonChild.NoArgs,
				Flags:   autoFlags(e.AnonChild.flagsByVar, e.AnonChild.HasVersion),
				Persist: e.AnonChild.persistByVar,
			}
		}
	}
	for _, e := range b.Edges {
		parent, ok := nodes[e.Parent]
		if !ok {
			continue
		}
		child, ok := nodes[anonKey(e)]
		if !ok {
			continue
		}
		if child.Parent != nil {
			continue // already linked (defensive; shouldn't happen with well-formed AddCommand usage)
		}
		child.Parent = parent
		parent.Children = append(parent.Children, child)
	}
	root, ok := nodes["rootCmd"]
	if !ok {
		return nil, fmt.Errorf("no rootCmd construction found")
	}
	if root.Name != "mindspec" {
		return nil, fmt.Errorf("rootCmd.Use %q does not start with \"mindspec\"", root.Use)
	}
	return root, nil
}

func firstField(use string) string {
	fields := strings.Fields(use)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// --- helpers shared with docs_truth_test.go --------------------------

// realCmdDir resolves the repo's cmd/mindspec directory relative to
// this test file, independent of the test binary's working directory.
func realCmdDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "cmd", "mindspec")
}

// dumpTree is a debugging aid (unused by any assertion; kept for
// developer sanity-checking with `go test -run TestDumpCmdTree -v`).
func dumpTree(n *cmdNode, depth int) []string {
	var out []string
	stub := ""
	if n.IsStub {
		stub = " [STUB]"
	}
	out = append(out, strings.Repeat("  ", depth)+n.path()+stub)
	children := append([]*cmdNode{}, n.Children...)
	sort.Slice(children, func(i, j int) bool { return children[i].Name < children[j].Name })
	for _, c := range children {
		out = append(out, dumpTree(c, depth+1)...)
	}
	return out
}

func TestDumpCmdTree(t *testing.T) {
	root, err := buildCmdTree(realCmdDir(t))
	if err != nil {
		t.Fatalf("buildCmdTree: %v", err)
	}
	if testing.Verbose() {
		for _, line := range dumpTree(root, 0) {
			t.Log(line)
		}
	}
}
