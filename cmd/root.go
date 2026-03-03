package cmd

import (
	"github.com/ryugen04/grove/internal/worktree"
	"github.com/spf13/cobra"
)

var cfgFile string
var worktreeFlag string

var rootCmd = &cobra.Command{
	Use:   "grove",
	Short: "ポリレポ開発オーケストレーター",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "grove.yaml", "設定ファイルパス")
	rootCmd.PersistentFlags().StringVar(&worktreeFlag, "worktree", "", "ワークツリー名（省略時はアクティブ）")
}

// Execute はルートコマンドを実行する
func Execute() error {
	return rootCmd.Execute()
}

// resolveActiveWorktree は使用するworktree名を解決する
// --worktreeフラグ指定時はそれを使い、未指定時はworktrees.jsonのactiveを返す
// worktrees.jsonが存在しない場合は"main"を返す
func resolveActiveWorktree(groveDir string) string {
	if worktreeFlag != "" {
		return worktreeFlag
	}

	ws, err := worktree.Load(groveDir)
	if err != nil {
		return "main"
	}
	if ws.Active == "" {
		return "main"
	}
	return ws.Active
}
