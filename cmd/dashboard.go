package cmd

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ryugen04/sango-tree/internal/tui"
	"github.com/spf13/cobra"
)

var dashboardCmd = &cobra.Command{
	Use:     "dashboard",
	Aliases: []string{"dash", "ui"},
	Short:   "インタラクティブなダッシュボードを起動する",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		model := tui.New(cfg, cfgFile)
		p := tea.NewProgram(model, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("ダッシュボードの起動に失敗: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(dashboardCmd)
}
