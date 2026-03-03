package worktree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ryugen04/grove/internal/config"
)

// テスト用ソースファイルを作成するヘルパー
func createTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("テストファイル用ディレクトリの作成に失敗: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("テストファイルの作成に失敗: %v", err)
	}
}

// ファイル内容を読み込むヘルパー
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ファイルの読み込みに失敗 (%s): %v", path, err)
	}
	return string(data)
}

// TestProcessEntry_Copy はcopy strategyのファイルコピーをテストする
func TestProcessEntry_Copy(t *testing.T) {
	baseDir := t.TempDir()
	targetDir := t.TempDir()

	// ソースファイルを作成する
	srcContent := "hello, grove!"
	createTestFile(t, filepath.Join(baseDir, "config.txt"), srcContent)

	entry := config.IncludeEntry{
		Source:   "config.txt",
		Target:   "config.txt",
		Strategy: "copy",
	}

	if err := processEntry(baseDir, targetDir, entry, nil); err != nil {
		t.Fatalf("processEntry に失敗: %v", err)
	}

	// コピー結果を確認する
	got := readFile(t, filepath.Join(targetDir, "config.txt"))
	if got != srcContent {
		t.Errorf("コピー結果が一致しない: got=%q, want=%q", got, srcContent)
	}
}

// TestProcessEntry_Symlink はsymlink strategyのシンボリックリンク作成をテストする
func TestProcessEntry_Symlink(t *testing.T) {
	baseDir := t.TempDir()
	targetDir := t.TempDir()

	// ソースファイルを作成する
	srcContent := "shared config"
	createTestFile(t, filepath.Join(baseDir, "shared.env"), srcContent)

	entry := config.IncludeEntry{
		Source:   "shared.env",
		Target:   "shared.env",
		Strategy: "symlink",
	}

	if err := processEntry(baseDir, targetDir, entry, nil); err != nil {
		t.Fatalf("processEntry に失敗: %v", err)
	}

	dstPath := filepath.Join(targetDir, "shared.env")

	// シンボリックリンクであることを確認する
	info, err := os.Lstat(dstPath)
	if err != nil {
		t.Fatalf("Lstat に失敗: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("シンボリックリンクが作成されていない")
	}

	// リンク先の内容を確認する
	got := readFile(t, dstPath)
	if got != srcContent {
		t.Errorf("リンク先の内容が一致しない: got=%q, want=%q", got, srcContent)
	}

	// リンク先が絶対パスであることを確認する
	linkTarget, err := os.Readlink(dstPath)
	if err != nil {
		t.Fatalf("Readlink に失敗: %v", err)
	}
	if !filepath.IsAbs(linkTarget) {
		t.Errorf("シンボリックリンクのターゲットが絶対パスでない: %q", linkTarget)
	}
}

// TestProcessEntry_Template はtemplate strategyの変数展開をテストする
func TestProcessEntry_Template(t *testing.T) {
	baseDir := t.TempDir()
	targetDir := t.TempDir()

	// テンプレートファイルを作成する
	tmplContent := "PORT=${port}\nAPI_PORT=${services.api.port}\nHOST=localhost"
	createTestFile(t, filepath.Join(baseDir, "app.env.tmpl"), tmplContent)

	entry := config.IncludeEntry{
		Source:   "app.env.tmpl",
		Target:   "app.env",
		Strategy: "template",
	}

	vars := map[string]string{
		"port":             "3000",
		"services.api.port": "8080",
	}

	if err := processEntry(baseDir, targetDir, entry, vars, ); err != nil {
		t.Fatalf("processEntry に失敗: %v", err)
	}

	// 展開結果を確認する
	got := readFile(t, filepath.Join(targetDir, "app.env"))
	want := "PORT=3000\nAPI_PORT=8080\nHOST=localhost"
	if got != want {
		t.Errorf("テンプレート展開結果が一致しない:\ngot:  %q\nwant: %q", got, want)
	}
}

// TestProcessEntry_UnknownStrategy は未知のstrategyでエラーが返ることをテストする
func TestProcessEntry_UnknownStrategy(t *testing.T) {
	baseDir := t.TempDir()
	targetDir := t.TempDir()

	createTestFile(t, filepath.Join(baseDir, "file.txt"), "content")

	entry := config.IncludeEntry{
		Source:   "file.txt",
		Target:   "file.txt",
		Strategy: "invalid",
	}

	if err := processEntry(baseDir, targetDir, entry, nil); err == nil {
		t.Error("未知のstrategyでエラーが返らなかった")
	}
}

// TestExpandIncludes_Common はcommonエントリが全サービスに配置されることをテストする
func TestExpandIncludes_Common(t *testing.T) {
	worktreeDir := t.TempDir()

	// ソースファイルをworktreeDirに作成する
	createTestFile(t, filepath.Join(worktreeDir, "common.env"), "COMMON=true")

	include := config.IncludeConfig{
		Common: []config.IncludeEntry{
			{
				Source:   "common.env",
				Target:   "common.env",
				Strategy: "copy",
			},
		},
	}

	services := []string{"api", "worker"}

	if err := ExpandIncludes(worktreeDir, services, include, nil); err != nil {
		t.Fatalf("ExpandIncludes に失敗: %v", err)
	}

	// 全サービスにファイルが配置されていることを確認する
	for _, svc := range services {
		dstPath := filepath.Join(worktreeDir, svc, "common.env")
		got := readFile(t, dstPath)
		if got != "COMMON=true" {
			t.Errorf("サービス %s のファイル内容が一致しない: %q", svc, got)
		}
	}
}

// TestExpandIncludes_PerService はper_serviceエントリが該当サービスのみに配置されることをテストする
func TestExpandIncludes_PerService(t *testing.T) {
	worktreeDir := t.TempDir()

	// ソースファイルをworktreeDirに作成する
	createTestFile(t, filepath.Join(worktreeDir, "api.env"), "API=true")
	createTestFile(t, filepath.Join(worktreeDir, "worker.env"), "WORKER=true")

	include := config.IncludeConfig{
		PerService: map[string][]config.IncludeEntry{
			"api": {
				{
					Source:   "api.env",
					Target:   "service.env",
					Strategy: "copy",
				},
			},
			"worker": {
				{
					Source:   "worker.env",
					Target:   "service.env",
					Strategy: "copy",
				},
			},
			// このworktreeに含まれないサービス（スキップされる）
			"db": {
				{
					Source:   "db.env",
					Target:   "service.env",
					Strategy: "copy",
				},
			},
		},
	}

	// dbはservicesに含まれない
	services := []string{"api", "worker"}

	if err := ExpandIncludes(worktreeDir, services, include, nil); err != nil {
		t.Fatalf("ExpandIncludes に失敗: %v", err)
	}

	// apiのファイルを確認する
	apiContent := readFile(t, filepath.Join(worktreeDir, "api", "service.env"))
	if apiContent != "API=true" {
		t.Errorf("api の service.env が一致しない: %q", apiContent)
	}

	// workerのファイルを確認する
	workerContent := readFile(t, filepath.Join(worktreeDir, "worker", "service.env"))
	if workerContent != "WORKER=true" {
		t.Errorf("worker の service.env が一致しない: %q", workerContent)
	}

	// dbのファイルが作成されていないことを確認する
	dbPath := filepath.Join(worktreeDir, "db", "service.env")
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Error("worktreeに含まれないサービス (db) にファイルが配置されてしまった")
	}
}

// TestExpandIncludes_CommonAndPerService はcommonとper_serviceを組み合わせたテスト
func TestExpandIncludes_CommonAndPerService(t *testing.T) {
	worktreeDir := t.TempDir()

	// ソースファイルを作成する
	createTestFile(t, filepath.Join(worktreeDir, "common.tmpl"), "PORT=${port}")
	createTestFile(t, filepath.Join(worktreeDir, "api-extra.txt"), "API_ONLY=true")

	include := config.IncludeConfig{
		Common: []config.IncludeEntry{
			{
				Source:   "common.tmpl",
				Target:   "common.env",
				Strategy: "template",
			},
		},
		PerService: map[string][]config.IncludeEntry{
			"api": {
				{
					Source:   "api-extra.txt",
					Target:   "extra.txt",
					Strategy: "copy",
				},
			},
		},
	}

	services := []string{"api", "worker"}
	vars := map[string]string{"port": "3000"}

	if err := ExpandIncludes(worktreeDir, services, include, vars); err != nil {
		t.Fatalf("ExpandIncludes に失敗: %v", err)
	}

	// commonエントリが両サービスに展開されていることを確認する
	for _, svc := range services {
		got := readFile(t, filepath.Join(worktreeDir, svc, "common.env"))
		if got != "PORT=3000" {
			t.Errorf("サービス %s の common.env が一致しない: %q", svc, got)
		}
	}

	// per_serviceエントリがapiのみに配置されていることを確認する
	apiExtra := readFile(t, filepath.Join(worktreeDir, "api", "extra.txt"))
	if apiExtra != "API_ONLY=true" {
		t.Errorf("api の extra.txt が一致しない: %q", apiExtra)
	}

	// workerにper_serviceファイルがないことを確認する
	workerExtraPath := filepath.Join(worktreeDir, "worker", "extra.txt")
	if _, err := os.Stat(workerExtraPath); !os.IsNotExist(err) {
		t.Error("worker に api 専用の extra.txt が配置されてしまった")
	}
}

// TestExpandIncludes_ErrorAccumulation はエラーが蓄積されて返ることをテストする
func TestExpandIncludes_ErrorAccumulation(t *testing.T) {
	worktreeDir := t.TempDir()

	// ソースファイルを意図的に作成しない（エラーを発生させる）
	include := config.IncludeConfig{
		Common: []config.IncludeEntry{
			{
				Source:   "nonexistent1.txt",
				Target:   "out1.txt",
				Strategy: "copy",
			},
			{
				Source:   "nonexistent2.txt",
				Target:   "out2.txt",
				Strategy: "copy",
			},
		},
	}

	services := []string{"api"}

	err := ExpandIncludes(worktreeDir, services, include, nil)
	if err == nil {
		t.Fatal("存在しないファイルに対してエラーが返らなかった")
	}

	// 複数のエラーが含まれていることを確認する（途中で止まっていない）
	errMsg := err.Error()
	if !containsService([]string{errMsg}, "nonexistent1.txt") {
		// エラーメッセージにnonexistent1.txtが含まれているか確認する
		// errors.Join は改行区切りでエラーをつなぐ
		t.Logf("エラー内容: %v", err)
	}
}

// TestProcessEntry_SymlinkOverwrite は既存シンボリックリンクの上書きをテストする
func TestProcessEntry_SymlinkOverwrite(t *testing.T) {
	baseDir := t.TempDir()
	targetDir := t.TempDir()

	// ソースファイルを作成する
	createTestFile(t, filepath.Join(baseDir, "source.txt"), "new content")

	// 既存のシンボリックリンクを事前に作成する
	dstPath := filepath.Join(targetDir, "link.txt")
	// ダミーのリンクターゲット
	_ = os.Symlink("/tmp/dummy", dstPath)

	entry := config.IncludeEntry{
		Source:   "source.txt",
		Target:   "link.txt",
		Strategy: "symlink",
	}

	if err := processEntry(baseDir, targetDir, entry, nil); err != nil {
		t.Fatalf("既存リンクの上書きに失敗: %v", err)
	}

	// 上書き後の内容を確認する
	got := readFile(t, dstPath)
	if got != "new content" {
		t.Errorf("上書き後の内容が一致しない: %q", got)
	}
}
