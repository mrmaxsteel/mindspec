package complete

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func TestComplete_ResumeRechecksDurableDeferrals(t *testing.T) {
	for _, change := range []string{"open", "closed", "deleted", "edge removed", "text removed", "reparented", "bd error", "closed present", "refuted-other present", "abandoned-handset present", "env hatch", "config hatch", "refuted", "refuted audited"} {
		t.Run(change, func(t *testing.T) { runDeferredMaterializationRetry(t, change) })
	}
}

func runDeferredMaterializationRetry(t *testing.T, change string) {
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
	if change == "refuted audited" {
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
	if change != "open" {
		setDeferredIssue(t, fakeBD, strings.TrimSuffix(strings.NewReplacer("abandoned-handset", "closed", "refuted-other", "closed").Replace(change), " present"))
	}
	if change == "env hatch" || change == "config hatch" || change == "refuted" || change == "refuted audited" {
		setDeferredIssue(t, fakeBD, "closed")
	}
	if change == "env hatch" {
		panelSkipEnvFn = func() bool { return true }
	}
	if change == "config hatch" {
		writeFile(t, root, ".mindspec/config.yaml", "enforcement:\n  panel_gate: false\n")
		config.ResetCache()
	}
	if change == "refuted" || change == "refuted audited" {
		ref := panel.Refutation{Slot: d.Slot, Round: d.Round, Finding: d.Finding, FindingText: d.FindingText, FindingSHA256: d.FindingSHA256, Reason: "fixed", Ground: "disproven", Evidence: "fixing commit", By: "max", CreatedAt: core3Time}
		core3Append(t, dir, panel.RefutationRecord{Record: panel.RecordRefutation, Spec: specID, Panel: reg.Slug(), Refutation: ref})
		reg.Panel.Refutations = append(reg.Panel.Refutations, ref)
		if err := panel.WriteSnapshot(dir, reg.Panel); err != nil {
			t.Fatal(err)
		}
	} else if change == "refuted-other present" {
		v2 := panel.FindingVerdict{Verdict: "REQUEST_CHANGES", ConcreteChangesRequired: []string{"a different finding"}}
		core3JSON(t, dir, "R1-round-1.json", v2)
		fs2, err := panel.Findings(v2)
		if err != nil {
			t.Fatal(err)
		}
		ref := panel.Refutation{Slot: "R1", Round: 1, Finding: fs2[0].ID, FindingText: fs2[0].Text, FindingSHA256: fs2[0].SHA256, Reason: "fixed", Ground: "disproven", Evidence: "fixing commit", By: "max", CreatedAt: core3Time}
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
	if change == "open" || change == "env hatch" || change == "config hatch" || change == "refuted" || change == "refuted audited" {
		if err != nil || result == nil || !result.BeadClosed {
			t.Fatalf("open control must finish: result=%+v err=%v", result, err)
		}
		if change == "refuted" || change == "refuted audited" {
			refs, err := decodeRefutations(store.data[beadID]["panel_refuted_entries"])
			wantRefs := 1
			if change == "refuted audited" {
				wantRefs++
				if len(refs) != wantRefs || refs[1].Slot != "R2" || refs[1].Reason != "earlier reason" || refs[1].Evidence != "earlier evidence" {
					t.Fatalf("resume lost the historical refutation audit: %+v %v", refs, err)
				}
			}
			if err != nil || len(refs) != wantRefs || refs[0].Finding != d.Finding || refs[0].FindingSHA256 != d.FindingSHA256 || refs[0].Evidence != "fixing commit" {
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
	for _, want := range []string{"R1", "ccr[0]", "follow-up", "bd reopen", "panel refute"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("recovery omits %q: %v", want, err)
		}
	}
	if closedOnRetry {
		t.Error("blocked retry must not close the bead")
	}
	if len(retry.calls) > 0 {
		t.Fatalf("blocked retry must not complete: %v", retry.calls)
	}
}
