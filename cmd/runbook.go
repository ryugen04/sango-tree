package cmd

import (
	"fmt"
	"strings"

	"github.com/ryugen04/sango-tree/internal/runbook"
	"github.com/spf13/cobra"
)

var runbookServiceFilter string

var runbookCmd = &cobra.Command{
	Use:   "runbook",
	Short: "サービスのRunbookを検索・一覧表示する",
}

var runbookSearchCmd = &cobra.Command{
	Use:   "search <keyword>",
	Short: "キーワードでRunbookを検索する",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		keyword := args[0]
		results := runbook.Search(cfg.Services, keyword)

		fmt.Printf("[sango] runbook検索: %q\n\n", keyword)

		if len(results) == 0 {
			fmt.Println("該当するエントリが見つかりませんでした")
			return nil
		}

		for _, r := range results {
			fmt.Printf("  [%s] %s\n", r.ServiceName, r.Entry.Title)
			if len(r.Entry.Symptoms) > 0 {
				fmt.Printf("    症状: %s\n", strings.Join(r.Entry.Symptoms, ", "))
			}
			if r.Entry.Cause != "" {
				fmt.Printf("    原因: %s\n", r.Entry.Cause)
			}
			if len(r.Entry.Steps) > 0 {
				fmt.Println("    手順:")
				for i, step := range r.Entry.Steps {
					fmt.Printf("      %d. %s\n", i+1, step)
				}
			}
			if len(r.Entry.Tags) > 0 {
				fmt.Printf("    タグ: %s\n", strings.Join(r.Entry.Tags, ", "))
			}
			fmt.Println()
		}

		fmt.Printf("%d件見つかりました\n", len(results))
		return nil
	},
}

var runbookListCmd = &cobra.Command{
	Use:   "list",
	Short: "Runbookを一覧表示する",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		fmt.Println("[sango] runbook一覧")
		fmt.Println()

		found := false
		for name, svc := range cfg.Services {
			if runbookServiceFilter != "" && name != runbookServiceFilter {
				continue
			}
			if len(svc.Runbook) == 0 {
				continue
			}
			found = true
			fmt.Printf("  %s:\n", name)
			for _, entry := range svc.Runbook {
				tagStr := ""
				if len(entry.Tags) > 0 {
					tagStr = fmt.Sprintf(" [%s]", strings.Join(entry.Tags, ", "))
				}
				fmt.Printf("    - %s%s\n", entry.Title, tagStr)
			}
		}

		if !found {
			fmt.Println("  Runbookが定義されているサービスがありません")
		}

		return nil
	},
}

func init() {
	runbookListCmd.Flags().StringVar(&runbookServiceFilter, "service", "", "サービス名で絞り込み")
	runbookCmd.AddCommand(runbookSearchCmd)
	runbookCmd.AddCommand(runbookListCmd)
	rootCmd.AddCommand(runbookCmd)
}
