package readiness

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
)

// All transitive lifecycle metadata I/O terminates at this local fake, never
// the operator's bd. The stateful mode exercises real MergeMetadata writes.
const metadataBDFake = `#!/usr/bin/env python3
import json, os, sys
p = os.environ.get("MF3_BD_STORE")
state = json.load(open(p)) if p else {}
args = sys.argv[1:]
if p:
    with open(p + ".calls", "a") as f: f.write(json.dumps(args) + "\n")
if len(args) == 3 and args[0] == "show" and args[2] == "--json":
    print(json.dumps([state.get(args[1], {"metadata": {}})]))
elif p and len(args) == 4 and args[0] == "update" and args[2] == "--metadata":
    state.setdefault(args[1], {})["metadata"] = json.loads(args[3])
    with open(p, "w") as f: json.dump(state, f)
else:
    sys.exit("unexpected fake bd arguments: " + repr(args))
`

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "mf3-bd-fake-")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bd"), []byte(metadataBDFake), 0o755); err != nil {
		panic(err)
	}
	os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	os.Exit(m.Run())
}

func statefulMetadataBD(t *testing.T, records map[string]map[string]any) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "bd.json")
	b, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MF3_BD_STORE", p)
	return p
}

func readMF3(t *testing.T, root, beadID string) Signal {
	t.Helper()
	r, err := EvaluateReadiness(root, beadID)
	if err != nil {
		t.Fatal(err)
	}
	return signalByID(r, SignalDependencies)
}

func TestReadiness_MF3Variants(t *testing.T) {
	const epic, spec, current = "mindspec-member", "996-membership", "mindspec-member.1"
	for _, tc := range []struct {
		name, id, specID, parent, title, want, class string
		parentError                                  bool
	}{
		{name: "a plain task", id: "plain-task", parent: "other-epic", want: "not a bead of spec 996-membership — landed-merge test not applicable"},
		{name: "b another spec plan bead", id: "other-epic.1", specID: "997-other", parent: "other-epic", want: "not a bead of spec 996-membership — landed-merge test not applicable"},
		{name: "c same spec plan bead", id: "mindspec-member.2", want: "no two-parent merge reachable", class: FailureNoCandidate},
		{name: "d metadata only", id: "metadata-task", specID: spec, want: "linked to spec 996-membership by metadata.spec_id but does not have the plan-bead form", class: FailureNoCandidate},
		{name: "e parent only", id: "reparented-task", parent: epic, want: "linked to spec 996-membership by parent edge but does not have the plan-bead form", class: FailureNoCandidate},
		{name: "f title only", id: "title-task", title: "[996-membership] plain task", want: "not a bead of spec 996-membership — landed-merge test not applicable"},
		{name: "g parent read error", id: "unreadable-task", want: "could not determine whether unreadable-task belongs to spec 996-membership", class: FailureIndeterminate, parentError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := writeWorkspace(root, spec, "# Spec\n", mf3PlanMD(spec)); err != nil {
				t.Fatal(err)
			}
			// The embedded parent is intentionally null, including leg e.
			parent := any(tc.parent)
			if tc.parentError {
				parent = 42 // A genuine parse failure on the dependency's OWN row.
			}
			statefulMetadataBD(t, map[string]map[string]any{
				current: {"dependencies": []any{map[string]any{"id": tc.id, "status": "closed", "dependency_type": "blocks", "parent": nil, "metadata": map[string]any{"spec_id": tc.specID}}}},
				tc.id:   {"parent": parent, "title": tc.title},
			})
			origEpic, origLanded := findEpicForBeadFn, findLandedMergeFn
			t.Cleanup(func() { findEpicForBeadFn, findLandedMergeFn = origEpic, origLanded })
			findEpicForBeadFn = func(string) (string, string, error) { return epic, spec, nil }
			calls := 0
			findLandedMergeFn = func(string, string, string) (*lifecycle.LandedMerge, error) {
				calls++
				return nil, lifecycle.ErrLandedMergeNoCandidate
			}
			s := readMF3(t, root, current)
			if s.Pass != (tc.class == "") || !strings.Contains(s.Detail, tc.want) {
				t.Fatalf("membership outcome: %+v; want %q class %q", s, tc.want, tc.class)
			}
			if tc.class != "" && !reflect.DeepEqual(s.Failures, []string{DependencyFailureRecord(tc.id, tc.class, "")}) {
				t.Fatalf("failure records: %v", s.Failures)
			}
			wantCalls := 0
			if tc.class == FailureNoCandidate {
				wantCalls = 1
			}
			if calls != wantCalls {
				t.Fatalf("landed checks=%d want %d", calls, wantCalls)
			}
			if tc.name == "c same spec plan bead" && !strings.Contains(s.Detail, "plan.md does not map it (a follow-up)") {
				t.Fatalf("unmapped follow-up hint missing: %s", s.Detail)
			}
			if tc.class == FailureNoCandidate && !strings.Contains(s.Detail, dependencyOverride(current)) {
				t.Fatalf("deliberate override missing: %s", s.Detail)
			}
			if tc.parentError && (s.Recovery != "bd show unreadable-task --json" || !IsIndeterminateRecord(s.Failures[0])) {
				t.Fatalf("read error must refuse inspection-first: %+v", s)
			}
		})
	}
}

func TestReadiness_MF3_ParentReadOnlyWhenNeeded(t *testing.T) {
	for _, tc := range []struct {
		name, id, specID string
		calls            int
	}{
		{"plan form", "owner-epic.1", "", 0},
		{"metadata", "plain-task", "996-parent", 0},
		{"neither", "plain-task", "997-other", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			origParent, origLanded := fetchDependencyParentFn, findLandedMergeFn
			t.Cleanup(func() { fetchDependencyParentFn, findLandedMergeFn = origParent, origLanded })
			calls := 0
			fetchDependencyParentFn = func(string) (string, error) { calls++; return "owner-epic", nil }
			findLandedMergeFn = func(string, string, string) (*lifecycle.LandedMerge, error) { return &lifecycle.LandedMerge{}, nil }
			s := evaluateMF3("", &Report{BeadID: "owner-epic.2", EpicID: "owner-epic", SpecID: "996-parent"}, "", []dependencyEdge{{ID: tc.id, SpecID: tc.specID, Status: "closed"}})
			if !s.Pass || calls != tc.calls {
				t.Fatalf("parent calls=%d want %d, signal=%+v", calls, tc.calls, s)
			}
		})
	}
}

func TestReadiness_MF3_MalformedMembershipMetadata(t *testing.T) {
	const spec, epic, current = "996-malformed", "owner-epic", "owner-epic.1"
	root := t.TempDir()
	if err := writeWorkspace(root, spec, "# Spec\n", mf3PlanMD(spec)); err != nil {
		t.Fatal(err)
	}
	statefulMetadataBD(t, map[string]map[string]any{
		current: {"dependencies": []any{map[string]any{"id": "plain-task", "status": "closed", "dependency_type": "blocks", "metadata": map[string]any{"spec_id": 42}}}},
	})
	origEpic, origParent := findEpicForBeadFn, fetchDependencyParentFn
	t.Cleanup(func() { findEpicForBeadFn, fetchDependencyParentFn = origEpic, origParent })
	findEpicForBeadFn = func(string) (string, string, error) { return epic, spec, nil }
	fetchDependencyParentFn = func(string) (string, error) {
		t.Fatal("malformed metadata must refuse before parent read")
		return "", nil
	}
	s := readMF3(t, root, current)
	if s.Pass || !reflect.DeepEqual(s.Failures, []string{"plain-task:indeterminate"}) || !strings.Contains(s.Detail, "could not determine whether plain-task belongs to spec 996-malformed: parsing dependency metadata:") || s.Recovery != "bd show plain-task --json" {
		t.Fatalf("malformed membership must fail closed: %+v", s)
	}
}

// A real-git #239 topology: the original spec reaches main through the
// second parent of a PR merge, then the spec fast-forwards to main.
func mf3GitFixture(t *testing.T, offFirstParent, wrapper, binding bool) (root, branch, dep, current, merge, second string) {
	t.Helper()
	root = t.TempDir()
	const spec, epic = "996-real-mf3", "mindspec-real"
	dep, current = epic+".2", epic+".1"
	if err := writeWorkspace(root, spec, "# Spec\n", mf3PlanMD(spec)); err != nil {
		t.Fatal(err)
	}
	var err error
	branch, err = initGitRepo(root, spec)
	if err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		if err := gitRun(root, args...); err != nil {
			t.Fatal(err)
		}
	}
	run("checkout", "-b", "bead/"+dep)
	if err := os.WriteFile(filepath.Join(root, "work.txt"), []byte("landed work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "work.txt")
	run("commit", "-m", "reviewed work")
	reviewed, err := gitOutput(root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if wrapper {
		// complete's materialized wrapper changes identity without changing
		// content: the panel reviewed B, but the merge's second parent is D.
		run("commit", "--allow-empty", "-m", "materialized landing wrapper")
	}
	second, err = gitOutput(root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if wrapper && reviewed == second {
		t.Fatal("wrapper fixture must have panel B different from second parent D")
	}
	run("checkout", branch)
	run("merge", "--no-ff", "-m", "Merge bead/"+dep, "bead/"+dep)
	merge, err = gitOutput(root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if err := registerLandedPanel(root, "round-one", dep, branch, reviewed); err != nil {
		t.Fatal(err)
	}
	run("branch", "-D", "bead/"+dep)
	if offFirstParent {
		run("checkout", "main")
		if err := os.WriteFile(filepath.Join(root, "main.txt"), []byte("main evolves\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		run("add", "main.txt")
		run("commit", "-m", "main evolves")
		run("merge", "--no-ff", "-m", "Merge PR", branch)
		run("checkout", branch)
		run("merge", "--ff-only", "main")
	}
	metadata := map[string]any{}
	if binding {
		metadata["mindspec_landed_merge_sha"], metadata["mindspec_landed_second_parent"] = merge, second
	}
	statefulMetadataBD(t, map[string]map[string]any{dep: {"metadata": metadata}})
	store := NewFakeBDStore()
	store.Lineage[current] = FakeLineage{EpicID: epic, SpecID: spec}
	store.Records[current] = FakeBeadRecord{Dependencies: []FakeDependency{{ID: dep, Status: "closed"}}}
	t.Cleanup(store.Install())
	return
}

func TestReadiness_MF3_OffFirstParentFailsWithExits(t *testing.T) {
	root, branch, dep, current, merge, _ := mf3GitFixture(t, true, false, false)
	_, err := lifecycle.FindLandedMergeReachable(root, branch, dep)
	var off *lifecycle.LandedMergeOffFirstParent
	if !errors.As(err, &off) {
		t.Fatalf("want off-first-parent, got %T %v", err, err)
	}
	s := readMF3(t, root, current)
	for _, want := range []string{merge, "off its first-parent chain", "automatic identification never accepts it without an audited reattest", "recovery (if reattest refuses): " + dependencyOverride(current)} {
		if !strings.Contains(s.Detail, want) {
			t.Fatalf("missing %q: %+v", want, s)
		}
	}
	if s.Pass || s.Recovery != "mindspec reattest "+dep || !reflect.DeepEqual(s.Failures, []string{DependencyFailureRecord(dep, FailureOffFirstParent, merge)}) || strings.Contains(s.Detail, "no two-parent merge reachable") || strings.Contains(s.Detail, "attested-restore") {
		t.Fatalf("wrong fallback exits: %+v", s)
	}
	res, err := lifecycle.ReattestLandedMerge(root, branch, dep, "readiness-fixture")
	if err != nil || !res.Wrote {
		t.Fatalf("real reattest: %+v %v", res, err)
	}
	if after := readMF3(t, root, current); !after.Pass {
		t.Fatalf("read-back: %+v", after)
	}
}

func TestReadiness_MF3_RevertedAfterReattest(t *testing.T) {
	root, branch, dep, current, merge, _ := mf3GitFixture(t, true, false, false)
	if _, err := lifecycle.ReattestLandedMerge(root, branch, dep, "readiness-fixture"); err != nil {
		t.Fatal(err)
	}
	if s := readMF3(t, root, current); !s.Pass {
		t.Fatalf("accepted fixture: %+v", s)
	}
	if err := gitRun(root, "revert", "--no-edit", "-m", "1", merge); err != nil {
		t.Fatal(err)
	}
	s := readMF3(t, root, current)
	if s.Pass || !strings.Contains(s.Detail, "merge "+merge+" landing bead/"+dep+" was reverted or cleanly removed after landing") || !reflect.DeepEqual(s.Failures, []string{DependencyFailureRecord(dep, FailureReverted, merge)}) {
		t.Fatalf("current-tip revert must fail: %+v", s)
	}
}

func TestReadiness_MF3_CompleteWrapperShape(t *testing.T) {
	root, _, _, current, _, _ := mf3GitFixture(t, false, true, true)
	log := traceMF3Git(t)
	if s := readMF3(t, root, current); !s.Pass {
		t.Fatalf("binding must confirm wrapper despite stale panel: %+v", s)
	}
	assertMF3NoOriginRead(t, log)
}

func TestReadiness_MF3_CompleteWrapperNoBindingReattest(t *testing.T) {
	root, branch, dep, current, merge, _ := mf3GitFixture(t, false, true, false)
	log := traceMF3Git(t)
	_, err := lifecycle.FindLandedMergeReachable(root, branch, dep)
	var noEvidence *lifecycle.LandedMergeNoEvidence
	if !errors.As(err, &noEvidence) || noEvidence.MergeSHA != merge {
		t.Fatalf("candidate evidence: %T %v", err, err)
	}
	s := readMF3(t, root, current)
	if s.Pass || !strings.Contains(s.Detail, merge) || !strings.Contains(s.Detail, "no admissible datum corroborates it") || strings.Contains(s.Detail, "no two-parent merge reachable") {
		t.Fatalf("missing candidate evidence: %+v", s)
	}
	if _, err := lifecycle.ReattestLandedMerge(root, branch, dep, "readiness-fixture"); err != nil {
		t.Fatal(err)
	}
	if s := readMF3(t, root, current); !s.Pass {
		t.Fatalf("reattested wrapper: %+v", s)
	}
	assertMF3NoOriginRead(t, log)
}

// Green today: a local landing remains valid with a stale remote-tracking ref.
func TestReadiness_MF3_LocalAheadOfRemotePasses(t *testing.T) {
	root, _, _, current, merge, _ := mf3GitFixture(t, false, false, true)
	if err := gitRun(root, "update-ref", "refs/remotes/origin/spec/996-real-mf3", merge+"^1"); err != nil {
		t.Fatal(err)
	}
	if s := readMF3(t, root, current); !s.Pass {
		t.Fatalf("local ahead of remote: %+v", s)
	}
}

func TestReadiness_MF3ErrorArmDiscrimination_PreciseClasses(t *testing.T) {
	const merge = "1111111111111111111111111111111111111111"
	const second = "2222222222222222222222222222222222222222"
	for _, tc := range []struct {
		name, class, want, recovery string
		err                         error
	}{
		{"no candidate", FailureNoCandidate, "no two-parent merge reachable from spec/996-errors's tip lands bead/owner-epic.2", "mindspec complete owner-epic.2", lifecycle.ErrLandedMergeNoCandidate},
		{"contradiction", FailureContradiction, "a merge landing bead/owner-epic.2 exists (" + merge + ") but", "bd show owner-epic.2 --json", fmt.Errorf("%w: landed-binding contradicts merge %s's second parent %s", lifecycle.ErrLandedMergeContradiction, merge, second)},
		{"reverted", FailureReverted, "merge " + merge + " landing bead/owner-epic.2 was reverted or cleanly removed after landing", "bd show owner-epic.2 --json", fmt.Errorf("%w: merge %s was reverted", lifecycle.ErrLandedMergeReverted, merge)},
		{"no evidence", FailureNoEvidence, "no admissible datum corroborates it", "mindspec reattest owner-epic.2", &lifecycle.LandedMergeNoEvidence{MergeSHA: merge, SecondParent: second}},
		{"plain F5 error", FailureIndeterminate, "could not evaluate landed-merge state: read failed", "investigate the landed-merge evaluation failure", errors.New("read failed\x1b[31m")},
		{"bare NotFound is indeterminate", FailureIndeterminate, "could not evaluate landed-merge state:", "investigate the landed-merge evaluation failure", fmt.Errorf("%w: unclassified read", lifecycle.ErrLandedMergeNotFound)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			orig := findLandedMergeFn
			t.Cleanup(func() { findLandedMergeFn = orig })
			findLandedMergeFn = func(string, string, string) (*lifecycle.LandedMerge, error) { return nil, tc.err }
			deps := []dependencyEdge{{ID: "owner-epic.2", Status: "closed"}}
			r := &Report{BeadID: "owner-epic.1", EpicID: "owner-epic", SpecID: "996-errors"}
			s := evaluateMF3("", r, mf3PlanMD(r.SpecID), deps)
			candidate := ""
			if tc.class == FailureNoEvidence || tc.class == FailureContradiction || tc.class == FailureReverted {
				candidate = merge
			}
			if s.Pass || !strings.Contains(s.Detail, tc.want) || !strings.Contains(s.Recovery, tc.recovery) || !reflect.DeepEqual(s.Failures, []string{DependencyFailureRecord("owner-epic.2", tc.class, candidate)}) {
				t.Fatalf("wrong error arm: %+v", s)
			}
			if tc.class != FailureNoCandidate && strings.Contains(s.Detail, "no two-parent merge reachable") {
				t.Fatalf("existing merge/read error misreported: %+v", s)
			}
			if tc.class != FailureNoCandidate && strings.Contains(s.Recovery, "mindspec complete") {
				t.Fatalf("wrong recovery: %+v", s)
			}
			if tc.class == FailureIndeterminate {
				if !IsIndeterminateRecord(s.Failures[0]) {
					t.Fatalf("must be uncoverable: %+v", s)
				}
				rendered := Render(&Report{BeadID: r.BeadID, Signals: []Signal{s}})
				if strings.ContainsRune(rendered, '\x1b') {
					t.Fatalf("unescaped read error: %q", rendered)
				}
				deps = append(deps, dependencyEdge{ID: "task-a", Status: "open"})
				combined := evaluateMF3("", r, "", deps)
				if len(combined.Failures) != 2 || combined.Recovery != s.Recovery {
					t.Fatalf("read failure must outrank not-closed: %+v", combined)
				}
			}
		})
	}
}

func TestReadiness_MF3AggregatedPrecedence(t *testing.T) {
	orig := findLandedMergeFn
	t.Cleanup(func() { findLandedMergeFn = orig })
	calls := map[string]int{}
	findLandedMergeFn = func(_, _, id string) (*lifecycle.LandedMerge, error) {
		calls[id]++
		return nil, lifecycle.ErrLandedMergeNoCandidate
	}
	r := &Report{BeadID: "owner-epic.1", EpicID: "owner-epic", SpecID: "996-aggregate"}
	s := evaluateMF3("", r, "", []dependencyEdge{{ID: "owner-epic.2", Status: "closed"}, {ID: "task-a", Status: "open"}})
	if s.Pass || !reflect.DeepEqual(s.Failures, []string{"owner-epic.2:no-candidate", "task-a:not-closed"}) || s.Recovery != "mindspec complete task-a" || calls["owner-epic.2"] != 1 {
		t.Fatalf("complete failure set and precedence: %+v, calls=%v", s, calls)
	}
	report := &Report{BeadID: r.BeadID, Signals: []Signal{s}}
	output := fmt.Sprint(guard.NewFailure(Render(report), report.RecoveryCommands()...))
	if !strings.Contains(output, "task-a [not-closed]") || !strings.Contains(output, "owner-epic.2 [no-candidate]") || strings.Count(output, "recovery:") != 1 || !strings.Contains(output, "recovery: mindspec complete task-a") {
		t.Fatalf("both records, one recovery required: %s", output)
	}
	calls = map[string]int{}
	all := evaluateMF3("", r, "", []dependencyEdge{
		{ID: "owner-epic.2", Status: "closed"},
		{ID: "owner-epic.3", Status: "closed"},
		{ID: "owner-epic.4", Status: "closed"},
	})
	if len(all.Failures) != 3 || !reflect.DeepEqual(calls, map[string]int{"owner-epic.2": 1, "owner-epic.3": 1, "owner-epic.4": 1}) {
		t.Fatalf("each closed member needs one evaluation: %+v, calls=%v", all, calls)
	}
	// Every pair pins the full class ordering; the parent read and finder
	// are separately exercised above. Reverse IDs pin class before ID.
	classes := []string{FailureIndeterminate, FailureNotClosed, FailureNoCandidate, FailureOffFirstParent, FailureNoEvidence, FailureContradiction, FailureReverted}
	for i, higher := range classes {
		for _, lower := range classes[i+1:] {
			got := aggregateDependencyFailures([]dependencyFailure{{id: "A", class: lower, recovery: "lower"}, {id: "Z", class: higher, recovery: "higher"}}, nil)
			if got.Recovery != "higher" || len(got.Failures) != 2 {
				t.Fatalf("%s > %s: %+v", higher, lower, got)
			}
		}
		got := aggregateDependencyFailures([]dependencyFailure{{id: "Z", class: higher, recovery: "Z"}, {id: "A", class: higher, recovery: "A"}}, nil)
		if got.Recovery != "A" {
			t.Fatalf("ID tie-break for %s: %+v", higher, got)
		}
	}
}
