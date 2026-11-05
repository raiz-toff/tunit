#!/usr/bin/env bash
set -euo pipefail

REPO="raiz-toff/tunit"
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

echo ""
echo "✓ Installed: $(command -v tunit)"
echo ""

# Run tunit (not exec — we need control back for cleanup)
set +e
tunit "$@"
TUNIT_EXIT=$?
set -e

echo ""
echo "────────────────────────────────────────────"
echo ""

# Find any QC reports generated in common locations
REPORTS=()
for dir in . /tmp "$HOME"; do
  while IFS= read -r -d '' f; do
    REPORTS+=("$f")
  done < <(find "$dir" -maxdepth 1 -name "QC_REPORT_*.txt" -print0 2>/dev/null)
done

if [ ${#REPORTS[@]} -gt 0 ]; then
  echo "  Found ${#REPORTS[@]} QC report(s)."
  printf '    • %s\n' "${REPORTS[@]}"
  echo ""
  printf "  Save report(s) to Desktop? [Y/n]: "
  read -r SAVE_CHOICE
  SAVE_CHOICE="${SAVE_CHOICE:-Y}"
  if [[ "$SAVE_CHOICE" =~ ^[Yy] ]]; then
    DESKTOP="$HOME/Desktop"
    mkdir -p "$DESKTOP"
    for r in "${REPORTS[@]}"; do
      cp "$r" "$DESKTOP/"
      echo "  ✓ Saved: $DESKTOP/$(basename "$r")"
    done
  fi
  # Remove report files from original locations
  for r in "${REPORTS[@]}"; do
    rm -f "$r"
  done
fi

echo ""
echo "  Cleaning up..."

# Remove tunit binary
if [ -w "$BIN_DIR" ]; then
  rm -f "$BIN_DIR/tunit"
else
  sudo rm -f "$BIN_DIR/tunit"
fi

echo "  ✓ Removed tunit binary"
echo "  ✓ All traces cleaned"
echo ""

exit $TUNIT_EXIT
