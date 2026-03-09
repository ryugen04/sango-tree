package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// BareRepoDir はベアリポジトリのディレクトリパスを返す
// 形式: <sangoDir>/bare/<name>.git
func BareRepoDir(sangoDir, name string) string {
	return filepath.Join(sangoDir, "bare", name+".git")
}

// BareClone はリポジトリをベアリポジトリとしてクローンする
// クローン先: .sango/bare/<name>.git
// shallow が true の場合は --depth 1 オプションを付与する
func BareClone(sangoDir, name, repoURL string, shallow bool) error {
	target := BareRepoDir(sangoDir, name)

	args := []string{"clone", "--bare"}
	if shallow {
		args = append(args, "--depth", "1")
	}
	args = append(args, repoURL, target)

	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone --bare 失敗 (%s): %w\n%s", name, err, out)
	}
	return nil
}

// WorktreeAdd は既存ブランチからgit worktreeを追加する
// ベアリポジトリのディレクトリから: git worktree add <path> <branch>
func WorktreeAdd(sangoDir, name, worktreePath, branch string) error {
	bareDir := BareRepoDir(sangoDir, name)

	cmd := exec.Command("git", "worktree", "add", worktreePath, branch)
	cmd.Dir = bareDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree add 失敗 (%s, branch=%s): %w\n%s", name, branch, err, out)
	}
	return nil
}

// WorktreeAddNewBranch は新規ブランチを作成してworktreeとして追加する
// ベアリポジトリのディレクトリから: git worktree add -b <newBranch> <path> <baseBranch>
func WorktreeAddNewBranch(sangoDir, name, worktreePath, newBranch, baseBranch string) error {
	bareDir := BareRepoDir(sangoDir, name)

	cmd := exec.Command("git", "worktree", "add", "-b", newBranch, worktreePath, baseBranch)
	cmd.Dir = bareDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree add -b 失敗 (%s, newBranch=%s): %w\n%s", name, newBranch, err, out)
	}
	return nil
}

// FetchOrigin はベアリポジトリで git fetch origin を実行する
func FetchOrigin(bareDir string) error {
	cmd := exec.Command("git", "fetch", "origin")
	cmd.Dir = bareDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// HasUncommittedChanges はworktreeディレクトリに未コミット変更があるか確認する
func HasUncommittedChanges(dir string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status 失敗 (%s): %w", dir, err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// GitStash はworktreeディレクトリで git stash を実行する
func GitStash(dir string) error {
	cmd := exec.Command("git", "stash")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git stash 失敗 (%s): %w\n%s", dir, err, out)
	}
	return nil
}

// GitStashPop はworktreeディレクトリで git stash pop を実行する
func GitStashPop(dir string) error {
	cmd := exec.Command("git", "stash", "pop")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git stash pop 失敗 (%s): %w\n%s", dir, err, out)
	}
	return nil
}

// GitRebase はworktreeディレクトリで git rebase origin/<baseBranch> を実行する
func GitRebase(dir, baseBranch string) error {
	cmd := exec.Command("git", "rebase", "origin/"+baseBranch)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git rebase 失敗 (%s, base=%s): %w\n%s", dir, baseBranch, err, out)
	}
	return nil
}

// GitRebaseAbort はworktreeディレクトリで git rebase --abort を実行する
func GitRebaseAbort(dir string) error {
	cmd := exec.Command("git", "rebase", "--abort")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git rebase --abort 失敗 (%s): %w\n%s", dir, err, out)
	}
	return nil
}

// WorktreeRemove はgit worktreeを削除する
// ベアリポジトリのディレクトリから: git worktree remove [--force] <path>
func WorktreeRemove(sangoDir, name, worktreePath string, force bool) error {
	bareDir := BareRepoDir(sangoDir, name)

	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, worktreePath)

	cmd := exec.Command("git", args...)
	cmd.Dir = bareDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove 失敗 (%s, path=%s): %w\n%s", name, worktreePath, err, out)
	}
	return nil
}
