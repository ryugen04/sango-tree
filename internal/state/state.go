package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// State はgroveのランタイム状態を保持する
type State struct {
	ActiveWorktree string                   `json:"active_worktree"`
	Services       map[string]*ServiceState `json:"services"`
}

// ServiceState は各サービスの状態を保持する
type ServiceState struct {
	Port      int       `json:"port"`
	Status    string    `json:"status"` // running | stopped | error
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
}

// DefaultDir は .grove ディレクトリのパスを返す
func DefaultDir() string {
	return ".grove"
}

// Load は .grove/state.json を読み込む。ファイルがなければ空のStateを返す
func Load(groveDir string) (*State, error) {
	p := filepath.Join(groveDir, "state.json")
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{Services: map[string]*ServiceState{}}, nil
		}
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.Services == nil {
		s.Services = map[string]*ServiceState{}
	}
	return &s, nil
}

// Save は .grove/state.json に書き込む
func (s *State) Save(groveDir string) error {
	if err := os.MkdirAll(groveDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(groveDir, "state.json"), data, 0o644)
}

// SetService はサービスの状態を更新する
func (s *State) SetService(name string, svc *ServiceState) {
	if s.Services == nil {
		s.Services = map[string]*ServiceState{}
	}
	s.Services[name] = svc
}

// RemoveService はサービスの状態を削除する
func (s *State) RemoveService(name string) {
	delete(s.Services, name)
}

// ClearAll は全サービスの状態をクリアする
func (s *State) ClearAll() {
	s.Services = map[string]*ServiceState{}
}
