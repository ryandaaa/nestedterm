#!/bin/sh
# NestedTerm installer: downloads a prebuilt Linux binary from GitHub Releases,
# verifies its SHA-256 checksum, and installs it for the current user.
#
#   curl -fsSL https://raw.githubusercontent.com/ryandaaa/nestedterm/main/install.sh | sh
#
# Environment:
#   NESTEDTERM_VERSION   version to install, e.g. 0.1.0 (default: latest release)
#   PREFIX               install prefix (default: $HOME/.local; binary goes in $PREFIX/bin)
#   NESTEDTERM_BASE_URL  override the download base URL (mirrors, testing)
#
# Uninstall: sh install.sh --uninstall
set -eu

REPO="ryandaaa/nestedterm"
BIN="nestedterm"
PREFIX="${PREFIX:-$HOME/.local}"
BINDIR="$PREFIX/bin"

say() { printf '%s\n' "$*"; }
die() { printf 'install.sh: error: %s\n' "$*" >&2; exit 1; }

if [ "${1:-}" = "--uninstall" ]; then
	if [ -e "$BINDIR/$BIN" ]; then
		rm -f "$BINDIR/$BIN"
		say "Removed $BINDIR/$BIN"
	else
		say "Nothing to remove at $BINDIR/$BIN"
	fi
	exit 0
fi

[ "$(uname -s)" = "Linux" ] || die "NestedTerm only supports Linux (this is $(uname -s))."

case "$(uname -m)" in
	x86_64 | amd64) ARCH=amd64 ;;
	aarch64 | arm64) ARCH=arm64 ;;
	*) die "unsupported CPU architecture: $(uname -m) (supported: x86_64, aarch64)." ;;
esac

if command -v curl >/dev/null 2>&1; then
	fetch() { curl -fsSL --retry 3 -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
	fetch() { wget -q -O "$2" "$1"; }
else
	die "need curl or wget to download the binary."
fi

if command -v sha256sum >/dev/null 2>&1; then
	sha256() { sha256sum "$1" | cut -d' ' -f1; }
elif command -v shasum >/dev/null 2>&1; then
	sha256() { shasum -a 256 "$1" | cut -d' ' -f1; }
else
	die "need sha256sum or shasum to verify the download."
fi

ASSET="$BIN-linux-$ARCH"
if [ -n "${NESTEDTERM_BASE_URL:-}" ]; then
	BASE="$NESTEDTERM_BASE_URL"
elif [ -n "${NESTEDTERM_VERSION:-}" ]; then
	BASE="https://github.com/$REPO/releases/download/v${NESTEDTERM_VERSION#v}"
else
	BASE="https://github.com/$REPO/releases/latest/download"
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT INT TERM

say "Downloading $ASSET ..."
fetch "$BASE/$ASSET" "$TMP/$ASSET" || die "could not download $BASE/$ASSET"
fetch "$BASE/SHA256SUMS" "$TMP/SHA256SUMS" || die "could not download $BASE/SHA256SUMS"

want="$(awk -v f="$ASSET" '$2 == f || $2 == "*" f { print $1 }' "$TMP/SHA256SUMS")"
[ -n "$want" ] || die "$ASSET is not listed in SHA256SUMS."
got="$(sha256 "$TMP/$ASSET")"
[ "$want" = "$got" ] || die "checksum mismatch for $ASSET (expected $want, got $got); nothing was installed."
say "Checksum OK."

mkdir -p "$BINDIR"
chmod 755 "$TMP/$ASSET"
# Write next to the destination then rename, so a running copy is replaced atomically.
cp "$TMP/$ASSET" "$BINDIR/.$BIN.new"
mv -f "$BINDIR/.$BIN.new" "$BINDIR/$BIN"

say "Installed $("$BINDIR/$BIN" --version 2>/dev/null || echo "$BIN") to $BINDIR/$BIN"

case ":$PATH:" in
	*":$BINDIR:"*) ;;
	*)
		say ""
		say "$BINDIR is not on your PATH. Add this to your shell profile (~/.bashrc or ~/.zshrc):"
		say "    export PATH=\"$BINDIR:\$PATH\""
		;;
esac
