package complete

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// firstGateRegistration keeps the pre-existing advisory/audit tests focused on
// the first registration; the decision collection itself is tested below.
func firstGateRegistration(matches []PanelGateResult) *panel.Registration {
	if len(matches) == 0 {
		return nil
	}
	return &matches[0].Registration
}

type landedReviewRecorder struct {
	executor.Executor
	calls            []string
	failCompleteOnce bool
}

func (e *landedReviewRecorder) SetLandedReview(sha, panel string) {
	e.calls = append(e.calls, fmt.Sprintf("SetLandedReview(%s, %s)", sha, panel))
}

func (e *landedReviewRecorder) CompleteBead(_, _, _, _ string, _ bool) error {
	e.calls = append(e.calls, "CompleteBead")
	if e.failCompleteOnce {
		e.failCompleteOnce = false
		return errors.New("injected completion failure")
	}
	return nil
}

func TestComplete_LandedReviewOnAllowOnly(t *testing.T) {
	for _, scenario := range []string{"allow", "no panel", "abandoned", "env hatch", "config hatch", "missing reviewed ref", "warn then two allows"} {
		t.Run(scenario, func(t *testing.T) {
			const specID, beadID = "130-landed", "mindspec-130landed.1"
			root, reviewedSHA := setupPanelGateRepo(t, specID, beadID)
			newFakeMetadataStore().wire(t)
			origSkip := panelSkipEnvFn
			t.Cleanup(func() { panelSkipEnvFn = origSkip })
			panelSkipEnvFn = func() bool { return scenario == "env hatch" }

			p := panel.Panel{BeadID: bp(beadID), Spec: specID, Round: 3,
				ExpectedReviewers: 6, ReviewedHeadSHA: reviewedSHA}
			if scenario == "abandoned" {
				p.Abandoned = true
				p.AbandonReason = "superseded"
			}
			if scenario != "no panel" {
				writePanel(t, root, "b-first-allow", p, landedReviewVerdicts(p.Round))
			}
			if scenario == "config hatch" {
				writeFile(t, root, ".mindspec/config.yaml", "enforcement:\n  panel_gate: false\n")
			}
			if scenario == "missing reviewed ref" {
				// R3.0 path 1 selects branch-less reconcile when the canonical
				// branch is absent, even with a listed recovery worktree.
				gitRun(t, root, "checkout", "-q", "bead/"+beadID)
				gitRun(t, root, "branch", "-m", "bead/mindspec-recovery.1")
				worktreeListFn = func() ([]bead.WorktreeListEntry, error) {
					return []bead.WorktreeListEntry{{Name: "worktree-" + beadID, Path: root, Branch: "bead/mindspec-recovery.1"}}, nil
				}
			}
			if scenario == "warn then two allows" {
				warn := p
				warn.Abandoned = true
				warn.AbandonReason = "old panel"
				writePanel(t, root, "a-warn", warn, landedReviewVerdicts(warn.Round))
				// The first Allow reviews a SHA different from the bead tip.
				// Both Allows have the same subject; the first panel SHA wins.
				gitRun(t, root, "checkout", "-q", "--detach", reviewedSHA)
				writeFile(t, root, "review/artifact.txt", "review-only commit\n")
				gitRun(t, root, "add", "review/artifact.txt")
				gitRun(t, root, "commit", "-q", "-m", "review artifacts")
				laterSHA := gateRevParse(t, root, "HEAD")
				gitRun(t, root, "checkout", "-q", "main")
				reader := panel.SubjectTreeReader(executor.NewMindspecExecutor(root).LsTree)
				subject := panel.Subject("", filepath.Join(root, ".mindspec/specs", specID))
				digest, err := reader(root, reviewedSHA, subject)
				if err != nil {
					t.Fatal(err)
				}
				p.ReviewedHeadSHA, p.SubjectDigest = laterSHA, digest
				writePanel(t, root, "b-first-allow", p, landedReviewVerdicts(p.Round))
				p.ReviewedHeadSHA = reviewedSHA
				writePanel(t, root, "c-last-allow", p, landedReviewVerdicts(p.Round))
				reviewedSHA = laterSHA
				if tip := gateRevParse(t, root, "bead/"+beadID); tip == reviewedSHA {
					t.Fatal("first Allow must review a SHA different from the bead tip")
				}
			}

			ex := &landedReviewRecorder{Executor: executor.NewMindspecExecutor(root)}
			closed := false
			closeBeadFn = func(...string) error { closed = true; return nil }
			res, err := Run(root, beadID, specID, "", ex, CompleteOpts{DocOwner: "fixture-doc-owner", AllowDocSkew: "test: source-only fixture"})
			if scenario == "missing reviewed ref" {
				if err == nil || !strings.Contains(err.Error(), "no landed merge commit") || !guard.HasFinalRecoveryLine(err.Error()) {
					t.Fatalf("branch-less reconcile without landed evidence must refuse with recovery: result=%+v err=%v", res, err)
				}
				if res != nil || closed || len(ex.calls) != 0 {
					t.Fatalf("refusal must precede close, review binding and completion: result=%+v closed=%v calls=%v", res, closed, ex.calls)
				}
				if tip := gateRevParse(t, root, "bead/mindspec-recovery.1"); tip != reviewedSHA {
					t.Fatalf("refusal must keep the recovery branch: got %s, want %s", tip, reviewedSHA)
				}
				return
			}
			if err != nil || res == nil || !res.BeadClosed {
				t.Fatalf("complete must reach terminal mutation: result=%+v err=%v calls=%v", res, err, ex.calls)
			}
			want := []string{"CompleteBead"}
			if scenario == "allow" || scenario == "warn then two allows" {
				want = []string{fmt.Sprintf("SetLandedReview(%s, b-first-allow@r3)", reviewedSHA), "CompleteBead"}
			}
			if !reflect.DeepEqual(ex.calls, want) {
				t.Fatalf("review setter must run exactly once before CompleteBead, only for the first Allow: got %v, want %v", ex.calls, want)
			}
		})
	}
}

func TestPanelGate_ReturnsEveryMatchedDecision(t *testing.T) {
	const specID, beadID = "130-decisions", "mindspec-130decisions.1"
	root, sha := setupPanelGateRepo(t, specID, beadID)
	origSkip := panelSkipEnvFn
	t.Cleanup(func() { panelSkipEnvFn = origSkip })
	panelSkipEnvFn = func() bool { return false }
	for _, slug := range []string{"a-warn", "b-allow", "c-allow"} {
		writePanel(t, root, slug, panel.Panel{BeadID: bp(beadID), Spec: specID, Round: 1,
			ExpectedReviewers: 6, ReviewedHeadSHA: sha, Abandoned: slug == "a-warn"}, approveVerdicts(6))
	}
	for _, enabled := range []bool{true, false} {
		matches, err := panelGate(beadID, []string{root}, "", enabled, nil)
		if err != nil || len(matches) != 3 {
			t.Fatalf("expected every registration, enabled=%v: matches=%+v err=%v", enabled, matches, err)
		}
		for i, slug := range []string{"a-warn", "b-allow", "c-allow"} {
			want := panel.Allow
			if i == 0 || !enabled {
				want = panel.Warn
			}
			if matches[i].Registration.Slug() != slug || matches[i].Decision.Action != want {
				t.Errorf("scan-order decision %d: got %+v, want %s %v", i, matches[i], slug, want)
			}
		}
	}
}

// Use verdict filenames matching the registration's round so the real gate Allows.
func landedReviewVerdicts(round int) map[string]string {
	verdicts := map[string]string{}
	for name, verdict := range approveVerdicts(6) {
		verdicts[strings.Replace(name, "-round-1.json", fmt.Sprintf("-round-%d.json", round), 1)] = verdict
	}
	return verdicts
}

func TestComplete_LandedReviewResume(t *testing.T) {
	for _, scenario := range []string{"allow", "no recorded identity", "legacy attestation"} {
		t.Run(scenario, func(t *testing.T) {
			saveAndRestore(t)
			const specID, beadID = "930-review-resume", "mindspec-reviewresume.1"
			root, _, beadBranch := setupRealGitFaultFixture(t, specID, beadID)
			wireRealGitFaultSeams(t, specID)
			wtPath := wireBeadWorktree(t, root, specID, beadID, beadBranch)
			store := newFakeMetadataStore()
			store.wire(t)
			if scenario == "no recorded identity" {
				// A fresh no-panel publication must overwrite an older marker's identity.
				store.data[beadID] = map[string]interface{}{
					"mindspec_stage_marker_oid":          "old-marker",
					"mindspec_stage_marker_reviewed_sha": "old-reviewed-sha",
					"mindspec_stage_marker_review_panel": "old-panel@r8",
				}
			}
			checkDirtyTreeFn = next.CheckDirtyTreeDetail
			origSkip := panelSkipEnvFn
			t.Cleanup(func() { panelSkipEnvFn = origSkip })
			panelSkipEnvFn = func() bool { return false }
			writeFile(t, wtPath, ".beads/issues.jsonl", "{\"id\":\"baseline\"}\n")
			gitRun(t, wtPath, "add", "--", ".beads/issues.jsonl")
			gitRun(t, wtPath, "commit", "-q", "-m", "baseline artifact")
			reviewedSHA := gateRevParse(t, root, beadBranch)
			const slug = "resume-allow"
			if scenario != "no recorded identity" {
				writePanel(t, root, slug, panel.Panel{BeadID: bp(beadID), Spec: specID, Round: 1,
					ExpectedReviewers: 6, ReviewedHeadSHA: reviewedSHA}, approveVerdicts(6))
			}
			writeFile(t, wtPath, ".beads/issues.jsonl", "{\"id\":\"changed\"}\n")
			first := &landedReviewRecorder{Executor: executor.NewMindspecExecutor(root), failCompleteOnce: true}
			if _, err := Run(root, beadID, specID, "", first, CompleteOpts{}); err == nil {
				t.Fatal("first completion must fail after full materialization")
			}
			want := []string{"CompleteBead"}
			if scenario != "no recorded identity" {
				want = []string{fmt.Sprintf("SetLandedReview(%s, %s@r1)", reviewedSHA, slug), "CompleteBead"}
			}
			if !reflect.DeepEqual(first.calls, want) {
				t.Fatalf("first completion calls: got %v, want %v", first.calls, want)
			}
			tip := gateRevParse(t, root, beadBranch)
			if oid, stage, present := readMarker(t, root, beadID); !present || oid != tip || tip == reviewedSHA || stage != stageArtifactSync {
				t.Fatalf("expected advanced, attested artifact-sync marker: oid=%s tip=%s reviewed=%s stage=%s present=%v", oid, tip, reviewedSHA, stage, present)
			}
			if scenario == "legacy attestation" {
				// Older publishers recorded only the marker OID.
				store.data[beadID] = map[string]interface{}{stageMarkerAttestationKey: tip}
				want = []string{"CompleteBead"}
			}
			retry := &landedReviewRecorder{Executor: executor.NewMindspecExecutor(root)}
			result, err := Run(root, beadID, specID, "", retry, CompleteOpts{ResolveMerge: true})
			if err != nil || result == nil || !result.BeadClosed {
				t.Fatalf("resume must converge: result=%+v err=%v calls=%v", result, err, retry.calls)
			}
			if !reflect.DeepEqual(retry.calls, want) {
				t.Fatalf("resume must replay the original panel identity before CompleteBead: got %v, want %v", retry.calls, want)
			}
		})
	}
}
