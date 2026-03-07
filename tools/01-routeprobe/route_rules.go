package main

import "strings"

type routeKeywordRule struct {
	Name     string
	Keywords []string
}

var routeDetectRules = []routeKeywordRule{
	{Name: "联通9929", Keywords: []string{"9929", "as9929", "cu9929", "chinaunicomglobal", "cuii", "218.105.", "210.13."}},
	{Name: "联通CUG", Keywords: []string{"as10099", "10099", "cug", "china unicom global"}},
	{Name: "电信CN2-GIA", Keywords: []string{"cn2-gia", "cn2gia", "ctg-cnhkg", "ctg-cngz", "gia"}},
	{Name: "电信CN2", Keywords: []string{"cn2", "ctgnet", "59.43."}},
	{Name: "电信163", Keywords: []string{"chinanet", "163.com", "as4134", "202.97.", "61.152.", "61.153."}},
	{Name: "联通169", Keywords: []string{"as4837", "cu169", "chinaunicom", "219.158.", "unicom"}},
	{Name: "移动CMI", Keywords: []string{"as58453", "chinamobileinternational", "cmi", "223.120."}},
	{Name: "移动CMNET", Keywords: []string{"as9808", "as56040", "cmnet", "cmcc", "chinamobile", "221.183.", "221.176.", "120.198."}},
	{Name: "教育网CERNET", Keywords: []string{"cernet"}},
	{Name: "Hurricane Electric", Keywords: []string{"he.net", "hurricane"}},
	{Name: "Cogent", Keywords: []string{"cogent"}},
	{Name: "Telia", Keywords: []string{"telia", "as1299"}},
	{Name: "PCCW", Keywords: []string{"pccw", "as3491", "pccwglobal"}},
	{Name: "Telstra", Keywords: []string{"telstra", "as1221"}},
	{Name: "Lumen/Level3", Keywords: []string{"lumen", "level3", "centurylink"}},
	{Name: "GTT", Keywords: []string{"gtt"}},
	{Name: "Zayo", Keywords: []string{"zayo"}},
	{Name: "NTT", Keywords: []string{"ntt"}},
}

var chinaCarrierRouteSet = map[string]struct{}{
	"电信163":      {},
	"电信CN2":      {},
	"电信CN2-GIA":  {},
	"联通169":      {},
	"联通9929":     {},
	"联通CUG":      {},
	"移动CMI":      {},
	"移动CMNET":    {},
	"教育网CERNET": {},
}

var foreignBackboneRouteSet = map[string]struct{}{
	"Hurricane Electric": {},
	"Cogent":             {},
	"Telia":              {},
	"NTT":                {},
	"Lumen/Level3":       {},
	"GTT":                {},
	"Zayo":               {},
	"PCCW":               {},
	"Telstra":            {},
}

func detectRouteByKeywords(pathText string) string {
	text := strings.ToLower(strings.TrimSpace(pathText))
	if text == "" {
		return ""
	}
	if cached, ok := loadCachedString(&routeKeywordLookupCache, text); ok {
		return cached
	}

	for _, rule := range routeDetectRules {
		if containsAnyLower(text, rule.Keywords) {
			return storeCachedString(&routeKeywordLookupCache, text, rule.Name)
		}
	}
	return storeCachedString(&routeKeywordLookupCache, text, "")
}

func isChinaCarrierRoute(route string) bool {
	_, ok := chinaCarrierRouteSet[strings.TrimSpace(route)]
	return ok
}

func isForeignBackboneRoute(route string) bool {
	_, ok := foreignBackboneRouteSet[strings.TrimSpace(route)]
	return ok
}

func containsAnyLower(text string, keys []string) bool {
	for _, key := range keys {
		if strings.Contains(text, key) {
			return true
		}
	}
	return false
}
