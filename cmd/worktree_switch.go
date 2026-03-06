package cmd

import (
	"fmt"

	"github.com/ryugen04/sango-tree/internal/process"
	"github.com/ryugen04/sango-tree/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	wtSwitchStopCurrent bool
	wtSwitchStart       bool
)

var worktreeSwitchCmd = &cobra.Command{
	Use:   "switch <branch>",
	Short: "アクティブワークツリーを切り替える",
	Long:  "デフォルトではactiveの切り替えのみ。--stop-currentで旧worktreeの停止、--startで新worktreeの起動も行う",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		branch := args[0]
		sangoDir := worktree.DefaultDir()

		ws, err := worktree.Load(sangoDir)
		if err != nil {
			return fmt.Errorf("worktrees.jsonの読み込みに失敗: %w", err)
		}

		// 対象worktreeの存在チェック
		if _, exists := ws.Worktrees[branch]; !exists {
			return fmt.Errorf("ワークツリー %q は存在しません", branch)
		}

		if ws.Active == branch {
			fmt.Printf("[sango] 既にワークツリー %q がアクティブです\n", branch)
			return nil
		}

		oldActive := ws.Active

		// --stop-current: 旧worktreeのサービスを停止
		if wtSwitchStopCurrent && oldActive != "" {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			oldKey := worktree.ToKey(oldActive)
			pm := process.NewManager(sangoDir, oldKey)

			d := buildDAG(cfg)
			order, err := d.Reverse()
			if err != nil {
				return fmt.Errorf("依存解決に失敗: %w", err)
			}

			for _, name := range order {
				svc := cfg.Services[name]
				if svc.Shared {
					continue
				}
				if !pm.IsRunning(name) {
					continue
				}
				fmt.Printf("[sango] %s を停止中... (worktree: %s)\n", name, oldActive)
				if err := pm.Stop(name); err != nil {
					fmt.Printf("[sango] %s の停止に失敗: %v\n", name, err)
				}
			}
		}

		// active切り替え
		ws.SetActive(branch)
		if err := ws.Save(sangoDir); err != nil {
			return fmt.Errorf("worktrees.jsonの保存に失敗: %w", err)
		}

		fmt.Printf("[sango] アクティブワークツリーを %q に切り替えました\n", branch)

		// --start: 新worktreeのサービスを起動
		if wtSwitchStart {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			fmt.Printf("[sango] %s のサービスを起動中...\n", branch)
			return runUp(cfg, nil, "")
		}

		return nil
	},
}

func init() {
	worktreeSwitchCmd.Flags().BoolVar(&wtSwitchStopCurrent, "stop-current", false, "旧ワークツリーのサービスを停止する")
	worktreeSwitchCmd.Flags().BoolVar(&wtSwitchStart, "start", false, "新ワークツリーのサービスを起動する")
	worktreeCmd.AddCommand(worktreeSwitchCmd)
}
