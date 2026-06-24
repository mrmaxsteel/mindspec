# v0.10.0

A large release. The headline is the **flattened `.mindspec/` layout**, but
v0.10.0 also lands ten specs' worth of lifecycle, gate, and CLI work merged
since v0.9.0 — the panel gate is now enforced inside `mindspec complete`, the
ownership/ADR gates resolve domains by file path, the `next`/`release` CLI
ergonomics are predictable, and a new `ms-spec-grill` skill hardens spec
authoring — plus a batch of correctness fixes. No migration is forced:
**upgrading the binary alone changes nothing on disk**, and the new lifecycle
gates engage only within the mindspec spec workflow (and only once you register
a panel), so existing layouts and non-panel usage keep working as before.

## Flattened `.mindspec/` layout

### Headline

`.mindspec/` is **flattened**: `specs/`, `adr/`, `domains/`, `core/`, and
`context-map.md` are now top-level children of `.mindspec/` — the
`.mindspec/docs/` wrapper is gone. Panel reviews are **co-located** under
`<spec-dir>/reviews/`. Repo dogfood documentation moved to a top-level
`project-docs/` tree. The vestigial `glossary.md` and `policies.yml` were
dropped.

### Non-breaking & opt-in

**Existing projects keep working with NO action required.** A multi-tier
resolver (flat → canonical → legacy) reads every layout, first-exists-wins, so
pre-flatten checkouts resolve exactly as before. Writes stay in your project's
current layout until you explicitly opt in — **upgrading the binary alone
changes nothing on disk.**

### Migrating an existing project (optional)

```bash
mindspec migrate layout
```

A transactional mover does **two commits per move** (a pure `git mv`, then a
link-rewrite), so history stays clean and bisectable.

**Preconditions:** a clean working tree and no unmerged pre-flatten branch. If an
unrelated stale pre-flatten branch trips the precondition, exempt it with
`--allow-branch <name>` (repeatable) or bypass the scan with `--force` (logged).

`mindspec migrate layout --abort` rolls back a **pre-publish** run; once the run
is published the flatten is **forward-only**. Run `mindspec doctor` afterward to
confirm links resolve.

### Also in the flatten

- **Directional cross-layout merge guard** — hard-fails the regression
  direction so a flattened branch can't be silently un-flattened.
- **Layout-aware panel gate** — the `complete` gate scans the honored review
  location(s) for the tree's detected layout.
- **`doctor` layout detection** — reports the detected docs layout and flags a
  tree that would flatten on the next migrate.
- **`migrate layout` hardening** — precondition scoping and a wider link-check.

## Workflow & lifecycle

- **Panel gate enforced inside `mindspec complete`** — the ADR-0037 review-panel
  gate now runs *inside* `mindspec complete <id>`, keyed off the bead ID it
  already knows, so it covers every invocation form (wrapped, quoted, aliased)
  with no shell-string guessing. The old PreToolUse hook stays only as a
  defense-in-depth backstop. A bead with no registered panel still completes
  silently — the gate fails closed only once a panel exists.
- **Ownership/ADR gates resolve domains by file path** — `## Impacted Domains`
  entries that are file paths (not bare domain names) now resolve to their owning
  domain by glob-matching the `OWNERSHIP.yaml` manifests, at one shared source
  consumed by the bead-time divergence gate *and* the plan-time coverage/citation
  gates. A correct manifest stops being rejected and stops forcing
  `--override-adr` on every bead; zero-owner and multi-owner cases still error
  clearly. Plan-validate ergonomics also improved: the `adr-coverage-missing`
  hint now mentions adding a `Domain(s)` to an existing ADR, `adr show`/`adr list`
  are worktree-aware, and the plan scaffold emits the `adr_citations` key.
- **Lifecycle CLI ergonomics** —
  - `mindspec next <bead-id>` now claims the **named** bead (or fails loudly),
    instead of silently claiming the first ready item; short-form IDs (`xxxx`)
    resolve as well as the full `project-xxxx` form.
  - A new `mindspec release <bead>` verb cleanly reverses a wrong claim (remove
    worktree first, then re-open the bead), refusing a dirty worktree unless
    `--force`.
  - `mindspec spec create` branches from `origin/<default-branch>` after a fetch
    (default branch detected, not hardcoded), falling back to local `HEAD` with a
    WARN when offline.
  - Claim failures surface bd's real stderr (so a stale-binary schema error is
    legible), and `mindspec doctor` gained bd schema-drift and
    multiple-`bd`-on-PATH checks.

## Spec authoring

- **`ms-spec-grill` skill** — a new agent skill that interrogates a draft spec
  one question at a time, grounded live in the real domains, ADRs, and code tree,
  to drive out vague language, synonym-dodges ("support/improve X"),
  non-falsifiable claims, cross-requirement contradictions, and unprobed edge
  cases. It ships as a plugin skill (installed by `mindspec setup`), is backed by
  a tracked benchmark eval under `bench/grill/`, and `ms-spec-create`
  auto-chains into it by default.

## Correctness & hardening

- **`mindspec complete` close-verify** — after `bd close`, `complete` forces a
  Dolt commit and verifies the bead actually persisted as closed; it can no
  longer print `closed` + exit 0 on a close that did not land, and keeps the
  worktree on failure so the run is re-runnable.
- **`bd close` bypass guard** — a lifecycle floor detects and blocks using
  `bd close` to skip the panel/merge gates, steering callers to
  `mindspec complete`.
- **Fresh-repo merge safety** — bootstrap now provisions the beads JSONL merge
  driver (portable, cross-worktree path) from commit 0, so a fresh clone is no
  longer one bad `.beads/issues.jsonl` merge away from corruption.
- **ADR numbering & worktree correctness** — `adr.NextID` parses slugged
  `ADR-NNNN-slug.md` filenames (no more colliding low IDs), and `adr create`
  writes into the invoking worktree instead of the main checkout.
- **`mindspec version` subcommand** — `mindspec version` now works and matches
  `--version` byte-for-byte.
- **Plan-approve loosening** — a plan missing only `version` is auto-filled to
  `"1"` instead of being hard-blocked (`status`/`spec_id` stay required).
- **Git argument safety** — `internal/gitutil` rejects hostile `-`-prefixed
  ref/branch operands at its own boundary (defense-in-depth) and fast-fails with
  `GIT_TERMINAL_PROMPT=0` so a slow/auth-prompting origin can't hang
  `spec create`.
- **Phase-cache breadth** — the phase cache now counts `blocked` and custom-status
  children, matching the state-advance path so derived phases don't skew.

## Internal / under the hood

- **ZFC cleanup** — retired the PreToolUse heuristic complete-matcher (now
  redundant with the in-binary gate) and the harness analyzer's prose
  path-scraper; spec 097 also moved bead dependencies, ADR citations, and key
  file paths onto declared plan frontmatter instead of prose regexes, and merged
  the stale `speclist` package into `spec`.
- **Harness audit + `MS_HARNESS_MODEL`** — a Sonnet full-suite audit fixed three
  LLM-test failures, added the `MS_HARNESS_MODEL` override, and cut a slow
  scenario's turn count.
- **Spec-orchestrator agent** — added a `spec-orchestrator` agent definition under
  `.claude/agents/` for multi-bead/multi-spec autonomous runs.

## Governance

- **ADR-0039** (Flat `.mindspec/` Layout v2) — **Accepted**.
- **ADR-0037** (panel gate) amended — in-binary enforcement is now authoritative,
  the heuristic matcher retired, and the review location moved to
  `<spec-dir>/reviews/`.
- **ADR-0032** (ADR semantic gates) amended — path-like Impacted-Domains entries
  are normalized to their owning domain rather than rejected outright.
- **DOCS-LAYOUT.md** amended to the flat layout.

## Known issues

- On a **branch-protected** `main`, `mindspec impl approve` can momentarily leave
  the bead tracker's committed `.beads/issues.jsonl` out of sync with the
  source-of-truth Dolt store, because the lifecycle's finalize commit cannot land
  directly on protected `main`. A one-time manual `.beads` sync resolves it (the
  post-merge hook is then idempotent). Tracked as `mindspec-wu7t`. Normal feature
  work and non-protected repositories are unaffected.
