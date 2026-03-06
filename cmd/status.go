package cmd

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/lipgloss"
	"github.com/ryugen04/sango-tree/internal/service"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "サービスの状態を表示する",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		orch, err := service.NewOrchestratorWithWorktree(cfg, cfgFile, worktreeFlag)
		if err != nil {
			return err
		}

		result, err := orch.Status()
		if err != nil {
			return err
		}

		green := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
		gray := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

		fmt.Printf("Sango Status (worktree: %s)\n", result.Worktree)
		fmt.Println("============")
		fmt.Println()
		fmt.Printf("%-13s%-10s%-8s%-11s%-11s%-11s%s\n", "SERVICE", "TYPE", "PORT", "STATUS", "HEALTH", "RESTARTS", "PID")

		for _, svc := range result.Services {
			portStr := "-"
			if svc.Port > 0 {
				portStr = strconv.Itoa(svc.Port)
			}

			var styledStatus string
			switch svc.Status {
			case "running":
				styledStatus = green.Render(svc.Status)
			default:
				styledStatus = gray.Render(svc.Status)
			}
			healthStr := "-"
			if svc.Health != "" {
				healthStr = svc.Health
			}

			restartsStr := "-"
			if svc.RestartCount > 0 || svc.Status == "running" {
				restartsStr = strconv.Itoa(svc.RestartCount)
			}

			pidStr := "-"
			if svc.PID > 0 {
				pidStr = strconv.Itoa(svc.PID)
			}

			statusPadding := 11 + len(styledStatus) - len(svc.Status)
			fmtStr := fmt.Sprintf("%%-%ds%%-%ds%%-%ds%%-%ds%%-%ds%%-%ds%%s\n", 13, 10, 8, statusPadding, 11, 11)
			fmt.Printf(fmtStr, svc.Name, svc.Type, portStr, styledStatus, healthStr, restartsStr, pidStr)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
