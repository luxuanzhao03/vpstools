package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	targetLocationCache sync.Map
	builtinTargetLocZH  = map[string]string{
		"223.5.5.5":  "中国大陆",
		"74.82.42.42": "美国西海岸",
		"1.1.1.1":    "美国",
		"8.8.8.8":    "美国",
	}
)

func formatTargetCNLabel(target string) string {
	loc := strings.TrimSpace(resolveTargetLocationZH(target))
	if loc == "" || loc == "未知地区" {
		return strings.TrimSpace(target)
	}
	return fmt.Sprintf("%s（%s）", loc, strings.TrimSpace(target))
}

func resolveTargetLocationZH(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return "未知地区"
	}

	if v, ok := builtinTargetLocZH[strings.ToLower(target)]; ok {
		return v
	}

	ip := target
	if net.ParseIP(ip) == nil {
		resolved := resolveHostToIP(target)
		if resolved == "" {
			return "未知地区"
		}
		ip = resolved
	}

	if v, ok := builtinTargetLocZH[strings.ToLower(ip)]; ok {
		return v
	}
	if parsed := net.ParseIP(ip); parsed != nil {
		if parsed.IsPrivate() || parsed.IsLoopback() || parsed.IsLinkLocalUnicast() {
			return "本地私有网络"
		}
	}

	if cached, ok := targetLocationCache.Load(ip); ok {
		return cached.(string)
	}

	loc := lookupLocationFromAPI(ip)
	if loc == "" {
		loc = roughCountryByPrefix(ip)
	}
	if loc == "" {
		loc = "未知地区"
	}
	targetLocationCache.Store(ip, loc)
	return loc
}

func resolveHostToIP(host string) string {
	ips, err := net.LookupIP(strings.TrimSpace(host))
	if err != nil || len(ips) == 0 {
		return ""
	}
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			return v4.String()
		}
	}
	return ips[0].String()
}

func lookupLocationFromAPI(ip string) string {
	if ip == "" {
		return ""
	}

	if loc := lookupByIPWhoIS(ip); loc != "" {
		return loc
	}
	if loc := lookupByIPAPI(ip); loc != "" {
		return loc
	}
	return ""
}

func lookupByIPWhoIS(ip string) string {
	url := fmt.Sprintf("https://ipwho.is/%s?lang=zh", strings.TrimSpace(ip))
	body, err := fetchText(url, 3*time.Second)
	if err != nil {
		return ""
	}

	var resp struct {
		Success bool   `json:"success"`
		Country string `json:"country"`
		Region  string `json:"region"`
		City    string `json:"city"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return ""
	}
	if !resp.Success {
		return ""
	}
	country := normalizeCountryZH(resp.Country)
	if country == "" {
		return ""
	}
	return mergeLocation(country, resp.Region, resp.City)
}

func lookupByIPAPI(ip string) string {
	url := fmt.Sprintf("http://ip-api.com/json/%s?lang=zh-CN&fields=status,country,regionName,city", strings.TrimSpace(ip))
	body, err := fetchText(url, 3*time.Second)
	if err != nil {
		return ""
	}

	var resp struct {
		Status     string `json:"status"`
		Country    string `json:"country"`
		RegionName string `json:"regionName"`
		City       string `json:"city"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return ""
	}
	if strings.ToLower(strings.TrimSpace(resp.Status)) != "success" {
		return ""
	}
	country := normalizeCountryZH(resp.Country)
	if country == "" {
		return ""
	}
	return mergeLocation(country, resp.RegionName, resp.City)
}

func fetchText(url string, timeout time.Duration) (string, error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "routeprobe/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("http status %d", resp.StatusCode)
	}

	bytes, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(bytes)), nil
}

func mergeLocation(country, region, city string) string {
	country = strings.TrimSpace(country)
	region = strings.TrimSpace(region)
	city = strings.TrimSpace(city)
	if country == "" {
		return ""
	}

	parts := []string{country}
	if region != "" && !strings.Contains(country, region) {
		parts = append(parts, region)
	}
	if city != "" && !strings.Contains(region, city) {
		parts = append(parts, city)
	}
	return strings.Join(parts, "-")
}

func normalizeCountryZH(country string) string {
	country = strings.TrimSpace(country)
	if country == "" {
		return ""
	}
	if strings.Contains(country, "中国") {
		return "中国"
	}
	if strings.Contains(country, "美国") {
		return "美国"
	}
	lower := strings.ToLower(country)
	switch lower {
	case "china", "people's republic of china", "pr china", "cn":
		return "中国"
	case "united states", "usa", "us", "united states of america":
		return "美国"
	case "hong kong":
		return "中国香港"
	case "japan":
		return "日本"
	case "singapore":
		return "新加坡"
	case "germany":
		return "德国"
	case "united kingdom", "uk":
		return "英国"
	}
	return country
}

func roughCountryByPrefix(ip string) string {
	ip = strings.TrimSpace(ip)
	if strings.HasPrefix(ip, "223.") {
		return "中国"
	}
	if strings.HasPrefix(ip, "74.") || strings.HasPrefix(ip, "8.") || strings.HasPrefix(ip, "1.") {
		return "美国"
	}
	return ""
}
