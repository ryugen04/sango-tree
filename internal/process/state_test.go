package process

import (
	"os"
	"testing"
)

func TestWriteAndReadState(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sango-state-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	state := &ServiceState{
		RestartCount: 3,
		HealthStatus: "healthy",
	}

	if err := WriteState(tmpDir, "test-wt", "myservice", state); err != nil {
		t.Fatalf("WriteState failed: %v", err)
	}

	got := ReadState(tmpDir, "test-wt", "myservice")
	if got.RestartCount != 3 {
		t.Errorf("RestartCount = %d, want 3", got.RestartCount)
	}
	if got.HealthStatus != "healthy" {
		t.Errorf("HealthStatus = %q, want %q", got.HealthStatus, "healthy")
	}
}

func TestReadStateFileNotExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sango-state-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	got := ReadState(tmpDir, "nonexistent", "noservice")
	if got.RestartCount != 0 {
		t.Errorf("RestartCount = %d, want 0", got.RestartCount)
	}
	if got.HealthStatus != "" {
		t.Errorf("HealthStatus = %q, want empty", got.HealthStatus)
	}
}
