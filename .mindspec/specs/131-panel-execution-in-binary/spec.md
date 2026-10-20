---
status: Draft
approved_at: ""
approved_by: ""
---
# Spec 131-panel-execution-in-binary: Panel Execution In Binary

*Citation scope: code citations were checked against `main` at `d15787fd` (spec 130 merged) on 2026-10-20. Line numbers are approximate; the named symbol is authoritative.*

## Goal

After this spec, the binary runs a review panel and takes in its verdicts. A skill only dispatches.

- A reviewer gives its verdict through `mindspec panel submit`. The binary validates it against verdict schema v2. On a new round, a verdict file with no submission does not count.
- Reviewer launchers, lenses and acceptance questions are config data. The binary ships no runner for a named tool. With zero config, every slot runs on the orchestrating harness. A one-family panel is valid.
- Each reviewer works in its own clone and scratch directory inside the repository. No skill tells an agent to use `/tmp`.
- `panel verify` gives a closed status for each slot. Its exit code says done, pending or stuck.
- One rule table holds the panel and bead-loop rules. Skills cite rule ids. `doctor` checks the launchers and the staged-rule markers.
- `ms-panel-run` and `ms-panel-tally` become short adapter steps.

## Background

Spec 130 (ADR-0045 status note, roadmap §4.2.1) moved verdict schema v2, `panel submit` and the rule table (ADR-0045 step 1, and the step-3 brief template) to this spec. The round limit stays in spec 133.

Defects, each checked on `d15787fd`:

- **#249 / #229 (hard-coded families).** `ms-panel-run` names two tool families in its description. Its launch, failure-detection and working-directory sections are about one CLI only. `ms-panel.js` allows one CLI invocation (`ALLOWED_CLI`) and two family names (`VALID_MIX_FAMILIES`). `config.resolveGateReviewers` (`internal/config/config.go:878`) fills zero-config panels from a two-name literal. `knownModels` (`config.go:219`) holds the same two names. A host with one harness, or with another CLI, cannot run a panel as shipped.
- **#249 part 2 (no generic review discipline).** `instruct` renders no review rules. Conduct text exists only in skill prose.
- **#253 (`/tmp` scratch).** `ms-panel-run`, `ms-bead-fix` and `ms-spec-final-review` require scratch "at an ABSOLUTE path under `/tmp`". Some reviewer CLIs deny writes outside their launch directory, so agents invent untracked, unignored scratch directories.
- **#254 / #227 item 3 (dangling schema citation).** `renderBriefHeader` (`internal/panel/create.go:397`) tells every reviewer that the verdict format is in `.mindspec/domains/workflow/interfaces.md`. That path exists only in this repository.
- **#227 items 1–2 (no isolation, no fix-dispatch rule).** Reviewers that run tests share one worktree. A reviewer measured a tree that another reviewer had reverted. Nothing says when a fix agent may start.
- **#227 item 3 (stub verdicts).** A verdict has no `status` field. A preliminary stub counts as a final REQUEST_CHANGES.
- **Unvalidated verdicts.** Reviewers write `<slot>-round-<N>.json` by hand. The gate parses only `verdict` and `hard_block` (`internal/panel/tally.go:67`). No field says whether a finding was executed or only traced.
- **`panel verify` always exits 0** (`cmd/mindspec/panel.go:262`). An orchestrator cannot wait on it.

This spec changes four spec-130 rules. (1) R2.1 "today's substitution applies unchanged" is replaced: substitution happens only when a runner configures `on_quota` (R4.7). (2) R2.1's zero-config fill uses the runner table (R4.4). (3) R5.3's pinned-field list gains `verdict_schema` (R3.1). (4) R2.4's carried roster gains `family` (R4.6).

Dependency: #252 (skills cite an ADR) is fixed by a separate surface-leak change. R12 rewrites the same skill files. R12 lands after that change, or includes its edits.

## Impacted Domains

- **workflow**:
  - `internal/panel/**`: v2 schema and validator, submission binding in the gate, three ledger kinds, BRIEF regions, slot prompt, slot status.
  - `cmd/mindspec/**`: new `panel submit|schema|checkout|launch|recover|prompt`, `rules`, `config resolve`; `panel verify` exit code and `--json`; `doctor` checks.
  - `internal/instruct/**`: the `## Rules` section.
  - `internal/doctor/**`: launcher, tool-suggestion, staged-marker, ignore-line and checkout checks; the ownership and links walks skip the isolation root.
  - `internal/setup/**`, `plugins/mindspec/skills/**`, `.claude/skills/**`, `plugins/mindspec/workflows/ms-panel.js`, `.claude/workflows/ms-panel.js`, `plugins/mindspec/scripts/`: skill and workflow slimming; historical snapshots; pin tests.
- **core**: `internal/config/**` (`panel.runners`, `panel.lenses`, `panel.gates.<g>.lenses|acceptance`, `panel.isolation_root`, `.mindspec/config.local.yaml`, retired `panel.substitution`, `knownModels`); a new `internal/rules` package with the embedded rule table; `internal/bootstrap/**` (`.gitignore` lines); `cmd/mindspec/migrate.go` (walk skips the isolation root).
- **execution**: `internal/executor/**` (slot clone, `check-ignore`, `cat-file -e`).

Documentation surfaces: ADR-0037, ADR-0040, ADR-0043, ADR-0044 and ADR-0045 amendments (R13); `.mindspec/domains/workflow/interfaces.md` § Panel Artifact Schema; `CHANGELOG.md`.

## ADR Touchpoints

- [ADR-0045](../../adr/ADR-0045-binary-owns-sequence-and-rules.md) — AMENDED: step 1 (rule table) and the step-3 BRIEF and slot-prompt templates ship. Step 5 starts: `instruct` renders the rule table. The staged-rule marker form changes (R10.5).
- [ADR-0044](../../adr/ADR-0044-panel-review-conduct.md) — AMENDED: each Decision moves to a rule row (R10.2). Decision 4 changes: scratch goes in the slot or fixer directory, not `/tmp`.
- [ADR-0037](../../adr/ADR-0037-panel-gate-enforced-contract.md) — AMENDED: on a v2 round, the gate counts each slot's latest submission (R3). §8 applies: submit binds bytes to a ledger record and does not authenticate the caller.
- [ADR-0043](../../adr/ADR-0043-panel-disposition-telemetry-store.md) — AMENDED: three new ledger kinds (`verdict`, `launch`, `substitution`).
- [ADR-0040](../../adr/ADR-0040-orchestration-layering-ratchet.md) — AMENDED: launchers are config data (`panel.runners`). The `runner:` key keeps its meaning.
- [ADR-0036](../../adr/ADR-0036-ownership-discovery.md) — APPLIED (ZFC): quota detection matches only configured literal strings and the exit code. No code judges reviewer output. No code names a tool family.
- [ADR-0030](../../adr/ADR-0030-executor-boundary.md) — APPLIED: clone, `check-ignore` and `cat-file` calls go through the Executor. `internal/panel` stays free of git and `os/exec`.
- [ADR-0035](../../adr/ADR-0035-agent-error-contract.md) — APPLIED: every new refusal ends with one `recovery:` line.

## Requirements

**Terms.** Each term has this meaning only. Spec 130's terms (slot, roster, finding, ledger, legacy panel) apply unchanged.

- *v2 round*: a round whose `round` ledger event records `verdict_schema: 2`.
- *Runner*: one `panel.runners` entry. It tells the binary how to start a reviewer of one family. A family is a free label.
- *Slot directory*: the per-slot directory that R6 creates.
- *Submission*: a `verdict` ledger record that `panel submit` wrote.
- *Latest submission*: the last submission in ledger order for a slot and round.
- *Effective runner*: the runner of the slot's family, or of the `to_family` of the slot's latest `substitution` event in the round.

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
4. The validator has two layers.
   - *Schema layer*: types, required fields, enums, unknown-field refusal at every level, at most 100 items and at most 200 `commands_run` entries. The JSON Schema states exactly this layer.
   - *Transport and relation layer*: the file is at most 1 MiB; each `tail` is at most 4096 bytes in UTF-8; R1.1 `void_reason`, R1.2 and R1.3. The JSON Schema `description` lists these checks by name.
5. One Go definition produces the schema layer and the JSON Schema that `mindspec panel schema` prints.
6. Spec 130's finding model applies to the projection of a v2 verdict: item `i` is `ccr[i]`, its raw text is `text`, and the slot hash is the hash of the compact JSON array of the item texts.

### R2 — `panel submit`

1. `mindspec panel submit <slug> --slot S --round N --file F [--spec ID]` submits a verdict. `--file -` reads stdin.
2. Short form: `mindspec panel submit --file F`, run in a slot directory or under it. Submit takes the spec, slug, round and slot from the cwd path `<isolation_root>/<spec>/<slug>/r<N>/<slot>/…`. It never reads a `slot.json` under `tree/`. It refuses when the slot directory's own `slot.json` disagrees with the path, and when explicit flags disagree with the path.
3. Submit refuses each of these, and writes nothing:
   - a round that is not a v2 round (recovery: `mindspec panel create <slug> --round <N+1>`);
   - a file that fails R1 (the message names the JSON pointer of the first error);
   - a `slot` or `round` field that differs from the target slot and round;
   - a slot that is not on the roster, a round other than `panel.json`'s `round`, or an abandoned panel;
   - a `reviewed_sha` other than the current `reviewed_head_sha`, except for a same-bytes rerun (R2.5);
   - a `model` other than the slot's effective model (R4.6). The slot prompt states that value;
   - a direct submission when the slot's effective runner is a process runner (recovery: `mindspec panel launch` or `mindspec panel recover`).
4. A later submission for the same slot and round replaces the earlier one, at any status. The ledger keeps every `verdict` record. The gate reads only the latest submission.
5. Submit holds the panel ledger lock from the R2.3 checks to the end of the write. Under that lock it appends one `verdict` record, then writes the exact bytes to `<slot>-round-<N>.json` by temp file and rename. When the latest submission already has the same `file_sha256`, submit appends nothing, rewrites the file, and exits 0. This repairs a crash between append and rename.

   | `record` | Fields (plus `spec`, `panel`) | Key |
   |:---------|:-------|:----|
   | `verdict` | slot, round, status, verdict, family, model, via (`direct`, `launch` or `recover`), attempt, reason, file_sha256, created_at | slot, round, file_sha256 |

6. Submit prints the slot status (R8) and spec 130's rule line.
7. `panel disposition append` refuses the kinds `verdict`, `launch` and `substitution`. `panel disposition validate|check|query` accept them.

### R3 — The gate on a v2 round

1. `create` writes `verdict_schema: 2` to `panel.json` and to the `round` event of every new round. No config key or flag changes it. This spec adds `verdict_schema` to spec 130 R5.3's pinned-field list.
2. The gate takes v2-ness from the latest `round` event only, never from `panel.json`. When an earlier `round` event of the panel has `verdict_schema: 2` and the latest one does not, the gate blocks with a registration error.
3. On a v2 round, a roster slot is in only when its latest submission is `final`, its verdict is not `VOID`, and the sha256 of `<slot>-round-<N>.json` equals that submission's `file_sha256`.
4. A verdict file with no matching latest submission counts for nothing. Spec 130 R2.5 (an off-roster REJECT, INVALID or `hard_block` blocks) applies unchanged.
5. Spec 130's pass rule, discharge rules and coverage manifest use only the slots that are in. A discharge applies while its `finding_sha256` equals a finding text in the latest submission. When a replacement removes or changes that text, the finding is open again.
6. A round whose `round` event has no `verdict_schema` keeps spec 130's rules. Such a round only exists from before this change.

### R4 — Runners and slot resolution

1. `panel.runners` is an ordered list. Each runner has `family` (unique, non-empty) and `kind`.
   - `kind: harness`: the orchestrating harness dispatches the slot with its own agent tool. The runner can also have `on_quota`.
   - `kind: process`: the binary starts the reviewer. The runner needs `argv`, `prompt` (`stdin`, `file` or `arg`) and `output` (`stdout` or `file`). It can have `quota_signals` (literal strings), `on_quota` (another family), `probe` (an argv) and `timeout_minutes`.
2. `argv[0]` is a literal with no placeholder. Each other element is a literal or exactly one placeholder: `{prompt}`, `{prompt_file}`, `{model}`, `{slot_dir}` or `{tree}`. A placeholder expands to one whole element. `prompt: file` needs `{prompt_file}`, and `prompt: arg` needs `{prompt}`.
3. Load refuses any other placeholder, a placeholder inside a longer element, a duplicate family, and an `on_quota` that names the runner's own family or an undeclared family.
4. The shipped default is one harness runner with family `harness`. The binary ships no process runner and no runner named after a tool. Spec 130 R2.1's zero-config fill goes round-robin over the runner order. With zero config, every slot has family `harness`.
5. No non-test Go file and no shipped data file in `internal/config`, `internal/panel` or `cmd/mindspec/panel*.go` contains a tool family name. `knownModels` loses its two family names.
6. Each roster entry gains `family`. `create` takes it from the reviewer entry. A slot's model is the reviewer entry's `model`, else `models["reviewer.<family>"]`, else the family string. The effective model is the model of the effective family, resolved the same way.
7. `create` refuses a v2 roster slot with an empty family or a family with no runner. The recovery names the reviewer entry and `panel.runners`.
8. The first v2 round of a panel copies the prior roster (spec 130 R2.4). For a carried slot with no `family`, `create` sets `family` to the slot's model only when a runner has exactly that family. Otherwise `create` refuses. The recovery is `mindspec panel create <slug> --round <N+1> --reviewers <n>`, which takes a fresh roster from config. `create` never changes an earlier round record.
9. `panel.substitution` is retired. Load refuses it with the recovery `set on_quota on the runner in panel.runners`. Substitution happens only when a runner has `on_quota`.
10. `.mindspec/config.local.yaml` is optional and machine-local. It can set only `panel.runners` and `models`. A local runner replaces the runner of the same family, or is appended. Every command refuses when the file is over 65536 bytes, does not parse, has another key, or cannot be read for a reason other than absence. `mindspec init` adds the file to `.gitignore`.
11. `mindspec config resolve --gate G [--json]` prints each slot's `slot family model lens kind`. Each value shows its source: `config.yaml`, `config.local.yaml` or `default`. It is a preview of a new panel. It is not the roster of an existing panel (R8.5).
12. A data file lists known review tools as `{name, command}` pairs. When a listed `command` is on `PATH` and no runner uses it as `argv[0]`, `config resolve` and `doctor` print one suggestion line. The line asks the operator to configure a process runner for review diversity. It is advice only. Nothing requires a second family.

### R5 — Lenses and acceptance questions as config

1. `panel.lenses` maps a lens name to its focus text. `panel.gates.<g>.lenses` is the ordered lens list for gate `g`. `panel.gates.<g>.acceptance` is `{question, not_blocking[]}`.
2. A `panel.gates.<g>` entry is valid when it sets at least one of `reviewers`, `approve_threshold`, `lenses` or `acceptance`.
3. A lens-less slot takes the next lens from its gate's list, with spec 112's cursor rule. An explicit reviewer `lens` keeps its value. Load refuses a gate lens name that `panel.lenses` does not define.
4. The shipped defaults are data. The focus texts come from the current `ms-panel-run` lens tables, with every language, library and tool name removed:

   | Lens | Gates | Focus |
   |:-----|:------|:------|
   | author-of-record | bead, final_review, adhoc | Does the change do what its plan says, and nothing else? |
   | empirical-prober | bead, final_review, adhoc | Run the tests and validators. Do the claims hold? |
   | codebase-pin | bead, final_review, adhoc | Do the named files, symbols and tests exist and pass? |
   | adversarial | bead, final_review, adhoc | Which input, order or failure makes this change wrong? |
   | contract-stability | bead, final_review, adhoc | Do types, schemas, outputs and messages stay compatible? |
   | integration | bead, final_review, adhoc | Can the next unit of work use this change without rework? |
   | falsifiability | spec_approve, plan_approve | Is every acceptance criterion concrete and testable? |
   | scope-honesty | spec_approve, plan_approve | Do the scope and impacted domains match what the document asks for? |
   | contradiction | spec_approve, plan_approve | Do two requirements, or a requirement and a non-goal, conflict? |
   | feasibility | spec_approve, plan_approve | Is the approach sound and in proportion to its risk? |

5. Each gate has a default acceptance question:

   | Gate | Question | Not blocking |
   |:-----|:---------|:-------------|
   | spec_approve | Can a planner write a plan from this spec without inventing a requirement? | wording style; a scope item that the spec names as another spec's |
   | plan_approve | Does every bead trace to a requirement, and can each bead be built and tested alone? | estimates; names |
   | bead | Does the change meet the bead's acceptance criteria with passing tests, and break nothing else? | style the linter accepts; defects outside the diff (file a follow-up) |
   | final_review | Does the branch meet every acceptance criterion, and does the project's CI command pass? | style already reviewed per bead |
   | adhoc | Is the change correct, and does it do what its description says? | style |

6. The `defaultLenses` Go literal goes.

### R6 — Slot clones and scratch (#227, #253)

1. `panel.isolation_root` defaults to `.mindspec-scratch`, relative to the main working tree root. Load refuses an empty value, an absolute value, `.`, a value with `..`, a value under `.git`, `.worktrees/` or `.mindspec/`, and a value that `git ls-files` reports as tracked.
2. `panel checkout` and `panel launch` refuse when a component of the resolved root is a symlink, and when `git check-ignore` does not ignore the root (recovery: `mindspec init`). `mindspec init` adds the default root to `.gitignore` on a new or existing install.
3. A slot directory is `<isolation_root>/<spec or adhoc>/<slug>/r<N>/<slot>/`. It holds `slot.json`, `prompt.md` and `tree/`. `tree/` is `git clone --shared --no-checkout` of the repository, then a detached checkout of the reviewed SHA. The clone's git directory is inside `tree/`, so a reviewer CLI that may write only under its cwd can run git in the tree.
4. `mindspec panel checkout <slug> (--slot S | --all) [--spec ID]` creates slot directories and prints each absolute path. A rerun reuses a directory whose tree `HEAD` is the reviewed SHA. It refuses an abandoned panel, an off-roster slot, and a tree at another SHA (recovery: `mindspec panel checkout <slug> --prune --slot S`).
5. For a slot whose effective runner is a harness runner, `panel checkout --slot S` also appends a `launch` event with status `dispatched`.
6. `panel checkout <slug> --fixer` creates `<…>/r<N>/fixer/` with no tree. It is scratch only. The fixer works in the target's own worktree: the bead worktree for a bead panel, the spec worktree for a final review.
7. `panel checkout <slug> --prune [--slot S]` removes the checkouts of the panel, or of one slot. `create` for round N+1 removes the panel's checkouts of earlier rounds. `abandon` removes all. Removal deletes the directory, dirty or not, under the verified root, and does not follow symlinks.
8. These walks skip the resolved isolation root: panel registration lookup, the `doctor` ownership walk, the `doctor` links walk, and the `migrate` walks. No ledger record contains an isolation-root path.

### R7 — `panel launch` and `panel recover`

1. `mindspec panel launch <slug> <slot> [--spec ID] [--replace]` starts the slot's effective process runner and waits. It refuses: a slot whose effective runner is a harness runner (recovery: `mindspec panel prompt`); an abandoned panel; a round that is not a v2 round; a slot whose latest submission is final, unless `--replace` is set; a slot whose lock another launch holds (recovery: `mindspec panel verify`).
2. Launch holds an exclusive lock on the slot directory until it exits. Each start is a new attempt `k`, in `attempt-<k>/` in the slot directory. Launch never reads files of an earlier attempt.
3. For each attempt, launch:
   - creates a random nonce and writes `prompt.md` with it (R9.3);
   - appends a `launch` event with status `started`, `attempt`, `runner_source`, `argv_sha256` and `nonce_sha256`;
   - runs `argv` with no shell, with the slot directory as cwd, and with every `MINDSPEC_` variable removed from the environment;
   - writes stdout to `attempt-<k>/stdout.log` and stderr to `attempt-<k>/stderr.log`;
   - kills the process group at `timeout_minutes`.
4. Launch records one outcome:
   - `submitted`: exit 0 before the timeout, and R2 accepts the verdict. With `output: stdout`, the verdict is the one block in `stdout.log` between `MINDSPEC-VERDICT-BEGIN <nonce>` and `MINDSPEC-VERDICT-END <nonce>`. With `output: file`, it is `attempt-<k>/verdict.json`;
   - `quota_wall`: a non-zero exit or a timeout, and the last 64 KiB of `stderr.log` contains a configured quota signal;
   - `launch_failed`: any other non-zero exit or timeout;
   - `malformed`: exit 0 and no block, more than one block, a block with another nonce, or an R2 refusal.
5. On the first `malformed`, launch starts one more attempt. Its prompt gets the refusal message. A second `malformed` ends the launch.
6. On `quota_wall`, when the runner has `on_quota`, launch appends a `substitution` event. A process substitute is started once as a new attempt. For a harness substitute, launch prints the `panel prompt` command and exits non-zero.

   | `record` | Fields (plus `spec`, `panel`) | Key |
   |:---------|:-------|:----|
   | `launch` | slot, round, attempt, family, model, status, exit_code, runner_source, argv_sha256, nonce_sha256, created_at | slot, round, attempt, status |
   | `substitution` | slot, round, from_family, to_family, model, created_at | slot, round, to_family |

7. Launch exits 0 only when it recorded `submitted`.
8. `mindspec panel recover <slug> <slot> --reason R [--spec ID]` repeats the R7.4 extraction on the latest attempt only, with any exit code. The block nonce must hash to that attempt's `nonce_sha256`. The submission has `via: recover` and the reason. Recover starts no process. It refuses when the latest attempt has no output.

### R8 — `panel verify` slot status

1. `panel verify` gives each roster slot one status from this closed set: `final`, `damaged`, `void`, `draft`, `running`, `quota_wall`, `launch_failed`, `malformed`, `unsubmitted`, `missing`.
2. On a v2 round, the status comes from the first rule that applies:
   - the latest submission is final and not `VOID`: `final` when the file matches its `file_sha256`, else `damaged` (recovery: `mindspec panel submit` with the original bytes, or `mindspec panel recover`);
   - the latest event among the slot's submissions and its `launch` events, ignoring `submitted`, gives the status: `VOID` gives `void`; `draft` gives `draft`; `started` or `dispatched` gives `running`; `quota_wall`, `launch_failed` and `malformed` give their own names;
   - with no such event, a verdict file gives `unsubmitted`, and no file gives `missing`.
3. A slot with a substitution also shows `substituted <from>-><to>`. A slot whose effective runner comes from `config.local.yaml` also shows `local runner`.
4. On another round, a parseable verdict file gives `final`, an unparseable file gives `malformed`, and no file gives `missing`.
5. `--json` prints, for each slot: slot, family, model, lens, effective kind, status, and annotations. Adapters take the roster of an existing panel from this output.
6. Exit codes: 0 when every slot is `final`; 1 when a slot is `running` or `missing`, or is `draft` with a harness effective runner; 2 in every other case. Verify prints the rule line and the PASS/BLOCK preview, and writes nothing.
7. Every reviewer-supplied string that submit, verify or prompt prints goes through `termsafe`.

### R9 — BRIEF and slot prompt (#254)

1. `create` renders these machine-owned BRIEF regions:
   - *acceptance*: the gate's acceptance question and not-blocking list (R5);
   - *spec ACs*: the `## Acceptance Criteria` section of `spec.md` at the reviewed SHA, as a fenced, `termsafe`-escaped quotation, or `none (adhoc panel)`, or `no acceptance criteria section found`;
   - *conduct*: each reviewer-scope rule for the gate, as `id: prohibition — replacement` (R10);
   - *verdict contract*: the R1 fields, `mindspec panel schema` and `mindspec panel submit`.
2. The quoted spec-AC text is the reviewed subject, not text the binary authors. R9.3 and R10.7 do not apply inside it.
3. Text that the binary authors names no documentation path and no internal reference. Each repository path it names must exist at the reviewed SHA. `create` checks this and refuses on a missing path. Paths inside the quotation are not checked.
4. `mindspec panel prompt <slug> --slot S [--spec ID]` prints the slot prompt and writes nothing. The prompt holds:
   - the BRIEF;
   - the slot's id, effective family, effective model (the value for `model`), lens and focus;
   - the absolute slot directory and tree;
   - the output instruction for the effective runner: the nonce marker lines, `verdict.json`, or the `panel submit` command;
   - from round 2, the slot's own prior-round items, each to grade ADDRESSED, PARTIAL, MISSED or NEW_ISSUE;
   - the instruction to write scratch only in the slot directory.

### R10 — Rule table

1. One embedded YAML file holds the rule table. Each rule has `id`, `scope` (a subset of `orchestrator`, `implementer`, `reviewer`, `fixer`), `prohibition`, `replacement`, `enforced_by` (`advisory` or a mindspec verb name), and optional `gates` and `target` (the planned home of an advisory rule).
2. The table holds at least these rules:

   | id | scope | rule (summary) | enforced_by |
   |:---|:------|:---------------|:------------|
   | `panel.verbs-only` | all | no hand edit of `panel.json` or a verdict file; use the panel verbs | `complete` |
   | `panel.submit` | reviewer | no direct write of a verdict file; use `panel submit` or the output channel in the prompt | `panel submit` |
   | `panel.isolated-checkout` | reviewer | no test run or edit in a shared worktree; use the slot tree | advisory |
   | `panel.scratch-in-slot` | reviewer, fixer | no scratch in a system temp directory or at a bare relative path; use the slot or fixer directory | advisory |
   | `panel.fix-after-final` | orchestrator | no fix agent before `panel verify` exits 0 | advisory |
   | `panel.one-fixer` | orchestrator | one fix agent for each panel round, in the target's own worktree, never in a slot tree | advisory |
   | `panel.artifact-gate` | orchestrator | a finding that names a missing measurement artifact stays open until the artifact exists; a text edit does not close it | advisory |
   | `panel.clean-before-complete` | orchestrator | no `mindspec complete` on a dirty worktree | `complete` |
   | `review.basis` | reviewer | no finding without its basis; an executed finding names its command | `panel submit` |
   | `review.property` | reviewer | state the violated property, not only a remedy | advisory |
   | `review.mutation` | reviewer | no guard accepted without a mutation that makes it fail | advisory |
   | `review.stated-limit` | reviewer | state what you did not check | advisory |
   | `review.ci-parity` | reviewer (`gates: final_review`) | run the project's `commands.ci`, else `commands.test`, verbatim; when neither is set, say so in the rationale | advisory |
   | `review.reproduce` | orchestrator | reproduce each blocking finding in an isolated checkout before a fix dispatch or a refutation; weight the deeper trace | advisory |
   | `review.no-vote` | all | an APPROVE count resolves no finding; fix, refute or defer it | `complete` |
   | `review.classifier-redispatch` | orchestrator | when a reviewer refuses a security-framed lens, dispatch the same slot again with a correctness framing | advisory |
   | `review.derive` | implementer, fixer | derive from the source of truth; do not restate it | advisory |
   | `lifecycle.no-probe-complete` | orchestrator | no `mindspec complete` to test a gate; use `panel verify` and `mindspec validate` | advisory |
   | `lifecycle.complete-by-orchestrator` | implementer, fixer | no `mindspec complete`, `bd close` or raw bead merge by a subagent | advisory |
   | `sequence.bead-impl-prompt` | orchestrator | stage the implementation prompt before dispatch | advisory (`target: next-action`) |

3. The rule "a finding stands until its raiser withdraws it" is declined. Spec 130's refute and defer verbs are the only discharge.
4. `mindspec rules [--scope S] [--json]` lists rules. `mindspec rules show <id>` prints one rule. An unknown id or scope refuses.
5. A staged-rule marker is `<!-- staged-rule: id=<rule-id>; remove-by=YYYY-MM-DD -->`. The `ms-bead-impl` marker uses this form with `id=sequence.bead-impl-prompt`. The pins in `internal/setup/readiness_skill_pin_test.go` (`readinessStagedRule`) and `internal/setup/review_discipline_test.go` change with it.
6. `instruct` renders a `## Rules` section: orchestrator-scope rules in every mode, plus implementer-scope rules in implement mode. `ms-bead-fix` puts the output of `mindspec rules --scope fixer` in the fixer prompt.
7. Rule text, rule ids, the shipped data files and every string that R2–R12 add to the product surface cite no ADR, spec number, requirement id, bead id or issue number.

### R11 — Doctor checks

1. For each process runner that a resolved roster of any gate uses, `doctor` checks that `argv[0]` is on `PATH`. A failure is an Error.
2. `doctor` runs a runner's `probe` only when the runner comes from `config.local.yaml`, or under `doctor --probe`. A probe must exit 0 within 10 seconds, else Error. A probe from `config.yaml` without `--probe` gets one Info line that says it was not run.
3. `doctor` prints the R4.12 tool suggestion as Info.
4. `doctor` warns when `.mindspec/config.local.yaml` is tracked, when an existing config file or isolation root is not ignored (recovery: `mindspec init`), and when config sets the retired `panel.substitution`.
5. `doctor` warns about each staged-rule marker in an installed skill that has no id, an unknown id, or a past `remove-by` date. It lists the rules whose `enforced_by` is not `advisory` (Info).
6. `doctor` warns about a slot directory whose panel is missing, abandoned, or at a later round. The recovery is `mindspec panel checkout <slug> --prune`.
7. `doctor --ci` skips R11.1, R11.2, R11.3 and R11.6.

### R12 — Skill and workflow slimming

1. Both copies of `ms-panel-run` have at most 80 lines. For each `runner:` value:
   - `claude-code-workflow`: run the `/ms-panel` workflow once with `{slug, spec, target, bead_id, round}`;
   - `claude-code-skills` and `external`: `panel create`; `panel verify --json` for the roster; for each process slot, `panel launch` in the background; for each harness slot, `panel checkout --slot`, `panel prompt --slot` and a dispatch with this harness's agent tool; wait; on verify exit 2, relaunch or report; on exit 0, run `ms-panel-tally`.
2. Both copies of `ms-panel-tally` have at most 80 lines. Each current section is kept short or replaced:

   | Section | Fate |
   |:--------|:-----|
   | Steps 1 (tally), Then | kept |
   | Workflow-path note | cut; the workflow returns verify and tally output |
   | Step 2 consolidate | kept; contested re-verify becomes `review.reproduce` |
   | Step 3 dispositions | kept; the JSON goes on stdin (`--data -`) from a quoted heredoc, with no temp file |
   | Step 4 report | kept; family counts come from `panel verify --json` |
   | Artifact gates | becomes `panel.artifact-gate` |
   | After a halt | kept as one line per verb: `instruct --panel-state`, `create --round`, `rebind`, `abandon`, `refute` |
   | Escape hatch | kept, two lines |
   | Anti-patterns | become `review.no-vote`; the confidence note goes (v2 has no `confidence`) |
   | Defer a finding | kept, three lines; the verb enforces the rest |

3. No copy of `ms-panel-run`, `ms-panel-tally`, `ms-bead-fix`, `ms-spec-final-review`, `ms-bead-cycle` or `ms-bead-impl` contains `/tmp`. None contains a tool family name, except the harness's own name in a path such as `.claude/`. These skills contain no lens table, no size ladder and no failure-detection section. They cite rule ids for conduct. `ms-bead-cycle`'s family table becomes family-neutral.
4. `ms-spec-final-review` uses `panel checkout --slot` for isolated reviewer work. `ms-bead-fix` uses `panel checkout --fixer` for scratch and dispatches the fixer in the target's own worktree (R6.6).
5. Both copies of `ms-panel.js` name no tool family. `ALLOWED_CLI` is exactly `mindspec panel create`, `mindspec panel verify`, `mindspec panel tally`, `mindspec panel launch`, `mindspec panel checkout` and `mindspec panel prompt`. `VALID_MIX_FAMILIES`, the substitution flag and the parse-retry ladder go. The workflow takes the roster from `panel verify --json` after `create`. A harness reviewer runs `mindspec panel submit` itself, as its prompt says.
6. `plugins/mindspec/scripts/codex_verdict_extract.sh` is deleted. `panel recover` replaces it.
7. `setup`'s historical skill snapshots gain the replaced versions of every R12.3 skill, so an unmodified installed copy refreshes.

### R13 — ADR and document amendments

Each amendment is an appended `## Amendment (Spec 131)` section. It lands with the bead that ships the code it describes. A text test pins it.

1. ADR-0045: steps 1 and 3 (BRIEF and slot prompt) shipped; step 5 started (R10.6); the R10.5 marker form.
2. ADR-0044: each Decision maps to its R10.2 rule; Decision 4 now says the slot or fixer directory.
3. ADR-0037: R3 counting (latest submission, replacement, discharge by text), the pinned `verdict_schema` field, and the §8 statement for submit.
4. ADR-0043: the three new record kinds.
5. ADR-0040: `panel.runners`, `.mindspec/config.local.yaml`, and no shipped tool runner.
6. `interfaces.md` § Panel Artifact Schema describes v2, submit and slot status. `CHANGELOG.md` names: v2 on every new round with no opt-out (a verdict file with no submission no longer counts); the latest submission replaces earlier ones; zero config runs every slot on the harness, and no tool runner ships; `panel.substitution` is retired; the `panel verify` exit codes; the new spec and plan default lenses; the first-v2-round roster recovery.

## Scope

### In Scope

- Every file in Impacted Domains, and the documentation surfaces listed there.

### Out of Scope

- Every tracked legacy panel directory. No file in them changes.
- `.mindspec/domains/**` files other than `workflow/interfaces.md`.

## Non-Goals

- Authentication of a submitter. Submit binds bytes to a ledger record. Any process with write access to the repository can submit, or can append to the ledger file. ADR-0037 §8 applies.
- A round limit, `next-action`, `bead prompt`, the implementation and fix prompt templates, `config resolve --role` and the drafting and implementation model keys (spec 133).
- A family minimum, `exclude_author_family`, a binding of a harness runner to a named harness, and any rule that judges reviewer output (operator ruling).
- Repository overrides of the BRIEF or prompt templates (ADR-0045 Decision 3 override path).
- In the BRIEF: open linked beads, the review-bar projection and a measurement contract (follow-up bead).
- `panel.gates.<g>.families` (follow-up bead; the operator ruling makes it advice at most).
- `complete --dry-run`, test-selection rules, and the scope-declaration read-back for implementers (#249 part 2; spec 133).
- #252 (the separate surface-leak change).
- Stub skills from a canonical source, and the skill-drift checks (ADR-0045 step 6).

## Acceptance Criteria

- [ ] **AC-1 (R1)**: The validator accepts a fixture of each verdict kind. It refuses each of these, naming the JSON pointer: no `status`; `verdict: "MAYBE"`; `VOID` without `void_reason`; `void_reason` on an APPROVE; an unknown top-level field; an unknown item field; an `executed` item without `command`; `command: 5` with 2 commands; a `traced` item with `command`; an APPROVE with one item; a REQUEST_CHANGES with none; a compact 1 MiB + 1 byte file. A valid object padded with spaces past 1 MiB passes the JSON Schema and fails the validator. A `tail` of 4096 bytes with multibyte characters passes; 4097 bytes fails. On the schema-layer fixtures, `panel schema` output and the validator agree. Tests: `TestVerdictV2_Accepts`, `TestVerdictV2_Refusals`, `TestVerdictV2_TransportLayer`, `TestVerdictV2_SchemaMatchesValidator`.
- [ ] **AC-2 (R1.6)**: A v2 REQUEST_CHANGES with items `a` and `b` gives findings `ccr[0]` and `ccr[1]` with the sha256 of `a` and `b`. A `panel refute --finding ccr[1]` discharges `ccr[1]` only. Test: `TestVerdictV2_FindingProjection`.
- [ ] **AC-3 (R2)**: On a v2 round:
  - submit writes the file and one `verdict` record; a same-bytes rerun adds no record;
  - each R2.3 refusal leaves the panel directory and ledger byte-identical; the v1-round refusal names `--round`;
  - a final APPROVE, then a final REQUEST_CHANGES, is accepted, and the ledger holds both records;
  - with a crash injected after the append and before the rename, a same-bytes rerun makes the slot `final` and adds no record;
  - two concurrent submits for one slot leave one file whose sha256 equals the latest record;
  - submit from `<slot_dir>/tree/x/`, where `tree/x/slot.json` names another slot, submits for the path's slot;
  - a direct submit on a process slot refuses; a `model` other than the effective model refuses;
  - `panel disposition append` refuses each new kind.

  Tests: `TestPanelSubmit_Writes`, `TestPanelSubmit_Refusals`, `TestPanelSubmit_Replace`, `TestPanelSubmit_CrashRepair`, `TestPanelSubmit_Race`, `TestPanelSubmit_SlotDirPath`, `TestDispositionAppend_RefusesV2Kinds`.
- [ ] **AC-4 (R3)**: On a v2 round with 4 roster slots:
  - 4 submitted final APPROVE files pass;
  - a hand-written file in place of a submission blocks, and verify shows `unsubmitted`;
  - a submitted file changed by one byte blocks, and verify shows `damaged`;
  - a draft blocks; a `VOID` blocks;
  - an APPROVE replaced by a REQUEST_CHANGES blocks;
  - a refuted `ccr[0]`, then a replacement with the same item text: the refutation still applies;
  - the same, with the item text changed: the finding is open again;
  - `panel.json` with `verdict_schema` deleted blocks; a v2 panel whose newest `round` event lacks the field blocks;
  - a fixture round with no `verdict_schema` keeps spec 130's rules; with any config, `create` writes `verdict_schema: 2`.

  Tests: `TestGateV2_SubmittedOnly`, `TestGateV2_DigestBinding`, `TestGateV2_DraftAndVoidMissing`, `TestGateV2_ReplacementAndDischarge`, `TestGate_VerdictSchemaPinned`, `TestGate_PreV2RoundKeepsV1Rules`, `TestCreate_AlwaysV2`.
- [ ] **AC-5 (R4.1–R4.5, R4.9)**: Load refuses: a duplicate family; a harness runner with `argv`; a process runner without `output`; `prompt: file` without `{prompt_file}`; a `{shell}` placeholder; `x{tree}`; `argv[0]` = `{tree}/run`; `on_quota` set to its own family; `panel.substitution` (the recovery names `on_quota`). Zero-config `bead` gives 4 slots, all family `harness`. With runners `a` and `b` and no reviewers, zero-config `bead` is a, b, a, b. No non-test Go file or shipped data file in the R4.5 paths matches the tool-name list. Tests: `TestRunners_Validate`, `TestRunners_ZeroConfigHarness`, `TestRunners_ZeroConfigOrder`, `TestSubstitution_Retired`, `TestNoFamilyLiterals`.
- [ ] **AC-6 (R4.6–R4.12)**:
  - `create` writes `family` on each roster entry; a slot with only `family: b` and `models: {reviewer.b: m1}` has model `m1`;
  - `create` refuses a model-only reviewer entry, and a family with no runner;
  - a pre-change panel whose roster has model `harness` and no family gets family `harness` at its first v2 round; one with model `m9` and no runner `m9` refuses with `--reviewers`; earlier round records stay byte-identical;
  - a local file that replaces runner `b` changes `config resolve` output, and the source column says `config.local.yaml`;
  - a local file with `loop:`, a 65537-byte local file, and an unparseable local file each make `mindspec config show` refuse;
  - with a listed tool command on `PATH` and no runner for it, `config resolve` prints one suggestion line; without it, none.

  Tests: `TestRoster_Family`, `TestSlotModelResolution`, `TestCreate_RefusesUnrunnableSlot`, `TestCreate_FirstV2RoundFamily`, `TestLocalConfig_Merge`, `TestLocalConfig_Refusals`, `TestConfigResolve_Provenance`, `TestConfigResolve_ToolSuggestion`.
- [ ] **AC-7 (R5)**: Zero-config `spec_approve` slots get the four document lenses in order, then repeat. Zero-config `bead` slots get the first four R5.4 lenses. A gate entry with only `lenses` loads. Load refuses `panel.gates.bead.lenses: [nope]`. The BRIEF of each gate holds its R5.5 question. No default focus text contains a tool, language or library name. Tests: `TestLenses_GateDefaults`, `TestLenses_LensOnlyGateEntry`, `TestLenses_UnknownRefused`, `TestBrief_AcceptanceRegion`.
- [ ] **AC-8 (R6)**:
  - `panel checkout --all` on a 4-slot panel creates 4 slot directories, each with `slot.json` and a `tree/` whose `HEAD` is the reviewed SHA; a rerun changes nothing;
  - in a slot tree, `git status` and `git stash` change nothing in the main repository's git directory;
  - a tree at another SHA refuses with `--prune --slot` in the recovery; `--prune --slot R2` leaves R1's tree;
  - a dirty tree is removed by `create --round 2`; `abandon` removes all;
  - a harness slot checkout appends one `dispatched` event;
  - load refuses an empty, absolute, `.`, `.git/x`, `.worktrees/x` and tracked root; checkout refuses a symlinked root and a non-ignored root (recovery `mindspec init`);
  - on an existing install, `mindspec init` adds the ignore line;
  - a `panel.json` and an OWNERSHIP-matching file under the root are not seen by `panel tally`, the `doctor` ownership walk or the links walk.

  Tests: `TestPanelCheckout_Creates`, `TestPanelCheckout_GitStaysInSlot`, `TestPanelCheckout_Refusals`, `TestPanelCheckout_Prune`, `TestPanelCheckout_HarnessDispatched`, `TestIsolationRoot_Validate`, `TestBootstrap_ScratchIgnoredExisting`, `TestWalks_SkipIsolationRoot`.
- [ ] **AC-9 (R7)**: With a fake runner script per case:
  - a valid nonce block on stdout and exit 0 gives `submitted`; so does a valid `verdict.json` in file mode;
  - a valid block on stderr only gives `malformed`; a block with another nonce gives `malformed`; two blocks give `malformed`;
  - a valid block with exit 1 gives `launch_failed`, and `recover --reason r` then submits it with `via: recover`;
  - exit 2 with the quota string on stderr gives `quota_wall`; the same string on stdout only gives `launch_failed`; a timeout gives `launch_failed`;
  - the first `malformed` starts one more attempt whose prompt holds the refusal; a second `malformed` stops;
  - on `quota_wall` with `on_quota: b`, one `substitution` event is appended, the slot starts on `b` in a new attempt, and verify shows `substituted a->b`; a stale `verdict.json` from the `a` attempt is not submitted when `b` fails;
  - with no `on_quota`, a quota wall starts no other runner;
  - a draft verdict and a `VOID` verdict from a launch give verify status `draft` and `void`;
  - the child environment holds no `MINDSPEC_` variable;
  - a second concurrent launch of the slot refuses; a harness slot refuses, naming `panel prompt`;
  - launch exits 0 only for `submitted`; no ledger record contains the isolation root.

  Tests: `TestPanelLaunch_Outcomes`, `TestPanelLaunch_Channels`, `TestPanelLaunch_Retry`, `TestPanelLaunch_Substitution`, `TestPanelLaunch_DraftAndVoid`, `TestPanelLaunch_Env`, `TestPanelLaunch_Refusals`, `TestPanelRecover`, `TestLedger_NoScratchPaths`.
- [ ] **AC-10 (R8)**: A fixture with one slot in each R8.1 status gives that status. Verify exits 0 when every slot is `final`, 1 with one `running` slot, and 2 with one `launch_failed` slot and no pending slot. A v1 round with all files parseable exits 0. `--json` gives the same statuses, and each slot's family, model, lens and kind. An ESC byte in a verdict `model` is escaped in verify output. Verify writes nothing. Tests: `TestPanelVerify_SlotStatus`, `TestPanelVerify_ExitCodes`, `TestPanelVerify_V1Round`, `TestPanelVerify_JSONRoster`, `TestPanelVerify_Termsafe`.
- [ ] **AC-11 (R9)**: A spec fixture whose AC section holds an ADR citation, `.mindspec/domains/x.md`, a file to create and a file to delete:
  - `create` succeeds, and the BRIEF quotes the section verbatim inside its fence;
  - outside the quotation, the BRIEF contains no `.mindspec/domains/` and no internal reference;
  - the conduct region holds every reviewer-scope rule id for the gate; `review.ci-parity` appears only for `final_review`;
  - the verdict contract names `mindspec panel submit`;
  - a template path absent at the reviewed SHA makes `create` refuse;
  - a spec with no AC section renders `no acceptance criteria section found`; an ESC byte in an AC is escaped;
  - `panel prompt` for a process slot names the nonce marker lines; for a harness slot it names `panel submit`; at round 2 it lists that slot's round-1 items only, with an ESC byte escaped.

  Tests: `TestBrief_QuotedACs`, `TestBrief_Regions`, `TestBrief_PathCheck`, `TestPanelPrompt_Render`, `TestPanelPrompt_PriorRound`.
- [ ] **AC-12 (R10)**: The table loads and holds every R10.2 id. `rules --scope reviewer` lists exactly the reviewer-scope rules. `rules show nope` refuses. `instruct` in implement mode lists `lifecycle.no-probe-complete` and `review.derive`; in spec mode it lists the first and not the second. `ms-bead-fix` renders the fixer-scope rules into its prompt. The `ms-bead-impl` marker parses with `id=sequence.bead-impl-prompt`. No rule field, no shipped data file, and no R2–R12 string literal in non-test code matches `ADR-[0-9]`, `[Ss]pec [0-9]{3}`, `\bR[0-9]+\.[0-9]`, `#[0-9]{3}` or the bead-id pattern. Tests: `TestRuleTable_MinimumSet`, `TestRulesCmd`, `TestInstruct_RulesSection`, `TestBeadFix_FixerRules`, `TestStagedMarker_Parse`, `TestProductSurface_NoInternalRefs`.
- [ ] **AC-13 (R11)**:
  - a process runner whose `argv[0]` is not on `PATH` gives an Error naming the family;
  - a `probe` in `config.yaml` that creates a marker file is not run by plain `doctor`, and gives an Info line; under `--probe` it runs; a local-config probe that exits 1 gives an Error;
  - a tracked `config.local.yaml`, a non-ignored isolation root, and `panel.substitution` each give a Warn;
  - a marker with `remove-by=2020-01-01` and a marker with an unknown id each give a Warn;
  - a slot directory of an abandoned panel gives a Warn with `--prune`;
  - `doctor --ci` gives none of the R11.7 lines.

  Tests: `TestDoctor_Launchers`, `TestDoctor_ProbeNotRunFromTrackedConfig`, `TestDoctor_ConfigWarnings`, `TestDoctor_StagedMarkers`, `TestDoctor_StaleCheckouts`, `TestDoctor_CISkips`.
- [ ] **AC-14 (R12)**: `wc -l` is 80 or less for both copies of `ms-panel-run` and `ms-panel-tally`. For every R12.3 skill copy, `grep -n -i -E '/tmp|codex|copilot|gemini'` prints nothing, and every `claude` match is a path token such as `.claude/`. Both `ms-panel.js` copies pass the same grep, and `ALLOWED_CLI` equals the R12.5 set. `codex_verdict_extract.sh` is absent. `mindspec setup claude` over an unmodified spec-130 install replaces every R12.3 skill. Tests: `TestPanelSkills_AdapterShape`, `TestSkills_NoFamilyNoTmp`, `TestMsPanelWorkflow_AllowedCLIExactSet`, `TestSetup_RefreshesSpec130Skills`.
- [ ] **AC-15 (R13)**: ADR-0037, ADR-0040, ADR-0043, ADR-0044 and ADR-0045 each contain `## Amendment (Spec 131)` with the R13 items. `interfaces.md` § Panel Artifact Schema names `panel submit` and the R8.1 statuses. `CHANGELOG.md` names the seven R13.6 changes. Test: `TestADRAmendments_Spec131`.
- [ ] **AC-16 (method)**: `go build ./... && go test ./... -short` is green, including `TestPanelLeafImports_StdlibPlusTermsafeOnly` and `TestEnforcementHasNoGitLeaks`.

## Validation Proofs

- In a scratch repo with a 2-slot zero-config v2 bead panel: `mindspec panel verify p; echo $?` prints `missing` twice and `1` (AC-10).
- `mindspec panel verify p --json` lists two slots with family `harness` and kind `harness` (AC-5, AC-10).
- `mindspec panel submit p --slot R1 --round 1 --file v.json` with a valid final APPROVE, then `mindspec panel verify p` shows `R1 final` (AC-3).
- Write `R2-round-1.json` by hand, then `mindspec panel tally p`: the panel blocks, and verify shows `R2 unsubmitted` (AC-4).
- With a fake process runner configured in `.mindspec/config.local.yaml`: `mindspec panel launch p R2; echo $?` prints `0`, and `mindspec panel verify p; echo $?` prints `0` (AC-9, AC-10).
- `mindspec rules --scope reviewer` lists `panel.submit` and `review.basis` (AC-12).
- `mindspec validate spec 131-panel-execution-in-binary` reports 0 errors.

## Open Questions

- [x] **OQ-1 (harness slots)** — RESOLVED. The binary cannot start a harness-native agent. A `harness` runner gives the slot to the skill's dispatch step. The binary still renders the prompt and validates the verdict through `submit`.
- [x] **OQ-2 (where the verdict comes from in a sandboxed CLI)** — RESOLVED. A process reviewer never runs `mindspec`. Launch reads the verdict from stdout markers or `verdict.json` and submits it from outside the sandbox.
- [x] **OQ-3 (checkout type)** — RESOLVED. Slot trees are `git clone --shared` clones. The clone's git directory is inside the slot directory, so a cwd-scoped sandbox can run git there. Shared objects keep a 6-slot panel cheap. A clone does not appear in `git worktree list`.
- [x] **OQ-4 (quota detection and ZFC)** — RESOLVED. Detection uses the exit code and configured literal strings in the stderr tail only. A runner with no `quota_signals` never reports `quota_wall`.
- [x] **OQ-5 (v2 default)** — RESOLVED (operator). v2 is the default for every new round, with no opt-out (R3.1). Rounds created before this change keep spec 130's rules through the pinned `verdict_schema` field (R3.6).
- [x] **OQ-6 (replacing a final verdict)** — RESOLVED (operator, revised). No verdict is frozen. A later submission for the same slot and round replaces the earlier one. The ledger keeps every record, and the gate reads the latest (R2.4). Spec 130's text binding decides each discharge (R3.5).
- [x] **OQ-7 (families and harness binding)** — RESOLVED (operator). No family rule and no hard-coded family, in Go or in shipped runner data. Zero config runs every slot on the orchestrating harness. A one-family panel is valid. Diversity is a suggestion only (R4.12). No family minimum and no harness binding.

## Approval

- **Status**: DRAFT
- **Approved By**: -
- **Approval Date**: -
- **Notes**: -
