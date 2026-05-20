# Installing AgentMind

AgentMind is the standalone observability binary that mindspec spawns to
collect OTLP telemetry, render the visualization UI, and replay recordings.
The binary lives at `github.com/mrmaxsteel/agentmind`; mindspec imports
only its `client` and `wire` Go packages and discovers the binary at
runtime via the documented lookup order
(`$AGENTMIND_BIN` -> `<mindspec-root>/bin/agentmind` -> PATH).

This page documents the **supported install path for AgentMind v1.0.0**:
documented manual download with checksum verification. A first-party
`mindspec install agentmind` subcommand is deferred to a follow-up spec
(see spec 083 Non-Goals).

> **Status (current).** AgentMind v1.0.0 has not yet been published
> upstream. The URLs and checksums below describe the **release plan**
> that the agentmind side will satisfy when it tags v1.0.0. Until then
> mindspec runs against a sibling checkout produced by
> `scripts/checkout-agentmind.sh`. Once upstream ships v1.0.0, run
> `scripts/pin-agentmind-release.sh v1.0.0` to drop the local `replace`
> directive from this repo's `go.mod` and pin the released tag.

## Release artifact layout

Each AgentMind release publishes the following artifacts to its GitHub
release page (`https://github.com/mrmaxsteel/agentmind/releases/tag/v<VERSION>`):

| File | Platform |
|------|----------|
| `agentmind-<VERSION>-darwin-arm64.tar.gz` | macOS, Apple Silicon |
| `agentmind-<VERSION>-darwin-amd64.tar.gz` | macOS, Intel |
| `agentmind-<VERSION>-linux-amd64.tar.gz` | Linux, x86_64 |
| `agentmind-<VERSION>-windows-amd64.zip` | Windows, x86_64 |
| `SHA256SUMS` | Checksums for all archives |
| `SHA256SUMS.sig` | Detached signature (if available) |

The archive's top-level entry is a single `agentmind` (or `agentmind.exe`)
binary. No other layout assumptions are made.

## URL pattern

Artifacts follow this URL pattern; replace `<VERSION>` with the release
tag (e.g. `v1.0.0`) and `<PLATFORM>` with one of `darwin-arm64`,
`darwin-amd64`, `linux-amd64`, `windows-amd64`:

```
https://github.com/mrmaxsteel/agentmind/releases/download/<VERSION>/agentmind-<VERSION>-<PLATFORM>.tar.gz
https://github.com/mrmaxsteel/agentmind/releases/download/<VERSION>/agentmind-<VERSION>-<PLATFORM>.zip
https://github.com/mrmaxsteel/agentmind/releases/download/<VERSION>/SHA256SUMS
```

## Install + verify (Linux / macOS)

Adjust `VERSION` and `PLATFORM` (one of `darwin-arm64`, `darwin-amd64`,
`linux-amd64`):

```bash
VERSION=v1.0.0
PLATFORM=darwin-arm64   # or darwin-amd64, linux-amd64
BASE=https://github.com/mrmaxsteel/agentmind/releases/download/${VERSION}

# 1. Download the archive and the checksums file.
curl -fL -o "agentmind-${VERSION}-${PLATFORM}.tar.gz" \
    "${BASE}/agentmind-${VERSION}-${PLATFORM}.tar.gz"
curl -fL -o SHA256SUMS "${BASE}/SHA256SUMS"

# 2. Verify the archive checksum.
#    Linux: use `sha256sum`. macOS: use `shasum -a 256`.
if command -v sha256sum >/dev/null 2>&1; then
    grep " agentmind-${VERSION}-${PLATFORM}.tar.gz$" SHA256SUMS | sha256sum -c -
else
    grep " agentmind-${VERSION}-${PLATFORM}.tar.gz$" SHA256SUMS | shasum -a 256 -c -
fi

# 3. Extract and install.
tar -xzf "agentmind-${VERSION}-${PLATFORM}.tar.gz"

# Place at <mindspec-root>/bin/agentmind (preferred — found via the
# mindspec-root step of the binary lookup order) or any directory on PATH.
mkdir -p ./bin
mv agentmind ./bin/agentmind
chmod +x ./bin/agentmind

# 4. Smoke-test.
./bin/agentmind --version
```

## Install + verify (Windows / PowerShell)

```powershell
$Version  = "v1.0.0"
$Platform = "windows-amd64"
$Base     = "https://github.com/mrmaxsteel/agentmind/releases/download/$Version"

# 1. Download the archive and the checksums file.
Invoke-WebRequest -OutFile "agentmind-$Version-$Platform.zip" `
    -Uri "$Base/agentmind-$Version-$Platform.zip"
Invoke-WebRequest -OutFile "SHA256SUMS" -Uri "$Base/SHA256SUMS"

# 2. Verify the archive checksum.
$expected = (Get-Content SHA256SUMS | `
    Select-String "agentmind-$Version-$Platform.zip" | `
    ForEach-Object { ($_ -split '\s+')[0] }).ToLower()
$actual = (Get-FileHash -Algorithm SHA256 "agentmind-$Version-$Platform.zip").Hash.ToLower()
if ($expected -ne $actual) { throw "checksum mismatch" }

# 3. Extract and install.
Expand-Archive -Path "agentmind-$Version-$Platform.zip" -DestinationPath .
New-Item -ItemType Directory -Force -Path .\bin | Out-Null
Move-Item -Force .\agentmind.exe .\bin\agentmind.exe

# 4. Smoke-test.
.\bin\agentmind.exe --version
```

## Binary discovery order

Once installed, mindspec finds the binary via this exact order (mirrored
in `agentmind/client.findBinary`):

1. `$AGENTMIND_BIN` if set and pointing at an executable file.
2. `<mindspec-root>/bin/agentmind` (resolved against the mindspec module
   root, not the current working directory).
3. The first `agentmind` on `PATH`.

If none resolves, mindspec consumers fall into the spec's three command
classes (see spec 083 Hard Constraint #4):

- **Telemetry-as-output** (`mindspec record start`) and **interactive**
  (`mindspec viz`, `mindspec agentmind serve`, `mindspec agentmind replay`)
  commands exit non-zero with a clear error.
- **Batch / side-effect** commands (`mindspec bench run`,
  `mindspec agentmind setup`) exit 0 after emitting exactly one warn line:
  `WARN: agentmind binary not found; telemetry export will drop silently`.

## Verifying a working install

After placing the binary, run:

```bash
mindspec viz --help
```

The command should re-exec into the agentmind binary and print its
`agentmind serve --help` output. If you instead see
`agentmind binary not found`, double-check the location and that the file
is executable.

## Rollback

To uninstall AgentMind: remove the binary file. mindspec's batch commands
remain functional with the warn-line contract; interactive and
telemetry-as-output commands stop working until the binary returns.

## See also

- Spec 083 (AgentMind Extraction v2): `.mindspec/docs/specs/083-agentmind-extraction-v2/spec.md`
- ADR-0011 (One-way mindspec -> agentmind dependency)
- ADR-0026 (AgentMind extracted to standalone repo)
