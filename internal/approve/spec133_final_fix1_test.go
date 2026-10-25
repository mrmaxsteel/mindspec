package approve

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

func TestS133Fix1_PlaceholderWriteSet(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, storage := range []string{"committed", "untracked", "modified"} {
			for _, kind := range []string{"other id", "no flag", "changed domains"} {
				t.Run(policy+"/"+storage+"/"+kind, func(t *testing.T) {
					f := s133FinalFixtureNew(t, policy, 1, "pass")
					opts := ImplOpts{SupersedeADR: "ADR-9800"}
					id := "ADR-9801"
					if kind == "no flag" {
						opts.SupersedeADR = ""
					}
					if kind == "changed domains" {
						id = "ADR-9800"
					}
					name, err := adr.CreateWithID(f.specWT, id, "Placeholder for "+id, adr.CreateOpts{Domains: []string{"unreviewed domain"}})
					if err != nil {
						t.Fatal(err)
					}
					if storage == "committed" {
						s133Commit(t, f.specWT, "unreviewed placeholder")
					}
					if storage == "modified" {
						s133Commit(t, f.specWT, "placeholder before review")
						s133SeedPanel(t, f, "final_review", "spec/010-test", "", "pass", "final-new")
						s133Commit(t, f.specWT, "new review ledger")
						data, err := os.ReadFile(name)
						if err != nil {
							t.Fatal(err)
						}
						s133Write(t, f.specWT, filepath.ToSlash(strings.TrimPrefix(name, f.specWT+string(filepath.Separator))), strings.Replace(string(data), "unreviewed domain", "changed domain", 1))
					}
					before, err := os.ReadFile(name)
					if err != nil {
						t.Fatal(err)
					}
					refusal := f.refuse("gate final_review", opts)
					rel, err := filepath.Rel(f.specWT, name)
					if err != nil {
						t.Fatal(err)
					}
					if !strings.Contains(refusal.Error(), filepath.ToSlash(rel)) || !strings.Contains(refusal.Error(), "new final-review round") {
						t.Fatal(refusal)
					}
					after, err := os.ReadFile(name)
					if err != nil || !bytes.Equal(before, after) {
						t.Fatalf("placeholder changed: %v", err)
					}
					if _, err := os.Stat(filepath.Join(f.specWT, ".mindspec/adr/ADR-9800.md")); kind != "changed domains" && !os.IsNotExist(err) {
						t.Fatal("refusal wrote this run's placeholder")
					}
				})
			}
		}
	}
}

func TestS133Fix1_ReviewedApprovalTextRefuses(t *testing.T) {
	for _, row := range []struct{ name, policy, state string }{
		{"warn", "warn", "pass"},
		{"enforce", "enforce", "pass"},
		{"warn_no_panel", "warn", "absent"},
		{"warn_mixed", "warn", "mixed"},
		{"enforce_mixed", "enforce", "mixed"},
	} {
		t.Run(row.name, func(t *testing.T) {
			f := s133Document(t, row.policy, "spec_approve", "absent")
			const line = "Reviewed approval context must survive.\x1b[31m"
			f.writeSpec(f.read("spec.md") + "\n" + line + "\n")
			f.commit("approval context before review")
			if row.state == "absent" {
				if _, err := f.approve("spec_approve"); err != nil {
					t.Fatalf("no-panel approval must rewrite annotated Approval: %v", err)
				}
				text := f.read("spec.md")
				if strings.Contains(text, line) || !strings.Contains(text, "- **Status**: APPROVED") || !strings.Contains(text, "status: Approved") {
					t.Fatalf("no-panel Approval was not rewritten: %q", text)
				}
				if f.exec.commits != 2 || !f.epic || f.meta["panel_absent"] != true {
					t.Fatalf("no-panel approval did not finish: commits=%d epic=%v metadata=%v", f.exec.commits, f.epic, f.meta)
				}
				return
			}
			fixture := f.seed("spec_approve", "pass")
			if row.state == "mixed" {
				p := fixture.Panel
				paneltest.Lifecycle(t, filepath.Join(filepath.Dir(fixture.Dir), "abandoned"), s133DocID, "spec_approve", p.ReviewedHeadSHA, p.SubjectDigest, "abandoned")
			}
			before := f.read("spec.md")
			f.exec.failCommit = 2
			refusal := f.unchanged("spec_approve", func() error { _, err := f.approve("spec_approve"); return err })
			if !strings.Contains(refusal.Error(), "## Approval") || !strings.Contains(refusal.Error(), "Reviewed approval context must survive.") || strings.Contains(refusal.Error(), "\x1b") || !strings.Contains(refusal.Error(), "new spec-approve round") {
				t.Fatal(refusal)
			}
			if want := "panel " + termsafe.Escape(filepath.Base(fixture.Dir)) + ":"; !strings.Contains(refusal.Error(), want) || strings.Contains(refusal.Error(), "panel <slug>") {
				t.Fatalf("refusal must name the live panel %q: %v", want, refusal)
			}
			if before != f.read("spec.md") || f.exec.commits != 0 {
				t.Fatal("refusal changed spec or reached a commit")
			}
			f.exec.failCommit = 0
			if _, err := f.approve("spec_approve"); err == nil || !strings.Contains(err.Error(), "## Approval") {
				t.Fatalf("replay must refuse before writes: %v", err)
			}
		})
	}
}

func TestS133Fix1_ApprovalRewriteWithoutLivePanel(t *testing.T) {
	for _, state := range []string{"absent", "skip", "disabled", "abandoned"} {
		for _, section := range []string{"annotated", "hidden"} {
			t.Run(state+"/"+section, func(t *testing.T) {
				policy := "enforce"
				if state == "absent" {
					policy = "warn"
				}
				f := s133Document(t, policy, "spec_approve", "absent")
				text := f.read("spec.md")
				if section == "annotated" {
					text += "\nPending.\n"
				} else {
					text = strings.Replace(text, "## Approval", "<!--\n## Approval", 1)
				}
				f.writeSpec(text)
				if state == "disabled" {
					f.write(".mindspec/config.yaml", "enforcement:\n  panel_gate: false\n  panel_presence: enforce\n")
					config.ResetCache()
				}
				f.commit("approval text without live panel evidence")
				if state == "abandoned" {
					f.seed("spec_approve", "abandoned")
				}
				if state == "skip" {
					t.Setenv(panel.SkipPanelEnv, "1")
				}
				if _, err := f.approve("spec_approve"); err != nil {
					t.Fatalf("%s approval must retain the old rewrite behavior: %v", state, err)
				}
				want, err := upsertSpecFrontmatterApproval(text, "operator", time.Now().UTC())
				if err != nil {
					t.Fatal(err)
				}
				want = upsertSpecApprovalSection(want, "operator", time.Now().UTC())
				_, gotBody, _ := splitFrontmatter(f.read("spec.md"))
				_, wantBody, _ := splitFrontmatter(want)
				if gotBody != wantBody || f.exec.commits != 2 {
					t.Fatalf("rewrite changed: body=%q want=%q commits=%d", gotBody, wantBody, f.exec.commits)
				}
			})
		}
	}
}

func TestS133Fix1_ApprovalSectionFaultReplay(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, section := range []string{"draft", "approved"} {
			t.Run(fmt.Sprintf("%s/%s", policy, section), func(t *testing.T) {
				f := s133Document(t, policy, "spec_approve", "pass")
				if section == "approved" {
					if _, err := f.approve("spec_approve"); err != nil {
						t.Fatal(err)
					}
				}
				f.exec.commits = 0
				f.exec.failCommit = 2
				if _, err := f.approve("spec_approve"); err == nil || !strings.Contains(err.Error(), "injected commit fault") {
					t.Fatalf("fault did not fire: %v", err)
				}
				f.exec.failCommit = 0
				if _, err := f.approve("spec_approve"); err != nil {
					t.Fatal(err)
				}
				f.verifyDocument("spec_approve")
			})
		}
	}
}

func TestS133Fix1_AllPlaceholderRoots(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f := s133FinalFixtureNew(t, policy, 1, "pass")
			name, err := adr.CreateWithID(f.specWT, "ADR-9811", "Placeholder for ADR-9811", adr.CreateOpts{})
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			const committed = ".mindspec/adr/ADR-9811.md"
			s133Commit(t, f.specWT, "unreviewed committed placeholder")
			paths := []string{committed, ".mindspec/docs/adr/ADR-9812.md", "docs/adr/ADR-9813-extra.md"}
			s133Write(t, f.specWT, paths[1], strings.ReplaceAll(string(data), "ADR-9811", "ADR-9812"))
			s133Write(t, f.specWT, paths[2], strings.ReplaceAll(string(data), "ADR-9811", "ADR-9813-extra"))
			refusal := f.refuse("gate final_review", ImplOpts{SupersedeADR: "ADR-9800"})
			for _, path := range paths {
				if !strings.Contains(refusal.Error(), path) {
					t.Fatalf("missing path %s: %v", path, refusal)
				}
			}
		})
	}
}

func TestS133Fix1_PublishedPlaceholderChange(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f := s133FinalFixtureNew(t, policy, 1, "pass")
			t.Cleanup(executor.SetFinalizeStepHookForTest(func(stage string) error {
				if stage == "pre_cleanup" {
					return fmt.Errorf("fault before cleanup")
				}
				return nil
			}))
			if _, err := f.approve(); err == nil || !strings.Contains(err.Error(), "fault before cleanup") {
				t.Fatalf("fault did not fire: %v", err)
			}
			executor.SetFinalizeStepHookForTest(nil)
			name, err := adr.CreateWithID(f.specWT, "ADR-9811", "Placeholder for ADR-9811", adr.CreateOpts{})
			if err != nil {
				t.Fatal(err)
			}
			s133Commit(t, f.specWT, "placeholder after publication")
			if err := os.Rename(f.specWT, filepath.Join(t.TempDir(), "preserved-spec")); err != nil {
				t.Fatal(err)
			}
			refs := preflightGit(t, f.root, "show-ref")
			before, err := os.ReadFile(f.db)
			if err != nil {
				t.Fatal(err)
			}
			_, err = f.approve()
			if err == nil || !strings.Contains(err.Error(), "gate final_review") || !strings.Contains(err.Error(), filepath.Base(name)) {
				t.Fatalf("unreviewed placeholder passed published check: %v", err)
			}
			after, readErr := os.ReadFile(f.db)
			if readErr != nil || !bytes.Equal(before, after) || refs != preflightGit(t, f.root, "show-ref") {
				t.Fatal("published refusal changed state")
			}
		})
	}
}

func TestS133Fix1_DeletedReviewedPlaceholder(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, storage := range []string{"committed", "uncommitted"} {
			t.Run(policy+"/"+storage, func(t *testing.T) {
				f := s133FinalFixtureNew(t, policy, 1, "pass")
				name, err := adr.CreateWithID(f.specWT, "ADR-9811", "Placeholder for ADR-9811", adr.CreateOpts{})
				if err != nil {
					t.Fatal(err)
				}
				s133Commit(t, f.specWT, "placeholder before review")
				s133SeedPanel(t, f, "final_review", "spec/010-test", "", "pass", "final-new")
				s133Commit(t, f.specWT, "new review ledger")
				if err := os.Rename(name, filepath.Join(t.TempDir(), "preserved-placeholder")); err != nil {
					t.Fatal(err)
				}
				if storage == "committed" {
					s133Commit(t, f.specWT, "delete reviewed placeholder")
				}
				refusal := f.refuse("gate final_review", ImplOpts{})
				if !strings.Contains(refusal.Error(), ".mindspec/adr/ADR-9811.md") {
					t.Fatal(refusal)
				}
			})
		}
	}
}

func TestS133Fix1_SeededPlaceholderFaultReplay(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f := s133FinalFixtureNew(t, policy, 1, "pass", func(f *s133FinalFixture, _ []string) {
				name := ".mindspec/specs/010-test/plan.md"
				data, err := os.ReadFile(filepath.Join(f.specWT, name))
				if err != nil {
					t.Fatal(err)
				}
				s133Write(t, f.specWT, name, strings.Replace(string(data), "adr_citations:\n  - id: ADR-0195\n", "", 1))
				s133Commit(t, f.specWT, "uncovered workflow domain")
			})
			opts := ImplOpts{SupersedeADR: "ADR-9800"}
			implCreateWithIDFn = func(root, id, title string, createOpts adr.CreateOpts) (string, error) {
				if strings.Join(createOpts.Domains, ",") != "workflow" {
					t.Fatalf("wrong domain seed: %v", createOpts.Domains)
				}
				name, err := adr.CreateWithID(root, id, title, createOpts)
				if err != nil {
					t.Fatal(err)
				}
				return name, fmt.Errorf("fault after seeded placeholder")
			}
			if _, err := ApproveImpl(f.root, "010-test", f.exec, opts); err == nil || !strings.Contains(err.Error(), "fault after seeded placeholder") {
				t.Fatalf("fault did not fire: %v", err)
			}
			implCreateWithIDFn = adr.CreateWithID
			if _, err := ApproveImpl(f.root, "010-test", f.exec, opts); err != nil {
				t.Fatal(err)
			}
			f.assertConverged()
		})
	}
}

func TestS133Fix1_HiddenApprovalRewriteRefuses(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f := s133Document(t, policy, "spec_approve", "absent")
			body, section, found := strings.Cut(f.read("spec.md"), "## Approval")
			if !found {
				t.Fatal("fixture has no Approval section")
			}
			f.writeSpec(body + "<!--\n## Approval" + section)
			f.commit("approval heading in a comment")
			fixture := f.seed("spec_approve", "pass")
			refusal := f.unchanged("spec_approve", func() error { _, err := f.approve("spec_approve"); return err })
			if want := "panel " + termsafe.Escape(filepath.Base(fixture.Dir)) + ":"; !strings.Contains(refusal.Error(), want) || strings.Contains(refusal.Error(), "panel <slug>") {
				t.Fatalf("refusal must name the live panel %q: %v", want, refusal)
			}
			if !strings.Contains(refusal.Error(), "## Approval rewrite changes subject content") {
				t.Fatal(refusal)
			}
		})
	}
}

func TestS133Fix1_PlaceholderReplayKeepsCreationDate(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f := s133FinalFixtureNew(t, policy, 1, "pass")
			opts := ImplOpts{SupersedeADR: "ADR-9800"}
			var name string
			implCreateWithIDFn = func(root, id, title string, createOpts adr.CreateOpts) (string, error) {
				var err error
				name, err = adr.CreateWithID(root, id, title, createOpts)
				if err != nil {
					t.Fatal(err)
				}
				return name, fmt.Errorf("fault after placeholder")
			}
			if _, err := ApproveImpl(f.root, "010-test", f.exec, opts); err == nil || !strings.Contains(err.Error(), "fault after placeholder") {
				t.Fatalf("fault did not fire: %v", err)
			}
			data, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			// Model the bytes left by an invocation on an earlier day.
			old := strings.Replace(string(data), "- **Date**: "+time.Now().Format("2006-01-02"), "- **Date**: 2000-01-02", 1)
			if old == string(data) {
				t.Fatal("creation date was not changed")
			}
			s133Write(t, f.specWT, ".mindspec/adr/ADR-9800.md", old)
			implCreateWithIDFn = func(string, string, string, adr.CreateOpts) (string, error) {
				t.Fatal("replay tried to create the placeholder again")
				return "", nil
			}
			if _, err := ApproveImpl(f.root, "010-test", f.exec, opts); err != nil {
				t.Fatal(err)
			}
			f.assertConverged()
			landed := preflightGit(t, f.root, "show", "main:.mindspec/adr/ADR-9800.md")
			if landed != strings.TrimSpace(old) {
				t.Fatal("replay changed the placeholder bytes")
			}
		})
	}
}
