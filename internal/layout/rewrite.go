package layout

import (
	"os"
	"path/filepath"
	"strings"
)

// RewriteRule is a single literal (Old → New) substring substitution applied
// to markdown content. The link-rewriter (Req 5) is a FINITE-PATTERN rewriter,
// NOT a general markdown parser: it applies a small, closed, ordered list of
// these literal rules to the breaking subset of link forms and leaves
// everything else byte-for-byte unchanged.
type RewriteRule struct {
	Old string
	New string
}

// DefaultFlattenRules is the closed breaking set for the full spec-106 move:
// the symmetric flatten of .mindspec/docs/{specs,adr,domains,core} +
// context-map.md into .mindspec/, AND the ASYMMETRIC dogfood eviction of
// .mindspec/docs/{user,installation,research} OUT to top-level project-docs/.
//
// Each symmetric rule drops the `docs/` segment from an ABSOLUTE
// (.mindspec-rooted) reference — the form repo-root docs (README.md /
// AGENTS.md) and any absolute in-tree reference use to point INTO a moved tree.
// The three dogfood rules rewrite an absolute `.mindspec/docs/<dogfood>/`
// reference to its evicted `project-docs/<dogfood>/` location (a depth change
// OUT of `.mindspec/`). Rules are ordered most-specific-first so the
// context-map.md file rule fires before the directory rules.
//
// Crucially, NO rule touches a `../`-relative target, so SYMMETRIC sibling
// links are PRESERVED unchanged: the spec→ADR links like `../../adr/ADR-NNNN.md`
// (both specs/ and adr/ shed the same `docs/` level) AND the dogfood
// cross-sibling links like `../installation/setup.md` (both user/ and
// installation/ shed `.mindspec/docs/` and gain `project-docs/` at the same
// depth) resolve unchanged — exactly the Req 5 invariant. The review
// co-location move's per-group depth-change rules are GENERATED at run time
// (they depend on each review's resolved owning spec) and appended to this base
// set by the mover, not listed statically here.
func DefaultFlattenRules() []RewriteRule {
	return []RewriteRule{
		{Old: ".mindspec/docs/context-map.md", New: ".mindspec/context-map.md"},
		{Old: ".mindspec/docs/specs/", New: ".mindspec/specs/"},
		{Old: ".mindspec/docs/adr/", New: ".mindspec/adr/"},
		{Old: ".mindspec/docs/domains/", New: ".mindspec/domains/"},
		{Old: ".mindspec/docs/core/", New: ".mindspec/core/"},
		{Old: ".mindspec/docs/user/", New: "project-docs/user/"},
		{Old: ".mindspec/docs/installation/", New: "project-docs/installation/"},
		{Old: ".mindspec/docs/research/", New: "project-docs/research/"},
	}
}

// applyRewriteRules applies every rule, in order, to content and returns the
// result plus whether anything changed. It is idempotent: re-applying the same
// rules to already-rewritten content is a no-op (the Old tokens are gone).
func applyRewriteRules(content string, rules []RewriteRule) (string, bool) {
	out := content
	for _, r := range rules {
		out = strings.ReplaceAll(out, r.Old, r.New)
	}
	return out, out != content
}

// rewriteFile applies the rules to a single markdown file in place. Returns
// true when the file changed.
func rewriteFile(path string, rules []RewriteRule) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	out, changed := applyRewriteRules(string(data), rules)
	if !changed {
		return false, nil
	}
	info, statErr := os.Stat(path)
	mode := os.FileMode(0o644)
	if statErr == nil {
		mode = info.Mode().Perm()
	}
	if err := os.WriteFile(path, []byte(out), mode); err != nil {
		return false, err
	}
	return true, nil
}

// applyRewritesInTree applies the rules to every markdown file under target
// (which may be a directory subtree OR a single file). Returns the absolute
// paths of the files that changed. Idempotent.
func applyRewritesInTree(target string, rules []RewriteRule) ([]string, error) {
	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // already-moved / absent subtree → nothing to do
		}
		return nil, err
	}
	var changed []string
	if !info.IsDir() {
		if strings.EqualFold(filepath.Ext(target), ".md") {
			ok, err := rewriteFile(target, rules)
			if err != nil {
				return nil, err
			}
			if ok {
				changed = append(changed, target)
			}
		}
		return changed, nil
	}
	err = filepath.WalkDir(target, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(d.Name()), ".md") {
			return nil
		}
		ok, err := rewriteFile(path, rules)
		if err != nil {
			return err
		}
		if ok {
			changed = append(changed, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return changed, nil
}

// applyRewritesToFiles applies the rules to a fixed list of absolute file
// paths (the affected repo-root docs), skipping any that are absent.
func applyRewritesToFiles(paths []string, rules []RewriteRule) ([]string, error) {
	var changed []string
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		ok, err := rewriteFile(p, rules)
		if err != nil {
			return nil, err
		}
		if ok {
			changed = append(changed, p)
		}
	}
	return changed, nil
}
