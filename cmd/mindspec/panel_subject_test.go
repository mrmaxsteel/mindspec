package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/complete"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/instruct"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func subjectCLIGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.name=Subject Test", "-c", "user.email=subject@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/usr/bin/false")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func subjectCLIWrite(t *testing.T, root, file, text string) {
	t.Helper()
	file = filepath.Join(root, file)
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestPanelConsumers_SubjectTreeParity(t *testing.T) {
	for _, specs := range []string{".mindspec/specs", ".mindspec/docs/specs"} {
		for _, gateCase := range []string{"spec_approve", "plan_approve", "bead", "final_review", "adhoc", "adhoc bead", "adhoc bead target", "", "omitted gate"} {
			gate := gateCase
			switch gateCase {
			case "adhoc bead", "adhoc bead target":
				gate = "adhoc"
			case "":
				// Completion matching requires a bead gate and a matching target.
				gate = "bead"
			case "omitted gate":
				gate = ""
			}
			changes := []string{"archive", "ownership", "real edit", "internal reviews"}
			if gate == "spec_approve" {
				changes = append(changes, "target off checkout")
			}
			for _, change := range changes {
				t.Run(specs+"/"+gateCase+"/"+change, func(t *testing.T) {
					resetPanelCreateFlags(t)
					stubWorktreeListEmpty(t)
					config.ResetCache()
					t.Cleanup(config.ResetCache)
					root := t.TempDir()
					withTestChdir(t, root)
					subjectCLIGit(t, root, "init", "-q", "-b", "main")
					subjectCLIWrite(t, root, specs+"/130-example/spec.md", "base\n")
					subjectCLIWrite(t, root, "internal/reviews/x.go", "base\n")
					subjectCLIWrite(t, root, ".mindspec/domains/workflow/OWNERSHIP.yaml", "base\n")
					// Canonical fixtures must not contain flat lifecycle markers.
					if specs == ".mindspec/docs/specs" {
						// OWNERSHIP is an ordinary subject file in this fixture's domains tree.
						// Only specs/adr/core/domain DIRECTORY markers determine layout.
						// Use the canonical domains path instead of a flat lifecycle marker.
						source := filepath.Join(root, ".mindspec/domains")
						dest := filepath.Join(root, ".mindspec/docs/domains")
						if err := os.Rename(source, dest); err != nil {
							t.Fatal(err)
						}
					}
					subjectCLIGit(t, root, "add", "-A")
					subjectCLIGit(t, root, "commit", "-qm", "base")
					target := "main"
					bead := gate == "bead"
					if bead || gate == "" || gateCase == "adhoc bead" || gateCase == "adhoc bead target" {
						target = "bead/mindspec-subject.1"
						subjectCLIGit(t, root, "switch", "-qc", target)
					}
					lsTreeForPanelFn = func(workdir, ref string) (string, error) { return (&executor.MindspecExecutor{}).LsTree(workdir, ref) }
					args := []string{"create", "subject", "--target", target}
					if gate != "" {
						args = append(args, "--gate", gate)
					}
					if gate != "adhoc" {
						args = append(args, "--spec", "130-example")
					}
					if bead || gateCase == "adhoc bead" {
						args = append(args, "--bead", "mindspec-subject.1")
					}
					if gateCase == "adhoc bead" {
						// This binding is refused; the target-only sibling retains
						// the workspace-root subject and staleness coverage.
						if msg := assertPanelRefusal(t, root, args...); !strings.Contains(msg, "--bead requires --gate bead") {
							t.Fatalf("unexpected refusal: %s", msg)
						}
						return
					}
					if out, err := runPanelCmd(args...); err != nil {
						t.Fatalf("create: %v\n%s", err, out)
					}
					reg, err := findPanelRegistration(root, "subject")
					if err != nil {
						t.Fatal(err)
					}
					if gate == "adhoc" && specs == ".mindspec/specs" && reg.Dir != filepath.Join(root, ".mindspec/reviews/subject") {
						t.Fatalf("adhoc bead panel at %s, want workspace review root", reg.Dir)
					}
					if !strings.HasPrefix(reg.Panel.SubjectDigest, "sha256:") || len(reg.Panel.SubjectDigest) != 71 {
						t.Fatalf("create did not record digest: %+v", reg.Panel)
					}
					for i := 1; i <= reg.Panel.ExpectedReviewers; i++ {
						slot := reg.Panel.Roster[i-1]
						verdict := panel.VerdictV2{Schema: 2, Slot: slot.Slot, Round: 1, ReviewedSHA: reg.Panel.ReviewedHeadSHA, Status: "final", Verdict: "APPROVE", Model: slot.Model, Rationale: "reviewed", ConcreteChangesRequired: []panel.ChangeRequired{}, CommandsRun: []panel.CommandRun{}}
						raw, err := json.Marshal(verdict)
						if err != nil {
							t.Fatal(err)
						}
						if _, err := panel.Submit(panel.SubmitTarget{Dir: reg.Dir, Slot: slot.Slot, Round: 1}, raw, func(_ panel.Panel, s panel.RosterSlot) (panel.SubmitResolution, error) {
							return panel.SubmitResolution{Family: s.Family, Model: s.Model, RunnerKind: "harness", RunnerSource: "default"}, nil
						}); err != nil {
							t.Fatal(err)
						}
					}
					file := ""
					wantStale := change != "archive"
					switch change {
					case "archive":
						file = specs + "/130-example/reviews/old/BRIEF-round-1.md"
					case "ownership":
						file = ".mindspec/domains/workflow/OWNERSHIP.yaml"
						if specs == ".mindspec/docs/specs" {
							file = ".mindspec/docs/domains/workflow/OWNERSHIP.yaml"
						}
					case "real edit", "target off checkout":
						file = specs + "/130-example/spec.md"
					case "internal reviews":
						file = "internal/reviews/x.go"
					}
					if gate == "spec_approve" || gate == "plan_approve" {
						wantStale = change == "real edit" || change == "target off checkout"
					}
					subjectCLIWrite(t, root, file, "changed\n")
					subjectCLIGit(t, root, "add", "-f", "--", file)
					subjectCLIGit(t, root, "commit", "-qm", "subject change")
					if change == "target off checkout" {
						subjectCLIGit(t, root, "switch", "-q", "--detach", "HEAD~1")
						if head := subjectCLIGit(t, root, "rev-parse", "HEAD"); head != reg.Panel.ReviewedHeadSHA {
							t.Fatalf("HEAD=%s, want reviewed SHA %s", head, reg.Panel.ReviewedHeadSHA)
						}
						if tip := subjectCLIGit(t, root, "rev-parse", target); tip == reg.Panel.ReviewedHeadSHA {
							t.Fatal("target must differ from the reviewed SHA")
						}
					}
					// Verify is read-only and always exits zero; assert its rendered decision.
					out, err := runPanelCmd("verify", "subject")
					if err != nil {
						t.Fatalf("verify: %v %s", err, out)
					}
					want := "gate would PASS"
					if wantStale {
						want = "gate would BLOCK"
					}
					verifyWant := "\nPASS\n"
					if wantStale {
						verifyWant = "\nBLOCK:"
					}
					if !strings.Contains(out, verifyWant) {
						t.Fatalf("verify want %s:\n%s", want, out)
					}
					out, err = runPanelCmd("tally", "subject")
					if (err != nil) != wantStale {
						t.Fatalf("tally stale=%v err=%v:\n%s", wantStale, err, out)
					}
					roots := []string{root, filepath.Join(root, specs, "130-example"), filepath.Join(root, ".mindspec")}
					state := instruct.PanelState(roots...)
					if !strings.Contains(state, want) {
						t.Fatalf("instruct want %s:\n%s", want, state)
					}
					if bead {
						var warnings bytes.Buffer
						matched, err := complete.PanelGate("mindspec-subject.1", roots, "", true, &warnings)
						if matched == nil || (err != nil) != wantStale || warnings.Len() != 0 {
							t.Fatalf("complete stale=%v matched=%v err=%v warnings=%s", wantStale, matched != nil, err, warnings.String())
						}
					}
				})
			}
		}
	}
}

func TestSubjectDigest_FailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name, listing string
		err           error
	}{
		{"ls-tree error", "", errors.New("unreadable tree")}, {"empty tree", "", nil}, {"excluded only", "100644 blob abc\t.beads/issues.jsonl\n", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetPanelCreateFlags(t)
			root := mkFlatPanelTestRoot(t)
			withTestChdir(t, root)
			config.ResetCache()
			t.Cleanup(config.ResetCache)
			stubNonBeadRevParse(t, func(string, string) (string, error) { return "abc123", nil })
			lsTreeForPanelFn = func(string, string) (string, error) { return tc.listing, tc.err }
			before := snapshotTree(t, root)
			out, err := runPanelCmd("create", "subject", "--gate", "adhoc", "--target", "main")
			if err == nil {
				t.Fatalf("create must refuse: %s", out)
			}
			after := snapshotTree(t, root)
			b, _ := json.Marshal(before)
			a, _ := json.Marshal(after)
			if string(a) != string(b) {
				t.Fatalf("refused create wrote files: before %s after %s", b, a)
			}
		})
	}
}

func TestSubjectDigest_CreateUsesTargetCommit(t *testing.T) {
	resetPanelCreateFlags(t)
	config.ResetCache()
	t.Cleanup(config.ResetCache)
	root := t.TempDir()
	withTestChdir(t, root)
	subjectCLIGit(t, root, "init", "-q", "-b", "main")
	subjectCLIWrite(t, root, ".mindspec/specs/130-example/spec.md", "reviewed target\n")
	subjectCLIGit(t, root, "add", "-A")
	subjectCLIGit(t, root, "commit", "-qm", "target")
	targetSHA := subjectCLIGit(t, root, "rev-parse", "HEAD")
	subjectCLIGit(t, root, "branch", "review-target")
	subjectCLIWrite(t, root, ".mindspec/specs/130-example/spec.md", "different HEAD\n")
	subjectCLIGit(t, root, "add", "-A")
	subjectCLIGit(t, root, "commit", "-qm", "head")
	if head := subjectCLIGit(t, root, "rev-parse", "HEAD"); head == targetSHA {
		t.Fatal("fixture requires HEAD != target")
	}
	listing := subjectCLIGit(t, root, "ls-tree", "-r", "--full-tree", targetSHA, "--") + "\n"
	want, err := panel.SubjectDigest(listing, panel.Subject("adhoc", ".mindspec/specs"))
	if err != nil {
		t.Fatal(err)
	}
	var refs []string
	lsTreeForPanelFn = func(workdir, ref string) (string, error) {
		refs = append(refs, ref)
		return (&executor.MindspecExecutor{}).LsTree(workdir, ref)
	}
	if out, err := runPanelCmd("create", "subject", "--gate", "adhoc", "--target", "review-target"); err != nil {
		t.Fatalf("create: %v\n%s", err, out)
	}
	reg, err := findPanelRegistration(root, "subject")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0] != targetSHA {
		t.Errorf("ls-tree refs=%v, want resolved target %s", refs, targetSHA)
	}
	if reg.Panel.ReviewedHeadSHA != targetSHA || reg.Panel.SubjectDigest != want {
		t.Errorf("recorded SHA=%s digest=%s, want target SHA=%s digest=%s", reg.Panel.ReviewedHeadSHA, reg.Panel.SubjectDigest, targetSHA, want)
	}
}

func TestSubjectDigest_QuotedSpecPath(t *testing.T) {
	root := t.TempDir()
	subjectCLIGit(t, root, "init", "-q", "-b", "main")
	subjectCLIGit(t, root, "config", "core.quotePath", "true")
	const specDir = ".mindspec/specs/130-example"
	const file = specDir + "/spé.md"
	subjectCLIWrite(t, root, specDir+"/spec.md", "base\n")
	subjectCLIWrite(t, root, file, "base\n")
	subjectCLIGit(t, root, "add", "-A")
	subjectCLIGit(t, root, "commit", "-qm", "base")
	digest := func() string {
		t.Helper()
		listing, err := (&executor.MindspecExecutor{}).LsTree(root, "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(listing, `".mindspec/specs/130-example/sp\303\251.md"`) {
			t.Fatalf("listing must contain a quoted non-ASCII spec path: %s", listing)
		}
		got, err := panel.SubjectDigest(listing, panel.Subject("spec_approve", specDir))
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	before := digest()
	subjectCLIWrite(t, root, file, "changed\n")
	subjectCLIGit(t, root, "add", "--", file)
	subjectCLIGit(t, root, "commit", "-qm", "quoted path edit")
	if after := digest(); after == before {
		t.Fatal("edit to quoted spec path did not change digest")
	}
}
