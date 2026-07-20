package main

// Bug mindspec-wu7t panel round 1 (Group 2) — implApproveTail guidance
// composition. result.Pushed is true whenever MergeStrategy=="pr", so
// without the FinalizeBranch gate the orphaned case printed the old
// "Branch pushed to remote. Create a PR to merge into main" spec-branch
// instruction TOGETHER with the wu7t NOTE — contradictory instructions
// about a dead branch. The two blocks must be mutually exclusive.

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/approve"
	"github.com/mrmaxsteel/mindspec/internal/config"
)

// TestImplApproveTail_FinalizeBranchComposition pins the mutual exclusion:
// the orphan case (FinalizeBranch set) prints ONLY the wu7t NOTE; the
// normal case (FinalizeBranch empty) prints ONLY the spec-branch PR
// instruction.
func TestImplApproveTail_FinalizeBranchComposition(t *testing.T) {
	const (
		specBranchMsg = "Branch pushed to remote. Create a PR to merge into main"
		noteMsg       = "was already merged into main, so the epic-close JSONL export landed on a separate branch"
	)
	tests := []struct {
		name           string
		finalizeBranch string
		wantSpecPRMsg  bool
		wantNote       bool
	}{
		{
			name:           "normal pushed case: spec-branch PR instruction only",
			finalizeBranch: "",
			wantSpecPRMsg:  true,
			wantNote:       false,
		},
		{
			name:           "orphan case: wu7t NOTE only, no dead-branch PR instruction",
			finalizeBranch: "chore/finalize-091-x",
			wantSpecPRMsg:  false,
			wantNote:       true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			// implApproveTail chdirs to root (a temp dir deleted at test
			// end); restore the original cwd so later tests in the
			// package never run from a dead directory (the
			// chdirIntoDoomed discipline).
			origWd, _ := os.Getwd()
			t.Cleanup(func() { _ = os.Chdir(origWd) })

			var stdout, stderr bytes.Buffer
			// AutoOpenFinalizePR false (zero-value cfg): this test pins
			// NOTE composition only, not the spec 121 finalize-PR
			// automation — a zero-value config keeps the automation
			// inert so it never spawns a real gh process here.
			tailErr := implApproveTail(&stdout, &stderr, root, root, "091-x", &config.Config{},
				&approve.ImplResult{
					SpecID:         "091-x",
					SpecBranch:     "spec/091-x",
					CommitCount:    2,
					Pushed:         true,
					FinalizeBranch: tc.finalizeBranch,
				},
				nil,
				func(string) error { return nil })
			if tailErr != nil {
				t.Fatalf("tail returned error on success path: %v", tailErr)
			}

			out := stdout.String()
			if got := strings.Contains(out, specBranchMsg); got != tc.wantSpecPRMsg {
				t.Errorf("spec-branch PR instruction present = %v, want %v; stdout:\n%s", got, tc.wantSpecPRMsg, out)
			}
			if got := strings.Contains(out, noteMsg); got != tc.wantNote {
				t.Errorf("wu7t NOTE present = %v, want %v; stdout:\n%s", got, tc.wantNote, out)
			}
			if tc.wantNote {
				if !strings.Contains(out, "gh pr create --head chore/finalize-091-x") {
					t.Errorf("orphan case must name the chore branch in its PR instruction; stdout:\n%s", out)
				}
				// The dead spec branch must not be offered as a PR head
				// anywhere in the orphan-case output.
				if strings.Contains(out, "gh pr create --head spec/091-x") {
					t.Errorf("orphan case must not instruct a PR from the dead spec branch; stdout:\n%s", out)
				}
			}
		})
	}
}
