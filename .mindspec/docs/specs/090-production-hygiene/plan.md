---
adr_citations:
    - id: ADR-0025
    - id: ADR-0027
    - id: ADR-0029
approved_at: "2026-05-20T18:22:47Z"
approved_by: user
bead_ids:
    - mindspec-hdjv.1
    - mindspec-hdjv.2
    - mindspec-hdjv.3
spec_id: 090-production-hygiene
status: Approved
version: "1"
---
# Plan: 090-production-hygiene

## ADR Fitness

- **ADR-0029** (new — "Supply Chain Attestations via Cosign Keyless +
  Syft SBOM"): authored by this spec. The stub at
  `.mindspec/docs/adr/ADR-0029-supply-chain-attestations.md` already
  carries Status=Accepted and records the three substantive choices
  (keyless via GitHub OIDC; SPDX-JSON via syft through GoReleaser's
  native `sboms:` directive; signing targets are GoReleaser archives +
  `checksums.txt`). Bead 1 finalizes ADR-0029 to the **as-planned**
  state: the `signs:` / `sboms:` stanza shapes defined in this plan
  (Bead 2 step 3), the `.cosign.bundle` naming convention, the
  keyless-via-OIDC choice. Bead 1 is genuinely independent and can
  land first; the ADR's Finalized status reflects the planned design,
  not a post-implementation reconciliation. If post-Bead-3 minor edits
  are needed for accuracy (e.g., a third-party action's pinned SHA
  shifts), that is a small fixup commit on the ADR, not a
  re-finalization.
- **ADR-0025** (Accepted — "JSONL as build artifact"): carried
  forward unchanged. F6 leaves the JSONL build-artifact contract
  untouched; cosign signatures and SPDX SBOMs are emitted **alongside**
  the existing archive set, not in place of it. Verified by Bead 3's
  dryrun assertion that the pre-existing archive shapes
  (`mindspec_<version>_<os>_<arch>.tar.gz` / `.zip` + `checksums.txt`)
  are still produced.
- **ADR-0027** (Accepted — "MindSpec is OTEL-config only"): carried
  forward unchanged. F6 touches no `cmd/`, no `internal/`, no `go.mod`,
  no `go.sum`; the no-network-side-effects + no-spawn invariants
  recorded by ADR-0027 are out-of-scope for this spec and trivially
  preserved by construction. Cosign and syft execute only inside
  GitHub Actions runners; nothing ships into the mindspec binary or
  its dep graph.

No accepted ADR is contradicted by this plan. ADR-0028
(bench-rescue-procedure) is the most recent CI-touching ADR but is
not impacted: F6 adds a new workflow file (`release-dryrun.yml`) and
modifies an existing one (`release.yml`); it does not touch the
bench-rescue path or the `pre-spec-084-bench-delete` annotated tag.

## Testing Strategy

F6 has **no Go-test surface** — it modifies no `cmd/` or `internal/`
files. Validation is workflow-level and binary-output-level:

1. **`release-dryrun.yml` is itself the integration test.** The
   workflow file added in Bead 3 runs on every PR that modifies
   release infrastructure (including the PR that lands F6 — the PR
   modifies `release.yml`, `.goreleaser.yml`, `SECURITY.md`, and
   `release-dryrun.yml` itself, so the workflow self-triggers on its
   introducing PR). It executes `goreleaser release --snapshot
   --skip=publish` end-to-end, which forces cosign and syft to run
   against real archives produced by the actual GoReleaser config.
   This is the equivalent of a unit test for the release pipeline.

2. **Cosign verification in dryrun mode: structural only.** The dryrun
   does **not** attempt `cosign verify-blob` with a pinned identity
   regex. Reason: snapshot builds run on a PR-context OIDC token
   whose subject is `…/release-dryrun.yml@refs/pull/<N>/merge`, which
   cannot satisfy the spec's pinned Fulcio identity regex
   (`^https://github\.com/mrmaxsteel/mindspec/\.github/workflows/release\.yml@refs/tags/v.*$`).
   The dryrun therefore asserts (a) cosign and syft processes exited
   0 — any non-zero exit fails `goreleaser` itself and aborts the
   workflow before the assertion step runs — (b) each archive has a
   non-zero-byte `.sig` and `.cosign.bundle` side-car, and (c) the
   SBOM parses as valid SPDX JSON. The identity-bound check runs
   only on the post-tag release path and is recorded in the spec's
   Validation Proofs section as the reviewer-runnable verification
   command.

3. **SBOM presence + validity asserted in dryrun.** For every archive
   under `dist/`, the dryrun asserts `[ -s <archive>.spdx.json ]` and
   `jq -e '.spdxVersion' <archive>.spdx.json >/dev/null`. SPDX
   well-formedness is enforced; richer schema validation
   (e.g., `spdx-tools` validate) is out of scope for this spec — the
   `jq -e '.spdxVersion'` check matches the spec's Validation Proofs
   contract verbatim.

4. **Workflow YAML lint.** Bead 2 and Bead 3 each run `actionlint`
   (via `rhysd/actionlint` Docker image, SHA-pinned) against the
   touched workflow files locally before the commit lands. The bead's
   verification block records the actionlint command and its
   pass/fail. `actionlint` enforces YAML syntax, `uses:` reference
   shape, and runner image validity; SHA-pinning enforcement is
   covered separately by the spec's `grep -E '@[0-9a-f]{40}\b'`
   contract (Requirement #5; verified in every bead's checklist).

5. **No snapshot/golden test for workflow YAML itself.** Workflow YAML
   is the source of truth; a golden file would just duplicate it.
   What we assert is **behavioral outcome** via the dryrun job and
   **shape** via the grep contracts in spec's Validation Proofs.

6. **Existing Go test surface unaffected (HC-3, HC-5).** The bead
   verification blocks include `go build ./... && go test -short
   ./...` to confirm the surface remains green by construction — F6
   touches no Go files, so this is a trivial check that catches
   accidental .go edits.

## Decomposition rationale

The 3-bead split below maps 1:1 to the spec's four Requirements with
clean dependency edges:

- Bead 1 (SECURITY.md + ADR-0029) is **independent of CI changes** —
  pure docs. Can land first, in parallel with Beads 2-3, or last.
- Bead 2 (release.yml + .goreleaser.yml) is the **substantive
  pipeline change**. Produces the signed/SBOMed artifacts the dryrun
  validates.
- Bead 3 (release-dryrun.yml) **depends on Bead 2** because there is
  nothing to verify until Bead 2's GoReleaser config emits `.sig` /
  `.cosign.bundle` / `.spdx.json` files.

Max dependency depth is 1 (Bead 3 → Bead 2; Bead 1 has no parent).
Alternative splits considered: (a) combining Beads 2 and 3 into one
bead — rejected because the dryrun workflow can self-test once Bead 2
is on disk, so splitting gives a cleaner first-PR-already-validated
property and keeps the Bead 2 diff small; (b) splitting Bead 2 into
"release.yml edits" + ".goreleaser.yml edits" — rejected because the
two files are coupled (workflow installs cosign/syft; goreleaser
invokes them) and must land in the same commit to stay
build-and-test green per HC-3/HC-5.

## Bead 1: SECURITY.md at repo root + ADR-0029 finalization

Lands the human-facing security-reporting policy and finalizes the
ADR-0029 stub against the as-shipped pipeline shape that Beads 2-3
will produce. This bead is documentation-only and has no CI surface
of its own; it is independent of Beads 2 and 3 and can land in any
order relative to them.

**Steps**

1. Create `SECURITY.md` at the repo root with the three required
   section headings (exact heading text required by the spec's grep
   contract):
   - `## Reporting a Vulnerability` — directs reporters to GitHub
     Security Advisory
     `https://github.com/mrmaxsteel/mindspec/security/advisories/new`
     and provides a security contact email as fallback (the spec
     accepts either or both; this plan ships both for redundancy).
   - `## Supported Versions` — Markdown table listing currently
     supported release lines. Initial table content: latest minor
     line is "Supported"; older lines are "Best-effort"; pre-1.0
     versions noted as "no LTS guarantee, latest tag only".
   - `## Disclosure Timeline` — indicative SLA: acknowledgement
     within 5 business days; triage + fix target within 30 days for
     high/critical, 90 days for low/medium; coordinated public
     disclosure after a fixed release is available, with credit to
     the reporter if requested.
2. Finalize `.mindspec/docs/adr/ADR-0029-supply-chain-attestations.md`
   to the **as-planned** state. The ADR's frontmatter already reads
   Status=Accepted, but the body's Status section explicitly calls
   itself a stub; rewrite the Status section to read "Finalized" and
   inline the planned signs:/sboms: stanza shapes verbatim from
   Bead 2 step 3 below (the `cmd: cosign … artifacts: archive` +
   `artifacts: checksum` shape; the `cmd: syft … artifacts: archive`
   + `artifacts: checksum` shape; the `--bundle=${artifact}.cosign.bundle`
   flag). The ADR documents the **planned** design, not a
   post-implementation reconciliation — Bead 1 has no temporal
   dependency on Beads 2-3. If Beads 2-3 reveal a minor inaccuracy at
   implementation time (e.g., a third-party action's pinned SHA
   shifts, or GoReleaser's snapshot-mode override flag changes), a
   small fixup commit amends the ADR; this is not a re-finalization
   and does not unblock or block any bead.
3. Update the forward reference in ADR-0029 that currently reads
   "Bead N of spec 090" to name the actual beads (Bead 2 for the
   workflow + goreleaser changes; Bead 3 for the dryrun gate).
4. Verify no other ADR (ADR-0025, ADR-0027, ADR-0028) contains stale
   references that need updating to mention ADR-0029. Grep:
   `grep -l '0029' .mindspec/docs/adr/` returns at minimum the new
   ADR-0029 file; no edits to prior ADRs are required by this spec.

**Verification**
- [ ] `test -f SECURITY.md` succeeds.
- [ ] `grep -Eq '^##\s+Reporting a Vulnerability' SECURITY.md` succeeds.
- [ ] `grep -Eq '^##\s+Supported Versions' SECURITY.md` succeeds.
- [ ] `grep -Eq '^##\s+Disclosure Timeline' SECURITY.md` succeeds.
- [ ] `grep -Eqi '(security@|github\.com/.+/security/advisories)' SECURITY.md`
      succeeds (Reporting section contains a contact channel).
- [ ] `.mindspec/docs/adr/ADR-0029-supply-chain-attestations.md`
      Status section reads "Finalized" (or equivalent), not "Stub".
- [ ] ADR-0029 inlines the planned signs:/sboms: stanza shapes from
      Bead 2 step 3 (`artifacts: archive` + `artifacts: checksum`;
      `--bundle=${artifact}.cosign.bundle`):
      `grep -q 'artifacts: archive' .mindspec/docs/adr/ADR-0029-supply-chain-attestations.md`
      AND
      `grep -q 'artifacts: checksum' .mindspec/docs/adr/ADR-0029-supply-chain-attestations.md`
      both succeed.
- [ ] ADR-0029 forward references name "Bead 2" and "Bead 3" of spec
      090 (not "Bead N").
- [ ] `go build ./... && go test -short ./...` passes (trivially —
      no Go files touched).

**Acceptance Criteria**
- [ ] Spec AC "`SECURITY.md` exists at repo root and contains all
      three required section headings … The Reporting section links
      to either a GitHub Security Advisory URL or a security contact
      email (or both)" is satisfied.
- [ ] Spec AC "An ADR named `ADR-<NNNN>-supply-chain-attestations.md`
      exists under `.mindspec/docs/adr/` … and records the
      keyless-via-OIDC choice, the GoReleaser-native integration path,
      and the rollback procedure" is satisfied (NNNN = 0029 as
      reserved; the spec's placeholder caveat about renumbering if
      0029 is taken at PR-open does not apply because ADR-0029 is
      already present in this branch).
- [ ] Spec Requirement #1 (`SECURITY.md` exists at repo root with
      three required sections) is satisfied.

**Depends on**
None.

## Bead 2: release.yml + .goreleaser.yml — cosign keyless signing + syft SBOM via GoReleaser-native config

The substantive pipeline change. Expands `.github/workflows/release.yml`
to install cosign and syft and to grant the OIDC token, then routes
all actual signing and SBOM generation through GoReleaser's native
`signs:` and `sboms:` blocks added to `.goreleaser.yml`. Single-bead
landing of both files because they are coupled — splitting risks an
intermediate state where the workflow installs the tools but
goreleaser doesn't invoke them, or vice versa.

**Steps**

1. **Pre-state (verified at planning time, stated here as fact, not a
   conditional):** `.goreleaser.yml` exists at the repo root on the F6
   worktree. It is at `version: 2`, builds `./cmd/mindspec` for
   linux/darwin/windows × amd64/arm64 with `CGO_ENABLED=0`, archives
   as `mindspec_{{.Version}}_{{.Os}}_{{.Arch}}` (tar.gz default; zip
   for windows via `format_overrides`), and emits `checksums.txt` via
   `checksum.name_template`. It has NO `signs:` and NO `sboms:`
   top-level blocks. `.github/workflows/release.yml` exists with
   `permissions: contents: write` only (no `id-token: write`), uses
   floating tags `actions/checkout@v4`, `actions/setup-go@v5`,
   `goreleaser/goreleaser-action@v6`, and runs `args: release --clean`
   with `version: "~> v2"`. Bead 2 **amends** these two files in
   place; it does NOT create either file from scratch.
2. Edit `.github/workflows/release.yml`:
   - **Expand the `permissions:` block** to include
     `id-token: write` **alongside** the existing `contents: write`.
     The implementer MUST NOT replace the existing `contents: write`
     line; the spec is explicit (Requirement #2: "preserved, required
     by GoReleaser's release upload"). The final block reads:
     ```
     permissions:
       contents: write
       id-token: write
     ```
   - **Add a `sigstore/cosign-installer` step** before the GoReleaser
     step, pinned to a 40-char commit SHA (resolved at
     implementation time from
     `https://github.com/sigstore/cosign-installer/releases`; current
     latest as of plan drafting is `v3.x`, but the implementer
     pins the SHA of whatever release is current at PR-open).
   - **Add a syft installer step** — `anchore/sbom-action/download-syft`
     pinned to a 40-char SHA. This installs the `syft` binary on the
     runner so GoReleaser's `sboms:` block can invoke it.
   - **Re-pin `actions/checkout`, `actions/setup-go`, and
     `goreleaser/goreleaser-action`** from their current floating
     tags (`@v4`, `@v5`, `@v6`) to 40-char SHAs. Every `uses:` line
     in the file MUST match `@[0-9a-f]{40}\b` after this edit.
   - The `env:` block on the GoReleaser step keeps
     `GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}` unchanged; no
     additional secrets are introduced (cosign keyless uses the
     OIDC token via `id-token: write`, not a stored secret).
3. Append two top-level blocks to `.goreleaser.yml` (in this exact
   YAML shape — inlined here so the implementer has no schema
   ambiguity). Note: GoReleaser substitutes `${artifact}` literally
   inside both `signs:` and `sboms:` blocks. The spec's
   `{{ .Env.artifact }}` text (spec.md line 57) is illustrative
   prose, NOT literal YAML — use `${artifact}` verbatim in `args`,
   `signature`, `certificate`, and `documents` fields.

   **`signs:` block** — two entries, one for archives, one for the
   checksums file. The `artifacts:` enum is `archive | binary |
   source | package | checksum | any`; use `archive` and `checksum`
   (NOT `all` or `any` + glob — those are either invalid or fragile
   per GoReleaser's documented schema). The `--bundle` flag is
   required by spec Requirement #2 so each artifact gets a matching
   `.cosign.bundle`.

   ```yaml
   signs:
     - id: archives
       cmd: cosign
       signature: "${artifact}.sig"
       certificate: "${artifact}.pem"
       args:
         - "sign-blob"
         - "--yes"
         - "--bundle=${artifact}.cosign.bundle"
         - "--output-signature=${artifact}.sig"
         - "--output-certificate=${artifact}.pem"
         - "${artifact}"
       artifacts: archive
     - id: checksums
       cmd: cosign
       signature: "${artifact}.sig"
       certificate: "${artifact}.pem"
       args:
         - "sign-blob"
         - "--yes"
         - "--bundle=${artifact}.cosign.bundle"
         - "--output-signature=${artifact}.sig"
         - "--output-certificate=${artifact}.pem"
         - "${artifact}"
       artifacts: checksum
   ```

   **`sboms:` block** — two entries, mirroring the `signs:` shape.
   `artifacts: checksum` is the documented schema for targeting
   `checksums.txt` (R1:C1, R2:C3, R4:C3, R5:C2, R6:C1 panel
   consensus); the previous draft's `artifacts: any` + glob is
   replaced because GoReleaser's `sboms:` block has no native glob
   include mechanism.

   ```yaml
   sboms:
     - id: archives
       cmd: syft
       args:
         - "scan"
         - "${artifact}"
         - "-o"
         - "spdx-json=${document}"
       documents:
         - "${artifact}.spdx.json"
       artifacts: archive
     - id: checksums
       cmd: syft
       args:
         - "scan"
         - "${artifact}"
         - "-o"
         - "spdx-json=${document}"
       documents:
         - "${artifact}.spdx.json"
       artifacts: checksum
   ```
4. Run `actionlint` against the edited `release.yml`:
   `docker run --rm -v "$(pwd):/repo" -w /repo
   rhysd/actionlint@sha256:<digest> -color
   .github/workflows/release.yml`. The pinned digest is recorded
   in the bead's commit message. Zero lint findings required.
5. Run `goreleaser check --config .goreleaser.yml` locally to verify
   the config parses (this command does not require cosign/syft
   binaries; it only validates schema). Zero findings required.
6. Local snapshot smoke (manual; recorded in the bead's commit
   message — not a CI gate): `goreleaser release --snapshot
   --skip=publish --clean` against a checkout. Inspect `dist/` and
   confirm the archives are produced. **Note:** per
   https://goreleaser.com/customization/sign/, GoReleaser skips
   `signs:` (and historically `sboms:`) in `--snapshot` mode by
   default, so the local smoke confirms (a) the config parses, (b)
   archives are produced, and (c) the `signs:` / `sboms:` blocks do
   NOT cause goreleaser to error out at parse or run time. End-to-end
   verification that cosign/syft actually produce side-cars is done
   by Bead 3's dryrun, which invokes them manually (Bead 3 step 2,
   Block A).
7. Run the SHA-pin grep contract from spec Validation Proofs — BOTH
   halves (exclusion + inclusion, per panel revision 7):
   - **Exclusion grep** (zero output required):
     `grep -hE '^\s*-?\s*uses:' .github/workflows/release*.yml | grep
     -vE '@[0-9a-f]{40}\b'`. Asserts no floating tag survives.
   - **Inclusion grep** (asserts SHA *form*, not just tag absence):
     compute `N=$(grep -cE '^\s*-?\s*uses:' .github/workflows/release.yml)`
     (the number of `uses:` lines in `release.yml`), then assert
     `[ "$(grep -cE '@[a-f0-9]{40}' .github/workflows/release.yml)" -ge "$N" ]`.
     Record N in the bead's commit message. At this bead only
     `release.yml` exists in the `release*.yml` glob; Bead 3 re-runs
     the same paired check across both files with the corresponding
     N for `release-dryrun.yml`.
8. Run `go build ./... && go test -short ./...`. Trivially green
   (no Go files modified).

**Verification**
- [ ] `grep -Eq '^\s*contents:\s*write' .github/workflows/release.yml`
      succeeds (existing line preserved).
- [ ] `grep -Eq '^\s*id-token:\s*write' .github/workflows/release.yml`
      succeeds (new line added).
- [ ] `grep -q 'sigstore/cosign-installer' .github/workflows/release.yml`
      succeeds.
- [ ] `grep -q 'anchore/sbom-action' .github/workflows/release.yml`
      succeeds (or the equivalent syft installer the implementer
      chose, documented in the commit message).
- [ ] SHA-pin **exclusion** grep
      `grep -hE '^\s*-?\s*uses:' .github/workflows/release.yml |
      grep -vE '@[0-9a-f]{40}\b'` produces no output.
- [ ] SHA-pin **inclusion** grep: with
      `N=$(grep -cE '^\s*-?\s*uses:' .github/workflows/release.yml)`,
      the count `grep -cE '@[a-f0-9]{40}' .github/workflows/release.yml`
      is `>= N`. N is recorded in the bead's commit message.
- [ ] `.goreleaser.yml` exists and contains a `signs:` top-level key
      and a `sboms:` top-level key.
- [ ] `grep -q 'cosign.bundle' .goreleaser.yml` succeeds (the
      explicit `--bundle=${artifact}.cosign.bundle` arg is present).
- [ ] `grep -q 'spdx-json' .goreleaser.yml` succeeds (the syft
      `-o spdx-json=…` arg is present).
- [ ] Both archive AND checksum coverage in `signs:` and `sboms:`
      (panel revision 1): `grep -c 'artifacts: archive' .goreleaser.yml`
      is `>= 2` AND `grep -c 'artifacts: checksum' .goreleaser.yml`
      is `>= 2` (each block has one entry per artifact class).
- [ ] `goreleaser check --config .goreleaser.yml` exits 0.
- [ ] `actionlint` against `.github/workflows/release.yml` exits 0.
- [ ] Local snapshot smoke (step 6) ran `goreleaser release
      --snapshot --skip=publish --clean` to completion with archives
      in `dist/` and no parse/run errors from the `signs:`/`sboms:`
      blocks; result recorded in commit message. Side-car presence is
      asserted by Bead 3's dryrun (not here, because `--snapshot`
      skips `signs:` by default).
- [ ] `go build ./... && go test -short ./...` passes.

**Acceptance Criteria**
- [ ] Spec AC "On a release tag (`v*`), `.github/workflows/release.yml`
      produces signed archive assets that pass `cosign verify-blob
      --certificate-identity-regexp … --certificate-oidc-issuer
      https://token.actions.githubusercontent.com` for every archive
      in `dist/`" is satisfied (verified post-tag; the workflow
      machinery is in place after this bead).
- [ ] Spec AC "On a release tag, GoReleaser's `sboms:` block
      produces an SPDX-JSON SBOM attached to the release as
      `<archive>.spdx.json` for every archive asset … and for
      `checksums.txt`" is satisfied.
- [ ] Spec AC "`gh release view <tag> --json assets` shows, for
      every archive asset, a matching `.sig`, a cosign bundle
      (`.cosign.bundle`, produced via explicit `--bundle <path>` in
      the GoReleaser `signs:` block), and a `.spdx.json` SBOM" is
      satisfied (post-tag observable; the `--bundle` flag is wired
      in the `signs:` stanza per step 3).
- [ ] Spec AC "Every `uses:` line in `.github/workflows/release.yml`
      and `.github/workflows/release-dryrun.yml` references a
      40-character commit SHA" is satisfied for `release.yml`
      (Bead 3 covers `release-dryrun.yml`).
- [ ] Spec Requirement #2 (cosign keyless on `release.yml` with both
      `contents: write` and `id-token: write` permissions) is
      satisfied.
- [ ] Spec Requirement #3 (syft `sboms:` stanza emits SPDX-JSON per
      archive asset including `checksums.txt`) is satisfied.
- [ ] Spec Requirement #5 (SHA-pinning enforced for all third-party
      actions in `release.yml`) is satisfied.
- [ ] Spec Requirement #6 (Go test suite remains green) holds.

**Depends on**
None. (Independent of Bead 1.)

## Bead 3: release-dryrun.yml — snapshot run + structural verify on every PR that touches release infra

Adds the PR-time gate. The new workflow self-triggers on the PR that
introduces it (because that PR modifies `release.yml`,
`.goreleaser.yml`, `SECURITY.md`, and `release-dryrun.yml` itself),
which provides the first integration test of Bead 2's pipeline
output. Same-repo gate per spec Requirement #4 — fork PRs cannot
mint a usable OIDC token and are explicitly out of scope.

**Steps**

1. Create `.github/workflows/release-dryrun.yml`:
   - `name: Release Dryrun`
   - **Trigger is `on: pull_request` (NOT `pull_request_target`).**
     This means the PR-supplied workflow YAML executes with PR-context
     permissions; the OIDC token cannot satisfy the production
     identity regex anyway, so PR-head checkout is safe under
     `pull_request`. Using `pull_request_target` here would be a
     supply-chain vulnerability (panel revision 8, R6:C2): a same-repo
     collaborator could exfiltrate the OIDC token via a malicious
     edit to the dryrun YAML itself. Inclusive `paths:` list (panel
     revision 6 — enumerated literals; GitHub Actions does not glob
     across sibling files without `**`):
     ```yaml
     on:
       pull_request:
         paths:
           - .github/workflows/release.yml
           - .github/workflows/release-dryrun.yml
           - .goreleaser.yml
           - SECURITY.md
           - .github/actions/**
     ```
     Any future `release-*.yml` sibling workflow added later requires
     an explicit `paths:` addition — note this as a leading comment
     in the workflow file.
   - Job-level same-repo gate (placed at the JOB level, not step
     level — so `id-token: write` is never exposed to fork PRs):
     `if: github.event.pull_request.head.repo.full_name == github.repository`
     (verbatim per spec Requirement #4).
   - Permissions: `contents: read` and `id-token: write` (the dryrun
     needs OIDC so the parallel cosign sign-blob step in step 2 can
     run end-to-end against snapshot artifacts; the cert won't
     satisfy the production identity regex, but the **signing
     operation** itself must succeed for the assertions to be
     meaningful).
   - Runner: `ubuntu-latest`.
   - Steps:
     - `actions/checkout@<sha>` (fetch-depth: 0 for goreleaser; uses
       the default ref — PR-head SHA — which is safe under
       `pull_request` per the trigger-event note above).
     - `actions/setup-go@<sha>` with go-version pinned to whatever
       version is currently in `release.yml` (today: `"1.22"`). To
       prevent silent drift the version MUST match `release.yml`;
       the workflow includes a comment line
       `# go-version: must match release.yml's setup-go input` to
       make the coupling explicit.
     - `sigstore/cosign-installer@<sha>` (same SHA as `release.yml`).
     - `anchore/sbom-action/download-syft@<sha>` (same SHA as
       `release.yml`).
     - `goreleaser/goreleaser-action@<sha>` with
       `args: release --snapshot --skip=publish --clean` and the
       same `~> v2` version pin as `release.yml`. Env:
       `GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}`. **Important:**
       per https://goreleaser.com/customization/sign/, GoReleaser
       skips the `signs:` block by default under `--snapshot` (and
       historically also `sboms:` in earlier versions). The dryrun
       therefore CANNOT rely on GoReleaser to invoke cosign/syft
       during snapshot. Step 2 below runs cosign and syft manually
       against the snapshot archives.
     - Manual sign + SBOM step (panel revision 3) — runs after
       GoReleaser produces archives; the inline `run:` block in
       step 2 below.
     - Assert step (separate inline `run:` block) — the assertion
       script in step 2.
2. Two inline `run:` blocks follow the GoReleaser step:

   **Block A — Manual sign + SBOM (panel revision 3).** Because
   `goreleaser --snapshot` skips the `signs:` block by default
   (documented at https://goreleaser.com/customization/sign/), the
   dryrun invokes cosign and syft directly against the snapshot
   archives. `--tlog-upload=false` is mandatory (panel revision 3,
   R5:C5) so PR-context Fulcio certificates do not pollute the
   public Rekor transparency log. Env: `COSIGN_EXPERIMENTAL: "1"`,
   `COSIGN_YES: "true"` set on this step. Script (literal `run:`
   content):
   ```bash
   set -euo pipefail
   shopt -s nullglob
   artifacts=( dist/mindspec_*.tar.gz dist/mindspec_*.zip dist/checksums.txt )
   for artifact in "${artifacts[@]}"; do
     [ -f "$artifact" ] || continue
     cosign sign-blob \
       --yes \
       --tlog-upload=false \
       --bundle="${artifact}.cosign.bundle" \
       --output-signature="${artifact}.sig" \
       --output-certificate="${artifact}.pem" \
       "$artifact"
     syft scan "$artifact" -o spdx-json="${artifact}.spdx.json"
   done
   ```

   **Block B — Assertion script (covers spec Requirement #4 a–c).**
   Mirrors the spec's Validation Proofs dryrun stanza, extended to
   include `checksums.txt` per spec Requirement #3 (note: this is
   strictly stricter than the spec's example loop, which iterates
   only over archives; the divergence is intentional and matches the
   spec's broader coverage requirement):
   ```bash
   set -euo pipefail
   shopt -s nullglob
   archives=( dist/mindspec_*.tar.gz dist/mindspec_*.zip dist/checksums.txt )
   if [ ${#archives[@]} -eq 0 ]; then
     echo "no archives found in dist/ — goreleaser snapshot produced nothing"
     exit 1
   fi
   for archive in "${archives[@]}"; do
     [ -f "$archive" ] || continue
     [ -s "${archive}.sig" ]           || { echo "missing ${archive}.sig"; exit 1; }
     [ -s "${archive}.cosign.bundle" ] || { echo "missing ${archive}.cosign.bundle"; exit 1; }
     [ -s "${archive}.spdx.json" ]     || { echo "missing ${archive}.spdx.json"; exit 1; }
     jq -e '.spdxVersion' "${archive}.spdx.json" >/dev/null \
       || { echo "invalid SPDX in ${archive}.spdx.json"; exit 1; }
   done
   echo "release-dryrun: $(printf '%s\n' "${archives[@]}" | wc -l) artifacts verified"
   ```
   Because the manual sign step in Block A iterates the same
   `artifacts` list, the assertion step can never observe a missing
   side-car except via a real cosign/syft exit-nonzero failure (which
   `set -euo pipefail` in Block A would have already aborted on).
   The "the dryrun fail is the forcing function" debug loop from the
   round-1 draft is no longer needed — the manual invocation
   guarantees the side-cars exist as long as cosign + syft succeed.
3. The dryrun does NOT call `cosign verify-blob` with a pinned
   identity regex. Per the spec's Testing Strategy rationale: the
   snapshot OIDC subject is `…/release-dryrun.yml@refs/pull/<N>/merge`,
   which cannot satisfy
   `^https://github\.com/mrmaxsteel/mindspec/\.github/workflows/release\.yml@refs/tags/v.*$`.
   Identity-bound verification runs only on the post-tag path and is
   the reviewer-runnable command in spec Validation Proofs.
4. SHA-pin all `uses:` lines in `release-dryrun.yml`. The SHAs MUST
   match the corresponding SHAs in `release.yml` (drift between the
   two would mean the dryrun is testing a different toolchain than
   the production release; a Renovate rule, if added later, must
   rotate both files together — out of scope for this spec, but
   noted in the workflow's leading comment).
5. Run actionlint:
   `docker run --rm -v "$(pwd):/repo" -w /repo
   rhysd/actionlint@sha256:<digest> -color
   .github/workflows/release-dryrun.yml`. Zero findings required.
6. Re-run the SHA-pin grep contract across BOTH workflow files
   (panel revision 7 — both halves):
   - **Exclusion**: `grep -hE '^\s*-?\s*uses:' .github/workflows/release*.yml | grep
     -vE '@[0-9a-f]{40}\b'`. Zero output required.
   - **Inclusion**: for each file F in
     `.github/workflows/release.yml` and
     `.github/workflows/release-dryrun.yml`, with
     `N=$(grep -cE '^\s*-?\s*uses:' F)`,
     assert `grep -cE '@[a-f0-9]{40}' F` is `>= N`.
   Both Ns are recorded in the bead's commit message.
7. Open the PR that introduces all three changes (Beads 1+2+3 may
   land in one PR or in separate PRs; if separate, this bead's PR
   triggers the dryrun against the already-merged Bead 2 changes).
   Confirm the dryrun job runs and passes. Record the workflow-run
   URL in the bead's commit message as the integration-test proof.
8. Run `go build ./... && go test -short ./...`. Trivially green.

**Verification**
- [ ] `test -f .github/workflows/release-dryrun.yml` succeeds.
- [ ] `grep -q "github.event.pull_request.head.repo.full_name == github.repository"
      .github/workflows/release-dryrun.yml` succeeds (same-repo gate
      present verbatim).
- [ ] `grep -Eq '^\s*id-token:\s*write' .github/workflows/release-dryrun.yml`
      succeeds.
- [ ] `grep -q 'release --snapshot' .github/workflows/release-dryrun.yml`
      succeeds (or the equivalent goreleaser-action `args:` form).
- [ ] `grep -q 'spdxVersion' .github/workflows/release-dryrun.yml`
      succeeds (SPDX validity check is present).
- [ ] Manual cosign + syft step is present:
      `grep -q 'cosign sign-blob' .github/workflows/release-dryrun.yml`
      AND `grep -q 'tlog-upload=false' .github/workflows/release-dryrun.yml`
      AND `grep -q 'syft scan' .github/workflows/release-dryrun.yml`
      all succeed.
- [ ] **Trigger event is `pull_request` (NOT `pull_request_target`)**
      — panel revision 8:
      `grep -q '^\s*pull_request:' .github/workflows/release-dryrun.yml`
      succeeds AND
      `! grep -q 'pull_request_target' .github/workflows/release-dryrun.yml`
      succeeds.
- [ ] Workflow trigger `paths:` list contains all five required
      entries (panel revision 6): `release.yml`, `release-dryrun.yml`,
      `.goreleaser.yml`, `SECURITY.md`, `.github/actions/**`.
- [ ] SHA-pin **exclusion** grep across both workflow files
      (`grep -hE '^\s*-?\s*uses:' .github/workflows/release*.yml |
      grep -vE '@[0-9a-f]{40}\b'`) produces no output.
- [ ] SHA-pin **inclusion** grep on `release-dryrun.yml` (panel
      revision 7): with
      `N=$(grep -cE '^\s*-?\s*uses:' .github/workflows/release-dryrun.yml)`,
      `grep -cE '@[a-f0-9]{40}' .github/workflows/release-dryrun.yml`
      is `>= N`. N is recorded in the bead's commit message.
- [ ] `actionlint` against `release-dryrun.yml` exits 0.
- [ ] The dryrun job on the PR that introduces this file runs to
      completion and asserts `.sig` / `.cosign.bundle` / `.spdx.json`
      presence for every archive in `dist/`; workflow-run URL
      recorded in commit message.
- [ ] `go build ./... && go test -short ./...` passes.

**Acceptance Criteria**
- [ ] Spec AC "The `release-dryrun` CI job (runs on **same-repo**
      PRs that modify `.github/workflows/release*.yml`,
      `.github/workflows/release-dryrun.yml`, `.github/actions/**`,
      `.goreleaser.yml`, or `SECURITY.md`) fails if: cosign or syft
      commands return non-zero, any expected `.sig` /
      `.cosign.bundle` / `.spdx.json` output file is missing or
      zero-byte, or any SBOM fails `jq -e '.spdxVersion'`. The
      dryrun does NOT run `cosign verify-blob` against snapshot
      artifacts" is satisfied verbatim.
- [ ] Spec AC "Every `uses:` line in `.github/workflows/release.yml`
      and `.github/workflows/release-dryrun.yml` references a
      40-character commit SHA, not a floating tag" is satisfied
      (Bead 2 covered `release.yml`; this bead covers
      `release-dryrun.yml` and re-runs the cross-file grep).
- [ ] Spec Requirement #4 (release-dryrun CI job exists, same-repo
      gated, runs snapshot + asserts file presence + SPDX validity,
      does NOT do identity-bound cosign verify) is satisfied.
- [ ] Spec Requirement #5 (SHA-pinning enforced) is satisfied for
      both workflow files.
- [ ] Spec Requirement #6 (Go test suite remains green) holds.
- [ ] Spec Requirement #7 (no new runtime dependencies; cosign and
      syft are CI-only) is satisfied — both tools are installed
      only by `uses:` steps in `release.yml` / `release-dryrun.yml`,
      not added to `go.mod`.

**Depends on**
Bead 2. (The dryrun has nothing to verify until `.goreleaser.yml`
emits `.sig` / `.cosign.bundle` / `.spdx.json` files.)

## Provenance

| Acceptance Criterion (from spec) | Verified By |
|---|---|
| `SECURITY.md` exists at repo root with all three required section headings; Reporting section links to a GitHub Security Advisory URL or security contact email | Bead 1 verification (grep contract from spec Validation Proofs) |
| On a release tag (`v*`), `release.yml` produces signed archive assets that pass `cosign verify-blob` with the pinned identity regex `^https://github\.com/mrmaxsteel/mindspec/\.github/workflows/release\.yml@refs/tags/v.*$` and OIDC issuer `https://token.actions.githubusercontent.com` for every archive in `dist/` | Bead 2 (machinery in place — `permissions: id-token: write`, cosign-installer, GoReleaser `signs:` block with `--bundle`); post-tag observable, runnable per spec Validation Proofs |
| On a release tag, GoReleaser's `sboms:` block produces an SPDX-JSON SBOM attached as `<archive>.spdx.json` for every archive asset and for `checksums.txt` | Bead 2 (GoReleaser `sboms:` block with two entries: `artifacts: archive` and `artifacts: checksum` — panel revision 1); post-tag observable; structurally verified per-PR by Bead 3's dryrun (which runs syft manually because `--snapshot` skips `signs:`/`sboms:` by default — panel revision 3) |
| `release-dryrun` CI job (same-repo PRs, paths-gated) fails if cosign/syft non-zero, missing/zero-byte side-car file, or invalid SPDX | Bead 3 (workflow + assertion script; runs against Bead 2's output) |
| `gh release view <tag> --json assets` shows matching `.sig`, `.cosign.bundle`, `.spdx.json` per archive | Bead 2 (GoReleaser config emits all three with explicit `--bundle` arg); post-tag observable per spec Validation Proofs `gh release view` jq command |
| Every `uses:` line in `release.yml` and `release-dryrun.yml` is SHA-pinned (40-char hex); grep produces no output | Bead 2 (release.yml SHA-pin pass) + Bead 3 (release-dryrun.yml SHA-pin + cross-file grep verification) |
| `go build ./...` and `go test -short ./...` remain green on the F6 branch | Every bead verification block (trivially green by construction — no Go files touched; HC-3, HC-5, HC-6) |
| ADR-0029-supply-chain-attestations.md exists, Status=Accepted, records keyless-via-OIDC choice, GoReleaser-native integration path, rollback procedure | Bead 1 (finalizes stub to as-planned design — panel revision 5; no temporal dependency on Beads 2-3, optional fixup commit if implementation reveals a minor inaccuracy) |
