package main

import (
	"strings"
	"sync"
)

var (
	lineNameLookupCache     sync.Map
	routeKeywordLookupCache sync.Map
	routeIPLookupCache      sync.Map
	routeTextLookupCache    sync.Map
)

func loadCachedString(cache *sync.Map, key string) (string, bool) {
	if value, ok := cache.Load(key); ok {
		return value.(string), true
	}
	return "", false
}

func storeCachedString(cache *sync.Map, key, value string) string {
	cache.Store(key, value)
	return value
}

func joinCacheKey(parts ...string) string {
	trimmed := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed = append(trimmed, strings.TrimSpace(part))
	}
	return strings.Join(trimmed, "\x00")
}
