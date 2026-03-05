package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

func runLocalTraceWithRetry(target string, cfg config) (traceResult, string) {
	best, raw := runLocalTrace(target, cfg)
	if !shouldRetryUnrecognized(best) {
		return best, raw
	}

	rawParts := make([]string, 0, 3)
	if strings.TrimSpace(raw) != "" {
		rawParts = append(rawParts, "# default\n"+raw)
	}

	for _, mode := range []string{"icmp", "tcp"} {
		candidate, cRaw := runLocalTraceByMode(target, cfg, mode)
		if strings.TrimSpace(cRaw) != "" {
			rawParts = append(rawParts, "# "+mode+"\n"+cRaw)
		}
		best = pickBetterTrace(best, candidate)
		if isRouteRecognized(best.Hops) {
			break
		}
	}

	if !isRouteRecognized(best.Hops) {
		best.Error = joinErrors(best.Error, "已自动重试多种探测方式，线路仍未识别")
	}
	return best, strings.TrimSpace(strings.Join(rawParts, "\n\n"))
}

func runLocalTraceByMode(target string, cfg config, mode string) (traceResult, string) {
	if runtime.GOOS == "windows" {
		return runLocalTrace(target, cfg)
	}
	if mode != "icmp" && mode != "tcp" {
		return runLocalTrace(target, cfg)
	}

	tr := traceResult{Direction: "outbound"}
	if _, err := exec.LookPath("traceroute"); err != nil {
		tr.Error = "traceroute command not found in PATH"
		return tr, ""
	}

	args := []string{
		"-m", strconv.Itoa(cfg.MaxHops),
		"-w", strconv.Itoa(cfg.WaitSec),
		"-q", strconv.Itoa(cfg.QueriesPerHop),
	}
	if mode == "icmp" {
		args = append(args, "-I")
	}
	if mode == "tcp" {
		args = append(args, "-T", "-p", "443")
	}
	if cfg.NoDNS {
		args = append(args, "-n")
	}
	args = append(args, target)

	tr.Command = buildCommandString("traceroute", args)
	output, runErr := runCommand("traceroute", args, cfg.CommandTimeoutSec)
	tr.Hops = parseTracerouteOutput(output)
	tr.DestinationRTTMs = inferDestinationRTT(tr.Hops)
	if runErr != nil {
		tr.Error = fmt.Sprintf("traceroute (%s) failed: %v", mode, runErr)
	}
	if len(tr.Hops) == 0 && tr.Error == "" {
		tr.Error = fmt.Sprintf("traceroute (%s) returned no hop data", mode)
	}
	return tr, output
}

func runThirdPartyReverseTraceWithRetry(target, localIP string, cfg config) (traceResult, string) {
	baseCfg := cfg
	if strings.TrimSpace(baseCfg.ThirdPartyLocation) == "" {
		baseCfg.ThirdPartyLocation = guessThirdPartyLocation(target)
	}
	if baseCfg.ThirdPartyProbeLimit < 2 {
		baseCfg.ThirdPartyProbeLimit = 2
	}

	best, raw := runThirdPartyReverseTrace(target, localIP, baseCfg)
	if !shouldRetryUnrecognized(best) {
		return best, raw
	}

	rawParts := make([]string, 0, 3)
	if strings.TrimSpace(raw) != "" {
		rawParts = append(rawParts, "# default\n"+raw)
	}

	retryCfg := baseCfg
	if retryCfg.ThirdPartyProbeLimit < 3 {
		retryCfg.ThirdPartyProbeLimit = 3
	}
	if strings.TrimSpace(retryCfg.ThirdPartyLocation) == "" {
		retryCfg.ThirdPartyLocation = guessThirdPartyLocation(target)
	}

	candidate, cRaw := runThirdPartyReverseTrace(target, localIP, retryCfg)
	if strings.TrimSpace(cRaw) != "" {
		rawParts = append(rawParts, "# retry-location\n"+cRaw)
	}
	best = pickBetterTrace(best, candidate)

	if !isRouteRecognized(best.Hops) {
		altCfg := retryCfg
		if strings.EqualFold(strings.TrimSpace(altCfg.ThirdPartyLocation), "CN") {
			altCfg.ThirdPartyLocation = "US"
		} else if strings.EqualFold(strings.TrimSpace(altCfg.ThirdPartyLocation), "US") {
			altCfg.ThirdPartyLocation = "CN"
		}
		if strings.TrimSpace(altCfg.ThirdPartyLocation) != strings.TrimSpace(retryCfg.ThirdPartyLocation) {
			candidate2, cRaw2 := runThirdPartyReverseTrace(target, localIP, altCfg)
			if strings.TrimSpace(cRaw2) != "" {
				rawParts = append(rawParts, "# retry-alt-location\n"+cRaw2)
			}
			best = pickBetterTrace(best, candidate2)
		}
	}

	if !isRouteRecognized(best.Hops) {
		best.Error = joinErrors(best.Error, "已自动重试多探针与多地区，线路仍未识别")
	}

	return best, strings.TrimSpace(strings.Join(rawParts, "\n\n"))
}

func shouldRetryUnrecognized(tr traceResult) bool {
	if len(tr.Hops) == 0 {
		return true
	}
	return !isRouteRecognized(tr.Hops)
}

func isRouteRecognized(hops []hopResult) bool {
	if len(hops) == 0 {
		return false
	}

	route := detectMajorRoute(hops)
	if route == "未识别" {
		return false
	}
	if isChinaCarrierRoute(route) {
		return true
	}

	// If only overseas backbone is visible, keep retrying to find a mainland carrier segment.
	return hasChinaCarrierSignal(hops)
}

func hasChinaCarrierSignal(hops []hopResult) bool {
	if route := detectRouteByHopDatabase(hops); isChinaCarrierRoute(route) {
		return true
	}

	text := collectPathText(hops)
	if text == "" {
		return false
	}

	if route := detectRouteByKeywords(text); isChinaCarrierRoute(route) {
		return true
	}
	if route := lookupRouteByTextDatabase(text); isChinaCarrierRoute(route) {
		return true
	}
	return false
}

func pickBetterTrace(a, b traceResult) traceResult {
	if traceQualityScore(b) > traceQualityScore(a) {
		return b
	}
	return a
}

func traceQualityScore(tr traceResult) int {
	score := 0
	if tr.Error == "" {
		score += 2
	}
	if isRouteRecognized(tr.Hops) {
		score += 50
	}
	if tr.DestinationRTTMs > 0 {
		score += 2
	}

	nonTimeout := 0
	for _, h := range tr.Hops {
		if !h.Timeout {
			nonTimeout++
		}
	}
	if nonTimeout > 20 {
		nonTimeout = 20
	}
	score += nonTimeout
	return score
}

func guessThirdPartyLocation(target string) string {
	key := strings.ToLower(strings.TrimSpace(target))
	switch key {
	case defaultChinaMainlandTarget:
		return "CN"
	case defaultUSWestTarget:
		return "US"
	}

	loc := strings.TrimSpace(resolveTargetLocationZH(target))
	if strings.Contains(loc, "中国") {
		return "CN"
	}
	if strings.Contains(loc, "美国") {
		return "US"
	}
	return ""
}




