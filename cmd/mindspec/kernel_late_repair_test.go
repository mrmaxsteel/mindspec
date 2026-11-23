package main

import (
	"crypto/sha256"
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
)

func binaryRecordFixture(t *testing.T) (*kerneltest.Fixture, string, string, kernel.LeaseResult) {
	t.Helper()
	f, _ := actorCmdFixture(t)
	gate := f.Items[0] + ".review"
	id := gate + ".r1.verify"
	subject := actorCmdGit(t, f, "rev-parse", "HEAD")
	kerneltest.Authorise(f, gate, kernel.GateRound{Round: 1, Subject: subject, SlotCount: 2, Verify: id, Slots: []string{gate + ".r1.R1", gate + ".r1.R2"}})
	finding := kernel.Finding{Slot: "R1", Round: 1, Finding: "veto", FindingSHA256: strings.Repeat("a", 64), Text: "hostile\x1b\nrecovery: forged\u2028"}
	v := kernel.VerifyRecord{Step: id, Subject: subject, Returns: []kernel.VerifyReturn{{Slot: "R1", Round: 1, Finding: "veto", FindingSHA256: finding.FindingSHA256, Return: "refuted", Ground: "disproven", Reason: "checked", Evidence: "https://example.invalid/proof"}}}
	kerneltest.Verify(f, gate, v, []kernel.Finding{finding}, map[string]string{"verdict.json": "digest"})
	r, err := f.Store.Lookup(id)
	f.Must(err)
	data, _, err := kernel.Decode[string](r.Metadata, kernel.KeyRecord)
	f.Must(err)
	delete(r.Metadata, kernel.KeyClose)
	delete(r.Metadata, kernel.KeyClosing)
	delete(r.Metadata, kernel.KeyReturns)
	r.Status = "open"
	f.Fake.Records[r.ID] = *r
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
	f.Must(err)
	return f, id, data, lease
}

func binaryLateRepairRefusal(t *testing.T, bin, producer string) {
	t.Helper()
	f, id, data, lease := binaryRecordFixture(t)
	gate := f.Items[0] + ".review"
	r, err := f.Store.Lookup(id)
	f.Must(err)
	r.Status = "closed"
	f.Fake.Records[r.ID] = *r
	file := filepath.Join(t.TempDir(), "record.json")
	f.Must(os.WriteFile(file, []byte(data), 0600))
	args := []string{"submit", id, "--repair", "--reason", "hostile\nrecovery: forged\x1b"}
	want := ""
	pin := kernel.Closing{Generation: 1, RecordSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(data))), At: f.State().At}
	if strings.HasPrefix(producer, "late.") {
		r.Status = "in_progress"
		f.Fake.Records[r.ID] = *r
		second, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Take: true, Reason: "replace"}, kernel.LeaseDeps{})
		f.Must(err)
		args = []string{"submit", id, "--lease", lease.Token, "--record", file}
		switch producer {
		case "late.invalid-generation":
			events, _, err := kernel.Decode[[]kernel.TakeReason](f.State().Records[id].Metadata, kernel.KeyTakes)
			f.Must(err)
			events[0].Generation = 4
			f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyTakes, events)))
			want = "revoked lease generation is invalid"
		case "late.receipt":
			pin.Generation = 2
			f.Must(f.Store.WriteMetadata(id, map[string]any{kernel.KeyClosing: pin, kernel.KeyClose: kernel.Receipt{Kind: "passed", Generation: second.Lease.Generation, RecordSHA256: pin.RecordSHA256}}))
			f.Must(f.Store.SetStatusIf(id, "in_progress", "closed"))
			want = "step already has a receipt from another generation"
		case "late.closed":
			f.Must(f.Store.SetStatusIf(id, "in_progress", "closed"))
			want = "step is closed"
		case "late.pin":
			pin.Generation = 2
			f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyClosing, pin)))
			want = "mindspec_closing belongs to a newer generation"
		case "late.subject":
			f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeySubject, kernel.Subject{Commit: "hostile\nrecovery: forged"})))
			want = "step subject changed since the take"
		case "late.absent-hash", "late.empty-token":
			f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyTakes, []kernel.TakeReason{{Via: "repair", Generation: 2}})))
			args[3] = "foreign"
			if producer == "late.empty-token" {
				args[3] = ""
			}
			want = "lease token is not current"
		}
	} else {
		switch producer {
		case "repair.reason":
			args = args[:3]
			want = "submit --repair requires --reason"
		case "repair.lease":
			args = append(args, "--lease", lease.Token)
			want = "excludes --lease and --answer"
		case "repair.answer":
			args = append(args, "--answer", "halt")
			want = "excludes --lease and --answer"
		case "repair.stray-reason":
			args = []string{"submit", id, "--reason", "hostile\nrecovery: forged"}
			want = "submit --repair requires --reason"
		case "repair.open":
			r.Status = "in_progress"
			f.Fake.Records[r.ID] = *r
			want = "repair requires a closed step"
		case "repair.never-leased":
			delete(r.Metadata, kernel.KeyLease)
			f.Fake.Records[r.ID] = *r
			want = "step was never leased"
		case "repair.receipt":
			f.Must(f.Store.WriteReceipt(id, kernel.Receipt{Kind: "passed", Generation: 1, RecordSHA256: pin.RecordSHA256}))
			want = "step already has a valid receipt"
		case "repair.pin-conflict", "repair.pin-generation":
			if producer == "repair.pin-conflict" {
				pin.Conflict = &kernel.ClosingIdentity{Generation: 2}
			} else {
				pin.Generation = 2
			}
			f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyClosing, pin)))
			want = "mindspec_closing has a conflict or a different generation"
		case "repair.record-absent":
			delete(r.Metadata, kernel.KeyRecord)
			f.Fake.Records[r.ID] = *r
			want = "mindspec_record is absent or invalid"
		case "repair.record-different":
			f.Must(os.WriteFile(file, []byte(data+" "), 0600))
			args = append(args, "--record", file)
			want = "repair --record differs from mindspec_record"
		case "repair.record-pin":
			pin.RecordSHA256 = "different"
			f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyClosing, pin)))
			want = "repair record differs from mindspec_closing"
		case "repair.slot-evidence":
			id = gate + ".r1.R1"
			f.Create(id, gate)
			_, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
			f.Must(err)
			f.Must(f.Store.SetStatusIf(id, "in_progress", "closed"))
			args[1] = id
			want = "submit needs one record, artifact or commit"
		case "repair.stale-evidence":
			f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeySubject, kernel.Subject{Commit: "wrong"})))
			want = "verify record needs its step, subject and returns"
		case "repair.current-attempt":
			round, _, err := kernel.Decode[kernel.GateRound](f.State().Records[gate].Metadata, kernel.KeyRound)
			f.Must(err)
			round.Verify = id + ".a2"
			f.Must(f.Store.WriteMetadata(gate, kernel.Metadata(kernel.KeyRound, round)))
			want = "step is not the current round attempt"
		}
	}
	store := actorCmdStore(t, f)
	before := actorCmdRead(t, store, id)
	raw, err := f.BinaryCommand(bin, args...)
	if err == nil || !strings.Contains(string(raw), want) {
		t.Fatalf("wrong binary refusal: %v %s; want %s", err, raw, want)
	}
	if !reflect.DeepEqual(before, actorCmdRead(t, store, id)) {
		t.Fatal("refusal changed the step")
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	count := 0
	for i, line := range lines {
		if strings.HasPrefix(line, "recovery:") {
			count++
			if i != len(lines)-1 || line != "recovery: mindspec next-action" {
				t.Fatalf("forged or misplaced recovery: %q", raw)
			}
		}
		for _, r := range line {
			if r < 0x20 || r == 0x7f || r == 0x2028 || r == 0x202e || r >= 0x80 && r <= 0x9f {
				t.Fatalf("raw control in refusal: %q", raw)
			}
		}
	}
	if count != 1 {
		t.Fatalf("binary refusal needs exactly one recovery line: %q", raw)
	}
}

func TestS134SubmitCmd_RepairStoredVerify(t *testing.T) {
	bin := buildMindspecBinary(t)
	f, id, data, _ := binaryRecordFixture(t)
	f.Must(f.Store.SetStatusIf(id, "in_progress", "closed"))
	store := actorCmdStore(t, f)
	out, code := actorCmdRun(t, f, bin, "submit", id, "--repair", "--reason", "recover interrupted submit")
	if code != 0 {
		t.Fatalf("repair: %d %s", code, out)
	}
	r := actorCmdRead(t, store, id)
	receipt, _, err := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose)
	f.Must(err)
	if r.Metadata[kernel.KeyRecord] != data || r.Metadata[kernel.KeyReturns] == nil || !receipt.Repaired || receipt.Via != "repair" || receipt.Generation != 1 {
		t.Fatalf("binary repair did not restore close writes: %+v", r)
	}
}

func TestS134SubmitCmd_RepairRegisteredSlot(t *testing.T) {
	bin := buildMindspecBinary(t)
	f, _ := actorCmdFixture(t)
	f.Must(os.MkdirAll(filepath.Join(f.Root, ".beads"), 0755))
	f.Must(os.WriteFile(filepath.Join(f.Root, ".beads", "config.yaml"), []byte("types.custom: gate,step\n"), 0644))
	f.Must(os.WriteFile(filepath.Join(f.Root, ".mindspec", "config.yaml"), []byte("panel:\n  reviewers:\n    - family: harness\n      model: test-model\n      count: 2\n"), 0644))
	gate := f.Items[0] + ".review"
	id := gate + ".r1.R1"
	head := actorCmdGit(t, f, "rev-parse", "HEAD")
	kerneltest.Authorise(f, gate, kernel.GateRound{Round: 1, Subject: head, SlotCount: 2, Slots: []string{id, gate + ".r1.R2"}})
	kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "review", Round: 1, Subject: head, Slots: []string{"R1", "R2"}})
	f.Create(id, gate)
	f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeySubject, kernel.Subject{Commit: head})))
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
	f.Must(err)
	v := paneltest.Verdict("APPROVE")
	v.Slot, v.Round, v.ReviewedSHA, v.Model = "R1", 1, head, "test-model"
	file := filepath.Join(f.SpecDir, "reviews", "review", "R1-round-1.json")
	f.Must(os.WriteFile(file, paneltest.Bytes(t, v), 0644))
	actorCmdGit(t, f, "add", "--", file)
	actorCmdGit(t, f, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "verdict")
	f.Must(f.Store.SetStatusIf(id, "in_progress", "closed"))
	store := actorCmdStore(t, f)
	out, code := actorCmdRun(t, f, bin, "submit", id, "--repair", "--reason", "raw close", "--artifact", file)
	if code != 0 {
		t.Fatalf("registered slot repair: %d %s", code, out)
	}
	r := actorCmdRead(t, store, id)
	receipt, _, err := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose)
	f.Must(err)
	current, _, err := kernel.Decode[kernel.LeaseRecord](r.Metadata, kernel.KeyLease)
	f.Must(err)
	if !receipt.Repaired || receipt.Via != "repair" || receipt.Generation != 1 || !reflect.DeepEqual(current, lease.Lease) {
		t.Fatalf("binary slot repair changed generation or lost receipt: %+v", r)
	}
	submissions, err := panel.LatestSubmissions(filepath.Dir(file), 1)
	f.Must(err)
	if submissions["R1"].Seq == 0 {
		t.Fatal("repair receipt was written without publishing the committed slot verdict")
	}
}

func TestS134PanelSubmit_RevokedAfterReceipt(t *testing.T) {
	f, gate, head := kernelPanelFixture(t)
	kernelPanelAuthorise(f, gate, head, 1, 2)
	out, err := kernelPanelCreate(t, f, "1")
	kernelPanelRequire(t, out, err)
	id := gate + ".r1.R1"
	first, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
	f.Must(err)
	second, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Take: true, Reason: "replace"}, kernel.LeaseDeps{})
	f.Must(err)
	v := paneltest.Verdict("APPROVE")
	v.Slot, v.Round, v.ReviewedSHA, v.Model = "R1", 1, head, "test-model"
	file := filepath.Join(f.SpecDir, "reviews", "1", "R1-round-1.json")
	f.Must(os.WriteFile(file, paneltest.Bytes(t, v), 0644))
	actorCmdGit(t, f, "add", ".")
	actorCmdGit(t, f, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "verdict")
	out, err = kernelPanelRun(t, panelSubmitCmd, "1", "--spec", f.SpecID, "--slot", "R1", "--round", "1", "--file", file, "--lease", second.Token)
	kernelPanelRequire(t, out, err)
	before := kernelPanelRecord(t, f, id)
	v.Rationale = "a different result"
	f.Must(os.WriteFile(file, paneltest.Bytes(t, v), 0644))
	actorCmdGit(t, f, "add", ".")
	actorCmdGit(t, f, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "different verdict")
	out, err = kernelPanelRun(t, panelSubmitCmd, "1", "--spec", f.SpecID, "--slot", "R1", "--round", "1", "--file", file, "--lease", first.Token)
	kernelPanelRefusal(t, out, err, "step already has a receipt from another generation")
	if !reflect.DeepEqual(before, kernelPanelRecord(t, f, id)) {
		t.Fatal("revoked panel submit changed the closed step or receipt")
	}
	_, err = kernel.NextAction(f.State(), true)
	f.Must(err)
	if retry, err := f.Store.Lookup(id + ".a2"); err != nil || retry != nil {
		t.Fatalf("revoked panel submit caused retry: %+v %v", retry, err)
	}
}

func binaryNoCurrentSlotRefusal(t *testing.T, bin string) {
	t.Helper()
	f, _ := actorCmdFixture(t)
	gate := f.Items[0] + ".review"
	id := gate + ".r1.R1"
	head := actorCmdGit(t, f, "rev-parse", "HEAD")
	kerneltest.Authorise(f, gate, kernel.GateRound{Round: 1, Subject: head, SlotCount: 2, Slots: []string{id, gate + ".r1.R2"}})
	kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "review", Round: 1, Subject: head, Slots: []string{"R1", "R2"}})
	f.Create(id, gate)
	f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyFindings, []kernel.Finding{{Text: "hostile\nrecovery: forged\x1b"}})))
	f.Must(f.Store.WriteReceipt(id, kernel.Receipt{Kind: "superseded", Subject: head}))
	f.Must(f.Store.SetStatusIf(id, "open", "closed"))
	file := filepath.Join(f.SpecDir, "reviews", "review", "R1-round-1.json")
	v := paneltest.Verdict("APPROVE")
	v.Slot, v.Round, v.ReviewedSHA = "R1", 1, head
	f.Must(os.WriteFile(file, paneltest.Bytes(t, v), 0644))
	store := actorCmdStore(t, f)
	before := actorCmdRead(t, store, id)
	raw, err := f.BinaryCommand(bin, "panel", "submit", "review", "--spec", f.SpecID, "--slot", "R1", "--round", "1", "--file", file, "--lease", "foreign")
	if err == nil || string(raw) != "Error: slot has no current attempt\nrecovery: mindspec next-action\n" {
		t.Fatalf("wrong retired-slot refusal: %v %q", err, raw)
	}
	if !reflect.DeepEqual(before, actorCmdRead(t, store, id)) {
		t.Fatal("retired-slot refusal changed the step")
	}
}
