package kernel_test

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// specWorktree adds a linked worktree that has the spec branch checked out.
func specWorktree(t *testing.T, f *kerneltest.Fixture) string {
	t.Helper()
	tree := filepath.Join(t.TempDir(), "spec")
	actorCommand(t, f, "worktree", "add", tree, "spec/"+f.SpecID)
	real, err := filepath.EvalSymlinks(tree)
	f.Must(err)
	return real
}

// assertOneRecovery checks that err has exactly one recovery line, that it is
// the last line, and that it contains want.
func assertOneRecovery(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatal("want a refusal with a recovery, got none")
	}
	var failure *termsafe.Failure
	if !errors.As(err, &failure) {
		t.Fatalf("error has no recovery: %v", err)
	}
	lines := strings.Split(err.Error(), "\n")
	count := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "recovery:") {
			count++
		}
	}
	last := lines[len(lines)-1]
	if count != 1 || !strings.HasPrefix(last, "recovery:") || !strings.Contains(last, want) {
		t.Fatalf("want one recovery line, last, with %q:\n%v", want, err)
	}
}

// gitWrapper puts a git script first on PATH. The script runs action once,
// the first time git runs in dir with a listed subcommand. For a subcommand in
// after, the action runs when that git command has returned; for one in
// before, it runs before the command starts. Every other call goes to the
// real git. It returns the marker file that the action creates.
func gitWrapper(t *testing.T, dir string, before, after []string, action string) string {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	mark := filepath.Join(t.TempDir(), "fired")
	script := "#!/bin/sh\n" +
		"REAL_GIT='" + realGit + "'\n" +
		"if [ \"$(pwd -P)\" = '" + dir + "' ] && [ ! -e '" + mark + "' ]; then\n" +
		"  case \"$1\" in\n"
	if len(before) > 0 {
		script += "  " + strings.Join(before, "|") + ")\n" +
			"    : > '" + mark + "'\n" +
			"    " + action + "\n" +
			"    exec \"$REAL_GIT\" \"$@\" ;;\n"
	}
	if len(after) > 0 {
		script += "  " + strings.Join(after, "|") + ")\n" +
			"    out=$(\"$REAL_GIT\" \"$@\"); rc=$?\n" +
			"    : > '" + mark + "'\n" +
			"    " + action + "\n" +
			"    [ -n \"$out\" ] && printf '%s\\n' \"$out\"\n" +
			"    exit $rc ;;\n"
	}
	script += "  esac\nfi\nexec \"$REAL_GIT\" \"$@\"\n"
	f := filepath.Join(bin, "git")
	if err := os.WriteFile(f, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return mark
}

func fired(t *testing.T, mark string) {
	t.Helper()
	if _, err := os.Stat(mark); err != nil {
		t.Fatalf("the concurrent change did not run, so the test proves nothing: %v", err)
	}
}

// An index lock that another git command holds stops the sync after the spec
// branch moved. The printed recovery (the same submit again) brings the
// worktree to the holding commit, and a later ordinary commit there keeps the
// verdict.
func TestS134Submit_SlotSyncRetryRepairsWorktree(t *testing.T) {
	f, id, lease, subject, want := slotPublishFixture(t)
	tree := specWorktree(t, f)
	lock := actorCommand(t, f, "-C", tree, "rev-parse", "--path-format=absolute", "--git-path", "index.lock")
	f.Must(os.WriteFile(lock, []byte("held by another git command"), 0o600))
	file := attemptVerdict(t, f, "R1", verdictBytes(subject, "R1"))
	_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file})
	assertOneRecovery(t, err, "run the same submit command again")
	// The git cause shows on one readable line: no escaped line break, and the
	// recovery names each cause that stops the index move.
	lines := strings.Split(err.Error(), "\n")
	if len(lines) != 2 || strings.Contains(lines[0], `\n`) || !strings.Contains(lines[0], "index.lock") {
		t.Fatalf("sync failure does not show the git cause on one readable line:\n%v", err)
	}
	if last := lines[len(lines)-1]; !strings.Contains(last, "index lock") || !strings.Contains(last, "unmerged path") || !strings.Contains(last, "unstage any change") {
		t.Fatalf("sync recovery does not name the lock, the unmerged path and the staged change: %q", last)
	}
	if actorRecord(t, f, id).Status == "closed" {
		t.Fatal("slot closed while a worktree is not in step")
	}
	f.Must(os.Remove(lock))
	if _, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file}); err != nil {
		t.Fatalf("the printed recovery was refused: %v", err)
	}
	if status := actorCommand(t, f, "-C", tree, "status", "--porcelain"); status != "" {
		t.Fatalf("retry left the spec worktree out of step:\n%s", status)
	}
	f.Must(os.WriteFile(filepath.Join(tree, "unrelated.txt"), []byte("work\n"), 0o644))
	actorCommand(t, f, "-C", tree, "add", "unrelated.txt")
	actorCommand(t, f, "-C", tree, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "unrelated")
	if changed := actorCommand(t, f, "-C", tree, "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD"); changed != "unrelated.txt" {
		t.Fatalf("an unrelated commit changed %q", changed)
	}
	if held := actorCommand(t, f, "show", "spec/"+f.SpecID+":"+want); held != verdictBytes(subject, "R1") {
		t.Fatalf("spec branch lost the verdict: %q", held)
	}
}

// A crash between the ref update and the sync leaves the worktree index at the
// old tip. A retry with the same bytes repairs it from the parent of the
// holding commit.
func TestS134Submit_SlotPublishRetryRepairsStaleIndex(t *testing.T) {
	f, id, _, subject, _ := slotPublishFixture(t)
	tree := specWorktree(t, f)
	branch := "refs/heads/spec/" + f.SpecID
	tip := actorCommand(t, f, "rev-parse", branch)
	file := attemptVerdict(t, f, "R1", verdictBytes(subject, "R1"))
	first, err := kernel.PublishSlotVerdictForTest(f.State(), id, file)
	f.Must(err)
	// Simulate the crash: put the worktree index and files back at the old tip.
	actorCommand(t, f, "-C", tree, "read-tree", "--reset", "-u", tip)
	if status := actorCommand(t, f, "-C", tree, "status", "--porcelain"); status == "" {
		t.Fatal("fixture did not make the index stale")
	}
	second, err := kernel.PublishSlotVerdictForTest(f.State(), id, file)
	f.Must(err)
	if second != first {
		t.Fatalf("retry gave %s, want %s", second, first)
	}
	if status := actorCommand(t, f, "-C", tree, "status", "--porcelain"); status != "" {
		t.Fatalf("retry did not repair the stale index:\n%s", status)
	}
}

// A user stages different bytes at the verdict path between the spec branch
// move and the index write. The sync keeps the staged bytes and the file,
// stops with a recovery, and the recovery finishes it.
func TestS134Submit_SlotSyncKeepsConcurrentStagedEdit(t *testing.T) {
	f, id, lease, subject, want := slotPublishFixture(t)
	tree := specWorktree(t, f)
	user := "user staged verdict edit\n"
	action := "mkdir -p \"$(dirname '" + want + "')\" && printf 'user staged verdict edit\\n' > '" + want + "' && \"$REAL_GIT\" add -- '" + want + "'"
	mark := gitWrapper(t, tree, []string{"read-tree", "update-index", "checkout-index"}, []string{"ls-files"}, action)
	file := attemptVerdict(t, f, "R1", verdictBytes(subject, "R1"))
	_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file})
	fired(t, mark)
	if staged := actorCommand(t, f, "-C", tree, "show", ":"+want); staged+"\n" != user {
		t.Fatalf("sync replaced the staged bytes: index holds %q", staged)
	}
	if work, e := os.ReadFile(filepath.Join(tree, filepath.FromSlash(want))); e != nil || string(work) != user {
		t.Fatalf("sync replaced the file: %q %v", work, e)
	}
	assertOneRecovery(t, err, "run the same submit command again")
	// The recovery: unstage the change, then submit again.
	actorCommand(t, f, "-C", tree, "restore", "--staged", "--", want)
	if _, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file}); err != nil {
		t.Fatalf("the recovery was refused: %v", err)
	}
	if work, e := os.ReadFile(filepath.Join(tree, filepath.FromSlash(want))); e != nil || string(work) != user {
		t.Fatalf("the retry replaced the file: %q %v", work, e)
	}
	if staged := actorCommand(t, f, "-C", tree, "show", ":"+want); staged != verdictBytes(subject, "R1") {
		t.Fatalf("index at the verdict path is %q, want the holding commit's bytes", staged)
	}
}

// A user (or an editor) writes the verdict file between the inspection of the
// worktree and the write. The sync keeps the bytes. One case adds the path; the
// other replaces a verdict that the spec branch already holds.
func TestS134Submit_SlotSyncKeepsConcurrentUnstagedEdit(t *testing.T) {
	for _, name := range []string{"new-path", "existing-verdict"} {
		t.Run(name, func(t *testing.T) {
			f, id, lease, subject, want := slotPublishFixture(t)
			tree := specWorktree(t, f)
			data := verdictBytes(subject, "R1")
			if name == "existing-verdict" {
				f.Must(os.MkdirAll(filepath.Dir(filepath.Join(tree, want)), 0o755))
				f.Must(os.WriteFile(filepath.Join(tree, want), []byte(data), 0o644))
				actorCommand(t, f, "-C", tree, "add", want)
				actorCommand(t, f, "-C", tree, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "existing verdict")
				data = strings.Replace(data, `"rationale":"reviewed"`, `"rationale":"reviewed again"`, 1)
			}
			action := "mkdir -p \"$(dirname '" + want + "')\" && printf 'user unstaged verdict edit' > '" + want + "'"
			mark := gitWrapper(t, tree, []string{"read-tree", "checkout-index"}, []string{"hash-object"}, action)
			file := attemptVerdict(t, f, "R1", data)
			if _, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file}); err != nil {
				t.Fatalf("submit refused: %v", err)
			}
			fired(t, mark)
			if work, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(want))); err != nil || string(work) != "user unstaged verdict edit" {
				t.Fatalf("sync overwrote the unstaged edit: %q %v", work, err)
			}
			if staged := actorCommand(t, f, "-C", tree, "show", ":"+want); staged != data {
				t.Fatalf("index at the verdict path is %q, want the published bytes", staged)
			}
		})
	}
}

// Another writer commits on the spec branch between the read of its tip and
// the ref update. The compare-and-swap update refuses, publish builds again on
// the new tip, and the concurrent commit stays on the branch.
func TestS134Submit_SlotPublishKeepsConcurrentBranchCommit(t *testing.T) {
	f, id, _, subject, want := slotPublishFixture(t)
	branch := "refs/heads/spec/" + f.SpecID
	root, err := filepath.EvalSymlinks(f.Root)
	f.Must(err)
	note := filepath.Join(t.TempDir(), "concurrent")
	action := "c=$(\"$REAL_GIT\" -c user.name=Other -c user.email=other@example.invalid commit-tree " + branch + "^{tree} -p " + branch + " -m concurrent) && \"$REAL_GIT\" update-ref " + branch + " \"$c\" && printf '%s' \"$c\" > '" + note + "'"
	file := attemptVerdict(t, f, "R1", verdictBytes(subject, "R1"))
	state := f.State()
	mark := gitWrapper(t, root, []string{"update-ref"}, nil, action)
	commit, err := kernel.PublishSlotVerdictForTest(state, id, file)
	f.Must(err)
	fired(t, mark)
	concurrent, err := os.ReadFile(note)
	f.Must(err)
	if _, err := f.Command("git", "merge-base", "--is-ancestor", string(concurrent), branch); err != nil {
		t.Fatalf("publish dropped the concurrent spec branch commit %s", concurrent)
	}
	if parent := actorCommand(t, f, "rev-parse", commit+"^1"); parent != string(concurrent) {
		t.Fatalf("holding commit parent %s, want the concurrent commit %s", parent, concurrent)
	}
	if tip := actorCommand(t, f, "rev-parse", branch); tip != commit {
		t.Fatalf("spec branch at %s, want the holding commit %s", tip, commit)
	}
	if held := actorCommand(t, f, "show", commit+":"+want); held != verdictBytes(subject, "R1") {
		t.Fatalf("holding commit bytes %q", held)
	}
}

// existingVerdictTree commits a verdict at the registered path in a linked
// spec worktree and returns the worktree and the changed bytes for the next
// submit.
func existingVerdictTree(t *testing.T, f *kerneltest.Fixture, subject, want string) (string, string) {
	t.Helper()
	tree := specWorktree(t, f)
	data := verdictBytes(subject, "R1")
	f.Must(os.MkdirAll(filepath.Dir(filepath.Join(tree, want)), 0o755))
	f.Must(os.WriteFile(filepath.Join(tree, want), []byte(data), 0o644))
	actorCommand(t, f, "-C", tree, "add", want)
	actorCommand(t, f, "-C", tree, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "existing verdict")
	return tree, strings.Replace(data, `"rationale":"reviewed"`, `"rationale":"reviewed again"`, 1)
}

// A user deletes the tracked verdict file before submit and does not stage
// the deletion. The sync keeps the deletion: the index moves to the holding
// commit and the file stays absent.
func TestS134Submit_SlotSyncKeepsUnstagedDeletion(t *testing.T) {
	f, id, lease, subject, want := slotPublishFixture(t)
	tree, data := existingVerdictTree(t, f, subject, want)
	work := filepath.Join(tree, filepath.FromSlash(want))
	f.Must(os.Remove(work))
	file := attemptVerdict(t, f, "R1", data)
	if _, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file}); err != nil {
		t.Fatalf("submit refused: %v", err)
	}
	if _, err := os.Lstat(work); !os.IsNotExist(err) {
		t.Fatalf("sync recreated the deleted verdict file: %v; status %q", err, actorCommand(t, f, "-C", tree, "status", "--porcelain"))
	}
	if staged := actorCommand(t, f, "-C", tree, "show", ":"+want); staged != data {
		t.Fatalf("index at the verdict path is %q, want the published bytes", staged)
	}
	// The output is trimmed: " D path" reads "D path"; a staged deletion reads
	// "D  path".
	if status := actorCommand(t, f, "-C", tree, "status", "--porcelain", "--", want); status != "D "+want {
		t.Fatalf("status of the verdict path is %q, want an unstaged deletion", status)
	}
}

// A deletion of the unchanged verdict file just before the git update starts
// gets the published bytes back, as git checkout does: the two-tree update
// writes a file whose old bytes are gone. No user bytes are lost.
func TestS134Submit_SlotSyncConcurrentDeletionGetsPublishedBytes(t *testing.T) {
	f, id, lease, subject, want := slotPublishFixture(t)
	tree, data := existingVerdictTree(t, f, subject, want)
	work := filepath.Join(tree, filepath.FromSlash(want))
	mark := gitWrapper(t, tree, []string{"read-tree", "checkout-index"}, nil, "mv -- '"+want+"' '"+filepath.Join(t.TempDir(), "moved")+"'")
	file := attemptVerdict(t, f, "R1", data)
	if _, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file}); err != nil {
		t.Fatalf("submit refused: %v", err)
	}
	fired(t, mark)
	if got, err := os.ReadFile(work); err != nil || string(got) != data {
		t.Fatalf("verdict file is %q %v, want the published bytes", got, err)
	}
	if status := actorCommand(t, f, "-C", tree, "status", "--porcelain", "--", want); status != "" {
		t.Fatalf("status of the verdict path is %q, want clean", status)
	}
}

// writeWrapper puts a git script first on PATH. The script runs action once,
// before the first git command in dir that can write the verdict file:
// checkout-index, or read-tree with -u.
func writeWrapper(t *testing.T, dir, action string) string {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	mark := filepath.Join(t.TempDir(), "fired")
	script := "#!/bin/sh\n" +
		"if [ \"$(pwd -P)\" = '" + dir + "' ] && [ ! -e '" + mark + "' ]; then\n" +
		"  case \" $* \" in\n" +
		"  ' checkout-index '*|' read-tree '*' -u '*)\n" +
		"    : > '" + mark + "'\n" +
		"    " + action + " ;;\n" +
		"  esac\n" +
		"fi\n" +
		"exec '" + realGit + "' \"$@\"\n"
	f := filepath.Join(bin, "git")
	if err := os.WriteFile(f, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return mark
}

// A user edits the verdict file just before the git command that writes it.
// The edit survives, and the index holds the published bytes.
func TestS134Submit_SlotSyncKeepsEditAtWrite(t *testing.T) {
	for _, name := range []string{"new-path", "unchanged-verdict"} {
		t.Run(name, func(t *testing.T) {
			f, id, lease, subject, want := slotPublishFixture(t)
			data := verdictBytes(subject, "R1")
			var tree string
			if name == "new-path" {
				tree = specWorktree(t, f)
			} else {
				tree, data = existingVerdictTree(t, f, subject, want)
			}
			action := "mkdir -p \"$(dirname '" + want + "')\" && printf 'user edit at write' > '" + want + "'"
			mark := writeWrapper(t, tree, action)
			file := attemptVerdict(t, f, "R1", data)
			if _, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file}); err != nil {
				t.Fatalf("submit refused: %v", err)
			}
			fired(t, mark)
			if work, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(want))); err != nil || string(work) != "user edit at write" {
				t.Fatalf("sync overwrote the edit made before its write: %q %v", work, err)
			}
			if staged := actorCommand(t, f, "-C", tree, "show", ":"+want); staged != data {
				t.Fatalf("index at the verdict path is %q, want the published bytes", staged)
			}
		})
	}
}

// A tracked verdict file that the user did not change moves to the published
// bytes, and the worktree stays clean.
func TestS134Submit_SlotSyncUpdatesUnchangedVerdict(t *testing.T) {
	f, id, lease, subject, want := slotPublishFixture(t)
	tree, data := existingVerdictTree(t, f, subject, want)
	file := attemptVerdict(t, f, "R1", data)
	if _, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file}); err != nil {
		t.Fatalf("submit refused: %v", err)
	}
	if work, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(want))); err != nil || string(work) != data {
		t.Fatalf("sync did not update the unchanged verdict file: %q %v", work, err)
	}
	if status := actorCommand(t, f, "-C", tree, "status", "--porcelain"); status != "" {
		t.Fatalf("sync left the spec worktree out of step:\n%s", status)
	}
}

// A linked worktree on the spec branch whose directory is gone (git lists it
// as prunable) does not stop the publish. The first submit closes the slot,
// and a retry with the same bytes writes nothing.
func TestS134Submit_SlotSyncSkipsMissingWorktree(t *testing.T) {
	f, id, lease, subject, want := slotPublishFixture(t)
	tree := specWorktree(t, f)
	f.Must(os.Rename(tree, filepath.Join(t.TempDir(), "moved")))
	if list := actorCommand(t, f, "worktree", "list", "--porcelain"); !strings.Contains(list, "\nprunable") {
		t.Fatalf("fixture did not make the worktree prunable:\n%s", list)
	}
	file := attemptVerdict(t, f, "R1", verdictBytes(subject, "R1"))
	if _, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file}); err != nil {
		t.Fatalf("a missing spec worktree stopped the submit: %v", err)
	}
	if status := actorRecord(t, f, id).Status; status != "closed" {
		t.Fatalf("slot status %q, want closed", status)
	}
	receipt := actorReceipt(t, f, id)
	if held := actorCommand(t, f, "show", receipt.Commit+":"+want); held != verdictBytes(subject, "R1") {
		t.Fatalf("holding commit bytes %q", held)
	}
	before := gitObjects(t, f)
	if _, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file}); err != nil {
		t.Fatalf("a second submit with the same verdict was refused: %v", err)
	}
	if status := actorRecord(t, f, id).Status; status != "closed" {
		t.Fatalf("slot status %q after the second submit, want closed", status)
	}
	again, err := kernel.PublishSlotVerdictForTest(f.State(), id, file)
	if err != nil {
		t.Fatalf("a retry was refused: %v", err)
	}
	if again != receipt.Commit {
		t.Fatalf("retry gave %s, want the first holding commit %s", again, receipt.Commit)
	}
	if after := gitObjects(t, f); after != before {
		t.Fatalf("retry wrote a ref or commit:\nbefore %s\nafter %s", before, after)
	}
}

// fakeSigner writes a gpg.program that logs each call. A failing signer exits
// non-zero; a working one prints a signature that git accepts.
func fakeSigner(t *testing.T, works bool) (program, log string) {
	t.Helper()
	dir := t.TempDir()
	log = filepath.Join(dir, "calls")
	program = filepath.Join(dir, "gpg")
	script := "#!/bin/sh\necho \"$@\" >> '" + log + "'\ncat > /dev/null\n"
	if works {
		script += "printf '\\n[GNUPG:] SIG_CREATED D 1 8 00 1 FAKE\\n' >&2\n" +
			"printf -- '-----BEGIN PGP SIGNATURE-----\\n\\nZmFrZQ==\\n-----END PGP SIGNATURE-----\\n'\n"
	} else {
		script += "echo 'no signing key' >&2\nexit 1\n"
	}
	if err := os.WriteFile(program, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return program, log
}

// With commit.gpgsign on, the verdict commit is signed with the operator's
// identity, as git commit does; a failing signer refuses the publish and the
// spec branch does not move.
func TestS134Submit_SlotVerdictCommitFollowsSigning(t *testing.T) {
	t.Run("signed", func(t *testing.T) {
		f, id, _, subject, _ := slotPublishFixture(t)
		program, log := fakeSigner(t, true)
		for _, kv := range [][2]string{{"user.name", "Operator Name"}, {"user.email", "operator@example.invalid"}, {"gpg.program", program}, {"commit.gpgsign", "true"}} {
			actorCommand(t, f, "config", kv[0], kv[1])
		}
		file := attemptVerdict(t, f, "R1", verdictBytes(subject, "R1"))
		commit, err := kernel.PublishSlotVerdictForTest(f.State(), id, file)
		f.Must(err)
		if calls, err := os.ReadFile(log); err != nil || len(calls) == 0 {
			t.Fatalf("the signing program was not called: %v", err)
		}
		raw := actorCommand(t, f, "cat-file", "commit", commit)
		header, _, _ := strings.Cut(raw, "\n\n")
		if !strings.Contains(header, "\ngpgsig ") {
			t.Fatalf("verdict commit has no gpgsig header:\n%s", header)
		}
		if author := actorCommand(t, f, "log", "-1", "--format=%an <%ae>|%cn <%ce>", commit); author != "Operator Name <operator@example.invalid>|Operator Name <operator@example.invalid>" {
			t.Fatalf("verdict commit identity %q, want the operator's", author)
		}
	})
	t.Run("signer-fails", func(t *testing.T) {
		f, id, _, subject, _ := slotPublishFixture(t)
		program, log := fakeSigner(t, false)
		for _, kv := range [][2]string{{"user.name", "Operator Name"}, {"user.email", "operator@example.invalid"}, {"gpg.program", program}, {"commit.gpgsign", "true"}} {
			actorCommand(t, f, "config", kv[0], kv[1])
		}
		branch := "refs/heads/spec/" + f.SpecID
		tip := actorCommand(t, f, "rev-parse", branch)
		file := attemptVerdict(t, f, "R1", verdictBytes(subject, "R1"))
		_, err := kernel.PublishSlotVerdictForTest(f.State(), id, file)
		assertOneRecovery(t, err, "commit signing")
		if calls, e := os.ReadFile(log); e != nil || len(calls) == 0 {
			t.Fatalf("the signing program was not called: %v", e)
		}
		if now := actorCommand(t, f, "rev-parse", branch); now != tip {
			t.Fatalf("a failed signature moved the spec branch to %s", now)
		}
	})
}

// The first submit publishes and syncs a new verdict, and the receipt write
// then fails. The user deletes the published file. The retry of the same
// submit keeps the deletion, because the worktree index already tracks the
// path, and the slot closes.
func TestS134Submit_SlotRetryKeepsDeletionAfterFirstSync(t *testing.T) {
	f, id, lease, subject, want := slotPublishFixture(t)
	tree := specWorktree(t, f)
	file := attemptVerdict(t, f, "R1", verdictBytes(subject, "R1"))
	realRun := f.Store.Run
	failed := false
	f.Store.Run = func(args ...string) ([]byte, error) {
		for i, arg := range args {
			if arg != "--metadata" || i+1 >= len(args) {
				continue
			}
			data, err := os.ReadFile(strings.TrimPrefix(args[i+1], "@"))
			var fields map[string]json.RawMessage
			if err == nil && json.Unmarshal(data, &fields) == nil && fields[kernel.KeyClose] != nil {
				failed = true
				return nil, errors.New("receipt write failed")
			}
		}
		return realRun(args...)
	}
	_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file})
	f.Store.Run = realRun
	if err == nil || !failed {
		t.Fatalf("the receipt write did not fail, so the test proves nothing: %v", err)
	}
	work := filepath.Join(tree, filepath.FromSlash(want))
	if got, e := os.ReadFile(work); e != nil || string(got) != verdictBytes(subject, "R1") {
		t.Fatalf("first submit did not sync the verdict file: %q %v", got, e)
	}
	f.Must(os.Remove(work))
	if status := actorCommand(t, f, "-C", tree, "status", "--porcelain", "--", want); status != "D "+want {
		t.Fatalf("status before the retry is %q, want an unstaged deletion", status)
	}
	if _, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file}); err != nil {
		t.Fatalf("the retry was refused: %v", err)
	}
	if _, err := os.Lstat(work); !os.IsNotExist(err) {
		t.Fatalf("the retry recreated the deleted verdict file: %v; status %q", err, actorCommand(t, f, "-C", tree, "status", "--porcelain", "--", want))
	}
	if status := actorCommand(t, f, "-C", tree, "status", "--porcelain", "--", want); status != "D "+want {
		t.Fatalf("status after the retry is %q, want the unstaged deletion kept", status)
	}
	if status := actorRecord(t, f, id).Status; status != "closed" {
		t.Fatalf("slot status %q, want closed", status)
	}
}

// An index lock stops the sync after the spec branch moved, so the worktree
// shows a staged deletion of the verdict. The route that the binary prints
// next finishes the sync: after a raw close it is a repair with the
// registered path, and after a full close it is a replay of the same submit.
// No staged deletion stays, so a later ordinary commit keeps the verdict.
func TestS134Submit_SlotRepairRoutesFinishSync(t *testing.T) {
	for _, route := range []string{"repair-after-raw-close", "closed-receipt-replay"} {
		t.Run(route, func(t *testing.T) {
			f, id, lease, subject, want := slotPublishFixture(t)
			tree := specWorktree(t, f)
			data := verdictBytes(subject, "R1")
			file := attemptVerdict(t, f, "R1", data)
			work := filepath.Join(tree, filepath.FromSlash(want))
			if route == "repair-after-raw-close" {
				lock := actorCommand(t, f, "-C", tree, "rev-parse", "--path-format=absolute", "--git-path", "index.lock")
				f.Must(os.WriteFile(lock, []byte("held by another git command"), 0o600))
				_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file})
				assertOneRecovery(t, err, "run the same submit command again")
				f.Must(os.Rename(lock, filepath.Join(t.TempDir(), "index.lock")))
				// The panel verb leaves an untracked copy of the verdict at
				// the registered path, and a raw close closes the slot with
				// no receipt.
				f.Must(os.MkdirAll(filepath.Dir(work), 0o755))
				f.Must(os.WriteFile(work, []byte(data), 0o644))
				changeActorMetadata(t, f, id, "closed")
			} else {
				f.Must(func() error {
					_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file})
					return err
				}())
				// A crash of a later git command puts the index back at the
				// parent of the holding commit; the file stays.
				actorCommand(t, f, "-C", tree, "read-tree", "HEAD~1")
			}
			if status := actorCommand(t, f, "-C", tree, "status", "--porcelain", "--", want); !strings.HasPrefix(status, "D  "+want) {
				t.Fatalf("fixture shows %q, want a staged deletion of the verdict", status)
			}
			var err error
			if route == "repair-after-raw-close" {
				_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Repair: true, Reason: "raw close", Artifact: work})
			} else {
				_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Artifact: file})
			}
			if err != nil {
				t.Fatalf("the printed route was refused: %v", err)
			}
			if status := actorCommand(t, f, "-C", tree, "status", "--porcelain"); status != "" {
				t.Fatalf("the printed route left the spec worktree out of step:\n%s", status)
			}
			if status := actorRecord(t, f, id).Status; status != "closed" {
				t.Fatalf("slot status %q, want closed", status)
			}
		})
	}
}
