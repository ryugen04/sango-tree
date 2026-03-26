package cmd

import (
	"fmt"

	"github.com/ryugen04/sango-tree/internal/config"
	"github.com/ryugen04/sango-tree/internal/service"
	"github.com/spf13/cobra"
)

var (
	upProfile    string
	defaultPorts bool
)

var upCmd = &cobra.Command{
	Use:   "up [services...]",
	Short: "サービスを起動する",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		return runUp(cfg, args, upProfile)
	},
}

func init() {
	upCmd.Flags().StringVar(&upProfile, "profile", "", "プロファイル名")
	upCmd.Flags().BoolVar(&defaultPorts, "default-ports", false, "デフォルトポート（offset=0）で起動")
	rootCmd.AddCommand(upCmd)
}

// runUp はサービスを起動するコアロジック
func runUp(cfg *config.Config, args []string, profile string) error {
	orch, err := service.NewOrchestratorWithWorktree(cfg, cfgFile, service.OrchestratorOptions{
		WorktreeFlag: worktreeFlag,
		DefaultPorts: defaultPorts,
	})
	if err != nil {
		return err
	}
	result, err := orch.Up(args, profile)
	if err != nil {
		return err
	}
	for _, s := range result.Started {
		switch s.Status {
		case "already_running":
			fmt.Printf("[sango] %s は既に起動中 (shared, port: %d)\n", s.Name, s.Port)
		case "completed":
			fmt.Printf("[sango] %s を実行しました\n", s.Name)
		default:
			if s.PID > 0 {
				fmt.Printf("[sango] %s を起動しました (PID: %d, Port: %d)\n", s.Name, s.PID, s.Port)
			}
		}
		if s.OpenURL != "" {
			fmt.Printf("[sango] URL: %s\n", s.OpenURL)
		}
	}
	for _, e := range result.Errors {
		fmt.Printf("[sango] エラー: %s\n", e)
	}
	if len(result.Errors) > 0 {
		return fmt.Errorf("一部サービスの起動に失敗しました")
	}
	return nil
}
