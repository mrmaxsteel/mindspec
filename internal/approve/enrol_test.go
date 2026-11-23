package approve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/validate"
)

func enrolWrite(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}
func enrolPlan(n int) string { return kerneltest.EnrolmentPlan("001-x", n) }
func enrolInstall(t *testing.T, fake *kerneltest.FakeBD) {
	t.Helper()
	hook := enrolStageHook
	enrolStageHook = nil
	t.Cleanup(func() { enrolStageHook = hook })
	t.Cleanup(SetPlanRunBDForTest(fake.Run))
	t.Cleanup(phase.SetRunBDForTest(fake.Run))
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		out, err := fake.Run(append([]string{"list"}, args...)...)
		if err != nil {
			return nil, err
		}
		if slices.Contains(args, "--type=epic") {
			var rows []kernel.Record
			if err = json.Unmarshal(out, &rows); err != nil {
				return nil, err
			}
			epics := []kernel.Record{}
			for _, r := range rows {
				if r.Type == "epic" {
					epics = append(epics, r)
				}
			}
			return json.Marshal(epics)
		}
		return out, nil
	}))
	t.Cleanup(SetLifecycleMergeMetadataForTest(func(id string, updates map[string]interface{}) error {
		r := fake.Records[id]
		if r.Metadata == nil {
			r.Metadata = map[string]any{}
		}
		for k, v := range updates {
			r.Metadata[k] = v
		}
		fake.Records[id] = r
		return nil
	}))
	// The validator has its own process boundary. Keep it on a local fake too.
	bin := t.TempDir()
	enrolWrite(t, filepath.Join(bin, "bd"), "#!/bin/sh\nif [ \"$1\" = show ] || [ \"$1\" = export ]; then exit 0; fi\nexit 90\n")
	if err := os.Chmod(filepath.Join(bin, "bd"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	config.ResetCache()
	t.Cleanup(config.ResetCache)
}
func enrolFixture(t *testing.T) *kerneltest.Fixture {
	t.Helper()
	// Use the shared repository builder with an entirely local fake store.
	f := kerneltest.PreKernelSpec(t, kerneltest.WithFake(kernel.NativeIDs))
	f.SpecID = "001-x"
	f.SpecDir = filepath.Join(f.Root, ".mindspec", "specs", f.SpecID)
	f.Items = nil
	for id, r := range f.Fake.Records {
		if id != f.Epic {
			delete(f.Fake.Records, id)
			continue
		}
		r.Metadata = map[string]any{"spec_id": f.SpecID, "mindspec_phase": "plan"}
		r.Title = "[SPEC 001-x] kernel fixture"
		r.Labels = nil
		f.Fake.Records[id] = r
	}
	enrolWrite(t, filepath.Join(f.SpecDir, "plan.md"), enrolPlan(3))
	enrolWrite(t, filepath.Join(f.SpecDir, "spec.md"), "---\nstatus: Approved\n---\n# Example\n")
	enrolWrite(t, filepath.Join(f.Root, ".mindspec", "config.yaml"), "enforcement:\n  panel_gate: false\n")
	enrolWrite(t, filepath.Join(f.Root, ".beads", "config.yaml"), "types.custom: gate,step\n")
	enrolInstall(t, f.Fake)
	return f
}
func enrolApprove(f *kerneltest.Fixture) (*PlanResult, error) {
	return ApprovePlan(f.Root, f.SpecID, "operator", &executor.MockExecutor{})
}
func enrolIDs(t *testing.T, f *kerneltest.Fixture) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.SpecDir, "plan.md"))
	f.Must(err)
	fm, err := validate.ParsePlanFrontmatter(string(data))
	f.Must(err)
	return fm.BeadIDs
}
func enrolSnap(t *testing.T, f *kerneltest.Fixture) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.SpecDir, "plan.md"))
	f.Must(err)
	return kerneltest.Snapshot(t, f) + "\n" + string(data)
}
func enrolAssertRefusal(t *testing.T, f *kerneltest.Fixture, want string) {
	t.Helper()
	before := enrolSnap(t, f)
	_, err := enrolApprove(f)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("want refusal %q, got %v", want, err)
	}
	if after := enrolSnap(t, f); after != before {
		t.Fatal("refusal changed snapshot")
	}
}
func TestS134Enrol_CreatesAndRefusesSecondApproval(t *testing.T) {
	f := enrolFixture(t)
	result, err := enrolApprove(f)
	f.Must(err)
	if !result.Enrolled {
		t.Fatal("new approval did not report enrollment")
	}
	if len(result.BeadIDs) != 3 {
		t.Fatalf("items: %v", result.BeadIDs)
	}
	epic := f.Fake.Records[f.Epic]
	links, ok, err := kernel.Decode[kernel.Links](epic.Metadata, kernel.KeyLinks)
	f.Must(err)
	if !ok || len(links) != 3 || !reflect.DeepEqual(links, enrolLinks(result.BeadIDs, []validate.WorkChunk{{ID: 1}, {ID: 2, DependsOn: []int{1}}, {ID: 3, DependsOn: []int{1}}})) {
		t.Fatalf("links: %+v", links)
	}
	policy, ok, err := kernel.Decode[kernel.Policy](epic.Metadata, kernel.KeyRun)
	f.Must(err)
	if !ok || policy != kernel.DefaultPolicy {
		t.Fatalf("policy: %+v", policy)
	}
	approval, ok, err := kernel.Decode[kernel.PlanApproval](epic.Metadata, kernel.KeyPlanApproval)
	f.Must(err)
	if !ok || !bool(approval) {
		t.Fatal("missing approval")
	}
	for _, id := range append([]string{f.Epic}, result.BeadIDs...) {
		r := f.Fake.Records[id]
		if !strings.HasPrefix(r.Title, "[mindspec]") || !slices.Contains(r.Labels, "mindspec") {
			t.Fatalf("unmarked %s: %+v", id, r)
		}
	}
	for _, id := range result.BeadIDs {
		gate, err := f.Store.Lookup(id + ".review")
		f.Must(err)
		if gate == nil || gate.Parent != id || gate.Status != "open" {
			t.Fatalf("gate: %+v", gate)
		}
		if _, err = f.Fake.Run("close", id); err == nil {
			t.Fatal("raw close ignored open review child")
		}
	}
	if len(f.Fake.Records) != 8 {
		t.Fatalf("extra or missing records: %d", len(f.Fake.Records))
	}
	enrolAssertRefusal(t, f, "enrolment")
}

// enrolShape compares every kernel metadata key, dependency and gate field. Only
// physical identities differ between fixtures; the producer's work text is separate.
func enrolShape(t *testing.T, f *kerneltest.Fixture, items []string) string {
	t.Helper()
	all, err := f.Store.All()
	f.Must(err)
	owned := []kernel.Record{}
	for _, r := range all {
		if r.ID == f.Epic || slices.Contains(items, r.ID) || r.Metadata[kernel.KeyTuple] != nil {
			owned = append(owned, r)
		}
	}
	data, err := json.Marshal(owned)
	f.Must(err)
	text := string(data)
	// Replace longest physical identities first, and use computed keys for gates.
	replacements := [][2]string{{f.Epic, "EPIC"}}
	for i, id := range items {
		replacements = append(replacements, [2]string{id, fmt.Sprintf("ITEM%d", i+1)})
	}
	slices.SortFunc(replacements, func(a, b [2]string) int { return len(b[0]) - len(a[0]) })
	for _, p := range replacements {
		text = strings.ReplaceAll(text, p[0], p[1])
	}
	return text
}
func cloneEnrolMetadata(t *testing.T, in map[string]any) map[string]any {
	t.Helper()
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err = json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func TestS134Enrol_FixtureMatches(t *testing.T) {
	actual := enrolFixture(t)
	result, err := enrolApprove(actual)
	actual.Must(err)
	expected := kerneltest.Enrolled(t, kerneltest.WithFake(kernel.NativeIDs))
	want := enrolShape(t, expected, expected.Items)
	got := enrolShape(t, actual, result.BeadIDs)
	if got != want {
		t.Fatalf("enrolment differs from shared fixture\nwant %s\ngot  %s", want, got)
	}
	// Prove that the comparator observes changes in every governed class.
	for _, mutation := range []string{"key", "link", "gate"} {
		t.Run(mutation, func(t *testing.T) {
			twin := kerneltest.Enrolled(t, kerneltest.WithFake(kernel.NativeIDs))
			baseline := enrolShape(t, twin, twin.Items)
			switch mutation {
			case "key":
				r := twin.Fake.Records[twin.Epic]
				r.Metadata[kernel.KeyRun] = kernel.Policy{RoundLimit: 99}
				twin.Fake.Records[r.ID] = r
			case "link":
				r := twin.Fake.Records[twin.Items[1]]
				r.Dependencies = nil
				twin.Fake.Records[r.ID] = r
			case "gate":
				delete(twin.Fake.Records, twin.ID(twin.Items[0]+".review"))
			}
			if enrolShape(t, twin, twin.Items) == baseline {
				t.Fatal("comparator missed mutation")
			}
		})
	}
}
func TestS134Enrol_CycleRefusalAndForeignChildRetry(t *testing.T) {
	f := enrolFixture(t)
	f.Fake.Hook = func(args []string) error {
		if len(args) > 1 && args[0] == "dep" && args[1] == "add" {
			return fmt.Errorf("cycle refused")
		}
		return nil
	}
	_, err := enrolApprove(f)
	if err == nil || !strings.Contains(err.Error(), "link "+f.Epic+".2 -> "+f.Epic+".1") {
		t.Fatalf("wrong refusal: %v", err)
	}
	if len(f.Fake.Records) != 4 {
		t.Fatal("gate created before cycle refusal")
	}
	epic := f.Fake.Records[f.Epic]
	if epic.Metadata[kernel.KeyRun] != nil || epic.Metadata[kernel.KeyPlanApproval] != nil {
		t.Fatal("cycle wrote enrolment or approval")
	}
	f.Fake.Hook = nil
	out, err := f.Fake.Run("create", "--title", "foreign", "--type", "task", "--parent", f.Epic, "--json")
	f.Must(err)
	var foreign kernel.Record
	f.Must(json.Unmarshal(out, &foreign))
	before, err := json.Marshal(f.Fake.Records[foreign.ID])
	f.Must(err)
	result, err := enrolApprove(f)
	f.Must(err)
	after, err := json.Marshal(f.Fake.Records[foreign.ID])
	f.Must(err)
	if len(result.BeadIDs) != 3 || string(after) != string(before) || len(f.Fake.Records) != 9 {
		t.Fatal("retry changed foreign child or doubled work")
	}
}

func enrolKill(t *testing.T, f *kerneltest.Fixture, stage string) {
	t.Helper()
	storePath := filepath.Join(f.Root, "fake-store.json")
	data, err := json.Marshal(f.Fake.Records)
	f.Must(err)
	enrolWrite(t, storePath, string(data))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	t.Setenv("S134_UNEXPECTED", "caller-only")
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run", "^TestS134EnrolHelper$")
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + f.Root, "TMPDIR=" + t.TempDir(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/usr/bin/false", "S134_KILL_AT=" + stage, "S134_ROOT=" + f.Root, "S134_EPIC=" + f.Epic}
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 137 {
		t.Fatalf("helper did not die at %s: %v %s", stage, err, out)
	}
	data, err = os.ReadFile(storePath)
	f.Must(err)
	f.Must(json.Unmarshal(data, &f.Fake.Records))
}
func TestS134EnrolHelper(t *testing.T) {
	stage := os.Getenv("S134_KILL_AT")
	if stage == "" {
		return
	}
	if os.Getenv("S134_UNEXPECTED") != "" {
		t.Fatal("helper inherited caller environment")
	}
	root, epic := os.Getenv("S134_ROOT"), os.Getenv("S134_EPIC")
	fake := kerneltest.NewFakeBD()
	data, err := os.ReadFile(filepath.Join(root, "fake-store.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fake.Records); err != nil {
		t.Fatal(err)
	}
	enrolInstall(t, fake)
	planRunBDFn = func(args ...string) ([]byte, error) {
		out, err := fake.Run(args...)
		data, e := json.Marshal(fake.Records)
		if e != nil {
			t.Fatal(e)
		}
		enrolWrite(t, filepath.Join(root, "fake-store.json"), string(data))
		return out, err
	}
	enrolStageHook = func(s string) {
		if s == stage {
			os.Exit(137)
		}
	}
	f := &kerneltest.Fixture{Root: root, SpecID: "001-x", SpecDir: filepath.Join(root, ".mindspec", "specs", "001-x"), Epic: epic, Fake: fake}
	if _, err = enrolApprove(f); err != nil {
		t.Fatal(err)
	}
	t.Fatal("kill stage not reached")
}
func TestS134Enrol_KillCreatesForeignCycleNoLinkWrite(t *testing.T) {
	f := enrolFixture(t)
	enrolKill(t, f, "creates")
	ids := enrolIDs(t, f)
	out, err := f.Fake.Run("create", "--title", "foreign bug", "--type", "bug", "--json")
	f.Must(err)
	var bug kernel.Record
	f.Must(json.Unmarshal(out, &bug))
	_, err = f.Fake.Run("dep", "add", ids[0], bug.ID, "--type", "blocks")
	f.Must(err)
	_, err = f.Fake.Run("dep", "add", bug.ID, ids[2], "--type", "blocks")
	f.Must(err)
	f.Fake.Calls = nil
	_, err = enrolApprove(f)
	if err == nil || !strings.Contains(err.Error(), "bd dep remove "+ids[0]+" "+bug.ID) {
		t.Fatalf("missing chain recovery: %v", err)
	}
	for _, call := range f.Fake.Calls {
		if len(call) > 1 && call[0] == "dep" && call[1] == "add" {
			t.Fatalf("cycle wrote link: %v", call)
		}
	}
}
func enrolEditPlan(t *testing.T, f *kerneltest.Fixture, edit func(string) string) {
	t.Helper()
	p := filepath.Join(f.SpecDir, "plan.md")
	data, err := os.ReadFile(p)
	f.Must(err)
	enrolWrite(t, p, edit(string(data)))
}
func TestS134Enrol_KillLinksRejectsPlanEdits(t *testing.T) {
	for _, tc := range []struct{ name, old, new string }{
		{"drop", "id: 2\n    depends_on: [1]", "id: 2\n    depends_on: []"},
		{"add", "id: 3\n    depends_on: [1]", "id: 3\n    depends_on: [1, 2]"},
		{"text", "Build the item.", "Build a different item."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := enrolFixture(t)
			enrolKill(t, f, "links")
			if tc.name == "text" {
				enrolEditPlan(t, f, func(s string) string { return strings.Replace(s, tc.old, tc.new, 1) })
			} else if tc.name == "drop" {
				enrolSetDeps(t, f, 2, nil)
			} else {
				enrolSetDeps(t, f, 3, []int{1, 2})
			}
			ids := enrolIDs(t, f)
			want := map[string]string{
				"drop": "plan-order upstream link differs for " + ids[1] + ": ",
				"add":  "plan-order upstream link differs for " + ids[2] + ": " + ids[0] + "," + ids[1],
				"text": "chunk text differs for " + ids[0],
			}[tc.name]
			enrolAssertExactRefusal(t, f, want)
		})
	}
}
func enrolAddFourth(t *testing.T, f *kerneltest.Fixture) {
	t.Helper()
	ids := enrolIDs(t, f)
	enrolWrite(t, filepath.Join(f.SpecDir, "plan.md"), enrolPlan(4))
	f.Must(writeBeadIDsToFrontmatter(filepath.Join(f.SpecDir, "plan.md"), ids))
}
func TestS134Enrol_AddChunkAcrossTwoKills(t *testing.T) {
	f := enrolFixture(t)
	enrolKill(t, f, "links")
	enrolAddFourth(t, f)
	enrolKill(t, f, "links")
	result, err := enrolApprove(f)
	f.Must(err)
	if len(result.BeadIDs) != 4 || len(f.Fake.Records) != 10 {
		t.Fatalf("doubled fourth item: %v", result.BeadIDs)
	}
	fourth := f.Fake.Records[result.BeadIDs[3]]
	if !reflect.DeepEqual(fourth.Dependencies, []kernel.Dependency{{ID: result.BeadIDs[0], Type: "blocks"}}) {
		t.Fatalf("fourth links: %+v", fourth.Dependencies)
	}
}
func TestS134Enrol_FourthChunkLinkEditsRefused(t *testing.T) {
	for _, deps := range []string{"[2]", "[]"} {
		t.Run(deps, func(t *testing.T) {
			f := enrolFixture(t)
			enrolKill(t, f, "links")
			enrolAddFourth(t, f)
			enrolKill(t, f, "links")
			if deps == "[2]" {
				enrolSetDeps(t, f, 4, []int{2})
			} else {
				enrolSetDeps(t, f, 4, nil)
			}
			ids := enrolIDs(t, f)
			upstream := ""
			if deps == "[2]" {
				upstream = ids[1]
			}
			enrolAssertExactRefusal(t, f, "plan-order upstream link differs for "+ids[3]+": "+upstream)
		})
	}
}
func TestS134Enrol_PinPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, config string
		limit        int
		ttl          string
	}{{"default", "", 5, "2h"}, {"panel", "panel:\n  max_rounds: 3\n  lease_ttl: 30m\n", 3, "30m"}, {"alias", "loop:\n  halt:\n    max_rounds_per_bead: 4\n", 4, "2h"}} {
		t.Run(tc.name, func(t *testing.T) {
			f := enrolFixture(t)
			enrolWrite(t, filepath.Join(f.Root, ".mindspec", "config.yaml"), "enforcement:\n  panel_gate: false\n"+tc.config)
			config.ResetCache()
			_, err := enrolApprove(f)
			f.Must(err)
			p, ok, err := kernel.Decode[kernel.Policy](f.Fake.Records[f.Epic].Metadata, kernel.KeyRun)
			f.Must(err)
			if !ok || p.RoundLimit != tc.limit || p.LeaseTTL != tc.ttl || p.Rules != "kernel-1" {
				t.Fatalf("pin: %+v", p)
			}
		})
	}
}
func TestS134Enrol_RequireStep(t *testing.T) {
	f := enrolFixture(t)
	enrolWrite(t, filepath.Join(f.Root, ".beads", "config.yaml"), "types.custom: gate,spike2\n")
	enrolAssertRefusal(t, f, "mindspec setup")
}
func TestS134Enrol_PreKernelForeignChildUnchanged(t *testing.T) {
	f := kerneltest.PreKernelSpec(t, kerneltest.WithFake(kernel.NativeIDs))
	enrolInstall(t, f.Fake)
	// Supply a valid approved document without introducing any kernel witness.
	text := strings.ReplaceAll(enrolPlan(3), "001-x", f.SpecID)
	text = strings.Replace(text, "status: Draft", "status: Approved", 1)
	enrolWrite(t, filepath.Join(f.SpecDir, "plan.md"), text)
	f.Must(writeBeadIDsToFrontmatter(filepath.Join(f.SpecDir, "plan.md"), f.Items))
	enrolWrite(t, filepath.Join(f.Root, ".mindspec", "config.yaml"), "enforcement:\n  panel_gate: false\n")
	_, err := f.Fake.Run("create", "--title", "foreign", "--type", "task", "--parent", f.Epic, "--json")
	f.Must(err)
	before := enrolSnap(t, f)
	result, err := enrolApprove(f)
	f.Must(err)
	if !reflect.DeepEqual(result.BeadIDs, f.Items) || enrolSnap(t, f) != before {
		t.Fatal("pre-kernel rerun mutated records")
	}
	if result.Enrolled || f.State().Enrolled {
		t.Fatal("pre-kernel rerun enrolled spec")
	}
}

func enrolSetDeps(t *testing.T, f *kerneltest.Fixture, n int, deps []int) {
	t.Helper()
	f.Must(mutateFrontmatterFile(filepath.Join(f.SpecDir, "plan.md"), func(fm map[string]interface{}) {
		chunks := fm["work_chunks"].([]interface{})
		for _, raw := range chunks {
			chunk := raw.(map[string]interface{})
			if chunk["id"] == n {
				values := []interface{}{}
				for _, dep := range deps {
					values = append(values, dep)
				}
				chunk["depends_on"] = values
			}
		}
	}))
}

func TestS134Enrol_UnrecordedLinksTakeEditedPlan(t *testing.T) {
	f := enrolFixture(t)
	enrolKill(t, f, "creates")
	enrolSetDeps(t, f, 3, []int{2})
	result, err := enrolApprove(f)
	f.Must(err)
	if !reflect.DeepEqual(f.Fake.Records[result.BeadIDs[2]].Dependencies, []kernel.Dependency{{ID: result.BeadIDs[1], Type: "blocks"}}) {
		t.Fatal("unwritten links did not take edited plan")
	}
}
func TestS134Enrol_NoRecordedItemDiscardsStaleSnapshot(t *testing.T) {
	f := enrolFixture(t)
	enrolKill(t, f, "links")
	for _, id := range enrolIDs(t, f) {
		delete(f.Fake.Records, id)
	}
	out, err := f.Fake.Run("create", "--title", "foreign", "--type", "task", "--parent", f.Epic, "--json")
	f.Must(err)
	var child kernel.Record
	f.Must(json.Unmarshal(out, &child))
	// The foreign child can reuse an old numeric ID. Use a different explicit ID.
	delete(f.Fake.Records, child.ID)
	child.ID = f.Epic + ".99"
	f.Fake.Records[child.ID] = child
	result, err := enrolApprove(f)
	f.Must(err)
	if len(result.BeadIDs) != 3 || !reflect.DeepEqual(f.Fake.Records[child.ID], child) {
		t.Fatal("stale records selected a foreign child")
	}
}
func TestS134Enrol_RemovedRecordedChunkRefusesBeforeWrite(t *testing.T) {
	f := enrolFixture(t)
	enrolKill(t, f, "links")
	ids := enrolIDs(t, f)
	enrolWrite(t, filepath.Join(f.SpecDir, "plan.md"), enrolPlan(2))
	f.Must(writeBeadIDsToFrontmatter(filepath.Join(f.SpecDir, "plan.md"), ids))
	enrolAssertRefusal(t, f, "no chunk")
}
func TestS134Enrol_PinnedPolicySurvivesInterruptedRetry(t *testing.T) {
	f := enrolFixture(t)
	enrolKill(t, f, "run")
	enrolWrite(t, filepath.Join(f.Root, ".mindspec", "config.yaml"), "enforcement:\n  panel_gate: false\npanel:\n  max_rounds: 99\n  lease_ttl: 10m\n")
	config.ResetCache()
	_, err := enrolApprove(f)
	f.Must(err)
	policy, _, err := kernel.Decode[kernel.Policy](f.Fake.Records[f.Epic].Metadata, kernel.KeyRun)
	f.Must(err)
	if policy != kernel.DefaultPolicy {
		t.Fatalf("retry repinned policy: %+v", policy)
	}
}

func enrolAssertExactRefusal(t *testing.T, f *kerneltest.Fixture, detail string) {
	t.Helper()
	before := enrolSnap(t, f)
	_, err := enrolApprove(f)
	want := "plan changed: " + detail + "; restore the recorded plan before retrying approval\nrecovery: mindspec plan approve " + f.SpecID
	if err == nil || err.Error() != want {
		t.Fatalf("want refusal %q, got %v", want, err)
	}
	if enrolSnap(t, f) != before {
		t.Fatal("refusal changed snapshot")
	}
}

func TestS134Enrol_PanelWitnessBeforeRunPinsPolicy(t *testing.T) {
	f := enrolFixture(t)
	enrolWrite(t, filepath.Join(f.Root, ".mindspec", "config.yaml"), "enforcement:\n  panel_gate: false\npanel:\n  max_rounds: 3\n  lease_ttl: 30m\n")
	config.ResetCache()
	enrolWrite(t, filepath.Join(f.SpecDir, "reviews", "plan-panel", "panel.json"), `{"orchestrated":true}`)
	_, err := enrolApprove(f)
	f.Must(err)
	policy, present, err := kernel.Decode[kernel.Policy](f.Fake.Records[f.Epic].Metadata, kernel.KeyRun)
	f.Must(err)
	if !present || policy != (kernel.Policy{RoundLimit: 3, LeaseTTL: "30m", Rules: "kernel-1"}) {
		t.Fatalf("missing pinned policy: present=%v policy=%+v", present, policy)
	}
}

func TestS134Enrol_PanelWitnessBeforeRunRefusesCycle(t *testing.T) {
	for _, check := range []bool{false, true} {
		t.Run(fmt.Sprint("check=", check), func(t *testing.T) {
			f := enrolFixture(t)
			enrolWrite(t, filepath.Join(f.SpecDir, "reviews", "plan-panel", "panel.json"), `{"orchestrated":true}`)
			if check {
				enrolKill(t, f, "creates")
				ids := enrolIDs(t, f)
				_, err := f.Fake.Run("dep", "add", ids[0], ids[1], "--type", "blocks")
				f.Must(err)
			} else {
				f.Fake.Hook = func(args []string) error {
					if len(args) > 1 && args[0] == "dep" && args[1] == "add" {
						return fmt.Errorf("cycle refused")
					}
					return nil
				}
			}
			_, err := enrolApprove(f)
			if err == nil || !strings.Contains(err.Error(), "link "+f.Epic+".2 -> "+f.Epic+".1") {
				t.Fatalf("want cycle refusal, got %v", err)
			}
			epic := f.Fake.Records[f.Epic]
			if _, ok := epic.Metadata[kernel.KeyRun]; ok {
				t.Fatal("cycle wrote run marker")
			}
			if _, ok := epic.Metadata[kernel.KeyPlanApproval]; ok {
				t.Fatal("cycle wrote approval")
			}
			if len(f.Fake.Records) != 4 {
				t.Fatalf("cycle wrote gates: %d records", len(f.Fake.Records))
			}
		})
	}
}

func TestS134Enrol_RawFinalCollisionBeforeWrite(t *testing.T) {
	f := enrolFixture(t)
	_, err := f.Fake.Run("create", "--title", "foreign", "--type", "task", "--parent", f.Epic, "--id", f.Epic+".final", "--json")
	f.Must(err)
	for range 2 {
		enrolAssertRefusal(t, f, "id-collision: "+f.Epic+".final")
	}
}

func TestS134Enrol_UnrelatedRecordsDoNotAddCalls(t *testing.T) {
	var baseline []int
	for _, n := range []int{0, 50} {
		f := enrolFixture(t)
		for i := 0; i < n; i++ {
			_, err := f.Fake.Run("create", "--title", fmt.Sprint("unrelated ", i), "--type", "task", "--json")
			f.Must(err)
		}
		f.Fake.Calls = nil
		_, err := enrolApprove(f)
		f.Must(err)
		deps := 0
		for _, call := range f.Fake.Calls {
			if len(call) > 1 && call[0] == "dep" && call[1] == "list" {
				deps++
			}
		}
		counts := []int{len(f.Fake.Calls), deps}
		t.Logf("N=%d total=%d dep-list=%d", n, counts[0], counts[1])
		if baseline == nil {
			baseline = counts
		} else if !reflect.DeepEqual(counts, baseline) {
			t.Fatalf("unrelated records added calls: %v -> %v", baseline, counts)
		}
	}
}

func TestS134Enrol_RunMarkerKeepsPolicyAfterCycleRefusal(t *testing.T) {
	f := enrolFixture(t)
	enrolKill(t, f, "run")
	ids := enrolIDs(t, f)
	item := f.Fake.Records[ids[1]]
	item.Dependencies = nil
	f.Fake.Records[item.ID] = item
	f.Fake.Hook = func(args []string) error {
		if len(args) > 1 && args[0] == "dep" && args[1] == "add" && args[2] == item.ID {
			return fmt.Errorf("cycle refused")
		}
		return nil
	}
	_, err := enrolApprove(f)
	f.Must(err)
	policy, present, err := kernel.Decode[kernel.Policy](f.Fake.Records[f.Epic].Metadata, kernel.KeyRun)
	f.Must(err)
	if !present || policy != kernel.DefaultPolicy {
		t.Fatalf("retry changed pinned policy: present=%v policy=%+v", present, policy)
	}
	if len(f.Fake.Records[item.ID].Dependencies) != 0 {
		t.Fatal("retry wrote refused link")
	}
	if f.Fake.Records[f.Epic].Metadata[kernel.KeyPlanApproval] != true || len(f.Fake.Records) != 8 {
		t.Fatal("post-marker refusal stopped approval or gate creation")
	}
}

func TestS134Enrol_PanelWitnessChainCycleNoLinkWrite(t *testing.T) {
	f := enrolFixture(t)
	enrolWrite(t, filepath.Join(f.SpecDir, "reviews", "plan-panel", "panel.json"), `{"orchestrated":true}`)
	enrolKill(t, f, "creates")
	ids := enrolIDs(t, f)
	out, err := f.Fake.Run("create", "--title", "foreign bug", "--type", "bug", "--json")
	f.Must(err)
	var bug kernel.Record
	f.Must(json.Unmarshal(out, &bug))
	_, err = f.Fake.Run("dep", "add", ids[0], bug.ID, "--type", "blocks")
	f.Must(err)
	_, err = f.Fake.Run("dep", "add", bug.ID, ids[2], "--type", "blocks")
	f.Must(err)
	f.Fake.Calls = nil
	_, err = enrolApprove(f)
	if err == nil || !strings.Contains(err.Error(), "bd dep remove "+ids[0]+" "+bug.ID) {
		t.Fatalf("missing chain recovery: %v", err)
	}
	for _, call := range f.Fake.Calls {
		if len(call) > 1 && call[0] == "dep" && call[1] == "add" {
			t.Fatalf("cycle wrote link: %v", call)
		}
	}
}

func TestS134Enrol_RawReviewCollisionBeforeWrite(t *testing.T) {
	f := enrolFixture(t)
	enrolKill(t, f, "creates")
	ids := enrolIDs(t, f)
	_, err := f.Fake.Run("create", "--title", "foreign", "--type", "task", "--parent", ids[0], "--id", ids[0]+".review", "--json")
	f.Must(err)
	for range 2 {
		enrolAssertRefusal(t, f, "id-collision: "+ids[0]+".review")
	}
}

func TestS134Enrol_BadPanelRefusesBeforeWrite(t *testing.T) {
	f := enrolFixture(t)
	path := filepath.Join(f.SpecDir, "reviews", "old", "panel.json")
	enrolWrite(t, path, `{not json`)
	before := enrolSnap(t, f)
	f.Fake.Calls = nil
	_, err := enrolApprove(f)
	if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("want parse refusal naming %s, got %v", path, err)
	}
	if enrolSnap(t, f) != before {
		t.Fatal("bad panel refusal changed snapshot")
	}
	for _, call := range f.Fake.Calls {
		if call[0] != "list" && call[0] != "show" && !(len(call) > 1 && call[0] == "dep" && call[1] == "list") {
			t.Fatalf("bad panel refusal wrote: %v", call)
		}
	}
}

func TestS134Enrol_NewUpstreamRefusesExactly(t *testing.T) {
	f := enrolFixture(t)
	enrolKill(t, f, "links")
	ids := enrolIDs(t, f)
	enrolAddFourth(t, f)
	enrolSetDeps(t, f, 2, []int{4})
	enrolAssertExactRefusal(t, f, "upstream differs for "+ids[1])
}

func TestS134Enrol_SuccessfulLinkUpdatesCycleCheck(t *testing.T) {
	f := kerneltest.Enrolled(t, kerneltest.WithFake(kernel.NativeIDs))
	graph := enrolWaits{}
	f.Must(enrolAddLink(f.Store, graph, f.Items[2], f.Items[1]))
	f.Fake.Calls = nil
	err := enrolAddLink(f.Store, graph, f.Items[1], f.Items[2])
	want := "foreign-blocker: link " + f.Items[1] + " -> " + f.Items[2] + " forms a cycle through " + f.Items[2] + " -> " + f.Items[1] + "\nrecovery: bd dep remove " + f.Items[2] + " " + f.Items[1]
	if err == nil || err.Error() != want {
		t.Fatalf("want graph refusal %q, got %v", want, err)
	}
	for _, call := range f.Fake.Calls {
		if len(call) > 1 && call[0] == "dep" && call[1] == "add" {
			t.Fatalf("cycle reached link write: %v", call)
		}
	}
}

func TestS134Enrol_CreatedGateUpdatesCycleCheck(t *testing.T) {
	f := kerneltest.Enrolled(t, kerneltest.WithFake(kernel.NativeIDs))
	graph := enrolWaits{}
	item := f.Items[0]
	gate := item + ".review"
	delete(f.Fake.Records, f.ID(gate))
	f.Must(enrolGate(f.Store, graph, gate, item))
	physical := f.ID(gate)
	f.Fake.Calls = nil
	err := enrolAddLink(f.Store, graph, gate, item)
	want := "foreign-blocker: link " + physical + " -> " + item + " forms a cycle through " + item + " -> " + physical + "\nrecovery: bd dep remove " + item + " " + physical
	if err == nil || err.Error() != want {
		t.Fatalf("want graph refusal %q, got %v", want, err)
	}
	for _, call := range f.Fake.Calls {
		if len(call) > 1 && call[0] == "dep" && call[1] == "add" {
			t.Fatalf("child cycle reached link write: %v", call)
		}
	}
}

func TestS134Enrol_FinalGateReadFailureStopsLinks(t *testing.T) {
	f := enrolFixture(t)
	enrolKill(t, f, "creates")
	final := f.Create(f.Epic+".final", f.Epic)
	readErr := errors.New("cannot read final dependencies")
	f.Fake.Hook = func(args []string) error {
		if len(args) > 2 && args[0] == "dep" && args[1] == "list" && args[2] == final.ID {
			return readErr
		}
		return nil
	}
	f.Fake.Calls = nil
	_, err := enrolApprove(f)
	if !errors.Is(err, readErr) {
		t.Fatalf("want final dependency read refusal, got %v", err)
	}
	for _, call := range f.Fake.Calls {
		if len(call) > 1 && call[0] == "dep" && call[1] == "add" {
			t.Fatalf("read refusal wrote link: %v", call)
		}
	}
	if f.Fake.Records[f.Epic].Metadata[kernel.KeyRun] != nil || f.Fake.Records[f.Epic].Metadata[kernel.KeyPlanApproval] != nil {
		t.Fatal("read refusal wrote run marker or approval")
	}
}

func TestS134Enrol_FinalCycleAfterKilledCreate(t *testing.T) {
	f := enrolFixture(t)
	enrolKill(t, f, "final")
	ids := enrolIDs(t, f)
	final := f.Epic + ".final"
	finalID := f.ID(final)
	bug := kernel.Record{ID: "foreign-bug", Status: "open", Type: "bug", Dependencies: []kernel.Dependency{{ID: finalID, Type: "blocks"}}}
	f.Fake.Records[bug.ID] = bug
	third := f.Fake.Records[ids[2]]
	third.Dependencies = append(third.Dependencies, kernel.Dependency{ID: bug.ID, Type: "blocks"})
	f.Fake.Records[third.ID] = third
	out, err := kernel.NextAction(f.State(), true)
	f.Must(err)
	cycle := false
	for _, flag := range out.Flags {
		if flag.Code == "foreign-blocker" && slices.Contains(flag.Repair, finalID) && slices.Contains(flag.Repair, bug.ID) {
			cycle = true
		}
	}
	if !cycle {
		t.Fatalf("missing cycle flag: %+v", out.Flags)
	}
	r, err := f.Store.Lookup(final)
	f.Must(err)
	for _, d := range r.Dependencies {
		if d.ID == third.ID {
			t.Fatal("cyclic final link written")
		}
	}
	bug.Dependencies = nil
	f.Fake.Records[bug.ID] = bug
	out, err = kernel.NextAction(f.State(), true)
	f.Must(err)
	for _, flag := range out.Flags {
		if slices.Contains(flag.Repair, finalID) && slices.Contains(flag.Repair, bug.ID) {
			t.Fatalf("cycle flag retained: %+v", flag)
		}
	}
	r, err = f.Store.Lookup(final)
	f.Must(err)
	found := false
	for _, d := range r.Dependencies {
		found = found || d.ID == third.ID
	}
	if !found {
		t.Fatal("missing final link after removal")
	}
}
