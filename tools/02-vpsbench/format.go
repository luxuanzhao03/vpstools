package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

func formatHumanReport(rep report) string {
	var b strings.Builder

	b.WriteString("VPS 参数测试报告\n")
	b.WriteString(fmt.Sprintf("测试时间：%s\n", rep.Timestamp))
	b.WriteString(fmt.Sprintf("主机名称：%s\n", rep.Hostname))
	b.WriteString(fmt.Sprintf(
		"系统信息：%s/%s，逻辑核心 %d，Go %s",
		rep.System.OS,
		rep.System.Arch,
		rep.System.LogicalCores,
		rep.System.GoVersion,
	))
	if rep.System.CPUModel != "" {
		b.WriteString(fmt.Sprintf("，CPU %s", rep.System.CPUModel))
	}
	if rep.System.TotalMemoryBytes > 0 {
		b.WriteString(fmt.Sprintf("，内存 %s", formatBytesIEC(rep.System.TotalMemoryBytes)))
	}
	if rep.System.KernelVersion != "" {
		b.WriteString(fmt.Sprintf("，内核 %s", rep.System.KernelVersion))
	}
	b.WriteString("\n")

	if rep.CPU != nil {
		if rep.CPU.Error != "" {
			b.WriteString(fmt.Sprintf("CPU：测试失败：%s\n", rep.CPU.Error))
		} else {
			b.WriteString(fmt.Sprintf(
				"CPU：单核 SHA256 %s，多核 SHA256 %s，并发线程 %d\n",
				formatMiBPerSec(rep.CPU.SingleCoreMiBPS),
				formatMiBPerSec(rep.CPU.MultiCoreMiBPS),
				rep.CPU.Workers,
			))
		}
	}

	if rep.Memory != nil {
		if rep.Memory.Error != "" {
			b.WriteString(fmt.Sprintf("内存：测试失败：%s\n", rep.Memory.Error))
		} else {
			b.WriteString(fmt.Sprintf(
				"内存：拷贝 %s，填充 %s，缓冲区 %s\n",
				formatGiBPerSec(rep.Memory.CopyGiBPS),
				formatGiBPerSec(rep.Memory.FillGiBPS),
				formatBytesIEC(uint64(rep.Memory.BufferBytes)),
			))
		}
	}

	if rep.Disk != nil {
		if rep.Disk.Error != "" {
			b.WriteString(fmt.Sprintf("磁盘：测试失败：%s\n", rep.Disk.Error))
		} else {
			b.WriteString(fmt.Sprintf(
				"磁盘：写入 %s，读取 %s，fsync %.2f ms，测试文件 %s，目录 %s\n",
				formatMiBPerSec(rep.Disk.WriteMiBPS),
				formatMiBPerSec(rep.Disk.ReadMiBPS),
				rep.Disk.FsyncMs,
				formatBytesIEC(uint64(rep.Disk.FileBytes)),
				rep.Disk.Directory,
			))
		}
	}

	if rep.Network != nil {
		b.WriteString("网络：")
		if rep.Network.Download.Error == "" {
			b.WriteString(fmt.Sprintf(" 下载 %s", formatMbps(rep.Network.Download.ThroughputMbps)))
		} else {
			b.WriteString(fmt.Sprintf(" 下载失败（%s）", rep.Network.Download.Error))
		}
		if rep.Network.Upload.Error == "" {
			b.WriteString(fmt.Sprintf("，上传 %s", formatMbps(rep.Network.Upload.ThroughputMbps)))
		} else {
			b.WriteString(fmt.Sprintf("，上传失败（%s）", rep.Network.Upload.Error))
		}
		b.WriteString(fmt.Sprintf("，并发流 %d\n", rep.Network.Streams))
	}

	if len(rep.Errors) > 0 {
		b.WriteString("提示信息：\n")
		for _, err := range rep.Errors {
			b.WriteString("- ")
			b.WriteString(err)
			b.WriteString("\n")
		}
	}

	return b.String()
}

func parseByteSize(raw string) (int64, error) {
	input := strings.TrimSpace(raw)
	if input == "" {
		return 0, fmt.Errorf("大小不能为空")
	}

	input = strings.ReplaceAll(input, " ", "")
	split := 0
	for split < len(input) {
		ch := input[split]
		if (ch >= '0' && ch <= '9') || ch == '.' {
			split++
			continue
		}
		break
	}
	if split == 0 {
		return 0, fmt.Errorf("缺少数字部分")
	}

	numberPart := input[:split]
	unitPart := strings.ToUpper(input[split:])

	value, err := strconv.ParseFloat(numberPart, 64)
	if err != nil {
		return 0, fmt.Errorf("数字部分解析失败：%w", err)
	}
	if value <= 0 {
		return 0, fmt.Errorf("大小必须大于 0")
	}

	multiplier, ok := byteUnits[unitPart]
	if !ok {
		return 0, fmt.Errorf("不支持的单位 %q", input[split:])
	}

	size := value * multiplier
	if size > math.MaxInt64 {
		return 0, fmt.Errorf("大小超出 int64 范围")
	}

	return int64(size), nil
}

var byteUnits = map[string]float64{
	"":    1,
	"B":   1,
	"K":   1000,
	"KB":  1000,
	"M":   1000 * 1000,
	"MB":  1000 * 1000,
	"G":   1000 * 1000 * 1000,
	"GB":  1000 * 1000 * 1000,
	"T":   1000 * 1000 * 1000 * 1000,
	"TB":  1000 * 1000 * 1000 * 1000,
	"KI":  1024,
	"KIB": 1024,
	"MI":  1024 * 1024,
	"MIB": 1024 * 1024,
	"GI":  1024 * 1024 * 1024,
	"GIB": 1024 * 1024 * 1024,
	"TI":  1024 * 1024 * 1024 * 1024,
	"TIB": 1024 * 1024 * 1024 * 1024,
}

func formatBytesIEC(value uint64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	if value < 1024 {
		return fmt.Sprintf("%d B", value)
	}

	v := float64(value)
	unit := 0
	for v >= 1024 && unit < len(units)-1 {
		v /= 1024
		unit++
	}

	return fmt.Sprintf("%.2f %s", v, units[unit])
}

func formatMiBPerSec(value float64) string {
	return fmt.Sprintf("%.2f MiB/s", value)
}

func formatGiBPerSec(value float64) string {
	return fmt.Sprintf("%.2f GiB/s", value)
}

func formatMbps(value float64) string {
	return fmt.Sprintf("%.2f Mbps", value)
}

func bytesPerSecondMiB(bytes uint64, elapsed time.Duration) float64 {
	if elapsed <= 0 {
		return 0
	}
	return float64(bytes) / elapsed.Seconds() / (1024 * 1024)
}

func bytesPerSecondGiB(bytes uint64, elapsed time.Duration) float64 {
	if elapsed <= 0 {
		return 0
	}
	return float64(bytes) / elapsed.Seconds() / (1024 * 1024 * 1024)
}

func bitsPerSecondMbps(bytes uint64, elapsed time.Duration) float64 {
	if elapsed <= 0 {
		return 0
	}
	return float64(bytes) * 8 / elapsed.Seconds() / 1_000_000
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
