# VPS Tools (Terminal Panel)

This module currently includes one built in tool: `Outbound/Return Route Probe`.
It is a **VPS terminal panel** (used over SSH), not a web browser panel.

## 1. One Step Initialization on VPS (Auto check/install Go)

Run inside the tool directory (not repository root):

```bash
cd /var/vpstry/vpscesu/tools/01-routeprobe
chmod +x bootstrap.sh
./bootstrap.sh
```

The script will automatically:

- Check whether `go` is installed
- If missing or lower than `1.18`, try package manager first (`apt/dnf/yum/zypper/pacman/apk`)
- If package manager version is insufficient, try multiple Go download mirrors (including China friendly mirrors)
- Build `./vps-tools`

Then start terminal panel:

```bash
./vps-tools -panel
```

Or build and run panel in one command:

```bash
./bootstrap.sh --run-panel
```

## 2. Panel Usage

After startup, terminal menu shows:

- `1` Outbound/Return Route Probe
- `0` Exit

Fill prompts and run probe. By default it shows simple results (route name + latency for outbound/return). You can optionally view full JSON and save to file.

Panel output behavior:

- Targets are shown with Chinese region labels (for example: China Mainland, US West Coast)
- If route is not recognized, the tool automatically retries with multiple probe strategies

Target input behavior:

- If user enters targets manually: use user provided targets
- If empty input or separators only: use default targets
- Default targets: `223.5.5.5` (China Mainland) and `74.82.42.42` (US West Coast)

Return path input behavior:

- If `return SSH mapping` is provided: do real return probe from target host
- If SSH is not provided and third party is enabled: use third party probe as approximate return path
- `local reachable IP` can be empty: program auto detects local IP (fill manually only when auto detection fails)

## 3. CLI Mode (without panel)

If you do not want panel mode:

```bash
go run . -targets "1.1.1.1,8.8.8.8" -out report.json
```

## 4. Probe Capability

- Outbound path: local host -> target (`traceroute` / `tracert`)
- Return path:
  - Mode A: target -> local (run `traceroute` on target host through SSH)
  - Mode B: third party probe -> local (approximate return, default provider `globalping`)
- Per hop latency, destination latency inference, optional ping summary
- Heuristic route naming (CT163, CU169, CU9929, CN2/CN2-GIA, etc.)
- Auto retry:
  - Outbound: UDP/ICMP/TCP retry
  - Third party return: multi probe and multi region retry
- Built in route database:
  - Maintain CIDR/ASN mappings via `route_db.json`
  - Base data from public carrier ASN references (for example PeeringDB) + common backbone patterns

Note: third party mode is an **approximate** return path, not a true on target return path.

## 5. Auto Dependency Installation

Auto checks and installs dependencies on Linux:

- Local: `traceroute` (or `tracert` on Windows), `ping`, `ssh`
- Remote: checks `traceroute` on target host in SSH return mode

Default: `-auto-install-deps=true`

Notes:

- Requires root or sudo privileges
- If sudo asks for interactive password, prefer running as root or configure passwordless sudo

## 6. Common CLI Flags

- `-panel`: run terminal panel mode
- `-targets`: target addresses (comma separated)
- `-reverse-ssh`: return SSH mapping, format `target=user@host`
- `-local-ip`: local reachable IP for return probe (optional, auto detected when empty)
- `-max-hops`: max hops, default `30`
- `-wait-sec`: per probe timeout seconds, default `2`
- `-queries-per-hop`: probes per hop, default `3`
- `-ping-count`: ping count, default `4`, set `0` to disable
- `-no-dns`: disable DNS reverse lookup
- `-cmd-timeout-sec`: command timeout, default `120`
- `-auto-install-deps`: auto install dependencies, default `true`
- `-include-raw`: include raw command output
- `-out`: output JSON path

Third party return flags:

- `-thirdparty-return`: enable third party return (used when SSH mapping is missing)
- `-thirdparty-provider`: third party provider (currently `globalping`)
- `-thirdparty-location`: probe location hint (empty = near target)
- `-thirdparty-token`: API token (or env `GLOBALPING_TOKEN`)
- `-thirdparty-limit`: probe count (default `1`)
- `-thirdparty-timeout-sec`: timeout for third party measurement (default `90`)

## 7. Commercial Compliance Notes

This module is self implemented and does not directly copy GPL code.
Implementation ideas and licenses referenced:

- `aeden/traceroute` (MIT): <https://github.com/aeden/traceroute>
- `pixelbender/go-traceroute` (MIT): <https://github.com/pixelbender/go-traceroute>
- `prometheus-community/pro-bing` (MIT): <https://github.com/prometheus-community/pro-bing>
- `jsdelivr/globalping` (open source project, third party probe network): <https://github.com/jsdelivr/globalping>
- `nxtrace/NTrace-core` (GPL-3.0): <https://github.com/nxtrace/NTrace-core>

For closed source commercial usage, prefer MIT/BSD/Apache dependencies and avoid directly integrating GPL code.
