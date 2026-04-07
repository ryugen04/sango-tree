package cmd

import "github.com/spf13/cobra"

var includeCmd = &cobra.Command{
	Use:   "include",
	Short: "includeファイル管理",
	Long:  "テンプレートファイルの展開・更新等を管理するコマンド",
}

func init() {
	rootCmd.AddCommand(includeCmd)
}
