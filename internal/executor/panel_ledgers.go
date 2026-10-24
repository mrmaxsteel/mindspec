package executor

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// panelLedgerPaths resolves ownership once for staging, snapshots and status
// consumers. HEAD evidence preserves deletions without reserving consumer paths.
func panelLedgerPaths(root string) ([]string, error) {
	var headPaths []string
	if _, err := gitutil.RevParseRef(root, "HEAD"); err == nil {
		listing, err := gitutil.LsTree(root, "HEAD")
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(listing, "\n") {
			if line == "" {
				continue
			}
			_, name, ok := strings.Cut(line, "\t")
			if !ok {
				return nil, fmt.Errorf("malformed HEAD tree listing")
			}
			if strings.HasPrefix(name, "\"") {
				name, err = strconv.Unquote(name)
				if err != nil {
					return nil, err
				}
			}
			headPaths = append(headPaths, name)
		}
	} else if !errors.Is(err, gitutil.ErrRefNotFound) {
		return nil, err
	}
	status, err := gitutil.StatusNoRenamesZWithStderr(root)
	if err != nil {
		return nil, err
	}
	entries, err := panel.ParseStatusZ(status)
	if err != nil {
		return nil, err
	}
	var deleted []string
	for _, entry := range entries {
		if entry.Index == 'D' || entry.Worktree == 'D' {
			deleted = append(deleted, entry.Path)
		}
	}
	return panel.ResolveLedgerArtifacts(root, headPaths, deleted), nil
}
