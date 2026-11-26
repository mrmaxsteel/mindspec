package kernel_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func TestS134Submit_RepairFinalMetadataBound(t *testing.T) {
	for _, kind := range []string{"verify", "lens", "slot"} {
		t.Run(kind, func(t *testing.T) {
			var f *kerneltest.Fixture
			var id, artifact string
			if kind == "slot" {
				var subject string
				f, id, _, subject = slotFixture(t)
				artifact = "verdict.json"
				data := verdictBytes(subject, "R1")
				actorCommit(t, f, artifact, data, "")
				// A crash after the record write leaves the stored verdict.
				f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyRecord, data)))
			} else {
				f, id, _ = seededRecordActor(t, kind)
				changeActorMetadata(t, f, id, "open", kernel.KeyClose, kernel.KeyClosing, kernel.KeyReturns)
				leaseActor(t, f, id, kernel.LeaseDeps{})
			}
			if kind == "lens" {
				// Model the lens hook's round update before the receipt.
				gate := f.Items[0] + ".review"
				round, _, err := kernel.Decode[kernel.GateRound](actorRecord(t, f, gate).Metadata, kernel.KeyRound)
				f.Must(err)
				round.Lenses = nil
				f.Must(f.Store.WriteMetadata(gate, kernel.Metadata(kernel.KeyRound, round)))
				t.Cleanup(kernel.ReplaceCloseWriteHookForTest(kind, func(_ kernel.State, r kernel.Record, _ kernel.Receipt) ([]kernel.Write, error) {
					data, _, err := kernel.Decode[string](r.Metadata, kernel.KeyRecord)
					if err != nil {
						return nil, err
					}
					var record kernel.LensRecord
					if err := json.Unmarshal([]byte(data), &record); err != nil {
						return nil, err
					}
					round.Lenses = record.Lenses
					return []kernel.Write{{Kind: "metadata", ID: gate, Metadata: kernel.Metadata(kernel.KeyRound, round)}}, nil
				}))
			}
			changeActorMetadata(t, f, id, "closed")
			f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyTakes, []kernel.TakeReason{{Via: "lease", Generation: 1, Reason: strings.Repeat("h", kernel.MetadataLimit-4096)}})))
			before := actorRecord(t, f, id)
			_, err := kernel.CheckMetadataSize(id, before.Metadata)
			f.Must(err)
			beforeAll := kerneltest.Snapshot(t, f)
			published := false
			_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Artifact: artifact, Repair: true, Reason: strings.Repeat("r", 8192), BeforeClose: func(kernel.State, kernel.Receipt) error {
				published = true
				return nil
			}})
			assertMetadataSizeRefusal(t, err)
			after := actorRecord(t, f, id)
			if _, has := after.Metadata[kernel.KeyClosing]; has {
				t.Error("oversized repair stored mindspec_closing")
			}
			if !reflect.DeepEqual(before.Metadata[kernel.KeyTakes], after.Metadata[kernel.KeyTakes]) {
				t.Error("oversized repair changed mindspec_takes")
			}
			if !reflect.DeepEqual(before, after) || beforeAll != kerneltest.Snapshot(t, f) {
				t.Error("oversized repair changed stored metadata or status")
			}
			if published {
				t.Error("oversized repair reached BeforeClose")
			}
		})
	}
}

func TestS134Submit_LateLeaseFinalMetadataBound(t *testing.T) {
	f, id, data := seededRecordActor(t, "verify")
	changeActorMetadata(t, f, id, "open", kernel.KeyClose, kernel.KeyClosing, kernel.KeyReturns)
	first := leaseActor(t, f, id, kernel.LeaseDeps{})
	second, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Take: true, Reason: "replace"}, kernel.LeaseDeps{})
	f.Must(err)
	projected := actorRecord(t, f, id).Metadata
	at := f.State().At.UTC()
	projected[kernel.KeyClose] = kernel.Receipt{Kind: "passed", RecordSHA256: digest(data), Generation: 1, SupersededGeneration: 2, At: at.Format("2006-01-02T15:04:05.999999999Z07:00")}
	projected[kernel.KeyClosing] = kernel.Closing{Generation: 1, RecordSHA256: digest(data), At: at}
	var record kernel.VerifyRecord
	f.Must(json.Unmarshal([]byte(data), &record))
	projected[kernel.KeyReturns] = record.Returns
	projected["pad"] = ""
	raw, err := json.Marshal(projected)
	f.Must(err)
	pad := strings.Repeat("p", kernel.MetadataLimit-len(raw))
	projected["pad"] = pad
	_, err = kernel.CheckMetadataSize(id, projected)
	f.Must(err)
	ended := second.Lease
	ended.Ended = "superseded"
	projected[kernel.KeyLease] = ended
	_, err = kernel.CheckMetadataSize(id, projected)
	assertMetadataSizeRefusal(t, err)
	// Only the ended field makes the complete close exceed the bound.
	f.Must(f.Store.WriteMetadata(id, map[string]any{"pad": pad}))
	before := actorRecord(t, f, id)
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: first.Token, Record: recordFile(t, data)})
	assertMetadataSizeRefusal(t, err)
	after := actorRecord(t, f, id)
	if _, has := after.Metadata[kernel.KeyClosing]; has {
		t.Error("oversized superseded lease stored mindspec_closing")
	}
	if !reflect.DeepEqual(before, after) {
		t.Error("oversized superseded lease changed stored metadata or status")
	}
}

func assertMetadataSizeRefusal(t *testing.T, err error) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), "metadata-too-large") {
		t.Fatalf("expected metadata-too-large refusal: %v", err)
	}
}

func TestS134Submit_RepairEventAndReceiptTogether(t *testing.T) {
	f, id, _, subject := slotFixture(t)
	data := verdictBytes(subject, "R1")
	actorCommit(t, f, "verdict.json", data, "")
	f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyRecord, data)))
	changeActorMetadata(t, f, id, "closed")
	before := actorRecord(t, f, id)
	run := f.Store.Run
	receiptError := errors.New("receipt store failed")
	failed := false
	f.Store.Run = func(args ...string) ([]byte, error) {
		if len(args) > 3 && args[0] == "update" && args[1] == before.ID && args[2] == "--metadata" && hasReceiptMetadata(t, args[3]) {
			failed = true
			return nil, receiptError
		}
		return run(args...)
	}
	_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Repair: true, Reason: "recover", Artifact: "verdict.json"})
	f.Store.Run = run
	if !failed || !errors.Is(err, receiptError) {
		t.Fatalf("receipt write failure did not fire: failed=%v err=%v", failed, err)
	}
	after := actorRecord(t, f, id)
	if !reflect.DeepEqual(before.Metadata[kernel.KeyTakes], after.Metadata[kernel.KeyTakes]) {
		t.Error("failed receipt stored a repair event")
	}
	if !reflect.DeepEqual(before, after) {
		t.Error("failed repair receipt changed the step")
	}
}

// A halt first stores an answer, then adds halt_answer to its final receipt.
func TestS134Submit_HaltFinalMetadataBound(t *testing.T) {
	f, _, _ := actorFixture(t)
	id := kerneltest.OpenDecision(f, f.Items[0]+".review", "reject", 1, "halt")
	answer := kernel.DecisionAnswer{Option: "halt", At: f.State().At}
	projected := actorRecord(t, f, id).Metadata
	projected[kernel.KeyAnswer] = answer
	projected[kernel.KeyClose] = kernel.Receipt{Kind: "passed", Answer: &answer}
	projected["pad"] = ""
	raw, err := json.Marshal(projected)
	f.Must(err)
	pad := strings.Repeat("p", kernel.MetadataLimit-len(raw))
	projected["pad"] = pad
	_, err = kernel.CheckMetadataSize(id, projected)
	f.Must(err)
	projected[kernel.KeyClose] = kernel.Receipt{Kind: "halted", HaltAnswer: id, Answer: &answer}
	_, err = kernel.CheckMetadataSize(id, projected)
	assertMetadataSizeRefusal(t, err)
	f.Must(f.Store.WriteMetadata(id, map[string]any{"pad": pad}))
	before := actorRecord(t, f, id)
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Answer: "halt"})
	assertMetadataSizeRefusal(t, err)
	if !reflect.DeepEqual(before, actorRecord(t, f, id)) {
		t.Fatal("oversized halt receipt stored an answer or changed the step")
	}
}
