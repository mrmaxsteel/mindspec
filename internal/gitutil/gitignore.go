package gitutil

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RuntimeIgnoreEntries are the MindSpec local runtime files (ADR-0015) that
// must never be tracked by git: the session/focus state `mindspec init` and
// every `mindspec setup <agent>` verb ensures is gitignored (spec 123 R4).
// This is the single canonical list — bootstrap, setup, and doctor all
// consume it rather than each declaring their own copy, so the set cannot
// drift between the writer sides and the doctor detection side.
var RuntimeIgnoreEntries = []string{
	".mindspec/session.json",
	".mindspec/focus",
}

// gitignoreRuntimeHeader is the comment written above any entries
// EnsureGitignoreEntries appends, so a human reading .gitignore later
// understands why the lines are there.
const gitignoreRuntimeHeader = "# MindSpec local runtime files (not version-controlled)"

// EnsureGitignoreEntries ensures each of entries is ACTUALLY ignored by git
// under root — not merely present as an exact line in root/.gitignore. It is
// entry-granular and idempotent: existing bytes — content, order, comments —
// are NEVER reordered or rewritten; only the entries that are actually
// missing, or present-but-defeated by a later negation rule (see below), are
// appended, once, under a single shared header comment. If .gitignore does
// not exist yet, it is created containing exactly the header plus the given
// entries. Calling it when every entry is already present AND actually
// ignored is a true no-op (the file is not even opened for writing), so
// repeated calls — including a fresh `mindspec init` immediately after a
// create-from-scratch write that already contains these entries — are
// byte-identical (spec 123 R4a/AC-5, R4b/AC-6).
//
// This is deliberately more general than the pre-existing
// EnsureGitignoreEntry (singular): that helper is specialized for directory
// entries (it appends a trailing "/" and a fixed "# mindspec worktrees"
// header) and is not reused here — a runtime FILE entry must not gain a
// trailing slash.
func EnsureGitignoreEntries(root string, entries ...string) error {
	if len(entries) == 0 {
		return nil
	}

	path := filepath.Join(root, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	content := string(data)

	// Exact-line presence detection. We compare each existing line to the
	// requested entry WITHOUT trimming semantic whitespace — only the line
	// delimiter is stripped (the "\n" by Split, plus a trailing "\r" for
	// CRLF files). Leading whitespace is significant to git: a line like
	// " .mindspec/session.json" (leading space) is a DIFFERENT pattern git
	// does NOT honor, so treating it as the required entry would leave
	// .gitignore converged-but-unsafe (git check-ignore still misses). Such
	// a line must NOT satisfy presence, so the real entry gets appended
	// (FX-2).
	present := make(map[string]bool, len(entries))
	for _, line := range strings.Split(content, "\n") {
		present[strings.TrimSuffix(line, "\r")] = true
	}

	// toAppend collects entries that need a fresh line written: either the
	// exact line is altogether absent, OR it is present but a LATER
	// negation rule (e.g. "!.mindspec/session.json") un-ignores it anyway
	// (G1 final-review fix). git applies .gitignore patterns in file
	// order with last-match-wins, so line-presence alone is not proof the
	// path is actually ignored — a negation rule appended after the entry
	// (by a human, or by a template) silently defeats it while this
	// function reports "converged". We ask git itself via `git
	// check-ignore` for any entry that IS line-present, and re-append it
	// (a harmless duplicate line) when git disagrees, so the LAST match
	// in the file is always the plain ignore rule and the path is
	// actually ignored again.
	var toAppend []string
	for _, e := range entries {
		if !present[e] {
			toAppend = append(toAppend, e)
			continue
		}
		if ignored, ok := checkIgnored(root, e); ok && !ignored {
			toAppend = append(toAppend, e)
		}
	}
	if len(toAppend) == 0 {
		return nil
	}

	var b strings.Builder
	b.WriteString(content)
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		b.WriteString("\n")
	}
	if len(content) > 0 {
		b.WriteString("\n")
	}
	b.WriteString(gitignoreRuntimeHeader)
	b.WriteString("\n")
	for _, e := range toAppend {
		b.WriteString(e)
		b.WriteString("\n")
	}

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// checkIgnored reports whether git actually treats entry as ignored under
// root, via `git check-ignore --quiet`. ok is false when git could not make
// a determination — most notably when root is not (yet) inside a git
// repository, which the pre-existing EnsureGitignoreEntries unit tests
// exercise deliberately against a plain (non-git) tempdir — so callers can
// fall back to line-presence alone instead of misreading an indeterminate
// result as "not ignored" and forcing a spurious re-append. `git
// check-ignore` exits 0 when the path is ignored, 1 when it plainly is not,
// and a non-{0,1} status (e.g. 128, "not a git repository") for every other
// failure mode; only the first two are conclusive.
func checkIgnored(root, entry string) (ignored, ok bool) {
	cmd := execCommand("git", gitArgs(root, "check-ignore", "--quiet", "--", entry)...)
	err := cmd.Run()
	if err == nil {
		return true, true
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, true
	}
	return false, false
}
