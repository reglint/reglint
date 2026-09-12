#!/bin/sh
# RegLint installer: downloads a release archive from GitHub Releases,
# verifies its SHA-256 checksum, and installs the binary.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/iyaki/reglint/main/install.sh | sh
#
# Environment:
#   REGLINT_VERSION      Version to install (e.g. 0.1.0 or v0.1.0). Default: latest release.
#   REGLINT_INSTALL_DIR  Install directory. Default: $HOME/.local/bin

set -eu

REPO="iyaki/reglint"
BIN_NAME="reglint"

log() {
	printf '%s\n' "$1"
}

fail() {
	printf 'install.sh: %s\n' "$1" >&2
	exit 1
}

need_cmd() {
	command -v "$1" >/dev/null 2>&1 || fail "$1 is required."
}

need_cmd curl
need_cmd tar

if command -v sha256sum >/dev/null 2>&1; then
	CHECKSUM_CMD="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
	CHECKSUM_CMD="shasum -a 256"
else
	fail "sha256sum or shasum is required to verify the release."
fi

hash_of() {
	# CHECKSUM_CMD may contain a flag argument ("shasum -a 256").
	# shellcheck disable=SC2086
	$CHECKSUM_CMD "$1" | cut -d ' ' -f1
}

os=$(uname -s)
arch=$(uname -m)

case "$os" in
Linux)
	os_name="linux"
	;;
Darwin)
	os_name="darwin"
	;;
*)
	fail "unsupported operating system: $os (prebuilt binaries target linux and darwin; on Windows download the .zip from https://github.com/$REPO/releases or use go install)"
	;;
esac

case "$arch" in
x86_64 | amd64)
	arch_name="amd64"
	;;
arm64 | aarch64)
	arch_name="arm64"
	;;
*)
	fail "unsupported architecture: $arch"
	;;
esac

version="${REGLINT_VERSION:-}"
if [ -z "$version" ]; then
	log "Determining latest release..."
	release_url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest")
	version=$(basename "$release_url")
fi
version=${version#v}
tag="v$version"

archive="${BIN_NAME}_${version}_${os_name}_${arch_name}.tar.gz"
base_url="https://github.com/$REPO/releases/download/$tag"
tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

log "Downloading reglint $version for $os_name/$arch_name..."
curl -fsSL -o "$tmpdir/$archive" "$base_url/$archive"
curl -fsSL -o "$tmpdir/checksums.txt" "$base_url/checksums.txt"

expected=$(grep -F "  $archive" "$tmpdir/checksums.txt" | cut -d ' ' -f1)
[ -n "$expected" ] || fail "no checksum entry for $archive in checksums.txt"
actual=$(hash_of "$tmpdir/$archive")
[ "$actual" = "$expected" ] || fail "checksum mismatch for $archive (download corrupted or tampered)"
log "Checksum OK."

tar -xzf "$tmpdir/$archive" -C "$tmpdir"
[ -f "$tmpdir/$BIN_NAME" ] || fail "archive did not contain the $BIN_NAME binary"

install_dir="${REGLINT_INSTALL_DIR:-$HOME/.local/bin}"
mkdir -p "$install_dir"
mv "$tmpdir/$BIN_NAME" "$install_dir/$BIN_NAME"

case ":$PATH:" in
*":$install_dir:"*) ;;
*)
	log "Note: $install_dir is not in your PATH."
	log "Add it with: export PATH=\"$install_dir:\$PATH\""
	;;
esac

"$install_dir/$BIN_NAME" version
log "Installed reglint $version to $install_dir/$BIN_NAME"
