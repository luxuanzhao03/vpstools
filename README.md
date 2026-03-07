# VPS Tools

A terminal based toolkit for VPS operations.
Current release includes route probing, one-command VPS benchmarking, and a unified interactive toolbox entrypoint.

## Current Modules

- `01-routeprobe`: outbound and return route identification + latency measurement
- `02-vpsbench`: one-command CPU / memory / disk I/O / network throughput benchmark
- Runtime: terminal-based tools inside VPS (via SSH)
- User type: non technical users can run it by following prompts

## One Line Install + Run

Recommended one-liner (auto clone/update and enter the toolbox main menu):

```bash
curl -fsSL https://raw.githubusercontent.com/<your-username>/<your-repo>/main/onekey.sh | bash -s -- https://github.com/<your-username>/<your-repo>.git
```

If you set your real repo URL as default inside `onekey.sh`, users can run an even shorter command:

```bash
curl -fsSL https://raw.githubusercontent.com/<your-username>/<your-repo>/main/onekey.sh | bash
```

## Quick Start
One-line install and start the toolbox main menu:

```bash
curl -fsSL https://raw.githubusercontent.com/luxuanzhao03/vpstools/main/onekey.sh | bash
```

The toolbox main menu lets users choose the route probe or the VPS benchmark, and each module auto-installs its own required dependencies when launched.

Manual start from a cloned repo:

```bash
git clone https://github.com/<your-username>/<your-repo>.git
cd <your-repo>
chmod +x toolbox.sh
./toolbox.sh
```

Start the route probe module directly:

```bash
git clone https://github.com/<your-username>/<your-repo>.git
cd <your-repo>/tools/01-routeprobe
chmod +x bootstrap.sh
./bootstrap.sh --run-panel
```

Start the benchmark module directly:

```bash
git clone https://github.com/<your-username>/<your-repo>.git
cd <your-repo>/tools/02-vpsbench
chmod +x bootstrap.sh
./bootstrap.sh
```

## Repository Structure

```text
.
├─ toolbox.sh
├─ tools/
│  ├─ 01-routeprobe/
│  └─ 02-vpsbench/
└─ README.md
```

## Documentation

- [tools/01-routeprobe/README.md](tools/01-routeprobe/README.md)
- [tools/02-vpsbench/README.md](tools/02-vpsbench/README.md)

## Roadmap

- [x] Route and latency probe
- [x] CPU / memory / disk I/O / network throughput benchmark
- [ ] More VPS network tools
- [ ] More system inspection tools
- [ ] One click O&M helpers

## License

MIT License:

- [tools/01-routeprobe/LICENSE](tools/01-routeprobe/LICENSE)
- [tools/02-vpsbench/LICENSE](tools/02-vpsbench/LICENSE)

