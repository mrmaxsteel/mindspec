package phase_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

func TestS134Derive_ClosedRetypedWitnessIgnoresForeignTask(t *testing.T) {
	f := kerneltest.Enrolled(t, kerneltest.WithFake(kernel.NativeIDs))
	epic := f.Fake.Records[f.Epic]
	delete(epic.Metadata, kernel.KeyRun)
	delete(epic.Metadata, "mindspec_phase")
	f.Fake.Records[epic.ID] = epic
	for id, r := range f.Fake.Records {
		if id != f.Epic {
			r.Status = "closed"
			r.Type = "chore"
			f.Fake.Records[id] = r
		}
	}
	_, err := f.Fake.Run("create", "--title", "foreign task", "--type", "task", "--parent", f.Epic, "--json")
	f.Must(err)
	t.Cleanup(phase.SetRunBDForTest(f.Fake.Run))
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) { return f.Fake.Run(append([]string{"list"}, args...)...) }))
	derived, err := phase.DerivePhase(f.Epic)
	f.Must(err)
	if derived != "review" {
		t.Fatalf("foreign task or retyped work changed phase: %s", derived)
	}
	detail, err := phase.DerivePhaseDetail(f.Epic)
	f.Must(err)
	if detail.Derived != "review" {
		t.Fatalf("detail used type rule: %+v", detail)
	}
	// A valid identity is a witness even after its type changes.
	delete(f.Fake.Records, f.ID(f.Epic+".final"))
	derived, err = phase.DerivePhase(f.Epic)
	f.Must(err)
	if derived != "review" {
		t.Fatalf("nested witness lost: %s", derived)
	}
}
func TestS134Derive_PreKernelKeepsTypeRule(t *testing.T) {
	f := kerneltest.PreKernelSpec(t, kerneltest.WithFake(kernel.NativeIDs))
	epic := f.Fake.Records[f.Epic]
	delete(epic.Metadata, "mindspec_phase")
	f.Fake.Records[epic.ID] = epic
	for _, id := range f.Items {
		r := f.Fake.Records[id]
		r.Status = "closed"
		f.Fake.Records[id] = r
	}
	_, err := f.Fake.Run("create", "--title", "foreign task", "--type", "task", "--parent", f.Epic, "--json")
	f.Must(err)
	t.Cleanup(phase.SetRunBDForTest(f.Fake.Run))
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) { return f.Fake.Run(append([]string{"list"}, args...)...) }))
	got, err := phase.DerivePhase(f.Epic)
	f.Must(err)
	if got != "implement" {
		t.Fatalf("pre-kernel discriminator changed: %s", got)
	}
}
func TestS134Derive_PanelOnlyWitnessUsesRecordedPlan(t *testing.T) {
	f := kerneltest.PreKernelSpec(t, kerneltest.WithFake(kernel.NativeIDs))
	// A quoted fence in a YAML value is not the closing frontmatter fence.
	planPath := filepath.Join(f.SpecDir, "plan.md")
	planData, readErr := os.ReadFile(planPath)
	f.Must(readErr)
	f.Must(os.WriteFile(planPath, []byte(strings.Replace(string(planData), "status: Approved", "status: Approved\nversion: \"---\"", 1)), 0600))
	// Use a different working directory from the repository context.
	cwd, err := os.Getwd()
	f.Must(err)
	f.Must(os.Chdir(t.TempDir()))
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Error(err)
		}
	})
	epic := f.Fake.Records[f.Epic]
	delete(epic.Metadata, "mindspec_phase")
	f.Fake.Records[epic.ID] = epic
	for _, id := range f.Items {
		r := f.Fake.Records[id]
		r.Status = "closed"
		f.Fake.Records[id] = r
	}
	_, err = f.Fake.Run("create", "--title", "foreign task", "--type", "task", "--parent", f.Epic, "--json")
	f.Must(err)
	panel := filepath.Join(f.SpecDir, "reviews", "witness", "panel.json")
	f.Must(os.MkdirAll(filepath.Dir(panel), 0755))
	f.Must(os.WriteFile(panel, []byte(`{"orchestrated":true}`), 0600))
	t.Cleanup(phase.SetRunBDForTest(f.Fake.Run))
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) { return f.Fake.Run(append([]string{"list"}, args...)...) }))
	got, err := phase.DerivePhaseWithCache(phase.NewCache().WithRoot(f.Root), f.Epic)
	f.Must(err)
	if got != "review" {
		data, _ := json.Marshal(f.Fake.Records)
		t.Fatalf("panel witness lost: %s: %s", got, data)
	}
}

func TestS134Derive_NestedActorCountsWithoutChangingItsParent(t *testing.T) {
	f := kerneltest.Enrolled(t, kerneltest.WithFake(kernel.NativeIDs))
	epic := f.Fake.Records[f.Epic]
	delete(epic.Metadata, "mindspec_phase")
	f.Fake.Records[epic.ID] = epic
	actor := f.Create(f.Items[0]+".impl.a1", f.Items[0])
	actor.Status = "in_progress"
	f.Fake.Records[actor.ID] = *actor
	t.Cleanup(phase.SetRunBDForTest(f.Fake.Run))
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) { return f.Fake.Run(append([]string{"list"}, args...)...) }))
	got, err := phase.DerivePhase(f.Epic)
	f.Must(err)
	if got != "implement" {
		t.Fatalf("nested actor did not count: %s", got)
	}
}

func TestS134Derive_BadOptionalWitnessKeepsLegacyPhase(t *testing.T) {
	for _, bad := range []string{"panel", "plan"} {
		t.Run(bad, func(t *testing.T) {
			f := kerneltest.PreKernelSpec(t, kerneltest.WithFake(kernel.NativeIDs))
			epic := f.Fake.Records[f.Epic]
			delete(epic.Metadata, "mindspec_phase")
			f.Fake.Records[f.Epic] = epic
			t.Cleanup(phase.SetRunBDForTest(f.Fake.Run))
			t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
				return f.Fake.Run(append([]string{"list"}, args...)...)
			}))
			derive := func() string {
				got, err := phase.DerivePhaseWithCache(phase.NewCache().WithRoot(f.Root), f.Epic)
				f.Must(err)
				return got
			}
			want := "plan"
			if bad == "panel" {
				for _, id := range f.Items {
					r := f.Fake.Records[id]
					r.Status = "closed"
					f.Fake.Records[id] = r
				}
				_, err := f.Fake.Run("create", "--title", "foreign task", "--type", "task", "--parent", f.Epic, "--json")
				f.Must(err)
				want = "implement"
			}
			before := derive()
			if before != want {
				t.Fatalf("unexpected baseline phase: %s", before)
			}
			if bad == "panel" {
				path := filepath.Join(f.SpecDir, "reviews", "old", "panel.json")
				f.Must(os.MkdirAll(filepath.Dir(path), 0755))
				f.Must(os.WriteFile(path, []byte(`{not json`), 0600))
			} else {
				f.Must(os.WriteFile(filepath.Join(f.SpecDir, "plan.md"), []byte("---\nstatus: Approved\nbead_ids: [unclosed\n---\n"), 0600))
			}
			if after := derive(); after != before {
				t.Fatalf("bad optional witness changed phase: %s -> %s", before, after)
			}
		})
	}
}

func TestS134Derive_PreKernelDoesNotQueryNestedChildren(t *testing.T) {
	var baseline int
	for _, n := range []int{0, 50} {
		f := kerneltest.PreKernelSpec(t, kerneltest.WithFake(kernel.NativeIDs))
		epic := f.Fake.Records[f.Epic]
		delete(epic.Metadata, "mindspec_phase")
		f.Fake.Records[f.Epic] = epic
		for i := 0; i < n; i++ {
			_, err := f.Fake.Run("create", "--title", "unrelated", "--type", "task", "--json")
			f.Must(err)
		}
		t.Cleanup(phase.SetRunBDForTest(f.Fake.Run))
		t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
			if slices.Contains(args, "--parent") && !slices.Contains(args, f.Epic) {
				t.Fatalf("pre-kernel nested query: %v", args)
			}
			return f.Fake.Run(append([]string{"list"}, args...)...)
		}))
		f.Fake.Calls = nil
		got, err := phase.DerivePhaseWithCache(phase.NewCache().WithRoot(f.Root), f.Epic)
		f.Must(err)
		if got != "plan" {
			t.Fatalf("wrong legacy phase: %s", got)
		}
		calls := len(f.Fake.Calls)
		t.Logf("N=%d total=%d", n, calls)
		if n == 0 {
			baseline = calls
		} else if calls != baseline {
			t.Fatalf("unrelated records added calls: %d -> %d", baseline, calls)
		}
	}
}

func TestS134Derive_BadPanelDoesNotHideWitness(t *testing.T) {
	for _, badDir := range []string{"aaa-old", "zzz-old"} {
		t.Run(badDir, func(t *testing.T) {
			f := kerneltest.PreKernelSpec(t, kerneltest.WithFake(kernel.NativeIDs))
			epic := f.Fake.Records[f.Epic]
			delete(epic.Metadata, "mindspec_phase")
			f.Fake.Records[f.Epic] = epic
			for _, id := range f.Items {
				r := f.Fake.Records[id]
				r.Status = "closed"
				f.Fake.Records[id] = r
			}
			_, err := f.Fake.Run("create", "--title", "foreign task", "--type", "task", "--parent", f.Epic, "--json")
			f.Must(err)
			t.Cleanup(phase.SetRunBDForTest(f.Fake.Run))
			t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
				return f.Fake.Run(append([]string{"list"}, args...)...)
			}))
			for _, panel := range []struct{ dir, data string }{{"witness", `{"orchestrated":true}`}, {badDir, `{not json`}} {
				path := filepath.Join(f.SpecDir, "reviews", panel.dir, "panel.json")
				f.Must(os.MkdirAll(filepath.Dir(path), 0755))
				f.Must(os.WriteFile(path, []byte(panel.data), 0600))
				got, err := phase.DerivePhaseWithCache(phase.NewCache().WithRoot(f.Root), f.Epic)
				f.Must(err)
				if got != "review" {
					t.Fatalf("panel %s: enrolled phase = %s, want review", panel.dir, got)
				}
			}
		})
	}
}

func TestS134Derive_StoreReadSharedAcrossEpics(t *testing.T) {
	f := kerneltest.PreKernelSpec(t, kerneltest.WithFake(kernel.NativeIDs))
	epic := f.Fake.Records[f.Epic]
	delete(epic.Metadata, "mindspec_phase")
	f.Fake.Records[f.Epic] = epic
	other := kernel.Record{ID: "other-1", Type: "epic", Status: "open", Title: "[SPEC 003-other] Other", Metadata: map[string]any{"spec_id": "003-other"}}
	f.Fake.Records[other.ID] = other
	f.Fake.Records["other-1.1"] = kernel.Record{ID: "other-1.1", Parent: other.ID, Type: "task", Status: "open"}
	t.Cleanup(phase.SetRunBDForTest(f.Fake.Run))
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return f.Fake.Run(append([]string{"list"}, args...)...)
	}))
	f.Fake.Calls = nil
	cache := phase.NewCache().WithRoot(f.Root)
	for _, id := range []string{f.Epic, other.ID} {
		got, err := phase.DerivePhaseWithCache(cache, id)
		f.Must(err)
		if got != "plan" {
			t.Fatalf("%s: phase = %s, want plan", id, got)
		}
	}
	reads := 0
	for _, call := range f.Fake.Calls {
		if call[0] == "list" && slices.Contains(call, "--all") {
			reads++
		}
	}
	if reads != 1 {
		t.Fatalf("store-wide reads = %d, want 1", reads)
	}
}
