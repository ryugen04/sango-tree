package cmd

import (
	"fmt"

	"github.com/ryugen04/sango-tree/internal/config"
	"github.com/ryugen04/sango-tree/internal/service"
	"github.com/spf13/cobra"
)

var restartProfile string

var restartCmd = &cobra.Command{
	Use:   "restart [services...]",
	Short: "サービスを再起動する",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		return runRestart(cfg, args, restartProfile)
	},
}

func init() {
	restartCmd.Flags().StringVar(&restartProfile, "profile", "", "プロファイル名")
	rootCmd.AddCommand(restartCmd)
}

func runRestart(cfg *config.Config, args []string, profile string) error {
	orch, err := service.NewOrchestratorWithWorktree(cfg, cfgFile, worktreeFlag)
	if err != nil {
		return err
	}
	result, err := orch.Restart(args, profile)
	if err != nil {
		return err
	}
	for _, s := range result.Started {
		if s.PID > 0 {
			fmt.Printf("[sango] %s を再起動しました (PID: %d, Port: %d)\n", s.Name, s.PID, s.Port)
		}
	}
	for _, e := range result.Errors {
		fmt.Printf("[sango] エラー: %s\n", e)
	}
	return nil
}
