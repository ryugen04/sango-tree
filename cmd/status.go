package cmd

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
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
		yellow := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
		bold := lipgloss.NewStyle().Bold(true)
		dim := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

		headerStyle := lipgloss.NewStyle().Bold(true).Padding(0, 1)
		cellStyle := lipgloss.NewStyle().Padding(0, 1)

		// サービスを分類（shared判定用）
		var shared []service.ServiceInfo
		for _, svc := range result.Services {
			if svc.IsShared {
				shared = append(shared, svc)
			}
		}

		// --- Shared Services ---
		if len(shared) > 0 {
			fmt.Println(bold.Render("Shared Services"))
			rows := make([][]string, 0, len(shared))
			for _, svc := range shared {
				portStr := dim.Render("-")
				if svc.Port > 0 {
					portStr = strconv.Itoa(svc.Port)
				}
				rows = append(rows, []string{svc.Name, portStr, renderStatus(svc.Status, green, gray, yellow)})
			}
			fmt.Println(renderTable([]string{"SERVICE", "PORT", "STATUS"}, rows, headerStyle, cellStyle, dim))
			fmt.Println()
		}

		// --- ワークツリー一覧 ---
		if len(result.Worktrees) > 0 {
			fmt.Println(bold.Render("Worktrees"))
			sort.Slice(result.Worktrees, func(i, j int) bool {
				return result.Worktrees[i].Name < result.Worktrees[j].Name
			})
			wtRows := make([][]string, 0, len(result.Worktrees))
			for _, wt := range result.Worktrees {
				svcStatus := dim.Render(fmt.Sprintf("0/%d", wt.TotalServices))
				if wt.RunningServices > 0 {
					svcStatus = green.Render(fmt.Sprintf("%d/%d", wt.RunningServices, wt.TotalServices))
				}
				offsetStr := dim.Render(fmt.Sprintf("%d", wt.Offset))
				webStr := dim.Render("-")
				if wt.WebPort > 0 {
					webStr = strconv.Itoa(wt.WebPort)
				}
				wtRows = append(wtRows, []string{wt.Name, offsetStr, webStr, svcStatus})
			}
			fmt.Println(renderTable([]string{"WORKTREE", "OFFSET", "WEB", "SERVICES"}, wtRows, headerStyle, cellStyle, dim))
			fmt.Println()
		}

		// --- 起動中worktreeのサービス詳細 ---
		sort.Slice(result.Worktrees, func(i, j int) bool {
			return result.Worktrees[i].Name < result.Worktrees[j].Name
		})
		for _, wt := range result.Worktrees {
			if wt.RunningServices == 0 {
				continue
			}
			fmt.Println(bold.Render(fmt.Sprintf("Services (%s)", wt.Name)))
			rows := make([][]string, 0, len(wt.Services))
			for _, svc := range wt.Services {
				portStr := dim.Render("-")
				if svc.Port > 0 {
					portStr = strconv.Itoa(svc.Port)
				}
				healthStr := dim.Render("-")
				if svc.Health != "" {
					switch svc.Health {
					case "healthy":
						healthStr = green.Render(svc.Health)
					default:
						healthStr = yellow.Render(svc.Health)
					}
				}
				pidStr := dim.Render("-")
				if svc.PID > 0 {
					pidStr = strconv.Itoa(svc.PID)
				}
				rows = append(rows, []string{
					svc.Name,
					portStr,
					renderStatus(svc.Status, green, gray, yellow),
					healthStr,
					pidStr,
				})
			}
			fmt.Println(renderTable([]string{"SERVICE", "PORT", "STATUS", "HEALTH", "PID"}, rows, headerStyle, cellStyle, dim))
			fmt.Println()
		}

		return nil
	},
}

func renderStatus(status string, green, gray, yellow lipgloss.Style) string {
	switch status {
	case "running":
		return green.Render(status)
	case "failed":
		return yellow.Render(status)
	default:
		return gray.Render(status)
	}
}

func renderTable(headers []string, rows [][]string, headerStyle, cellStyle lipgloss.Style, borderStyle lipgloss.Style) string {
	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(borderStyle).
		BorderRow(false).
		BorderColumn(false).
		BorderLeft(false).
		BorderRight(false).
		BorderTop(false).
		BorderBottom(false).
		BorderHeader(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		}).
		Headers(headers...).
		Rows(rows...)

	return t.String()
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
