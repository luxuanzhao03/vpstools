package main

import (
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
	routeDBOnce     sync.Once
	routePrefixDB   []routePrefixMatcher
	routeASNDB      map[int]string
	asnTokenRe      = regexp.MustCompile(`(?i)\bas\s*([0-9]{2,10})\b`)
	builtinRouteDB  = routeDBFile{
		Prefixes: []routePrefixRule{
			{CIDR: "59.43.0.0/16", Route: "电信CN2"},
			{CIDR: "202.97.0.0/16", Route: "电信163"},
			{CIDR: "61.152.0.0/16", Route: "电信163"},
			{CIDR: "61.153.0.0/16", Route: "电信163"},
			{CIDR: "219.158.0.0/16", Route: "联通169"},
			{CIDR: "218.105.0.0/16", Route: "联通9929"},
			{CIDR: "221.183.0.0/16", Route: "移动CMI"},
			{CIDR: "221.176.0.0/16", Route: "移动CMI"},
			{CIDR: "223.120.0.0/16", Route: "移动CMI"},
			{CIDR: "216.218.0.0/16", Route: "Hurricane Electric"},
			{CIDR: "184.105.0.0/16", Route: "Hurricane Electric"},
			{CIDR: "154.54.0.0/16", Route: "Cogent"},
			{CIDR: "62.115.0.0/16", Route: "Telia"},
			{CIDR: "129.250.0.0/16", Route: "NTT"},
		},
		ASN: map[string]string{
			"4134":  "电信163",
			"4809":  "电信CN2",
			"4837":  "联通169",
			"9929":  "联通9929",
			"9808":  "移动CMI",
			"58453": "移动CMI",
			"6939":  "Hurricane Electric",
			"174":   "Cogent",
			"1299":  "Telia",
			"2914":  "NTT",
			"3356":  "Lumen/Level3",
			"3257":  "GTT",
			"6461":  "Zayo",
		},
	}
)

func initRouteDatabase() {
	routePrefixDB = make([]routePrefixMatcher, 0, len(builtinRouteDB.Prefixes)+32)
	routeASNDB = make(map[int]string, len(builtinRouteDB.ASN)+32)

	mergeRouteDB(builtinRouteDB)

	if external, ok := loadExternalRouteDB(); ok {
		mergeRouteDB(external)
	}

	sort.SliceStable(routePrefixDB, func(i, j int) bool {
		onesI, _ := routePrefixDB[i].net.Mask.Size()
		onesJ, _ := routePrefixDB[j].net.Mask.Size()
		return onesI > onesJ
	})
}

func mergeRouteDB(db routeDBFile) {
	for _, p := range db.Prefixes {
		cidr := strings.TrimSpace(p.CIDR)
		route := strings.TrimSpace(p.Route)
		if cidr == "" || route == "" {
			continue
		}
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil || ipNet == nil {
			continue
		}
		routePrefixDB = append(routePrefixDB, routePrefixMatcher{route: route, net: ipNet})
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
	case strings.Contains(n, "移动cmi"):
		return "移动CMI"
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
	default:
		return ""
	}
}
