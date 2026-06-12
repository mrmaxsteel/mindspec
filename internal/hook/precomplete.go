package hook

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// SkipPanelEnv is the env-only escape hatch for the pre-complete gate
// (Spec 093 Req 13a, ADR-0037 §7). It is read via os.Getenv ONLY — the
// command string is NEVER consulted, because a PreToolUse hook inherits
// Claude Code's process environment (which the agent cannot alter) while
// the command line is the agent-writable channel. The variable name is
// documented for HUMANS in /ms-panel-tally § Escape hatch; it is NEVER
// printed in a Block message (HC-7) — see decision tests.
const SkipPanelEnv = "MINDSPEC_SKIP_PANEL"

// rawMergeFence is appended to every Block message (Spec 093 Req 12 / G3-1,
// ADR-0037 §8). Once the gate blocks `mindspec complete`, a raw
// `git merge bead/<id>` on the spec branch is the obvious workaround; the
// matcher only fires on `mindspec complete` and mindspec installs no
// pre-merge-commit git hook, so the fence is prose by necessity.
func rawMergeFence(beadID string) string {
	return fmt.Sprintf(
		"\nDo NOT bypass with raw `git merge bead/%s` — it skips bd closure, "+
			"worktree cleanup, and this gate; only `mindspec complete` merges bead branches.",
		beadID)
}

// skipHumanHint is the Block-message tail that tells the agent the hatch is
// human-only (Spec 093 Req 13a, HC-7). It deliberately does NOT print the
// MINDSPEC_SKIP_PANEL variable — a blocked LLM's highest-probability next
// action is pasting a suggested prefix.
const skipHumanHint = "\nSkipping the panel requires a human: a user must set the skip variable in " +
	"their own environment before launching the session — see /ms-panel-tally § Escape hatch"

// gateFacts is the fully-resolved, I/O-free input to panelGateDecision.
// Every field is gathered by runPreComplete's I/O layer (env, fs scan,
// git) so the decision itself is a pure function of these facts — the one
// testable home of the allow/block logic (Spec 093 Req 12).
type gateFacts struct {
	// beadID is the bead the matched `mindspec complete` targets.
	beadID string

	// skipEnv reports whether MINDSPEC_SKIP_PANEL == "1" (Req 13a).
	skipEnv bool

	// reg is the registration that named this bead, with its tally. nil
	// means no panel.json referenced the bead — fail-open (HC-4).
	reg *panel.Registration
	res *panel.Result

	// headSHA is the current `git rev-parse bead/<id>` in the scan root.
	// missingRef true means the branch GENUINELY no longer exists (exit-1 /
	// ErrRefNotFound) — the rerun-after-merge pass-through (Req 11). When
	// missingRef is true headSHA is "".
	headSHA    string
	missingRef bool

	// gitErr true means the rev-parse failed with a TRANSIENT or structural
	// error (not a clean "ref absent"): exit 128, git missing, lock
	// contention. It is deliberately NOT folded into missingRef so a transient
	// failure is not silently treated as a confirmed branch deletion (the
	// false-clear noted by the round-2 panel). Still fail-open per the spec's
	// deliberate posture (Req 11/12, advisory-backstopped), but surfaced
	// HONESTLY as a distinct Warn rather than a "merge already landed" note.
	gitErr error

	// worktreeAbsent reports that the bead worktree could not be found, so
	// the porcelain dirty check was skipped (Req 11 missing-worktree
	// pass-through). userDirt lists user-authored uncommitted paths
	// (artifact paths already filtered out) in the resolved worktree.
	worktreeAbsent bool
	worktreePath   string
	userDirt       []string
}

// panelGateDecision renders the pass/block decision for a single matched
// panel from fully-resolved facts (Spec 093 Req 12, ADR-0037 §§3-6). It is
// pure: no env, no fs, no git. The short-circuit ORDER is load-bearing and
// pinned by gate finding T3-1 —
//
//	(0) escape hatch  → Pass + Warn (audited at complete time)
//	(1) no panel      → Pass (fail-open, HC-4)
//	(2) malformed reg → Block (a registered-but-unparseable panel is NOT
//	                    "no panel"; it must not tip fail-open)
//	(3) abandoned     → Pass + Warn  (BEFORE staleness — an abandoned
//	                    panel whose branch gained commits must never be
//	                    false-Blocked by the stale-SHA rule)
//	(4) round mismatch→ Block
//	(5) missing ref   → Pass + Warn  (rerun-after-merge)
//	(6) stale SHA     → Block  (the lola-f4a8 pin)
//	(7) dirty tree    → Block  (CommitAll bypass; skipped when worktree absent)
//	(8) incomplete    → Block  (verdicts < expected_reviewers)
//	(9) REJECT/hard   → Block  (halt path, no vote count overrides)
//	(10) threshold    → Pass iff APPROVE ≥ N−1, else Block
//
// false POSITIVES (a wrongful Block) are the pinned bug class (Req 9); the
// missing-ref and missing-worktree pass-throughs exist to keep the
// documented partial-failure recovery rerun unblocked.
func panelGateDecision(f gateFacts) Result {
	// (0) Escape hatch — env-only, audited. Never names the variable in a
	// Block (this is a Warn/Pass path, so HC-7 is moot here, but we still
	// keep the message hatch-name-free except for this legitimate audit).
	if f.skipEnv {
		return Result{Action: Warn, Message: fmt.Sprintf(
			"panel gate skipped via %s for %s", SkipPanelEnv, f.beadID)}
	}

	// (1) Fail-open: no registered panel for the bead.
	if f.reg == nil || f.res == nil {
		return Result{Action: Pass}
	}

	slug := f.reg.Slug()

	// (2) A panel.json exists but could not be parsed — registered but
	// malformed. It must NOT read as "no panel".
	if f.res.PanelErr != nil || f.res.Panel == nil {
		return Result{Action: Block, Message: fmt.Sprintf(
			"panel %s registration (panel.json) is unreadable — fix or remove it before completing%s",
			slug, rawMergeFence(f.beadID))}
	}
	p := f.res.Panel
	round := f.res.LatestRound
	if round == 0 {
		round = p.Round
	}

	// (3) Abandoned — legitimate exit, audited on bead metadata at complete
	// time (Req 13e). Checked BEFORE staleness (T3-1) so an abandoned panel
	// whose branch advanced is never false-Blocked.
	if p.Abandoned {
		reason := strings.TrimSpace(p.AbandonReason)
		if reason == "" {
			reason = "(no reason recorded — abandon_reason is required; /ms-panel-tally abandon procedure)"
		}
		return Result{Action: Warn, Message: fmt.Sprintf(
			"panel %s round %d abandoned: %s — completing per the recorded abandonment", slug, round, reason)}
	}

	// (4) Round mismatch — panel.json.round disagrees with the
	// filename-derived latest round in either direction (Req 11).
	if f.res.RoundMismatch {
		return Result{Action: Block, Message: fmt.Sprintf(
			"panel %s: panel.json round (%d) out of date vs verdict files (round %d) — re-run /ms-panel-run step 0%s",
			slug, p.Round, f.res.LatestRound, rawMergeFence(f.beadID))}
	}

	// (5) Missing ref — the bead branch GENUINELY no longer exists (exit-1
	// ErrRefNotFound). The merge already landed (completion deletes the
	// branch); pass through to complete.Run's idempotent handling rather than
	// false-block the recovery rerun (Req 11).
	if f.missingRef {
		return Result{Action: Warn, Message: fmt.Sprintf(
			"panel for %s references branch %s, which no longer exists — assuming the merge already landed; "+
				"deferring to mindspec complete's own handling", f.beadID, workspace.BeadBranch(f.beadID))}
	}

	// (5b) Transient/structural git error — the staleness rev-parse could not
	// run (not a clean "ref absent"). The spec's posture is deliberately
	// fail-open (Req 11/12, advisory-backstopped), so we still Pass+Warn; but
	// — unlike (5) — we do NOT claim the merge landed, because a transient
	// error is NOT evidence the branch was deleted. Surfacing it honestly
	// closes the round-2 false-clear (a transient error conflated with a
	// genuine deletion) without false-blocking a legitimate completion.
	if f.gitErr != nil {
		return Result{Action: Warn, Message: fmt.Sprintf(
			"panel for %s: could not verify branch %s (transient git error: %v) — staleness check skipped; "+
				"proceeding per the gate's fail-open posture, but this is NOT a confirmed merge",
			f.beadID, workspace.BeadBranch(f.beadID), f.gitErr)}
	}

	// (6) Stale SHA — verdicts reviewed a different commit. BLOCK, never
	// Warn (a Warn here is the same prose-under-pressure failure the gate
	// exists to close — the lola-f4a8 bypass class, Req 11).
	if p.ReviewedHeadSHA != "" && f.headSHA != "" && !shaEqual(p.ReviewedHeadSHA, f.headSHA) {
		return Result{Action: Block, Message: fmt.Sprintf(
			"panel round %d reviewed %s, branch now at %s — commits landed after review; "+
				"bump round and re-panel (/ms-panel-run step 0)%s",
			round, short(p.ReviewedHeadSHA), short(f.headSHA), rawMergeFence(f.beadID))}
	}

	// (7) Dirty tree — uncommitted USER edits would be auto-committed past
	// review by CommitAll (Req 11). Artifact dirt is already filtered out by
	// the caller. Skipped entirely when the worktree is absent (the
	// missing-worktree partial-failure rerun window).
	if !f.worktreeAbsent && len(f.userDirt) > 0 {
		var b strings.Builder
		fmt.Fprintf(&b, "uncommitted changes in %s — `mindspec complete` would auto-commit them past review (CommitAll); commit and re-panel, or revert:",
			f.worktreePath)
		for _, d := range f.userDirt {
			fmt.Fprintf(&b, "\n  %s", d)
		}
		b.WriteString(rawMergeFence(f.beadID))
		return Result{Action: Block, Message: b.String()}
	}

	// (8) Incomplete — fewer valid verdicts than expected reviewers. Name
	// the PRESENT verdict files; the missing-slot NAMES are not derivable
	// from the Req 6 schema (it carries only an expected_reviewers int —
	// gate finding T3-2).
	n := p.ExpectedReviewers
	if !f.res.Complete() {
		present := presentVerdictFiles(f.res)
		return Result{Action: Block, Message: fmt.Sprintf(
			"panel %s round %d incomplete: %d/%d verdicts present (%s) — finish /ms-panel-run or tally first%s",
			slug, round, len(f.res.Verdicts), n, present, rawMergeFence(f.beadID))}
	}

	// (9) REJECT or hard_block — halt path; no vote count overrides an
	// evidence-bearing gate (the lola-f4a8 artifact-gate rule, mechanized).
	if f.res.Rejects > 0 || len(f.res.HardBlocks) > 0 {
		detail := fmt.Sprintf("%d/%d APPROVE", f.res.Approves, n)
		if len(f.res.HardBlocks) > 0 {
			detail += fmt.Sprintf(", hard_block from %s", strings.Join(f.res.HardBlocks, ", "))
		}
		return Result{Action: Block, Message: fmt.Sprintf(
			"panel %s round %d: %s — HARD block / REJECT recorded — halt path, see /ms-panel-tally%s",
			slug, round, detail, rawMergeFence(f.beadID))}
	}

	// (10) Threshold — N−1 (Req 12, single home in panel.ApproveThreshold).
	threshold := p.ApproveThreshold()
	if f.res.Approves >= threshold && threshold > 0 {
		return Result{Action: Pass}
	}
	return Result{Action: Block, Message: fmt.Sprintf(
		"panel %s round %d: %d/%d APPROVE — threshold is %d/%d. Run /ms-bead-fix with %s, then re-panel%s",
		slug, round, f.res.Approves, n, threshold, n, panel.ConsolidatedName(round), rawMergeFence(f.beadID))}
}

// presentVerdictFiles renders the present-verdict-file list for the
// incomplete Block. Missing-slot names are not derivable from the schema,
// so the contract is to enumerate what IS present (T3-2). Malformed files
// (counted as missing) are named separately so the agent can fix them.
func presentVerdictFiles(res *panel.Result) string {
	var files []string
	for _, v := range res.Verdicts {
		files = append(files, v.File)
	}
	sort.Strings(files)
	out := "present: "
	if len(files) == 0 {
		out += "none"
	} else {
		out += strings.Join(files, ", ")
	}
	if len(res.Malformed) > 0 {
		out += "; malformed (count as missing): " + strings.Join(res.Malformed, ", ")
	}
	return out
}

// shaEqual compares two git SHAs allowing one to be an abbreviation of the
// other (panel.json may record a short reviewed_head_sha; rev-parse yields
// the full 40-char form).
func shaEqual(a, b string) bool {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	if a == "" || b == "" {
		return false
	}
	if len(a) == len(b) {
		return a == b
	}
	if len(a) < len(b) {
		return strings.HasPrefix(b, a)
	}
	return strings.HasPrefix(a, b)
}

func short(sha string) string {
	sha = strings.TrimSpace(sha)
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// --- command matching (Spec 093 Req 9, S3-6) --------------------------------

// matchMindspecComplete reports whether command invokes `mindspec complete`
// at COMMAND POSITION — string start or immediately after an unquoted shell
// separator (`;`, `&&`, `||`, `|`, newline, `$(`, backtick), optionally
// preceded by env assignments (FOO=1) and/or a `cd <path> &&` / pushd
// prefix. Quoted-string mentions MUST NOT match (commit messages quoting the
// phrase, `grep 'mindspec complete' SKILL.md`, echoed --panel-state text).
//
// The tokenizer walks the command character by character, tracking quote
// state, and at each command-position boundary checks whether the next
// token sequence is `[env=…|cd …]* (…/)?mindspec complete`. Remaining false
// NEGATIVES fail open — the REQUIRED complete-side advisory (Req 13d) is the
// backstop; false POSITIVES are the pinned bug class (Req 9).
func matchMindspecComplete(command string) bool {
	for _, seg := range commandSegments(command) {
		if segmentInvokesComplete(seg) {
			return true
		}
	}
	return false
}

// commandSegments splits a command line into command-position segments,
// honoring quoting so that separators inside quotes do not split (and the
// quoted content is therefore never reachable as a command position).
func commandSegments(command string) []string {
	var segs []string
	var cur strings.Builder
	flush := func() {
		segs = append(segs, cur.String())
		cur.Reset()
	}
	runes := []rune(command)
	var quote rune // 0, '\'', or '"'
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		if quote != 0 {
			cur.WriteRune(c)
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
			cur.WriteRune(c)
		case '\n', ';', '|', '&':
			// Separator: end the current segment. `&&`/`||` collapse via
			// the empty-segment skip in segmentInvokesComplete.
			flush()
		case '`':
			// Backtick opens a command substitution → a fresh command
			// position. Treat the backtick as a separator.
			flush()
		case '$':
			if i+1 < len(runes) && runes[i+1] == '(' {
				// $( opens a command substitution → fresh command position.
				flush()
				i++ // skip '('
				continue
			}
			cur.WriteRune(c)
		default:
			cur.WriteRune(c)
		}
	}
	flush()
	return segs
}

// segmentInvokesComplete reports whether a single command-position segment,
// after stripping leading env assignments and `cd …`/`pushd …` prefixes,
// begins with `mindspec complete` (optionally a `<path>/mindspec`).
func segmentInvokesComplete(seg string) bool {
	fields := strings.Fields(seg)
	for len(fields) > 0 {
		head := fields[0]
		switch {
		case isEnvAssignment(head):
			fields = fields[1:]
			continue
		case head == "cd" || head == "pushd":
			// Drop `cd` and its single path argument (the && that followed
			// it was already consumed as a separator by commandSegments, so
			// the next field is the path).
			if len(fields) >= 2 {
				fields = fields[2:]
			} else {
				fields = fields[1:]
			}
			continue
		}
		break
	}
	if len(fields) < 2 {
		return false
	}
	if !isMindspecBinary(fields[0]) {
		return false
	}
	return fields[1] == "complete"
}

// isEnvAssignment reports whether a token is a leading shell env assignment
// (FOO=bar), which may legitimately precede a command at command position.
func isEnvAssignment(tok string) bool {
	eq := strings.IndexByte(tok, '=')
	if eq <= 0 {
		return false
	}
	name := tok[:eq]
	for i, c := range name {
		isAlpha := c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		isDigit := c >= '0' && c <= '9'
		if i == 0 && !isAlpha {
			return false
		}
		if !isAlpha && !isDigit {
			return false
		}
	}
	return true
}

// isMindspecBinary reports whether tok is the mindspec binary at command
// position: bare `mindspec` or any `<path>/mindspec`.
func isMindspecBinary(tok string) bool {
	return tok == "mindspec" || strings.HasSuffix(tok, "/mindspec")
}

// completeBeadID extracts the bead-id argument of a matched
// `mindspec complete` invocation: the first non-flag token after
// `complete`. Returns "" when absent (explicit-id completes only in v1 —
// a bare `mindspec complete` passes the gate, Req 10).
func completeBeadID(command string) string {
	for _, seg := range commandSegments(command) {
		if id := segmentCompleteBeadID(seg); id != "" {
			return id
		}
	}
	return ""
}

func segmentCompleteBeadID(seg string) string {
	fields := strings.Fields(seg)
	for len(fields) > 0 {
		head := fields[0]
		switch {
		case isEnvAssignment(head):
			fields = fields[1:]
			continue
		case head == "cd" || head == "pushd":
			if len(fields) >= 2 {
				fields = fields[2:]
			} else {
				fields = fields[1:]
			}
			continue
		}
		break
	}
	if len(fields) < 2 || !isMindspecBinary(fields[0]) || fields[1] != "complete" {
		return ""
	}
	args := fields[2:]
	for i := 0; i < len(args); i++ {
		tok := args[i]
		if strings.HasPrefix(tok, "-") {
			// Skip a known value-taking flag AND its space-separated value
			// (`--spec 093`). `--spec=093` carries its value inline, so only
			// the flag token is skipped. Other flags are valueless toggles.
			if isValueFlag(tok) && i+1 < len(args) {
				i++
			}
			continue
		}
		return tok
	}
	return ""
}

// isValueFlag reports whether a `mindspec complete` flag consumes the next
// token as its value (so the bead-id scan skips both). Only the value-taking
// flags need listing; everything else is treated as a valueless toggle. An
// inline `--flag=value` form carries its own value and never reaches here as
// a bare flag.
//
// The set is enumerated against the ACTUAL flags reachable on a
// `mindspec complete` invocation: the four local string flags on completeCmd
// (cmd/mindspec/complete.go:144-147) — --spec, --allow-doc-skew,
// --override-adr, --supersede-adr — PLUS the root-level persistent string
// flag --trace (cmd/mindspec/root.go:167), which any subcommand inherits.
// None of these has a shorthand (the previously-listed `-s` was bogus —
// --spec carries no `-s` alias). Omitting --trace let
// `mindspec complete --trace <file> <bead>` mis-extract <file> as the bead-id.
func isValueFlag(tok string) bool {
	if strings.Contains(tok, "=") {
		return false
	}
	switch tok {
	case "--spec", "--allow-doc-skew", "--override-adr", "--supersede-adr", "--trace":
		return true
	}
	return false
}

// cdPrefixPath extracts the `cd <path>` prefix that immediately precedes the
// `mindspec complete` invocation, resolved against sessionCwd (Req 10 scan-
// root (a)). Because commandSegments splits on `&&`/`;`/newline, the common
// `cd <path> && mindspec complete X` form lands the cd in the segment BEFORE
// the complete segment; this checks both that preceding segment and the
// (rare) same-segment leading cd. Returns "" when there is no cd prefix.
func cdPrefixPath(command, sessionCwd string) string {
	segs := commandSegments(command)
	resolve := func(p string) string {
		if p != "" && !filepath.IsAbs(p) && sessionCwd != "" {
			return filepath.Join(sessionCwd, p)
		}
		return p
	}
	for i, seg := range segs {
		if !segmentInvokesComplete(seg) {
			continue
		}
		// Same-segment leading cd (after env assignments).
		if p := leadingCdPath(seg); p != "" {
			return resolve(p)
		}
		// Preceding non-empty segment's cd (`cd wt && mindspec complete X`).
		// `&&`/`||` produce an empty segment between operands, so skip back
		// over blank segments.
		for j := i - 1; j >= 0; j-- {
			if strings.TrimSpace(segs[j]) == "" {
				continue
			}
			if p := leadingCdPath(segs[j]); p != "" {
				return resolve(p)
			}
			break
		}
		return ""
	}
	return ""
}

// leadingCdPath returns the path argument of a leading `cd`/`pushd` in a
// segment (after any env assignments), or "" if the segment does not start
// with cd.
func leadingCdPath(seg string) string {
	fields := strings.Fields(seg)
	for len(fields) > 0 && isEnvAssignment(fields[0]) {
		fields = fields[1:]
	}
	if len(fields) >= 2 && (fields[0] == "cd" || fields[0] == "pushd") {
		return fields[1]
	}
	return ""
}

// --- orchestration (the I/O layer; Spec 093 Reqs 9-13) ----------------------

// Seams for testing the scan-root resolution and git work without a real
// repo/bd. The decision itself (panelGateDecision) is pure and tested
// directly; these cover runPreComplete's wiring.
var (
	preCompleteFindRootFn   = workspace.FindLocalRoot
	preCompleteConfigLoadFn = config.Load
	// beadSpecLookupFn maps a bead-id to its owning spec id via the bd/phase
	// lookup (Req 10). It is NOT hook.ReadState's active-phase resolution,
	// which would pick the wrong spec under multi-active-spec when the
	// completed bead belongs to a different spec (gate finding T3-3).
	beadSpecLookupFn = func(beadID string) (specID string, err error) {
		_, specID, err = phase.FindEpicForBead(beadID)
		return specID, err
	}
	preCompleteRevParseFn = gitutil.RevParseRef
	preCompleteStatusFn   = gitutil.Status
	// worktreeListFn mirrors complete.Run's worktree resolution precedent
	// (bead.WorktreeList match on workspace.BeadWorktreeName). It is only
	// consulted as a FALLBACK when the BeadWorktreePath probe misses, so
	// the common matched-path case pays no bd subprocess.
	worktreeListFn = bead.WorktreeList
)

// runPreComplete is the pre-complete hook's I/O layer (Spec 093 Req 9 short-
// circuit order). It does the minimum work to reach panelGateDecision, and
// — critically — does ZERO config/git/fs/state work on a non-matching Bash
// command (HC-3): steps (1)-(2) below exit first.
//
// Scan-root resolution (Req 10) covers three roots, unioned and deduped:
//
//	(a) the matched command's `cd <path>` prefix, resolved against session cwd;
//	(b) the spec worktree derived from the COMMAND's bead-id via the bd/phase
//	    lookup (beadSpecLookupFn) — NOT hook.ReadState's active-phase result;
//	(c) workspace.FindLocalRoot(session cwd).
//
// If the bead-id→spec lookup (b) fails, coverage rests on roots (a)/(c).
func runPreComplete(inp *Input) Result {
	// (1) pure-stdin: the command we are gating.
	if inp == nil {
		return Result{Action: Pass}
	}
	command := inp.Command

	// (2) anchored command match — non-match exits with ZERO further work
	// (no env read, no config, no git, no fs). HC-3 zero-cost invariant.
	if !matchMindspecComplete(command) {
		return Result{Action: Pass}
	}

	beadID := completeBeadID(command)
	// Bare `mindspec complete` (no explicit id) passes — explicit-id
	// completes only in v1 (Req 10).
	if beadID == "" {
		return Result{Action: Pass}
	}

	// (3) escape hatch — env-only, audited (Req 13a). Checked before any
	// config/git/fs work so a human-set skip pays nothing.
	skipEnv := os.Getenv(SkipPanelEnv) == "1"
	if skipEnv {
		return panelGateDecision(gateFacts{beadID: beadID, skipEnv: true})
	}

	// (4) root + config — config toggle short-circuit (Req 13c).
	sessionCwd, _ := os.Getwd()
	root, err := preCompleteFindRootFn(sessionCwd)
	if err != nil {
		// Not inside a mindspec project — fail open.
		return Result{Action: Pass}
	}
	cfg, err := preCompleteConfigLoadFn(root)
	if err != nil {
		return Result{Action: Pass}
	}
	if !cfg.Enforcement.PanelGate {
		return Result{Action: Pass}
	}

	// (5) bead-id + scan-root resolution (Req 10).
	roots := resolveScanRoots(command, sessionCwd, root, cfg, beadID)

	// (6) panel.Scan (fs-only) → ForBead.
	regs := panel.ForBead(panel.Scan(roots...), beadID)
	if len(regs) == 0 {
		// No registered panel for this bead — fail open (HC-4).
		return Result{Action: Pass}
	}
	// One bead, one live panel by construction; if more than one matched
	// across roots, evaluate each ONCE. A Block from any matched panel wins
	// (a Pass requires every matched panel to pass); otherwise surface the
	// first Warn (the abandoned/missing-ref note), else Pass.
	var firstWarn *Result
	for i := range regs {
		f := resolvePanelFacts(regs[i], beadID, cfg)
		res := panelGateDecision(f)
		switch res.Action {
		case Block:
			return res
		case Warn:
			if firstWarn == nil {
				r := res
				firstWarn = &r
			}
		}
	}
	if firstWarn != nil {
		return *firstWarn
	}
	return Result{Action: Pass}
}

// resolvePanelFacts gathers the git facts (Req 11) for one matched
// registration and returns the I/O-free gateFacts for panelGateDecision.
// Matched-path git budget: at most TWO subprocesses (rev-parse + porcelain).
func resolvePanelFacts(reg panel.Registration, beadID string, cfg *config.Config) gateFacts {
	f := gateFacts{beadID: beadID, reg: &reg}

	res, err := panel.Tally(reg.Dir)
	if err != nil {
		// Directory unreadable — treat as malformed registration (Block),
		// not "no panel": surface a non-nil Result with PanelErr so the
		// decision blocks rather than fails open.
		f.res = &panel.Result{Dir: reg.Dir, PanelErr: err}
		return f
	}
	f.res = res

	// Abandoned and round-mismatch decisions need no git. To honor T3-1
	// (abandoned checked before staleness) AND keep the git budget low, we
	// still resolve git facts here, but panelGateDecision orders abandoned
	// FIRST so an abandoned panel is never false-Blocked. We can cheaply
	// skip git entirely on the abandoned/round-mismatch paths.
	if res.Panel != nil && (res.Panel.Abandoned || res.RoundMismatch) {
		return f
	}

	// The scan root that owns this panel dir is review/<slug>'s grandparent.
	scanRoot := filepath.Dir(filepath.Dir(reg.Dir))

	// (7) staleness — one `git rev-parse bead/<id>` in the scan root.
	sha, rerr := preCompleteRevParseFn(scanRoot, workspace.BeadBranch(beadID))
	if rerr != nil {
		// Distinguish a GENUINE missing ref (branch deleted — the merge
		// landed) from a transient/structural git error. Only the former is
		// the rerun-after-merge pass-through (Req 11); a transient error is
		// surfaced as a distinct Warn so it is not mistaken for a confirmed
		// deletion (round-2 false-clear).
		if errors.Is(rerr, gitutil.ErrRefNotFound) {
			f.missingRef = true
		} else {
			f.gitErr = rerr
		}
		return f
	}
	f.headSHA = sha

	// (8) dirty tree — one `git status --porcelain` in the resolved bead
	// worktree (Req 11). Runs only on the panel-matched path.
	wt := resolveBeadWorktree(scanRoot, cfg, beadID)
	if wt == "" {
		f.worktreeAbsent = true
		return f
	}
	f.worktreePath = wt
	out, serr := preCompleteStatusFn(wt)
	if serr != nil {
		// Porcelain failed (missing worktree raced in) → skip the dirty
		// check, mirroring the missing-worktree pass-through (Req 11).
		f.worktreeAbsent = true
		return f
	}
	f.userDirt = userDirtPaths(out)
	return f
}

// resolveScanRoots builds the union of scan roots (Req 10), deduped.
func resolveScanRoots(command, sessionCwd, sessionRoot string, cfg *config.Config, beadID string) []string {
	var roots []string
	add := func(p string) {
		if p == "" {
			return
		}
		for _, r := range roots {
			if r == p {
				return
			}
		}
		roots = append(roots, p)
	}

	// (a) cd-prefix root, normalized via FindLocalRoot.
	if cd := cdPrefixPath(command, sessionCwd); cd != "" {
		if r, err := preCompleteFindRootFn(cd); err == nil {
			add(r)
		} else {
			add(cd)
		}
	}

	// (b) spec worktree derived from the COMMAND's bead-id (bd/phase
	// lookup, NOT active-phase resolution — T3-3). Lookup failure → fall
	// back to (a)/(c).
	if specID, err := beadSpecLookupFn(beadID); err == nil && specID != "" {
		add(workspace.SpecWorktreePath(sessionRoot, cfg, specID))
	}

	// (c) session-cwd root.
	add(sessionRoot)
	return roots
}

// resolveBeadWorktree finds the bead worktree under the scan root per
// complete.Run's precedent (the nested worktree path convention). Returns ""
// when the worktree directory does not exist (partial-failure rerun window).
func resolveBeadWorktree(scanRoot string, cfg *config.Config, beadID string) string {
	wt := workspace.BeadWorktreePath(scanRoot, cfg, beadID)
	if dirExists(wt) {
		return wt
	}
	// The scan root may itself BE the spec worktree (when (b) resolved it),
	// in which case BeadWorktreePath nests correctly; if not, fall back to
	// the worktree-list match on the bead worktree name (complete.Run's
	// precedent). A list failure leaves the worktree unresolved → the
	// caller treats it as absent and skips the dirty check.
	entries, err := worktreeListFn()
	if err != nil {
		return ""
	}
	want := workspace.BeadWorktreeName(beadID)
	for _, e := range entries {
		if filepath.Base(e.Path) == want && dirExists(e.Path) {
			return e.Path
		}
	}
	return ""
}

// userDirtPaths parses `git status --porcelain` output and returns the
// user-authored dirty paths, filtering out ADR-0025 artifact paths
// (.beads/issues.jsonl) which are designed-for and never block (Req 11).
// Pure path filtering — no bd-export normalization call (matched-path git
// budget stays at two subprocesses).
func userDirtPaths(porcelain string) []string {
	var out []string
	for _, line := range strings.Split(porcelain, "\n") {
		line = strings.TrimRight(line, "\r")
		if len(line) < 4 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		if idx := strings.Index(path, " -> "); idx >= 0 {
			path = strings.TrimSpace(path[idx+4:])
		}
		if path == "" || isArtifactPath(path) {
			continue
		}
		out = append(out, path)
	}
	return out
}

// artifactPaths mirrors internal/next's ADR-0025 classification
// (.beads/issues.jsonl). Kept as a local copy rather than importing
// internal/next to avoid a hook→next dependency edge; the list is one
// entry today and any addition is a one-line append in both places.
var artifactPaths = []string{
	".beads/issues.jsonl",
}

func isArtifactPath(p string) bool {
	for _, a := range artifactPaths {
		if p == a {
			return true
		}
	}
	return false
}
