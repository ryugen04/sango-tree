package worktree

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// WorktreeState はworktrees.jsonのルート構造体
type WorktreeState struct {
	Active         string                    `json:"active"`
	Worktrees      map[string]*WorktreeInfo  `json:"worktrees"`
	SharedServices map[string]*SharedService `json:"shared_services"`
	NextOffset     int                       `json:"next_offset"`
}

// WorktreeInfo は個別ワークツリーのメタデータ
type WorktreeInfo struct {
	Offset     int       `json:"offset"`
	CreatedAt  time.Time `json:"created_at"`
	Services   []string  `json:"services"`
	FromBranch string    `json:"from_branch,omitempty"`
}

// SharedService は共有サービスのメタデータ
type SharedService struct {
	Port int `json:"port"`
}

// DefaultDir は.groveディレクトリのパスを返す
func DefaultDir() string {
	return ".grove"
}

const stateFile = "worktrees.json"

// Load はworktrees.jsonを読み込む。ファイルがなければ空のStateを返す
func Load(groveDir string) (*WorktreeState, error) {
	p := filepath.Join(groveDir, stateFile)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return newEmptyState(), nil
		}
		return nil, err
	}
	var s WorktreeState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.Worktrees == nil {
		s.Worktrees = make(map[string]*WorktreeInfo)
	}
	if s.SharedServices == nil {
		s.SharedServices = make(map[string]*SharedService)
	}
	return &s, nil
}

// Save はworktrees.jsonに書き込む
func (s *WorktreeState) Save(groveDir string) error {
	if err := os.MkdirAll(groveDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(groveDir, stateFile), data, 0o644)
}

// GetActiveWorktree はアクティブなworktreeの情報を返す
func (s *WorktreeState) GetActiveWorktree() (*WorktreeInfo, bool) {
	wt, ok := s.Worktrees[s.Active]
	return wt, ok
}

// SetActive はアクティブworktreeを変更する
func (s *WorktreeState) SetActive(name string) {
	s.Active = name
}

// AddWorktree はworktreeを追加する
func (s *WorktreeState) AddWorktree(name string, info *WorktreeInfo) {
	if s.Worktrees == nil {
		s.Worktrees = make(map[string]*WorktreeInfo)
	}
	s.Worktrees[name] = info
}

// RemoveWorktree はworktreeを削除する
func (s *WorktreeState) RemoveWorktree(name string) {
	delete(s.Worktrees, name)
}

// AllocateOffset は次のオフセットを割り当てて返す
// baseOffsetが0以下の場合はデフォルト100を使用する
func (s *WorktreeState) AllocateOffset(baseOffset int) int {
	if baseOffset <= 0 {
		baseOffset = 100
	}
	offset := s.NextOffset
	s.NextOffset += baseOffset
	return offset
}

func newEmptyState() *WorktreeState {
	return &WorktreeState{
		Worktrees:      make(map[string]*WorktreeInfo),
		SharedServices: make(map[string]*SharedService),
	}
}
