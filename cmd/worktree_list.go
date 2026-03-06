package cmd

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/ryugen04/sango-tree/internal/worktree"
	"github.com/spf13/cobra"
)

var worktreeListCmd = &cobra.Command{
	Use:     "list",
	Short:   "ワークツリー一覧を表示する",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		sangoDir := worktree.DefaultDir()
		ws, err := worktree.Load(sangoDir)
		if err != nil {
			return fmt.Errorf("worktrees.jsonの読み込みに失敗: %w", err)
		}

		if len(ws.Worktrees) == 0 {
			fmt.Println("ワークツリーがありません。sango clone を実行してください。")
			return nil
		}

		green := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
		bold := lipgloss.NewStyle().Bold(true)
		dim := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

		headerStyle := lipgloss.NewStyle().Bold(true).Padding(0, 1)
		cellStyle := lipgloss.NewStyle().Padding(0, 1)

		rows := make([][]string, 0, len(ws.Worktrees))
		for name, wt := range ws.Worktrees {
			display := name
			if name == ws.Active {
				display = bold.Render("* " + green.Render(name))
			}

			rows = append(rows, []string{
				display,
				fmt.Sprintf("%d", wt.Offset),
				fmt.Sprintf("%d", len(wt.Services)),
				wt.CreatedAt.Format("2006-01-02"),
			})
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
			Headers("WORKTREE", "OFFSET", "SERVICES", "CREATED").
			Rows(rows...)

		fmt.Println(t)

		return nil
	},
}

func init() {
	worktreeCmd.AddCommand(worktreeListCmd)
}
