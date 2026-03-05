package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

func ensureReturnLocalIP(cfg *config) error {
	if cfg == nil {
		return fmt.Errorf("nil config")
	}

	needReturn := len(cfg.ReverseSSH) > 0 || cfg.ThirdPartyReturn
	cfg.LocalIP = strings.TrimSpace(cfg.LocalIP)
	if !needReturn {
		return nil
	}

	if cfg.LocalIP != "" {
		if !validIP(cfg.LocalIP) {
			return fmt.Errorf("invalid -local-ip: %s", cfg.LocalIP)
		}
		return nil
	}

	ip, source, err := detectLocalReachableIP()
	if err != nil {
		return fmt.Errorf("回程测量需要本机可回测IP，自动获取失败: %v；请手动指定 -local-ip", err)
	}

	cfg.LocalIP = ip
	if source != "" {
		fmt.Fprintf(os.Stderr, "[info] local-ip auto-detected: %s (%s)\n", cfg.LocalIP, source)
	} else {
		fmt.Fprintf(os.Stderr, "[info] local-ip auto-detected: %s\n", cfg.LocalIP)
	}
	return nil
}

func detectLocalReachableIP() (string, string, error) {
	errs := make([]string, 0, 3)

	if ip, source, err := detectPublicIPByHTTP(); err == nil && ip != "" {
		return ip, source, nil
	} else if err != nil {
		errs = append(errs, "http探测失败: "+err.Error())
	}

	if ip := detectOutboundRouteIP(); ip != "" {
		if isLikelyPublicIP(ip) {
			return ip, "udp-outbound", nil
		}
		return ip, "udp-outbound-private", nil
	}
	errs = append(errs, "udp路由探测失败")

	if ip := detectInterfaceFallbackIP(); ip != "" {
		if isLikelyPublicIP(ip) {
			return ip, "net-interface", nil
		}
		return ip, "net-interface-private", nil
	}
	errs = append(errs, "网卡探测失败")

	return "", "", fmt.Errorf(strings.Join(errs, "; "))
}

func detectPublicIPByHTTP() (string, string, error) {
	endpoints := []string{
		"https://4.ipw.cn",
		"https://myip.ipip.net",
		"https://api.ipify.org",
		"https://ifconfig.me/ip",
		"https://ifconfig.co/ip",
		"https://ip.sb",
		"http://4.ipw.cn",
		"http://ip.3322.net",
	}

	client := &http.Client{Timeout: 4 * time.Second}
	var lastErr error
	for _, url := range endpoints {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "routeprobe/1.0")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 2048))
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("%s status %d", url, resp.StatusCode)
			continue
		}

		ip := firstIP(string(body))
		if ip == "" {
			lastErr = fmt.Errorf("%s no ip in response", url)
			continue
		}
		if !validIP(ip) {
			lastErr = fmt.Errorf("%s returned invalid ip: %s", url, ip)
			continue
		}
		if !isLikelyPublicIP(ip) {
			lastErr = fmt.Errorf("%s returned non-public ip: %s", url, ip)
			continue
		}
		return ip, "endpoint:" + url, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no endpoint available")
	}
	return "", "", lastErr
}

func detectOutboundRouteIP() string {
	checks := []struct {
		network string
		address string
	}{
		{network: "udp4", address: "223.5.5.5:53"},
		{network: "udp4", address: "8.8.8.8:53"},
		{network: "udp4", address: "1.1.1.1:53"},
		{network: "udp6", address: "[2400:3200::1]:53"},
		{network: "udp6", address: "[2001:4860:4860::8888]:53"},
	}

	bestAny := ""
	for _, item := range checks {
		conn, err := net.DialTimeout(item.network, item.address, 2*time.Second)
		if err != nil {
			continue
		}
		localAddr := conn.LocalAddr()
		_ = conn.Close()

		ip := extractIPFromAddr(localAddr)
		if ip == "" {
			continue
		}
		if isLikelyPublicIP(ip) {
			return ip
		}
		if bestAny == "" {
			bestAny = ip
		}
	}
	return bestAny
}

func detectInterfaceFallbackIP() string {
	ifs, err := net.Interfaces()
	if err != nil {
		return ""
	}

	bestAny := ""
	for _, iface := range ifs {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ip := extractIPFromAddr(addr)
			if ip == "" {
				continue
			}
			if isLikelyPublicIP(ip) {
				return ip
			}
			if bestAny == "" {
				bestAny = ip
			}
		}
	}
	return bestAny
}

func extractIPFromAddr(addr net.Addr) string {
	if addr == nil {
		return ""
	}

	switch v := addr.(type) {
	case *net.UDPAddr:
		if v.IP == nil {
			return ""
		}
		return strings.TrimSpace(v.IP.String())
	case *net.TCPAddr:
		if v.IP == nil {
			return ""
		}
		return strings.TrimSpace(v.IP.String())
	case *net.IPAddr:
		if v.IP == nil {
			return ""
		}
		return strings.TrimSpace(v.IP.String())
	case *net.IPNet:
		if v.IP == nil {
			return ""
		}
		return strings.TrimSpace(v.IP.String())
	default:
		host, _, err := net.SplitHostPort(addr.String())
		if err == nil && validIP(host) {
			return strings.TrimSpace(host)
		}
		if validIP(addr.String()) {
			return strings.TrimSpace(addr.String())
		}
		return ""
	}
}

func isLikelyPublicIP(ipStr string) bool {
	ip := net.ParseIP(strings.TrimSpace(ipStr))
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return false
	}

	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 0 || ip4[0] == 127 || ip4[0] >= 224 {
			return false
		}
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return false
		}
		if ip4[0] == 169 && ip4[1] == 254 {
			return false
		}
		if ip4[0] == 198 && (ip4[1] == 18 || ip4[1] == 19) {
			return false
		}
	}

	return true
}
