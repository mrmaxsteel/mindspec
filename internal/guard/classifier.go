package guard

import (
	"regexp"
	"strings"
)

// Classifier — spec 127 R5(a).
//
// Req-19 (spec 092) banned exactly one destructive command shape: raw
// `bd update ... --metadata`. This file expands that to a CLOSED,
// REVIEWED set of named token-aware families covering the live discard
// forms inventoried in spec 127's Background. It is deliberately NOT a
// closure claim: "is this text dangerous?" is not decidable from the
// text (spec 127 Non-Goals — three fuller designs, a finite-floor
// closure, a deny-by-default partition, and a prescriptive-form
// discriminator, each fell to empirical attack across rounds 4-6 of
// the spec gate). A command family outside AllFamilies, or a git
// global option outside globalOptionPrefixes/globalOptionStandalone,
// is caught by review under the in-diff extension obligation recorded
// in the ADR-0035 amendment — NOT by this file.
//
// FindFloorMatches operates on TEXT, not on a single parsed shell
// command: its callers are (1) internal/lint's repo-wide convention
// scan, scanning prose/markdown/Go-string-literal content for an
// embedded destructive command regardless of its rendering form —
// fenced code block, inline code span, or bare prose all tokenize
// identically (H-r6-2: for the coding-agent consumer of these files,
// rendering form separates nothing) — and (2) NewDestructiveCommand's
// own validation that its operand is actually a floor match.

// DestructiveFamily identifies one member of the closed floor. The
// string value is a stable, human-readable family label — used in
// diagnostics and as part of the registries' rationale text, never
// parsed back.
type DestructiveFamily string

const (
	FamilyGitMerge                   DestructiveFamily = "git merge"
	FamilyGitReset                   DestructiveFamily = "git reset"
	FamilyGitRestore                 DestructiveFamily = "git restore"
	FamilyGitCleanForce              DestructiveFamily = "git clean -f"
	FamilyGitBranchDeleteForce       DestructiveFamily = "git branch -D"
	FamilyGitPushForce               DestructiveFamily = "git push --force"
	FamilyGitStashDropClear          DestructiveFamily = "git stash drop/clear"
	FamilyGitWorktreeRemoveForce     DestructiveFamily = "git worktree remove --force"
	FamilyGitCheckoutPathspecDiscard DestructiveFamily = "git checkout -- <path>"
	FamilyGitSwitchDiscardChanges    DestructiveFamily = "git switch --discard-changes"
	FamilyGitCheckoutForce           DestructiveFamily = "git checkout -f"
	FamilyGitUpdateRefDelete         DestructiveFamily = "git update-ref -d"
	FamilyGitTagDelete               DestructiveFamily = "git tag -d"
	FamilyGitPushDelete              DestructiveFamily = "git push --delete"
	FamilyGitReflogExpireOrGCPrune   DestructiveFamily = "git reflog expire / gc --prune"
	FamilyRmRf                       DestructiveFamily = "rm -rf"
	FamilyBdDeleteForce              DestructiveFamily = "bd delete --force"
)

// AllFamilies is the closed, reviewed floor (spec 127 R5(a)). Order is
// documentation only; registries_test.go's DestructiveFamilyCount
// fixture pins the length so a family added here without a
// corresponding AC-9(i) probe is caught at development time (the
// bead-1 exhaustiveness-sentinel pattern, B-r4-3, applied to this
// closed set).
var AllFamilies = []DestructiveFamily{
	FamilyGitMerge,
	FamilyGitReset,
	FamilyGitRestore,
	FamilyGitCleanForce,
	FamilyGitBranchDeleteForce,
	FamilyGitPushForce,
	FamilyGitStashDropClear,
	FamilyGitWorktreeRemoveForce,
	FamilyGitCheckoutPathspecDiscard,
	FamilyGitSwitchDiscardChanges,
	FamilyGitCheckoutForce,
	FamilyGitUpdateRefDelete,
	FamilyGitTagDelete,
	FamilyGitPushDelete,
	FamilyGitReflogExpireOrGCPrune,
	FamilyRmRf,
	FamilyBdDeleteForce,
}

// DestructiveFamilyCount is AllFamilies' length, exported so tests can
// assert their probe tables stay in lockstep with the floor (the
// bead-1 count-sentinel pattern, B-r4-3).
const DestructiveFamilyCount = 17

// Match is one occurrence of a floor family found in scanned text.
type Match struct {
	// Family is the matched floor family.
	Family DestructiveFamily
	// Text is the exact matched token span, whitespace-normalized to a
	// single space between tokens (markdown decoration — backticks,
	// `**`, fences — is never part of Text: the tokenizer treats it as
	// a separator, per the design principle that rendering form
	// separates nothing, H-r6-2).
	Text string
}

// tokenRe extracts maximal runs of non-separator characters. The
// separator set is deliberately narrow — whitespace plus the markdown/
// prose decoration characters that never appear WITHIN a real git/bd/
// rm token (backtick, `*`, parens, comma, semicolon, quotes, pipe) —
// so flag tokens carrying `-`, `:`, `/`, `=`, `<`, `>`, `.` survive
// intact (`--force-with-lease`, `--git-dir=/x`, `:<ref>`,
// `bead/<id>`).
var tokenRe = regexp.MustCompile("[^\\s`*(),;\"'|]+")

// tokenize splits text into a flat token stream, stripping a single
// trailing '.' from any token (a sentence-ending period is common
// after a bare command in prose, e.g. "run `git gc --prune`.").
func tokenize(text string) []string {
	raw := tokenRe.FindAllString(text, -1)
	out := make([]string, 0, len(raw))
	for _, tok := range raw {
		tok = strings.TrimSuffix(tok, ".")
		if tok == "" {
			continue
		}
		out = append(out, tok)
	}
	return out
}

// globalOptionPrefixes is the pinned, closed set of git global-option
// spellings that take their operand FUSED into the same token via
// `=` (spec 127 H-r6-8). A global option outside this set is the
// stated finite-set limitation (R5(a)), not a bug here.
var globalOptionPrefixes = []string{
	"--git-dir=",
	"--work-tree=",
	"--exec-path=",
}

// globalOptionStandalone take no operand.
var globalOptionStandalone = map[string]bool{
	"-p":         true,
	"--paginate": true,
	"--no-pager": true,
}

// globalOptionWithOperand take their operand as the FOLLOWING token
// (`git -C <path> reset --hard`, this repo's own house style for
// operating on another worktree — H-r6-8's motivating, non-adversarial
// case).
var globalOptionWithOperand = map[string]bool{
	"-C": true,
	"-c": true,
}

// skipGlobalOptions returns the index of the git subcommand token,
// having stripped every contiguous global option (and its operand)
// from the pinned, closed normalization set starting right after
// "git". Returns len(tokens) if the stream runs out first (no
// subcommand present — not a match).
func skipGlobalOptions(tokens []string) int {
	i := 1 // tokens[0] == "git"
	for i < len(tokens) {
		t := tokens[i]
		switch {
		case globalOptionWithOperand[t]:
			i += 2
		case globalOptionStandalone[t]:
			i++
		default:
			matched := false
			for _, p := range globalOptionPrefixes {
				if strings.HasPrefix(t, p) {
					matched = true
					break
				}
			}
			if !matched {
				return i
			}
			i++
		}
	}
	return i
}

// argWindow bounds how many tokens past the subcommand a family
// matcher will inspect for a qualifying flag — generous enough for
// every real-world invocation in the Background inventory (the
// longest, `git push --no-ff -m "Merge bead/<id>" bead/<id>`-shaped
// forms, needs at most 4), bounded so one destructive trigger word
// deep in an unrelated later sentence can never be folded into an
// earlier, unrelated git invocation.
const argWindow = 8

// clusterHasFlag reports whether tok is either the exact long flag, or
// a single-dash short-option cluster containing letter (option-
// cluster equivalence, spec 127 R5(a): `git clean -df`/`-fdx`,
// `rm -fr`/`-Rf` match their family same as the un-clustered spelling).
func clusterHasFlag(tok, long string, letter byte) bool {
	if tok == long {
		return true
	}
	if len(tok) < 2 || tok[0] != '-' || tok[1] == '-' {
		return false
	}
	for i := 1; i < len(tok); i++ {
		if tok[i] == letter {
			return true
		}
	}
	return false
}

// findArg scans rest (bounded by argWindow) for a token satisfying
// pred, returning its index (relative to rest) or -1.
func findArg(rest []string, pred func(string) bool) int {
	n := len(rest)
	if n > argWindow {
		n = argWindow
	}
	for i := 0; i < n; i++ {
		if pred(rest[i]) {
			return i
		}
	}
	return -1
}

// matchGit attempts every git family against tokens (tokens[0] ==
// "git"). Returns the matched family and the total token count
// consumed (including "git", any stripped globals, and the
// subcommand), or ok=false.
func matchGit(tokens []string) (DestructiveFamily, int, bool) {
	subIdx := skipGlobalOptions(tokens)
	if subIdx >= len(tokens) {
		return "", 0, false
	}
	sub := tokens[subIdx]
	rest := tokens[subIdx+1:]
	base := subIdx + 1 // tokens consumed through the subcommand

	switch sub {
	case "merge":
		// Merge-STARTING forms only. `git merge --abort` restores the
		// pre-merge state (recorded floor-level exclusion, Background
		// :1641) — excluded whenever --abort is the merge's own
		// argument, not some unrelated later token.
		if idx := findArg(rest, func(t string) bool { return t == "--abort" }); idx == 0 {
			return "", 0, false
		}
		return FamilyGitMerge, base, true
	case "reset":
		return FamilyGitReset, base, true
	case "restore":
		return FamilyGitRestore, base, true
	case "clean":
		if idx := findArg(rest, func(t string) bool { return clusterHasFlag(t, "--force", 'f') }); idx >= 0 {
			return FamilyGitCleanForce, base + idx + 1, true
		}
	case "branch":
		if idx := findArg(rest, func(t string) bool { return clusterHasFlag(t, "-D", 'D') }); idx >= 0 {
			return FamilyGitBranchDeleteForce, base + idx + 1, true
		}
	case "push":
		// Exact-token equality on "--force"/"-f" naturally excludes
		// the lease-guarded forms `--force-with-lease` /
		// `--force-if-includes` (distinct tokens) — floor-level
		// exclusion by construction, not by a discriminator (F3-r2-2).
		if idx := findArg(rest, func(t string) bool { return t == "--force" || t == "-f" }); idx >= 0 {
			return FamilyGitPushForce, base + idx + 1, true
		}
		if idx := findArg(rest, func(t string) bool { return t == "--delete" }); idx >= 0 {
			return FamilyGitPushDelete, base + idx + 1, true
		}
		// Refspec-deletion spelling: `git push <remote> :<ref>`.
		if idx := findArg(rest, func(t string) bool {
			return strings.HasPrefix(t, ":") && len(t) > 1
		}); idx >= 0 {
			return FamilyGitPushDelete, base + idx + 1, true
		}
	case "stash":
		if idx := findArg(rest, func(t string) bool { return t == "drop" || t == "clear" }); idx >= 0 {
			return FamilyGitStashDropClear, base + idx + 1, true
		}
	case "worktree":
		if len(rest) > 0 && rest[0] == "remove" {
			if idx := findArg(rest[1:], func(t string) bool { return clusterHasFlag(t, "--force", 'f') }); idx >= 0 {
				return FamilyGitWorktreeRemoveForce, base + 1 + idx + 1, true
			}
		}
	case "checkout":
		if idx := findArg(rest, func(t string) bool { return t == "--" }); idx >= 0 {
			return FamilyGitCheckoutPathspecDiscard, base + idx + 1, true
		}
		if idx := findArg(rest, func(t string) bool { return t == "-f" || t == "--force" }); idx >= 0 {
			return FamilyGitCheckoutForce, base + idx + 1, true
		}
	case "switch":
		if idx := findArg(rest, func(t string) bool { return t == "--discard-changes" }); idx >= 0 {
			return FamilyGitSwitchDiscardChanges, base + idx + 1, true
		}
	case "update-ref":
		if idx := findArg(rest, func(t string) bool { return clusterHasFlag(t, "--delete", 'd') }); idx >= 0 {
			return FamilyGitUpdateRefDelete, base + idx + 1, true
		}
	case "tag":
		if idx := findArg(rest, func(t string) bool { return clusterHasFlag(t, "--delete", 'd') }); idx >= 0 {
			return FamilyGitTagDelete, base + idx + 1, true
		}
	case "reflog":
		if len(rest) > 0 && rest[0] == "expire" {
			return FamilyGitReflogExpireOrGCPrune, base + 1, true
		}
	case "gc":
		if idx := findArg(rest, func(t string) bool { return strings.HasPrefix(t, "--prune") }); idx >= 0 {
			return FamilyGitReflogExpireOrGCPrune, base + idx + 1, true
		}
	}
	return "", 0, false
}

// matchBd attempts the bd family against tokens (tokens[0] == "bd").
func matchBd(tokens []string) (DestructiveFamily, int, bool) {
	if len(tokens) < 2 || tokens[1] != "delete" {
		return "", 0, false
	}
	rest := tokens[2:]
	if idx := findArg(rest, func(t string) bool { return t == "--force" }); idx >= 0 {
		return FamilyBdDeleteForce, 2 + idx + 1, true
	}
	return "", 0, false
}

// matchRm attempts the coreutils `rm -rf` family against tokens
// (tokens[0] == "rm", the bare program — NEVER `git rm`, whose only
// floor-adjacent form, `git rm --cached`, is a recorded floor-level
// exclusion because it keeps working-tree bytes; `git rm` is simply
// not a named family, so a bare, non---cached `git rm <path>` is
// outside this floor's stated scope, not a silent miss of a claimed
// family).
func matchRm(tokens []string) (DestructiveFamily, int, bool) {
	rest := tokens[1:]
	hasR := findArg(rest, func(t string) bool {
		return clusterHasFlag(t, "--recursive", 'r') || clusterHasFlag(t, "--recursive", 'R')
	}) >= 0
	fIdx := findArg(rest, func(t string) bool { return clusterHasFlag(t, "--force", 'f') })
	if hasR && fIdx >= 0 {
		// A single clustered token (`-rf`, `-fr`, `-Rf`) satisfies
		// both checks at once; report through the flag's own index so
		// Text stays minimal in that common case, and through the
		// later of the two indices when -r/-f are separate tokens.
		rIdx := findArg(rest, func(t string) bool {
			return clusterHasFlag(t, "--recursive", 'r') || clusterHasFlag(t, "--recursive", 'R')
		})
		last := rIdx
		if fIdx > last {
			last = fIdx
		}
		return FamilyRmRf, 1 + last + 1, true
	}
	return "", 0, false
}

// FindFloorMatches scans text for every occurrence of a reviewed floor
// family and returns them in order of appearance, non-overlapping
// (each match consumes its tokens; scanning resumes right after).
func FindFloorMatches(text string) []Match {
	tokens := tokenize(text)
	var out []Match
	i := 0
	for i < len(tokens) {
		switch tokens[i] {
		case "git":
			if fam, n, ok := matchGit(tokens[i:]); ok && n > 0 {
				out = append(out, Match{Family: fam, Text: strings.Join(tokens[i:i+n], " ")})
				i += n
				continue
			}
		case "bd":
			if fam, n, ok := matchBd(tokens[i:]); ok && n > 0 {
				out = append(out, Match{Family: fam, Text: strings.Join(tokens[i:i+n], " ")})
				i += n
				continue
			}
		case "rm":
			if fam, n, ok := matchRm(tokens[i:]); ok && n > 0 {
				out = append(out, Match{Family: fam, Text: strings.Join(tokens[i:i+n], " ")})
				i += n
				continue
			}
		}
		i++
	}
	return out
}

// IsFloorMatch reports whether text contains at least one reviewed
// floor-family match. Used by NewDestructiveCommand (constructor.go)
// to reject non-destructive operands.
func IsFloorMatch(text string) bool {
	return len(FindFloorMatches(text)) > 0
}
