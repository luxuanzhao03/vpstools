//go:build !linux

package main

func detectPlatformSystemInfo() platformSystemInfo {
	return platformSystemInfo{}
}
