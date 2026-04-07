package cmd

import (
	"fmt"
	"os"

	"github.com/ryugen04/sango-tree/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	wtRebaseNoSetup bool
	wtRebaseNoHooks bool
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

		fmt.Fprintf(os.Stderr, "[sango] Rebasing worktree %s...\n", wtName)

		// rebaseを実行
		rebaseResult := worktree.RebaseServices(cfg, sangoDir, wtName, wtInfo, baseBranch)

		for _, name := range rebaseResult.Rebased {
			fmt.Fprintf(os.Stderr, "  rebase完了: %s\n", name)
		}
		for _, name := range rebaseResult.Failed {
			fmt.Fprintf(os.Stderr, "  rebase失敗: %s\n", name)
		}

		// rebase成功後に後処理を実行
		if !rebaseResult.HasError {
			fmt.Fprintf(os.Stderr, "[sango] 後処理を実行...\n")
			ppResult := worktree.RunPostProcess(cfg, sangoDir, wtName, wtInfo.Services, wtInfo.Offset, worktree.PostProcessOptions{
				SkipSetup: wtRebaseNoSetup,
				SkipHooks: wtRebaseNoHooks,
			})

			if ppResult.IncludeResult != nil {
				for _, w := range ppResult.IncludeResult.Warnings {
					fmt.Fprintf(os.Stderr, "  include警告: %s\n", w)
				}
			}
			for _, e := range ppResult.SetupErrors {
				fmt.Fprintf(os.Stderr, "  setup失敗: %v\n", e)
			}
		}

		if rebaseResult.HasError {
			return fmt.Errorf("一部のサービスでrebaseが失敗しました")
		}
		return nil
	},
}

func init() {
	worktreeRebaseCmd.Flags().BoolVar(&wtRebaseNoSetup, "no-setup", false, "セットアップをスキップする")
	worktreeRebaseCmd.Flags().BoolVar(&wtRebaseNoHooks, "no-hooks", false, "フックをスキップする")
	worktreeCmd.AddCommand(worktreeRebaseCmd)
}
