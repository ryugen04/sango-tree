package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"syscall"
	"time"

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
	wtDir    string // worktreeディレクトリ (例: "worktrees/main")
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
	var wtServices []string
	if wt, ok := ws.Worktrees[wtName]; ok {
		offset = wt.Offset
		wtServices = wt.Services
	}

	// オフセット決定後に変数展開を実行（パーシャルワークツリー対応）
	config.ExpandVariablesWithOffset(cfg, offset, wtServices)

	return &Orchestrator{
		cfg:      cfg,
		cfgFile:  cfgFile,
		sangoDir: sangoDir,
		wtName:   wtName,
		wtKey:    wtKey,
		wtDir:    cfg.Worktree.WorktreeDir(wtName),
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

// ResolveServicePorts は全サービスのポートを解決したマップを返す
func (o *Orchestrator) ResolveServicePorts() map[string]int {
	ports := make(map[string]int)
	for name, svc := range o.cfg.Services {
		if svc.Port > 0 {
			ports[name] = port.ResolvePort(svc.Port, o.offset, svc.Shared)
		}
	}
	return ports
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

		// commandなしサービス（repo-only）はスキップ
		if svc.Command == "" && svc.Type != "docker" {
			result.Started = append(result.Started, ServiceInfo{
				Name:   name,
				Type:   svc.Type,
				Status: "skipped",
			})
			continue
		}

		resolvedPort := port.ResolvePort(svc.Port, o.offset, svc.Shared)

		if svc.Shared {
			// sharedのscriptタイプはヘルスチェック用コマンド（外部管理サービス）
			// PID管理不要、コマンド実行して成功すればOK
			if svc.Type == "script" {
				c := exec.Command("sh", "-c", svc.Command)
				if out, err := c.CombinedOutput(); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("sharedサービス %s の確認に失敗: %v\n%s", name, err, out))
					continue
				}
				result.Started = append(result.Started, ServiceInfo{
					Name:   name,
					Type:   svc.Type,
					Port:   resolvedPort,
					Status: "external",
				})
				continue
			}

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

		// 既に起動中ならスキップ
		if pm.IsRunning(name) {
			result.Started = append(result.Started, ServiceInfo{
				Name:   name,
				Type:   svc.Type,
				Port:   resolvedPort,
				Status: "already_running",
			})
			continue
		}

		// ポート競合チェック: 他worktreeの残存プロセスを検出・kill
		if resolvedPort > 0 {
			if holderPID, err := port.GetPortHolder(resolvedPort); err == nil && holderPID > 0 {
				wtOwner, svcOwner, found := process.FindPIDOwner(o.sangoDir, holderPID)
				if found {
					log.Warn().
						Str("service", name).
						Int("port", resolvedPort).
						Int("pid", holderPID).
						Str("owner_worktree", wtOwner).
						Str("owner_service", svcOwner).
						Msg("ポート競合: 他worktreeの残存プロセスを検出、killします")
				} else {
					log.Warn().
						Str("service", name).
						Int("port", resolvedPort).
						Int("pid", holderPID).
						Msg("ポート競合: 孤児プロセスを検出、killします")
				}
				// プロセスグループにSIGTERMを試みてからSIGKILL（孤児プロセス防止）
				pgid, pgidErr := syscall.Getpgid(holderPID)
				if pgidErr == nil && pgid > 0 {
					_ = syscall.Kill(-pgid, syscall.SIGTERM)
				} else {
					_ = syscall.Kill(holderPID, syscall.SIGTERM)
				}
				time.Sleep(2 * time.Second)
				if process.IsProcessRunning(holderPID) {
					if pgidErr == nil && pgid > 0 {
						_ = syscall.Kill(-pgid, syscall.SIGKILL)
					} else {
						_ = syscall.Kill(holderPID, syscall.SIGKILL)
					}
					time.Sleep(200 * time.Millisecond)
				}
				// PIDファイルのクリーンアップ
				if found {
					_ = process.RemovePID(o.sangoDir, wtOwner, svcOwner)
				}
			}
		}

		workingDir := ResolveWorkingDir(svc, o.wtDir, name)

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
				Name:    name,
				Type:    svc.Type,
				Port:    resolvedPort,
				Status:  "started",
				PID:     pid,
				OpenURL: svc.OpenURL,
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

	// Up失敗時のロールバック: 起動済みプロセスサービスを停止
	if len(result.Errors) > 0 {
		for _, info := range result.Started {
			if info.Status == "started" {
				log.Warn().Str("service", info.Name).Msg("起動失敗によるロールバック: 停止します")
				svc := o.cfg.Services[info.Name]
				var mgr *process.Manager
				if svc != nil && svc.Shared {
					mgr = process.NewManager(o.sangoDir, "shared")
				} else {
					mgr = process.NewManager(o.sangoDir, o.wtKey)
				}
				if err := mgr.Stop(info.Name); err != nil {
					log.Warn().Str("service", info.Name).Err(err).Msg("ロールバック停止に失敗")
				}
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

		// ポートベースのクリーンアップ: Stop後もポートが使用中なら残存プロセスをkill
		if svc.Port > 0 {
			resolvedPort := svc.Port
			if !svc.Shared {
				resolvedPort = svc.Port + o.offset
			}
			if holderPID, err := port.GetPortHolder(resolvedPort); err == nil && holderPID > 0 {
				log.Warn().Str("service", name).Int("port", resolvedPort).Int("pid", holderPID).
					Msg("停止後もポートが使用中、残存プロセスをkill")
				pgid, pgidErr := syscall.Getpgid(holderPID)
				if pgidErr == nil && pgid > 0 {
					_ = syscall.Kill(-pgid, syscall.SIGTERM)
					time.Sleep(1 * time.Second)
					if process.IsProcessRunning(holderPID) {
						_ = syscall.Kill(-pgid, syscall.SIGKILL)
					}
				} else {
					_ = syscall.Kill(holderPID, syscall.SIGKILL)
				}
			}
		}
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

	// ワークツリー一覧を取得
	ws, err := worktree.Load(o.sangoDir)
	if err == nil && ws != nil {
		for name, wt := range ws.Worktrees {
			result.Worktrees = append(result.Worktrees, WorktreeInfo{
				Name:     name,
				Offset:   wt.Offset,
				IsActive: name == ws.Active,
			})
		}
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

		// shared + script サービス（外部管理コンテナ等）はコマンド実行で状態判定
		if svc.Shared && svc.Type == "script" && svc.Command != "" {
			c := exec.Command("sh", "-c", svc.Command)
			if err := c.Run(); err == nil {
				status = "running"
			}
		} else {
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

			// PIDが見つからなくても、ポートがリッスンされていればrunningとする
			// (Gradleのような多段プロセスでは親PIDが終了し子プロセスが残る)
			if status == "stopped" && resolvedPort > 0 {
				checkCmd := exec.Command("lsof", "-t", "-i", fmt.Sprintf(":%d", resolvedPort), "-sTCP:LISTEN")
				if out, err := checkCmd.Output(); err == nil && len(out) > 0 {
					status = "running"
				}
			}
		}

		state := process.ReadState(o.sangoDir, o.wtKey, name)
		health := ""
		if state.HealthStatus != "" {
			health = state.HealthStatus
		}

		// プロセスが停止しているのにhealthがhealthyのままの場合はstaleとしてリセット
		if status == "stopped" && health == "healthy" {
			health = ""
		}

		// repo-onlyサービス判定: repoあり + commandなし
		isRepoOnly := svc.Repo != "" && svc.Command == ""

		result.Services = append(result.Services, ServiceInfo{
			Name:         name,
			Type:         svc.Type,
			Port:         resolvedPort,
			Status:       status,
			Health:       health,
			PID:          pid,
			RestartCount: state.RestartCount,
			IsRepoOnly:   isRepoOnly,
			IsShared:     svc.Shared,
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
// wtName配下を優先し、存在しなければdirName直下にフォールバックする
// （sango clone未実行のmainワークツリー対応）
func ResolveWorkingDir(svc *config.Service, wtName, serviceName string) string {
	// repo_nameが設定されている場合、参照先サービスのディレクトリを使う
	dirName := serviceName
	if svc.RepoName != "" {
		dirName = svc.RepoName
	}

	if svc.WorkingDir != "" {
		// wtName/dirName/WorkingDir を優先
		fullPath := filepath.Join(wtName, dirName, svc.WorkingDir)
		if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
			return fullPath
		}
		// フォールバック: dirName/WorkingDir（clone未実行のmain用）
		fallback := filepath.Join(dirName, svc.WorkingDir)
		if info, err := os.Stat(fallback); err == nil && info.IsDir() {
			return fallback
		}
		return fullPath
	}

	// WorkingDir未設定: wtName/dirName を優先
	wtDir := filepath.Join(wtName, dirName)
	if info, err := os.Stat(wtDir); err == nil && info.IsDir() {
		return wtDir
	}
	// フォールバック: dirName直下（clone未実行のmain用）
	if info, err := os.Stat(dirName); err == nil && info.IsDir() {
		return dirName
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

// LoadAndValidateConfig は設定ファイルの読み込み・検証をまとめて行う
// 変数展開はオフセット決定後にNewOrchestratorWithWorktreeで実行される
func LoadAndValidateConfig(cfgFile string) (*config.Config, error) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}
