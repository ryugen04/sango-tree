package log

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRotate(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "api.jsonl")

	// 初期ファイルを作成
	os.WriteFile(basePath, []byte("line1\n"), 0o644)

	// 1回目のローテーション
	Rotate(basePath, 5)

	if _, err := os.Stat(basePath + ".1"); err != nil {
		t.Error("expected api.jsonl.1 to exist")
	}
	if _, err := os.Stat(basePath); err == nil {
		t.Error("expected api.jsonl to be removed after rotation")
	}

	// 新しいファイルを作成してもう一度ローテーション
	os.WriteFile(basePath, []byte("line2\n"), 0o644)
	Rotate(basePath, 5)

	if _, err := os.Stat(basePath + ".2"); err != nil {
		t.Error("expected api.jsonl.2 to exist")
	}
	if _, err := os.Stat(basePath + ".1"); err != nil {
		t.Error("expected api.jsonl.1 to exist")
	}

	// .2の内容は元の.1（line1）
	data, _ := os.ReadFile(basePath + ".2")
	if string(data) != "line1\n" {
		t.Errorf("unexpected content in .2: %q", string(data))
	}
}

func TestRotateMaxFiles(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "api.jsonl")

	// maxFiles=2で3回ローテーション
	for i := 0; i < 3; i++ {
		os.WriteFile(basePath, []byte("line\n"), 0o644)
		Rotate(basePath, 2)
	}

	// .1と.2のみ存在
	if _, err := os.Stat(basePath + ".1"); err != nil {
		t.Error("expected .1 to exist")
	}
	if _, err := os.Stat(basePath + ".2"); err != nil {
		t.Error("expected .2 to exist")
	}
	// .3は存在しない
	if _, err := os.Stat(basePath + ".3"); err == nil {
		t.Error("expected .3 to NOT exist")
	}
}

func TestCompressFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl.1")

	os.WriteFile(path, []byte("test content\n"), 0o644)

	if err := CompressFile(path); err != nil {
		t.Fatalf("CompressFile failed: %v", err)
	}

	// 元ファイルは削除されている
	if _, err := os.Stat(path); err == nil {
		t.Error("expected original file to be removed")
	}

	// .gzファイルが存在する
	if _, err := os.Stat(path + ".gz"); err != nil {
		t.Error("expected .gz file to exist")
	}
}
