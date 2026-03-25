package worktree

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()

	s := &WorktreeState{
		Active: "main",
		Worktrees: map[string]*WorktreeInfo{
			"main": {
				Offset:    0,
				CreatedAt: time.Now().Truncate(time.Second),
				Services:  []string{"api", "web"},
			},
		},
		SharedServices: map[string]*SharedService{
			"postgres": {Port: 5432},
		},
		NextOffset: 100,
	}

	if err := s.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// ファイルが作られたか確認
	if _, err := os.Stat(filepath.Join(dir, "worktrees.json")); err != nil {
		t.Fatalf("worktrees.json not found: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Active != "main" {
		t.Errorf("Active = %q, want %q", loaded.Active, "main")
	}
	if loaded.NextOffset != 100 {
		t.Errorf("NextOffset = %d, want 100", loaded.NextOffset)
	}
	if len(loaded.Worktrees) != 1 {
		t.Errorf("Worktrees count = %d, want 1", len(loaded.Worktrees))
	}
	if wt, ok := loaded.Worktrees["main"]; !ok {
		t.Error("Worktrees[main] not found")
	} else {
		if len(wt.Services) != 2 {
			t.Errorf("Services count = %d, want 2", len(wt.Services))
		}
	}
	if ss, ok := loaded.SharedServices["postgres"]; !ok {
		t.Error("SharedServices[postgres] not found")
	} else if ss.Port != 5432 {
		t.Errorf("postgres port = %d, want 5432", ss.Port)
	}
}

func TestLoadNonExistent(t *testing.T) {
	dir := t.TempDir()
	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Active != "" {
		t.Errorf("Active = %q, want empty", s.Active)
	}
	if len(s.Worktrees) != 0 {
		t.Errorf("Worktrees should be empty")
	}
}

func TestAddRemoveWorktree(t *testing.T) {
	s := newEmptyState()
	s.AddWorktree("main", &WorktreeInfo{
		Offset:    0,
		CreatedAt: time.Now(),
		Services:  []string{"api"},
	})

	if _, ok := s.Worktrees["main"]; !ok {
		t.Error("main worktree not found after add")
	}

	s.RemoveWorktree("main")
	if _, ok := s.Worktrees["main"]; ok {
		t.Error("main worktree still exists after remove")
	}
}

func TestSetActive(t *testing.T) {
	s := newEmptyState()
	s.SetActive("feature/auth")
	if s.Active != "feature/auth" {
		t.Errorf("Active = %q, want %q", s.Active, "feature/auth")
	}
}

func TestDetectFromPath(t *testing.T) {
	// テスト用ディレクトリ構造:
	// tmpdir/.sango/worktrees.json
	// tmpdir/worktrees/ryugen04/feat-100/app-web/
	// tmpdir/worktrees/ryugen04/feat-200/app-backend/
	// tmpdir/worktrees/develop/
	tmpdir := t.TempDir()
	sangoDir := filepath.Join(tmpdir, ".sango")
	os.MkdirAll(sangoDir, 0o755)

	ws := &WorktreeState{
		Active: "ryugen04/feat-200",
		Worktrees: map[string]*WorktreeInfo{
			"develop":           {Offset: 0},
			"ryugen04/feat-100":  {Offset: 1600},
			"ryugen04/feat-200":  {Offset: 1500},
		},
	}

	tests := []struct {
		name string
		cwd  string
		want string
	}{
		{
			name: "worktreeルートにいる場合",
			cwd:  filepath.Join(tmpdir, "worktrees", "ryugen04", "feat-100"),
			want: "ryugen04/feat-100",
		},
		{
			name: "worktree内のサブディレクトリにいる場合",
			cwd:  filepath.Join(tmpdir, "worktrees", "ryugen04", "feat-100", "app-web", "src"),
			want: "ryugen04/feat-100",
		},
		{
			name: "別のworktreeにいる場合",
			cwd:  filepath.Join(tmpdir, "worktrees", "ryugen04", "feat-200"),
			want: "ryugen04/feat-200",
		},
		{
			name: "developにいる場合",
			cwd:  filepath.Join(tmpdir, "worktrees", "develop"),
			want: "develop",
		},
		{
			name: "プロジェクトルートにいる場合（worktrees外）",
			cwd:  tmpdir,
			want: "",
		},
		{
			name: "worktreesディレクトリ直下にいる場合",
			cwd:  filepath.Join(tmpdir, "worktrees"),
			want: "",
		},
		{
			name: "未登録のworktreeパスにいる場合",
			cwd:  filepath.Join(tmpdir, "worktrees", "unknown", "branch"),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectFromPath(sangoDir, tt.cwd, ws)
			if got != tt.want {
				t.Errorf("detectFromPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectFromPath_NilState(t *testing.T) {
	got := detectFromPath("/tmp/.sango", "/tmp/worktrees/foo", nil)
	if got != "" {
		t.Errorf("expected empty for nil state, got %q", got)
	}
}

func TestDetectFromPath_EmptyWorktrees(t *testing.T) {
	ws := &WorktreeState{Worktrees: map[string]*WorktreeInfo{}}
	got := detectFromPath("/tmp/.sango", "/tmp/worktrees/foo", ws)
	if got != "" {
		t.Errorf("expected empty for empty worktrees, got %q", got)
	}
}

func TestAllocateOffset(t *testing.T) {
	s := newEmptyState()
	s.NextOffset = 0

	o1 := s.AllocateOffset(100)
	if o1 != 0 {
		t.Errorf("first offset = %d, want 0", o1)
	}
	if s.NextOffset != 100 {
		t.Errorf("NextOffset = %d, want 100", s.NextOffset)
	}

	o2 := s.AllocateOffset(100)
	if o2 != 100 {
		t.Errorf("second offset = %d, want 100", o2)
	}
}
