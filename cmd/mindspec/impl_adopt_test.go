package main

// Spec 127 bead 3 — the `mindspec impl adopt` cmd-layer surface:
// leaf-identity registration, flag-set membership (AC-11's "resolves at
// leaf identity" discipline, ceremony_guard_test.go's resolveCommand
// pattern), and R1(e)'s call-site enumeration ("the only caller of the
// adopt entrypoint is its own registered command handler").

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

// approveImportPath is the import path findAdoptSpecCallSites resolves
// against, by IDENTITY, never by the literal source-level spelling of a
// file's local qualifier for it (O2-1/G1-B3-05).
const approveImportPath = "github.com/mrmaxsteel/mindspec/internal/approve"

// TestImplAdoptCmd_ResolvesAtLeafIdentity pins AC-11(a): `impl adopt`
// resolves against the real command tree at leaf identity
// (cmd.Name() == "adopt", not merely "found something under impl").
func TestImplAdoptCmd_ResolvesAtLeafIdentity(t *testing.T) {
	cmd := resolveCommand(t, "impl", "adopt")
	if cmd != implAdoptCmd {
		t.Fatalf("resolveCommand(impl, adopt) returned a different *cobra.Command than implAdoptCmd")
	}
}

// TestImplAdoptCmd_FlagSetIsExactlyReasonAndAttest pins AC-11(a)'s
// named-flag assertion via flag-set membership on the resolved leaf:
// exactly {--reason, --attest-unverified}, no bypass-shaped extra flag.
func TestImplAdoptCmd_FlagSetIsExactlyReasonAndAttest(t *testing.T) {
	cmd := resolveCommand(t, "impl", "adopt")
	var names []string
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) { names = append(names, f.Name) })
	want := setOf("reason", "attest-unverified")
	got := setOf(names...)
	if len(got) != len(want) {
		t.Fatalf("impl adopt local flag set = %v, want exactly {reason, attest-unverified}", names)
	}
	for n := range want {
		if !got[n] {
			t.Errorf("impl adopt is missing expected flag --%s", n)
		}
	}
	for n := range got {
		if !want[n] {
			t.Errorf("impl adopt has unexpected extra flag --%s", n)
		}
	}
}

// TestImplAdoptCmd_ReasonRequiredArgs pins the ExactArgs(1) contract:
// exactly one positional (the spec id).
func TestImplAdoptCmd_ReasonRequiredArgs(t *testing.T) {
	cmd := resolveCommand(t, "impl", "adopt")
	if err := cmd.Args(cmd, []string{"127-test"}); err != nil {
		t.Errorf("one spec-id arg must be accepted: %v", err)
	}
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("zero args must be rejected")
	}
	if err := cmd.Args(cmd, []string{"127-test", "extra"}); err == nil {
		t.Error("a second positional operand must be rejected")
	}
}

// TestAdoptSpec_OnlyProductionCallerIsItsCommandHandler is R1(e): "the
// only caller of the adopt entrypoint is its own registered command
// handler" (O2-8's rewrite of the untestable negative universal) —
// pinned as a POSITIVE, testable claim: exactly one call site of
// approve.AdoptSpec across every non-test .go file under cmd/ and
// internal/, and it must be inside cmd/mindspec/impl.go's
// adoptSpecRunE (the registered handler above).
func TestAdoptSpec_OnlyProductionCallerIsItsCommandHandler(t *testing.T) {
	root := repoRootFromTestDir(t)
	var sites []adoptCallSite
	for _, top := range []string{"cmd", "internal"} {
		dir := filepath.Join(root, top)
		if err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			sites = append(sites, findAdoptSpecCallSites(t, path)...)
			return nil
		}); err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
	}
	if len(sites) == 0 {
		t.Fatal("found ZERO call sites of approve.AdoptSpec in production source — this scan has regressed (it should find exactly the one registered handler call)")
	}
	if len(sites) != 1 {
		t.Fatalf("expected exactly ONE production call site of approve.AdoptSpec, found %d: %v", len(sites), sites)
	}
	got := sites[0]
	if got.file != "cmd/mindspec/impl.go" || got.funcName != "adoptSpecRunE" {
		t.Fatalf("the sole call site of approve.AdoptSpec must be cmd/mindspec/impl.go's adoptSpecRunE (the registered `impl adopt` handler), got %s in %s", got.funcName, got.file)
	}
}

type adoptCallSite struct {
	file, funcName string
}

// findAdoptSpecCallSites parses absPath and returns one adoptCallSite
// per call expression that resolves — by IMPORT IDENTITY, never by the
// literal source-level spelling of a qualifier (O2-1/G1-B3-05: the prior
// scan matched a CallExpr only when its qualifier's Ident.Name was
// LITERALLY "approve", so an aliased import (`ap "…/internal/approve"`)
// or a dot import made a real second call site invisible — proven by
// mutation: an aliased-import probe call site still reported as a
// single-call-site pass) — to internal/approve.AdoptSpec, tagged with
// the name of its nearest enclosing top-level function/method (or
// "<package-level>" if none).
//
// Resolution: first walk this FILE's own import specs to learn its local
// name(s) for approveImportPath — an explicit alias, a dot import, or
// (with no alias) the default "approve" — then match CallExprs against
// those resolved facts rather than against the string "approve". A
// same-spelled import from a DIFFERENT path (e.g. some other package
// also named "approve") is correctly NOT matched, because its import
// path differs.
func findAdoptSpecCallSites(t *testing.T, absPath string) []adoptCallSite {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, absPath, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", absPath, err)
	}
	root := repoRootFromTestDir(t)
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		t.Fatalf("relativizing %s: %v", absPath, err)
	}

	qualifiedNames := map[string]bool{} // local identifier -> true, iff it names approveImportPath in THIS file
	dotImported := false
	for _, imp := range file.Imports {
		path, unquoteErr := strconv.Unquote(imp.Path.Value)
		if unquoteErr != nil || path != approveImportPath {
			continue
		}
		switch {
		case imp.Name == nil:
			qualifiedNames["approve"] = true
		case imp.Name.Name == ".":
			dotImported = true
		case imp.Name.Name == "_":
			// Blank import: no identifier can ever call through it.
		default:
			qualifiedNames[imp.Name.Name] = true
		}
	}

	var sites []adoptCallSite
	var funcStack []string
	var visit func(n ast.Node) bool
	visit = func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.FuncDecl:
			name := v.Name.Name
			if v.Recv != nil && len(v.Recv.List) == 1 {
				if t, ok := v.Recv.List[0].Type.(*ast.StarExpr); ok {
					if id, ok := t.X.(*ast.Ident); ok {
						name = id.Name + "." + name
					}
				} else if id, ok := v.Recv.List[0].Type.(*ast.Ident); ok {
					name = id.Name + "." + name
				}
			}
			funcStack = append(funcStack, name)
			ast.Inspect(v.Body, visit)
			funcStack = funcStack[:len(funcStack)-1]
			return false
		case *ast.CallExpr:
			matched := false
			switch fn := v.Fun.(type) {
			case *ast.SelectorExpr:
				if pkgIdent, ok := fn.X.(*ast.Ident); ok && qualifiedNames[pkgIdent.Name] && fn.Sel.Name == "AdoptSpec" {
					matched = true
				}
			case *ast.Ident:
				if dotImported && fn.Name == "AdoptSpec" {
					matched = true
				}
			}
			if matched {
				name := "<package-level>"
				if len(funcStack) > 0 {
					name = funcStack[len(funcStack)-1]
				}
				sites = append(sites, adoptCallSite{file: rel, funcName: name})
			}
		}
		return true
	}
	ast.Inspect(file, visit)
	return sites
}

// TestFindAdoptSpecCallSites_ResolvesByImportIdentityNotSpelling is the
// O2-1/G1-B3-05 regression: the enumeration must resolve call sites by
// this file's ACTUAL import path for internal/approve, never by the
// literal source-level spelling of the qualifier "approve" — an aliased
// or dot-imported call must be found exactly like the plain one, and a
// SAME-SPELLED import from an UNRELATED path must never be matched.
func TestFindAdoptSpecCallSites_ResolvesByImportIdentityNotSpelling(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{
			name: "plain_import",
			source: `package probe

import "github.com/mrmaxsteel/mindspec/internal/approve"

func caller() { approve.AdoptSpec("", "", nil, approve.AdoptOpts{}) }
`,
		},
		{
			// O2's exact mutation probe (verbatim shape): an aliased
			// import must resolve identically to the plain case.
			name: "aliased_import",
			source: `package probe

import adoptpkg "github.com/mrmaxsteel/mindspec/internal/approve"

func caller() { adoptpkg.AdoptSpec("", "", nil, adoptpkg.AdoptOpts{}) }
`,
		},
		{
			name: "dot_import",
			source: `package probe

import . "github.com/mrmaxsteel/mindspec/internal/approve"

func caller() { AdoptSpec("", "", nil, AdoptOpts{}) }
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "probe.go")
			if err := os.WriteFile(path, []byte(tc.source), 0o644); err != nil {
				t.Fatalf("writing probe source: %v", err)
			}
			sites := findAdoptSpecCallSites(t, path)
			if len(sites) != 1 {
				t.Fatalf("%s: expected exactly one resolved call site, got %d: %v", tc.name, len(sites), sites)
			}
			if sites[0].funcName != "caller" {
				t.Errorf("expected the call site tagged with its enclosing func %q, got %q", "caller", sites[0].funcName)
			}
		})
	}

	t.Run("same_spelled_import_from_a_different_path_is_not_matched", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "probe.go")
		source := `package probe

import approve "github.com/example/other/approve"

func caller() { approve.AdoptSpec() }
`
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			t.Fatalf("writing probe source: %v", err)
		}
		sites := findAdoptSpecCallSites(t, path)
		if len(sites) != 0 {
			t.Fatalf("a same-named import from a DIFFERENT path must never match; got %d sites: %v", len(sites), sites)
		}
	})

	t.Run("blank_import_matches_nothing", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "probe.go")
		source := `package probe

import _ "github.com/mrmaxsteel/mindspec/internal/approve"

func caller() {}
`
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			t.Fatalf("writing probe source: %v", err)
		}
		sites := findAdoptSpecCallSites(t, path)
		if len(sites) != 0 {
			t.Fatalf("a blank import names no callable identifier; got %d sites: %v", len(sites), sites)
		}
	})
}
