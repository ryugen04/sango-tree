package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)


// setupBareAndSource はテスト用のベアリポジトリとソースリポジトリをセットアップする
// ベアリポジトリに初期コミットをプッシュし、そのパスを返す
func setupBareAndSource(t *testing.T) (bareDir, sourceDir string) {
	t.Helper()
	tmpDir := t.TempDir()

	// ベアリポジトリを初期化する
	bareDir = filepath.Join(tmpDir, "origin.git")
	if err := os.MkdirAll(bareDir, 0o755); err != nil {
		t.Fatalf("ベアリポジトリディレクトリの作成に失敗: %v", err)
	}
	runGit(t, bareDir, "init", "--bare")

	// 通常リポジトリを作成してコミットをプッシュする
	sourceDir = filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("ソースリポジトリディレクトリの作成に失敗: %v", err)
	}
	runGit(t, sourceDir, "init", "-b", "main")
	runGit(t, sourceDir, "config", "user.name", "Test User")
	runGit(t, sourceDir, "config", "user.email", "test@example.com")
	// GPG署名を無効化してCIや隔離環境でもコミットできるようにする
	runGit(t, sourceDir, "config", "commit.gpgsign", "false")

	// 初期ファイルを作成してコミットする
	readmePath := filepath.Join(sourceDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("README.mdの作成に失敗: %v", err)
	}
	runGit(t, sourceDir, "add", ".")
	runGit(t, sourceDir, "commit", "-m", "initial commit")

	// ベアリポジトリをリモートとして追加してプッシュする
	runGit(t, sourceDir, "remote", "add", "origin", bareDir)
	runGit(t, sourceDir, "push", "origin", "main")

	return bareDir, sourceDir
}

// runGit はgitコマンドを指定ディレクトリで実行する（テストヘルパー）
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	// グローバルgit設定を無効化して署名等の干渉を防ぐ
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v 失敗 (dir=%s): %v\n%s", args, dir, err, out)
	}
}

// TestBareRepoDir はBareRepoDirが正しいパスを返すことを検証する
func TestBareRepoDir(t *testing.T) {
	groveDir := "/tmp/grove"
	name := "api"
	got := BareRepoDir(groveDir, name)
	want := "/tmp/grove/bare/api.git"
	if got != want {
		t.Errorf("BareRepoDir = %q, want %q", got, want)
	}
}

// TestBareClone はBareCloneが正しくベアリポジトリをクローンすることを検証する
func TestBareClone(t *testing.T) {
	originBare, _ := setupBareAndSource(t)

	groveDir := t.TempDir()
	name := "myservice"

	if err := BareClone(groveDir, name, originBare, false); err != nil {
		t.Fatalf("BareClone 失敗: %v", err)
	}

	// ベアリポジトリのディレクトリが作成されているか確認する
	clonedDir := BareRepoDir(groveDir, name)
	if _, err := os.Stat(clonedDir); err != nil {
		t.Fatalf("クローン先ディレクトリが存在しない: %v", err)
	}

	// HEADファイルが存在するか確認する（ベアリポジトリの指標）
	headPath := filepath.Join(clonedDir, "HEAD")
	if _, err := os.Stat(headPath); err != nil {
		t.Fatalf("HEADファイルが存在しない（ベアリポジトリではない可能性あり）: %v", err)
	}
}

// TestBareCloneShallow はshallowオプション付きのBareCloneを検証する
func TestBareCloneShallow(t *testing.T) {
	originBare, _ := setupBareAndSource(t)

	groveDir := t.TempDir()
	name := "shallowsvc"

	if err := BareClone(groveDir, name, originBare, true); err != nil {
		t.Fatalf("BareClone（shallow）失敗: %v", err)
	}

	clonedDir := BareRepoDir(groveDir, name)
	if _, err := os.Stat(clonedDir); err != nil {
		t.Fatalf("クローン先ディレクトリが存在しない: %v", err)
	}
}

// TestWorktreeAdd は既存ブランチからworktreeを追加できることを検証する
func TestWorktreeAdd(t *testing.T) {
	originBare, _ := setupBareAndSource(t)

	groveDir := t.TempDir()
	name := "myservice"

	// まずベアリポジトリをクローンする
	if err := BareClone(groveDir, name, originBare, false); err != nil {
		t.Fatalf("BareClone 失敗: %v", err)
	}

	// worktreeのパスを指定してmainブランチをチェックアウトする
	worktreePath := filepath.Join(t.TempDir(), "wt-main")
	if err := WorktreeAdd(groveDir, name, worktreePath, "main"); err != nil {
		t.Fatalf("WorktreeAdd 失敗: %v", err)
	}

	// worktreeディレクトリが作成されているか確認する
	if _, err := os.Stat(worktreePath); err != nil {
		t.Fatalf("worktreeディレクトリが存在しない: %v", err)
	}

	// README.mdが存在するか確認する（ファイルが正しくチェックアウトされた指標）
	readmePath := filepath.Join(worktreePath, "README.md")
	if _, err := os.Stat(readmePath); err != nil {
		t.Fatalf("README.mdが存在しない: %v", err)
	}
}

// TestWorktreeAddNewBranch は新規ブランチを作成してworktreeを追加できることを検証する
func TestWorktreeAddNewBranch(t *testing.T) {
	originBare, _ := setupBareAndSource(t)

	groveDir := t.TempDir()
	name := "myservice"

	// まずベアリポジトリをクローンする
	if err := BareClone(groveDir, name, originBare, false); err != nil {
		t.Fatalf("BareClone 失敗: %v", err)
	}

	// 新規ブランチfeature/newをmainから作成してworktreeとして追加する
	worktreePath := filepath.Join(t.TempDir(), "wt-feature")
	if err := WorktreeAddNewBranch(groveDir, name, worktreePath, "feature/new", "main"); err != nil {
		t.Fatalf("WorktreeAddNewBranch 失敗: %v", err)
	}

	// worktreeディレクトリが作成されているか確認する
	if _, err := os.Stat(worktreePath); err != nil {
		t.Fatalf("worktreeディレクトリが存在しない: %v", err)
	}

	// README.mdが存在するか確認する
	readmePath := filepath.Join(worktreePath, "README.md")
	if _, err := os.Stat(readmePath); err != nil {
		t.Fatalf("README.mdが存在しない: %v", err)
	}
}

// TestWorktreeRemove はworktreeを正しく削除できることを検証する
func TestWorktreeRemove(t *testing.T) {
	originBare, _ := setupBareAndSource(t)

	groveDir := t.TempDir()
	name := "myservice"

	// ベアリポジトリをクローンしてworktreeを追加する
	if err := BareClone(groveDir, name, originBare, false); err != nil {
		t.Fatalf("BareClone 失敗: %v", err)
	}

	worktreePath := filepath.Join(t.TempDir(), "wt-to-remove")
	if err := WorktreeAdd(groveDir, name, worktreePath, "main"); err != nil {
		t.Fatalf("WorktreeAdd 失敗: %v", err)
	}

	// worktreeが存在することを確認してから削除する
	if _, err := os.Stat(worktreePath); err != nil {
		t.Fatalf("削除前にworktreeが存在しない: %v", err)
	}

	if err := WorktreeRemove(groveDir, name, worktreePath, false); err != nil {
		t.Fatalf("WorktreeRemove 失敗: %v", err)
	}

	// worktreeディレクトリが削除されているか確認する
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Fatalf("WorktreeRemove後もディレクトリが残っている: %v", err)
	}
}

// TestWorktreeRemoveForce はforce付きのworktree削除を検証する
func TestWorktreeRemoveForce(t *testing.T) {
	originBare, _ := setupBareAndSource(t)

	groveDir := t.TempDir()
	name := "myservice"

	// ベアリポジトリをクローンしてworktreeを追加する
	if err := BareClone(groveDir, name, originBare, false); err != nil {
		t.Fatalf("BareClone 失敗: %v", err)
	}

	worktreePath := filepath.Join(t.TempDir(), "wt-force-remove")
	if err := WorktreeAdd(groveDir, name, worktreePath, "main"); err != nil {
		t.Fatalf("WorktreeAdd 失敗: %v", err)
	}

	// worktree内に未コミットファイルを作成する（force削除が必要な状況を作る）
	dirtyFile := filepath.Join(worktreePath, "dirty.txt")
	if err := os.WriteFile(dirtyFile, []byte("uncommitted\n"), 0o644); err != nil {
		t.Fatalf("未コミットファイルの作成に失敗: %v", err)
	}

	// force=trueで削除する
	if err := WorktreeRemove(groveDir, name, worktreePath, true); err != nil {
		t.Fatalf("WorktreeRemove（force）失敗: %v", err)
	}

	// worktreeディレクトリが削除されているか確認する
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Fatalf("WorktreeRemove後もディレクトリが残っている: %v", err)
	}
}
