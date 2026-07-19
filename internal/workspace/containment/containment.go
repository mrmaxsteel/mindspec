// Package containment implements spec 120's R5: the agent-writable config
// `worktree_root` ingress predicate, symlink-aware containment for every
// composed worktree path, and the single shell-safe `cd` emitter every
// executable-`cd` render in the codebase routes through. See
// ADR-0042-render-derivation-provenance.md §4 for the decision record.
//
// Package layout note (single-home requirement, AC-10/AC-12): the natural
// home for this predicate is internal/workspace (OQ1 — "core, where paths
// compose"), but internal/workspace itself depends on internal/config (for
// *config.Config access in WorktreesDir/SpecWorktreePath/BeadWorktreePath),
// while the worktree_root ingress check must run INSIDE internal/config
// (config.go, where the ".worktrees" default is backfilled). config ->
// workspace would therefore close a two-package import cycle
// (workspace -> config -> workspace). This package is instead a stdlib-only
// leaf NESTED under internal/workspace/ — it imports nothing from this
// module except internal/termsafe (itself a leaf) — so BOTH internal/config
// and internal/workspace (plus every higher consumer: internal/guard,
// internal/executor, internal/gitutil, cmd/mindspec, ...) can import it
// without a cycle, while there remains exactly ONE non-test implementation
// of the predicate and ONE of the emitter. This mirrors the precedent
// internal/idvalidate set for its idrender sub-package (OQ5): "sub-package
// or sibling stdlib-only leaf" is the established repo pattern for exactly
// this shape of import-graph constraint.
package containment

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// RejectionLever is the single convergent recovery lever every
// worktree_root rejection names (Zero Framework Cognition / ADR-0035):
// one concrete instruction that changes or routes around the offending
// config state, never embedding the hostile value itself. Callers that
// can import internal/guard should prefer
// guard.NewFailure(msg, containment.RejectionLever) so the recovery line
// is built (and validated) by the guard package's own formatter;
// internal/config cannot import internal/guard (the same cycle this
// package's doc comment describes), so it assembles the "recovery: "
// line locally using this constant — see ValidateWorktreeRoot's caller in
// config.go.
const RejectionLever = "set worktree_root to .worktrees (the default) in .mindspec/config.yaml, then re-run"

// isSafeSegmentByte reports whether r is in the conservative charset a
// worktree_root path segment may use: ASCII letters, digits, '.', '-',
// '_'. This is also, not coincidentally, the POSIX Portable Filename
// Character Set (minus '/', which is the segment separator, handled
// separately by ValidateWorktreeRoot/ShellSafe).
func isSafeSegmentByte(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		r == '.' || r == '-' || r == '_'
}

// ValidateWorktreeRoot is the worktree_root ingress predicate (ADR-0042
// §4, AC-10): raw must be a RELATIVE path (no leading '/'), every '/'-
// separated segment non-empty and drawn from the conservative charset
// (letters, digits, '.', '-', '_'), and no segment may be exactly "..".
//
// This is a PURELY LEXICAL check — it never touches the filesystem and
// proves NOTHING about symlinks (filepath.Rel/lexical checks are
// explicitly named lexical-only per the spec: a charset-clean value can
// still resolve, via a symlinked ancestor, to a location outside the
// project root). Symlink-aware physical containment is CheckContainment,
// below, applied separately at config ingress time is NOT this
// function's job — see the plan's ingress/containment/check-at-use
// three-way split. Call this at config ingress (internal/config's
// backfill site) and nowhere else — there is exactly one non-test
// implementation of this predicate (AC-10/AC-22).
//
// termsafe.Escape is deliberately NOT sufficient here (this is the
// escaping-is-insufficient proof AC-10 pins): termsafe.Escape is the
// identity on printable ASCII, so a hostile-but-printable value like
// ".worktrees && echo INJECTED #" survives it byte-identically. The
// charset gate below is what actually rejects it; termsafe.Escape is
// used only when quoting the (already-rejected) raw value INTO this
// function's own error text, so a control byte or embedded newline in a
// malformed worktree_root cannot forge a fake terminal line in the
// refusal message itself (R4).
func ValidateWorktreeRoot(raw string) error {
	if raw == "" {
		return fmt.Errorf("worktree_root must not be empty")
	}
	if strings.HasPrefix(raw, "/") || filepath.IsAbs(raw) {
		return fmt.Errorf("worktree_root %s: must be a relative path (no leading '/')", termsafe.Escape(raw))
	}
	segments := strings.Split(raw, "/")
	for _, seg := range segments {
		if seg == "" {
			return fmt.Errorf("worktree_root %s: contains an empty path segment", termsafe.Escape(raw))
		}
		if seg == ".." {
			return fmt.Errorf("worktree_root %s: '..' path segment is not allowed", termsafe.Escape(raw))
		}
		for _, r := range seg {
			if !isSafeSegmentByte(r) {
				return fmt.Errorf("worktree_root %s: segment %s contains a disallowed character (allowed: letters, digits, '.', '-', '_')", termsafe.Escape(raw), termsafe.Escape(seg))
			}
		}
	}
	return nil
}

// lexicalRelContained reports whether composed is lexically within root,
// using ONLY filepath.Rel + filepath.Clean — no filesystem access, no
// symlink resolution.
//
// NAMED LEXICAL-ONLY (ADR-0042 §4): this function proves NOTHING about
// symlinks. A charset-clean, lexically-contained composed path can still
// have a symlinked ancestor that resolves, physically, to a location
// outside root — that is exactly the AC-10 discriminator
// (TestWorktreeRootPredicate's symlinked-ancestor fixture): this
// function alone would ACCEPT it, while CheckContainment's physical
// EvalSymlinks layer REJECTS it. It is retained as one named
// defense-in-depth layer inside CheckContainment (a cheap, filesystem-
// free rejection of an obvious lexical `..` escape before paying for
// symlink resolution), never as a substitute for the physical check.
func lexicalRelContained(root, composed string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(composed))
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// nearestExistingAncestor walks up from path (made absolute first) until
// it finds a directory entry that exists (checked via os.Lstat, so a
// symlink itself counts as "existing" — its target is resolved by the
// caller's subsequent EvalSymlinks, never silently followed here).
// Returns the existing ancestor and the (possibly empty) remaining tail
// to re-join after the caller resolves the ancestor's real path. Mirrors
// the config.CanonicalPath / internal/journal.go:164 nearest-existing-
// ancestor precedent cited in the spec's Background (a not-yet-created
// worktree directory must still have its ancestry checked for a
// symlinked component).
func nearestExistingAncestor(path string) (ancestor, tail string, err error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("resolving absolute path: %w", err)
	}
	cur := filepath.Clean(abs)
	var tailParts []string
	for {
		if _, statErr := os.Lstat(cur); statErr == nil {
			return cur, filepath.Join(tailParts...), nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", "", fmt.Errorf("no existing ancestor found")
		}
		tailParts = append([]string{filepath.Base(cur)}, tailParts...)
		cur = parent
	}
}

// CheckContainment is the symlink-aware containment predicate (ADR-0042
// §4, AC-10/AC-11): composed must resolve, physically, to a location
// under the resolved root. root MUST already exist (it is always the
// project root or an already-materialized anchor worktree); composed
// need not exist yet.
//
// It resolves root via filepath.EvalSymlinks, resolves the deepest
// EXISTING ancestor of composed via filepath.EvalSymlinks (the
// nearestExistingAncestor precedent — internal/validate/specid.go:
// SafePath resolves both sides directly, which requires composed to
// already exist; a not-yet-created worktree path does not, hence the
// ancestor-walk variant here), re-joins the non-existent tail, and
// requires the result to sit under the resolved root. Any symlink
// component anywhere in the agent-controlled suffix ancestry is
// therefore caught: EvalSymlinks resolves EVERY component of the path it
// is given, not just the last one.
//
// This is the CHECK-AT-USE gate: call it immediately before EVERY use of
// a composed worktree path — a `git worktree add`/`WorktreeAddDetach`
// invocation, an os.Chdir, or an os.MkdirAll on a path composed from the
// agent-writable worktree_root (AC-11's grep-complete inventory). It does
// NOT claim atomic containment: the window between this check and the
// actual kernel/git operation remains (TOCTOU), an ACCEPTED, honestly
// bounded residual — see the spec's Non-Goals. An adversary able to win
// that race already holds concurrent local write access to the working
// tree, the same capability plane as editing hooks or config directly,
// already outside the threat model.
func CheckContainment(root, composed string) error {
	if !lexicalRelContained(root, composed) {
		return fmt.Errorf("path %s escapes root %s (lexical check)", termsafe.Escape(composed), termsafe.Escape(root))
	}

	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolving root %s: %w", termsafe.Escape(root), err)
	}
	resolvedRoot = filepath.Clean(resolvedRoot)

	ancestor, tail, err := nearestExistingAncestor(composed)
	if err != nil {
		return fmt.Errorf("resolving composed path %s: %w", termsafe.Escape(composed), err)
	}
	resolvedAncestor, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return fmt.Errorf("resolving ancestor of %s: %w", termsafe.Escape(composed), err)
	}
	resolvedAncestor = filepath.Clean(resolvedAncestor)

	resolvedComposed := resolvedAncestor
	if tail != "" {
		resolvedComposed = filepath.Join(resolvedAncestor, tail)
	}

	if resolvedComposed != resolvedRoot &&
		!strings.HasPrefix(resolvedComposed+string(filepath.Separator), resolvedRoot+string(filepath.Separator)) {
		return fmt.Errorf(
			"path %s resolves outside the project root (symlink-aware containment refused) — a symlinked ancestor may have replaced part of the worktree path",
			termsafe.Escape(composed),
		)
	}
	return nil
}

// isUnquotedSafeByte reports whether r may appear unquoted in a shell
// command line: the POSIX Portable Filename Character Set (letters,
// digits, '.', '-', '_') plus '/', the path separator every worktree
// path also carries.
func isUnquotedSafeByte(r rune) bool {
	return isSafeSegmentByte(r) || r == '/'
}

// ShellSafe renders s for safe embedding in a shell command line:
// byte-identical when every byte is unquoted-safe (the POSIX portable
// filename charset plus '/'), otherwise POSIX single-quoted, with any
// embedded single quote escaped via the standard close-quote/backslash-
// quote/reopen-quote technique. This mirrors the representation
// cmd/mindspec/panel.go:shellQuoteTarget uses, but — unlike that
// helper, which ALWAYS quotes — ShellSafe quotes ONLY when needed, so a
// clean path renders exactly as it always has (AC-12: no rendering
// change for the entire existing repo, whose paths are all clean today).
func ShellSafe(s string) string {
	for _, r := range s {
		if !isUnquotedSafeByte(r) {
			return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
		}
	}
	return s
}

// EmitCd renders a `cd <target>` line using ShellSafe's conditional
// quoting. This is the SINGLE exported shell-safe `cd` emitter every
// executable-`cd` render in the codebase routes through (ADR-0042 §4,
// AC-12) — there is exactly one non-test implementation. Root-only sinks
// (a trusted, operator-chosen repo root — e.g. cwdsafety.go's
// emitCdBackNote, directMergeConflictFailure's root-anchored recovery)
// route through this too: they never refuse (root is never subjected to
// CheckContainment), but they DO get the same conditional quoting —
// defense-in-depth against a metacharacter-bearing root, not a
// validation claim (Non-Goals).
func EmitCd(target string) string {
	return "cd " + ShellSafe(target)
}
