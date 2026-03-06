package cmd

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/charmbracelet/lipgloss"
	"github.com/ryugen04/sango-tree/internal/process"
	"github.com/ryugen04/sango-tree/internal/worktree"
	"github.com/spf13/cobra"
)

var worktreeStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "全ワークツリーの状態を横断表示する",
	RunE: func(cmd *cobra.Command, args []string) error {
		sangoDir := worktree.DefaultDir()
		ws, err := worktree.Load(sangoDir)
		if err != nil {
			return fmt.Errorf("worktrees.jsonの読み込みに失敗: %w", err)
		}

		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		green := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
		gray := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		bold := lipgloss.NewStyle().Bold(true)

		// worktree名をソート
		wtNames := make([]string, 0, len(ws.Worktrees))
		for name := range ws.Worktrees {
			wtNames = append(wtNames, name)
		}
		sort.Strings(wtNames)

		for _, wtName := range wtNames {
			wt := ws.Worktrees[wtName]
			wtKey := worktree.ToKey(wtName)

			// ヘッダー
			header := wtName
			if wtName == ws.Active {
				header = "* " + wtName + " (active)"
			}
			fmt.Println(bold.Render(header))
			fmt.Printf("  offset: %d, services: %d\n", wt.Offset, len(wt.Services))

			// サービス状態
			serviceNames := make([]string, len(wt.Services))
			copy(serviceNames, wt.Services)
			sort.Strings(serviceNames)

			for _, name := range serviceNames {
				svc := cfg.Services[name]
				if svc == nil {
					continue
				}

				status := "stopped"
				pidStr := "-"

				pidWorktree := wtKey
				if svc.Shared {
					pidWorktree = "shared"
				}

				if pid, err := process.ReadPID(sangoDir, pidWorktree, name); err == nil {
					if process.IsProcessRunning(pid) {
						status = "running"
						pidStr = strconv.Itoa(pid)
					}
				}

				var styledStatus string
				if status == "running" {
					styledStatus = green.Render(status)
				} else {
					styledStatus = gray.Render(status)
				}

				portStr := "-"
				if svc.Port > 0 {
					resolvedPort := svc.Port
					if !svc.Shared {
						resolvedPort += wt.Offset
					}
					portStr = strconv.Itoa(resolvedPort)
				}

				fmt.Printf("  %-15s %-8s %-8s %s\n", name, portStr, styledStatus, pidStr)
			}
			fmt.Println()
		}

		// sharedサービス表示
		if len(ws.SharedServices) > 0 {
			fmt.Println(bold.Render("Shared Services"))
			for name, ss := range ws.SharedServices {
				status := "stopped"
				pidStr := "-"
				if pid, err := process.ReadPID(sangoDir, "shared", name); err == nil {
					if process.IsProcessRunning(pid) {
						status = "running"
						pidStr = strconv.Itoa(pid)
					}
				}

				var styledStatus string
				if status == "running" {
					styledStatus = green.Render(status)
				} else {
					styledStatus = gray.Render(status)
				}

				fmt.Printf("  %-15s %-8d %-8s %s\n", name, ss.Port, styledStatus, pidStr)
			}
		}

		return nil
	},
}

func init() {
	worktreeCmd.AddCommand(worktreeStatusCmd)
}
