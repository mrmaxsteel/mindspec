package main

import (
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func TestS134PanelCreate_AdhocCheckoutCopy(t *testing.T) {
	bin := kernelE2EBinary(t)
	for _, layout := range []string{"legacy", "flat"} {
		t.Run(layout, func(t *testing.T) {
			f := kerneltest.EmptyRepo(t)
			e := &kernelE2E{t: t, f: f, bin: bin, cwd: f.Root}
			e.git(f.Root, "config", "user.name", "Panel Test")
			e.git(f.Root, "config", "user.email", "panel@example.invalid")
			e.git(f.Root, "config", "core.hooksPath", "/dev/null")
			e.git(f.Root, "branch", "-M", "main")
			e.write(f.Root, ".mindspec/.keep", "")
			e.write(f.Root, ".gitignore", ".worktrees/\n")
			if layout == "flat" {
				e.write(f.Root, ".mindspec/specs/.keep", "")
				e.write(f.Root, ".mindspec/adr/.keep", "")
			}
			e.git(f.Root, "add", ".")
			e.git(f.Root, "commit", "-qm", "fixture")
			e.ok("panel", "create", "demo", "--gate", "adhoc", "--target", "main")
			e.git(f.Root, "add", ".")
			e.git(f.Root, "commit", "-qm", "record panel")
			wt := filepath.Join(f.Root, ".worktrees", "worktree-spec-001-x")
			e.git(f.Root, "worktree", "add", "-b", "spec/001-x", wt)
			e.ok("panel", "create", "demo", "--gate", "adhoc", "--round", "2", "--target", "main")
			copyDir := filepath.Join(wt, "review", "demo")
			if layout == "flat" {
				copyDir = filepath.Join(wt, ".mindspec", "reviews", "demo")
			}
			f.Must(os.WriteFile(filepath.Join(copyDir, panel.FileName), []byte("{bad"), 0644))
			out, code := e.run("panel", "create", "demo", "--gate", "adhoc", "--round", "3", "--target", "main")
			if code == 0 || !strings.Contains(out, "damaged") || !strings.Contains(out, copyDir) || !strings.Contains(out, "repair panel.json") || strings.Contains(out, "already registered") {
				t.Fatalf("damaged copy recovery: %d %s", code, out)
			}
		})
	}
}

func TestS134PanelLookup_WorktreeIsolation(t *testing.T) {
	root := setupCreateIntegrity(t)
	tree := filepath.Join(root, ".worktrees", "worktree-spec-901-other")
	dir := writePanelFixture(t, filepath.Join(root, ".mindspec-scratch"), "custom", panel.Panel{Spec: "901-other", Gate: "final_review", Round: 1})
	parent := filepath.Join(tree, "review")
	if err := os.MkdirAll(parent, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dir, filepath.Join(parent, "custom")); err != nil {
		t.Fatal(err)
	}
	if got := panel.Scan(tree); len(got) != 1 {
		t.Fatalf("missing positive control: %v", got)
	}
	if got := panelSlugMatches(root, "custom"); len(got) != 0 {
		t.Fatalf("isolation registration leaked: %v", got)
	}
}
