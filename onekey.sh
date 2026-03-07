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
    echo "[onekey] 安装依赖需要 root 或 sudo 权限"
    exit 1
  fi
}

validate_install_dir() {
  case "$INSTALL_DIR" in
    ""|"/"|"."|"..")
      echo "[onekey] 安装目录无效：$INSTALL_DIR"
      exit 1
      ;;
  esac
}

ensure_git() {
  if need_cmd git; then
    return
  fi

  echo "[onekey] 未检测到 git，正在安装..."
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
    echo "[onekey] 当前系统的包管理器不受支持，请手动安装 git"
    exit 1
  fi
}

resolve_install_dir() {
  validate_install_dir

  if [[ -e "$INSTALL_DIR" ]]; then
    if [[ ! -w "$INSTALL_DIR" ]]; then
      local fallback="$HOME/.vps-tools"
      echo "[onekey] 对 $INSTALL_DIR 没有写权限，改用 $fallback"
      INSTALL_DIR="$fallback"
    fi
  else
    if ! mkdir -p "$INSTALL_DIR" 2>/dev/null; then
      local fallback="$HOME/.vps-tools"
      echo "[onekey] 无法创建 $INSTALL_DIR，改用 $fallback"
      INSTALL_DIR="$fallback"
      mkdir -p "$INSTALL_DIR"
    fi
  fi
}

sync_repo() {
  if [[ -d "$INSTALL_DIR/.git" ]]; then
    echo "[onekey] 正在更新仓库：$INSTALL_DIR"
    git -C "$INSTALL_DIR" fetch origin "$BRANCH"
    git -C "$INSTALL_DIR" checkout "$BRANCH"
    git -C "$INSTALL_DIR" pull --ff-only origin "$BRANCH"
    return
  fi

  if [[ -e "$INSTALL_DIR" && -n "$(ls -A "$INSTALL_DIR" 2>/dev/null || true)" ]]; then
    echo "[onekey] 安装目录已存在且不是 git 仓库：$INSTALL_DIR"
    echo "[onekey] 请将 VPS_TOOLS_DIR 指向一个空目录，或手动清理该目录"
    exit 1
  fi

  echo "[onekey] 正在克隆仓库到：$INSTALL_DIR"
  git clone -b "$BRANCH" "$REPO_URL" "$INSTALL_DIR"
}

run_toolbox() {
  cd "$INSTALL_DIR"
  chmod +x toolbox.sh

  if [[ -t 0 && -t 1 ]]; then
    exec ./toolbox.sh
  fi

  if [[ -r /dev/tty && -w /dev/tty ]]; then
    echo "[onekey] 检测到当前 stdin 非交互式，切换到 /dev/tty 打开工具箱"
    exec ./toolbox.sh </dev/tty >/dev/tty
  fi

  echo "[onekey] 未检测到可交互终端，暂时无法打开工具箱"
  echo "[onekey] 安装已完成，稍后可手动运行："
  echo "[onekey]   cd $INSTALL_DIR && ./toolbox.sh"
}

ensure_git
resolve_install_dir
sync_repo
run_toolbox
