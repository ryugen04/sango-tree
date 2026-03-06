package log

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestLogs(t *testing.T, sangoDir string) {
	t.Helper()

	// main/api.jsonl
	apiDir := filepath.Join(sangoDir, "logs", "main")
	os.MkdirAll(apiDir, 0o755)

	now := time.Now()
	entries := []LogEntry{
		{Timestamp: now.Add(-3 * time.Minute), Service: "api", Worktree: "main", Stream: "stdout", Level: "info", Message: "server started"},
		{Timestamp: now.Add(-2 * time.Minute), Service: "api", Worktree: "main", Stream: "stderr", Level: "error", Message: "connection refused"},
		{Timestamp: now.Add(-1 * time.Minute), Service: "api", Worktree: "main", Stream: "stdout", Level: "info", Message: "request handled"},
	}
	writeEntries(t, filepath.Join(apiDir, "api.jsonl"), entries)

	// main/bff.jsonl
	bffEntries := []LogEntry{
		{Timestamp: now.Add(-2*time.Minute - 30*time.Second), Service: "bff", Worktree: "main", Stream: "stdout", Level: "info", Message: "listening on :3001"},
	}
	writeEntries(t, filepath.Join(apiDir, "bff.jsonl"), bffEntries)
}

func writeEntries(t *testing.T, path string, entries []LogEntry) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	for _, e := range entries {
		data, _ := e.Marshal()
		f.Write(append(data, '\n'))
	}
}

func TestReadLogs(t *testing.T) {
	tmpDir := t.TempDir()
	sangoDir := filepath.Join(tmpDir, ".sango")
	setupTestLogs(t, sangoDir)

	// 全ログ読み込み
	entries, err := ReadLogs(sangoDir, Filter{})
	if err != nil {
		t.Fatalf("ReadLogs failed: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}

	// タイムスタンプ順にソートされていること
	for i := 1; i < len(entries); i++ {
		if entries[i].Timestamp.Before(entries[i-1].Timestamp) {
			t.Errorf("entries not sorted: [%d]=%v > [%d]=%v", i-1, entries[i-1].Timestamp, i, entries[i].Timestamp)
		}
	}
}

func TestReadLogsServiceFilter(t *testing.T) {
	tmpDir := t.TempDir()
	sangoDir := filepath.Join(tmpDir, ".sango")
	setupTestLogs(t, sangoDir)

	entries, err := ReadLogs(sangoDir, Filter{Services: []string{"api"}})
	if err != nil {
		t.Fatalf("ReadLogs failed: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
}

func TestReadLogsLevelFilter(t *testing.T) {
	tmpDir := t.TempDir()
	sangoDir := filepath.Join(tmpDir, ".sango")
	setupTestLogs(t, sangoDir)

	entries, err := ReadLogs(sangoDir, Filter{Level: "error"})
	if err != nil {
		t.Fatalf("ReadLogs failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Message != "connection refused" {
		t.Errorf("unexpected message: %q", entries[0].Message)
	}
}

func TestReadLogsGrepFilter(t *testing.T) {
	tmpDir := t.TempDir()
	sangoDir := filepath.Join(tmpDir, ".sango")
	setupTestLogs(t, sangoDir)

	entries, err := ReadLogs(sangoDir, Filter{Grep: "started"})
	if err != nil {
		t.Fatalf("ReadLogs failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func TestReadLogsLimit(t *testing.T) {
	tmpDir := t.TempDir()
	sangoDir := filepath.Join(tmpDir, ".sango")
	setupTestLogs(t, sangoDir)

	entries, err := ReadLogs(sangoDir, Filter{Limit: 2})
	if err != nil {
		t.Fatalf("ReadLogs failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestFollowLogs(t *testing.T) {
	tmpDir := t.TempDir()
	sangoDir := filepath.Join(tmpDir, ".sango")
	setupTestLogs(t, sangoDir)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := FollowLogs(ctx, sangoDir, Filter{})
	if err != nil {
		t.Fatalf("FollowLogs failed: %v", err)
	}

	// 新しいエントリを書き込む
	time.Sleep(300 * time.Millisecond)
	logPath := filepath.Join(sangoDir, "logs", "main", "api.jsonl")
	f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0o644)
	entry := &LogEntry{
		Timestamp: time.Now(),
		Service:   "api",
		Worktree:  "main",
		Stream:    "stdout",
		Level:     "info",
		Message:   "new log entry",
	}
	data, _ := entry.Marshal()
	f.Write(append(data, '\n'))
	f.Close()

	// エントリが届くことを確認
	select {
	case got := <-ch:
		if got.Message != "new log entry" {
			t.Errorf("unexpected message: %q", got.Message)
		}
	case <-time.After(1500 * time.Millisecond):
		t.Error("timeout waiting for log entry")
	}
}
