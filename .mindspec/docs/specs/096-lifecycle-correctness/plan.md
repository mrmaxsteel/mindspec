---
adr_citations:
    - id: ADR-0025
      sections:
        - jsonl-as-build-artifact / regenerate-from-DB merge (Bead 1 merge-driver provisioning)
    - id: ADR-0035
      sections:
        - recovery-line + "exit codes never lie" contract (Bead 1 doctor messages; Bead 2 close-loss hard error)
approved_at: "2026-06-13T11:52:00Z"
approved_by: user
bead_ids:
    - mindspec-xayb.1
    - mindspec-xayb.2
    - mindspec-xayb.3
    - mindspec-xayb.4
    - mindspec-xayb.5
spec_id: 096-lifecycle-correctness
status: Approved
version: "1"
---
# Plan: 096-lifecycle-correctness

> Five independent correctness fixes, one bead per spec requirement, each grounded in cited
> `file:line` against the spec/096 branch (= current `main`) and each proven RED-on-revert.
> Sole impacted domain is **workflow**; every owned path touched is already claimed by
> `.mindspec/docs/domains/workflow/OWNERSHIP.yaml` EXCEPT `internal/adr/**`, which Bead 3
> (bn3u) claims ON ITS OWN BRANCH so its `parse.go` change satisfies its own adr-divergence
> gate. No new ADR: Bead 1 applies the existing ADR-0025 merge decision to the bootstrap
> provisioning path; Beads 2-5 are correctness fixes inside the existing contracts.

## ADR Fitness

- **ADR-0025** (jsonl-as-build-artifact; Status: **Accepted**; Domain(s): workflow, execution,
  bootstrap): the beads merge driver regenerates `.beads/issues.jsonl` from the canonical Dolt
  DB on merge. Bead 1 provisioning that driver in bootstrap is a DIRECT application of ADR-0025's
  "the jsonl is a deterministic projection; regenerate-from-DB is the correct merge" decision —
  **no new ADR** (applies an existing decision). Domain(s) include **workflow** (the sole
  impacted domain) → no `adr-cite-irrelevant`.
- **ADR-0035** (agent error contract — recovery lines + exit codes; Status: **Accepted**;
  Domain(s): workflow, execution, core): Bead 1's doctor failure messages and Bead 2's hard
  close-loss error follow its recovery-line convention and the spec-092 "exit codes never lie"
  invariant. **No new ADR** (the fixes restore the existing contract). Domain(s) include
  **workflow** → no `adr-cite-irrelevant`.

No new ADR is required by any bead (resolves the spec's Design Question on whether merge-driver
provisioning warrants one — it does not; ADR-0025 covers it). Both cited ADRs are Accepted and
their Domain(s) intersect **workflow**, the only impacted domain.

## Testing Strategy

Every behavioral change is proven **RED-on-revert** — the test FAILS if the fix is reverted to
the cited pre-fix code:
- Bead 1: drop the bootstrap provisioning → a real `git merge` of a both-sides-changed
  `.beads/issues.jsonl` in a freshly bootstrapped repo leaves UNMERGED STAGES (the actual
  merge-execution direction, not a doctor-detection substitute); write the wrapper `0644` instead
  of `0755` → the exec-bit assertion (`Mode().Perm()&0o111`) goes RED; restore the absolute
  `--fix` value → the cross-worktree test fails (machine-specific path baked into shared
  `.git/config`). The GREEN counterpart: with provisioning present, that same merge resolves
  cleanly via the driver (`git ls-files -u` empty).
- Bead 2: restore the unconditional `Result{BeadClosed: true}` → a nil-close whose re-read affirms
  `in_progress` reports success + exit 0 instead of erroring (case b RED). Separately, the
  false-hard-error guard: a nil-close whose re-read ERRORS must NOT fail (case c) — RED if the
  predicate over-fires on a transient fetch error.
- Bead 3: restore the whole-suffix `Atoi` → `NextID` over slugged ADRs returns a colliding low
  ID instead of `0036`; drop the OWNERSHIP claim → `mindspec complete` hard-errors on the unowned
  `internal/adr/parse.go`.
- Bead 4: restore `workspace.FindRoot` → `adr create` from a worktree writes into main.
- Bead 5: remove the subcommand → `mindspec version` errors `unknown command`.

Beyond per-package units, each bead runs **golangci-lint locally (CI Lint-job parity)**: American
spelling only (`behavior`, not `behaviour` — the spec-094 Lint-failure lesson) and **no new
gosec** findings. Every bead gates on `go build ./...` + `go test -short -race ./...` green.

**Bead dependency:** Bead 4 (8lzq) depends on Bead 3 (bn3u) — Req 4's worktree-local `NextID`
numbering must be computed with Bead 3's leading-integer `NextID` fix in place, and sequencing
keeps the two `internal/adr`-adjacent changes off each other (Bead 3 edits
`internal/adr/parse.go`; Bead 4 edits `cmd/mindspec/adr.go` and only CALLS `internal/adr`). Beads
1, 2, and 5 are mutually independent — different files, no shared seam.

## Bead 1: Bootstrap provisions the beads jsonl merge driver (portable, cross-worktree, EXECUTABLE) + doctor portable-path fix + regression-lock (oe0u)

Make a freshly bootstrapped repo merge-driver-safe from commit 0: write the `.gitattributes`
`merge=beads` attribute, set `merge.beads.driver` to a PORTABLE (worktree-relative, not absolute)
value, and EMBED + write the `bd-jsonl-merge-driver.sh` wrapper into the target repo **with the
exec bit set** (not assume it is tracked). The headline proof is GREEN end-to-end: a real
`git merge` of a both-sides-changed `.beads/issues.jsonl` in a freshly bootstrapped repo resolves
cleanly via the provisioned driver (regenerate-from-DB), with NO unmerged stages. Fix the doctor
`--fix` non-portable absolute path and regression-lock the existing `checkBeadsMergeDriver`
detection lane. Heaviest bead (M).

**Steps**
1. Embed the wrapper: add `internal/bootstrap/assets/bd-jsonl-merge-driver.sh` (a copy of the
   tracked `scripts/bd-jsonl-merge-driver.sh`) and `//go:embed assets/bd-jsonl-merge-driver.sh`
   into a new bootstrap source (`go:embed` can only reach files inside the package tree, so the
   wrapper must live under `internal/bootstrap/`; note `go:embed` stores BYTES only — no file
   mode — so the exec bit must be set explicitly on write, see step 3). Add a **drift-guard test**
   asserting the embedded bytes are byte-equal to `scripts/bd-jsonl-merge-driver.sh` so the two
   copies never diverge.
2. Add a `provisionBeadsMergeDriver(root, dryRun)` step wired into `bootstrap.Run`
   (`internal/bootstrap/bootstrap.go:108`, AFTER the manifest write loop) that, for a non-dry-run
   repo, **ensure-if-absent** (never clobber a user-authored value): (a) writes the embedded
   wrapper to `<root>/scripts/bd-jsonl-merge-driver.sh` if absent (mode `0755` — see step 3);
   (b) appends `.beads/issues.jsonl merge=beads` to `<root>/.gitattributes` if that mapping is
   absent (newline-safe — see step 4); (c) sets `merge.beads.driver` via `git config` to the
   PORTABLE value `'scripts/bd-jsonl-merge-driver.sh' %A %O %B` only when no driver is already
   configured AND only when `<root>/.git` exists — see step 5. Record each action in `Result`
   (Created/Skipped) and honor `dryRun` (report, never write).
3. **EXEC-BIT (critical — the fix is a no-op without it):** the bootstrap manifest write path
   uses `safeio.WriteFileNoSymlink(target, content, 0644)` (`bootstrap.go:193`), which HARDCODES
   mode `0644`. A non-executable wrapper makes `resolveDriverCommand`'s `0o111` gate fail and git
   silently TEXT-merges `.beads/issues.jsonl` — the fix does nothing. Do NOT route the wrapper
   through that `0644` call-site. `safeio.WriteFileNoSymlink` ALREADY honors its `perm` argument
   (`safeopen.go:68`, `tmp.Chmod(perm)` is umask-independent), so write the wrapper with
   `safeio.WriteFileNoSymlink(scriptPath, embeddedBytes, 0o755)` (a primitive that ACTUALLY sets
   the bit); if a future refactor narrows that helper, fall back to `os.Chmod(scriptPath, 0o755)`
   immediately after the write. The bead test MUST `os.Stat` the ACTUALLY-WRITTEN wrapper and
   assert `Mode().Perm()&0o111 != 0`, and be RED if the wrapper is written `0644`.
4. **NEWLINE-SAFE `.gitattributes` append (edge):** appending to a `.gitattributes` that lacks a
   trailing newline must NOT concatenate onto the last line (e.g. `*.png binary.beads/issues.jsonl
   merge=beads` would corrupt the pattern, yet `gitattributesHasBeadsMerge`'s `strings.Fields`
   scan would still find a `merge=beads` token and falsely report present). The append MUST
   prepend a `\n` when the existing file is non-empty and lacks a trailing newline so the
   `.beads/issues.jsonl merge=beads` line lands on its OWN line; the `gitattributesHasBeadsMerge`
   detection (`beads.go:464`, line-split + `strings.Fields` requiring the PATTERN field to be
   exactly `.beads/issues.jsonl`) must agree. A test with a no-trailing-newline `.gitattributes`
   asserts the resulting line is exactly `.beads/issues.jsonl merge=beads` on its own line.
5. **Provision-after-init / no-`.git` guard (edge):** `git config merge.beads.driver` needs a git
   repo (`<root>/.git`). Bootstrap can run in a non-git dir (`mindspec init` before `git init`),
   so step 2(c) is BEST-EFFORT: skip the `git config` write (Skipped, no error) when `<root>/.git`
   is absent — the wrapper + `.gitattributes` are still provisioned, and `doctor --fix` (or a
   re-run of bootstrap) converges the config once the repo exists. Do NOT hard-fail bootstrap in a
   non-git dir. A no-`.git` test asserts bootstrap succeeds and writes wrapper + gitattributes
   without erroring.
6. Portable path + doctor `--fix` converge + PR-merge residual: write the repo-RELATIVE
   single-quoted `scripts/bd-jsonl-merge-driver.sh`, NOT an absolute path — `resolveDriverCommand`
   (`internal/doctor/beads.go:543`) already resolves a relative path containing `/` against the
   worktree top-level, and git runs merge drivers from the worktree root, so the SAME shared
   `.git/config` value is valid from every linked worktree. Fix the doctor `--fix` `wantDriver`
   (`internal/doctor/beads.go:364`) to write this same relative portable form, so `doctor --fix`
   CONVERGES an existing machine-specific absolute value to the portable one instead of re-baking
   an absolute path. Document the GitHub-PR-merge residual (a code comment + the spec record): PR
   merges on GitHub never run local merge drivers; the post-merge beads-sync pattern compensates
   — documented, NOT fixed here.
7. Tests:
   - **GREEN end-to-end merge (the headline proof, P3 + edge):** in a freshly bootstrapped repo
     with `bd`/Dolt initialized, create a both-sides-changed `.beads/issues.jsonl` conflict (two
     branches each closing/adding a different bead, then `git merge` the other) and run the real
     `git merge` → it resolves CLEANLY via the provisioned driver (regenerate-from-DB), `git
     ls-files -u` is EMPTY (zero unmerged stages) and the merged jsonl is a valid superset. The
     driver is EXERCISED, not just present. If this needs `bd`/Dolt and cannot run under `-short`,
     mark it an integration test gated on `bd` presence (`t.Skip` when absent) and pin that it
     runs in the non-short CI lane.
   - **Tightened RED-on-revert:** reverting the provisioning makes the SAME both-sides-changed
     `.beads/issues.jsonl` `git merge` FAIL with unmerged stages (`git ls-files -u` non-empty) —
     the ACTUAL merge-execution direction. (No "OR doctor flags a missing driver" escape: the
     driver path itself must be falsifiable, not its doctor detector.)
   - **Provisioning units (RED-on-revert):** a freshly bootstrapped repo has `.gitattributes`
     mapping `.beads/issues.jsonl` to `merge=beads`, `merge.beads.driver` set to the portable
     RELATIVE value, and the wrapper FILE present with `Mode().Perm()&0o111 != 0` (step 3 RED at
     `0644`); `checkBeadsMergeDriver` then reports OK with NO `--fix`. Newline-safe append test
     (step 4). No-`.git` best-effort test (step 5). **Cross-worktree** test: the provisioned value
     is valid from a linked worktree (no absolute path baked in). **Ensure-if-absent** test: a
     pre-existing user-set driver/attribute/wrapper is NOT clobbered. **Drift-guard** test
     (step 1).
   - **Doctor regression-lock (behavior UNCHANGED):** pin the three `checkBeadsMergeDriver`
     classes — orphaned `bd merge` (`beads.go:433`), missing driver with attribute
     (`beads.go:374`), inverse missing-attribute (`beads.go:402`) — and assert the portable
     relative value re-validates through `resolveDriverCommand`; assert `--fix` now writes the
     relative form. RED if detection is loosened.

**Verification**
- [ ] `go build ./... && go test -race ./internal/bootstrap/... ./internal/doctor/...` green
- [ ] GREEN: a real `git merge` of a both-sides-changed `.beads/issues.jsonl` in a freshly
      bootstrapped repo resolves cleanly via the provisioned driver — `git ls-files -u` empty (no
      unmerged stages); RED-on-revert: dropping provisioning makes the SAME merge leave unmerged
      stages (the actual merge, not doctor detection)
- [ ] Fresh bootstrap → gitattributes `merge=beads` + portable relative `merge.beads.driver` +
      wrapper file present with the exec bit (`Mode().Perm()&0o111 != 0`, RED at `0644`);
      `checkBeadsMergeDriver` OK with no `--fix`
- [ ] Newline-safe append: a no-trailing-newline `.gitattributes` yields exactly
      `.beads/issues.jsonl merge=beads` on its own line (not concatenated onto the prior line)
- [ ] No-`.git` dir: bootstrap provisions wrapper + gitattributes and skips the `git config`
      write without erroring (provision-after-init / best-effort)
- [ ] Provisioned value valid from a linked worktree (no absolute path) — tested
- [ ] Embedded wrapper is byte-equal to `scripts/bd-jsonl-merge-driver.sh` (drift guard)
- [ ] Ensure-if-absent: an existing user-set driver is not clobbered; `doctor --fix` converges an
      absolute value to the relative portable form
- [ ] golangci-lint clean (American spelling; no new gosec); `go test -short -race ./...` green

**Acceptance Criteria**
- [ ] In a freshly bootstrapped repo, a real `git merge` of a both-sides-changed
      `.beads/issues.jsonl` resolves cleanly via the provisioned driver (regenerate-from-DB) with
      NO unmerged stages — the driver is EXERCISED end-to-end, and reverting provisioning makes
      that same merge fail with unmerged stages.
- [ ] A freshly bootstrapped repo has `.gitattributes` mapping `.beads/issues.jsonl` to
      `merge=beads`, a `merge.beads.driver` config pointing at the wrapper via a PORTABLE path,
      and the wrapper script FILE written by bootstrap (the embedded wrapper, not a pre-existing
      tracked file) present on disk WITH the exec bit set (`Mode().Perm()&0o111 != 0`) — and
      `checkBeadsMergeDriver` reports OK with no `--fix`.
- [ ] The provisioned driver config is valid from a linked worktree (cross-worktree safe) — no
      machine-specific absolute path baked into the shared `.git/config`; the `.gitattributes`
      append is newline-safe; the `git config` write is best-effort/skipped when `<root>/.git` is
      absent (provision-after-init).
- [ ] The `checkBeadsMergeDriver` detection lane (orphaned `bd merge`, missing driver, missing
      attribute) is regression-locked and its behavior is unchanged; the GitHub-PR-merge residual
      is documented, not fixed.

**Depends on**
None

## Bead 2: `complete` verifies the close persisted, hard-errors on a silent close-loss (2u0u)

After the close step, re-read the bead's persisted status and surface a non-zero error when a
nominally-successful `closeBeadFn` did NOT actually persist `closed` — restoring the spec-092
"exit codes never lie" invariant. Preserve the already-closed tolerance and the distinct
post-merge CLOSED-but-unmerged guard. (S.)

**Steps**
1. `internal/complete/complete.go` close step (`:348-357`): on the SUCCESS branch (where
   `closeBeadFn(beadID)` returns nil), re-read the bead via the existing `fetchBeadByIDFn(beadID)`
   path (the same fetcher used in the error branch at `:351`). Use the SAME status predicate
   already in the error branch at `:352` —
   `strings.EqualFold(strings.TrimSpace(info.Status), "closed")` — so both branches agree on what
   "closed" means.
2. **3-case re-read outcome (mirror the already-closed branch at `:351-353`, do NOT over-fire):**
   - **(a) re-read SUCCEEDS (fetchErr == nil) and AFFIRMS `closed`:** proceed to merge + cleanup;
     set `Result{BeadClosed: true}` at `:370`. `BeadClosed: true` is asserted ONLY here (a
     confirmed-closed re-read) and via the pre-existing already-closed tolerance — never
     unconditionally.
   - **(b) re-read SUCCEEDS (fetchErr == nil) and AFFIRMS `open`/`in_progress`:** this is the REAL
     spec-092 Bead 7 bug (prints `closed`, exits 0, but `bd show` later reports `in_progress` /
     `closed_at None`). Return a HARD error (ADR-0035 recovery line) so `complete` exits non-zero
     — never `closed` + exit 0 on an unpersisted close.
   - **(c) re-read itself ERRORS (fetchErr != nil) after `closeBeadFn` returned nil:** the close
     DID succeed (the close call returned nil); a transient/eventually-consistent fetch failure
     (Dolt hiccup, race) must NOT be turned into a NEW non-zero exit. TOLERATE: warn and proceed
     to merge + cleanup (mirroring the already-closed tolerance), do NOT hard-block. `complete` is
     idempotent and re-runnable, so a false hard-fail here would be the OPPOSITE "exit codes lie"
     inversion. Do NOT add any "fetch cannot confirm closed → hard error" path.
3. Preserve the already-closed tolerance (`:351-353`: a genuine prior close still proceeds to
   merge + cleanup) and leave the distinct post-merge CLOSED-but-unmerged guard (`:416-428`)
   unchanged.
4. Tests (RED-on-revert):
   - **(b) confirmed-in_progress-after-nil-close:** a fake `closeBeadFn` returns nil while
     `fetchBeadByIDFn` reports `in_progress` (fetchErr == nil) → `complete.Run` returns a non-zero
     error and does NOT report `BeadClosed: true` (FAILS if reverted to the unconditional
     `BeadClosed: true`). This is the headline RED proof.
   - **(c) close-persisted-but-fetch-errors (false-hard-error guard):** `closeBeadFn` returns nil
     while `fetchBeadByIDFn` returns an ERROR → `complete.Run` does NOT spuriously fail (proceeds,
     no new false-hard-error). This pins that a transient fetch failure after a good close is
     tolerated.
   - **(a) close-that-persisted:** `closeBeadFn` nil + `fetchBeadByIDFn` reports `closed` →
     proceeds normally, `BeadClosed: true`.
   - **already-closed:** a genuine already-closed bead (error branch `:351-353`) still completes
     (merge + cleanup), no spurious failure.

**Verification**
- [ ] `go build ./... && go test -race ./internal/complete/...` green
- [ ] (b) Simulated nil-but-confirmed-in_progress close → non-zero error, no `closed` + exit 0,
      RED on revert to unconditional `BeadClosed: true` — tested
- [ ] (c) Close persisted but the re-read fetch ERRORS → NO spurious failure (the false-hard-error
      guard) — tested
- [ ] A true already-closed bead still completes (merge + cleanup) without spurious failure
- [ ] golangci-lint clean (American spelling; no new gosec); `go test -short -race ./...` green

**Acceptance Criteria**
- [ ] `mindspec complete` exits NON-ZERO with a clear error when the close step returns success but
      a SUCCESSFUL re-read affirms the bead is still `open`/`in_progress`; it never prints `closed`
      + exit 0 on an unpersisted close. `BeadClosed: true` is set only after a confirmed-closed
      re-read (or the pre-existing already-closed tolerance).
- [ ] When the post-close re-read itself ERRORS (after a nil `closeBeadFn`), `complete` does NOT
      spuriously hard-fail — it tolerates, warns, and proceeds (no false "exit codes lie"
      inversion).
- [ ] A true already-closed bead still completes (merge + cleanup) without spurious failure, and
      the post-merge CLOSED-but-unmerged guard is preserved.

**Depends on**
None

## Bead 3: `adr.NextID` parses the leading integer of slugged ADR filenames + on-branch OWNERSHIP claim (bn3u)

Parse the leading numeric run of `ADR-NNNN-slug.md` so slugged ADRs count toward `maxNum` and
`NextID` returns the next free ID instead of a collision. Because this is the first `.go` change
under `internal/adr/**` and that path is unclaimed, the bead claims it under **workflow** ON ITS
OWN BRANCH. (S.)

**Steps**
1. `internal/adr/parse.go::NextID` (`:242-255`): replace `numStr := strings.TrimPrefix(name,
   "ADR-")` + `strconv.Atoi(numStr)` with extraction of the LEADING numeric run after the `ADR-`
   prefix (the digits up to the first non-digit / first hyphen). For `ADR-0025-jsonl-as-build-
   artifact.md` this yields `0025`; bare legacy `ADR-NNNN.md` still parses; a non-numeric head
   (e.g. `ADR-foo.md`) still `continue`s (skipped).
2. ON the bn3u bead branch, claim `internal/adr/**` in
   `.mindspec/docs/domains/workflow/OWNERSHIP.yaml` (add to the `paths:` list) so the changed
   `internal/adr/parse.go` passes the `adr-divergence-unowned` gate
   (`internal/validate/divergence.go:194-203`). Workflow is already covered by the Accepted
   ADR-0025 + ADR-0035, so this claim clears BOTH the unowned block AND the follow-on uncovered-
   domain coverage check — NO new ADR, NO `--override-adr`. (Spec 096 runs on v0.9.0 with the
   vvs9 ref-reading gate, so the OWNERSHIP claim committed on the bn3u branch is READ from that
   branch and SATISFIES ITS OWN GATE.)
3. Tests (RED-on-revert): `NextID` over slugged ADR fixtures up to `ADR-0035-*` returns `0036`
   (FAILS if reverted to the whole-suffix `Atoi`, which skips every file and returns a colliding
   low ID); a mixed legacy bare `ADR-0007.md` + slugged set is counted correctly; a single-digit
   `ADR-1.md` and a leading-zero `ADR-00xx-slug.md` parse via the leading-numeric run; an empty
   dir returns `0001`; a malformed `ADR-foo.md` is skipped.
4. golangci-lint local parity (American spelling; no new gosec) + `go test ./internal/adr/...`.

**Verification**
- [ ] `go build ./... && go test -race ./internal/adr/...` green
- [ ] `NextID` over slugged fixtures up to `ADR-0035-*` returns `0036`; bare + slugged forms parse
- [ ] `mindspec complete` for bn3u emits NO `adr-divergence-unowned` and needs NO `--override-adr`
      (the on-branch `internal/adr/**` OWNERSHIP claim satisfies its own vvs9 ref-read gate)
- [ ] golangci-lint clean (American spelling; no new gosec); `go test -short -race ./...` green

**Acceptance Criteria**
- [ ] `adr.NextID` over a directory of slugged ADRs up to `ADR-0035-*` returns `0036` (not a
      colliding lower ID); both legacy bare `ADR-NNNN.md` and slugged `ADR-NNNN-slug.md` forms
      parse.
- [ ] The bn3u diff (which touches `internal/adr/parse.go`) passes the adr-divergence lane: the
      on-branch `internal/adr/**` OWNERSHIP claim in the workflow manifest is present, so
      `mindspec complete` for bn3u emits NO `adr-divergence-unowned` and needs NO `--override-adr`.

**Depends on**
None

## Bead 4: `adr create` writes into the invoking worktree, not the main checkout (8lzq)

Resolve a worktree-LOCAL root for `adr create` so a new ADR authored from a bead/spec worktree
lands in that worktree (numbered against branch-side ADRs via Bead 3's `NextID`), not in main. (S.)

**Steps**
1. `cmd/mindspec/adr.go` `adrCreateCmd` RunE (`:28`): replace `workspace.FindRoot(cwd)` with
   `workspace.FindLocalRoot(cwd)` (`internal/workspace/workspace.go:94`, already exists — unlike
   `FindRoot`, it does NOT resolve a worktree back to the main repo) so the WRITE target `root` is
   the invoking worktree. The new ADR is written to
   `<worktree>/.mindspec/docs/adr/ADR-NNNN-*.md` and does NOT appear in main. The READ/list
   commands keep `FindRoot` (out of scope; only the CREATE/WRITE path changes).
2. **Union NextID (create/read-consistency, edge):** the WRITE store is worktree-local
   (`FindLocalRoot`), but READS/validation union branch + main via `OverlayStore` (e.g.
   `internal/validate/plan.go:205` = `NewOverlayStore(NewFileStore(treeRoot), NewFileStore(root))`).
   A `NextID` computed over ONLY the worktree-local store could collide with a main-only ADR added
   after the branch diverged. So compute the new ID over the OVERLAY/UNION (worktree-local
   branch ADRs ∪ main-checkout ADRs): take the max leading-integer (Bead 3's leading-numeric-run
   parse) across BOTH the `FindLocalRoot` root and the `FindRoot` (main) root, `+1`. Concretely:
   resolve both roots in `adr.go` and number against their union (e.g. a union-aware `NextID`, or
   `max(NextID(localRoot), NextID(mainRoot))`), so the new ID is GLOBALLY collision-free across the
   branch + main. (This coordinates with Bead 3's `NextID` fix; the 4→3 dependency already exists.)
   If a fuller `OverlayStore.NextID` is judged out of scope, the bound is the two-root union above
   and it MUST be documented as such.
3. Tests (RED-on-revert): `adr create` invoked from a synthetic linked worktree writes the ADR
   into THAT worktree's `.mindspec/docs/adr/` and the file does NOT appear in the main checkout's
   ADR directory (FAILS if reverted to `FindRoot`); a main-only ADR added AFTER branch divergence
   (present in the main root but not the worktree) is COUNTED by the union `NextID` so the
   worktree create does not collide with it (FAILS if `NextID` sees only the worktree-local root).
4. golangci-lint local parity (American spelling; no new gosec) + `go test ./cmd/mindspec/...`.

**Verification**
- [ ] `go build ./... && go test -race ./cmd/mindspec/...` green
- [ ] `adr create` from a worktree writes into that worktree's `.mindspec/docs/adr/`, not main —
      tested RED-on-revert from `FindRoot`
- [ ] `NextID` numbering computed over the branch+main UNION so a main-only ADR added after
      divergence does not collide — tested
- [ ] golangci-lint clean (American spelling; no new gosec); `go test -short -race ./...` green

**Acceptance Criteria**
- [ ] `mindspec adr create` run from a bead/spec worktree writes the ADR into THAT worktree's
      `.mindspec/docs/adr/`, and the file does NOT appear in the main checkout.
- [ ] NextID numbering is computed over the worktree-local branch ADRs UNION the main-checkout
      ADRs (the same union reads/validation see) so the new ADR does not collide with branch-side
      OR main-only ADRs.

**Depends on**
Bead 3 (Req 4's worktree-local `NextID` numbering builds on Bead 3's leading-integer `NextID` fix;
sequencing also keeps the two `internal/adr`-adjacent changes off each other)

## Bead 5: Add a `version` subcommand byte-equal to `--version` (2b4n)

Add a `mindspec version` cobra subcommand whose stdout reproduces the SAME cobra-decorated string
the `--version` flag emits (`mindspec version <version> (commit) date`), and register it in
`root.go`. (S.)

**Steps**
1. Add `cmd/mindspec/version.go` with a `versionCmd` whose RunE reproduces the SAME decorated
   string cobra's `--version` template produces. Cobra 1.10.x's default version template is
   `{{with .DisplayName}}{{printf "%s " .}}{{end}}{{printf "version %s" .Version}}\n` — i.e. it
   uses the command's `DisplayName` (NOT a hand-built literal name) and ends with a trailing
   newline. So derive the prefix from the SAME source — print
   `<rootCmd.DisplayName()> version <rootCmd.Version>\n` (reading the same `version`/`commit`/`date`
   vars at `root.go:35-39` via `rootCmd.Version`), reproducing the identical decorated form
   including the trailing newline (not just the bare version value, and not a hardcoded `mindspec`
   literal). No custom `SetVersionTemplate` exists in `cmd/` or `internal/`, so the default
   template is authoritative.
2. Register `rootCmd.AddCommand(versionCmd)` in `root.go`'s AddCommand list (`root.go:216-249`).
3. Tests (RED-on-revert): capture the ACTUAL stdout of `mindspec --version` (the real
   cobra-decorated `DisplayName`-prefixed string + trailing newline, NOT a hand-built literal) and
   the stdout of `mindspec version`, and assert they are BYTE-EQUAL; removing the subcommand makes
   `mindspec version` error `unknown command` (the RED proof). Comparing against the live
   `--version` output (rather than a literal) is what pins the `DisplayName` decoration + newline.
4. golangci-lint local parity (American spelling; no new gosec) + `go build/test ./cmd/mindspec/...`.

**Verification**
- [ ] `go build ./... && go test -race ./cmd/mindspec/...` green
- [ ] `mindspec version` stdout is byte-equal to `mindspec --version` — tested
- [ ] Removing the subcommand → `mindspec version` errors `unknown command` (RED-on-revert)
- [ ] golangci-lint clean (American spelling; no new gosec); `go test -short -race ./...` green

**Acceptance Criteria**
- [ ] `mindspec version` succeeds and its stdout is byte-equal to `mindspec --version` (both emit
      the cobra-decorated `mindspec version <version> ...` string, not just the bare value).
- [ ] The subcommand is registered in `root.go`'s AddCommand list.

**Depends on**
None

## Provenance

| Acceptance Criterion (spec) | Bead | Verified By |
|-----------------------------|------|-------------|
| End-to-end: a both-sides-changed `.beads/issues.jsonl` merge in a fresh repo resolves cleanly via the provisioned driver, NO unmerged stages (spec.md Validation Proof) | Bead 1 | Step 7 (GREEN merge test + tightened RED) + verification |
| Fresh repo has gitattributes + portable driver + EXECUTABLE embedded wrapper; doctor OK no `--fix` | Bead 1 | Steps 1,2,3,7 + verification |
| `.gitattributes` append newline-safe; `git config` best-effort/skip when no `.git` | Bead 1 | Steps 4,5,7 |
| Provisioned driver valid from a linked worktree (no absolute path) | Bead 1 | Steps 6,7 |
| `checkBeadsMergeDriver` regression-locked, unchanged; PR-merge residual documented | Bead 1 | Steps 6,7 |
| `complete` exits non-zero on a nil-close whose re-read AFFIRMS not-closed (no `closed` + exit 0) | Bead 2 | Steps 1,2(b),4 |
| `complete` does NOT spuriously fail when the post-close re-read itself errors (false-hard-error guard) | Bead 2 | Steps 2(c),4 |
| A true already-closed bead still completes (merge + cleanup) | Bead 2 | Steps 3,4 |
| `NextID` over slugged ADRs up to `ADR-0035-*` returns `0036` | Bead 3 | Steps 1,3 |
| bn3u diff passes adr-divergence lane via on-branch `internal/adr/**` OWNERSHIP claim | Bead 3 | Step 2 + verification |
| `adr create` from a worktree writes into that worktree, not main | Bead 4 | Steps 1,3 |
| NextID numbering computed over the branch+main UNION (collision-free vs main-only ADRs) | Bead 4 | Steps 2,3 |
| `mindspec version` stdout byte-equal to actual `mindspec --version` (DisplayName-decorated) | Bead 5 | Steps 1,3 |
| `go build` + `go test -short -race ./...` + golangci-lint green | All beads | Each bead's verification |
