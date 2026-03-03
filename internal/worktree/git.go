package worktree

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// BareRepoDir はベアリポジトリのディレクトリパスを返す
// 形式: <groveDir>/bare/<name>.git
func BareRepoDir(groveDir, name string) string {
	return filepath.Join(groveDir, "bare", name+".git")
}

// BareClone はリポジトリをベアリポジトリとしてクローンする
// クローン先: .grove/bare/<name>.git
// shallow が true の場合は --depth 1 オプションを付与する
func BareClone(groveDir, name, repoURL string, shallow bool) error {
	target := BareRepoDir(groveDir, name)

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
func WorktreeAdd(groveDir, name, worktreePath, branch string) error {
	bareDir := BareRepoDir(groveDir, name)

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
func WorktreeAddNewBranch(groveDir, name, worktreePath, newBranch, baseBranch string) error {
	bareDir := BareRepoDir(groveDir, name)

	cmd := exec.Command("git", "worktree", "add", "-b", newBranch, worktreePath, baseBranch)
	cmd.Dir = bareDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree add -b 失敗 (%s, newBranch=%s): %w\n%s", name, newBranch, err, out)
	}
	return nil
}

// WorktreeRemove はgit worktreeを削除する
// ベアリポジトリのディレクトリから: git worktree remove [--force] <path>
func WorktreeRemove(groveDir, name, worktreePath string, force bool) error {
	bareDir := BareRepoDir(groveDir, name)

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
