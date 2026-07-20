# ADR-0043: Panel Disposition Telemetry Store — Per-Panel JSONL + Coverage Manifest + Go-Verb Append Contract

- **Date**: 2026-07-20
- **Status**: Accepted
- **Domain(s)**: workflow
- **Deciders**: Max
- **Supersedes**: n/a
- **Superseded-by**: n/a
- **Related**: [ADR-0037](ADR-0037-panel-gate-enforced-contract.md) (the disposition rows/manifest live inside §8's agent-writable trust boundary and the gate's decision matrix consumes none of them — telemetry, decision-inert, not tamper-proofing), [ADR-0023](ADR-0023.md) (beads remain the single lifecycle-state authority; this store is an analysis dataset that POINTS at follow-up beads via `evidence_ref`, never a second state authority), [ADR-0025](ADR-0025-jsonl-as-build-artifact.md) (JSONL-as-projection precedent — this store is a git-versioned JSONL projection of orchestrator judgment, not a source of gate truth), [ADR-0040](ADR-0040-orchestration-layering-ratchet.md) (disposition JUDGMENT stays at the `/ms-panel-tally` skill layer; the binary contributes only mechanism), [ADR-0041](ADR-0041-gate-before-mutate.md) (the append op's validate-then-mutate ordering is this contract's preflight discipline), [ADR-0042](ADR-0042-render-derivation-provenance.md) (the store's agent-writable free text is untrusted-provenance and every render routes through the shipped termsafe/idrender sinks)

---

## Context

MindSpec runs multi-model review panels at every lifecycle gate (spec/plan
≈ 9–12 slots, bead = 8, final = 12), all driven to unanimous approve under the
findings-never-out-voted doctrine. Each panel produces per-reviewer verdict
JSONs, and `/ms-panel-tally` — the single decision authority — resolves every
finding (fixed, deferred, refuted, or exposed as contamination). Today that
resolution evaporates: the verdict JSONs record what a reviewer *claimed*, but
the orchestrator-decided `disposition` (was it genuine?) and `convergent_with`
(which other lenses independently raised it?) are captured NOWHERE structured.
Raw finding counts are actively misleading without that layer — they count
noise, duplicates, and contamination as genuine. Spec 117 makes the disposition
layer a durable, queryable dataset. This ADR records WHERE that store lives and
HOW it is written — the fork the spec-approve panel resolved (OQ1).

Two hard constraints shaped the decision, both repo-verified:

1. **Panels are tallied INSIDE spec worktrees.** Worktrees do not share a live
   Dolt database with main (`.beads/config.yaml`'s own `export.git-add` comment),
   and the store must survive worktree teardown WITHOUT relying on orchestrator
   diligence — spec 116's eight panels left NOTHING in the repo and were rescued
   only by a hand-built sibling archive.
2. **Both load-bearing query properties depend on per-panel slot metadata.** The
   R1(b) completeness floor needs the set of REQUEST_CHANGES/REJECT slots per
   panel; Q4's yield needs the slot COUNT per panel. That metadata lives only in
   the raw verdict files, which the spec deliberately does NOT commit (they embed
   machine-local absolute paths). So a durable store could survive while the
   evidence to prove its own floor / compute Q4 disappeared.

The public-repo posture is fixed either way: the repo and bd's Dolt remote are
public, and disposition rows are internal process data about MindSpec's own
development (code paths, SHAs, bead IDs, reviewer/model names) — never client or
private content.

## Decision

**The store is append-only JSONL, one file per panel**, at
`.mindspec/specs/<spec>/reviews/<panel>/dispositions.jsonl`, git-versioned and
riding the same lifecycle auto-commits that already persist spec artifacts.
Cross-panel / cross-spec reads glob `.mindspec/specs/*/reviews/*/dispositions.jsonl`;
"queryable" means glob + jq plus a small machine-checkable Go surface — NOT SQL.

Each per-panel file carries two record kinds, distinguished by a `record`
discriminator (`record ∈ {"disposition", "panel"}`), position-independent:

- **`record: "disposition"`** — one row per distinct finding:
  `{record, id, spec, gate, panel, reviewer, model, severity, summary,
  convergent_with[], disposition, evidence_ref?, note?, created_at, round?,
  backfilled}`. `gate ∈ config.PanelGateKeys`; `disposition` is a closed enum;
  `reviewer` and every `convergent_with[]` entry byte-match a verdict-file slot
  token (the slot-identity contract — verdict filenames are the sole slot
  identity); `id` is a stable content-derived key.
- **`record: "panel"` (coverage manifest)** — one per terminal panel:
  `{record, spec, gate, panel, round, slots: [{slot, model, verdict}], backfilled}`,
  where `slots` enumerates EVERY verdict-file slot (token, model, terminal
  `verdict ∈ {APPROVE, REQUEST_CHANGES, REJECT}`) and the slot count is the array
  length. This makes the completeness floor AND Q4's denominator derivable from
  the durable store ALONE — no raw-verdict commit needed. Every terminal panel
  writes its manifest regardless of finding count (a finding-less all-APPROVE
  panel still produces a manifest-only file), else that gate's Q4 denominator
  undercounts.

**One file per panel** (not a single per-spec file) because a per-spec
`DISPOSITIONS.jsonl` is merge-safe only CROSS-spec: parallel bead worktrees/panels
WITHIN one spec would all append to the same EOF and git-conflict, or force a
hand-merge that silently drops rows. One writer per panel file is merge-safe
within a spec.

**Writes go through ONE canonical Go verb**, `mindspec panel disposition`
(leaves `validate` / `append` / `check` / `query`), invoked by the
`/ms-panel-tally` skill. Disposition rows are appended INCREMENTALLY as findings
resolve across the tally→fix→re-panel loop; the manifest is appended ONCE at
terminal state. The serialization mechanism is the shipped **`internal/journal`
dedicated-lockfile idiom** (spec 094): the op holds a cross-process advisory lock
on a SEPARATE `dispositions.lock` file — NEVER on `dispositions.jsonl` itself — via
the build-tagged `acquireFileLock` (unix `syscall.Flock` `LOCK_EX`, blocking;
windows `O_EXCL`-lockfile with bounded retry). Locking a separate file means lock
acquisition never opens/creates the data file before validation, and the manifest
path never needs an in-place "update". Under that lock, each write performs, as an
indivisible unit: (a) R2-schema + path-hygiene validation of the record, BEFORE
touching the data file, (b) a read of the current file + the uniqueness/idempotency
check on the stable key (a row on its `id`; a manifest on `{spec, panel, round}`),
and (c) the mutation — a row by atomic `O_APPEND`; the manifest **no-op-if-exists**
(its terminal content is deterministic per `{spec, panel, round}`, so a present
manifest is left as-is, never rewritten). Consequences guaranteed: concurrent
DISTINCT records both persist with no loss and no interleaved partial line;
concurrent DUPLICATES collapse to exactly one; any validation/hygiene refusal exits
non-zero and leaves the data file BYTE-UNCHANGED (gate-before-mutate, ADR-0041).
Because the lock is build-tagged, a `GOOS=windows go build ./...` smoke gates every
PR so the release cross-compile cannot break invisibly.

**The mechanism is a Go verb, not a tracked jq/bash script.** A Go verb — and
only a Go verb — inherits the five mechanisms the spec mandates: the ADR-0042
safe-render sinks (`internal/termsafe.Escape` + `internal/idvalidate/idrender`)
for agent-writable `summary`/`note`/`reviewer` text; the `internal/lint`
render-ratchet that mechanically flags raw renders; the ADR-0041 gate-before-mutate
ordering; the transactional-write / atomicity contract (real file-locking,
`-race`-testable); and CI-runnable unit tests pinning the acceptance numbers and
the exhaustive validator negative-fixture matrix. A script has none of these.

Disposition rows and manifests are decision-inert with respect to the panel gate
(the ADR-0037 decision matrix consumes none of them). No lifecycle verb reads the
store to make a decision (ADR-0023); the store is telemetry, audited-not-
authenticated exactly like the verdict files it summarizes.

## Consequences

- The completeness floor and Q4 are provable / computable after any worktree
  teardown, from committed JSONL alone — the spec-116 loss mode is closed without
  committing raw verdicts.
- The `/ms-panel-tally` skill gains a mandated capture step (both skill surfaces,
  kept byte-identical) but no new operator-facing manual step outside its own flow.
- The spec-116 seed (21 disposition rows across 8 panels + 8 synthesized coverage
  manifests) migrates in as the first data points and the schema-validation
  fixture, marked `backfilled: true` so migration provenance is distinguishable
  from live capture.
- No change to `internal/panel/gate.go`, the decision matrix, thresholds, hatches,
  or `panel.json`; no `mindspec complete` / `impl approve` gate behavior changes
  (a completeness advisory at the gate is a deliberately separate future decision).
- bd/beads core schema and verbs are untouched — the store is entirely outside
  bd's Dolt DB.

## Alternatives Considered

- **Dolt table in bd's embedded DB (REJECTED).** SQL-queryable and synced via
  `bd dolt push`, but bd's schema is upstream-owned with no custom-table verb
  (raw `dolt sql` against `.beads/embeddeddolt` contends with the shared server),
  and — decisively — worktrees do not share a live Dolt DB with main while panels
  are tallied inside worktrees, making the write path structurally hostile (the
  spec-116 loss mode).
- **Both — JSONL write-time source ingested into a SQL surface (REJECTED).** Buys
  SQL only at the cost of an ingest step that can drift; the Q1–Q5 surface is
  cheaply served by jq + a small Go surface over the JSONL without it.
- **A single per-spec `DISPOSITIONS.jsonl` (REJECTED as layout).** Merge-safe only
  cross-spec; parallel same-spec panels conflict at the shared EOF. Sharding to one
  file per panel removes the conflict.
- **A tracked jq/bash script as the write/query mechanism (REJECTED).** Forgoes the
  safe-render sinks, the render ratchet, the gate-before-mutate ordering, the
  transactional atomicity contract, and CI-runnable regression tests — every one of
  which the spec mandates. A Go verb inherits all five.
