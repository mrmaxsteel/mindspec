package guard

import (
	"strings"
	"testing"
)

// classifier_test.go pins the R5(a) floor's precision at the unit
// level: one representative probe per family (positive), the pinned
// global-option normalization, option-cluster equivalence, and the
// four recorded floor-level exclusions (negative — must NOT match).
// The repo-wide AC-9(i) injection fixture in internal/approve (see
// internal/lint/destructive_guidance_test.go) exercises the SAME
// classifier through the scan; this file is the classifier's own
// contract, independent of any scan machinery.

func TestAllFamilies_CountSentinel(t *testing.T) {
	if len(AllFamilies) != DestructiveFamilyCount {
		t.Fatalf("AllFamilies has %d entries, DestructiveFamilyCount says %d — keep the sentinel in lockstep (spec 127 B-r4-3 pattern)", len(AllFamilies), DestructiveFamilyCount)
	}
}

func TestFindFloorMatches_OnePerFamily(t *testing.T) {
	cases := []struct {
		name string
		text string
		fam  DestructiveFamily
	}{
		{"merge", "recovery: git merge --no-ff bead/x", FamilyGitMerge},
		{"reset", "recovery: git reset --hard origin/main", FamilyGitReset},
		{"restore", "recovery: git restore .", FamilyGitRestore},
		{"clean", "recovery: git clean -fd", FamilyGitCleanForce},
		{"branch-delete", "recovery: git branch -D bead/x", FamilyGitBranchDeleteForce},
		{"push-force", "recovery: git push --force origin main", FamilyGitPushForce},
		{"push-force-short", "recovery: git push -f origin main", FamilyGitPushForce},
		{"stash-drop", "recovery: git stash drop", FamilyGitStashDropClear},
		{"stash-clear", "recovery: git stash clear", FamilyGitStashDropClear},
		{"worktree-remove-force", "recovery: git worktree remove --force ../wt", FamilyGitWorktreeRemoveForce},
		{"worktree-remove-force-short", "recovery: git worktree remove -f ../wt", FamilyGitWorktreeRemoveForce},
		{"checkout-pathspec", "recovery: git checkout -- path/to/file", FamilyGitCheckoutPathspecDiscard},
		{"checkout-pathspec-ref", "recovery: git checkout main -- path/to/file", FamilyGitCheckoutPathspecDiscard},
		{"switch-discard", "recovery: git switch --discard-changes main", FamilyGitSwitchDiscardChanges},
		{"checkout-force", "recovery: git checkout -f main", FamilyGitCheckoutForce},
		{"checkout-force-long", "recovery: git checkout --force main", FamilyGitCheckoutForce},
		{"update-ref-delete", "recovery: git update-ref -d refs/heads/x", FamilyGitUpdateRefDelete},
		{"tag-delete", "recovery: git tag -d v1.2.3", FamilyGitTagDelete},
		{"push-delete", "recovery: git push origin --delete bead/x", FamilyGitPushDelete},
		{"push-refspec-delete", "recovery: git push origin :bead/x", FamilyGitPushDelete},
		{"reflog-expire", "recovery: git reflog expire --expire=now --all", FamilyGitReflogExpireOrGCPrune},
		{"gc-prune", "recovery: git gc --prune=now", FamilyGitReflogExpireOrGCPrune},
		{"rm-rf", "recovery: rm -rf ../stale-worktree", FamilyRmRf},
		{"bd-delete-force", "recovery: bd delete mindspec-abcd.1 --force", FamilyBdDeleteForce},
	}
	if len(cases) < DestructiveFamilyCount {
		t.Fatalf("this table has %d rows, the floor has %d families — every family needs at least one probe (AC-9(i))", len(cases), DestructiveFamilyCount)
	}
	seen := map[DestructiveFamily]bool{}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			matches := FindFloorMatches(c.text)
			if len(matches) == 0 {
				t.Fatalf("expected a %s match in %q, got none", c.fam, c.text)
			}
			found := false
			for _, m := range matches {
				if m.Family == c.fam {
					found = true
				}
			}
			if !found {
				t.Fatalf("expected family %s in %q, got %+v", c.fam, c.text, matches)
			}
			seen[c.fam] = true
		})
	}
	for _, fam := range AllFamilies {
		if !seen[fam] {
			t.Errorf("family %s has no probe in this table", fam)
		}
	}
}

func TestFindFloorMatches_OptionClusterEquivalence(t *testing.T) {
	cases := []struct {
		name string
		text string
		fam  DestructiveFamily
	}{
		{"clean-df", "git clean -df", FamilyGitCleanForce},
		{"clean-fdx", "git clean -fdx", FamilyGitCleanForce},
		{"rm-fr", "rm -fr some/dir", FamilyRmRf},
		{"rm-Rf", "rm -Rf some/dir", FamilyRmRf},
		{"rm-separate-flags", "rm -r -f some/dir", FamilyRmRf},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			matches := FindFloorMatches(c.text)
			if len(matches) != 1 || matches[0].Family != c.fam {
				t.Fatalf("%q: expected exactly one %s match, got %+v", c.text, c.fam, matches)
			}
		})
	}
}

func TestFindFloorMatches_GlobalOptionNormalization(t *testing.T) {
	// H-r6-8: `git -C <path> <subcommand>` is this repo's own house
	// style for operating on another worktree — the classifier must
	// strip the pinned global-option set before family matching.
	cases := []struct {
		name string
		text string
		fam  DestructiveFamily
	}{
		{"dash-C-reset-hard", "git -C ../other-worktree reset --hard origin/main", FamilyGitReset},
		{"dash-C-worktree-remove-force", "git -C ../other-worktree worktree remove -f ../wt", FamilyGitWorktreeRemoveForce},
		{"dash-c-then-subcommand", "git -c advice.graftFileDeprecated=false reset --hard", FamilyGitReset},
		{"git-dir-equals", "git --git-dir=/repo/.git reset --hard", FamilyGitReset},
		{"no-pager-then-subcommand", "git --no-pager reset --hard", FamilyGitReset},
		{"work-tree-equals", "git --work-tree=/repo/wt clean -f", FamilyGitCleanForce},
		{"exec-path-equals", "git --exec-path=/usr/lib/git-core reset --hard", FamilyGitReset},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			matches := FindFloorMatches(c.text)
			found := false
			for _, m := range matches {
				if m.Family == c.fam {
					found = true
				}
			}
			if !found {
				t.Fatalf("%q: expected %s after normalization, got %+v", c.text, c.fam, matches)
			}
		})
	}
}

// TestFindFloorMatches_RealGitCallSurvivesNormalization pins the
// bead-1 lesson named in this bead's brief: bead 1 shipped a real git
// call with `-c advice.graftFileDeprecated=false` — normalization must
// strip that global option WITHOUT making the underlying git
// subcommand invisible when it genuinely is NOT destructive (a
// too-strong classifier is exactly as wrong as a too-weak one).
func TestFindFloorMatches_RealGitCallSurvivesNormalization(t *testing.T) {
	text := "git -c advice.graftFileDeprecated=false log --oneline"
	if matches := FindFloorMatches(text); len(matches) != 0 {
		t.Fatalf("%q: expected no floor match (git log is not on the floor), got %+v", text, matches)
	}
}

func TestFindFloorMatches_RecordedExclusions_NoMatch(t *testing.T) {
	cases := []string{
		"recovery: git merge --abort",
		"run `git worktree prune` to clean up stale administrative data",
		"add to .beads/.gitignore and run `git rm --cached <file>`",
		"push with the lease-guarded form: git push --force-with-lease origin main",
		"or: git push --force-if-includes origin main",
		// The fifth recorded exclusion (O2-r2-13/S1-r2-5): mindspec's
		// own destructive verbs are outside this floor's {git,bd,rm}
		// program set. release.go's own recovery line is the most
		// consequential example.
		"commit them (git add -A && git commit) then re-run `mindspec release %s`, or discard them by re-running with `mindspec release %s --force`",
	}
	for _, text := range cases {
		t.Run(text, func(t *testing.T) {
			if matches := FindFloorMatches(text); len(matches) != 0 {
				t.Fatalf("%q: expected NO floor match (recorded exclusion), got %+v", text, matches)
			}
		})
	}
}

// TestFindFloorMatches_ResetRestore_DeliberateOverMatch pins O2-r2-12:
// reset/restore are matched at SUBCOMMAND grain — deliberately
// over-inclusive, per R5(a)'s own unqualified spelling of these two
// families (unlike its flag-qualified neighbors) — so a non-discarding
// spelling still matches. This is a recorded, tested DECISION, not an
// accidental over-match a future author should "fix".
func TestFindFloorMatches_ResetRestore_DeliberateOverMatch(t *testing.T) {
	cases := []struct {
		name string
		text string
		fam  DestructiveFamily
	}{
		{"reset-bare", "git reset", FamilyGitReset},
		{"reset-soft", "git reset --soft HEAD~1", FamilyGitReset},
		{"reset-index-only", "git reset HEAD -- file.txt", FamilyGitReset},
		{"restore-staged", "git restore --staged file.txt", FamilyGitRestore},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			matches := FindFloorMatches(c.text)
			if len(matches) != 1 || matches[0].Family != c.fam {
				t.Fatalf("%q: expected exactly one %s match (deliberate over-match), got %+v", c.text, c.fam, matches)
			}
		})
	}
}

// TestFindFloorMatches_AlternateFlagSpellings pins O1-r2-1: four
// families' documented alternate spellings of their OWN qualifying
// flag are genuine matches, not silent misses, and the family's
// SAFE spelling (`git branch --delete` alone) stays a non-match.
func TestFindFloorMatches_AlternateFlagSpellings(t *testing.T) {
	positive := []struct {
		name string
		text string
		fam  DestructiveFamily
	}{
		{"bd-delete-short", "bd delete mindspec-a.1 -f", FamilyBdDeleteForce},
		{"switch-force-short", "git switch -f main", FamilyGitSwitchDiscardChanges},
		{"switch-force-long", "git switch --force main", FamilyGitSwitchDiscardChanges},
		{"branch-delete-force-long", "git branch --delete --force bead/x", FamilyGitBranchDeleteForce},
		{"push-delete-short", "git push -d origin bead/x", FamilyGitPushDelete},
	}
	for _, c := range positive {
		t.Run(c.name, func(t *testing.T) {
			matches := FindFloorMatches(c.text)
			found := false
			for _, m := range matches {
				if m.Family == c.fam {
					found = true
				}
			}
			if !found {
				t.Fatalf("%q: expected %s, got %+v", c.text, c.fam, matches)
			}
		})
	}
	if matches := FindFloorMatches("git branch --delete bead/x"); len(matches) != 0 {
		t.Fatalf("expected git branch --delete ALONE (the safe delete) to NOT match, got %+v", matches)
	}
}

// TestFindFloorMatches_ClauseBoundary_NoCrossFold pins O1-r2-3: a
// sentence/clause boundary ('.', ':', '!', '?', ';', ',') stops the
// per-family flag search — a destructive trigger word deep in a
// LATER, unrelated clause is never folded into an earlier, unrelated
// invocation — while a real invocation's own operand list (however
// long) is still scanned to its true end (TestZZProbe_F1_1 in
// zzprobe_test.go pins the unbounded half over an actual 12-ID `bd
// delete` string).
func TestFindFloorMatches_ClauseBoundary_NoCrossFold(t *testing.T) {
	noMatch := []string{
		"Use `git stash push` to park work in progress; never `drop` a stash you did not create.",
		"After `git checkout main`, rebase -- and only then open the PR.",
		"`git branch` lists local branches; only the orchestrator may -D one.",
		"Never `rm` a worktree by hand: -r and -f are how people lose uncommitted work.",
		"git push origin main. Then, if the remote rejects, --force is not allowed.",
	}
	for _, text := range noMatch {
		t.Run(text, func(t *testing.T) {
			if matches := FindFloorMatches(text); len(matches) != 0 {
				t.Fatalf("%q: expected no match (clause boundary), got %+v", text, matches)
			}
		})
	}
	// The existing positive table (TestFindFloorMatches_OnePerFamily)
	// must stay green — clause-boundary detection must not touch a
	// same-clause invocation.
	if !IsFloorMatch("recovery: git stash drop") {
		t.Fatal("expected git stash drop to still match (no false clause boundary within one invocation)")
	}
}

// TestFindFloorMatches_QuotedGlobalOptionOperand pins G1-r2-2/
// O1-r2-9, then bead-2 rework round 2's confirm (G1-2/O1-confirm-1/
// O1-9): a shell-quoted, whitespace-bearing global-option operand is
// one shell argument and must normalize as exactly one token, whatever
// its interior content — never leaking a later word in as the
// subcommand (the original under-match this test pinned), and never
// letting an embedded bare "reset"/"restore"/flag-shaped WORD inside
// the quotes be misread as the subcommand either (the round-2-confirm
// over-match: `git -c "user.name=Set to reset later" status` must
// stay a NON-match for FamilyGitReset — "status" is the real
// subcommand, never reached by round 1's greedy-token heuristic).
//
// Round 3 (RULING 3/O1-confirm2-1) widened the covered flags: round 2
// wired this quote-awareness to -C/-c alone, leaving the fused
// `=`-operand globals (--git-dir=/--work-tree=/--exec-path=) on a
// separate, quote-oblivious path that under-matched the moment their
// operand was quoted and whitespace-bearing. globalOptionsWithOperand
// (classifier.go) now derives ALL five quote-bearing globals from one
// list, so the fused-flag fixtures below exercise the SAME
// consumeGlobalOptionOperand path as -C/-c, not a separate mechanism.
func TestFindFloorMatches_QuotedGlobalOptionOperand(t *testing.T) {
	positive := []struct {
		name string
		text string
		fam  DestructiveFamily
	}{
		{"dash-C-quoted-space", `git -C "/tmp/work tree" reset --hard HEAD`, FamilyGitReset},
		{"dash-c-quoted-space", `git -c "user.name=A B" reset --hard`, FamilyGitReset},
		// The genuinely destructive command G1-2 found round 1 still
		// missed: a quoted -C operand that happens to START WITH '-'.
		// Round 1's greedy scan misread the operand itself as the
		// stopping "flag-shaped" token; a single-token consume has no
		// such heuristic to fool.
		{"dash-C-quoted-leading-dash", `git -C "-tmp path" reset --hard HEAD`, FamilyGitReset},
		// A quoted value containing an embedded git-subcommand-shaped
		// or short-flag-shaped WORD must not fool the single-token
		// consume either (it is swallowed whole, not re-scanned).
		{"dash-c-quoted-embedded-flag-value", `git -c 'core.pager=less -R' reset --hard`, FamilyGitReset},
		// Nested quotes (a single quote inside a double-quoted span,
		// or vice versa) stay one token — the inner quote character is
		// literal content, not a new span.
		{"dash-c-nested-quotes", `git -c "alias.st='status --short'" reset --hard`, FamilyGitReset},
		// An escaped quote of the SAME kind as the span's own
		// delimiter does not close the span early.
		{"dash-c-escaped-quote", `git -c "note=say \"hi\" then reset" reset --hard`, FamilyGitReset},
		// A literal newline inside the quoted span is ordinary
		// content, not a token/clause boundary.
		{"dash-C-quoted-newline", "git -C \"/tmp/work\ntree\" reset --hard", FamilyGitReset},
		// Spec 127 bead-2 rework round 3, RULING 3/O1-confirm2-1
		// (BLOCKING): the FUSED `=`-operand globals get the same
		// quote-awareness as -C/-c, not a separate, narrower path.
		{"git-dir-equals-quoted-space", `git --git-dir="/path with spaces" reset --hard`, FamilyGitReset},
		{"work-tree-equals-quoted-space", `git --work-tree="/my work tree" clean -f`, FamilyGitCleanForce},
	}
	for _, c := range positive {
		t.Run(c.name, func(t *testing.T) {
			matches := FindFloorMatches(c.text)
			found := false
			for _, m := range matches {
				if m.Family == c.fam {
					found = true
				}
			}
			if !found {
				t.Fatalf("%q: expected %s after normalization, got %+v", c.text, c.fam, matches)
			}
		})
	}

	negative := []string{
		// O1-confirm-1/G1-2's over-match: a quoted -c VALUE containing
		// the bare word "reset"/"restore" must never be read as the
		// subcommand — the real subcommand (status/log) is what
		// matters, and it is not on the floor.
		`git -c "user.name=Set to reset later" status`,
		`git -c "alias.info=explains reset semantics" log`,
		// A quoted operand containing an entire destructive-looking
		// command as inert TEXT must not match — it is one opaque
		// token, never re-tokenized.
		`git -C "reset --hard" status`,
		// Spec 127 bead-2 rework round 3, RULING 3/O1-confirm2-2
		// (BLOCKING): when the resolved "subcommand" position (here
		// "status") fails to match any family, the outer scan must NOT
		// fall back to re-examining the quoted span's own interior
		// tokens ("git", "reset", "--hard") one at a time as fresh
		// candidates — they are inert text inside a -c operand's quoted
		// value, not a second invocation.
		`git -c "note=do not run: git reset --hard" status`,
	}
	for _, text := range negative {
		t.Run(text, func(t *testing.T) {
			if matches := FindFloorMatches(text); len(matches) != 0 {
				t.Fatalf("%q: expected NO floor match (quoted operand content is inert), got %+v", text, matches)
			}
		})
	}
}

// TestFindFloorMatches_UnbalancedQuotedGlobalOptionOperand pins the
// unbalanced-quote direction of consumeGlobalOptionOperand
// (classifier.go) AS IT STANDS AFTER spec 127 bead-2 rework round 4
// (RULING 2/G1-confirm3-1): an unbalanced quote gives no sound operand
// boundary, so this is UNRESOLVED rather than a guess in either
// direction — no subcommand is ever read out of the displaced tokens.
// This particular fixture (a two-word malformed operand,
// "/tmp/work tree") is a non-match for the same underlying reason as
// the single-word cases below: the unresolved boundary means nothing
// past the flag is ever tried as a subcommand candidate, regardless of
// how many words the malformed operand happens to span.
func TestFindFloorMatches_UnbalancedQuotedGlobalOptionOperand(t *testing.T) {
	if matches := FindFloorMatches(`git -C "/tmp/work tree reset --hard`); len(matches) != 0 {
		t.Fatalf("expected no match (unbalanced quote boundary is unresolved, not a guess), got %+v", matches)
	}
}

// TestFindFloorMatches_UnbalancedQuoteIsUnresolved is spec 127 bead-2
// rework round 4's required fixture (RULING 2/G1-confirm3-1, BLOCKING;
// this test replaces round 3's TestFindFloorMatches_UnbalancedQuoteFailsTowardMatching,
// whose own name encoded a ruling that round 4 reversed).
//
// Round 3 (RULING 3/S1-confirm2-new-1) made the unbalanced-quote
// fallback consume exactly one token as the operand and then read
// WHATEVER TOKEN FOLLOWS as the subcommand — reasoning that failing
// toward a match was the only forbidden-evasion-proof direction, since
// round 2's swallow-to-end could hide a real destructive command
// sitting in plain text right after a malformed quote
// (`git -C "innocent\" reset --hard`, below). G1 proved that
// reasoning was ALSO wrong, empirically: honest prose containing a
// lone stray quote plus the word "reset" —
// `Documentation calls git -C "configuration reset behavior benign.`
// — tokenizes to the EXACT SAME shape (flag, one displaced bare word,
// then "reset") and round 3's fallback classified it as a destructive
// command. There is no signal, once a quote fails to close, that
// distinguishes the honest-prose case from the genuinely-destructive
// one; guessing either way is a coin flip dressed as a principle.
//
// The only honest answer is that the boundary is UNRESOLVED: NEITHER
// case matches. The genuinely-destructive case below is now a
// deliberate, ACKNOWLEDGED miss (documented in
// consumeGlobalOptionOperand's own doc comment) — not a silent one —
// traded for never manufacturing a false refusal on the honest-prose
// case, which this scan's precision contract forbids outright.
func TestFindFloorMatches_UnbalancedQuoteIsUnresolved(t *testing.T) {
	cases := []string{
		// The genuinely-destructive shape: a real, unquoted
		// `git reset --hard` sits in plain text right after the
		// malformed quote. This is the acknowledged miss — the
		// classifier cannot tell this apart from the honest-prose case
		// below, so it must not match either.
		`git -C "innocent\" reset --hard`,
		// G1-confirm3-1's exact probe: honest documentation with a lone
		// stray quote and the noun "reset". Must never match — this is
		// the direction the precision contract forbids.
		`Documentation calls git -C "configuration reset behavior benign.`,
		// The same false-match reproduced with a lone apostrophe global
		// (G1-confirm3-1's second reproduction).
		`Documentation calls git -c 'configuration reset behavior benign.`,
		// The same false-match reproduced with a fused, quote-bearing
		// `=`-operand global (G1-confirm3-1's third reproduction).
		`Documentation calls git --git-dir="configuration reset behavior benign.`,
	}
	for _, text := range cases {
		t.Run(text, func(t *testing.T) {
			if matches := FindFloorMatches(text); len(matches) != 0 {
				t.Fatalf("%q: expected NO match (an unbalanced quote's operand boundary is unresolved — neither swallowed nor promoted to a subcommand), got %+v", text, matches)
			}
		})
	}
}

// TestFindFloorMatches_DesignPrinciple_ProhibitionTextStillMatches pins
// the round-5/round-6 ruling directly (spec 127 R5(a) design
// principle): the classifier is polarity-blind BY DESIGN — a
// prohibition of raw merging matches exactly like an instruction to
// run it. Distinguishing the two is the exemption list's job (a
// review-level list), never the classifier's.
func TestFindFloorMatches_DesignPrinciple_ProhibitionTextStillMatches(t *testing.T) {
	text := "Never merge a bead branch with raw `git merge bead/<id>`"
	matches := FindFloorMatches(text)
	if len(matches) != 1 || matches[0].Family != FamilyGitMerge {
		t.Fatalf("expected the prohibition text to still match FamilyGitMerge (polarity-blind by design), got %+v", matches)
	}
}

func TestFindFloorMatches_CompoundLineFindsBoth(t *testing.T) {
	text := "mindspec complete <id> && git push --force"
	matches := FindFloorMatches(text)
	if len(matches) != 1 || matches[0].Family != FamilyGitPushForce {
		t.Fatalf("expected exactly one FamilyGitPushForce match, got %+v", matches)
	}
}

// TestFindFloorMatches_JoinedMultiIDBdDeleteForce pins F1-r2-1: the
// trailing-flag scan for `bd delete`'s own qualifying flag is
// unbounded (structural clause-boundary stop only, findArg above) —
// a joined multi-ID `bd delete <ids...> --force` line, exactly the
// shape internal/approve/plan.go's beadCreateFailure renders via
// strings.Join(created, " "), still matches at every width, including
// past the OLD fixed 8-token bound this regression class named.
func TestFindFloorMatches_JoinedMultiIDBdDeleteForce(t *testing.T) {
	ids := make([]string, 12)
	for i := range ids {
		ids[i] = "mindspec-ab0" + string(rune('0'+i%10)) + ".1"
	}
	for n := 1; n <= len(ids); n++ {
		cmd := "bd delete " + strings.Join(ids[:n], " ") + " --force"
		if !IsFloorMatch(cmd) {
			t.Fatalf("n=%d ids: expected a floor match for %q", n, cmd)
		}
	}
}

func TestIsFloorMatch(t *testing.T) {
	if !IsFloorMatch("git reset --hard HEAD~1") {
		t.Error("expected git reset --hard to be a floor match")
	}
	if IsFloorMatch("mindspec complete <bead>") {
		t.Error("expected mindspec complete to NOT be a floor match")
	}
	if IsFloorMatch("") {
		t.Error("expected the empty string to not be a floor match")
	}
}
