package bootstrap

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/safeio"
)

// beadsMergeDriverWrapper is the embedded copy of the tracked
// scripts/bd-jsonl-merge-driver.sh wrapper. go:embed can only reach files
// inside the package tree, so the wrapper is mirrored under assets/ and a
// drift-guard test asserts the two copies stay byte-equal. go:embed stores
// BYTES ONLY (no file mode), so the executable bit MUST be set explicitly on
// write — see provisionBeadsMergeDriver (mindspec-oe0u).
//
//go:embed assets/bd-jsonl-merge-driver.sh
var beadsMergeDriverWrapper []byte

const (
	// beadsMergeDriverScriptRel is the repo-relative path of the wrapper.
	beadsMergeDriverScriptRel = "scripts/bd-jsonl-merge-driver.sh"
	// beadsMergeAttrPattern is the .gitattributes pattern field for the jsonl.
	beadsMergeAttrPattern = ".beads/issues.jsonl"
	// beadsMergeAttrLine is the full .gitattributes mapping line (no newline).
	beadsMergeAttrLine = beadsMergeAttrPattern + " merge=beads"
	// beadsMergeDriverConfig is the PORTABLE repo-relative merge.beads.driver
	// value. A relative path containing '/' is resolved by git (and by
	// doctor's resolveDriverCommand) against the worktree top-level, where git
	// runs merge drivers — so this single shared .git/config value is valid
	// from every linked worktree and every fresh clone. Single-quoted so it
	// round-trips through driverTokens and survives a repo path with spaces.
	beadsMergeDriverConfig = "'" + beadsMergeDriverScriptRel + "' %A %O %B"
)

// provisionBeadsMergeDriver makes a freshly bootstrapped repo merge-driver-safe
// from commit 0 (mindspec-oe0u, ADR-0025: the jsonl is a deterministic
// projection of the Dolt DB, so regenerate-from-DB is the correct merge). It
// is ensure-if-absent (never clobbers a user-authored value) and honors
// dryRun (reports, never writes):
//
//	(a) writes the embedded wrapper to <root>/scripts/bd-jsonl-merge-driver.sh
//	    with the EXECUTABLE bit set (0o755) when absent;
//	(b) appends `.beads/issues.jsonl merge=beads` to <root>/.gitattributes when
//	    that mapping is absent (newline-safe);
//	(c) sets merge.beads.driver to the portable repo-relative value via
//	    `git config` — BEST-EFFORT: skipped (no error) when <root>/.git is
//	    absent, since bootstrap may run before `git init`.
func provisionBeadsMergeDriver(r *Result, root string, dryRun bool) error {
	if err := provisionMergeDriverWrapper(r, root, dryRun); err != nil {
		return err
	}
	if err := provisionGitattributesBeadsMerge(r, root, dryRun); err != nil {
		return err
	}
	provisionMergeDriverConfig(r, root, dryRun)
	return nil
}

// provisionMergeDriverWrapper writes the embedded wrapper EXECUTABLE.
func provisionMergeDriverWrapper(r *Result, root string, dryRun bool) error {
	scriptPath := filepath.Join(root, filepath.FromSlash(beadsMergeDriverScriptRel))
	if fileExists(scriptPath) {
		r.Skipped = append(r.Skipped, beadsMergeDriverScriptRel)
		return nil
	}
	r.Created = append(r.Created, beadsMergeDriverScriptRel)
	if dryRun {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		return fmt.Errorf("creating scripts dir: %w", err)
	}
	// EXEC-BIT (critical): the merge driver is a no-op unless it is
	// executable — resolveDriverCommand's 0o111 gate fails and git silently
	// TEXT-merges the jsonl. WriteFileNoSymlink honors its perm argument
	// (tmp.Chmod(perm), umask-independent), so 0o755 ACTUALLY sets the bit;
	// do NOT route this through the bootstrap manifest's hardcoded-0644
	// helper.
	if err := safeio.WriteFileNoSymlink(scriptPath, beadsMergeDriverWrapper, 0o755); err != nil {
		return fmt.Errorf("writing merge driver wrapper: %w", err)
	}
	return nil
}

// provisionGitattributesBeadsMerge appends the merge=beads mapping newline-safe.
func provisionGitattributesBeadsMerge(r *Result, root string, dryRun bool) error {
	path := filepath.Join(root, ".gitattributes")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading .gitattributes: %w", err)
	}
	if gitattributesHasBeadsMergeBytes(data) {
		r.Skipped = append(r.Skipped, ".gitattributes (merge=beads present)")
		return nil
	}

	// NEWLINE-SAFE: if the existing file is non-empty and lacks a trailing
	// newline, prepend one so the mapping lands on its OWN line — otherwise
	// `*.png binary` + the append would corrupt into
	// `*.png binary.beads/issues.jsonl merge=beads`, a bogus pattern that
	// detection (which requires fields[0] == .beads/issues.jsonl) rejects.
	var suffix strings.Builder
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		suffix.WriteByte('\n')
	}
	suffix.WriteString(beadsMergeAttrLine)
	suffix.WriteByte('\n')

	if len(data) == 0 {
		r.Created = append(r.Created, ".gitattributes")
	} else {
		r.Appended = append(r.Appended, ".gitattributes")
	}
	if dryRun {
		return nil
	}

	if len(data) == 0 {
		if err := safeio.WriteFileNoSymlink(path, []byte(suffix.String()), 0o644); err != nil {
			return fmt.Errorf("writing .gitattributes: %w", err)
		}
		return nil
	}
	f, err := safeio.OpenAppendNoSymlink(path, 0o644)
	if err != nil {
		return fmt.Errorf("appending to .gitattributes: %w", err)
	}
	_, writeErr := f.WriteString(suffix.String())
	closeErr := f.Close()
	if writeErr != nil {
		return fmt.Errorf("writing .gitattributes: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("closing .gitattributes: %w", closeErr)
	}
	return nil
}

// provisionMergeDriverConfig sets merge.beads.driver best-effort. The git
// config write needs a repo; when <root>/.git is absent (bootstrap ran before
// `git init`), it is skipped silently — the wrapper + .gitattributes are still
// provisioned, and a re-run of bootstrap or `mindspec doctor --fix` converges
// the config once the repo exists. It is NOT recorded in Result here so the
// no-.git path adds no spurious "Skipped (already exist)" entry.
func provisionMergeDriverConfig(r *Result, root string, dryRun bool) {
	if !gitRepoPresent(root) {
		// Best-effort: defer cleanly until `git init` runs. Not an error.
		return
	}
	if existing, ok := readGitConfigValue(root, "merge.beads.driver"); ok && strings.TrimSpace(existing) != "" {
		r.Skipped = append(r.Skipped, "merge.beads.driver (already configured)")
		return
	}
	r.Created = append(r.Created, "merge.beads.driver")
	if dryRun {
		return
	}
	if err := setGitConfigValue(root, "merge.beads.driver", beadsMergeDriverConfig); err != nil {
		// Best-effort: surface the failure but never hard-fail bootstrap over
		// a git config write (doctor --fix converges it later).
		r.BeadsConfErr = fmt.Errorf("setting merge.beads.driver: %w", err)
	}
}

// gitRepoPresent reports whether <root> hosts a git repo. .git is a directory
// in a normal repo and a FILE in a linked worktree — both count.
func gitRepoPresent(root string) bool {
	_, err := os.Stat(filepath.Join(root, ".git"))
	return err == nil
}

// gitattributesHasBeadsMergeBytes mirrors doctor.gitattributesHasBeadsMerge:
// the merge=beads attribute mapped to the EXACT .beads/issues.jsonl pattern.
// Kept package-local (no cross-package import) but semantically identical so
// the writer and the doctor detector agree.
func gitattributesHasBeadsMergeBytes(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != beadsMergeAttrPattern {
			continue
		}
		for _, attr := range fields[1:] {
			if attr == "merge=beads" {
				return true
			}
		}
	}
	return false
}

// readGitConfigValue returns a git config value for the repo at root; ok=false
// when the key is unset or git is unavailable.
func readGitConfigValue(root, key string) (string, bool) {
	cmd := exec.Command("git", "config", "--get", key)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// setGitConfigValue sets a git config key for the repo at root. In a linked
// worktree this lands in the shared .git/config, covering main and all
// worktrees at once (so the portable relative value is set once).
func setGitConfigValue(root, key, value string) error {
	cmd := exec.Command("git", "config", key, value)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git config %s: %v: %s", key, err, strings.TrimSpace(string(out)))
	}
	return nil
}
