package log

import (
	"testing"
	"time"
)

func TestMarshalUnmarshal(t *testing.T) {
	entry := &LogEntry{
		Timestamp: time.Date(2026, 3, 4, 9, 15, 30, 0, time.UTC),
		Service:   "api",
		Worktree:  "main",
		Stream:    "stdout",
		Level:     "info",
		Message:   "server started on :8080",
	}

	data, err := entry.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	got, err := UnmarshalEntry(data)
	if err != nil {
		t.Fatalf("UnmarshalEntry failed: %v", err)
	}

	if got.Service != entry.Service {
		t.Errorf("Service: got %q, want %q", got.Service, entry.Service)
	}
	if got.Message != entry.Message {
		t.Errorf("Message: got %q, want %q", got.Message, entry.Message)
	}
	if got.Stream != entry.Stream {
		t.Errorf("Stream: got %q, want %q", got.Stream, entry.Stream)
	}
	if got.Level != entry.Level {
		t.Errorf("Level: got %q, want %q", got.Level, entry.Level)
	}
}

func TestDetectLevel(t *testing.T) {
	tests := []struct {
		line   string
		stream string
		want   string
	}{
		{"server started", "stdout", "info"},
		{"ERROR: connection refused", "stdout", "error"},
		{"[error] something failed", "stdout", "error"},
		{"WARN: disk almost full", "stdout", "warn"},
		{"WARNING: deprecated API", "stdout", "warn"},
		{"DEBUG: request payload", "stdout", "debug"},
		{"anything on stderr", "stderr", "error"},
		{"ERR something broke", "stdout", "error"},
	}

	for _, tt := range tests {
		got := DetectLevel(tt.line, tt.stream)
		if got != tt.want {
			t.Errorf("DetectLevel(%q, %q) = %q, want %q", tt.line, tt.stream, got, tt.want)
		}
	}
}
