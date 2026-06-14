package hook

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// SkipPanelEnv is the env-only escape hatch for the pre-complete gate
// (Spec 093 Req 13a, ADR-0037 §7). It re-exports panel.SkipPanelEnv (the
// single home of the variable name, Spec 099) so existing callers and tests
// that read hook.SkipPanelEnv keep working. It is read via os.Getenv ONLY —
// the command string is NEVER consulted, because a PreToolUse hook inherits
// Claude Code's process environment (which the agent cannot alter) while the
// command line is the agent-writable channel.
const SkipPanelEnv = panel.SkipPanelEnv

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
// after stripping leading env assignments, `cd …`/`pushd …`, and the catchable
// unquoted wrapper prefixes (`env`/`timeout`/`command`/`xargs`), begins with
// `mindspec complete` (optionally a `<path>/mindspec`).
//
// ADR-0037 residual: the QUOTED wrapper forms — `sh -c '… complete'`,
// `eval '… complete'` — are an explicitly-accepted residual. A non-executing
// tokenizer structurally cannot reach the wrapped command inside the quoted
// string (it is a single argument token, not a command-position segment), so we
// deliberately do NOT attempt to match them here.
func segmentInvokesComplete(seg string) bool {
	_, ok := strippedCompleteFields(seg)
	return ok
}

// strippedCompleteFields strips a single command-position segment's leading env
// assignments, `cd …`/`pushd …`, and the catchable unquoted wrapper prefixes
// (`env`/`timeout`/`command`/`xargs`), then reports whether the remainder is a
// `mindspec complete …` invocation. On a match it returns the remaining fields
// starting at the `mindspec` binary token (fields[0] is the binary, fields[1]
// is "complete", fields[2:] are the complete args) and ok=true; otherwise it
// returns nil, false.
//
// This is the SINGLE source of wrapper-stripping shared by both
// segmentInvokesComplete (the match guard) and segmentCompleteBeadID (the
// bead-id extractor) so the two can never diverge — a divergence is exactly
// how a wrapped complete could be recognized-but-fail-open.
func strippedCompleteFields(seg string) ([]string, bool) {
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
		case head == "env":
			// `env [-i] [-u VAR] [VAR=val …] command …`. Drop the bare `env`
			// keyword; the `-i`/`-u` flags and the `VAR=val` assignments that
			// may follow are consumed below (the assignments via the loop's
			// existing isEnvAssignment case, -u's operand here).
			fields = fields[1:]
			for len(fields) > 0 {
				switch {
				case fields[0] == "-i" || fields[0] == "-":
					fields = fields[1:]
				case fields[0] == "-u":
					// -u takes a VAR operand.
					if len(fields) >= 2 {
						fields = fields[2:]
					} else {
						fields = fields[1:]
					}
				default:
					goto envDone
				}
			}
		envDone:
			continue
		case head == "timeout":
			// `timeout [FLAGS] DURATION command …`. Drop `timeout`, its flags,
			// then EXACTLY ONE non-flag operand (the mandatory DURATION) so the
			// loop returns to the wrapped command — never over-skipping onto it.
			fields = fields[1:]
			for len(fields) > 0 && strings.HasPrefix(fields[0], "-") {
				switch fields[0] {
				case "-s", "-k":
					// Value-taking short flags (signal / kill-after) whose
					// value is a separate token.
					if len(fields) >= 2 {
						fields = fields[2:]
					} else {
						fields = fields[1:]
					}
				default:
					// --signal=SIG / --kill-after=DUR (value attached) and
					// the valueless --preserve-status / --foreground.
					fields = fields[1:]
				}
			}
			if len(fields) > 0 {
				// Skip exactly the one DURATION operand.
				fields = fields[1:]
			}
			continue
		case head == "command":
			// `command [-p] [-v] [-V] command …`. Drop `command` and its flags.
			fields = fields[1:]
			for len(fields) > 0 && strings.HasPrefix(fields[0], "-") {
				fields = fields[1:]
			}
			continue
		case head == "xargs":
			// `xargs [FLAGS] command …`. Drop `xargs`, its flags (some take a
			// value operand), then continue at the wrapped command. xargs does
			// NOT take a positional operand before the command, so no extra
			// non-flag skip — the first non-flag token IS the command.
			fields = fields[1:]
			for len(fields) > 0 && strings.HasPrefix(fields[0], "-") {
				switch fields[0] {
				case "-I", "-n", "-P", "-d", "-E", "-s", "-L", "-a":
					// Value-taking flags whose value is a separate token.
					if len(fields) >= 2 {
						fields = fields[2:]
					} else {
						fields = fields[1:]
					}
				default:
					// Valueless flags (-0/-r/-t/-p/-x) and any --long=val /
					// attached-value short flag (e.g. -I{}, -n1).
					fields = fields[1:]
				}
			}
			continue
		}
		break
	}
	if len(fields) < 2 {
		return nil, false
	}
	if !isMindspecBinary(fields[0]) {
		return nil, false
	}
	if fields[1] != "complete" {
		return nil, false
	}
	return fields, true
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
	// Share the SAME wrapper-stripping as segmentInvokesComplete (the match
	// guard) so a wrapped complete the gate RECOGNIZES also has its bead-id
	// EXTRACTED — otherwise the gate fails open on env/timeout/xargs/command
	// forms (recognized-but-unenforced).
	fields, ok := strippedCompleteFields(seg)
	if !ok {
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
// repo/bd. The decision itself (panel.PanelGateDecision) is pure and tested
// directly in internal/panel; these cover runPreComplete's wiring. The
// rev-parse / porcelain seams are injected into panel.ResolveGateFacts via
// hookGateIO, so the GREEN-parity run tests that stub these vars drive the
// rewired path unchanged.
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

// hookGateIO is the GateIO seam the hook injects into
// panel.ResolveGateFacts: the real git rev-parse / porcelain / ref-not-found
// wiring plus the lazy bead-worktree resolver. It reads the unexported seam
// vars (preCompleteRevParseFn / preCompleteStatusFn / worktreeListFn, via
// resolveBeadWorktree) so the GREEN-parity run tests that stub those vars keep
// driving the rewired path unchanged.
func hookGateIO(scanRoot, beadID string, cfg *config.Config) panel.GateIO {
	return panel.GateIO{
		RevParse:      preCompleteRevParseFn,
		Status:        preCompleteStatusFn,
		IsRefNotFound: func(e error) bool { return errors.Is(e, gitutil.ErrRefNotFound) },
		// Lazy: only invoked on the dirty-check path (after a successful
		// rev-parse), so the worktree-list fallback subprocess is NOT paid on
		// the abandoned / mismatch / missing-ref / transient-gitErr short
		// circuits — preserving the original two-subprocess budget.
		Worktree: func() string { return resolveBeadWorktree(scanRoot, cfg, beadID) },
	}
}

// ADR-0037 AMENDMENT NOTE (Spec 099 R4): the panel-gate DECISION + the
// fact-gathering now live in the internal/panel leaf
// (panel.PanelGateDecision / panel.ResolveGateFacts). `mindspec complete`'s
// in-binary gate (Spec 099 Bead 2) is the AUTHORITATIVE enforcement point;
// this PreToolUse hook is a defense-in-depth BACKSTOP that invokes the
// IDENTICAL decision over IDENTICAL facts, so the two cannot disagree. The
// matcher below is LEFT IN PLACE, behaviorally unchanged; the hook +
// heuristic-matcher RETIREMENT is a deferred follow-up bead (filed at merge).

// runPreComplete is the pre-complete hook's I/O layer (Spec 093 Req 9 short-
// circuit order). It does the minimum work to reach panel.PanelGateDecision,
// and — critically — does ZERO config/git/fs/state work on a non-matching Bash
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
		return toResult(panel.PanelGateDecision(panel.GateFacts{BeadID: beadID, SkipEnv: true}))
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
		res := toResult(panel.PanelGateDecision(f))
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

// toResult maps the extracted panel.Decision back to the hook's Result
// (panel.GateAction Allow/Block/Warn → hook.Action Pass/Block/Warn), copying
// the message verbatim so the hook's exit-2 emission stays byte-identical.
func toResult(d panel.Decision) Result {
	var a Action
	switch d.Action {
	case panel.Block:
		a = Block
	case panel.Warn:
		a = Warn
	default: // panel.Allow
		a = Pass
	}
	return Result{Action: a, Message: d.Message}
}

// resolvePanelFacts gathers the git facts (Req 11) for one matched
// registration and returns the I/O-free panel.GateFacts for
// panel.PanelGateDecision. The hook resolves the scan root + bead worktree
// path and injects the git I/O via the GateIO seam (so internal/panel stays a
// leaf). Matched-path git budget: at most TWO subprocesses (rev-parse +
// porcelain).
func resolvePanelFacts(reg panel.Registration, beadID string, cfg *config.Config) panel.GateFacts {
	// The scan root that owns this panel dir is review/<slug>'s grandparent.
	scanRoot := panel.PanelDirScanRoot(reg.Dir)
	return panel.ResolveGateFacts(reg, beadID, scanRoot, hookGateIO(scanRoot, beadID, cfg))
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

// NOTE (Spec 099): userDirtPaths / artifactPaths / isArtifactPath (the
// ADR-0025 porcelain filtering) moved to internal/panel (panel.userDirtPaths)
// alongside the relocated decision, so both the hook and the in-binary gate
// share the one filter. They are invoked from panel.ResolveGateFacts.
