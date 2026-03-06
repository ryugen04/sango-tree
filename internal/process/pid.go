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
func PIDDir(sangoDir, worktree string) string {
	return filepath.Join(sangoDir, "pids", worktree)
}

// pidPath はPIDファイルのパスを返す
func pidPath(sangoDir, worktree, service string) string {
	return filepath.Join(PIDDir(sangoDir, worktree), service+".pid")
}

// WritePID はPIDファイルを書き込む
func WritePID(sangoDir, worktree, service string, pid int) error {
	dir := PIDDir(sangoDir, worktree)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(pidPath(sangoDir, worktree, service), []byte(strconv.Itoa(pid)), 0o644)
}

// ReadPID はPIDファイルからPIDを読み取る
func ReadPID(sangoDir, worktree, service string) (int, error) {
	data, err := os.ReadFile(pidPath(sangoDir, worktree, service))
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
func RemovePID(sangoDir, worktree, service string) error {
	return os.Remove(pidPath(sangoDir, worktree, service))
}

// IsProcessRunning はPIDのプロセスが生存しているか確認する
func IsProcessRunning(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
