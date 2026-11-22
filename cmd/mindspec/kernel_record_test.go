package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

func TestS134JudgeCmd_RecordEvidence(t *testing.T) {
	bin := buildMindspecBinary(t)
	for _, mode := range []string{"success", "worktree", "unknown-field", "invalid-utf8", "artifact", "link-in-tree", "link-parent-target", "link-parent-hardlink", "link-parent-spelling", "relative-link-parent", "absolute-up", "relative-up", "relative-cwd"} {
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
			want := ""
			accepted := mode == "success" || mode == "relative-up" || mode == "absolute-up" || mode == "relative-cwd"
			cwd := f.Root
			switch mode {
			case "link-in-tree":
				original := path
				f.Must(os.WriteFile(original, []byte(data), 0600))
				path = filepath.Join(f.Root, "alias.json")
				f.Must(os.Symlink(original, path))
				want = "outside every worktree"
			case "link-parent-target", "link-parent-hardlink":
				child := filepath.Join(f.Root, "child")
				f.Must(os.Mkdir(child, 0700))
				f.Must(os.WriteFile(filepath.Join(f.Root, "record.json"), []byte(data), 0600))
				alias := filepath.Join(filepath.Dir(path), "alias")
				f.Must(os.Symlink(child, alias))
				if mode == "link-parent-hardlink" {
					f.Must(os.Link(filepath.Join(f.Root, "record.json"), path))
					want = "outside every worktree"
				} else {
					f.Must(os.WriteFile(path, []byte(data), 0600))
					want = "two different files"
				}
				path = alias + "/../record.json"
			case "link-parent-spelling", "relative-link-parent":
				child := filepath.Join(filepath.Dir(path), "one", "two")
				f.Must(os.MkdirAll(child, 0700))
				f.Must(os.WriteFile(filepath.Join(filepath.Dir(f.Root), "record.json"), []byte(data), 0600))
				alias := filepath.Join(f.Root, "alias")
				f.Must(os.Symlink(child, alias))
				path = alias + "/../../record.json"
				if mode == "relative-link-parent" {
					path = "alias/../../record.json"
				}
				want = "two different files"
			case "relative-cwd":
				cwd = filepath.Join(f.Root, "child")
				f.Must(os.Mkdir(cwd, 0700))
				path = "../../record.json"
			case "absolute-up", "relative-up":
				path = f.Root + "/../record.json"
				if mode == "relative-up" {
					path = "../record.json"
				}
			case "worktree":
				path = filepath.Join(f.Root, "record.json")
			case "unknown-field":
				data = strings.Replace(data, `"step":`, `"extra":1,"step":`, 1)
			case "invalid-utf8":
				data = strings.Replace(data, "inspect paths", "invalid\xff", 1)
			case "artifact":
				flag = "--artifact"
			}
			writePath := path
			if !filepath.IsAbs(writePath) {
				writePath = cwd + string(filepath.Separator) + writePath
			}
			f.Must(os.WriteFile(writePath, []byte(data), 0600))
			beforeRow, err := json.Marshal(actorCmdRead(t, store, id))
			f.Must(err)
			before := actorCmdGit(t, f, "status", "--porcelain")
			var raw string
			var code int
			if mode == "relative-cwd" {
				out, err := f.BinaryCommandIn(cwd, bin, "submit", id, "--lease", token, flag, path)
				raw = string(out)
				if err != nil {
					var exit *exec.ExitError
					if !errors.As(err, &exit) {
						t.Fatalf("binary failed to start: %v", err)
					}
					code = exit.ExitCode()
				}
			} else {
				raw, code = actorCmdRun(t, f, bin, "submit", id, "--lease", token, flag, path)
			}
			row := actorCmdRead(t, store, id)
			if !accepted {
				if code == 0 || row.Status == "closed" {
					t.Fatalf("%s accepted: exit=%d %s", mode, code, raw)
				}
				if want != "" && (!strings.Contains(raw, want) || !strings.Contains(raw, "recovery:")) {
					t.Fatalf("wrong path refusal: %s", raw)
				}
				afterRow, err := json.Marshal(row)
				f.Must(err)
				if string(beforeRow) != string(afterRow) {
					t.Fatal("refused path changed the step")
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

func TestS134SubmitCmd_ArtifactPathIdentity(t *testing.T) {
	bin := buildMindspecBinary(t)
	for _, mode := range []string{"api-absolute", "api-relative", "cli-absolute", "cli-relative"} {
		t.Run(mode, func(t *testing.T) {
			f, gate, head := kernelPanelFixture(t)
			kernelPanelAuthorise(f, gate, head, 1, 2)
			out, err := kernelPanelCreate(t, f, "1")
			kernelPanelRequire(t, out, err)
			id := gate + ".r1.R1"
			lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
			f.Must(err)
			v := paneltest.Verdict("APPROVE")
			v.Slot, v.Round, v.ReviewedSHA, v.Model = "R1", 1, head, "test-model"
			dir := filepath.Join(f.SpecDir, "reviews", "1")
			file := filepath.Join(dir, "verdict.json")
			data := paneltest.Bytes(t, v)
			f.Must(os.WriteFile(file, data, 0600))
			actorCmdGit(t, f, "add", ".")
			actorCmdGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "verdict")
			outside := t.TempDir()
			child := filepath.Join(outside, "child")
			f.Must(os.Mkdir(child, 0700))
			f.Must(os.WriteFile(filepath.Join(outside, "verdict.json"), data, 0600))
			alias := filepath.Join(dir, "alias")
			f.Must(os.Symlink(child, alias))
			path := alias + "/../verdict.json"
			if strings.HasSuffix(mode, "relative") {
				rel, err := filepath.Rel(f.Root, dir)
				f.Must(err)
				path = rel + "/alias/../verdict.json"
			}
			before := actorRecordForPath(t, f, id)
			if strings.HasPrefix(mode, "api") {
				_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: path})
				if err == nil || !strings.Contains(err.Error(), "two different files") {
					t.Fatalf("ambiguous artifact accepted or wrong refusal: %v", err)
				}
				if string(before) != string(actorRecordForPath(t, f, id)) {
					t.Fatal("ambiguous artifact changed step")
				}
			} else {
				store := actorCmdStore(t, f)
				raw, code := actorCmdRun(t, f, bin, "submit", id, "--lease", lease.Token, "--artifact", path)
				if code == 0 || !strings.Contains(raw, "two different files") {
					t.Fatalf("ambiguous artifact accepted or wrong refusal: exit=%d %s", code, raw)
				}
				after, err := json.Marshal(actorCmdRead(t, store, id))
				f.Must(err)
				if string(before) != string(after) {
					t.Fatal("ambiguous artifact changed step")
				}
			}
		})
	}
}

func actorRecordForPath(t *testing.T, f *kerneltest.Fixture, id string) []byte {
	t.Helper()
	row, err := f.Store.Lookup(id)
	f.Must(err)
	data, err := json.Marshal(row)
	f.Must(err)
	return data
}
