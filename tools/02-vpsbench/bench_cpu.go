package main

import (
	"crypto/sha256"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

func benchmarkCPU(duration time.Duration, workers int) cpuResult {
	result := cpuResult{
		Workers:     workers,
		BufferBytes: 1 << 20,
	}

	if workers <= 0 {
		workers = 1
		result.Workers = 1
	}
	if duration <= 0 {
		result.Error = "测试时长必须大于 0"
		return result
	}

	singleDuration := duration
	if singleDuration > 2*time.Second {
		singleDuration = 2 * time.Second
	}

	prev := runtime.GOMAXPROCS(1)
	singleBytes, singleIterations, singleElapsed := runSHA256Workers(singleDuration, 1, result.BufferBytes)
	runtime.GOMAXPROCS(prev)

	prev = runtime.GOMAXPROCS(workers)
	multiBytes, multiIterations, multiElapsed := runSHA256Workers(duration, workers, result.BufferBytes)
	runtime.GOMAXPROCS(prev)

	result.SingleCoreDurationSec = round2(singleElapsed.Seconds())
	result.MultiCoreDurationSec = round2(multiElapsed.Seconds())
	result.SingleCoreIterations = singleIterations
	result.MultiCoreIterations = multiIterations
	result.SingleCoreMiBPS = round2(bytesPerSecondMiB(singleBytes, singleElapsed))
	result.MultiCoreMiBPS = round2(bytesPerSecondMiB(multiBytes, multiElapsed))

	if singleIterations == 0 || multiIterations == 0 {
		result.Error = "SHA256 测试未完成有效迭代"
	}

	return result
}

func runSHA256Workers(duration time.Duration, workers int, bufferBytes int) (uint64, uint64, time.Duration) {
	if workers <= 0 {
		workers = 1
	}
	if bufferBytes <= 0 {
		bufferBytes = 1 << 20
	}

	template := make([]byte, bufferBytes)
	for i := range template {
		template[i] = byte(i)
	}

	var totalIterations uint64
	deadline := time.Now().Add(duration)
	start := time.Now()

	var wg sync.WaitGroup
	wg.Add(workers)
	for workerID := 0; workerID < workers; workerID++ {
		go func(seed byte) {
			defer wg.Done()

			buf := make([]byte, len(template))
			copy(buf, template)
			var localIterations uint64

			for time.Now().Before(deadline) {
				sum := sha256.Sum256(buf)
				buf[0] ^= sum[0]
				buf[len(buf)-1] ^= seed
				localIterations++
			}

			atomic.AddUint64(&totalIterations, localIterations)
		}(byte(workerID))
	}

	wg.Wait()
	elapsed := time.Since(start)
	totalBytes := totalIterations * uint64(bufferBytes)
	return totalBytes, totalIterations, elapsed
}
