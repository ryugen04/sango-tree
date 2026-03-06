package cmd

import (
	"github.com/ryugen04/sango-tree/internal/config"
	"github.com/ryugen04/sango-tree/internal/dag"
	"github.com/ryugen04/sango-tree/internal/service"
	"github.com/spf13/cobra"
)

var cfgFile string
var worktreeFlag string

var rootCmd = &cobra.Command{
	Use:   "sango",
	Short: "ポリレポ開発オーケストレーター",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "sango.yaml", "設定ファイルパス")
	rootCmd.PersistentFlags().StringVar(&worktreeFlag, "worktree", "", "ワークツリー名（省略時はアクティブ）")
}

// Execute はルートコマンドを実行する
func Execute() error {
	return rootCmd.Execute()
}

// loadConfig は設定ファイルの読み込み・検証・変数展開をまとめて行う
func loadConfig() (*config.Config, error) {
	return service.LoadAndValidateConfig(cfgFile)
}

// resolveActiveWorktree は使用するworktree名を解決する
func resolveActiveWorktree(sangoDir string) string {
	return service.ResolveActiveWorktree(sangoDir, worktreeFlag)
}

// buildDAG は設定からDAGを構築する
func buildDAG(cfg *config.Config) *dag.DAG {
	return service.BuildDAG(cfg)
}
