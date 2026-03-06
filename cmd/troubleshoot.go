package cmd

import (
	"fmt"
	"os/exec"

	"github.com/ryugen04/sango-tree/internal/troubleshoot"
	"github.com/spf13/cobra"
)

var troubleshootFix bool

var troubleshootCmd = &cobra.Command{
	Use:   "troubleshoot [service]",
	Short: "サービスのトラブルシュートチェックを実行する",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		type target struct {
			name    string
			results []troubleshoot.CheckResult
		}

		var targets []target

		if len(args) > 0 {
			// 指定サービスのみ
			svcName := args[0]
			svc, ok := cfg.Services[svcName]
			if !ok {
				return fmt.Errorf("サービス %q が見つかりません", svcName)
			}
			if len(svc.Troubleshoot) == 0 {
				fmt.Printf("[sango] %s にトラブルシュートチェックが定義されていません\n", svcName)
				return nil
			}
			results := troubleshoot.Run(svc.Troubleshoot)
			targets = append(targets, target{name: svcName, results: results})
		} else {
			// 全サービス
			for name, svc := range cfg.Services {
				if len(svc.Troubleshoot) == 0 {
					continue
				}
				results := troubleshoot.Run(svc.Troubleshoot)
				targets = append(targets, target{name: name, results: results})
			}
		}

		if len(targets) == 0 {
			fmt.Println("[sango] トラブルシュートチェックが定義されているサービスがありません")
			return nil
		}

		totalPass := 0
		totalFail := 0

		for _, t := range targets {
			fmt.Printf("[sango] %s のトラブルシュート実行中...\n", t.name)
			for _, r := range t.results {
				switch r.Status {
				case troubleshoot.StatusPass:
					fmt.Printf("  %s %s - %s\n", passStyle.Render("[pass]"), r.Name, r.Output)
					totalPass++
				case troubleshoot.StatusFail:
					fmt.Printf("  %s %s - %s\n", failStyle.Render("[fail]"), r.Name, r.Output)
					totalFail++
					if r.Fix != "" {
						fmt.Printf("    修復: %s\n", r.Fix)
					}
				}
			}
			fmt.Println()
		}

		fmt.Printf("結果: %d passed, %d failed\n", totalPass, totalFail)

		if troubleshootFix {
			for _, t := range targets {
				for _, r := range t.results {
					if r.Status == troubleshoot.StatusFail && r.Fix != "" {
						fmt.Printf("\n[fix] %s/%s: %s\n", t.name, r.Name, r.Fix)
						out, err := exec.Command("sh", "-c", r.Fix).CombinedOutput()
						if err != nil {
							fmt.Printf("[fix] 失敗: %s\n", string(out))
						} else {
							fmt.Printf("[fix] 成功: %s\n", string(out))
						}
					}
				}
			}
		}

		if totalFail > 0 {
			return fmt.Errorf("%d件のチェックが失敗しました", totalFail)
		}

		return nil
	},
}

func init() {
	troubleshootCmd.Flags().BoolVar(&troubleshootFix, "fix", false, "失敗したチェックの修復コマンドを実行する")
	rootCmd.AddCommand(troubleshootCmd)
}
