package cmd

import (
	"fmt"

	"github.com/ryugen04/sango-tree/internal/service"
	"github.com/spf13/cobra"
)

var downAll bool

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "サービスを停止する",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		orch, err := service.NewOrchestratorWithWorktree(cfg, cfgFile, worktreeFlag)
		if err != nil {
			return err
		}
		result, err := orch.Down(args, downAll)
		if err != nil {
			return err
		}
		for _, name := range result.Stopped {
			fmt.Printf("[sango] %s を停止しました\n", name)
		}
		for _, e := range result.Errors {
			fmt.Printf("[sango] エラー: %s\n", e)
		}
		return nil
	},
}

func init() {
	downCmd.Flags().BoolVar(&downAll, "all", false, "sharedサービスも含めて全て停止する")
	rootCmd.AddCommand(downCmd)
}
