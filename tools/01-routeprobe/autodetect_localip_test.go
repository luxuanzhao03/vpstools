package main

import "testing"

func TestIsLikelyPublicIP(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{ip: "8.8.8.8", want: true},
		{ip: "223.5.5.5", want: true},
		{ip: "1.1.1.1", want: true},
		{ip: "127.0.0.1", want: false},
		{ip: "10.0.0.1", want: false},
		{ip: "172.16.1.1", want: false},
		{ip: "192.168.1.1", want: false},
		{ip: "169.254.1.1", want: false},
		{ip: "100.64.0.1", want: false},
		{ip: "", want: false},
		{ip: "not-an-ip", want: false},
	}

	for _, tc := range cases {
		got := isLikelyPublicIP(tc.ip)
		if got != tc.want {
			t.Fatalf("isLikelyPublicIP(%q) = %v, want %v", tc.ip, got, tc.want)
		}
	}
}
