package process

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// PIDDir はPIDファイルの格納ディレクトリを返す
func PIDDir(groveDir, worktree string) string {
	return filepath.Join(groveDir, "pids", worktree)
}

// pidPath はPIDファイルのパスを返す
func pidPath(groveDir, worktree, service string) string {
	return filepath.Join(PIDDir(groveDir, worktree), service+".pid")
}

// WritePID はPIDファイルを書き込む
func WritePID(groveDir, worktree, service string, pid int) error {
	dir := PIDDir(groveDir, worktree)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(pidPath(groveDir, worktree, service), []byte(strconv.Itoa(pid)), 0o644)
}

// ReadPID はPIDファイルからPIDを読み取る
func ReadPID(groveDir, worktree, service string) (int, error) {
	data, err := os.ReadFile(pidPath(groveDir, worktree, service))
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, fmt.Errorf("invalid pid file: %w", err)
	}
	return pid, nil
}

// RemovePID はPIDファイルを削除する
func RemovePID(groveDir, worktree, service string) error {
	return os.Remove(pidPath(groveDir, worktree, service))
}

// IsProcessRunning はPIDのプロセスが生存しているか確認する
func IsProcessRunning(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
