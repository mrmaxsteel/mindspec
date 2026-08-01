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
// converts its own site.
//
// BEAD 7 FINAL DISPOSITION (resolving the tension bead-2 rework round 1
// left recorded above, now corrected in place rather than left as an
// open either/or): spec.md's Allowlist paragraph (R5(d), ruled
// O1-r2-6/C2-r2-2, seed rule corrected per C-r4-9) reads as pinning
// bead 7's final allowlist to "exactly one entry — release.go's labeled
// operator discard" (obligation O2-r2-5). That literal count is
// UNREACHABLE, and not by omission: cmd/mindspec/release.go's own
// recovery text — `mindspec release <id> --force` — leads with the
// mindspec verb, not git/bd/rm, so FindFloorMatches on it returns []
// (verified empirically, both before and after bead 7's release.go
// split) and it can never be admitted to a scan-seeded allowlist
// without fabricating a finding the scan itself does not produce — the
// exact unfalsifiable-leftover-at-burn-down shape the seed rule exists
// to forbid. The other branch bead-2 left open — deliberately
// extending R5(a)'s floor with a `mindspec <verb> --force`-shaped
// family — is FORECLOSED, not merely undesirable: classifier.go's own
// header (O2-r2-13) already records, as a shipped, reviewed decision,
// that this floor's program set is closed to exactly {git, bd, rm} and
// that mindspec's own destructive verbs — naming
// `mindspec release <id> --force` specifically — "match no family here
// and never will while the program set stays closed to these three."
// Reopening that ruling to manufacture a seedable match would repeat
// the very defect R5(a)'s Non-Goals section names (a mechanism
// claiming more than a test can support) for a family this spec's own
// prior bead already declined to add. So: the reachable, DERIVED final
// state is ZERO entries — not a written literal, but the value
// `len(destructive_seed_manifest.txt)`'s `[destructive_guidance_
// allowlist]` section reconciles to once every seeded site converts
// (registries_test.go's fixture (β), TestBootstrapManifest_
// AllowlistRegistryIdentity) — and release.go's discard is instead
// covered by its OWN obligation, entirely outside this registry:
// cmd/mindspec/release_test.go's TestRunRelease_DiscardNeverInRecoveryLine
// asserts the rendered failure's machine-greppable `recovery: ` line
// never contains `--force`, and that the discard invocation appears
// only in the message body's separately-labeled operator-choice text —
// literally what O2-r2-5's obligation names, just proven by a
// dedicated test rather than by allowlist membership a real scan can
// never produce. AC-9(ii)'s literal "one entry" text is, on this
// evidence, itself the defect the registries.go comment it amends was
// already flagging for bead 7 to resolve; this comment records the
// resolution rather than silently diverging from it.
// The three merge producers (`CompleteBead`'s/`FinalizeEpic`'s
// MergeInto, `MergeBranch`) emit no strings and never seed (they are
// governed by R4, not R5).
// Both plan.go entries THIS COMMENT ORIGINALLY DESCRIBED (beadCreateFailure,
// checkExistingBeadsSafety) EXITED this allowlist in bead 5 (R3c): both
// call sites now route their `bd delete ... --force` line through
// guard.NewDestructiveCommand (see internal/approve/plan.go's
// closedChildDeletionRefusal and beadCreateFailure), so the operand is
// constructor-derived — provenance-exempt per R5(b) — and no longer a
// live allowlist finding. Their obligations lived on:
// internal/approve/plan_test.go's TestBeadCreateFailure_EmitsBdDeleteForceLine
// still pins beadCreateFailure's rendered line directly, and bead 5's own
// AC-6 table (internal/approve/plan_provenance_test.go) is
// checkExistingBeadsSafety's obligation. The seed manifest
// (internal/lint/testdata/destructive_seed_manifest.txt) drops both
// lines in the SAME commit per fixture (β)'s own documented discipline
// (TestBootstrapManifest_AllowlistRegistryIdentity's doc comment).
//
// Prior to bead 5, this comment cited a since-corrected Obligation-field
// defect (spec 127 bead-2's rework round, RULING 6/O1-r2-4/O2-r2-6/O3-r2-4):
// both entries previously cited "registries_test.go's
// TestDestructiveGuidanceAllowlist_ObligationsHold" (no such test was
// ever declared) and, for beadCreateFailure, an additional
// "plan_test.go's existing TestBeadCreateFailure-shaped coverage" (the
// nearest real test, TestCreateImplementationBeads_BDCreateFails, forces
// the FIRST bead-create call to fail, so `created` is empty and the
// partial-set `bd delete` line this entry was ABOUT was never even
// rendered) — fixed before either entry existed to see this exit.
// TestRegistryObligations_NamedTestsExist (registries_test.go) verifies
// mechanically, for every entry in all three registries, that every
// obligation names only tests that exist.
// Bead 6 (spec 127 R5(d)): BOTH seeded entries above this comment (their
// original text preserved in git history) EXIT the allowlist. Neither
// beadToSpecConflictFailure nor directMergeConflictFailure prints a
// `git merge` line — or any destructive-floor-matching text — any
// longer: the R5(d) conversion replaces both raw-merge recovery lines
// with a `--resolve-merge` re-entry invocation of the owning lifecycle
// verb (a `mindspec complete`/`mindspec impl approve` command, never
// git/bd/rm-led). The reachable state after this bead is therefore ZERO
// entries — and stays zero after bead 7 (see the FINAL DISPOSITION note
// above): release.go's discard is off this floor's program set by
// design and is never seeded here.
var DestructiveGuidanceAllowlist = []DestructiveGuidanceAllowlistEntry{}

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
		// Bead 6 (spec 127 R5(d)): RENAMED from "arg3:unprovable" — the
		// R5(d) conversion dropped this function's EmitCd/git-merge
		// arguments, so the recovery-line operand (still `rerun`, still
		// unprovable) shifted from guard.NewFailure's 4th argument to its
		// 2nd (index 1): `guard.NewFailure(b.String(), rerun+" "+
		// ResolveMergeFlag)`.
		File:       "internal/executor/mindspec_executor.go",
		Func:       "beadToSpecConflictFailure",
		Detail:     "arg1:unprovable",
		Rationale:  "the recovery-line argument is `rerun+\" \"+ResolveMergeFlag` — rerun is a plain string parameter (the caller-supplied `mindspec complete <bead-id>` / `mindspec impl approve <spec-id>` re-invocation); its value originates at this function's TWO real callers, not at a literal in this file, so no fold can prove it here. ResolveMergeFlag is a package const (`--resolve-merge`, never git/bd/rm-led).",
		Obligation: "registries_test.go's TestOpaqueOperandRegistry_RerunCallers asserts the classifier finds no floor match in either real caller's actual rerun argument (`mindspec complete <bead>`, `mindspec impl approve <spec>`).",
	},
	{
		// Bead 6 (spec 127 R5(d)): NEW — directMergeConflictFailure
		// gained a `rerun` parameter as part of the same conversion
		// (it previously had no rerun invocation at all, only a bare
		// `git branch -d` instruction). Identical shape to
		// beadToSpecConflictFailure's entry above.
		File:       "internal/executor/mindspec_executor.go",
		Func:       "directMergeConflictFailure",
		Detail:     "arg1:unprovable",
		Rationale:  "the recovery-line argument is `rerun+\" \"+ResolveMergeFlag` — rerun is a plain string parameter (the caller-supplied `mindspec impl approve <spec-id>` re-invocation), same shape as beadToSpecConflictFailure's entry above.",
		Obligation: "registries_test.go's TestOpaqueOperandRegistry_RerunCallers asserts the classifier finds no floor match in this function's real caller's actual rerun argument.",
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
	// internal/approve/impl.go's implOrphanRefusal EXITED this entry's
	// original shape in bead 4 (spec 127 R2): the bare `o.RecoveryCommand()`
	// call this entry described is GONE — implOrphanRefusal now renders
	// the derivation's output instead (lifecycle.EvaluateOrphanHint /
	// orphan_hints.go). It re-enters this registry immediately below
	// under a NEW detail key (a variadic spread of the derivation's
	// OrphanHint.Lines, not a bare method call), because that spread is
	// itself unprovable by this scan for a different reason — see that
	// entry's own rationale.
	{
		File:       "internal/approve/impl.go",
		Func:       "implOrphanRefusal",
		Detail:     "variadic-spread:hint.Lines",
		Rationale:  "`guard.NewFailure(msg, hint.Lines...)` — hint is a lifecycle.OrphanHint returned by implEvaluateOrphanHintFn (spec 127 R2's single hint derivation), a runtime-computed []string this file's fold rules cannot resolve at all (the same variadic-spread shape as bead_ready.go's report.RecoveryCommands()... above), and — unlike that entry — DeriveOrphanHint's own Lines CAN legitimately contain a destructive `git branch -D` line for two of its five outcomes (ancestor-of-main, superseded), so this entry cannot claim 'never matches a floor family' the way the other variadic-spread entries do. What it claims instead: every line this call site can ever receive that DOES match a floor family is EXACTLY the one reviewed, guard.NewDestructiveCommand-derived shape — never a raw or differently-shaped destructive string. internal/lint's provenance-tracing (isConstructorDerived) only walks the SAME enclosing function as the guard.NewFailure call for a checked-and-returned NewDestructiveCommand bind (R5(b)'s own stated limitation) — since the constructor call lives inside orphan_hints.go's deletionHint, several functions removed from this call site, the scan cannot trace across that boundary, so this operand is registered rather than proven provenance-exempt.",
		Obligation: "internal/lifecycle's TestDeriveOrphanHint_DestructiveLinesMatchOnlyReviewedShape (orphan_hints_test.go) enumerates every guard.DestructionOutcome over a representative beadBranch/specID battery and asserts any line matching a destructive floor family is byte-identical to `git branch -D <branch>` (FamilyGitBranchDeleteForce) and nothing else.",
	},
	{
		File:       "internal/approve/adopt.go",
		Func:       "adoptOrphanPresentRefusal",
		Detail:     "variadic-spread:lines",
		Rationale:  "`guard.NewFailure(msg, lines...)` — the SAME shape and SAME rationale as implOrphanRefusal's entry above: adoptOrphanPresentRefusal (R1(g)'s composite-incident refusal, spec 127 bead 4) spreads a []string built from lifecycle.EvaluateOrphanHintAgainstMain's OrphanHint.Lines (occasionally appending its own adopt-rerun invocation, a pure literal Sprintf template already provable elsewhere), which can legitimately carry a `git branch -D` line for the ancestor/superseded outcomes; this entry's own OLD arg1/arg2 shape (the bead-3 interim's hardcoded, always-non-destructive inspection lines) EXITED with this bead — see this registry's own bootstrap history for that shape's removal.",
		Obligation: "internal/lifecycle's TestDeriveOrphanHint_DestructiveLinesMatchOnlyReviewedShape (same obligation as implOrphanRefusal's entry above — both call sites spread the identical OrphanHint.Lines shape).",
	},
	// Bead 6 (spec 127 R5(d)): the two containment.EmitCd(...) entries
	// this comment used to describe (beadToSpecConflictFailure's
	// `EmitCd(specWtPath)`, directMergeConflictFailure's `EmitCd(root)`)
	// EXIT here — the R5(d) conversion dropped the `cd` recovery line
	// entirely: the --resolve-merge re-entry surface resolves its own
	// target worktree/branch (R5(d)(ii)), so the operator no longer
	// needs to `cd` anywhere first.
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

	// The 38 entries below are new in bead-2's SECOND rework round (spec
	// 127, O2c-1, BLOCKING): fixing foldExpr's fullyLiteral to actually
	// propagate through fmt.Sprintf, localBinds, and pkgConstFolder.Get
	// (previously all three silently discarded it, laundering a
	// partially-foldable operand to fullyLiteral=true) newly governs
	// every command-position `fmt.Sprintf` call with a substituted
	// argument that this rework-round-1 tree had wrongly exempted. Every
	// one leads with the literal verb "mindspec" — never git/bd/rm, this
	// floor's own closed program set — with an ID-typed, branch/ref-
	// typed, or path-typed value substituted in. Three shapes recur
	// (each entry below names which applies):
	//   - idrender-rendered: the value is idrender.Bead(id)/
	//     idrender.Spec(id), or a local (safeBeadID/safeBid) already
	//     bound to one — a validated ID renders byte-identically
	//     (idvalidate's grammar has no whitespace/tokenizer-separator
	//     character, so it is always exactly one token) and a malformed
	//     one is forced through strconv.Quote, which the classifier's
	//     own quote-aware tokenizer (bead-2 rework round 2, G1-2/O1-9)
	//     reads as a single, inert token regardless of its content;
	//   - idvalidate-guarded-raw: the value is a raw specID/epicID/
	//     parentID/beadID the enclosing function (or its own caller, at
	//     this file's established entry-validation convention) rejects
	//     via idvalidate.SpecID/BeadID before any of these Sprintf calls
	//     run — same single-token guarantee, no render step;
	//   - disclosed-residual: the value is NOT provably single-token
	//     (an operator-typed panel slug, a git ref/branch name, or a
	//     resolved filesystem path) — registered, not converted, with
	//     the residual gap stated honestly rather than overclaimed.
	// All 38 share one Obligation: TestOpaqueOperandRegistry_
	// MindspecVerbTemplatesAgainstFloor (registries_test.go) re-parses
	// each named site's REAL current source, extracts every fmt.Sprintf
	// template in that function/method, and asserts none matches under
	// a realistic-value battery — see that test's own doc comment for
	// why the battery is realistic-value rather than maximally
	// adversarial.
	{
		File:       "cmd/mindspec/bead_clarify.go",
		Func:       "beadClarifyCmd",
		Detail:     "arg1:unprovable",
		Rationale:  "idrender-rendered: `fmt.Sprintf(\"mindspec bead clarify %s --file <record.json>\", idrender.Bead(beadID))`.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "cmd/mindspec/panel.go",
		Func:       "panelCreateCmd",
		Detail:     "arg1:unprovable",
		Rationale:  "disclosed-residual: `fmt.Sprintf(\"use `+\"`panel create %s --spec <id> --target <ref>`\"+` for a gated panel, or `+\"`panel create %s --gate adhoc --target <ref>`\"+` for an ad-hoc panel\", slug, slug)` and a second Sprintf substituting `keys` (`strings.Join(config.PanelGateKeys, \", \")` — a fixed, code-defined 5-entry list, never user input). slug is validated by validatePanelSlug (rejects empty/\".\"/\"..\"/path separators/control bytes) but NOT whitespace or other printable content — an operator-chosen slug containing floor-shaped text (e.g. a slug literally spelled \"git reset --hard\") is a distinct, unexamined risk this obligation does not cover. Narrowing the claim rather than overclaiming it: this text is advisory guidance shown back to the SAME operator who named the slug, never executed by the product, and is reviewed at the moment it is read, same as any other diagnostic string.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "cmd/mindspec/panel.go",
		Func:       "findPanelRegistration",
		Detail:     "arg1:unprovable",
		Rationale:  "disclosed-residual: `fmt.Sprintf(\"mindspec panel create %s --spec <id> --target <ref>\", slug)` — same slug-typed residual as panelCreateCmd above (validatePanelSlug does not close whitespace/printable content).",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "cmd/mindspec/panel.go",
		Func:       "tallyExitAction",
		Detail:     "arg1:unprovable",
		Rationale:  "disclosed-residual: `fmt.Sprintf(\"re-run the panel (%s), then mindspec complete %s\", recreate, safeBeadID)`, where recreate is this function's own `mindspec panel create %s --bead %s --round %d` template plus optional `--spec %s`/`--target %s`/`--gate %s` fragments (spec 127 final review, mindspec-tyi3: the recovery now carries the matched registration's binding instead of a static placeholder argv) — a mindspec verb, never git/bd/rm-led. The ID-typed operands are idrender-rendered (idrender.Bead/idrender.Spec: a validated ID renders byte-identically, a malformed one is forced through strconv.Quote, which this floor's quote-aware tokenizer reads as one inert token) and target/gate go through escapeConfigValue + shellQuoteTarget, so the sole residual is the slug-typed one: same as panelCreateCmd above (validatePanelSlug does not close whitespace/printable content).",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "cmd/mindspec/reattest.go",
		Func:       "runReattest",
		Detail:     "arg1:unprovable",
		Rationale:  "idrender-rendered: every command-position Sprintf in this function substitutes `safeBeadID := idrender.Bead(beadID)` (line 189), bound once at function entry before any of these calls.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "cmd/mindspec/reattest.go",
		Func:       "reattestRefusalFailure",
		Detail:     "arg1:unprovable",
		Rationale:  "idrender-rendered: safeBeadID is this function's own PARAMETER, and its sole caller (runReattest) always passes idrender.Bead(beadID) — never a raw ID.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "cmd/mindspec/reattest.go",
		Func:       "reattestRefusalFailure",
		Detail:     "arg2:unprovable",
		Rationale:  "idrender-rendered: the second command-position Sprintf (`\"bd show %s --json ... mindspec reattest %s\"`-shaped recovery lines) also substitutes only safeBeadID — same guarantee as arg1 above.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "cmd/mindspec/release.go",
		Func:       "runRelease",
		Detail:     "arg1:unprovable",
		Rationale:  "idrender-rendered: every command-position Sprintf here substitutes `idrender.Bead(beadID)` inline (three call sites: the dirty-tree commit-and-rerun hint, the worktree-removal-failed hint, the return-to-open-failed hint). Bead 7 (spec 127 R5(d)/(e)): the dirty-tree site's recovery-line Sprintf no longer carries the `--force` discard tail — the discard moved into the MESSAGE body (arg0), a separately-labeled operator choice (registries.go's DestructiveGuidanceAllowlist doc comment explains why that operand needs no registry entry of its own: `mindspec release ... --force` matches no reviewed floor family at all, classifier.go's own {git,bd,rm}-program-set exclusion, O2-r2-13, so the scan's fold rules never flag arg0's static template here regardless of its dynamic porcelain-line content).",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "cmd/mindspec/repair.go",
		Func:       "repairSpecTitleRunE",
		Detail:     "arg1:unprovable",
		Rationale:  "idvalidate-guarded-raw: epicID is this function's own args[0], rejected via `idvalidate.BeadID(epicID)` at the top of the function (its own-arg gate comment: \"validate epicID BEFORE any bd argv embed\") before any of the four command-position Sprintf calls that substitute it run.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "cmd/mindspec/repair.go",
		Func:       "repairPhaseRunE",
		Detail:     "arg1:unprovable",
		Rationale:  "disclosed-residual: specID is this function's own args[0] and epicID is derived from it (`phase.FindEpicBySpecID(specID)`) — NEITHER is idvalidate'd inside this function before the command-position Sprintf calls that substitute them (`\"bd show %s\"`, `\"mindspec repair phase %s\"`). Unlike repairSpecTitleRunE's sibling command above, this function has no own-arg gate. Registered, not converted, pending that gate; the residual is stated honestly rather than assumed closed by analogy with the sibling command.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/approve/impl.go",
		Func:       "ApproveImpl",
		Detail:     "arg1:unprovable",
		Rationale:  "mixed idvalidate-guarded-raw/idrender-rendered: most command-position Sprintf calls in this function substitute specID, validated at the top of ApproveImpl via validate.SpecID (== idvalidate.SpecID, per this file's own comment) before any of them run; the remainder substitute safeBid := idrender.Bead(bid) (idrender-rendered).",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/approve/impl.go",
		Func:       "runOrphanObligationGate",
		Detail:     "arg1:unprovable",
		Rationale:  "idvalidate-guarded-raw: substitutes specID, reached only via ApproveImpl's own preflight after its top-of-function idvalidate.SpecID gate (this file's single entry-validation convention, not re-checked per helper).",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/approve/impl.go",
		Func:       "runWorktreeEnumerationLeg",
		Detail:     "arg1:unprovable",
		Rationale:  "idvalidate-guarded-raw: substitutes specID, same ApproveImpl-preflight convention as runOrphanObligationGate above.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/approve/impl.go",
		Func:       "implBranchIndeterminateRefusal",
		Detail:     "arg1:unprovable",
		Rationale:  "idvalidate-guarded-raw: substitutes specID, same ApproveImpl-preflight convention as runOrphanObligationGate above (spec 127 R3a's AC-4(ii) probe-error leg).",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/approve/impl.go",
		Func:       "implBranchMissingRefusal",
		Detail:     "arg1:unprovable",
		Rationale:  "idvalidate-guarded-raw: substitutes specID, same ApproveImpl-preflight convention as runOrphanObligationGate above (spec 127 R3a's AC-4(i) missing-branch leg, naming the R1 adopt invocation in full).",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/approve/impl.go",
		Func:       "implObligationRefusal",
		Detail:     "arg1:unprovable",
		Rationale:  "idvalidate-guarded-raw: substitutes specID, same ApproveImpl-preflight convention as runOrphanObligationGate above.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/approve/impl.go",
		Func:       "implObligationRefusal",
		Detail:     "arg2:unprovable",
		Rationale:  "idvalidate-guarded-raw: the second command-position Sprintf also substitutes specID — same guarantee as arg1 above.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/approve/plan.go",
		Func:       "closedChildPreserveRefusal",
		Detail:     "arg1:unprovable",
		Rationale:  "idrender-rendered: the command-position Sprintf here is `fmt.Sprintf(\"bd show %s --json   (inspect its recorded history before deciding)\", idrender.Bead(id))` — the same idrender-rendered discipline as this file's checkExistingBeadsSafety entry below (in_progress leg): id is bd-sourced (`bd list --parent`), never idvalidate'd at this rendering site. Spec 127 R3c: fires for a closed child whose durable evidence is completed-work OR ambiguous/unavailable — a `bd show --json` inspection command, never `bd delete`.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/approve/plan.go",
		Func:       "resolvePlanApprovePreflight",
		Detail:     "arg1:unprovable",
		Rationale:  "idvalidate-guarded-raw: substitutes specID, validated at ApprovePlan's own top-of-function gate before this preflight helper runs (this file's single entry-validation convention).",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/approve/plan.go",
		Func:       "resolveTargetEpic",
		Detail:     "arg1:unprovable",
		Rationale:  "idvalidate-guarded-raw: substitutes specID; this function itself imports and calls idvalidate for its own epic-resolution logic, and is reached only after ApprovePlan's own top-of-function specID gate.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/approve/plan.go",
		Func:       "ApprovePlan",
		Detail:     "arg1:unprovable",
		Rationale:  "idvalidate-guarded-raw: substitutes specID, THIS function's own top-of-function idvalidate.SpecID gate (the source convention every other plan.go/impl.go helper above relies on).",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/approve/plan.go",
		Func:       "planValidationFailure",
		Detail:     "arg1:unprovable",
		Rationale:  "idvalidate-guarded-raw: substitutes specID, reached only from ApprovePlan after its own gate (`fmt.Sprintf(\"mindspec plan approve %s\", specID)`).",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/approve/plan.go",
		Func:       "beadCreateFailure",
		Detail:     "arg1:unprovable",
		Rationale:  "idvalidate-guarded-raw: this function has TWO guard.NewFailure call sites. The len(created)==0 leg's OPAQUE command-position Sprintf (its own arg1) is `fmt.Sprintf(\"mindspec plan approve %s\", specID)`.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/approve/plan.go",
		Func:       "beadCreateFailure",
		Detail:     "arg2:unprovable",
		Rationale:  "idvalidate-guarded-raw: the OTHER call site's THIRD argument (arg2) is the SAME `fmt.Sprintf(\"mindspec plan approve %s\", specID)` template, following that call's arg1. This same function's genuine `bd delete %s --force` floor match is now constructor-derived (guard.NewDestructiveCommand, spec 127 R3c), not a raw allowlisted string; the two DestructiveGuidanceAllowlist entries this rationale used to cite were REMOVED (not converted-with-continuing-obligation) in bead 5 — see the allowlist's own doc comment above.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/approve/plan.go",
		Func:       "queryExistingChildren",
		Detail:     "arg1:unprovable",
		Rationale:  "idvalidate-guarded-raw: substitutes specID; parentID (the OTHER substituted value, in the message-position arg0, not this command position) is likewise ApprovePlan-gated.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/approve/plan.go",
		Func:       "checkExistingBeadsSafety",
		Detail:     "arg1:unprovable",
		Rationale:  "idrender-rendered: the OPAQUE command-position Sprintf here is `fmt.Sprintf(\"mindspec complete %s\", idrender.Bead(c.ID))`. This function no longer renders a `bd delete %s --force` Sprintf directly (bead 5 moved it into the separate closedChildDeletionRefusal helper, called from this function's closed leg) — that template is now constructor-derived (guard.NewDestructiveCommand, spec 127 R3c); the DestructiveGuidanceAllowlist entry this rationale used to cite was REMOVED (not converted-with-continuing-obligation) in bead 5 — see the allowlist's own doc comment above.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/complete/complete.go",
		Func:       "Run",
		Detail:     "arg1:unprovable",
		Rationale:  "mixed idrender-rendered/disclosed-residual: the majority of Run's unprovable command-position operands substitute safeBeadID := idrender.Bead(beadID) (bound once near the top of Run); a minority substitute a resolved filesystem path (root, checkPath — the main-checkout path this process itself resolved, never free-form text) or the epic-linkage-derived branch name (derived). None of the latter is exhaustively adversarial-tested by this obligation; registered rather than converted, the residual stated honestly.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/complete/complete.go",
		Func:       "adrDivergenceFailure",
		Detail:     "arg1:unprovable",
		Rationale:  "idrender-rendered: all three command-position Sprintf calls in this function substitute safeBeadID := idrender.Bead(beadID), bound once at function entry.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/complete/complete.go",
		Func:       "adrDivergenceFailure",
		Detail:     "arg2:unprovable",
		Rationale:  "idrender-rendered: same safeBeadID guarantee as arg1 above (the second of three near-identical recovery-line Sprintf calls).",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/complete/complete.go",
		Func:       "adrDivergenceFailure",
		Detail:     "arg3:unprovable",
		Rationale:  "idrender-rendered: same safeBeadID guarantee as arg1 above (the third of three near-identical recovery-line Sprintf calls).",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/complete/complete.go",
		Func:       "attestedRestoreFailure",
		Detail:     "arg1:unprovable",
		Rationale:  "disclosed-residual, off-floor by construction: `restoreCmd := fmt.Sprintf(\"git branch %s %s\", beadBranch, ne.SecondParent)` — a git-led command, but `git branch <name> <ref>` is ORDINARY branch creation; matchGit's \"branch\" case only matches with a `-D`/`--delete`+`--force` flag pair present, so no substitution of beadBranch/ne.SecondParent (a branch name and a git-produced hex commit SHA, never free-form text) can supply that pair without ITSELF being reviewed content — off this floor family's own matched shape, not merely unprovable by this scan's fold rules.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/complete/complete.go",
		Func:       "attestedRestoreFailure",
		Detail:     "arg2:unprovable",
		Rationale:  "idrender-rendered: the second command-position argument is `fmt.Sprintf(\"mindspec complete %s\", safeBeadID)`, safeBeadID := idrender.Bead(beadID).",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/complete/panel_advisory.go",
		Func:       "panelGate",
		Detail:     "arg1:unprovable",
		Rationale:  "idrender-rendered: this function's command-position Sprintf calls substitute idrender.Bead(beadID)/idrender-rendered locals, mirroring cmd/mindspec/panel.go's tallyExitAction sibling.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/complete/panel_advisory.go",
		Func:       "reconcilePendingRefutations",
		Detail:     "arg1:unprovable",
		Rationale:  "idrender-rendered: `fmt.Sprintf(\"mindspec complete %s\", idrender.Bead(beadID))`, the entirety of this function's refuse(msg) helper's second argument.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/executor/layout_guard.go",
		Func:       "mergeLayoutRegressionFailure",
		Detail:     "arg1:unprovable",
		Rationale:  "disclosed-residual, off-floor by construction: `fmt.Sprintf(\"git rebase %s %s\", targetRef, sourceRef)` — `git rebase` is not a family on this spec's closed floor (R5(a)'s AllFamilies has no rebase entry) at all, so no substitution of targetRef/sourceRef makes this TEMPLATE'S OWN leading command match; a theoretical residual (one of the two refs itself containing an embedded git/bd/rm floor token as free text) is not exhaustively tested here — both are this repo's own resolved git ref/layout-fingerprint labels, never operator-typed free text.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/executor/mindspec_executor.go",
		Func:       "MindspecExecutor.CompleteBead",
		Detail:     "arg1:unprovable",
		Rationale:  "idvalidate-guarded-raw: `fmt.Sprintf(\"mindspec complete %s\", beadID)` substitutes this function's own beadID parameter RAW (no idrender wrap) — but workspace.BeadBranch(beadID)/workspace.BeadWorktreeName(beadID), both called earlier in this same function and both erroring out on a malformed beadID (ADR-0042's composition-waist convention), gate every path that reaches this Sprintf.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/executor/mindspec_executor.go",
		Func:       "MindspecExecutor.FinalizeEpic",
		Detail:     "arg1:unprovable",
		Rationale:  "idvalidate-guarded-raw: `fmt.Sprintf(\"mindspec impl approve %s\", specID)` substitutes specID RAW, but THIS function's own composition-waist gate — `idvalidate.SpecID(specID)`, checked at the very top of FinalizeEpic with its own doc comment (\"a malformed specID must refuse before any composed worktree path is used\") — runs before any of the four command-position Sprintf calls that substitute it.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/lifecycle/finalize_orphans.go",
		Func:       "FinalizeOrphan.RecoveryCommand",
		Detail:     "arg0:unprovable",
		Rationale:  "mixed disclosed-residual/idrender-rendered: the \"finalize_branch\" case substitutes o.Branch (a branch name this file's own orphan scan resolved from `git for-each-ref`, never operator-typed free text) into `\"open a PR for %s and merge it (or delete the branch if it is superseded)\"`; the default case substitutes idrender.Spec(o.SpecID) into `\"mindspec impl approve %s\"`.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/next/guard.go",
		Func:       "DirtyTreeFailure",
		Detail:     "arg1:unprovable",
		Rationale:  "already-established mechanism: `fmt.Sprintf(\"%s && mindspec next\", containment.EmitCd(activeWorktree))` — containment.EmitCd's own worktree-path safety is the SAME mechanism this registry already covers three times above (internal/executor/mindspec_executor.go's two EmitCd entries, internal/guard/guard.go's checkCWDWithCache entry) via TestOpaqueOperandRegistry_EmitCdWorktreePathsAgainstFloor.",
		Obligation: "registries_test.go's TestOpaqueOperandRegistry_EmitCdWorktreePathsAgainstFloor (same obligation as the three existing EmitCd entries above) plus TestOpaqueOperandRegistry_MindspecVerbTemplatesAgainstFloor for the \"mindspec next\" literal suffix, which has no substitution at all.",
	},
	{
		File:       "internal/next/guard.go",
		Func:       "ClaimFailure",
		Detail:     "arg1:unprovable",
		Rationale:  "idrender-rendered: `fmt.Sprintf(\"mindspec next --spec %s   (re-run to auto-recover the worktree)\", idrender.Spec(specID))`.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/next/guard.go",
		Func:       "WorktreeSetupFailure",
		Detail:     "arg1:unprovable",
		Rationale:  "idrender-rendered: same shape as ClaimFailure above — `fmt.Sprintf(\"mindspec next --spec %s   (re-run detects the in-progress bead and auto-recovers the worktree)\", idrender.Spec(specID))`.",
		Obligation: mindspecVerbTemplateObligation,
	},

	// The ten entries below are spec 127 bead 3's own new adopt surface
	// (internal/approve/adopt.go): every recovery/message line is a
	// non-destructive `mindspec`/`git diff`/`git log`/`bd list`-led
	// Sprintf whose ONLY destructive-floor line — the stale-branch
	// deletion hint — is separately constructor-derived (provenance-
	// exempt, never registered here; see adoptStaleBranchPresentRefusal's
	// own comment for why its error path is a checked-and-returned
	// shape rather than a reassignment). All ten share the same
	// obligation as the "mindspec ..." template family above.
	{
		File:       "internal/approve/adopt.go",
		Func:       "AdoptSpec",
		Detail:     "arg1:unprovable",
		Rationale:  "idrender-rendered: the --reason-required refusal's message body substitutes idrender.Spec(specID), and specID is itself validate.SpecID-gated at this function's own entry before any Sprintf here runs. Also covers the fix-round open-lifecycle-bead refusal's recovery line (`fmt.Sprintf(\"mindspec complete %s\", idrender.Bead(lcID))`) — same idrender-rendered shape, a second call site collapsing to this key.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/approve/adopt.go",
		Func:       "AdoptSpec",
		Detail:     "arg2:unprovable",
		Rationale:  "fix round G1-B3-02: the review-state precondition refusal's SECOND recovery line, `fmt.Sprintf(\"mindspec impl adopt %s --reason \\\"<why>\\\"   (retry once the spec is in review mode)\", idrender.Spec(specID))` — idrender-rendered, same guarantee as arg1 above; the FIRST recovery line of that same call (\"mindspec complete <bead-id>   (close remaining lifecycle beads, then re-run)\") is a pure literal with no substitution and folds cleanly on its own.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/approve/adopt.go",
		Func:       "adoptCurrentBranchPresentRefusal",
		Detail:     "arg1:unprovable",
		Rationale:  "idrender-rendered/disclosed-residual: `fmt.Sprintf(\"spec %s's branch %s still exists and is current...\", idrender.Spec(specID), termsafe.Escape(specBranch))` — specBranch is a waist-composed `spec/<id>` branch name (scan (a)'s composition-helper allowlist covers its construction), termsafe-escaped for display.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/approve/adopt.go",
		Func:       "adoptStaleBranchPresentRefusal",
		Detail:     "arg1:unprovable",
		Rationale:  "idrender-rendered/disclosed-residual: the message body substitutes idrender.Spec(specID), termsafe.Escape(specBranch), and the DestructionOutcome's own String() (a small closed enum with no user input) — same shape as adoptCurrentBranchPresentRefusal above.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/approve/adopt.go",
		Func:       "adoptStaleBranchPresentRefusal",
		Detail:     "arg3:unprovable",
		Rationale:  "the constructor-derived deletion line's error-refusal FALLBACK path — `fmt.Sprintf(\"could not construct the stale-branch deletion recovery for %s: %v\", termsafe.Escape(specBranch), ctorErr)` — reached only when NewDestructiveCommand itself refuses (git branch -D is on the reviewed floor, so unreachable in practice); the SUCCESS path's deletion line is separately provenance-exempt, never registered.",
		Obligation: mindspecVerbTemplateObligation,
	},
	// adoptOrphanPresentRefusal's bead-3 interim arg1/arg2 entries
	// (the hardcoded, always-non-destructive "git diff main %s" +
	// "mindspec impl adopt ... --reason" lines) EXITED in bead 4: that
	// function no longer builds those two literals inline — it renders
	// lifecycle.EvaluateOrphanHintAgainstMain's derived OrphanHint.Lines
	// instead, a shape re-registered above (in the opaque-operand block
	// this same function's implOrphanRefusal-mirroring entry lives in)
	// under "variadic-spread:lines".
	{
		File:       "internal/approve/adopt.go",
		Func:       "adoptEvidenceErrorRefusal",
		Detail:     "arg1:unprovable",
		Rationale:  "idrender-rendered: the fail-closed retryable refusal's message body substitutes idrender.Spec(specID) plus a caller-supplied, already-rendered detail string (every caller of this shared renderer idrender/termsafe-escapes its own dynamic content before composing detail).",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/approve/adopt.go",
		Func:       "adoptEvidenceErrorRefusal",
		Detail:     "arg2:unprovable",
		Rationale:  "idrender-rendered: the retry-named-first recovery line substitutes idrender.Spec(specID) — same guarantee as arg1 above.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/approve/adopt.go",
		Func:       "adoptFinalize",
		Detail:     "arg1:unprovable",
		Rationale:  "fix round G1-B3-03 (idempotent finalize protocol): covers both new refusal call sites' single recovery line — the already-done-via-normal-path refusal (`fmt.Sprintf(\"bd show %s --json   (inspect the epic's existing done-state metadata)\", idrender.Bead(epicID))`) and the already-adopted refusal (`fmt.Sprintf(\"bd show %s --json   (inspect the recorded adopt audit marker)\", idrender.Bead(epicID))`) — both idrender-rendered, epicID already idvalidate.BeadID-gated at this function's own entry before either Sprintf runs.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/approve/adopt.go",
		Func:       "adoptRefusalFailure",
		Detail:     "arg1:unprovable",
		Rationale:  "idrender-rendered: the lattice-refusal message body, `fmt.Sprintf(\"%s: %s\", r.Marker, r.Detail)` — r.Marker is one of four pinned literal constants (adoptMarker*) and r.Detail is built by evaluateAdoptLattice from idrender/termsafe-escaped fragments.",
		Obligation: mindspecVerbTemplateObligation,
	},
	{
		File:       "internal/approve/adopt.go",
		Func:       "adoptRefusalFailure",
		Detail:     "arg2:unprovable",
		Rationale:  "idrender-rendered: covers the shared attestLine (`mindspec impl adopt %s --reason \"<why>\" --attest-unverified ...`, substituting idrender.Spec(specID) and the pinned adoptAttestTrigger string) and the per-class inspection lines (`git log --first-parent --merges main`, a literal with no substitution; `bd list --parent <epic-id> --status=%s`, substituting bead.AllStatuses(root)'s own computed status list — fix round O1-3, no longer a hardcoded 4-built-in literal — a project-controlled, non-adversarial set) — three call sites in this function collapse to this one key.",
		Obligation: mindspecVerbTemplateObligation,
	},

	// Spec 127 bead 6 (R4): the merge-destruction preflight's own refusal
	// builders, one pair per boundary layer (internal/executor's own
	// backstop and internal/lifecycle's ADR-0030 wrapper the §1-phase
	// verb-layer preflights consume — see each file's doc comment for why
	// the logic is independently duplicated rather than shared across
	// that boundary). All four share the identical shape: the recovery
	// line is `fmt.Sprintf("... %s \"<reason>\" ... — then %s",
	// AllowNetDeletionFlag, rerun)` — AllowNetDeletionFlag is a package
	// const (provable), but rerun is a plain string PARAMETER (the
	// caller-supplied `mindspec complete <bead-id>` / `mindspec impl
	// approve <spec-id>` re-invocation), the same opacity shape
	// beadToSpecConflictFailure's own "arg1:unprovable" entry above
	// already registers for an identical rerun parameter.
	{
		File:       "internal/executor/merge_preflight.go",
		Func:       "evidenceErrorRefusal",
		Detail:     "arg1:unprovable",
		Rationale:  "the recovery line's `rerun` operand is a plain string parameter supplied by this function's two callers (preflightMergeDestruction's own two callers, CompleteBead and FinalizeEpic) — no fold can prove it here.",
		Obligation: "registries_test.go's TestOpaqueOperandRegistry_MergePreflightRerunCallers asserts the classifier finds no floor match in every real caller's actual rerun argument, the same shape beadToSpecConflictFailure's entry above already registers via TestOpaqueOperandRegistry_RerunCallers.",
	},
	{
		File:       "internal/executor/merge_preflight.go",
		Func:       "destructionRefusal",
		Detail:     "arg1:unprovable",
		Rationale:  "same shape as evidenceErrorRefusal above — the recovery line's `rerun` operand is a plain string parameter, not a literal.",
		Obligation: "registries_test.go's TestOpaqueOperandRegistry_MergePreflightRerunCallers (same obligation as evidenceErrorRefusal's entry above).",
	},
	{
		File:       "internal/lifecycle/merge_preflight.go",
		Func:       "workDestructionEvidenceErrorRefusal",
		Detail:     "arg1:unprovable",
		Rationale:  "the §1-phase (verb-layer) counterpart of internal/executor's evidenceErrorRefusal above — same shape, a plain string `rerun` parameter supplied by this function's two callers (internal/complete and internal/approve's own §1 preflights).",
		Obligation: "registries_test.go's TestOpaqueOperandRegistry_MergePreflightRerunCallers asserts the classifier finds no floor match in every real caller's actual rerun argument.",
	},
	{
		File:       "internal/lifecycle/merge_preflight.go",
		Func:       "workDestructionRefusal",
		Detail:     "arg1:unprovable",
		Rationale:  "same shape as workDestructionEvidenceErrorRefusal above.",
		Obligation: "registries_test.go's TestOpaqueOperandRegistry_MergePreflightRerunCallers (same obligation as workDestructionEvidenceErrorRefusal's entry above).",
	},
	{
		// Spec 127 bead 6 (R5(d)): the --resolve-merge resumption
		// surface's "still conflicted, re-print steps" leg.
		File:       "internal/executor/merge_resumption.go",
		Func:       "stillConflictedRefusal",
		Detail:     "arg1:unprovable",
		Rationale:  "`guard.NewFailure(resolutionSteps(...), reentryHint)` — reentryHint is a plain string parameter (the caller-supplied `mindspec complete <bead-id> --resolve-merge` / `mindspec impl approve <spec-id> --resolve-merge` re-invocation), the same opacity shape as the merge-preflight rerun entries above.",
		Obligation: "registries_test.go's TestOpaqueOperandRegistry_MergeResumptionReentryHintCallers asserts the classifier finds no floor match in every real caller's actual reentryHint argument.",
	},
	{
		// Spec 127 bead 6 (R5(d)): resumeAwareMerge's own "resolved but
		// not completed" refusal leg (the operator has not passed
		// --resolve-merge yet).
		File:       "internal/executor/merge_resumption.go",
		Func:       "resumeAwareMerge",
		Detail:     "arg1:unprovable",
		Rationale:  "`guard.NewFailure(..., fmt.Sprintf(\"re-run with %s to finish it: %s\", ResolveMergeFlag, reentryHint))` — ResolveMergeFlag is a package const (provable), but reentryHint is the same plain string parameter as stillConflictedRefusal's entry above.",
		Obligation: "registries_test.go's TestOpaqueOperandRegistry_MergeResumptionReentryHintCallers (same obligation as stillConflictedRefusal's entry above — both receive reentryHint from the identical three real callers).",
	},
}

// mindspecVerbTemplateObligation is the shared Obligation text for
// every entry above added in bead-2's second rework round (O2c-1):
// one real test, internal/guard/registries_test.go's
// TestOpaqueOperandRegistry_MindspecVerbTemplatesAgainstFloor, covers
// all of them by re-parsing each named site's actual current source.
const mindspecVerbTemplateObligation = "internal/guard's TestOpaqueOperandRegistry_MindspecVerbTemplatesAgainstFloor re-parses this site's real, current source (never a hand-copied literal), extracts every fmt.Sprintf template found in the named function/method, and asserts none matches a destructive floor family under a realistic-value substitution battery — see that test's own doc comment for why the battery is realistic-value rather than a maximal adversarial fuzz."

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
// spec's own undercount lesson, spec 127 bead-2 rework RULING 7). Prior
// to bead 7 this list ALSO carried four seed-only orchestrator-block
// entries (fourteen total) that existed only because
// .claude/agents/spec-orchestrator.md's raw bypass block was still
// live; bead 7 deletes that block outright (R5(e)) and exits all four
// entries in the same commit (see the exit note at the end of this
// slice literal) — ten live entries, ten total, is the state from bead
// 7 onward. KnownSitesExemptionListLiveCount/
// KnownSitesExemptionListTotalCount below pin both halves so this
// count can never drift silently a fourth time;
// TestKnownSitesExemptionList_CountSentinel asserts
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

	// BEAD 7 EXIT (spec 127 R5(c)'s bootstrap discipline, fixture (β)):
	// the four seed-only orchestrator-block entries this section used to
	// carry — .claude/agents/spec-orchestrator.md's "git stash drop",
	// "git merge --no-ff bead/<bead-id> ...", "git worktree remove
	// <bead-worktree> --force", and "git branch -D bead/<bead-id>" — EXIT
	// here. Bead 7 deletes that file's raw bypass block outright (R5(e)):
	// the file no longer contains any of those four floor-matching
	// strings (grep-verified — see cmd/mindspec's AC-10(i) absence test
	// covering the same four command shapes at the same surface), so
	// there is nothing left on this surface for these entries to exempt.
	// destructive_seed_manifest.txt's [known_sites_exemption_list] drops
	// these same four lines in this SAME commit, per the manifest's own
	// documented discipline (TestBootstrapManifest_ExemptionListIdentity's
	// two-way comparison — a registry entry with no manifest line is red,
	// exactly like an addition). These four entries reconciled against the
	// Background's bypass-block bullet at command-family grain only
	// (J-r7-1) — never at the ten-live-entries' exact quoted-string grain
	// — so their exit is covered by fixture (β)'s registry-identity check
	// alone, not by fixture (α)'s Background reconciliation.
	//
	// KnownSitesExemptionList is therefore the TEN live entries above and
	// nothing else: TestKnownSitesExemptionList_CountSentinel pins
	// len(KnownSitesExemptionList) == 10 (derived from the slice, not a
	// hand-typed literal re-typed here) — never 14 again.
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
