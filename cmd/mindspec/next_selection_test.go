package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

// The fake honors the default cap and edge-only --parent; all tracker state is local.
const nextHonestyFakeBD = `#!/usr/bin/env python3
import json, os, sys
p=os.environ["HONEST_TRACKER"]
d=json.load(open(p))
d["rows"]=d.get("rows") or []
a=sys.argv[1:]
with open(p+".argv","a") as f: f.write(json.dumps(a)+"\n")
def emit(rows): print(json.dumps(rows))
def arg(name, default=None):
 for i,v in enumerate(a):
  if v==name and i+1<len(a): return a[i+1]
  if v.startswith(name+"="): return v.split("=",1)[1]
 return default
if a[0]=="ready":
 rows=[r for r in d["rows"] if r.get("status")=="open"]
 if arg("--parent"): rows=[r for r in rows if r.get("parent")==arg("--parent")]
 n=int(arg("-n",arg("--limit",100)))
 emit(rows if n==0 else rows[:n])
elif a[0]=="list":
 if d.get("fail_children") and arg("--parent") and arg("--status")!="closed": sys.exit("child read failed")
 rows=d["epics"] if "--type=epic" in a else d.get("children",d["rows"])
 if arg("--parent"): rows=[r for r in rows if r.get("parent")==arg("--parent")]
 if arg("--status"): rows=[r for r in rows if r.get("status") in arg("--status").split(",")]
 emit(rows)
elif a[0]=="show":
 if d.get("fail_show")==a[1]: sys.exit("show failed")
 row=next((r.copy() for r in d["epics"]+d["rows"]+d.get("others",[]) if r["id"]==a[1]),None)
 if row:
  deps=d.get("deps",{}).get(a[1],[])[:]
  if row.get("parent"):
   parent=next((r for r in d["epics"]+d["rows"]+d.get("others",[]) if r["id"]==row["parent"]),{})
   deps.append(dict(id=row["parent"],issue_type=parent.get("issue_type","task"),dependency_type="parent-child"))
  row["dependencies"]=deps
 emit([row] if row else [])
elif a[0]=="update":
 row=next(r for r in d["epics"]+d["rows"] if r["id"]==a[1])
 if arg("--parent"): row["parent"]=arg("--parent")
 if "--claim" in a: row["status"]="in_progress"
 if arg("--metadata"): row["metadata"]=json.loads(arg("--metadata"))
 with open(p,"w") as f: json.dump(d,f)
 emit([])
else: sys.exit("unexpected fake argv: "+repr(a))
`

type nextHonestyFixture struct {
	root, path     string
	rows, children []map[string]interface{}
	epics          []map[string]interface{}
	deps           map[string][]map[string]string
	failChildren   bool
}

func nextHonestyRow(id, title, parent, kind, status string) map[string]interface{} {
	return map[string]interface{}{"id": id, "title": title, "parent": parent, "issue_type": kind, "status": status, "description": "AC-1"}
}
func nextHonestyPlan(n int) string {
	s := "---\nwork_chunks:\n"
	for i := 1; i <= n; i++ {
		s += fmt.Sprintf("  - id: %d\n    key_file_paths: [internal/fixture.go]\n", i)
	}
	s += "---\n"
	for i := 1; i <= n; i++ {
		s += fmt.Sprintf("## Bead %d: Fixture\n\n**Acceptance Criteria**\n- AC-1: works\n", i)
	}
	return s
}
func (f *nextHonestyFixture) write(t *testing.T) {
	t.Helper()
	d := map[string]interface{}{"rows": f.rows, "epics": f.epics, "deps": f.deps, "fail_children": f.failChildren}
	if f.children != nil {
		d["children"] = f.children
	}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}
func honestyGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.email=test@example.invalid", "-c", "user.name=test"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return string(out)
}
func newNextHonestyFixture(t *testing.T) *nextHonestyFixture {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_ASKPASS", "/usr/bin/false")
	root := t.TempDir()
	f := &nextHonestyFixture{root: root, path: filepath.Join(root, "tracker.json"), deps: map[string][]map[string]string{}}
	f.epics = []map[string]interface{}{{"id": "epic-129", "title": "[SPEC 129] fixture", "status": "open", "issue_type": "epic", "metadata": map[string]interface{}{"spec_num": 129, "spec_title": "fixture", "mindspec_phase": "plan"}}}
	f.rows = []map[string]interface{}{nextHonestyRow("epic-129.1", "neutral", "epic-129", "task", "open"), nextHonestyRow("epic-129.2", "[129-fixture] reporter", "", "task", "open"), nextHonestyRow("epic-129.3", "[129] numeric", "", "bug", "open")}
	f.write(t)
	dir := filepath.Join(root, ".mindspec/specs/129-fixture")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{".gitignore": "tracker*\nbd\ncaptured-*\n", ".mindspec/specs/129-fixture/plan.md": nextHonestyPlan(3), ".mindspec/specs/129-fixture/spec.md": "# Fixture\n## Acceptance Criteria\n- AC-1: works\n", "bd": nextHonestyFakeBD} {
		mode := os.FileMode(0644)
		if name == "bd" {
			mode = 0755
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), mode); err != nil {
			t.Fatal(err)
		}
	}
	honestyGit(t, root, "init", "-b", "main")
	honestyGit(t, root, "add", ".")
	honestyGit(t, root, "commit", "-m", "fixture")
	t.Setenv("HONEST_TRACKER", f.path)
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Cleanup(phase.SetRunBDForTest(bead.RunBD))
	t.Cleanup(phase.SetListJSONForTest(bead.ListJSON))
	oldExec, oldWT := newExecutorFn, nextEnsureWorktreeFn
	newExecutorFn = func(string) executor.Executor { return &executor.MockExecutor{} }
	nextEnsureWorktreeFn = func(string, string, string, executor.Executor) (string, error) { return "", nil }
	t.Cleanup(func() { newExecutorFn, nextEnsureWorktreeFn = oldExec, oldWT })
	withTestChdir(t, root)
	return f
}
func (f *nextHonestyFixture) run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	// Files avoid the pipe-buffer deadlock for a verbose ready menu.
	out, err := os.CreateTemp(f.root, "captured-")
	if err != nil {
		t.Fatal(err)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = out, out
	cmd := &cobra.Command{Use: "next", RunE: nextCmd.RunE, SilenceErrors: true, SilenceUsage: true}
	registerNextFlags(cmd)
	cmd.SetArgs(args)
	runErr := cmd.Execute()
	os.Stdout, os.Stderr = oldOut, oldErr
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(raw), runErr
}
func requireText(t *testing.T, out string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q:\n%s", want, out)
		}
	}
}
func (f *nextHonestyFixture) audit(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(f.path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw) + honestyGit(t, f.root, "status", "--porcelain") + honestyGit(t, f.root, "for-each-ref", "--format=%(refname) %(objectname)") + honestyGit(t, f.root, "worktree", "list", "--porcelain")
}
func (f *nextHonestyFixture) status(t *testing.T, id string) string {
	t.Helper()
	raw, err := os.ReadFile(f.path)
	if err != nil {
		t.Fatal(err)
	}
	var d struct{ Rows []struct{ ID, Status string } }
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	for _, r := range d.Rows {
		if r.ID == id {
			return r.Status
		}
	}
	t.Fatalf("missing row %s", id)
	return ""
}
func TestNext_101ReadyBeads_AllVisible(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".mindspec"), 0755); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Error(err)
		}
	})
	rows := make([]map[string]interface{}, 101)
	for i := range rows {
		rows[i] = map[string]interface{}{"id": fmt.Sprintf("epic-129.%d", i+1), "title": fmt.Sprintf("Ready row %d", i+1), "status": "open", "issue_type": "task"}
	}
	for name, n := range map[string]int{"all": 101, "capped": 100} {
		raw, err := json.Marshal(rows[:n])
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), raw, 0644); err != nil {
			t.Fatal(err)
		}
	}
	script := `#!/bin/sh
case "$1" in
 ready)
  previous=''; unlimited=no
  for arg in "$@"; do
   if { [ "$previous" = '-n' ] || [ "$previous" = '--limit' ]; } && [ "$arg" = '0' ]; then unlimited=yes; fi
   previous="$arg"
  done
  if [ "$unlimited" = yes ]; then cat "$ROWCAP_FIXTURE/all"; else cat "$ROWCAP_FIXTURE/capped"; fi
  ;;
 show) printf '[{"id":"%s","title":"Selected row","description":"Fixture work"}]' "$2" ;;
 *) printf 'unexpected fake bd command: %s\n' "$1" >&2; exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "bd"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ROWCAP_FIXTURE", dir)
	// Pin the fake's cap semantics independently of the production waist's spelling.
	for _, limit := range [][]string{nil, {"-n", "0"}, {"--limit", "0"}} {
		args := append([]string{"ready"}, limit...)
		raw, err := exec.Command(filepath.Join(dir, "bd"), args...).Output()
		if err != nil {
			t.Fatal(err)
		}
		var got []map[string]interface{}
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		want := 101
		if limit == nil {
			want = 100
		}
		if len(got) != want {
			t.Fatalf("capping fake with %q returned %d rows, want %d", args, len(got), want)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, pick := range []int{0, 101} {
		// A file avoids pipe-buffer deadlocks while the 101-item menu is written.
		f, err := os.CreateTemp(dir, "stdout-")
		if err != nil {
			t.Fatal(err)
		}
		orig := os.Stdout
		os.Stdout = f
		// Fresh flags keep the real CLI RunE independent of other CLI tests.
		cmd := &cobra.Command{Use: "next", RunE: nextCmd.RunE}
		registerNextFlags(cmd)
		cmd.SetArgs([]string{"--emit-only", fmt.Sprintf("--pick=%d", pick)})
		err = cmd.Execute()
		os.Stdout = orig
		if closeErr := f.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		raw, readErr := os.ReadFile(f.Name())
		if readErr != nil {
			t.Fatal(readErr)
		}
		out := string(raw)
		if err != nil {
			t.Fatalf("pick %d: %v\n%s", pick, err, out)
		}
		if !strings.Contains(out, "Ready work (101 items):") {
			t.Fatalf("101 rows must be visible:\n%s", out)
		}
		for i := 1; i <= 101; i++ {
			if !strings.Contains(out, fmt.Sprintf("epic-129.%d", i)) {
				t.Fatalf("row %d missing", i)
			}
		}
		if pick == 101 && !strings.Contains(out, "**Bead**: epic-129.101") {
			t.Fatalf("pick 101 did not emit last bead:\n%s", out)
		}
	}
}

func TestNextSelection_EmitOnlyPickParity_DifferentBeadsPerPick(t *testing.T) {
	f := newNextHonestyFixture(t)
	for pick, id := range []string{"epic-129.1", "epic-129.2"} {
		out, err := f.run(t, "--spec", "129-fixture", "--emit-only", fmt.Sprintf("--pick=%d", pick+1))
		if err != nil {
			t.Fatalf("%v %s", err, out)
		}
		requireText(t, out, "**Bead**: "+id)
	}
	_, err := f.run(t, "--spec", "129-fixture", "--emit-only", "--pick=4")
	if err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("range: %v", err)
	}
}
func TestNextSelection_ClaimPickMatchesEmitOnlyPick(t *testing.T) {
	f := newNextHonestyFixture(t)
	honestyGit(t, f.root, "branch", "spec/129-fixture")
	out, err := f.run(t, "--spec", "129-fixture", "--emit-only", "--pick=2")
	if err != nil {
		t.Fatal(err)
	}
	requireText(t, out, "**Bead**: epic-129.2")
	out, err = f.run(t, "--spec", "129-fixture", "--pick=2")
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if f.status(t, "epic-129.2") != "in_progress" || f.status(t, "epic-129.1") != "open" {
		t.Fatalf("claim pick parity: %s", out)
	}
}
func TestNextSelection_SingleReadyItem_OutOfRangePickErrors(t *testing.T) {
	f := newNextHonestyFixture(t)
	f.rows = f.rows[:1]
	f.write(t)
	_, err := f.run(t, "--spec", "129-fixture", "--emit-only", "--pick=2")
	if err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("range: %v", err)
	}
}
func TestNextSelection_PositionalExcludedByFilter_NamesReason(t *testing.T) {
	f := newNextHonestyFixture(t)
	f.rows = append(f.rows, nextHonestyRow("nonplan-r", "neutral", "epic-129", "task", "open"))
	f.write(t)
	_, err := f.run(t, "--spec", "129-fixture", "nonplan-r")
	if err == nil {
		t.Fatal("non-plan child selected")
	}
	requireText(t, err.Error(), "bead nonplan-r is bd-ready", "not a plan bead", "recovery: bd show nonplan-r --json")
}
func TestNextSelection_AllExcludedByFilter_LoudError(t *testing.T) {
	for _, emit := range []bool{false, true} {
		t.Run(fmt.Sprint(emit), func(t *testing.T) {
			f := newNextHonestyFixture(t)
			f.rows = []map[string]interface{}{nextHonestyRow("nonplan-r", "neutral", "epic-129", "task", "open")}
			f.write(t)
			args := []string{"--spec", "129-fixture"}
			if emit {
				args = append(args, "--emit-only")
			}
			out, err := f.run(t, args...)
			if err == nil {
				t.Fatalf("blocking near-miss exit zero: %s", out)
			}
			requireText(t, out, "nonplan-r", "child-derived phase: plan", "an empty ready set is not evidence the spec is complete; the derived phase above is")
		})
	}
}
func TestNextSelection_ExplicitSpecScopeHonesty_NonexistentSpec(t *testing.T) {
	f := newNextHonestyFixture(t)
	for _, emit := range []bool{false, true} {
		args := []string{"--spec", "999-no-such-spec"}
		if emit {
			args = append(args, "--emit-only")
		}
		_, err := f.run(t, args...)
		if err == nil {
			t.Fatal("nonexistent scope succeeded")
		}
	}
	raw, _ := os.ReadFile(f.path + ".argv")
	if strings.Contains(string(raw), `"ready"`) {
		t.Fatalf("unresolved scope queried ready: %s", raw)
	}
}
func TestNextSelection_ExplicitSpecScopeHonesty_ExistingDraft(t *testing.T) {
	f := newNextHonestyFixture(t)
	dir := filepath.Join(f.root, ".mindspec/specs/777-draft")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Draft"), 0644); err != nil {
		t.Fatal(err)
	}
	honestyGit(t, f.root, "add", ".mindspec/specs/777-draft")
	honestyGit(t, f.root, "commit", "-m", "draft")
	honestyGit(t, f.root, "branch", "spec/777-draft")
	for _, emit := range []bool{false, true} {
		args := []string{"--spec", "777-draft"}
		if emit {
			args = append(args, "--emit-only")
		}
		_, err := f.run(t, args...)
		if err == nil {
			t.Fatal("draft must refuse")
		}
		requireText(t, err.Error(), "no epic", "spec approve")
	}
	raw, _ := os.ReadFile(f.path + ".argv")
	if strings.Contains(string(raw), `"ready"`) {
		t.Fatalf("draft scope queried ready: %s", raw)
	}
}

const honestyDefaultLine = "Defaulting to item 1 (bd ready priority order). Use --pick=N or a bead ID to select a specific item."

func TestNextSelection_EmitOnlyDefaultPick_PrintsLineAndSelectsRow1(t *testing.T) {
	f := newNextHonestyFixture(t)
	out, err := f.run(t, "--spec", "129-fixture", "--emit-only")
	if err != nil {
		t.Fatal(err)
	}
	requireText(t, out, honestyDefaultLine, "**Bead**: epic-129.1")
}
func TestNextSelection_ClaimDefaultPick_PrintsLineAndClaimsRow1(t *testing.T) {
	f := newNextHonestyFixture(t)
	honestyGit(t, f.root, "branch", "spec/129-fixture")
	out, err := f.run(t, "--spec", "129-fixture")
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	requireText(t, out, honestyDefaultLine)
	if f.status(t, "epic-129.1") != "in_progress" {
		t.Fatal("default did not claim first row")
	}
}
func TestNextSelection_PositionalEmitOnly_BypassesSpecFilter(t *testing.T) {
	f := newNextHonestyFixture(t)
	f.rows = []map[string]interface{}{nextHonestyRow("nonplan-r", "neutral", "epic-129", "task", "open")}
	f.write(t)
	before := f.audit(t)
	out, err := f.run(t, "--spec", "129-fixture", "--emit-only", "nonplan-r")
	if err != nil {
		t.Fatal(err)
	}
	requireText(t, out, "emit-only: bead fetched by ID; readiness not checked.", "**Bead**: nonplan-r")
	if strings.Contains(out, "not a plan bead") || f.audit(t) != before {
		t.Fatalf("positional preview mutated or classified: %s", out)
	}
}
func TestNextSelection_NegativePick_PositionalConflict_ZeroSideEffects(t *testing.T) {
	f := newNextHonestyFixture(t)
	before := f.audit(t)
	for _, args := range [][]string{{"epic-129.1", "--pick=-1"}, {"epic-129.1", "--pick=-1", "--emit-only"}} {
		_, err := f.run(t, args...)
		if err == nil {
			t.Fatal("negative pick accepted")
		}
		requireText(t, err.Error(), "invalid")
	}
	if f.audit(t) != before {
		t.Fatal("negative pick mutated")
	}
}
func TestNextSelection_NegativePick_EmptyReadySet_NeverSucceeds(t *testing.T) {
	f := newNextHonestyFixture(t)
	f.rows = nil
	f.write(t)
	for _, args := range [][]string{{"--spec", "129-fixture", "--pick=-1"}, {"--spec", "129-fixture", "--pick=-1", "--emit-only"}} {
		_, err := f.run(t, args...)
		if err == nil {
			t.Fatal("negative pick accepted")
		}
		requireText(t, err.Error(), "invalid")
	}
}
func TestNextSelection_PositionalEmitOnly_HostileIDRendersEscaped(t *testing.T) {
	f := newNextHonestyFixture(t)
	_, err := f.run(t, "--emit-only", "evil\x1b[31m")
	if err == nil || strings.ContainsRune(err.Error(), 0x1b) {
		t.Fatalf("hostile ID: %v", err)
	}
}
func testNextPartialNotice(t *testing.T, emit bool) {
	f := newNextHonestyFixture(t)
	honestyGit(t, f.root, "branch", "spec/129-fixture")
	f.rows = append(f.rows, nextHonestyRow("nonplan-r", "neutral", "epic-129", "task", "open"))
	f.write(t)
	args := []string{"--spec", "129-fixture"}
	if emit {
		args = append(args, "--emit-only")
	}
	out, err := f.run(t, args...)
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	requireText(t, out, "bead nonplan-r is bd-ready", "not a plan bead", "recovery: bd show nonplan-r --json", "**Bead**: epic-129.1")
	if f.status(t, "nonplan-r") != "open" {
		t.Fatal("near-miss claimed")
	}
}
func TestNextSelection_ClaimPath_PartialExclusionNoticePrinted(t *testing.T) {
	testNextPartialNotice(t, false)
}
func TestNextSelection_EmitOnlyPath_PartialExclusionNoticePrinted(t *testing.T) {
	testNextPartialNotice(t, true)
}

func TestNext_ScopedSelection_ReporterShapesClaimable(t *testing.T) {
	for _, id := range []string{"epic-129.2", "epic-129.3"} {
		t.Run(id, func(t *testing.T) {
			f := newNextHonestyFixture(t)
			honestyGit(t, f.root, "branch", "spec/129-fixture")
			out, err := f.run(t, "--spec", "129-fixture", id)
			if err != nil {
				t.Fatalf("reporter shape: %v %s", err, out)
			}
			if f.status(t, id) != "in_progress" {
				t.Fatal("reporter shape not claimed")
			}
		})
	}
}
func TestNext_ScopedSelection_NamesNearMisses(t *testing.T) {
	f := newNextHonestyFixture(t)
	f.rows = append(f.rows, nextHonestyRow("epic-129.4", "neutral", "", "task", "open"), nextHonestyRow("nonplan-m", "neutral", "", "task", "open"), nextHonestyRow("nonplan-t", "[129-fixture] title", "", "task", "open"))
	f.rows[len(f.rows)-2]["metadata"] = map[string]interface{}{"spec_id": "129-fixture"}
	f.write(t)
	out, err := f.run(t, "--spec", "129-fixture", "--emit-only", "--pick=2")
	if err != nil {
		t.Fatal(err)
	}
	requireText(t, out, "**Bead**: epic-129.2", "bead epic-129.4", "bead nonplan-m", "by metadata.spec_id", "bead nonplan-t", "by title")
}

func TestNext_EmitOnlyReadsPlanFromSpecBranch(t *testing.T) {
	f := newNextHonestyFixture(t)
	honestyGit(t, f.root, "branch", "spec/129-fixture")
	if err := os.Rename(filepath.Join(f.root, ".mindspec/specs/129-fixture/plan.md"), filepath.Join(f.root, "saved-plan")); err != nil {
		t.Fatal(err)
	}
	out, err := f.run(t, "--spec", "129-fixture", "--emit-only", "--pick=2")
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	requireText(t, out, "**Bead**: epic-129.2")
	if strings.Contains(out, "differs from spec/") {
		t.Fatalf("absent checkout is not divergence: %s", out)
	}
}
func TestNext_PlanDivergenceNotice(t *testing.T) {
	for _, source := range []string{"checkout", "spec-worktree"} {
		t.Run(source, func(t *testing.T) {
			f := newNextHonestyFixture(t)
			honestyGit(t, f.root, "branch", "spec/129-fixture")
			if source == "spec-worktree" {
				name, err := workspace.SpecWorktreeName("129-fixture")
				if err != nil {
					t.Fatal(err)
				}
				honestyGit(t, f.root, "worktree", "add", filepath.Join(f.root, ".worktrees", name), "spec/129-fixture")
			}
			dir, err := workspace.SpecDir(f.root, "129-fixture")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "plan.md"), []byte(nextHonestyPlan(2)), 0644); err != nil {
				t.Fatal(err)
			}
			out, err := f.run(t, "--spec", "129-fixture", "--emit-only", "--pick=3")
			if err != nil {
				t.Fatalf("%v %s", err, out)
			}
			requireText(t, out, "**Bead**: epic-129.3", "plan.md in this checkout differs from spec/129-fixture; mapping used: spec/129-fixture")
			if strings.Count(out, "plan.md in this checkout differs") != 1 {
				t.Fatal("divergence printed more than once")
			}
		})
	}
}
func TestNext_PlanBlobAbsentUsesCheckout(t *testing.T) {
	f := newNextHonestyFixture(t)
	// Create a branch whose tree predates the plan, without changing the checkout.
	tree := strings.TrimSpace(honestyGit(t, f.root, "hash-object", "-w", "-t", "tree", "--stdin"))
	// A scratch orphan branch is represented by a commit-tree over the empty tree.
	cmd := exec.Command("git", "-C", f.root, "-c", "user.email=test@example.invalid", "-c", "user.name=test", "commit-tree", tree, "-m", "before plan")
	raw, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("commit-tree: %v %s", err, raw)
	}
	honestyGit(t, f.root, "branch", "spec/129-fixture", strings.TrimSpace(string(raw)))
	out, err := f.run(t, "--spec", "129-fixture", "--emit-only", "--pick=2")
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	requireText(t, out, "**Bead**: epic-129.2")
	if strings.Contains(out, "differs from spec/") {
		t.Fatalf("blob absent divergence: %s", out)
	}
}
func TestNext_PlanProbeFailureRefuses(t *testing.T) {
	f := newNextHonestyFixture(t)
	restore := next.SetPlanProbeForTest(func(string, string, string) ([]byte, error) { return nil, errors.New("injected git failure") })
	defer restore()
	before := f.audit(t)
	_, err := f.run(t, "--spec", "129-fixture", "--emit-only")
	if err == nil {
		t.Fatal("probe failure fell through to checkout")
	}
	requireText(t, err.Error(), "could not read plan.md for 129-fixture", "injected git failure")
	if f.audit(t) != before {
		t.Fatal("probe failure mutated")
	}
}
func TestNext_LazyPlanReadForNoLineageRow(t *testing.T) {
	for _, mapped := range []bool{true, false} {
		t.Run(fmt.Sprint(mapped), func(t *testing.T) {
			f := newNextHonestyFixture(t)
			f.rows = []map[string]interface{}{nextHonestyRow("epic-129.1", "neutral", "", "task", "open")}
			f.write(t)
			plan := nextHonestyPlan(1)
			if !mapped {
				plan = strings.Replace(plan, "id: 1", "id: 2", 1)
			}
			if err := os.WriteFile(filepath.Join(f.root, ".mindspec/specs/129-fixture/plan.md"), []byte(plan), 0644); err != nil {
				t.Fatal(err)
			}
			out, err := f.run(t, "--spec", "129-fixture", "--emit-only")
			if mapped {
				if err == nil {
					t.Fatal("N1 must block")
				}
				requireText(t, out, "plan.md maps Bead 1", "recovery: bd update epic-129.1 --parent epic-129")
			} else {
				if err != nil {
					t.Fatalf("N4 should be informational: %v %s", err, out)
				}
				requireText(t, out, "no work_chunks id 1", "it also has no lineage")
			}
		})
	}
}
func TestNext_NoPlanYetAllN4(t *testing.T) {
	f := newNextHonestyFixture(t)
	if err := os.Rename(filepath.Join(f.root, ".mindspec/specs/129-fixture/plan.md"), filepath.Join(f.root, "saved-plan")); err != nil {
		t.Fatal(err)
	}
	out, err := f.run(t, "--spec", "129-fixture", "--emit-only")
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	for _, id := range []string{"epic-129.1", "epic-129.2", "epic-129.3"} {
		requireText(t, out, "bead "+id+" is a follow-up child")
	}
	requireText(t, out, "no plan.md exists yet for 129-fixture")
}

func TestNext_EmptyScoped_DerivedState(t *testing.T) {
	for _, tc := range []struct {
		name, stored, derived string
		statuses              []string
		arm                   string
	}{
		{"i", "implement", "review", []string{"closed"}, "recovery: mindspec impl approve '129-fixture'"},
		{"ii", "implement", "implement", []string{"in_progress"}, "Recovering..."},
		{"ii-b", "implement", "implement", []string{"in_progress", "open"}, "Recovering..."},
		{"iii-a", "implement", "implement", []string{"closed", "open", "blocked"}, "2 open lifecycle bead(s), none selectable"},
		{"iii-b", "implement", "plan", []string{"open", "blocked"}, "2 open lifecycle bead(s), none selectable"},
		{"iv", "plan", "plan", nil, "recovery: mindspec plan approve '129-fixture'"},
		{"no-stored", "", "plan", nil, "recovery: mindspec plan approve '129-fixture'"},
		{"invalid-stored", "unknown", "plan", nil, "recovery: mindspec plan approve '129-fixture'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newNextHonestyFixture(t)
			f.rows = nil
			f.children = []map[string]interface{}{}
			for i, status := range tc.statuses {
				f.children = append(f.children, nextHonestyRow(fmt.Sprintf("epic-129.%d", i+1), "neutral", "epic-129", "task", status))
			}
			f.epics[0]["metadata"].(map[string]interface{})["mindspec_phase"] = tc.stored
			f.write(t)
			if tc.name == "iv" {
				if err := os.Rename(filepath.Join(f.root, ".mindspec/specs/129-fixture/plan.md"), filepath.Join(f.root, "saved-plan")); err != nil {
					t.Fatal(err)
				}
			}
			reads := 0
			restore := next.SetPlanProbeForTest(func(string, string, string) ([]byte, error) { reads++; return nil, errors.New("unexpected plan read") })
			defer restore()
			recovered := 0
			if tc.name == "ii" || tc.name == "ii-b" {
				nextEnsureWorktreeFn = func(root, id, spec string, exec executor.Executor) (string, error) {
					recovered++
					expectedRoot, err := filepath.EvalSymlinks(f.root)
					if err != nil {
						t.Fatal(err)
					}
					if root != expectedRoot || id != "epic-129.1" || spec != "129-fixture" {
						t.Fatalf("recovery dispatch: %s %s %s", root, id, spec)
					}
					return filepath.Join(f.root, "recovered"), nil
				}
			}
			// ResolveActiveBead uses the list query; the fake exposes children to it.
			out, err := f.run(t, "--spec", "129-fixture")
			if err != nil {
				t.Fatalf("empty state: %v %s", err, out)
			}
			closed, progress, open := 0, 0, 0
			for _, status := range tc.statuses {
				switch status {
				case "closed":
					closed++
				case "in_progress":
					progress++
				default:
					open++
				}
			}
			tally := fmt.Sprintf("lifecycle children: %d closed / %d in_progress / %d open", closed, progress, open)
			displayedStored := tc.stored
			if tc.name == "invalid-stored" {
				// PhaseDetail sanitizes unrecognized stored values to absent.
				displayedStored = ""
			}
			phaseLine := fmt.Sprintf("stored phase: %s; child-derived phase: %s", displayedStored, tc.derived)
			sentence := "an empty ready set is not evidence the spec is complete; the derived phase above is"
			requireText(t, out, tally+"\n"+phaseLine+"\n"+sentence, tc.arm)
			if strings.Index(out, sentence) > strings.Index(out, tc.arm) {
				t.Fatal("derived-state report must precede the recovery arm")
			}
			if reads != 0 {
				t.Fatalf("empty ready set read plan %d times", reads)
			}
			if tc.name == "i" {
				requireText(t, out, "recovery: mindspec repair phase '129-fixture'")
				if strings.Index(out, tc.arm) > strings.Index(out, "recovery: mindspec repair phase") {
					t.Fatal("phase repair must follow the tally arm")
				}
			} else if strings.Contains(out, "recovery: mindspec repair phase") {
				t.Fatalf("unexpected phase repair for stored=%q derived=%q: %s", tc.stored, tc.derived, out)
			}
			if strings.HasPrefix(tc.name, "iii") {
				requireText(t, out, "recovery: bd blocked --parent epic-129")
				if strings.Contains(out, "recovery: mindspec plan approve") {
					t.Fatal("open children sent to plan approve")
				}
			}
			if tc.name == "ii" || tc.name == "ii-b" {
				if strings.Contains(out, "recovery: bd blocked --parent epic-129") {
					t.Fatal("in-progress recovery must outrank open-child recovery")
				}
				if recovered != 1 {
					t.Fatalf("expected one worktree recovery, got %d", recovered)
				}
				requireText(t, out, "Worktree recovered: "+filepath.Join(f.root, "recovered"))
				if strings.Index(out, "child-derived phase:") > strings.Index(out, "Recovering...") {
					t.Fatal("state must precede mutation")
				}
			}
		})
	}
}
func TestNext_EmptyScoped_InformationalNearMissExitsZero(t *testing.T) {
	for _, blocking := range []bool{false, true} {
		t.Run(fmt.Sprint(blocking), func(t *testing.T) {
			f := newNextHonestyFixture(t)
			f.rows = []map[string]interface{}{nextHonestyRow("epic-129.4", "follow-up", "epic-129", "bug", "open"), nextHonestyRow("nonplan-f", "reparented", "epic-129", "bug", "open"), nextHonestyRow("nonplan-t", "[129-fixture] title", "", "task", "open")}
			f.children = []map[string]interface{}{nextHonestyRow("epic-129.1", "closed", "epic-129", "task", "closed"), f.rows[0], f.rows[1]}
			if blocking {
				// Bead .2 is mapped and detached: N1.
				f.rows = append(f.rows, nextHonestyRow("epic-129.2", "neutral", "", "task", "open"))
			}
			f.epics[0]["metadata"].(map[string]interface{})["mindspec_phase"] = "review"
			f.write(t)
			out, err := f.run(t, "--spec", "129-fixture")
			if (err != nil) != blocking {
				t.Fatalf("blocking=%t err=%v %s", blocking, err, out)
			}
			requireText(t, out, "bead epic-129.4", "bead nonplan-f", "bead nonplan-t", "lifecycle children: 1 closed / 0 in_progress / 0 open", "stored phase: review; child-derived phase: review", "recovery: mindspec impl approve '129-fixture'")
			if strings.Contains(out, "recovery: mindspec repair phase") {
				t.Fatalf("equal phases must not offer phase repair: %s", out)
			}
		})
	}
}
func TestNext_EmptyScoped_ChildReadFailure(t *testing.T) {
	f := newNextHonestyFixture(t)
	f.rows = nil
	f.failChildren = true
	f.write(t)
	out, err := f.run(t, "--spec", "129-fixture")
	if err == nil {
		t.Fatalf("child read failure succeeded: %s", out)
	}
	requireText(t, err.Error(), "child-derived phase: indeterminate", "child read failed")
	if strings.Contains(out, "lifecycle children:") {
		t.Fatalf("failure printed fabricated tally: %s", out)
	}
	if strings.Contains(out, "recovery: mindspec repair phase") {
		t.Fatalf("indeterminate phase must not offer phase repair: %s", out)
	}
}

func TestNext_EmptyScoped_UnresolvedActiveBead(t *testing.T) {
	for _, fail := range []bool{false, true} {
		for _, blocking := range []bool{false, true} {
			t.Run(fmt.Sprintf("error=%t/blocking=%t", fail, blocking), func(t *testing.T) {
				f := newNextHonestyFixture(t)
				f.rows = nil
				if blocking {
					f.rows = []map[string]interface{}{nextHonestyRow("nonplan-r", "neutral", "epic-129", "task", "open")}
				}
				f.children = []map[string]interface{}{nextHonestyRow("epic-129.1", "active", "epic-129", "task", "in_progress")}
				f.epics[0]["metadata"].(map[string]interface{})["mindspec_phase"] = "implement"
				f.write(t)
				old := nextResolveActiveBeadFn
				calls := 0
				nextResolveActiveBeadFn = func(root, spec string) (string, error) {
					calls++
					if spec != "129-fixture" {
						t.Fatalf("wrong spec %q", spec)
					}
					if fail {
						return "", errors.New("active bead lookup failed")
					}
					return "", nil
				}
				t.Cleanup(func() { nextResolveActiveBeadFn = old })
				nextEnsureWorktreeFn = func(string, string, string, executor.Executor) (string, error) {
					t.Fatal("unresolved bead triggered worktree mutation")
					return "", nil
				}
				before := f.audit(t)
				out, err := f.run(t, "--spec", "129-fixture")
				if (err != nil) != blocking || calls != 1 {
					t.Fatalf("active lookup altered exit status: calls=%d err=%v output=%s", calls, err, out)
				}
				const recovery = "recovery: bd list --parent epic-129 --status=in_progress"
				requireText(t, out, "lifecycle children: 0 closed / 1 in_progress / 0 open", "stored phase: implement; child-derived phase: implement", recovery)
				if strings.Index(out, recovery) < strings.Index(out, "child-derived phase:") || strings.Contains(out, "active bead lookup failed") || f.audit(t) != before {
					t.Fatalf("unresolved recovery ordering or side effect: %s", out)
				}
			})
		}
	}
}

func TestNext_EmptyScoped_EmitOnlyInProgress(t *testing.T) {
	f := newNextHonestyFixture(t)
	f.rows = nil
	f.children = []map[string]interface{}{nextHonestyRow("epic-129.1", "active", "epic-129", "task", "in_progress")}
	f.epics[0]["metadata"].(map[string]interface{})["mindspec_phase"] = "implement"
	f.write(t)
	old := nextResolveActiveBeadFn
	calls := 0
	nextResolveActiveBeadFn = func(string, string) (string, error) { calls++; return "epic-129.1", nil }
	t.Cleanup(func() { nextResolveActiveBeadFn = old })
	nextEnsureWorktreeFn = func(string, string, string, executor.Executor) (string, error) {
		t.Fatal("emit-only triggered worktree mutation")
		return "", nil
	}
	before := f.audit(t)
	out, err := f.run(t, "--spec", "129-fixture", "--emit-only")
	if err != nil {
		t.Fatalf("emit-only empty state: %v %s", err, out)
	}
	requireText(t, out, "lifecycle children: 0 closed / 1 in_progress / 0 open", "recovery: mindspec next --spec '129-fixture'")
	if calls != 0 || f.audit(t) != before {
		t.Fatalf("emit-only mutated state or resolved active bead: calls=%d", calls)
	}
}

func TestNext_EmptyScoped_StoredPhaseReadFailure(t *testing.T) {
	f := newNextHonestyFixture(t)
	f.rows = nil
	f.children = []map[string]interface{}{}
	f.write(t)
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) > 1 && args[0] == "show" && args[1] == "epic-129" {
			return nil, os.ErrPermission
		}
		t.Fatalf("unexpected stored-phase read: %v", args)
		return nil, nil
	}))
	err := handleEmptyScoped(f.root, next.ReadySet{EpicID: "epic-129", SpecFlag: "129-fixture", Cache: phase.NewCache()}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "stored phase: indeterminate: ") || !errors.Is(err, os.ErrPermission) || !strings.HasSuffix(err.Error(), "\nrecovery: mindspec state show") {
		t.Fatalf("stored-phase refusal recovery: %v", err)
	}
}
func TestNextMenu_PrintsSpecIDForm(t *testing.T) {
	f := newNextHonestyFixture(t)
	f.epics = append(f.epics, map[string]interface{}{"id": "epic-777", "title": "[SPEC 777] other", "status": "open", "issue_type": "epic", "metadata": map[string]interface{}{"spec_num": 777, "spec_title": "other", "mindspec_phase": "plan"}})
	f.write(t)
	out, err := f.run(t)
	if err == nil {
		t.Fatalf("ambiguous scope succeeded: %s", out)
	}
	requireText(t, out, "129-fixture", "777-other", "mindspec next --spec=<spec-id>")
	if strings.Contains(out, "--spec=<number>") {
		t.Fatal("numeric menu placeholder survived")
	}
}

// Both fixture constructors must consume the production pflag definitions.
func TestNextSelection_UsesProductionFlagRegistration(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("missing source location")
	}
	file, err := parser.ParseFile(token.NewFileSet(), source, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"run", "TestNext_101ReadyBeads_AllVisible"} {
		calls := 0
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != name {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				id, ok := call.Fun.(*ast.Ident)
				if ok && id.Name == "registerNextFlags" {
					calls++
				}
				return true
			})
		}
		if calls != 1 {
			t.Fatalf("fixture %s must call registerNextFlags exactly once; got %d", name, calls)
		}
	}
}
