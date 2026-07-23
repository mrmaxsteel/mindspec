package gitutil

import (
	"os"
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

// EnsureGitignoreEntries ensures each of entries is present as an exact
// line in root/.gitignore. It is entry-granular and idempotent: existing
// bytes — content, order, comments — are NEVER reordered or rewritten; only
// the entries that are actually missing are appended, once, under a single
// shared header comment. If .gitignore does not exist yet, it is created
// containing exactly the header plus the given entries. Calling it when
// every entry is already present is a true no-op (the file is not even
// opened for writing), so repeated calls — including a fresh `mindspec
// init` immediately after a create-from-scratch write that already
// contains these entries — are byte-identical (spec 123 R4a/AC-5,
// R4b/AC-6).
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

	var missing []string
	for _, e := range entries {
		if !present[e] {
			missing = append(missing, e)
		}
	}
	if len(missing) == 0 {
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
	for _, e := range missing {
		b.WriteString(e)
		b.WriteString("\n")
	}

	return os.WriteFile(path, []byte(b.String()), 0o644)
}
