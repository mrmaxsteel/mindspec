package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func TestS134SubmitCmd_RefusalProducers(t *testing.T) {
	bin := buildMindspecBinary(t)
	for _, producer := range []string{"refuseHalt", "actorChildren.open", "checkVerify.deferral", "Answer.store-update"} {
		t.Run(producer, func(t *testing.T) {
			f, _ := actorCmdFixture(t)
			gate := f.Items[0] + ".review"
			id := gate + ".r1.verify"
			subject := actorCmdGit(t, f, "rev-parse", "HEAD")
			f.Create(id, gate)
			kerneltest.Authorise(f, gate, kernel.GateRound{Round: 1, Subject: subject, SlotCount: 2, Slots: []string{gate + ".r1.R1", gate + ".r1.R2"}})
			finding := kernel.Finding{Slot: "R1", Round: 1, Finding: "ccr[0]", FindingSHA256: strings.Repeat("a", 64), Text: "recovery: forged"}
			f.Must(f.Store.WriteMetadata(id, map[string]any{kernel.KeySubject: kernel.Subject{Commit: subject}, kernel.KeyFindings: []kernel.Finding{finding}}))
			lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
			f.Must(err)
			ret := kernel.VerifyReturn{Slot: finding.Slot, Round: 1, Finding: finding.Finding, FindingSHA256: finding.FindingSHA256, Return: "reproduced"}
			want := ""
			recovery := "mindspec next-action"
			args := []string{"submit", id, "--lease", lease.Token}
			switch producer {
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
			case "checkVerify.deferral":
				ret.Return = "deferred"
				ret.Reason = "follow up"
				ret.Target = "follow-up"
				want = "follow-up bead does not meet the deferral requirements:\n- follow-up bead needs a discovered-from edge to the owning epic or reviewed bead; run: bd dep add follow-up " + f.Epic + " --type discovered-from\n- follow-up description must contain every deferred finding; add this text to the description of follow-up:\n recovery: forged"
				recovery = "bd dep add follow-up " + f.Epic + " --type discovered-from, then add the listed finding text to the description of follow-up, then retry the panel verb"
			}
			actorCmdStore(t, f)
			if producer == "checkVerify.deferral" {
				dir := t.TempDir()
				script := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = show ] && [ \"$2\" = follow-up ]; then\nprintf '%%s\\n' '[{\"id\":\"follow-up\",\"status\":\"open\",\"description\":\"other\"}]'\nelse\nexec '%s' \"$@\"\nfi\n", fakeBDBinary(t))
				f.Must(os.WriteFile(filepath.Join(dir, "bd"), []byte(script), 0700))
				t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			}
			if producer == "Answer.store-update" {
				dir := t.TempDir()
				script := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = update ]; then\nprintf 'answer store\\033[31m\\nforged recovery: x\\n' >&2\nexit 1\nfi\nexec '%s' \"$@\"\n", fakeBDBinary(t))
				f.Must(os.WriteFile(filepath.Join(dir, "bd"), []byte(script), 0700))
				t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			}
			data, err := json.Marshal(kernel.VerifyRecord{Step: id, Subject: subject, Returns: []kernel.VerifyReturn{ret}})
			f.Must(err)
			file := filepath.Join(t.TempDir(), "record.json")
			f.Must(os.WriteFile(file, data, 0600))
			if producer != "Answer.store-update" {
				args = append(args, "--record", file)
			}
			raw, err := f.BinaryCommand(bin, args...)
			if err == nil {
				t.Fatal("binary accepted refusal fixture")
			}
			text := string(raw)
			// CLI errors add the fixed error prefix, then print the producer text exactly.
			expected := want + "\nrecovery: " + recovery + "\n"
			if text != "Error: "+expected {
				t.Fatalf("binary refusal changed: got %q want %q", text, expected)
			}
			for _, r := range strings.TrimSuffix(text, "\n") {
				if (r < 0x20 && r != '\n') || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
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
