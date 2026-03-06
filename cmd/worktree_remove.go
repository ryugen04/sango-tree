package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ryugen04/sango-tree/internal/process"
	"github.com/ryugen04/sango-tree/internal/worktree"
	"github.com/spf13/cobra"
)

var wtRemoveForce bool

var worktreeRemoveCmd = &cobra.Command{
	Use:   "remove <branch>",
	Short: "ワークツリーを削除する",
	Aliases: []string{"rm"},
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		branch := args[0]
		sangoDir := worktree.DefaultDir()

		ws, err := worktree.Load(sangoDir)
		if err != nil {
			return fmt.Errorf("worktrees.jsonの読み込みに失敗: %w", err)
		}

		// アクティブworktreeは削除不可
		if ws.Active == branch {
			return fmt.Errorf("アクティブなワークツリー %q は削除できません。先にswitchしてください", branch)
		}

		wt, exists := ws.Worktrees[branch]
		if !exists {
			return fmt.Errorf("ワークツリー %q は存在しません", branch)
		}

		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		wtKey := worktree.ToKey(branch)
		pm := process.NewManager(sangoDir, wtKey)

		// 実行中サービスのチェック
		var running []string
		for _, name := range wt.Services {
			if pm.IsRunning(name) {
				running = append(running, name)
			}
		}

		if len(running) > 0 && !wtRemoveForce {
			return fmt.Errorf("実行中のサービスがあります: %v\n--force で強制停止・削除できます", running)
		}

		// --force: 実行中サービスを停止
		if len(running) > 0 {
			for _, name := range running {
				fmt.Printf("[sango] %s を強制停止中...\n", name)
				if err := pm.Stop(name); err != nil {
					fmt.Printf("[sango] %s の停止に失敗: %v\n", name, err)
				}
			}
		}

		// pre_removeフック実行
		if len(cfg.Worktree.Hooks.PreRemove) > 0 {
			fmt.Println("[sango] pre_removeフックを実行中...")
			if err := worktree.RunHooks(cfg.Worktree.Hooks.PreRemove, branch, wt.Services); err != nil {
				fmt.Printf("[sango] pre_removeフック警告: %v\n", err)
			}
		}

		// git worktree remove
		for _, name := range wt.Services {
			svc := cfg.Services[name]
			if svc == nil || svc.Type == "docker" || svc.Repo == "" {
				continue
			}

			wtPath := filepath.Join(branch, name)
			fmt.Printf("[sango] %s のワークツリーを削除中...\n", name)
			if err := worktree.WorktreeRemove(sangoDir, name, wtPath, wtRemoveForce); err != nil {
				fmt.Printf("[sango] %s のワークツリー削除に失敗: %v\n", name, err)
			}
		}

		// PIDディレクトリのクリーンアップ
		pidDir := process.PIDDir(sangoDir, wtKey)
		_ = os.RemoveAll(pidDir)

		// worktrees.json更新
		ws.RemoveWorktree(branch)
		if err := ws.Save(sangoDir); err != nil {
			return fmt.Errorf("worktrees.jsonの保存に失敗: %w", err)
		}

		fmt.Printf("[sango] ワークツリー %q を削除しました\n", branch)
		return nil
	},
}

func init() {
	worktreeRemoveCmd.Flags().BoolVar(&wtRemoveForce, "force", false, "実行中サービスを強制停止して削除する")
	worktreeCmd.AddCommand(worktreeRemoveCmd)
}
