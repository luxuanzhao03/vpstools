package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const userAgent = "vps-bench/1.0"

func benchmarkNetwork(cfg config) networkResult {
	result := networkResult{
		Streams: cfg.NetworkStreams,
	}

	client := newHTTPClient(cfg.HTTPTimeout, cfg.NetworkStreams)
	result.Download = benchmarkDownload(client, cfg.NetworkDownloadURL, cfg.NetworkDuration, cfg.NetworkStreams)
	result.Upload = benchmarkUpload(client, cfg.NetworkUploadURL, cfg.NetworkDuration, cfg.NetworkStreams, cfg.NetworkUploadPayloadBytes)

	return result
}

func newHTTPClient(timeout time.Duration, streams int) *http.Client {
	if streams < 1 {
		streams = 1
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          streams * 2,
		MaxIdleConnsPerHost:   streams * 2,
		MaxConnsPerHost:       streams * 2,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    true,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}

func benchmarkDownload(client *http.Client, rawURL string, duration time.Duration, workers int) networkEndpointResult {
	result := networkEndpointResult{
		URL: rawURL,
	}

	if strings.TrimSpace(rawURL) == "" {
		result.Error = "download url is empty"
		return result
	}
	if duration <= 0 {
		result.Error = "download duration must be > 0"
		return result
	}

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var totalBytes uint64
	var totalRequests uint64
	var firstErr string
	var once sync.Once

	setError := func(err error) {
		if err == nil || ctx.Err() != nil {
			return
		}
		once.Do(func() {
			firstErr = err.Error()
			cancel()
		})
	}

	start := time.Now()
	deadline := start.Add(duration)
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()

			for time.Now().Before(deadline) {
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
				if err != nil {
					setError(err)
					return
				}
				req.Header.Set("User-Agent", userAgent)

				resp, err := client.Do(req)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					setError(err)
					return
				}

				if resp.StatusCode < 200 || resp.StatusCode >= 300 {
					body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
					resp.Body.Close()
					setError(fmt.Errorf("download returned %s: %s", resp.Status, strings.TrimSpace(string(body))))
					return
				}

				n, err := io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					setError(err)
					return
				}

				atomic.AddUint64(&totalBytes, uint64(n))
				atomic.AddUint64(&totalRequests, 1)
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	result.DurationSec = round2(elapsed.Seconds())
	result.Bytes = totalBytes
	result.Requests = totalRequests
	result.ThroughputMbps = round2(bitsPerSecondMbps(totalBytes, elapsed))
	result.Error = firstErr
	if result.Error == "" && ctx.Err() == context.DeadlineExceeded && totalBytes == 0 {
		result.Error = "download timed out without a successful transfer"
	}
	if totalBytes == 0 && result.Error == "" {
		result.Error = "no data downloaded"
	}

	return result
}

func benchmarkUpload(client *http.Client, rawURL string, duration time.Duration, workers int, payloadBytes int) networkEndpointResult {
	result := networkEndpointResult{
		URL: rawURL,
	}

	if strings.TrimSpace(rawURL) == "" {
		result.Error = "upload url is empty"
		return result
	}
	if duration <= 0 {
		result.Error = "upload duration must be > 0"
		return result
	}
	if payloadBytes <= 0 {
		result.Error = "upload payload size must be > 0"
		return result
	}

	payload := bytes.Repeat([]byte("0123456789abcdef"), payloadBytes/16+1)
	payload = payload[:payloadBytes]

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var totalBytes uint64
	var totalRequests uint64
	var firstErr string
	var once sync.Once

	setError := func(err error) {
		if err == nil || ctx.Err() != nil {
			return
		}
		once.Do(func() {
			firstErr = err.Error()
			cancel()
		})
	}

	start := time.Now()
	deadline := start.Add(duration)
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()

			for time.Now().Before(deadline) {
				body := bytes.NewReader(payload)
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, body)
				if err != nil {
					setError(err)
					return
				}
				req.Header.Set("Content-Type", "application/octet-stream")
				req.Header.Set("User-Agent", userAgent)

				resp, err := client.Do(req)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					setError(err)
					return
				}

				_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
				resp.Body.Close()
				if resp.StatusCode < 200 || resp.StatusCode >= 300 {
					setError(fmt.Errorf("upload returned %s", resp.Status))
					return
				}

				atomic.AddUint64(&totalBytes, uint64(payloadBytes))
				atomic.AddUint64(&totalRequests, 1)
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	result.DurationSec = round2(elapsed.Seconds())
	result.Bytes = totalBytes
	result.Requests = totalRequests
	result.ThroughputMbps = round2(bitsPerSecondMbps(totalBytes, elapsed))
	result.Error = firstErr
	if result.Error == "" && ctx.Err() == context.DeadlineExceeded && totalBytes == 0 {
		result.Error = "upload timed out without a successful transfer"
	}
	if totalBytes == 0 && result.Error == "" {
		result.Error = "no data uploaded"
	}

	return result
}
