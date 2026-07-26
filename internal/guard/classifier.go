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
//
// Recorded floor-level exclusions (each with its own non-match
// fixture in classifier_test.go): `git merge --abort` (restores the
// pre-merge state); `git worktree prune` (removes only stale
// administrative metadata, no working-tree bytes); `git rm --cached`
// (keeps working-tree bytes); `git push --force-with-lease`/
// `--force-if-includes` (lease-guarded, excluded by construction —
// distinct tokens from `--force`). A FIFTH: this floor's program set
// is exactly {git, bd, rm} (matchGit/matchBd/matchRm's own leading
// tokens) — mindspec's OWN destructive verbs (most pointedly
// `mindspec release <id> --force`, cmd/mindspec/release.go's own
// recovery line, which discards uncommitted work by design) match no
// family here and never will while the program set stays closed to
// these three; that destructiveness is caught by review under the
// in-diff extension obligation, exactly like any other off-floor
// program (O2-r2-13). "Zero matches" on a `mindspec`-led line is this
// recorded exclusion, not "checked and safe".

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
// documentation only; it is NOT consulted by matchGit/matchBd/matchRm
// (those switch on literal subcommand/flag tokens directly), so its
// own length agreeing with DestructiveFamilyCount
// (classifier_test.go's TestAllFamilies_CountSentinel) is necessary
// but not sufficient — a new DestructiveFamily constant plus a live
// matcher case could still ship unregistered here. What actually
// catches that at development time is
// family_sentinel_test.go's TestAllFamilies_EveryDeclaredFamilyIsRegistered,
// which walks this file's own AST rather than trusting this list
// (spec 127 bead-2 rework, O2-r2-3; the bead-1 exhaustiveness-sentinel
// pattern, B-r4-3, applied to this closed set).
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

// sentenceEnders is the trailing-punctuation set tokenize treats as a
// hard invocation/clause boundary (O1-r2-3): a token immediately
// followed — in the ORIGINAL text, with no intervening non-separator
// character — by one of these marks ends the sentence/clause it is
// part of. These four survive INTO the raw regex match (they are not
// in tokenRe's exclusion set) and are stripped here, one per token,
// same as the prior single-'.'-only behavior this replaces.
const sentenceEnders = ".:!?"

// clauseBoundarySeparators is the set of tokenRe SEPARATOR characters
// (already excluded from every token, same as whitespace) that ALSO
// carry clause-boundary meaning: a semicolon or comma right after a
// token ends that token's clause exactly as a stripped '.'/':'/'!'/'?'
// does, even though — unlike those four — it never survives into the
// token text itself for tokenize to strip.
const clauseBoundarySeparators = ";,"

// isTokenSeparatorByte reports whether b is one of tokenRe's own
// excluded (separator) characters — whitespace or one of the
// markdown/prose decoration characters tokenRe never lets into a
// token. Used only to walk PAST decorative separators (a backtick
// closing an inline code span, e.g. "main`,") when looking for a
// clause-boundary mark that follows them in the same separator run.
func isTokenSeparatorByte(b byte) bool {
	switch b {
	case '`', '*', '(', ')', ',', ';', '"', '\'', '|':
		return true
	}
	switch b {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	}
	return false
}

// tokenize splits text into a flat token stream plus a same-length
// boundary slice: boundary[i] is true when tokens[i] is followed by a
// clause-ending mark — a trailing '.'/':'/'!'/'?' stripped from the
// token itself, or a ';'/',' found while walking forward through the
// run of pure separator characters between this token and the next
// (this walk is what lets the mark be detected even behind decorative
// punctuation, e.g. the comma in "`git checkout main`, rebase --",
// which sits right after a closing backtick, not right after "main"
// itself). findArg (below) stops scanning at the first boundary — a
// destructive trigger word in a LATER, unrelated clause can never be
// folded into an earlier, unrelated invocation (O1-r2-3) — while an
// invocation's own operand list is scanned to its true end regardless
// of length (F1-r2-1: no arbitrary token-count cap).
func tokenize(text string) ([]string, []bool) {
	idx := tokenRe.FindAllStringIndex(text, -1)
	out := make([]string, 0, len(idx))
	boundary := make([]bool, 0, len(idx))
	for _, span := range idx {
		tok := text[span[0]:span[1]]
		end := false
		if len(tok) > 0 && strings.ContainsRune(sentenceEnders, rune(tok[len(tok)-1])) {
			tok = tok[:len(tok)-1]
			end = true
		} else {
			for j := span[1]; j < len(text) && isTokenSeparatorByte(text[j]); j++ {
				if strings.IndexByte(clauseBoundarySeparators, text[j]) >= 0 {
					end = true
					break
				}
			}
		}
		if tok == "" {
			continue
		}
		out = append(out, tok)
		boundary = append(boundary, end)
	}
	return out, boundary
}

// globalOptionPrefixes is the pinned, closed set of git global-option
// spellings that take their operand FUSED into the same token via
// `=` (spec 127 H-r6-8). A global option outside this set is the
// stated finite-set limitation (R5(a)), not a bug here — e.g.
// `--no-replace-objects`, already live in this codebase's own real
// git invocation (internal/gitutil/workdestruction.go's
// revListCommitTrees), is a concrete, PRESENT-DAY instance of that
// disclosed gap, not a hypothetical one (S1-r2-6): a synthetic command
// combining it with `-C`/`-c` is not matched, while the same command
// without it is. No change needed for this bead; named here as a
// starting point for a future normalization-set extension.
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

// gitSubcommandNames is the vocabulary matchGit's own switch below
// recognizes. Duplicated here — not derived from the switch — purely
// so skipGlobalOptions can decide where a -C/-c operand span ends
// without a forward reference into matchGit; keeping the two lists in
// sync is a review-time obligation of the same shape as AllFamilies
// itself (spec 127 R5(a)): a subcommand added to the switch but not
// here degrades this file's own MATCHING precision for that one
// family when it is invoked behind `-C`/`-c`, never its SAFETY —
// the floor already fails toward requiring review, never toward
// proving a command safe.
var gitSubcommandNames = map[string]bool{
	"merge": true, "reset": true, "restore": true, "clean": true,
	"branch": true, "push": true, "stash": true, "worktree": true,
	"checkout": true, "switch": true, "update-ref": true, "tag": true,
	"reflog": true, "gc": true,
}

// looksLikeGitSubcommandOrFlag reports whether tok is shaped like the
// START of a new git argument — either a flag (leading '-') or a
// known git subcommand name — the stopping condition
// skipGlobalOptions uses to find the end of a -C/-c operand span
// (below).
func looksLikeGitSubcommandOrFlag(tok string) bool {
	if strings.HasPrefix(tok, "-") {
		return true
	}
	return gitSubcommandNames[tok]
}

// skipGlobalOptions returns the index of the git subcommand token,
// having stripped every contiguous global option (and its operand)
// from the pinned, closed normalization set starting right after
// "git". Returns t.len() if the stream runs out first (no subcommand
// present — not a match).
//
// A globalOptionWithOperand flag's (-C/-c) operand is consumed
// GREEDILY — every token up to (not including) the next token that
// itself looks like a git subcommand or a flag — rather than exactly
// one token (spec 127 bead-2 rework, G1-r2-2/O1-r2-9): tokenRe's
// separator set includes both quote characters, so a shell-quoted,
// whitespace-bearing operand (`git -C "/tmp/work tree" reset --hard`,
// `git -c "user.name=A B" reset --hard`) arrives here already split
// into multiple plain tokens with no leading quote left to notice —
// consuming exactly one token would leave the operand's SECOND word
// to be misread as the subcommand (and, for `-c`, its value entirely
// unrelated words could then masquerade as the subcommand too). The
// greedy scan keeps the whole operand attached to its flag in both
// the ordinary single-token case (the very next token IS a
// subcommand, so the loop consumes zero extra tokens) and the
// quoted-multi-token case.
func skipGlobalOptions(t tokStream) int {
	i := 1 // t.tok(0) == "git"
	for i < t.len() {
		tok := t.tok(i)
		switch {
		case globalOptionWithOperand[tok]:
			i++
			// Consume every operand token up to (not including) the
			// next token that itself looks like a subcommand or a
			// flag — the loop already leaves i pointing AT that
			// stopping token (or at t.len()), so no further
			// adjustment is needed: the single-token case advances
			// exactly once, the quoted-multi-token case advances
			// through every word of the operand.
			for i < t.len() && !looksLikeGitSubcommandOrFlag(t.tok(i)) {
				i++
			}
		case globalOptionStandalone[tok]:
			i++
		default:
			matched := false
			for _, p := range globalOptionPrefixes {
				if strings.HasPrefix(tok, p) {
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

// tokStream is a tokenized text span paired with its per-token
// sentence-boundary flags (tokenize, above). Matchers take a tokStream
// (or a suffix of one, via from) rather than a bare []string so a
// destructive trigger word is never read out of its own sentence
// (spec 127 bead-2 rework, O1-r2-3), while an invocation's OWN operand
// list is scanned to its true end with no arbitrary token-count cap
// (F1-r2-1's `bd delete <n ids> --force` regression class).
type tokStream struct {
	text     []string
	boundary []bool
}

func (t tokStream) len() int         { return len(t.text) }
func (t tokStream) tok(i int) string { return t.text[i] }
func (t tokStream) from(i int) tokStream {
	return tokStream{text: t.text[i:], boundary: t.boundary[i:]}
}

// findArg scans rest for a token satisfying pred, STOPPING at the
// first sentence boundary crossed (a destructive trigger word deep in
// a later, unrelated sentence can never be folded into an earlier,
// unrelated invocation) — otherwise unbounded, so a real invocation's
// own operand list, however long, is scanned to its end. Returns the
// matching token's index (relative to rest) or -1.
func findArg(rest tokStream, pred func(string) bool) int {
	for i := 0; i < rest.len(); i++ {
		if i > 0 && rest.boundary[i-1] {
			break
		}
		if pred(rest.tok(i)) {
			return i
		}
	}
	return -1
}

// matchGit attempts every git family against t (t.tok(0) == "git").
// Returns the matched family and the total token count consumed
// (including "git", any stripped globals, and the subcommand), or
// ok=false.
func matchGit(t tokStream) (DestructiveFamily, int, bool) {
	subIdx := skipGlobalOptions(t)
	if subIdx >= t.len() {
		return "", 0, false
	}
	sub := t.tok(subIdx)
	rest := t.from(subIdx + 1)
	base := subIdx + 1 // tokens consumed through the subcommand

	switch sub {
	case "merge":
		// Merge-STARTING forms only. `git merge --abort` restores the
		// pre-merge state (recorded floor-level exclusion, Background
		// :1641) — excluded only when --abort is the IMMEDIATE
		// argument after `merge`; --abort at any OTHER position
		// matches, deliberately, because no other position is a valid
		// abort invocation (`git merge --no-ff --abort` is not real
		// git syntax, so treating it as a merge-starting form is the
		// conservative, correct-for-a-floor reading, O1-r2-8).
		if idx := findArg(rest, func(t string) bool { return t == "--abort" }); idx == 0 {
			return "", 0, false
		}
		return FamilyGitMerge, base, true
	case "reset":
		// Matched at SUBCOMMAND grain, unqualified — deliberately
		// over-inclusive (O2-r2-12): `git reset --soft`/`git reset`
		// (index-only, no worktree change) match exactly like `git
		// reset --hard`. R5(a)'s own family list spells this family
		// bare, unlike its flag-qualified neighbors (`clean -f`,
		// `branch -D`), and both `--mixed`/bare and `--merge`/`--keep`
		// discard the INDEX — the staged-conflict-resolution case
		// bead 6's AC-9(v)(delta) protects — so under-matching here
		// would be the wrong direction for a floor. See
		// TestFindFloorMatches_ResetRestore_DeliberateOverMatch for
		// the pinned fixtures this comment describes.
		return FamilyGitReset, base, true
	case "restore":
		// Same deliberate over-match as "reset" above: `git restore`
		// bare/`--staged` (safe: it never touches the worktree) still
		// matches, because `git restore <path>`'s DEFAULT (no
		// --staged) overwrites the worktree from the index —
		// erring conservative on the common, unqualified spelling is
		// correct for a floor (O2-r2-12).
		return FamilyGitRestore, base, true
	case "clean":
		if idx := findArg(rest, func(t string) bool { return clusterHasFlag(t, "--force", 'f') }); idx >= 0 {
			return FamilyGitCleanForce, base + idx + 1, true
		}
	case "branch":
		if idx := findArg(rest, func(t string) bool { return clusterHasFlag(t, "-D", 'D') }); idx >= 0 {
			return FamilyGitBranchDeleteForce, base + idx + 1, true
		}
		// `--delete --force` (co-occurring) is `-D`'s long spelling
		// (O1-r2-1) — `--delete` ALONE is the safe delete and must
		// stay a non-match, so both flags are required, order-
		// insensitive, each found independently.
		delIdx := findArg(rest, func(t string) bool { return t == "--delete" })
		forceIdx := findArg(rest, func(t string) bool { return t == "--force" })
		if delIdx >= 0 && forceIdx >= 0 {
			last := delIdx
			if forceIdx > last {
				last = forceIdx
			}
			return FamilyGitBranchDeleteForce, base + last + 1, true
		}
	case "push":
		// Exact-token equality on "--force"/"-f" naturally excludes
		// the lease-guarded forms `--force-with-lease` /
		// `--force-if-includes` (distinct tokens) — floor-level
		// exclusion by construction, not by a discriminator (F3-r2-2).
		if idx := findArg(rest, func(t string) bool { return t == "--force" || t == "-f" }); idx >= 0 {
			return FamilyGitPushForce, base + idx + 1, true
		}
		// `-d` is git-push(1)'s own documented short spelling of
		// `--delete` (O1-r2-1).
		if idx := findArg(rest, func(t string) bool { return t == "--delete" || t == "-d" }); idx >= 0 {
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
		if rest.len() > 0 && rest.tok(0) == "remove" {
			if idx := findArg(rest.from(1), func(t string) bool { return clusterHasFlag(t, "--force", 'f') }); idx >= 0 {
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
		// `-f`/`--force` is git-switch(1)'s own documented alias of
		// `--discard-changes` (O1-r2-1).
		if idx := findArg(rest, func(t string) bool {
			return t == "--discard-changes" || t == "-f" || t == "--force"
		}); idx >= 0 {
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
		if rest.len() > 0 && rest.tok(0) == "expire" {
			return FamilyGitReflogExpireOrGCPrune, base + 1, true
		}
	case "gc":
		if idx := findArg(rest, func(t string) bool { return strings.HasPrefix(t, "--prune") }); idx >= 0 {
			return FamilyGitReflogExpireOrGCPrune, base + idx + 1, true
		}
	}
	return "", 0, false
}

// matchBd attempts the bd family against t (t.tok(0) == "bd").
func matchBd(t tokStream) (DestructiveFamily, int, bool) {
	if t.len() < 2 || t.tok(1) != "delete" {
		return "", 0, false
	}
	rest := t.from(2)
	// `-f` is bd's own documented short spelling of `--force`
	// (O1-r2-1: `bd delete --help` prints "-f, --force  Actually
	// delete"). The search itself is unbounded past the subcommand
	// (findArg, above) — a joined multi-ID `bd delete <ids...> --force`
	// form no longer stops matching once 8+ operand tokens intervene
	// (F1-r2-1).
	if idx := findArg(rest, func(t string) bool { return t == "--force" || t == "-f" }); idx >= 0 {
		return FamilyBdDeleteForce, 2 + idx + 1, true
	}
	return "", 0, false
}

// matchRm attempts the coreutils `rm -rf` family against t (t.tok(0)
// == "rm"). This is the BARE program only. A `git rm` invocation is
// NOT excluded by this function — FindFloorMatches (below) never
// dispatches to matchRm for a `git`-led token stream in the first
// place, since matchGit's own subcommand switch has no "rm" case, so
// a `git rm -r -f <path>` re-enters the outer scan loop at the "rm"
// token itself (matchGit declined the whole stream, scanning resumes
// one token later) and IS matched here, reported as family `rm -rf`
// with Text "rm -r -f" — a real, if git-subcommand-attributed, floor
// match, not a silent miss (O1-r2-7). `git rm --cached` (which keeps
// working-tree bytes) stays a recorded, tested non-match because
// `--cached` alone never satisfies the -r/-f pair this function
// requires.
func matchRm(t tokStream) (DestructiveFamily, int, bool) {
	rest := t.from(1)
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
	toks, boundary := tokenize(text)
	t := tokStream{text: toks, boundary: boundary}
	var out []Match
	i := 0
	for i < t.len() {
		switch t.tok(i) {
		case "git":
			if fam, n, ok := matchGit(t.from(i)); ok && n > 0 {
				out = append(out, Match{Family: fam, Text: strings.Join(toks[i:i+n], " ")})
				i += n
				continue
			}
		case "bd":
			if fam, n, ok := matchBd(t.from(i)); ok && n > 0 {
				out = append(out, Match{Family: fam, Text: strings.Join(toks[i:i+n], " ")})
				i += n
				continue
			}
		case "rm":
			if fam, n, ok := matchRm(t.from(i)); ok && n > 0 {
				out = append(out, Match{Family: fam, Text: strings.Join(toks[i:i+n], " ")})
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
// to reject operands that match no reviewed floor family — NOT to
// certify a non-matching operand as non-destructive (O1-r2-2): a
// command genuinely off this closed, reviewed floor is caught by
// review under the in-diff extension obligation (ADR-0035 amendment),
// never proven safe by this function's false return.
func IsFloorMatch(text string) bool {
	return len(FindFloorMatches(text)) > 0
}
