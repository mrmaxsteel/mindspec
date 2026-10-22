package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func resetV2Flags(t *testing.T) {
	t.Helper()
	for _, cmd := range []*cobra.Command{panelSubmitCmd, panelVerifyCmd, panelTallyCmd} {
		reset := func() { cmd.Flags().VisitAll(func(f *pflag.Flag) { _ = f.Value.Set(f.DefValue); f.Changed = false }) }
		reset()
		t.Cleanup(reset)
	}
	resetDischargeFlags(t)
}

type cliV2Fixture struct {
	root, dir string
	p         panel.Panel
	layout    panel.SlotLayout
}

func newCLIV2(t *testing.T) cliV2Fixture {
	t.Helper()
	root := mkPanelTestRoot(t, "")
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	root = real
	withTestChdir(t, root)
	resetV2Flags(t)
	config.ResetCache()
	t.Cleanup(config.ResetCache)
	stubNonBeadRevParse(t, func(string, string) (string, error) { return paneltest.ReviewedSHA, nil })
	dir := filepath.Join(root, "review", "p")
	in := panel.CreateInput{Spec: "900-test", Target: "main", Gate: "final_review", Round: 1, ReviewedHeadSHA: paneltest.ReviewedSHA, SubjectDigest: "sha256:test", Roster: []panel.RosterSlot{{Slot: "R1", Family: "harness", Model: "m", Lens: "one"}, {Slot: "R2", Family: "harness", Model: "m", Lens: "two"}}}
	if err := panel.Create(dir, in); err != nil {
		t.Fatal(err)
	}
	r, err := panel.Tally(dir)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := panel.NewSlotLayout(root, ".mindspec-scratch", "900-test", "p", 1, "R1")
	if err != nil {
		t.Fatal(err)
	}
	return cliV2Fixture{root, dir, *r.Panel, layout}
}
func (f cliV2Fixture) verdict() panel.VerdictV2 { return paneltest.Verdict("APPROVE") }
func (f cliV2Fixture) target(slot string) panel.SubmitTarget {
	return panel.SubmitTarget{Dir: f.dir, Panel: "p", Spec: "900-test", Slot: slot, Round: 1, SlotDir: f.layout.Dir}
}
func (f cliV2Fixture) submit(t *testing.T, v panel.VerdictV2) panel.SubmissionRecord {
	t.Helper()
	r, err := panel.Submit(f.target(v.Slot), paneltest.Bytes(t, v), paneltest.Resolve)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func (f cliV2Fixture) file(t *testing.T, v panel.VerdictV2) string {
	t.Helper()
	file := filepath.Join(f.root, "input.json")
	if err := os.WriteFile(file, paneltest.Bytes(t, v), 0600); err != nil {
		t.Fatal(err)
	}
	return file
}
func (f cliV2Fixture) launch(t *testing.T, status string, k int) panel.LaunchRecord {
	t.Helper()
	r := panel.LaunchRecord{Record: panel.RecordLaunch, Spec: "900-test", Panel: "p", Slot: "R1", Round: 1, Attempt: k, Family: "harness", Model: "m", Status: status, RunnerKind: "harness", RunnerSource: "default", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	paneltest.Seed(t, f.dir, r)
	return r
}
func (f cliV2Fixture) args(file string) []string {
	return []string{"submit", "p", "--slot", "R1", "--round", "1", "--spec", "900-test", "--file", file}
}
func (f cliV2Fixture) shortSetup(t *testing.T, k int) string {
	t.Helper()
	if err := os.MkdirAll(f.layout.Tree, 0700); err != nil {
		t.Fatal(err)
	}
	if err := panel.WriteSlotMetadata(f.layout, panel.SlotMetadata{Spec: "900-test", Slug: "p", Round: 1, Slot: "R1", MainRoot: f.root}); err != nil {
		t.Fatal(err)
	}
	dir := f.layout.AttemptDir(k)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "verdict.json")
	if err := os.WriteFile(file, paneltest.Bytes(t, f.verdict()), 0600); err != nil {
		t.Fatal(err)
	}
	return file
}
func cliV2Refusal(t *testing.T, f cliV2Fixture, want string, args ...string) {
	t.Helper()
	before := panelTreeBytes(t, f.dir)
	out, err := runPanelVerbCmd(args...)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("want %q: %v %s", want, err, out)
	}
	if after := panelTreeBytes(t, f.dir); !reflect.DeepEqual(before, after) {
		t.Fatalf("refusal changed panel bytes: before=%v after=%v", before, after)
	}
}
func TestPanelSubmit_Refusals(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"schema", "/schema"}, {"slot_field", "/slot"}, {"round_field", "/round"}, {"off_roster", "roster"}, {"round_target", "current round"}, {"abandoned", "abandoned"}, {"sha", "/reviewed_sha"}, {"wrong_model", "/model"}, {"unsafe_model", "/model"}, {"process", "panel launch"}, {"v1", "--round"}, {"local_invalid", "config.local.yaml"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCLIV2(t)
			v := f.verdict()
			args := f.args("")
			switch tc.name {
			case "schema":
				v.Schema = 1
			case "slot_field":
				v.Slot = "R2"
			case "round_field":
				v.Round = 2
			case "off_roster":
				args[3] = "R9"
			case "round_target":
				args[5] = "2"
			case "abandoned":
				f.p.Abandoned = true
				f.p.AbandonReason = "stop"
				if err := panel.WriteSnapshot(f.dir, f.p); err != nil {
					t.Fatal(err)
				}
			case "sha":
				v.ReviewedSHA = strings.Repeat("b", 40)
			case "wrong_model":
				v.Model = "other"
			case "unsafe_model":
				v.Model = "-m"
			case "process":
				if err := os.WriteFile(filepath.Join(f.root, ".mindspec", "config.local.yaml"), []byte("panel:\n  runners:\n    - {family: harness, kind: process, argv: [fake], prompt: stdin, output: stdout}\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "v1":
				f = seedCLIV1(t, f)
				args[1] = "old"
			case "local_invalid":
				if err := os.WriteFile(filepath.Join(f.root, ".mindspec", "config.local.yaml"), []byte("unexpected: true\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			args[len(args)-1] = f.file(t, v)
			cliV2Refusal(t, f, tc.want, args...)
		})
	}
}

func TestPanelSubmit_SlotDirectoryRefusals(t *testing.T) {
	for _, state := range []string{"explicit_round", "current_round"} {
		t.Run(state, func(t *testing.T) {
			f := newCLIV2(t)
			file := f.shortSetup(t, 1)
			f.launch(t, "dispatched", 1)
			args := []string{"submit", "--file", file}
			want := "explicit target differs from the slot directory"
			if state == "explicit_round" {
				args = append(args, "--round", "2")
			} else {
				p := f.p
				if err := panel.Create(f.dir, panel.CreateInput{Spec: p.Spec, Target: p.Target, Gate: p.Gate, Round: 2, ReviewedHeadSHA: p.ReviewedHeadSHA, SubjectDigest: p.SubjectDigest, Roster: p.Roster}); err != nil {
					t.Fatal(err)
				}
				want = "slot directory differs from the current panel"
			}
			cliV2Refusal(t, f, want, args...)
		})
	}
}

// seedCLIV1 creates a historical registration, without rewriting a round record.
func seedCLIV1(t *testing.T, f cliV2Fixture) cliV2Fixture {
	t.Helper()
	f.dir = filepath.Join(f.root, "review", "old")
	f.p.VerdictSchema = 0
	if err := panel.WriteSnapshot(f.dir, f.p); err != nil {
		t.Fatal(err)
	}
	paneltest.Seed(t, f.dir, panel.RoundRecord{Record: panel.RecordRound, Spec: f.p.Spec, Panel: "old", Round: 1, ReviewedHeadSHA: f.p.ReviewedHeadSHA, SubjectDigest: f.p.SubjectDigest, Target: f.p.Target, Gate: f.p.Gate, Roster: f.p.Roster, ExpectedReviewers: 2, Telemetry: "required", CreatedAt: paneltest.CreatedAt})
	// Select this panel under the normal slug for callers through a fresh root.
	return f
}
func TestPanelSubmit_NoViaFlag(t *testing.T) {
	f := newCLIV2(t)
	args := append(f.args(f.file(t, f.verdict())), "--via", "launch")
	cliV2Refusal(t, f, "unknown flag: --via", args...)
}
func TestPanelSubmit_LandedBindingRefused(t *testing.T) {
	f := newCLIV2(t)
	f.p.BeadID = ptrStr("probe-x.1")
	if err := panel.WriteSnapshot(f.dir, f.p); err != nil {
		t.Fatal(err)
	}
	old := panelSubmitMetadataFn
	panelSubmitMetadataFn = func(id string) (map[string]interface{}, error) {
		if id != "probe-x.1" {
			t.Fatalf("wrong bead %s", id)
		}
		return map[string]interface{}{"mindspec_landed_review_panel": "p@r1"}, nil
	}
	t.Cleanup(func() { panelSubmitMetadataFn = old })
	cliV2Refusal(t, f, "--round 2", f.args(f.file(t, f.verdict()))...)
}
func TestPanelSubmit_ABA(t *testing.T) {
	f := newCLIV2(t)
	a := f.verdict()
	b := a
	b.Verdict = "REJECT"
	for _, v := range []panel.VerdictV2{a, b, a} {
		file := f.file(t, v)
		if out, err := runPanelVerbCmd(f.args(file)...); err != nil || !strings.Contains(out, "R1 final") || !strings.Contains(out, "rule:") {
			t.Fatalf("submit: %v %s", err, out)
		}
	}
	lines, err := panel.ReadLedger(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	var seqs []int
	for _, line := range lines {
		var r panel.SubmissionRecord
		if err := json.Unmarshal(line, &r); err != nil {
			t.Fatal(err)
		}
		if r.Record == panel.RecordVerdict {
			seqs = append(seqs, r.Seq)
		}
	}
	if !reflect.DeepEqual(seqs, []int{1, 2, 3}) {
		t.Fatalf("seqs %v", seqs)
	}
	other := a
	other.Slot = "R2"
	f.submit(t, other)
	res, err := panel.Tally(f.dir)
	if err != nil || !res.PassRule(true) || res.Approves != 2 {
		t.Fatalf("gate did not count A: %+v %v", res, err)
	}
}
func TestPanelSubmit_SupersededAttemptRefused(t *testing.T) {
	f := newCLIV2(t)
	file := f.shortSetup(t, 1)
	f.launch(t, "dispatched", 1)
	f.launch(t, "dispatched", 2)
	cliV2Refusal(t, f, "attempt 1 was superseded by 2", "submit", "--file", file)
}
func TestPanelSubmit_LongFormSuperseded(t *testing.T) {
	f := newCLIV2(t)
	file := f.shortSetup(t, 1)
	f.launch(t, "dispatched", 1)
	f.launch(t, "dispatched", 2)
	cliV2Refusal(t, f, "attempt 1 was superseded by 2", f.args(file)...)
}
func TestPanelSubmit_EndedAttemptRefused(t *testing.T) {
	for _, state := range []string{"ended", "future", "none"} {
		t.Run(state, func(t *testing.T) {
			f := newCLIV2(t)
			file := f.shortSetup(t, 1)
			want := "was not dispatched"
			if state == "ended" {
				f.launch(t, "dispatched", 1)
				f.launch(t, "launch_failed", 1)
				want = "attempt 1 ended with launch_failed"
			}
			if state == "future" {
				file = f.shortSetup(t, 2)
				f.launch(t, "dispatched", 1)
			}
			cliV2Refusal(t, f, want, "submit", "--file", file)
		})
	}
}
func TestPanelSubmit_SameAttemptDraftThenFinal(t *testing.T) {
	f := newCLIV2(t)
	file := f.shortSetup(t, 1)
	f.launch(t, "dispatched", 1)
	v := f.verdict()
	for _, state := range []string{"draft", "final"} {
		v.Status = state
		if err := os.WriteFile(file, paneltest.Bytes(t, v), 0600); err != nil {
			t.Fatal(err)
		}
		if out, err := runPanelVerbCmd("submit", "--file", file); err != nil || !strings.Contains(out, "R1 "+state) {
			t.Fatalf("%s: %v %s", state, err, out)
		}
	}
	subs, err := panel.LatestSubmissions(f.dir, 1)
	if err != nil || subs["R1"].Seq != 2 {
		t.Fatalf("draft/final seq: %+v %v", subs, err)
	}
}
func TestPanelSubmit_SameAttemptRerunRepairs(t *testing.T) {
	f := newCLIV2(t)
	file := f.shortSetup(t, 1)
	f.launch(t, "dispatched", 1)
	if out, err := runPanelVerbCmd("submit", "--file", file); err != nil {
		t.Fatalf("seed: %v %s", err, out)
	}
	before, err := panel.ReadLedger(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(f.dir, "R1-round-1.json")
	if err := os.WriteFile(dest, []byte("damaged"), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := runPanelVerbCmd("submit", "--file", file); err != nil {
		t.Fatalf("repair: %v %s", err, out)
	}
	after, err := panel.ReadLedger(f.dir)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("rerun added record: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil || !bytes.Equal(got, paneltest.Bytes(t, f.verdict())) {
		t.Fatalf("repair failed: %v %s", err, got)
	}
}
func TestVerdictV2_FindingProjection(t *testing.T) {
	f := newCLIV2(t)
	v := f.verdict()
	v.Verdict = "REQUEST_CHANGES"
	v.ConcreteChangesRequired = []panel.ChangeRequired{{Text: "a", Basis: "traced"}, {Text: "b", Basis: "traced"}}
	f.submit(t, v)
	other := f.verdict()
	other.Slot = "R2"
	f.submit(t, other)
	r, err := panel.Tally(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	for i, text := range []string{"a", "b"} {
		sum := sha256.Sum256([]byte(text))
		if r.Verdicts[0].Findings[i].ID != fmt.Sprintf("ccr[%d]", i) || r.Verdicts[0].Findings[i].SHA256 != hex.EncodeToString(sum[:]) {
			t.Fatalf("projection: %+v", r.Verdicts[0])
		}
	}
	out, err := runPanelVerbCmd("refute", "p", "--spec", "900-test", "--slot", "R1", "--round", "1", "--finding", "ccr[1]", "--ground", "disproven", "--reason", "verified", "--evidence", "https://example.com/proof", "--by", "operator")
	if err != nil {
		t.Fatalf("refute: %v %s", err, out)
	}
	r, err = panel.Tally(f.dir)
	if err != nil || len(r.AppliedRefutations()) != 1 || r.AppliedRefutations()[0].Finding != "ccr[1]" || !strings.Contains(r.UnresolvedNames(), "ccr[0]") || strings.Contains(r.UnresolvedNames(), "ccr[1]") || r.PassRule(true) {
		t.Fatalf("refutation discharges wrong item: %+v %v", r, err)
	}
}

// Real slot clone helpers keep the reviewed .mindspec copy independent.
func cloneSlotV2(t *testing.T, f cliV2Fixture, slot string) panel.SlotLayout {
	t.Helper()
	layout, err := panel.NewSlotLayout(f.root, ".mindspec-scratch", "900-test", "p", 1, slot)
	if err != nil {
		t.Fatal(err)
	}
	if err := panel.WriteSlotMetadata(layout, panel.SlotMetadata{Spec: "900-test", Slug: "p", Round: 1, Slot: slot, MainRoot: f.root}); err != nil {
		t.Fatal(err)
	}
	rebindGit(t, f.root, "clone", "--shared", "--no-checkout", f.root, layout.Tree)
	rebindGit(t, layout.Tree, "checkout", "--detach", "HEAD")
	rebindGit(t, layout.Tree, "remote", "remove", "origin")
	if err := os.MkdirAll(filepath.Join(layout.Tree, "sub"), 0700); err != nil {
		t.Fatal(err)
	}
	return layout
}
func realCloneFixture(t *testing.T) cliV2Fixture {
	t.Helper()
	f := newCLIV2(t)
	rebindWrite(t, f.root, ".mindspec/config.yaml", "version: 1\n")
	rebindWrite(t, f.root, "sub/tracked.txt", "tracked\n")
	rebindGit(t, f.root, "init", "-q", "-b", "main")
	rebindGit(t, f.root, "add", ".")
	rebindGit(t, f.root, "commit", "-qm", "fixture")
	return f
}
func TestSlotRoot_SubmitFromClone(t *testing.T) {
	for _, long := range []bool{false, true} {
		t.Run(fmt.Sprint(long), func(t *testing.T) {
			f := realCloneFixture(t)
			layout := cloneSlotV2(t, f, "R1")
			before := panelTreeBytes(t, filepath.Join(layout.Tree, ".mindspec"))
			withTestChdir(t, filepath.Join(layout.Tree, "sub"))
			file := "v.json"
			if err := os.WriteFile(file, paneltest.Bytes(t, f.verdict()), 0600); err != nil {
				t.Fatal(err)
			}
			args := []string{"submit", "--file", file}
			if long {
				args = f.args(file)
			}
			if out, err := runPanelVerbCmd(args...); err != nil {
				t.Fatalf("submit: %v %s", err, out)
			}
			subs, err := panel.LatestSubmissions(f.dir, 1)
			if err != nil || subs["R1"].Seq != 1 {
				t.Fatalf("main ledger unchanged: %+v %v", subs, err)
			}
			if got, err := os.ReadFile(filepath.Join(f.dir, "R1-round-1.json")); err != nil || !bytes.Equal(got, paneltest.Bytes(t, f.verdict())) {
				t.Fatalf("main verdict: %v %s", err, got)
			}
			if after := panelTreeBytes(t, filepath.Join(layout.Tree, ".mindspec")); !reflect.DeepEqual(before, after) {
				t.Fatal("clone .mindspec changed")
			}
		})
	}
}
func TestSlotRoot_SubmitFromFilePath(t *testing.T) {
	f := realCloneFixture(t)
	a := cloneSlotV2(t, f, "R1")
	b := cloneSlotV2(t, f, "R2")
	file := filepath.Join(a.Dir, "v.json")
	if err := os.WriteFile(file, paneltest.Bytes(t, f.verdict()), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := runPanelVerbCmd("submit", "--file", file); err != nil {
		t.Fatalf("absolute file: %v %s", err, out)
	}
	rel, err := filepath.Rel(f.root, file)
	if err != nil {
		t.Fatal(err)
	}
	cliV2Refusal(t, f, "needs a slug", "submit", "--file", rel)
	withTestChdir(t, filepath.Join(b.Tree, "sub"))
	before := panelTreeBytes(t, f.dir)
	out, err := runPanelVerbCmd("submit", "--file", file)
	if err == nil || !strings.Contains(err.Error(), a.Dir) || !strings.Contains(err.Error(), b.Dir) {
		t.Fatalf("conflict: %v %s", err, out)
	}
	if !reflect.DeepEqual(before, panelTreeBytes(t, f.dir)) {
		t.Fatal("conflict wrote panel")
	}
	cliV2Refusal(t, f, "explicit target differs", "submit", "p", "--slot", "R1", "--file", "-")
}
func TestSlotRoot_Refusals(t *testing.T) {
	for _, state := range []string{"stray", "identity", "outside"} {
		t.Run(state, func(t *testing.T) {
			f := realCloneFixture(t)
			layout := cloneSlotV2(t, f, "R1")
			cwd := filepath.Join(layout.Tree, "sub")
			switch state {
			case "stray":
				rebindWrite(t, cwd, "slot.json", `{"slot":"wrong"}`)
			case "identity":
				if err := panel.WriteSlotMetadata(layout, panel.SlotMetadata{Spec: "900-test", Slug: "p", Round: 1, Slot: "R2", MainRoot: f.root}); err != nil {
					t.Fatal(err)
				}
			case "outside":
				layout.Dir = filepath.Join(t.TempDir(), "R1")
				layout.Metadata = filepath.Join(layout.Dir, "slot.json")
				layout.Tree = filepath.Join(layout.Dir, "tree")
				if err := os.MkdirAll(layout.Tree, 0700); err != nil {
					t.Fatal(err)
				}
				if err := panel.WriteSlotMetadata(layout, panel.SlotMetadata{Spec: "900-test", Slug: "p", Round: 1, Slot: "R1", MainRoot: f.root}); err != nil {
					t.Fatal(err)
				}
				cwd = layout.Tree
			}
			withTestChdir(t, cwd)
			root, err := findPanelRoot(config.IsolationRootFor)
			if state == "stray" {
				if err != nil || root != f.root {
					t.Fatalf("stray marker not ignored: %s %v", root, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "invalid slot directory") {
				t.Fatalf("invalid root accepted: %s %v", root, err)
			}
		})
	}
}
func TestSlotRoot_SubmitTargetRefusals(t *testing.T) {
	for _, state := range []string{"explicit_round", "registration_round", "registration_spec"} {
		t.Run(state, func(t *testing.T) {
			f := newCLIV2(t)
			spec := f.p.Spec
			args := []string{"submit", "--file", f.file(t, f.verdict())}
			want := "slot directory differs from the current panel"
			switch state {
			case "explicit_round":
				args = append(args, "--round", "2")
				want = "explicit target differs from the slot directory"
			case "registration_round":
				p := f.p
				if err := panel.Create(f.dir, panel.CreateInput{Spec: p.Spec, Target: p.Target, Gate: p.Gate, Round: 2, ReviewedHeadSHA: p.ReviewedHeadSHA, SubjectDigest: p.SubjectDigest, Roster: p.Roster}); err != nil {
					t.Fatal(err)
				}
			case "registration_spec":
				spec = ""
			}
			layout, err := panel.NewSlotLayout(f.root, ".mindspec-scratch", spec, "p", 1, "R1")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(layout.Tree, 0700); err != nil {
				t.Fatal(err)
			}
			if err := panel.WriteSlotMetadata(layout, panel.SlotMetadata{Spec: spec, Slug: "p", Round: 1, Slot: "R1", MainRoot: f.root}); err != nil {
				t.Fatal(err)
			}
			withTestChdir(t, layout.Tree)
			cliV2Refusal(t, f, want, args...)
		})
	}
}
func TestSlotRoot_Rules(t *testing.T) {
	f := realCloneFixture(t)
	layout := cloneSlotV2(t, f, "R1")
	withTestChdir(t, layout.Tree)
	rebindWrite(t, layout.Tree, ".mindspec/config.local.yaml", "invalid: true\n")
	var out bytes.Buffer
	cmd := newRulesCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--scope", "reviewer"})
	if err := cmd.Execute(); err != nil || !strings.Contains(out.String(), "panel.submit") {
		t.Fatalf("rules did not use main config: %v %s", err, out.String())
	}
	root, err := findPanelRoot(config.IsolationRootFor)
	if err != nil || root != f.root {
		t.Fatalf("root %s %v", root, err)
	}
}
func TestSlotRoot_DispositionQuery(t *testing.T) {
	f := realCloneFixture(t)
	layout := cloneSlotV2(t, f, "R1")
	dir := filepath.Join(f.root, ".mindspec", "specs", "900-test", "reviews", "q")
	paneltest.Seed(t, dir, panel.CoverageManifest{Record: panel.RecordPanelManifest, Spec: "900-test", Panel: "q", Gate: "bead", Round: 1, Slots: []panel.ManifestSlot{{Slot: "R1", Model: "main-model", Verdict: "APPROVE"}}})
	external := filepath.Join(t.TempDir(), "reviews")
	paneltest.Seed(t, filepath.Join(external, "x"), panel.CoverageManifest{Record: panel.RecordPanelManifest, Spec: "900-test", Panel: "x", Gate: "bead", Round: 1, Slots: []panel.ManifestSlot{{Slot: "R1", Model: "external-model", Verdict: "APPROVE"}}})
	withTestChdir(t, filepath.Join(layout.Tree, "sub"))
	for _, tc := range []struct{ dir, spec, model string }{{"", "", "main-model"}, {"", "900-test", "main-model"}, {external, "", "external-model"}} {
		pattern, err := resolveDispositionQueryGlob(tc.dir, tc.spec)
		if err != nil {
			t.Fatal(err)
		}
		_, manifests, err := panel.LoadStore(pattern)
		if err != nil || len(manifests) != 1 || manifests[0].Slots[0].Model != tc.model {
			t.Fatalf("query %s: %+v %v", pattern, manifests, err)
		}
	}
}

type panelSubmitErrorReader struct{ err error }

func (r panelSubmitErrorReader) Read([]byte) (int, error) { return 0, r.err }

func TestPanelSubmit_TerminalSafeFileErrors(t *testing.T) {
	for _, source := range []string{"open", "read"} {
		t.Run(source, func(t *testing.T) {
			f := newCLIV2(t)
			path := filepath.Join(f.root, "missing\x1b[31m\nrecovery: printf INJECTED")
			file := path
			if source == "read" {
				file = "-"
				rootCmd.SetIn(panelSubmitErrorReader{&os.PathError{Op: "read", Path: path, Err: fmt.Errorf("read failed")}})
				t.Cleanup(func() { rootCmd.SetIn(nil) })
			}
			out, err := runPanelVerbCmd(f.args(file)...)
			if err == nil || !strings.Contains(out, "INJECTED") || !strings.Contains(out, "recovery: select a readable") {
				t.Fatalf("file error did not reach Cobra output: %q %v", out, err)
			}
			for _, rendered := range []string{out, err.Error()} {
				if strings.Contains(rendered, "\x1b") || strings.Contains(rendered, "\nrecovery: printf INJECTED") {
					t.Errorf("file error contains terminal controls or a forged recovery line: %q", rendered)
				}
			}
		})
	}
}
