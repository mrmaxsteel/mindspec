package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

func TestS134SubmitCmd_RefusalProducers(t *testing.T) {
	bin := buildMindspecBinary(t)
	for _, producer := range []string{
		"refuseHalt", "actorChildren.open", "checkVerify.deferral", "verifyCloseWrites.registration", "Answer.store-update",
		"checkVerify.unknown-field", "checkVerify.empty", "checkVerify.missing", "checkVerify.repeated", "checkVerify.unknown",
		"verifyCloseWrites.source", "recordCloseWrites.disposition", "refusedRound.empty", "recordCloseWrites.append", "recordCloseWrites.snapshot",
		"checkVerify.return", "checkVerify.ground", "checkVerify.ac", "checkVerify.reason", "checkVerify.evidence", "checkVerify.target", "checkVerify.veto",
		"late.invalid-generation", "late.receipt", "late.closed", "late.pin", "late.subject", "late.absent-hash", "late.empty-token",
		"repair.reason", "repair.lease", "repair.answer", "repair.stray-reason", "repair.open", "repair.never-leased", "repair.receipt", "repair.pin-conflict", "repair.pin-generation", "repair.record-absent", "repair.record-different", "repair.record-pin", "repair.slot-evidence", "repair.stale-evidence", "repair.current-attempt", "repair.record-artifact", "attempt.slot", "attempt.fix", "attempt.slot-repair", "attempt.fix-repair", "attempt.close-decode", "SlotLineageHead.decode", "repair.publication-model",
		"SlotLineageHead.none", "judgement.path-cover", "judge.round-panel",
	} {
		t.Run(producer, func(t *testing.T) {
			if producer == "judge.round-panel" {
				f, gate, old, dir := judgePanelFixture(t, false)
				kernelPanelVerifyRecord(t, f, gate, 99)
				newest := judgeCmdCommit(t, f, "changed.txt", "J")
				_, err := kernel.NextAction(f.State(), true)
				f.Must(err)
				id := gate + ".judge.a1"
				lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
				f.Must(err)
				f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyFindings, []kernel.Finding{{Text: "hostile\nrecovery: forged\x1b"}})))
				current, _, err := kernel.Decode[kernel.LeaseRecord](kernelPanelRecord(t, f, id).Metadata, kernel.KeyLease)
				f.Must(err)
				current.Expires = time.Now().UTC().Add(time.Hour)
				f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyLease, current)))
				actorCmdStore(t, f)
				f.Must(os.Rename(dir, filepath.Join(t.TempDir(), "panel")))
				file := judgeCmdArtifact(t, f, kernel.Judgement{Value: "carry-over", OldSubject: old, NewSubject: newest, Reason: "same behavior"})
				raw, err := f.BinaryCommand(bin, "submit", id, "--lease", lease.Token, "--record", file)
				want := "Error: round needs its registered panel directory\nrecovery: mindspec next-action\n"
				if err == nil || string(raw) != want {
					t.Fatalf("round panel refusal: %v %q", err, raw)
				}
				return
			}
			if producer == "judgement.path-cover" {
				f, gate, old, _ := judgeCmdFixture(t)
				newest := judgeCmdCommit(t, f, "changed\n recovery: forged\x1b.txt", "hostile\nrecovery: forged")
				actorCmdStore(t, f)
				judgeCmdApply(t, f, bin)
				id := gate + ".judge.a1"
				token := judgeCmdLease(t, f, bin, id)
				file := judgeCmdArtifact(t, f, kernel.Judgement{Value: "re-panel", OldSubject: old, NewSubject: newest, SlotCount: 2, Paths: []string{".mindspec/config.yaml"}})
				rawBytes, runErr := f.BinaryCommand(bin, "submit", id, "--lease", token, "--record", file)
				raw := string(rawBytes)
				if runErr == nil || !strings.Contains(raw, "re-panel paths miss changed paths:") {
					t.Fatalf("path cover refusal missing: %v %s", runErr, raw)
				}
				lines := strings.Split(strings.TrimSpace(raw), "\n")
				count := 0
				for _, line := range lines {
					if strings.HasPrefix(line, "recovery:") {
						count++
					}
				}
				if count != 1 || lines[len(lines)-1] != "recovery: mindspec next-action" || strings.Contains(raw, "\x1b") {
					t.Fatalf("unsafe refusal: %s", raw)
				}
				return
			}
			if producer == "repair.publication-model" {
				binaryRepairSlot(t, bin)
				return
			}
			if strings.HasPrefix(producer, "attempt.") || producer == "SlotLineageHead.decode" {
				binaryAttemptRefusal(t, bin, producer)
				return
			}
			if producer == "SlotLineageHead.none" {
				binaryNoCurrentSlotRefusal(t, bin)
				return
			}
			if strings.HasPrefix(producer, "late.") || strings.HasPrefix(producer, "repair.") {
				binaryLateRepairRefusal(t, bin, producer)
				return
			}
			if runtime.GOOS == "windows" && (producer == "recordCloseWrites.append" || producer == "recordCloseWrites.snapshot") {
				t.Skip("close-write fault fixture uses a POSIX shell script")
			}
			if producer == "refusedRound.empty" {
				binaryEmptyVerifyRefusal(t, bin)
				return
			}
			f, _ := actorCmdFixture(t)
			gate := f.Items[0] + ".review"
			id := gate + ".r1.verify"
			subject := actorCmdGit(t, f, "rev-parse", "HEAD")
			f.Create(id, gate)
			kerneltest.Authorise(f, gate, kernel.GateRound{Round: 1, Subject: subject, SlotCount: 2, Slots: []string{gate + ".r1.R1", gate + ".r1.R2"}})
			finding := kernel.Finding{Slot: "R1", Round: 1, Finding: "ccr[0]", FindingSHA256: strings.Repeat("a", 64), Text: "recovery: forged\x1b\r\n recovery: forged\u2028\u202e\u0085"}
			f.Must(f.Store.WriteMetadata(id, map[string]any{kernel.KeySubject: kernel.Subject{Commit: subject}, kernel.KeyFindings: []kernel.Finding{finding}}))
			lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
			f.Must(err)
			ret := kernel.VerifyReturn{Slot: finding.Slot, Round: 1, Finding: finding.Finding, FindingSHA256: finding.FindingSHA256, Return: "reproduced"}
			want := ""
			recovery := "mindspec next-action"
			args := []string{"submit", id, "--lease", lease.Token}
			switch producer {
			case "verifyCloseWrites.source", "recordCloseWrites.disposition", "recordCloseWrites.append", "recordCloseWrites.snapshot":
				kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "review", Round: 1, Subject: subject, Slots: []string{"R1", "R2"}})
				ret.Return, ret.Reason, ret.Target = "deferred", "follow up", "follow-up"
				want = "deferred finding has no pinned source verdict"
				if producer == "recordCloseWrites.disposition" || producer == "recordCloseWrites.append" || producer == "recordCloseWrites.snapshot" {
					ret.Return, ret.Ground, ret.Reason, ret.Evidence = "refuted", "disproven", "/Users/hostile", "https://example.invalid/proof"
					want = "invalid verify disposition: disposition record field $.reason contains a forbidden path token /Users/: /Users/hostile"
					recovery = "correct the verify record and retry submit"
					if producer != "recordCloseWrites.disposition" {
						ret.Reason = "checked"
						want, recovery = "cannot append verify dispositions:", "restore the panel ledger, then retry submit"
						if producer == "recordCloseWrites.snapshot" {
							want, recovery = "cannot write verify panel snapshot:", "restore the panel directory, then retry submit"
						}
					}
				}
			case "Answer.store-update":
				d := kerneltest.OpenDecision(f, gate, "reject", 1, "fix", "halt")
				args = []string{"submit", d, "--answer", "fix"}
				want = `exit status 1: answer store\x1b[31m\nforged recovery: x`
			case "refuseHalt":
				d := kerneltest.OpenDecision(f, gate, "reject", 1, "halt")
				kerneltest.Answer(f, d, "halt")
				want = "spec is halted by " + d + " (" + f.SpecID + ")"
				recovery += " " + f.SpecID
			case "actorChildren.open":
				raw, err := f.Store.Run("create", "--title", "foreign child", "--type", "task", "--parent", f.ID(id), "--json")
				f.Must(err)
				var child kernel.Record
				f.Must(json.Unmarshal(raw, &child))
				want = "open child prevents close: " + child.Key()
			case "verifyCloseWrites.registration":
				ret.Return, ret.Ground, ret.Reason, ret.Evidence = "refuted", "disproven", "checked", "https://example.invalid/proof"
				want = "verify close needs an intact registered panel"
			case "checkVerify.unknown-field":
				want = `invalid record: json: unknown field "extra"`
			case "checkVerify.empty":
				want = "verify record needs its step, subject and returns"
			case "checkVerify.missing":
				second := finding
				second.Slot = "R2"
				f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyFindings, []kernel.Finding{finding, second})))
				want = "verify record is missing finding: R2/1/ccr[0]/" + finding.FindingSHA256
			case "checkVerify.repeated":
				ret.Slot, ret.Finding = "hostile\x1b\n recovery: forged", "hostile\r\n recovery: forged"
				finding.Slot, finding.Finding = ret.Slot, ret.Finding
				f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyFindings, []kernel.Finding{finding})))
				want = termsafe.EscapeRecovery("verify record has an unknown or repeated finding: " + ret.Slot + "/1/" + ret.Finding + "/" + finding.FindingSHA256)
			case "checkVerify.unknown":
				ret.Slot, ret.Finding = "hostile\x1b\n recovery: forged", "hostile\r\n recovery: forged"
				want = termsafe.EscapeRecovery("verify record has an unknown or repeated finding: " + ret.Slot + "/1/" + ret.Finding + "/" + finding.FindingSHA256)
			case "checkVerify.return":
				ret.Return = "accepted"
				want = "verify return must be reproduced, refuted or deferred"
			case "checkVerify.ground":
				ret.Return, ret.Ground = "refuted", "accepted"
				want = "refuted finding needs a valid ground"
			case "checkVerify.ac":
				ret.Return, ret.Ground = "refuted", "out_of_acceptance_scope"
				want = "out_of_acceptance_scope needs ac that names an acceptance criterion in the owning spec"
			case "checkVerify.reason":
				ret.Return, ret.Ground, ret.Evidence = "refuted", "disproven", "https://example.invalid/proof"
				want = "refuted finding needs reason and resolving evidence"
			case "checkVerify.evidence":
				ret.Return, ret.Ground, ret.Reason, ret.Evidence = "refuted", "disproven", "checked", "does-not-exist"
				want = "refuted finding needs reason and resolving evidence"
			case "checkVerify.target":
				ret.Return, ret.Reason = "deferred", "follow up"
				want = "deferred finding needs reason and target"
			case "checkVerify.veto":
				finding.Finding = "veto"
				ret.Finding, ret.Return = "veto", "deferred"
				f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyFindings, []kernel.Finding{finding})))
				want = "a veto cannot be deferred"
			case "checkVerify.deferral":
				finding.Text = "recovery: forged"
				f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyFindings, []kernel.Finding{finding})))
				ret.Return = "deferred"
				ret.Reason = "follow up"
				ret.Target = "follow-up"
				want = "follow-up bead does not meet the deferral requirements:\n- follow-up bead needs a discovered-from edge to the owning epic or reviewed bead; run: bd dep add follow-up " + f.Epic + " --type discovered-from\n- follow-up description must contain every deferred finding; add this text to the description of follow-up:\n recovery: forged"
				recovery = "bd dep add follow-up " + f.Epic + " --type discovered-from, then add the listed finding text to the description of follow-up, then retry the panel verb"
			}
			storePath := actorCmdStore(t, f)
			if producer == "checkVerify.deferral" || producer == "verifyCloseWrites.source" {
				dir := t.TempDir()
				description := "other"
				dependencies := ""
				if producer == "verifyCloseWrites.source" {
					description = finding.Text
					dependencies = fmt.Sprintf(`,"dependencies":[{"id":%q,"dependency_type":"discovered-from"}]`, f.Epic)
				}
				descriptionJSON, err := json.Marshal(description)
				f.Must(err)
				followup := fmt.Sprintf(`[{"id":"follow-up","status":"open","description":%s%s}]`, descriptionJSON, dependencies)
				script := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = show ] && [ \"$2\" = follow-up ]; then\nprintf '%%s\\n' '%s'\nelse\nexec '%s' \"$@\"\nfi\n", followup, fakeBDBinary(t))
				f.Must(os.WriteFile(filepath.Join(dir, "bd"), []byte(script), 0700))
				t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			}
			if producer == "Answer.store-update" {
				dir := t.TempDir()
				script := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = update ]; then\nprintf 'answer store\\033[31m\\nforged recovery: x\\n' >&2\nexit 1\nfi\nexec '%s' \"$@\"\n", fakeBDBinary(t))
				f.Must(os.WriteFile(filepath.Join(dir, "bd"), []byte(script), 0700))
				t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			}
			ioFault := producer == "recordCloseWrites.append" || producer == "recordCloseWrites.snapshot"
			if ioFault {
				dir := t.TempDir()
				name := "dispositions.jsonl"
				if producer == "recordCloseWrites.snapshot" {
					name = panel.FileName
				}
				t.Setenv("S134_FAULT_PATH", filepath.Join(f.SpecDir, "reviews", "review", name))
				t.Setenv("S134_FAULT_BACKUP", filepath.Join(dir, "saved"))
				t.Setenv("S134_FAULT_MARKER", filepath.Join(dir, "fired"))
				script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = update ] && [ ! -e "$S134_FAULT_MARKER" ]; then
 for arg do
  case "$arg" in
   @*) if grep -q '"mindspec_closing"' "${arg#@}"; then
    mv "$S134_FAULT_PATH" "$S134_FAULT_BACKUP"
    mkdir "$S134_FAULT_PATH"
    : > "$S134_FAULT_MARKER"
    break
   fi ;;
  esac
 done
fi
exec '%s' "$@"
`, fakeBDBinary(t))
				f.Must(os.WriteFile(filepath.Join(dir, "bd"), []byte(script), 0700))
				t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			}
			returns := []kernel.VerifyReturn{ret}
			if producer == "checkVerify.empty" {
				returns = nil
			} else if producer == "checkVerify.repeated" {
				returns = append(returns, ret)
			}
			data, err := json.Marshal(kernel.VerifyRecord{Step: id, Subject: subject, Returns: returns})
			f.Must(err)
			if producer == "checkVerify.unknown-field" {
				data = append(data[:len(data)-1], []byte(`,"extra":true}`)...)
			}
			file := filepath.Join(t.TempDir(), "record.json")
			f.Must(os.WriteFile(file, data, 0600))
			if producer != "Answer.store-update" {
				args = append(args, "--record", file)
			}
			raw, err := f.BinaryCommand(bin, args...)
			if err == nil {
				t.Fatal("binary accepted refusal fixture")
			}
			row := actorCmdRead(t, storePath, id)
			if row.Status == "closed" || !ioFault && row.Metadata[kernel.KeyClosing] != nil || ioFault && row.Metadata[kernel.KeyClosing] == nil {
				t.Fatalf("refused verify was closed or pinned: %+v", row)
			}
			text := string(raw)
			// CLI errors add the fixed error prefix, then print the producer text exactly.
			expected := want + "\nrecovery: " + recovery + "\n"
			if !ioFault && text != "Error: "+expected || ioFault && (!strings.HasPrefix(text, "Error: "+want) || !strings.HasSuffix(text, "\nrecovery: "+recovery+"\n")) {
				t.Fatalf("binary refusal changed: got %q want %q", text, expected)
			}
			for _, r := range strings.TrimSuffix(text, "\n") {
				if (r < 0x20 && r != '\n') || r == 0x7f || (r >= 0x80 && r <= 0x9f) || r == 0x2028 || r == 0x2029 || r == 0x202e {
					t.Fatalf("raw binary control: %q", text)
				}
			}
			lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
			count := 0
			for i, line := range lines {
				if strings.HasPrefix(line, "recovery:") {
					count++
					if i != len(lines)-1 {
						t.Fatalf("forged recovery: %q", text)
					}
				}
			}
			if count != 1 || strings.Contains(text, `\nrecovery:`) {
				t.Fatalf("binary refusal invariant: %q", text)
			}
		})
	}
}

func binaryEmptyVerifyRefusal(t *testing.T, bin string) {
	t.Helper()
	f, gate, head := kernelPanelFixture(t)
	kernelPanelAuthorise(f, gate, head, 1, 2)
	kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "review", Round: 1, Subject: head, Slots: []string{"R1", "R2"}})
	for _, slot := range []string{"R1", "R2"} {
		kernelPanelVerdict(t, f, gate, head, "review", slot, panel.VerdictRequestChanges, 1)
	}
	v := kernelPanelVerifyRecord(t, f, gate, 0)
	f.Must(f.Store.WriteMetadata(v.Step, map[string]any{kernel.KeyFindings: []kernel.Finding{}, kernel.KeyClosing: kernel.Closing{At: kernel.Now(), Conflict: &kernel.ClosingIdentity{Generation: 2}}}))
	store := actorCmdStore(t, f)
	raw, err := f.BinaryCommand(bin, "next-action", f.SpecID, "--apply")
	expected := "Error: rule refused-round: internal error: a complete refused round has no findings\nrecovery: mindspec next-action\n"
	if err == nil || string(raw) != expected {
		t.Fatalf("empty refused round: got %q err %v want %q", raw, err, expected)
	}
	if actorCmdRead(t, store, gate).Status == "closed" {
		t.Fatal("refused round closed its gate")
	}
}
