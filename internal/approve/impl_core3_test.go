package approve

import (
	"bytes"
	"encoding/json"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const core3Time = "2026-10-20T00:00:00Z"

func core3JSON(t *testing.T, dir, name string, value interface{}) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), raw, 0600); err != nil {
		t.Fatal(err)
	}
}
func core3Append(t *testing.T, dir string, values ...interface{}) {
	t.Helper()
	var lines [][]byte
	for _, value := range values {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, raw)
	}
	if err := panel.AppendRecords(dir, lines...); err != nil {
		t.Fatal(err)
	}
}
func core3Fixture(t *testing.T, root, slug, gate string, rc bool, specIDs ...string) panel.Registration {
	t.Helper()
	dir := filepath.Join(root, "review", slug)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	id := "mindspec-core3.1"
	p := panel.Panel{Spec: "130-core3", Target: "spec/130-core3", Gate: gate, Round: 1,
		ExpectedReviewers: 2, ReviewedHeadSHA: "abc123", SubjectDigest: "sha256:abc", Telemetry: "required",
		Roster: []panel.RosterSlot{{Slot: "R1", Model: "model-a", Lens: "correctness"}, {Slot: "R2", Model: "model-b", Lens: "architecture"}}}
	if len(specIDs) > 0 {
		p.Spec = specIDs[0]
		p.Target = "spec/" + p.Spec
	}
	if gate == "bead" {
		p.BeadID = &id
		p.Target = "bead/" + id
	}
	event := panel.RoundRecord{Record: panel.RecordRound, Spec: p.Spec, Panel: slug, Round: p.Round,
		ReviewedHeadSHA: p.ReviewedHeadSHA, SubjectDigest: p.SubjectDigest, Target: p.Target, Gate: p.Gate,
		BeadID: p.BeadID, Roster: p.Roster, ExpectedReviewers: p.ExpectedReviewers, Telemetry: p.Telemetry, CreatedAt: core3Time}
	core3Append(t, dir, event)
	core3JSON(t, dir, panel.FileName, p)
	verdict := "APPROVE"
	if rc {
		verdict = "REQUEST_CHANGES"
	}
	core3JSON(t, dir, "R1-round-1.json", map[string]interface{}{"verdict": verdict, "rationale": "test finding"})
	core3JSON(t, dir, "R2-round-1.json", map[string]string{"verdict": "APPROVE"})
	if rc {
		res, err := panel.Tally(dir)
		if err != nil {
			t.Fatal(err)
		}
		ref := panel.Refutation{Slot: "R1", Round: 1, Finding: "slot", FindingSHA256: res.Verdicts[0].SlotSHA256,
			FindingText: "test finding", Ground: "disproven", Reason: "tested", Evidence: "abc123", By: "operator", CreatedAt: core3Time}
		core3Append(t, dir, panel.RefutationRecord{Record: panel.RecordRefutation, Spec: p.Spec, Panel: slug, Refutation: ref})
		p.Refutations = []panel.Refutation{ref}
		core3JSON(t, dir, panel.FileName, p)
	}
	return panel.Registration{Dir: dir, Panel: p}
}
func core3Cover(t *testing.T, reg panel.Registration) {
	t.Helper()
	row := panel.DispositionRow{Record: panel.RecordDisposition, ID: "core3-row", Spec: reg.Panel.Spec, Gate: reg.Panel.Gate,
		Panel: reg.Slug(), Reviewer: "R1", Model: "model-a", Severity: "major", Summary: "test finding", ConvergentWith: []string{},
		Disposition: "false-contamination", CreatedAt: core3Time}
	core3Append(t, reg.Dir, row)
}
func core3Manifest(t *testing.T, reg panel.Registration) {
	t.Helper()
	res, err := panel.Tally(reg.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if !res.PassRule(true) {
		t.Fatalf("fixture must pass without freshness: %+v", res)
	}
	if err := panel.WriteCoverageManifest(res); err != nil {
		t.Fatal(err)
	}
}

func core3ApproveRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0700); err != nil {
		t.Fatal(err)
	}
	return root
}
func TestImplApprove_Core3Leg(t *testing.T) {
	for _, gate := range []string{"spec_approve", "plan_approve", "bead", "final_review"} {
		t.Run(gate, func(t *testing.T) {
			t.Setenv(panel.SkipPanelEnv, "0")
			root := core3ApproveRoot(t)
			reg := core3Fixture(t, root, "coverage", gate, true)
			core3Manifest(t, reg)
			err := implPanelCore3Preflight(root, "130-core3", nil)
			if err == nil || !strings.Contains(err.Error(), "mindspec panel refute 'coverage' --spec '130-core3' --slot 'R1' --round 1") || strings.Contains(err.Error(), "mindspec panel tally") {
				t.Fatalf("uncovered slot needs exact refute recovery: %v", err)
			}
			core3Cover(t, reg)
			if err := implPanelCore3Preflight(root, "130-core3", nil); err != nil {
				t.Fatal(err)
			}
			orphan := filepath.Join(root, "review", "orphan")
			if err := os.MkdirAll(orphan, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(orphan, "dispositions.jsonl"), []byte("malformed"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := implPanelCore3Preflight(root, "130-core3", nil); err != nil {
				t.Fatalf("ledger-only directory is not a registration: %v", err)
			}
		})
	}
}
func TestImplApprove_Core3EveryRegistration(t *testing.T) {
	for _, failure := range []string{"uncovered", "missing store", "malformed registration"} {
		for _, failingFirst := range []bool{false, true} {
			order := "failing last"
			if failingFirst {
				order = "failing first"
			}
			t.Run(failure+"/"+order, func(t *testing.T) {
				t.Setenv(panel.SkipPanelEnv, "0")
				root := core3ApproveRoot(t)
				passingSlug, failingSlug := "a-bead", "z-final-review"
				if failingFirst {
					passingSlug, failingSlug = "z-bead", "a-final-review"
				}
				passing := core3Fixture(t, root, passingSlug, "bead", false)
				core3Manifest(t, passing)
				if err := implPanelCore3Preflight(root, "130-core3", nil); err != nil {
					t.Fatalf("covered APPROVE registration must pass: %v", err)
				}
				failing := core3Fixture(t, root, failingSlug, "final_review", true)
				core3Manifest(t, failing)
				switch failure {
				case "missing store":
					if err := os.Rename(filepath.Join(failing.Dir, "dispositions.jsonl"), filepath.Join(failing.Dir, "saved-ledger")); err != nil {
						t.Fatal(err)
					}
				case "malformed registration":
					if err := os.WriteFile(filepath.Join(failing.Dir, panel.FileName), []byte(`{"spec":`), 0600); err != nil {
						t.Fatal(err)
					}
				}
				err := implPanelCore3Preflight(root, "130-core3", nil)
				if err == nil || !strings.Contains(err.Error(), "panel "+failingSlug+" slots [") {
					t.Fatalf("must block naming failing registration %s: %v", failingSlug, err)
				}
				switch failure {
				case "uncovered":
					if !strings.Contains(err.Error(), "mindspec panel refute '"+failingSlug+"' --spec '130-core3' --slot 'R1' --round 1") {
						t.Fatalf("must name uncovered slot in recovery: %v", err)
					}
				case "missing store":
					if !strings.Contains(err.Error(), "R1") || !strings.Contains(err.Error(), "only one legacy marker") {
						t.Fatalf("must name slots and refuse missing store: %v", err)
					}
				case "malformed registration":
					if !strings.Contains(err.Error(), "parse panel.json") {
						t.Fatalf("must refuse malformed registration: %v", err)
					}
				}
			})
		}
	}
}

func TestImplApprove_AbandonedSkipIntegrity(t *testing.T) {
	for _, scenario := range []string{"audited abandon", "manual abandon", "one marker", "roster edit", "legacy"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv(panel.SkipPanelEnv, "0")
			root := core3ApproveRoot(t)
			reg := core3Fixture(t, root, "coverage", "final_review", false)
			reg.Panel.Abandoned = true
			reg.Panel.AbandonReason = "operator: superseded"
			if scenario == "audited abandon" || scenario == "roster edit" {
				core3Append(t, reg.Dir, panel.AbandonRecord{Record: panel.RecordAbandon, Spec: reg.Panel.Spec, Panel: reg.Slug(), Reason: reg.Panel.AbandonReason, By: "operator", CreatedAt: core3Time})
			}
			if scenario == "roster edit" {
				reg.Panel.Roster[0].Model = "manual"
			}
			if scenario == "one marker" || scenario == "legacy" {
				reg.Panel.Telemetry = ""
			}
			if scenario == "legacy" {
				if err := os.Rename(filepath.Join(reg.Dir, "dispositions.jsonl"), filepath.Join(reg.Dir, "saved-ledger")); err != nil {
					t.Fatal(err)
				}
			}
			core3JSON(t, reg.Dir, panel.FileName, reg.Panel)
			core3JSON(t, reg.Dir, "R1-round-1.json", map[string]string{"verdict": "REJECT"})
			var out bytes.Buffer
			err := implPanelCore3Preflight(root, "130-core3", &out)
			if scenario == "audited abandon" || scenario == "legacy" {
				if err != nil || !strings.Contains(out.String(), "skipped") {
					t.Fatalf("audited exit needs advisory: %q %v", out.String(), err)
				}
				return
			}
			if err == nil {
				t.Fatal("integrity must block before abandoned skip")
			}
		})
	}
}
func TestImplApprove_Core3Hatches(t *testing.T) {
	for _, hatch := range []string{"env", "config"} {
		t.Run(hatch, func(t *testing.T) {
			root := core3ApproveRoot(t)
			core3Fixture(t, root, "coverage", "final_review", false)
			t.Setenv(panel.SkipPanelEnv, "0")
			if hatch == "env" {
				t.Setenv(panel.SkipPanelEnv, "1")
			} else {
				if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte("enforcement:\n  panel_gate: false\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := implPanelCore3Preflight(root, "130-core3", nil); err != nil {
				t.Fatalf("%s hatch must skip Core 3: %v", hatch, err)
			}
		})
	}
}

func TestImplApprove_Core3LegBeforeMutation(t *testing.T) {
	root := core3ApproveRoot(t)
	writeSpecDir(t, root, "010-test")
	writePlanWithBeads(t, root, "010-test", []string{"bead-1"})
	saveAndRestore(t)
	t.Setenv(panel.SkipPanelEnv, "0")
	reg := core3Fixture(t, root, "final-review", "final_review", false, "010-test")
	mock := &executor.MockExecutor{CommitCountResult: 5, FinalizeEpicResult: executor.FinalizeResult{MergeStrategy: "direct", CommitCount: 5}}
	mutated := false
	implRunBDCombinedFn = func(...string) ([]byte, error) { mutated = true; return []byte("ok"), nil }
	_, err := ApproveImpl(root, "010-test", mock)
	if err == nil || !strings.Contains(err.Error(), reg.Slug()) || !strings.Contains(err.Error(), "R1") {
		t.Fatalf("ApproveImpl must refuse coverage naming final panel and slot: %v", err)
	}
	if mutated || len(mock.CallsTo("FinalizeEpic")) != 0 {
		t.Fatal("approval mutated before coverage")
	}
}

func TestImplPreflight_BeadIDHandEdit(t *testing.T) {
	root := core3ApproveRoot(t)
	reg := core3Fixture(t, root, "binding", "bead", false)
	wrong := "probe-wrong.1"
	reg.Panel.BeadID = &wrong
	core3JSON(t, reg.Dir, panel.FileName, reg.Panel)
	if err := implPanelCore3Preflight(root, reg.Panel.Spec, nil); err == nil || !strings.Contains(err.Error(), "changed outside the panel verbs (bead_id)") {
		t.Fatalf("snapshot bead edit escaped approval preflight: %v", err)
	}
}

func TestImplPreflight_CorruptBeadRegistration(t *testing.T) {
	for _, damage := range []string{"malformed ledger", "unparseable snapshot"} {
		t.Run(damage, func(t *testing.T) {
			root := core3ApproveRoot(t)
			reg := core3Fixture(t, root, "binding", "bead", false)
			want := "parse panel.json"
			if damage == "unparseable snapshot" {
				if err := os.WriteFile(filepath.Join(reg.Dir, panel.FileName), []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				wrong := "probe-wrong.1"
				reg.Panel.BeadID = &wrong
				core3JSON(t, reg.Dir, panel.FileName, reg.Panel)
				f, err := os.OpenFile(filepath.Join(reg.Dir, "dispositions.jsonl"), os.O_APPEND|os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				_, writeErr := f.WriteString("<<<<<<< HEAD\n")
				closeErr := f.Close()
				if writeErr != nil || closeErr != nil {
					t.Fatalf("append conflict: %v %v", writeErr, closeErr)
				}
				want = "ledger line"
			}
			if err := implPanelCore3Preflight(root, reg.Panel.Spec, nil); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("corrupt registration escaped preflight: %v", err)
			}
		})
	}
}
