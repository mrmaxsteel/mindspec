package complete

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

func deferralBD(t *testing.T, epic string) string {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\ncase \"$1\" in\nlist) cat '" + dir + "/epics.json';;\nshow) cat '" + dir + "/'\"$2\"'.json';;\n*) exit 1;;\nesac\n"
	if err := os.WriteFile(filepath.Join(dir, "bd"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	core3JSON(t, dir, "epics.json", []interface{}{map[string]interface{}{"id": epic, "metadata": map[string]interface{}{"spec_num": 130, "spec_title": "core3"}}})
	setDeferredIssue(t, dir, "open")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}
func setDeferredIssue(t *testing.T, dir, change string) {
	t.Helper()
	issue := map[string]interface{}{"id": "follow-up", "status": "open", "description": "test finding /tmp/x\nsecond ask", "dependencies": []map[string]string{{"id": "epic-130-core3", "dependency_type": "discovered-from"}}}
	switch change {
	case "closed":
		issue["status"] = "closed"
	case "edge removed":
		issue["dependencies"] = []interface{}{}
	case "text removed":
		issue["description"] = "other"
	case "reparented":
		issue["parent"] = "child-x"
		core3JSON(t, dir, "child-x.json", []interface{}{map[string]interface{}{"id": "child-x", "parent": "epic-130-core3"}})
		core3JSON(t, dir, "epic-130-core3.json", []interface{}{map[string]interface{}{"id": "epic-130-core3", "status": "open"}})
	case "deleted":
		core3JSON(t, dir, "follow-up.json", []interface{}{})
		return
	case "bd error":
		if err := os.WriteFile(filepath.Join(dir, "follow-up.json"), []byte("bad json"), 0600); err != nil {
			t.Fatal(err)
		}
		return
	}
	core3JSON(t, dir, "follow-up.json", []interface{}{issue})
}
func deferredFixture(t *testing.T, root string, slotLevel bool) panel.Registration {
	t.Helper()
	reg := core3Fixture(t, root, "deferred", "bead", false)
	v := panel.FindingVerdict{Verdict: "REQUEST_CHANGES", ConcreteChangesRequired: []string{"test finding /tmp/x", "second ask"}}
	core3JSON(t, reg.Dir, "R1-round-1.json", v)
	fs, err := panel.Findings(v)
	if err != nil {
		t.Fatal(err)
	}
	if slotLevel {
		f, err := panel.SlotFinding(v)
		if err != nil {
			t.Fatal(err)
		}
		fs = []panel.Finding{f}
	}
	for _, f := range fs {
		d := panel.Deferral{Slot: "R1", Round: 1, Finding: f.ID, FindingSHA256: f.SHA256, FindingText: f.Text, Bead: "follow-up", By: "max", CreatedAt: core3Time}
		reg.Panel.Deferrals = append(reg.Panel.Deferrals, d)
		core3Append(t, reg.Dir, panel.DeferralRecord{Record: panel.RecordDeferral, Spec: reg.Panel.Spec, Panel: reg.Slug(), Deferral: d})
	}
	core3JSON(t, reg.Dir, panel.FileName, reg.Panel)
	return reg
}
func deferredResults(t *testing.T, reg panel.Registration) []PanelGateResult {
	t.Helper()
	res, err := panel.Tally(reg.Dir)
	if err != nil {
		t.Fatal(err)
	}
	d := panel.PanelGateDecision(panel.GateFacts{Reg: &reg, Res: res, HeadSHA: reg.Panel.ReviewedHeadSHA, WorktreeAbsent: true})
	if d.Action != panel.Allow {
		t.Fatalf("fixture must Allow: %+v", d)
	}
	return []PanelGateResult{{Registration: reg, Decision: d}}
}
func TestComplete_PanelDeferredAudit(t *testing.T) {
	for _, failure := range []string{"none", "pending write", "audit write", "corrupt pending", "corrupt audit"} {
		t.Run(failure, func(t *testing.T) {
			saveAndRestore(t)
			stubPhaseEpic(t, "130-core3", "epic-130-core3")
			deferralBD(t, "epic-130-core3")
			store := newFakeMetadataStore()
			store.wire(t)
			reg := deferredFixture(t, t.TempDir(), false)
			if failure == "pending write" {
				store.failMerge = func(_ string, m map[string]interface{}) bool { return m["deferral_pending_entries"] != nil }
			}
			err := checkAndRecordPanelDeferrals("reviewed-bead", deferredResults(t, reg), true)
			if failure == "pending write" {
				if err == nil {
					t.Fatal("pending failure allowed")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if store.data["reviewed-bead"]["panel_deferred"] != nil {
				t.Fatal("pending write prematurely satisfied audit")
			}
			if failure == "audit write" {
				store.failMerge = func(_ string, m map[string]interface{}) bool { return m["panel_deferred"] != nil }
			}
			if failure == "corrupt pending" {
				store.data["reviewed-bead"]["deferral_pending_entries"] = nil
			}
			if failure == "corrupt audit" {
				store.data["reviewed-bead"]["panel_deferred_entries"] = "invalid"
			}
			err = reconcilePendingDeferrals("reviewed-bead")
			if failure != "none" {
				if err == nil {
					t.Fatal("corrupt or failed audit allowed")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			entries, err := DecodePanelDeferred(store.data["reviewed-bead"], "panel_deferred_entries")
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 2 || store.data["reviewed-bead"]["panel_deferred"] != true {
				t.Fatalf("audit lost entries: %+v", store.data)
			}
			for _, entry := range entries {
				if entry.FindingText == "" || entry.FindingSHA256 == "" || entry.Bead != "follow-up" || entry.By != "max" {
					t.Fatalf("audit incomplete: %+v", entry)
				}
			}
			// A retry uses only its self-contained marker, without a registration.
			if err := reconcilePendingDeferrals("reviewed-bead"); err != nil {
				t.Fatal(err)
			}
			again, _ := DecodePanelDeferred(store.data["reviewed-bead"], "panel_deferred_entries")
			if len(again) != 2 {
				t.Fatal("audit retry duplicated entries")
			}
		})
	}
}
func TestGateTime_DeferralRecheck(t *testing.T) {
	for _, change := range []string{"reparented", "deleted", "edge removed", "text removed", "bd error"} {
		t.Run(change, func(t *testing.T) {
			saveAndRestore(t)
			stubPhaseEpic(t, "130-core3", "epic-130-core3")
			fake := deferralBD(t, "epic-130-core3")
			newFakeMetadataStore().wire(t)
			reg := deferredFixture(t, t.TempDir(), false)
			results := deferredResults(t, reg)
			if err := checkAndRecordPanelDeferrals("reviewed-bead", results, true); err != nil {
				t.Fatal(err)
			}
			setDeferredIssue(t, fake, change)
			err := checkAndRecordPanelDeferrals("reviewed-bead", results, true)
			if change == "reparented" && (err == nil || !strings.Contains(err.Error(), "outside the owning epic")) {
				t.Fatalf("reparented deferral must reach epic comparison: %v", err)
			}
			for _, want := range []string{"R1", "ccr[0]", "follow-up"} {
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("changed deferral allowed or unnamed %s: %v", change, err)
				}
			}
		})
	}
}
func TestGateTime_DeferralClosedBead(t *testing.T) {
	saveAndRestore(t)
	stubPhaseEpic(t, "130-core3", "epic-130-core3")
	fake := deferralBD(t, "epic-130-core3")
	newFakeMetadataStore().wire(t)
	reg := deferredFixture(t, t.TempDir(), false)
	setDeferredIssue(t, fake, "closed")
	err := checkAndRecordPanelDeferrals("reviewed-bead", deferredResults(t, reg), true)
	if err == nil || !strings.Contains(err.Error(), "bd reopen 'follow-up'") || !strings.Contains(err.Error(), "--finding 'ccr[0]'") || !strings.Contains(err.Error(), "mindspec panel refute") {
		t.Fatalf("closed deferral recovery: %v", err)
	}
}
func TestGateTime_RefuteSupersedesDeferral(t *testing.T) {
	for _, refKind := range []string{"same", "other finding", "different text", "slot"} {
		t.Run(refKind, func(t *testing.T) {
			saveAndRestore(t)
			stubPhaseEpic(t, "130-core3", "epic-130-core3")
			fake := deferralBD(t, "epic-130-core3")
			newFakeMetadataStore().wire(t)
			reg := deferredFixture(t, t.TempDir(), false)
			d := reg.Panel.Deferrals[0]
			// Keep the second finding resolved by a valid refutation.
			for i, entry := range reg.Panel.Deferrals {
				ref := panel.Refutation{Slot: entry.Slot, Round: entry.Round, Finding: entry.Finding, FindingSHA256: entry.FindingSHA256, FindingText: entry.FindingText, Ground: "disproven", Reason: "fixed", Evidence: "abc123", By: "max", CreatedAt: core3Time}
				if i == 0 {
					switch refKind {
					case "other finding":
						ref.Finding = "ccr[9]"
					case "different text":
						ref.FindingSHA256 = strings.Repeat("a", 64)
					case "slot":
						v, _ := panel.ReadFindingVerdict(reg.Dir, "R1", 1)
						f, _ := panel.SlotFinding(v)
						ref.Finding = f.ID
						ref.FindingSHA256 = f.SHA256
						ref.FindingText = f.Text
					}
				}
				reg.Panel.Refutations = append(reg.Panel.Refutations, ref)
				core3Append(t, reg.Dir, panel.RefutationRecord{Record: panel.RecordRefutation, Spec: reg.Panel.Spec, Panel: reg.Slug(), Refutation: ref})
			}
			core3JSON(t, reg.Dir, panel.FileName, reg.Panel)
			res, _ := panel.Tally(reg.Dir)
			setDeferredIssue(t, fake, "closed")
			err := CheckPanelDeferrals(reg, res.EffectiveDeferrals(), false)
			if (err == nil) != (refKind == "same" || refKind == "slot") {
				t.Fatalf("supersession %s: %v; deferral=%+v", refKind, err, d)
			}
		})
	}
}
func TestGateTime_SlotLevelDeferralRecovery(t *testing.T) {
	saveAndRestore(t)
	stubPhaseEpic(t, "130-core3", "epic-130-core3")
	fake := deferralBD(t, "epic-130-core3")
	newFakeMetadataStore().wire(t)
	reg := deferredFixture(t, t.TempDir(), true)
	setDeferredIssue(t, fake, "closed")
	err := checkAndRecordPanelDeferrals("reviewed-bead", deferredResults(t, reg), true)
	if err == nil || strings.Contains(err.Error(), "--finding") || !strings.Contains(err.Error(), "mindspec panel refute") {
		t.Fatalf("slot recovery must omit finding: %v", err)
	}
}
func TestGateTime_DeferralAbandonedSkipped(t *testing.T) {
	saveAndRestore(t)
	stubPhaseEpic(t, "130-core3", "epic-130-core3")
	fake := deferralBD(t, "epic-130-core3")
	newFakeMetadataStore().wire(t)
	reg := deferredFixture(t, t.TempDir(), false)
	results := deferredResults(t, reg)
	setDeferredIssue(t, fake, "closed")
	results[0].Registration.Panel.Abandoned = true
	results[0].Decision.Action = panel.Warn
	if err := checkAndRecordPanelDeferrals("reviewed-bead", results, true); err != nil {
		t.Fatal(err)
	}
	originalSkip := panelSkipEnvFn
	t.Cleanup(func() { panelSkipEnvFn = originalSkip })
	for _, enabled := range []bool{true, false} {
		panelSkipEnvFn = func() bool { return enabled }
		if err := checkAndRecordPanelDeferrals("reviewed-bead", deferredResults(t, reg), enabled); err != nil {
			t.Fatal(err)
		}
	}
}
func TestGateTime_DeferralRedactedText(t *testing.T) {
	saveAndRestore(t)
	stubPhaseEpic(t, "130-core3", "epic-130-core3")
	deferralBD(t, "epic-130-core3")
	newFakeMetadataStore().wire(t)
	reg := deferredFixture(t, t.TempDir(), false)
	if err := checkAndRecordPanelDeferrals("reviewed-bead", deferredResults(t, reg), true); err != nil {
		t.Fatal(err)
	}
}
func TestGateTime_OpenBeadDeferralResolves(t *testing.T) {
	root, _ := setupPanelGateRepo(t, "130-core3", "mindspec-core3.1")
	store := newFakeMetadataStore()
	store.wire(t)
	deferralBD(t, "epic-130-core3")
	reg := deferredFixture(t, root, false)
	orig := gateRevParseFn
	t.Cleanup(func() { gateRevParseFn = orig })
	gateRevParseFn = func(string, string) (string, error) { return reg.Panel.ReviewedHeadSHA, nil }
	core3Cover(t, reg)
	core3Manifest(t, reg)
	ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
	res, err := Run(root, "mindspec-core3.1", "130-core3", "", ex, CompleteOpts{AllowDocSkew: "test: fixture"})
	if err != nil || !ex.completeCalled || res == nil || !res.BeadClosed {
		t.Fatalf("open deferral must complete: %+v %v merge=%v", res, err, ex.completeCalled)
	}
	entries, err := DecodePanelDeferred(store.data["mindspec-core3.1"], "panel_deferred_entries")
	if err != nil || len(entries) != 2 {
		t.Fatalf("Run did not settle audit: %+v %v", entries, err)
	}
}
func TestGateTime_DeferThenCloseBlocks(t *testing.T) {
	// Build the real writer once. The subprocess writes the records under test.
	moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "mindspec")
	build := exec.Command("go", "build", "-o", binary, "./cmd/mindspec")
	build.Dir = moduleRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build writer: %v %s", err, out)
	}
	for _, change := range []string{"closed", "edge removed", "closed slot"} {
		t.Run(change, func(t *testing.T) {
			root, sha := setupPanelGateRepo(t, "130-core3", "mindspec-core3.1")
			newFakeMetadataStore().wire(t)
			fake := deferralBD(t, "epic-130-core3")
			writeFile(t, root, ".mindspec/config.yaml", "version: 1\n")
			dir := filepath.Join(root, "review", "deferred")
			if err := paneltest.CreateV1(dir, panel.CreateInput{Spec: "130-core3", Gate: "bead", Target: "bead/mindspec-core3.1", Round: 1, ReviewedHeadSHA: sha, SubjectDigest: "sha256:abc", ExpectedReviewers: 2, Roster: []panel.RosterSlot{{Slot: "R1", Model: "model-a", Lens: "correctness"}, {Slot: "R2", Model: "model-b", Lens: "tests"}}}); err != nil {
				t.Fatal(err)
			}
			core3JSON(t, dir, "R1-round-1.json", panel.FindingVerdict{Verdict: "REQUEST_CHANGES", ConcreteChangesRequired: []string{"test finding /tmp/x"}})
			core3JSON(t, dir, "R2-round-1.json", panel.FindingVerdict{Verdict: "APPROVE"})
			deferArgs := []string{"panel", "defer", "deferred", "--slot", "R1", "--round", "1", "--bead", "follow-up", "--by", "max"}
			if change != "closed slot" {
				deferArgs = append(deferArgs, "--finding", "ccr[0]")
			}
			cmd := exec.Command(binary, deferArgs...)
			cmd.Dir = root
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("write path: %v %s", err, out)
			}
			tallied, err := panel.Tally(dir)
			if err != nil || len(tallied.EffectiveDeferrals()) != 1 {
				t.Fatalf("writer did not resolve: %v %+v", err, tallied)
			}
			if err := panel.WriteCoverageManifest(tallied); err != nil {
				t.Fatal(err)
			}
			if change == "closed slot" {
				setDeferredIssue(t, fake, "closed")
			} else {
				setDeferredIssue(t, fake, change)
			}
			orig := completeWorkDestructionPreflightFn
			reached := false
			t.Cleanup(func() { completeWorkDestructionPreflightFn = orig })
			completeWorkDestructionPreflightFn = func(workdir, branch, target, overrideReason, rerun string) error {
				reached = true
				return orig(workdir, branch, target, overrideReason, rerun)
			}
			ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
			_, err = Run(root, "mindspec-core3.1", "130-core3", "", ex, CompleteOpts{AllowDocSkew: "test: fixture"})
			finding := "ccr[0]"
			if change == "closed slot" {
				finding = "slot"
			}
			if err == nil || !strings.Contains(err.Error(), "follow-up") || !strings.Contains(err.Error(), "R1") || !strings.Contains(err.Error(), finding) {
				t.Fatalf("changed deferral must block: %v", err)
			}
			if reached || ex.completeCalled {
				t.Fatal("changed deferral reached materialization or merge")
			}
			if change == "closed slot" && strings.Contains(err.Error(), "--finding") {
				t.Fatalf("slot recovery has an item flag: %v", err)
			}
			refuteArgs := []string{"panel", "refute", "deferred", "--slot", "R1", "--round", "1", "--ground", "disproven", "--evidence", "https://example.com/fix", "--reason", "fixed", "--by", "max"}
			if change != "closed slot" {
				refuteArgs = append(refuteArgs, "--finding", "ccr[0]")
			}
			refute := exec.Command(binary, refuteArgs...)
			refute.Dir = root
			if out, err := refute.CombinedOutput(); err != nil {
				t.Fatalf("refute recovery: %v %s", err, out)
			}
			completeWorkDestructionPreflightFn = orig
			res, err := Run(root, "mindspec-core3.1", "130-core3", "", ex, CompleteOpts{AllowDocSkew: "test: fixture"})
			if err != nil || res == nil || !res.BeadClosed || !ex.completeCalled {
				t.Fatalf("refute recovery must complete: %+v %v merge=%v", res, err, ex.completeCalled)
			}
		})
	}
}
func TestGateTime_DeferralRecoveryOperands(t *testing.T) {
	saveAndRestore(t)
	stubPhaseEpic(t, "130-core3", "epic-130-core3")
	fake := deferralBD(t, "epic-130-core3")
	reg := deferredFixture(t, t.TempDir(), false)
	setDeferredIssue(t, fake, "closed")
	reg.Dir = filepath.Join(filepath.Dir(reg.Dir), "quote ; $(echo probe)'\nnext")
	// Audited failures need no mutable directory. Their only recovery is reopen.
	err := CheckPanelDeferrals(reg, reg.Panel.Deferrals, true)
	if err == nil || !guard.HasFinalRecoveryLine(err.Error()) || len(guard.FindFloorMatches(finalRecoveryCommand(t, err.Error()))) != 0 {
		t.Fatalf("unsafe recovery: %v", err)
	}
	for _, id := range []string{"reviewed-bead", "quote ; $(echo probe)'\nnext"} {
		err := PanelDeferredAuditFailure(id, fmt.Errorf("metadata unavailable"))
		if !guard.HasFinalRecoveryLine(err.Error()) || len(guard.FindFloorMatches(finalRecoveryCommand(t, err.Error()))) != 0 {
			t.Fatalf("unsafe audit recovery: %v", err)
		}
	}

	raw, _ := json.Marshal(reg.Panel.Deferrals)
	if strings.Contains(string(raw), "/tmp/x") {
		t.Fatal("audit leaked path")
	}
}

func TestGateTime_DeferralAuditPreservesFindingBoundaries(t *testing.T) {
	saveAndRestore(t)
	stubPhaseEpic(t, "130-core3", "epic-130-core3")
	fake := deferralBD(t, "epic-130-core3")
	store := newFakeMetadataStore()
	store.wire(t)
	reg := deferredFixture(t, t.TempDir(), true)
	// Each finding is present, separated by other description text.
	core3JSON(t, fake, "follow-up.json", []interface{}{map[string]interface{}{"id": "follow-up", "status": "open", "description": "test finding /tmp/x\ncontext\nsecond ask", "dependencies": []map[string]string{{"id": "epic-130-core3", "dependency_type": "discovered-from"}}}})
	if err := checkAndRecordPanelDeferrals("reviewed-bead", deferredResults(t, reg), true); err != nil {
		t.Fatal(err)
	}
	if err := reconcilePendingDeferrals("reviewed-bead"); err != nil {
		t.Fatal(err)
	}
	audit, err := DecodePanelDeferred(store.data["reviewed-bead"], "panel_deferred_entries")
	if err != nil {
		t.Fatal(err)
	}
	if len(audit) != 1 || len(audit[0].FindingTexts) != 2 {
		t.Fatalf("lost finding boundaries: %+v", audit)
	}
	if err := CheckAuditedPanelDeferrals(reg, audit); err != nil {
		t.Fatal(err)
	}
}
func TestGateTime_AnotherDeferralCoversVoidedEntry(t *testing.T) {
	saveAndRestore(t)
	stubPhaseEpic(t, "130-core3", "epic-130-core3")
	fake := deferralBD(t, "epic-130-core3")
	newFakeMetadataStore().wire(t)
	reg := deferredFixture(t, t.TempDir(), true)
	v, _ := panel.ReadFindingVerdict(reg.Dir, "R1", 1)
	fs, _ := panel.Findings(v)
	for _, f := range fs {
		d := panel.Deferral{Slot: "R1", Round: 1, Finding: f.ID, FindingSHA256: f.SHA256, FindingText: f.Text, Bead: "other-followup", By: "max", CreatedAt: core3Time}
		reg.Panel.Deferrals = append(reg.Panel.Deferrals, d)
		core3Append(t, reg.Dir, panel.DeferralRecord{Record: panel.RecordDeferral, Spec: reg.Panel.Spec, Panel: reg.Slug(), Deferral: d})
	}
	core3JSON(t, reg.Dir, panel.FileName, reg.Panel)
	core3JSON(t, fake, "other-followup.json", []interface{}{map[string]interface{}{"id": "other-followup", "status": "open", "description": "test finding /tmp/x\nsecond ask", "dependencies": []map[string]string{{"id": "epic-130-core3", "dependency_type": "discovered-from"}}}})
	setDeferredIssue(t, fake, "closed")
	valid, err := checkedPanelDeferrals(reg, deferredResults(t, reg)[0].Decision.AppliedDeferrals)
	if err != nil || len(valid) != 2 {
		t.Fatalf("valid alternate discharge lost: %+v %v", valid, err)
	}
	for _, d := range valid {
		if d.Bead == "follow-up" {
			t.Fatal("voided deferral entered audit")
		}
	}
}

func TestComplete_PanelDeferredAuditReplacesStalePending(t *testing.T) {
	saveAndRestore(t)
	stubPhaseEpic(t, "130-core3", "epic-130-core3")
	fake := deferralBD(t, "epic-130-core3")
	store := newFakeMetadataStore()
	store.wire(t)
	reg := deferredFixture(t, t.TempDir(), false)
	if err := checkAndRecordPanelDeferrals("reviewed-bead", deferredResults(t, reg), true); err != nil {
		t.Fatal(err)
	}
	pending, err := DecodePanelDeferred(store.data["reviewed-bead"], "deferral_pending_entries")
	if err != nil || len(pending) != 2 {
		t.Fatalf("first run pending: %+v %v", pending, err)
	}
	// The first run stops before reconciliation. Both findings are then fixed.
	for _, d := range reg.Panel.Deferrals {
		ref := panel.Refutation{Slot: d.Slot, Round: d.Round, Finding: d.Finding, FindingSHA256: d.FindingSHA256, FindingText: d.FindingText, Ground: "disproven", Reason: "fixed", Evidence: "abc123", By: "max", CreatedAt: core3Time}
		reg.Panel.Refutations = append(reg.Panel.Refutations, ref)
		core3Append(t, reg.Dir, panel.RefutationRecord{Record: panel.RecordRefutation, Spec: reg.Panel.Spec, Panel: reg.Slug(), Refutation: ref})
	}
	core3JSON(t, reg.Dir, panel.FileName, reg.Panel)
	setDeferredIssue(t, fake, "closed")
	results := deferredResults(t, reg)
	if len(results[0].Decision.AppliedDeferrals) != 0 || len(results[0].Decision.AppliedRefutations) != 2 {
		t.Fatalf("retry must use refutations: %+v", results[0].Decision)
	}
	if err := checkAndRecordPanelDeferrals("reviewed-bead", results, true); err != nil {
		t.Fatal(err)
	}
	if err := reconcilePendingDeferrals("reviewed-bead"); err != nil {
		t.Fatal(err)
	}
	audit, err := DecodePanelDeferred(store.data["reviewed-bead"], "panel_deferred_entries")
	if err != nil {
		t.Fatal(err)
	}
	if len(audit) != 0 {
		t.Errorf("superseded pending entries entered audit: %+v", audit)
	}
	if err := CheckAuditedPanelDeferrals(reg, audit); err != nil {
		t.Errorf("retry audit must allow: %v", err)
	}
}

func TestGateTime_AuditedDeferralAlternateCoverage(t *testing.T) {
	for _, change := range []string{"slot closed", "only cover closed", "wrong finding", "wrong text", "partial item cover", "duplicate text only one cover", "duplicate text both covers", "duplicate text legacy slot one cover", "duplicate text legacy slot both covers"} {
		t.Run(change, func(t *testing.T) {
			saveAndRestore(t)
			stubPhaseEpic(t, "130-core3", "epic-130-core3")
			fake := deferralBD(t, "epic-130-core3")
			store := newFakeMetadataStore()
			store.wire(t)
			reg := deferredFixture(t, t.TempDir(), true)
			v, err := panel.ReadFindingVerdict(reg.Dir, "R1", 1)
			if err != nil {
				t.Fatal(err)
			}
			fs, err := panel.Findings(v)
			if err != nil {
				t.Fatal(err)
			}
			for i, f := range fs {
				d := panel.Deferral{Slot: "R1", Round: 1, Finding: f.ID, FindingSHA256: f.SHA256, FindingText: f.Text, Bead: fmt.Sprintf("alternate-%d", i), By: "max", CreatedAt: core3Time}
				reg.Panel.Deferrals = append(reg.Panel.Deferrals, d)
				core3Append(t, reg.Dir, panel.DeferralRecord{Record: panel.RecordDeferral, Spec: reg.Panel.Spec, Panel: reg.Slug(), Deferral: d})
				core3JSON(t, fake, d.Bead+".json", []interface{}{map[string]interface{}{"id": d.Bead, "status": "open", "description": f.Text, "dependencies": []map[string]string{{"id": "epic-130-core3", "dependency_type": "discovered-from"}}}})
			}
			core3JSON(t, reg.Dir, panel.FileName, reg.Panel)
			if err := checkAndRecordPanelDeferrals("reviewed-bead", deferredResults(t, reg), true); err != nil {
				t.Fatal(err)
			}
			if err := reconcilePendingDeferrals("reviewed-bead"); err != nil {
				t.Fatal(err)
			}
			audit, err := DecodePanelDeferred(store.data["reviewed-bead"], "panel_deferred_entries")
			if err != nil || len(audit) != 3 {
				t.Fatalf("expected slot and two items: %+v %v", audit, err)
			}
			if err := CheckAuditedPanelDeferrals(reg, audit); err != nil {
				t.Fatalf("open baseline: %v", err)
			}
			setDeferredIssue(t, fake, "closed")
			switch change {
			case "partial item cover":
				audit = audit[:2]
			case "only cover closed":
				core3JSON(t, fake, "alternate-1.json", []interface{}{map[string]interface{}{"id": "alternate-1", "status": "closed"}})
			case "wrong finding":
				// A valid item for a different finding cannot cover the failed item.
				audit = audit[1:]
				audit[0].Finding = "ccr[9]"
				core3JSON(t, fake, "alternate-1.json", []interface{}{map[string]interface{}{"id": "alternate-1", "status": "closed"}})
				audit[0].FindingText = audit[1].FindingText
				audit[0].FindingTexts = audit[1].FindingTexts
				audit[0].FindingSHA256 = audit[1].FindingSHA256
				core3JSON(t, fake, "alternate-0.json", []interface{}{map[string]interface{}{"id": "alternate-0", "status": "open", "description": audit[1].FindingText, "dependencies": []map[string]string{{"id": "epic-130-core3", "dependency_type": "discovered-from"}}}})
			case "duplicate text only one cover", "duplicate text both covers", "duplicate text legacy slot one cover", "duplicate text legacy slot both covers":
				duplicate := panel.FindingVerdict{Verdict: "REQUEST_CHANGES", ConcreteChangesRequired: []string{"second ask", "second ask"}}
				slot, err := panel.SlotFinding(duplicate)
				if err != nil {
					t.Fatal(err)
				}
				items, err := panel.Findings(duplicate)
				if err != nil {
					t.Fatal(err)
				}
				if change == "duplicate text only one cover" {
					audit = audit[:2]
				}
				audit[0].FindingText = slot.Text
				audit[0].FindingSHA256 = slot.SHA256
				audit[0].FindingTexts = []string{items[0].Text, items[1].Text}
				audit[0].Findings = items
				audit[1].FindingText = items[0].Text
				audit[1].FindingSHA256 = items[0].SHA256
				audit[1].FindingTexts = []string{items[0].Text}
				if strings.Contains(change, "legacy slot") {
					audit = audit[:2]
					audit[1].Finding = "slot"
					audit[1].FindingText = slot.Text
					audit[1].FindingSHA256 = slot.SHA256
					audit[1].Findings = nil
					if change == "duplicate text legacy slot both covers" {
						audit[1].FindingTexts = []string{items[0].Text, items[1].Text}
					}
				}
				core3JSON(t, fake, "alternate-0.json", []interface{}{map[string]interface{}{"id": "alternate-0", "status": "open", "description": items[0].Text, "dependencies": []map[string]string{{"id": "epic-130-core3", "dependency_type": "discovered-from"}}}})
			case "wrong text":
				core3JSON(t, fake, "alternate-1.json", []interface{}{map[string]interface{}{"id": "alternate-1", "status": "open", "description": "different finding", "dependencies": []map[string]string{{"id": "epic-130-core3", "dependency_type": "discovered-from"}}}})
			}
			// Approval must not reconstruct coverage from later mutable verdicts.
			core3JSON(t, reg.Dir, "R1-round-1.json", panel.FindingVerdict{Verdict: "APPROVE"})
			err = CheckAuditedPanelDeferrals(reg, audit)
			if change == "slot closed" || change == "duplicate text both covers" || change == "duplicate text legacy slot both covers" {
				if err != nil {
					t.Fatalf("valid item alternatives must cover closed slot: %v", err)
				}
			} else if err == nil {
				t.Fatal("uncovered audited finding must block")
			}
		})
	}
}

func TestGateTime_AuditedDeferralFindingIdentity(t *testing.T) {
	for _, change := range []string{"unchanged", "reordered", "raw path changed", "legacy slot"} {
		t.Run(change, func(t *testing.T) {
			saveAndRestore(t)
			stubPhaseEpic(t, "130-core3", "epic-130-core3")
			fake := deferralBD(t, "epic-130-core3")
			store := newFakeMetadataStore()
			store.wire(t)
			reg := deferredFixture(t, t.TempDir(), false)
			record := func() []PanelDeferredEntry {
				t.Helper()
				if err := checkAndRecordPanelDeferrals("reviewed-bead", deferredResults(t, reg), true); err != nil {
					t.Fatal(err)
				}
				pending, err := DecodePanelDeferred(store.data["reviewed-bead"], "deferral_pending_entries")
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range pending {
					if len(entry.Findings) != len(entry.FindingTexts) || len(entry.Findings) == 0 {
						t.Fatalf("pending must retain each finding: %+v", entry)
					}
				}
				if err := reconcilePendingDeferrals("reviewed-bead"); err != nil {
					t.Fatal(err)
				}
				audit, err := DecodePanelDeferred(store.data["reviewed-bead"], "panel_deferred_entries")
				if err != nil {
					t.Fatal(err)
				}
				return audit
			}
			if audit := record(); len(audit) != 2 {
				t.Fatalf("expected original item entries: %+v", audit)
			}
			v, err := panel.ReadFindingVerdict(reg.Dir, "R1", 1)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "reordered":
				v.ConcreteChangesRequired[0], v.ConcreteChangesRequired[1] = v.ConcreteChangesRequired[1], v.ConcreteChangesRequired[0]
			case "raw path changed", "legacy slot":
				v.ConcreteChangesRequired[0] = "test finding /tmp/y"
			}
			core3JSON(t, reg.Dir, "R1-round-1.json", v)
			f, err := panel.SlotFinding(v)
			if err != nil {
				t.Fatal(err)
			}
			d := panel.Deferral{Slot: "R1", Round: 1, Finding: f.ID, FindingSHA256: f.SHA256, FindingText: f.Text, Bead: "alternate-slot", By: "max", CreatedAt: core3Time}
			reg.Panel.Deferrals = append(reg.Panel.Deferrals, d)
			core3Append(t, reg.Dir, panel.DeferralRecord{Record: panel.RecordDeferral, Spec: reg.Panel.Spec, Panel: reg.Slug(), Deferral: d})
			core3JSON(t, reg.Dir, panel.FileName, reg.Panel)
			core3JSON(t, fake, "alternate-slot.json", []interface{}{map[string]interface{}{"id": "alternate-slot", "status": "open", "description": f.Text, "dependencies": []map[string]string{{"id": "epic-130-core3", "dependency_type": "discovered-from"}}}})
			audit := record()
			if len(audit) != 3 || len(audit[2].Findings) != 2 {
				t.Fatalf("audit must retain slot member identities: %+v", audit)
			}
			want, err := panel.Findings(v)
			if err != nil {
				t.Fatal(err)
			}
			for i, got := range audit[2].Findings {
				if got != want[i] {
					t.Fatalf("audit member %d: got %+v, want %+v", i, got, want[i])
				}
			}
			if change == "legacy slot" {
				audit[2].Findings = nil
			}
			if err := CheckAuditedPanelDeferrals(reg, audit); err != nil {
				t.Fatalf("open baseline: %v", err)
			}
			setDeferredIssue(t, fake, "closed")
			core3JSON(t, reg.Dir, "R1-round-1.json", panel.FindingVerdict{Verdict: "APPROVE"})
			err = CheckAuditedPanelDeferrals(reg, audit)
			if change == "unchanged" || change == "legacy slot" {
				if err != nil {
					t.Fatalf("matching alternative must cover original items: %v", err)
				}
			} else if err == nil {
				t.Fatal("changed slot member identity must not cover original items")
			}
		})
	}
}

func TestGateTime_AuditedDeferralScope(t *testing.T) {
	for _, scope := range []string{"other slot", "other round"} {
		t.Run(scope, func(t *testing.T) {
			saveAndRestore(t)
			stubPhaseEpic(t, "130-core3", "epic-130-core3")
			fake := deferralBD(t, "epic-130-core3")
			reg := deferredFixture(t, t.TempDir(), false)
			d := reg.Panel.Deferrals[0]
			original := PanelDeferredEntry{Deferral: d, FindingTexts: []string{d.FindingText}}
			alternative := original
			alternative.Bead = "alternate-slot"
			if scope == "other slot" {
				alternative.Slot = "R2"
			} else {
				alternative.Round = 2
			}
			core3JSON(t, fake, "alternate-slot.json", []interface{}{map[string]interface{}{"id": "alternate-slot", "status": "open", "description": d.FindingText, "dependencies": []map[string]string{{"id": "epic-130-core3", "dependency_type": "discovered-from"}}}})
			audit := []PanelDeferredEntry{original, alternative}
			if err := CheckAuditedPanelDeferrals(reg, audit); err != nil {
				t.Fatalf("open baseline: %v", err)
			}
			setDeferredIssue(t, fake, "closed")
			if err := CheckAuditedPanelDeferrals(reg, audit); err == nil {
				t.Fatal("an alternative from another slot or round must not cover the finding")
			}
		})
	}
}

// This guards the gate rejection; the completion-time currency recheck is
// exercised separately below, after a prior Allow decision.
func TestComplete_GateBlocksSupersededDeferrals(t *testing.T) {
	for _, source := range []string{panel.VerdictApprove, panel.VerdictRequestChanges} {
		t.Run(source, func(t *testing.T) {
			t.Setenv(panel.SkipPanelEnv, "0")
			root := t.TempDir()
			reg := core3Fixture(t, root, "bound-deferral", "bead", false)
			v := panel.FindingVerdict{Verdict: panel.VerdictRequestChanges, ConcreteChangesRequired: []string{"same finding"}}
			core3JSON(t, reg.Dir, "R1-round-1.json", v)
			res, err := panel.Tally(reg.Dir)
			if err != nil {
				t.Fatal(err)
			}
			f := res.Verdicts[0].Findings[0]
			d := panel.Deferral{Slot: "R1", Round: 1, Finding: f.ID, FindingSHA256: f.SHA256, FindingText: f.Text, Bead: "follow-up", By: "operator", CreatedAt: core3Time, SourceVerdict: source, SourceFileSHA256: res.Verdicts[0].FileSHA256}
			reg.Panel.Deferrals = []panel.Deferral{d}
			core3Append(t, reg.Dir, panel.DeferralRecord{Record: panel.RecordDeferral, Spec: reg.Panel.Spec, Panel: reg.Slug(), Deferral: d})
			core3JSON(t, reg.Dir, panel.FileName, reg.Panel)
			v.Rationale = "resubmitted"
			core3JSON(t, reg.Dir, "R1-round-1.json", v)
			orig := gateRevParseFn
			t.Cleanup(func() { gateRevParseFn = orig })
			gateRevParseFn = func(string, string) (string, error) { return reg.Panel.ReviewedHeadSHA, nil }
			results, err := PanelGate(*reg.Panel.BeadID, []string{root}, "", true, nil)
			if err == nil || len(results) != 1 || results[0].Decision.Action != panel.Block || !strings.Contains(err.Error(), "unresolved") {
				t.Fatalf("complete accepted ineffective %s deferral: decisions=%+v err=%v", source, results, err)
			}
		})
	}
}

// A verdict may be resubmitted after the gate produced an Allow decision.
// The completion-time recheck must discard its now audit-only deferrals.
func TestComplete_DeferralRecheckDropsSupersededSources(t *testing.T) {
	for _, source := range []string{panel.VerdictApprove, panel.VerdictRequestChanges} {
		t.Run(source, func(t *testing.T) {
			saveAndRestore(t)
			t.Setenv(panel.SkipPanelEnv, "0")
			stubPhaseEpic(t, "130-core3", "epic-130-core3")
			deferralBD(t, "epic-130-core3")
			store := newFakeMetadataStore()
			store.wire(t)
			reg := deferredFixture(t, t.TempDir(), false)
			res, err := panel.Tally(reg.Dir)
			if err != nil {
				t.Fatal(err)
			}
			d := reg.Panel.Deferrals[0]
			d.SourceVerdict, d.SourceFileSHA256 = source, res.Verdicts[0].FileSHA256
			// Preserve the source binding in both the registration and its audit.
			reg.Panel.Deferrals[0] = d
			core3Append(t, reg.Dir, panel.DeferralRecord{Record: panel.RecordDeferral, Spec: reg.Panel.Spec, Panel: reg.Slug(), Deferral: d})
			core3JSON(t, reg.Dir, panel.FileName, reg.Panel)
			v := panel.FindingVerdict{Verdict: panel.VerdictRequestChanges, ConcreteChangesRequired: []string{"test finding /tmp/x", "second ask"}, Rationale: "resubmitted after gate"}
			core3JSON(t, reg.Dir, "R1-round-1.json", v)
			results := []PanelGateResult{{Registration: reg, Decision: panel.Decision{Action: panel.Allow, AppliedDeferrals: []panel.Deferral{d}}}}
			if err := checkAndRecordPanelDeferrals("reviewed-bead", results, true); err != nil {
				t.Fatal(err)
			}
			pending, err := DecodePanelDeferred(store.data["reviewed-bead"], "deferral_pending_entries")
			if err != nil {
				t.Fatal(err)
			}
			if len(pending) != 0 {
				t.Fatalf("superseded %s source became a completion obligation: %+v", source, pending)
			}
		})
	}
}
