# ADR-0029: Supply Chain Attestations via Cosign Keyless + Syft SBOM

- **Date**: 2026-05-20
- **Status**: Accepted
- **Domain(s)**: security, release, ci
- **Deciders**: Max
- **Supersedes**: n/a
- **Superseded-by**: n/a
- **Related**: [ADR-0025](ADR-0025-jsonl-as-build-artifact.md)

---

## Status

Finalized in spec 090 Bead 1 alongside the SECURITY.md drop; the
cosign + syft workflow integration ships in Bead 2 and the
`release-dryrun` verification gate ships in Bead 3. The ADR documents
the **planned** as-implemented design; Beads 2 and 3 implement it
mechanically against the YAML stanzas inlined in the Decision section
below.

## Context

The mindspec release pipeline currently publishes unsigned GoReleaser
archives with no machine-verifiable provenance. This is below the bar that
is now standard for OSS Go tooling and below the bar already met by the
user's adjacent gascity repo, whose `SECURITY.md` and signed release
pipeline are the reference design. Downstream consumers — and any future
audit — cannot today distinguish a legitimate release artifact from one
substituted in transit or at the registry.

Plan item F6 in the converged transformation plan
(`/Users/Max/replit/mindspec-transformation-plan.md` §F6) calls for
landing cosign keyless signing and a syft SBOM on the existing release
workflow, with a `release-dryrun` CI job verifying both on every PR that
touches release infrastructure. This ADR records the substantive
choices that flow from "make releases attestable" so the Bead 2 and
Bead 3 implementations are mechanical translations of an
already-decided design.

## Decision

### 1. Signing method: cosign keyless via GitHub OIDC

Releases are signed with **cosign keyless** using the GitHub Actions OIDC
token as the identity. No key material is stored in the repo or in CI
secrets; rotation is therefore a non-concern.

The Bead 2 `.goreleaser.yml` `signs:` stanza takes the following
as-planned shape, with two entries so both archives and the
`checksums.txt` line file are signed:

```yaml
signs:
  - id: archives
    cmd: cosign
    artifacts: archive
    signature: ${artifact}.sig
    args:
      - sign-blob
      - --yes
      - --output-signature=${artifact}.sig
      - --bundle=${artifact}.cosign.bundle
      - ${artifact}
  - id: checksums
    cmd: cosign
    artifacts: checksum
    signature: ${artifact}.sig
    args:
      - sign-blob
      - --yes
      - --output-signature=${artifact}.sig
      - --bundle=${artifact}.cosign.bundle
      - ${artifact}
```

The explicit `--bundle=${artifact}.cosign.bundle` flag is load-bearing:
the verification path in Bead 3 (and post-tag) consumes the
`.cosign.bundle` companion file, so its filename is pinned here.

**Rejected alternatives:**
- *Key-based cosign* — rejected for the key-management burden (secret
  storage, rotation policy, leak response) that keyless avoids entirely.
- *Unsigned releases (status quo)* — rejected because it leaves a missing
  supply-chain hygiene gate that the plan explicitly closes.

### 2. SBOM format and tool: SPDX-JSON via syft, GoReleaser-native

SBOMs are emitted as **SPDX-JSON** by **syft**, invoked through
GoReleaser's native `sboms:` directive rather than as a separate CI step.

The Bead 2 `.goreleaser.yml` `sboms:` stanza takes the following
as-planned shape — again with two entries so both archives and
`checksums.txt` receive an SBOM:

```yaml
sboms:
  - id: archives
    cmd: syft
    artifacts: archive
    documents:
      - "${artifact}.spdx.json"
    args:
      - $artifact
      - --output
      - spdx-json=${document}
  - id: checksums
    cmd: syft
    artifacts: checksum
    documents:
      - "${artifact}.spdx.json"
    args:
      - $artifact
      - --output
      - spdx-json=${document}
```

**Rejected alternatives:**
- *CycloneDX* — rejected for narrower tooling support today; SPDX-JSON is
  the more universal interchange format for the consumers we care about.
- *`anchore/sbom-action` invoked as a separate workflow step* — rejected
  in favor of GoReleaser-native invocation so one tool owns the full
  artifact set; this avoids artifact-naming drift between the release
  archives and their SBOMs.

### 3. Target artifacts: GoReleaser archives + checksums.txt

Signing and SBOM generation target the **GoReleaser-produced archives**
(`mindspec_<version>_<os>_<arch>.tar.gz` / `.zip`) plus the
`checksums.txt` file — not the raw per-OS binaries.

**Rejected alternative:**
- *Per-binary signing* — rejected because GoReleaser publishes archives,
  not raw binaries, as the release entry-point. Signing the archive
  combined with a signed `checksums.txt` transitively covers every
  binary inside, so per-binary signatures add cost without adding
  verifiable coverage.

## Consequences

- (+) Attestable supply chain: every release archive carries a verifiable
  cosign signature tied to a GitHub-issued OIDC identity.
- (+) No key-management burden — nothing to store, nothing to rotate.
- (+) Auditable via `cosign verify` against the published bundle.
- (−) All downstream verifiers need `cosign` installed (and a recent
  enough version to handle keyless bundles).
- (−) The release pipeline now depends on GitHub OIDC availability; a
  GitHub OIDC outage blocks tagged releases until it recovers.
- (−) Release CI duration increases by ~30s per release for the sign +
  SBOM steps.

## Rollback

Revert the `.github/workflows/release.yml` change (and the paired
`.goreleaser.yml` `signs:` / `sboms:` blocks) in a single commit; the
prior unsigned release path is preserved in git history and can be
re-tagged with no schema change. The cosign/syft additions are purely
**additive** to the release pipeline — removing them does not affect
the unsigned binary artifacts themselves, only the signature and SBOM
side-cars.

## Related

- [ADR-0025](ADR-0025-jsonl-as-build-artifact.md) —
  jsonl-as-build-artifact. F6 leaves the JSONL build-artifact contract
  entirely unchanged; supply-chain attestations sign the release
  archive, which contains the binary, not the bench JSONL pipeline.
- Forward reference: spec
  [`090-production-hygiene`](../specs/090-production-hygiene/spec.md) —
  the implementing spec. **Bead 1** of that spec ships the
  `SECURITY.md` drop and finalizes this ADR. **Bead 2** lands the
  `.github/workflows/release.yml` permissions expansion and the paired
  `.goreleaser.yml` `signs:` / `sboms:` stanzas inlined in the Decision
  section. **Bead 3** lands the `.github/workflows/release-dryrun.yml`
  same-repo PR gate that asserts cosign + syft artifact presence and
  SBOM validity on every PR that touches release infrastructure.
