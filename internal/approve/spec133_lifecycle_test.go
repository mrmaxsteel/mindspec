package approve

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

func s133GateFixture(t *testing.T, policy, gate, state string) (string, string) {
	t.Helper()
	root := t.TempDir()
	write := func(name, text string) {
		t.Helper()
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(".mindspec/config.yaml", "enforcement:\n  panel_presence: "+policy+"\n")
	write(".mindspec/specs/900-example/spec.md", "---\nstatus: Draft\ntitle: Example\n---\n# Spec\n")
	write(".gitignore", ".mindspec/specs/*/reviews/\n")
	preflightGit(t, root, "init", "-q", "-b", "spec/900-example")
	preflightGit(t, root, "add", ".")
	preflightGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "base")
	exec := executor.NewMindspecExecutor(root)
	head, err := exec.RevParseRef(root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, ".mindspec/specs/900-example/reviews/review")
	if state != "absent" {
		specDir := filepath.Join(root, ".mindspec/specs/900-example")
		digest, err := panel.DocumentSubjectTreeReader(gate, specDir, exec.LsTree, func(_, ref, name string) ([]byte, error) { return exec.FileAtRef(ref, name) })(root, head, panel.Subject(gate, specDir))
		if err != nil {
			t.Fatal(err)
		}
		paneltest.Lifecycle(t, dir, "900-example", gate, head, digest, state)
		if state == "stale" || state == "dirty" {
			write(".mindspec/specs/900-example/spec.md", "# Changed body\n")
			if state == "stale" {
				preflightGit(t, root, "add", ".")
				preflightGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "edit")
			}
		}
	}
	config.ResetCache()
	t.Cleanup(config.ResetCache)
	t.Setenv(panel.SkipPanelEnv, "")
	return root, dir
}

func TestS133LifecycleGate_FailingLegs(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, gate := range []string{"spec_approve", "plan_approve", "final_review"} {
			for _, state := range []string{"malformed", "integrity", "round mismatch", "stale", "dirty", "incomplete", "reject", "hard block", "unresolved", "coverage"} {
				t.Run(policy+"/"+gate+"/"+state, func(t *testing.T) {
					root, _ := s133GateFixture(t, policy, gate, state)
					var out bytes.Buffer
					o, err := runLifecycleGate(root, "900-example", gate, root, &out)
					if err == nil || o.Action != panel.Block {
						t.Fatalf("failing %s became pass/warn: %+v %v %s", state, o, err, out.String())
					}
					message := err.Error()
					want := map[string]string{"malformed": "registration", "integrity": "outside the panel verbs", "round mismatch": "out of date", "stale": "commits landed after review", "dirty": "uncommitted changes", "incomplete": "incomplete", "reject": "REJECT", "hard block": "hard_block", "unresolved": "unresolved", "coverage": "coverage"}[state]
					if !strings.Contains(message, want) {
						t.Fatalf("wrong failing leg, want %q: %s", want, message)
					}
					s133GateText(t, message, gate, root)
				})
			}
		}
	}
}

func s133GateText(t *testing.T, text, gate, root string) {
	t.Helper()
	for _, want := range []string{"gate " + gate, "panel ", "recovery:"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q: %s", want, text)
		}
	}
	if strings.Count(text, "recovery:") != 1 {
		t.Errorf("expected one recovery: %s", text)
	}
	for _, bad := range []string{"Do NOT bypass", "raw `git merge", "/ms-bead-fix", panel.SkipPanelEnv} {
		if strings.Contains(text, bad) {
			t.Errorf("forbidden %q: %s", bad, text)
		}
	}
	// Runtime paths are operator data, not authored gate guidance. Strip both
	// spellings because Git and temporary directories may resolve symlinks.
	referenceText := text
	for _, path := range []string{root, os.TempDir()} {
		realPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, runtimePath := range []string{path, realPath} {
			referenceText = strings.ReplaceAll(referenceText, runtimePath, "<path>")
		}
	}
	if regexp.MustCompile(`(?i)ADR-[0-9]+|spec[ -][0-9]+|\bR[0-9]+\.[0-9]+|mindspec-[a-z0-9]+|#[0-9]+`).MatchString(strings.ReplaceAll(referenceText, "900-example", "<id>")) {
		t.Errorf("internal reference: %s", text)
	}
}

func TestS133LifecycleGate_PresenceAndHatches(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, gate := range []string{"spec_approve", "plan_approve", "final_review"} {
			for _, state := range []string{"absent", "pass", "abandoned", "skip", "disabled"} {
				t.Run(policy+"/"+gate+"/"+state, func(t *testing.T) {
					root, _ := s133GateFixture(t, policy, gate, state)
					if state == "skip" {
						t.Setenv(panel.SkipPanelEnv, "1")
					}
					if state == "disabled" {
						if err := os.WriteFile(filepath.Join(root, ".mindspec/config.yaml"), []byte("enforcement:\n  panel_gate: false\n  panel_presence: "+policy+"\n"), 0644); err != nil {
							t.Fatal(err)
						}
						config.ResetCache()
					}
					var out bytes.Buffer
					o, err := runLifecycleGate(root, "900-example", gate, root, &out)
					if state == "absent" && policy == "enforce" {
						if err == nil || o.Action != panel.Block {
							t.Fatalf("missing presence refusal: %+v %v", o, err)
						}
						want := "recovery: mindspec panel create <slug> --spec 900-example --gate " + gate + " --target spec/900-example"
						if !strings.HasSuffix(err.Error(), want) {
							t.Fatalf("wrong recovery: %v", err)
						}
						s133GateText(t, err.Error(), gate, root)
						return
					}
					if err != nil || o.Action == panel.Block {
						t.Fatalf("unexpected refusal: %+v %v", o, err)
					}
					wantWarn := state == "absent" || state == "abandoned" || state == "skip"
					if (o.Action == panel.Warn) != wantWarn || (out.Len() > 0) != wantWarn {
						t.Fatalf("wrong hatch outcome: %+v %s", o, out.String())
					}
					if o.Absent != (state == "absent") || o.Skipped != (state == "skip") || o.Abandoned != (state == "abandoned") {
						t.Fatalf("wrong audit: %+v", o)
					}
				})
			}
		}
	}
}

func TestS133LifecycleGate_Audit(t *testing.T) {
	orig := lifecycleMergeMetadataFn
	t.Cleanup(func() { lifecycleMergeMetadataFn = orig })
	for _, o := range []Outcome{{Gate: "spec_approve", Skipped: true}, {Gate: "plan_approve", Abandoned: true, AbandonReason: "stopped"}, {Gate: "final_review", Absent: true}, {Gate: "spec_approve"}} {
		for _, ids := range [][]string{nil, {}, {"deferral-one", "deferral-two"}} {
			t.Run(fmt.Sprintf("%+v/%v", o, ids), func(t *testing.T) {
				var got map[string]interface{}
				lifecycleMergeMetadataFn = func(id string, meta map[string]interface{}) error {
					if id != "epic" {
						t.Fatalf("wrong epic %s", id)
					}
					got = meta
					return nil
				}
				if err := recordLifecycleGateAudit("epic", o, ids); err != nil {
					t.Fatal(err)
				}
				for key, want := range map[string]bool{"panel_gate_skipped": o.Skipped, "panel_abandoned": o.Abandoned, "panel_absent": o.Absent} {
					_, present := got[key]
					if present != want {
						t.Fatalf("%s=%v want %v: %v", key, present, want, got)
					}
					if want {
						if got[key] != true {
							t.Fatal("audit value must be true")
						}
						if _, err := time.Parse(time.RFC3339, got[key+"_at"].(string)); err != nil {
							t.Fatal(err)
						}
					}
				}
				_, present := got["plan_deferral_ids"]
				if present != (ids != nil) {
					t.Fatalf("nil-list contract broken: %v", got)
				}
				if ids != nil && !reflect.DeepEqual(got["plan_deferral_ids"], ids) {
					t.Fatalf("ids=%v", got)
				}
				if o.Abandoned && got["panel_abandoned_reason"] != "stopped" {
					t.Fatal("reason missing")
				}
				if o.Absent && got["panel_absent_gate"] != o.Gate {
					t.Fatal("gate missing")
				}
			})
		}
	}
}

func TestS133LifecycleGate_TargetAndAllRegistrations(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, gate := range []string{"spec_approve", "plan_approve", "final_review"} {
			for _, state := range []string{"old target", "second incomplete", "renamed gate"} {
				t.Run(policy+"/"+gate+"/"+state, func(t *testing.T) {
					root, dir := s133GateFixture(t, policy, gate, "pass")
					res, err := panel.Tally(dir)
					if err != nil {
						t.Fatal(err)
					}
					p := *res.Panel
					switch state {
					case "old target":
						p.Target = "old-target"
						if err := panel.WriteSnapshot(dir, p); err != nil {
							t.Fatal(err)
						}
					case "renamed gate":
						p.Gate = "adhoc"
						if err := panel.WriteSnapshot(dir, p); err != nil {
							t.Fatal(err)
						}
					case "second incomplete":
						paneltest.Lifecycle(t, filepath.Join(filepath.Dir(dir), "second"), p.Spec, gate, p.ReviewedHeadSHA, p.SubjectDigest, "incomplete")
					}
					o, err := runLifecycleGate(root, "900-example", gate, root, nil)
					if err == nil || o.Action != panel.Block {
						t.Fatalf("bad registration passed: %+v %v", o, err)
					}
					s133GateText(t, err.Error(), gate, root)
					if state == "old target" && !strings.Contains(err.Error(), "old-target") {
						t.Fatal("target not named")
					}
					if state == "second incomplete" && !strings.Contains(err.Error(), "second") {
						t.Fatal("second panel not named")
					}
				})
			}
		}
	}
}

func TestS133LifecycleGate_OwnWritesAndExclusions(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, gate := range []string{"spec_approve", "plan_approve"} {
			t.Run(policy+"/"+gate, func(t *testing.T) {
				root, _ := s133GateFixture(t, policy, gate, "pass")
				name := filepath.Join(root, ".mindspec/specs/900-example/spec.md")
				// Exercise the actual writer, including the absent-section path.
				if err := updateSpecApproval(name, "operator"); err != nil {
					t.Fatal(err)
				}
				plan := filepath.Join(root, ".mindspec/specs/900-example/plan.md")
				if err := os.WriteFile(plan, []byte(scaffoldPlan("900-example")), 0644); err != nil {
					t.Fatal(err)
				}
				o, err := runLifecycleGate(root, "900-example", gate, root, nil)
				if err != nil || o.Action != panel.Allow {
					t.Fatalf("approval own writes refused: %+v %v", o, err)
				}
				preflightGit(t, root, "add", ".")
				preflightGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "approval")
				o, err = runLifecycleGate(root, "900-example", gate, root, nil)
				if err != nil || o.Action != panel.Allow {
					t.Fatalf("committed own writes stale: %+v %v", o, err)
				}
			})
		}
	}
}
