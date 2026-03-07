#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

ensure_interactive_tty() {
  if [[ -t 0 && -t 1 ]]; then
    return 0
  fi

  if [[ -r /dev/tty && -w /dev/tty ]]; then
    exec </dev/tty >/dev/tty
    return 0
  fi

  echo "[工具箱] 未检测到可交互的终端"
  echo "[工具箱] 请在交互式终端中重新运行："
  echo "[工具箱]   cd ${ROOT_DIR} && ./toolbox.sh"
  exit 1
}

clear_screen() {
  printf '\033[2J\033[H'
}

pause_for_enter() {
  printf '\n按 Enter 返回主菜单...'
  read -r _ || true
}

print_header() {
  cat <<'EOF'
VPS 工具包 - 终端面板
========================================================
1) 去程回程线路探测
   测量本机到目标 IP 的去程/回程线路名称与延迟。

2) VPS 参数测试
   一键测试 CPU、内存、磁盘 I/O 与网络吞吐。

0) 退出

进入对应工具后，会自动检查并安装该工具所需依赖。
EOF
  printf '\n'
}

ensure_module_bootstrap() {
  local module_dir="$1"
  local bootstrap="${module_dir}/bootstrap.sh"

  if [[ ! -f "$bootstrap" ]]; then
    echo "[工具箱] 未找到启动脚本：$bootstrap"
    return 1
  fi
}

run_routeprobe() {
  local module_dir="${ROOT_DIR}/tools/01-routeprobe"
  ensure_module_bootstrap "$module_dir"
  (
    cd "$module_dir"
    bash ./bootstrap.sh --run-panel
  )
}

run_vpsbench() {
  local module_dir="${ROOT_DIR}/tools/02-vpsbench"
  ensure_module_bootstrap "$module_dir"
  (
    cd "$module_dir"
    bash ./bootstrap.sh
  )
}

main_menu() {
  ensure_interactive_tty

  while true; do
    clear_screen
    print_header
    printf '请选择编号：'

    local choice
    if ! read -r choice; then
      printf '\n'
      return 0
    fi

    case "${choice,,}" in
      1)
        if ! run_routeprobe; then
          echo
          echo "[工具箱] 线路探测启动失败"
          pause_for_enter
        fi
        ;;
      2)
        if ! run_vpsbench; then
          echo
          echo "[工具箱] VPS 参数测试启动失败"
        fi
        pause_for_enter
        ;;
      0|q|quit|exit)
        return 0
        ;;
      *)
        echo
        echo "[工具箱] 无效选择：${choice}"
        pause_for_enter
        ;;
    esac
  done
}

main_menu "$@"
