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
- `complete` and `impl approve` enforce the disposition completeness floor (Core 3) over a coverage manifest that the binary writes.
- The landed binding records which panel SHA was reviewed, so the record survives finalize.

## Background

Provenance: spec 129 OQ-1 assigned #233's review-to-landing binding and the rest of #251 to spec 130. Spec 130 takes only #233 part (a), the durable reviewed-SHA key (R12). #233's reviewed-tree-landed check and #251 move to the spec-129 follow-on spec.

Defects, each verified on the v0.16.0 code:

- **A0 (same-round create).** `panel create --round N` over a round that has verdicts moves `reviewed_head_sha` to the tip. The old verdicts then pass. `cmd/mindspec/panel.go:143` only clamps `round <= 0`. `panel.Create` (`internal/panel/create.go:134`) never reads verdict files. Gate leg 6 (`internal/panel/gate.go:280`) then sees equal SHAs.
- **#238 (lost refutations).** `Create` builds a new `Panel{}` and overwrites `panel.json` (`create.go:135-176`). A round bump drops `refutations` and clears `abandoned`. A corrupt `panel.json` is overwritten without a warning.
- **#247 (BRIEF history).** No per-round BRIEF archive exists (this repo holds 31 hand-made `BRIEF-round-N.md` files). The "prior-round asks" region is a skill TODO (`create.go:40`) and goes stale.
- **#235 / #228 (whole-commit staleness).** Leg 6 compares whole-commit SHAs (`gate.go:280`). A commit that touches only `reviews/` or one `OWNERSHIP.yaml` line blocks and demands a re-panel. For non-bead panels, `nonBeadTargetRevParse` (`cmd/mindspec/panel.go:500`) resolves the spec branch, which contains the panel directory itself.
- **#242 (bead panel fails open).** `create --gate bead` without `--bead` writes `bead_id: null` and 6 reviewers. `complete` finds panels with `panel.ForBead` (`internal/complete/panel_advisory.go:179`), so it finds none and passes ("no registered panel → no gate").
- **#243 (wrong header).** `tally` prints `threshold 5/6` (`cmd/mindspec/panel.go:759`), but leg 9.5 blocks on any unresolved REQUEST_CHANGES first (`gate.go:353`). The header counts an unrecognised verdict as REQUEST_CHANGES.
- **#237 (one-slot panel).** With `expected_reviewers: 1`, the threshold is 0 and `gate.go:371` requires `threshold > 0`, so the panel blocks forever. Config already refuses fewer than 2 (`internal/config/config.go:707,783`), but its text says "always-pass", which is wrong. No panel subcommand sets `SilenceUsage`, so every Block prints cobra usage.
- **Slug hijack.** `findPanelRegistration` (`cmd/mindspec/panel.go:431`) returns the first `Slug()` match over a sorted scan that includes `.worktrees/*` (`cmd/mindspec/config.go:606`). A 0-verdict panel in spec 901 tallied as another spec's 6/6 PASS.
- **Silent threshold fallback.** `Panel.ApproveThreshold` (`internal/panel/panel.go:150`) turns a recorded `unanimous`, `7` or `0` into N−1 with no message.
- **Adhoc recovery.** `tallyExitActionNonBead` (`cmd/mindspec/panel.go:923`) always prints `--spec <id>`, which `create` refuses for `--gate adhoc` (`:135`), and prints the placeholder `<id>` when the spec is known.
- **Roster drift.** A re-panel takes the reviewer count from the current config, not from the prior round. Any `<slot>-round-N.json` file counts as a slot, so a stray `zz-extra` file made a 5-slot panel pass.
- **One-refutation cap.** One refuted RC clears 5/6. Two refuted RCs give 4/6 and block with "Run /ms-bead-fix", with nothing left to fix (`gate.go:370`).
- **Three stores.** Decisions live in `panel.json` (gate), `dispositions.jsonl` (telemetry only) and bead metadata. Skills tell the agent to hand-edit `panel.json` for refute and abandon (`plugins/mindspec/skills/ms-panel-tally/SKILL.md:109-119`, `ms-panel-run/SKILL.md:71`).
- **Agent-written manifest.** `panel.CheckCompleteness` (`internal/panel/disposition_store.go:395`) needs a `record:"panel"` coverage manifest. Only the skill writes it, through `panel disposition append` (`ms-panel-tally/SKILL.md:58-63`). The check uses the last manifest it reads, of any round.
- **R8.4 retention.** Spec 129 R8.1 ignores all of `reviews/**`. Finalize removes the spec worktree, so `dispositions.jsonl` is lost. This reverses ADR-0043 Decision ("git-versioned") without citing it.
- **Core 3 unenforced.** `CheckCompleteness` has one caller, `panel disposition check` (`cmd/mindspec/panel_disposition_store.go:147`). `complete` (`internal/complete/complete.go:839-886`) and `impl approve` (`internal/approve/impl.go:876`) never call it.
- **#233 (a).** After finalize, the ignored `panel.json` is gone. Nothing durable records which SHA the panel reviewed.

Corrections to the drafting brief: the landed-binding write is `ensureLandedBinding` (`internal/executor/mindspec_executor.go:1737`, write at `:1771`), not `:1660`. `configShowReviewRoots` is in `cmd/mindspec/config.go`, not `internal/config`. No refutation "ground" enum exists today; R9.2 adds it as a new field. `reviewedHeadSHAForBead` reads `ReviewedHeadSHA` at `internal/lifecycle/landed.go:752`.

## Impacted Domains

- **workflow**:
  - `internal/panel/**`: create merge-forward, roster, subject derivation, gate leg 6, pass rule, rule-line render, ledger kinds, hand-edit check, coverage manifest, tool-artifact path home.
  - `cmd/mindspec/**`: `panel create` flags, new `panel rebind|refute|defer|abandon`, slug resolution, `SilenceUsage`, tally and verify render, `panel disposition append` refusals.
  - `internal/complete/**`: subject-tree I/O, Core 3 call, `panel_deferred` audit, defer re-check, ledger commit, reviewed-SHA keys passed to the executor.
  - `internal/approve/**`: the Core 3 leg, the defer re-check and the uncommitted-ledger refusal in `impl approve`.
  - `internal/instruct/**`: panel-state render and non-bead target resolution (`panelstate.go`).
  - `internal/next/**`: the tool-artifact list moves to its single home.
  - `internal/lifecycle/**`: `reviewedHeadSHAForBead` includes rebind `from_sha` values.
  - `internal/bootstrap/**`: `mindspec init` writes the R6 negation lines.
  - `internal/setup/**`, `plugins/mindspec/skills/ms-panel-{run,tally}/SKILL.md`, `.claude/skills/ms-panel-{run,tally}/SKILL.md`, `plugins/mindspec/skills/ms-bead-impl/SKILL.md`: hand-edit text removed; roster ids; skill-text pins.
- **core**: `internal/config/**` (per-gate default sizes, `panel.rebind_allowlist`, error text); `.gitignore` (R6).
- **execution**: `internal/executor/**` (R12 keys, `ls-tree` method); `internal/gitutil/**` (`ls-tree` primitive, `ReviewIgnoreEntries` negations).

Documentation surfaces: ADR-0037, ADR-0041, ADR-0043, ADR-0044 and ADR-0045 amendments (R13); `.mindspec/domains/workflow/interfaces.md` § Panel Artifact Schema; `CHANGELOG.md` (size change); spec 129 R7.2 and R8.4 status lines.

## ADR Touchpoints

- [ADR-0037](../../adr/ADR-0037-panel-gate-enforced-contract.md) — AMENDED (R13.1):
  - §1: new `panel.json` fields (R1, R2, R3, R4, R9, R11); the verbs are the only writers.
  - §3: the pass rule (R7) replaces the N−1 threshold. It supersedes the Spec-114 amendment sentences "the approve threshold remains a NECESSARY floor" and "refutation cannot buy a sub-threshold panel past the gate". It retires the Spec-109 `approve_threshold` extension.
  - §4: subject-tree staleness and `panel rebind` (R3, R4).
  - Decision Details: the two-subprocess matched-path cap becomes three on a SHA mismatch (R3.3). The R10.2 re-check and the R11 Core 3 leg are `complete` preflight legs outside the cap.
  - §7: abandon is a verb (R9.4). A Block message and the R8 rule line may name the refute and defer verbs. Core 3 runs only on an Allow. A Warn registration (abandoned, missing ref, git error) skips Core 3 with one advisory line, so abandonment stays the audited exit (R11.6).
  - §8: hand-edit detection (R5.3) detects accidental desync only. It is anti-footgun, not anti-adversary, and adds no signing. The Spec-114 audit-durability carve-out extends to the `panel_deferred` audit (R10.1), the tracked ledger (R6) and the R12 keys.
  - Spec-115 amendment ("contract reach"): R10.2 and R11.5 add `impl approve` legs that block and that honour the two hatches. The orphan/obligation refusal keeps its no-hatch rule.
  - Spec-116 amendment: `artifactPaths` is no longer a local copy; it moves to the R4.6 single home.
- [ADR-0043](../../adr/ADR-0043-panel-disposition-telemetry-store.md) — AMENDED (R13.2):
  - Decision and Context §1: `dispositions.jsonl` is tracked again (R6).
  - Decision: new ledger record kinds (R5.1); the binary writes the coverage manifest (R11.2). The manifest lists roster slots only, not every verdict file. The manifest key gains `created_at`: a new manifest is appended, never rewritten, and the check reads the last manifest of the round.
  - Decision paragraph ("decision-inert", "No lifecycle verb reads the store") and the Related line ("consumes none of them"): the gate reads `round`, `rebind` and `abandon` events (R5.3), and `complete` and `impl approve` gate on the store (R11).
  - Consequences: `complete` and `impl approve` now gate on completeness (R11).
  - Amendment (Spec 126): the new section supersedes the bullet "The shipped default is 6 / `n-1`, unconditionally". The default becomes 6/6/4/6 per gate (R2.1). This reverses spec 112 R2.
- [ADR-0030](../../adr/ADR-0030-executor-boundary.md) — APPLIED: `internal/complete` gets `ls-tree` through a new Executor method, as it gets `RevParse` today (`panel_advisory.go:103`), never through `gitutil`. The R10.2 bd re-checks go through `internal/bead`. `internal/panel` stays free of git.
- [ADR-0035](../../adr/ADR-0035-agent-error-contract.md) — APPLIED: every new refusal goes through `guard.NewFailure` and ends with one copy-pastable `recovery:` line.
- [ADR-0041](../../adr/ADR-0041-gate-before-mutate.md) — AMENDED (R13.3): Spec-129 amendment (ii) datum (c) also includes `rebinds[].from_sha` (R3.5). APPLIED: each verb checks before it writes; Core 3 runs before materialization; the `impl approve` legs run in preflight; `panel_deferred` and `panel_rebound_entries` use the same §3(i) stage as `panel_refuted`.
- [ADR-0044](../../adr/ADR-0044-panel-review-conduct.md) — APPLIED: conduct is unchanged. Back-pointers to R7 (R13.4).
- [ADR-0045](../../adr/ADR-0045-binary-owns-sequence-and-rules.md) — AMENDED (R13.5): a status note moves the rule table (step 1) and the `panel brief` template (step 3) to spec 131. APPLIED: R5.4 and R9 move the refute and abandon procedures out of skills into verbs.

## Requirements

**Terms.** Each term has this meaning only.

- *Slot*: one roster entry. Its verdict file is `<slot>-round-<N>.json`.
- *Roster*: the ordered slot list of a round. Each slot has an id `R1..Rn`, a model and a lens.
- *Finding*: one blocking item of a REQUEST_CHANGES verdict. Item `i` of `concrete_changes_required` has the id `ccr[i]`. A verdict with an empty list has one finding, `ccr[0]`, whose text is its `rationale`. The verdict file's `findings` array is reviewer evidence, not a Finding.
- *Discharge*: a refutation or deferral record for one finding at the latest round.
- *Resolved slot*: a REQUEST_CHANGES slot whose every finding has a discharge.
- *INVALID*: a parseable verdict file whose verdict is not APPROVE, REQUEST_CHANGES or REJECT, or a REQUEST_CHANGES with an empty list and an empty `rationale`.
- *Subject*: the path set that R3.1 derives.
- *Ledger*: the panel directory's `dispositions.jsonl`.
- *Legacy panel*: a panel whose `panel.json` has no `telemetry` field AND whose ledger has no `round` event. A panel with exactly one of these two markers is a registration error (Block).

### R1 — Create integrity

1. **Round refusal (A0).** `create` computes LatestRound from the verdict files with `verdictFileRE`. It refuses N < LatestRound. It refuses N = LatestRound when that round has a verdict file. A refusal writes nothing. The recovery names `--round <LatestRound+1>` and `mindspec panel rebind`.
2. **Merge-forward (#238).** `create` reads the existing `panel.json` first. It carries `refutations`, `deferrals` and `rebinds` forward unchanged. It never removes an entry that the ledger backs. On a non-legacy panel, it carries only the entries that R5.3 matches, copies the roster from the latest `round` event, and names each entry that it drops. A revival of an abandoned panel appends `{reason, abandoned_at, revived_at}` to `abandon_history` and clears `abandoned`. On a legacy panel, the first `create` that writes a `round` event records each carried entry in that event's `carried` field (R5.3 baseline).
3. **Unparseable refusal.** If `panel.json` exists and does not parse, `create` refuses and writes nothing.
4. **BRIEF archive (#247).** On a round bump, `create` copies `BRIEF.md` to `BRIEF-round-<old>.md`. It never overwrites an existing archive. The archive must end in `.md`, because `verdictFileRE` counts `*-round-N.json` as slots.
5. **Prior-round region.** The machine-owned BRIEF region gains a `<!-- mindspec:prior-round -->` block. `create` renders it from `consolidated-round-<N-1>.md` through `termsafe`. If that file is absent, the block says `none recorded`.
6. **Bead id (#242).** For `--gate bead`, `create` derives the bead id from a `bead/<id>` target and validates it with `idvalidate.BeadID`. `create` refuses: `--gate bead` with any other target form; `--bead X` with a target other than `bead/X`; `--bead` with a non-bead gate. A refusal writes no file.
7. **Slug ambiguity.** `findPanelRegistration` collects all matches. Among matches with one spec id, the match under the current checkout wins (a tracked panel also appears under `.worktrees/`). Any other ambiguity refuses unless `--spec` selects one match. `tally` and `verify` print the resolved directory and round. `create` refuses a slug registered under another spec. `create --gate adhoc` refuses a slug registered anywhere. A non-adhoc `create` refuses a slug registered as adhoc.
8. **Adhoc recovery.** `tallyExitActionNonBead` omits `--spec` for `--gate adhoc`. It prints the recorded spec id, not `<id>`, when the spec is known. Every recovery line that names `panel create` omits `--spec` for an adhoc panel.
9. **Usage noise.** Every `panel` subcommand sets `SilenceUsage: true` (cobra v1.8.1 reads it from the executed command or the root only). The config errors at `config.go:707,783` say "a one-reviewer panel cannot pass", not "always-pass".

### R2 — Panel size and roster

1. **Per-gate defaults.** `DefaultConfig` holds no `panel.reviewers` list. The `loadUncached` backfill (`config.go:618`) and the `globalReviewers` fallback (`:861`) go. The `validateOrchestration` sum-below-2 check and the `approve_threshold` range check (`:694-708`) apply only to an explicit global list. When a gate resolves no reviewer list (its own, the global list, or for `adhoc` an explicit `bead` list), the size is: `spec_approve` 6, `plan_approve` 6, `bead` 4, `final_review` 6, `adhoc` 6. The binary distributes the size round-robin over the built-in family order `claude, codex`; zero-config `bead` is 2 claude and 2 codex. Today's codex→claude substitution applies unchanged. No family minimum exists. An explicit list keeps spec-112 semantics: `panel.reviewers: [{family: claude, count: 3}]` gives 3 claude slots for every gate without its own list.
2. **Floor.** A panel has at least 2 slots. `create` refuses fewer. The gate reads a `panel.json` with fewer than 2 slots as a registration error (Block). Its recovery is `mindspec panel create <slug> --spec <S> --round <N+1> --reviewers 2`, never `/ms-bead-fix`.
3. **`--reviewers N`.** `create` accepts `--reviewers N` with N ≥ 2.
4. **Roster record.** `create` writes `roster: [{slot, model, lens}]` to `panel.json` and to the `round` ledger event, from `config.PanelGateReviewerSlots` (called in `cmd/mindspec`). The BRIEF region lists each roster entry as `slot model lens`. A re-panel copies the prior round's roster (on a non-legacy panel, from the latest `round` event) unless `--reviewers` is given.
5. **Off-roster files.** When a roster is recorded, a verdict file whose slot is not on the roster is an *unexpected slot*. `tally` names it. It counts for nothing, with one exception: an unexpected slot whose verdict is REJECT or INVALID, or sets `hard_block`, is a Block that names the file. Every verdict file still counts for LatestRound (ADR-0037 §2 is unchanged). Legacy panels keep today's counting.

### R3 — Subject-tree staleness

1. **Derived subject.** One pure function `panel.Subject(gate, specDir)` returns the roots and exclusions. It never reads the subject from `panel.json`. `<specs>` is `workspace.SpecsDir` (flat `.mindspec/specs`, canonical `.mindspec/docs/specs`).

   | Gate | Roots |
   |:-----|:------|
   | `spec_approve`, `plan_approve` | `<specs>/<spec>/` |
   | `bead`, `final_review`, `adhoc`, empty (legacy) | whole tree |

   Every subject excludes exactly: `<specs>/*/reviews/**`, `.mindspec/reviews/**`, `review/**` (the non-flat and legacy panel root) and `.beads/issues.jsonl`. No other directory named `reviews` is excluded.
2. **Digest.** The subject digest is `sha256:` plus the sha256 of `git ls-tree -r <target> -- <roots>` output, filtered in Go by the exclusions. (`ls-tree` rejects exclude pathspec magic.) `create` and `rebind` record it as `subject_digest`. Any `ls-tree` error refuses. An empty filtered listing refuses; the binary never hashes empty input.
3. **Gate leg 6.** `GateIO` gains `SubjectTree(scanRoot, ref, roots)`.

   | Recorded state | Rule |
   |:---------------|:-----|
   | SHA equals live tip | fresh; no extra call |
   | SHA differs, `subject_digest` recorded | one `SubjectTree` call; stale only if digests differ; an error is Block |
   | SHA differs, no `subject_digest` (legacy) | stale; today's message, byte-identical |

4. **Consumers.** These call sites pass `SubjectTree` and decide identically: `complete`'s `panelGate` (`panel_advisory.go:211`, bead panels only), `panel verify` and `panel tally` (`cmd/mindspec/panel.go:459-475`), and `instruct` panel state. `gatherPanelState` newly resolves the target and calls `SubjectTree` for non-bead panels; `PanelStateEntry.verdict` stays pure.
5. **Landed set.** `reviewedHeadSHAForBead` adds each `rebinds[].from_sha` to its exact-equality set. Equality stays exact.

### R4 — `panel rebind`

1. `mindspec panel rebind <slug> --reason "<who>: <why>"` moves `reviewed_head_sha` and `subject_digest` to the live tip without a round bump. It refuses an empty reason.
2. It refuses unless all of these hold:
   - the panel parses, is not abandoned, has no round mismatch and passes R5.3;
   - the panel passes R7 when staleness is ignored (a rebind never turns a failing panel into a passing one);
   - the recorded SHA is an ancestor of the tip;
   - `.mindspec/config.yaml` is not in `git diff --name-only <recorded>..<tip>`;
   - every other path in that diff, minus the R3.1 exclusions, matches `panel.rebind_allowlist`.
3. `panel.rebind_allowlist` is a new config list. The default is `[".mindspec/domains/*/OWNERSHIP.yaml"]`. An empty list disables rebind. Matching uses `validate.GlobMatch`, called in `cmd/mindspec`.
4. It writes the SHA, the digest and one `rebinds[]` entry `{round, from_sha, to_sha, paths, reason, created_at}` in one write, plus a `rebind` ledger event. If the SHA already equals the tip, it prints `already fresh` and writes nothing.
5. At `complete`, a bead panel's `rebinds[]` is copied to bead metadata `panel_rebound_entries`.
6. The tool-artifact path list has one exported home in `internal/panel`: `.beads/issues.jsonl` and the pattern `<specs>/*/reviews/*/dispositions.jsonl` (matched with stdlib `path.Match`). `complete`'s `materializedArtifactPaths`, `internal/next/guard.go`'s `artifactPaths` and `internal/panel`'s `artifactPaths` all use it.

### R5 — One ledger; verbs are the only writers

1. **Record kinds.** The ledger gains five `record` kinds. The store gains a form keyed by the panel directory (flat, non-flat and adhoc layouts alike), and a batch append that runs `Validate`, `HygienePredicate` and the key check for each record under ONE lock hold. A same-key append is a no-op when every field except `created_at` is equal; otherwise it is refused (today's conflict rule). Every record carries `spec` and `panel`. An adhoc panel has no spec, so its records carry `spec: "adhoc"`. `panel disposition validate|check|query` accept the five kinds. `panel disposition append` refuses them, and refuses a `record:"panel"` manifest on a non-legacy panel; only the verb in each row writes that kind.

   | `record` | Fields (plus `spec`, `panel`) | Key | Written by |
   |:---------|:-------|:----|:-----------|
   | `round` | round, reviewed_head_sha, subject_digest, target, gate, bead_id, roster, expected_reviewers, telemetry, carried, created_at | round, created_at | `create` |
   | `refutation` | slot, round, finding, finding_sha256, finding_text, ground, ac?, reason, evidence, by, created_at | slot, round, finding, finding_sha256 | `refute` |
   | `deferral` | slot, round, finding, finding_sha256, finding_text, bead, by, created_at | slot, round, finding, finding_sha256 | `defer` |
   | `rebind` | round, from_sha, to_sha, subject_digest, paths, reason, created_at | to_sha | `rebind` |
   | `abandon` | reason, by, created_at | created_at | `abandon` |

   Every `create` appends a `round` event. `finding_sha256` is the sha256 of the raw finding text; R7.3 compares it. `finding_text` and the disposition row `summary` store that text with each `/tmp/` and `/Users/` token replaced by `<path>`, so `HygienePredicate` passes. A slot-level record (no `--finding`) has `finding: "slot"`, and its `finding_sha256` hashes the slot's whole `concrete_changes_required` list (or its `rationale` when the list is empty).

2. **Snapshot.** `panel.json` is a snapshot. Only the five verbs write it. Every verb appends its ledger event before it rewrites `panel.json`. The gate's read model does not change.
3. **Hand-edit detection.** On a non-legacy panel, the gate checks, before the abandoned leg:
   - `round`, `roster`, `gate`, `target`, `bead_id`, `expected_reviewers` (= len(roster)) and `telemetry` equal the latest `round` event;
   - `reviewed_head_sha` and `subject_digest` equal the later of the latest `round` and `rebind` events;
   - each refutation, deferral and `rebinds[]` entry has a matching ledger event, or is named in the `carried` field of the first `round` event;
   - `abandoned: true` and `abandon_reason` match an `abandon` event after the latest `round` event.

   A mismatch is Block: `panel.json changed outside the panel verbs`, with `recovery: mindspec panel create <slug> --spec <S> --round <N+1>`. Only R5.3-matched `from_sha` values join the R3.5 landed set. Legacy panels are exempt: every tracked panel directory in this repo (73 at `8744eeee`: 60 under `.mindspec/specs/*/reviews/`, 5 under `.mindspec/reviews/`, 8 under `review/`).
4. **Skill text.** Both copies of `ms-panel-tally` and `ms-panel-run` lose every instruction to edit `panel.json` by hand. They name the verbs instead. Both `ms-panel-run` copies name the roster ids as the verdict file prefix. A skill-text test pins both.

### R6 — Tracked ledger (R8.4 option B)

1. `gitutil.ReviewIgnoreEntries` gains two negation lines after the spec-129 entries: `!.mindspec/specs/*/reviews/*/` and `!.mindspec/specs/*/reviews/*/dispositions.jsonl`. BRIEF files, verdicts, `panel.json` and `dispositions.lock` stay ignored. Adhoc ledgers under `.mindspec/reviews/**` stay ignored.
2. `mindspec init` writes them after the R8.1 entries, by exact-line presence only. It does not use the `EnsureGitignoreEntriesWithHeader` check-ignore re-append, because git never reports a `!` line as ignored. A second `init` leaves `.gitignore` byte-identical (spec 129 AC-20).
3. This repo's `.gitignore` gains the same two lines. Its R8.4 comment changes to name option (B).
4. Spec 129's R8.4 text gains one status line: `Decided 2026-10-19: option (B), shipped by spec 130 R6`.
5. R6 lands in the same bead as R3, or after it. Without R3, tracked ledger commits make non-bead panels stale (#235).
6. **Ledger commits.** The ledger is a tool artifact (R4.6), like `.beads/issues.jsonl`. Workflow guards and the gate's dirty-tree leg do not count it as user dirt. `complete` commits the bead panel's ledger with its artifact commit, as `exec.CommitPaths` commits `.beads/issues.jsonl` (`internal/approve/adopt.go:479`). The `impl approve` preflight refuses when any `<specs>/<spec>/reviews/*/dispositions.jsonl` has uncommitted changes, with `recovery: git add <path> && git commit -m "chore(panel): ledger"`.

### R7 — Pass rule

1. A panel passes only when every row holds. The other gate legs (registration error, abandoned, round mismatch, missing ref, dirty tree, hatches) are unchanged.

   | Term | Must hold |
   |:-----|:----------|
   | complete | every roster slot has a parseable latest-round verdict (legacy: `expected_reviewers` parseable verdicts, as today) |
   | no veto | 0 REJECT, 0 `hard_block`, 0 INVALID, on roster or unexpected (R2.5) |
   | fresh | R3.3 says fresh |
   | discharged | every REQUEST_CHANGES slot is a resolved slot |
   | intact | R2.2 floor and R5.3 hold |

2. A panel with zero APPROVE verdicts passes when every row holds. A resolved slot is reported as `resolved`, never as an approval.
3. Nothing discharges a REJECT, a `hard_block`, an INVALID verdict, staleness or incompleteness. A discharge is pinned to its round; a REQUEST_CHANGES at a newer round needs a new discharge. A discharge applies only while its `finding_sha256` equals the sha256 of the current text of its finding. On a mismatch, the finding is unresolved and `tally` names it.
4. A slot-level refute or defer (no `--finding`) discharges every finding of that slot. Any change to that slot's latest-round verdict list (or `rationale`) voids it. A legacy entry with no `finding_sha256` is slot-level and not text-pinned. It behaves as today only on a legacy panel.
5. `approve_threshold` stays parse-compatible and is not a decision input. `create` no longer writes it. The `panel.approve_threshold` config key still parses and validates, and has no effect. When `panel.json` records the field, `tally`, `verify` and `instruct` print `approve_threshold <value> ignored (legacy; not decision-bearing)`.

### R8 — Rule-line render (#243)

1. `tally`, `verify` and `instruct` panel state print one shared line:
   `rule: PASS iff all <n> slots in · 0 REJECT · 0 hard_block · 0 INVALID · 0 unresolved REQUEST_CHANGES (fix/refute/defer) — now: <k>/<n> in, <a> APPROVE, <r> resolved, <u> unresolved (<slots>), <i> INVALID`.
2. No surface prints a threshold: this includes the gate Block messages (`gate.go:361,375`), which reach `complete` and `tally`, and `tally.go:304-310`, `panel.go:759` and `panelstate.go:134,182`.
3. `.mindspec/domains/workflow/interfaces.md` § Panel Artifact Schema describes R7, not N−1.

### R9 — Discharge and abandon verbs

1. **Shared contract.** Each verb uses the R5.1 batch append. In one lock hold it appends its R5.1 event and, for refute and defer, one spec-117 disposition row. It then rewrites `panel.json` (the snapshot copy) by temp file and rename. The row's `id` is `<panel>:<slot>:r<N>:<finding>:<sha12>:<kind>`, where `<sha12>` is the first 12 hex digits of `finding_sha256`. Its `model` is the roster entry's model, its `severity` is `blocking` and its `summary` is the redacted finding text (R5.1) through `termsafe`. A rerun after a crash converges through the R5.1 keys. Each verb prints the R8 line. Each refusal writes nothing. Refute and defer both refuse: a round other than LatestRound; a slot with no latest-round verdict file; a slot whose verdict is APPROVE, REJECT, INVALID or has `hard_block`; an unknown finding id; an empty `--by`.

   | Verb | Flags | Disposition row |
   |:-----|:------|:----------------|
   | `panel refute <slug>` | `--slot S --round N [--finding ccr[i]] --ground G [--ac AC-n] --reason T --evidence REF --by WHO` | `audited-refuted` |
   | `panel defer <slug>` | `--slot S --round N [--finding ccr[i]] --bead B --by WHO` | `confirmed-deferred` |
   | `panel abandon <slug>` | `--reason T --by WHO` | none |

2. **Refute.** `--ground` is one of `disproven`, `not-load-bearing`, `out_of_acceptance_scope`. `out_of_acceptance_scope` requires `--ac`, and the verb refuses unless that AC id exists in the spec's `spec.md`. The verb refuses an empty reason or evidence. `--evidence` must resolve: a commit that `git cat-file -e` finds, an existing repo path, or an `https://` URL.
3. **Defer.** The verb refuses on a `spec_approve` panel and on an adhoc panel, because no epic exists to anchor the edge. The recovery names `mindspec panel refute` or a fix. On other gates, the verb checks through bd, at write time, that bead B: exists and is open; is not the spec's epic, a child of it, or a descendant of it; has a `discovered-from` edge to the epic or to the panel's bead; and has a description that contains each deferred finding's text. The leaf gate in `internal/panel` checks shape only (bead id valid, fields present).
4. **Abandon.** The verb sets `abandoned` and `abandon_reason` and appends an `abandon` event. It writes no manifest. It refuses an empty reason or by. The `panel_abandoned` audit at `complete` is unchanged.

### R10 — Discharge audits at gate time

1. `complete` writes a durable `panel_deferred` audit beside `panel_refuted`. It uses the same pending-marker protocol as `writePanelRefutedMetadata` (`panel_advisory.go:456-490`, Spec 114 R2). Both handle any number of entries per panel.
2. `complete` and `impl approve` re-run every R9.3 bd check on each deferral: the bead exists and is open; it is not under the spec's epic; its `discovered-from` edge exists; its description contains the finding text. A closed bead fails at both verbs, for bead and non-bead panels alike. A failed check, or a bd error, makes the finding unresolved and the gate blocks, naming the slot, the finding and the bead. For a closed bead, the recovery names `bd reopen <B>` or `mindspec panel refute <slug> --slot S --round N --ground disproven --evidence <fixing commit> ...`.

### R11 — Core 3 enforcement

1. **Telemetry field.** `create` writes `telemetry: "required"` to `panel.json` and to the `round` event. Enforcement keys on this field, never on whether a store exists. R5.3 pins the field to the event.
2. **Coverage manifest.** Only `tally` writes it. On an Allow, `tally` appends one `record:"panel"` manifest for the latest round, derived from the roster and the latest-round verdict files, unless the last manifest of that round already equals the derivation. The manifest lists roster slots only. Its key is `spec, panel, round, created_at`, so a later Allow at the same round with a different derivation appends a new manifest. An agent-written manifest never counts (R5.1).
3. **The check.** For a required registration, the Core 3 check: validates every ledger record, all kinds included, and blocks on a malformed one; reads the LAST manifest of LatestRound and requires that its slots and verdicts equal the derivation from the roster and the verdict files; then applies the `CheckCompleteness` floor to that manifest. An older-round manifest never satisfies a newer round.
4. **complete.** For every registration that `panel.ForBead` matches, after `panelGate` returns Allow and before materialization, `complete` runs the check on that registration's directory. A Warn registration follows R11.6. A failure on a required registration is a guard failure that names the panel and the slot. The recovery is `mindspec panel tally <slug> --spec <S>` for a missing or stale manifest, and the R9 verb for an uncovered slot. `MINDSPEC_SKIP_PANEL` and `enforcement.panel_gate: false` skip it, as they skip the gate.
5. **impl approve.** A preflight leg runs the check on every required registration of the spec that is not abandoned, final review included. It fails closed with the same message shape and honours the same two hatches.
6. **Skips.** Core 3 runs only when `panelGate` returns Allow. A Warn registration (abandoned, missing ref, git error) skips Core 3 with one advisory line; abandonment stays the audited exit (ADR-0037 §7). A legacy registration passes with one advisory line.

### R12 — Durable reviewed SHA (#233 part a)

1. When `complete`'s gate decision for the matched panel is Allow, `ensureLandedBinding` writes `mindspec_landed_reviewed_sha=<reviewed_head_sha>` and `mindspec_landed_review_panel=<slug>@r<N>` in the same `mergeBindingFn` call as the binding. A Warn (abandoned, hatch, missing ref) writes neither key.
2. Only the `CompleteBead` caller (`mindspec_executor.go:482`) passes the two keys. The `FinalizeEpic` callers (`:722`, `:781`) pass none.
3. The "already bound" check (`:1764`) compares the two keys only when the caller passes them. With no keys passed, the binding writes neither key and removes neither key.
4. No reader of these keys changes in this spec.

### R13 — ADR amendments

Each amendment is an appended section, lands with the bead that ships the citing code, and is pinned by a text test.

1. ADR-0037: the items in ADR Touchpoints, with a back-pointer at the Spec-116 `artifactPaths` text.
2. ADR-0043: the items in ADR Touchpoints, with an inline back-pointer at the superseded Spec-126 bullet. The `## Amendment (Spec 126)` heading stays (`TestReviewDiscipline_ADR0043Amendment`). The same bead updates the spec-126 pins: the "shipped default: 6 reviewers" and "n-1" fragment rows (`internal/setup/review_discipline_test.go:802-805`), `TestReviewDiscipline_AC1PanelSizeConsistency`, and the `ms-panel-run/SKILL.md:32` sentence "`DefaultConfig` panel VALUES are unchanged". It also updates the spec-112 pins: the `DefaultConfig` 3+3 assertions in `internal/config/config_test.go` (`TestLoad_GatesAbsentByteIdentical109` and its siblings). A CHANGELOG line names the size change.
3. ADR-0041: a back-pointer at datum (c) (line 741) to R3.5.
4. ADR-0044: back-pointers from "The threshold is a" (line 84) and from "evidence-refuted via the audited refutation procedure" (line 83) to R7, which adds deferral as a third outcome.
5. ADR-0045: a status note that moves step 1 and the step-3 `panel brief` template to spec 131. The same bead updates the ms-bead-impl staged-rule marker text ("the spec-130 rule table", `SKILL.md:31`) and its pin `TestReviewDiscipline_AC10StagedRuleMarkerOnly`. Spec 129 R7.2 gains a status line: `Re-pointed 2026-10-19: the rule table moves to spec 131; #233's reviewed-tree check and #251 move to the spec-129 follow-on`.

## Scope

### In Scope

- Every file in Impacted Domains, and the documentation surfaces listed there.

### Out of Scope

- Every tracked legacy panel directory (R5.3): no file in them changes.

## Non-Goals

- Verdict schema v2, `panel submit`, the S0 rule table and the `panel brief` template (spec 131).
- #254 (the BRIEF header cites a repo path for the schema) and #227 (per-reviewer isolation, one fix agent after all verdicts) go to spec 131. #227's adhoc recovery line is R1.8.
- #233's reviewed-tree-landed check, #251 ancestor arrival, and e07y item 1 (the spec-129 follow-on).
- A round limit (spec 133). This spec sets no round count.
- Rebuilding `panel.json` from the ledger alone.
- A `require_adjudication.min_families` policy or any family minimum.
- Signing or hashing verdicts against an adversary (ADR-0037 §8).

## Acceptance Criteria

- [ ] **AC-1 (R1.1)**: A bead panel and a plan panel each have `R1-round-1.json`. `create --round 1` exits non-zero, names `--round 2` and `panel rebind`, and leaves `panel.json` byte-identical. On a round-2 panel, `create --round 1` refuses. A round with no verdict files is re-created. Tests: `TestPanelCreate_RefusesSameRoundWithVerdicts` (bead, non-bead), `TestPanelCreate_RefusesOlderRound`, `TestPanelCreate_SameRoundNoVerdictsAllowed`. `TestCreate_RepanelOfAbandonedPanelRevivesIt` uses round 2.
- [ ] **AC-2 (R1.2, R1.3)**: A round-1 panel with 2 refutations, bumped to round 2, keeps both entries. A revived abandoned panel has one `abandon_history` entry. A `panel.json` of `{` refuses with no write. Tests: `TestCreate_CarriesRefutationsAcrossRoundBump`, `TestCreate_RevivalAppendsAbandonHistory`, `TestCreate_UnparseablePanelJSONRefuses`.
- [ ] **AC-3 (R1.4, R1.5)**: A bump from round 2 writes `BRIEF-round-2.md` equal to the old BRIEF. A pre-existing archive is not overwritten. Tally still counts only `.json` slots. The prior-round block renders `consolidated-round-2.md` with an ESC byte escaped, or `none recorded`. Tests: `TestCreate_ArchivesBriefOnBump`, `TestCreate_NeverOverwritesArchive`, `TestCreate_PriorRoundRegion`.
- [ ] **AC-4 (R1.6, R2.1)**: Zero-config `create x --gate bead --spec S --target bead/probe-x.1` records `bead_id: "probe-x.1"` and 4 slots. `--target main`, `--bead probe-y.1 --target bead/probe-x.1` and `--gate plan_approve --bead probe-x.1` each refuse with no file written. `complete probe-x.1` finds the panel and blocks while it is incomplete. Tests: `TestPanelCreate_BeadIDDerivedFromTarget`, `TestPanelCreate_BeadTargetRefusals`, `TestComplete_FindsDerivedBeadPanel`.
- [ ] **AC-5 (R1.7)**: Two worktrees register slug `b1` under specs 900 and 901. `tally b1` refuses, listing both. `tally b1 --spec 901` prints the 901 dir and round. `create b1 --spec 902` refuses. A spec-900 panel that appears in the checkout and under `.worktrees/` resolves to the checkout copy. `create b1 --gate adhoc` refuses; a non-adhoc `create` of an adhoc slug refuses. Tests: `TestFindPanelRegistration_AmbiguousRefuses`, `TestFindPanelRegistration_CheckoutCopyWins`, `TestPanelCreate_SlugTakenByOtherSpec`, `TestPanelCreate_AdhocSlugCollision`.
- [ ] **AC-6 (R1.8, R1.9)**: An adhoc Block prints a recovery with no `--spec`. A spec panel Block prints the recorded spec id. A Block from the executed `panel tally` prints no `Usage:` text. The config error contains "cannot pass" and not "always-pass". Tests: `TestTallyExitActionNonBead_AdhocNoSpec`, `TestTallyExitActionNonBead_RecordedSpec`, `TestPanelSubcommands_SilenceUsage`, `TestConfigValidate_SingleReviewerText`.
- [ ] **AC-7 (R2.1)**: `DefaultConfig` resolves 6, 6, 4, 6, 6 for the five gates, and zero-config `bead` is 2 claude and 2 codex. `panel.reviewers: [{family: claude, count: 3}]` gives 3 claude slots for every gate without its own list. A `panel.gates.bead.reviewers` list keeps its own size, and `adhoc` inherits it. A `config.yaml` with no `panel.reviewers` and `approve_threshold: 3` loads. Tests: `TestPanelGateDefaults_PerGateSizes`, `TestPanelGateDefaults_RoundRobinFamilies`, `TestPanelGateDefaults_ExplicitGlobalWins`.
- [ ] **AC-8 (R2.2, R2.3)**: `create --reviewers 1` refuses. A `panel.json` with `expected_reviewers: 1` blocks with a registration error naming `--reviewers 2`, and the message has no `/ms-bead-fix`. Tests: `TestPanelCreate_ReviewersFloor`, `TestGate_SubFloorRegistrationError`.
- [ ] **AC-9 (R2.4, R2.5)**: A config change between rounds does not change the round-2 roster. The BRIEF lists each slot as `slot model lens`. A `zz-extra-round-1.json` APPROVE on a 5-slot roster with 4 APPROVE does not pass, and tally names `zz-extra` as unexpected. A `zz-extra` REJECT on a fully approved roster blocks, naming the file. A `zz-extra` INVALID on a fully approved roster blocks, naming the file. A legacy panel with `codex-R4` files tallies as today. Tests: `TestCreate_RosterCarriedForward`, `TestCreate_BriefListsRoster`, `TestTally_OffRosterSlotCountsNothing`, `TestTally_OffRosterRejectBlocks`, `TestTally_OffRosterInvalidBlocks`, `TestTally_LegacyNoRosterUnchanged`.
- [ ] **AC-10 (R3.1, R3.2)**: `Subject` returns the R3.1 table for each gate and both layouts. An archive-only commit gives an equal digest. An edit under `internal/reviews/` or `src/reviews/` changes the digest. An `ls-tree` error and an empty listing each refuse at `create`. Tests: `TestSubject_PerGate`, `TestSubjectDigest_ExcludesPanelRootsOnly`, `TestSubjectDigest_FailsClosed`.
- [ ] **AC-11 (R3.3, R3.4)**: Fixtures: an archive-only commit; an `OWNERSHIP.yaml`-only commit; a real edit; an edit to `internal/reviews/x.go`. Results: archive-only is fresh on all gates; OWNERSHIP-only is fresh on spec/plan and stale on bead/final; a real edit and the `internal/reviews` edit are stale on bead/final. A legacy panel gives today's message byte-for-byte. Equal SHAs make no `SubjectTree` call. On bead fixtures, `complete`, `verify`, `tally` and `instruct` give the same decision; on spec/plan fixtures, `verify`, `tally` and `instruct` do. Tests: `TestGate_SubjectTreeStaleness`, `TestGate_LegacyStaleMessageUnchanged`, `TestGate_EqualSHANoSubjectCall`, `TestPanelConsumers_SubjectTreeParity`.
- [ ] **AC-12 (R4)**: After an OWNERSHIP-only commit on a passing bead panel, `rebind --reason "max: ownership claim"` makes it fresh with no round bump and one `rebinds[]` entry. Rebind refuses: an empty reason; a failing panel; a non-ancestor SHA; a path outside the allowlist; a diff that contains `.mindspec/config.yaml`, even with a widened allowlist; an empty allowlist. `complete` writes `panel_rebound_entries`. `reviewedHeadSHAForBead` contains `from_sha`. Tests: `TestPanelRebind_OwnershipOnly`, `TestPanelRebind_Refusals`, `TestComplete_CopiesRebinds`, `TestReviewedHeadSHASet_IncludesRebindFrom`, `TestToolArtifactPaths_SingleHome`.
- [ ] **AC-13 (R5.1, R5.2)**: Each verb appends its R5.1 record with `spec` and `panel`; an adhoc record has `spec: "adhoc"`. A second `round` event and a second `rebind` event each append. A same-key rerun that differs only in `created_at` adds nothing; one that differs in another field refuses. A non-flat panel and an adhoc panel append to their own panel directory. `panel disposition validate` accepts all five kinds and rejects an unknown kind. `panel disposition append` refuses each of the five kinds and a manifest on a non-legacy panel. Tests: `TestLedger_RecordKinds`, `TestLedger_RecordKeys`, `TestDispositionValidate_LedgerKinds`, `TestDispositionAppend_RefusesVerbKinds`.
- [ ] **AC-14 (R5.3)**: On a non-legacy panel, each single-field hand-edit blocks with `changed outside the panel verbs`: an added refutation; an added `rebinds[]` entry; `reviewed_head_sha`; `roster`; `bead_id`; `expected_reviewers`; a hand-set `abandoned: true`. A `panel.json` with the `telemetry` field removed blocks, and a ledger with the `round` event removed blocks. A legacy panel with a hand-added refutation behaves as today. A legacy panel with a round-1 hand refutation, bumped to round 2 by `create`, does not block on the carried entry. On a non-legacy panel, a hand-added refutation, then the R5.3 recovery `create --round <N+1>`: `create` names the dropped entry, and the panel no longer blocks on R5.3. Tests: `TestGate_HandEditDetected`, `TestGate_OneMarkerBlocks`, `TestGate_LegacyHandEditExempt`, `TestGate_CarriedBaseline`, `TestCreate_DropsUnbackedEntries`.
- [ ] **AC-15 (R5.4)**: `grep -rn -e 'there is no CLI verb' -e 'set `"abandoned": true`' -e 'sets by hand' -e 'directly in `panel.json`' -e 'append one entry to the `refutations` array' plugins/mindspec/skills/ms-panel-tally plugins/mindspec/skills/ms-panel-run .claude/skills/ms-panel-tally .claude/skills/ms-panel-run` prints nothing. Both skill copies name `mindspec panel refute`, `mindspec panel defer` and `mindspec panel abandon`, and both `ms-panel-run` copies name the roster ids. Test: `TestPanelSkills_NoHandEditInstructions`.
- [ ] **AC-16 (R6)**: In a temp repo after `bootstrap.Run`, `git add -A` stages `.mindspec/specs/1-x/reviews/p/dispositions.jsonl` and no `panel.json`, `BRIEF.md`, verdict or `dispositions.lock`. A second run leaves `.gitignore` byte-identical. This repo's `.gitignore` contains both lines. A dirty ledger does not block `next`. `complete` commits the bead panel's ledger. `impl approve` refuses an uncommitted spec ledger. Tests: `TestBootstrap_ReviewsLedgerTracked`, `TestBootstrap_ReviewsIgnored{Greenfield,ExistingGitignore,Idempotent}`, `TestNextGuard_LedgerIsArtifact`, `TestComplete_CommitsLedger`, `TestImplApprove_UncommittedLedgerRefuses`.
- [ ] **AC-17 (R7.1–R7.3)**: Each row of the R7.1 table, when false alone, blocks. Six REQUEST_CHANGES slots with every finding discharged pass and report `0 APPROVE, 6 resolved`. A refuted REJECT, `hard_block` or INVALID slot still blocks. A REQUEST_CHANGES with an empty list and empty `rationale` is INVALID. A round-1 discharge does not clear a round-2 REQUEST_CHANGES. A discharge whose `finding_sha256` differs from the hash of the current `ccr[i]` text leaves it unresolved. Tests: `TestPassRule_EachTermBlocks`, `TestPassRule_ZeroApprovePasses`, `TestPassRule_NeverDischarged`, `TestPassRule_EmptyRCInvalid`, `TestPassRule_RoundPinned`, `TestPassRule_FindingTextPinned`.
- [ ] **AC-18 (R7.4, R7.5)**: A slot with 3 findings and a refute of `ccr[0]` only stays unresolved. A slot-level refute resolves it. A fourth item added to that verdict file voids the slot-level refute. Recorded `approve_threshold` values `unanimous`, `7` and `0` print the legacy line and do not change the decision. A new `create` writes no `approve_threshold`. Tests: `TestPassRule_ItemLevelDischarge`, `TestPassRule_SlotLevelVoidedByChange`, `TestApproveThreshold_LegacyNotice`, `TestPanelCreate_NoApproveThreshold`.
- [ ] **AC-19 (R8)**: `tally`, `verify` and `instruct` print the same rule line for a fixture with 5 APPROVE, 1 unresolved and 1 INVALID. `grep -rn -e 'threshold is %d' -e 'threshold %d' cmd/mindspec internal/panel internal/instruct internal/complete --include='*.go' | grep -v _test` prints nothing. Tests: `TestRuleLine_SharedRender`, `TestRuleLine_INVALIDCount`.
- [ ] **AC-20 (R9.1, R9.2)**: `refute` writes the event and an `audited-refuted` row in one lock hold; a rerun adds nothing. `out_of_acceptance_scope` without `--ac`, or with `--ac AC-99`, refuses. Refute refuses an APPROVE, REJECT, `hard_block` and INVALID slot, a slot with no latest-round verdict file, a stale round, an unknown finding, an empty by, and `--evidence` that resolves to nothing. A finding whose text contains `/tmp/x` is refuted; the stored text and `summary` hold `<path>`, and the discharge applies. Tests: `TestPanelRefute_OneOperationIdempotent`, `TestPanelRefute_ACResolved`, `TestPanelRefute_Refusals`, `TestPanelRefute_RedactsPaths`.
- [ ] **AC-21 (R9.1, R9.3)**: With a fake bd, `defer` refuses a closed bead, a child of the epic, a bead with no `discovered-from` edge, and a description without the finding text. It refuses a stale round and an APPROVE, REJECT, `hard_block` or INVALID slot. It refuses on a `spec_approve` panel and on an adhoc panel, with a recovery that names `refute`. A valid bead writes a `confirmed-deferred` row. Tests: `TestPanelDefer_BDChecks`, `TestPanelDefer_SharedRefusals`, `TestPanelDefer_NoEpicGates`, `TestPanelDefer_Writes`.
- [ ] **AC-22 (R9.4)**: `abandon --reason "max: rework" --by max` sets the fields and the event, and writes no manifest. An empty reason refuses. Test: `TestPanelAbandon_Verb`.
- [ ] **AC-23 (R10)**: `complete` writes `panel_deferred` with 2 entries for 2 deferrals, through the pending-marker protocol. Each of these makes `complete` and `impl approve` block, naming the slot, the finding and the bead: a deferral bead re-parented under the epic; deleted; with its `discovered-from` edge removed; with the finding text removed from its description; a bd error. A closed bead blocks `complete`. A bead closed after `complete` blocks `impl approve`, and the recovery names `bd reopen` and `panel refute`. A closed bead deferred on a `final_review` panel blocks `impl approve`. Tests: `TestComplete_PanelDeferredAudit`, `TestGateTime_DeferralRecheck`, `TestGateTime_DeferralClosedBead`.
- [ ] **AC-24 (R11)**: On a required registration, each of these blocks `complete`, and nothing is merged or materialized: a REQUEST_CHANGES slot with no covering row; a deleted store; no manifest; a round-1 all-APPROVE manifest on a round-2 registration; a manifest whose verdicts differ from the files; a malformed manifest; a malformed disposition row. A tally-written manifest with covering rows passes. After a same-round verdict change, a refute and a new Allow, `tally` appends a second manifest and `complete` passes. An unexpected slot is not in the manifest. With two panels for one bead, a missing store on the second blocks. A legacy registration passes with an advisory. An abandoned registration with a REJECT slot passes `complete` and `impl approve` with an advisory. `impl approve` refuses, naming the final-review panel and slot. Each hatch skips the check. Tests: `TestComplete_Core3WarnSkips`, `TestTally_AppendsManifestOnChange`, `TestTally_WritesManifestOnAllow`, `TestComplete_Core3Enforced`, `TestComplete_Core3ManifestRound`, `TestComplete_Core3MalformedRecords`, `TestComplete_Core3EveryMatchedPanel`, `TestComplete_Core3LegacyAdvisory`, `TestImplApprove_Core3Leg`.
- [ ] **AC-25 (R12)**: After `complete` with an Allowed panel at round 3, bead metadata has `mindspec_landed_reviewed_sha` equal to the panel SHA and `mindspec_landed_review_panel` equal to `<slug>@r3`, written in the binding's one call. Without a panel, or with an abandoned panel, neither key is written. A `FinalizeEpic` re-bind keeps the two keys unchanged. Tests: `TestEnsureLandedBinding_ReviewedKeys`, `TestEnsureLandedBinding_NoPanelNoKeys`, `TestEnsureLandedBinding_FinalizeKeepsKeys`.
- [ ] **AC-26 (R13)**: ADR-0037, ADR-0041, ADR-0043, ADR-0044 and ADR-0045 each contain a `## Amendment (Spec 130)` section or back-pointer with the R13 items. `CHANGELOG.md` names the 6/6/4/6 change. Spec 129 contains both status lines. Test: `TestADRAmendments_Spec130`.
- [ ] **AC-27 (method)**: `go build ./... && go test ./... -short` is green, including `TestPanelLeafImports_StdlibPlusTermsafeOnly` (`internal/panel` imports no git or `os/exec`) and `TestEnforcementHasNoGitLeaks` (`internal/complete` and `internal/approve`).

## Validation Proofs

- In a scratch repo with a bead panel holding `R1-round-1.json`: `mindspec panel create p --spec S --gate bead --target bead/probe-x.1 --round 1` exits non-zero and prints `recovery:` naming `--round 2` (AC-1).
- `mindspec panel create p --spec S --gate bead --target main` exits non-zero; `ls <panel-dir>` shows no new file (AC-4).
- On a passing panel, a commit that touches only `reviews/`, then `mindspec panel tally p`: exit 0 (AC-11).
- An OWNERSHIP-only commit on a bead panel, then `mindspec panel rebind p --reason "max: ownership"` and `mindspec complete probe-x.1`: the panel gate passes (AC-12).
- `mindspec panel refute p --slot R2 --round 1 --ground disproven --reason r --evidence "$(git rev-parse HEAD)" --by max`, then `mindspec panel tally p`: the rule line shows `1 resolved` (AC-17, AC-20).
- `mindspec init` in a scratch repo, then `git check-ignore -q .mindspec/specs/1-x/reviews/p/dispositions.jsonl` exits 1, and `git check-ignore -q .mindspec/specs/1-x/reviews/p/panel.json` exits 0 (AC-16).
- `go test ./internal/panel/... ./cmd/mindspec/... ./internal/complete/... ./internal/approve/... -short` is green (AC-27).
- `mindspec validate spec 130-panel-substrate-integrity` reports 0 errors.

## Open Questions

- [x] **OQ-1 (explicit config vs per-gate defaults)** — RESOLVED. An explicit `panel.reviewers` applies to every gate without its own entry, as in spec 112. The 6/6/4/6 defaults apply only when a gate resolves no reviewer list.
- [x] **OQ-2 (slot names)** — RESOLVED. New panels use the roster ids `R1..Rn` from `PanelGateReviewerSlots`. The BRIEF lists each as `slot model lens`, and both `ms-panel-run` copies name them as the verdict prefix. Kits that use other names (`F1`, `codex-R4`) must adopt the roster ids for new panels. Legacy panels are unaffected.
- [x] **OQ-3 (hatches on the `impl approve` legs)** — RESOLVED. The Core 3 and deferral legs honour `MINDSPEC_SKIP_PANEL` and `enforcement.panel_gate: false`, like `complete`. `runOrphanObligationGate` keeps its own no-hatch rule.
- [x] **OQ-4 (empty finding list)** — RESOLVED. A REQUEST_CHANGES with an empty list has one finding `ccr[0]` (its rationale). With an empty rationale too, it is INVALID.
- [x] **OQ-5 (rebind and the landed set)** — RESOLVED. R5.3-matched `from_sha` values join the exact-equality set (R3.5). No ancestor tolerance returns.

## Approval

- **Status**: DRAFT
- **Approved By**: -
- **Approval Date**: -
- **Notes**: -
