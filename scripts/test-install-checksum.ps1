#Requires -Version 5.0
# Regression guard for the v0.8.0 install-blocker (PowerShell side).
#
# Mirror of scripts/test-install-checksum.sh for install.ps1. v0.8.0 added syft
# SBOMs, so checksums.txt gained an <archive>.spdx.json line for every archive.
# install.ps1's unanchored `Select-String -Pattern $archiveName` matched both the
# archive and its SBOM line, yielding two hashes and failing the compare. The fix
# (Get-ExpectedChecksum: exact field-2 equality + Select -First 1) is dot-sourced
# here so this test exercises the real extraction logic.
#
# Deterministic, offline. Exits non-zero on any mismatch.

$ErrorActionPreference = 'Stop'

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$installPs1 = Join-Path $repoRoot 'install.ps1'

# Dot-source for functions only — MINDSPEC_INSTALL_NO_MAIN stops Install-MindSpec
# from running (which would detect the OS and hit the network). Enable StrictMode
# only afterwards so install.ps1's own top-level setup isn't held to it.
$env:MINDSPEC_INSTALL_NO_MAIN = '1'
. $installPs1
Set-StrictMode -Version Latest

if (-not (Get-Command Get-ExpectedChecksum -ErrorAction SilentlyContinue)) {
    Write-Host 'FAIL: install.ps1 does not expose a Get-ExpectedChecksum function'
    exit 1
}

$work = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $work | Out-Null
$fixture = Join-Path $work 'checksums.txt'

$archiveAmd64Hash = '89c1ca7500000000000000000000000000000000000000000000000000000001'
$sbomAmd64Hash    = 'ffffffff00000000000000000000000000000000000000000000000000000002'
$archiveArm64Hash = 'abcabcab00000000000000000000000000000000000000000000000000000003'
$sbomArm64Hash    = 'dededede00000000000000000000000000000000000000000000000000000004'

$archiveAmd64 = 'mindspec_0.8.0_windows_amd64.tar.gz'
$archiveArm64 = 'mindspec_0.8.0_windows_arm64.tar.gz'

# Real v0.8.0 goreleaser+syft layout: archive line then its .spdx.json SBOM line,
# two spaces between hash and filename.
@(
    "$archiveAmd64Hash  $archiveAmd64",
    "$sbomAmd64Hash  $archiveAmd64.spdx.json",
    "$archiveArm64Hash  $archiveArm64",
    "$sbomArm64Hash  $archiveArm64.spdx.json"
) | Set-Content -Path $fixture -Encoding ascii

function Fail([string]$msg) {
    Write-Host "FAIL: $msg"
    Remove-Item -Path $work -Recurse -Force -ErrorAction SilentlyContinue
    exit 1
}

# Guard 0: prove the fixture reproduces the collision — the OLD buggy extraction
# (Select-String -Pattern) returns TWO matches for one archive.
$oldBuggy = @(Get-Content $fixture | Select-String -Pattern $archiveAmd64)
if ($oldBuggy.Count -ne 2) {
    Fail "fixture does not reproduce the SBOM collision (old -Pattern matched $($oldBuggy.Count), expected 2)"
}

# Guard 1: amd64 returns exactly one hash, the archive's.
$got = @(Get-ExpectedChecksum -ArchiveName $archiveAmd64 -ChecksumPath $fixture)
if ($got.Count -ne 1) { Fail "amd64 extraction returned $($got.Count) hashes (expected 1): $got" }
if ($got[0] -ne $archiveAmd64Hash) { Fail "amd64 extraction returned wrong hash: $($got[0])" }
if ($got[0] -eq $sbomAmd64Hash)    { Fail "amd64 extraction returned the SBOM hash, not the archive's" }

# Guard 2: a second arch behaves identically.
$got = @(Get-ExpectedChecksum -ArchiveName $archiveArm64 -ChecksumPath $fixture)
if ($got.Count -ne 1) { Fail "arm64 extraction returned $($got.Count) hashes (expected 1)" }
if ($got[0] -ne $archiveArm64Hash) { Fail "arm64 extraction returned wrong hash: $($got[0])" }

# Guard 3: the SBOM filename itself still resolves to its own single hash.
$got = Get-ExpectedChecksum -ArchiveName "$archiveAmd64.spdx.json" -ChecksumPath $fixture
if ($got -ne $sbomAmd64Hash) { Fail "explicit SBOM lookup returned wrong hash: $got" }

# Guard 4: unknown arch yields nothing (skip-verification path).
$got = Get-ExpectedChecksum -ArchiveName 'mindspec_0.8.0_windows_riscv64.tar.gz' -ChecksumPath $fixture
if ($null -ne $got -and "$got" -ne '') { Fail "unknown arch should yield empty, got: $got" }

Remove-Item -Path $work -Recurse -Force -ErrorAction SilentlyContinue
Write-Host 'PASS: Get-ExpectedChecksum returns exactly the archive hash and ignores SBOM lines'
exit 0
