#!/usr/bin/env bash
set -euo pipefail

DEFAULT_REPO_URL="https://github.com/luxuanzhao03/vpstools.git"
REPO_URL="${1:-${VPS_TOOLS_REPO:-$DEFAULT_REPO_URL}}"
BRANCH="${VPS_TOOLS_BRANCH:-main}"
INSTALL_DIR="${VPS_TOOLS_DIR:-/opt/vps-tools}"

need_cmd() {
  command -v "$1" >/dev/null 2>&1
}

run_root() {
  if [[ "$(id -u)" -eq 0 ]]; then
    "$@"
  elif need_cmd sudo; then
    sudo "$@"
  else
    echo "[onekey] need root or sudo to install dependencies"
    exit 1
  fi
}

validate_install_dir() {
  case "$INSTALL_DIR" in
    ""|"/"|"."|"..")
      echo "[onekey] invalid install dir: $INSTALL_DIR"
      exit 1
      ;;
  esac
}

ensure_git() {
  if need_cmd git; then
    return
  fi

  echo "[onekey] git not found, installing..."
  if need_cmd apt-get; then
    run_root apt-get update
    run_root apt-get install -y git
  elif need_cmd dnf; then
    run_root dnf install -y git
  elif need_cmd yum; then
    run_root yum install -y git
  elif need_cmd zypper; then
    run_root zypper install -y git
  elif need_cmd pacman; then
    run_root pacman -Sy --noconfirm git
  elif need_cmd apk; then
    run_root apk add --no-cache git
  else
    echo "[onekey] unsupported package manager, please install git manually"
    exit 1
  fi
}

resolve_install_dir() {
  validate_install_dir

  if [[ -e "$INSTALL_DIR" ]]; then
    if [[ ! -w "$INSTALL_DIR" ]]; then
      local fallback="$HOME/.vps-tools"
      echo "[onekey] no write permission on $INSTALL_DIR, fallback to $fallback"
      INSTALL_DIR="$fallback"
    fi
  else
    if ! mkdir -p "$INSTALL_DIR" 2>/dev/null; then
      local fallback="$HOME/.vps-tools"
      echo "[onekey] cannot create $INSTALL_DIR, fallback to $fallback"
      INSTALL_DIR="$fallback"
      mkdir -p "$INSTALL_DIR"
    fi
  fi
}

sync_repo() {
  if [[ -d "$INSTALL_DIR/.git" ]]; then
    echo "[onekey] updating repo: $INSTALL_DIR"
    git -C "$INSTALL_DIR" fetch origin "$BRANCH"
    git -C "$INSTALL_DIR" checkout "$BRANCH"
    git -C "$INSTALL_DIR" pull --ff-only origin "$BRANCH"
    return
  fi

  if [[ -e "$INSTALL_DIR" && -n "$(ls -A "$INSTALL_DIR" 2>/dev/null || true)" ]]; then
    echo "[onekey] install dir exists and is not a git repo: $INSTALL_DIR"
    echo "[onekey] please set VPS_TOOLS_DIR to an empty directory, or clean it manually"
    exit 1
  fi

  echo "[onekey] cloning repo into: $INSTALL_DIR"
  git clone -b "$BRANCH" "$REPO_URL" "$INSTALL_DIR"
}

run_toolbox() {
  cd "$INSTALL_DIR"
  chmod +x toolbox.sh

  if [[ -t 0 && -t 1 ]]; then
    exec ./toolbox.sh
  fi

  if [[ -r /dev/tty && -w /dev/tty ]]; then
    echo "[onekey] non-interactive stdin detected, switching toolbox to /dev/tty"
    exec ./toolbox.sh </dev/tty >/dev/tty
  fi

  echo "[onekey] no interactive TTY detected, toolbox cannot be opened now"
  echo "[onekey] installed successfully. run this command later:"
  echo "[onekey]   cd $INSTALL_DIR && ./toolbox.sh"
}

ensure_git
resolve_install_dir
sync_repo
run_toolbox
