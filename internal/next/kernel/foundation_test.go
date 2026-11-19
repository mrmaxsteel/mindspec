package kernel_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func TestS134IDs_Grammar(t *testing.T) {
	for _, base := range []string{"test-abc.1.impl", "test-abc.1.review", "test-abc.final", "test-abc.1.review.r2.R3", "test-abc.final.r2.R3", "test-abc.1.review.r2.fix", "test-abc.final.r2.fix", "test-abc.1.review.decide.reject.r1", "test-abc.final.decide.limit.r5", "test-abc.1.review.judge", "test-abc.final.judge"} {
		for _, suffix := range []string{"", ".a1", ".a2"} {
			id := base + suffix
			t.Run(id, func(t *testing.T) {
				tuple, err := kernelid.Parse(id)
				if err != nil {
					t.Fatal(err)
				}
				formatted, err := kernelid.Format(tuple)
				if err != nil {
					t.Fatal(err)
				}
				again, err := kernelid.Parse(formatted)
				if err != nil || again != tuple {
					t.Fatalf("round trip %v %s %v", tuple, formatted, err)
				}
			})
		}
	}
	for _, id := range []string{"test-abc.1.review.r99999999999999999999999999999.R1", "test-abc.final.r1.R99999999999999999999999999999", "test-abc.final.a99999999999999999999999999999", "test-abc.1.review.1", "test-abc", "test-abc.1", "test-abc.0.review", "test-abc.1.review.a0", "test-abc.impl.a1", "test-abc.1.final", "test-abc.1.impl.r1.R1", "test-abc.1.review.r0.fix", "test-abc.final.judge.1", "test-abc.1.review.a2.r1.R1", "test-abc.1.review.decide..r1", "test-abc.1.review.r1.R0", "test-abc.final.r1.R1.extra"} {
		if kernelid.IsKernel(id) {
			t.Errorf("accepted foreign id %s", id)
		}
	}
}
func TestS134Witness_Independent(t *testing.T) {
	cases := []struct {
		name              string
		w                 kernelid.Witnesses
		enrolled, missing bool
	}{
		{"none", kernelid.Witnesses{Epic: "test-abc"}, false, false},
		{"marker", kernelid.Witnesses{Epic: "test-abc", RunPresent: true}, true, false},
		{"closed-retyped-id", kernelid.Witnesses{Epic: "test-abc", IDs: []string{"test-abc.1.review"}}, true, true},
		{"fallback-key", kernelid.Witnesses{Epic: "test-abc", Keys: []string{"test-abc.final.r1.R1"}}, true, true},
		{"panel", kernelid.Witnesses{Epic: "test-abc", Orchestrated: true}, true, true},
		{"foreign-tree", kernelid.Witnesses{Epic: "test-abc", IDs: []string{"other-abc.final", "test-abc.1.review.1"}}, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, b := kernelid.Enrolled(c.w)
			if a != c.enrolled || b != c.missing {
				t.Fatalf("got %t %t", a, b)
			}
		})
	}
	root := t.TempDir()
	path := filepath.Join(root, "review", "test", "panel.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"orchestrated":true}`), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := kernelid.PanelOrchestrated(root)
	if err != nil || !got {
		t.Fatalf("panel witness %t %v", got, err)
	}
}
func TestS134Store_IDModesAndOwnership(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(fmt.Sprint(native), func(t *testing.T) {
			f := kerneltest.Enrolled(t, kerneltest.WithFake(native))
			work := f.Items[0]
			ids := []string{work + ".impl.a1", work + ".review.decide.reject.r1"}
			for _, id := range ids {
				parent := work
				if strings.Contains(id, "decide") {
					parent = work + ".review"
				}
				r := f.Create(id, parent)
				if r.Title != "[mindspec] "+mustTuple(t, id).Kind || !reflect.DeepEqual(r.Labels, []string{"mindspec"}) {
					t.Fatalf("unmarked create: %+v", r)
				}
				if f.ID(id) != r.ID {
					t.Fatal("resolver failed")
				}
				if native != (r.ID == id) {
					t.Fatalf("wrong identity mode %+v", r)
				}
				before := kerneltest.Snapshot(t, f)
				f.Create(id, parent)
				if after := kerneltest.Snapshot(t, f); after != before {
					t.Fatal("create is not idempotent")
				}
			}
			collision := work + ".review.r1.R1"
			physical := collision
			metadata := map[string]any{}
			if !native {
				physical = "foreign-abc"
				metadata[kernel.KeyID] = collision
			}
			f.Fake.Records[physical] = kernel.Record{ID: physical, Parent: work, Title: "foreign", Type: "bug", Status: "open", Metadata: metadata}
			before := kerneltest.Snapshot(t, f)
			if err := f.Store.Collision(collision, mustTuple(t, collision)); err == nil || !strings.Contains(err.Error(), "id-collision") {
				t.Fatalf("collision accepted %v", err)
			}
			_, err := f.Store.Create(kernel.CreateRecord{ID: collision, Parent: work, Type: "step", Tuple: mustTuple(t, collision)})
			if err == nil {
				t.Fatal("adopted collision")
			}
			if after := kerneltest.Snapshot(t, f); after != before {
				t.Fatal("collision changed foreign record")
			}
			if err := f.Store.AddLink(work, physical); err == nil {
				t.Fatal("foreign link write accepted")
			}
		})
	}
}
func TestStoreRejectsMalformedPhysicalIDs(t *testing.T) {
	for _, id := range []string{"--help", "../test-abc", "test-abc.*", "test-abc.1.review.extra"} {
		for _, op := range []string{"status", "metadata", "downstream", "upstream", "parent"} {
			t.Run(id+"/"+op, func(t *testing.T) {
				f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
				work := f.Items[0]
				r := f.Fake.Records[work]
				r.ID = id
				r.Metadata[kernel.KeyID] = work
				f.Fake.Records[work] = r
				writes := 0
				f.Fake.Hook = func(args []string) error {
					if args[0] != "list" {
						writes++
					}
					return nil
				}
				var err error
				switch op {
				case "status":
					err = f.Store.SetStatusIf(work, "open", "closed")
				case "metadata":
					err = f.Store.WriteMetadata(work, kernel.Metadata("note", "test"))
				case "downstream":
					err = f.Store.AddLink(work, f.Items[1])
				case "upstream":
					err = f.Store.AddLink(f.Items[1], work)
				case "parent":
					key := work + ".impl.a1"
					_, err = f.Store.Create(kernel.CreateRecord{ID: key, Parent: work, Type: "step", Tuple: mustTuple(t, key)})
				}
				if err == nil || writes != 0 {
					t.Fatalf("invalid physical ID reached a write: err=%v writes=%d", err, writes)
				}
			})
		}
	}
}

func mustTuple(t *testing.T, id string) kernel.Tuple {
	t.Helper()
	tuple, err := kernelid.Parse(id)
	if err != nil {
		t.Fatal(err)
	}
	return tuple
}
func TestS134Store_CAS(t *testing.T) {
	f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
	id := f.Items[0] + ".impl.a1"
	f.Create(id, f.Items[0])
	before := kerneltest.Snapshot(t, f)
	writes := 0
	f.Fake.Hook = func(args []string) error {
		if args[0] == "update" {
			writes++
			return kerneltest.ExitError(13)
		}
		return nil
	}
	if err := f.Store.SetStatusIf(id, "open", "closed"); !errors.Is(err, kernel.ErrCASMismatch) {
		t.Fatalf("mismatch lost: %v", err)
	}
	if after := kerneltest.Snapshot(t, f); after != before || writes != 1 {
		t.Fatalf("mismatch wrote or retried: %d", writes)
	}
	for _, failures := range []int{1, 2} {
		calls := 0
		reads := 0
		f.Fake.Hook = func(args []string) error {
			if args[0] == "list" {
				reads++
			}
			if args[0] == "update" {
				calls++
				if calls <= failures {
					return kerneltest.ExitError(13)
				}
			}
			return nil
		}
		err := kernel.WithCASRetry(func() error {
			r, e := f.Store.Lookup(id)
			if e != nil {
				return e
			}
			return f.Store.SetStatusIf(id, r.Status, "in_progress")
		})
		if (failures == 1) != (err == nil) || calls != 2 || reads < 2 {
			t.Fatalf("retry failures=%d calls=%d reads=%d: %v", failures, calls, reads, err)
		}
	}
}
func TestS134State_EnrolledAndForeign(t *testing.T) {
	for _, native := range []bool{false, true} {
		f := kerneltest.Enrolled(t, kerneltest.WithFake(native))
		f.Fake.Records["foreign-abc"] = kernel.Record{ID: "foreign-abc", Parent: f.Epic, Type: "task", Status: "open"}
		s := f.State()
		if !s.Enrolled || s.MarkerMissing || !reflect.DeepEqual(s.Items, f.Items) || len(s.Gates) != 4 || len(s.Links) != 3 || !reflect.DeepEqual(s.Foreign, []string{"foreign-abc"}) {
			t.Fatalf("bad state %+v", s)
		}
		if !reflect.DeepEqual(s.Links[1].Upstream, []string{f.Items[0]}) || !reflect.DeepEqual(s.Links[2].Upstream, []string{f.Items[0]}) {
			t.Fatal("plan links lost")
		}
		epic := f.Fake.Records[f.Epic]
		delete(epic.Metadata, kernel.KeyRun)
		f.Fake.Records[f.Epic] = epic
		s = f.State()
		if !s.MarkerMissing || s.Policy != kernel.DefaultPolicy {
			t.Fatal("missing marker did not use pinned default")
		}
	}
}
func TestS134State_RealStore(t *testing.T) {
	f := kerneltest.Enrolled(t)
	s := f.State()
	r := s.Records[f.Items[1]]
	if len(r.Dependencies) == 0 {
		t.Errorf("real state lost dependencies: %+v", r)
	}
	if !s.Enrolled || len(s.Items) != 3 || len(s.Gates) != 4 {
		t.Fatalf("real store read: %+v", s)
	}
}
func TestS134Contract_PreKernelEntryPoints(t *testing.T) {
	f := kerneltest.PreKernelSpec(t, kerneltest.WithFake(false))
	s := f.State()
	before := kerneltest.Snapshot(t, f)
	_, a := kernel.NextAction(s, false)
	_, b := kernel.Lease(s, kernel.LeaseOptions{}, kernel.LeaseDeps{})
	_, c := kernel.Submit(s, kernel.SubmitOptions{})
	_, d := kernel.Answer(s, "", "")
	for _, err := range []error{a, b, c, d} {
		if !errors.Is(err, kernel.ErrPreKernel) {
			t.Fatalf("wrong refusal %v", err)
		}
	}
	if after := kerneltest.Snapshot(t, f); after != before {
		t.Fatal("pre-kernel entry wrote")
	}
}
func TestS134Contract_Registry(t *testing.T) {
	reg := kernel.NewRegistry("first", "second")
	noop := func(kernel.State) (kernel.Changes, error) { return kernel.Changes{}, nil }
	if reg.Register("other", kernel.Ordinary, noop) == nil {
		t.Fatal("unlisted name accepted")
	}
	if err := reg.Register("first", kernel.Ordinary, noop); err != nil {
		t.Fatal(err)
	}
	if reg.Register("first", kernel.Ordinary, noop) == nil {
		t.Fatal("duplicate accepted")
	}
	if kernel.RegisterRule("unknown-test", kernel.Ordinary, noop) == nil || kernel.RegisterRule("recreate", kernel.Halt, noop) == nil {
		t.Fatal("default contract violation accepted")
	}
	if kernel.Rules() != kernel.DefaultRegistry {
		t.Fatal("default registry not selected")
	}
	func() {
		kerneltest.WithRules(t, reg)
		if kernel.Rules() != reg {
			t.Fatal("override not installed")
		}
	}()
}
func TestS134Contract_WithRulesRestores(t *testing.T) {
	reg := kernel.NewRegistry("test")
	t.Run("override", func(t *testing.T) {
		kerneltest.WithRules(t, reg)
		if kernel.Rules() != reg {
			t.Fatal("override ignored")
		}
	})
	if kernel.Rules() != kernel.DefaultRegistry {
		t.Fatal("registry not restored")
	}
}
func TestS134Contract_RuleGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/rules.json")
	if err != nil {
		t.Fatal(err)
	}
	actual, err := json.MarshalIndent(kernel.RuleContracts, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != string(actual) {
		t.Fatalf("rule contract changed:\n%s", actual)
	}
}

var fixedTime = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func TestS134State_ForeignAnswerCannotHalt(t *testing.T) {
	f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
	f.Fake.Records["foreign-abc"] = kernel.Record{ID: "foreign-abc", Parent: f.Epic, Status: "closed", Type: "decision", Metadata: kernel.Metadata(kernel.KeyClose, kernel.Receipt{Kind: "passed", Answer: &kernel.DecisionAnswer{Option: "halt", At: fixedTime, Actor: "foreign"}})}
	if f.State().Halted() {
		t.Fatal("foreign answer halted the spec")
	}
}

func TestS134Store_RequireStepType(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".beads"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".beads", "config.yaml"), []byte("types.custom: gate,spike2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := kernel.RequireStepType(root); err == nil || !strings.Contains(err.Error(), "mindspec setup") {
		t.Fatalf("missing type refusal: %v", err)
	}
	if _, err := bead.EnsureBeadsConfig(root, false); err != nil {
		t.Fatal(err)
	}
	if err := kernel.RequireStepType(root); err != nil {
		t.Fatal(err)
	}
}

func TestS134State_PreKernelForeignWitness(t *testing.T) {
	for _, mode := range []string{"fallback", "native", "real"} {
		t.Run(mode, func(t *testing.T) {
			var options []kerneltest.Option
			if mode != "real" {
				options = append(options, kerneltest.WithFake(mode == "native"))
			}
			f := kerneltest.PreKernelSpec(t, options...)
			if f.State().Enrolled {
				t.Fatal("fixture is already enrolled")
			}
			key := f.Epic + ".final"
			if _, err := f.Store.Run("create", "foreign", "--id", key, "--type", "bug", "--json"); err != nil {
				t.Fatal(err)
			}
			before := kerneltest.Snapshot(t, f)
			s := f.State()
			if s.Enrolled {
				t.Error("foreign kernel-shaped bead enrolled a pre-kernel spec")
			}
			if !reflect.DeepEqual(s.Collisions, []string{key}) {
				t.Errorf("foreign identity not reported as a collision: %v", s.Collisions)
			}
			if _, err := kernel.NextAction(s, false); !errors.Is(err, kernel.ErrPreKernel) {
				t.Errorf("wrong refusal: %v", err)
			}
			if kerneltest.Snapshot(t, f) != before {
				t.Fatal("foreign witness check wrote")
			}
		})
	}
}

func TestS134Store_NonCanonicalCreate(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(fmt.Sprint(native), func(t *testing.T) {
			f := kerneltest.Enrolled(t, kerneltest.WithFake(native))
			gate := f.Items[0] + ".review"
			for _, id := range []string{f.Items[0] + ".impl", gate + ".r1.R1.a1", gate + ".judge", gate + ".a1", gate + ".r2.fix.a1", gate + ".decide.reject.r1.a1"} {
				before := kerneltest.Snapshot(t, f)
				typ := "step"
				if mustTuple(t, id).Kind == "decision" {
					typ = "decision"
				}
				if _, err := f.Store.Create(kernel.CreateRecord{ID: id, Parent: f.Items[0], Type: typ, Tuple: mustTuple(t, id)}); err == nil {
					t.Errorf("non-canonical create accepted: %s", id)
				}
				if kerneltest.Snapshot(t, f) != before {
					t.Errorf("non-canonical create wrote: %s", id)
				}
			}
		})
	}
}

func TestS134State_DuplicateKeyRecovery(t *testing.T) {
	f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
	key := f.Items[0] + ".impl.a1"
	f.Create(key, f.Items[0])
	f.Fake.Records[key] = kernel.Record{ID: key, Parent: f.Items[0], Type: "bug", Status: "open"}
	_, err := kernel.ReadState(f.Store, f.SpecID, f.SpecDir)
	if err == nil || !strings.Contains(err.Error(), "id-collision: "+key) || !strings.Contains(err.Error(), "remove that bead with bd") {
		t.Fatalf("collision recovery lacks identity or action: %v", err)
	}
}

func TestS134State_WitnessNeedsSubtreeAndTuple(t *testing.T) {
	for _, native := range []bool{false, true} {
		for _, tc := range []struct {
			name                    string
			parent, tuple, mismatch bool
		}{
			{name: "detached-matching-tuple", tuple: true},
			{name: "child-no-tuple", parent: true},
			{name: "child-wrong-tuple", parent: true, tuple: true, mismatch: true},
			{name: "child-matching-tuple", parent: true, tuple: true},
		} {
			t.Run(fmt.Sprintf("native=%t/%s", native, tc.name), func(t *testing.T) {
				f := kerneltest.PreKernelSpec(t, kerneltest.WithFake(native))
				key := f.Epic + ".final"
				id := key
				metadata := map[string]any{}
				if !native {
					id = "foreign-physical"
					metadata[kernel.KeyID] = key
				}
				if tc.tuple {
					tuple := mustTuple(t, key)
					if tc.mismatch {
						tuple.Attempt++
					}
					metadata[kernel.KeyTuple] = tuple
				}
				parent := ""
				if tc.parent {
					parent = f.Epic
				}
				f.Fake.Records[id] = kernel.Record{ID: id, Parent: parent, Type: "step", Status: "open", Metadata: metadata}
				s := f.State()
				want := tc.parent && tc.tuple && !tc.mismatch
				if s.Enrolled != want {
					t.Errorf("enrolled=%t want=%t", s.Enrolled, want)
				}
				if !want {
					if _, err := kernel.NextAction(s, false); !errors.Is(err, kernel.ErrPreKernel) {
						t.Errorf("wrong refusal: %v", err)
					}
				}
				if (!tc.tuple || tc.mismatch) && !reflect.DeepEqual(s.Collisions, []string{key}) {
					t.Errorf("collision not reported: %v", s.Collisions)
				}
			})
		}
	}
}
