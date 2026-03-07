package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

const (
	defaultCPUDurationSec     = 4
	defaultMemoryDurationSec  = 4
	defaultNetworkDurationSec = 8
	defaultHTTPTimeoutSec     = 20
	defaultDownloadURL        = "https://speed.cloudflare.com/__down?bytes=25000000"
	defaultUploadURL          = "https://speed.cloudflare.com/__up"
)

type config struct {
	JSONOnly                  bool
	OutputPath                string
	Strict                    bool
	SkipCPU                   bool
	SkipMemory                bool
	SkipDisk                  bool
	SkipNetwork               bool
	CPUDuration               time.Duration
	MemoryDuration            time.Duration
	MemoryBufferBytes         int64
	DiskFileBytes             int64
	DiskBlockBytes            int
	DiskDir                   string
	NetworkDuration           time.Duration
	NetworkStreams            int
	NetworkDownloadURL        string
	NetworkUploadURL          string
	NetworkUploadPayloadBytes int
	HTTPTimeout               time.Duration
}

type report struct {
	Timestamp string         `json:"timestamp"`
	Hostname  string         `json:"hostname"`
	System    systemInfo     `json:"system"`
	Settings  benchSettings  `json:"settings"`
	CPU       *cpuResult     `json:"cpu,omitempty"`
	Memory    *memoryResult  `json:"memory,omitempty"`
	Disk      *diskResult    `json:"disk,omitempty"`
	Network   *networkResult `json:"network,omitempty"`
	Errors    []string       `json:"errors,omitempty"`
}

type benchSettings struct {
	CPUDurationSec            float64 `json:"cpu_duration_sec"`
	MemoryDurationSec         float64 `json:"memory_duration_sec"`
	MemoryBufferBytes         int64   `json:"memory_buffer_bytes"`
	DiskFileBytes             int64   `json:"disk_file_bytes"`
	DiskBlockBytes            int     `json:"disk_block_bytes"`
	DiskDir                   string  `json:"disk_dir"`
	NetworkDurationSec        float64 `json:"network_duration_sec"`
	NetworkStreams            int     `json:"network_streams"`
	NetworkDownloadURL        string  `json:"network_download_url,omitempty"`
	NetworkUploadURL          string  `json:"network_upload_url,omitempty"`
	NetworkUploadPayloadBytes int     `json:"network_upload_payload_bytes"`
	HTTPTimeoutSec            float64 `json:"http_timeout_sec"`
}

type cpuResult struct {
	Workers               int     `json:"workers"`
	BufferBytes           int     `json:"buffer_bytes"`
	SingleCoreDurationSec float64 `json:"single_core_duration_sec"`
	MultiCoreDurationSec  float64 `json:"multi_core_duration_sec"`
	SingleCoreIterations  uint64  `json:"single_core_iterations"`
	MultiCoreIterations   uint64  `json:"multi_core_iterations"`
	SingleCoreMiBPS       float64 `json:"single_core_sha256_mib_per_sec,omitempty"`
	MultiCoreMiBPS        float64 `json:"multi_core_sha256_mib_per_sec,omitempty"`
	Error                 string  `json:"error,omitempty"`
}

type memoryResult struct {
	BufferBytes     int64   `json:"buffer_bytes"`
	CopyDurationSec float64 `json:"copy_duration_sec"`
	FillDurationSec float64 `json:"fill_duration_sec"`
	CopyGiBPS       float64 `json:"copy_gib_per_sec,omitempty"`
	FillGiBPS       float64 `json:"fill_gib_per_sec,omitempty"`
	Error           string  `json:"error,omitempty"`
}

type diskResult struct {
	Directory        string  `json:"directory"`
	FileBytes        int64   `json:"file_bytes"`
	BlockBytes       int     `json:"block_bytes"`
	WriteDurationSec float64 `json:"write_duration_sec,omitempty"`
	ReadDurationSec  float64 `json:"read_duration_sec,omitempty"`
	WriteMiBPS       float64 `json:"write_mib_per_sec,omitempty"`
	ReadMiBPS        float64 `json:"read_mib_per_sec,omitempty"`
	FsyncMs          float64 `json:"fsync_ms,omitempty"`
	Error            string  `json:"error,omitempty"`
}

type networkResult struct {
	Streams  int                   `json:"streams"`
	Download networkEndpointResult `json:"download"`
	Upload   networkEndpointResult `json:"upload"`
}

type networkEndpointResult struct {
	URL            string  `json:"url,omitempty"`
	DurationSec    float64 `json:"duration_sec,omitempty"`
	Bytes          uint64  `json:"bytes,omitempty"`
	Requests       uint64  `json:"requests,omitempty"`
	ThroughputMbps float64 `json:"throughput_mbps,omitempty"`
	Error          string  `json:"error,omitempty"`
}

func main() {
	var (
		jsonOnlyFlag        = flag.Bool("json", false, "仅输出 JSON 报告")
		outputFlag          = flag.String("out", "", "将 JSON 报告写入文件")
		strictFlag          = flag.Bool("strict", false, "任一测试失败时返回非 0 退出码")
		skipCPUFlag         = flag.Bool("skip-cpu", false, "跳过 CPU 测试")
		skipMemoryFlag      = flag.Bool("skip-memory", false, "跳过内存测试")
		skipDiskFlag        = flag.Bool("skip-disk", false, "跳过磁盘测试")
		skipNetworkFlag     = flag.Bool("skip-network", false, "跳过网络测试")
		cpuDurationFlag     = flag.Int("cpu-duration-sec", defaultCPUDurationSec, "CPU 测试时长，单位秒")
		memoryDurationFlag  = flag.Int("memory-duration-sec", defaultMemoryDurationSec, "内存测试总时长，单位秒")
		memorySizeFlag      = flag.String("memory-size", "", "内存测试缓冲区大小，例如 64MiB、1GiB")
		diskSizeFlag        = flag.String("disk-size", "", "磁盘测试文件大小，例如 256MiB、2GiB")
		diskBlockSizeFlag   = flag.String("disk-block-size", "1MiB", "磁盘 I/O 块大小")
		diskDirFlag         = flag.String("disk-dir", "", "磁盘测试临时文件目录")
		networkDurationFlag = flag.Int("network-duration-sec", defaultNetworkDurationSec, "网络测试时长，单位秒")
		networkStreamsFlag  = flag.Int("network-streams", 4, "网络测试并发 HTTP 流数量")
		downloadURLFlag     = flag.String("network-download-url", defaultDownloadURL, "HTTP 下载测速地址")
		uploadURLFlag       = flag.String("network-upload-url", defaultUploadURL, "HTTP 上传测速地址")
		uploadSizeFlag      = flag.String("network-upload-size", "4MiB", "每次上传请求的负载大小")
		httpTimeoutFlag     = flag.Int("http-timeout-sec", defaultHTTPTimeoutSec, "单次 HTTP 请求超时秒数")
	)
	flag.Parse()

	system := detectSystemInfo()
	cfg, err := buildConfig(
		system,
		*jsonOnlyFlag,
		*outputFlag,
		*strictFlag,
		*skipCPUFlag,
		*skipMemoryFlag,
		*skipDiskFlag,
		*skipNetworkFlag,
		*cpuDurationFlag,
		*memoryDurationFlag,
		*memorySizeFlag,
		*diskSizeFlag,
		*diskBlockSizeFlag,
		*diskDirFlag,
		*networkDurationFlag,
		*networkStreamsFlag,
		*downloadURLFlag,
		*uploadURLFlag,
		*uploadSizeFlag,
		*httpTimeoutFlag,
	)
	if err != nil {
		exitWithError(err.Error())
	}

	rep := runBenchmarks(cfg, system)

	jsonBytes, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		exitWithError(fmt.Sprintf("序列化报告失败：%v", err))
	}

	if cfg.OutputPath != "" {
		if err := os.WriteFile(cfg.OutputPath, jsonBytes, 0644); err != nil {
			exitWithError(fmt.Sprintf("写入 %s 失败：%v", cfg.OutputPath, err))
		}
	}

	if cfg.JSONOnly {
		fmt.Println(string(jsonBytes))
	} else {
		fmt.Print(formatHumanReport(rep))
	}

	if cfg.Strict && len(rep.Errors) > 0 {
		os.Exit(1)
	}
}

func buildConfig(
	system systemInfo,
	jsonOnly bool,
	outputPath string,
	strict bool,
	skipCPU bool,
	skipMemory bool,
	skipDisk bool,
	skipNetwork bool,
	cpuDurationSec int,
	memoryDurationSec int,
	memorySizeRaw string,
	diskSizeRaw string,
	diskBlockRaw string,
	diskDir string,
	networkDurationSec int,
	networkStreams int,
	downloadURL string,
	uploadURL string,
	uploadSizeRaw string,
	httpTimeoutSec int,
) (config, error) {
	if cpuDurationSec <= 0 {
		return config{}, fmt.Errorf("-cpu-duration-sec 必须大于 0")
	}
	if memoryDurationSec <= 0 {
		return config{}, fmt.Errorf("-memory-duration-sec 必须大于 0")
	}
	if networkDurationSec <= 0 {
		return config{}, fmt.Errorf("-network-duration-sec 必须大于 0")
	}
	if networkStreams <= 0 {
		return config{}, fmt.Errorf("-network-streams 必须大于 0")
	}
	if httpTimeoutSec <= 0 {
		return config{}, fmt.Errorf("-http-timeout-sec 必须大于 0")
	}

	memoryBytes, err := resolveByteFlag(memorySizeRaw, defaultMemoryBufferBytes(system.TotalMemoryBytes))
	if err != nil {
		return config{}, fmt.Errorf("-memory-size 参数无效：%w", err)
	}
	diskBytes, err := resolveByteFlag(diskSizeRaw, defaultDiskFileBytes(system.TotalMemoryBytes))
	if err != nil {
		return config{}, fmt.Errorf("-disk-size 参数无效：%w", err)
	}
	diskBlockBytes64, err := resolveByteFlag(diskBlockRaw, 1<<20)
	if err != nil {
		return config{}, fmt.Errorf("-disk-block-size 参数无效：%w", err)
	}
	uploadBytes64, err := resolveByteFlag(uploadSizeRaw, 4<<20)
	if err != nil {
		return config{}, fmt.Errorf("-network-upload-size 参数无效：%w", err)
	}

	if memoryBytes <= 0 {
		return config{}, fmt.Errorf("-memory-size 必须大于 0")
	}
	if diskBytes <= 0 {
		return config{}, fmt.Errorf("-disk-size 必须大于 0")
	}
	if diskBlockBytes64 <= 0 {
		return config{}, fmt.Errorf("-disk-block-size 必须大于 0")
	}
	if uploadBytes64 <= 0 {
		return config{}, fmt.Errorf("-network-upload-size 必须大于 0")
	}
	if diskBlockBytes64 > diskBytes {
		diskBlockBytes64 = diskBytes
	}

	if err := ensureAllocFitsInt("memory-size", memoryBytes); err != nil {
		return config{}, err
	}
	if err := ensureAllocFitsInt("disk-block-size", diskBlockBytes64); err != nil {
		return config{}, err
	}
	if err := ensureAllocFitsInt("network-upload-size", uploadBytes64); err != nil {
		return config{}, err
	}

	dir := strings.TrimSpace(diskDir)
	if dir == "" {
		dir = os.TempDir()
	}

	return config{
		JSONOnly:                  jsonOnly,
		OutputPath:                strings.TrimSpace(outputPath),
		Strict:                    strict,
		SkipCPU:                   skipCPU,
		SkipMemory:                skipMemory,
		SkipDisk:                  skipDisk,
		SkipNetwork:               skipNetwork,
		CPUDuration:               time.Duration(cpuDurationSec) * time.Second,
		MemoryDuration:            time.Duration(memoryDurationSec) * time.Second,
		MemoryBufferBytes:         memoryBytes,
		DiskFileBytes:             diskBytes,
		DiskBlockBytes:            int(diskBlockBytes64),
		DiskDir:                   dir,
		NetworkDuration:           time.Duration(networkDurationSec) * time.Second,
		NetworkStreams:            networkStreams,
		NetworkDownloadURL:        strings.TrimSpace(downloadURL),
		NetworkUploadURL:          strings.TrimSpace(uploadURL),
		NetworkUploadPayloadBytes: int(uploadBytes64),
		HTTPTimeout:               time.Duration(httpTimeoutSec) * time.Second,
	}, nil
}

func runBenchmarks(cfg config, system systemInfo) report {
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		hostname = "未知"
	}

	rep := report{
		Timestamp: time.Now().Format(time.RFC3339),
		Hostname:  hostname,
		System:    system,
		Settings: benchSettings{
			CPUDurationSec:            round2(cfg.CPUDuration.Seconds()),
			MemoryDurationSec:         round2(cfg.MemoryDuration.Seconds()),
			MemoryBufferBytes:         cfg.MemoryBufferBytes,
			DiskFileBytes:             cfg.DiskFileBytes,
			DiskBlockBytes:            cfg.DiskBlockBytes,
			DiskDir:                   cfg.DiskDir,
			NetworkDurationSec:        round2(cfg.NetworkDuration.Seconds()),
			NetworkStreams:            cfg.NetworkStreams,
			NetworkDownloadURL:        cfg.NetworkDownloadURL,
			NetworkUploadURL:          cfg.NetworkUploadURL,
			NetworkUploadPayloadBytes: cfg.NetworkUploadPayloadBytes,
			HTTPTimeoutSec:            round2(cfg.HTTPTimeout.Seconds()),
		},
	}

	if !cfg.SkipCPU {
		res := benchmarkCPU(cfg.CPUDuration, runtime.GOMAXPROCS(0))
		rep.CPU = &res
		if res.Error != "" {
			rep.Errors = append(rep.Errors, "CPU："+res.Error)
		}
	}

	if !cfg.SkipMemory {
		res := benchmarkMemory(cfg.MemoryDuration, cfg.MemoryBufferBytes)
		rep.Memory = &res
		if res.Error != "" {
			rep.Errors = append(rep.Errors, "内存："+res.Error)
		}
	}

	if !cfg.SkipDisk {
		res := benchmarkDisk(cfg.DiskDir, cfg.DiskFileBytes, cfg.DiskBlockBytes)
		rep.Disk = &res
		if res.Error != "" {
			rep.Errors = append(rep.Errors, "磁盘："+res.Error)
		}
	}

	if !cfg.SkipNetwork {
		res := benchmarkNetwork(cfg)
		rep.Network = &res
		if res.Download.Error != "" {
			rep.Errors = append(rep.Errors, "网络下载："+res.Download.Error)
		}
		if res.Upload.Error != "" {
			rep.Errors = append(rep.Errors, "网络上传："+res.Upload.Error)
		}
	}

	return rep
}

func resolveByteFlag(raw string, defaultValue int64) (int64, error) {
	if strings.TrimSpace(raw) == "" {
		return defaultValue, nil
	}
	return parseByteSize(raw)
}

func ensureAllocFitsInt(name string, value int64) error {
	if value > int64(^uint(0)>>1) {
		return fmt.Errorf("-%s 对当前平台来说过大", name)
	}
	return nil
}

func defaultMemoryBufferBytes(total uint64) int64 {
	const (
		minBuffer = int64(32 << 20)
		maxBuffer = int64(128 << 20)
	)
	if total == 0 {
		return 64 << 20
	}
	candidate := int64(total / 16)
	candidate = clampInt64(candidate, minBuffer, maxBuffer)
	return alignDown(candidate, 1<<20)
}

func defaultDiskFileBytes(total uint64) int64 {
	const (
		minFile = int64(64 << 20)
		maxFile = int64(256 << 20)
	)
	if total == 0 {
		return 128 << 20
	}
	candidate := int64(total / 8)
	candidate = clampInt64(candidate, minFile, maxFile)
	return alignDown(candidate, 1<<20)
}

func clampInt64(v, minValue, maxValue int64) int64 {
	if v < minValue {
		return minValue
	}
	if v > maxValue {
		return maxValue
	}
	return v
}

func alignDown(v, align int64) int64 {
	if align <= 0 {
		return v
	}
	if v < align {
		return align
	}
	return (v / align) * align
}

func exitWithError(msg string) {
	fmt.Fprintln(os.Stderr, "错误：", msg)
	os.Exit(1)
}
