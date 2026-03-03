package cmd

import (
	"fmt"

	"github.com/ryugen04/grove/internal/process"
	"github.com/ryugen04/grove/internal/worktree"
	"github.com/spf13/cobra"
)

var restartProfile string

var restartCmd = &cobra.Command{
	Use:   "restart [services...]",
	Short: "サービスを再起動する",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		groveDir := worktree.DefaultDir()
		wtName := resolveActiveWorktree(groveDir)
		wtKey := worktree.ToKey(wtName)
		pm := process.NewManager(groveDir, wtKey)

		d := buildDAG(cfg)

		// 対象サービスを決定
		targets := resolveTargets(cfg, args, restartProfile)

		// --- 停止フェーズ（逆順） ---
		var stopOrder []string
		if len(targets) > 0 {
			// 対象サービスのみ逆順で停止
			resolved, err := d.Resolve(targets...)
			if err != nil {
				return fmt.Errorf("依存解決に失敗: %w", err)
			}
			// resolvedを逆順にする
			for i := len(resolved) - 1; i >= 0; i-- {
				stopOrder = append(stopOrder, resolved[i])
			}
		} else {
			stopOrder, err = d.Reverse()
			if err != nil {
				return fmt.Errorf("依存解決に失敗: %w", err)
			}
		}

		for _, name := range stopOrder {
			if !pm.IsRunning(name) {
				continue
			}
			fmt.Printf("[grove] %s を停止中...\n", name)
			if err := pm.Stop(name); err != nil {
				return fmt.Errorf("サービス %s の停止に失敗: %w", name, err)
			}
			fmt.Printf("[grove] %s を停止しました\n", name)
		}

		// --- 起動フェーズ ---
		return runUp(cfg, args, restartProfile)
	},
}

func init() {
	restartCmd.Flags().StringVar(&restartProfile, "profile", "", "プロファイル名")
	rootCmd.AddCommand(restartCmd)
}
