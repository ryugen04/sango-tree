package cmd

import (
	"fmt"
	"os"

	"github.com/ryugen04/sango-tree/internal/service"
	"github.com/ryugen04/sango-tree/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	wtSyncNoSetup bool
	wtSyncNoHooks bool
)

var worktreeSyncCmd = &cobra.Command{
	Use:   "sync [worktree-name]",
	Short: "ワークツリーをrebase + 後処理を一括実行する",
	Long:  "git rebase実行後にinclude再展開・setup・hooksを実行する",
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

		// worktree名解決
		wtName := ""
		if len(args) > 0 {
			wtName = args[0]
		} else {
			wtName = service.ResolveActiveWorktree(sangoDir, worktreeFlag)
		}

		wtInfo, ok := ws.Worktrees[wtName]
		if !ok {
			return fmt.Errorf("ワークツリー %q が見つかりません", wtName)
		}

		baseBranch := wtInfo.FromBranch
		if baseBranch == "" {
			baseBranch = cfg.Worktree.DefaultBranch
			if baseBranch == "" {
				baseBranch = "main"
			}
		}

		// 1. Rebase
		fmt.Fprintf(os.Stderr, "[sango] %s: rebase開始...\n", wtName)
		rebaseResult := worktree.RebaseServices(cfg, sangoDir, wtName, wtInfo, baseBranch)

		for _, name := range rebaseResult.Rebased {
			fmt.Fprintf(os.Stderr, "  rebase完了: %s\n", name)
		}
		for _, name := range rebaseResult.Failed {
			fmt.Fprintf(os.Stderr, "  rebase失敗: %s\n", name)
		}

		// 2. 後処理（rebase失敗があっても実行する - 成功したサービスのinclude等は更新すべき）
		fmt.Fprintf(os.Stderr, "[sango] 後処理を実行...\n")
		ppResult := worktree.RunPostProcess(cfg, sangoDir, wtName, wtInfo.Services, wtInfo.Offset, worktree.PostProcessOptions{
			SkipSetup: wtSyncNoSetup,
			SkipHooks: wtSyncNoHooks,
		})

		if ppResult.IncludeResult != nil {
			for _, w := range ppResult.IncludeResult.Warnings {
				fmt.Fprintf(os.Stderr, "  include警告: %s\n", w)
			}
		}
		for _, e := range ppResult.SetupErrors {
			fmt.Fprintf(os.Stderr, "  setup失敗: %v\n", e)
		}

		if rebaseResult.HasError {
			return fmt.Errorf("一部のサービスでrebaseが失敗しました")
		}
		return nil
	},
}

func init() {
	worktreeSyncCmd.Flags().BoolVar(&wtSyncNoSetup, "no-setup", false, "セットアップをスキップする")
	worktreeSyncCmd.Flags().BoolVar(&wtSyncNoHooks, "no-hooks", false, "フックをスキップする")
	worktreeCmd.AddCommand(worktreeSyncCmd)
}
