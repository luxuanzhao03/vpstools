package main

import "testing"

func TestDetectRouteByKeywords(t *testing.T) {
	cases := []struct {
		text string
		want string
	}{
		{text: "be14.core1.lax2.he.net", want: "Hurricane Electric"},
		{text: "AS3491 pccwglobal transit", want: "PCCW"},
		{text: "as1221 telstra international", want: "Telstra"},
		{text: "AS10099 china unicom global", want: "联通CUG"},
	}

	for _, tc := range cases {
		got := detectRouteByKeywords(tc.text)
		if got != tc.want {
			t.Fatalf("detectRouteByKeywords(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
}

func TestLookupRouteByTextDatabaseASN(t *testing.T) {
	if got := lookupRouteByTextDatabase("transit AS3491"); got != "PCCW" {
		t.Fatalf("lookupRouteByTextDatabase(AS3491) = %q, want %q", got, "PCCW")
	}
	if got := lookupRouteByTextDatabase("network as10099"); got != "联通CUG" {
		t.Fatalf("lookupRouteByTextDatabase(AS10099) = %q, want %q", got, "联通CUG")
	}
}

func TestLookupRouteByIPDatabase(t *testing.T) {
	if got := lookupRouteByIPDatabase("59.43.200.1"); got != "电信CN2-GIA" {
		t.Fatalf("lookupRouteByIPDatabase(59.43.200.1) = %q, want %q", got, "电信CN2-GIA")
	}
	if got := lookupRouteByIPDatabase("63.218.1.1"); got != "PCCW" {
		t.Fatalf("lookupRouteByIPDatabase(63.218.1.1) = %q, want %q", got, "PCCW")
	}
}
