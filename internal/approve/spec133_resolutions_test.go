package approve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

func TestS133Resolutions_ApprovalObligations(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, state := range []string{"live", "abandoned", "deleted ledger", "replaced", "ledger only", "outside section", "id prefix", "all named"} {
			t.Run(policy+"/"+state, func(t *testing.T) {
				f := s133Document(t, policy, "spec_approve", "unresolved")
				id := f.deferFinding(false)
				if _, err := f.approve("spec_approve"); err != nil {
					t.Fatal(err)
				}
				// The epic and ledger must both contribute: the second id exists only
				// on the epic, and the first is removed from that list on ledger cases.
				f.meta["plan_deferral_ids"] = []string{"plan-epic-only"}
				if state == "deleted ledger" {
					f.meta["plan_deferral_ids"] = []string{id, "plan-epic-only"}
				}
				ledger := filepath.Join(f.dir, "reviews/spec_approve/dispositions.jsonl")
				if state != "deleted ledger" {
					preflightGit(t, f.root, "add", "-f", ledger)
					preflightGit(t, f.root, "commit", "-qm", "track spec ledger")
				}
				switch state {
				case "abandoned":
					dir := filepath.Dir(ledger)
					res, err := panel.Tally(dir)
					if err != nil {
						t.Fatal(err)
					}
					paneltest.Seed(t, dir, panel.AbandonRecord{Record: panel.RecordAbandon, Spec: s133DocID, Panel: "spec_approve", Reason: "review stopped", By: "operator", CreatedAt: paneltest.CreatedAt})
					res.Panel.Abandoned = true
					res.Panel.AbandonReason = "review stopped"
					if err := panel.WriteSnapshot(dir, *res.Panel); err != nil {
						t.Fatal(err)
					}
				case "deleted ledger":
					preflightGit(t, f.root, "add", "-f", ledger)
					preflightGit(t, f.root, "commit", "-qm", "track before deletion")
					if err := os.Rename(ledger, filepath.Join(t.TempDir(), "deleted-ledger")); err != nil {
						t.Fatal(err)
					}
				case "ledger only":
					if err := os.Rename(filepath.Join(filepath.Dir(ledger), panel.FileName), filepath.Join(t.TempDir(), "old-snapshot")); err != nil {
						t.Fatal(err)
					}
				case "replaced":
					res, err := panel.Tally(filepath.Dir(ledger))
					if err != nil {
						t.Fatal(err)
					}
					p := res.Panel
					if err := panel.Create(filepath.Dir(ledger), panel.CreateInput{Spec: s133DocID, Target: p.Target, Gate: "spec_approve", Round: 2, ReviewedHeadSHA: p.ReviewedHeadSHA, SubjectDigest: p.SubjectDigest, Roster: p.Roster}); err != nil {
						t.Fatal(err)
					}
				}
				content := strings.Replace(strings.ReplaceAll(validSingleBeadPlan, "042-test", s133DocID), "version: \"1.0\"", "version: \"1.0\"\nadr_citations: [ADR-0001]", 1)
				content += "\n## Resolutions\n\n"
				if state == "all named" {
					content += "Resolve `" + id + "` and `plan-epic-only`.\n"
				} else if state == "id prefix" {
					content += id + "-extra plan-epic-only-extra\n"
				} else {
					content += "None.\n"
				}
				if state == "outside section" {
					content += "\n## Appendix\n\n" + id + " plan-epic-only\n"
				}
				f.writePlan(content)
				f.commit("ready plan")
				f.seed("plan_approve", "pass")
				f.exec.commits = 0
				f.mutations = nil
				if state == "all named" {
					if _, err := f.approve("plan_approve"); err != nil {
						t.Fatal(err)
					}
					if f.creates != 1 {
						t.Fatal("resolution-complete plan made no bead")
					}
					return
				}
				before := f.read("plan.md")
				head := preflightGit(t, f.root, "rev-parse", "HEAD")
				_, err := f.approve("plan_approve")
				if err == nil || !strings.Contains(err.Error(), id) || !strings.Contains(err.Error(), "plan-epic-only") {
					t.Fatalf("missing obligations not named: %v", err)
				}
				if before != f.read("plan.md") || head != preflightGit(t, f.root, "rev-parse", "HEAD") || len(f.mutations) != 0 || f.closes != 0 || f.creates != 0 || f.exec.commits != 0 {
					t.Fatalf("resolution refusal mutated: %v", f.mutations)
				}
			})
		}
	}
}

func TestS133Resolutions_SectionBoundary(t *testing.T) {
	for _, text := range []string{"## Appendix\nplan-one", "## Resolutions\nNone\n## Appendix\nplan-one", "```md\n## Resolutions\nplan-one\n```", "## Resolutions\nplan-one-extra"} {
		if resolutionNamesID(planResolutionsSection(text), "plan-one") {
			t.Fatalf("accepted outside or partial id: %s", text)
		}
	}
	if !resolutionNamesID(planResolutionsSection("## Resolutions\n### Decision\nResolve `plan-one`.\n"), "plan-one") {
		t.Fatal("nested resolution did not name id")
	}
}

func TestS133Resolutions_ReApprovalRetainsAcceptedIDs(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f := s133Document(t, policy, "spec_approve", "unresolved")
			id := f.deferFinding(false)
			f.epic = true
			f.meta["plan_deferral_ids"] = []string{"plan-older-deleted-ledger"}
			if _, err := f.approve("spec_approve"); err != nil {
				t.Fatal(err)
			}
			ids, err := recordedPlanDeferralIDs(f.meta)
			if err != nil || len(ids) != 2 || !resolutionNamesID(strings.Join(ids, " "), id) || !resolutionNamesID(strings.Join(ids, " "), "plan-older-deleted-ledger") {
				t.Fatalf("accepted obligation lost: %v %v", ids, err)
			}
		})
	}
}

func TestS133Resolutions_TrackedCommitUsesSpecCheckout(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f := s133Document(t, policy, "spec_approve", "unresolved")
			id := f.deferFinding(false)
			ledger := filepath.Join(f.dir, "reviews/spec_approve/dispositions.jsonl")
			preflightGit(t, f.root, "add", "-f", ledger)
			preflightGit(t, f.root, "commit", "-qm", "tracked ledger on spec only")
			primary := filepath.Join(t.TempDir(), "primary")
			preflightGit(t, f.root, "worktree", "add", "--detach", primary, "HEAD~1")
			if err := os.Rename(filepath.Join(filepath.Dir(ledger), panel.FileName), filepath.Join(t.TempDir(), "snapshot")); err != nil {
				t.Fatal(err)
			}
			exec := executor.NewMindspecExecutor(primary)
			err := checkPlanResolutions(f.root, s133DocID, "## Resolutions\nNone.\n", nil, exec)
			if err == nil || !strings.Contains(err.Error(), id) {
				t.Fatalf("read primary HEAD instead of spec commit: %v", err)
			}
			ids, err := specPlanDeferralIDs(f.root, f.root, s133DocID, exec)
			if err != nil || len(ids) != 1 || ids[0] != id {
				t.Fatalf("ledger-only obligation lost at spec approval: %v %v", ids, err)
			}
		})
	}
}
