#!/usr/bin/env bash
set -euo pipefail

REPO="tunit/tunit"
BIN_DIR="${TUNIT_BIN_DIR:-/usr/local/bin}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# Detect OS and arch
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

case "$OS" in
  linux|darwin) ;;
  *) echo "Unsupported OS: $OS (use tunit.ps1 on Windows)" >&2; exit 1 ;;
esac

ASSET="tunit-${OS}-${ARCH}"
VERSION="${TUNIT_VERSION:-latest}"

if [ "$VERSION" = "latest" ]; then
  DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"
else
  DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
fi

echo "Downloading tunit ${VERSION} for ${OS}/${ARCH}..."
curl -fsSL "$DOWNLOAD_URL" -o "$TMP/tunit"
chmod +x "$TMP/tunit"

# Verify it runs
"$TMP/tunit" --version >/dev/null 2>&1 || true

# Install
if [ -w "$BIN_DIR" ]; then
  mv "$TMP/tunit" "$BIN_DIR/tunit"
else
  sudo mv "$TMP/tunit" "$BIN_DIR/tunit"
fi

echo "Installed: $(command -v tunit)"
echo "Run: tunit"
