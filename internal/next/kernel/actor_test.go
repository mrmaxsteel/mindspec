package kernel_test

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func actorCommand(t *testing.T, f *kerneltest.Fixture, args ...string) string {
	t.Helper()
	data, err := f.Command("git", args...)
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, data)
	}
	return strings.TrimSpace(string(data))
}
func actorFixture(t *testing.T) (*kerneltest.Fixture, string, kernel.LeaseDeps) {
	t.Helper()
	kerneltest.SetClock(t, fixedTime)
	f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
	actorCommand(t, f, "branch", "spec/"+f.SpecID)
	actorCommand(t, f, "branch", "bead/"+f.Items[0])
	id := f.Items[0] + ".impl.a1"
	f.Create(id, f.Items[0])
	deps := kernel.LeaseDeps{EnsureWorktree: func(root, item, spec string) (string, error) {
		if root != f.Root || item != f.Items[0] || spec != f.SpecID {
			t.Fatal("wrong worktree inputs")
		}
		return f.Root, nil
	}}
	return f, id, deps
}
func leaseActor(t *testing.T, f *kerneltest.Fixture, id string, deps kernel.LeaseDeps) kernel.LeaseResult {
	t.Helper()
	result, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, deps)
	f.Must(err)
	return result
}
func actorCommit(t *testing.T, f *kerneltest.Fixture, path, content, branch string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(f.Root, path)), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.Root, path), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	actorCommand(t, f, "add", "--", path)
	actorCommand(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "evidence")
	commit := actorCommand(t, f, "rev-parse", "HEAD")
	if branch != "" {
		actorCommand(t, f, "update-ref", "refs/heads/"+branch, commit)
	}
	return commit
}
func actorRecord(t *testing.T, f *kerneltest.Fixture, id string) kernel.Record {
	t.Helper()
	r, err := f.Store.Lookup(id)
	f.Must(err)
	if r == nil {
		t.Fatal("record absent")
	}
	return *r
}
func actorReceipt(t *testing.T, f *kerneltest.Fixture, id string) kernel.Receipt {
	t.Helper()
	receipt, ok, err := kernel.Decode[kernel.Receipt](actorRecord(t, f, id).Metadata, kernel.KeyClose)
	f.Must(err)
	if !ok {
		t.Fatal("missing receipt")
	}
	return receipt
}
func actorRefusal(t *testing.T, f *kerneltest.Fixture, call func() error, contains string) {
	t.Helper()
	if contains == "" {
		t.Fatal("refusal must name its expected text")
	}
	before := kerneltest.Snapshot(t, f)
	err := call()
	if err == nil || !strings.Contains(err.Error(), contains) {
		t.Fatalf("wanted refusal %q, got %v", contains, err)
	}
	if after := kerneltest.Snapshot(t, f); after != before {
		t.Fatal("refusal changed snapshot")
	}
}

func TestS134Lease_GenerationsPinnedTTLAndTokens(t *testing.T) {
	f, id, deps := actorFixture(t)
	policy := kernel.DefaultPolicy
	policy.LeaseTTL = "90m"
	f.Must(f.Store.WriteMetadata(f.Epic, kernel.Metadata(kernel.KeyRun, policy)))
	if err := os.WriteFile(filepath.Join(f.Root, ".mindspec", "config.yaml"), []byte("panel:\n  lease_ttl: 10m\n"), 0644); err != nil {
		t.Fatal(err)
	}
	result := leaseActor(t, f, id, deps)
	if result.Token == "" || result.Lease.Generation != 1 || result.Lease.ClaimBase != actorCommand(t, f, "rev-parse", "bead/"+f.Items[0]) || !result.Lease.Expires.Equal(fixedTime.Add(90*time.Minute)) {
		t.Fatalf("bad first lease: %+v", result)
	}
	stored, _, err := kernel.Decode[kernel.LeaseRecord](actorRecord(t, f, id).Metadata, kernel.KeyLease)
	f.Must(err)
	raw, err := json.Marshal(actorRecord(t, f, id))
	f.Must(err)
	expectedHash := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(result.Token)))
	if !reflect.DeepEqual(stored, result.Lease) || stored.TokenHash != expectedHash || strings.Contains(string(raw), result.Token) {
		t.Fatal("lease not persisted or token leaked")
	}
	actorRefusal(t, f, func() error { _, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, deps); return err }, "lease --take")
	kerneltest.SetClock(t, fixedTime.Add(time.Hour))
	renewed, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Renew: result.Token}, deps)
	f.Must(err)
	if !renewed.Lease.Expires.Equal(fixedTime.Add(150*time.Minute)) || renewed.Lease.Generation != 1 || renewed.Token != "" {
		t.Fatal("renew did not use pinned duration or exposed token")
	}
	taken, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Take: true, Reason: "replace actor"}, deps)
	f.Must(err)
	if taken.Lease.Generation != 2 || taken.Token == result.Token {
		t.Fatal("take did not fence old actor")
	}
	take, _, err := kernel.Decode[kernel.TakeReason](actorRecord(t, f, id).Metadata, kernel.KeyTake)
	f.Must(err)
	if take.Generation != 2 || take.Reason != "replace actor" || !take.At.Equal(kernel.Now()) {
		t.Fatalf("missing take reason: %+v", take)
	}
	for _, o := range []kernel.LeaseOptions{{Step: id, Renew: result.Token}, {Step: id, Release: result.Token}, {Step: id, Take: true}} {
		want := "lease token is not current"
		if o.Take {
			want = "lease --take requires --reason"
		}
		actorRefusal(t, f, func() error { _, err := kernel.Lease(f.State(), o, deps); return err }, want)
	}
	_, err = kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Release: taken.Token}, deps)
	f.Must(err)
	released, _, _ := kernel.Decode[kernel.LeaseRecord](actorRecord(t, f, id).Metadata, kernel.KeyLease)
	if released.TokenHash != "" || actorRecord(t, f, id).Status != "open" {
		t.Fatal("release did not free step")
	}
	third := leaseActor(t, f, id, deps)
	if third.Lease.Generation != 3 {
		t.Fatal("release reset generation")
	}
	kerneltest.SetClock(t, third.Lease.Expires.Add(-30*time.Second))
	actorRefusal(t, f, func() error { _, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, deps); return err }, "lease --take")
	kerneltest.SetClock(t, third.Lease.Expires.Add(time.Second))
	fourth := leaseActor(t, f, id, deps)
	if fourth.Lease.Generation != 4 {
		t.Fatal("expiry did not advance generation")
	}
}

func TestS134Lease_ExpiredFirstLease(t *testing.T) {
	f, id, deps := actorFixture(t)
	first := leaseActor(t, f, id, deps)
	kerneltest.SetClock(t, first.Lease.Expires.Add(time.Second))
	if second := leaseActor(t, f, id, deps); second.Lease.Generation != 2 {
		t.Fatalf("expired first lease: %+v", second)
	}
}
func TestS134Lease_RefusalTable(t *testing.T) {
	for _, name := range []string{"foreign", "gate", "decision", "pre-kernel", "halted", "held-step", "held-parent", "held-upstream", "unanswered", "foreign-blocker"} {
		t.Run(name, func(t *testing.T) {
			f, id, deps := actorFixture(t)
			want := ""
			switch name {
			case "foreign":
				raw, err := f.Store.Run("create", "--title", "foreign", "--type", "bug", "--parent", f.Epic, "--json")
				f.Must(err)
				var foreign kernel.Record
				f.Must(json.Unmarshal(raw, &foreign))
				id = foreign.ID
				want = "foreign step"
			case "gate":
				id = f.Items[0] + ".review"
				want = "identity is not an actor step"
			case "decision":
				id = kerneltest.OpenDecision(f, f.Items[0]+".review", "reject", 1, "fix", "halt")
				want = "identity is not an actor step"
			case "pre-kernel":
				f = kerneltest.PreKernelSpec(t, kerneltest.WithFake(false))
				id = f.Items[0]
				want = "this spec uses the existing lifecycle"
			case "halted":
				d := kerneltest.OpenDecision(f, f.Items[0]+".review", "reject", 1, "fix", "halt")
				kerneltest.Answer(f, d, "halt")
				want = d
			case "held-step":
				f.Must(f.Store.SetStatusIf(id, "open", "blocked"))
				want = id
			case "held-parent":
				f.Must(f.Store.SetStatusIf(f.Items[0], "open", "deferred"))
				want = f.Items[0]
			case "held-upstream":
				f.Must(f.Store.AddLink(id, f.Items[1]))
				want = f.Items[1]
			case "unanswered":
				id = f.Items[0] + ".review.r2.fix"
				f.Create(id, f.Items[0]+".review")
				want = kerneltest.OpenDecision(f, f.Items[0]+".review", "reject", 1, "fix", "halt")
			case "foreign-blocker":
				raw, err := f.Store.Run("create", "--title", "foreign blocker", "--type", "bug", "--json")
				f.Must(err)
				var foreign kernel.Record
				f.Must(json.Unmarshal(raw, &foreign))
				want = foreign.ID
				_, err = f.Store.Run("dep", "add", f.Items[0], foreign.ID, "--type", "blocks")
				f.Must(err)
				id = f.Items[0] + ".review.r1.R1"
				f.Create(id, f.Items[0]+".review")
			}
			actorRefusal(t, f, func() error { _, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, deps); return err }, want)
		})
	}
}
func TestS134Lease_AtomicCASAndForeignIsolation(t *testing.T) {
	f, id, deps := actorFixture(t)
	// Isolate actor CAS from the first implementation phase transition.
	// The command tests cover that transition and its cache-failure recovery.
	f.Must(f.Store.WriteMetadata(f.Epic, map[string]any{"mindspec_phase": "implement"}))
	_, err := f.Store.Run("update", f.ID(id), "--claim")
	f.Must(err) // raw claim, no kernel lease
	if actorRecord(t, f, id).Status != "in_progress" {
		t.Fatal("fake raw claim did not claim")
	}
	for _, title := range []string{"foreign note", "foreign bug"} {
		typ := "task"
		if title == "foreign bug" {
			typ = "bug"
		}
		raw, e := f.Store.Run("create", "--title", title, "--type", typ, "--parent", f.Epic, "--json")
		f.Must(e)
		if typ == "bug" {
			var bug kernel.Record
			f.Must(json.Unmarshal(raw, &bug))
			_, e = f.Store.Run("dep", "add", f.Items[1], bug.ID, "--type", "blocks")
			f.Must(e)
		}
	}
	before, err := f.Store.All()
	f.Must(err)
	beforeLease := kerneltest.Snapshot(t, f)
	leaseRow := actorRecord(t, f, id)
	assertOnlyActorChanged := func(before string, original kernel.Record) {
		current := f.Fake.Records[original.ID]
		f.Fake.Records[original.ID] = original
		normalized := kerneltest.Snapshot(t, f)
		f.Fake.Records[original.ID] = current
		if normalized != before {
			t.Fatal("snapshot changed outside the computed actor identity")
		}
	}
	calls := 0
	f.Fake.Hook = func(args []string) error {
		if args[0] == "update" && strings.Contains(strings.Join(args, " "), "--if-status") {
			calls++
			if !strings.Contains(strings.Join(args, " "), "--metadata") {
				t.Fatal("lease status and token are not atomic")
			}
			if calls == 1 {
				return kerneltest.ExitError(13)
			}
		}
		return nil
	}
	lease := leaseActor(t, f, id, deps)
	if calls != 2 || lease.Lease.Generation != 1 {
		t.Fatal("lease did not retry fresh CAS")
	}
	f.Fake.Hook = nil
	assertOnlyActorChanged(beforeLease, leaseRow)
	commit := actorCommit(t, f, "proof.json", "{}", "bead/"+f.Items[0])
	beforeSubmit := kerneltest.Snapshot(t, f)
	gateID := f.Items[0] + ".review"
	gateBefore := actorRecord(t, f, gateID)
	submitRow := actorRecord(t, f, id)
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Commit: commit})
	f.Must(err)
	// The close authorises a review round. Check that write in full before
	// checking that all other rows, links and git state stay the same.
	n, err := config.DefaultConfig().PanelGateExpectedReviewers("bead")
	f.Must(err)
	wantRound := kernel.GateRound{Round: 1, Subject: commit, SlotCount: n, Slots: []string{}}
	for i := 1; i <= n; i++ {
		wantRound.Slots = append(wantRound.Slots, fmt.Sprintf("%s.r1.R%d", gateID, i))
	}
	gateAfter := actorRecord(t, f, gateID)
	wantGate := gateBefore
	wantGate.Metadata = map[string]any{}
	for key, value := range gateBefore.Metadata {
		wantGate.Metadata[key] = value
	}
	wantGate.Metadata[kernel.KeyRound] = wantRound
	wantJSON, err := json.Marshal(wantGate)
	f.Must(err)
	gotJSON, err := json.Marshal(gateAfter)
	f.Must(err)
	var wantValue, gotValue any
	f.Must(json.Unmarshal(wantJSON, &wantValue))
	f.Must(json.Unmarshal(gotJSON, &gotValue))
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("unexpected gate write: got %s want %s", gotJSON, wantJSON)
	}
	f.Fake.Records[gateBefore.ID] = gateBefore
	assertOnlyActorChanged(beforeSubmit, submitRow)
	f.Fake.Records[gateBefore.ID] = gateAfter
	after, err := f.Store.All()
	f.Must(err)
	for i, r := range before {
		if r.ID != f.ID(id) && r.ID != gateBefore.ID && !reflect.DeepEqual(r, after[i]) {
			t.Fatalf("foreign or other record changed: %s", r.ID)
		}
	}
}

func TestS134Submit_ImplementReceiptGoldenAndRetry(t *testing.T) {
	f, id, deps := actorFixture(t)
	lease := leaseActor(t, f, id, deps)
	commit := actorCommit(t, f, "proof.json", "{}", "bead/"+f.Items[0])
	o := kernel.SubmitOptions{Step: id, Lease: lease.Token, Commit: commit}
	_, err := kernel.Submit(f.State(), o)
	f.Must(err)
	receipt := actorReceipt(t, f, id)
	if receipt.Kind != "passed" || receipt.Commit != commit || receipt.Path != "proof.json" || receipt.Blob != actorCommand(t, f, "rev-parse", commit+":proof.json") || actorRecord(t, f, id).Status != "closed" {
		t.Fatalf("bad producer receipt: %+v", receipt)
	}
	normal := receipt
	normal.Commit = "commit"
	normal.Blob = "blob"
	golden(t, "receipt-implement", normal) // R1-F2: real producer, not a hand-built receipt
	// Register the authorised round before testing retries as the branch moves.
	gate := f.Items[0] + ".review"
	round, ok, err := kernel.Decode[kernel.GateRound](actorRecord(t, f, gate).Metadata, kernel.KeyRound)
	f.Must(err)
	if !ok || round.Round != 1 || round.Subject != commit || round.SlotCount < 1 {
		t.Fatalf("submit did not authorise its review: %+v", round)
	}
	slots := []string{}
	for i := 1; i <= round.SlotCount; i++ {
		slots = append(slots, fmt.Sprintf("R%d", i))
	}
	kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "panel", Round: 1, Subject: commit, Slots: slots})
	_, err = kernel.Pass(kernel.Rules(), f.State(), true)
	f.Must(err)
	before, err := f.Store.Run("show", f.ID(id), "--json")
	f.Must(err)
	calls := len(f.Fake.Calls)
	_, err = kernel.Submit(f.State(), o)
	f.Must(err)
	for _, args := range f.Fake.Calls[calls:] {
		if args[0] == "update" {
			t.Fatal("idempotent retry wrote")
		}
	}
	after, err := f.Store.Run("show", f.ID(id), "--json")
	f.Must(err)
	if string(after) != string(before) {
		t.Fatal("rerun changed bd show")
	}
	other := actorCommit(t, f, "other.json", "{}", "bead/"+f.Items[0])
	o.Commit = other
	actorRefusal(t, f, func() error { _, err := kernel.Submit(f.State(), o); return err }, "different result")
}
func TestS134Submit_BadImplement(t *testing.T) {
	for _, name := range []string{"old-token", "bad-token", "no-token", "pre-lease", "not-descendant", "wrong-branch", "missing-commit", "no-artifact", "verdict-instead", "expired", "forged-kind", "new-blocker"} {
		t.Run(name, func(t *testing.T) {
			f, id, deps := actorFixture(t)
			base := actorCommand(t, f, "rev-parse", "HEAD")
			if name == "pre-lease" {
				base = actorCommit(t, f, "earlier.json", "{}", "bead/"+f.Items[0])
			}
			lease := leaseActor(t, f, id, deps)
			if name == "old-token" {
				_, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Take: true, Reason: "replace"}, deps)
				f.Must(err)
			}
			commit := actorCommit(t, f, "proof.json", "{}", "bead/"+f.Items[0])
			o := kernel.SubmitOptions{Step: id, Lease: lease.Token, Commit: commit}
			switch name {
			case "bad-token":
				o.Lease = "invalid-token"
			case "no-token":
				o.Lease = ""
			case "pre-lease":
				o.Commit = base
			case "not-descendant":
				tree := actorCommand(t, f, "rev-parse", commit+"^{tree}")
				o.Commit = actorCommand(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit-tree", tree, "-m", "unrelated evidence")
			case "wrong-branch":
				actorCommand(t, f, "update-ref", "refs/heads/bead/"+f.Items[0], base)
			case "missing-commit":
				o.Commit = strings.Repeat("0", 40)
			case "no-artifact":
				o.Commit = ""
			case "verdict-instead":
				o.Commit = ""
				o.Artifact = "proof.json"
			case "expired":
				kerneltest.SetClock(t, lease.Lease.Expires.Add(time.Second))
			case "forged-kind":
				f.Must(f.Store.WriteReceipt(id, kernel.Receipt{Kind: "passed", Retally: "forged"}))
			case "new-blocker":
				f.Must(f.Store.AddLink(id, f.Items[1]))
			}
			want := map[string]string{
				"old-token": "lease token is not current", "bad-token": "lease token is not current", "no-token": "lease token is not current",
				"pre-lease": "commit is not past the claim base or round subject", "not-descendant": "commit is not past the claim base or round subject",
				"wrong-branch": "commit must be on bead/", "missing-commit": "cannot resolve committed evidence", "no-artifact": "submit needs one record, artifact or commit",
				"verdict-instead": "artifact does not fit the step kind", "expired": "lease has expired", "forged-kind": "step already has a different result", "new-blocker": "step is held or blocked by " + f.Items[1],
			}[name]
			actorRefusal(t, f, func() error { _, err := kernel.Submit(f.State(), o); return err }, want)
			if actorRecord(t, f, id).Status == "closed" {
				t.Fatal("bad submit closed step")
			}
		})
	}
}
func slotFixture(t *testing.T) (*kerneltest.Fixture, string, kernel.LeaseResult, string) {
	t.Helper()
	f, _, _ := actorFixture(t)
	gate := f.Items[0] + ".review"
	id := gate + ".r1.R1"
	subject := actorCommand(t, f, "rev-parse", "HEAD")
	f.Create(id, gate)
	kerneltest.Authorise(f, gate, kernel.GateRound{Round: 1, Subject: subject, Slots: []string{gate + ".r1.R1", gate + ".r1.R2"}, SlotCount: 2})
	f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeySubject, kernel.Subject{Commit: subject})))
	lease := leaseActor(t, f, id, kernel.LeaseDeps{})
	return f, id, lease, subject
}
func verdictBytes(subject, slot string) string {
	return `{"schema":2,"slot":"` + slot + `","round":1,"reviewed_sha":"` + subject + `","status":"final","verdict":"APPROVE","model":"test","rationale":"reviewed","concrete_changes_required":[],"commands_run":[]}`
}
func TestS134Submit_BadVerdictsAndFix(t *testing.T) {
	for _, name := range []string{"old-subject", "another-slot", "changed-blob", "uncommitted", "missing", "stale-round", "stale-step-subject", "draft", "foreign-slot", "verdict-round", "unlisted-slot", "old-token", "bad-token"} {
		t.Run(name, func(t *testing.T) {
			f, id, lease, subject := slotFixture(t)
			data := verdictBytes(subject, "R1")
			if name == "old-subject" {
				data = verdictBytes(strings.Repeat("0", 40), "R1")
			}
			if name == "another-slot" {
				data = verdictBytes(subject, "R2")
			}
			if name == "verdict-round" {
				data = strings.Replace(data, `"round":1`, `"round":2`, 1)
			}
			if name == "draft" {
				data = strings.Replace(data, `"final"`, `"draft"`, 1)
			}
			actorCommit(t, f, "verdict.json", data, "")
			switch name {
			case "old-token":
				_, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Take: true, Reason: "replace"}, kernel.LeaseDeps{})
				f.Must(err)
			case "bad-token":
				lease.Token = "invalid-token"
			case "unlisted-slot":
				kerneltest.Authorise(f, f.Items[0]+".review", kernel.GateRound{Round: 1, Subject: subject, Slots: []string{"R2"}, SlotCount: 1})
			case "changed-blob":
				f.Must(os.WriteFile(filepath.Join(f.Root, "verdict.json"), []byte(data+" "), 0644))
			case "uncommitted":
				f.Must(os.WriteFile(filepath.Join(f.Root, "new.json"), []byte(data), 0644))
			case "stale-round":
				kerneltest.Authorise(f, f.Items[0]+".review", kernel.GateRound{Round: 2, Subject: subject, Slots: []string{"R1"}, SlotCount: 1})
			case "stale-step-subject":
				f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeySubject, kernel.Subject{Commit: strings.Repeat("0", 40)})))
			case "foreign-slot":
				r := f.Fake.Records[f.ID(id)]
				delete(r.Metadata, kernel.KeyTuple)
				f.Fake.Records[r.ID] = r
			}
			artifact := "verdict.json"
			if name == "missing" {
				artifact = "missing.json"
			}
			if name == "uncommitted" {
				artifact = "new.json"
			}
			want := map[string]string{
				"old-subject": "verdict slot, round or subject does not match the step", "another-slot": "verdict slot, round or subject does not match the step",
				"verdict-round": "verdict slot, round or subject does not match the step", "draft": "verdict slot, round or subject does not match the step",
				"changed-blob": "artifact blob differs from its holding commit", "uncommitted": "artifact has no holding commit", "missing": "artifact must be a regular committed file",
				"stale-round": "step subject or round is stale", "stale-step-subject": "step subject is stale", "foreign-slot": "foreign step", "unlisted-slot": "slot is not in the active gate round",
				"old-token": "lease token is not current", "bad-token": "lease token is not current",
			}[name]
			actorRefusal(t, f, func() error {
				_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: artifact})
				return err
			}, want)
			if actorRecord(t, f, id).Status == "closed" {
				t.Fatal("bad verdict closed slot")
			}
		})
	}
	t.Run("verdict-to-fix", func(t *testing.T) {
		f, _, _, subject := slotFixture(t)
		id := f.Items[0] + ".review.r2.fix"
		f.Create(id, f.Items[0]+".review")
		lease := leaseActor(t, f, id, kernel.LeaseDeps{})
		actorCommit(t, f, "verdict.json", verdictBytes(subject, "R1"), "")
		actorRefusal(t, f, func() error {
			_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: "verdict.json"})
			return err
		}, "kind")
	})
}

func TestS134Submit_ReceiptBeforeCloseCrashAndCAS(t *testing.T) {
	for _, mode := range []string{"crash", "first-conflict", "always-conflict"} {
		t.Run(mode, func(t *testing.T) {
			f, id, deps := actorFixture(t)
			lease := leaseActor(t, f, id, deps)
			commit := actorCommit(t, f, "proof.json", "{}", "bead/"+f.Items[0])
			o := kernel.SubmitOptions{Step: id, Lease: lease.Token, Commit: commit}
			writes, closes := 0, 0
			physical := f.ID(id)
			f.Fake.Hook = func(args []string) error {
				if len(args) < 2 || args[0] != "update" || args[1] != physical {
					return nil
				}
				joined := strings.Join(args, " ")
				if strings.Contains(joined, "--metadata") {
					writes++
				}
				if strings.Contains(joined, "--if-status") {
					closes++
					r := f.Fake.Records[f.ID(id)]
					if _, ok := r.Metadata[kernel.KeyClose]; !ok {
						t.Fatal("close preceded receipt")
					}
					if mode == "crash" {
						return errors.New("simulated kill after receipt")
					}
					if mode == "always-conflict" || closes == 1 {
						return kerneltest.ExitError(13)
					}
				}
				return nil
			}
			_, err := kernel.Submit(f.State(), o)
			f.Fake.Hook = nil
			receipt := actorReceipt(t, f, id)
			if receipt.Commit != commit || writes != 1 {
				t.Fatalf("receipt writes=%d receipt=%+v", writes, receipt)
			}
			if mode == "first-conflict" {
				if err != nil || closes != 2 || actorRecord(t, f, id).Status != "closed" {
					t.Fatalf("retry failed: %v closes=%d", err, closes)
				}
			} else {
				if err == nil || actorRecord(t, f, id).Status == "closed" {
					t.Fatal("fault did not leave open receipt")
				}
				if mode == "always-conflict" && closes != 2 {
					t.Fatal("unbounded retry")
				}
				passes := 0
				reg := kernel.NewRegistry("spy")
				f.Must(reg.Register("spy", kernel.Ordinary, func(s kernel.State) (kernel.Changes, error) {
					if r, exists := s.Records[id]; exists {
						if r.Status != "closed" {
							t.Fatal("retry pass preceded close")
						}
						passes++
					}
					return kernel.Changes{}, nil
				}))
				kerneltest.WithRules(t, reg)
				_, err = kernel.Submit(f.State(), o)
				f.Must(err)
				if passes != 1 {
					t.Fatalf("receipt retry post-close passes=%d", passes)
				}
				if actorRecord(t, f, id).Status != "closed" || !reflect.DeepEqual(receipt, actorReceipt(t, f, id)) {
					t.Fatal("receipt retry did not finish close")
				}
			}
		})
	}
}

func TestS134Submit_PassOnceAfterCloseAndError(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "spy", true: "error"}[fail], func(t *testing.T) {
			f, id, deps := actorFixture(t)
			lease := leaseActor(t, f, id, deps)
			commit := actorCommit(t, f, "proof.json", "{}", "bead/"+f.Items[0])
			count := 0
			reg := kernel.NewRegistry("spy")
			f.Must(reg.Register("spy", kernel.Ordinary, func(s kernel.State) (kernel.Changes, error) {
				if _, exists := s.Records[id]; !exists {
					return kernel.Changes{}, nil
				}
				count++
				if s.Records[id].Status != "closed" || actorRecord(t, f, id).Status != "closed" {
					t.Fatal("pass preceded close")
				}
				if fail {
					return kernel.Changes{}, errors.New("test rule failed")
				}
				return kernel.Changes{}, nil
			}))
			kerneltest.WithRules(t, reg)
			_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Commit: commit})
			if (err != nil) != fail || count != 1 || actorRecord(t, f, id).Status != "closed" || actorReceipt(t, f, id).Kind != "passed" {
				t.Fatalf("pass err=%v count=%d", err, count)
			}
		})
	}
}
func TestS134Submit_HaltedNoWrites(t *testing.T) {
	for _, kind := range []string{"implement", "verdict", "answer-change", "answer-new", "answer-new-halt", "answer-prior", "answer-foreign"} {
		for _, native := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/native=%t", kind, native), func(t *testing.T) {
				f, id, deps := actorFixture(t)
				lease := leaseActor(t, f, id, deps)
				commit := actorCommit(t, f, "proof.json", "{}", "bead/"+f.Items[0])
				o := kernel.SubmitOptions{Step: id, Lease: lease.Token, Commit: commit}
				if kind == "verdict" {
					var subject string
					f, id, lease, subject = slotFixture(t)
					actorCommit(t, f, "verdict.json", verdictBytes(subject, "R1"), "")
					o = kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: "verdict.json"}
				}
				d := kerneltest.OpenDecision(f, f.Items[0]+".review", "reject", 1, "fix", "halt")
				kerneltest.Answer(f, d, "halt")
				switch kind {
				case "answer-change":
					o = kernel.SubmitOptions{Step: d, Answer: "fix"}
				case "answer-new", "answer-new-halt", "answer-prior":
					otherDecision := kerneltest.OpenDecision(f, f.Items[1]+".review", "reject", 1, "fix", "halt")
					o = kernel.SubmitOptions{Step: otherDecision, Answer: "fix"}
					if kind == "answer-new-halt" {
						o.Answer = "halt"
					}
					if kind == "answer-prior" {
						kerneltest.Answer(f, otherDecision, "fix")
					}
				case "answer-foreign":
					o = kernel.SubmitOptions{Step: "foreign", Answer: "halt"}
				}
				f.Store.NativeIDs = native
				if kind != "answer-foreign" {
					o.Step = f.ID(o.Step)
				}
				other := f.Items[1] + ".impl.a1"
				f.Create(other, f.Items[1])
				before := kerneltest.Snapshot(t, f)
				count := 0
				reg := kernel.NewRegistry("ordinary", "halt", "spy")
				f.Must(reg.Register("ordinary", kernel.Ordinary, func(kernel.State) (kernel.Changes, error) {
					count++
					return kernel.Changes{}, nil
				}))
				f.Must(reg.Register("halt", kernel.Halt, func(kernel.State) (kernel.Changes, error) {
					count++
					return kernel.Changes{Writes: []kernel.Write{{Kind: "close", ID: other, Receipt: &kernel.Receipt{Kind: "halted", HaltAnswer: d}}}}, nil
				}))
				f.Must(reg.Register("spy", kernel.FlagOnly, func(kernel.State) (kernel.Changes, error) { count++; return kernel.Changes{}, nil }))
				kerneltest.WithRules(t, reg)
				var err error
				if o.Answer != "" {
					_, err = kernel.Answer(f.State(), o.Step, o.Answer)
				} else {
					_, err = kernel.Submit(f.State(), o)
				}
				want := fmt.Sprintf("spec is halted by %s (%s)\nrecovery: mindspec next-action %s", d, f.SpecID, f.SpecID)
				if err == nil || err.Error() != want {
					t.Errorf("halt refusal: %v", err)
				}
				if count != 0 {
					t.Errorf("halt refusal ran rules: calls=%d", count)
				}
				if kerneltest.Snapshot(t, f) != before {
					t.Error("halt refusal changed snapshot")
				}
			})
		}
	}
}
func TestS134Submit_FinalFixBranch(t *testing.T) {
	for _, final := range []bool{false, true} {
		t.Run(map[bool]string{false: "bead-fix", true: "final-fix"}[final], func(t *testing.T) {
			f, _, _ := actorFixture(t)
			gate := f.Items[0] + ".review"
			branch := "bead/" + f.Items[0]
			if final {
				gate = f.Epic + ".final"
				branch = "spec/" + f.SpecID
				// Final gate upstreams must be closed before its fix can run.
				for _, item := range f.Items {
					f.Must(f.Store.SetStatusIf(item, "open", "closed"))
				}
			}
			subject := actorCommand(t, f, "rev-parse", "HEAD")
			id := gate + ".r2.fix"
			f.Create(id, gate)
			kerneltest.Authorise(f, gate, kernel.GateRound{Round: 1, Subject: subject, Slots: []string{gate + ".r1.R1", gate + ".r1.R2"}, SlotCount: 2})
			kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "panel", Round: 1, Subject: subject, Slots: []string{"R1", "R2"}})
			_, err := kernel.Pass(kernel.Rules(), f.State(), true)
			f.Must(err)
			lease := leaseActor(t, f, id, kernel.LeaseDeps{})
			commit := actorCommit(t, f, "proof.json", "{}", "bead/"+f.Items[0])
			o := kernel.SubmitOptions{Step: id, Lease: lease.Token, Commit: commit}
			if final {
				actorRefusal(t, f, func() error { _, err := kernel.Submit(f.State(), o); return err }, branch)
				actorCommand(t, f, "update-ref", "refs/heads/"+branch, commit)
			}
			_, err = kernel.Submit(f.State(), o)
			f.Must(err)
			if actorRecord(t, f, id).Status != "closed" || actorReceipt(t, f, id).Kind != "passed" {
				t.Fatal("valid fix not passed")
			}
		})
	}
}
func TestS134SeedMatch_Answer(t *testing.T) {
	for _, option := range []string{"fix", "halt", "continue:2"} {
		t.Run(option, func(t *testing.T) {
			kerneltest.WithRules(t, kernel.NewRegistry())
			fixtures := []*kerneltest.Fixture{kerneltest.Enrolled(t, kerneltest.WithFake(true)), kerneltest.Enrolled(t, kerneltest.WithFake(true))}
			kerneltest.SetClock(t, fixedTime)
			var records []kernel.Record
			for i, f := range fixtures {
				d := kerneltest.OpenDecision(f, f.Items[0]+".review", "reject", 1, "fix", "halt", "continue:<n>")
				if i == 0 {
					_, err := kernel.Answer(f.State(), d, option)
					f.Must(err)
				} else {
					kerneltest.Answer(f, d, option)
				}
				records = append(records, actorRecord(t, f, d))
			}
			if !jsonEqual(records[0], records[1]) {
				t.Fatalf("answer producer differs from seeder: %+v / %+v", records[0], records[1])
			}
		})
	}
}
func TestS134SeedMatch_Repaired(t *testing.T) {
	kerneltest.WithRules(t, kernel.NewRegistry())
	f, id, lease, subject := slotFixture(t)
	commit := actorCommit(t, f, "verdict.json", verdictBytes(subject, "R1"), "")
	f.Must(f.Store.SetStatusIf(id, "in_progress", "closed")) // raw slot close
	twin := kerneltest.Enrolled(t, kerneltest.WithFake(false))
	// Share git evidence only; compare the full raw row on a copied fake store.
	twin.Root = f.Root
	twin.SpecID = f.SpecID
	twin.SpecDir = f.SpecDir
	twin.Epic = f.Epic
	twin.Items = f.Items
	twin.Store = kernel.NewStore(f.Root, f.Epic, twin.Fake.Run)
	rows, err := f.Store.All()
	f.Must(err)
	twin.Fake.Records = map[string]kernel.Record{}
	for _, row := range rows {
		data, _ := json.Marshal(row)
		var copy kernel.Record
		f.Must(json.Unmarshal(data, &copy))
		twin.Fake.Records[row.ID] = copy
	}
	receipt := kernel.Receipt{Kind: "passed", Path: "verdict.json", Commit: commit, Blob: actorCommand(t, f, "rev-parse", commit+":verdict.json"), Generation: lease.Lease.Generation}
	kerneltest.Repaired(twin, id, receipt)
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: "verdict.json"})
	f.Must(err)
	if !reflect.DeepEqual(actorRecord(t, f, id), actorRecord(t, twin, id)) || !actorReceipt(t, f, id).Repaired {
		t.Fatal("repair producer differs from seeder")
	}
	before := kerneltest.Snapshot(t, f)
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: "verdict.json"})
	f.Must(err)
	if kerneltest.Snapshot(t, f) != before {
		t.Fatal("repair retry changed snapshot")
	}
}
func TestS134Submit_AnswerRetryAndForeignChild(t *testing.T) {
	kerneltest.WithRules(t, kernel.NewRegistry())
	f, _, _ := actorFixture(t)
	d := kerneltest.OpenDecision(f, f.Items[0]+".review", "reject", 1, "fix", "halt")
	actorRefusal(t, f, func() error { _, err := kernel.Answer(f.State(), d, "accept"); return err }, "listed")
	raw, err := f.Store.Run("create", "--title", "foreign child", "--type", "task", "--parent", f.ID(d), "--json")
	f.Must(err)
	var child kernel.Record
	f.Must(json.Unmarshal(raw, &child))
	out, err := kernel.Answer(f.State(), d, "halt")
	f.Must(err)
	if out.Status != "halted" || !f.State().Halted() || actorRecord(t, f, d).Status == "closed" {
		t.Fatal("answer lost behind child")
	}
	actorRefusal(t, f, func() error { _, err := kernel.Answer(f.State(), d, "fix"); return err }, "halt")
	_, err = f.Store.Run("update", child.ID, "--status", "closed")
	f.Must(err)
	out, err = kernel.Answer(f.State(), d, "halt")
	f.Must(err)
	if out.Status != "halted" || actorRecord(t, f, d).Status != "closed" {
		t.Fatal("halt retry did not close decision")
	}
	before := kerneltest.Snapshot(t, f)
	_, err = kernel.Answer(f.State(), d, "halt")
	f.Must(err)
	if before != kerneltest.Snapshot(t, f) {
		t.Fatal("answer retry wrote")
	}
}

func TestS134Submit_HaltRetryFinishesPass(t *testing.T) {
	f, id, _ := actorFixture(t)
	d := kerneltest.OpenDecision(f, f.Items[0]+".review", "reject", 1, "fix", "halt")
	reg := kernel.NewRegistry("finish")
	fail := true
	retry := false
	count := 0
	f.Must(reg.Register("finish", kernel.Halt, func(s kernel.State) (kernel.Changes, error) {
		if _, exists := s.Records[d]; !exists {
			if retry {
				t.Fatal("halt retry ran a pass before the answer check")
			}
			return kernel.Changes{}, nil
		}
		count++
		if s.Records[d].Status != "closed" {
			t.Fatal("halt pass before decision close")
		}
		if fail {
			return kernel.Changes{}, errors.New("halt interrupted")
		}
		return kernel.Changes{Writes: []kernel.Write{{Kind: "close", ID: id, Receipt: &kernel.Receipt{Kind: "halted", HaltAnswer: d}}}}, nil
	}))
	kerneltest.WithRules(t, reg)
	_, err := kernel.Answer(f.State(), d, "halt")
	if err == nil || actorRecord(t, f, d).Status != "closed" || actorRecord(t, f, id).Status == "closed" {
		t.Fatal("did not leave interrupted halt")
	}
	fail = false
	retry = true
	out, err := kernel.Answer(f.State(), d, "halt")
	f.Must(err)
	if count != 2 || out.Status != "halted" || actorRecord(t, f, id).Status != "closed" || actorReceipt(t, f, id).Kind != "halted" {
		t.Fatal("retry of closed halt answer did not finish pass")
	}
}
func TestS134Submit_OpenForeignChildNoWrite(t *testing.T) {
	f, id, deps := actorFixture(t)
	lease := leaseActor(t, f, id, deps)
	commit := actorCommit(t, f, "proof.json", "{}", "bead/"+f.Items[0])
	raw, err := f.Store.Run("create", "--title", "foreign child", "--type", "task", "--parent", f.ID(id), "--json")
	f.Must(err)
	var child kernel.Record
	f.Must(json.Unmarshal(raw, &child))
	actorRefusal(t, f, func() error {
		_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Commit: commit})
		return err
	}, child.ID)
	if _, exists := actorRecord(t, f, id).Metadata[kernel.KeyClose]; exists {
		t.Fatal("receipt written over open child")
	}
}
func TestS134Lease_UnrecordedWorkItem(t *testing.T) {
	f, _, deps := actorFixture(t)
	raw, err := f.Store.Run("create", "--title", "foreign task", "--type", "task", "--parent", f.Epic, "--json")
	f.Must(err)
	var task kernel.Record
	f.Must(json.Unmarshal(raw, &task))
	// A tuple alone cannot add an unapproved work item to the plan.
	id := task.ID + ".impl.a1"
	tuple := mustTuple(t, id)
	data, err := json.Marshal(map[string]any{kernel.KeyID: id, kernel.KeyTuple: tuple})
	f.Must(err)
	_, err = f.Store.Run("create", "--title", "forged step", "--type", "step", "--parent", task.ID, "--metadata", string(data), "--json")
	f.Must(err)
	actorRefusal(t, f, func() error { _, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, deps); return err }, task.ID)
}

func TestS134Submit_SlotReceiptCannotSubstituteRetally(t *testing.T) {
	f, id, lease, subject := slotFixture(t)
	actorCommit(t, f, "verdict.json", verdictBytes(subject, "R1"), "")
	f.Must(f.Store.WriteReceipt(id, kernel.Receipt{Kind: "passed", Retally: "forged-tally"}))
	actorRefusal(t, f, func() error {
		_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: "verdict.json"})
		return err
	}, "different result")
	if actorRecord(t, f, id).Status == "closed" {
		t.Fatal("slot accepted a gate's receipt kind without its verdict")
	}
}

func TestS134Submit_VerdictWorktreeIdentity(t *testing.T) {
	for _, otherRepo := range []bool{false, true} {
		t.Run(map[bool]string{false: "linked-worktree", true: "different-repository"}[otherRepo], func(t *testing.T) {
			f, id, lease, subject := slotFixture(t)
			gate := f.Items[0] + ".review"
			kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "panel", Round: 1, Subject: subject, Slots: []string{"R1", "R2"}})
			_, err := kernel.Pass(kernel.Rules(), f.State(), true)
			f.Must(err)
			var source *kerneltest.Fixture
			if otherRepo {
				source = kerneltest.Enrolled(t, kerneltest.WithFake(false))
			} else {
				linked := filepath.Join(t.TempDir(), "review-tree")
				actorCommand(t, f, "worktree", "add", "-qb", "review-evidence", linked, "HEAD")
				// Commands run in the linked tree but share the fixture's git objects.
				copy := *f
				copy.Root = linked
				source = &copy
			}
			path := filepath.Join(".mindspec", "specs", f.SpecID, "reviews", "panel", "R1-round-1.json")
			commit := actorCommit(t, source, path, verdictBytes(subject, "R1"), "")
			o := kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: filepath.Join(source.Root, path)}
			if otherRepo {
				actorRefusal(t, f, func() error { _, err := kernel.Submit(f.State(), o); return err }, "this repository")
			} else {
				_, err := kernel.Submit(f.State(), o)
				f.Must(err)
				receipt := actorReceipt(t, f, id)
				if receipt.Path != filepath.ToSlash(path) || receipt.Commit != commit || receipt.Kind != "passed" || actorRecord(t, f, id).Status != "closed" {
					t.Fatalf("linked evidence identity lost: %+v", receipt)
				}
			}
		})
	}
}

func TestS134Submit_PassBeforeRefusal(t *testing.T) {
	for _, verb := range []string{"submit", "answer"} {
		for _, native := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/native=%t", verb, native), func(t *testing.T) {
				f, id, deps := actorFixture(t)
				f.Store.NativeIDs = native
				lease := leaseActor(t, f, id, deps)
				commit := actorCommit(t, f, "proof.json", "{}", "bead/"+f.Items[0])
				if verb == "answer" {
					id = kerneltest.OpenDecision(f, f.Items[0]+".review", "reject", 1, "fix", "halt")
				}
				before := actorRecord(t, f, id)
				physical := before.ID
				other := f.Items[1] + ".review"
				calls := 0
				reg := kernel.NewRegistry("spy", "after")
				f.Must(reg.Register("spy", kernel.Ordinary, func(s kernel.State) (kernel.Changes, error) {
					calls++
					for key, row := range s.Records {
						if key == id || row.ID == physical {
							t.Fatal("first pass saw the named step")
						}
					}
					for _, step := range s.Steps {
						if step == id {
							t.Fatal("first pass listed the named step")
						}
					}
					if !reflect.DeepEqual(before, actorRecord(t, f, id)) {
						t.Fatal("first pass followed the verb write")
					}
					return kernel.Changes{Writes: []kernel.Write{
						{Kind: "metadata", ID: other, Metadata: kernel.Metadata("pass_seen", true)},
						{Kind: "metadata", ID: id, Metadata: kernel.Metadata("must_not_write", true)},
					}}, nil
				}))
				f.Must(reg.Register("after", kernel.FlagOnly, func(s kernel.State) (kernel.Changes, error) {
					if _, exists := s.Records[id]; exists {
						t.Fatal("later rule saw the named step")
					}
					if s.Records[other].Metadata["pass_seen"] != true {
						t.Fatal("first pass lost its overlay")
					}
					return kernel.Changes{}, nil
				}))
				kerneltest.WithRules(t, reg)
				var err error
				if verb == "submit" {
					_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: physical, Lease: lease.Token + "wrong", Commit: commit})
				} else {
					_, err = kernel.Answer(f.State(), physical, "invalid")
				}
				want := "lease token is not current"
				if verb == "answer" {
					want = "answer must be a listed decision option"
				}
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("wanted %q, got %v", want, err)
				}
				if calls != 1 {
					t.Fatalf("first pass calls=%d", calls)
				}
				if !reflect.DeepEqual(before, actorRecord(t, f, id)) {
					t.Fatal("refusal touched named step")
				}
				if actorRecord(t, f, other).Metadata["pass_seen"] != true {
					t.Fatal("refusal lost first pass write")
				}
			})
		}
	}
}
