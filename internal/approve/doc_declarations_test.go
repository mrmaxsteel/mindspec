package approve

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/complete"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/validate"
)

func s133DocFinalFixture(t *testing.T, policy string, count int, declared []string, ownerDocs bool, deferralOwner string, direct bool, legacy bool, directDocs ...bool) *s133FinalFixture {
	t.Helper()
	f := s133DocFinalChangesFixture(t, policy, count, declared, ownerDocs, deferralOwner, direct, legacy, directDocs...)
	s133SeedPanel(t, f, "final_review", "spec/010-test", "", "pass", "final")
	s133Commit(t, f.specWT, "final review ledger")
	return f
}

func s133DocFinalChangesFixture(t *testing.T, policy string, count int, declared []string, ownerDocs bool, deferralOwner string, direct bool, legacy bool, directDocs ...bool) *s133FinalFixture {
	f := s133FinalFixtureNew(t, policy, count, "absent", func(f *s133FinalFixture, ids []string) {
		for _, id := range ids {
			wt := filepath.Join(f.root, ".worktrees", "doc-"+id)
			preflightGit(t, f.root, "worktree", "add", wt, "bead/"+id)
			if id == "bead-1" || !ownerDocs {
				s133Write(t, wt, "internal/widget/"+id+".go", "package widget\n")
			}
			if ownerDocs && id == "bead-2" {
				s133Write(t, wt, ".mindspec/domains/workflow/interfaces.md", "Updated workflow docs.\n")
			}
			s133Commit(t, wt, "domain work "+id)
			preflightGit(t, f.root, "worktree", "remove", wt)
		}
		for _, id := range declared {
			if err := f.metadata(id, map[string]interface{}{validate.NoDocImpactKey: []validate.DocDeclaration{{Domain: "workflow", Reason: "internal change", By: "reviewer", At: "2026-01-01T00:00:00Z"}}}); err != nil {
				t.Fatal(err)
			}
		}
		if deferralOwner != "" {
			d := validate.DocDeferral{ID: "doc-obligation", Domains: []string{"workflow"}, Source: "bead-1", Owner: deferralOwner, Reason: "doc follow-up", By: "reviewer", At: "2026-01-01T00:00:00Z"}
			for _, id := range []string{"bead-1", "bead-2"} {
				if err := f.metadata(id, map[string]interface{}{validate.DocDeferralsKey: []validate.DocDeferral{d}}); err != nil {
					t.Fatal(err)
				}
			}
		}
		if legacy {
			if err := f.metadata("bead-1", map[string]interface{}{"mindspec_doc_skew_reason": "old override", "mindspec_doc_skew_by": "actor", "mindspec_doc_skew_at": "2026-01-01T00:00:00Z"}); err != nil {
				t.Fatal(err)
			}
		}
	})
	if direct {
		s133Write(t, f.specWT, "internal/widget/bead-1.go", "package widget\nconst Direct = 1\n")
		s133Commit(t, f.specWT, "direct spec source change")
	}
	if len(directDocs) > 0 && directDocs[0] {
		s133Write(t, f.specWT, ".mindspec/domains/workflow/interfaces.md", "Docs from a direct spec commit.\n")
		s133Commit(t, f.specWT, "direct docs do not resolve the owner obligation")
	}
	return f
}

func TestS133NoDocImpact_ImplRequiresEverySourceMergeDeclaration(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, tc := range []struct {
			name     string
			count    int
			declared []string
			direct   bool
			pass     bool
		}{
			{"single-declared", 1, []string{"bead-1"}, false, true},
			{"two-one-declared", 2, []string{"bead-1"}, false, false},
			{"two-all-declared", 2, []string{"bead-1", "bead-2"}, false, true},
			{"direct-spec-commit", 2, []string{"bead-1", "bead-2"}, true, false},
		} {
			t.Run(policy+"/"+tc.name, func(t *testing.T) {
				f := s133DocFinalFixture(t, policy, tc.count, tc.declared, false, "", tc.direct, false)
				if !tc.pass {
					f.refuse("doc-sync:", ImplOpts{})
					return
				}
				if _, err := f.approve(); err != nil {
					t.Fatal(err)
				}
				f.assertConverged()
			})
		}
	}
}

func TestS133DocOwner_ImplOpenResolvedTransferredAndOverride(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, action := range []string{"open", "other-docs", "resolved", "transferred", "override"} {
			t.Run(policy+"/"+action, func(t *testing.T) {
				owner := "bead-2"
				if action == "transferred" {
					owner = "new-doc-owner"
				}
				f := s133DocFinalFixture(t, policy, 2, nil, action == "resolved", owner, false, false, action == "other-docs")
				if action == "open" || action == "other-docs" || action == "transferred" {
					err := f.refuse("open doc deferrals:", ImplOpts{})
					if len(guard.FindFloorMatches("git reset --hard HEAD")) == 0 {
						t.Fatal("floor detector did not fire")
					}
					if !guard.HasFinalRecoveryLine(err.Error()) {
						t.Fatal(err)
					}
					for _, line := range strings.Split(err.Error(), "\n") {
						if strings.HasPrefix(line, "recovery:") && len(guard.FindFloorMatches(strings.TrimPrefix(line, "recovery:"))) > 0 {
							t.Fatalf("destructive recovery: %s", line)
						}
					}
					for _, value := range []string{"doc-obligation", owner, "workflow"} {
						if !strings.Contains(err.Error(), value) {
							t.Fatalf("missing %s: %v", value, err)
						}
					}
					return
				}
				opts := ImplOpts{}
				if action == "override" {
					opts.AllowDocSkew = "reviewed doc debt"
				}
				if _, err := ApproveImpl(f.root, "010-test", f.exec, opts); err != nil {
					t.Fatal(err)
				}
				f.assertConverged()
				if action == "override" {
					meta := f.read()["epic-parent"].Metadata
					if meta["mindspec_impl_skew_reason"] != "reviewed doc debt" || !strings.Contains(fmt.Sprint(meta["mindspec_impl_doc_deferrals"]), "doc-obligation") {
						t.Fatalf("missing epic override audit: %+v", meta)
					}
				}
			})
		}
	}
}

func TestS133DocOwner_LegacyOwnerlessSkewUsesBranchDocGate(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, docs := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/docs=%t", policy, docs), func(t *testing.T) {
				f := s133DocFinalFixture(t, policy, 2, nil, docs, "", false, true)
				if !docs {
					err := f.refuse("doc-sync:", ImplOpts{})
					if strings.Contains(err.Error(), "open doc deferrals") {
						t.Fatalf("legacy record became owned deferral: %v", err)
					}
					return
				}
				if _, err := f.approve(); err != nil {
					t.Fatal(err)
				}
				f.assertConverged()
			})
		}
	}
}

func TestS133DocOwner_OwnerOutsideSourceSetStillResolves(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f := s133DocFinalFixture(t, policy, 2, nil, true, "bead-2", false, false)
			base, err := f.exec.MergeBase("main", "spec/010-test")
			if err != nil {
				t.Fatal(err)
			}
			docs := validate.ValidateDocsRange(f.root, base, "spec/010-test", "spec/010-test", f.exec)
			open, err := implDocGate(f.root, base, "spec/010-test", "010-test", f.exec, []string{"bead-1"}, docs, "")
			if err != nil || len(open) != 0 {
				t.Fatalf("landed owner docs did not resolve deferral: %+v %v", open, err)
			}
		})
	}
}

func TestS133DocOwner_FollowUpSourceDeferralStillBlocks(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f := s133DocFinalFixture(t, policy, 2, nil, false, "new-doc-owner", false, false, true)
			rows, err := validate.ReadDocRecords[validate.DocDeferral](f.read()["bead-1"].Metadata, validate.DocDeferralsKey)
			if err != nil || len(rows) != 1 {
				t.Fatalf("fixture deferral: %+v %v", rows, err)
			}
			rows[0].Source = "bead-2"
			for _, id := range []string{"bead-1", "bead-2"} {
				if err := f.metadata(id, map[string]interface{}{validate.DocDeferralsKey: rows}); err != nil {
					t.Fatal(err)
				}
			}
			t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
				if contains(args, "--type=epic") {
					return json.Marshal([]phase.EpicInfo{f.read()["epic-parent"]})
				}
				return json.Marshal([]phase.ChildInfo{{ID: "bead-1", Parent: "epic-parent", Status: "closed", IssueType: "task"}, {ID: "bead-2", Parent: "epic-parent", Status: "closed", IssueType: "bug"}})
			}))
			err = f.refuse("open doc deferrals:", ImplOpts{})
			if !strings.Contains(err.Error(), "doc-obligation") {
				t.Fatal(err)
			}
		})
	}
}

// A subject can name a bead without being that bead's recorded landing.
func TestS133NoDocImpact_UnlandedMergeCannotBorrowDeclaration(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, action := range []string{"landed-control", "unbound-merge", "wrong-second-parent"} {
			t.Run(policy+"/"+action, func(t *testing.T) {
				f := s133DocFinalChangesFixture(t, policy, 1, []string{"bead-1"}, false, "", false, false)
				if action == "unbound-merge" {
					wt := filepath.Join(f.root, ".worktrees", "unlanded")
					preflightGit(t, f.root, "worktree", "add", "-b", "side", wt, "spec/010-test")
					s133Write(t, wt, "internal/widget/extra.go", "package widget\nconst Extra = 1\n")
					s133Commit(t, wt, "unlanded source")
					preflightGit(t, f.root, "worktree", "remove", wt)
					preflightGit(t, f.specWT, "merge", "--no-ff", "-m", "Merge bead/bead-1", "side")
				}
				if action == "wrong-second-parent" {
					if err := f.metadata("bead-1", map[string]interface{}{"mindspec_landed_second_parent": "wrong-parent"}); err != nil {
						t.Fatal(err)
					}
				}
				s133SeedPanel(t, f, "final_review", "spec/010-test", "", "pass", "final")
				s133Commit(t, f.specWT, "refresh final review")
				if action != "landed-control" {
					f.refuse("doc-sync:", ImplOpts{})
					return
				}
				if _, err := f.approve(); err != nil {
					t.Fatal(err)
				}
				f.assertConverged()
			})
		}
	}
}

func TestS133DocOwner_UnlandedMergeCannotResolveDeferral(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, landed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/landed=%t", policy, landed), func(t *testing.T) {
				// The owner is outside the epic's children and source audit set.
				owner := "new-doc-owner"
				f := s133DocFinalChangesFixture(t, policy, 1, []string{"bead-1"}, false, owner, false, false)
				wt := filepath.Join(f.root, ".worktrees", "owner-docs")
				preflightGit(t, f.root, "worktree", "add", "-b", "side", wt, "spec/010-test")
				s133Write(t, wt, ".mindspec/domains/workflow/interfaces.md", "Deferred workflow docs.\n")
				s133Commit(t, wt, "owner docs")
				second := preflightGit(t, wt, "rev-parse", "HEAD")
				preflightGit(t, f.root, "worktree", "remove", wt)
				preflightGit(t, f.specWT, "merge", "--no-ff", "-m", "Merge bead/"+owner, "side")
				if landed {
					merge := preflightGit(t, f.specWT, "rev-parse", "HEAD")
					if err := f.metadata(owner, map[string]interface{}{"mindspec_landed_merge_sha": merge, "mindspec_landed_second_parent": second}); err != nil {
						t.Fatal(err)
					}
				}
				s133SeedPanel(t, f, "final_review", "spec/010-test", "", "pass", "final")
				s133Commit(t, f.specWT, "refresh final review")
				if !landed {
					f.refuse("doc-sync:", ImplOpts{})
					return
				}
				if _, err := f.approve(); err != nil {
					t.Fatal(err)
				}
				f.assertConverged()
			})
		}
	}
}

func TestS133NoDocImpact_MergeOnlyKeepsExemption(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f := s133DocFinalChangesFixture(t, policy, 1, nil, false, "", false, false)
			if err := f.metadata("bead-1", map[string]interface{}{"mindspec_landed_merge_sha": "", "mindspec_landed_second_parent": ""}); err != nil {
				t.Fatal(err)
			}
			r, err := complete.Run(f.root, "bead-1", "010-test", "", f.exec, complete.CompleteOpts{
				MergeOnly: true, NoDocImpact: []string{"workflow: internal only"}, OverrideADR: "fixture architecture context",
			})
			if err != nil || r == nil || !r.MergeOnly {
				t.Fatalf("merge-only failed: %+v %v", r, err)
			}
			meta := f.read()["bead-1"].Metadata
			if meta["mindspec_landed_merge_sha"] != r.MergeSHA || meta["mindspec_landed_second_parent"] != r.MergeParents[1] {
				t.Fatalf("merge-only lost landing binding: %+v", meta)
			}
			s133SeedPanel(t, f, "final_review", "spec/010-test", "", "pass", "final")
			s133Commit(t, f.specWT, "final review ledger")
			if _, err := f.approve(); err != nil {
				t.Fatal(err)
			}
			f.assertConverged()
		})
	}
}
