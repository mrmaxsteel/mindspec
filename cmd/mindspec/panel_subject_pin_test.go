package main

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/instruct"
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

// SHA-shaped targets follow the spec subject even when another ref has that name.
func TestPanelVerify_CommitPinRefCollisionTracksSpecBranch(t *testing.T) {
	for _, kind := range []string{"branch", "tag"} {
		for _, length := range []int{7, 40} {
			t.Run(fmt.Sprintf("%s/%d", kind, length), func(t *testing.T) {
				root, _, reviewed := setupRebind(t, "plan-pinned")
				target := reviewed[:length]
				dir := createObjectPinPanel(t, root, "plan_approve", target, reviewed)
				rebindGit(t, root, kind, target, reviewed)
				rebindGit(t, root, "checkout", "-q", "spec/900-demo")
				tip := editPinnedPlan(t, root)
				rebindGit(t, root, "checkout", "-q", "main")
				assertObjectPinStale(t, root, dir)
				if out := instruct.PanelState(root); !strings.Contains(out, "**object-pin** — gate would BLOCK") {
					t.Errorf("instruct must detect stale pin: %s", out)
				}
				assertObjectPinRebind(t, root, dir, reviewed, tip)
			})
		}
	}
}

func TestPanelVerify_PinnedSpecBranchTagCannotShadowRealBranch(t *testing.T) {
	for _, gate := range []string{"spec_approve", "plan_approve"} {
		t.Run(gate, func(t *testing.T) {
			root, _, reviewed := setupRebind(t, "plan-pinned")
			dir := createObjectPinPanel(t, root, gate, reviewed, reviewed)
			rebindGit(t, root, "tag", "spec/900-demo", reviewed)
			rebindGit(t, root, "checkout", "-q", "refs/heads/spec/900-demo")
			tip := editPinnedPlan(t, root)
			rebindGit(t, root, "update-ref", "refs/heads/spec/900-demo", tip)
			rebindGit(t, root, "checkout", "-q", "main")
			assertObjectPinStale(t, root, dir)
			if out := instruct.PanelState(root); !strings.Contains(out, "**object-pin** — gate would BLOCK") {
				t.Errorf("instruct must follow real spec branch: %s", out)
			}
			assertObjectPinRebind(t, root, dir, reviewed, tip)
		})
	}
}

func TestPanelVerify_PresentSpecBranchMissingObjectFailsClosed(t *testing.T) {
	for _, gate := range []string{"spec_approve", "plan_approve"} {
		t.Run(gate, func(t *testing.T) {
			root, _, reviewed := setupRebind(t, "plan-pinned")
			dir := createObjectPinPanel(t, root, gate, reviewed, reviewed)
			rebindGit(t, root, "checkout", "-q", "spec/900-demo")
			editPinnedPlan(t, root)
			rebindGit(t, root, "checkout", "-q", "main")
			rebindWrite(t, root, ".git/refs/heads/spec/900-demo", strings.Repeat("1", 40)+"\n")
			out, err := runPanelVerbCmd("verify", "object-pin")
			if err == nil && !strings.Contains(out, "BLOCK:") {
				t.Errorf("present branch with unreadable commit must fail closed: %s", out)
			}
			if d := rebindGate(t, root, dir); d.Action != panel.Block {
				t.Errorf("unreadable spec branch gate must BLOCK: %+v", d)
			}
			if out := instruct.PanelState(root); !strings.Contains(out, "**object-pin** — gate would BLOCK") {
				t.Errorf("instruct must block unreadable spec branch: %s", out)
			}
			out, err = runPanelVerbCmd("tally", "object-pin")
			if err == nil || !strings.Contains(out, "decision: BLOCK") {
				t.Errorf("tally must block unreadable spec branch: %v %s", err, out)
			}
			if out, err := runPanelVerbCmd("rebind", "object-pin", "--reason", "operator: checked spec branch"); err == nil {
				t.Errorf("rebind must refuse unreadable spec branch: %s", out)
			}
		})
	}
}

func requirePanelShowRefExists(t *testing.T, root string) {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "show-ref", "--exists", "refs/heads/spec/900-demo").CombinedOutput()
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 129 {
		t.Skip("git show-ref --exists requires git 2.43 or later")
	}
	if err != nil {
		t.Fatalf("probe git show-ref --exists: %v: %s", err, out)
	}
}

func assertObjectPinSubjectBlocked(t *testing.T, root, dir string) {
	t.Helper()
	facts := resolvePanelGateFacts(root, rebindRegistration(t, dir))
	if !errors.Is(facts.SubjectErr, panel.ErrSubjectHead) || facts.MissingRef || facts.GitErr != nil {
		t.Errorf("unreadable branch must report ErrSubjectHead, not missing ref: %+v", facts)
	}
	if d := panel.PanelGateDecision(facts); d.Action != panel.Block {
		t.Errorf("unreadable spec branch gate must BLOCK: %+v", d)
	}
	if out, err := runPanelVerbCmd("verify", "object-pin"); err != nil || !strings.Contains(out, "BLOCK:") || !strings.Contains(out, panel.ErrSubjectHead.Error()) {
		t.Errorf("verify must BLOCK on ErrSubjectHead: %v %s", err, out)
	}
	if out, err := runPanelVerbCmd("tally", "object-pin"); err == nil || !strings.Contains(out, "decision: BLOCK") || !strings.Contains(out, panel.ErrSubjectHead.Error()) {
		t.Errorf("tally must BLOCK on ErrSubjectHead: %v %s", err, out)
	}
	if out := instruct.PanelState(root); !strings.Contains(out, "**object-pin** — gate would BLOCK") || !strings.Contains(out, panel.ErrSubjectHead.Error()) {
		t.Errorf("instruct must block unreadable spec branch: %s", out)
	}
	before := rebindRegistration(t, dir).Panel
	if out, err := runPanelVerbCmd("rebind", "object-pin", "--reason", "operator: checked spec branch"); err == nil {
		t.Errorf("rebind must refuse an unreadable spec branch: %s", out)
	}
	after := rebindRegistration(t, dir).Panel
	if after.ReviewedHeadSHA != before.ReviewedHeadSHA || len(after.Rebinds) != len(before.Rebinds) {
		t.Errorf("refused rebind changed the binding: before=%+v after=%+v", before, after)
	}
}

// PR 273: an uncertain branch lookup must never reuse a fresh reviewed SHA.
func TestPanelVerify_CorruptSpecBranchFailsClosed(t *testing.T) {
	for _, gate := range []string{"spec_approve", "plan_approve"} {
		for _, tc := range []struct{ name, contents string }{
			{"empty", ""},
			{"truncated", "1234\n"},
			{"garbage", "garbage\n"},
			{"dangling-symbolic", "ref: refs/heads/nowhere\n"},
		} {
			t.Run(gate+"/"+tc.name, func(t *testing.T) {
				root, _, reviewed := setupRebind(t, "plan-pinned")
				requirePanelShowRefExists(t, root)
				dir := createObjectPinPanel(t, root, gate, reviewed, reviewed)
				rebindGit(t, root, "checkout", "-q", "spec/900-demo")
				editPinnedPlan(t, root)
				rebindGit(t, root, "checkout", "-q", "main")
				rebindWrite(t, root, ".git/refs/heads/spec/900-demo", tc.contents)
				assertObjectPinSubjectBlocked(t, root, dir)
			})
		}
	}
}

func TestPanelVerify_NonCommitSpecBranchFailsClosed(t *testing.T) {
	for _, gate := range []string{"spec_approve", "plan_approve"} {
		for _, object := range []string{"HEAD^{tree}", "HEAD:source.go"} {
			t.Run(gate+"/"+object, func(t *testing.T) {
				root, _, reviewed := setupRebind(t, "plan-pinned")
				requirePanelShowRefExists(t, root)
				dir := createObjectPinPanel(t, root, gate, reviewed, reviewed)
				sha := rebindGit(t, root, "rev-parse", object)
				// update-ref rejects non-commit branch tips, so write the loose ref.
				rebindWrite(t, root, ".git/refs/heads/spec/900-demo", sha+"\n")
				assertObjectPinSubjectBlocked(t, root, dir)
			})
		}
	}
}

func TestPanelVerify_DeletedSpecBranchFallsBackToCommitPin(t *testing.T) {
	for _, gate := range []string{"spec_approve", "plan_approve"} {
		t.Run(gate, func(t *testing.T) {
			root, _, reviewed := setupRebind(t, "plan-pinned")
			requirePanelShowRefExists(t, root)
			dir := createObjectPinPanel(t, root, gate, reviewed, reviewed)
			rebindGit(t, root, "branch", "-D", "spec/900-demo")
			if d := rebindGate(t, root, dir); d.Action != panel.Allow {
				t.Fatalf("deleted spec branch must fall back to target: %+v", d)
			}
			out, err := runPanelVerbCmd("verify", "object-pin")
			if err != nil || strings.Contains(out, "BLOCK:") || strings.Contains(out, "no longer exists") {
				t.Fatalf("existing pin must remain fresh after branch deletion: %v %s", err, out)
			}
		})
	}
}

func TestPanelVerify_HexLookingNameTracksSpecBranch(t *testing.T) {
	for _, kind := range []string{"branch", "tag"} {
		t.Run(kind, func(t *testing.T) {
			root, _, reviewed := setupRebind(t, "plan-pinned")
			dir := createObjectPinPanel(t, root, "plan_approve", "cafe", reviewed)
			rebindGit(t, root, kind, "cafe", reviewed)
			rebindGit(t, root, "checkout", "-qb", "other-subject")
			otherTip := editPinnedPlan(t, root)
			rebindGit(t, root, "checkout", "-q", "main")
			rebindGit(t, root, kind, "-f", "cafe", otherTip)
			if d := rebindGate(t, root, dir); d.Action != panel.Allow {
				t.Fatalf("hex-looking name must ignore same-named %s tip: %+v", kind, d)
			}
			rebindGit(t, root, "checkout", "-q", "spec/900-demo")
			tip := editPinnedPlan(t, root)
			rebindGit(t, root, "checkout", "-q", "main")
			assertObjectPinStale(t, root, dir)
			assertObjectPinRebind(t, root, dir, reviewed, tip)
		})
	}
}

func TestPanelVerify_NamedTargetKeepsOwnTip(t *testing.T) {
	for _, kind := range []string{"branch", "tag"} {
		t.Run(kind, func(t *testing.T) {
			root, _, reviewed := setupRebind(t, "plan-pinned")
			dir := createObjectPinPanel(t, root, "plan_approve", "review-target", reviewed)
			rebindGit(t, root, kind, "review-target", reviewed)
			rebindGit(t, root, "checkout", "-q", "spec/900-demo")
			tip := editPinnedPlan(t, root)
			rebindGit(t, root, "checkout", "-q", "main")
			if d := rebindGate(t, root, dir); d.Action != panel.Allow {
				t.Fatalf("ordinary %s target must keep its own tip: %+v", kind, d)
			}
			rebindGit(t, root, kind, "-f", "review-target", tip)
			assertObjectPinStale(t, root, dir)
		})
	}
}

func TestPanelVerify_BeadPanelKeepsBeadBranchSubject(t *testing.T) {
	root, dir, _ := setupRebind(t, "bead")
	rebindGit(t, root, "checkout", "-qb", "spec/900-demo")
	editPinnedPlan(t, root)
	rebindGit(t, root, "checkout", "-q", "bead/mindspec-demo.1")
	if d := rebindGate(t, root, dir); d.Action != panel.Allow {
		t.Fatalf("spec branch edit must not move bead subject: %+v", d)
	}
	editPinnedPlan(t, root)
	if d := rebindGate(t, root, dir); d.Action != panel.Block {
		t.Fatalf("bead edit must block: %+v", d)
	}
}

func TestPanelVerify_DeletedSpecBranchAndMissingPinUsesAdvisory(t *testing.T) {
	root, _, reviewed := setupRebind(t, "plan-pinned")
	dir := createObjectPinPanel(t, root, "plan_approve", strings.Repeat("1", 40), reviewed)
	rebindGit(t, root, "branch", "-D", "spec/900-demo")
	facts := resolvePanelGateFacts(root, rebindRegistration(t, dir))
	if !facts.MissingRef || facts.SubjectErr != nil || facts.GitErr != nil {
		t.Fatalf("confirmed absent branch and target must take missing-ref path: %+v", facts)
	}
	if d := panel.PanelGateDecision(facts); d.Action != panel.Warn {
		t.Fatalf("confirmed deletion should retain advisory: %+v", d)
	}
}
