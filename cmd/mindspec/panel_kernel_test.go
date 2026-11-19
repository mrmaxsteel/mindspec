package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func kernelPanelFixture(t *testing.T) (*kerneltest.Fixture, string, string) {
	t.Helper()
	f := kerneltest.Enrolled(t, kerneltest.WithFake(true))
	withTestChdir(t, f.Root)
	rebindWrite(t, f.Root, ".beads/config.yaml", "types.custom: gate,step\n")
	rebindWrite(t, f.Root, ".gitignore", ".mindspec/phase.lock\n")
	rebindWrite(t, f.Root, ".mindspec/config.yaml", "panel:\n  reviewers:\n    - family: harness\n      model: test-model\n      lens: test-lens\n      count: 2\n")
	rebindWrite(t, f.Root, "proof.txt", "proof")
	actorCmdGit(t, f, "add", ".")
	actorCmdGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "fixture")
	head := actorCmdGit(t, f, "rev-parse", "HEAD")
	actorCmdGit(t, f, "branch", "spec/"+f.SpecID)
	for _, item := range f.Items {
		actorCmdGit(t, f, "branch", "bead/"+item)
	}
	impl := f.Items[0] + ".impl.a1"
	f.Create(impl, f.Items[0])
	kernelPanelClose(t, f, impl, head)
	old := panelKernelStateFn
	panelKernelStateFn = func(_, _ string) (*kernel.State, error) { s := f.State(); return &s, nil }
	t.Cleanup(func() { panelKernelStateFn = old })
	oldMeta := panelSubmitMetadataFn
	panelSubmitMetadataFn = func(string) (map[string]interface{}, error) { return map[string]interface{}{}, nil }
	t.Cleanup(func() { panelSubmitMetadataFn = oldMeta })
	return f, f.Items[0] + ".review", head
}
func kernelPanelClose(t *testing.T, f *kerneltest.Fixture, id, head string) {
	t.Helper()
	receipt := kernel.Receipt{Kind: "passed", Path: "proof.txt", Commit: head, Blob: actorCmdGit(t, f, "rev-parse", head+":proof.txt")}
	f.Must(f.Store.WriteReceipt(id, receipt))
	r, e := f.Store.Lookup(id)
	f.Must(e)
	if r.Status != "closed" {
		f.Must(f.Store.SetStatusIf(id, r.Status, "closed"))
	}
}
func kernelPanelAuthorise(f *kerneltest.Fixture, gate, head string, n, size int) {
	slots := []string{}
	s := f.State()
	base := gateBase(s, gate)
	for i := 1; i <= size; i++ {
		slots = append(slots, fmt.Sprintf("%s.r%d.R%d", base, n, i))
	}
	kerneltest.Authorise(f, gate, kernel.GateRound{Round: n, Subject: head, Slots: slots, SlotCount: size})
}
func kernelPanelRun(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	reset := func() {
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if v, ok := f.Value.(pflag.SliceValue); ok {
				_ = v.Replace(nil)
			} else {
				_ = f.Value.Set(f.DefValue)
			}
			f.Changed = false
		})
	}
	reset()
	defer reset()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	defer cmd.SetOut(nil)
	defer cmd.SetErr(nil)
	if e := cmd.ParseFlags(args); e != nil {
		return out.String(), e
	}
	e := cmd.RunE(cmd, cmd.Flags().Args())
	return out.String(), e
}
func kernelPanelCreate(t *testing.T, f *kerneltest.Fixture, slug string, extra ...string) (string, error) {
	return kernelPanelRun(t, panelCreateCmd, append([]string{slug, "--spec", f.SpecID}, extra...)...)
}
func kernelPanelRequire(t *testing.T, out string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", out, err)
	}
}
func kernelPanelRefusal(t *testing.T, out string, err error, word string) {
	t.Helper()
	if err == nil || !strings.Contains(out+err.Error(), word) {
		t.Fatalf("want refusal %q: %s %v", word, out, err)
	}
}
func kernelPanelRecord(t *testing.T, f *kerneltest.Fixture, id string) kernel.Record {
	t.Helper()
	r, e := f.Store.Lookup(id)
	f.Must(e)
	if r == nil {
		t.Fatal("missing", id)
	}
	return *r
}

type kernelPanelProtected struct {
	Rows map[string]kernel.Record
	Refs string
}

func kernelPanelForeign(t *testing.T, f *kerneltest.Fixture) kernelPanelProtected {
	t.Helper()
	f.Fake.Records["foreign-note"] = kernel.Record{ID: "foreign-note", Parent: f.Epic, Type: "task", Status: "open", Title: "note", Metadata: map[string]any{"foreign": true}}
	f.Fake.Records["foreign-bug"] = kernel.Record{ID: "foreign-bug", Type: "bug", Status: "open", Title: "bug", Metadata: map[string]any{"foreign": true}}
	r := f.Fake.Records[f.Items[1]]
	r.Dependencies = append(r.Dependencies, kernel.Dependency{ID: "foreign-bug", Type: "blocks"})
	f.Fake.Records[r.ID] = r
	f.Create(f.Items[2]+".impl.a1", f.Items[2])
	r = f.Fake.Records[f.Items[2]+".impl.a1"]
	r.Status = "in_progress"
	f.Fake.Records[r.ID] = r
	return kernelPanelProtected{Rows: map[string]kernel.Record{"foreign-note": f.Fake.Records["foreign-note"], "foreign-bug": f.Fake.Records["foreign-bug"], f.Items[1]: f.Fake.Records[f.Items[1]]}, Refs: actorCmdGit(t, f, "for-each-ref")}
}
func kernelPanelCheckForeign(t *testing.T, f *kerneltest.Fixture, before kernelPanelProtected) {
	t.Helper()
	if actorCmdGit(t, f, "for-each-ref") != before.Refs {
		t.Fatal("panel verb changed repository refs")
	}
	for id, want := range before.Rows {
		if !reflect.DeepEqual(f.Fake.Records[id], want) {
			t.Fatal("foreign row changed", id)
		}
	}
	for _, call := range f.Fake.Calls {
		if len(call) > 1 && (call[0] == "update" || call[0] == "close") {
			if _, ok := before.Rows[call[1]]; ok {
				t.Fatal("foreign write", call)
			}
		}
	}
}

func TestS134PanelCreate_AuthorisationAndFrozenGate(t *testing.T) {
	for _, mode := range []string{"missing", "registered", "reject", "refused"} {
		t.Run(mode, func(t *testing.T) {
			f, gate, head := kernelPanelFixture(t)
			if mode != "missing" {
				kernelPanelAuthorise(f, gate, head, 1, 2)
				kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "old", Round: 1, Subject: head, Slots: []string{"R1", "R2"}})
			}
			if mode == "reject" || mode == "refused" {
				word := "REQUEST_CHANGES"
				if mode == "reject" {
					word = "REJECT"
				}
				kernelPanelVerdict(t, f, gate, head, "old", "R1", word, 1)
				kernelPanelVerdict(t, f, gate, head, "old", "R2", "REQUEST_CHANGES", 1)
				fix := gate + ".r2.fix"
				f.Create(fix, gate)
				if mode == "reject" {
					d := kerneltest.OpenDecision(f, gate, "reject", 1, "fix", "halt")
					kerneltest.Answer(f, d, "fix")
				}
			}
			before := kerneltest.Snapshot(t, f)
			out, e := kernelPanelCreate(t, f, "1")
			n := 1
			if mode != "missing" {
				n = 2
			}
			kernelPanelRefusal(t, out, e, fmt.Sprintf("round %d is not authorized", n))
			if before != kerneltest.Snapshot(t, f) {
				t.Fatal("unauthorized create wrote")
			}
			if mode == "reject" || mode == "refused" {
				kernelPanelClose(t, f, gate+".r2.fix", head)
				kernelPanelAuthorise(f, gate, head, 2, 2)
			} else {
				kernelPanelAuthorise(f, gate, head, n, 2)
			}
			out, e = kernelPanelCreate(t, f, "1")
			kernelPanelRequire(t, out, e)
			before = kerneltest.Snapshot(t, f)
			out, e = kernelPanelCreate(t, f, "1")
			kernelPanelRefusal(t, out, e, "not authorized")
			if before != kerneltest.Snapshot(t, f) {
				t.Fatal("duplicate round wrote")
			}
		})
	}
	t.Run("passed and target", func(t *testing.T) {
		f, gate, head := kernelPanelFixture(t)
		kerneltest.PassGate(f, gate, head)
		out, e := kernelPanelCreate(t, f, "1")
		kernelPanelRefusal(t, out, e, "closed")
		out, e = kernelPanelCreate(t, f, "1", "--target", "another")
		kernelPanelRefusal(t, out, e, "target must be")
	})
}
func TestS134PanelCreate_CollisionHaltAndOpenSlot(t *testing.T) {
	for _, mode := range []string{"collision", "halt", "slot"} {
		t.Run(mode, func(t *testing.T) {
			f, gate, head := kernelPanelFixture(t)
			kernelPanelAuthorise(f, gate, head, 1, 2)
			switch mode {
			case "collision":
				id := gate + ".r1.R1"
				f.Fake.Records[id] = kernel.Record{ID: id, Type: "bug", Status: "open", Title: "foreign"}
			case "halt":
				d := kerneltest.OpenDecision(f, gate, "reject", 1, "fix", "halt")
				kerneltest.Answer(f, d, "halt")
			case "slot":
				f.Create(gate+".r1.R1", gate)
			}
			before := kerneltest.Snapshot(t, f)
			target := "1"
			if mode == "halt" {
				target = "2"
			}
			out, e := kernelPanelCreate(t, f, target)
			word := map[string]string{"collision": "id-collision", "halt": "halted", "slot": "is open"}[mode]
			kernelPanelRefusal(t, out, e, word)
			if before != kerneltest.Snapshot(t, f) {
				t.Fatal("refusal changed snapshot")
			}
		})
	}
}
func TestS134PanelCreate_LocalDecisionAndLease(t *testing.T) {
	f, _, head := kernelPanelFixture(t)
	gate := f.Items[1] + ".review"
	kernelPanelAuthorise(f, gate, head, 1, 2)
	fix := gate + ".r2.fix"
	f.Create(fix, gate)
	d := kerneltest.OpenDecision(f, gate, "reject", 1, "fix", "halt")
	kerneltest.PassGate(f, f.Items[0]+".review", head)
	f.Must(f.Store.SetStatusIf(f.Items[0], "open", "closed"))
	_, e := kernel.Lease(f.State(), kernel.LeaseOptions{Step: fix}, kernel.LeaseDeps{})
	kernelPanelRefusal(t, "", e, d)
	out, e := kernelPanelCreate(t, f, "2")
	kernelPanelRefusal(t, out, e, d)
	_, e = kernel.Answer(f.State(), d, "fix")
	f.Must(e)
	_, e = kernel.Lease(f.State(), kernel.LeaseOptions{Step: fix}, kernel.LeaseDeps{})
	f.Must(e)
}
func TestS134PanelRepanel_ScopeAndCount(t *testing.T) {
	f, base, head := kernelPanelFixture(t)
	kernelPanelAuthorise(f, base, head, 1, 3)
	out, e := kernelPanelCreate(t, f, "1")
	kernelPanelRequire(t, out, e)
	for _, slot := range []string{"R1", "R2", "R3"} {
		kernelPanelVerdict(t, f, base, head, "1", slot, "APPROVE", 1)
	}
	kerneltest.PassGate(f, base, head)
	gate := base + ".a2"
	f.Create(gate, f.Items[0])
	f.Must(f.Store.WriteMetadata(gate, map[string]any{kernel.KeyFrom: base, kernel.KeyRePanel: kernel.RePanel{SlotCount: 2, Paths: []string{"source.go"}}}))
	kernelPanelAuthorise(f, gate, head, 2, 2)
	out, e = kernelPanelCreate(t, f, "1", "--reviewers", "6")
	kernelPanelRefusal(t, out, e, "requires 2")
	out, e = kernelPanelCreate(t, f, "1")
	kernelPanelRequire(t, out, e)
	r := kernelPanelRecord(t, f, gate)
	reg, ok, e := kernel.Decode[kernel.RegistrationRecord](r.Metadata, kernel.KeyRegistration)
	f.Must(e)
	if !ok || reg.Round != 2 || !reflect.DeepEqual(reg.Slots, []string{"R1", "R2"}) {
		t.Fatal(reg)
	}
	res, e := panel.Tally(filepath.Join(f.SpecDir, "reviews", "1"))
	f.Must(e)
	if res.Panel.KernelGate != gate || !reflect.DeepEqual(res.Panel.Scope, []string{"source.go"}) {
		t.Fatal(res.Panel)
	}
	lines, e := panel.ReadLedger(res.Dir)
	f.Must(e)
	var round panel.RoundRecord
	for _, line := range lines {
		var candidate panel.RoundRecord
		f.Must(json.Unmarshal(line, &candidate))
		if candidate.Record == panel.RecordRound {
			round = candidate
		}
	}
	if round.Round != 2 || !reflect.DeepEqual(round.Scope, []string{"source.go"}) {
		t.Fatal(round)
	}
}

func kernelPanelVerdict(t *testing.T, f *kerneltest.Fixture, gate, head, slug, slot, word string, n int) string {
	t.Helper()
	dir := filepath.Join(f.SpecDir, "reviews", slug)
	v := paneltest.Verdict(word)
	v.Slot = slot
	v.Round = n
	v.ReviewedSHA = head
	v.Model = "test-model"
	if word == "REQUEST_CHANGES" {
		v.ConcreteChangesRequired = []panel.ChangeRequired{{Text: "missing test", Basis: "traced"}}
	}
	data := paneltest.Bytes(t, v)
	name := filepath.Join(dir, fmt.Sprintf("%s-round-%d.json", slot, n))
	f.Must(os.WriteFile(name, data, 0644))
	_, e := panel.Submit(panel.SubmitTarget{Dir: dir, Spec: f.SpecID, Panel: slug, Slot: slot, Round: n}, data, func(panel.Panel, panel.RosterSlot) (panel.SubmitResolution, error) {
		return panel.SubmitResolution{Family: "harness", Model: "test-model", RunnerKind: "harness", RunnerSource: "default"}, nil
	})
	f.Must(e)
	id := fmt.Sprintf("%s.r%d.%s", gateBase(f.State(), gate), n, slot)
	if _, ok := f.State().Records[id]; !ok {
		f.Create(id, gate)
	}
	kernelPanelClose(t, f, id, head)
	return name
}
func TestS134PanelCreate_RoundAllowanceAcrossSlugs(t *testing.T) {
	for _, final := range []bool{false, true} {
		t.Run(fmt.Sprint(final), func(t *testing.T) {
			f, gate, head := kernelPanelFixture(t)
			extra := []string{}
			if final {
				gate = f.Epic + ".final"
				extra = []string{"--gate", "final_review"}
				for _, item := range f.Items {
					kerneltest.PassGate(f, item+".review", head)
					f.Must(f.Store.SetStatusIf(item, "open", "closed"))
				}

			}
			for n := 1; n <= 5; n++ {
				slug := "A"
				if n > 3 {
					slug = "B"
				}
				kernelPanelAuthorise(f, gate, head, n, 2)
				if n > 1 {
					fix := fmt.Sprintf("%s.r%d.fix", gate, n)
					f.Create(fix, gate)
					kernelPanelClose(t, f, fix, head)
				}
				out, e := kernelPanelCreate(t, f, slug, append(extra, "--bead", func() string {
					if final {
						return ""
					}
					return f.Items[0]
				}())...)
				kernelPanelRequire(t, out, e)
				kernelPanelVerdict(t, f, gate, head, slug, "R1", "REQUEST_CHANGES", n)
				kernelPanelVerdict(t, f, gate, head, slug, "R2", "REQUEST_CHANGES", n)
				if n == 3 {
					out, e = kernelPanelRun(t, panelAbandonCmd, "A", "--spec", f.SpecID, "--reason", "use B", "--by", "operator")
					kernelPanelRequire(t, out, e)
				}
			}
			out, e := kernelPanelCreate(t, f, "B", append(extra, "--bead", func() string {
				if final {
					return ""
				}
				return f.Items[0]
			}())...)
			kernelPanelRefusal(t, out, e, "round 6")
			kernelPanelAuthorise(f, gate, head, 6, 2)
			out, e = kernelPanelCreate(t, f, "B", append(extra, "--bead", func() string {
				if final {
					return ""
				}
				return f.Items[0]
			}())...)
			kernelPanelRefusal(t, out, e, "allowance")
			d := kerneltest.OpenDecision(f, gate, "limit", 5, "continue:<n>", "halt")
			fix := gate + ".r6.fix"
			f.Create(fix, gate)
			kernelPanelClose(t, f, fix, head)
			kernelPanelAuthorise(f, gate, head, 6, 2)
			_, e = kernel.Answer(f.State(), d, "continue:1")
			f.Must(e)
			out, e = kernelPanelCreate(t, f, "B", append(extra, "--bead", func() string {
				if final {
					return ""
				}
				return f.Items[0]
			}())...)
			kernelPanelRequire(t, out, e)
			kernelPanelVerdict(t, f, gate, head, "B", "R1", "REQUEST_CHANGES", 6)
			kernelPanelVerdict(t, f, gate, head, "B", "R2", "REQUEST_CHANGES", 6)
			out, e = kernelPanelCreate(t, f, "B", append(extra, "--bead", func() string {
				if final {
					return ""
				}
				return f.Items[0]
			}())...)
			kernelPanelRefusal(t, out, e, "allowance")
		})
	}
}
func TestS134PanelSubmit_LeaseReceiptAndForeignSnapshot(t *testing.T) {
	f, gate, head := kernelPanelFixture(t)
	kernelPanelAuthorise(f, gate, head, 1, 2)
	foreign := kernelPanelForeign(t, f)
	out, e := kernelPanelCreate(t, f, "1")
	kernelPanelRequire(t, out, e)
	kernelPanelCheckForeign(t, f, foreign)
	id := gate + ".r1.R1"
	lease, e := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
	f.Must(e)
	v := paneltest.Verdict("APPROVE")
	v.Slot = "R1"
	v.Round = 1
	v.ReviewedSHA = head
	v.Model = "test-model"
	file := filepath.Join(f.SpecDir, "reviews", "1", "R1-round-1.json")
	f.Must(os.WriteFile(file, paneltest.Bytes(t, v), 0644))
	actorCmdGit(t, f, "add", ".")
	actorCmdGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "verdict")
	foreign.Refs = actorCmdGit(t, f, "for-each-ref")
	args := []string{"1", "--spec", f.SpecID, "--slot", "R1", "--round", "1", "--file", file}
	out, e = kernelPanelRun(t, panelSubmitCmd, args...)
	kernelPanelRefusal(t, out, e, "--lease")
	if kernelPanelRecord(t, f, id).Status == "closed" {
		t.Fatal("closed without token")
	}
	out, e = kernelPanelRun(t, panelSubmitCmd, append(args, "--lease", lease.Token)...)
	kernelPanelRequire(t, out, e)
	r := kernelPanelRecord(t, f, id)
	receipt, ok, e := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose)
	f.Must(e)
	if !ok || r.Status != "closed" || receipt.Path == "" {
		t.Fatal(r)
	}
	f.Must(kernel.ValidReceipt(f.Root, receipt))
	submitted, e := panel.LatestSubmissions(filepath.Dir(file), 1)
	f.Must(e)
	if len(submitted) != 1 || submitted["R1"].Seq == 0 || !strings.Contains(out, "R1 final") {
		t.Fatalf("verdict not published: %s %+v", out, submitted)
	}
	kernelPanelCheckForeign(t, f, foreign)
}

func kernelPanelDischarge(t *testing.T, deferIt bool, limit bool) {
	f, gate, head := kernelPanelFixture(t)
	kernelPanelAuthorise(f, gate, head, 1, 2)
	out, e := kernelPanelCreate(t, f, "1")
	kernelPanelRequire(t, out, e)
	kernelPanelVerdict(t, f, gate, head, "1", "R1", "REQUEST_CHANGES", 1)
	kernelPanelVerdict(t, f, gate, head, "1", "R2", "APPROVE", 1)
	extra := gate + ".r2.fix"
	if limit {
		extra = kerneltest.OpenDecision(f, gate, "limit", 1, "continue:<n>", "halt")
	} else {
		f.Create(extra, gate)
	}
	foreign := kernelPanelForeign(t, f)
	args := []string{"1", "--spec", f.SpecID, "--slot", "R1", "--round", "1", "--by", "operator"}
	if deferIt {
		fake := fakeDeferralBD(t)
		rebindWrite(t, fake, "epics.json", fmt.Sprintf(`[{"id":%q,"issue_type":"epic","metadata":{"spec_id":%q}}]`, f.Epic, f.SpecID))
		rebindWrite(t, fake, "follow-up.json", fmt.Sprintf(`[{"id":"follow-up","status":"open","description":"missing test","dependencies":[{"id":%q,"dependency_type":"discovered-from"}]}]`, f.Epic))
		out, e = kernelPanelRun(t, panelDeferCmd, append(args, "--bead", "follow-up")...)
	} else {
		out, e = kernelPanelRun(t, panelRefuteCmd, append(args, "--ground", "disproven", "--reason", "covered", "--evidence", "proof.txt")...)
	}
	kernelPanelRequire(t, out, e)
	for id, want := range map[string]string{gate: "passed", extra: "superseded"} {
		r := kernelPanelRecord(t, f, id)
		receipt, ok, e := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose)
		f.Must(e)
		if r.Status != "closed" || !ok || receipt.Kind != want {
			t.Fatalf("%s: %+v", id, r)
		}
		f.Must(kernel.ValidReceipt(f.Root, receipt))
	}
	if deferIt {
		_, ok, e := kernel.Decode[kernel.DeferRecord](kernelPanelRecord(t, f, gate).Metadata, kernel.KeyDefer)
		f.Must(e)
		if !ok {
			t.Fatal("defer audit missing")
		}
	}
	next, e := kernel.NextAction(f.State(), false)
	f.Must(e)
	complete := false
	for _, run := range next.Run {
		if reflect.DeepEqual(run.Argv, []string{"mindspec", "panel", "create", f.Items[0]}) {
			t.Fatal("passed gate opens another round")
		}
		if reflect.DeepEqual(run.Argv, []string{"mindspec", "complete", f.Items[0]}) {
			complete = true
		}
	}
	if !complete {
		t.Fatalf("complete missing: %+v", next)
	}
	kernelPanelCheckForeign(t, f, foreign)
}
func TestS134PanelRefute_RetallySupersedes(t *testing.T) {
	for _, limit := range []bool{false, true} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) { kernelPanelDischarge(t, false, limit) })
	}
}
func TestS134PanelDefer_RetallySupersedes(t *testing.T) {
	for _, limit := range []bool{false, true} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) { kernelPanelDischarge(t, true, limit) })
	}
}
func TestS134PanelRebind_OnePassAndForeignSnapshot(t *testing.T) {
	for _, refused := range []bool{false, true} {
		t.Run(fmt.Sprint(refused), func(t *testing.T) {
			f, gate, head := kernelPanelFixture(t)
			kernelPanelAuthorise(f, gate, head, 1, 2)
			out, e := kernelPanelCreate(t, f, "1")
			kernelPanelRequire(t, out, e)
			kernelPanelVerdict(t, f, gate, head, "1", "R1", "APPROVE", 1)
			kernelPanelVerdict(t, f, gate, head, "1", "R2", "APPROVE", 1)
			kerneltest.PassGate(f, gate, head)
			path := "source.go"
			if refused {
				path = ".mindspec/config.yaml"
			}
			rebindWrite(t, f.Root, path, "changed\n")
			actorCmdGit(t, f, "add", ".")
			actorCmdGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "changed")
			tip := actorCmdGit(t, f, "rev-parse", "HEAD")
			actorCmdGit(t, f, "update-ref", "refs/heads/bead/"+f.Items[0], tip)
			foreign := kernelPanelForeign(t, f)
			calls := 0
			reg := kernel.NewRegistry("counter")
			f.Must(reg.Register("counter", kernel.Ordinary, func(kernel.State) (kernel.Changes, error) { calls++; return kernel.Changes{}, nil }))
			kerneltest.WithRules(t, reg)
			out, e = kernelPanelRun(t, panelRebindCmd, "1", "--spec", f.SpecID, "--reason", "operator: reviewed diff")
			if refused {
				kernelPanelRefusal(t, out, e, "config.yaml")
				if calls != 0 {
					t.Fatal("refused rebind ran pass", calls)
				}
			} else {
				kernelPanelRequire(t, out, e)
				if calls != 1 {
					t.Fatal("successful rebind pass count", calls)
				}
			}
			kernelPanelCheckForeign(t, f, foreign)
		})
	}
}
func TestS134PanelCreate_PlanRoundsWarn(t *testing.T) {
	f, _, head := kernelPanelFixture(t)
	// Document gates remain count-and-warn even when the work lifecycle is enrolled.
	for n := 1; n <= 6; n++ {
		out, e := kernelPanelRun(t, panelCreateCmd, "plan", "--spec", f.SpecID, "--gate", "plan_approve", "--target", "spec/"+f.SpecID, "--round", fmt.Sprint(n))
		kernelPanelRequire(t, out, e)
		if n == 6 && (strings.Count(out, "warning:") != 1 || !strings.Contains(out, "count 6")) {
			t.Fatal("sixth round warning", out)
		}
	}
	_ = head
}
func TestS134SeedMatch_Register(t *testing.T) {
	f, gate, head := kernelPanelFixture(t)
	kernelPanelAuthorise(f, gate, head, 1, 2)
	out, e := kernelPanelCreate(t, f, "1")
	kernelPanelRequire(t, out, e)
	actual := kernelPanelRecord(t, f, gate)
	registration, _, e := kernel.Decode[kernel.RegistrationRecord](actual.Metadata, kernel.KeyRegistration)
	f.Must(e)
	dir := filepath.Join(f.SpecDir, "reviews", "1")
	res, e := panel.Tally(dir)
	f.Must(e)
	// A twin has the same store, refs and committed content, with no panel files.
	twin := kerneltest.Enrolled(t, kerneltest.WithFake(true))
	twin.Root = f.Root
	twin.SpecDir = filepath.Join(f.Root, ".mindspec", "specs", "seed-copy")
	twin.SpecID = f.SpecID
	twin.Store.Root = f.Root
	kernelPanelAuthorise(twin, gate, head, 1, 2)
	kerneltest.Register(twin, gate, registration)
	seeded := kernelPanelRecord(t, twin, gate)
	if !reflect.DeepEqual(actual.Metadata, seeded.Metadata) {
		t.Fatalf("registration metadata mismatch\n%+v\n%+v", actual.Metadata, seeded.Metadata)
	}
	seedDir := filepath.Join(twin.SpecDir, "reviews", "1")
	seed, e := panel.Tally(seedDir)
	f.Must(e)
	if !reflect.DeepEqual(res.Panel, seed.Panel) {
		t.Fatalf("snapshot mismatch\n%+v\n%+v", res.Panel, seed.Panel)
	}
	a, e := panel.ReadLedger(dir)
	f.Must(e)
	b, e := panel.ReadLedger(seedDir)
	f.Must(e)
	normal := func(lines [][]byte) []map[string]any {
		result := []map[string]any{}
		for _, line := range lines {
			var m map[string]any
			f.Must(json.Unmarshal(line, &m))
			delete(m, "created_at")
			result = append(result, m)
		}
		return result
	}
	if !reflect.DeepEqual(normal(a), normal(b)) {
		t.Fatalf("round ledger mismatch\n%s\n%s", a, b)
	}
}

func TestS134PanelCreate_CommandResolvesEnrolledGate(t *testing.T) {
	bin := buildMindspecBinary(t)
	f, gate, head := kernelPanelFixture(t)
	kernelPanelAuthorise(f, gate, head, 1, 2)
	store := actorCmdStore(t, f)
	out, code := actorCmdRun(t, f, bin, "panel", "create", "1")
	if code != 0 {
		t.Fatalf("create command: %d %s", code, out)
	}
	row := actorCmdRead(t, store, gate)
	reg, ok, e := kernel.Decode[kernel.RegistrationRecord](row.Metadata, kernel.KeyRegistration)
	f.Must(e)
	if !ok || reg.Subject != head || len(reg.Slots) != 2 {
		t.Fatal("command registration", row)
	}
	before, e := os.ReadFile(store)
	f.Must(e)
	out, code = actorCmdRun(t, f, bin, "panel", "create", "1")
	after, e := os.ReadFile(store)
	f.Must(e)
	if code == 0 || !strings.Contains(out, "not authorized") || !bytes.Equal(before, after) {
		t.Fatalf("duplicate command: %d %s", code, out)
	}
}

func TestS134PanelCreate_KeepsBindingAndBriefRefusals(t *testing.T) {
	for _, mode := range []string{"final-bead", "bead-without-item", "blank-brief"} {
		t.Run(mode, func(t *testing.T) {
			f, gate, head := kernelPanelFixture(t)
			kernelPanelAuthorise(f, gate, head, 1, 2)
			args := []string{"1", "--spec", f.SpecID}
			word := ""
			switch mode {
			case "final-bead":
				args = append(args, "--gate", "final_review", "--bead", f.Items[0])
				word = "--bead is only valid"
			case "bead-without-item":
				args[0] = f.SpecID
				args = append(args, "--gate", "bead")
				word = "requires a recorded work item"
			case "blank-brief":
				args = append(args, "--brief-file", "")
				word = "absolute path"
			}
			before := kerneltest.Snapshot(t, f)
			out, e := kernelPanelRun(t, panelCreateCmd, args...)
			kernelPanelRefusal(t, out, e, word)
			if kerneltest.Snapshot(t, f) != before {
				t.Fatal("binding or brief refusal wrote")
			}
		})
	}
}
