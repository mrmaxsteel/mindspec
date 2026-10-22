package lifecycle

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// Capture the shipped reader before init installs the hermetic default.
var ancestorDependentsFnDefault = ancestorDependentsFn

func init() {
	ancestorDependentsFn = func(string) ([]string, error) { return nil, nil }
}

const ancestorBead = "mindspec-ab12"
const ancestorSpec = "spec/132-test"

func ancestorFixture(t *testing.T) fallbackFixture {
	t.Helper()
	dir, run := initLandedRepo(t, "132-test")
	run("checkout", "-b", "bead/"+ancestorBead)
	writeFallbackFile(t, dir, "work.txt", "prerequisite work\n")
	run("add", ".")
	run("commit", "-m", "prerequisite work")
	tip := revParseIn(t, dir, "HEAD")
	run("checkout", "-b", "bead/mindspec-carrier")
	writeFallbackFile(t, dir, "carrier.txt", "later bead work\n")
	run("add", ".")
	run("commit", "-m", "later bead work")
	run("checkout", ancestorSpec)
	run("merge", "--no-ff", "-m", "Merge bead/mindspec-carrier", "bead/mindspec-carrier")
	return fallbackFixture{dir: dir, run: run, second: tip, merge: revParseIn(t, dir, "HEAD")}
}

func TestReattest_AncestorWithoutNamingMerge(t *testing.T) {
	f := ancestorFixture(t)
	writes := installReattestRecorder(t)
	result, err := ReattestLandedMerge(f.dir, ancestorSpec, ancestorBead, "tester")
	var refusal *ReattestRefusal
	if result != nil || !errors.As(err, &refusal) || refusal.State != "ancestor-without-naming-merge" {
		t.Fatalf("want ancestor-without-naming-merge, got %+v %v", result, err)
	}
	for _, want := range []string{"changes are present", "no merge names", "re-merge cannot help", "person must verify", f.second} {
		if !strings.Contains(refusal.Detail, want) {
			t.Errorf("missing %q in %s", want, refusal.Detail)
		}
	}
	if len(*writes) != 0 {
		t.Fatal("ancestor diagnosis wrote a binding")
	}
}

func ancestorRefusal(t *testing.T, f fallbackFixture, state string) *ReattestRefusal {
	t.Helper()
	writes := installReattestRecorder(t)
	result, err := ReattestLandedMerge(f.dir, ancestorSpec, ancestorBead, "tester")
	var refusal *ReattestRefusal
	if result != nil || !errors.As(err, &refusal) || refusal.State != state {
		t.Fatalf("want %s, got %+v %v", state, result, err)
	}
	if len(*writes) != 0 {
		t.Fatal("diagnosis wrote a binding")
	}
	return refusal
}

func noAncestorClaim(t *testing.T, r *ReattestRefusal) {
	t.Helper()
	for _, forbidden := range []string{"changes are present", "commits are in", "commits in ", "--allow-not-ready"} {
		if strings.Contains(r.Detail+strings.Join(r.Recovery, "\n"), forbidden) {
			t.Errorf("unexpected claim %q: %+v", forbidden, r)
		}
	}
}

func TestReattest_AncestorZeroCommitsNoClaim(t *testing.T) {
	dir, run := initLandedRepo(t, "132-test")
	run("branch", "bead/"+ancestorBead)
	noAncestorClaim(t, ancestorRefusal(t, fallbackFixture{dir: dir, run: run}, ReattestStateNoOwnedMerge))
}

func TestReattest_AncestorFirstParentRestoreNoClaim(t *testing.T) {
	dir, run := initLandedRepo(t, "132-test")
	writeFallbackFile(t, dir, "first.txt", "first-parent work\n")
	run("add", ".")
	run("commit", "-m", "first-parent work")
	run("branch", "bead/"+ancestorBead)
	writeFallbackFile(t, dir, "later.txt", "later work\n")
	run("add", ".")
	run("commit", "-m", "later work")
	noAncestorClaim(t, ancestorRefusal(t, fallbackFixture{dir: dir, run: run}, ReattestStateNoOwnedMerge))
}

func TestReattest_AncestorDeletedBranch(t *testing.T) {
	f := ancestorFixture(t)
	f.run("branch", "-D", "bead/"+ancestorBead)
	noAncestorClaim(t, ancestorRefusal(t, f, ReattestStateNoOwnedMerge))
}

func TestReattest_AncestorNotInSpecTip(t *testing.T) {
	f := ancestorFixture(t)
	f.run("checkout", "bead/"+ancestorBead)
	writeFallbackFile(t, f.dir, "new.txt", "not landed\n")
	f.run("add", ".")
	f.run("commit", "-m", "not landed")
	noAncestorClaim(t, ancestorRefusal(t, f, ReattestStateNoOwnedMerge))
}

func TestReattest_AncestorInsideOtherRangeChecksContent(t *testing.T) {
	for _, revert := range []bool{false, true} {
		t.Run(fmt.Sprint(revert), func(t *testing.T) {
			dir, run := initLandedRepo(t, "132-test")
			run("checkout", "-b", "bead/mindspec-other")
			writeFallbackFile(t, dir, "work.txt", "prerequisite work\n")
			run("add", ".")
			run("commit", "-m", "interior work")
			tip := revParseIn(t, dir, "HEAD")
			run("branch", "bead/"+ancestorBead, tip)
			writeFallbackFile(t, dir, "other.txt", "other work\n")
			run("add", ".")
			run("commit", "-m", "other work")
			run("checkout", ancestorSpec)
			run("merge", "--no-ff", "-m", "Merge bead/mindspec-other", "bead/mindspec-other")
			f := fallbackFixture{dir: dir, run: run, second: tip}
			state := ReattestStateAncestor
			if revert {
				f.run("revert", "--no-edit", f.second)
				state = ReattestStateNoOwnedMerge
			}
			r := ancestorRefusal(t, f, state)
			if strings.Contains(r.Detail, "not "+ancestorBead+"'s own work") {
				t.Fatal("interior tip was treated as another bead's exact tip")
			}
			if revert && !strings.Contains(r.Detail, "cannot show") {
				t.Fatal(r.Detail)
			}
		})
	}
}

func TestReattest_AncestorOtherBeadTipNotOwn(t *testing.T) {
	for _, shape := range []string{"restored", "zero-commit"} {
		t.Run(shape, func(t *testing.T) {
			f := ancestorFixture(t)
			otherTip := revParseIn(t, f.dir, "bead/mindspec-carrier")
			if shape == "zero-commit" {
				f.run("branch", "-D", "bead/"+ancestorBead)
				f.run("branch", "bead/"+ancestorBead, otherTip)
			} else {
				f.run("branch", "-f", "bead/"+ancestorBead, otherTip)
			}
			r := ancestorRefusal(t, f, ReattestStateNoOwnedMerge)
			if !strings.Contains(r.Detail, "not "+ancestorBead+"'s own work") || !r.NoRestore || len(r.Recovery) > 0 {
				t.Fatalf("wrong work must offer no override: %+v", r)
			}
			assertAncestorNoOwnedDetail(t, r)
		})
	}
}

func TestReattest_AncestorEarlierCommitReverted(t *testing.T) {
	dir, run := initLandedRepo(t, "132-test")
	run("checkout", "-b", "bead/"+ancestorBead)
	writeFallbackFile(t, dir, "early.txt", "early work\n")
	run("add", ".")
	run("commit", "-m", "early work")
	early := revParseIn(t, dir, "HEAD")
	writeFallbackFile(t, dir, "last.txt", "last work\n")
	run("add", ".")
	run("commit", "-m", "last work")
	run("checkout", ancestorSpec)
	run("merge", "--no-ff", "-m", "Import work", "bead/"+ancestorBead)
	run("revert", "--no-edit", early)
	r := ancestorRefusal(t, fallbackFixture{dir: dir, run: run}, ReattestStateNoOwnedMerge)
	if !strings.Contains(r.Detail, "cannot show") {
		t.Fatal(r.Detail)
	}
}

func TestReattest_AncestorRevertedHistoryOnly(t *testing.T) {
	f := ancestorFixture(t)
	f.run("revert", "--no-edit", f.second)
	r := ancestorRefusal(t, f, ReattestStateNoOwnedMerge)
	if !strings.Contains(r.Detail, "are in the spec history") || !strings.Contains(r.Detail, "cannot show that their changes are present") || strings.Contains(r.Detail, "changes are present on") || !r.NoRestore {
		t.Fatalf("wrong history evidence: %+v", r)
	}
	assertAncestorNoOwnedDetail(t, r)
}

func assertAncestorNoOwnedDetail(t *testing.T, r *ReattestRefusal) {
	t.Helper()
	for _, want := range []string{"no first-parent or admitted reachable merge", "attested restore", "only after a person verifies the merge"} {
		if !strings.Contains(r.Detail, want) {
			t.Errorf("no-owned detail missing %q: %s", want, r.Detail)
		}
	}
}

// The two checks must each refuse even when the other check passes.
func TestReattest_AncestorContentAndRevertChecks(t *testing.T) {
	for _, tc := range []struct {
		name     string
		outcome  gitutil.Subsumption
		reverted bool
	}{
		{"revert-alone", gitutil.SubsumptionLanded, true},
		{"content-alone", gitutil.SubsumptionCleanDivergence, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := ancestorFixture(t)
			content, revert := ancestorContentFn, ancestorRangeRevertedFn
			t.Cleanup(func() { ancestorContentFn, ancestorRangeRevertedFn = content, revert })
			contentCalls, revertCalls := 0, 0
			ancestorContentFn = func(root, base, head, target string) (gitutil.Subsumption, error) {
				contentCalls++
				if root != f.dir || head != f.second || target != f.merge || base != revParseIn(t, f.dir, f.merge+"^1") {
					t.Fatalf("wrong content range: %s %s %s %s", root, base, head, target)
				}
				return tc.outcome, nil
			}
			ancestorRangeRevertedFn = func(root, base, head, target string) (bool, error) {
				revertCalls++
				if root != f.dir || head != f.second || target != f.merge || base != revParseIn(t, f.dir, f.merge+"^1") {
					t.Fatalf("wrong revert range: %s %s %s %s", root, base, head, target)
				}
				return tc.reverted, nil
			}
			r := ancestorRefusal(t, f, ReattestStateNoOwnedMerge)
			if contentCalls != 1 || revertCalls != 1 || !strings.Contains(r.Detail, "cannot show that their changes are present") || strings.Contains(r.Detail, "changes are present on") {
				t.Fatalf("unsafe content claim: content calls=%d revert calls=%d refusal=%+v", contentCalls, revertCalls, r)
			}
		})
	}
}

func TestReattest_AncestorNoBaseUnknown(t *testing.T) {
	dir, run := initLandedRepo(t, "132-test")
	run("checkout", "--orphan", "bead/"+ancestorBead)
	writeFallbackFile(t, dir, "orphan.txt", "unrelated work\n")
	run("add", ".")
	run("commit", "-m", "unrelated root")
	run("checkout", ancestorSpec)
	run("merge", "--no-ff", "--allow-unrelated-histories", "-m", "Import unrelated work", "bead/"+ancestorBead)
	r := ancestorRefusal(t, fallbackFixture{dir: dir, run: run}, ReattestStateUnknown)
	if !strings.Contains(r.Detail, "unknown") || !r.NoRestore || len(r.Recovery) > 0 {
		t.Fatalf("unknown base must offer no override: %+v", r)
	}
	noAncestorClaim(t, r)
}

func TestReattest_AncestorReadErrorIndeterminate(t *testing.T) {
	for _, seam := range []string{"tip", "ancestry", "content", "reverse"} {
		t.Run(seam, func(t *testing.T) {
			f := ancestorFixture(t)
			read, anc, content, reverse := landedRevParseRefFn, isAncestorFn, ancestorContentFn, ancestorRangeRevertedFn
			t.Cleanup(func() {
				landedRevParseRefFn = read
				isAncestorFn = anc
				ancestorContentFn = content
				ancestorRangeRevertedFn = reverse
			})
			injected := errors.New("read failed\x1b[31m")
			state := ReattestStateIndeterminate
			switch seam {
			case "tip":
				landedRevParseRefFn = func(string, string) (string, error) { return "", injected }
			case "ancestry":
				isAncestorFn = func(string, string, string) (bool, error) { return false, injected }
			case "content":
				ancestorContentFn = func(string, string, string, string) (gitutil.Subsumption, error) {
					return gitutil.SubsumptionLanded, injected
				}
				state = ReattestStateNoOwnedMerge
			case "reverse":
				ancestorRangeRevertedFn = func(string, string, string, string) (bool, error) { return false, injected }
				state = ReattestStateNoOwnedMerge
			}
			r := ancestorRefusal(t, f, state)
			if !strings.Contains(r.Detail, "indeterminate") || strings.Contains(r.Detail, "\x1b") || strings.Contains(r.Detail, "changes are present on") || !r.NoRestore {
				t.Fatalf("unsafe read refusal: %+v", r)
			}
		})
	}
}

func TestReattest_AncestorRecoveryPerDependent(t *testing.T) {
	f := ancestorFixture(t)
	// Use the real dependency reader with hermetic tracker reads.
	old := ancestorDependentsFn
	t.Cleanup(func() { ancestorDependentsFn = old })
	ancestorDependentsFn = ancestorDependents
	stubAncestorTracker(t)
	r := ancestorRefusal(t, f, ReattestStateAncestor)
	want := []string{"mindspec next mindspec-dep1 --allow-not-ready=MF-3 --evidence <file>", "mindspec next mindspec-dep2 --allow-not-ready=MF-3 --evidence <file>"}
	if !reflect.DeepEqual(r.Recovery, want) {
		t.Fatalf("recovery = %v, want %v", r.Recovery, want)
	}
}

func TestReattest_AncestorProductionDefaultRecovery(t *testing.T) {
	f := ancestorFixture(t)
	old := ancestorDependentsFn
	t.Cleanup(func() { ancestorDependentsFn = old })
	ancestorDependentsFn = ancestorDependentsFnDefault
	stubAncestorTracker(t)
	r := ancestorRefusal(t, f, ReattestStateAncestor)
	want := []string{"mindspec next mindspec-dep1 --allow-not-ready=MF-3 --evidence <file>", "mindspec next mindspec-dep2 --allow-not-ready=MF-3 --evidence <file>"}
	if !reflect.DeepEqual(r.Recovery, want) {
		t.Fatalf("production default recovery = %v, want %v", r.Recovery, want)
	}
}

func TestReattest_AncestorZeroDependents(t *testing.T) {
	f := ancestorFixture(t)
	r := ancestorRefusal(t, f, ReattestStateAncestor)
	if !strings.Contains(r.Detail, "No bead waits on "+ancestorBead) || !reflect.DeepEqual(r.Recovery, []string{"mindspec doctor"}) {
		t.Fatalf("zero dependents: %+v", r)
	}
}

func stubAncestorTracker(t *testing.T) {
	t.Helper()
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "--type=epic") {
			return []byte(`[{"id":"mindspec-epic","issue_type":"epic","metadata":{"spec_num":132,"spec_title":"test"}}]`), nil
		}
		return nil, fmt.Errorf("unexpected list read %v", args)
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		switch args[1] {
		case ancestorBead:
			return []byte(`[{"id":"mindspec-ab12","dependencies":[{"id":"mindspec-epic","issue_type":"epic","dependency_type":"parent-child"}]}]`), nil
		case "mindspec-epic":
			return []byte(`[{"id":"mindspec-epic","issue_type":"epic","metadata":{"spec_num":132,"spec_title":"test"}}]`), nil
		default:
			return nil, fmt.Errorf("unexpected tracker read %v", args)
		}
	}))
	oldChildren, oldIssue := ancestorChildrenFn, ancestorIssueFn
	t.Cleanup(func() { ancestorChildrenFn = oldChildren; ancestorIssueFn = oldIssue })
	ancestorChildrenFn = func(epic string) ([]phase.ChildInfo, error) {
		if epic != "mindspec-epic" {
			t.Fatalf("wrong epic %s", epic)
		}
		return []phase.ChildInfo{{ID: ancestorBead, Status: "closed"}, {ID: "mindspec-dep1", Status: "open"}, {ID: "mindspec-dep2", Status: "in_progress"}, {ID: "mindspec-done", Status: "closed"}, {ID: "mindspec-free", Status: "open"}, {ID: "mindspec-foreign", Status: "open", Parent: "mindspec-other"}, {ID: "mindspec-dep1", Status: "open"}}, nil
	}
	ancestorIssueFn = func(args ...string) ([]byte, error) {
		if args[1] == "mindspec-foreign" {
			t.Fatal("read a dependent from another epic")
		}
		dep := ancestorBead
		kind := "blocks"
		if args[1] == "mindspec-free" {
			kind = "related"
		}
		return []byte(fmt.Sprintf(`[{"id":%q,"dependencies":[{"id":%q,"dependency_type":%q}]}]`, args[1], dep, kind)), nil
	}
}

func TestScanIntegrityFindings_ClosedPrerequisite(t *testing.T) {
	f := ancestorFixture(t)
	stubAncestorTracker(t)
	old := ancestorDependentsFn
	t.Cleanup(func() { ancestorDependentsFn = old })
	ancestorDependentsFn = ancestorDependents
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "--type=epic") {
			return []byte(`[{"id":"mindspec-epic","status":"in_progress","issue_type":"epic","metadata":{"spec_num":132,"spec_title":"test"}}]`), nil
		}
		if strings.Contains(strings.Join(args, " "), "--status=open,in_progress") {
			return []byte(`[]`), nil
		}
		return nil, fmt.Errorf("unexpected list read %v", args)
	}))
	got := ScanIntegrityFindings(f.dir, phase.NewCache()).Ancestors
	if len(got) != 1 || got[0].BeadID != ancestorBead || len(got[0].Recovery) != 2 {
		t.Fatalf("scan lost prerequisite: %+v", got)
	}
	stubBindingRead(t, map[string]interface{}{"mindspec_landed_merge_sha": f.merge})
	if got := ScanIntegrityFindings(f.dir, phase.NewCache()).Ancestors; len(got) != 0 {
		t.Fatalf("bound bead reported: %+v", got)
	}
}
