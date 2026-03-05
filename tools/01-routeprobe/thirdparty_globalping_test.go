package main

import "testing"

func TestParseGlobalpingHops(t *testing.T) {
	items := []interface{}{
		map[string]interface{}{
			"hop":              float64(1),
			"resolvedHostname": "core1.example.net",
			"resolvedAddress":  "203.0.113.1",
			"timings": []interface{}{
				map[string]interface{}{"rtt": float64(10.1)},
				map[string]interface{}{"rtt": float64(11.2)},
			},
		},
		map[string]interface{}{
			"hop": float64(2),
		},
	}

	hops := parseGlobalpingHops(items)
	if len(hops) != 2 {
		t.Fatalf("len(hops) = %d, want 2", len(hops))
	}

	if hops[0].TTL != 1 || hops[0].IP != "203.0.113.1" {
		t.Fatalf("unexpected first hop: %+v", hops[0])
	}
	if hops[0].RTTMs <= 0 {
		t.Fatalf("expected positive RTT for first hop, got %+v", hops[0])
	}

	if hops[1].TTL != 2 {
		t.Fatalf("unexpected second hop ttl: %+v", hops[1])
	}
	if !hops[1].Timeout {
		t.Fatalf("second hop should be timeout when hostname/ip/rtt missing: %+v", hops[1])
	}
}
