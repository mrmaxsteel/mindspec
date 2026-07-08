---
adr_citations:
    - ADR-0040
    - ADR-0037
    - ADR-0036
    - ADR-0035
    - ADR-0034
spec_id: 111-workflow-panel-runner
status: Draft
version: "1"
work_chunks:
    - depends_on: []
      id: 1
      key_file_paths:
        - .mindspec/domains/workflow/OWNERSHIP.yaml
        - internal/validate/ownership_wave2_test.go
        - .mindspec/domains/workflow/architecture.md
    - depends_on:
        - 1
      id: 2
      key_file_paths:
        - plugins/mindspec/workflows/ms-panel.js
        - .claude/workflows/ms-panel.js
        - plugins/mindspec/embed.go
        - plugins/mindspec/workflow_test.go
        - internal/setup/claude.go
        - internal/setup/claude_test.go
        - .mindspec/domains/workflow/interfaces.md
    - depends_on:
        - 2
      id: 3
      key_file_paths:
        - plugins/mindspec/skills/ms-panel-run/SKILL.md
        - plugins/mindspec/skills/ms-panel-tally/SKILL.md
        - .claude/skills/ms-panel-run/SKILL.md
        - .claude/skills/ms-panel-tally/SKILL.md
        - .mindspec/domains/workflow/runbook.md
---
# Plan: 111-workflow-panel-runner

## ADR Fitness

The sole impacted domain is **workflow** (spec § Impacted Domains: every source
edit — the new `.claude/workflows/**` + `plugins/mindspec/workflows/**` workflow
artifact, `plugins/mindspec/embed.go`, `internal/setup/**`, `internal/validate/**`,
`plugins/mindspec/skills/**`, `.claude/skills/**`, and
`.mindspec/domains/workflow/OWNERSHIP.yaml` — lands under the `workflow`
OWNERSHIP globs; `internal/config` (the 109 `runner:`/`panel:` resolvers),
`internal/panel` + `cmd/**` (the 110 `mindspec panel` verbs + verdict schema),
and `internal/executor` are consumed **read-only as an existing CLI + artifact
contract** and are **not** impacted — 111 adds no source to any of them). Five
ADRs genuinely constrain the plan and are cited; three frequently-adjacent ADRs
were evaluated and **deliberately not cited**. **No bead diverges from any
accepted ADR** — the honest boundaries this spec draws (the workflow is an
adapter, never a second decision authority) are the accepted designs, not
departures from them.

- **ADR-0040 — Orchestration Layering Ratchet** (Domain(s): core, workflow;
  intersects workflow). The license and load-bearing frame: agents integrate at
  the **artifact + CLI contract** level (the `panel.json`/verdict-JSON schemas +
  the `mindspec panel` verbs), never at the prompt-format level, and
  orchestration **runners** are *adapters* behind those contracts with
  **degraded modes** for hosts lacking a capability. Spec 109 landed ADR-0040 on
  `main` (and declared the `runner:` key) before this branch rebased forward, so
  it is citable here (unlike 109's own plan). This plan makes it real: the
  workflow (Bead 2) is the first non-default runner adapter — it targets 110's
  verbs and verdict schema and never re-implements them (R2, R5); the runner
  dispatch (Bead 3) degrades a workflow-less host to the skills path (R6); and
  the workflow ships to the Claude Code target **only** (R8), codex/copilot
  staying skills-path per the capability tiers. This plan **adheres**.
- **ADR-0037 — Panel Gate as Enforced Contract** (Domain(s): workflow,
  execution; intersects workflow). The spine, binding Bead 2. §3's single home
  (`internal/panel.PanelGateDecision`, reached only through `mindspec complete`)
  is **not weakened**: the workflow is a read-side + write-side adapter that
  registers via `mindspec panel create` and returns `mindspec panel verify` /
  `mindspec panel tally` output verbatim, adding **no** second interpreter and
  **never** running `mindspec complete` (structurally pinned — the string
  appears nowhere in the file, AC4 + Bead 2's exact-set test). §8's
  "plain reviewable files, no signing" trust boundary is **extended, not
  changed**, to the codex transcription step: each codex slot persists its raw
  stdout to a tracked `<slot>-round-<N>.codex.log` audit artifact (R3b) so the
  wrapper's verdict is auditable against its source. This plan **adheres**.
- **ADR-0036 — Ownership Discovery** (Domain(s): workflow, validation, doc-sync,
  ownership; intersects workflow). Governs Bead 1's new `.claude/workflows/**`
  claim: `.claude/workflows/ms-panel.js` is governable source (`isDocFile` and
  `isProcessArtifact` both return false for it — verified against
  `internal/validate` below), so editing it unclaimed trips
  `adr-divergence-unowned`. The claim must be present **same-diff-or-earlier**
  than any `.claude/workflows/**` edit (the exact spec-108 `internal/trace` /
  `.golangci.yml` invariant), so it lands in Bead 1 (a real edge before Bead 2's
  workflow file). Also governs the gate-forward doc-sync every bead honors.
  Sound as-is.
- **ADR-0035 — Agent Error Contract** (Domain(s): workflow, execution, core;
  intersects workflow). Constrains Bead 2's failure surfaces: a codex quota
  wall, the substitution branch, a parse failure, and a MISSING slot must be
  guard-style, never silent. The workflow surfaces every failure through
  `mindspec panel verify`'s named-malformed / incomplete report and `mindspec
  panel tally`'s non-zero Block + recovery line (both already ADR-0035-compliant
  in 110), passed through **verbatim** in the workflow result — never as a
  dropped slot the workflow hides. Sound as-is.
- **ADR-0034 — Ceremony Collapse** (Domain(s): workflow; intersects workflow).
  The workflow adds **no new ceremony step and no new gate**: it replaces the
  launch mechanics of an existing panel with one invocation and hands off to the
  same `mindspec complete` merge terminal the skills path uses (the runner
  selection in Bead 3 is a dispatch, not a lifecycle transition). Sound as-is.

**Evaluated, deliberately NOT cited:**
- **ADR-0039 — Flat `.mindspec/` Layout v2** (Domain(s): core, workflow,
  execution, context-system). Real *context* — the workflow's verdict files land
  under the layout-aware `<spec-dir>/reviews/<slug>/` tree — but 111 introduces
  **no** `.mindspec/` layout logic: the panel-dir layout is 110's `mindspec
  panel create` concern (111 passes `<slug>` and the binary resolves the tree),
  and `.claude/workflows/` is a fixed Claude Code install location, not a
  `.mindspec/` layout tier. Citing it would add a non-constraining ADR; omitting
  it keeps `adr_citations` load-bearing (110 cited it because `panel create`
  *writes* the layout-aware dir; 111 does not). `workflow` coverage is
  unaffected — the five cited ADRs all name it.
- **ADR-0030 — Executor Boundary** (Domain(s): execution, validation, lifecycle,
  lint). The spec's ADR-0037 touchpoint draws the honest boundary vs 110
  explicitly: unlike 110's `panel create` (which rev-parses through the executor
  git-I/O boundary), 111 adds **no** executor code — the codex CLI is exec'd by a
  workflow agent's own shell tool, outside `internal/executor`. ADR-0030's
  Domain (`execution`) does **not** intersect `workflow`, so citing it would fire
  `adr-cite-irrelevant` at plan-approve. Mirrors 110's plan omitting ADR-0030.
- **ADR-0027 — MindSpec OTEL-Only** (Domain(s): observability, telemetry,
  recording, extraction). A spec Non-Goal reaffirmation ("no telemetry stream")
  — the workflow emits no metrics by adding none. It constrains no bead, and its
  Domain does not intersect `workflow`, so citing it would fire
  `adr-cite-irrelevant`.

**Coverage check.** `workflow` (the only impacted domain) is covered by all five
citations — ADR-0040/0037/0036/0035/0034 each name `workflow` in their
`Domain(s)` line and all are Accepted, so `checkADRCoverage` finds a cited
Accepted covering ADR and `checkADRCitations` finds no irrelevant citation.

**Divergence report: none.** No bead is better served by a design that departs
from an accepted ADR. The two designs a reviewer might probe are both refused by
the spec itself: (a) letting the workflow run `mindspec complete` / consolidate /
author `consolidated-round-<N>.md` — rejected, it breaks ADR-0037's single-home
invariant (R5, structurally pinned); (b) trusting a Claude Code
platform-level output-schema guarantee for verdict conformance — rejected, the
docs describe none, so conformance is enforced by prompt + `mindspec panel
verify` + the R3 same-reviewer re-serialize-or-MISSING ladder (Non-Goals).

## Testing Strategy

**Approach.** The runner is a Claude Code dynamic workflow — a `.js`
orchestration script whose *behavior* (registration → fan-out → parse-retry →
quota-wall substitution → verify/tally-return) executes only inside the Claude
Code runtime with live agents, so it is **not** unit-testable in `go test`. The
automated gates therefore pin the workflow's **artifact identity, structure,
distribution, and ownership** deterministically, and the behavioral requirements
(R2–R5, R3b) are proven by the spec's **Manual e2e** (Bead 2 Validation Proof,
run against a built binary + live agents). Concretely, the Go-testable surface is:

- **Ownership** (Bead 1): `internal/validate.attributeDomain` resolves
  `.claude/workflows/ms-panel.js` to `workflow` — a pure path-glob test mirroring
  spec 108's `TestWorkflowOwnsTraceAndGolangci` (no filesystem read of the
  workflow file: `attributeDomain` matches path strings against the manifest, so
  the test passes on the claim alone, before Bead 2's file exists).
- **Embed + distribution** (Bead 2): `plugins/mindspec.WorkflowFiles()` returns
  the embedded workflow, and `internal/setup.RunClaude` installs it to
  `.claude/workflows/` byte-identical while `RunCodex`/`RunCopilot` do **not**.
- **Structural / anti-laundering** (Bead 2): the `ALLOWED_CLI` allowlist is a
  static literal array admitting **exactly** the four permitted commands, `mindspec
  complete` appears nowhere, `.codex.log` and `claude-sub` are present, and there
  is **no dynamic command construction** (`mindspec ${…}` / concatenation) that
  could smuggle a fifth command past the grep AC.

**The AC4 exact-set strengthening (spec-approve round-2 carry-forward #1).** R5's
requirement text claims `ALLOWED_CLI` admits *exactly* four commands, but spec
AC4 only proves the four are **present** and `mindspec complete` is **absent** —
a fifth admitted command would pass AC4 while falsifying R5. Because ACs are
floors, Bead 2 adds `TestMsPanelWorkflow_AllowedCLIExactSet` (Go test over the
embedded workflow text) that extracts the `ALLOWED_CLI` array literal, parses its
string elements, and asserts the set **equals** the four-element list `{"mindspec
panel create", "codex exec", "mindspec panel verify", "mindspec panel tally"}` —
failing on any extra, any missing, or any rename. The same test asserts (a)
`mindspec complete` is absent from the whole file and (b) **no dynamic
mindspec-command construction** appears (spec-approve carry-forward #3: it greps
the file for `mindspec ${`, a backtick-template `mindspec` command, and a string
`"mindspec " +` concatenation, each an indirection that would defeat a
literal-only allowlist). This strengthens R5 to its full "exactly four"
guarantee without a spec edit.

**Per-test proof discipline.** Every new Go test is verified with an anchored
PASS-line grep so a reviewer sees the specific test pass, not a bare package
green: `go test ./PKG -v -run 'TestName$' | grep -q -- '--- PASS: TestName'`. The
`$` anchor on `-run` stops a prefix sweeping a sibling. The three named test
functions use exactly the spec's Acceptance-Criteria names
(`TestWorkflowOwnsClaudeWorkflows`, `TestWorkflowFiles_EmbedsMsPanel`,
`TestClaudeSetup_InstallsWorkflowClaudeTargetOnly`); the fourth
(`TestMsPanelWorkflow_AllowedCLIExactSet`) is the carry-forward floor-raise.
**Grep note:** this machine's `grep` is ugrep; the fixed-string `grep -q` /
`grep -qF` forms below are ugrep-safe as written; any name-anchored file-membership
check uses `/usr/bin/grep -qxF` explicitly. **Any self-check that runs the binary
runs a BRANCH-BUILT binary** (`go build -o /tmp/ms111 ./cmd/mindspec`), never the
pre-installed `~/.local/bin/mindspec`, which predates this branch.

**JS syntactic validity (advisory, not a CI gate).** A JS toolchain is present on
this machine (`node v25.2.1`), so Bead 2 records `node --check
.claude/workflows/ms-panel.js` (and the plugin copy) exiting `0`. Per the spec's
Validation Proofs this is **recorded when a JS toolchain is present, not a CI
gate** — CI has no guaranteed node, and the workflow's authority is Claude Code,
not node.

**Consumer ownership (the created-count pin).** Wiring `installWorkflows` into
`RunClaude` grows a fresh Claude setup from 14 to 15 created items, so Bead 2
**owns** its consumers in `internal/setup/claude_test.go`:
`TestRunClaude_FreshSetup`'s `len(r.Created) != 14` and `TestRunClaude_Idempotent`'s
`len(r2.Skipped) != 14` both move to `15` (with the comment updated) in the same
bead — without the update `go test ./internal/setup` goes red. This is why
`internal/setup/claude_test.go` is in Bead 2's `key_file_paths`. `skillFiles()`
still returns 12 skills (workflows are not skills), so
`TestSkillInventory_Twelve` is unaffected.

**Regression.** Full `go test ./...` runs once at **plan time** and again
**pre-`/ms-impl-approve`** — not per bead; per-bead gates run the touched packages
only. Plan-time result (2026-07-08, this post-109 worktree): `go build ./...`
green; the three packages this spec touches — `internal/validate`,
`internal/setup`, `plugins/mindspec` (currently `[no test files]`; Bead 2 adds
the first) — are green. The pre-existing `internal/instruct`
`TestRun_IdleNoBeads` environment-isolation failure (tracked as `z4ps`) is
unrelated and **not** in this spec's surface — 111 touches no
`internal/instruct` code. Git-touching tests run with `GIT_TERMINAL_PROMPT=0`. No
new external dependency, no network access. **Advisory WARN expected:** the repo
has no `source_globs` in `.mindspec/config.yaml`, so `validate plan` emits the
non-gating `missing-source-globs` migration nudge; this is pre-existing and
unrelated to 111.

**Dependency shape (decomposition / the DAG).** Three beads (within the 3–5
optimal band, ≤ the 6 advisory cap), forming the single chain the spec's
compile/gate facts force — **1 → 2 → 3**, depth 3 (= the advisory MaxChainDepth
of 3), with Bead 1 the sole root (parallelism 0.33, above the 0.25 floor). Both
edges are **real**, not ordering wishes:
- **Bead 2 `depends_on: [1]`** — Bead 2 adds `.claude/workflows/ms-panel.js`,
  which trips `adr-divergence-unowned` unless the `.claude/workflows/**` claim
  Bead 1 lands is visible at Bead 2's diffed ref. Divergence reads each domain's
  `OWNERSHIP.yaml` at the diffed ref, so the claim must appear in an **earlier**
  bead (or the same diff); an earlier bead is the cleaner, independently-reviewable
  form (the spec-108 precedent this spec cites). Not a false edge — a parallel
  split would let Bead 2 be diffed before Bead 1 and fail its own per-bead gate.
- **Bead 3 `depends_on: [2]`** — Bead 3's `ms-panel-run` runner branch invokes
  the `/ms-panel` workflow when `runner: claude-code-workflow`. That dispatch is
  only truthful once the workflow artifact exists and ships (Bead 2); merging the
  skill first would leave the dispatch pointing at an unshipped workflow. A real
  functional edge (mirrors 110's Bead 5 → Bead 4 "skills must reference verbs
  that exist").

The single-root chain is the honest shape of a **three-layer adapter** —
gate-enablement (ownership) → artifact + distribution → skill dispatch — not a
decomposition defect: each layer is a compile/gate prerequisite of the next, and
forcing parallelism would introduce a false edge or a dangling-reference hazard.
Because the three beads touch **disjoint** file sets (ownership manifest+test /
`.js`+embed+setup / skills), `validate plan` emits a non-gating
`decomposition-scope-redundancy` low-overlap WARN (R≈0.04 < 0.15) — the expected
shape of a cleanly separated three-domain-slice change, not a decomposition
defect (110's plan noted the same for its five-package split).
**Doc-sync is collision-free by construction:** the three beads append to
**disjoint** `workflow` domain-doc files — Bead 1 → `architecture.md` (the
ownership-claim lineage, joining spec 108's "Ownership claims" section), Bead 2 →
`interfaces.md` (the workflow adapter + embed/install distribution surface), Bead
3 → `runbook.md` (the runner-selection operator procedure) — so even a
hypothetical reordering never conflicts.

**Doc-sync footprint (precise).** The repo declares no `source_globs`, so the
built-in classifier drives doc-sync: only `.go` files under `internal/`/`cmd/`
(non-`_test.go`) count as source requiring a same-diff `workflow` doc region. By
that rule the **only** file strictly forcing a doc region is Bead 2's
`internal/setup/claude.go`; Bead 1's `OWNERSHIP.yaml` classifies as a doc
(`.mindspec/domains/**`) and its test is `_test.go`, and Bead 3's `.js`/SKILL.md
edits are non-`.go` — so those two beads' doc regions are added to honor the
spec's Scope gate-forward planning constraint (every workflow-source bead
documents its change) and are verified by a `git show --name-only` membership
grep, **not** because the doc-sync gate would block them. This mirrors 110's
skills-only Bead 5, which carried a `runbook.md` region under the same planning
constraint.

**Requirement → bead map.** R1 → Bead 2 (the tracked workflow artifact, both
byte-identical copies); R2 → Bead 2 (registration through `mindspec panel
create`); R3 → Bead 2 (fan-out + the anti-laundering re-serialize-or-MISSING
ladder); R3b → Bead 2 (the `.codex.log` audit artifact); R4 → Bead 2 (the
deterministic quota-wall substitution branch); R5 → Bead 2 (verify+tally return +
the `ALLOWED_CLI` allowlist, strengthened to exactly-four by
`TestMsPanelWorkflow_AllowedCLIExactSet`); R6 → Bead 3 (runner dispatch in
`ms-panel-run`); R7 → Bead 3 (skill slimming + the retained judgment sections);
R8 → Bead 2 (embed + Claude-target-only install); R9 → Bead 1 (the
`.claude/workflows/**` claim + `TestWorkflowOwnsClaudeWorkflows`). Every spec
requirement is delivered; the Provenance table maps every spec acceptance
criterion.

## Bead 1: ownership — claim `.claude/workflows/**` for the workflow domain + attribution test

Delivers R9. The `.claude/workflows/ms-panel.js` runner artifact (Bead 2) is
governable source, so its owning domain must claim it **before** it is edited.
This bead lands the claim and the attribution test as a small, independently
reviewable root — the spec-108 `internal/trace` / `.golangci.yml`
same-diff-or-earlier precedent. Doc-sync: `.mindspec/domains/workflow/architecture.md`
(the ownership-claim lineage).

**Steps**
1. Add `- .claude/workflows/**` to the `paths:` list of
   `.mindspec/domains/workflow/OWNERSHIP.yaml`. Place it adjacent to the existing
   `.claude/skills/**` claim (line 21) so the two `.claude/**` sub-tree claims sit
   together; `plugins/mindspec/workflows/**` needs **no** new claim (already
   covered by the existing `plugins/mindspec/**` glob). Verify against
   `internal/validate` that the target file classifies as governable source:
   `isDocFile(".claude/workflows/ms-panel.js")` is false (no `docs/`,
   `.mindspec/`, `project-docs/`, or root-operator-doc prefix) and
   `isProcessArtifact(...)` is false (no `/reviews/` segment, not under `.beads/`
   or `review/`) — so without the claim it would trip `adr-divergence-unowned`.
2. Append `TestWorkflowOwnsClaudeWorkflows` to
   `internal/validate/ownership_wave2_test.go` (reusing the file's existing
   `repoRootForWorkflowManifest(t)` helper, which walks up to the **live**
   committed `.mindspec/domains/workflow/OWNERSHIP.yaml` so removing the claim
   fails the test): call
   `attributeDomain(nil, root, "", ".claude/workflows/ms-panel.js",
   []string{"workflow"})` and assert it returns `("workflow", non-nil Ownership,
   nil err)` — the exact shape of the sibling `TestWorkflowOwnsTraceAndGolangci`.
   The test needs no on-disk workflow file: `attributeDomain` glob-matches the
   path string against the manifest, so it passes on the claim alone.
3. Doc-sync (workflow): add a bead-unique region to
   `.mindspec/domains/workflow/architecture.md` (under/after the existing
   "Ownership claims + carve-out cleanup — spec 108 wave 2" section) recording
   the spec-111 `.claude/workflows/**` claim — that dynamic workflows are
   governable source the workflow domain now owns, added same-diff-or-earlier
   than the workflow artifact per ADR-0036 (the spec-108 invariant).

**Verification**
- [ ] `grep -Eq '^[[:space:]]*-[[:space:]]+\.claude/workflows/\*\*' .mindspec/domains/workflow/OWNERSHIP.yaml` exits `0` (spec AC2, R9 — the claim is present)
- [ ] `go test ./internal/validate -v -run 'TestWorkflowOwnsClaudeWorkflows$' | grep -q -- '--- PASS: TestWorkflowOwnsClaudeWorkflows'` (spec AC3, R9)
- [ ] `go test ./internal/validate` exits `0` (whole package green — the existing ownership/divergence/doc-sync tests still pass, proving no manifest-parse regression)
- [ ] `git show --name-only HEAD | /usr/bin/grep -qxF '.mindspec/domains/workflow/architecture.md'` (doc-sync: the ownership claim carries a workflow domain-doc region — spec Scope gate-forward constraint)
- [ ] `go build ./...` exits `0`

**Acceptance Criteria**
- [ ] `.mindspec/domains/workflow/OWNERSHIP.yaml` claims `.claude/workflows/**`
  and `attributeDomain` returns `"workflow"` for `.claude/workflows/ms-panel.js`,
  so no later bead editing it trips `adr-divergence-unowned` (spec AC2, AC3, R9)

**Depends on**
None

## Bead 2: the `/ms-panel` workflow adapter — tracked artifact (both copies) + embed + Claude-target-only install + structural/exact-set tests

Delivers R1, R2, R3, R3b, R4, R5, R8. The workflow `.js` is the runner adapter
behind 110's `mindspec panel` verbs + verdict schema and 109's `panel:` mix /
`substitution.claude_sub_on_quota` config; `plugins/mindspec/embed.go` ships it
and `internal/setup/claude.go` installs it to the Claude target only. **Depends
on Bead 1** — the `.claude/workflows/**` claim must be visible at this bead's
diffed ref before `.claude/workflows/ms-panel.js` is added (a real
adr-divergence edge). Doc-sync: `.mindspec/domains/workflow/interfaces.md`.

**Steps**
1. Author `plugins/mindspec/workflows/ms-panel.js` — the Claude Code dynamic
   workflow (a JavaScript orchestration script invocable as `/ms-panel`) whose
   **script coordinates agents and itself does no shell/file I/O** (the
   documented workflow limit; every CLI touch + file write is an `agent()`
   step). It accepts via the documented `args` input `{slug, spec, target,
   bead_id?, round, sha?, lenses[], mix}`. Declare the command allowlist as a
   **static literal array at the top of the file** — no template interpolation,
   no concatenation — so it is machine-parseable and cannot smuggle a fifth
   command:
   ```js
   // ALLOWED_CLI — the exact, exhaustive set of shell commands any /ms-panel
   // agent step may exec. Adding a command here (or building one dynamically)
   // is a gate-integrity change; TestMsPanelWorkflow_AllowedCLIExactSet enforces
   // this set is exactly these four. `mindspec complete` is intentionally absent
   // — this workflow is an adapter, never a lifecycle mutator (ADR-0037/0040).
   const ALLOWED_CLI = [
     "mindspec panel create",
     "codex exec",
     "mindspec panel verify",
     "mindspec panel tally",
   ];
   ```
   `sha?` is **advisory-only** (an optional BRIEF display hint); the authoritative
   `reviewed_head_sha` is self-resolved by `mindspec panel create` from
   `--target` at write time (110 R1), so the workflow **never** uses `sha?` to
   set the recorded SHA and works when it is omitted.
2. Registration step (R2): a single `agent()` step that execs `mindspec panel
   create <slug> --spec <spec> --target <target> [--bead <bead_id>] [--round
   <round>]` — so `panel.json` (round + `reviewed_head_sha` co-bumped by
   construction, `expected_reviewers` / `approve_threshold` stamped from the 109
   resolvers) is written **by the binary**. A re-panel passes `--round N+1`
   (re-resolving the SHA in the same write; prior-round `<slot>-round-<K>.json`
   files untouched). The workflow contains **no** hand-typed `panel.json` schema
   and **no** re-implementation of the round+SHA co-bump — it reads back only the
   BRIEF path `panel create` reports.
3. Fan-out step + the anti-laundering ladder (R3, R3b): iterate `mix` (the
   resolved `[{family,count}]` from config `panel:`), emitting one agent step per
   slot with per-slot lens from `lenses[]`. A **claude** slot is an `agent()`
   step prompted with the BRIEF path + its lens + the 110 verdict-JSON shape
   (`reviewer_id`, `verdict`, `confidence`, `rationale`,
   `concrete_changes_required`, `findings`; optional top-level `hard_block`),
   instructed to **write** its verdict to the 110 contract path
   `<spec-dir>/reviews/<slug>/<slot>-round-<N>.json`. A **codex** slot is a
   **wrapper agent** that: execs `codex exec` with the BRIEF prompt + lens;
   **persists codex's unmodified stdout** to
   `<spec-dir>/reviews/<slug>/<slot>-round-<N>.codex.log` (R3b — the string
   `.codex.log` must appear literally in the file, AC5) **before/as** it
   transcribes; parses that stdout into the verdict shape; and writes the verdict
   file **itself** via its `Write` tool. **Codex never writes files** — neither
   the verdict nor the log; the wrapper writes both (eliminating the
   sandbox-file-write failure class by construction). Verdict conformance is
   **not** assumed as a platform guarantee (the docs describe none): it is
   enforced by the prompt shape **plus** deterministic post-hoc `mindspec panel
   verify` (step 4). A rendered verdict can never be replaced by a different
   reviewer's verdict or a re-review — the ladder is:
   - (a) **Parse failure on a *rendered* verdict** (content produced, not valid
     schema JSON) → re-prompt the **same** reviewer **exactly once**, feeding
     back that reviewer's own rendered output (for a codex slot, the persisted
     `.codex.log`) with the instruction to re-emit **that same verdict** as valid
     JSON **without re-reviewing** — a serialization retry keeping the **same
     slot id and same family** `reviewer_id` (e.g. `R4 codex` stays `R4 codex`,
     **never** `claude-sub`).
   - (b) **Still unparseable after the single re-prompt** → the slot **fails
     CLOSED to a MISSING verdict** (no file written) → an incomplete panel → the
     gate Blocks. Never substituted, never replaced by another reviewer's verdict.
   - (c) **Substitution (step 4) is reserved EXCLUSIVELY** for a quota wall in
     which *no* verdict content was ever rendered — a rendered-but-malformed
     verdict is out of substitution's reach.
4. Quota-wall substitution branch (R4): when a codex wrapper reports a quota wall
   (the codex CLI usage-limit signal) **with no verdict rendered** (the step-3(c)
   precondition) **and** config `panel.substitution.claude_sub_on_quota == true`,
   substitute a **claude** `agent()` for that slot in the same round, **keeping
   the slot id** and writing `reviewer_id: "<slot> claude-sub"` (the string
   `claude-sub` must appear literally, AC4). When the flag is `false`, **leave the
   slot unfilled** → a missing verdict at `panel verify` → the gate Blocks (the
   workflow never fabricates or silently skips a slot). A code branch driven by
   the wrapper's returned status — **no** human judgment on the workflow path.
   Then the return step (R5): one `agent()` step execs `mindspec panel verify
   <slug>` and another execs `mindspec panel tally <slug>`, and the workflow
   **returns their combined stdout verbatim** (unmodified, not re-rendered or
   paraphrased) as its single structured **result**. The workflow does **not**:
   run `mindspec complete` (the string appears nowhere in the file — not a
   command, not a comment, AC4); perform tally **consolidation** (semantic dedup
   + criticality ranking — 110 R8 skill judgment); author
   `consolidated-round-<N>.md`; or mutate `panel.json` beyond `panel create`.
5. Ship the byte-identical dogfood copy + embed + install (R1, R8): copy
   `plugins/mindspec/workflows/ms-panel.js` to `.claude/workflows/ms-panel.js`
   **byte-for-byte** (the tracked copy Claude Code reads). In
   `plugins/mindspec/embed.go` add `//go:embed workflows/*` into a new
   `workflowsFS embed.FS` var and a `WorkflowFiles() map[string]string` accessor
   mirroring `SkillFiles()` — walking `workflows/`, keyed by **file basename**
   (e.g. `"ms-panel.js"` → content), map rebuilt fresh per call. In
   `internal/setup/claude.go` add `installWorkflows(workflowsDir, workflowsRel,
   pluginmindspec.WorkflowFiles(), check, r)` mirroring `installSkills`'
   create/refresh/skip/notice disposition (write `<workflowsDir>/<basename>`),
   and call it from `RunClaude` after `installSkills` (writing to
   `.claude/workflows`). Do **not** add any workflow install to `RunCodex` /
   `RunCopilot` (they install only to `.agents/skills/` — dynamic workflows are a
   Claude Code capability, ADR-0040 tiers). Update the created-count consumers in
   `internal/setup/claude_test.go`: `TestRunClaude_FreshSetup` `14 → 15` and its
   comment, `TestRunClaude_Idempotent` `len(r2.Skipped) 14 → 15`.
6. Add `plugins/mindspec/workflow_test.go` (the package's first test file) with
   `TestWorkflowFiles_EmbedsMsPanel` (asserts `WorkflowFiles()["ms-panel.js"]` is
   non-empty and **equals** the on-disk `workflows/ms-panel.js` read via the
   embed FS — pinning embed == plugin copy) and
   `TestMsPanelWorkflow_AllowedCLIExactSet` (the carry-forward floor-raise:
   extract the `ALLOWED_CLI` array literal from `WorkflowFiles()["ms-panel.js"]`,
   parse its double-quoted string elements, assert the set **equals exactly**
   `{"mindspec panel create","codex exec","mindspec panel verify","mindspec panel
   tally"}`; assert `mindspec complete` is absent from the whole content; assert
   **no dynamic command construction** — the content contains none of `mindspec
   ${`, a backtick-delimited `mindspec` command template, or a `"mindspec " +`
   concatenation). Add `TestClaudeSetup_InstallsWorkflowClaudeTargetOnly` to
   `internal/setup/claude_test.go`: `RunClaude(tmp, false)` writes
   `.claude/workflows/ms-panel.js` **byte-identical** to
   `pluginmindspec.WorkflowFiles()["ms-panel.js"]`, while `RunCodex(tmp2,false)`
   and `RunCopilot(tmp3,false)` create **no** `.claude/workflows/**` and no
   `.agents/workflows/**` file. Doc-sync (workflow): add a bead-unique region to
   `.mindspec/domains/workflow/interfaces.md` documenting the `/ms-panel` runner
   adapter — its `args` contract, the `ALLOWED_CLI` exactly-four allowlist, the
   codex-wrapper `.codex.log` audit artifact, the same-reviewer
   re-serialize-or-MISSING ladder, the quota-wall substitution branch, and that
   the workflow embeds + installs to the Claude target only.

**Verification**
- [ ] `test -f .claude/workflows/ms-panel.js && test -f plugins/mindspec/workflows/ms-panel.js && diff -q .claude/workflows/ms-panel.js plugins/mindspec/workflows/ms-panel.js` exits `0` (spec AC1, R1/R8 — both tracked copies present and byte-identical; combined with the embed test [embed == plugin] and the install test [installed == embed], all four copies are transitively identical)
- [ ] `W=.claude/workflows/ms-panel.js; grep -q 'ALLOWED_CLI' "$W" && for c in 'mindspec panel create' 'mindspec panel verify' 'mindspec panel tally' 'codex exec'; do grep -qF "$c" "$W" || exit 1; done && grep -q 'claude-sub' "$W" && ! grep -qF 'mindspec complete' "$W"` exits `0` (spec AC4, R2/R4/R5)
- [ ] `grep -qF '.codex.log' .claude/workflows/ms-panel.js` exits `0` (spec AC5, R3b)
- [ ] `go test ./plugins/mindspec -v -run 'TestWorkflowFiles_EmbedsMsPanel$' | grep -q -- '--- PASS: TestWorkflowFiles_EmbedsMsPanel'` (spec AC6, R8)
- [ ] `go test ./plugins/mindspec -v -run 'TestMsPanelWorkflow_AllowedCLIExactSet$' | grep -q -- '--- PASS: TestMsPanelWorkflow_AllowedCLIExactSet'` (carry-forward #1/#3: ALLOWED_CLI is EXACTLY the four commands, no `mindspec complete`, no dynamic construction — strengthens AC4 beyond a present/absent grep)
- [ ] `go test ./internal/setup -v -run 'TestClaudeSetup_InstallsWorkflowClaudeTargetOnly$' | grep -q -- '--- PASS: TestClaudeSetup_InstallsWorkflowClaudeTargetOnly'` (spec AC7, R8 — Claude target writes it byte-identical; codex/copilot do not)
- [ ] `go test ./internal/setup` exits `0` (whole package green — proves the created-count consumer updates [14→15] landed; a skipped update leaves this red)
- [ ] `go test ./plugins/mindspec` exits `0`
- [ ] `node --check .claude/workflows/ms-panel.js && node --check plugins/mindspec/workflows/ms-panel.js` exits `0` (advisory — recorded because `node v25.2.1` is present; NOT a CI gate, per spec Validation Proofs)
- [ ] `git show --name-only HEAD | /usr/bin/grep -qxF '.mindspec/domains/workflow/interfaces.md'` (doc-sync)
- [ ] `go build ./...` exits `0`
- [ ] **Manual e2e (spec Validation Proof) — requires the Claude Code runtime + live agents; NOT automatable in CI.** In a scratch repo (post-109/110 base) with `.mindspec/config.yaml` `runner: claude-code-workflow`, `go build -o /tmp/ms111 ./cmd/mindspec`, then invoke `/ms-panel` with a fixture mix `[{claude,2},{codex,1}]` against a throwaway `--target`, and confirm, in order:
  1. **Verdict + audit files:** 3 verdict files land at `<spec-dir>/reviews/<slug>/<slot>-round-1.json`, and the codex slot also leaves a `<slot>-round-1.codex.log` raw-stdout audit artifact in the panel dir (R3, R3b).
  2. **Parse-failure re-prompt (R3 steps 1–2), with VALUE fidelity (carry-forward #2):** a codex slot forced to render malformed JSON is re-prompted **once** to re-serialize *its own* verdict; the re-emitted verdict keeps the **same** `reviewer_id` (same slot, same family — e.g. `R4 codex`, never `claude-sub`) **and its decoded verdict value equals the rendered one** (diff the re-emitted JSON's `verdict`/`concrete_changes_required` against the `.codex.log` source — a re-serialize, not a re-review); a slot still unparseable after that single re-prompt leaves **no verdict file** (MISSING → `panel verify` incomplete → gate Blocks) and is **not** substituted.
  3. **Quota-wall substitution (R4):** a slot whose codex hits its usage limit *with no verdict rendered* appears as `reviewer_id: "<slot> claude-sub"` when `claude_sub_on_quota` is true, and as a missing verdict when false.
  4. **Result (R5):** the workflow **result** carries the `mindspec panel verify` report + `mindspec panel tally` preview **verbatim** (unmodified CLI stdout), with `mindspec complete` never invoked (confirm via the run transcript + `git log` showing no merge).

**Acceptance Criteria**
- [ ] The workflow exists at both tracked locations byte-identical; it accepts
  `{slug,spec,target,bead_id?,round,sha?,lenses[],mix}`, derives its mix only
  from `mix`, never uses `sha?` to set the recorded SHA, and never itself execs a
  CLI or writes a file (script vs agent) (spec AC1, R1)
- [ ] Registration is through `mindspec panel create` only — no hand-written
  `panel.json`, no re-implemented co-bump (R2)
- [ ] Each mix slot produces a verdict at `<slot>-round-<N>.json`; codex slots
  are wrapper agents that write both the verdict and the `.codex.log`, codex
  writing neither; a rendered-but-malformed verdict resolves only to the same
  reviewer's re-serialized verdict or a MISSING slot — never a substitution or a
  re-reviewed APPROVE (spec AC5, R3, R3b)
- [ ] Quota-wall substitution is a deterministic branch honoring
  `claude_sub_on_quota`, keeping the slot id + `claude-sub` `reviewer_id`; a
  `false` flag leaves the slot missing (R4)
- [ ] The workflow returns `panel verify` + `panel tally` output verbatim;
  `ALLOWED_CLI` admits **exactly** the four commands (enforced by
  `TestMsPanelWorkflow_AllowedCLIExactSet`, not just present/absent greps);
  `mindspec complete` appears nowhere; no consolidation / `consolidated-round-*.md`
  / `panel.json` mutation beyond `create` (spec AC4, R5)
- [ ] `WorkflowFiles()` embeds the workflow and `mindspec setup` (Claude target)
  installs it byte-identical to the embed while codex/copilot do not; the
  fresh-setup created-count consumers are updated in the same bead (spec AC6,
  AC7, R8)

**Depends on**
Bead 1

## Bead 3: skills — runner dispatch in `ms-panel-run` + the workflow-result note in `ms-panel-tally`, judgment sections retained

Delivers R6 and R7. Edited on the **post-110** skill structure (110's Bead 5
already slimmed step 0 to `mindspec panel create` and removed
`ms-panel-tally`'s decision-matrix table — see Risks/Sequencing): this bead adds
the runner branch and gates the mechanized launch prose behind it, without
deleting the skills-path mechanics or any judgment section. The four files are
the **workflow** domain (`plugins/mindspec/skills/**` + their byte-identical
`.claude/skills/**` mirrors — each pair edited identically); doc-sync lands in
`.mindspec/domains/workflow/runbook.md`. **Depends on Bead 2** — the runner
branch dispatches to a shipped `/ms-panel` workflow.

**Steps**
1. In `ms-panel-run/SKILL.md` (both the `plugins/…` file **and** its
   `.claude/skills/…` mirror, edited identically), add a **Runner dispatch**
   section near the top that reads the effective `runner:` value via `mindspec
   config show` (spec 109) and branches, **explicitly labelling each section
   workflow-path or skills-path**:
   - `claude-code-workflow` → **compose the slot lenses** (the retained judgment
     step, § Slot lens defaults) then invoke the `/ms-panel` workflow **once**
     with the resolved `{slug, spec, target, bead_id?, round, lenses[], mix}`;
   - `claude-code-skills` (the **default** until the workflow path is proven) →
     the existing manual launch path runs **unchanged**;
   - `external` → a documented out-of-scope **stub** (no adapter ships; the panel
     runs human/skills-path per ADR-0040 degraded modes).
   State that a host lacking workflow capability degrades to the skills path.
2. In `ms-panel-run/SKILL.md`, **gate behind the `claude-code-workflow` branch**
   (superseded-for-workflow-path, **not deleted**) the **Launch the panel**,
   **Codex failure detection (deterministic)**, **Working directory matters**,
   and the codex-specific **Anti-patterns**: label them as the
   `claude-code-skills` default-path mechanics, and state that on the workflow
   path they are superseded by the schema+wrapper mechanism (R3–R4) and the
   single `/ms-panel` invocation (R6) — do **not** re-state the bash detection
   for the workflow path. **Keep § Slot lens defaults intact** as the workflow
   path's sole authoring step. The grep AC requires the literal strings
   `claude-code-workflow`, `/ms-panel`, `Slot lens defaults`, and `Launch the
   panel` to all remain present.
3. In `ms-panel-tally/SKILL.md` (both files), add a note that on the workflow
   path the per-slot verdict table + decision arrive **pre-rendered in the
   workflow result** (an already-run `mindspec panel tally`), so the skill's job
   narrows to Step 4 consolidation + the merge terminal. **Leave unchanged** the
   judgment sections: **Consolidate** (semantic dedup + ranking, authoring
   `consolidated-round-<N>.md`), **§ Artifact gates**, **§ After a halt —
   recovery**, **§ Escape hatch**, and **§ Abandon procedure**. The grep AC
   requires the literal strings `workflow`, `## Artifact gates`, `Consolidate`,
   `After a halt`, and `Escape hatch` to all remain present.
4. Keep the two `.claude/skills/**` mirrors **byte-identical** to their
   `plugins/mindspec/skills/**` sources (they are today; a `diff -q` gate below
   pins it).
5. Doc-sync (workflow): add a bead-unique region to
   `.mindspec/domains/workflow/runbook.md` noting the panel operator procedure now
   selects the runner via the 109 `runner:` key — `claude-code-workflow` invokes
   the `/ms-panel` workflow once (registration + fan-out + verify/tally-return
   mechanized), `claude-code-skills` retains the hand-driven launch, `external`
   is a documented stub — with the judgment sections (Slot lens defaults,
   Consolidate, Artifact gates) retained in the skills.

**Verification**
- [ ] `R=plugins/mindspec/skills/ms-panel-run/SKILL.md; grep -q 'claude-code-workflow' "$R" && grep -q '/ms-panel' "$R" && grep -q 'Slot lens defaults' "$R" && grep -q 'Launch the panel' "$R"` exits `0` (spec AC8, R6/R7 — plugins copy)
- [ ] `T=plugins/mindspec/skills/ms-panel-tally/SKILL.md; grep -q 'workflow' "$T" && grep -q '## Artifact gates' "$T" && grep -q 'Consolidate' "$T" && grep -q 'After a halt' "$T" && grep -q 'Escape hatch' "$T"` exits `0` (spec AC9, R7 — plugins copy)
- [ ] `R=.claude/skills/ms-panel-run/SKILL.md; grep -q 'claude-code-workflow' "$R" && grep -q '/ms-panel' "$R" && grep -q 'Slot lens defaults' "$R" && grep -q 'Launch the panel' "$R"` exits `0` (the `.claude` mirror carries the same runner branch + retained sections)
- [ ] `T=.claude/skills/ms-panel-tally/SKILL.md; grep -q 'workflow' "$T" && grep -q '## Artifact gates' "$T" && grep -q 'Consolidate' "$T" && grep -q 'After a halt' "$T" && grep -q 'Escape hatch' "$T"` exits `0` (the `.claude` mirror)
- [ ] `diff -q plugins/mindspec/skills/ms-panel-run/SKILL.md .claude/skills/ms-panel-run/SKILL.md && diff -q plugins/mindspec/skills/ms-panel-tally/SKILL.md .claude/skills/ms-panel-tally/SKILL.md` exits `0` (the mirrors stay byte-identical — no drift between the two copies)
- [ ] `git show --name-only HEAD | /usr/bin/grep -qxF '.mindspec/domains/workflow/runbook.md'` (doc-sync)
- [ ] `go build ./...` exits `0` (no Go code touched — the tree still builds)

**Acceptance Criteria**
- [ ] `ms-panel-run` reads the runner value from config and branches:
  `claude-code-workflow` composes lenses then invokes `/ms-panel` once;
  `claude-code-skills` runs the existing launch **unchanged**; `external` is a
  documented stub — the mechanized launch prose is gated behind the runner branch
  (superseded for the workflow path), **not deleted**, and § Slot lens defaults
  and § Launch the panel both survive (spec AC8, R6, R7)
- [ ] `ms-panel-tally` notes the workflow-result pre-rendered path while all four
  named judgment sections (Consolidate, Artifact gates, After a halt — recovery,
  Escape hatch) survive; both `plugins` and `.claude` copies are edited
  identically and stay byte-identical (spec AC9, R7)

**Depends on**
Bead 2

## Risks / Sequencing

**Hard prerequisites: specs 109, 110, and 112 land on `main` before 111's
beads (spec Open Questions, resolved).** This plan is authored on a **post-109**
base and **assumes 110 and 112 have merged** before 111 enters Implementation
Mode — per the spec, `plan approve` for 111 may run only after both 109 and 110
have merged to `main` **and** `spec/111-workflow-panel-runner` has been
rebased/merged-forward onto that base (the spec-108→107 ordering pattern); the
standing model-tiering protocol adds 112 (per-gate `gates:` config) to the same
forward-rebase.

- **110 (`mindspec-fbel.1–.5`, plan-approved, not yet merged)** provides the
  `mindspec panel create | verify | tally` verbs + the verdict-file/slot schema
  the workflow consumes as an **unchanged CLI + artifact contract**, and 110's
  Bead 5 slims `ms-panel-run` step 0 (to `mindspec panel create`, removing the
  hand-typed `panel.json` schema and the skill-authored verdict-output block) and
  `ms-panel-tally` (removing the `| Condition | Action |` decision matrix). Bead 3
  branches **on top of that slimmed structure** — it adds the runner dispatch and
  gates the surviving launch mechanics behind it. **Rebase point:** if 111 is
  implemented before 110 merges, Bead 3's skill edits would target the pre-110
  (un-slimmed) skills; the forward-rebase resolves this so Bead 3 sees the
  post-110 sections named in its Steps.
- **112 (`mindspec-lma4.1–.3`, plan-approved, not yet merged)** adds the per-gate
  `gates:` map + `substitutes` + the recorded `panel.Panel.Gate` field. **111
  reads none of 112's new symbols**: the workflow consumes `runner:`,
  `panel:` mix, and `substitution.claude_sub_on_quota` (all 109 surfaces 112
  keeps), so a post-112 rebase touches no 111 line.
- **Mechanical-rebase safety.** 111's Go work is **additive on new files/symbols**:
  Bead 1 appends a test + one OWNERSHIP line + a doc region; Bead 2 adds two new
  `.js` files, a new `embed.go` var + accessor, a new `installWorkflows` call +
  its two count-consumer edits, and a new `plugins/mindspec` test file; Bead 3
  edits only skills + a doc. **No 111 bead edits a function 110 or 112 edits** —
  110 touches `internal/panel` / `cmd/mindspec` / `internal/instruct` /
  `internal/validate` (`ValidateSpec`), 112 touches `internal/config` /
  `internal/panel` / `internal/complete` / `cmd/mindspec`; 111's only overlap is
  the **file** `internal/validate/ownership_wave2_test.go` (110 does not touch it)
  and the workflow OWNERSHIP.yaml (additive line), so the forward-rebase is a
  clean fast-forward on every 111-owned path.

**No platform output-schema dependency (honest limit).** The Claude Code
workflows docs describe no structured-output enforcement, so Bead 2 does **not**
assume one: verdict conformance rests on the agent prompt + `mindspec panel
verify`'s deterministic parse-status report + the R3 same-reviewer
re-serialize-or-MISSING ladder. The `.js` extension is inferred from the
documented "JavaScript script" language; the `.claude/workflows/` location and
`/ms-panel` slash-command invocation are documented. If a future Claude Code
release changes the workflow file format, the adapter — not the contract — is
what updates (ADR-0040 portability).

**Spec-approve round-2 carry-forward #4 (`../../adr/` relative-link convention).**
Info-only, pre-existing since round 1, not a regression, known repo-wide. It
concerns anchored links inside `spec.md`'s `## ADR Touchpoints`; the plan writes
no such links (it cites ADRs by ID in frontmatter + prose), so there is **no
plan action** — recorded here for disposition completeness.

**Panel-substitution posture (orchestration note, not a plan constraint).** Per
the standing model-tiering protocol, the 9-reviewer spec/plan panels add 3×
Fable; when Codex is quota-walled, substitute Sonnet/Claude personas rather than
block.

## Provenance

Spec ACs are numbered in the order they appear in the spec's Acceptance Criteria
checklist. Every spec AC traces to a bead; every requirement R1–R9 is delivered
(R9 → Bead 1; R1–R5, R3b, R8 → Bead 2; R6, R7 → Bead 3).

| Acceptance Criterion (spec) | Verified By |
|---------------------------|-------------|
| AC1 — both tracked workflow copies present + byte-identical (`diff -q`) (R1, R8) | Bead 2 verification (`diff -q`; embed test [embed == plugin] + install test [installed == embed] make all four copies transitively identical) |
| AC2 — `.claude/workflows/**` claim present in workflow `OWNERSHIP.yaml` (R9) | Bead 1 verification (anchored `grep -Eq`) |
| AC3 — `TestWorkflowOwnsClaudeWorkflows`: `attributeDomain` → `"workflow"` for the workflow file (R9) | Bead 1 verification (PASS-line grep) |
| AC4 — workflow declares `ALLOWED_CLI` with the four commands present, honors `claude-sub`, and `mindspec complete` appears nowhere (R2, R4, R5) | Bead 2 verification (structural grep) **strengthened** by `TestMsPanelWorkflow_AllowedCLIExactSet` (exactly-four exact-set + no dynamic construction — carry-forwards #1/#3) |
| AC5 — workflow persists each codex slot's `<slot>-round-<N>.codex.log` (`grep -qF '.codex.log'`) (R3b) | Bead 2 verification (grep) |
| AC6 — `TestWorkflowFiles_EmbedsMsPanel`: `WorkflowFiles()` returns the embedded workflow (R8) | Bead 2 verification (PASS-line grep) |
| AC7 — `TestClaudeSetup_InstallsWorkflowClaudeTargetOnly`: Claude setup writes it byte-identical; codex/copilot do not (R8) | Bead 2 verification (PASS-line grep) |
| AC8 — `ms-panel-run` gains the runner branch + `/ms-panel`, retains § Slot lens defaults + § Launch the panel (R6, R7) | Bead 3 verification (plugins + `.claude` mirror greps) |
| AC9 — `ms-panel-tally` notes the workflow-result path; Consolidate / Artifact gates / After a halt / Escape hatch survive (R7) | Bead 3 verification (plugins + `.claude` mirror greps) |
| AC10 — tree builds + touched packages green (`go build ./... && go test ./internal/validate/... ./internal/setup/... ./plugins/mindspec/...`) | every bead's `go build ./...` + per-package `go test`; full `go test ./...` regression at plan time and pre-`/ms-impl-approve` (Testing Strategy) |
| Validation Proof — `node --check` on both copies (where a JS toolchain is present) | Bead 2 verification (advisory, `node v25.2.1` present; not a CI gate) |
| Validation Proof (Manual, live agents) — verdict + `.codex.log` files; parse-failure re-prompt with same `reviewer_id` **and value fidelity** (carry-forward #2), MISSING-not-substituted; quota-wall `claude-sub` vs missing per flag; result carries verify+tally verbatim, `mindspec complete` never invoked (R2–R5, R3b) | Bead 2 verification (Manual e2e, run in order) |
| Structural workflow proof — calls the verbs, never mutates the lifecycle | Bead 2 verification (AC4 grep + `TestMsPanelWorkflow_AllowedCLIExactSet`) |
