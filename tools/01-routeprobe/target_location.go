package main

import (
	"context"
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
	targetResolveCache  sync.Map
	builtinTargetLocZH  = map[string]string{
		"223.5.5.5":   "中国大陆",
		"74.82.42.42": "美国西海岸",
		"1.1.1.1":     "全球公共 DNS",
		"8.8.8.8":     "全球公共 DNS",
	}
)

func prefetchTargetLocations(targets []string) {
	if len(targets) == 0 {
		return
	}

	unique := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		unique[t] = struct{}{}
	}

	var wg sync.WaitGroup
	for target := range unique {
		wg.Add(1)
		go func(t string) {
			defer wg.Done()
			_ = resolveTargetLocationZH(t)
		}(target)
	}
	wg.Wait()
}

func formatTargetCNLabel(target string) string {
	loc := strings.TrimSpace(resolveTargetLocationZH(target))
	if loc == "" || loc == "未知地区" {
		return strings.TrimSpace(target)
	}
	return fmt.Sprintf("%s (%s)", loc, strings.TrimSpace(target))
}

func resolveTargetLocationZH(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return "未知地区"
	}

	targetKey := "target:" + strings.ToLower(target)
	if cached, ok := targetLocationCache.Load(targetKey); ok {
		return cached.(string)
	}

	if v, ok := builtinTargetLocZH[strings.ToLower(target)]; ok {
		targetLocationCache.Store(targetKey, v)
		return v
	}

	ip := target
	if net.ParseIP(ip) == nil {
		resolved := resolveHostToIP(target)
		if resolved == "" {
			targetLocationCache.Store(targetKey, "未知地区")
			return "未知地区"
		}
		ip = resolved
	}

	ipKey := "ip:" + strings.ToLower(ip)
	if v, ok := builtinTargetLocZH[strings.ToLower(ip)]; ok {
		targetLocationCache.Store(targetKey, v)
		targetLocationCache.Store(ipKey, v)
		return v
	}

	if parsed := net.ParseIP(ip); parsed != nil {
		if parsed.IsPrivate() || parsed.IsLoopback() || parsed.IsLinkLocalUnicast() {
			targetLocationCache.Store(targetKey, "本地私有网络")
			targetLocationCache.Store(ipKey, "本地私有网络")
			return "本地私有网络"
		}
	}

	if cached, ok := targetLocationCache.Load(ipKey); ok {
		loc := cached.(string)
		targetLocationCache.Store(targetKey, loc)
		return loc
	}

	loc := lookupLocationFromAPI(ip)
	if loc == "" {
		loc = roughCountryByPrefix(ip)
	}
	if loc == "" {
		loc = "未知地区"
	}

	targetLocationCache.Store(targetKey, loc)
	targetLocationCache.Store(ipKey, loc)
	return loc
}

func resolveHostToIP(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}

	cacheKey := strings.ToLower(host)
	if cached, ok := targetResolveCache.Load(cacheKey); ok {
		return cached.(string)
	}

	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		targetResolveCache.Store(cacheKey, "")
		return ""
	}

	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			targetResolveCache.Store(cacheKey, v4.String())
			return v4.String()
		}
	}

	targetResolveCache.Store(cacheKey, ips[0].String())
	return ips[0].String()
}

func lookupLocationFromAPI(ip string) string {
	if ip == "" {
		return ""
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	results := make(chan string, 2)
	go func() {
		results <- lookupByIPWhoIS(ctx, ip)
	}()
	go func() {
		results <- lookupByIPAPI(ctx, ip)
	}()

	for i := 0; i < 2; i++ {
		loc := strings.TrimSpace(<-results)
		if loc == "" {
			continue
		}
		cancel()
		return loc
	}

	return ""
}

func lookupByIPWhoIS(ctx context.Context, ip string) string {
	url := fmt.Sprintf("https://ipwho.is/%s?lang=zh", strings.TrimSpace(ip))
	body, err := fetchText(ctx, url, 3*time.Second)
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

func lookupByIPAPI(ctx context.Context, ip string) string {
	url := fmt.Sprintf("https://ipapi.co/%s/json/", strings.TrimSpace(ip))
	body, err := fetchText(ctx, url, 3*time.Second)
	if err != nil {
		return ""
	}

	var resp struct {
		Error       bool   `json:"error"`
		CountryName string `json:"country_name"`
		Region      string `json:"region"`
		City        string `json:"city"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return ""
	}
	if resp.Error {
		return ""
	}

	country := normalizeCountryZH(resp.CountryName)
	if country == "" {
		return ""
	}
	return mergeLocation(country, resp.Region, resp.City)
}

func fetchText(ctx context.Context, url string, timeout time.Duration) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "routeprobe/1.0")

	resp, err := sharedHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("http status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
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

	switch strings.ToLower(country) {
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
	default:
		return country
	}
}

func roughCountryByPrefix(ip string) string {
	ip = strings.TrimSpace(ip)
	if strings.HasPrefix(ip, "223.") {
		return "中国"
	}
	if strings.HasPrefix(ip, "74.") {
		return "美国"
	}
	return ""
}
