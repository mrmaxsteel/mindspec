package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/approve"
	"github.com/mrmaxsteel/mindspec/internal/harness"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// seedPreKernelPlan retains custom readiness evidence in older lifecycle fixtures.
// The recovery creator writes the same recorded work graph without enrolment.
func seedPreKernelPlan(t *testing.T, sb *harness.Sandbox, spec string) {
	t.Helper()
	run := func(args ...string) ([]byte, error) { out, err := sb.Run("bd", args...); return []byte(out), err }
	list := func(args ...string) ([]byte, error) {
		return run(append(append([]string{"list"}, args...), "--json")...)
	}
	restoreRun := approve.SetPlanRunBDForTest(run)
	defer restoreRun()
	restoreCombined := approve.SetPlanRunBDCombinedForTest(run)
	defer restoreCombined()
	restoreList := approve.SetPlanListJSONForTest(list)
	defer restoreList()
	restorePhaseRun := phase.SetRunBDForTest(run)
	defer restorePhaseRun()
	restorePhaseList := phase.SetListJSONForTest(list)
	defer restorePhaseList()
	result, err := approve.CreateBeadsFromPlan(sb.Root, spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.BeadIDs) == 0 {
		t.Fatal("legacy plan fixture created no recorded work")
	}
	dir, err := workspace.SpecDir(sb.Root, spec)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "plan.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	approved := strings.Replace(string(data), "status: Draft", "status: Approved", 1)
	if approved == string(data) {
		t.Fatal("legacy fixture plan was not a draft")
	}
	if err := os.WriteFile(path, []byte(approved), 0600); err != nil {
		t.Fatal(err)
	}
	tree := workspace.TreeRootForSpecDir(dir)
	for _, args := range [][]string{{"-C", tree, "add", "--", path}, {"-C", tree, "-c", "core.hooksPath=/dev/null", "commit", "-m", "fixture: pre-kernel approved plan"}} {
		if out, err := sb.Run("git", args...); err != nil {
			t.Fatalf("commit legacy plan: %v %s", err, out)
		}
	}
	epic, err := phase.FindEpicBySpecID(spec)
	if err != nil {
		t.Fatal(err)
	}
	state, err := kernel.ReadState(kernel.NewStore(sb.Root, epic, run), spec, dir)
	if err != nil {
		t.Fatal(err)
	}
	if state.Enrolled {
		t.Fatalf("legacy fixture enrolled %s", spec)
	}
}
