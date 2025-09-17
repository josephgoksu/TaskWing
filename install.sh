#!/bin/sh
# TaskWing installation script
# Usage:
#   curl -sSfL https://raw.githubusercontent.com/josephgoksu/TaskWing/main/install.sh | sh
#
# Flags (pass after `sh -s --` when piping):
#   --version <tag>        Install a specific release (ex: v0.9.4 or 0.9.4)
#   --install-dir <path>   Install to the provided directory
#   --help                 Show this message
#
set -eu

REPO="josephgoksu/TaskWing"
BINARY_NAME="taskwing"
API_URL="https://api.github.com/repos/$REPO"
DOWNLOAD_URL_BASE="https://github.com/$REPO/releases/download"

REQUESTED_VERSION="${TASKWING_VERSION:-}"
CUSTOM_INSTALL_DIR="${TASKWING_INSTALL_DIR:-}"

usage() {
  cat <<'USAGE'
TaskWing installer
  Installs the latest TaskWing CLI release from GitHub.

Options:
  --version <tag>        Install a specific release version (with or without leading "v")
  --install-dir <path>   Install to a custom directory
  --help                 Show this help text

Environment variables:
  TASKWING_VERSION       Same as --version
  TASKWING_INSTALL_DIR   Same as --install-dir

Examples:
  curl -sSfL .../install.sh | sh
  curl -sSfL .../install.sh | sh -s -- --version v0.9.4
  curl -sSfL .../install.sh | sh -s -- --install-dir "$HOME/bin"
USAGE
}

log() {
  printf '%s\n' "$*"
}

fatal() {
  printf 'Error: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    fatal "Required command '$1' not found. Please install it and re-run."
  fi
}

parse_args() {
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --version|-v)
        [ "$#" -ge 2 ] || fatal "--version requires a value"
        REQUESTED_VERSION="$2"
        shift 2
        ;;
      --install-dir|--prefix)
        [ "$#" -ge 2 ] || fatal "--install-dir requires a value"
        CUSTOM_INSTALL_DIR="$2"
        shift 2
        ;;
      --help|-h)
        usage
        exit 0
        ;;
      *)
        fatal "Unknown option: $1"
        ;;
    esac
  done
}

normalize_version() {
  if [ -z "$1" ]; then
    return 0
  fi

  case "$1" in
    v*) printf '%s' "$1" ;;
    *) printf 'v%s' "$1" ;;
  esac
}

detect_os() {
  case "$(uname -s)" in
    Linux)  printf 'Linux' ;;
    Darwin) printf 'Darwin' ;;
    *)      fatal "Unsupported operating system: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf 'x86_64' ;;
    arm64|aarch64) printf 'arm64' ;;
    armv7l|armv7*) printf 'armv7' ;;
    armv6l|armv6*) printf 'armv6' ;;
    *) fatal "Unsupported architecture: $(uname -m)" ;;
  esac
}

fetch_latest_version() {
  curl -fsSL "$API_URL/releases/latest" \
    | awk -F'"' '/"tag_name":/{print $4; exit}'
}

resolve_version() {
  version="$(normalize_version "$REQUESTED_VERSION")"

  if [ -n "$version" ]; then
    printf '%s' "$version"
    return 0
  fi

  log "Fetching latest TaskWing release tag..."
  version="$(fetch_latest_version)"

  if [ -z "$version" ]; then
    fatal "Unable to determine latest release. Check your network connection or GitHub rate limits."
  fi

  printf '%s' "$version"
}

create_tmpdir() {
  if command -v mktemp >/dev/null 2>&1; then
    if TMPDIR=$(mktemp -d 2>/dev/null); then
      printf '%s' "$TMPDIR"
      return 0
    fi
    if TMPDIR=$(mktemp -d -t taskwing 2>/dev/null); then
      printf '%s' "$TMPDIR"
      return 0
    fi
  fi

  fatal "Failed to create temporary directory"
}

resolve_install_dir() {
  if [ -n "$CUSTOM_INSTALL_DIR" ]; then
    printf '%s' "$CUSTOM_INSTALL_DIR"
    return 0
  fi

  if [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
    printf '/usr/local/bin'
    return 0
  fi

  printf '%s/.local/bin' "$HOME"
}

install_binary() {
  src="$1"
  dest_dir="$2"
  dest="$dest_dir/$BINARY_NAME"

  if ! mkdir -p "$dest_dir" >/dev/null 2>&1; then
    fatal "Unable to create install directory: $dest_dir"
  fi

  if command -v install >/dev/null 2>&1; then
    if ! install -m 755 "$src" "$dest" 2>/dev/null; then
      fatal "Failed to install binary to $dest"
    fi
  else
    if ! cp "$src" "$dest" >/dev/null 2>&1; then
      fatal "Failed to copy binary to $dest"
    fi
    if ! chmod 755 "$dest" >/dev/null 2>&1; then
      fatal "Failed to set executable permissions on $dest"
    fi
  fi

  printf '%s' "$dest"
}

verify_in_path() {
  dir="$1"
  case ":$PATH:" in
    *":$dir:"*) return 0 ;;
    *) return 1 ;;
  esac
}

main() {
  parse_args "$@"

  require_cmd curl
  require_cmd tar
  require_cmd awk

  OS=$(detect_os)
  ARCH=$(detect_arch)
  VERSION=$(resolve_version)
  ARCHIVE_NAME="TaskWing_${OS}_${ARCH}.tar.gz"
  DOWNLOAD_URL="$DOWNLOAD_URL_BASE/$VERSION/$ARCHIVE_NAME"

  TMP_DIR=$(create_tmpdir)
  trap 'rm -rf "$TMP_DIR"' EXIT INT HUP TERM

  log "Downloading TaskWing $VERSION ($OS/$ARCH)..."
  if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/$ARCHIVE_NAME"; then
    fatal "Failed to download release asset: $DOWNLOAD_URL"
  fi

  log "Extracting archive..."
  if ! tar -xzf "$TMP_DIR/$ARCHIVE_NAME" -C "$TMP_DIR"; then
    fatal "Failed to extract archive $ARCHIVE_NAME"
  fi

  BIN_PATH=""
  if [ -f "$TMP_DIR/$BINARY_NAME" ]; then
    BIN_PATH="$TMP_DIR/$BINARY_NAME"
  else
    BIN_PATH=$(find "$TMP_DIR" -type f -name "$BINARY_NAME" -perm -111 2>/dev/null | head -n1)
  fi

  if [ -z "$BIN_PATH" ] || [ ! -f "$BIN_PATH" ]; then
    fatal "TaskWing binary not found in release archive"
  fi

  INSTALL_DIR=$(resolve_install_dir)
  log "Installing to $INSTALL_DIR..."
  INSTALLED_PATH=$(install_binary "$BIN_PATH" "$INSTALL_DIR")

  log "TaskWing installed to $INSTALLED_PATH"

  if verify_in_path "$INSTALL_DIR"; then
    log "Run 'taskwing version' to verify the installation."
  else
    log "Note: $INSTALL_DIR is not in your PATH. Add the following line to your shell profile:"
    printf '  export PATH="%s:$PATH"\n' "$INSTALL_DIR"
  fi

  if command -v "$INSTALLED_PATH" >/dev/null 2>&1; then
    if VERSION_OUTPUT=$("$INSTALLED_PATH" version 2>/dev/null); then
      log "Detected TaskWing version: $VERSION_OUTPUT"
    else
      log "Detected TaskWing version: unknown"
    fi
  fi
}

main "$@"
