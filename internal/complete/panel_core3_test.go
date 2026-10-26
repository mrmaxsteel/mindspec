package complete

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"os"
	"os/exec"
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
func core3Fixture(t *testing.T, root, slug, gate string, rc bool) panel.Registration {
	t.Helper()
	dir := filepath.Join(root, "review", slug)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	id := "mindspec-core3.1"
	p := panel.Panel{Spec: "130-core3", Target: "spec/130-core3", Gate: gate, Round: 1,
		ExpectedReviewers: 2, ReviewedHeadSHA: "abc123", SubjectDigest: "sha256:abc", Telemetry: "required",
		Roster: []panel.RosterSlot{{Slot: "R1", Model: "model-a", Lens: "correctness"}, {Slot: "R2", Model: "model-b", Lens: "architecture"}}}
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

func core3Check(t *testing.T, regs ...panel.Registration) error {
	t.Helper()
	var decisions []PanelGateResult
	for _, reg := range regs {
		decisions = append(decisions, PanelGateResult{Registration: reg, Decision: panel.Decision{Action: panel.Allow}})
	}
	return checkPanelCore3(decisions, true, nil)
}
func TestComplete_Core3Enforced(t *testing.T) {
	for _, scenario := range []string{"covered", "uncovered", "missing store", "no manifest", "changed verdicts"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv(panel.SkipPanelEnv, "0")
			reg := core3Fixture(t, t.TempDir(), "coverage", "bead", true)
			if scenario != "uncovered" {
				core3Cover(t, reg)
			}
			if scenario != "no manifest" {
				core3Manifest(t, reg)
			}
			if scenario == "missing store" {
				if err := os.Rename(filepath.Join(reg.Dir, "dispositions.jsonl"), filepath.Join(reg.Dir, "saved-ledger")); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "changed verdicts" {
				core3JSON(t, reg.Dir, "R1-round-1.json", map[string]string{"verdict": "APPROVE"})
			}
			err := core3Check(t, reg)
			if scenario == "covered" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "coverage") || !strings.Contains(err.Error(), "R1") {
				t.Fatalf("must block naming panel and slot: %v", err)
			}
			if scenario == "uncovered" {
				if !strings.Contains(err.Error(), "mindspec panel refute 'coverage' --spec '130-core3' --slot 'R1' --round 1") || strings.Contains(err.Error(), "mindspec panel tally") {
					t.Fatalf("uncovered slot needs exact refute recovery: %v", err)
				}
			}
		})
	}
}
func TestComplete_Core3ManifestRound(t *testing.T) {
	t.Setenv(panel.SkipPanelEnv, "0")
	reg := core3Fixture(t, t.TempDir(), "coverage", "bead", false)
	core3Manifest(t, reg)
	reg.Panel.Round = 2
	core3JSON(t, reg.Dir, panel.FileName, reg.Panel)
	for _, slot := range []string{"R1", "R2"} {
		core3JSON(t, reg.Dir, slot+"-round-2.json", map[string]string{"verdict": "APPROVE"})
	}
	if err := core3Check(t, reg); err == nil || !strings.Contains(err.Error(), "round") {
		t.Fatalf("old round must block: %v", err)
	}
}
func TestComplete_Core3MalformedRecords(t *testing.T) {
	for _, kind := range []string{"panel", "disposition", "round", "refutation", "deferral", "rebind", "abandon"} {
		t.Run(kind, func(t *testing.T) {
			t.Setenv(panel.SkipPanelEnv, "0")
			reg := core3Fixture(t, t.TempDir(), "coverage", "bead", false)
			core3Manifest(t, reg)
			f, err := os.OpenFile(filepath.Join(reg.Dir, "dispositions.jsonl"), os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				t.Fatal(err)
			}
			_, err = f.WriteString(`{"record":"` + kind + `"}` + "\n")
			if err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			if err := core3Check(t, reg); err == nil {
				t.Fatalf("malformed %s must block", kind)
			}
		})
	}
}
func TestComplete_Core3EveryMatchedPanel(t *testing.T) {
	t.Setenv(panel.SkipPanelEnv, "0")
	root := t.TempDir()
	a := core3Fixture(t, root, "a", "bead", false)
	b := core3Fixture(t, root, "b", "bead", false)
	core3Manifest(t, a)
	if err := core3Check(t, a, b); err == nil || !strings.Contains(err.Error(), "panel b") {
		t.Fatalf("second panel must block: %v", err)
	}
}
func TestComplete_Core3LegacyAdvisory(t *testing.T) {
	t.Setenv(panel.SkipPanelEnv, "0")
	dir := t.TempDir()
	p := panel.Panel{Round: 1, ExpectedReviewers: 2}
	core3JSON(t, dir, panel.FileName, p)
	var out bytes.Buffer
	err := checkPanelCore3([]PanelGateResult{{Registration: panel.Registration{Dir: dir, Panel: p}, Decision: panel.Decision{Action: panel.Allow}}}, true, &out)
	if err != nil || !strings.Contains(out.String(), "legacy") {
		t.Fatalf("legacy needs advisory: %q %v", out.String(), err)
	}
}
func TestComplete_Core3WarnSkips(t *testing.T) {
	for _, scenario := range []string{"abandoned", "missing-ref", "git-error", "hand-set abandoned with Allow"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv(panel.SkipPanelEnv, "0")
			root := t.TempDir()
			reg := core3Fixture(t, root, "coverage", "bead", false)
			savedRev, savedMissing := gateRevParseFn, gateIsRefNotFoundFn
			t.Cleanup(func() { gateRevParseFn = savedRev; gateIsRefNotFoundFn = savedMissing })
			gateRevParseFn = func(string, string) (string, error) { return "", errors.New("missing") }
			gateIsRefNotFoundFn = func(error) bool { return scenario == "missing-ref" }
			if scenario == "abandoned" {
				core3Append(t, reg.Dir, panel.AbandonRecord{Record: panel.RecordAbandon, Spec: reg.Panel.Spec, Panel: reg.Slug(), Reason: "operator: superseded", By: "operator", CreatedAt: core3Time})
				reg.Panel.Abandoned = true
				reg.Panel.AbandonReason = "operator: superseded"
				core3JSON(t, reg.Dir, panel.FileName, reg.Panel)
				core3JSON(t, reg.Dir, "R1-round-1.json", map[string]string{"verdict": "REJECT"})
			}
			results, err := panelGate("mindspec-core3.1", []string{root}, "", true, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != 1 || results[0].Decision.Action != panel.Warn {
				t.Fatalf("expected real gate Warn: %+v", results)
			}
			if scenario == "hand-set abandoned with Allow" {
				results[0].Decision.Action = panel.Allow
				results[0].Registration.Panel.Abandoned = true
			}
			var out bytes.Buffer
			err = checkPanelCore3(results, true, &out)
			if scenario == "hand-set abandoned with Allow" {
				if err == nil {
					t.Fatal("abandoned field must not override returned Allow")
				}
				return
			}
			if err != nil || strings.Count(out.String(), "coverage check skipped") != 1 {
				t.Fatalf("Warn must skip once without a manifest: %q %v", out.String(), err)
			}
		})
	}
}
func TestComplete_Core3Hatches(t *testing.T) {
	reg := core3Fixture(t, t.TempDir(), "coverage", "bead", false)
	orig := panelSkipEnvFn
	t.Cleanup(func() { panelSkipEnvFn = orig })
	for _, enabled := range []bool{true, false} {
		panelSkipEnvFn = func() bool { return enabled }
		if err := checkPanelCore3([]PanelGateResult{{Registration: reg, Decision: panel.Decision{Action: panel.Allow}}}, enabled, nil); err != nil {
			t.Fatal(err)
		}
	}
}

func TestComplete_Core3RecoveryOperands(t *testing.T) {
	for _, operand := range []string{"slug", "quote' ; $(echo probe)", "line\nnext"} {
		out, err := exec.Command("/bin/sh", "-c", "printf '%s' "+core3Quote(operand)).Output()
		if err != nil || string(out) != termsafe.Escape(operand) {
			t.Fatalf("shell operand changed: %q => %q (%v)", operand, out, err)
		}
	}

	reg := panel.Registration{Dir: "/review/quote ; $(echo probe)'\nnext", Panel: panel.Panel{Spec: "130-quote ; $(echo probe)'\nnext"}}
	for _, cause := range []error{errors.New("missing manifest"), &panel.UncoveredSlotError{Slot: "R1"}} {
		err := PanelCore3Failure(reg, cause)
		if !guard.HasFinalRecoveryLine(err.Error()) {
			t.Fatalf("missing recovery: %v", err)
		}
		recovery := strings.Split(err.Error(), "recovery: ")[1]
		if matches := guard.FindFloorMatches(recovery); len(matches) != 0 {
			t.Fatalf("unsafe recovery: %+v %s", matches, recovery)
		}
	}
}

func TestComplete_Core3BeforeMaterialization(t *testing.T) {
	const specID, beadID = "130-core3", "mindspec-core3.1"
	root, _ := setupPanelGateRepo(t, specID, beadID)
	newFakeMetadataStore().wire(t)
	t.Setenv(panel.SkipPanelEnv, "0")
	reg := core3Fixture(t, root, "coverage", "bead", false)
	orig := gateRevParseFn
	t.Cleanup(func() { gateRevParseFn = orig })
	gateRevParseFn = func(string, string) (string, error) { return reg.Panel.ReviewedHeadSHA, nil }
	origPreflight := completeWorkDestructionPreflightFn
	t.Cleanup(func() { completeWorkDestructionPreflightFn = origPreflight })
	reached := false
	completeWorkDestructionPreflightFn = func(workdir, branch, target, overrideReason, rerun string) error {
		reached = true
		return origPreflight(workdir, branch, target, overrideReason, rerun)
	}
	ex := &landedReviewRecorder{Executor: executor.NewMindspecExecutor(root)}
	_, err := Run(root, beadID, specID, "", ex, CompleteOpts{DocOwner: "fixture-doc-owner", AllowDocSkew: "test: fixture"})
	if err == nil || !strings.Contains(err.Error(), "no coverage manifest") {
		t.Fatalf("Run must refuse absent coverage: %v", err)
	}
	if reached {
		t.Fatal("destruction preflight reached before coverage refusal")
	}
	if len(ex.calls) != 0 {
		t.Fatalf("terminal mutation reached before coverage: %v", ex.calls)
	}
}
