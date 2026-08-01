package executor

// Spec 127 AC-7(iv): the chokepoint anti-drift test. Every
// gitutil.MergeInto/gitutil.MergeBranch call site in this package must be
// preceded — in the same enclosing function-like construct (a *ast.FuncDecl
// or a *ast.FuncLit, whichever DIRECTLY contains it — never inherited from
// an enclosing scope), by its OWN, dedicated, operand-corresponding call to
// preflightMergeDestruction. A new raw producer that skips the preflight
// call entirely is caught here, at build-and-test time, regardless of
// whether any behavioral test happens to exercise it.
//
// Bead-6 fix round 1 rewrite (O2-1, S3-1, S3-2/G1-2 — three independently
// demonstrated ways the original scan was defeated):
//
//   - O2-1: the original scan only visited *ast.FuncDecl nodes reached via
//     file.Decls, so a package-level `var sneakyMerge = func(...) error {
//     return gitutil.MergeInto(...) }` was never visited AT ALL — not
//     merely unattributed, invisible. This rewrite treats every *ast.FuncLit
//     ANYWHERE in the file (at any nesting depth: bound to a package-level
//     var, a struct field, a closure inside a FuncDecl, a goroutine
//     argument) as its OWN span with its OWN preflight obligation, found by
//     a whole-file ast.Inspect that dispatches a per-span sub-scan at every
//     *ast.FuncDecl and *ast.FuncLit it encounters.
//   - S3-1: the original scan matched only a literal
//     `gitutil.MergeInto(...)`/`gitutil.MergeBranch(...)` *ast.SelectorExpr,
//     so `var adversarialMergeFn = gitutil.MergeInto` followed by
//     `adversarialMergeFn(...)` was invisible. This rewrite collects a
//     package-level alias set FIRST (any `var X = gitutil.MergeInto` /
//     `= gitutil.MergeBranch` assignment, across every file) and treats a
//     bare `X(...)` call as the aliased kind wherever X resolves.
//   - S3-2/G1-2: the original scan required only that SOME preflight call
//     precede ALL merge calls in the function — one preflight call could
//     "certify" an unbounded number of merge calls over DIFFERENT operand
//     pairs, and a nested closure's merge call was certified by an OUTER
//     function's preflight it can execute independently of. This rewrite
//     requires each merge call to consume its OWN, DISTINCT, nearest-
//     preceding preflight call within the SAME span whose operands
//     CORRESPOND (the preflight's branch argument must equal the merge
//     call's source-branch argument, by identifier/selector/literal text;
//     for gitutil.MergeBranch, which also carries an explicit target
//     branch argument, the preflight's target argument must correspond
//     too) — a preflight call already consumed by an earlier merge call in
//     the same span can never certify a second one.
//
// STATED RESIDUAL LIMITATIONS (this is still an AST-level presence-and-
// correspondence check, not a full dataflow analysis — G1's own ruling on
// this bead's panel: "appropriate as an intended-inventory review ratchet,
// not a substitute for operand dataflow" applies to what follows too):
//
//   - gitutil.MergeInto's signature is (targetWorkdir, sourceBranch) — the
//     target is a WORKDIR PATH, not a branch name, so it has no textual
//     form comparable to preflightMergeDestruction's target argument (a
//     branch name). Only the SOURCE operand is correspondence-checked for
//     MergeInto call sites; the target cannot be, structurally. This is a
//     stated, review-caught residual, not a silent gap.
//   - Operand correspondence is TEXTUAL (identifier name, `X.Sel` selector
//     form, or literal value) — not type-checked, not resolved through
//     assignment chains. Two DIFFERENT variables that happen to hold the
//     same runtime value at different textual names are not recognized as
//     corresponding; conversely this cannot be fooled by mere accidental
//     name reuse of UNRELATED values, since a real producer's own operand
//     naming is what this test's fixtures below assert against.
//   - (CLOSED, bead-6 fix round 3, G1-2's confirm-round finding) A
//     var-to-var alias CHAIN of any depth (`var A = gitutil.MergeInto; var
//     B = A; var C = B`) is now fully resolved by collectMergeFnAliases'
//     fixed-point closure — not merely the first hop. A function-typed
//     PARAMETER whose signature matches gitutil.MergeInto/MergeBranch
//     exactly, called by its parameter name, is a structurally DIFFERENT
//     shape this scan cannot trace to a concrete value without full
//     call-graph/dataflow analysis — it FAILS CLOSED instead
//     (failClosedOnMergeSignatureParams): finding one REDS the test,
//     naming the location, rather than silently leaving it uncounted.
//   - (CLOSED, bead-6 fix round 4, item 2) A package-level `var` declared
//     WITHOUT an initializer whose OWN declared type matches
//     gitutil.MergeInto/MergeBranch's signature exactly (`var mysteryFn
//     func(string, string) error`, assigned a concrete value only
//     elsewhere — a separate statement, an init() func, another file) was
//     a THIRD unresolvable shape, invisible to both
//     collectMergeFnAliases (no initializer expression to trace) and
//     failClosedOnMergeSignatureParams (not a function parameter). This is
//     exactly as unresolvable as the parameter case, for the same reason,
//     and now FAILS CLOSED the same way
//     (collectUnresolvedMergeSignatureVars +
//     failClosedOnUnresolvedMergeSignatureVars).
//   - This is lexically-scoped, not a full call-graph trace: a merge call
//     reached through a call to a SEPARATE, independently-declared
//     function (not a closure, not an alias) must carry its OWN preflight
//     call within ITS OWN body — this was already true of the original
//     scan (every *ast.FuncDecl is independently required to hold its own
//     preflight call) and remains true here; this rewrite does not
//     introduce or remove that property.
//   - (CLOSED, bead-6 fix round 5, G1's confirm-round finding) THREE more
//     unresolvable shapes, all bounded extensions of the var/parameter
//     fail-closed machinery above rather than new mechanisms:
//     (i) a package-level var declared with a TYPE ALIAS or a distinct
//     NAMED TYPE (`type MergeSignature = func(string, string) error` or
//     `type MergeSignature func(string, string) error`) whose OWN
//     underlying type matches the signature exactly — the var's declared
//     Type is an *ast.Ident naming the alias/named type, never literally
//     an *ast.FuncType, so it was invisible to
//     collectUnresolvedMergeSignatureVars's literal type-assertion;
//     resolveFuncType now follows that one indirection
//     (collectFuncTypeAliases resolves every top-level `type` declaration
//     whose Type is itself a *ast.FuncType, alias or not, once, and every
//     var/parameter/struct-field/method-receiver check below resolves
//     through it); (ii) a STRUCT FIELD of this exact signature, invoked
//     via a selector (`h.run(...)`); (iii) a NAMED FUNCTION TYPE
//     converted from gitutil.MergeInto/MergeBranch and invoked through a
//     METHOD whose OWN receiver carries that exact underlying signature
//     (`m.Invoke(...)`, where m's type's underlying type matches). (ii)
//     and (iii) are the SAME unresolvable shape from this scan's own
//     vantage point — a selector call whose receiver's identity this scan
//     cannot trace — so collectRiskyMemberNames collects both field names
//     and such method names into one vocabulary, and
//     failClosedOnRiskyMemberCalls REDS on either, by the same
//     "cannot resolve, fail closed" standing instruction as the
//     parameter/uninitialized-var legs above.
//
// A NARROW SET OF INLINE CALL-ARGUMENT CLOSURES ARE TRANSPARENT, ON
// PURPOSE — NAMED TO ONE CALL SHAPE, NOT ANY DIRECT CALL ARGUMENT (bead-6
// fix round 2, G1's confirm-round finding: an EARLIER version of this
// scan treated ANY *ast.FuncLit that is a direct element of ANY
// *ast.CallExpr's Args list as transparent — a claim that is FALSE for a
// closure passed to some OTHER function with no synchronous-execution
// guarantee at all; see
// TestAdversaryArbitraryInlineCallbackIsNotTransparent below, which
// fails red against that broader rule). This package's own real
// producers pass their merge attempt as an INLINE func literal directly
// into resumeAwareMerge's mergeFn parameter —
// `resumeAwareMerge(..., func() error { return gitutil.MergeInto(...) },
// ...)` — evaluated SYNCHRONOUSLY as part of that very call, not stored,
// not deferred, not handed to a goroutine. G1-2's own finding is about a
// closure that "can execute independently" of the outer preflight's
// timing; an inline argument to resumeAwareMerge specifically cannot —
// it runs inside the same invocation, immediately after the preflight
// call the enclosing function made moments before. That guarantee comes
// from resumeAwareMerge's OWN audited implementation, not from the mere
// syntactic shape "argument of a call" — an inline closure passed to
// some ARBITRARY other function carries no such guarantee: the callee
// could store it, hand it to a goroutine, or invoke it conditionally,
// and nothing in the AST can tell those cases apart from resumeAwareMerge's
// synchronous contract. So ONLY a *ast.FuncLit that is a DIRECT argument
// of a call to resumeAwareMerge (identifier name match — this package's
// own free function, never a method) is treated as TRANSPARENT: its
// calls are attributed to its ENCLOSING span, not a new one. A
// *ast.FuncLit bound to a `var` (O2-1's shape) OR passed as a direct
// argument to any OTHER call still gets its own, independent span and
// its own obligation.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
)

// mergeChokepointRepoRoot resolves the repo root from this test file's own
// location (mirrors internal/guard/registries_test.go's
// repoRootFromGuardTestDir pattern) so the scan reads THIS worktree's real
// source, never an assumption about the caller's cwd.
func mergeChokepointRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Join(wd, "..", "..")
}

// mergeCallInfo is one gitutil.MergeInto/MergeBranch call site (or an
// alias-resolved equivalent) found inside a single span.
type mergeCallInfo struct {
	pos       token.Pos
	kind      string // "MergeInto" or "MergeBranch"
	sourceKey string // the source-branch operand, textually
	targetKey string // the target-branch operand, textually — "" for
	// MergeInto (structurally uncheckable; see the stated residual above)
}

// preflightCallInfo is one preflightMergeDestruction(branch, target, ...)
// call site found inside a single span.
type preflightCallInfo struct {
	pos       token.Pos
	branchKey string
	targetKey string
}

// funcSpan is one *ast.FuncDecl or *ast.FuncLit's OWN merge/preflight
// calls — never a nested FuncLit's (those get their own span; see
// collectSpanCalls).
type funcSpan struct {
	file       string
	label      string
	mergeCalls []mergeCallInfo
	preflight  []preflightCallInfo
}

// operandKey renders an operand expression to a comparable text form:
// an identifier's name, a selector's dotted form (e.g. "e.Branch"), or a
// literal's raw value (e.g. `"main"`, quotes included so a literal never
// collides with an identically-named identifier). Any other expression
// shape renders to a form that can never equal another operand's key
// (fail-closed: an unresolvable operand can never be treated as a match).
func operandKey(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return operandKey(v.X) + "." + v.Sel.Name
	case *ast.BasicLit:
		return v.Value
	case *ast.ParenExpr:
		return operandKey(v.X)
	default:
		return fmt.Sprintf("<unresolved:%T@%v>", e, e.Pos())
	}
}

// resumeAwareMergeFuncName is the ONE call shape whose direct
// call-argument func literals are synchronous-by-construction (see this
// file's package doc comment). Bead-6 fix round 2 narrows
// collectInlineArgFuncLits to exactly this identifier — a closure passed
// as a direct argument to any OTHER call gets its own span, same as one
// bound to a var.
const resumeAwareMergeFuncName = "resumeAwareMerge"

// collectInlineArgFuncLits finds every *ast.FuncLit that is a DIRECT
// element of a call to resumeAwareMergeFuncName's Args list anywhere in
// file — a closure passed inline to THAT specific, audited-synchronous
// call, never stored or deferred. These are TRANSPARENT to the
// span-collection walk below (see this file's package doc comment, "A
// NARROW SET OF INLINE CALL-ARGUMENT CLOSURES ARE TRANSPARENT"): their
// calls belong to their ENCLOSING span, not a new one of their own. A
// *ast.FuncLit passed as a direct argument to any OTHER call is NOT
// collected here (bead-6 fix round 2, G1's confirm-round finding: the
// prior version collected one for ANY call, which is unsound — nothing
// in the AST can attest that an arbitrary callee invokes its argument
// synchronously the way resumeAwareMerge is itself audited to).
func collectInlineArgFuncLits(file *ast.File) map[*ast.FuncLit]bool {
	inline := map[*ast.FuncLit]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		id, ok := call.Fun.(*ast.Ident)
		if !ok || id.Name != resumeAwareMergeFuncName {
			return true
		}
		for _, arg := range call.Args {
			if fl, ok := arg.(*ast.FuncLit); ok {
				inline[fl] = true
			}
		}
		return true
	})
	return inline
}

// gitutilImportPath is this repository's own import path for internal/
// gitutil — the ONE package whose MergeInto/MergeBranch this whole ratchet
// exists to gate.
const gitutilImportPath = "github.com/mrmaxsteel/mindspec/internal/gitutil"

// gitutilLocalName resolves the identifier file's OWN import line binds
// gitutilImportPath to — "gitutil" for a plain, unaliased import (every
// production call site in this package today), or whatever alias an
// import line gives it (`import gu ".../internal/gitutil"`). Bead-6 fix
// round 6 (G1-2's confirm-round finding, shape 3): every direct-call/
// alias/conversion check below previously hard-coded the literal
// identifier "gitutil", so a merge producer reached through
// `gu.MergeInto(...)` with gitutil imported under an alias — ordinary Go,
// not an adversarial contrivance — was invisible to every one of them at
// once. Falls back to the literal "gitutil" when file carries no matching
// import at all: this ratchet's OWN ad hoc, single-file unit-test fixture
// sources (below) are minimal syntax snippets that reference
// `gitutil.MergeInto` directly without ever declaring an import block
// (parser.ParseFile only parses syntax, never resolves imports) — every
// REAL production file in this package does import gitutil under some
// name, so this fallback never masks a real alias there.
func gitutilLocalName(file *ast.File) string {
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != gitutilImportPath {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name
		}
		return "gitutil"
	}
	return "gitutil"
}

// collectSpanCalls walks root (a *ast.FuncDecl's or *ast.FuncLit's Body)
// collecting merge/preflight calls that belong DIRECTLY to it — descent
// stops at any NESTED, NON-INLINE *ast.FuncLit boundary, so a closure
// bound to a var/field/return (never a direct call argument) never has
// its calls attributed to its enclosing span (it gets its own span from
// the whole-file walk that dispatches this function). A nested FuncLit
// that IS a direct call argument (inlineArgs[fl]) is transparent: descent
// continues into it as if it were part of the current span. gitutilName
// is the enclosing file's OWN resolved gitutil identifier
// (gitutilLocalName) — never the literal "gitutil" (bead-6 fix round 6).
func collectSpanCalls(root ast.Node, aliases map[string]string, inlineArgs map[*ast.FuncLit]bool, gitutilName string) (merges []mergeCallInfo, preflights []preflightCallInfo) {
	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		if n != root {
			if fl, ok := n.(*ast.FuncLit); ok && !inlineArgs[fl] {
				return false
			}
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := call.Fun.(type) {
		case *ast.SelectorExpr:
			if xid, ok := fn.X.(*ast.Ident); ok && xid.Name == gitutilName &&
				(fn.Sel.Name == "MergeInto" || fn.Sel.Name == "MergeBranch") {
				merges = append(merges, buildMergeCall(call, fn.Sel.Name))
			}
			if fn.Sel.Name == "preflightMergeDestruction" {
				preflights = append(preflights, buildPreflightCall(call))
			}
		case *ast.Ident:
			if kind, ok := aliases[fn.Name]; ok {
				merges = append(merges, buildMergeCall(call, kind))
			}
		}
		return true
	})
	return merges, preflights
}

func buildMergeCall(call *ast.CallExpr, kind string) mergeCallInfo {
	mc := mergeCallInfo{pos: call.Pos(), kind: kind}
	switch kind {
	case "MergeInto":
		// gitutil.MergeInto(targetWorkdir, sourceBranch string) — Args[1]
		// is the source branch; Args[0] is a workdir PATH, structurally
		// uncomparable to a preflight's branch-name target argument (see
		// the stated residual in this file's package doc comment).
		if len(call.Args) >= 2 {
			mc.sourceKey = operandKey(call.Args[1])
		}
	case "MergeBranch":
		// gitutil.MergeBranch(workdir, source, target string) — both
		// branch-name operands are checkable.
		if len(call.Args) >= 3 {
			mc.sourceKey = operandKey(call.Args[1])
			mc.targetKey = operandKey(call.Args[2])
		}
	}
	return mc
}

func buildPreflightCall(call *ast.CallExpr) preflightCallInfo {
	pc := preflightCallInfo{pos: call.Pos()}
	if len(call.Args) >= 2 {
		pc.branchKey = operandKey(call.Args[0])
		pc.targetKey = operandKey(call.Args[1])
	}
	return pc
}

// collectMergeFnAliases scans every top-level `var` declaration across
// every parsed file for an alias of gitutil.MergeInto/gitutil.MergeBranch
// (spec 127 bead-6 fix round 1, S3-1): `var adversarialMergeFn =
// gitutil.MergeInto`. Bead-6 fix round 3 (G1-2's confirm-round finding):
// this now resolves a CHAIN of var-to-var aliases of ANY depth — `var A =
// gitutil.MergeInto; var B = A; var C = B` — not merely the first hop,
// via the fixed-point closure below. A `var Y = X` whose chain bottoms
// out at something OTHER than gitutil.MergeInto/MergeBranch (an alias of
// an unrelated function, or a chain this scan cannot resolve at all — a
// call expression, a struct-field selector, etc.) is correctly left
// unresolved: it is not a merge producer, and flagging it would be pure
// false-positive noise. A function-PARAMETER indirection (as opposed to
// a package-level var) is a structurally different shape — see
// funcParamsMatchingMergeSignature below, which fails closed on it
// instead of attempting to resolve it.
func collectMergeFnAliases(files []*ast.File) map[string]string {
	aliases := map[string]string{}
	pending := map[string]string{} // varName -> the identifier it was assigned, for var-to-var hops not yet resolved
	for _, file := range files {
		// Bead-6 fix round 6: resolve THIS file's own gitutil identifier —
		// never the literal "gitutil" (see gitutilLocalName's doc comment).
		gitutilName := gitutilLocalName(file)
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, val := range vs.Values {
					if i >= len(vs.Names) {
						continue
					}
					name := vs.Names[i].Name
					switch v := val.(type) {
					case *ast.SelectorExpr:
						xid, ok := v.X.(*ast.Ident)
						if !ok || xid.Name != gitutilName {
							continue
						}
						if v.Sel.Name != "MergeInto" && v.Sel.Name != "MergeBranch" {
							continue
						}
						aliases[name] = v.Sel.Name
					case *ast.Ident:
						// A var-to-var hop (`var Y = X`) — may chain to
						// gitutil.MergeInto/MergeBranch through any number
						// of further hops; resolved below.
						pending[name] = v.Name
					}
				}
			}
		}
	}
	// Fixed-point closure over pending var-to-var aliases: repeat until a
	// pass resolves nothing new, so a chain of ANY length is fully
	// traced, not just the first hop (bead-6 fix round 3, G1-2).
	for changed := true; changed; {
		changed = false
		for name, target := range pending {
			if _, already := aliases[name]; already {
				continue
			}
			if kind, ok := aliases[target]; ok {
				aliases[name] = kind
				changed = true
			}
		}
	}
	return aliases
}

// mergeFuncSignatureShape reports whether ft matches gitutil.MergeInto's
// or gitutil.MergeBranch's OWN signature exactly — respectively (string,
// string) error and (string, string, string) error, every param
// unnamed/string-typed, one error result. This is the ONE shape a
// function-typed PARAMETER could hold either of those two functions
// DIRECTLY (passed through, not wrapped) — resumeAwareMerge's own
// mergeFn (`func() error`) and conflictFailure (`func(error) error`)
// parameters never match this shape, by construction, since they WRAP a
// merge attempt rather than being called AS one.
func mergeFuncSignatureShape(ft *ast.FuncType) (kind string, ok bool) {
	if ft.Results == nil || len(ft.Results.List) != 1 {
		return "", false
	}
	resIdent, ok2 := ft.Results.List[0].Type.(*ast.Ident)
	if !ok2 || resIdent.Name != "error" {
		return "", false
	}
	if ft.Params == nil {
		return "", false
	}
	nParams := 0
	for _, f := range ft.Params.List {
		id, ok3 := f.Type.(*ast.Ident)
		if !ok3 || id.Name != "string" {
			return "", false
		}
		n := len(f.Names)
		if n == 0 {
			n = 1
		}
		nParams += n
	}
	switch nParams {
	case 2:
		return "MergeInto", true
	case 3:
		return "MergeBranch", true
	default:
		return "", false
	}
}

// collectFuncTypeAliases scans every top-level `type` declaration across
// every file for one whose OWN Type is directly an *ast.FuncType — this
// captures BOTH a type alias (`type MergeSignature = func(string, string)
// error`, TypeSpec.Assign set) and a distinct NAMED type (`type
// MergeSignature func(string, string) error`, TypeSpec.Assign unset):
// bead-6 fix round 5 (G1's confirm-round finding, shape (a)) needs both,
// since a var/parameter/struct-field/method-receiver can be declared with
// either spelling and neither is literally an *ast.FuncType at the
// declaration site that names it — only resolveFuncType's one-hop lookup
// through this map makes it visible to mergeFuncSignatureShape at all.
func collectFuncTypeAliases(files []*ast.File) map[string]*ast.FuncType {
	aliases := map[string]*ast.FuncType{}
	for _, file := range files {
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if ft, ok := ts.Type.(*ast.FuncType); ok {
					aliases[ts.Name.Name] = ft
				}
			}
		}
	}
	return aliases
}

// resolveFuncType resolves e to its own *ast.FuncType: directly, if e IS
// one, or through exactly ONE typeAliases lookup if e is a bare
// *ast.Ident naming a type alias or named type collectFuncTypeAliases
// found (bead-6 fix round 5). Nil typeAliases is valid (an *ast.Ident
// then never resolves) — every caller below still fails closed on
// whatever it cannot trace, precisely as it did before this indirection
// existed. Anything else (a selector, a pointer, an unresolvable name)
// returns ok=false: this is a bounded, ONE-HOP extension of the existing
// var/parameter fail-closed machinery, never an attempt at general type
// resolution.
func resolveFuncType(e ast.Expr, typeAliases map[string]*ast.FuncType) (*ast.FuncType, bool) {
	switch v := e.(type) {
	case *ast.FuncType:
		return v, true
	case *ast.Ident:
		ft, ok := typeAliases[v.Name]
		return ft, ok
	default:
		return nil, false
	}
}

// funcParamsMatchingMergeSignature returns the names of every parameter
// in params whose type EXACTLY matches gitutil.MergeInto's or
// gitutil.MergeBranch's own signature (mergeFuncSignatureShape,
// resolveFuncType) — the shape a caller could pass either of those two
// functions through DIRECTLY, unwrapped, as a value, spelled as a literal
// func type OR (bead-6 fix round 5) a type alias/named type resolving to
// one.
func funcParamsMatchingMergeSignature(params *ast.FieldList, typeAliases map[string]*ast.FuncType) []string {
	if params == nil {
		return nil
	}
	var names []string
	for _, f := range params.List {
		ft, ok := resolveFuncType(f.Type, typeAliases)
		if !ok {
			continue
		}
		if _, ok := mergeFuncSignatureShape(ft); !ok {
			continue
		}
		names = append(names, namesOrBlank(f.Names)...)
	}
	return names
}

// isMergeProducerExpr reports whether e is (or, through any number of
// type-conversion/paren layers, ultimately reduces to) a direct reference
// to gitutil.MergeInto/MergeBranch: the literal selector itself, a
// package-level var mergeAliases (collectMergeFnAliases' own output)
// already resolved to one, or a type-conversion call (`SomeType(X)`,
// syntactically a *ast.CallExpr with exactly one argument) whose
// argument itself satisfies this predicate. Used to find where a merge
// producer is ACTUALLY assigned into a struct field or converted to a
// named type — see collectRiskyMemberNames' own doc comment for why this
// narrower "genuinely assigned" bar, not shape alone, is what keeps that
// scan from false-positiving on every unrelated same-shaped field this
// package's own mock/fixture types declare (e.g. MockExecutor's
// GitMvFn/ResetHardFn, which share gitutil.MergeInto/MergeBranch's bare
// string-arity shape by coincidence, never assigned a merge producer).
func isMergeProducerExpr(e ast.Expr, mergeAliases map[string]string, gitutilName string) bool {
	switch v := e.(type) {
	case *ast.SelectorExpr:
		xid, ok := v.X.(*ast.Ident)
		return ok && xid.Name == gitutilName && (v.Sel.Name == "MergeInto" || v.Sel.Name == "MergeBranch")
	case *ast.Ident:
		_, ok := mergeAliases[v.Name]
		return ok
	case *ast.CallExpr:
		if len(v.Args) != 1 {
			return false
		}
		return isMergeProducerExpr(v.Args[0], mergeAliases, gitutilName)
	case *ast.ParenExpr:
		return isMergeProducerExpr(v.X, mergeAliases, gitutilName)
	default:
		return false
	}
}

// collectNamedStructTypes scans every top-level `type X struct {...}`
// declaration across every file, mirroring collectFuncTypeAliases for
// struct types — needed to resolve a POSITIONAL composite literal's
// (`handler{gitutil.MergeInto}`) field name by index, since a positional
// element carries no field name of its own (bead-6 fix round 6, G1-2's
// confirm-round finding, shape 2).
func collectNamedStructTypes(files []*ast.File) map[string]*ast.StructType {
	structs := map[string]*ast.StructType{}
	for _, file := range files {
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if st, ok := ts.Type.(*ast.StructType); ok {
					structs[ts.Name.Name] = st
				}
			}
		}
	}
	return structs
}

// structFieldNamesInOrder flattens st's own field list into the ordered
// name sequence a POSITIONAL composite literal's elements line up
// against — one entry per name a field declares (`a, b int` contributes
// two slots), or "" for an embedded field (no Names), so position
// counting stays correct even though an embedded field can never match a
// risky member name added elsewhere by field NAME.
func structFieldNamesInOrder(st *ast.StructType) []string {
	if st == nil || st.Fields == nil {
		return nil
	}
	var names []string
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			names = append(names, "")
			continue
		}
		for _, n := range f.Names {
			names = append(names, n.Name)
		}
	}
	return names
}

// resolveStructType resolves e to its own *ast.StructType: directly, if e
// IS one (an anonymous struct type literal, right there in the
// composite-literal expression), or through exactly ONE namedStructs
// lookup if e is a bare *ast.Ident naming a declared struct type —
// mirroring resolveFuncType's bounded, one-hop philosophy. Anything else
// (nil Type, elided in a nested literal; a pointer; an unresolvable name)
// returns ok=false.
func resolveStructType(e ast.Expr, namedStructs map[string]*ast.StructType) (*ast.StructType, bool) {
	switch v := e.(type) {
	case *ast.StructType:
		return v, true
	case *ast.Ident:
		st, ok := namedStructs[v.Name]
		return st, ok
	default:
		return nil, false
	}
}

// collectRiskyMemberNames scans every file for a struct field or a named
// function type GENUINELY assigned/converted from gitutil.MergeInto/
// MergeBranch (isMergeProducerExpr) — bead-6 fix round 5 (G1's
// confirm-round finding, shapes (b) and (c)), extended by bead-6 fix
// round 6 (G1-2's confirm-round finding) to two further ways a field can
// be GENUINELY assigned:
//
//   - a STRUCT FIELD assigned a merge producer via a composite-literal
//     KEYED element (`handler{run: gitutil.MergeInto}`, round 5) or a
//     POSITIONAL element (`handler{gitutil.MergeInto}`, round 6 — the
//     element carries no field name of its own, so its slot's name is
//     resolved by index against the literal's own struct type), reached
//     elsewhere via a selector call (`h.run(...)`);
//   - a STRUCT FIELD assigned a merge producer via a POST-CONSTRUCTION
//     assignment statement (`h.run = gitutil.MergeInto`, round 6 —
//     ordinary Go, not merely a contrived construction-time shape);
//   - a METHOD whose OWN receiver's named function type was itself
//     converted from a merge producer somewhere (`type mergeFunc
//     func(string, string) error; ...; m := mergeFunc(gitutil.MergeInto)`,
//     `func (f mergeFunc) Invoke(a, b string) error { return f(a, b) }`),
//     reached via a selector call (`m.Invoke(...)`).
//
// This is deliberately NARROWER than "any field/receiver whose declared
// type happens to match the bare signature shape" — (string, string)
// error and (string, string, string) error are common, unremarkable
// shapes (this very package's own MockExecutor fixture fields,
// GitMvFn/ResetHardFn, share them purely by coincidence, assigned real
// test closures that have nothing to do with gitutil.MergeInto/
// MergeBranch) — flagging every same-shaped field/method would be pure
// false-positive noise, not a finding. Requiring a genuine assignment/
// conversion FROM a merge producer, found ANYWHERE in the scanned files
// (the same textual, not-full-dataflow philosophy collectMergeFnAliases
// already uses for package vars), is what makes this a real signal: it
// fires only when a merge producer's own identity has genuinely been
// funneled through this member, which is exactly G1's own adversarial
// shape. From there this is the SAME unresolvable indirection a function-
// typed parameter or an uninitialized package-level var already fails
// closed on: a selector call whose receiver's concrete value this scan
// cannot trace without full call-graph/dataflow analysis. Field names and
// matching method names share ONE vocabulary here, so
// failClosedOnRiskyMemberCalls treats them identically.
//
// STATED RESIDUAL (bead-6 fix round 6, G1-2's confirm-round finding, the
// false-positive half — judged NONBLOCKING and left as a disclosed
// trade-off, not fixed): this vocabulary is keyed by BARE member name
// alone, package-wide, never by the receiver's own type identity —
// resolving that would require go/types, full type-checking this
// deliberately AST-level, textual ratchet does not otherwise attempt (see
// this file's own package doc comment). So a field or method GENUINELY
// assigned a merge producer on one type can make an UNRELATED type's
// same-named, different-signature member call red purely on the name
// collision. This is a real, demonstrated false-positive (a maintenance
// cost — rename or extend this scan) but never a false NEGATIVE: it can
// only make this ratchet fail CI on safe code, never let an unpreflighted
// merge call through silently, so it does not compromise the guarantee
// this bead exists to provide.
func collectRiskyMemberNames(files []*ast.File, typeAliases map[string]*ast.FuncType, mergeAliases map[string]string) map[string]bool {
	risky := map[string]bool{}
	namedStructs := collectNamedStructTypes(files)

	// Named types genuinely converted FROM a merge producer somewhere —
	// the narrower bar shape (c) needs, before any method on that type is
	// treated as risky.
	convertedTypes := map[string]bool{}
	for _, file := range files {
		gitutilName := gitutilLocalName(file)
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := call.Fun.(*ast.Ident)
			if !ok || len(call.Args) != 1 {
				return true
			}
			if _, ok := typeAliases[id.Name]; !ok {
				return true
			}
			if isMergeProducerExpr(call.Args[0], mergeAliases, gitutilName) {
				convertedTypes[id.Name] = true
			}
			return true
		})
	}

	for _, file := range files {
		gitutilName := gitutilLocalName(file)

		// Struct fields genuinely assigned a merge producer via a
		// composite-literal element — KEYED or POSITIONAL. A struct
		// literal's elements are, per Go's own grammar, either ALL keyed
		// or ALL positional — never mixed — so finding one non-KeyValueExpr
		// element means every element in this literal is positional, and
		// each slot's field name is resolved by index against the
		// literal's own struct type (round 6).
		ast.Inspect(file, func(n ast.Node) bool {
			cl, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			var positionalNames []string
			for _, elt := range cl.Elts {
				if _, keyed := elt.(*ast.KeyValueExpr); !keyed {
					if st, ok := resolveStructType(cl.Type, namedStructs); ok {
						positionalNames = structFieldNamesInOrder(st)
					}
					break
				}
			}
			for i, elt := range cl.Elts {
				if kv, ok := elt.(*ast.KeyValueExpr); ok {
					keyID, ok := kv.Key.(*ast.Ident)
					if !ok {
						continue
					}
					if isMergeProducerExpr(kv.Value, mergeAliases, gitutilName) {
						risky[keyID.Name] = true
					}
					continue
				}
				if i >= len(positionalNames) || positionalNames[i] == "" {
					continue
				}
				if isMergeProducerExpr(elt, mergeAliases, gitutilName) {
					risky[positionalNames[i]] = true
				}
			}
			return true
		})

		// Struct fields genuinely assigned a merge producer via a
		// POST-CONSTRUCTION assignment (`h.run = gitutil.MergeInto`, round
		// 6) — the same "genuinely assigned" bar as the composite-literal
		// leg above, just via *ast.AssignStmt rather than at construction.
		ast.Inspect(file, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok || as.Tok != token.ASSIGN {
				return true
			}
			for i, lhs := range as.Lhs {
				if i >= len(as.Rhs) {
					continue
				}
				sel, ok := lhs.(*ast.SelectorExpr)
				if !ok {
					continue
				}
				if isMergeProducerExpr(as.Rhs[i], mergeAliases, gitutilName) {
					risky[sel.Sel.Name] = true
				}
			}
			return true
		})

		// Methods whose receiver's named type was itself converted from a
		// merge producer above.
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || len(fd.Recv.List) != 1 {
				continue
			}
			recvType := fd.Recv.List[0].Type
			if star, ok := recvType.(*ast.StarExpr); ok {
				recvType = star.X
			}
			id, ok := recvType.(*ast.Ident)
			if !ok || !convertedTypes[id.Name] {
				continue
			}
			risky[fd.Name.Name] = true
		}
	}
	return risky
}

// namesOrBlank returns each *ast.Ident's Name — a parameter field can
// carry zero names (an unnamed/anonymous parameter, which can never be
// called by identifier, so it is correctly excluded by returning
// nothing for it).
func namesOrBlank(idents []*ast.Ident) []string {
	names := make([]string, 0, len(idents))
	for _, id := range idents {
		names = append(names, id.Name)
	}
	return names
}

// paramCalledAsFunc reports whether body calls paramName as a bare
// function call (`paramName(...)`) anywhere within it, at any nesting
// depth (deliberately not stopping at a nested FuncLit boundary — a
// parameter closed over by an inner closure and invoked there is still
// the same unresolvable indirection this check exists to catch).
func paramCalledAsFunc(body ast.Node, paramName string) bool {
	if body == nil {
		return false
	}
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if id, ok := call.Fun.(*ast.Ident); ok && id.Name == paramName {
			found = true
			return false
		}
		return true
	})
	return found
}

// collectUnresolvedMergeSignatureVars scans every top-level `var`
// declaration (across every file) for one declared WITHOUT an
// initializer whose OWN declared type matches gitutil.MergeInto/
// MergeBranch's exact signature (mergeFuncSignatureShape, resolveFuncType)
// — e.g. `var mysteryFn func(string, string) error`, left nil at
// declaration and assigned a concrete value only somewhere else (a
// separate assignment statement, an init() func, another file). Spec 127
// bead-6 fix round 4 (item 2, the confirm-round's own ruling):
// collectMergeFnAliases can only ever trace a var's own INITIALIZER
// expression — a var with NO initializer carries nothing for it to trace,
// so it was previously invisible to BOTH collectMergeFnAliases (not an
// alias assignment) and failClosedOnMergeSignatureParams (not a function
// parameter) — a third, silently uncounted shape. This is exactly as
// unresolvable, by the same reasoning failClosedOnMergeSignatureParams
// already applies to a function-typed parameter: the standing instruction
// is fail CLOSED on a shape this scan cannot resolve, not silently pass it
// over. Bead-6 fix round 5 (G1's confirm-round finding, shape (a)): the
// var's declared type is now resolved through typeAliases too, so `var
// mysteryFn MergeSignature` (a type alias or named type resolving to the
// exact signature) is caught identically to the literal `func(string,
// string) error` spelling — resolveFuncType's one-hop lookup is the ONLY
// difference from round 4's own check. Returns the set of such var names,
// collected once and shared across every span (a package-level var is
// visible package-wide, unlike a function parameter scoped to its own
// function).
func collectUnresolvedMergeSignatureVars(files []*ast.File, typeAliases map[string]*ast.FuncType) map[string]bool {
	unresolved := map[string]bool{}
	for _, file := range files {
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok || len(vs.Values) != 0 || vs.Type == nil {
					// A var WITH an initializer is collectMergeFnAliases'
					// own concern (a chain rooted at gitutil.MergeInto/
					// MergeBranch resolves there; a chain rooted at
					// something else is correctly left unresolved, not a
					// merge producer at all — re-flagging it here on
					// signature shape alone would be pure false-positive
					// noise for the common case of an unrelated
					// same-shaped function value).
					continue
				}
				ft, ok := resolveFuncType(vs.Type, typeAliases)
				if !ok {
					continue
				}
				if _, ok := mergeFuncSignatureShape(ft); !ok {
					continue
				}
				for _, name := range vs.Names {
					unresolved[name.Name] = true
				}
			}
		}
	}
	return unresolved
}

// failClosedOnUnresolvedMergeSignatureVars is
// collectUnresolvedMergeSignatureVars' enforcement leg, mirroring
// failClosedOnMergeSignatureParams below: any span that calls one of these
// package-level, uninitialized-at-declaration vars directly (by name) REDS
// — this scan cannot trace what value the var holds at that call site
// (the assignment could be anywhere: a different file, an init() func, a
// runtime-conditional branch), so it cannot confirm a preflight covers
// whatever it resolves to. Shares paramCalledAsFunc's direct-call
// detection — the check is identical once the vocabulary of "an
// unresolvable name that might be a merge producer" is the same, whether
// that name arrived as a function parameter or a package-level var.
func failClosedOnUnresolvedMergeSignatureVars(t *testing.T, unresolvedVars map[string]bool, body ast.Node, fileName, label string, pos token.Position) {
	for name := range unresolvedVars {
		if paramCalledAsFunc(body, name) {
			t.Errorf("%s (%s, at %s): package-level var %q is declared with a signature matching gitutil.MergeInto/MergeBranch exactly but NO initializer, and is called directly here — this AST-level scan cannot trace what value it is assigned elsewhere (a separate statement, an init() func, another file), so it cannot confirm a preflight covers whatever it resolves to at runtime. Give it a traceable shape (an initializer this scan can resolve, e.g. `= gitutil.MergeInto` or a chain rooted at one) or extend this scan; a merge producer must never reach gitutil.MergeInto/MergeBranch through an indirection this ratchet cannot see.", label, fileName, pos, name)
		}
	}
}

// failClosedOnMergeSignatureParams is bead-6 fix round 3's fail-closed
// leg for the OTHER unresolvable indirection G1-2's confirm-round
// finding demonstrated (alongside the chained-alias shape
// collectMergeFnAliases now fully resolves): a function-typed PARAMETER
// whose signature matches gitutil.MergeInto/MergeBranch exactly, called
// directly by its parameter name inside the span. This scan cannot trace
// WHAT VALUE such a parameter is bound to at any given call site — that
// would require full call-graph/dataflow analysis, which this ratchet
// deliberately does not attempt (G1's own ruling: "fail closed on shapes
// it cannot resolve, not full call-graph"). So rather than silently
// passing over it (the pre-round-3 defect: a param of this exact shape,
// called, was invisible to totalCalls and to the preflight-correspondence
// check alike), it REDS — a human must either give the scan a traceable
// shape (a direct call or a var alias) or extend it, never leave an
// unresolvable indirection matching a merge producer's own signature
// silently uncounted. mergeFn (`func() error`) and conflictFailure
// (`func(error) error`) never match this signature (0 and 1 string-typed
// params respectively, not 2 or 3), so this never fires against
// resumeAwareMerge/attemptFreshMerge/completeDriftedResumedMerge's own,
// already-audited plumbing.
func failClosedOnMergeSignatureParams(t *testing.T, params *ast.FieldList, body ast.Node, fileName, label string, pos token.Position, typeAliases map[string]*ast.FuncType) {
	for _, name := range funcParamsMatchingMergeSignature(params, typeAliases) {
		if paramCalledAsFunc(body, name) {
			t.Errorf("%s (%s, at %s): parameter %q has a signature matching gitutil.MergeInto/MergeBranch exactly and is called directly — this AST-level scan cannot trace what value it is bound to at any call site (not full call-graph analysis, per design), so it cannot confirm a preflight covers whatever it resolves to at runtime. Give it a traceable shape (a direct gitutil.MergeInto/MergeBranch call, or a package-level var alias) or extend this scan; a merge producer must never reach gitutil.MergeInto/MergeBranch through an indirection this ratchet cannot see.", label, fileName, pos, name)
		}
	}
}

// failClosedOnRiskyMemberCalls is collectRiskyMemberNames' enforcement
// leg (bead-6 fix round 5, G1's confirm-round finding, shapes (b) and
// (c)): any selector call `X.Name(...)` inside body, at ANY nesting depth
// (deliberately not stopping at a nested FuncLit boundary — same reason
// paramCalledAsFunc does not), where Name is a struct field or method
// name collectRiskyMemberNames matched, REDS — this scan cannot trace
// what concrete value X.Name resolves to. `gitutil.<anything>` selectors
// are excluded here: those are collectSpanCalls' own, already-precise
// concern (a real gitutil.MergeInto/MergeBranch call, or an unrelated
// gitutil function — never a risky member by construction, since nothing
// in gitutil's own source is part of the files this scan parses).
func failClosedOnRiskyMemberCalls(t *testing.T, risky map[string]bool, body ast.Node, fileName, label string, pos token.Position, gitutilName string) {
	if body == nil || len(risky) == 0 {
		return
	}
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if xid, ok := sel.X.(*ast.Ident); ok && xid.Name == gitutilName {
			return true
		}
		if risky[sel.Sel.Name] {
			t.Errorf("%s (%s, at %s): %q is a struct field or a named-function-type method whose declared/receiver type has a signature matching gitutil.MergeInto/MergeBranch exactly, and is called here via a selector — this AST-level scan cannot trace what concrete function value it holds at this call site (not full call-graph/dataflow analysis, per design), so it cannot confirm a preflight covers whatever it resolves to at runtime. Give it a traceable shape or extend this scan; a merge producer must never reach gitutil.MergeInto/MergeBranch through an indirection this ratchet cannot see.", label, fileName, pos, sel.Sel.Name)
		}
		return true
	})
}

// TestMergeChokepoint_EveryProducerConsultsThePreflight is AC-7(iv):
// every gitutil.MergeInto/gitutil.MergeBranch call site (direct or
// single-hop-aliased) in this package, in every function-like construct
// (FuncDecl or FuncLit, at any nesting depth), is preceded by its OWN,
// distinct, operand-corresponding call to preflightMergeDestruction.
func TestMergeChokepoint_EveryProducerConsultsThePreflight(t *testing.T) {
	root := mergeChokepointRepoRoot(t)
	pkgDir := filepath.Join(root, "internal", "executor")
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		t.Fatalf("reading %s: %v", pkgDir, err)
	}

	fset := token.NewFileSet()
	var files []*ast.File
	var fileNames []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".go" {
			continue
		}
		// Scan every non-test .go file: a producer belongs in production
		// code, and scanning test files too would only ever ADD false
		// findings (a test helper calling MergeInto directly against a
		// real repo, with no preflight, is a fixture-building convenience,
		// not a producer) — excluding _test.go avoids that noise while
		// never narrowing the production-code coverage this AC requires.
		if len(e.Name()) > 8 && e.Name()[len(e.Name())-8:] == "_test.go" {
			continue
		}
		path := filepath.Join(pkgDir, e.Name())
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}
		files = append(files, file)
		fileNames = append(fileNames, e.Name())
	}

	aliases := collectMergeFnAliases(files)
	typeAliases := collectFuncTypeAliases(files)
	unresolvedVars := collectUnresolvedMergeSignatureVars(files, typeAliases)
	riskyMembers := collectRiskyMemberNames(files, typeAliases, aliases)

	var spans []*funcSpan
	for i, file := range files {
		fileName := fileNames[i]
		inlineArgs := collectInlineArgFuncLits(file)
		// Bead-6 fix round 6: resolve THIS file's own gitutil identifier —
		// never the literal "gitutil" (see gitutilLocalName's doc comment).
		gitutilName := gitutilLocalName(file)
		ast.Inspect(file, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.FuncDecl:
				if v.Body == nil {
					return true
				}
				failClosedOnMergeSignatureParams(t, v.Type.Params, v.Body, fileName, v.Name.Name, fset.Position(v.Pos()), typeAliases)
				failClosedOnUnresolvedMergeSignatureVars(t, unresolvedVars, v.Body, fileName, v.Name.Name, fset.Position(v.Pos()))
				failClosedOnRiskyMemberCalls(t, riskyMembers, v.Body, fileName, v.Name.Name, fset.Position(v.Pos()), gitutilName)
				merges, preflights := collectSpanCalls(v.Body, aliases, inlineArgs, gitutilName)
				spans = append(spans, &funcSpan{file: fileName, label: v.Name.Name, mergeCalls: merges, preflight: preflights})
			case *ast.FuncLit:
				if inlineArgs[v] {
					// Transparent — its calls are already attributed to
					// its enclosing span (see collectSpanCalls); it gets
					// no span of its own.
					return true
				}
				label := fmt.Sprintf("func literal at %s", fset.Position(v.Pos()))
				failClosedOnMergeSignatureParams(t, v.Type.Params, v.Body, fileName, label, fset.Position(v.Pos()), typeAliases)
				failClosedOnUnresolvedMergeSignatureVars(t, unresolvedVars, v.Body, fileName, label, fset.Position(v.Pos()))
				failClosedOnRiskyMemberCalls(t, riskyMembers, v.Body, fileName, label, fset.Position(v.Pos()), gitutilName)
				merges, preflights := collectSpanCalls(v.Body, aliases, inlineArgs, gitutilName)
				spans = append(spans, &funcSpan{
					file:       fileName,
					label:      label,
					mergeCalls: merges,
					preflight:  preflights,
				})
			}
			return true
		})
	}

	nonEmpty := 0
	totalCalls := 0
	for _, fs := range spans {
		if len(fs.mergeCalls) == 0 {
			continue
		}
		nonEmpty++
		totalCalls += len(fs.mergeCalls)

		sort.Slice(fs.mergeCalls, func(a, b int) bool { return fs.mergeCalls[a].pos < fs.mergeCalls[b].pos })
		sort.Slice(fs.preflight, func(a, b int) bool { return fs.preflight[a].pos < fs.preflight[b].pos })

		consumed := make([]bool, len(fs.preflight))
		for _, mc := range fs.mergeCalls {
			best := -1
			for i, pc := range fs.preflight {
				if pc.pos >= mc.pos || consumed[i] {
					continue
				}
				if pc.branchKey != mc.sourceKey {
					continue
				}
				if mc.targetKey != "" && pc.targetKey != mc.targetKey {
					continue
				}
				if best == -1 || pc.pos > fs.preflight[best].pos {
					best = i
				}
			}
			if best == -1 {
				t.Errorf("%s (%s): a gitutil.%s call at %s has no DISTINCT, operand-corresponding preflightMergeDestruction call preceding it in the same function-like construct — a merge producer must consult the R4 preflight over its OWN operands (AC-7(iv))", fs.label, fs.file, mc.kind, fset.Position(mc.pos))
				continue
			}
			consumed[best] = true
		}
	}

	if nonEmpty == 0 {
		t.Fatal("found zero MergeInto/MergeBranch call sites in internal/executor — this test's own probe is broken (the producers must have moved or been renamed)")
	}

	// AC-7(iv) pins the enumeration as "whole and unstaged": exactly the
	// three producers this spec names (CompleteBead's bead→spec MergeInto,
	// FinalizeEpic's bead→spec auto-merge, and the direct spec→main
	// MergeBranch). A count outside {3} means either a producer was
	// removed (shrinking the surface this AC guards) or a new one
	// appeared — both worth a human's attention, not a silent pass. G1's
	// ruling on this literal (bead-6 fix round 1): "appropriate as an
	// intended-inventory review ratchet, but not a substitute for operand
	// dataflow" — kept alongside, never instead of, the correspondence
	// check above.
	const wantMergeCallSites = 3
	if totalCalls != wantMergeCallSites {
		t.Errorf("found %d total MergeInto/MergeBranch call site(s) in internal/executor production code, want exactly %d (spec 127's three enumerated producers) — re-audit this count if a producer was legitimately added or removed", totalCalls, wantMergeCallSites)
	}
}

// TestAdversaryArbitraryInlineCallbackIsNotTransparent is bead-6 fix
// round 2 (G1's confirm-round finding): an inline func literal passed as
// a direct argument to some OTHER function — never resumeAwareMerge — is
// syntactically identical to the ONE shape this file's own transparency
// rule is scoped to (a *ast.FuncLit that is a direct *ast.CallExpr
// argument), but carries none of the synchronous-execution guarantee
// that shape's carve-out depends on. Restoring the prior, unqualified
// collectInlineArgFuncLits (any call, not just resumeAwareMerge) makes
// this test fail red: it marks the closure below transparent, which
// would let an UNRELATED preflightMergeDestruction call in the enclosing
// function wrongly "cover" a merge call that established no preflight
// obligation of its own.
func TestAdversaryArbitraryInlineCallbackIsNotTransparent(t *testing.T) {
	src := `package p

func someOtherHelper(cb func() error) error { return cb() }

func producer() error {
	preflightMergeDestruction("branch1", "target1", "", "rerun")
	return someOtherHelper(func() error {
		return gitutil.MergeInto("wt", "branch1")
	})
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "adversary.go", src, 0)
	if err != nil {
		t.Fatalf("parsing fixture source: %v", err)
	}

	inline := collectInlineArgFuncLits(file)

	var closureLit *ast.FuncLit
	var producerDecl *ast.FuncDecl
	ast.Inspect(file, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.FuncLit:
			closureLit = v
		case *ast.FuncDecl:
			if v.Name.Name == "producer" {
				producerDecl = v
			}
		}
		return true
	})
	if closureLit == nil || producerDecl == nil {
		t.Fatal("fixture invariant broken: expected exactly one func literal and a producer FuncDecl")
	}

	if inline[closureLit] {
		t.Fatal("a func literal passed to an ARBITRARY function (not resumeAwareMerge) must NOT be treated as transparent — nothing guarantees it runs synchronously the way resumeAwareMerge's own mergeFn argument does")
	}

	// Consequence check: with the closure correctly NOT transparent,
	// collectSpanCalls over producer's OWN span must not see the merge
	// call inside it at all (descent stops at the nested, non-inline
	// FuncLit boundary) — the merge call belongs to the closure's OWN
	// span, which the real repo-wide scan visits separately and which
	// carries no preflight call of its own in this fixture.
	outerMerges, _ := collectSpanCalls(producerDecl.Body, nil, inline, "gitutil")
	if len(outerMerges) != 0 {
		t.Fatalf("producer's own span must not see the closure's merge call once the closure is correctly non-transparent; got %d", len(outerMerges))
	}
	closureMerges, closurePreflights := collectSpanCalls(closureLit.Body, nil, inline, "gitutil")
	if len(closureMerges) != 1 {
		t.Fatalf("the closure's own span must see its own merge call; got %d", len(closureMerges))
	}
	if len(closurePreflights) != 0 {
		t.Fatal("fixture invariant broken: the closure must carry no preflight call of its own")
	}
}

// TestCollectMergeFnAliases_ResolvesChainOfAnyDepth is bead-6 fix round
// 3's acceptance test for G1-2's confirm-round finding: O2's own
// adversarial repro (`var A = gitutil.MergeInto; var B = A`) escaped the
// pre-round-3 single-hop alias resolution entirely — B's call sites were
// invisible to totalCalls. collectMergeFnAliases must now resolve a
// chain of ANY depth, and must NOT resolve a chain that bottoms out at
// something unrelated to gitutil.MergeInto/MergeBranch.
func TestCollectMergeFnAliases_ResolvesChainOfAnyDepth(t *testing.T) {
	src := `package p

var directMergeInto = gitutil.MergeInto
var oneHop = directMergeInto
var twoHop = oneHop
var threeHop = twoHop

var directMergeBranch = gitutil.MergeBranch
var branchOneHop = directMergeBranch

var unrelated = someOtherPackage.SomeFunc
var unrelatedHop = unrelated
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "aliaschain.go", src, 0)
	if err != nil {
		t.Fatalf("parsing fixture source: %v", err)
	}

	aliases := collectMergeFnAliases([]*ast.File{file})

	for _, name := range []string{"directMergeInto", "oneHop", "twoHop", "threeHop"} {
		if got := aliases[name]; got != "MergeInto" {
			t.Errorf("aliases[%q] = %q, want %q (a chain of any depth must resolve to MergeInto, matching O2's exact adversarial repro)", name, got, "MergeInto")
		}
	}
	for _, name := range []string{"directMergeBranch", "branchOneHop"} {
		if got := aliases[name]; got != "MergeBranch" {
			t.Errorf("aliases[%q] = %q, want %q", name, got, "MergeBranch")
		}
	}
	for _, name := range []string{"unrelated", "unrelatedHop"} {
		if _, ok := aliases[name]; ok {
			t.Errorf("aliases[%q] must NOT be resolved — its chain bottoms out at an unrelated function, never gitutil.MergeInto/MergeBranch", name)
		}
	}
}

// TestFuncParamsMatchingMergeSignature_CatchesParameterIndirection is
// bead-6 fix round 3's acceptance test for the OTHER half of G1-2's
// confirm-round finding: a function-typed PARAMETER matching
// gitutil.MergeInto/MergeBranch's own signature, called directly, must be
// flagged — this AST scan cannot trace what value such a parameter is
// bound to at any call site. resumeAwareMerge's own mergeFn/
// conflictFailure parameters (`func() error`/`func(error) error`) must
// NEVER match, since flagging those would make this ratchet unusable
// against its own file.
func TestFuncParamsMatchingMergeSignature_CatchesParameterIndirection(t *testing.T) {
	src := `package p

func g1UnpreflightedProducer(mergeIntoLike func(string, string) error, mergeBranchLike func(string, string, string) error) error {
	return mergeIntoLike("wt", "branch1")
}

func ordinaryResumeAwareMergeShape(mergeFn func() error, conflictFailure func(error) error) error {
	return mergeFn()
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "paramindirection.go", src, 0)
	if err != nil {
		t.Fatalf("parsing fixture source: %v", err)
	}

	var adversary, ordinary *ast.FuncDecl
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		switch fd.Name.Name {
		case "g1UnpreflightedProducer":
			adversary = fd
		case "ordinaryResumeAwareMergeShape":
			ordinary = fd
		}
	}
	if adversary == nil || ordinary == nil {
		t.Fatal("fixture invariant broken: expected both FuncDecls")
	}

	adversaryParams := funcParamsMatchingMergeSignature(adversary.Type.Params, nil)
	wantParams := map[string]bool{"mergeIntoLike": true, "mergeBranchLike": true}
	if len(adversaryParams) != len(wantParams) {
		t.Fatalf("funcParamsMatchingMergeSignature(adversary) = %v, want exactly %v", adversaryParams, wantParams)
	}
	for _, name := range adversaryParams {
		if !wantParams[name] {
			t.Errorf("unexpected matched param %q", name)
		}
		if !paramCalledAsFunc(adversary.Body, name) && name == "mergeIntoLike" {
			t.Errorf("paramCalledAsFunc must detect %q being called directly", name)
		}
	}

	if got := funcParamsMatchingMergeSignature(ordinary.Type.Params, nil); len(got) != 0 {
		t.Errorf("resumeAwareMerge's own mergeFn()/conflictFailure() shapes must NEVER match gitutil.MergeInto/MergeBranch's signature (0/1 string params, not 2/3); got %v", got)
	}
}

// TestCollectUnresolvedMergeSignatureVars_CatchesUninitializedVarIndirection
// is bead-6 fix round 4's acceptance test for item 2: a package-level var
// declared WITHOUT an initializer, whose OWN declared type matches
// gitutil.MergeInto/MergeBranch's signature exactly, and called directly —
// the THIRD unresolvable shape (alongside the alias-chain and
// parameter-indirection shapes rounds 3/1 already closed) that was
// previously invisible to this scan entirely: not an alias assignment
// (collectMergeFnAliases has no initializer expression to trace) and not a
// function parameter (failClosedOnMergeSignatureParams only looks at
// Params). A var WITH an initializer — even one this scan cannot resolve
// to gitutil.MergeInto/MergeBranch — must NOT be caught here: that is
// collectMergeFnAliases' own concern, and conflating the two would flag
// every ordinary same-shaped function value as if it were unresolvable.
func TestCollectUnresolvedMergeSignatureVars_CatchesUninitializedVarIndirection(t *testing.T) {
	src := `package p

var mergeIntoLikeUninitialized func(string, string) error
var mergeBranchLikeUninitialized func(string, string, string) error

// An ordinary initialized var of a DIFFERENT (unrelated) shape must never
// be caught here — it is a real, resolvable value, just not one this
// scan traces (that is collectMergeFnAliases' own residual, not this
// check's concern).
var initializedUnrelated func(string, string) error = someOtherPackage.SomeFunc

func g2UnpreflightedProducer() error {
	return mergeIntoLikeUninitialized("wt", "branch1")
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "unresolvedvar.go", src, 0)
	if err != nil {
		t.Fatalf("parsing fixture source: %v", err)
	}

	unresolved := collectUnresolvedMergeSignatureVars([]*ast.File{file}, nil)

	for _, name := range []string{"mergeIntoLikeUninitialized", "mergeBranchLikeUninitialized"} {
		if !unresolved[name] {
			t.Errorf("collectUnresolvedMergeSignatureVars must catch %q: declared with a matching signature and no initializer", name)
		}
	}
	if unresolved["initializedUnrelated"] {
		t.Error("collectUnresolvedMergeSignatureVars must NOT catch a var that DOES carry an initializer — that is collectMergeFnAliases' own concern, not this check's")
	}

	var producer *ast.FuncDecl
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == "g2UnpreflightedProducer" {
			producer = fd
		}
	}
	if producer == nil {
		t.Fatal("fixture invariant broken: expected the producer FuncDecl")
	}
	if !paramCalledAsFunc(producer.Body, "mergeIntoLikeUninitialized") {
		t.Fatal("paramCalledAsFunc must detect the uninitialized var being called directly — this is the same direct-call detection failClosedOnUnresolvedMergeSignatureVars relies on")
	}
}

// TestCollectUnresolvedMergeSignatureVars_ResolvesTypeAliasIndirection is
// bead-6 fix round 5's acceptance test for G1's confirm-round finding,
// shape (a): a package-level var declared with a TYPE ALIAS (or a
// distinct NAMED TYPE) whose OWN underlying type matches
// gitutil.MergeInto/MergeBranch's signature exactly — `type
// MergeSignature = func(string, string) error; var f MergeSignature`,
// assigned only inside an init() func — is exactly the same unresolvable
// shape as the literal `func(string, string) error` spelling round 4
// caught, but round 4's own `vs.Type.(*ast.FuncType)` type assertion
// missed it entirely (an *ast.Ident naming the alias, never literally a
// FuncType). This reproduces G1's own real repro (the type-alias +
// init() attack) at the collector level.
func TestCollectUnresolvedMergeSignatureVars_ResolvesTypeAliasIndirection(t *testing.T) {
	src := `package p

type MergeSignature = func(string, string) error

var mergeIntoLikeAliased MergeSignature

func init() {
	mergeIntoLikeAliased = gitutil.MergeInto
}

func g3UnpreflightedProducer() error {
	return mergeIntoLikeAliased("wt", "branch1")
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "typealiasvar.go", src, 0)
	if err != nil {
		t.Fatalf("parsing fixture source: %v", err)
	}

	typeAliases := collectFuncTypeAliases([]*ast.File{file})
	if _, ok := typeAliases["MergeSignature"]; !ok {
		t.Fatal("collectFuncTypeAliases must resolve the `type MergeSignature = func(string, string) error` alias declaration")
	}

	unresolved := collectUnresolvedMergeSignatureVars([]*ast.File{file}, typeAliases)
	if !unresolved["mergeIntoLikeAliased"] {
		t.Error("collectUnresolvedMergeSignatureVars must catch a var declared with a TYPE ALIAS matching the signature, not only the literal func-type spelling")
	}

	var producer *ast.FuncDecl
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == "g3UnpreflightedProducer" {
			producer = fd
		}
	}
	if producer == nil {
		t.Fatal("fixture invariant broken: expected the producer FuncDecl")
	}
	if !paramCalledAsFunc(producer.Body, "mergeIntoLikeAliased") {
		t.Fatal("paramCalledAsFunc must detect the aliased var being called directly")
	}
}

// TestCollectRiskyMemberNames_CatchesStructFieldIndirection is bead-6 fix
// round 5's acceptance test for G1's confirm-round finding, shape (b): a
// struct field GENUINELY assigned gitutil.MergeInto via a composite
// literal, invoked elsewhere via a selector (`h.run(...)`), is invisible
// to every prior leg (not a package var, not a function parameter) —
// reproduces G1's own real repro at the collector level. Deliberately
// includes an UNRELATED field of the identical bare shape
// (mirroring this package's own real MockExecutor.ResetHardFn) to prove
// this is a "genuinely assigned" check, not shape alone.
func TestCollectRiskyMemberNames_CatchesStructFieldIndirection(t *testing.T) {
	src := `package p

type g1ScratchHandler struct {
	run           func(string, string) error
	unrelatedSame func(string, string) error
}

func newG1ScratchHandler() g1ScratchHandler {
	return g1ScratchHandler{run: gitutil.MergeInto}
}

func g4UnpreflightedProducer(h g1ScratchHandler) error {
	return h.run("wt", "branch1")
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "structfield.go", src, 0)
	if err != nil {
		t.Fatalf("parsing fixture source: %v", err)
	}

	risky := collectRiskyMemberNames([]*ast.File{file}, nil, collectMergeFnAliases([]*ast.File{file}))
	if !risky["run"] {
		t.Error(`collectRiskyMemberNames must catch a struct field named "run" GENUINELY assigned gitutil.MergeInto`)
	}
	if risky["unrelatedSame"] {
		t.Error(`collectRiskyMemberNames must NOT catch "unrelatedSame" — same bare shape, never assigned a merge producer (the false-positive this narrower check exists to avoid)`)
	}

	var producer *ast.FuncDecl
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == "g4UnpreflightedProducer" {
			producer = fd
		}
	}
	if producer == nil {
		t.Fatal("fixture invariant broken: expected the producer FuncDecl")
	}
	found := false
	ast.Inspect(producer.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == "run" {
			found = true
		}
		return true
	})
	if !found {
		t.Fatal("fixture invariant broken: expected an `h.run(...)` selector call in the producer body")
	}
}

// TestCollectRiskyMemberNames_CatchesNamedFuncTypeMethodIndirection is
// bead-6 fix round 5's acceptance test for G1's confirm-round finding,
// shape (c): a NAMED FUNCTION TYPE whose underlying type matches
// gitutil.MergeInto/MergeBranch's signature exactly, converted from
// gitutil.MergeInto and invoked through a METHOD on that named type
// (`m.Invoke(...)`) — reproduces G1's own real repro at the collector
// level.
func TestCollectRiskyMemberNames_CatchesNamedFuncTypeMethodIndirection(t *testing.T) {
	src := `package p

type g1ScratchNamedFunc func(string, string) error

func (f g1ScratchNamedFunc) Invoke(a, b string) error { return f(a, b) }

// An UNRELATED named type of the identical bare shape, never converted
// from a merge producer — proves this is a "genuinely converted" check,
// not shape alone. A DIFFERENT method name so its absence from the risky
// set is actually observable (the risky vocabulary is by name, so an
// identically-named method on the converted type would be flagged
// regardless of which type this one belongs to).
type g1ScratchUnrelatedNamedFunc func(string, string) error

func (f g1ScratchUnrelatedNamedFunc) InvokeUnrelated(a, b string) error { return f(a, b) }

func newG1ScratchNamedFunc() g1ScratchNamedFunc {
	return g1ScratchNamedFunc(gitutil.MergeInto)
}

func g5UnpreflightedProducer(m g1ScratchNamedFunc) error {
	return m.Invoke("wt", "branch1")
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "namedfunctypemethod.go", src, 0)
	if err != nil {
		t.Fatalf("parsing fixture source: %v", err)
	}

	typeAliases := collectFuncTypeAliases([]*ast.File{file})
	if _, ok := typeAliases["g1ScratchNamedFunc"]; !ok {
		t.Fatal("collectFuncTypeAliases must resolve the named function type declaration")
	}
	risky := collectRiskyMemberNames([]*ast.File{file}, typeAliases, collectMergeFnAliases([]*ast.File{file}))
	if !risky["Invoke"] {
		t.Error(`collectRiskyMemberNames must catch a method named "Invoke" whose receiver's named type was GENUINELY converted from gitutil.MergeInto`)
	}
	if risky["InvokeUnrelated"] {
		t.Error(`collectRiskyMemberNames must NOT catch "InvokeUnrelated" — its receiver's named type was never converted from a merge producer (the false-positive this narrower check exists to avoid)`)
	}

	var producer *ast.FuncDecl
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == "g5UnpreflightedProducer" {
			producer = fd
		}
	}
	if producer == nil {
		t.Fatal("fixture invariant broken: expected the producer FuncDecl")
	}
	found := false
	ast.Inspect(producer.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == "Invoke" {
			found = true
		}
		return true
	})
	if !found {
		t.Fatal("fixture invariant broken: expected an `m.Invoke(...)` selector call in the producer body")
	}
}

// TestCollectRiskyMemberNames_CatchesPositionalCompositeLiteral is bead-6
// fix round 6's acceptance test for G1-2's confirm-round finding, shape 2:
// a struct field genuinely assigned a merge producer via a POSITIONAL
// composite-literal element (`handler{gitutil.MergeInto}`, no field key)
// — round 5's collectRiskyMemberNames only ever walked KeyValueExpr
// elements, so this shape was invisible even though the field is just as
// genuinely assigned as the keyed form.
func TestCollectRiskyMemberNames_CatchesPositionalCompositeLiteral(t *testing.T) {
	src := `package p

type g6ScratchPositionalHandler struct {
	run func(string, string) error
}

func newG6ScratchPositionalHandler() g6ScratchPositionalHandler {
	return g6ScratchPositionalHandler{gitutil.MergeInto}
}

func g6PositionalUnpreflightedProducer(h g6ScratchPositionalHandler) error {
	return h.run("wt", "branch1")
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "positionalliteral.go", src, 0)
	if err != nil {
		t.Fatalf("parsing fixture source: %v", err)
	}

	risky := collectRiskyMemberNames([]*ast.File{file}, nil, collectMergeFnAliases([]*ast.File{file}))
	if !risky["run"] {
		t.Error(`collectRiskyMemberNames must catch a struct field named "run" GENUINELY assigned gitutil.MergeInto via a POSITIONAL composite-literal element`)
	}

	var producer *ast.FuncDecl
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == "g6PositionalUnpreflightedProducer" {
			producer = fd
		}
	}
	if producer == nil {
		t.Fatal("fixture invariant broken: expected the producer FuncDecl")
	}
	found := false
	ast.Inspect(producer.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "run" {
			found = true
		}
		return true
	})
	if !found {
		t.Fatal("fixture invariant broken: expected an `h.run(...)` selector call in the producer body")
	}
}

// TestCollectRiskyMemberNames_CatchesPostConstructionAssignment is bead-6
// fix round 6's acceptance test for G1-2's confirm-round finding, shape 1:
// a struct field genuinely assigned a merge producer via a
// POST-CONSTRUCTION assignment statement (`h.run = gitutil.MergeInto`) —
// round 5's collectRiskyMemberNames only ever walked composite-literal
// elements, never *ast.AssignStmt, so this ordinary Go spelling (not an
// adversarial contrivance) was invisible.
func TestCollectRiskyMemberNames_CatchesPostConstructionAssignment(t *testing.T) {
	src := `package p

type g6ScratchAssignedHandler struct {
	run func(string, string) error
}

func g6AssignedUnpreflightedProducer() error {
	var h g6ScratchAssignedHandler
	h.run = gitutil.MergeInto
	return h.run("wt", "branch1")
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "postassign.go", src, 0)
	if err != nil {
		t.Fatalf("parsing fixture source: %v", err)
	}

	risky := collectRiskyMemberNames([]*ast.File{file}, nil, collectMergeFnAliases([]*ast.File{file}))
	if !risky["run"] {
		t.Error(`collectRiskyMemberNames must catch a struct field named "run" GENUINELY assigned gitutil.MergeInto via a POST-CONSTRUCTION assignment statement`)
	}

	var producer *ast.FuncDecl
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == "g6AssignedUnpreflightedProducer" {
			producer = fd
		}
	}
	if producer == nil {
		t.Fatal("fixture invariant broken: expected the producer FuncDecl")
	}
	found := false
	ast.Inspect(producer.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "run" {
			found = true
		}
		return true
	})
	if !found {
		t.Fatal("fixture invariant broken: expected an `h.run(...)` selector call in the producer body")
	}
}

// TestGitutilLocalName_ResolvesImportAlias is bead-6 fix round 6's
// acceptance test for G1-2's confirm-round finding, shape 3: a direct
// gitutil.MergeInto/MergeBranch call reached through an ALIASED import
// (`import gu ".../internal/gitutil"; gu.MergeInto(...)`) — ordinary Go,
// not an adversarial contrivance — was invisible to every hard-coded
// literal "gitutil" check in this ratchet at once (collectSpanCalls,
// collectMergeFnAliases, isMergeProducerExpr, failClosedOnRiskyMemberCalls
// all shared the same defect). Proves gitutilLocalName resolves the
// alias, and that collectSpanCalls driven by the stale, hard-coded
// literal "gitutil" — round 5's actual behavior — would have missed the
// call entirely.
func TestGitutilLocalName_ResolvesImportAlias(t *testing.T) {
	src := `package p

import gu "github.com/mrmaxsteel/mindspec/internal/gitutil"

func g6AliasUnpreflightedProducer() error {
	return gu.MergeInto("wt", "branch1")
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "importalias.go", src, 0)
	if err != nil {
		t.Fatalf("parsing fixture source: %v", err)
	}

	gitutilName := gitutilLocalName(file)
	if gitutilName != "gu" {
		t.Fatalf(`gitutilLocalName must resolve the aliased import to "gu", got %q`, gitutilName)
	}

	var producer *ast.FuncDecl
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == "g6AliasUnpreflightedProducer" {
			producer = fd
		}
	}
	if producer == nil {
		t.Fatal("fixture invariant broken: expected the producer FuncDecl")
	}

	inline := collectInlineArgFuncLits(file)
	merges, _ := collectSpanCalls(producer.Body, nil, inline, gitutilName)
	if len(merges) != 1 {
		t.Fatalf("collectSpanCalls, driven by the resolved alias %q, must see the gu.MergeInto call; got %d", gitutilName, len(merges))
	}

	// The defect this round fixes: driven by the stale, hard-coded literal
	// "gitutil" (round 5's own behavior), the identical call is invisible.
	staleMerges, _ := collectSpanCalls(producer.Body, nil, inline, "gitutil")
	if len(staleMerges) != 0 {
		t.Fatalf(`fixture invariant broken: the literal "gitutil" name must NOT match a gu.-aliased call — got %d`, len(staleMerges))
	}
}

// TestCollectRiskyMemberNames_BareNameCollisionAcrossUnrelatedTypesIsAcceptedResidual
// documents bead-6 fix round 6's disclosed, NONBLOCKING trade-off (G1-2's
// confirm-round finding, the false-positive half — see
// collectRiskyMemberNames' own doc comment): the risky vocabulary is keyed
// by BARE member name alone, never by receiver type identity, so a field
// genuinely assigned a merge producer on one type makes an UNRELATED
// type's same-named, different-signature member call red purely on the
// name collision. Pinned here, attacking the OPPOSITE direction from
// TestCollectRiskyMemberNames_CatchesStructFieldIndirection's own
// "unrelatedSame" false-positive check (same bare shape, never assigned —
// correctly NOT risky) — this fixture's unrelated field IS caught, a
// real, demonstrated false positive, never a false negative, so a future
// change to this scan's precision is a deliberate choice, not an
// accidental regression.
func TestCollectRiskyMemberNames_BareNameCollisionAcrossUnrelatedTypesIsAcceptedResidual(t *testing.T) {
	src := `package p

type g6ScratchAssignedHandler struct {
	run func(string, string) error
}

func newG6ScratchAssignedHandler() g6ScratchAssignedHandler {
	return g6ScratchAssignedHandler{run: gitutil.MergeInto}
}

// An UNRELATED type sharing the bare field name "run", never itself
// assigned a merge producer, and a DIFFERENT signature — this scan
// cannot rule out that v.run() below resolves to something other than
// g6ScratchAssignedHandler's own genuinely-assigned field, so it fires
// anyway (the accepted residual this test documents).
type g6UnrelatedType struct {
	run func() error
}

func g6UnrelatedProducer(v g6UnrelatedType) error {
	return v.run()
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "collision.go", src, 0)
	if err != nil {
		t.Fatalf("parsing fixture source: %v", err)
	}

	risky := collectRiskyMemberNames([]*ast.File{file}, nil, collectMergeFnAliases([]*ast.File{file}))
	if !risky["run"] {
		t.Fatal(`fixture invariant broken: expected "run" to be risky via g6ScratchAssignedHandler's genuine assignment`)
	}

	var unrelatedProducer *ast.FuncDecl
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == "g6UnrelatedProducer" {
			unrelatedProducer = fd
		}
	}
	if unrelatedProducer == nil {
		t.Fatal("fixture invariant broken: expected the unrelated producer FuncDecl")
	}
	found := false
	ast.Inspect(unrelatedProducer.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "run" {
			found = true
		}
		return true
	})
	if !found {
		t.Fatal("fixture invariant broken: expected a `v.run()` selector call in the unrelated producer body")
	}
	// risky["run"] is already true above (verified against
	// g6ScratchAssignedHandler's genuine assignment); a repo-wide
	// failClosedOnRiskyMemberCalls pass over this same file would
	// therefore RED g6UnrelatedProducer's v.run() call too, even though it
	// has nothing to do with gitutil.MergeInto/MergeBranch — the accepted,
	// disclosed false-positive this test exists to pin.
}

// TestFailClosedOnUnresolvedMergeSignatureVars_ZeroFalsePositivesOnRealTree
// confirms (real-git-repo-adjacent, but here a real-source-tree check) that
// the new fail-closed leg does not fire against internal/executor's own
// production code: TestMergeChokepoint_EveryProducerConsultsThePreflight
// already runs failClosedOnUnresolvedMergeSignatureVars over every real
// span in this package as part of its normal pass — this test exists
// separately so a false positive here is diagnosable on its own, without
// wading through the chokepoint test's other assertions, and so the ONE
// package-level func-typed var without an initializer this package
// actually has (finalizeStepHookFn, `func(stage string) error` — one
// string param, not two or three) is pinned as a known-safe non-match by
// name, not merely by the absence of a failure.
func TestFailClosedOnUnresolvedMergeSignatureVars_ZeroFalsePositivesOnRealTree(t *testing.T) {
	root := mergeChokepointRepoRoot(t)
	pkgDir := filepath.Join(root, "internal", "executor")
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		t.Fatalf("reading %s: %v", pkgDir, err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".go" || len(e.Name()) > 8 && e.Name()[len(e.Name())-8:] == "_test.go" {
			continue
		}
		path := filepath.Join(pkgDir, e.Name())
		file, ferr := parser.ParseFile(fset, path, nil, 0)
		if ferr != nil {
			t.Fatalf("parsing %s: %v", path, ferr)
		}
		files = append(files, file)
	}

	typeAliases := collectFuncTypeAliases(files)
	unresolved := collectUnresolvedMergeSignatureVars(files, typeAliases)
	if unresolved["finalizeStepHookFn"] {
		t.Error("finalizeStepHookFn (func(stage string) error, one string param) must NOT match gitutil.MergeInto/MergeBranch's signature (two or three) — a real, pre-existing package-level var without an initializer must never be a false positive")
	}
	if len(unresolved) != 0 {
		t.Errorf("expected zero unresolved-var false positives against the real internal/executor tree, got %v", unresolved)
	}
}

// TestCollectRiskyMemberNames_ZeroFalsePositivesOnRealTree mirrors
// TestFailClosedOnUnresolvedMergeSignatureVars_ZeroFalsePositivesOnRealTree
// for bead-6 fix round 5's struct-field/named-function-type-method leg
// (collectRiskyMemberNames): internal/executor's own production code
// declares no struct field or method whose type carries
// gitutil.MergeInto/MergeBranch's exact signature, so this must find
// nothing — a separate, dedicated test so a real false positive here
// (a genuine, unrelated same-shaped field or method added later) is
// diagnosable on its own, without wading through the chokepoint test's
// other assertions.
func TestCollectRiskyMemberNames_ZeroFalsePositivesOnRealTree(t *testing.T) {
	root := mergeChokepointRepoRoot(t)
	pkgDir := filepath.Join(root, "internal", "executor")
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		t.Fatalf("reading %s: %v", pkgDir, err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".go" || len(e.Name()) > 8 && e.Name()[len(e.Name())-8:] == "_test.go" {
			continue
		}
		path := filepath.Join(pkgDir, e.Name())
		file, ferr := parser.ParseFile(fset, path, nil, 0)
		if ferr != nil {
			t.Fatalf("parsing %s: %v", path, ferr)
		}
		files = append(files, file)
	}

	typeAliases := collectFuncTypeAliases(files)
	mergeAliases := collectMergeFnAliases(files)
	risky := collectRiskyMemberNames(files, typeAliases, mergeAliases)
	if len(risky) != 0 {
		t.Errorf("expected zero risky-member false positives against the real internal/executor tree, got %v", risky)
	}
}
