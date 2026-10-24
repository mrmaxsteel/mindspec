package complete

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

func TestS133ArtifactPath_Complete(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, scenario := range []string{"absent", "rebind", "refutation", "deferral", "new round", "present unresolved", "no field unresolved", "no field approve"} {
			t.Run(policy+"/"+scenario, func(t *testing.T) {
				const specID, beadID, artifact = "130-core3", "mindspec-artifact.1", "evidence/result.txt"
				root, sha := setupPanelGateRepo(t, specID, beadID)
				t.Setenv(panel.SkipPanelEnv, "")
				config.ResetCache()
				t.Cleanup(config.ResetCache)
				writeFile(t, root, ".mindspec/config.yaml", "enforcement:\n  panel_presence: "+policy+"\n")
				// The lifecycle lock predates the panel gate; seed it before the
				// no-write snapshot so the assertion measures this refusal.
				writeFile(t, root, ".mindspec/phase.lock", "")
				store := newFakeMetadataStore()
				store.wire(t)
				closed := false
				closeBeadFn = func(...string) error { closed = true; return nil }
				dir := filepath.Join(root, "review", "artifact")
				in := panel.CreateInput{Spec: specID, BeadID: bp(beadID), Target: "bead/" + beadID, Gate: "bead", Round: 1, ReviewedHeadSHA: sha, SubjectDigest: "sha256:fixture",
					Roster: []panel.RosterSlot{{Slot: "R1", Family: "harness", Model: "m", Lens: "correctness"}, {Slot: "R2", Family: "harness", Model: "m", Lens: "architecture"}}}
				if err := panel.Create(dir, in); err != nil {
					t.Fatal(err)
				}
				submit := func(round int, sha, slot, verdict, text string) {
					t.Helper()
					v := paneltest.Verdict(verdict)
					v.Slot, v.Round, v.ReviewedSHA = slot, round, sha
					if verdict == panel.VerdictRequestChanges {
						v.ConcreteChangesRequired[0].Text = text
						if !strings.HasPrefix(scenario, "no field") {
							v.ConcreteChangesRequired[0].ArtifactPath = artifact
						}
					}
					if _, err := panel.Submit(panel.SubmitTarget{Dir: dir, Slot: slot, Round: round}, paneltest.Bytes(t, v), paneltest.Resolve); err != nil {
						t.Fatal(err)
					}
				}
				firstVerdict := panel.VerdictRequestChanges
				if scenario == "no field approve" {
					firstVerdict = panel.VerdictApprove
				}
				submit(1, sha, "R1", firstVerdict, "test finding /tmp/x")
				submit(1, sha, "R2", panel.VerdictApprove, "")
				read := func() *panel.Result {
					t.Helper()
					r, err := panel.Tally(dir)
					if err != nil || r.IntegrityErr != nil {
						t.Fatalf("fixture tally: %v %+v", err, r)
					}
					return r
				}
				refute := func() {
					t.Helper()
					r := read()
					f := r.Verdicts[0].Findings[0]
					ref := panel.Refutation{Slot: "R1", Round: r.Panel.Round, Finding: f.ID, FindingSHA256: f.SHA256, FindingText: f.Text, Ground: "disproven", Reason: "checked the finding", Evidence: "local evidence", By: "operator", CreatedAt: paneltest.CreatedAt}
					paneltest.Seed(t, dir, panel.RefutationRecord{Record: panel.RecordRefutation, Spec: specID, Panel: "artifact", Refutation: ref})
					p := *r.Panel
					p.Refutations = append(p.Refutations, ref)
					if err := panel.WriteSnapshot(dir, p); err != nil {
						t.Fatal(err)
					}
					paneltest.Seed(t, dir, panel.DispositionRow{Record: panel.RecordDisposition, ID: "refutation-row", Spec: specID, Panel: "artifact", Gate: "bead", Reviewer: "R1", Model: "m", Severity: "major", Summary: f.Text, ConvergentWith: []string{}, Disposition: "false-contamination", CreatedAt: paneltest.CreatedAt})
				}
				if scenario == "refutation" || scenario == "rebind" {
					refute()
				}
				if scenario == "rebind" || scenario == "new round" || scenario == "present unresolved" {
					gitRun(t, root, "checkout", "-q", "bead/"+beadID)
					writeFile(t, root, artifact, "measured\n")
					gitRun(t, root, "add", artifact)
					gitRun(t, root, "commit", "-q", "-m", "add artifact")
					tip := gateRevParse(t, root, "HEAD")
					gitRun(t, root, "checkout", "-q", "main")
					if scenario == "rebind" {
						r := read()
						reg := panel.Registration{Dir: dir, Panel: *r.Panel}
						// Rebind needs a passing panel. A later, changed submission
						// revokes the prior refutation in this same round.
						if d := panel.RebindDecision(reg, r); d.Action != panel.Allow {
							t.Fatalf("rebind precondition: %+v", d)
						}
						entry := panel.Rebind{Round: 1, FromSHA: sha, ToSHA: tip, Paths: []string{artifact}, Reason: "operator checked diff", CreatedAt: paneltest.CreatedAt}
						paneltest.Seed(t, dir, panel.RebindRecord{Record: panel.RecordRebind, Spec: specID, Panel: "artifact", SubjectDigest: "sha256:rebound", Rebind: entry})
						p := *r.Panel
						p.ReviewedHeadSHA, p.SubjectDigest = tip, "sha256:rebound"
						p.Rebinds = append(p.Rebinds, entry)
						if err := panel.WriteSnapshot(dir, p); err != nil {
							t.Fatal(err)
						}
						submit(1, tip, "R1", panel.VerdictRequestChanges, "renewed finding")
						if read().RoundReviewedHeadSHA != sha {
							t.Fatal("rebind changed the round commit")
						}
					} else {
						in.Round, in.ReviewedHeadSHA = 2, tip
						if err := panel.Create(dir, in); err != nil {
							t.Fatal(err)
						}
						submit(2, tip, "R1", panel.VerdictRequestChanges, "test finding /tmp/x")
						submit(2, tip, "R2", panel.VerdictApprove, "")
						if scenario == "new round" {
							refute()
						}
					}
				}
				if scenario == "deferral" {
					deferralBD(t, "epic-130-core3")
					r := read()
					f := r.Verdicts[0].Findings[0]
					d := panel.Deferral{Slot: "R1", Round: 1, Finding: f.ID, FindingSHA256: f.SHA256, FindingText: f.Text, Bead: "follow-up", By: "operator", CreatedAt: paneltest.CreatedAt, SourceVerdict: panel.VerdictRequestChanges, SourceFileSHA256: r.Verdicts[0].FileSHA256}
					paneltest.Seed(t, dir, panel.DeferralRecord{Record: panel.RecordDeferral, Spec: specID, Panel: "artifact", Deferral: d},
						panel.DispositionRow{Record: panel.RecordDisposition, ID: panel.DeferralRowID("artifact", d), Spec: specID, Panel: "artifact", Gate: "bead", Reviewer: "R1", Model: "m", Severity: "major", Summary: f.Text, ConvergentWith: []string{}, Disposition: "confirmed-deferred", EvidenceRef: d.Bead, CreatedAt: paneltest.CreatedAt})
					p := *r.Panel
					p.Deferrals = append(p.Deferrals, d)
					if err := panel.WriteSnapshot(dir, p); err != nil {
						t.Fatal(err)
					}
				}
				r := read()
				if err := panel.WriteCoverageManifest(r); err != nil {
					t.Fatal(err)
				}
				regs := panel.ForBead(panel.Scan(root), beadID)
				if len(regs) != 1 || len(r.Verdicts) != 2 {
					t.Fatalf("missing bead registration or submissions: %+v %+v", regs, r)
				}
				if scenario != "no field approve" && len(r.Verdicts[0].Findings) != 1 {
					t.Fatal("fixture lost its finding")
				}
				ex := &readStubMergeExecutor{Executor: executor.NewMindspecExecutor(root)}
				beforeHead := gateRevParse(t, root, "HEAD")
				beforeStatus, err := ex.Status(root)
				if err != nil {
					t.Fatal(err)
				}
				beforeMetadata, err := store.Get(beadID)
				if err != nil {
					t.Fatal(err)
				}
				result, err := Run(root, beadID, specID, "", ex, CompleteOpts{DocOwner: "fixture-doc-owner", AllowDocSkew: "test: fixture"})
				blocked := scenario == "absent" || scenario == "rebind" || scenario == "present unresolved" || scenario == "no field unresolved"
				if blocked {
					if err == nil || !strings.Contains(err.Error(), "unresolved REQUEST_CHANGES") || closed || ex.completeCalled || result != nil {
						t.Fatalf("finding must block before close/merge: %v closed=%v merged=%v result=%+v", err, closed, ex.completeCalled, result)
					}
					for _, want := range []string{"R1", "ccr[0]"} {
						if !strings.Contains(err.Error(), want) {
							t.Fatalf("missing %s: %v", want, err)
						}
					}
					wantPath := scenario == "absent" || scenario == "rebind"
					if strings.Contains(err.Error(), artifact) != wantPath || strings.Contains(err.Error(), "artifact paths absent from round tree") != wantPath {
						t.Fatalf("wrong round-tree path detail: %v", err)
					}
					afterStatus, statusErr := ex.Status(root)
					afterMetadata, metaErr := store.Get(beadID)
					if statusErr != nil || metaErr != nil || afterStatus != beforeStatus || gateRevParse(t, root, "HEAD") != beforeHead || !reflect.DeepEqual(beforeMetadata, afterMetadata) {
						t.Fatalf("refusal mutated repository or metadata: status %q -> %q (err=%v), metadata %#v -> %#v (err=%v), HEAD %s -> %s", beforeStatus, afterStatus, statusErr, beforeMetadata, afterMetadata, metaErr, beforeHead, gateRevParse(t, root, "HEAD"))
					}
					return
				}
				if err != nil || result == nil || !result.BeadClosed || !closed || !ex.completeCalled {
					t.Fatalf("discharged finding did not complete: %v result=%+v closed=%v merged=%v", err, result, closed, ex.completeCalled)
				}
				if scenario == "refutation" || scenario == "new round" {
					if store.data[beadID]["panel_refuted"] != true {
						t.Fatal("refutation was not applied and audited")
					}
				}
				if scenario == "deferral" && store.data[beadID]["panel_deferred"] != true {
					t.Fatal("deferral was not applied and audited")
				}
				if _, err := os.Stat(dir); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
