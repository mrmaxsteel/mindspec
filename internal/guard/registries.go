package guard

// Three disjoint, content-grain-keyed registries (spec 127 R5, bootstrap
// discipline G-r5-4/H-r6-5): membership is derived by scanning the
// pre-change tree (`main` at 09f62bd9) with the real
// internal/lint scan — never by recall — and every entry carries a
// rationale AND a testable obligation (an entry with only a rationale
// is on the same footing as an unregistered site). Verified: no
// diagnostic-emitting file this registry cites (internal/approve,
// internal/executor, cmd/mindspec, internal/next) differs between
// 09f62bd9 and this bead's base — `git diff 09f62bd9 HEAD --stat` over
// those trees is empty (bead 1 touched only internal/guard/outcome.go,
// internal/gitutil, internal/lifecycle/gitquery.go, and
// internal/executor/merge_golden_test.go — none of them diagnostic
// emitters), so scanning this worktree IS scanning 09f62bd9 for every
// site below. See internal/lint/testdata/destructive_seed_manifest.txt
// for the generation command and the reconciliation fixtures
// (internal/lint/destructive_guidance_test.go).

// DestructiveGuidanceAllowlistEntry is one site whose foldable
// diagnostic operand matches a reviewed floor family but is not yet
// routed through the evidence-carrying constructor. Keyed at content
// grain (repo-relative file + enclosing function/method + the exact
// matched text) — never by line number, which is advisory only.
type DestructiveGuidanceAllowlistEntry struct {
	File       string
	Func       string
	Detail     string
	Family     DestructiveFamily
	Rationale  string
	Obligation string
}

// DestructiveGuidanceAllowlist is seeded from the scan's own first run
// over the pre-change tree — the Background inventory's live emitter
// sites (spec 127 R5d) — and burned down by later beads as each
// converts its own site; the FINAL state (bead 7) is pinned to exactly
// one entry (release.go's labeled operator discard). The three merge
// producers (`CompleteBead`'s/`FinalizeEpic`'s MergeInto,
// `MergeBranch`) emit no strings and never seed (they are governed by
// R4, not R5).
var DestructiveGuidanceAllowlist = []DestructiveGuidanceAllowlistEntry{
	{
		File:       "internal/approve/plan.go",
		Func:       "beadCreateFailure",
		Detail:     "bd delete %s --force",
		Family:     FamilyBdDeleteForce,
		Rationale:  "the partial-bead-create recovery — created is BY CONSTRUCTION the partial set this very run created (a provenance-carrying deletion, F1-2), but this spec's constructor requires a DestructionOutcome value, and this site has none to give it yet. Seeded here; converted by bead 5 (R3c: both plan.go sites route through the constructor, neither stays on the allowlist).",
		Obligation: "plan_test.go's existing TestBeadCreateFailure-shaped coverage pins the emitted `bd delete <ids> --force` line; registries_test.go's TestDestructiveGuidanceAllowlist_ObligationsHold re-derives the same template and confirms it still matches FamilyBdDeleteForce, so drift in either direction is caught.",
	},
	{
		File:       "internal/approve/plan.go",
		Func:       "checkExistingBeadsSafety",
		Detail:     "bd delete %s --force",
		Family:     FamilyBdDeleteForce,
		Rationale:  "the closed-child case — the exact live defect this spec exists to fix (R3c, Background 'the live class, inventoried'): emitted UNCONDITIONALLY today, gated by no verified fact. Seeded here (the tree must stay green while bead 5 does the actual gating work); converted — not merely allowlisted — by bead 5, which routes this site through the constructor and gates it on positive provenance.",
		Obligation: "registries_test.go's TestDestructiveGuidanceAllowlist_ObligationsHold re-derives the template and confirms it still matches FamilyBdDeleteForce; bead 5's own AC-6 table test is the obligation that actually gates the behavior change.",
	},
	{
		File:       "internal/executor/mindspec_executor.go",
		Func:       "beadToSpecConflictFailure",
		Detail:     "git merge",
		Family:     FamilyGitMerge,
		Rationale:  "the post-conflict raw-merge recovery line at :1688 (`fmt.Sprintf(`git merge --no-ff -m \"Merge %s\" %s`, beadBranch, beadBranch)`) — its allowlisting rationale is OVERTURNED per this spec's own revision-2 ruling (C2-r2-1): reachability of the emitter and safety of the emitted command are different questions. Seeded here so the tree stays green; CONVERTED (not merely allowlisted) by bead 6, which replaces it with the R5(d) preflighted re-entry surface — no conflict emitter prints a raw `git merge` line at all once bead 6 lands.",
		Obligation: "registries_test.go's TestDestructiveGuidanceAllowlist_ContentRegeneration re-derives the classifier match and confirms it still matches FamilyGitMerge, so this entry cannot silently drift to describe a different command while bead 6 is pending.",
	},
	{
		File:       "internal/executor/mindspec_executor.go",
		Func:       "directMergeConflictFailure",
		Detail:     "git merge",
		Family:     FamilyGitMerge,
		Rationale:  "the post-conflict raw-merge recovery line at :1721 (spec→main leg, `\"git merge --no-ff \"+specBranch`) — same overturned rationale as beadToSpecConflictFailure above. The literal prefix alone (\"git merge --no-ff \", concatenated with the dynamic spec-branch operand) already matches the floor family regardless of the operand's value — flags are always literal, values are always dynamic, in this codebase's own convention. Seeded here; converted by bead 6 (the same re-entry surface covers both legs).",
		Obligation: "registries_test.go's TestDestructiveGuidanceAllowlist_ContentRegeneration re-derives the classifier match and confirms it still matches FamilyGitMerge.",
	},
}

// OpaqueOperandEntry is one call-site operand the scan cannot fold to
// a literal at all — no literal skeleton whatsoever, e.g. a runtime-
// built variadic spread or a bare function parameter with no literal
// component — registered rather than converted, per spec 127 R5c.
// Membership carries no final-count pin (unlike the allowlist): an
// entry stays registered for as long as its site remains genuinely
// unprovable; additions are red, but shrinking to zero is not a goal
// this spec sets.
type OpaqueOperandEntry struct {
	File       string
	Func       string
	Detail     string
	Rationale  string
	Obligation string
}

// OpaqueOperandRegistry is seeded from the scan's own first run over
// the pre-change tree. A site a later bead rewrites exits the
// registry in that bead (e.g. impl.go:850, converted by bead 4 — not
// seeded here because `Orphan.RecoveryCommand()`'s CURRENT body always
// renders "mindspec complete <bead>", which the classifier never
// matches; only once bead 4 makes it evidence-derived and sometimes
// destructive does that call site become interesting to this scan).
var OpaqueOperandRegistry = []OpaqueOperandEntry{
	{
		File:       "cmd/mindspec/bead_ready.go",
		Func:       "beadReadyCheckCmd",
		Detail:     "variadic-spread:",
		Rationale:  "the canonical example this spec's own R5c text names: readiness.Report.RecoveryCommands() builds its return slice from the runtime-resolved set of FAILING readiness signals (internal/validate/readiness/report.go:93-99) — no static fold can prove its contents, because which signals fail, and what each one's Recovery text says, is a property of the bead being checked, not of this source file.",
		Obligation: "registries_test.go's TestOpaqueOperandRegistry_RuntimeInventoryAgainstFloor enumerates every readiness signal's registered Recovery template (internal/validate/readiness's own signal catalog) and asserts NONE matches a reviewed floor family — the runtime string inventory this operand can ever actually produce, checked against the classifier.",
	},
	{
		File:       "internal/executor/mindspec_executor.go",
		Func:       "beadToSpecConflictFailure",
		Detail:     "arg3:unprovable",
		Rationale:  "the fourth NewFailure argument is `rerun`, a plain string parameter (the caller-supplied `mindspec complete <bead-id>` / `mindspec impl approve <spec-id>` re-invocation); its value originates at this function's TWO real callers, not at a literal in this file, so no fold can prove it here.",
		Obligation: "registries_test.go's TestOpaqueOperandRegistry_RerunCallers asserts the classifier finds no floor match in either real caller's actual rerun argument (`mindspec complete <bead>`, `mindspec impl approve <spec>`).",
	},
	{
		File:       "internal/next/ready_gate.go",
		Func:       "GateReadiness",
		Detail:     "variadic-spread:recovery",
		Rationale:  "same runtime-resolved readiness-signal shape as bead_ready.go's above, with two additional Sprintf-literal-template/plain-string entries appended before the spread — the WHOLE slice is opaque as a variadic spread even though those two appended entries are individually provable-safe.",
		Obligation: "registries_test.go's TestOpaqueOperandRegistry_RuntimeInventoryAgainstFloor covers the same readiness-signal catalog as the bead_ready.go entry (one catalog, one obligation, two call sites).",
	},
}

// KnownSiteExemptionEntry is one exact, reviewed floor match shipped
// in guidance today (spec 127 R5a). It claims only that this exact
// quoted string, at this exact surface, is known and was reviewed at
// spec time — no polarity claim, no prescriptive/descriptive claim.
type KnownSiteExemptionEntry struct {
	// Surface is a guidance-glob repo-relative file path, or a named
	// setup builder key (a skill-map key, "claudeMDManagedBlock", or
	// "agentsMDBlockTemplate").
	Surface string
	// Text is the exact matched line, trimmed of leading/trailing
	// whitespace — content grain, never a line number.
	Text string
	// Count is Text's occurrence count on Surface.
	Count int
	// Family is the floor family Text matches.
	Family DestructiveFamily
}

// KnownSitesExemptionList is the scan-derived, eight-entry enumeration
// (spec 127 R5a/H-r6-4) plus bead 7's four seed-only orchestrator-block
// entries (exited when that bead deletes the bypass block, per named
// bead-exit records — R5c's bootstrap discipline fixture (β)).
var KnownSitesExemptionList = []KnownSiteExemptionEntry{
	// The eight live entries — reconciled against this spec's
	// Background inventory at quoted-string grain (bootstrap
	// fixture (α)).
	//
	// TWO ADDITIONAL entries, found by RUNNING the scan rather than by
	// the Background's own recall (H-r6-4's own lesson, recurring): the
	// canonical skill-map surface key "ms-bead-cycle" — ms-bead-cycle is
	// NOT one of the 4 inlined lifecycle-gate skills in claude.go, so
	// its skillFiles() entry comes PURELY from the go:embed'd plugin
	// source (pluginmindspec.SkillFiles()) and mirrors
	// plugins/mindspec/skills/ms-bead-cycle/SKILL.md byte-for-byte —
	// but R5(c)'s canonical-surface leg sweeps skillFiles()'s FULL
	// returned map, not only the 4 inlined entries, so this surface
	// key needs its own exemption entries alongside the tracked-file
	// and plugin-file ones. Recorded here rather than silently
	// resolved, per this spec's own stated method (Background,
	// "derived by scan, not recall").
	{
		Surface: ".claude/skills/ms-bead-cycle/SKILL.md",
		Text:    "3. **Do NOT push, do NOT raw-merge.** The user-controlled push gate is at end-of-spec (after `/ms-impl-approve`), not per-bead. **Never merge a bead branch with raw `git merge bead/<id>`** — it bypasses `bd` closure, worktree cleanup, AND the panel gate (no git hook fires on merge commits; raw merge is the obvious gate workaround). Only `mindspec complete` merges bead branches (AGENTS.md § Bead-loop guardrails).",
		Count:   1,
		Family:  FamilyGitMerge,
	},
	{
		Surface: ".claude/skills/ms-bead-cycle/SKILL.md",
		Text:    "**Partial-failure rule (verbatim):** \"Don't proceed to the next bead if the merge failed mid-way (e.g. ADR check stopped between bd-close and the actual git merge). Resolve the failure first.\" Re-running `mindspec complete <id>` after a partial failure is the documented recovery path; the gate passes a deleted-branch / missing-worktree rerun through to `complete`'s own idempotent handling.",
		Count:   1,
		Family:  FamilyGitMerge,
	},
	{
		Surface: "plugins/mindspec/skills/ms-bead-cycle/SKILL.md",
		Text:    "3. **Do NOT push, do NOT raw-merge.** The user-controlled push gate is at end-of-spec (after `/ms-impl-approve`), not per-bead. **Never merge a bead branch with raw `git merge bead/<id>`** — it bypasses `bd` closure, worktree cleanup, AND the panel gate (no git hook fires on merge commits; raw merge is the obvious gate workaround). Only `mindspec complete` merges bead branches (AGENTS.md § Bead-loop guardrails).",
		Count:   1,
		Family:  FamilyGitMerge,
	},
	{
		Surface: "plugins/mindspec/skills/ms-bead-cycle/SKILL.md",
		Text:    "**Partial-failure rule (verbatim):** \"Don't proceed to the next bead if the merge failed mid-way (e.g. ADR check stopped between bd-close and the actual git merge). Resolve the failure first.\" Re-running `mindspec complete <id>` after a partial failure is the documented recovery path; the gate passes a deleted-branch / missing-worktree rerun through to `complete`'s own idempotent handling.",
		Count:   1,
		Family:  FamilyGitMerge,
	},
	{
		Surface: ".claude/skills/ms-impl-approve/SKILL.md",
		Text:    "Disclosed residual: a bead branch already raw-`git merge`d and then",
		Count:   1,
		Family:  FamilyGitMerge,
	},
	{
		Surface: "ms-impl-approve",
		Text:    "bead branch already raw-`git merge`d and then `bd close`d,",
		Count:   1,
		Family:  FamilyGitMerge,
	},
	{
		Surface: "ms-bead-cycle",
		Text:    "3. **Do NOT push, do NOT raw-merge.** The user-controlled push gate is at end-of-spec (after `/ms-impl-approve`), not per-bead. **Never merge a bead branch with raw `git merge bead/<id>`** — it bypasses `bd` closure, worktree cleanup, AND the panel gate (no git hook fires on merge commits; raw merge is the obvious gate workaround). Only `mindspec complete` merges bead branches (AGENTS.md § Bead-loop guardrails).",
		Count:   1,
		Family:  FamilyGitMerge,
	},
	{
		Surface: "ms-bead-cycle",
		Text:    "**Partial-failure rule (verbatim):** \"Don't proceed to the next bead if the merge failed mid-way (e.g. ADR check stopped between bd-close and the actual git merge). Resolve the failure first.\" Re-running `mindspec complete <id>` after a partial failure is the documented recovery path; the gate passes a deleted-branch / missing-worktree rerun through to `complete`'s own idempotent handling.",
		Count:   1,
		Family:  FamilyGitMerge,
	},
	{
		Surface: "claudeMDManagedBlock",
		Text:    "See **AGENTS.md § Bead-loop guardrails (mindspec)** for the canonical orchestrator rules and subagent prompt fences (only the cycle runs `mindspec complete`, after the panel gate passes; never raw `git merge bead/<id>`; one `git push` at end-of-spec; subagents make exactly one commit, tests must PASS). Surviving skills reference that section rather than re-stating it.",
		Count:   1,
		Family:  FamilyGitMerge,
	},
	{
		Surface: "agentsMDBlockTemplate",
		Text:    "- **Never merge a bead branch with raw `git merge bead/<id>`** — only `mindspec complete` merges. Raw merge bypasses `bd` closure, worktree cleanup, AND the panel gate (no git hook fires on automatic merge commits, so raw merge is the obvious gate workaround).",
		Count:   1,
		Family:  FamilyGitMerge,
	},

	// The four seed-only orchestrator-block entries — bead 7 deletes
	// `.claude/agents/spec-orchestrator.md`'s bypass block outright, so
	// these reconcile against Background's bypass-block bullet at
	// command-family grain (per J-r7-1's honest scope), not at exact
	// quoted-string grain like the eight above.
	{
		Surface: ".claude/agents/spec-orchestrator.md",
		Text:    "git stash drop",
		Count:   1,
		Family:  FamilyGitStashDropClear,
	},
	{
		Surface: ".claude/agents/spec-orchestrator.md",
		Text:    "git merge --no-ff bead/<bead-id> -m \"Merge bead/<id>: <summary>\"",
		Count:   1,
		Family:  FamilyGitMerge,
	},
	{
		Surface: ".claude/agents/spec-orchestrator.md",
		Text:    "git worktree remove <bead-worktree> --force",
		Count:   1,
		Family:  FamilyGitWorktreeRemoveForce,
	},
	{
		Surface: ".claude/agents/spec-orchestrator.md",
		Text:    "git branch -D bead/<bead-id>",
		Count:   1,
		Family:  FamilyGitBranchDeleteForce,
	},
}
