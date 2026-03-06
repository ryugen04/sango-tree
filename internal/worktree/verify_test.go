package worktree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ryugen04/sango-tree/internal/config"
)

// TestVerifyIncludes_CopyOK はコピー済みファイルの検証がOKになることをテストする
func TestVerifyIncludes_CopyOK(t *testing.T) {
	worktreeDir := t.TempDir()

	content := "hello"
	createTestFile(t, filepath.Join(worktreeDir, "src.txt"), content)
	createTestFile(t, filepath.Join(worktreeDir, "api", "dst.txt"), content)

	include := config.IncludeConfig{
		PerService: map[string][]config.IncludeEntry{
			"api": {
				{Source: "src.txt", Target: "dst.txt", Strategy: "copy"},
			},
		},
	}

	results := VerifyIncludes(worktreeDir, []string{"api"}, include, nil)
	if len(results) != 1 {
		t.Fatalf("結果の数が不正: got=%d, want=1", len(results))
	}
	if results[0].Status != VerifyOK {
		t.Errorf("ステータスが不正: got=%s, want=%s, detail=%s", results[0].Status, VerifyOK, results[0].Detail)
	}
}

// TestVerifyIncludes_SymlinkOK はsymlink済みファイルの検証がOKになることをテストする
func TestVerifyIncludes_SymlinkOK(t *testing.T) {
	worktreeDir := t.TempDir()

	srcPath := filepath.Join(worktreeDir, "src.txt")
	createTestFile(t, srcPath, "hello")

	dstPath := filepath.Join(worktreeDir, "dst.txt")
	absSrc, _ := filepath.Abs(srcPath)
	if err := os.Symlink(absSrc, dstPath); err != nil {
		t.Fatalf("symlink作成に失敗: %v", err)
	}

	include := config.IncludeConfig{
		Root: []config.IncludeEntry{
			{Source: "src.txt", Target: "dst.txt", Strategy: "symlink"},
		},
	}

	results := VerifyIncludes(worktreeDir, []string{"api"}, include, nil)
	if len(results) != 1 {
		t.Fatalf("結果の数が不正: got=%d, want=1", len(results))
	}
	if results[0].Status != VerifyOK {
		t.Errorf("ステータスが不正: got=%s, want=%s, detail=%s", results[0].Status, VerifyOK, results[0].Detail)
	}
}

// TestVerifyIncludes_TemplateOK はテンプレート展開済みファイルの検証がOKになることをテストする
func TestVerifyIncludes_TemplateOK(t *testing.T) {
	worktreeDir := t.TempDir()

	createTestFile(t, filepath.Join(worktreeDir, "tmpl.env"), "PORT=${port}")
	createTestFile(t, filepath.Join(worktreeDir, "api", "app.env"), "PORT=3000")

	include := config.IncludeConfig{
		PerService: map[string][]config.IncludeEntry{
			"api": {
				{Source: "tmpl.env", Target: "app.env", Strategy: "template"},
			},
		},
	}

	vars := map[string]string{"port": "3000"}
	results := VerifyIncludes(worktreeDir, []string{"api"}, include, vars)
	if len(results) != 1 {
		t.Fatalf("結果の数が不正: got=%d, want=1", len(results))
	}
	if results[0].Status != VerifyOK {
		t.Errorf("ステータスが不正: got=%s, want=%s, detail=%s", results[0].Status, VerifyOK, results[0].Detail)
	}
}

// TestVerifyIncludes_Missing はファイルが存在しない場合にmissingになることをテストする
func TestVerifyIncludes_Missing(t *testing.T) {
	worktreeDir := t.TempDir()

	createTestFile(t, filepath.Join(worktreeDir, "src.txt"), "hello")

	include := config.IncludeConfig{
		Root: []config.IncludeEntry{
			{Source: "src.txt", Target: "missing.txt", Strategy: "copy"},
		},
	}

	results := VerifyIncludes(worktreeDir, []string{"api"}, include, nil)
	if len(results) != 1 {
		t.Fatalf("結果の数が不正: got=%d, want=1", len(results))
	}
	if results[0].Status != VerifyMissing {
		t.Errorf("ステータスが不正: got=%s, want=%s", results[0].Status, VerifyMissing)
	}
}

// TestVerifyIncludes_BrokenLink は壊れたsymlinkの検出をテストする
func TestVerifyIncludes_BrokenLink(t *testing.T) {
	worktreeDir := t.TempDir()

	srcPath := filepath.Join(worktreeDir, "src.txt")
	createTestFile(t, srcPath, "hello")

	dstPath := filepath.Join(worktreeDir, "dst.txt")
	absSrc, _ := filepath.Abs(srcPath)
	if err := os.Symlink(absSrc, dstPath); err != nil {
		t.Fatalf("symlink作成に失敗: %v", err)
	}

	// ソースを削除してリンクを壊す
	os.Remove(srcPath)

	include := config.IncludeConfig{
		Root: []config.IncludeEntry{
			{Source: "src.txt", Target: "dst.txt", Strategy: "symlink"},
		},
	}

	results := VerifyIncludes(worktreeDir, []string{"api"}, include, nil)
	if len(results) != 1 {
		t.Fatalf("結果の数が不正: got=%d, want=1", len(results))
	}
	if results[0].Status != VerifyBrokenLink {
		t.Errorf("ステータスが不正: got=%s, want=%s", results[0].Status, VerifyBrokenLink)
	}
}

// TestVerifyIncludes_Mismatch は内容不一致の検出をテストする
func TestVerifyIncludes_Mismatch(t *testing.T) {
	worktreeDir := t.TempDir()

	createTestFile(t, filepath.Join(worktreeDir, "src.txt"), "original")
	createTestFile(t, filepath.Join(worktreeDir, "api", "dst.txt"), "modified")

	include := config.IncludeConfig{
		PerService: map[string][]config.IncludeEntry{
			"api": {
				{Source: "src.txt", Target: "dst.txt", Strategy: "copy"},
			},
		},
	}

	results := VerifyIncludes(worktreeDir, []string{"api"}, include, nil)
	if len(results) != 1 {
		t.Fatalf("結果の数が不正: got=%d, want=1", len(results))
	}
	if results[0].Status != VerifyMismatch {
		t.Errorf("ステータスが不正: got=%s, want=%s", results[0].Status, VerifyMismatch)
	}
}

// TestVerifyIncludes_SymlinkDir はディレクトリsymlinkの検証をテストする
func TestVerifyIncludes_SymlinkDir(t *testing.T) {
	worktreeDir := t.TempDir()

	srcDir := filepath.Join(worktreeDir, ".claude")
	createTestDir(t, srcDir)
	createTestFile(t, filepath.Join(srcDir, "config.json"), "{}")

	dstPath := filepath.Join(worktreeDir, "linked-claude")
	absSrc, _ := filepath.Abs(srcDir)
	if err := os.Symlink(absSrc, dstPath); err != nil {
		t.Fatalf("symlink作成に失敗: %v", err)
	}

	include := config.IncludeConfig{
		Root: []config.IncludeEntry{
			{Source: ".claude", Target: "linked-claude", Strategy: "symlink"},
		},
	}

	results := VerifyIncludes(worktreeDir, []string{"api"}, include, nil)
	if len(results) != 1 {
		t.Fatalf("結果の数が不正: got=%d, want=1", len(results))
	}
	if results[0].Status != VerifyOK {
		t.Errorf("ステータスが不正: got=%s, want=%s, detail=%s", results[0].Status, VerifyOK, results[0].Detail)
	}
}
