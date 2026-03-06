//go:build !windows

package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// Lock はファイルロックを表す
type Lock struct {
	file *os.File
	path string
}

// AcquireLock はファイルロックを取得する（ブロッキング）
func AcquireLock(sangoDir, name string) (*Lock, error) {
	lockDir := filepath.Join(sangoDir, "locks")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return nil, fmt.Errorf("ロックディレクトリの作成に失敗: %w", err)
	}

	lockPath := filepath.Join(lockDir, name+".lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("ロックファイルのオープンに失敗: %w", err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("ロックの取得に失敗: %w", err)
	}

	return &Lock{file: f, path: lockPath}, nil
}

// TryLock はファイルロックを非ブロッキングで試行する
func TryLock(sangoDir, name string) (*Lock, error) {
	lockDir := filepath.Join(sangoDir, "locks")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return nil, fmt.Errorf("ロックディレクトリの作成に失敗: %w", err)
	}

	lockPath := filepath.Join(lockDir, name+".lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("ロックファイルのオープンに失敗: %w", err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("ロックは既に取得されています: %w", err)
	}

	return &Lock{file: f, path: lockPath}, nil
}

// Release はファイルロックを解放する
func (l *Lock) Release() error {
	if l.file == nil {
		return nil
	}
	if err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN); err != nil {
		l.file.Close()
		return err
	}
	return l.file.Close()
}
