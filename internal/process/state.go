package process

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ServiceState はサービスの実行状態を保持する
type ServiceState struct {
	RestartCount int    `json:"restart_count"`
	HealthStatus string `json:"health_status"` // "healthy" | "unhealthy" | ""
}

// statePath はステートファイルのパスを返す
func statePath(sangoDir, worktree, service string) string {
	return filepath.Join(PIDDir(sangoDir, worktree), service+".state.json")
}

// ReadState はサービスの状態を読み取る。ファイルが存在しない場合はゼロ値を返す
func ReadState(sangoDir, worktree, service string) *ServiceState {
	data, err := os.ReadFile(statePath(sangoDir, worktree, service))
	if err != nil {
		return &ServiceState{}
	}
	var state ServiceState
	if err := json.Unmarshal(data, &state); err != nil {
		return &ServiceState{}
	}
	return &state
}

// WriteState はサービスの状態をファイルに書き込む
func WriteState(sangoDir, worktree, service string, state *ServiceState) error {
	dir := PIDDir(sangoDir, worktree)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(statePath(sangoDir, worktree, service), data, 0o644)
}
