package main

import "runtime"

type systemInfo struct {
	OS               string `json:"os"`
	Arch             string `json:"arch"`
	KernelVersion    string `json:"kernel_version,omitempty"`
	GoVersion        string `json:"go_version"`
	CPUModel         string `json:"cpu_model,omitempty"`
	LogicalCores     int    `json:"logical_cores"`
	TotalMemoryBytes uint64 `json:"total_memory_bytes,omitempty"`
}

type platformSystemInfo struct {
	KernelVersion    string
	CPUModel         string
	TotalMemoryBytes uint64
}

func detectSystemInfo() systemInfo {
	platform := detectPlatformSystemInfo()
	return systemInfo{
		OS:               runtime.GOOS,
		Arch:             runtime.GOARCH,
		KernelVersion:    platform.KernelVersion,
		GoVersion:        runtime.Version(),
		CPUModel:         platform.CPUModel,
		LogicalCores:     runtime.NumCPU(),
		TotalMemoryBytes: platform.TotalMemoryBytes,
	}
}
