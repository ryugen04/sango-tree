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
)

// Manager はサービスプロセスの起動・停止を管理する
type Manager struct {
	groveDir string
	worktree string
	mu       sync.Mutex
	cmds     map[string]*exec.Cmd
}

// NewManager はProcessManagerを生成する
func NewManager(groveDir, worktree string) *Manager {
	return &Manager{
		groveDir: groveDir,
		worktree: worktree,
		cmds:     make(map[string]*exec.Cmd),
	}
}

// StartOptions はプロセス起動のオプション
type StartOptions struct {
	Name       string
	Command    string
	Args       []string
	WorkingDir string
	Env        map[string]string
}

// Start はプロセスを起動し、PIDファイルに記録する
func (m *Manager) Start(opts StartOptions) (int, error) {
	cmd := exec.Command(opts.Command, opts.Args...)
	cmd.Dir = opts.WorkingDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// 環境変数を設定
	if len(opts.Env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range opts.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start process %s: %w", opts.Name, err)
	}

	pid := cmd.Process.Pid

	// cmd参照を保持
	m.mu.Lock()
	m.cmds[opts.Name] = cmd
	m.mu.Unlock()

	// バックグラウンドでWaitしてゾンビを回収
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Warn().Str("service", opts.Name).Err(err).Msg("process exited with error")
		}
		m.mu.Lock()
		delete(m.cmds, opts.Name)
		m.mu.Unlock()
	}()

	if err := WritePID(m.groveDir, m.worktree, opts.Name, pid); err != nil {
		return pid, fmt.Errorf("failed to write pid file: %w", err)
	}

	log.Info().Str("service", opts.Name).Int("pid", pid).Msg("process started")
	return pid, nil
}

// Stop は指定サービスのプロセスを停止する
func (m *Manager) Stop(service string) error {
	pid, err := ReadPID(m.groveDir, m.worktree, service)
	if err != nil {
		return fmt.Errorf("failed to read pid for %s: %w", service, err)
	}

	// プロセスグループ全体にSIGTERMを送信（孤児プロセス防止）
	// 負PID = プロセスグループ全体に送信
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			_ = RemovePID(m.groveDir, m.worktree, service)
			return nil
		}
		// プロセスグループへの送信に失敗した場合、単一PIDにフォールバック
		if err2 := syscall.Kill(pid, syscall.SIGTERM); err2 != nil {
			if errors.Is(err2, syscall.ESRCH) {
				_ = RemovePID(m.groveDir, m.worktree, service)
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
					_ = RemovePID(m.groveDir, m.worktree, service)
					return nil
				}
				// フォールバック: 単一PIDにSIGKILL
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
			// Wait goroutineがゾンビを回収するのを待つ
			time.Sleep(200 * time.Millisecond)
			_ = RemovePID(m.groveDir, m.worktree, service)
			return nil
		case <-ticker.C:
			if !IsProcessRunning(pid) {
				_ = RemovePID(m.groveDir, m.worktree, service)
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

	pid, err := ReadPID(m.groveDir, m.worktree, service)
	if err != nil {
		return false
	}
	return IsProcessRunning(pid)
}
