package cmd

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ryugen04/grove/internal/config"
	"github.com/ryugen04/grove/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	wtCreateServices string
	wtCreateNoSetup  bool
	wtCreateFrom     string
)

var worktreeCreateCmd = &cobra.Command{
	Use:   "create <branch>",
	Short: "新しいワークツリーを作成する",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		branch := args[0]
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		return runWorktreeCreate(cfg, branch)
	},
}

func init() {
	worktreeCreateCmd.Flags().StringVar(&wtCreateServices, "services", "", "対象サービス（カンマ区切り）")
	worktreeCreateCmd.Flags().BoolVar(&wtCreateNoSetup, "no-setup", false, "セットアップをスキップする")
	worktreeCreateCmd.Flags().StringVar(&wtCreateFrom, "from", "", "ベースブランチ名")
	worktreeCmd.AddCommand(worktreeCreateCmd)
}

func runWorktreeCreate(cfg *config.Config, branch string) error {
	groveDir := worktree.DefaultDir()

	// ロック取得
	lock, err := worktree.AcquireLock(groveDir, "worktree-op")
	if err != nil {
		return fmt.Errorf("ロックの取得に失敗: %w", err)
	}
	defer lock.Release()

	ws, err := worktree.Load(groveDir)
	if err != nil {
		return fmt.Errorf("worktrees.jsonの読み込みに失敗: %w", err)
	}

	// 既存チェック
	if _, exists := ws.Worktrees[branch]; exists {
		return fmt.Errorf("ワークツリー %q は既に存在します", branch)
	}

	// オフセット割り当て
	baseOffset := cfg.Ports.BaseOffset
	if baseOffset == 0 {
		baseOffset = 100
	}
	offset := ws.AllocateOffset(baseOffset)

	// 対象サービスを決定
	targetServices := resolveWorktreeServices(cfg, wtCreateServices)

	// ベースブランチ
	baseBranch := wtCreateFrom
	if baseBranch == "" {
		baseBranch = cfg.Worktree.DefaultBranch
		if baseBranch == "" {
			baseBranch = "main"
		}
	}

	// 作成済みworktreeを追跡（ロールバック用）
	var createdPaths []string
	var createdServiceNames []string

	// 各サービスについてworktree作成
	for _, name := range targetServices {
		svc := cfg.Services[name]
		if svc.Type == "docker" || svc.Repo == "" {
			createdServiceNames = append(createdServiceNames, name)
			continue
		}

		wtPath := filepath.Join(branch, name)
		fmt.Printf("[grove] %s のワークツリーを作成中... (branch: %s from %s)\n", name, branch, baseBranch)

		if err := worktree.WorktreeAddNewBranch(groveDir, name, wtPath, branch, baseBranch); err != nil {
			// ロールバック
			rollbackWorktrees(groveDir, cfg, createdPaths, createdServiceNames, branch)
			return fmt.Errorf("サービス %s のワークツリー作成に失敗: %w", name, err)
		}

		createdPaths = append(createdPaths, wtPath)
		createdServiceNames = append(createdServiceNames, name)
	}

	// include展開
	if len(cfg.Worktree.Include.Common) > 0 || len(cfg.Worktree.Include.PerService) > 0 {
		fmt.Println("[grove] includeファイルを展開中...")
		vars := buildIncludeVars(cfg, offset)
		if err := worktree.ExpandIncludes(branch, createdServiceNames, cfg.Worktree.Include, vars); err != nil {
			fmt.Printf("[grove] include展開で警告: %v\n", err)
		}
	}

	// auto_setupの実行
	if cfg.Worktree.AutoSetup && !wtCreateNoSetup {
		for _, name := range createdServiceNames {
			svc := cfg.Services[name]
			if len(svc.Setup) > 0 {
				fmt.Printf("[grove] %s のセットアップを実行中...\n", name)
				for _, setupCmd := range svc.Setup {
					c := exec.Command("sh", "-c", setupCmd)
					c.Dir = filepath.Join(branch, name)
					if out, err := c.CombinedOutput(); err != nil {
						fmt.Printf("[grove] %s のセットアップ警告: %v\n%s", name, err, out)
					}
				}
			}
		}
	}

	// worktrees.json更新
	ws.AddWorktree(branch, &worktree.WorktreeInfo{
		Offset:     offset,
		CreatedAt:  time.Now(),
		Services:   createdServiceNames,
		FromBranch: baseBranch,
	})

	if err := ws.Save(groveDir); err != nil {
		return fmt.Errorf("worktrees.jsonの保存に失敗: %w", err)
	}

	fmt.Printf("[grove] ワークツリー %q を作成しました (offset: %d)\n", branch, offset)
	return nil
}

// resolveWorktreeServices は対象サービスリストを返す
func resolveWorktreeServices(cfg *config.Config, servicesFlag string) []string {
	if servicesFlag != "" {
		return strings.Split(servicesFlag, ",")
	}
	var names []string
	for name := range cfg.Services {
		names = append(names, name)
	}
	return names
}

// buildIncludeVars はinclude/template展開用の変数マップを構築する
func buildIncludeVars(cfg *config.Config, offset int) map[string]string {
	vars := make(map[string]string)
	for name, svc := range cfg.Services {
		resolvedPort := svc.Port
		if !svc.Shared {
			resolvedPort += offset
		}
		vars[fmt.Sprintf("services.%s.port", name)] = fmt.Sprintf("%d", resolvedPort)
	}
	return vars
}

// rollbackWorktrees は作成済みworktreeをロールバックする
func rollbackWorktrees(groveDir string, cfg *config.Config, paths, serviceNames []string, branch string) {
	fmt.Println("[grove] エラー発生。作成済みワークツリーをロールバック中...")
	for i, wtPath := range paths {
		name := serviceNames[i]
		svc := cfg.Services[name]
		if svc.Type == "docker" || svc.Repo == "" {
			continue
		}
		if err := worktree.WorktreeRemove(groveDir, name, wtPath, true); err != nil {
			fmt.Printf("[grove] ロールバック警告: %s の削除に失敗: %v\n", name, err)
		}
	}
}
