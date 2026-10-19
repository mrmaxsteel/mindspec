package next

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

var fileAtRefTypedFn = gitutil.FileAtRefTyped
var readPlanFileFn = os.ReadFile

// readPlanForSpec prefers the committed spec branch; only typed absence permits fallback.
func readPlanForSpec(root, specID string) (content string, absent bool, notice string, err error) {
	branch, err := workspace.SpecBranch(specID)
	if err != nil {
		return "", false, "", err
	}
	dir, err := workspace.SpecDir(root, specID)
	if err != nil {
		return "", false, "", err
	}
	path := filepath.Join(dir, "plan.md")
	// Blob paths belong to the git tree, independently of which worktree
	// SpecDir selected for the checkout read. Match workspace's layout tiers.
	var committed []byte
	var probeErr error
	for _, tier := range []string{".mindspec/specs", ".mindspec/docs/specs", "docs/specs"} {
		committed, probeErr = fileAtRefTypedFn(root, branch, tier+"/"+specID+"/plan.md")
		if !errors.Is(probeErr, gitutil.ErrPathNotAtRef) {
			break
		}
	}
	if probeErr != nil && !errors.Is(probeErr, gitutil.ErrRefAbsent) && !errors.Is(probeErr, gitutil.ErrPathNotAtRef) {
		return "", false, "", probeErr
	}
	local, localErr := readPlanFileFn(path)
	if localErr != nil && !errors.Is(localErr, os.ErrNotExist) {
		return "", false, "", localErr
	}
	if probeErr == nil {
		content = string(committed)
		if localErr == nil && !bytes.Equal(local, committed) {
			notice = fmt.Sprintf("plan.md in this checkout differs from spec/%s; mapping used: spec/%s", idrender.Spec(specID), idrender.Spec(specID))
		}
	} else if localErr == nil {
		content = string(local)
	} else {
		return "", true, "", nil
	}
	if _, err := validate.ParsePlanFrontmatter(content); err != nil {
		return "", false, "", err
	}
	return content, false, notice, nil
}

// SetPlanProbeForTest replaces only the committed-blob probe, retaining source
// selection, checkout reads, parsing, and classification in command tests.
func SetPlanProbeForTest(fn func(string, string, string) ([]byte, error)) func() {
	old := fileAtRefTypedFn
	fileAtRefTypedFn = fn
	return func() { fileAtRefTypedFn = old }
}
