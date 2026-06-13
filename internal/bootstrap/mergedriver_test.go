package bootstrap

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestEmbeddedWrapperMatchesTrackedScript is the drift guard: the embedded
// bytes must stay byte-equal to the tracked scripts/bd-jsonl-merge-driver.sh
// so the two copies never diverge (plan Bead 1, step 1).
func TestEmbeddedWrapperMatchesTrackedScript(t *testing.T) {
	tracked, err := os.ReadFile(filepath.Join("..", "..", "scripts", "bd-jsonl-merge-driver.sh"))
	if err != nil {
		t.Fatalf("reading tracked wrapper: %v", err)
	}
	if !bytes.Equal(tracked, beadsMergeDriverWrapper) {
		t.Errorf("embedded wrapper drifted from scripts/bd-jsonl-merge-driver.sh "+
			"(embedded %d bytes, tracked %d bytes) — re-copy the tracked file into "+
			"internal/bootstrap/assets/", len(beadsMergeDriverWrapper), len(tracked))
	}
}

// TestProvision_WrapperWrittenExecutable is the EXEC-BIT proof: the wrapper
// bootstrap writes MUST be executable, else resolveDriverCommand's 0o111 gate
// fails and git silently text-merges the jsonl. RED if written 0644.
func TestProvision_WrapperWrittenExecutable(t *testing.T) {
	root := t.TempDir()
	if _, err := Run(root, false); err != nil {
		t.Fatalf("Run: %v", err)
	}

	scriptPath := filepath.Join(root, "scripts", "bd-jsonl-merge-driver.sh")
	info, err := os.Stat(scriptPath)
	if err != nil {
		t.Fatalf("wrapper not written: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("wrapper is not executable: mode %v (the merge driver is a no-op without the exec bit)", info.Mode().Perm())
	}

	// Content must be the embedded wrapper, byte-for-byte.
	got, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, beadsMergeDriverWrapper) {
		t.Error("written wrapper does not match the embedded bytes")
	}
}

// TestProvision_GitattributesAndConfig pins the provisioning units: a fresh
// bootstrapped git repo gets the merge=beads mapping AND the portable
// relative merge.beads.driver value (no absolute path baked in).
func TestProvision_GitattributesAndConfig(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)

	if _, err := Run(root, false); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// .gitattributes maps the jsonl on its own line.
	ga, err := os.ReadFile(filepath.Join(root, ".gitattributes"))
	if err != nil {
		t.Fatalf("reading .gitattributes: %v", err)
	}
	if !gitattributesHasBeadsMergeBytes(ga) {
		t.Errorf(".gitattributes missing the merge=beads mapping; got:\n%s", ga)
	}
	if !lineExactlyPresent(string(ga), beadsMergeAttrLine) {
		t.Errorf("expected exactly %q on its own line; got:\n%s", beadsMergeAttrLine, ga)
	}

	// merge.beads.driver is the PORTABLE relative value — no absolute path.
	val := gitConfig(t, root, "merge.beads.driver")
	if val != beadsMergeDriverConfig {
		t.Errorf("merge.beads.driver = %q, want %q", val, beadsMergeDriverConfig)
	}
	if strings.Contains(val, root) || strings.Contains(val, "/scripts/") {
		t.Errorf("merge.beads.driver baked an absolute path: %q", val)
	}
}

// TestProvision_NoGit_BestEffort: bootstrap in a non-git dir provisions the
// wrapper + .gitattributes and skips the git config write WITHOUT erroring.
func TestProvision_NoGit_BestEffort(t *testing.T) {
	root := t.TempDir()

	r, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run in non-git dir must not error: %v", err)
	}
	if r.BeadsConfErr != nil {
		t.Errorf("unexpected BeadsConfErr in non-git dir: %v", r.BeadsConfErr)
	}

	// Wrapper + .gitattributes still provisioned.
	if _, err := os.Stat(filepath.Join(root, "scripts", "bd-jsonl-merge-driver.sh")); err != nil {
		t.Errorf("wrapper not provisioned in non-git dir: %v", err)
	}
	ga, err := os.ReadFile(filepath.Join(root, ".gitattributes"))
	if err != nil || !gitattributesHasBeadsMergeBytes(ga) {
		t.Errorf(".gitattributes not provisioned in non-git dir (err=%v)", err)
	}
	// No git config recorded as a Skipped "already exist" entry.
	for _, s := range r.Skipped {
		if strings.Contains(s, "merge.beads.driver") {
			t.Errorf("non-git dir should not record a merge.beads.driver skip; got %q", s)
		}
	}
}

// TestProvision_NewlineSafeAppend: appending to a .gitattributes that lacks a
// trailing newline must land the mapping on its OWN line (not concatenated).
func TestProvision_NewlineSafeAppend(t *testing.T) {
	root := t.TempDir()
	// Pre-existing file with NO trailing newline.
	if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("*.png binary"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(root, false); err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".gitattributes"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	// The original line must be intact and the mapping on its own line.
	if strings.Contains(content, "binary"+beadsMergeAttrPattern) {
		t.Errorf("mapping concatenated onto prior line (corrupted): %q", content)
	}
	if !lineExactlyPresent(content, "*.png binary") {
		t.Errorf("original *.png binary line not preserved intact: %q", content)
	}
	if !lineExactlyPresent(content, beadsMergeAttrLine) {
		t.Errorf("expected %q on its own line; got %q", beadsMergeAttrLine, content)
	}
	if !gitattributesHasBeadsMergeBytes(data) {
		t.Errorf("detection should report the mapping present; got %q", content)
	}
}

// TestProvision_EnsureIfAbsent: a user-set driver / attribute / wrapper is NOT
// clobbered by a re-bootstrap.
func TestProvision_EnsureIfAbsent(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)

	// User-authored driver value and wrapper content.
	userDriver := "my-custom-driver %A %O %B"
	mustGit(t, root, "config", "merge.beads.driver", userDriver)
	scriptPath := filepath.Join(root, "scripts", "bd-jsonl-merge-driver.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	userScript := "#!/bin/sh\n# user wrapper\nexit 0\n"
	if err := os.WriteFile(scriptPath, []byte(userScript), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(root, false); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := gitConfig(t, root, "merge.beads.driver"); got != userDriver {
		t.Errorf("clobbered user driver: got %q, want %q", got, userDriver)
	}
	got, _ := os.ReadFile(scriptPath)
	if string(got) != userScript {
		t.Errorf("clobbered user wrapper: got %q", string(got))
	}
}

// TestProvision_Idempotent: re-bootstrapping does not duplicate the
// .gitattributes mapping or rewrite the config.
func TestProvision_Idempotent(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)

	if _, err := Run(root, false); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if _, err := Run(root, false); err != nil {
		t.Fatalf("second Run: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(root, ".gitattributes"))
	if n := strings.Count(string(data), beadsMergeAttrLine); n != 1 {
		t.Errorf("expected exactly 1 merge=beads mapping after re-bootstrap, got %d:\n%s", n, data)
	}
}

// TestProvision_CrossWorktree: the provisioned (relative) config + committed
// wrapper resolve from a LINKED worktree — no absolute path baked into the
// shared .git/config.
func TestProvision_CrossWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	gitInit(t, root)
	if _, err := Run(root, false); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Commit the provisioned files so a linked worktree checks them out.
	mustGit(t, root, "add", "-A")
	mustGit(t, root, "commit", "-q", "-m", "bootstrap")

	linked := filepath.Join(t.TempDir(), "linked")
	mustGit(t, root, "worktree", "add", "-q", "-b", "wt", linked)

	// Shared config value is the same (relative) value from the linked tree.
	if got := gitConfig(t, linked, "merge.beads.driver"); got != beadsMergeDriverConfig {
		t.Errorf("linked worktree sees merge.beads.driver=%q, want %q", got, beadsMergeDriverConfig)
	}
	// The relative path resolves to an executable wrapper in the linked tree.
	wrapper := filepath.Join(linked, "scripts", "bd-jsonl-merge-driver.sh")
	info, err := os.Stat(wrapper)
	if err != nil {
		t.Fatalf("wrapper absent in linked worktree: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("wrapper not executable in linked worktree: %v", info.Mode().Perm())
	}
}

// ─── e2e: a real both-sides-changed merge resolves via the provisioned driver ─

// TestProvision_E2EMergeResolvesViaDriver is the headline GREEN proof: in a
// freshly bootstrapped repo with bd/Dolt, a real `git merge` of a
// both-sides-changed .beads/issues.jsonl resolves CLEANLY via the provisioned
// driver (regenerate-from-DB) — `git ls-files -u` is EMPTY. Skips cleanly when
// bd is unavailable; designed to RUN where bd/Dolt exist.
func TestProvision_E2EMergeResolvesViaDriver(t *testing.T) {
	root, ids := setupBeadsMergeRepo(t)

	// Merge side into main. The provisioned driver regenerates the jsonl from
	// the (accumulated) Dolt DB, producing a superset → clean merge.
	out, err := runGitOut(t, root, "merge", "--no-edit", "side")
	if err != nil {
		t.Fatalf("merge failed (driver should have resolved it): %v\n%s", err, out)
	}

	// No unmerged stages.
	unmerged := mustGitOut(t, root, "ls-files", "-u")
	if strings.TrimSpace(unmerged) != "" {
		t.Fatalf("expected no unmerged stages, got:\n%s", unmerged)
	}

	// The merged jsonl is a superset of every bead id created on either side.
	merged, err := os.ReadFile(filepath.Join(root, ".beads", "issues.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if !strings.Contains(string(merged), id) {
			t.Errorf("merged jsonl missing bead %q (driver dropped a row):\n%s", id, merged)
		}
	}
}

// TestProvision_E2EMergeRevertedLeavesUnmergedStages is the tightened
// RED-on-revert: reverting the provisioning (here: unsetting the driver) makes
// the SAME both-sides-changed merge FAIL with unmerged stages — the ACTUAL
// merge-execution direction, not a doctor-detection substitute.
func TestProvision_E2EMergeRevertedLeavesUnmergedStages(t *testing.T) {
	root, _ := setupBeadsMergeRepo(t)

	// Revert provisioning: drop the driver config. With the merge=beads
	// attribute present but no driver, git falls back to a plain text merge,
	// which conflicts on the both-sides-changed jsonl.
	mustGit(t, root, "config", "--unset", "merge.beads.driver")

	_, err := runGitOut(t, root, "merge", "--no-edit", "side")
	if err == nil {
		t.Fatal("expected the merge to FAIL without the provisioned driver, but it succeeded")
	}
	unmerged := mustGitOut(t, root, "ls-files", "-u")
	if strings.TrimSpace(unmerged) == "" {
		t.Fatal("expected unmerged stages after reverting provisioning, got none")
	}
}

// setupBeadsMergeRepo builds a bootstrapped, bd-initialized git repo with a
// both-sides-changed .beads/issues.jsonl conflict staged: a base bead on main,
// a different bead on `side`, and a third bead on `main`. The local Dolt DB
// (untracked) accumulates all three, so the merge driver's regenerate-from-DB
// produces a superset. Returns the repo root and all created bead ids. Skips
// when bd is unavailable.
func setupBeadsMergeRepo(t *testing.T) (string, []string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd not available — e2e merge test requires bd/Dolt")
	}

	root := t.TempDir()
	gitInit(t, root)
	if _, err := Run(root, false); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	// bd init (sandbox, embedded Dolt). The Dolt DB lives under .beads/dolt
	// and is left UNTRACKED so it persists (accumulates) across git checkouts.
	bdInit(t, root)

	var ids []string
	commitBead := func(branch, title string) string {
		id := bdCreate(t, root, title)
		bdExport(t, root)
		// Track ONLY the canonical files — never bd runtime artifacts
		// (.beads/last-touched, the Dolt DB, sockets) which would otherwise
		// add their own merge conflicts unrelated to the jsonl driver.
		mustGit(t, root, "add", ".gitattributes", "scripts", filepath.Join(".beads", "issues.jsonl"))
		mustGit(t, root, "commit", "-q", "-m", branch+": "+title)
		return id
	}

	// Base on main.
	ids = append(ids, commitBead("base", "base bead"))

	// Diverge: a bead on side.
	mustGit(t, root, "checkout", "-q", "-b", "side")
	ids = append(ids, commitBead("side", "side bead"))

	// A different bead on main.
	mustGit(t, root, "checkout", "-q", "main")
	ids = append(ids, commitBead("main", "main bead"))

	return root, ids
}

// ─── small git/bd helpers (package-local) ────────────────────────────────────

func gitInit(t *testing.T, root string) {
	t.Helper()
	mustGit(t, root, "init", "-q", "-b", "main")
	mustGit(t, root, "config", "user.email", "test@example.com")
	mustGit(t, root, "config", "user.name", "Test")
	mustGit(t, root, "config", "commit.gpgsign", "false")
}

func mustGit(t *testing.T, root string, args ...string) {
	t.Helper()
	if out, err := runGitOut(t, root, args...); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func runGitOut(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func mustGitOut(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := runGitOut(t, root, args...)
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

func gitConfig(t *testing.T, root, key string) string {
	t.Helper()
	cmd := exec.Command("git", "config", "--get", key)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func bdInit(t *testing.T, root string) {
	t.Helper()
	cmd := exec.Command("bd", "init", "--sandbox", "--skip-hooks", "-q", "--database", "beads")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("bd init failed (skipping e2e): %v\n%s", err, out)
	}
}

func bdCreate(t *testing.T, root, title string) string {
	t.Helper()
	cmd := exec.Command("bd", "create", "--title", title, "--type", "task", "--priority", "2", "--json")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("bd create %q: %v", title, err)
	}
	var issue struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out, &issue); err != nil {
		t.Fatalf("parsing bd create output %q: %v", out, err)
	}
	if issue.ID == "" {
		t.Fatalf("bd create returned empty id: %s", out)
	}
	return issue.ID
}

func bdExport(t *testing.T, root string) {
	t.Helper()
	cmd := exec.Command("bd", "export", "-o", filepath.Join(".beads", "issues.jsonl"))
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bd export: %v\n%s", err, out)
	}
}

// lineExactlyPresent reports whether content has a line that equals want
// exactly (after trimming surrounding whitespace).
func lineExactlyPresent(content, want string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}
