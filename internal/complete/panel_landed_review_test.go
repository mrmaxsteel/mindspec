package complete

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
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
	calls []string
}

func (e *landedReviewRecorder) SetLandedReview(sha, panel string) {
	e.calls = append(e.calls, fmt.Sprintf("SetLandedReview(%s, %s)", sha, panel))
}

func (e *landedReviewRecorder) CompleteBead(_, _, _, _ string, _ bool) error {
	e.calls = append(e.calls, "CompleteBead")
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

			p := panel.Panel{BeadID: bp(beadID), Spec: specID, Round: 1,
				ExpectedReviewers: 6, ReviewedHeadSHA: reviewedSHA}
			if scenario == "abandoned" {
				p.Abandoned = true
				p.AbandonReason = "superseded"
			}
			if scenario != "no panel" {
				writePanel(t, root, "b-first-allow", p, approveVerdicts(6))
			}
			if scenario == "config hatch" {
				writeFile(t, root, ".mindspec/config.yaml", "enforcement:\n  panel_gate: false\n")
			}
			if scenario == "missing reviewed ref" {
				// The canonical panel target is absent, while the listed bead
				// worktree has a valid recovery branch for Run's later gates.
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
				writePanel(t, root, "a-warn", warn, approveVerdicts(6))
				// A later Allow has a different reviewed SHA but the same
				// subject. Both pass the real subject-tree gate; first wins.
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
				writePanel(t, root, "c-last-allow", p, approveVerdicts(6))
			}

			ex := &landedReviewRecorder{Executor: executor.NewMindspecExecutor(root)}
			res, err := Run(root, beadID, specID, "", ex, CompleteOpts{AllowDocSkew: "test: source-only fixture"})
			if err != nil || res == nil || !res.BeadClosed {
				t.Fatalf("complete must reach terminal mutation: result=%+v err=%v calls=%v", res, err, ex.calls)
			}
			want := []string{"CompleteBead"}
			if scenario == "allow" || scenario == "warn then two allows" {
				want = []string{fmt.Sprintf("SetLandedReview(%s, b-first-allow@r1)", reviewedSHA), "CompleteBead"}
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
