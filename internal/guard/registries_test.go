package guard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/workspace/containment"
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

// TestKnownSitesExemptionList_EveryEntryHasFamilyAndCount pins EVERY
// field the test's own NAME promises (spec 127 bead-2 rework,
// O1-r2-10: the prior body never once referenced e.Family, so an
// entry with an empty Family passed despite the test's name).
func TestKnownSitesExemptionList_EveryEntryHasFamilyAndCount(t *testing.T) {
	registered := map[DestructiveFamily]bool{}
	for _, fam := range AllFamilies {
		registered[fam] = true
	}
	for _, e := range KnownSitesExemptionList {
		if e.Surface == "" || e.Text == "" {
			t.Errorf("exemption entry %+v has an empty surface/text", e)
		}
		if e.Count < 1 {
			t.Errorf("exemption entry %s/%q has a non-positive count", e.Surface, e.Text)
		}
		if e.Family == "" {
			t.Errorf("exemption entry %s/%q has an empty Family", e.Surface, e.Text)
		} else if !registered[e.Family] {
			t.Errorf("exemption entry %s/%q names Family %s, which is not in AllFamilies", e.Surface, e.Text, e.Family)
		}
	}
}

// TestKnownSitesExemptionList_EveryEntryHasRationaleAndObligation
// mirrors the allowlist/opaque-registry discipline checks above,
// extended to the exemption list now that it carries the same two
// fields (spec 127 bead-2 rework, RULING 6/G1-r2-3).
func TestKnownSitesExemptionList_EveryEntryHasRationaleAndObligation(t *testing.T) {
	for _, e := range KnownSitesExemptionList {
		if e.Rationale == "" {
			t.Errorf("exemption entry %s/%q has no rationale", e.Surface, e.Text)
		}
		if e.Obligation == "" {
			t.Errorf("exemption entry %s/%q has no obligation", e.Surface, e.Text)
		}
	}
}

// TestKnownSitesExemptionList_CountSentinel pins the live/total split
// derived, not hand-typed (spec 127 bead-2 rework, RULING 7): fails
// if len(KnownSitesExemptionList) or the live/seed-only split ever
// drifts from what the exported count vars themselves compute — which
// can only happen if this test is stale against a code change that
// also updated the vars, since both are derived from the same slice.
// Its real value is pinning the CURRENT correct numbers (ten live,
// fourteen total) so a reviewer sees them fail loudly if a future
// edit to the slice's structure (e.g. renaming the seed-only surface)
// silently changes what "live" means.
func TestKnownSitesExemptionList_CountSentinel(t *testing.T) {
	if got, want := len(KnownSitesExemptionList), 14; got != want {
		t.Errorf("len(KnownSitesExemptionList) = %d, want %d", got, want)
	}
	if got, want := KnownSitesExemptionListTotalCount, 14; got != want {
		t.Errorf("KnownSitesExemptionListTotalCount = %d, want %d", got, want)
	}
	if got, want := KnownSitesExemptionListLiveCount, 10; got != want {
		t.Errorf("KnownSitesExemptionListLiveCount = %d, want %d", got, want)
	}
}

// TestRegistryObligations_NamedTestsExist is spec 127 bead-2 rework's
// RULING 6 systemic fix (G1-r2-3/O1-r2-4/O2-r2-6/O3-r2-4): parses
// every Rationale and Obligation string in all three registries for a
// Test[A-Za-z0-9_]+-shaped identifier and asserts each one names a
// real test function declared SOMEWHERE in the repo's *_test.go
// files — not merely that the field is non-empty (the mechanism this
// spec's own dominant defect class keeps exploiting: an obligation
// naming a test that does not exist is, by ADR-0035's own rule, on
// the same footing as an unregistered site). This is what would have
// caught the two prior citations of a nonexistent
// TestDestructiveGuidanceAllowlist_ObligationsHold and the nonexistent
// "TestBeadCreateFailure"-shaped plan_test.go coverage.
func TestRegistryObligations_NamedTestsExist(t *testing.T) {
	root := repoRootFromGuardTestDir(t)
	declared := collectDeclaredTestNames(t, root)
	testNameRe := regexp.MustCompile(`Test[A-Za-z0-9_]+`)

	check := func(label, text string) {
		for _, name := range testNameRe.FindAllString(text, -1) {
			if !declared[name] {
				t.Errorf("%s names %s, which is not a test function declared anywhere in the repo's *_test.go files", label, name)
			}
		}
	}
	for _, e := range DestructiveGuidanceAllowlist {
		check("allowlist "+e.File+"/"+e.Func+" Rationale", e.Rationale)
		check("allowlist "+e.File+"/"+e.Func+" Obligation", e.Obligation)
	}
	for _, e := range OpaqueOperandRegistry {
		check("opaque-operand "+e.File+"/"+e.Func+" Rationale", e.Rationale)
		check("opaque-operand "+e.File+"/"+e.Func+" Obligation", e.Obligation)
	}
	for _, e := range KnownSitesExemptionList {
		check("exemption "+e.Surface+"/"+e.Text+" Rationale", e.Rationale)
		check("exemption "+e.Surface+"/"+e.Text+" Obligation", e.Obligation)
	}
}

// collectDeclaredTestNames walks every *_test.go file under cmd/ and
// internal/ (skipping testdata dirs, same as ratchet_universe_test.go's
// own walk in package lint) and returns the set of every top-level,
// receiver-less Test*-named function declared anywhere in the repo —
// obligations legitimately cite tests in OTHER packages (e.g.
// internal/approve/plan_test.go), so this walk is repo-wide, not
// package-local.
func collectDeclaredTestNames(t *testing.T, root string) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	fset := token.NewFileSet()
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
			if !strings.HasSuffix(d.Name(), "_test.go") {
				return nil
			}
			file, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return nil // a broken test file is not this test's concern
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if ok && fn.Recv == nil && strings.HasPrefix(fn.Name.Name, "Test") {
					names[fn.Name.Name] = true
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", top, err)
		}
	}
	if len(names) == 0 {
		t.Fatal("collected zero test names — this probe's own walk is broken")
	}
	return names
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
//
// Uses strconv.Unquote — not raw byte-slicing — so an escape sequence
// (readiness.go:219/:228 already carry `\"` inside their templates)
// decodes to the value the string ACTUALLY renders at runtime, not
// its undecoded source spelling (spec 127 bead-2 rework, O1-r2-5).
// strconv.Unquote failing is NOT treated as "no floor match" — it
// returns ok=false, which the caller (TestOpaqueOperandRegistry_
// RuntimeInventoryAgainstFloor) already turns into an explicit
// t.Errorf, never a silent pass.
func recoveryTemplate(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			if v, err := strconv.Unquote(e.Value); err == nil {
				return v, true
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

// TestRecoveryTemplate_DecodesEscapesLikeTheRealString pins O1-r2-5's
// fold: a Recovery: field whose literal carries an escape sequence
// must decode to the value that literal ACTUALLY renders at runtime
// — a destructive command hidden behind an escape must still be
// caught, not silently folded to its undecoded source spelling.
func TestRecoveryTemplate_DecodesEscapesLikeTheRealString(t *testing.T) {
	src := `package readiness

var x = Signal{Recovery: "git push --force\n"}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "synthetic.go", src, 0)
	if err != nil {
		t.Fatalf("parsing fixture: %v", err)
	}
	var value ast.Expr
	ast.Inspect(file, func(n ast.Node) bool {
		kv, ok := n.(*ast.KeyValueExpr)
		if !ok {
			return true
		}
		if id, ok := kv.Key.(*ast.Ident); ok && id.Name == "Recovery" {
			value = kv.Value
		}
		return true
	})
	if value == nil {
		t.Fatal("fixture setup error: no Recovery: field found")
	}
	tmpl, ok := recoveryTemplate(value)
	if !ok {
		t.Fatal("expected recoveryTemplate to fold the literal")
	}
	if matches := FindFloorMatches(tmpl); len(matches) == 0 {
		t.Fatalf("expected the decoded template %q to match FamilyGitPushForce, got no matches", tmpl)
	}
}

// TestOpaqueOperandRegistry_RerunCallers is the second half of
// beadToSpecConflictFailure's `rerun` obligation. Spec 127 bead-2
// rework, O3-r2-5: the prior version hardcoded two hand-written
// strings and asserted against those — a stub fabricating the value
// the AC then asserts, never reading mindspec_executor.go's real call
// sites at all, so a caller later rewritten to pass a destructive
// rerun string would leave this test green. This version parses
// internal/executor/mindspec_executor.go's real AST, finds EVERY call
// to beadToSpecConflictFailure, folds argument index 3 (`rerun`) the
// same way the scan folds a command operand (recoveryTemplate, above
// — a literal or an fmt.Sprintf literal template), and asserts no
// floor match — the same shape TestOpaqueOperandRegistry_
// RuntimeInventoryAgainstFloor already uses in this file for the
// readiness-signal catalog.
func TestOpaqueOperandRegistry_RerunCallers(t *testing.T) {
	root := repoRootFromGuardTestDir(t)
	path := filepath.Join(root, "internal", "executor", "mindspec_executor.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	found := 0
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		id, ok := call.Fun.(*ast.Ident)
		if !ok || id.Name != "beadToSpecConflictFailure" {
			return true
		}
		if len(call.Args) <= 3 {
			t.Errorf("a call to beadToSpecConflictFailure has %d args, expected at least 4 (rerun is index 3) — this test's own probe is broken", len(call.Args))
			return true
		}
		tmpl, ok := recoveryTemplate(call.Args[3])
		if !ok {
			t.Errorf("beadToSpecConflictFailure call at %s: the rerun argument (index 3) is not a literal or fmt.Sprintf-with-literal-template — this test cannot prove it, which itself is the obligation failing", fset.Position(call.Pos()))
			return true
		}
		found++
		if matches := FindFloorMatches(tmpl); len(matches) > 0 {
			t.Errorf("beadToSpecConflictFailure call at %s: rerun template %q matches a destructive floor family: %+v", fset.Position(call.Pos()), tmpl, matches)
		}
		return true
	})
	if found == 0 {
		t.Fatal("found zero calls to beadToSpecConflictFailure — this test's own probe is broken (the call sites must have changed shape)")
	}
}

// TestOpaqueOperandRegistry_EmitCdWorktreePathsAgainstFloor is the
// named obligation for the three containment.EmitCd-bare-call entries
// added in the bead-2 rework (beadToSpecConflictFailure,
// directMergeConflictFailure, checkCWDWithCache): runs the REAL
// containment.EmitCd over representative worktree/root PATH shapes
// (never free-form/adversarial text — that broader, false "for any
// target" claim was corrected in destructive_guidance_test.go's
// former EmitCd trust boundary, spec 127 bead-2 rework O2-r2-9) and
// asserts the classifier finds no floor match.
func TestOpaqueOperandRegistry_EmitCdWorktreePathsAgainstFloor(t *testing.T) {
	paths := []string{
		"/repo",
		"/repo/.worktrees/worktree-spec-127-lifecycle-verb-trustworthiness",
		"/repo/.worktrees/worktree-mindspec-2vtk.2",
		"../other-worktree",
		"/tmp/mindspec-work",
	}
	for _, p := range paths {
		rendered := containment.EmitCd(p)
		if matches := FindFloorMatches(rendered); len(matches) > 0 {
			t.Errorf("containment.EmitCd(%q) = %q unexpectedly matches a destructive floor family: %+v", p, rendered, matches)
		}
	}
}

// TestOpaqueOperandRegistry_RecoveryCommandTemplatesAgainstFloor is
// the named obligation for Orphan.RecoveryCommand/
// StaleOpenBead.RecoveryCommand's own `"mindspec complete " +
// idrender.Bead(id)` return shape (bead-2 rework): checks the
// rendered form over representative PATTERN-VALID bead-ID strings
// (idvalidate.BeadID's charset — lowercase alnum, hyphens, dots only,
// no whitespace) — the literal prefix alone is already off-floor
// (mindspec, not git/bd/rm), and a pattern-valid id can never itself
// spell a multi-token destructive command (no whitespace to separate
// tokens). Package guard cannot import internal/lifecycle (a real
// import cycle — internal/lifecycle/gitquery.go imports internal/
// guard), so this checks the rendered TEXT SHAPE directly rather than
// calling Orphan.RecoveryCommand()/StaleOpenBead.RecoveryCommand()
// themselves.
func TestOpaqueOperandRegistry_RecoveryCommandTemplatesAgainstFloor(t *testing.T) {
	ids := []string{
		"mindspec-abcd.1",
		"mindspec-ab01.2.3",
		"proj-slug-123",
	}
	for _, id := range ids {
		rendered := "mindspec complete " + id
		if matches := FindFloorMatches(rendered); len(matches) > 0 {
			t.Errorf("%q unexpectedly matches a destructive floor family: %+v", rendered, matches)
		}
	}
}

// TestOpaqueOperandRegistry_PanelRecreateRerunAgainstFloor is the
// named obligation for cmd/mindspec/panel.go's tallyExitActionNonBead
// (bead-2 rework): representative renderings of its own `"re-run the
// panel: " + "mindspec panel create %s --round <N+1> --spec <id>"`
// template, with and without the optional `--target`/`--gate`
// fragments, checked against the classifier.
func TestOpaqueOperandRegistry_PanelRecreateRerunAgainstFloor(t *testing.T) {
	renderings := []string{
		`re-run the panel: mindspec panel create bead-127.2 --round <N+1> --spec <id>`,
		`re-run the panel: mindspec panel create bead-127.2 --round <N+1> --spec <id> --target main`,
		`re-run the panel: mindspec panel create bead-127.2 --round <N+1> --spec <id> --target main --gate impl`,
	}
	for _, r := range renderings {
		if matches := FindFloorMatches(r); len(matches) > 0 {
			t.Errorf("%q unexpectedly matches a destructive floor family: %+v", r, matches)
		}
	}
}

// mindspecVerbTemplateSites is the (repo-relative file, enclosing
// func/method) pair for every opaque-operand registry entry added in
// bead-2 rework round 2 (O2c-1's fullyLiteral fix: propagating
// foldExpr's third return value through fmt.Sprintf, localBinds, and
// pkgConstFolder.Get newly governs ~38 real command-position Sprintf
// sites the rework-round-1 tree had silently exempted). Every named
// site's own command-position value is a `fmt.Sprintf` call whose
// literal template leads with the verb "mindspec" — never git/bd/rm,
// this floor's own closed program set (classifier.go) — with an
// ID-typed or error-typed value substituted in, either via
// idrender.Bead/idrender.Spec's dual-safe render (a validated ID
// renders byte-identically; a malformed one is forced through
// strconv.Quote, which the classifier's own quote-aware tokenizer
// (bead-2 rework round 2, G1-2/O1-9) then reads as a single, inert
// token) or, for a handful of sites, a raw specID/epicID/parentID
// that the enclosing function itself idvalidate's at entry before any
// of these Sprintf calls run — same single-token charset guarantee,
// no render step.
var mindspecVerbTemplateSites = []struct{ file, fn string }{
	{"internal/approve/adopt.go", "AdoptSpec"},
	{"internal/approve/adopt.go", "adoptCurrentBranchPresentRefusal"},
	{"internal/approve/adopt.go", "adoptStaleBranchPresentRefusal"},
	{"internal/approve/adopt.go", "adoptOrphanPresentRefusal"},
	{"internal/approve/adopt.go", "adoptEvidenceErrorRefusal"},
	{"internal/approve/adopt.go", "adoptRefusalFailure"},
	{"cmd/mindspec/bead_clarify.go", "beadClarifyCmd"},
	{"cmd/mindspec/panel.go", "panelCreateCmd"},
	{"cmd/mindspec/panel.go", "findPanelRegistration"},
	{"cmd/mindspec/panel.go", "tallyExitAction"},
	{"cmd/mindspec/reattest.go", "runReattest"},
	{"cmd/mindspec/reattest.go", "reattestRefusalFailure"},
	{"cmd/mindspec/release.go", "runRelease"},
	{"cmd/mindspec/repair.go", "repairSpecTitleRunE"},
	{"cmd/mindspec/repair.go", "repairPhaseRunE"},
	{"internal/approve/impl.go", "ApproveImpl"},
	{"internal/approve/impl.go", "runOrphanObligationGate"},
	{"internal/approve/impl.go", "runWorktreeEnumerationLeg"},
	{"internal/approve/impl.go", "implObligationRefusal"},
	{"internal/approve/plan.go", "resolvePlanApprovePreflight"},
	{"internal/approve/plan.go", "resolveTargetEpic"},
	{"internal/approve/plan.go", "ApprovePlan"},
	{"internal/approve/plan.go", "planValidationFailure"},
	{"internal/approve/plan.go", "beadCreateFailure"},
	{"internal/approve/plan.go", "queryExistingChildren"},
	{"internal/approve/plan.go", "checkExistingBeadsSafety"},
	{"internal/complete/complete.go", "Run"},
	{"internal/complete/complete.go", "adrDivergenceFailure"},
	{"internal/complete/complete.go", "attestedRestoreFailure"},
	{"internal/complete/panel_advisory.go", "panelGate"},
	{"internal/complete/panel_advisory.go", "reconcilePendingRefutations"},
	{"internal/executor/layout_guard.go", "mergeLayoutRegressionFailure"},
	{"internal/executor/mindspec_executor.go", "MindspecExecutor.CompleteBead"},
	{"internal/executor/mindspec_executor.go", "MindspecExecutor.FinalizeEpic"},
	{"internal/lifecycle/finalize_orphans.go", "FinalizeOrphan.RecoveryCommand"},
	{"internal/next/guard.go", "DirtyTreeFailure"},
	{"internal/next/guard.go", "ClaimFailure"},
	{"internal/next/guard.go", "WorktreeSetupFailure"},
}

// mindspecVerbRealisticValues is the battery substituted into every
// %s/%v/%q slot of every template collected from the sites above.
//
// This is deliberately a REALISTIC-VALUE battery, not a maximal
// adversarial fuzz: the 30 sites above substitute several genuinely
// different kinds of value — an idrender.Bead/idrender.Spec-rendered
// ID (validated-ID-or-quoted, dual-safe by construction), a raw
// specID/epicID/parentID the enclosing function idvalidate's at entry
// (idvalidate's grammar admits no whitespace or tokenizer-separator
// character at all, so a value that PASSES it is provably always
// exactly one token — see idvalidate/ids.go's specIDPattern/
// beadIDPattern), a git ref/branch/SHA, a filesystem path, or an
// operator-typed panel `slug` whose OWN validator (validatePanelSlug)
// rejects path separators and control bytes but NOT whitespace or
// other printable content. Collapsing all of these into one maximal
// battery (tried during this bead's rework, and reverted) produces
// false failures for the validated-ID class (testing multi-token
// values idvalidate provably never lets through) while still not
// proving anything stronger for the slug class (a `panel create`
// operator names their own slug and is the one who would see any
// resulting oddity in their own later recovery hint — a distinct,
// narrower risk than an externally-attacker-supplied string). This
// battery instead proves the thing that actually matters uniformly:
// every template renders safely under NORMAL, expected content, and a
// later edit that changes a template's literal wording in a way that
// newly collides with a floor family REDs immediately. The slug-typed
// sites' entries below additionally disclose, in their own Rationale,
// the residual gap this battery does not close — narrowing the claim
// rather than overclaiming it, per this spec's own standing
// requirement.
var mindspecVerbRealisticValues = []string{
	"mindspec-abcd.1", "mindspec-9cyu.2.3", "mindspec-mol-015",
	"127-lifecycle-verb-trustworthiness", "092-req19-metadata-ban",
	"review-round-2", "final-review-panel",
	"origin/main", "bead/mindspec-abcd.1", "spec/127-lifecycle-verb-trustworthiness",
}

// sprintfVerbRe matches one printf verb ("%s", "%q", "%v", "%d", ...)
// — used only to count how many adversarial values a given template
// needs, never to distinguish verb TYPES: every value substituted
// below is a plain string, which fmt accepts cleanly for %s/%v/%q and
// renders as an inert `%!verb(string=...)` diagnostic wrapper for any
// other verb — itself still opaque to the classifier (its own `(`/`)`
// are tokenizer separators, so the wrapped value's words never fuse
// with "string=" into a single "git"/"bd"/"rm" token).
var sprintfVerbRe = regexp.MustCompile(`%[a-zA-Z]`)

// fillTemplate substitutes value at every printf verb in tmpl.
func fillTemplate(tmpl, value string) string {
	n := len(sprintfVerbRe.FindAllString(tmpl, -1))
	if n == 0 {
		return tmpl
	}
	args := make([]interface{}, n)
	for i := range args {
		args[i] = value
	}
	return fmt.Sprintf(tmpl, args...)
}

// sprintfTemplatesInFunc parses relFile fresh (never trusting a
// cached/prior AST — the whole point of this obligation is to catch
// drift in the REAL, current source) and returns every fmt.Sprintf
// literal-template string found anywhere inside the named top-level
// function/method's body OR package-level var's initializer (the
// cmd/mindspec cobra-command convention: `var xCmd = &cobra.Command{
// RunE: func(...) {...} }` — a RunE closure, not a FuncDecl, so its
// own enclosing name is the VAR's, matching internal/lint's
// enclosingFunc's own two cases). funcName is "Name" for a plain
// function or var, "Recv.Name" for a method.
func sprintfTemplatesInFunc(t *testing.T, root, relFile, funcName string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath.Join(root, relFile), nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", relFile, err)
	}
	var target ast.Node
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			name := d.Name.Name
			if d.Recv != nil && len(d.Recv.List) > 0 {
				name = recvTypeNameForObligation(d.Recv.List[0].Type) + "." + name
			}
			if name == funcName {
				target = d
			}
		case *ast.GenDecl:
			if d.Tok != token.VAR {
				continue
			}
			for _, spec := range d.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, nm := range vs.Names {
					if nm.Name == funcName && i < len(vs.Values) {
						target = vs.Values[i]
					}
				}
			}
		}
	}
	if target == nil {
		t.Fatalf("function/method/var %q not found in %s — this obligation's own probe is broken (the site moved or was renamed; fix mindspecVerbTemplateSites)", funcName, relFile)
	}
	var templates []string
	ast.Inspect(target, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		xid, ok := sel.X.(*ast.Ident)
		if !ok || xid.Name != "fmt" || sel.Sel.Name != "Sprintf" || len(call.Args) == 0 {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		v, uerr := strconv.Unquote(lit.Value)
		if uerr != nil {
			return true
		}
		templates = append(templates, v)
		return true
	})
	return templates
}

// recvTypeNameForObligation mirrors internal/lint's recvTypeName —
// duplicated rather than imported (package guard cannot import
// internal/lint's test-only helpers across the package boundary).
func recvTypeNameForObligation(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return recvTypeNameForObligation(e.X)
	}
	return ""
}

// TestOpaqueOperandRegistry_MindspecVerbTemplatesAgainstFloor is the
// Obligation named by every "mindspec <verb>..."-templated entry added
// in this bead's SECOND rework round (spec 127, O2c-1). Rather than
// hand-copy each site's template as a literal — exactly the kind of
// carried-over, driftable claim this spec exists to prevent, and the
// defect class RawMergeFence's own doc comment (internal/panel/
// gate.go) already lived through once — this test RE-PARSES the real
// source at every named site, extracts every fmt.Sprintf template
// found in that function/method's body, and asserts the classifier
// finds NO floor match after substituting a realistic-value battery
// (mindspecVerbRealisticValues, above — deliberately NOT a maximal
// adversarial fuzz; see that variable's own doc comment for why) at
// every printf verb. A template edited later is re-checked
// automatically on every run; a named site that moves or is renamed
// fails loudly (sprintfTemplatesInFunc's own "not found" fatal) rather
// than silently passing zero templates.
func TestOpaqueOperandRegistry_MindspecVerbTemplatesAgainstFloor(t *testing.T) {
	root := repoRootFromGuardTestDir(t)
	// knownAllowlistTemplates skips templates that legitimately DO
	// match a floor family — plan.go's beadCreateFailure and
	// checkExistingBeadsSafety each contain BOTH an opaque
	// "mindspec ..." template (this obligation's actual concern) AND a
	// SEPARATE, already-governed "bd delete %s --force" Sprintf
	// (DestructiveGuidanceAllowlist's own first two entries, above,
	// each with their own ContentRegeneration obligation) in the same
	// function body. This obligation is scoped to the OPAQUE templates
	// only; asserting no-match against a template the allowlist
	// mechanism deliberately DOES match would contradict that entry.
	knownAllowlistTemplates := map[string]bool{
		"bd delete %s --force": true,
		// adopt.go's adoptStaleBranchPresentRefusal feeds this exact
		// template to guard.NewDestructiveCommand — it is SUPPOSED to
		// match FamilyGitBranchDeleteForce (that is the whole point of
		// the constructor call); the resulting DestructiveCommand is
		// provenance-exempt (R5(b)), never registered here. This
		// obligation covers only the function's OTHER, genuinely opaque
		// templates (arg1/arg3's fallback), same scoping as the
		// bd-delete exclusion above.
		"git branch -D %s": true,
	}
	totalTemplates := 0
	for _, site := range mindspecVerbTemplateSites {
		templates := sprintfTemplatesInFunc(t, root, site.file, site.fn)
		if len(templates) == 0 {
			t.Errorf("%s/%s: found zero fmt.Sprintf templates — this obligation's own extraction is broken for this site's shape (fix sprintfTemplatesInFunc or mindspecVerbTemplateSites)", site.file, site.fn)
		}
		for _, tmpl := range templates {
			if knownAllowlistTemplates[tmpl] {
				continue
			}
			totalTemplates++
			for _, v := range mindspecVerbRealisticValues {
				rendered := fillTemplate(tmpl, v)
				if matches := FindFloorMatches(rendered); len(matches) > 0 {
					t.Errorf("%s/%s template %q with adversarial value %q renders %q, which matches the destructive floor: %+v", site.file, site.fn, tmpl, v, rendered, matches)
				}
			}
		}
	}
	if totalTemplates == 0 {
		t.Fatal("collected zero fmt.Sprintf templates across every named site — this obligation's own probe is broken")
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
