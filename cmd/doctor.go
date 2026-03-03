package cmd

import (
	"fmt"
	"os/exec"

	"github.com/charmbracelet/lipgloss"
	"github.com/ryugen04/grove/internal/doctor"
	"github.com/spf13/cobra"
)

var doctorFix bool

var (
	passStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))  // 緑
	failStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))  // 赤
	warnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))  // 黄
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "開発環境の状態をチェックする",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		// DoctorCheckからdoctor.Checkに変換
		checks := make([]doctor.Check, len(cfg.Doctor.Checks))
		for i, c := range cfg.Doctor.Checks {
			checks[i] = doctor.Check{
				Name:    c.Name,
				Command: c.Command,
				Expect:  c.Expect,
				Fix:     c.Fix,
			}
		}

		results := doctor.Run(checks)

		// ヘッダー出力
		fmt.Println("Grove Doctor")
		fmt.Println("============")
		fmt.Println()

		passed := 0
		failed := 0
		warned := 0

		for _, r := range results {
			switch r.Status {
			case doctor.StatusPass:
				fmt.Printf("%s %s - %s\n", passStyle.Render("[✓]"), r.Name, r.Message)
				passed++
			case doctor.StatusFail:
				fmt.Printf("%s %s - %s\n", failStyle.Render("[✗]"), r.Name, r.Message)
				failed++
				if r.Fix != "" {
					fmt.Printf("    Fix: %s\n", r.Fix)
				}
			case doctor.StatusWarn:
				fmt.Printf("%s %s - %s\n", warnStyle.Render("[!]"), r.Name, r.Message)
				warned++
				if r.Fix != "" {
					fmt.Printf("    Fix: %s\n", r.Fix)
				}
			}
		}

		// サマリー出力
		fmt.Println()
		summary := fmt.Sprintf("Results: %d passed", passed)
		if failed > 0 {
			summary += fmt.Sprintf(", %d failed", failed)
		}
		if warned > 0 {
			summary += fmt.Sprintf(", %d warned", warned)
		}
		fmt.Println(summary)

		// --fix オプション: failしたチェックのfixコマンドを実行
		if doctorFix {
			for _, r := range results {
				if r.Status == doctor.StatusFail && r.Fix != "" {
					fmt.Printf("\n[fix] %s: %s\n", r.Name, r.Fix)
					out, err := exec.Command("sh", "-c", r.Fix).CombinedOutput()
					if err != nil {
						fmt.Printf("[fix] 失敗: %s\n", string(out))
					} else {
						fmt.Printf("[fix] 成功: %s\n", string(out))
					}
				}
			}
		}

		return nil
	},
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "失敗したチェックの修復コマンドを実行する")
	rootCmd.AddCommand(doctorCmd)
}
