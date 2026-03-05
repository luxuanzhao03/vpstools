package main

import (
	_ "embed"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type routePrefixRule struct {
	CIDR  string `json:"cidr"`
	Route string `json:"route"`
}

type routeDBFile struct {
	Prefixes []routePrefixRule `json:"prefixes"`
	ASN      map[string]string `json:"asn"`
}

type routePrefixMatcher struct {
	route string
	net   *net.IPNet
}

var (
	routeDBOnce   sync.Once
	routePrefixDB []routePrefixMatcher
	routeASNDB    map[int]string
	asnTokenRe    = regexp.MustCompile(`(?i)\bas\s*([0-9]{2,10})\b`)

	//go:embed route_db.json
	embeddedRouteDBJSON []byte
)

func initRouteDatabase() {
	routePrefixDB = make([]routePrefixMatcher, 0, 64)
	routeASNDB = make(map[int]string, 64)

	seenPrefix := make(map[string]struct{}, 128)

	if embedded, ok := loadEmbeddedRouteDB(); ok {
		mergeRouteDB(embedded, seenPrefix)
	}

	if external, ok := loadExternalRouteDB(); ok {
		mergeRouteDB(external, seenPrefix)
	}

	sort.SliceStable(routePrefixDB, func(i, j int) bool {
		onesI, _ := routePrefixDB[i].net.Mask.Size()
		onesJ, _ := routePrefixDB[j].net.Mask.Size()
		return onesI > onesJ
	})
}

func mergeRouteDB(db routeDBFile, seenPrefix map[string]struct{}) {
	for _, p := range db.Prefixes {
		cidr := strings.TrimSpace(p.CIDR)
		route := strings.TrimSpace(p.Route)
		if cidr == "" || route == "" {
			continue
		}
		key := cidr + "|" + route
		if _, ok := seenPrefix[key]; ok {
			continue
		}

		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil || ipNet == nil {
			continue
		}
		routePrefixDB = append(routePrefixDB, routePrefixMatcher{route: route, net: ipNet})
		seenPrefix[key] = struct{}{}
	}

	for k, v := range db.ASN {
		asn, err := strconv.Atoi(strings.TrimSpace(k))
		if err != nil || asn <= 0 {
			continue
		}
		route := strings.TrimSpace(v)
		if route == "" {
			continue
		}
		routeASNDB[asn] = route
	}
}

func loadEmbeddedRouteDB() (routeDBFile, bool) {
	if len(embeddedRouteDBJSON) == 0 {
		return routeDBFile{}, false
	}
	var db routeDBFile
	if err := json.Unmarshal(embeddedRouteDBJSON, &db); err != nil {
		return routeDBFile{}, false
	}
	return db, true
}

func loadExternalRouteDB() (routeDBFile, bool) {
	paths := []string{
		"route_db.json",
		filepath.Join(filepath.Dir(os.Args[0]), "route_db.json"),
	}

	for _, p := range paths {
		content, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var db routeDBFile
		if err := json.Unmarshal(content, &db); err != nil {
			continue
		}
		return db, true
	}

	return routeDBFile{}, false
}

func lookupRouteByIPDatabase(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return ""
	}

	routeDBOnce.Do(initRouteDatabase)

	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ""
	}

	for _, rule := range routePrefixDB {
		if rule.net.Contains(parsed) {
			return rule.route
		}
	}
	return ""
}

func lookupRouteByTextDatabase(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	routeDBOnce.Do(initRouteDatabase)

	for _, m := range asnTokenRe.FindAllStringSubmatch(strings.ToLower(text), -1) {
		if len(m) < 2 {
			continue
		}
		asn, err := strconv.Atoi(strings.TrimSpace(m[1]))
		if err != nil || asn <= 0 {
			continue
		}
		if route := routeASNDB[asn]; route != "" {
			return route
		}
	}
	return ""
}

func normalizeRouteAlias(name string) string {
	n := strings.TrimSpace(strings.ToLower(name))
	if n == "" {
		return ""
	}

	switch {
	case strings.Contains(n, "电信163"):
		return "电信163"
	case strings.Contains(n, "电信cn2-gia"):
		return "电信CN2-GIA"
	case strings.Contains(n, "电信cn2"):
		return "电信CN2"
	case strings.Contains(n, "联通169"):
		return "联通169"
	case strings.Contains(n, "联通9929"):
		return "联通9929"
	case strings.Contains(n, "as10099") || strings.Contains(n, "联通cug"):
		return "联通CUG"
	case strings.Contains(n, "移动cmi") || strings.Contains(n, "china mobile cmi"):
		return "移动CMI"
	case strings.Contains(n, "移动cmnet") || strings.Contains(n, "china mobile cmnet"):
		return "移动CMNET"
	case strings.Contains(n, "cmnet") || strings.Contains(n, "as9808") || strings.Contains(n, "as56040"):
		return "移动CMNET"
	case strings.Contains(n, "hurricane electric"):
		return "Hurricane Electric"
	case strings.Contains(n, "cogent"):
		return "Cogent"
	case strings.Contains(n, "telia"):
		return "Telia"
	case strings.Contains(n, "ntt"):
		return "NTT"
	case strings.Contains(n, "lumen") || strings.Contains(n, "level3"):
		return "Lumen/Level3"
	case strings.Contains(n, "gtt"):
		return "GTT"
	case strings.Contains(n, "zayo"):
		return "Zayo"
	case strings.Contains(n, "pccw") || strings.Contains(n, "as3491"):
		return "PCCW"
	case strings.Contains(n, "telstra") || strings.Contains(n, "as1221"):
		return "Telstra"
	default:
		return ""
	}
}
