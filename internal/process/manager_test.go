package process

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStartAndStop(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, "main")

	pid, err := m.Start(StartOptions{
		Name:    "test-sleep",
		Command: "sleep",
		Args:    []string{"60"},
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if pid == 0 {
		t.Fatal("pid should not be 0")
	}

	// PIDファイルが存在することを確認
	pidFile := filepath.Join(dir, "pids", "main", "test-sleep.pid")
	if _, err := os.Stat(pidFile); err != nil {
		t.Fatalf("pid file not found: %v", err)
	}

	// プロセスが動作していることを確認
	if !IsProcessRunning(pid) {
		t.Fatal("process should be running")
	}

	// 停止
	if err := m.Stop("test-sleep"); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// プロセスが停止していることを確認（少し待つ）
	time.Sleep(200 * time.Millisecond)
	if IsProcessRunning(pid) {
		t.Fatal("process should not be running after Stop")
	}

	// PIDファイルが削除されていることを確認
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Fatal("pid file should be removed after Stop")
	}
}

func TestIsRunning(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, "main")

	// 起動前はfalse
	if m.IsRunning("not-exist") {
		t.Fatal("IsRunning should return false for non-existent service")
	}

	pid, err := m.Start(StartOptions{
		Name:    "test-running",
		Command: "sleep",
		Args:    []string{"60"},
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// 起動中はtrue
	if !m.IsRunning("test-running") {
		t.Fatal("IsRunning should return true for running service")
	}

	// クリーンアップ
	if err := m.Stop("test-running"); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	_ = pid
}

func TestBuildDockerArgs(t *testing.T) {
	args := BuildDockerArgs(DockerOptions{
		Name:  "postgres",
		Image: "postgres:15",
		Port:  5432,
		Env: map[string]string{
			"POSTGRES_PASSWORD": "secret",
		},
		Volumes: []string{"/data:/var/lib/postgresql/data"},
	})

	// 基本引数の確認
	expected := map[string]bool{
		"run":                              false,
		"-d":                               false,
		"--name":                           false,
		"sango-postgres":                   false,
		"-p":                               false,
		"5432:5432":                        false,
		"-e":                               false,
		"POSTGRES_PASSWORD=secret":         false,
		"-v":                               false,
		"/data:/var/lib/postgresql/data":   false,
		"postgres:15":                      false,
	}

	for _, arg := range args {
		if _, ok := expected[arg]; ok {
			expected[arg] = true
		}
	}

	for k, found := range expected {
		if !found {
			t.Errorf("expected arg %q not found in %v", k, args)
		}
	}

	// イメージは最後の引数であること
	if args[len(args)-1] != "postgres:15" {
		t.Errorf("last arg should be image, got %q", args[len(args)-1])
	}
}
