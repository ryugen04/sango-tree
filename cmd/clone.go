package cmd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/ryugen04/grove/internal/worktree"
	"github.com/spf13/cobra"
)

var cloneShallow bool

var cloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "リポジトリをクローンしてworktreeを初期化する",
	Long:  "grove.yamlのrepoフィールドを持つ各サービスをbare cloneし、mainワークツリーを作成する",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		groveDir := worktree.DefaultDir()

		// 各サービスのリポジトリをbare clone + worktree作成
		var services []string
		for name, svc := range cfg.Services {
			// dockerサービスやrepo未設定はgit操作不要
			if svc.Type == "docker" || svc.Repo == "" {
				continue
			}

			services = append(services, name)

			// repo_pathが指定されている場合はbare cloneせず、
			// 既存クローンからworktree addする想定（別途対応）
			if svc.RepoPath != "" {
				fmt.Printf("[grove] %s: repo_pathが指定されています。既存クローンを使用します\n", name)
				continue
			}

			fmt.Printf("[grove] %s をクローン中... (%s)\n", name, svc.Repo)
			if err := worktree.BareClone(groveDir, name, svc.Repo, cloneShallow); err != nil {
				return fmt.Errorf("サービス %s のクローンに失敗: %w", name, err)
			}

			// mainワークツリーを作成
			defaultBranch := cfg.Worktree.DefaultBranch
			if defaultBranch == "" {
				defaultBranch = "main"
			}

			wtPath, err := filepath.Abs(filepath.Join("main", name))
			if err != nil {
				return fmt.Errorf("ワークツリーパスの解決に失敗: %w", err)
			}
			fmt.Printf("[grove] %s のワークツリーを作成中... (branch: %s)\n", name, defaultBranch)
			if err := worktree.WorktreeAdd(groveDir, name, wtPath, defaultBranch); err != nil {
				return fmt.Errorf("サービス %s のワークツリー作成に失敗: %w", name, err)
			}
		}

		// worktrees.json初期化
		baseOffset := cfg.Ports.BaseOffset
		if baseOffset == 0 {
			baseOffset = 100
		}

		ws := &worktree.WorktreeState{
			Active: "main",
			Worktrees: map[string]*worktree.WorktreeInfo{
				"main": {
					Offset:    0,
					CreatedAt: time.Now(),
					Services:  services,
				},
			},
			SharedServices: make(map[string]*worktree.SharedService),
			NextOffset:     baseOffset,
		}

		// sharedサービスを登録
		for name, svc := range cfg.Services {
			if svc.Shared {
				ws.SharedServices[name] = &worktree.SharedService{
					Port: svc.Port,
				}
			}
		}

		if err := ws.Save(groveDir); err != nil {
			return fmt.Errorf("worktrees.jsonの保存に失敗: %w", err)
		}

		fmt.Println("[grove] クローン完了")
		return nil
	},
}

func init() {
	cloneCmd.Flags().BoolVar(&cloneShallow, "shallow", false, "浅いクローンを実行する")
	rootCmd.AddCommand(cloneCmd)
}
