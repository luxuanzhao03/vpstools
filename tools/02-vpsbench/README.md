# VPS Tools - VPS Bench

`02-vpsbench` is a one-command benchmark utility for VPS instances.
It measures CPU, memory, disk I/O, and network throughput in a single run.

Current benchmark coverage:

- CPU: single-core and multi-core SHA256 throughput
- Memory: sequential copy and fill bandwidth
- Disk I/O: sequential write/read throughput and `fsync` latency
- Network: multi-stream HTTP download/upload throughput

## One Step Run on a VPS

Run inside the module directory:

```bash
cd /opt/vps-tools/tools/02-vpsbench
chmod +x bootstrap.sh
./bootstrap.sh
```

The script will:

- Check whether `go` is installed
- Auto-install or upgrade Go when possible
- Build `./vps-bench`
- Run the full benchmark immediately

## Common CLI Usage

Run the benchmark directly:

```bash
./vps-bench
```

Write JSON report:

```bash
./vps-bench -json -out report.json
```

Skip public network test:

```bash
./vps-bench -skip-network
```

Increase disk file size for a more stable disk result:

```bash
./vps-bench -disk-size 1GiB
```

Use your own commercial network endpoints:

```bash
./vps-bench \
  -network-download-url https://speed.example.com/down.bin \
  -network-upload-url https://speed.example.com/upload
```

## Important Flags

- `-cpu-duration-sec`: multi-core CPU phase duration
- `-memory-duration-sec`: total memory phase duration
- `-memory-size`: memory buffer size, supports `MiB` and `GiB`
- `-disk-size`: temporary file size used for disk benchmark
- `-disk-block-size`: write/read block size
- `-disk-dir`: directory for the temporary disk file
- `-network-duration-sec`: total duration for download and upload phases
- `-network-streams`: parallel HTTP streams for network test
- `-network-download-url`: download endpoint
- `-network-upload-url`: upload endpoint
- `-network-upload-size`: upload payload size per request
- `-skip-network`: disable public network test
- `-strict`: exit non-zero when any benchmark phase fails

## Output Notes

- CPU output is SHA256 throughput, not an artificial point score.
- Memory output is bandwidth from Go-level copy/fill loops.
- Disk read throughput can be affected by page cache when the test file is smaller than available RAM.
- Default sizes are conservative so the tool can run on small VPS plans without exhausting memory or disk.

## Commercial Compliance Notes

- This module is implemented with the Go standard library only.
- The module license is MIT, so the source can be used in commercial projects.
- Public default network endpoints are only defaults for convenience. If you need large-scale commercial testing, point the tool at your own download/upload endpoints and follow the service terms of the endpoint provider you choose.

## License

MIT License:

- [LICENSE](./LICENSE)
