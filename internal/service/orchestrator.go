package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/rs/zerolog/log"
	"github.com/ryugen04/sango-tree/internal/config"
	"github.com/ryugen04/sango-tree/internal/dag"
	sangoLog "github.com/ryugen04/sango-tree/internal/log"
	"github.com/ryugen04/sango-tree/internal/port"
	"github.com/ryugen04/sango-tree/internal/process"
	"github.com/ryugen04/sango-tree/internal/worktree"
)

// Orchestrator はサービスのライフサイクルを管理する
type Orchestrator struct {
	cfg      *config.Config
	cfgFile  string
	sangoDir string
	wtName   string
	wtKey    string
	offset   int
}

// NewOrchestrator はOrchestratorを生成する
func NewOrchestrator(cfg *config.Config, cfgFile string) (*Orchestrator, error) {
	return NewOrchestratorWithWorktree(cfg, cfgFile, "")
}

// NewOrchestratorWithWorktree はworktree名を指定してOrchestratorを生成する
func NewOrchestratorWithWorktree(cfg *config.Config, cfgFile, worktreeFlag string) (*Orchestrator, error) {
	sangoDir := worktree.DefaultDir()
	wtName := ResolveActiveWorktree(sangoDir, worktreeFlag)
	wtKey := worktree.ToKey(wtName)

	ws, err := worktree.Load(sangoDir)
	if err != nil {
		return nil, fmt.Errorf("worktrees.jsonの読み込みに失敗: %w", err)
	}
	offset := 0
	if wt, ok := ws.Worktrees[wtName]; ok {
		offset = wt.Offset
	}

	return &Orchestrator{
		cfg:      cfg,
		cfgFile:  cfgFile,
		sangoDir: sangoDir,
		wtName:   wtName,
		wtKey:    wtKey,
		offset:   offset,
	}, nil
}

// ResolveActiveWorktree は使用するworktree名を解決する
func ResolveActiveWorktree(sangoDir, worktreeFlag string) string {
	if worktreeFlag != "" {
		return worktreeFlag
	}
	ws, err := worktree.Load(sangoDir)
	if err != nil {
		return "main"
	}
	if ws.Active == "" {
		return "main"
	}
	return ws.Active
}

// Up はサービスを起動する
func (o *Orchestrator) Up(services []string, profile string) (*UpResult, error) {
	targets := ResolveTargets(o.cfg, services, profile)
	if len(targets) == 0 {
		for name := range o.cfg.Services {
			targets = append(targets, name)
		}
	}

	d := BuildDAG(o.cfg)
	order, err := d.Resolve(targets...)
	if err != nil {
		return nil, fmt.Errorf("依存解決に失敗: %w", err)
	}

	pm := process.NewManager(o.sangoDir, o.wtKey)
	sharedPM := process.NewManager(o.sangoDir, "shared")

	result := &UpResult{}

	for _, name := range order {
		svc := o.cfg.Services[name]
		resolvedPort := port.ResolvePort(svc.Port, o.offset, svc.Shared)

		if svc.Shared {
			lock, err := worktree.AcquireLock(o.sangoDir, fmt.Sprintf("shared-%s", name))
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("sharedロックの取得に失敗 (%s): %v", name, err))
				continue
			}

			if sharedPM.IsRunning(name) {
				lock.Release()
				result.Started = append(result.Started, ServiceInfo{
					Name:   name,
					Type:   svc.Type,
					Port:   resolvedPort,
					Status: "already_running",
				})
				continue
			}

			pid, err := o.startService(sharedPM, name, svc, resolvedPort, "")
			lock.Release()
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("sharedサービス %s の起動に失敗: %v", name, err))
				continue
			}
			result.Started = append(result.Started, ServiceInfo{
				Name:   name,
				Type:   svc.Type,
				Port:   resolvedPort,
				Status: "started",
				PID:    pid,
			})
			continue
		}

		workingDir := ResolveWorkingDir(svc, o.wtName, name)

		switch svc.Type {
		case "docker":
			dockerArgs := process.BuildDockerArgs(process.DockerOptions{
				Name:    fmt.Sprintf("sango-%s-%s", o.wtKey, name),
				Image:   svc.Image,
				Port:    resolvedPort,
				Env:     MergeEnvWithPort(svc, resolvedPort, o.cfg, o.offset, o.cfgFile),
				Volumes: svc.Volumes,
			})
			pid, err := pm.Start(process.StartOptions{
				Name:    name,
				Command: "docker",
				Args:    dockerArgs,
			})
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("サービス %s の起動に失敗: %v", name, err))
				continue
			}
			result.Started = append(result.Started, ServiceInfo{
				Name:   name,
				Type:   svc.Type,
				Port:   resolvedPort,
				Status: "started",
				PID:    pid,
			})

		case "process":
			env := MergeEnvWithPort(svc, resolvedPort, o.cfg, o.offset, o.cfgFile)
			pid, err := pm.Start(process.StartOptions{
				Name:         name,
				Command:      svc.Command,
				Args:         svc.CommandArgs,
				WorkingDir:   workingDir,
				Env:          env,
				Restart:      svc.Restart,
				RestartDelay: svc.ParseRestartDelay(),
				MaxRestarts:  svc.MaxRestarts,
			})
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("サービス %s の起動に失敗: %v", name, err))
				continue
			}
			result.Started = append(result.Started, ServiceInfo{
				Name:   name,
				Type:   svc.Type,
				Port:   resolvedPort,
				Status: "started",
				PID:    pid,
			})

		case "script":
			c := exec.Command(svc.Command, svc.CommandArgs...)
			c.Dir = workingDir
			env := MergeEnvWithPort(svc, resolvedPort, o.cfg, o.offset, o.cfgFile)
			c.Env = os.Environ()
			for k, v := range env {
				c.Env = append(c.Env, fmt.Sprintf("%s=%s", k, v))
			}
			collector, collErr := sangoLog.NewCollector(o.sangoDir, o.wtKey, name)
			if collErr != nil {
				log.Warn().Str("service", name).Err(collErr).Msg("ログコレクターの初期化に失敗")
				c.Stdout = os.Stdout
				c.Stderr = os.Stderr
			} else if stdoutW, err := collector.StdoutWriter(); err != nil {
				log.Warn().Str("service", name).Err(err).Msg("stdoutパイプの作成に失敗")
				collector.Close()
				collector = nil
				c.Stdout = os.Stdout
				c.Stderr = os.Stderr
			} else if stderrW, err := collector.StderrWriter(); err != nil {
				log.Warn().Str("service", name).Err(err).Msg("stderrパイプの作成に失敗")
				collector.Close()
				collector = nil
				c.Stdout = os.Stdout
				c.Stderr = os.Stderr
			} else {
				c.Stdout = stdoutW
				c.Stderr = stderrW
			}
			if err := c.Run(); err != nil {
				if collector != nil {
					collector.Close()
				}
				result.Errors = append(result.Errors, fmt.Sprintf("スクリプト %s の実行に失敗: %v", name, err))
				continue
			}
			if collector != nil {
				collector.Close()
			}
			result.Started = append(result.Started, ServiceInfo{
				Name:   name,
				Type:   svc.Type,
				Port:   resolvedPort,
				Status: "completed",
			})
		}

		// ヘルスチェック
		if svc.Healthcheck != nil && svc.Type != "script" {
			hcCfg := process.HealthcheckConfig{
				Command:     svc.Healthcheck.Command,
				URL:         svc.Healthcheck.URL,
				Interval:    svc.Healthcheck.ParseInterval(),
				Timeout:     svc.Healthcheck.ParseTimeout(),
				Retries:     svc.Healthcheck.Retries,
				StartPeriod: svc.Healthcheck.ParseStartPeriod(),
				WorkingDir:  workingDir,
			}
			if err := process.RunHealthcheck(context.Background(), name, hcCfg); err != nil {
				if stopErr := pm.Stop(name); stopErr != nil {
					log.Warn().Str("service", name).Err(stopErr).Msg("ヘルスチェック失敗後の停止に失敗")
				}
				result.Errors = append(result.Errors, fmt.Sprintf("ヘルスチェック失敗 (%s): %v", name, err))
				continue
			}
			_ = process.WriteState(o.sangoDir, o.wtKey, name, &process.ServiceState{HealthStatus: "healthy"})
			// Startedの最後のエントリにHealth情報を追加
			if len(result.Started) > 0 {
				result.Started[len(result.Started)-1].Health = "healthy"
			}
		}
	}

	return result, nil
}

// Down はサービスを停止する
func (o *Orchestrator) Down(services []string, all bool) (*DownResult, error) {
	d := BuildDAG(o.cfg)

	var order []string
	if len(services) > 0 {
		resolved, err := d.Resolve(services...)
		if err != nil {
			return nil, fmt.Errorf("依存解決に失敗: %w", err)
		}
		// 逆順で停止
		for i := len(resolved) - 1; i >= 0; i-- {
			order = append(order, resolved[i])
		}
	} else {
		var err error
		order, err = d.Reverse()
		if err != nil {
			return nil, fmt.Errorf("依存解決に失敗: %w", err)
		}
	}

	pm := process.NewManager(o.sangoDir, o.wtKey)
	result := &DownResult{}

	for _, name := range order {
		svc := o.cfg.Services[name]

		if svc.Shared && !all {
			continue
		}

		if svc.Shared && all {
			sharedPM := process.NewManager(o.sangoDir, "shared")
			if sharedPM.IsRunning(name) {
				if err := sharedPM.Stop(name); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("sharedサービス %s の停止に失敗: %v", name, err))
				} else {
					result.Stopped = append(result.Stopped, name)
				}
			}
			continue
		}

		if !pm.IsRunning(name) {
			continue
		}
		if err := pm.Stop(name); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("サービス %s の停止に失敗: %v", name, err))
			continue
		}
		result.Stopped = append(result.Stopped, name)
	}

	return result, nil
}

// Restart はサービスを再起動する
func (o *Orchestrator) Restart(services []string, profile string) (*UpResult, error) {
	d := BuildDAG(o.cfg)
	targets := ResolveTargets(o.cfg, services, profile)

	pm := process.NewManager(o.sangoDir, o.wtKey)

	// 停止フェーズ
	var stopOrder []string
	if len(targets) > 0 {
		resolved, err := d.Resolve(targets...)
		if err != nil {
			return nil, fmt.Errorf("依存解決に失敗: %w", err)
		}
		for i := len(resolved) - 1; i >= 0; i-- {
			stopOrder = append(stopOrder, resolved[i])
		}
	} else {
		var err error
		stopOrder, err = d.Reverse()
		if err != nil {
			return nil, fmt.Errorf("依存解決に失敗: %w", err)
		}
	}

	for _, name := range stopOrder {
		if !pm.IsRunning(name) {
			continue
		}
		if err := pm.Stop(name); err != nil {
			log.Warn().Str("service", name).Err(err).Msg("停止に失敗")
		}
	}

	// 起動フェーズ
	return o.Up(services, profile)
}

// Status はサービス状態を取得する
func (o *Orchestrator) Status() (*StatusResult, error) {
	result := &StatusResult{
		Worktree: o.wtName,
	}

	names := make([]string, 0, len(o.cfg.Services))
	for name := range o.cfg.Services {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		svc := o.cfg.Services[name]

		resolvedPort := 0
		if svc.Port > 0 {
			resolvedPort = svc.Port
			if !svc.Shared {
				resolvedPort += o.offset
			}
		}

		status := "stopped"
		pid := 0

		pidWorktree := o.wtKey
		if svc.Shared {
			pidWorktree = "shared"
		}

		if p, err := process.ReadPID(o.sangoDir, pidWorktree, name); err == nil {
			if process.IsProcessRunning(p) {
				status = "running"
				pid = p
			}
		}

		state := process.ReadState(o.sangoDir, pidWorktree, name)
		health := ""
		if state.HealthStatus != "" {
			health = state.HealthStatus
		}

		result.Services = append(result.Services, ServiceInfo{
			Name:         name,
			Type:         svc.Type,
			Port:         resolvedPort,
			Status:       status,
			Health:       health,
			PID:          pid,
			RestartCount: state.RestartCount,
		})
	}

	return result, nil
}

// startService はsharedサービスを起動する
func (o *Orchestrator) startService(pm *process.Manager, name string, svc *config.Service, resolvedPort int, workingDir string) (int, error) {
	switch svc.Type {
	case "docker":
		dockerArgs := process.BuildDockerArgs(process.DockerOptions{
			Name:    fmt.Sprintf("sango-shared-%s", name),
			Image:   svc.Image,
			Port:    resolvedPort,
			Env:     MergeEnv(svc.Env, svc.EnvDynamic),
			Volumes: svc.Volumes,
		})
		return pm.Start(process.StartOptions{
			Name:    name,
			Command: "docker",
			Args:    dockerArgs,
		})
	case "process":
		env := MergeEnv(svc.Env, svc.EnvDynamic)
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

// BuildDAG は設定からDAGを構築する
func BuildDAG(cfg *config.Config) *dag.DAG {
	depMap := make(map[string][]string)
	for name, svc := range cfg.Services {
		depMap[name] = svc.DependsOn
	}
	return dag.BuildFromServices(depMap)
}

// ResolveTargets は起動対象サービスのリストを返す
func ResolveTargets(cfg *config.Config, args []string, profile string) []string {
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

// ResolveWorkingDir はサービスのWorkingDirを解決する
func ResolveWorkingDir(svc *config.Service, wtName, serviceName string) string {
	if svc.WorkingDir != "" {
		return filepath.Join(wtName, serviceName, svc.WorkingDir)
	}
	wtDir := filepath.Join(wtName, serviceName)
	if info, err := os.Stat(wtDir); err == nil && info.IsDir() {
		return wtDir
	}
	return svc.WorkingDir
}

// MergeEnvAll はenv_file, Env, EnvDynamicを優先順位つきでマージする
func MergeEnvAll(svc *config.Service, cfgDir string) (map[string]string, error) {
	merged := make(map[string]string)
	if svc.EnvFile != "" {
		envFilePath := svc.EnvFile
		if !filepath.IsAbs(envFilePath) {
			envFilePath = filepath.Join(cfgDir, envFilePath)
		}
		envFromFile, err := config.LoadEnvFile(envFilePath)
		if err != nil {
			return nil, err
		}
		for k, v := range envFromFile {
			merged[k] = v
		}
	}
	for k, v := range svc.Env {
		merged[k] = v
	}
	for k, v := range svc.EnvDynamic {
		merged[k] = v
	}
	return merged, nil
}

// MergeEnvWithPort はEnvとEnvDynamicをマージし、ポートオフセットを考慮する
func MergeEnvWithPort(svc *config.Service, resolvedPort int, cfg *config.Config, offset int, cfgFile string) map[string]string {
	cfgDir := filepath.Dir(cfgFile)
	merged, err := MergeEnvAll(svc, cfgDir)
	if err != nil {
		log.Warn().Err(err).Msg("env_fileの読み込みに失敗")
		merged = MergeEnv(svc.Env, svc.EnvDynamic)
	}
	if _, ok := svc.EnvDynamic["PORT"]; ok {
		merged["PORT"] = strconv.Itoa(resolvedPort)
	}
	return merged
}

// MergeEnv はEnvとEnvDynamicをマージする
func MergeEnv(env, envDynamic map[string]string) map[string]string {
	merged := make(map[string]string)
	for k, v := range env {
		merged[k] = v
	}
	for k, v := range envDynamic {
		merged[k] = v
	}
	return merged
}

// LoadAndValidateConfig は設定ファイルの読み込み・検証・変数展開をまとめて行う
func LoadAndValidateConfig(cfgFile string) (*config.Config, error) {
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
