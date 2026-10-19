---
status: Draft
approved_at: ""
approved_by: ""
---
# Spec 130-panel-substrate-integrity: Panel Substrate Integrity

*Citation scope: code citations were verified against `main` at `8735fbfb` (v0.16.0) on 2026-10-19 in the spec-130 worktree. Line numbers are approximate; the named symbol is authoritative.*

## Goal

After this spec, the panel gate decides on facts that only the binary writes:

- `panel create` cannot move a reviewed SHA under existing verdicts, cannot lose audit records, and cannot register a bead panel that `complete` will not find.
- Staleness compares the reviewed subject, not the whole commit. An archive-only commit no longer makes a panel stale.
- A panel passes when every finding is fixed, refuted or deferred. The APPROVE count is not a decision input.
- Panel verbs (`create`, `rebind`, `refute`, `defer`, `abandon`) are the only writers of `panel.json`. The gate detects a hand-edit.
- `complete` and `impl approve` enforce the disposition completeness floor (Core 3).
- The landed binding records which panel SHA was reviewed, so the record survives finalize.

## Background

Provenance: spec 129 OQ-1 assigned #233's review-to-landing binding and the rest of #251 to spec 130. Spec 130 takes only #233 part (a), the durable reviewed-SHA key (R12). #233's reviewed-tree-landed check and #251 move to the spec-129 follow-on spec.

Defects, each verified on the v0.16.0 code:

- **A0 (same-round create).** `panel create --round N` over a round that has verdicts moves `reviewed_head_sha` to the tip. The old verdicts then pass. `cmd/mindspec/panel.go:143` only clamps `round <= 0`. `panel.Create` (`internal/panel/create.go:134`) never reads verdict files. Gate leg 6 (`internal/panel/gate.go:280`) then sees equal SHAs.
- **#238 (lost refutations).** `Create` builds a new `Panel{}` and overwrites `panel.json` (`create.go:135-176`). A round bump drops `refutations` and clears `abandoned`. A corrupt `panel.json` is overwritten without a warning.
- **#247 (BRIEF history).** No per-round BRIEF archive exists (this repo holds 31 hand-made `BRIEF-round-N.md` files). The "prior-round asks" region is a skill TODO (`create.go:40`) and goes stale.
- **#235 / #228 (whole-commit staleness).** Leg 6 compares whole-commit SHAs (`gate.go:280`). A commit that touches only `reviews/` or one `OWNERSHIP.yaml` line blocks and demands a re-panel. For non-bead panels, `nonBeadTargetRevParse` (`cmd/mindspec/panel.go:500`) resolves the spec branch, which contains the panel directory itself.
- **#242 (bead panel fails open).** `create --gate bead` without `--bead` writes `bead_id: null` and 6 reviewers. `complete` finds panels with `panel.ForBead` (`internal/complete/panel_advisory.go:179`), so it finds none and passes ("no registered panel → no gate").
- **#243 (wrong header).** `tally` prints `threshold 5/6` (`cmd/mindspec/panel.go:758`), but leg 9.5 blocks on any unresolved REQUEST_CHANGES first (`gate.go:353`). The header counts an unrecognised verdict as REQUEST_CHANGES.
- **#237 (one-slot panel).** With `expected_reviewers: 1`, the threshold is 0 and `gate.go:371` requires `threshold > 0`, so the panel blocks forever. Config already refuses fewer than 2 (`internal/config/config.go:707,783`), but its text says "always-pass", which is wrong. `panelCmd` has no `SilenceUsage`, so every Block prints cobra usage.
- **Slug hijack.** `findPanelRegistration` (`cmd/mindspec/panel.go:431`) returns the first `Slug()` match over a sorted scan that includes `.worktrees/*` (`cmd/mindspec/config.go:606`). A 0-verdict panel in spec 901 tallied as another spec's 6/6 PASS.
- **Silent threshold fallback.** `Panel.ApproveThreshold` (`internal/panel/panel.go:150`) turns a recorded `unanimous`, `7` or `0` into N−1 with no message.
- **Adhoc recovery.** `tallyExitActionNonBead` (`cmd/mindspec/panel.go:923`) always prints `--spec <id>`, which `create` refuses for `--gate adhoc` (`:135`), and prints the placeholder `<id>` when the spec is known.
- **Roster drift.** A re-panel takes the reviewer count from the current config, not from the prior round. Any `<slot>-round-N.json` file counts as a slot, so a stray `zz-extra` file made a 5-slot panel pass.
- **One-refutation cap.** One refuted RC clears 5/6. Two refuted RCs give 4/6 and block with "Run /ms-bead-fix", with nothing left to fix (`gate.go:370`).
- **Three stores.** Decisions live in `panel.json` (gate), `dispositions.jsonl` (telemetry only) and bead metadata. Skills tell the agent to hand-edit `panel.json` for refute and abandon (`plugins/mindspec/skills/ms-panel-tally/SKILL.md:109-119`, `ms-panel-run/SKILL.md:71`).
- **R8.4 retention.** Spec 129 R8.1 ignores all of `reviews/**`. Finalize removes the spec worktree, so `dispositions.jsonl` is lost. This reverses ADR-0043 Decision ("git-versioned") without citing it.
- **Core 3 unenforced.** `panel.CheckCompleteness` (`internal/panel/disposition_store.go:395`) has one caller, `panel disposition check` (`cmd/mindspec/panel_disposition_store.go:147`). `complete` (`internal/complete/complete.go:839-886`) and `impl approve` (`internal/approve/impl.go:876`) never call it.
- **#233 (a).** After finalize, the ignored `panel.json` is gone. Nothing durable records which SHA the panel reviewed.

Corrections to the drafting brief: the landed-binding write is `ensureLandedBinding` (`internal/executor/mindspec_executor.go:1736`, write at `:1771`), not `:1660`. `configShowReviewRoots` is in `cmd/mindspec/config.go`, not `internal/config`. No refutation "ground" enum exists today; R9.2 adds it as a new field. `reviewedHeadSHAForBead` reads `ReviewedHeadSHA` at `internal/lifecycle/landed.go:752`.

## Impacted Domains

- **workflow**:
  - `internal/panel/**`: create merge-forward, roster, subject derivation, gate leg 6, pass rule, rule-line render, ledger kinds, hand-edit check, verb writers.
  - `cmd/mindspec/**`: `panel create` flags, new `panel rebind|refute|defer|abandon`, slug resolution, `SilenceUsage`, tally and verify render.
  - `internal/complete/**`: subject-tree I/O, Core 3 call, `panel_deferred` audit, defer re-check, reviewed-SHA keys passed to the executor.
  - `internal/approve/**`: the Core 3 leg and the defer re-check in `impl approve`.
  - `internal/instruct/**`: panel-state render (`panelstate.go`).
  - `internal/lifecycle/**`: `reviewedHeadSHAForBead` includes rebind `from_sha` values.
  - `internal/bootstrap/**`: `mindspec init` writes the R6 negation lines.
  - `internal/setup/**`, `plugins/mindspec/skills/ms-panel-{run,tally}/SKILL.md`, `.claude/skills/ms-panel-{run,tally}/SKILL.md`: hand-edit text removed; skill-text pins.
- **core**: `internal/config/**` (per-gate default sizes, `panel.rebind_allowlist`, error text); `.gitignore` (R6).
- **execution**: `internal/executor/**` (R12 keys in the landed-binding write); `internal/gitutil/**` (`ls-tree` primitive, `ReviewIgnoreEntries` negations).

Documentation surfaces: ADR-0037, ADR-0043, ADR-0030 and ADR-0044 amendments (R13); `.mindspec/domains/workflow/interfaces.md` § Panel Artifact Schema; `CHANGELOG.md` (size change); spec 129 R8.4 status line.

## ADR Touchpoints

- [ADR-0037](../../adr/ADR-0037-panel-gate-enforced-contract.md) — AMENDED (R13.1):
  - §1: new `panel.json` fields (R1, R2, R3, R4, R9); the verbs are the only writers.
  - §3: the pass rule (R7) replaces the N−1 threshold and extends the Spec-114 amendment.
  - §4: subject-tree staleness and `panel rebind` (R3, R4).
  - Decision Details: the two-subprocess matched-path cap becomes three on a SHA mismatch (R3.3).
  - §7: abandon is a verb (R9.4).
  - §8: hand-edit detection (R5.3) detects accidental desync only. It is anti-footgun, not anti-adversary, and adds no signing.
- [ADR-0043](../../adr/ADR-0043-panel-disposition-telemetry-store.md) — AMENDED (R13.2):
  - Decision and Context §1: `dispositions.jsonl` is tracked again (R6).
  - Decision: new ledger record kinds (R5.1).
  - Consequences: `complete` and `impl approve` now gate on completeness (R11).
  - Amendment (Spec 126): the shipped default becomes 6/6/4/6 per gate (R2.1). This reverses spec 112 R2.
- [ADR-0030](../../adr/ADR-0030-executor-boundary.md) — AMENDED (R13.3): one extra `git ls-tree` on the mismatch path, through the `GateIO` seam. `internal/panel` stays free of git.
- [ADR-0035](../../adr/ADR-0035-agent-error-contract.md) — APPLIED: every new refusal ends with one `recovery:` line.
- [ADR-0041](../../adr/ADR-0041-gate-before-mutate.md) — APPLIED: each verb checks before it writes; Core 3 runs before materialization; the `impl approve` leg runs in preflight.
- [ADR-0044](../../adr/ADR-0044-panel-review-conduct.md) — APPLIED: conduct is unchanged. Its "threshold is a floor" sentence gets a back-pointer to R7 (R13.4).
- [ADR-0045](../../adr/ADR-0045-binary-owns-sequence-and-rules.md) — APPLIED: R5.4 and R9 move the refute and abandon procedures out of skills into verbs.

## Requirements

**Terms.** Each term has this meaning only.

- *Slot*: one roster entry. Its verdict file is `<slot>-round-<N>.json`.
- *Roster*: the ordered slot list of a round. Each slot has an id `R1..Rn`, a model and a lens.
- *Finding*: one blocking item of a REQUEST_CHANGES verdict. Item `i` of `concrete_changes_required` has the id `ccr[i]`. A verdict with an empty list has one finding, `ccr[0]`, whose text is its `rationale`.
- *Discharge*: a refutation or deferral record for one finding at the latest round.
- *Resolved slot*: a REQUEST_CHANGES slot whose every finding has a discharge.
- *INVALID*: a parseable verdict file whose verdict is not APPROVE, REQUEST_CHANGES or REJECT.
- *Subject*: the path set that R3.1 derives.
- *Ledger*: the panel directory's `dispositions.jsonl`.
- *Legacy panel*: a panel whose ledger has no `round` event.

### R1 — Create integrity

1. **Same-round refusal (A0).** `create` computes LatestRound from the verdict files with `verdictFileRE`. If a verdict file exists at `--round N` and N ≤ LatestRound, `create` refuses and writes nothing. The recovery names `--round <LatestRound+1>` and `mindspec panel rebind`. A round with zero verdict files stays allowed.
2. **Merge-forward (#238).** `create` reads the existing `panel.json` first. It carries `refutations`, `deferrals` and `rebinds` forward unchanged. It never removes an entry. A revival of an abandoned panel appends `{reason, abandoned_at, revived_at}` to `abandon_history` and clears `abandoned`.
3. **Unparseable refusal.** If `panel.json` exists and does not parse, `create` refuses and writes nothing.
4. **BRIEF archive (#247).** On a round bump, `create` copies `BRIEF.md` to `BRIEF-round-<old>.md`. It never overwrites an existing archive. The archive must end in `.md`, because `verdictFileRE` counts `*-round-N.json` as slots.
5. **Prior-round region.** The machine-owned BRIEF region gains a `<!-- mindspec:prior-round -->` block. `create` renders it from `consolidated-round-<N-1>.md` through `termsafe`. If that file is absent, the block says `none recorded`.
6. **Bead id (#242).** For `--gate bead`, `create` derives the bead id from a `bead/<id>` target and validates it with `idvalidate.BeadID`. `create` refuses: `--gate bead` with any other target form; `--bead X` with a target other than `bead/X`; `--bead` with a non-bead gate. A refusal writes no file.
7. **Slug ambiguity.** `findPanelRegistration` collects all matches. More than one match refuses unless `--spec` selects one. `tally` and `verify` print the resolved directory and round. `create` refuses a slug that is registered under another spec.
8. **Adhoc recovery.** `tallyExitActionNonBead` omits `--spec` for `--gate adhoc`. It prints the recorded spec id, not `<id>`, when the spec is known.
9. **Usage noise.** `panelCmd` sets `SilenceUsage: true`. The config errors at `config.go:707,783` say "a one-reviewer panel cannot pass", not "always-pass".

### R2 — Panel size and roster

1. **Per-gate defaults.** With no `panel.reviewers` and no `panel.gates.<gate>.reviewers` in config, the size is: `spec_approve` 6, `plan_approve` 6, `bead` 4, `final_review` 6, `adhoc` 6. The binary distributes the size round-robin over the configured families in config order. Today's codex→claude substitution applies unchanged. No family minimum exists. An explicit config value keeps spec-112 semantics.
2. **Floor.** A panel has at least 2 slots. `create` refuses fewer. The gate reads a `panel.json` with fewer than 2 slots as a registration error (Block). Its recovery is `mindspec panel create <slug> --round <N+1> --reviewers 2`, never `/ms-bead-fix`. The threshold is never clamped to 1.
3. **`--reviewers N`.** `create` accepts `--reviewers N` with N ≥ 2. It refuses the flag when the target round already has a verdict file.
4. **Roster record.** `create` writes `roster: [{slot, model, lens}]` to `panel.json` and to the `round` ledger event, from `config.PanelGateReviewerSlots`. The BRIEF region lists the slot ids. A re-panel copies the prior round's roster unless `--reviewers` is given.
5. **Off-roster files.** When a roster is recorded, a verdict file whose slot is not on the roster is an *unexpected slot*. It counts for nothing, and `tally` names it. Legacy panels keep today's counting.

### R3 — Subject-tree staleness

1. **Derived subject.** One pure function `panel.Subject(gate, spec)` returns the roots and exclusions. It never reads the subject from `panel.json`.

   | Gate | Roots |
   |:-----|:------|
   | `spec_approve`, `plan_approve` | `.mindspec/specs/<spec>/` |
   | `bead`, `final_review`, `adhoc`, empty (legacy) | whole tree |

   Every subject excludes `**/reviews/**`, `.mindspec/reviews/**` and `.beads/issues.jsonl`.
2. **Digest.** The subject digest is `sha256:` plus the sha256 of `git ls-tree -r <target> -- <roots>` output, filtered in Go by the exclusions. (`ls-tree` rejects exclude pathspec magic.) `create` and `rebind` record it as `subject_digest`. Any `ls-tree` error refuses. An empty filtered listing refuses; the binary never hashes empty input.
3. **Gate leg 6.** `GateIO` gains `SubjectTree(scanRoot, ref, roots)`.

   | Recorded state | Rule |
   |:---------------|:-----|
   | SHA equals live tip | fresh; no extra call |
   | SHA differs, `subject_digest` recorded | one `SubjectTree` call; stale only if digests differ; an error is Block |
   | SHA differs, no `subject_digest` (legacy) | stale; today's message, byte-identical |

4. **Consumers.** These call sites pass `SubjectTree` and decide identically: `complete`'s `panelGate` (`panel_advisory.go:211`), `panel verify` and `panel tally` (`cmd/mindspec/panel.go:459-475`), and `instruct` panel state (`panelstate.go:123,270`).
5. **Landed set.** `reviewedHeadSHAForBead` adds each `rebinds[].from_sha` to its exact-equality set. Equality stays exact.

### R4 — `panel rebind`

1. `mindspec panel rebind <slug> --reason "<who>: <why>"` moves `reviewed_head_sha` and `subject_digest` to the live tip without a round bump. It refuses an empty reason.
2. It refuses unless all of these hold:
   - the panel parses, is not abandoned, and has no round mismatch;
   - the panel passes R7 when staleness is ignored (a rebind never turns a failing panel into a passing one);
   - the recorded SHA is an ancestor of the tip;
   - every path in `git diff --name-only <recorded>..<tip>`, minus the R3.1 exclusions, matches `panel.rebind_allowlist`.
3. `panel.rebind_allowlist` is a new config list. The default is `[".mindspec/domains/*/OWNERSHIP.yaml"]`. An empty list disables rebind. Matching uses `validate.GlobMatch`.
4. It writes the SHA, the digest and one `rebinds[]` entry `{round, from_sha, to_sha, paths, reason, at}` in one write, plus a `rebind` ledger event. If the SHA already equals the tip, it prints `already fresh` and writes nothing.
5. At `complete`, a bead panel's `rebinds[]` is copied to bead metadata `panel_rebound_entries`.
6. The tool-artifact path list (`.beads/issues.jsonl`) has one exported home. `complete`'s `materializedArtifactPaths`, `internal/next/guard.go`'s list and `internal/panel`'s `artifactPaths` all use it.

### R5 — One ledger; verbs are the only writers

1. **Record kinds.** The ledger gains five `record` kinds. Each goes through `panel.AppendRecord`. `panel disposition validate|check|query` accept them.

   | `record` | Fields | Written by |
   |:---------|:-------|:-----------|
   | `round` | round, reviewed_head_sha, subject_digest, target, gate, roster, at | `create` |
   | `refutation` | slot, round, finding, ground, ac?, reason, evidence, by, at | `refute` |
   | `deferral` | slot, round, finding, bead, finding_text, by, at | `defer` |
   | `rebind` | round, from_sha, to_sha, subject_digest, paths, reason, at | `rebind` |
   | `abandon` | reason, by, at | `abandon` |

2. **Snapshot.** `panel.json` is a snapshot. Only the five verbs write it. The gate's read model does not change.
3. **Hand-edit detection.** If the ledger has a `round` event, the gate checks that: each `panel.json` refutation and deferral has a matching ledger event; and `round`, `reviewed_head_sha`, `subject_digest` and `roster` equal the latest `round` or `rebind` event. A mismatch is Block: `panel.json changed outside the panel verbs` with `recovery: mindspec panel <verb> …`. Legacy panels are exempt (all 60 tracked panels in this repo).
4. **Skill text.** Both copies of `ms-panel-tally` and `ms-panel-run` lose every instruction to edit `panel.json` by hand. They name the verbs instead. A skill-text test pins the absence.

### R6 — Tracked ledger (R8.4 option B)

1. `gitutil.ReviewIgnoreEntries` gains two negation lines after the spec-129 entries: `!.mindspec/specs/*/reviews/*/` and `!.mindspec/specs/*/reviews/*/dispositions.jsonl`. BRIEF files, verdicts, `panel.json` and `dispositions.lock` stay ignored.
2. `mindspec init` writes them after the R8.1 entries, by exact-line presence. A second `init` leaves `.gitignore` byte-identical (spec 129 AC-20).
3. This repo's `.gitignore` gains the same two lines. Its R8.4 comment changes to name option (B).
4. Spec 129's R8.4 text gains one status line: `Decided 2026-10-19: option (B), shipped by spec 130 R6`.
5. R6 lands in the same bead as R3, or after it. Without R3, tracked ledger commits make non-bead panels stale (#235).

### R7 — Pass rule

1. A panel passes only when every row holds. The other gate legs (registration error, abandoned, round mismatch, missing ref, dirty tree, hatches) are unchanged.

   | Term | Must hold |
   |:-----|:----------|
   | complete | every roster slot has a parseable latest-round verdict |
   | no veto | 0 REJECT, 0 `hard_block`, 0 INVALID |
   | fresh | R3.3 says fresh |
   | discharged | every REQUEST_CHANGES slot is a resolved slot |
   | intact | R2.2 floor and R5.3 hold |

2. A panel with zero APPROVE verdicts passes when every row holds. A resolved slot is reported as `resolved`, never as an approval.
3. Nothing discharges a REJECT, a `hard_block`, an INVALID verdict, staleness or incompleteness. A discharge is pinned to its round; a REQUEST_CHANGES at a newer round needs a new discharge.
4. A slot-level refute or defer (no `--finding`) discharges every finding of that slot.
5. `approve_threshold` stays parse-compatible and is not a decision input. When it is present, `tally`, `verify` and `instruct` print `approve_threshold <value> ignored (legacy; not decision-bearing)`.

### R8 — Rule-line render (#243)

1. `tally`, `verify` and `instruct` panel state print one shared line:
   `rule: PASS iff all <n> slots in · 0 REJECT · 0 hard_block · 0 INVALID · 0 unresolved REQUEST_CHANGES (fix/refute/defer) — now: <k>/<n> in, <a> APPROVE, <r> resolved, <u> unresolved (<slots>), <i> INVALID`.
2. No surface prints `threshold X/N`.
3. `.mindspec/domains/workflow/interfaces.md` § Panel Artifact Schema describes R7, not N−1.

### R9 — Discharge and abandon verbs

1. **Shared contract.** Each verb holds the dispositions lock. It writes the discharge record, its ledger event and the spec-117 disposition row in one operation, then rewrites `panel.json` by temp file and rename. Each verb is idempotent on (slot, round, finding, kind); a rerun after a crash converges. Each verb prints the R8 line. Each refusal writes nothing.

   | Verb | Flags | Disposition row |
   |:-----|:------|:----------------|
   | `panel refute <slug>` | `--slot S --round N [--finding ccr[i]] --ground G [--ac AC-n] --reason T --evidence REF --by WHO` | `audited-refuted` |
   | `panel defer <slug>` | `--slot S --round N [--finding ccr[i]] --bead B --by WHO` | `confirmed-deferred` |
   | `panel abandon <slug>` | `--reason T --by WHO` | none |

2. **Refute.** `--ground` is one of `disproven`, `not-load-bearing`, `out_of_acceptance_scope`. `out_of_acceptance_scope` requires `--ac`, and the verb refuses unless that AC id exists in the spec's `spec.md`. The verb refuses: an empty reason, evidence or by; a slot whose verdict is APPROVE, REJECT, INVALID or has `hard_block`; a round other than LatestRound; an unknown finding id.
3. **Defer.** The verb checks through bd, at write time, that bead B: exists and is open; is not the spec's epic, a child of it, or a descendant of it; has a `discovered-from` edge to the epic or to the panel's bead; and has a description that contains each deferred finding's text. The leaf gate in `internal/panel` checks shape only (bead id valid, fields present).
4. **Abandon.** The verb sets `abandoned`, `abandon_reason` and an `abandon` ledger event. It refuses an empty reason or by. The `panel_abandoned` audit at `complete` is unchanged.

### R10 — Discharge audits at gate time

1. `complete` writes a durable `panel_deferred` audit beside `panel_refuted` (`panel_advisory.go:152-260`). Both handle any number of entries per panel.
2. `complete` and `impl approve` re-check each deferral through bd. If the bead is deleted, or is now under the spec's epic, the finding is unresolved again and the gate blocks, naming the slot, the finding and the bead.

### R11 — Core 3 enforcement

1. **Telemetry field.** `create` writes `telemetry: "required"` to `panel.json` and to the `round` event. Enforcement keys on this field, never on whether a store exists.
2. **complete.** After `panelGate` allows a matched registration, and before materialization, `complete` calls `CheckCompleteness` on the registration's own directory. A missing store or a floor violation on a required registration is a guard failure that names the slot, with `recovery: mindspec panel disposition check --spec <S> --panel <P>`. `MINDSPEC_SKIP_PANEL` and `enforcement.panel_gate: false` skip it, as they skip the gate.
3. **impl approve.** A preflight leg runs `CheckCompleteness` on every required registration of the spec, final review included. It fails closed with the same message shape and honours the same two hatches.
4. **Legacy.** A registration without the field passes with one advisory line.

### R12 — Durable reviewed SHA (#233 part a)

1. When `complete` has a matched panel, `ensureLandedBinding` writes `mindspec_landed_reviewed_sha=<reviewed_head_sha>` and `mindspec_landed_review_panel=<slug>@r<N>` in the same `mergeBindingFn` call as the binding.
2. The "already bound" check also compares these two keys.
3. With no matched panel, the binding writes neither key and removes neither key.
4. No reader of these keys changes in this spec.

### R13 — ADR amendments

Each amendment is an appended section, lands with the bead that ships the citing code, and is pinned by a text test.

1. ADR-0037: the six items in ADR Touchpoints.
2. ADR-0043: the four items in ADR Touchpoints, and a CHANGELOG line for the size change.
3. ADR-0030: the third subprocess on the mismatch path.
4. ADR-0044: a back-pointer from "The threshold is a" (line 84) to R7.

## Scope

### In Scope

- Every file in Impacted Domains, and the documentation surfaces listed there.

### Out of Scope

- The 60 tracked legacy panel directories: no file in them changes.

## Non-Goals

- Verdict schema v2, `panel submit`, and the S0 rule table (spec 131).
- #233's reviewed-tree-landed check, #251 ancestor arrival, and e07y item 1 (the spec-129 follow-on).
- A round limit (spec 133). This spec sets no round count.
- Rebuilding `panel.json` from the ledger alone.
- A `require_adjudication.min_families` policy or any family minimum.
- Signing or hashing verdicts against an adversary (ADR-0037 §8).

## Acceptance Criteria

- [ ] **AC-1 (R1.1)**: A bead panel and a plan panel each have `R1-round-1.json`. `create --round 1` exits non-zero, names `--round 2` and `panel rebind`, and leaves `panel.json` byte-identical. A round with no verdict files is re-created. Tests: `TestPanelCreate_RefusesSameRoundWithVerdicts` (bead, non-bead), `TestPanelCreate_SameRoundNoVerdictsAllowed`. `TestCreate_RepanelOfAbandonedPanelRevivesIt` uses round 2.
- [ ] **AC-2 (R1.2, R1.3)**: A round-1 panel with 2 refutations, bumped to round 2, keeps both entries. A revived abandoned panel has one `abandon_history` entry. A `panel.json` of `{` refuses with no write. Tests: `TestCreate_CarriesRefutationsAcrossRoundBump`, `TestCreate_RevivalAppendsAbandonHistory`, `TestCreate_UnparseablePanelJSONRefuses`.
- [ ] **AC-3 (R1.4, R1.5)**: A bump from round 2 writes `BRIEF-round-2.md` equal to the old BRIEF. A pre-existing archive is not overwritten. Tally still counts only `.json` slots. The prior-round block renders `consolidated-round-2.md` with an ESC byte escaped, or `none recorded`. Tests: `TestCreate_ArchivesBriefOnBump`, `TestCreate_NeverOverwritesArchive`, `TestCreate_PriorRoundRegion`.
- [ ] **AC-4 (R1.6, R2.1)**: Zero-config `create x --gate bead --spec S --target bead/x.1` records `bead_id: "x.1"` and 4 slots. `--target main`, `--bead y --target bead/x.1` and `--gate plan_approve --bead x.1` each refuse with no file written. `complete x.1` finds the panel and blocks while it is incomplete. Tests: `TestPanelCreate_BeadIDDerivedFromTarget`, `TestPanelCreate_BeadTargetRefusals`, `TestComplete_FindsDerivedBeadPanel`.
- [ ] **AC-5 (R1.7)**: Two worktrees register slug `b1` under specs 900 and 901. `tally b1` refuses, listing both. `tally b1 --spec 901` prints the 901 dir and round. `create b1 --spec 902` refuses. Tests: `TestFindPanelRegistration_AmbiguousRefuses`, `TestPanelCreate_SlugTakenByOtherSpec`.
- [ ] **AC-6 (R1.8, R1.9)**: An adhoc Block prints a recovery with no `--spec`. A spec panel Block prints the recorded spec id. A Block prints no `Usage:` text. The config error contains "cannot pass" and not "always-pass". Tests: `TestTallyExitActionNonBead_AdhocNoSpec`, `TestTallyExitActionNonBead_RecordedSpec`, `TestPanelCmd_SilenceUsage`, `TestConfigValidate_SingleReviewerText`.
- [ ] **AC-7 (R2.1)**: `DefaultConfig` resolves 6, 6, 4, 6, 6 for the five gates. A config with only `claude` gives 4 claude slots for `bead`. A config with `panel.reviewers` set gives that size for every gate without its own entry. Tests: `TestPanelGateDefaults_PerGateSizes`, `TestPanelGateDefaults_SingleFamily`, `TestPanelGateDefaults_ExplicitGlobalWins`.
- [ ] **AC-8 (R2.2, R2.3)**: `create --reviewers 1` refuses. A `panel.json` with `expected_reviewers: 1` blocks with a registration error naming `--reviewers 2`, and the message has no `/ms-bead-fix`. `--reviewers 3` on a round with a verdict refuses. Tests: `TestPanelCreate_ReviewersFloor`, `TestGate_SubFloorRegistrationError`, `TestPanelCreate_ReviewersRefusedAfterVerdict`.
- [ ] **AC-9 (R2.4, R2.5)**: A config change between rounds does not change the round-2 roster. A `zz-extra-round-1.json` APPROVE on a 5-slot roster with 4 APPROVE does not pass, and tally names `zz-extra` as unexpected. A legacy panel with `codex-R4` files tallies as today. Tests: `TestCreate_RosterCarriedForward`, `TestTally_OffRosterSlotCountsNothing`, `TestTally_LegacyNoRosterUnchanged`.
- [ ] **AC-10 (R3.1, R3.2)**: `Subject` returns the R3.1 table for each gate. An archive-only commit gives an equal digest. An `ls-tree` error and an empty listing each refuse at `create`. Tests: `TestSubject_PerGate`, `TestSubjectDigest_ExcludesReviewsAndJSONL`, `TestSubjectDigest_FailsClosed`.
- [ ] **AC-11 (R3.3, R3.4)**: Fixtures: an archive-only commit; an `OWNERSHIP.yaml`-only commit; a real edit. Results: archive-only is fresh on all gates; OWNERSHIP-only is fresh on spec/plan and stale on bead/final; a real edit is stale on all gates. A legacy panel gives today's message byte-for-byte. Equal SHAs make no `SubjectTree` call. `complete`, `verify`, `tally` and `instruct` give the same decision on each fixture. Tests: `TestGate_SubjectTreeStaleness`, `TestGate_LegacyStaleMessageUnchanged`, `TestGate_EqualSHANoSubjectCall`, `TestPanelConsumers_SubjectTreeParity`.
- [ ] **AC-12 (R4)**: After an OWNERSHIP-only commit on a passing bead panel, `rebind --reason "max: ownership claim"` makes it fresh with no round bump and one `rebinds[]` entry. Rebind refuses: an empty reason; a failing panel; a non-ancestor SHA; a path outside the allowlist; an empty allowlist. `complete` writes `panel_rebound_entries`. `reviewedHeadSHAForBead` contains `from_sha`. Tests: `TestPanelRebind_OwnershipOnly`, `TestPanelRebind_Refusals`, `TestComplete_CopiesRebinds`, `TestReviewedHeadSHASet_IncludesRebindFrom`, `TestToolArtifactPaths_SingleHome`.
- [ ] **AC-13 (R5.1, R5.2)**: Each verb appends its R5.1 record. `panel disposition validate` accepts all five kinds and rejects an unknown kind. Tests: `TestLedger_RecordKinds`, `TestDispositionValidate_LedgerKinds`.
- [ ] **AC-14 (R5.3)**: On a panel with a `round` event: a refutation added without an event blocks with `changed outside the panel verbs`; a `reviewed_head_sha` edit blocks; a `roster` edit blocks. A legacy panel with a hand-added refutation behaves as today. Tests: `TestGate_HandEditDetected`, `TestGate_LegacyHandEditExempt`.
- [ ] **AC-15 (R5.4)**: `grep -rn -e 'there is no CLI verb' -e 'set `"abandoned": true`' -e 'append one entry to the `refutations` array' plugins/mindspec/skills/ms-panel-tally plugins/mindspec/skills/ms-panel-run .claude/skills/ms-panel-tally .claude/skills/ms-panel-run` prints nothing. Both skill copies name `mindspec panel refute`, `mindspec panel defer` and `mindspec panel abandon`. Test: `TestPanelSkills_NoHandEditInstructions`.
- [ ] **AC-16 (R6)**: In a temp repo after `bootstrap.Run`, `git add -A` stages `.mindspec/specs/1-x/reviews/p/dispositions.jsonl` and no `panel.json`, `BRIEF.md`, verdict or `dispositions.lock`. A second run leaves `.gitignore` byte-identical. This repo's `.gitignore` contains both lines. Tests: `TestBootstrap_ReviewsLedgerTracked`, `TestBootstrap_ReviewsIgnored{Greenfield,ExistingGitignore,Idempotent}`.
- [ ] **AC-17 (R7.1–R7.3)**: Each row of the R7.1 table, when false alone, blocks. Six REQUEST_CHANGES slots with every finding discharged pass and report `0 APPROVE, 6 resolved`. A refuted REJECT, `hard_block` or INVALID slot still blocks. A round-1 discharge does not clear a round-2 REQUEST_CHANGES. Tests: `TestPassRule_EachTermBlocks`, `TestPassRule_ZeroApprovePasses`, `TestPassRule_NeverDischarged`, `TestPassRule_RoundPinned`.
- [ ] **AC-18 (R7.4, R7.5)**: A slot with 3 findings and a refute of `ccr[0]` only stays unresolved. A slot-level refute resolves it. Recorded `approve_threshold` values `unanimous`, `7` and `0` print the legacy line and do not change the decision. Tests: `TestPassRule_ItemLevelDischarge`, `TestApproveThreshold_LegacyNotice`.
- [ ] **AC-19 (R8)**: `tally`, `verify` and `instruct` print the same rule line for a fixture with 5 APPROVE, 1 unresolved and 1 INVALID. `grep -rn 'threshold %d/%d' cmd internal --include='*.go' | grep -v _test` prints nothing. Tests: `TestRuleLine_SharedRender`, `TestRuleLine_INVALIDCount`.
- [ ] **AC-20 (R9.1, R9.2)**: `refute` writes the discharge, the event and an `audited-refuted` row in one operation; a rerun adds nothing. `out_of_acceptance_scope` without `--ac`, or with `--ac AC-99`, refuses. Refute refuses an APPROVE, REJECT, `hard_block` and INVALID slot, a stale round, an unknown finding and an empty by. Tests: `TestPanelRefute_OneOperationIdempotent`, `TestPanelRefute_ACResolved`, `TestPanelRefute_Refusals`.
- [ ] **AC-21 (R9.3)**: With a fake bd, `defer` refuses a closed bead, a child of the epic, a bead with no `discovered-from` edge, and a description without the finding text. A valid bead writes a `confirmed-deferred` row. Tests: `TestPanelDefer_BDChecks`, `TestPanelDefer_Writes`.
- [ ] **AC-22 (R9.4)**: `abandon --reason "max: rework" --by max` sets the fields and the event. An empty reason refuses. Test: `TestPanelAbandon_Verb`.
- [ ] **AC-23 (R10)**: `complete` writes `panel_deferred` with 2 entries for 2 deferrals. A deferral bead re-parented under the epic, or deleted, makes `complete` and `impl approve` block, naming the slot, the finding and the bead. Tests: `TestComplete_PanelDeferredAudit`, `TestGateTime_DeferralRecheck`.
- [ ] **AC-24 (R11)**: On a required registration: a REQUEST_CHANGES slot with no covering row blocks `complete` and nothing is merged or materialized; a covering row passes; a deleted store blocks. A legacy registration passes with an advisory. `impl approve` refuses naming the final-review panel and slot. Each hatch skips the check. Tests: `TestComplete_Core3Enforced`, `TestComplete_Core3DeletedStoreBlocks`, `TestComplete_Core3LegacyAdvisory`, `TestImplApprove_Core3Leg`.
- [ ] **AC-25 (R12)**: After `complete` with a panel at round 3, bead metadata has `mindspec_landed_reviewed_sha` equal to the panel SHA and `mindspec_landed_review_panel` equal to `<slug>@r3`, written in the binding's one call. Without a panel, neither key is written. Tests: `TestEnsureLandedBinding_ReviewedKeys`, `TestEnsureLandedBinding_NoPanelNoKeys`.
- [ ] **AC-26 (R13)**: ADR-0037, ADR-0043, ADR-0030 and ADR-0044 each contain a `## Amendment (Spec 130)` section or back-pointer with the R13 items. `CHANGELOG.md` names the 6/6/4/6 change. Test: `TestADRAmendments_Spec130`.
- [ ] **AC-27 (method)**: `go build ./... && go test ./... -short` is green, including the ADR-0030 boundary lint (`internal/panel` imports no git or `os/exec`).

## Validation Proofs

- In a scratch repo with a bead panel holding `R1-round-1.json`: `mindspec panel create p --spec S --gate bead --target bead/x.1 --round 1` exits non-zero and prints `recovery:` naming `--round 2` (AC-1).
- `mindspec panel create p --spec S --gate bead --target main` exits non-zero; `ls <panel-dir>` shows no new file (AC-4).
- A commit that touches only `reviews/`, then `mindspec panel verify p`: exit 0 (AC-11).
- An OWNERSHIP-only commit on a bead panel, then `mindspec panel rebind p --reason "max: ownership"` and `mindspec complete x.1`: the panel gate passes (AC-12).
- `mindspec panel refute p --slot R2 --round 1 --ground disproven --reason r --evidence abc123 --by max`, then `mindspec panel tally p`: the rule line shows `1 resolved` (AC-17, AC-20).
- `mindspec init` in a scratch repo, then `git check-ignore -v .mindspec/specs/1-x/reviews/p/dispositions.jsonl` exits 1, and `git check-ignore -v .mindspec/specs/1-x/reviews/p/panel.json` exits 0 (AC-16).
- `go test ./internal/panel/... ./cmd/mindspec/... ./internal/complete/... ./internal/approve/... -short` is green (AC-27).
- `mindspec validate spec 130-panel-substrate-integrity` reports 0 errors.

## Open Questions

- [x] **OQ-1 (explicit config vs per-gate defaults)** — RESOLVED. An explicit `panel.reviewers` applies to every gate without its own entry, as in spec 112. The 6/6/4/6 defaults apply only with no reviewer config.
- [x] **OQ-2 (slot names)** — RESOLVED. New panels use the roster ids `R1..Rn` from `PanelGateReviewerSlots`. The BRIEF lists them. Kits that use other names (`F1`, `codex-R4`) must adopt the roster ids for new panels. Legacy panels are unaffected.
- [x] **OQ-3 (hatches on the `impl approve` Core 3 leg)** — RESOLVED. The leg honours `MINDSPEC_SKIP_PANEL` and `enforcement.panel_gate: false`, like `complete`. `runOrphanObligationGate` keeps its own no-hatch rule.
- [x] **OQ-4 (empty finding list)** — RESOLVED. A REQUEST_CHANGES with an empty list has one finding `ccr[0]` (its rationale), so the pass rule always has an item to discharge.
- [x] **OQ-5 (rebind and the landed set)** — RESOLVED. `from_sha` values join the exact-equality set (R3.5). No ancestor tolerance returns.

## Approval

- **Status**: DRAFT
- **Approved By**: -
- **Approval Date**: -
- **Notes**: -
