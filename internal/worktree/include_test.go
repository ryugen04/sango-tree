package worktree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ryugen04/sango-tree/internal/config"
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

// テスト用ディレクトリを作成するヘルパー
func createTestDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("テストディレクトリの作成に失敗: %v", err)
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
	srcContent := "hello, sango!"
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

// TestProcessEntry_SymlinkDir はディレクトリへのsymlinkが正しく作成されるかテストする
func TestProcessEntry_SymlinkDir(t *testing.T) {
	baseDir := t.TempDir()
	targetDir := t.TempDir()

	// ソースディレクトリを作成する
	srcDir := filepath.Join(baseDir, ".claude")
	createTestDir(t, srcDir)
	createTestFile(t, filepath.Join(srcDir, "settings.json"), `{"key": "value"}`)

	entry := config.IncludeEntry{
		Source:   ".claude",
		Target:   ".claude",
		Strategy: "symlink",
	}

	if err := processEntry(baseDir, targetDir, entry, nil); err != nil {
		t.Fatalf("processEntry に失敗: %v", err)
	}

	dstPath := filepath.Join(targetDir, ".claude")

	// シンボリックリンクであることを確認する
	info, err := os.Lstat(dstPath)
	if err != nil {
		t.Fatalf("Lstat に失敗: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("ディレクトリへのシンボリックリンクが作成されていない")
	}

	// リンク先のファイルにアクセスできることを確認する
	got := readFile(t, filepath.Join(dstPath, "settings.json"))
	if got != `{"key": "value"}` {
		t.Errorf("リンク先ディレクトリの内容が一致しない: %q", got)
	}
}

// TestProcessEntry_CopyDir_Error はディレクトリにcopy指定でエラーが返ることをテストする
func TestProcessEntry_CopyDir_Error(t *testing.T) {
	baseDir := t.TempDir()
	targetDir := t.TempDir()

	// ソースディレクトリを作成する
	createTestDir(t, filepath.Join(baseDir, "mydir"))

	entry := config.IncludeEntry{
		Source:   "mydir",
		Target:   "mydir",
		Strategy: "copy",
	}

	if err := processEntry(baseDir, targetDir, entry, nil); err == nil {
		t.Error("ディレクトリにcopy指定でエラーが返らなかった")
	}
}

// TestProcessEntry_TemplateDir_Error はディレクトリにtemplate指定でエラーが返ることをテストする
func TestProcessEntry_TemplateDir_Error(t *testing.T) {
	baseDir := t.TempDir()
	targetDir := t.TempDir()

	createTestDir(t, filepath.Join(baseDir, "mydir"))

	entry := config.IncludeEntry{
		Source:   "mydir",
		Target:   "mydir",
		Strategy: "template",
	}

	if err := processEntry(baseDir, targetDir, entry, nil); err == nil {
		t.Error("ディレクトリにtemplate指定でエラーが返らなかった")
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
		"port":              "3000",
		"services.api.port": "8080",
	}

	if err := processEntry(baseDir, targetDir, entry, vars); err != nil {
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

// TestExpandIncludes_Root はrootエントリがworktreeルートに配置されることをテストする
func TestExpandIncludes_Root(t *testing.T) {
	worktreeDir := t.TempDir()

	// ソースファイルをworktreeDirに作成する
	createTestFile(t, filepath.Join(worktreeDir, "root.env"), "ROOT=true")

	include := config.IncludeConfig{
		Root: []config.IncludeEntry{
			{
				Source:   "root.env",
				Target:   "root-copy.env",
				Strategy: "copy",
			},
		},
	}

	services := []string{"api", "worker"}

	result := ExpandIncludes(worktreeDir, worktreeDir, services, include, nil)
	if result.HasErrors() {
		t.Fatalf("ExpandIncludes でエラー: %v", result.CriticalError())
	}
	if result.WarningError() != nil {
		t.Fatalf("ExpandIncludes で警告: %v", result.WarningError())
	}

	// worktreeルートにファイルが配置されていることを確認する
	got := readFile(t, filepath.Join(worktreeDir, "root-copy.env"))
	if got != "ROOT=true" {
		t.Errorf("ルートのファイル内容が一致しない: %q", got)
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

	result := ExpandIncludes(worktreeDir, worktreeDir, services, include, nil)
	if result.HasErrors() {
		t.Fatalf("ExpandIncludes でエラー: %v", result.CriticalError())
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

// TestExpandIncludes_RootAndPerService はrootとper_serviceを組み合わせたテスト
func TestExpandIncludes_RootAndPerService(t *testing.T) {
	worktreeDir := t.TempDir()

	// ソースファイルを作成する
	createTestFile(t, filepath.Join(worktreeDir, "shared.env"), "SHARED=true")
	createTestFile(t, filepath.Join(worktreeDir, "api-extra.txt"), "API_ONLY=true")

	include := config.IncludeConfig{
		Root: []config.IncludeEntry{
			{
				Source:   "shared.env",
				Target:   "shared-copy.env",
				Strategy: "copy",
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

	result := ExpandIncludes(worktreeDir, worktreeDir, services, include, nil)
	if result.HasErrors() {
		t.Fatalf("ExpandIncludes でエラー: %v", result.CriticalError())
	}

	// rootエントリがworktreeルートに配置されていることを確認する
	got := readFile(t, filepath.Join(worktreeDir, "shared-copy.env"))
	if got != "SHARED=true" {
		t.Errorf("ルートの shared-copy.env が一致しない: %q", got)
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

// TestExpandIncludes_RequiredError はrequired=trueのエントリ失敗がErrorsに入ることをテストする
func TestExpandIncludes_RequiredError(t *testing.T) {
	worktreeDir := t.TempDir()

	include := config.IncludeConfig{
		Root: []config.IncludeEntry{
			{
				Source:   "nonexistent.txt",
				Target:   "out.txt",
				Strategy: "copy",
				Required: true,
			},
		},
	}

	result := ExpandIncludes(worktreeDir, worktreeDir, []string{"api"}, include, nil)
	if !result.HasErrors() {
		t.Fatal("required=trueのエントリ失敗がErrorsに入っていない")
	}
	if result.WarningError() != nil {
		t.Error("required=trueの失敗がWarningsにも入ってしまった")
	}
}

// TestExpandIncludes_OptionalWarning はrequired=falseのエントリ失敗がWarningsに入ることをテストする
func TestExpandIncludes_OptionalWarning(t *testing.T) {
	worktreeDir := t.TempDir()

	include := config.IncludeConfig{
		Root: []config.IncludeEntry{
			{
				Source:   "nonexistent.txt",
				Target:   "out.txt",
				Strategy: "copy",
				Required: false,
			},
		},
	}

	result := ExpandIncludes(worktreeDir, worktreeDir, []string{"api"}, include, nil)
	if result.HasErrors() {
		t.Error("required=falseの失敗がErrorsに入ってしまった")
	}
	if result.WarningError() == nil {
		t.Fatal("required=falseの失敗がWarningsに入っていない")
	}
}

// TestExpandIncludes_PerServiceRequired はper_serviceのrequiredエントリ失敗をテストする
func TestExpandIncludes_PerServiceRequired(t *testing.T) {
	worktreeDir := t.TempDir()

	include := config.IncludeConfig{
		PerService: map[string][]config.IncludeEntry{
			"api": {
				{
					Source:   "nonexistent.txt",
					Target:   "out.txt",
					Strategy: "copy",
					Required: true,
				},
			},
		},
	}

	result := ExpandIncludes(worktreeDir, worktreeDir, []string{"api"}, include, nil)
	if !result.HasErrors() {
		t.Fatal("per_serviceのrequired=trueエントリ失敗がErrorsに入っていない")
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
