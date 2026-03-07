#!/usr/bin/env bash
set -euo pipefail

APP_NAME="vps-bench"
REQUIRED_GO_VERSION="${REQUIRED_GO_VERSION:-1.18.0}"
INSTALL_GO_VERSION="${INSTALL_GO_VERSION:-1.22.12}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

log() {
  printf "[bootstrap] %s\n" "$*"
}

err() {
  printf "[bootstrap] ERROR: %s\n" "$*" >&2
}

version_ge() {
  [ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -n1)" = "$2" ]
}

detect_go_version() {
  if ! command -v go >/dev/null 2>&1; then
    return 1
  fi

  local raw
  raw="$(go version 2>/dev/null || true)"
  raw="$(printf '%s' "$raw" | awk '{print $3}' | sed 's/^go//')"
  if [ -z "$raw" ]; then
    return 1
  fi

  printf '%s' "$raw"
  return 0
}

run_as_root() {
  if [ "$(id -u)" -eq 0 ]; then
    "$@"
  elif command -v sudo >/dev/null 2>&1; then
    sudo "$@"
  else
    err "need root or sudo for: $*"
    return 1
  fi
}

pm_name() {
  if command -v apt-get >/dev/null 2>&1; then
    printf 'apt-get'
    return 0
  fi
  if command -v dnf >/dev/null 2>&1; then
    printf 'dnf'
    return 0
  fi
  if command -v yum >/dev/null 2>&1; then
    printf 'yum'
    return 0
  fi
  if command -v zypper >/dev/null 2>&1; then
    printf 'zypper'
    return 0
  fi
  if command -v pacman >/dev/null 2>&1; then
    printf 'pacman'
    return 0
  fi
  if command -v apk >/dev/null 2>&1; then
    printf 'apk'
    return 0
  fi
  return 1
}

pm_update_if_needed() {
  local pm="$1"
  case "$pm" in
    apt-get)
      run_as_root apt-get update
      ;;
    pacman)
      run_as_root pacman -Sy
      ;;
  esac
}

pm_install() {
  local pm="$1"
  shift
  local pkgs=("$@")

  case "$pm" in
    apt-get)
      run_as_root apt-get install -y "${pkgs[@]}"
      ;;
    dnf)
      run_as_root dnf install -y "${pkgs[@]}"
      ;;
    yum)
      run_as_root yum install -y "${pkgs[@]}"
      ;;
    zypper)
      run_as_root zypper install -y "${pkgs[@]}"
      ;;
    pacman)
      run_as_root pacman -S --noconfirm "${pkgs[@]}"
      ;;
    apk)
      run_as_root apk add --no-cache "${pkgs[@]}"
      ;;
    *)
      err "unsupported package manager: $pm"
      return 1
      ;;
  esac
}

install_go_via_package_manager() {
  local pm pkg current
  if ! pm="$(pm_name)"; then
    log "package manager not found; skip package install"
    return 1
  fi

  log "trying package manager install via ${pm}"
  pm_update_if_needed "$pm"

  case "$pm" in
    apt-get)
      for pkg in golang-go golang; do
        if pm_install "$pm" "$pkg"; then
          if current="$(detect_go_version)"; then
            if version_ge "$current" "$REQUIRED_GO_VERSION"; then
              log "go ready from package manager: ${current}"
              return 0
            fi
          fi
        fi
      done
      ;;
    dnf|yum|zypper|pacman|apk)
      for pkg in golang go; do
        if pm_install "$pm" "$pkg"; then
          if current="$(detect_go_version)"; then
            if version_ge "$current" "$REQUIRED_GO_VERSION"; then
              log "go ready from package manager: ${current}"
              return 0
            fi
          fi
        fi
      done
      ;;
  esac

  return 1
}

ensure_fetch_tools() {
  local pm
  if command -v curl >/dev/null 2>&1 || command -v wget >/dev/null 2>&1; then
    return 0
  fi

  if ! pm="$(pm_name)"; then
    err "no downloader and no known package manager"
    return 1
  fi

  log "curl/wget not found, installing downloader tools"
  pm_update_if_needed "$pm"
  pm_install "$pm" curl wget ca-certificates tar
}

download_file() {
  local url="$1"
  local out="$2"

  if command -v curl >/dev/null 2>&1; then
    curl -fL --connect-timeout 10 --max-time 120 --retry 2 --retry-delay 1 "$url" -o "$out"
    return 0
  fi

  if command -v wget >/dev/null 2>&1; then
    wget --timeout=20 -O "$out" "$url"
    return 0
  fi

  return 1
}

install_go_official() {
  local os arch tmpdir tarball url current

  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"

  case "$os" in
    linux) ;;
    *) err "unsupported OS for tarball install: $os"; return 1 ;;
  esac

  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) err "unsupported arch for tarball install: $arch"; return 1 ;;
  esac

  ensure_fetch_tools

  tmpdir="$(mktemp -d)"
  tarball="${tmpdir}/go.tgz"

  for url in \
    "https://golang.google.cn/dl/go${INSTALL_GO_VERSION}.${os}-${arch}.tar.gz" \
    "https://mirrors.aliyun.com/golang/go${INSTALL_GO_VERSION}.${os}-${arch}.tar.gz" \
    "https://mirrors.cloud.tencent.com/golang/go${INSTALL_GO_VERSION}.${os}-${arch}.tar.gz" \
    "https://mirrors.ustc.edu.cn/golang/go${INSTALL_GO_VERSION}.${os}-${arch}.tar.gz" \
    "https://go.dev/dl/go${INSTALL_GO_VERSION}.${os}-${arch}.tar.gz" \
    "https://dl.google.com/go/go${INSTALL_GO_VERSION}.${os}-${arch}.tar.gz"; do

    rm -f "$tarball"
    log "trying download: ${url}"
    if ! download_file "$url" "$tarball"; then
      continue
    fi

    if [ ! -s "$tarball" ]; then
      continue
    fi

    if ! tar -tzf "$tarball" >/dev/null 2>&1; then
      continue
    fi

    log "installing Go into /usr/local/go"
    run_as_root rm -rf /usr/local/go
    run_as_root tar -C /usr/local -xzf "$tarball"

    if [ -x /usr/local/go/bin/go ]; then
      run_as_root ln -sf /usr/local/go/bin/go /usr/local/bin/go
      run_as_root ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt
    fi

    rm -rf "$tmpdir"

    if current="$(detect_go_version)" && version_ge "$current" "$REQUIRED_GO_VERSION"; then
      log "go ready from tarball: ${current}"
      return 0
    fi

    if [ -x /usr/local/go/bin/go ]; then
      export PATH="/usr/local/go/bin:${PATH}"
      if current="$(detect_go_version)" && version_ge "$current" "$REQUIRED_GO_VERSION"; then
        log "go ready from tarball: ${current}"
        return 0
      fi
    fi
  done

  rm -rf "$tmpdir"
  return 1
}

ensure_go() {
  local current

  if current="$(detect_go_version)"; then
    if version_ge "$current" "$REQUIRED_GO_VERSION"; then
      log "go detected: ${current} (meets >= ${REQUIRED_GO_VERSION})"
      return 0
    fi
  fi

  if install_go_via_package_manager; then
    return 0
  fi

  log "package manager path not enough; trying tarball mirrors"
  if install_go_official; then
    return 0
  fi

  err "unable to install Go automatically"
  err "please install Go >= ${REQUIRED_GO_VERSION} manually and re-run"
  return 1
}

build_binary() {
  cd "$SCRIPT_DIR"
  log "building ${APP_NAME}"
  go build -o "$APP_NAME" .
  log "build complete: ${SCRIPT_DIR}/${APP_NAME}"
}

main() {
  local mode="run"
  local args=("$@")

  if [ "${1:-}" = "--build-only" ]; then
    mode="build"
    args=("${@:2}")
  fi

  ensure_go
  build_binary

  if [ "$mode" = "run" ]; then
    log "starting benchmark"
    exec "${SCRIPT_DIR}/${APP_NAME}" "${args[@]}"
  fi

  log "done. next run: ./${APP_NAME}"
}

main "$@"
