package doctor

import (
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
)

// checkGit runs git-related health checks.
func checkGit(r *Report, root string) {
	checkRuntimeFilesTracked(r, root)
}

// runtimeFiles lists MindSpec local runtime files that should be gitignored.
// Sourced from gitutil.RuntimeIgnoreEntries — the single canonical list
// bootstrap, setup, and doctor all share, so the set of files this check
// protects can never drift from the set init/setup actually gitignore.
var runtimeFiles = gitutil.RuntimeIgnoreEntries

// checkRuntimeFilesTracked detects MindSpec runtime files tracked by git,
// and (spec 123 R4c) proactively checks ignore-ness for files that are NOT
// yet tracked: an untracked-but-unignored runtime file is one `git add
// .mindspec/` away from becoming the tracked-file Error below, so it gets
// its own Warn rather than a silent OK.
func checkRuntimeFilesTracked(r *Report, root string) {
	for _, file := range runtimeFiles {
		if err := gitutil.LsFilesErrorUnmatch(root, file); err != nil {
			// Not tracked. Still needs to be genuinely gitignored — the
			// pre-accident state this check exists to catch is exactly
			// "not tracked YET" with no ignore rule in place.
			if ignoreErr := gitutil.CheckIgnore(root, file); ignoreErr == nil {
				r.Checks = append(r.Checks, Check{
					Name:    file + " git tracking",
					Status:  OK,
					Message: "not tracked by git",
				})
				continue
			}

			entry := file
			r.Checks = append(r.Checks, Check{
				Name:   file + " git tracking",
				Status: Warn,
				Message: "runtime file not gitignored — one `git add .mindspec/` from being committed " +
					"(run with --fix to add the .gitignore entry)",
				FixFunc: func() error {
					return gitutil.EnsureGitignoreEntries(root, entry)
				},
			})
			continue
		}

		// Still tracked — needs fix. Takes precedence over the ignore-ness
		// Warn above: a tracked file is the worse, already-happened state.
		trackedFile := file
		r.Checks = append(r.Checks, Check{
			Name:    file + " git tracking",
			Status:  Error,
			Message: "tracked by git — runtime file should be gitignored (run with --fix to auto-repair)",
			FixFunc: func() error {
				return untrackRuntimeFile(root, trackedFile)
			},
		})
	}
}

// untrackRuntimeFile removes a runtime file from git index and ensures
// it is listed in .gitignore.
func untrackRuntimeFile(root, file string) error {
	// git rm --cached (keeps file on disk)
	if err := gitutil.RmCached(root, file); err != nil {
		return &fixError{op: "git rm --cached " + file, detail: strings.TrimPrefix(err.Error(), "rm --cached: ")}
	}

	// Ensure .gitignore has the entry
	if err := gitutil.CheckIgnore(root, file); err == nil {
		return nil // already gitignored
	}

	// Append to .gitignore via the shared entry-granular helper (spec 123
	// R4c) — the same one bootstrap/setup use, so doctor's --fix and the
	// scaffolding verbs can never disagree about how an entry is appended.
	return gitutil.EnsureGitignoreEntries(root, file)
}

type fixError struct {
	op     string
	detail string
}

func (e *fixError) Error() string {
	if e.detail != "" {
		return e.op + ": " + e.detail
	}
	return e.op + " failed"
}
