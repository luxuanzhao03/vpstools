# VPS 工具包

一个面向 VPS 运维与测试的终端工具集。
当前版本提供统一工具箱入口，内置线路延迟测试与 VPS 参数测试两个模块。

## 当前模块

- `01-routeprobe`：去程/回程线路识别与延迟测试
- `02-vpsbench`：一键测试 CPU、内存、磁盘 I/O、网络吞吐
- 运行方式：SSH 终端内交互使用
- 适用人群：非技术用户也可以按提示完成测试

## 一键安装并进入主界面

推荐命令：

```bash
curl -fsSL https://raw.githubusercontent.com/luxuanzhao03/vpstools/main/onekey.sh | bash
```

执行后会自动：

- 安装或检查 `git`
- 克隆或更新仓库
- 进入工具箱主界面
- 由用户选择运行线路测试或 VPS 参数测试

## 手动启动

如果你已经克隆了仓库，可以直接运行工具箱主界面：

```bash
git clone https://github.com/luxuanzhao03/vpstools.git
cd vpstools
chmod +x toolbox.sh
./toolbox.sh
```

也可以直接启动某个模块。

线路测试模块：

```bash
cd tools/01-routeprobe
chmod +x bootstrap.sh
./bootstrap.sh --run-panel
```

VPS 参数测试模块：

```bash
cd tools/02-vpsbench
chmod +x bootstrap.sh
./bootstrap.sh
```

## 仓库结构

```text
.
├─ toolbox.sh
├─ tools/
│  ├─ 01-routeprobe/
│  └─ 02-vpsbench/
└─ README.md
```

## 模块说明

- [tools/01-routeprobe/README.md](tools/01-routeprobe/README.md)
- [tools/02-vpsbench/README.md](tools/02-vpsbench/README.md)

## 路线图

- [x] 线路识别与延迟测试
- [x] CPU / 内存 / 磁盘 I/O / 网络吞吐测试
- [ ] 更多网络测试工具
- [ ] 更多系统巡检工具
- [ ] 一键运维辅助工具

## 许可证

MIT License：

- [tools/01-routeprobe/LICENSE](tools/01-routeprobe/LICENSE)
- [tools/02-vpsbench/LICENSE](tools/02-vpsbench/LICENSE)
