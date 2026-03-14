#Requires -Version 5.0

<#
.SYNOPSIS
    MindSpec installer script for Windows
.DESCRIPTION
    Downloads and installs the latest MindSpec release for Windows
.PARAMETER InstallDir
    Installation directory (default: $env:LOCALAPPDATA\Programs\mindspec)
.PARAMETER Force
    Force reinstallation even if already installed
.EXAMPLE
    irm https://raw.githubusercontent.com/mrmaxsteel/mindspec/main/install.ps1 | iex
.EXAMPLE
    irm https://raw.githubusercontent.com/mrmaxsteel/mindspec/main/install.ps1 | iex -Force
#>

[CmdletBinding()]
param(
    [string]$InstallDir = "$env:LOCALAPPDATA\Programs\mindspec",
    [switch]$Force
)

$ErrorActionPreference = 'Stop'

# Enforce TLS 1.2 minimum - equivalent to curl --tlsv1.2
# Prevents downgrade attacks on older .NET/PowerShell versions
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

# Configuration
$Repo = "mrmaxsteel/mindspec"
$BinaryName = "mindspec.exe"
$LogFile = "$env:USERPROFILE\.mindspec\install.log"

# Setup logging
function Initialize-Logging {
    $logDir = Split-Path $LogFile -Parent
    if (-not (Test-Path $logDir)) {
        New-Item -ItemType Directory -Path $logDir -Force | Out-Null
    }
    
    # Determine privilege level
    $isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
    $privilegeLevel = if ($isAdmin) { "Administrator" } else { "Non-Administrator" }
    
    "=== MindSpec Installation Log ===" | Out-File -FilePath $LogFile -Append
    "Date: $((Get-Date).ToUniversalTime().ToString('yyyy-MM-dd HH:mm:ss UTC'))" | Out-File -FilePath $LogFile -Append
    "User: $env:USERNAME" | Out-File -FilePath $LogFile -Append
    "Privileges: $privilegeLevel" | Out-File -FilePath $LogFile -Append
    "Install Dir: $InstallDir" | Out-File -FilePath $LogFile -Append
    "" | Out-File -FilePath $LogFile -Append
}

# Log message to file
function Write-Log {
    param([string]$Message)
    try {
        $timestamp = (Get-Date).ToUniversalTime().ToString('yyyy-MM-dd HH:mm:ss')
        "[$timestamp] $Message" | Out-File -FilePath $LogFile -Append -ErrorAction SilentlyContinue
    }
    catch {
        # Silently fail if logging doesn't work
    }
}

# Helper functions
function Write-Info {
    param([string]$Message)
    Write-Host "==> " -ForegroundColor Green -NoNewline
    Write-Host $Message
    Write-Log "INFO: $Message"
}

function Write-Warn {
    param([string]$Message)
    Write-Host "Warning: " -ForegroundColor Yellow -NoNewline
    Write-Host $Message
    Write-Log "WARN: $Message"
}

function Write-Error {
    param([string]$Message)
    Write-Host "Error: " -ForegroundColor Red -NoNewline
    Write-Host $Message
    Write-Log "ERROR: $Message"
    exit 1
}

function Invoke-SecureRequest {
    param(
        [string]$Uri,
        [string]$OutFile = $null
    )
    # Enforce HTTPS-only - equivalent to curl --proto '=https'
    if ($Uri -notmatch '^https://') {
        Write-Error "Refusing non-HTTPS URL: $Uri"
    }
    if ($OutFile) {
        Invoke-WebRequest -Uri $Uri -OutFile $OutFile -UseBasicParsing -MaximumRedirection 5
    }
    else {
        Invoke-RestMethod -Uri $Uri -UseBasicParsing -MaximumRedirection 5
    }
}
    $arch = $env:PROCESSOR_ARCHITECTURE
    switch ($arch) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        default { Write-Error "Unsupported architecture: $arch" }
    }
}

function Get-LatestVersion {
    try {
        $apiUrl = "https://api.github.com/repos/$Repo/releases/latest"
        $response = Invoke-SecureRequest -Uri $apiUrl
        return $response.tag_name
    }
    catch {
        Write-Error "Failed to fetch latest version: $_"
    }
}

function Test-ExistingInstallation {
    $binaryPath = Join-Path $InstallDir $BinaryName
    
    if (Test-Path $binaryPath) {
        try {
            # Parse "mindspec version X.Y.Z ..." format
            $versionOutput = & $binaryPath --version 2>$null
            if ($versionOutput -match 'version\s+(\S+)') {
                $existingVersion = $matches[1]
            }
            else {
                $existingVersion = "unknown"
            }
        }
        catch {
            $existingVersion = "unknown"
        }
        
        Write-Warn "Found existing installation: $existingVersion"
        
        # If Force flag is set, proceed
        if ($Force) {
            Write-Info "Overwriting existing installation (--Force)"
            return $true
        }
        
        # Check if running interactively
        if ([Environment]::UserInteractive -and -not [Console]::IsInputRedirected) {
            $response = Read-Host "Overwrite existing installation? (y/n)"
            if ($response -match '^[yY]') {
                Write-Info "Proceeding with installation..."
                return $true
            }
            else {
                Write-Info "Installation cancelled."
                exit 0
            }
        }
        else {
            # Non-interactive (piped from irm)
            $version = $script:Version
            if ($existingVersion -eq "unknown") {
                Write-Info "Existing installation found (version unknown). Use -Force to reinstall."
                Write-Info "Example: irm <url> | iex -Force"
                exit 0
            }
            elseif ($existingVersion -eq $version -or $existingVersion -eq $version.TrimStart('v')) {
                Write-Info "Already installed ($existingVersion). Use -Force to reinstall."
                Write-Info "Example: irm <url> | iex -Force"
                exit 0
            }
            else {
                Write-Error "Different version installed ($existingVersion). Use -Force to upgrade/downgrade."
            }
        }
    }
    
    return $true
}

function Add-ToPath {
    param([string]$Directory)
    
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    
    if ($userPath -notlike "*$Directory*") {
        Write-Info "Adding $Directory to user PATH..."
        $newPath = "$userPath;$Directory"
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        
        # Update current session
        $env:Path = "$env:Path;$Directory"
        
        Write-Info "PATH updated. Restart your terminal for changes to take effect."
        return $true
    }
    
    return $false
}

function Install-MindSpec {
    Initialize-Logging
    Write-Log "Installation started"
    
    Write-Info "Installing MindSpec..."
    
    # Detect architecture
    $arch = Get-Architecture
    Write-Info "Detected: windows/$arch"
    
    # Get latest version
    Write-Info "Fetching latest release..."
    $script:Version = Get-LatestVersion
    if (-not $Version) {
        Write-Error "Failed to fetch latest version"
    }
    Write-Info "Latest version: $Version"
    
    # Check for existing installation
    Test-ExistingInstallation | Out-Null
    
    # Construct download URL
    $versionNumber = $Version.TrimStart('v')
    $archiveName = "mindspec_${versionNumber}_windows_${arch}.zip"
    $downloadUrl = "https://github.com/$Repo/releases/download/$Version/$archiveName"
    
    # Create temporary directory
    $tempDir = Join-Path $env:TEMP "mindspec-install-$(Get-Random)"
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
    
    try {
        # Download archive
        $archivePath = Join-Path $tempDir $archiveName
        Write-Info "Downloading $archiveName..."
        
        try {
            Invoke-SecureRequest -Uri $downloadUrl -OutFile $archivePath
        }
        catch {
            Write-Error "Failed to download: $_"
        }
        
        # Download and verify checksum
        Write-Info "Verifying checksum..."
        $checksumUrl = "https://github.com/$Repo/releases/download/$Version/checksums.txt"
        $checksumPath = Join-Path $tempDir "checksums.txt"
        
        try {
            Invoke-SecureRequest -Uri $checksumUrl -OutFile $checksumPath
            
            # Extract expected checksum for our archive
            $checksumContent = Get-Content $checksumPath
            $expectedChecksum = ($checksumContent | Select-String -Pattern $archiveName | ForEach-Object { $_.Line.Split()[0] })
            
            if (-not $expectedChecksum) {
                Write-Warn "Checksum not found for $archiveName, skipping verification"
            }
            else {
                # Calculate actual checksum
                $actualChecksum = (Get-FileHash -Path $archivePath -Algorithm SHA256).Hash.ToLower()
                
                if ($actualChecksum -ne $expectedChecksum) {
                    Write-Error "Checksum verification failed!`nExpected: $expectedChecksum`nGot:      $actualChecksum"
                }
                Write-Info "Checksum verified successfully"
            }
        }
        catch {
            Write-Warn "Failed to download or verify checksum: $_"
            Write-Warn "Continuing without checksum verification"
        }
        
        # Extract archive
        Write-Info "Extracting..."
        try {
            Expand-Archive -Path $archivePath -DestinationPath $tempDir -Force
        }
        catch {
            Write-Error "Failed to extract: $_"
        }
        
        # Create install directory
        Write-Info "Installing to $InstallDir..."
        if (-not (Test-Path $InstallDir)) {
            New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
        }
        
        # Copy binary
        $sourceBinary = Join-Path $tempDir $BinaryName
        $destBinary = Join-Path $InstallDir $BinaryName
        
        if (-not (Test-Path $sourceBinary)) {
            Write-Error "Binary not found in archive: $BinaryName"
        }
        
        Copy-Item -Path $sourceBinary -Destination $destBinary -Force
        
        # Add to PATH
        $pathUpdated = Add-ToPath -Directory $InstallDir
        
        # Verify installation
        $installedBinary = Join-Path $InstallDir $BinaryName
        if (Test-Path $installedBinary) {
            Write-Info "Successfully installed MindSpec $Version"
            Write-Info "Run 'mindspec --help' to get started"
            Write-Log "Installation completed successfully: version $Version"
            
            Write-Host ""
            Write-Info "IMPORTANT: MindSpec requires additional dependencies:"
            Write-Host ""
            Write-Host "  1. Beads (issue tracker):"
            Write-Host "     irm https://raw.githubusercontent.com/steveyegge/beads/main/install.ps1 | iex"
            Write-Host ""
            Write-Host "  2. Dolt (database for Beads):"
            Write-Host "     Download and install: https://github.com/dolthub/dolt/releases/latest/download/dolt-windows-amd64.msi"
            Write-Host ""
            Write-Host "  Verify installation with: bd --version; dolt version"
            Write-Host ""
            
            if (-not $pathUpdated) {
                Write-Info "Installation directory already in PATH"
            }
        }
        else {
            Write-Error "Installation verification failed"
        }
        
        Write-Log "Installation finished"
        Write-Log ""
    }
    finally {
        # Cleanup
        if (Test-Path $tempDir) {
            Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

# Main execution
try {
    Install-MindSpec
}
catch {
    Write-Error "Installation failed: $_"
}
