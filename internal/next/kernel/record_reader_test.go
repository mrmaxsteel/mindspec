package kernel_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// refusedRoundVerify refuses a round of vetoes and returns the findings and
// the read set of the verify step that the reducer makes. With fresh, the
// verdict files are moved away first, as in a fresh clone that has only the
// stored slot records.
func refusedRoundVerify(t *testing.T, fresh bool) ([]kernel.Finding, map[string]string) {
	t.Helper()
	f := reducerFixture(t)
	gate := f.Items[0] + ".review"
	refusedPanel(t, f, gate, "review", 1, panel.VerdictReject)
	for _, slot := range []string{"R1", "R2", "R3"} {
		// Each slot holds its verdict as a stored record with a receipt, as
		// the slot submit leaves it.
		file := filepath.Join(f.SpecDir, "reviews", "review", slot+"-round-1.json")
		data, err := os.ReadFile(file)
		f.Must(err)
		id := gate + ".r1." + slot
		f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyRecord, string(data))))
		f.Must(f.Store.WriteReceipt(id, kernel.Receipt{Kind: "passed", Generation: 1, RecordSHA256: digestOf(string(data))}))
		if fresh {
			f.Must(os.Rename(file, filepath.Join(t.TempDir(), slot+".json")))
		}
	}
	reduce(t, f, true)
	round, _, err := kernel.Decode[kernel.GateRound](record(t, f, gate).Metadata, kernel.KeyRound)
	f.Must(err)
	if round.Verify == "" {
		t.Fatalf("refused round made no verify step (fresh=%v)", fresh)
	}
	v := record(t, f, round.Verify)
	findings, _, err := kernel.Decode[[]kernel.Finding](v.Metadata, kernel.KeyFindings)
	f.Must(err)
	read, _, err := kernel.Decode[map[string]string](v.Metadata, kernel.KeyRead)
	f.Must(err)
	return findings, read
}

// The verify step of a refused round has the same findings and the same read
// set in the publishing checkout and in a fresh clone. Each veto hash is the
// hash of the stored record and its text is the reason in that record.
func TestVerify_FreshCloneReadsStoredRecords(t *testing.T) {
	published, publishedRead := refusedRoundVerify(t, false)
	fresh, freshRead := refusedRoundVerify(t, true)
	if len(published) != 3 {
		t.Fatalf("want three vetoes in the publishing checkout: %+v", published)
	}
	empty := digestOf("")
	for _, finding := range fresh {
		if finding.FindingSHA256 == empty || strings.Contains(finding.Text, "gives no reason") {
			t.Fatalf("fresh clone lost the stored veto: %+v", finding)
		}
	}
	if !reflect.DeepEqual(published, fresh) {
		t.Fatalf("finding identity differs:\npublished %+v\nfresh     %+v", published, fresh)
	}
	if len(publishedRead) != 3 || !reflect.DeepEqual(publishedRead, freshRead) {
		t.Fatalf("read set differs:\npublished %v\nfresh     %v", publishedRead, freshRead)
	}
}

// A hand edit replaces the verdict file and appends a matching ledger line.
// The stored record stays the verdict, so the edited file does not count.
func TestTally_EditedVerdictWithLedgerLineDoesNotCount(t *testing.T) {
	f, id, lease, subject := slotRecordFixture(t)
	rc := strings.Replace(verdictBytes(subject, "R1"), `"verdict":"APPROVE"`, `"verdict":"REQUEST_CHANGES"`, 1)
	rc = strings.Replace(rc, `"concrete_changes_required":[]`, `"concrete_changes_required":[{"text":"defect","basis":"traced"}]`, 1)
	if _, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: attemptVerdict(t, f, "R1", rc)}); err != nil {
		t.Fatal(err)
	}
	path, err := kernel.SlotVerdictPath(f.State(), id)
	f.Must(err)
	path = filepath.Join(f.Root, path)
	resolve := func(panel.Panel, panel.RosterSlot) (panel.SubmitResolution, error) {
		return panel.SubmitResolution{Family: "harness", Model: "test", RunnerKind: "harness", RunnerSource: "default"}, nil
	}
	f.Must(os.WriteFile(path, []byte(rc), 0644))
	_, err = panel.Submit(panel.SubmitTarget{Dir: filepath.Dir(path), Spec: f.SpecID, Panel: "panel", Slot: "R1", Round: 1}, []byte(rc), resolve)
	f.Must(err)
	if got := slotTally(t, f, "R1"); got != panel.VerdictRequestChanges {
		t.Fatalf("tally after the submit: %q, want %s", got, panel.VerdictRequestChanges)
	}
	approve := verdictBytes(subject, "R1")
	f.Must(os.WriteFile(path, []byte(approve), 0644))
	_, err = panel.Submit(panel.SubmitTarget{Dir: filepath.Dir(path), Spec: f.SpecID, Panel: "panel", Slot: "R1", Round: 1}, []byte(approve), resolve)
	f.Must(err)
	if got := slotTally(t, f, "R1"); got != "file differs from the stored record" {
		t.Fatalf("edited verdict with a ledger line: %q, want the record refusal", got)
	}
}

// publishSlot writes data as the registered verdict file of slot and appends
// its ledger line, as panel submit does.
func publishSlot(t *testing.T, f *kerneltest.Fixture, dir, slot, data string) {
	t.Helper()
	f.Must(os.WriteFile(filepath.Join(dir, slot+"-round-1.json"), []byte(data), 0644))
	_, err := panel.Submit(panel.SubmitTarget{Dir: dir, Spec: f.SpecID, Panel: "panel", Slot: slot, Round: 1}, []byte(data), func(panel.Panel, panel.RosterSlot) (panel.SubmitResolution, error) {
		return panel.SubmitResolution{Family: "harness", Model: "test", RunnerKind: "harness", RunnerSource: "default"}, nil
	})
	f.Must(err)
}

// requestChanges is the APPROVE verdict of slot with one finding and the
// REQUEST_CHANGES verdict word.
func requestChanges(subject, slot, finding string) string {
	rc := strings.Replace(verdictBytes(subject, slot), `"verdict":"APPROVE"`, `"verdict":"REQUEST_CHANGES"`, 1)
	return strings.Replace(rc, `"concrete_changes_required":[]`, `"concrete_changes_required":[{"text":"`+finding+`","basis":"traced"}]`, 1)
}

// A raw metadata write changes the stored record of a receipted slot, so the
// record no longer matches the receipt hash. The slot is not counted, and the
// APPROVE file on disk does not count in its place.
func TestTally_CorruptStoredRecordDoesNotCountDiskVerdict(t *testing.T) {
	f, id, lease, subject := slotRecordFixture(t)
	rc := requestChanges(subject, "R1", "defect")
	_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: attemptVerdict(t, f, "R1", rc)})
	f.Must(err)
	rel, err := kernel.SlotVerdictPath(f.State(), id)
	f.Must(err)
	dir := filepath.Dir(filepath.Join(f.Root, rel))
	publishSlot(t, f, dir, "R1", rc)
	id2 := strings.TrimSuffix(id, "R1") + "R2"
	l2 := leaseActor(t, f, id2, kernel.LeaseDeps{})
	approve2 := verdictBytes(subject, "R2")
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id2, Lease: l2.Token, Artifact: attemptVerdict(t, f, "R2", approve2)})
	f.Must(err)
	publishSlot(t, f, dir, "R2", approve2)
	publishSlot(t, f, dir, "R1", verdictBytes(subject, "R1"))
	if got := slotTally(t, f, "R1"); got != "file differs from the stored record" {
		t.Fatalf("edited file before the corruption: %q", got)
	}
	f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyRecord, requestChanges(subject, "R1", "changed defect"))))
	if _, err := kernel.ReceiptBytes(actorRecord(t, f, id), actorReceipt(t, f, id)); err == nil {
		t.Fatal("the changed record still matches its receipt")
	}
	if got := slotTally(t, f, "R1"); got != "stored record does not match its receipt" {
		t.Fatalf("slot with a corrupt record tallies %q, want the receipt refusal", got)
	}
	for _, p := range f.State().Panels {
		if p.Slug() != "panel" {
			continue
		}
		res, err := kernel.TallyPanel(f.State(), p)
		f.Must(err)
		if res.Approves != 1 || res.PassRule(true) {
			t.Fatalf("disk APPROVE counted for a corrupt record: approves=%d pass=%v", res.Approves, res.PassRule(true))
		}
	}
}

// After a retry opens for an edited verdict file, the file still belongs to
// the first attempt, whose stored record says REQUEST_CHANGES. The edited
// APPROVE file does not count while the retry is open or after a raw close of
// the retry.
func TestTally_EditedVerdictWithOpenRetryDoesNotCount(t *testing.T) {
	f, id, lease, subject := slotRecordFixture(t)
	rc := requestChanges(subject, "R1", "defect")
	_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: attemptVerdict(t, f, "R1", rc)})
	f.Must(err)
	rel, err := kernel.SlotVerdictPath(f.State(), id)
	f.Must(err)
	path := filepath.Join(f.Root, rel)
	publishSlot(t, f, filepath.Dir(path), "R1", rc)
	publishSlot(t, f, filepath.Dir(path), "R1", verdictBytes(subject, "R1"))
	retry := id + ".a2"
	f.Create(retry, f.Items[0]+".review")
	// The kernel opens an uncounted retry for a slot whose file is not counted.
	f.Must(f.Store.WriteMetadata(retry, map[string]any{kernel.KeyFrom: kernel.From(id), kernel.KeySubject: kernel.Subject{Commit: subject}, kernel.KeyUncounted: true}))
	if root, err := kernel.SlotLineageRoot(f.State(), retry); err != nil || root != id {
		t.Fatalf("the retry is not in the lineage of the first attempt: %q %v", root, err)
	}
	if got := slotTally(t, f, "R1"); got != "file differs from the stored record" {
		t.Fatalf("edited verdict with an open retry tallies %q, want the record refusal", got)
	}
	f.Must(f.Store.SetStatusIf(retry, "open", "closed"))
	if got := slotTally(t, f, "R1"); got != "file differs from the stored record" {
		t.Fatalf("edited verdict with a raw-closed retry tallies %q, want the record refusal", got)
	}
}
