#!/bin/sh
# Install the deal CLI.
#
#   curl -fsSL https://raw.githubusercontent.com/oliviosubelza/deal-dev-kit/main/tool/scripts/install.sh | sh
#
# Environment:
#   DEAL_VERSION / DEAL_KIT_VERSION  release tag to install (default: latest)
#   DEAL_BIN_DIR / DEAL_KIT_BIN_DIR  install directory (default: $HOME/.local/bin)
#   DEAL_REPO    / DEAL_KIT_REPO     kit repository to verify access to
#
# The DEAL_KIT_* names still work. The command was renamed, not the contract:
# breaking a variable someone already set in CI is not worth the tidiness.

set -eu

REPO="oliviosubelza/deal-dev-kit"
VERSION="${DEAL_VERSION:-${DEAL_KIT_VERSION:-latest}}"
BIN_DIR="${DEAL_BIN_DIR:-${DEAL_KIT_BIN_DIR:-$HOME/.local/bin}}"
# Must match kit.DefaultRepo in the CLI: checking a URL the CLI never uses
# reports a problem that does not exist, and misses one that does.
KIT_REPO="${DEAL_REPO:-${DEAL_KIT_REPO:-https://github.com/oliviosubelza/deal-dev-kit.git}}"

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
		*) die "unsupported OS: $os (on Windows use install.ps1, or run this from WSL)" ;;
	esac
	echo "${os}_${arch}"
}

resolve_version() {
	[ "$VERSION" != "latest" ] && { echo "$VERSION"; return; }
	# 404 here means the repository has no published release yet, which is a
	# different problem from a network failure and deserves a different fix.
	if ! body=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null); then
		die "no published release found for ${REPO}.
  Someone needs to cut one first: push a v* tag and let CI build the binaries.
  To install a specific version once it exists: DEAL_VERSION=v0.1.0"
	fi
	echo "$body" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1
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

	echo "==> downloading deal ${version} (${target})"
	# The published asset keeps the old name on purpose: internal/selfupdate
	# builds it from a literal, so renaming it would leave every
	# already-installed binary unable to find its own update. Only the
	# installed file is `deal`.
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
	install -m 0755 "$tmp/deal-kit" "$BIN_DIR/deal"
	echo "==> installed to $BIN_DIR/deal"

	# A pre-rename install is still on PATH under its old name, and two copies
	# with different versions is worse than either alone: the user runs one and
	# reads the other's release notes.
	if old=$(command -v deal-kit 2>/dev/null); then
		echo "    NOTE: an older install is still at $old" >&2
		echo "    It is the same tool under the previous name; delete it." >&2
	fi

	echo "==> checking access to the kit repository"
	# Never prompt: this runs inside a `curl | sh`, where a host-key or
	# credential question has no terminal to answer it.
	if GIT_TERMINAL_PROMPT=0 \
		GIT_SSH_COMMAND="ssh -o BatchMode=yes -o StrictHostKeyChecking=accept-new" \
		git ls-remote --exit-code "$KIT_REPO" HEAD >/dev/null 2>&1; then
		echo "    ok"
	else
		echo "    WARNING: cannot reach $KIT_REPO" >&2
		echo "    deal is installed but cannot fetch the kit yet." >&2
		echo "    If the repository is private, make sure your git credentials" >&2
		echo "    or SSH key have access, then run: deal status" >&2
	fi

	case ":$PATH:" in
		*":$BIN_DIR:"*) ;;
		*) echo "==> add $BIN_DIR to your PATH" ;;
	esac
}

main "$@"
