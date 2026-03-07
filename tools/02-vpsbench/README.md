# VPS 工具包 - VPS 参数测试

`02-vpsbench` 是一个面向 VPS 的一键参数测试工具。
它可以在一次运行中测量 CPU、内存、磁盘 I/O 和网络吞吐。

## 测试内容

- CPU：单核与多核 SHA256 吞吐
- 内存：顺序拷贝与填充带宽
- 磁盘 I/O：顺序写入、顺序读取、`fsync` 延迟
- 网络：多线程 HTTP 下载/上传吞吐

## 在 VPS 上一键运行

进入模块目录后执行：

```bash
cd /opt/vps-tools/tools/02-vpsbench
chmod +x bootstrap.sh
./bootstrap.sh
```

脚本会自动：

- 检查是否已安装 `go`
- 尝试自动安装或升级 Go
- 构建 `./vps-bench`
- 直接开始完整测试

## 常用命令

直接运行测试：

```bash
./vps-bench
```

输出 JSON 报告：

```bash
./vps-bench -json -out report.json
```

跳过公网网络测试：

```bash
./vps-bench -skip-network
```

增大磁盘测试文件，提升结果稳定性：

```bash
./vps-bench -disk-size 1GiB
```

使用你自己的商用测速端点：

```bash
./vps-bench \
  -network-download-url https://speed.example.com/down.bin \
  -network-upload-url https://speed.example.com/upload
```

## 重要参数

- `-cpu-duration-sec`：CPU 测试时长
- `-memory-duration-sec`：内存测试总时长
- `-memory-size`：内存测试缓冲区大小，支持 `MiB`、`GiB`
- `-disk-size`：磁盘测试文件大小
- `-disk-block-size`：磁盘读写块大小
- `-disk-dir`：磁盘测试临时文件目录
- `-network-duration-sec`：网络测试总时长
- `-network-streams`：网络测试并发 HTTP 流数量
- `-network-download-url`：下载测速地址
- `-network-upload-url`：上传测速地址
- `-network-upload-size`：每次上传请求的负载大小
- `-skip-network`：跳过公网网络测试
- `-strict`：任意测试失败时返回非 0 退出码

## 结果说明

- CPU 结果是 SHA256 吞吐，不是虚构跑分。
- 内存结果来自 Go 层的拷贝/填充循环带宽。
- 当测试文件小于可用内存时，磁盘读取结果可能受到页缓存影响。
- 默认测试规模较保守，避免在小内存、小磁盘 VPS 上直接跑满资源。

## 商用说明

- 本模块仅使用 Go 标准库实现。
- 模块采用 MIT 许可证，可用于商业项目。
- 默认公网测速地址只是便捷默认值。如果你要做长期或大规模商用测试，建议替换成你自己的下载/上传测速端点，并自行遵守测速服务提供方条款。

## 许可证

MIT License：

- [LICENSE](./LICENSE)
