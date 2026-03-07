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

  echo "[toolbox] no interactive TTY detected"
  echo "[toolbox] run this script again in an interactive shell:"
  echo "[toolbox]   cd ${ROOT_DIR} && ./toolbox.sh"
  exit 1
}

clear_screen() {
  printf '\033[2J\033[H'
}

pause_for_enter() {
  printf '\nPress Enter to return to the main menu...'
  read -r _ || true
}

print_header() {
  cat <<'EOF'
VPS Tools
========================================================
1) Route and Latency Probe
   Measure outbound/return route and latency.

2) VPS Bench
   Benchmark CPU, memory, disk I/O, and network throughput.

0) Exit

Each tool installs or checks its own required dependencies when it starts.
EOF
  printf '\n'
}

ensure_module_bootstrap() {
  local module_dir="$1"
  local bootstrap="${module_dir}/bootstrap.sh"

  if [[ ! -f "$bootstrap" ]]; then
    echo "[toolbox] bootstrap not found: $bootstrap"
    return 1
  fi

  chmod +x "$bootstrap"
}

run_routeprobe() {
  local module_dir="${ROOT_DIR}/tools/01-routeprobe"
  ensure_module_bootstrap "$module_dir"
  (
    cd "$module_dir"
    ./bootstrap.sh --run-panel
  )
}

run_vpsbench() {
  local module_dir="${ROOT_DIR}/tools/02-vpsbench"
  ensure_module_bootstrap "$module_dir"
  (
    cd "$module_dir"
    ./bootstrap.sh
  )
}

main_menu() {
  ensure_interactive_tty

  while true; do
    clear_screen
    print_header
    printf 'Select a tool: '

    local choice
    if ! read -r choice; then
      printf '\n'
      return 0
    fi

    case "${choice,,}" in
      1)
        if ! run_routeprobe; then
          echo
          echo "[toolbox] route probe failed"
          pause_for_enter
        fi
        ;;
      2)
        if ! run_vpsbench; then
          echo
          echo "[toolbox] VPS bench failed"
        fi
        pause_for_enter
        ;;
      0|q|quit|exit)
        return 0
        ;;
      *)
        echo
        echo "[toolbox] invalid selection: ${choice}"
        pause_for_enter
        ;;
    esac
  done
}

main_menu "$@"
