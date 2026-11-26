package kernel_test

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

// changeActorMetadata models a crash or a raw close without a lifecycle command.
func changeActorMetadata(t *testing.T, f *kerneltest.Fixture, id string, status string, remove ...string) {
	t.Helper()
	r := actorRecord(t, f, id)
	for _, key := range remove {
		delete(r.Metadata, key)
	}
	if status != "" {
		r.Status = status
	}
	f.Fake.Records[r.ID] = r
}

func seededRecordActor(t *testing.T, kind string) (*kerneltest.Fixture, string, string) {
	t.Helper()
	f := reducerFixture(t)
	f.Create(f.Items[0]+".impl.a1", f.Items[0])
	gate := f.Items[0] + ".review"
	id := gate + ".r1." + kind
	subject := reducerGit(t, f, "rev-parse", "HEAD")
	reducerGit(t, f, "update-ref", "refs/heads/bead/"+f.Items[0], subject)
	put(t, filepath.Join(f.Root, ".mindspec", "config.yaml"), "panel:\n  reviewers:\n    - family: harness\n      count: 2\n")
	round := kernel.GateRound{Round: 1, Subject: subject, SlotCount: 2, Slots: []string{gate + ".r1.R1", gate + ".r1.R2"}}
	if kind == "verify" {
		round.Verify = id
	} else {
		round.Lens = id
	}
	kerneltest.Authorise(f, gate, round)
	if kind == "verify" {
		finding := kernel.Finding{Slot: "R1", Round: 1, Finding: "veto", FindingSHA256: strings.Repeat("a", 64), Text: "hostile\nrecovery: forged\x1b"}
		v := kernel.VerifyRecord{Step: id, Subject: subject, Returns: []kernel.VerifyReturn{{Slot: "R1", Round: 1, Finding: "veto", FindingSHA256: finding.FindingSHA256, Return: "refuted", Ground: "disproven", Reason: "checked", Evidence: "https://example.invalid/proof"}}}
		kerneltest.Verify(f, gate, v, []kernel.Finding{finding}, map[string]string{"verdict.json": "digest"})
	} else {
		kerneltest.Lens(f, gate, kernel.LensRecord{Step: id, Lenses: []kernel.SlotLens{{Slot: "R1", Lens: "paths"}, {Slot: "R2", Lens: "tests"}}})
	}
	data, _, err := kernel.Decode[string](actorRecord(t, f, id).Metadata, kernel.KeyRecord)
	f.Must(err)
	return f, id, data
}

func useDefaultRules(t *testing.T) {
	t.Helper()
	kerneltest.WithRules(t, kernel.DefaultRegistry)
}

func TestS134Submit_LateGeneration(t *testing.T) {
	for _, kind := range []string{"slot", "verify"} {
		t.Run(kind, func(t *testing.T) {
			var f *kerneltest.Fixture
			var id, artifact, data string
			var first kernel.LeaseResult
			if kind == "slot" {
				var subject string
				f, id, first, subject = slotFixture(t)
				actorCommit(t, f, "verdict.json", verdictBytes(subject, "R1"), "")
				artifact = "verdict.json"
			} else {
				f, id, data = seededRecordActor(t, kind)
				changeActorMetadata(t, f, id, "open", kernel.KeyClose, kernel.KeyClosing, kernel.KeyReturns, kernel.KeyRecord)
				first = leaseActor(t, f, id, kernel.LeaseDeps{})
			}
			useDefaultRules(t)
			second, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Take: true, Reason: "replace"}, kernel.LeaseDeps{})
			f.Must(err)
			takes, _, err := kernel.Decode[[]kernel.TakeReason](actorRecord(t, f, id).Metadata, kernel.KeyTakes)
			f.Must(err)
			if len(takes) != 1 || takes[0].Via != "lease" || takes[0].Generation != 2 || takes[0].RevokedTokenHash != first.Lease.TokenHash || takes[0].Subject == "" {
				t.Fatalf("take event does not fence the revoked actor: %+v", takes)
			}
			o := kernel.SubmitOptions{Step: id, Lease: first.Token, Artifact: artifact}
			if kind == "verify" {
				o.Record = recordFile(t, data)
			}
			_, err = kernel.Submit(f.State(), o)
			f.Must(err)
			receipt := actorReceipt(t, f, id)
			lease, _, err := kernel.Decode[kernel.LeaseRecord](actorRecord(t, f, id).Metadata, kernel.KeyLease)
			f.Must(err)
			if receipt.Generation != 1 || receipt.SupersededGeneration != 2 || lease.Generation != 2 || lease.Ended != "superseded" {
				t.Fatalf("late receipt or lease is wrong: receipt=%+v lease=%+v", receipt, lease)
			}
			before := kerneltest.Snapshot(t, f)
			_, err = kernel.Submit(f.State(), o)
			f.Must(err)
			if before != kerneltest.Snapshot(t, f) {
				t.Fatal("accepted late receipt replay changed the step")
			}
			o.Lease = second.Token
			actorRefusal(t, f, func() error { _, err := kernel.Submit(f.State(), o); return err }, "different result")
			assertRetry(t, f, id, id+".a2", false)
		})
	}
}

func TestS134Submit_LateImplementAfterCommit(t *testing.T) {
	f, id, deps := actorFixture(t)
	first := leaseActor(t, f, id, deps)
	commit := actorCommit(t, f, "implementation.txt", "finished before the takeover", "bead/"+f.Items[0])
	second, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Take: true, Reason: "replace"}, deps)
	f.Must(err)
	if second.Lease.ClaimBase != first.Lease.ClaimBase {
		t.Fatalf("take changed the attempt's immutable claim base: old=%s new=%s", first.Lease.ClaimBase, second.Lease.ClaimBase)
	}
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: first.Token, Commit: commit})
	f.Must(err)
	receipt := actorReceipt(t, f, id)
	if receipt.Commit != commit || receipt.Generation != 1 || receipt.SupersededGeneration != 2 {
		t.Fatalf("late implementation was not accepted on its original subject: %+v", receipt)
	}
}

func TestS134Submit_RevokedCannotConflict(t *testing.T) {
	for _, mode := range []string{"receipt", "pin", "closed-without-receipt", "changed-subject"} {
		t.Run(mode, func(t *testing.T) {
			f, id, data := seededRecordActor(t, "verify")
			changeActorMetadata(t, f, id, "open", kernel.KeyClose, kernel.KeyClosing, kernel.KeyRecord, kernel.KeyReturns)
			first := leaseActor(t, f, id, kernel.LeaseDeps{})
			second, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Take: true, Reason: "replace"}, kernel.LeaseDeps{})
			f.Must(err)
			useDefaultRules(t)
			want := "mindspec_closing belongs to a newer generation"
			switch mode {
			case "receipt":
				_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: second.Token, Record: recordFile(t, data)})
				f.Must(err)
				want = "step already has a receipt from another generation"
			case "pin":
				f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyClosing, kernel.Closing{Generation: 2, RecordSHA256: digest(data), At: f.State().At})))
			case "closed-without-receipt":
				changeActorMetadata(t, f, id, "closed")
				want = "step is closed"
			case "changed-subject":
				f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeySubject, kernel.Subject{Commit: strings.Repeat("b", 40)})))
				want = "step subject changed since the take"
			}
			before := actorRecord(t, f, id)
			for _, candidate := range []string{data, strings.Replace(data, "checked", "different result", 1)} {
				actorRefusal(t, f, func() error {
					_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: first.Token, Record: recordFile(t, candidate)})
					return err
				}, want)
			}
			if !reflect.DeepEqual(before, actorRecord(t, f, id)) {
				t.Fatal("refused revoked token wrote a conflict or changed its receipt or pin")
			}
			if mode == "receipt" || mode == "pin" {
				reduce(t, f, true)
				assertRetry(t, f, id, id+".a2", false)
			}
		})
	}
}

func TestS134Submit_AbsentRevokedHashMatchesNoToken(t *testing.T) {
	for _, token := range []string{"foreign", "1." + strings.Repeat("a", 64), ""} {
		t.Run(fmt.Sprintf("token=%q", token), func(t *testing.T) {
			f, id, data := seededRecordActor(t, "verify")
			changeActorMetadata(t, f, id, "open", kernel.KeyClose, kernel.KeyClosing)
			leaseActor(t, f, id, kernel.LeaseDeps{})
			_, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Take: true, Reason: "replace"}, kernel.LeaseDeps{})
			f.Must(err)
			subject, _, err := kernel.Decode[kernel.Subject](actorRecord(t, f, id).Metadata, kernel.KeySubject)
			f.Must(err)
			f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyTakes, []kernel.TakeReason{{Via: "repair", Generation: 2, Subject: subject.Commit, Reason: "prior repair"}})))
			useDefaultRules(t)
			actorRefusal(t, f, func() error {
				_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: token, Record: recordFile(t, data)})
				return err
			}, "lease token is not current")
		})
	}
}

func TestS134Closes_RepairSlot(t *testing.T) {
	for _, mode := range []string{"fitting", "missing", "another-subject"} {
		t.Run(mode, func(t *testing.T) {
			f, id, lease, subject := slotFixture(t)
			put(t, filepath.Join(f.Root, ".beads", "config.yaml"), "types.custom: gate,step\n")
			gate := f.Items[0] + ".review"
			kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "review", Round: 1, Subject: subject, Slots: []string{"R1", "R2"}})
			// A crash after the record write leaves the stored verdict and no receipt.
			if mode != "missing" {
				verdictSubject := subject
				if mode == "another-subject" {
					verdictSubject = strings.Repeat("b", 40)
				}
				f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyRecord, verdictBytes(verdictSubject, "R1"))))
			}
			useDefaultRules(t)
			reduce(t, f, true)
			changeActorMetadata(t, f, id, "closed")
			before := kerneltest.Snapshot(t, f)
			out := reduce(t, f, false)
			if before != kerneltest.Snapshot(t, f) {
				t.Fatal("repair probe wrote")
			}
			argv := []string{"mindspec", "submit", id, "--repair", "--reason", "<text>"}
			fitting := mode == "fitting"
			if hasRun(out, argv...) != fitting {
				t.Fatalf("repair run entry differs: %+v", out.Run)
			}
			assertRetry(t, f, id, id+".a2", false)
			if !fitting {
				actorRefusal(t, f, func() error {
					_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Repair: true, Reason: "raw close"})
					return err
				}, map[string]string{"missing": "mindspec_record is absent or invalid", "another-subject": "verdict slot, round or subject"}[mode])
				reduce(t, f, true)
				assertRetry(t, f, id, id+".a2", true)
				return
			}
			_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Repair: true, Reason: "raw close"})
			f.Must(err)
			receipt := actorReceipt(t, f, id)
			r := actorRecord(t, f, id)
			current, _, err := kernel.Decode[kernel.LeaseRecord](r.Metadata, kernel.KeyLease)
			f.Must(err)
			events, _, err := kernel.Decode[[]kernel.TakeReason](r.Metadata, kernel.KeyTakes)
			f.Must(err)
			if !receipt.Repaired || receipt.Via != "repair" || receipt.Generation != 1 || !reflect.DeepEqual(current, lease.Lease) || len(events) != 1 || events[0].Via != "repair" || events[0].Generation != 1 || events[0].RevokedTokenHash != "" || events[0].Subject != subject || events[0].Reason != "raw close" || !events[0].At.Equal(f.State().At) {
				t.Fatalf("repair issued a generation or lost its event: receipt=%+v lease=%+v events=%+v", receipt, current, events)
			}
			raw, err := json.Marshal(r.Metadata[kernel.KeyTakes])
			f.Must(err)
			if strings.Contains(string(raw), "revoked_token_hash") {
				t.Fatal("repair event contains a revoked token hash field")
			}
			reduce(t, f, true)
			assertRetry(t, f, id, id+".a2", false)
		})
	}
}

func TestS134Submit_RepairRecordHooks(t *testing.T) {
	for _, kind := range []string{"lens", "verify"} {
		for _, tokened := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/tokened=%v", kind, tokened), func(t *testing.T) {
				f, id, data := seededRecordActor(t, kind)
				changeActorMetadata(t, f, id, "open", kernel.KeyClose, kernel.KeyClosing)
				lease := leaseActor(t, f, id, kernel.LeaseDeps{})
				changeActorMetadata(t, f, id, "closed")
				useDefaultRules(t)
				t.Cleanup(kernel.ReplaceCloseWriteHookForTest(kind, func(_ kernel.State, row kernel.Record, _ kernel.Receipt) ([]kernel.Write, error) {
					if _, present := row.Metadata[kernel.KeyClose]; present {
						t.Fatal("hook ran after receipt")
					}
					return []kernel.Write{{Kind: "metadata", ID: id, Metadata: kernel.Metadata("hook-ran", true)}}, nil
				}))
				if !tokened {
					out := reduce(t, f, true)
					if !hasRun(out, "mindspec", "submit", id, "--repair", "--reason", "<text>") {
						t.Fatalf("stored record repair run missing: %+v", out.Run)
					}
					actorRefusal(t, f, func() error {
						_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Repair: true, Reason: "crash", Record: recordFile(t, data+" ")})
						return err
					}, "repair --record differs from mindspec_record")
				}
				o := kernel.SubmitOptions{Step: id, Repair: !tokened, Reason: "crash"}
				if tokened {
					o.Lease, o.Record, o.Reason = lease.Token, recordFile(t, data), ""
				}
				_, err := kernel.Submit(f.State(), o)
				f.Must(err)
				r := actorRecord(t, f, id)
				if r.Metadata["hook-ran"] != true || r.Metadata[kernel.KeyRecord] != data || actorReceipt(t, f, id).RecordSHA256 != digest(data) || !actorReceipt(t, f, id).Repaired {
					t.Fatal("repair wrote only a receipt or did not use stored bytes")
				}
				assertRetry(t, f, id, id+".a2", false)
			})
		}
	}
}

func TestS134Closes_RetryKindContext(t *testing.T) {
	for _, kind := range []string{"lens", "verify", "fix"} {
		t.Run(kind, func(t *testing.T) {
			var f *kerneltest.Fixture
			var id string
			if kind == "fix" {
				f = reducerFixture(t)
				id = f.Items[0] + ".review.r2.fix"
				f.Create(id, f.Items[0]+".review")
				f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyReproduced, []kernel.Finding{{Slot: "R1", Finding: "veto", Text: "must survive"}})))
				changeActorMetadata(t, f, id, "closed")
			} else {
				f, id, _ = seededRecordActor(t, kind)
				changeActorMetadata(t, f, id, "closed", kernel.KeyClose, kernel.KeyRecord, kernel.KeyClosing, kernel.KeyReturns)
			}
			useDefaultRules(t)
			before := actorRecord(t, f, id)
			reduce(t, f, true)
			assertRetry(t, f, id, id+".a2", true)
			retry := actorRecord(t, f, id+".a2")
			for _, key := range []string{kernel.KeySubject, kernel.KeyFindings, kernel.KeyRead, kernel.KeyReproduced} {
				if !reflect.DeepEqual(before.Metadata[key], retry.Metadata[key]) {
					t.Fatalf("retry lost kind context %s", key)
				}
			}
			if kind != "fix" {
				round, _, err := kernel.Decode[kernel.GateRound](actorRecord(t, f, f.Items[0]+".review").Metadata, kernel.KeyRound)
				f.Must(err)
				pointer := round.Lens
				if kind == "verify" {
					pointer = round.Verify
				}
				if pointer != id+".a2" {
					t.Fatalf("round still names the raw-closed %s: %+v", kind, round)
				}
			}
			for _, key := range []string{kernel.KeyRecord, kernel.KeyLease, kernel.KeyClose, kernel.KeyClosing} {
				if _, exists := retry.Metadata[key]; exists {
					t.Fatalf("retry inherited %s", key)
				}
			}
		})
	}
}

func TestS134Closes_LargeVerifyRetryMetadataFile(t *testing.T) {
	f, id, _ := seededRecordActor(t, "verify")
	changeActorMetadata(t, f, id, "closed", kernel.KeyClose, kernel.KeyRecord, kernel.KeyClosing, kernel.KeyReturns)
	findings := []kernel.Finding{{Slot: "R1", Finding: "veto", Text: strings.Repeat("full text ", 20000)}}
	f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyFindings, findings)))
	useDefaultRules(t)
	calls := len(f.Fake.Calls)
	reduce(t, f, true)
	seen := false
	for _, args := range f.Fake.Calls[calls:] {
		if len(args) == 0 || args[0] != "create" {
			continue
		}
		for i, arg := range args {
			if arg == "--metadata" && i+1 < len(args) {
				seen = true
				if !strings.HasPrefix(args[i+1], "@") || !filepath.IsAbs(strings.TrimPrefix(args[i+1], "@")) {
					t.Fatalf("large retry did not use metadata file: %v", args)
				}
			}
		}
	}
	got, _, err := kernel.Decode[[]kernel.Finding](actorRecord(t, f, id+".a2").Metadata, kernel.KeyFindings)
	f.Must(err)
	raw, err := json.Marshal(got)
	f.Must(err)
	if !seen || len(raw) <= 128*1024 || !reflect.DeepEqual(got, findings) {
		t.Fatal("large retry cut or lost findings instead of creating with metadata file")
	}
}

func TestS134Submit_RepairStoredVerify(t *testing.T) {
	for _, supplied := range []bool{false, true} {
		t.Run(fmt.Sprintf("supplied=%v", supplied), func(t *testing.T) {
			f, id, data := seededRecordActor(t, "verify")
			changeActorMetadata(t, f, id, "open", kernel.KeyClose, kernel.KeyClosing, kernel.KeyReturns)
			lease := leaseActor(t, f, id, kernel.LeaseDeps{})
			// The submit stopped after storing its bytes. A raw close followed.
			changeActorMetadata(t, f, id, "closed")
			useDefaultRules(t)
			o := kernel.SubmitOptions{Step: id, Repair: true, Reason: "recover stored record"}
			if supplied {
				o.Record = recordFile(t, data)
			}
			_, err := kernel.Submit(f.State(), o)
			f.Must(err)
			r := actorRecord(t, f, id)
			returns, _, err := kernel.Decode[[]kernel.VerifyReturn](r.Metadata, kernel.KeyReturns)
			f.Must(err)
			var v kernel.VerifyRecord
			f.Must(json.Unmarshal([]byte(data), &v))
			receipt := actorReceipt(t, f, id)
			current, _, err := kernel.Decode[kernel.LeaseRecord](r.Metadata, kernel.KeyLease)
			f.Must(err)
			if !reflect.DeepEqual(returns, v.Returns) || r.Metadata[kernel.KeyRecord] != data || receipt.RecordSHA256 != digest(data) || receipt.Via != "repair" || current.Generation != lease.Lease.Generation {
				t.Fatalf("repair did not restore verify close writes from stored bytes: %+v", r)
			}
			gateReceipt := actorReceipt(t, f, f.Items[0]+".review")
			if gateReceipt.Kind != "passed" || gateReceipt.Retally != "verify" {
				t.Fatalf("repair did not pass the all-refuted verify: %+v", gateReceipt)
			}
			assertRetry(t, f, id, id+".a2", false)
		})
	}
}

func TestS134Closes_NeverLeasedVerify(t *testing.T) {
	f, id, _ := seededRecordActor(t, "verify")
	changeActorMetadata(t, f, id, "closed", kernel.KeyClose)
	useDefaultRules(t)
	actorRefusal(t, f, func() error {
		_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Repair: true, Reason: "raw close"})
		return err
	}, "step was never leased")
	out := reduce(t, f, true)
	if hasRun(out, "mindspec", "submit", id, "--repair", "--reason", "<text>") {
		t.Fatal("never-leased verify was offered a repair")
	}
	assertRetry(t, f, id, id+".a2", true)
}

func TestS134Submit_ClosedSlotRevokedDifferentResult(t *testing.T) {
	f, id, first, subject := slotFixture(t)
	second, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Take: true, Reason: "replace"}, kernel.LeaseDeps{})
	f.Must(err)
	actorCommit(t, f, "verdict.json", verdictBytes(subject, "R1"), "")
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: second.Token, Artifact: "verdict.json"})
	f.Must(err)
	before := actorRecord(t, f, id)
	actorCommit(t, f, "other-verdict.json", strings.Replace(verdictBytes(subject, "R1"), `"reviewed"`, `"different"`, 1), "")
	actorRefusal(t, f, func() error {
		_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: first.Token, Artifact: "other-verdict.json"})
		return err
	}, "step already has a receipt from another generation")
	if !reflect.DeepEqual(before, actorRecord(t, f, id)) {
		t.Fatal("refused different result changed the closed slot")
	}
	useDefaultRules(t)
	put(t, filepath.Join(f.Root, ".beads", "config.yaml"), "types.custom: gate,step\n")
	reduce(t, f, true)
	assertRetry(t, f, id, id+".a2", false)
}

func TestS134Closes_RetryRoundPointerResume(t *testing.T) {
	for _, kind := range []string{"lens", "verify"} {
		t.Run(kind, func(t *testing.T) {
			f, id, _ := seededRecordActor(t, kind)
			changeActorMetadata(t, f, id, "closed", kernel.KeyClose, kernel.KeyRecord, kernel.KeyClosing, kernel.KeyReturns)
			retry := f.Create(id+".a2", f.Items[0]+".review")
			f.Must(f.Store.WriteMetadata(retry.Key(), kernel.Metadata(kernel.KeyFrom, kernel.From(id))))
			useDefaultRules(t)
			reduce(t, f, true)
			round, _, err := kernel.Decode[kernel.GateRound](actorRecord(t, f, f.Items[0]+".review").Metadata, kernel.KeyRound)
			f.Must(err)
			pointer := round.Lens
			if kind == "verify" {
				pointer = round.Verify
			}
			if pointer != retry.Key() {
				t.Fatalf("retry create resumed without its round pointer: %+v", round)
			}
		})
	}
}

func TestS134Closes_RetryPointerRejectsForeign(t *testing.T) {
	f, id, _ := seededRecordActor(t, "verify")
	changeActorMetadata(t, f, id, "closed", kernel.KeyClose, kernel.KeyRecord, kernel.KeyClosing, kernel.KeyReturns)
	retry := f.Create(id+".a2", f.Items[0]+".review")
	f.Must(f.Store.WriteMetadata(retry.Key(), kernel.Metadata(kernel.KeyFrom, kernel.From(id))))
	changeActorMetadata(t, f, retry.Key(), "open", kernel.KeyTuple)
	useDefaultRules(t)
	before := actorRecord(t, f, retry.Key())
	_, err := kernel.Pass(kernel.Rules(), f.State(), true)
	f.Must(err)
	round, _, err := kernel.Decode[kernel.GateRound](actorRecord(t, f, f.Items[0]+".review").Metadata, kernel.KeyRound)
	f.Must(err)
	if round.Verify != id || !reflect.DeepEqual(before, actorRecord(t, f, retry.Key())) {
		t.Fatal("retry resume adopted or changed a foreign collision")
	}
}

func TestS134Closes_RepairCommit(t *testing.T) {
	for _, kind := range []string{"implement", "fix"} {
		t.Run(kind, func(t *testing.T) {
			f, id, deps := actorFixture(t)
			put(t, filepath.Join(f.Root, ".beads", "config.yaml"), "types.custom: gate,step\n")
			gate := f.Items[0] + ".review"
			if kind == "fix" {
				subject := actorCommand(t, f, "rev-parse", "HEAD")
				kerneltest.Authorise(f, gate, kernel.GateRound{Round: 1, Subject: subject, SlotCount: 2, Slots: []string{gate + ".r1.R1", gate + ".r1.R2"}})
				kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "review", Round: 1, Subject: subject, Slots: []string{"R1", "R2"}})
				finding := kernel.Finding{Slot: "R1", Round: 1, Finding: "ccr[0]", FindingSHA256: strings.Repeat("a", 64), Text: "fix this finding"}
				verify := gate + ".r1.verify"
				kerneltest.Verify(f, gate, kernel.VerifyRecord{Step: verify, Subject: subject, Returns: []kernel.VerifyReturn{{Slot: finding.Slot, Round: 1, Finding: finding.Finding, FindingSHA256: finding.FindingSHA256, Return: "reproduced"}}}, []kernel.Finding{finding}, map[string]string{})
				id = gate + ".r2.fix"
				f.Create(id, gate)
				f.Must(f.Store.WriteMetadata(id, map[string]any{kernel.KeySubject: kernel.Subject{Commit: subject}, kernel.KeyFrom: kernel.From(verify)}))
			}
			lease := leaseActor(t, f, id, deps)
			commit := actorCommit(t, f, "code.txt", "implementation or fix", "bead/"+f.Items[0])
			changeActorMetadata(t, f, id, "closed")
			useDefaultRules(t)
			out := reduce(t, f, true)
			if !hasRun(out, "mindspec", "submit", id, "--repair", "--reason", "<text>", "--commit", commit) {
				t.Fatalf("fitting code commit did not offer repair: %+v", out.Run)
			}
			assertRetry(t, f, id, strings.TrimSuffix(id, ".a1")+".a2", false)
			_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Repair: true, Reason: "raw close", Commit: commit})
			f.Must(err)
			receipt := actorReceipt(t, f, id)
			current, _, err := kernel.Decode[kernel.LeaseRecord](actorRecord(t, f, id).Metadata, kernel.KeyLease)
			f.Must(err)
			if receipt.Commit != commit || receipt.Via != "repair" || !receipt.Repaired || current.Generation != lease.Lease.Generation {
				t.Fatalf("code repair lost its evidence or generation: %+v", receipt)
			}
		})
	}
}
