package main

import (
	"strings"
	"sync"
)

type pingExecutionResult struct {
	summary *pingSummary
	cmd     string
	raw     string
	err     error
}

type reverseExecutionResult struct {
	trace *traceResult
	raw   string
}

var remoteTracerouteReady sync.Map

func remoteTracerouteCached(endpoint string) bool {
	_, ok := remoteTracerouteReady.Load(strings.TrimSpace(endpoint))
	return ok
}

func rememberRemoteTraceroute(endpoint string) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return
	}
	remoteTracerouteReady.Store(endpoint, struct{}{})
}

func prepareReverseDependencies(cfg config) error {
	if len(cfg.ReverseSSH) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(cfg.ReverseSSH))
	endpoints := make([]string, 0, len(cfg.ReverseSSH))
	for _, endpoint := range cfg.ReverseSSH {
		endpoint = strings.TrimSpace(endpoint)
		if endpoint == "" {
			continue
		}
		if _, ok := seen[endpoint]; ok {
			continue
		}
		seen[endpoint] = struct{}{}
		endpoints = append(endpoints, endpoint)
	}
	if len(endpoints) == 0 {
		return nil
	}

	parallelism := targetProbeWorkerLimit(len(endpoints))
	sem := make(chan struct{}, parallelism)
	errs := make(chan error, len(endpoints))

	var wg sync.WaitGroup
	for _, endpoint := range endpoints {
		endpoint := endpoint
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			errs <- ensureRemoteTraceroute(endpoint, cfg)
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}

func populateTargetProbes(results []targetProbe, cfg config, includeRaw bool) {
	if len(results) == 0 {
		return
	}

	parallelism := targetProbeWorkerLimit(len(results))
	sem := make(chan struct{}, parallelism)

	var wg sync.WaitGroup
	for i, target := range cfg.Targets {
		i := i
		target := target

		wg.Add(1)
		go func() {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			results[i] = buildTargetProbe(target, cfg, includeRaw)
		}()
	}

	wg.Wait()
}

func buildTargetProbe(target string, cfg config, includeRaw bool) targetProbe {
	res := targetProbe{Target: target}

	var (
		outbound   traceResult
		outboundRaw string
		ping       pingExecutionResult
		reverse    reverseExecutionResult
	)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		outbound, outboundRaw = runLocalTraceWithRetry(target, cfg)
	}()

	if cfg.PingCount > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ping.summary, ping.cmd, ping.raw, ping.err = runLocalPing(target, cfg)
		}()
	}

	if endpoint, ok := cfg.ReverseSSH[target]; ok {
		wg.Add(1)
		go func(endpoint string) {
			defer wg.Done()
			back, rawBack := runReverseTrace(endpoint, cfg.LocalIP, cfg)
			reverse.trace = &back
			reverse.raw = rawBack
		}(endpoint)
	} else if cfg.ThirdPartyReturn {
		wg.Add(1)
		go func() {
			defer wg.Done()
			back, rawBack := runThirdPartyReverseTraceWithRetry(target, cfg.LocalIP, cfg)
			reverse.trace = &back
			reverse.raw = rawBack
		}()
	}

	wg.Wait()

	if includeRaw {
		outbound.RawOutput = outboundRaw
	}

	if cfg.PingCount > 0 {
		if ping.cmd != "" {
			outbound.Command = strings.TrimSpace(outbound.Command + " | " + ping.cmd)
		}
		if ping.err != nil {
			outbound.Error = joinErrors(outbound.Error, "ping failed: "+ping.err.Error())
		} else {
			outbound.Ping = ping.summary
		}
		if includeRaw && ping.raw != "" {
			outbound.RawOutput = strings.TrimSpace(outbound.RawOutput + "\n\n# ping\n" + ping.raw)
		}
	}

	res.Outbound = outbound

	if reverse.trace != nil {
		if includeRaw {
			reverse.trace.RawOutput = reverse.raw
		}
		res.Return = reverse.trace
	}

	return res
}

func targetProbeWorkerLimit(total int) int {
	switch {
	case total <= 1:
		return 1
	case total <= 3:
		return total
	default:
		return 3
	}
}
