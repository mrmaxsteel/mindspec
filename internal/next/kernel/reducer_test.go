package kernel_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

func reducerFixture(t *testing.T) *kerneltest.Fixture {
	t.Helper()
	kerneltest.SetClock(t, fixedTime)
	f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
	put(t, filepath.Join(f.Root, ".beads", "config.yaml"), "types.custom: gate,step\n")
	put(t, filepath.Join(f.Root, ".gitignore"), ".mindspec/phase.lock\n")
	// Pin three review slots explicitly. The product's default roster remains configurable.
	put(t, filepath.Join(f.Root, ".mindspec", "config.yaml"), "panel:\n  reviewers:\n    - family: harness\n      count: 3\n")
	tree := reducerGit(t, f, "rev-parse", "HEAD^{tree}")
	commitPath := filepath.Join(f.Root, "initial-commit")
	put(t, commitPath, "tree "+tree+"\nauthor Kernel Test <kernel@example.invalid> 1760000000 +0000\ncommitter Kernel Test <kernel@example.invalid> 1760000000 +0000\n\ninitial\n")
	head := reducerGit(t, f, "hash-object", "-t", "commit", "-w", commitPath)
	reducerGit(t, f, "branch", "spec/"+f.SpecID, head)
	for _, item := range f.Items {
		reducerGit(t, f, "branch", "bead/"+item, head)
	}
	return f
}
func put(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}
func reducerGit(t *testing.T, f *kerneltest.Fixture, args ...string) string {
	t.Helper()
	out, err := f.Command("git", args...)
	f.Must(err)
	return strings.TrimSpace(string(out))
}
func reduce(t *testing.T, f *kerneltest.Fixture, apply bool) kernel.Output {
	t.Helper()
	out, err := kernel.NextAction(f.State(), apply)
	f.Must(err)
	return out
}
func hasRun(out kernel.Output, argv ...string) bool {
	for _, r := range out.Run {
		if reflect.DeepEqual(r.Argv, argv) {
			return true
		}
	}
	return false
}
func hasFlag(out kernel.Output, code string) bool {
	for _, f := range out.Flags {
		if f.Code == code {
			return true
		}
	}
	return false
}
func seedClosed(t *testing.T, f *kerneltest.Fixture, id string, receipt *kernel.Receipt) {
	t.Helper()
	parent := f.Items[0]
	if strings.Contains(id, ".review.") {
		parent = f.Items[0] + ".review"
	}
	f.Create(id, parent)
	if receipt != nil {
		f.Must(f.Store.WriteReceipt(id, *receipt))
	}
	f.Must(f.Store.SetStatusIf(id, "open", "closed"))
}
func closePass(t *testing.T, f *kerneltest.Fixture, id string) {
	t.Helper()
	seedClosed(t, f, id, &kernel.Receipt{Kind: "passed", Retally: "seeded-artifact"})
}
func registerRound(t *testing.T, f *kerneltest.Fixture, n int) {
	t.Helper()
	gate := f.Items[0] + ".review"
	head := reducerGit(t, f, "rev-parse", "bead/"+f.Items[0])
	slots := []string{"R1", "R2", "R3"}
	kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "panel", Round: n, Subject: head, Slots: slots})
}
func counted(t *testing.T, f *kerneltest.Fixture, n int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		registerRound(t, f, i)
	}
}
func authorise(t *testing.T, f *kerneltest.Fixture, n int) {
	t.Helper()
	gate := f.Items[0] + ".review"
	round := kernel.GateRound{Round: n, Subject: reducerGit(t, f, "rev-parse", "bead/"+f.Items[0]), SlotCount: 3, Slots: []string{}}
	for i := 1; i <= 3; i++ {
		round.Slots = append(round.Slots, fmt.Sprintf("%s.r%d.R%d", gate, n, i))
	}
	kerneltest.Authorise(f, gate, round)
}
func newCommit(t *testing.T, f *kerneltest.Fixture, ref string) string {
	t.Helper()
	old := reducerGit(t, f, "rev-parse", ref)
	tree := reducerGit(t, f, "rev-parse", old+"^{tree}")
	oid := reducerGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit-tree", tree, "-p", old, "-m", "subject change")
	reducerGit(t, f, "update-ref", "refs/heads/"+ref, oid)
	return oid
}
func TestS134Reducer_ReadOnlyAndApply(t *testing.T) {
	f := reducerFixture(t)
	before := kerneltest.Snapshot(t, f)
	a, b := reduce(t, f, false), reduce(t, f, false)
	if !jsonEqual(a, b) || kerneltest.Snapshot(t, f) != before || len(a.Actions) != 1 || a.Actions[0].Step != f.Items[0]+".impl.a1" || a.Actions[0].Kind != "implement" || a.Actions[0].Exists {
		t.Fatalf("read-only contract: %+v", a)
	}
	out := reduce(t, f, true)
	snap := kerneltest.Snapshot(t, f)
	if len(out.Actions) != 1 || out.Actions[0].Step != f.Items[0]+".impl.a1" || !out.Actions[0].Exists || !reflect.DeepEqual(f.State().Steps, []string{f.Items[0] + ".impl.a1"}) {
		t.Fatalf("apply created wrong steps: %+v", out)
	}
	f.Fake.Calls = nil
	again := reduce(t, f, true)
	if snap != kerneltest.Snapshot(t, f) || out.Revision != again.Revision {
		t.Fatal("second apply changed state or revision")
	}
	for _, args := range f.Fake.Calls {
		if args[0] != "list" && args[0] != "dep" {
			t.Fatalf("no-change apply wrote: %v", args)
		}
	}
	// This is exactly the state after the only create, before any subsequent read.
	killed := reducerFixture(t)
	killed.Create(killed.Items[0]+".impl.a1", killed.Items[0])
	killedBefore := kerneltest.Snapshot(t, killed)
	resumed := reduce(t, killed, true)
	if killedBefore != kerneltest.Snapshot(t, killed) || resumed.Revision != reduce(t, killed, true).Revision {
		t.Fatal("create-crash replay changed state")
	}
}
func TestS134Reducer_DeletedStepsAndFinal(t *testing.T) {
	f := reducerFixture(t)
	reduce(t, f, true)
	impl := f.Items[0] + ".impl.a1"
	delete(f.Fake.Records, f.ID(impl))
	reduce(t, f, true)
	if r, err := f.Store.Lookup(impl); err != nil || r == nil {
		t.Fatal("deleted implement was not recreated")
	}
	final := f.Epic + ".final"
	delete(f.Fake.Records, f.ID(final))
	out := reduce(t, f, true)
	r, err := f.Store.Lookup(final)
	f.Must(err)
	if r == nil || len(r.Dependencies) != 3 {
		t.Fatalf("recreated final has no work-item links: %+v", r)
	}
	before := kerneltest.Snapshot(t, f)
	again := reduce(t, f, true)
	if before != kerneltest.Snapshot(t, f) || again.Revision != out.Revision {
		t.Fatal("final recreation is not idempotent")
	}
}
func TestS134Reducer_CorePositive(t *testing.T) {
	f := reducerFixture(t)
	item := f.Items[0]
	gate := item + ".review"
	closePass(t, f, item+".impl.a1")
	out := reduce(t, f, true)
	if !hasRun(out, "mindspec", "panel", "create", item) {
		t.Fatalf("no panel create: %+v", out)
	}
	registerRound(t, f, 1)
	out = reduce(t, f, true)
	if len(out.Actions) != 3 {
		t.Fatalf("slots: %+v", out)
	}
	for i := 1; i <= 3; i++ {
		id := fmt.Sprintf("%s.r1.R%d", gate, i)
		if r, err := f.Store.Lookup(id); err != nil || r == nil {
			t.Fatalf("missing %s", id)
		}
	}
	// A separate gate can authorise the fifth round after four registered rounds.
	f = reducerFixture(t)
	item = f.Items[0]
	counted(t, f, 4)
	closePass(t, f, item+".review.r5.fix")
	out = reduce(t, f, true)
	if !hasRun(out, "mindspec", "panel", "create", item) {
		t.Fatalf("fix did not open next round: %+v", out)
	}
	f = reducerFixture(t)
	item = f.Items[0]
	kerneltest.PassGate(f, item+".review", reducerGit(t, f, "rev-parse", "bead/"+item))
	out = reduce(t, f, false)
	if !hasRun(out, "mindspec", "complete", item) {
		t.Fatalf("passing gate has no complete: %+v", out)
	}
}
func TestS134Reducer_RoundNeedsPassedReceiptAndOpenGate(t *testing.T) {
	for _, kind := range []string{"implement", "fix"} {
		for _, state := range []string{"raw", "superseded", "halted", "bad-receipt", "gate-passed", "authorised-gate-passed"} {
			t.Run(kind+"/"+state, func(t *testing.T) {
				f := reducerFixture(t)
				item := f.Items[0]
				gate := item + ".review"
				id := item + ".impl.a1"
				if kind == "fix" {
					counted(t, f, 4)
					id = gate + ".r5.fix"
				}
				var receipt *kernel.Receipt
				switch state {
				case "superseded":
					receipt = &kernel.Receipt{Kind: "superseded", Retally: "pass"}
				case "halted":
					receipt = &kernel.Receipt{Kind: "halted", HaltAnswer: "answer"}
				case "bad-receipt":
					receipt = &kernel.Receipt{Kind: "passed"}
				case "gate-passed", "authorised-gate-passed":
					receipt = &kernel.Receipt{Kind: "passed", Retally: "pass"}
				}
				seedClosed(t, f, id, receipt)
				if strings.Contains(state, "gate-passed") {
					if state == "authorised-gate-passed" {
						authorise(t, f, 2)
					}
					kerneltest.PassGate(f, gate, reducerGit(t, f, "rev-parse", "bead/"+item))
				}
				before, _, _ := kernel.Decode[kernel.GateRound](f.State().Records[gate].Metadata, kernel.KeyRound)
				out := reduce(t, f, true)
				after, _, _ := kernel.Decode[kernel.GateRound](f.State().Records[gate].Metadata, kernel.KeyRound)
				if hasRun(out, "mindspec", "panel", "create", item) || !reflect.DeepEqual(before, after) {
					t.Fatalf("invalid close opened round: %+v", out)
				}
				if strings.Contains(state, "gate-passed") && !hasRun(out, "mindspec", "complete", item) {
					t.Fatalf("frozen gate has no completion: %+v", out)
				}
			})
		}
	}
}
func TestS134Reducer_OpenJudgeBlocksComplete(t *testing.T) {
	f := reducerFixture(t)
	item := f.Items[0]
	gate := item + ".review"
	kerneltest.PassGate(f, gate, reducerGit(t, f, "rev-parse", "bead/"+item))
	head := newCommit(t, f, "bead/"+item)
	judge := kerneltest.Judge(f, gate, head, 1)
	if f.State().Records[judge].Parent != item {
		t.Fatal("judge fixture has wrong parent")
	}
	before := kerneltest.Snapshot(t, f)
	out := reduce(t, f, true)
	if hasRun(out, "mindspec", "complete", item) || before != kerneltest.Snapshot(t, f) {
		t.Fatalf("judge permits complete or writes: %+v", out)
	}
}
func TestS134Reducer_DoneNeedsMerge(t *testing.T) {
	f := reducerFixture(t)
	item := f.Items[0]
	head := newCommit(t, f, "bead/"+item)
	kerneltest.PassGate(f, item+".review", head)
	out := reduce(t, f, true)
	for _, a := range out.Actions {
		if strings.HasPrefix(a.Step, f.Items[1]) || strings.HasPrefix(a.Step, f.Items[2]) {
			t.Fatalf("unmerged pass released work: %+v", out)
		}
	}
	for _, id := range f.Items[1:] {
		if r, _ := f.Store.Lookup(id + ".impl.a1"); r != nil {
			t.Fatal("created downstream implement")
		}
	}
	f.Must(f.Store.SetStatusIf(item, "open", "closed"))
	out = reduce(t, f, true)
	if len(out.Actions) != 0 {
		t.Fatal("raw close of unmerged pass released work")
	}
	reducerGit(t, f, "update-ref", "refs/heads/spec/"+f.SpecID, head)
	out = reduce(t, f, true)
	if len(out.Actions) != 2 {
		t.Fatalf("merged pass did not release both dependents: %+v", out)
	}
}
func TestS134Reducer_AuthorisedSlotsAndSubjectMove(t *testing.T) {
	t.Run("unauthorised", func(t *testing.T) {
		f := reducerFixture(t)
		authorise(t, f, 1)
		registerRound(t, f, 2)
		out := reduce(t, f, true)
		if len(out.Actions) != 0 {
			t.Fatalf("registered unauthorised slots: %+v", out)
		}
		if len(f.State().Steps) != 0 {
			t.Fatal("created unauthorised slots")
		}
	})
	t.Run("subject-move", func(t *testing.T) {
		f := reducerFixture(t)
		item := f.Items[0]
		gate := item + ".review"
		closePass(t, f, item+".impl.a1")
		reduce(t, f, true)
		count, err := kernel.RoundsForGate(f.State(), gate)
		f.Must(err)
		head := newCommit(t, f, "bead/"+item)
		out := reduce(t, f, true)
		round, _, _ := kernel.Decode[kernel.GateRound](f.State().Records[gate].Metadata, kernel.KeyRound)
		after, err := kernel.RoundsForGate(f.State(), gate)
		f.Must(err)
		if round.Round != 1 || round.Subject != head || count != after || len(out.Run) != 1 || !hasRun(out, "mindspec", "panel", "create", item) || len(out.Actions) != 0 {
			t.Fatalf("subject move contract: %+v round %+v", out, round)
		}
	})
}
func TestS134Reducer_LeaseExpiryAndRefusals(t *testing.T) {
	f := reducerFixture(t)
	reduce(t, f, true)
	id := f.Items[0] + ".impl.a1"
	expires := fixedTime.Add(time.Hour)
	f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyLease, kernel.LeaseRecord{Generation: 2, Expires: expires, TokenHash: "hash"})))
	out := reduce(t, f, false)
	if len(out.Actions) != 0 || len(out.Waiting) != 1 || out.Waiting[0].Lease.Generation != 2 || out.RetryAt != expires.Format(time.RFC3339) || kernel.OutputExitCode(out) != 3 {
		t.Fatalf("lease not waiting: %+v", out)
	}
	kerneltest.SetClock(t, expires.Add(time.Second))
	after := reduce(t, f, false)
	if len(after.Actions) != 1 || after.RetryAt != "" || after.Revision == out.Revision {
		t.Fatalf("expired lease not released: %+v", after)
	}
	for _, failure := range []string{"silent", "no-step", "no-approval"} {
		t.Run(failure, func(t *testing.T) {
			f := reducerFixture(t)
			state := f.State()
			before := kerneltest.Snapshot(t, f)
			if failure == "silent" {
				f.Fake.Hook = func([]string) error { return kerneltest.ExitError(1) }
			}
			if failure == "no-step" {
				put(t, filepath.Join(f.Root, ".beads", "config.yaml"), "types.custom: gate\n")
				before = kerneltest.Snapshot(t, f)
			}
			if failure == "no-approval" {
				r := f.Fake.Records[f.Epic]
				delete(r.Metadata, kernel.KeyPlanApproval)
				f.Fake.Records[f.Epic] = r
				state = f.State()
				before = kerneltest.Snapshot(t, f)
			}
			_, err := kernel.NextAction(state, true)
			f.Fake.Hook = nil
			if err == nil || kerneltest.Snapshot(t, f) != before {
				t.Fatalf("refusal wrote or succeeded: %v", err)
			}
			if failure == "no-step" && !strings.Contains(err.Error(), "mindspec setup") {
				t.Fatalf("no setup recovery: %v", err)
			}
		})
	}
}

// passingPanel uses the real v2 submission and tally readers. Kernel receipts remain seeded.
func passingPanel(t *testing.T, f *kerneltest.Fixture) {
	t.Helper()
	item := f.Items[0]
	gate := item + ".review"
	closePass(t, f, item+".impl.a1")
	reduce(t, f, true)
	registerRound(t, f, 1)
	reduce(t, f, true)
	dir := filepath.Join(f.SpecDir, "reviews", "panel")
	head := reducerGit(t, f, "rev-parse", "bead/"+item)
	for i := 1; i <= 3; i++ {
		slot := fmt.Sprintf("R%d", i)
		v := paneltest.Verdict("APPROVE")
		v.Slot = slot
		v.ReviewedSHA = head
		v.Model = "test-model"
		_, err := panel.Submit(panel.SubmitTarget{Dir: dir, Spec: f.SpecID, Panel: "panel", Slot: slot, Round: 1}, paneltest.Bytes(t, v), func(panel.Panel, panel.RosterSlot) (panel.SubmitResolution, error) {
			return panel.SubmitResolution{Family: "harness", Model: "test-model", RunnerKind: "harness", RunnerSource: "default"}, nil
		})
		f.Must(err)
		id := gate + ".r1." + slot
		f.Must(f.Store.WriteReceipt(id, kernel.Receipt{Kind: "passed", Retally: "seeded-verdict"}))
		f.Must(f.Store.SetStatusIf(id, "open", "closed"))
	}
	res, err := panel.Tally(dir)
	f.Must(err)
	if !res.PassRule(true) {
		t.Fatalf("fixture is not a real passing tally: %+v", res)
	}
}
func TestS134Reducer_TallyForeignChildAndCAS(t *testing.T) {
	t.Run("foreign-child", func(t *testing.T) {
		f := reducerFixture(t)
		passingPanel(t, f)
		gate := f.Items[0] + ".review"
		foreign := gate + ".1"
		f.Fake.Records[foreign] = kernel.Record{ID: foreign, Parent: f.ID(gate), Type: "task", Status: "open"}
		out := reduce(t, f, true)
		if f.State().Records[gate].Status == "closed" || !hasFlag(out, "foreign-child-open") || kernel.OutputExitCode(out) != 3 {
			t.Fatalf("foreign child did not hold gate: %+v", out)
		}
		r := f.Fake.Records[foreign]
		r.Status = "closed"
		f.Fake.Records[foreign] = r
		reduce(t, f, true)
		receipt, _, _ := kernel.Decode[kernel.Receipt](f.State().Records[gate].Metadata, kernel.KeyClose)
		if f.State().Records[gate].Status != "closed" || receipt.Kind != "passed" {
			t.Fatal("closing child did not release tally")
		}
	})
	t.Run("cas-retry", func(t *testing.T) {
		f := reducerFixture(t)
		passingPanel(t, f)
		calls, closes := 0, 0
		f.Fake.Hook = func(args []string) error {
			if strings.Contains(strings.Join(args, " "), "--if-status") {
				calls++
				if calls == 1 {
					return kerneltest.ExitError(13)
				}
				closes++
			}
			return nil
		}
		out := reduce(t, f, true)
		if calls != 2 || closes != 1 || !hasRun(out, "mindspec", "complete", f.Items[0]) {
			t.Fatalf("CAS retry failed: calls=%d closes=%d %+v", calls, closes, out)
		}
	})
}
func TestS134SeedMatch_PassGate(t *testing.T) {
	real := reducerFixture(t)
	seed := reducerFixture(t)
	passingPanel(t, real)
	passingPanel(t, seed)
	gate := real.Items[0] + ".review"
	reduce(t, real, true)
	kerneltest.PassGate(seed, gate, reducerGit(t, seed, "rev-parse", "bead/"+seed.Items[0]))
	a, b := real.State().Records[gate], seed.State().Records[gate]
	if !jsonEqual(a, b) {
		t.Fatalf("PassGate differs from tally:\n%+v\n%+v", a, b)
	}
}
func TestS134SeedMatch_Authorise(t *testing.T) {
	for _, n := range []int{1, 5} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			real := reducerFixture(t)
			seed := reducerFixture(t)
			id := real.Items[0] + ".impl.a1"
			if n > 1 {
				counted(t, real, n-1)
				counted(t, seed, n-1)
				id = real.Items[0] + fmt.Sprintf(".review.r%d.fix", n)
			}
			closePass(t, real, id)
			closePass(t, seed, id)
			reduce(t, real, true)
			authorise(t, seed, n)
			gate := real.Items[0] + ".review"
			a, b := real.State().Records[gate], seed.State().Records[gate]
			if !jsonEqual(a, b) {
				t.Fatalf("Authorise differs from producer:\n%+v\n%+v", a, b)
			}
		})
	}
}

func TestS134Reducer_FrozenSupersededFix(t *testing.T) {
	f := reducerFixture(t)
	gate := f.Items[0] + ".review"
	seedClosed(t, f, gate+".r2.fix", &kernel.Receipt{Kind: "superseded", Retally: "passing-tally"})
	kerneltest.PassGate(f, gate, reducerGit(t, f, "rev-parse", "bead/"+f.Items[0]))
	before := kerneltest.Snapshot(t, f)
	out := reduce(t, f, true)
	if before != kerneltest.Snapshot(t, f) || hasRun(out, "mindspec", "panel", "create", f.Items[0]) {
		t.Fatalf("frozen gate was reopened: %+v", out)
	}
}
func TestS134Reducer_BranchRemovalKeepsMerge(t *testing.T) {
	f := reducerFixture(t)
	item := f.Items[0]
	base := reducerGit(t, f, "rev-parse", "spec/"+f.SpecID)
	head := newCommit(t, f, "bead/"+item)
	kerneltest.PassGate(f, item+".review", head)
	f.Must(f.Store.SetStatusIf(item, "open", "closed"))
	tree := reducerGit(t, f, "rev-parse", head+"^{tree}")
	merge := reducerGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit-tree", tree, "-p", base, "-p", head, "-m", "Merge bead/"+item)
	reducerGit(t, f, "update-ref", "refs/heads/spec/"+f.SpecID, merge)
	f.Must(f.Store.WriteMetadata(item, map[string]any{"mindspec_landed_merge_sha": merge, "mindspec_landed_second_parent": head}))
	reducerGit(t, f, "update-ref", "-d", "refs/heads/bead/"+item)
	if out := reduce(t, f, true); len(out.Actions) != 2 {
		t.Fatalf("removed merged branch held downstream: %+v", out)
	}
}

func TestS134Reducer_PassedGateHasPendingAuthorisation(t *testing.T) {
	f := reducerFixture(t)
	item := f.Items[0]
	gate := item + ".review"
	registerRound(t, f, 1)
	closePass(t, f, gate+".r2.fix")
	authorise(t, f, 2)
	kerneltest.PassGate(f, gate, reducerGit(t, f, "rev-parse", "bead/"+item))
	before := kerneltest.Snapshot(t, f)
	out := reduce(t, f, true)
	if !hasRun(out, "mindspec", "complete", item) || hasRun(out, "mindspec", "panel", "create", item) || kerneltest.Snapshot(t, f) != before {
		t.Fatalf("passed gate with a pending authorisation changed: %+v", out)
	}
}

func TestS134Reducer_UnapprovedReadOnlyWritesNothing(t *testing.T) {
	f := reducerFixture(t)
	r := f.Fake.Records[f.Epic]
	delete(r.Metadata, kernel.KeyPlanApproval)
	f.Fake.Records[f.Epic] = r
	before := kerneltest.Snapshot(t, f)
	_, err := kernel.NextAction(f.State(), false)
	if err == nil || !strings.Contains(err.Error(), "plan is not approved") || before != kerneltest.Snapshot(t, f) {
		t.Fatalf("unapproved read did not refuse without writes: %v", err)
	}
}

func TestS134Reducer_TallyFreshSubjectAndRound(t *testing.T) {
	for _, mismatch := range []string{"subject", "round"} {
		t.Run(mismatch, func(t *testing.T) {
			f := reducerFixture(t)
			passingPanel(t, f)
			gate := f.Items[0] + ".review"
			if mismatch == "subject" {
				newCommit(t, f, "bead/"+f.Items[0])
			}
			state := f.State()
			if mismatch == "round" {
				// The real tally still passes round 1; this candidate is a different round.
				state.Panels[0].Panel.Round = 2
			}
			out, err := kernel.NextAction(state, false)
			f.Must(err)
			if hasRun(out, "mindspec", "complete", f.Items[0]) {
				t.Fatalf("mismatched %s tally passed the gate: %+v", mismatch, out)
			}
			// Also inspect the overlaid state, independent of the completion command.
			result, err := kernel.Pass(kernel.Rules(), state, false)
			f.Must(err)
			if result.State.Records[gate].Status == "closed" {
				t.Fatalf("mismatched %s tally closed the gate", mismatch)
			}
		})
	}
}

func TestS134Reducer_SlotsNeedRegisteredSubject(t *testing.T) {
	f := reducerFixture(t)
	authorise(t, f, 1)
	registerRound(t, f, 1)
	state := f.State()
	gate := f.Items[0] + ".review"
	r := state.Records[gate]
	reg, _, err := kernel.Decode[kernel.RegistrationRecord](r.Metadata, kernel.KeyRegistration)
	f.Must(err)
	reg.Subject = "different-subject"
	r.Metadata[kernel.KeyRegistration] = reg
	state.Records[gate] = r
	result, err := kernel.Pass(kernel.Rules(), state, false)
	f.Must(err)
	if len(result.State.Steps) != 0 || len(result.Output.Actions) != 0 {
		t.Fatalf("stale registration created slots: %v", result.State.Steps)
	}
}

func TestS134Reducer_BranchRemovalRejectsWrongParent(t *testing.T) {
	f := reducerFixture(t)
	item := f.Items[0]
	base := reducerGit(t, f, "rev-parse", "spec/"+f.SpecID)
	head := newCommit(t, f, "bead/"+item)
	kerneltest.PassGate(f, item+".review", head)
	f.Must(f.Store.SetStatusIf(item, "open", "closed"))
	tree := reducerGit(t, f, "rev-parse", head+"^{tree}")
	merge := reducerGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit-tree", tree, "-p", base, "-p", head, "-m", "Merge bead/"+item)
	reducerGit(t, f, "update-ref", "refs/heads/spec/"+f.SpecID, merge)
	// A valid two-parent merge exists, but its second parent is not the recorded commit.
	f.Must(f.Store.WriteMetadata(item, map[string]any{"mindspec_landed_merge_sha": merge, "mindspec_landed_second_parent": base}))
	reducerGit(t, f, "update-ref", "-d", "refs/heads/bead/"+item)
	if f.State().Merged[item] {
		t.Fatal("wrong second parent counted as merged")
	}
	if out := reduce(t, f, true); len(out.Actions) != 0 {
		t.Fatalf("wrong second parent released downstream: %+v", out)
	}
}
