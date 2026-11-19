package kerneltest

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
)

type Fixture struct {
	Root    string
	SpecID  string
	SpecDir string
	Epic    string
	Items   []string
	Store   *kernel.Store
	Fake    *FakeBD
	t       testing.TB
}
type Option func(*fixtureOptions)
type fixtureOptions struct {
	fake   bool
	native bool
}

func WithFake(native bool) Option {
	return func(o *fixtureOptions) { o.fake = true; o.native = native }
}
func fixture(t testing.TB, enrolled bool, options ...Option) *Fixture {
	t.Helper()
	o := fixtureOptions{}
	for _, option := range options {
		option(&o)
	}
	f := &Fixture{SpecID: "001-x", t: t}
	if !enrolled {
		f.SpecID = "002-y"
	}
	var run kernel.Runner
	if o.fake {
		f.Root = gitRepo(t)
		f.Fake = NewFakeBD()
		run = f.Fake.Run
	} else {
		f.Root = NewRepo(t)
		run = Runner(t, f.Root)
	}
	f.SpecDir = filepath.Join(f.Root, ".mindspec", "specs", f.SpecID)
	raw := func(args ...string) kernel.Record {
		out, err := run(args...)
		if err != nil {
			t.Fatal(err)
		}
		var r kernel.Record
		if err = json.Unmarshal(out, &r); err != nil {
			t.Fatal(err)
		}
		f.Must(idvalidate.BeadID(r.ID))
		return r
	}
	phase := "implement"
	if enrolled {
		phase = "plan"
	}
	epic := raw("create", "--title", fmt.Sprintf("[mindspec] [SPEC %s] kernel fixture", f.SpecID), "--type", "epic", "--labels", "mindspec", "--metadata", fmt.Sprintf(`{"spec_id":%q,"mindspec_phase":%q}`, f.SpecID, phase), "--json")
	f.Epic = epic.ID
	f.Store = kernel.NewStore(f.Root, f.Epic, run)
	if o.fake {
		f.Store.NativeIDs = o.native
	}
	for i := 1; i <= 3; i++ {
		r := raw("create", "--title", fmt.Sprintf("[mindspec] [%s] Bead %d: Item %d", f.SpecID, i, i), "--type", "task", "--labels", "mindspec", "--parent", f.Epic, "--description", EnrolmentBody(), "--metadata", fmt.Sprintf(`{"spec_id":%q,"file_paths":null}`, f.SpecID), "--json")
		f.Items = append(f.Items, r.ID)
	}
	links := kernel.Links{{ID: f.Items[0], Upstream: []string{}}, {ID: f.Items[1], Upstream: []string{f.Items[0]}}, {ID: f.Items[2], Upstream: []string{f.Items[0]}}}
	if enrolled {
		f.Must(f.Store.WriteMetadata(f.Epic, kernel.Metadata(kernel.KeyLinks, links)))
	}
	for _, link := range links {
		for _, up := range link.Upstream {
			if enrolled {
				f.Must(f.Store.AddLink(link.ID, up))
			} else {
				_, err := run("dep", "add", link.ID, up, "--type", "blocks")
				f.Must(err)
			}
		}
	}
	plan := "---\nstatus: Approved\nbead_ids:\n"
	for _, id := range f.Items {
		plan += "  - " + id + "\n"
	}
	plan += "---\n# Plan\n"
	write(t, filepath.Join(f.SpecDir, "plan.md"), []byte(plan))
	write(t, filepath.Join(f.SpecDir, "spec.md"), []byte("---\nstatus: Approved\n---\n# Fixture\n"))
	if enrolled {
		f.Must(f.Store.WriteMetadata(f.Epic, kernel.Metadata(kernel.KeyRun, kernel.DefaultPolicy)))
		f.Must(f.Store.WriteMetadata(f.Epic, kernel.Metadata(kernel.KeyPlanApproval, kernel.PlanApproval(true))))
		for _, item := range f.Items {
			f.Create(item+".review", item)
		}
		f.Create(f.Epic+".final", f.Epic)
		for _, item := range f.Items {
			f.Must(f.Store.AddLink(f.Epic+".final", item))
		}
	}
	return f
}
func Enrolled(t testing.TB, options ...Option) *Fixture      { return fixture(t, true, options...) }
func PreKernelSpec(t testing.TB, options ...Option) *Fixture { return fixture(t, false, options...) }
func (f *Fixture) Must(err error) {
	f.t.Helper()
	if err != nil {
		f.t.Fatal(err)
	}
}
func (f *Fixture) State() kernel.State {
	f.t.Helper()
	s, err := kernel.ReadState(f.Store, f.SpecID, f.SpecDir)
	f.Must(err)
	return s
}
func (f *Fixture) Create(id, parent string) *kernel.Record {
	f.t.Helper()
	tuple, err := kernelid.Parse(id)
	f.Must(err)
	typ := "step"
	if tuple.Kind == "decision" {
		typ = "decision"
	}
	r, err := f.Store.Create(kernel.CreateRecord{ID: id, Parent: parent, Type: typ, Title: tuple.Kind, Tuple: tuple})
	f.Must(err)
	return r
}

// ID resolves a computed identity to the current physical bd identity.
func ID(t testing.TB, f *Fixture, computed string) string {
	t.Helper()
	r, err := f.Store.Lookup(computed)
	if err != nil {
		t.Fatal(err)
	}
	if r == nil {
		t.Fatalf("missing computed identity %s", computed)
	}
	return r.ID
}
func (f *Fixture) ID(computed string) string { return ID(f.t, f, computed) }

// Snapshot captures raw store rows, dependency lists, refs and repository status.
func Snapshot(t testing.TB, f *Fixture) string {
	t.Helper()
	records, err := f.Store.All()
	f.Must(err)
	data, err := f.Store.Run("list", "--all", "--json", "-n", "0")
	f.Must(err)
	for _, record := range records {
		if f.Fake == nil {
			if err := idvalidate.BeadID(record.ID); err != nil {
				_, err = kernelid.Parse(record.ID)
				f.Must(err)
			}
		}
		deps, err := f.Store.Run("dep", "list", record.ID, "--json")
		f.Must(err)
		data = append(data, '\n')
		data = append(data, deps...)
	}
	refs, err := command(t, f.Root, "git", "for-each-ref")
	f.Must(err)
	status, err := command(t, f.Root, "git", "status", "--porcelain")
	f.Must(err)
	return string(data) + "\n" + string(refs) + "\n" + string(status)
}

// Command runs a bounded subprocess with the fixture's isolated environment.
func (f *Fixture) Command(name string, args ...string) ([]byte, error) {
	return command(f.t, f.Root, name, args...)
}

// EnrolmentBody is the work text shared by enrolment producers and fixtures.
func EnrolmentBody() string {
	return "**Steps**\n1. Build the item.\n\n**Verification**\n- [ ] `go test ./internal/approve`\n\n**Acceptance Criteria**\n- The item works."
}

// EnrolmentPlan builds a valid plan with each later item depending on the first.
func EnrolmentPlan(specID string, n int) string {
	text := "---\nstatus: Draft\nspec_id: " + specID + "\nversion: '1'\nwork_chunks:\n"
	for i := 1; i <= n; i++ {
		deps := "[]"
		if i > 1 {
			deps = "[1]"
		}
		text += fmt.Sprintf("  - id: %d\n    depends_on: %s\n", i, deps)
	}
	text += "---\n# Plan\n\n## ADR Fitness\nNo relevant decisions.\n\n## Testing Strategy\nUnit tests.\n"
	for i := 1; i <= n; i++ {
		text += fmt.Sprintf("\n## Bead %d: Item %d\n\n%s\n", i, i, EnrolmentBody())
	}
	return text
}
