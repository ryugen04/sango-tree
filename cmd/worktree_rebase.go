package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/ryugen04/sango-tree/internal/worktree"
	"github.com/spf13/cobra"
)

var worktreeRebaseCmd = &cobra.Command{
	Use:   "rebase [worktree-name]",
	Short: "ワークツリーの各リポジトリをrebaseする",
	Long:  "ベースブランチに対してgit rebaseを実行する。未コミット変更はstashして復元する",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		sangoDir := worktree.DefaultDir()
		ws, err := worktree.Load(sangoDir)
		if err != nil {
			return fmt.Errorf("worktrees.jsonの読み込みに失敗: %w", err)
		}

		// 対象worktree名を決定
		var wtName string
		if len(args) > 0 {
			wtName = args[0]
		} else {
			wtName = ws.Active
		}
		if wtName == "" {
			return fmt.Errorf("ワークツリー名を指定するか、アクティブワークツリーを設定してください")
		}

		wtInfo, exists := ws.Worktrees[wtName]
		if !exists {
			return fmt.Errorf("ワークツリー %q は存在しません", wtName)
		}

		// ベースブランチを決定
		baseBranch := wtInfo.FromBranch
		if baseBranch == "" {
			baseBranch = cfg.Worktree.DefaultBranch
			if baseBranch == "" {
				baseBranch = "main"
			}
		}

		fmt.Printf("[sango] Rebasing worktree %s...\n", wtName)

		var hasError bool
		for _, svcName := range wtInfo.Services {
			svc, ok := cfg.Services[svcName]
			if !ok {
				continue
			}
			// dockerサービスやリポジトリなしのサービスはスキップ
			if svc.Type == "docker" || svc.Repo == "" {
				continue
			}

			// リポジトリ名を解決
			repoName := svcName
			if svc.RepoName != "" {
				repoName = svc.RepoName
			}

			// ベアリポジトリでfetch
			bareDir := worktree.BareRepoDir(sangoDir, svcName)
			fmt.Printf("[sango] Fetching latest changes for %s...\n", repoName)
			if err := worktree.FetchOrigin(bareDir); err != nil {
				fmt.Printf("[sango]   ✗ %s: fetch failed: %v\n", repoName, err)
				hasError = true
				continue
			}

			// worktreeディレクトリを特定
			wtDir := filepath.Join(cfg.Worktree.WorktreeDir(wtName), svcName)
			absWtDir, err := filepath.Abs(wtDir)
			if err != nil {
				fmt.Printf("[sango]   ✗ %s: パス解決に失敗: %v\n", repoName, err)
				hasError = true
				continue
			}

			fmt.Printf("[sango] Rebasing %s onto %s...\n", repoName, baseBranch)

			// 未コミット変更の確認
			hasChanges, err := worktree.HasUncommittedChanges(absWtDir)
			if err != nil {
				fmt.Printf("[sango]   ✗ %s: 状態確認に失敗: %v\n", repoName, err)
				hasError = true
				continue
			}

			// stash
			if hasChanges {
				if err := worktree.GitStash(absWtDir); err != nil {
					fmt.Printf("[sango]   ✗ %s: stashに失敗: %v\n", repoName, err)
					hasError = true
					continue
				}
			}

			// rebase
			if err := worktree.GitRebase(absWtDir, baseBranch); err != nil {
				fmt.Printf("[sango]   ✗ %s: rebase conflict detected, aborted\n", repoName)
				fmt.Printf("[sango]     Run manually: cd %s && git rebase origin/%s\n", absWtDir, baseBranch)
				worktree.GitRebaseAbort(absWtDir)
				// stashがあった場合はpopして復元
				if hasChanges {
					worktree.GitStashPop(absWtDir)
				}
				hasError = true
				continue
			}

			// stash pop
			if hasChanges {
				if err := worktree.GitStashPop(absWtDir); err != nil {
					fmt.Printf("[sango]   ✗ %s: stash popに失敗: %v\n", repoName, err)
					hasError = true
					continue
				}
				fmt.Printf("[sango]   ✓ %s: rebased (stashed changes restored)\n", repoName)
			} else {
				fmt.Printf("[sango]   ✓ %s: rebased (no local changes)\n", repoName)
			}
		}

		if hasError {
			fmt.Println("[sango] Done (with errors).")
		} else {
			fmt.Println("[sango] Done.")
		}
		return nil
	},
}

func init() {
	worktreeCmd.AddCommand(worktreeRebaseCmd)
}
