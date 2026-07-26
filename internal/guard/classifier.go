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
// global option outside globalOptionsWithOperand/globalOptionStandalone,
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
//
// Quote characters stay in this SEPARATOR set — tokenize does NOT
// treat them as span-opening delimiters — by design, not oversight
// (bead-2 rework round 2's confirm, S1/S2/S3/O1/G1 jointly): this
// tokenizer's dominant caller is prose/markdown scanning, where a
// quote is ordinary punctuation (a quoted warning, a quoted rule, a
// quoted error message) that must NEVER swallow the words inside it
// into one opaque, unscannable token — "rendering form separates
// nothing" (classifier.go's own header, H-r6-2) applies to quote
// marks exactly like backticks and `**`. A blanket quote-aware
// tokenizer was tried and reverted: it fixed the -C/-c shell-operand
// defect (G1-2/O1-9) but broke exactly this — a `"...git merge..."`
// quoted sentence inside shipped guidance stopped matching at all,
// silently hiding real content from the SAME scan this floor exists
// to feed. The fix instead lives in consumeGlobalOptionOperand
// (below), which is quote-aware ONLY for the genuinely-shell global-
// option-operand positions named in globalOptionsWithOperand (spec 127
// bead-2 rework round 3, RULING 3: originally scoped to -C/-c alone,
// widened to include the fused `=`-operand globals after O1-confirm2-1
// proved two of five quote-bearing globals were left uncovered) —
// scoped narrowly rather than applied to every quote mark in every
// scanned text.
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

// isQuoteByte reports whether b is a quote character — used only by
// consumeGlobalOptionOperand's narrowly-scoped quote-awareness
// (below), never by tokenize itself.
func isQuoteByte(b byte) bool {
	return b == '"' || b == '\''
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
//
// A third return value, starts, records each token's byte offset in
// the ORIGINAL text — needed only by consumeGlobalOptionOperand
// (below) to re-examine the raw text right after a -C/-c flag for a
// quote character tokenize itself does not treat specially.
func tokenize(text string) ([]string, []bool, []int) {
	idx := tokenRe.FindAllStringIndex(text, -1)
	out := make([]string, 0, len(idx))
	boundary := make([]bool, 0, len(idx))
	starts := make([]int, 0, len(idx))
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
		starts = append(starts, span[0])
	}
	return out, boundary, starts
}

// globalOptionSpec describes one git global option this floor's
// skipGlobalOptions normalizes past that takes an operand. flag is the
// option's own token spelling: for a FUSED `=`-operand option
// (`--git-dir=<path>`) this includes the trailing "="; for a space-
// separated option (`-C <path>`) it is the bare flag.
//
// Spec 127 bead-2 rework round 3, RULING 3/O1-confirm2-1: round 2
// wired quote-awareness to ONLY the space-separated pair (-C/-c) via a
// separate globalOptionWithOperand map, while the fused-`=`-operand
// globals lived in a SECOND, independently-edited list
// (globalOptionPrefixes) that was never wired to the same fix — the
// fused globals' quoted, whitespace-bearing operands
// (`--git-dir="/path with spaces"`) silently under-matched, two of
// five quote-bearing globals covered. Merging both into this ONE list
// — every entry routed through the SAME consumeGlobalOptionOperand
// (below) — derives "does this global take a quote-bearing operand"
// from a single source, so a global option can no longer be added to
// only one of two paths, because there is only one path.
type globalOptionSpec struct {
	flag string
}

// globalOptionsWithOperand is the pinned, closed set of git global
// options that take an operand (H-r6-8/S1-r2-6). A global option
// outside this set is the stated finite-set limitation (R5(a)), not a
// bug here — e.g. `--no-replace-objects`, already live in this
// codebase's own real git invocation
// (internal/gitutil/workdestruction.go's revListCommitTrees), is a
// concrete, PRESENT-DAY instance of that disclosed gap, not a
// hypothetical one (S1-r2-6): a synthetic command combining it with
// `-C`/`-c` is not matched, while the same command without it is. No
// change needed for this bead; named here as a starting point for a
// future normalization-set extension.
var globalOptionsWithOperand = []globalOptionSpec{
	{flag: "-C"},
	{flag: "-c"},
	{flag: "--git-dir="},
	{flag: "--work-tree="},
	{flag: "--exec-path="},
}

// globalOptionStandalone take no operand.
var globalOptionStandalone = map[string]bool{
	"-p":         true,
	"--paginate": true,
	"--no-pager": true,
}

// skipGlobalOptions returns the index of the git subcommand token,
// having stripped every contiguous global option (and its operand)
// from the pinned, closed normalization set starting right after
// "git", plus whether any of those operands was consumed as a
// MULTI-TOKEN quoted span (as opposed to a single unquoted token or a
// fused, self-contained one) — needed by matchGit (below) to decide
// how far to skip forward even when the resolved "subcommand" turns
// out not to match any family (O1-confirm2-2: those interior tokens,
// already split OUT of the quoted span by tokenize's own quote-
// oblivious pass, must never be individually re-examined as fresh
// candidates by the outer scan). Returns t.len() as the index if the
// stream runs out first (no subcommand present — not a match).
//
// Every globalOptionsWithOperand entry's operand is consumed via
// consumeGlobalOptionOperand (below) — NOT a plain "next token"
// advance — because tokenize (this file's general prose/markdown
// scanner) deliberately leaves quote characters in its separator set
// (rendering form separates nothing, H-r6-2), so a shell-quoted,
// whitespace-bearing operand
// (`git -C "/tmp/work tree" reset --hard`) arrives at THIS function
// already split into multiple plain tokens with no leading quote left
// to notice. Bead-2 rework round 2 tried making tokenize itself
// globally quote-aware (G1-r2-2/O1-r2-9's own fix) and that broke
// prose scanning outright — a quoted SENTENCE in shipped guidance
// (`"...the actual git merge..."`) silently stopped matching, because
// the quotes swallowed the whole sentence into one opaque token. The
// confirm round's OWN findings (G1-2/O1-confirm-1) were about that
// same broken mechanism cutting the other way (a quoted value's
// embedded words leaking in as a false subcommand, or a quoted
// value's leading '-' defeating the greedy multi-token scan it
// replaced). The fix that survives both directions scopes quote-
// awareness to exactly the genuinely-shell positions in
// globalOptionsWithOperand instead of the whole tokenizer.
func skipGlobalOptions(raw string, t tokStream) (int, bool) {
	i := 1 // t.tok(0) == "git"
	quotedSpanConsumed := false
	for i < t.len() {
		tok := t.tok(i)
		if globalOptionStandalone[tok] {
			i++
			continue
		}
		matched := false
		for _, spec := range globalOptionsWithOperand {
			if tok == spec.flag {
				// Exact match: either a space-separated flag (-C/-c) or
				// a fused `=`-flag whose operand did NOT end up fused
				// into this same token — a quote character right after
				// "=" is a separator to tokenize (H-r6-2), so
				// `--git-dir="/path with spaces"` splits into the bare
				// "--git-dir=" token plus the quoted content's own
				// fragments, exactly the shape consumeGlobalOptionOperand
				// exists to re-examine. This exact-equality check works
				// unchanged for both shapes: it only needs the current
				// token's own end position, which consumeGlobalOptionOperand
				// re-derives from t.starts[i] regardless.
				var quoted bool
				i, quoted = consumeGlobalOptionOperand(raw, t, i)
				quotedSpanConsumed = quotedSpanConsumed || quoted
				matched = true
				break
			}
			if strings.HasSuffix(spec.flag, "=") && strings.HasPrefix(tok, spec.flag) {
				// Fused, unquoted, no internal whitespace: the operand
				// is already inside THIS token in full
				// (`--git-dir=/x`) — one token, nothing more to consume.
				i++
				matched = true
				break
			}
		}
		if !matched {
			return i, quotedSpanConsumed
		}
	}
	return i, quotedSpanConsumed
}

// consumeGlobalOptionOperand returns the token index right past
// globalOptionsWithOperand flag t.tok(i)'s operand, plus whether that
// operand was consumed as a MULTI-TOKEN quoted span (as opposed to a
// single unquoted token). It re-examines the ORIGINAL text right after
// the flag's own span for a quote character tokenize's general pass
// does not treat specially:
//
//   - no quote there (the ordinary case, `git -C ../wt reset --hard`,
//     this repo's own house style, or a fused `--git-dir=` flag whose
//     token already ended mid-word): the operand is exactly the next
//     token, so this returns (i+2, false).
//   - a quote there, and it BALANCES (a matching close found before
//     end of text — a backslash immediately before the SAME quote
//     character escapes it, staying inside the span; a DIFFERENT quote
//     character nested inside is literal content, not a new span):
//     the operand is everything between the quotes. This returns
//     (j, true), where j is the index of the first EXISTING token
//     (from tokenize's own separate, quote-oblivious pass) whose start
//     offset is at or past the closing quote — i.e., every token
//     tokenize already split OUT of the quoted span's interior is
//     skipped over as a block, never individually re-examined as a
//     candidate subcommand or flag by skipGlobalOptions' own caller.
//   - a quote there, and it NEVER closes before end of text (spec 127
//     bead-2 rework round 3, RULING 3/S1-confirm2-new-1): round 2
//     swallowed the remainder as the operand, reasoning that nothing
//     past a stray quote could then be misread as a fresh subcommand —
//     S1 proved that reasoning backward. A hostile author is in this
//     spec's threat model, and a swallow-to-end is an EVASION (it
//     hides whatever REAL content follows the malformed quote, e.g. a
//     genuine `git reset --hard` sitting in plain text right after
//     it), not an honest miss. This falls back to the SAME single-
//     next-token consumption the unquoted case uses — (i+2, false) —
//     so scanning resumes normally afterward: fail toward matching,
//     not toward swallowing.
func consumeGlobalOptionOperand(raw string, t tokStream, i int) (int, bool) {
	flagEnd := t.starts[i] + len(t.tok(i))
	pos := flagEnd
	for pos < len(raw) && (raw[pos] == ' ' || raw[pos] == '\t') {
		pos++
	}
	if pos >= len(raw) || !isQuoteByte(raw[pos]) {
		return i + 2, false // unquoted: exactly the next token
	}
	quote := raw[pos]
	pos++ // skip opening quote
	closed := false
	for pos < len(raw) {
		c := raw[pos]
		if c == '\\' && pos+1 < len(raw) && raw[pos+1] == quote {
			pos += 2
			continue
		}
		if c == quote {
			pos++ // skip closing quote
			closed = true
			break
		}
		pos++
	}
	if !closed {
		// Unbalanced: fail toward matching, not toward swallowing (see
		// this function's own doc comment above).
		return i + 2, false
	}
	// pos is now just past the consumed quoted span. Find the first
	// token whose own span starts at or after pos.
	j := i + 1
	for j < t.len() && t.starts[j] < pos {
		j++
	}
	return j, true
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
// (F1-r2-1's `bd delete <n ids> --force` regression class). starts
// carries each token's byte offset in the original text — needed only
// by consumeGlobalOptionOperand's narrowly-scoped quote-awareness.
type tokStream struct {
	text     []string
	boundary []bool
	starts   []int
}

func (t tokStream) len() int         { return len(t.text) }
func (t tokStream) tok(i int) string { return t.text[i] }
func (t tokStream) from(i int) tokStream {
	return tokStream{text: t.text[i:], boundary: t.boundary[i:], starts: t.starts[i:]}
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
// raw is the full original text, needed only to re-examine a global
// option's operand for a quote character (consumeGlobalOptionOperand,
// via skipGlobalOptions). Returns the matched family and the total
// token count consumed (including "git", any stripped globals, and
// the subcommand), or ok=false with a still-meaningful skip count.
//
// That skip count matters even on a non-match (spec 127 bead-2 rework
// round 3, RULING 3/O1-confirm2-2): when skipGlobalOptions consumed a
// MULTI-TOKEN quoted global-option operand, its interior tokens —
// already split OUT of the quoted span by tokenize's own quote-
// oblivious pass — must never be individually re-examined as fresh
// candidates by FindFloorMatches' own outer scan loop just because the
// resolved "subcommand" position happened not to match any family
// (e.g. `git -c "note=... git reset --hard ..." status`: the quoted
// note's own embedded "git reset --hard" is inert text, not a second
// invocation). So every non-match return below carries minSkip — 1
// when no quoted span was involved (the ordinary single-step scan,
// unchanged), or the full distance through the quoted span otherwise —
// rather than a bare 0.
func matchGit(raw string, t tokStream) (DestructiveFamily, int, bool) {
	subIdx, quotedSpanConsumed := skipGlobalOptions(raw, t)
	minSkip := 1
	if quotedSpanConsumed {
		minSkip = subIdx
	}
	if subIdx >= t.len() {
		return "", minSkip, false
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
			return "", minSkip, false
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
	return "", minSkip, false
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
	toks, boundary, starts := tokenize(text)
	t := tokStream{text: toks, boundary: boundary, starts: starts}
	var out []Match
	i := 0
	for i < t.len() {
		switch t.tok(i) {
		case "git":
			// matchGit's own n is meaningful even when ok is false (spec
			// 127 bead-2 rework round 3, RULING 3/O1-confirm2-2): it
			// carries the distance through any MULTI-TOKEN quoted
			// global-option operand that was consumed while resolving
			// (and rejecting) a candidate subcommand, so this loop skips
			// past that operand's interior tokens on a failed match
			// exactly as it would on a successful one, instead of
			// stepping one token at a time and re-entering them as fresh
			// candidates.
			fam, n, ok := matchGit(text, t.from(i))
			if n < 1 {
				n = 1
			}
			if ok {
				out = append(out, Match{Family: fam, Text: strings.Join(toks[i:i+n], " ")})
			}
			i += n
			continue
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
