#!/bin/sh
# mello installer.
#
#   curl -fsSL https://raw.githubusercontent.com/minhlucncc/mello-cli/main/install.sh | sh
#
# Environment variables:
#   MELLO_VERSION      version to install (default: latest release), e.g. v1.2.0
#   MELLO_INSTALL_DIR  install directory (default: /usr/local/bin or ~/.local/bin)
#   MELLO_REPO         GitHub repository (default: minhlucncc/mello-cli)
set -e

REPO="${MELLO_REPO:-minhlucncc/mello-cli}"
BIN="mello"

info() { printf '\033[32m==>\033[0m %s\n' "$1"; }
warn() { printf '\033[33mwarning:\033[0m %s\n' "$1" >&2; }
die()  { printf '\033[31merror:\033[0m %s\n' "$1" >&2; exit 1; }

detect_os() {
	os="$(uname -s | tr '[:upper:]' '[:lower:]')"
	case "$os" in
		linux) echo linux ;;
		darwin) echo darwin ;;
		mingw* | msys* | cygwin* | windows*) echo windows ;;
		*) die "unsupported operating system: $os" ;;
	esac
}

detect_arch() {
	arch="$(uname -m)"
	case "$arch" in
		x86_64 | amd64) echo amd64 ;;
		arm64 | aarch64) echo arm64 ;;
		*) die "unsupported architecture: $arch" ;;
	esac
}

fetch() {
	# fetch <url> <output>
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$1" -o "$2"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO "$2" "$1"
	else
		die "curl or wget is required"
	fi
}

fetch_stdout() {
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$1"
	else
		wget -qO- "$1"
	fi
}

latest_version() {
	fetch_stdout "https://api.github.com/repos/${REPO}/releases/latest" |
		grep '"tag_name"' | head -1 |
		sed -E 's/.*"tag_name": *"([^"]+)".*/\1/'
}

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | awk '{print $1}'
	fi
}

install_dir() {
	if [ -n "$MELLO_INSTALL_DIR" ]; then
		echo "$MELLO_INSTALL_DIR"
	elif [ -w /usr/local/bin ] 2>/dev/null; then
		echo /usr/local/bin
	elif command -v sudo >/dev/null 2>&1 && [ -d /usr/local/bin ]; then
		echo /usr/local/bin
	else
		echo "$HOME/.local/bin"
	fi
}

main() {
	os="$(detect_os)"
	arch="$(detect_arch)"
	version="${MELLO_VERSION:-$(latest_version)}"
	[ -n "$version" ] || die "could not determine the latest version; set MELLO_VERSION"
	v="${version#v}"

	ext="tar.gz"
	[ "$os" = "windows" ] && ext="zip"
	archive="${BIN}_${v}_${os}_${arch}.${ext}"
	base="https://github.com/${REPO}/releases/download/v${v}"

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT

	info "downloading ${archive}"
	fetch "${base}/${archive}" "${tmp}/${archive}" || die "download failed: ${base}/${archive}"

	if fetch "${base}/checksums.txt" "${tmp}/checksums.txt" 2>/dev/null; then
		want="$(grep " ${archive}\$" "${tmp}/checksums.txt" | awk '{print $1}')"
		got="$(sha256 "${tmp}/${archive}")"
		if [ -n "$want" ] && [ -n "$got" ]; then
			[ "$want" = "$got" ] || die "checksum verification failed for ${archive}"
			info "checksum verified"
		else
			warn "could not verify checksum (missing entry or no sha256 tool)"
		fi
	else
		warn "checksums.txt not found; skipping verification"
	fi

	if [ "$ext" = "zip" ]; then
		command -v unzip >/dev/null 2>&1 || die "unzip is required to install on this platform"
		unzip -o "${tmp}/${archive}" -d "$tmp" >/dev/null
	else
		tar -xzf "${tmp}/${archive}" -C "$tmp"
	fi

	src="${tmp}/${BIN}"
	[ "$os" = "windows" ] && src="${tmp}/${BIN}.exe"
	[ -f "$src" ] || die "binary not found inside the archive"

	dir="$(install_dir)"
	target="${dir}/$(basename "$src")"
	info "installing to ${target}"
	if mkdir -p "$dir" 2>/dev/null && [ -w "$dir" ]; then
		cp "$src" "$target"
		chmod +x "$target"
	else
		warn "elevated permissions required to write ${dir}"
		sudo mkdir -p "$dir"
		sudo cp "$src" "$target"
		sudo chmod +x "$target"
	fi

	info "installed mello ${version}"
	case ":$PATH:" in
		*":$dir:"*) ;;
		*) warn "${dir} is not on your PATH — add it, e.g. 'export PATH=\"${dir}:\$PATH\"'" ;;
	esac
}

main "$@"
