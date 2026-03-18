#!/bin/sh
set -e

# MindSpec installer script
# Usage: curl -fsSL https://raw.githubusercontent.com/mrmaxsteel/mindspec/main/install.sh | sh
#        curl -fsSL https://raw.githubusercontent.com/mrmaxsteel/mindspec/main/install.sh | sh -s -- --force

REPO="mrmaxsteel/mindspec"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="mindspec"
FORCE_INSTALL="${FORCE_INSTALL:-false}"
LOG_FILE="${HOME}/.mindspec/install.log"

# Parse arguments
for arg in "$@"; do
    case "$arg" in
        --force|-f)
            FORCE_INSTALL=true
            ;;
    esac
done

# Setup logging
setup_logging() {
    LOG_DIR=$(dirname "$LOG_FILE")
    if [ ! -d "$LOG_DIR" ]; then
        mkdir -p "$LOG_DIR" 2>/dev/null || true
    fi
    if [ -w "$LOG_DIR" ] || [ -w "$LOG_FILE" ]; then
        # Determine privilege level
        if [ "$(id -u)" -eq 0 ]; then
            PRIVILEGE_LEVEL="root"
        else
            PRIVILEGE_LEVEL="non-root"
        fi
        
        echo "=== MindSpec Installation Log ===" >> "$LOG_FILE"
        echo "Date: $(date -u +"%Y-%m-%d %H:%M:%S UTC")" >> "$LOG_FILE"
        echo "User: $(whoami)" >> "$LOG_FILE"
        echo "Privileges: $PRIVILEGE_LEVEL" >> "$LOG_FILE"
        echo "Install Dir: $INSTALL_DIR" >> "$LOG_FILE"
        echo "" >> "$LOG_FILE"
    fi
}

# Log message to file and optionally to stdout
log() {
    message="$1"
    if [ -w "$LOG_FILE" ] || [ -w "$(dirname "$LOG_FILE")" ]; then
        echo "[$(date -u +"%Y-%m-%d %H:%M:%S")] $message" >> "$LOG_FILE" 2>/dev/null || true
    fi
}

# Colors for output
if [ -t 1 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    NC='\033[0m'
else
    RED=''
    GREEN=''
    YELLOW=''
    NC=''
fi

info() {
    printf "${GREEN}==>${NC} %s\n" "$1"
    log "INFO: $1"
}

warn() {
    printf "${YELLOW}Warning:${NC} %s\n" "$1"
    log "WARN: $1"
}

error() {
    printf "${RED}Error:${NC} %s\n" "$1" >&2
    log "ERROR: $1"
    exit 1
}

# Detect OS
detect_os() {
    case "$(uname -s)" in
        Linux*)     echo "linux" ;;
        Darwin*)    echo "darwin" ;;
        *)          error "Unsupported operating system: $(uname -s)" ;;
    esac
}

# Detect architecture
detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)   echo "amd64" ;;
        aarch64|arm64)  echo "arm64" ;;
        *)              error "Unsupported architecture: $(uname -m)" ;;
    esac
}

# Get latest release version from GitHub
get_latest_version() {
    if command -v curl >/dev/null 2>&1; then
        curl --proto '=https' --tlsv1.2 -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | \
            grep '"tag_name":' | \
            sed 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/'
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | \
            grep '"tag_name":' | \
            sed 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/'
    else
        error "Neither curl nor wget found. Please install one of them."
    fi
}

# Download file
download() {
    url="$1"
    output="$2"
    
    if command -v curl >/dev/null 2>&1; then
        curl --proto '=https' --tlsv1.2 -fsSL -o "$output" "$url"
    elif command -v wget >/dev/null 2>&1; then
        wget -q -O "$output" "$url"
    else
        error "Neither curl nor wget found. Please install one of them."
    fi
}

# Check if running with sufficient privileges
check_privileges() {
    if [ ! -w "$INSTALL_DIR" ]; then
        if [ "$(id -u)" -ne 0 ]; then
            warn "Installation directory $INSTALL_DIR is not writable."
            warn "You may need to run this script with sudo or set INSTALL_DIR to a writable location."
            warn "Example: curl -fsSL <url> | INSTALL_DIR=~/.local/bin sh"
            return 1
        fi
    fi
    return 0
}

# Check for existing installation and prompt for overwrite
check_existing_installation() {
    if [ -f "$INSTALL_DIR/$BINARY_NAME" ]; then
        # Try to get version - parse "mindspec version X.Y.Z ..." format
        EXISTING_VERSION=$("$INSTALL_DIR/$BINARY_NAME" --version 2>/dev/null | awk '{print $3}' || echo "unknown")
        if [ -z "$EXISTING_VERSION" ]; then
            EXISTING_VERSION="unknown"
        fi
        
        # If force flag is set, skip prompts
        if [ "$FORCE_INSTALL" = "true" ]; then
            info "Found existing installation (${EXISTING_VERSION}), overwriting (--force)"
            return 0
        fi
        
        warn "Found existing installation: ${EXISTING_VERSION}"
        
        # Check if stdin is a terminal (interactive)
        if [ -t 0 ]; then
            # Interactive mode - prompt user
            printf "Overwrite existing installation? (y/n): "
            read -r response
            case "$response" in
                [yY]|[yY][eE][sS])
                    info "Proceeding with installation..."
                    return 0
                    ;;
                *)
                    info "Installation cancelled."
                    exit 0
                    ;;
            esac
        else
            # Non-interactive (piped from curl)
            # If version is unknown, treat as same version (idempotent - reinstall is safe)
            if [ "$EXISTING_VERSION" = "unknown" ]; then
                info "Existing installation found (version unknown). Use --force to reinstall."
                info "Example: curl -fsSL <url> | sh -s -- --force"
                exit 0
            elif [ "$EXISTING_VERSION" = "$VERSION" ] || [ "$EXISTING_VERSION" = "${VERSION#v}" ]; then
                info "Already installed (${EXISTING_VERSION}). Use --force to reinstall."
                info "Example: curl -fsSL <url> | sh -s -- --force"
                exit 0
            else
                error "Different version installed (${EXISTING_VERSION}). Use --force to upgrade/downgrade."
            fi
        fi
    fi
    return 0
}

main() {
    setup_logging
    log "Installation started"
    
    info "Installing MindSpec..."
    
    # Detect system
    OS=$(detect_os)
    ARCH=$(detect_arch)
    info "Detected: ${OS}/${ARCH}"
    
    # Get latest version
    info "Fetching latest release..."
    VERSION=$(get_latest_version)
    if [ -z "$VERSION" ]; then
        error "Failed to fetch latest version"
    fi
    info "Latest version: ${VERSION}"
    
    # Construct download URL
    ARCHIVE_NAME="${BINARY_NAME}_${VERSION#v}_${OS}_${ARCH}.tar.gz"
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE_NAME}"
    
    # Create temporary directory
    TMP_DIR=$(mktemp -d)
    trap 'rm -rf "$TMP_DIR"' EXIT
    
    # Download archive
    info "Downloading ${ARCHIVE_NAME}..."
    download "$DOWNLOAD_URL" "$TMP_DIR/$ARCHIVE_NAME"
    
    # Download and verify checksum
    info "Verifying checksum..."
    CHECKSUM_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"
    download "$CHECKSUM_URL" "$TMP_DIR/checksums.txt"
    
    # Extract expected checksum for our archive
    EXPECTED_CHECKSUM=$(grep "$ARCHIVE_NAME" "$TMP_DIR/checksums.txt" | awk '{print $1}')
    if [ -z "$EXPECTED_CHECKSUM" ]; then
        warn "Checksum not found for ${ARCHIVE_NAME}, skipping verification"
    else
        # Calculate actual checksum
        if command -v sha256sum >/dev/null 2>&1; then
            ACTUAL_CHECKSUM=$(sha256sum "$TMP_DIR/$ARCHIVE_NAME" | awk '{print $1}')
        elif command -v shasum >/dev/null 2>&1; then
            ACTUAL_CHECKSUM=$(shasum -a 256 "$TMP_DIR/$ARCHIVE_NAME" | awk '{print $1}')
        elif command -v openssl >/dev/null 2>&1; then
            ACTUAL_CHECKSUM=$(openssl dgst -sha256 "$TMP_DIR/$ARCHIVE_NAME" | awk '{print $NF}')
        else
            warn "sha256sum/shasum/openssl not found, skipping checksum verification"
            ACTUAL_CHECKSUM=""
        fi
        
        if [ -n "$ACTUAL_CHECKSUM" ]; then
            if [ "$ACTUAL_CHECKSUM" != "$EXPECTED_CHECKSUM" ]; then
                error "Checksum verification failed!
Expected: $EXPECTED_CHECKSUM
Got:      $ACTUAL_CHECKSUM"
            fi
            info "Checksum verified successfully"
        fi
    fi
    
    # Extract binary
    info "Extracting..."
    tar -xzf "$TMP_DIR/$ARCHIVE_NAME" -C "$TMP_DIR"
    
    # Check privileges before attempting install
    if ! check_privileges; then
        error "Cannot write to $INSTALL_DIR"
    fi
    
    # Check for existing installation
    check_existing_installation
    
    # Install binary
    info "Installing to ${INSTALL_DIR}/${BINARY_NAME}..."
    mkdir -p "$INSTALL_DIR"
    mv "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
    chmod +x "$INSTALL_DIR/$BINARY_NAME"
    
    # Verify installation
    if command -v "$BINARY_NAME" >/dev/null 2>&1; then
        info "Successfully installed MindSpec ${VERSION}"
        info "Run 'mindspec --help' to get started"
        log "Installation completed successfully: version ${VERSION}"
        
        echo ""
        info "IMPORTANT: MindSpec requires additional dependencies:"
        echo ""
        echo "  1. Beads (issue tracker):"
        echo "     curl --proto '=https' --tlsv1.2 -fsSL https://raw.githubusercontent.com/steveyegge/beads/main/scripts/install.sh | bash"
        echo ""
        echo "  2. Dolt (database for Beads):"
        echo "     sudo bash -c \"curl --proto '=https' --tlsv1.2 -L https://github.com/dolthub/dolt/releases/latest/download/install.sh | bash\""
        echo ""
        echo "  Verify installation with: bd --version && dolt version"
        echo ""
    else
        warn "Installation complete, but ${BINARY_NAME} is not in PATH"
        warn "Add ${INSTALL_DIR} to your PATH or run: export PATH=\"\$PATH:${INSTALL_DIR}\""
        log "Installation completed but binary not in PATH"
    fi
    
    log "Installation finished"
    log ""
}

main
