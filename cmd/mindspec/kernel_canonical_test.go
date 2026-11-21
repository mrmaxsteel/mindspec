package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

func TestS134Final_CanonicalLayout(t *testing.T) {
	bin := kernelE2EBinary(t)
	f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
	t.Setenv("MINDSPEC_TEST_FULL_BD", "1")
	f.Fake.Records = map[string]kernel.Record{}
	f.Must(os.MkdirAll(filepath.Join(f.Root, ".beads"), 0755))
	f.Must(os.WriteFile(filepath.Join(f.Root, ".beads/config.yaml"), []byte("types.custom: gate,step\n"), 0600))
	f.Must(os.Rename(filepath.Join(f.Root, ".mindspec", "specs"), filepath.Join(t.TempDir(), "flat-specs")))
	actorCmdStore(t, f)
	e := newKernelE2EWithLayout(t, bin, f, ".mindspec/docs", 1)
	layout, err := workspace.DetectLayout(e.cwd)
	e.must(err)
	if layout != workspace.LayoutCanonical {
		t.Fatalf("fixture layout=%s", layout)
	}
	e.write(e.cwd, ".mindspec/docs/domains/workflow/OWNERSHIP.yaml", "paths:\n  - internal/widget/**\n")
	e.write(e.cwd, ".mindspec/docs/domains/workflow/architecture.md", "Workflow documentation.\n")
	e.write(f.Root, ".git/info/exclude", ".mindspec-scratch\n")
	e.git(e.cwd, "add", ".")
	e.git(e.cwd, "commit", "-qm", "context")
	item := f.Items[0]
	tree := filepath.Join(f.Root, ".worktrees", "worktree-"+item)
	e.git(f.Root, "worktree", "add", "-b", "bead/"+item, tree, "spec/"+f.SpecID)
	e.apply()
	e.create(e.implement(0))
	review := func(gate string) {
		t.Helper()
		reg, ok, err := kernel.Decode[kernel.RegistrationRecord](e.row(gate).Metadata, kernel.KeyRegistration)
		e.must(err)
		dir := filepath.ToSlash(filepath.Join("review", reg.Slug))
		if !ok || !reflect.DeepEqual(reg.Directories, []string{dir}) {
			t.Fatalf("registration=%+v", reg)
		}
		if _, err := os.Stat(filepath.Join(e.cwd, dir, "panel.json")); err != nil {
			t.Fatalf("panel outside spec worktree: %v", err)
		}
		if _, err := os.Stat(filepath.Join(f.Root, dir)); !os.IsNotExist(err) {
			t.Fatalf("panel created in main checkout: %v", err)
		}
		e.write(e.cwd, filepath.Join(dir, "BRIEF.md"), "Review the implementation against the approved spec.\n")
		out := e.apply()
		count := 0
		for _, a := range out.Actions {
			if a.Kind != "review-slot" {
				continue
			}
			if len(a.Brief) < 2 || a.Brief[0] != "mindspec" {
				t.Fatalf("invalid review brief argv: %v", a.Brief)
			}
			for _, cwd := range []string{e.cwd, f.Root} {
				original := e.cwd
				e.cwd = cwd
				prompt := e.ok(a.Brief[1:]...)
				if !strings.Contains(prompt, "Review the implementation against the approved spec.") {
					t.Fatalf("review brief missing from prompt: %s", prompt)
				}
				e.cwd = original
			}
			slot := fmt.Sprintf("R%d", count+1)
			e.ok("panel", "checkout", reg.Slug, "--spec", f.SpecID, "--slot", slot, "--dispatch")
			token := e.lease(a.Step)
			tuple, _, err := e.row(a.Step).Tuple()
			e.must(err)
			word := "APPROVE"
			if count == 0 {
				word = "REQUEST_CHANGES"
			}
			v := paneltest.Verdict(word)
			v.Slot = fmt.Sprintf("R%d", tuple.Slot)
			v.Round = a.Round
			v.ReviewedSHA = a.Subject.Commit
			v.Model = "test-model"
			path := filepath.Join(dir, "input-"+v.Slot+".json")
			holding := e.commit(e.cwd, path, string(paneltest.Bytes(t, v)))
			e.ok("submit", a.Step, "--lease", token, "--artifact", filepath.Join(e.cwd, path))
			receipt, ok, err := kernel.Decode[kernel.Receipt](e.row(a.Step).Metadata, kernel.KeyClose)
			e.must(err)
			if !ok || receipt.Commit != holding || receipt.Path != filepath.ToSlash(path) || kernel.ValidReceipt(f.Root, receipt) != nil {
				t.Fatalf("holding commit not retained: %+v", receipt)
			}
			if count == 0 {
				e.ok("panel", "refute", reg.Slug, "--spec", f.SpecID, "--slot", v.Slot, "--round", fmt.Sprint(a.Round), "--by", "operator", "--ground", "disproven", "--reason", "The implementation covers the finding.", "--evidence", a.Subject.Commit)
			}
			count++
		}
		for _, cwd := range []string{e.cwd, f.Root} {
			original := e.cwd
			e.cwd = cwd
			e.ok("panel", "verify", reg.Slug, "--spec", f.SpecID)
			e.ok("panel", "tally", reg.Slug, "--spec", f.SpecID)
			e.cwd = original
		}
		if count != 3 {
			t.Fatalf("reviewed %d slots", count)
		}
	}
	review(item + ".review")
	e.ok("complete", item)
	e.create(e.apply())
	review(f.Epic + ".final")
	e.ok("impl", "approve", f.SpecID)
	if out := e.apply(); out.Status != "done" {
		t.Fatalf("final status=%s", out.Status)
	}
}
