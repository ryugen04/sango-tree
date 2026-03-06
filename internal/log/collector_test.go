package log

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCollectorWritesJSONL(t *testing.T) {
	tmpDir := t.TempDir()
	sangoDir := filepath.Join(tmpDir, ".sango")

	c, err := NewCollector(sangoDir, "main", "api")
	if err != nil {
		t.Fatalf("NewCollector failed: %v", err)
	}

	stdoutW, err := c.StdoutWriter()
	if err != nil {
		t.Fatalf("StdoutWriter failed: %v", err)
	}
	stderrW, err := c.StderrWriter()
	if err != nil {
		t.Fatalf("StderrWriter failed: %v", err)
	}

	stdoutW.Write([]byte("server started on :8080\n"))
	stderrW.Write([]byte("connection refused\n"))

	// goroutineの処理を待つ
	time.Sleep(100 * time.Millisecond)
	c.Close()

	logPath := filepath.Join(sangoDir, "logs", "main", "api.jsonl")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	content := string(data)
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 log lines, got %d: %s", len(lines), content)
	}

	// 順序に依存せず、エントリをマップで検証
	entries := make(map[string]*LogEntry)
	for _, line := range lines {
		entry, err := UnmarshalEntry([]byte(line))
		if err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		entries[entry.Stream] = entry
	}

	// stdout行
	stdoutEntry, ok := entries["stdout"]
	if !ok {
		t.Fatal("stdout entry not found")
	}
	if stdoutEntry.Level != "info" {
		t.Errorf("stdout level: got %q, want info", stdoutEntry.Level)
	}
	if stdoutEntry.Message != "server started on :8080" {
		t.Errorf("stdout message: got %q", stdoutEntry.Message)
	}

	// stderr行
	stderrEntry, ok := entries["stderr"]
	if !ok {
		t.Fatal("stderr entry not found")
	}
	if stderrEntry.Level != "error" {
		t.Errorf("stderr level: got %q, want error", stderrEntry.Level)
	}
	if stderrEntry.Message != "connection refused" {
		t.Errorf("stderr message: got %q", stderrEntry.Message)
	}
}

func TestCollectorServiceAndWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	sangoDir := filepath.Join(tmpDir, ".sango")

	c, err := NewCollector(sangoDir, "feature___auth", "bff")
	if err != nil {
		t.Fatalf("NewCollector failed: %v", err)
	}

	stdoutW, err := c.StdoutWriter()
	if err != nil {
		t.Fatalf("StdoutWriter failed: %v", err)
	}
	stdoutW.Write([]byte("listening on :3001\n"))
	time.Sleep(100 * time.Millisecond)
	c.Close()

	logPath := filepath.Join(sangoDir, "logs", "feature___auth", "bff.jsonl")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	entry, err := UnmarshalEntry([]byte(strings.TrimSpace(string(data))))
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if entry.Service != "bff" {
		t.Errorf("service: got %q, want bff", entry.Service)
	}
	if entry.Worktree != "feature___auth" {
		t.Errorf("worktree: got %q, want feature___auth", entry.Worktree)
	}
}
