package worktree

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ryugen04/sango-tree/internal/config"
)

// RebaseResult はrebase結果を保持する
type RebaseResult struct {
	Rebased  []string
	Failed   []string
	HasError bool
}

// RebaseServices はworktree内の全リポジトリサービスをrebaseする
func RebaseServices(cfg *config.Config, sangoDir string, wtName string, wtInfo *WorktreeInfo, baseBranch string) *RebaseResult {
	result := &RebaseResult{}
	wtDir := cfg.Worktree.WorktreeDir(wtName)

	for _, svcName := range wtInfo.Services {
		svc, ok := cfg.Services[svcName]
		if !ok || svc.Repo == "" {
			continue
		}

		// リポジトリ名を解決
		repoName := svcName
		if svc.RepoName != "" {
			repoName = svc.RepoName
		}

		// ベアリポジトリでfetch
		bareDir := BareRepoDir(sangoDir, svcName)
		fmt.Fprintf(os.Stderr, "[sango] Fetching latest changes for %s...\n", repoName)
		if err := FetchOrigin(bareDir); err != nil {
			fmt.Fprintf(os.Stderr, "[sango]   ✗ %s: fetch failed: %v\n", repoName, err)
			result.Failed = append(result.Failed, svcName)
			result.HasError = true
			continue
		}

		// worktreeディレクトリを特定
		absWtDir := filepath.Join(wtDir, svcName)
		if !filepath.IsAbs(absWtDir) {
			abs, err := filepath.Abs(absWtDir)
			if err == nil {
				absWtDir = abs
			}
		}

		fmt.Fprintf(os.Stderr, "[sango] Rebasing %s onto %s...\n", repoName, baseBranch)

		// 未コミット変更の確認
		hasChanges, err := HasUncommittedChanges(absWtDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[sango]   ✗ %s: 状態確認に失敗: %v\n", repoName, err)
			result.Failed = append(result.Failed, svcName)
			result.HasError = true
			continue
		}

		// stash
		if hasChanges {
			if err := GitStash(absWtDir); err != nil {
				fmt.Fprintf(os.Stderr, "[sango]   ✗ %s: stashに失敗: %v\n", repoName, err)
				result.Failed = append(result.Failed, svcName)
				result.HasError = true
				continue
			}
		}

		// rebase
		if err := GitRebase(absWtDir, baseBranch); err != nil {
			fmt.Fprintf(os.Stderr, "[sango]   ✗ %s: rebase conflict detected, aborted\n", repoName)
			fmt.Fprintf(os.Stderr, "[sango]     Run manually: cd %s && git rebase origin/%s\n", absWtDir, baseBranch)
			GitRebaseAbort(absWtDir)
			// stashがあった場合はpopして復元
			if hasChanges {
				GitStashPop(absWtDir)
			}
			result.Failed = append(result.Failed, svcName)
			result.HasError = true
			continue
		}

		// stash pop
		if hasChanges {
			if err := GitStashPop(absWtDir); err != nil {
				fmt.Fprintf(os.Stderr, "[sango]   ✗ %s: stash popに失敗: %v\n", repoName, err)
				result.Failed = append(result.Failed, svcName)
				result.HasError = true
				continue
			}
			fmt.Fprintf(os.Stderr, "[sango]   ✓ %s: rebased (stashed changes restored)\n", repoName)
		} else {
			fmt.Fprintf(os.Stderr, "[sango]   ✓ %s: rebased (no local changes)\n", repoName)
		}

		result.Rebased = append(result.Rebased, svcName)
	}

	return result
}
