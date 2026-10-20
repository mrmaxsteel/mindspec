package complete

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// SpecLedgerPaths resolves existing flat-layout ledgers in their owning checkout.
// Empty beadID selects every spec ledger, including directories without panel.json.
func SpecLedgerPaths(root, specID, beadID string) (string, []string, error) {
	dir, err := workspace.SpecDir(root, specID)
	if err != nil {
		return "", nil, err
	}
	wt := workspace.TreeRootForSpecDir(dir)
	if wt == "" || filepath.Clean(dir) != filepath.Join(wt, ".mindspec", "specs", specID) {
		return wt, nil, nil
	}
	var candidates []string
	if beadID == "" {
		candidates, err = filepath.Glob(filepath.Join(dir, "reviews", "*", "dispositions.jsonl"))
		if err != nil {
			return wt, nil, err
		}
	} else {
		for _, reg := range panel.ForBead(panel.Scan(dir), beadID) {
			candidates = append(candidates, filepath.Join(reg.Dir, "dispositions.jsonl"))
		}
	}
	var paths []string
	for _, file := range candidates {
		info, err := os.Stat(file)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return wt, nil, err
		}
		if !info.Mode().IsRegular() {
			return wt, nil, fmt.Errorf("panel ledger is not a regular file: %s", termsafe.Escape(file))
		}
		rel, err := filepath.Rel(wt, file)
		if err != nil {
			return wt, nil, err
		}
		paths = append(paths, filepath.ToSlash(rel))
	}
	return wt, paths, nil
}

// ledgerShellArg quotes an operand for a POSIX shell, escaping display controls.
func ledgerShellArg(s string) string {
	return "'" + strings.ReplaceAll(termsafe.Escape(s), "'", "'\"'\"'") + "'"
}

// CheckPanelLedgers is an unconditional preflight, independent of panel hatches.
func CheckPanelLedgers(wt string, paths []string, exec executor.Executor) error {
	for _, file := range paths {
		err := exec.CheckIgnore(wt, file)
		if err == nil {
			return guard.NewFailure(fmt.Sprintf("panel ledger %s is ignored in %s", termsafe.Escape(file), termsafe.Escape(wt)),
				fmt.Sprintf(`(cd %s && mindspec init) && git -C %s commit -m "chore: un-ignore panel ledger" .gitignore`, ledgerShellArg(wt), ledgerShellArg(wt)))
		}
		var exit interface{ ExitCode() int }
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			return guard.NewFailure(fmt.Sprintf("could not check whether panel ledger %s is ignored: %s", termsafe.Escape(file), termsafe.Escape(err.Error())),
				fmt.Sprintf("git -C %s check-ignore -- %s", ledgerShellArg(wt), ledgerShellArg(file)))
		}
	}
	return nil
}

// LedgerCommitRecovery is shared by complete's commit failure and impl approve.
func LedgerCommitRecovery(wt string, paths []string) string {
	quoted := make([]string, len(paths))
	for i, file := range paths {
		quoted[i] = ledgerShellArg(file)
	}
	return fmt.Sprintf(`git -C %s add -- %s && git -C %s commit -m "chore(panel): ledger"`, ledgerShellArg(wt), strings.Join(quoted, " "), ledgerShellArg(wt))
}

func commitPanelLedgers(wt string, paths []string, exec executor.Executor) error {
	if len(paths) == 0 {
		return nil
	}
	if err := exec.CommitPaths(wt, "chore(panel): ledger", paths); err != nil {
		return guard.NewFailure("could not commit panel ledger: "+termsafe.Escape(err.Error()), LedgerCommitRecovery(wt, paths))
	}
	return nil
}
