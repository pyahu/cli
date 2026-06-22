#!/bin/sh
# Pyahu CLI installer.
#
#   curl -fsSL https://cli.pyahu.io/install.sh | sh
#   curl -fsSL https://cli.pyahu.io/install.sh | sh -s -- --bin-dir "$HOME/.local/bin"
#   curl -fsSL https://cli.pyahu.io/install.sh | sh -s -- --version v1.2.3
#
# Downloads a released binary from GitHub Releases. macOS and Linux only;
# on Windows download the .zip from the releases page.
set -eu

REPO="pyahu/cli"
BINARY="pyahu"
BIN_DIR="/usr/local/bin"
VERSION="${PYAHU_VERSION:-latest}"

err() {
  echo "error: $*" >&2
  exit 1
}

while [ $# -gt 0 ]; do
  case "$1" in
    --bin-dir) BIN_DIR="$2"; shift 2 ;;
    --bin-dir=*) BIN_DIR="${1#*=}"; shift ;;
    --version) VERSION="$2"; shift 2 ;;
    --version=*) VERSION="${1#*=}"; shift ;;
    -h|--help) echo "usage: install.sh [--bin-dir DIR] [--version vX.Y.Z]"; exit 0 ;;
    *) err "unknown option: $1" ;;
  esac
done

command -v curl >/dev/null 2>&1 || err "curl is required"
command -v tar >/dev/null 2>&1 || err "tar is required"

os="$(uname -s)"
arch="$(uname -m)"
case "$arch" in
  x86_64 | amd64) arch="x86_64" ;;
  aarch64 | arm64) arch="arm64" ;;
  *) err "unsupported architecture: $arch" ;;
esac
case "$os" in
  Linux | Darwin) ;;
  *) err "unsupported OS: $os (download the Windows zip from the releases page)" ;;
esac

if [ "$VERSION" = "latest" ]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name":' | head -1 | sed -E 's/.*"(v[^"]+)".*/\1/')"
  [ -n "$VERSION" ] || err "could not resolve the latest version"
fi

archive="${BINARY}_${os}_${arch}.tar.gz"
url="https://github.com/$REPO/releases/download/$VERSION/$archive"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "downloading $BINARY $VERSION ($os/$arch)..."
curl -fsSL "$url" -o "$tmp/$archive" || err "download failed: $url"
tar -xzf "$tmp/$archive" -C "$tmp" || err "could not extract $archive"

need_sudo=""
if [ -d "$BIN_DIR" ]; then
  [ -w "$BIN_DIR" ] || need_sudo="sudo"
else
  mkdir -p "$BIN_DIR" 2>/dev/null || need_sudo="sudo"
fi

$need_sudo install -d "$BIN_DIR" || err "could not create $BIN_DIR"
$need_sudo install -m 0755 "$tmp/$BINARY" "$BIN_DIR/$BINARY" \
  || err "could not install to $BIN_DIR (try --bin-dir \"\$HOME/.local/bin\")"

echo "installed $BINARY to $BIN_DIR/$BINARY"
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "note: $BIN_DIR is not in your PATH" ;;
esac
"$BIN_DIR/$BINARY" --version 2>/dev/null || true
