package config

import (
	"path/filepath"
	"runtime"
	"testing"
)

// testdataディレクトリのパスを取得する
func testdataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	return filepath.Join(dir, "..", "..", "testdata", name)
}

func TestLoadValidConfig(t *testing.T) {
	cfg, err := Load(testdataPath("valid.yaml"))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Name != "test-app" {
		t.Errorf("Name = %q, want %q", cfg.Name, "test-app")
	}
	if cfg.Version != "1.0" {
		t.Errorf("Version = %q, want %q", cfg.Version, "1.0")
	}
	if len(cfg.Services) != 3 {
		t.Errorf("Services count = %d, want 3", len(cfg.Services))
	}

	// postgresサービスの確認
	pg := cfg.Services["postgres"]
	if pg == nil {
		t.Fatal("postgres service not found")
	}
	if pg.Type != "docker" {
		t.Errorf("postgres.Type = %q, want %q", pg.Type, "docker")
	}
	if pg.Image != "postgres:16" {
		t.Errorf("postgres.Image = %q, want %q", pg.Image, "postgres:16")
	}
	if pg.Port != 5432 {
		t.Errorf("postgres.Port = %d, want 5432", pg.Port)
	}
	if !pg.Shared {
		t.Error("postgres.Shared = false, want true")
	}

	// apiサービスの確認
	api := cfg.Services["api"]
	if api == nil {
		t.Fatal("api service not found")
	}
	if len(api.DependsOn) != 1 || api.DependsOn[0] != "postgres" {
		t.Errorf("api.DependsOn = %v, want [postgres]", api.DependsOn)
	}
	if len(api.CommandArgs) != 2 {
		t.Errorf("api.CommandArgs count = %d, want 2", len(api.CommandArgs))
	}

	// portsの確認
	if cfg.Ports.Strategy != "fixed" {
		t.Errorf("Ports.Strategy = %q, want %q", cfg.Ports.Strategy, "fixed")
	}
	if cfg.Ports.Range != [2]int{3000, 9999} {
		t.Errorf("Ports.Range = %v, want [3000, 9999]", cfg.Ports.Range)
	}

	// profilesの確認
	backend, ok := cfg.Profiles["backend"]
	if !ok {
		t.Fatal("backend profile not found")
	}
	if len(backend.Services) != 2 {
		t.Errorf("backend.Services count = %d, want 2", len(backend.Services))
	}

	// doctorの確認
	if len(cfg.Doctor.Checks) != 2 {
		t.Errorf("Doctor.Checks count = %d, want 2", len(cfg.Doctor.Checks))
	}

	// Validate が通ること
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate failed: %v", err)
	}
}

func TestLoadMinimalConfig(t *testing.T) {
	cfg, err := Load(testdataPath("minimal.yaml"))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Name != "minimal" {
		t.Errorf("Name = %q, want %q", cfg.Name, "minimal")
	}
	if len(cfg.Services) != 1 {
		t.Errorf("Services count = %d, want 1", len(cfg.Services))
	}

	app := cfg.Services["app"]
	if app == nil {
		t.Fatal("app service not found")
	}
	if app.Port != 3000 {
		t.Errorf("app.Port = %d, want 3000", app.Port)
	}
	if app.Command != "npm start" {
		t.Errorf("app.Command = %q, want %q", app.Command, "npm start")
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate failed: %v", err)
	}
}

func TestValidateInvalidType(t *testing.T) {
	cfg, err := Load(testdataPath("invalid_type.yaml"))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	err = cfg.Validate()
	if err == nil {
		t.Fatal("Validate should fail for invalid type")
	}
	t.Logf("Expected error: %v", err)
}

func TestValidateMissingImage(t *testing.T) {
	cfg := &Config{
		Name: "test",
		Services: map[string]*Service{
			"db": {
				Type: "docker",
				// Image未設定
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate should fail for docker without image")
	}
	t.Logf("Expected error: %v", err)
}

func TestValidateMissingCommand(t *testing.T) {
	cfg := &Config{
		Name: "test",
		Services: map[string]*Service{
			"api": {
				Type: "process",
				// Command未設定
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate should fail for process without command")
	}
	t.Logf("Expected error: %v", err)
}

func TestValidateUnknownDependency(t *testing.T) {
	cfg := &Config{
		Name: "test",
		Services: map[string]*Service{
			"api": {
				Type:      "process",
				Command:   "go run .",
				DependsOn: []string{"nonexistent"},
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate should fail for unknown dependency")
	}
	t.Logf("Expected error: %v", err)
}

func TestExpandVariables(t *testing.T) {
	cfg := &Config{
		Name: "test",
		Services: map[string]*Service{
			"postgres": {
				Type:  "docker",
				Image: "postgres:16",
				Port:  5432,
			},
			"api": {
				Type:    "process",
				Port:    8080,
				Command: "go run .",
				CommandArgs: []string{
					"--port=${port}",
					"--db-port=${services.postgres.port}",
				},
				EnvDynamic: map[string]string{
					"PORT":         "${port}",
					"DATABASE_URL": "postgres://localhost:${services.postgres.port}/myapp",
				},
				Healthcheck: &Healthcheck{
					URL: "http://localhost:${port}/health",
				},
			},
		},
	}

	ExpandVariables(cfg)

	api := cfg.Services["api"]

	// CommandArgs の検証
	if api.CommandArgs[0] != "--port=8080" {
		t.Errorf("CommandArgs[0] = %q, want %q", api.CommandArgs[0], "--port=8080")
	}
	if api.CommandArgs[1] != "--db-port=5432" {
		t.Errorf("CommandArgs[1] = %q, want %q", api.CommandArgs[1], "--db-port=5432")
	}

	// EnvDynamic の検証
	if api.EnvDynamic["PORT"] != "8080" {
		t.Errorf("EnvDynamic[PORT] = %q, want %q", api.EnvDynamic["PORT"], "8080")
	}
	if api.EnvDynamic["DATABASE_URL"] != "postgres://localhost:5432/myapp" {
		t.Errorf("EnvDynamic[DATABASE_URL] = %q, want %q", api.EnvDynamic["DATABASE_URL"], "postgres://localhost:5432/myapp")
	}

	// Healthcheck.URL の検証
	if api.Healthcheck.URL != "http://localhost:8080/health" {
		t.Errorf("Healthcheck.URL = %q, want %q", api.Healthcheck.URL, "http://localhost:8080/health")
	}
}
