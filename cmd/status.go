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

		// サービスを分類
		var shared []service.ServiceInfo
		var servers []service.ServiceInfo
		var repos []string
		for _, svc := range result.Services {
			if svc.IsRepoOnly {
				repos = append(repos, svc.Name)
				continue
			}
			if svc.IsShared {
				shared = append(shared, svc)
				continue
			}
			servers = append(servers, svc)
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

		// --- ワークツリー情報 + サーバー ---
		// ワークツリー一覧（複数ある場合は全表示）
		if len(result.Worktrees) > 1 {
			fmt.Println(bold.Render("Worktrees"))
			// ソート
			sort.Slice(result.Worktrees, func(i, j int) bool {
				return result.Worktrees[i].Name < result.Worktrees[j].Name
			})
			for _, wt := range result.Worktrees {
				marker := "  "
				display := wt.Name
				if wt.IsActive {
					marker = "* "
					display = green.Render(wt.Name)
				}
				fmt.Printf("%s%s  %s\n", marker, display, dim.Render(fmt.Sprintf("offset:%d", wt.Offset)))
			}
			fmt.Println()
		}

		// サーバー一覧
		wtLabel := result.Worktree
		for _, wt := range result.Worktrees {
			if wt.IsActive && wt.Offset > 0 {
				wtLabel = fmt.Sprintf("%s, offset:%d", wt.Name, wt.Offset)
			}
		}
		fmt.Println(bold.Render(fmt.Sprintf("Services (worktree: %s)", wtLabel)))

		rows := make([][]string, 0, len(servers))
		for _, svc := range servers {
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

		// repo-onlyサービス
		if len(repos) > 0 {
			repoList := ""
			for i, name := range repos {
				if i > 0 {
					repoList += ", "
				}
				repoList += name
			}
			fmt.Println(dim.Render("repos: " + repoList))
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
