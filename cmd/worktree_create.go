package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/ryugen04/sango-tree/internal/config"
	"github.com/ryugen04/sango-tree/internal/worktree"
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
	sangoDir := worktree.DefaultDir()

	// ロック取得
	lock, err := worktree.AcquireLock(sangoDir, "worktree-op")
	if err != nil {
		return fmt.Errorf("ロックの取得に失敗: %w", err)
	}
	defer lock.Release()

	ws, err := worktree.Load(sangoDir)
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
	targetServices, err := resolveWorktreeServices(cfg, wtCreateServices)
	if err != nil {
		return err
	}

	// ベースブランチ
	baseBranch := wtCreateFrom
	if baseBranch == "" {
		baseBranch = cfg.Worktree.DefaultBranch
		if baseBranch == "" {
			baseBranch = "main"
		}
	}

	// ロールバック用: worktreeパスとサービス名のペアを追跡
	type createdEntry struct {
		path        string
		serviceName string
	}
	var created []createdEntry
	var allServiceNames []string

	// 各サービスについてworktree作成
	for _, name := range targetServices {
		svc, ok := cfg.Services[name]
		if !ok {
			return fmt.Errorf("サービス %q は設定に存在しません", name)
		}

		if svc.Type == "docker" || svc.Repo == "" {
			allServiceNames = append(allServiceNames, name)
			continue
		}

		wtDir := cfg.Worktree.WorktreeDir(branch)
		absWtPath, err := filepath.Abs(filepath.Join(wtDir, name))
		if err != nil {
			return fmt.Errorf("ワークツリーパスの解決に失敗: %w", err)
		}
		fmt.Printf("[sango] %s のワークツリーを作成中... (branch: %s from %s)\n", name, branch, baseBranch)

		// まず新規ブランチ作成を試み、既存ブランチなら既存ブランチでworktree追加
		if err := worktree.WorktreeAddNewBranch(sangoDir, name, absWtPath, branch, baseBranch); err != nil {
			// 既存ブランチの場合はWorktreeAddにフォールバック
			if err2 := worktree.WorktreeAdd(sangoDir, name, absWtPath, branch); err2 != nil {
				// 両方失敗した場合のみロールバック
				for _, e := range created {
					fmt.Printf("[sango] ロールバック: %s を削除中...\n", e.serviceName)
					if rbErr := worktree.WorktreeRemove(sangoDir, e.serviceName, e.path, true); rbErr != nil {
						fmt.Printf("[sango] ロールバック警告: %s の削除に失敗: %v\n", e.serviceName, rbErr)
					}
				}
				return fmt.Errorf("サービス %s のワークツリー作成に失敗: %w", name, err2)
			}
		}

		created = append(created, createdEntry{path: absWtPath, serviceName: name})
		allServiceNames = append(allServiceNames, name)
	}

	// include展開
	if len(cfg.Worktree.Include.Root) > 0 || len(cfg.Worktree.Include.PerService) > 0 {
		fmt.Println("[sango] includeファイルを展開中...")
		vars := buildIncludeVars(cfg, offset)
		// sourceはプロジェクトルート（cwd）基準、targetはworktreeDir基準
		projectRoot, _ := os.Getwd()
		result := worktree.ExpandIncludes(projectRoot, cfg.Worktree.WorktreeDir(branch), allServiceNames, cfg.Worktree.Include, vars)
		if result.HasErrors() {
			// ロールバック実行
			for _, e := range created {
				fmt.Printf("[sango] ロールバック: %s を削除中...\n", e.serviceName)
				if rbErr := worktree.WorktreeRemove(sangoDir, e.serviceName, e.path, true); rbErr != nil {
					fmt.Printf("[sango] ロールバック警告: %s の削除に失敗: %v\n", e.serviceName, rbErr)
				}
			}
			return fmt.Errorf("必須includeエントリの展開に失敗: %w", result.CriticalError())
		}
		if warning := result.WarningError(); warning != nil {
			fmt.Printf("[sango] include展開で警告: %v\n", warning)
		}
	}

	// auto_setupの実行
	if cfg.Worktree.AutoSetup && !wtCreateNoSetup {
		for _, name := range allServiceNames {
			svc := cfg.Services[name]
			if len(svc.Setup) > 0 {
				fmt.Printf("[sango] %s のセットアップを実行中...\n", name)
				// repo_nameが設定されている場合、参照先のディレクトリを使う
				dirName := name
				if svc.RepoName != "" {
					dirName = svc.RepoName
				}
				for _, setupCmd := range svc.Setup {
					c := exec.Command("sh", "-c", setupCmd)
					c.Dir = filepath.Join(cfg.Worktree.WorktreeDir(branch), dirName)
					if out, err := c.CombinedOutput(); err != nil {
						fmt.Printf("[sango] %s のセットアップ警告: %v\n%s", name, err, out)
					}
				}
			}
		}
	}

	// post_createフック実行
	if len(cfg.Worktree.Hooks.PostCreate) > 0 {
		fmt.Println("[sango] post_createフックを実行中...")
		if err := worktree.RunHooks(cfg.Worktree.Hooks.PostCreate, cfg.Worktree.WorktreeDir(branch), allServiceNames); err != nil {
			fmt.Printf("[sango] post_createフック警告: %v\n", err)
		}
	}

	// worktrees.json更新
	ws.AddWorktree(branch, &worktree.WorktreeInfo{
		Offset:     offset,
		CreatedAt:  time.Now(),
		Services:   allServiceNames,
		FromBranch: baseBranch,
	})

	if err := ws.Save(sangoDir); err != nil {
		return fmt.Errorf("worktrees.jsonの保存に失敗: %w", err)
	}

	fmt.Printf("[sango] ワークツリー %q を作成しました (offset: %d)\n", branch, offset)
	return nil
}

// repoInfo はリポジトリの表示用情報
type repoInfo struct {
	Name    string   // リポジトリ名（サービス名）
	Servers []string // このリポジトリに紐づくサーバー名
}

// collectRepos はConfigからリポジトリ一覧を収集する
func collectRepos(cfg *config.Config) []repoInfo {
	// repo フィールドを持つサービス = リポジトリ
	repos := make(map[string]*repoInfo)
	var repoOrder []string

	for name, svc := range cfg.Services {
		if svc.Repo != "" {
			repos[name] = &repoInfo{Name: name}
			repoOrder = append(repoOrder, name)
		}
	}
	sort.Strings(repoOrder)

	// repo_name で参照しているサーバーを紐づける
	for name, svc := range cfg.Services {
		if svc.RepoName != "" {
			if ri, ok := repos[svc.RepoName]; ok {
				ri.Servers = append(ri.Servers, name)
			}
		}
	}

	// サーバー名をソート
	for _, ri := range repos {
		sort.Strings(ri.Servers)
	}

	var result []repoInfo
	for _, name := range repoOrder {
		result = append(result, *repos[name])
	}
	return result
}

// reposToServices は選択されたリポジトリ名から対象サービスリストを返す
// リポジトリ自体 + repo_name で参照するサーバー + shared サービスを含む
func reposToServices(cfg *config.Config, selectedRepos []string) []string {
	selected := make(map[string]bool)
	for _, r := range selectedRepos {
		selected[r] = true
	}

	var services []string
	for name, svc := range cfg.Services {
		// sharedサービスは常に含める
		if svc.Shared {
			services = append(services, name)
			continue
		}
		// 選択されたリポジトリ自体
		if selected[name] {
			services = append(services, name)
			continue
		}
		// repo_name が選択されたリポジトリを参照している
		if svc.RepoName != "" && selected[svc.RepoName] {
			services = append(services, name)
		}
	}
	sort.Strings(services)
	return services
}

// resolveWorktreeServices は対象サービスリストを返す
func resolveWorktreeServices(cfg *config.Config, servicesFlag string) ([]string, error) {
	// --services フラグが指定された場合はそのまま返す
	if servicesFlag != "" {
		return strings.Split(servicesFlag, ","), nil
	}

	// インタラクティブにリポジトリを選択
	repos := collectRepos(cfg)
	if len(repos) == 0 {
		// リポジトリがない場合は全サービスを返す
		var names []string
		for name := range cfg.Services {
			names = append(names, name)
		}
		return names, nil
	}

	// 選択肢を構築（デフォルトで全リポジトリを選択状態）
	var options []huh.Option[string]
	for _, ri := range repos {
		desc := ri.Name
		if len(ri.Servers) > 0 {
			desc = fmt.Sprintf("%s (%s)", ri.Name, strings.Join(ri.Servers, ", "))
		} else {
			desc = fmt.Sprintf("%s (サーバーなし)", ri.Name)
		}
		options = append(options, huh.NewOption(desc, ri.Name).Selected(true))
	}

	var selectedRepos []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("対象リポジトリを選択してください").
				Options(options...).
				Value(&selectedRepos),
		),
	)

	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("リポジトリ選択がキャンセルされました: %w", err)
	}

	if len(selectedRepos) == 0 {
		return nil, fmt.Errorf("リポジトリが選択されていません")
	}

	return reposToServices(cfg, selectedRepos), nil
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
