package next

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/validate/readiness"
)

// This fake applies the default ready cap and edge-only --parent semantics.
// Show records include parent-child dependencies, as bd does; the resolver is real.
const nearMissFakeBD = `#!/usr/bin/env python3
import json, os, sys
p=os.environ["HONEST_TRACKER"]
d=json.load(open(p))
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
 emit(d["epics"] if "--type=epic" in a else [r for r in d["rows"] if r.get("parent")==arg("--parent")])
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
elif a[0]=="update" and arg("--parent"):
 row=next(r for r in d["rows"] if r["id"]==a[1])
 row["parent"]=arg("--parent")
 with open(p,"w") as f: json.dump(d,f)
 emit([])
else: sys.exit("unexpected fake argv: "+repr(a))
`

type nearMissFixture struct {
	root, path string
	rows       []BeadInfo
	epics      []phase.EpicInfo
	deps       map[string][]map[string]string
	failShow   string
}

func (f *nearMissFixture) row(id, title, parent, kind string) BeadInfo {
	return BeadInfo{ID: id, Title: title, Parent: parent, IssueType: kind, Status: "open"}
}
func (f *nearMissFixture) save(t *testing.T) {
	t.Helper()
	data, err := json.Marshal(map[string]interface{}{"rows": f.rows, "epics": f.epics, "deps": f.deps, "others": []BeadInfo{{ID: "other-parent", IssueType: "task"}}, "fail_show": f.failShow})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(f.path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
func fixturePlan(headings int, chunks ...int) string {
	var b strings.Builder
	b.WriteString("---\nwork_chunks:\n")
	for _, n := range chunks {
		fmt.Fprintf(&b, "  - id: %d\n    key_file_paths: [internal/fixture.go]\n", n)
	}
	b.WriteString("---\n")
	for n := 1; n <= headings; n++ {
		label := n
		if n == 8 {
			label = 12
		}
		fmt.Fprintf(&b, "## Bead %d: Fixture\n\n**Acceptance Criteria**\n- AC-1: works\n", label)
	}
	return b.String()
}
func newNearMissFixture(t *testing.T, duplicate bool) *nearMissFixture {
	t.Helper()
	requireGit(t)
	root := newBareGitRoot(t)
	dir := filepath.Join(root, ".mindspec", "specs", "129-fixture")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plan.md"), []byte(fixturePlan(8, 1, 2, 3, 4, 5, 6, 7, 8, 10)), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Fixture\n## Acceptance Criteria\n- AC-1: works\n"), 0644); err != nil {
		t.Fatal(err)
	}
	epic := func(id string, n int, title string) phase.EpicInfo {
		return phase.EpicInfo{ID: id, IssueType: "epic", Status: "open", Metadata: map[string]interface{}{"spec_num": n, "spec_title": title}}
	}
	f := &nearMissFixture{root: root, path: filepath.Join(root, "tracker.json"), epics: []phase.EpicInfo{epic("epic-129", 129, "fixture"), epic("epic-777", 777, "other")}, deps: map[string][]map[string]string{}}
	if duplicate {
		f.epics = append([]phase.EpicInfo{epic("duplicate-129", 129, "duplicate")}, f.epics...)
	}
	rows := [][4]string{
		{"epic-129.1", "neutral", "epic-129", "bug"}, // A
		{"epic-129.2", "[129-fixture] G", "", "task"},
		{"epic-129.3", "[129] G prime", "", "task"},
		{"epic-129.4", "neutral", "", "task"}, // D
		{"epic-129.5", "[129-fixture] P", "other-parent", "task"},
		{"epic-129.7", "[777] Q2", "", "task"},
		{"epic-129.8", "[129-fixture] X", "epic-129", "task"},
		{"epic-129.12", "[129-fixture] X prime", "epic-777", "task"},
		{"epic-129.9", "[129-fixture] O", "epic-129", "bug"},
		{"epic-129.10", "[129-fixture] H", "epic-129", "task"},
		{"epic-129.11", "neutral", "", "task"},
		{"nonplan-r", "neutral", "epic-129", "task"},
		{"nonplan-f", "neutral", "epic-129", "bug"},
		{"nonplan-m", "neutral", "", "task"},
		{"nonplan-t", "[129-fixture] T", "", "task"},
		{"epic-129", "[129-fixture] epic", "", "epic"},
	}
	for _, r := range rows {
		f.rows = append(f.rows, f.row(r[0], r[1], r[2], r[3]))
	}
	f.rows[13].Metadata = map[string]interface{}{"spec_id": "129-fixture"}
	// X: realisability on real bd unverified (plan gap 3). X prime uses the measured foreign-parent shape.
	f.deps["epic-129.8"] = []map[string]string{{"id": "epic-777", "issue_type": "epic", "dependency_type": "blocks"}}
	if duplicate {
		f.rows = []BeadInfo{f.row("epic-129.6", "[129-fixture] Q", "", "task")}
	}
	f.save(t)
	if err := os.WriteFile(filepath.Join(root, "bd"), []byte(nearMissFakeBD), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HONEST_TRACKER", f.path)
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	// Reset seams explicitly; tests exercise the production resolver and shared cache.
	oldReady, oldRun, oldFind := readyJSONFn, runBDFn, findEpicForBeadFn
	readyJSONFn, runBDFn, findEpicForBeadFn = bead.ReadyJSON, bead.RunBD, phase.FindEpicForBeadWithCache
	t.Cleanup(func() { readyJSONFn, runBDFn, findEpicForBeadFn = oldReady, oldRun, oldFind })
	t.Cleanup(phase.SetRunBDForTest(bead.RunBD))
	t.Cleanup(phase.SetListJSONForTest(bead.ListJSON))
	return f
}
func ids(rows []BeadInfo) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}
func missByID(t *testing.T, rs ReadySet, id string) NearMiss {
	t.Helper()
	for _, m := range rs.NearMisses {
		if m.Bead.ID == id {
			return m
		}
	}
	t.Fatalf("missing near-miss %s in %+v", id, rs.NearMisses)
	return NearMiss{}
}

func TestNearMissN1_RecoveryYieldsClaimable(t *testing.T) {
	f := newNearMissFixture(t, false)
	f.rows = []BeadInfo{f.row("epic-129.4", "neutral", "", "task")}
	f.save(t)
	rs, err := BuildReadySet(f.root, "129-fixture")
	if err == nil {
		t.Fatal("detached mapped bead must be blocking")
	}
	m := missByID(t, rs, "epic-129.4")
	want := "bead epic-129.4 has the plan-bead form for epic epic-129 and plan.md maps Bead 4, but it has no lineage (no parent edge and no spec tag in its title)\nrecovery: bd update epic-129.4 --parent epic-129"
	if m.String() != want {
		t.Fatalf("N1 diagnostic:\n%s", m.String())
	}
	// Invoke the fake's own update path, never execute the emitted string.
	if out, err := exec.Command(filepath.Join(f.root, "bd"), "update", "epic-129.4", "--parent", "epic-129").CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	rs, err = BuildReadySet(f.root, "129-fixture")
	if err != nil || len(rs.Filtered) != 1 {
		t.Fatalf("recovery did not yield member: %+v %v", rs, err)
	}
	report, err := readiness.EvaluateReadiness(f.root, "epic-129.4")
	if err != nil || report == nil || len(report.Signals) != 4 {
		t.Fatalf("recovered bead never reached readiness signals: %+v %v", report, err)
	}
}
func TestNearMissN2b_SameNumberReparentCorrectsLineage(t *testing.T) {
	f := newNearMissFixture(t, true)
	rs, err := BuildReadySet(f.root, "129-fixture")
	if err == nil {
		t.Fatal("duplicate epic title lineage must block")
	}
	m := missByID(t, rs, "epic-129.6")
	if m.Class != "N2b" || !strings.Contains(m.String(), "its lineage resolves to duplicate-129 by title number 129\nrecovery: bd update epic-129.6 --parent epic-129") {
		t.Fatalf("same number recovery: %s", m.String())
	}
	if out, err := exec.Command(filepath.Join(f.root, "bd"), "update", "epic-129.6", "--parent", "epic-129").CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	rs, err = BuildReadySet(f.root, "129-fixture")
	if err != nil || len(rs.Filtered) != 1 {
		t.Fatalf("reparent: %+v %v", rs, err)
	}
	epic, _, err := phase.FindEpicForBead("epic-129.6")
	if err != nil || epic != "epic-129" {
		t.Fatalf("actual lineage after update: %s %v", epic, err)
	}
}
func TestNearMissN2b_OtherNumberInspectsFirst(t *testing.T) {
	f := newNearMissFixture(t, false)
	rs, err := BuildReadySet(f.root, "129-fixture")
	if err != nil {
		t.Fatal(err)
	}
	m := missByID(t, rs, "epic-129.7")
	want := []string{"recovery: bd show epic-129.7 --json", "recovery (only if epic-129.7 belongs to 129-fixture): bd update epic-129.7 --parent epic-129"}
	if m.Class != "N2b" || !reflect.DeepEqual(m.Recoveries, want) {
		t.Fatalf("other-number: %+v", m)
	}
}
func TestNearMissN4_NoClaimableRecovery(t *testing.T) {
	f := newNearMissFixture(t, false)
	rs, err := BuildReadySet(f.root, "129-fixture")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"epic-129.9", "epic-129.10", "epic-129.11"} {
		m := missByID(t, rs, id)
		if m.Class != "N4" || m.Blocking || strings.Contains(m.String(), "bd update") {
			t.Fatalf("N4: %+v", m)
		}
	}
	if !strings.Contains(missByID(t, rs, "epic-129.11").Message, "; it also has no lineage, so readiness cannot evaluate it at all") {
		t.Fatal("missing no-lineage fact")
	}
}
func TestNearMissN2a_NeverReparentRecovery(t *testing.T) {
	for _, tc := range []struct{ parent, class, sentence string }{
		{"other-parent", "N2a", "but its parent is other-parent"},
		{"epic-777", "N2c", "but its lineage resolves to epic-777 through its dependency on epic epic-777"},
	} {
		t.Run(tc.class, func(t *testing.T) {
			f := newNearMissFixture(t, false)
			f.rows = []BeadInfo{f.row("epic-129.5", "[129-fixture]", tc.parent, "task")}
			f.save(t)
			rs, err := BuildReadySet(f.root, "129-fixture")
			if err == nil {
				t.Fatal("foreign parent must block")
			}
			m := missByID(t, rs, "epic-129.5")
			if m.Class != tc.class || !strings.HasSuffix(m.Message, tc.sentence) || !reflect.DeepEqual(m.Recoveries, []string{"recovery: bd show epic-129.5 --json"}) || strings.Contains(m.String(), "bd update") {
				t.Fatalf("parent recovery: %+v", m)
			}
		})
	}
}

func TestNearMissN2a_NamedEpicParentWithoutMetadata(t *testing.T) {
	f := newNearMissFixture(t, false)
	f.epics = append(f.epics, phase.EpicInfo{ID: "plain-epic", IssueType: "epic", Status: "open"})
	f.rows = []BeadInfo{
		f.row("epic-129.4", "neutral", "plain-epic", "task"),
		f.row("epic-129.7", "[777] Q2", "plain-epic", "task"),
	}
	f.save(t)
	rs, err := BuildReadySet(f.root, "129-fixture")
	if err == nil || len(rs.Filtered) != 0 || len(rs.NearMisses) != 2 {
		t.Fatalf("named parents must remain blocking near-misses: %+v %v", rs, err)
	}
	for _, row := range f.rows {
		m := missByID(t, rs, row.ID)
		want := fmt.Sprintf("bead %s has the plan-bead form for epic epic-129, but its parent is plain-epic\nrecovery: bd show %s --json", row.ID, row.ID)
		if m.Class != "N2a" || !m.Blocking || m.String() != want || strings.Contains(m.String(), "bd update") || strings.Contains(m.String(), "no parent edge") {
			t.Errorf("named parent must use inspection-only recovery: %s", m.String())
		}
	}
}
func TestExclusionNotice_NamesIDs(t *testing.T) {
	f := newNearMissFixture(t, false)
	rs, err := BuildReadySet(f.root, "129-fixture")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range rs.NearMisses {
		if !strings.Contains(rs.ExclusionNotice(), idrender.Bead(m.Bead.ID)) {
			t.Fatalf("notice dropped %s", m.Bead.ID)
		}
	}
	hostile := "x;evil"
	f.rows = append(f.rows, f.row(hostile, "neutral", "epic-129", "bug"))
	f.save(t)
	rs, err = BuildReadySet(f.root, "129-fixture")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rs.ExclusionNotice(), idrender.Bead(hostile)) {
		t.Fatalf("ID render gate lost: %s", rs.ExclusionNotice())
	}
	if strings.Contains(rs.ExclusionNotice(), "bead x;evil ") {
		t.Fatalf("raw hostile ID survived: %s", rs.ExclusionNotice())
	}
}
