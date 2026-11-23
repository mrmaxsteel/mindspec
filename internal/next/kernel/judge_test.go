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

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

func judgeCommit(t *testing.T, f *kerneltest.Fixture, path, content string) string {
	t.Helper()
	put(t, filepath.Join(f.Root, path), content)
	reducerGit(t, f, "add", "--", path)
	tree := reducerGit(t, f, "write-tree")
	parent := reducerGit(t, f, "rev-parse", "HEAD")
	raw := "tree " + tree + "\nparent " + parent + "\nauthor Kernel Test <kernel@example.invalid> 1760000000 +0000\ncommitter Kernel Test <kernel@example.invalid> 1760000000 +0000\n\n" + path + "\n"
	scratch := filepath.Join(t.TempDir(), "commit")
	put(t, scratch, raw)
	oid := reducerGit(t, f, "hash-object", "-t", "commit", "-w", scratch)
	reducerGit(t, f, "update-ref", "HEAD", oid)
	return oid
}
func judgeFixture(t *testing.T) (*kerneltest.Fixture, string, string) {
	t.Helper()
	f := reducerFixture(t)
	branch, err := workspace.BeadBranch(f.Items[0])
	f.Must(err)
	reducerGit(t, f, "switch", branch)
	judgeCommit(t, f, ".mindspec/config.yaml", "panel:\n  reviewers:\n    - family: harness\n      count: 3\n")
	passingPanel(t, f)
	reduce(t, f, true)
	gate := f.Items[0] + ".review"
	if r := actorReceipt(t, f, gate); r.Kind != "passed" {
		t.Fatal("fixture did not pass")
	}
	old, err := kernel.AcceptedSubject(f.State(), gate)
	f.Must(err)
	return f, gate, old
}
func judgeChange(t *testing.T, f *kerneltest.Fixture) string {
	t.Helper()
	return judgeCommit(t, f, ".mindspec/config.yaml", "panel:\n  reviewers:\n    - family: harness\n      count: 3\n  lease_ttl: 3h\n")
}
func judgeArtifact(t *testing.T, f *kerneltest.Fixture, j kernel.Judgement, slugs ...string) string {
	t.Helper()
	data, err := json.Marshal(j)
	f.Must(err)
	path := filepath.Join(t.TempDir(), "judgement.json")
	f.Must(os.WriteFile(path, data, 0600))
	return path
}
func judgeSubmit(t *testing.T, f *kerneltest.Fixture, gate string, j kernel.Judgement) kernel.Output {
	t.Helper()
	id := gate + ".judge.a1"
	lease := leaseActor(t, f, id, kernel.LeaseDeps{})
	reg, _, err := kernel.Decode[kernel.RegistrationRecord](f.State().Records[gate].Metadata, kernel.KeyRegistration)
	f.Must(err)
	artifact := judgeArtifact(t, f, j, reg.Slug)
	out, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: artifact})
	f.Must(err)
	return out
}
func judgeCount(t *testing.T, f *kerneltest.Fixture, gate string, want int) {
	t.Helper()
	n, err := kernel.RoundsForGate(f.State(), gate)
	f.Must(err)
	if n != want {
		t.Fatalf("round count = %d, want %d", n, want)
	}
}

func TestS134Judge_CreateSupersedeAndStaleSubmit(t *testing.T) {
	f, gate, old := judgeFixture(t)
	newest := judgeChange(t, f)
	before := kerneltest.Snapshot(t, f)
	out := reduce(t, f, false)
	if kerneltest.Snapshot(t, f) != before || len(out.Actions) != 1 || out.Actions[0].Exists || out.Actions[0].Step != gate+".judge.a1" || out.Actions[0].Subject.Commit != newest {
		t.Fatalf("read-only judge: %+v", out)
	}
	reduce(t, f, true)
	judgeCount(t, f, gate, 1)
	id := gate + ".judge.a1"
	if r := actorRecord(t, f, id); r.Parent != f.ID(f.Items[0]) {
		t.Fatalf("judge parent: %+v", r)
	}
	lease := leaseActor(t, f, id, kernel.LeaseDeps{})
	newest = judgeCommit(t, f, "app.go", "package app\n")
	reduce(t, f, true)
	if r := actorReceipt(t, f, id); r.Kind != "superseded" || r.Subject != newest {
		t.Fatalf("stale receipt: %+v", r)
	}
	s := f.State()
	sub, _, err := kernel.Decode[kernel.Subject](s.Records[gate+".judge.a2"].Metadata, kernel.KeySubject)
	f.Must(err)
	if len(s.Steps) != 6 || sub.Commit != newest { // implement, three slots, two judges
		t.Fatalf("judge attempts: %v subject %+v", s.Steps, sub)
	}
	artifact := judgeArtifact(t, f, kernel.Judgement{Value: "carry-over", OldSubject: old, NewSubject: newest, Reason: "same result"})
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: artifact})
	if err == nil {
		t.Fatal("stale judge submit succeeded")
	}
	judgeCount(t, f, gate, 1)
}

func TestS134Judge_CarryOverAndPanelOnlyCommit(t *testing.T) {
	f, gate, old := judgeFixture(t)
	newest := judgeChange(t, f)
	reduce(t, f, true)
	j := kernel.Judgement{Value: "carry-over", OldSubject: old, NewSubject: newest, Reason: "configuration keeps the reviewed behavior"}
	judgeSubmit(t, f, gate, j)
	s := f.State()
	accepted, err := kernel.AcceptedSubject(s, gate)
	f.Must(err)
	receipt := actorReceipt(t, f, gate)
	if accepted != newest || s.Records[gate].Status != "closed" || receipt.Kind != "passed" || len(receipt.CarryOvers) != 1 {
		t.Fatalf("carry-over: %+v accepted %s", receipt, accepted)
	}
	carry := receipt.CarryOvers[0]
	judgeReceipt := actorReceipt(t, f, gate+".judge.a1")
	if carry.OldSubject != old || carry.NewSubject != newest || carry.Judge != gate+".judge.a1" || carry.Receipt == nil || !reflect.DeepEqual(*carry.Receipt, judgeReceipt) {
		t.Fatalf("carry evidence: %+v", carry)
	}
	// The artifact commit itself is newer than the subject, but does not need judgement.
	reduce(t, f, true)
	before := kerneltest.Snapshot(t, f)
	out := reduce(t, f, true)
	if kerneltest.Snapshot(t, f) != before || len(out.Actions) != 0 || !hasRun(out, "mindspec", "complete", f.Items[0]) {
		t.Fatalf("panel-only commit created judge: %+v", out)
	}
	// All registrations of the gate, including abandoned directories, are exempt.
	kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "older", Round: 1, Subject: old, Slots: []string{"R1", "R2", "R3"}})
	f.Must(os.Rename(filepath.Join(f.SpecDir, "reviews", "older", panel.FileName), filepath.Join(t.TempDir(), "old-panel.json")))
	judgeCommit(t, f, filepath.ToSlash(filepath.Join(".mindspec", "specs", f.SpecID, "reviews", "older", "note.txt")), "review note")
	if out := reduce(t, f, true); len(out.Actions) != 0 {
		t.Fatalf("other registration artifact created judge: %+v", out)
	}
}

func TestS134Judge_RePanelValidationAndScope(t *testing.T) {
	for _, n := range []int{2, 3} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			f, gate, old := judgeFixture(t)
			newest := judgeChange(t, f)
			reduce(t, f, true)
			out := judgeSubmit(t, f, gate, kernel.Judgement{Value: "re-panel", OldSubject: old, NewSubject: newest, SlotCount: n, Paths: []string{".mindspec/config.yaml"}})
			successor := actorRecord(t, f, gate+".a2")
			from, _, err := kernel.Decode[kernel.From](successor.Metadata, kernel.KeyFrom)
			f.Must(err)
			scope, _, err := kernel.Decode[kernel.RePanel](successor.Metadata, kernel.KeyRePanel)
			f.Must(err)
			round, _, err := kernel.Decode[kernel.GateRound](successor.Metadata, kernel.KeyRound)
			f.Must(err)
			if string(from) != gate || scope.SlotCount != n || round.SlotCount != n || round.Round != 2 || round.Subject != newest || !reflect.DeepEqual(round.Paths, scope.Paths) || len(round.Slots) != n {
				t.Fatalf("successor: %+v", successor)
			}
			found := false
			for _, entry := range out.Run {
				found = found || (reflect.DeepEqual(entry.Argv, []string{"mindspec", "panel", "create", f.Items[0]}) && reflect.DeepEqual(entry.Paths, scope.Paths))
			}
			if !found {
				t.Fatalf("missing scoped panel command: %+v", out)
			}
			judgeCount(t, f, gate, 1)
			before := kerneltest.Snapshot(t, f)
			reduce(t, f, true)
			if kerneltest.Snapshot(t, f) != before {
				t.Fatal("replayed re-panel changed snapshot")
			}
		})
	}
	for _, bad := range []string{"absolute-path", "parent-path", "unclean-path", "carry-slots", "carry-paths", "one-slot", "four-slots", "unknown-value", "blank-reason", "no-paths", "wrong-old", "wrong-new", "unknown-field", "trailing-json", "bad-receipt"} {
		t.Run(bad, func(t *testing.T) {
			f, gate, old := judgeFixture(t)
			middle := judgeCommit(t, f, "middle.go", "package app\n")
			newest := judgeChange(t, f)
			reduce(t, f, true)
			id := gate + ".judge.a1"
			lease := leaseActor(t, f, id, kernel.LeaseDeps{})
			j := kernel.Judgement{Value: "re-panel", OldSubject: old, NewSubject: newest, SlotCount: 2, Paths: []string{".mindspec/config.yaml"}}
			switch bad {
			case "absolute-path":
				j.Paths = []string{"/app.go"}
			case "parent-path":
				j.Paths = []string{"../app.go"}
			case "unclean-path":
				j.Paths = []string{"src/../app.go"}
			case "carry-slots":
				j = kernel.Judgement{Value: "carry-over", OldSubject: old, NewSubject: newest, Reason: "same", SlotCount: 2}
			case "carry-paths":
				j = kernel.Judgement{Value: "carry-over", OldSubject: old, NewSubject: newest, Reason: "same", Paths: []string{"app.go"}}
			case "one-slot":
				j.SlotCount = 1
			case "four-slots":
				j.SlotCount = 4
			case "unknown-value":
				j.Value = "approve"
			case "blank-reason":
				j = kernel.Judgement{Value: "carry-over", OldSubject: old, NewSubject: newest, Reason: " "}
			case "no-paths":
				j.Paths = nil
			case "wrong-old":
				j.OldSubject = middle
			case "wrong-new":
				j.NewSubject = middle
			}
			artifact := judgeArtifact(t, f, j)
			if bad == "unknown-field" || bad == "trailing-json" {
				data, _ := json.Marshal(j)
				if bad == "unknown-field" {
					data = append(data[:len(data)-1], []byte(`,"extra":true}`)...)
				} else {
					data = append(data, []byte(` {}`)...)
				}
				f.Must(os.WriteFile(artifact, data, 0600))
			}
			if bad == "bad-receipt" {
				f.Must(f.Store.WriteReceipt(gate, kernel.Receipt{Kind: "passed"}))
			}
			before := kerneltest.Snapshot(t, f)
			gateBefore := actorRecord(t, f, gate)
			_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: artifact})
			assertSubmitRefusal(t, err, "", "mindspec next-action")
			if err == nil || actorRecord(t, f, id).Status != "in_progress" {
				t.Fatalf("invalid judgement accepted: %v", err)
			}
			if bad == "wrong-old" || bad == "wrong-new" {
				if !strings.Contains(err.Error(), "expected old subject "+old+" and new subject "+newest+"; slot ceiling 3") {
					t.Fatalf("stale subject context: %v", err)
				}
			}
			after := kerneltest.Snapshot(t, f)
			if bad == "bad-receipt" {
				// The first pass repairs the invalid gate close even when submit refuses.
				retry := actorRecord(t, f, gate+".a2")
				want := gateBefore
				want.ID, want.Status = retry.ID, "open"
				delete(want.Metadata, kernel.KeyClose)
				want.Metadata[kernel.KeyID] = gate + ".a2"
				want.Metadata[kernel.KeyFrom] = gate
				tuple, _, err := gateBefore.Tuple()
				f.Must(err)
				tuple.Attempt = 2
				want.Metadata[kernel.KeyTuple] = tuple
				wantData, err := json.Marshal(want)
				f.Must(err)
				retryData, err := json.Marshal(retry)
				f.Must(err)
				if !jsonEqual(want, retry) {
					t.Fatalf("invalid close retry: %s, want %s", retryData, wantData)
				}
				after = judgeSnapshotWithoutRetry(t, after, retry.ID)
			} else if r, _ := f.Store.Lookup(gate + ".a2"); r != nil {
				t.Fatal("invalid judgement made successor")
			}
			if before != after {
				t.Fatal("invalid judgement changed state beyond the gate close retry")
			}
			judgeCount(t, f, gate, 1)
		})
	}
}

// judgeSnapshotWithoutRetry removes only the checked retry row and its empty link list.
func judgeSnapshotWithoutRetry(t *testing.T, snapshot, id string) string {
	t.Helper()
	lines := strings.Split(snapshot, "\n")
	var rows []json.RawMessage
	if err := json.Unmarshal([]byte(lines[0]), &rows); err != nil {
		t.Fatal(err)
	}
	for i, data := range rows {
		var row kernel.Record
		if err := json.Unmarshal(data, &row); err != nil {
			t.Fatal(err)
		}
		if row.ID != id {
			continue
		}
		if lines[i+1] != "null" {
			t.Fatal("gate close retry has links")
		}
		rows = append(rows[:i], rows[i+1:]...)
		data, err := json.Marshal(rows)
		if err != nil {
			t.Fatal(err)
		}
		lines[0] = string(data)
		lines = append(lines[:i+1], lines[i+2:]...)
		return strings.Join(lines, "\n")
	}
	t.Fatal("gate close retry missing from snapshot")
	return ""
}

func judgePanelRead(t *testing.T, f *kerneltest.Fixture, dir string) *panel.Panel {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, panel.FileName))
	f.Must(err)
	var p panel.Panel
	f.Must(json.Unmarshal(data, &p))
	return &p
}
func judgeRebind(t *testing.T, f *kerneltest.Fixture, gate, old, newest string) {
	t.Helper()
	dir := filepath.Join(f.SpecDir, "reviews", "panel")
	p := judgePanelRead(t, f, dir)
	entry := panel.Rebind{Round: 1, FromSHA: old, ToSHA: newest, Paths: []string{"app.go"}, Reason: "reviewer accepts this change", CreatedAt: fixedTime.Format("2006-01-02T15:04:05Z07:00")}
	event, err := json.Marshal(panel.RebindRecord{Record: panel.RecordRebind, Spec: f.SpecID, Panel: "panel", SubjectDigest: "rebound-digest", Rebind: entry})
	f.Must(err)
	f.Must(panel.AppendRecords(dir, event))
	p.ReviewedHeadSHA = newest
	p.SubjectDigest = "rebound-digest"
	p.Rebinds = append(p.Rebinds, entry)
	f.Must(panel.WriteSnapshot(dir, *p))
}
func TestS134Judge_RebindSupersedesAndPrecedence(t *testing.T) {
	f, gate, old := judgeFixture(t)
	newest := judgeCommit(t, f, "app.go", "package app\n")
	reduce(t, f, true)
	judgeRebind(t, f, gate, old, newest)
	reduce(t, f, true)
	receipt := actorReceipt(t, f, gate+".judge.a1")
	accepted, err := kernel.AcceptedSubject(f.State(), gate)
	f.Must(err)
	if receipt.Kind != "superseded" || !strings.Contains(receipt.Rebind, "#rebind:"+newest) || accepted != newest {
		t.Fatalf("rebind ignored: %+v accepted %s", receipt, accepted)
	}
	if r, _ := f.Store.Lookup(gate + ".judge.a2"); r != nil {
		t.Fatal("rebind created second judge")
	}
	// An unbacked snapshot edit cannot accept a subject.
	next := judgeCommit(t, f, "next.go", "package app\n")
	dir := filepath.Join(f.SpecDir, "reviews", "panel")
	p := judgePanelRead(t, f, dir)
	p.ReviewedHeadSHA = next
	p.Rebinds = append(p.Rebinds, panel.Rebind{Round: 1, FromSHA: newest, ToSHA: next})
	f.Must(panel.WriteSnapshot(dir, *p))
	accepted, err = kernel.AcceptedSubject(f.State(), gate)
	f.Must(err)
	if accepted != newest {
		t.Fatal("unbacked rebind accepted")
	}
}

func TestS134Judge_FinalGateAndEndStates(t *testing.T) {
	t.Run("final-carry", func(t *testing.T) {
		f, _, _ := judgeFixture(t)
		branch, err := workspace.SpecBranch(f.SpecID)
		f.Must(err)
		reducerGit(t, f, "switch", branch)
		old := judgeCommit(t, f, "final.go", "package final\n")
		gate := f.Epic + ".final"
		kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "final-panel", Round: 1, Subject: old, Slots: []string{"R1", "R2", "R3"}})
		// Release the final gate's plan blockers with real merge evidence.
		for _, item := range f.Items {
			kerneltest.PassGate(f, item+".review", old)
			f.Must(f.Store.SetStatusIf(item, "open", "closed"))
			branch, err := workspace.BeadBranch(item)
			f.Must(err)
			reducerGit(t, f, "update-ref", "refs/heads/"+branch, old)
		}
		kerneltest.PassGate(f, gate, old)
		newest := judgeCommit(t, f, "final.go", "package final\n// final export\n")
		reduce(t, f, true)
		id := gate + ".judge.a1"
		if r := actorRecord(t, f, id); r.Parent != f.Epic {
			t.Fatalf("final judge parent %+v", r)
		}
		judgeSubmit(t, f, gate, kernel.Judgement{Value: "carry-over", OldSubject: old, NewSubject: newest, Reason: "export preserves the reviewed result"})
		accepted, err := kernel.AcceptedSubject(f.State(), gate)
		f.Must(err)
		if accepted != newest || actorReceipt(t, f, gate).Kind != "passed" {
			t.Fatal("final carry-over did not preserve pass")
		}
	})
	t.Run("merged-item", func(t *testing.T) {
		f, gate, _ := judgeFixture(t)
		branch, err := workspace.SpecBranch(f.SpecID)
		f.Must(err)
		reducerGit(t, f, "switch", branch)
		beadBranch, err := workspace.BeadBranch(f.Items[0])
		f.Must(err)
		reducerGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "merge", "--no-ff", beadBranch, "-m", "merge reviewed work")
		reducerGit(t, f, "switch", beadBranch)
		judgeChange(t, f)
		reduce(t, f, true)
		if r, _ := f.Store.Lookup(gate + ".judge.a1"); r != nil {
			t.Fatal("merged work created judge")
		}
	})
	t.Run("closed-epic", func(t *testing.T) {
		f, _, _ := judgeFixture(t)
		branch, err := workspace.SpecBranch(f.SpecID)
		f.Must(err)
		reducerGit(t, f, "switch", branch)
		old := judgeCommit(t, f, "final.go", "package final\n")
		gate := f.Epic + ".final"
		for _, item := range f.Items {
			kerneltest.PassGate(f, item+".review", old)
			f.Must(f.Store.SetStatusIf(item, "open", "closed"))
		}
		kerneltest.PassGate(f, gate, old)
		f.Must(f.Store.SetStatusIf(f.Epic, "open", "closed"))
		judgeCommit(t, f, "final.go", "package final\n// later\n")
		reduce(t, f, true)
		if r, _ := f.Store.Lookup(gate + ".judge.a1"); r != nil {
			t.Fatal("closed epic created final judge")
		}
	})
}

func TestS134SeedMatch_Judge(t *testing.T) {
	real, gate, _ := judgeFixture(t)
	seed, _, _ := judgeFixture(t)
	newest := judgeChange(t, real)
	if other := judgeChange(t, seed); other != newest {
		t.Fatal("twin commit mismatch")
	}
	reduce(t, real, true)
	id := kerneltest.Judge(seed, gate, newest, 1)
	if !jsonEqual(actorRecord(t, real, id), actorRecord(t, seed, id)) {
		t.Fatal("Judge seeder differs from real pass")
	}
}
func TestS134SeedMatch_AuthoriseRepanel(t *testing.T) {
	real, gate, old := judgeFixture(t)
	seed, _, _ := judgeFixture(t)
	newest := judgeChange(t, real)
	judgeChange(t, seed)
	reduce(t, real, true)
	j := kernel.Judgement{Value: "re-panel", OldSubject: old, NewSubject: newest, SlotCount: 2, Paths: []string{".mindspec/config.yaml"}}
	judgeSubmit(t, real, gate, j)
	successor := gate + ".a2"
	kerneltest.Judge(seed, gate, newest, 1)
	seed.Create(successor, seed.Items[0])
	seed.Must(seed.Store.WriteMetadata(successor, map[string]any{kernel.KeyFrom: kernel.From(gate), kernel.KeyRePanel: kernel.RePanel{Judge: gate + ".judge.a1", SlotCount: 2, Paths: j.Paths}}))
	count, err := kernel.RoundsForGate(seed.State(), gate)
	seed.Must(err)
	round := kernel.GateRound{Round: count + 1, Subject: newest, SlotCount: 2, Paths: j.Paths, Slots: []string{fmt.Sprintf("%s.r%d.R1", gate, count+1), fmt.Sprintf("%s.r%d.R2", gate, count+1)}}
	kerneltest.Authorise(seed, successor, round)
	if !jsonEqual(actorRecord(t, real, successor), actorRecord(t, seed, successor)) {
		t.Fatalf("Authorise differs: real %+v seed %+v", actorRecord(t, real, successor), actorRecord(t, seed, successor))
	}
}

func TestS134Judge_SubmitPreservesForeignRecords(t *testing.T) {
	f, gate, old := judgeFixture(t)
	newest := judgeChange(t, f)
	reduce(t, f, true)
	for _, r := range []kernel.Record{{ID: "foreign-1", Parent: f.Epic, Type: "task", Status: "open", Metadata: map[string]any{"note": "keep"}}, {ID: "foreign-2", Type: "bug", Status: "open"}} {
		f.Fake.Records[r.ID] = r
	}
	row := f.Fake.Records[f.Items[1]]
	row.Dependencies = append(row.Dependencies, kernel.Dependency{ID: "foreign-2", Type: "blocks"})
	row.Status = "in_progress"
	f.Fake.Records[row.ID] = row
	lease := leaseActor(t, f, gate+".judge.a1", kernel.LeaseDeps{})
	artifact := judgeArtifact(t, f, kernel.Judgement{Value: "carry-over", OldSubject: old, NewSubject: newest, Reason: "review remains valid"})
	before := f.State()
	refsBefore := reducerGit(t, f, "for-each-ref")
	statusBefore := reducerGit(t, f, "status", "--porcelain")
	foreignBefore := kerneltest.Snapshot(t, f)
	_, err := kernel.Submit(before, kernel.SubmitOptions{Step: gate + ".judge.a1", Lease: lease.Token, Record: artifact})
	f.Must(err)
	after := f.State()
	if !reflect.DeepEqual(before.Refs, after.Refs) || refsBefore != reducerGit(t, f, "for-each-ref") || statusBefore != reducerGit(t, f, "status", "--porcelain") {
		t.Fatal("submit changed refs or repository status")
	}
	if foreignBefore == kerneltest.Snapshot(t, f) {
		t.Fatal("submit did not exercise a write")
	}
	for id, r := range before.Records {
		next := after.Records[id]
		if !jsonEqual(r, next) && id != gate && id != gate+".judge.a1" {
			t.Fatalf("submit changed unrelated record %s", id)
		}
	}
	for _, id := range []string{"foreign-1", "foreign-2"} {
		if !jsonEqual(before.Graph[id], after.Graph[id]) {
			t.Fatalf("foreign record changed %s", id)
		}
	}
	if len(after.Records) != len(before.Records) {
		t.Fatal("submit created unrelated records")
	}
}

func TestS134Judge_AcceptanceFollowsPassOrder(t *testing.T) {
	f, gate, old := judgeFixture(t)
	rebound := judgeCommit(t, f, "app.go", "package app\n")
	judgeRebind(t, f, gate, old, rebound)
	reduce(t, f, true)
	intermediate := judgeCommit(t, f, "intermediate.go", "package app\n")
	newest := judgeChange(t, f)
	reduce(t, f, true)
	// No earlier change required a judge, so this is the first judge attempt.
	judgeSubmit(t, f, gate, kernel.Judgement{Value: "carry-over", OldSubject: rebound, NewSubject: newest, Reason: "review remains valid"})
	accepted, err := kernel.AcceptedSubject(f.State(), gate)
	f.Must(err)
	if accepted != newest {
		t.Fatal("older rebind defeated a later carry-over")
	}
	carry := actorReceipt(t, f, gate).CarryOvers[0]
	if !strings.Contains(carry.Rebind, "#rebind:"+rebound) {
		t.Fatalf("missing rebind checkpoint: %+v", carry)
	}
	// A later rebind can accept a commit before the last carry-over's commit.
	// This makes commit ancestry an invalid substitute for pass order.
	dir := filepath.Join(f.SpecDir, "reviews", "panel")
	p := judgePanelRead(t, f, dir)
	entry := panel.Rebind{Round: 1, FromSHA: rebound, ToSHA: intermediate, Paths: []string{"app.go"}, Reason: "later acceptance", CreatedAt: fixedTime.Add(1).Format("2006-01-02T15:04:05.999999999Z07:00")}
	// The live branch can move back while remaining past the panel's subject.
	actorCommand(t, f, "update-ref", "HEAD", intermediate)
	data, err := json.Marshal(panel.RebindRecord{Record: panel.RecordRebind, Spec: f.SpecID, Panel: "panel", SubjectDigest: "later-digest", Rebind: entry})
	f.Must(err)
	f.Must(panel.AppendRecords(dir, data))
	p.Rebinds = append(p.Rebinds, entry)
	p.ReviewedHeadSHA = intermediate
	p.SubjectDigest = "later-digest"
	f.Must(panel.WriteSnapshot(dir, *p))
	accepted, err = kernel.AcceptedSubject(f.State(), gate)
	f.Must(err)
	if accepted != intermediate {
		t.Fatal("later rebind lost to the older carry-over")
	}
}

func TestS134Judge_ResumeReceiptedSubmit(t *testing.T) {
	for _, value := range []string{"carry-over", "re-panel"} {
		t.Run(value, func(t *testing.T) {
			f, gate, old := judgeFixture(t)
			newest := judgeChange(t, f)
			reduce(t, f, true)
			id := gate + ".judge.a1"
			lease := leaseActor(t, f, id, kernel.LeaseDeps{})
			j := kernel.Judgement{Value: value, OldSubject: old, NewSubject: newest, Reason: "review result"}
			if value == "re-panel" {
				j.SlotCount = 2
				j.Paths = []string{".mindspec/config.yaml"}
			}
			artifact := judgeArtifact(t, f, j)
			restore := kernel.ReplaceRulesForTest(kernel.NewRegistry())
			_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: artifact})
			restore()
			f.Must(err)
			if actorRecord(t, f, id).Status != "closed" {
				t.Fatal("missing crash-state actor close")
			}
			if _, ok, _ := kernel.Decode[kernel.Judgement](actorRecord(t, f, id).Metadata, kernel.KeyJudgement); ok {
				t.Fatal("fixture already applied judgement")
			}
			reduce(t, f, true)
			if value == "carry-over" {
				accepted, err := kernel.AcceptedSubject(f.State(), gate)
				f.Must(err)
				if accepted != newest {
					t.Fatal("receipt did not resume carry-over")
				}
			} else {
				if r, _ := f.Store.Lookup(gate + ".a2"); r == nil {
					t.Fatal("receipt did not resume re-panel")
				}
			}
			before := kerneltest.Snapshot(t, f)
			reduce(t, f, true)
			if kerneltest.Snapshot(t, f) != before {
				t.Fatal("resumed judgement is not idempotent")
			}
		})
	}
}

func TestS134Judge_HighestGateAndSeparateAttempts(t *testing.T) {
	f, gate, old := judgeFixture(t)
	newest := judgeChange(t, f)
	reduce(t, f, true)
	judgeSubmit(t, f, gate, kernel.Judgement{Value: "re-panel", OldSubject: old, NewSubject: newest, SlotCount: 2, Paths: []string{".mindspec/config.yaml"}})
	successor := gate + ".a2"
	kerneltest.Register(f, successor, kernel.RegistrationRecord{Slug: "panel", Round: 2, Subject: newest, Slots: []string{"R1", "R2"}})
	kerneltest.PassGate(f, successor, newest)
	accepted, err := kernel.AcceptedSubject(f.State(), gate)
	f.Must(err)
	if accepted != newest {
		t.Fatal("accepted subject ignored highest gate attempt")
	}
	latest := judgeCommit(t, f, ".mindspec/config.yaml", "panel:\n  reviewers:\n    - family: harness\n      count: 3\n  lease_ttl: 4h\n")
	reduce(t, f, true)
	id := gate + ".judge.a2"
	row := actorRecord(t, f, id)
	sub, _, err := kernel.Decode[kernel.Subject](row.Metadata, kernel.KeySubject)
	f.Must(err)
	if sub.Commit != latest || row.Parent != f.ID(f.Items[0]) {
		t.Fatalf("wrong judge attempt: %+v", row)
	}
	if r, _ := f.Store.Lookup(gate + ".judge.a3"); r != nil {
		t.Fatal("gate attempt inflated judge attempt")
	}
	lease := leaseActor(t, f, id, kernel.LeaseDeps{})
	artifact := judgeArtifact(t, f, kernel.Judgement{Value: "re-panel", OldSubject: newest, NewSubject: latest, SlotCount: 3, Paths: []string{".mindspec/config.yaml"}})
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: artifact})
	if err == nil {
		t.Fatal("successor passing-round ceiling ignored")
	}
}

func TestS134Judge_SecondRePanel(t *testing.T) {
	f, gate, old := judgeFixture(t)
	newest := judgeChange(t, f)
	reduce(t, f, true)
	judgeSubmit(t, f, gate, kernel.Judgement{Value: "re-panel", OldSubject: old, NewSubject: newest, SlotCount: 2, Paths: []string{".mindspec/config.yaml"}})
	successor := gate + ".a2"
	kerneltest.Register(f, successor, kernel.RegistrationRecord{Slug: "panel", Round: 2, Subject: newest, Slots: []string{"R1", "R2"}})
	kerneltest.PassGate(f, successor, newest)
	latest := judgeCommit(t, f, ".mindspec/config.yaml", "panel:\n  reviewers:\n    - family: harness\n      count: 3\n  lease_ttl: 4h\n")
	reduce(t, f, true)
	id := gate + ".judge.a2"
	lease := leaseActor(t, f, id, kernel.LeaseDeps{})
	artifact := judgeArtifact(t, f, kernel.Judgement{Value: "re-panel", OldSubject: newest, NewSubject: latest, SlotCount: 2, Paths: []string{".mindspec/config.yaml"}})
	if _, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: artifact}); err != nil {
		t.Fatalf("second re-panel refused: %v", err)
	}
	reduce(t, f, true)
	if r, _ := f.Store.Lookup(gate + ".a3"); r == nil {
		t.Fatal("second re-panel created no successor a3")
	}
	scope, ok, err := kernel.Decode[kernel.RePanel](actorRecord(t, f, gate+".a3").Metadata, kernel.KeyRePanel)
	f.Must(err)
	if !ok || scope.Judge != id {
		t.Fatalf("second re-panel record names the wrong judge: %+v", scope)
	}
}

func TestS134Judge_ResumeReceiptBeforeNewCommit(t *testing.T) {
	f, gate, old := judgeFixture(t)
	newest := judgeChange(t, f)
	reduce(t, f, true)
	id := gate + ".judge.a1"
	lease := leaseActor(t, f, id, kernel.LeaseDeps{})
	artifact := judgeArtifact(t, f, kernel.Judgement{Value: "carry-over", OldSubject: old, NewSubject: newest, Reason: "review result"})
	restore := kernel.ReplaceRulesForTest(kernel.NewRegistry())
	_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: artifact})
	restore()
	f.Must(err)
	later := judgeCommit(t, f, "later.go", "package app\n")
	reduce(t, f, true)
	accepted, err := kernel.AcceptedSubject(f.State(), gate)
	f.Must(err)
	next := actorRecord(t, f, gate+".judge.a2")
	sub, _, err := kernel.Decode[kernel.Subject](next.Metadata, kernel.KeySubject)
	f.Must(err)
	if accepted != newest || sub.Commit != later {
		t.Fatal("receipt did not apply before the next judge")
	}
}

func TestS134Judge_ParentFromTuple(t *testing.T) {
	f, gate, _ := judgeFixture(t)
	// Tracker reads can expose the parent only as a dependency.
	row := f.Fake.Records[f.ID(gate)]
	row.Parent = ""
	row.Dependencies = append(row.Dependencies, kernel.Dependency{ID: f.Items[0], Type: "parent-child"})
	f.Fake.Records[row.ID] = row
	judgeChange(t, f)
	reduce(t, f, true)
	if r := actorRecord(t, f, gate+".judge.a1"); r.Parent != f.ID(f.Items[0]) {
		t.Fatalf("judge lacks computed parent: %+v", r)
	}
}

func TestS134Judge_OneLiveJudge(t *testing.T) {
	f, gate, _ := judgeFixture(t)
	judgeChange(t, f)
	for i := 0; i < 3; i++ {
		reduce(t, f, true)
	}
	judges := []string{}
	s := f.State()
	for _, id := range s.Steps {
		tuple, _, err := s.Records[id].Tuple()
		f.Must(err)
		if tuple.Kind == "change-judge" {
			judges = append(judges, id)
		}
	}
	if !reflect.DeepEqual(judges, []string{gate + ".judge.a1"}) {
		t.Fatalf("judge ids: %v", judges)
	}
}

func TestS134Judge_RePanelSubjectBound(t *testing.T) {
	f, gate, old := judgeFixture(t)
	newest := judgeChange(t, f)
	reduce(t, f, true)
	judgeSubmit(t, f, gate, kernel.Judgement{Value: "re-panel", OldSubject: old, NewSubject: newest, SlotCount: 2, Paths: []string{".mindspec/config.yaml"}})
	successor := gate + ".a2"
	scope, ok, err := kernel.Decode[kernel.RePanel](actorRecord(t, f, successor).Metadata, kernel.KeyRePanel)
	f.Must(err)
	if !ok || scope.Judge != gate+".judge.a1" || scope.SlotCount != 2 || !reflect.DeepEqual(scope.Paths, []string{".mindspec/config.yaml"}) {
		t.Fatalf("missing judge authorisation: %+v", scope)
	}
	// The committed judgement is panel-only and must retain the reduced round.
	reduce(t, f, true)
	round, _, err := kernel.Decode[kernel.GateRound](actorRecord(t, f, successor).Metadata, kernel.KeyRound)
	f.Must(err)
	if round.Subject != newest || round.SlotCount != 2 || !reflect.DeepEqual(round.Paths, scope.Paths) {
		t.Fatalf("panel-only commit changed the authorised round: %+v", round)
	}
	judgeCommit(t, f, "evil.go", "package evil\n")
	reduce(t, f, true)
	round, _, err = kernel.Decode[kernel.GateRound](actorRecord(t, f, successor).Metadata, kernel.KeyRound)
	f.Must(err)
	if round.Subject != newest || round.Round != 2 || round.SlotCount != 2 || len(round.Slots) != 2 || !reflect.DeepEqual(round.Paths, scope.Paths) {
		t.Fatalf("authorised scope was reset without a judgement: %+v", round)
	}
	saved, ok, err := kernel.Decode[kernel.RePanel](actorRecord(t, f, successor).Metadata, kernel.KeyRePanel)
	f.Must(err)
	if !ok || !reflect.DeepEqual(saved, scope) {
		t.Fatalf("full round changed the judge authorisation: got %+v, want %+v", saved, scope)
	}
	before := kerneltest.Snapshot(t, f)
	reduce(t, f, true)
	if kerneltest.Snapshot(t, f) != before {
		t.Fatal("subject recovery did not converge")
	}
}

func TestS134Judge_RePanelCrashRecovery(t *testing.T) {
	f, gate, old := judgeFixture(t)
	newest := judgeChange(t, f)
	reduce(t, f, true)
	id := gate + ".judge.a1"
	lease := leaseActor(t, f, id, kernel.LeaseDeps{})
	artifact := judgeArtifact(t, f, kernel.Judgement{Value: "re-panel", OldSubject: old, NewSubject: newest, SlotCount: 2, Paths: []string{".mindspec/config.yaml"}})
	crash := errors.New("crash before applied marker")
	f.Fake.Hook = func(args []string) error {
		if args[0] == "update" {
			for i, arg := range args {
				if arg == "--metadata" {
					raw, e := os.ReadFile(strings.TrimPrefix(args[i+1], "@"))
					f.Must(e)
					var metadata map[string]any
					f.Must(json.Unmarshal(raw, &metadata))
					if _, has := metadata[kernel.KeyJudgement]; has {
						return crash
					}
				}
			}
		}
		return nil
	}
	_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: artifact})
	if !errors.Is(err, crash) {
		t.Fatalf("crash point not reached: %v", err)
	}
	f.Fake.Hook = nil
	if _, ok, err := kernel.Decode[kernel.Judgement](actorRecord(t, f, id).Metadata, kernel.KeyJudgement); err != nil || ok {
		t.Fatalf("marker survived: %v %v", ok, err)
	}
	actorRecord(t, f, gate+".a2")
	judgeCommit(t, f, "evil.go", "package evil\n")
	for i := 0; i < 4; i++ {
		reduce(t, f, true)
	}
	before := kerneltest.Snapshot(t, f)
	reduce(t, f, true)
	if kerneltest.Snapshot(t, f) != before {
		t.Fatal("crash recovery did not converge")
	}
}

func TestS134Judge_WorktreeLayouts(t *testing.T) {
	for _, layout := range []string{"nested", "external"} {
		t.Run(layout, func(t *testing.T) {
			f, _, _ := judgeFixture(t)
			branch, err := workspace.SpecBranch(f.SpecID)
			f.Must(err)
			reducerGit(t, f, "switch", branch)
			old := judgeCommit(t, f, "final.go", "package final\n")
			gate := f.Epic + ".final"
			kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "final-panel", Round: 1, Subject: old, Slots: []string{"R1", "R2", "R3"}})
			kerneltest.Authorise(f, gate, kernel.GateRound{Round: 1, Subject: old, SlotCount: 3})
			for _, item := range f.Items {
				kerneltest.PassGate(f, item+".review", old)
				f.Must(f.Store.SetStatusIf(item, "open", "closed"))
				beadBranch, err := workspace.BeadBranch(item)
				f.Must(err)
				reducerGit(t, f, "update-ref", "refs/heads/"+beadBranch, old)
			}
			kerneltest.PassGate(f, gate, old)
			newest := judgeCommit(t, f, "final.go", "package final\n// changed\n")
			reducerGit(t, f, "switch", "--detach", old)
			wt := filepath.Join(f.Root, ".worktrees", "worktree-spec-"+f.SpecID)
			if layout == "external" {
				wt = filepath.Join(t.TempDir(), "spec")
			}
			reducerGit(t, f, "worktree", "add", wt, branch)
			specDir := filepath.Join(wt, ".mindspec", "specs", f.SpecID)
			f.Must(os.MkdirAll(filepath.Dir(specDir), 0755))
			f.Must(os.Rename(f.SpecDir, specDir))
			// Fixture commands use the worktree. The store still uses the main checkout.
			f.Root, f.SpecDir = wt, specDir
			reduce(t, f, true)
			judgeSubmit(t, f, gate, kernel.Judgement{Value: "carry-over", OldSubject: old, NewSubject: newest, Reason: "same result"})
			accepted, err := kernel.AcceptedSubject(f.State(), gate)
			f.Must(err)
			if accepted != newest {
				t.Fatalf("accepted %s, want %s", accepted, newest)
			}
			judgeCommit(t, f, filepath.Join(".mindspec", "specs", f.SpecID, "reviews", "final-panel", "note.txt"), "panel note")
			reduce(t, f, true)
			if r, _ := f.Store.Lookup(gate + ".judge.a2"); r != nil {
				t.Fatal("panel-only commit moved subject")
			}
		})
	}
}

func TestS134Judge_OutsidePanelLocation(t *testing.T) {
	f, gate, old := judgeFixture(t)
	newest := judgeChange(t, f)
	reduce(t, f, true)
	id := gate + ".judge.a1"
	lease := leaseActor(t, f, id, kernel.LeaseDeps{})
	// Keep the artifact commit off the subject branch to isolate the location check.
	wt := filepath.Join(t.TempDir(), "artifact")
	reducerGit(t, f, "worktree", "add", "--detach", wt, newest)
	mainRoot := f.Root
	f.Root = wt
	data, err := json.Marshal(kernel.Judgement{Value: "carry-over", OldSubject: old, NewSubject: newest, Reason: "same result"})
	f.Must(err)
	judgeCommit(t, f, "other.json", string(data))
	f.Root = mainRoot
	before := kerneltest.Snapshot(t, f)
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: filepath.Join(wt, "other.json")})
	if err == nil || !strings.Contains(err.Error(), "outside every worktree") {
		t.Fatalf("location refusal: %v", err)
	}
	if before != kerneltest.Snapshot(t, f) {
		t.Fatal("location refusal wrote state")
	}
}

func TestS134Judge_PassingRoundCeiling(t *testing.T) {
	for _, variant := range []string{"other-round", "edited-snapshot", "absent-snapshot"} {
		t.Run(variant, func(t *testing.T) {
			f, gate, old := judgeFixture(t)
			reg, _, err := kernel.Decode[kernel.RegistrationRecord](actorRecord(t, f, gate).Metadata, kernel.KeyRegistration)
			f.Must(err)
			switch variant {
			case "other-round":
				kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "earlier", Round: 7, Subject: old, Slots: []string{"R1", "R2", "R3", "R4", "R5"}})
				f.Must(f.Store.WriteMetadata(gate, kernel.Metadata(kernel.KeyRegistration, reg)))
			case "edited-snapshot":
				dir := filepath.Join(f.SpecDir, "reviews", reg.Slug)
				snapshot := judgePanelRead(t, f, dir)
				snapshot.ExpectedReviewers = 5
				f.Must(panel.WriteSnapshot(dir, *snapshot))
			case "absent-snapshot":
				f.Must(os.Rename(filepath.Join(f.SpecDir, "reviews", reg.Slug, panel.FileName), filepath.Join(t.TempDir(), "snapshot.json")))
			}
			newest := judgeChange(t, f)
			reduce(t, f, true)
			id := gate + ".judge.a1"
			lease := leaseActor(t, f, id, kernel.LeaseDeps{})
			artifact := judgeArtifact(t, f, kernel.Judgement{Value: "re-panel", OldSubject: old, NewSubject: newest, SlotCount: 4, Paths: []string{"app.go"}}, reg.Slug)
			_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: artifact})
			if err == nil || !strings.Contains(err.Error(), "2 to 3 slots") {
				t.Fatalf("passing-round ceiling refusal: %v", err)
			}
			// The gate's frozen roster also allows valid scope without a snapshot.
			artifact = judgeArtifact(t, f, kernel.Judgement{Value: "re-panel", OldSubject: old, NewSubject: newest, SlotCount: 3, Paths: []string{"app.go"}}, reg.Slug)
			_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: artifact})
			f.Must(err)
		})
	}
}

func TestS134Judge_LegacyPanelDirectory(t *testing.T) {
	f, gate, old := judgeFixture(t)
	newest := judgeChange(t, f)
	dir := filepath.Join(f.Root, "review", "panel")
	f.Must(os.MkdirAll(filepath.Dir(dir), 0755))
	f.Must(os.Rename(filepath.Join(f.SpecDir, "reviews", "panel"), dir))
	reduce(t, f, true)
	id := gate + ".judge.a1"
	lease := leaseActor(t, f, id, kernel.LeaseDeps{})
	data, err := json.Marshal(kernel.Judgement{Value: "carry-over", OldSubject: old, NewSubject: newest, Reason: "same result"})
	f.Must(err)
	// The store must record a relocated registration before it can be exempt.
	reg, _, err := kernel.Decode[kernel.RegistrationRecord](f.State().Records[gate].Metadata, kernel.KeyRegistration)
	f.Must(err)
	reg.Directories = []string{"review/panel"}
	f.Must(f.Store.WriteMetadata(gate, kernel.Metadata(kernel.KeyRegistration, reg)))
	reducerGit(t, f, "add", "--", filepath.Join("review", "panel", "panel.json"), filepath.Join("review", "panel", "dispositions.jsonl"))
	path := filepath.Join(t.TempDir(), "judgement.json")
	f.Must(os.WriteFile(path, data, 0600))
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: path})
	f.Must(err)
	accepted, err := kernel.AcceptedSubject(f.State(), gate)
	f.Must(err)
	if accepted != newest {
		t.Fatalf("accepted %s, want %s", accepted, newest)
	}
}
