package approve

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/complete"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

type s133FinalFixture struct {
	*rerunFixture
	out bytes.Buffer
	dir string
}

func s133Write(t *testing.T, root, name, text string) {
	t.Helper()
	full := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}

func s133Commit(t *testing.T, root, message string) {
	t.Helper()
	preflightGit(t, root, "add", "-A")
	preflightGit(t, root, "commit", "-qm", message)
}

func s133FinalFixtureNew(t *testing.T, policy string, count int, finalState string, beforeMerge ...func(*s133FinalFixture, []string)) *s133FinalFixture {
	t.Helper()
	f := &s133FinalFixture{rerunFixture: newRerunFixture(t, false, false)}
	f.dir = filepath.Join(f.specWT, ".mindspec/specs/010-test/reviews/final")
	t.Setenv(panel.SkipPanelEnv, "")
	for _, root := range []string{f.root, f.specWT} {
		s133Write(t, root, ".mindspec/config.yaml", "enforcement:\n  panel_presence: "+policy+"\n")
		s133Write(t, root, ".gitignore", ".worktrees/\n.mindspec/phase.lock\n.mindspec/specs/*/reviews/*/*\n!.mindspec/specs/*/reviews/*/dispositions.jsonl\n")
		s133Commit(t, root, "gate configuration")
	}
	preflightGit(t, f.specWT, "merge", "--no-ff", "-m", "shared configuration", "main")
	config.ResetCache()
	t.Cleanup(config.ResetCache)
	oldWriter := warnWriter
	warnWriter = &f.out
	t.Cleanup(func() { warnWriter = oldWriter })
	oldAudit := lifecycleMergeMetadataFn
	lifecycleMergeMetadataFn = f.metadata
	t.Cleanup(func() { lifecycleMergeMetadataFn = oldAudit })
	oldCreate := implCreateWithIDFn
	implCreateWithIDFn = adr.CreateWithID
	t.Cleanup(func() { implCreateWithIDFn = oldCreate })
	ids := []string{"bead-1"}
	if count == 2 {
		ids = append(ids, "bead-2")
		r := f.read()
		r["bead-2"] = phase.EpicInfo{ID: "bead-2", IssueType: "task", Status: "closed", Metadata: map[string]interface{}{}}
		f.write(r)
		plan, err := os.ReadFile(filepath.Join(f.specWT, ".mindspec/specs/010-test/plan.md"))
		if err != nil {
			t.Fatal(err)
		}
		s133Write(t, f.specWT, ".mindspec/specs/010-test/plan.md", strings.Replace(string(plan), "  - bead-1\n", "  - bead-1\n  - bead-2\n", 1))
		s133Commit(t, f.specWT, "second plan bead")
		preflightGit(t, f.root, "branch", "bead/bead-2", "spec/010-test")
		wt := filepath.Join(f.root, ".worktrees/worktree-bead-2")
		preflightGit(t, f.root, "worktree", "add", wt, "bead/bead-2")
		s133Write(t, wt, "spec.txt", "spec work and second bead\n")
		s133Commit(t, wt, "second bead work")
		preflightGit(t, f.root, "worktree", "remove", wt)
	}
	wt := filepath.Join(f.root, ".worktrees/ledger-base")
	preflightGit(t, f.root, "worktree", "add", wt, "bead/bead-1")
	s133Write(t, wt, ".mindspec/specs/010-test/reviews/leftover/dispositions.jsonl", "")
	s133Commit(t, wt, "tracked ledger baseline")
	preflightGit(t, f.root, "worktree", "remove", wt)
	for _, prepare := range beforeMerge {
		prepare(f, ids)
	}
	// Each planned bead has a real naming merge and a passing bead panel.
	for _, id := range ids {
		s133SeedPanel(t, f, "bead", "bead/"+id, id, "pass", "bead-"+id)
		preflightGit(t, f.specWT, "merge", "--no-ff", "-m", "Merge bead/"+id, "bead/"+id)
		merge := preflightGit(t, f.specWT, "rev-parse", "HEAD")
		second := preflightGit(t, f.root, "rev-parse", "bead/"+id)
		if err := f.metadata(id, map[string]interface{}{"mindspec_landed_merge_sha": merge, "mindspec_landed_second_parent": second}); err != nil {
			t.Fatal(err)
		}
	}
	s133Commit(t, f.specWT, "bead review ledgers")
	if finalState != "absent" {
		s133SeedPanel(t, f, "final_review", "spec/010-test", "", finalState, "final")
		s133Commit(t, f.specWT, "final review ledger")
	}
	implWorktreeListFn = f.worktrees.List
	implIsAncestorFn = lifecycle.IsAncestor
	implClosedEpicBeadIDsFn = func(string) ([]string, error) { return ids, nil }
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		if contains(args, "--type=epic") {
			return json.Marshal([]phase.EpicInfo{f.read()["epic-parent"]})
		}
		var children []phase.ChildInfo
		for _, id := range ids {
			children = append(children, phase.ChildInfo{ID: id, Parent: "epic-parent", Status: f.read()[id].Status, IssueType: "task"})
		}
		return json.Marshal(children)
	}))
	show := func(args ...string) ([]byte, error) {
		data, err := json.Marshal(f.read()[args[1]])
		if err != nil {
			return nil, err
		}
		var row map[string]interface{}
		if err := json.Unmarshal(data, &row); err != nil {
			return nil, err
		}
		if args[1] != "epic-parent" {
			row["dependencies"] = []map[string]string{{"id": "epic-parent", "issue_type": "epic", "dependency_type": "parent-child"}}
		}
		return json.Marshal([]interface{}{row})
	}
	t.Cleanup(phase.SetRunBDForTest(show))
	implRunBDFn = show
	// A local fake tracker serves complete's production seams as well as export.
	bdPath, err := exec.LookPath("bd")
	if err != nil {
		t.Fatal(err)
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	script := "#!" + python + "\n" + `import json, os, subprocess, sys
path = os.environ['FINALIZE_TEST_DB']
root = os.environ['S133_FINAL_ROOT']
with open(path) as f: records = json.load(f)
args = sys.argv[1:]
if args[0] == 'show':
    row = records[args[1]].copy()
    if args[1] != 'epic-parent': row['dependencies'] = [{'id':'epic-parent','issue_type':'epic','dependency_type':'parent-child'}]
    print(json.dumps([row]))
elif args[0] == 'close':
    records[args[1]]['status'] = 'closed'
    with open(path, 'w') as f: json.dump(records, f)
elif args[:2] == ['dolt','commit']:
    pass
elif args[0] == 'update':
    if '--metadata' in args:
        records[args[1]]['metadata'] = json.loads(args[args.index('--metadata')+1])
    for i, arg in enumerate(args):
        if arg == '--unset-metadata': records[args[1]]['metadata'].pop(args[i+1], None)
    with open(path, 'w') as f: json.dump(records, f)
elif args[0] == 'export':
    os.makedirs('.beads', exist_ok=True)
    with open('.beads/issues.jsonl', 'w') as f:
        for key in sorted(records): f.write(json.dumps(records[key], sort_keys=True)+'\n')
elif args[:2] == ['worktree','list']:
    out = subprocess.check_output(['git','-C',root,'worktree','list','--porcelain'], text=True)
    rows = []
    for block in out.strip().split('\n\n'):
        fields = dict(line.split(' ',1) for line in block.splitlines() if ' ' in line)
        branch = fields.get('branch','').removeprefix('refs/heads/')
        if branch.startswith('bead/'): rows.append({'name':os.path.basename(fields['worktree']), 'path':fields['worktree'], 'branch':branch})
    print(json.dumps(rows))
elif args[:2] == ['worktree','remove']:
    subprocess.check_call(['git','-C',root,'worktree','remove','--force',os.path.join(root,'.worktrees',args[2])])
else:
    sys.exit('unexpected fake tracker command: '+repr(args))
`
	if err := os.WriteFile(bdPath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("S133_FINAL_ROOT", f.root)
	return f
}

func s133SeedPanel(t *testing.T, f *s133FinalFixture, gate, target, id, state, slug string) {
	t.Helper()
	head := preflightGit(t, f.root, "rev-parse", target)
	specDir := filepath.Join(f.specWT, ".mindspec/specs/010-test")
	roots := panel.Subject(gate, specDir)
	reader := panel.DocumentSubjectTreeReader(gate, specDir, f.exec.LsTree, func(_, ref, name string) ([]byte, error) { return f.exec.FileAtRef(ref, name) })
	digest, err := reader(f.root, head, roots)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(specDir, "reviews", slug)
	p := panel.Panel{VerdictSchema: 2, Spec: "010-test", Gate: gate, Target: target, Round: 1, ReviewedHeadSHA: head, SubjectDigest: digest, ExpectedReviewers: 2, Telemetry: "required",
		Roster: []panel.RosterSlot{{Slot: "R1", Family: "harness", Model: "m", Lens: "correctness"}, {Slot: "R2", Family: "harness", Model: "m", Lens: "correctness"}}}
	if gate == "bead" {
		p.BeadID = &id
	}
	if state == "sha" {
		p.Target = head // Seed the ledger-backed format from an older binary.
	}
	if err := panel.WriteSnapshot(dir, p); err != nil {
		t.Fatal(err)
	}
	paneltest.Fixture{Dir: dir, Panel: p}.Round(t, 2)
	for _, slot := range p.Roster {
		v := paneltest.Verdict("APPROVE")
		v.Slot, v.ReviewedSHA = slot.Slot, head
		if _, err := panel.Submit(panel.SubmitTarget{Dir: dir, Slot: slot.Slot, Round: 1}, paneltest.Bytes(t, v), paneltest.Resolve); err != nil {
			t.Fatal(err)
		}
	}
	res, err := panel.Tally(dir)
	if err != nil || res.IntegrityErr != nil {
		t.Fatalf("panel fixture: %v %+v", err, res)
	}
	if err := panel.WriteCoverageManifest(res); err != nil {
		t.Fatal(err)
	}
	reg := panel.Registration{Dir: dir, Panel: *res.Panel}
	if err := panel.CheckCore3(reg); err != nil {
		t.Fatal(err)
	}
	if d := panel.PanelGateDecision(panel.GateFacts{Reg: &reg, Res: res, HeadSHA: head, SubjectDigest: digest, WorktreeAbsent: true}); d.Action != panel.Allow {
		t.Fatalf("fixture is not a passing panel: %+v", d)
	}
}

func (f *s133FinalFixture) snapshot() string {
	f.t.Helper()
	data, err := json.Marshal(f.read())
	if err != nil {
		f.t.Fatal(err)
	}
	return preflightGit(f.t, f.root, "show-ref") + "\n" + preflightGit(f.t, f.specWT, "status", "--porcelain") + "\n" + string(data)
}

func (f *s133FinalFixture) refuse(want string, opts ImplOpts) error {
	f.t.Helper()
	before := f.snapshot()
	_, err := ApproveImpl(f.root, "010-test", f.exec, opts)
	if err == nil || !strings.Contains(err.Error(), want) {
		f.t.Fatalf("want refusal %q, got %v", want, err)
	}
	if after := f.snapshot(); before != after {
		f.t.Fatalf("refusal mutated state:\nbefore %s\nafter %s", before, after)
	}
	return err
}

func TestS133FinalGate_PresenceBeforeWrites(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, count := range []int{1, 2} {
			t.Run(fmt.Sprintf("%s/%d", policy, count), func(t *testing.T) {
				f := s133FinalFixtureNew(t, policy, count, "absent")
				if policy == "enforce" {
					f.refuse("gate final_review", ImplOpts{SupersedeADR: "ADR-9800"})
					if _, err := os.Stat(filepath.Join(f.specWT, ".mindspec/adr/ADR-9800.md")); !os.IsNotExist(err) {
						t.Fatal("refused approval created a placeholder")
					}
					return
				}
				if _, err := f.approve(); err != nil {
					t.Fatal(err)
				}
				f.assertConverged()
				if text := f.out.String(); strings.Count(text, "warning:") != 1 || !strings.Contains(text, "final_review") {
					t.Fatalf("need exactly one named warning: %s", text)
				}
				meta := f.read()["epic-parent"].Metadata
				if meta["panel_absent"] != true || meta["panel_absent_gate"] != "final_review" || meta["panel_absent_at"] == nil {
					t.Fatalf("absence audit missing: %v", meta)
				}
			})
		}
	}
}

func TestS133FinalGate_SHATarget(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f := s133FinalFixtureNew(t, policy, 2, "sha")
			head := preflightGit(t, f.root, "rev-parse", "spec/010-test~1")
			err := f.refuse("recorded target is "+head, ImplOpts{SupersedeADR: "ADR-9800"})
			s133GateText(t, strings.ReplaceAll(err.Error(), "010-test", "900-example"), "final_review", f.root)
		})
	}
}

func TestS133FinalGate_SubjectDirtAndExclusions(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, kind := range []string{"source", "ledger", "export"} {
			t.Run(policy+"/"+kind, func(t *testing.T) {
				f := s133FinalFixtureNew(t, policy, 1, "pass")
				switch kind {
				case "source":
					s133Write(t, f.specWT, "feature.txt", "unreviewed source\n")
					f.refuse("feature.txt", ImplOpts{})
				case "ledger":
					ledger := filepath.Join(f.dir, "dispositions.jsonl")
					file, err := os.OpenFile(ledger, os.O_APPEND|os.O_WRONLY, 0644)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := file.WriteString("\n"); err != nil {
						t.Fatal(err)
					}
					if err := file.Close(); err != nil {
						t.Fatal(err)
					}
					if _, err := runLifecycleGate(f.root, "010-test", "final_review", f.specWT, nil); err != nil {
						t.Fatalf("subject dirt check counted ledger: %v", err)
					}
					err = f.refuse("panel ledger has uncommitted changes", ImplOpts{})
					if !strings.Contains(err.Error(), "dispositions.jsonl") || strings.Contains(err.Error(), "gate final_review") {
						t.Fatalf("wrong ledger gate: %v", err)
					}
					s133Commit(t, f.specWT, "ledger only")
				case "export":
					s133Write(t, f.specWT, ".beads/issues.jsonl", "export only\n")
				}
				if kind != "source" {
					if _, err := f.approve(); err != nil {
						t.Fatalf("excluded path or ledger-only commit blocked approval: %v", err)
					}
					f.assertConverged()
				}
			})
		}
	}
}

func (f *s133FinalFixture) beadWorktree() string {
	f.t.Helper()
	wt := filepath.Join(f.root, ".worktrees/worktree-bead-1")
	preflightGit(f.t, f.root, "worktree", "add", wt, "bead/bead-1")
	f.worktrees.entries[0].Path = wt
	return wt
}

func (f *s133FinalFixture) completeBead() error {
	f.t.Helper()
	_, err := complete.Run(f.root, "bead-1", "010-test", "", f.exec, complete.CompleteOpts{})
	return err
}

func TestS133FinalGate_UnmergedPlanBeads(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, status := range []string{"closed", "open"} {
			t.Run(policy+"/"+status, func(t *testing.T) {
				f := s133FinalFixtureNew(t, policy, 1, "pass")
				wt := f.beadWorktree()
				s133Write(t, wt, "feature.txt", "unmerged commit\n")
				s133Commit(t, wt, "move bead tip")
				r := f.read()
				b := r["bead-1"]
				b.Status = status
				r["bead-1"] = b
				f.write(r)
				f.refuse("bead-1", ImplOpts{})
			})
		}
	}
}

func TestS133FinalGate_MergedDirtyWorktreesAndRecovery(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, kind := range []string{"tracked discard", "untracked discard", "tracked commit", "skip", "disabled", "export", "ledger", "clean"} {
			t.Run(policy+"/"+kind, func(t *testing.T) {
				f := s133FinalFixtureNew(t, policy, 1, "pass")
				wt := f.beadWorktree()
				if kind == "clean" {
					if _, err := f.approve(); err != nil {
						t.Fatal(err)
					}
					f.assertConverged()
					return
				}
				name := "feature.txt"
				original := "bead work\n"
				switch kind {
				case "untracked discard":
					name = "untracked-source.txt"
				case "export":
					name = ".beads/issues.jsonl"
				case "ledger":
					name = ".mindspec/specs/010-test/reviews/leftover/dispositions.jsonl"
				}
				if kind == "skip" {
					t.Setenv(panel.SkipPanelEnv, "1")
				}
				if kind == "disabled" {
					s133Write(t, f.root, ".mindspec/config.yaml", "enforcement:\n  panel_gate: false\n  panel_presence: "+policy+"\n")
					config.ResetCache()
				}
				s133Write(t, wt, name, "pending\n")
				// Pending own writes in the spec expose a gate placed inside FinalizeEpic.
				if _, err := adr.CreateWithID(f.specWT, "ADR-9801", "Placeholder for ADR-9801", adr.CreateOpts{}); err != nil {
					t.Fatal(err)
				}
				beforeBead := preflightGit(t, wt, "status", "--porcelain")
				err := f.refuse(name, ImplOpts{})
				if !strings.Contains(err.Error(), "bead-1") || !strings.Contains(err.Error(), "recovery: mindspec complete bead-1") || beforeBead != preflightGit(t, wt, "status", "--porcelain") {
					t.Fatalf("missing identity or changed bead state: %v", err)
				}
				exportOnly := kind == "export" || kind == "ledger"
				if exportOnly {
					if strings.Contains(err.Error(), "panel create") || strings.Contains(err.Error(), "commit or discard") || strings.Count(err.Error(), "recovery:") != 1 {
						t.Fatalf("export-only recovery must only complete: %v", err)
					}
				} else {
					if strings.Index(err.Error(), "commit or discard") > strings.Index(err.Error(), "recovery: mindspec complete") || !strings.Contains(err.Error(), "--gate final_review --target spec/010-test") {
						t.Fatalf("wrong recovery order: %v", err)
					}
					if err := f.completeBead(); err == nil {
						t.Fatal("complete accepted uncommitted user changes")
					}
					if kind == "tracked commit" {
						s133Commit(t, wt, "pending user changes")
						err := f.completeBead()
						if err == nil || !strings.Contains(err.Error(), "follow-up") {
							t.Fatalf("moved tip must require follow-up bead: %v", err)
						}
						f.refuse("bead-1", ImplOpts{})
						return
					}
					if kind == "untracked discard" {
						if err := os.Rename(filepath.Join(wt, name), filepath.Join(t.TempDir(), "preserved")); err != nil {
							t.Fatal(err)
						}
					} else {
						s133Write(t, wt, name, original)
					}
				}
				if err := f.completeBead(); err != nil {
					t.Fatalf("completion recovery failed: %v", err)
				}
				if _, err := os.Stat(wt); !os.IsNotExist(err) {
					t.Fatal("complete did not remove bead worktree")
				}
				f.worktrees.entries = nil
				if _, err := f.approve(); err != nil {
					t.Fatalf("approval after complete: %v", err)
				}
			})
		}
	}
}

func TestS133FinalGate_FaultsAndReruns(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, site := range []string{"spec commit", "epic close", "placeholder"} {
			t.Run(policy+"/"+site, func(t *testing.T) {
				f := s133FinalFixtureNew(t, policy, 1, "pass")
				opts := ImplOpts{}
				switch site {
				case "spec commit":
					// The auto_merge hook follows the spec artifact commit. All beads
					// are merged, so this leg makes no further content change.
					if _, err := adr.CreateWithID(f.specWT, "ADR-9801", "Placeholder for ADR-9801", adr.CreateOpts{}); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(executor.SetFinalizeStepHookForTest(func(stage string) error {
						if stage == "auto_merge" {
							return errors.New("fault after spec artifact commit")
						}
						return nil
					}))
				case "epic close":
					implPhaseMetadataFn = func(string, map[string]interface{}) error { return errors.New("fault after epic close") }
				case "placeholder":
					opts.SupersedeADR = "ADR-9800"
					implCreateWithIDFn = func(root, id, title string, opts adr.CreateOpts) (string, error) {
						name, err := adr.CreateWithID(root, id, title, opts)
						if err != nil {
							t.Fatal(err)
						}
						return name, errors.New("fault after placeholder write")
					}
				}
				before := preflightGit(t, f.specWT, "rev-parse", "HEAD")
				if _, err := ApproveImpl(f.root, "010-test", f.exec, opts); err == nil || !strings.Contains(err.Error(), "fault after") {
					t.Fatalf("fault did not fire: %v", err)
				}
				if site == "spec commit" && before == preflightGit(t, f.specWT, "rev-parse", "HEAD") {
					t.Fatal("fault did not follow a real spec commit")
				}
				if site == "epic close" && f.read()["epic-parent"].Status != "closed" {
					t.Fatal("fault did not follow real epic close")
				}
				executor.SetFinalizeStepHookForTest(nil)
				implPhaseMetadataFn = f.metadata
				implCreateWithIDFn = adr.CreateWithID
				if _, err := ApproveImpl(f.root, "010-test", f.exec, opts); err != nil {
					t.Fatalf("same run did not converge: %v", err)
				}
				f.assertConverged()
			})
		}
	}
}

func TestS133FinalGate_PlaceholderPermissionsRerun(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f := s133FinalFixtureNew(t, policy, 1, "pass")
			opts := ImplOpts{SupersedeADR: "ADR-9800"}
			var placeholder string
			implCreateWithIDFn = func(root, id, title string, opts adr.CreateOpts) (string, error) {
				name, err := adr.CreateWithID(root, id, title, opts)
				if err != nil {
					t.Fatal(err)
				}
				placeholder = name
				return name, errors.New("fault after placeholder write")
			}
			if _, err := ApproveImpl(f.root, "010-test", f.exec, opts); err == nil || !strings.Contains(err.Error(), "fault after placeholder write") {
				t.Fatalf("fault did not follow a real placeholder write: %v", err)
			}
			// Set permissions explicitly so the retry does not depend on the process umask.
			if err := os.Chmod(placeholder, 0o600); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(placeholder)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0o600 {
				t.Fatalf("placeholder permissions = %o, want 600", info.Mode().Perm())
			}
			implCreateWithIDFn = adr.CreateWithID
			if _, err := ApproveImpl(f.root, "010-test", f.exec, opts); err != nil {
				t.Fatalf("same run did not converge with private placeholder permissions: %v", err)
			}
			f.assertConverged()
		})
	}
}

func TestS133FinalGate_PlaceholderContentStales(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f := s133FinalFixtureNew(t, policy, 1, "pass")
			name, err := adr.CreateWithID(f.specWT, "ADR-9800", "Placeholder for ADR-9800", adr.CreateOpts{})
			if err != nil {
				t.Fatal(err)
			}
			s133Commit(t, f.specWT, "own placeholder")
			if _, err := runLifecycleGate(f.root, "010-test", "final_review", f.specWT, nil); err != nil {
				t.Fatalf("exact placeholder is own content: %v", err)
			}
			data, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			s133Write(t, f.specWT, ".mindspec/adr/ADR-9800.md", string(data)+"\nA new decision.\n")
			s133Commit(t, f.specWT, "edit placeholder")
			err = f.refuse("commits landed after review", ImplOpts{SupersedeADR: "ADR-9800"})
			if !strings.Contains(err.Error(), "gate final_review") {
				t.Fatalf("need stale-subject leg: %v", err)
			}
		})
	}
}

func TestS133FinalGate_ClosedEpicRerunsGate(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f := s133FinalFixtureNew(t, policy, 1, "pass")
			t.Cleanup(executor.SetFinalizeStepHookForTest(func(stage string) error {
				if stage == "auto_merge" {
					return errors.New("fault after close and spec commit")
				}
				return nil
			}))
			if _, err := f.approve(); err == nil || !strings.Contains(err.Error(), "fault after close") {
				t.Fatalf("fault did not fire: %v", err)
			}
			if epic := f.read()["epic-parent"]; epic.Status != "closed" || epic.Metadata["mindspec_phase"] != "done" {
				t.Fatalf("fault did not leave closed/done: %+v", epic)
			}
			executor.SetFinalizeStepHookForTest(nil)
			s133Write(t, f.specWT, "feature.txt", "new source after fault\n")
			s133Commit(t, f.specWT, "unreviewed retry content")
			f.refuse("commits landed after review", ImplOpts{})
		})
	}
}

func TestS133FinalGate_NoWorktreeAndAlreadyLanded(t *testing.T) {
	t.Run("already landed", func(t *testing.T) {
		f := s133FinalFixtureNew(t, "enforce", 1, "pass")
		if _, err := f.approve(); err != nil {
			t.Fatal(err)
		}
		f.assertConverged()
		result, err := f.approve()
		if err != nil || result == nil || !strings.Contains(strings.Join(result.Warnings, "\n"), "already landed") {
			t.Fatalf("branchless replay: %+v %v", result, err)
		}
	})
	for _, policy := range []string{"warn", "enforce"} {
		for _, change := range []string{"same", "excluded", "source"} {
			t.Run(policy+"/"+change, func(t *testing.T) {
				f := s133FinalFixtureNew(t, policy, 1, "pass")
				t.Cleanup(executor.SetFinalizeStepHookForTest(func(stage string) error {
					if stage == "pre_cleanup" {
						return errors.New("fault before cleanup")
					}
					return nil
				}))
				if _, err := f.approve(); err == nil || !strings.Contains(err.Error(), "fault before cleanup") {
					t.Fatalf("publication fault did not fire: %v", err)
				}
				executor.SetFinalizeStepHookForTest(nil)
				if change != "same" {
					name := "feature.txt"
					if change == "excluded" {
						name = ".beads/issues.jsonl"
					}
					s133Write(t, f.specWT, name, "new committed content\n")
					s133Commit(t, f.specWT, "after publication")
				}
				preflightGit(t, f.root, "worktree", "remove", f.specWT)
				before := preflightGit(t, f.root, "show-ref")
				data, err := os.ReadFile(f.db)
				if err != nil {
					t.Fatal(err)
				}
				if change == "excluded" {
					// Only subject equality is under test here. The existing merge
					// destruction guard can still refuse an export-only stale snapshot.
					_, replay, err := implFinalReviewGate(f.root, "010-test", "epic-parent", "done", filepath.Join(f.root, ".mindspec/specs/010-test"), f.exec)
					if err != nil || !replay {
						t.Fatalf("excluded content changed published subject: replay=%v %v", replay, err)
					}
					return
				}
				result, err := f.approve()
				if change == "source" {
					if err == nil || !strings.Contains(err.Error(), "gate final_review") || !strings.HasSuffix(err.Error(), "recovery: re-create the spec worktree at spec/010-test, then run a final-review panel") {
						t.Fatalf("unreviewed content passed missing-worktree recovery: %v", err)
					}
					after, readErr := os.ReadFile(f.db)
					if readErr != nil || !bytes.Equal(data, after) || before != preflightGit(t, f.root, "show-ref") {
						t.Fatal("missing-worktree refusal mutated state")
					}
				} else if err != nil || result == nil {
					t.Fatalf("same published subject must resume: %+v %v", result, err)
				}
			})
		}
	}
}

func TestS133FinalGate_ProductMessagesHaveNoInternalReferences(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "impl_final_gate.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	forbidden := regexp.MustCompile(`(?i)ADR-[0-9]+|spec[ -][0-9]+|\bR[0-9]+\.[0-9]+|mindspec-[a-z0-9]+|#[0-9]+`)
	// Positive control: the scanner must recognize each forbidden class.
	for _, text := range []string{"ADR-0041", "spec 133", "R2.5", "mindspec-example", "#212"} {
		if !forbidden.MatchString(text) {
			t.Fatalf("scanner missed %q", text)
		}
	}
	literals := 0
	ast.Inspect(f, func(node ast.Node) bool {
		lit, ok := node.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		text, err := strconv.Unquote(lit.Value)
		if err != nil {
			t.Fatal(err)
		}
		literals++
		if forbidden.MatchString(text) {
			t.Errorf("internal reference in product string: %q", text)
		}
		return true
	})
	if literals == 0 {
		t.Fatal("no product literals scanned")
	}
}
