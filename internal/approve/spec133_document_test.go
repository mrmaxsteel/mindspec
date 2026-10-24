package approve

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

const s133DocID = "900-example"

func stubDocumentGateAudit(t *testing.T) {
	t.Helper()
	orig := lifecycleMergeMetadataFn
	lifecycleMergeMetadataFn = func(string, map[string]interface{}) error { return nil }
	t.Cleanup(func() { lifecycleMergeMetadataFn = orig })
}

type s133DocExecutor struct {
	*executor.MindspecExecutor
	root       string
	commits    int
	failCommit int
}

func (e *s133DocExecutor) CommitAll(_, message string) error {
	e.commits++
	if e.commits == e.failCommit {
		return fmt.Errorf("injected commit fault")
	}
	return e.MindspecExecutor.CommitAll(e.root, message)
}

func (e *s133DocExecutor) IsTreeClean(string) error { return e.MindspecExecutor.IsTreeClean(e.root) }

type s133DocumentFixture struct {
	t                    *testing.T
	root, dir            string
	exec                 *s133DocExecutor
	epic                 bool
	failEpic, failCreate bool
	meta                 map[string]interface{}
	mutations            []string
	children             []string
	creates, closes      int
}

func s133Document(t *testing.T, policy, gate, state string) *s133DocumentFixture {
	t.Helper()
	for k, v := range map[string]string{"GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_SYSTEM": "/dev/null", "GIT_TERMINAL_PROMPT": "0", "GIT_ASKPASS": "/usr/bin/false", panel.SkipPanelEnv: ""} {
		t.Setenv(k, v)
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "bd"), []byte("#!/bin/sh\ncase \"$1\" in\n export) exit 0 ;;\n show) printf '[{\"id\":\"%s\"}]\\n' \"$2\" ;;\n *) exit 1 ;;\nesac\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	f := &s133DocumentFixture{t: t, root: t.TempDir(), meta: map[string]interface{}{"spec_id": s133DocID, "mindspec_phase": "plan", phase.PhaseSemanticsV2MarkerKey: "v2"}}
	f.dir = filepath.Join(f.root, ".mindspec/specs", s133DocID)
	f.write(".mindspec/config.yaml", "enforcement:\n  panel_presence: "+policy+"\n")
	f.write(".gitignore", ".mindspec/specs/*/reviews/\n")
	text := specE2EValidSpecMD(s133DocID)
	text = strings.Replace(text, "Pending.\n", "- **Status**: DRAFT\n- **Approved By**: -\n- **Approval Date**: -\n- **Notes**: -\n", 1)
	f.writeSpec(text)
	f.write(".mindspec/adr/ADR-0001-core.md", "# ADR-0001: Core\n\n- **Status**: Accepted\n- **Domain(s)**: core\n")
	if gate == "plan_approve" {
		f.epic = true
		f.writePlan(strings.Replace(strings.ReplaceAll(validSingleBeadPlan, "042-test", s133DocID), "version: \"1.0\"", "version: \"1.0\"\nadr_citations: [ADR-0001]", 1) + "\n## Resolutions\n\nNone.\n")
	}
	preflightGit(t, f.root, "init", "-q", "-b", "spec/"+s133DocID)
	preflightGit(t, f.root, "config", "user.name", "Test")
	preflightGit(t, f.root, "config", "user.email", "test@example.invalid")
	preflightGit(t, f.root, "config", "commit.gpgsign", "false")
	preflightGit(t, f.root, "config", "core.hooksPath", filepath.Join(f.root, "empty-hooks"))
	f.exec = &s133DocExecutor{MindspecExecutor: executor.NewMindspecExecutor(f.root), root: f.root}
	f.commit("base")
	config.ResetCache()
	t.Cleanup(config.ResetCache)
	if state != "absent" {
		if state == "coverage" {
			fixture := f.seed(gate, "unresolved")
			v, err := panel.ReadSubmittedFindingVerdict(fixture.Dir, "R1", 1)
			if err != nil {
				t.Fatal(err)
			}
			findings, err := panel.Findings(v)
			if err != nil || len(findings) != 1 {
				t.Fatalf("coverage finding: %v %v", findings, err)
			}
			finding := findings[0]
			ref := panel.Refutation{Slot: "R1", Round: 1, Finding: finding.ID, FindingSHA256: finding.SHA256, FindingText: finding.Text, Ground: "disproven", Reason: "local test passes", Evidence: "test output", By: "operator", CreatedAt: paneltest.CreatedAt}
			paneltest.Seed(t, fixture.Dir, panel.RefutationRecord{Record: panel.RecordRefutation, Spec: s133DocID, Panel: gate, Refutation: ref})
			res, err := panel.Tally(fixture.Dir)
			if err != nil {
				t.Fatal(err)
			}
			res.Panel.Refutations = []panel.Refutation{ref}
			if err := panel.WriteSnapshot(fixture.Dir, *res.Panel); err != nil {
				t.Fatal(err)
			}
			res, err = panel.Tally(fixture.Dir)
			if err != nil || !res.PassRule(true) {
				t.Fatalf("coverage fixture must pass decision: %+v %v", res, err)
			}
			if err := panel.WriteCoverageManifest(res); err != nil {
				t.Fatal(err)
			}
		} else {
			f.seed(gate, state)
		}
	}
	epics := func() []byte {
		if !f.epic {
			return []byte(`[]`)
		}
		b, err := json.Marshal([]interface{}{map[string]interface{}{"id": "epic-doc", "status": "open", "issue_type": "epic", "metadata": f.meta}})
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	t.Cleanup(phase.SetListJSONForTest(func(...string) ([]byte, error) { return epics(), nil }))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if args[0] == "show" {
			return epics(), nil
		}
		return []byte(`[]`), nil
	}))
	merge := func(_ string, m map[string]interface{}) error {
		f.mutations = append(f.mutations, "metadata")
		for k, v := range m {
			f.meta[k] = v
		}
		return nil
	}
	t.Cleanup(phase.SetMergeMetadataForTest(merge))
	t.Cleanup(SetSpecMergeMetadataForTest(merge))
	t.Cleanup(SetPlanMergeMetadataForTest(merge))
	origAudit := lifecycleMergeMetadataFn
	lifecycleMergeMetadataFn = merge
	t.Cleanup(func() { lifecycleMergeMetadataFn = origAudit })
	t.Cleanup(SetSpecRunBDForTest(func(args ...string) ([]byte, error) {
		if args[0] == "create" {
			f.mutations = append(f.mutations, "epic-create")
			if f.failEpic {
				return nil, fmt.Errorf("injected epic fault")
			}
			f.epic = true
			return []byte(`{"id":"epic-doc"}`), nil
		}
		return nil, nil
	}))
	t.Cleanup(SetPlanListJSONForTest(func(...string) ([]byte, error) { return existingChildrenJSON(f.children, nil), nil }))
	t.Cleanup(SetPlanRunBDCombinedForTest(func(args ...string) ([]byte, error) {
		if args[0] == "close" {
			f.closes++
			f.mutations = append(f.mutations, "close")
		}
		return nil, nil
	}))
	t.Cleanup(SetPlanRunBDForTest(func(args ...string) ([]byte, error) {
		if args[0] == "create" {
			if f.failCreate {
				return nil, fmt.Errorf("injected create fault")
			}
			f.creates++
			f.mutations = append(f.mutations, "bead-create")
			return []byte(fmt.Sprintf(`{"id":"doc-%d"}`, f.creates)), nil
		}
		return nil, nil
	}))
	return f
}

func (f *s133DocumentFixture) write(name, text string) {
	f.t.Helper()
	p := filepath.Join(f.root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0644); err != nil {
		f.t.Fatal(err)
	}
}
func (f *s133DocumentFixture) writeSpec(text string) {
	f.write(".mindspec/specs/"+s133DocID+"/spec.md", text)
}
func (f *s133DocumentFixture) writePlan(text string) {
	f.write(".mindspec/specs/"+s133DocID+"/plan.md", text)
}
func (f *s133DocumentFixture) read(name string) string {
	f.t.Helper()
	b, e := os.ReadFile(filepath.Join(f.dir, name))
	if e != nil {
		f.t.Fatal(e)
	}
	return string(b)
}
func (f *s133DocumentFixture) commit(msg string) {
	f.t.Helper()
	preflightGit(f.t, f.root, "add", "-A")
	preflightGit(f.t, f.root, "commit", "-qm", msg)
}
func (f *s133DocumentFixture) seed(gate, state string) paneltest.Fixture {
	f.t.Helper()
	head, e := f.exec.RevParseRef(f.root, "HEAD")
	if e != nil {
		f.t.Fatal(e)
	}
	digest, e := panel.DocumentSubjectTreeReader(gate, f.dir, f.exec.LsTree, func(_, ref, name string) ([]byte, error) { return f.exec.FileAtRef(ref, name) })(f.root, head, panel.Subject(gate, f.dir))
	if e != nil {
		f.t.Fatal(e)
	}
	return paneltest.Lifecycle(f.t, filepath.Join(f.dir, "reviews", gate), s133DocID, gate, head, digest, state)
}
func (f *s133DocumentFixture) approve(gate string) ([]string, error) {
	if gate == "spec_approve" {
		r, e := ApproveSpec(f.root, s133DocID, "operator", f.exec)
		if r != nil {
			return r.Warnings, e
		}
		return nil, e
	}
	r, e := ApprovePlan(f.root, s133DocID, "operator", f.exec)
	if r != nil {
		return r.Warnings, e
	}
	return nil, e
}
func (f *s133DocumentFixture) unchanged(gate string, call func() error) error {
	f.t.Helper()
	spec := f.read("spec.md")
	plan, _ := os.ReadFile(filepath.Join(f.dir, "plan.md"))
	head := preflightGit(f.t, f.root, "rev-parse", "HEAD")
	meta, _ := json.Marshal(f.meta)
	epic := f.epic
	err := call()
	if err == nil {
		f.t.Fatal("gate accepted wrong fixture")
	}
	if spec != f.read("spec.md") {
		f.t.Fatal("refusal wrote spec.md")
	}
	afterPlan, _ := os.ReadFile(filepath.Join(f.dir, "plan.md"))
	if string(plan) != string(afterPlan) {
		f.t.Fatal("refusal wrote plan.md")
	}
	if head != preflightGit(f.t, f.root, "rev-parse", "HEAD") || f.exec.commits != 0 {
		f.t.Fatal("refusal made a commit")
	}
	afterMeta, _ := json.Marshal(f.meta)
	if string(meta) != string(afterMeta) || epic != f.epic || len(f.mutations) != 0 || f.creates != 0 || f.closes != 0 {
		f.t.Fatalf("refusal mutated lifecycle: %v", f.mutations)
	}
	s133GateText(f.t, err.Error(), gate, f.root)
	return err
}

func TestS133DocGate_SpecApproveBeforeFirstWrite(t *testing.T) {
	f := s133Document(t, "enforce", "spec_approve", "absent")
	err := f.unchanged("spec_approve", func() error { _, e := f.approve("spec_approve"); return e })
	if !strings.HasSuffix(err.Error(), "recovery: mindspec panel create <slug> --spec 900-example --gate spec_approve --target spec/900-example") {
		t.Fatal(err)
	}
	if _, e := os.Stat(filepath.Join(f.dir, "plan.md")); !os.IsNotExist(e) {
		t.Fatal("refusal created plan scaffold")
	}
}

func TestS133DocGate_SpecApproveReplayRunsGate(t *testing.T) {
	f := s133Document(t, "enforce", "spec_approve", "absent")
	f.epic = true
	if e := updateSpecApproval(filepath.Join(f.dir, "spec.md"), "operator"); e != nil {
		t.Fatal(e)
	}
	f.writePlan(scaffoldPlan(s133DocID))
	f.commit("old approval")
	f.unchanged("spec_approve", func() error { _, e := f.approve("spec_approve"); return e })
}

func TestS133DocGate_PlanApproveBeforeFirstWrite(t *testing.T) {
	f := s133Document(t, "enforce", "plan_approve", "absent")
	err := f.unchanged("plan_approve", func() error { _, err := f.approve("plan_approve"); return err })
	if !strings.HasSuffix(err.Error(), "recovery: mindspec panel create <slug> --spec 900-example --gate plan_approve --target spec/900-example") {
		t.Fatal(err)
	}
}

func TestS133DocGate_PlanGateBeforeMigration(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f := s133Document(t, policy, "plan_approve", "reject")
			delete(f.meta, "mindspec_phase")
			delete(f.meta, phase.PhaseSemanticsV2MarkerKey)
			f.unchanged("plan_approve", func() error { _, err := f.approve("plan_approve"); return err })
			if _, err := os.Stat(filepath.Join(f.root, phase.PhaseLockRelPath)); !os.IsNotExist(err) {
				t.Fatalf("document refusal reached migration lock: %v", err)
			}
		})
	}
}

func TestS133DocGate_FailingLegs(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, gate := range []string{"spec_approve", "plan_approve"} {
			for _, state := range []string{"malformed", "integrity", "round mismatch", "incomplete", "reject", "hard block", "unresolved", "coverage"} {
				t.Run(policy+"/"+gate+"/"+state, func(t *testing.T) {
					f := s133Document(t, policy, gate, state)
					err := f.unchanged(gate, func() error { _, e := f.approve(gate); return e })
					want := map[string]string{"malformed": "registration", "integrity": "integrity", "round mismatch": "out of date", "incomplete": "incomplete", "reject": "REJECT", "hard block": "hard_block", "unresolved": "unresolved", "coverage": "coverage"}[state]
					if !strings.Contains(err.Error(), want) {
						t.Fatalf("wrong leg, want %s: %v", want, err)
					}
				})
			}
		}
	}
}

func TestS133DocGate_PassingPanels(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, gate := range []string{"spec_approve", "plan_approve"} {
			t.Run(policy+"/"+gate, func(t *testing.T) {
				f := s133Document(t, policy, gate, "pass")
				if _, e := f.approve(gate); e != nil {
					t.Fatal(e)
				}
				if gate == "plan_approve" && f.creates != 1 {
					t.Fatal("no bead created")
				}
			})
		}
	}
}

func TestS133DocGate_SubjectEdits(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, gate := range []string{"spec_approve", "plan_approve"} {
			for _, committed := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/committed=%v", policy, gate, committed), func(t *testing.T) {
					f := s133Document(t, policy, gate, "pass")
					name := "spec.md"
					if gate == "plan_approve" {
						name = "plan.md"
					}
					f.write(".mindspec/specs/"+s133DocID+"/"+name, f.read(name)+"\nBody edit.\n")
					if committed {
						f.commit("body edit")
					}
					err := f.unchanged(gate, func() error { _, e := f.approve(gate); return e })
					want := name
					if committed {
						want = "commits landed after review"
					}
					if !strings.Contains(err.Error(), want) {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func TestS133DocGate_PresenceAndHatches(t *testing.T) {
	for _, state := range []string{"warn", "default", "skip", "disabled", "abandoned"} {
		t.Run(state, func(t *testing.T) {
			policy := "enforce"
			panelState := "absent"
			if state == "warn" || state == "default" {
				policy = "warn"
			}
			if state == "abandoned" {
				panelState = state
			}
			f := s133Document(t, policy, "spec_approve", panelState)
			if state == "default" {
				f.write(".mindspec/config.yaml", "enforcement: {}\n")
				f.commit("default policy")
				config.ResetCache()
			}
			if state == "disabled" {
				f.write(".mindspec/config.yaml", "enforcement:\n  panel_presence: enforce\n  panel_gate: false\n")
				f.commit("disabled gate")
				config.ResetCache()
			}
			if state == "skip" {
				t.Setenv(panel.SkipPanelEnv, "1")
			}
			warnings, e := f.approve("spec_approve")
			if e != nil {
				t.Fatal(e)
			}
			key := map[string]string{"warn": "panel_absent", "default": "panel_absent", "skip": "panel_gate_skipped", "abandoned": "panel_abandoned"}[state]
			if key != "" && f.meta[key] != true {
				t.Fatalf("missing %s: %v", key, f.meta)
			}
			if state == "disabled" {
				for _, k := range []string{"panel_absent", "panel_gate_skipped", "panel_abandoned"} {
					if _, ok := f.meta[k]; ok {
						t.Fatalf("disabled gate wrote %s", k)
					}
				}
			}
			text := strings.Join(warnings, "\n")
			if strings.Contains(text, panel.SkipPanelEnv) {
				t.Fatal(text)
			}
			if state == "warn" || state == "default" {
				count := 0
				for _, w := range warnings {
					if strings.Contains(w, "gate spec_approve") {
						count++
					}
				}
				if count != 1 || f.meta["panel_absent_gate"] != "spec_approve" {
					t.Fatalf("absence warning/audit: %v %v", warnings, f.meta)
				}
			}
			if state == "abandoned" && (!strings.Contains(text, "operator stopped the review") || f.meta["panel_abandoned_reason"] != "operator stopped the review") {
				t.Fatalf("abandon reason lost: %v %v", warnings, f.meta)
			}
		})
	}
}

func TestS133DocGate_HandWrittenApprovalDoesNotBypass(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, gate := range []string{"spec_approve", "plan_approve"} {
			for _, state := range []string{"absent", "incomplete"} {
				if policy == "warn" && state == "absent" {
					continue
				}
				t.Run(policy+"/"+gate+"/"+state, func(t *testing.T) {
					f := s133Document(t, policy, gate, state)
					var e error
					if gate == "spec_approve" {
						e = updateSpecApproval(filepath.Join(f.dir, "spec.md"), "hand")
					} else {
						e = updatePlanApproval(filepath.Join(f.dir, "plan.md"), "hand")
					}
					if e != nil {
						t.Fatal(e)
					}
					f.commit("hand approval")
					f.unchanged(gate, func() error { _, e := f.approve(gate); return e })
				})
			}
		}
	}
}

func TestS133DocGate_InFlightSpecKeepsApproval(t *testing.T) {
	f := s133Document(t, "enforce", "plan_approve", "absent")
	if e := updateSpecApproval(filepath.Join(f.dir, "spec.md"), "prior"); e != nil {
		t.Fatal(e)
	}
	f.commit("prior spec approval")
	f.seed("plan_approve", "pass")
	before := f.read("spec.md")
	if _, e := f.approve("plan_approve"); e != nil {
		t.Fatal(e)
	}
	if f.read("spec.md") != before {
		t.Fatal("plan approval changed prior spec approval")
	}
	if !reflect.DeepEqual(f.meta["mindspec_phase"], "plan") {
		t.Fatal(f.meta)
	}
}

func TestS133DocGate_ReApproveEditedPlanRunsGateBeforeSupersede(t *testing.T) {
	f := s133Document(t, "enforce", "plan_approve", "absent")
	f.children = []string{"old-bead"}
	if e := updatePlanApproval(filepath.Join(f.dir, "plan.md"), "prior"); e != nil {
		t.Fatal(e)
	}
	f.writePlan(f.read("plan.md") + "\nEdited plan.\n")
	f.commit("edited approved plan")
	f.unchanged("plan_approve", func() error { _, e := f.approve("plan_approve"); return e })
	f.seed("plan_approve", "pass")
	if _, e := f.approve("plan_approve"); e != nil {
		t.Fatal(e)
	}
	if f.closes != 1 || f.creates != 1 {
		t.Fatalf("did not replace bead: close=%d create=%d", f.closes, f.creates)
	}
}
