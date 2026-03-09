package process

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

// FindPIDOwner は全worktreeのPIDファイルを走査し、指定PIDの所有者を特定する
// sangoDir: .sangoディレクトリのパス
// targetPID: 検索対象のPID
// 戻り値: worktree名, service名, 見つかったか
func FindPIDOwner(sangoDir string, targetPID int) (worktreeName string, serviceName string, found bool) {
	pidsDir := filepath.Join(sangoDir, "pids")
	entries, err := os.ReadDir(pidsDir)
	if err != nil {
		return "", "", false
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		wtName := entry.Name()
		wtDir := filepath.Join(pidsDir, wtName)
		pidFiles, err := os.ReadDir(wtDir)
		if err != nil {
			continue
		}
		for _, pidFile := range pidFiles {
			if pidFile.IsDir() {
				continue
			}
			svcName := strings.TrimSuffix(pidFile.Name(), ".pid")
			if svcName == pidFile.Name() {
				continue // .pid拡張子でないファイルはスキップ
			}
			pid, err := ReadPID(sangoDir, wtName, svcName)
			if err != nil {
				continue
			}
			if pid == targetPID {
				return wtName, svcName, true
			}
		}
	}
	return "", "", false
}
