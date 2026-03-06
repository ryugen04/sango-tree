package cmd

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
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
		dim := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

		headerStyle := lipgloss.NewStyle().Bold(true).Padding(0, 1)
		cellStyle := lipgloss.NewStyle().Padding(0, 1)

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
				header = "* " + green.Render(wtName) + " (active)"
			}
			fmt.Printf("%s  %s\n", bold.Render(header), dim.Render(fmt.Sprintf("offset:%d", wt.Offset)))

			// サーバーサービスのみ抽出
			serviceNames := make([]string, 0)
			for _, name := range wt.Services {
				svc := cfg.Services[name]
				if svc == nil {
					continue
				}
				// repo-onlyサービスをスキップ
				if svc.Repo != "" && svc.Command == "" {
					continue
				}
				serviceNames = append(serviceNames, name)
			}
			sort.Strings(serviceNames)

			if len(serviceNames) == 0 {
				fmt.Println()
				continue
			}

			rows := make([][]string, 0, len(serviceNames))
			for _, name := range serviceNames {
				svc := cfg.Services[name]

				status := "stopped"
				pidStr := dim.Render("-")

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

				portStr := dim.Render("-")
				if svc.Port > 0 {
					resolvedPort := svc.Port
					if !svc.Shared {
						resolvedPort += wt.Offset
					}
					portStr = strconv.Itoa(resolvedPort)
				}

				rows = append(rows, []string{name, portStr, styledStatus, pidStr})
			}

			t := table.New().
				Border(lipgloss.NormalBorder()).
				BorderStyle(dim).
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
				Headers("SERVICE", "PORT", "STATUS", "PID").
				Rows(rows...)

			fmt.Println(t)
			fmt.Println()
		}

		// sharedサービス表示
		if len(ws.SharedServices) > 0 {
			fmt.Println(bold.Render("Shared Services"))
			rows := make([][]string, 0, len(ws.SharedServices))
			for name, ss := range ws.SharedServices {
				status := "stopped"
				pidStr := dim.Render("-")
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

				rows = append(rows, []string{name, strconv.Itoa(ss.Port), styledStatus, pidStr})
			}

			t := table.New().
				Border(lipgloss.NormalBorder()).
				BorderStyle(dim).
				BorderRow(false).
				BorderColumn(false).
				BorderLeft(false).
				BorderRight(false).
				BorderTop(false).
				BorderBottom(false).
				BorderHeader(true).
				StyleFunc(func(row, col int) lipgloss.Style {
					if row == table.HeaderRow {
						return lipgloss.NewStyle().Bold(true).Padding(0, 1)
					}
					return lipgloss.NewStyle().Padding(0, 1)
				}).
				Headers("SERVICE", "PORT", "STATUS", "PID").
				Rows(rows...)

			fmt.Println(t)
		}

		return nil
	},
}

func init() {
	worktreeCmd.AddCommand(worktreeStatusCmd)
}
