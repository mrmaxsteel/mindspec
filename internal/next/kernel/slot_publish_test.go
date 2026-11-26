package kernel_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// attemptVerdict writes data as the verdict of a slot attempt directory, the
// path that the slot prompt names. No commit holds it.
func attemptVerdict(t *testing.T, f *kerneltest.Fixture, slot, data string) string {
	t.Helper()
	iso, err := config.IsolationRootFor(f.Root)
	f.Must(err)
	layout, err := panel.NewSlotLayout(f.Root, iso, f.SpecID, "panel", 1, slot)
	f.Must(err)
	file := filepath.Join(layout.Dir, "attempt-1", "verdict.json")
	f.Must(os.MkdirAll(filepath.Dir(file), 0755))
	f.Must(os.WriteFile(file, []byte(data), 0644))
	return file
}

// gitObjects lists every ref and every commit object of the fixture
// repository, unreachable commits included.
func gitObjects(t *testing.T, f *kerneltest.Fixture) string {
	t.Helper()
	refs := actorCommand(t, f, "for-each-ref", "--format=%(refname) %(objectname)")
	var commits []string
	for _, line := range strings.Split(actorCommand(t, f, "cat-file", "--batch-all-objects", "--batch-check=%(objecttype) %(objectname)"), "\n") {
		if strings.HasPrefix(line, "commit ") {
			commits = append(commits, line)
		}
	}
	return refs + "\n" + strings.Join(commits, "\n")
}

func slotPublishFixture(t *testing.T) (*kerneltest.Fixture, string, kernel.LeaseResult, string, string) {
	t.Helper()
	f, id, lease, subject := slotFixture(t)
	kerneltest.Register(f, f.Items[0]+".review", kernel.RegistrationRecord{Slug: "panel", Round: 1, Subject: subject, Slots: []string{"R1", "R2"}})
	return f, id, lease, subject, ".mindspec/specs/" + f.SpecID + "/reviews/panel/R1-round-1.json"
}

func TestS134Submit_SlotAttemptVerdictPublished(t *testing.T) {
	f, id, lease, subject, want := slotPublishFixture(t)
	branch := "refs/heads/spec/" + f.SpecID
	tip := actorCommand(t, f, "rev-parse", branch)
	data := verdictBytes(subject, "R1")
	file := attemptVerdict(t, f, "R1", data)
	if _, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file}); err != nil {
		t.Fatalf("the printed slot submit was refused: %v", err)
	}
	if status := actorRecord(t, f, id).Status; status != "closed" {
		t.Fatalf("slot status %q, want closed", status)
	}
	receipt := actorReceipt(t, f, id)
	if receipt.Path != want {
		t.Fatalf("receipt path %q, want the registered path %q", receipt.Path, want)
	}
	if err := kernel.ValidReceipt(f.Root, receipt, nil); err != nil {
		t.Fatalf("published receipt is not valid: %v", err)
	}
	if held := actorCommand(t, f, "show", receipt.Commit+":"+want); held != data {
		t.Fatalf("published bytes differ: %q", held)
	}
	// The spec branch holds the commit, so its push and its merge carry it.
	if moved := actorCommand(t, f, "rev-parse", branch); moved != receipt.Commit {
		t.Fatalf("spec branch is at %s, want the holding commit %s", moved, receipt.Commit)
	}
	if parent := actorCommand(t, f, "rev-parse", receipt.Commit+"^1"); parent != tip {
		t.Fatalf("holding commit parent %s, want the old spec branch tip %s", parent, tip)
	}
	if changed := actorCommand(t, f, "diff-tree", "--no-commit-id", "--name-only", "-r", receipt.Commit); changed != want {
		t.Fatalf("holding commit changes %q, want only %q", changed, want)
	}
	if refs := actorCommand(t, f, "for-each-ref", "refs/mindspec/"); refs != "" {
		t.Fatalf("publish wrote a local-only ref: %s", refs)
	}
	if head := actorCommand(t, f, "rev-parse", "HEAD"); head != subject {
		t.Fatalf("publish moved HEAD to %s", head)
	}
}

func TestS134Submit_SlotAttemptOfOtherSlotNamesCommand(t *testing.T) {
	f, id, lease, subject, _ := slotPublishFixture(t)
	file := attemptVerdict(t, f, "R2", verdictBytes(subject, "R1"))
	before := gitObjects(t, f)
	_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file})
	assertSubmitRefusal(t, err, "artifact has no holding commit", "mindspec panel prompt 'panel' --slot R1 --spec '"+f.SpecID+"', then write the verdict to the path it names and run the submit command it prints")
	if actorRecord(t, f, id).Status == "closed" {
		t.Fatal("a verdict of another slot closed this slot")
	}
	if after := gitObjects(t, f); after != before {
		t.Fatalf("a refused submit published a verdict:\nbefore %s\nafter %s", before, after)
	}
}

// A refused submit on the attempt path writes no ref and no commit: the checks
// run before the publish.
func TestS134Submit_RefusedSlotVerdictPublishesNothing(t *testing.T) {
	for _, name := range []string{"draft", "wrong-reviewed-sha", "changed-after-close"} {
		t.Run(name, func(t *testing.T) {
			f, id, lease, subject, _ := slotPublishFixture(t)
			data := verdictBytes(subject, "R1")
			switch name {
			case "draft":
				data = strings.Replace(data, `"final"`, `"draft"`, 1)
			case "wrong-reviewed-sha":
				data = verdictBytes(strings.Repeat("0", 40), "R1")
			case "changed-after-close":
				file := attemptVerdict(t, f, "R1", data)
				_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file})
				f.Must(err)
				data = strings.Replace(data, `"reviewed"`, `"reviewed again"`, 1)
			}
			file := attemptVerdict(t, f, "R1", data)
			before := gitObjects(t, f)
			_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file})
			if err == nil {
				t.Fatal("submit accepted the refusal fixture")
			}
			if after := gitObjects(t, f); after != before {
				t.Fatalf("a refused submit wrote a ref or commit: %v\nbefore %s\nafter %s", err, before, after)
			}
		})
	}
}

// A retry of the publish with the same bytes returns the first holding commit
// and does not move the spec branch.
func TestS134Submit_SlotVerdictPublishRetryIsIdempotent(t *testing.T) {
	f, id, _, subject, want := slotPublishFixture(t)
	branch := "refs/heads/spec/" + f.SpecID
	file := attemptVerdict(t, f, "R1", verdictBytes(subject, "R1"))
	first, err := kernel.PublishSlotVerdictForTest(f.State(), id, file)
	f.Must(err)
	before := gitObjects(t, f)
	second, err := kernel.PublishSlotVerdictForTest(f.State(), id, file)
	f.Must(err)
	if second != first {
		t.Fatalf("retry gave holding commit %s, want %s", second, first)
	}
	if after := gitObjects(t, f); after != before {
		t.Fatalf("retry wrote a ref or commit:\nbefore %s\nafter %s", before, after)
	}
	if tip := actorCommand(t, f, "rev-parse", branch); tip != first {
		t.Fatalf("spec branch at %s, want %s", tip, first)
	}
	if held := actorCommand(t, f, "rev-parse", first+":"+want); held == "" {
		t.Fatal("holding commit lost the verdict")
	}
}

// A worktree that has the spec branch checked out stays clean at the verdict
// path, and its own staged and unstaged work stays as it was.
func TestS134Submit_SlotVerdictKeepsSpecWorktreeWork(t *testing.T) {
	f, id, lease, subject, want := slotPublishFixture(t)
	tree := filepath.Join(t.TempDir(), "spec")
	actorCommand(t, f, "worktree", "add", tree, "spec/"+f.SpecID)
	f.Must(os.WriteFile(filepath.Join(tree, "staged.txt"), []byte("staged\n"), 0644))
	f.Must(os.WriteFile(filepath.Join(tree, "unstaged.txt"), []byte("unstaged\n"), 0644))
	actorCommand(t, f, "-C", tree, "add", "staged.txt")
	status := actorCommand(t, f, "-C", tree, "status", "--porcelain")
	file := attemptVerdict(t, f, "R1", verdictBytes(subject, "R1"))
	if _, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file}); err != nil {
		t.Fatal(err)
	}
	if after := actorCommand(t, f, "-C", tree, "status", "--porcelain"); after != status {
		t.Fatalf("spec worktree status changed:\nbefore %s\nafter %s", status, after)
	}
	if head := actorCommand(t, f, "-C", tree, "rev-parse", "HEAD"); head != actorReceipt(t, f, id).Commit {
		t.Fatalf("spec worktree HEAD %s, want the holding commit", head)
	}
	if _, err := os.Stat(filepath.Join(tree, filepath.FromSlash(want))); err != nil {
		t.Fatalf("spec worktree lacks the verdict file: %v", err)
	}
}
