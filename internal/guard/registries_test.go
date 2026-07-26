package guard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// registries_test.go pins the three registries' internal disciplines
// (spec 127 R5): every entry has a rationale AND a testable
// obligation, additions are red (a count assertion against the
// pinned bootstrap manifest — internal/lint owns the manifest-vs-
// live-scan reconciliation; this file owns each entry's OWN stated
// obligation).

func TestDestructiveGuidanceAllowlist_EveryEntryHasRationaleAndObligation(t *testing.T) {
	for _, e := range DestructiveGuidanceAllowlist {
		if e.Rationale == "" {
			t.Errorf("allowlist entry %s/%s has no rationale", e.File, e.Func)
		}
		if e.Obligation == "" {
			t.Errorf("allowlist entry %s/%s has no obligation — a rationale-only entry is on the same footing as unregistered (spec 127 R5d)", e.File, e.Func)
		}
	}
}

func TestOpaqueOperandRegistry_EveryEntryHasRationaleAndObligation(t *testing.T) {
	for _, e := range OpaqueOperandRegistry {
		if e.Rationale == "" {
			t.Errorf("opaque-operand entry %s/%s has no rationale", e.File, e.Func)
		}
		if e.Obligation == "" {
			t.Errorf("opaque-operand entry %s/%s has no obligation", e.File, e.Func)
		}
	}
}

func TestKnownSitesExemptionList_EveryEntryHasFamilyAndCount(t *testing.T) {
	for _, e := range KnownSitesExemptionList {
		if e.Surface == "" || e.Text == "" {
			t.Errorf("exemption entry %+v has an empty surface/text", e)
		}
		if e.Count < 1 {
			t.Errorf("exemption entry %s/%q has a non-positive count", e.Surface, e.Text)
		}
	}
}

// TestDestructiveGuidanceAllowlist_ContentRegeneration is fixture (γ)
// of the bootstrap discipline, applied to the allowlist: each entry's
// recorded Detail must still match its recorded Family — a hollow
// entry (one that never matched anything) or classifier drift is red.
func TestDestructiveGuidanceAllowlist_ContentRegeneration(t *testing.T) {
	for _, e := range DestructiveGuidanceAllowlist {
		matches := FindFloorMatches(e.Detail)
		found := false
		for _, m := range matches {
			if m.Family == e.Family {
				found = true
			}
		}
		if !found {
			t.Errorf("allowlist entry %s/%s recorded family %s, but the classifier no longer matches %q: got %+v", e.File, e.Func, e.Family, e.Detail, matches)
		}
	}
}

// TestOpaqueOperandRegistry_RuntimeInventoryAgainstFloor is the
// obligation named in this file's two readiness-signal-shaped entries
// (cmd/mindspec/bead_ready.go:57, internal/next/ready_gate.go:92):
// `readiness.Report.RecoveryCommands()` builds its result from every
// FAILING signal's `Recovery` field — a runtime-resolved slice no
// static fold can prove — so the obligation is a RUNTIME check over
// the actual template inventory: every `Recovery:` composite-literal
// field in internal/validate/readiness's own source, folded the same
// way the scan folds (Sprintf literal template, plain literal), must
// not match the classifier. This is real AST parsing of the real
// source tree (not a stub fabricating the value this test then
// asserts): a regression that adds a destructive Recovery template
// there REDs this test without needing a live bead to trigger it.
func TestOpaqueOperandRegistry_RuntimeInventoryAgainstFloor(t *testing.T) {
	root := repoRootFromGuardTestDir(t)
	dir := filepath.Join(root, "internal", "validate", "readiness")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	found := 0
	for _, ent := range entries {
		name := ent.Name()
		if ent.IsDir() || filepath.Ext(name) != ".go" {
			continue
		}
		if len(name) > 8 && name[len(name)-8:] == "_test.go" {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			kv, ok := n.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != "Recovery" {
				return true
			}
			tmpl, ok := recoveryTemplate(kv.Value)
			if !ok {
				t.Errorf("%s: a Recovery: field's value is not a literal or a fmt.Sprintf-with-literal-template — this test cannot prove it, which itself is the obligation failing (widen recoveryTemplate or hand-audit the new shape)", name)
				return true
			}
			found++
			if matches := FindFloorMatches(tmpl); len(matches) > 0 {
				t.Errorf("%s: a readiness Recovery template matches a destructive floor family: %q -> %+v", name, tmpl, matches)
			}
			return true
		})
	}
	if found == 0 {
		t.Fatal("found zero Recovery: fields — the obligation's own probe is broken (readiness.go's shape must have changed)")
	}
}

// recoveryTemplate folds a Recovery: field's value the same way the
// scan folds a command operand: a plain string literal, or
// fmt.Sprintf's literal template (substituted args are irrelevant to
// classification, spec 127 R5c).
func recoveryTemplate(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			s := e.Value
			if len(s) >= 2 {
				return s[1 : len(s)-1], true
			}
		}
	case *ast.CallExpr:
		sel, ok := e.Fun.(*ast.SelectorExpr)
		if !ok {
			return "", false
		}
		xid, ok := sel.X.(*ast.Ident)
		if !ok || xid.Name != "fmt" || sel.Sel.Name != "Sprintf" || len(e.Args) == 0 {
			return "", false
		}
		return recoveryTemplate(e.Args[0])
	}
	return "", false
}

// TestOpaqueOperandRegistry_RerunCallers is the second half of
// beadToSpecConflictFailure's `rerun` obligation: both real callers'
// actual rerun argument, checked directly against the classifier.
func TestOpaqueOperandRegistry_RerunCallers(t *testing.T) {
	realReruns := []string{
		"mindspec complete bead/mindspec-abcd.1",
		"mindspec impl approve 127-lifecycle-verb-trustworthiness",
	}
	for _, r := range realReruns {
		if matches := FindFloorMatches(r); len(matches) > 0 {
			t.Errorf("rerun invocation %q unexpectedly matches a destructive floor family: %+v", r, matches)
		}
	}
}

func repoRootFromGuardTestDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}
