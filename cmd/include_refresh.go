package cmd

import (
	"fmt"
	"os"

	"github.com/ryugen04/sango-tree/internal/service"
	"github.com/ryugen04/sango-tree/internal/worktree"
	"github.com/spf13/cobra"
)

var includeRefreshWorktree string

var includeRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "includeテンプレートを再展開する",
	Long:  "指定ワークツリーのincludeテンプレートをsango.yamlの定義に基づいて再展開する",
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

		wtName := includeRefreshWorktree
		if wtName == "" {
			wtName = service.ResolveActiveWorktree(sangoDir, worktreeFlag)
		}

		wtInfo, ok := ws.Worktrees[wtName]
		if !ok {
			return fmt.Errorf("ワークツリー %q が見つかりません", wtName)
		}

		fmt.Fprintf(os.Stderr, "[sango] %s: includeテンプレートを再展開中...\n", wtName)

		result := worktree.RunPostProcess(cfg, sangoDir, wtName, wtInfo.Services, wtInfo.Offset, worktree.PostProcessOptions{
			SkipSetup: true,
			SkipHooks: true,
		})

		if result.IncludeResult != nil {
			for _, w := range result.IncludeResult.Warnings {
				fmt.Fprintf(os.Stderr, "  警告: %s\n", w)
			}
			if result.IncludeResult.HasErrors() {
				for _, e := range result.IncludeResult.Errors {
					fmt.Fprintf(os.Stderr, "  エラー: %s\n", e)
				}
				return fmt.Errorf("include展開でエラーが発生しました")
			}
		}

		fmt.Fprintf(os.Stderr, "[sango] includeテンプレートの再展開が完了しました\n")
		return nil
	},
}

func init() {
	includeRefreshCmd.Flags().StringVar(&includeRefreshWorktree, "worktree", "", "ワークツリー名（省略時はアクティブ）")
	includeCmd.AddCommand(includeRefreshCmd)
}
