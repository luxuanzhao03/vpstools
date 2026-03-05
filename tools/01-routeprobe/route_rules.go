package main

import "strings"

type routeKeywordRule struct {
	Name     string
	Keywords []string
}

var routeDetectRules = []routeKeywordRule{
	{Name: "联通9929", Keywords: []string{"9929", "as9929", "cu9929", "chinaunicomglobal", "cuii", "218.105.", "210.13."}},
	{Name: "电信CN2-GIA", Keywords: []string{"cn2-gia", "cn2gia", "ctg-cnhkg", "ctg-cngz"}},
	{Name: "电信CN2", Keywords: []string{"cn2", "ctgnet", "59.43."}},
	{Name: "电信163", Keywords: []string{"chinanet", "163.com", "as4134", "202.97.", "61.152.", "61.153."}},
	{Name: "联通169", Keywords: []string{"as4837", "cu169", "chinaunicom", "219.158.", "unicom"}},
	{Name: "移动CMI", Keywords: []string{"cmi", "cmcc", "chinamobile", "as9808", "221.183.", "221.176.", "223.120."}},
	{Name: "教育网CERNET", Keywords: []string{"cernet"}},
	{Name: "Hurricane Electric", Keywords: []string{"he.net", "hurricane"}},
	{Name: "Cogent", Keywords: []string{"cogent"}},
	{Name: "Telia", Keywords: []string{"telia"}},
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
	"移动CMI":      {},
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
}

func detectRouteByKeywords(pathText string) string {
	text := strings.ToLower(strings.TrimSpace(pathText))
	if text == "" {
		return ""
	}

	for _, rule := range routeDetectRules {
		if containsAnyLower(text, rule.Keywords) {
			return rule.Name
		}
	}
	return ""
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

