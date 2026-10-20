package complete

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func TestComplete_ResumeRechecksDurableDeferrals(t *testing.T) {
	for _, change := range []string{"open", "closed", "deleted", "edge removed", "text removed", "reparented", "bd error", "closed present", "refuted-other present", "abandoned-handset present", "abandoned present", "env hatch", "config hatch", "refuted", "refuted audited"} {
		t.Run(change, func(t *testing.T) { runDeferredMaterializationRetry(t, change) })
	}
}

func TestComplete_ResumePreservesLegacyDeferralIdentity(t *testing.T) {
	runDeferredMaterializationRetry(t, "legacy identity")
}

func TestComplete_ResumeSlotRefutationSupersedesItemDeferral(t *testing.T) {
	for _, change := range []string{"slot refuted", "slot refuted audited", "slot refuted-other present"} {
		t.Run(change, func(t *testing.T) { runDeferredMaterializationRetry(t, change) })
	}
}

func runDeferredMaterializationRetry(t *testing.T, change string) {
	saveAndRestore(t)
	var binary string
	if change == "abandoned present" {
		moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
		if err != nil {
			t.Fatal(err)
		}
		binary = filepath.Join(t.TempDir(), "mindspec")
		build := exec.Command("go", "build", "-o", binary, "./cmd/mindspec")
		build.Dir = moduleRoot
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build abandonment writer: %v %s", err, out)
		}
	}
	const specID, beadID = "130-core3", "mindspec-core3.1"
	root, _, branch := setupRealGitFaultFixture(t, specID, beadID)
	wireRealGitFaultSeams(t, specID)
	wt := wireBeadWorktree(t, root, specID, beadID, branch)
	stubPhaseEpic(t, specID, "epic-130-core3")
	fakeBD := deferralBD(t, "epic-130-core3")
	shim := filepath.Join(fakeBD, "bd")
	script, err := os.ReadFile(shim)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shim, []byte(strings.Replace(string(script), "*) exit 1;;", "export|update) ;;\n*) exit 1;;", 1)), 0700); err != nil {
		t.Fatal(err)
	}
	store := newFakeMetadataStore()
	store.wire(t)
	checkDirtyTreeFn = next.CheckDirtyTreeDetail
	panelSkipEnvFn = func() bool { return false }
	writeFile(t, wt, ".beads/issues.jsonl", "{\"id\":\"baseline\"}\n")
	gitRun(t, wt, "add", "--", ".beads/issues.jsonl")
	gitRun(t, wt, "commit", "-q", "-m", "baseline artifact")
	sha := gateRevParse(t, root, branch)
	dir := filepath.Join(root, "review", "deferred-resume")
	if err := panel.Create(dir, panel.CreateInput{Spec: specID, Target: branch, BeadID: bp(beadID), Gate: "bead", Round: 1, ReviewedHeadSHA: sha, SubjectDigest: "sha256:fixture", Roster: []panel.RosterSlot{{Slot: "R1", Model: "model-a", Lens: "correctness"}, {Slot: "R2", Model: "model-b", Lens: "tests"}}}); err != nil {
		t.Fatal(err)
	}
	v := panel.FindingVerdict{Verdict: "REQUEST_CHANGES", ConcreteChangesRequired: []string{"test finding /tmp/x"}}
	core3JSON(t, dir, "R1-round-1.json", v)
	core3JSON(t, dir, "R2-round-1.json", map[string]string{"verdict": "APPROVE"})
	fs, err := panel.Findings(v)
	if err != nil {
		t.Fatal(err)
	}
	d := panel.Deferral{Slot: "R1", Round: 1, Finding: fs[0].ID, FindingSHA256: fs[0].SHA256, FindingText: fs[0].Text, Bead: "follow-up", By: "max", CreatedAt: core3Time}
	core3Append(t, dir, panel.DeferralRecord{Record: panel.RecordDeferral, Spec: specID, Panel: "deferred-resume", Deferral: d})
	regs := panel.ForBead(panel.Scan(root), beadID)
	if len(regs) != 1 {
		t.Fatalf("registrations=%+v", regs)
	}
	reg := regs[0]
	reg.Panel.Deferrals = []panel.Deferral{d}
	if err := panel.WriteSnapshot(dir, reg.Panel); err != nil {
		t.Fatal(err)
	}
	core3Cover(t, reg)
	core3Manifest(t, reg)
	writeFile(t, wt, ".beads/issues.jsonl", "{\"id\":\"changed\"}\n")
	first := &landedReviewRecorder{Executor: executor.NewMindspecExecutor(root), failCompleteOnce: true}
	if _, err := Run(root, beadID, specID, "", first, CompleteOpts{}); err == nil || !strings.Contains(err.Error(), "injected completion failure") {
		t.Fatalf("expected failure after materialization; got %v calls=%v", err, first.calls)
	}
	if _, stage, present := readMarker(t, root, beadID); !present || stage != stageArtifactSync {
		t.Fatalf("no resume marker stage=%s present=%v", stage, present)
	}
	if strings.HasSuffix(change, "refuted audited") {
		// A prior attempt can settle the audit before completion fails.
		if err := reconcilePendingDeferrals(beadID); err != nil {
			t.Fatal(err)
		}
		audited, err := DecodePanelDeferred(store.data[beadID], "panel_deferred_entries")
		if err != nil || len(audited) != 1 {
			t.Fatalf("expected settled deferral before retry: %+v %v", audited, err)
		}
		store.data[beadID]["panel_refuted_entries"] = []refutationPendingEntry{{Spec: specID, Panel: "earlier-panel", Slot: "R2", Round: 1, Reason: "earlier reason", Evidence: "earlier evidence"}}
	}
	if change == "legacy identity" {
		if err := reconcilePendingDeferrals(beadID); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"deferral_pending_entries", "panel_deferred_entries"} {
			entries, err := DecodePanelDeferred(store.data[beadID], key)
			if err != nil || len(entries) != 1 {
				t.Fatalf("expected durable deferral in %s: %+v %v", key, entries, err)
			}
			entries[0].Spec, entries[0].Panel = "", ""
			store.data[beadID][key] = entries
		}
	}
	refuted := strings.Contains(change, "refuted") && !strings.Contains(change, "refuted-other")
	if change != "open" && change != "legacy identity" {
		setDeferredIssue(t, fakeBD, strings.TrimSuffix(strings.NewReplacer("abandoned-handset", "closed", "abandoned", "closed", "refuted-other", "closed").Replace(change), " present"))
	}
	if change == "env hatch" || change == "config hatch" || strings.Contains(change, "refuted") {
		setDeferredIssue(t, fakeBD, "closed")
	}
	if change == "env hatch" {
		panelSkipEnvFn = func() bool { return true }
	}
	if change == "config hatch" {
		writeFile(t, root, ".mindspec/config.yaml", "enforcement:\n  panel_gate: false\n")
		config.ResetCache()
	}
	if refuted {
		finding := fs[0]
		if strings.HasPrefix(change, "slot ") {
			finding, err = panel.SlotFinding(v)
			if err != nil {
				t.Fatal(err)
			}
		}
		ref := panel.Refutation{Slot: d.Slot, Round: d.Round, Finding: finding.ID, FindingText: finding.Text, FindingSHA256: finding.SHA256, Reason: "fixed", Ground: "disproven", Evidence: "fixing commit", By: "max", CreatedAt: core3Time}
		core3Append(t, dir, panel.RefutationRecord{Record: panel.RecordRefutation, Spec: specID, Panel: reg.Slug(), Refutation: ref})
		reg.Panel.Refutations = append(reg.Panel.Refutations, ref)
		if err := panel.WriteSnapshot(dir, reg.Panel); err != nil {
			t.Fatal(err)
		}
		res, err := panel.Tally(dir)
		if err != nil || !res.PassRule(true) || len(res.EffectiveDeferrals()) != 0 || len(res.AppliedRefutations()) != 1 {
			t.Fatalf("refutation must supersede the live deferral: %+v %v", res, err)
		}
	} else if change == "abandoned present" {
		cmd := exec.Command(binary, "panel", "abandon", reg.Slug(), "--spec", specID, "--reason", "rework", "--by", "max")
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("abandon panel: %v %s", err, out)
		}
	} else if strings.HasSuffix(change, "refuted-other present") {
		v2 := panel.FindingVerdict{Verdict: "REQUEST_CHANGES", ConcreteChangesRequired: []string{"a different finding"}}
		core3JSON(t, dir, "R1-round-1.json", v2)
		fs2, err := panel.Findings(v2)
		if err != nil {
			t.Fatal(err)
		}
		finding := fs2[0]
		if strings.HasPrefix(change, "slot ") {
			finding, err = panel.SlotFinding(v2)
			if err != nil {
				t.Fatal(err)
			}
		}
		ref := panel.Refutation{Slot: "R1", Round: 1, Finding: finding.ID, FindingText: finding.Text, FindingSHA256: finding.SHA256, Reason: "fixed", Ground: "disproven", Evidence: "fixing commit", By: "max", CreatedAt: core3Time}
		core3Append(t, dir, panel.RefutationRecord{Record: panel.RecordRefutation, Spec: specID, Panel: reg.Slug(), Refutation: ref})
		reg.Panel.Refutations = append(reg.Panel.Refutations, ref)
		if err := panel.WriteSnapshot(dir, reg.Panel); err != nil {
			t.Fatal(err)
		}
	} else if change == "abandoned-handset present" {
		reg.Panel.Abandoned = true
		reg.Panel.AbandonReason = "hand"
		if err := panel.WriteSnapshot(dir, reg.Panel); err != nil {
			t.Fatal(err)
		}
	} else if change != "closed present" {
		// The completion record must suffice even if the original registration disappears.
		if err := os.Rename(dir, filepath.Join(t.TempDir(), "saved-panel")); err != nil {
			t.Fatal(err)
		}
	}
	closedOnRetry := false
	originalClose := closeBeadFn
	closeBeadFn = func(args ...string) error { closedOnRetry = true; return originalClose(args...) }
	retry := &landedReviewRecorder{Executor: executor.NewMindspecExecutor(root)}
	result, err := Run(root, beadID, specID, "", retry, CompleteOpts{ResolveMerge: true})
	t.Logf("follow-up %s on resume: result=%+v err=%v calls=%v", change, result, err, retry.calls)
	if change == "open" || change == "legacy identity" || change == "abandoned present" || change == "env hatch" || change == "config hatch" || refuted {
		if err != nil || result == nil || !result.BeadClosed {
			t.Fatalf("retry must finish: result=%+v err=%v", result, err)
		}
		if change == "legacy identity" {
			for _, key := range []string{"deferral_pending_entries", "panel_deferred_entries"} {
				entries, err := DecodePanelDeferred(store.data[beadID], key)
				if err != nil || len(entries) != 1 || entries[0].Spec != "" || entries[0].Panel != "" {
					t.Errorf("resume changed stored identity in %s: %+v %v", key, entries, err)
				}
			}
		}
		if change == "abandoned present" {
			for _, key := range []string{"deferral_pending_entries", "panel_deferred_entries"} {
				entries, err := DecodePanelDeferred(store.data[beadID], key)
				if err != nil || len(entries) != 0 {
					t.Errorf("abandoned deferral remains in %s: %+v %v", key, entries, err)
				}
			}
		}
		if refuted {
			refs, err := decodeRefutations(store.data[beadID]["panel_refuted_entries"])
			wantRefs := 1
			if strings.HasSuffix(change, "refuted audited") {
				wantRefs++
				if len(refs) != wantRefs || refs[1].Slot != "R2" || refs[1].Reason != "earlier reason" || refs[1].Evidence != "earlier evidence" {
					t.Fatalf("resume lost the historical refutation audit: %+v %v", refs, err)
				}
			}
			finding := fs[0]
			if strings.HasPrefix(change, "slot ") {
				finding, err = panel.SlotFinding(v)
				if err != nil {
					t.Fatal(err)
				}
			}
			if err != nil || len(refs) != wantRefs || refs[0].Finding != finding.ID || refs[0].FindingSHA256 != finding.SHA256 || refs[0].Evidence != "fixing commit" {
				t.Fatalf("resume refutation lost its durable audit: %+v %v", refs, err)
			}
			for _, key := range []string{"deferral_pending_entries", "panel_deferred_entries"} {
				audited, err := DecodePanelDeferred(store.data[beadID], key)
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range audited {
					if entry.Slot == d.Slot && entry.Finding == d.Finding && entry.FindingSHA256 == d.FindingSHA256 {
						t.Errorf("refuted finding remains in %s: %+v", key, entry)
					}
				}
				if err := CheckAuditedPanelDeferrals(reg, audited); err != nil {
					t.Errorf("successful refuted retry left an active deferral in %s: %v", key, err)
				}
			}
		}
		return
	}
	if err == nil {
		t.Fatal("invalid follow-up must block completion on retry")
	}
	for _, want := range []string{"R1", "ccr[0]", "follow-up", "bd reopen"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("recovery omits %q: %v", want, err)
		}
	}
	if change == "closed present" || strings.HasSuffix(change, "refuted-other present") {
		if !strings.Contains(err.Error(), "panel refute") {
			t.Errorf("live registration must offer refutation: %v", err)
		}
	} else if !strings.HasPrefix(err.Error(), "bead "+beadID+" slot ") || !strings.HasSuffix(err.Error(), "recovery: bd reopen 'follow-up'") {
		t.Errorf("missing or invalid registration must name the bead and offer reopen only: %v", err)
	}
	if closedOnRetry {
		t.Error("blocked retry must not close the bead")
	}
	if len(retry.calls) > 0 {
		t.Fatalf("blocked retry must not complete: %v", retry.calls)
	}
}

func TestComplete_ResumeTrimsRefutedSlotMembers(t *testing.T) {
	for _, missingEffective := range []bool{false, true} {
		name := "refuted text removed"
		if missingEffective {
			name = "effective text removed"
		}
		t.Run(name, func(t *testing.T) {
			saveAndRestore(t)
			const specID, beadID = "130-core3", "mindspec-core3.1"
			root, _, branch := setupRealGitFaultFixture(t, specID, beadID)
			wireRealGitFaultSeams(t, specID)
			wt := wireBeadWorktree(t, root, specID, beadID, branch)
			stubPhaseEpic(t, specID, "epic-130-core3")
			fakeBD := deferralBD(t, "epic-130-core3")
			shim := filepath.Join(fakeBD, "bd")
			script, err := os.ReadFile(shim)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(shim, []byte(strings.Replace(string(script), "*) exit 1;;", "export|update) ;;\n*) exit 1;;", 1)), 0700); err != nil {
				t.Fatal(err)
			}
			store := newFakeMetadataStore()
			store.wire(t)
			checkDirtyTreeFn = next.CheckDirtyTreeDetail
			panelSkipEnvFn = func() bool { return false }
			writeFile(t, wt, ".beads/issues.jsonl", "{\"id\":\"baseline\"}\n")
			gitRun(t, wt, "add", "--", ".beads/issues.jsonl")
			gitRun(t, wt, "commit", "-q", "-m", "baseline artifact")
			sha := gateRevParse(t, root, branch)
			dir := filepath.Join(root, "review", "deferred-resume")
			if err := panel.Create(dir, panel.CreateInput{Spec: specID, Target: branch, BeadID: bp(beadID), Gate: "bead", Round: 1, ReviewedHeadSHA: sha, SubjectDigest: "sha256:fixture", Roster: []panel.RosterSlot{{Slot: "R1", Model: "model-a", Lens: "correctness"}, {Slot: "R2", Model: "model-b", Lens: "tests"}}}); err != nil {
				t.Fatal(err)
			}
			v := panel.FindingVerdict{Verdict: "REQUEST_CHANGES", ConcreteChangesRequired: []string{"test finding /tmp/x", "second ask"}}
			core3JSON(t, dir, "R1-round-1.json", v)
			core3JSON(t, dir, "R2-round-1.json", map[string]string{"verdict": "APPROVE"})
			fs, err := panel.Findings(v)
			if err != nil {
				t.Fatal(err)
			}
			slotFinding, err := panel.SlotFinding(v)
			if err != nil {
				t.Fatal(err)
			}
			d := panel.Deferral{Slot: "R1", Round: 1, Finding: slotFinding.ID, FindingSHA256: slotFinding.SHA256, FindingText: slotFinding.Text, Bead: "follow-up", By: "max", CreatedAt: core3Time}
			core3Append(t, dir, panel.DeferralRecord{Record: panel.RecordDeferral, Spec: specID, Panel: "deferred-resume", Deferral: d})
			regs := panel.ForBead(panel.Scan(root), beadID)
			if len(regs) != 1 {
				t.Fatalf("registrations=%+v", regs)
			}
			reg := regs[0]
			reg.Panel.Deferrals = []panel.Deferral{d}
			if err := panel.WriteSnapshot(dir, reg.Panel); err != nil {
				t.Fatal(err)
			}
			core3Cover(t, reg)
			core3Manifest(t, reg)
			writeFile(t, wt, ".beads/issues.jsonl", "{\"id\":\"changed\"}\n")
			first := &landedReviewRecorder{Executor: executor.NewMindspecExecutor(root), failCompleteOnce: true}
			if _, err := Run(root, beadID, specID, "", first, CompleteOpts{}); err == nil || !strings.Contains(err.Error(), "injected completion failure") {
				t.Fatalf("expected failure after materialization; got %v calls=%v", err, first.calls)
			}
			if _, stage, present := readMarker(t, root, beadID); !present || stage != stageArtifactSync {
				t.Fatalf("no resume marker stage=%s present=%v", stage, present)
			}
			if err := reconcilePendingDeferrals(beadID); err != nil {
				t.Fatal(err)
			}
			f := fs[0]
			ref := panel.Refutation{Slot: "R1", Round: 1, Finding: f.ID, FindingText: f.Text, FindingSHA256: f.SHA256, Ground: "disproven", Reason: "fixed", Evidence: "fix commit", By: "max", CreatedAt: core3Time}
			core3Append(t, dir, panel.RefutationRecord{Record: panel.RecordRefutation, Spec: specID, Panel: reg.Slug(), Refutation: ref})
			reg.Panel.Refutations = append(reg.Panel.Refutations, ref)
			if err := panel.WriteSnapshot(dir, reg.Panel); err != nil {
				t.Fatal(err)
			}
			description := "second ask"
			if missingEffective {
				description = "test finding /tmp/x"
			}
			core3JSON(t, fakeBD, "follow-up.json", []interface{}{map[string]interface{}{"id": "follow-up", "status": "open", "description": description, "dependencies": []map[string]string{{"id": "epic-130-core3", "dependency_type": "discovered-from"}}}})
			res, err := panel.Tally(dir)
			if err != nil {
				t.Fatal(err)
			}
			if !res.PassRule(true) {
				t.Fatal("live panel must pass")
			}

			retry := &landedReviewRecorder{Executor: executor.NewMindspecExecutor(root)}
			result, err := Run(root, beadID, specID, "", retry, CompleteOpts{ResolveMerge: true})
			t.Logf("partial slot retry: result=%+v error=%v calls=%v", result, err, retry.calls)
			if missingEffective {
				if err == nil || !strings.Contains(err.Error(), "follow-up") || len(retry.calls) > 0 {
					t.Fatalf("missing effective member must block before completion: %v calls=%v", err, retry.calls)
				}
				return
			}
			if err != nil || result == nil || !result.BeadClosed {
				t.Fatalf("retry should finish with remaining effective finding: %v", err)
			}

			for _, key := range []string{"deferral_pending_entries", "panel_deferred_entries"} {
				entries, err := DecodePanelDeferred(store.data[beadID], key)
				if err != nil || len(entries) != 1 {
					t.Fatalf("%s: %+v %v", key, entries, err)
				}
				e := entries[0]
				if e.Panel != reg.Slug() || e.Spec != specID || e.Slot != d.Slot || e.Round != d.Round || e.Finding != d.Finding || e.FindingSHA256 != d.FindingSHA256 {
					t.Fatalf("slot identity changed: %+v", e)
				}
				if len(e.Findings) != 1 || e.Findings[0] != fs[1] || len(e.FindingTexts) != 1 || e.FindingTexts[0] != fs[1].Text {
					t.Fatalf("superseded member remains: %+v", e)
				}
				if err := CheckAuditedPanelDeferrals(reg, entries); err != nil {
					t.Fatalf("persisted audit must pass: %v", err)
				}
			}
			refs, err := decodeRefutations(store.data[beadID]["panel_refuted_entries"])
			if err != nil || len(refs) != 1 || refs[0].Finding != fs[0].ID || refs[0].FindingSHA256 != fs[0].SHA256 {
				t.Fatalf("refutation audit lost: %+v %v", refs, err)
			}

		})
	}
}

func TestComplete_FreshRetryReconcilesDeferredAudit(t *testing.T) {
	const specID, beadID = "130-core3", "mindspec-core3.1"
	for _, discharge := range []string{"refuted", "abandoned", "unchanged"} {
		t.Run(discharge, func(t *testing.T) {
			root, sha := setupPanelGateRepo(t, specID, beadID)
			fake := deferralBD(t, "epic-130-core3")
			store := newFakeMetadataStore()
			store.wire(t)
			panelSkipEnvFn = func() bool { return false }
			dir := filepath.Join(root, "review", "retry-deferred")
			if err := panel.Create(dir, panel.CreateInput{Spec: specID, Target: "bead/" + beadID, Gate: "bead", Round: 1, ReviewedHeadSHA: sha, SubjectDigest: "sha256:fixture", Roster: []panel.RosterSlot{{Slot: "R1", Model: "a", Lens: "correctness"}, {Slot: "R2", Model: "b", Lens: "tests"}}}); err != nil {
				t.Fatal(err)
			}
			v := panel.FindingVerdict{Verdict: "REQUEST_CHANGES", ConcreteChangesRequired: []string{"test finding /tmp/x"}}
			core3JSON(t, dir, "R1-round-1.json", v)
			core3JSON(t, dir, "R2-round-1.json", map[string]string{"verdict": "APPROVE"})
			fs, err := panel.Findings(v)
			if err != nil {
				t.Fatal(err)
			}
			d := panel.Deferral{Slot: "R1", Round: 1, Finding: fs[0].ID, FindingSHA256: fs[0].SHA256, FindingText: fs[0].Text, Bead: "follow-up", By: "max", CreatedAt: core3Time}
			core3Append(t, dir, panel.DeferralRecord{Record: panel.RecordDeferral, Spec: specID, Panel: "retry-deferred", Deferral: d})
			reg := panel.Scan(root)[0]
			reg.Panel.Deferrals = []panel.Deferral{d}
			if err := panel.WriteSnapshot(dir, reg.Panel); err != nil {
				t.Fatal(err)
			}
			core3Cover(t, reg)
			core3Manifest(t, reg)
			closeBeadFn = func(...string) error { return errors.New("injected close failure") }
			fetchBeadByIDFn = func(id string) (next.BeadInfo, error) { return next.BeadInfo{ID: id, Status: "in_progress"}, nil }
			first := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
			_, err = Run(root, beadID, specID, "", first, CompleteOpts{AllowDocSkew: "test fixture"})
			if err == nil || !strings.Contains(err.Error(), "injected close failure") || first.completeCalled {
				t.Fatalf("wrong first failure: %v merge=%v", err, first.completeCalled)
			}
			audit, err := DecodePanelDeferred(store.data[beadID], "panel_deferred_entries")
			if err != nil || len(audit) != 1 {
				t.Fatalf("first audit: %+v %v", audit, err)
			}
			_, stage, present := readMarker(t, root, beadID)
			if present {
				t.Fatalf("fresh retry requires no materialization marker: %s", stage)
			}
			t.Logf("first attempt: close failed, merge=%v, marker=%v, settled audit=%+v", first.completeCalled, present, audit)

			setDeferredIssue(t, fake, "closed")
			if discharge == "refuted" {
				ref := panel.Refutation{Slot: d.Slot, Round: d.Round, Finding: d.Finding, FindingSHA256: d.FindingSHA256, FindingText: d.FindingText, Ground: "disproven", Reason: "fixed", Evidence: "fix commit", By: "max", CreatedAt: core3Time}
				core3Append(t, dir, panel.RefutationRecord{Record: panel.RecordRefutation, Spec: specID, Panel: reg.Slug(), Refutation: ref})
				reg.Panel.Refutations = []panel.Refutation{ref}
				if err := panel.WriteSnapshot(dir, reg.Panel); err != nil {
					t.Fatal(err)
				}
				live, err := panel.Tally(dir)
				if err != nil || !live.PassRule(true) || len(live.EffectiveDeferrals()) != 0 || len(live.AppliedRefutations()) != 1 {
					t.Fatalf("refutation must fully supersede live deferral: %+v %v", live, err)
				}
			} else if discharge == "abandoned" {
				core3Append(t, dir, panel.AbandonRecord{Record: panel.RecordAbandon, Spec: specID, Panel: reg.Slug(), Reason: "rework", By: "max", CreatedAt: core3Time})
				reg.Panel.Abandoned, reg.Panel.AbandonReason = true, "rework"
				if err := panel.WriteSnapshot(dir, reg.Panel); err != nil {
					t.Fatal(err)
				}
			}
			// Other panels and pending refutations remain independent obligations.
			other := audit[0]
			other.Panel = "other-panel"
			other.Bead = "other-follow-up"
			core3JSON(t, fake, "other-follow-up.json", []interface{}{map[string]interface{}{
				"id": "other-follow-up", "status": "open", "description": other.FindingText,
				"dependencies": []map[string]string{{"id": "epic-130-core3", "dependency_type": "discovered-from"}},
			}})
			store.data[beadID]["panel_deferred_entries"] = append(audit, other)
			store.data[beadID]["deferral_pending_entries"] = []PanelDeferredEntry{other}
			unrelated := refutationPendingEntry{Spec: specID, Panel: "earlier-panel", Slot: "R2", Round: 1, Reason: "earlier reason", Evidence: "earlier evidence"}
			store.data[beadID]["refutation_pending_entries"] = []refutationPendingEntry{unrelated}
			closeBeadFn = func(...string) error { return nil }
			fetchBeadByIDFn = func(id string) (next.BeadInfo, error) { return next.BeadInfo{ID: id, Status: "closed"}, nil }
			retry := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
			result, err := Run(root, beadID, specID, "", retry, CompleteOpts{AllowDocSkew: "test fixture"})
			if discharge == "unchanged" {
				if err == nil || !strings.Contains(err.Error(), "bd reopen") || retry.completeCalled {
					t.Fatalf("retry without discharge must block: %v merge=%v", err, retry.completeCalled)
				}
				return
			}
			if err != nil || result == nil || !result.BeadClosed || !retry.completeCalled {
				t.Fatalf("retry must complete: %+v %v merge=%v", result, err, retry.completeCalled)
			}
			audit, err = DecodePanelDeferred(store.data[beadID], "panel_deferred_entries")
			if err != nil {
				t.Fatal(err)
			}
			var own []PanelDeferredEntry
			for _, entry := range audit {
				if entry.Panel == reg.Slug() {
					own = append(own, entry)
				}
			}
			approvalReg := panel.Registration{Panel: panel.Panel{Spec: specID, BeadID: bp(beadID)}}
			gateErr := CheckAuditedPanelDeferrals(approvalReg, audit)
			if !reflect.DeepEqual(audit, []PanelDeferredEntry{other}) {
				t.Fatalf("audit must retain only the other panel: %+v", audit)
			}
			pending, pendingErr := DecodePanelDeferred(store.data[beadID], "deferral_pending_entries")
			if pendingErr != nil || !reflect.DeepEqual(pending, []PanelDeferredEntry{other}) {
				t.Fatalf("unrelated pending lost: %+v %v", pending, pendingErr)
			}
			refs, refErr := decodeRefutationAudits(store.data[beadID]["panel_refuted_entries"])
			if refErr != nil {
				t.Fatal(refErr)
			}
			foundOther, foundRef := false, false
			for _, entry := range refs {
				if entry.Panel == unrelated.Panel && entry.Reason == unrelated.Reason && entry.Evidence == unrelated.Evidence {
					foundOther = true
				}
				if entry.Panel == reg.Slug() && entry.Finding == d.Finding && entry.FindingSHA256 == d.FindingSHA256 {
					foundRef = true
				}
			}
			if !foundOther || discharge == "refuted" && !foundRef {
				t.Fatalf("refutation audits lost: %+v", refs)
			}
			t.Logf("successful fresh retry: closed=%v merge=%v audit=%+v approval deferral check=%v", result.BeadClosed, retry.completeCalled, audit, gateErr)
			if len(own) != 0 || gateErr != nil {
				t.Fatalf("superseded deferral survived successful fresh completion: entries=%+v approval=%v", audit, gateErr)
			}
		})
	}
}

func TestComplete_ResumeAuditRegistrationLedgerIdentity(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "mindspec")
	build := exec.Command("go", "build", "-o", binary, "./cmd/mindspec")
	build.Dir = filepath.Join("..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build panel writer: %v %s", err, out)
	}
	for _, swap := range []bool{false, true} {
		t.Run(fmt.Sprintf("swap=%t", swap), func(t *testing.T) {
			saveAndRestore(t)
			stubPhaseEpic(t, "130-core3", "epic-130-core3")
			fake := deferralBD(t, "epic-130-core3")
			store := newFakeMetadataStore()
			store.wire(t)
			panelSkipEnvFn = func() bool { return false }
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0700); err != nil {
				t.Fatal(err)
			}
			reg := deferredFixture(t, root, false)
			const beadID = "mindspec-core3.1"
			var entries []PanelDeferredEntry
			for _, d := range reg.Panel.Deferrals {
				entries = append(entries, PanelDeferredEntry{Deferral: d, Spec: reg.Panel.Spec, Panel: reg.Slug()})
			}
			store.data[beadID] = map[string]interface{}{"panel_deferred_entries": entries}
			roots := PanelGateRoots(root, "", reg.Panel.Spec)
			setDeferredIssue(t, fake, "closed")
			if err := recheckResumedPanelDeferrals(beadID, reg.Panel.Spec, roots, true); err == nil || !strings.Contains(err.Error(), "bd reopen") {
				t.Fatalf("live panel must block: %v", err)
			}
			abandoned := reg
			if swap {
				abandoned = core3Fixture(t, root, "throwaway", "bead", false)
			}
			cmd := exec.Command(binary, "panel", "abandon", abandoned.Slug(), "--spec", reg.Panel.Spec, "--reason", "rework", "--by", "max")
			cmd.Dir = root
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("abandon: %v %s", err, out)
			}
			if swap {
				if err := os.Rename(reg.Dir, reg.Dir+"-old"); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(abandoned.Dir, reg.Dir); err != nil {
					t.Fatal(err)
				}
			}
			err := recheckResumedPanelDeferrals(beadID, reg.Panel.Spec, roots, true)
			audit, decodeErr := DecodePanelDeferred(store.data[beadID], "panel_deferred_entries")
			if decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if swap {
				if err == nil || !strings.Contains(err.Error(), "bd reopen 'follow-up'") {
					t.Errorf("renamed abandonment must not skip the audit: %v", err)
				}
				if !reflect.DeepEqual(audit, entries) {
					t.Errorf("blocked resume changed audit: %+v; want %+v", audit, entries)
				}
				pending, pendingErr := DecodePanelDeferred(store.data[beadID], "deferral_pending_entries")
				if pendingErr != nil || len(pending) != 0 {
					t.Errorf("blocked resume changed pending audit: %+v %v", pending, pendingErr)
				}
			} else if err != nil || len(audit) != 0 {
				t.Fatalf("own verb-backed abandonment must skip the audit: %+v %v", audit, err)
			}
		})
	}
}

func TestComplete_DuplicatePanelIdentityPreservesAudit(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "mindspec")
	build := exec.Command("go", "build", "-o", binary, "./cmd/mindspec")
	build.Dir = filepath.Join("..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build panel writer: %v %s", err, out)
	}
	for _, attempt := range []string{"resume-review", "resume-reviews", "pending", "settle"} {
		t.Run(attempt, func(t *testing.T) {
			saveAndRestore(t)
			stubPhaseEpic(t, "130-core3", "epic-130-core3")
			fake := deferralBD(t, "epic-130-core3")
			store := newFakeMetadataStore()
			store.wire(t)
			panelSkipEnvFn = func() bool { return false }
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0700); err != nil {
				t.Fatal(err)
			}
			reg := deferredFixture(t, root, false)
			beadID := *reg.Panel.BeadID
			var entries []PanelDeferredEntry
			for _, d := range reg.Panel.Deferrals {
				entries = append(entries, PanelDeferredEntry{Deferral: d, Spec: reg.Panel.Spec, Panel: reg.Slug()})
			}
			pending := entries
			if attempt == "settle" {
				pending = []PanelDeferredEntry{}
			}
			store.data[beadID] = map[string]interface{}{"panel_deferred_entries": entries, "deferral_pending_entries": pending}
			roots := PanelGateRoots(root, "", reg.Panel.Spec)
			setDeferredIssue(t, fake, "closed")
			if err := recheckResumedPanelDeferrals(beadID, reg.Panel.Spec, roots, true); err == nil || !strings.Contains(err.Error(), "bd reopen") {
				t.Fatalf("closed follow-up must block before the copy: %v", err)
			}
			live := reg.Dir
			aside := filepath.Join(t.TempDir(), "live")
			if err := os.Rename(live, aside); err != nil {
				t.Fatal(err)
			}
			if err := os.CopyFS(live, os.DirFS(aside)); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(binary, "panel", "abandon", reg.Slug(), "--spec", reg.Panel.Spec, "--reason", "rework", "--by", "max")
			cmd.Dir = root
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("abandon byte copy: %v %s", err, out)
			}
			segment := "review"
			if attempt == "resume-reviews" {
				segment = "reviews"
			}
			duplicate := filepath.Join(root, ".mindspec", "docs", "specs", reg.Panel.Spec, segment, reg.Slug())
			if err := os.MkdirAll(filepath.Dir(duplicate), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(live, duplicate); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(aside, live); err != nil {
				t.Fatal(err)
			}
			regs := panel.Scan(roots...)
			if len(regs) != 2 {
				t.Fatalf("want live and abandoned registrations: %+v", regs)
			}
			var results []PanelGateResult
			for _, current := range regs {
				field, err := panel.CheckHandEdits(current.Dir, current.Panel)
				if err != nil || field != "" || !PanelDeferredRegistrationMatches(current, reg.Panel.Spec, reg.Slug()) {
					t.Fatalf("copy must have intact ledger identity: %+v %q %v", current, field, err)
				}
				results = append(results, PanelGateResult{Registration: current, Decision: panel.Decision{Action: panel.Allow}})
			}
			if strings.HasPrefix(attempt, "resume-") {
				err := recheckResumedPanelDeferrals(beadID, reg.Panel.Spec, roots, true)
				for _, want := range []string{"panel " + reg.Slug() + " for spec " + reg.Panel.Spec + " is registered at 2 paths:", live, duplicate, "remove the duplicates"} {
					if err == nil || !strings.Contains(err.Error(), want) {
						t.Errorf("duplicate identity must block and name both paths; want %q: %v", want, err)
					}
				}
			} else if err := replaceGatedDeferrals(beadID, results, nil, attempt == "settle"); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"panel_deferred_entries", "deferral_pending_entries"} {
				audit, err := DecodePanelDeferred(store.data[beadID], key)
				want := entries
				if key == "deferral_pending_entries" {
					want = pending
				}
				if err != nil || !reflect.DeepEqual(audit, want) {
					t.Errorf("duplicate identity changed %s: %+v %v; want %+v", key, audit, err, want)
				}
			}
			if strings.HasPrefix(attempt, "resume-") {
				if err := os.Rename(live, filepath.Join(t.TempDir(), "live")); err != nil {
					t.Fatal(err)
				}
				if err := recheckResumedPanelDeferrals(beadID, reg.Panel.Spec, roots, true); err != nil {
					t.Fatalf("single abandoned registration must skip: %v", err)
				}
				for _, key := range []string{"panel_deferred_entries", "deferral_pending_entries"} {
					audit, err := DecodePanelDeferred(store.data[beadID], key)
					if err != nil || len(audit) != 0 {
						t.Errorf("single abandoned registration retains %s: %+v %v", key, audit, err)
					}
				}
			}
		})
	}
}

func TestComplete_ResumeMissingPanelNamesBeadRecovery(t *testing.T) {
	saveAndRestore(t)
	stubPhaseEpic(t, "130-core3", "epic-130-core3")
	fake := deferralBD(t, "epic-130-core3")
	setDeferredIssue(t, fake, "closed")
	store := newFakeMetadataStore()
	store.wire(t)
	panelSkipEnvFn = func() bool { return false }
	root := t.TempDir()
	reg := deferredFixture(t, root, false)
	beadID := *reg.Panel.BeadID
	entry := PanelDeferredEntry{Deferral: reg.Panel.Deferrals[0], Spec: reg.Panel.Spec, Panel: reg.Slug()}
	store.data[beadID] = map[string]interface{}{"panel_deferred_entries": []PanelDeferredEntry{entry}}
	if err := os.Rename(reg.Dir, filepath.Join(t.TempDir(), "saved-panel")); err != nil {
		t.Fatal(err)
	}
	err := recheckResumedPanelDeferrals(beadID, reg.Panel.Spec, []string{root}, true)
	if err == nil || !strings.HasPrefix(err.Error(), "bead "+beadID+" slot R1 finding ccr[0] follow-up follow-up:") || !strings.HasSuffix(err.Error(), "recovery: bd reopen 'follow-up'") || strings.Contains(err.Error(), "panel refute") {
		t.Fatalf("missing registration must name the bead and offer reopen only: %v", err)
	}
}
