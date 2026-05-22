#!/usr/bin/env sh
# remetric install script - see https://github.com/remetric-dev/remetric
set -eu

REPO="remetric-dev/remetric"
INSTALL_DIR="${REMETRIC_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${REMETRIC_VERSION:-latest}"

# Detect OS
case "$(uname -s)" in
  Linux*)  OS=linux ;;
  Darwin*) OS=darwin ;;
  *) echo "remetric: unsupported OS: $(uname -s)" >&2; exit 1 ;;
esac

# Detect arch
case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) echo "remetric: unsupported arch: $(uname -m)" >&2; exit 1 ;;
esac

# Resolve "latest" via GitHub API
if [ "$VERSION" = "latest" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' \
    | head -n1)
  if [ -z "$VERSION" ]; then
    echo "remetric: failed to resolve latest version" >&2
    exit 1
  fi
fi
VER_NOPREFIX="${VERSION#v}"

ARCHIVE="remetric_${VER_NOPREFIX}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"
CHECKSUM_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "remetric: downloading ${ARCHIVE}..."
curl -fsSL -o "$TMP/$ARCHIVE" "$URL"
curl -fsSL -o "$TMP/checksums.txt" "$CHECKSUM_URL"

EXPECTED=$(grep " ${ARCHIVE}$" "$TMP/checksums.txt" | awk '{print $1}')
if [ -z "$EXPECTED" ]; then
  echo "remetric: ${ARCHIVE} not in checksums.txt" >&2
  exit 1
fi
ACTUAL=$(sha256sum "$TMP/$ARCHIVE" 2>/dev/null | awk '{print $1}' || \
         shasum -a 256 "$TMP/$ARCHIVE" | awk '{print $1}')
if [ "$EXPECTED" != "$ACTUAL" ]; then
  echo "remetric: checksum mismatch (expected $EXPECTED, got $ACTUAL)" >&2
  exit 1
fi

tar -xzf "$TMP/$ARCHIVE" -C "$TMP"
mkdir -p "$INSTALL_DIR"
mv "$TMP/remetric" "$INSTALL_DIR/remetric"
chmod +x "$INSTALL_DIR/remetric"

echo "remetric: installed to ${INSTALL_DIR}/remetric"
if ! echo ":$PATH:" | grep -q ":${INSTALL_DIR}:"; then
  echo "remetric: ${INSTALL_DIR} is not in your PATH - add it to use 'remetric' from any shell"
fi
"$INSTALL_DIR/remetric" --version
