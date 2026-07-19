package instruct

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/workspace/containment"
)

// getwdFn is a package-level variable for testability.
var getwdFn = os.Getwd

// CheckWorktree verifies that the current working directory matches the
// expected active worktree path from state. Returns an informational message
// if CWD doesn't match, or empty string if OK or no worktree is active.
func CheckWorktree(activeWorktree string) string {
	if activeWorktree == "" {
		return ""
	}

	cwd, err := getwdFn()
	if err != nil {
		return fmt.Sprintf("Could not determine working directory: %v", err)
	}

	cwdAbs, _ := filepath.Abs(cwd)
	wtAbs, _ := filepath.Abs(activeWorktree)

	if strings.HasPrefix(cwdAbs, wtAbs) {
		return ""
	}

	return "Switch to worktree to begin work: " + containment.EmitCd(activeWorktree)
}
