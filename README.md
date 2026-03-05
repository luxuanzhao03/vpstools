# VPS Tools

A terminal based toolkit for VPS operations.
Current release includes the first module: **Route and Latency Probe**.

## Current Module

- `01-routeprobe`: outbound and return route identification + latency measurement
- Runtime: terminal panel inside VPS (via SSH)
- User type: non technical users can run it by following prompts

## One Line Install + Run

Recommended one-liner (auto clone/update and enter terminal panel):

```bash
curl -fsSL https://raw.githubusercontent.com/<your-username>/<your-repo>/main/onekey.sh | bash -s -- https://github.com/<your-username>/<your-repo>.git
```

If you set your real repo URL as default inside `onekey.sh`, users can run an even shorter command:

```bash
curl -fsSL https://raw.githubusercontent.com/<your-username>/<your-repo>/main/onekey.sh | bash
```
## Quick Start

```bash
git clone https://github.com/<your-username>/<your-repo>.git
cd <your-repo>/tools/01-routeprobe
chmod +x bootstrap.sh
./bootstrap.sh --run-panel
```

## Repository Structure

```text
.
├─ tools/
│  └─ 01-routeprobe/
└─ README.md
```

## Documentation

- Module docs: [tools/01-routeprobe/README.md](tools/01-routeprobe/README.md)

## Roadmap

- [x] Route and latency probe
- [ ] More VPS network tools
- [ ] System inspection tools
- [ ] One click O&M helpers

## License

MIT License:

- [tools/01-routeprobe/LICENSE](tools/01-routeprobe/LICENSE)

