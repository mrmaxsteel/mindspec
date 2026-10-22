// Package mergefollowup builds recovery text for work added after a naming merge.
// It reads no state and executes no commands.
package mergefollowup

import (
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// Commit is a new commit that must survive recovery.
type Commit struct {
	SHA     string
	Subject string
}

// Commands returns steps to close the original work and prepare a follow-up worktree.
func Commands(beadID, epicID string) []string {
	return []string{
		"run a new panel round for the bead at its tip if the latest round did not review that tip",
		"mindspec complete " + idrender.Bead(beadID),
		"bd create \"<title>\" --parent " + idrender.Bead(epicID) + " --deps " + idrender.Bead(beadID),
		"edit plan.md on the spec branch: append the Nth ## Bead heading with a concrete acceptance criterion; add work_chunks id N and key_file_paths; commit plan.md",
		"mindspec next <new bead>",
	}
}

// MovedTipFailure preserves later commits before restoring the original tip.
func MovedTipFailure(beadID, epicID, worktree, parent, tip string, commits []Commit) error {
	var body strings.Builder
	fmt.Fprintf(&body, "bead tip moved from merge parent %s to %s. Put later fixes in a follow-up bead.\n", termsafe.Escape(parent), termsafe.Escape(tip))
	for _, commit := range commits {
		fmt.Fprintf(&body, "commit %s: %s\n", termsafe.Escape(commit.SHA), termsafe.Escape(commit.Subject))
	}
	body.WriteString("A panel round at the fix tip does not carry over. The new bead needs its own panel.")
	if worktree == "" {
		worktree = "<bead worktree>"
	}
	quotedWT := "'" + strings.ReplaceAll(termsafe.Escape(worktree), "'", "'\"'\"'") + "'"
	id := idrender.Bead(beadID)
	commands := []string{
		"git branch " + id + "-followup " + termsafe.Escape(tip),
		"git -C " + quotedWT + " reset --keep " + termsafe.Escape(parent),
	}
	commands = append(commands, Commands(beadID, epicID)...)
	commands = append(commands,
		"git -C '<new bead worktree>' cherry-pick "+termsafe.Escape(parent)+".."+id+"-followup",
		"git branch -D "+id+"-followup",
	)
	return guard.NewFailure(body.String(), commands...)
}
