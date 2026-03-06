package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/ryugen04/sango-tree/internal/worktree"
	"github.com/spf13/cobra"
)

var worktreeVerifyCmd = &cobra.Command{
	Use:   "verify [branch]",
	Short: "ワークツリーのinclude状態を検証する",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		sangoDir := worktree.DefaultDir()
		ws, err := worktree.Load(sangoDir)
		if err != nil {
			return fmt.Errorf("worktrees.jsonの読み込みに失敗: %w", err)
		}

		// ブランチ名の解決
		var branch string
		if len(args) > 0 {
			branch = args[0]
		} else {
			branch = ws.Active
			if branch == "" {
				return fmt.Errorf("アクティブなワークツリーがありません。ブランチ名を指定してください")
			}
		}

		wt, exists := ws.Worktrees[branch]
		if !exists {
			return fmt.Errorf("ワークツリー %q が見つかりません", branch)
		}

		// include設定がなければスキップ
		if len(cfg.Worktree.Include.Root) == 0 && len(cfg.Worktree.Include.PerService) == 0 {
			fmt.Println("[sango] include設定がありません")
			return nil
		}

		// 変数マップを構築
		vars := buildIncludeVars(cfg, wt.Offset)

		// 検証実行
		results := worktree.VerifyIncludes(branch, wt.Services, cfg.Worktree.Include, vars)

		return printVerifyResults(branch, results)
	},
}

func init() {
	worktreeCmd.AddCommand(worktreeVerifyCmd)
}

func printVerifyResults(branch string, results []worktree.VerifyEntry) error {
	fmt.Printf("[sango] include検証: %s\n", branch)

	// サービスごとにグループ化
	grouped := make(map[string][]worktree.VerifyEntry)
	for _, r := range results {
		grouped[r.Service] = append(grouped[r.Service], r)
	}

	// ソートしたキーで出力
	keys := make([]string, 0, len(grouped))
	for k := range grouped {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	okCount := 0
	failCount := 0
	hasRequiredFailure := false

	for _, svc := range keys {
		entries := grouped[svc]
		label := svc
		if label == "" {
			label = "(root)"
		}
		fmt.Printf("  %s:\n", label)

		for _, e := range entries {
			statusStr := string(e.Status)
			detail := ""
			if e.Entry.Required && e.Status != worktree.VerifyOK {
				detail = " (required)"
				hasRequiredFailure = true
			}
			if e.Status == worktree.VerifyOK {
				okCount++
			} else {
				failCount++
			}

			fmt.Printf("    %-20s %-10s %s%s\n", e.Entry.Target, e.Entry.Strategy, statusStr, detail)
		}
	}

	total := okCount + failCount
	fmt.Printf("\n結果: %d/%d OK", okCount, total)
	if failCount > 0 {
		fmt.Printf(", %d 失敗", failCount)
	}
	fmt.Println()

	if hasRequiredFailure {
		os.Exit(1)
	}
	return nil
}
