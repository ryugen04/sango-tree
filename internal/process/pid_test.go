package process

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindPIDOwner_Found(t *testing.T) {
	dir := t.TempDir()

	// worktree "feature-a" の "app-bff" に PID 12345 を書き込む
	if err := WritePID(dir, "feature-a", "app-bff", 12345); err != nil {
		t.Fatalf("WritePID failed: %v", err)
	}
	// worktree "feature-b" の "general-api" に PID 67890 を書き込む
	if err := WritePID(dir, "feature-b", "general-api", 67890); err != nil {
		t.Fatalf("WritePID failed: %v", err)
	}

	wt, svc, found := FindPIDOwner(dir, 12345)
	if !found {
		t.Fatal("PID 12345 が見つからなかった")
	}
	if wt != "feature-a" {
		t.Errorf("worktree: got %q, want %q", wt, "feature-a")
	}
	if svc != "app-bff" {
		t.Errorf("service: got %q, want %q", svc, "app-bff")
	}

	wt, svc, found = FindPIDOwner(dir, 67890)
	if !found {
		t.Fatal("PID 67890 が見つからなかった")
	}
	if wt != "feature-b" || svc != "general-api" {
		t.Errorf("got %s/%s, want feature-b/general-api", wt, svc)
	}
}

func TestFindPIDOwner_NotFound(t *testing.T) {
	dir := t.TempDir()

	if err := WritePID(dir, "feature-a", "app-bff", 12345); err != nil {
		t.Fatalf("WritePID failed: %v", err)
	}

	_, _, found := FindPIDOwner(dir, 99999)
	if found {
		t.Error("存在しないPIDが見つかったと報告された")
	}
}

func TestFindPIDOwner_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	_, _, found := FindPIDOwner(dir, 12345)
	if found {
		t.Error("空ディレクトリでPIDが見つかったと報告された")
	}
}

func TestFindPIDOwner_SkipsNonPidFiles(t *testing.T) {
	dir := t.TempDir()

	// .pidでないファイルを作成
	wtDir := filepath.Join(dir, "pids", "feature-a")
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtDir, "state.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 正規のPIDファイルも作成
	if err := WritePID(dir, "feature-a", "app-web", 11111); err != nil {
		t.Fatal(err)
	}

	wt, svc, found := FindPIDOwner(dir, 11111)
	if !found {
		t.Fatal("PID 11111 が見つからなかった")
	}
	if wt != "feature-a" || svc != "app-web" {
		t.Errorf("got %s/%s, want feature-a/app-web", wt, svc)
	}
}
