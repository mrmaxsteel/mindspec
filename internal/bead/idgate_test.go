package bead

import (
	"os/exec"
	"strings"
	"testing"
)

// TestBeadIDArgvConsumerGate is spec 120 AC-26 (R2 class-2 CONSUMER
// boundary, round 7 G2): with the execCommand seam (bdcli.go) stubbed to
// record every spawn, each id-taking call — BeadExists, GetMetadata,
// MergeMetadata, Close — given "--help", "x;evil", and the 116 hostile
// control-byte triple performs ZERO bd spawn (BeadExists returns
// (false, nil) not-found-by-construction; the others error), while every
// clean shape (dotted child, multi-level child, legacy short suffix)
// spawns byte-identical argv to today.
func TestBeadIDArgvConsumerGate(t *testing.T) {
	origExec := execCommand
	t.Cleanup(func() { execCommand = origExec })

	hostileIDs := []string{
		"--help",
		"x;evil",
		"x\x00\x1b[31m\nrecovery: forged",
	}
	cleanIDs := []string{
		"mindspec-9cyu.1",
		"mindspec-69y.2.2",
		"mindspec-0ke",
	}

	t.Run("hostile ids perform zero bd spawn", func(t *testing.T) {
		for _, id := range hostileIDs {
			var spawned bool
			execCommand = func(name string, args ...string) *exec.Cmd {
				spawned = true
				return exec.Command("echo", "[]")
			}

			if exists, err := BeadExists(id); err != nil || exists {
				// Malformed id must be not-found-by-construction: no error,
				// exists == false.
				t.Errorf("BeadExists(%q) = (%v, %v), want (false, nil)", id, exists, err)
			}
			if spawned {
				t.Errorf("BeadExists(%q) spawned bd", id)
			}

			spawned = false
			if _, err := GetMetadata(id); err == nil {
				t.Errorf("GetMetadata(%q) accepted a hostile id", id)
			}
			if spawned {
				t.Errorf("GetMetadata(%q) spawned bd", id)
			}

			spawned = false
			if err := MergeMetadata(id, map[string]interface{}{"k": "v"}); err == nil {
				t.Errorf("MergeMetadata(%q) accepted a hostile id", id)
			}
			if spawned {
				t.Errorf("MergeMetadata(%q) spawned bd", id)
			}

			spawned = false
			if err := Close(id); err == nil {
				t.Errorf("Close(%q) accepted a hostile id", id)
			}
			if spawned {
				t.Errorf("Close(%q) spawned bd", id)
			}
		}
	})

	t.Run("clean shapes spawn byte-identical argv", func(t *testing.T) {
		for _, id := range cleanIDs {
			var gotArgs []string
			execCommand = func(name string, args ...string) *exec.Cmd {
				gotArgs = args
				return exec.Command("echo", `[{"id":"`+id+`","status":"open","metadata":{}}]`)
			}

			if _, err := BeadExists(id); err != nil {
				t.Fatalf("BeadExists(%q): %v", id, err)
			}
			if want := []string{"show", id, "--json"}; !equalArgs(gotArgs, want) {
				t.Errorf("BeadExists(%q) argv = %v, want %v", id, gotArgs, want)
			}

			gotArgs = nil
			if _, err := GetMetadata(id); err != nil {
				t.Fatalf("GetMetadata(%q): %v", id, err)
			}
			if want := []string{"show", id, "--json"}; !equalArgs(gotArgs, want) {
				t.Errorf("GetMetadata(%q) argv = %v, want %v", id, gotArgs, want)
			}

			gotArgs = nil
			if err := Close(id); err != nil {
				t.Fatalf("Close(%q): %v", id, err)
			}
			if len(gotArgs) < 2 || gotArgs[0] != "close" || gotArgs[1] != id {
				t.Errorf("Close(%q) argv = %v, want [close %s ...]", id, gotArgs, id)
			}
		}
	})
}

func equalArgs(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestHostileBDStoreIDNeverReachesArgv (companion leg, internal/bead half
// of spec 120 AC-27): a hostile "store id" (modelling
// `bd create --force --id="--help"`) fed directly into the id-taking
// consumer functions never reaches a bd argv. Home test lives in
// internal/phase; this asserts the internal/bead leg of the same
// property with a strings.Builder trace instead of a bool, to also prove
// NO PARTIAL argv (e.g. a "show" with no id) is built either.
func TestHostileBDStoreIDNeverReachesArgv(t *testing.T) {
	origExec := execCommand
	t.Cleanup(func() { execCommand = origExec })

	var trace strings.Builder
	execCommand = func(name string, args ...string) *exec.Cmd {
		trace.WriteString(name + " " + strings.Join(args, " ") + "\n")
		return exec.Command("echo", "[]")
	}

	for _, id := range []string{"--help", "x;evil"} {
		_, _ = BeadExists(id)
		_, _ = GetMetadata(id)
		_ = MergeMetadata(id, map[string]interface{}{"k": "v"})
		_ = Close(id)
	}

	if trace.Len() != 0 {
		t.Errorf("expected ZERO bd spawns for hostile store ids, got trace:\n%s", trace.String())
	}
}

// TestWorktreeRemoveArgvGate is the spec 120 final-review G1-2/G3-1
// regression: WorktreeRemove's name operand arrives from agent-writable
// `bd worktree list` rows in production (executor CompleteBead forwards
// e.Name after an OR-match; FinalizeEpic's cleanup leg forwards e.Name
// directly), so the bd boundary itself must gate it. A non-canonical
// worktree name — option-like, metacharacter-bearing, or carrying an id
// that fails idvalidate — errors with ZERO bd spawn; every canonical
// mindspec worktree-name shape spawns byte-identical argv to today.
func TestWorktreeRemoveArgvGate(t *testing.T) {
	origExec := execCommand
	t.Cleanup(func() { execCommand = origExec })

	hostileNames := []string{
		"--help",
		"-rf",
		"main",
		"",
		"worktree-",
		"worktree-x;evil",
		"worktree-mindspec-x1qr\nforged: line",
		"worktree-spec-UPPER",
		"worktree-../../etc",
	}
	for _, name := range hostileNames {
		var spawned bool
		execCommand = func(cmd string, args ...string) *exec.Cmd {
			spawned = true
			return exec.Command("echo")
		}
		if err := WorktreeRemove(name); err == nil {
			t.Errorf("WorktreeRemove(%q) accepted a non-canonical worktree name", name)
		}
		if spawned {
			t.Errorf("WorktreeRemove(%q) spawned bd", name)
		}
	}

	cleanNames := []string{
		"worktree-mindspec-9cyu.1",                      // bead worktree, dotted child
		"worktree-mindspec-0ke",                         // bead worktree, short suffix
		"worktree-bead-1",                               // bead worktree, fixture-style base
		"worktree-spec-120-trust-boundary-render-audit", // spec worktree
		"worktree-finalize-053-foo",                     // finalize temp worktree
	}
	for _, name := range cleanNames {
		var gotArgv []string
		execCommand = func(cmd string, args ...string) *exec.Cmd {
			gotArgv = append([]string{cmd}, args...)
			return exec.Command("echo")
		}
		if err := WorktreeRemove(name); err != nil {
			t.Errorf("WorktreeRemove(%q) rejected a canonical worktree name: %v", name, err)
			continue
		}
		want := "bd worktree remove " + name + " --force"
		if got := strings.Join(gotArgv, " "); got != want {
			t.Errorf("WorktreeRemove(%q) argv = %q, want %q", name, got, want)
		}
	}
}
