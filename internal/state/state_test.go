package state

import (
	"testing"
	"time"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()

	original := &State{
		ActiveWorktree: "feature-x",
		Services: map[string]*ServiceState{
			"api": {
				Port:      8080,
				Status:    "running",
				PID:       12345,
				StartedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	if err := original.Save(dir); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.ActiveWorktree != original.ActiveWorktree {
		t.Errorf("ActiveWorktree: got %q, want %q", loaded.ActiveWorktree, original.ActiveWorktree)
	}

	svc, ok := loaded.Services["api"]
	if !ok {
		t.Fatal("service 'api' not found")
	}
	if svc.Port != 8080 {
		t.Errorf("Port: got %d, want %d", svc.Port, 8080)
	}
	if svc.Status != "running" {
		t.Errorf("Status: got %q, want %q", svc.Status, "running")
	}
	if svc.PID != 12345 {
		t.Errorf("PID: got %d, want %d", svc.PID, 12345)
	}
}

func TestLoadNotExist(t *testing.T) {
	dir := t.TempDir()

	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if s.ActiveWorktree != "" {
		t.Errorf("ActiveWorktree: got %q, want empty", s.ActiveWorktree)
	}
	if len(s.Services) != 0 {
		t.Errorf("Services: got %d entries, want 0", len(s.Services))
	}
}

func TestSetAndRemoveService(t *testing.T) {
	s := &State{Services: map[string]*ServiceState{}}

	s.SetService("db", &ServiceState{
		Port:   5432,
		Status: "running",
		PID:    999,
	})

	if _, ok := s.Services["db"]; !ok {
		t.Fatal("service 'db' not found after SetService")
	}

	s.RemoveService("db")

	if _, ok := s.Services["db"]; ok {
		t.Fatal("service 'db' still exists after RemoveService")
	}
}
