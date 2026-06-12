# Security Policy

The mindspec maintainers take security seriously. This document explains
how to report a vulnerability, which versions receive security fixes,
and what response timeline you can expect.

## Reporting a Vulnerability

Please **do not** open a public GitHub issue for security problems.

Use one of the following private channels, in order of preference:

1. **GitHub Security Advisory** (preferred):
   <https://github.com/mrmaxsteel/mindspec/security/advisories/new>
2. **Email**: <security@cloudlete.ai> with subject line
   `mindspec security: <short description>`.

When reporting, please include:

- The mindspec version (`mindspec --version`) and OS / architecture.
- A minimal reproduction (command, config, or repository snippet).
- The observed impact and, if known, any mitigation you have applied.
- Whether you would like public credit when the fix is disclosed.

We will acknowledge receipt and assign a coordinator within the timeline
below.

## Supported Versions

mindspec has not yet cut a `1.0` release. While the project is pre-1.0,
the support policy is intentionally narrow:

| Version            | Supported          | Notes                                                  |
| ------------------ | ------------------ | ------------------------------------------------------ |
| Latest tagged `v*` | Yes                | Security fixes land on `main` and ship in a new tag.   |
| Older tagged `v*`  | Best-effort        | No backports; please upgrade to the latest tag.        |
| `main` (untagged)  | Best-effort        | No LTS commitment; security fixes land on `main` as a normal commit. |

There is no LTS line and no semantic-versioning guarantee before `1.0`.
A supported-versions matrix with explicit minor-version rows will be
added once a `1.0` release exists.

## Disclosure Timeline

The maintainers follow a coordinated-disclosure model with the following
indicative SLAs, measured from the first report received via a channel
above:

- **Acknowledgement**: within **5 business days**.
- **Triage and severity assignment** (CVSS-style high / critical vs.
  low / medium): within **10 business days** of acknowledgement.
- **Fix or mitigation target**:
  - **High / Critical**: a fix or documented mitigation within
    **30 days** of triage.
  - **Low / Medium**: a fix or documented mitigation within **90 days**
    of triage.
- **Public disclosure**: coordinated with the reporter, after a fixed
  release is available. We default to crediting the reporter in the
  release notes and any published advisory unless you ask us not to.

If a report falls outside these timelines (for example, an upstream
dependency that we cannot patch ourselves), we will say so explicitly
in our reply rather than letting the report stall.

## Verifying Releases

Releases from `v0.8.0` onward are signed with
[cosign](https://github.com/sigstore/cosign) keyless signing, using
the GitHub Actions OIDC identity of the release workflow. No signing
keys exist to leak; each signature chains to the exact workflow run
that built the release.

Every release publishes, for each archive
(`mindspec_<version>_<os>_<arch>.tar.gz`, or `.zip` on Windows) and
for `checksums.txt`:

- `<asset>.sig` — the cosign signature
- `<asset>.pem` — the signing certificate

Each archive additionally ships an SPDX SBOM as
`<archive>.spdx.json`. (`checksums.txt` is signed but has no SBOM —
a checksum manifest carries no content beyond itself.)

To verify a downloaded archive, fetch it together with its `.sig` and
`.pem` companions, then run:

```bash
cosign verify-blob \
  --certificate-identity-regexp '^https://github\.com/mrmaxsteel/mindspec/\.github/workflows/release\.yml@refs/tags/v.*$' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --signature "${ARCHIVE}.sig" \
  --certificate "${ARCHIVE}.pem" \
  "${ARCHIVE}"
```

The identity regexp is pinned to this repository's release workflow
running on a `v*` tag — a signature produced by a fork, or by any
other workflow, will not verify. The same command verifies
`checksums.txt` against its own `.sig` / `.pem` files.

The install scripts (`install.sh` / `install.ps1`) verify SHA-256
checksums automatically; cosign verification of `checksums.txt` as
above extends that chain of trust back to the release workflow.
