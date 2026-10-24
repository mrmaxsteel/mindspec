package lifecycle

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

const mergeStateBranch = "spec/132-test"
const mergeStateBead = "mindspec-test.1"

func mergeStateRepo(t *testing.T) (string, func(...string), *LandedMerge) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_ASKPASS", "/usr/bin/false")
	dir, run := initLandedRepo(t, "132-test")
	mergeStateWrite(t, dir, "a", "before\n")
	run("add", ".")
	run("commit", "-m", "base")
	run("switch", "-c", "bead/"+mergeStateBead)
	mergeStateWrite(t, dir, "a", "after\n")
	mergeStateWrite(t, dir, "c", "new\n")
	run("add", ".")
	run("commit", "-m", "work")
	run("switch", mergeStateBranch)
	run("merge", "--no-ff", "-m", "Merge bead/"+mergeStateBead, "bead/"+mergeStateBead)
	merges, err := FindNamingMerges(dir, mergeStateBranch, mergeStateBead)
	if err != nil || len(merges) != 1 {
		t.Fatalf("naming merges: %+v, %v", merges, err)
	}
	return dir, run, namingMergeIdentity(merges[0])
}

func mergeStateWrite(t *testing.T, dir, name, data string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

func mergeStatePresence(t *testing.T, dir string, m *LandedMerge) MergePresence {
	t.Helper()
	p, err := CheckMergePresence(dir, mergeStateBranch, filepath.Join(dir, ".mindspec", "specs", "132-test"), m)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMergeState_MultipleNamingMergesInvalid(t *testing.T) {
	dir, run, _ := mergeStateRepo(t)
	run("switch", "bead/"+mergeStateBead)
	mergeStateWrite(t, dir, "followup", "second change\n")
	run("add", ".")
	run("commit", "-m", "second change")
	run("switch", mergeStateBranch)
	run("merge", "--no-ff", "-m", "Merge bead/"+mergeStateBead, "bead/"+mergeStateBead)
	merges, err := FindNamingMerges(dir, mergeStateBranch, mergeStateBead)
	if err != nil || len(merges) != 2 {
		t.Fatalf("want two naming merges: %+v, %v", merges, err)
	}
	tip := revParseIn(t, dir, "bead/"+mergeStateBead)
	if tip != merges[0].Parents[1] {
		t.Fatalf("bead tip %s differs from newest second parent %s", tip, merges[0].Parents[1])
	}
	for _, merge := range merges {
		presence := mergeStatePresence(t, dir, namingMergeIdentity(merge))
		if len(presence.ChangedPaths) == 0 || len(presence.RevertedPaths) != 0 {
			t.Fatalf("merge content must remain present: %+v", presence)
		}
	}
	valid, err := FindValidMerge(dir, mergeStateBranch, mergeStateBead)
	if err != nil {
		t.Fatal(err)
	}
	if valid != nil {
		t.Fatalf("multiple naming merges must not be valid: %+v", valid)
	}
}

func TestMergeState_LaterMergeExemptsPath(t *testing.T) {
	dir, run, m := mergeStateRepo(t)
	run("switch", "-c", "bead/mindspec-test.2")
	mergeStateWrite(t, dir, "a", "before\n")
	run("commit", "-am", "replace earlier work")
	run("switch", mergeStateBranch)
	run("merge", "--no-ff", "-m", "Merge bead/mindspec-test.2", "bead/mindspec-test.2")
	if p := mergeStatePresence(t, dir, m); len(p.RevertedPaths) != 0 {
		t.Fatalf("later bead must exempt a: %+v", p)
	}
	if valid, err := FindValidMerge(dir, mergeStateBranch, mergeStateBead); err != nil || valid == nil {
		t.Fatalf("valid merge: %+v, %v", valid, err)
	}
}

func TestMergeState_LaterEditNotRevert(t *testing.T) {
	dir, run, m := mergeStateRepo(t)
	mergeStateWrite(t, dir, "a", "later content\n")
	run("commit", "-am", "later edit")
	if p := mergeStatePresence(t, dir, m); len(p.RevertedPaths) != 0 {
		t.Fatalf("later edits are not reverts: %+v", p)
	}
}

func TestMergeState_RestoredContentWithModeChange(t *testing.T) {
	dir, run, m := mergeStateRepo(t)
	mergeStateWrite(t, dir, "a", "before\n")
	run("add", "a")
	run("update-index", "--chmod=+x", "a")
	run("commit", "-m", "restore content and change mode")
	if p := mergeStatePresence(t, dir, m); !reflect.DeepEqual(p.RevertedPaths, []string{"a"}) {
		t.Fatalf("mode change must not hide restored content: %+v", p)
	}
}

func TestMergeState_RenameInMergeRestoredOldPathIsReverted(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	dir, run := initLandedRepo(t, "132-test")
	mergeStateWrite(t, dir, "a", "same content\n")
	run("add", ".")
	run("commit", "-m", "base")
	run("switch", "-c", "bead/"+mergeStateBead)
	run("mv", "a", "b")
	run("commit", "-m", "rename")
	run("switch", mergeStateBranch)
	run("merge", "--no-ff", "-m", "Merge bead/"+mergeStateBead, "bead/"+mergeStateBead)
	merges, err := FindNamingMerges(dir, mergeStateBranch, mergeStateBead)
	if err != nil || len(merges) != 1 {
		t.Fatalf("merge: %v, %v", merges, err)
	}
	m := namingMergeIdentity(merges[0])
	mergeStateWrite(t, dir, "a", "same content\n")
	run("add", "a")
	run("commit", "-m", "restore old name")
	p := mergeStatePresence(t, dir, m)
	if !reflect.DeepEqual(p.RevertedPaths, []string{"a"}) {
		t.Fatalf("old path must be reverted: %+v", p)
	}
	if valid, err := FindValidMerge(dir, mergeStateBranch, mergeStateBead); err != nil || valid != nil {
		t.Fatalf("reverted merge is valid: %+v, %v", valid, err)
	}
	// Positive control: the legacy helper hides the deleted side of a rename.
	legacy, err := gitutil.DiffNameOnly(dir, m.FirstParent, m.SHA)
	if err != nil || !reflect.DeepEqual(legacy, []string{"b"}) {
		t.Fatalf("rename control: %v, %v", legacy, err)
	}
}

func TestMergeState_PresenceIndependentOfTip(t *testing.T) {
	dir, run, m := mergeStateRepo(t)
	if valid, err := FindValidMerge(dir, mergeStateBranch, mergeStateBead); err != nil || valid == nil {
		t.Fatalf("valid: %v, %v", valid, err)
	}
	run("switch", "bead/"+mergeStateBead)
	mergeStateWrite(t, dir, "followup", "fix\n")
	run("add", ".")
	run("commit", "-m", "fix")
	run("switch", mergeStateBranch)
	if p := mergeStatePresence(t, dir, m); len(p.RevertedPaths) != 0 {
		t.Fatal(p)
	}
	if valid, err := FindValidMerge(dir, mergeStateBranch, mergeStateBead); err != nil || valid != nil {
		t.Fatalf("moved tip must not be valid: %v, %v", valid, err)
	}
}

func TestMergeState_ToolArtifactsExcluded(t *testing.T) {
	dir, run, _ := mergeStateRepo(t)
	run("switch", "-c", "bead/mindspec-test.2")
	if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0700); err != nil {
		t.Fatal(err)
	}
	mergeStateWrite(t, dir, ".beads/issues.jsonl", "tool output\n")
	run("add", ".")
	run("commit", "-m", "tool artifact only")
	run("switch", mergeStateBranch)
	run("merge", "--no-ff", "-m", "Merge bead/mindspec-test.2", "bead/mindspec-test.2")
	merges, err := FindNamingMerges(dir, mergeStateBranch, "mindspec-test.2")
	if err != nil || len(merges) != 1 {
		t.Fatalf("naming merge: %v, %v", merges, err)
	}
	m := namingMergeIdentity(merges[0])
	run("revert", "-m", "1", "--no-edit", m.SHA)
	p := mergeStatePresence(t, dir, m)
	if len(p.ChangedPaths) != 0 || len(p.RevertedPaths) != 0 {
		t.Fatalf("tool artifacts must be excluded: %+v", p)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0700); err != nil {
		t.Fatal(err)
	}
	mergeStateWrite(t, dir, ".beads/issues.jsonl", "untracked tool output\n")
	if got := MergeRestoreOutcome(dir, ".mindspec/specs/132-test"); got != guard.DestructionClean {
		t.Fatalf("artifact dirt = %s", got)
	}
}

func TestMergeState_RestoreHistoryAndOperands(t *testing.T) {
	dir, run, m := mergeStateRepo(t)
	mergeStateWrite(t, dir, "a", "intermediate\n")
	mergeStateWrite(t, dir, "c", "later content\n")
	run("commit", "-am", "first change")
	first := revParseIn(t, dir, "HEAD")
	mergeStateWrite(t, dir, "a", "before\n")
	subject := "restore \x1b[31munsafe"
	run("commit", "-am", subject)
	second := revParseIn(t, dir, "HEAD")
	p := mergeStatePresence(t, dir, m)
	if len(p.LaterChanges) != 2 || p.LaterChanges[0].Commit.SHA != second || p.LaterChanges[1].Commit.SHA != first || !reflect.DeepEqual(p.LaterChanges[1].Paths, []string{"a", "c"}) {
		t.Fatalf("later commits and M paths: %+v", p)
	}
	text, err := RestoreMergeRecovery("/spec path/'quote", mergeStateBead, "mindspec-test", m, p, guard.DestructionClean)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(text, "\x1b") || !strings.Contains(text, termsafe.Escape(subject)) {
		t.Fatalf("subject was not escaped: %q", text)
	}
	if strings.Index(text, " revert '"+second+"'") >= strings.Index(text, " revert '"+first+"'") {
		t.Fatalf("revert order: %s", text)
	}
	if !strings.Contains(text, "git -C \"/spec path/'quote\"") || !strings.Contains(text, "--parent mindspec-test --deps "+mergeStateBead) {
		t.Fatalf("quoted operands or follow-up recovery: %s", text)
	}
}

func TestMergeState_RestoreMergeCommitUsesMainline(t *testing.T) {
	dir, run, m := mergeStateRepo(t)
	run("switch", "-c", "other")
	mergeStateWrite(t, dir, "a", "before\n")
	run("commit", "-am", "restore")
	run("switch", mergeStateBranch)
	run("merge", "--no-ff", "-m", "Merge other", "other")
	later := revParseIn(t, dir, "HEAD")
	p := mergeStatePresence(t, dir, m)
	text, err := RestoreMergeRecovery(dir, mergeStateBead, "mindspec-test", m, p, guard.DestructionClean)
	if err != nil || !strings.Contains(text, " revert -m 1 '"+later+"'") {
		t.Fatalf("merge recovery: %s, %v", text, err)
	}
}

func TestMergeState_WorktreeOperands(t *testing.T) {
	for _, path := range []string{"/space path", "/single'quote", "/both'\"quotes/$dollar/`tick`", "/trailing'\\"} {
		quoted := mergeWorktreeQuote(path)
		cmd := exec.Command("/bin/sh", "-c", "set -- "+quoted+"; printf '%s' \"$1\"")
		out, err := cmd.Output()
		want := path
		if strings.HasSuffix(path, "\\") {
			want += "/."
		}
		if err != nil || string(out) != want {
			t.Fatalf("worktree operand %q: %q, %v", path, out, err)
		}
		if _, err := guard.NewDestructiveCommand("git -C "+quoted+" checkout abc -- a", guard.DestructionClean); err != nil {
			t.Fatalf("constructor rejected quoted worktree %q: %v", path, err)
		}
	}
}

func TestMergeState_RestoreCheckoutFromConstructor(t *testing.T) {
	source, err := os.ReadFile("merge_state.go")
	if err != nil {
		t.Fatal(err)
	}
	assertRestoreCheckoutConstructor(t, source)
	dir, run, m := mergeStateRepo(t)
	mergeStateWrite(t, dir, "a", "before\n")
	run("commit", "-am", "restore before")
	p := mergeStatePresence(t, dir, m)
	text, err := RestoreMergeRecovery(dir, mergeStateBead, "mindspec-test", m, p, MergeRestoreOutcome(dir, ".mindspec/specs/132-test"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, " checkout '"+m.SHA+"' -- ':(literal)a'") || !strings.Contains(text, "commit -m 'Restore merged paths'") {
		t.Fatalf("missing guarded restore: %s", text)
	}
	if !guard.HasFinalRecoveryLine(text) {
		t.Fatal("missing final recovery")
	}
}

// Every checkout literal in the production builder must be inside the
// actual guard constructor call. Raw concatenation outside it fails.
func assertRestoreCheckoutConstructor(t *testing.T, source []byte) {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "merge_state.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	guarded := make(map[token.Pos]bool)
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "guard" || sel.Sel.Name != "NewDestructiveCommand" {
			return true
		}
		if len(call.Args) != 2 {
			t.Error("constructor must take the caller outcome")
			return true
		}
		outcome, ok := call.Args[1].(*ast.Ident)
		if !ok || outcome.Name != "outcome" {
			t.Error("constructor must use the evaluated caller outcome")
		}
		ast.Inspect(call.Args[0], func(n ast.Node) bool {
			if n != nil {
				guarded[n.Pos()] = true
			}
			return true
		})
		return true
	})
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(lit.Value)
		if err == nil && strings.Contains(value, " checkout ") {
			found = true
			if !guarded[lit.Pos()] {
				t.Error("checkout command is built outside guard.NewDestructiveCommand")
			}
		}
		return true
	})
	if !found {
		t.Fatal("no checkout command found")
	}
}

func TestMergeState_RestoreOmitsCheckoutOnUserChanges(t *testing.T) {
	dir, run, m := mergeStateRepo(t)
	run("revert", "-m", "1", "--no-edit", m.SHA)
	p := mergeStatePresence(t, dir, m)
	for _, outcome := range []guard.DestructionOutcome{guard.DestructionAncestor, guard.DestructionSuperseded, guard.DestructionStaleDeletion, guard.DestructionEvidenceError} {
		text, err := RestoreMergeRecovery(dir, mergeStateBead, "mindspec-test", m, p, outcome)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(text, " checkout ") || !strings.Contains(text, " revert '") {
			t.Fatalf("unsafe restore for %s: %s", outcome, text)
		}
	}
	mergeStateWrite(t, dir, "user", "untracked\n")
	if got := MergeRestoreOutcome(dir, ".mindspec/specs/132-test"); got != guard.DestructionStaleDeletion {
		t.Fatalf("user changes outcome = %s", got)
	}
	if got := MergeRestoreOutcome(filepath.Join(dir, "missing"), ".mindspec/specs/132-test"); got != guard.DestructionEvidenceError {
		t.Fatalf("read error outcome = %s", got)
	}
}

func TestMergeRestoreOutcome_LegacyLedgerRemainsUserDirt(t *testing.T) {
	dir, run, _ := mergeStateRepo(t)
	panelDir := filepath.Join(dir, "review", "local")
	if err := os.MkdirAll(panelDir, 0755); err != nil {
		t.Fatal(err)
	}
	mergeStateWrite(t, dir, "review/local/panel.json", "{}")
	mergeStateWrite(t, dir, "review/local/dispositions.jsonl", "before")
	run("add", "--", "review/local")
	run("commit", "-m", "registered legacy panel")
	mergeStateWrite(t, dir, "review/local/dispositions.jsonl", "local edit")
	if got := MergeRestoreOutcome(dir, ".mindspec/specs/132-test"); got != guard.DestructionStaleDeletion {
		t.Fatalf("local ledger enabled restoration: %s", got)
	}
}
