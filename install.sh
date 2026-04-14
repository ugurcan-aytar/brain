#!/usr/bin/env sh
# brain installer — fetches the latest prebuilt binary from GitHub Releases
# and installs it into a writable prefix. Runs `brain doctor` at the end so
# the user can see whether their LLM backend is wired up.
#
# Retrieval is handled in-process now (by the embedded recall library), so
# there's no npm / Node.js step anymore.
#
# Usage:
#   curl -sSfL https://raw.githubusercontent.com/ugurcan-aytar/brain/main/install.sh | sh
#
# Environment overrides:
#   BRAIN_VERSION   Pin a specific release tag (default: latest).
#   BRAIN_PREFIX    Install prefix (default: /usr/local, falls back to
#                   $HOME/.local if /usr/local isn't writable and sudo is absent).

set -eu

REPO="ugurcan-aytar/brain"
VERSION="${BRAIN_VERSION:-latest}"
PREFIX="${BRAIN_PREFIX:-}"

red()    { printf '\033[31m%s\033[0m\n' "$*"; }
green()  { printf '\033[32m%s\033[0m\n' "$*"; }
yellow() { printf '\033[33m%s\033[0m\n' "$*"; }
dim()    { printf '\033[2m%s\033[0m\n' "$*"; }
bold()   { printf '\033[1m%s\033[0m\n' "$*"; }

die() {
  red "error: $*" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

need uname
need tar
need mktemp
if command -v curl >/dev/null 2>&1; then
  DL="curl -fsSL"
elif command -v wget >/dev/null 2>&1; then
  DL="wget -qO-"
else
  die "neither curl nor wget is available"
fi

# --- detect platform ---------------------------------------------------------

os_raw=$(uname -s)
arch_raw=$(uname -m)

case "$os_raw" in
  Darwin)  GOOS=Darwin ;;
  Linux)   GOOS=Linux ;;
  MINGW*|MSYS*|CYGWIN*)
    die "Windows isn't supported at the moment. Open an issue at https://github.com/$REPO/issues if you'd like it — we'll scope the work."
    ;;
  *) die "unsupported OS: $os_raw" ;;
esac

case "$arch_raw" in
  x86_64|amd64) GOARCH=x86_64 ;;
  arm64|aarch64) GOARCH=arm64 ;;
  *) die "unsupported architecture: $arch_raw" ;;
esac

# --- resolve version ---------------------------------------------------------

if [ "$VERSION" = "latest" ]; then
  bold "Resolving latest release..."
  # The /releases/latest redirect gives us the tag without hitting the rate-
  # limited API. We follow it manually so we can read the Location header.
  api_url="https://api.github.com/repos/$REPO/releases/latest"
  tag=$($DL "$api_url" 2>/dev/null | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1 || true)
  if [ -z "${tag:-}" ]; then
    die "could not determine latest release tag (rate-limited? set BRAIN_VERSION=v1.x.x)"
  fi
  VERSION="$tag"
fi

version_num="${VERSION#v}"
archive="brain_${version_num}_${GOOS}_${GOARCH}.tar.gz"
url="https://github.com/$REPO/releases/download/${VERSION}/${archive}"
checksums_url="https://github.com/$REPO/releases/download/${VERSION}/checksums.txt"

bold "Installing brain $VERSION for $GOOS/$GOARCH"
dim "  $url"

# --- download + verify -------------------------------------------------------

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT INT TERM

if command -v curl >/dev/null 2>&1; then
  curl -fsSL -o "$tmpdir/$archive" "$url" || die "download failed: $url"
  curl -fsSL -o "$tmpdir/checksums.txt" "$checksums_url" || die "download failed: $checksums_url"
else
  wget -q -O "$tmpdir/$archive" "$url" || die "download failed: $url"
  wget -q -O "$tmpdir/checksums.txt" "$checksums_url" || die "download failed: $checksums_url"
fi

# Verify the sha256 if we can — best effort, since not every box has a sha tool.
if command -v shasum >/dev/null 2>&1; then
  SHA="shasum -a 256"
elif command -v sha256sum >/dev/null 2>&1; then
  SHA="sha256sum"
else
  SHA=""
fi

if [ -n "$SHA" ]; then
  expected=$(grep " $archive\$" "$tmpdir/checksums.txt" | awk '{print $1}')
  if [ -z "$expected" ]; then
    yellow "warning: archive not listed in checksums.txt, skipping verification"
  else
    actual=$(cd "$tmpdir" && $SHA "$archive" | awk '{print $1}')
    if [ "$expected" != "$actual" ]; then
      die "checksum mismatch for $archive (expected $expected, got $actual)"
    fi
    green "  checksum ok"
  fi
else
  yellow "warning: no shasum/sha256sum found, skipping verification"
fi

# --- extract -----------------------------------------------------------------

tar -xzf "$tmpdir/$archive" -C "$tmpdir" || die "extraction failed"
if [ ! -f "$tmpdir/brain" ]; then
  die "brain binary not found in archive"
fi
chmod +x "$tmpdir/brain"

# --- pick install prefix -----------------------------------------------------

pick_prefix() {
  if [ -n "$PREFIX" ]; then
    echo "$PREFIX"
    return
  fi
  if [ -w /usr/local/bin ] 2>/dev/null; then
    echo /usr/local
    return
  fi
  if command -v sudo >/dev/null 2>&1; then
    echo /usr/local
    return
  fi
  # Fall back to a per-user prefix — the user may need to add it to PATH.
  echo "$HOME/.local"
}

PREFIX=$(pick_prefix)
bindir="$PREFIX/bin"
mkdir -p "$bindir" 2>/dev/null || true

if [ -w "$bindir" ]; then
  mv "$tmpdir/brain" "$bindir/brain"
else
  yellow "  $bindir is not writable — using sudo"
  sudo mv "$tmpdir/brain" "$bindir/brain"
fi

green "  installed to $bindir/brain"

case ":$PATH:" in
  *":$bindir:"*) ;;
  *)
    yellow "  $bindir is not on your PATH"
    dim "    Add this to your shell rc: export PATH=\"$bindir:\$PATH\""
    ;;
esac

# --- final verification ------------------------------------------------------

echo
if command -v brain >/dev/null 2>&1; then
  brain doctor || true
else
  # brain isn't on PATH yet — call it by absolute path.
  "$bindir/brain" doctor || true
fi

echo
bold "Next steps:"
dim "  brain add ~/Documents/my-notes     # register a folder"
dim "  brain ask \"your first question\"    # ask it something"
