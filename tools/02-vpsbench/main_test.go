package main

import "testing"

func TestParseByteSize(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{input: "512", want: 512},
		{input: "1KiB", want: 1024},
		{input: "64MiB", want: 64 << 20},
		{input: "1.5GiB", want: 1610612736},
		{input: "2GB", want: 2000000000},
	}

	for _, tc := range tests {
		got, err := parseByteSize(tc.input)
		if err != nil {
			t.Fatalf("parseByteSize(%q) returned error: %v", tc.input, err)
		}
		if got != tc.want {
			t.Fatalf("parseByteSize(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestDefaultMemoryBufferBytes(t *testing.T) {
	if got := defaultMemoryBufferBytes(0); got != 64<<20 {
		t.Fatalf("defaultMemoryBufferBytes(0) = %d, want %d", got, 64<<20)
	}
	if got := defaultMemoryBufferBytes(512 << 20); got != 32<<20 {
		t.Fatalf("defaultMemoryBufferBytes(512MiB) = %d, want %d", got, 32<<20)
	}
	if got := defaultMemoryBufferBytes(8 << 30); got != 128<<20 {
		t.Fatalf("defaultMemoryBufferBytes(8GiB) = %d, want %d", got, 128<<20)
	}
}

func TestDefaultDiskFileBytes(t *testing.T) {
	if got := defaultDiskFileBytes(0); got != 128<<20 {
		t.Fatalf("defaultDiskFileBytes(0) = %d, want %d", got, 128<<20)
	}
	if got := defaultDiskFileBytes(512 << 20); got != 64<<20 {
		t.Fatalf("defaultDiskFileBytes(512MiB) = %d, want %d", got, 64<<20)
	}
	if got := defaultDiskFileBytes(8 << 30); got != 256<<20 {
		t.Fatalf("defaultDiskFileBytes(8GiB) = %d, want %d", got, 256<<20)
	}
}
