---
approved_at: "2026-05-20T18:06:31Z"
approved_by: user
status: Approved
---
# Spec 090-production-hygiene: Production hygiene: SECURITY.md + cosign signing + syft SBOM

## Goal

Bring the mindspec release pipeline up to the supply-chain hygiene bar that
is now standard for OSS Go tooling: the repo ships a `SECURITY.md` with a
clear vulnerability reporting channel; the existing GoReleaser-driven
`.github/workflows/release.yml` gains **cosign keyless signing (GitHub
OIDC)** for every release artifact and **syft-generated SPDX SBOMs**
attached as release assets; and a `release-dryrun` CI job verifies cosign
**signatures** and SBOM presence on every PR that touches the release
workflows, so a broken release pipeline cannot reach a tagged release.

This is F6 from the converged transformation plan. It is intentionally a
**CI/release-only** change: no mindspec-internal Go code is modified, no
new runtime dependency is introduced (cosign and syft run only in CI),
and the feature has no code coupling with F1–F5 — it can land
independently and in parallel.

## Background

The wider transformation plan calls for landing the supply-chain hygiene
gates that are already standard in the user's other repo (gascity), whose
`SECURITY.md` and signed release pipeline serve as the design inspiration.
If the gascity templates (`/tmp/gascity/SECURITY.md`,
`/tmp/gascity/.github/workflows/release.yml`) are reachable from the
implementation environment, they SHOULD be used verbatim per plan F6
guidance. If they are not, the implementer re-derives equivalent YAML
from upstream READMEs (`sigstore/cosign-installer`, `anchore/sbom-action`,
`goreleaser/goreleaser-action`) and from GoReleaser's `signs:` / `sboms:`
documentation. The acceptance criteria below are independent of which
source the YAML is drawn from — they assert outcomes, not implementation
provenance.

## Impacted Domains

- **`SECURITY.md`** (new, at repo root): Vulnerability reporting policy
  with a single canonical reporting channel (GitHub Security Advisory
  preferred, email fallback), supported versions table, and disclosure
  timeline.
- **`.github/workflows/release.yml`** (modified): Existing GoReleaser
  job adds `sigstore/cosign-installer` and `anchore/syft` install steps,
  expands the `permissions:` block to include `id-token: write` alongside
  the existing `contents: write`, and delegates the actual signing and
  SBOM generation to GoReleaser's native `signs:` and `sboms:` blocks
  (added to `.goreleaser.yml`). Signatures (`*.sig`), cosign bundles
  (`*.cosign.bundle`, produced via explicit `--bundle <path>`), and
  SBOMs (`*.spdx.json`) are emitted by GoReleaser as release assets
  alongside the archive binaries.
- **`.goreleaser.yml`** (modified): Adds `signs:` block invoking cosign
  keyless against every archive asset with `--bundle
  {{ .Env.artifact }}.cosign.bundle`, and `sboms:` block invoking syft
  to produce SPDX-JSON per archive. The release artifacts being signed
  and SBOMed are the GoReleaser-produced archives
  (`mindspec_<version>_<os>_<arch>.tar.gz` / `.zip`) and
  `checksums.txt`, not raw binaries.
- **`.github/workflows/release-dryrun.yml`** (new): Runs on
  same-repo PRs (gated by
  `if: github.event.pull_request.head.repo.full_name == github.repository`)
  that modify `.github/workflows/release*.yml`,
  `.github/workflows/release-dryrun.yml`, `.github/actions/**`,
  `.goreleaser.yml`, or `SECURITY.md`. Executes
  `goreleaser release --snapshot --skip=publish`, then asserts that
  cosign and syft exited 0, that the expected `.sig`, `.cosign.bundle`,
  and `.spdx.json` output files exist with non-zero size, and that the
  SBOM is valid SPDX JSON (`jq -e '.spdxVersion'`). The dryrun does
  **not** attempt `cosign verify-blob` against snapshot artifacts —
  identity-bound verification is deferred to the post-tag release path
  because a non-tag PR-context OIDC token cannot satisfy the pinned
  Fulcio identity regex.

No files under `cmd/`, `internal/`, `go.mod`, or `go.sum` are touched.

## ADR Touchpoints

- [ADR-0029-supply-chain-attestations.md](../../adr/ADR-0029-supply-chain-attestations.md)
  (**new**): Records the choice of **cosign keyless signing via GitHub
  OIDC** over the alternatives (key-based cosign, SLSA-framework
  provenance, in-toto attestations beyond cosign defaults). Records the
  choice of **syft SPDX-JSON** as the SBOM format. Includes the
  **rollback procedure**: revert the release workflow change in a
  single commit; the prior unsigned release path is preserved in git
  history and can be re-tagged with no schema change.
- [ADR-0025-jsonl-as-build-artifact.md](../../adr/ADR-0025-jsonl-as-build-artifact.md):
  Cross-referenced as the most recent ADR touching release artifact
  conventions. F6 leaves the JSONL build-artifact contract unchanged;
  signatures/SBOMs are added alongside, not in place of, existing assets.
- [ADR-0027-mindspec-otel-only.md](../../adr/ADR-0027-mindspec-otel-only.md)
  and [ADR-0028-bench-rescue-procedure.md](../../adr/ADR-0028-bench-rescue-procedure.md):
  Cross-referenced as the most recent CI-touching ADRs; F6 does not
  alter any decisions recorded in either.

No existing ADR is amended; F6 only adds ADR-0029.

> **ADR number is a placeholder.** F6 is explicitly parallelizable with
> F1–F5; a sibling spec may claim 0029 first. The implementer MUST
> re-check `.mindspec/docs/adr/` at PR-open time and renumber to the
> next free integer if 0029 is taken.

## Requirements

1. **`SECURITY.md` exists at repo root** with three required sections:
   (a) a `## Reporting a Vulnerability` section that lists a GitHub
   Security Advisory URL (`https://github.com/mrmaxsteel/mindspec/security/advisories/new`)
   and/or a security contact email; (b) a `## Supported Versions` section
   containing a Markdown table of currently-supported release lines; and
   (c) a `## Disclosure Timeline` section stating an indicative response /
   fix / public-disclosure SLA. All three section headings MUST be
   present so the AC grep cannot be satisfied by a one-line file.
2. **`.github/workflows/release.yml` runs cosign keyless signing
   (GitHub OIDC)** against every archive asset produced by GoReleaser.
   The workflow's `permissions:` block MUST be expanded to include
   **both** `contents: write` (preserved, required by GoReleaser's
   release upload) **and** `id-token: write` (new, required by cosign
   for OIDC token minting). The implementer MUST NOT replace the
   existing block. Signing is driven by a `signs:` stanza in
   `.goreleaser.yml` that invokes cosign with explicit
   `--bundle {{ .Env.artifact }}.cosign.bundle` so each archive gets a
   matching `.sig` and `.cosign.bundle` pair.
3. **`.goreleaser.yml` runs syft** via a `sboms:` stanza to produce an
   **SPDX-JSON SBOM per archive asset**, named `<archive>.spdx.json`.
   "Per archive asset" means each `mindspec_<version>_<os>_<arch>.tar.gz`
   (and `.zip` for Windows) plus `checksums.txt`. The raw extracted
   binary is not separately SBOMed.
4. **A `release-dryrun` CI job verifies cosign signatures and SBOM
   presence before a tagged release is allowed.** The job runs only on
   **same-repo** PRs (gated by
   `if: github.event.pull_request.head.repo.full_name == github.repository`)
   that modify any of: `.github/workflows/release*.yml`,
   `.github/workflows/release-dryrun.yml`, `.github/actions/**`,
   `.goreleaser.yml`, `SECURITY.md`. It runs
   `goreleaser release --snapshot --skip=publish`, then asserts:
   (a) `cosign` and `syft` processes returned exit 0, (b) the expected
   `.sig`, `.cosign.bundle`, and `.spdx.json` files exist with non-zero
   size for every archive asset under `dist/`, (c) every SBOM parses as
   valid SPDX JSON (`jq -e '.spdxVersion'` exits 0). The dryrun does
   **not** attempt `cosign verify-blob` against snapshot artifacts;
   identity-bound verification only runs in the post-tag release path
   where the OIDC subject matches the pinned Fulcio identity regex.
5. **All third-party actions in `release.yml` and `release-dryrun.yml`
   MUST be pinned to a 40-character commit SHA**, not a floating tag.
   This applies to at minimum `sigstore/cosign-installer`,
   `anchore/sbom-action` (or equivalent syft installer),
   `goreleaser/goreleaser-action`, and `actions/checkout` / `actions/setup-go`.
   Every `uses:` line in the touched workflows MUST match
   `@[0-9a-f]{40}`.
6. **The existing Go test suite remains green.** Because F6 touches no
   files under `cmd/`, `internal/`, `go.mod`, or `go.sum`, the Go test
   surface is unaffected by construction; `go test -short ./...` MUST
   exit 0 on the F6 branch.
7. **No new runtime dependencies are introduced.** Cosign and syft are
   installed only inside CI runners via pinned-SHA actions; nothing
   ships into the mindspec binary or its `go.mod` graph.
8. **F6 has no code coupling with F1–F5** and can land independently
   in any order relative to them.

## Scope

### In Scope

- `SECURITY.md` at the repo root, with the three required sections
  (Reporting, Supported Versions, Disclosure Timeline).
- `.github/workflows/release.yml` modifications: install
  `sigstore/cosign-installer` and syft, expand `permissions:` to add
  `id-token: write` alongside `contents: write`, and SHA-pin all
  third-party actions.
- `.goreleaser.yml` modifications: `signs:` block (cosign keyless with
  explicit `--bundle <path>`) and `sboms:` block (syft SPDX-JSON) so
  GoReleaser itself emits `.sig`, `.cosign.bundle`, and `.spdx.json`
  alongside each archive asset.
- `.github/workflows/release-dryrun.yml` (new): same-repo PR gate that
  runs `goreleaser --snapshot --skip=publish` and asserts file
  presence + valid SPDX JSON (no identity-bound cosign verify in
  dryrun).
- `ADR-0029-supply-chain-attestations.md` (placeholder number;
  re-check at PR open) recording the keyless-via-OIDC choice, the
  GoReleaser-native integration path, and the rollback procedure.

### Out of Scope

- **Signing of pre-release / dev / nightly binaries.** Only artifacts
  emitted on a `v*` git tag are signed in this spec; dev builds remain
  unsigned.
- **Key-based signing.** We use cosign keyless (GitHub OIDC)
  exclusively. No key generation, no key rotation, no secret storage.
- **SLSA-framework provenance** (SLSA-provenance / `slsa-github-generator`).
  Out of scope for this spec; may be a follow-up.
- **In-toto attestations beyond cosign's default** (cosign emits its
  own attestation bundle; nothing further is added here).
- **Mirroring/republishing artifacts to other registries** (Docker Hub,
  GHCR-images, package managers).
- **Any change to mindspec's Go source, `go.mod`, or runtime behavior.**

## Non-Goals

- This spec does not introduce a vulnerability-scanning gate on
  dependencies (e.g., `govulncheck` in CI). That belongs to a separate
  hygiene spec if pursued.
- This spec does not change the cadence, naming, or versioning of
  mindspec releases.

## Acceptance Criteria

- [ ] `SECURITY.md` exists at repo root and contains all three
  required section headings: `## Reporting a Vulnerability`,
  `## Supported Versions`, `## Disclosure Timeline`. The Reporting
  section links to either a GitHub Security Advisory URL or a
  security contact email (or both).
- [ ] On a release tag (`v*`), `.github/workflows/release.yml`
  produces signed archive assets that pass
  `cosign verify-blob --certificate-identity-regexp '^https://github\.com/mrmaxsteel/mindspec/\.github/workflows/release\.yml@refs/tags/v.*$' --certificate-oidc-issuer https://token.actions.githubusercontent.com`
  for every archive in `dist/`. The identity regex is pinned to this
  repo + the tag ref; a generic regex is unacceptable because it
  would accept signatures from any fork.
- [ ] On a release tag, GoReleaser's `sboms:` block produces an
  SPDX-JSON SBOM attached to the release as `<archive>.spdx.json` for
  every archive asset (`mindspec_<version>_<os>_<arch>.tar.gz` /
  `.zip`) and for `checksums.txt`.
- [ ] The `release-dryrun` CI job (runs on **same-repo** PRs that
  modify `.github/workflows/release*.yml`,
  `.github/workflows/release-dryrun.yml`, `.github/actions/**`,
  `.goreleaser.yml`, or `SECURITY.md`) fails if: cosign or syft
  commands return non-zero, any expected `.sig` / `.cosign.bundle` /
  `.spdx.json` output file is missing or zero-byte, or any SBOM fails
  `jq -e '.spdxVersion'`. The dryrun does NOT run
  `cosign verify-blob` against snapshot artifacts.
- [ ] `gh release view <tag> --json assets` shows, for every archive
  asset, a matching `.sig`, a cosign bundle (`.cosign.bundle`,
  produced via explicit `--bundle <path>` in the GoReleaser `signs:`
  block), and a `.spdx.json` SBOM.
- [ ] Every `uses:` line in `.github/workflows/release.yml` and
  `.github/workflows/release-dryrun.yml` references a 40-character
  commit SHA, not a floating tag. A grep
  `grep -E '^\s*-?\s*uses:' .github/workflows/release*.yml | grep -vE '@[0-9a-f]{40}\b'`
  produces no output.
- [ ] `go build ./...` and `go test -short ./...` remain green on the
  branch that lands F6.
- [ ] An ADR named `ADR-<NNNN>-supply-chain-attestations.md` exists
  under `.mindspec/docs/adr/` (where `NNNN` is the next free integer
  at PR-open time; 0029 is a placeholder) and records the
  keyless-via-OIDC choice, the GoReleaser-native integration path,
  and the rollback procedure.

## Validation Proofs

A reviewer (or the release-dryrun job) can establish each acceptance
criterion via concrete commands:

- **SECURITY.md present and well-formed (all three section headings):**
  ```
  test -f SECURITY.md \
    && grep -Eq '^##\s+Reporting a Vulnerability' SECURITY.md \
    && grep -Eq '^##\s+Supported Versions' SECURITY.md \
    && grep -Eq '^##\s+Disclosure Timeline' SECURITY.md \
    && grep -Eqi '(security@|github\.com/.+/security/advisories)' SECURITY.md
  ```
- **Workflow declares BOTH `contents: write` and `id-token: write`:**
  ```
  grep -Eq '^\s*contents:\s*write' .github/workflows/release.yml \
    && grep -Eq '^\s*id-token:\s*write' .github/workflows/release.yml
  ```
- **All third-party actions are SHA-pinned (no floating tags):**
  ```
  # Should produce NO output. Any line printed is a violation.
  grep -hE '^\s*-?\s*uses:' .github/workflows/release*.yml \
    | grep -vE '@[0-9a-f]{40}\b'
  ```
- **Cosign keyless verification against a tagged release archive
  (post-tag only; not run in dryrun):**
  ```
  ARCHIVE=dist/mindspec_${VERSION}_linux_amd64.tar.gz
  cosign verify-blob \
    --certificate-identity-regexp '^https://github\.com/mrmaxsteel/mindspec/\.github/workflows/release\.yml@refs/tags/v.*$' \
    --certificate-oidc-issuer https://token.actions.githubusercontent.com \
    --signature  "${ARCHIVE}.sig" \
    --bundle     "${ARCHIVE}.cosign.bundle" \
    "${ARCHIVE}"
  ```
- **Dryrun assertions (run in `release-dryrun.yml` after
  `goreleaser --snapshot --skip=publish`):**
  ```
  set -euo pipefail
  # cosign + syft non-zero exit would have failed goreleaser already.
  for archive in dist/mindspec_*.tar.gz dist/mindspec_*.zip; do
    [ -s "${archive}.sig" ]            || { echo "missing ${archive}.sig"; exit 1; }
    [ -s "${archive}.cosign.bundle" ]  || { echo "missing ${archive}.cosign.bundle"; exit 1; }
    [ -s "${archive}.spdx.json" ]      || { echo "missing ${archive}.spdx.json"; exit 1; }
    jq -e '.spdxVersion' "${archive}.spdx.json" >/dev/null \
      || { echo "invalid SPDX in ${archive}.spdx.json"; exit 1; }
  done
  ```
- **Release assets exist post-tag (every archive has .sig +
  .cosign.bundle + .spdx.json):**
  ```
  gh release view "$TAG" --json assets \
    | jq -e '
        (.assets | map(.name)) as $names
        | ($names | map(select(test("\\.tar\\.gz$|\\.zip$")))) as $archives
        | all($archives[]; . as $a |
            ($names | any(. == $a + ".sig"))            and
            ($names | any(. == $a + ".cosign.bundle"))  and
            ($names | any(. == $a + ".spdx.json")))
      '
  ```
- **Go surface unchanged:**
  ```
  go build ./... && go test -short ./...
  ```

## Hard Constraints

- **HC-1 Solo-developer UX preserved.** Tagging a release remains a
  single `git tag vX.Y.Z && git push --tags`. No additional manual
  steps, no key material to manage locally.
- **HC-2 Standalone CLI.** No additional daemons or long-lived services
  are introduced; cosign and syft are short-lived CI processes.
- **HC-3 Existing Go test suite preserved and green.** F6 touches no
  Go source; `go test -short ./...` MUST exit 0 on the F6 branch. The
  plan-level HC is "tests remain green", not a specific test count.
- **HC-4 viz / agentmind / bench excluded.** Those subsystems have been
  removed per specs 083/084; nothing in F6 re-introduces them.
- **HC-5 Each commit `go build ./... && go test -short ./...` green.**
  Trivially satisfied because no Go files are modified.
- **HC-6 No new runtime dependencies.** Cosign and syft execute only
  inside GitHub Actions runners; the mindspec binary and `go.mod` graph
  are unchanged.
- **HC-7 F6 has no code coupling with F1–F5.** It can be landed in
  parallel with, before, or after any other transformation-plan feature.

## Risks

- **Provenance provider choice (GitHub OIDC vs. Sigstore TUF).**
  Decision per the transformation plan: pick **GitHub OIDC** as the
  simplest path; no TUF root management is taken on. Recorded in
  ADR-0029.
- **Action version drift.** Promoted from a Risks-section mitigation to
  Requirement #5: every `uses:` line in the touched workflows MUST pin
  to a 40-char commit SHA, enforced by a grep in Validation Proofs.
  Floating tags are explicitly rejected.
- **Fork-PR keyless-signing gap.** A PR opened from a fork does not
  receive `id-token: write` by default and cannot mint a Fulcio
  certificate matching the pinned identity regex. Mitigation: the
  `release-dryrun` job is gated to same-repo PRs only (Requirement #4);
  fork PRs are explicitly out of scope for the dryrun gate and rely on
  human review of release-pipeline changes before merge.
- **Tag-push bypasses PR gate.** A maintainer pushing a tag directly
  (the documented HC-1 path) does not trigger the PR dryrun. This is
  accepted: the dryrun catches PR-time regressions, and branch
  protection wiring on `main` is a follow-up repo-hygiene task outside
  this spec's scope.
- **Template re-derivation.** If the gascity templates cited by plan F6
  are not reachable from the implementation environment, YAML is
  re-derived from upstream READMEs and GoReleaser docs; outcomes (ACs)
  are unchanged.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-05-20
- **Notes**: Approved via mindspec approve spec