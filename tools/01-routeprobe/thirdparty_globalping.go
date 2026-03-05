package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

func runThirdPartyReverseTrace(target, localIP string, cfg config) (traceResult, string) {
	provider := strings.ToLower(strings.TrimSpace(cfg.ThirdPartyProvider))
	if provider == "" {
		provider = "globalping"
	}

	switch provider {
	case "globalping":
		return runGlobalpingReverseTrace(target, localIP, cfg)
	default:
		tr := traceResult{Direction: "return_third_party"}
		tr.Error = fmt.Sprintf("unsupported third-party provider: %s", provider)
		return tr, ""
	}
}

func runGlobalpingReverseTrace(target, localIP string, cfg config) (traceResult, string) {
	tr := traceResult{Direction: "return_third_party"}

	location := strings.TrimSpace(cfg.ThirdPartyLocation)
	probeHint := "auto"
	if location != "" {
		probeHint = location
	} else if strings.TrimSpace(target) != "" {
		probeHint = "auto-near-target:" + strings.TrimSpace(target)
	}

	tr.Command = fmt.Sprintf("globalping traceroute target=%s from=%s limit=%d", localIP, probeHint, cfg.ThirdPartyProbeLimit)

	measurementID, createRaw, createErr := globalpingCreateTracerouteMeasurement(localIP, location, cfg)
	if createErr != nil {
		tr.Error = fmt.Sprintf("third-party return failed: %v", createErr)
		return tr, createRaw
	}

	resultMap, fetchRaw, fetchErr := globalpingWaitMeasurement(measurementID, cfg)
	rawCombined := strings.TrimSpace(createRaw + "\n\n# globalping result\n" + fetchRaw)
	if fetchErr != nil {
		tr.Error = fmt.Sprintf("third-party return failed: %v", fetchErr)
		return tr, rawCombined
	}

	hops, rawOutput, parseMsg := parseGlobalpingResult(resultMap)
	tr.Hops = hops
	tr.DestinationRTTMs = inferDestinationRTT(hops)

	if parseMsg != "" {
		tr.Error = joinErrors(tr.Error, parseMsg)
	}
	if len(tr.Hops) == 0 && tr.Error == "" {
		tr.Error = "third-party return returned no hop data"
	}
	if strings.TrimSpace(rawOutput) != "" {
		rawCombined = strings.TrimSpace(rawCombined + "\n\n# parsed raw_output\n" + rawOutput)
	}

	return tr, rawCombined
}

func globalpingCreateTracerouteMeasurement(localIP, location string, cfg config) (string, string, error) {
	if strings.TrimSpace(localIP) == "" {
		return "", "", fmt.Errorf("local-ip is empty")
	}

	limit := cfg.ThirdPartyProbeLimit
	if limit <= 0 {
		limit = 1
	}
	if limit > 5 {
		limit = 5
	}

	payload := map[string]interface{}{
		"target": localIP,
		"type":   "traceroute",
		"limit":  limit,
	}
	if strings.TrimSpace(location) != "" {
		payload["locations"] = []map[string]interface{}{
			{
				"magic": strings.TrimSpace(location),
			},
		}
	}

	url := "https://api.globalping.io/v1/measurements"
	statusCode, body, err := globalpingRequest(http.MethodPost, url, cfg.ThirdPartyToken, payload, cfg.ThirdPartyTimeoutSec)
	bodyText := strings.TrimSpace(string(body))
	if err != nil {
		if bodyText != "" {
			return "", bodyText, fmt.Errorf("create measurement failed: %v: %s", err, bodyText)
		}
		return "", bodyText, fmt.Errorf("create measurement failed: %v", err)
	}
	if statusCode < 200 || statusCode >= 300 {
		if bodyText != "" {
			return "", bodyText, fmt.Errorf("create measurement failed: status %d: %s", statusCode, bodyText)
		}
		return "", bodyText, fmt.Errorf("create measurement failed: status %d", statusCode)
	}

	var resp map[string]interface{}
	if uerr := json.Unmarshal(body, &resp); uerr != nil {
		if bodyText != "" {
			return "", bodyText, fmt.Errorf("create measurement failed: invalid json: %v: %s", uerr, bodyText)
		}
		return "", bodyText, fmt.Errorf("create measurement failed: invalid json: %v", uerr)
	}

	measurementID := stringifyAny(resp["id"])
	if measurementID == "" {
		if bodyText != "" {
			return "", bodyText, fmt.Errorf("create measurement failed: missing measurement id: %s", bodyText)
		}
		return "", bodyText, fmt.Errorf("create measurement failed: missing measurement id")
	}

	return measurementID, bodyText, nil
}
func globalpingWaitMeasurement(measurementID string, cfg config) (map[string]interface{}, string, error) {
	if strings.TrimSpace(measurementID) == "" {
		return nil, "", fmt.Errorf("empty measurement id")
	}

	timeoutSec := cfg.ThirdPartyTimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 90
	}
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)

	var lastMap map[string]interface{}
	var lastBody string
	var lastErr error

	for time.Now().Before(deadline) {
		result, bodyText, err := globalpingFetchMeasurement(measurementID, cfg)
		if err != nil {
			lastErr = err
			lastBody = bodyText
			time.Sleep(2 * time.Second)
			continue
		}

		lastMap = result
		lastBody = bodyText

		status := strings.ToLower(strings.TrimSpace(stringifyAny(result["status"])))
		if status == "" {
			if results := interfaceSlice(result["results"]); len(results) > 0 {
				return result, bodyText, nil
			}
			time.Sleep(2 * time.Second)
			continue
		}

		switch status {
		case "in-progress", "in_progress", "pending", "running", "created":
			time.Sleep(2 * time.Second)
			continue
		case "finished", "done", "completed", "success", "succeeded":
			return result, bodyText, nil
		case "failed", "error", "timeout", "timedout", "timed_out":
			return result, bodyText, fmt.Errorf("third-party measurement status: %s", status)
		default:
			if results := interfaceSlice(result["results"]); len(results) > 0 {
				return result, bodyText, nil
			}
			time.Sleep(2 * time.Second)
			continue
		}
	}

	if lastErr != nil {
		return lastMap, lastBody, fmt.Errorf("wait measurement timeout: %v", lastErr)
	}
	return lastMap, lastBody, fmt.Errorf("wait measurement timeout")
}

func globalpingFetchMeasurement(measurementID string, cfg config) (map[string]interface{}, string, error) {
	url := fmt.Sprintf("https://api.globalping.io/v1/measurements/%s", measurementID)

	statusCode, body, err := globalpingRequest(http.MethodGet, url, cfg.ThirdPartyToken, nil, cfg.ThirdPartyTimeoutSec)
	bodyText := strings.TrimSpace(string(body))
	if err != nil {
		if bodyText != "" {
			return nil, bodyText, fmt.Errorf("fetch measurement failed: %v: %s", err, bodyText)
		}
		return nil, bodyText, fmt.Errorf("fetch measurement failed: %v", err)
	}
	if statusCode < 200 || statusCode >= 300 {
		if bodyText != "" {
			return nil, bodyText, fmt.Errorf("fetch measurement failed: status %d: %s", statusCode, bodyText)
		}
		return nil, bodyText, fmt.Errorf("fetch measurement failed: status %d", statusCode)
	}

	var resp map[string]interface{}
	if uerr := json.Unmarshal(body, &resp); uerr != nil {
		if bodyText != "" {
			return nil, bodyText, fmt.Errorf("fetch measurement failed: invalid json: %v: %s", uerr, bodyText)
		}
		return nil, bodyText, fmt.Errorf("fetch measurement failed: invalid json: %v", uerr)
	}
	return resp, bodyText, nil
}
func globalpingRequest(method, url, token string, payload interface{}, timeoutSec int) (int, []byte, error) {
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	if timeoutSec > 60 {
		timeoutSec = 60
	}

	var bodyReader io.Reader
	if payload != nil {
		jsonBytes, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
		bodyReader = bytes.NewReader(jsonBytes)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
		req.Header.Set("x-api-key", strings.TrimSpace(token))
	}

	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return resp.StatusCode, nil, readErr
	}
	return resp.StatusCode, body, nil
}

func parseGlobalpingResult(result map[string]interface{}) ([]hopResult, string, string) {
	if result == nil {
		return nil, "", "empty third-party response"
	}

	var messages []string
	if msg := strings.TrimSpace(stringifyAny(result["message"])); msg != "" {
		messages = append(messages, msg)
	}
	if msg := strings.TrimSpace(stringifyAny(result["error"])); msg != "" {
		messages = append(messages, msg)
	}

	results := interfaceSlice(result["results"])
	if len(results) == 0 {
		return nil, "", joinMessage(messages)
	}

	bestHops := make([]hopResult, 0)
	bestRaw := ""

	for _, item := range results {
		itemMap := interfaceMap(item)
		if itemMap == nil {
			continue
		}

		if em := strings.TrimSpace(stringifyAny(itemMap["error"])); em != "" {
			messages = append(messages, em)
		}

		resObj := interfaceMap(itemMap["result"])
		if resObj == nil {
			continue
		}

		rawOutput := strings.TrimSpace(stringifyAny(resObj["rawOutput"]))
		if rawOutput == "" {
			rawOutput = strings.TrimSpace(stringifyAny(resObj["raw_output"]))
		}

		hops := parseGlobalpingHops(interfaceSlice(resObj["hops"]))
		if len(hops) == 0 && rawOutput != "" {
			hops = parseTracerouteOutput(rawOutput)
		}

		if len(hops) > len(bestHops) {
			bestHops = hops
			bestRaw = rawOutput
		}
	}

	msg := joinMessage(messages)
	return bestHops, bestRaw, msg
}

func parseGlobalpingHops(items []interface{}) []hopResult {
	if len(items) == 0 {
		return nil
	}

	hops := make([]hopResult, 0, len(items))
	for i, item := range items {
		hm := interfaceMap(item)
		if hm == nil {
			continue
		}

		ttl := intFromAny(hm["hop"])
		if ttl <= 0 {
			ttl = intFromAny(hm["ttl"])
		}
		if ttl <= 0 {
			ttl = i + 1
		}

		host := firstNonEmpty(
			stringifyAny(hm["resolvedHostname"]),
			stringifyAny(hm["hostname"]),
			stringifyAny(hm["host"]),
			stringifyAny(hm["name"]),
		)
		ip := firstNonEmpty(
			stringifyAny(hm["resolvedAddress"]),
			stringifyAny(hm["address"]),
			stringifyAny(hm["ip"]),
		)

		rttSamples := parseGlobalpingRTTSamples(interfaceSlice(hm["timings"]))
		if len(rttSamples) == 0 {
			rttSamples = parseGlobalpingRTTSamples(interfaceSlice(hm["rtts"]))
		}

		rtt := 0.0
		if len(rttSamples) > 0 {
			rtt = round2(avg(rttSamples))
		} else {
			rtt = round2(floatFromAny(hm["avg"]))
		}

		rawJSON, _ := json.Marshal(hm)
		raw := strings.TrimSpace(string(rawJSON))

		timeout := host == "" && ip == "" && len(rttSamples) == 0 && rtt <= 0
		if timeout {
			host = "*"
		}

		hop := hopResult{
			TTL:      ttl,
			Host:     host,
			IP:       ip,
			LineName: classifyLineName(host, ip),
			RTTMs:    rtt,
			Raw:      raw,
			Timeout:  timeout,
		}
		if len(rttSamples) > 0 {
			hop.RTTSamplesMs = rttSamples
		}
		hops = append(hops, hop)
	}

	sort.SliceStable(hops, func(i, j int) bool {
		return hops[i].TTL < hops[j].TTL
	})
	return hops
}

func interfaceMap(v interface{}) map[string]interface{} {
	m, _ := v.(map[string]interface{})
	return m
}

func interfaceSlice(v interface{}) []interface{} {
	s, _ := v.([]interface{})
	return s
}

func stringifyAny(v interface{}) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case json.Number:
		return t.String()
	default:
		return ""
	}
}

func intFromAny(v interface{}) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case float32:
		return int(t)
	case json.Number:
		i, err := t.Int64()
		if err == nil {
			return int(i)
		}
	}
	return 0
}

func floatFromAny(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		f, err := t.Float64()
		if err == nil {
			return f
		}
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err == nil {
			return f
		}
	}
	return 0
}

func parseGlobalpingRTTSamples(items []interface{}) []float64 {
	if len(items) == 0 {
		return nil
	}
	out := make([]float64, 0, len(items))
	for _, item := range items {
		if m := interfaceMap(item); m != nil {
			for _, key := range []string{"rtt", "time", "latency", "value", "avg"} {
				f := floatFromAny(m[key])
				if f > 0 {
					out = append(out, round2(f))
					break
				}
			}
			continue
		}

		f := floatFromAny(item)
		if f > 0 {
			out = append(out, round2(f))
		}
	}
	return out
}
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func joinMessage(msgs []string) string {
	if len(msgs) == 0 {
		return ""
	}
	uniq := make(map[string]struct{})
	parts := make([]string, 0, len(msgs))
	for _, m := range msgs {
		t := strings.TrimSpace(m)
		if t == "" {
			continue
		}
		if _, ok := uniq[t]; ok {
			continue
		}
		uniq[t] = struct{}{}
		parts = append(parts, t)
	}
	return strings.Join(parts, "; ")
}






