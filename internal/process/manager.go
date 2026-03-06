package process

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	sangoLog "github.com/ryugen04/sango-tree/internal/log"
)

// Manager はサービスプロセスの起動・停止を管理する
type Manager struct {
	sangoDir string
	worktree string
	mu       sync.Mutex
	cmds     map[string]*exec.Cmd
	stopping map[string]bool
}

// NewManager はProcessManagerを生成する
func NewManager(sangoDir, worktree string) *Manager {
	return &Manager{
		sangoDir: sangoDir,
		worktree: worktree,
		cmds:     make(map[string]*exec.Cmd),
		stopping: make(map[string]bool),
	}
}

// StartOptions はプロセス起動のオプション
type StartOptions struct {
	Name         string
	Command      string
	Args         []string
	WorkingDir   string
	Env          map[string]string
	Restart      string        // "always" | "on-failure" | "" (no)
	RestartDelay time.Duration
	MaxRestarts  int
}

// Start はプロセスを起動し、PIDファイルに記録する
func (m *Manager) Start(opts StartOptions) (int, error) {
	cmd := exec.Command(opts.Command, opts.Args...)
	cmd.Dir = opts.WorkingDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// ログコレクターを初期化
	collector, err := sangoLog.NewCollector(m.sangoDir, m.worktree, opts.Name)
	if err != nil {
		log.Warn().Str("service", opts.Name).Err(err).Msg("ログコレクターの初期化に失敗、標準出力に直接出力")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		stdoutW, err := collector.StdoutWriter()
		if err != nil {
			log.Warn().Str("service", opts.Name).Err(err).Msg("stdoutパイプの作成に失敗")
			collector.Close()
			collector = nil
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		} else {
			stderrW, err := collector.StderrWriter()
			if err != nil {
				log.Warn().Str("service", opts.Name).Err(err).Msg("stderrパイプの作成に失敗")
				collector.Close()
				collector = nil
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
			} else {
				cmd.Stdout = stdoutW
				cmd.Stderr = stderrW
			}
		}
	}

	// 環境変数を設定
	if len(opts.Env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range opts.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	if err := cmd.Start(); err != nil {
		if collector != nil {
			collector.Close()
		}
		return 0, fmt.Errorf("failed to start process %s: %w", opts.Name, err)
	}

	pid := cmd.Process.Pid

	// cmd参照を保持
	m.mu.Lock()
	m.cmds[opts.Name] = cmd
	m.mu.Unlock()

	// バックグラウンドでWaitしてゾンビを回収、必要に応じて再起動
	go func() {
		restartCount := 0
		for {
			err := cmd.Wait()
			if err != nil {
				log.Warn().Str("service", opts.Name).Err(err).Msg("process exited with error")
			}
			if collector != nil {
				collector.Close()
				collector = nil
			}

			m.mu.Lock()
			stopping := m.stopping[opts.Name]
			m.mu.Unlock()
			if stopping {
				break
			}

			shouldRestart := false
			switch opts.Restart {
			case "always":
				shouldRestart = true
			case "on-failure":
				shouldRestart = (err != nil)
			}

			if !shouldRestart || (opts.MaxRestarts > 0 && restartCount >= opts.MaxRestarts) {
				break
			}

			restartCount++
			// 既存stateを読み込んでRestartCountのみ更新（HealthStatusを保持）
			existingState := ReadState(m.sangoDir, m.worktree, opts.Name)
			existingState.RestartCount = restartCount
			_ = WriteState(m.sangoDir, m.worktree, opts.Name, existingState)
			log.Info().Str("service", opts.Name).Int("restart_count", restartCount).Msg("restarting process")

			if opts.RestartDelay > 0 {
				time.Sleep(opts.RestartDelay)
			}

			// sleep後に再度stoppingを確認（Stop()がsleep中に呼ばれた場合）
			m.mu.Lock()
			stopping = m.stopping[opts.Name]
			m.mu.Unlock()
			if stopping {
				break
			}

			// 新しいコマンドを構築・起動
			newCmd := exec.Command(opts.Command, opts.Args...)
			newCmd.Dir = opts.WorkingDir
			newCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			if len(opts.Env) > 0 {
				newCmd.Env = os.Environ()
				for k, v := range opts.Env {
					newCmd.Env = append(newCmd.Env, fmt.Sprintf("%s=%s", k, v))
				}
			}

			// ログコレクターを再初期化
			newCollector, collErr := sangoLog.NewCollector(m.sangoDir, m.worktree, opts.Name)
			if collErr != nil {
				newCmd.Stdout = os.Stdout
				newCmd.Stderr = os.Stderr
			} else if stdoutW, err := newCollector.StdoutWriter(); err != nil {
				newCollector.Close()
				newCollector = nil
				newCmd.Stdout = os.Stdout
				newCmd.Stderr = os.Stderr
			} else if stderrW, err := newCollector.StderrWriter(); err != nil {
				newCollector.Close()
				newCollector = nil
				newCmd.Stdout = os.Stdout
				newCmd.Stderr = os.Stderr
			} else {
				newCmd.Stdout = stdoutW
				newCmd.Stderr = stderrW
			}
			collector = newCollector

			if err := newCmd.Start(); err != nil {
				log.Error().Str("service", opts.Name).Err(err).Msg("restart failed")
				break
			}

			_ = WritePID(m.sangoDir, m.worktree, opts.Name, newCmd.Process.Pid)
			m.mu.Lock()
			m.cmds[opts.Name] = newCmd
			m.mu.Unlock()
			cmd = newCmd
		}
		// ループ脱出: クリーンアップ
		_ = RemovePID(m.sangoDir, m.worktree, opts.Name)
		m.mu.Lock()
		delete(m.cmds, opts.Name)
		m.mu.Unlock()
	}()

	if err := WritePID(m.sangoDir, m.worktree, opts.Name, pid); err != nil {
		return pid, fmt.Errorf("failed to write pid file: %w", err)
	}

	log.Info().Str("service", opts.Name).Int("pid", pid).Msg("process started")
	return pid, nil
}

// Stop は指定サービスのプロセスを停止する
func (m *Manager) Stop(service string) error {
	m.mu.Lock()
	m.stopping[service] = true
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		delete(m.stopping, service)
		m.mu.Unlock()
	}()

	pid, err := ReadPID(m.sangoDir, m.worktree, service)
	if err != nil {
		return fmt.Errorf("failed to read pid for %s: %w", service, err)
	}

	// プロセスグループ全体にSIGTERMを送信（孤児プロセス防止）
	// 負PID = プロセスグループ全体に送信
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			_ = RemovePID(m.sangoDir, m.worktree, service)
			return nil
		}
		// プロセスグループへの送信に失敗した場合、単一PIDにフォールバック
		if err2 := syscall.Kill(pid, syscall.SIGTERM); err2 != nil {
			if errors.Is(err2, syscall.ESRCH) {
				_ = RemovePID(m.sangoDir, m.worktree, service)
				return nil
			}
			return fmt.Errorf("failed to send SIGTERM to %s (pid=%d): %w", service, pid, err2)
		}
	}

	// 10秒間待機してプロセス終了を確認
	deadline := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			// タイムアウト: プロセスグループ全体にSIGKILLを送信
			log.Warn().Str("service", service).Int("pid", pid).Msg("SIGTERM timeout, sending SIGKILL")
			if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
				if errors.Is(err, syscall.ESRCH) {
					_ = RemovePID(m.sangoDir, m.worktree, service)
					return nil
				}
				// フォールバック: 単一PIDにSIGKILL
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
			// Wait goroutineがゾンビを回収するのを待つ
			time.Sleep(200 * time.Millisecond)
			_ = RemovePID(m.sangoDir, m.worktree, service)
			return nil
		case <-ticker.C:
			if !IsProcessRunning(pid) {
				_ = RemovePID(m.sangoDir, m.worktree, service)
				log.Info().Str("service", service).Int("pid", pid).Msg("process stopped")
				return nil
			}
		}
	}
}

// StopAll は全サービスを停止する
func (m *Manager) StopAll(services []string) error {
	var lastErr error
	for _, svc := range services {
		if err := m.Stop(svc); err != nil {
			log.Error().Str("service", svc).Err(err).Msg("failed to stop service")
			lastErr = err
		}
	}
	return lastErr
}

// IsRunning は指定サービスが実行中か確認する
func (m *Manager) IsRunning(service string) bool {
	m.mu.Lock()
	_, managed := m.cmds[service]
	m.mu.Unlock()
	if managed {
		return true
	}

	pid, err := ReadPID(m.sangoDir, m.worktree, service)
	if err != nil {
		return false
	}
	return IsProcessRunning(pid)
}
