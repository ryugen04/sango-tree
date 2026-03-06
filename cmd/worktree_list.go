package cmd

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/lipgloss"
	"github.com/ryugen04/sango-tree/internal/worktree"
	"github.com/spf13/cobra"
)

var worktreeListCmd = &cobra.Command{
	Use:   "list",
	Short: "ワークツリー一覧を表示する",
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

		// ソートして表示
		names := make([]string, 0, len(ws.Worktrees))
		for name := range ws.Worktrees {
			names = append(names, name)
		}
		sort.Strings(names)

		fmt.Printf("%-30s%-10s%-12s%s\n", "WORKTREE", "OFFSET", "SERVICES", "CREATED")

		for _, name := range names {
			wt := ws.Worktrees[name]
			display := name
			if name == ws.Active {
				display = bold.Render("* " + green.Render(name))
			}

			// lipglossのパディング補正
			padding := 30 + len(display) - len(name)
			if name == ws.Active {
				padding += 2 // "* " の分
			}

			fmtStr := fmt.Sprintf("%%-%ds%%-%ds%%-%ds%%s\n", padding, 10, 12)
			fmt.Printf(fmtStr,
				display,
				fmt.Sprintf("%d", wt.Offset),
				fmt.Sprintf("%d", len(wt.Services)),
				wt.CreatedAt.Format("2006-01-02"),
			)
		}

		return nil
	},
}

func init() {
	worktreeCmd.AddCommand(worktreeListCmd)
}
