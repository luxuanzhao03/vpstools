package main

import "time"

func benchmarkMemory(duration time.Duration, bufferBytes int64) memoryResult {
	result := memoryResult{
		BufferBytes: bufferBytes,
	}

	if duration <= 0 {
		result.Error = "测试时长必须大于 0"
		return result
	}
	if bufferBytes <= 0 {
		result.Error = "缓冲区大小必须大于 0"
		return result
	}

	bufferSize := int(bufferBytes)
	src := make([]byte, bufferSize)
	dst := make([]byte, bufferSize)
	for i := range src {
		src[i] = byte(i)
	}

	copyDuration := duration / 2
	fillDuration := duration - copyDuration
	if copyDuration <= 0 {
		copyDuration = duration
		fillDuration = duration
	}
	if fillDuration <= 0 {
		fillDuration = copyDuration
	}

	copyBytes, copyElapsed := runCopyLoop(src, dst, copyDuration)
	fillBytes, fillElapsed := runFillLoop(dst, fillDuration)

	result.CopyDurationSec = round2(copyElapsed.Seconds())
	result.FillDurationSec = round2(fillElapsed.Seconds())
	result.CopyGiBPS = round2(bytesPerSecondGiB(copyBytes, copyElapsed))
	result.FillGiBPS = round2(bytesPerSecondGiB(fillBytes, fillElapsed))

	if copyBytes == 0 || fillBytes == 0 {
		result.Error = "内存测试未产生有效数据"
	}

	return result
}

func runCopyLoop(src, dst []byte, duration time.Duration) (uint64, time.Duration) {
	start := time.Now()
	deadline := start.Add(duration)
	var copied uint64

	for time.Now().Before(deadline) {
		copied += uint64(copy(dst, src))
		src[0] ^= dst[0]
	}

	return copied, time.Since(start)
}

func runFillLoop(buf []byte, duration time.Duration) (uint64, time.Duration) {
	start := time.Now()
	deadline := start.Add(duration)
	var filled uint64
	var value byte

	for time.Now().Before(deadline) {
		for i := range buf {
			buf[i] = value
		}
		value++
		filled += uint64(len(buf))
	}

	return filled, time.Since(start)
}
