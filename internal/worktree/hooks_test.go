package worktree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ryugen04/sango-tree/internal/config"
)

func TestRunHooks(t *testing.T) {
	dir := t.TempDir()
	// マーカーファイルを作成するフック
	hooks := []config.HookEntry{
		{Command: "touch hook_executed", PerService: false},
	}
	err := RunHooks(hooks, dir, []string{"svc1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "hook_executed")); os.IsNotExist(err) {
		t.Fatal("hook was not executed")
	}
}

func TestRunHooksPerService(t *testing.T) {
	dir := t.TempDir()
	// サービスディレクトリを作成
	os.MkdirAll(filepath.Join(dir, "svc1"), 0o755)
	os.MkdirAll(filepath.Join(dir, "svc2"), 0o755)

	hooks := []config.HookEntry{
		{Command: "touch per_svc_marker", PerService: true},
	}
	err := RunHooks(hooks, dir, []string{"svc1", "svc2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, svc := range []string{"svc1", "svc2"} {
		marker := filepath.Join(dir, svc, "per_svc_marker")
		if _, err := os.Stat(marker); os.IsNotExist(err) {
			t.Fatalf("per_service hook not executed for %s", svc)
		}
	}
}
