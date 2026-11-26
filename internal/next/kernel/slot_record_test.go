package kernel_test

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// attemptVerdict writes data as the verdict of a slot attempt directory, the
// path that the slot prompt names.
func attemptVerdict(t *testing.T, f *kerneltest.Fixture, slot, data string) string {
	t.Helper()
	iso, err := config.IsolationRootFor(f.Root)
	f.Must(err)
	layout, err := panel.NewSlotLayout(f.Root, iso, f.SpecID, "panel", 1, slot)
	f.Must(err)
	file := filepath.Join(layout.Dir, "attempt-1", "verdict.json")
	f.Must(os.MkdirAll(filepath.Dir(file), 0755))
	f.Must(os.WriteFile(file, []byte(data), 0644))
	return file
}

// gitObjects lists every ref and every commit object of the fixture
// repository, unreachable commits included.
func gitObjects(t *testing.T, f *kerneltest.Fixture) string {
	t.Helper()
	refs := actorCommand(t, f, "for-each-ref", "--format=%(refname) %(objectname)")
	var commits []string
	for _, line := range strings.Split(actorCommand(t, f, "cat-file", "--batch-all-objects", "--batch-check=%(objecttype) %(objectname)"), "\n") {
		if strings.HasPrefix(line, "commit ") {
			commits = append(commits, line)
		}
	}
	return refs + "\n" + strings.Join(commits, "\n")
}

func slotRecordFixture(t *testing.T) (*kerneltest.Fixture, string, kernel.LeaseResult, string) {
	t.Helper()
	f, _, _ := actorFixture(t)
	gate := f.Items[0] + ".review"
	id := gate + ".r1.R1"
	subject := actorCommand(t, f, "rev-parse", "HEAD")
	f.Create(id, gate)
	kerneltest.Authorise(f, gate, kernel.GateRound{Round: 1, Subject: subject, Slots: []string{gate + ".r1.R1", gate + ".r1.R2"}, SlotCount: 2})
	f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeySubject, kernel.Subject{Commit: subject})))
	kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "panel", Round: 1, Subject: subject, Slots: []string{"R1", "R2"}})
	// The lease applies the pending steps, so a later submit adds none.
	lease := leaseActor(t, f, id, kernel.LeaseDeps{})
	if _, err := kernel.Pass(kernel.Rules(), f.State(), true); err != nil {
		t.Fatal(err)
	}
	return f, id, lease, subject
}

func digestOf(data string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(data)))
}

// The printed slot submit stores the verdict bytes as the step record. It
// writes no ref and no commit, and a worktree on the spec branch keeps its
// index and files.
func TestS134Submit_SlotVerdictStoredAsRecord(t *testing.T) {
	f, id, lease, subject := slotRecordFixture(t)
	tree := filepath.Join(t.TempDir(), "spec")
	actorCommand(t, f, "worktree", "add", tree, "spec/"+f.SpecID)
	f.Must(os.WriteFile(filepath.Join(tree, "staged.txt"), []byte("staged\n"), 0644))
	f.Must(os.WriteFile(filepath.Join(tree, "unstaged.txt"), []byte("unstaged\n"), 0644))
	actorCommand(t, f, "-C", tree, "add", "staged.txt")
	status := actorCommand(t, f, "-C", tree, "status", "--porcelain")
	data := verdictBytes(subject, "R1")
	file := attemptVerdict(t, f, "R1", data)
	before := gitObjects(t, f)
	if _, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file}); err != nil {
		t.Fatalf("the printed slot submit was refused: %v", err)
	}
	row := actorRecord(t, f, id)
	if row.Status != "closed" {
		t.Fatalf("slot status %q, want closed", row.Status)
	}
	receipt := actorReceipt(t, f, id)
	if receipt.RecordSHA256 != digestOf(data) || receipt.Path != "" || receipt.Commit != "" || receipt.Blob != "" {
		t.Fatalf("slot receipt %+v, want record_sha256 %s and no path, commit or blob", receipt, digestOf(data))
	}
	if row.Metadata[kernel.KeyRecord] != data {
		t.Fatalf("mindspec_record %q, want the verdict bytes", row.Metadata[kernel.KeyRecord])
	}
	if err := kernel.ValidReceipt(f.Root, receipt, row.Metadata); err != nil {
		t.Fatalf("stored receipt is not valid: %v", err)
	}
	if after := gitObjects(t, f); after != before {
		t.Fatalf("slot submit wrote a ref or commit:\nbefore %s\nafter %s", before, after)
	}
	if after := actorCommand(t, f, "-C", tree, "status", "--porcelain"); after != status {
		t.Fatalf("spec worktree status changed:\nbefore %s\nafter %s", status, after)
	}
}

// A reader takes the verdict bytes from the stored record alone: the
// verdict file and every git object of it can be gone.
func TestS134Submit_SlotReceiptBytesComeFromRecord(t *testing.T) {
	f, id, lease, subject := slotRecordFixture(t)
	data := verdictBytes(subject, "R1")
	file := attemptVerdict(t, f, "R1", data)
	if _, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file}); err != nil {
		t.Fatal(err)
	}
	f.Must(os.Remove(file))
	row := actorRecord(t, f, id)
	got, err := kernel.ReceiptBytes(row, actorReceipt(t, f, id))
	if err != nil || string(got) != data {
		t.Fatalf("receipt bytes %q, %v; want the stored verdict", got, err)
	}
	if _, err := f.Command("git", "cat-file", "-e", strings.TrimSpace(actorHashObject(t, f, data))); err == nil {
		t.Fatal("slot submit wrote the verdict bytes as a git object")
	}
}

func actorHashObject(t *testing.T, f *kerneltest.Fixture, data string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "verdict")
	f.Must(os.WriteFile(file, []byte(data), 0644))
	return actorCommand(t, f, "hash-object", file)
}

// A changed mindspec_record does not pass: the receipt hash must match it.
func TestS134Submit_SlotReceiptHashCheckedAgainstRecord(t *testing.T) {
	f, id, lease, subject := slotRecordFixture(t)
	data := verdictBytes(subject, "R1")
	if _, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: attemptVerdict(t, f, "R1", data)}); err != nil {
		t.Fatal(err)
	}
	receipt := actorReceipt(t, f, id)
	f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyRecord, strings.Replace(data, "APPROVE", "REQUEST_CHANGES", 1))))
	row := actorRecord(t, f, id)
	if err := kernel.ValidReceipt(f.Root, receipt, row.Metadata); err == nil {
		t.Fatal("a changed mindspec_record passed the receipt check")
	}
	if _, err := kernel.ReceiptBytes(row, receipt); err == nil || !strings.Contains(err.Error(), "record_sha256 does not match mindspec_record") {
		t.Fatalf("receipt bytes of a changed mindspec_record: %v", err)
	}
	// A git-backed slot receipt is not valid either.
	git := kernel.Receipt{Kind: "passed", Path: "verdict.json", Commit: strings.Repeat("a", 40), Blob: strings.Repeat("b", 40)}
	if err := kernel.ValidReceipt(f.Root, git, row.Metadata); err == nil || !strings.Contains(err.Error(), "slot receipt must name record_sha256") {
		t.Fatalf("git-backed slot receipt: %v", err)
	}
}

// A retry with the same bytes changes nothing; other bytes are refused.
func TestS134Submit_SlotRecordRetryIsIdempotent(t *testing.T) {
	f, id, lease, subject := slotRecordFixture(t)
	data := verdictBytes(subject, "R1")
	file := attemptVerdict(t, f, "R1", data)
	if _, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file}); err != nil {
		t.Fatal(err)
	}
	first := actorReceipt(t, f, id)
	before := kerneltest.Snapshot(t, f)
	if _, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file}); err != nil {
		t.Fatalf("retry with the same bytes was refused: %v", err)
	}
	if after := kerneltest.Snapshot(t, f); after != before || !reflect.DeepEqual(actorReceipt(t, f, id), first) {
		t.Fatal("retry with the same bytes changed the store")
	}
	file = attemptVerdict(t, f, "R1", strings.Replace(data, `"reviewed"`, `"reviewed again"`, 1))
	_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file})
	assertSubmitRefusal(t, err, "step already has a different result", "mindspec next-action")
}

// A refused slot submit writes no record, ref or commit.
func TestS134Submit_RefusedSlotVerdictWritesNothing(t *testing.T) {
	for _, name := range []string{"draft", "wrong-reviewed-sha", "other-slot-attempt", "outside-panel"} {
		t.Run(name, func(t *testing.T) {
			f, id, lease, subject := slotRecordFixture(t)
			data := verdictBytes(subject, "R1")
			file := ""
			want := ""
			switch name {
			case "draft":
				file, want = attemptVerdict(t, f, "R1", strings.Replace(data, `"final"`, `"draft"`, 1)), "verdict"
			case "wrong-reviewed-sha":
				file, want = attemptVerdict(t, f, "R1", verdictBytes(strings.Repeat("0", 40), "R1")), "does not match the step"
			case "other-slot-attempt":
				file, want = attemptVerdict(t, f, "R2", data), "verdict must be the slot attempt verdict or a file in its registered panel directory"
			case "outside-panel":
				file = filepath.Join(f.Root, "verdict.json")
				f.Must(os.WriteFile(file, []byte(data), 0644))
				want = "verdict must be the slot attempt verdict or a file in its registered panel directory"
			}
			before, objects := kerneltest.Snapshot(t, f), gitObjects(t, f)
			_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file})
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("want refusal %q, got %v", want, err)
			}
			if strings.Count(err.Error(), "recovery:") != 1 {
				t.Fatalf("want one recovery line: %v", err)
			}
			if after := kerneltest.Snapshot(t, f); after != before || gitObjects(t, f) != objects {
				t.Fatalf("a refused slot submit wrote state: %v\nbefore %s\nafter %s", err, before, after)
			}
		})
	}
}

// A verdict in the registered panel directory is accepted, as the printed
// recovery path.
func TestS134Submit_SlotVerdictInRegisteredPanelDirectory(t *testing.T) {
	f, id, lease, subject := slotRecordFixture(t)
	path, err := kernel.SlotVerdictPath(f.State(), id)
	f.Must(err)
	file := filepath.Join(f.Root, filepath.FromSlash(path))
	f.Must(os.MkdirAll(filepath.Dir(file), 0755))
	data := verdictBytes(subject, "R1")
	f.Must(os.WriteFile(file, []byte(data), 0644))
	if _, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file}); err != nil {
		t.Fatalf("registered verdict file was refused: %v", err)
	}
	if actorReceipt(t, f, id).RecordSHA256 != digestOf(data) {
		t.Fatal("receipt does not name the stored verdict")
	}
}

// A verdict larger than the verdict bound is refused with one recovery that
// names the cause.
func TestS134Submit_SlotVerdictSizeBound(t *testing.T) {
	f, id, lease, subject := slotRecordFixture(t)
	data := strings.Replace(verdictBytes(subject, "R1"), `"reviewed"`, `"`+strings.Repeat("x", panel.MaxVerdictBytes)+`"`, 1)
	file := attemptVerdict(t, f, "R1", data)
	before := kerneltest.Snapshot(t, f)
	_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file})
	assertSubmitRefusal(t, err, fmt.Sprintf("verdict file is larger than %d bytes", panel.MaxVerdictBytes), fmt.Sprintf("make the verdict shorter than %d bytes (put long evidence in a file and name its path), then run the same submit command again", panel.MaxVerdictBytes))
	if kerneltest.Snapshot(t, f) != before {
		t.Fatal("an oversized verdict wrote state")
	}
}

// rawSlotClose closes the step outside the kernel. With stored, a submit
// first fails after its record write, so the step keeps the record and no
// receipt. Without it, the slot was never submitted.
func rawSlotClose(t *testing.T, f *kerneltest.Fixture, id string, lease kernel.LeaseResult, data string, stored bool) {
	t.Helper()
	if stored {
		failAfterSlotRecord(t, f, id, lease, attemptVerdict(t, f, "R1", data))
	}
	f.Must(f.Store.SetStatusIf(id, actorRecord(t, f, id).Status, "closed"))
}

// failAfterSlotRecord runs a slot submit that fails after the record write
// and before the receipt, as a crash there does.
func failAfterSlotRecord(t *testing.T, f *kerneltest.Fixture, id string, lease kernel.LeaseResult, artifact string) {
	t.Helper()
	// The record write succeeds. The next write, which carries the receipt,
	// fails.
	f.Fake.Hook = func(args []string) error {
		for i, a := range args {
			if a != "--metadata" || i+1 >= len(args) {
				continue
			}
			m := args[i+1]
			if strings.HasPrefix(m, "@") {
				raw, err := os.ReadFile(strings.TrimPrefix(m, "@"))
				if err != nil {
					return err
				}
				m = string(raw)
			}
			if strings.Contains(m, `"`+kernel.KeyClose+`"`) {
				return errors.New("killed after the record write")
			}
		}
		return nil
	}
	_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: artifact})
	f.Fake.Hook = nil
	if err == nil || !strings.Contains(err.Error(), "killed after the record write") {
		t.Fatalf("injected failure did not stop the submit: %v", err)
	}
	r := actorRecord(t, f, id)
	if _, has := r.Metadata[kernel.KeyRecord]; !has {
		t.Fatal("submit wrote no record before the failure")
	}
	if _, has := r.Metadata[kernel.KeyClose]; has {
		t.Fatal("failed submit wrote a receipt")
	}
}

// Repair of a slot uses the stored record. A supplied record or artifact
// must have its hash. A step with no stored record takes its verdict file,
// named or at the registered path, with the plain submit checks.
func TestS134Submit_SlotRepairUsesStoredRecord(t *testing.T) {
	for _, name := range []string{"stored", "same-record", "same-artifact", "other-record", "other-artifact", "no-record", "no-record-artifact", "no-record-record", "no-record-registered", "no-record-wrong-round"} {
		t.Run(name, func(t *testing.T) {
			f, id, lease, subject := slotRecordFixture(t)
			data := verdictBytes(subject, "R1")
			rawSlotClose(t, f, id, lease, data, !strings.HasPrefix(name, "no-record"))
			other := strings.Replace(data, `"reviewed"`, `"reviewed again"`, 1)
			o := kernel.SubmitOptions{Step: id, Repair: true, Reason: "recover"}
			switch name {
			case "same-record":
				o.Record = recordFile(t, data)
			case "same-artifact":
				o.Artifact = attemptVerdict(t, f, "R1", data)
			case "other-record":
				o.Record = recordFile(t, other)
			case "other-artifact":
				o.Artifact = attemptVerdict(t, f, "R1", other)
			case "no-record-artifact":
				o.Artifact = attemptVerdict(t, f, "R1", data)
			case "no-record-record":
				o.Record = recordFile(t, data)
			case "no-record-registered":
				path, err := kernel.SlotVerdictPath(f.State(), id)
				f.Must(err)
				f.Must(os.MkdirAll(filepath.Dir(filepath.Join(f.Root, path)), 0755))
				f.Must(os.WriteFile(filepath.Join(f.Root, path), []byte(data), 0644))
			case "no-record-wrong-round":
				o.Artifact = attemptVerdict(t, f, "R1", strings.Replace(data, `"round":1`, `"round":2`, 1))
			}
			before := kerneltest.Snapshot(t, f)
			_, err := kernel.Submit(f.State(), o)
			switch name {
			case "other-record":
				assertSubmitRefusal(t, err, "repair --record differs from mindspec_record", "mindspec next-action")
			case "other-artifact":
				assertSubmitRefusal(t, err, "repair --artifact differs from mindspec_record", "mindspec next-action")
			case "no-record":
				assertSubmitRefusal(t, err, "verdict file is not readable", kernel.SlotPromptRecovery(f.State(), id))
			case "no-record-record":
				assertSubmitRefusal(t, err, "slot has no mindspec_record", "mindspec next-action")
			case "no-record-wrong-round":
				assertSubmitRefusal(t, err, "verdict slot, round or subject does not match the step", "mindspec next-action")
			default:
				if err != nil {
					t.Fatalf("repair from the stored record was refused: %v", err)
				}
				receipt := actorReceipt(t, f, id)
				if receipt.RecordSHA256 != digestOf(data) || receipt.Via != "repair" || receipt.Commit != "" {
					t.Fatalf("repair receipt %+v, want the stored record hash", receipt)
				}
				return
			}
			if kerneltest.Snapshot(t, f) != before {
				t.Fatal("a refused repair wrote state")
			}
		})
	}
}

// slotTally tallies the registered panel of the fixture and returns the
// counted verdict of slot, or the reason the tally left it out.
func slotTally(t *testing.T, f *kerneltest.Fixture, slot string) string {
	t.Helper()
	s := f.State()
	for _, p := range s.Panels {
		if p.Slug() != "panel" {
			continue
		}
		res, err := kernel.TallyPanel(s, p)
		f.Must(err)
		for _, v := range res.Verdicts {
			if v.Slot == slot {
				return v.Verdict
			}
		}
		for _, v := range res.NotIn {
			if strings.HasPrefix(v.File, slot+"-") {
				return v.Reason
			}
		}
		for _, name := range res.Unsubmitted {
			if strings.HasPrefix(name, slot+"-") {
				return "unsubmitted"
			}
		}
		return "absent"
	}
	t.Fatal("fixture has no registered panel")
	return ""
}

// A tally takes the verdict of a slot from its stored record when the
// verdict file is absent, as in a fresh clone. The receipt hash must match
// the record, or the slot is not counted and names that cause. A verdict
// file that differs from the record does not count.
func TestS134Submit_TallyReadsStoredSlotRecord(t *testing.T) {
	f, id, lease, subject := slotRecordFixture(t)
	data := verdictBytes(subject, "R1")
	if _, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: attemptVerdict(t, f, "R1", data)}); err != nil {
		t.Fatal(err)
	}
	path, err := kernel.SlotVerdictPath(f.State(), id)
	f.Must(err)
	path = filepath.Join(f.Root, path)
	// The publication hook of the binary records the submission in the ledger.
	f.Must(os.WriteFile(path, []byte(data), 0644))
	_, err = panel.Submit(panel.SubmitTarget{Dir: filepath.Dir(path), Spec: f.SpecID, Panel: "panel", Slot: "R1", Round: 1}, []byte(data), func(panel.Panel, panel.RosterSlot) (panel.SubmitResolution, error) {
		return panel.SubmitResolution{Family: "harness", Model: "test", RunnerKind: "harness", RunnerSource: "default"}, nil
	})
	f.Must(err)
	if got := slotTally(t, f, "R1"); got != "APPROVE" {
		t.Fatalf("tally with the verdict file: %q, want APPROVE", got)
	}
	f.Must(os.Rename(path, filepath.Join(t.TempDir(), "moved.json")))
	if got := slotTally(t, f, "R1"); got != "APPROVE" {
		t.Fatalf("tally with no verdict file: %q, want APPROVE from the stored record", got)
	}
	f.Must(os.WriteFile(path, []byte(strings.Replace(data, "reviewed", "edited", 1)), 0644))
	if got := slotTally(t, f, "R1"); got != "file differs from the stored record" {
		t.Fatalf("tally with an edited verdict file: %q, want the record refusal", got)
	}
	f.Must(os.Rename(path, filepath.Join(t.TempDir(), "edited.json")))
	f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyRecord, strings.Replace(data, "reviewed", "changed", 1))))
	if got := slotTally(t, f, "R1"); got != "stored record does not match its receipt" {
		t.Fatalf("tally with a record that fails its receipt hash: %q, want the receipt refusal", got)
	}
}
