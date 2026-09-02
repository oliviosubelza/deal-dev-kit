#!/bin/sh
# Install the deal-kit CLI.
#
#   curl -fsSL https://raw.githubusercontent.com/deal/deal-kit-cli/main/scripts/install.sh | sh
#
# Environment:
#   DEAL_KIT_VERSION  release tag to install (default: latest)
#   DEAL_KIT_BIN_DIR  install directory (default: $HOME/.local/bin)
#   DEAL_KIT_REPO     kit repository to verify SSH access against

set -eu

REPO="oliviosubelza/deal-dev-kit"
VERSION="${DEAL_KIT_VERSION:-latest}"
BIN_DIR="${DEAL_KIT_BIN_DIR:-$HOME/.local/bin}"
KIT_REPO="${DEAL_KIT_REPO:-git@github.com:oliviosubelza/deal-dev-kit.git}"

die() { echo "install: $*" >&2; exit 1; }

detect_target() {
	os=$(uname -s | tr '[:upper:]' '[:lower:]')
	arch=$(uname -m)
	case "$arch" in
		x86_64|amd64) arch=amd64 ;;
		arm64|aarch64) arch=arm64 ;;
		*) die "unsupported architecture: $arch" ;;
	esac
	case "$os" in
		linux|darwin) ;;
		*) die "unsupported OS: $os (on Windows, run this from WSL)" ;;
	esac
	echo "${os}_${arch}"
}

resolve_version() {
	[ "$VERSION" != "latest" ] && { echo "$VERSION"; return; }
	curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
		| sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' \
		| head -n1
}

main() {
	command -v curl >/dev/null 2>&1 || die "curl is required"
	command -v git  >/dev/null 2>&1 || die "git is required"

	target=$(detect_target)
	version=$(resolve_version)
	[ -n "$version" ] || die "could not resolve a release version"

	base="https://github.com/${REPO}/releases/download/${version}"
	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' EXIT

	echo "==> downloading deal-kit ${version} (${target})"
	curl -fsSL "${base}/deal-kit_${target}" -o "$tmp/deal-kit" \
		|| die "download failed"
	curl -fsSL "${base}/checksums.txt" -o "$tmp/checksums.txt" \
		|| die "checksums download failed"

	echo "==> verifying checksum"
	expected=$(grep " deal-kit_${target}\$" "$tmp/checksums.txt" | awk '{print $1}')
	[ -n "$expected" ] || die "no checksum published for ${target}"
	if command -v sha256sum >/dev/null 2>&1; then
		actual=$(sha256sum "$tmp/deal-kit" | awk '{print $1}')
	else
		actual=$(shasum -a 256 "$tmp/deal-kit" | awk '{print $1}')
	fi
	[ "$expected" = "$actual" ] || die "checksum mismatch (expected $expected, got $actual)"

	mkdir -p "$BIN_DIR"
	install -m 0755 "$tmp/deal-kit" "$BIN_DIR/deal-kit"
	echo "==> installed to $BIN_DIR/deal-kit"

	echo "==> checking SSH access to the kit repository"
	if git ls-remote --exit-code "$KIT_REPO" HEAD >/dev/null 2>&1; then
		echo "    ok"
	else
		echo "    WARNING: no SSH access to $KIT_REPO" >&2
		echo "    deal-kit is installed but cannot fetch the kit yet." >&2
		echo "    Add your SSH key to GitHub and request access to the repository." >&2
	fi

	case ":$PATH:" in
		*":$BIN_DIR:"*) ;;
		*) echo "==> add $BIN_DIR to your PATH" ;;
	esac
}

main "$@"
