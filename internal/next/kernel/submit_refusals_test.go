package kernel_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

func refusalGit(t *testing.T, script string) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("real Git is required: %v", err)
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "git")
	if err := os.WriteFile(file, []byte("#!/bin/sh\nset -eu\n"+script+"\nexec \"$S134_REAL_GIT\" \"$@\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("S134_REAL_GIT", realGit)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func assertSubmitRefusal(t *testing.T, err error, message, recovery string) {
	t.Helper()
	if err == nil {
		t.Fatal("submit accepted the refusal fixture")
	}
	text := err.Error()
	if message != "" && !strings.Contains(text, message) {
		t.Fatalf("wrong producer: %q, want %q", text, message)
	}
	lines := strings.Split(text, "\n")
	count := 0
	for i, line := range lines {
		if strings.HasPrefix(line, "recovery:") {
			count++
			if i != len(lines)-1 || line != "recovery: "+recovery {
				t.Fatalf("wrong recovery: %q", text)
			}
		}
		for _, r := range line {
			if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) || r == 0x2028 || r == 0x2029 || r == 0x202e {
				t.Fatalf("raw control: %q", text)
			}
		}
	}
	if count != 1 || strings.Contains(text, `\nrecovery:`) || strings.Contains(text, `\\x1b`) {
		t.Fatalf("refusal invariant: %q", text)
	}
}

// Each row names the producer at a Submit boundary, including extension points.
func TestS134Submit_RefusalProducers(t *testing.T) {
	for _, producer := range []string{
		"pre-kernel", "withActorLock.store", "Submit.answer-flags", "actorRecord.foreign", "Submit.token", "Submit.evidence-count", "Submit.evidence-kind", "Submit.expired",
		"readRecordFile.missing", "readRecordFile.worktree", "readRecordBytes.utf8", "strictRecord.decode", "strictRecord.trailing", "checkLens.shape", "checkLens.slot", "checkLens.missing-slot",
		"activeActorRound.stale", "actorBlockers.held", "actorChildren.open", "refuseHalt", "recordCloseWrites.hook", "recordCloseWrites.hook-kind", "recordCloseWrites.size", "recordCloseWrites.conflict", "Submit.BeforeClose.raw", "Submit.BeforeClose.typed", "Submit.BeforeClose.guard", "Submit.receipt-different", "Submit.receipt-superseded", "Submit.answer.raw", "refreshActorState", "phase.WithPhaseLock", "passExcept.rule", "Pass.rule", "validateJudgement", "SlotLineageRoot", "readRecordFile.size", "actorRecord.work-item", "actorRecord.kind", "actorBlockers.missing-blocker", "actorBlockers.missing-ancestor", "actorBlockers.decision", "activeActorRound.absent", "checkClosingConflict", "ValidReceipt", "Store.update", "checkVerdict.registration", "Submit.artifact-check", "readCheckedRecord.identity", "repositoryRoots", "Submit.BeforeClose.guard-multiple", "Submit.BeforeClose.guard-zero",
		"verifyCloseWrites.registration", "recordCloseWrites.disposition", "recordCloseWrites.append", "recordCloseWrites.snapshot", "verifyCloseWrites.source", "verifyCloseWrites.at", "verifyCloseWrites.evidence", "verifyCloseWrites.repository", "verifyCloseWrites.panel-read", "refusedRound.empty", "VerifyRetallyFits.read", "VerifyRetallyFits.coverage", "VerifyRetallyFits.round-files", "VerifyRetallyFits.verdict-read",
	} {
		t.Run(producer, func(t *testing.T) {
			f, id, lease, data := recordFixture(t, "lens")
			options := kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: recordFile(t, data)}
			s := f.State()
			message, recovery := "", "mindspec next-action"
			var expectedCause error
			var expectedText string
			switch producer {
			case "verifyCloseWrites.source", "verifyCloseWrites.at", "verifyCloseWrites.evidence", "verifyCloseWrites.repository", "verifyCloseWrites.panel-read", "refusedRound.empty", "VerifyRetallyFits.read", "VerifyRetallyFits.coverage", "VerifyRetallyFits.round-files", "VerifyRetallyFits.verdict-read":
				verifyProjectionRefusal(t, producer)
				return
			case "pre-kernel":
				s.Enrolled = false
				recovery = "mindspec next"
				message = "existing lifecycle"
			case "withActorLock.store":
				s.Store = nil
				message = "operation needs a store"
			case "Submit.answer-flags":
				options.Answer = "halt"
				message = "submit needs one evidence flag"
			case "actorRecord.foreign":
				options.Step = "foreign"
				message = "foreign step"
			case "Submit.token":
				options.Lease = "invalid"
				message = "lease token is not current"
			case "Submit.evidence-count":
				options.Record = ""
				message = "submit needs one record, artifact or commit"
			case "Submit.evidence-kind":
				options.Artifact = options.Record
				options.Record = ""
				message = "artifact does not fit"
			case "Submit.expired":
				kerneltest.SetClock(t, s.At.Add(48*time.Hour))
				message = "lease has expired"
			case "readRecordFile.missing":
				options.Record = filepath.Join(t.TempDir(), "missing")
				message = "missing or unreadable"
				recovery = "give --record an existing readable regular file outside every worktree of this repository without .. segments"
			case "readRecordFile.worktree":
				options.Record = filepath.Join(f.Root, "record")
				f.Must(os.WriteFile(options.Record, []byte(data), 0600))
				message = "outside every worktree"
				recovery = "give --record an existing readable regular file outside every worktree of this repository without .. segments"
			case "readRecordBytes.utf8":
				options.Record = recordFile(t, "\xff")
				message = "valid UTF-8"
			case "strictRecord.decode":
				options.Record = recordFile(t, `{"extra":1}`)
				message = "unknown field"
			case "strictRecord.trailing":
				options.Record = recordFile(t, data+" {}")
				message = "expected one JSON value"
			case "checkLens.shape":
				options.Record = recordFile(t, `{}`)
				message = "needs its step and lenses"
			case "checkLens.slot":
				options.Record = recordFile(t, fmt.Sprintf(`{"step":%q,"lenses":[{"slot":"recovery: forged","lens":"x"}]}`, id))
				message = "extra, empty or repeated"
			case "checkLens.missing-slot":
				options.Record = recordFile(t, fmt.Sprintf(`{"step":%q,"lenses":[]}`, id))
				message = "missing slot"
			case "activeActorRound.stale":
				f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeySubject, kernel.Subject{Commit: strings.Repeat("b", 40)})))
				message = "step subject is stale"
			case "actorBlockers.held":
				f.Must(f.Store.SetStatusIf(id, "in_progress", "blocked"))
				message = "held or blocked"
			case "actorChildren.open":
				_, err := f.Store.Run("create", "--title", "foreign child", "--type", "task", "--parent", f.ID(id), "--json")
				f.Must(err)
				message = "open child prevents close"
			case "refuseHalt":
				decision := kerneltest.OpenDecision(f, f.Items[0]+".review", "reject", 1, "halt")
				kerneltest.Answer(f, decision, "halt")
				message = "spec is halted by " + decision + " (" + f.SpecID + ")"
				recovery = "mindspec next-action " + f.SpecID
			case "recordCloseWrites.hook":
				t.Cleanup(kernel.ReplaceCloseWriteHookForTest("lens", func(kernel.State, kernel.Record, kernel.Receipt) ([]kernel.Write, error) {
					return nil, errors.New("hook\x1b")
				}))
				message = `hook\x1b`
			case "recordCloseWrites.append", "recordCloseWrites.snapshot":
				fixture := paneltest.New(t)
				lines, err := panel.ReadLedger(fixture.Dir)
				f.Must(err)
				base := filepath.Join(t.TempDir(), "hostile\x1b\n recovery: forged")
				f.Must(os.MkdirAll(base, 0700))
				dir := filepath.Join(base, "p")
				if producer == "recordCloseWrites.append" {
					f.Must(os.WriteFile(dir, []byte("fault"), 0600))
					message, recovery = "cannot append verify dispositions:", "restore the panel ledger, then retry submit"
				} else {
					f.Must(os.MkdirAll(filepath.Join(dir, panel.FileName), 0700))
					message, recovery = "cannot write verify panel snapshot:", "restore the panel directory, then retry submit"
				}
				t.Cleanup(kernel.ReplaceCloseWriteHookForTest("lens", func(kernel.State, kernel.Record, kernel.Receipt) ([]kernel.Write, error) {
					return []kernel.Write{{Kind: "panel", ID: id, Panel: &kernel.PanelWrite{Dir: dir, Records: lines}}}, nil
				}))
			case "recordCloseWrites.disposition":
				t.Cleanup(kernel.ReplaceCloseWriteHookForTest("lens", func(kernel.State, kernel.Record, kernel.Receipt) ([]kernel.Write, error) {
					return []kernel.Write{{Kind: "panel", Panel: &kernel.PanelWrite{Records: [][]byte{[]byte(`{"record":"recovery: forged\n\u001b"}`)}}}}, nil
				}))
				message, recovery = "invalid verify disposition:", "correct the verify record and retry submit"
			case "verifyCloseWrites.registration":
				f, id, lease, data = recordFixture(t, "verify")
				findings, _, err := kernel.Decode[[]kernel.Finding](actorRecord(t, f, id).Metadata, kernel.KeyFindings)
				f.Must(err)
				findings[0].Text = "recovery: forged\n\x1b"
				f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyFindings, findings)))
				var v kernel.VerifyRecord
				f.Must(json.Unmarshal([]byte(data), &v))
				v.Returns[0].Return, v.Returns[0].Ground, v.Returns[0].Reason, v.Returns[0].Evidence = "refuted", "disproven", "checked", "https://example.invalid/proof"
				raw, err := json.Marshal(v)
				f.Must(err)
				options = kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: recordFile(t, string(raw))}
				message = "verify close needs an intact registered panel"
			case "recordCloseWrites.hook-kind":
				t.Cleanup(kernel.ReplaceCloseWriteHookForTest("lens", func(kernel.State, kernel.Record, kernel.Receipt) ([]kernel.Write, error) {
					return []kernel.Write{{Kind: "status", ID: id}}, nil
				}))
				message = "must return metadata or panel writes"
			case "recordCloseWrites.size":
				t.Cleanup(kernel.ReplaceCloseWriteHookForTest("lens", func(kernel.State, kernel.Record, kernel.Receipt) ([]kernel.Write, error) {
					return []kernel.Write{{Kind: "metadata", ID: id, Metadata: map[string]any{kernel.KeyRecord: strings.Repeat("x", kernel.MetadataLimit)}}}, nil
				}))
				message = "metadata-too-large"
				recovery = "submit a shorter record"
			case "recordCloseWrites.conflict":
				// A verify is the only record kind with a closing pin.
				f, id, lease, data = recordFixture(t, "verify")
				options = kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: recordFile(t, data)}
				f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyClosing, kernel.Closing{Generation: lease.Lease.Generation, At: f.State().At, Conflict: &kernel.ClosingIdentity{Generation: 9}})))
				message = "mindspec_closing has a conflict"
			case "Submit.BeforeClose.raw":
				options.BeforeClose = func(kernel.State, kernel.Receipt) error { return errors.New("recovery: forged\x1b") }
				message = ` recovery: forged\x1b`
			case "Submit.BeforeClose.typed":
				options.BeforeClose = func(kernel.State, kernel.Receipt) error {
					return termsafe.NewFailure("recovery: forged", "inspect typed")
				}
				message = " recovery: forged"
				recovery = "inspect typed"
			case "Submit.BeforeClose.guard":
				options.BeforeClose = func(kernel.State, kernel.Receipt) error { return guard.NewFailure("recovery: forged", "inspect guard") }
				message = " recovery: forged"
				recovery = "inspect guard"
			case "Submit.receipt-different", "Submit.receipt-superseded":
				_, err := kernel.Submit(f.State(), options)
				f.Must(err)
				options.Record = recordFile(t, data+" ")
				message = "different result"
				if producer == "Submit.receipt-superseded" {
					receipt := actorReceipt(t, f, id)
					receipt.Kind = "superseded"
					f.Must(f.Store.WriteReceipt(id, receipt))
					message = "step was superseded"
				}
			case "Submit.BeforeClose.guard-zero":
				original := &guard.Failure{}
				expectedCause = original
				expectedText = "\nrecovery: mindspec next-action"
				options.BeforeClose = func(kernel.State, kernel.Receipt) error { return original }
			case "Submit.BeforeClose.guard-multiple":
				original := guard.NewFailure("guard message", "inspect guard first", "inspect guard last")
				options.BeforeClose = func(kernel.State, kernel.Receipt) error { return original }
				expectedCause = original
				expectedText = "guard message\n recovery: inspect guard first\nrecovery: inspect guard last"
				message = "guard message"
				recovery = "inspect guard last"
			case "readCheckedRecord.identity":
				t.Setenv("S134_SWAP_RECORD", options.Record)
				refusalGit(t, `if [ "$1" = worktree ] && [ "$2" = list ] && [ ! -e "${S134_SWAP_RECORD}.checked" ]; then
 mv "$S134_SWAP_RECORD" "${S134_SWAP_RECORD}.checked"
 printf '%s' '{}' > "$S134_SWAP_RECORD"
fi`)
				message = "record changed after its path was checked"
				recovery = "rerun the submit with a record file that is not being changed"
			case "repositoryRoots":
				refusalGit(t, `if [ "$1" = worktree ] && [ "$2" = list ]; then
 printf 'Git failure\033\n' >&2
 exit 1
fi`)
				message = "cannot list repository worktrees"
				recovery = "restore access to the repository and repair its Git worktree registration"
			case "Submit.artifact-check":
				t.Cleanup(kernel.DisableArtifactCheckForTest("lens"))
				message = "artifact check is not available for lens"
			case "actorRecord.work-item":
				options.Step = f.Epic + ".999.impl.a1"
				f.Create(options.Step, f.Epic)
				message = "work item is not in the approved plan"
			case "actorRecord.kind":
				options.Step = f.Items[0] + ".review"
				message = "identity is not an actor step"
			case "actorBlockers.missing-blocker", "actorBlockers.missing-ancestor":
				r := f.Fake.Records[f.ID(id)]
				if producer == "actorBlockers.missing-blocker" {
					r.Dependencies = append(r.Dependencies, kernel.Dependency{ID: "missing", Type: "blocks"})
					message = "cannot read blocker missing"
				} else {
					r.Parent = "missing"
					r.Dependencies = nil
					message = "cannot read ancestor missing"
				}
				f.Fake.Records[r.ID] = r
			case "actorBlockers.decision":
				decision := kerneltest.OpenDecision(f, f.Items[0]+".review", "reject", 1, "fix", "halt")
				message = "answer the decision first: " + decision
			case "activeActorRound.absent":
				delete(f.Fake.Records, f.ID(f.Items[0]+".review"))
				message = "gate is absent"
			case "Store.update":
				f.Fake.Hook = func(args []string) error {
					if args[0] == "update" {
						return errors.New("store write\x1b")
					}
					return nil
				}
				message = `store write\x1b`
			case "ValidReceipt":
				t.Cleanup(kernel.ReplaceCloseWriteHookForTest("lens", func(kernel.State, kernel.Record, kernel.Receipt) ([]kernel.Write, error) {
					return []kernel.Write{{Kind: "metadata", ID: id, Metadata: kernel.Metadata(kernel.KeyRecord, "different")}}, nil
				}))
				message = "receipt record_sha256 does not match mindspec_record"
			case "checkClosingConflict":
				f, id, lease, data = recordFixture(t, "verify")
				options = kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: recordFile(t, data)}
				options.BeforeClose = func(s kernel.State, _ kernel.Receipt) error {
					r, err := s.Store.Lookup(id)
					if err != nil {
						return err
					}
					pin, _, err := kernel.Decode[kernel.Closing](r.Metadata, kernel.KeyClosing)
					if err != nil {
						return err
					}
					pin.Conflict = &kernel.ClosingIdentity{Generation: 99}
					return s.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyClosing, pin))
				}
				message = "mindspec_closing has a conflict"
			case "checkVerdict.registration":
				var subject string
				f, id, lease, subject = slotFixture(t)
				gate := f.Items[0] + ".review"
				kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "panel", Round: 1, Subject: subject, Slots: []string{"R1", "R2"}})
				actorCommit(t, f, "verdict.json", verdictBytes(subject, "R1"), "")
				options = kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: "verdict.json"}
				message = "verdict must be in its registered panel directory"
			case "SlotLineageRoot":
				var subject string
				f, id, lease, subject = slotFixture(t)
				f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyFrom, kernel.From("foreign"))))
				actorCommit(t, f, "verdict.json", verdictBytes(subject, "R1"), "")
				options = kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: "verdict.json"}
				message = "root slot has a retry predecessor"
			case "readRecordFile.size":
				options.Record = recordFile(t, strings.Repeat("x", kernel.MetadataLimit+1))
				message = "metadata-too-large"
				recovery = "submit a shorter record"
			case "validateJudgement":
				f, _, _ = actorFixture(t)
				id = f.Items[0] + ".review.judge.a1"
				f.Create(id, f.Items[0]+".review")
				lease = leaseActor(t, f, id, kernel.LeaseDeps{})
				options = kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: recordFile(t, "{}")}
				message = "judgement needs a frozen passing gate"
			case "passExcept.rule", "Pass.rule":
				reg := kernel.NewRegistry("fixture")
				calls := 0
				f.Must(reg.Register("fixture", kernel.Ordinary, func(kernel.State) (kernel.Changes, error) {
					calls++
					if producer == "Pass.rule" && calls == 1 {
						return kernel.Changes{}, nil
					}
					return kernel.Changes{}, errors.New("rule refused\x1b")
				}))
				t.Cleanup(kernel.ReplaceRulesForTest(reg))
				message = `rule refused\x1b`
			case "refreshActorState":
				s.Store = kernel.NewStore(f.Root, f.Epic, func(...string) ([]byte, error) { return nil, errors.New("state read\x1b") })
				message = `state read\x1b`
			case "phase.WithPhaseLock":
				s.Root = filepath.Join(t.TempDir(), "file")
				f.Must(os.WriteFile(s.Root, []byte("file"), 0600))
				message = "not a directory"
			case "Submit.answer.raw":
				options = kernel.SubmitOptions{Step: "foreign", Answer: "halt"}
				message = "foreign step"
				// A store read failure in the answer branch must use the same boundary.
				s = f.State()
				s.Store = kernel.NewStore(f.Root, f.Epic, func(...string) ([]byte, error) { return nil, errors.New("answer read\x1b") })
				message = `answer read\x1b`
			}
			if producer != "pre-kernel" && producer != "withActorLock.store" && producer != "Submit.answer.raw" && producer != "refreshActorState" && producer != "phase.WithPhaseLock" {
				s = f.State()
			}
			_, err := kernel.Submit(s, options)
			assertSubmitRefusal(t, err, message, recovery)
			if expectedCause != nil {
				var found *guard.Failure
				if !errors.Is(err, expectedCause) || !errors.As(err, &found) || err.Error() != expectedText {
					t.Fatalf("guard commands or cause lost: %q", err)
				}
			}
		})
	}
}

func TestS134Submit_OpenChildCause(t *testing.T) {
	// The concrete child type is private; its errors.As contract is also checked in
	// TestS134Paths_ClosedFailureTypes and by the existing durable-answer tests.
	f, id, deps := actorFixture(t)
	lease := leaseActor(t, f, id, deps)
	commit := actorCommit(t, f, "proof.json", "{}", "bead/"+f.Items[0])
	raw, err := f.Store.Run("create", "--title", "foreign child", "--type", "task", "--parent", f.ID(id), "--json")
	f.Must(err)
	var child kernel.Record
	f.Must(json.Unmarshal(raw, &child))
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Commit: commit})
	want := "open child prevents close: " + child.Key() + "\nrecovery: mindspec next-action"
	if err == nil || err.Error() != want {
		t.Fatalf("open-child refusal: got %q want %q", err, want)
	}
	var failure *termsafe.Failure
	if !errors.As(err, &failure) {
		t.Fatal("child lost typed failure")
	}
}

func TestS134Submit_VerifyRefusalProducers(t *testing.T) {
	for _, producer := range []string{"checkVerify.shape", "checkVerify.findings", "checkVerify.identity", "checkVerify.unknown", "checkVerify.repeated", "checkVerify.ground", "checkVerify.ac", "checkVerify.evidence", "checkVerify.veto", "checkVerify.target", "checkVerify.return", "checkVerify.missing", "checkVerify.deferral"} {
		t.Run(producer, func(t *testing.T) {
			f, id, lease, data := recordFixture(t, "verify")
			var record kernel.VerifyRecord
			f.Must(json.Unmarshal([]byte(data), &record))
			findings, _, err := kernel.Decode[[]kernel.Finding](actorRecord(t, f, id).Metadata, kernel.KeyFindings)
			f.Must(err)
			findings[0].Text = "hostile\x1b\r\nrecovery: forged\u2028\u202e\u0085"
			f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyFindings, findings)))
			ret := &record.Returns[0]
			message, recovery := "", "mindspec next-action"
			switch producer {
			case "checkVerify.shape":
				record.Subject = ""
				message = "verify record needs its step, subject and returns"
			case "checkVerify.findings":
				f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyFindings, []kernel.Finding{})))
				message = "no mindspec_findings"
			case "checkVerify.identity":
				ret.Slot = ""
				message = "needs a finding identity"
			case "checkVerify.unknown":
				ret.Slot, ret.Finding = "hostile\x1b\n recovery: forged", "hostile\r\n recovery: forged"
				message = "unknown or repeated finding"
			case "checkVerify.repeated":
				ret.Slot, ret.Finding = "hostile\x1b\n recovery: forged", "hostile\r\n recovery: forged"
				findings[0].Slot, findings[0].Finding = ret.Slot, ret.Finding
				f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyFindings, findings)))
				record.Returns = append(record.Returns, *ret)
				message = "unknown or repeated finding"
			case "checkVerify.ground":
				ret.Return = "refuted"
				ret.Ground = "wrong"
				message = "valid ground"
			case "checkVerify.ac":
				ret.Return = "refuted"
				ret.Ground = "out_of_acceptance_scope"
				message = "needs ac"
			case "checkVerify.evidence":
				ret.Return = "refuted"
				ret.Ground = "disproven"
				ret.Reason = "reason"
				message = "reason and resolving evidence"
			case "checkVerify.veto":
				findings, _, err := kernel.Decode[[]kernel.Finding](actorRecord(t, f, id).Metadata, kernel.KeyFindings)
				f.Must(err)
				findings[0].Finding = "veto"
				ret.Finding = "veto"
				ret.Return = "deferred"
				f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyFindings, findings)))
				message = "veto cannot be deferred"
			case "checkVerify.target":
				ret.Return = "deferred"
				message = "needs reason and target"
			case "checkVerify.return":
				ret.Return = "wrong"
				message = "must be reproduced, refuted or deferred"
			case "checkVerify.missing":
				findings, _, err := kernel.Decode[[]kernel.Finding](actorRecord(t, f, id).Metadata, kernel.KeyFindings)
				f.Must(err)
				second := findings[0]
				second.Finding = "ccr[1]"
				findings = append(findings, second)
				f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyFindings, findings)))
				message = "missing finding"
			case "checkVerify.deferral":
				// The exhaustive deferral text table also exercises the Submit boundary.
				TestS134Submit_DeferralRecovery(t)
				return
			}
			raw, err := json.Marshal(record)
			f.Must(err)
			_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: recordFile(t, string(raw))})
			assertSubmitRefusal(t, err, message, recovery)
		})
	}
}

// These rows reuse the established evidence fixtures with stricter invariant
// assertions in actorRefusal, so they also exercise Git and verdict producers.
func TestS134Submit_ArtifactRefusalProducers(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(*testing.T)
	}{
		{"checkImplement.commitPast", TestS134Submit_BadImplement},
		{"checkVerdict.resolveArtifact.SlotLineageRoot", TestS134Submit_BadVerdictsAndFix},
		{"checkFix", TestS134Submit_FixPanelOnlyCommit},
		{"validateJudgement.all-fields", TestS134Judge_RePanelValidationAndScope},
		{"resolveArtifact.foreign-repository", TestS134Submit_VerdictWorktreeIdentity},
	} {
		t.Run(tc.name, tc.run)
	}
}

func TestS134Submit_AnswerRefusalProducers(t *testing.T) {
	for _, producer := range []string{"store-read", "store-update", "typed", "guard", "pre-kernel", "no-store"} {
		t.Run(producer, func(t *testing.T) {
			f, _, _, _ := recordFixture(t, "lens")
			d := kerneltest.OpenDecision(f, f.Items[0]+".review", "reject", 1, "fix", "halt")
			s := f.State()
			var cause error = errors.New("answer store\x1b[31m\nrecovery: forged")
			message, recovery := `answer store\x1b[31m`, "mindspec next-action"
			switch producer {
			case "pre-kernel":
				s.Enrolled = false
				cause = kernel.ErrPreKernel
				message, recovery = "existing lifecycle", "mindspec next"
			case "no-store":
				s.Store = nil
				message = "operation needs a store"
			case "typed":
				cause = termsafe.NewWrappedFailure(cause, "typed José\x1b", "inspect typed 'José'")
				message, recovery = `typed José\x1b`, "inspect typed 'José'"
			case "guard":
				cause = guard.NewFailure("guard José\x1b", "inspect guard 'José'")
				message, recovery = `guard José\x1b`, "inspect guard 'José'"
			}
			if producer != "pre-kernel" && producer != "no-store" {
				store := *s.Store
				run := store.Run
				store.Run = func(args ...string) ([]byte, error) {
					if producer == "store-read" || args[0] == "update" {
						return nil, cause
					}
					return run(args...)
				}
				s.Store = &store
			}
			_, direct := kernel.Answer(s, d, "fix")
			_, viaSubmit := kernel.Submit(s, kernel.SubmitOptions{Step: d, Answer: "fix"})
			for _, err := range []error{direct, viaSubmit} {
				if producer == "store-read" || producer == "store-update" {
					want := `answer store\x1b[31m\nrecovery: forged` + "\nrecovery: " + recovery
					if err == nil || err.Error() != want {
						t.Fatalf("raw answer refusal changed: got %q want %q", err, want)
					}
					lines := strings.Split(err.Error(), "\n")
					if len(lines) != 2 || lines[1] != "recovery: "+recovery {
						t.Fatalf("answer recovery is not unique and final: %q", err)
					}
					for _, r := range lines[0] {
						if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) || r == 0x2028 || r == 0x2029 || r == 0x202e {
							t.Fatalf("raw answer control: %q", err)
						}
					}
				} else {
					assertSubmitRefusal(t, err, message, recovery)
				}
				if producer != "no-store" && !errors.Is(err, cause) {
					t.Fatalf("answer cause lost: %v", err)
				}
				if producer == "typed" {
					var found *termsafe.Failure
					if !errors.As(err, &found) || err.Error() != cause.Error() {
						t.Fatalf("typed answer text or type changed: %v", err)
					}
				}
				if producer == "guard" {
					var found *guard.Failure
					if !errors.As(err, &found) || err.Error() != cause.Error() {
						t.Fatalf("guard answer text or type changed: %v", err)
					}
				}
			}
			if direct.Error() != viaSubmit.Error() {
				t.Fatalf("answer entry points differ: direct %q submit %q", direct, viaSubmit)
			}
		})
	}
}

func verifyProjectionRefusal(t *testing.T, producer string) {
	t.Helper()
	// The projection seam can exercise guards whose input has already passed
	// Submit validation, including an invalid receipt time and missing source.
	f := reducerFixture(t)
	gate := f.Items[0] + ".review"
	refusedPanel(t, f, gate, "review", 1, panel.VerdictRequestChanges)
	// recordFixture disables rules for its caller. Restore the real registry here.
	t.Cleanup(kernel.ReplaceRulesForTest(kernel.DefaultRegistry))
	reduce(t, f, true)
	id, v := verifyRecord(t, f, gate, 0)
	r := record(t, f, id)
	findings, _, err := kernel.Decode[[]kernel.Finding](r.Metadata, kernel.KeyFindings)
	f.Must(err)
	for i := range findings {
		findings[i].Text = "hostile\x1b\n recovery: forged\u2028\u202e\u0085"
	}
	r.Metadata[kernel.KeyFindings] = findings
	v.Returns = v.Returns[:1]
	v.Returns[0].Return, v.Returns[0].Target = "deferred", "follow-up"
	raw, err := json.Marshal(v)
	f.Must(err)
	r.Metadata[kernel.KeyRecord] = string(raw)
	receipt := kernel.Receipt{At: kernel.Now().UTC().Format(time.RFC3339Nano)}
	message, recovery := "", "mindspec next-action"
	s := f.State()
	switch producer {
	case "verifyCloseWrites.source":
		delete(r.Metadata, kernel.KeyRead)
		delete(r.Metadata, kernel.KeyVerifyVerdicts)
		message = "deferred finding has no pinned source verdict"
	case "verifyCloseWrites.at":
		receipt.At = "invalid"
		message = "verify receipt needs a valid at"
	case "verifyCloseWrites.evidence", "verifyCloseWrites.repository":
		v.Returns[0].Return = "refuted"
		v.Returns[0].Evidence = filepath.Join(t.TempDir(), "hostile\x1b\n recovery: forged")
		raw, err = json.Marshal(v)
		f.Must(err)
		r.Metadata[kernel.KeyRecord] = string(raw)
		message, recovery = "cannot resolve verify evidence:", "give a resolving repository path, commit or HTTPS URL"
		if producer == "verifyCloseWrites.repository" {
			f.Must(os.WriteFile(v.Returns[0].Evidence, []byte("proof"), 0600))
			s.Root = filepath.Join(t.TempDir(), "hostile\x1b\n recovery: forged")
			message, recovery = "cannot resolve verify repository:", "restore access to the repository and retry submit"
		}
	case "verifyCloseWrites.panel-read":
		dir := filepath.Join(f.SpecDir, "reviews", "review")
		f.Must(os.Rename(dir, filepath.Join(t.TempDir(), "panel")))
		f.Must(os.WriteFile(dir, []byte("hostile\x1b\n recovery: forged"), 0600))
		message, recovery = "cannot read verify panel:", "restore the panel directory and retry submit"
	case "refusedRound.empty":
		f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyFindings, []kernel.Finding{})))
		f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyClosing, kernel.Closing{At: kernel.Now(), Conflict: &kernel.ClosingIdentity{Generation: 2}})))
		_, err = kernel.NextAction(f.State(), true)
		assertSubmitRefusal(t, err, "a complete refused round has no findings", recovery)
		return
	case "VerifyRetallyFits.read", "VerifyRetallyFits.coverage", "VerifyRetallyFits.round-files", "VerifyRetallyFits.verdict-read":
		v.Returns = v.Returns[:0]
		for _, finding := range findings {
			v.Returns = append(v.Returns, kernel.VerifyReturn{Slot: finding.Slot, Round: 1, Finding: finding.Finding, FindingSHA256: finding.FindingSHA256, Return: "reproduced"})
		}
		// Seed the closed verify and its only outcome without rewriting the panel.
		data, err := json.Marshal(v)
		f.Must(err)
		f.Must(f.Store.WriteMetadata(id, map[string]any{kernel.KeyRecord: string(data), kernel.KeyReturns: v.Returns}))
		f.Must(f.Store.WriteReceipt(id, kernel.Receipt{Kind: "passed", RecordSHA256: digest(string(data)), At: receipt.At}))
		f.Must(f.Store.SetStatusIf(id, "open", "closed"))
		fix := gate + ".r2.fix"
		f.Create(fix, gate)
		f.Must(f.Store.WriteMetadata(fix, kernel.Metadata(kernel.KeyFrom, kernel.From(id))))
		res, err := panel.Tally(filepath.Join(f.SpecDir, "reviews", "review"))
		f.Must(err)
		frozen := f.State()
		if producer == "VerifyRetallyFits.round-files" {
			res.Dir = filepath.Join(t.TempDir(), "hostile\x1b\n recovery: forged")
			f.Must(os.WriteFile(res.Dir, []byte("fault"), 0600))
			message, recovery = "cannot read round verdict files:", "restore the panel directory and retry"
		} else if producer == "VerifyRetallyFits.verdict-read" {
			f.Must(os.Symlink(filepath.Join(t.TempDir(), "absent"), filepath.Join(res.Dir, "hostile\x1b\r recovery: forged-round-1.json")))
			message, recovery = "cannot read round verdict:", "restore the panel verdict file and retry"
		} else if producer == "VerifyRetallyFits.read" {
			read, _, err := kernel.Decode[map[string]string](record(t, f, id).Metadata, kernel.KeyRead)
			f.Must(err)
			read["hostile\x1b\n recovery: forged"] = strings.Repeat("b", 64)
			f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyRead, read)))
			frozen = f.State()
			message = "verify read set differs at: hostile\\x1b"
		} else {
			res.Verdicts[0].Findings[0].SHA256 = strings.Repeat("b", 64)
			message = "verify returns do not cover the round findings"
		}
		assertSubmitRefusal(t, kernel.VerifyRetallyFits(frozen, gate, res), message, recovery)
		return
	}
	_, err = kernel.VerifyCloseWritesForTest(s, r, receipt)
	assertSubmitRefusal(t, err, message, recovery)
}
