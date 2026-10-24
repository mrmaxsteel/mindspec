package approve

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

func TestS133DocGate_SpecFaultReplay(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, section := range []string{"draft", "absent"} {
			for _, fault := range []int{0, 1, 2} {
				t.Run(fmt.Sprintf("%s/%s/fault=%d", policy, section, fault), func(t *testing.T) {
					f := s133Document(t, policy, "spec_approve", "absent")
					if section == "absent" {
						text, _, _ := strings.Cut(f.read("spec.md"), "## Approval")
						f.writeSpec(text)
						f.commit("absent approval section")
					}
					f.seed("spec_approve", "pass")
					f.exec.failCommit = fault
					_, err := f.approve("spec_approve")
					if (err != nil) != (fault > 0) {
						t.Fatalf("wrong fault outcome: %v", err)
					}
					f.verifyDocument("spec_approve")
					if fault > 0 {
						f.exec.failCommit = 0
						if _, err := f.approve("spec_approve"); err != nil {
							t.Fatal(err)
						}
					}
					f.verifyDocument("spec_approve")
					log := preflightGit(t, f.root, "log", "--format=%s")
					if fault == 1 && strings.Count(log, "chore: approve spec ") != 1 {
						t.Fatalf("replay must make one approval commit: %s", log)
					}
				})
			}
		}
	}
}

func (f *s133DocumentFixture) verifyDocument(gate string) {
	f.t.Helper()
	if _, err := runLifecycleGate(f.root, s133DocID, gate, f.root, nil); err != nil {
		f.t.Fatal(err)
	}
	res, err := panel.Tally(filepath.Join(f.dir, "reviews", gate))
	if err != nil {
		f.t.Fatal(err)
	}
	head, err := f.exec.RevParseRef(f.root, "HEAD")
	if err != nil {
		f.t.Fatal(err)
	}
	digest, err := panel.DocumentSubjectTreeReader(gate, f.dir, f.exec.LsTree, func(_, ref, name string) ([]byte, error) { return f.exec.FileAtRef(ref, name) })(f.root, head, panel.Subject(gate, f.dir))
	if err != nil {
		f.t.Fatal(err)
	}
	reg := panel.Registration{Dir: res.Dir, Panel: *res.Panel}
	d := panel.PanelGateDecision(panel.GateFacts{Reg: &reg, Res: res, HeadSHA: head, SubjectDigest: digest, WorktreeAbsent: true})
	if d.Action != panel.Allow {
		f.t.Fatalf("verify decision refused own writes: %+v", d)
	}
}

func TestS133DocGate_PlanFaultReplay(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, fault := range []string{"frontmatter", "commit"} {
			t.Run(policy+"/"+fault, func(t *testing.T) {
				f := s133Document(t, policy, "plan_approve", "pass")
				if fault == "frontmatter" {
					f.failCreate = true
				} else {
					f.exec.failCommit = 2
				}
				if _, err := f.approve("plan_approve"); err == nil {
					t.Fatal("fault did not fire")
				}
				if !strings.Contains(f.read("plan.md"), "status: Approved") {
					t.Fatal("fault did not follow frontmatter write")
				}
				if _, err := runLifecycleGate(f.root, s133DocID, "plan_approve", f.root, nil); err != nil {
					t.Fatal(err)
				}
				f.failCreate = false
				f.exec.failCommit = 0
				if _, err := f.approve("plan_approve"); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestS133DocGate_ApprovalSectionAndKeyOrder(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, which := range []string{"keys", "extra approval line", "ledger commit"} {
			t.Run(policy+"/"+which, func(t *testing.T) {
				f := s133Document(t, policy, "spec_approve", "absent")
				if which == "keys" {
					f.writeSpec(strings.Replace(f.read("spec.md"), "spec_id: 900-example\nstatus: Draft\nversion: 1", "version: '1'\nstatus: 'Draft'\nspec_id: '900-example'", 1))
					f.commit("unsorted keys")
				}
				f.seed("spec_approve", "pass")
				if which == "extra approval line" {
					f.writeSpec(f.read("spec.md") + "\nExtra approval content.\n")
					f.commit("approval content")
					err := f.unchanged("spec_approve", func() error { _, e := f.approve("spec_approve"); return e })
					if !strings.Contains(err.Error(), "commits landed after review") {
						t.Fatal(err)
					}
					return
				}
				if which == "ledger commit" {
					preflightGit(t, f.root, "add", "-f", filepath.Join(f.dir, "reviews/spec_approve/dispositions.jsonl"))
					preflightGit(t, f.root, "commit", "-qm", "ledger only")
				}
				if _, err := f.approve("spec_approve"); err != nil {
					t.Fatal(err)
				}
				f.verifyDocument("spec_approve")
			})
		}
	}
}

func TestS133DocGate_Targets(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, gate := range []string{"spec_approve", "plan_approve"} {
			for _, kind := range []string{"sha", "hand edit"} {
				t.Run(policy+"/"+gate+"/"+kind, func(t *testing.T) {
					f := s133Document(t, policy, gate, "absent")
					fixture := f.seed(gate, "pass")
					p := fixture.Panel
					if kind == "hand edit" {
						p.Target = p.ReviewedHeadSHA
						if err := panel.WriteSnapshot(fixture.Dir, p); err != nil {
							t.Fatal(err)
						}
					} else {
						// An old binary could record the SHA in both the round and snapshot.
						old := filepath.Join(f.dir, "reviews", "old-sha")
						p.Target = p.ReviewedHeadSHA
						if err := panel.WriteSnapshot(old, p); err != nil {
							t.Fatal(err)
						}
						paneltest.Fixture{Dir: old, Panel: p}.Round(t, 2)
					}
					err := f.unchanged(gate, func() error { _, e := f.approve(gate); return e })
					want := "target"
					if kind == "hand edit" {
						want = "integrity"
					}
					if !strings.Contains(err.Error(), want) || (kind == "sha" && !strings.Contains(err.Error(), p.Target)) {
						t.Fatal(err)
					}
				})
			}
		}
	}
	for _, gate := range []string{"spec_approve", "plan_approve", "final_review"} {
		t.Run("create/"+gate, func(t *testing.T) {
			f := s133Document(t, "enforce", "spec_approve", "absent")
			head := preflightGit(t, f.root, "rev-parse", "HEAD")
			dir := filepath.Join(f.dir, "reviews", "sha")
			err := panel.Create(dir, panel.CreateInput{Spec: s133DocID, Gate: gate, Target: head, Round: 1, ReviewedHeadSHA: head, Roster: []panel.RosterSlot{{Slot: "R1", Model: "m", Lens: "test"}, {Slot: "R2", Model: "m", Lens: "test"}}})
			if err == nil || !strings.Contains(err.Error(), head) {
				t.Fatalf("SHA target accepted: %v", err)
			}
			if _, e := os.Stat(filepath.Join(dir, panel.FileName)); !os.IsNotExist(e) {
				t.Fatal("invalid create wrote snapshot")
			}
		})
	}
}

func (f *s133DocumentFixture) deferFinding(artifact bool) string {
	f.t.Helper()
	dir := filepath.Join(f.dir, "reviews/spec_approve")
	res, err := panel.Tally(dir)
	if err != nil {
		f.t.Fatal(err)
	}
	if artifact {
		v := paneltest.Verdict(panel.VerdictRequestChanges)
		v.ReviewedSHA = res.Panel.ReviewedHeadSHA
		v.ConcreteChangesRequired[0].ArtifactPath = "evidence/missing.txt"
		if _, err := panel.Submit(panel.SubmitTarget{Dir: dir, Slot: "R1", Round: 1}, paneltest.Bytes(f.t, v), paneltest.Resolve); err != nil {
			f.t.Fatal(err)
		}
		_, err := f.approve("spec_approve")
		if err == nil || !strings.Contains(err.Error(), "evidence/missing.txt") {
			f.t.Fatalf("missing artifact not named: %v", err)
		}
	}
	v, err := panel.ReadSubmittedFindingVerdict(dir, "R1", 1)
	if err != nil {
		f.t.Fatal(err)
	}
	findings, err := panel.Findings(v)
	if err != nil || len(findings) != 1 {
		f.t.Fatalf("findings: %v %v", findings, err)
	}
	finding := findings[0]
	d := panel.PlanDeferral{ID: panel.PlanDeferralID(s133DocID, "spec_approve", "R1", 1, finding), Slot: "R1", Round: 1, Finding: finding.ID, FindingSHA256: finding.SHA256, FindingText: finding.Text, By: "operator", CreatedAt: paneltest.CreatedAt}
	round := 1
	paneltest.Seed(f.t, dir, panel.PlanDeferralRecord{Record: panel.RecordDeferredToPlan, Spec: s133DocID, Panel: "spec_approve", PlanDeferral: d}, panel.DispositionRow{Record: panel.RecordDisposition, Spec: s133DocID, Panel: "spec_approve", Reviewer: "R1", Round: &round, Model: "m", Severity: "blocking", ID: "deferral-row", Gate: "spec_approve", Summary: d.FindingText, ConvergentWith: []string{}, CreatedAt: paneltest.CreatedAt, Disposition: "confirmed-deferred", EvidenceRef: "plan:" + d.ID})
	res, err = panel.Tally(dir)
	if err != nil {
		f.t.Fatal(err)
	}
	p := *res.Panel
	p.PlanDeferrals = append(p.PlanDeferrals, d)
	if err := panel.WriteSnapshot(dir, p); err != nil {
		f.t.Fatal(err)
	}
	res, err = panel.Tally(dir)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := panel.WriteCoverageManifest(res); err != nil {
		f.t.Fatal(err)
	}
	if err := panel.CheckCompletenessAt(dir, "spec_approve"); err != nil {
		f.t.Fatal(err)
	}
	return d.ID
}

func TestS133DocGate_ArtifactAndPlanDeferral(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, artifact := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/artifact=%v", policy, artifact), func(t *testing.T) {
				f := s133Document(t, policy, "spec_approve", "unresolved")
				id := f.deferFinding(artifact)
				if _, err := f.approve("spec_approve"); err != nil {
					t.Fatal(err)
				}
				if ids, ok := f.meta["plan_deferral_ids"].([]string); !ok || len(ids) != 1 || ids[0] != id {
					t.Fatalf("accepted ids not recorded: %v", f.meta)
				}
				res, err := panel.Tally(filepath.Join(f.dir, "reviews/spec_approve"))
				if err != nil {
					t.Fatal(err)
				}
				res.Panel.PlanDeferrals[0].FindingText = "hand edited"
				if err := panel.WriteSnapshot(res.Dir, *res.Panel); err != nil {
					t.Fatal(err)
				}
				_, err = f.approve("spec_approve")
				if err == nil || !strings.Contains(err.Error(), "integrity") {
					t.Fatalf("hand-added deferral passed: %v", err)
				}
			})
		}
	}
}

func TestS133DocGate_EpicFailureReplaysDueAudit(t *testing.T) {
	for _, kind := range []string{"skip", "absent", "abandoned", "deferral"} {
		t.Run(kind, func(t *testing.T) {
			policy, state := "enforce", "absent"
			if kind == "absent" {
				policy = "warn"
			}
			if kind == "abandoned" {
				state = "abandoned"
			}
			if kind == "deferral" {
				state = "unresolved"
			}
			f := s133Document(t, policy, "spec_approve", state)
			var id string
			if kind == "skip" {
				t.Setenv(panel.SkipPanelEnv, "1")
			}
			if kind == "deferral" {
				id = f.deferFinding(false)
			}
			f.failEpic = true
			_, err := f.approve("spec_approve")
			key := map[string]string{"skip": "panel_gate_skipped", "absent": "panel_absent", "abandoned": "panel_abandoned", "deferral": "plan_deferral_ids"}[kind]
			if err == nil || !strings.Contains(err.Error(), key) || !strings.HasSuffix(err.Error(), "recovery: run the same command again in the same environment") {
				t.Fatalf("lost due audit: %v", err)
			}
			if strings.Contains(err.Error(), panel.SkipPanelEnv) || preflightGit(t, f.root, "status", "--porcelain") != "" || f.exec.commits != 2 {
				t.Fatalf("fault must follow clean approval commits: %v", err)
			}
			f.failEpic = false
			if _, err := f.approve("spec_approve"); err != nil {
				t.Fatal(err)
			}
			if kind == "deferral" {
				ids, ok := f.meta[key].([]string)
				if !ok || len(ids) != 1 || ids[0] != id {
					t.Fatal(f.meta)
				}
			} else if f.meta[key] != true {
				t.Fatal(f.meta)
			}
		})
	}
}

func TestReviewerR7CRLFOwnWrites(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f := s133Document(t, policy, "spec_approve", "absent")
			f.writeSpec(strings.ReplaceAll(f.read("spec.md"), "\n", "\r\n"))
			f.commit("CRLF spec")
			f.seed("spec_approve", "pass")
			if _, err := f.approve("spec_approve"); err != nil {
				t.Fatalf("initial approval: %v", err)
			}
			if _, err := f.approve("spec_approve"); err != nil {
				t.Fatalf("own-write replay refused: %v", err)
			}
		})
	}
}

func TestS133DocGate_CRLFPlanOwnWrites(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f := s133Document(t, policy, "plan_approve", "absent")
			f.writePlan(strings.ReplaceAll(f.read("plan.md"), "\n", "\r\n"))
			f.commit("CRLF plan")
			f.seed("plan_approve", "pass")
			if _, err := f.approve("plan_approve"); err != nil {
				t.Fatalf("initial approval: %v", err)
			}
			f.verifyDocument("plan_approve")
			if _, err := f.approve("plan_approve"); err != nil {
				t.Fatalf("own-write replay refused: %v", err)
			}
		})
	}
}

func TestS133DocGate_CRLFSpecBodyEdit(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f := s133Document(t, policy, "spec_approve", "absent")
			f.writeSpec(strings.ReplaceAll(f.read("spec.md"), "\n", "\r\n"))
			f.commit("CRLF spec")
			f.seed("spec_approve", "pass")
			if _, err := f.approve("spec_approve"); err != nil {
				t.Fatalf("initial approval: %v", err)
			}
			f.verifyDocument("spec_approve")
			f.writeSpec(strings.Replace(f.read("spec.md"), "## Approval", "Body edit.\r\n\r\n## Approval", 1))
			f.commit("body edit after approval")
			f.exec.commits = 0
			f.mutations = nil
			err := f.unchanged("spec_approve", func() error { _, err := f.approve("spec_approve"); return err })
			if !strings.Contains(err.Error(), "commits landed after review") {
				t.Fatalf("body edit did not refuse as stale: %v", err)
			}
		})
	}
}

func TestS133DocGate_CRLFApprovalPadding(t *testing.T) {
	for _, section := range []string{"absent", "draft", "appendix"} {
		for _, ending := range []string{"", "\r\n", "\r\n\r\n"} {
			t.Run(fmt.Sprintf("%s/padding=%d", section, len(ending)), func(t *testing.T) {
				const dir = ".mindspec/specs/example"
				body := "# Spec\r\n\r\nBody."
				if section != "absent" {
					body += "\r\n\r\n## Approval\r\n\r\n- **Status**: Draft\r\n- **Approved By**: -\r\n- **Approval Date**: -\r\n- **Notes**: -"
				}
				if section == "appendix" {
					body += "\r\n\r\n## Appendix\r\nKeep."
				}
				data := []byte("---\r\ntitle: Example\r\nstatus: Draft\r\n---\r\n" + body + ending)
				before, _, err := panel.DocumentSubjectContent(dir+"/spec.md", data, dir, "spec_approve")
				if err != nil {
					t.Fatal(err)
				}
				file := filepath.Join(t.TempDir(), "spec.md")
				if err := os.WriteFile(file, data, 0644); err != nil {
					t.Fatal(err)
				}
				if err := updateSpecApproval(file, "operator"); err != nil {
					t.Fatal(err)
				}
				afterData, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				after, _, err := panel.DocumentSubjectContent(dir+"/spec.md", afterData, dir, "spec_approve")
				if err != nil || string(before) != string(after) {
					t.Fatalf("own writes changed digest: before %s; after %s; %v", before, after, err)
				}
				afterData = []byte(strings.Replace(string(afterData), "Body.", "Body edit.", 1))
				edited, _, err := panel.DocumentSubjectContent(dir+"/spec.md", afterData, dir, "spec_approve")
				if err != nil || string(edited) == string(after) {
					t.Fatalf("body edit was omitted: %v", err)
				}
			})
		}
	}
}
