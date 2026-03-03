package cmd

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/charmbracelet/lipgloss"
	"github.com/ryugen04/grove/internal/process"
	"github.com/ryugen04/grove/internal/worktree"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "サービスの状態を表示する",
	RunE: func(cmd *cobra.Command, args []string) error {
		groveDir := worktree.DefaultDir()
		wtName := resolveActiveWorktree(groveDir)
		wtKey := worktree.ToKey(wtName)

		// 設定ファイルを読み込んでサービスタイプを取得
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		// worktrees.jsonからオフセットを取得
		ws, err := worktree.Load(groveDir)
		if err != nil {
			return fmt.Errorf("worktrees.jsonの読み込みに失敗: %w", err)
		}
		offset := 0
		if wt, ok := ws.Worktrees[wtName]; ok {
			offset = wt.Offset
		}

		// スタイル定義
		green := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
		red := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
		gray := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

		// ヘッダー出力
		fmt.Printf("Grove Status (worktree: %s)\n", wtName)
		fmt.Println("============")
		fmt.Println()
		fmt.Printf("%-13s%-10s%-8s%-11s%s\n", "SERVICE", "TYPE", "PORT", "STATUS", "PID")

		// サービス名をソートして安定した出力順にする
		names := make([]string, 0, len(cfg.Services))
		for name := range cfg.Services {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			svc := cfg.Services[name]
			svcType := svc.Type

			// ポート表示（オフセット適用）
			portStr := "-"
			if svc.Port > 0 {
				resolvedPort := svc.Port
				if !svc.Shared {
					resolvedPort += offset
				}
				portStr = strconv.Itoa(resolvedPort)
			}

			// PIDファイルベースの状態判定
			status := "stopped"
			pidStr := "-"

			// sharedサービスはsharedディレクトリからPIDを読む
			pidWorktree := wtKey
			if svc.Shared {
				pidWorktree = "shared"
			}

			if pid, err := process.ReadPID(groveDir, pidWorktree, name); err == nil {
				if process.IsProcessRunning(pid) {
					status = "running"
					pidStr = strconv.Itoa(pid)
				}
			}

			// ステータスに色を付ける
			var styledStatus string
			switch status {
			case "running":
				styledStatus = green.Render(status)
			default:
				styledStatus = gray.Render(status)
			}
			_ = red // errorステータス用に保持

			// lipglossのANSIエスケープシーケンス分のパディングを補正
			statusPadding := 11 + len(styledStatus) - len(status)
			fmtStr := fmt.Sprintf("%%-%ds%%-%ds%%-%ds%%-%ds%%s\n", 13, 10, 8, statusPadding)
			fmt.Printf(fmtStr, name, svcType, portStr, styledStatus, pidStr)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
