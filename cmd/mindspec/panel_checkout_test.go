package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bootstrap"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

func slotTestGitInput(t *testing.T, dir, input string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "core.fsmonitor=false", "-c", "core.hooksPath=/dev/null", "-c", "user.name=Slot Test", "-c", "user.email=slot@example.invalid", "-C", dir}, args...)...)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/usr/bin/false")
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture repository operation %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func slotTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return slotTestGitInput(t, dir, "", args...)
}
func slotWrite(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}
func slotRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func slotSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			out[rel] = "directory"
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			out[rel] = "symlink:" + target
			return err
		}
		raw, err := os.ReadFile(path)
		out[rel] = string(raw)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

type slotFixture struct {
	root, dir, isolation string
	p                    panel.Panel
}

func newCheckoutFixture(t *testing.T, isolation string) slotFixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	slotTestGit(t, root, "init", "-q", "-b", "main")
	ignore := isolation + "\n"
	input := fmt.Sprintf("commit refs/heads/main\ncommitter Slot Test <slot@example.invalid> 1760000000 +0000\ndata 7\ninitial\nM 100644 inline README.md\ndata 6\nhello\nM 100644 inline .gitignore\ndata %d\n%s\n\ndone\n", len(ignore), ignore)
	slotTestGitInput(t, root, input, "fast-import", "--quiet")
	slotTestGit(t, root, "checkout", "-f", "main")
	slotWrite(t, filepath.Join(root, ".mindspec", "config.yaml"), "panel: {isolation_root: "+isolation+"}\n")
	config.ResetCache()
	t.Cleanup(config.ResetCache)
	p := panel.Panel{Spec: "adhoc", Target: "main", Round: 1, ExpectedReviewers: 4, ReviewedHeadSHA: slotTestGit(t, root, "rev-parse", "HEAD"), SubjectDigest: "sha256:fixture", Gate: "adhoc", Telemetry: "required"}
	for i := 1; i <= 4; i++ {
		p.Roster = append(p.Roster, panel.RosterSlot{Slot: fmt.Sprintf("R%d", i), Family: "harness", Model: "m", Lens: "adversarial"})
	}
	dir := filepath.Join(root, "review", "p")
	if err := panel.WriteSnapshot(dir, p); err != nil {
		t.Fatal(err)
	}
	paneltest.Fixture{Dir: dir, Panel: p}.Round(t, 2)
	withTestChdir(t, root)
	stubWorktreeListEmpty(t)
	return slotFixture{root, dir, isolation, p}
}
func (f slotFixture) layout(t *testing.T, slot string) panel.SlotLayout {
	t.Helper()
	l, err := panel.NewSlotLayout(f.root, f.isolation, "", "p", f.p.Round, slot)
	if err != nil {
		t.Fatal(err)
	}
	return l
}
func checkoutRun(t *testing.T, args ...string) (string, error) {
	t.Helper()
	for _, name := range []string{"spec", "slot", "all", "dispatch", "replace", "fixer", "prune"} {
		flag := panelCheckoutCmd.Flags().Lookup(name)
		if err := flag.Value.Set(flag.DefValue); err != nil {
			t.Fatal(err)
		}
		flag.Changed = false
	}
	return runPanelCmd(append([]string{"checkout", "p"}, args...)...)
}
func checkoutOK(t *testing.T, args ...string) string {
	t.Helper()
	out, err := checkoutRun(t, args...)
	if err != nil {
		t.Fatalf("checkout %v: %v\n%s", args, err, out)
	}
	return out
}
func checkoutRefused(t *testing.T, want string, args ...string) {
	t.Helper()
	out, err := checkoutRun(t, args...)
	if err == nil || !strings.Contains(err.Error()+out, want) {
		t.Fatalf("checkout %v: wanted refusal %q, got %v\n%s", args, want, err, out)
	}
}
func seedSlotFinal(t *testing.T, f slotFixture, verdict string) {
	t.Helper()
	v := paneltest.Verdict(verdict)
	v.ReviewedSHA = f.p.ReviewedHeadSHA
	_, err := panel.Submit(panel.SubmitTarget{Dir: f.dir, Panel: "p", Slot: "R1", Round: 1}, paneltest.Bytes(t, v), paneltest.Resolve)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPanelCheckout_Creates(t *testing.T) {
	f := newCheckoutFixture(t, ".mindspec-scratch")
	out := checkoutOK(t, "--all")
	for _, s := range f.p.Roster {
		l := f.layout(t, s.Slot)
		if !strings.Contains(out, l.Dir+"\n") {
			t.Fatalf("missing absolute slot path: %s", out)
		}
		var metadata panel.SlotMetadata
		if err := json.Unmarshal(slotRead(t, l.Metadata), &metadata); err != nil {
			t.Fatal(err)
		}
		if metadata.MainRoot != f.root || metadata.Spec != "adhoc" || metadata.Slug != "p" || metadata.Round != 1 || metadata.Slot != s.Slot {
			t.Fatalf("metadata: %+v", metadata)
		}
		if got := slotTestGit(t, l.Tree, "rev-parse", "HEAD"); got != f.p.ReviewedHeadSHA {
			t.Fatalf("HEAD = %s", got)
		}
		if got := slotTestGit(t, l.Tree, "rev-parse", "--abbrev-ref", "HEAD"); got != "HEAD" {
			t.Fatalf("checkout is not detached: %s", got)
		}
	}
	before := slotSnapshot(t, filepath.Join(f.root, f.isolation))
	checkoutOK(t, "--all")
	if after := slotSnapshot(t, filepath.Join(f.root, f.isolation)); !reflect.DeepEqual(before, after) {
		t.Fatal("idempotent checkout changed slot files")
	}
	checkoutOK(t, "--fixer")
	fixer := f.layout(t, "fixer")
	if info, err := os.Stat(fixer.Dir); err != nil || !info.IsDir() {
		t.Fatalf("fixer: %v", err)
	}
	if _, err := os.Lstat(fixer.Tree); !os.IsNotExist(err) {
		t.Fatalf("fixer has a tree: %v", err)
	}
	t.Run("init custom root then checkout", func(t *testing.T) {
		f := newCheckoutFixture(t, "review-scratch")
		slotWrite(t, filepath.Join(f.root, ".gitignore"), "# existing install\n")
		if _, err := bootstrap.Run(f.root, false); err != nil {
			t.Fatal(err)
		}
		checkoutOK(t, "--slot", "R1")
		if got := slotTestGit(t, f.layout(t, "R1").Tree, "rev-parse", "HEAD"); got != f.p.ReviewedHeadSHA {
			t.Fatalf("custom-root HEAD: %s", got)
		}
	})
}

func TestPanelCheckout_NoOrigin(t *testing.T) {
	f := newCheckoutFixture(t, ".mindspec-scratch")
	checkoutOK(t, "--all")
	for _, s := range f.p.Roster {
		if got := slotTestGit(t, f.layout(t, s.Slot).Tree, "remote"); got != "" {
			t.Fatalf("remote remains: %s", got)
		}
	}
}

func TestPanelCheckout_GitStaysInSlot(t *testing.T) {
	f := newCheckoutFixture(t, ".mindspec-scratch")
	checkoutOK(t, "--slot", "R1")
	tree := f.layout(t, "R1").Tree
	slotWrite(t, filepath.Join(tree, "README.md"), "slot change\n")
	before := slotSnapshot(t, filepath.Join(f.root, ".git"))
	if got := slotTestGit(t, tree, "status", "--porcelain"); !strings.Contains(got, "README.md") {
		t.Fatalf("no slot change: %s", got)
	}
	slotTestGit(t, tree, "stash", "push", "-m", "slot-only")
	if after := slotSnapshot(t, filepath.Join(f.root, ".git")); !reflect.DeepEqual(before, after) {
		t.Fatal("slot status/stash changed main git directory")
	}
	if got := slotTestGit(t, tree, "rev-parse", "--git-dir"); got != ".git" {
		t.Fatalf("external git dir: %s", got)
	}
}

func TestPanelCheckout_Refusals(t *testing.T) {
	t.Run("off roster", func(t *testing.T) {
		newCheckoutFixture(t, ".mindspec-scratch")
		checkoutRefused(t, "roster", "--slot", "R9")
	})
	t.Run("other SHA", func(t *testing.T) {
		f := newCheckoutFixture(t, ".mindspec-scratch")
		checkoutOK(t, "--slot", "R1")
		tree := f.layout(t, "R1").Tree
		sha := slotTestGitInput(t, tree, "another\n", "commit-tree", "HEAD^{tree}", "-p", "HEAD")
		slotTestGit(t, tree, "checkout", "--detach", sha)
		checkoutRefused(t, "--prune --slot", "--slot", "R1")
	})
	t.Run("not ignored", func(t *testing.T) {
		f := newCheckoutFixture(t, ".mindspec-scratch")
		slotWrite(t, filepath.Join(f.root, ".gitignore"), "")
		checkoutRefused(t, "mindspec init", "--slot", "R1")
	})
	t.Run("symlink root component", func(t *testing.T) {
		f := newCheckoutFixture(t, "outer/scratch")
		if err := os.Symlink(t.TempDir(), filepath.Join(f.root, "outer")); err != nil {
			t.Fatal(err)
		}
		checkoutRefused(t, "mindspec init", "--slot", "R1")
	})
	t.Run("abandoned", func(t *testing.T) {
		f := newCheckoutFixture(t, ".mindspec-scratch")
		f.p.Abandoned, f.p.AbandonReason = true, "operator ended review"
		if err := panel.WriteSnapshot(f.dir, f.p); err != nil {
			t.Fatal(err)
		}
		checkoutRefused(t, "abandoned", "--all")
	})
	t.Run("held lock", func(t *testing.T) {
		f := newCheckoutFixture(t, ".mindspec-scratch")
		l := f.layout(t, "R1")
		lock, err := panel.AcquireSlotLock(l.Lock)
		if err != nil {
			t.Fatal(err)
		}
		defer lock.Release()
		checkoutRefused(t, "R1.lock", "--slot", "R1")
	})
}

func TestPanelCheckout_GitRedirectRefused(t *testing.T) {
	for _, redirect := range []string{"commondir", "core.worktree", "core.bare", "alternates"} {
		t.Run(redirect, func(t *testing.T) {
			f := newCheckoutFixture(t, ".mindspec-scratch")
			checkoutOK(t, "--slot", "R1")
			tree := f.layout(t, "R1").Tree
			switch redirect {
			case "commondir":
				slotWrite(t, filepath.Join(tree, ".git", "commondir"), filepath.Join(f.root, ".git")+"\n")
			case "core.worktree":
				slotTestGit(t, tree, "config", "--local", redirect, f.root)
			case "core.bare":
				slotTestGit(t, tree, "config", "--local", redirect, "false")
			case "alternates":
				path := filepath.Join(tree, ".git", "objects", "info", "alternates")
				slotWrite(t, path, string(slotRead(t, path))+filepath.Join(f.root, ".git", "objects")+"\n")
			}
			before := slotSnapshot(t, filepath.Join(f.root, ".git"))
			checkoutRefused(t, "--prune --slot", "--slot", "R1")
			if after := slotSnapshot(t, filepath.Join(f.root, ".git")); !reflect.DeepEqual(before, after) {
				t.Fatal("redirect refusal changed main repository")
			}
		})
	}
}

func TestPanelCheckout_TreeLstat(t *testing.T) {
	for _, kind := range []string{"tree symlink", "tree file", "gitdir file", "gitdir symlink"} {
		t.Run(kind, func(t *testing.T) {
			f := newCheckoutFixture(t, ".mindspec-scratch")
			checkoutOK(t, "--slot", "R1")
			l := f.layout(t, "R1")
			path := l.Tree
			if strings.HasPrefix(kind, "gitdir") {
				path = filepath.Join(l.Tree, ".git")
			}
			if err := os.Rename(path, path+"-quarantine"); err != nil {
				t.Fatal(err)
			}
			if strings.HasSuffix(kind, "symlink") {
				if err := os.Symlink(path+"-quarantine", path); err != nil {
					t.Fatal(err)
				}
			} else {
				slotWrite(t, path, "gitdir: "+filepath.Join(f.root, ".git")+"\n")
			}
			checkoutRefused(t, "--prune --slot", "--slot", "R1")
		})
	}
}

func TestPanelCheckout_NoFsmonitor(t *testing.T) {
	f := newCheckoutFixture(t, ".mindspec-scratch")
	checkoutOK(t, "--slot", "R1")
	tree := f.layout(t, "R1").Tree
	marker := filepath.Join(f.root, "monitor-ran")
	script := filepath.Join(f.root, "monitor")
	slotWrite(t, script, "#!/bin/sh\nprintf invoked > '"+marker+"'\n")
	if err := os.Chmod(script, 0o700); err != nil {
		t.Fatal(err)
	}
	slotTestGit(t, tree, "config", "core.fsmonitor", script)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	shimDir := t.TempDir()
	shim := filepath.Join(shimDir, "git")
	slotWrite(t, shim, "#!/bin/sh\ncase \" $* \" in\n  *' -c core.fsmonitor=false -c core.hooksPath=/dev/null '*) ;;\n  *) echo 'missing slot git overrides' >&2; exit 91 ;;\nesac\n[ \"$GIT_CONFIG_NOSYSTEM\" = 1 ] || exit 92\nexec '"+realGit+"' \"$@\"\n")
	if err := os.Chmod(shim, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	checkoutOK(t, "--slot", "R1")
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("fsmonitor ran: %v", err)
	}
	// Positive control: Git invokes this same hook without the binary's override.
	cmd := exec.Command(realGit, "-c", "core.hooksPath=/dev/null", "-C", tree, "status", "--porcelain")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/usr/bin/false"}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("positive fsmonitor control: %v %s", err, out)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("positive control did not run fsmonitor: %v", err)
	}
}

func TestPanelCheckout_GitEnvScrubbed(t *testing.T) {
	f := newCheckoutFixture(t, ".mindspec-scratch")
	t.Setenv("GIT_DIR", filepath.Join(f.root, ".git"))
	t.Setenv("GIT_WORK_TREE", f.root)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(f.root, ".git", "index"))
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.worktree")
	t.Setenv("GIT_CONFIG_VALUE_0", f.root)
	before := slotSnapshot(t, filepath.Join(f.root, ".git"))
	checkoutOK(t, "--slot", "R1")
	checkoutOK(t, "--slot", "R1")
	if after := slotSnapshot(t, filepath.Join(f.root, ".git")); !reflect.DeepEqual(before, after) {
		t.Fatal("inherited git variables redirected a slot operation")
	}
	tree := f.layout(t, "R1").Tree
	slotWrite(t, filepath.Join(tree, "README.md"), "slot only\n")
	slotTestGit(t, tree, "stash", "push", "-m", "isolated")
	if after := slotSnapshot(t, filepath.Join(f.root, ".git")); !reflect.DeepEqual(before, after) {
		t.Fatal("stash changed main refs")
	}
}

func TestPanelCheckout_DispatchFlag(t *testing.T) {
	newCheckoutFixture(t, ".mindspec-scratch")
	for _, args := range [][]string{{"--dispatch"}, {"--all", "--dispatch"}, {"--slot", "R1", "--replace"}, {"--prune", "--slot", "R1", "--dispatch"}, {"--fixer", "--slot", "R1"}} {
		checkoutRefused(t, "--dispatch needs --slot", args...)
	}
}

func TestPanelCheckout_HarnessDispatched(t *testing.T) {
	f := newCheckoutFixture(t, ".mindspec-scratch")
	checkoutOK(t, "--slot", "R1", "--dispatch")
	events, err := panel.SlotAttempts(f.dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(events["R1"]) != 1 || events["R1"][0].Status != "dispatched" || events["R1"][0].Attempt != 1 {
		t.Fatalf("first dispatch: %+v", events)
	}
	if info, err := os.Stat(f.layout(t, "R1").AttemptDir(1)); err != nil || !info.IsDir() {
		t.Fatalf("attempt: %v", err)
	}
	if err := os.Mkdir(f.layout(t, "R1").AttemptDir(3), 0o700); err != nil {
		t.Fatal(err)
	}
	checkoutOK(t, "--slot", "R1", "--dispatch")
	events, err = panel.SlotAttempts(f.dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(events["R1"]) != 3 || events["R1"][1].Status != "launch_failed" || events["R1"][1].Attempt != 1 || events["R1"][2].Status != "dispatched" || events["R1"][2].Attempt != 4 {
		t.Fatalf("redispatch: %+v", events)
	}
	seedSlotFinal(t, f, "APPROVE")
	before := slotRead(t, filepath.Join(f.dir, "dispositions.jsonl"))
	checkoutRefused(t, "--replace", "--slot", "R1", "--dispatch")
	if !bytes.Equal(before, slotRead(t, filepath.Join(f.dir, "dispositions.jsonl"))) {
		t.Fatal("refused dispatch appended")
	}
	checkoutOK(t, "--slot", "R1", "--dispatch", "--replace")
	t.Run("process", func(t *testing.T) {
		f := newCheckoutFixture(t, ".mindspec-scratch")
		slotWrite(t, filepath.Join(f.root, ".mindspec", "config.yaml"), "panel:\n  runners:\n    - {family: harness, kind: process, argv: [fake-reviewer], prompt: stdin, output: stdout}\n")
		config.ResetCache()
		before := slotSnapshot(t, f.dir)
		checkoutRefused(t, "panel launch", "--slot", "R1", "--dispatch")
		if after := slotSnapshot(t, f.dir); !reflect.DeepEqual(before, after) {
			t.Fatal("process refusal appended")
		}
	})
	t.Run("landed", func(t *testing.T) {
		f := newCheckoutFixture(t, ".mindspec-scratch")
		f.p.BeadID = ptrStr("unit-work")
		f.p.Gate = "bead"
		f.p.Target = "bead/unit-work"
		f.p.Round = 2
		if err := panel.WriteSnapshot(f.dir, f.p); err != nil {
			t.Fatal(err)
		}
		paneltest.Fixture{Dir: f.dir, Panel: f.p}.Round(t, 2)
		old := checkoutReadMetadataFn
		checkoutReadMetadataFn = func(string) (map[string]interface{}, error) {
			return map[string]interface{}{"mindspec_landed_review_panel": "p@r2"}, nil
		}
		t.Cleanup(func() { checkoutReadMetadataFn = old })
		before := slotSnapshot(t, f.dir)
		checkoutRefused(t, "already landed", "--slot", "R1", "--dispatch", "--replace")
		if after := slotSnapshot(t, f.dir); !reflect.DeepEqual(before, after) {
			t.Fatal("landed refusal appended")
		}
	})
}

func TestPanelCheckout_PlainWritesNoLedger(t *testing.T) {
	f := newCheckoutFixture(t, ".mindspec-scratch")
	seedSlotFinal(t, f, "APPROVE")
	before := slotRead(t, filepath.Join(f.dir, "dispositions.jsonl"))
	checkoutOK(t, "--slot", "R1")
	checkoutOK(t, "--all")
	if !bytes.Equal(before, slotRead(t, filepath.Join(f.dir, "dispositions.jsonl"))) {
		t.Fatal("plain checkout changed ledger")
	}
	latest, err := panel.LatestSubmissions(f.dir, 1)
	if err != nil || latest["R1"].Status != "final" {
		t.Fatalf("latest submission: %+v %v", latest, err)
	}
}

func TestPanelCheckout_Prune(t *testing.T) {
	t.Run("one slot", func(t *testing.T) {
		f := newCheckoutFixture(t, ".mindspec-scratch")
		checkoutOK(t, "--all")
		keep := slotSnapshot(t, f.layout(t, "R1").Dir)
		checkoutOK(t, "--prune", "--slot", "R2")
		if _, err := os.Stat(f.layout(t, "R2").Dir); !os.IsNotExist(err) {
			t.Fatalf("R2 remains: %v", err)
		}
		if !reflect.DeepEqual(keep, slotSnapshot(t, f.layout(t, "R1").Dir)) {
			t.Fatal("prune changed R1")
		}
	})
	t.Run("round bump", func(t *testing.T) {
		f := newCheckoutFixture(t, ".mindspec-scratch")
		checkoutOK(t, "--all")
		slotWrite(t, filepath.Join(f.layout(t, "R1").Tree, "README.md"), "dirty\n")
		resetPanelCreateFlags(t)
		if out, err := runPanelCmd("create", "p", "--gate", "adhoc", "--target", "main", "--round", "2"); err != nil {
			t.Fatalf("round bump: %v\n%s", err, out)
		}
		for _, s := range f.p.Roster {
			if _, err := os.Stat(f.layout(t, s.Slot).Dir); !os.IsNotExist(err) {
				t.Fatalf("prior slot remains: %v", err)
			}
		}
	})
	t.Run("abandon", func(t *testing.T) {
		f := newCheckoutFixture(t, ".mindspec-scratch")
		checkoutOK(t, "--all")
		checkoutOK(t, "--fixer")
		if out, err := runPanelCmd("abandon", "p", "--reason", "review ended", "--by", "operator"); err != nil {
			t.Fatalf("abandon: %v\n%s", err, out)
		}
		for _, s := range []string{"R1", "R2", "R3", "R4", "fixer"} {
			if _, err := os.Stat(f.layout(t, s).Dir); !os.IsNotExist(err) {
				t.Fatalf("abandoned slot remains: %v", err)
			}
		}
	})
	t.Run("symlink leaf", func(t *testing.T) {
		f := newCheckoutFixture(t, ".mindspec-scratch")
		checkoutOK(t, "--slot", "R1")
		l := f.layout(t, "R1")
		target := t.TempDir()
		slotWrite(t, filepath.Join(target, "keep"), "safe")
		if err := os.Rename(l.Dir, l.Dir+"-quarantine"); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, l.Dir); err != nil {
			t.Fatal(err)
		}
		checkoutOK(t, "--prune", "--slot", "R1")
		if string(slotRead(t, filepath.Join(target, "keep"))) != "safe" {
			t.Fatal("prune followed symlink")
		}
	})
}

func TestPanelCleanup_HeldLockNoChange(t *testing.T) {
	for _, verb := range []string{"prune", "create", "abandon"} {
		t.Run(verb, func(t *testing.T) {
			f := newCheckoutFixture(t, ".mindspec-scratch")
			checkoutOK(t, "--all")
			lock, err := panel.AcquireSlotLock(f.layout(t, "R2").Lock)
			if err != nil {
				t.Fatal(err)
			}
			defer lock.Release()
			beforePanel := slotSnapshot(t, f.dir)
			beforeScratch := slotSnapshot(t, filepath.Join(f.root, f.isolation))
			var out string
			switch verb {
			case "prune":
				out, err = checkoutRun(t, "--prune")
			case "create":
				resetPanelCreateFlags(t)
				out, err = runPanelCmd("create", "p", "--gate", "adhoc", "--target", "main", "--round", "2")
			case "abandon":
				out, err = runPanelCmd("abandon", "p", "--reason", "review ended", "--by", "operator")
			}
			if err == nil || !strings.Contains(err.Error()+out, "R2") {
				t.Fatalf("held-slot refusal: %v %s", err, out)
			}
			if !reflect.DeepEqual(beforePanel, slotSnapshot(t, f.dir)) || !reflect.DeepEqual(beforeScratch, slotSnapshot(t, filepath.Join(f.root, f.isolation))) {
				t.Fatal("held-lock refusal changed panel, ledger or scratch files")
			}
		})
	}
}

func TestPanelCheckout_PruneKeepsLockFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("persistent lock inodes are a POSIX contract")
	}
	f := newCheckoutFixture(t, ".mindspec-scratch")
	checkoutOK(t, "--slot", "R1")
	l := f.layout(t, "R1")
	before, err := os.Stat(l.Lock)
	if err != nil {
		t.Fatal(err)
	}
	paused, err := os.OpenFile(l.Lock, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer paused.Close()
	checkoutOK(t, "--prune", "--slot", "R1")
	after, err := os.Stat(l.Lock)
	if err != nil {
		t.Fatalf("lock was deleted: %v", err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("prune replaced lock inode")
	}
	first, err := panel.TrySlotLock(l.Lock)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	if second, err := panel.TrySlotLock(l.Lock); err == nil {
		second.Release()
		t.Fatal("two holders acquired the retained lock")
	}
}

func TestWalks_SkipIsolationRoot(t *testing.T) {
	t.Run("registration", func(t *testing.T) {
		f := newCheckoutFixture(t, "review")
		// The main registration is under the configured isolation root here.
		// Scan must see the positive control before the exclusion is applied.
		if got := panel.Scan(f.root); len(got) != 1 {
			t.Fatalf("positive registration control: %+v", got)
		}
		if got := panelSlugMatches(f.root, "p"); len(got) != 0 {
			t.Fatalf("isolation registration leaked: %+v", got)
		}
	})
	t.Run("migrate", func(t *testing.T) {
		root := t.TempDir()
		slotWrite(t, filepath.Join(root, ".mindspec", "config.yaml"), "panel: {isolation_root: project-docs/scratch}\n")
		slotWrite(t, filepath.Join(root, "live.md"), "live")
		slotWrite(t, filepath.Join(root, "project-docs", "scratch", "slot.md"), "scratch")
		config.ResetCache()
		t.Cleanup(config.ResetCache)
		files, err := scanSourceMarkdown(root)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(files, []string{"live.md"}) {
			t.Fatalf("migrate isolation leak: %v", files)
		}
		canonical, err := scanCanonicalDocs(root)
		if err != nil || len(canonical) != 0 {
			t.Fatalf("canonical scan: %v %v", canonical, err)
		}
	})
}
