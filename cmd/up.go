package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/ryugen04/grove/internal/config"
	"github.com/ryugen04/grove/internal/dag"
	"github.com/ryugen04/grove/internal/port"
	"github.com/ryugen04/grove/internal/process"
	"github.com/ryugen04/grove/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	upProfile string
	downAll   bool
)

var upCmd = &cobra.Command{
	Use:   "up [services...]",
	Short: "サービスを起動する",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		return runUp(cfg, args, upProfile)
	},
}

func init() {
	upCmd.Flags().StringVar(&upProfile, "profile", "", "プロファイル名")
	rootCmd.AddCommand(upCmd)
}

// runUp はサービスを起動するコアロジック
func runUp(cfg *config.Config, args []string, profile string) error {
	// 対象サービスを決定
	targets := resolveTargets(cfg, args, profile)
	if len(targets) == 0 {
		for name := range cfg.Services {
			targets = append(targets, name)
		}
	}

	// DAGを構築
	d := buildDAG(cfg)
	order, err := d.Resolve(targets...)
	if err != nil {
		return fmt.Errorf("依存解決に失敗: %w", err)
	}

	groveDir := worktree.DefaultDir()
	wtName := resolveActiveWorktree(groveDir)
	wtKey := worktree.ToKey(wtName)

	// worktrees.jsonからオフセットを取得
	ws, err := worktree.Load(groveDir)
	if err != nil {
		return fmt.Errorf("worktrees.jsonの読み込みに失敗: %w", err)
	}
	offset := 0
	if wt, ok := ws.Worktrees[wtName]; ok {
		offset = wt.Offset
	}

	pm := process.NewManager(groveDir, wtKey)
	sharedPM := process.NewManager(groveDir, "shared")

	for _, name := range order {
		svc := cfg.Services[name]

		// ポートオフセット適用
		resolvedPort := port.ResolvePort(svc.Port, offset, svc.Shared)

		// sharedサービスの処理
		if svc.Shared {
			lock, err := worktree.AcquireLock(groveDir, fmt.Sprintf("shared-%s", name))
			if err != nil {
				return fmt.Errorf("sharedロックの取得に失敗 (%s): %w", name, err)
			}

			if sharedPM.IsRunning(name) {
				lock.Release()
				fmt.Printf("[grove] %s は既に起動中 (shared, port: %d)\n", name, resolvedPort)
				continue
			}

			pid, err := startService(sharedPM, cfg, name, svc, resolvedPort, wtKey, "")
			lock.Release()
			if err != nil {
				return fmt.Errorf("sharedサービス %s の起動に失敗: %w", name, err)
			}
			fmt.Printf("[grove] %s を起動しました (shared, PID: %d, Port: %d)\n", name, pid, resolvedPort)
			continue
		}

		// 非sharedサービス
		fmt.Printf("[grove] %s を起動中...\n", name)

		// WorkingDir解決: worktreeディレクトリ内のサービスディレクトリ
		workingDir := resolveWorkingDir(svc, wtName, name)

		switch svc.Type {
		case "docker":
			dockerArgs := process.BuildDockerArgs(process.DockerOptions{
				Name:    fmt.Sprintf("grove-%s-%s", wtKey, name),
				Image:   svc.Image,
				Port:    resolvedPort,
				Env:     mergeEnvWithPort(svc, resolvedPort, cfg, offset),
				Volumes: svc.Volumes,
			})
			pid, err := pm.Start(process.StartOptions{
				Name:    name,
				Command: "docker",
				Args:    dockerArgs,
			})
			if err != nil {
				return fmt.Errorf("サービス %s の起動に失敗: %w", name, err)
			}
			fmt.Printf("[grove] %s を起動しました (PID: %d, Port: %d)\n", name, pid, resolvedPort)

		case "process":
			env := mergeEnvWithPort(svc, resolvedPort, cfg, offset)
			pid, err := pm.Start(process.StartOptions{
				Name:       name,
				Command:    svc.Command,
				Args:       svc.CommandArgs,
				WorkingDir: workingDir,
				Env:        env,
			})
			if err != nil {
				return fmt.Errorf("サービス %s の起動に失敗: %w", name, err)
			}
			fmt.Printf("[grove] %s を起動しました (PID: %d, Port: %d)\n", name, pid, resolvedPort)

		case "script":
			fmt.Printf("[grove] %s を実行中...\n", name)
			c := exec.Command(svc.Command, svc.CommandArgs...)
			c.Dir = workingDir
			env := mergeEnvWithPort(svc, resolvedPort, cfg, offset)
			c.Env = os.Environ()
			for k, v := range env {
				c.Env = append(c.Env, fmt.Sprintf("%s=%s", k, v))
			}
			if err := c.Run(); err != nil {
				return fmt.Errorf("スクリプト %s の実行に失敗: %w", name, err)
			}
			fmt.Printf("[grove] %s を実行しました\n", name)
		}
	}

	return nil
}

// startService はsharedサービスを起動する
func startService(pm *process.Manager, cfg *config.Config, name string, svc *config.Service, resolvedPort int, wtKey, workingDir string) (int, error) {
	switch svc.Type {
	case "docker":
		dockerArgs := process.BuildDockerArgs(process.DockerOptions{
			Name:    fmt.Sprintf("grove-shared-%s", name),
			Image:   svc.Image,
			Port:    resolvedPort,
			Env:     mergeEnv(svc.Env, svc.EnvDynamic),
			Volumes: svc.Volumes,
		})
		return pm.Start(process.StartOptions{
			Name:    name,
			Command: "docker",
			Args:    dockerArgs,
		})
	case "process":
		env := mergeEnv(svc.Env, svc.EnvDynamic)
		return pm.Start(process.StartOptions{
			Name:       name,
			Command:    svc.Command,
			Args:       svc.CommandArgs,
			WorkingDir: workingDir,
			Env:        env,
		})
	default:
		return 0, fmt.Errorf("sharedサービス %s の type %q は非対応です", name, svc.Type)
	}
}

// runDown はサービスを停止するコアロジック
func runDown(cfg *config.Config, args []string) error {
	d := buildDAG(cfg)
	order, err := d.Reverse()
	if err != nil {
		return fmt.Errorf("依存解決に失敗: %w", err)
	}

	groveDir := worktree.DefaultDir()
	wtName := resolveActiveWorktree(groveDir)
	wtKey := worktree.ToKey(wtName)
	pm := process.NewManager(groveDir, wtKey)

	for _, name := range order {
		svc := cfg.Services[name]

		// sharedサービスは通常のdown時は停止しない
		if svc.Shared && !downAll {
			log.Info().Str("service", name).Msg("sharedサービスのため停止しません")
			continue
		}

		// sharedサービスのdown --all
		if svc.Shared && downAll {
			sharedPM := process.NewManager(groveDir, "shared")
			if sharedPM.IsRunning(name) {
				fmt.Printf("[grove] %s を停止中... (shared)\n", name)
				if err := sharedPM.Stop(name); err != nil {
					fmt.Printf("[grove] sharedサービス %s の停止に失敗: %v\n", name, err)
				} else {
					fmt.Printf("[grove] %s を停止しました (shared)\n", name)
				}
			}
			continue
		}

		if !pm.IsRunning(name) {
			continue
		}
		fmt.Printf("[grove] %s を停止中...\n", name)
		if err := pm.Stop(name); err != nil {
			return fmt.Errorf("サービス %s の停止に失敗: %w", name, err)
		}
		fmt.Printf("[grove] %s を停止しました\n", name)
	}

	return nil
}

// resolveWorkingDir はサービスのWorkingDirを解決する
func resolveWorkingDir(svc *config.Service, wtName, serviceName string) string {
	if svc.WorkingDir != "" {
		return filepath.Join(wtName, serviceName, svc.WorkingDir)
	}
	// worktreeディレクトリが存在する場合のみそれを使用
	wtDir := filepath.Join(wtName, serviceName)
	if info, err := os.Stat(wtDir); err == nil && info.IsDir() {
		return wtDir
	}
	return svc.WorkingDir
}

// mergeEnvWithPort はEnvとEnvDynamicをマージし、ポートオフセットを考慮する
func mergeEnvWithPort(svc *config.Service, resolvedPort int, cfg *config.Config, offset int) map[string]string {
	merged := mergeEnv(svc.Env, svc.EnvDynamic)
	// PORT環境変数が動的変数で定義されている場合、オフセット適用後のポートに更新
	if _, ok := svc.EnvDynamic["PORT"]; ok {
		merged["PORT"] = fmt.Sprintf("%d", resolvedPort)
	}
	return merged
}

// buildDAG は設定からDAGを構築する
func buildDAG(cfg *config.Config) *dag.DAG {
	depMap := make(map[string][]string)
	for name, svc := range cfg.Services {
		depMap[name] = svc.DependsOn
	}
	return dag.BuildFromServices(depMap)
}

// loadConfig は設定ファイルの読み込み・検証・変数展開をまとめて行う
func loadConfig() (*config.Config, error) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	config.ExpandVariables(cfg)
	return cfg, nil
}

// resolveTargets は起動対象サービスのリストを返す
func resolveTargets(cfg *config.Config, args []string, profile string) []string {
	if len(args) > 0 {
		return args
	}
	if profile != "" {
		if p, ok := cfg.Profiles[profile]; ok {
			return p.Services
		}
	}
	return nil
}

// mergeEnv はEnvとEnvDynamicをマージする
func mergeEnv(env, envDynamic map[string]string) map[string]string {
	merged := make(map[string]string)
	for k, v := range env {
		merged[k] = v
	}
	for k, v := range envDynamic {
		merged[k] = v
	}
	return merged
}
