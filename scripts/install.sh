#!/bin/sh
# carty install script.
#
#   curl -fsSL https://carty.sh/install.sh | sh
#
# Detects OS/arch, downloads the matching binary from the latest GitHub
# Release, verifies it against SHA256SUMS, and installs it to
# ~/.carty/bin/carty. Does NOT touch shell rc files or PATH — see the
# README for why.
set -eu

REPO="dvdsvds/carty"
INSTALL_DIR="${HOME}/.carty/bin"

log() { printf '%s\n' "$*" >&2; }
die() {
	log "error: $*"
	exit 1
}

need() {
	command -v "$1" >/dev/null 2>&1 || die "'$1' is required but not found"
}

detect_platform() {
	os=$(uname -s)
	case "$os" in
	Linux) goos="linux" ;;
	*) die "carty only supports Linux (got: $os). See the README's 지원 범위 section." ;;
	esac

	arch=$(uname -m)
	case "$arch" in
	x86_64 | amd64) goarch="amd64" ;;
	aarch64 | arm64) goarch="arm64" ;;
	*) die "unsupported architecture: $arch" ;;
	esac
}

main() {
	need curl
	need sha256sum
	need tar
	need mktemp

	detect_platform
	asset="carty_${goos}_${goarch}"
	log "detected platform: ${goos}/${goarch}"

	latest_url="https://api.github.com/repos/${REPO}/releases/latest"
	tag=$(curl -fsSL "$latest_url" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
	[ -n "$tag" ] || die "couldn't determine the latest release tag"
	log "latest version: ${tag}"

	tmpdir=$(mktemp -d)
	trap 'rm -rf "$tmpdir"' EXIT

	base_url="https://github.com/${REPO}/releases/download/${tag}"
	log "downloading ${asset}..."
	curl -fsSL -o "${tmpdir}/${asset}" "${base_url}/${asset}" || die "failed to download ${asset} for ${tag}"
	curl -fsSL -o "${tmpdir}/SHA256SUMS" "${base_url}/SHA256SUMS" || die "failed to download SHA256SUMS for ${tag}"

	log "verifying checksum..."
	( cd "$tmpdir" && grep " ${asset}\$" SHA256SUMS | sha256sum -c - ) \
		|| die "checksum verification failed for ${asset}"

	mkdir -p "$INSTALL_DIR"
	install -m 0755 "${tmpdir}/${asset}" "${INSTALL_DIR}/carty"

	log ""
	log "carty ${tag} installed to ${INSTALL_DIR}/carty"
	log ""
	log "PATH is not modified automatically. Add this to your shell rc file:"
	log ""
	log "  export PATH=\"\$HOME/.carty/bin:\$PATH\""
	log ""
}

main "$@"
