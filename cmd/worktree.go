package cmd

import "github.com/spf13/cobra"

var worktreeCmd = &cobra.Command{
	Use:   "worktree",
	Short: "ワークツリー管理",
	Long:  "ワークツリーの作成・切り替え・削除等を行うサブコマンド群",
}

func init() {
	rootCmd.AddCommand(worktreeCmd)
}
