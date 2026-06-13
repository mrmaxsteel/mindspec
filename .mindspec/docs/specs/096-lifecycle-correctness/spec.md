---
approved_at: "2026-06-13T11:12:44Z"
approved_by: user
drafted_at: "2026-06-13"
drafted_by: spec-drafting research agent
roadmap_step: mindspec-jkhd.2
source_beads: mindspec-oe0u, mindspec-2u0u, mindspec-bn3u, mindspec-8lzq, mindspec-2b4n
status: Approved
---
# Spec 096-lifecycle-correctness: Lifecycle correctness — merge-driver provisioning, close-leg verification, ADR worktree/numbering, version subcommand

## Goal

Close the remaining VERIFIED-OPEN lifecycle-correctness bugs that survived spec 092's
fixes: (1) fresh repos are never provisioned with the beads jsonl merge driver, so the very
corruption class spec 092 documented re-arms on every new clone (**mindspec-oe0u**); (2)
`mindspec complete`'s close step can report `closed` + exit 0 while the Dolt close did not
persist, violating the spec-092 "exit codes never lie" invariant (**mindspec-2u0u**); (3)
`adr.NextID` skips every slugged ADR filename and returns a colliding ID (**mindspec-bn3u**);
(4) `mindspec adr create` resolves a bead/spec worktree back to the MAIN checkout and writes
the new ADR outside the invoking worktree (**mindspec-8lzq**); and (5) `mindspec version`
errors because only the `--version` flag exists (**mindspec-2b4n**). After this spec, a fresh
clone is merge-driver-safe from commit 0, `complete` never reports a close that did not
persist, ADR creation and numbering are worktree-correct and collision-free, and both
`version` forms work.

This spec deliberately re-verified each candidate against the spec/096 branch (= current
`main`) BEFORE writing a requirement — a prior 096 draft discovered that 5 of its 6
candidates had already been fixed by spec 092. All five requirements below were confirmed
STILL OPEN by reading the actual cited code; none is a re-implementation of a landed fix.

## Background

Each candidate was grounded against the CURRENT `main` (= spec/096 branch). The verify result
is recorded inline.

- **mindspec-oe0u (P1) — VERIFIED OPEN.** The beads merge driver (`.gitattributes merge=beads`
  on `.beads/issues.jsonl` + the `merge.beads.driver` git config + the
  `scripts/bd-jsonl-merge-driver.sh` wrapper) is never provisioned for a fresh repo.
  `grep -rn "merge.beads.driver|merge=beads|bd-jsonl-merge" internal/bootstrap/` returns ZERO
  hits — bootstrap writes nothing. The only code that knows about the driver is the doctor lane
  `checkBeadsMergeDriver` (`internal/doctor/beads.go:355`), and its `--fix` writes a
  NON-PORTABLE absolute path: `wantDriver := "'" + scriptAbs + "' %A %O %B"`
  (`internal/doctor/beads.go:364`, where `scriptAbs` is the absolute path to the wrapper). A
  fresh clone therefore has no driver until someone runs `mindspec doctor --fix`, and even then
  the config bakes in a machine-specific absolute path. This is the exact incident from
  spec-092 Bead 2 (2026-06-11): a both-sides-changed `.beads/issues.jsonl` merge fails, the
  bead is left CLOSED-but-unmerged. The doctor detection lane (`checkBeadsMergeDriver` — the
  `bd merge`-removed class at `internal/doctor/beads.go:435`, the missing-driver class at
  `:374`, and the inverse missing-attribute class at `:404`) already exists and is
  regression-locked here, not re-implemented. The GitHub-PR-merge residual (PR merges on GitHub
  never run local merge drivers; the post-merge beads-sync pattern compensates) is documented,
  not fixed here.

- **mindspec-2u0u (P2) — VERIFIED OPEN.** `complete.Run`'s close step swallows a silent
  close-leg loss. At `internal/complete/complete.go:348-357` the code calls `closeBeadFn(beadID)`
  and only re-reads the bead status when that call returns a non-nil error (the already-closed
  tolerance path at `:351-353`). When `closeBeadFn` returns nil but the close did NOT persist
  (Dolt close lost / raced — the spec-092 Bead 7 symptom: prints `closed`, exits 0, but
  `bd show` later reports `in_progress` with `closed_at None`), nothing verifies the persisted
  status: `Result{BeadClosed: true}` is set unconditionally at `:370`. The existing
  CLOSED-but-unmerged guard at `complete.go:418` is a DIFFERENT check — it fires on a post-close
  MERGE failure, not on a close that silently never landed. The silent close-loss path is
  unaddressed.

- **mindspec-bn3u (P2) — VERIFIED OPEN.** `internal/adr/parse.go::NextID` (now at `:230`; the
  bead cited the old `:165`) parses the WHOLE `NNNN-slug` suffix. At
  `internal/adr/parse.go:246-248` it does `name := strings.TrimSuffix(base, ".md")` →
  `numStr := strings.TrimPrefix(name, "ADR-")` → `strconv.Atoi(numStr)`. For a slugged filename
  `ADR-0025-jsonl-as-build-artifact.md`, `numStr` is `"0025-jsonl-as-build-artifact"`, `Atoi`
  fails, and the file is skipped via `continue` at `:250`. In a repo whose ADRs are all slugged
  (the live convention — every ADR file is `ADR-NNNN-slug.md`), every file is skipped, `maxNum`
  stays 0, and `NextID` returns a colliding low ID (verified empirically 2026-06-11 during
  spec-092 Bead 1).

- **mindspec-8lzq (P2) — VERIFIED OPEN.** `mindspec adr create` writes to the MAIN checkout,
  not the invoking worktree. At `cmd/mindspec/adr.go:28` the create command resolves
  `root, err := workspace.FindRoot(cwd)`; `FindRoot` deliberately resolves a worktree back to
  the main repo (`internal/workspace/workspace.go:20`, the `resolveWorktreeRoot` branch at
  `:28-31`, which returns the main root at `:83-85`). It then builds
  `store := adr.NewFileStore(root)` (`cmd/mindspec/adr.go:55`) and `store.Create(...)` writes
  into `<main>/.mindspec/docs/adr/`. The OverlayStore added since
  (`internal/adr/overlaystore.go`) only fixes the READ lanes — its `List`/`Get`/`Search` union
  the branch store over the primary for validate. Although `OverlayStore.Create` routes to the
  branch store (`internal/adr/overlaystore.go:68`), the create CLI command never constructs an
  OverlayStore; it still uses `NewFileStore(FindRoot(cwd))`. So the CREATE/WRITE path is
  unfixed: a new ADR authored from a bead/spec worktree lands in main's tree (verified
  empirically 2026-06-11 during spec-092 Bead 1 — the implementer had to hand-write ADR-0035).

- **mindspec-2b4n (P3) — VERIFIED OPEN.** There is no `version` subcommand. `cmd/mindspec/` has
  no version command file; the only version surface is the cobra `Version:` field on the root
  command (`cmd/mindspec/root.go:57`), which wires the `--version` flag. `mindspec version`
  errors with `unknown command "version" for "mindspec"`, even though install/instruct/docs
  references reach for the subcommand form.

## Impacted Domains

- **workflow**: every owned path touched by this spec is in the workflow domain
  (`.mindspec/docs/domains/workflow/OWNERSHIP.yaml`):
  - `internal/bootstrap/**` — provision the merge driver for fresh repos (Req 1).
  - `internal/doctor/**` + `scripts/bd-jsonl-merge-driver.sh` — regression-lock the existing
    driver detection lane and the wrapper (Req 1).
  - `internal/complete/**` — verify the close-leg persisted (Req 2).
  - `cmd/**` — worktree-aware `adr create` root (Req 4) and the new `version` subcommand
    (Req 5).

`internal/adr/parse.go` (Req 3) is NOT claimed by any domain `OWNERSHIP.yaml` — it is unowned
code, so it carries no doc-sync / ADR-divergence attribution requirement of its own; the fix
is a self-contained parse correction. **workflow** is therefore the sole impacted domain.

## ADR Touchpoints

- [ADR-0025](../../adr/ADR-0025-jsonl-as-build-artifact.md): jsonl-as-build-artifact
  (Status: **Accepted**; Domain(s): workflow, execution, bootstrap). Req 1's merge driver
  regenerates `.beads/issues.jsonl` from the canonical Dolt DB on merge — the wrapper
  provisioning is a direct application of this ADR's "the jsonl is a deterministic projection;
  regenerate-from-DB is the correct merge" decision. Covers the **workflow** domain.
- [ADR-0035](../../adr/ADR-0035-agent-error-contract.md): agent error/recovery contract
  (Status: **Accepted**; Domain(s): workflow, execution, core). Req 1's doctor failures and
  Req 2's hard close-loss error follow its recovery-line convention and the spec-092
  "exit codes never lie" invariant. Covers the **workflow** domain.

No new ADR is required: Req 1 applies ADR-0025's existing merge decision to the bootstrap
provisioning path; Reqs 2-5 are correctness fixes within the existing contracts. (Recorded as
a Design Question below, not a blocking open question.)

## Requirements

1. **(oe0u) Bootstrap provisions the beads jsonl merge driver with a portable, cross-worktree-safe path.**
   `internal/bootstrap` MUST provision, for a fresh repo: (a) the `.gitattributes` entry
   mapping `.beads/issues.jsonl` to `merge=beads`; (b) the `merge.beads.driver` git config
   pointing at the tracked `scripts/bd-jsonl-merge-driver.sh` wrapper with a PORTABLE path (not
   a machine-specific absolute path — e.g. a path resolved against the git top-level), so the
   config is valid across clones and across linked worktrees that share the common
   `.git/config`; and (c) the wrapper script itself shipped/tracked so the repo is covered from
   commit 0. The provisioned config MUST be accepted by the existing `checkBeadsMergeDriver`
   doctor lane. The doctor detection lane (the `bd merge`-removed class, the missing-driver
   class, and the inverse missing-attribute class) MUST be regression-locked with tests but
   MUST NOT change behavior. The GitHub-PR-merge residual is documented (a comment / the spec
   record), not fixed.

2. **(2u0u) `complete` verifies the close persisted and surfaces a hard error on failure.**
   `complete.Run` MUST, after the close step, VERIFY the bead's persisted status (re-read via
   the bead fetcher) and confirm it is `closed` before reporting success and proceeding to
   merge/cleanup. If the close did not persist (status is still `open`/`in_progress` after a
   nominally-successful `closeBeadFn` call), `complete` MUST surface a hard error and a non-zero
   exit — never print `closed` + exit 0 on an unpersisted close ("exit codes never lie", the
   spec-092 invariant). The existing already-closed tolerance (a true prior close is still
   accepted) and the distinct post-merge CLOSED-but-unmerged guard MUST be preserved.

3. **(bn3u) `adr.NextID` parses the leading integer of slugged ADR filenames.**
   `internal/adr/parse.go::NextID` MUST extract the leading numeric run of an `ADR-NNNN-slug.md`
   filename (the digits before the first hyphen following `ADR-`), so slugged ADR files COUNT
   toward `maxNum`. After the fix, `NextID` over a directory whose ADRs are all slugged (e.g. up
   to `ADR-0035-...`) returns the next free ID (`0036`), never a colliding lower ID. Both legacy
   bare `ADR-NNNN.md` and slugged `ADR-NNNN-slug.md` forms parse.

4. **(8lzq) `adr create` writes into the invoking worktree, not the main checkout.**
   `mindspec adr create` MUST author the new ADR file into the worktree it was invoked from —
   resolving a worktree-LOCAL root (not `workspace.FindRoot`, which resolves a worktree back to
   the main repo). Running `adr create` from a bead/spec worktree MUST write
   `<that-worktree>/.mindspec/docs/adr/ADR-NNNN-*.md`, and the file MUST NOT appear in the main
   checkout's ADR directory. NextID numbering (Req 3) MUST be computed against the same
   worktree-local root so the new ADR does not collide with branch-side ADRs.

5. **(2b4n) Add a `version` subcommand aliasing the `--version` output.**
   A `mindspec version` cobra subcommand MUST exist and emit the SAME version string the
   `--version` flag produces (the decorated `version + (commit) + date` form). Both
   `mindspec version` and `mindspec --version` MUST succeed and agree.

## Scope

### In Scope
- `internal/bootstrap/*` — write `.gitattributes merge=beads`, the portable `merge.beads.driver`
  config, and ensure the wrapper script is tracked, for a fresh repo (Req 1).
- `internal/doctor/beads.go` — regression tests for `checkBeadsMergeDriver`; accept the portable
  driver value bootstrap writes (Req 1). No behavior change to detection.
- `scripts/bd-jsonl-merge-driver.sh` — the tracked wrapper (ensure shipped; Req 1).
- `internal/complete/complete.go` — post-close persisted-status verification + hard error (Req 2).
- `internal/adr/parse.go` — leading-integer `NextID` parse (Req 3).
- `cmd/mindspec/adr.go` — worktree-local root for `adr create` (Req 4).
- `cmd/mindspec/` — new `version` subcommand (Req 5).

### Out of Scope
- The ADR READ overlay lanes (`internal/adr/overlaystore.go` validate-side union) — already
  landed; Req 4 fixes only the CREATE/WRITE path.
- The per-bead / whole-branch ownership-attribution gates and phase derivation / doc-sync
  ref-anchoring — landed in spec 095; unrelated.
- The merge-before-close ordering question for `complete` — the close-before-merge contract is
  settled (the closed-but-unmerged window at `complete.go:418` is explicit + reconvergent) and
  is not reopened; Req 2 only adds persisted-status verification of the close itself.

## Non-Goals

- Fixing GitHub PR merges of `.beads/issues.jsonl` (PR merges never run local merge drivers).
  The post-merge beads-sync pattern compensates; this residual is documented, not fixed (Req 1).
- Re-implementing the `bd merge`-removed doctor detection lane — it already exists; Req 1 only
  regression-locks it and makes bootstrap provision a config that satisfies it.
- A broad `internal/adr` worktree-store refactor — Req 4 makes the create command worktree-local
  by root resolution; it does not re-architect the store.
- Any change to the canonical Dolt DB or the jsonl projection format (ADR-0025 preserved).
- `mindspec-bk5t` (the external `bd update --parent ""` reverse-index bug) — lives in the `bd`
  tool, not the mindspec codebase.

## Acceptance Criteria

- [ ] A freshly bootstrapped repo has `.gitattributes` mapping `.beads/issues.jsonl` to
      `merge=beads`, a `merge.beads.driver` config pointing at the tracked wrapper via a PORTABLE
      path, and the wrapper present — and `checkBeadsMergeDriver` reports OK with no `--fix`. RED
      on revert: drop the bootstrap provisioning and a fresh-repo merge of a both-sides-changed
      jsonl fails / doctor flags a missing driver.
- [ ] The provisioned driver config is valid from a linked worktree (cross-worktree safe) — no
      machine-specific absolute path baked in. RED on revert: a value valid from only one
      checkout fails the cross-worktree test.
- [ ] `mindspec complete` exits NON-ZERO with a clear error when the close step reports success
      but the bead's persisted status is not `closed`; it never prints `closed` + exit 0 on an
      unpersisted close. RED on revert: restore the unconditional `BeadClosed: true` and a
      simulated silent close-loss reports success.
- [ ] A true already-closed bead still completes (merge + cleanup) without spurious failure.
- [ ] `adr.NextID` over a directory of slugged ADRs up to `ADR-0035-*` returns `0036` (not a
      colliding lower ID). RED on revert: restore the whole-suffix `Atoi` and `NextID` returns a
      collision.
- [ ] `mindspec adr create` run from a bead/spec worktree writes the ADR into THAT worktree's
      `.mindspec/docs/adr/`, and the file does NOT appear in the main checkout. RED on revert:
      restore `FindRoot` and the file lands in main.
- [ ] `mindspec version` succeeds and prints the same string as `mindspec --version`. RED on
      revert: remove the subcommand and `mindspec version` errors `unknown command`.
- [ ] `go build ./...` + `go test -short -race ./...` green; golangci-lint (the CI Lint job)
      clean (American spelling; no new gosec).

## Validation Proofs

- `go test ./internal/bootstrap/...`: a freshly bootstrapped repo provisions the
  `.gitattributes` attribute, the portable `merge.beads.driver` config, and the wrapper; the
  value satisfies `checkBeadsMergeDriver`; omitting any of the three steps fails the test.
- `go test ./internal/doctor/...`: regression tests pin the three `checkBeadsMergeDriver` classes
  (orphaned `bd merge`, missing driver, missing attribute) and accept the bootstrap-written
  portable value; RED if detection is loosened.
- `go test ./internal/complete/...`: a simulated successful-but-unpersisted close yields a
  non-zero error from `complete.Run`; a genuine already-closed bead still completes.
- `go test ./internal/adr/...`: `NextID` over slugged ADR fixtures up to `ADR-0035-*` returns
  `0036`; bare and slugged forms both parse.
- `go test ./cmd/mindspec/...`: `adr create` invoked with a worktree-local root writes into the
  worktree, not main; `mindspec version` emits the `--version` string.
- End-to-end: a both-sides-changed `.beads/issues.jsonl` merge in a freshly bootstrapped repo
  resolves cleanly via the provisioned driver (regenerate-from-DB), no unmerged stages.

## Design Questions (for the panel)

None blocking approval. Refined at planning / by the implementation panel:

- The exact PORTABLE form for `merge.beads.driver` (Req 1): a git top-level-relative path
  resolved at merge time, a `%(prefix)`-style value, or a relative path the driver resolves
  itself. Draft position: write a value that resolves the wrapper against the git top-level so it
  is valid across clones and linked worktrees, and that `checkBeadsMergeDriver`'s
  `resolveDriverCommand` (which already resolves relative paths against the worktree top-level)
  accepts unchanged; have `doctor --fix` converge the existing absolute value to it.
- Bootstrap idempotency / brownfield (Req 1): whether bootstrap WRITES the config unconditionally
  or only when absent, and whether it ever rewrites a user-set driver. Draft position:
  ensure-if-absent (never clobber a user-authored driver); brownfield repos get the doctor lane,
  not a forced bootstrap rewrite.
- The worktree-local root primitive for Req 4: reuse `workspace.FindLocalRoot` (which, unlike
  `FindRoot`, does NOT resolve a worktree back to main) vs a dedicated helper. Draft position:
  `FindLocalRoot(cwd)` for the create command only, leaving the read/list commands on `FindRoot`
  unless the panel finds a READ regression.
- Whether Req 2's persisted-status re-read should reuse the existing `fetchBeadByIDFn` path
  (already used in the error branch at `complete.go:351`) for the success branch too. Draft
  position: yes — one fetcher, checked on both branches.
- Whether the merge-driver provisioning warrants a new ADR or is covered by applying ADR-0025.
  Draft position: covered by ADR-0025; no new ADR.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-06-13
- **Notes**: Approved via mindspec approve spec