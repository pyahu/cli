#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
temp_dir="$(mktemp -d)"
trap 'rm -rf "$temp_dir"' EXIT

case "$(uname -s)" in
  Linux | Darwin) os="$(uname -s)" ;;
  *) echo "installer test only supports macOS and Linux" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch="x86_64" ;;
  aarch64 | arm64) arch="arm64" ;;
  *) echo "installer test does not support this architecture" >&2; exit 1 ;;
esac

archive="pyahu_${os}_${arch}.tar.gz"
fixture_dir="$temp_dir/fixtures"
mock_bin="$temp_dir/bin"
mkdir -p "$fixture_dir" "$mock_bin" "$temp_dir/payload"

printf '#!/bin/sh\necho pyahu test\n' >"$temp_dir/payload/pyahu"
chmod +x "$temp_dir/payload/pyahu"
tar -czf "$fixture_dir/$archive" -C "$temp_dir/payload" pyahu

if command -v sha256sum >/dev/null 2>&1; then
  checksum="$(sha256sum "$fixture_dir/$archive" | awk '{ print $1 }')"
else
  checksum="$(shasum -a 256 "$fixture_dir/$archive" | awk '{ print $1 }')"
fi
printf '%s  %s\n' "$checksum" "$archive" >"$fixture_dir/checksums.txt"

cat >"$mock_bin/curl" <<'SH'
#!/bin/sh
set -eu
url="$2"
destination="$4"
cp "$FIXTURE_DIR/$(basename "$url")" "$destination"
SH
chmod +x "$mock_bin/curl"

install_dir="$temp_dir/install"
PATH="$mock_bin:$PATH" FIXTURE_DIR="$fixture_dir" PYAHU_DOWNLOAD_BASE="https://example.invalid" \
  sh "$repo_root/website/public/install.sh" --version vtest --bin-dir "$install_dir" >/dev/null
cmp "$temp_dir/payload/pyahu" "$install_dir/pyahu"

printf 'corrupt' >>"$fixture_dir/$archive"
bad_install_dir="$temp_dir/install-bad"
if PATH="$mock_bin:$PATH" FIXTURE_DIR="$fixture_dir" PYAHU_DOWNLOAD_BASE="https://example.invalid" \
  sh "$repo_root/website/public/install.sh" --version vtest --bin-dir "$bad_install_dir" >/dev/null 2>&1; then
  echo "installer accepted an archive with an invalid SHA-256" >&2
  exit 1
fi
if [[ -e "$bad_install_dir/pyahu" ]]; then
  echo "installer wrote a binary after checksum verification failed" >&2
  exit 1
fi

echo "Installer checksum tests passed"
