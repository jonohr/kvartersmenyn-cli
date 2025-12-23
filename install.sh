#!/usr/bin/env bash
set -euo pipefail

REPO="jonohr/kvartersmenyn-cli"
BIN_NAME="kvartersmenyn-cli"

log() {
  echo "[install] $*"
}

usage() {
  cat <<'EOF'
Usage: ./install.sh [--dest DIR]

Installs the latest release binary into DIR.
Default: /usr/local/bin if writable, otherwise ~/.local/bin
EOF
}

dest=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dest)
      dest="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage
      exit 1
      ;;
  esac
done

os="$(uname -s)"
arch="$(uname -m)"
log "Detected OS: ${os}"
log "Detected arch: ${arch}"

case "$os" in
  Darwin)
    os_label="macOS"
    ;;
  Linux)
    os_label="linux"
    ;;
  *)
    echo "Unsupported OS: $os" >&2
    exit 1
    ;;
esac

case "$arch" in
  x86_64|amd64)
    arch_label="amd64"
    ;;
  arm64|aarch64)
    arch_label="arm64"
    ;;
  *)
    echo "Unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

if [[ -z "$dest" ]]; then
  dest="/usr/local/bin"
fi
log "Install destination: ${dest}"

if [[ "${os_label}" = "macOS" ]]; then
  archive="${BIN_NAME}_${os_label}_universal.tar.gz"
else
  archive="${BIN_NAME}_${os_label}_${arch_label}.tar.gz"
fi
url="https://github.com/${REPO}/releases/latest/download/${archive}"
log "Download URL: ${url}"

tmpdir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

if command -v curl >/dev/null 2>&1; then
  log "Downloading with curl..."
  curl -fsSL -o "${tmpdir}/${archive}" "$url"
elif command -v wget >/dev/null 2>&1; then
  log "Downloading with wget..."
  wget -q -O "${tmpdir}/${archive}" "$url"
else
  echo "Missing curl or wget." >&2
  exit 1
fi

log "Extracting archive..."
tar -C "$tmpdir" -xzf "${tmpdir}/${archive}"

install_cmd=()
if command -v install >/dev/null 2>&1; then
  install_cmd=(install -m 0755 "${tmpdir}/${BIN_NAME}" "${dest}/${BIN_NAME}")
else
  install_cmd=(cp "${tmpdir}/${BIN_NAME}" "${dest}/${BIN_NAME}")
fi

if [[ -w "$dest" ]]; then
  log "Installing to ${dest}..."
  mkdir -p "$dest"
  "${install_cmd[@]}"
else
  if command -v sudo >/dev/null 2>&1; then
    log "Installing to ${dest} with sudo..."
    sudo mkdir -p "$dest"
    sudo "${install_cmd[@]}"
  else
    dest="${HOME}/.local/bin"
    mkdir -p "$dest"
    log "Installing to ${dest} (no sudo available)..."
    install_cmd=()
    if command -v install >/dev/null 2>&1; then
      install_cmd=(install -m 0755 "${tmpdir}/${BIN_NAME}" "${dest}/${BIN_NAME}")
    else
      install_cmd=(cp "${tmpdir}/${BIN_NAME}" "${dest}/${BIN_NAME}")
    fi
    "${install_cmd[@]}"
  fi
fi

chmod 0755 "${dest}/${BIN_NAME}" 2>/dev/null || true

echo "Installed ${BIN_NAME} to ${dest}/${BIN_NAME}"

if [[ "$os" = "Darwin" ]]; then
  printf "Remove macOS quarantine attribute (recommended)? [y/N] "
  if [[ -r /dev/tty ]]; then
    read -r reply </dev/tty
  else
    read -r reply
  fi
  if [[ "$reply" = "y" || "$reply" = "Y" ]]; then
    xattr -dr com.apple.quarantine "${dest}/${BIN_NAME}"
    echo "Removed quarantine attribute."
  fi
fi
