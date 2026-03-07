package main

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestCountingReaderTracksBytes(t *testing.T) {
	reader := &countingReader{reader: bytes.NewReader([]byte("abcdef"))}
	buf := make([]byte, 4)

	n, err := reader.Read(buf)
	if err != nil {
		t.Fatalf("first read error: %v", err)
	}
	if n != 4 {
		t.Fatalf("first read bytes = %d, want 4", n)
	}

	n, err = reader.Read(buf)
	if err != nil {
		t.Fatalf("second read unexpected error: %v", err)
	}
	if n != 2 {
		t.Fatalf("second read bytes = %d, want 2", n)
	}
	if got := reader.Count(); got != 6 {
		t.Fatalf("reader.Count() = %d, want 6", got)
	}

	n, err = reader.Read(buf)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("third read error = %v, want EOF", err)
	}
	if n != 0 {
		t.Fatalf("third read bytes = %d, want 0", n)
	}
}

func TestDiscardAndCountKeepsPartialBytesOnError(t *testing.T) {
	wantErr := errors.New("boom")
	reader := &stubErrorReader{
		chunks: [][]byte{
			[]byte("abcd"),
			[]byte("ef"),
		},
		err: wantErr,
	}

	gotBytes, err := discardAndCount(reader)
	if !errors.Is(err, wantErr) {
		t.Fatalf("discardAndCount error = %v, want %v", err, wantErr)
	}
	if gotBytes != 6 {
		t.Fatalf("discardAndCount bytes = %d, want 6", gotBytes)
	}
}

type stubErrorReader struct {
	chunks [][]byte
	index  int
	err    error
}

func (r *stubErrorReader) Read(p []byte) (int, error) {
	if r.index >= len(r.chunks) {
		return 0, r.err
	}

	chunk := r.chunks[r.index]
	r.index++
	n := copy(p, chunk)
	if r.index >= len(r.chunks) {
		return n, r.err
	}
	return n, nil
}
