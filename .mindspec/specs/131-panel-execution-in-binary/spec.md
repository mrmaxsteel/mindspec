---
status: Draft
approved_at: ""
approved_by: ""
---
# Spec 131-panel-execution-in-binary: Panel Execution In Binary

*Citation scope: code citations were checked against `main` at `d15787fd` (spec 130 merged) on 2026-10-20. Line numbers are approximate; the named symbol is authoritative.*

## Goal

After this spec, the binary runs a review panel and takes in its verdicts. A skill only dispatches.

- A reviewer gives its verdict through `mindspec panel submit`. The binary validates it against verdict schema v2. A hand-written verdict file does not count.
- Reviewer launchers, families, lenses and acceptance questions are config data. No Go code names a reviewer family.
- Each reviewer works in its own checkout and scratch directory inside the repository. No skill tells an agent to use `/tmp`.
- `panel verify` gives a closed status for each slot and exits non-zero until every slot is final.
- One rule table holds the panel and bead-loop rules. Skills cite rule ids. `doctor` checks the launchers and the staged-rule markers.
- `ms-panel-run` and `ms-panel-tally` become short adapter steps.

## Background

Spec 130 (ADR-0045 status note, roadmap §4.2.1) moved verdict schema v2, `panel submit` and the rule table (ADR-0045 Implementation order step 1, and the step-3 `panel brief` template) to this spec. The round limit stays in spec 133.

Defects, each checked on `d15787fd`:

- **#249 / #229 (hard-coded families).** `ms-panel-run/SKILL.md` names "Claude Agents and Codex CLI sessions" in its description, and its launch, failure-detection and working-directory sections are about `codex exec` only. `ms-panel.js` allows only `codex exec --sandbox read-only --skip-git-repo-check` (`ALLOWED_CLI`) and refuses any family except `claude` and `codex` (`VALID_MIX_FAMILIES`). `config.resolveGateReviewers` (`internal/config/config.go:878`) fills zero-config panels from the literal `[]string{"claude", "codex"}`. A host whose second family is another CLI cannot launch a panel.
- **#249 part 2 (no generic review discipline).** `instruct` renders no review rules. Conduct text exists only in skill prose (`ms-panel-run` § Reviewer conduct).
- **#253 (`/tmp` scratch).** `ms-panel-run`, `ms-bead-fix` and `ms-spec-final-review` require scratch "at an ABSOLUTE path under `/tmp`". Some reviewer CLIs deny writes outside their launch directory, so agents invent untracked, unignored scratch directories.
- **#254 / #227 item 3 (dangling schema citation).** `renderBriefHeader` (`internal/panel/create.go:397`) tells every reviewer that the verdict format is in `.mindspec/domains/workflow/interfaces.md`. That path exists only in this repository.
- **#227 items 1–2 (no isolation, no fix-dispatch rule).** Reviewers that run tests share one worktree. A reviewer measured a tree that another reviewer had reverted. Nothing says when a fix agent may start.
- **#227 item 3 (stub verdicts).** A verdict has no `status` field. A preliminary stub counts as a final REQUEST_CHANGES.
- **Unvalidated verdicts.** Reviewers write `<slot>-round-<N>.json` by hand. The gate parses only `verdict` and `hard_block` (`internal/panel/tally.go:67`). No field says whether a finding was executed or only traced.
- **`panel verify` always exits 0** (`cmd/mindspec/panel.go:262`). An orchestrator cannot wait on it.

Dependency: #252 (skills cite ADR-0044) is fixed by a separate surface-leak change. R12 rewrites the same skill files. R12 lands after that change, or includes its edits.

## Impacted Domains

- **workflow**:
  - `internal/panel/**`: v2 schema and validator, submit binding in the gate, three ledger kinds, BRIEF regions, slot prompt template, slot status.
  - `cmd/mindspec/**`: new `panel submit|schema|checkout|launch|recover|prompt`, `rules`, `config resolve`; `panel verify` exit code; `doctor` checks.
  - `internal/instruct/**`: the `## Rules` section.
  - `internal/doctor/**`: launcher, staged-marker and checkout checks.
  - `internal/setup/**`, `plugins/mindspec/skills/**`, `.claude/skills/**`, `plugins/mindspec/workflows/ms-panel.js`, `.claude/workflows/ms-panel.js`, `plugins/mindspec/scripts/`: skill and workflow slimming; historical snapshots.
- **core**: `internal/config/**` (`panel.runners`, `panel.lenses`, `panel.gates.<g>.lenses|acceptance`, `panel.isolation_root`, `.mindspec/config.local.yaml`); a new `internal/rules` package with the embedded rule table; `internal/bootstrap/**` (`.gitignore` lines).
- **execution**: `internal/executor/**` (worktree add and remove for slot checkouts, `cat-file -e` for the BRIEF path check).

Documentation surfaces: ADR-0037, ADR-0040, ADR-0043, ADR-0044 and ADR-0045 amendments (R13); `.mindspec/domains/workflow/interfaces.md` § Panel Artifact Schema; `CHANGELOG.md`.

## ADR Touchpoints

- [ADR-0045](../../adr/ADR-0045-binary-owns-sequence-and-rules.md) — AMENDED: step 1 (rule table) and the step-3 BRIEF and slot-prompt templates ship. Step 5 starts: `instruct` renders the rule table. The staged-rule marker format changes (R10.4).
- [ADR-0044](../../adr/ADR-0044-panel-review-conduct.md) — AMENDED: the conduct text moves to reviewer-scope rules and the BRIEF conduct region. The conduct itself does not change.
- [ADR-0037](../../adr/ADR-0037-panel-gate-enforced-contract.md) — AMENDED: on a v2 round, only a submitted final verdict counts (R3). `verdict_schema` is a pinned field.
- [ADR-0043](../../adr/ADR-0043-panel-disposition-telemetry-store.md) — AMENDED: three new ledger kinds (`verdict`, `launch`, `substitution`).
- [ADR-0040](../../adr/ADR-0040-orchestration-layering-ratchet.md) — AMENDED: launchers are config data (`panel.runners`). The `runner:` key keeps its meaning.
- [ADR-0036](../../adr/ADR-0036-ownership-discovery.md) — APPLIED (ZFC): quota detection matches only configured literal strings and the exit code. No code judges reviewer output.
- [ADR-0030](../../adr/ADR-0030-executor-boundary.md) — APPLIED: worktree and `cat-file` calls go through the Executor. `internal/panel` stays free of git and `os/exec`.
- [ADR-0035](../../adr/ADR-0035-agent-error-contract.md) — APPLIED: every new refusal ends with one `recovery:` line.

## Requirements

**Terms.** Each term has this meaning only. Spec 130's terms (slot, roster, finding, ledger, legacy panel) apply unchanged.

- *v2 round*: a round whose `round` ledger event records `verdict_schema: 2`.
- *Runner*: one `panel.runners` entry. It tells the binary how to start a reviewer of one family.
- *Slot directory*: the per-slot scratch directory that R6 creates.
- *Submission*: a `verdict` ledger event that `panel submit` wrote.
- *Final verdict*: a submission with `status: final` and a verdict other than `VOID`.

### R1 — Verdict schema v2

1. A v2 verdict is one JSON object with exactly these top-level fields:

   | Field | Rule |
   |:------|:-----|
   | `schema` | required; the integer 2 |
   | `slot`, `round` | required; the roster id and the round |
   | `reviewed_sha` | required; 40 lowercase hex digits |
   | `status` | required; `draft` or `final` |
   | `verdict` | required; `APPROVE`, `REQUEST_CHANGES`, `REJECT` or `VOID` |
   | `model` | required; non-empty |
   | `rationale` | required; non-empty unless `verdict` is `VOID` |
   | `concrete_changes_required` | required array of items `{text, basis, command?}` |
   | `commands_run` | required array of `{cmd, exit, tail}` |
   | `void_reason` | required when `verdict` is `VOID`; refused otherwise |
   | `hard_block` | optional boolean |
   | `findings`, `inputs_read` | optional string arrays; reviewer evidence only |

2. An item's `text` is non-empty. Its `basis` is `executed`, `traced` or `hypothesis`. An `executed` item has `command`, the 0-based index of an existing `commands_run` entry. Other items have no `command`.
3. A `REQUEST_CHANGES` verdict has at least one item. `APPROVE`, `REJECT` and `VOID` have none.
4. The validator refuses an unknown field at any level. It refuses a file over 1 MiB, more than 100 items, more than 200 `commands_run` entries, and a `tail` over 4096 bytes.
5. Spec 130's finding model applies to the projection of a v2 verdict: item `i` is `ccr[i]`, its raw text is `text`, and the slot hash is the hash of the compact JSON array of the item texts.
6. One Go definition produces the validator and the JSON Schema that `mindspec panel schema` prints.

### R2 — `panel submit`

1. `mindspec panel submit <slug> --slot S --round N --file F [--spec ID]` submits a verdict. `--file -` reads stdin. Inside a slot directory, `mindspec panel submit --file F` reads the slug, spec, slot and round from the directory's `slot.json`.
2. Submit refuses each of these, and writes nothing:
   - a round that is not a v2 round;
   - a file that fails R1 (the message names the JSON pointer of the first error);
   - a `slot` or `round` field that differs from the flags or `slot.json`;
   - a slot that is not on the roster;
   - a round other than `panel.json`'s `round`;
   - an abandoned panel;
   - a `reviewed_sha` other than `panel.json`'s `reviewed_head_sha`;
   - a slot and round that already have a final verdict.
3. A draft or `VOID` submission can be replaced by a later submission for the same slot and round. A final verdict is frozen: one final verdict per slot per round. A finding in it is corrected by refutation through the existing discharge rules, or by a new round.
4. Submit appends one `verdict` ledger record, then writes the exact submitted bytes to `<slot>-round-<N>.json` by temp file and rename. A rerun with the same bytes converges.

   | `record` | Fields (plus `spec`, `panel`) | Key |
   |:---------|:-------|:----|
   | `verdict` | slot, round, status, verdict, model, file_sha256, created_at | slot, round, file_sha256 |

5. Submit prints the slot status (R8) and spec 130's rule line.
6. `panel disposition append` refuses the three new kinds (`verdict`, `launch`, `substitution`). `panel disposition validate|check|query` accept them.

### R3 — The gate on a v2 round

1. `create` records `verdict_schema: 2` in `panel.json` and in the `round` event of every new round. No config key or flag changes this value. Spec 130 R5.3 pins the field, so a hand edit of it blocks.
2. On a v2 round, a roster slot is in only when it has a final verdict, and the sha256 of its `<slot>-round-<N>.json` equals that submission's `file_sha256`.
3. On a v2 round, a verdict file with no matching submission counts for nothing. Spec 130 R2.5 (an off-roster REJECT, INVALID or `hard_block` blocks) applies unchanged.
4. A draft or `VOID` verdict counts as missing.
5. Spec 130's pass rule, discharge rules and coverage manifest use only the slots that are in.
6. A round created before this change has no `verdict_schema: 2`. It keeps spec 130's rules.

### R4 — Runners and slot resolution

1. `panel.runners` is an ordered list. Each runner has `family` (unique, non-empty) and `kind`.
   - `kind: harness`: the orchestrating harness dispatches the slot with its own agent tool. The runner has no other field except `on_quota`.
   - `kind: process`: the binary starts the reviewer. The runner needs `argv`, `prompt` (`stdin`, `file` or `arg`) and `output` (`stdout` or `file`). It can have `quota_signals` (literal strings), `on_quota` (another family), `probe` (an argv) and `timeout_minutes`.
2. `argv` placeholders are `{prompt}`, `{prompt_file}`, `{model}`, `{slot_dir}` and `{tree}`. `prompt: file` needs `{prompt_file}` in `argv`. `prompt: arg` needs `{prompt}`. Load refuses any other placeholder, a duplicate family, and an `on_quota` that names the runner's own family or an undeclared family.
3. The shipped default is data, not code: a `claude` harness runner, then a `codex` process runner. The codex runner uses today's allowlisted invocation, `prompt: stdin`, `output: stdout` and today's usage-limit message as its one quota signal. It has no `on_quota`.
4. Spec 130 R2.1's zero-config fill uses the runner table's family order. No non-test Go file in `internal/config`, `internal/panel` or `cmd/mindspec/panel*.go` contains `"claude"` or `"codex"`.
5. Each roster entry gains `family`, the reviewer entry's `family` (empty for a model-only entry). A slot's `model` is the reviewer entry's `model`, else `models["reviewer.<family>"]`, else the family string.
6. `.mindspec/config.local.yaml` is optional and machine-local. It can set only `panel.runners` and `models`. A local runner replaces the runner of the same family, or is appended. Load refuses every command when the file is over 65536 bytes, does not parse, has another key, or cannot be read for a reason other than absence. `mindspec init` adds the file to `.gitignore`.
7. `mindspec config resolve --gate G [--json]` prints each slot's `slot family model lens kind`. Each value shows its source: `config.yaml`, `config.local.yaml` or `default`.

### R5 — Lenses and acceptance questions as config

1. `panel.lenses` maps a lens name to its focus text. `panel.gates.<g>.lenses` is the ordered lens list for gate `g`. `panel.gates.<g>.acceptance` is `{question, not_blocking[]}`.
2. A lens-less slot takes the next lens from its gate's list, with spec 112's cursor rule. An explicit reviewer `lens` keeps its value. Load refuses a gate lens name that `panel.lenses` does not define.
3. The shipped defaults are data. `bead`, `final_review` and `adhoc` use today's six `defaultLenses` names. `spec_approve` and `plan_approve` use the four document lenses: falsifiability, scope honesty, contradiction, feasibility. The focus texts name no language or library. Each gate has a default acceptance question.
4. The `defaultLenses` Go literal goes.

### R6 — Slot checkouts and scratch (#227, #253)

1. `panel.isolation_root` defaults to `.mindspec-scratch` under the main working tree root. Load refuses a value that contains `..`, or that is under `.worktrees/` or `.mindspec/`. `mindspec init` adds the default to `.gitignore`.
2. A slot directory is `<isolation_root>/<spec or adhoc>/<slug>/r<N>/<slot>/`. It holds `slot.json`, `prompt.md` and `tree/`, a detached git worktree at the reviewed SHA.
3. `mindspec panel checkout <slug> (--slot S | --all) [--spec ID]` creates slot directories and prints each absolute path. A rerun reuses a directory whose tree is at the reviewed SHA. It refuses an abandoned panel, an off-roster slot, and a tree at another SHA (recovery: `--prune`).
4. `panel checkout <slug> --fixer` creates `<…>/r<N>/fixer/` with no tree. `panel checkout <slug> --prune` removes every checkout of the panel.
5. `create` for round N+1 removes the panel's checkouts of earlier rounds. `abandon` removes all of them.
6. Panel registration lookup never reads under the isolation root. No ledger record contains an isolation-root path.

### R7 — `panel launch` and `panel recover`

1. `mindspec panel launch <slug> <slot> [--spec ID]` starts one process runner and waits for it to exit. It refuses: a harness slot (recovery: `mindspec panel prompt`); a slot with no runner for its family; an abandoned panel; a round that is not a v2 round; a slot with a final verdict.
2. Launch creates the checkout (R6), writes `prompt.md` (R9.3) and appends a `launch` event with `status: started`. It runs `argv` with no shell, with the slot directory as cwd and `MINDSPEC_PANEL_SLOT` and `MINDSPEC_PANEL_ROUND` set. Output and errors go to `<slot_dir>/launch.log`. At `timeout_minutes`, launch kills the process group.
3. After exit, launch takes the verdict from the last block between the lines `MINDSPEC-VERDICT-BEGIN` and `MINDSPEC-VERDICT-END` in `launch.log` (`output: stdout`), or from `<slot_dir>/verdict.json` (`output: file`). It submits the verdict through R2.
4. Launch records one outcome, in this order: `submitted` (R2 accepted it); `quota_wall` (non-zero exit and the log contains a configured quota signal); `launch_failed` (non-zero exit or timeout); `malformed` (anything else).
5. On `quota_wall`, when the runner has `on_quota`, launch starts that family's runner once for the same slot. It appends a `substitution` event. A harness substitute is not started: launch prints the `panel prompt` command and exits non-zero.

   | `record` | Fields (plus `spec`, `panel`) | Key |
   |:---------|:-------|:----|
   | `launch` | slot, round, family, model, status, exit_code, created_at | slot, round, created_at |
   | `substitution` | slot, round, from_family, to_family, model, created_at | slot, round |

6. Launch exits 0 only when it recorded a submission.
7. `mindspec panel recover <slug> <slot> [--spec ID]` repeats step 3 on the existing log or `verdict.json`. It starts no process. It refuses when neither file exists.

### R8 — `panel verify` slot status

1. `panel verify` gives each roster slot one status from this closed set: `final`, `void`, `draft`, `running`, `quota_wall`, `launch_failed`, `malformed`, `unsubmitted`, `missing`.
2. On a v2 round, a slot with a final verdict is `final`. Otherwise the status comes from the slot's latest `verdict` or `launch` event: `VOID` gives `void`; `draft` gives `draft`; `started` gives `running`; a launch outcome gives its own name. With no event, a verdict file gives `unsubmitted`, and no file gives `missing`. A substituted slot also shows `substituted <from>-><to>`.
3. On another round, a parseable verdict file gives `final`, an unparseable file gives `malformed`, and no file gives `missing`.
4. `panel verify` exits 0 only when every roster slot is `final`. Otherwise it exits 1. `--json` prints the same statuses. Verify still prints the rule line and the PASS/BLOCK preview, and writes nothing.

### R9 — BRIEF and slot prompt (#254)

1. `create` renders these machine-owned BRIEF regions, with `termsafe` on every value:
   - *acceptance*: the gate's acceptance question and not-blocking list (R5);
   - *spec ACs*: the verbatim `## Acceptance Criteria` section of `spec.md` at the reviewed SHA, or `none (adhoc panel)`, or `no acceptance criteria section found`;
   - *conduct*: each reviewer-scope rule as `id: prohibition — replacement` (R10);
   - *verdict contract*: the R1 fields, `mindspec panel schema` and `mindspec panel submit`.
2. No machine-owned region names a documentation path. Each repository path that a region names must exist at the reviewed SHA. `create` checks this, refuses on a missing path, and names it.
3. `mindspec panel prompt <slug> --slot S [--spec ID]` prints the slot prompt and writes nothing. The prompt holds the BRIEF; the slot's id, family, model, lens and focus; the absolute slot directory and tree; and the output instruction for the slot's runner (the marker lines, `verdict.json`, or the `panel submit` command for a harness slot). From round 2, it lists the slot's own prior-round items and asks for each one: ADDRESSED, PARTIAL, MISSED or NEW_ISSUE.
4. The prompt tells the reviewer to write scratch files only in its slot directory.

### R10 — Rule table

1. One embedded YAML file holds the rule table. Each rule has `id`, `scope` (a subset of `orchestrator`, `implementer`, `reviewer`, `fixer`), `prohibition`, `replacement`, `enforced_by` (`advisory` or a mindspec verb name) and an optional `target` (the planned home of an advisory rule).
2. The table holds at least these rules:

   | id | scope | rule (summary) | enforced_by |
   |:---|:------|:---------------|:------------|
   | `panel.verbs-only` | all | no hand edit of `panel.json` or a verdict file; use the panel verbs | `complete` |
   | `panel.submit` | reviewer | no direct write of a verdict file; use `panel submit` | `panel submit` |
   | `panel.isolated-checkout` | reviewer | no test runs or edits in a shared worktree; use the slot tree | advisory |
   | `panel.scratch-in-slot` | reviewer, fixer | no scratch in a system temp directory or at a bare relative path; use the slot or fixer directory | advisory |
   | `panel.fix-after-final` | orchestrator | no fix agent before `panel verify` exits 0 | advisory |
   | `panel.one-fixer` | orchestrator | one fix agent for each panel round, in its own worktree | advisory |
   | `review.basis` | reviewer | no finding without its basis; an executed finding names its command | `panel submit` |
   | `review.property` | reviewer | state the violated property, not only a remedy | advisory |
   | `review.mutation` | reviewer | no guard accepted without a mutation that makes it fail | advisory |
   | `review.no-vote` | all | an APPROVE count resolves no finding; fix, refute or defer it | `complete` |
   | `review.derive` | implementer, fixer | derive from the source of truth; do not restate it | advisory |
   | `lifecycle.no-probe-complete` | orchestrator | no `mindspec complete` to test a gate; use `panel verify` and `mindspec validate` | advisory |
   | `lifecycle.complete-by-orchestrator` | implementer, fixer | no `mindspec complete`, `bd close` or raw bead merge by a subagent | advisory |
   | `sequence.bead-impl-prompt` | orchestrator | stage the implementation prompt before dispatch | advisory (`target: next-action`) |

3. `mindspec rules [--scope S] [--json]` lists rules. `mindspec rules show <id>` prints one rule. An unknown id or scope refuses.
4. A staged-rule marker is `<!-- staged-rule: id=<rule-id>; remove-by=YYYY-MM-DD -->`. The `ms-bead-impl` marker uses this form with `id=sequence.bead-impl-prompt`.
5. `instruct` renders a `## Rules` section: orchestrator-scope rules in every mode, plus implementer-scope rules in implement mode.
6. Rule text, rule ids and every string that R2–R11 add to the product surface cite no ADR, spec number, requirement id, bead id or issue number.

### R11 — Doctor checks

1. For each runner that a resolved roster of any gate uses, `doctor` checks:
   - a process runner's `argv[0]` is on `PATH`;
   - its `probe`, when set, exits 0 within 10 seconds.

   A failure is an Error when the runner comes from `config.yaml` or `config.local.yaml`, and a Warn when it is a shipped default. A harness runner gets one Info line.
2. `doctor` warns about a roster slot with an empty family or a family with no runner.
3. `doctor` warns about each staged-rule marker in an installed skill that has no id, an unknown id, or a past `remove-by` date. It lists the rules whose `enforced_by` is not `advisory` (Info).
4. `doctor` warns about a slot directory whose panel is missing, abandoned, or at a later round. The recovery is `mindspec panel checkout <slug> --prune`.
5. `doctor --ci` skips R11.1 and R11.4.

### R12 — Skill and workflow slimming

1. Both copies of `ms-panel-run` have at most 80 lines. The steps are: `panel create`; for each process slot, `panel launch` in the background; for each harness slot, `panel checkout`, `panel prompt` and a dispatch with this harness's agent tool; wait; `panel verify`; then `ms-panel-tally`.
2. Both copies of `ms-panel-tally` have at most 80 lines.
3. `grep -n -i -e codex -e /tmp` prints nothing for both copies of `ms-panel-run`, `ms-panel-tally`, `ms-bead-fix` and `ms-spec-final-review`. These skills contain no lens table, no size ladder and no failure-detection section. They cite rule ids for conduct.
4. `ms-spec-final-review` and `ms-bead-fix` use `panel checkout` (`--slot` or `--fixer`) for isolated work.
5. Both copies of `ms-panel.js` drop `codex exec` from `ALLOWED_CLI`, drop `VALID_MIX_FAMILIES`, and add `mindspec panel launch`, `mindspec panel prompt` and `mindspec panel verify`. The workflow takes the roster from `mindspec config resolve --json`.
6. `plugins/mindspec/scripts/codex_verdict_extract.sh` is deleted. `panel recover` replaces it.
7. `setup`'s historical skill snapshots gain the replaced versions, so an unmodified installed copy refreshes.

### R13 — ADR and document amendments

Each amendment is an appended `## Amendment (Spec 131)` section. It lands with the bead that ships the code it describes. A text test pins it.

1. ADR-0045: steps 1 and 3 (BRIEF and slot prompt) shipped; step 5 started (R10.5); the R10.4 marker form.
2. ADR-0044: reviewer conduct lives in reviewer-scope rules and the BRIEF conduct region. Skills cite rule ids.
3. ADR-0037: R3 counting, the pinned `verdict_schema` field with no opt-out, and frozen final verdicts (R2.3).
4. ADR-0043: the three new record kinds.
5. ADR-0040: `panel.runners` and `.mindspec/config.local.yaml`.
6. `interfaces.md` § Panel Artifact Schema describes v2, submit and slot status. `CHANGELOG.md` names: v2 on every new round with no opt-out (a hand-written verdict file no longer counts), frozen final verdicts, the `panel verify` exit code, and the new spec and plan default lenses.

## Scope

### In Scope

- Every file in Impacted Domains, and the documentation surfaces listed there.

### Out of Scope

- Every tracked legacy panel directory. No file in them changes.
- `.mindspec/domains/**` files other than `workflow/interfaces.md`.

## Non-Goals

- A round limit (spec 133).
- `next-action`, `bead prompt` and the implementation and fix prompt templates (spec 133).
- Repository overrides of the BRIEF or prompt templates (ADR-0045 Decision 3 override path).
- `complete --dry-run` (#249 part 2). File a follow-up bead.
- Test-selection rules (#249 part 2) and the deferral-dependency warning; spec 130's `defer` already needs a `discovered-from` edge.
- `exclude_author_family`, a family minimum, or any rule that judges reviewer output.
- #252 (the separate surface-leak change).
- Stub skills from a canonical source, and the skill-drift checks (ADR-0045 step 6).
- Rules for this repository's own AGENTS.md beyond R10.2.

## Acceptance Criteria

- [ ] **AC-1 (R1)**: The validator accepts a fixture of each verdict kind. It refuses each of these, naming the JSON pointer: no `status`; `verdict: "MAYBE"`; `VOID` without `void_reason`; `void_reason` on an APPROVE; an unknown top-level field; an unknown item field; an `executed` item without `command`; `command: 5` with 2 commands; a `traced` item with `command`; an APPROVE with one item; a REQUEST_CHANGES with none; a 1 MiB + 1 byte file; a 4097-byte `tail`. `panel schema` output and the validator agree on every fixture. Tests: `TestVerdictV2_Accepts`, `TestVerdictV2_Refusals`, `TestVerdictV2_SchemaMatchesValidator`.
- [ ] **AC-2 (R1.5)**: A v2 REQUEST_CHANGES with items `a` and `b` gives findings `ccr[0]` and `ccr[1]` with the sha256 of `a` and `b`. A `panel refute --finding ccr[1]` discharges `ccr[1]` only. Test: `TestVerdictV2_FindingProjection`.
- [ ] **AC-3 (R2)**: On a v2 round, submit writes the file and one `verdict` record. A rerun with the same bytes adds nothing. Each R2.2 refusal leaves the panel directory and ledger byte-identical. A draft, then a final, is accepted; a second final is refused. A `VOID`, then a final, is accepted. Submit from inside a slot directory with only `--file` succeeds. `panel disposition append` refuses each new kind. Tests: `TestPanelSubmit_Writes`, `TestPanelSubmit_Refusals`, `TestPanelSubmit_Replace`, `TestPanelSubmit_SlotDir`, `TestDispositionAppend_RefusesV2Kinds`.
- [ ] **AC-4 (R3)**: On a v2 round with 4 roster slots, all APPROVE: 4 submitted files pass. One hand-written APPROVE file in place of a submission blocks as incomplete, and verify shows `unsubmitted`. One submitted file changed by one byte blocks. One draft blocks. One `VOID` blocks. A fixture round created without `verdict_schema: 2` keeps spec 130's rules. With any config, `create` writes `verdict_schema: 2`, and load refuses a `panel.verdict_schema` key as unknown. A hand edit of `verdict_schema` in `panel.json` blocks. Tests: `TestGateV2_SubmittedOnly`, `TestGateV2_DigestBinding`, `TestGateV2_DraftAndVoidMissing`, `TestGate_PreV2RoundKeepsV1Rules`, `TestCreate_AlwaysV2`, `TestGate_VerdictSchemaPinned`.
- [ ] **AC-5 (R4.1–R4.4)**: Load refuses: a duplicate family; a harness runner with `argv`; a process runner without `output`; `prompt: file` without `{prompt_file}`; a `{shell}` placeholder; `on_quota` set to its own family. Zero-config `bead` is 2 claude and 2 codex. With `panel.runners: [{family: a, kind: harness}, {family: b, kind: harness}]` and no reviewers, zero-config `bead` is a, b, a, b. The R4.4 grep prints nothing. Tests: `TestRunners_Validate`, `TestRunners_ZeroConfigOrder`, `TestNoFamilyLiterals`.
- [ ] **AC-6 (R4.5–R4.7)**: `create` writes `family` on each roster entry. A slot with only `family: b` and `models: {reviewer.b: m1}` has model `m1`. A local file that replaces runner `b` changes `config resolve` output, and the source column says `config.local.yaml`. A local file with `loop:`, a 65537-byte local file, and an unparseable local file each make `mindspec config show` refuse. Tests: `TestRoster_Family`, `TestSlotModelResolution`, `TestLocalConfig_Merge`, `TestLocalConfig_Refusals`, `TestConfigResolve_Provenance`.
- [ ] **AC-7 (R5)**: Zero-config `spec_approve` slots get the four document lenses in order, then repeat. Zero-config `bead` slots get today's first four lens names. Load refuses `panel.gates.bead.lenses: [nope]`. The BRIEF of each gate holds its acceptance question. Tests: `TestLenses_GateDefaults`, `TestLenses_UnknownRefused`, `TestBrief_AcceptanceRegion`.
- [ ] **AC-8 (R6)**: `panel checkout --all` on a 4-slot panel creates 4 slot directories under `.mindspec-scratch/`, each with `slot.json`, and a `tree/` whose `HEAD` is the reviewed SHA. A rerun changes nothing. A tree at another SHA refuses with `--prune` in the recovery. `create --round 2` removes the round-1 checkouts. `abandon` removes all. A `panel.json` placed under the isolation root is not found by `panel tally`. After `mindspec init`, `git check-ignore -q .mindspec-scratch/x` exits 0. Load refuses `isolation_root: .worktrees/x`. Tests: `TestPanelCheckout_Creates`, `TestPanelCheckout_Refusals`, `TestPanelCheckout_PrunedOnRound`, `TestRegistration_IgnoresIsolationRoot`, `TestBootstrap_ScratchIgnored`.
- [ ] **AC-9 (R7)**: With a fake runner script per case, launch records: `submitted` for a valid marker block (and the verdict is submitted); `submitted` for a valid `verdict.json` in file mode; `quota_wall` for exit 2 with the quota string; `launch_failed` for exit 1 without it; `launch_failed` for a timeout; `malformed` for exit 0 with no block. On `quota_wall` with `on_quota: b`, the slot is relaunched once on `b`, one `substitution` event is appended, and verify shows `substituted a->b`. A harness slot refuses, naming `panel prompt`. Launch exits 0 only for `submitted`. `recover` on a log with a valid block submits it without a process start. No ledger record contains `.mindspec-scratch`. Tests: `TestPanelLaunch_Outcomes`, `TestPanelLaunch_Substitution`, `TestPanelLaunch_Refusals`, `TestPanelRecover`, `TestLedger_NoScratchPaths`.
- [ ] **AC-10 (R8)**: A fixture with one slot in each R8.1 status gives that status, and verify exits 1. When every slot is final, verify exits 0. A v1 round with all files parseable exits 0. `--json` gives the same statuses. Verify writes nothing. Tests: `TestPanelVerify_SlotStatus`, `TestPanelVerify_ExitCode`, `TestPanelVerify_V1Round`.
- [ ] **AC-11 (R9)**: A new BRIEF contains no `.mindspec/domains/`. It holds the spec's AC section verbatim, the conduct region with every reviewer-scope rule id, and the verdict contract naming `mindspec panel submit`. A spec with no AC section renders `no acceptance criteria section found`. `create` refuses when a path in a region is absent at the reviewed SHA. `panel prompt` for a process slot names the marker lines; for a harness slot it names `panel submit`; at round 2 it lists that slot's round-1 items only. An ESC byte in an AC is escaped. Tests: `TestBrief_NoDocPath`, `TestBrief_Regions`, `TestBrief_PathCheck`, `TestPanelPrompt_Render`, `TestPanelPrompt_PriorRound`.
- [ ] **AC-12 (R10)**: The table loads and holds every R10.2 id. `rules --scope reviewer` lists exactly the reviewer-scope rules. `rules show nope` refuses. `instruct` in implement mode lists `lifecycle.no-probe-complete` and `review.derive`; in spec mode it lists the first and not the second. The `ms-bead-impl` marker parses with `id=sequence.bead-impl-prompt`. No rule field, and no R2–R11 string literal in non-test code, matches `ADR-[0-9]`, `[Ss]pec [0-9]{3}`, `\bR[0-9]+\.[0-9]`, `#[0-9]{3}` or the bead-id pattern. Tests: `TestRuleTable_MinimumSet`, `TestRulesCmd`, `TestInstruct_RulesSection`, `TestStagedMarker_Parse`, `TestProductSurface_NoInternalRefs`.
- [ ] **AC-13 (R11)**: With a configured process runner whose `argv[0]` is not on `PATH`, `doctor` reports an Error naming the family. With the shipped codex default and no `codex` on `PATH`, it reports a Warn. A `probe` that exits 1 gives the same level. A harness runner gives an Info line. A marker with `remove-by=2020-01-01` and a marker with an unknown id each give a Warn. A slot directory of an abandoned panel gives a Warn with `--prune`. `doctor --ci` gives none of the launcher or checkout lines. Tests: `TestDoctor_Launchers`, `TestDoctor_StagedMarkers`, `TestDoctor_StaleCheckouts`, `TestDoctor_CISkipsLaunchers`.
- [ ] **AC-14 (R12)**: `wc -l` is 80 or less for both copies of `ms-panel-run` and `ms-panel-tally`. The R12.3 grep prints nothing. Both `ms-panel.js` copies have no `codex exec` and no `VALID_MIX_FAMILIES`, and their `ALLOWED_CLI` names `mindspec panel launch`. `codex_verdict_extract.sh` is absent. `mindspec setup claude` over an unmodified spec-130 install replaces the four skills. Tests: `TestPanelSkills_AdapterShape`, `TestPanelWorkflow_NoFamilies`, `TestSetup_RefreshesSpec130Skills`.
- [ ] **AC-15 (R13)**: ADR-0037, ADR-0040, ADR-0043, ADR-0044 and ADR-0045 each contain `## Amendment (Spec 131)` with the R13 items. `interfaces.md` § Panel Artifact Schema names `panel submit` and the R8.1 statuses. `CHANGELOG.md` names the four R13.6 changes. Test: `TestADRAmendments_Spec131`.
- [ ] **AC-16 (method)**: `go build ./... && go test ./... -short` is green, including `TestPanelLeafImports_StdlibPlusTermsafeOnly` and `TestEnforcementHasNoGitLeaks`.

## Validation Proofs

- In a scratch repo with a 2-slot v2 bead panel: `mindspec panel verify p; echo $?` prints `missing` twice and `1` (AC-10).
- `mindspec panel submit p --slot R1 --round 1 --file v.json` with a valid final APPROVE, then `mindspec panel verify p` shows `R1 final` (AC-3, AC-10).
- Write `R2-round-1.json` by hand, then `mindspec panel tally p`: the panel blocks, and verify shows `R2 unsubmitted` (AC-4).
- With a fake process runner on `PATH`: `mindspec panel launch p R2; echo $?` prints `0`, and `mindspec panel verify p; echo $?` prints `0` (AC-9, AC-10).
- `grep -c 'domains/workflow' .mindspec/specs/1-x/reviews/p/BRIEF.md` prints `0` (AC-11).
- `mindspec rules --scope reviewer` lists `panel.submit` and `review.basis` (AC-12).
- `mindspec doctor` with the codex default and no `codex` on `PATH` prints one Warn naming `codex` (AC-13).
- `mindspec validate spec 131-panel-execution-in-binary` reports 0 errors.

## Open Questions

- [x] **OQ-1 (harness slots)** — RESOLVED. The binary cannot start a harness-native agent. A `harness` runner gives the slot to the skill's dispatch step. The binary still renders the prompt and validates the verdict through `submit`.
- [x] **OQ-2 (where the verdict comes from in a sandboxed CLI)** — RESOLVED. A process reviewer never runs `mindspec`. Launch reads the verdict from stdout markers or `verdict.json` and submits it from outside the sandbox.
- [x] **OQ-3 (checkout type)** — RESOLVED. Slot trees are detached git worktrees. They share the object store, so a 6-slot panel is cheap. They live outside `.worktrees/`, so lifecycle worktree scans do not see them as spec or bead worktrees.
- [x] **OQ-4 (quota detection and ZFC)** — RESOLVED. Detection uses the exit code and configured literal strings only. A runner with no `quota_signals` never reports `quota_wall`.

### Open questions for the operator

- [x] **OQ-5 (v2 default)** — RESOLVED (operator). v2 is the default for every new round, with no opt-out (R3.1). Rounds created before this change keep spec 130's rules through the pinned `verdict_schema` field (R3.6). `CHANGELOG.md` says that a hand-written verdict file no longer counts on a new round.
- [x] **OQ-6 (final verdicts are frozen in a round)** — RESOLVED (operator, option A). One final verdict per slot per round. A second final is refused. Correction is by refutation through the existing discharge rules, or by a new round. Draft and `VOID` stay replaceable (R2.3).

## Approval

- **Status**: DRAFT
- **Approved By**: -
- **Approval Date**: -
- **Notes**: -
