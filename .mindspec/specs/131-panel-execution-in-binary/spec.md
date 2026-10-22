---
approved_at: "2026-10-21T13:10:40Z"
approved_by: user
status: Approved
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
- **Root discovery stops at a clone.** `workspace.FindRoot` (`internal/workspace/workspace.go:33`) maps a linked worktree to its main root, but it takes any directory with a real `.git` as the root. A command run inside a slot clone (R6.3) would act on the clone.

This spec changes six spec-130 rules. (1) R2.1 "today's substitution applies unchanged" is replaced: substitution happens only when a runner configures `on_quota` (R4.9). (2) R2.1's zero-config fill uses the runner table (R4.4). (3) R5.3's pinned-field list gains `verdict_schema` (R3.1). (4) R2.4's carried roster gains `family` (R4.8). (5) R5.1's key semantics: the `verdict` key holds a per-slot sequence number, so two submissions never share a key (R2.5). (6) R2.3: when no reviewer list resolves, `--reviewers N` goes round-robin over the merged runner order (R4.4), not over the two-name literal in `config.go:878`.

User documents that describe the removed behavior: `project-docs/user/guides/review-panels.md` (lines 24–26 "two model families", 61–65 `substitution: claude_sub_on_quota`), `README.md` (lines 26, 37, 73 "two model families") and `plugins/mindspec/README.md` (lines 3, 48 "3 Claude + 3 Codex", 135 "`codex` CLI on PATH", 140 quota substitution). R13.7 changes them.

Dependency: #252 (skills cite an ADR) is fixed by a separate surface-leak change. R12 rewrites the same skill files. R12 lands after that change, or includes its edits.

## Impacted Domains

- **workflow**:
  - `internal/panel/**`: v2 schema and validator, submission binding in the gate, three ledger kinds, BRIEF regions, slot prompt, slot status.
  - `cmd/mindspec/**`: new `panel submit|schema|checkout|launch|recover|prompt`, `rules`, `config resolve`; `panel verify` exit code and `--json`; `doctor` checks; the `config show` known-model warning goes.
  - `internal/instruct/**`: the `## Rules` section.
  - `internal/doctor/**`: launcher, tool-suggestion, family, staged-marker, ignore-line and checkout checks; the ownership and links walks skip the isolation root.
  - `internal/setup/**`, `plugins/mindspec/skills/**`, `.claude/skills/**`, `plugins/mindspec/workflows/ms-panel.js`, `.claude/workflows/ms-panel.js`, `plugins/mindspec/scripts/`: skill and workflow slimming; historical snapshots; pin tests.
- **core**: `internal/config/**` (`panel.runners`, `panel.lenses`, `panel.gates.<g>.lenses|acceptance`, `panel.isolation_root`, `.mindspec/config.local.yaml`, retired `panel.substitution`, deleted `knownModels`, the tool-suggestion data file); `internal/workspace/**` (slot-directory root discovery, R6.9); a new `internal/rules` package with the embedded rule table; `internal/bootstrap/**` (`.gitignore` lines); `cmd/mindspec/migrate.go` (the walk skips the isolation root).
- **execution**: `internal/executor/**` (slot clone, `remote remove`, `check-ignore`, `cat-file -e`).

Repository config: this repository's `.mindspec/config.yaml` gains `panel.runners` (R4.13).

Documentation surfaces: ADR-0037, ADR-0040, ADR-0043, ADR-0044 and ADR-0045 amendments (R13); `.mindspec/domains/workflow/interfaces.md` § Panel Artifact Schema; `CHANGELOG.md`; `project-docs/user/guides/review-panels.md`; `README.md` (panel paragraphs only); `plugins/mindspec/README.md`.

## ADR Touchpoints

- [ADR-0045](../../adr/ADR-0045-binary-owns-sequence-and-rules.md) — AMENDED: step 1 (rule table) and the step-3 BRIEF and slot-prompt templates ship. Step 5 starts: `instruct` renders the rule table. The staged-rule marker form changes (R10.5).
- [ADR-0044](../../adr/ADR-0044-panel-review-conduct.md) — AMENDED: each Decision moves to a rule row (R10.2). Decision 4 changes: scratch goes in the slot or fixer directory, not `/tmp`.
- [ADR-0037](../../adr/ADR-0037-panel-gate-enforced-contract.md) — AMENDED: on a v2 round, the gate counts each slot's latest submission (R3). §8 applies: submit binds bytes to a ledger record and does not authenticate the caller.
- [ADR-0043](../../adr/ADR-0043-panel-disposition-telemetry-store.md) — AMENDED: three new ledger kinds (`verdict`, `launch`, `substitution`); the `verdict` key holds a sequence number.
- [ADR-0040](../../adr/ADR-0040-orchestration-layering-ratchet.md) — AMENDED: launchers are config data (`panel.runners`). The `runner:` key keeps its meaning.
- [ADR-0036](../../adr/ADR-0036-ownership-discovery.md) — APPLIED (ZFC): quota detection matches only configured literal strings and the exit code. No code judges reviewer output. No code names a tool family.
- [ADR-0030](../../adr/ADR-0030-executor-boundary.md) — APPLIED: clone, remote, `check-ignore` and `cat-file` calls go through the Executor. `internal/panel` stays free of git and `os/exec`.
- [ADR-0035](../../adr/ADR-0035-agent-error-contract.md) — APPLIED: every new refusal ends with one `recovery:` line.

## Requirements

**Terms.** Each term has this meaning only. Spec 130's terms (slot, roster, finding, ledger, legacy panel) apply unchanged.

- *v2 round*: a round whose `round` ledger event records `verdict_schema: 2`.
- *Runner*: one `panel.runners` entry. It tells the binary how to start a reviewer of one family. A family is a free label.
- *Slot directory*: the per-slot directory that R6 creates.
- *Submission*: a `verdict` ledger record that `panel submit` (or launch or recover) wrote. Its `seq` is its position among the slot's submissions in the round, from 1.
- *Latest submission*: the submission with the highest `seq` for a slot and round.
- *Attempt*: one `started` (process) or `dispatched` (harness) `launch` event. Its number `k` counts all attempts of the slot in the round, from 1.
- *Effective runner*: the runner of the slot's family, or of the `to_family` of the slot's `substitution` event in the round. Its *effective kind* is its `kind`.
- *Harness redispatch*: the adapter action `panel checkout --slot S --dispatch` (with `--replace` after a final that is not `VOID`), then `panel prompt --slot S`, then one dispatch of that prompt with the harness's agent tool. `--dispatch` appends a new `dispatched` event (R6.5).
- *Closed attempt*: an attempt whose latest outcome event is a `submitted` outcome that names the `seq` of the slot's latest submission.
- *Slot lock*: the lock file `<isolation_root>/.locks/<spec or adhoc>/<slug>/r<N>/<slot>.lock`. On a host with POSIX advisory locks, no verb deletes a file under `.locks/`. R7.9 gives the rule for other hosts.
- *Model-safe value*: a non-empty string that does not start with `-` and holds no whitespace and no control byte.

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
2. Short form: `mindspec panel submit --file F`, run in a slot directory or under it, or with `F` an absolute path inside a slot directory. Submit takes the spec, slug, round and slot from the slot directory of the cwd, else of `F`, with the R6.9 checks. A relative `F` gives no slot. When the cwd and `F` are in different slot directories, submit refuses and names both. Attempt binding is in R2.9. Explicit flags that disagree with the path refuse.
3. Submit refuses each of these, and writes nothing:
   - a round that is not a v2 round (recovery: `mindspec panel create <slug> --round <N+1>`);
   - a file that fails R1 (the message names the JSON pointer of the first error);
   - a `slot` or `round` field that differs from the target slot and round;
   - a slot that is not on the roster, a round other than `panel.json`'s `round`, or an abandoned panel;
   - a `reviewed_sha` other than the current `reviewed_head_sha`, except for a same-bytes rerun (R2.5);
   - a `model` other than the slot's effective model (R4.6), or a `model` that is not a model-safe value. The slot prompt states the effective model;
   - a CLI submission when the slot's effective runner is a process runner (recovery: `mindspec panel launch` or `mindspec panel recover`);
   - a panel and round that a landed binding names (the bead's `mindspec_landed_review_panel` metadata). The recovery is `mindspec panel create <slug> --round <N+1>`.
4. A later submission for the same slot and round replaces the earlier one, at any status. The ledger keeps every `verdict` record. The gate reads only the latest submission.
5. Submit holds the panel ledger lock from the R2.3 checks to the end of the write. Under that lock:
   - when the latest submission has the same `file_sha256`, submit appends no `verdict` record and rewrites the file. This repairs a crash between append and rename. It applies only against the latest submission;
   - otherwise submit appends one `verdict` record with `seq` = 1 + the highest `seq` of the slot and round, then writes the exact bytes to `<slot>-round-<N>.json` by temp file and rename;
   - in both cases, a CLI submission appends a `submitted` outcome (R7.4) for the slot's latest `dispatched` attempt when that attempt has no outcome event. The outcome names the `seq` that the submission selected. Launch and recover append their own outcome event.

   | `record` | Fields (plus `spec`, `panel`) | Key |
   |:---------|:-------|:----|
   | `verdict` | slot, round, seq, status, verdict, family, model, runner_kind, runner_source, via (`direct`, `launch` or `recover`), attempt, reason, file_sha256, created_at | slot, round, seq |

   Submit resolves `runner_kind` (the effective kind) and `runner_source` (`config.yaml`, `config.local.yaml` or `default`) under the ledger lock, from the same config load as the R2.3 process-runner check.

6. The CLI always writes `via: direct`. No flag and no environment variable of `panel submit` sets `via`, `attempt` or `seq`. Launch and recover call the submit path in-process.
7. Submit prints the slot status (R8) and spec 130's rule line.
8. `panel disposition append` refuses the kinds `verdict`, `launch` and `substitution`. `panel disposition validate|check|query` accept them.
9. Attempt binding. This applies to every CLI submit, long form and short form, whose `F` is under `attempt-<j>/` of the target slot. Launch and recover do not apply it; they bind the attempt by R7.4 and R7.8. Let `k` be the slot's highest attempt with a `launch` event (`dispatched` for a harness slot, `started` for a process slot). Submit refuses with `attempt <j> was superseded by <k>` when `j` < `k`; with `attempt <j> was not dispatched` when `j` > `k` or the slot has no attempt; and with `attempt <j> ended with <outcome>` when attempt `j` has an outcome other than `submitted`, for example `launch_failed` (recovery: `mindspec panel checkout --slot S --dispatch --replace`). No current flow writes such an outcome for the latest harness attempt; this refusal is defence in depth. A `submitted` outcome never refuses, so a draft then a final, and a same-bytes rerun, work from the same attempt. (A process slot also gets the R2.3 refusal of every CLI submit.)

### R3 — The gate on a v2 round

1. `create` writes `verdict_schema: 2` to `panel.json` and to the `round` event of every new round. No config key or flag changes it. This spec adds `verdict_schema` to spec 130 R5.3's pinned-field list.
2. The gate takes v2-ness from the latest `round` event only, never from `panel.json`. When an earlier `round` event of the panel has `verdict_schema: 2` and the latest one does not, the gate blocks with a registration error.
3. On a v2 round, a roster slot is in only when its latest submission is `final`, its verdict is not `VOID`, the sha256 of `<slot>-round-<N>.json` equals that submission's `file_sha256`, and either no `launch` event is later than it in the ledger, or the highest-numbered attempt with a `launch` event later than it is closed (R8.2). A later attempt supersedes earlier open or failed attempts. So a replacement in flight, or one that failed, keeps the slot out, and a later attempt that returns the same final bytes puts it in again.
4. A verdict file with no matching latest submission counts for nothing. Spec 130 R2.5 (an off-roster REJECT, INVALID or `hard_block` blocks) applies unchanged.
5. Spec 130's pass rule, discharge rules and coverage manifest use only the slots that are in. A discharge of `ccr[i]` applies while its `finding_sha256` equals the sha256 of `ccr[i]`'s text in the latest submission (spec 130 R7.3, unchanged). A slot-level discharge applies while the slot hash is unchanged (spec 130 R7.4).
6. A round whose `round` event has no `verdict_schema` keeps spec 130's rules. Such a round only exists from before this change.

### R4 — Runners and slot resolution

1. `panel.runners` is an ordered list. Each runner has `family` (unique, non-empty) and `kind`.
   - `kind: harness`: the orchestrating harness dispatches the slot with its own agent tool. The runner can have only `timeout_minutes` (no default; absent means no limit). Load refuses `on_quota` on a harness runner, because no path records a harness quota wall.
   - `kind: process`: the binary starts the reviewer. The runner needs `argv`, `prompt` (`stdin`, `file` or `arg`) and `output` (`stdout` or `file`). It can have `quota_signals` (literal strings), `on_quota` (another family), `probe` (an argv) and `timeout_minutes` (default 60).
2. `argv[0]` is a literal with no placeholder. Each other element is a literal or exactly one placeholder: `{prompt}`, `{prompt_file}`, `{model}`, `{slot_dir}` or `{tree}`. A placeholder expands to one whole element. `prompt: file` needs `{prompt_file}`, and `prompt: arg` needs `{prompt}`.
3. Load refuses any other placeholder, a placeholder inside a longer element, a duplicate family, and an `on_quota` that names the runner's own family or an undeclared family.
4. The shipped default is one runner, `{family: harness, kind: harness}`. The binary ships no process runner and no runner named after a tool. A `panel.runners` list in `config.yaml` merges with the default by family: an entry with family `harness` replaces the default, and every other entry is appended. Spec 130 R2.1's zero-config fill goes round-robin over the merged runner order. With zero config, every slot has family `harness`.
5. `TestNoFamilyLiterals` reads the tool names from the R4.12 data file. It checks every Go string literal in non-test files and every shipped data value in `internal/config`, `internal/panel` and `cmd/mindspec/panel*.go`. No match is allowed, with these exemptions:
   - by path: the R4.12 data file;
   - by value: the `runner:` adapter values `claude-code-skills` and `claude-code-workflow`. They name install targets for the skills, not reviewer families;
   - by value: the frozen historical keys and values of the rename table in `internal/panel/disposition_migrate.go` (for example the key `G-codex` and the value `G1-codex`). They name panel files that already exist.

   `knownModels`, `KnownModels()` and the `config show` known-model warning are deleted. They drive only an advisory warning on an unlisted model id.
6. Each roster entry gains `family`. `create` takes it from the reviewer entry. A slot's model is the reviewer entry's `model`, else `models["reviewer.<family>"]`, else the family string. The model of a substitute is `models["reviewer.<to_family>"]`, else the to-family string. It never uses the reviewer entry's `model`. The substitution record stores it. The *effective model* is the model of the effective runner. Load and `create` refuse a model value that is not model-safe, from any source.
7. A reviewer entry with no `family` gets family `harness`. Its `model` stays its model: the adapter passes it to its agent tool (R9.5). `config resolve` and `doctor` print one Warn for each such entry: `<entry> runs on the harness runner; add a family and a runner to run it elsewhere`. When a reviewer entry names a family with no runner, `create` refuses, `config resolve` refuses, and `doctor` reports an Error. Each names the reviewer entry, with the recovery `add a runner for the family to panel.runners`.
8. The first v2 round of a panel copies the prior roster (spec 130 R2.4). For a carried slot with no `family`, `create` sets `family` to the slot's model only when a runner has exactly that family. Otherwise `create` refuses. The recovery is `mindspec panel create <slug> --round <N+1> --reviewers <n>`, which takes a fresh roster from config. `create` never changes an earlier round record.
9. `panel.substitution` is retired. Load refuses a `substitution` key under `panel`, with the recovery `set on_quota on the runner in panel.runners`. Substitution happens only when a runner has `on_quota`.
10. `.mindspec/config.local.yaml` is optional and machine-local. It can set only `panel.runners` and `models`. It merges after `config.yaml` with the R4.4 rule: a local runner replaces the runner of the same family, or is appended. Every command refuses when the file is over 65536 bytes, does not parse, has another key, or cannot be read for a reason other than absence. `mindspec init` adds the file to `.gitignore`.
11. `mindspec config resolve --gate G [--json]` prints each slot's `slot family model lens kind`. Each value shows its source: `config.yaml`, `config.local.yaml` or `default`. It is a preview of a new panel. It is not the roster of an existing panel (R8.5).
12. `internal/config/review_tools.yaml` lists known review tools as `{name, command}` pairs. When a listed `command` is on `PATH` and no runner uses it as `argv[0]`, `config resolve` and `doctor` print one suggestion line. The line asks the operator to configure a process runner for review diversity. It is advice only. Nothing requires a second family.
13. This repository's `.mindspec/config.yaml` gains `panel.runners`, in the same bead as R4.7. It is repository config, not shipped data:

    ```yaml
    runners:
      - {family: claude, kind: harness}
      - family: codex
        kind: process
        argv: [codex, exec, --skip-git-repo-check, -s, workspace-write, -m, "{model}", -C, "{slot_dir}", "-"]
        prompt: stdin
        output: stdout
        quota_signals: ["ERROR: You've hit your usage limit"]
        on_quota: claude
    ```

    The quota string is the literal that `ms-panel-run` uses today. Panels in this repository run with the installed release binary, not a spec-branch build. So this spec's own bead and final panels run on the released binary until the spec merges. Config load does not refuse unknown keys (`yaml.Unmarshal` in `config.go`), so a released binary ignores `panel.runners`, and the block can also land on `main` before the spec merges.

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
2. `panel checkout` and `panel launch` refuse when a component of the resolved root is a symlink, and when `git check-ignore` does not ignore the root (recovery: `mindspec init`). `mindspec init` adds the effective isolation root (configured or default) to `.gitignore`, on a new or an existing install.
3. A slot directory is `<isolation_root>/<spec or adhoc>/<slug>/r<N>/<slot>/`. It holds `slot.json`, `tree/` and one `attempt-<k>/` directory for each attempt. A process attempt's prompt is `attempt-<k>/prompt.md`; `panel prompt` prints a harness prompt and writes no file. `slot.json` records the spec, slug, round, slot and the absolute main root. `tree/` is `git clone --shared --no-checkout` of the repository, then a detached checkout of the reviewed SHA, then removal of the `origin` remote. The clone's git directory is inside `tree/`, so a reviewer CLI that may write only under its cwd can run git in the tree.
4. `mindspec panel checkout <slug> (--slot S | --all) [--spec ID]` creates slot directories and prints each absolute path. A rerun reuses a directory whose tree `HEAD` is the reviewed SHA. It refuses an abandoned panel, an off-roster slot, a slot whose lock is held (R7.2), and a tree at another SHA (recovery: `mindspec panel checkout <slug> --prune --slot S`). Plain checkout (`--slot`, `--all`, or a rerun) writes no ledger record.
   - Before it reuses a tree, checkout `lstat`s `tree/` and `tree/.git`. Each must be a real directory, not a symlink and not a file. Checkout also refuses when `tree/.git/commondir` exists, when the tree's local git config sets `core.worktree` or `core.bare`, or when `tree/.git/objects/info/alternates` differs from the one line that the clone wrote. Each refusal has the same `--prune --slot` recovery.
   - Every git call of the binary in an existing tree runs with `-c core.fsmonitor=false -c core.hooksPath=/dev/null` and with `GIT_CONFIG_NOSYSTEM=1`.
   - Every git call of the binary for a slot (clone, checkout, remote, check) and every reviewer process run with git's repository-locating variables removed from the environment: each name that `git rev-parse --local-env-vars` prints (for example `GIT_DIR`, `GIT_WORK_TREE`, `GIT_INDEX_FILE`, `GIT_OBJECT_DIRECTORY`, `GIT_ALTERNATE_OBJECT_DIRECTORIES`, `GIT_COMMON_DIR`, `GIT_NAMESPACE`, `GIT_CONFIG*`).
5. `panel checkout <slug> --slot S --dispatch [--replace]` does the R6.4 checkout and creates `attempt-<k>/` with an exclusive `mkdir` (`k` as in R7.2). It appends `launch_failed` for each earlier `dispatched` attempt of the slot with no outcome event, then a `launch` event with status `dispatched` and attempt `k`. It refuses a slot whose effective runner is a process runner (recovery: `mindspec panel launch`), and a slot whose latest submission is final and not `VOID`, unless `--replace` is set. `--dispatch` needs `--slot`. It refuses a panel and round that a landed binding names (R2.3), before any ledger append, with the same recovery.
6. `panel checkout <slug> --fixer` creates `<…>/r<N>/fixer/` with no tree. It is scratch only. The fixer works in the target's own worktree: the bead worktree for a bead panel, the spec worktree for a final review.
7. `panel checkout <slug> --prune [--slot S]` removes the checkouts of the panel, or of one slot. `create` for round N+1 removes the panel's checkouts of earlier rounds. `abandon` removes all. Removal deletes the directory, dirty or not, under the verified root, and does not follow symlinks. Before it changes anything, each of these three verbs takes the slot lock (Terms) of every affected slot, with the R7.3 bounded retry, in sorted slot-name order, and before the ledger lock. Then it runs the R7.3 orphan check on each affected slot. When a slot lock is held, or the orphan check refuses, the verb removes nothing and names each held slot or attempt directory (recovery for a held slot lock: `mindspec panel verify`). On a host with POSIX advisory locks, removal never deletes a slot lock file, so every verb that locks a slot locks the same file.
8. These walks skip the resolved isolation root: panel registration lookup, the `doctor` ownership walk, the `doctor` links walk, and the `migrate` walks. No ledger record contains an isolation-root path.
9. Root discovery. When the cwd is in a slot directory or under it, every `mindspec panel` verb and `mindspec rules` take the main root from the slot directory's `slot.json`. The slot directory is the nearest ancestor of the cwd that holds both `slot.json` and `tree/`. The verb refuses unless the slot directory path is exactly `<main root>/<isolation_root>/<spec>/<slug>/r<N>/<slot>`, with the isolation root that the main root's config resolves, and unless `slot.json` names the same spec, slug, round and slot. These verbs never use the clone as the root. Every other `mindspec` command run under `tree/` acts on the clone; a reviewer can use this to probe the reviewed code.

### R7 — `panel launch` and `panel recover`

1. `mindspec panel launch <slug> <slot> [--spec ID] [--replace]` starts the slot's effective process runner and waits. It refuses: a slot whose effective runner is a harness runner (recovery: `mindspec panel prompt`); an abandoned panel; a round that is not a v2 round; a slot whose latest submission is final with a verdict other than `VOID`, unless `--replace` is set (a final `VOID` needs no `--replace`); a slot whose lock another process holds (recovery: `mindspec panel verify`); a panel and round that a landed binding names (R2.3), before any ledger append.
2. Launch takes the slot lock (Terms) and holds it until it exits. Lock order: the slot lock first, then the panel ledger lock for each append. No verb takes them in the other order. Under the slot lock, launch ensures the R6 checkout at the reviewed SHA, with the R6.2 and R6.4 refusals, then runs the R7.3 orphan check. Each start is a new attempt `k`, in `attempt-<k>/` in the slot directory. `k` is 1 + the larger of the highest ledger attempt and the highest attempt directory on disk. Launch never reads files of an earlier attempt.
3. Each attempt runs under a *supervisor*: the `mindspec` binary in an internal mode, started by launch as the leader of a new process group.
   - Launch creates `attempt-<k>/` with an exclusive `mkdir`, writes `attempt-<k>/prompt.md` with a new random nonce (R9.4), and takes an exclusive lock on `attempt-<k>/run.lock`. It passes that one descriptor and a start pipe to the supervisor only.
   - Launch takes the ledger lock, starts the supervisor, appends a `launch` event with status `started`, `attempt`, `pgid` (the supervisor's pid), `runner_kind`, `runner_source`, `argv_sha256` (the sha256 of the argv template before expansion) and `nonce_sha256`, releases the ledger lock, and then writes `start` to the start pipe.
   - The supervisor sets close-on-exec on `run.lock` and writes its pid to `attempt-<k>/pgid` by temp file and rename. Then it reads the start pipe. On end-of-file before `start`, it exits and starts no `argv`. After `start`, it starts `argv` as its child in the same group: with no shell, with the slot directory as cwd, with stdout to `attempt-<k>/stdout.log` and stderr to `attempt-<k>/stderr.log`, and with every `MINDSPEC_` variable and every R6.4 git variable removed from the environment.
   - The child inherits only stdin, stdout and stderr. For `prompt: stdin`, the child's stdin is `attempt-<k>/prompt.md`; else it is the null device. The slot lock, the ledger lock and `run.lock` are close-on-exec in the child.
   - The supervisor enforces `timeout_minutes`, also when launch is dead. When the child exits or times out, the supervisor writes `attempt-<k>/exit.json` (exit code and timeout flag) by temp file and rename, then sends SIGKILL to its own group, which ends it too. It catches SIGINT, SIGTERM and SIGHUP, and on each of them and on every exit path it sends SIGKILL to its own group. On Linux, the plan may also set a parent-death signal on the child; this is allowed, not required.

   `run.lock` is held from before the supervisor starts until both launch and the supervisor end. The orphan check runs only under the slot lock, so launch is dead then, and a held `run.lock` means a live supervisor. Its pid is the ledger `pgid`, which no other process can have while it lives.

   *Orphan check*, for each attempt of the slot with no outcome event (an attempt directory with no ledger event, or a `started` event with no outcome). With a `started` event, the target group is that event's `pgid`, and the check does not read the `pgid` file, because the reviewer can write it. Only for a directory with no ledger event is the target the `pgid` file. `run.lock` counts as held only after the bounded retry:
   - `run.lock` free: no signal;
   - `run.lock` held and the target is a decimal value greater than 1 that is not the caller's own group: SIGKILL to that group, then a wait of up to 5 seconds for `run.lock` to be free;
   - `run.lock` still held, or the target absent, not decimal, 1 or less, or the caller's own group: refuse, naming the attempt directory;
   - otherwise, a `started` event with no outcome gets a `launch_failed` outcome. It records `exit_code` from `exit.json` when that file exists, and the sha256 of each of `stdout.log`, `stderr.log` and `verdict.json` that exists, so `panel recover` can use the attempt.

   An attempt with an outcome event is not tested.

   *Lock tests.* A verify lock test takes the lock without waiting and releases it at once. Every verb that takes the slot lock or `run.lock` to hold it, and the orphan check, retry for up to 1 second before they count the lock as held. So a verify probe never makes a verb refuse or send a signal.
4. When the supervisor ends, launch reads `attempt-<k>/exit.json` (absent: `launch_failed`) and appends one outcome event with `stdout_sha256`, `stderr_sha256` and, when `attempt-<k>/verdict.json` exists, `verdict_sha256`:
   - `submitted`: exit 0 before the timeout, and R2 accepts the verdict. The event names the `seq` that R2.5 selected, also when no new `verdict` record was appended. With `output: stdout`, the verdict is the one block in `stdout.log` between `MINDSPEC-VERDICT-BEGIN <nonce>` and `MINDSPEC-VERDICT-END <nonce>`. With `output: file`, it is `attempt-<k>/verdict.json`;
   - `quota_wall`: a non-zero exit or a timeout, and the last 64 KiB of `stderr.log` contains a configured quota signal;
   - `launch_failed`: any other non-zero exit or timeout;
   - `malformed`: exit 0 and no block, more than one block, a block with another nonce, or an R2 refusal.
5. On the first `malformed`, launch starts one more attempt. Its prompt gets the refusal message through `termsafe`, with the JSON pointer cut to 200 bytes. A second `malformed` ends the launch.
6. On `quota_wall`, when the effective runner has `on_quota` and the slot has no `substitution` event in the round, launch appends a `substitution` event. A process substitute is started once as a new attempt. For a harness substitute, launch prints the `panel checkout --slot --dispatch` and `panel prompt --slot` commands and exits non-zero. A second `quota_wall` in the round is terminal.

   | `record` | Fields (plus `spec`, `panel`) | Key |
   |:---------|:-------|:----|
   | `launch` | slot, round, attempt, family, model, status, seq, exit_code, pgid, runner_kind, runner_source, argv_sha256, nonce_sha256, stdout_sha256, stderr_sha256, verdict_sha256, created_at | slot, round, attempt, status |
   | `substitution` | slot, round, from_family, to_family, model, created_at | slot, round |

7. Launch exits 0 only when it recorded `submitted` for a `final` verdict. Otherwise it exits non-zero and prints the slot status.
8. `mindspec panel recover <slug> <slot> --reason R [--spec ID]` submits a verdict from the latest attempt, with any exit code. The latest attempt is the highest `attempt` among the slot's `started` events in the round, not a directory on disk. Recover starts no process. It refuses when that attempt has no outcome event (recovery: `mindspec panel launch`), and when a log or verdict file of the attempt does not match the digest in the outcome event.
   - `output: stdout`: recover extracts the one block from `stdout.log` (R7.4). The block nonce must hash to the attempt's `nonce_sha256`.
   - `output: file`: recover reads `attempt-<k>/verdict.json`. The binding is the attempt path and `verdict_sha256`. There is no nonce.

   The submission has `via: recover` and the reason. Recover prints the attempt's exit code and the selected `seq`.
9. Hosts without POSIX process groups (Windows). `panel launch` and `panel recover` refuse a process slot with `process runners need a Unix host` (recovery: use a harness runner). Harness slots, `submit`, `verify`, `tally` and `checkout` work. The slot lock is a lock file made by exclusive create, as the binary already does on such hosts. The file holds the holder's pid, and the holder deletes it on release. A lock test reads the file and does not create it. A file whose pid is not a live process is stale and counts as free. An empty or unparseable lock file counts as held, never as stale. A verb that takes the lock removes a stale file first: it renames the file to a unique name and reads the pid of the renamed file again. It deletes the file only when that pid is the stale pid it read; else it puts the file back without replacing a file: it links the moved file to the lock path, then removes the moved name, and counts the lock as held. When the link fails because a lock file exists again, it deletes the moved file only when its pid is dead; else it refuses and names both files. No verb deletes a file whose pid is live. When a lock file stays after its holder is gone (for example, its pid is in use again), the verb names the file (recovery: delete `<path>` after you make sure that no mindspec process uses the slot). There is no `run.lock` and no orphan check, so a `started` event is `running` only while the slot lock is held. The supervisor, process-group and signal code is in `_unix.go` files, so every release target builds.

### R8 — `panel verify` slot status

1. `panel verify` gives each roster slot one status from this closed set: `final`, `damaged`, `void`, `draft`, `running`, `quota_wall`, `launch_failed`, `malformed`, `unsubmitted`, `missing`.
2. On a v2 round, the status comes from the first rule that applies:
   - the latest submission is final and not `VOID`, and either no `launch` event is later than it in the ledger, or the highest-numbered attempt with a `launch` event later than it is closed: `final` when the file matches its `file_sha256`, else `damaged` (recovery: `mindspec panel submit` with the original bytes, or `mindspec panel recover`);
   - a verdict file exists whose sha256 matches no latest submission: `unsubmitted`;
   - the latest event among the slot's submissions and `launch` events decides:
     - a submission, or a `submitted` outcome (through the submission its `seq` names): `void` for `VOID`, `draft` for `draft`. This bullet never sees a closed final, because the first rule takes it;
     - `started`: `running` while the slot lock or the attempt's `run.lock` is held, else `launch_failed`;
     - `dispatched`: `launch_failed` when the harness runner has `timeout_minutes` and the event is older than that limit, else `running`;
     - `quota_wall`, `launch_failed` and `malformed`: their own names;
   - with no such event: `missing`.

   So a replacement in flight (a `started` event under a held lock, or a `dispatched` event, after the latest final submission) gives `running`. A replacement that ends with no submission gives its own outcome status (for example `launch_failed`), never the earlier `final`. The earlier final counts again only when a later rule run gives `final`, for example after a submission.
3. Annotations, advice only, never a gate input. Each comes from ledger records, except where a bullet names the current config:
   - `substituted <from>-><to>` for a slot with a substitution;
   - `local runner` when the latest submission's `runner_source` is `config.local.yaml`;
   - `direct on process family` when a submission of the round has `via: direct` and the slot's family has a process runner in the current config;
   - `replaced <old>-><new>` when any earlier final submission of the slot in the round has `REQUEST_CHANGES`, `REJECT` or `hard_block`, and the latest submission has none of them. `<old>` is the verdict of the most recent such submission. `tally` prints one line for each such slot.
4. On another round, a parseable verdict file gives `final`, an unparseable file gives `malformed`, and no file gives `missing`.
5. `--json` prints, for each slot: slot, family, model, lens, effective kind, status, annotations, and `handoff_pending`. `handoff_pending` is true when the slot has a `substitution` event to a family with a harness runner and no `dispatched` event after it. Adapters take the roster of an existing panel, and the column of the R12.1 table, from this output.
6. Exit codes: 0 when every slot is `final`; 1 when a slot is `running` or `missing`, is `draft` with a harness effective runner, or is `quota_wall` with `handoff_pending`; 2 in every other case. Verify prints the rule line and the PASS/BLOCK preview, and writes nothing. It tests the slot lock and `run.lock` by the R7.3 lock test, and never waits.
7. Every reviewer-supplied string that submit, verify, prompt, launch or recover prints goes through `termsafe`.

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
   - the output instruction for the effective runner: the nonce marker lines; for `output: file`, the absolute `<slot_dir>/attempt-<k>/verdict.json` of the attempt that launch starts; or, for a harness runner, the command `mindspec panel submit --file <slot_dir>/attempt-<k>/verdict.json` with the absolute slot directory and the attempt of the latest `dispatched` event;
   - from round 2, the slot's own prior-round items, each to grade ADDRESSED, PARTIAL, MISSED or NEW_ISSUE. On a v2 prior round, the items are the `concrete_changes_required` texts of the slot's latest submission. On a prior round with no `verdict_schema`, they are the parsed `concrete_changes_required` of `<slot>-round-<N-1>.json`; an unparseable or absent file gives `no prior-round items`;
   - the instruction to write scratch only in the slot directory.
5. The adapter passes the slot's effective model to its agent tool. When the tool cannot run that model, the adapter reports the slot and does not run it on another model. When the effective model equals the family string (the zero-config case), the adapter dispatches with the harness's default model, and the verdict `model` is the slot model, as a label.

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
2. `doctor` runs a runner's `probe` only when the runner comes from `config.local.yaml` and `git ls-files` does not track that file, or under `doctor --probe`. A probe must exit 0 within 10 seconds, else Error. A probe that plain `doctor` does not run gets one Info line that says so.
3. `doctor` prints the R4.12 tool suggestion as Info, the R4.7 family Error, and the R4.7 model-only Warn.
4. When config does not load (for example, the retired `panel.substitution`, or an R4.10 local-file refusal), `doctor` reports the load error as one Error with its recovery.
5. `doctor` warns when `.mindspec/config.local.yaml` is tracked, and when an existing config file or the effective isolation root is not ignored (recovery: `mindspec init`).
6. `doctor` warns about each staged-rule marker in an installed skill that has no id, an unknown id, or a past `remove-by` date. It lists the rules whose `enforced_by` is not `advisory` (Info).
7. `doctor` warns about a slot directory whose panel is missing, abandoned, or at a later round. The recovery is `mindspec panel checkout <slug> --prune`.
8. `doctor --ci` skips R11.1, R11.2, the R11.3 suggestion and R11.7.

### R12 — Skill and workflow slimming

1. Both copies of `ms-panel-run` have at most 80 lines. For each `runner:` value:
   - `claude-code-workflow`: run the `/ms-panel` workflow once with `{slug, spec, target, bead_id, round}`;
   - `claude-code-skills` and `external`: `panel create`; `panel verify --json` for the roster; act on each slot by the table below; run `panel verify --json` again after each process or agent ends; on exit 0, run `ms-panel-tally`.

   The column is the slot's effective kind in the latest `verify --json` output. "Once" counts per slot and round: after one retry action, the next non-final, non-running status is reported, not retried. *Redispatch* is the harness redispatch (Terms).

   | Status | Process slot | Harness slot |
   |:-------|:-------------|:-------------|
   | `missing` | `panel launch` in the background | redispatch |
   | `running` | wait | wait while the agent runs; when the agent has returned and the slot is still `running`, redispatch once, then report |
   | `draft`, `unsubmitted`, `void` | `panel launch` once more, then report | redispatch once, then report |
   | `launch_failed`, `malformed` | `panel launch` once more, then report | redispatch once, then report |
   | `quota_wall` | report (a harness substitute shows the harness column) | with `handoff_pending`: redispatch; else report |
   | `damaged` | `panel recover`, then report | report |

2. Both copies of `ms-panel-tally` have at most 80 lines. Each current section is kept short or replaced:

   | Section | Fate |
   |:--------|:-----|
   | Steps 1 (tally), Then | kept |
   | Workflow-path note | cut; the workflow returns verify and tally output |
   | Step 2 consolidate | kept; contested re-verify becomes `review.reproduce` |
   | Step 3 dispositions | kept; the JSON goes on stdin (`--data -`) from a quoted heredoc, with no temp file |
   | Step 4 report | kept; family counts and `replaced` lines come from `panel verify --json` |
   | Artifact gates | becomes `panel.artifact-gate` |
   | After a halt | kept as one line per verb: `instruct --panel-state`, `create --round`, `rebind`, `abandon`, `refute` |
   | Escape hatch | kept, two lines |
   | Anti-patterns | become `review.no-vote`; the confidence note goes (v2 has no `confidence`) |
   | Defer a finding | kept, three lines; the verb enforces the rest |

3. No copy of `ms-panel-run`, `ms-panel-tally`, `ms-bead-fix`, `ms-spec-final-review`, `ms-bead-cycle` or `ms-bead-impl` contains `/tmp`. None contains a tool family name, except the harness's own name in a path such as `.claude/` and the R4.5 `runner:` values. These skills contain no lens table, no size ladder and no failure-detection section. They cite rule ids for conduct. `ms-bead-cycle`'s family table becomes family-neutral.
4. `ms-spec-final-review` uses `panel checkout --slot` for isolated reviewer work. `ms-bead-fix` uses `panel checkout --fixer` for scratch and dispatches the fixer in the target's own worktree (R6.6).
5. Both copies of `ms-panel.js` name no tool family. `ALLOWED_CLI` is exactly `mindspec panel create`, `mindspec panel verify`, `mindspec panel tally`, `mindspec panel launch`, `mindspec panel checkout`, `mindspec panel prompt`, `mindspec panel submit` and `mindspec panel recover`. `buildCommand` builds each one with fixed flags; `submit` takes only `--file <slot_dir>/attempt-<k>/verdict.json`, the absolute path that the slot prompt names, and uses the R2.2 short form; `checkout` for a dispatch takes `--slot S --dispatch` and, after a final, `--replace`. `VALID_MIX_FAMILIES`, the substitution flag and the parse-retry ladder go. The workflow takes the roster from `panel verify --json` after `create` and follows the R12.1 table. A harness reviewer step submits through a `buildCommand` `panel submit` call. It never writes a verdict file with the Write tool, as the reviewer steps of today's `ms-panel.js` do.
6. `plugins/mindspec/scripts/codex_verdict_extract.sh` is deleted. `panel recover` replaces it.
7. `setup`'s historical skill snapshots gain the replaced versions of every R12.3 skill, so an unmodified installed copy refreshes.

### R13 — ADR and document amendments

Each amendment is an appended `## Amendment (Spec 131)` section. It lands with the bead that ships the code it describes. A text test pins it.

1. ADR-0045: steps 1 and 3 (BRIEF and slot prompt) shipped; step 5 started (R10.6); the R10.5 marker form.
2. ADR-0044: each Decision maps to its R10.2 rule; Decision 4 now says the slot or fixer directory.
3. ADR-0037: R3 counting (latest submission, replacement, positional discharge), the pinned `verdict_schema` field, and the §8 statement for submit.
4. ADR-0043: the three new record kinds, and the `seq` in the `verdict` key.
5. ADR-0040: `panel.runners`, the merge rule, `.mindspec/config.local.yaml`, and no shipped tool runner.
6. `interfaces.md` § Panel Artifact Schema describes v2, submit and slot status. `CHANGELOG.md` names:
   - v2 on every new round with no opt-out; a verdict file with no submission no longer counts;
   - the latest submission replaces earlier ones;
   - zero config runs every slot on the harness, and no tool runner ships;
   - a reviewer family needs a runner, with this before and after:

     ```yaml
     # before: create now refuses this entry
     reviewers: [{family: codex, model: m1}]
     # after
     runners: [{family: codex, kind: process, argv: [codex, "{prompt_file}"], prompt: file, output: stdout}]
     reviewers: [{family: codex, model: m1}]
     ```

   - `panel.substitution` is retired; use `on_quota` on a runner;
   - the `panel verify` exit codes;
   - the new spec and plan default lenses;
   - the first-v2-round roster recovery;
   - a model-only reviewer entry now runs on the harness with its model, and `config resolve` and `doctor` warn about it (R4.7).
7. User documents:
   - `project-docs/user/guides/review-panels.md`: zero config is one harness family; `panel.runners`, with an `on_quota` example, replaces `panel.substitution`; reviewers use `panel submit`; the R8.1 statuses and R8.6 exit codes of `panel verify`;
   - `README.md` (panel paragraphs only) and `plugins/mindspec/README.md`: no claim that a panel has two model families or a fixed family split, and no requirement for a named reviewer CLI. The plugin README describes `on_quota` in place of the quota substitution paragraph.

## Scope

### In Scope

- Every file in Impacted Domains, and the documentation surfaces listed there.

### Out of Scope

- Every tracked legacy panel directory. No file in them changes.
- `.mindspec/domains/**` files other than `workflow/interfaces.md`.

## Non-Goals

- Authentication of a submitter. Submit binds bytes to a ledger record. Any process with write access to the repository can submit, or can append to the ledger file. ADR-0037 §8 applies.
- Process runners on Windows (R7.9).
- Three or more verbs that take one stale Windows slot lock at the same moment.
- A supervisor killed with SIGKILL (for example by the OOM killer) can leave its child running. `run.lock` is then free, and the orphan check sends no signal to that group.
- A descendant of a reviewer that calls `setsid` leaves the attempt's group, and the supervisor does not kill it. It holds no mindspec lock, so it blocks no verb.
- Protection from the code under review. The nonce stops marker lines that appear by accident in output. It does not stop code that the reviewer runs, because that code can read the slot directory.
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
  - submit writes the file and one `verdict` record with `seq: 1`; a same-bytes rerun adds no record;
  - each R2.3 refusal leaves the panel directory and ledger byte-identical; the v1-round refusal names `--round`;
  - a final APPROVE, then a final REQUEST_CHANGES, is accepted, and the ledger holds both records;
  - A, then B, then A gives three records with `seq` 1, 2 and 3; the latest digest is A's, the file holds A, and the gate counts A; an immediate rerun of the third A adds no record;
  - with a crash injected after the append and before the rename, a same-bytes rerun makes the slot `final` and adds no record;
  - two concurrent submits for one slot leave one file whose sha256 equals the latest record;
  - a CLI submit on a process slot refuses; `panel submit --via launch` is an unknown flag; a `model` other than the effective model refuses;
  - `panel disposition append` refuses each new kind;
  - a submit to a panel and round that a bead's `mindspec_landed_review_panel` names refuses, and the recovery names `--round`;
  - a local file flips family `b` to a harness runner, a direct submit for a `b` slot succeeds with `runner_source: config.local.yaml`, then the local file is deleted: verify still shows `local runner` and `direct on process family`.

  Tests: `TestPanelSubmit_LandedBindingRefused`, `TestPanelVerify_AnnotationsFromRecords`, `TestPanelSubmit_Writes`, `TestPanelSubmit_Refusals`, `TestPanelSubmit_Replace`, `TestPanelSubmit_ABA`, `TestPanelSubmit_CrashRepair`, `TestPanelSubmit_Race`, `TestPanelSubmit_NoViaFlag`, `TestDispositionAppend_RefusesV2Kinds`.
- [ ] **AC-4 (R3)**: On a v2 round with 4 roster slots:
  - 4 submitted final APPROVE files pass;
  - a hand-written file in place of a submission blocks, and verify shows `unsubmitted`;
  - a submitted file changed by one byte blocks, and verify shows `damaged`;
  - a draft blocks; a `VOID` blocks;
  - an APPROVE replaced by a REQUEST_CHANGES blocks;
  - a refuted `ccr[0]`, then a replacement with the same item text: the refutation still applies;
  - the same, with the item text changed: the finding is open again;
  - a refuted `ccr[0]` with text X, then a replacement with items [Y, X]: the gate blocks on `ccr[0]` and `ccr[1]`;
  - `panel.json` with `verdict_schema` deleted blocks; a v2 panel whose newest `round` event lacks the field blocks;
  - a fixture round with no `verdict_schema` keeps spec 130's rules; with any config, `create` writes `verdict_schema: 2`.

  Tests: `TestGateV2_SubmittedOnly`, `TestGateV2_DigestBinding`, `TestGateV2_DraftAndVoidMissing`, `TestGateV2_ReplacementAndDischarge`, `TestGateV2_DischargePositional`, `TestGate_VerdictSchemaPinned`, `TestGate_PreV2RoundKeepsV1Rules`, `TestCreate_AlwaysV2`.
- [ ] **AC-5 (R4.1–R4.5, R4.9)**:
  - load refuses: a duplicate family; a harness runner with `argv`; a process runner without `output`; `prompt: file` without `{prompt_file}`; a `{shell}` placeholder; `x{tree}`; `argv[0]` = `{tree}/run`; `on_quota` set to its own family; `panel.substitution` (the recovery names `on_quota`); `models: {reviewer.b: "--x"}`, a model with a space, and an empty model; `on_quota` on a harness runner; `create` refuses a reviewer entry `model: "-x"`;
  - a process runner with no `timeout_minutes` has a 60-minute limit;
  - zero-config `bead` gives 4 slots, all family `harness`;
  - config runners `a` and `b` and no reviewers give `bead` slots harness, a, b, harness; a config runner `{family: harness, kind: process, …}` replaces the default;
  - `TestNoFamilyLiterals` finds no tool name from the data file in the R4.5 paths outside the exemptions, and passes on the rename-table value `G1-codex`; a planted literal `"codex"` in `internal/panel` fails it; `KnownModels` does not exist.

  Tests: `TestModelValue_Refusals`, `TestRunners_Validate`, `TestRunners_TimeoutDefault`, `TestRunners_ZeroConfigHarness`, `TestRunners_MergeByFamily`, `TestSubstitution_Retired`, `TestNoFamilyLiterals`.
- [ ] **AC-6 (R4.6–R4.13)**:
  - `create` writes `family` on each roster entry; a slot with only `family: b` and `models: {reviewer.b: m1}` has model `m1`;
  - a model-only reviewer entry `{model: m7}` gets family `harness` and model `m7`; `config resolve` and `doctor` each print one Warn for it with `add a family and a runner`;
  - a config with `{family: x}` and no runner `x`: `create` refuses, `config resolve` refuses, and `doctor` gives an Error; each recovery names `panel.runners`;
  - a substitution from `a` (entry model `m1`) to `b` with `models: {reviewer.b: m2}` records model `m2`; with no such key, model `b`;
  - a pre-change panel whose roster has model `harness` and no family gets family `harness` at its first v2 round; one with model `m9` and no runner `m9` refuses with `--reviewers`; earlier round records stay byte-identical;
  - a local file that replaces runner `b` changes `config resolve` output, and the source column says `config.local.yaml`;
  - a local file with `loop:`, a 65537-byte local file, and an unparseable local file each make `mindspec config show` refuse;
  - with a listed tool command on `PATH` and no runner for it, `config resolve` prints one suggestion line; without it, none;
  - this repository's `.mindspec/config.yaml` loads, `config resolve --gate bead` gives no refusal, and the `codex` runner has the R4.13 `quota_signals` and `on_quota: claude`.

  Tests: `TestRoster_Family`, `TestSlotModelResolution`, `TestReviewer_ModelOnlyIsHarness`, `TestReviewer_ModelOnlyWarn`, `TestCreate_RefusesUnrunnableFamily`, `TestSubstituteModel`, `TestCreate_FirstV2RoundFamily`, `TestLocalConfig_Merge`, `TestLocalConfig_Refusals`, `TestConfigResolve_Provenance`, `TestConfigResolve_ToolSuggestion`, `TestRepoConfig_Resolves`.
- [ ] **AC-7 (R5)**: Zero-config `spec_approve` slots get the four document lenses in order, then repeat. Zero-config `bead` slots get the first four R5.4 lenses. A gate entry with only `lenses` loads. Load refuses `panel.gates.bead.lenses: [nope]`. The BRIEF of each gate holds its R5.5 question. No default focus text contains a tool, language or library name. Tests: `TestLenses_GateDefaults`, `TestLenses_LensOnlyGateEntry`, `TestLenses_UnknownRefused`, `TestBrief_AcceptanceRegion`.
- [ ] **AC-8 (R6)**:
  - `panel checkout --all` on a 4-slot panel creates 4 slot directories, each with `slot.json` (with the main root) and a `tree/` whose `HEAD` is the reviewed SHA; a rerun changes nothing; `git -C tree remote` prints nothing;
  - in a slot tree, `git status` and `git stash` change nothing in the main repository's git directory;
  - a tree at another SHA refuses with `--prune --slot` in the recovery; `--prune --slot R2` leaves R1's tree;
  - a dirty tree is removed by `create --round 2`; `abandon` removes all;
  - `checkout --slot --dispatch` on a harness slot appends one `dispatched` event; on a process slot it refuses; after a final APPROVE it refuses without `--replace`;
  - after a final, plain `checkout --slot` and `checkout --all` leave the slot `final` and the ledger byte-identical;
  - after `--dispatch --replace`, a submit with no new `attempt-<k>/verdict.json` refuses, and the slot stays `running`; a submit of `attempt-1/verdict.json` refuses with `superseded`, and the slot stays `running`;
  - from one dispatched `attempt-1/`: a draft then a final gives `final`; a second submit of the same bytes succeeds and repairs a damaged or absent `<slot>-round-<N>.json`; after a `launch_failed` outcome for attempt 1 (the fixture writes it to the ledger directly), a submit refuses with `ended with launch_failed`; a long-form submit of `attempt-1/verdict.json` after `--dispatch --replace` refuses with `superseded`;
  - a `tree/.git/commondir` file, `core.worktree` or `core.bare` in the tree's local config, and an extra line in `objects/info/alternates` each make a rerun refuse with `--prune --slot`;
  - a launcher paused after it opens the slot lock file and before it locks it; `checkout --prune` runs; the launcher resumes beside a second launch: at most one runner starts, and the lock file still exists;
  - load refuses an empty, absolute, `.`, `.git/x`, `.worktrees/x` and tracked root; checkout refuses a symlinked root and a non-ignored root (recovery `mindspec init`);
  - on an existing install, `mindspec init` adds the default ignore line; with `panel.isolation_root: review-scratch`, it adds `review-scratch`, and checkout then succeeds;
  - a `panel.json` and an OWNERSHIP-matching file under the root are not seen by `panel tally`, the `doctor` ownership walk or the links walk;
  - a `tree/.git` that is a gitdir file, and a `tree/` that is a symlink, each make checkout refuse with `--prune --slot`; a `core.fsmonitor` command set in the tree's git config, which writes a marker file, does not run on a checkout rerun;
  - with `GIT_DIR` set to the main repository's git directory in the caller's environment, checkout creates the clone in the slot, the fake reviewer child sees no `GIT_` variable from `git rev-parse --local-env-vars`, and `git stash` in the tree leaves the main refs unchanged;
  - while a fake runner blocks under a held slot lock: `checkout --prune`, `create --round 2` and `abandon` each refuse, name the held slot, and change no file and no ledger record.

  Tests: `TestPanelDispatch_StaleFileRefused`, `TestPanelSubmit_SupersededAttemptRefused`, `TestPanelSubmit_SameAttemptDraftThenFinal`, `TestPanelSubmit_SameAttemptRerunRepairs`, `TestPanelSubmit_EndedAttemptRefused`, `TestPanelSubmit_LongFormSuperseded`, `TestPanelCheckout_DispatchFlag`, `TestPanelCheckout_PlainWritesNoLedger`, `TestPanelCheckout_GitRedirectRefused`, `TestSlotLock_StableAcrossPrune`, `TestPanelCheckout_TreeLstat`, `TestPanelCheckout_NoFsmonitor`, `TestPanelCheckout_GitEnvScrubbed`, `TestPanelCleanup_RefusesHeldLock`, `TestPanelCheckout_Creates`, `TestPanelCheckout_GitStaysInSlot`, `TestPanelCheckout_NoOrigin`, `TestPanelCheckout_Refusals`, `TestPanelCheckout_Prune`, `TestPanelCheckout_HarnessDispatched`, `TestIsolationRoot_Validate`, `TestBootstrap_ScratchIgnoredExisting`, `TestBootstrap_CustomRootIgnored`, `TestWalks_SkipIsolationRoot`.
- [ ] **AC-9 (R6.9)**: With a REAL slot clone of a fixture repository:
  - `mindspec panel submit --file v.json` run from `<slot_dir>/tree/sub/` appends to the main ledger and writes the main verdict file; the clone's `.mindspec/` stays byte-identical;
  - the long form run from `tree/sub/` does the same;
  - a `tree/sub/slot.json` with no sibling `tree/` is ignored; a slot directory whose `slot.json` names another slot refuses; a copy of a slot directory outside the isolation root refuses;
  - `mindspec rules` run from `tree/` reads the main root.

  - `TestSlotRoot_SubmitFromFilePath`: from the main root with an absolute `F` in a slot directory, submit succeeds; a relative `F` refuses; with the cwd in slot A and `F` in slot B, submit refuses and names both.

  Tests: `TestSlotRoot_SubmitFromFilePath`, `TestSlotRoot_SubmitFromClone`, `TestSlotRoot_Refusals`, `TestSlotRoot_Rules`.
- [ ] **AC-10 (R7)**: With a fake runner script per case:
  - `create`, then the R12.1 adapter sequence, runs a process runner that reads a tracked file in `tree/` and checks that `tree/` `HEAD` is the reviewed SHA before it prints its verdict;
  - a valid nonce block on stdout and exit 0 gives `submitted`; so does a valid `verdict.json` in file mode;
  - a valid block on stderr only gives `malformed`; a block with another nonce gives `malformed`; two blocks give `malformed`;
  - a valid block with exit 1 gives `launch_failed`, and `recover --reason r` then submits it with `via: recover`; after one byte of `stdout.log` changes, `recover` refuses;
  - in file mode, a valid `verdict.json` with exit 1 gives `launch_failed`, and `recover` submits it; after the file changes, `recover` refuses;
  - exit 2 with the quota string on stderr gives `quota_wall`; the same string on stdout only gives `launch_failed`; a timeout gives `launch_failed`;
  - the first `malformed` starts one more attempt whose prompt holds the refusal; an unknown field whose name holds an ESC byte is escaped in launch output and in the retry prompt; a second `malformed` stops;
  - on `quota_wall` with `on_quota: b`, one `substitution` event is appended, the slot starts on `b` in a new attempt, and verify shows `substituted a->b`; a stale `verdict.json` from the `a` attempt is not submitted when `b` fails; a second `quota_wall` is terminal;
  - with no `on_quota`, a quota wall starts no other runner;
  - a draft verdict and a `VOID` verdict from a launch give verify status `draft` and `void`, and launch exits non-zero for each;
  - two launches that return the same draft give `draft`, not `running`; the same holds for two identical `VOID` verdicts; a harness redispatch (`checkout --slot --dispatch`) followed by the same draft gives `draft`;
  - the child environment holds no `MINDSPEC_` variable;
  - a second concurrent launch of the slot refuses; a harness slot refuses, naming `panel prompt`;
  - launch exits 0 only for a submitted final; no ledger record contains the isolation root;
  - a final `VOID`, then the R12.1 relaunch without `--replace`, starts attempt 2; a final APPROVE from it makes verify exit 0, and tally runs;
  - launch is killed with SIGKILL while the fake reviewer sleeps: the supervisor still kills the group at the timeout; before the timeout, a second launch kills the group first (its marker file stops growing) and appends `launch_failed` for attempt 1 before it starts attempt 2; `create --round N+1` in the same state kills the group and succeeds;
  - the supervisor is killed before its `pgid` rename: `run.lock` becomes free, no argv ran, and the next launch proceeds;
  - launch is killed after it starts the supervisor and before it writes `start`: the supervisor exits, no argv ran, and the next launch proceeds and runs alone;
  - a supervisor paused while it holds `run.lock`, with a `started` event, launch killed, and the `pgid` file absent or changed to another value: the next launch sends SIGKILL to the ledger `pgid` only, the attempt gets `launch_failed`, and the launch proceeds;
  - a supervisor paused before its `pgid` rename, with launch killed before the `started` append: the next launch refuses, naming the attempt directory;
  - launch is killed, the supervisor ends with exit 1 and a valid verdict: the next launch's orphan check appends `launch_failed` with the exit code and the three digests, and `panel recover` submits the verdict;
  - verify runs in a tight loop against a slot with a dead earlier attempt whose ledger `pgid` names a live unrelated group, while 20 launches start: no signal is sent, and no launch refuses;
  - on Windows, one harness slot goes 20 times through `checkout --dispatch`, verify probes, `submit`, a redispatch and `--prune`: no lock file stays after a verb ends, a stale lock file with a dead pid does not block the next verb, an empty lock file counts as held, and when two verbs race to take one stale lock, exactly one holds it (build-tagged test);
  - the package builds for `GOOS=windows`; on Windows, launch of a process slot refuses with `process runners need a Unix host` (build-tagged test);
  - SIGTERM to a supervisor while its child runs: after the supervisor exits, every process of the group stops within a bounded deadline (the test does not check the exit order);
  - a fake reviewer with `prompt: file` that reads stdin, run from a terminal: it reads end-of-file at once and does not stop;
  - a fake reviewer starts a `setsid` sleeper and exits 0 with a final verdict: `create --round 2` succeeds, and a later launch of the slot sends no signal;
  - an attempt whose `run.lock` is free and whose `pgid` names a live unrelated process group gets no signal; a `pgid` of 0 or 1 gets no signal;
  - while the fake runner sleeps, `panel verify` and a submit to another slot end without waiting;
  - each of these gives `final`, verify exit 0, a passing gate and no new `verdict` record: final A, a replacement that ends `launch_failed`, then a replacement that returns A; a replacement that is `malformed`, then the automatic retry that returns A; a harness dispatch that returns with no submission, then a redispatch that submits A;
  - a submitted final A, then `launch --replace` that returns A again: no new `verdict` record, the attempt is closed, the slot is `final`, verify exits 0, and the gate passes; the same for a harness `checkout --slot --dispatch --replace` and a CLI submit of A; a later `launch --replace` that ends with exit 1 blocks again;
  - `launch --replace` and `checkout --slot --dispatch --replace` on a landed panel and round refuse and append nothing;
  - quota handoff, end to end: a process slot with `on_quota` to a harness family hits `quota_wall`; verify `--json` gives kind `harness`, status `quota_wall` and `handoff_pending: true`, and exits 1; the R12.1 harness column redispatches; a fake harness agent submits; the slot is `final`. The same holds through the workflow fixture of AC-15.

  Tests: `TestPanelLaunch_VoidRetry`, `TestPanelLaunch_KillsOrphan`, `TestPanelLaunch_SupervisorTimeoutAfterLaunchDeath`, `TestPanelLaunch_FaultBeforePgid`, `TestPanelLaunch_LaunchDiesBeforeStart`, `TestPanelLaunch_LedgerPgidOnly`, `TestPanelLaunch_NoEventNoPgidRefuses`, `TestPanelLaunch_OrphanOutcomeDigests`, `TestPanelLaunch_VerifyLoopNoRefusal`, `TestPanelLaunch_WindowsRefuses`, `TestSlotLock_WindowsLifecycle`, `TestPanelLaunch_SupervisorTermKillsGroup`, `TestPanelLaunch_ChildStdinNull`, `TestPanelLaunch_SetsidDescendant`, `TestPanelLaunch_NoSignalStalePgid`, `TestPanelLaunch_LocksNotInherited`, `TestPanelLaunch_RecoverAfterFailedReplacement`, `TestPanelLaunch_IdenticalFinalReplacement`, `TestPanelDispatch_IdenticalFinalReplacement`, `TestPanelLaunch_LandedRoundRefused`, `TestPanelLaunch_QuotaHandoffE2E`, `TestPanelLaunch_AdapterE2E`, `TestPanelLaunch_Outcomes`, `TestPanelLaunch_Channels`, `TestPanelLaunch_Retry`, `TestPanelLaunch_Substitution`, `TestPanelLaunch_DraftAndVoid`, `TestPanelLaunch_IdenticalResubmit`, `TestPanelLaunch_Env`, `TestPanelLaunch_Refusals`, `TestPanelRecover_Stdout`, `TestPanelRecover_File`, `TestLedger_NoScratchPaths`.
- [ ] **AC-11 (R8)**:
  - a fixture with one slot in each R8.1 status gives that status;
  - a launch killed with SIGKILL while its supervisor lives (free slot lock, held `run.lock`) gives `running`, and verify exits 1; after the supervisor's timeout, with both locks free and no outcome, it gives `launch_failed`, and verify exits 2;
  - a `dispatched` slot with a hand-written verdict file gives `unsubmitted`;
  - a `dispatched` slot whose harness runner has `timeout_minutes: 1` and an event 2 minutes old gives `launch_failed`;
  - an APPROVE that replaced a final REQUEST_CHANGES shows `replaced REQUEST_CHANGES->APPROVE`, and `tally` prints one line for it; final RC, then APPROVE, then a different APPROVE still shows `replaced REQUEST_CHANGES->APPROVE`;
  - a final APPROVE, then `launch --replace` while the fake runner holds the lock, gives `running`, verify exits 1, and the gate blocks; when that replacement ends with exit 1, the slot is `launch_failed`, not `final`; a harness `checkout --slot --dispatch --replace` after a final APPROVE gives `running`;
  - verify exits 0 when every slot is `final`, 1 with one `running` slot, and 2 with one `launch_failed` slot and no pending slot;
  - a v1 round with all files parseable exits 0;
  - `--json` gives the same statuses and each slot's family, model, lens and kind;
  - an ESC byte in a verdict `model` is escaped in verify output; verify writes nothing.

  Tests: `TestPanelVerify_SlotStatus`, `TestPanelVerify_KilledLaunchLiveSupervisor`, `TestPanelVerify_KilledLaunchBothLocksFree`, `TestPanelVerify_DispatchedUnsubmitted`, `TestPanelVerify_HarnessTimeout`, `TestPanelVerify_Replaced`, `TestPanelVerify_ReplacementInFlight`, `TestPanelVerify_ExitCodes`, `TestPanelVerify_V1Round`, `TestPanelVerify_JSONRoster`, `TestPanelVerify_Termsafe`.
- [ ] **AC-12 (R9)**: A spec fixture whose AC section holds an ADR citation, `.mindspec/domains/x.md`, a file to create and a file to delete:
  - `create` succeeds, and the BRIEF quotes the section verbatim inside its fence;
  - outside the quotation, the BRIEF contains no `.mindspec/domains/` and no internal reference;
  - the conduct region holds every reviewer-scope rule id for the gate; `review.ci-parity` appears only for `final_review`;
  - the verdict contract names `mindspec panel submit`;
  - a template path absent at the reviewed SHA makes `create` refuse;
  - a spec with no AC section renders `no acceptance criteria section found`; an ESC byte in an AC is escaped;
  - `panel prompt` for a process slot names the nonce marker lines; for a harness slot it names `panel submit`; at round 2 it lists that slot's round-1 items only, with an ESC byte escaped; when round 1 has no `verdict_schema`, the items come from the parsed round-1 file.

  Tests: `TestBrief_QuotedACs`, `TestBrief_Regions`, `TestBrief_PathCheck`, `TestPanelPrompt_Render`, `TestPanelPrompt_PriorRound`, `TestPanelPrompt_PriorRoundV1`.
- [ ] **AC-13 (R10)**: The table loads and holds every R10.2 id. `rules --scope reviewer` lists exactly the reviewer-scope rules. `rules show nope` refuses. `instruct` in implement mode lists `lifecycle.no-probe-complete` and `review.derive`; in spec mode it lists the first and not the second. `ms-bead-fix` renders the fixer-scope rules into its prompt. The `ms-bead-impl` marker parses with `id=sequence.bead-impl-prompt`. No rule field, no shipped data file, and no R2–R12 string literal in non-test code matches `ADR-[0-9]`, `[Ss]pec [0-9]{3}`, `\bR[0-9]+\.[0-9]`, `#[0-9]{3}` or the bead-id pattern. Tests: `TestRuleTable_MinimumSet`, `TestRulesCmd`, `TestInstruct_RulesSection`, `TestBeadFix_FixerRules`, `TestStagedMarker_Parse`, `TestProductSurface_NoInternalRefs`.
- [ ] **AC-14 (R11)**:
  - a process runner whose `argv[0]` is not on `PATH` gives an Error naming the family;
  - a `probe` in `config.yaml` that creates a marker file is not run by plain `doctor`, and gives an Info line; under `--probe` it runs; an untracked local-config probe that exits 1 gives an Error;
  - a tracked `config.local.yaml` whose probe creates a marker file is not run by plain `doctor`, and gives the tracked-file Warn;
  - a config with `panel.substitution` gives one Error, the load error, with the `on_quota` recovery;
  - a non-ignored isolation root gives a Warn;
  - a marker with `remove-by=2020-01-01` and a marker with an unknown id each give a Warn;
  - a slot directory of an abandoned panel gives a Warn with `--prune`;
  - `doctor --ci` gives none of the R11.8 lines.

  Tests: `TestDoctor_Launchers`, `TestDoctor_ProbeNotRunFromTrackedConfig`, `TestDoctor_TrackedLocalProbe`, `TestDoctor_LoadErrorReported`, `TestDoctor_ConfigWarnings`, `TestDoctor_StagedMarkers`, `TestDoctor_StaleCheckouts`, `TestDoctor_CISkips`.
- [ ] **AC-15 (R12)**: `wc -l` is 80 or less for both copies of `ms-panel-run` and `ms-panel-tally`. For every R12.3 skill copy, `grep -n -i -E '/tmp|codex|copilot|gemini'` prints nothing, and every `claude` match is a path token such as `.claude/` or an R4.5 `runner:` value. Both `ms-panel.js` copies pass the same grep, and `ALLOWED_CLI` equals the R12.5 set, including `panel submit` and `panel recover`. A workflow fixture with one harness slot and a fake agent ends with the slot `final`, and no reviewer step uses the Write tool on a verdict path. In an adapter fixture, a fake harness agent that returns with no submission and no file gets exactly one redispatch (one more `dispatched` event), and then the slot is reported. Each `ms-panel-run` copy has one R12.1 table row for each non-final status. `codex_verdict_extract.sh` is absent. `mindspec setup claude` over an unmodified spec-130 install replaces every R12.3 skill. Tests: `TestPanelSkills_AdapterShape`, `TestPanelRun_StatusTable`, `TestSkills_NoFamilyNoTmp`, `TestMsPanelWorkflow_AllowedCLIExactSet`, `TestMsPanelWorkflow_HarnessSubmits`, `TestPanelRun_SilentHarnessRedispatchOnce`, `TestSetup_RefreshesSpec130Skills`.
- [ ] **AC-16 (R13)**: ADR-0037, ADR-0040, ADR-0043, ADR-0044 and ADR-0045 each contain `## Amendment (Spec 131)` with the R13 items. `interfaces.md` § Panel Artifact Schema names `panel submit` and the R8.1 statuses. `CHANGELOG.md` names the nine R13.6 changes and holds the before-and-after example. In `review-panels.md`, `README.md` and `plugins/mindspec/README.md`, `claude_sub_on_quota` and `panel.substitution` appear only in a sentence that says they are retired; neither README says "two model families" or "3 Claude + 3 Codex"; the plugin README has no line that requires the `codex` CLI. Tests: `TestADRAmendments_Spec131`, `TestUserDocs_NoRetiredPanelClaims`.
- [ ] **AC-17 (method)**: `go build ./... && go test ./... -short` is green, including `TestPanelLeafImports_StdlibPlusTermsafeOnly` and `TestEnforcementHasNoGitLeaks`.

## Validation Proofs

- In a scratch repo with a 2-slot zero-config v2 bead panel: `mindspec panel verify p; echo $?` prints `missing` twice and `1` (AC-11).
- `mindspec panel verify p --json` lists two slots with family `harness` and kind `harness` (AC-5, AC-11).
- `mindspec panel submit p --slot R1 --round 1 --file v.json` with a valid final APPROVE, then `mindspec panel verify p` shows `R1 final` (AC-3).
- Write `R2-round-1.json` by hand, then `mindspec panel tally p`: the panel blocks, and verify shows `R2 unsubmitted` (AC-4).
- With a fake process runner configured in `.mindspec/config.local.yaml`: `mindspec panel launch p R2; echo $?` prints `0`, and `mindspec panel verify p; echo $?` prints `0` (AC-10, AC-11).
- `cd "$(mindspec panel checkout p --slot R1)/tree" && mindspec panel verify p` reports on the main panel (AC-9).
- `mindspec rules --scope reviewer` lists `panel.submit` and `review.basis` (AC-13).
- `mindspec validate spec 131-panel-execution-in-binary` reports 0 errors.

## Open Questions

- [x] **OQ-1 (harness slots)** — RESOLVED. The binary cannot start a harness-native agent. A `harness` runner gives the slot to the skill's dispatch step. The binary still renders the prompt and validates the verdict through `submit`.
- [x] **OQ-2 (where the verdict comes from in a sandboxed CLI)** — RESOLVED. A process reviewer never runs `mindspec`. Launch reads the verdict from stdout markers or `verdict.json` and submits it from outside the sandbox.
- [x] **OQ-3 (checkout type)** — RESOLVED. Slot trees are `git clone --shared` clones with no `origin` remote. The clone's git directory is inside the slot directory, so a cwd-scoped sandbox can run git there. Shared objects keep a 6-slot panel cheap. A clone does not appear in `git worktree list`. Panel verbs find the main root through `slot.json` (R6.9).
- [x] **OQ-4 (quota detection and ZFC)** — RESOLVED. Detection uses the exit code and configured literal strings in the stderr tail only. A runner with no `quota_signals` never reports `quota_wall`.
- [x] **OQ-5 (v2 default)** — RESOLVED (operator). v2 is the default for every new round, with no opt-out (R3.1). Rounds created before this change keep spec 130's rules through the pinned `verdict_schema` field (R3.6).
- [x] **OQ-6 (replacing a final verdict)** — RESOLVED (operator, revised). No verdict is frozen. A later submission for the same slot and round replaces the earlier one. The ledger keeps every record, and the gate reads the latest (R2.4). Spec 130's positional text binding decides each discharge (R3.5).
- [x] **OQ-7 (families and harness binding)** — RESOLVED (operator). No family rule and no hard-coded family, in Go or in shipped runner data. Zero config runs every slot on the orchestrating harness. A one-family panel is valid. Diversity is a suggestion only (R4.12). No family minimum and no harness binding.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-10-21
- **Notes**: Approved via mindspec approve spec