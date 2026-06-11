package main

// Spec 092 Bead 4, AC "qxsy unit (Req 5)": the `mindspec next`
// completion guidance is location-agnostic — it must NOT instruct
// cd-into-worktree-then-complete (the removed worktree strands the
// shell, field notes mindspec-qxsy / mindspec-tjat) and MUST state that
// `mindspec complete` may run from the repo root.

import (
	"strings"
	"testing"
)

func TestCompletionGuidance_LocationAgnostic(t *testing.T) {
	out := completionGuidance("mindspec-abc.1")

	// Positive: names the exact command with the bead ID.
	if !strings.Contains(out, "mindspec complete mindspec-abc.1") {
		t.Errorf("guidance should name `mindspec complete <id>`; got:\n%s", out)
	}
	// Positive: states it may run from the repo root.
	if !strings.Contains(out, "repo root") {
		t.Errorf("guidance should state `mindspec complete` runs from the repo root; got:\n%s", out)
	}
	// Positive: warns the bead worktree is removed on success.
	if !strings.Contains(out, "remove the bead worktree") {
		t.Errorf("guidance should state the bead worktree is removed on success; got:\n%s", out)
	}
	// Positive: the bd-close/raw-git prohibition is retained.
	if !strings.Contains(out, "Do NOT use `bd close` or raw git") {
		t.Errorf("guidance should retain the bd close / raw git prohibition; got:\n%s", out)
	}

	// Negative: no cd-then-complete instruction in any phrasing.
	for _, banned := range []string{
		"`cd` into the worktree",
		"cd into the worktree",
		"`cd`",
		"cd ",
	} {
		if strings.Contains(out, banned) {
			t.Errorf("guidance must not instruct cd-into-worktree-then-complete (spec 092 Req 5); found %q in:\n%s", banned, out)
		}
	}
}
