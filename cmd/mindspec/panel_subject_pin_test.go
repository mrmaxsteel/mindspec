package main

import (
	"crypto/sha1"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

// A target object and the reviewed commit are distinct for annotated tags.
func createObjectPinPanel(t *testing.T, root, gate, target, reviewed string) string {
	t.Helper()
	digest, err := panel.SubjectDigest(rebindGit(t, root, "ls-tree", "-r", reviewed)+"\n", panel.Subject(gate, ".mindspec/specs/900-demo"))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "review", "object-pin")
	if err := paneltest.CreateV1(dir, panel.CreateInput{Spec: "900-demo", Target: target, Gate: gate, Round: 1, ExpectedReviewers: 2, ReviewedHeadSHA: reviewed, SubjectDigest: digest, Roster: []panel.RosterSlot{{Slot: "R1", Model: "codex", Lens: "correctness"}, {Slot: "R2", Model: "claude", Lens: "coverage"}}}); err != nil {
		t.Fatal(err)
	}
	for _, slot := range []string{"R1", "R2"} {
		rebindWrite(t, dir, slot+"-round-1.json", `{"verdict":"APPROVE"}`)
	}
	return dir
}

func assertObjectPinStale(t *testing.T, root, dir string) {
	t.Helper()
	out, err := runPanelVerbCmd("verify", "object-pin")
	if err != nil || !strings.Contains(out, "BLOCK:") || !strings.Contains(out, "commits landed after review") {
		t.Errorf("object pin must detect stale spec branch: err=%v output=%s", err, out)
	}
	if d := rebindGate(t, root, dir); d.Action != panel.Block || !strings.Contains(d.Message, "commits landed after review") {
		t.Errorf("object pin gate must block staleness: %+v", d)
	}
	out, err = runPanelVerbCmd("tally", "object-pin")
	if err == nil || !strings.Contains(out, "decision: BLOCK") {
		t.Errorf("tally must detect stale object pin: err=%v output=%s", err, out)
	}
}

func assertObjectPinRebind(t *testing.T, root, dir, reviewed, tip string) {
	t.Helper()
	out, err := runPanelVerbCmd("rebind", "object-pin", "--reason", "operator: checked spec branch")
	if err != nil {
		t.Fatalf("rebind must follow spec branch: %v %s", err, out)
	}
	p := rebindRegistration(t, dir).Panel
	if p.ReviewedHeadSHA != tip || len(p.Rebinds) != 1 || p.Rebinds[0].FromSHA != reviewed || p.Rebinds[0].ToSHA != tip {
		t.Fatalf("rebind must record spec tip %s: %+v", tip, p)
	}
	if d := rebindGate(t, root, dir); d.Action != panel.Allow {
		t.Fatalf("rebound panel must pass: %+v", d)
	}
}

func editPinnedPlan(t *testing.T, root string) string {
	t.Helper()
	rebindWrite(t, root, ".mindspec/specs/900-demo/plan.md", "# unreviewed plan edit\n")
	rebindGit(t, root, "add", ".mindspec/specs/900-demo/plan.md")
	rebindGit(t, root, "commit", "-qm", "unreviewed plan edit")
	return rebindGit(t, root, "rev-parse", "HEAD")
}

func TestPanelVerify_GarbageCollectedCommitPinBlocks(t *testing.T) {
	for _, gate := range []string{"spec_approve", "plan_approve"} {
		for _, length := range []int{40, 7} {
			t.Run(fmt.Sprintf("%s/%d", gate, length), func(t *testing.T) {
				root, _, base := setupRebind(t, "plan-pinned")
				rebindGit(t, root, "checkout", "-q", "spec/900-demo")
				rebindWrite(t, root, ".mindspec/specs/900-demo/plan.md", "# reviewed plan\n")
				rebindGit(t, root, "add", ".mindspec/specs/900-demo/plan.md")
				rebindGit(t, root, "commit", "-qm", "reviewed plan")
				reviewed := rebindGit(t, root, "rev-parse", "HEAD")
				dir := createObjectPinPanel(t, root, gate, reviewed[:length], reviewed)
				rebindGit(t, root, "reset", "-q", "--hard", base)
				editPinnedPlan(t, root)
				rebindGit(t, root, "checkout", "-q", "main")
				rebindGit(t, root, "reflog", "expire", "--expire=now", "--all")
				rebindGit(t, root, "gc", "-q", "--prune=now")
				if err := exec.Command("git", "-C", root, "cat-file", "-e", reviewed).Run(); err == nil {
					t.Fatal("reviewed object survived GC")
				}
				assertObjectPinStale(t, root, dir)
				// A deleted reviewed commit cannot supply the ancestry/diff evidence
				// required for rebind. The operator must start another review round.
				if out, err := runPanelVerbCmd("rebind", "object-pin", "--reason", "operator: checked spec branch"); err == nil || !strings.Contains(out, "rev-parse") {
					t.Fatalf("rebind must refuse missing reviewed commit: %v %s", err, out)
				}
			})
		}
	}
}

func TestPanelVerify_GarbageCollectedTagPinRebindsSpecTip(t *testing.T) {
	for _, length := range []int{40, 7} {
		t.Run(fmt.Sprint(length), func(t *testing.T) {
			root, _, reviewed := setupRebind(t, "plan-pinned")
			rebindGit(t, root, "tag", "-a", "review-pin", "-m", "reviewed", reviewed)
			target := rebindGit(t, root, "rev-parse", "refs/tags/review-pin")
			dir := createObjectPinPanel(t, root, "plan_approve", target[:length], reviewed)
			rebindGit(t, root, "tag", "-d", "review-pin")
			rebindGit(t, root, "checkout", "-q", "spec/900-demo")
			tip := editPinnedPlan(t, root)
			rebindGit(t, root, "checkout", "-q", "main")
			rebindGit(t, root, "reflog", "expire", "--expire=now", "--all")
			rebindGit(t, root, "gc", "-q", "--prune=now")
			if err := exec.Command("git", "-C", root, "cat-file", "-e", target).Run(); err == nil {
				t.Fatal("tag object survived GC")
			}
			assertObjectPinStale(t, root, dir)
			assertObjectPinRebind(t, root, dir, reviewed, tip)
		})
	}
}

func TestPanelVerify_AnnotatedTagObjectPinIgnoresSameNamedBranch(t *testing.T) {
	root, _, reviewed := setupRebind(t, "plan-pinned")
	rebindGit(t, root, "tag", "-a", "review-pin", "-m", "reviewed", reviewed)
	target := rebindGit(t, root, "rev-parse", "refs/tags/review-pin")
	rebindGit(t, root, "branch", target, reviewed)
	dir := createObjectPinPanel(t, root, "plan_approve", target, reviewed)
	rebindGit(t, root, "checkout", "-q", "spec/900-demo")
	tip := editPinnedPlan(t, root)
	rebindGit(t, root, "checkout", "-q", "main")
	assertObjectPinStale(t, root, dir)
	assertObjectPinRebind(t, root, dir, reviewed, tip)
}

func TestPanelVerify_AmbiguousObjectPrefixFollowsSpecBranch(t *testing.T) {
	root, _, reviewed := setupRebind(t, "plan-pinned")
	prefix := reviewed[:4]
	// Find a blob sharing the commit prefix without relying on random object IDs.
	var content string
	for i := 0; ; i++ {
		content = fmt.Sprintf("ambiguous review pin %d\n", i)
		sum := sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(content), content)))
		if strings.HasPrefix(fmt.Sprintf("%x", sum), prefix) {
			break
		}
	}
	name := filepath.Join(t.TempDir(), "collision")
	if err := os.WriteFile(name, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	blob := rebindGit(t, root, "hash-object", "-w", name)
	if !strings.HasPrefix(blob, prefix) {
		t.Fatalf("fixture collision failed: %s / %s", blob, prefix)
	}
	if err := exec.Command("git", "-C", root, "rev-parse", "--verify", "--quiet", prefix).Run(); err == nil {
		t.Fatal("prefix must be ambiguous")
	}
	dir := createObjectPinPanel(t, root, "plan_approve", prefix, reviewed)
	rebindGit(t, root, "checkout", "-q", "spec/900-demo")
	tip := editPinnedPlan(t, root)
	rebindGit(t, root, "checkout", "-q", "main")
	assertObjectPinStale(t, root, dir)
	assertObjectPinRebind(t, root, dir, reviewed, tip)
}
