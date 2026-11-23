package kernel_test

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

func gateRound(t *testing.T, f *kerneltest.Fixture, gate string, n int, head string) kernel.GateRound {
	t.Helper()
	r := kernel.GateRound{Round: n, Subject: head, SlotCount: 3, Slots: []string{}}
	for i := 1; i <= 3; i++ {
		r.Slots = append(r.Slots, fmt.Sprintf("%s.r%d.R%d", gate, n, i))
	}
	return r
}
func gateHead(t *testing.T, f *kerneltest.Fixture, gate string) string {
	t.Helper()
	if gate == f.Epic+".final" {
		return reducerGit(t, f, "rev-parse", "spec/"+f.SpecID)
	}
	return reducerGit(t, f, "rev-parse", "bead/"+strings.TrimSuffix(gate, ".review"))
}
func registerGate(t *testing.T, f *kerneltest.Fixture, gate, slug string, n int) {
	t.Helper()
	head := gateHead(t, f, gate)
	kerneltest.Authorise(f, gate, gateRound(t, f, gate, n, head))
	kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: slug, Round: n, Subject: head, Slots: []string{"R1", "R2", "R3"}})
}
func refusedPanel(t *testing.T, f *kerneltest.Fixture, gate, slug string, n int, verdict string) {
	t.Helper()
	registerGate(t, f, gate, slug, n)
	dir := filepath.Join(f.SpecDir, "reviews", slug)
	for i := 1; i <= 3; i++ {
		slot := fmt.Sprintf("R%d", i)
		v := paneltest.Verdict(verdict)
		v.Slot = slot
		v.Round = n
		v.ReviewedSHA = gateHead(t, f, gate)
		v.Model = "test-model"
		_, err := panel.Submit(panel.SubmitTarget{Dir: dir, Spec: f.SpecID, Panel: slug, Slot: slot, Round: n}, paneltest.Bytes(t, v), func(panel.Panel, panel.RosterSlot) (panel.SubmitResolution, error) {
			return panel.SubmitResolution{Family: "harness", Model: "test-model", RunnerKind: "harness", RunnerSource: "default"}, nil
		})
		f.Must(err)
		id := fmt.Sprintf("%s.r%d.%s", gate, n, slot)
		f.Create(id, gate)
		f.Must(f.Store.WriteReceipt(id, kernel.Receipt{Kind: "passed", Retally: "seeded-verdict"}))
		f.Must(f.Store.SetStatusIf(id, "open", "closed"))
	}
	tally, err := panel.Tally(dir)
	f.Must(err)
	if !tally.Complete() || tally.PassRule(true) {
		t.Fatalf("not a refused tally: %+v", tally)
	}
}
func abandonPanel(t *testing.T, f *kerneltest.Fixture, slug string) {
	t.Helper()
	dir := filepath.Join(f.SpecDir, "reviews", slug)
	var p kerneltest.RegisteredPanel
	data, err := json.Marshal(f.State().Panels[0].Panel)
	f.Must(err)
	f.Must(json.Unmarshal(data, &p))
	p.Abandoned = true
	p.AbandonReason = "replacement panel"
	paneltest.Seed(t, dir, panel.AbandonRecord{Record: panel.RecordAbandon, Spec: f.SpecID, Panel: slug, Reason: p.AbandonReason, By: "operator", CreatedAt: kernel.Now().UTC().Format("2006-01-02T15:04:05Z")})
	data, err = json.Marshal(p)
	f.Must(err)
	put(t, filepath.Join(dir, panel.FileName), string(data))
}
func action(out kernel.Output, id string) bool {
	for _, a := range out.Actions {
		if a.Step == id {
			return true
		}
	}
	return false
}
func record(t *testing.T, f *kerneltest.Fixture, id string) kernel.Record {
	t.Helper()
	r, err := f.Store.Lookup(id)
	f.Must(err)
	if r == nil {
		t.Fatalf("missing %s", id)
	}
	return *r
}
func assertNoPanel(t *testing.T, out kernel.Output, target string) {
	t.Helper()
	if hasRun(out, "mindspec", "panel", "create", target) {
		t.Fatalf("unexpected panel create: %+v", out)
	}
}
func beadCommit(t *testing.T, f *kerneltest.Fixture, ref string) string {
	t.Helper()
	old := reducerGit(t, f, "rev-parse", ref)
	tree := reducerGit(t, f, "rev-parse", old+"^{tree}")
	path := filepath.Join(f.Root, "subject-commit")
	put(t, path, "tree "+tree+"\nparent "+old+"\nauthor Kernel Test <kernel@example.invalid> 1760000001 +0000\ncommitter Kernel Test <kernel@example.invalid> 1760000001 +0000\n\nsubject change on "+ref+"\n")
	head := reducerGit(t, f, "hash-object", "-t", "commit", "-w", path)
	reducerGit(t, f, "update-ref", "refs/heads/"+ref, head)
	return head
}
func mergeItem(t *testing.T, f *kerneltest.Fixture, item, head string) {
	t.Helper()
	base := reducerGit(t, f, "rev-parse", "spec/"+f.SpecID)
	tree := reducerGit(t, f, "rev-parse", head+"^{tree}")
	path := filepath.Join(f.Root, "merge-commit")
	put(t, path, "tree "+tree+"\nparent "+base+"\nparent "+head+"\nauthor Kernel Test <kernel@example.invalid> 1760000002 +0000\ncommitter Kernel Test <kernel@example.invalid> 1760000002 +0000\n\nMerge bead/"+item+"\n")
	merge := reducerGit(t, f, "hash-object", "-t", "commit", "-w", path)
	reducerGit(t, f, "update-ref", "refs/heads/spec/"+f.SpecID, merge)
}

func doneItem(t *testing.T, f *kerneltest.Fixture, item string) {
	t.Helper()
	head := beadCommit(t, f, "bead/"+item)
	kerneltest.PassGate(f, item+".review", head)
	f.Must(f.Store.SetStatusIf(item, "open", "closed"))
	mergeItem(t, f, item, head)
	f.Must(f.Store.WriteMetadata(item, map[string]any{kernel.KeyCompletion: head}))
	if !f.State().Merged[item] {
		t.Fatal("fixture branch not merged")
	}
}

func limitFixture(t *testing.T, final bool) (*kerneltest.Fixture, string, string) {
	t.Helper()
	f := reducerFixture(t)
	gate, target := f.Items[0]+".review", f.Items[0]
	if final {
		for _, item := range f.Items {
			doneItem(t, f, item)
		}
		gate, target = f.Epic+".final", f.SpecID
	}
	for n := 1; n <= 3; n++ {
		registerGate(t, f, gate, "A", n)
	}
	abandonPanel(t, f, "A")
	registerGate(t, f, gate, "B", 4)
	refusedPanel(t, f, gate, "B", 5, "REJECT")
	reproduceVerify(t, f, gate)
	return f, gate, target
}
func TestS134Reject_LimitAcrossPanels(t *testing.T) {
	for _, final := range []bool{false, true} {
		t.Run(fmt.Sprintf("final=%t", final), func(t *testing.T) {
			f, gate, target := limitFixture(t, final)
			ref := "bead/" + target
			if final {
				ref = "spec/" + f.SpecID
			}
			beadCommit(t, f, ref)
			before := kerneltest.Snapshot(t, f)
			read := reduce(t, f, false)
			assertNoPanel(t, read, target)
			if before != kerneltest.Snapshot(t, f) {
				t.Fatal("read wrote")
			}
			d := gate + ".decide.limit.r5"
			if len(read.Decisions) != 1 || read.Decisions[0].Step != d || !reflect.DeepEqual(read.Decisions[0].Options, []string{"continue:<n>", "halt"}) {
				t.Fatalf("wrong limit decision: %+v", read)
			}
			out := reduce(t, f, true)
			assertNoPanel(t, out, target)
			if r, _ := f.Store.Lookup(gate + ".r6.fix"); r != nil {
				t.Fatal("fix before continuation")
			}
			old, _, err := kernel.Decode[kernel.GateRound](record(t, f, gate).Metadata, kernel.KeyRound)
			f.Must(err)
			out, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: d, Answer: "continue:1"})
			f.Must(err)
			assertNoPanel(t, out, target)
			after, _, err := kernel.Decode[kernel.GateRound](record(t, f, gate).Metadata, kernel.KeyRound)
			f.Must(err)
			if old.Round != 5 || !reflect.DeepEqual(old, after) || !action(out, gate+".r6.fix") || kernel.Allowance(f.State(), gate) != 6 {
				t.Fatalf("answer authorised a round or lost fix: %+v", out)
			}
			assertNoPanel(t, reduce(t, f, true), target)
			fix := gate + ".r6.fix"
			// A committed artifact is durable evidence for the passed fix.
			commit := actorCommit(t, f, "fix-proof.txt", "fixed", ref)
			blob := reducerGit(t, f, "rev-parse", commit+":fix-proof.txt")
			f.Must(f.Store.WriteReceipt(fix, kernel.Receipt{Kind: "passed", Path: "fix-proof.txt", Commit: commit, Blob: blob}))
			f.Must(f.Store.SetStatusIf(fix, "open", "closed"))
			out = reduce(t, f, true)
			round, _, _ := kernel.Decode[kernel.GateRound](record(t, f, gate).Metadata, kernel.KeyRound)
			if round.Round != 6 || round.Subject != commit || !hasRun(out, "mindspec", "panel", "create", target) {
				t.Fatalf("passed fix did not authorise sixth round: %+v %+v", round, out)
			}
		})
	}
}
func TestS134Reject_ForeignChildAnswerReleasesFix(t *testing.T) {
	f := reducerFixture(t)
	gate := f.Items[0] + ".review"
	refusedPanel(t, f, gate, "reject", 1, "REJECT")
	reproduceVerify(t, f, gate)
	d := kerneltest.OpenDecision(f, gate, "reject", 1, "fix", "halt")
	raw, err := f.Store.Run("create", "--title", "foreign child", "--type", "task", "--parent", f.ID(d), "--json")
	f.Must(err)
	var foreign kernel.Record
	f.Must(json.Unmarshal(raw, &foreign))
	out, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: d, Answer: "fix"})
	f.Must(err)
	receipt, _, err := kernel.Decode[kernel.Receipt](record(t, f, d).Metadata, kernel.KeyClose)
	f.Must(err)
	if receipt.Answer == nil || receipt.Answer.Option != "fix" || record(t, f, d).Status != "open" || !hasFlag(out, "foreign-child-open") {
		t.Fatalf("answer lost behind child: %+v", out)
	}
	if !action(reduce(t, f, false), gate+".r2.fix") {
		t.Fatal("open answered decision held fix")
	}
	if record(t, f, foreign.ID).Status != "open" {
		t.Fatal("foreign child changed")
	}
}
func TestS134Reject_LocalDecision(t *testing.T) {
	f := reducerFixture(t)
	doneItem(t, f, f.Items[0])
	gate := f.Items[1] + ".review"
	refusedPanel(t, f, gate, "reject", 1, "REJECT")
	reproduceVerify(t, f, gate)
	out := reduce(t, f, true)
	d := gate + ".decide.reject.r1"
	if len(out.Decisions) != 1 || out.Decisions[0].Step != d || !action(out, f.Items[2]+".impl.a1") || action(out, gate+".r2.fix") {
		t.Fatalf("decision is not local: %+v", out)
	}
	if !strings.Contains(record(t, f, d).Title, "halt stops the spec and every work item") {
		t.Fatal("halt scope missing from decision text")
	}
	_, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: gate + ".r2.fix"}, kernel.LeaseDeps{})
	if err == nil || !strings.Contains(err.Error(), d) {
		t.Fatalf("unanswered decision allowed fix lease: %v", err)
	}
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: d, Answer: "fix"})
	f.Must(err)
	_, err = kernel.Lease(f.State(), kernel.LeaseOptions{Step: gate + ".r2.fix"}, kernel.LeaseDeps{})
	f.Must(err)
}
func TestS134Reject_RequestChanges(t *testing.T) {
	f := reducerFixture(t)
	gate := f.Items[0] + ".review"
	refusedPanel(t, f, gate, "changes", 1, "REQUEST_CHANGES")
	reproduceVerify(t, f, gate)
	out := reduce(t, f, true)
	if len(out.Decisions) != 0 || !action(out, gate+".r2.fix") {
		t.Fatalf("ordinary refusal did not release fix: %+v", out)
	}
	assertNoPanel(t, out, f.Items[0])
}

func TestS134Final_RequiresLastMerge(t *testing.T) {
	f := reducerFixture(t)
	for _, item := range f.Items[:2] {
		doneItem(t, f, item)
	}
	last := f.Items[2]
	head := beadCommit(t, f, "bead/"+last)
	kerneltest.PassGate(f, last+".review", head)
	f.Must(f.Store.SetStatusIf(last, "open", "closed"))
	out := reduce(t, f, true)
	assertNoPanel(t, out, f.SpecID)
	if _, has, _ := kernel.Decode[kernel.GateRound](record(t, f, f.Epic+".final").Metadata, kernel.KeyRound); has {
		t.Fatal("unmerged last item authorised final")
	}
	base := reducerGit(t, f, "rev-parse", "spec/"+f.SpecID)
	tree := reducerGit(t, f, "rev-parse", head+"^{tree}")
	merge := reducerGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit-tree", tree, "-p", base, "-p", head, "-m", "Merge bead/"+last)
	reducerGit(t, f, "update-ref", "refs/heads/spec/"+f.SpecID, merge)
	out = reduce(t, f, true)
	assertNoPanel(t, out, f.SpecID)
	f.Must(f.Store.WriteMetadata(last, map[string]any{kernel.KeyCompletion: head}))
	out = reduce(t, f, true)
	round, _, _ := kernel.Decode[kernel.GateRound](record(t, f, f.Epic+".final").Metadata, kernel.KeyRound)
	if round.Round != 1 || round.Subject != merge || !hasRun(out, "mindspec", "panel", "create", f.SpecID) {
		t.Fatalf("missing final authorisation: %+v %+v", round, out)
	}
	before := kerneltest.Snapshot(t, f)
	reduce(t, f, true)
	if before != kerneltest.Snapshot(t, f) {
		t.Fatal("final retry wrote")
	}
}

func TestS134SeedMatch_OpenDecision(t *testing.T) {
	for _, cause := range []string{"reject", "limit"} {
		t.Run(cause, func(t *testing.T) {
			real, seed := reducerFixture(t), reducerFixture(t)
			gate := real.Items[0] + ".review"
			n := 1
			options := []string{"fix", "halt"}
			if cause == "limit" {
				n = 5
				options = []string{"continue:<n>", "halt"}
			}
			for _, f := range []*kerneltest.Fixture{real, seed} {
				for i := 1; i < n; i++ {
					registerGate(t, f, gate, "review", i)
				}
				refusedPanel(t, f, gate, "review", n, "REJECT")
			}
			reproduceVerify(t, real, gate)
			reduce(t, seed, true)
			verify, v := verifyRecord(t, seed, gate, int(^uint(0)>>1))
			lease, err := kernel.Lease(seed.State(), kernel.LeaseOptions{Step: verify}, kernel.LeaseDeps{})
			seed.Must(err)
			restore := kernel.ReplaceRulesForTest(kernel.NewRegistry())
			_, err = submitVerify(t, seed, v, lease, nil)
			restore()
			seed.Must(err)
			id := kerneltest.OpenDecision(seed, gate, cause, n, options...)
			a, b := record(t, real, id), record(t, seed, id)
			a.ID = b.ID
			if !jsonEqual(a, b) {
				t.Fatalf("decision seeder differs: %+v %+v", record(t, real, id), record(t, seed, id))
			}
		})
	}
}

func TestS134SeedMatch_AuthoriseFinal(t *testing.T) {
	real, seed := reducerFixture(t), reducerFixture(t)
	for _, f := range []*kerneltest.Fixture{real, seed} {
		for _, item := range f.Items {
			doneItem(t, f, item)
		}
	}
	gate := real.Epic + ".final"
	reduce(t, real, true)
	kerneltest.Authorise(seed, gate, gateRound(t, seed, gate, 1, gateHead(t, seed, gate)))
	if !jsonEqual(record(t, real, gate), record(t, seed, gate)) {
		t.Fatalf("final authorisation differs from seeder: %+v %+v", record(t, real, gate), record(t, seed, gate))
	}
}

func TestS134Reject_ConsumerText(t *testing.T) {
	internal := regexp.MustCompile(`(?i)\badr[- ]?[0-9]{4}\b|\bspec[ _-]?[0-9]{3}\b|\b(?:r|b|ac|oq)[0-9]+\b|§|#[0-9]+|mindspec-[a-z0-9]{3,8}\.[0-9]+`)
	for _, cause := range []string{"reject", "limit"} {
		t.Run(cause, func(t *testing.T) {
			f := reducerFixture(t)
			gate := f.Items[0] + ".review"
			n := 1
			if cause == "limit" {
				n = 5
			}
			for i := 1; i < n; i++ {
				registerGate(t, f, gate, "review", i)
			}
			refusedPanel(t, f, gate, "review", n, "REJECT")
			reproduceVerify(t, f, gate)
			out := reduce(t, f, true)
			d := record(t, f, fmt.Sprintf("%s.decide.%s.r%d", gate, cause, n))
			if len(out.Decisions) != 1 {
				t.Fatalf("missing decision: %+v", out)
			}
			text := d.Title + " " + strings.Join(out.Decisions[0].Options, " ")
			if internal.MatchString(text) {
				t.Fatalf("internal reference in consumer text: %s", text)
			}
			if !strings.Contains(d.Title, "halt stops the spec and every work item") {
				t.Fatal("halt option lacks its scope")
			}
		})
	}
}

func TestS134Reject_CommitAfterRefusedTally(t *testing.T) {
	for _, verdict := range []string{"REJECT", "REQUEST_CHANGES"} {
		t.Run(verdict, func(t *testing.T) {
			f := reducerFixture(t)
			gate := f.Items[0] + ".review"
			refusedPanel(t, f, gate, "review", 1, verdict)
			reproduceVerify(t, f, gate)
			old, _, err := kernel.Decode[kernel.GateRound](record(t, f, gate).Metadata, kernel.KeyRound)
			f.Must(err)
			beadCommit(t, f, "bead/"+f.Items[0])
			out := reduce(t, f, true)
			after, _, err := kernel.Decode[kernel.GateRound](record(t, f, gate).Metadata, kernel.KeyRound)
			f.Must(err)
			if !reflect.DeepEqual(old, after) {
				t.Fatalf("commit after a refused tally authorised round %d", after.Round)
			}
			assertNoPanel(t, out, f.Items[0])
			record(t, f, gate+".r2.fix")
			if verdict == "REJECT" {
				d := gate + ".decide.reject.r1"
				if len(out.Decisions) != 1 || out.Decisions[0].Step != d || action(out, gate+".r2.fix") {
					t.Fatalf("commit bypassed reject decision: %+v", out)
				}
			} else if len(out.Decisions) != 0 || !action(out, gate+".r2.fix") {
				t.Fatalf("commit bypassed refused-round fix: %+v", out)
			}
			assertNoPanel(t, reduce(t, f, false), f.Items[0])
			assertNoPanel(t, reduce(t, f, true), f.Items[0])
		})
	}
}

func TestS134Reject_RawFixCloseAfterCommit(t *testing.T) {
	f := reducerFixture(t)
	gate := f.Items[0] + ".review"
	refusedPanel(t, f, gate, "review", 1, "REQUEST_CHANGES")
	reproduceVerify(t, f, gate)
	reduce(t, f, true)
	fix := gate + ".r2.fix"
	beadCommit(t, f, "bead/"+f.Items[0])
	f.Must(f.Store.SetStatusIf(fix, "open", "closed"))
	out := reduce(t, f, true)
	round, _, err := kernel.Decode[kernel.GateRound](record(t, f, gate).Metadata, kernel.KeyRound)
	f.Must(err)
	if round.Round != 1 {
		t.Fatalf("raw fix close authorised round %d without a passed receipt", round.Round)
	}
	assertNoPanel(t, out, f.Items[0])
	assertNoPanel(t, reduce(t, f, false), f.Items[0])
	assertNoPanel(t, reduce(t, f, true), f.Items[0])
}

func TestS134Final_FixBrief(t *testing.T) {
	f, gate, _ := limitFixture(t, true)
	reduce(t, f, true)
	out, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: gate + ".decide.limit.r5", Answer: "continue:1"})
	f.Must(err)
	fix := gate + ".r6.fix"
	for _, a := range out.Actions {
		if a.Step == fix {
			want := []string{"mindspec", "next", fix, "--emit-only"}
			if !reflect.DeepEqual(a.Brief, want) {
				t.Fatalf("final fix brief = %q, want %q", a.Brief, want)
			}
			return
		}
	}
	t.Fatal("final fix action missing")
}

func TestS134Final_LimitUsesInitialRound(t *testing.T) {
	f := reducerFixture(t)
	for _, item := range f.Items {
		doneItem(t, f, item)
	}
	gate := f.Epic + ".final"
	// Prior registrations count against the allowance, but do not name the
	// initial round that this gate would authorise.
	for n := 1; n <= 5; n++ {
		kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "old", Round: n, Subject: gateHead(t, f, gate), Slots: []string{"R1", "R2", "R3"}})
	}
	out := reduce(t, f, true)
	want := gate + ".decide.limit.r1"
	if len(out.Decisions) != 1 || out.Decisions[0].Step != want {
		t.Fatalf("final limit used registration count instead of initial round: %+v", out.Decisions)
	}
	if _, has, err := kernel.Decode[kernel.GateRound](record(t, f, gate).Metadata, kernel.KeyRound); err != nil || has {
		t.Fatalf("final limit authorised a round: has=%t err=%v", has, err)
	}
	assertNoPanel(t, out, f.SpecID)
}

func TestS134Submit_FixPreservesRound(t *testing.T) {
	f, _, _ := actorFixture(t)
	put(t, filepath.Join(f.Root, ".beads", "config.yaml"), "types.custom: gate,step\n")
	gate := f.Items[0] + ".review"
	refusedPanel(t, f, gate, "changes", 1, "REQUEST_CHANGES")
	reproduceVerify(t, f, gate)
	reduce(t, f, true)
	fix := gate + ".r2.fix"
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: fix}, kernel.LeaseDeps{})
	f.Must(err)
	commit := actorCommit(t, f, "fix-proof.txt", "fixed", "bead/"+f.Items[0])
	out, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: fix, Lease: lease.Token, Commit: commit})
	f.Must(err)
	round, _, err := kernel.Decode[kernel.GateRound](record(t, f, gate).Metadata, kernel.KeyRound)
	f.Must(err)
	receipt, _, err := kernel.Decode[kernel.Receipt](record(t, f, fix).Metadata, kernel.KeyClose)
	f.Must(err)
	if round.Round != 2 || round.Subject != commit || receipt.Kind != "passed" || record(t, f, fix).Status != "closed" || !hasRun(out, "mindspec", "panel", "create", f.Items[0]) {
		t.Fatalf("fix submission superseded its subject or did not open the next round: %+v %+v", round, out)
	}
}

func TestS134Reducer_ExistingOutcomeHoldsOpenSlots(t *testing.T) {
	for _, outcome := range []string{"closed-fix", "answered-open-decision", "answered-closed-decision", "closed-decision"} {
		t.Run(outcome, func(t *testing.T) {
			f := reducerFixture(t)
			gate := f.Items[0] + ".review"
			registerGate(t, f, gate, "review", 1)
			reduce(t, f, true)
			if outcome == "closed-fix" {
				fix := gate + ".r2.fix"
				f.Create(fix, gate)
				f.Must(f.Store.SetStatusIf(fix, "open", "closed"))
			} else {
				d := kerneltest.OpenDecision(f, gate, "reject", 1, "fix", "halt")
				if outcome == "closed-decision" {
					f.Must(f.Store.SetStatusIf(d, "open", "closed"))
				} else {
					kerneltest.Answer(f, d, "fix")
					if outcome == "answered-open-decision" {
						f.Must(f.Store.SetStatusIf(d, "closed", "open"))
					}
				}
			}
			beadCommit(t, f, "bead/"+f.Items[0])
			out := reduce(t, f, true)
			round, _, err := kernel.Decode[kernel.GateRound](record(t, f, gate).Metadata, kernel.KeyRound)
			f.Must(err)
			if round.Round != 1 {
				t.Fatalf("%s allowed supersede to authorise round %d", outcome, round.Round)
			}
			for _, id := range round.Slots {
				if record(t, f, id).Status != "open" {
					t.Fatalf("%s superseded an open slot: %s", outcome, id)
				}
			}
			assertNoPanel(t, out, f.Items[0])
		})
	}
}

func TestS134Submit_ExcludedDecisionAnswerHoldsRound(t *testing.T) {
	f := reducerFixture(t)
	gate := f.Items[0] + ".review"
	registerGate(t, f, gate, "review", 1)
	reduce(t, f, true)
	d := kerneltest.OpenDecision(f, gate, "reject", 1, "fix", "halt")
	before, has, err := kernel.Decode[kernel.GateRound](record(t, f, gate).Metadata, kernel.KeyRound)
	f.Must(err)
	if !has || before.Round != 1 || len(before.Slots) == 0 {
		t.Fatalf("missing initial round: %+v", before)
	}
	beadCommit(t, f, "bead/"+f.Items[0])
	out, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: d, Answer: "fix"})
	f.Must(err)
	after, _, err := kernel.Decode[kernel.GateRound](record(t, f, gate).Metadata, kernel.KeyRound)
	f.Must(err)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("answer pre-pass changed the round: before=%+v after=%+v", before, after)
	}
	for _, id := range before.Slots {
		if record(t, f, id).Status != "open" {
			t.Fatalf("answer pre-pass superseded slot %s", id)
		}
	}
	assertNoPanel(t, out, f.Items[0])
	if action(out, gate+".r2.fix") || record(t, f, d).Status != "closed" {
		t.Fatalf("answer created a fix before a complete verified round: %+v", out)
	}
}

func TestS134Submit_ExcludedClosedFixHoldsRound(t *testing.T) {
	f := reducerFixture(t)
	gate := f.Items[0] + ".review"
	registerGate(t, f, gate, "review", 1)
	reduce(t, f, true)
	fix := gate + ".r2.fix"
	f.Create(fix, gate)
	// A raw close has no passed receipt. It also bypasses the open-fix hold.
	f.Must(f.Store.SetStatusIf(fix, "open", "closed"))
	before, has, err := kernel.Decode[kernel.GateRound](record(t, f, gate).Metadata, kernel.KeyRound)
	f.Must(err)
	if !has || before.Round != 1 || len(before.Slots) == 0 {
		t.Fatalf("missing initial round: %+v", before)
	}
	head := beadCommit(t, f, "bead/"+f.Items[0])
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: fix, Lease: "stale-token", Commit: head})
	if err == nil || !strings.Contains(err.Error(), "lease token is not current") {
		t.Fatalf("submit did not refuse the stale lease: %v", err)
	}
	// The pre-pass stands after refusal, but must not open a round for this fix.
	after, _, err := kernel.Decode[kernel.GateRound](record(t, f, gate).Metadata, kernel.KeyRound)
	f.Must(err)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("fix pre-pass changed the round: before=%+v after=%+v", before, after)
	}
	for _, id := range before.Slots {
		if record(t, f, id).Status != "open" {
			t.Fatalf("fix pre-pass superseded slot %s", id)
		}
	}
	if _, has, err := kernel.Decode[kernel.Receipt](record(t, f, fix).Metadata, kernel.KeyClose); err != nil || has {
		t.Fatalf("refused fix gained a receipt: present=%t err=%v", has, err)
	}
	assertNoPanel(t, reduce(t, f, false), f.Items[0])
}
