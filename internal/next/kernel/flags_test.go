package kernel_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func TestS134Flags_MissingLinkAndForeignCycle(t *testing.T) {
	f := reducerFixture(t)
	final := f.Epic + ".final"
	physical := f.ID(final)
	r := f.Fake.Records[physical]
	r.Dependencies = r.Dependencies[:2]
	f.Fake.Records[physical] = r
	reduce(t, f, true)
	if len(f.State().Records[final].Dependencies) != 3 {
		t.Fatal("missing final link not repaired")
	}
	r = f.Fake.Records[physical]
	r.Dependencies = r.Dependencies[:2]
	f.Fake.Records[physical] = r
	// The cycle traverses an unrelated foreign record outside the epic subtree.
	foreign := "foreign-bug"
	f.Fake.Records[foreign] = kernel.Record{ID: foreign, Status: "open", Type: "bug", Dependencies: []kernel.Dependency{{ID: physical, Type: "blocks"}}}
	item := f.Fake.Records[f.Items[2]]
	item.Dependencies = append(item.Dependencies, kernel.Dependency{ID: foreign, Type: "blocks"})
	f.Fake.Records[item.ID] = item
	out := reduce(t, f, true)
	if len(f.State().Records[final].Dependencies) != 2 || !hasFlag(out, "foreign-blocker") {
		t.Fatalf("cyclic missing link written: %+v", out)
	}
	named := false
	for _, flag := range out.Flags {
		if reflect.DeepEqual(flag.Repair, []string{"bd", "dep", "remove", foreign, physical}) {
			named = true
		}
	}
	if !named {
		t.Fatalf("first foreign link from final not named: %+v", out.Flags)
	}
	bug := f.Fake.Records[foreign]
	bug.Dependencies = nil
	f.Fake.Records[foreign] = bug
	out = reduce(t, f, true)
	if len(f.State().Records[final].Dependencies) != 3 {
		t.Fatal("link not repaired after foreign cycle removed")
	}
	for _, flag := range out.Flags {
		if reflect.DeepEqual(flag.Repair, []string{"bd", "dep", "remove", foreign, physical}) {
			t.Fatal("removed cycle still flagged")
		}
	}
}
func TestS134Flags_ParentCycleAndForeignIsolation(t *testing.T) {
	f := reducerFixture(t)
	final := f.Epic + ".final"
	r := f.Fake.Records[f.ID(final)]
	r.Dependencies = nil
	f.Fake.Records[r.ID] = r
	foreign := "foreign-note"
	f.Fake.Records[foreign] = kernel.Record{ID: foreign, Parent: f.Epic, Status: "open", Type: "task", Title: "note"}
	before := f.Fake.Records[foreign]
	out := reduce(t, f, true)
	if !hasFlag(out, "foreign-child-open") || !reflect.DeepEqual(before, f.Fake.Records[foreign]) {
		t.Fatalf("foreign child altered or not flagged: %+v", out)
	}
	for _, a := range out.Actions {
		if a.Step == foreign {
			t.Fatal("foreign note selected")
		}
	}
	for _, r := range f.State().Records {
		if r.ID == foreign {
			continue
		}
		if r.ID == f.Epic || strings.Contains(r.ID, ".review") || r.ID == f.ID(final) || r.ID == f.Items[0] || r.ID == f.Items[1] || r.ID == f.Items[2] {
			continue
		}
		if _, ok, err := r.Tuple(); !ok || err != nil {
			t.Fatalf("created noncomputed record: %+v", r)
		}
	}
	// A parent edge also closes a cycle and must not be ignored.
	r = f.Fake.Records[f.ID(final)]
	r.Dependencies = r.Dependencies[:2]
	f.Fake.Records[r.ID] = r
	parent := f.Fake.Records[f.Items[2]]
	parent.Parent = f.ID(final)
	f.Fake.Records[parent.ID] = parent
	out = reduce(t, f, true)
	if len(f.State().Records[final].Dependencies) != 2 {
		t.Fatal("ancestor/descendant block link written")
	}
	_ = out
}
func TestS134Flags_MarkerAndCollision(t *testing.T) {
	f := reducerFixture(t)
	for id, r := range f.Fake.Records {
		if _, ok, _ := r.Tuple(); ok {
			r.Status = "closed"
			r.Type = "chore"
			f.Fake.Records[id] = r
		}
	}
	epic := f.Fake.Records[f.Epic]
	delete(epic.Metadata, kernel.KeyRun)
	f.Fake.Records[f.Epic] = epic
	if out := reduce(t, f, false); !hasFlag(out, "run-marker-missing") {
		t.Fatalf("closed retyped witness lost: %+v", out)
	}
	f = reducerFixture(t)
	collision := f.Items[0] + ".impl.a1"
	f.Fake.Records[collision] = kernel.Record{ID: collision, Parent: f.Items[0], Type: "bug", Status: "open"}
	before := kerneltest.Snapshot(t, f)
	out := reduce(t, f, true)
	if !hasFlag(out, "id-collision") || before != kerneltest.Snapshot(t, f) {
		t.Fatalf("collision adopted or not flagged: %+v", out)
	}
}

func TestS134Flags_ApplyDiffOnlyComputedWrites(t *testing.T) {
	f := reducerFixture(t)
	for _, r := range []kernel.Record{
		{ID: "foreign-note", Parent: f.Epic, Type: "task", Status: "open", Title: "note"},
		{ID: "foreign-bug", Type: "bug", Status: "open", Title: "bug"},
	} {
		f.Fake.Records[r.ID] = r
	}
	row := f.Fake.Records[f.Items[1]]
	row.Dependencies = append(row.Dependencies, kernel.Dependency{ID: "foreign-bug", Type: "blocks"})
	f.Fake.Records[row.ID] = row
	f.Create(f.Items[0]+".impl.a1", f.Items[0])
	f.Must(f.Store.SetStatusIf(f.Items[0]+".impl.a1", "open", "in_progress"))
	final := f.Epic + ".final"
	finalRow := f.Fake.Records[f.ID(final)]
	finalRow.Dependencies = finalRow.Dependencies[:2]
	f.Fake.Records[finalRow.ID] = finalRow
	before := f.State()
	f.Fake.Calls = nil
	reduce(t, f, true)
	after := f.State()
	for id, r := range before.Records {
		if id == final {
			if len(after.Records[id].Dependencies) != len(r.Dependencies)+1 {
				t.Fatal("missing final link was not repaired")
			}
			r.Dependencies = after.Records[id].Dependencies
		}
		if !reflect.DeepEqual(r, after.Records[id]) {
			t.Fatalf("existing record changed: %s", id)
		}
	}
	for id, r := range after.Records {
		if _, ok := before.Records[id]; !ok {
			if _, ok, err := r.Tuple(); err != nil || !ok {
				t.Fatalf("new noncomputed record: %s", id)
			}
		}
	}
	writes := 0
	for _, args := range f.Fake.Calls {
		switch args[0] {
		case "list":
		case "create":
		case "dep":
			writes++
			if args[1] != "add" {
				t.Fatalf("unexpected dependency write: %v", args)
			}
		default:
			t.Fatalf("unexpected write: %v", args)
		}
	}
	if writes != 1 {
		t.Fatalf("expected one computed link write, got %d", writes)
	}
}

func TestS134Flags_RawClaimAncestor(t *testing.T) {
	for _, ancestor := range []string{"epic", "item", "gate"} {
		t.Run(ancestor, func(t *testing.T) {
			f := reducerFixture(t)
			id := f.Epic
			if ancestor == "item" {
				id = f.Items[0]
			}
			if ancestor == "gate" {
				closePass(t, f, f.Items[0]+".impl.a1")
				reduce(t, f, true)
				registerRound(t, f, 1)
				id = f.Items[0] + ".review"
			}
			f.Must(f.Store.SetStatusIf(id, "open", "in_progress"))
			out := reduce(t, f, true)
			if len(out.Actions) == 0 || out.Status != "work" || kernel.OutputExitCode(out) != 0 {
				t.Errorf("raw claim held work: %+v", out)
			}
			named := false
			for _, flag := range out.Flags {
				if flag.Code == "raw-claim" && flag.Step == id {
					named = true
					want := []string{"bd", "update", f.ID(id), "--status", "open"}
					if !reflect.DeepEqual(flag.Repair, want) {
						t.Errorf("raw claim repair for %s = %q, want %q", id, flag.Repair, want)
					}
				}
			}
			if !named {
				t.Errorf("raw claim did not name %s: %+v", id, out.Flags)
			}
		})
	}
}

func TestS134Flags_ForeignInProgressParentHoldsWork(t *testing.T) {
	f := reducerFixture(t)
	foreign := "foreign-parent"
	f.Fake.Records[foreign] = kernel.Record{ID: foreign, Type: "epic", Status: "open"}
	epic := f.Fake.Records[f.Epic]
	epic.Parent = foreign
	f.Fake.Records[epic.ID] = epic
	if out := reduce(t, f, false); len(out.Actions) == 0 {
		t.Fatalf("open foreign parent must allow work: %+v", out)
	}
	parent := f.Fake.Records[foreign]
	parent.Status = "in_progress"
	f.Fake.Records[foreign] = parent
	state := f.State()
	if _, inside := state.Records[foreign]; inside {
		t.Fatal("foreign parent must be outside the spec records")
	}
	out, err := kernel.NextAction(state, true)
	f.Must(err)
	if len(out.Actions)+len(out.Run)+len(out.Decisions) != 0 || out.Status == "work" {
		t.Fatalf("in_progress foreign parent did not hold descendant work: %+v", out)
	}
	for _, item := range f.Items {
		if _, exists := f.State().Records[item+".impl.a1"]; exists {
			t.Errorf("held descendant %s acquired an implementation step", item)
		}
	}
	if !reflect.DeepEqual(parent, f.Fake.Records[foreign]) {
		t.Fatal("foreign parent was changed")
	}
	parent.Status = "open"
	f.Fake.Records[foreign] = parent
	if out := reduce(t, f, true); len(out.Actions) == 0 || out.Status != "work" {
		t.Fatalf("reopening foreign parent did not release work: %+v", out)
	}
}
