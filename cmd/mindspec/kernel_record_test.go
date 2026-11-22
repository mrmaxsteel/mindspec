package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func TestS134JudgeCmd_RecordEvidence(t *testing.T) {
	bin := buildMindspecBinary(t)
	for _, mode := range []string{"success", "worktree", "unknown-field", "invalid-utf8", "artifact"} {
		t.Run(mode, func(t *testing.T) {
			f, _ := actorCmdFixture(t)
			f.Must(os.MkdirAll(filepath.Join(f.Root, ".beads"), 0755))
			f.Must(os.WriteFile(filepath.Join(f.Root, ".beads", "config.yaml"), []byte("types.custom: gate,step\n"), 0644))
			gate := f.Items[0] + ".review"
			subject := actorCmdGit(t, f, "rev-parse", "HEAD")
			id := gate + ".r1.lens"
			f.Create(id, gate)
			kerneltest.Authorise(f, gate, kernel.GateRound{Round: 1, Subject: subject, SlotCount: 2, Slots: []string{gate + ".r1.R1", gate + ".r1.R2"}})
			f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeySubject, kernel.Subject{Commit: subject})))
			store := actorCmdStore(t, f)
			token := judgeCmdLease(t, f, bin, id)
			data := fmt.Sprintf(` {"lenses":[{"lens":"inspect paths", "slot":"R1"},{"slot":"R2","lens":"check tests"}], "step":%q } `, id)
			path := filepath.Join(t.TempDir(), "record.json")
			flag := "--record"
			switch mode {
			case "worktree":
				path = filepath.Join(f.Root, "record.json")
			case "unknown-field":
				data = strings.Replace(data, `"step":`, `"extra":1,"step":`, 1)
			case "invalid-utf8":
				data = strings.Replace(data, "inspect paths", "invalid\xff", 1)
			case "artifact":
				flag = "--artifact"
			}
			f.Must(os.WriteFile(path, []byte(data), 0600))
			before := actorCmdGit(t, f, "status", "--porcelain")
			raw, code := actorCmdRun(t, f, bin, "submit", id, "--lease", token, flag, path)
			row := actorCmdRead(t, store, id)
			if mode != "success" {
				if code == 0 || row.Status == "closed" {
					t.Fatalf("%s accepted: exit=%d %s", mode, code, raw)
				}
				if _, has := row.Metadata[kernel.KeyRecord]; has {
					t.Fatal("refused evidence wrote record")
				}
				return
			}
			if code != 0 {
				t.Fatalf("lens record submit: exit=%d %s", code, raw)
			}
			receipt, has, err := kernel.Decode[kernel.Receipt](row.Metadata, kernel.KeyClose)
			f.Must(err)
			if !has || row.Status != "closed" || row.Metadata[kernel.KeyRecord] != data || receipt.RecordSHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(data))) || receipt.Commit != "" || receipt.Path != "" || receipt.Blob != "" {
				t.Fatalf("record receipt: %+v", receipt)
			}
			f.Must(kernel.ValidReceipt(f.Root, receipt, row.Metadata))
			if actorCmdGit(t, f, "status", "--porcelain") != before {
				t.Fatal("record submission changed git status")
			}
		})
	}
}
