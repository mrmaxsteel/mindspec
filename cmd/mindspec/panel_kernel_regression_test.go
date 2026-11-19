package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

var panelKernelRealResolver = panelKernelStateFn

func kernelPanelAssertUnpublished(t *testing.T, f *kerneltest.Fixture, id, dir string) {
	t.Helper()
	r, err := f.Store.Lookup(id)
	f.Must(err)
	if r != nil {
		_, receipt, err := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose)
		f.Must(err)
		if r.Status == "closed" || receipt {
			t.Fatalf("refused slot closed or receipted: %+v", r)
		}
	}
	subs, err := panel.LatestSubmissions(dir, 1)
	f.Must(err)
	if len(subs) != 0 {
		t.Fatalf("refused verdict published: %+v", subs)
	}
}

func TestS134PanelSubmit_WrongAndOldLease(t *testing.T) {
	for _, old := range []bool{false, true} {
		t.Run(fmt.Sprint(old), func(t *testing.T) {
			f, gate, head := kernelPanelFixture(t)
			kernelPanelAuthorise(f, gate, head, 1, 2)
			out, err := kernelPanelCreate(t, f, "1")
			kernelPanelRequire(t, out, err)
			id := gate + ".r1.R1"
			lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
			f.Must(err)
			token := "wrong-token"
			if old {
				_, err = kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Take: true, Reason: "new operator"}, kernel.LeaseDeps{})
				f.Must(err)
				token = lease.Token
				current, ok, err := kernel.Decode[kernel.LeaseRecord](kernelPanelRecord(t, f, id).Metadata, kernel.KeyLease)
				f.Must(err)
				if !ok || current.Generation != 2 {
					t.Fatal("take did not advance generation", current)
				}
			}
			v := paneltest.Verdict("APPROVE")
			v.Slot, v.Round, v.ReviewedSHA, v.Model = "R1", 1, head, "test-model"
			file := filepath.Join(f.SpecDir, "reviews", "1", "R1-round-1.json")
			f.Must(os.WriteFile(file, paneltest.Bytes(t, v), 0644))
			actorCmdGit(t, f, "add", ".")
			actorCmdGit(t, f, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "verdict")
			out, err = kernelPanelRun(t, panelSubmitCmd, "1", "--spec", f.SpecID, "--slot", "R1", "--round", "1", "--file", file, "--lease", token)
			kernelPanelRefusal(t, out, err, "lease token is not current")
			kernelPanelAssertUnpublished(t, f, id, filepath.Dir(file))
		})
	}
}

func TestS134PanelSubmit_PublishesCommittedBytes(t *testing.T) {
	f, gate, head := kernelPanelFixture(t)
	kernelPanelAuthorise(f, gate, head, 1, 2)
	out, err := kernelPanelCreate(t, f, "1")
	kernelPanelRequire(t, out, err)
	id := gate + ".r1.R1"
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
	f.Must(err)
	mk := func(word string) []byte {
		v := paneltest.Verdict(word)
		v.Slot, v.Round, v.ReviewedSHA, v.Model = "R1", 1, head, "test-model"
		if word == "REQUEST_CHANGES" {
			v.ConcreteChangesRequired = []panel.ChangeRequired{{Text: "real defect", Basis: "traced"}}
		}
		return paneltest.Bytes(t, v)
	}
	committed, earlier := mk("REQUEST_CHANGES"), mk("APPROVE")
	file := filepath.Join(f.SpecDir, "reviews", "1", "R1-round-1.json")
	f.Must(os.WriteFile(file, committed, 0644))
	actorCmdGit(t, f, "add", ".")
	actorCmdGit(t, f, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "verdict")
	f.Must(os.WriteFile(file, earlier, 0644))
	resolver := panelKernelStateFn
	panelKernelStateFn = func(root, spec string) (*kernel.State, error) {
		f.Must(os.WriteFile(file, committed, 0644))
		return resolver(root, spec)
	}
	t.Cleanup(func() { panelKernelStateFn = resolver })
	out, err = kernelPanelRun(t, panelSubmitCmd, "1", "--spec", f.SpecID, "--slot", "R1", "--round", "1", "--file", file, "--lease", lease.Token)
	if err != nil {
		kernelPanelAssertUnpublished(t, f, id, filepath.Dir(file))
		return
	}
	subs, err := panel.LatestSubmissions(filepath.Dir(file), 1)
	f.Must(err)
	receipt, ok, err := kernel.Decode[kernel.Receipt](kernelPanelRecord(t, f, id).Metadata, kernel.KeyClose)
	f.Must(err)
	if !ok || subs["R1"].Verdict != "REQUEST_CHANGES" {
		t.Fatal("published verdict differs from receipt", subs, receipt, out)
	}
	published, err := os.ReadFile(file)
	f.Must(err)
	if !bytes.Equal(published, committed) || actorCmdGit(t, f, "rev-parse", receipt.Commit+":"+receipt.Path) != receipt.Blob {
		t.Fatal("published bytes differ from committed evidence")
	}
}

func panelRealResolver(t *testing.T, f *kerneltest.Fixture) string {
	t.Helper()
	resolver := panelKernelStateFn
	panelKernelStateFn = panelKernelRealResolver
	t.Cleanup(func() { panelKernelStateFn = resolver })
	return actorCmdStore(t, f)
}

func panelPreKernelFixture(t *testing.T) (*kerneltest.Fixture, string) {
	t.Helper()
	f := kerneltest.PreKernelSpec(t, kerneltest.WithFake(true))
	withTestChdir(t, f.Root)
	rebindWrite(t, f.Root, ".beads/config.yaml", "types.custom: gate,step\n")
	rebindWrite(t, f.Root, ".mindspec/config.yaml", "panel:\n  reviewers:\n    - family: harness\n      model: test-model\n      lens: test-lens\n      count: 2\n")
	rebindWrite(t, f.Root, "proof.txt", "proof")
	actorCmdGit(t, f, "add", ".")
	actorCmdGit(t, f, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "fixture")
	head := actorCmdGit(t, f, "rev-parse", "HEAD")
	actorCmdGit(t, f, "branch", "bead/"+f.Items[0])
	actorCmdGit(t, f, "branch", "spec/"+f.SpecID)
	return f, head
}

func TestS134PanelCreate_PreKernelRealResolver(t *testing.T) {
	f, head := panelPreKernelFixture(t)
	storePath := panelRealResolver(t, f)
	state, err := panelKernelStateFn(f.Root, f.SpecID)
	f.Must(err)
	if state != nil {
		t.Fatal("pre-kernel epic selected", state)
	}
	snapshot := func() string { out, err := os.ReadFile(storePath); f.Must(err); return string(out) }
	before := snapshot()
	check := func() {
		t.Helper()
		if snapshot() != before {
			t.Fatal("pre-kernel verb wrote kernel records")
		}
	}
	out, err := kernelPanelCreate(t, f, "legacy", "--gate", "bead", "--target", "bead/"+f.Items[0])
	kernelPanelRequire(t, out, err)
	check()
	dir := filepath.Join(f.SpecDir, "reviews", "legacy")
	for _, slot := range []string{"R1", "R2"} {
		v := paneltest.Verdict("APPROVE")
		if slot == "R1" {
			v.Verdict = "REQUEST_CHANGES"
			v.ConcreteChangesRequired = []panel.ChangeRequired{{Text: "missing test", Basis: "traced"}}
		}
		v.Slot, v.Round, v.Model, v.ReviewedSHA = slot, 1, "test-model", head
		file := filepath.Join(dir, slot+"-round-1.json")
		f.Must(os.WriteFile(file, paneltest.Bytes(t, v), 0644))
		out, err = kernelPanelRun(t, panelSubmitCmd, "legacy", "--spec", f.SpecID, "--slot", slot, "--round", "1", "--file", file)
		kernelPanelRequire(t, out, err)
		check()
	}
	fake := fakeDeferralBD(t)
	rebindWrite(t, fake, "epics.json", fmt.Sprintf(`[{"id":%q,"issue_type":"epic","metadata":{"spec_id":%q}}]`, f.Epic, f.SpecID))
	rebindWrite(t, fake, "follow-up.json", fmt.Sprintf(`[{"id":"follow-up","status":"open","description":"missing test","dependencies":[{"id":%q,"dependency_type":"discovered-from"}]}]`, f.Epic))
	out, err = kernelPanelRun(t, panelDeferCmd, "legacy", "--spec", f.SpecID, "--slot", "R1", "--round", "1", "--bead", "follow-up", "--by", "operator")
	kernelPanelRequire(t, out, err)
	check()
	out, err = kernelPanelRun(t, panelRefuteCmd, "legacy", "--spec", f.SpecID, "--slot", "R1", "--round", "1", "--ground", "disproven", "--reason", "covered", "--evidence", "proof.txt", "--by", "operator")
	kernelPanelRequire(t, out, err)
	check()
	out, err = kernelPanelRun(t, panelRebindCmd, "legacy", "--spec", f.SpecID, "--reason", "operator: checked")
	kernelPanelRequire(t, out, err)
	check()
}

func TestS134PanelCreate_RealResolverAmbiguous(t *testing.T) {
	f, _, _ := kernelPanelFixture(t)
	epic := f.Fake.Records[f.Epic]
	epic.ID = "other-epic"
	epic.Metadata = map[string]any{"spec_id": "003-other", kernel.KeyRun: kernel.DefaultPolicy}
	f.Fake.Records[epic.ID] = epic
	rebindWrite(t, f.Root, ".mindspec/specs/003-other/spec.md", "# Other")
	storePath := panelRealResolver(t, f)
	before, err := os.ReadFile(storePath)
	f.Must(err)
	s, err := panelKernelStateFn(f.Root, "")
	if err == nil || s != nil || !strings.Contains(err.Error(), "select one enrolled spec") {
		t.Fatalf("ambiguous selection: %v %v", s, err)
	}
	after, err := os.ReadFile(storePath)
	f.Must(err)
	var a, b any
	f.Must(json.Unmarshal(before, &a))
	f.Must(json.Unmarshal(after, &b))
	if !reflect.DeepEqual(a, b) {
		t.Fatal("resolver wrote")
	}
}

func TestS134PanelCreate_NonBeadNoTracker(t *testing.T) {
	for _, gate := range []string{"", "adhoc", "spec_approve", "plan_approve"} {
		t.Run(gate, func(t *testing.T) {
			f, _, head := kernelPanelFixture(t)
			panelKernelStateFn = panelKernelRealResolver
			git, err := exec.LookPath("git")
			f.Must(err)
			path := t.TempDir()
			f.Must(os.Symlink(git, filepath.Join(path, "git")))
			t.Setenv("PATH", path)
			t.Setenv("BEADS_DIR", "")
			if _, err := exec.LookPath("bd"); err == nil {
				t.Fatal("tracker CLI present")
			}
			args := []string{"legacy", "--target", "spec/" + f.SpecID}
			spec := f.SpecID
			if gate == "adhoc" {
				spec = ""
				args = append(args, "--gate", gate)
			} else {
				args = append(args, "--spec", spec)
				if gate != "" {
					args = append(args, "--gate", gate)
				}
			}
			out, err := kernelPanelRun(t, panelCreateCmd, args...)
			kernelPanelRequire(t, out, err)
			reg, err := findPanelRegistration(f.Root, "legacy", spec)
			f.Must(err)
			for _, slot := range []string{"R1", "R2"} {
				v := paneltest.Verdict("APPROVE")
				if slot == "R1" {
					v.Verdict = "REQUEST_CHANGES"
					v.ConcreteChangesRequired = []panel.ChangeRequired{{Text: "missing test", Basis: "traced"}}
				}
				v.Slot, v.Round, v.Model, v.ReviewedSHA = slot, 1, "test-model", head
				file := filepath.Join(reg.Dir, slot+"-round-1.json")
				f.Must(os.WriteFile(file, paneltest.Bytes(t, v), 0644))
				out, err = kernelPanelRun(t, panelSubmitCmd, "legacy", "--spec", spec, "--slot", slot, "--round", "1", "--file", file)
				kernelPanelRequire(t, out, err)
			}
			out, err = kernelPanelRun(t, panelDeferCmd, "legacy", "--spec", spec, "--slot", "R1", "--round", "1")
			kernelPanelRefusal(t, out, err, "--by must not be empty")
			out, err = kernelPanelRun(t, panelRefuteCmd, "legacy", "--spec", spec, "--slot", "R1", "--round", "1", "--ground", "disproven", "--reason", "covered", "--evidence", "proof.txt", "--by", "operator")
			kernelPanelRequire(t, out, err)
			out, err = kernelPanelRun(t, panelRebindCmd, "legacy", "--spec", spec, "--reason", "operator: checked")
			kernelPanelRequire(t, out, err)
		})
	}
}

func TestS134PanelCreate_PreKernelNoRoundWarning(t *testing.T) {
	f, _ := panelPreKernelFixture(t)
	// The panel has no enrollment witness and uses no tracker lookup.
	panelKernelStateFn = panelKernelRealResolver
	for n := 1; n <= 6; n++ {
		out, err := kernelPanelRun(t, panelCreateCmd, "plan", "--spec", f.SpecID, "--gate", "plan_approve", "--target", "spec/"+f.SpecID, "--round", fmt.Sprint(n))
		kernelPanelRequire(t, out, err)
		if strings.Contains(out, "warning:") {
			t.Fatal("pre-kernel round warning", out)
		}
	}
	// A corrupt ledger on another slug cannot add a round-count refusal.
	rebindWrite(t, f.Root, filepath.Join(".mindspec/specs", f.SpecID, "reviews/plan/dispositions.jsonl"), "bad-json\n")
	if _, err := panel.RoundsForGate(panel.Scan(f.SpecDir), f.SpecID, "plan_approve", ""); err == nil {
		t.Fatal("corrupt ledger did not fail the counter")
	}
	out, err := kernelPanelRun(t, panelCreateCmd, "other", "--spec", f.SpecID, "--gate", "plan_approve", "--target", "spec/"+f.SpecID)
	kernelPanelRequire(t, out, err)
}

func TestS134PanelRefute_OpenChildrenSorted(t *testing.T) {
	for _, underFix := range []bool{false, true} {
		t.Run(fmt.Sprint(underFix), func(t *testing.T) {
			f, gate, head := kernelPanelFixture(t)
			kernelPanelAuthorise(f, gate, head, 1, 2)
			out, err := kernelPanelCreate(t, f, "1")
			kernelPanelRequire(t, out, err)
			for _, slot := range []string{"R1", "R2"} {
				kernelPanelVerdict(t, f, gate, head, "1", slot, "APPROVE", 1)
			}
			parent := gate
			if underFix {
				parent = gate + ".r2.fix"
				f.Create(parent, gate)
			}
			for _, id := range []string{"foreign-z", "foreign-a"} {
				f.Fake.Records[id] = kernel.Record{ID: id, Parent: parent, Status: "open", Type: "task", Title: id}
			}
			before := kerneltest.Snapshot(t, f)
			for n := 0; n < 10; n++ {
				err := closeRetalliedChildren(f.State(), gate, filepath.Join(f.SpecDir, "reviews", "1"))
				if err == nil || !strings.Contains(err.Error(), "foreign-a, foreign-z") {
					t.Fatal("open children not sorted", err)
				}
			}
			if before != kerneltest.Snapshot(t, f) {
				t.Fatal("foreign child refusal wrote")
			}
		})
	}
}

func TestS134PanelCreate_StoreRecovery(t *testing.T) {
	f, _, _ := kernelPanelFixture(t)
	panelKernelStateFn = panelKernelRealResolver
	t.Setenv("PATH", t.TempDir())
	t.Setenv("BEADS_DIR", "")
	_, err := panelKernelStateFn(f.Root, f.SpecID)
	if err == nil || !strings.Contains(err.Error(), "panel tracker store is unavailable") || !strings.Contains(err.Error(), "bd list --all --json") {
		t.Fatal("store recovery missing", err)
	}
}

func TestS134PanelCreate_InferGateIds(t *testing.T) {
	f, _ := panelPreKernelFixture(t)
	for _, tt := range []struct{ slug, spec, target, want string }{
		{f.Items[0], f.SpecID, "", "bead"},
		{"1", f.SpecID, "another", "bead"},
		{f.SpecID, f.SpecID, "", "final_review"},
		{"legacy", f.SpecID, "spec/" + f.SpecID, ""},
		{"legacy", "../outside", "main", ""},
		{"legacy", "bad;value", "main", ""},
	} {
		if got := panelCreateKernelGate(f.Root, tt.slug, tt.spec, tt.target, ""); got != tt.want {
			t.Fatalf("%+v: got %q", tt, got)
		}
	}
}
