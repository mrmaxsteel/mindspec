package lifecycle

// Spec 127 R2/AC-3 — the outcome oracle: ground truth for an orphan
// hint's CLAIMED outcome, re-derived through pinned, non-mutating,
// RAW git probes — never by calling the shared work-destruction
// predicate, the hint derivation, or the exported landed/net-effect
// classifiers this file's own independence scan (below) forbids.
// Agreement against any of those would be guaranteed by construction,
// proving nothing; this file computes its verdicts from first
// principles, using only `git merge-base --is-ancestor`, `git rev-list`,
// `git for-each-ref`, `git diff --name-status`, and a test-side
// `git merge-tree --write-tree` (P2-plan-2's pinned probe set).
//
// CONFINEMENT RULE: every ground-truth helper this oracle consumes is
// DEFINED IN THIS FILE. A sibling test file (outcome_oracle_ac3_test.go)
// legitimately calls the REAL derivation (lifecycle.DeriveOrphanHint /
// EvaluateOrphanHint) to drive the AC-3 table — that is not an
// indirection around this file's fence, because this file's own
// oracleProbe/oracleJudge* functions never call back into it.
//
// INDEPENDENCE, mechanized (P2-plan-2, two legs, both red-on-violation):
//  1. TestOutcomeOracle_IndependenceCallScan (below) parses THIS FILE's
//     own source and fails on any CallExpr or selector/identifier
//     reference to the named forbidden set: EvaluateWorkDestruction (the
//     gitutil symbol AND the same-package lifecycle wrapper var/func),
//     DeriveOrphanHint, EvaluateOrphanHint, EvaluateOrphanHintAgainstMain,
//     FindLandedMerge, NetEffectLanded, ContentSubsumedOutcome,
//     PreviewDeletedPaths.
//  2. TestOutcomeOracle_NoGitutilImport asserts this file has no
//     "internal/gitutil" import at all — meaningful for the
//     cross-package classifiers (the call-scan leg above already covers
//     the same-package wrapper/derivation, which needs no import to
//     name).
import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// --- raw, non-mutating ground-truth probes (no gitutil, no bd) --------

func oracleGitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := oracleGitRunAllowFail(dir, args...)
	if err != nil {
		t.Fatalf("git -C %s %v: %v\n%s", dir, args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func oracleGitRunAllowFail(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=oracle", "GIT_AUTHOR_EMAIL=oracle@test.invalid",
		"GIT_COMMITTER_NAME=oracle", "GIT_COMMITTER_EMAIL=oracle@test.invalid",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
	)
	return cmd.CombinedOutput()
}

func oracleWriteFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func oracleInitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	oracleGitRun(t, dir, "init", "-q", "-b", "main")
	oracleWriteFile(t, dir, "README.md", "root\n")
	oracleGitRun(t, dir, "add", ".")
	oracleGitRun(t, dir, "commit", "-q", "-m", "root")
	return dir
}

func oracleCommit(t *testing.T, dir, msg string) {
	t.Helper()
	oracleGitRun(t, dir, "add", "-A")
	out, err := oracleGitRunAllowFail(dir, "commit", "-q", "-m", msg)
	if err != nil && !strings.Contains(string(out), "nothing to commit") {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
}

// oracleIsAncestor is probe (1): `git merge-base --is-ancestor A B`.
// Exit 0 -> true, exit 1 -> false, anything else is a test-fatal probe
// failure (never guessed).
func oracleIsAncestor(t *testing.T, dir, ancestor, descendant string) bool {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "merge-base", "--is-ancestor", ancestor, descendant)
	err := cmd.Run()
	if err == nil {
		return true
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false
	}
	t.Fatalf("git merge-base --is-ancestor %s %s: %v", ancestor, descendant, err)
	return false
}

// oracleUnreachableCommits is probe (2): `git rev-list <ref> --not
// <excl...>` — every commit reachable from ref but from none of excl.
// A non-empty result means ref carries commits not guaranteed reachable
// from any of excl (the preserve-first clause's own ground truth).
func oracleUnreachableCommits(t *testing.T, dir, ref string, excl ...string) []string {
	t.Helper()
	args := append([]string{"rev-list", ref, "--not"}, excl...)
	out := oracleGitRun(t, dir, args...)
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// oracleBranchExists is probe (3): `git for-each-ref refs/heads/<name>`.
func oracleBranchExists(t *testing.T, dir, name string) bool {
	t.Helper()
	out := oracleGitRun(t, dir, "for-each-ref", "--format=%(refname)", "refs/heads/"+name)
	return out != ""
}

// oraclePreviewDeletedPaths is probes (4)+(5): a test-side, non-mutating
// `git merge-tree --write-tree target branch` (modern two-ref form —
// git >= 2.38) computes the merge preview's resulting tree without
// touching the real index/worktree/refs; `git diff --name-status
// <previewTree> <target>` then names paths present in target's OWN tip
// tree but ABSENT from the preview — i.e. paths the merge would delete.
// On a real content conflict `merge-tree` exits non-zero but still
// prints a resulting tree OID on its first output line (git's own
// documented conflict-tree behavior), which this probe uses exactly as
// the real preview would have to.
func oraclePreviewDeletedPaths(t *testing.T, dir, target, branch string) []string {
	t.Helper()
	out, _ := oracleGitRunAllowFail(dir, "merge-tree", "--write-tree", target, branch)
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("git merge-tree --write-tree %s %s produced no tree OID: %s", target, branch, out)
	}
	previewTree := strings.TrimSpace(lines[0])
	diffOut := oracleGitRun(t, dir, "diff", "--name-status", previewTree, target)
	var deleted []string
	for _, line := range strings.Split(diffOut, "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 2)
		// Diffing previewTree -> target: an "A" (added in target
		// relative to preview) is a path the preview LACKS that target
		// HAS — exactly the deletion-from-the-merge's-perspective set.
		if len(fields) == 2 && strings.HasPrefix(fields[0], "A") {
			deleted = append(deleted, fields[1])
		}
	}
	return deleted
}

// oracleMergeBaseExists reports whether branch and target share a
// merge-base at all (a non-zero `git merge-base` exit means unrelated
// histories — one of the two whole-repository preconditions
// EvaluateWorkDestruction's own doc comment names as a guaranteed
// DestructionEvidenceError).
func oracleMergeBaseExists(t *testing.T, dir, branch, target string) bool {
	t.Helper()
	_, err := oracleGitRunAllowFail(dir, "merge-base", branch, target)
	return err == nil
}

// --- byte-match template discipline (AC-3's own well-formedness leg) --

// shellMetacharacters is the closed set whose presence anywhere in a
// recovery line fails WELL-FORMED — a compound line is never one
// command (ADR-0035's own "one command per line" convention, sharpened
// here to a mechanical check).
var shellMetacharacters = []string{"&&", ";", "|", "`", "$("}

// oracleTemplate is one pinned, whole-line template: re is matched
// against the ENTIRE line (anchored), so any unconsumed residue fails.
type oracleTemplate struct {
	name string
	re   *regexp.Regexp
}

// oracleTemplates is the closed, pinned set of every line shape
// DeriveOrphanHint can ever produce (orphan_hints.go's own rendered
// shapes, transcribed by hand — never generated by calling that file).
// Parameterized only by validated identifiers (a bead ID matching
// idvalidate.BeadID's charset — lowercase alnum, hyphen, dot) and by
// ref names (matched permissively here since a ref name's own charset
// is git's, not this scan's concern — the ESCAPING discipline is
// orphan_hints_test.go's job, this is WELL-FORMEDNESS only).
var oracleTemplates = []oracleTemplate{
	{"mindspec complete", regexp.MustCompile(`^mindspec complete [A-Za-z0-9._-]+$`)},
	{"git branch -D", regexp.MustCompile(`^git branch -D \S+$`)},
	{"git tag preserve", regexp.MustCompile(`^git tag preserve/[A-Za-z0-9._-]+ \S+   \(preserve .+\)$`)},
	{"mindspec impl adopt", regexp.MustCompile(`^mindspec impl adopt [A-Za-z0-9._-]+ --reason "<.+>"(   \(re-run once .+\))?$`)},
	{"git diff inspection", regexp.MustCompile(`^git diff \S+ \S+   \(.+\)$`)},
}

// oracleWellFormed reports whether line matches EXACTLY ONE of the
// pinned templates in full (anchored, whole-line) AND contains none of
// shellMetacharacters. A line resolving to a REAL, different command
// (e.g. `mindspec doctor --fix`) but not matching any PINNED template
// still fails — resolution alone is existence-shaped, never sufficient
// (B-r4-2/D-r4-5).
func oracleWellFormed(line string) (matchedTemplate string, ok bool) {
	for _, mc := range shellMetacharacters {
		if strings.Contains(line, mc) {
			return "", false
		}
	}
	for _, tmpl := range oracleTemplates {
		if tmpl.re.MatchString(line) {
			return tmpl.name, true
		}
	}
	return "", false
}

func TestOutcomeOracle_WellFormedTemplates(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"mindspec complete bead-1", true},
		{"mindspec complete mindspec-abcd.1", true},
		{"git branch -D bead/bead-1", true},
		{`git tag preserve/bead-1 bead/bead-1   (preserve bead-1's commits before deleting — its content is not guaranteed reachable from main)`, true},
		{`mindspec impl adopt 042-test --reason "<why bead-1's content already reached main outside the lifecycle>"`, true},
		{`mindspec impl adopt 042-test --reason "<why>"   (re-run once bead-1's branch state is resolved)`, true},
		{"git diff spec/foo bead/foo   (inspect before doing anything else)", true},
	}
	for _, c := range cases {
		if _, ok := oracleWellFormed(c.line); ok != c.want {
			t.Errorf("oracleWellFormed(%q) = %v, want %v", c.line, ok, c.want)
		}
	}
}

// TestOutcomeOracle_InjectionFixtures pins the fail-closed claim with
// TWO fixtures (AC-3's preamble): a compound line, and a
// resolvable-but-wrong mutating leaf. Neither may pass WELL-FORMED.
func TestOutcomeOracle_InjectionFixtures(t *testing.T) {
	t.Run("compound_line", func(t *testing.T) {
		line := "mindspec complete bead-1 && git push --force"
		if _, ok := oracleWellFormed(line); ok {
			t.Errorf("a compound line must fail WELL-FORMED, got ok=true for %q", line)
		}
	})
	t.Run("resolvable_but_wrong_mutating_leaf", func(t *testing.T) {
		// mindspec doctor --fix is a REAL, resolvable command — but it
		// is not one of THIS derivation's pinned templates. Resolution
		// alone (it names a live command) must never substitute for the
		// byte-match proof that the line is ONLY that invocation.
		line := "mindspec doctor --fix"
		if _, ok := oracleWellFormed(line); ok {
			t.Errorf("a resolvable-but-wrong mutating leaf must fail WELL-FORMED, got ok=true for %q", line)
		}
	})
}

// --- independence mechanization (P2-plan-2) ----------------------------

// outcomeOracleForbiddenSymbols is the named forbidden set: this
// oracle's ground truth must never be computed by calling any of these.
var outcomeOracleForbiddenSymbols = map[string]bool{
	"EvaluateWorkDestruction":       true, // gitutil symbol AND the lifecycle wrapper (same identifier both places)
	"evaluateWorkDestructionFn":     true, // the lifecycle package's own unexported seam var
	"DeriveOrphanHint":              true,
	"EvaluateOrphanHint":            true,
	"EvaluateOrphanHintAgainstMain": true,
	"FindLandedMerge":               true,
	"NetEffectLanded":               true,
	"ContentSubsumedOutcome":        true,
	"PreviewDeletedPaths":           true,
}

// thisOracleFileSource resolves outcome_oracle_test.go's own absolute
// path via its package directory (found the same way
// orphan_hint_emitters_test.go's repoRootFromLifecycleTestDir does —
// this file lives in internal/lifecycle itself, so no repo-root walk is
// needed at all, just the CWD `go test` already sets).
func thisOracleFileSource(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(wd, "outcome_oracle_test.go")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("could not locate this oracle's own source file at %s: %v", path, err)
	}
	return path
}

// TestOutcomeOracle_IndependenceCallScan is independence leg (1): parses
// EVERY .go file in this package directory (bead-4 fix round 1,
// BLOCKING-2 — see scanForForbiddenRefsWithClosure's own doc comment)
// and fails the moment any same-package call reachable from
// outcome_oracle_test.go's own non-exempt declarations resolves to
// outcomeOracleForbiddenSymbols, wherever in the package that reference
// actually lives. This test itself is necessarily EXEMPT from its own
// ban (it must name the forbidden strings to check for them) — enforced
// by skipping this function's own FuncDecl body during the walk,
// verified never to hide a real violation by TWO defeat tests below: one
// planting a violation directly inside the scanned file (sensitivity),
// one planting it behind an innocuously-named call into a SIBLING file
// (scope — the class this fix round closes).
func TestOutcomeOracle_IndependenceCallScan(t *testing.T) {
	rootPath := thisOracleFileSource(t)
	violations := scanForForbiddenRefsWithClosure(t, filepath.Dir(rootPath), rootPath,
		"TestOutcomeOracle_IndependenceCallScan",
		"TestOutcomeOracle_IndependenceCallScan_DefeatPlantedViolation",
		"TestOutcomeOracle_IndependenceCallScan_DefeatSiblingIndirection",
	)
	for _, v := range violations {
		t.Errorf("outcome_oracle_test.go independence violation (package-wide helper-closure scan): %s", v)
	}
}

// scanForForbiddenRefs parses absPath and returns one message per
// reference (CallExpr callee, SelectorExpr, or bare Ident) to
// outcomeOracleForbiddenSymbols, EXCLUDING any reference textually
// inside a FuncDecl named in exemptFuncs (this scan's own necessarily-
// self-naming test functions).
func scanForForbiddenRefs(t *testing.T, absPath string, exemptFuncs ...string) []string {
	t.Helper()
	exempt := map[string]bool{}
	for _, f := range exemptFuncs {
		exempt[f] = true
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, absPath, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", absPath, err)
	}

	var exemptRanges []struct{ start, end token.Pos }
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if ok && exempt[fd.Name.Name] {
			exemptRanges = append(exemptRanges, struct{ start, end token.Pos }{fd.Pos(), fd.End()})
		}
	}
	inExempt := func(pos token.Pos) bool {
		for _, r := range exemptRanges {
			if pos >= r.start && pos <= r.end {
				return true
			}
		}
		return false
	}

	var violations []string
	report := func(pos token.Pos, name string) {
		if inExempt(pos) {
			return
		}
		violations = append(violations, fset.Position(pos).String()+": reference to forbidden symbol "+name)
	}

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.SelectorExpr:
			if outcomeOracleForbiddenSymbols[node.Sel.Name] {
				report(node.Pos(), node.Sel.Name)
				return false
			}
		case *ast.Ident:
			// Bare identifiers ONLY (a same-package call/reference, or a
			// dot-imported one) — SelectorExpr's own Sel is already
			// handled above and must not be double-counted, so skip an
			// Ident that is itself a SelectorExpr's Sel by checking
			// nothing further here: ast.Inspect visits the SelectorExpr
			// node (handled, returns false to stop descending into its
			// children, including Sel) before it would ever visit Sel as
			// a standalone Ident, so no double-count occurs.
			if outcomeOracleForbiddenSymbols[node.Name] {
				report(node.Pos(), node.Name)
			}
		}
		return true
	})
	return violations
}

// TestOutcomeOracle_IndependenceCallScan_DefeatPlantedViolation is the
// mechanism's own defeat test (the house lesson: an anti-drift scan
// that silently exempts more than it should is worse than none — spec
// 127 bead 5's own decoy-shaped bug). It plants a reference to a
// forbidden symbol in a SYNTHETIC fixture file (never this real file)
// OUTSIDE any exempt function name, and asserts the scan finds it.
func TestOutcomeOracle_IndependenceCallScan_DefeatPlantedViolation(t *testing.T) {
	src := `package lifecycle

func plantedViolation(workdir, branch, target string) {
	_, _, _ = EvaluateWorkDestruction(workdir, branch, target)
}
`
	dir := t.TempDir()
	path := filepath.Join(dir, "planted.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	violations := scanForForbiddenRefs(t, path, "TestOutcomeOracle_IndependenceCallScan", "TestOutcomeOracle_IndependenceCallScan_DefeatPlantedViolation")
	if len(violations) == 0 {
		t.Fatal("planted violation was not detected — the independence scan is hollow")
	}
}

// --- helper-closure confinement (bead-4 fix round 1, BLOCKING-2) -------

// packageFuncTable maps every top-level, receiver-less function
// declaration's name to its own *ast.FuncDecl; packageVarTable maps
// every top-level `var name = <expr>` declaration's name to its RHS
// expression — both built across EVERY .go file (production and test)
// in one flat package directory, with one shared *token.FileSet so a
// violation found through a cross-file call still reports its true
// file:line.
type packageFuncTable map[string]*ast.FuncDecl
type packageVarTable map[string]ast.Expr

// parsePackageGoFiles parses every *.go file directly inside dir (no
// recursion — this is one flat package directory) with a single shared
// token.FileSet, so positions resolved from ANY of the returned files
// report the correct filename.
func parsePackageGoFiles(t *testing.T, dir string) (*token.FileSet, []*ast.File) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading package dir %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Fatalf("parsing %s: %v", path, perr)
		}
		files = append(files, f)
	}
	return fset, files
}

// buildPackageTables walks every top-level declaration in files and
// records every receiver-less function and every single-value `var`
// assignment, keyed by name — the same-package call/value-indirection
// surface scanForForbiddenRefsWithClosure's walk below can follow.
func buildPackageTables(files []*ast.File) (packageFuncTable, packageVarTable) {
	funcs := packageFuncTable{}
	vars := packageVarTable{}
	for _, f := range files {
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil {
					funcs[d.Name.Name] = d
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
					for i, name := range vs.Names {
						if i < len(vs.Values) {
							vars[name.Name] = vs.Values[i]
						}
					}
				}
			}
		}
	}
	return funcs, vars
}

// scanForForbiddenRefsWithClosure is scanForForbiddenRefs's package-wide
// upgrade (bead-4 fix round 1, BLOCKING-2 / G1-2). The plain scan above
// only ever sees a reference textually inside the ONE file it is
// handed — provably insufficient: a helper function (or a
// function-valued package var) defined in ANY sibling file in the same
// directory, called from rootPath under an innocuous name, can reach a
// forbidden production symbol with NO direct reference ever appearing
// inside rootPath's own source at all (see
// TestOutcomeOracle_IndependenceCallScan_DefeatSiblingIndirection below,
// which plants exactly this shape). This walks every same-package,
// same-directory `name(...)` call reachable from rootPath's own
// declarations — following it into whichever FILE actually declares
// that name, wherever in the package that is — and reports a violation
// the moment the walk reaches a forbidden symbol, from any file.
//
// STATED LIMIT (the same disclosure discipline bead 2's
// samePackageEscapeShapes settled on for its own unfixturable
// same-package escape shapes, internal/lint/destructive_guidance_test.go):
// this follows a bare `name(...)` call to a package-level function or a
// package-level `var name = <expr>` (recursing again if that RHS is
// itself another followable name, so a short var-to-var indirection
// chain IS covered). It does NOT resolve a function value threaded
// through a LOCAL variable, a struct field, an interface method set, or
// a runtime-constructed closure; NOR a package-level `var name`
// declared WITHOUT its own initializer whose value is instead assigned
// by a sibling file's `init()` and read only by value at the call site
// (bead-4 fix round 2, O2 new MINOR — buildPackageTables above only
// populates `vars[name] = vs.Values[i]` when the declaration itself
// carries an initializer expression, so an init()-populated var has no
// entry for walk to descend into, and init() is never itself reached
// since nothing textually calls it by name) — that class is
// REVIEW-CAUGHT, not mechanically closed here.
func scanForForbiddenRefsWithClosure(t *testing.T, dir, rootPath string, exemptFuncs ...string) []string {
	t.Helper()
	fset, files := parsePackageGoFiles(t, dir)
	funcs, vars := buildPackageTables(files)

	exempt := map[string]bool{}
	for _, f := range exemptFuncs {
		exempt[f] = true
	}

	var rootFile *ast.File
	for _, f := range files {
		if fset.Position(f.Pos()).Filename == rootPath {
			rootFile = f
			break
		}
	}
	if rootFile == nil {
		t.Fatalf("root file %s was not found while parsing package dir %s", rootPath, dir)
	}

	var violations []string
	visitedFuncs := map[string]bool{}
	visitedVars := map[string]bool{}

	var walk func(n ast.Node)
	walk = func(n ast.Node) {
		ast.Inspect(n, func(node ast.Node) bool {
			switch expr := node.(type) {
			case *ast.SelectorExpr:
				if outcomeOracleForbiddenSymbols[expr.Sel.Name] {
					violations = append(violations, fset.Position(expr.Pos()).String()+": reference to forbidden symbol "+expr.Sel.Name)
					return false
				}
			case *ast.Ident:
				if outcomeOracleForbiddenSymbols[expr.Name] {
					violations = append(violations, fset.Position(expr.Pos()).String()+": reference to forbidden symbol "+expr.Name)
				}
				if fd, ok := funcs[expr.Name]; ok && !visitedFuncs[expr.Name] {
					visitedFuncs[expr.Name] = true
					walk(fd.Body)
				} else if ve, ok := vars[expr.Name]; ok && !visitedVars[expr.Name] {
					visitedVars[expr.Name] = true
					walk(ve)
				}
			}
			return true
		})
	}

	for _, decl := range rootFile.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if exempt[d.Name.Name] {
				continue
			}
			visitedFuncs[d.Name.Name] = true
			walk(d.Body)
		case *ast.GenDecl:
			if d.Tok != token.VAR {
				continue
			}
			for _, spec := range d.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range vs.Names {
					visitedVars[name.Name] = true
				}
				for _, val := range vs.Values {
					walk(val)
				}
			}
		}
	}

	return violations
}

// TestOutcomeOracle_IndependenceCallScan_DefeatSiblingIndirection is the
// helper-closure scan's own defeat test for BLOCKING-2 (bead-4 fix round
// 1, G1-2): the plain, file-scoped scanForForbiddenRefs only ever sees a
// reference textually inside the ONE file it is handed — a helper
// function defined in a SIBLING file in the same package, called from
// the "root" file under an innocuous name, reaches a forbidden
// production symbol with NO direct reference ever appearing in the root
// file's own source. This plants exactly that shape (never inside the
// real outcome_oracle_test.go/outcome_oracle_ac3_test.go — a fresh
// synthetic two-file package fixture) and confirms
// scanForForbiddenRefsWithClosure still finds it — proving the scan's
// SCOPE, not merely its sensitivity (the pre-existing
// TestOutcomeOracle_IndependenceCallScan_DefeatPlantedViolation above
// already proved sensitivity within one file; this is the class G1
// reported the plain scan could not see at all).
func TestOutcomeOracle_IndependenceCallScan_DefeatSiblingIndirection(t *testing.T) {
	dir := t.TempDir()
	siblingSrc := `package lifecycle

func siblingPredicateBackdoor(workdir, branch, target string) {
	_, _, _ = EvaluateWorkDestruction(workdir, branch, target)
}
`
	rootSrc := `package lifecycle

func innocuousHelper(workdir, branch, target string) {
	siblingPredicateBackdoor(workdir, branch, target)
}
`
	siblingPath := filepath.Join(dir, "sibling_helper.go")
	rootPath := filepath.Join(dir, "root_probe.go")
	if err := os.WriteFile(siblingPath, []byte(siblingSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rootPath, []byte(rootSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	violations := scanForForbiddenRefsWithClosure(t, dir, rootPath)
	if len(violations) == 0 {
		t.Fatal("a sibling-file helper reached through an innocuously-named same-directory call was not detected — the helper-closure scan is scoped too narrowly (bead-4 fix round 1, G1-2)")
	}
}

// fileImportsGitutil reports whether absPath's parsed import list
// includes internal/gitutil.
func fileImportsGitutil(t *testing.T, absPath string) bool {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, absPath, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", absPath, err)
	}
	for _, imp := range file.Imports {
		path, uerr := strconv.Unquote(imp.Path.Value)
		if uerr == nil && path == "github.com/mrmaxsteel/mindspec/internal/gitutil" {
			return true
		}
	}
	return false
}

// TestOutcomeOracle_NoGitutilImport is independence leg (2): this file
// must carry NO "internal/gitutil" import at all.
func TestOutcomeOracle_NoGitutilImport(t *testing.T) {
	if fileImportsGitutil(t, thisOracleFileSource(t)) {
		t.Fatal("outcome_oracle_test.go must never import internal/gitutil (independence leg 2)")
	}
}

// TestOutcomeOracle_NoGitutilImport_DefeatPlantedImport proves the
// mechanism above actually detects a gitutil import when one is
// present — the same "verify the scan finds something before trusting
// it found nothing" discipline as the call-scan's own defeat test.
func TestOutcomeOracle_NoGitutilImport_DefeatPlantedImport(t *testing.T) {
	src := `package lifecycle

import "github.com/mrmaxsteel/mindspec/internal/gitutil"

var _ = gitutil.WorkDestructionEvidence{}
`
	dir := t.TempDir()
	path := filepath.Join(dir, "planted_import.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if !fileImportsGitutil(t, path) {
		t.Fatal("planted gitutil import was not detected — the independence import check is hollow")
	}
}
