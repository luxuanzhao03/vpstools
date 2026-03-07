package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func benchmarkDisk(dir string, fileBytes int64, blockBytes int) diskResult {
	result := diskResult{
		Directory:  dir,
		FileBytes:  fileBytes,
		BlockBytes: blockBytes,
	}

	if fileBytes <= 0 {
		result.Error = "file size must be > 0"
		return result
	}
	if blockBytes <= 0 {
		result.Error = "block size must be > 0"
		return result
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		result.Error = fmt.Sprintf("prepare directory failed: %v", err)
		return result
	}

	file, err := os.CreateTemp(dir, "vpsbench-*.bin")
	if err != nil {
		result.Error = fmt.Sprintf("create temp file failed: %v", err)
		return result
	}
	path := file.Name()
	defer os.Remove(path)

	buffer := make([]byte, blockBytes)
	for i := range buffer {
		buffer[i] = byte(i)
	}

	writeStart := time.Now()
	var written int64
	for written < fileBytes {
		chunk := blockBytes
		if remaining := fileBytes - written; remaining < int64(chunk) {
			chunk = int(remaining)
		}
		n, writeErr := file.Write(buffer[:chunk])
		if writeErr != nil {
			file.Close()
			result.Error = fmt.Sprintf("write %s failed: %v", filepath.Base(path), writeErr)
			return result
		}
		written += int64(n)
	}

	fsyncStart := time.Now()
	if err := file.Sync(); err != nil {
		file.Close()
		result.Error = fmt.Sprintf("fsync %s failed: %v", filepath.Base(path), err)
		return result
	}
	fsyncElapsed := time.Since(fsyncStart)
	writeElapsed := time.Since(writeStart)

	if err := file.Close(); err != nil {
		result.Error = fmt.Sprintf("close %s failed: %v", filepath.Base(path), err)
		return result
	}

	reader, err := os.Open(path)
	if err != nil {
		result.Error = fmt.Sprintf("reopen %s failed: %v", filepath.Base(path), err)
		return result
	}
	defer reader.Close()

	readStart := time.Now()
	readBytes, err := io.CopyBuffer(io.Discard, reader, buffer)
	readElapsed := time.Since(readStart)
	if err != nil && !errors.Is(err, io.EOF) {
		result.Error = fmt.Sprintf("read %s failed: %v", filepath.Base(path), err)
		return result
	}

	result.WriteDurationSec = round2(writeElapsed.Seconds())
	result.ReadDurationSec = round2(readElapsed.Seconds())
	result.WriteMiBPS = round2(bytesPerSecondMiB(uint64(written), writeElapsed))
	result.ReadMiBPS = round2(bytesPerSecondMiB(uint64(readBytes), readElapsed))
	result.FsyncMs = round2(fsyncElapsed.Seconds() * 1000)

	return result
}
