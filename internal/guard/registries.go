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
// ORIGINALLY seeded site below. See
// internal/lint/testdata/destructive_seed_manifest.txt for the
// generation command and the reconciliation fixtures
// (internal/lint/destructive_guidance_test.go).
//
// The opaque-operand registry's seven newest entries (spec 127 bead-2
// rework, RULING 4/RULING 5) additionally cover
// cmd/mindspec/panel.go, internal/guard/guard.go, and
// internal/lifecycle/{orphans,stale_open}.go — sourced from THIS
// bead's rework base (885728f7), not the original 09f62bd9 seed, and
// added because the rework's own fixes (the deleted foldExpr trust
// boundaries; the partial-fold governance fix) exposed them as real,
// previously-masked findings, not because those five files changed.

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
// converts its own site. The reachable final state is ZERO entries,
// not one (spec 127 bead-2 rework, O2-r2-8/F1-r2-2: the spec/plan text
// pins bead 7's final allowlist to "exactly one entry — release.go's
// labeled operator discard", but cmd/mindspec/release.go:257-258's own
// recovery line — `mindspec release <id> --force` — leads with the
// mindspec verb, not git/bd/rm, so FindFloorMatches on it returns []
// (verified) and it can never be seeded by this scan-derived
// mechanism; seeding it anyway would be the exact unfalsifiable-
// leftover-at-burn-down shape the seed rule exists to prevent). This
// is a recorded spec/plan-level adjudication bead 7 inherits, not a
// bead-2 code change: either AC-9(ii)'s final count becomes zero with
// release.go's discard recorded as a labeled-operator-choice OUTSIDE
// the allowlist mechanism, or the floor is deliberately extended with
// a `mindspec <verb> --force`-shaped family before bead 7 dispatches.
// The three merge producers (`CompleteBead`'s/`FinalizeEpic`'s
// MergeInto, `MergeBranch`) emit no strings and never seed (they are
// governed by R4, not R5).
// Both plan.go entries' Obligation fields were corrected in spec 127
// bead-2's rework round (RULING 6/O1-r2-4/O2-r2-6/O3-r2-4): they
// previously cited "registries_test.go's
// TestDestructiveGuidanceAllowlist_ObligationsHold" (no such test was
// ever declared) and, for beadCreateFailure, an additional
// "plan_test.go's existing TestBeadCreateFailure-shaped coverage"
// (the nearest real test, TestCreateImplementationBeads_BDCreateFails,
// forces the FIRST bead-create call to fail, so `created` is empty
// and the partial-set `bd delete` line this entry is ABOUT is never
// even rendered). The corrected Obligation fields below name only
// tests that exist; TestRegistryObligations_NamedTestsExist
// (registries_test.go) now verifies that mechanically for every
// entry in all three registries, not just these two.
var DestructiveGuidanceAllowlist = []DestructiveGuidanceAllowlistEntry{
	{
		File:       "internal/approve/plan.go",
		Func:       "beadCreateFailure",
		Detail:     "bd delete %s --force",
		Family:     FamilyBdDeleteForce,
		Rationale:  "the partial-bead-create recovery — created is BY CONSTRUCTION the partial set this very run created (a provenance-carrying deletion, F1-2), but this spec's constructor requires a DestructionOutcome value, and this site has none to give it yet. Seeded here; converted by bead 5 (R3c: both plan.go sites route through the constructor, neither stays on the allowlist).",
		Obligation: "internal/approve/plan_test.go's TestBeadCreateFailure_EmitsBdDeleteForceLine pins the emitted `bd delete <ids> --force` line directly; registries_test.go's TestDestructiveGuidanceAllowlist_ContentRegeneration re-derives the same template and confirms it still matches FamilyBdDeleteForce, so drift in either direction is caught.",
	},
	{
		File:       "internal/approve/plan.go",
		Func:       "checkExistingBeadsSafety",
		Detail:     "bd delete %s --force",
		Family:     FamilyBdDeleteForce,
		Rationale:  "the closed-child case — the exact live defect this spec exists to fix (R3c, Background 'the live class, inventoried'): emitted UNCONDITIONALLY today, gated by no verified fact. Seeded here (the tree must stay green while bead 5 does the actual gating work); converted — not merely allowlisted — by bead 5, which routes this site through the constructor and gates it on positive provenance.",
		Obligation: "registries_test.go's TestDestructiveGuidanceAllowlist_ContentRegeneration re-derives the template and confirms it still matches FamilyBdDeleteForce; bead 5's own AC-6 table test is the obligation that actually gates the behavior change.",
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

	// The seven entries below are new in this bead's REWORK round
	// (spec 127 bead-2 rework, RULING 4/RULING 5): the first four were
	// invisible only because foldExpr's since-deleted RecoveryCommand/
	// containment.EmitCd "trusted helper" rules asserted PROOF they
	// lacked (O2-r2-2); the last three surfaced independently, once
	// fixed, from the S1-r2-1 partial-fold fix (a `+`-concatenation
	// whose destructive content — none, here — lives in an unfoldable
	// remainder is no longer silently exempted just because ITS OTHER
	// half is literal). None of the seven is destructive: each is
	// checked below against the classifier over representative real
	// values, not against a stub that fabricates them.
	{
		File:       "internal/approve/impl.go",
		Func:       "implOrphanRefusal",
		Detail:     "arg1:unprovable",
		Rationale:  "a bare `o.RecoveryCommand()` call — the deleted trust boundary asserted this call contributes no literal content, PROVEN, when in fact it is simply unresolved by this file's fold rules; Orphan.RecoveryCommand's own body is independently scanned by leg (ii) (see the orphans.go entry below) and is not destructive today.",
		Obligation: "registries_test.go's TestOpaqueOperandRegistry_RecoveryCommandTemplatesAgainstFloor covers Orphan.RecoveryCommand's actual rendered shape.",
	},
	{
		File:       "internal/executor/mindspec_executor.go",
		Func:       "beadToSpecConflictFailure",
		Detail:     "arg1:unprovable",
		Rationale:  "a bare `containment.EmitCd(specWtPath)` call — specWtPath is a resolved spec-worktree filesystem path, never free-form text.",
		Obligation: "registries_test.go's TestOpaqueOperandRegistry_EmitCdWorktreePathsAgainstFloor runs the REAL containment.EmitCd over representative worktree/root path shapes and asserts no floor match.",
	},
	{
		File:       "internal/executor/mindspec_executor.go",
		Func:       "directMergeConflictFailure",
		Detail:     "arg1:unprovable",
		Rationale:  "a bare `containment.EmitCd(root)` call — root is the repo root path, never free-form text.",
		Obligation: "registries_test.go's TestOpaqueOperandRegistry_EmitCdWorktreePathsAgainstFloor (same obligation as beadToSpecConflictFailure above).",
	},
	{
		File:       "internal/guard/guard.go",
		Func:       "checkCWDWithCache",
		Detail:     "arg1:unprovable",
		Rationale:  "a bare `containment.EmitCd(gs.ActiveWorktree)` call — ActiveWorktree is a resolved worktree filesystem path recorded in this package's own guard-state file, never free-form text.",
		Obligation: "registries_test.go's TestOpaqueOperandRegistry_EmitCdWorktreePathsAgainstFloor (same obligation as the two executor entries above).",
	},
	{
		File:       "internal/lifecycle/orphans.go",
		Func:       "Orphan.RecoveryCommand",
		Detail:     "arg0:unprovable",
		Rationale:  "the method's own return, `\"mindspec complete \" + idrender.Bead(o.BeadID)` — idrender.Bead is an unresolved call (a rendering helper this file cannot fold), so the whole expression is only PARTIALLY literal: the literal prefix alone is safe (a mindspec verb, not git/bd/rm), and a PATTERN-VALID bead ID (idvalidate.BeadID's own charset: lowercase alnum, hyphens, dots — no whitespace, so a single tokenRe token) can never itself spell a multi-token destructive command. An ALREADY-invalid BeadID reaching idrender.Bead's quoting fallback is a distinct, unexamined risk this obligation does not cover — narrowing the claim rather than overclaiming it (spec 127 bead-2 rework standing requirement).",
		Obligation: "registries_test.go's TestOpaqueOperandRegistry_RecoveryCommandTemplatesAgainstFloor checks the rendered \"mindspec complete <id>\" shape over representative PATTERN-VALID bead-ID strings.",
	},
	{
		File:       "internal/lifecycle/stale_open.go",
		Func:       "StaleOpenBead.RecoveryCommand",
		Detail:     "arg0:unprovable",
		Rationale:  "the same shape as Orphan.RecoveryCommand above — `\"mindspec complete \" + idrender.Bead(s.BeadID)`.",
		Obligation: "registries_test.go's TestOpaqueOperandRegistry_RecoveryCommandTemplatesAgainstFloor (same obligation as the orphans.go entry above).",
	},
	{
		File:       "cmd/mindspec/panel.go",
		Func:       "tallyExitActionNonBead",
		Detail:     "arg1:unprovable",
		Rationale:  "`\"re-run the panel: \" + cmd.String()`, where cmd is a strings.Builder assembled from this function's own `mindspec panel create %s --round <N+1> --spec <id>` template plus optional `--target %s`/`--gate %s` fragments — a mindspec verb, never git/bd/rm-led, but the Builder value itself is unresolved by this file's fold rules.",
		Obligation: "registries_test.go's TestOpaqueOperandRegistry_PanelRecreateRerunAgainstFloor checks representative renderings of this exact template shape.",
	},
}

// KnownSiteExemptionEntry is one exact, reviewed floor match shipped
// in guidance today (spec 127 R5a). It claims only that this exact
// quoted string, at this exact surface, is known and was reviewed at
// spec time — no polarity claim, no prescriptive/descriptive claim.
//
// Rationale/Obligation added in spec 127 bead-2's rework round
// (RULING 6/G1-r2-3): the discipline this list was always supposed to
// carry (rationale + a tested obligation, mirroring the allowlist and
// opaque registries above) had no fields to hold it at all, so an
// exemption entry could never be checked against a real fixture the
// way the OTHER two registries' entries are.
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
	// Rationale states why this exact quoted line is a known,
	// reviewed match — never a destructiveness or polarity claim
	// (R5a's design principle: the classifier is polarity-blind, so a
	// prohibition of raw merging matches exactly like an instruction
	// to run it — this field exists to record that the SITE, not the
	// verdict, was reviewed).
	Rationale string
	// Obligation names the real test(s) that would catch this entry
	// going hollow (its quoted text no longer matching Family),
	// stale (its quoted text no longer present on Surface), or
	// duplicated/undercounted.
	Obligation string
}

// exemptionHollowObligation is the shared Obligation text for every
// entry below: the three real, existing fixtures that together
// enforce non-hollowness, presence, and count for every entry in this
// list (spec 127 bead-2 rework, RULING 6). Factored into a constant
// so it is spelled identically everywhere rather than retyped (and
// potentially drifted) 14 times.
const exemptionHollowObligation = "internal/lint's TestDestructiveGuidanceSweep_ExemptionEntriesNotHollow re-matches this entry's Text against its recorded Family independent of the live scan (a hollow entry — one that never matched anything — REDs); TestBootstrapManifest_ExemptionListIdentity two-way-compares this list against the committed seed manifest (an addition with no manifest line, or a manifest line with no entry, REDs); TestBootstrapManifest_ContentRegeneration is fixture (γ) applied to this entry's manifest line."

// KnownSitesExemptionList is the scan-derived, TEN-entry live
// enumeration (spec 127 R5a/H-r6-4 named eight; running the scan
// itself — not recall — found two MORE under the canonical
// "ms-bead-cycle" skill-map surface key, the third recurrence of this
// spec's own undercount lesson, spec 127 bead-2 rework RULING 7)
// plus bead 7's four seed-only orchestrator-block entries (exited
// when that bead deletes the bypass block, per named bead-exit
// records — R5c's bootstrap discipline fixture (β)). Fourteen total —
// KnownSitesExemptionListLiveCount/KnownSitesExemptionListTotalCount
// below pin both halves so this count can never drift silently a
// fourth time; TestKnownSitesExemptionList_CountSentinel asserts
// len(KnownSitesExemptionList) against the derived total, never a
// hand-typed number.
var KnownSitesExemptionList = []KnownSiteExemptionEntry{
	// The ten LIVE entries — reconciled against this spec's
	// Background inventory at quoted-string grain (the eight
	// Background named) plus the two "ms-bead-cycle" skill-map-key
	// entries below, found by running the scan.
	{
		Surface:    ".claude/skills/ms-bead-cycle/SKILL.md",
		Text:       "3. **Do NOT push, do NOT raw-merge.** The user-controlled push gate is at end-of-spec (after `/ms-impl-approve`), not per-bead. **Never merge a bead branch with raw `git merge bead/<id>`** — it bypasses `bd` closure, worktree cleanup, AND the panel gate (no git hook fires on merge commits; raw merge is the obvious gate workaround). Only `mindspec complete` merges bead branches (AGENTS.md § Bead-loop guardrails).",
		Count:      1,
		Family:     FamilyGitMerge,
		Rationale:  "documented guidance PROHIBITING raw merge and directing the reader to `mindspec complete` instead — reviewed at spec time (R5a's Background inventory) as a known instance of the design principle that a guard on destructive guidance must not penalize the text that warns against the destructive act.",
		Obligation: exemptionHollowObligation,
	},
	{
		Surface:    ".claude/skills/ms-bead-cycle/SKILL.md",
		Text:       "**Partial-failure rule (verbatim):** \"Don't proceed to the next bead if the merge failed mid-way (e.g. ADR check stopped between bd-close and the actual git merge). Resolve the failure first.\" Re-running `mindspec complete <id>` after a partial failure is the documented recovery path; the gate passes a deleted-branch / missing-worktree rerun through to `complete`'s own idempotent handling.",
		Count:      1,
		Family:     FamilyGitMerge,
		Rationale:  "documented guidance DESCRIBING the partial-failure recovery path (mentions 'the actual git merge' in the course of explaining when to stop, never instructing a raw merge) — reviewed at spec time as a known descriptive instance.",
		Obligation: exemptionHollowObligation,
	},
	{
		Surface:    "plugins/mindspec/skills/ms-bead-cycle/SKILL.md",
		Text:       "3. **Do NOT push, do NOT raw-merge.** The user-controlled push gate is at end-of-spec (after `/ms-impl-approve`), not per-bead. **Never merge a bead branch with raw `git merge bead/<id>`** — it bypasses `bd` closure, worktree cleanup, AND the panel gate (no git hook fires on merge commits; raw merge is the obvious gate workaround). Only `mindspec complete` merges bead branches (AGENTS.md § Bead-loop guardrails).",
		Count:      1,
		Family:     FamilyGitMerge,
		Rationale:  "the plugin-mirror copy of the tracked ms-bead-cycle SKILL.md prohibition above — same reviewed text, a distinct swept surface (plugins/*/skills/**).",
		Obligation: exemptionHollowObligation,
	},
	{
		Surface:    "plugins/mindspec/skills/ms-bead-cycle/SKILL.md",
		Text:       "**Partial-failure rule (verbatim):** \"Don't proceed to the next bead if the merge failed mid-way (e.g. ADR check stopped between bd-close and the actual git merge). Resolve the failure first.\" Re-running `mindspec complete <id>` after a partial failure is the documented recovery path; the gate passes a deleted-branch / missing-worktree rerun through to `complete`'s own idempotent handling.",
		Count:      1,
		Family:     FamilyGitMerge,
		Rationale:  "the plugin-mirror copy of the tracked ms-bead-cycle SKILL.md partial-failure description above — same reviewed text, a distinct swept surface.",
		Obligation: exemptionHollowObligation,
	},
	{
		Surface:    ".claude/skills/ms-impl-approve/SKILL.md",
		Text:       "Disclosed residual: a bead branch already raw-`git merge`d and then",
		Count:      1,
		Family:     FamilyGitMerge,
		Rationale:  "documented guidance naming a residual failure MODE (a branch already raw-merged by some prior, out-of-band action) as a known disclosed limitation of the recovery procedure — not an instruction to raw-merge.",
		Obligation: exemptionHollowObligation,
	},
	{
		Surface:    "ms-impl-approve",
		Text:       "bead branch already raw-`git merge`d and then `bd close`d,",
		Count:      1,
		Family:     FamilyGitMerge,
		Rationale:  "the canonical skill-map builder's copy of the same disclosed-residual line above (setup.CanonicalGuidanceSurfaces' \"ms-impl-approve\" key) — the evaluated Go-literal form of the tracked SKILL.md file's text, a distinct swept surface from the tracked file itself.",
		Obligation: exemptionHollowObligation,
	},
	{
		Surface:    "ms-bead-cycle",
		Text:       "3. **Do NOT push, do NOT raw-merge.** The user-controlled push gate is at end-of-spec (after `/ms-impl-approve`), not per-bead. **Never merge a bead branch with raw `git merge bead/<id>`** — it bypasses `bd` closure, worktree cleanup, AND the panel gate (no git hook fires on merge commits; raw merge is the obvious gate workaround). Only `mindspec complete` merges bead branches (AGENTS.md § Bead-loop guardrails).",
		Count:      1,
		Family:     FamilyGitMerge,
		Rationale:  "one of the TWO entries this spec's own Background inventory missed (found by RUNNING the scan, not recall, H-r6-4's own lesson recurring): the canonical skill-map builder key \"ms-bead-cycle\" is NOT one of the four inlined lifecycle-gate skills in claude.go, so its setup.CanonicalGuidanceSurfaces() entry comes purely from the go:embed'd plugin source and mirrors plugins/mindspec/skills/ms-bead-cycle/SKILL.md byte-for-byte — but R5(c)'s canonical-surface leg sweeps skillFiles()'s FULL returned map, not only the four inlined entries, so this surface key needs its own entry.",
		Obligation: exemptionHollowObligation,
	},
	{
		Surface:    "ms-bead-cycle",
		Text:       "**Partial-failure rule (verbatim):** \"Don't proceed to the next bead if the merge failed mid-way (e.g. ADR check stopped between bd-close and the actual git merge). Resolve the failure first.\" Re-running `mindspec complete <id>` after a partial failure is the documented recovery path; the gate passes a deleted-branch / missing-worktree rerun through to `complete`'s own idempotent handling.",
		Count:      1,
		Family:     FamilyGitMerge,
		Rationale:  "the second of the two scan-found \"ms-bead-cycle\" skill-map entries above — same reasoning, the partial-failure description mirrored through the canonical builder.",
		Obligation: exemptionHollowObligation,
	},
	{
		Surface:    "claudeMDManagedBlock",
		Text:       "See **AGENTS.md § Bead-loop guardrails (mindspec)** for the canonical orchestrator rules and subagent prompt fences (only the cycle runs `mindspec complete`, after the panel gate passes; never raw `git merge bead/<id>`; one `git push` at end-of-spec; subagents make exactly one commit, tests must PASS). Surviving skills reference that section rather than re-stating it.",
		Count:      1,
		Family:     FamilyGitMerge,
		Rationale:  "the CLAUDE.md managed block's own prohibition ('never raw `git merge bead/<id>`') — reviewed at spec time as a known descriptive/prohibitive instance, evaluated via the setup builder test seam rather than a tracked file (no single tracked CLAUDE.md carries the literal; the block is templated into every project's file at setup time).",
		Obligation: exemptionHollowObligation,
	},
	{
		Surface:    "agentsMDBlockTemplate",
		Text:       "- **Never merge a bead branch with raw `git merge bead/<id>`** — only `mindspec complete` merges. Raw merge bypasses `bd` closure, worktree cleanup, AND the panel gate (no git hook fires on automatic merge commits, so raw merge is the obvious gate workaround).",
		Count:      1,
		Family:     FamilyGitMerge,
		Rationale:  "the AGENTS.md managed block's own prohibition — same reasoning as claudeMDManagedBlock above, a distinct templated surface.",
		Obligation: exemptionHollowObligation,
	},

	// The four seed-only orchestrator-block entries — bead 7 deletes
	// `.claude/agents/spec-orchestrator.md`'s bypass block outright, so
	// these reconcile against Background's bypass-block bullet at
	// command-family grain (per J-r7-1's honest scope), not at exact
	// quoted-string grain like the ten above.
	{
		Surface:    ".claude/agents/spec-orchestrator.md",
		Text:       "git stash drop",
		Count:      1,
		Family:     FamilyGitStashDropClear,
		Rationale:  "part of the orchestrator's historical bypass-block recipe (a raw command sequence the orchestrator once ran directly, pre-dating this spec's constructor discipline) — seeded so the tree stays green while the block still exists; bead 7 deletes the whole block outright, exiting this entry.",
		Obligation: exemptionHollowObligation,
	},
	{
		Surface:    ".claude/agents/spec-orchestrator.md",
		Text:       "git merge --no-ff bead/<bead-id> -m \"Merge bead/<id>: <summary>\"",
		Count:      1,
		Family:     FamilyGitMerge,
		Rationale:  "same historical bypass-block recipe as git-stash-drop above.",
		Obligation: exemptionHollowObligation,
	},
	{
		Surface:    ".claude/agents/spec-orchestrator.md",
		Text:       "git worktree remove <bead-worktree> --force",
		Count:      1,
		Family:     FamilyGitWorktreeRemoveForce,
		Rationale:  "same historical bypass-block recipe as git-stash-drop above.",
		Obligation: exemptionHollowObligation,
	},
	{
		Surface:    ".claude/agents/spec-orchestrator.md",
		Text:       "git branch -D bead/<bead-id>",
		Count:      1,
		Family:     FamilyGitBranchDeleteForce,
		Rationale:  "same historical bypass-block recipe as git-stash-drop above.",
		Obligation: exemptionHollowObligation,
	},
}

// KnownSitesExemptionListLiveCount/KnownSitesExemptionListTotalCount
// are derived, not hand-typed (spec 127 bead-2 rework, RULING 7): the
// live/seed-only split is computed once, here, from the slice itself,
// so the doc comment above and TestKnownSitesExemptionList_CountSentinel
// both read from the SAME derived number instead of each carrying its
// own copy that can drift out of step with the other (H-r6-4's own
// "eight, not five" lesson, now recurring a fourth time in this spec
// before this fix).
var (
	// KnownSitesExemptionListTotalCount is len(KnownSitesExemptionList).
	KnownSitesExemptionListTotalCount = len(KnownSitesExemptionList)
	// KnownSitesExemptionListLiveCount is the count of entries NOT on
	// the seed-only orchestrator surface (.claude/agents/spec-orchestrator.md).
	KnownSitesExemptionListLiveCount = func() int {
		n := 0
		for _, e := range KnownSitesExemptionList {
			if e.Surface != ".claude/agents/spec-orchestrator.md" {
				n++
			}
		}
		return n
	}()
)
