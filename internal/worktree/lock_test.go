//go:build !windows

package worktree

import (
	"testing"
)

func TestAcquireRelease(t *testing.T) {
	dir := t.TempDir()

	lock, err := AcquireLock(dir, "test-op")
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}

	if err := lock.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

func TestTryLockConflict(t *testing.T) {
	dir := t.TempDir()

	lock1, err := AcquireLock(dir, "test-op")
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	defer lock1.Release()

	// 同じ名前で非ブロッキングロックを試行 → 失敗するはず
	_, err = TryLock(dir, "test-op")
	if err == nil {
		t.Fatal("TryLock should fail when lock is held")
	}
}

func TestDifferentLockNames(t *testing.T) {
	dir := t.TempDir()

	lock1, err := AcquireLock(dir, "op-a")
	if err != nil {
		t.Fatalf("AcquireLock op-a: %v", err)
	}
	defer lock1.Release()

	lock2, err := TryLock(dir, "op-b")
	if err != nil {
		t.Fatalf("TryLock op-b should succeed: %v", err)
	}
	defer lock2.Release()
}

func TestReleaseNil(t *testing.T) {
	l := &Lock{}
	if err := l.Release(); err != nil {
		t.Fatalf("Release nil file: %v", err)
	}
}
