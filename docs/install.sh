#!/bin/sh
# POSIX-compliant installer for SOLV Server Tracker

set -e

REPO="AlvaroRiveraCarhuani/server_tracker"
GITHUB_API="https://api.github.com/repos/${REPO}/releases/latest"
INSTALL_DIR=""

# Sobriquet Unicode icons (zero emojis)
CHECK="✓"
CROSS="✗"
WARN="⚠"
INFO="ℹ"

# Cleanup on exit
cleanup() {
    if [ -n "${TMPDIR:-}" ] && [ -d "$TMPDIR" ]; then
        rm -rf "$TMPDIR"
    fi
}
trap cleanup EXIT INT TERM

# Verify required core dependencies
command -v curl >/dev/null 2>&1 || { echo "${CROSS} Error: curl is required to run this installer"; exit 1; }
command -v tar >/dev/null 2>&1 || { echo "${CROSS} Error: tar is required to extract the distribution package"; exit 1; }

if command -v sha256sum >/dev/null 2>&1; then
    SHA256_CMD="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
    SHA256_CMD="shasum -a 256"
else
    echo "${CROSS} Error: sha256sum or shasum is required for cryptographic verification"; exit 1
fi

echo "========================================================"
echo " SOLV Server Tracker :: Installer"
echo " Active Observability, AIOps & ChatOps for Docker"
echo "========================================================"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
    linux)
        OS="linux"
        ;;
    darwin)
        OS="darwin"
        echo "${INFO} macOS detected. Darwin binaries are provided as best-effort support."
        ;;
    msys*|mingw*|cygwin*|windows*)
        echo "${CROSS} Native Windows is not supported."
        echo "${INFO} Please run SOLV inside WSL2 (Windows Subsystem for Linux)."
        exit 1
        ;;
    *)
        echo "${CROSS} Unsupported operating system: $OS"
        echo "${INFO} Windows users: please use WSL2."
        exit 1
        ;;
esac

case "$ARCH" in
    x86_64|amd64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        echo "${CROSS} Unsupported architecture: $ARCH (supported: amd64, arm64)"
        exit 1
        ;;
esac

# Check for existing installation (idempotency)
if command -v solv-agent >/dev/null 2>&1; then
    EXISTING_VERSION=$(solv-agent --version 2>/dev/null | awk '{print $2}')
    echo "${INFO} Found existing installation: solv-agent ${EXISTING_VERSION:-installed}"
    printf "Reinstall? [y/N]: "
    read -r response
    case "$response" in
        [yY][eE][sS]|[yY])
            echo "${INFO} Proceeding with reinstallation..."
            ;;
        *)
            echo "Installation aborted by user."
            exit 0
            ;;
    esac
fi

# Language selection if ~/.solv/config does not exist
CONFIG_DIR="$HOME/.solv"
CONFIG_FILE="$CONFIG_DIR/config"

if [ ! -f "$CONFIG_FILE" ]; then
    echo ""
    echo "idioma / language:"
    echo "  [1] español (default)"
    echo "  [2] english"
    printf "Choose [1/2]: "
    read -r lang_choice
    case "$lang_choice" in
        2)
            LANGUAGE="en"
            ;;
        *)
            LANGUAGE="es"
            ;;
    esac
else
    LANGUAGE=$(grep -o '"language"[[:space:]]*:[[:space:]]*"[^"]*"' "$CONFIG_FILE" 2>/dev/null | cut -d'"' -f4 || echo "es")
    if [ -z "$LANGUAGE" ]; then
        LANGUAGE="es"
    fi
fi

# Fetch latest version from GitHub API (with fallback)
echo "${INFO} Querying latest release from GitHub..."
LATEST_VERSION=""
if API_RESPONSE=$(curl -fsSL --connect-timeout 5 "$GITHUB_API" 2>/dev/null); then
    LATEST_VERSION=$(echo "$API_RESPONSE" | grep '"tag_name"' | head -n 1 | cut -d'"' -f4)
fi

if [ -z "$LATEST_VERSION" ]; then
    echo "${WARN} Could not resolve latest release via GitHub API (network or rate limit)"
    echo "${INFO} Using default version v1.0.0"
    LATEST_VERSION="v1.0.0"
fi

echo "${INFO} Selected version: ${LATEST_VERSION}"

# Construct download URLs
FILENAME="solv-agent-${OS}-${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_VERSION}/${FILENAME}"
CHECKSUM_URL="${DOWNLOAD_URL}.sha256"

# Create secure temporary directory
TMPDIR=$(mktemp -d /tmp/solv-install.XXXXXX)
echo "${INFO} Downloading ${FILENAME}..."

if ! curl -fSL --progress-bar -o "$TMPDIR/$FILENAME" "$DOWNLOAD_URL"; then
    echo "${CROSS} Failed to download package from: $DOWNLOAD_URL"
    echo "${INFO} Please verify network connectivity or check releases at https://github.com/${REPO}/releases"
    exit 1
fi

if ! curl -fsSL -o "$TMPDIR/package.sha256" "$CHECKSUM_URL"; then
    echo "${CROSS} Failed to download SHA256 checksum file from: $CHECKSUM_URL"
    exit 1
fi

# Verify cryptographic checksum
echo "${INFO} Verifying SHA256 cryptographic checksum..."
EXPECTED_HASH=$(awk '{print $1}' "$TMPDIR/package.sha256")
if [ -z "$EXPECTED_HASH" ]; then
    echo "${CROSS} Checksum file is empty or corrupted"
    exit 1
fi

cd "$TMPDIR"
if [ "$SHA256_CMD" = "sha256sum" ]; then
    ACTUAL_HASH=$(sha256sum "$FILENAME" | awk '{print $1}')
else
    ACTUAL_HASH=$(shasum -a 256 "$FILENAME" | awk '{print $1}')
fi
cd - >/dev/null

if [ "$EXPECTED_HASH" != "$ACTUAL_HASH" ]; then
    echo "${CROSS} Checksum verification FAILED!"
    echo "  Expected: $EXPECTED_HASH"
    echo "  Actual:   $ACTUAL_HASH"
    echo "${CROSS} The downloaded file may be corrupted or tampered with. Aborting installation."
    exit 1
fi

echo "${CHECK} Checksum verified: ${ACTUAL_HASH}"

# Determine target directory
if [ -w "/usr/local/bin" ]; then
    INSTALL_DIR="/usr/local/bin"
elif command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null; then
    INSTALL_DIR="/usr/local/bin"
else
    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$INSTALL_DIR"
fi

echo "${INFO} Installing binaries to ${INSTALL_DIR}..."
tar -xzf "$TMPDIR/$FILENAME" -C "$TMPDIR"

if [ "$INSTALL_DIR" = "/usr/local/bin" ] && [ ! -w "/usr/local/bin" ]; then
    sudo mv "$TMPDIR/solv-agent" "$INSTALL_DIR/solv-agent"
    sudo chmod +x "$INSTALL_DIR/solv-agent"
    if [ -f "$TMPDIR/solv-update" ]; then
        sudo mv "$TMPDIR/solv-update" "$INSTALL_DIR/solv-update"
        sudo chmod +x "$INSTALL_DIR/solv-update"
    fi
else
    mv "$TMPDIR/solv-agent" "$INSTALL_DIR/solv-agent"
    chmod +x "$INSTALL_DIR/solv-agent"
    if [ -f "$TMPDIR/solv-update" ]; then
        mv "$TMPDIR/solv-update" "$INSTALL_DIR/solv-update"
        chmod +x "$INSTALL_DIR/solv-update"
    fi
fi

# Write initial managed configuration (Cero .env)
mkdir -p "$CONFIG_DIR"
if [ ! -f "$CONFIG_FILE" ]; then
    cat > "$CONFIG_FILE" <<EOF
{
  "language": "${LANGUAGE}"
}
EOF
    chmod 600 "$CONFIG_FILE"
    echo "${CHECK} Configuration created at ${CONFIG_FILE}"
fi

# Save installed version
echo "$LATEST_VERSION" > "$CONFIG_DIR/version"
chmod 600 "$CONFIG_DIR/version"

# Validate PATH
if [ "$INSTALL_DIR" = "$HOME/.local/bin" ]; then
    case ":$PATH:" in
        *":$HOME/.local/bin:"*) ;;
        *)
            echo ""
            echo "${WARN} $HOME/.local/bin is not in your current PATH."
            echo "${INFO} Add it to your shell profile (~/.bashrc, ~/.zshrc):"
            echo "       export PATH=\"\$HOME/.local/bin:\$PATH\""
            ;;
    esac
fi

echo ""
echo "${CHECK} SOLV Server Tracker ${LATEST_VERSION} successfully installed!"
echo "--------------------------------------------------------"
echo "Quick start:"
echo "  solv-agent --mode=tui"
echo ""
echo "For background agent collection:"
echo "  solv-agent --mode=daemon"
echo ""
echo "To update in the future:"
echo "  solv-update"
echo "========================================================"
