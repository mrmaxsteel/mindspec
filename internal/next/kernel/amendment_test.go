package kernel_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func recordFile(t *testing.T, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "record.json")
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func recordFixture(t *testing.T, kind string) (*kerneltest.Fixture, string, kernel.LeaseResult, string) {
	t.Helper()
	kerneltest.WithRules(t, kernel.NewRegistry())
	f := reducerFixture(t)
	gate := f.Items[0] + ".review"
	subject := reducerGit(t, f, "rev-parse", "HEAD")
	kerneltest.Authorise(f, gate, kernel.GateRound{Round: 1, Subject: subject, SlotCount: 2, Slots: []string{gate + ".r1.R1", gate + ".r1.R2"}})
	id := gate + ".r1." + kind
	f.Create(id, gate)
	f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeySubject, kernel.Subject{Commit: subject})))
	lease := leaseActor(t, f, id, kernel.LeaseDeps{})
	data := fmt.Sprintf(` { "lenses" : [{"lens":"inspect paths", "slot":"R1"},{"slot":"R2","lens":"check tests"}], "step":%q } `, id)
	if kind == "verify" {
		finding := kernel.Finding{Slot: "R1", Round: 1, Finding: "ccr[0]", FindingSHA256: strings.Repeat("a", 64), Text: "fix this"}
		f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyFindings, []kernel.Finding{finding})))
		data = fmt.Sprintf(`{"step":%q,"subject":%q,"returns":[{"slot":"R1","round":1,"finding":"ccr[0]","finding_sha256":%q,"return":"reproduced"}]}`, id, subject, finding.FindingSHA256)
	}
	return f, id, lease, data
}

func TestS134Submit_RecordBytesAndReceipt(t *testing.T) {
	f, id, lease, data := recordFixture(t, "lens")
	path := recordFile(t, data)
	status := reducerGit(t, f, "status", "--porcelain")
	out, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: path})
	f.Must(err)
	if out.Schema != "mindspec.next-action/v2" {
		t.Fatalf("submit schema=%q", out.Schema)
	}
	r := actorRecord(t, f, id)
	receipt := actorReceipt(t, f, id)
	if r.Status != "closed" || r.Metadata[kernel.KeyRecord] != data || receipt.RecordSHA256 != digest(data) || receipt.Path != "" || receipt.Commit != "" || receipt.Blob != "" {
		t.Fatalf("record bytes or receipt lost: %+v metadata=%v", receipt, r.Metadata)
	}
	if reducerGit(t, f, "status", "--porcelain") != status {
		t.Fatal("record submit changed git status")
	}
	f.Must(kernel.ValidReceipt(f.Root, receipt, r.Metadata))
	f.Must(f.Store.WriteMetadata(id, map[string]any{"unrelated": "later write"}))
	r = actorRecord(t, f, id)
	f.Must(kernel.ValidReceipt(f.Root, receipt, r.Metadata))
	if r.Metadata[kernel.KeyRecord] != data || actorReceipt(t, f, id).RecordSHA256 != digest(data) {
		t.Fatal("later metadata write changed record bytes or digest")
	}
	for _, mode := range []string{"wrong-digest", "absent-record"} {
		t.Run(mode, func(t *testing.T) {
			metadata := map[string]any{}
			for k, v := range r.Metadata {
				metadata[k] = v
			}
			bad := receipt
			if mode == "wrong-digest" {
				bad.RecordSHA256 = strings.Repeat("b", 64)
			} else {
				delete(metadata, kernel.KeyRecord)
			}
			if kernel.ValidReceipt(f.Root, bad, metadata) == nil {
				t.Fatal("invalid record receipt accepted: " + mode)
			}
		})
	}
	before := kerneltest.Snapshot(t, f)
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: path})
	f.Must(err)
	if before != kerneltest.Snapshot(t, f) {
		t.Fatal("record receipt retry wrote")
	}
}

func TestS134Submit_FixPanelOnlyCommit(t *testing.T) {
	f := reducerFixture(t)
	reducerGit(t, f, "switch", "bead/"+f.Items[0])
	gate := f.Items[0] + ".review"
	registerGate(t, f, gate, "review", 1)
	id := gate + ".r2.fix"
	f.Create(id, gate)
	lease := leaseActor(t, f, id, kernel.LeaseDeps{})
	path, err := filepath.Rel(f.Root, filepath.Join(f.SpecDir, "reviews", "review", "note.txt"))
	f.Must(err)
	commit := actorCommit(t, f, path, "review note", "bead/"+f.Items[0])
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Commit: commit})
	assertSubmitRefusal(t, err, "outside panel directories", "mindspec next-action")
	if err == nil || !strings.Contains(err.Error(), "outside panel directories") || actorRecord(t, f, id).Status == "closed" {
		t.Fatalf("panel-only fix accepted: %v", err)
	}
}

func digest(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }

func TestS134Submit_RecordRefusals(t *testing.T) {
	for _, mode := range []string{"worktree", "symlink-worktree", "unknown", "invalid-utf8", "artifact", "slot-record", "implement-record", "fix-record"} {
		t.Run(mode, func(t *testing.T) {
			f, id, lease, data := recordFixture(t, "lens")
			path := recordFile(t, data)
			want := ""
			switch mode {
			case "worktree", "symlink-worktree":
				inside := filepath.Join(f.Root, "record.json")
				f.Must(os.WriteFile(inside, []byte(data), 0600))
				path = inside
				if mode == "symlink-worktree" {
					path = filepath.Join(t.TempDir(), "alias.json")
					f.Must(os.Symlink(inside, path))
				}
				want = "outside every worktree"
			case "unknown":
				data = strings.Replace(data, `"step":`, `"extra":1,"step":`, 1)
				path = recordFile(t, data)
				want = "unknown field"
			case "invalid-utf8":
				path = recordFile(t, strings.Replace(data, "inspect paths", "invalid\xff", 1))
				want = "UTF-8"
			case "slot-record", "implement-record", "fix-record":
				kind := map[string]string{"slot-record": ".review.r1.R1", "implement-record": ".impl.a1", "fix-record": ".review.r2.fix"}[mode]
				id = f.Items[0] + kind
				parent := f.Items[0] + ".review"
				if mode == "implement-record" {
					parent = f.Items[0]
				}
				f.Create(id, parent)
				lease = leaseActor(t, f, id, kernel.LeaseDeps{EnsureWorktree: func(string, string, string) (string, error) { return f.Root, nil }})
				want = "kind"
			case "artifact":
				want = "kind"
			}
			opts := kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: path}
			if mode == "artifact" {
				opts.Record = ""
				opts.Artifact = path
			}
			before := kerneltest.Snapshot(t, f)
			_, err := kernel.Submit(f.State(), opts)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("%s refusal: %v", mode, err)
			}
			if actorRecord(t, f, id).Status == "closed" || before != kerneltest.Snapshot(t, f) {
				t.Fatal("record refusal changed step")
			}
		})
	}
}

func TestS134Store_MetadataFiles(t *testing.T) {
	f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
	gate := f.Items[0] + ".review"
	id := gate + ".r1.verify"
	var files []string
	f.Fake.Hook = func(args []string) error {
		for _, arg := range args {
			if len(arg) > 128<<10 {
				t.Fatal("metadata exceeded argv limit")
			}
		}
		for i, arg := range args {
			if arg != "--metadata" {
				continue
			}
			if i+1 == len(args) || !strings.HasPrefix(args[i+1], "@") {
				t.Fatal("metadata was passed inline")
			}
			path := strings.TrimPrefix(args[i+1], "@")
			info, err := os.Stat(path)
			f.Must(err)
			if info.Mode().Perm() != 0600 {
				t.Fatalf("metadata mode=%o", info.Mode().Perm())
			}
			files = append(files, path)
		}
		return nil
	}
	var findings []kernel.Finding
	for slot := 1; slot <= 9; slot++ {
		for n := 0; n < 100; n++ {
			findings = append(findings, kernel.Finding{Slot: fmt.Sprintf("R%d", slot), Round: 1, Finding: fmt.Sprintf("ccr[%d]", n), FindingSHA256: strings.Repeat("a", 64), Text: strings.Repeat("x", 2048)})
		}
	}
	_, err := f.Store.Create(kernel.CreateRecord{ID: id, Parent: gate, Type: "step", Title: "verify", Tuple: mustTuple(t, id), Metadata: kernel.Metadata(kernel.KeyFindings, findings)})
	f.Must(err)
	f.Must(f.Store.WriteMetadata(id, map[string]any{"large": strings.Repeat("y", 1<<20)}))
	f.Must(f.Store.SetStatusMetadataIf(id, "open", "in_progress", map[string]any{"cas": strings.Repeat("z", 1<<20)}))
	if len(files) != 3 {
		t.Fatalf("metadata calls=%d", len(files))
	}
	seen := map[string]bool{}
	for _, path := range files {
		if seen[path] {
			t.Fatal("metadata file name reused")
		}
		seen[path] = true
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("metadata file left after call: %s %v", path, err)
		}
	}
	r := actorRecord(t, f, id)
	stored, _, err := kernel.Decode[[]kernel.Finding](r.Metadata, kernel.KeyFindings)
	f.Must(err)
	if r.Metadata["large"] != strings.Repeat("y", 1<<20) || r.Metadata["cas"] != strings.Repeat("z", 1<<20) {
		t.Fatal("stored metadata text was cut")
	}
	if !reflect.DeepEqual(findings, stored) {
		t.Fatal("stored findings were cut")
	}
}

func TestS134Store_MetadataBound(t *testing.T) {
	for _, mode := range []string{"create", "update", "cas"} {
		t.Run(mode, func(t *testing.T) {
			f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
			gate := f.Items[0] + ".review"
			id := gate + ".r1.verify"
			if mode != "create" {
				f.Create(id, gate)
			}
			metadata := kernel.Metadata(kernel.KeyFindings, strings.Repeat("x", 17<<20))
			calls := len(f.Fake.Calls)
			var err error
			switch mode {
			case "create":
				_, err = f.Store.Create(kernel.CreateRecord{ID: id, Parent: gate, Type: "step", Tuple: mustTuple(t, id), Metadata: metadata})
			case "update":
				err = f.Store.WriteMetadata(id, metadata)
			case "cas":
				err = f.Store.SetStatusMetadataIf(id, "open", "in_progress", metadata)
			}
			if err == nil {
				t.Fatal("oversized metadata accepted")
			}
			for _, word := range []string{"metadata-too-large", gate, id, kernel.KeyFindings, "size "} {
				if !strings.Contains(err.Error(), word) {
					t.Fatalf("metadata size refusal missing %s: %v", word, err)
				}
			}
			sized := map[string]any{kernel.KeyFindings: metadata[kernel.KeyFindings]}
			if mode == "create" {
				sized[kernel.KeyTuple] = mustTuple(t, id)
				sized[kernel.KeyID] = kernel.ComputedKey(id)
			}
			raw, marshalErr := json.Marshal(sized)
			f.Must(marshalErr)
			sizeKind := "size"
			if mode != "create" {
				sizeKind = "update size"
			}
			if !strings.Contains(err.Error(), fmt.Sprintf("%s %d bytes", sizeKind, len(raw))) {
				t.Fatalf("wrong size in refusal: %v", err)
			}
			if len(f.Fake.Calls) != calls {
				t.Fatal("oversized metadata called bd")
			}
		})
	}
}

func TestS134Store_MetadataSweep(t *testing.T) {
	f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
	dir, err := kernel.KernelTempDir(f.Root)
	f.Must(err)
	cmd := exec.Command("true")
	f.Must(cmd.Start())
	pid := cmd.Process.Pid
	f.Must(cmd.Wait())
	stale := filepath.Join(dir, fmt.Sprintf("metadata-%d-seeded", pid))
	live := filepath.Join(dir, fmt.Sprintf("metadata-%d-live", os.Getpid()))
	keep := filepath.Join(dir, "records", "record.json")
	f.Must(os.MkdirAll(filepath.Dir(keep), 0700))
	for _, path := range []string{stale, live, keep} {
		f.Must(os.WriteFile(path, []byte("keep"), 0600))
	}
	_, err = kernel.Pass(kernel.NewRegistry(), f.State(), false)
	f.Must(err)
	if _, err = os.Stat(stale); err != nil {
		t.Fatal("read-only pass swept files")
	}
	_, err = kernel.Pass(kernel.NewRegistry(), f.State(), true)
	f.Must(err)
	if _, err = os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("dead process metadata remains: %v", err)
	}
	for _, path := range []string{keep, live} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "keep" {
			t.Fatalf("sweep removed retained file %s: %v", path, err)
		}
	}
}

func TestS134Submit_ClosingPin(t *testing.T) {
	for _, mode := range []string{"matching-rerun", "different-record", "different-generation", "receipted-record", "receipted-generation", "bad-record", "bad-token", "oversized-record", "oversized-hook"} {
		t.Run(mode, func(t *testing.T) {
			f, id, lease, data := recordFixture(t, "verify")
			kerneltest.SetClock(t, fixedTime)
			path := recordFile(t, data)
			opts := kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: path}
			hookWrites, builds := 0, 0
			t.Cleanup(kernel.ReplaceCloseWriteHookForTest("verify", func(s kernel.State, r kernel.Record, receipt kernel.Receipt) ([]kernel.Write, error) {
				builds++
				value := any([]string{fmt.Sprintf("recording-hook-%d", builds)})
				if mode == "oversized-hook" {
					value = strings.Repeat("x", 17<<20)
				}
				return []kernel.Write{{Kind: "metadata", ID: id, Metadata: kernel.Metadata(kernel.KeyReturns, value)}}, nil
			}))
			f.Fake.Hook = func(args []string) error {
				for i, arg := range args {
					if arg != "--metadata" {
						continue
					}
					raw, err := os.ReadFile(strings.TrimPrefix(args[i+1], "@"))
					f.Must(err)
					var m map[string]any
					f.Must(json.Unmarshal(raw, &m))
					if _, ok := m[kernel.KeyReturns]; ok {
						if _, ok := m[kernel.KeyClose]; ok {
							continue
						}
						prior := f.Fake.Records[f.ID(id)]
						if jsonEqual(prior.Metadata[kernel.KeyReturns], m[kernel.KeyReturns]) {
							continue
						}
						if _, ok := prior.Metadata[kernel.KeyClosing]; !ok {
							t.Fatal("hook write preceded mindspec_closing")
						}
						if prior.Metadata[kernel.KeyRecord] != data {
							t.Fatal("record was not the first write after pin")
						}
						hookWrites++
					}
				}
				return nil
			}
			if mode == "bad-record" {
				opts.Record = recordFile(t, strings.TrimSuffix(data, "}")+`,"unknown":1}`)
			}
			if mode == "bad-token" {
				opts.Lease = "bad-token"
			}
			if mode == "oversized-record" {
				f.Must(f.Store.WriteMetadata(id, map[string]any{"stored": strings.Repeat("y", 9<<20)}))
				body := strings.Replace(data, `"return":"reproduced"`, `"return":"reproduced","reason":"`+strings.Repeat("x", 9<<20)+`"`, 1)
				if len(body) >= kernel.MetadataLimit {
					t.Fatal("oversized metadata fixture does not pass record read bound")
				}
				opts.Record = recordFile(t, body)
			}
			if strings.HasPrefix(mode, "bad-") || strings.HasPrefix(mode, "oversized-") {
				_, err := kernel.Submit(f.State(), opts)
				if err == nil {
					t.Fatal("bad submit accepted")
				}
				if strings.HasPrefix(mode, "oversized-") && !strings.Contains(err.Error(), "metadata-too-large") {
					t.Fatalf("wrong oversized metadata refusal: %v", err)
				}
				if _, ok := actorRecord(t, f, id).Metadata[kernel.KeyClosing]; ok {
					t.Fatal("refused submit wrote mindspec_closing")
				}
				return
			}
			crash := errors.New("killed after close hook")
			opts.BeforeClose = func(kernel.State, kernel.Receipt) error { return crash }
			_, err := kernel.Submit(f.State(), opts)
			if !errors.Is(err, crash) {
				t.Fatalf("crash fixture: %v", err)
			}
			pin, has, err := kernel.Decode[kernel.Closing](actorRecord(t, f, id).Metadata, kernel.KeyClosing)
			f.Must(err)
			if !has || pin.At != fixedTime || pin.RecordSHA256 != digest(data) || hookWrites != 1 {
				t.Fatalf("pin=%+v hook writes=%d", pin, hookWrites)
			}
			if _, has, _ := kernel.Decode[kernel.Receipt](actorRecord(t, f, id).Metadata, kernel.KeyClose); has {
				t.Fatal("kill wrote receipt")
			}
			opts.BeforeClose = nil
			kerneltest.SetClock(t, fixedTime.Add(time.Minute))
			if strings.HasPrefix(mode, "receipted-") || mode == "matching-rerun" {
				_, err = kernel.Submit(f.State(), opts)
				f.Must(err)
				if hookWrites != 2 {
					t.Fatalf("matching rerun did not rerun hook: %d", hookWrites)
				}
				receipt := actorReceipt(t, f, id)
				if receipt.At != fixedTime.Format(time.RFC3339Nano) {
					t.Fatalf("rerun changed at: %+v", receipt)
				}
				if mode == "matching-rerun" {
					return
				}
			}
			if strings.HasSuffix(mode, "generation") {
				if strings.HasPrefix(mode, "receipted-") {
					// A different generation can hold a token after a raw controller write.
					current := lease.Lease
					current.Generation++
					current.TokenHash = "sha256:" + digest("new-token")
					f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyLease, current)))
					opts.Lease = "new-token"
				} else {
					taken, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Take: true, Reason: "replace"}, kernel.LeaseDeps{})
					f.Must(err)
					opts.Lease = taken.Token
				}
			} else {
				opts.Record = recordFile(t, strings.Replace(data, `"reproduced"`, `"refuted","ground":"disproven","reason":"tested","evidence":"https://example.com/proof"`, 1))
			}
			before := actorRecord(t, f, id)
			submitSnapshot := kerneltest.Snapshot(t, f)
			_, err = kernel.Submit(f.State(), opts)
			if err == nil {
				t.Fatal("different record or generation was accepted")
			}
			after := actorRecord(t, f, id)
			current, _, err2 := kernel.Decode[kernel.Closing](after.Metadata, kernel.KeyClosing)
			f.Must(err2)
			if strings.HasPrefix(mode, "receipted-") {
				if current.Conflict != nil {
					t.Fatal("receipted step gained mindspec_closing.conflict")
				}
				if !jsonEqual(before.Metadata[kernel.KeyClose], after.Metadata[kernel.KeyClose]) {
					t.Fatal("receipt changed")
				}
				if hookWrites != 2 || submitSnapshot != kerneltest.Snapshot(t, f) {
					t.Fatal("receipted refusal made a close write")
				}
				_, err = kernel.Pass(kernel.DefaultRegistry, f.State(), true)
				f.Must(err)
				if _, ok := f.State().Records[id+".a2"]; ok {
					t.Fatal("receipted refusal created a retry")
				}
				return
			}
			if !strings.Contains(err.Error(), "mindspec_closing") || current.Conflict == nil || !current.At.Equal(pin.At) {
				t.Fatalf("missing immutable conflict: %+v %v", current, err)
			}
			delete(before.Metadata, kernel.KeyClosing)
			delete(after.Metadata, kernel.KeyClosing)
			if !jsonEqual(before, after) {
				t.Fatal("conflict wrote outside mindspec_closing")
			}
			if _, ok := after.Metadata[kernel.KeyClose]; ok {
				t.Fatal("conflict wrote a receipt")
			}
			snapshot := kerneltest.Snapshot(t, f)
			opts.Record = path
			_, err = kernel.Submit(f.State(), opts)
			if err == nil || !strings.Contains(err.Error(), "mindspec_closing") || kerneltest.Snapshot(t, f) != snapshot {
				t.Fatalf("conflicted attempt accepted or wrote: %v", err)
			}
		})
	}
}

func TestS134Submit_RoundSubjectAfterCommit(t *testing.T) {
	f := reducerFixture(t)
	doneItem(t, f, f.Items[0])
	gate := f.Items[1] + ".review"
	registerGate(t, f, gate, "review", 1)
	registerGate(t, f, gate, "review", 2)
	reduce(t, f, true)
	round := recoveryRound(t, f, gate)
	leases := map[string]kernel.LeaseResult{}
	for _, id := range round.Slots {
		leases[id] = leaseActor(t, f, id, kernel.LeaseDeps{})
	}
	beadCommit(t, f, "bead/"+f.Items[1])
	out := reduce(t, f, true)
	for _, id := range round.Slots {
		if actorRecord(t, f, id).Status != "in_progress" {
			t.Fatalf("subject move closed slot %s", id)
		}
	}
	if !jsonEqual(round, recoveryRound(t, f, gate)) || hasRun(out, "mindspec", "panel", "create", f.Items[1]) {
		t.Fatal("subject move authorised another round")
	}
	for i, id := range round.Slots {
		path := recoveryVerdict(t, f, fmt.Sprintf("R%d", i+1), 2, round.Subject, fmt.Sprintf("verdict-%d.json", i+1))
		_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: leases[id].Token, Artifact: path})
		f.Must(err)
		if actorReceipt(t, f, id).Kind != "passed" {
			t.Fatalf("round-subject verdict did not submit: %s", id)
		}
	}
}

func TestS134Submit_ClosingConflictBeforeReceipt(t *testing.T) {
	f, id, lease, data := recordFixture(t, "verify")
	_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: recordFile(t, data), BeforeClose: func(_ kernel.State, _ kernel.Receipt) error {
		pin, has, err := kernel.Decode[kernel.Closing](actorRecord(t, f, id).Metadata, kernel.KeyClosing)
		f.Must(err)
		if !has {
			t.Fatal("missing closing pin before receipt")
		}
		pin.Conflict = &kernel.ClosingIdentity{Generation: 2, RecordSHA256: "different"}
		return f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyClosing, pin))
	}})
	if err == nil || !strings.Contains(err.Error(), "mindspec_closing") {
		t.Fatalf("closing conflict was not reread: %v", err)
	}
	if _, has := actorRecord(t, f, id).Metadata[kernel.KeyClose]; has {
		t.Fatal("conflict gained a receipt")
	}
}
